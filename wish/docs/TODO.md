# TODO

Plan for wish, the browser workspace shell: the product is in [IDEA](IDEA.md), the architecture in
[ARCH](ARCH.md), the security profile in [SEC](SEC.md), and the research in [SOTA](SOTA.md).
[ROAD](ROAD.md) defines the lean phase 1 and the conditional phase 2 that the milestones below
implement; open questions are in [GAPS](GAPS.md), and progress is logged in [DONE](DONE.md). The
process is [SDLC](../../docs/SDLC.md); the rules in [AGENTS.md](../../AGENTS.md) take precedence.
Implement the next requested, ready milestone; a planning task does not authorize executing the
entire backlog. Status: 2026-11-10 09:50; the site workspace (groups 10–14, 9.6–9.7) was added
on that date, and the groups 1–9 of 2026-10-14 are now the workspace substrate
([WORKSPACE](WORKSPACE.md)).

Status: `[ ]` open, `[x]` done. Tags:
- *(decision)*: a choice to make and record in DONE before the dependent work starts.
- *(variant)*: an alternative built next to the main design so the two can be compared.
- *(skipped)*: set aside by the user, with date; stays open as a record.

IDs are stable task numbers, not phase numbers, and independent of ush's. An item is done when its
acceptance checks pass, with behavioural and relevant failure/security tests, and DONE has an entry
for it. Repository-wide tooling and process work is tracked in milestone 0. Security tests run
against every release candidate.

---

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
- [x] 0.8 Move both projects to propeller's document regime: shared process in
  [SDLC](../../docs/SDLC.md); TODO, DONE, ARCH, ROAD, SEC, and GAPS in each project's
  `docs/`; DONE newest first with the agent line; GAPS for the user's questions; a PR
  template with the definition of done; root README with ush first. Acceptance: IDs and
  status unchanged, earlier DONE entries kept, links and PDFs build.
- [x] 0.9 Parity layout for ush: `ush/` renamed to `ushpy/` next to `ushgo/`; `CLAUDE.md`
  moved to [AGENTS.md](../../AGENTS.md) with a one-line `CLAUDE.md` pointing to it; the
  repository roadmap in [docs/ROAD.md](../../docs/ROAD.md). Acceptance: history kept by `git mv`;
  links, both suites, and the local CI pass; runtime paths (`~/.config/ush`) unchanged.
- [x] 0.10 Re-centre wish on websites as the space to work on: page, site and link namespaces,
  agentic crawling, a references-or-data overlay, structured retrieval with explanations,
  evidence and trends; Playwright for capture; open source without Lakeshore components. Acceptance: IDEA,
  ROAD, ARCH, SEC, SOTA, TODO and GAPS agree; the earlier concept is kept in WORKSPACE; IDs and
  status of groups 1–9 unchanged; links and PDFs build.
- [ ] 0.11 Move wish to a project and repository of its own, as the "web intelligent shell"
  (decided 2026-11-10; details in GAPS Q13). Acceptance: the new repository holds `wish/`'s
  history (README, `docs/`, images), a LICENSE (MIT), its own AGENTS.md, SDLC, CI and document
  tooling adapted from this repository, with stable TODO/GAPS IDs and DONE entries kept; links
  and PDFs build there; this repository then keeps only a pointer (root README, repository ROAD,
  AGENTS.md) and its `wish/` directory is removed in a reviewed PR. The `//` request contract
  shared with ush (repository ROAD step 5) is documented in both.


## Site workspace

Websites as the space to work on ([IDEA](IDEA.md), decided 2026-11-10; phase order in
[ROAD](ROAD.md), GAPS Q5). Phase 1 is 10, 11.1–11.2 and 9.6 together with the substrate's
2, 3.1–3.2 and 6; phases 2 and 3 select from 11.3–14 one item at a time. Every test runs against
local fixture sites and a mock AI provider; no test contacts a real site or a paid provider. No
item takes a dependency on a Lakeshore component.

### 10. Host, fixtures, capture, and namespaces

- [ ] 10.1 *(decision)* Record the host (local Node.js process in TypeScript, program `wish`;
  GAPS Q6, Q11), the
  pinned Playwright version and browsers, the dependency/license inventory (recheck the SOTA
  table), the workspace layout, and the overlay store (JSON Lines plus SQLite, GAPS Q8). Acceptance: DONE entry with the
  versions, licenses and reasons; a hello-world capture runs from `just`.
- [ ] 10.2 Fixture-site server for tests: login, templates, pagination, an infinite calendar,
  logout/delete links and state-changing GET endpoints that record hits, cross-site links,
  injected instructions, two "days" of changed content. Acceptance: served locally by a `just`
  recipe; every later site test uses it; hit counters are readable by tests.
- [ ] 10.3 Site profiles and login: `site open URL [--login]`, `site profiles`, `site forget`.
  Headed login in a persistent context under the state directory (mode 0700). Acceptance: login
  on the fixture works; synthetic cookies never appear in the workspace, captures, logs, history
  or AI requests; `site forget` removes the profile; a missing or expired session is reported.
- [ ] 10.4 Capture engine: per-page snapshot (URLs, status, time, browser, accessibility tree
  joined with DOM facts, links with roles, digests) in `--refs` mode, append-only.
  Acceptance: snapshots of the fixture pages match golden files; a recapture of an unchanged page
  has equal digests; redirects, errors and timeouts are recorded, not dropped.
- [ ] 10.5 Page namespace as a read-only virtual filesystem in the shell: `ls`, `cat`, `cat -l`,
  `find`, `grep` over semantic nodes; stable paths; moved/missing nodes on recapture.
  Acceptance: path grammar documented; repeated names indexed; writes fail; a changed fixture
  page reports moved and missing nodes; output escapes control characters.
- [ ] 10.6 Site and link namespaces: `/site/<host>/<path>`, `@links` with role, text, target and
  state; `links --to` backlinks; `/cap/<run>`. Live versus captured is shown. Entering an
  uncaptured page in a live session is a policy-checked navigation. Acceptance: denied
  navigations (W1 deny list, off-origin) fail with the rule that refused them; offline browsing of
  captures loads nothing.

- [ ] 10.7 AI configuration per site profile: provider (remote with the user's key, or Ollama on
  loopback), model, disclosure grant for remote providers, and a privacy step before every model
  call with configurable data masking (patterns, field types) and synonymization (consistent
  stand-ins, mapping kept locally and reversed in the response). Acceptance: with a mock remote
  provider, no masked or synonymized original reaches the request; answers show the originals;
  the mapping never appears in requests, exports or logs; a remote call without a grant is
  refused; a local-model profile in the 13B–30B range runs the phase 1 `//` fixture tasks, and
  its results are recorded (GAPS Q9).

### 11. Crawling and capture policies

- [ ] 11.1 Deterministic bounded crawl: scope, depth, page/time budgets, rate, concurrency,
  canonicalisation and digest deduplication, a configurable robots policy (honour, show only,
  per site profile; default honour unauthenticated, show logged in; GAPS Q7), the
  navigation-safety policy, cancellation, and a decision record per link. Acceptance: on the
  fixtures no state-changing endpoint is hit, the calendar trap stops at the budget, logged-in
  crawls stay on the declared origins, each robots setting behaves as configured and is
  recorded in the run, the run reports its stop reason and unreached frontier.
  Decide whether Crawlee's queue is used under wish's policy.
- [ ] 11.2 Mirror mode: WARC 1.1 of every response plus a browsable tree generated from it;
  headers with credentials redacted. Logged-in mirrors need an explicit flag. Acceptance: the
  fixture site mirrors completely; replay in a sandboxed context without profile or network shows
  the pages; no synthetic secret in the WARC; record fidelity of the chosen recording approach.
- [ ] 11.3 Agentic link decisions: link features in the frontier; a model ranks candidate IDs
  against the goal and labels relations; the policy admits; records with model and prompt
  version. Acceptance: on a fixture goal, equal recall with fewer fetches than 11.1's breadth
  first, with a remote mock and with a 13B–30B local model; injected instructions change no
  policy outcome; the model can't add URLs; without a usable provider the crawl falls back to
  rules.
- [ ] 11.4 Templates and extraction: structural clustering, model-proposed schemas checked on a
  second exemplar, deterministic application, rules for headings/tables/lists/breadcrumbs and
  JSON-LD/microdata. Acceptance: precision/recall on labelled fixture pages; model calls grow with
  the number of templates, not pages; failed checks fall back and are recorded.
- [ ] 11.5 Data capture (`--data`): selected passages, tables and values with anchors.
  Acceptance: re-extraction from the capture equals the live result on unchanged pages; `--refs`
  runs contain no attribute values or passages (tested).

### 12. Semantic overlay across sites

- [ ] 12.1 Overlay format `wish-overlay/1`: statements, entities, anchors with Web Annotation
  selectors, methods, runs, supersession; JSON Schemas in the repository; SQLite index rebuilt
  from the files. Acceptance: schema validation in tests; dropping the index and rebuilding gives
  identical query results.
- [ ] 12.2 `/sem` namespace and cross-site browsing over captures; proposed `same_as` identity
  links with evidence, never silent merges. Acceptance: an entity on two fixture sites shows each
  occurrence with its source; rejecting a `same_as` separates them again.
- [ ] 12.3 Recapture and diff: changed, added and removed statements between snapshots of the
  same pages. Acceptance: the fixture's second "day" yields exactly its scripted changes.

### 13. Structured retrieval, explanation, and evidence

- [ ] 13.1 `ask`: graph-guided selection of statements and passages with heading paths;
  answers cite item IDs, citations are checked. Acceptance: on a fixture question set, compared
  with plain chunk retrieval (and GraphRAG or LightRAG as baselines), answer correctness and
  citation accuracy are recorded; unsupported questions abstain.
- [ ] 13.2 `explain`: statement → anchors and capture → extraction method and checks → crawl path
  → coverage. Acceptance: every answer item resolves on all four levels; coverage matches the run
  log; disagreements are shown with both sides.
- [ ] 13.3 Evidence research: `evidence CLAIM` (argument or stdin) matches claims and entities to
  the overlay as supporting, contradicting or unknown, then researches additional sites chosen
  from a configured source list and off-site links, admitted only by allowlist or confirmation
  (GAPS Q12), with unauthenticated agentic crawls. Messages are not read by wish (GAPS Q10).
  Acceptance: a labelled claim set against several fixture sites; injected instructions in the
  claim text or on pages have no effect; unlisted, private-network and loopback sites are never
  fetched; the report lists the coverage.
- [ ] 13.4 `trend TOPIC`: changes over time between captures and the spread across sites, with
  statements traced to one origin counted once; recaptures proposed, never automatic.
  Acceptance: scripted changes over three fixture "days" on two sites are reported with
  citations; a copied statement isn't counted as independent.

### 14. Export and interoperability

- [ ] 14.1 Document and version the export (captures, WARC, `wish-overlay/1`, decision records),
  with JSON Schemas and an example workspace. Acceptance: an importer written only from the
  documentation (a small script in the tests) reads the example; profiles, keys and the index are
  never in an export. No consumer-specific code; Lakeshore or any other consumer imports on its
  own side.

## Workspace substrate: phase 1 of 2026-10-14

The roadmap of 2026-10-14 proved CSV import → shell transformation → Python notebook/plot
→ save/reload, plus equivalent `//` commands and replay. Compose through files,
byte/text pipes, and JSON/CSV. Evaluate just-bash before writing shell machinery.
Since 2026-11-10, groups 2, 3.1–3.2 and 6 are also site-workspace phase 1 work, run in the
local host rather than a browser worker (ARCH, GAPS Q6); the rest is used where a site
workflow needs it ([ROAD](ROAD.md#relationship-to-the-workspace-substrate)).

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
- [ ] 9.2 Implement the recurring release security gate in [AGENTS.md](../../AGENTS.md).
  Acceptance: run hostile-output/runtime, file-access/bridge, AI disclosure, egress,
  and applicable server tests against every exact release candidate; include secret
  and dependency scans. Earlier passing releases never waive the gate.
- [ ] 9.3 Prepare reproducible artifacts, license notices, supported-browser results,
  operational limits, and storage recovery/migration instructions. Acceptance:
  clean build, installation, export recovery, and applicable upgrades are exercised.
- [ ] 9.4 Prepare the phase 1 release PR with TODO IDs, tests, security evidence,
  limitations, and release notes. Acceptance: a human reviews/merges through GitHub
  and authorizes publication; AI never merges or releases.

## Workspace substrate: phase 2 of 2026-10-14, selected extensions

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
- [ ] 9.6 Automate the site workspace's phase 1 acceptance ([ROAD](ROAD.md#phase-1-one-site))
  and add the site-capture security tests W1–W10 ([SEC](SEC.md#site-capture-and-crawling)) to
  the recurring release gate. Acceptance: all run against every release candidate on the fixture
  sites; supported browsers and OSes recorded; nothing contacts a real site.
- [ ] 9.7 Extend 9.6 for each selected phase 2 or 3 capability (11.3–14). Acceptance: its
  behavioural and W-series tests pass; only tested capabilities are in a human-reviewed release.
