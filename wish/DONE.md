# wish implementation notes

Append a dated entry for every implemented TODO milestone using the same `x.x`
identifier. Record what actually exists, exact validation performed, deviations,
learnings, and commit/PR status. Do not replace earlier entries or describe proposed
features as implemented. Append corrections or later outcomes as new entries.

Implementation completion does not imply a PR was merged or a release shipped.
When a PR is opened, a human merges, or a release is published, append the reference
and outcome. All future work follows [CLAUDE.md](../CLAUDE.md).

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
