"""The zsh plugin, driven through a pseudo-terminal with fake `ush` and `sudo` commands."""

import json
import os
import pty
import re
import select
import shutil
import time
import unittest

from helpers import TempDirTest

PLUGIN = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "shell", "ush.plugin.zsh")
ZSH = shutil.which("zsh") or ""

FAKE = """#!/bin/sh
exec python3 -c 'import json, sys; print("ARGS" + json.dumps(sys.argv[1:]))' "{prefix}" "$@"
"""


@unittest.skipUnless(ZSH, "zsh not installed")
class ZshPluginTest(TempDirTest):
    def setUp(self):
        super().setUp()
        self.bin = os.path.join(self.tmp, "bin")
        os.mkdir(self.bin)
        for name, prefix in (("ush", "ush"), ("sudo", "sudo")):  # never the real sudo
            path = os.path.join(self.bin, name)
            with open(path, "w") as f:
                f.write(FAKE.format(prefix=prefix))
            os.chmod(path, 0o755)

    def run_lines(self, lines, env=None):
        pid, fd = pty.fork()
        if pid == 0:
            os.environ.update({"PATH": self.bin + ":/usr/bin:/bin", "PS1": "PROMPT> ",
                               "HOME": self.tmp, **(env or {})})
            os.execv(ZSH, [ZSH, "-f", "-i"])
        try:
            self._read(fd, "PROMPT> ")
            os.write(fd, f"source {PLUGIN}\r".encode())
            self._read(fd, "PROMPT> ")
            results = []
            for line in lines:
                os.write(fd, (line + "\r").encode())
                output = self._read(fd, "PROMPT> ")
                found = re.findall(r"ARGS(\[.*?\])\r?\n", output)
                results.append(json.loads(found[-1]) if found else output)
            os.write(fd, b"exit\r")
            return results
        finally:
            os.close(fd)
            os.waitpid(pid, 0)

    @staticmethod
    def _read(fd, until, timeout=10.0):
        out, end = "", time.time() + timeout
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.1)
            if ready:
                try:
                    out += os.read(fd, 4096).decode(errors="replace")
                except OSError:
                    break
                plain = re.sub(r"\x1b\[[0-9;?]*[A-Za-z]|\r", "", out)  # e.g. bracketed paste
                if plain.rstrip().endswith(until.rstrip()):
                    return out
        return out

    def test_requests_reach_ush_verbatim(self):
        ush = os.path.join(self.bin, "ush")
        results = self.run_lines([
            "ush //why don't my tools work? *",
            "// what's using $HOME and `space`?",
            "sudo -E ush //clean it's done",
            "  ush  //",
            "ush history",
        ])
        self.assertEqual(results[0], ["ush", "//why don't my tools work? *"])
        self.assertEqual(results[1], ["ush", "// what's using $HOME and `space`?"])
        self.assertEqual(results[2], ["sudo", "-E", ush, "//clean it's done"])
        self.assertEqual(results[3], ["ush", "//"])
        self.assertEqual(results[4], ["ush", "history"])

    def test_bare_slash_can_be_disabled(self):
        results = self.run_lines(["// hello"], env={"USH_BARE_SLASH": "0"})
        self.assertIsInstance(results[0], str)  # zsh ran "//" as a path, not ush
        self.assertNotIn("ARGS", results[0])


PROPOSER = """#!/bin/sh
exec python3 - "$@" <<'PY'
import json, os, sys
d = os.environ["USH_HANDOFF"]
args = sys.argv[1:]
if args[:1] == ["explain"]:
    print("EXPLAIN" + json.dumps(args[1:]))
elif args == ["//"]:
    def read(name):
        try:
            return open(os.path.join(d, name)).read()
        except OSError:
            return None
    print("RAN" + json.dumps([read("ran"), read("output")]))
    for name in ("ran", "output"):
        if os.path.exists(os.path.join(d, name)):
            os.remove(os.path.join(d, name))
else:
    mode, command = args[0][2:].strip().split(" ", 1)
    with open(os.path.join(d, "proposal"), "w") as f:
        f.write(mode + "\\n" + command)
PY
"""


@unittest.skipUnless(ZSH, "zsh not installed")
class ZshProposeTest(TempDirTest):
    """Propose mode in a real zsh: the proposal lands on the command line, and `//` gets what
    ran there. The fake ush proposes `//MODE COMMAND` and prints the records for `//`."""

    def setUp(self):
        super().setUp()
        self.bin = os.path.join(self.tmp, "bin")
        os.mkdir(self.bin)
        path = os.path.join(self.bin, "ush")
        with open(path, "w") as f:
            f.write(PROPOSER)
        os.chmod(path, 0o755)
        self.tmpdir = os.path.join(self.tmp, "t")
        os.mkdir(self.tmpdir)

    def session(self, steps):
        """steps: (keys to send, text to wait for). Returns the output after each step."""
        pid, fd = pty.fork()
        if pid == 0:
            os.environ.update({"PATH": self.bin + ":/usr/bin:/bin", "PS1": "PROMPT> ",
                               "HOME": self.tmp, "TMPDIR": self.tmpdir})
            os.execv(ZSH, [ZSH, "-f", "-i"])
        try:
            ZshPluginTest._read(fd, "PROMPT> ")
            os.write(fd, f"source {PLUGIN}\r".encode())
            ZshPluginTest._read(fd, "PROMPT> ")
            outputs = []
            for keys, until in steps:
                os.write(fd, keys.encode())
                outputs.append(ZshPluginTest._read(fd, until))
            os.write(fd, b"\x15exit\r")  # ^U clears the line first
            return outputs
        finally:
            os.close(fd)
            os.waitpid(pid, 0)

    @staticmethod
    def records(output):
        found = re.findall(r"RAN(\[.*?\])\r?\n", output)
        return json.loads(found[-1]) if found else output

    def test_proposal_is_prefilled_and_what_ran_is_recorded(self):
        out = self.session([
            ("// capture echo captured-$((6*7))\r", "PROMPT> echo captured-$((6*7))"),
            ("\r", "PROMPT> "),
            ("//\r", "PROMPT> "),
        ])
        self.assertIn("captured-42", out[1])  # it ran in the shell, on the terminal
        ran, output = self.records(out[2])
        self.assertEqual(ran, "0\necho captured-$((6*7))\n")
        self.assertEqual(output, "captured-42\n")
        self.assertEqual(os.listdir(self.tmpdir), [], "handoff directory left behind")

    def test_edited_failing_and_interactive_runs(self):
        out = self.session([
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x15sh -c 'echo two; exit 3'\r", "PROMPT> "),  # replaced: what ran counts
            ("//\r", "PROMPT> "),
            ("// interactive echo three\r", "PROMPT> echo three"),
            ("\r", "PROMPT> "),
            ("//\r", "PROMPT> "),
        ])
        ran, output = self.records(out[2])
        self.assertEqual((ran, output), ("3\nsh -c 'echo two; exit 3'\n", "two\n"))
        ran, output = self.records(out[5])
        self.assertEqual((ran, output), ("0\necho three\n", None))  # not captured

    def test_a_new_request_instead_is_not_a_run(self):
        out = self.session([
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x15//\r", "PROMPT> "),
        ])
        self.assertEqual(self.records(out[1]), [None, None])

    def test_esc_removes_the_proposal(self):
        out = self.session([
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x1b", "proposal removed"),
            ("echo other\r", "PROMPT> "),
            ("//\r", "PROMPT> "),
        ])
        self.assertNotIn("oneecho", out[2])  # the line was empty before typing
        self.assertIn("other", out[2])
        self.assertEqual(self.records(out[3]), [None, None])  # nothing counts as the proposal

    def test_esc_keeps_its_own_binding_outside_a_proposal(self):
        out = self.session([
            ("bindkey -M emacs '^[' backward-word\r", "PROMPT> "),
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\r", "PROMPT> "),                                   # ran it
            ("bindkey -M emacs '^['\r", "PROMPT> "),
            ("// capture echo two\r", "PROMPT> echo two"),
            ("\x1b", "proposal removed"),                          # removed it
            ("bindkey -M emacs '^['\r", "PROMPT> "),
            ("// capture echo three\r", "PROMPT> echo three"),
            ("\x03", "PROMPT> "),                                 # abandoned it (Ctrl-C)
            ("bindkey -M emacs '^['\r", "PROMPT> "),
        ])
        for i in (3, 6, 9):
            self.assertIn('"^[" backward-word', out[i])

    def test_down_explains_the_proposal_and_keeps_it(self):
        out = self.session([
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x1b[B", "PROMPT> echo one"),        # Down: explain, the proposal stays
            ("\r", "PROMPT> "),                    # then run it
            ("//\r", "PROMPT> "),
            ("\x1b[B", "PROMPT> "),                # Down without a proposal: history, no ush
            ("\x15bindkey -M main $'\\e[B'\r", "PROMPT> "),
        ])
        self.assertIn('EXPLAIN["--", "echo one"]', out[1])
        self.assertEqual(self.records(out[3])[0], "0\necho one\n")
        self.assertNotIn("EXPLAIN", out[4] + out[5])
        self.assertIn('"^[[B" down-line-or-history', out[5])

    def test_no_handoff_as_root_or_twice(self):
        with open(PLUGIN) as f:
            source = f.read()
        self.assertIn("(( EUID != 0 ))", source)


if __name__ == "__main__":
    unittest.main()
