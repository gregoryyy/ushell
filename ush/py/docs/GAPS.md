# Gaps

Open questions and explicit assumptions for ush. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted because they seemed clear enough
not to ask; check them and correct any that are wrong. The rules for this file are in
[SDLC](../../../docs/SDLC.md#work-items). Status: 2026-10-29 01:29.

- Answers carry date, time and author: `**Answer** (2026-10-27 15:10, Gregor Heinrich): …`. Short
  answers are fine. A changed answer goes on top; earlier ones stay below it, struck through.
- Once answered, the decision moves into the document it affects, and the entry gets a
  `**Resolved**` line saying where. Resolved entries stay as a record.
- A question that blocks a TODO item is marked **blocking**.

## Questions

### Q1. Which licence? — blocking for TODO 3.1

**Answer** (2026-10-28 21:20, Gregor Heinrich): MIT.

**Resolved:** [LICENSE](../../../LICENSE) at the repository root.

The repository has no licence file, so nobody else may use, change or redistribute ush. A release
needs one. ush's dependencies (the Anthropic SDK and its tree) are MIT, Apache-2.0, BSD-3-Clause
and PSF-2.0, so any of the common choices is compatible.

- **Proposed:** Apache-2.0 for the whole repository (permissive, with a patent grant). MIT if you
  prefer the shortest licence; GPL-3.0-or-later if derived tools must stay open.

### Q2. How is ush distributed? — blocking for TODO 3.1

**Answer** (2026-10-28 21:20, Gregor Heinrich): asked to choose between a GitHub release, a GitHub
release with a Homebrew tap, and a tag only: "try all three options". 0.2.0-rc1 ships the Go
binaries at parity (asked separately: "Go binaries at parity").

**Resolved:** release plan in ushgo TODO 2.5–2.7: a tag, a GitHub release with the archives and
`SHA256SUMS`, and a Homebrew tap formula (the tap repository is created by the user).

`install.sh` copies ush into a venv today, per user or root-owned. Other channels need their own
packaging and maintenance.

- **Proposed:** `install.sh` from a tagged release for 0.1; a Homebrew tap later, once ushgo
  ([ROAD](ROAD.md) phase 3) shows whether the Python or the Go implementation is the one to ship.

### Q3. Which platforms does a release support?

The suite runs on macOS locally; the Linux matrix entries in CI don't run while GitHub Actions
is unavailable ([tools/ci/README.md](../../../tools/ci/README.md)).

- **Proposed:** macOS 14 or newer and Ubuntu 22.04 or newer, Python 3.10–3.13, zsh 5.8 or newer.
  A release needs one recorded run on Linux, by hand if Actions is still unavailable.

### Q4. Should ush suggest or enable a checker by default?

The checker (TODO 2.13) is off unless configured (A6). With a local model it costs nothing but a
second or so per command; with Anthropic it costs a small request per command.

- **Proposed:** keep it off by default; `ush ai configure` offers it, and the README recommends a
  local model of 3B or more once the corpus (TODO 2.11) shows one that misses no attack in it.

### Q5. Should ush have a blacklist mode?

Asked by the user on 2026-10-28 together with the allowlist (TODO 2.15). A blacklist mode would run
everything except listed commands without approval.

- **Proposed:** no. It fails open on every command nobody listed, and an injected instruction needs
  only one ([ARCH](ARCH.md#the-users-allowlist)). Instead, if wanted: deny rules that only add
  friction, `ask` (always ask, even when the table would run it) and `never` (refuse even
  when approved), for commands a user wants extra protection from (`rm -rf`, `diskutil erase`).

### Q6. Should ush run commands, or propose them for the user to run?

Raised by the user on 2026-10-28: "the idea of the ushell is to propose specific shell commands and
prewrite them to be executed by the user. right now the system performs the actions directly and
prints the result without showing what was executed". Asked as three questions.

**Answer** (2026-10-28 18:40, Gregor Heinrich):
- **Investigation:** "Ask each time". ush runs nothing to investigate either: it proposes
  investigation commands too, the user runs them, and their output becomes context for the next
  proposal.
- **Agent mode:** "Keep behind a flag". Proposing is the default; `ush --run //…`, or the `mode`
  setting, keeps the agent mode.
- **Several commands:** "One at a time". The first command goes on the command line; after it ran,
  `//` continues with its output and proposes the next one.

**Resolved:** propose mode, [ARCH](ARCH.md#propose-mode), TODO 2.17.

### Q7. Notes (TODO 2.25): which key, and which output?

Two decisions, from the user's request of 2026-10-29.

1. **The key.** Ctrl-N is zsh's "next history line" in emacs mode (Down does the same).
   Ctrl-N for notes would be the first key ush takes over outside a proposal; so far the user
   chose to keep keys as they are there (Esc, 2026-10-28).
   - **Proposed:** bind Ctrl-N in emacs mode for notes, as asked; Down keeps history. Make the key
     a setting, `USH_NOTES_KEY` (empty: no binding). In vi mode, leave it unbound.
   - **Answer** (2026-10-29 01:05, Gregor Heinrich): "then use ctrl-m". Ctrl-M is the byte Enter
     sends (`^M`), so a terminal can't tell them apart. Offered Ctrl-X n (unbound in zsh), Ctrl-N
     or Alt-N; the answer: "Ctrl-X n".
   - **Resolved:** Ctrl-X n, which changes no existing binding. `USH_NOTES_KEY` (empty: no
     binding) stays in TODO 2.25.
2. **Which output.** The plugin records output only for commands ush proposed. Recording every
   command's output would run every command with its output piped (no colours, no pagers).
   - **Proposed:** Ctrl-N on an empty line takes the last recorded output. Ctrl-N on a typed
     command runs it the way a proposal runs, recorded, and then gives notes. Any other command's
     output is not recorded.
   - **Answer** (2026-10-29, Gregor Heinrich): "implement the changes", given after the
     agent asked to confirm this or say otherwise; taken as acceptance of the proposal, with the
     key Ctrl-X n. **Resolved:** TODO 2.25 as proposed.

### Q8. Should ush add an agent and a sandbox?

Asked by the user on 2026-10-29, with the state of the art (docs/SOTA.md) in view.

- **Proposed** (agent): no larger agent. ush is small and focused on a manual workflow;
  planning, multi-step autonomy and file editing are what Claude Code, Codex CLI and Gemini CLI
  do, and `--run` already covers investigation. Instead, the OS sandbox (TODO 2.6) for what agent
  mode runs without asking, first; the argument-origin rule (2.23) unconditional rather than a
  fallback to the checker. Propose mode needs neither: the user runs every command.
- **Answer** (2026-10-29, Gregor Heinrich): "do it. promote the named items" (history as
  context, bash and other shells, argument origin).
- **Resolved:** TODO 2.6 and 2.23 rewritten; 2.26 (history), 2.27 (bash) and 2.28 (fish) added;
  ROAD reordered.

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | One person uses ush per account; under `sudo`, the invoking user (`SUDO_USER`) owns the key and settings. | [SEC](SEC.md), `config.py` | |
| A2 | Users accept that redacted command output goes to the selected provider, as the banner says before each request. | [SEC](SEC.md), README | |
| A3 | zsh is the only supported shell; bash and fish users quote requests or don't use the plugin. | [ARCH](ARCH.md), plugin | Until TODO 2.27 (bash) and 2.28 (fish) |
| A4 | Anthropic's default model stays `claude-opus-5-5` with effort `medium`, even though it is the most expensive choice. | `config.py`, README | |
| A5 | "An adversarial model to check the proposed implementation that takes independent sources; it can be a small model" (2026-10-27) means a runtime checker: a second model that judges each command or script the assistant proposes, from independent input (the user's requests only, not the output the assistant read). Not a reviewer of code changes in this repository. | TODO 2.13, [ARCH](ARCH.md), `check.py` | |
| A6 | The checker is off unless configured: it needs a model, adds a request per command, and small models raise false alarms. | `config.py`, README | |
| A7 | Allow rules are for the user's own account; a root session (`sudo ush`) uses none, even the invoking user's. | `agent.py`, `cli.py` | |
| A8 | In propose mode, the line the user runs from the prompt that held the proposal is its result, even if they replaced it entirely; only another `ush` call there counts as not running it. | plugin, `handoff.py` | |
| A9 | A conversation continues in the same shell for an hour without use; `--new` starts over. Under `sudo` it doesn't continue, because each `sudo` is a new parent process. | `handoff.py`, `cli.py` | |
