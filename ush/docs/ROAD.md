# Roadmap

Where ush goes after version 0.1. The phases build on each other, and each one ends with a gate:
a tested result that decides whether the next phase starts. When a phase starts, its work becomes
items in [TODO](TODO.md), and this file links to them. Open questions are in [GAPS](GAPS.md).
Status: 2026-10-27 14:52.

| Phase | Goal | TODO | Gate to the next phase |
| --- | --- | --- | --- |
| 1. Release 0.1 | The local `//` shell, safe to hand to others | 1 (done), 2.9–2.11, 3.1 | The release gate in [CLAUDE.md](../../CLAUDE.md) and [SECTEST](SECTEST.md): no open High finding, the suite on macOS and Linux, model-in-the-loop results recorded; licence and distribution decided (GAPS Q1, Q2) |
| 2. Daily use | Fewer approvals for harmless commands, enforced read-only, long sessions | 2.1–2.6 | A few weeks of daily use without a policy bypass; approvals per request measured and falling |
| 3. Compiled port | A single static binary with the same behaviour | [ushgo TODO](../../ushgo/docs/TODO.md) | The port passes ush's black-box tests as a conformance suite; then decide which implementation is maintained |
| 4. Shared paradigm with wish | One `//` contract and policy semantics for both shells | 2.7 | wish's shell integration (wish TODO 2.1, 6.2) exists |

## 1. Release 0.1

Fix the display-integrity finding F1 (TODO 2.9) and the DNS exfiltration finding F2 (2.10), fill
the security test gaps (2.11), then run the release gate (3.1). The live smoke test against the paid
API (2.1) and the keychain check under sudo (2.2) are manual steps for the user.

## 2. Daily use

- Extend the read-only command table from real sessions (2.4), each command with negative tests.
- An OS sandbox for auto-approved commands (2.6), so "read-only" is enforced rather than inferred:
  `sandbox-exec` profiles on macOS, Landlock or namespaces on Linux.
- Compaction or context editing for long sessions (2.5).

## 3. Compiled port

A port to Go, [ushgo](../../ushgo/README.md): one static binary, no interpreter or installed
packages for root to trust, and no two-second SDK start. The Python implementation stays the
reference until the port passes the same black-box tests. Rust was considered: stronger guarantees for the command parser, but no official
Anthropic SDK and more effort for the process and terminal work.

## 4. Shared paradigm with wish

Align the `//` contract (what counts as a request, what history records, what replay does) and the
policy semantics with wish once wish has a shell (2.7).

## Ideas not scheduled

- **More shells:** bash and fish integrations next to the zsh plugin (GAPS A3).
- **More providers:** OpenAI-compatible local servers (llama.cpp, LM Studio) through the same
  loopback rules as Ollama.
- **A command explainer:** `ush explain CMD` that runs nothing and describes what a command line
  would do, using the policy's own parse.
