"""The Python installer, ush/py/install.sh, and shell/zshrc.sh, which both installers use to add the
zsh plugin to ~/.zshrc. The Go installer is tested with the release archive (tools/release)."""

import os
import pty
import subprocess
import time
import unittest

from helpers import TempDirTest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "..", "..", "shell", "zshrc.sh")
INSTALLER = os.path.join(HERE, "..", "install.sh")
MANUAL_PAGE = os.path.join(HERE, "..", "..", "man", "ush.1")
# Stands in for Python in install.sh: makes a venv whose pip installs nothing (no network) and
# whose python names a site directory, so the test sees where the installer puts each file.
FAKE_PYTHON = """#!/bin/sh
case "$1" in
    -c) exit 0 ;;
    -m) dir=$4; mkdir -p "$dir/bin" "$dir/site"
        printf '#!/bin/sh\\nexit 0\\n' > "$dir/bin/pip"
        printf '#!/bin/sh\\necho %s/site\\n' "$dir" > "$dir/bin/python"
        chmod 755 "$dir/bin/pip" "$dir/bin/python" ;;
esac
"""
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


class InstallerTest(TempDirTest):
    def install(self, *args):
        python = os.path.join(self.tmp, "python3")
        with open(python, "w") as f:
            f.write(FAKE_PYTHON)
        os.chmod(python, 0o755)
        env = {"PATH": "/usr/bin:/bin", "HOME": self.tmp, "PYTHON": python}
        return subprocess.run(["sh", INSTALLER, "--no-zshrc", *args], capture_output=True,
                              text=True, env=env, stdin=subprocess.DEVNULL, timeout=60)

    def assert_installed(self, prefix, lib):
        with open(MANUAL_PAGE, "rb") as f:
            page = f.read()
        for path in ("bin/ush", f"{lib}/ush.plugin.zsh", f"{lib}/venv/site/ush/cli.py",
                     "share/man/man1/ush.1"):
            with self.subTest(path=path):
                info = os.stat(os.path.join(prefix, path))
                self.assertEqual(info.st_mode & 0o022, 0, f"{path} is writable by others")
        with open(os.path.join(prefix, "share/man/man1/ush.1"), "rb") as f:
            self.assertEqual(f.read(), page)

    def test_for_the_user_the_manual_page_goes_to_share_man(self):
        result = self.install()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_installed(os.path.join(self.tmp, ".local"), "share/ush")

    def test_with_a_prefix_the_manual_page_goes_to_prefix_share_man(self):
        prefix = os.path.join(self.tmp, "opt")
        result = self.install("--prefix", prefix)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_installed(prefix, "lib/ush")


if __name__ == "__main__":
    unittest.main()
