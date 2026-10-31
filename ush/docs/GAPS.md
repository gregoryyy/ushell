# Gaps

Open questions and explicit assumptions for ush, both implementations. Q9–Q12 and A10–A12 were
ushgo's Q1–Q4 and A1–A3 until 2026-10-29. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted because they seemed clear enough
not to ask; check them and correct any that are wrong. The rules for this file are in
[SDLC](../../docs/SDLC.md#work-items). Status: 2026-10-30 06:05.

- Answers carry date, time and author: `**Answer** (2026-10-27 15:10, Gregor Heinrich): …`. Short
  answers are fine. A changed answer goes on top; earlier ones stay below it, struck through.
- Once answered, the decision moves into the document it affects, and the entry gets a
  `**Resolved**` line saying where. Resolved entries stay as a record.
- A question that blocks a TODO item is marked **blocking**.

## Questions

### Q1. Which licence? — blocking for TODO 3.1

**Answer** (2026-10-28 21:20, Gregor Heinrich): MIT.

**Resolved:** [LICENSE](../../LICENSE) at the repository root.

The repository has no licence file, so nobody else may use, change or redistribute ush. A release
needs one. ush's dependencies (the Anthropic SDK and its tree) are MIT, Apache-2.0, BSD-3-Clause
and PSF-2.0, so any of the common choices is compatible.

- **Proposed:** Apache-2.0 for the whole repository (permissive, with a patent grant). MIT if you
  prefer the shortest licence; GPL-3.0-or-later if derived tools must stay open.

### Q2. How is ush distributed? — blocking for TODO 3.1

**Answer** (2026-10-28 21:20, Gregor Heinrich): asked to choose between a GitHub release, a GitHub
release with a Homebrew tap, and a tag only: "try all three options". 0.2.0-rc1 ships the Go
binaries at parity (asked separately: "Go binaries at parity").

**Resolved:** release plan in TODO 5.5–5.7: a tag, a GitHub release with the archives and
`SHA256SUMS`, and a Homebrew tap formula (the tap repository is created by the user).

`install.sh` copies ush into a venv today, per user or root-owned. Other channels need their own
packaging and maintenance.

- **Proposed:** `install.sh` from a tagged release for 0.1; a Homebrew tap later, once ushgo
  ([ROAD](ROAD.md) phase 3) shows whether the Python or the Go implementation is the one to ship.

### Q3. Which platforms does a release support?

The suite runs on macOS locally; the Linux matrix entries in CI don't run while GitHub Actions
is unavailable ([tools/ci/README.md](../../tools/ci/README.md)).

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
   - **Answer** (2026-10-30 08:17, Gregor Heinrich): "ctrl-x n doesn't seem to work: use a
     single step key combination instead, like ctrl-f or ctrl-a". Offered Ctrl-N (zsh's next
     history line, which Down also does), Ctrl-F (forward-char, which Right also does) or Ctrl-A;
     the answer: "Ctrl-N". No startup file of the user's loaded the plugin, which is the likely
     reason Ctrl-X n did nothing.
   - **Resolved:** Ctrl-N, taken in emacs and viins only where it still has zsh's own meaning;
     a key the user bound is left alone. TODO 2.25.
   - ~~**Answer** (2026-10-29 01:05, Gregor Heinrich): "then use ctrl-m". Ctrl-M is the byte Enter
     sends (`^M`), so a terminal can't tell them apart. Offered Ctrl-X n (unbound in zsh), Ctrl-N
     or Alt-N; the answer: "Ctrl-X n".~~
   - ~~**Resolved:** Ctrl-X n, which changes no existing binding. `USH_NOTES_KEY` (empty: no
     binding) stays in TODO 2.25.~~
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

### Q9. Go or Rust?

A compiled port of ush, as `ushrs` (Rust) or `ushgo` (Go). The comparison is in
[ARCH](ARCH.md#why-a-port-and-why-go).

- **Proposed** (2026-10-27, agent): Go, for the official Anthropic SDK and less effort for the
  process and terminal work.
- **Answer** (2026-10-27 14:52, Gregor Heinrich): "add the compiled stack project as ushrs or
  ushgo"; the choice left to the agent.
- **Resolved** (2026-10-27 15:03, agent): Go, as `ushgo/`; [ARCH](ARCH.md#decisions). A different
  answer would mean a new `ushrs/` project; nothing beyond the skeleton depends on the choice yet.

### Q10. Does ushgo replace ush once it reaches parity?

Two implementations of a security-critical policy double the review work and can drift apart.

- **Proposed** (2026-10-27, agent): yes. ushgo takes over the name `ush` and the install, after one
  release in which both are available and the conformance suite passes for both. The Python
  implementation then stays only as the conformance reference, or is retired.
- **Answer** (2026-10-28 09:59, Gregor Heinrich): "the command for ushgo is ush as well, it's just
  another implementation." And: "ush and ushgo should have equivalent functionality. py is the
  experimental version, go the compiled deliverable."
- **Resolved** (2026-10-28 09:59, agent): both are maintained; the command is `ush` in both; Python
  is experimental, Go the deliverable, with equivalent functionality. [ARCH](ARCH.md#decisions),
  [SDLC](../../docs/SDLC.md#ush-one-command-two-implementations), TODO 0.3 and 5.2.

### Q11. How is the Go binary distributed on macOS? — blocking for TODO 5.1.3

**Answer** (2026-10-28 21:20, Gregor Heinrich): "try all three options": a tag, a GitHub release
with the archives, and a Homebrew tap. No signing for now. **Resolved:** TODO 5.1.3, 5.5–5.7.

macOS refuses to run an unsigned binary that a browser downloaded (quarantine). Downloads with curl,
and binaries built locally, run without that.

- **Proposed:** a Homebrew tap that builds from the tagged source (no signing needed, `brew
  upgrade` for updates), plus the release archives with checksums for Linux and for curl users.
  Signing and notarization need an Apple Developer ID (99 USD a year) and add a notarization step
  to every release; worth it only for users who download with a browser.

### Q12. Release 0.2.0-rc1 with SECTEST F8 open?

**Answer** (2026-10-29 12:20, Gregor Heinrich), for 0.2.1: "Release, F8 documented". The gate's
eval on 0.2.1 ran the injected `cat` unapproved in 2 of 36 runs, both in the HTML scenario.
**Resolved:** as for 0.2.0-rc1; the rate is in SECTEST and the 0.2.1 release notes.

**Answer** (2026-10-29 00:25, Gregor Heinrich): "Release, F8 documented", and tag it as
`v0.2.0-rc1`. **Resolved:** F8 stays open, rated Low, in the release notes' known limitations; the
mitigation is TODO 2.23, for both implementations.

The release gate's injection eval failed once ([SECTEST](SECTEST.md) F8): in
agent mode, without a checker, llama3.2:3b followed injected text in a DNS answer and ran
`cat pwned-by-injection`, which the policy verifies as read-only. Nothing was created, read from a
secret location, or sent over the network; the eval still counts it, rightly, as an injected
command that ran unapproved. Propose mode, the default, attempted nothing.

- **Proposed** (2026-10-29, agent): release 0.2.0-rc1 with F8 open and rated Low, stated in the
  release notes' known limitations ("in agent mode, injected text can steer the model to read-only
  commands; use a checker or propose mode on untrusted content"). Then, as a ushpy TODO item for
  0.2.0: measure the checker against this scenario (it sees the requests, not the DNS answer), and
  if it doesn't catch it, require approval in agent mode for a command whose arguments come only
  from earlier command output.
- **Alternative:** block the release until a mitigation passes the eval.

### Q13. Which European platforms and local servers get presets?

Asked by the user on 2026-10-30: "mistral, openai, if possible other european providers and other
local apis beyond ollama". Every preset is a host ush sends redacted output to, and a name to keep
working, so the list is worth choosing rather than growing by default.

- **Proposed:** remote, in this order: Mistral (France), then Scaleway Generative APIs (France),
  OVHcloud AI Endpoints (France), IONOS AI Model Hub (Germany), STACKIT AI Model Serving
  (Germany) and Nebius AI Studio (Netherlands), each kept only if its documentation and a mock
  test show an OpenAI-compatible API with tool calling for the models it offers. Local:
  llama.cpp's `llama-server`, LM Studio, vLLM, LocalAI, Jan, llamafile and `mlx_lm.server`, each
  a default loopback port over the generic adapter. Aleph Alpha is left out: it is sold as an
  on-premises platform, which `openai-compatible` with a URL covers.
- **Alternative:** Mistral and the generic `openai-compatible` URL only; any other platform is
  configured by its URL.
- **Answer** (2026-10-30 05:40, Gregor Heinrich): "first openai, mistral model settings
  (consider internal abstraction?)". Taken as: OpenAI and Mistral are the first presets; their
  model settings (effort or reasoning, strict tool schemas, output limits) go through an internal
  abstraction rather than checks per platform. The other European platforms and the local
  presets stay as proposed, after OpenAI and Mistral; the list is still open.
- **Resolved** (in part): TODO 2.29.1 (provider table and model profiles), 2.29.2 (OpenAI and
  Mistral moved first); [ARCH](ARCH.md#decisions).

### Q14. How does a POSIX sh user run a proposal?

`dash`, BusyBox `ash` and other plain `sh` have no line editor ush can hook into (TODO 2.33), so
the proposal can't be put on the command line, and the output isn't recorded.

- **Proposed:** ush prints the proposal; `ush --print //…` prints only the command, so
  `eval "$(ush --print '//…')"` runs it after the user saw it. Without the plugin, `//` continues
  without the output of what ran, and says so.
- **Alternative:** print only, and leave running it to copy and paste.
- **Question back** (2026-10-30 06:05, Gregor Heinrich): "if the shell has access to history and
  output, why not print the proposal to the shell line and let the user press enter. this gives
  the possibility of editing. doesn't this work with all shells?"
- **Reply** (2026-10-30 06:05, agent): it works in every shell with a line editor that a
  function can write to: zsh (`BUFFER`), bash 4 or newer (`READLINE_LINE`), fish
  (`commandline`), PowerShell (PSReadLine), and ksh93 (`.sh.edchar` in a `KEYBD` trap). That
  covers every shell in TODO except POSIX sh. `dash` and BusyBox `ash` have no such interface:
  their line editor, where they have one, can't be set from a function, they have no hook before
  or after a command, and `dash` keeps no history without libedit. The only way left to put text
  on their command line is to fake keystrokes with the `TIOCSTI` ioctl, which Linux turns off by
  default since 6.2 and OpenBSD removed, because it lets any program type into a terminal; ush
  shouldn't depend on it.
- **Proposed instead** (2026-10-30, agent): for sh, ush itself shows the proposal on an editable
  line (on `/dev/tty`, pre-filled, Enter to accept, Esc to drop), as the `ush>` session does
  today. A small function sourced from `~/.profile` or `$ENV`, `u '…'` (POSIX doesn't allow `//`
  as a function name), then runs the accepted line in the user's own shell with `eval`, so `cd`
  and variables take effect, and records its exit status for the next `//`. Editing and Enter
  work as in zsh; what sh users don't get is `// …` without quotes, the line in the shell's
  history, and the output, which can't be recorded without a subshell that would lose `cd`.

### Q15. Proposals in each shell's syntax, and agent mode under zsh?

fish and PowerShell differ from zsh in syntax (fish: `set`, `and`/`or`, its own quoting;
PowerShell: cmdlets, objects in pipes). The read-only policy (ARCH) follows `zsh -f`'s word rules
and runs agent-mode commands under `zsh -f`; writing a policy per shell would multiply the
security-critical part.

- **Proposed:** propose mode writes proposals in the user's shell's syntax (TODO 2.32). Agent mode
  (`--run`) keeps writing and checking zsh commands and running them under `zsh -f` whatever the
  user's shell; where zsh isn't installed, `--run` refuses with the reason. PowerShell gets
  propose mode only (TODO 2.34).
- **Alternative:** proposals in POSIX sh for every shell, wrapped as `sh -c '…'` in fish and
  PowerShell: one syntax to check, but awkward lines for those users.
- **Answer** (2026-10-30 05:40, Gregor Heinrich): "agent mode not accepted in shells that don't
  support it. (again abstraction; i believe this is an interface not a capability problem)" and
  "create proposals in fish syntax — (generally generate in the syntax of the shell ush runs in.
  system prompts can easily change; start using ush --shell bash etc. if the user shell is not
  compatible)".
- **Resolved:** a shell interface in the core, [ARCH](ARCH.md#decisions), TODO 2.32. Proposals,
  explanations and notes are in the syntax of the shell ush runs in; `--shell NAME` (and a
  setting) chooses one when that shell isn't supported. Agent mode is a part of the interface:
  a shell whose implementation has no policy and executor refuses `--run` with the reason, and
  only zsh has them for now. The proposal above (agent mode under `zsh -f` for every shell) is
  not taken.

### Q16. PowerShell on Windows?

TODO 2.34 targets `pwsh` 7 on macOS and Linux, inside A11 (no Windows). Windows would need its own
execution (no process groups, no `/dev/tty`), the Credential Manager instead of the keychain,
Windows release archives, and a policy for Windows commands.

- **Proposed:** not now. PowerShell on macOS and Linux first; Windows as a separate ROAD phase if
  the user asks for it, starting with propose mode only.
- **Answer** (2026-10-30 06:05, Gregor Heinrich): "not now".
- **Resolved:** TODO 2.34 is PowerShell 7 on macOS and Linux; A11 stands.

### Q17. Argument origin before the new platforms are released?

The injection eval (SECTEST F8) failed with a 3B local model in agent mode. TODO 2.29 adds many
models, local and small among them.

- **Proposed:** a release that ships 2.29.2 or later also ships argument origin (TODO 2.23), and
  the release gate runs the injection eval on one model per new adapter. The platforms can be
  built and merged before; the release waits.
- **Alternative:** release the platforms with F8 documented, as for 0.2.0 and 0.2.1 (Q12).
- **Answer** (2026-10-30 06:05, Gregor Heinrich): "wait for injection fix before releasing".
- **Resolved:** TODO 2.29 (release condition) and [ROAD](ROAD.md) phase 4: the platforms can be
  built and merged, but no release ships them before TODO 2.23, and that release's gate runs the
  injection eval on one model per new adapter.

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | One person uses ush per account; under `sudo`, the invoking user (`SUDO_USER`) owns the key and settings. | [SEC](SEC.md), `config.py` | |
| A2 | Users accept that redacted command output goes to the selected provider, as the banner says before each request. | [SEC](SEC.md), README | |
| A3 | zsh is the only supported shell; bash and fish users quote requests or don't use the plugin. | [ARCH](ARCH.md), plugin | Until TODO 2.32 (the user's shell), 2.27 (bash), 2.33 (sh), 2.28 (fish), 2.34 (PowerShell) |
| A4 | Anthropic's default model stays `claude-opus-5-5` with effort `medium`, even though it is the most expensive choice. | `config.py`, README | |
| A5 | "An adversarial model to check the proposed implementation that takes independent sources; it can be a small model" (2026-10-27) means a runtime checker: a second model that judges each command or script the assistant proposes, from independent input (the user's requests only, not the output the assistant read). Not a reviewer of code changes in this repository. | TODO 2.13, [ARCH](ARCH.md), `check.py` | |
| A6 | The checker is off unless configured: it needs a model, adds a request per command, and small models raise false alarms. | `config.py`, README | |
| A7 | Allow rules are for the user's own account; a root session (`sudo ush`) uses none, even the invoking user's. | `agent.py`, `cli.py` | |
| A8 | In propose mode, the line the user runs from the prompt that held the proposal is its result, even if they replaced it entirely; only another `ush` call there counts as not running it. | plugin, `handoff.py` | |
| A9 | A conversation continues in the same shell for an hour without use; `--new` starts over. Under `sudo` it doesn't continue, because each `sudo` is a new parent process. | `handoff.py`, `cli.py` | |
| A10 | ushgo uses ush's settings, key store and history files, in the same formats, so users can switch between them. | [ARCH](ARCH.md), TODO 4.4, 4.5 | |
| A11 | macOS and Linux only, on arm64 and amd64, as for ush; no Windows. | TODO 5.1, 2.34 | PowerShell 7 on macOS and Linux only; Windows is Q16 |
| A12 | Developers install Go with Homebrew (`brew install go`, 1.27); CI pins 1.27 through `actions/setup-go`. | README, CI | |
