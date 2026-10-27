# ush implementation plan

Read with [CLAUDE.md](../CLAUDE.md), [DESIGN.md](docs/DESIGN.md), and
[SEC.md](docs/SEC.md). IDs are stable ush task numbers, independent of wish's.
Mark an item complete only after its acceptance checks pass and a DONE entry
records the implementation, validation, deviations, and learnings.

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

- [ ] 2.1 Opt-in live smoke test against the real API (`USH_LIVE_TEST=1`, costs
  money). Acceptance: a human runs it with their key; result recorded in DONE.
- [ ] 2.2 Verify macOS keychain access under `sudo` and document the outcome.
  Acceptance: manual run of `sudo ush ai status` with a keychain-stored key.
- [ ] 2.3 First green run of `.github/workflows/ci.yml` on a PR (Linux and macOS,
  Python 3.10/3.13); configure branch protection once `main` exists remotely.
- [ ] 2.4 Extend the read-only command table from real sessions (Linux package
  managers, `podman`, `kubectl get`, `zfs list`, …) with negative tests per command.
- [ ] 2.5 Compaction or context editing for long sessions; measure token use.
- [ ] 2.6 Enforce read-only execution with an OS sandbox (macOS `sandbox-exec`, Linux
  Landlock/namespaces) for auto-approved commands. Acceptance: a write attempt by an
  auto-approved command fails; update SEC.
- [ ] 2.7 Align the `//` contract and policy semantics with wish once wish's shell
  integration (wish 2.1/6.2) exists.
- [x] 2.8 Local models through Ollama's Anthropic-compatible API: provider setting
  and `USH_PROVIDER`/`USH_MODEL`/`USH_OLLAMA_URL` (not as root), loopback-only
  endpoint, no proxies or redirects, cloud-model disclosure, no key; `.env.template`
  and `just ush` for development. Acceptance: mock-API request shape, misconfiguration
  and remote-URL refusals before any request, honest banner; live run against a local
  Ollama; SEC updated.
- [ ] 2.9 Neutralise control characters (C0/C1, ANSI/OSC, bidi overrides, zero-width)
  in everything ush writes to the terminal: commands and scripts in the approval
  prompt, command output, and answers (SECTEST F1). Acceptance: rendered-bytes tests
  and a pty test show a `\r`/escape-sequence command visibly; no raw sequence reaches
  the terminal.
- [ ] 2.10 Close DNS exfiltration through auto-approved `dig`/`host`/`nslookup`
  (SECTEST F2), e.g. approval for names the model chose. Acceptance: negative policy
  tests; SEC residual risk updated.
- [ ] 2.11 Fill the remaining gaps in [SECTEST.md](docs/SECTEST.md): injection corpus
  and opt-in model-in-the-loop evals (free on a local Ollama model), tokenizer
  differential fuzzing against zsh, a GTFOBins sweep, redaction vectors, redirect,
  `USH_BASE_URL` under root, planted symlinks, binary output, and oversized stdin.
  Acceptance: each listed gap has a committed test or a documented reason.

## 3. Release

- [ ] 3.1 Release 0.1 gate per [CLAUDE.md](../CLAUDE.md): fresh suite on the exact
  candidate on each supported OS/Python, dependency audit, release notes, human merge.
