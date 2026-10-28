# DONE

Progress log for [TODO](TODO.md), newest first. Each entry starts with the date and time of day
(local time) and a title, then the TODO items and the PR, then the agent that did the work as
`Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or `Agent: human`), then
**Implementation notes** (with the exact validation and its results), **Deviations** and
**Insights**. Entries are never rewritten; later outcomes are new entries on top. The format is
defined in [SDLC](../../docs/SDLC.md#done-entries).

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
