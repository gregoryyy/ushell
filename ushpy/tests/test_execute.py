import _thread
import os
import threading
import time
import unittest
from unittest import mock

from helpers import SYNTHETIC_KEY, TempDirTest
from ush import execute


class ExecuteTest(TempDirTest):
    def test_output_status_and_pipefail(self):
        result = execute.run_command("echo out; echo err >&2; exit 3", timeout=10, limit=1000)
        self.assertEqual(result.exit_code, 3)
        self.assertIn("out", result.output)
        self.assertIn("err", result.output)
        self.assertNotEqual(execute.run_command("false | cat", timeout=10, limit=100).exit_code, 0)

    def test_stdin_is_empty(self):
        result = execute.run_command("cat; echo done", timeout=10, limit=100)
        self.assertEqual(result.output.strip(), "done")

    def test_output_is_bounded_keeping_head_and_tail(self):
        result = execute.run_command("seq 1 200000", timeout=20, limit=2000)
        self.assertTrue(result.truncated)
        self.assertLess(len(result.output), 2200)
        self.assertTrue(result.output.startswith("1\n2\n"))
        self.assertIn("200000", result.output)
        self.assertIn("omitted", result.output)

    def test_timeout_stops_the_whole_pipeline(self):
        marker = os.path.join(self.tmp, "late")
        start = time.monotonic()
        result = execute.run_command(f"sleep 30 | cat; touch {marker}", timeout=1, limit=100)
        self.assertTrue(result.timed_out)
        self.assertLess(time.monotonic() - start, 10)
        time.sleep(0.5)
        self.assertFalse(os.path.exists(marker))

    def test_interrupt_cancels_and_kills(self):
        marker = os.path.join(self.tmp, "late")
        threading.Timer(0.5, _thread.interrupt_main).start()
        with self.assertRaises(execute.Cancelled):
            execute.run_command(f"sleep 5; touch {marker}", timeout=30, limit=100)
        time.sleep(0.5)
        self.assertFalse(os.path.exists(marker))

    def test_provider_credentials_are_not_passed_to_commands(self):
        env = {"PATH": os.environ["PATH"], "ANTHROPIC_API_KEY": SYNTHETIC_KEY,
               "USH_API_KEY": SYNTHETIC_KEY, "ANTHROPIC_AUTH_TOKEN": SYNTHETIC_KEY, "KEEP": "yes"}
        with mock.patch.dict(os.environ, env, clear=True):
            result = execute.run_command("env", timeout=10, limit=10000)
        self.assertNotIn(SYNTHETIC_KEY, result.output)
        self.assertIn("KEEP=yes", result.output)
        self.assertIn("PAGER=cat", result.output)

    def test_user_rc_files_do_not_change_commands(self):
        with open(os.path.join(self.tmp, ".zshenv"), "w") as f:
            f.write("alias ls='echo hijacked'\nls() { echo hijacked; }\n")
        with mock.patch.dict(os.environ, {"ZDOTDIR": self.tmp, "HOME": self.tmp}):
            result = execute.run_command(f"ls {self.tmp}", timeout=10, limit=1000)
        self.assertNotIn("hijacked", result.output)

    def test_scripts_run_in_a_private_temporary_file(self):
        result = execute.run_script("sh", 'echo "$0"; ls -l "$0"', timeout=10, limit=1000)
        self.assertEqual(result.exit_code, 0)
        path = result.output.splitlines()[0]
        self.assertIn("-rw-------", result.output)
        self.assertFalse(os.path.exists(path))
        py = execute.run_script("python3", "import sys; print(sys.flags.isolated)", timeout=20,
                                limit=100)
        self.assertEqual(py.output.strip(), "1")
        with self.assertRaises(ValueError):
            execute.run_script("perl", "print 1", timeout=5, limit=10)


if __name__ == "__main__":
    unittest.main()
