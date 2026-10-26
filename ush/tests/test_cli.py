"""End-to-end CLI runs without a terminal, against the mock API."""

import json
import os
import subprocess
import sys
import unittest

from helpers import SYNTHETIC_KEY, TempDirTest
from mock_api import MockAPI

SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "src")

try:
    import anthropic  # noqa: F401
    HAVE_SDK = True
except ImportError:
    HAVE_SDK = False


class CliTest(TempDirTest):
    def ush(self, *args, stdin="", env=None):
        base = {k: v for k, v in os.environ.items()
                if not k.startswith(("ANTHROPIC_", "USH_", "XDG_"))}
        base.update({"PYTHONPATH": SRC, "XDG_CONFIG_HOME": os.path.join(self.tmp, "config"),
                     "XDG_STATE_HOME": os.path.join(self.tmp, "state"), "NO_COLOR": "1"})
        base.update(env or {})
        # A new session has no controlling terminal, as in CI or a cron job.
        return subprocess.run([sys.executable, "-m", "ush", *args], input=stdin, text=True,
                              capture_output=True, env=base, cwd=self.tmp, timeout=60,
                              start_new_session=True)

    def test_help_and_usage_errors(self):
        self.assertIn("usage:", self.ush("help").stdout)
        result = self.ush("frobnicate")
        self.assertEqual(result.returncode, 2)
        self.assertIn("requests start with //", result.stderr)
        self.assertEqual(self.ush("-e", "ultra", "//x").returncode, 2)
        self.assertEqual(self.ush().returncode, 2)  # no terminal for a session

    def test_missing_key_fails_before_anything_runs(self):
        result = self.ush("//why is my disk full")
        self.assertEqual(result.returncode, 2)
        self.assertIn("no API key", result.stderr)
        self.assertEqual(self.ush("history").stdout, "")

    def test_ai_status_never_prints_the_key(self):
        result = self.ush("ai", "status", env={"ANTHROPIC_API_KEY": SYNTHETIC_KEY})
        self.assertIn("environment (ANTHROPIC_API_KEY)", result.stdout)
        self.assertNotIn(SYNTHETIC_KEY, result.stdout + result.stderr)

    @unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed")
    def test_request_with_piped_input_history_and_replay(self):
        victim = os.path.join(self.tmp, "victim")
        replies = [
            ([{"type": "tool_use", "id": "t1", "name": "run_command",
               "input": {"command": "echo checked-$((1+1))-ok | cat", "purpose": "probe",
                         "effect": "read", "timeout_seconds": 20}},
              {"type": "tool_use", "id": "t2", "name": "run_command",
               "input": {"command": "echo fine", "purpose": "probe", "effect": "read",
                         "timeout_seconds": 20}},
              {"type": "tool_use", "id": "t3", "name": "run_command",
               "input": {"command": f"touch {victim}", "purpose": "fix", "effect": "change",
                         "timeout_seconds": 20}}], "tool_use"),
            ([{"type": "text", "text": "The log shows a disk error."}], "end_turn"),
        ]
        env = {"ANTHROPIC_API_KEY": SYNTHETIC_KEY}
        with MockAPI(replies) as api:
            env["USH_BASE_URL"] = api.url
            result = self.ush("//what does this log mean?", stdin="kernel: disk0 I/O error\n",
                              env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("The log shows a disk error.", result.stdout)
        self.assertFalse(os.path.exists(victim))
        first = json.dumps(api.requests[0]["body"]["messages"])
        self.assertIn("disk0 I/O error", first)
        results = api.requests[1]["body"]["messages"][2]["content"]
        self.assertIn("approval", results[0]["content"])  # $((...)) is not auto-approved
        self.assertIn("fine", results[1]["content"])
        self.assertIn("approval", results[2]["content"])

        listing = self.ush("history").stdout
        self.assertIn("// what does this log mean?", listing)
        self.assertIn("auto      $ echo fine", listing)
        self.assertIn("declined  $ touch", listing)

        replay = self.ush("replay", "last")
        self.assertEqual(replay.returncode, 0, replay.stderr)
        self.assertIn("fine", replay.stderr)
        self.assertFalse(os.path.exists(victim))
        with open(os.path.join(self.tmp, "state", "ush", "history.jsonl")) as f:
            self.assertNotIn(SYNTHETIC_KEY, f.read())


    @unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed")
    def test_terminal_approval_and_follow_up(self):
        import pty
        import select
        import time
        made = os.path.join(self.tmp, "made")
        replies = [
            ([{"type": "tool_use", "id": "t1", "name": "run_command",
               "input": {"command": f"touch {made}", "purpose": "create it", "effect": "change",
                         "timeout_seconds": 20}}], "tool_use"),
            ([{"type": "text", "text": "Created the file."}], "end_turn"),
        ]
        with MockAPI(replies) as api:
            pid, fd = pty.fork()
            if pid == 0:
                env = {k: v for k, v in os.environ.items() if not k.startswith(("ANTHROPIC_", "USH_"))}
                env.update({"PYTHONPATH": SRC, "ANTHROPIC_API_KEY": SYNTHETIC_KEY,
                            "USH_BASE_URL": api.url, "NO_COLOR": "1",
                            "XDG_CONFIG_HOME": os.path.join(self.tmp, "config"),
                            "XDG_STATE_HOME": os.path.join(self.tmp, "state")})
                os.chdir(self.tmp)
                os.execve(sys.executable, [sys.executable, "-m", "ush", "//make a file"], env)
            output = ""

            def until(text, timeout=20):
                nonlocal output
                end = time.time() + timeout
                while time.time() < end and text not in output:
                    if select.select([fd], [], [], 0.1)[0]:
                        try:
                            output += os.read(fd, 4096).decode(errors="replace")
                        except OSError:
                            break
                self.assertIn(text, output)

            try:
                until("run it?")
                self.assertIn("needs approval: touch is not a known read-only command", output)
                os.write(fd, b"y\r")
                until("ush> ")
                self.assertIn("Created the file.", output)
                os.write(fd, b"\r")  # an empty line ends the follow-up
                _, status = os.waitpid(pid, 0)
            finally:
                os.close(fd)
        self.assertTrue(os.path.exists(made))
        self.assertEqual(os.waitstatus_to_exitcode(status), 0)


if __name__ == "__main__":
    unittest.main()
