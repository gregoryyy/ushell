# ushgo: ush in Go

ushgo is the Go implementation of `ush`, the local `//` shell for zsh: the same command as the
Python implementation in [ushpy/](../ushpy/README.md), and the one users will install. One static
binary: no interpreter or installed packages for root to trust under `sudo`, and no two-second start
while a Python SDK loads. Python is where features are tried first; ushgo gets each of them, and the
two must behave the same ([SDLC](../docs/SDLC.md#ush-one-command-two-implementations)).

**Status:** set up (TODO 0.1–0.3). The binary builds and tests run in CI, but it answers no requests
yet: `ush --version` (`ush 0.0.0-dev (go)`) and `ush --help` work, and everything else is refused
as a usage error. The port's plan is in [TODO](docs/TODO.md).

## Build and test

ushgo needs Go 1.27 (`brew install go`) and nothing else; it has no dependencies yet.

```sh
just ushgo-test                         # from the repository root: gofmt, go vet, go test
cd ushgo && go build -o ush ./cmd/ush   # writes ./ush
```

### Release build

```sh
just ushgo-release 0.1.0                # or scripts/release.sh 0.1.0: archives in ushgo/dist/
scripts/release.sh 0.1.0 --check        # build twice, compare, keep nothing
```

One `ush-VERSION-OS-ARCH.tar.gz` per platform (macOS and Linux, arm64 and amd64), each with the
binary, `install.sh`, the zsh plugin and a README, plus `SHA256SUMS`. The build is deterministic:
the same commit and version give the same checksums on any machine, so anyone can check a release
against the source. It takes about two seconds for all four with a warm build cache; an archive is about 0.7 MB today. To
install from an archive: `tar -xzf ush-….tar.gz && ush-…/install.sh` (or `sudo … --prefix
/usr/local`). Publishing a release is a human's step.

`scripts/test.sh` sets `GOTOOLCHAIN=local`, so Go never downloads another toolchain, and
`CGO_ENABLED=0`, so the binary is static.

## Documents

- [USER](docs/USER.md): the user guide: asking, approving, the allowlist, the checker, what leaves
  the machine, troubleshooting.
- [ADMIN](docs/ADMIN.md): the admin guide: install and verify releases, root-owned installs for
  `sudo`, settings and files, network, upgrade and removal.
- [ARCH](docs/ARCH.md): how the port maps ush's modules to Go packages, and why Go.
- [SEC](docs/SEC.md): what changes from [ush's threat model](../ushpy/docs/SEC.md) with a compiled binary.
- [TODO](docs/TODO.md) and [ROAD](docs/ROAD.md): the port's plan and phases.
- [GAPS](docs/GAPS.md): questions for the user and assumptions.
- The user smoke test is shared with Python: [SMOKE](../ushpy/docs/SMOKE.md), `../ushpy/scripts/smoke.sh --bin ./ush`.
- [DONE](docs/DONE.md): progress log, newest first.

The process is the repository's [SDLC](../docs/SDLC.md); TODO and GAPS numbers are ushgo's own.

## Layout

```text
cmd/ush/       the command (main.go) and its tests
scripts/       test.sh: format, vet, test
docs/          ARCH, SEC, TODO, ROAD, GAPS, DONE
```
