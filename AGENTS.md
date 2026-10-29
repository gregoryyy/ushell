# ushell development process

These instructions apply to human and AI contributors throughout this repository, whatever
the agent ([CLAUDE.md](CLAUDE.md) only points here).
It holds projects that share the `//` command paradigm:

- **ush**, the local command-line shell extension for zsh. It runs real commands on
  the user's machine, possibly as root. One command, two implementations with
  equivalent functionality ([SDLC](docs/SDLC.md#ush-one-command-two-implementations)):
  Python in [ush/py/](ush/py/), the experimental one where features are tried first, and
  Go in [ush/go/](ush/go/), the compiled deliverable.
- **wish** ([wish/](wish/)), the browser workspace shell. It is still a design
  project; commands described in `wish/docs/` are proposed behavior until
  implemented and tested.

The workflow, the document set (TODO, DONE, ARCH, ROAD, SEC, GAPS in each project's
`docs/`), and the DONE entry format are defined in [docs/SDLC.md](docs/SDLC.md). The
rules below take precedence over it. The repository's roadmap across projects is
[docs/ROAD.md](docs/ROAD.md). Each project has independent `x.x` IDs; refer to them
as "ushpy TODO 1.3", "ushgo TODO 1.1" or "wish GAPS Q2" where the project is not
obvious. Before 2026-10-28, ushpy was the directory `ush/`, and its items are cited
as "ush TODO …" in earlier records.

## Start each task

1. Read the project's TODO, DONE, GAPS, and the relevant design documents.
   ushpy: [TODO](ush/py/docs/TODO.md), [DONE](ush/py/docs/DONE.md), [GAPS](ush/py/docs/GAPS.md),
   [ARCH](ush/py/docs/ARCH.md), [SEC](ush/py/docs/SEC.md), [SECTEST](ush/py/docs/SECTEST.md),
   and [ROAD](ush/py/docs/ROAD.md). wish: [TODO](wish/docs/TODO.md),
   [DONE](wish/docs/DONE.md), [GAPS](wish/docs/GAPS.md), [IDEA](wish/docs/IDEA.md),
   [ARCH](wish/docs/ARCH.md), [SEC](wish/docs/SEC.md), and [SOTA](wish/docs/SOTA.md);
   [ROAD](wish/docs/ROAD.md) defines the active phase 1 scope and conditional phase 2.
   ushgo: [TODO](ush/go/docs/TODO.md), [GAPS](ush/go/docs/GAPS.md),
   [ARCH](ush/go/docs/ARCH.md), [SEC](ush/go/docs/SEC.md), and ushpy's documents above.
2. Check the branch, working-tree changes, and applicable repository instructions.
   Preserve unrelated work. If on `main`, create/switch to a feature branch before
   editing; carry existing changes only when their ownership and scope are clear.
3. Identify the requested milestone and acceptance criteria. Implement the next
   ready TODO item when asked to continue implementation; a planning/documentation
   task does not authorize executing the whole backlog.
4. Check dependency licenses and maturity before selecting components. Record
   material architecture changes and their reasons in the relevant design document.

## Branches, commits, and GitHub PRs

- All work happens on feature branches, including documentation, tests, and fixes.
  Name them `feat/…`, `fix/…`, `chore/…`, or `docs/…`, e.g. `feat/ushpy-2.9-escapes`.
- No feature commits on `main`; never push changes directly to `main`.
- Merge changes only through a GitHub PR with passing required checks and human
  review. Configure protected-branch checks/review requirements when repository
  administration is available; these instructions alone do not enforce protection.
- **AI never merges.** AI must not merge PRs, run local Git merges, enable auto-merge,
  approve its own changes, or bypass required review/checks. A human performs the
  GitHub merge. AI may prepare code, tests, commits, and PRs within the requested scope.
- Keep commits focused and reviewable, with a `Refs:` trailer naming the TODO items
  or GAPS entries they serve. Stage explicit files; inspect the staged diff and check
  for secrets/generated artifacts before committing.
- Do not rewrite shared history, force-push, delete others' branches, reset unrelated
  work, or run destructive operations without explicit authorization.
- CI is [.github/workflows/ci.yml](.github/workflows/ci.yml). While GitHub Actions
  doesn't start jobs, run it locally ([tools/ci/README.md](tools/ci/README.md)): push,
  run `just ci --post` on the pushed head commit, then open or update the PR. Repeat
  after every push. The PR states which commit local CI ran on and that its macOS-only
  run leaves the Linux matrix entries unchecked.
- PR descriptions state the problem, resulting behavior, TODO IDs, tests/results,
  deviations, security implications, and remaining limitations. Do not claim a test
  passed if it was skipped, unavailable, or run against a different revision.
- A missing GitHub remote does not authorize a local merge or a direct-main shortcut.
  Prepare the local feature-branch work and report what remains needed for a PR.

## TODO and DONE discipline

- Read TODO before implementing. Preserve stable `x.x` IDs and `[ ]` / `[x]` status.
  Add or split milestones when needed; keep the remaining plan accurate.
- Keep an item unchecked while acceptance criteria, tests, or necessary
  documentation remain incomplete. Do not mark a feature complete for scaffolding
  alone, or because only its happy path works.
- For each completed item, add a DONE entry on top (newest first, format in
  [SDLC](docs/SDLC.md#done-entries)): time from `date`, TODO items, PR, `Agent:` line,
  implementation notes with exact validation commands/results, deviations, insights.
- Never rewrite earlier DONE entries. Add corrections, follow-up findings, and
  merge/release outcomes as new entries on top.
- Questions only the user can answer go into the project's GAPS with a proposal;
  never act on a blocking question's proposed answer without the user's reply.
- Update TODO, DONE, and GAPS in the same feature branch/PR as the implementation.
  Implementation completion is distinct from human merge and release approval.

## Implementation and tests

- Implement the smallest complete milestone. Prefer existing kernels, notebooks,
  SQL engines, and automation adapters where SOTA establishes a suitable foundation.
- wish: start with the lean phase 1 plan: evaluate just-bash, compose through files
  and byte/text pipes, and add a few explicit tool commands. Defer typed pipelines,
  public resource handles, and general placement abstractions until a demonstrated
  workflow justifies them. Phase 2 is selected work, not an automatic scope expansion.
- wish: keep the TypeScript core independent of providers and runtimes. Use explicit
  contracts, validated inputs, typed results, and actionable failures.
- ush: one command, two implementations with equivalent functionality. A feature
  lands in ushpy (Python, experimental) with the ushgo TODO item that ports it, in
  the same PR; ushgo (Go) is the deliverable ([SDLC](docs/SDLC.md#ush-one-command-two-implementations)).
  Keep policy, execution, and history independent of the provider adapter in both.
  Tests use fake providers, a local mock API server, and temporary directories; they
  never call a paid provider or run a mutating command outside a temporary directory.
  Run them with `just ushpy-test`, `just ushgo-test`, or both with `just ush-test`.
- Every feature has automated tests for externally observable behavior and relevant
  errors. Add regression tests for bugs. Use integration/browser tests where unit
  tests cannot establish filesystem, runtime, origin, or server behavior.
- Tests cover cancellation, conflicts, limits, and denied permissions when relevant.
  Use deterministic provider/server fixtures for routine CI; live paid-provider or
  device tests require explicit configuration and must not silently incur costs.
- Run the affected tests and required project checks before considering the item
  complete. Document exact commands in the PR/DONE; if a check cannot run, report
  why and leave any dependent acceptance criterion incomplete.
- Documentation-only changes require syntax/link checks and rendering checks where
  applicable. Generated PDFs and stamps remain untracked. Add usable test commands
  to the repository when test infrastructure is implemented; do not invent results
  for runners that do not yet exist.
- Fix failing checks; never disable tests, weaken assertions, or bypass checks solely
  to obtain a passing result. Keep interfaces/documentation aligned with behavior.

## Security and dependencies

- Follow SEC's trust boundaries from the first runtime feature. Keep user code and
  rich executable output isolated from the shell, credentials, and trusted storage.
- Typed commands, AI-generated programs, notebooks, tools, and modules use the same
  broker checks. Treat imported content and model output as untrusted input.
- Keep AI keys in the trusted transport; exclude credentials from source, history,
  logs, fixtures, exports, and runtime messages. Use synthetic test secrets.
- Pin dependencies and runtime artifacts; review licenses, notices, provenance,
  and known vulnerabilities. Do not add a library or hosted service on the strength
  of its wrapper's license alone.
- Changes to grants, storage formats, isolation, egress, credentials, or server
  authorization include a threat-model update and negative security tests.
- Make storage/compute placement and AI disclosure explicit. Preserve data on
  failure; design migrations with tested recovery and rollback where supported.

## Before every release

Every release, including previews and patches, requires fresh checks on the exact
candidate commit and built artifacts. Record the revision, configuration, commands,
and results. Earlier passing checks do not satisfy a new release gate.

1. Run required feature/regression tests and the connected browser acceptance suite
   for every supported execution profile and browser in the release. For ush, run
   the suite of the shipped implementation (ushgo, or ushpy until ushgo is at parity)
   and the conformance suite on every supported OS (and Python version for ushpy),
   including root/sudo resolution and the zsh integration, plus the user smoke test
   ([SMOKE](ush/py/docs/SMOKE.md)) on the built artifact.
2. Run applicable adversarial security tests. For ush: command-policy bypasses,
   prompt injection through command output, secret redaction, credential file and
   key handling under sudo, and history contents. For wish: path traversal and cross-workspace
   access; forged/replayed bridge messages and revoked grants; malicious HTML/SVG
   and notebook outputs; credential leakage and prompt injection; unapproved AI
   disclosure; runtime egress channels; resource exhaustion and cancellation.
3. For server-enabled releases, also test cross-tenant authorization, CSRF and
   WebSocket authentication where applicable, SSRF/redirect controls, upload bounds,
   kernel isolation, lease expiry, and reconnect/mutation receipts. Record omitted
   suites only when the released profile demonstrably excludes that subsystem.
4. Scan the candidate and dependency/artifact inventory for secrets and known
   vulnerabilities; verify license notices, deployment headers, and isolation
   configuration. Scans supplement behavioral security tests rather than replacing them.
5. Resolve security-test failures before release. Record findings and residual
   limitations for human review; do not waive a failed security check by changing
   its expected result or marking the security milestone complete.
6. Prepare the release PR, notes, compatibility results, migration/recovery guidance,
   and security evidence. A human reviews, merges in GitHub, and authorizes/publishes
   the release. AI never merges and never publishes or deploys a release.
