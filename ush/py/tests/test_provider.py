"""The real Anthropic SDK against a local mock server: request shape and streaming."""

import json
import os
import unittest

from helpers import SYNTHETIC_KEY, FakeTerminal, TempDirTest, fake_identity
from mock_api import MockAPI

try:
    import anthropic  # noqa: F401
    HAVE_SDK = True
except ImportError:
    HAVE_SDK = False

from ush.agent import Agent
from ush.config import Settings
from ush.history import History
from unittest import mock

from ush.agent import PROPOSE_TOOL, TOOLS
from ush.provider import (FALLBACK_BETA, UPDATES_BETA, AnthropicProvider, HeldText,
                          OllamaProvider, ProviderError, ollama_remote_host, text_tool_call)

COMMAND = {"command": "echo mock-output", "purpose": "check", "effect": "read",
           "timeout_seconds": 20}


@unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed (pip install -r requirements.lock)")
class ProviderTest(TempDirTest):
    def provider(self, url, model="claude-opus-5-5"):
        return AnthropicProvider(SYNTHETIC_KEY, model, "medium", base_url=url, max_retries=0)

    def test_tool_round_trip_through_the_sdk(self):
        replies = [
            ([{"type": "thinking", "thinking": "Checking free space"},
              {"type": "tool_use", "id": "toolu_1", "name": "run_command", "input": COMMAND}],
             "tool_use"),
            ([{"type": "text", "text": "All good."}], "end_turn"),
        ]
        with MockAPI(replies) as api:
            ui = FakeTerminal()
            agent = Agent(self.provider(api.url), ui, Settings(show_lines=0),
                          fake_identity(self.tmp), History(os.path.join(self.tmp, "s")),
                          secrets=(SYNTHETIC_KEY,))
            entry = agent.ask("how is my disk?")
        self.assertEqual(entry.outcome, "answered")
        self.assertIn("All good.", ui.out.getvalue())
        self.assertIn("Checking free space", ui.err.getvalue())

        first, second = api.requests
        self.assertEqual(first["path"], "/v1/messages")
        headers = {k.lower(): v for k, v in first["headers"].items()}
        # SDK 2.x (API version 2026-10-21) sends API keys as a bearer token.
        self.assertEqual(headers["authorization"], "Bearer " + SYNTHETIC_KEY)
        self.assertIn(FALLBACK_BETA, headers["anthropic-beta"])
        self.assertIn(UPDATES_BETA, headers["anthropic-beta"])
        body = first["body"]
        self.assertEqual(body["model"], "claude-opus-5-5")
        self.assertTrue(body["stream"])
        self.assertEqual(body["fallbacks"], "default")
        self.assertEqual(body["thinking"], {"type": "adaptive", "display": "updates"})
        self.assertEqual(body["output_config"], {"effort": "medium"})
        self.assertEqual(body["cache_control"], {"type": "ephemeral"})
        for tool in body["tools"]:
            self.assertTrue(tool["strict"])
            self.assertTrue(tool["eager_input_streaming"])
        for request in api.requests:
            self.assertNotIn(SYNTHETIC_KEY, json.dumps(request["body"]))

        # The assistant turn is echoed unchanged (thinking signature included).
        assistant = second["body"]["messages"][1]
        self.assertEqual(assistant["role"], "assistant")
        self.assertEqual(assistant["content"][0]["signature"], "sig-test")
        self.assertEqual(assistant["content"][1]["input"], COMMAND)
        result = second["body"]["messages"][2]["content"][0]
        self.assertEqual(result["tool_use_id"], "toolu_1")
        self.assertIn("mock-output", result["content"])

    def test_other_models_get_no_unsupported_options(self):
        with MockAPI([([{"type": "text", "text": "hi"}], "end_turn")]) as api:
            self.provider(api.url, model="claude-haiku-5-5").turn(
                "sys", [], [{"role": "user", "content": "x"}], lambda t: None, lambda t: None)
        body = api.requests[0]["body"]
        self.assertNotIn("fallbacks", body)
        self.assertEqual(body["thinking"], {"type": "adaptive"})
        self.assertNotIn("anthropic-beta", {k.lower() for k in api.requests[0]["headers"]})

    def test_errors_become_actionable_messages(self):
        cases = [(401, "authentication_error", "rejected"), (404, "not_found_error", "unknown model"),
                 (429, "rate_limit_error", "rate limited"), (400, "invalid_request_error", "rejected")]
        for status, kind, expected in cases:
            with self.subTest(status=status), MockAPI([(status, {"type": kind})]) as api:
                with self.assertRaisesRegex(ProviderError, expected):
                    self.provider(api.url).turn("s", [], [{"role": "user", "content": "x"}],
                                                lambda t: None, lambda t: None)

    def test_refusal_is_reported(self):
        with MockAPI([([{"type": "text", "text": ""}], "refusal")]) as api:
            turn = self.provider(api.url).turn("s", [], [{"role": "user", "content": "x"}],
                                               lambda t: None, lambda t: None)
        self.assertEqual(turn.stop_reason, "refusal")



def ask(provider):
    return provider.turn("sys", TOOLS, [{"role": "user", "content": "x"}],
                         lambda t: None, lambda t: None)


@unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed (pip install -r requirements.lock)")
class OllamaProviderTest(TempDirTest):
    def test_requests_carry_no_anthropic_options_and_no_real_key(self):
        reply = ([{"type": "tool_use", "id": "call_1", "name": "run_command", "input": COMMAND}],
                 "tool_use")
        # A key or a proxy in the environment must not reach a local Ollama request.
        env = {"ANTHROPIC_API_KEY": SYNTHETIC_KEY, "HTTP_PROXY": "http://127.0.0.1:9",
               "HTTPS_PROXY": "http://127.0.0.1:9", "ALL_PROXY": "http://127.0.0.1:9"}
        with mock.patch.dict(os.environ, env), MockAPI([reply]) as api:
            turn = ask(OllamaProvider("qwen3:8b", api.url, max_retries=0))
        self.assertEqual(turn.content[0]["input"], COMMAND)
        (request,) = api.requests
        self.assertEqual(request["path"], "/v1/messages")
        body = request["body"]
        self.assertEqual(body["model"], "qwen3:8b")
        self.assertEqual({t["name"] for t in body["tools"]}, {"run_command", "run_script"})
        for option in ("fallbacks", "thinking", "output_config", "cache_control"):
            self.assertNotIn(option, body)
        headers = {k.lower(): v for k, v in request["headers"].items()}
        self.assertNotIn("anthropic-beta", headers)
        self.assertNotIn(SYNTHETIC_KEY, json.dumps(request))

    def test_errors_name_the_ollama_fix(self):
        cases = [((404, {"type": "not_found_error"}), "ollama pull qwen3:8b"),
                 ((410, {"message": "model retired"}), "Ollama error 410: model retired"),
                 ((401, {"type": "authentication_error"}), "ollama signin")]
        for reply, expected in cases:
            with self.subTest(expected=expected), MockAPI([reply]) as api:
                with self.assertRaisesRegex(ProviderError, expected):
                    ask(OllamaProvider("qwen3:8b", api.url, max_retries=0))
        with MockAPI([]) as api:
            closed = api.url
        with self.assertRaisesRegex(ProviderError, "cannot reach Ollama at .*ollama serve"):
            ask(OllamaProvider("qwen3:8b", closed, max_retries=0))


class TextToolCallTest(unittest.TestCase):
    """Tool calls small models write as text (llama3.2:3b, 2026-10-29: `{"name": "ls",
    "parameters": {"command": "-d", …, "interactive": "false"}}` for "list the current directory")."""

    PROPOSAL = {"command": "ls -bA", "purpose": "list", "effect": "read", "interactive": False}

    def call(self, text, tools=(PROPOSE_TOOL,)):
        return text_tool_call(text, list(tools))

    def test_a_named_tool_is_taken_as_written(self):
        self.assertEqual(self.call(json.dumps({"name": "propose_command",
                                               "parameters": self.PROPOSAL})),
                         ("propose_command", self.PROPOSAL))
        self.assertEqual(self.call(json.dumps({"name": "run_command", "arguments": COMMAND}),
                                   TOOLS), ("run_command", COMMAND))

    def test_a_program_as_the_name_is_the_tool_its_input_fits(self):
        written = '{"name": "ls", "parameters": {"command": "-d", "purpose": "list", ' \
                  '"effect": "read", "interactive": "false"}}'
        self.assertEqual(self.call(written), ("propose_command", {
            "command": "ls -d", "purpose": "list", "effect": "read", "interactive": False}))
        # The program already leads the command: nothing is added.
        self.assertEqual(self.call(json.dumps({"name": "ls", "parameters": self.PROPOSAL})),
                         ("propose_command", self.PROPOSAL))
        # In agent mode, the input fits run_command and not run_script.
        self.assertEqual(self.call(json.dumps({"name": "wc", "parameters": {**COMMAND,
                                                                            "command": "-l x"}}),
                                   TOOLS), ("run_command", {**COMMAND, "command": "wc -l x"}))

    def test_a_name_that_is_no_program_is_not_put_in_front(self):
        made_up = "no-such-program-ush-test"
        self.assertEqual(self.call(json.dumps({"name": made_up, "parameters": self.PROPOSAL})),
                         ("propose_command", self.PROPOSAL))
        self.assertEqual(self.call(json.dumps({"name": "list files", "parameters": self.PROPOSAL})),
                         ("propose_command", self.PROPOSAL))

    def test_input_that_fits_no_tool_keeps_its_name_for_the_agent_to_refuse(self):
        partial = {"command": "-d", "purpose": "list"}
        self.assertEqual(self.call(json.dumps({"name": "ls", "parameters": partial})),
                         ("ls", partial))
        extra = {**self.PROPOSAL, "shell": "zsh"}
        self.assertEqual(self.call(json.dumps({"name": "ls", "parameters": extra})), ("ls", extra))

    def test_fences_and_the_parameters_slip(self):
        body = json.dumps({"name": "propose_command", "parameters": self.PROPOSAL})
        expected = ("propose_command", self.PROPOSAL)
        self.assertEqual(self.call(f"```json\n{body}\n```"), expected)
        self.assertEqual(self.call(f"  ```\n{body}```\n"), expected)
        slip = body.replace('"parameters": {', '"parameters{')
        self.assertEqual(self.call(slip), expected)

    def test_other_text_is_no_call(self):
        for text in ("Run `ls -d` to list it.", '{"command": "ls"}', '{"name": "ls"}',
                     '{"name": "ls", "parameters": "ls -d"}',
                     '{"name": "ls", "parameters": {}, "extra": 1}', "[1, 2]", "{not json",
                     'Here: {"name": "ls", "parameters": {}}', '{"name": 3, "parameters": {}}'):
            with self.subTest(text=text):
                self.assertIsNone(self.call(text))


class HeldTextTest(unittest.TestCase):
    def test_prose_streams_and_a_possible_call_waits_for_the_end(self):
        out = []
        held = HeldText(out.append)
        for chunk in ("  ", "Run ", "`ls`."):
            held.write(chunk)
        self.assertEqual(out, ["  Run ", "`ls`."])
        out.clear()
        for opening in ("{", "`"):
            held = HeldText(out.append)
            held.write(" " + opening)
            held.write('"name"')
            self.assertEqual(out, [])
            held.flush()
            self.assertEqual(out, [" " + opening + '"name"'])
            out.clear()


@unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed (pip install -r requirements.lock)")
class OllamaTextToolCallTest(TempDirTest):
    WRITTEN = ('{"name": "ls", "parameters": {"command": "-d", "purpose": "list", '
               '"effect": "read", "interactive": "false"}}')

    def turn(self, provider, tools=(PROPOSE_TOOL,)):
        shown = []
        turn = provider.turn("sys", list(tools), [{"role": "user", "content": "x"}],
                             shown.append, lambda t: None)
        return turn, "".join(shown)

    def test_ollama_turns_a_written_call_into_a_tool_use_and_shows_nothing(self):
        with MockAPI([([{"type": "text", "text": self.WRITTEN}], "end_turn")]) as api:
            turn, shown = self.turn(OllamaProvider("llama3.2:3b", api.url, max_retries=0))
        self.assertEqual(shown, "")
        self.assertEqual(turn.stop_reason, "tool_use")
        (block,) = turn.content
        self.assertEqual((block["type"], block["name"], block["input"]["command"],
                          block["input"]["interactive"]),
                         ("tool_use", "propose_command", "ls -d", False))
        self.assertRegex(block["id"], r"^toolu_text_[0-9a-f]{24}$")

    def test_prose_and_a_real_call_are_unchanged(self):
        use = {"type": "tool_use", "id": "call_1", "name": "run_command", "input": COMMAND}
        both = [{"type": "text", "text": self.WRITTEN}, use]
        with MockAPI([([{"type": "text", "text": "It is empty."}], "end_turn"),
                      (both, "tool_use")]) as api:
            provider = OllamaProvider("llama3.2:3b", api.url, max_retries=0)
            turn, shown = self.turn(provider)
            self.assertEqual((shown, turn.content[0]["text"]), ("It is empty.", "It is empty."))
            turn, shown = self.turn(provider, TOOLS)
            self.assertEqual(shown, self.WRITTEN)
            self.assertEqual([b["type"] for b in turn.content], ["text", "tool_use"])

    def test_anthropic_text_stays_text(self):
        with MockAPI([([{"type": "text", "text": self.WRITTEN}], "end_turn")]) as api:
            provider = AnthropicProvider(SYNTHETIC_KEY, "claude-opus-5-5", "medium",
                                         base_url=api.url, max_retries=0)
            turn, shown = self.turn(provider)
        self.assertEqual((shown, turn.stop_reason), (self.WRITTEN, "end_turn"))


class OllamaRemoteHostTest(TempDirTest):
    TAGS = [{"name": "llama3.2:latest"}, {"name": "qwen3:8b"},
            {"name": "gemma4:31b-cloud", "remote_host": "https://ollama.com:443"},
            {"name": "renamed:latest", "remote_host": "https://ollama.com:443"}]

    def test_cloud_models_are_never_reported_as_local(self):
        with MockAPI([], tags=self.TAGS) as api:
            self.assertIsNone(ollama_remote_host(api.url, "qwen3:8b"))
            self.assertIsNone(ollama_remote_host(api.url, "llama3.2"))
            self.assertEqual(ollama_remote_host(api.url, "gemma4:31b-cloud"),
                             "https://ollama.com:443")
            self.assertEqual(ollama_remote_host(api.url, "renamed"), "https://ollama.com:443")
            self.assertEqual(ollama_remote_host(api.url, "other:cloud"), "https://ollama.com")
        with MockAPI([]) as api:
            closed = api.url
        self.assertEqual(ollama_remote_host(closed, "x:cloud", timeout=0.5), "https://ollama.com")
        self.assertIsNone(ollama_remote_host(closed, "qwen3:8b", timeout=0.5))


if __name__ == "__main__":
    unittest.main()


@unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed")
class CompleteTest(TempDirTest):
    """The checker's request: one message, no tools, no thinking, no streaming."""

    def test_anthropic_and_ollama_send_a_plain_message(self):
        reply = ([{"type": "text", "text": "FLAG: unrelated"}], "end_turn")
        for make in (lambda url: AnthropicProvider(SYNTHETIC_KEY, "claude-haiku-5-5", "",
                                                   base_url=url, max_retries=0),
                     lambda url: OllamaProvider("qwen3.5:2b", url, max_retries=0)):
            with MockAPI([reply]) as api:
                answer = make(api.url).complete("system text", "the prompt")
            self.assertEqual(answer, "FLAG: unrelated")
            (request,) = api.requests
            body = request["body"]
            self.assertEqual(body["system"], "system text")
            self.assertEqual(body["messages"], [{"role": "user", "content": "the prompt"}])
            for option in ("stream", "tools", "thinking", "output_config", "fallbacks"):
                self.assertNotIn(option, body)

    def test_errors_are_provider_errors(self):
        with MockAPI([(404, {"type": "not_found_error"})]) as api:
            with self.assertRaisesRegex(ProviderError, "ollama pull qwen3.5:2b"):
                OllamaProvider("qwen3.5:2b", api.url, max_retries=0).complete("s", "p")

