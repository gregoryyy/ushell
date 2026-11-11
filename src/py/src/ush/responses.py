"""OpenAI through its Responses API (TODO 2.29.6), with the standard library only.

The agent keeps Anthropic-shaped messages (ARCH); this adapter translates at its edge:
the system prompt becomes `instructions`, `tool_use` a `function_call` item, `tool_result` a
`function_call_output`, and the reply's items become content blocks again. Requests are
stateless (`store: false`): the reasoning items of a turn are kept in the conversation as
`reasoning` blocks, with their encrypted content, and sent back with the next request, as
OpenAI asks for tool calls. The finished reply is read from `response.completed`; the deltas
only stream text to the terminal.
"""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
from typing import Callable, Iterator, Optional

from .provider import CHECK_MAX_TOKENS, CHECK_TIMEOUT, ProviderError, Turn, _NoRedirect
from .providers import ModelProfile, profile

TIMEOUT = 600.0
MAX_WAIT = 20.0  # longest wait for a retry, whatever retry-after says
RETRY = {408, 409, 429, 500, 502, 503, 504}


def opener() -> urllib.request.OpenerDirector:
    """Redirects are not followed: a request goes where the settings say or nowhere."""
    return urllib.request.build_opener(_NoRedirect())


def to_input(messages: list[dict]) -> list[dict]:
    """The conversation as Responses input items."""
    items: list[dict] = []
    for m in messages:
        content = m.get("content")
        blocks = [{"type": "text", "text": content}] if isinstance(content, str) else content or []
        if m.get("role") == "user":
            texts = []
            for b in blocks:
                if b.get("type") == "text":
                    texts.append({"type": "input_text", "text": b.get("text", "")})
                elif b.get("type") == "tool_result":
                    if texts:
                        items.append({"type": "message", "role": "user", "content": texts})
                        texts = []
                    items.append({"type": "function_call_output", "call_id": b.get("tool_use_id"),
                                  "output": _result_text(b.get("content"))})
            if texts:
                items.append({"type": "message", "role": "user", "content": texts})
            continue
        for b in blocks:
            kind = b.get("type")
            if kind == "text" and b.get("text"):
                item = {"type": "message", "role": "assistant",
                        "content": [{"type": "output_text", "text": b["text"]}]}
                if b.get("openai_id"):
                    item["id"] = b["openai_id"]
                items.append(item)
            elif kind == "tool_use":
                item = {"type": "function_call", "call_id": b.get("id"), "name": b.get("name"),
                        "arguments": json.dumps(b.get("input") or {})}
                if b.get("openai_id"):
                    item["id"] = b["openai_id"]
                items.append(item)
            elif kind == "reasoning" and isinstance(b.get("item"), dict):
                items.append(b["item"])
            # Blocks of another provider (thinking, fallback) don't go to OpenAI.
    return items


def _result_text(content: object) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(b.get("text", "") for b in content if isinstance(b, dict))
    return ""


def to_blocks(output: list[dict]) -> tuple[list[dict], str, list[str]]:
    """The reply's items as content blocks, its refusal text, and its reasoning summaries."""
    blocks: list[dict] = []
    refusal, summaries = "", []
    for item in output if isinstance(output, list) else []:
        kind = item.get("type") if isinstance(item, dict) else None
        if kind == "reasoning":
            keep = {k: v for k, v in item.items() if k != "status"}
            blocks.append({"type": "reasoning", "item": keep})
            summaries += [s.get("text", "") for s in item.get("summary") or []
                          if isinstance(s, dict) and s.get("text")]
        elif kind == "message":
            for part in item.get("content") or []:
                if part.get("type") == "output_text":
                    blocks.append({"type": "text", "text": part.get("text", ""),
                                   "openai_id": item.get("id", "")})
                elif part.get("type") == "refusal":
                    refusal += part.get("refusal", "")
        elif kind == "function_call":
            try:
                arguments = json.loads(item.get("arguments") or "{}")
            except ValueError:
                raise _Unreadable() from None
            blocks.append({"type": "tool_use", "id": item.get("call_id"), "name": item.get("name"),
                           "input": arguments, "openai_id": item.get("id", "")})
    return blocks, refusal, summaries


class _Unreadable(Exception):
    """Tool-call arguments that are no JSON: the turn is asked again, as for Anthropic."""


class OpenAIProvider:
    """OpenAI's Responses API. What a request carries comes from the model's profile."""

    name = "OpenAI"
    provider = "openai"

    def __init__(self, api_key: str, model: str, effort: str, base_url: str,
                 max_retries: int = 2):
        self.key, self.model, self.effort = api_key, model, effort
        self.base_url = base_url.rstrip("/")
        self.max_retries = max_retries
        self.profile: ModelProfile = profile(self.provider, model)

    def _body(self, system: str, tools: list[dict], messages: list[dict]) -> dict:
        p = self.profile
        body: dict = {"model": self.model, "instructions": system, "input": to_input(messages),
                      "store": False, "stream": True}
        if tools:   # a research request (TODO 2.38) has none
            body["tools"] = [{"type": "function", "name": t["name"],
                              "description": t.get("description", ""),
                              "parameters": t["input_schema"],
                              "strict": bool(p.strict_tools and t.get("strict"))} for t in tools]
        if p.effort:
            body["reasoning"] = {"effort": self.effort}
            body["include"] = ["reasoning.encrypted_content"]
        if p.output_limit:
            body["max_output_tokens"] = p.output_limit
        return body

    def turn(self, system: str, tools: list[dict], messages: list[dict],
             on_text: Callable[[str], None], on_progress: Callable[[str], None]) -> Turn:
        body = self._body(system, tools, messages)
        for _ in range(3):
            response = self._stream(body, on_text)
            try:
                blocks, refusal, summaries = to_blocks(response.get("output") or [])
            except _Unreadable:
                continue
            for text in summaries:
                on_progress(text)
            return Turn(blocks, _stop_reason(response, blocks, refusal), refusal,
                        _usage(response))
        raise ProviderError("the model produced unreadable tool input three times")

    def complete(self, system: str, text: str) -> str:
        """One short answer without tools: the checker's request."""
        body = {"model": self.model, "instructions": system, "store": False,
                "input": [{"type": "message", "role": "user",
                           "content": [{"type": "input_text", "text": text}]}],
                "max_output_tokens": CHECK_MAX_TOKENS}
        if self.profile.effort:
            body["reasoning"] = {"effort": "low"}
        with self._post(body, CHECK_TIMEOUT) as reply:
            response = _json(reply.read())
        blocks, _, _ = to_blocks(response.get("output") or [])
        return "".join(b["text"] for b in blocks if b["type"] == "text")

    def _post(self, body: dict, timeout: float):
        request = urllib.request.Request(
            f"{self.base_url}/v1/responses", data=json.dumps(body).encode(), method="POST",
            headers={"Authorization": f"Bearer {self.key}", "Content-Type": "application/json",
                     "Accept": "text/event-stream" if body.get("stream") else "application/json"})
        for attempt in range(self.max_retries + 1):
            try:
                return opener().open(request, timeout=timeout)
            except urllib.error.HTTPError as exc:
                with exc:
                    if exc.code in RETRY and attempt < self.max_retries:
                        wait = _wait(exc.headers.get("retry-after"), attempt)
                    else:
                        raise self._error(exc.code, _message(exc)) from None
                time.sleep(wait)
                continue
            except (urllib.error.URLError, OSError) as exc:
                if attempt < self.max_retries:
                    time.sleep(_wait(None, attempt))
                    continue
                raise ProviderError("cannot reach OpenAI; check the network") from exc
        raise ProviderError("cannot reach OpenAI; check the network")  # not reached

    def _stream(self, body: dict, on_text: Callable[[str], None]) -> dict:
        """Posts the request and reads its events; returns the finished response object."""
        try:
            with self._post(body, TIMEOUT) as reply:
                for event in _events(reply):
                    kind = event.get("type", "")
                    if kind == "response.output_text.delta":
                        on_text(event.get("delta", ""))
                    elif kind in ("response.completed", "response.incomplete"):
                        return event.get("response") or {}
                    elif kind == "response.failed":
                        error = (event.get("response") or {}).get("error") or {}
                        raise ProviderError(f"OpenAI error: {error.get('message') or 'failed'}")
                    elif kind == "error":
                        raise ProviderError(f"OpenAI error: {event.get('message') or event}")
        except OSError as exc:
            raise ProviderError("the connection to the provider failed while reading its "
                                "reply") from exc
        raise ProviderError("the connection to the provider ended before the reply did")

    def _error(self, code: int, message: str) -> ProviderError:
        if code == 401:
            return ProviderError("the API key was rejected; run `ush ai configure`")
        if code == 403:
            return ProviderError(f"the API key may not use {self.model}: {message}")
        if code == 404:
            return ProviderError(f"unknown model {self.model!r}; run `ush ai configure`")
        if code == 429:
            return ProviderError("rate limited by the provider; try again shortly")
        if code == 400:
            return ProviderError(f"the provider rejected the request: {message}")
        return ProviderError(f"provider error {code}: {message}")


def _events(reply) -> Iterator[dict]:
    """Server-sent events: each `data:` line holds one JSON event with its `type`."""
    data: list[str] = []
    for raw in reply:
        line = raw.decode("utf-8", "replace").rstrip("\r\n")
        if line.startswith("data:"):
            data.append(line[5:].lstrip())
        elif not line and data:
            text, data = "\n".join(data), []
            if text != "[DONE]":
                yield _json(text.encode())
    if data and "\n".join(data) != "[DONE]":
        yield _json("\n".join(data).encode())


def _json(raw: bytes) -> dict:
    try:
        value = json.loads(raw)
    except ValueError:
        raise ProviderError("OpenAI sent a reply that is not JSON") from None
    return value if isinstance(value, dict) else {}


def _message(exc: urllib.error.HTTPError) -> str:
    try:
        error = json.loads(exc.read()).get("error") or {}
        return str(error.get("message") or exc.reason)
    except (ValueError, AttributeError, OSError):
        return str(exc.reason)


def _wait(retry_after: Optional[str], attempt: int) -> float:
    try:
        return min(MAX_WAIT, max(0.0, float(retry_after)))  # type: ignore[arg-type]
    except (TypeError, ValueError):
        return min(MAX_WAIT, 0.5 * 2 ** attempt)


def _stop_reason(response: dict, blocks: list[dict], refusal: str) -> str:
    if any(b["type"] == "tool_use" for b in blocks):
        return "tool_use"
    if refusal:
        return "refusal"
    reason = (response.get("incomplete_details") or {}).get("reason", "")
    if response.get("status") == "incomplete":
        return "refusal" if reason == "content_filter" else "max_tokens"
    return "end_turn"


def _usage(response: dict) -> dict:
    usage = response.get("usage") or {}
    return {k: usage[k] for k in ("input_tokens", "output_tokens") if isinstance(usage.get(k), int)}


def list_models(base_url: str, key: str, timeout: float = 5.0) -> list[str]:
    """OpenAI's models that answer in text, newest first; empty if they can't be listed."""
    request = urllib.request.Request(f"{base_url.rstrip('/')}/v1/models",
                                     headers={"Authorization": f"Bearer {key}"})
    try:
        with opener().open(request, timeout=timeout) as reply:
            data = json.loads(reply.read()).get("data") or []
    except (OSError, ValueError, AttributeError):
        return []
    models = [m for m in data if isinstance(m, dict) and isinstance(m.get("id"), str)
              and chat_model(m["id"])]
    models.sort(key=lambda m: (-(m.get("created") or 0) if isinstance(m.get("created"), int)
                               else 0, m["id"]))
    return [m["id"] for m in models]


NOT_CHAT = ("embedding", "tts", "whisper", "dall-e", "image", "audio", "realtime", "transcribe",
            "moderation", "search", "instruct", "babbage", "davinci", "sora", "computer-use")


def chat_model(name: str) -> bool:
    """Whether a model of OpenAI's list is one ush can talk to: a GPT or o-series text model."""
    lower = name.lower()
    return (lower.startswith("gpt-") or (lower[:1] == "o" and lower[1:2].isdigit())) and \
        not any(word in lower for word in NOT_CHAT)
