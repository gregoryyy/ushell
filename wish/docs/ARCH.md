# wish architecture

Status: proposed phase 1 architecture, 2026-10-14; runtime features are not
implemented. [PHASES.md](PHASES.md) defines the lean starting scope and conditional
phase 2 ideas. Product behavior is in [IDEA.md](IDEA.md), security boundaries in
[SEC.md](SEC.md), and component research in [SOTA.md](SOTA.md).

Phase 1 is a browser shell over workspace files with one Python notebook and
optional natural-language translation. Reuse just-bash if its integration spike
passes. Compose through byte/text pipes and JSON/CSV files. TypeScript interfaces
and internal job/kernel IDs help implement the system; they do not create typed
shell streams or a public resource-handle language.

## How a phase 1 command runs

![Proposed lean phase 1 flow: optional AI translation, shared shell execution, checked files, a Python notebook, and saved results. Server adapters are phase 2.](images/wish-execution.svg)

1. **Input:** the user submits a shell program or a standalone `//` request.
2. **Optional translation:** the trusted AI transport uses the user's key and only
   approved context to return a shell program. Ordinary commands work without AI.
3. **Validation:** check the supported syntax, permitted commands/arguments, input
   bounds, and selected workspace before execution. Both input paths use this policy.
4. **Execution:** just-bash runs in an isolated worker. A vetted notebook command
   invokes the isolated Pyodide worker; notebook source never executes in the app.
5. **File access:** a checked adapter validates caller identity, path scope,
   read/write permissions, file versions, and size limits. Runtime workers receive
   scoped file snapshots or checked messages, never the AI key or physical file grants.
6. **Storage:** keep bounded session files in memory; one browser storage adapter
   saves files/metadata. Import/export is available even without persistent storage.
7. **Results:** commit successful authorized changes against expected versions and
   update the prompt, file editor, and notebook view. Record output, errors, and
   command history without credentials. Report conflicts and cancelled/failed jobs.

Server storage/compute and additional page/module adapters are phase 2 extensions.
The diagram's server box is outside the phase 1 acceptance path.

## Components and deployment

Use a small TypeScript application, a shell worker adapter, a notebook adapter,
checked file operations, and an AI transport. UI views share the same saved files
and notifications. Avoid a general resource scheduler or plugin framework.

The trusted app owns permissions, saved files/metadata, history, and AI settings.
The execution launcher is served at a separate runtime site, with no app session
cookies or AI key. It hosts the shell/Python workers and any executable notebook
widgets. Origin separation protects the trusted app; workers provide responsiveness
and termination. A worker alone is not the credential/storage boundary.

Bootstrap a dedicated message port after checking the expected frame and origin.
Bind its caller/workspace server-side in the trusted app, validate messages, and
invalidate the binding on reload or disposal. Start with one active workspace and
simple permitted file subtrees/operations; add more elaborate identity/grant models
only when supported workloads need them.

Pin runtime assets and limit destinations required to load them. Keep arbitrary
runtime network access disabled in the initial execution profile. Test the actual
remaining egress channels as described in SEC. SharedArrayBuffer, threading,
service-worker filesystem bridges, and per-workspace origin orchestration are not
prerequisites for the first file-snapshot workflow.

## Shell, commands, and state

just-bash is a candidate dependency, not a security policy or a committed runtime
choice before the spike. Define the tested syntax subset for quoting, arguments,
newlines, pipes, and redirection; register a few commands such as
`notebook create/open/run`, `jobs`, `cancel`, and `ai configure/status/clear-key`.
Validate unsupported constructs and command arguments before starting execution.
Keep policy checks on file/notebook operations at runtime as well.

Use the engine's parser and execution machinery rather than building a second Bash
implementation. Use its parsed representation for validation where the selected
version permits. Do not evaluate generated source as privileged JavaScript, and do
not assume parsing proves the effects of notebook code.

Pipes carry bytes/text; JSON and CSV are explicit formats. Binary input/output,
pipeline failures, output bounds, and redirection must pass the integration tests.
A failed pipeline must report failure and prevent dependent work. Confirm the
engine's pipeline-failure behavior rather than inheriting an unchecked default.

The application records the selected workspace/directory, job ID, source program,
results, and limits. Verify cwd/environment behavior between prompt submissions in
the spike: an engine API call is not automatically a persistent interactive shell
session. Saved files survive kernel/shell restarts; interpreter variables may not.

Jobs initially run through a small queue with output limits, deadlines, and explicit
cancellation. Terminate unresponsive workers, reject late results, and invalidate
their authority. Already completed external effects are not rolled back by a later
failure; never retry an uncertain mutation without checking its outcome.

## Files and persistence

Start with the engine's bounded memory filesystem behind checked adapters. File
paths such as `/data/measurements.csv` and `/notebooks/analysis.ipynb` are workspace
paths, not physical disk authority. Import selected files using browser controls;
export a workspace without credentials or transient runtime state.

Choose one persistence backend. IndexedDB is the initial candidate for small file
blobs and revision metadata committed together. Large-file OPFS access and physical
directory mounts are later options, not mandatory parallel implementations. Show
memory/persistent mode, dirty state, storage failures, and export recovery.

Use expected file versions for commits. Serialize writes or return a conflict
when another writer changed an input/output. Stage job output and publish permitted
changes only after success; cancelled/failed staged writes leave the old files.
Do not describe a complete shell program as an atomic transaction. Test interruption,
quota failures, and restore; browser storage remains subject to clearing/eviction.

Data begins as JSON/CSV files. File versions and change notifications coordinate
shell, editor, and notebook views. Named dataset services, schemas on every shell
value, SQL engines, and a general subscription bus can wait for a demonstrated need.

## Python notebook adapter

Use Pyodide in an isolated worker and one existing notebook UI if its file/kernel
integration fits. Implement start, execute, interrupt, restart, and dispose through
that adapter. Keep `.ipynb` source, cell IDs, saved outputs, and enough execution
metadata to understand a run. A kernel restart loses interpreter variables, not
saved notebook source.

For the first synchronous Python file interface, copy the run's granted inputs into
its filesystem and record their versions. After success, validate and stage changed
files/deletions for the trusted adapter to commit. Reject changes outside the grant
or against stale versions. This avoids introducing a shared-memory filesystem
bridge for the initial small CSV workload.

Render text and supported rich output safely; isolate executable widgets. The
notebook's plot/output view is the first interactive tool. An edit can be observed
through file-change notifications and an explicit notebook rerun. Automatic reactive
execution is deferred until its side effects and permissions are defined.

## Natural language and credentials

Recognize `//` only at the start of a standalone prompt submission, outside quoted
shell text. Send approved context and available command descriptions to one
configured provider. Return a bounded structured response containing shell source;
validate, display, and run it using the ordinary command path. Mixed `//` lines in
larger Bash scripts and new user-language syntax are deferred.

Keep the user's key in trusted session memory. Runtime commands, files, history,
exports, prompts, and notebook outputs do not receive it. Browser-compatible
provider transport or an explicitly configured user-controlled broker is required.
AI context disclosure is a separate permission from file access. Generated programs
cannot change the provider, widen grants, or trigger nested AI calls.

Record the request, displayed program, and actual outcome. Replay uses the program
without another model call. An absent key, invalid response, stale target, exhausted
budget, or ambiguity executes no generated command. Preview is optional; normal
execution proceeds automatically within existing permissions.

## Phase 2 extensions and deferred full-concept ideas

Add adapters individually for live pages, JavaScript/Wasm commands, remote files,
leased kernels, databases, search/recall, external tabs, and selected devices. Use
filenames, byte/text outputs, JSON, and explicit commands as the default interface.
Storage and compute remain independently selected; no automatic server fallback.

Remote storage needs authentication, server-side authorization, conditional writes,
quotas, and receipts. Remote kernels need isolation, bounded leases, controlled
egress, cancellation, and reconnection. A Jupyter-compatible adapter is a candidate;
[Jupyter Server APIs](https://jupyter-server.readthedocs.io/en/latest/developers/rest-api.html)
already cover contents, sessions, and kernels.

A Wasm command adapter must document its argument/byte/file interface; an arbitrary
module is not automatically a shell executable. Component/WASI support can be a
later adapter. External tab access and physical devices require the relevant browser
permissions and tested lifecycle handling.

The original full concept proposed typed record/object pipelines and a uniform
resource-handle API across files, datasets, tools, devices, and compute. These are
optional research directions. Internal types and IDs remain useful, but a new
public protocol needs an acceptance example and migration plan before adoption.
[PHASES.md](PHASES.md) and TODO 8.5 record that decision gate.

## Acceptance

Run the local CSV → shell transform → notebook plot → save/reload workflow, then
verify equivalent `//` execution and replay. Test denial, hostile output, secret
isolation, cancellation, conflict, interruption, and export recovery. Record browser
support and startup/memory measurements. Every selected phase 2 feature extends
these tests; security tests repeat before every release.
