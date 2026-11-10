# wish: a shell for working on websites

wish is the web interactive shell. Its working space is a **website**: the pages of one site,
the structure inside each page, and the links between them. You move through a site the way you
move through a filesystem. You capture what it means, follow its links by content rather than
by URL pattern, and keep a semantic overlay, with evidence, that you can browse and question
later across sites.

Status: idea and design, 2026-11-10 (decisions on GAPS Q5–Q11 at 10:38); nothing described here is implemented. It
re-centres the browser-workspace concept of 2026-10-14, which stays as wish's substrate
(shell, files, `//`, history, notebook) in [WORKSPACE](WORKSPACE.md). The phases are in
[ROAD](ROAD.md), the architecture in [ARCH](ARCH.md), security in [SEC](SEC.md), prior art in
[SOTA](SOTA.md), and the decisions and open questions in [GAPS](GAPS.md).

## The idea in one paragraph

A crawler sees a site as a set of URLs to fetch. A person sees it as a body of information:
a product catalogue, a project wiki, a forum, a customer portal, each with sections, items,
categories and references. wish gives that second view a shell. The page you are on is the
current directory and its semantic structure (landmarks, sections, tables, lists, forms) is
what `ls` shows. Other pages and the hyperlink graph have namespaces of their own. Crawling
becomes an agent's task with a stated goal: it reads the page's logical structure, decides
which links lead to the information it wants, and records why. What it finds becomes a
**semantic overlay** of statements that point back to exact places on exact pages at exact
times, so every answer drawn from it can be explained down to the evidence and the path that
reached it.

## Use cases

| # | Use case | What the user gets |
| --- | --- | --- |
| U1 | **Semantic overlay of a site you're logged into.** You want to know what a site holds and how it relates, not copy it. | References: the semantic structure (entities, topics, relations, page roles) anchored to page locations, without the page bodies. Optional data capture keeps selected content (tables, values, passages) as data. |
| U2 | **Hard mirror.** You want the site's structure and content, completely and faithfully. | A browsable mirror plus a standard web archive (WARC) of every page and resource fetched, with the URL tree as directories. |
| U3 | **Browse semantics across sites after capture.** You captured several sites and want to move through what they mean together. | One semantic namespace over all captures: the same entity or topic across sites, each occurrence with its source. Works offline, on captures. |
| U4 | **AI analysis for trends and evidence.** A claim or topic comes up, for instance in a message that arrived elsewhere. | The evidence that supports or contradicts it, researched in the captures and on additional sites, and the trend over time and across sites, every finding cited and explained. Messages themselves are handled outside wish; their text can be passed in. |

The use cases share one engine: capture with Playwright, the three namespaces, the overlay
and its evidence. U2 is the "hard" end of a scale whose "soft" end is U1: the same crawl with a
different capture policy.

## Websites as the space to work on

The workspace of [WORKSPACE](WORKSPACE.md) held files, notebooks and tools. wish's subject is
now a site, and the workspace files are where captures, exports and analyses land.

**A site, not a page and not the web.** The unit of work is one site (an origin or a declared
set of origins: `docs.example.org` and `example.org/docs`). Within it, wish navigates, crawls
and captures. Links leaving the site are recorded as references, never followed in a logged-in
session; another site is another workspace entry, opened explicitly. Cross-site work (U3, U4)
happens over captures.

**Next-generation crawling** means three changes against a classic crawler:

1. **Structure before text.** A page is read as its rendered, logical structure (the
   accessibility tree and DOM, with headings, landmarks, lists, tables, forms and their
   accessible names), not as a bag of text or a list of `href`s. Page templates are recognised
   as types: forty product pages with one layout are forty instances of one type.
2. **Content decides where to go.** Links are judged by what they lead to, given the goal:
   their role on the page (navigation, content, pagination, footer), anchor text and context,
   the template of pages already seen behind similar links, and the information still missing.
   A model proposes; a deterministic policy decides (scope, budget, safety). Every decision is
   recorded with its reasons.
3. **Meaning is the output, not files.** The result is an overlay of statements with
   evidence, which a shell, an agent or an analysis can query. The mirror is one possible
   by-product.

## A filesystem over a site

The shell keeps ordinary shell operations (`cd`, `ls`, `cat`, `find`, `grep`, pipes,
redirection) and gives them three kinds of place, plus the overlay and the captures. Each has
its own structure, because a page's interior is a tree, a site's URLs are a tree of a different
kind, and hyperlinks are a graph.

| Namespace | Shape | What a path names | Analogy |
| --- | --- | --- | --- |
| **Page** (`.` when you are on a page) | Tree | A semantic node of the page: `main/`, `section:Pricing/`, `table:Plans/`, `row:3` | The current directory |
| **Site** (`/site/<host>/<url-path>`) | Tree | A page, by its URL path; entering it makes its page tree the current directory | The directory hierarchy |
| **Links** (`@links` in a page, `links` command) | Directed graph | A hyperlink with its role, anchor text, target and state (seen, captured, out of scope, refused) | Symbolic links, plus backlinks |
| **Overlay** (`/sem/<type>/<entity>`) | Graph, browsed as a tree | An entity or topic and its statements, across pages and sites | An index, or a symlink farm into the site trees |
| **Captures** (`/cap/<run>`) | Flat, by run | A capture run: its snapshots, WARC, crawl decisions and log | A build or log directory |

Page paths are built from roles and accessible names (`section:Pricing`), with an index where
names repeat (`row:3`, `link:More[2]`). The same path then finds the same node in a later
capture of the same page when the page hasn't changed its structure; when it has, the node is
reported as moved or missing rather than silently re-targeted.

```text
site open https://wiki.example.org --login      # headed browser; you log in there, wish keeps the session
cd /site/wiki.example.org/projects/atlas        # navigate (a policy-checked page load) or enter a capture
ls                                               # banner/  nav:Main/  main/  contentinfo/
ls main/section:Milestones                       # heading:Milestones  list:Milestones/  table:Owners/
cat main/section:Milestones/table:Owners         # the table as CSV, with its anchor on stderr
ls -l @links | grep -v nav                       # content links: role, anchor text, target, state
links --to /site/wiki.example.org/projects/atlas # backlinks within what has been captured

crawl --goal "every project's milestones and owners" --refs --budget 200
                                                 # agentic crawl, references only
explain /sem/project/atlas/owner                 # statement -> evidence -> page -> crawl path
ls /sem/project                                  # entities found, across captured sites
// which projects changed owners since last month's capture?
```

The `//` line is translated into ordinary wish commands (a diff of two captures over the
overlay, here), displayed, validated and run, as defined in [WORKSPACE](WORKSPACE.md). History
keeps the generated program so it can run again without a model.

**AI, configurable.** The provider is configured per site profile: a remote API with the user's
key, or a local model through Ollama; models of 13B to 30B parameters are the design target for
local use, so every model task (link ranking, schema proposal, extraction, answers with
citations) is kept within what they handle. Before content goes to a model, configurable
**data masking** (patterns such as e-mail addresses, numbers, IDs) and **synonymization**
(consistent stand-ins for names and identifiers, reversed locally in the answer) apply.

**Live and captured.** A page directory is either backed by a live session (you opened the
site, the page is loaded) or by a capture (a snapshot taken at a known time). Each listing says
which. Entering a page that hasn't been captured, in a live session, loads it: that is a
navigation on a site where you may be logged in, so it goes through the same safety policy as
the crawler ([SEC](SEC.md#site-capture-and-crawling)). Captured pages never load anything.

## Capturing: references or data

Capture runs Playwright: a real browser renders each page as the user would see it, with the
user's session if they logged in. What is kept depends on the capture policy:

| Policy | Keeps | For |
| --- | --- | --- |
| `--refs` (default for logged-in sites) | Per page: URL, final URL, time, content digest, the semantic tree's shape and anchors; the statements derived from it (entities, types, relations, page roles) | U1: knowing what is where, without a copy |
| `--data` | Additionally the content selected by the extraction: passages, tables, values, the anchored text | U1 with data, U3, U4 |
| `--mirror` | Every response fetched (HTML, CSS, scripts, images) as WARC, plus a browsable file tree | U2 |

References are anchored with the W3C Web Annotation selectors: a semantic path, a CSS or ARIA
locator, and a text position and quote digest. In `--refs` mode, re-reading the anchored
content needs the live site and its session; wish says so rather than pretending to hold the
content. `--refs` minimises what is copied; it isn't anonymisation: the names and labels in
the overlay are information from the site.

## Agentic crawling according to content

A crawl has a **goal** (stated or generated by `//`), a **scope** (origins, path prefixes,
depth), a **budget** (pages, time, model calls, cost) and a **capture policy**. Each step:

1. **Read** the page's logical structure, and match it against known templates. A new
   template gets an extraction schema proposed by the model on one exemplar and checked on a
   second; later pages of the same template are extracted deterministically, with the model
   used only where extraction fails a check. This is wrapper induction with a model in the
   loop, and it keeps the cost of large sites close to that of a classic crawler.
2. **Extract** statements from the structure: headings give topics, tables and definition
   lists give attributes, lists give members, breadcrumbs and navigation give categories,
   schema.org and other embedded metadata give types directly. Each statement carries its
   anchor and its method.
3. **Decide links.** Every link of the page enters the frontier with its role, anchor text,
   context and target pattern. The model ranks candidates against the goal and labels their
   semantic relation (detail of, next page of, category of, translation of, unrelated); the
   policy admits or refuses each one (scope, budget, the configured robots policy, the
   navigation-safety rules, duplicates). The model chooses only among links present on
   captured pages; it never writes a URL.
4. **Record** the decision: what was followed or skipped, why, with which inputs, by which
   model or rule. These records are what `explain` reads.
5. **Stop** when the goal is met, the budget is exhausted, or the frontier holds nothing the
   policy admits; the run reports which of these, and what it didn't reach.

The deterministic crawl (U2, or `crawl --all`) is the same loop with the model step replaced
by a rule (follow every admitted link, breadth first).

## Structured retrieval and deep explainability

The overlay is the retrieval structure. A question (`ask`, `//`, or a claim to research in U4)
is answered from statements and their anchored passages, chosen through the graph: entities,
their relations, page types and the semantic paths that give a passage its context (the
heading path `Pricing › Enterprise › Limits` travels with the passage). This is **structured
RAG**: retrieval over a graph built with provenance, not over anonymous text chunks.

Every answer is explainable on four levels, each a command:

- **What**: the statements used, each with its source page, anchor, quote and capture time.
- **How it was read**: the extraction method (template schema, rule, model and prompt
  version) and the checks it passed.
- **How it was reached**: the crawl path to the page and the link decisions on the way.
- **What wasn't seen**: the coverage behind the answer: pages in scope never captured, budget
  stops, refused links, stale captures. "None found" is reported as "none found in 140 of
  about 600 linked pages", never as "none".

Statements that disagree (two pages, two captures, two sites) are shown as a disagreement with
both sides, not blended into one answer.

## Evidence and trends from additional sites (U4)

`evidence CLAIM` and `trend TOPIC` research a claim or topic across sites. The text comes as an
argument or on stdin, so a mail client, chat bot or script outside wish can hand over what a
message says; wish itself reads no mailboxes or feeds (decision of 2026-11-10).

1. **Captures first.** Claims and entities in the text are matched to the overlay: supporting,
   contradicting (same subject and property, different value) or unknown.
2. **Additional sites.** Where the captures don't settle it, wish proposes sites to research:
   from a configured source list for the topic and from off-site links in existing captures.
   A site enters scope only from the allowlist or with the user's confirmation (GAPS Q12); the
   research crawl is an ordinary agentic crawl with that claim as its goal, without a login.
3. **Trends.** Recaptures of the same pages over time and the spread across sites show what
   changed, since when, and how many independent sites state it; repeated statements traced to
   one origin count once.
4. **Report.** A file with every finding cited and explainable on the four levels above,
   disagreements shown with both sides, and the coverage: which sites and pages were searched.

The claim text is untrusted input, exactly like page content.

## What stays from the workspace concept

The shell (just-bash as first candidate), checked workspace files, `//` translation with the
user's key, history and replay, and jobs with cancellation carry over unchanged in intent
([WORKSPACE](WORKSPACE.md)). Captures, the overlay and the decision records are workspace
files in open formats, so `jq`, `grep`, a notebook or another tool can work on them. The
notebook becomes an analysis tool for captures (phase 2). Wasm, devices and remote compute
stay optional ideas. wish is its own program, separate from ush, and shares ush's `//` request
contract (GAPS Q11).

## Open source, and Lakeshore as one consumer

wish is open source (MIT, [GAPS](GAPS.md) Q4) and self-sufficient: it is built only on open
components and on its own code. It doesn't use Lakeshore's code, schemas, services or models,
and it doesn't need Lakeshore to work. (Decision by the user, 2026-11-10.)

Lakeshore, a separate, permission-aware semantic context layer over an organization's systems,
can **consume** wish: a site captured by wish is one more source, read through wish's open
export format (statements, anchors, captures, decision records; [ARCH](ARCH.md#export-and-interoperability)).
The importer lives on Lakeshore's side. The two share ideas that predate both (evidence for
every statement, models propose and tools decide, disagreements shown rather than blended,
source permissions carried along), but wish implements its own versions with open
components, and its export format is specified and versioned in wish.

## What is not new, and what would be

Each ingredient has prior art ([SOTA](SOTA.md#crawling-capture-and-semantic-extraction)):
browser automation (Playwright), crawling frameworks (Crawlee, Scrapy), LLM crawlers and
extractors (Crawl4AI, Firecrawl, Browser Use, Stagehand), web archiving (WARC, Browsertrix,
ArchiveBox), wrapper induction and web knowledge graphs (Diffbot), Web Annotation, GraphRAG.
TabFS showed browser resources as files. What wish would add is the combination, in an open
tool for one person: a shell over a site's three structures, content-driven crawling with
recorded decisions, a reference-only overlay of sites the user is logged into, and answers
explained down to evidence, path and coverage. It is unproven until the phase 1 acceptance
in [ROAD](ROAD.md) passes on fixture sites.
