"""shell/zshrc.sh, which both installers use to add the zsh plugin to ~/.zshrc."""

import os
import pty
import subprocess
import time

from helpers import TempDirTest

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "shell", "zshrc.sh")
PLUGIN = "/opt/ush/lib/ush.plugin.zsh"


class ZshrcTest(TempDirTest):
    def setUp(self):
        super().setUp()
        self.rc = os.path.join(self.tmp, ".zshrc")
        self.env = {"PATH": "/usr/bin:/bin", "HOME": self.tmp}

    def run_script(self, *args, env=None):
        return subprocess.run(["sh", SCRIPT, *args], capture_output=True, text=True,
                              env={**self.env, **(env or {})}, stdin=subprocess.DEVNULL,
                              timeout=10)

    def rc_text(self):
        try:
            with open(self.rc) as f:
                return f.read()
        except FileNotFoundError:
            return None

    def test_yes_adds_the_line_once(self):
        with open(self.rc, "w") as f:
            f.write("alias ll='ls -l'\n")
        first = self.run_script(PLUGIN, "yes")
        self.assertEqual(first.returncode, 0, first.stderr)
        self.assertIn("added the zsh plugin", first.stdout)
        second = self.run_script(PLUGIN, "yes")
        self.assertIn("already in", second.stdout)
        text = self.rc_text()
        self.assertTrue(text.startswith("alias ll='ls -l'\n"))
        self.assertEqual(text.count(f"source '{PLUGIN}'"), 1)

    def test_no_and_ask_without_a_terminal_only_print(self):
        for mode in ("no", "ask"):
            with self.subTest(mode=mode):
                result = self.run_script(PLUGIN, mode)
                self.assertEqual(result.returncode, 0)
                self.assertIn(f"source '{PLUGIN}'", result.stdout)
                self.assertIsNone(self.rc_text())

    def test_an_existing_plugin_line_is_left_alone(self):
        with open(self.rc, "w") as f:
            f.write("source ~/src/ushell/ushpy/shell/ush.plugin.zsh\n")
        result = self.run_script(PLUGIN, "yes")
        self.assertIn("already in", result.stdout)
        self.assertNotIn(PLUGIN, self.rc_text())

    def test_zdotdir_and_quotes(self):
        zdot = os.path.join(self.tmp, "zdot")
        os.mkdir(zdot)
        self.run_script(PLUGIN, "yes", env={"ZDOTDIR": zdot})
        self.assertIsNone(self.rc_text())
        with open(os.path.join(zdot, ".zshrc")) as f:
            self.assertIn(PLUGIN, f.read())
        bad = self.run_script("/tmp/it's/ush.plugin.zsh", "yes")
        self.assertEqual(bad.returncode, 2)
        self.assertIsNone(self.rc_text())

    def test_ask_in_a_terminal(self):
        for answer, added in (("\r", True), ("n\r", False)):
            with self.subTest(answer=answer):
                if os.path.exists(self.rc):
                    os.remove(self.rc)
                pid, fd = pty.fork()
                if pid == 0:
                    os.execve("/bin/sh", ["sh", SCRIPT, PLUGIN, "ask"], self.env)
                output, end = b"", time.time() + 10
                while b"[Y/n]" not in output and time.time() < end:
                    output += os.read(fd, 1024)
                os.write(fd, answer.encode())
                try:
                    while True:
                        chunk = os.read(fd, 1024)
                        if not chunk:
                            break
                        output += chunk
                except OSError:
                    pass
                os.close(fd)
                os.waitpid(pid, 0)
                self.assertEqual(PLUGIN in (self.rc_text() or ""), added, output)
