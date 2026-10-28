# wish security profile and browser obstacles

Status: proposed requirements; phase scope updated 2026-10-14. This describes
the intended boundaries and their limits; it is not a security assessment of an
implemented system.
Read alongside [IDEA.md](IDEA.md) and [ARCH.md](ARCH.md).

## Applying this profile to the two phases

[ROAD.md](ROAD.md) starts with files, byte/text shell commands, one notebook,
and optional AI translation. Phase 1 needs checked file paths and operations,
runtime/credential isolation, safe output, bounded execution, version conflicts,
and separate AI-disclosure permission. Implement these with a small checked file
adapter and notebook bridge; no public resource-handle protocol is required.

Evaluate just-bash as an interpreter dependency. Its callbacks and filesystem
interfaces must reach only the authority intentionally supplied by wish. Run the
interpreter outside the trusted credential/UI origin and test its actual isolation
and cancellation behavior; library limits do not replace the application's checks.

Server, external-tab, and hardware sections apply when those phase 2 adapters are
selected. Their authorization, isolation, and lifecycle requirements remain intact.
The original generalized grant/lease model below describes the broader profile;
phase 1 implements only the subjects, paths, operations, and limits it actually
exposes. Security tests repeat for every released profile and candidate.

wish executes user code and AI-generated commands against files, datasets, live
tools, and optional server resources. Its central security requirement is that
**execution authority follows explicit resource grants**, regardless of whether a
request came from a typed command, notebook cell, tool, or AI translation.

## Assets, trust, and threat model

Protect the user's AI key, authentication sessions, workspace contents and history,
physical file grants, remote allocations, and other users' resources. Treat
imported HTML, notebook outputs, packages, generated programs, and tool descriptions
as potentially hostile. A user's own notebook can include code they did not audit.

| Component | Trust and permitted authority |
| --- | --- |
| Shell application, broker, and AI transport | Trusted to enforce grants and handle credentials |
| Notebook kernels, user modules, and tool pages | Receive scoped data and operations; no shell credentials |
| AI provider | Receives approved context; returns untrusted program input |
| Server gateway and storage operator | Trusted with server plaintext under the default server model |
| Remote kernel | Untrusted execution with a bounded allocation and controlled egress |
| Optional extension/native adapter | Adds access to browser tabs or host facilities within its grants |

The profile addresses malicious imported content, compromised dependencies,
prompt injection, confused resource routing, unauthorized server requests, and
resource exhaustion. A compromised trusted shell can misuse keys and grants;
browser isolation does not eliminate that risk. Protection against a compromised
browser, operating system, or browser extension is outside this application model.

## Resource access and data disclosure

Grants name the subject, workspace, resource or path subtree, operations, bounds,
and expiry. Separate read, write, execute, network, AI disclosure, device, and
remote-allocation rights. Discovering a resource name does not authorize access.
Validate the caller from its bound channel or authenticated server session, not
from an identity field supplied in a request.

The broker normalizes paths, confines them to mounts, checks revisions and leases,
and limits payloads. Imported symlinks, archive entries, encodings, and remote
object keys must not escape the selected subtree. Never expose a server filesystem
path or credential merely because a client names a matching resource.

Default automatic execution is useful within existing grants. Creating a notebook
or editing an owned workspace file need not ask for permission on every command.
Acquiring a new physical directory, sending additional data to a provider, or
starting paid remote compute requires the relevant grant. Runtime operations remain
checked even after the shell command has been approved.

Reading sensitive data and sending network traffic are a dangerous combination.
Capability checks alone do not prevent a program from transmitting data it was
allowed to read. Prefer an offline compute profile with brokered inputs/outputs;
otherwise disclose and constrain the allowed destinations. Sanitization and secret
redaction help, but cannot establish that arbitrary notebook text contains no secrets.

## Browser restrictions and their architectural consequences

| Restriction | Consequence for wish | Design response |
| --- | --- | --- |
| Same-origin policy | An ordinary web app cannot inspect arbitrary tabs or other origins' DOM/storage | Keep work inside the workspace; use explicit bridges or an optional extension |
| CORS | Some providers and resource servers do not permit browser response access | Configure compatible endpoints or an authenticated broker; preserve server authorization |
| Origin-scoped storage | Untrusted code on the shell origin could access its IndexedDB/OPFS | Execute code on a separate runtime site; broker the shell's workspace storage |
| File picker permissions | Workspace paths do not imply physical disk access | Request selected files/directories through a real user gesture, with import/export fallback |
| Quotas and eviction | Browser storage can become full, disappear, or be cleared | Track quotas and save state; request persistence where available; offer exports/backups |
| CSP | Wasm compilation, dynamic modules, workers, and network access may be blocked | Define distinct shell/runtime policies and pin allowed assets |
| Frame sandboxing | Stronger isolation removes direct DOM, storage, or worker assumptions | Use separate origins and a checked message bridge; test the required sandbox flags |
| Cross-origin isolation | Shared memory needs isolation headers and compatible embedding | Make synchronous bridges/threads optional; provide a copy/message-based baseline |
| Worker lifecycle | Workers can be terminated; tabs can suspend or close | Keep artifacts durable, bound jobs, and let server leases expire independently |
| Device API availability | Hardware and GPU interfaces vary across browsers and contexts | Feature-detect, require browser grants, and offer simulated or remote adapters |
| Extension policies | Store-distributed extensions restrict executable loading | Bundle extension handlers and treat workspace execution as a separate design |

The same-origin and CORS rules restrict cross-origin access; they are not a substitute
for authenticating a server request. Some cross-origin requests can still be sent
even when their response is unreadable. See [MDN's same-origin policy](https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Same-origin_policy).

Directory pickers require transient user activation and have limited availability.
An AI round trip cannot manufacture that gesture. Pause at a trusted "Choose
directory" control and resume after selection. See [MDN's directory picker](https://developer.mozilla.org/en-US/docs/Web/API/Window/showDirectoryPicker).
Browser-managed persistence remains subject to user deletion; see
[storage quotas and eviction](https://developer.mozilla.org/en-US/docs/Web/API/Storage_API/Storage_quotas_and_eviction_criteria).

## Executable code, origins, and output rendering

Serve the shell and authentication UI separately from user-authored executable
content. Prefer a different registrable domain for the runtime site as well as
separate runtime origins where workspaces require isolation. A different URL path
does not create a browser security boundary. The runtime site gets no ambient
application cookies; physical file handles and privileged storage APIs stay in
the trusted broker.

A same-origin iframe with both `allow-scripts` and `allow-same-origin` is unsuitable
as an untrusted-code boundary: its document can remove the sandbox. A cross-origin
runtime can retain its own origin for worker/module support without sharing the
shell's origin. An opaque sandbox origin is another option, but loses capabilities
that some notebook runtimes need. See [MDN's iframe sandbox documentation](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe).

Check `event.origin` and `event.source` during bridge bootstrap, transfer a dedicated
port to the expected frame, and bind that port to server-side/broker-maintained
grants. Reset bindings on navigation or runtime destruction. Reject unexpected
messages, invalid schemas, oversized payloads, and replayed mutation requests.
Opaque-origin frames require a deliberately designed source/port bootstrap rather
than trusting an origin string of `null`. See [MDN's postMessage guidance](https://developer.mozilla.org/en-US/docs/Web/API/Window/postMessage).

The shell must not evaluate AI output or user files as privileged JavaScript.
Notebook cells execute in their kernels; JavaScript modules execute in their runtime.
Render terminal text as text. Sanitize notebook HTML and SVG; executable widgets
belong in isolated frames. Never serve uploaded HTML as an application-origin page,
and never turn a saved notebook output into trusted UI simply because it came from
a previous run.

CSP should restrict the trusted application's scripts, workers, framing, and
connections. Enable Wasm compilation only in runtimes that require it:
`'wasm-unsafe-eval'` permits Wasm compilation without permitting general JavaScript
string evaluation. Additional permissions for authored JavaScript belong solely
to the isolated runtime. See [MDN's CSP reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy).

Network restrictions need special care. `connect-src` covers connections such as
fetch and WebSocket, but does not by itself block image/script loads, forms, or
frame navigation. A live HTML tool can navigate its own frame. Package loaders can
also create permitted request channels. Do not claim complete egress prevention
for arbitrary HTML/JavaScript based only on a CSP connection allowlist. Sensitive
offline computations need a narrower worker profile, curated rendering, carefully
restricted loading, and tests of every remaining channel. Runtime CSP must come
from a trusted response/launcher, not user-editable workspace content.

Cross-origin isolation for shared memory commonly uses
`Cross-Origin-Opener-Policy: same-origin` plus
`Cross-Origin-Embedder-Policy: require-corp` or `credentialless`. It affects external
assets, embedded tools, and popup relationships, and does not make same-origin code
untrusted. Feature-detect `crossOriginIsolated`; keep a non-shared-memory baseline.
See [MDN's crossOriginIsolated documentation](https://developer.mozilla.org/en-US/docs/Web/API/Window/crossOriginIsolated).

## AI credentials and generated programs

Keep the AI key in the trusted transport's session memory by default. A password
field hides display; it does not protect a key from compromised application code.
Persistent storage needs an explicit choice. Encrypting a stored key while saving
its decryption key beside it does not protect against same-origin compromise.
A local broker or server credential service changes who must be trusted with it.

Pin the chosen provider endpoint in trusted configuration. Model output cannot
redirect credential-bearing requests, change the provider, initiate another AI
request, or widen context disclosure. Avoid secrets in URL parameters, error
messages, telemetry, crash reports, notebook outputs, and audit records. Clearing
the key removes application-held credentials but does not undo earlier disclosure.

Treat instructions inside files, datasets, pages, and tool descriptions as data.
Use bounded context and structured command responses, then validate the complete
program before starting it. Reject unknown commands, invalid targets, nested `//`
requests in generated programs, and attempts to alter authorization. Permission
enforcement and destination controls provide the boundary; a prompt telling the
model to ignore malicious instructions is insufficient.

Syntax checks cannot prove that generated Python or JavaScript has only the
advertised effects. Enforce file/network/resource limits where execution actually
occurs. Stop on ambiguity, expired leases, stale revisions, or invalid plans.
Track provider/request budgets. Do not retry a mutation until its receipt or
resource state establishes whether it already completed.

## Server storage and compute

The server authenticates every operation and independently authorizes tenant,
workspace, resource, and action. Client-side grants are useful policy inputs,
not server authority. Use HTTPS, scoped identities, revision-conditional writes,
bounded uploads, resource quotas, and mutation receipts. Serve user content with
appropriate MIME types and a separate content origin or download disposition.
Encrypt stored artifacts and backups with server-managed keys, restrict operator
access, and define retention, deletion, and audit-log access. These controls protect
the service's storage; they do not remove the operator from the trust model.

Cookie-authenticated APIs need CSRF protection and origin checks. Use host-only,
Secure, HttpOnly session cookies where applicable; sibling subdomains are not a
complete SameSite boundary. Authenticate WebSocket setup as well as HTTP requests.
See [OWASP's CSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

A provider/network proxy needs destination controls, redirect validation, and
protection against private-address and metadata-service requests. A generic
"fetch any URL" backend can expose internal networks even though the browser could
not access them. See [OWASP's SSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html).

Remote user code requires an isolation design separate from the web application's
browser sandbox. Use per-session containers or stronger VM boundaries according
to the trust model, restricted privileges and mounts, controlled network egress,
hard execution quotas, and leases. No host control sockets or inherited cloud
administrator credentials reach a kernel. Closing a browser tab must not leave
unbounded compute running or allow an expired lease to be reused.

The default server profile trusts the operator with plaintext and relies on
transport security, server authorization, and protected storage. End-to-end
encrypted storage would be a separate profile: arbitrary server-side analysis
cannot operate on ciphertext without additional mechanisms or disclosure. Likewise,
calling an external AI provider explicitly discloses the selected context to it.

## Verification and unresolved obstacles

Before treating the architecture as secure, demonstrate that:

- A hostile tool/kernel cannot read the AI key, shell storage, another workspace,
  or a file outside its granted mount; forged bridge identities are rejected.
- Malicious notebook outputs and imported pages cannot execute in the trusted UI.
- The server denies another user's resource ID even when a client bypasses the UI.
- Generated programs cannot change provider destinations or obtain new grants,
  and approved file reads do not silently become AI context disclosure.
- Stale writes conflict, quota failures preserve recoverable artifacts, cancelled
  jobs stop further operations, and uncertain mutations are not repeated.
- Runtime egress tests include fetch, WebSocket, imports, images, forms, navigation,
  and package loading; server leases expire after disconnects.

Remaining constraints include incomplete physical-file/device support across
browsers, limited control of local CPU and JavaScript memory consumption, expensive
or incompatible Python packages, and the tension between shared-memory performance
and flexible embedding. Browser persistence and arbitrary live HTML also cannot
provide the same durability and confinement guarantees as a carefully operated
server runtime. Each supported execution profile needs an explicit tested contract.
