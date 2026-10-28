# DONE

Progress log for [TODO](TODO.md), newest first. Each entry starts with the date and time of day
(local time) and a title, then the TODO items and the PR, then the agent that did the work as
`Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or `Agent: human`), then:
- **Implementation notes:** what was built and where, with the exact validation and its results.
- **Deviations:** what differs from the plan, and why.
- **Insights:** what we learned that matters for later work.

Entries are never rewritten; corrections and later outcomes (merges, releases) are new entries on
top. The format is defined in [SDLC](../../docs/SDLC.md#done-entries).

---

## 2026-10-28 19:33 — Explain a proposal: Down, or `ush explain`

TODO items: 2.19. PR: see below (with 2.18).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request:** "add a function to explain the proposed command. arrow down beyond the proposed
  command explains the command, using the language model."
- **`ush explain COMMAND…`** (`cli._explain`) runs nothing:
  - It prints the disclosure (`explaining, runs nothing · the command goes to …`) and ush's policy
    verdict, which is local, without the model.
  - It sends one request without tools through the provider's `complete`, as the checker does.
    The system prompt asks for the whole first, then each part, what it reads, changes, sends,
    and what is risky.
  - If this shell has a pending proposal, it adds the user's last request and the proposal's
    purpose, or notes that the command was edited.
  - The command sits between markers with a random tag, as data; everything sent is redacted.
  - The proposal now keeps its `purpose` in the saved conversation for this.
- **Down in the plugin** (`_ush_explain`):
  - It is bound to `^[[B` and `^[OB` in the main keymap, only while a proposal is on the line,
    with the same save and restore as Esc (`_ush_keys_bind` / `_ush_keys_restore`).
  - It explains only at the newest history line (`HISTNO == HISTCMD`), past the last line of the
    buffer, and with something on it. Otherwise it calls the key's own widget.
  - `zle -I`, then `ush explain -- "$BUFFER"`; zle redraws the prompt with the proposal still on
    it, and the explanation shows what's on the line, edits included.
- **Docs:** the guides' "Proposals" and "Other commands", what leaves the machine, the ushpy README
  and usage text. ROAD: the `ush explain` idea is now done. ushgo TODO 1.12 includes it.
- **Validation:**
  - `just ush-test`, both suites OK.
  - `test_explain_runs_nothing_and_uses_the_proposal_context`, against the mock API:
    - the context lines, no tools;
    - the synthetic key redacted;
    - nothing created; the handoff proposal untouched;
    - `ush explain` without a command is a usage error.
  - `test_down_explains_the_proposal_and_keeps_it`, in a real zsh:
    - Down calls `ush explain -- 'echo one'`, and the proposal stays and then runs and is
      recorded;
    - Down without a proposal calls no ush, and its binding is `down-line-or-history` again.
  - The raw terminal output of an explanation, two lines, shows `\r\n` line ends and the prompt
    redrawn with the proposal.

### Deviations

- `ush explain` is also a subcommand for any command line, since Down needs a program to call; it
  was an unscheduled ROAD idea.

### Insights

- `complete`, written for the checker, serves any one-shot question; explanations are not part of
  the conversation, so they don't disturb the pending proposal.

---

## 2026-10-28 19:28 — The plugin on install; Esc removes a proposal

TODO items: 2.18. PR: see below.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The report:** "there's no way to get the proposed command directly on the commandline.
  pre-fill the commandline with the command, esc to remove it and start from scratch". The cause
  was that the plugin wasn't in the user's `~/.zshrc`. The installer only printed the line, and
  without the plugin ush can only print a proposal: a child process can't fill its shell's
  command line.
- **`shell/zshrc.sh`, used by both installers:**
  - It adds `source '…/ush.plugin.zsh'` to `${ZDOTDIR:-$HOME}/.zshrc`.
  - `ask` (the default) asks only when stdin and stdout are a terminal, with yes as the default
    answer.
  - `--zshrc` adds the line without asking; `--no-zshrc` and runs without a terminal only print it.
  - It never adds the line twice (any `ush.plugin.zsh` line counts) or as root, and refuses a
    path with a quote.
  - The Go release archive carries the script.
- **Esc:**
  - The binding: the user's follow-up, "esc should only be used within the context of a command
    proposed after //, everything else stays". So `precmd` binds Esc in the emacs keymap only
    when it puts a proposal on the line. It saves the old binding as `bindkey -L` prints it,
    and restores it in `preexec`, in the next `precmd` (a line abandoned with Ctrl-C), and when Esc
    is used.
  - What Esc does: it clears the line and the offered state, so `//` hears that nothing ran.
  - Vi mode is left alone; Ctrl-U clears the line there.
- **Without the plugin**, ush now prints the exact `source` line for the installed plugin (next to
  the venv).
- **Docs:** the guides' shared "First steps" and "Proposals"; both ADMIN install sections; the
  ushpy README.
- **Validation:**
  - `just ush-test`: ushpy OK; ushgo ok, including the release build, which now unpacks
    `zshrc.sh` and runs `install.sh` with a temporary `HOME`.
  - New `test_install`, 5 tests: each mode, an existing line, `ZDOTDIR`, a quote, and the
    question answered in a real terminal with Enter and with `n`.
  - Two new plugin tests in a real zsh: Esc clears the line and nothing counts as run. A custom
    Esc binding (`backward-word`) is still there after a proposal ran, was removed with Esc, and
    was abandoned with Ctrl-C.

### Deviations

- None.

### Insights

- Binding Esc only around a proposal keeps zsh's own handling of Esc as the start of Alt-key and
  arrow-key sequences everywhere else, and keeps any binding the user made.

---

## 2026-10-28 19:13 — Propose mode: ush proposes, the user runs

TODO items: 2.17.1 (2.17.2, the manual checks, stays open); ushgo TODO 1.12 ports it. GAPS Q6
answered by the user. PR: see below.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request:** "the idea of the ushell is to propose specific shell commands and prewrite them
  to be executed by the user." The user's choices (GAPS Q6):
  - ush proposes investigation commands too, rather than running them;
  - agent mode stays, behind a flag;
  - one command at a time.
- **Agent** (`agent.py`):
  - `Options.propose` selects one strict tool, `propose_command` (`command`, `purpose`, `effect`,
    `interactive`), and a system prompt that says ush runs nothing.
  - The first valid proposal is shown with notes: a change, not verified as read-only, or the
    checker's objection. Any other tool_use in that turn gets a "one at a time" result, due with the
    next request.
  - `ask(request, ran, tracked)` answers the pending proposal with what ran: same or edited
    command, exit status, changed working directory, and redacted bounded output, or that it
    didn't run, or that ush can't tell (no plugin).
  - `state()` and `restore()` carry the conversation between processes.
  - Proposals with C0/C1 controls, bidi overrides or zero-width characters fail validation, and the
    model is asked again.
- **Handoff** (`handoff.py`, new):
  - The plugin's directory (`USH_HANDOFF`) is used only if it is the user's, 0700, not a link,
    and never as root.
  - `write_proposal` replaces atomically. `take_ran` reads `ran` and `output` with `O_NOFOLLOW`
    as regular files, keeps head and tail like execution does, and deletes them.
  - `Sessions` keeps one 0600 conversation per shell in `state/ush/sessions/`. It is keyed by the
    handoff directory, or the parent process without it, expires after an hour, and saving prunes
    other expired ones.
- **CLI** (`cli.py`):
  - The `mode` setting (default `propose`), `-r/--run`, `-N/--new`, `ush //` to continue, and
    `--dry-run` implies agent mode.
  - `_propose` runs one step and hands over the proposal, or prints it without the plugin.
  - `ai configure` asks for the mode first.
  - `ai status` shows the mode.
  - The session pre-fills the `ush>` prompt with the proposal (readline), and a line run there
    answers it.
- **Plugin:**
  - `precmd` (put first, as it needs `$?`) pushes the proposal with `print -rz`, records the exit
    status and command line, and ends the capture.
  - `preexec` treats the line run from that prompt as the result, unless it is another `ush`
    call. Unless interactive, it sends stdout and stderr through a FIFO into `tee`.
  - `zshexit` removes the directory.
  - A bare `//` now becomes `ush //` (continue), not `ush`.
- **Also fixed:** `ush ai configure` took the answer `on` as an Ollama model named "on". It now
  refuses `on`, `yes`, `true` and `enable`, and checks an Ollama checker against the pulled models.
- **Docs:**
  - ARCH (section "Propose mode", decision), SEC (assets, boundaries, four threats, two residual
    risks), GAPS (Q6, A8, A9).
  - The guides' shared sections ("Proposals", "Agent mode", options, data kept); ADMIN (`mode`,
    conversations, handoff, `USH_HANDOFF`, sudo).
  - Both READMEs, usage, SMOKE (M1–M10 marked agent mode, M11–M15 new), `smoke.sh` (two propose
    checks; agent checks with `--run`).
  - Root ROAD step 3, plus two ideas the user added: `ush analyze` and data loss prevention
    filters.
- **Validation:** `just ushpy-test`: 129 tests OK (108 before). New:
  - `test_propose`: 15 tests (agent and handoff).
  - Two CLI propose tests: handoff and continue end to end against the mock API; without the
    plugin, and `--new`.
  - `ZshProposeTest`: 4 tests in a real zsh through a pty:
    - the proposal pre-filled;
    - output captured by `tee`;
    - an edited, failing command recorded with exit 3;
    - an interactive one not captured;
    - a new request instead counted as not run;
    - the directory removed on exit.
  - Smoke-test fixtures for the new checks.

### Deviations

- Propose mode runs the policy and the checker only as advice, since nothing runs without the
  user's Enter. Agent mode is unchanged.
- **Output capture** uses `tee` in the user's own shell, so `cd` and variables persist. The cost is
  a pipe as stdout for that one command; a pty wrapper would keep the terminal, but would run the
  command in a child shell.

### Insights

- Your history showed what prompted the request. In agent mode the commands were displayed and
  approved, but the investigation and the answer read as one block. Proposing makes each step the
  user's own action.
- `print -z` from `precmd` gives a pre-filled command line without replacing ZLE widgets, so it
  coexists with other plugins.

---

## 2026-10-28 18:15 — Checker names without the provider; `ai configure` asks again; tests kept off the keychain

TODO items: none new; fixes to 2.13 (checker) and the test suite, from the user's first
`ush ai configure`. ushgo TODO 1.4 ports the behaviour. PR: see below.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The report:** "the model name expected seems to require the provider twice". After choosing
  Anthropic, the checker answer `claude-haiku-5-5` was refused as "not provider:model". The error
  also ended `ai configure`, so the answers given before it (model `claude-haiku-5-5`) were lost.
- **Bare checker names** (`Settings.set_checker`): `claude-…` is Anthropic's, anything else
  Ollama's (`qwen3.5:4b`, `library/qwen3.5:4b`). The explicit `anthropic:` / `ollama:` form still
  works. The setting is stored as `provider:model`, so the file stays unambiguous. The rule
  depends on the name, not on the main provider, so `-k`, `USH_CHECKER` and the settings file
  give the same result in any order.
- **`ai configure` asks again** (`cli._ask`) after an invalid provider, Ollama URL, model, effort
  or checker. An empty answer keeps the current value, as before. It also prints how it read the
  checker (`checker: claude-haiku-5-5 through anthropic`).
- **Test isolation bug, found while testing this:**
  - `test_missing_key_fails_before_anything_runs` ran `//why is my disk full` without a key and
    without a mock endpoint. On the user's machine, the key just stored in the macOS keychain
    stood in for the missing one. The test then most likely sent a real request to Anthropic, and
    timed out after 60 s.
  - Fixed twice over:
    - `CliTest.ush` now points `USH_BASE_URL` and `USH_OLLAMA_URL` at a closed loopback port
      (127.0.0.1:9) unless a test sets a mock, so no CLI test can reach a real provider.
    - The missing-key test runs in-process, with `SECURITY` patched to a missing path and an empty
      stdin, so it doesn't depend on the developer's keychain.
- Usage text, both guides' shared sections, `.env.template` and the ushpy README describe the
  bare form.
- **Validation:**
  - `just ushpy-test`: 108 tests OK, among them the new
    `test_bare_checker_names_choose_the_provider_by_name` and
    `test_configure_asks_again_after_a_bad_answer`.
  - Reinstalled with `ushpy/install.sh`; `ush --help` shows `-k, --checker MODEL`.

### Deviations

- None from the plan. The isolation fix goes beyond the reported problem, because the suite
  failed on the machine once a key was stored.

### Insights

- The keychain belongs to the user, not to `XDG_*`: tests that isolate settings through
  `XDG_CONFIG_HOME` still see a real key. Any test that runs the CLI without a mock must not be able
  to reach a provider at all. The closed-port default makes that hold for future tests too.
- The rule "tests never call a paid provider" (AGENTS.md) was broken by the environment, not by a
  test that meant to. Release gate checks should include a run on a machine with a stored key.

---

## 2026-10-28 10:23 — User allowlist; no blacklist mode

TODO items: 2.15 (ported by ushgo TODO 1.11); new GAPS Q5 and A7. PR:
[#11](https://github.com/gregoryyy/ushell/pull/11) (stacked on #10).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- `policy.py`: `parse_rule` (plain words, at most 8; refused for commands in the table, runners
  and interpreters, and commands that change or send data), `classify(command, rules)` lets a
  simple command the table doesn't know run if its words start with a rule; secret paths are still
  checked; `Verdict.rule` names the rule(s).
- `config.py`: `allow` in the settings file, `add_rule`, `remove_rule`, `allow_rules`; an invalid
  rule in the file is a ConfigError. No environment variable.
- `agent.py`: rules from the settings, none as root; the prompt line says "allowed by your rule";
  `Step.rule` in history. `cli.py`: `ush allow [add|remove RULE]`, the rule count in `ai status`,
  rules in `replay`.
- Docs: README, ARCH ("The user's allowlist", and why no blacklist mode), SEC (threat row,
  residual risk of broad rules), SECTEST, GAPS Q5 (deny rules that only add friction, proposed),
  A7; ushgo TODO 1.11; root ROAD.
- **Validation:** `just ushpy-test`: 104 tests OK (96 before); new tests: `AllowRuleTest` (4: matching, prefix
  semantics, the rest of the policy, refused rules), `AllowSettingsTest`, `AllowRuleAgentTest` (2:
  a rule runs and is shown and recorded; a `change` or a root session still asks), and the CLI
  test for `ush allow` and `ai status`. `just ci`: see the PR.

### Deviations

- None from the request; the refusal list for writing commands goes beyond "a whitelist" because a
  rule for `rm` would let a forged `read` claim run it.

### Insights

- The model's `effect` claim narrows but never widens, so a rule only matters for commands the
  model calls `read`; that is also why rules for writing commands are refused, not merely warned.
- A single-word rule for a multi-purpose tool (`git`) allows its writing subcommands; the README
  shows narrow rules, and SEC records the risk.

---

## 2026-10-28 09:59 — User smoke test

TODO items: 2.1.1 (2.1 and 2.1.2 stay open for the user's paid run). PR: [#8](https://github.com/gregoryyy/ushell/pull/8) (stacked on #7).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- `scripts/smoke.sh`: smoke test of an installed `ush`, either implementation (`--bin`). Scratch
  directory, scratch `XDG_*` state, the user's `USH_*` variables ignored; offline checks (version,
  help, usage error, `ai status` without the key in full, no session without a terminal, empty
  history), model checks without a terminal (read-only request, disclosure and checker banners,
  declined change, injected instructions, piped input, dry run, replay); a model check whose
  request fails fails; `--paid` required for Anthropic; `--ollama-url`; `smoke-report.md`.
- [SMOKE.md](SMOKE.md): principles, the checks, manual checks M1–M10, what a failure means.
- `ush --version` prints `ush 0.1.0 (python)`, so the report says which implementation ran.
- Docs: README smoke section; SDLC section on the two implementations (Python experimental, Go the
  deliverable, how a feature moves, the shared contract); ROAD phase 3.
- **Validation:**
  - `test_smoke` (5 tests): offline pass for Python; a fake `ush` that accepts everything fails;
    `--paid` required; the model checks against a scripted mock Ollama pass and use every scripted
    turn; failing requests fail their checks. `test_cli`: the version line.
  - Live, local: `smoke.sh --bin <python ush> --provider ollama --model llama3.2:3b --checker
    ollama:llama3.2:3b`: 13 PASS, 1 WARN (the model proposed no command for the file request).
  - Offline against the Go binary: 4 PASS, 2 FAIL (`ai status`, `history`: not ported yet).
  - `just ush-test`, `just ci`: see the PR.

### Deviations

- TODO 2.1 asked for an opt-in test behind `USH_LIVE_TEST=1` in the suite. A user-run script fits
  better: it tests the installed command, not the source tree, and serves both implementations.
  2.1 is split into the kit (2.1.1, done) and the paid run (2.1.2, open).

### Insights

- The first version cleared all `USH_*` variables, so a test's `USH_OLLAMA_URL` was dropped and
  requests went to the real local Ollama, which lacked the model; two checks passed on failed
  requests. Hence `--ollama-url` and the rule that a failed request fails its check.

---

## 2026-10-27 15:15 — Model menu and checker model

TODO items: 2.12, 2.13. PR: [#6](https://github.com/gregoryyy/ushell/pull/6) (stacked on #5).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **Model menu (2.12):** `.env.template` lists `USH_MODEL` choices: Anthropic's three tiers, and
  Ollama tags by size with download size and memory needs (qwen3.5 2b/4b/9b/27b/35b-a3b,
  gemma4:12b; sizes from ollama.com/library on 2026-10-27), plus the `USH_CHECKER` choices. The
  `just ush` recipe drops a trailing ` # comment`, so a menu line works when uncommented. The local
  `.env` (ignored) was regenerated from the template with its values kept.
- **Checker (2.13):** `check.py`: `Checker` (lazy client), `build_prompt` (requests and proposal
  between random-tag markers), `parse` (one `OK` or `FLAG: reason` line; `<think>` blocks
  skipped; anything else is no verdict), a system prompt with five worked examples. `provider.py`:
  `complete()`, a non-streaming request without tools or thinking (2,048 tokens, 60 s). `agent.py`:
  after the policy, before approval; the agent keeps the user's requests and passes them and the
  proposal redacted; a flag or no verdict sets `auto = False` and adds the reason to the prompt;
  `Step.check` records the verdict in history. `config.py`: `checker` setting (`provider:model`),
  `USH_CHECKER` (not as root). `cli.py`: `-k/--checker`, `ai configure` asks for it, banner and
  `ai status` say where the checker's input goes; the Anthropic key is resolved when either model
  needs it. `ui.py`: the verdict under the proposal.
- **Docs:** README (checker section, option, settings, what leaves the machine, limitations), ARCH
  (component, request flow, decision), SEC (trust boundary, two threats, residual risk), SECTEST
  (tests, F2 status, results), GAPS (Q4 default, A5 interpretation, A6 off by default).
- **Validation:**
  - `just ush-test`: 91 tests OK (73 before; 18 new: `test_check` 12, `CheckerSettingsTest` 2,
    `EnvTemplateTest` 1, `CompleteTest` 2, one CLI end-to-end test).
  - pyright 1.1.414 over `src/ush`: the same 19 errors as before the change, none new.
  - Live, local, no cost: the checker against Ollama 0.40.7 with qwen2.5:1.5b and llama3.2:3b on
    six, then seven proposals (SECTEST "Results"). The worked examples took llama3.2:3b from 3 to
    6 of 7 expected verdicts, catching 4 of 4 attacks; qwen2.5:1.5b caught 1 of 4 with them.
  - `just ci`: see the PR.

### Deviations

- The request was ambiguous ("an adversarial model to check the proposed implementation that
  takes independent sources"); it was built as a runtime checker of proposed commands and
  recorded as GAPS A5, so it can be corrected.
- No newer small model (qwen3.5:2b or 4b) was pulled to measure: that's a multi-GB download for the
  user to choose. The menu recommends the measured llama3.2:3b and marks qwen3.5:4b as unmeasured.

### Insights

- Small models need worked examples: without them, both flagged `df -h /`; with them, the 3B model
  passed it and caught every attack. A model under 2B is no useful checker.
- Failing closed matters more than accuracy: a false alarm costs one approval, a missed attack
  leaves the policy's decision in place, but "no verdict" must never mean "run".
- The checker narrows F2 (DNS exfiltration) only when configured; TODO 2.10 still has to close it
  in the policy.

---

## 2026-10-27 14:52 — Documents moved to the shared regime

TODO items: none of ush's; the change is wish TODO 0.8 ([wish DONE](../../wish/docs/DONE.md)).
PR: [#4](https://github.com/gregoryyy/ushell/pull/4).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- `ush/TODO.md` and `ush/DONE.md` moved to `ush/docs/`, `docs/DESIGN.md` renamed to
  [ARCH.md](ARCH.md), and [ROAD.md](ROAD.md) and [GAPS.md](GAPS.md) added, following
  [SDLC](../../docs/SDLC.md). TODO IDs and their status are unchanged.
- This log is now newest first. The entries below the marker use the earlier, oldest-first format
  and are kept as they were, except for one path-only link fix in the 1.1–1.9 entry (its links to
  `README.md`, `docs/DESIGN.md` and `docs/SEC.md` point to the moved files).

### Deviations

None.

### Insights

- The earlier entries' acceptance and validation details carry over to the new format unchanged:
  the new format names the same content (validation, deviations, learnings) under fixed headings.

---

## Earlier entries (oldest first, earlier format)

## 2026-10-26 — 1.1–1.9 ush 0.1: the local `//` shell for zsh

Branch: `feature/split-wish-ush` (local; not committed or pushed at the time of
writing). No PR yet.

Implemented:

- `ush` Python package (`src/ush/`): `cli`, `agent`, `policy`, `execute`, `redact`,
  `ui`, `provider`, `config`, `history`. Features:
  - `//` requests, follow-up questions, sessions, and piped input.
  - Agent loop with strict `run_command`/`run_script` tools.
  - Read-only policy with a zsh-aware tokenizer and secret-path checks.
  - Approval prompts with yes/no-with-reason/edit/abort, read from `/dev/tty`
    when stdin is a pipe.
  - Bounded execution under `zsh -f -o pipefail`.
  - Redaction; append-only history and `replay` without AI.
  - Keychain or 0600-file key storage; identity mapping under sudo.
- `shell/ush.plugin.zsh`: a `zle-line-finish` hook that quotes `ush //…`,
  `sudo [opts] ush //…` (absolute path), and bare `// …`; completion.
- `install.sh` (per user, or root-owned with `--prefix`), `scripts/test.sh`,
  `requirements.lock` (hashed, universal, `uv pip compile`), and `pyproject.toml`.
- Tests (63): policy, redaction, execution, config/keys, history, agent (fake
  provider), provider (real SDK against `tests/mock_api.py`), CLI end-to-end
  without a terminal and in a pty, and the zsh plugin in a pty with fake
  `ush`/`sudo`.
- Docs: [README.md](../README.md), [DESIGN.md](ARCH.md), [SEC.md](SEC.md),
  [TODO.md](TODO.md). Repository CI: `.github/workflows/checks.yml` (links; ush on
  Ubuntu/macOS × Python 3.10/3.13; actions pinned by commit SHA).

Validation (macOS 26, arm64, zsh 5.9):

- `just ush-test` with Python 3.11.12: 63 tests OK.
- Python 3.10 and 3.13 venvs created with `uv venv -p 3.10|3.13`, dependencies
  installed with `uv pip install --require-hashes -r requirements.lock`, then
  `python -m unittest discover` in `tests/`: 61 tests OK on each. These ran before
  the last two tests were added; the 63-test run used 3.11 only.
- `./install.sh --prefix <scratch>`: installed. `ush --version`, `ush ai status`,
  and the missing-key error behave as documented.
- `uvx pip-audit@2.9.0 -r requirements.lock --require-hashes --disable-pip`: no
  known vulnerabilities. Licenses: MIT, Apache-2.0, BSD-3-Clause, PSF-2.0 (see
  DESIGN decisions).
- `python3 tools/check-links.py`; `sh -n`/`zsh -n` on the scripts and plugin.
- `just pdfs ush wish/README.md`: every PDF was written. Not inspected visually,
  because `pdftoppm` is not installed.
- Not run: any live API request (no key configured; paid calls need explicit
  opt-in, TODO 2.1); keychain access under sudo (TODO 2.2); GitHub CI (TODO 2.3).

Deviations and decisions:

- Name: `wsh` and then `zish` were requested; both are taken (Wave Terminal's `wsh`
  CLI; an active Zig shell named `zish`). The user chose `ush`, "the you shell".
- Python instead of wish's TypeScript core. Reasons: sudo/root-owned installs,
  standard-library pty/process control, and the official SDK. SDK 2.3.0 (released
  2026-10-23) targets API version 2026-10-21 and sends keys as bearer tokens; it
  requires Python ≥ 3.10, so macOS `/usr/bin/python3` (3.9) is unsupported.
- `actions/setup-python` is pinned to v7.0.0 because v7.1.0 was four days old.

Learnings:

- `shlex` loses quoting, so it cannot tell `'*'` from `*`. Shell expansion can
  inject options (`find *` with a file named `-delete`), so the policy expands
  words the way `zsh -f` does before validating them.
- Modifying `$BUFFER` in a `zle-line-finish` hook takes effect before zsh parses
  the line, so unbalanced quotes in natural language are handled without
  wrapping `accept-line`.
- zsh emits a bracketed-paste escape after the prompt; pty tests must strip it.
- During development, a manual pty experiment invoked the real `sudo`, which
  produced two failed sudo authentications in the system log. The committed
  tests use a fake `sudo`.
- Importing the Anthropic SDK takes ~2 s warm on this machine; follow-ups in a
  session avoid it.

Status (2026-10-26): committed as `de2a5b4` (`feat(ush): add ush 0.1, the local //
shell for zsh`) on `feature/split-wish-ush` and opened as
[PR #1](https://github.com/gregoryyy/ushell/pull/1). Before committing, `just ush-test` was rerun
on this revision: 63 tests OK. The PR's CI run is the first one (TODO 2.3). Not merged.

## 2026-10-27 — 2.8 Local models through Ollama; security test plan

Branch: `feature/ush-2.8-ollama`, stacked on `feature/local-ci` (PR #2, open).

Question from the user: can ush run on a local Ollama? A probe with ush's own provider
class against Ollama 0.40.7 (`/v1/messages`, Anthropic-compatible) returned a correct
`run_command` tool call from `llama3.2:3b`. Pointing `USH_BASE_URL` at Ollama already
worked end to end, but the banner said "go to Anthropic", a dummy key was needed, and
it couldn't work under sudo. This milestone makes it a supported provider.

Implemented:

- `config.py`: `provider` (`anthropic`/`ollama`), `ollama_url`, `ollama_model`;
  `set_provider`/`set_model` validate per provider (the configure prompt didn't
  validate model names before); `check_ollama_url` accepts only http(s) to a loopback
  address without credentials, path, query, or fragment; `apply_environment` applies
  `USH_PROVIDER`/`USH_MODEL`/`USH_OLLAMA_URL` except as root.
- `provider.py`: error mapping moved into `_error`; `OllamaProvider` sends only model,
  limits, system, tools, and messages, uses a placeholder key, and a plain `httpx2`
  client with `trust_env=False` and no redirects. `ollama_remote_host` reads
  `remote_host` from `/api/tags`; a `cloud` tag counts as remote regardless.
- `cli.py`: provider-aware banner (`disclosure`), `ai status`, `ai configure`
  (provider first; lists pulled models; saves settings without environment overrides);
  no key needed for Ollama; a missing Ollama model fails before anything runs.
- `.env.template` (from `../kibitz/.env.example`, reduced to ush's variables), `.env`
  git-ignored; `.env` created locally with the Anthropic key copied from
  `../kibitz/.env` (mode 600, not committed). `just ush` runs ush from source with
  `.env`, reading it as KEY=value lines; shell variables win.
- Tests (+10, 73 total): URL acceptance/refusal cases, settings round trip and remote
  URL refusal, per-provider model names, environment vs. root; Ollama request shape
  with a synthetic Anthropic key and bogus proxy variables in the environment, error
  mapping (404, 410, 401, connection refused); remote-host detection; CLI end to end
  without a key, cloud disclosure, misconfiguration before any request.
- Docs: `docs/SECTEST.md` (new security test plan: levels, eight test classes with
  status, findings, release gate); SEC (trust boundaries, threats, residual risks),
  DESIGN decision, README Ollama section; TODO 2.8 done, 2.9–2.11 added.

Validation (macOS 26, arm64):

- `just ush-test`: 73 tests OK (Python 3.11.12).
- Live, local Ollama 0.40.7: banner "stay on this machine" for `llama3.2:3b`; a missing
  model reports `ollama pull`; a non-loopback URL exits 2 before any request; with
  `HTTP_PROXY`/`ALL_PROXY` pointing at a closed port the request still reached Ollama.
  The cloud-model banner check sent one benign request ("how much free disk space is
  on /? use df", no command output) through Ollama to ollama.com, which refused it
  (model retired, 410).
- `just ush ai status` with and without `USH_PROVIDER=ollama` in the shell.

Learnings:

- The SDK's `DefaultHttpxClient` mounts environment proxies even with
  `trust_env=False`; the bogus-proxy test caught it.
- `llama3.2:3b` sometimes answers without running commands or prints tool calls as
  text; the README recommends a model with reliable tool calling.
- Writing the test plan found two gaps, recorded as SECTEST F1 (unfiltered terminal
  output can spoof the approval prompt; TODO 2.9) and F2 (DNS exfiltration through
  auto-approved `dig`/`host`/`nslookup`; TODO 2.10).
