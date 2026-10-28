# ush user guide

How to use `ush`, the `//` shell for zsh: asking, approving, following up, and keeping control of
what runs and what leaves your machine. For installing ush for others, under `sudo`, or on many
machines, see the [admin guide](ADMIN.md). Status: 2026-10-28 10:23.

> **Which ush this describes.** `ush` is one command with two implementations that behave the same:
> the Go one built from `ushgo/` (the one releases will ship) and the Python one in `ushpy/`. Today
> the Go implementation answers only `ush --version` and `ush --help`; everything below works in the
> Python implementation, and each part reaches the Go one as it is ported
> ([TODO](TODO.md)). `ush --version` tells you which one you have: `ush 0.1.0 (python)` or
> `ush 0.0.0-dev (go)`.

<!-- shared:begin (identical in ushpy/docs and ushgo/docs; test_docs) -->

## What ush does

You ask a question about your machine in plain language. ush investigates with real commands, reads
their output, and answers. Commands it can verify as read-only run immediately; everything else is
shown to you first and runs only if you approve.

```console
$ ush //why is my disk full?
ush · claude-opus-5-5 · requests and command output go to Anthropic, secrets redacted
$ df -h /   # free space on the startup volume
  └ exit 0, 0.1s
$ du -xsh ~/Library/* 2>/dev/null | sort -h | tail -8   # largest folders in Library
  └ exit 0, 41.2s
Most of the space is in ~/Library/Developer (71 GB) and local Time Machine snapshots (96 GB) ...
follow up with // QUESTION, run a command, or press Enter to finish
ush> // which of the Xcode folders can I delete safely?
```

## First steps

1. **Install** it (or have it installed; [ADMIN](ADMIN.md#install)). Check: `ush --version`.
2. **Choose a model:** `ush ai configure` asks for the provider, then the key or the local model.
   - **Anthropic:** an API key from console.anthropic.com. It is stored in the macOS keychain (or
     in `~/.config/ush/credentials`, mode 0600, with `ush ai configure --file`).
   - **Ollama on this machine:** no key, and nothing leaves the machine. Pull a model that supports
     tools first, e.g. `ollama pull qwen3.5:9b` on a 16 GB machine.
3. **Add the zsh plugin** to `~/.zshrc`, so you can type requests without quotes. The installer
   prints the line, e.g. `source ~/.local/share/ush/ush.plugin.zsh`.
4. **Check it:** `ush ai status` shows the provider, model, where the key comes from (never the
   key), and the paths ush uses.

## Asking

| You type | What happens |
| --- | --- |
| `ush //REQUEST` | ush answers, then offers follow-up questions in the same context |
| `// REQUEST` (with the plugin) | the same, without typing `ush` |
| `ush` | a session: `// TEXT` asks, any other line runs as a command whose output becomes context, `exit` or Ctrl-D ends |
| `… \| ush //explain this` | piped text becomes context for the request (up to 200 KB) |
| `sudo ush //…` | commands run as root; see [Using sudo](#using-sudo) |

Without the plugin, quote requests (`ush '//why is my disk full?'`): otherwise zsh interprets `?`,
`'` and `*` before ush sees them. In a follow-up, an empty line ends the conversation.

## Approving commands

ush runs a command without asking only if all of these hold:
- ush's own policy recognises it as read-only (e.g. `df`, `du`, `ls`, `find` without `-delete` or
  `-exec`, `diskutil list`), or it matches one of [your allow rules](#your-allowlist);
- it uses no shell tricks: no `$(…)`, no redirection to a file, no `;` after a comment, no
  background `&`, and so on;
- it reads no secret files (`~/.ssh`, keychains, `.env`, credentials);
- the assistant didn't mark it as a change, you didn't pass `--confirm`, and the checker model (if
  you use one) didn't flag it.

Everything else is shown with the reason, and you answer:

| Answer | Effect |
| --- | --- |
| `y` | run it |
| `n` | don't; you may give a reason, which the assistant gets |
| `e` | edit the command, then run your version |
| `a` | stop the whole request |

Scripts the assistant writes are always shown in full and always need approval. The assistant's own
claim that something is read-only never makes it run unasked. Ctrl-C stops a running command and
everything it started.

## Your allowlist

If ush keeps asking about a harmless command it doesn't know, allow it:

```sh
ush allow add kubectl get        # kubectl get pods -A, kubectl get ns … run without asking
ush allow add git status
ush allow                        # list your rules
ush allow remove git status
```

A rule matches commands whose words start with it. Keep rules narrow: `git status`, not `git`
(which would also allow `git clean -fdx` if the assistant called it read-only). ush refuses rules
for commands it already checks (`find`, `sed`), for commands that run other commands (`sh`, `env`,
`xargs`, `python`), and for commands that change or send data (`rm`, `curl`). Under `sudo`, rules are
not used.

## The checker model

A second, small model can check each command before it runs. It sees only your requests and the
proposed command, never command output, so text that tricked the assistant can't talk to it. What it
flags needs your approval, even if ush would have run it; it never approves anything.

```sh
ush -k llama3.2:3b //why is my disk full?     # for one run, with a local model
ush ai configure                              # to keep it (last question)
```

A model name starting with `claude-` is Anthropic's, any other is Ollama's; `anthropic:…` or
`ollama:…` in front says so explicitly.

## Options

| Option | Effect |
| --- | --- |
| `-n`, `--dry-run` | show what the assistant would run; run nothing |
| `-c`, `--confirm` | ask before every command, read-only ones included |
| `-1`, `--once` | answer, then stop (no follow-ups) |
| `-v`, `--verbose` | show complete command output (default: the first 12 lines) |
| `-m`, `--model M` | model for this run |
| `-e`, `--effort L` | Anthropic effort: `low`, `medium`, `high`, `xhigh`, `max` |
| `-k`, `--checker MODEL` | checker for this run (`claude-haiku-5-5`, `llama3.2:3b`, `off`) |

## Other commands

| Command | Effect |
| --- | --- |
| `ush ai configure [--file]` | provider, key or Ollama URL, model, effort, checker |
| `ush ai status` | what is configured, and where things are |
| `ush ai clear-key` | remove the stored API key |
| `ush allow [add\|remove RULE]` | your allowlist |
| `ush history [-n N]` | recent requests and the commands they ran (default 10) |
| `ush replay ID\|last [-n]` | run those commands again, without AI, through the same approval checks |
| `ush help`, `ush --version` | usage; version and implementation |

## What leaves your machine

Before each session ush says where requests go. With Anthropic, each request sends your text, the OS
name, your user name, the working directory, the output of the commands that ran (up to 32 KB each)
and piped input. With a local Ollama model, all of that stays on this machine; with an Ollama
*cloud* model, Ollama forwards it to ollama.com, and ush says so. A checker gets your requests and
the proposed commands, never output.

Before sending, ush removes known token formats, private keys, URL credentials, `password=`
assignments and your API key. That is pattern matching: assume what a command prints may reach the
provider, and use `--confirm` on sensitive machines.

History (`~/.local/state/ush/history.jsonl`, mode 0600) keeps requests, commands, decisions, exit
statuses and a shortened answer, but no command output.

## Using sudo

`sudo ush //…` runs approved commands as root. ush uses your key and settings (it knows who called
`sudo`), keeps root's history separately, ignores `USH_*` variables, and uses none of your allow
rules. Only use it with an install that root owns ([ADMIN](ADMIN.md#for-sudo-a-root-owned-install)).
Alternatively, run ush without `sudo`: commands the assistant prefixes with `sudo` are shown for
approval and ask for your password as usual.

## Without a terminal

In cron jobs and scripts, ush can't ask, so it runs read-only commands only and declines everything
else; the answer says so. Exit status: 0 answered (or dry run), 1 error or refused, 2 usage or
configuration error, 130 stopped.

## When something goes wrong

| Symptom | Fix |
| --- | --- |
| "no API key" | `ush ai configure`; under `sudo` the keychain may be locked: `ush ai configure --file` |
| "Ollama has no model …" | `ollama pull MODEL`, or pick one from `ollama list` |
| "cannot reach Ollama" | start it: `ollama serve` |
| "Ollama URL … is not on this machine" | ush only uses an Ollama on `127.0.0.1`, `localhost` or `::1` |
| The answer comes without any command run | small local models do that; use a larger one (≥ 8B) |
| zsh says `no matches found` | quote the request or add the plugin |
| A command you trust keeps asking | `ush allow add …`, or check why: the reason is in the prompt |

To check a whole install, run the smoke test: `ushpy/scripts/smoke.sh --bin "$(command -v ush)"`
([SMOKE](../../ushpy/docs/SMOKE.md)).

<!-- shared:end -->
