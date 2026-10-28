# Roadmap

Where the Go port goes. Each phase ends with a gate that decides whether the next one starts; the
work is in [TODO](TODO.md). The port is phase 3 of [ush's roadmap](../../ushpy/docs/ROAD.md).
Status: 2026-10-28 09:59.

| Phase | Goal | TODO | Gate to the next phase |
| --- | --- | --- | --- |
| 1. Set up | A Go project that builds and tests in CI | 0 (done) | — |
| 2. Parity | Every ush module ported, the conformance suite passing | 1 | ush's black-box tests and the policy corpus pass against both binaries on macOS and Linux |
| 3. Release | Static `ush` binaries people install, instead of the Python one | 2 | The release gate and the smoke test on the binaries |

## When to start phase 2

Port against a stable ush: after ush's 0.1 release, so the display-integrity and DNS fixes (ush
TODO 2.9, 2.10) are part of what is ported, not a second change later. The policy package (TODO
1.1) can start earlier, because its corpus also tests ush.

## Ideas not scheduled

- **One policy, two implementations, one corpus:** the corpus from TODO 1.1 could become the shared
  `//` policy contract that ush TODO 2.7 wants for wish.
- **Faster start-up as a feature:** a binary that starts in milliseconds could run the zsh plugin's
  checks on every line, e.g. warning before a mistyped command runs.
