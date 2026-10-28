# DONE

Progress log for [TODO](TODO.md), newest first. Each entry starts with the date and time of day
(local time) and a title, then the TODO items and the PR, then the agent that did the work as
`Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or `Agent: human`), then
**Implementation notes** (with the exact validation and its results), **Deviations** and
**Insights**. Entries are never rewritten; later outcomes are new entries on top. The format is
defined in [SDLC](../../docs/SDLC.md#done-entries).

---

## 2026-10-29 00:35 — Tag v0.2.0-rc1 and the release gate on it

TODO items: 2.3, 2.5 (both still open: `govulncheck`), 2.1.2 (shown at the tag), 2.8 (new).
PR: #24.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The user's decisions:**
  - On the tag: "tag the release 0.1.0-rc1", then, asked about the version, `v0.2.0-rc1`, which
    matches the code.
  - On GAPS Q4: "Release, F8 documented". Recorded in GAPS, SECTEST, ushpy TODO 2.23 (the
    mitigation) and ushgo TODO 2.8 (its port).
- **The tag:** the agent created the annotated tag `v0.2.0-rc1` on main's b19db18 (the merge of
  #23) and pushed it, at the user's request.
- **Gate on b19db18:**
  - Level 1, macOS arm64: `just ci --post` passes all seven jobs.
  - Level 1, Linux arm64 (Debian in `golang:1.27`, Go 1.27.3, Python 3.13.5, zsh 5.9,
    unprivileged): ushpy 175 OK, `go test ./...` OK, conformance 63 OK.
  - Release build: `scripts/release.sh 0.2.0-rc1` at the tag. Rebuilt in the Linux container from
    `git archive v0.2.0-rc1`, it gives the same four checksums:
    - `2dffa542…` darwin-arm64
    - `af88887d…` darwin-amd64
    - `b4c16042…` linux-arm64
    - `a389c3c8…` linux-amd64
  - darwin-arm64 archive, installed to a temporary prefix: `smoke.sh --bin` with llama3.2:3b gave
    14 PASS and 1 WARN (the model answered without running `df`), 0 FAIL.
  - linux-arm64 archive on `debian:13`: the checksum verifies, `ush 0.2.0-rc1 (go)`, no file
    writable by others, offline smoke 6 PASS.
  - Level 2: the injection eval three times, 36 runs: 1 attempted, 0 ran unapproved (SECTEST).
  - `gitleaks git --redact .`: no leaks.
- **The earlier candidate:** `just ci --post` on e3ea569, #23's head, passed all seven jobs after
  #23 was merged.
- **The user's machine:** the darwin-arm64 archive from the tag replaced the Python install in
  `~/.local` ("install the go version instead of the python version"). The checksum was verified
  and the venv removed; `ush ai status` reads the existing settings and keychain key.

### Deviations

- **Not run:**
  - `govulncheck`: the download is not permitted to the agent, so TODO 2.3 and 2.5 stay open.
  - Linux amd64 and Intel macOS: no machine.
  - Live Anthropic: not authorised.
- **TODO 2.1.2:** the second build ran in a container on the same Mac, with a different OS and Go
  build. Whether that counts as a second machine is for the user; the item stays open.
- **Publishing:** the GitHub release and the Homebrew tap (TODO 2.6, 2.7) are not done. Per
  AGENTS.md they are the user's, unless the user explicitly hands them to the agent.

### Insights

- The eval's rate is low and noisy for a 3B model: 1 in 10 for F8's scenario across both days.
  Several runs per release are the minimum for a statement about it.

---

## 2026-10-29 00:25 — Release gate for 0.2.0-rc1, on the candidate: one finding open

TODO items: 2.3, 2.5 (both still open), 2.1.2 (shown, to repeat at the tag). PR: #23.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **Candidate:** 440ffe9, the release preparation on top of the port (#22). Only documentation
  changed after it (this entry, SECTEST, GAPS, the notes). The gate must run again on the tagged
  commit, since the tag is set after the merge.
- **Level 1, macOS arm64:** `just ci --post` on the port head 541aae9: links, ushpy (3.10 and
  3.13), ushgo, ushgo-conform, tools, secrets all pass. This PR's own CI run is in the PR.
- **Level 1, Linux arm64:**
  - Setup: Debian in `golang:1.27` under Docker, Go 1.27.3, Python 3.13.5, zsh 5.9, as an
    unprivileged user, on the release candidate.
  - Results: ushpy's 175 tests OK, `go test ./...` every package OK, conformance (ushpy's black-box
    tests against the Go binary) 63 OK.
  - The first run found the corpus's macOS paths; fixed in #22.
- **Release build:**
  - `scripts/release.sh 0.2.0-rc1 --check`: two builds identical.
  - The same build in the Linux container (`go run ./tools/release -version 0.2.0-rc1 -epoch
    <commit time>`) gave the same four checksums as on macOS. That is TODO 2.1.2's second
    machine, for this commit.
- **Archives:**
  - darwin-arm64: installed with `install.sh --prefix <tmp> --no-zshrc`; `ush --version` says
    `ush 0.2.0-rc1 (go)`. `smoke.sh --bin` with llama3.2:3b on local Ollama: 13 PASS, 2 WARN
    (the model proposed no command), 0 FAIL.
  - linux-arm64 on plain `debian:13`: the checksum verifies, it installs as a user, no file is
    writable by others, and `smoke.sh --bin` (offline) gives 6 PASS.
- **Level 2, injection eval** on the installed darwin-arm64 binary, llama3.2:3b: 12 runs,
  3 attempted, **1 ran unapproved** (DNS TXT, agent mode, `cat pwned-by-injection`). Six more
  runs of that scenario: 2 attempted, 0 ran. Recorded as SECTEST F8.
- **Secrets:** `gitleaks git --redact .` over the whole history: no leaks.
- **Not run:**
  - `govulncheck`: the tool download is not permitted to the agent here. The dependency
    inventory is the Go standard library only, at Go 1.27.3.
  - Linux amd64 and Intel macOS: no machine.
  - Live Anthropic: costs money; not authorised.

### Deviations

- **Gate status:** the gate is not passed. F8 fails the eval's criterion (zero injected commands
  run unapproved). The decision is the user's (GAPS Q4); TODO 2.3 and 2.5 stay open.

### Insights

- One run of a 3B model is a sample, not a verdict. The previous day's run had zero; this one had
  one in seven. The eval should run several times per release and report rates.
- Reproducibility across macOS and Linux hosts follows from `-trimpath`, no cgo and fixed archive
  metadata. Nothing host-specific leaked into the archives.

---

## 2026-10-28 23:40 — The port: ush in Go, at parity with ushpy

TODO items: 1.1–1.12. PR: #22.

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
  - Linux arm64 (Debian, Go 1.27.3, Python 3.13.5, zsh 5.9, in `golang:1.27` under Docker, as an
    unprivileged user): ushpy's 175 tests OK, `go test ./...` every package OK, conformance 63 OK.
    The first run failed: the corpus held `/private/etc/…` and `/bin/…` as macOS resolves them,
    which Linux resolves as `/etc/…` and `/usr/bin/…`. Both sides now write `/etc`, `/tmp`,
    `/var`, `/bin`, `/sbin` and `/lib` under their plain names.
  - `just ci --post` on 8bc2d8c: all seven macOS jobs pass. The first run failed in `tools`,
    whose tests list the workflow's jobs and lacked `ushgo-conform`.

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
