"""Provider adapters: Anthropic, and a local Ollama through the same Messages API.
The rest of ush sees only `Turn` and `ProviderError`."""

from __future__ import annotations

import json
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Callable, Optional, Protocol

API_URL = "https://api.anthropic.com"
OLLAMA_KEY = "ollama"  # Ollama ignores the key; the SDK requires one
MAX_TOKENS = 64000
# The checker's answer is one line; local thinking models need room to think first.
CHECK_MAX_TOKENS = 2048
CHECK_TIMEOUT = 60.0
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
                 base_url: Optional[str] = None, max_retries: int = 2, trust_env: bool = True):
        import anthropic  # imported lazily: commands without AI do not need the SDK

        self._anthropic = anthropic
        kwargs: dict[str, object] = {"max_retries": max_retries}
        if api_key:
            kwargs["api_key"] = api_key
        if base_url:
            kwargs["base_url"] = base_url
        if not trust_env:
            # No proxies from the environment and no redirects. The SDK's default client adds
            # environment proxies even with trust_env=False, so use its HTTP library directly.
            import httpx2

            kwargs["http_client"] = httpx2.Client(
                trust_env=False, follow_redirects=False, timeout=anthropic.DEFAULT_TIMEOUT,
                limits=anthropic.DEFAULT_CONNECTION_LIMITS)
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
            except anthropic.APIError as exc:
                raise self._error(exc) from None
        if message is None:
            raise ProviderError("the model produced unreadable tool input three times")
        content = [b.to_dict() for b in message.content]
        refusal = ""
        if message.stop_reason == "refusal" and message.stop_details:
            refusal = message.stop_details.explanation or message.stop_details.category or ""
        usage = message.usage.to_dict() if message.usage else {}
        return Turn(content, message.stop_reason or "", refusal, usage)

    def complete(self, system: str, text: str) -> str:
        """One short answer without tools, thinking, or streaming: the checker's request."""
        try:
            message = self.client.with_options(timeout=CHECK_TIMEOUT).messages.create(
                model=self.model, max_tokens=CHECK_MAX_TOKENS, system=system,
                messages=[{"role": "user", "content": text}])
        except self._anthropic.APIError as exc:
            raise self._error(exc) from None
        return "".join(b.text for b in message.content if b.type == "text")

    def _error(self, exc: Any) -> ProviderError:
        a = self._anthropic
        if isinstance(exc, a.AuthenticationError):
            return ProviderError("the API key was rejected; run `ush ai configure`")
        if isinstance(exc, a.PermissionDeniedError):
            return ProviderError(f"the API key may not use {self.model}: {exc.message}")
        if isinstance(exc, a.NotFoundError):
            return ProviderError(f"unknown model {self.model!r}; run `ush ai configure`")
        if isinstance(exc, a.RateLimitError):
            return ProviderError("rate limited by the provider; try again shortly")
        if isinstance(exc, a.BadRequestError):
            return ProviderError(f"the provider rejected the request: {exc.message}")
        if isinstance(exc, a.APIStatusError):
            return ProviderError(f"provider error {exc.status_code}: {exc.message}")
        if isinstance(exc, a.APIConnectionError):
            return ProviderError("cannot reach the provider; check the network")
        return ProviderError(f"provider error: {exc}")


class OllamaProvider(AnthropicProvider):
    """A local Ollama through its Anthropic-compatible Messages API (`/v1/messages`).

    Ollama needs no key and has none of Anthropic's server features, so requests carry only
    the model, limits, system prompt, tools, and messages. Proxy variables and redirects are
    not followed: the endpoint is on this machine (config.check_ollama_url) and requests must
    stay there.
    """

    name = "Ollama"

    def __init__(self, model: str, base_url: str, max_retries: int = 2):
        super().__init__(OLLAMA_KEY, model, "", base_url=base_url, max_retries=max_retries,
                         trust_env=False)
        self.base_url = base_url

    def _params(self, system: str, tools: list[dict], messages: list[dict]) -> dict:
        return {"model": self.model, "max_tokens": MAX_TOKENS, "system": system, "tools": tools,
                "messages": messages}

    def _error(self, exc: Any) -> ProviderError:
        a = self._anthropic
        if isinstance(exc, a.NotFoundError):
            return ProviderError(f"Ollama has no model {self.model!r}; run `ollama pull "
                                 f"{self.model}` or choose one from `ollama list`")
        if isinstance(exc, a.APIConnectionError):
            return ProviderError(f"cannot reach Ollama at {self.base_url}; start it with "
                                 "`ollama serve`")
        if isinstance(exc, a.AuthenticationError):
            return ProviderError(f"Ollama refused {self.model!r}; cloud models need `ollama signin`")
        if isinstance(exc, a.APIStatusError):
            body = exc.body if isinstance(exc.body, dict) else {}
            error = body.get("error") if isinstance(body.get("error"), dict) else {}
            return ProviderError(f"Ollama error {exc.status_code}: "
                                 f"{error.get('message') or exc.message}")
        return super()._error(exc)


def ollama_remote_host(base_url: str, model: str, timeout: float = 2.0) -> Optional[str]:
    """Where Ollama sends requests for `model`: its cloud host, or None for a local model.

    Ollama lists cloud models with a `remote_host`. A `cloud` tag counts as remote even if the
    list says otherwise or Ollama can't be asked, so ush never claims a cloud model is local.
    """
    name, _, tag = model.partition(":")
    guess = "https://ollama.com" if "cloud" in tag or name.endswith("-cloud") else None
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(f"{base_url}/api/tags", timeout=timeout) as response:
            models = json.load(response).get("models", [])
    except (OSError, ValueError, AttributeError):
        return guess
    for entry in models if isinstance(models, list) else []:
        if isinstance(entry, dict) and entry.get("name") in (model, f"{model}:latest"):
            return entry.get("remote_host") or guess
    return guess


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
