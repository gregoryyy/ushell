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
        self.assertEqual(results[3], ["ush"])
        self.assertEqual(results[4], ["ush", "history"])

    def test_bare_slash_can_be_disabled(self):
        results = self.run_lines(["// hello"], env={"USH_BARE_SLASH": "0"})
        self.assertIsInstance(results[0], str)  # zsh ran "//" as a path, not ush
        self.assertNotIn("ARGS", results[0])


if __name__ == "__main__":
    unittest.main()
