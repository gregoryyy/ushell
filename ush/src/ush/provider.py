"""Anthropic provider adapter. The rest of ush sees only `Turn` and `ProviderError`."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Optional, Protocol

API_URL = "https://api.anthropic.com"
MAX_TOKENS = 64000
# Models that accept server-side refusal fallbacks and between-tool progress notes.
FALLBACK_MODELS = {"claude-opus-5-5", "claude-opus-5", "claude-sonnet-5-5", "claude-fable-5-1",
                   "claude-fable-5"}
UPDATES_MODELS = {"claude-opus-5-5", "claude-sonnet-5-5", "claude-fable-5-1", "claude-fable-5"}
FALLBACK_BETA = "server-side-fallback-2026-07-01"
UPDATES_BETA = "thinking-display-updates-2026-08-18"


class ProviderError(Exception):
    pass


@dataclass
class Turn:
    content: list[dict]
    stop_reason: str
    refusal: str = ""
    usage: dict = field(default_factory=dict)


class Provider(Protocol):
    name: str
    model: str

    def turn(self, system: str, tools: list[dict], messages: list[dict],
             on_text: Callable[[str], None], on_progress: Callable[[str], None]) -> Turn: ...


class AnthropicProvider:
    name = "Anthropic"

    def __init__(self, api_key: Optional[str], model: str, effort: str,
                 base_url: Optional[str] = None, max_retries: int = 2):
        import anthropic  # imported lazily: commands without AI do not need the SDK

        self._anthropic = anthropic
        kwargs: dict[str, object] = {"max_retries": max_retries}
        if api_key:
            kwargs["api_key"] = api_key
        if base_url:
            kwargs["base_url"] = base_url
        self.client = anthropic.Anthropic(**kwargs)
        self.model = model
        self.effort = effort

    def _params(self, system: str, tools: list[dict], messages: list[dict]) -> dict:
        betas = []
        params = {
            "model": self.model,
            "max_tokens": MAX_TOKENS,
            "system": system,
            "tools": [{**t, "eager_input_streaming": True} for t in tools],
            "messages": messages,
            "cache_control": {"type": "ephemeral"},
            "output_config": {"effort": self.effort},
        }
        if self.model in UPDATES_MODELS:
            params["thinking"] = {"type": "adaptive", "display": "updates"}
            betas.append(UPDATES_BETA)
        else:
            params["thinking"] = {"type": "adaptive"}
        if self.model in FALLBACK_MODELS:
            params["fallbacks"] = "default"
            betas.append(FALLBACK_BETA)
        if betas:
            params["betas"] = betas
        return params

    def turn(self, system, tools, messages, on_text, on_progress) -> Turn:
        anthropic = self._anthropic
        params = self._params(system, tools, messages)
        message = None
        for _ in range(3):
            try:
                with self.client.beta.messages.stream(**params) as stream:
                    for event in stream:
                        if event.type == "text":
                            on_text(event.text)
                        elif event.type == "content_block_stop":
                            block = event.content_block
                            if block.type == "thinking" and block.thinking.strip():
                                on_progress(block.thinking)
                    message = stream.get_final_message()
                break
            except ValueError:
                # Tool input the SDK could not parse at all (eager streaming); there is
                # no tool_use id to answer, so re-issue the turn. API errors propagate.
                continue
            except anthropic.AuthenticationError:
                raise ProviderError("the API key was rejected; run `ush ai configure`") from None
            except anthropic.PermissionDeniedError as exc:
                raise ProviderError(f"the API key may not use {self.model}: {exc.message}") from None
            except anthropic.NotFoundError:
                raise ProviderError(f"unknown model {self.model!r}; run `ush ai configure`") from None
            except anthropic.RateLimitError:
                raise ProviderError("rate limited by the provider; try again shortly") from None
            except anthropic.BadRequestError as exc:
                raise ProviderError(f"the provider rejected the request: {exc.message}") from None
            except anthropic.APIStatusError as exc:
                raise ProviderError(f"provider error {exc.status_code}: {exc.message}") from None
            except anthropic.APIConnectionError:
                raise ProviderError("cannot reach the provider; check the network") from None
        if message is None:
            raise ProviderError("the model produced unreadable tool input three times")
        content = [b.to_dict() for b in message.content]
        refusal = ""
        if message.stop_reason == "refusal" and message.stop_details:
            refusal = message.stop_details.explanation or message.stop_details.category or ""
        usage = message.usage.to_dict() if message.usage else {}
        return Turn(content, message.stop_reason or "", refusal, usage)


def echo_content(content: list[dict]) -> list[dict]:
    """Prepare an assistant turn for the next request.

    After a mid-output fallback, thinking and tool_use blocks before the last
    `fallback` block belong to the declined model and must not be echoed back.
    """
    last = max((i for i, b in enumerate(content) if b.get("type") == "fallback"), default=-1)
    if last < 0:
        return content
    drop = {"thinking", "redacted_thinking", "tool_use", "server_tool_use"}
    return [b for i, b in enumerate(content) if i >= last or b.get("type") not in drop]
