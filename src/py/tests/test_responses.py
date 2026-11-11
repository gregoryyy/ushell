"""OpenAI's Responses API adapter (TODO 2.29.6) against a mock: the translation of the
conversation both ways, streaming, reasoning items kept for the next request, errors."""

import json
import unittest

import helpers  # noqa: F401 - puts the package on the path
from mock_openai import SYNTHETIC_OPENAI_KEY, MockOpenAI, call, reasoning, text

from ush.agent import PROPOSE_TOOL, TOOLS
from ush.provider import ProviderError
from ush.responses import OpenAIProvider, chat_model, list_models, to_blocks, to_input

PROPOSAL = {"command": "df -h", "purpose": "free space", "effect": "read", "interactive": False}


def provider(url, effort="medium", model="gpt-6-luna"):
    return OpenAIProvider(SYNTHETIC_OPENAI_KEY, model, effort, base_url=url, max_retries=0)


def ask(p, messages=None, tools=(PROPOSE_TOOL,)):
    shown, progress = [], []
    turn = p.turn("SYSTEM", list(tools), messages or [{"role": "user", "content": "space?"}],
                  shown.append, progress.append)
    return turn, "".join(shown), progress


class TranslationTest(unittest.TestCase):
    def test_the_conversation_becomes_input_items(self):
        messages = [
            {"role": "user", "content": "free space?"},
            {"role": "assistant", "content": [
                {"type": "thinking", "thinking": "Anthropic's", "signature": "s"},
                {"type": "reasoning", "item": {"type": "reasoning", "id": "rs_1",
                                               "encrypted_content": "x", "summary": []}},
                {"type": "text", "text": "Checking.", "openai_id": "msg_1"},
                {"type": "tool_use", "id": "call_1", "name": "propose_command", "input": PROPOSAL,
                 "openai_id": "fc_1"}]},
            {"role": "user", "content": [
                {"type": "tool_result", "tool_use_id": "call_1", "content": "exit 0\n/ 12G"},
                {"type": "text", "text": "and now?"}]},
        ]
        self.assertEqual(to_input(messages), [
            {"type": "message", "role": "user",
             "content": [{"type": "input_text", "text": "free space?"}]},
            {"type": "reasoning", "id": "rs_1", "encrypted_content": "x", "summary": []},
            {"type": "message", "role": "assistant", "id": "msg_1",
             "content": [{"type": "output_text", "text": "Checking."}]},
            {"type": "function_call", "call_id": "call_1", "name": "propose_command",
             "arguments": json.dumps(PROPOSAL), "id": "fc_1"},
            {"type": "function_call_output", "call_id": "call_1", "output": "exit 0\n/ 12G"},
            {"type": "message", "role": "user",
             "content": [{"type": "input_text", "text": "and now?"}]},
        ])  # Anthropic's thinking block is not sent to OpenAI

    def test_a_reply_becomes_content_blocks(self):
        blocks, refusal, summaries = to_blocks([
            reasoning("rs_1", "Looking at disks."), text("Here."),
            call("propose_command", PROPOSAL)])
        self.assertEqual([b["type"] for b in blocks], ["reasoning", "text", "tool_use"])
        self.assertNotIn("status", blocks[0]["item"])
        self.assertEqual(blocks[2], {"type": "tool_use", "id": "call_1", "name": "propose_command",
                                     "input": PROPOSAL, "openai_id": "fc_1"})
        self.assertEqual((refusal, summaries), ("", ["Looking at disks."]))

    def test_chat_models_are_told_from_the_others(self):
        for name in ("gpt-6-astra", "gpt-6.1-terra", "o5-mini", "gpt-6-luna-2026-10-01"):
            self.assertTrue(chat_model(name), name)
        for name in ("text-embedding-4-large", "gpt-6-tts", "whisper-2", "dall-e-4", "gpt-image-2",
                     "gpt-6-realtime", "omni-moderation-latest", "babbage-002", "sora-3"):
            self.assertFalse(chat_model(name), name)


class OpenAIProviderTest(unittest.TestCase):
    def test_a_streamed_reply_with_a_tool_call(self):
        with MockOpenAI([[reasoning(), text("Let me check."),
                          call("propose_command", PROPOSAL)]]) as api:
            turn, shown, _ = ask(provider(api.url))
        self.assertEqual(shown, "Let me check.")
        self.assertEqual(turn.stop_reason, "tool_use")
        self.assertEqual(turn.content[-1]["input"], PROPOSAL)
        self.assertEqual(turn.usage, {"input_tokens": 11, "output_tokens": 7})
        request = api.requests[0]
        self.assertEqual(request["path"], "/v1/responses")
        self.assertEqual(request["headers"]["Authorization"], f"Bearer {SYNTHETIC_OPENAI_KEY}")
        body = request["body"]
        self.assertEqual((body["model"], body["instructions"], body["store"], body["stream"]),
                         ("gpt-6-luna", "SYSTEM", False, True))
        self.assertEqual(body["reasoning"], {"effort": "medium"})
        self.assertEqual(body["include"], ["reasoning.encrypted_content"])
        self.assertNotIn("max_output_tokens", body)
        (tool,) = body["tools"]
        self.assertEqual((tool["type"], tool["name"], tool["strict"]),
                         ("function", "propose_command", True))
        self.assertEqual(tool["parameters"], PROPOSE_TOOL["input_schema"])

    def test_the_reasoning_of_a_turn_goes_back_with_its_tool_result(self):
        with MockOpenAI([[reasoning("rs_9"), call("run_command", {
                "command": "df -h", "purpose": "p", "effect": "read", "timeout_seconds": 20})],
                [text("12G free.")]]) as api:
            p = provider(api.url)
            first, _, _ = ask(p, tools=TOOLS)
            messages = [{"role": "user", "content": "space?"},
                        {"role": "assistant", "content": first.content},
                        {"role": "user", "content": [{"type": "tool_result",
                                                      "tool_use_id": "call_1",
                                                      "content": "/ 12G"}]}]
            second, shown, _ = ask(p, messages, tools=TOOLS)
        self.assertEqual((second.stop_reason, shown), ("end_turn", "12G free."))
        sent = api.requests[1]["body"]["input"]
        self.assertEqual([i["type"] for i in sent],
                         ["message", "reasoning", "function_call", "function_call_output"])
        self.assertEqual(sent[1]["encrypted_content"], "gAAAA-opaque-rs_9")

    def test_research_sends_no_tools(self):
        with MockOpenAI([[text("Compare A and B.")]]) as api:
            ask(provider(api.url), tools=())
        self.assertNotIn("tools", api.requests[0]["body"])

    def test_a_cut_off_reply_and_a_refusal(self):
        refused = {"type": "message", "id": "msg_2", "role": "assistant",
                   "content": [{"type": "refusal", "refusal": "No."}]}
        with MockOpenAI([{"output": [text("Half")], "status": "incomplete",
                          "reason": "max_output_tokens"}, [refused]]) as api:
            cut, _, _ = ask(provider(api.url))
            no, _, _ = ask(provider(api.url))
        self.assertEqual(cut.stop_reason, "max_tokens")
        self.assertEqual((no.stop_reason, no.refusal), ("refusal", "No."))

    def test_unreadable_arguments_ask_again(self):
        with MockOpenAI([[call("propose_command", "{not json")],
                         [call("propose_command", PROPOSAL)]]) as api:
            turn, _, _ = ask(provider(api.url))
        self.assertEqual((len(api.requests), turn.content[-1]["input"]), (2, PROPOSAL))

    def test_errors_name_the_fix(self):
        cases = [((401, "Incorrect API key"), "API key was rejected"),
                 ((404, "The model does not exist"), "unknown model 'gpt-6-luna'"),
                 ((429, "Rate limit"), "rate limited"),
                 ((400, "Unsupported value: 'max'"), "rejected the request: Unsupported value"),
                 ((503, "overloaded"), "provider error 503: overloaded")]
        for reply, expected in cases:
            with MockOpenAI([reply]) as api:
                with self.assertRaisesRegex(ProviderError, expected):
                    ask(provider(api.url))
        with MockOpenAI([{"events": [{"type": "error", "message": "server_error"}]}]) as api:
            with self.assertRaisesRegex(ProviderError, "OpenAI error: server_error"):
                ask(provider(api.url))
        with MockOpenAI([]) as api:
            closed = api.url
        with self.assertRaisesRegex(ProviderError, "cannot reach OpenAI"):
            ask(provider(closed))

    def test_a_rate_limit_is_retried_after_its_wait(self):
        with MockOpenAI([(429, "slow down", {"retry-after": "0"}), [text("ok")]]) as api:
            p = OpenAIProvider(SYNTHETIC_OPENAI_KEY, "gpt-6-luna", "low", base_url=api.url,
                               max_retries=1)
            turn, shown, _ = ask(p)
        self.assertEqual((len(api.requests), shown), (2, "ok"))

    def test_the_checker_gets_one_unstreamed_answer(self):
        with MockOpenAI([[text("OK")]]) as api:
            answer = provider(api.url).complete("judge", "rm -rf build")
        self.assertEqual(answer, "OK")
        body = api.requests[0]["body"]
        self.assertNotIn("stream", body)
        self.assertEqual(body["reasoning"], {"effort": "low"})

    def test_models_are_listed_newest_first_without_the_others(self):
        models = [{"id": "gpt-6-luna", "created": 100}, {"id": "text-embedding-4", "created": 300},
                  {"id": "gpt-6-astra", "created": 200}, {"id": "dall-e-4", "created": 400}]
        with MockOpenAI([], models=models) as api:
            self.assertEqual(list_models(api.url, SYNTHETIC_OPENAI_KEY),
                             ["gpt-6-astra", "gpt-6-luna"])
        self.assertEqual(api.requests[0]["headers"]["Authorization"],
                         f"Bearer {SYNTHETIC_OPENAI_KEY}")
        with MockOpenAI([], models=None) as api:
            self.assertEqual(list_models(api.url, SYNTHETIC_OPENAI_KEY), [])


if __name__ == "__main__":
    unittest.main()
