"""The providers ush can use, and what their models take (TODO 2.29.1, ARCH).

The provider table says, per provider, where it is, whether it must be on this machine, where its
key comes from, and how its models are named. A model profile says what a model takes: effort,
thinking, strict tool schemas, streamed tool input, an output limit, whether it may write tool
calls as text. Settings stay neutral (`effort`, `model`); the adapters in provider.py read the
profile, and the CLI reads the table, so a new provider is a row rather than a branch.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

API_URL = "https://api.anthropic.com"
OLLAMA_URL = "http://127.0.0.1:11434"
ANTHROPIC_MODEL = re.compile(r"^[a-z0-9.\-]{3,64}$")
OLLAMA_MODEL = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:/\-]{0,127}$")
KEY_ENV = ("USH_API_KEY", "ANTHROPIC_API_KEY")
MAX_TOKENS = 64000
# Models that accept server-side refusal fallbacks and between-tool progress notes.
FALLBACK_MODELS = {"claude-opus-5-5", "claude-opus-5", "claude-sonnet-5-5", "claude-fable-5-1",
                   "claude-fable-5"}
UPDATES_MODELS = {"claude-opus-5-5", "claude-sonnet-5-5", "claude-fable-5-1", "claude-fable-5"}


@dataclass(frozen=True)
class ProviderSpec:
    name: str                  # as in settings and `provider:model`
    label: str                 # as the disclosure and `ush ai status` name it
    api: str                   # "messages": Anthropic's Messages API (Ollama serves it too)
    url: str                   # the default endpoint
    url_setting: str           # the setting that holds the endpoint, or "" for `url`
    loopback: bool             # must be on this machine: no proxy, no redirect, stays here
    key_env: tuple             # variables a key may come from, before the key store
    key_store: bool            # needs a key, from the keychain or the credentials file
    model_pattern: re.Pattern
    model_setting: str         # the setting that holds its model
    lists_models: bool         # can say which models it has and which are in a cloud
    model_hint: str = ""       # where to find a model name, for the error when none is set


PROVIDERS = {
    "anthropic": ProviderSpec("anthropic", "Anthropic", "messages", API_URL, "", False, KEY_ENV,
                              True, ANTHROPIC_MODEL, "model", False),
    "ollama": ProviderSpec("ollama", "Ollama", "messages", OLLAMA_URL, "ollama_url", True, (),
                           False, OLLAMA_MODEL, "ollama_model", True, "see `ollama list`"),
}
NAMES = tuple(PROVIDERS)


@dataclass(frozen=True)
class ModelProfile:
    effort: bool               # takes an effort level (output_config.effort)
    thinking: str              # "", "adaptive", or "adaptive-updates" (progress notes)
    server_fallbacks: bool     # server-side fallbacks after a refusal
    strict_tools: bool         # strict tool schemas are sent as such
    eager_tool_input: bool     # tool input streamed eagerly (eager_input_streaming)
    prompt_cache: bool         # cache_control on the request
    output_limit: int          # max_tokens of a turn
    text_tool_calls: bool      # may write tool calls as text, read as calls (TODO 2.31)


def profile(provider: str, model: str) -> ModelProfile:
    """What `model` of `provider` takes."""
    if provider == "anthropic":
        return ModelProfile(
            effort=True,
            thinking="adaptive-updates" if model in UPDATES_MODELS else "adaptive",
            server_fallbacks=model in FALLBACK_MODELS, strict_tools=True, eager_tool_input=True,
            prompt_cache=True, output_limit=MAX_TOKENS, text_tool_calls=False)
    # Ollama: only model, limits, system prompt, tools and messages; small models write calls
    # as text.
    return ModelProfile(effort=False, thinking="", server_fallbacks=False, strict_tools=True,
                        eager_tool_input=False, prompt_cache=False, output_limit=MAX_TOKENS,
                        text_tool_calls=True)
