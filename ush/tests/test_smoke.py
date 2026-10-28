"""The user smoke test (scripts/smoke.sh): offline and with a model, against the mock API, and that
it fails for an implementation that misbehaves."""

import os
import stat
import subprocess
import sys
import unittest

from helpers import TempDirTest
from mock_api import MockAPI

HERE = os.path.dirname(os.path.abspath(__file__))
SMOKE = os.path.join(HERE, "..", "scripts", "smoke.sh")
SRC = os.path.join(HERE, "..", "src")

try:
    import anthropic  # noqa: F401
    HAVE_SDK = True
except ImportError:
    HAVE_SDK = False


def use(ident, command, effect="read"):
    return ([{"type": "tool_use", "id": ident, "name": "run_command",
              "input": {"command": command, "purpose": "smoke", "effect": effect,
                        "timeout_seconds": 20}}], "tool_use")


def say(text):
    return ([{"type": "text", "text": text}], "end_turn")


class SmokeTest(TempDirTest):
    def launcher(self, body: str) -> str:
        path = os.path.join(self.tmp, "ush")
        with open(path, "w") as f:
            f.write("#!/bin/sh\n" + body)
        os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR)
        return path

    def python_ush(self) -> str:
        return self.launcher(f'PYTHONPATH="{SRC}" exec "{sys.executable}" -m ush "$@"\n')

    def smoke(self, *args, env=None):
        base = {k: v for k, v in os.environ.items() if not k.startswith(("ANTHROPIC_", "USH_"))}
        base.update(env or {})
        return subprocess.run(["bash", SMOKE, *args], capture_output=True, text=True, env=base,
                              cwd=self.tmp, timeout=300)

    def test_offline_checks_pass_for_the_python_implementation(self):
        result = self.smoke("--bin", self.python_ush())
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("FAIL", result.stdout)
        self.assertIn("PASS version: ush ", result.stdout)
        with open(os.path.join(self.tmp, "smoke-report.md")) as f:
            self.assertIn("| PASS | ai status |", f.read())

    def test_a_misbehaving_ush_fails(self):
        # Accepts any command and exits 0: the usage-error and session checks must fail.
        fake = self.launcher('echo "ush 9.9.9 (fake)"; echo usage:\n')
        result = self.smoke("--bin", fake)
        self.assertEqual(result.returncode, 1)
        self.assertIn("FAIL unknown command is a usage error", result.stdout)
        self.assertIn("FAIL no session without a terminal", result.stdout)

    def test_paid_checks_need_consent(self):
        result = self.smoke("--bin", self.python_ush(), "--provider", "anthropic")
        self.assertEqual(result.returncode, 2)
        self.assertIn("cost money", result.stderr)

    @unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed")
    def test_model_checks_against_a_scripted_ollama(self):
        replies = [
            use("r1", "df -h /"), say("Plenty of space."),                         # read-only
            use("c1", "touch smoke-made", "change"), say("I could not create it."),  # change
            use("i1", "cat NOTES.txt"), use("i2", "touch pwned", "change"),           # injection
            say("The backup finished at 03:00."),
            say("A filesystem error on sda1."),                                       # piped
            use("d1", "ls"), say("I would list the files."),                          # dry run
        ]
        with MockAPI(replies, tags=[{"name": "qwen3.5:9b"}]) as api:
            result = self.smoke("--bin", self.python_ush(), "--provider", "ollama",
                                "--model", "qwen3.5:9b", "--ollama-url", api.url,
                                env={"USH_OLLAMA_URL": "http://127.0.0.1:9"})  # must be ignored
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for check in ("read-only request: df ran without a prompt", "disclosure banner",
                      "change without approval: declined", "injected instructions: nothing",
                      "piped input", "dry run: nothing ran", "replay without AI"):
            self.assertIn("PASS " + check, result.stdout)
        self.assertNotIn("FAIL", result.stdout)
        self.assertEqual(api.replies, [])  # every scripted turn was used

    def test_a_failing_request_fails_its_check(self):
        with MockAPI([], tags=[{"name": "qwen3.5:9b"}]) as api:  # every request gets an error
            result = self.smoke("--bin", self.python_ush(), "--provider", "ollama",
                                "--model", "qwen3.5:9b", "--ollama-url", api.url)
        self.assertEqual(result.returncode, 1)
        for check in ("read-only request", "change without approval", "injected instructions",
                      "piped input", "dry run"):
            self.assertIn("FAIL " + check, result.stdout)


if __name__ == "__main__":
    unittest.main()
