# TODO

Plan for ushgo, the Go port of ush: the approach is in [ARCH](ARCH.md), what changes for security
in [SEC](SEC.md), the phases in [ROAD](ROAD.md), open questions in [GAPS](GAPS.md), and progress in
[DONE](DONE.md). The process is [SDLC](../../docs/SDLC.md); the rules in
[CLAUDE.md](../../CLAUDE.md) take precedence. IDs are ushgo's own, independent of ush's and wish's.
Status: 2026-10-28 09:59.

Status: `[ ]` open, `[x]` done. Tags:
- *(decision)*: a choice to make and record in DONE before the dependent work starts.
- *(variant)*: an alternative built next to the main design so the two can be compared.
- *(skipped)*: set aside by the user, with date; stays open as a record.

Each port item is done when the package reproduces the ush module's behaviour, its tests are ported
(ush's test names in the Go test names, so the two can be compared), and DONE has an entry.

---

## 0. Project setup

- [x] 0.1 Project documents: README, ARCH (packages, Go over Rust), SEC (what a compiled port
  changes), TODO, ROAD, GAPS, DONE.
- [x] 0.2 Go module (Go 1.27, no dependencies), `cmd/ushgo` that answers `--version` and `--help`
  and refuses everything else, `scripts/test.sh` (gofmt, vet, test), `just ushgo-test`, a CI job
  on Ubuntu and macOS, and `actions/setup-go` in the local CI. Acceptance: tests for the CLI stub
  and the built binary; the local CI maps the new job; `just ci` passes.
- [x] 0.3 The command is `ush` (GAPS Q2): `cmd/ush`, `ush --version` prints `ush X.Y.Z (go)`;
  equivalence with the Python implementation as the rule
  ([SDLC](../../docs/SDLC.md#ush-one-command-two-implementations)). Acceptance: tests for the
  name and version; the user smoke test (`ush/scripts/smoke.sh --bin`) runs against the Go binary
  and reports the checks it doesn't pass yet.

## 1. Port

In dependency order: the packages that don't depend on others first.

- [ ] 1.1 `internal/policy`: tokenizer, command table, validators, secret paths. *(decision)* Port
  ush's tokenizer or use `mvdan.cc/sh` (ARCH proposes the port). Acceptance: a shared corpus of
  command lines with expected decisions (JSON, generated from ush's policy tests) passes in both
  implementations; a fuzz test of the tokenizer against `zsh -f` finds no disagreement on a line
  the policy accepts.
- [ ] 1.2 `internal/redact`. Acceptance: ush's redaction tests ported; the synthetic tokens stay in
  `.gitleaksignore`.
- [ ] 1.3 `internal/execute`: `zsh -f -o pipefail`, `/dev/null` stdin, process groups, timeouts,
  interrupt, bounded output, scrubbed environment, private script files. Acceptance: ush's
  execution tests ported, including the pipeline that ignores SIGTERM.
- [ ] 1.4 `internal/config`: identity under sudo, settings, keychain (secret on stdin) or 0600
  file, environment overrides ignored as root. Acceptance: ush's config tests ported; settings
  written by one implementation read by the other.
- [ ] 1.5 `internal/history`: append-only JSONL, `history`, `replay` through the policy.
  Acceptance: ush's history tests ported; history files interchangeable.
- [ ] 1.6 `internal/provider`: Anthropic through `anthropic-sdk-go` (streaming, eager tool input,
  fallbacks, progress updates) and Ollama through the same API (loopback only, no proxies or
  redirects, cloud-model disclosure). Acceptance: request shape and errors against ush's mock
  Messages API (`ush/tests/mock_api.py`); SDK licence and maturity in ARCH.
- [ ] 1.7 `internal/agent`: the loop with strict tools, schema validation, step limits, every
  `tool_use` answered. Acceptance: ush's agent tests ported with a scripted fake provider.
- [ ] 1.8 `internal/ui`: status lines and the approval prompt via `/dev/tty`. Acceptance: pty tests;
  the display-integrity rules of ush TODO 2.9 apply here from the start.
- [ ] 1.9 `cmd/ush`: the full CLI (requests, sessions, piped input, options, `ai`, `history`,
  `replay`). Acceptance: ush's CLI tests ported; the smoke test passes offline.
- [ ] 1.10 Conformance: ush's black-box tests (`test_cli`, `test_zsh_plugin`, the pty approval
  tests) run against a binary chosen by an environment variable, and pass for both. Acceptance: a
  CI job runs them against the Go binary; the smoke test passes with a local model.

## 2. Release

- [ ] 2.1 Static release builds for macOS and Linux on arm64 and amd64 (`-trimpath`,
  `CGO_ENABLED=0`), an installer for a root-owned copy, checksums. Acceptance: reproducible from
  the tag on a second machine.
- [x] 2.2 *(decision)* Which implementation is maintained after parity: both, Go as the
  deliverable, Python as the experimental implementation (GAPS Q2, 2026-10-28).
- [ ] 2.3 Release gate per [CLAUDE.md](../../CLAUDE.md) and [SEC](SEC.md): conformance on each
  supported OS, ush SECTEST levels 1 and 2 on the binary, `govulncheck`, secret scan.
