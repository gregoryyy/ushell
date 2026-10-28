# Gaps

Open questions and explicit assumptions for ushgo. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted; check them and correct any
that are wrong. The rules for this file are in [SDLC](../../docs/SDLC.md#work-items). Status:
2026-10-28 10:17.

## Questions

### Q1. Go or Rust?

A compiled port of ush, as `ushrs` (Rust) or `ushgo` (Go). The comparison is in
[ARCH](ARCH.md#why-a-port-and-why-go).

- **Proposed** (2026-10-27, agent): Go, for the official Anthropic SDK and less effort for the
  process and terminal work.
- **Answer** (2026-10-27 14:52, Gregor Heinrich): "add the compiled stack project as ushrs or
  ushgo"; the choice left to the agent.
- **Resolved** (2026-10-27 15:03, agent): Go, as `ushgo/`; [ARCH](ARCH.md#decisions). A different
  answer would mean a new `ushrs/` project; nothing beyond the skeleton depends on the choice yet.

### Q2. Does ushgo replace ush once it reaches parity?

Two implementations of a security-critical policy double the review work and can drift apart.

- **Proposed** (2026-10-27, agent): yes. ushgo takes over the name `ush` and the install, after one
  release in which both are available and the conformance suite passes for both. The Python
  implementation then stays only as the conformance reference, or is retired.
- **Answer** (2026-10-28 09:59, Gregor Heinrich): "the command for ushgo is ush as well, it's just
  another implementation." And: "ush and ushgo should have equivalent functionality. py is the
  experimental version, go the compiled deliverable."
- **Resolved** (2026-10-28 09:59, agent): both are maintained; the command is `ush` in both; Python
  is experimental, Go the deliverable, with equivalent functionality. [ARCH](ARCH.md#decisions),
  [SDLC](../../docs/SDLC.md#ush-one-command-two-implementations), TODO 0.3 and 2.2.

### Q3. How is the Go binary distributed on macOS? — blocking for TODO 2.1.3

macOS refuses to run an unsigned binary that a browser downloaded (quarantine). Downloads with curl,
and binaries built locally, run without that.

- **Proposed:** a Homebrew tap that builds from the tagged source (no signing needed, `brew
  upgrade` for updates), plus the release archives with checksums for Linux and for curl users.
  Signing and notarization need an Apple Developer ID (99 USD a year) and add a notarization step
  to every release; worth it only for users who download with a browser.

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | ushgo uses ush's settings, key store and history files, in the same formats, so users can switch between them. | [ARCH](ARCH.md), TODO 1.4, 1.5 | |
| A2 | macOS and Linux only, on arm64 and amd64, as for ush; no Windows. | TODO 2.1 | |
| A3 | Developers install Go with Homebrew (`brew install go`, 1.27); CI pins 1.27 through `actions/setup-go`. | README, CI | |
