# ush user guide

How to use `ush`, the `//` shell for zsh: asking, approving, following up, and keeping control of
what runs and what leaves your machine. For installing ush for others, under `sudo`, or on many
machines, see the [admin guide](ADMIN.md). Status: 2026-10-29 02:01.

`ush` has two implementations that behave the same: the Go one in `ush/go/`, which releases ship,
and the Python one in `ush/py/`, where features are tried first. Everything below works in both;
`ush --version` tells you which one you have: `ush 0.2.2 (go)` or `ush 0.2.2 (python)`.

## What ush does

You ask about your machine, or for a task, in plain language. ush proposes the shell command that
answers it and puts it on your command line. You read it, edit it if you like, and run it with
Enter, in your own shell. Then `//` continues: ush sees what you ran and its output, and proposes the
next command or answers. ush itself runs nothing.

```console
$ // why is my disk full?
ush · claude-opus-5-5 · proposes, you run · requests and command output go to Anthropic, secrets redacted
First, how full each volume is.
→ df -h / /System/Volumes/Data   # free space on the startup volumes
on your command line: Enter runs it, or edit it first · then // continues
$ df -h / /System/Volumes/Data
Filesystem        Size    Used   Avail Capacity  ...
$ //
The data volume is 97% full. Next, the largest folders in your home.
→ du -xsh ~/* 2>/dev/null | sort -h | tail -8   # largest folders in your home
$ du -xsh ~/* 2>/dev/null | sort -h | tail -8
...
```

If you'd rather have ush investigate on its own, use agent mode, `ush --run //…`: it then runs
read-only commands itself and asks before anything else ([Agent mode](#agent-mode)).

## First steps

1. **Install** it (or have it installed; [ADMIN](ADMIN.md#install)). Check: `ush --version`.
2. **Choose a model:** `ush ai configure` asks for the provider, then the key or the local model.
   - **Anthropic:** an API key from console.anthropic.com. It is stored in the macOS keychain (or
     in `~/.config/ush/credentials`, mode 0600, with `ush ai configure --file`).
   - **Ollama on this machine:** no key, and nothing leaves the machine. Pull a model that supports
     tools first, e.g. `ollama pull qwen3.5:9b` on a 16 GB machine.
3. **The zsh plugin** puts proposals on your command line and lets you type requests without
   quotes. The installer offers to add it to `~/.zshrc`; otherwise add the line it prints, e.g.
   `source ~/.local/share/ush/ush.plugin.zsh`. Open a new terminal afterwards.
4. **Check it:** `ush ai status` shows the provider, model, where the key comes from (never the
   key), and the paths ush uses.

## Asking

| You type | What happens |
| --- | --- |
| `// REQUEST` (with the plugin) | ush proposes a command on your command line, or answers |
| `//` | continue: ush gets what you ran (command, exit status, output) and proposes the next one |
| `// TEXT` after a proposal | continue with a remark or a new question, e.g. `// that's the wrong disk` |
| `ush //REQUEST` | the same as `// REQUEST`, without the plugin's `//` shortcut |
| `ush --new //REQUEST` | start over instead of continuing this shell's conversation |
| `ush --run //REQUEST` | [agent mode](#agent-mode): ush runs the commands itself, then offers follow-up questions |
| `ush` | a session, with zsh's line editing and history (Up): `// TEXT` asks, proposals appear at the `ush>` prompt (with the same keys), any other line runs as a command whose output becomes context, `exit` or Ctrl-D ends |
| `… \| ush //explain this` | piped text becomes context for the request (up to 200 KB) |
| `sudo ush //…` | commands run as root; see [Using sudo](#using-sudo) |

Without the plugin, quote requests (`ush '//why is my disk full?'`): otherwise zsh interprets `?`,
`'` and `*` before ush sees them.

## Proposals

A proposal shows the command, its purpose, and in yellow what deserves a second look: that it
changes something, that ush's policy can't verify it as read-only, or that the checker model
objected. With the zsh plugin it then waits on your command line:
- **Enter** runs it in your shell, as if you had typed it: `cd` and variables persist, and it lands
  in your shell history. ush records the exit status and the output for the next `//`.
- **Edit** it first; ush sees the command that actually ran.
- **Down** (the arrow key, past the last line of the proposal) explains it: what each part of
  the command does, and what it accesses: files and network, read and write, and whether
  personal data or secrets end up hidden or exposed. With `-v` (`ush -v` for a session, or `ush -v //…`) you also get a summary,
  notes on what it reads, changes and risks, ush's own verdict, and where the command was sent.
  The explanation appears above the command; the cursor stays in the command, so you can still
  edit it, and Enter runs it. `ush explain [-v] COMMAND` explains any command line. A command
  is explained once: Down again, or the same command later (say, from your history), shows the
  same explanation without asking the model; only a changed command line is explained anew.
- **Up** closes the explanation too; otherwise it is your history, as always.
- **Esc** closes the explanation, or else removes the proposal, so you can start from scratch;
  ush then knows you didn't run it. Esc does this only while a proposal is on the line; otherwise
  it keeps its usual meaning. (In vi mode, use Ctrl-U.) Typing a new `// …` over it also tells ush you didn't run it.

While a proposed command runs, ush records its output through a pipe, so colours and pagers are off
for that one command. Commands that need the terminal (`less`, `top`, editors) are proposed as
interactive and run normally, without their output recorded.

Without the plugin, ush prints the proposal for you to copy, and can't know what you ran; paste the
relevant output into `// …` if it matters. The conversation continues in the same shell for an
hour; `ush --new` starts over.

## Notes

**Ctrl-N** asks the model for notes on command output or files: what it is, what stands out
(errors, warnings, failures, odd values), and what to do next. The notes are printed before the
next prompt, so they stay in your scrollback. A comment at the end of the line is your question:

| On the command line | Notes on |
| --- | --- |
| nothing, or only `# why so slow?` | the output of the last command ush recorded (a proposal you ran, or the last Ctrl-N command) |
| a command: `make test # why does it fail?` | that command's output: it runs as a proposal would, recorded, then gets notes |
| files: `app.log nginx.conf why the 502?` | the files the words name; the other words are the question |

For files, the line runs as `ush notes …`, so your history shows what was sent. Ctrl-N doesn't
work on `sudo` or `//` lines. `USH_NOTES_KEY` in `~/.zshrc`, before the plugin, sets another key,
or none when empty. ush takes Ctrl-N only where it still has zsh's own meaning (the next
history line, which Down also gives you), and another key only where it is free; a key you bound
yourself is left alone.

Without the plugin, or in a script: `ush notes [-v] [QUESTION] FILE…`, or pipe into it:
`journalctl -n 200 | ush notes "anything unusual?"`. Words that name existing files are files; the
rest is the question.

ush tells you what it sends and where, and sends at most 200 KB in all, secrets redacted. It
refuses, with the reason, files in secret locations (`~/.ssh`, `.env`, keys and the like, also
through a link), directories and other special files, binary files, and files past the limit.
It never runs as root: under sudo it would send what only root can read.

## Agent mode

With `--run`, or `mode` set to `run` in `ush ai configure`, ush investigates on its own: it runs
commands, reads their output, and answers. It runs a command without asking only if all of these
hold:
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
| `-r`, `--run` | agent mode: ush runs the commands itself |
| `-N`, `--new` | start a new conversation instead of continuing |
| `-n`, `--dry-run` | agent mode, showing what it would run; run nothing |
| `-c`, `--confirm` | agent mode: ask before every command, read-only ones included |
| `-1`, `--once` | agent mode: answer, then stop (no follow-ups) |
| `-v`, `--verbose` | full explanations (summary, parts, notes, ush's verdict); in agent mode, complete command output (default: the first 12 lines) |
| `-m`, `--model M` | model for this run |
| `-e`, `--effort L` | Anthropic effort: `low`, `medium`, `high`, `xhigh`, `max` |
| `-k`, `--checker MODEL` | checker for this run (`claude-haiku-5-5`, `llama3.2:3b`, `off`) |

## Other commands

| Command | Effect |
| --- | --- |
| `ush ai configure [--file]` | mode (propose or run), provider, key or Ollama URL, model, effort, checker |
| `ush ai status` | what is configured, and where things are |
| `ush ai clear-key` | remove the stored API key |
| `ush allow [add\|remove RULE]` | your allowlist |
| `ush explain [-v] COMMAND…` | explain a command line with the model, without running it |
| `ush notes [-v] [QUESTION] FILE…` | notes on files or piped input with the model (see [Notes](#notes)) |
| `ush history [-n N]` | recent requests and the commands they ran (default 10) |
| `ush replay ID\|last [-n]` | run those commands again, without AI, through the same approval checks |
| `ush help`, `ush --version` | usage; version and implementation |
| `man ush` | the manual page: all of the above in one place |

## What leaves your machine

Before each session ush says where requests go. With Anthropic, each request sends your text, the OS
name, your user name, the working directory, the output of the commands that ran (up to 32 KB each;
in propose mode, of the proposals you ran) and piped input. With a local Ollama model, all of that stays on this machine; with an Ollama
*cloud* model, Ollama forwards it to ollama.com, and ush says so. A checker gets your requests and
the proposed commands, never output. `ush explain` (and Down on a proposal) sends the command line
and your last request, unless that command was explained before. `ush notes` (and Ctrl-N) sends
the files, piped input or recorded output it names when it starts, up to 200 KB in all.

Before sending, ush removes known token formats, private keys, URL credentials, `password=`
assignments and your API key. That is pattern matching: assume what a command prints may reach the
provider, and use `--confirm` on sensitive machines.

History (`~/.local/state/ush/history.jsonl`, mode 0600) keeps requests, commands, decisions, exit
statuses and a shortened answer, but no command output. In propose mode the conversation, with the
redacted output you sent, is kept for an hour in `~/.local/state/ush/sessions/` (mode 0600) so that
`//` can continue it.

## Using sudo

In propose mode, `sudo ush //…` prints proposals meant for root without putting them on your command
line; run them with `sudo` yourself. More simply, ask without `sudo`: proposals that need root
start with `sudo`. In agent mode, `sudo ush --run //…` runs approved commands as root. ush uses your key and settings (it knows who called
`sudo`), keeps root's history separately, ignores `USH_*` variables, and uses none of your allow
rules. Only use it with an install that root owns ([ADMIN](ADMIN.md#for-sudo-a-root-owned-install)).
Alternatively, run ush without `sudo`: commands the assistant prefixes with `sudo` are shown for
approval and ask for your password as usual.

## Without a terminal

In cron jobs and scripts, propose mode prints the proposal. In agent mode ush can't ask, so it runs
read-only commands only and declines everything else; the answer says so. Exit status: 0 answered (or dry run), 1 error or refused, 2 usage or
configuration error, 130 stopped.

## When something goes wrong

| Symptom | Fix |
| --- | --- |
| "no API key" | `ush ai configure`; under `sudo` the keychain may be locked: `ush ai configure --file` |
| "Ollama has no model …" | `ollama pull MODEL`, or pick one from `ollama list` |
| "cannot reach Ollama" | start it: `ollama serve` |
| "Ollama URL … is not on this machine" | ush only uses an Ollama on `127.0.0.1`, `localhost` or `::1` |
| The answer comes without any command run | small local models do that; use a larger one (≥ 8B). llama3.2:3b often answers agent-mode questions without running anything, with made-up numbers |
| "the model's tool calls were refused 3 times in a row" | the model kept calling a tool that doesn't exist, or with the wrong input; use a larger model |
| zsh says `no matches found` | quote the request or add the plugin |
| A command you trust keeps asking (agent mode) | `ush allow add …`, or check why: the reason is in the prompt |
| The proposal doesn't appear on the command line | add the zsh plugin and open a new terminal; `echo $USH_HANDOFF` should print a directory |
| `//` says nothing about what you ran | the plugin records only the line run right after the proposal appeared |

To check a whole install, run the smoke test: `ush/py/scripts/smoke.sh --bin "$(command -v ush)"`
([SMOKE](SMOKE.md)).

