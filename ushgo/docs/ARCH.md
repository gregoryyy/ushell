# ushgo architecture

How ush is ported to Go: the package layout, how behaviour is kept equal, and the decisions behind
it. ush's own architecture, which this port reproduces, is in [ush ARCH](../../ushpy/docs/ARCH.md).
Status: 2026-10-28 09:59, project set up; no module ported yet.

## Why a port, and why Go

The analysis of 2026-10-27 (recorded in [GAPS](GAPS.md) Q1) compared Go and Rust for ush's
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

## Principles

- **ush is the specification.** Behaviour comes from ush's code and tests, not from a new design.
  A deliberate difference is recorded here and in ush's documents.
- **The same files.** ushgo reads and writes ush's settings (`~/.config/ush/config.json`), key store
  (keychain service or 0600 file) and history (`~/.local/state/ush/history.jsonl`) in the same
  formats, so the two can be swapped (GAPS A1).
- **Conformance by black-box tests.** ush's tests that drive the program from outside (the CLI
  against the mock Messages API, the approval prompt in a pty, the zsh plugin) run against the
  ushgo binary as well (TODO 1.10). Policy decisions are compared on a shared corpus of command
  lines (TODO 1.1).
- **Provider independence,** as in ush: `policy`, `execute`, `redact` and `history` don't import
  the provider package; `agent` sees only an interface.

## Packages

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

Only `cmd/ush` exists today. The zsh plugin (`ushpy/shell/ush.plugin.zsh`) is shared unchanged: both
implementations are the command `ush`.

## Decisions

- **2026-10-27: Go, in `ushgo/` of this repository.** The user left the choice between `ushrs`
  and `ushgo` open (GAPS Q1).
- **2026-10-28: the command is `ush`, and Go is the deliverable** (GAPS Q2). Both implementations
  build the command `ush`; `ush --version` names the implementation (`(go)`, `(python)`). Python is
  the experimental implementation where features are tried first; Go is what users install; both
  must behave the same. This replaces the binary name `ushgo` of 2026-10-27, which existed so both
  could be installed side by side: to compare them, install one and pass the other's path to the
  tests (`--bin`, conformance).
- **2026-10-27: Go 1.27, no dependencies until a package needs one.** Each dependency is pinned in
  `go.sum`, with its licence and maturity recorded here when it is added: the Anthropic SDK with
  the provider (TODO 1.6), `golang.org/x/term` with the approval prompt (TODO 1.8).
- **Open *(decision)*, TODO 1.1: port ush's tokenizer or use `mvdan.cc/sh`.** `mvdan.cc/sh` parses
  POSIX shell and bash, not zsh: zsh's glob qualifiers, `=cmd` expansion and other word rules
  differ. Porting ush's own tokenizer (about 110 lines, built for exactly the `zsh -f` rules the policy
  needs) keeps the decisions equal by construction. Proposed: port it, and use `mvdan.cc/sh` only if
  the differential tests show the port can't keep up.
- **2026-10-28: release builds with Go's own archive writer** (`tools/release`, TODO 2.1.1).
  `tar` differs between macOS (bsdtar) and Linux (GNU tar) in headers and options, so archives
  built on two machines would differ; `archive/tar` and `compress/gzip` from the standard library
  give the same bytes everywhere. Binaries: `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`,
  `-ldflags "-s -w -buildid= -X main.version=…"`. Archive members: the binary, `install.sh`, the
  zsh plugin from `ushpy/shell/` (one plugin for both implementations) and a README; timestamps
  are the commit time. The installer uses the same places as ushpy's (`~/.local/bin`,
  `~/.local/share/ush`; `PREFIX/bin`, `PREFIX/lib/ush`), so the plugin line in `~/.zshrc` stays
  valid when switching implementations. On macOS a Go binary links `libSystem` and `libresolv`
  from the system, as every macOS program does; nothing else.
- **CI:** the `ushgo` job in [ci.yml](../../.github/workflows/ci.yml) runs `scripts/test.sh` on
  Ubuntu and macOS with `actions/setup-go`; the local CI maps that action to the machine's Go when
  its version matches ([tools/ci/README.md](../../tools/ci/README.md)).
