# Gaps

Open questions and explicit assumptions for wish. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted because they seemed clear enough
not to ask; check them and correct any that are wrong. The rules for this file are in
[SDLC](../../docs/SDLC.md#work-items). Status: 2026-11-10 10:38.

- Answers carry date, time and author: `**Answer** (2026-10-27 15:10, Gregor Heinrich): …`. Short
  answers are fine. A changed answer goes on top; earlier ones stay below it, struck through.
- Once answered, the decision moves into the document it affects, and the entry gets a
  `**Resolved**` line saying where. Resolved entries stay as a record.
- A question that blocks a TODO item is marked **blocking**. Design questions that a spike is meant
  to answer (just-bash, the notebook UI) live in [TODO](TODO.md), not here.

## Questions

### Q1. When does wish implementation start?

wish has been design-only since 2026-10-14, while ush was built and is heading for its first
release ([ush ROAD](../../ush/docs/ROAD.md)).

- **Proposed:** after the ush 0.1 release, starting with TODO 1.1 and the just-bash spike (2.1).

### Q2. Which browsers does phase 1 support? — blocking for TODO 1.1

The persistence, worker and isolation choices depend on it (IndexedDB, cross-origin isolation,
Pyodide).

- **Proposed:** current Chrome and Firefox on the desktop; Safari once phase 1 works, because its
  storage limits and eviction rules differ most.

### Q3. Which AI providers does phase 1 offer? — blocking for TODO 6.1

Calling a provider directly from the browser needs the provider to allow it; otherwise a broker
carries the key, which changes the security profile ([SEC](SEC.md)).

- **Proposed:** Anthropic with direct browser access and the key held in session memory, as ush
  does; a local Ollama as the second option, behind the same loopback rules as in ush.

### Q4. Which licence?

The repository has no licence file. The same question for ush is [ush GAPS](../../ush/docs/GAPS.md)
Q1; one answer covers both.

**Answer** (2026-10-28 21:20, Gregor Heinrich): MIT. **Resolved:** [LICENSE](../../LICENSE).

### Q5. Does the site workspace replace the CSV/notebook workflow as phase 1? — blocking for TODO 10.1

On 2026-11-10 wish was re-centred on websites ([IDEA](IDEA.md)). The roadmap of 2026-10-14 had a
local CSV → shell → notebook → save/reload workflow as its phase 1 gate.

- **Proposed:** yes. Phase 1 becomes "one site" ([ROAD](ROAD.md#phase-1-one-site)); the shell,
  checked files and `//` (TODO 2, 3.1–3.2, 6) are built for it; the notebook becomes an analysis
  option for captures in phase 2; the remaining substrate items stay open as a menu
  ([WORKSPACE](WORKSPACE.md)).

**Answer** (2026-11-10 10:38, Gregor Heinrich): yes. **Resolved:** [ROAD](ROAD.md) phases, [TODO](TODO.md)
site workspace.

### Q6. What hosts wish: a local process, a browser extension, or a web app? — blocking for TODO 10.1

Playwright drives browsers from a process, so a pure web app can't capture sites. This changes
assumption A1 for the site workspace.

- **Proposed:** a local TypeScript/Node.js program with a terminal shell (like ush) and a headed
  Playwright browser for login and watching the crawl; Chromium first, Firefox and WebKit through
  Playwright when phase 1 works. A loopback web UI for browsing the overlay can follow, with its
  own origin and CSRF analysis in SEC. A browser extension is not planned. A1 then holds only for
  a browser-hosted substrate.

**Answer** (2026-11-10 10:38, Gregor Heinrich): yes. **Resolved:** [ARCH](ARCH.md#decisions); A1 corrected.

### Q7. What crawling policy applies to logged-in sites and mirrors? — blocking for TODO 11.1

Crawling a site one is logged into can break its terms and can have side effects ([SEC](SEC.md#site-capture-and-crawling) W1, W7).

- **Proposed:** wish works only with the user's own access, never bypasses login, paywalls or
  CAPTCHAs; robots rules are honoured for unauthenticated crawls and shown (not enforced) for
  logged-in ones; logged-in crawls default to concurrency 1 and one page per two seconds; mirrors
  of logged-in sites need an explicit flag; captures are for the user's own use. The user stays
  responsible for the site's terms, and the README says so.

**Answer** (2026-11-10 10:38, Gregor Heinrich): yes, and the robots policy is configurable. **Resolved:**
[SEC](SEC.md#site-capture-and-crawling) W7, [ARCH](ARCH.md#crawl-loop), TODO 11.1.

### Q8. Where does the overlay live? — blocking for TODO 12.1

- **Proposed:** JSON Lines files in the workspace (`overlay/`, `captures/`) as the record, with a
  derived SQLite index (FTS5) that can be rebuilt. No graph database in phase 1–2; revisit if
  query needs outgrow SQLite.

**Answer** (2026-11-10 10:38, Gregor Heinrich): yes. **Resolved:** [ARCH](ARCH.md#decisions), TODO 12.1.

### Q9. May page content go to an AI provider by default? — blocking for TODO 11.3

Agentic link decisions and extraction need excerpts of pages, which may be private.

- **Proposed:** no default disclosure. A grant per site profile and provider; without one, crawls
  and extraction use rules and templates only. For logged-in sites the documentation recommends a
  local model (Ollama, as in ush; GAPS Q3).

**Answer** (2026-11-10 10:38, Gregor Heinrich): the AI provider is configurable; synonymization and data
masking are configurable; Ollama locally is included, and 13B to 30B models are considered
sufficient. **Resolved:** [ARCH](ARCH.md#ai), [SEC](SEC.md#site-capture-and-crawling) W5,
TODO 10.7. Interpretation recorded there: remote providers still need a disclosure grant per
site; a local provider on loopback keeps content on the machine and needs none; synonymization
means consistent stand-ins for names and identifiers, reversed locally.

### Q10. Which message sources come first for evidence and trends (U4)? — blocking for TODO 13.3

- **Proposed:** files dropped into `messages/` (EML/mbox, JSON, RSS/Atom) in phase 3; mailbox,
  chat or feed connectors only for a named workflow, each as its own item.

**Answer** (2026-11-10 10:38, Gregor Heinrich): evidence and trends are researched from additional sites, not
from messages; integration with messages is outside the tool. **Resolved:** [IDEA](IDEA.md)
U4, [ARCH](ARCH.md#evidence-and-trends-from-additional-sites), TODO 13.3–13.4. A claim or topic
reaches wish as an argument or on stdin, so an external tool can pass message text in.

### Q11. Is the site shell part of ush or its own program?

ush is already a local shell with `//`; wish is now local too.

- **Proposed:** its own program (`wish`), sharing the `//` request contract and policy
  semantics with ush ([repository ROAD](../../docs/ROAD.md) step 5) but not its code base.
  ush could later offer `wish` commands as an integration.

**Answer** (2026-11-10 10:38, Gregor Heinrich): a separate program, `wish`. **Resolved:** [IDEA](IDEA.md),
[ARCH](ARCH.md#decisions).

### Q12. How are additional sites chosen for evidence and trend research? — blocking for TODO 13.3

Following Q10, wish researches a claim or topic on sites beyond those already captured.

- **Proposed:** candidates come from a configured source list per topic and from off-site links in
  existing captures; the model may rank them, but a site enters scope only if it is on the
  configured allowlist or the user confirms it. Research crawls run without a site profile
  (unauthenticated), under each site's robots policy. A search-engine adapter is a later,
  configurable option, not phase 3 scope.

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | Phase 1 needs no server: static files, with all workspace data in the browser. | [ARCH](ARCH.md), [ROAD](ROAD.md) | Holds for a browser-hosted substrate only; the site workspace is a local program (Q6, answered 2026-11-10). |
| A2 | The user brings their own AI key and pays for it; wish holds no key of its own. | [IDEA](IDEA.md), [SEC](SEC.md) | |
| A3 | One user per workspace in phase 1; sharing comes only with remote storage (TODO 7.1). | [ARCH](ARCH.md) | |
| A4 | wish captures only sites the user may access, with the user's own accounts, on the user's own machine. | [IDEA](IDEA.md), [SEC](SEC.md) | |
| A5 | wish uses no Lakeshore component; any integration is a consumer importing wish's open export. Stated by the user, 2026-11-10. | [IDEA](IDEA.md), [ARCH](ARCH.md#export-and-interoperability) | |
