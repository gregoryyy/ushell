# ushgo security

ushgo inherits [ush's threat model](../../ushpy/docs/SEC.md) unchanged: the same assets, trust
boundaries, threats and controls, and the same open findings in
[ush SECTEST](../../ushpy/docs/SECTEST.md). This document records only what a compiled port changes.
Status: 2026-10-27 15:03; the binary answers no requests yet, so none of the controls below is in
effect.

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
| **Behavioural drift.** The two implementations decide differently on a command line, so a command ush asks about runs unasked in ushgo. | One corpus of command lines with expected decisions, run against both policies; differential fuzzing (`go test -fuzz`) of the tokenizer against `zsh -f`. A difference that auto-approves where ush asks fails the build. | 1.1, 1.10 |
| **Go toolchain and modules.** A dependency or the toolchain is compromised. | `go.sum` pins every module; Go's checksum database verifies them; `govulncheck` before each release; `GOTOOLCHAIN=local` in tests and CI, so no toolchain is fetched. Each dependency's licence and maturity in [ARCH](ARCH.md). | 2.3 |
| **Release binaries.** A binary differs from the source. | Reproducible builds (`-trimpath`, fixed `-ldflags`), checksums published with the release; the release gate in [AGENTS.md](../../AGENTS.md) runs on the built binaries. | 2.1, 2.3 |
| **Shared files with ush.** ushgo writes settings, keys or history that ush then trusts. | The same validation as ush on every read (0600 files, owner, no symlinks); the conformance tests cover the file formats. | 1.4, 1.5 |

## Release gate

Every release of the Go binary runs ush's release gate on it: the conformance suite on each
supported OS, the adversarial tests in ush SECTEST (levels 1 and 2), `govulncheck`, a secret scan,
and the policy corpus against both implementations.
