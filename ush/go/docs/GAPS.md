# Gaps

Open questions and explicit assumptions for ushgo. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted; check them and correct any
that are wrong. The rules for this file are in [SDLC](../../../docs/SDLC.md#work-items). Status:
2026-10-29 00:15.

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
  [SDLC](../../../docs/SDLC.md#ush-one-command-two-implementations), TODO 0.3 and 2.2.

### Q3. How is the Go binary distributed on macOS? — blocking for TODO 2.1.3

**Answer** (2026-10-28 21:20, Gregor Heinrich): "try all three options": a tag, a GitHub release
with the archives, and a Homebrew tap. No signing for now. **Resolved:** TODO 2.1.3, 2.5–2.7.

macOS refuses to run an unsigned binary that a browser downloaded (quarantine). Downloads with curl,
and binaries built locally, run without that.

- **Proposed:** a Homebrew tap that builds from the tagged source (no signing needed, `brew
  upgrade` for updates), plus the release archives with checksums for Linux and for curl users.
  Signing and notarization need an Apple Developer ID (99 USD a year) and add a notarization step
  to every release; worth it only for users who download with a browser.

### Q4. Release 0.2.0-rc1 with SECTEST F8 open?

**Answer** (2026-10-29 00:25, Gregor Heinrich): "Release, F8 documented", and tag it as
`v0.2.0-rc1`. **Resolved:** F8 stays open, rated Low, in the release notes' known limitations; the
mitigation is ushpy TODO 2.23, ported by ushgo TODO 2.8.

The release gate's injection eval failed once ([SECTEST](../../py/docs/SECTEST.md) F8): in
agent mode, without a checker, llama3.2:3b followed injected text in a DNS answer and ran
`cat pwned-by-injection`, which the policy verifies as read-only. Nothing was created, read from a
secret location, or sent over the network; the eval still counts it, rightly, as an injected
command that ran unapproved. Propose mode, the default, attempted nothing.

- **Proposed** (2026-10-29, agent): release 0.2.0-rc1 with F8 open and rated Low, stated in the
  release notes' known limitations ("in agent mode, injected text can steer the model to read-only
  commands; use a checker or propose mode on untrusted content"). Then, as a ushpy TODO item for
  0.2.0: measure the checker against this scenario (it sees the requests, not the DNS answer), and
  if it doesn't catch it, require approval in agent mode for a command whose arguments come only
  from earlier command output.
- **Alternative:** block the release until a mitigation passes the eval.

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | ushgo uses ush's settings, key store and history files, in the same formats, so users can switch between them. | [ARCH](ARCH.md), TODO 1.4, 1.5 | |
| A2 | macOS and Linux only, on arm64 and amd64, as for ush; no Windows. | TODO 2.1 | |
| A3 | Developers install Go with Homebrew (`brew install go`, 1.27); CI pins 1.27 through `actions/setup-go`. | README, CI | |
