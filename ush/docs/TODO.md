# TODO

Plan for `ush`, the `//` shell for zsh, in both implementations: Python in [ush/py](../py/),
where features are tried first, and Go in [ush/go](../go/), the deliverable. What is built is in
[ARCH](ARCH.md), checked against [SEC](SEC.md) and [SECTEST](SECTEST.md); phases in
[ROAD](ROAD.md), open questions in [GAPS](GAPS.md), progress in [DONE](DONE.md). The process is
[SDLC](../../docs/SDLC.md); the rules in [AGENTS.md](../../AGENTS.md) take precedence. IDs are ush's
own, independent of wish's. Until 2026-10-29 each implementation had its own TODO; their old IDs
map to these as listed in [Old IDs](#old-ids). Status: 2026-10-30 05:40.

Status: `[ ]` open, `[x]` done. An item of section 2 also says where it is built: *py ✓ go ✓* in
both, *py ✓ go –* in Python so far, *py – go –* in neither. Tags:
- *(decision)*: a choice to make and record in DONE before the dependent work starts.
- *(variant)*: an alternative built next to the main design so the two can be compared.
- *(skipped)*: set aside by the user, with date; stays open as a record.

An item is done when it works in both implementations, its acceptance criteria pass in both, its
tests and documents are in the same PR, and DONE has an entry for it. A feature may land in Python
first; the item stays open, marked *py ✓ go –*, until Go has it too.

---

## 0. Go project setup

- [x] 0.1 Project documents: README, ARCH (packages, Go over Rust), SEC (what a compiled port
  changes), TODO, ROAD, GAPS, DONE.
- [x] 0.2 Go module (Go 1.27, no dependencies), `cmd/ushgo` that answers `--version` and `--help`
  and refuses everything else, `scripts/test.sh` (gofmt, vet, test), `just ushgo-test`, a CI job
  on Ubuntu and macOS, and `actions/setup-go` in the local CI. Acceptance: tests for the CLI stub
  and the built binary; the local CI maps the new job; `just ci` passes.
- [x] 0.3 The command is `ush` (GAPS Q10): `cmd/ush`, `ush --version` prints `ush X.Y.Z (go)`;
  equivalence with the Python implementation as the rule
  ([SDLC](../../docs/SDLC.md#ush-one-command-two-implementations)). Acceptance: tests for the
  name and version; the user smoke test (`ush/py/scripts/smoke.sh --bin`) runs against the Go binary
  and reports the checks it doesn't pass yet.
- [x] 0.4 User and admin documentation for the `ush` command as shipped by ushgo:
  [USER](USER.md) and [ADMIN](ADMIN.md), each saying what the Go implementation does today.
  Acceptance: every option, subcommand, setting, path, variable and exit status checked against
  ushpy's code; links pass.

## 1. Version 0.1: the local `//` shell

Done in Python, and in Go through the port (section 4).

- [x] 1.1 CLI with `ush //REQUEST`, follow-up questions, sessions where `//` asks and
  other lines run as commands, piped input, and `--dry-run/--confirm/--once/--verbose`.
  Acceptance: end-to-end runs against a mock API with and without a terminal.
- [x] 1.2 Agent loop with strict `run_command`/`run_script` tools, schema validation
  of eager-streamed input, step limits, and an append-only conversation where every
  `tool_use` is answered. Acceptance: approve/decline/edit/abort, refusal,
  truncation, invalid input, and follow-up after abort pass.
- [x] 1.3 Read-only policy with a zsh-aware tokenizer, command validators, and secret
  paths. Acceptance: injection, false `read` claims, and shell-syntax bypass attempts
  need approval; common diagnostics run without prompts.
- [x] 1.4 Bounded execution: `zsh -f -o pipefail`, `/dev/null` stdin, timeouts,
  process-group kill, output bounds, scrubbed environment, private script files.
  Acceptance: timeout/interrupt stop pipelines; credentials absent from command env.
- [x] 1.5 Identity under sudo, settings, and key storage (keychain on macOS via stdin,
  else a validated 0600 file). Acceptance: unit tests for resolution order, file
  permission refusal, keychain argv, and sudo user mapping. Live keychain-under-sudo
  behavior is tracked in 2.2.
- [x] 1.6 Redaction, history, and replay without AI. Acceptance: secrets absent from
  provider requests and history; replay reapplies the policy.
- [x] 1.7 zsh plugin for unquoted `ush //…`, `sudo ush //…`, and `// …`, with
  completion. Acceptance: pty tests with fake `ush`/`sudo` binaries.
- [x] 1.8 Anthropic adapter on the pinned SDK (2.3.0, hashed lock): streaming,
  progress updates, server-side fallbacks, actionable errors. Acceptance: the real
  SDK against a local mock server checks request shape, streaming, and error mapping.
- [x] 1.9 Installer (per user and root-owned), test runner, and CI workflow; docs.
  Acceptance: install into a scratch prefix works; suite passes on Python 3.10,
  3.11, and 3.13 on macOS. The first GitHub CI run is tracked in 2.3.

## 2. Follow-up work

- [ ] 2.1 *py – go –* User smoke test against a real model ([SMOKE](SMOKE.md)).
  - [x] 2.1.1 `scripts/smoke.sh` for any `ush` (`--bin`; Python or Go): offline checks, model
    checks without a terminal (read-only request, disclosure, declined change, injected
    instructions, piped input, dry run, replay), `--paid` for Anthropic, a report; manual checks
    M1–M10 in SMOKE. Acceptance: tests against the mock API, including a failing implementation
    and failing requests; a local Ollama run.
  - [ ] 2.1.2 A human runs it with their Anthropic key (`--paid`) and the manual checks, and records
    `smoke-report.md` in DONE.
- [ ] 2.2 *py – go –* Verify macOS keychain access under `sudo` and document the outcome.
  Acceptance: manual run of `sudo ush ai status` with a keychain-stored key.
- [ ] 2.3 *py – go –* First green run of `.github/workflows/ci.yml` on a PR (Linux and macOS,
  Python 3.10/3.13); configure branch protection once `main` exists remotely.
- [ ] 2.4 *py – go –* Extend the read-only command table from real sessions (Linux package
  managers, `podman`, `kubectl get`, `zfs list`, …) with negative tests per command.
- [ ] 2.5 *py – go –* Compaction or context editing for long sessions; measure token use.
- [ ] 2.6 *py – go –* Enforce read-only execution in agent mode with an OS sandbox (GAPS Q8, first of the
  agent-mode work). The commands agent mode runs without asking run without write access (apart
  from a private temporary directory) and without network: macOS `sandbox-exec` (Seatbelt),
  Linux bubblewrap or Landlock, as Claude Code's and Codex CLI's sandboxes do (docs/SOTA.md). Then
  "read-only" is enforced, not only inferred by the policy, and an injected read (F8) can't leave
  the machine. Approved commands and propose mode run unsandboxed, as the user's own commands.
  Acceptance: a write and a network connection by an auto-approved command fail on macOS and
  Linux; where the sandbox can't be set up, auto-approval is off and every command asks (fail
  closed, with the reason); SEC and SECTEST updated.
- [ ] 2.7 *py – go –* Align the `//` contract and policy semantics with wish once wish's shell
  integration (wish 2.1/6.2) exists.
- [x] 2.8 *py ✓ go ✓* Local models through Ollama's Anthropic-compatible API: provider setting
  and `USH_PROVIDER`/`USH_MODEL`/`USH_OLLAMA_URL` (not as root), loopback-only
  endpoint, no proxies or redirects, cloud-model disclosure, no key; `.env.template`
  and `just ush` for development. Acceptance: mock-API request shape, misconfiguration
  and remote-URL refusals before any request, honest banner; live run against a local
  Ollama; SEC updated.
- [x] 2.9 *py ✓ go ✓* Neutralise control characters (C0/C1, ANSI/OSC, bidi overrides, zero-width)
  in everything ush writes to the terminal: commands and scripts in the approval
  prompt, command output, and answers (SECTEST F1). Acceptance: rendered-bytes tests
  and a pty test show a `\r`/escape-sequence command visibly; no raw sequence reaches
  the terminal.
- [x] 2.10 *py ✓ go ✓* Close DNS exfiltration through auto-approved `dig`/`host`/`nslookup`
  (SECTEST F2), e.g. approval for names the model chose. Acceptance: negative policy
  tests; SEC residual risk updated.
- [x] 2.11 *py ✓ go ✓* Fill the remaining gaps in [SECTEST.md](SECTEST.md): injection corpus
  and opt-in model-in-the-loop evals (free on a local Ollama model), tokenizer
  differential fuzzing against zsh, a GTFOBins sweep, redaction vectors, redirect,
  `USH_BASE_URL` under root, planted symlinks, binary output, and oversized stdin.
  Acceptance: each listed gap has a committed test or a documented reason.
- [x] 2.12 *py ✓ go ✓* Model menu in `.env.template`: `USH_MODEL` choices per provider by size (Anthropic
  tiers; Ollama tags with download size and the memory they need, checked against the Ollama
  library), and `just ush` drops a trailing ` # comment` after a value. Acceptance: every listed
  model name passes ush's validation; uncommenting a menu line works.
- [x] 2.13 *py ✓ go ✓* Checker model: an optional second model (`provider:model`, `checker` setting,
  `USH_CHECKER`, `-k`) judges each proposed command or script against the user's requests only,
  never output, piped input, or the main model's reasoning; a flag, an unclear reply, or an error
  makes the command need approval; OK never approves; disclosed in the banner and `ai status`.
  Acceptance: agent, parser, settings, provider (`complete`) and end-to-end tests; a live run on a
  local model recorded in SECTEST; SEC updated.
- [x] 2.14 *py ✓ go ✓* A man page, `ush.1`, for the command in both implementations: synopsis, the `//`
  forms, options, subcommands, environment (`USH_*`), files, what leaves the machine, exit
  statuses, examples. One source in the repository; `install.sh` installs it to
  `share/man/man1`, and the Go release archive carries it (TODO 5.4). Acceptance: `man ush`
  after either install; a test that every option in `ush help` appears in the page. Done
  2026-10-29: `ush/man/ush.1` (man macros, `mandoc -T lint` clean), installed by both installers,
  in the archive and the formula (DONE).
- [x] 2.15 *py ✓ go ✓* User allowlist: `ush allow [add|remove RULE]`, `allow` in the settings file. A rule
  (command name and leading words) lets commands the table doesn't know run without approval;
  refused for table commands, runners and interpreters, commands that change or send data, and
  shell syntax; not used as root; history and the prompt name the rule. No blacklist mode (ARCH,
  GAPS Q5). Ported by TODO 4.11. Acceptance: policy, settings, agent and CLI tests,
  including refusals and root; SEC and SECTEST updated.
- [x] 2.16 *py ✓ go ✓* User and admin guides for ushpy ([USER](USER.md), [ADMIN](ADMIN.md)), sharing their
  command sections with ushgo's (TODO 0.4). Acceptance: `test_docs` fails if a shared
  section differs between the two, or if an option or subcommand in `ush help` is missing.
- [ ] 2.17 *py ✓ go ✓* Propose mode, the default (ARCH, GAPS Q6): ush proposes one command at a time and runs
  nothing; the zsh plugin puts the proposal on the command line and records what the user runs
  there (command line, exit status, output through `tee` unless interactive); `//` continues the
  saved per-shell conversation with it; `--run` and `mode: run` keep agent mode; `--new` starts
  over; proposals with control characters are refused. Ported by TODO 4.12.
  - [x] 2.17.1 Implementation and automated tests: agent, handoff, CLI end to end, the plugin in a
    real zsh, smoke checks; ARCH, SEC, guides, READMEs, SMOKE (M11–M15).
  - [ ] 2.17.2 Manual checks M11–M18 by the user with a real model, and a live smoke run in
    propose mode.
- [x] 2.18 *py ✓ go ✓* Plugin setup and Esc: both installers offer to add the plugin to `~/.zshrc`
  (`shell/zshrc.sh`: asks only from a terminal, `--zshrc`/`--no-zshrc`, never twice or as root);
  Esc removes a proposal from the command line and is bound only while one is there, restoring
  the user's own binding afterwards; without the plugin, ush prints the line to add. Acceptance:
  `test_install` (each mode, a real terminal answer), the plugin in a real zsh (Esc, the binding
  kept after running, removing and abandoning a proposal), the release archive carries the script.
- [x] 2.19 *py ✓ go ✓* Explain a command: `ush explain COMMAND…` sends the command line (and, for the pending
  proposal, the user's last request and its purpose) to the model as one request without tools,
  shows ush's policy verdict first, and runs nothing; Down on a proposal, past its last line and
  at the newest history line, runs it from the plugin, bound only while the proposal is there.
  Acceptance: CLI test against the mock API (context, redaction, no tools, nothing run or
  touched), the Down key in a real zsh (explains, the proposal stays and runs, Down elsewhere and
  its binding unchanged).
- [x] 2.20 *py ✓ go ✓* Explanations above the command, and proposals pre-filled in a session. Down shows the
  explanation above the proposal as display-only text (zle `PREDISPLAY`), with the cursor still
  in the command; Esc closes it first, a second Esc removes the proposal, Enter runs it, and the
  text is gone when the line is accepted. The `ush>` session pre-fills its proposal with zsh's
  `vared` and the same keys, because Python's readline can't pre-fill with libedit (macOS).
  Acceptance: the plugin in a real zsh (explanation before the command, edit with it shown, Esc
  twice, Enter runs the edited command); the session editor in a pty (pre-filled and edited, Esc,
  Down and Esc).
- [x] 2.21 *py ✓ go ✓* Brief explanations: the model answers in a fixed form (Summary, Parts, Access, Notes);
  by default only the parts and the access (files and network, read and write, anonymization
  where it applies) are shown, and `-v` (`ush -v`, `ush -v //…`, carried by the proposal, or
  `ush explain -v`) adds the banner, ush's verdict, the summary, the notes, and the key hint. The
  progress text and the explanation take the prompt's place above the command, without leftover
  lines; `{}` is literal in the policy, so `find -exec … {} +` names `-exec`. Acceptance: CLI
  tests (brief exactly the parts, verbose in full, hint only for the key, the proposal's -v), the
  order on a full redraw in a real zsh and in the session editor, policy tests for `{}`.
- [x] 2.22 *py ✓ go ✓* Arrow keys in the session, Up on an explanation: the `ush>` session reads every line
  with zsh's `vared` (history of the session's lines, Ctrl-D ends it); before, lines without a
  proposal went to Python's plain `input()`, which echoed `^[[A`. Up closes an explanation, in
  the session and in the plugin, and is history otherwise. Acceptance: a real `ush` session in a
  pty (Up recalls and reruns a line, Ctrl-D ends it), Up in the plugin and the session editor.
- [ ] 2.23 *py – go –* Argument origin in agent mode (SECTEST F8, GAPS Q12; unconditional since
  GAPS Q8): agent mode asks before a command whose arguments (paths, hosts, words beyond the
  command's name and options) occur only in earlier command output or piped input, not in the
  user's requests, with that as the reason. Data from output then can't choose what runs unasked,
  the idea of CaMeL's data-flow policies ([SOTA](../../docs/SOTA.md)). The checker is measured on
  F8's scenario too, and the eval runs each scenario several times and reports rates. Acceptance:
  F8's scenario ran unapproved 0 times in at least 20 runs; tests for the rule and its false
  positives (`ls` then `cat` of a listed file, a path the user named). Restored on 2026-10-29: the
  merge of #29 had brought back the text from before #28.
- [x] 2.24 *py ✓ go ✓* Explain once per command line (user, 2026-10-29): Down after Up or Esc closed an
  explanation asks the model again, though the command is the same. Keep the last explanation with
  the command line it explains, and show it again on Down while `$BUFFER` is unchanged; ask the
  model only when the line was edited. The plugin (`_ush_explain`, shared with ushgo) and the
  session editor (`ui.py`'s `vared` script) both do this; a new proposal starts without one.
  And "also cache any explanation of commands in the history" (user, 2026-10-29): `ush explain`
  keeps its answers in the state directory, `explanations.json`, keyed by a hash of the command
  line, so a command explained before, e.g. recalled from history, is not sent again.
  Acceptance: zsh tests in a pty with a counting fake provider: Down, Up, Down asks once; Down,
  Esc, Down asks once; Down, edit, Down asks twice; the same in the `ush>` session; a failed
  explanation is not kept; the store holds no command line, is 0600, keeps the newest 500, and
  ignores a broken or linked file; the Go session editor and store use the same file and keys.
- [x] 2.25 *py ✓ go ✓* Notes on command output and files (user, 2026-10-29: "add the possibility to analyze
  command output and files. this could be a shortcut like ctrl-n like notes"; ROAD's
  `ush analyze`). The key is Ctrl-X n, and the output is the plugin's recorded output (GAPS Q7,
  both answered):
  - **Ctrl-X n** on the command line asks the model for notes and prints them before the next
    prompt, so they stay in the scrollback. A comment ending the line is the question
    (`cat app.log # any errors?`); without one, the model says what it is and what stands out.
    `USH_NOTES_KEY` sets another key (empty: none); the key is bound only where it is free.
  - **Command output:** on an empty line, or one with only a question, notes on the output of the
    last command the plugin recorded, whether or not `//` has taken it (it stays as `last-ran` and
    `last-output` in the handoff directory). On a command line, the command runs as a proposal
    would, with its output recorded, and then gets notes.
  - **Files:** on a line that isn't a command, words that name files send those files; the rest
    is the question (`app.log nginx.conf why the 502?`). The line runs as `ush notes …`, so the
    history shows what was sent.
  - **Command line:** `ush notes [-v] [INSTRUCTION] FILE…`, also with piped input, for scripts
    and shells without the plugin; `--last` is the plugin's. Words naming existing paths are
    files, the rest is the instruction.
  - **Limits:** the same disclosure, redaction and 200 KB limit as piped input, for all the
    material together; files in secret locations (the policy's list, also through a link),
    directories, special files, binary files and files past the limit are refused with the
    reason; never as root, and the plugin refuses sudo and ush lines.

  Acceptance: zsh pty tests with a fake ush (Ctrl-X n on an empty line, on a command, on a line
  naming files, with a comment as question; sudo and ush lines refused; no other binding changed,
  `USH_NOTES_KEY`) and with the real ush against the mock API (a command, then its output again);
  `ush notes` against both implementations with files, piped input and `--last`; the refusals
  (secret file and link, binary, too large, directory, FIFO, root) and redaction of file content
  and output; SEC and SECTEST rows for files sent to the provider. The plugin's Ctrl-X n is shared
  by both.
- [ ] 2.26 *py – go –* History as context (ROAD idea, scheduled 2026-10-29): with the plugin, a request
  carries the shell's recent history (at most the last N lines: `fc -ln` in zsh and bash,
  `history` in fish, `Get-History` in PowerShell; none in sh), so "repeat that with
  these changes" works. The lines are filtered before they leave: secrets redacted, lines that
  read secret files (the policy's list) or carry tokens dropped, ush's own `//` lines dropped.
  Off by default; a setting turns it on, and the disclosure banner says so. Acceptance: zsh pty
  tests with a fake provider (history reaches the request when on, never when off, filtered
  lines absent); redaction tests with synthetic secrets; SEC, SECTEST, USER updated.
- [ ] 2.27 *py – go –* bash (ROAD idea, scheduled 2026-10-29), after 2.32: a bash plugin with the zsh plugin's
  behaviour as far as bash allows: `// …` without quotes (a `DEBUG` trap or `bind -x` on Enter),
  the proposal pre-filled (`bind -x` sets `READLINE_LINE`, bash 4 or newer), output recorded,
  Esc and Down on a proposal, the installer offering `~/.bashrc`; proposals in bash's syntax,
  no agent mode until bash has a policy and executor (GAPS Q15). The `ush>` session keeps
  zsh's `vared` where zsh exists. Acceptance: the zsh plugin's pty tests, ported to bash 4 and
  5 (macOS ships bash 3.2: the plugin refuses it with the reason); the Go release archive carries
  the bash plugin, and its installer offers `~/.bashrc`.
- [ ] 2.28 *py – go –* fish (ROAD idea, scheduled 2026-10-29): the same in fish (`commandline`, `bind`,
  `fish_preexec`/`fish_postexec`), after bash, with proposals in fish's syntax and no agent
  mode (GAPS Q15).
  Acceptance: as for 2.27, in fish 3, with the Go archive and installer.
- [ ] 2.29 *py – go –* More model platforms (user, 2026-10-29: "widen the compatibility with external models:
  openai, mistral, huggingface, any other important platforms"; 2026-10-30: "we need in
  particular mistral, openai, if possible other european providers and other local apis beyond
  ollama"). One adapter for the OpenAI Chat Completions API with tool calling, which most
  platforms serve. The agent keeps its `Provider` protocol and its Anthropic-shaped messages
  (ARCH); the adapter translates at its edge (`tool_use` to `tool_calls`, `tool_result` to
  `role: tool`, tool-call arguments streamed in pieces, stop reasons, strict schemas where the
  platform has them). In the user's order of need, one PR each, in both implementations:
  - [ ] 2.29.1 A provider table and model profiles in both implementations (GAPS Q13: "consider
    internal abstraction"). The table: name, display name, host, local or remote, key source and
    variable, model pattern, network rules. A model profile: what a model takes (effort or
    reasoning levels, strict tool schemas, tool choice, output limit, streamed tool arguments,
    tool calls written as text), so settings stay neutral (`effort`) and each adapter maps them,
    or says in `ush ai` that a model ignores one. Anthropic and Ollama move onto both with no
    change in behaviour; the `provider == "ollama"` checks in the CLI and settings become
    lookups. Acceptance: the existing suites pass unchanged in both; tests for the mapping of each
    setting per profile.
  - [ ] 2.29.2 The Chat Completions adapter, with `openai` and `mistral` (La Plateforme,
    `api.mistral.ai`) and their keys, the first presets (GAPS Q13), and `openai-compatible` with
    an explicit URL (`https`, or loopback `http`). Go: a Chat Completions client on `net/http`
    beside the Messages client, no dependencies.
  - [ ] 2.29.3 Presets for local servers beyond Ollama, each with its default loopback port:
    llama.cpp's `llama-server`, LM Studio, vLLM, LocalAI, Jan, llamafile and `mlx_lm.server`
    (GAPS Q13). Local servers get Ollama's loopback rules: no proxies, no redirects, "stays on
    this machine"; 2.31's reading of tool calls written as text applies.
  - [ ] 2.29.4 Other European platforms with an OpenAI-compatible API and tool calling, candidates
    to confirm against each one's documentation (GAPS Q13): Scaleway Generative APIs, OVHcloud
    AI Endpoints, IONOS AI Model Hub, STACKIT AI Model Serving, Nebius AI Studio. `ush ai
    configure` and the banner show the platform's country and region.
  - [ ] 2.29.5 The remaining presets: `huggingface` (the Inference Providers router), `gemini`
    (Google's OpenAI-compatible endpoint), `groq`, `openrouter`, `together`, `deepseek`, `azure`
    (the user's resource URL).
  - **Presets:** each a known host checked against the platform's docs when implemented; a
    platform whose models don't call tools reliably through its API is left out, with the reason
    in DONE.
  - **Keys:** per platform, in the keychain (account = preset) or the 0600 file, or from the
    platform's usual variable (`OPENAI_API_KEY`, `MISTRAL_API_KEY`, `HF_TOKEN`, …) as for
    Anthropic, never as root. The key is redacted from everything sent, and those variables are
    removed from the commands ush runs. Local servers need none unless configured.
  - **Disclosure:** the banner names the platform and host. `ush ai configure` offers the presets,
    and the checker can use any of them (`provider:model`).
  - **Acceptance** (each sub-item for what it adds):
    - a local mock of Chat Completions, streamed and not, with tool calls split across chunks;
      errors, retries and rate limits; tests for every preset's URL and key source, the
      loopback rules, key redaction, and root ignoring the variables;
    - a live smoke test and the injection eval on at least one OpenAI-compatible local server,
      free; paid platforms only with explicit configuration;
    - SEC (new destinations), ADMIN (network table), USER and SOTA updated.
- [ ] 2.30 *py – go –* Cloud platforms that need their own signing, after 2.29: AWS Bedrock (SigV4) and
  Google Vertex AI (service accounts), for Anthropic and other models there. Where a platform's
  OpenAI-compatible endpoint covers tool calling, 2.29's adapter is used instead. Acceptance: as
  for 2.29, with signing tested against fixed vectors; credentials from the platform's standard
  chain, never logged; in Go with the standard library only.
- [x] 2.31 *py ✓ go ✓* Tool calls written as text by local models (user, 2026-10-29: with llama3.2:3b,
  `//list the current directory` printed `{"name":"ls","parameters":{"command":"ls -bA",…}}`).
  Ollama turns Llama's call format into a call only when the name is a tool's; the model used the
  command as the name. The Ollama adapter reads such text as the call meant, the agent names the
  tools when it refuses an unknown one, and three turns of refused calls stop a request.
  Acceptance: unit, provider (mock Ollama) and CLI tests in both implementations; llama3.2:3b
  proposes a command for the request in 6 of 6 runs in each. Done 2026-10-29 (DONE).

- [ ] 2.32 *py – go –* A shell interface in the core (user, 2026-10-30: "bash, sh, potentially fish and
  powershell"; GAPS Q15), before 2.27. The prompts, proposals and explanations say "the user's
  shell is zsh" today, so a proposal for a bash or fish user is written for zsh. Each shell is an
  implementation of one interface: its name and syntax for the prompts, a parse check for
  proposals, the plugin's capabilities (pre-filled line, recorded output, keys), and, optionally,
  agent mode (a policy tokenizer and an executor). zsh is the first implementation and the only
  one with agent mode; bash, sh, fish and PowerShell get theirs in 2.27, 2.33, 2.28 and 2.34.
  - **Which shell:** the shell ush runs in. The plugin tells ush (`USH_SHELL`, set when it
    loads); without a plugin, the parent process, then `$SHELL`. `--shell NAME` and a `shell`
    setting choose one, e.g. `ush --shell bash` in a shell ush doesn't support; an unsupported
    shell without them is refused with the names ush supports.
  - **Agent mode:** `--run` (or `mode: run`) in a shell whose implementation has no agent mode is
    refused with the reason; propose mode works in every implemented shell.
  - **The plugin's contract** with ush (the `USH_HANDOFF` files, exit status, recorded output,
    keys) is written down in ARCH, and the pty tests take the shell as a parameter.
  - **Acceptance:** tests that the shell from `--shell`, the setting, `USH_SHELL`, the parent and
    `$SHELL` reaches the system prompt, in that order of precedence; `--run` refused in a shell
    without agent mode; a proposal that doesn't parse in the shell (`bash -n`, `sh -n`,
    `fish --no-execute`) sent back to the model once, then shown with a warning, against the mock;
    the zsh suites unchanged; ARCH, USER, the man page updated.
- [ ] 2.33 *py – go –* POSIX sh (`dash`, BusyBox `ash`, `sh` on macOS) (user, 2026-10-30), after 2.32. These
  shells have no line editor ush can hook into, so there is no plugin: `ush '//…'` is quoted,
  proposals are in POSIX syntax and are printed, and running one is the user's step (GAPS Q14).
  No agent mode (GAPS Q15).
  Acceptance: the CLI tests under `dash` and BusyBox `sh`; proposals pass `sh -n`; USER says
  what sh users don't get (no `// …`, no pre-filled line, no recorded output unless Q14 adds it).
- [ ] 2.34 *py – go –* PowerShell 7 (`pwsh`) on macOS and Linux (user, 2026-10-30: "potentially"), after
  2.28: a module for the profile with PSReadLine (`Set-PSReadLineKeyHandler` on Enter for
  `// …`, `[Microsoft.PowerShell.PSConsoleReadLine]::Replace` to pre-fill the proposal, the
  exit status from `$?`/`$LASTEXITCODE`, output through `Tee-Object`), proposals in PowerShell's
  syntax, propose mode only (GAPS Q15). Windows is out of scope until GAPS Q16 is answered.
  Acceptance: the plugin's pty tests ported to pwsh 7 on macOS and Linux; the installer offers
  the profile; the Go archive carries the module.

## 3. Release

- [ ] 3.1 Release 0.1 gate per [AGENTS.md](../../AGENTS.md): fresh suite on the exact
  candidate on each supported OS/Python, dependency audit, release notes, human merge.
  Overtaken: releases ship the Go binary (5.2), and each release runs the gate of 5.3; 0.1 was
  never released. Open as a record until the user closes it.

## 4. The Go port

The port of ushpy to Go, in dependency order. Until 2026-10-29: ushgo TODO 1.1–1.12.

- [x] 4.1 `internal/policy`: tokenizer, command table, validators, secret paths. *(decision)* Port
  ush's tokenizer or use `mvdan.cc/sh` (ARCH proposes the port). Acceptance: a shared corpus of
  command lines with expected decisions (JSON, generated from ush's policy tests) passes in both
  implementations; a fuzz test of the tokenizer against `zsh -f` finds no disagreement on a line
  the policy accepts.
- [x] 4.2 `internal/redact`. Acceptance: ush's redaction tests ported; the synthetic tokens stay in
  `.gitleaksignore`.
- [x] 4.3 `internal/execute`: `zsh -f -o pipefail`, `/dev/null` stdin, process groups, timeouts,
  interrupt, bounded output, scrubbed environment, private script files. Acceptance: ush's
  execution tests ported, including the pipeline that ignores SIGTERM.
- [x] 4.4 `internal/config`: identity under sudo, settings, keychain (secret on stdin) or 0600
  file, environment overrides ignored as root; checker names bare or `provider:model`, and
  `ai configure` asking again after a bad answer (ushpy, 2026-10-28). Acceptance: ush's config
  tests ported; settings written by one implementation read by the other.
- [x] 4.5 `internal/history`: append-only JSONL, `history`, `replay` through the policy.
  Acceptance: ush's history tests ported; history files interchangeable.
- [x] 4.6 `internal/provider`: Anthropic through `anthropic-sdk-go` (streaming, eager tool input,
  fallbacks, progress updates) and Ollama through the same API (loopback only, no proxies or
  redirects, cloud-model disclosure). Acceptance: request shape and errors against ush's mock
  Messages API (`ush/py/tests/mock_api.py`); SDK licence and maturity in ARCH.
- [x] 4.7 `internal/agent`: the loop with strict tools, schema validation, step limits, every
  `tool_use` answered. Acceptance: ush's agent tests ported with a scripted fake provider.
- [x] 4.8 `internal/ui`: status lines and the approval prompt via `/dev/tty`. Acceptance: pty tests;
  the display-integrity rules of TODO 2.9 apply here from the start.
- [x] 4.9 `cmd/ush`: the full CLI (requests, sessions, piped input, options, `ai`, `history`,
  `replay`). Acceptance: ush's CLI tests ported; the smoke test passes offline.
- [x] 4.10 Conformance: ush's black-box tests (`test_cli`, `test_zsh_plugin`, the pty approval
  tests) run against a binary chosen by an environment variable, and pass for both. Acceptance: a
  CI job runs them against the Go binary; the smoke test passes with a local model.
- [x] 4.11 Allow rules (TODO 2.15): `ush allow`, the `allow` setting, `parse_rule`'s
  refusals, prefix matching, not used as root. Acceptance: ushpy's `AllowRuleTest` cases in the
  policy corpus; the CLI tests for `ush allow` pass against the Go binary.
- [x] 4.12 Propose mode (TODO 2.17): `propose_command`, the default `mode`, `--run`, `--new`,
  the handoff directory and per-shell conversations (same files and formats as ushpy, so the
  plugin and a saved conversation work with either), refusal of control characters, `ush explain`
  (TODO 2.19, brief and -v 2.21), the session's pre-filled proposal (2.20: zsh `vared`, or
  the Go line editor); the installer's `zshrc.sh` and the plugin's keys are shared already (2.18, 2.20).
  Acceptance: ushpy's `test_propose` cases ported; the CLI propose and explain tests and the zsh
  plugin tests pass against the Go binary.

Done 2026-10-28 (DONE): all twelve, with two deviations recorded in ARCH. The provider is a
client of its own on `net/http` instead of `anthropic-sdk-go` (4.6), so ushgo has no dependencies.
The tokenizer is fuzzed against zsh through ushpy (4.1): the Go policy matches ushpy's on all
1545 lines of `ush/conform/policy.json`, 1500 of them seeded random, and ushpy's tokenizer is fuzzed
against `zsh -f` (ushpy `test_security`).

## 5. Releases of the Go binary

Until 2026-10-29: ushgo TODO 2.1–2.7.

- [ ] 5.1 Static release builds for macOS and Linux on arm64 and amd64 (`-trimpath`,
  `CGO_ENABLED=0`), an installer for a root-owned copy, checksums. Acceptance: reproducible from
  the tag on a second machine.
  - [x] 5.1.1 `tools/release` and `scripts/release.sh VERSION [--check]` (`just ushgo-release`):
    one `ush-VERSION-OS-ARCH.tar.gz` per target with the binary, `install.sh`, the zsh plugin and
    a README, plus `SHA256SUMS`; deterministic (no build ID or paths, fixed order, owners and
    times from the commit). Acceptance: tests for the packing, version checks, two full builds
    identical, the host binary runs and reports the version, the installer puts files where
    ushpy's does and none writable by others.
  - [ ] 5.1.2 The same checksums from the same tag on a second machine (Linux CI once Actions runs,
    or by hand).
  - [x] 5.1.3 *(decision)* macOS distribution: a tag, a GitHub release with the archives, and a
    Homebrew tap; no signing for now (GAPS Q11, answered 2026-10-28).
- [x] 5.2 *(decision)* Which implementation is maintained after parity: both, Go as the
  deliverable, Python as the experimental implementation (GAPS Q10, 2026-10-28).
- [ ] 5.3 Release gate per [AGENTS.md](../../AGENTS.md) and [SEC](SEC.md): conformance on each
  supported OS, ush SECTEST levels 1 and 2 on the binary, `govulncheck`, secret scan. Run for 0.2.0
  on 2026-10-29 in full except Linux amd64 and Intel macOS (no machine) and a live Anthropic run.
- [x] 5.4 The man page `ush.1` (TODO 2.14) in the release archive, installed to
  `share/man/man1`. Done 2026-10-29; ships from 0.2.1.
- [x] 5.5 0.2.0-rc1, the first release, of the Go binaries at parity (user, 2026-10-28): version
  0.2.0-rc1 in both implementations, release notes, the release gate (5.3) on the tagged commit.
  The AI prepares; the user tags and publishes. GAPS Q12 answered 2026-10-29: release with SECTEST
  F8 documented. Tag `v0.2.0-rc1` on b19db18, pushed by the agent at the user's request, then
  moved to b4f9d82. Done 2026-10-29; not published as a GitHub release: 0.2.0 (5.8) replaced it.
- [x] 5.6 GitHub release from the tag: the four archives and `SHA256SUMS` attached, the notes, the
  install line for curl users. Done for v0.2.0 (5.8).
- [x] 5.7 Homebrew tap: a formula that builds `ush` from the tagged source with Go
  (`brew install gregoryyy/ush/ush`); the tap repository `homebrew-ush` is created by the user.
  For v0.2.0 the formula is written, passes `brew style`, and its build steps work by hand from the
  tag's tarball; creating the public repository was refused to the agent by its permission check,
  so the user pushes it (DONE 2026-10-29 11:10). Each release then updates `Formula/ush.rb` there
  with `scripts/formula.sh VERSION`. Done 2026-10-29: the user created the repository with the
  0.2.0 formula; the agent pushed 0.2.1's. Homebrew asks users to `brew trust` the tap first.
- [x] 5.8 0.2.0 on the commit of 0.2.0-rc1, b4f9d82 (user, 2026-10-29: "do it. also publish the
  github release and homebrew tap"): the release gate (5.3) again on that commit with version
  0.2.0, the tag `v0.2.0`, the GitHub release (5.6) and the tap (5.7), published by the agent at
  the user's request. Done; the tap (5.7) followed. Release notes: [docs/releases/0.2.0.md](../../docs/releases/0.2.0.md).
- [x] 5.9 0.2.1, the next release, from `main` (user, 2026-10-29: "add a man page and re-release to
  0.2.1"; it replaces the planned 0.3.0): the layout (#30), explanations kept per command line
  (2.24), notes (2.25) and the man page (2.14, 5.4). Version 0.2.1 in both implementations; after
  the merge the gate (5.3) on the merge commit, the tag `v0.2.1`, the GitHub release, and the
  formula from `scripts/formula.sh`, which has the new layout's paths. Done 2026-10-29 on 5c77891,
  with SECTEST F8 documented again (GAPS Q12).
- [ ] 5.10 0.2.2 (user, 2026-10-29: "deploy to a homebrew tap", for the fix of TODO 2.31): tool calls
  that local models write as text. Version 0.2.2 in both implementations and the man page; after
  the merge the gate (5.3) on the merge commit, the tag `v0.2.2`, the GitHub release, and the
  formula in the tap. Release notes: [docs/releases/0.2.2.md](../../docs/releases/0.2.2.md).

## Old IDs

Until 2026-10-29 ushpy and ushgo kept separate TODO lists. DONE entries, commits and PRs before
then cite their IDs ([archive](archive/)).

| Old | Now |
| --- | --- |
| ushpy TODO *x* (also "ush TODO *x*" before 2026-10-28) | TODO *x* |
| ushgo TODO 0.1–0.4 | TODO 0.1–0.4 |
| ushgo TODO 1.1–1.12 | TODO 4.1–4.12 |
| ushgo TODO 2.1–2.7 (and 2.1.1–2.1.3) | TODO 5.1–5.7 (5.1.1–5.1.3) |
| ushgo TODO 2.8, 2.9, 2.10 | The Go side of TODO 2.23, 2.24, 2.25 |
| ushgo TODO 2.11, 2.12, 2.13, 2.14 | The Go side of TODO 2.6, 2.26, 2.27, 2.28 |
| ushgo TODO 2.15, 2.16 | The Go side of TODO 2.29, 2.30 |
| ushgo GAPS Q1–Q4, A1–A3 | GAPS Q9–Q12, A10–A12 |
