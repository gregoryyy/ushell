# wish: phase 1 and phase 2

Decision: 2026-10-14. Status: implementation plan; neither runtime phase is
implemented. [TODO.md](../TODO.md) tracks the work, [DONE.md](../DONE.md) records
completed work, and [ARCH.md](ARCH.md) defines the current architecture.

Start with a useful browser shell built around files. Add broader integrations
after the first workflow works. Types and object IDs remain normal implementation
details; a new typed shell language and public resource-handle system are deferred
until a concrete use case justifies them.

## Phase 1: a lean browser workspace

Deliver one local workflow: import a CSV, transform it with shell commands, create
and run a Python notebook, display its plot, and save/reload the files. The user can
also request the same operations with `//`, inspect the generated program, and
rerun it without another model call.

### Shell and files

Evaluate **just-bash as the first shell candidate**, using the integration criteria
in [SOTA.md](SOTA.md). Reuse its shell machinery if the browser/file-adapter spike
passes. Do not build a competing Bash implementation in parallel. If it fails,
record the concrete obstacle and choose the smallest alternative for this workflow.

Use ordinary commands, byte/text pipelines, and files. JSON and CSV carry structured
data. Notebook commands take filenames; jobs have simple internal IDs. No public
typed-record streams, dataset-handle protocol, or generic `resource acquire` API
is required.

Keep the first supported grammar small even if the underlying interpreter supports
more. Define the permitted built-ins, notebook commands, quoting, pipes, redirection,
and failure behavior. Reject unsupported constructs before execution. Manual and
AI-generated programs follow the same supported syntax and access policy.

Start with bounded in-memory files and import/export. Add **one** browser persistence
backend; IndexedDB is the initial candidate for small files and their revisions.
OPFS and selected-directory mounts can follow when file sizes or workflows need
them. Show storage mode and unsaved state, and retain export as a recovery path.

### Notebook and natural language

Use a Pyodide kernel adapter and one existing notebook UI if it fits. The initial
live tool is the notebook's interactive plot/output view. Copy granted inputs into
the kernel and commit authorized output files after a successful run. Shell and
notebook use the same workspace files and detect conflicting writes.

Add one AI provider adapter and session-memory BYOK settings. A standalone prompt
submission beginning with `//` requests translation; normal quoted text, URLs,
and file contents do not. Display the generated shell program, validate it, and
automatically execute within existing permissions. Mixed natural-language lines
inside a larger Bash script are deferred. Save request/program/result history,
excluding secrets, and allow replay of the program without AI.

### Small adapters, real boundaries

A checked file adapter owns path scope, read/write permissions, versions, and
size limits. A narrow bridge exposes notebook operations. These can be ordinary
interfaces and functions; they do not need a general resource graph, plugin
marketplace, or placement scheduler.

Keep interpreter/kernel execution isolated from the trusted UI and AI transport.
Use checked messages, safe output rendering, pinned runtime assets, bounded jobs,
and cancellation. An interpreter's own limits supplement these boundaries.
Runtime file access and permission to disclose content to AI remain separate.
[SEC.md](SEC.md) applies to every implemented capability.

### Acceptance and phase boundary

Phase 1 passes when automated tests cover the connected local CSV/notebook workflow,
save/reload and export recovery, explicit-command use without a key, equivalent
`//` execution and replay, cancellation, failed writes, conflicting versions,
denied file access, hostile outputs, and attempted key/context leakage.

Apply the feature-test and recurring release-security rules in
[CLAUDE.md](../../CLAUDE.md). Runtime milestones remain unchecked until implemented
and tested. Phase 1 does not require a server, extension, general Wasm ABI, SQL
engine, external-tab access, or device adapter to be released.

## Phase 2: extend demonstrated workflows

Phase 2 is a menu of extensions, not a requirement to implement the entire original
concept. Select an extension with a concrete use case, update TODO, and add its
behavioral/security tests before shipping it.

| Extension | Initial approach | Evidence needed before expanding it |
| --- | --- | --- |
| Live workspace pages | Isolated HTML/JS tools with a small declared command bridge and file-change events | A second useful tool needs interaction beyond notebook outputs |
| JavaScript and Wasm commands | Explicit command adapters with byte/file inputs and outputs; document the selected Wasm interface | A transformation cannot be served adequately by existing shell/Python tools |
| Remote storage | An explicitly attached authenticated file adapter with conditional writes | A workflow needs sharing, backup, or larger durable storage |
| Remote compute | An explicit leased kernel command, independently of storage | Required packages or compute exceed the supported browser profile |
| Databases and datasets | Add a selected existing SQL engine or file-backed data commands | Query/update workloads justify more than JSON/CSV files |
| Search and recall | Local text/metadata search of workspace documents and command history first; evaluate semantic retrieval later | Search evaluation shows a benefit; captured page/history content has explicit scope, provenance, deletion, and AI-disclosure rules |
| External browser pages | Optional extension or external automation adapter with minimal permissions | A workflow genuinely needs tabs outside wish; visit history alone supplies no saved page body |
| Physical resources | Simulated devices followed by specific granted hardware/GPU adapters | A supported device workflow exists and browser limitations are tested |
| Larger-file/compatibility support | Evaluate OPFS, Linux/Node runtimes, WebMCP, or Wasm components independently | Measured file/runtime limits or interoperability requirements justify the cost and license obligations |

### Relationship to the original full concept

The original ambition was a uniform model for files, datasets, notebook cells,
pages, devices, and local/server compute. Phase 2 can move toward that where shared
interfaces reduce repeated integration code. It does not automatically introduce
typed pipelines, a custom shell language, public resource handles, or transparent
placement between local and remote compute.

Those abstractions need an acceptance example that ordinary files, JSON, names,
and explicit commands cannot express reasonably. Record their costs and migration
path before adopting them. Internal TypeScript types, kernel/job IDs, and required
remote lease IDs do not imply a new user-facing execution model.

Server-enabled features retain authentication, independent authorization, tenant
isolation, quotas, cleanup, and reconnect receipts. Simpler commands do not reduce
the security requirements of the resources they access.

## Implementation order and process

Phase 1 uses TODO groups 1–4 and 6, then the local acceptance/release checks in group
9. Phase 2 uses groups 5, 7, and selected items in group 8, with additional acceptance
in 9.5. Milestone numbers are stable task IDs, not phase numbers.

All implementation happens on feature branches, includes tests, and appends DONE
entries. Merges happen through human-reviewed GitHub PRs; AI never merges.
