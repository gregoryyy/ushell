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
