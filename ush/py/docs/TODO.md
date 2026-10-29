# TODO

Plan for ush, the local `//` shell for zsh: what is built in [ARCH](ARCH.md), checked against
[SEC](SEC.md) and [SECTEST](SECTEST.md). Phases after the current work are in [ROAD](ROAD.md), open
questions in [GAPS](GAPS.md), and progress is logged in [DONE](DONE.md). The process is
[SDLC](../../../docs/SDLC.md); the rules in [AGENTS.md](../../../AGENTS.md) take precedence. IDs are ush's
own, independent of ushgo's and wish's. Status: 2026-10-28 09:59.

Status: `[ ]` open, `[x]` done. Tags:
- *(decision)*: a choice to make and record in DONE before the dependent work starts.
- *(variant)*: an alternative built next to the main design so the two can be compared.
- *(skipped)*: set aside by the user, with date; stays open as a record.

An item is done when its acceptance criteria pass, its tests and documents are in the same PR, and
DONE has an entry for it.

---

## 1. Version 0.1: the local `//` shell

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

- [ ] 2.1 User smoke test against a real model ([SMOKE](SMOKE.md)).
  - [x] 2.1.1 `scripts/smoke.sh` for any `ush` (`--bin`; Python or Go): offline checks, model
    checks without a terminal (read-only request, disclosure, declined change, injected
    instructions, piped input, dry run, replay), `--paid` for Anthropic, a report; manual checks
    M1–M10 in SMOKE. Acceptance: tests against the mock API, including a failing implementation
    and failing requests; a local Ollama run.
  - [ ] 2.1.2 A human runs it with their Anthropic key (`--paid`) and the manual checks, and records
    `smoke-report.md` in DONE.
- [ ] 2.2 Verify macOS keychain access under `sudo` and document the outcome.
  Acceptance: manual run of `sudo ush ai status` with a keychain-stored key.
- [ ] 2.3 First green run of `.github/workflows/ci.yml` on a PR (Linux and macOS,
  Python 3.10/3.13); configure branch protection once `main` exists remotely.
- [ ] 2.4 Extend the read-only command table from real sessions (Linux package
  managers, `podman`, `kubectl get`, `zfs list`, …) with negative tests per command.
- [ ] 2.5 Compaction or context editing for long sessions; measure token use.
- [ ] 2.6 Enforce read-only execution in agent mode with an OS sandbox (GAPS Q8, first of the
  agent-mode work). The commands agent mode runs without asking run without write access (apart
  from a private temporary directory) and without network: macOS `sandbox-exec` (Seatbelt),
  Linux bubblewrap or Landlock, as Claude Code's and Codex CLI's sandboxes do (docs/SOTA.md). Then
  "read-only" is enforced, not only inferred by the policy, and an injected read (F8) can't leave
  the machine. Approved commands and propose mode run unsandboxed, as the user's own commands.
  Acceptance: a write and a network connection by an auto-approved command fail on macOS and
  Linux; where the sandbox can't be set up, auto-approval is off and every command asks (fail
  closed, with the reason); SEC and SECTEST updated; ushgo TODO 2.11 ports it.
- [ ] 2.7 Align the `//` contract and policy semantics with wish once wish's shell
  integration (wish 2.1/6.2) exists.
- [x] 2.8 Local models through Ollama's Anthropic-compatible API: provider setting
  and `USH_PROVIDER`/`USH_MODEL`/`USH_OLLAMA_URL` (not as root), loopback-only
  endpoint, no proxies or redirects, cloud-model disclosure, no key; `.env.template`
  and `just ush` for development. Acceptance: mock-API request shape, misconfiguration
  and remote-URL refusals before any request, honest banner; live run against a local
  Ollama; SEC updated.
- [x] 2.9 Neutralise control characters (C0/C1, ANSI/OSC, bidi overrides, zero-width)
  in everything ush writes to the terminal: commands and scripts in the approval
  prompt, command output, and answers (SECTEST F1). Acceptance: rendered-bytes tests
  and a pty test show a `\r`/escape-sequence command visibly; no raw sequence reaches
  the terminal.
- [x] 2.10 Close DNS exfiltration through auto-approved `dig`/`host`/`nslookup`
  (SECTEST F2), e.g. approval for names the model chose. Acceptance: negative policy
  tests; SEC residual risk updated.
- [x] 2.11 Fill the remaining gaps in [SECTEST.md](SECTEST.md): injection corpus
  and opt-in model-in-the-loop evals (free on a local Ollama model), tokenizer
  differential fuzzing against zsh, a GTFOBins sweep, redaction vectors, redirect,
  `USH_BASE_URL` under root, planted symlinks, binary output, and oversized stdin.
  Acceptance: each listed gap has a committed test or a documented reason.
- [x] 2.12 Model menu in `.env.template`: `USH_MODEL` choices per provider by size (Anthropic
  tiers; Ollama tags with download size and the memory they need, checked against the Ollama
  library), and `just ush` drops a trailing ` # comment` after a value. Acceptance: every listed
  model name passes ush's validation; uncommenting a menu line works.
- [x] 2.13 Checker model: an optional second model (`provider:model`, `checker` setting,
  `USH_CHECKER`, `-k`) judges each proposed command or script against the user's requests only,
  never output, piped input, or the main model's reasoning; a flag, an unclear reply, or an error
  makes the command need approval; OK never approves; disclosed in the banner and `ai status`.
  Acceptance: agent, parser, settings, provider (`complete`) and end-to-end tests; a live run on a
  local model recorded in SECTEST; SEC updated.
- [ ] 2.14 A man page, `ush.1`, for the command in both implementations: synopsis, the `//`
  forms, options, subcommands, environment (`USH_*`), files, what leaves the machine, exit
  statuses, examples. One source in the repository; `install.sh` installs it to
  `share/man/man1`, and the Go release archive carries it (ushgo TODO 2.4). Acceptance: `man ush`
  after either install; a test that every option in `ush help` appears in the page.
- [x] 2.15 User allowlist: `ush allow [add|remove RULE]`, `allow` in the settings file. A rule
  (command name and leading words) lets commands the table doesn't know run without approval;
  refused for table commands, runners and interpreters, commands that change or send data, and
  shell syntax; not used as root; history and the prompt name the rule. No blacklist mode (ARCH,
  GAPS Q5). Ported by ushgo TODO 1.11. Acceptance: policy, settings, agent and CLI tests,
  including refusals and root; SEC and SECTEST updated.
- [x] 2.16 User and admin guides for ushpy ([USER](USER.md), [ADMIN](ADMIN.md)), sharing their
  command sections with ushgo's (ushgo TODO 0.4). Acceptance: `test_docs` fails if a shared
  section differs between the two, or if an option or subcommand in `ush help` is missing.
- [ ] 2.17 Propose mode, the default (ARCH, GAPS Q6): ush proposes one command at a time and runs
  nothing; the zsh plugin puts the proposal on the command line and records what the user runs
  there (command line, exit status, output through `tee` unless interactive); `//` continues the
  saved per-shell conversation with it; `--run` and `mode: run` keep agent mode; `--new` starts
  over; proposals with control characters are refused. Ported by ushgo TODO 1.12.
  - [x] 2.17.1 Implementation and automated tests: agent, handoff, CLI end to end, the plugin in a
    real zsh, smoke checks; ARCH, SEC, guides, READMEs, SMOKE (M11–M15).
  - [ ] 2.17.2 Manual checks M11–M18 by the user with a real model, and a live smoke run in
    propose mode.
- [x] 2.18 Plugin setup and Esc: both installers offer to add the plugin to `~/.zshrc`
  (`shell/zshrc.sh`: asks only from a terminal, `--zshrc`/`--no-zshrc`, never twice or as root);
  Esc removes a proposal from the command line and is bound only while one is there, restoring
  the user's own binding afterwards; without the plugin, ush prints the line to add. Acceptance:
  `test_install` (each mode, a real terminal answer), the plugin in a real zsh (Esc, the binding
  kept after running, removing and abandoning a proposal), the release archive carries the script.
- [x] 2.19 Explain a command: `ush explain COMMAND…` sends the command line (and, for the pending
  proposal, the user's last request and its purpose) to the model as one request without tools,
  shows ush's policy verdict first, and runs nothing; Down on a proposal, past its last line and
  at the newest history line, runs it from the plugin, bound only while the proposal is there.
  Acceptance: CLI test against the mock API (context, redaction, no tools, nothing run or
  touched), the Down key in a real zsh (explains, the proposal stays and runs, Down elsewhere and
  its binding unchanged).
- [x] 2.20 Explanations above the command, and proposals pre-filled in a session. Down shows the
  explanation above the proposal as display-only text (zle `PREDISPLAY`), with the cursor still
  in the command; Esc closes it first, a second Esc removes the proposal, Enter runs it, and the
  text is gone when the line is accepted. The `ush>` session pre-fills its proposal with zsh's
  `vared` and the same keys, because Python's readline can't pre-fill with libedit (macOS).
  Acceptance: the plugin in a real zsh (explanation before the command, edit with it shown, Esc
  twice, Enter runs the edited command); the session editor in a pty (pre-filled and edited, Esc,
  Down and Esc).
- [x] 2.21 Brief explanations: the model answers in a fixed form (Summary, Parts, Access, Notes);
  by default only the parts and the access (files and network, read and write, anonymization
  where it applies) are shown, and `-v` (`ush -v`, `ush -v //…`, carried by the proposal, or
  `ush explain -v`) adds the banner, ush's verdict, the summary, the notes, and the key hint. The
  progress text and the explanation take the prompt's place above the command, without leftover
  lines; `{}` is literal in the policy, so `find -exec … {} +` names `-exec`. Acceptance: CLI
  tests (brief exactly the parts, verbose in full, hint only for the key, the proposal's -v), the
  order on a full redraw in a real zsh and in the session editor, policy tests for `{}`.
- [x] 2.22 Arrow keys in the session, Up on an explanation: the `ush>` session reads every line
  with zsh's `vared` (history of the session's lines, Ctrl-D ends it); before, lines without a
  proposal went to Python's plain `input()`, which echoed `^[[A`. Up closes an explanation, in
  the session and in the plugin, and is history otherwise. Acceptance: a real `ush` session in a
  pty (Up recalls and reruns a line, Ctrl-D ends it), Up in the plugin and the session editor.
- [ ] 2.23 Injected read-only commands in agent mode (SECTEST F8, GAPS answer in ushgo Q4): run
  the injection eval's DNS TXT scenario with a checker (llama3.2:3b and a tool-using model) and
  record whether it flags the injected `cat`. If it doesn't, agent mode asks before a command
  whose arguments occur only in earlier command output or piped input, not in the user's
  requests. The eval runs each scenario several times and reports rates. Acceptance: F8's scenario
  ran unapproved 0 times in at least 20 runs; tests for the new rule and its false positives
  (`ls` then `cat` of a listed file); ushgo TODO 2.8 ports it.
- [x] 2.24 Explain once per command line (user, 2026-10-29): Down after Up or Esc closed an
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
  ignores a broken or linked file; ushgo TODO 2.9 ports the session editor and the store.
- [ ] 2.25 Notes on command output and files (user, 2026-10-29: "add the possibility to analyze
  command output and files. this could be a shortcut like ctrl-n like notes"; ROAD's
  `ush analyze`). The key is Ctrl-X n (GAPS Q7, answered); blocked on Q7's second part, which
  output. Proposed:
  - **Ctrl-X n** on the command line asks the model for notes and shows them above the line, as an
    explanation is shown. A comment on the line is the instruction (`cat app.log # any errors?`);
    without one, the model summarises and points out what stands out.
  - **Command output:** on an empty line, notes on the output of the last command the plugin
    recorded. On a command line, the command runs as a proposal would, with its output recorded,
    and then gets notes.
  - **Files:** words on the line that name readable regular files send those files as context.
  - **Command line:** `ush notes [-v] [INSTRUCTION] FILE…`, also with piped input, for scripts and
    shells without the plugin.
  - **Limits:** the same disclosure, redaction and 200 KB limit as piped input; files in secret
    locations (the policy's list) and binary files are refused with the reason; never under
    `sudo`.

  Acceptance: zsh pty tests with a fake provider (Ctrl-X n on an empty line, on a command, on a
  line naming files, with a comment as instruction); the refusals (secret file, binary, too
  large, sudo) and redaction of file content; `ush notes` with files and piped input; no other
  binding changed; SEC and SECTEST rows for files sent
  to the provider; ushgo TODO 2.10 ports it.
- [ ] 2.26 History as context (ROAD idea, scheduled 2026-10-29): with the plugin, a request
  carries the shell's recent history (`fc -ln`, at most the last N lines), so "repeat that with
  these changes" works. The lines are filtered before they leave: secrets redacted, lines that
  read secret files (the policy's list) or carry tokens dropped, ush's own `//` lines dropped.
  Off by default; a setting turns it on, and the disclosure banner says so. Acceptance: zsh pty
  tests with a fake provider (history reaches the request when on, never when off, filtered
  lines absent); redaction tests with synthetic secrets; SEC, SECTEST, USER updated; ushgo TODO
  2.12 ports it.
- [ ] 2.27 bash (ROAD idea, scheduled 2026-10-29): a bash plugin with the zsh plugin's behaviour
  as far as bash allows: `// …` without quotes (a `DEBUG` trap or `bind -x` on Enter), the
  proposal pre-filled (`bind -x` sets `READLINE_LINE`, bash 4 or newer), output recorded,
  Esc and Down on a proposal, the installer offering `~/.bashrc`. The `ush>` session keeps
  zsh's `vared` where zsh exists. Acceptance: the zsh plugin's pty tests, ported to bash 4 and
  5 (macOS ships bash 3.2: the plugin refuses it with the reason); ushgo TODO 2.13 ports the
  installer part.
- [ ] 2.28 fish (ROAD idea, scheduled 2026-10-29): the same in fish (`commandline`, `bind`,
  `fish_preexec`/`fish_postexec`), after bash. Acceptance: as for 2.27, in fish 3; ushgo TODO
  2.14.
- [ ] 2.29 More model platforms (user, 2026-10-29: "widen the compatibility with external models:
  openai, mistral, huggingface, any other important platforms"). One adapter for the OpenAI Chat
  Completions API with tool calling, which most platforms serve. The agent keeps its `Provider`
  protocol (ARCH).
  - **Presets**, each a known host checked against the platform's docs when implemented:
    `openai`, `mistral`, `huggingface` (the Inference Providers router), `gemini` (Google's
    OpenAI-compatible endpoint), `groq`, `openrouter`, `together`, `deepseek`, `azure` (the
    user's resource URL).
  - **Any other endpoint:** `openai-compatible` with an explicit URL, `https` or loopback `http`.
    Local servers (llama.cpp, LM Studio, vLLM) get Ollama's loopback rules: no proxies, no
    redirects, "stays on this machine".
  - **Keys:** per platform, in the keychain (account = preset) or the 0600 file, or from the
    platform's usual variable (`OPENAI_API_KEY`, `MISTRAL_API_KEY`, `HF_TOKEN`, …) as for
    Anthropic, never as root. The key is redacted from everything sent, and those variables are
    removed from the commands ush runs.
  - **Disclosure:** the banner names the platform and host. `ush ai configure` offers the presets,
    and the checker can use any of them (`provider:model`).
  - **Acceptance:**
    - a local mock of Chat Completions, streamed and not, with tool calls split across chunks;
      errors, retries and rate limits; tests for every preset's URL and key source, the
      loopback rules, key redaction, and root ignoring the variables;
    - a live smoke test and the injection eval on at least one OpenAI-compatible local server,
      free; paid platforms only with explicit configuration;
    - SEC (new destinations), ADMIN (network table), USER and SOTA updated;
    - ushgo TODO 2.15 ports it.
- [ ] 2.30 Cloud platforms that need their own signing, after 2.29: AWS Bedrock (SigV4) and
  Google Vertex AI (service accounts), for Anthropic and other models there. Where a platform's
  OpenAI-compatible endpoint covers tool calling, 2.29's adapter is used instead. Acceptance: as
  for 2.29, with signing tested against fixed vectors; credentials from the platform's standard
  chain, never logged; ushgo TODO 2.16.

## 3. Release

- [ ] 3.1 Release 0.1 gate per [AGENTS.md](../../../AGENTS.md): fresh suite on the exact
  candidate on each supported OS/Python, dependency audit, release notes, human merge.
