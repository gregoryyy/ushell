# wish implementation plan

Read this plan before implementation, alongside [CLAUDE.md](../CLAUDE.md),
[PHASES.md](docs/PHASES.md), [IDEA.md](docs/IDEA.md), [ARCH.md](docs/ARCH.md),
[SEC.md](docs/SEC.md), and [SOTA.md](docs/SOTA.md). Implement the next requested,
ready milestone. A planning task does not authorize executing the entire backlog.

Milestones have stable `x.x` IDs and `[ ]` / `[x]` checkboxes. The IDs are task
numbers, not phase numbers. Mark an item complete only after its acceptance checks
pass and its implementation, validation, deviations, and learnings are appended to
[DONE.md](DONE.md). Record PR merge and release outcomes separately.

Every feature includes automated behavioral and relevant failure/security tests;
bug fixes include regression tests. Documentation gets syntax/link and applicable
rendering checks. All work is on feature branches, merges use GitHub PRs, and AI
never merges. Security tests run against every release candidate.

## 0. Completed planning and tooling

- [x] 0.1 Define wish's workspace, commands, AI flow, architecture, security profile,
  and initial SOTA research. Acceptance: linked documents distinguish design from
  implemented behavior.
- [x] 0.2 Add incremental `just pdf` / `just pdfs` tooling. Acceptance: single/bulk
  builds, quoted paths, invocation directories, unchanged builds, and failure
  preservation have been exercised. Committed regression coverage remains in 1.3.
- [x] 0.3 Rename `ARCH.md`, add the execution drawing, and document differentiation,
  maturity, license scope, and reuse decisions. Acceptance: links and PDFs checked.
- [x] 0.4 Establish TODO/DONE and CLAUDE process instructions. Acceptance: stable
  IDs, honest logs, feature branches, human GitHub merges, feature tests, and
  recurring release security checks are defined.
- [x] 0.5 Record lean phase 1 and conditional phase 2, update the design/architecture
  and just-bash research, and commit the planning. Acceptance: scope agrees across
  documents; runtime tasks remain unchecked; documentation checks and PDFs pass.
- [x] 0.6 Split the repository into `wish/` (web) and `ush/` (command line) with
  shared process rules and document tooling. Acceptance: wish documents moved with
  history, relative links checked, PDF defaults cover both projects.
- [x] 0.7 Run the CI workflow locally while GitHub Actions doesn't start jobs, after
  lakeshore's design (`just ci`, `--post` commit statuses). Acceptance: the workflow
  maps completely or fails before running; matrix entries run per platform; tests
  for the mapping; a full run passes on a commit; a failing step fails the run.

## Phase 1: a lean browser workspace

The first release proves CSV import → shell transformation → Python notebook/plot
→ save/reload, plus equivalent `//` commands and replay. Compose through files,
byte/text pipes, and JSON/CSV. Evaluate just-bash before writing shell machinery.

### 1. Development foundation

- [ ] 1.1 Record the browser support target, runtime origins, dependency/license
  inventory, and one notebook UI candidate. Acceptance: name the just-bash spike's
  required interfaces and choose a small CSV/plot fixture; avoid adopting multiple UIs.
- [ ] 1.2 Scaffold the trusted TypeScript app and separate runtime launcher with
  reproducible builds and development HTTPS. Acceptance: both origins load, the
  runtime receives no AI key, and baseline execution does not require shared memory.
- [ ] 1.3 Add unit, integration, browser, and security runners and GitHub PR checks;
  cover PDF-tool regressions. Acceptance: documented commands run locally; a failing
  test fails CI; configure branch protection when a GitHub repository is available.

### 2. Reused shell and small command adapters

- [ ] 2.1 Prototype and integrate a pinned just-bash browser build. Acceptance:
  quoting, pipes, redirection, file adapters, custom commands, cancellation, and
  bounded execution pass the fixture. Define a supported syntax subset and validate
  it before execution; record an evidence-based alternative if the spike fails.
- [ ] 2.2 Register the permitted built-ins and small notebook/job/settings commands,
  with argument validation and help. Acceptance: unknown commands and invalid
  arguments fail; nested execution cannot bypass permissions or the supported subset.
- [ ] 2.3 Verify byte/text pipes, JSON/CSV use, redirection, output bounds, and failure
  propagation through the reused engine. Acceptance: binary/text fixtures, a failed
  pipeline, and interrupted output preserve prior committed files; no typed-record
  pipeline or public resource-handle format is introduced.
- [ ] 2.4 Add a small job list, cancellation, deadlines, session context, and history.
  Acceptance: the prompt remains responsive, cancelled jobs stop later operations,
  and programs replay without AI. Test cwd/environment behavior across submissions.

### 3. Files and checked access

- [ ] 3.1 Connect bounded memory files to ordinary `ls`, `cat`, `mkdir`, `cp`, and
  redirection, reusing the chosen engine's file interfaces. Acceptance: a pipeline
  creates/transforms a file; path traversal and unsupported operations fail.
- [ ] 3.2 Implement checked file access and notebook-call boundaries with internal
  caller/job IDs. Acceptance: path scope, read/write rights, versions, payload limits,
  and revocation are checked; forged identity/cross-workspace calls fail. Generic
  resource acquisition and user-visible handles are deferred to optional 8.5.
- [ ] 3.3 Add one browser persistence backend, initially evaluating IndexedDB for
  small files and metadata, with conditional writes and recovery. Acceptance:
  save/reload, interruption, concurrent-writer conflicts, denied storage, and quota
  errors pass; memory fallback and unsaved state are visible. OPFS is not a prerequisite.
- [ ] 3.4 Add explicit file import and workspace export/restore. Acceptance: browser
  selection supplies input files; hostile archive paths fail; backups restore files
  without credentials. Persistent physical-directory mounts are a phase 2 extension.
- [ ] 3.5 Add a small file browser/editor with location and dirty-state display.
  Acceptance: shell/editor see committed changes; failed saves and conflicts remain
  recoverable; basic keyboard interaction works.

### 4. One Python notebook integration

- [ ] 4.1 Integrate Pyodide in an isolated worker with execute, interrupt, restart,
  and dispose. Acceptance: output, errors, cancellation, and incompatible packages
  behave predictably; the kernel cannot access the AI key or trusted storage directly.
- [ ] 4.2 Bridge granted file snapshots into the kernel and commit successful,
  authorized output against expected versions. Acceptance: Python reads shell edits;
  denied writes, conflicts, interrupted cells, and oversized inputs preserve data.
- [ ] 4.3 Add `notebook create/open/run` with one UI and `.ipynb` files. Acceptance:
  execute the CSV/plot fixture, save/reload source and output, and restart the kernel;
  sanitize rich output and isolate executable widgets. The plot is the first live tool.

### 6. One natural-language adapter

- [ ] 6.1 Add trusted `ai configure/status/clear-key`, one provider adapter,
  session-memory keys, endpoint selection, and budgets. Acceptance: mock-provider
  tests pass; no key reaches history, logs, exports, or runtime messages; unsupported
  direct browser transport fails clearly or uses an explicitly selected broker.
- [ ] 6.2 Recognize standalone `//` submissions and translate approved context into
  displayed, validated shell programs using the same executor and permissions.
  Acceptance: execute within existing grants and replay without AI; quoted text/URLs
  do not trigger translation; reject invalid, nested, oversized, or stale programs.
- [ ] 6.3 Test hostile context, disclosure boundaries, optional preview, and provider
  failures. Acceptance: injection cannot widen grants or change endpoints; missing
  keys, ambiguity, timeouts, and budget exhaustion execute no generated program;
  uncertain mutations are never blindly retried.

### 9. Local acceptance and recurring release checks

- [ ] 9.1 Automate phase 1's connected local workflow, including save/reload, export
  recovery, AI/manual equivalence, and replay. Acceptance: failure, conflict,
  cancellation, denial, and secret-handling cases accompany the success path;
  record browser compatibility, startup, responsiveness, and memory observations.
- [ ] 9.2 Implement the recurring release security gate in [CLAUDE.md](../CLAUDE.md).
  Acceptance: run hostile-output/runtime, file-access/bridge, AI disclosure, egress,
  and applicable server tests against every exact release candidate; include secret
  and dependency scans. Earlier passing releases never waive the gate.
- [ ] 9.3 Prepare reproducible artifacts, license notices, supported-browser results,
  operational limits, and storage recovery/migration instructions. Acceptance:
  clean build, installation, export recovery, and applicable upgrades are exercised.
- [ ] 9.4 Prepare the phase 1 release PR with TODO IDs, tests, security evidence,
  limitations, and release notes. Acceptance: a human reviews/merges through GitHub
  and authorizes publication; AI never merges or releases.

## Phase 2: selected extensions

These tasks require a demonstrated workload and explicit scope selection. An
unselected extension does not block a release that excludes it. Extend existing
commands/file adapters before adding a general execution platform.

### 5. Data, live pages, and additional runtimes

- [ ] 5.1 Add file-backed data commands or one SQL adapter when JSON/CSV operations
  are insufficient. Acceptance: shell/notebook share files; schema errors and
  conflicting updates pass; record why the chosen engine is needed.
- [ ] 5.2 Add isolated live pages with a small declared command/message bridge and
  file-change notifications. Acceptance: a second interactive tool shares files;
  forged/replayed messages, navigation, and revoked access fail; hostile pages cannot
  alter the trusted UI. Avoid requiring a general plugin manifest system initially.
- [ ] 5.3 Add a JavaScript command adapter in an isolated runtime. Acceptance:
  brokered file access, byte/text output, exceptions, runaway jobs, and credential
  denial pass; the trusted app never evaluates authored source.
- [ ] 5.4 Add Wasm export/command adapters with an explicit documented interface.
  Acceptance: a byte/file transformation works; malformed modules, invalid exports,
  denied imports, limits, and cancellation pass. General component/WASI support is optional.
- [ ] 5.5 Add workspace-page DOM commands where needed. Acceptance: mutations reject
  ambiguous targets and document changes invalidate target IDs; selectors/content
  never become privileged code. External tabs remain separate in 8.2.

### 7. Explicit server adapters

- [ ] 7.1 Add authenticated remote file storage independently of remote compute.
  Acceptance: conditional writes, receipts, upload bounds, cross-tenant denial,
  applicable CSRF/origin checks, and reconnect reconciliation pass.
- [ ] 7.2 Add explicit leased remote kernels through a Jupyter-compatible adapter.
  Acceptance: server isolation, quotas, controlled egress, cancellation, expiry after
  tab closure, and credential/cross-tenant denial pass. No implicit remote fallback.
- [ ] 7.3 Show storage/compute locations, transfer grants, detached jobs, and receipts.
  Acceptance: reconnect resolves uncertain outcomes without duplicate mutations;
  server destination/redirect checks block private/metadata-service access.

### 8. Optional capabilities and abstractions

- [ ] 8.1 Evaluate larger-file OPFS support and Linux/Node/richer Wasm runtimes only
  against measured limits. Acceptance: document licensing, deployment cost, browser
  support, file behavior, and tests before selecting an adapter.
- [ ] 8.2 Add an external-tab adapter if a workflow needs it. Acceptance: minimal
  permissions, explicit tab selection, document invalidation, and browser policy
  constraints pass; the ordinary workspace still works without the adapter.
- [ ] 8.3 Add selected-directory mounts or simulated/physical/GPU resource adapters
  individually. Acceptance: browser gestures/grants, feature detection, revocation,
  limits, and unsupported-capability behavior are tested.
- [ ] 8.4 Evaluate WebMCP or Wasm component interoperability for a supported use case.
  Acceptance: pin interface versions, enforce permissions, and preserve a working
  baseline when the experimental interface is unavailable.
- [ ] 8.5 Evaluate typed pipelines, public resource handles, or a common resource API
  only if an acceptance example is impractical with files, JSON, names, and explicit
  commands. Acceptance: document the benefit, complexity, compatibility/migration,
  and tests before committing to any new public abstraction.
- [ ] 8.6 Add opt-in workspace/history search, then evaluate semantic retrieval and
  captured-page search. Acceptance: source/date provenance, permission filtering,
  retention/deletion, stale content, injection, and AI/embedding disclosure tests pass;
  browser history metadata is not treated as captured page contents.

### 9. Extended acceptance

- [ ] 9.5 Extend the connected workflow and recurring release gate for each selected
  phase 2 capability. Acceptance: file sharing, placement, authorization, conflicts,
  cancellation, and relevant reconnect/device cases pass; only tested profiles are
  included in a human-reviewed release PR.
