# State of the art for wish

Researched: 2026-10-13. Scope: a browser workspace shell for files, data, notebooks,
live tools, WebAssembly, virtual resources, and natural-language commands using
the user's AI API key. The intended design is in [IDEA.md](IDEA.md).

The closest precedents are **WebVM for a browser shell with AI command execution**,
**JupyterLite and marimo for browser Python notebooks**, and **Puter for browser
applications with user-owned cloud resources**. Browser automation libraries cover
the natural-language interaction layer, while WebMCP provides an emerging interface
for pages to expose structured operations.

wish therefore combines several established capabilities. Its useful architectural
distinction would be one command language and resource model shared by shell
commands, notebooks, live tools, and explicitly selected local/server execution.
Browser-side computation, AI-generated commands, and bring-your-own-key access are
not individually new.

## Method and interpretation

This review uses maintainer documentation, official repositories, and first-party
technical articles. It compares documented interfaces and deployment models; it
does not establish performance rankings or verify security claims through a live
deployment. "Not established" means the reviewed sources did not demonstrate the
capability, rather than proving a product cannot support it. Documentation on a
moving branch is a snapshot, not a version-pinned dependency recommendation.

The important comparison axes are where code executes, how files/data are exposed,
whether tools remain interactive, how AI actions become executable operations,
and who holds credentials and resource authority.

## Comparison

| Tool | Execution and resources | Natural language / user's key | Relationship to wish |
| --- | --- | --- | --- |
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

## Closest precedents in detail

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

## Architectural conclusion and next research

The reviewed sources establish most individual building blocks, and WebVM already
establishes a substantial combination of browser virtualization and AI shell work.
They do not establish one directly reusable implementation of wish's entire
proposed contract: scoped mounts, shared datasets, notebooks and live tools,
displayed/reusable `//` programs, and interchangeable local/leased server resources.
That is a bounded finding about this review, not a claim that no such product exists.

For the first prototype, evaluate Pyodide with a notebook UI and a small file/resource
broker. Treat WebVM as the alternative when Linux compatibility is required,
WebContainers as an optional Node.js runtime, and Jupyter Server as a protocol
candidate for remote kernels. Keep direct provider keys in the trusted transport,
and test tool pages under the isolation model in [SEC.md](SEC.md).

Use the same small workload to compare integrations: import a CSV, change it from
the shell, read it in a Python cell, produce an interactive plot, save/reload the
workspace, run a Wasm transformation, and explicitly copy one result to server
storage. Then test interruption, conflicting writes, denied grants, secret handling,
and reconnects. Measure startup/download size, responsiveness, memory use, file
semantics, and package compatibility. Documentation alone cannot settle those choices.
