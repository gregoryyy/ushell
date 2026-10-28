"""The zsh plugin, driven through a pseudo-terminal with fake `ush` and `sudo` commands."""

import json
import os
import pty
import re
import select
import shutil
import sys
import time
import unittest

from helpers import TempDirTest, ush_argv

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
            os.environ.update({"PATH": self.bin + ":/usr/bin:/bin", "PS1": "PROMPT> ", "TERM": "xterm-256color",
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
        """Output until `until` has appeared and the terminal has been quiet for 0.5 s, which is
        also longer than zsh's KEYTIMEOUT, so a following key isn't read as Alt-key."""
        out, end = "", time.time() + timeout
        seen = time.time() if not until else None  # "": just until the terminal is quiet
        while time.time() < end:
            ready, _, _ = select.select([fd], [], [], 0.1)
            if ready:
                try:
                    chunk = os.read(fd, 4096)
                except OSError:
                    break
                if not chunk:  # the process ended
                    break
                out += chunk.decode(errors="replace")
                plain = re.sub(r"\x1b\[[0-9;?]*[A-Za-z]|\r", "", out)  # e.g. bracketed paste
                seen = time.time() if until.rstrip() in plain else None
            elif seen and time.time() - seen > 0.5:
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
            os.environ.update({"PATH": self.bin + ":/usr/bin:/bin", "PS1": "PROMPT> ", "TERM": "xterm-256color",
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

    def test_down_explains_above_the_command_and_esc_closes_it(self):
        out = self.session([
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x1b[B", "EXPLAIN"),                 # Down: the explanation, above the command
            ("\x0c", "echo one"),                   # Ctrl-L redraws all of it, in order
            (" two", "two"),                         # the cursor is still on the line
            ("\x1b", "echo one two"),               # Esc closes the explanation only
            ("\r", "PROMPT> "),                     # Enter runs the edited command
            ("//\r", "PROMPT> "),
            ("\x1b[B", ""),                         # Down without a proposal: no ush
            ("\x15bindkey -M main $'\\e[B'\r", "PROMPT> "),
        ])
        self.assertIn('EXPLAIN["--inline", "--", "echo one"]', out[1])
        plain = re.sub(r"\x1b\[[0-9;?]*[A-Za-z]|\r", "", out[2])
        self.assertLess(plain.index("PROMPT> "), plain.index("EXPLAIN"))
        self.assertLess(plain.index("EXPLAIN"), plain.rindex("→ echo one"))  # above the command
        self.assertNotIn("proposal removed", out[4])
        self.assertIn("one two", out[5])
        self.assertEqual(self.records(out[6])[0], "0\necho one two\n")
        self.assertNotIn("EXPLAIN", out[7] + out[8])
        self.assertIn('"^[[B" down-line-or-history', out[8])

    def test_up_closes_the_explanation_and_is_history_otherwise(self):
        out = self.session([
            ("echo earlier\r", "PROMPT> "),
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x1b[B", "EXPLAIN"),
            ("\x1b[A", "echo one"),   # Up: closes the explanation, the proposal stays
            ("\r", "PROMPT> "),
            ("//\r", "PROMPT> "),
            ("\x1b[A", "//"),          # Up without a proposal: history
            ("\x15bindkey -M main $'\\e[A'\r", "PROMPT> "),
        ])
        self.assertNotIn("EXPLAIN", out[3])
        self.assertEqual(self.records(out[5])[0], "0\necho one\n")
        self.assertIn('"^[[A" up-line-or-history', out[7])

    def test_second_esc_removes_the_explained_proposal(self):
        out = self.session([
            ("// capture echo one\r", "PROMPT> echo one"),
            ("\x1b[B", "EXPLAIN"),
            ("\x1b", "echo one"),
            ("\x1b", "proposal removed"),
            ("//\r", "PROMPT> "),
        ])
        self.assertIn("proposal removed", out[3])
        self.assertEqual(self.records(out[4]), [None, None])

    def test_no_handoff_as_root_or_twice(self):
        with open(PLUGIN) as f:
            source = f.read()
        self.assertIn("(( EUID != 0 ))", source)


EDITOR = """
import sys
from ush.ui import Terminal
print("GOT" + repr(Terminal().read_line("ush> ", prefill="echo prefilled")) + "END", flush=True)
"""


@unittest.skipUnless(ZSH, "zsh not installed")
class SessionPrefillTest(TempDirTest):
    """A session's proposal is pre-filled with zsh's vared (Python's readline can't with
    libedit), with the plugin's keys."""

    def edit(self, keys):
        src = os.path.join(os.path.dirname(PLUGIN), "..", "src")
        pid, fd = pty.fork()
        if pid == 0:
            os.environ.update({"PYTHONPATH": src, "HOME": self.tmp, "NO_COLOR": "1",
                               "TERM": "xterm-256color",
                               "XDG_CONFIG_HOME": os.path.join(self.tmp, "config"),
                               "XDG_STATE_HOME": os.path.join(self.tmp, "state"),
                               # a local model that isn't there: no key, no cost
                               "USH_PROVIDER": "ollama", "USH_MODEL": "qwen3.5:4b",
                               "USH_BASE_URL": "http://127.0.0.1:9",
                               "USH_OLLAMA_URL": "http://127.0.0.1:9"})
            os.execv(sys.executable, [sys.executable, "-c", EDITOR])
        try:
            outputs = [ZshPluginTest._read(fd, "ush> echo prefilled")]
            for key, until in keys:
                os.write(fd, key.encode())
                outputs.append(ZshPluginTest._read(fd, until, timeout=20))
            return outputs
        finally:
            os.close(fd)
            os.waitpid(pid, 0)

    def test_prefilled_and_editable(self):
        out = self.edit([(" more\r", "END")])
        self.assertIn("ush> echo prefilled", out[0])
        self.assertIn("GOT'echo prefilled more'", out[1])

    def test_esc_clears_the_line(self):
        out = self.edit([("\x1b", ""), ("ls\r", "END")])
        self.assertIn("GOT'ls'", out[2])

    def test_up_closes_the_explanation(self):
        out = self.edit([("\x1b[B", "cannot reach Ollama"), ("\x1b[A", "echo prefilled"),
                         ("\r", "END")])
        self.assertIn("GOT'echo prefilled'", out[3])

    def test_down_explains_and_esc_closes_the_explanation(self):
        out = self.edit([("\x1b[B", "cannot reach Ollama"), ("\x0c", "echo prefilled"),
                         ("\x1b", "echo prefilled"), ("\r", "END")])
        plain = re.sub(r"\x1b\[[0-9;?]*[A-Za-z]|\r", "", out[2])  # after Ctrl-L: in order
        self.assertLess(plain.index("cannot reach Ollama"), plain.rindex("→ echo prefilled"))
        self.assertNotIn("ush> ", plain)  # the explanation takes the prompt's place
        self.assertIn("GOT'echo prefilled'", out[4])


@unittest.skipUnless(ZSH, "zsh not installed")
class SessionTest(TempDirTest):
    """The `ush>` session reads every line with zsh's line editor: arrow keys work, Up recalls
    earlier lines (before, Python's plain input() echoed ^[[A), Ctrl-D ends it."""

    def test_up_recalls_earlier_lines(self):
        src = os.path.join(os.path.dirname(PLUGIN), "..", "src")
        pid, fd = pty.fork()
        if pid == 0:
            os.environ.update({"PYTHONPATH": src, "HOME": self.tmp, "NO_COLOR": "1",
                               "TERM": "xterm-256color",
                               "XDG_CONFIG_HOME": os.path.join(self.tmp, "config"),
                               "XDG_STATE_HOME": os.path.join(self.tmp, "state"),
                               "USH_BASE_URL": "http://127.0.0.1:9"})
            os.chdir(self.tmp)
            argv = ush_argv()
            os.execv(argv[0], argv)
        try:
            read = ZshPluginTest._read
            read(fd, "ush> ")
            os.write(fd, b"echo first-$((40+2))\r")
            read(fd, "ush> ")
            os.write(fd, b"\x1b[A")
            recalled = read(fd, "first")
            os.write(fd, b"\r")
            again = read(fd, "ush> ")
            os.write(fd, b"\x04")  # Ctrl-D on an empty line ends the session
            end = read(fd, "")
        finally:
            os.close(fd)
            _, status = os.waitpid(pid, 0)
        self.assertNotIn("^[[A", recalled)
        self.assertIn("echo first-$((40+2))", recalled)
        self.assertIn("first-42", again)
        self.assertEqual(os.waitstatus_to_exitcode(status), 0, end)


if __name__ == "__main__":
    unittest.main()
