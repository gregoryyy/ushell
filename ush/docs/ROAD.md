# Roadmap

Where ush goes, in both implementations. The phases build on each other, and each one ends with a gate:
a tested result that decides whether the next phase starts. When a phase starts, its work becomes
items in [TODO](TODO.md), and this file links to them. Open questions are in [GAPS](GAPS.md).
Status: 2026-10-29 21:55; phases 1 and 3 are done, 0.2.1 is released; 0.2.2 is next (TODO 5.10).

| Phase | Goal | TODO | Gate to the next phase |
| --- | --- | --- | --- |
| 1. First release (done) | The local `//` shell, safe to hand to others | 1, 2.9–2.11 (done); 5.3, 5.5 | The release gate in [AGENTS.md](../../AGENTS.md) and [SECTEST](SECTEST.md): no open High finding, the suite on macOS and Linux, model-in-the-loop results recorded; licence and distribution decided (GAPS Q1, Q2) |
| 2. Daily use | Fewer approvals for harmless commands, enforced read-only in agent mode, long sessions | 2.6, 2.23, 2.4, 2.5, 2.24, 2.25 | A few weeks of daily use without a policy bypass; approvals per request measured and falling; F8's scenario at 0 in 20 |
| 3. Compiled port (done) | The same `ush` as a single static Go binary, the deliverable | 0, 4 (done); 5 | The Go binary passes ush's black-box tests and the smoke test; releases then ship it |
| 4. Context, shells, platforms | Requests that see recent history; bash and fish; more model platforms | 2.26–2.30 | The plugin's pty tests pass in each shell; history filtered as SEC requires; each platform's mock tests and one free live run |
| 5. Shared paradigm with wish | One `//` contract and policy semantics for both shells | 2.7 | wish's shell integration (wish TODO 2.1, 6.2) exists |

## 1. First release

Fix the display-integrity finding F1 (TODO 2.9) and the DNS exfiltration finding F2 (2.10), fill
the security test gaps (2.11), then run the release gate. The live smoke test against the paid
API (2.1) and the keychain check under sudo (2.2) are manual steps for the user.

Done: the findings were fixed on 2026-10-28, and the first release was 0.2.0-rc1 of the Go binaries
on 2026-10-29 (TODO 5.5), with SECTEST F8 open and documented (GAPS Q12), then 0.2.0 from the
same commit, published as a GitHub release (5.8); the Homebrew tap's repository is the user's to
push (5.7). Version 0.1 of the Python
implementation was never released (3.1).

## 2. Daily use

ush stays small and focused on a manual workflow: no larger agent (GAPS Q8). Agent mode gets
the guarantees it lacks, in this order:
1. An OS sandbox for what agent mode runs without asking (2.6), so "read-only" is enforced, not
   only inferred: no writes, no network; `sandbox-exec` on macOS, bubblewrap or Landlock on Linux.
2. Argument origin (2.23): a command whose arguments come only from earlier output asks first.

Then, for everyone:
- Extend the read-only command table from real sessions (2.4), each command with negative tests.
- Explanations kept per command line (2.24); notes on output and files with Ctrl-X n (2.25).
- Compaction or context editing for long sessions (2.5).

## 3. Compiled port

The same `ush` in Go, [ush/go](../go/README.md): one static binary, no interpreter or
installed packages for root to trust, and no two-second SDK start. Python is the experimental
implementation, where features are tried first; Go is the deliverable, and both must behave the same
(decided 2026-10-28, [SDLC](../../docs/SDLC.md#ush-one-command-two-implementations)). Rust was
considered: stronger guarantees for the command parser, but no official Anthropic SDK and more
effort for the process and terminal work (GAPS Q9).

Done 2026-10-28: every module ported (TODO 4.1–4.12), the policy corpus and ushpy's black-box tests
passing against the Go binary on macOS and Linux. Releases ship the Go binary (TODO 5): 0.2.0 is
out (5.8) as a GitHub release (5.6), then 0.2.1 with the man page (5.9), both in the Homebrew tap (5.7). Every new feature now lands in both implementations (TODO, section 2).

## 4. Context and shells

- History as context (2.26): recent commands with a request, filtered, off by default.
- bash (2.27), then fish (2.28): the plugin's behaviour in each, as far as the shell allows.
- More model platforms (2.29): one OpenAI-compatible adapter with presets for OpenAI, Mistral,
  Hugging Face, Gemini, Groq, OpenRouter, Together, DeepSeek and Azure, any other endpoint by
  URL, and local servers under Ollama's loopback rules. Then Bedrock and Vertex (2.30).

## 5. Shared paradigm with wish

Align the `//` contract (what counts as a request, what history records, what replay does) and the
policy semantics with wish once wish has a shell (2.7).

## Ideas not scheduled

- **One policy contract for ush and wish:** the corpus in `ush/conform/` could become the shared
  `//` policy contract that TODO 2.7 wants for wish.
- **Faster start-up as a feature:** the Go binary starts in milliseconds, so the zsh plugin could
  run checks on every line, e.g. warn before a mistyped command runs.
