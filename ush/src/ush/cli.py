"""Command-line entry point."""

from __future__ import annotations

import getpass
import os
import sys
import time
from typing import Optional

from . import __version__, execute
from .agent import Agent, Options
from .config import (EFFORTS, PROVIDERS, ConfigError, Identity, KeyStore, Settings,
                     apply_environment, check_ollama_url, identity, load_settings, mask,
                     save_settings)
from .history import History, Step
from .policy import classify
from .provider import (API_URL, AnthropicProvider, OllamaProvider, Provider, Turn,
                       ollama_remote_host)
from .ui import Terminal

USAGE = """\
ush, the you shell: ask in natural language, get answers from real commands.

usage:
  ush [OPTIONS] //REQUEST...   investigate and answer; then follow-up questions
  ush [OPTIONS]                start a session
  ush ai configure [--file]    set the provider (Anthropic or a local Ollama), API key
                               (keychain on macOS), model, and effort
  ush ai status                show provider, model, key source, and paths
  ush ai clear-key             remove the stored API key
  ush history [-n N]           list recent requests and the commands they ran
  ush replay ID|last [-n]      run those commands again, without AI
  ush help                     show this help

options:
  -n, --dry-run      show the commands the assistant would run, run none
  -c, --confirm      ask before every command, including read-only ones
  -1, --once         answer once, without follow-up questions
  -v, --verbose      show complete command output
  -m, --model MODEL  model for this run (default from `ush ai configure`)
  -e, --effort LEVEL low, medium, high, xhigh, or max

In a session or follow-up, `// TEXT` asks, any other line runs as a command, and
an empty line (follow-up) or `exit` ends it. Commands that ush cannot verify as
read-only are shown and run only after you approve them. Requests and command
output (with secrets redacted) are sent to the configured AI provider; with a
local Ollama model they stay on this machine. USH_PROVIDER, USH_MODEL, and
USH_OLLAMA_URL override the settings (not as root).

With the zsh plugin, `ush //...` and `// ...` need no quoting:
  source ush/shell/ush.plugin.zsh
"""

STDIN_LIMIT = 200_000


class LazyProvider:
    """Create the provider on first use, so commands without AI need no key."""

    def __init__(self, factory):
        self._factory = factory
        self._provider: Optional[Provider] = None
        self.name = "Ollama" if factory.settings.provider == "ollama" else "Anthropic"

    @property
    def model(self) -> str:
        return self._factory.model

    def turn(self, *args, **kwargs) -> Turn:
        if self._provider is None:
            self._provider = self._factory()
        return self._provider.turn(*args, **kwargs)


class ProviderFactory:
    def __init__(self, ident: Identity, settings: Settings, keys: KeyStore):
        self.ident, self.settings, self.keys = ident, settings, keys
        self.model = settings.active_model
        self.key: Optional[str] = None

    @property
    def ollama(self) -> bool:
        return self.settings.provider == "ollama"

    def secrets(self) -> tuple[str, ...]:
        """Check that the provider can be used and return the secrets to redact. Fails before
        any command runs."""
        if self.ollama:
            if not self.settings.ollama_model:
                raise ConfigError("no Ollama model; run `ush ai configure` or set USH_MODEL "
                                  "(see `ollama list`)")
            return ()
        return (self.resolve_key(),)

    def resolve_key(self) -> str:
        if self.key is None:
            key, _ = self.keys.resolve()
            if not key:
                who = f" for {self.ident.owner_name}" if self.ident.via_sudo else ""
                raise ConfigError(f"no API key{who}; run `ush ai configure` or set "
                                  "ANTHROPIC_API_KEY")
            self.key = key
        return self.key

    def __call__(self) -> Provider:
        if self.ollama:
            return OllamaProvider(self.settings.ollama_model, self.settings.ollama_url)
        # As root, ignore environment overrides of the endpoint (sudo -E).
        base_url = API_URL if self.ident.is_root else os.environ.get("USH_BASE_URL") or None
        return AnthropicProvider(self.resolve_key(), self.settings.model, self.settings.effort,
                                 base_url=base_url)


def main(argv: Optional[list[str]] = None) -> int:
    argv = sys.argv[1:] if argv is None else argv
    ui = Terminal()
    try:
        return _main(argv, ui)
    except ConfigError as exc:
        ui.error(str(exc))
        return 2
    except KeyboardInterrupt:
        ui.finish_answer()
        return 130


def _main(argv: list[str], ui: Terminal) -> int:
    options, overrides, rest = _parse_options(argv)
    if rest and rest[0] in ("help", "-h", "--help"):
        print(USAGE, end="")
        return 0
    if rest and rest[0] == "--version":
        print(f"ush {__version__}")
        return 0
    ident = identity()
    settings = load_settings(ident)
    apply_environment(settings, ident)
    if "model" in overrides:
        settings.set_model(overrides["model"])
    if "effort" in overrides:
        settings.effort = overrides["effort"]
    keys = KeyStore(ident)
    history = History(ident.state_dir())

    if rest and rest[0] == "ai":
        return _ai(rest[1:], ui, ident, settings, keys)
    if rest and rest[0] == "history":
        return _history(rest[1:], history)
    if rest and rest[0] == "replay":
        return _replay(rest[1:], ui, history, settings, options)
    if rest and not rest[0].startswith("//"):
        ui.error(f"unknown command {rest[0]!r}; requests start with //, see `ush help`")
        return 2

    request = " ".join(rest)[2:].strip() if rest else ""
    piped = None
    if not sys.stdin.isatty() and request:
        data = sys.stdin.buffer.read(STDIN_LIMIT + 1)
        piped = data[:STDIN_LIMIT].decode("utf-8", "replace")
        if len(data) > STDIN_LIMIT:
            piped += "\n[... input shortened ...]"

    factory = ProviderFactory(ident, settings, keys)
    secrets = ()
    if request:
        secrets = factory.secrets()  # fail early, before any command runs
    agent = Agent(LazyProvider(factory), ui, settings, ident, history, options, secrets)
    _banner(ui, ident, settings, options)
    if piped:
        agent.add_context("Input piped to ush on stdin:", piped)
    if not request:
        if not ui.can_prompt:
            ui.error("no request given and no terminal for a session; see `ush help`")
            return 2
        _session(agent, ui, factory, follow_up=False)
        return 0
    entry = agent.ask(request)
    if not options.once and ui.can_prompt:
        _session(agent, ui, factory, follow_up=True)
    return {"answered": 0, "dry-run": 0, "aborted": 130}.get(entry.outcome, 1)


def _parse_options(argv: list[str]) -> tuple[Options, dict, list[str]]:
    options, overrides = Options(), {}
    i = 0
    while i < len(argv) and argv[i].startswith("-") and argv[i] not in ("-h", "--help", "--version"):
        a = argv[i]
        if a in ("-n", "--dry-run"):
            options.dry_run = True
        elif a in ("-c", "--confirm"):
            options.confirm_all = True
        elif a in ("-v", "--verbose"):
            options.verbose = True
        elif a in ("-1", "--once"):
            options.once = True
        elif a in ("-m", "--model", "-e", "--effort") and i + 1 < len(argv):
            value = argv[i + 1]
            if a in ("-e", "--effort"):
                if value not in EFFORTS:
                    raise ConfigError(f"effort must be one of {', '.join(EFFORTS)}")
                overrides["effort"] = value
            else:
                overrides["model"] = value
            i += 1
        else:
            raise ConfigError(f"unknown option {a}; see `ush help`")
        i += 1
    return options, overrides, argv[i:]


def _banner(ui: Terminal, ident: Identity, settings: Settings, options: Options) -> None:
    parts = [f"ush · {settings.active_model}"]
    if ident.is_root:
        parts.append("as root" + (f" (for {ident.owner_name})" if ident.via_sudo else ""))
    if options.dry_run:
        parts.append("dry run")
    parts.append(disclosure(settings))
    ui.info(" · ".join(parts))


def disclosure(settings: Settings) -> str:
    """Where requests and command output go, as shown before every session."""
    if settings.provider != "ollama":
        return "requests and command output go to Anthropic, secrets redacted"
    remote = ollama_remote_host(settings.ollama_url, settings.ollama_model)
    if remote:
        return (f"cloud model: requests and command output go to {remote} via Ollama, "
                "secrets redacted")
    return f"local Ollama ({settings.ollama_url}): requests and command output stay on this machine"


def _session(agent: Agent, ui: Terminal, factory: ProviderFactory, follow_up: bool) -> None:
    if follow_up:
        ui.info("follow up with // QUESTION, run a command, or press Enter to finish")
    else:
        ui.info("// REQUEST asks · other lines run as commands · exit or Ctrl-D ends")
    while True:
        try:
            line = ui.read_line("ush> ")
        except KeyboardInterrupt:
            ui.finish_answer()
            ui.err.write("\n")
            continue
        if line is None:
            break
        line = line.strip()
        if not line:
            if follow_up:
                break
            continue
        if line in ("exit", "quit"):
            break
        if line.startswith("//"):
            request = line[2:].strip()
            if not request:
                continue
            try:
                agent.secrets = factory.secrets()
            except ConfigError as exc:
                ui.error(str(exc))
                continue
            agent.ask(request)
        else:
            agent.run_explicit(line)


# ---------------------------------------------------------------------------
# Subcommands


def _ai(args: list[str], ui: Terminal, ident: Identity, settings: Settings,
        keys: KeyStore) -> int:
    action = args[0] if args else "status"
    if action == "configure":
        settings = load_settings(ident)  # environment overrides are not saved
        provider = input(f"provider {'/'.join(PROVIDERS)} [{settings.provider}]: ").strip()
        if provider:
            settings.set_provider(provider)
        if settings.provider == "ollama":
            url = input(f"Ollama URL [{settings.ollama_url}]: ").strip()
            if url:
                settings.ollama_url = check_ollama_url(url)
            pulled = _ollama_models(settings.ollama_url)
            if pulled:
                ui.info("pulled models: " + ", ".join(pulled))
        else:
            prefer_keychain = "--file" not in args
            key = getpass.getpass("API key (hidden; Enter keeps the current key): ").strip()
            if key:
                where = keys.store(key, prefer_keychain=prefer_keychain)
                ui.info(f"key stored in {where}")
        model = input(f"model [{settings.active_model}]: ").strip()
        if model:
            settings.set_model(model)
        if settings.provider == "anthropic":
            effort = input(f"effort {'/'.join(EFFORTS)} [{settings.effort}]: ").strip()
            if effort:
                if effort not in EFFORTS:
                    raise ConfigError(f"effort must be one of {', '.join(EFFORTS)}")
                settings.effort = effort
        ui.info(f"settings saved to {save_settings(ident, settings)}")
        return 0
    if action == "status":
        if settings.provider == "ollama":
            print(f"provider:  Ollama ({settings.ollama_url})")
            print(f"model:     {settings.ollama_model or 'not configured'}")
            print(f"data:      {disclosure(settings)}")
            print("key:       not needed")
        else:
            key, source = keys.resolve()
            print(f"provider:  Anthropic ({API_URL})")
            print(f"model:     {settings.model} (effort {settings.effort})")
            print(f"key:       {mask(key) + ' from ' + source if key else 'not configured'}")
        print(f"user:      {ident.name}" + (f" (key and settings of {ident.owner_name})"
                                              if ident.via_sudo else ""))
        print(f"settings:  {os.path.join(ident.config_dir(), 'config.json')}")
        print(f"history:   {os.path.join(ident.state_dir(), 'history.jsonl')}")
        print(f"shell:     {' '.join(execute.find_shell()[:-1])}")
        return 0
    if action == "clear-key":
        removed = keys.clear()
        ui.info("removed key from " + ", ".join(removed) if removed else "no stored key found")
        return 0
    raise ConfigError(f"unknown ai command {action!r}; use configure, status, or clear-key")


def _ollama_models(url: str) -> list[str]:
    """Names of the models a local Ollama has, for `ush ai configure`; empty if unreachable."""
    import json
    import urllib.request
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(f"{url}/api/tags", timeout=2) as response:
            models = json.load(response).get("models", [])
        return [m["name"] for m in models if isinstance(m, dict) and isinstance(m.get("name"), str)]
    except (OSError, ValueError, AttributeError, TypeError):
        return []


def _history(args: list[str], history: History) -> int:
    n = 10
    if args[:1] == ["-n"] and len(args) > 1 and args[1].isdigit():
        n = int(args[1])
    for entry in history.recent(n):
        stamp = time.strftime("%Y-%m-%d %H:%M", time.localtime(entry.time))
        print(f"{entry.id}  {stamp}  [{entry.outcome}]  // {entry.request}")
        for step in entry.steps:
            first = step.command.splitlines()[0] if step.command else ""
            label = f"{step.language} script" if step.kind == "script" else "$"
            status = "" if step.exit_code is None else f" → exit {step.exit_code}"
            print(f"    {step.decision:9} {label} {first}{status}")
    return 0


def _replay(args: list[str], ui: Terminal, history: History, settings: Settings,
            options: Options) -> int:
    dry_run = options.dry_run or any(a in ("-n", "--dry-run") for a in args)
    ids = [a for a in args if not a.startswith("-")]
    if not ids:
        raise ConfigError("usage: ush replay ID|last [--dry-run]")
    entry = history.find(ids[0])
    if entry is None:
        raise ConfigError(f"no history entry {ids[0]!r}; see `ush history`")
    ui.info(f"replaying {entry.id}: // {entry.request}")
    steps = [s for s in entry.steps if s.decision in ("auto", "approved", "edited", "explicit")]
    if not steps:
        ui.info("that request ran no commands")
    failures = 0
    for step in steps:
        try:
            result = _replay_step(step, ui, settings, options, dry_run)
        except (execute.Cancelled, KeyboardInterrupt):
            ui.warn("stopped")
            return 130
        if result == "abort":
            return 130
        failures += result == "failed"
    return 1 if failures else 0


def _replay_step(step: Step, ui: Terminal, settings: Settings, options: Options,
                 dry_run: bool) -> str:
    if step.kind == "script":
        ui.show_script(step.language, step.command, step.purpose, "")
        auto, reason = False, "scripts always need approval"
    else:
        ui.show_command(step.command, step.purpose, "")
        verdict = classify(step.command)
        auto = verdict.auto and not options.confirm_all
        reason = verdict.reason or "--confirm asks for every command"
    if dry_run:
        return "skipped"
    if not auto:
        if not ui.can_prompt:
            ui.warn("not run: needs approval and no terminal is available")
            return "skipped"
        choice = ui.confirm(reason, editable=False)
        if choice.action == "abort":
            return "abort"
        if choice.action != "yes":
            return "skipped"
    kwargs = {"timeout": settings.max_timeout, "limit": settings.output_limit,
              "interactive": not auto}
    if step.kind == "script":
        result = execute.run_script(step.language, step.command, **kwargs)
    else:
        result = execute.run_command(step.command, **kwargs)
    ui.show_output(result, 10**9 if options.verbose else settings.show_lines)
    return "failed" if result.exit_code or result.timed_out else "ok"
