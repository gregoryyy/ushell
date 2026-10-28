"""Command-line entry point."""

from __future__ import annotations

import getpass
import os
import sys
import time
from typing import Callable, Optional

from . import __version__, execute
from .agent import Agent, Options
from .check import Checker
from .config import (EFFORTS, PROVIDERS, ConfigError, Identity, KeyStore, Settings,
                     apply_environment, check_ollama_url, identity, load_settings, mask,
                     save_settings)
from .handoff import Sessions, handoff_dir, session_key, take_ran, write_proposal
from .history import History, Step
from .policy import classify
from .provider import (API_URL, AnthropicProvider, OllamaProvider, Provider, Turn,
                       ollama_remote_host)
from .ui import Terminal

USAGE = """\
ush, the you shell: ask in natural language, get the commands that answer it.

usage:
  ush [OPTIONS] //REQUEST...   propose a command for your command line, or answer
  ush [OPTIONS] //             continue: send what you ran, get the next proposal
  ush [OPTIONS]                start a session
  ush ai configure [--file]    set the provider (Anthropic or a local Ollama), API key
                               (keychain on macOS), model, and effort
  ush ai status                show provider, model, key source, and paths
  ush ai clear-key             remove the stored API key
  ush allow [add|remove RULE]  list, add, or remove commands that may run without
                               approval, e.g. ush allow add kubectl get
  ush history [-n N]           list recent requests and the commands they ran
  ush replay ID|last [-n]      run those commands again, without AI
  ush help                     show this help

options:
  -r, --run          agent mode: ush runs the commands itself, read-only ones at
                     once, others after your approval; then follow-up questions
  -N, --new          start a new conversation instead of continuing this shell's
  -n, --dry-run      agent mode, but show the commands it would run, run none
  -c, --confirm      ask before every command, including read-only ones
  -1, --once         answer once, without follow-up questions
  -v, --verbose      show complete command output
  -m, --model MODEL  model for this run (default from `ush ai configure`)
  -e, --effort LEVEL low, medium, high, xhigh, or max
  -k, --checker MODEL checker for this run: claude-haiku-5-5, qwen3.5:4b, or off

By default ush runs nothing: it proposes one command at a time. With the zsh
plugin the proposal lands on your command line; run it with Enter, edit it, or
ignore it, then `//` continues with what you ran, its exit status and output.
`ush ai configure` sets agent mode as the default instead.

In a session or agent-mode follow-up, `// TEXT` asks, any other line runs as a
command, and an empty line (follow-up) or `exit` ends it. In agent mode,
commands that ush cannot verify as read-only run only after you approve them.
Requests and command output (with secrets redacted) are sent to the configured
AI provider; with a local Ollama model they stay on this machine. A checker
model, if configured, judges each proposed command against your requests only;
what it flags gets a warning, or needs your approval in agent mode.
USH_PROVIDER, USH_MODEL, USH_CHECKER, and USH_OLLAMA_URL override the settings
(not as root).

With the zsh plugin, `ush //...` and `// ...` need no quoting:
  source ushpy/shell/ush.plugin.zsh
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
        """Check that the provider and the checker can be used and return the secrets to
        redact. Fails before any command runs."""
        if self.ollama and not self.settings.ollama_model:
            raise ConfigError("no Ollama model; run `ush ai configure` or set USH_MODEL "
                              "(see `ollama list`)")
        if not self.ollama or self.settings.checker_provider == "anthropic":
            return (self.resolve_key(),)
        return ()

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

    def checker(self) -> Optional[Checker]:
        """The checker model, if one is configured; its client is created on first use."""
        provider, model = self.settings.checker_provider, self.settings.checker_model
        if not provider:
            return None

        def create() -> AnthropicProvider:
            if provider == "ollama":
                return OllamaProvider(model, self.settings.ollama_url)
            base_url = API_URL if self.ident.is_root else os.environ.get("USH_BASE_URL") or None
            return AnthropicProvider(self.resolve_key(), model, "", base_url=base_url)

        return Checker(create, model)


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
        print(f"ush {__version__} (python)")
        return 0
    ident = identity()
    settings = load_settings(ident)
    apply_environment(settings, ident)
    if "model" in overrides:
        settings.set_model(overrides["model"])
    if "effort" in overrides:
        settings.effort = overrides["effort"]
    if "checker" in overrides:
        settings.set_checker(overrides["checker"])
    if "mode" in overrides:
        settings.set_mode(overrides["mode"])
    keys = KeyStore(ident)
    history = History(ident.state_dir())

    if rest and rest[0] == "ai":
        return _ai(rest[1:], ui, ident, settings, keys)
    if rest and rest[0] == "history":
        return _history(rest[1:], history)
    if rest and rest[0] == "allow":
        return _allow(rest[1:], ui, ident)
    if rest and rest[0] == "replay":
        return _replay(rest[1:], ui, history, settings, options)
    if rest and not rest[0].startswith("//"):
        ui.error(f"unknown command {rest[0]!r}; requests start with //, see `ush help`")
        return 2

    request = " ".join(rest)[2:].strip() if rest else ""
    continuing = bool(rest) and not request     # `ush //`: continue after a proposal
    piped = None
    if not sys.stdin.isatty() and (request or continuing):
        data = sys.stdin.buffer.read(STDIN_LIMIT + 1)
        piped = data[:STDIN_LIMIT].decode("utf-8", "replace")
        if len(data) > STDIN_LIMIT:
            piped += "\n[... input shortened ...]"

    options.propose = settings.mode == "propose" and not options.dry_run
    factory = ProviderFactory(ident, settings, keys)
    secrets = ()
    if request or continuing:
        secrets = factory.secrets()  # fail early, before any command runs
    agent = Agent(LazyProvider(factory), ui, settings, ident, history, options, secrets,
                  factory.checker())
    if options.propose and (request or continuing):
        handoff = handoff_dir(ident)
        sessions = Sessions(ident.state_dir(), session_key(handoff))
        if overrides.get("new"):
            sessions.clear()
        else:
            saved = sessions.load()
            if saved:
                agent.restore(saved)
        if request or piped or agent.proposal is not None:
            return _propose(agent, ui, ident, settings, options, request, piped, handoff,
                            sessions)
        # `//` with nothing to continue opens a session, as before.
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


def _propose(agent: Agent, ui: Terminal, ident: Identity, settings: Settings, options: Options,
             request: str, piped: Optional[str], handoff: Optional[str],
             sessions: Sessions) -> int:
    """One step of propose mode: send the request (or what ran), then hand over the proposal."""
    _banner(ui, ident, settings, options)
    if piped:
        agent.add_context("Input piped to ush on stdin:", piped)
    ran = None
    if handoff and agent.proposal is not None:
        ran = take_ran(handoff, settings.output_limit)
    entry = agent.ask(request, ran, tracked=handoff is not None)
    try:
        sessions.save(agent.state())
    except OSError as exc:
        ui.warn(f"could not save the conversation: {exc}")
    if entry.outcome != "proposed" or agent.proposal is None:
        return {"answered": 0, "aborted": 130}.get(entry.outcome, 1)
    if handoff:
        try:
            write_proposal(handoff, agent.proposal["command"], agent.proposal["interactive"])
            ui.info("on your command line: Enter runs it, or edit it first · then // continues")
            return 0
        except OSError as exc:
            ui.warn(f"could not put it on your command line: {exc}")
    if ident.is_root:
        ui.info("ush ran nothing: copy the command to run it as root · then `ush //` continues")
        return 0
    ui.info("ush ran nothing: copy the command to run it · then `ush //` continues")
    plugin = _plugin_path()
    ui.info("to get it on your command line, add the zsh plugin to ~/.zshrc and open a new "
            "terminal" + (f":\n  source '{plugin}'" if plugin else " (see `ush help`)"))
    return 0


def _plugin_path() -> str:
    """The installed zsh plugin, next to the venv the installer made (install.sh)."""
    path = os.path.join(os.path.dirname(sys.prefix), "ush.plugin.zsh")
    return path if os.path.isfile(path) and "'" not in path else ""


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
        elif a in ("-r", "--run"):
            overrides["mode"] = "run"
        elif a in ("-N", "--new"):
            overrides["new"] = True
        elif a in ("-m", "--model", "-e", "--effort", "-k", "--checker") and i + 1 < len(argv):
            value = argv[i + 1]
            if a in ("-e", "--effort"):
                if value not in EFFORTS:
                    raise ConfigError(f"effort must be one of {', '.join(EFFORTS)}")
                overrides["effort"] = value
            elif a in ("-k", "--checker"):
                overrides["checker"] = value
            else:
                overrides["model"] = value
            i += 1
        else:
            raise ConfigError(f"unknown option {a}; see `ush help`")
        i += 1
    return options, overrides, argv[i:]


def _banner(ui: Terminal, ident: Identity, settings: Settings, options: Options) -> None:
    parts = [f"ush · {settings.active_model}"]
    if options.propose:
        parts.append("proposes, you run")
    if ident.is_root:
        parts.append("as root" + (f" (for {ident.owner_name})" if ident.via_sudo else ""))
    if options.dry_run:
        parts.append("dry run")
    parts.append(disclosure(settings))
    ui.info(" · ".join(parts))
    if settings.checker:
        ui.info(checker_disclosure(settings))


def disclosure(settings: Settings) -> str:
    """Where requests and command output go, as shown before every session."""
    if settings.provider != "ollama":
        return "requests and command output go to Anthropic, secrets redacted"
    remote = ollama_remote_host(settings.ollama_url, settings.ollama_model)
    if remote:
        return (f"cloud model: requests and command output go to {remote} via Ollama, "
                "secrets redacted")
    return f"local Ollama ({settings.ollama_url}): requests and command output stay on this machine"


def checker_disclosure(settings: Settings) -> str:
    """Where the checker sends your requests and the proposed commands (never command output)."""
    model = settings.checker_model
    if settings.checker_provider == "anthropic":
        where = "go to Anthropic"
    else:
        remote = ollama_remote_host(settings.ollama_url, model)
        where = f"go to {remote} via Ollama" if remote else "stay on this machine"
    return (f"checker {model}: your requests and the proposed commands {where}, secrets "
            "redacted; command output is not sent to it")


def _session(agent: Agent, ui: Terminal, factory: ProviderFactory, follow_up: bool) -> None:
    if follow_up:
        ui.info("follow up with // QUESTION, run a command, or press Enter to finish")
    elif agent.options.propose:
        ui.info("// REQUEST asks · proposals appear at the prompt: Enter runs, // continues · "
                "exit or Ctrl-D ends")
    else:
        ui.info("// REQUEST asks · other lines run as commands · exit or Ctrl-D ends")
    offer = ""
    while True:
        try:
            line = ui.read_line("ush> ", prefill=offer)
        except KeyboardInterrupt:
            ui.finish_answer()
            ui.err.write("\n")
            continue
        finally:
            offer = ""
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
            if not request and agent.proposal is None:
                continue
            try:
                agent.secrets = factory.secrets()
            except ConfigError as exc:
                ui.error(str(exc))
                continue
            agent.ask(request)
            if agent.proposal is not None:
                offer = agent.proposal["command"]
        else:
            agent.run_explicit(line)


# ---------------------------------------------------------------------------
# Subcommands


def _ask(ui: Terminal, prompt: str, apply: Callable[[str], None]) -> None:
    """Ask until the answer is empty (keep the current value) or `apply` accepts it."""
    while True:
        answer = input(prompt).strip()
        if not answer:
            return
        try:
            apply(answer)
            return
        except ConfigError as exc:
            ui.error(str(exc))


def _ai(args: list[str], ui: Terminal, ident: Identity, settings: Settings,
        keys: KeyStore) -> int:
    action = args[0] if args else "status"
    if action == "configure":
        settings = load_settings(ident)  # environment overrides are not saved
        _ask(ui, f"mode: propose (you run the commands) or run (ush runs them) "
                 f"[{settings.mode}]: ", settings.set_mode)
        _ask(ui, f"provider {'/'.join(PROVIDERS)} [{settings.provider}]: ", settings.set_provider)
        if settings.provider == "ollama":
            def set_url(url: str) -> None:
                settings.ollama_url = check_ollama_url(url)
            _ask(ui, f"Ollama URL [{settings.ollama_url}]: ", set_url)
            pulled = _ollama_models(settings.ollama_url)
            if pulled:
                ui.info("pulled models: " + ", ".join(pulled))
        else:
            prefer_keychain = "--file" not in args
            key = getpass.getpass("API key (hidden; Enter keeps the current key): ").strip()
            if key:
                where = keys.store(key, prefer_keychain=prefer_keychain)
                ui.info(f"key stored in {where}")
        _ask(ui, f"model [{settings.active_model}]: ", settings.set_model)
        if settings.provider == "anthropic":
            def set_effort(effort: str) -> None:
                if effort not in EFFORTS:
                    raise ConfigError(f"effort must be one of {', '.join(EFFORTS)}")
                settings.effort = effort
            _ask(ui, f"effort {'/'.join(EFFORTS)} [{settings.effort}]: ", set_effort)
        def set_checker(value: str) -> None:
            settings.set_checker(value)
            if settings.checker_provider == "ollama":
                pulled = _ollama_models(settings.ollama_url)
                if pulled and settings.checker_model not in pulled and \
                        f"{settings.checker_model}:latest" not in pulled:
                    raise ConfigError(f"Ollama has no model {settings.checker_model!r}; "
                                      f"pulled: {', '.join(pulled)}")
        _ask(ui, f"checker model, or off [{settings.checker or 'off'}]: ", set_checker)
        if settings.checker:
            ui.info(f"checker: {settings.checker_model} through {settings.checker_provider}")
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
        print("mode:      " + ("propose: ush proposes commands, you run them" if
                                settings.mode == "propose" else "run: ush runs commands (agent)"))
        print(f"checker:   {checker_disclosure(settings) if settings.checker else 'off'}")
        print(f"allow:     {len(settings.allow)} rule(s), see `ush allow`" +
              (" (not used as root)" if ident.is_root and settings.allow else ""))
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


def _allow(args: list[str], ui: Terminal, ident: Identity) -> int:
    """The allowlist in the settings file (environment overrides are not saved)."""
    settings = load_settings(ident)
    action, rule = (args[0], " ".join(args[1:])) if args else ("list", "")
    if action == "list":
        if not settings.allow:
            ui.info("no allow rules; only ush's own read-only commands run without approval")
        for rule in settings.allow:
            print(rule)
        if ident.is_root and settings.allow:
            ui.info("as root, these rules are not used")
        return 0
    if action not in ("add", "remove") or not rule:
        raise ConfigError("usage: ush allow [list | add RULE | remove RULE]")
    if action == "add":
        rule = settings.add_rule(rule)
        ui.info(f"allowed without approval: {rule} … (still asks for a change, a redirection, "
                "a secret file, or --confirm)")
    elif not settings.remove_rule(rule):
        raise ConfigError(f"no allow rule {rule!r}; see `ush allow`")
    save_settings(ident, settings)
    return 0


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
        verdict = classify(step.command, () if identity().is_root else settings.allow_rules)
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
