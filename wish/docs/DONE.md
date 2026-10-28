# DONE

Progress log for [TODO](TODO.md), newest first. Each entry starts with the date and time of day
(local time) and a title, then the TODO items and the PR, then the agent that did the work as
`Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or `Agent: human`), then:
- **Implementation notes:** what exists and where, with the exact validation and its results.
- **Deviations:** what differs from the plan, and why.
- **Insights:** what we learned that matters for later work.

Entries are never rewritten, and proposed features are never described as implemented.
Implementation completion doesn't imply a merged PR or a release; corrections and later outcomes
(PRs, merges, releases) are new entries on top. The format is defined in
[SDLC](../../docs/SDLC.md#done-entries); all work follows [CLAUDE.md](../../CLAUDE.md).

---

## 2026-10-28 10:58 — The stack lands on the default branch

TODO items: 0.8, 0.9 (merge outcome). PR: [#12](https://github.com/gregoryyy/ushell/pull/12),
merged by the user as merge commit 06e3710 on `feature/wish-planning-process`.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- PRs #3–#6 had been merged into each other's branches, so the default branch held only #2. The
  open stack #7–#11 had the same shape. At the user's choice ("one pr"), #12 was retargeted to the
  default branch with all 11 commits from #3 on, and #7–#11 were closed with a pointer to it.
- Before the retarget, `git merge-tree --write-tree` of the default branch and 641f569 gave exactly
  the tree of 641f569, so the local CI run on 641f569 (all jobs passed on macOS; Linux entries not
  run) covers what was merged.
- [ROAD](../../docs/ROAD.md) step 0 records the landing.

### Deviations

- None of #7–#11 were merged on their own; their descriptions remain the detailed record of each
  part, and the DONE entries cite them.

### Insights

- A stacked PR only reaches the default branch if its base PR is merged *and* its branch is
  deleted (or GitHub otherwise retargets it) before the next merge. One PR per landing, against
  the default branch, avoids that trap; stack only when the base is about to merge.

---

## 2026-10-28 10:06 — ushpy and ushgo side by side; AGENTS.md; repository roadmap

TODO items: 0.9. PR: [#9](https://github.com/gregoryyy/ushell/pull/9) (stacked on #8).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **`ush/` → `ushpy/`** with `git mv`, so the two implementations of `ush` sit side by side as
  `ushpy/` (Python, experimental) and `ushgo/` (Go, the deliverable). The Python package stays
  `ush` (`ushpy/src/ush`), as do the command, the plugin, and every runtime path
  (`~/.config/ush`, `~/.local/state/ush`, `~/.local/share/ush`). Directory references were
  rewritten by a pattern that excludes those runtime paths, then checked by `just links`.
- **Build:** `just ushpy-test` (Python), `just ushgo-test` (Go), `just ush-test` runs both;
  `just ush` runs the Python implementation from `ushpy/`. The CI job `ush` is now `ushpy`; the
  local CI's tests follow. `.gitignore` and the PDF defaults use `ushpy/`.
- **`CLAUDE.md` → [AGENTS.md](../../AGENTS.md)** with `git mv`; the new `CLAUDE.md` is
  `@AGENTS.md`. AGENTS names the projects ushpy, ushgo and wish, the parity rule, the test recipes,
  and that earlier records cite ushpy items as "ush TODO". Document links to the rules point to
  AGENTS.md; links in earlier DONE entries still point to `CLAUDE.md`, which exists.
- **[docs/ROAD.md](../../docs/ROAD.md):** the roadmap across projects: repository step (land the
  stack, default branch `main`, CI, licence), ush 0.1, ush in Go, daily use, wish phase 1, one `//`
  paradigm, ideas.
- Path-only link fixes in this file's 0.8 entry (`../../ush/docs/` → `../../ushpy/docs/`).
- **Validation:** `python3 tools/check-links.py`: no broken links. `just ush-test`: ushpy 96 tests
  OK, ushgo ok. Local CI tool tests: 21 passed. `just ci`: see the PR.

### Deviations

- The existing `ushpy/.venv` moved with the directory (ignored files move with `git mv` of a
  directory); it keeps working for `python -m`. Recreate it if a script inside fails.

### Insights

- `ush` names four things here: the command, the Python package, the runtime directories, and
  (until now) a source directory. Only the last changed; the pattern had to leave
  `.config/ush`, `state/ush`, `share/ush`, `lib/ush` and `src/ush` alone.

---

## 2026-10-27 14:55 — Propeller's document regime for ush and wish

TODO items: 0.8. PR: [#4](https://github.com/gregoryyy/ushell/pull/4) (stacked on #3).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **Process:** [docs/SDLC.md](../../docs/SDLC.md) at the repository root, adapted from
  `../propeller/docs/SDLC.md`: lifecycle, the document set per project, work items and tags, GAPS
  rules, the DONE entry format, branches and `Refs:` trailers, CI, definition of done, TODO mode,
  decisions, releases. [CLAUDE.md](../../CLAUDE.md) points to it and keeps precedence (human merges,
  security, release gate); its TODO/DONE section now describes newest-first entries and GAPS.
- **Moves (`git mv`, IDs and status unchanged):** `wish/TODO.md` and `wish/DONE.md` to
  `wish/docs/`, `wish/docs/PHASES.md` to [ROAD.md](ROAD.md); `ush/TODO.md` and `ush/DONE.md` to
  `ush/docs/`, `ush/docs/DESIGN.md` to [ARCH.md](../../ushpy/docs/ARCH.md).
- **New:** [GAPS](GAPS.md) for wish (Q1 start, Q2 browsers, Q3 providers, Q4 licence; A1–A3) and
  for ush ([GAPS](../../ushpy/docs/GAPS.md): Q1 licence and Q2 distribution, both blocking the 0.1
  release, Q3 platforms; A1–A4); [ush ROAD](../../ushpy/docs/ROAD.md) with four phases and gates; a
  phase table with gates on top of wish's ROAD; `.github/pull_request_template.md` with the
  definition of done.
- **TODO and DONE headers** in the new form: status legend and tags, newest-first logs. The earlier
  entries stay below an "Earlier entries" marker, unchanged except for path-only link fixes after
  the move (ush DONE's 1.1–1.9 entry).
- **Root README:** ush first, with its quick start and document map, then wish, then how the work is
  organised. `just pdfs` without paths now also builds `docs/`.
- **Validation:** `python3 tools/check-links.py`: no broken links. `just ci` and `just pdfs`: see
  the PR.

### Deviations

- Propeller keeps one TODO for one project; here each project keeps its own documents, and the
  shared process sits in the root `docs/`, as propeller's BOOT allows for subprojects.
- Propeller merges PRs once CI passes; here a human reviews and merges every PR, as before.
- Repository-wide process work stays in wish's milestone 0, where 0.2, 0.4, 0.6 and 0.7 already are.

### Insights

- The old DONE preambles asked for "append-only"; newest first keeps that guarantee (no entry is
  rewritten) while putting the current state where readers look first.

---

## Earlier entries (oldest first, earlier format)

## 2026-10-13 — 0.1 Design and initial research

Implemented:

- Defined wish as a browser workspace shell for files, data, notebooks, live tools,
  JavaScript/Wasm, virtual resources, and explicit server storage/compute.
- Specified typed commands and natural-language `//` translation using the user's
  AI key, with generated programs sharing the ordinary executor and permissions.
- Added product, architecture, security, and state-of-the-art documents in `docs/`.

Validation: reviewed linked documents and command examples; generated their PDFs.
Research uses primary documentation. No wish runtime or security implementation
was executed, and no runtime feature is claimed complete.

Deviations: the scope expanded from DOM commands to an artifact/resource workspace.
The architecture was initially named `TECH.md`; its later rename is recorded in 0.3.

Learnings: notebook/kernel files and shell files need an explicit synchronization
contract. Browser-side execution, BYOK AI, and virtual shells already exist; wish's
proposed contribution is their shared command/resource model.

Status: bootstrap documentation commit `7736472`. This predates the feature-branch
and GitHub-PR process established in 0.4; it is not retroactively described as a PR.

## 2026-10-13 — 0.2 Markdown PDF tooling

Implemented:

- Adapted `just pdf FILE` and `just pdfs [PATHS]` from `../orgfab.ai`.
- Added Pandoc/isolated-headless-Chrome rendering, embedded assets, print styles,
  temporary-profile cleanup, and preservation of previous PDFs on build failure.
- Added checksum-based incremental builds and ignored generated PDFs/stamps.

Validation: exercised single/bulk conversion, paths containing spaces and quotes,
invocation from subdirectories, incremental no-op builds, failure preservation,
and bulk error propagation. Generated project PDFs and inspected their output.
These were smoke checks; a committed automated regression suite is still planned
in 1.3. No release security gate existed or was claimed to have run.

Deviations: rendering uses headless Chrome rather than a LaTeX toolchain. Mermaid
code blocks print as source; the architecture drawing now uses SVG.

Learnings: some Chrome builds print successfully without exiting, so the script
checks for a stable output and terminates its temporary browser. Image-only changes
are outside the incremental stamp; remove the affected PDF to force regeneration.

Status: bootstrap tooling commit `0f6fa49`, before the process in 0.4. Generated PDFs
are local artifacts, not tracked implementation files.

## 2026-10-13 — 0.3 Architecture and research refinements

Implemented:

- Renamed `docs/TECH.md` to `docs/ARCH.md` and updated documentation references.
- Added `docs/images/wish-execution.svg` and an execution walkthrough covering
  optional AI translation, validation, isolated runtimes, brokered resources,
  browser/server adapters, commits, and live-view updates.
- Added SOTA differentiation, 20 maturity/license comparison entries, separate
  SDK/engine/service license scopes, reuse decisions, and implementation priorities.

Validation: checked Markdown parsing, local references, and diff whitespace;
rebuilt affected PDFs. Visually inspected the architecture drawing and SOTA tables,
and confirmed new SOTA text in its 10-page PDF. License research checked repository
files, package metadata, and vendor terms as of 2026-10-13; it is not a license
audit of a selected dependency bundle or an integration benchmark.

Deviations: an SVG replaces the original architecture Mermaid overview so the
drawing appears in the existing PDF renderer. Maturity and licensing have separate
tables to keep the original capability comparison readable.

Learnings: a shell UI's license does not cover its execution engine. In particular,
CheerpX has separate terms, the current Wasmer SDK uses Modified MIT, and Puter SDK
metadata differs from the platform repository's default license. Recheck pinned
artifacts before adoption; keep compatibility runtimes optional.

Status: implemented in the working tree; carried onto
`feature/wish-planning-process`. No commit or GitHub PR yet for these refinements.

## 2026-10-13 — 0.4 Planning and contribution process

Implemented:

- Added root `TODO.md` with numbered checkbox milestones, dependencies,
  acceptance criteria, feature tests, and a recurring release security gate.
- Added this append-only implementation log, distinguishing implemented tooling
  and design work from the still-unimplemented wish runtime.
- Added root `CLAUDE.md` with feature-branch, GitHub-PR, human-only merge/release,
  testing, security, dependency, and change-management instructions.
- Linked the plan, log, and instructions from `README.md`.

Validation: parsed the Markdown; checked local links, unique milestone IDs,
checkbox/log agreement, required process rules, and diff whitespace. No runtime
tests apply to these documentation-only changes.

Deviations: the repository has no Git remote, so no GitHub PR or server-side
branch-protection configuration exists. The documents prescribe that workflow;
they do not claim it is enforced remotely. Earlier bootstrap commits on `main`
are retained as history; subsequent work is on a feature branch.

Learnings: marking a milestone implemented and merging/releasing it are separate
events. Security checks must be repeated for every release candidate rather than
being considered satisfied by a single checked milestone.

Status: working tree on `feature/wish-planning-process`; no commit, PR, merge,
or release created for this task.

Validation follow-up for 0.4: `git diff --check` passed. Pandoc JSON parsing and
scripted document checks validated eight Markdown files, 37 local links, and 38
unique TODO IDs; the four checked milestones match the DONE entries. Required
process rules and the feature branch were verified. `just pdfs README.md TODO.md
DONE.md` succeeded; PDF text checks passed and TODO's checkbox rendering was visually
inspected. No GitHub checks or release security suite was run for this documentation
task.

## 2026-10-14 — 0.5 Lean phase 1 and conditional phase 2 planning

Implemented documentation:

- Added `docs/PHASES.md` with a file-based browser shell/notebook/AI workflow for
  phase 1 and a menu of selected phase 2 extensions.
- Rescoped TODO around just-bash evaluation, ordinary byte/text pipelines, JSON/CSV,
  one notebook, one persistence backend, small checked adapters, and a local
  acceptance workflow. Preserved all 38 existing milestone IDs and added four;
  all 37 runtime/test/release milestones remain unchecked.
- Updated IDEA, ARCH, SEC, README, and CLAUDE to use the phased scope. Updated the
  SVG drawing for the lean command/file/notebook flow, with servers marked phase 2.
- Added just-bash to SOTA with primary-source capability, beta-maturity, and
  Apache-2.0 license information, integration limits, and a prototype decision gate.
- Included the previously uncommitted architecture rename, research refinements,
  TODO/DONE files, and contribution process in the planning commit.

Validation: `git diff --check`; Pandoc JSON parsing and scripted local-link,
unique-ID, checkbox/log, phase-scope, and SVG XML checks; `just pdfs README.md
TODO.md DONE.md docs`; PDF text checks and visual inspection of the revised
architecture drawing and phase/TODO output. These are documentation checks;
no wish runtime, integration spike, feature tests, or release was implemented.

Deviations: the earlier custom parser, typed streams, first-class dataset service,
and generic resource acquisition are no longer required for the starting release.
Public typed pipelines/resource handles are optional research even in phase 2.
Additional pages, Wasm commands, server resources, external tabs, and hardware move
to selected phase 2 work. Phase 1 recognizes standalone `//` submissions rather
than mixing natural-language lines into Bash scripts. IndexedDB is the first
small-file persistence candidate; an OPFS implementation is deferred.

Learnings: shell syntax/utilities are reuse opportunities, while checked shared
files and the notebook bridge are the useful integration work. just-bash is
documented as beta, resets shell state between executions, and omits some runtime
features in browsers; the spike must test actual file/session/cancellation behavior.
Simpler commands still require isolation, permission checks, recovery, and fresh
security checks for every released capability.

Status: planning is recorded by `docs: plan lean wish phase 1 and optional phase 2`
on `feature/wish-planning-process`. This is also the commit-status follow-up for
0.3 and 0.4; their earlier working-tree notes remain as historical entries.
No GitHub remote is configured, so no PR is opened and no merge or release occurs.

## 2026-10-26 — 0.6 Repository split into wish and ush

Branch: `feature/split-wish-ush` (local; not committed or pushed at the time of
writing), based on `feature/wish-planning-process`.

Implemented:

- Moved `docs/`, `TODO.md`, and `DONE.md` into `wish/` with `git mv`. Added
  `wish/README.md`; the root README now introduces both projects, with an ush
  quick start, the repository layout, and the CI checks.
- Updated `CLAUDE.md` for two projects with independent TODO/DONE IDs and
  ush-specific test and release-security rules.
- `just pdfs` now defaults to `README.md`, `wish/`, and `ush/`. Added `just links`
  (`tools/check-links.py`, relative Markdown links and images) and `just ush-test`.
- Path-only link fixes after the move: `CLAUDE.md` links in `wish/TODO.md` and
  `wish/docs/PHASES.md`, and in the preamble of this file (not in any entry).
- The GitHub repository was renamed from `lsh` to `ushell` at the user's request;
  `origin` now points to `https://github.com/gregoryyy/ushell.git`.

Validation: `python3 tools/check-links.py` reports no broken links. Moving the
directory carried the untracked PDFs along; no PDFs were rebuilt.

Remaining: the remote has no `main` branch (only `feature/wish-planning-process`),
so PRs and branch protection need a base branch decided by a human.

Status (2026-10-26): committed as `03b2d10` (`docs: split repository into wish and
ush projects`) on `feature/split-wish-ush` and opened as
[PR #1](https://github.com/gregoryyy/ushell/pull/1) against the remote default
branch `feature/wish-planning-process`, since the remote has no `main`. Not merged.

## 2026-10-26 — 0.7 Local CI

Branch: `feature/local-ci`, based on `feature/wish-planning-process` after PR #1 was
merged. Implementation commit `36c31f0`.

GitHub refused to start the Actions jobs of PR #1 ("recent account payments have
failed or your spending limit needs to be increased"). At the user's request, CI now
runs locally, following `../lakeshore/utils/ci/` and its `docs/ci/README.md`.

Implemented:

- `.github/workflows/checks.yml` renamed to `ci.yml` and restructured so the local
  runner can read it: the zsh install is an unconditional, guarded step instead of
  `if: runner.os == 'Linux'`; new `tools` job (ruff, format, pyright and pytest for
  `tools/ci/`, pinned as in lakeshore) and `secrets` job (gitleaks action v3.0.0).
- `tools/ci/workflow.py` (pure): maps the workflow to local steps and rejects anything
  it can't run. Unlike lakeshore's, it expands `strategy.matrix` (lists only), fills
  `${{ matrix.KEY }}` in names, `runs-on` and `with`, and runs only this platform's
  entries when the matrix chooses the runner. No service containers or ports.
- `tools/ci/run.py`: clean worktree per commit, `bash -eo pipefail` steps, reduced
  environment, failing `claude`/`codex`/`sudo` shims, `setup-python` mapped to an
  installed interpreter found by `uv python find` (the real executable, not a pyenv
  shim), one run at a time, `--post` for `local / <job>` commit statuses, logs and
  `summary.md` in `.ci/`.
- `just ci`; `.ci/`, `.ruff_cache/`, `.pytest_cache/` ignored; `.gitleaksignore` with
  the fingerprints of the three synthetic tokens in `ush/tests/test_redact.py` that
  gitleaks 8.30.1 flagged (gitlab-pat, jwt, aws-access-token); checked to be fixtures.
- Docs: `tools/ci/README.md`; root README checks and layout; CLAUDE.md PR rule (push,
  `just ci --post`, PR line); ush TODO 2.3 now names `ci.yml`.

Validation (macOS 26, arm64):

- `tools/ci`: `uvx ruff@0.17.2 check .` and `format --check .` clean; pyright 1.1.414:
  0 errors; pytest: 20 passed.
- `just ci` on `36c31f0`: links, ush (macos-latest, 3.10) with Python 3.10.18,
  ush (macos-latest, 3.13) with Python 3.13.7 (63 tests OK each), tools and secrets
  passed; both ubuntu-latest entries listed as not run here; `just pdfs` then built.
- A probe workflow on a throwaway detached commit: `ANTHROPIC_API_KEY` set in the
  caller's shell was absent from the step; `sudo` failed via the shim; a failing step
  failed the job, skipped the next step, still ran an `always()` step; the runner
  exited 1. `just ci --job nope` names the valid jobs and fails.
- `python3 tools/check-links.py`: no broken links.

Deviations: the Linux matrix entries of `ush` aren't checked locally on a Mac; they
run when Actions returns or when `just ci` runs on Linux. Local statuses are
self-reported by the developer's machine.

Status (2026-10-26): `just ci --post` passed on `8bba1af`; opened as
[PR #2](https://github.com/gregoryyy/ushell/pull/2) against `feature/wish-planning-process`.
Not merged.
