# ushgo, ush compiled

ushgo is the Go port of [ush](../ush/README.md), the local `//` shell for zsh. The goal is the same
behaviour as one static binary: no interpreter or installed packages for root to trust under `sudo`,
and no two-second start while a Python SDK loads. ush stays the reference implementation until
ushgo passes ush's black-box tests as a conformance suite.

**Status:** project set up (TODO 0.1, 0.2). The binary builds and tests run in CI, but it answers no
requests yet: `ushgo --version` and `ushgo --help` work, and everything else is refused as a usage
error. The port's plan is in [TODO](docs/TODO.md).

## Build and test

ushgo needs Go 1.27 (`brew install go`) and nothing else; it has no dependencies yet.

```sh
just ushgo-test                     # from the repository root: gofmt, go vet, go test
cd ushgo && go build ./cmd/ushgo    # writes ./ushgo
```

`scripts/test.sh` sets `GOTOOLCHAIN=local`, so Go never downloads another toolchain, and
`CGO_ENABLED=0`, so the binary is static.

## Documents

- [ARCH](docs/ARCH.md): how the port maps ush's modules to Go packages, and why Go.
- [SEC](docs/SEC.md): what changes from [ush's threat model](../ush/docs/SEC.md) with a compiled binary.
- [TODO](docs/TODO.md) and [ROAD](docs/ROAD.md): the port's plan and phases.
- [GAPS](docs/GAPS.md): questions for the user, such as whether ushgo replaces ush.
- [DONE](docs/DONE.md): progress log, newest first.

The process is the repository's [SDLC](../docs/SDLC.md); TODO and GAPS numbers are ushgo's own.

## Layout

```text
cmd/ushgo/     the command (main.go) and its tests
scripts/       test.sh: format, vet, test
docs/          ARCH, SEC, TODO, ROAD, GAPS, DONE
```
