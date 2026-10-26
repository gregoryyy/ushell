# ush design

Status: implemented as version 0.1.0 on 2026-10-26; see [DONE.md](../DONE.md).
Security model: [SEC.md](SEC.md). Shared paradigm: [wish IDEA.md](../../wish/docs/IDEA.md).

## Relationship to wish

wish and ush share the command paradigm. Explicit commands run as typed. A standalone
submission beginning with `//` is a natural-language request that becomes real shell
commands. Those commands are displayed, checked by the same policy as manual input,
executed, and kept in history for replay without AI. The two differ in where they
run:

| | wish | ush |
| --- | --- | --- |
| Runs in | Browser workspace, isolated runtimes | The user's zsh, on the real machine, possibly as root |
| Resources | Workspace files, notebooks, pages | The whole filesystem and system state |
| AI output | One validated program per request | An agent loop: command → output → next command → answer |
| Default execution | Within existing workspace grants | Read-only commands run; everything else needs approval |
| Follow-up | Edit/replay the program | `//` follow-up questions in the same conversation |

The agent loop is the main departure. Diagnosing a local problem ("why is my disk
full?") needs several rounds of looking at real output, which a single generated
program cannot do. Approval-by-default for anything not provably read-only replaces
wish's workspace grants, because a local machine has no workspace boundary.

## Components

```text
zsh ── ush.plugin.zsh ── quotes `ush //…`, `sudo ush //…`, `// …` before parsing
 │
 └─ ush (cli.py) ── session / follow-up loop, subcommands, piped input
     ├─ agent.py ──── request loop: provider turn → tool_use → policy → approval → execute
     │   ├─ policy.py ─── read-only classifier (zsh-aware tokenizer, command table, secret paths)
     │   ├─ execute.py ── bounded execution: zsh -f -o pipefail, timeouts, process groups, scrubbed env
     │   ├─ redact.py ─── secret redaction for provider input and history
     │   └─ ui.py ─────── terminal status/approval (via /dev/tty when stdin is a pipe)
     ├─ provider.py ── Anthropic SDK adapter (streaming, eager tool input, fallbacks)
     ├─ config.py ──── identity under sudo, settings, key storage (keychain / 0600 file)
     └─ history.py ─── append-only JSONL history for `history` and `replay`
```

`policy`, `execute`, `redact`, and `history` do not depend on the provider. `agent`
sees only the `Provider` protocol (`turn(...) -> Turn`), so tests drive it with a
scripted fake.

## Request flow

1. The plugin's `zle-line-finish` hook rewrites the accepted line before zsh parses
   it, so `ush //why don't I…?` reaches ush as one verbatim argument. Under sudo it
   substitutes the absolute path of `ush`, because sudo's `secure_path` usually omits
   the install directory.
2. `cli` resolves the identity (root via sudo uses `SUDO_USER`'s key and settings)
   and the key, then starts the agent. Piped stdin (≤200 KB, redacted) becomes context.
3. The agent sends the conversation with two strict tools, `run_command` and
   `run_script`. Each call declares `effect: read|change` and a timeout.
4. Every `tool_use` input is validated against the schema; eager input streaming
   leaves validation to the client. A command runs automatically only if the
   policy classifies it as read-only, the model marked it `read`, and `--confirm` is
   off. Otherwise the user approves, declines (with an optional reason sent to the
   model), edits, or aborts. Scripts always need approval.
5. Execution uses `zsh -f -o pipefail -c`: no rc files, so the user's aliases cannot
   change meaning. Commands get stdin from `/dev/null` and their own process group,
   with a timeout. Approved commands keep the terminal so `sudo` can prompt. Output
   is bounded (head and tail kept) and redacted before it becomes a `tool_result`.
6. The final text is the answer. Follow-up questions append to the same
   conversation. History stores the request, steps, and a redacted answer.

The conversation is append-only and every `tool_use` receives a `tool_result`:
declined, skipped, aborted, truncated, and refused turns included. Follow-ups
therefore remain valid requests, and the thinking blocks echoed back stay valid
under preserved thinking.

## The read-only policy

The policy errs toward asking. A command line is auto-approved only when:

- It contains none of the following: command or process substitution, `${…}`, `$'…'`,
  arithmetic expansion, brace expansion, a leading `=` (zsh `EQUALS`), comments,
  multiple lines, background `&`, subshells/grouping, environment assignments, or
  output redirection other than to `/dev/null` and fd duplication.
- Every simple command in its pipeline/list is in a table of known commands, and its
  validator accepts the arguments. Examples: `find` without
  `-delete/-exec/-ok/-fprint*`; `sed` limited to print/substitute scripts without
  `w`/`e`; `diskutil` limited to `list`/`info`/`apfs list…`; `ps` without
  environment flags; `brew cleanup` only with `-n`.
- Content readers (`cat`, `grep`, `head`, `log show`, …) do not read secret
  locations. Recursive readers do not descend into a directory that contains them
  (a home directory, `/`).

The tokenizer models `zsh -f` word rules: quotes, backslashes, `$NAME`, leading `~`,
and unquoted globs. Validators therefore see the arguments the command will receive.
This matters because shells expand before the command runs. `find *` in a directory
holding a file named `-delete` becomes `find -delete`, and `$VAR` may hold an option.
`shlex` was tried first and rejected because it loses quoting information.

## Decisions

- **Python with the official Anthropic SDK (pinned 2.3.0, hashed lock file)** rather
  than wish's TypeScript core. The tool must run under sudo from a root-owned install
  with no build step. Python's standard library covers the pty, process-group, and
  terminal work. The SDK supplies streaming, typed errors, and current API shapes
  (API version 2026-10-21, bearer-token keys). SDK 2.x requires Python ≥ 3.10;
  macOS's `/usr/bin/python3` (3.9) is therefore not supported.
- **Copy-install into a venv** (`install.sh`) instead of `pip install .`. Building
  the package would fetch unpinned build tools; copying does not.
- **Model defaults:** `claude-opus-5-5`, adaptive thinking, effort `medium`. Between
  tool calls, ush shows progress updates (`display: "updates"`), and refusals fall
  back server-side (`fallbacks: "default"`). Both features are sent only to models
  that support them.
- **Name:** `wsh` (Wave Terminal's CLI) and `zish` (an active Zig shell) were taken.
  `ush`, "the you shell", collides only with small, unrelated npm/PyPI packages.

## Future work

See [TODO.md](../TODO.md). The main candidates:

- An OS sandbox for auto-approved commands (macOS `sandbox-exec` profiles, Linux
  namespaces/Landlock), so that "read-only" is enforced rather than inferred.
- Compaction for long sessions.
- A shared `//` contract and policy semantics with wish.
