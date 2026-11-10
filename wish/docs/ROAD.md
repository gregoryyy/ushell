# Roadmap

Where wish goes, phase by phase. Each phase ends with a gate: a tested result that decides whether
the next work starts. The milestones that implement a phase are in [TODO](TODO.md), completed work
in [DONE](DONE.md), the architecture in [ARCH](ARCH.md), and open questions in [GAPS](GAPS.md).
Decisions: 2026-10-14 (lean browser workspace), 2026-11-10 (websites as the space to work on,
[IDEA](IDEA.md); the phase order below, host, crawl policy, store and AI configuration decided
at 10:38, GAPS Q5–Q11; a separate project and repository, Q13). Status: plan; nothing is
implemented.

| Phase | Goal | TODO | Gate to the next phase |
| --- | --- | --- | --- |
| 1. One site | Open a site (logged in or not), move through its page, site and link namespaces, capture a reference overlay and a mirror with a bounded deterministic crawl; the same with `//` and replay, with a configurable provider, masking and synonymization | 10, 11.1–11.2, 2, 3.1–3.2, 6, 9.6 | The acceptance below passes on the fixture sites, and the release gate in [AGENTS.md](../../AGENTS.md) on the candidate |
| 2. Content-driven | Agentic crawling with recorded link decisions, template extraction, data capture, the overlay across sites, structured retrieval with explanations | 11.3–11.5, 12, 13.1–13.2, 9.7 | Per capability: its acceptance and security tests; the crawl comparison in 11.3 |
| 3. Evidence and interop | Claims and topics researched on the captures and additional sites, trends over time and across sites, the open export format | 13.3–13.4, 14 | A claim set scored against fixture sites; an independent importer reads the export |
| Substrate | The browser-workspace concept of 2026-10-14: files, notebook, Wasm, pages, remote resources | 1, 3.3–3.5, 4, 5, 7, 8, 9.1–9.5 | Used where a site workflow needs it; otherwise as in [WORKSPACE](WORKSPACE.md) |
| Not scheduled | Typed pipelines, public resource handles, transparent local/remote placement | 8.5 | As before: an acceptance example that files, JSON, names and explicit commands can't express |

## Phase 1: one site

Deliver one workflow on one site, against local fixture sites only: open it in a headed
browser, log in there, `cd`/`ls`/`cat` through the page and site namespaces and `@links`,
capture the visited pages as references, run a bounded crawl, mirror the site, and reopen
everything offline from the captures. The user can request the same with `//`, inspect the
generated program, and rerun it without a model.

### Host and capture

wish runs on the user's machine as a local process with Playwright driving a browser
([ARCH](ARCH.md), GAPS Q6), a program of its own beside ush (Q11). The user logs in through the headed browser; wish keeps the session
in its own profile and never sees the password. The capture engine produces one snapshot per
page: final URL, time, digest, the semantic tree with anchors, the outgoing links, and, by
policy, content and WARC records.

### Namespaces

The page tree is derived from Playwright's accessibility snapshot and the DOM; path names come
from roles and accessible names. The site tree comes from captured URLs; the link graph from
each snapshot's links. Ordinary commands work on them read-only, through the shell's
filesystem interface. Live entries and captured entries are labelled; a live navigation is a
checked action.

### Deterministic crawl and mirror

A bounded breadth-first crawl with scope, depth, rate, budget and the navigation-safety
policy, in two capture modes: `--refs` and `--mirror`. The robots policy is configurable per site
profile; by default it is honoured for unauthenticated crawls and shown for logged-in ones
(GAPS Q7). The crawl reports why it stopped and what it didn't reach.

### Shell and `//`

The shell and `//` milestones of the substrate (TODO 2, 3.1–3.2, 6) apply as written, with
site commands added to the command set the translator sees. The provider is configured per site
profile, Ollama with a 13B–30B model included; configurable masking and synonymization apply
before any content reaches a model; a remote provider needs a per-site disclosure grant, separate
from capture (TODO 10.7, GAPS Q9).

### Acceptance and phase boundary

Phase 1 passes when automated tests on the fixture sites cover: login through the headed
browser with the session kept out of captures, exports, logs and AI context; the three
namespaces on live and captured pages; stable paths across an unchanged recapture and
"moved/missing" on a changed one; a refs crawl and a mirror of the same site, reopened offline;
scope, budget and rate limits; the navigation-safety traps (logout, delete links, state-changing
GETs, infinite calendars, cross-site links); injected instructions in page content; equivalent
`//` execution and replay with a mock remote provider and a local-model profile; masking and
synonymization (no masked value reaches the provider, stand-ins are reversed in the answer);
cancellation; and denied AI disclosure. No real site is contacted
by any test.

Phase 1 does not need agentic link decisions, data capture beyond `--refs`, the cross-site
overlay, a web UI, a notebook, or Lakeshore.

## Phase 2: content-driven crawling and the overlay

| Capability | Initial approach | Evidence needed before expanding it |
| --- | --- | --- |
| Agentic link decisions (11.3) | Frontier with link features; the model ranks and labels candidates by ID; the policy admits; decision records | On the fixture sites, reaches the goal's pages with fewer fetches than breadth first at equal recall; injected text changes no policy outcome |
| Template recognition and extraction (11.4) | Cluster pages by structure; model-proposed schema per template, checked on a second exemplar; deterministic extraction afterwards | Extraction precision/recall on labelled fixture pages; model calls grow with templates, not pages |
| Data capture (11.5) | `--data`: selected passages, tables and values with anchors | Re-extraction from the capture equals the live result on unchanged pages |
| Overlay across sites (12) | Entities and statements in an open format; `/sem` namespace; identity links proposed, never merged silently | Browsing the same entity across two fixture sites shows each occurrence with its source |
| Structured retrieval (13.1) | Graph-guided selection of statements and passages with heading paths; cited answers | Against plain chunk retrieval on the same captures: answer correctness and citation accuracy on a question set |
| Explanations (13.2) | `explain` over statements, extraction methods, crawl paths and coverage | Every answer item resolves to an anchor and a path; coverage statements match the run log |
| Notebook over captures | The substrate's notebook (TODO 4) on overlay files | An analysis that shell commands and `jq` can't do reasonably |

## Phase 3: evidence, trends and interoperability

- **Evidence (13.3):** `evidence CLAIM` (argument or stdin) matches the claim to the overlay,
  then researches additional sites, chosen from a configured source list and off-site links and
  admitted by allowlist or confirmation (GAPS Q12), with unauthenticated agentic crawls.
  Messages stay outside wish; an external tool passes their text in. Claim text is untrusted.
- **Trends (13.4):** changes between captures of the same pages over time and the spread of a
  statement across sites, each finding cited. Recapture is proposed, never started without the
  user.
- **Export (14):** wish's own, versioned, open export format: overlay statements, anchors,
  capture metadata, decision records, with WARC for content. Any consumer, Lakeshore among them,
  imports it on its own side. wish takes no dependency on a consumer.

## Relationship to the workspace substrate

The browser-workspace roadmap of 2026-10-14 (CSV import → shell → notebook plot →
save/reload) is no longer the phase 1 gate (GAPS Q5, 2026-11-10). Its milestones keep their IDs:
the shell (2), checked files (3.1–3.2) and `//` (6) are phase 1 work for the site workspace; the
notebook (4) is an analysis option in phase 2; the browser-only host (1.2, IndexedDB, Pyodide,
runtime origins) applies only if wish gets a browser-hosted UI (GAPS Q6). The remaining phase 2
extensions of that roadmap (5, 7, 8) stay a menu, each selected for a demonstrated workload.
[WORKSPACE](WORKSPACE.md) keeps the concept; [ARCH](ARCH.md) and [SEC](SEC.md) keep its
architecture and security profile in their later sections.

## Implementation order and process

First the move to wish's own repository (TODO 0.11, GAPS Q13), then phase 1: TODO 10.1–10.2 (host decision, fixture sites), then 2.1 (shell spike, now in Node),
10.3–10.7, 11.1–11.2, 3.1–3.2, 6, and 9.6. Phase 2 and 3 items are selected one at a time.
Milestone numbers are stable task IDs, not phase numbers.

All implementation happens on feature branches, includes tests, and adds DONE entries. Merges
happen through human-reviewed GitHub PRs; AI never merges.
