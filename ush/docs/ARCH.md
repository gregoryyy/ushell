# ush architecture

How ush is built, in both implementations: the Python one in [ush/py](../py/), described in the
sections up to the decisions, and the Go one in [ush/go](../go/), which reproduces it
([The Go implementation](#the-go-implementation)). Security model: [SEC.md](SEC.md). Shared
paradigm: [wish IDEA.md](../../wish/docs/IDEA.md). Status: 2026-10-29 02:01; both implementations
at parity, 0.2.0-rc1 released.

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
| AI output | One validated program per request | One proposed command at a time, for the user to run; in agent mode, a loop of commands ush runs |
| Default execution | Within existing workspace grants | ush runs nothing; the user runs each proposal from their command line. Agent mode (`--run`): read-only commands run, everything else needs approval |
| Follow-up | Edit/replay the program | `//` continues with what the user ran, or asks a follow-up question |

The step-by-step conversation is the main departure. Diagnosing a local problem ("why is my
disk full?") needs several rounds of looking at real output, which a single generated program
cannot do. In propose mode the user runs every round themselves; in agent mode, approval for
anything not provably read-only replaces wish's workspace grants, because a local machine has no
workspace boundary.

## Components

```text
zsh ── ush.plugin.zsh ── quotes `ush //…`, `sudo ush //…`, `// …` before parsing;
 │                        puts proposals on the command line, records what then ran
 └─ ush (cli.py) ── propose step, session / follow-up loop, subcommands, piped input
     ├─ handoff.py ── propose mode: the plugin's handoff directory, saved conversations
     ├─ notes.py ──── `ush notes`: which files are sent, and why one is refused
     ├─ agent.py ──── request loop: provider turn → tool_use → policy → approval → execute,
     │                or in propose mode: → policy and checker notes → proposal
     │   ├─ policy.py ─── read-only classifier (zsh-aware tokenizer, command table, secret paths)
     │   ├─ execute.py ── bounded execution: zsh -f -o pipefail, timeouts, process groups, scrubbed env
     │   ├─ check.py ──── optional checker model: requests + proposal → OK or FLAG
     │   ├─ redact.py ─── secret redaction for provider input and history
     │   └─ ui.py ─────── terminal status/approval (via /dev/tty when stdin is a pipe)
     ├─ provider.py ── Anthropic SDK adapter (streaming, eager tool input, fallbacks);
     │                 Ollama through the same SDK and its Anthropic-compatible API
     ├─ config.py ──── identity under sudo, settings, key storage (keychain / 0600 file)
     └─ history.py ─── append-only JSONL history for `history` and `replay`
```

`policy`, `execute`, `redact`, `check`, and `history` do not depend on the provider. `agent`
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
   policy classifies it as read-only, the model marked it `read`, `--confirm` is
   off, and the checker model, if one is configured, answers OK. Otherwise the user approves, declines (with an optional reason sent to the
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

## Propose mode

The default since 2026-10-28 (TODO 2.17). The user's request: "the idea of the ushell is to
propose specific shell commands and prewrite them to be executed by the user"; their choices are in
[GAPS](GAPS.md) Q6.

1. `// REQUEST` runs `ush` as in agent mode, but the model has one tool, `propose_command`
   (`command`, `purpose`, `effect`, `interactive`), and a system prompt that says it runs nothing.
   The model either proposes one command or answers. ush shows the proposal with notes: a change, a
   command the policy can't verify as read-only, or the checker's objection. It runs nothing.
2. **Handoff:** ush writes the proposal into the plugin's handoff directory (`USH_HANDOFF`), and
   saves the conversation in `sessions/KEY.json`, one per shell.
3. **On the command line:** the plugin's `precmd` hook pushes the proposal onto the next command line
   with `print -z`. The user runs it with Enter, edits it, or replaces it.
4. **Recording what ran:** `preexec` records the line that runs from that prompt, unless it is
   another `ush` call. Unless the proposal is interactive, it sends the command's stdout and stderr
   through a FIFO into `tee`. The command still prints to the terminal, and its output lands in the
   directory. The next `precmd` restores the descriptors, waits for `tee` (at most 2 s), and writes
   the exit status and command line.
5. **Continuing:** `//`, or a new `// TEXT`, restores the conversation. It answers the pending
   `propose_command` with what ran: the same or an edited command, the exit status, a changed
   working directory, and the redacted output (bounded head and tail), or that nothing ran. The
   model then proposes the next command or answers.

**Keys on a proposal** (TODO 2.18–2.21):
- The plugin binds Esc and Down only while a proposal is on the line. It saves their bindings as
  `bindkey -L` prints them and restores them when the line runs, is cleared, or is abandoned.
- **Down** runs `ush explain --inline` on the line as it stands. That is one request without
  tools (`complete`), with the request and the proposal's purpose as context. The answer has a
  fixed form (Summary, Parts, Access, Notes); brief shows Parts and Access, `-v` all of it.
- The explanation is drawn as zle's `PREDISPLAY`, above the command and display-only, so the
  cursor stays in the command.
- **Explained once** (TODO 2.24): Down again on an unchanged line shows the same explanation
  without starting ush, in the plugin and in the session editor. `ush explain` itself keeps
  every answer in `explanations.json` in the state directory (mode 0600, the newest 500). The key
  is the SHA-256 of the command line, so the file holds no command lines. A command explained
  before, e.g. one recalled from history, is shown again without a request. Failures are not
  kept.
- **Esc** clears the explanation, or else the proposal.
- **The installers** offer to add the plugin to `~/.zshrc` (`shell/zshrc.sh`).

**Notes** (TODO 2.25, GAPS Q7): Ctrl-X n, bound for the whole shell, not only on a proposal,
and only where the key is free. `_ush_notes` splits the line with zsh's own parser
(`${(Z+c+)BUFFER}`), so a trailing comment is one word: the question.
- **A command line** (its first word is something `whence` finds) runs like a proposal: the
  widget sets the captured state and accepts the line. `precmd` writes `ran` as usual, then
  runs `ush notes --last`.
- **Other lines** are replaced by `ush notes --last` (no word names a file) or
  `ush notes -- WORDS` and accepted, so the history holds what was sent.
- **`ush notes`** is one request without tools (`complete`), like `ush explain`. The material
  goes between random markers, redacted, at most 200 KB in all. `--last` reads `ran` and
  `output`, or, once `//` has taken them, `last-ran` and `last-output`: `take_ran` renames them
  instead of deleting them, and the next run replaces them. Refusals are in SEC.

**Without the plugin**, ush prints the proposal, and the conversation is keyed by the parent
process. The next `ush //` tells the model that ush can't know what ran.

**In a session** (`ush`), the proposal is pre-filled at the `ush>` prompt by zsh's `vared`,
with the same keys. Python's readline can't pre-fill with libedit, macOS's default. A line
the user runs there answers it, with output.

**As root**, there is no handoff: the plugin creates none, and ush ignores `USH_HANDOFF`. The
proposal is printed, for the user to run as root.

**Why the user's own shell, not ush:**
- The command runs exactly as the user's shell would run it, aliases and all.
- `cd` and variables persist.
- It lands in the shell history.
- Nothing runs that the user didn't start with Enter.

**What it costs:**
- While a captured command runs, its stdout is a pipe, not the terminal. Colours and pagers turn
  off, which is why the model marks full-screen and prompting commands as interactive.
- Each round needs a keystroke.

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

### The user's allowlist

`ush allow add WORDS` (TODO 2.15) stores a rule in the settings file. A simple command whose words
start with a rule's words runs without approval if the table doesn't know the command; every other
condition above still holds, and so do the model's `change`, `--confirm`, and the checker. The
verdict names the rule, the prompt line says "allowed by your rule", and history records it.

Rules are refused when they would widen the policy instead of extending it: commands the table
already validates (a rule `find` would skip `find`'s check for `-delete`), commands that run other
commands or code (shells, `env`, `xargs`, `sudo`, interpreters), commands whose purpose is to
change or send data (`rm`, `chmod`, `curl`, `scp`, …; a forged `read` claim would run them unasked),
and words with shell syntax, quotes, globs, or paths. Rules come only from the settings file, not
the environment, and a root session uses none: they were given for the user's own account.

**No blacklist mode** (2026-10-28). A mode in which everything runs except listed commands would
turn the policy around, from "known safe" to "not known dangerous". It fails open on everything
nobody thought of: a renamed binary, an interpreter, `find -exec`, `awk 'BEGIN{system()}'`, `env
rm`, `git -c core.pager=…`, a tool installed yesterday, and every shell trick the tokenizer has to
reject today. A model steered by injected text only needs one such command. The command table plus
the allowlist covers the wish behind it, fewer prompts, without that failure mode. A deny list that
can only add friction (always ask, or never run, even when approved) is safe and may come later
(GAPS Q5).

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
- **Local models through Ollama's Anthropic-compatible API** (TODO 2.8) rather than
  a second adapter for Ollama's native or OpenAI-style API. A probe on 2026-10-27
  (Ollama 0.40.7) showed that `/v1/messages` accepts ush's request, strict tools
  included, and returns `tool_use` blocks, so the agent loop and SDK stay the same.
  The Ollama adapter sends only model, limits, system prompt, tools, and messages. Its
  endpoint must be a loopback address, and its HTTP client ignores proxy variables and
  redirects, so "stays on this machine" is true by construction. Cloud models are
  detected through `remote_host` in `/api/tags` and disclosed. Settings select the
  provider; `USH_PROVIDER`, `USH_MODEL`, and `USH_OLLAMA_URL` override them except as
  root. ush never reads `.env` files itself; `just ush` passes `.env` for development.
- **An independent checker model** (TODO 2.13, 2026-10-27). The user asked for "an
  adversarial model to check the proposed implementation that takes independent sources; it can be
  a small model" (interpretation in [GAPS](GAPS.md) A5). A second model judges each proposed command
  or script against the user's requests only: it never sees command output, piped input, files, or
  the main model's reasoning, which is where injected instructions live. Its prompt marks the
  requests and the proposal with a random tag, so a proposal can't close them, and gives five
  worked examples, which took llama3.2:3b from 3 to 6 of 7 cases. It can only add friction: a flag,
  an unclear reply, or an error makes the command need approval (declined without a terminal), and
  OK changes nothing. It runs on any provider (`provider:model`), through a plain non-streaming
  request without tools or thinking (`complete`), and is disclosed in the banner. Dry runs and
  `replay` don't call it. Why not more: letting the checker approve, or show its reason to the main
  model, would give an injection a target to optimise against.
- **Propose mode is the default; agent mode is a flag** (TODO 2.17, 2026-10-28), at the user's
  direction (GAPS Q6).
  - The read-only policy and the checker still run, but only to annotate a proposal, since the
    user runs it.
  - Proposals with control characters, bidi overrides or zero-width characters are refused before
    they reach the command line (the proposal side of SECTEST F1).
  - Output capture uses `tee` in the user's shell rather than a pty wrapper (`script`), so the
    command runs in that shell itself.
  - The saved conversation holds redacted output. It is a per-user, 0600 file that expires after
    an hour; history still keeps no output.
- **Name:** `wsh` (Wave Terminal's CLI) and `zish` (an active Zig shell) were taken.
  `ush`, "the you shell", collides only with small, unrelated npm/PyPI packages.

## The Go implementation

ush/go is the same command in Go: the deliverable, one static binary without dependencies. It
reproduces the Python implementation described above; this section records how, and where the two
differ inside while behaving the same. Until 2026-10-29 it was ushgo's own ARCH.

### Why a port, and why Go

The analysis of 2026-10-27 (recorded in [GAPS](GAPS.md) Q9) compared Go and Rust for ush's
~2,200 lines of Python:

| | Go | Rust |
| --- | --- | --- |
| Anthropic SDK | Official: `anthropic-sdk-go` (Apache-2.0, v2.3.0 of 2026-10-23) | None official; a streaming client of our own |
| Processes, groups, timeouts | `os/exec`, `syscall.SysProcAttr{Setpgid}`, `context` | `std::process`, `nix`; more code for the same |
| Terminal and pty | `golang.org/x/term`; `creack/pty` (MIT) for tests | `nix`, `portable-pty`; fine, more effort |
| Shell parsing | `mvdan.cc/sh` (BSD-3-Clause), a mature POSIX/bash parser | No mature zsh-aware parser |
| Static binary, cross-compiling | `CGO_ENABLED=0`, `GOOS`/`GOARCH` | Good, with musl targets |
| Guarantees for the parser | Memory-safe, garbage-collected | Memory-safe, stronger types and fuzzing tools |

Go wins on the official SDK and on less effort for the process and terminal work, which is most of
ush. Rust's stronger guarantees matter most for the command parser, and Go's built-in fuzzing
(`go test -fuzz`) covers the differential tests that matter there ([SEC](SEC.md)).

### Principles

- **ush is the specification.** Behaviour comes from ush's code and tests, not from a new design.
  A deliberate difference is recorded here and in ush's documents.
- **The same files.** ushgo reads and writes ush's settings (`~/.config/ush/config.json`), key store
  (keychain service or 0600 file) and history (`~/.local/state/ush/history.jsonl`) in the same
  formats, so the two can be swapped (GAPS A10).
- **Conformance by black-box tests.** ush's tests that drive the program from outside (the CLI
  against the mock Messages API, the approval prompt in a pty, the zsh plugin) run against the
  ushgo binary as well (TODO 4.10). Policy decisions are compared on a shared corpus of command
  lines (TODO 4.1).
- **Provider independence,** as in ush: `policy`, `execute`, `redact` and `history` don't import
  the provider package; `agent` sees only an interface.

### Packages

Each ush module becomes one internal package; `cmd/ush` is the CLI.

```text
cmd/ush ─────────── CLI: //requests, sessions, ai/history/replay subcommands     (cli.py)
internal/agent ──── provider turn → tool_use → policy → approval → execute        (agent.py)
internal/policy ─── zsh-aware tokenizer, command table, validators, secret paths  (policy.py)
internal/execute ── zsh -f -o pipefail, process groups, timeouts, bounded output   (execute.py)
internal/redact ─── secret redaction                                               (redact.py)
internal/ui ─────── status lines, approval prompt via /dev/tty                     (ui.py)
internal/provider ─ Anthropic (official SDK), Ollama through the same API          (provider.py)
internal/config ─── identity under sudo, settings, keychain / 0600 key file        (config.py)
internal/history ── append-only JSONL history and replay                           (history.py)
```

Also `internal/handoff` (propose mode's handoff directory and saved conversations, handoff.py),
`internal/notes` (what `ush notes` sends, notes.py) and `internal/check` (the checker model,
check.py). The zsh plugin (`ush/shell/ush.plugin.zsh`) is shared unchanged: both
implementations are the command `ush`.

### Decisions

- **2026-10-27: Go, in `ushgo/` of this repository.** The user left the choice between `ushrs`
  and `ushgo` open (GAPS Q9).
- **2026-10-28: the command is `ush`, and Go is the deliverable** (GAPS Q10). Both implementations
  build the command `ush`; `ush --version` names the implementation (`(go)`, `(python)`). Python is
  the experimental implementation where features are tried first; Go is what users install; both
  must behave the same. This replaces the binary name `ushgo` of 2026-10-27, which existed so both
  could be installed side by side: to compare them, install one and pass the other's path to the
  tests (`--bin`, conformance).
- **2026-10-27: Go 1.27, no dependencies until a package needs one.** Each dependency is pinned in
  `go.sum`, with its licence and maturity recorded here when it is added: the Anthropic SDK with
  the provider (TODO 4.6), `golang.org/x/term` with the approval prompt (TODO 4.8).
- **2026-10-28: no dependencies; the Messages API client is ushgo's own** (TODO 4.6, replacing the
  plan for `anthropic-sdk-go`). ush uses a small part of the API: one streaming request with
  tools, thinking, fallbacks and beta flags, plus one plain request for the checker and `explain`.
  About 400 lines on `net/http` cover it, with ushpy's error messages and the SDK's retry rules
  (408/409/429/5xx, connection errors). Nothing third-party to review, pin or update, and the
  release stays reproducible from the standard library alone. The cost is that new API features
  are ours to add. For Ollama it uses no proxy and follows no redirect, as ushpy does.
- **2026-10-28: the tokenizer is ported, not `mvdan.cc/sh`** (TODO 4.1), as proposed below. The
  shared corpus `ush/conform/policy.json` holds 1545 command lines with ushpy's verdicts and reasons;
  `ush/py/scripts/policy_corpus.py` writes it, ushpy's `test_conform` keeps it current, and ushgo's
  `TestConformance` compares every case. RE2 has no lookaround or backreferences: the sed
  substitution check is a small parser, and redaction captures what ushpy's patterns look around.
  RE2 matches in linear time, so redaction can't stall (SECTEST F3).
- **2026-10-28: conformance by ushpy's black-box tests** (TODO 4.10). `test_cli`, `test_security`,
  `test_smoke` and `test_zsh_plugin` start ush as a program; `USH_TEST_BIN` points them at the Go
  binary (`ush/go/scripts/conform.sh`, `just ushgo-conform`, CI job `ushgo-conform`). They cover
  the CLI against the mock Messages API, the approval prompt and sessions in a pty, propose mode,
  explanations, the security tests that run ush, and the smoke test.
- **Earlier proposal, TODO 4.1: port ush's tokenizer or use `mvdan.cc/sh`.** `mvdan.cc/sh` parses
  POSIX shell and bash, not zsh: zsh's glob qualifiers, `=cmd` expansion and other word rules
  differ. Porting ush's own tokenizer (about 110 lines, built for exactly the `zsh -f` rules the policy
  needs) keeps the decisions equal by construction. Proposed: port it, and use `mvdan.cc/sh` only if
  the differential tests show the port can't keep up.
- **2026-10-28: release builds with Go's own archive writer** (`tools/release`, TODO 5.1.1).
  `tar` differs between macOS (bsdtar) and Linux (GNU tar) in headers and options, so archives
  built on two machines would differ; `archive/tar` and `compress/gzip` from the standard library
  give the same bytes everywhere. Binaries: `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`,
  `-ldflags "-s -w -buildid= -X main.version=…"`. Archive members: the binary, `install.sh`, the
  zsh plugin from `ush/shell/` (one plugin for both implementations), the manual page `ush.1` from
  `ush/man/` (one page for both, installed to `PREFIX/share/man/man1`) and a README; timestamps
  are the commit time. The installer uses the same places as ushpy's (`~/.local/bin`,
  `~/.local/share/ush`; `PREFIX/bin`, `PREFIX/lib/ush`), so the plugin line in `~/.zshrc` stays
  valid when switching implementations. On macOS a Go binary links `libSystem` and `libresolv`
  from the system, as every macOS program does; nothing else.
- **CI:** the `ushgo` job in [ci.yml](../../.github/workflows/ci.yml) runs `scripts/test.sh` on
  Ubuntu and macOS with `actions/setup-go`; the local CI maps that action to the machine's Go when
  its version matches ([tools/ci/README.md](../../tools/ci/README.md)).

## Future work

See [TODO.md](TODO.md). The main candidates:

- An OS sandbox for auto-approved commands (macOS `sandbox-exec` profiles, Linux
  namespaces/Landlock), so that "read-only" is enforced rather than inferred.
- Compaction for long sessions.
- A shared `//` contract and policy semantics with wish.
