# wish: a browser command shell

wish is the web interactive shell.

Status: design proposal. The commands and interfaces below define intended behavior;
they are not implemented yet.

## Definition

A **browser command shell**, or **web shell**, is an interactive command environment
whose resources are workspace files, datasets, notebooks, live web pages, virtual
devices, compute sessions, browser tabs, and JavaScript and WebAssembly runtimes.
It gives these resources the familiar shell operations of selection, inspection,
execution, composition, and modification.

In wish, a page is a live executable workspace: its scripts, event handlers, rendering,
and WebAssembly applications keep running while the user issues commands. The user
can create and change files, add data and resources, build interactive tools,
invoke application functions, and run computations from the same prompt. DOM
commands are one interface to that workspace.

The shell accepts both explicit commands and natural-language requests prefixed
with `//`. Using the user's own AI API key, wish translates each request into actual
wish commands and executes them through the ordinary shell runtime.

## The browser as a place to work

The primary environment is a workspace owned by the user. Work can stay in browser
memory, persist in browser storage, use explicitly selected local files, or access
authenticated server storage and compute. The shell presents these as named mounts
and resources with visible location, persistence, and access rights. "Local" work
means working in this environment; it does not require every resource to live on
the physical computer or every computation to run on the server.

For example, wish can create a Python notebook, import measurements, execute cells
in a browser kernel, save results, and display a plot as a live tool. The same
notebook can explicitly acquire a remote compute session when its packages or data
exceed browser capabilities. Moving data or execution to a server is a visible
choice, not an automatic consequence of opening the notebook.

Virtual resources give programs scoped access to useful physical capabilities:
storage volumes, memory-backed files, worker compute, an available GPU, simulated
devices, or a leased server process. Each exposes a defined interface and limits.
An actual camera, serial device, or other user hardware needs the browser's own
permission flow and a dedicated adapter. Browser virtualization does not imply
unrestricted access to the host machine.

The technical model is defined in [TECH.md](TECH.md), its trust boundaries and
browser obstacles in [SEC.md](SEC.md), and related tools in [SOTA.md](SOTA.md).

## Browser resources and shell state

| Shell concept | Browser equivalent |
| --- | --- |
| Current working context | Workspace, working directory, selected tool/page, and kernel |
| Addressable resources | File paths, dataset names, resource handles, URLs, selectors, and modules |
| Executable | Built-in command, registered page function, JavaScript module, or Wasm command |
| Standard input/output | Streams of text, JSON records, bytes, or scoped resource handles |
| Process/job | Command pipeline, page task, or worker computation |
| Filesystem | Mounted memory, browser storage, selected local directories, or server storage |
| Data and devices | Named datasets, database sessions, and scoped virtual resource handles |
| Environment/history | Session settings, selected context, commands, and results |

The selected page stays explicit; changing browser focus does not silently retarget
a running command. Handles belong to a particular document. Navigation or reload
invalidates its DOM handles and cancels work tied to that document. An explicit
navigation command may establish a new context after checking access to the target.

Workspace paths such as `/pages/counter/index.html` belong to wish's virtual
filesystem. Access to the user's actual files requires an explicit browser file
grant or import/export action. Commands run in browser runtimes or an explicitly
selected server session. A server process has access to its allocated resources;
access to processes on the user's computer would require a separate native adapter.

## Command language

Start with a small shell grammar: commands with quoted arguments, newlines between
commands, pipelines using `|`, output redirection using `>`, and `#` comments.
Command names resolve through a registry that declares argument schemas, input and
output types, capabilities, and effects. Browser objects flow as typed values;
resource handles are never converted into executable source.

Each job exposes output, diagnostics, and an exit status. Pipeline failures stop
dependent steps, and cancellation stops subsequent commands. Completed page or
network effects remain visible even if a later step fails; a pipeline is not an
atomic transaction. History preserves explicit commands so they can be inspected
and run again without an AI request.

Illustrative command vocabulary:

| Commands | Purpose |
| --- | --- |
| `ls [PATH]`, `mkdir PATH`, `cp SOURCE DEST`, `read PATH`, `write PATH` | Create, inspect, and change workspace files |
| `mount list`, `mount local PATH`, `mount remote NAME PATH` | Inspect or attach storage with explicit grants |
| `data import PATH --as NAME`, `data append NAME --from PATH` | Add and update named datasets |
| `notebook create PATH --kernel NAME`, `notebook open PATH`, `notebook run PATH` | Create and execute interactive notebooks |
| `resource list`, `resource acquire TYPE --as NAME`, `resource release NAME` | Discover and lease virtual resources |
| `tab list`, `tab use ID`, `tab open URL` | Inspect, select, or open browser tabs |
| `page run PATH`, `page reload`, `page info` | Launch workspace pages and manage the current page |
| `dom query SELECTOR`, `dom text SELECTOR`, `dom attr SELECTOR NAME` | Read elements, text, or attributes |
| `dom set-text SELECTOR TEXT`, `dom style SELECTOR PROPERTY VALUE` | Change page content or styles |
| `dom click SELECTOR`, `dom fill SELECTOR VALUE` | Interact with page controls |
| `page call NAME [ARGS...]` | Invoke a function exposed by a page bridge |
| `js run PATH [ARGS...]` | Execute a JavaScript command module in the workspace runtime |
| `wasm load PATH --as NAME`, `wasm call NAME.EXPORT [ARGS...]` | Instantiate a module and call a typed export |
| `wasm run PATH [ARGS...]` | Execute a module implementing the wish command interface |
| `fetch URL`, `json parse`, `json stringify` | Fetch and transform data for pipelines |
| `jobs`, `cancel ID`, `history`, `help [COMMAND]` | Manage and inspect shell work |
| `ai configure`, `ai status`, `ai clear-key` | Configure and inspect natural-language support |

DOM reads may return multiple matches. Mutations require one matching element,
unless a supported `--all` flag explicitly requests all matches. Missing or
ambiguous targets produce errors rather than guessed actions.

```text
# Work with files, data, and a browser Python notebook.
ls /data
mkdir /notebooks
data import /data/measurements.csv --as measurements
notebook create /notebooks/analysis.ipynb --kernel python
notebook open /notebooks/analysis.ipynb

# Run a live workspace application and invoke its exposed function.
page run /pages/counter/index.html
page call counter.increment 1
dom text "#counter"
dom set-text "#status" "Ready"

# Run a computation and pass its output into another command.
read /data/input.json | wasm run /modules/transform.wasm | write /data/output.json

# Attach server storage after choosing the configured server and granting access.
mount remote team /remote/team
cp /data/output.json /remote/team/output.json

# Work on an external page through an optional browser adapter.
tab open https://example.org
dom text "h1"
dom style "h1" color red
```

The file examples assume the input files have already been imported into the
workspace. A notebook stays open as an interactive tool while commands add data,
execute cells, or save artifacts.

## Live pages and execution

wish has two page environments:

- **Workspace pages and tools** run user-provided HTML, CSS, JavaScript,
  WebAssembly, and notebook kernels in runtimes separated from the shell's
  credentials and control UI. A bridge exposes selected application functions,
  scoped files and datasets, logs, and results to the shell.
- **Existing browser pages** are addressed through a browser adapter, such as an
  extension with access granted to the relevant tab or site. Reading and changing
  their DOM uses installed command handlers. Calling application functions requires
  an explicit page bridge or an adapter that supports the page's execution context.

A normal web application cannot control arbitrary external tabs or inspect
cross-origin documents. The adapter must respect those boundaries. For example,
Chrome script injection requires the `scripting` permission and host access or
temporary `activeTab` access. See [MDN's same-origin policy documentation](https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Same-origin_policy)
and [Chrome's scripting API](https://developer.chrome.com/docs/extensions/reference/api/scripting).

An extension's isolated script environment can share a page's DOM while having
separate JavaScript variables. Consequently, DOM access does not imply access to
every function or Wasm instance inside an application. See [Chrome's content script
execution model](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts).

Page edits affect the running document. Its application may overwrite them during
rendering, and navigation usually discards them. Persisting a change requires
editing workspace source files or saving an explicit transformation to reapply.
Loading a page executes its normal application code; changing text or styles does
not implicitly execute new scripts.

## WebAssembly

WebAssembly participates in two ways: a live page can run its own Wasm application,
and the shell can run Wasm modules as computations or pipeline commands.

`wasm load` creates a named instance with declared imports. `wasm call` invokes an
export using its declared value types. For example, a module exporting a numeric
`add` function could be used as follows:

```text
wasm load /modules/math.wasm --as math
wasm call math.add 2 3
```

`wasm run` requires a command interface defining argument encoding, input/output
streams, diagnostics, exit status, and any workspace imports. An arbitrary `.wasm`
file is not automatically a shell executable. String and JSON conversion require
an explicit memory/encoding interface or wrapper; they cannot be inferred from a
numeric export signature. A WASI-compatible interface could be a later adapter.

Wasm reaches browser functionality through host imports, including JavaScript
wrappers for DOM operations. Those wrappers enforce the same capabilities as
ordinary commands. Existing page instances must expose a bridge before the shell
can call them. See [MDN's WebAssembly concepts](https://developer.mozilla.org/en-US/docs/WebAssembly/Guides/Concepts).

Run shell computations in dedicated workers so they can be terminated without
blocking the prompt. Workers communicate with the page adapter for DOM work; they
cannot directly manipulate the DOM. Use deadlines, bounded outputs, and supported
Wasm memory limits; browser workers do not provide hard physical CPU reservations
or general JavaScript memory quotas. See [MDN's worker execution model](https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API/Using_web_workers).

Executable loading depends on the host. A Chrome Web Store extension must account
for Manifest V3 restrictions on remotely hosted JavaScript and Wasm. Its command
handlers should be bundled, with user programs assigned to a separately designed
workspace runtime. A worker alone does not remove extension restrictions. See
[Chrome's remotely hosted code guidance](https://developer.chrome.com/docs/extensions/develop/migrate/remote-hosted-code).

## Natural-language commands with `//`

When `//` is the first non-whitespace token on a top-level input line, the rest of
that line is a natural-language request. It is not a comment. `#` remains the shell
comment marker. URLs, quoted strings, and JavaScript module contents containing
`//` do not trigger translation. Each request is translated when execution reaches
its line, so it sees the context established by preceding commands.

```text
// create a Python notebook named analysis
```

The generated command could be:

```text
notebook create /notebooks/analysis.ipynb --kernel python
```

The shell displays that command and executes it automatically within the user's
configured permissions. The natural-language input, generated program, and result
are associated in history. The generated program remains editable and reusable.
An optional preview mode can stop before execution, but normal `//` operation
includes execution.

Translation follows a bounded sequence:

1. Capture the request, workspace and resource revisions, selected document identity
   when relevant, available command schemas, and context that the user has allowed
   wish to send to the AI provider.
2. Call the configured provider using the user's key. Request a structured response
   containing a finite wish program, for example
   `{"commands":"notebook create /notebooks/analysis.ipynb --kernel python"}`.
3. Parse the generated source with the same parser used for explicit commands.
   Validate command names, arguments, pipeline types, target scope, and step limits
   before starting execution. AI output is input to the shell language, never
   JavaScript evaluated inside the privileged shell.
4. Display the program, check that its workspace and document context is still valid,
   and run it through the ordinary command executor. Recheck permissions and
   targets before each step. Requests outside existing grants pause for the
   required grant.
5. Report actual output or the failing step. Do not silently retry actions that
   might already have taken effect.

Page text is task data, including any instructions embedded in it. It cannot grant
capabilities, change the configured provider, or initiate AI calls. Ambiguous
requests return a clarification without executing a guessed program. Translation
failures, an absent key, exhausted request budgets, or invalid output execute no
generated commands. Explicit commands remain available without AI.

## The user's AI API key

`ai configure` opens a trusted settings flow for the provider endpoint, model, and
API key. The key is entered in a secret field rather than as a shell argument, so
it does not enter command history. Provider adapters handle authentication and
response formats; the shell language stays independent of a particular provider.

Keep the key in session memory by default. Any persistent credential storage is an
explicit option whose protection depends on the chosen host. Keys are excluded
from page bridges, command environments, workspace files, exports, logs, and model
prompts; only the AI transport uses them for authentication. `ai clear-key` removes
the configured credential.

Only the user's request and permitted context are sent to the selected provider.
Workspace files, datasets, notebook cells, and page content are opt-in and limited
to relevant excerpts, with password fields, tokens, and other secrets excluded.
Show the configured provider/model and expose
request limits and usage when available. The user's provider account pays for the
requests; wish does not require a shared application key.

The transport must fit the host and provider. A web app needs an endpoint that
supports browser requests; an extension can make requests from its privileged
context with the relevant host permissions. If a provider does not support that
client setup, use a user-controlled local broker. Never pass credentials through
the target page. See [Chrome's cross-origin request documentation](https://developer.chrome.com/docs/extensions/develop/concepts/network-requests).

## Runtime structure

```mermaid
flowchart TD
    Explicit[Explicit commands] --> Parser[Shell parser and validator]
    Natural[// natural-language request] --> AI[AI translator using the user's key]
    AI --> Parser
    Registry[Command schemas and grants] --> AI
    Registry --> Parser
    Parser --> Runner[Command and pipeline executor]
    Runner --> Browser[Optional tab and page adapter]
    Runner --> Workspace[Mounted files, data, and live tools]
    Runner --> Workers[Python, JavaScript, and Wasm runtimes]
    Runner --> Remote[Optional server storage and compute]
    Browser --> Results[Output, diagnostics, and history]
    Workspace --> Results
    Workers --> Results
    Remote --> Results
```

The core owns parsing, command registration, typed pipelines, context, jobs, and
history. Host adapters own browser access, workspace isolation, executable loading,
and credential transport. Pages and modules receive scoped interfaces for their
task rather than the shell's AI credentials or full browser privileges. Manual and
AI-generated commands share the same executor and permission checks.

## First implementation scope

Begin with a standalone workspace shell that creates and edits files in memory,
persists them in browser storage, opens a live tool, runs a Python notebook, and
pipes a Wasm command's output into a workspace file. Give all these interfaces the
same named resources and scoped access model. Add authenticated server storage and
leased compute as explicit adapters, followed by an optional extension for existing
tabs.

The first AI path should configure one provider adapter with the user's key and
turn `// create a Python notebook named analysis` into a displayed, validated,
executed command. Demonstrate equivalent behavior for the explicit command,
continued tool execution, failure on stale resources or ambiguous targets, and
normal manual operation without a key. Broader shell syntax, additional providers,
persistent page transformations, and
WASI support can follow once these core interactions work.
