# State of the art for wish

Initial research: 2026-10-13; just-bash and phased-scope addendum: 2026-10-14.
Scope: a browser workspace shell for files, data, notebooks,
live tools, WebAssembly, virtual resources, and natural-language commands using
the user's AI API key. The intended design is in [IDEA.md](IDEA.md).

For the lean phase 1, **just-bash is the first shell-library candidate** and
**Pyodide plus one notebook UI** supplies interactive Python. The broader closest
precedents are **WebVM for a browser shell with AI command execution**,
**JupyterLite and marimo for browser Python notebooks**, and **Puter for browser
applications with user-owned cloud resources**. Browser automation libraries cover
the natural-language interaction layer, while WebMCP provides an emerging interface
for pages to expose structured operations.

wish therefore combines several established capabilities. Its useful architectural
distinction would be connected shell/file/notebook workflows with inspectable AI
programs and explicit permissions. A broader shared resource model is conditional
phase 2 work; a custom language or typed pipeline is not the first prototype's goal.
Browser-side computation, AI-generated commands, and bring-your-own-key access are
not individually new.
[PHASES.md](PHASES.md) separates the starting scope from later ideas.

## Method and interpretation

This review uses maintainer documentation, official repositories, and first-party
technical articles. It compares documented interfaces and deployment models; it
does not establish performance rankings or verify security claims through a live
deployment. "Not established" means the reviewed sources did not demonstrate the
capability, rather than proving a product cannot support it. Documentation on a
moving branch is a snapshot, not a version-pinned dependency recommendation.
Maturity assessments also use published releases and deployment documentation;
license classifications use repository license files, package metadata, and vendor
terms checked on the research date. Both must be rechecked for the chosen version.

The important comparison axes are where code executes, how files/data are exposed,
whether tools remain interactive, how AI actions become executable operations,
and who holds credentials and resource authority.

## Comparison

| Tool | Execution and resources | Natural language / user's key | Relationship to wish |
| --- | --- | --- | --- |
| [just-bash](https://github.com/vercel-labs/just-bash/tree/main/packages/just-bash) | TypeScript shell interpreter with browser-compatible core, virtual files, and custom commands | Embedding application supplies wish's BYOK translation and policy | First candidate to reuse parsing/Unix utilities; file/notebook adapters remain wish work |
| [WebVM](https://github.com/leaningtech/webvm) | Client-side Linux-compatible environment, x86-to-Wasm execution, virtual block storage and syscall emulation | Documents a Claude integration accepting the user's key | Closest complete precedent for a shell, virtual machine resources, and AI-assisted local work |
| [JupyterLite](https://github.com/jupyterlite/jupyterlite) | Browser Jupyter UI, worker kernels, editable files, local persistence, rich visualization | A built-in `//` command compiler is not established in the reviewed docs | Strongest notebook/workspace baseline; many file and kernel problems are already solved |
| [Pyodide](https://pyodide.org/en/stable/usage/webworker.html) | Python in a browser worker with a virtual filesystem and JavaScript interoperability | Runtime component; model integration belongs to the host | Direct candidate for wish's local Python kernel |
| [marimo](https://docs.marimo.io/guides/wasm/) | Interactive Python notebooks can execute in-browser through Pyodide or use a conventional Python host | Editor documentation supports configurable AI providers and user keys; that does not establish identical AI support in Wasm deployments | Strong precedent for notebooks that become interactive applications |
| [PyScript](https://github.com/pyscript/pyscript) | Browser Python applications with Pyodide/MicroPython and web UI integration | Host application supplies AI integration | Useful for page-level tools built from Python rather than a full notebook environment |
| [WebContainers](https://webcontainers.io/guides/introduction) | In-tab Node.js runtime, virtual files/processes, development servers and app previews | Supports embedding in AI applications; model and key policy are the host's responsibility | Strong JavaScript development-workspace infrastructure |
| [Wasmer / WebAssembly.sh](https://github.com/wasmerio/webassembly.sh) | Repository currently describes a Wasmer SDK Bash terminal; [wasmer.sh](https://www.wasmer.sh/) exposes language/runtime workspaces | An integrated wish-style `//` compiler is not established here | Direct precedent for Wasm executables behind a familiar terminal |
| [DuckDB-Wasm](https://duckdb.org/docs/current/clients/wasm/overview) | In-browser SQL engine and web shell; files, HTTP data and buffers through a virtual filesystem | AI translation can be added by an embedding application | Candidate for analytical datasets and data commands |
| [SQLite Wasm](https://sqlite.org/wasm/doc/trunk/index.md) / [PGlite](https://pglite.dev/docs/) | Embedded databases; SQLite provides browser persistence options, and PGlite supports memory or IndexedDB-backed browser Postgres | No wish-style AI shell established by these sources | Mutable data resources behind a common workspace API |
| [Puter.js](https://docs.puter.com/) | Browser-facing APIs for authentication, cloud files, databases and application services | Its advertised model provides AI without application API keys, unlike wish's direct provider-key model | Closest resource-service precedent for server-backed work that feels local |
| [TabFS](https://omar.website/tabfs/) | Extension/native filesystem bridge exposing live tabs to an ordinary desktop shell | Model integration can sit outside the bridge | Strong precedent for treating browser resources as composable shell objects |
| [Tridactyl](https://github.com/tridactyl/tridactyl) | Browser command mode and ex commands, configuration and optional native integration | A built-in notebook/resource/AI execution model is not established here | Useful precedent for discoverable browser command interaction |
| [Browser Use](https://github.com/browser-use/browser-use) | External agent runtime controls a browser; local and cloud browser configurations | Provider-specific wrappers support the user's provider keys | Agent execution precedent; its primary resource is a browser session |
| [Stagehand](https://docs.stagehand.dev/v4/first-steps/introduction) | Browser automation combining deterministic APIs with AI actions and extraction | Natural-language `act`, structured `extract`, and `observe` | Useful separation of model-assisted intent from explicit operations |
| [agent-browser](https://github.com/vercel-labs/agent-browser) | Native CLI, browser daemon, accessibility snapshots, element refs and structured output | Designed to be driven by an external AI agent | Strong precedent for a small command surface suitable for models |
| [BrowserOS](https://github.com/browseros-ai/BrowserOS) | Chromium-based agent browser; repository also ships BrowserOS neo for external MCP agents | BrowserOS documents user keys and local Ollama support | Demonstrates local-browser AI control with user-chosen models |
| [Jupyter Server](https://jupyter-server.readthedocs.io/en/latest/developers/rest-api.html) | Server-side files, sessions, kernels and terminal APIs | Model integration is separate | Existing protocol surface for explicitly remote Python sessions |
| [WebMCP](https://developer.chrome.com/docs/ai/webmcp) | Pages expose declared tools through browser APIs | Agent/model choice is separate; developer tooling supports keyed and client-side models | Emerging precedent for wish's explicit page/tool bridge |

## Maturity, licenses, and reuse decisions

The following assessments concern readiness for wish's intended integration, not
just the age of a project. **Established** means a released, documented foundation
for its stated workload; **evolving** means usable released software whose browser
integration or interfaces still need evaluation; **reference** means a working
precedent requiring substantial adaptation; **experimental** means a proposal or
trial that should not be required for the first prototype. These are review
judgments, not maintainer support guarantees or results of integration tests.

License types matter separately from maturity. MIT, BSD, Apache, and the PostgreSQL
License are permissive licenses with their respective conditions. MPL-2.0 has
file-level copyleft; GPL and AGPL have stronger source-sharing requirements, with
AGPL also covering relevant network use of modified software. Public-domain SQLite
and custom vendor licenses need separate treatment. A license for a shell UI, SDK,
or specification does not automatically cover its engine, server, dependencies,
or hosted service. [MPL explanation](https://www.mozilla.org/en-US/MPL/2.0/FAQ/)
and [AGPL text](https://raw.githubusercontent.com/HeyPuter/puter/main/LICENSE.txt).

### Notebook, data, and resource foundations

| Component and evidence | Maturity for wish | License type and scope | Reuse decision |
| --- | --- | --- | --- |
| [just-bash](https://github.com/vercel-labs/just-bash/tree/main/packages/just-bash) | Package documentation labels it beta; browser integration needs a spike | [Apache-2.0](https://github.com/vercel-labs/just-bash/blob/main/packages/just-bash/LICENSE), permissive; dependencies retain their terms | Evaluate first for phase 1; pin the tested artifact and avoid parallel development of a Bash replacement |
| [Pyodide](https://github.com/pyodide/pyodide/releases) | Established browser Python runtime; package and native-extension compatibility remain workload-specific | [MPL-2.0](https://raw.githubusercontent.com/pyodide/pyodide/main/LICENSE), file-level copyleft; Python and bundled packages have their own licenses | First local Python candidate; adapt its filesystem and worker interfaces to the broker |
| [JupyterLite](https://github.com/jupyterlite/jupyterlite/releases) | Evolving browser notebook distribution; documented kernel/file bridge has deployment prerequisites | [BSD-3-Clause](https://raw.githubusercontent.com/jupyterlite/jupyterlite/main/LICENSE), permissive; kernels/extensions are separate | Evaluate notebook UI and file integration before building equivalents |
| [marimo](https://github.com/marimo-team/marimo/releases) | Evolving released notebook/application editor; Wasm and AI deployment parity need checking | [Apache-2.0](https://raw.githubusercontent.com/marimo-team/marimo/main/LICENSE), permissive | Alternative interactive notebook tool; compare against Jupyter integration rather than adopting both initially |
| [PyScript](https://github.com/pyscript/pyscript/releases) | Evolving browser Python application layer | [Apache-2.0](https://raw.githubusercontent.com/pyscript/pyscript/main/LICENSE), permissive; embedded runtimes are separate | Optional live-tool adapter; avoid a second Python execution stack in the initial prototype |
| [DuckDB-Wasm](https://duckdb.org/docs/current/clients/wasm/overview) | Established analytics engine with a released browser client; memory/threading limits require testing | [MIT](https://raw.githubusercontent.com/duckdb/duckdb-wasm/main/LICENSE), permissive | Add for analytical SQL workloads; reuse query execution and data interchange |
| [SQLite Wasm](https://sqlite.org/wasm/doc/trunk/index.md) | Established database engine; browser storage and concurrency depend on the selected adapter | [Public domain](https://sqlite.org/copyright.html) for SQLite deliverable code; wrappers/build dependencies may differ | Candidate for mutable local datasets and metadata if browser persistence fits |
| [PGlite](https://pglite.dev/docs/) | Evolving released browser Postgres; persistence and multi-tab coordination need validation | Client [Apache-2.0](https://raw.githubusercontent.com/electric-sql/pglite/main/packages/pglite/package.json); embedded engine [PostgreSQL License](https://pglite.dev/docs/); both permissive | Add when Postgres semantics are required; do not introduce multiple database engines without a workload |
| [Jupyter Server](https://github.com/jupyter-server/jupyter_server/releases) | Established remote kernel/content/session implementation and protocols | [BSD-3-Clause](https://raw.githubusercontent.com/jupyter-server/jupyter_server/main/LICENSE), permissive; kernels/packages are separate | Reuse for an explicit remote-kernel adapter; wish still owns leases, grants, and tenant isolation |
| [jco](https://github.com/bytecodealliance/jco/releases) | Evolving released Wasm component toolchain; browser/WASI interface coverage varies | [Apache-2.0 with LLVM exception](https://raw.githubusercontent.com/bytecodealliance/jco/main/LICENSE), permissive with an exception | Evaluate for a later typed executable ABI; not a requirement for the first basic Wasm module |
| [Puter.js / Puter](https://docs.puter.com/) | Evolving SDK/platform with a documented hosted resource service | SDK metadata declares [Apache-2.0](https://raw.githubusercontent.com/HeyPuter/puter/main/src/puter-js/package.json); platform repository defaults to [AGPL-3.0](https://github.com/HeyPuter/puter#license), network copyleft | Optional cloud-resource adapter; check artifact notices and service terms before vendoring or hosting |

### Compatibility runtimes and browser-control tools

| Component and evidence | Maturity for wish | License type and scope | Reuse decision |
| --- | --- | --- | --- |
| [WebVM / CheerpX](https://github.com/leaningtech/webvm) | Working VM/shell reference backed by a specialized browser execution engine | WebVM UI [Apache-2.0](https://raw.githubusercontent.com/leaningtech/webvm/main/LICENSE.txt); CheerpX engine [custom Community/commercial terms](https://cheerpx.io/docs/licensing) | Conditional Linux-compatibility adapter; establish engine deployment rights before selecting it |
| [WebContainers](https://webcontainers.io/guides/introduction) | Established vendor-provided Node workspace runtime; browser/header requirements constrain embedding | [Vendor/commercial runtime terms](https://webcontainers.io/enterprise); SDK package licensing is a separate question | Optional Node adapter; prototype first and budget commercial production embedding separately |
| [WebAssembly.sh / Wasmer SDK](https://github.com/wasmerio/webassembly.sh) | Shell reference plus an evolving [released SDK](https://github.com/wasmerio/wasmer-sdk/releases) | Shell [MIT](https://raw.githubusercontent.com/wasmerio/webassembly.sh/master/LICENSE); current SDK [Modified MIT](https://raw.githubusercontent.com/wasmerio/wasmer-sdk/main/LICENSE), with an additional branding condition | Optional Wasm/Bash adapter; inspect pinned SDK and command-package terms rather than relying on the shell's license |
| [TabFS](https://github.com/osnr/TabFS) | Working filesystem-bridge reference; needs native/FUSE integration outside the tab | Repository identifies [GPL-3.0](https://github.com/osnr/TabFS), strong copyleft | Borrow the resource-mapping model; code reuse/native integration is a separate deliberate choice |
| [Tridactyl](https://github.com/tridactyl/tridactyl) | Established Firefox command-mode extension; native features require extra installation | [Apache-2.0](https://raw.githubusercontent.com/tridactyl/tridactyl/master/LICENSE), permissive | Interaction precedent or optional extension adapter; not the notebook/workspace engine |
| [Browser Use](https://github.com/browser-use/browser-use/releases) | Evolving released external browser-agent framework | [MIT](https://raw.githubusercontent.com/browser-use/browser-use/main/LICENSE), permissive; cloud services have separate terms | Reuse for optional external browser control; keep workspace resource authority in wish |
| [Stagehand](https://github.com/browserbase/stagehand/releases) | Evolving released browser-automation SDK; pin matching package/docs versions | [MIT](https://raw.githubusercontent.com/browserbase/stagehand/main/LICENSE), permissive; hosted Browserbase terms are separate | Candidate page-action/extraction adapter; avoid duplicating its general automation stack |
| [agent-browser](https://github.com/vercel-labs/agent-browser/releases) | Evolving released native CLI/daemon; requires an external runtime | [Apache-2.0](https://raw.githubusercontent.com/vercel-labs/agent-browser/main/LICENSE), permissive | Optional external-tab adapter and command-interface reference |
| [BrowserOS](https://github.com/browseros-ai/BrowserOS/releases) | Evolving released Chromium-based browser; distribution/update burden exceeds an in-page library | Project [AGPL-3.0](https://raw.githubusercontent.com/browseros-ai/BrowserOS/main/LICENSE), network copyleft; Chromium/third-party notices are separate | Reference or external integration; creating a wish browser fork is outside the initial scope |
| [WebMCP](https://developer.chrome.com/docs/ai/webmcp) | Experimental browser API/origin trial; interoperability remains unsettled | Spec reports use the [W3C Software and Document License](https://raw.githubusercontent.com/webmachinelearning/webmcp/main/LICENSE.md); implementation licenses are separate | Later optional tool bridge; retain a versioned wish interface as the baseline |

### License boundaries that affect the architecture

WebVM's Apache license covers its application code, not permission to redistribute
the CheerpX engine. Current CheerpX Community terms support specified personal,
open-source, and evaluation uses through the vendor-hosted runtime; self-hosting
or redistribution requires commercial terms. This makes Linux compatibility an
explicit product/deployment decision. [CheerpX licensing](https://cheerpx.io/docs/licensing).

The Wasmer SDK's current license is **Modified MIT**, not plain MIT. Its additional
condition requires prominent Wasmer branding for covered commercial products or
services exceeding one million monthly active users or USD one million monthly
revenue. WebAssembly.sh's MIT license does not remove that SDK condition.
[SDK license](https://raw.githubusercontent.com/wasmerio/wasmer-sdk/main/LICENSE).

Puter's SDK package metadata and platform repository state different licenses.
Inspect the actual distributed SDK artifact and notices before relying on its
Apache declaration, and assess the AGPL backend separately if hosting or modifying
it. Calling a hosted service also introduces its service terms. PGlite similarly
includes an Apache-licensed client and a PostgreSQL-licensed engine; these are
component licenses, not a blanket choice of either license for all included code.
[Puter package metadata](https://raw.githubusercontent.com/HeyPuter/puter/main/src/puter-js/package.json),
[Puter repository terms](https://github.com/HeyPuter/puter#license), and
[PGlite licensing](https://pglite.dev/docs/).

For adopted dependencies, record the exact version, artifact, license/notice files,
and any required source distribution or vendor agreement. Repeat that check for
Python wheels, Wasm binaries, extensions, and bundled commands. These findings
inform dependency selection; they do not select a license for wish itself.

### Where to focus work beyond the state of the art

The first integration candidate is **just-bash plus Pyodide and one notebook UI**,
with wish supplying checked file operations and a narrow notebook bridge. Select
one UI against the file/plot workflow. Compose with files, byte/text pipes, and
JSON/CSV. Reuse SQL engines, Jupyter Server protocols, compatibility runtimes,
external browser agents, or WebMCP only for selected phase 2 workloads.

Development should concentrate on the missing composition contract:

1. **One executable command representation.** Reuse shell parsing/execution, add
   command validation and replay, and make `//` produce the same inspectable program
   accepted by the ordinary shell. Demonstrate replay without another model call.
2. **Checked access to shared files.** Implement path permissions, versions,
   conflict handling, and bounded notebook file exchange. Demonstrate that shell
   and Python obey the same allowed scope. Broader resources/leases follow when added.
3. **Connected interactive views.** Bridge committed resource changes into notebook
   kernels and views, including cancellation and restart. Demonstrate a shell edit
   appearing in a notebook rerun without maintaining incompatible file copies.
4. **Explicit placement and disclosure.** Separate storage location, compute
   location, and AI-provider access. First demonstrate a browser workflow and
   denied AI disclosure; add authorized server saves/reconnect tests in phase 2.

Reuse Python execution, SQL engines, notebook rendering, and general browser
automation rather than rebuilding them. The measure of progress beyond these
precedents is the connected, reproducible workflow described below, not the number
of runtimes embedded in a terminal.

## Closest precedents in detail

### just-bash: a reusable shell, with integration limits

The package documents a browser-compatible shell core, virtual filesystem,
byte-oriented command input/output, and custom-command callbacks. It labels itself
beta. Shell state resets between `exec()` calls while files remain shared; the
browser build excludes its optional Python, JavaScript-execution, SQLite, and
physical-filesystem features. Callbacks can run with host authority, so a host must
control what it exposes. [Package documentation](https://github.com/vercel-labs/just-bash/blob/main/packages/just-bash/README.md).

**Implication for wish:** test browser packaging, file permissions/versioned commits,
cwd/session behavior, one notebook command, and cancellation before adoption. Reuse
the interpreter if these pass; keep Pyodide and checked file adapters under wish's
control. JSON/CSV provides the initial data boundary. Typed record/handle pipelines
are deferred rather than patched into the interpreter for the first release.

### WebVM: AI inside a shared browser terminal

WebVM uses CheerpX to run a Linux-compatible environment in the browser rather
than translating every application into a bespoke browser API. Its repository
documents virtual storage, syscall emulation, and an AI panel that accepts the
user's key. This is a substantive overlap with wish, not merely a terminal widget.
[WebVM repository](https://github.com/leaningtech/webvm).

Leaning Technologies describes the AI integration as executing model-requested
Bash commands inside the same terminal session used by the human, including
collection of command output. It also integrates graphical computer-use tools.
This already demonstrates natural-language-driven work over virtualized browser
resources. [First-party integration article](https://labs.leaningtech.com/blog/webvm-claude).

**Implication for wish:** investigate WebVM if compatibility with existing Linux
programs is central. A smaller workspace broker may be easier to compose with
notebooks and data services when that compatibility is unnecessary. The latter is
an architectural judgment, not a measured performance result.

### JupyterLite and Pyodide: local notebooks with real file workflows

JupyterLite runs a Jupyter environment in the browser, including worker-based Python
kernels, file editing, persistence, sessions, and interactive visualization. It can
be served statically without allocating a Python server per user. The project also
documents differences from full JupyterLab. [JupyterLite repository](https://github.com/jupyterlite/jupyterlite).

The hard part for wish is not just starting Python: files edited by the shell must
be usable by a synchronous kernel. JupyterLite exposes file-browser contents through
shared-memory communication or a service-worker-backed filesystem bridge. Its docs
explicitly identify configurations where those bridges are unavailable.
[Kernel filesystem integration](https://jupyterlite.readthedocs.io/en/stable/howto/content/python.html).

**Implication for wish:** reuse kernel and notebook interfaces where possible.
Define ownership, synchronization, and write conflicts before presenting one shared
filesystem. A browser Python runtime does not automatically supply shell-visible
files or server resource authorization.

### marimo: notebooks as interactive tools, with adjacent AI support

marimo's Wasm notebooks run Python through Pyodide without a Python execution backend.
Its docs cover sharing and embedding these notebooks, supported packages, and
runtime limitations. This closely matches wish's goal of creating interactive tools
from local computation. [Wasm notebook documentation](https://docs.marimo.io/guides/wasm/).

The editor's AI assistant separately documents configurable providers, local
providers, and user API keys. This is evidence for AI-assisted notebook development,
but does not prove that every AI feature works identically in a serverless Wasm
deployment. [AI editor documentation](https://docs.marimo.io/guides/editor_features/ai_completion/).

**Implication for wish:** test whether marimo can be an isolated workspace tool,
with shell commands controlling notebook artifacts and declared resources. Avoid
rebuilding its UI or assuming that its application/kernel/file boundaries match
wish's broker without an integration prototype.

### Puter: user-owned cloud resources through browser APIs

Puter exposes familiar file operations over the user's cloud storage. Its file API
includes reads, writes, directories, uploads, and sharing. Its CLI also distinguishes
local paths from cloud paths and supports transfers between them.
[Cloud storage API](https://docs.puter.com/FS/) and [CLI](https://docs.puter.com/cli/).

Its general API model supplies application services and AI without requiring the
application to bring a model API key. That is a different credential/billing model
from wish's direct use of a user's selected AI provider, even though both make
server-backed resources available to browser applications.
[Puter.js documentation](https://docs.puter.com/).

**Implication for wish:** a resource adapter should hide service-specific file APIs
while exposing location, permissions, and persistence. A cloud filesystem should
not silently turn into authorization for remote code execution or AI disclosure.

## Runtime and data components

WebContainers provide Node.js execution inside a browser tab, with filesystem and
process APIs suited to editors and live previews. Their deployment requires
cross-origin isolation headers because they use SharedArrayBuffer. Commercial
production embedding has licensing requirements; prototypes and proofs of concept
are treated separately by the vendor. [Introduction](https://webcontainers.io/guides/introduction),
[deployment headers](https://webcontainers.io/guides/configuring-headers), and
[commercial usage](https://webcontainers.io/enterprise).

The current WebAssembly.sh repository describes Wasmer plus Ghostty-web and Bash,
so its older WAPM/Wasmer-JS description should not be treated as a current dependency
specification. The current wasmer.sh site offers examples and a fuller shell, and
exposes a WISP server connection for commands requiring outbound networking.
[Repository](https://github.com/wasmerio/webassembly.sh) and [current site](https://www.wasmer.sh/).
**Implication:** inspect the actual command/runtime/network contracts before adopting
an older browser-shell example as a secure offline environment.

DuckDB-Wasm offers a full browser SQL engine, an interactive web shell, Arrow data
exchange, and a filesystem model spanning memory, files, and HTTP sources. Browser
memory and threading constraints still matter. [Official client documentation](https://duckdb.org/docs/current/clients/wasm/overview).
SQLite's Wasm documentation covers persistent storage approaches, while PGlite
provides browser Postgres with memory/IndexedDB persistence and coordination through
a multi-tab worker. [SQLite Wasm](https://sqlite.org/wasm/doc/trunk/index.md) and
[PGlite](https://pglite.dev/docs/).
**Implication:** represent databases as named resources with declared consistency
and file access; do not make a shell pipeline simulate database transactions.

For typed executable interfaces, Bytecode Alliance's **jco** can transpile Wasm
components to ES modules for browser execution, and includes WASI-related shims.
This supplies a practical path toward typed imports/exports without assuming a
browser natively implements every component or WASI interface.
[jco repository](https://github.com/bytecodealliance/jco).
**Implication:** evaluate component interfaces for a later wish ABI; keep resource
authority in wish's host imports and broker.

## Browser automation and emerging page interfaces

Browser Use supports provider-specific model wrappers with each provider's own key;
its optional model gateway/cloud browser are separate configurations. That makes
it useful evidence that model choice and browser placement can be independent.
[Maintainer README](https://github.com/browser-use/browser-use).

Stagehand mixes natural-language actions and structured extraction with explicit
browser control. agent-browser presents snapshots, element refs, and JSON output
as ordinary CLI operations. BrowserOS documents both an integrated BYOK/local-model
browser and a separate browser for external agents.
[Stagehand](https://docs.stagehand.dev/v4/first-steps/introduction),
[agent-browser](https://github.com/vercel-labs/agent-browser), and
[BrowserOS](https://github.com/browseros-ai/BrowserOS).
These are relevant to wish's translator and optional tab adapter; they are not
evidence of a common notebook/storage/compute resource model.

WebMCP is especially relevant to live tool pages. Chrome's current documentation
describes imperative JavaScript tools and declarative form annotations, with an
origin trial from Chrome 149. The API remains under active discussion and is
subject to change, so it is an experimental interoperability target rather than a
portable prerequisite. [Chrome WebMCP documentation](https://developer.chrome.com/docs/ai/webmcp).
**Implication:** define wish's own versioned tool bridge, then map compatible
WebMCP tools into its registry without trusting their descriptions as permission grants.

TabFS and Tridactyl remain useful interaction precedents. TabFS maps live tab state
into a desktop filesystem so ordinary scripts can operate on it. Tridactyl exposes
browser functionality through a command mode and ex commands. Their resource and
deployment models differ from a browser-contained notebook workspace.
[TabFS](https://omar.website/tabfs/) and [Tridactyl](https://github.com/tridactyl/tridactyl).

## Differentiation of wish

wish's phase 1 differentiation is a **usable file/shell/notebook workflow** with
inspectable, replayable AI-generated commands and checked access. The phase 2
opportunity is to extend it to live pages and explicitly selected server resources.
This is an integration goal, not a new Bash dialect or type system. The architecture
is defined in [ARCH.md](ARCH.md), and its scope in [PHASES.md](PHASES.md).

The distinctions below are design intentions inferred from the reviewed tools,
not demonstrated advantages of a working wish implementation. Existing products
may gain comparable integrations, and wish should reuse their components where
they already solve the underlying problem.

| Compared with | Established approach | wish's proposed distinction |
| --- | --- | --- |
| [just-bash](https://github.com/vercel-labs/just-bash/tree/main/packages/just-bash) | Reusable shell syntax, utilities, virtual files, and command hooks | A candidate dependency for a workspace UI, notebook/file integration, BYOK translation, saved artifacts, and access policy |
| [WebVM](https://github.com/leaningtech/webvm), [Wasmer](https://github.com/wasmerio/webassembly.sh), and [WebContainers](https://webcontainers.io/guides/introduction) | Browser-hosted command runtimes, virtual files/processes, and, in WebVM, AI-driven terminal work | A lean workflow built from files and explicit notebook/tool commands; specialized Linux/Node/Wasm compatibility is optional |
| [JupyterLite](https://github.com/jupyterlite/jupyterlite) and [marimo](https://docs.marimo.io/guides/wasm/) | Interactive computing organized around notebooks, kernels, files, and application views | A shell that coordinates multiple tools and kernels over the same named resources, so a file edit, data transformation, and notebook execution participate in one workflow |
| [Puter](https://docs.puter.com/) | Browser applications backed by cloud resource APIs and an integrated service/AI model | Independent choices of storage, compute, and AI provider, with browser-only work available and each attached service represented explicitly |
| [Browser Use](https://github.com/browser-use/browser-use), [Stagehand](https://docs.stagehand.dev/v4/first-steps/introduction), and [agent-browser](https://github.com/vercel-labs/agent-browser) | Browser sessions and page actions exposed to agents or automation scripts | Creation and modification of workspace artifacts and interactive tools as the primary workflow; external-tab control remains an optional resource adapter |
| [WebMCP](https://developer.chrome.com/docs/ai/webmcp) | Pages expose structured, callable tools to browser agents | A host-side shell, resource broker, and execution history that compose page tools with files, kernels, and remote allocations; WebMCP can provide one bridge into that model |

### Commands that remain useful after translation

`//` requests should produce displayed, validated wish programs that enter the same
executor as typed commands. Users can inspect, edit, and rerun the resulting program
without another model call. A program remains subject to the same grants whether
its origin was a person or a model. This gives natural-language convenience a
concrete, reusable representation; the `//` marker and use of a personal API key
are interaction choices rather than standalone technical breakthroughs. WebVM's
[AI terminal integration](https://labs.leaningtech.com/blog/webvm-claude) already
demonstrates substantial overlap in human/model collaboration through commands.

### Shared resources across interactive tools

The important integration is that a shell edit, Python cell, database query, and
live page can use the same workspace files. Versions and write conflicts should
follow the file contract instead of depending on which UI issued an operation.
Phase 1 refreshes a plot through an explicit notebook rerun; general reactive tools
are later work. Resource location and authority remain visible across these
views, and an application should expose a tool interface rather than require DOM
scraping to reach its own operations.

### Explicit placement and resource authority in phase 2

Storage and execution are separate choices. A notebook can use browser compute
with server-backed files, or request a leased server kernel while keeping its
source locally. Changing placement must make data transfer, resource limits, and
cost implications explicit. Backend adapters preserve the logical operation while
declaring differences in package support, latency, consistency, and available
devices; they do not promise identical execution semantics.

The same broker should enforce scoped file, dataset, network, device, and compute
rights for commands, tools, and kernels. Permission to read a file must remain
separate from permission to disclose it to an AI provider. This is a proposed
system-wide policy contract, not evidence that wish is inherently more secure
than the reviewed products. Its limits and required checks are in [SEC.md](SEC.md).

### What would demonstrate the distinction

The phase 1 prototype should import measurements, edit them from the shell, open
a Python notebook, generate a plot, and save/reload its files. Selected phase 2
extensions can add a Wasm transformation and explicit server save.
It should record and rerun the generated commands, propagate committed changes
between tools, reject conflicting writes, and enforce resource grants throughout.
The practical differentiation is demonstrated when that workflow needs little
service-specific glue and remains understandable to its user. Faster execution,
better model accuracy, or stronger security would require separate measurements.

## Architectural conclusion and next research

The reviewed sources establish most individual building blocks, and WebVM already
establishes a substantial combination of browser virtualization and AI shell work.
They do not establish one directly reusable implementation of wish's entire
proposed contract: scoped mounts, shared datasets, notebooks and live tools,
displayed/reusable `//` programs, and interchangeable local/leased server resources.
That is a bounded finding about this review, not a claim that no such product exists.
The original full contract is a set of possible directions, not the lean phase 1
acceptance checklist; phase 2 selects extensions rather than implementing it wholesale.

For the first prototype, evaluate just-bash and Pyodide with one notebook UI and
checked file adapters. Use ordinary byte/text pipes and JSON/CSV. Treat WebVM as
the alternative when Linux compatibility is required, WebContainers as an optional
Node.js runtime, and Jupyter Server as a protocol
candidate for remote kernels. Keep direct provider keys in the trusted transport,
and test tool pages under the isolation model in [SEC.md](SEC.md).

Use the same small workload to compare integrations: import a CSV, change it from
the shell, read it in a Python cell, produce an interactive plot, save/reload the
workspace, and replay an AI-generated program. Extend that workload with Wasm
and remote storage only when selected for phase 2. Test interruption, conflicting
writes, denied grants, and secret handling, plus reconnects when a server is added.
Measure startup/download size, responsiveness, memory use, file semantics, and
package compatibility. Documentation alone cannot settle those choices.
