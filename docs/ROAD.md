# Roadmap

The repository's roadmap across its projects: what comes next, in which order, and what gates each
step. Each project keeps its own phases and items ([ush ROAD](../ush/docs/ROAD.md),
[wish ROAD](../wish/docs/ROAD.md)); this file orders them and
holds the items that belong to no single project. Work becomes items in a project's TODO when it
starts. Status: 2026-10-29 02:01.

## Overview

| Step | What | Where | Gate |
| --- | --- | --- | --- |
| 0. Repository (done but CI) | Land the open stack; one default branch; CI; licence | Below | All PRs on the default branch; licence chosen |
| 1. First ush release (done) | The `//` shell for zsh, safe to hand to others | ush TODO 2.9–2.11, 5.3, 5.5 | Release gate: no open High finding, suites on macOS and Linux, smoke test on the artifact |
| 2. ush in Go (done) | The same `ush` as a static Go binary, the deliverable | ush TODO 4.1–4.12, 5.1–5.7 | ushgo passes ushpy's black-box tests and the smoke test; releases then ship it |
| 3. ush for daily use | Propose mode; fewer approvals for harmless commands, enforced read-only, long sessions | ush TODO 2.4–2.6, 2.17, 2.23–2.25 | Weeks of daily use without a policy bypass; approvals per request falling |
| 4. wish phase 1 | A lean browser workspace: files, pipes, one notebook, `//` | wish TODO 1–4, 6, 9.1–9.4 | wish ROAD phase 1 acceptance; release gate |
| 5. One `//` paradigm | The same request contract and policy semantics in ush and wish | ush TODO 2.7 | wish has a shell (wish TODO 2.1, 6.2) |

Steps 1 and 2 are done: the port was finished on 2026-10-28, and the first release was 0.2.0-rc1
of the Go binaries on 2026-10-29. Each ush feature now lands in Python first or in both at once,
and its TODO item stays open until Go has it ([SDLC](SDLC.md#ush-one-command-two-implementations)).
Step 4 starts when the user decides (wish GAPS Q1).

## 0. Repository

- **Land the stack:** done 2026-10-28. PRs #3–#6 had merged into each other's branches, not the
  default branch; [#12](https://github.com/gregoryyy/ushell/pull/12) landed them with #7–#11 as one
  PR. From now on, a PR targets the default branch unless its base is merged first; turn on
  automatic deletion of head branches so a stacked PR is retargeted when its base merges.
- **One default branch named `main`:** done 2026-10-29, as [AGENTS.md](../AGENTS.md) and SDLC
  assume.
- **CI on GitHub again,** when Actions starts jobs for the account; until then `just ci --post`,
  which leaves the Linux matrix entries unchecked. A ush release needs one recorded Linux run
  either way.
- **Licence:** MIT, decided 2026-10-28 (ush GAPS Q1); wish's licence question is wish GAPS Q4.

## 1. ush 0.1

In order: fix F1, terminal escape sequences in the approval prompt (ush TODO 2.9); close F2, DNS
exfiltration by auto-approved lookups (2.10); fill the security test gaps (2.11). Then the paid smoke
test and manual checks by the user (2.1.2), the keychain under sudo (2.2), distribution (ush GAPS
Q2), a man page (2.14), and the release gate (3.1).

Done: the fixes landed on 2026-10-28; the first release was 0.2.0-rc1 of the Go binaries
(ush TODO 5.5), with SECTEST F8 documented (ush GAPS Q12). The paid smoke test, the keychain
check and the man page remain open.

## 2. ush in Go

Ported on 2026-10-28, in dependency order: policy, redaction, execution, configuration, history,
provider, agent, terminal, command line, conformance (ush TODO 4.1–4.12); ushpy's black-box tests
pass against the Go binary. Release builds for macOS and Linux on
arm64 and amd64 (ush TODO 5.1; the deterministic build exists, distribution on macOS is ush
GAPS Q11), the man page (5.4), then the release gate on the binaries (5.3). Since 0.2.0-rc1, a ush
release ships the Go binary.

## 3. ush for daily use

- Propose mode as the default: ush proposes, the user runs from their command line, `//`
  continues with what ran; agent mode behind `--run`. Done in both (ush TODO 2.17, 4.12); the
  manual checks are open. With it, Esc removes a proposal and Down, or
  `ush explain`, explains it (ush TODO 2.18, 2.19).
- A user allowlist of commands that run without approval, on top of the built-in read-only table:
  done in both (ush TODO 2.15, 4.11). No blacklist mode; deny rules that only
  add friction may follow (ush GAPS Q5).
- A broader built-in command table from real sessions (ush TODO 2.4), each command with negative
  tests.
- ush stays small and focused on a manual workflow; no larger agent (ush GAPS Q8). For agent
  mode, first an OS sandbox for what it runs without asking (ush TODO 2.6): no writes, no
  network; `sandbox-exec` on macOS, bubblewrap or Landlock on Linux. Then argument origin (2.23):
  a command whose arguments come only from earlier output asks first.
- History as context (ush TODO 2.26), filtered and off by default; bash (2.27) and fish (2.28).
- More model platforms (ush TODO 2.29, 2.30): OpenAI, Mistral, Hugging Face, Gemini, Groq,
  OpenRouter, Together, DeepSeek, Azure and local OpenAI-compatible servers through one adapter;
  then AWS Bedrock and Google Vertex AI.
- Compaction or context editing for long sessions (2.5).
- Whether the checker model is on by default (ush GAPS Q4).

## 4. wish phase 1

wish's lean browser workspace (wish ROAD phase 1), after the just-bash spike (wish TODO 2.1). Open
for the user: when to start, which browsers, which providers (wish GAPS Q1–Q3).

## 5. One `//` paradigm

What counts as a request, what history records, what replay does, and how the policy decides, the
same in ush and wish (ush TODO 2.7).

## Ideas not scheduled

- **Analyse files with a set of instructions:** `ush analyze "INSTRUCTIONS" FILE…`, e.g.
  `ush analyze "list the commands this script runs; flag destructive ones" deploy/*.sh`. The
  instructions apply to each file in turn, with a result per file. The files go to the provider as
  context, under the same redaction and disclosure as piped input. Scheduled with Ctrl-X n for
  notes as ush TODO 2.25.
- **Data loss prevention on the way to the provider:** filters on everything ush sends (requests,
  command output, piped input, files) that remove credentials and system internals (host names,
  internal addresses, user and path names beyond what the request needs). This would go further
  than today's pattern redaction. The adversarial checker would also review outgoing data for
  leaks. Today it never sees command output, so that injected text can't steer it (ush ARCH). A
  leak check needs to see that output, so it would be a separate role that can only answer "leak"
  or "no leak" and only block, never approve.
- **One policy corpus** for both ush implementations and later wish: command lines with expected
  decisions, as the shared contract (ush ROAD).
