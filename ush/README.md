# ush, the you shell

ush brings wish's `//` paradigm to zsh on your own machine. Ask in plain language.
ush investigates with real Unix commands and short helper scripts, interprets the
output, and answers. You can then ask follow-up questions in the same context.

```console
$ ush //find out why my machine does not have enough disk space and visual tools don't seem to get anywhere
ush · claude-opus-5-5 · requests and command output go to Anthropic, secrets redacted
$ df -h /   # free space on the startup volume
  └ exit 0, 0.1s
$ tmutil listlocalsnapshots /   # local Time Machine snapshots
  └ exit 0, 0.3s
$ du -xsh ~/Library/* 2>/dev/null | sort -h | tail -8   # largest folders in Library
  └ exit 0, 41.2s
Your disk has 9 GB free of 460 GB. Most of the space Finder can't account for is
in local Time Machine snapshots (≈96 GB) and ~/Library/Developer (≈71 GB) ...
follow up with // QUESTION, run a command, or press Enter to finish
ush> // which of the Xcode folders can I delete safely?
```

The transcript above is illustrative; actual commands and answers depend on the
machine and the model.

## How it works

- **`//` asks; anything else is a command.** `ush //REQUEST` answers one request
  and then offers follow-ups. `ush` alone starts a session. In a session, `// TEXT`
  asks, and any other line runs as an ordinary command whose output becomes context
  for your next question.
- **Read-only commands run immediately.** ush has its own checker for command
  lines. It recognizes known read-only invocations such as `df`, `du`, `ls`,
  `find` without `-delete`/`-exec`, `diskutil list`, `tmutil listlocalsnapshots`,
  `docker system df`, and `log show`.
- **Everything else needs your approval:** deletions, writes, redirections,
  `sudo`, unknown commands, scripts, and commands that read secret files (e.g.
  `~/.ssh`, keychains, `.env`). ush shows the command and the reason; you answer
  `y`, `n` (optionally with a reason for the assistant), `e` to edit, or `a` to abort.
  The assistant's own claim that a command is read-only never bypasses this check.
- **Interpretation:** the model sees exit status and bounded output (secrets
  redacted), then explains the finding, key numbers, and next steps.
- **Replay without AI:** `ush history` lists requests and the commands they ran;
  `ush replay ID` runs those commands again through the same approval checks.

## Install

ush needs zsh, Python 3.10 or newer, and an Anthropic API key.

```sh
cd ush
./install.sh                     # installs ~/.local/bin/ush
ush ai configure                 # key goes to the macOS keychain (or a 0600 file)
```

Add the zsh plugin to `~/.zshrc` (the installer prints the exact line):

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

## Commands and options

```text
ush [OPTIONS] //REQUEST...   investigate and answer; then follow-up questions
ush [OPTIONS]                start a session
ush ai configure [--file]    set the API key, model, and effort
ush ai status                show provider, model, key source, and paths
ush ai clear-key             remove the stored API key
ush history [-n N]           list recent requests and the commands they ran
ush replay ID|last [-n]      run those commands again, without AI

-n, --dry-run   show what would run, run nothing     -c, --confirm  approve every command
-1, --once      no follow-up questions               -v, --verbose  show complete output
-m, --model M   model for this run                   -e, --effort L low … max
```

Piped input becomes context: `dmesg | tail -50 | ush //explain these errors`.
Without a terminal (cron, CI), ush cannot ask for approval. It runs read-only
commands only and declines the rest.

Settings live in `~/.config/ush/config.json` (`model`, `effort`, `max_steps`,
`command_timeout`, `max_timeout`, `output_limit`, `show_lines`). The default model
is `claude-opus-5-5` with effort `medium`. Refusals fall back server-side to a
model Anthropic selects (`fallbacks: "default"`).

## What leaves your machine

Each request sends your text, the OS name, your user name, and the working
directory. It also sends the output of every command that runs, up to 32 KB each,
plus any piped input. Before sending, ush redacts known token formats, private key
blocks, URL credentials, `password=`-style assignments, and your API key.
Redaction is pattern-based and cannot catch everything: assume that what a command
prints may reach the provider. Use `--confirm` to see each command before it runs.

History (`~/.local/state/ush/history.jsonl`, mode 0600) stores requests, commands,
decisions, exit codes, and a shortened, redacted answer, but no command output.
The API key is never passed to commands; `ANTHROPIC_*` variables are removed from
their environment.

## Limitations

- The checker is a convenience gate, not a sandbox. It judges command lines by
  name and arguments; a "read-only" tool with an unknown side effect would run
  without asking. See [SEC.md](docs/SEC.md) for the threat model.
- Interactive and full-screen programs (`vim`, `top` without `-l`) don't work
  inside ush; leave ush to use them.
- Starting ush takes about two seconds while the Anthropic SDK loads. Follow-ups
  in a session don't pay this again.
- Only the Anthropic provider is implemented.

## Development

```sh
just ush-test        # from the repository root; or ush/scripts/test.sh
```

The tests use fake providers, a local mock of the Messages API driven through the
real SDK, temporary directories, and a pseudo-terminal for the zsh plugin and the
approval prompt. They never call a paid provider. Design notes are in
[DESIGN.md](docs/DESIGN.md), the security model in [SEC.md](docs/SEC.md), and the
plan in [TODO.md](TODO.md) and [DONE.md](DONE.md).
