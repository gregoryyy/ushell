# DONE

Progress log for [TODO](TODO.md), newest first. Each entry starts with the date and time of day
(local time) and a title, then the TODO items and the PR, then the agent that did the work as
`Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or `Agent: human`), then
**Implementation notes** (with the exact validation and its results), **Deviations** and
**Insights**. Entries are never rewritten; later outcomes are new entries on top. The format is
defined in [SDLC](../../docs/SDLC.md#done-entries).

---

## 2026-10-28 23:40 — The port: ush in Go, at parity with ushpy

TODO items: 1.1–1.12. PR: see below.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request:** "the go version shall have feature parity with the python version", for the
  0.2.0-rc1 release of the Go binaries.
- **Packages:** `internal/` holds each ushpy module's counterpart:
  - `policy`, `redact`, `execute`, `config` (settings and key store), `history`;
  - `handoff` (propose mode's files and conversations), `check` (the checker);
  - `provider` (Messages API and Ollama), `ui` (terminal, approval, the `vared` session
    editor), `agent` (both modes);
  - `cmd/ush` has the full command line: requests, `//`, sessions, piped input, all options,
    `ai`, `allow`, `explain`, `history`, `replay`.
- **Exact behaviour:** prompts, tool schemas, usage text and messages are ushpy's word for word;
  a script compared the system prompts and the explanation prompt. The files are ushpy's, in the
  same formats: settings, key store, history, handoff directory, saved conversations.
- **No dependencies:** the provider is a streaming client on `net/http`, about 400 lines (ARCH).
  Terminal checks and echo control use `ioctl`, per OS.
- **Policy parity:**
  - `conform/policy.json` holds 1545 command lines with ushpy's verdicts and reasons, over a
    fixture tree with a fake home: ushpy's policy tests, an escape-vector sweep, allow rules, and
    1500 seeded random lines.
  - ushgo's `TestConformance` compares every case; planting one wrong reason made 8 cases fail.
  - `ushpy/scripts/policy_corpus.py` writes the corpus, and ushpy's `test_conform` fails if it is
    out of date.
- **Black-box parity:** `test_cli`, `test_security`, `test_smoke` and `test_zsh_plugin` start ush
  through `USH_TEST_BIN`. `ushgo/scripts/conform.sh` builds the Go binary and runs them against it
  (`just ushgo-conform`, part of `just ush-test`, and the new CI job `ushgo-conform`).
- **Validation:**
  - `go test ./...`: every package OK, on macOS; `go vet` for macOS and Linux.
  - Conformance against the Go binary: all 63 tests OK. That covers `test_cli` (16) and
    `test_security`, `test_smoke` and `test_zsh_plugin` (47), among them the pty approval prompt,
    propose mode with handoff and continuation, sessions with `vared` and Up, explanations, the
    injection eval harness, and the smoke test against a scripted Ollama.
  - Live: `smoke.sh --bin` on the Go binary with llama3.2:3b on local Ollama gave 14 PASS and
    1 WARN (the model proposed no command).

### Deviations

- **Own HTTP client:** replaces `anthropic-sdk-go` (planned in 1.6); the reasons are in ARCH.
- **Tokenizer fuzzing:** ushgo's tokenizer is not fuzzed against zsh directly. It equals ushpy's on
  the 1545 corpus lines, and ushpy's is fuzzed against zsh.
- **Test hook:** `ui.Terminal.Ask` lets the agent tests answer approval prompts.
- **Smoke test:** it now runs `//` only after the model proposed something, and warns otherwise.
  A weak model that proposes nothing had made the check fail, in either implementation.

### Insights

- The corpus found F7 in ushpy, a crash on `cat < 2>/dev/null`, before the Go side passed.
  Comparing two implementations tests both.
- RE2's lack of lookaround cost a small sed-substitution parser. Its linear time removes a whole
  class of findings (F3).
- Running the black-box tests against both binaries makes parity a CI result, not a claim.

---

## 2026-10-28 10:23 — User and admin guides

TODO items: 0.4; ushpy TODO 2.16. PR: [#12](https://github.com/gregoryyy/ushell/pull/12) (stacked on #11).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- [USER](USER.md): what ush does, first steps, asking (forms, quoting, piped input), approving
  (when a command runs unasked, the four answers), the allowlist, the checker, options, other
  commands, what leaves the machine, sudo, use without a terminal and exit statuses,
  troubleshooting.
- [ADMIN](ADMIN.md): requirements per implementation, install from a release archive (checksums,
  `install.sh`, macOS quarantine), from source (deterministic build), Python, root-owned installs
  for `sudo`; configuration (files, every setting with default and range, environment variables);
  behaviour under sudo; network destinations; verifying with `ush ai status` and the smoke test;
  upgrade, rollback, switching implementation, removal; security notes (F1, F2, `PATH`).
- Both open with what the Go implementation does today (`--version`, `--help`) and that the rest is
  the command's behaviour as ushpy implements it and ushgo will. Linked from ushgo's and the root
  README.
- The same guides for ushpy (ushpy TODO 2.16): the sections about the command are marked
  `shared:begin`/`shared:end` and identical in `ushpy/docs/` and `ushgo/docs/`, with links that
  work from both; each copy has its own banner, and ADMIN its own install and upgrade sections.
  `ushpy/tests/test_docs.py` fails if a shared section differs or an option or subcommand from
  `ush help` is missing from the user guide.
- **Validation:** facts checked against `ushpy/src/ush` (options and subcommands in `cli.py`,
  settings ranges and file checks in `config.py`, exit statuses, `SECRET_ENV_PREFIXES`, the plugin
  and installer paths); `python3 tools/check-links.py`: no broken links.

### Deviations

- The guides live in `ushgo/docs/` because ushgo is the deliverable, but describe behaviour only the
  Python implementation has today; they say so at the top rather than describing a stub.

### Insights

- Writing the admin guide surfaced no system-wide configuration: every setting is per user. For
  managed machines that may become a question (e.g. a fixed provider or a policy-wide allowlist).

---

## 2026-10-28 10:17 — Deterministic release build

TODO items: 2.1.1 (2.1 stays open for 2.1.2, a second machine, and 2.1.3, macOS distribution). New:
2.4 (man page in the archive), GAPS Q3. PR: [#10](https://github.com/gregoryyy/ushell/pull/10)
(stacked on #9).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- `tools/release` (Go, standard library only): builds `cmd/ush` for darwin and linux on arm64 and
  amd64 (`CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, `-s -w -buildid=`, version stamped),
  packs each with `install.sh`, `README.txt` and the zsh plugin from `ushpy/shell/` into
  `ush-VERSION-OS-ARCH.tar.gz` with `archive/tar` and `compress/gzip` (members sorted, owner 0, no
  names, timestamps from `-epoch`, no gzip name or time), writes `SHA256SUMS`; `-check` builds twice
  and compares. It finds the module root itself.
- `scripts/release.sh VERSION [--check]` and `just ushgo-release`: epoch from the commit time;
  refuses a real build with uncommitted changes in `ushgo/` or the plugin. `ushgo/dist/` ignored.
- `packaging/install.sh`: the same places as ushpy's installer, atomic replace of the binary, mode
  755/644; `packaging/README.txt`: contents, checksum check, macOS quarantine note.
- Docs: README (release build), ARCH (decision: Go's archive writer, flags, install places), SEC
  (release-binaries row), GAPS Q3 (macOS: Homebrew tap proposed vs. signing and notarization),
  TODO 2.1.1–2.1.3 and 2.4; ushpy TODO 2.14 (man page); root ROAD.
- **Validation:** `scripts/test.sh`: ok; `tools/release` tests: packing deterministic and ordered,
  epoch matters, owners and times fixed, member and directory names without paths, versions
  checked, two full builds identical, the host binary from the archive prints `ush 1.2.3 (go)`, the
  installer into a scratch prefix leaves nothing writable by others (13.7 s). `scripts/release.sh
  0.0.1 --check`: "reproducible: two builds are identical". Build of all four: 1.9 s (warm cache);
  archives 0.65–0.75 MB; the darwin/arm64 binary 1.6 MB, linked only to `libSystem` and
  `libresolv`. `just ci`: see the PR.

### Deviations

- Not yet tested: the same checksums on a second machine (2.1.2); the Linux CI job will show it
  when Actions runs, since the test builds twice there too, but not across machines.

### Insights

- The stub binary is 1.6 MB against a 69 MB Python venv; with the Anthropic SDK and the policy the
  binary will grow, but stays one file that root can own.
- Shell `tar` would have tied the archive bytes to the build machine's tar; the standard library
  removes that dependency at the cost of about 120 lines.

---

## 2026-10-28 09:59 — The command is `ush`; Go is the deliverable

TODO items: 0.3, 2.2. GAPS Q2 resolved. PR: [#8](https://github.com/gregoryyy/ushell/pull/8) (stacked on #7).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- `cmd/ushgo` moved to `cmd/ush`; `ush --version` prints `ush 0.0.0-dev (go)`, `help` works as in
  Python; the usage text says this implementation doesn't answer requests yet.
- The user's answers (GAPS Q2): the command is `ush` in both implementations; Python is the
  experimental one, Go the compiled deliverable, with equivalent functionality. Recorded in ARCH
  (decision, replacing the binary name `ushgo`), ROAD (releases ship the Go binary), README, and
  the repository's SDLC ("ush: one command, two implementations").
- **Validation:** `scripts/test.sh`: ok (3 tests, now asserting the `ush` name, the `(go)` version
  line, and `help`). `../ush/scripts/smoke.sh --bin <go build>`: version, help, usage error and
  no-session PASS; `ai status` and `history` FAIL, as expected before TODO 1.9.

### Deviations

- None.

### Insights

- One name for both implementations means they can't be installed side by side under it; the
  tests and the smoke test take the binary's path instead (`--bin`).

---

## 2026-10-27 15:03 — ushgo set up: documents, Go module, CI

TODO items: 0.1, 0.2. PR: [#5](https://github.com/gregoryyy/ushell/pull/5) (stacked on #4).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **Documents:** [README](../README.md), [ARCH](ARCH.md) (Go over Rust, ush modules mapped to
  `internal/` packages, conformance through ush's black-box tests, the tokenizer decision for TODO
  1.1), [SEC](SEC.md) (what a static binary changes: no interpreter for root to trust; new risks of
  drift between the two implementations, the Go toolchain and release binaries),
  [TODO](TODO.md) (setup, ten port items in dependency order, release), [ROAD](ROAD.md),
  [GAPS](GAPS.md) (Q1 resolved: Go; Q2 whether ushgo replaces ush).
- **Code:** `go.mod` (Go 1.27, no dependencies), `cmd/ushgo/main.go`: `--version` (stamped with
  `-ldflags "-X main.version=…"`) and `--help`; everything else is a usage error (exit 2, "use ush"),
  so nothing runs by accident. `scripts/test.sh`: gofmt check, `go vet`, `go test` with
  `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`, `CGO_ENABLED=0`.
- **Tests (3):** version and help output; requests refused as usage errors; the built binary's
  version stamping and exit status.
- **CI:** an `ushgo` job in `ci.yml` (Ubuntu and macOS, `actions/setup-go` v7.0.0 pinned by
  commit, `go-version: "1.27"`, no cache). The local CI maps `actions/setup-go` to the machine's Go
  if `go env GOVERSION` matches the requested version or prefix (`go_matches` in
  `tools/ci/workflow.py`, with a test), and sets `GOTOOLCHAIN=local`. `just ushgo-test` runs the
  script. `just pdfs` without paths includes `ushgo/`.
- **Toolchain:** Go 1.27.3 (BSD-3-Clause) installed with Homebrew on the development machine; it
  was not installed before.
- **Validation:** `ushgo/scripts/test.sh`: ok. `go test -count=1 -v ./...`: 3 passed. Local CI
  tests (`uv run … pytest` in `tools/ci`), `just links` and `just ci`: see the PR.

### Deviations

- None from the plan; the plan was written in this change.

### Insights

- Checked before choosing: `anthropic-sdk-go` is Apache-2.0 and at v2.3.0 (2026-10-23), in step with
  the Python SDK version ush pins; `mvdan.cc/sh` is BSD-3-Clause but parses POSIX shell and bash, not
  zsh, which is why ARCH proposes porting ush's tokenizer instead.
