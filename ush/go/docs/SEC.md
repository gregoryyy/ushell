# ushgo security

ushgo inherits [ush's threat model](../../py/docs/SEC.md) unchanged: the same assets, trust
boundaries, threats and controls, and the same open findings in
[ush SECTEST](../../py/docs/SECTEST.md). This document records only what a compiled port changes.
Status: 2026-10-28 23:45; the port is complete, and ushpy's security tests that run ush pass against
the Go binary (`ush/go/scripts/conform.sh`).

## What gets better

- **Nothing for root to trust but the binary.** ush under `sudo` runs a root-owned Python and a venv
  of installed packages, and has to keep `PYTHON*` variables and user site directories out
  (`python -I`). A static Go binary (`CGO_ENABLED=0`) has no interpreter, no import path and no
  dynamic libraries beyond the system's. A root-owned binary is the whole install.
- **Memory safety** is the same as Python's; the parser and redaction run on untrusted text in a
  memory-safe language either way.

## What is new

| Risk | Control | TODO |
| --- | --- | --- |
| **Behavioural drift.** The two implementations decide differently on a command line, so a command ush asks about runs unasked in ushgo. | `ush/conform/policy.json`: 1545 command lines with ushpy's verdicts and reasons, compared in full by ushgo's `TestConformance`; ushpy's `test_conform` keeps the corpus current, and ushpy's tokenizer is fuzzed against `zsh -f`. ushpy's black-box tests run against the Go binary (`ushgo-conform`). Any difference fails the build. The corpus found F7 in ushpy. | 1.1, 1.10 (done) |
| **Go toolchain and modules.** A dependency or the toolchain is compromised. | ushgo has no dependencies: standard library only, including the Messages API client (ARCH). `govulncheck` before each release covers the standard library and the toolchain; `GOTOOLCHAIN=local` in tests and CI, so no toolchain is fetched. | 2.3 |
| **An HTTP client of our own.** It mishandles TLS, redirects, or a broken stream. | `net/http` with default TLS verification; no redirects followed; for Ollama, no proxy; a reply cut off mid-stream is an error, not a crash (F6); retries only for connection errors and 408/409/429/5xx. Tests against a mock server, a redirecting server and a server that drops the connection. | 1.6 (done) |
| **Patterns on untrusted text.** Redaction or validation stalls on crafted input. | Go's RE2 matches in linear time; no backtracking pattern can stall (ushpy F3). | 1.2 (done) |
| **Release binaries.** A binary differs from the source, or an archive is altered. | Deterministic builds (`tools/release`: `-trimpath`, no build ID, no symbols, fixed archive order, owners and times from the commit), so anyone can rebuild a tag and compare `SHA256SUMS`; the installer writes nothing writable by others. Unsigned on macOS (GAPS Q3). | 2.1 (2.1.1 done), 2.3 |
| **Shared files with ush.** ushgo writes settings, keys or history that ush then trusts. | The same validation as ush on every read (0600 files, owner, no symlinks); the conformance tests cover the file formats. | 1.4, 1.5 |

## Release gate

Every release of the Go binary runs ush's release gate on it: the conformance suite on each
supported OS, the adversarial tests in ush SECTEST (levels 1 and 2), `govulncheck`, a secret scan,
and the policy corpus against both implementations.
