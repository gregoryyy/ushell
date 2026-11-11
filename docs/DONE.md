# DONE

Progress log for [TODO](TODO.md), for both implementations of ush, newest first. Each entry starts
with the date and time of day (local time) and a title, then the TODO items and the PR, then the
agent that did the work as `Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or
`Agent: human`), then:
- **Implementation notes:** what was built and where, with the exact validation and its results.
- **Deviations:** what differs from the plan, and why.
- **Insights:** what we learned that matters for later work.

Entries are never rewritten; corrections and later outcomes (merges, releases) are new entries on
top. The format is defined in [SDLC](SDLC.md#done-entries). Until 2026-10-29 each
implementation kept its own log: [ushpy](archive/DONE-ushpy.md), [ushgo](archive/DONE-ushgo.md).

---

## 2026-11-11 08:24 — Fn-Backspace, Home and End no longer insert `[3~`

TODO items: 2.40 (done; the item from the local branch `docs/ush-interaction-followups`).
PR: not yet opened (branch `fix/ush-2.40-delete-key`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The report** (user, 2026-11-11): "the [3~ issue exists for fn-backspace".
- **Reproduced** in a pty with `zsh -f` and `vared`:
  - With a widget on the bare Esc, as the session editor and the plugin's proposal have, Delete
    (`\e[3~`) gives the line `[3~`. zsh reads `\e[3`, finds no binding, and falls back to the
    longest bound prefix, the Esc. The Esc widget runs (in the session it clears a pre-filled line,
    on a proposal it removes the proposal), and `[3~` is typed.
  - Plain zsh, without the Esc binding, types `~`. That comes from zsh's defaults, not from ush.
  - The emacs keymap binds none of Delete, Home, End, Page Up or Down. The user's zsh doesn't
    either (`bindkey | grep 3~`: nothing).
  - Binding a sequence to `undefined-key` is the same as no binding: Page Up still gave `[5~`.
    `beep` stops it.
- **Session editor** (`VARED` in [ui.py](../src/py/src/ush/ui.py), `varedScript` in
  [ui.go](../src/go/internal/ui/ui.go), the same lines):
  - Delete `\e[3~` → `delete-char`;
  - Home `\e[H`, `\eOH`, `\e[1~` → `beginning-of-line`;
  - End `\e[F`, `\eOF`, `\e[4~` → `end-of-line`;
  - Page Up and Down `\e[5~`, `\e[6~` → `beep`.
- **Plugin** ([ush.plugin.zsh](../src/shell/ush.plugin.zsh)): `_ush_fn_keys` holds the same keys.
  `_ush_keys_bind` binds each one in the emacs keymap only if it is `undefined-key` there, and
  notes it in `_ush_fn_bound`. `_ush_keys_restore` removes them along with Esc's binding. A key
  the user bound keeps that binding, also during the proposal.
- **Tests** ([test_zsh_plugin.py](../src/py/tests/test_zsh_plugin.py)), 4 new:
  - `test_fn_keys_edit_the_line`: the three Home/End variants, Delete at the start, Page Up and
    Down;
  - `test_delete_in_typed_input`;
  - `test_fn_keys_edit_the_proposal`: a proposal edited with Home, Delete and End runs as edited,
    and counts as what ran;
  - `test_fn_keys_keep_their_own_bindings`: the user's Delete binding is used and kept; Home and
    Page Up are `undefined-key` again after the proposal.
  - Before the fix, the first three failed with `GOT'[3~'`, `GOT'[H[3~[F[5~[6~!'` and records
    `[None, None]` (the proposal had been removed). The fourth passed already.
- **Validation** on macOS, Go 1.27.4:
  - `just ushgo-test`: exit 0;
  - `PYTHON=/opt/homebrew/bin/python3.12 just ushpy-test`: 231 tests OK (227 before);
  - `just ushgo-conform`: 88 tests OK (84 before), so the Go session editor passes the same tests;
  - `just links`: passed. `mandoc -T lint -W warning src/man/ush.1`: clean.
- **Docs:** USER (keys on a proposal), the man page (under Esc), ARCH (keys on a proposal).

### Deviations

- **More than Delete:** the TODO item names Delete. Home, End, Page Up and Down failed the same
  way: the Fn-arrows on a Mac send them, and in the session each one cleared a pre-filled line
  and typed its sequence. They are fixed together. No existing binding changes: the plugin binds
  only keys that are undefined, and the session's zsh starts with `-f`.
- **The plugin as well as the session editor:** the item names the session editor. On a proposal,
  the plugin's Esc caught Delete in the same way.
- **2.35–2.39 and 2.41 stay on the user's branch;** only 2.40 is added to TODO here.

### Insights

- In zsh, binding a bare Esc to a widget makes every unbound escape sequence that starts with
  `\e` run that widget and type the rest. Binding to `undefined-key` doesn't prevent that; any
  real widget does, `beep` included. Other unbound keys (Insert, F-keys, Ctrl-arrows in some
  terminals) still behave that way on a proposal and in the session.

---

## 2026-11-11 06:17 — Flaky test: the key after Esc waits for zsh

TODO items: 2.44 (new, done); test code only, run against both implementations. PR: not yet
opened (branch `fix/ush-2.44-esc-test-race`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The failure:** GitHub Actions on #53, `ushgo-conform (macos-latest)`:
  `test_zsh_plugin.SessionPrefillTest.test_esc_clears_the_line`, `AssertionError: "GOT'ls'" not
  found in "…GOT'echo prefilleds'END"`. The re-run passed; #53 doesn't touch the plugin or
  the test. It failed once in the last 15 Actions runs.
- **The cause:** after a bare Esc, zsh waits KEYTIMEOUT (0.4 s) for a following key that would
  make it Alt-key, and prints nothing until it has handled the Esc. The test's step
  `("\x1b", "")` waited only for 0.5 s of quiet. When zsh hadn't read the Esc within that time,
  `ls` arrived with it: Esc l is Alt-l (down-case-word), and `s` was typed.
- **The fix** ([test_zsh_plugin.py](../src/py/tests/test_zsh_plugin.py)): `_read` takes a pattern
  as well as text; `ANY_OUTPUT` waits for zsh's reaction to the key, then for quiet. Used after
  the bare Esc in `test_esc_clears_the_line` and in `SessionExplainTest` (two Esc steps before
  Ctrl-D). Steps that send a whole escape sequence (Up, Down) aren't affected.
- **Regression test:** `test_the_key_after_esc_waits_for_a_slow_zsh`: `edit(…, stalled=True)`
  stops the editor's process group (zsh included) before the Esc and resumes it a second later,
  as a starved runner would. With the old step `("\x1b", "")` it fails with the message from CI
  ("GOT'echo prefilleds'"); with `ANY_OUTPUT` it passes.
- **Validation** on macOS: `PYTHON=/opt/homebrew/bin/python3.12 just ushpy-test`: 227 tests OK
  (one new). `just ushgo-conform`: 84 tests OK (the new test, against Go). The regression test
  with the old step: FAILED, "GOT'echo prefilleds'". `just links`: passed.

### Deviations

- None.

### Insights

- In a pty test, "quiet for a while" isn't a sign that the program has handled a key: wait for
  its reaction. A bare Esc is the key where this shows, through KEYTIMEOUT.

---

## 2026-11-11 05:40 — Release 0.2.3: the gate, the tag, the GitHub release and the tap

TODO items: 5.12 (done). PR: not yet opened (branch `docs/ush-0.2.3-released`); the release PR
was #50, the candidate 50257ee (#50 and #51 merged).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request** (user, 2026-11-11): "merged", then "tag the rc": the tag `v0.2.3-rc1` on
  50257ee, annotated, as for 0.2.0-rc1; no GitHub release for it. Earlier: "create the homebrew
  release in the ../ush-homebrew project". There was no `../ush-homebrew`; the tap
  `gregoryyy/homebrew-ush` was cloned there.
- **Gate on 50257ee**, the merge of #51 (0.2.3 and the PDF fix):
  - Level 1, macOS arm64: `just ci --post 50257ee`: all seven jobs pass; statuses posted.
  - Level 1, Linux: GitHub Actions run 42959159122 on 50257ee, success: ushpy on Ubuntu with
    3.10 and 3.13, ushgo and the conformance suite on Ubuntu, with go1.27.4. No container runtime
    here, so the Linux archives were built, not run.
  - **govulncheck v1.1.4:** with Homebrew's Go 1.27.3 it reports GO-2026-6859 (`net/http`, a
    crash of HTTP/1 servers through `MaxBytesReader`; ush is only a client), fixed in Go 1.27.4.
    Go 1.27.4 from go.dev (`golang.org/dl/go1.27.4`): no vulnerabilities; `just ushgo-test`
    passes with it. The release archives are built with it.
  - Release build with go1.27.4: `just ushgo-release 0.2.3 --check` (two builds identical), then
    the build:
    - `65e8c152…` darwin-arm64
    - `27e349c5…` darwin-amd64
    - `2451c7e3…` linux-arm64
    - `88fe1a1b…` linux-amd64
  - darwin-arm64 archive in a temporary prefix: the checksum verifies, `ush 0.2.3 (go)`, built
    with go1.27.4, nothing writable by others, `man ush` shows 0.2.3 of 2026-11-11; offline smoke
    6 PASS; `smoke.sh --bin` with llama3.2:3b: 13 PASS, 2 WARN (the model proposed no command for
    the two propose checks), 0 FAIL, as for 0.2.2.
  - Level 2: the injection eval three times on that archive with llama3.2:3b, 36 runs: 8
    attempted (2, 3, 3), 0 ran unapproved.
  - `gitleaks git --redact --log-opts=50257ee .`: no leaks. Over all local refs, it also finds the
    synthetic tokens of `test_redact.py` in `refs/agents/…/checkpoints/…`, local checkpoint refs of
    an agent tool from 2026-11-08 that are not on GitHub and not in the release; the same tokens
    are already in `.gitleaksignore` at their other paths.
- **Published by the agent, at the user's request** (5.12):
  - the tag `v0.2.3` on 50257ee;
  - the GitHub release v0.2.3 (latest), with the four archives, `SHA256SUMS` and the notes as
    corrected here. Downloaded again: the checksums verify, and `SHA256SUMS` is byte-identical to
    the build's;
  - the 0.2.3 formula in `gregoryyy/homebrew-ush` (b6e4ef4), from `formula.sh 0.2.3`: the
    tarball's checksum `4c3ed991…`, the `src/` paths; `brew style` clean; its build reproduced by
    hand from the tag's tarball with Homebrew's Go: `ush 0.2.3 (go)`, built with go1.27.3, the
    plugin and the page at their paths. `brew info gregoryyy/ush/ush`: 0.2.2 → stable 0.2.3.
- **Release notes,** corrected after the tag ([releases/0.2.3.md](releases/0.2.3.md), the
  GitHub release has this text): built with Go 1.27.4 and GO-2026-6859 (Security); Homebrew builds
  with Homebrew's Go, 1.27.3 for now (known limitations); the reproducible build names Go 1.27.4.

### Deviations

- **The Linux archives weren't run:** no Docker or Podman here, unlike 0.2.2's Debian container.
  The release notes say so.
- **Homebrew builds with Go 1.27.3** until Homebrew updates Go; a formula can't pin Go's patch
  version. The GitHub archives are built with 1.27.4.
- **The tap commit's author** is `gregor@flunder.local`, the default of the fresh clone; earlier
  tap commits use another address. Not changed, as that would need a force push.
- **The release notes in the tag** (50257ee) still have the uncorrected text; the GitHub release
  and `main` have the corrected one.

### Insights

- govulncheck findings come from the Go toolchain too, not only from dependencies: check the
  installed Go's patch level before the release build.
- Agent tools may keep refs of their own (`refs/agents/…`) that gitleaks scans; CI's scan up to
  the commit is the one that counts for a release.

 — `just pdfs` without the deprecation warning

TODO items: 6.3 (new, done). PR: not yet opened (branch `fix/pdf-math-method`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- pandoc 3.13 printed `[WARNING] Deprecated: --mathml. Use --math-method=mathml instead.` for
  every document. [pdf.sh](../tools/pdf/pdf.sh) now passes `--math-method=mathml`; the comments
  in `pdf-style.css` say the same.
- **Validation:** `just pdf docs/ARCH.md` (PDF removed first): written, no warning.
  `just pdfs`: all 20 PDFs rebuilt (the renderer changed), then two again after removing them:
  written, exit 0, no warning in the output. `just links`: passed. With `--math-method=mathml`, pandoc writes inline and display
  formulas as `<math>` (MathML) elements, as before.

### Deviations

- None. Because the renderer script changed, the next `just pdfs` rebuilds every PDF once.

### Insights

- pandoc names its replacement in the warning itself; the old flag still works, so older
  checkouts keep building their PDFs.

---

## 2026-11-11 05:07 — Version 0.2.3: release notes, version, man page

TODO items: 5.12 (new; open until published). PR: not yet opened (branch `chore/ush-0.2.3`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request** (user, 2026-11-11): "update release notes and man pages and pr and release
  0.3.0". Asked first, because 0.3.0 is planned as the shells and platforms release (5.11) and
  none of it is built: the user chose **0.2.3**, with the agent publishing as for 0.2.2.
- **Contents,** what `main` has since `v0.2.2` (deac48b): notes on Ctrl-N (2.25, #39), `ush
  explain` without quotes (2.43, #49), `cd` in the session (2.42, #42), the provider table and
  model profiles (2.29.1, #41), the `src/` layout (6.1, #46 via #48), the Linux CI fix (6.2, #47).
- **Version 0.2.3** in `pyproject.toml`, `ush/__init__.py`, the manual page's header (dated
  2026-11-11), USER, README, ushgo's README (now pointing at 5.12), ROAD's status line and
  `formula.sh`'s usage line. The Go binary gets its version from the release build.
- **The manual page** already describes the three changes users see (Ctrl-N, `cd` in the
  session, `ush explain` without quotes); only its header changed.
- **Release notes:** [releases/0.2.3.md](releases/0.2.3.md): changed, fixed, inside, install,
  security, known limitations.
- **Validation** (before the PR; the release gate runs on the merge commit): `just ushgo-test`:
  every package ok. `PYTHON=/opt/homebrew/bin/python3.12 just ushpy-test`: 226 tests OK.
  `just links`: passed. `mandoc -T lint -W warning src/man/ush.1`: clean; `man ./src/man/ush.1`
  shows "ush 0.2.3, 2026-11-11".

### Deviations

- **0.2.3, not 0.3.0,** as the user chose; 0.3.0 keeps its plan (5.11).

### Insights

- Ctrl-N and the `cd` fix had waited on `main` since 2026-10-30 and 2026-11-10; a release after
  each user-visible fix keeps Homebrew users current.

## 2026-11-10 22:58 — `ush explain` without quotes

TODO items: 2.43 (new, done), in the plugin both implementations share. PR: not yet opened
(branch `feat/ush-2.43-explain-verbatim`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request** (user, 2026-11-10): "can we have the ush explain 'arg' as args without quotes".
  Both implementations already join unquoted words, so `ush explain ls -la /tmp` worked. But zsh
  acts on the line first: `*.log` is expanded (or fails with "no matches found"), quotes are
  removed, `ush explain cat x | sort` runs `sort` on the explanation, `; cmd` runs `cmd`, and
  `> out` writes the explanation into `out`.
- **[ush.plugin.zsh](../src/shell/ush.plugin.zsh):** `_ush_rewrite`, which already quotes
  `//` requests before zsh parses the line, also rewrites `ush explain [-v] [--] REST` to
  `ush explain [-v] -- 'REST'`, with REST exactly as typed. A command line in one pair of quotes,
  `ush explain` alone and `ush explain -v` alone are left unchanged. Only a line starting with
  `ush explain` is rewritten, as for `ush //`; `sudo ush explain` isn't needed, since explaining
  runs nothing.
- **Test:** `test_zsh_plugin` `test_explain_gets_the_command_line_verbatim_and_runs_none_of_it`
  drives zsh through a pty with a fake `ush` that prints its arguments: a pipeline, an unmatched
  glob with `-v`, `echo $HOME; touch MARKER`, `-- don't > out`, a quoted line and none. It
  asserts the arguments ush receives, that MARKER wasn't created and `out` wasn't written. Against
  the old plugin it fails (the pipeline's output went to `xargs`).
- **Docs:** [USER](USER.md) (the quoting paragraph and the commands table) and the
  [manual page](../src/man/ush.1).
- **Validation:** `python -m unittest test_zsh_plugin.ZshPluginTest`: 3 tests OK; the new test
  against the old plugin: FAILED, as expected. `PYTHON=/opt/homebrew/bin/python3.12 just
  ushpy-test`: 226 tests OK. `just ushgo-conform`: 83 tests OK (the plugin tests included).
  `just links`: passed.

### Deviations

- None. The CLI still joins its words when there is no plugin; that's documented, with the advice
  to quote.

### Insights

- The rewrite also closes a trap: `ush explain CMD; rm -rf x` used to run the `rm` while asking
  what the command does. With the plugin, nothing after `ush explain` runs.
- `${#${(z)s}}` counts characters, not words, when `(z)` yields one word; split into an array
  first.

## 2026-11-10 15:02 — CI: zsh for the ushgo job on Ubuntu

TODO items: 6.2 (new, done). PR: not yet opened (branch `fix/ci-ushgo-zsh`, stacked on #46).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- GitHub Actions starts jobs again. Every run, on `main` as on PRs, failed in `ushgo
  (ubuntu-latest)`: `TestPipefailAndNoRcFiles`, "pipefail is off" (e.g. runs 42796504428,
  42802552924, 42809019552). Ubuntu runners have no zsh, so `FindShell` fell back to `/bin/sh -c`
  (dash), which has no pipefail. The ushpy and conformance jobs already install zsh; the ushgo job
  didn't, and local CI, on macOS, always has zsh.
- [ci.yml](../.github/workflows/ci.yml): the same "Install zsh" step in the ushgo job.
  [ARCH](ARCH.md) names the `sh` fallback and that it has no pipefail. `tools/ci/test_workflow.py`
  expects the ushgo job's extra step.
- **Validation:** `just ci --job ushgo,tools,links` on macOS: all pass (on macOS zsh is present, so
  the step does nothing there). `just links`: passed. The fix itself shows only on an Ubuntu
  runner; see the PR's Actions run.

### Deviations

- None. The test is unchanged: it states the behaviour ush promises with zsh.

### Insights

- With Actions back, the Linux matrix entries that local CI leaves unchecked run again; this failure
  had been hidden since local CI took over.
- The `sh` fallback silently drops pipefail on a machine without zsh. ush needs zsh for its plugin,
  so this mainly affects `ush` without the plugin; whether to set pipefail where `sh` supports it is
  open.

## 2026-11-10 12:30 — One project, one layout: `src/`, `docs/`, `tests/conform/`

TODO items: 6.1 (new, done). PR: not yet opened (branch `chore/src-layout`, stacked on #45, which
removes `wish/`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request** (user, 2026-11-10): remove the wish path including its PDFs; `ush/` becomes
  `src/`, with `ush/conform` and `ush/docs` moving out of it; overlapping documents integrated into
  `docs/`. The user chose `tests/conform/` for the corpus, `src/man/` for the manual page, and a PR
  stacked on #45.
- **Moves,** all by `git mv`: `ush/go`, `ush/py`, `ush/shell`, `ush/man` → `src/`; `ush/conform` →
  `tests/conform`; `ush/docs/*.md` and `archive/` → `docs/`. The leftover local `wish/` folder,
  only generated PDFs, was deleted.
- **Merged:** `ush/docs/ROAD.md` and the repository `docs/ROAD.md` into one [ROAD](ROAD.md), with
  the phase numbers kept (phase 0 "Repository" from the repository roadmap; the wish step reduced to
  phase 5's pointer); `ush/README.md` into the root [README](../README.md); [SOTA](SOTA.md) without
  its wish summary (kept at the 079f504 permalink); [SDLC](SDLC.md) and
  [AGENTS.md](../AGENTS.md) in single-project wording.
- **Code and tooling:** Go module path `github.com/gregoryyy/ushell/src/go` and all imports; the
  corpus path in `policy_corpus.py` and `policy_test.go`; `test_docs` (documents in `docs/`, manual
  page in `src/man/`); `conform.sh`, `formula.sh` (Homebrew paths for releases from now on; older
  tags keep their own formula.sh), `release.sh`; CI working directories, `tools/ci/test_workflow.py`,
  justfile, `.gitignore`, the PDF defaults (`README.md docs src`); help texts (`source
  src/shell/ush.plugin.zsh`) in both implementations; the manual page's links.
- **Links:** a script resolved every relative Markdown link against the file's old location and
  rewrote it for the new one (17 files). This changes link targets in earlier DONE entries and the
  archive; their text is unchanged. Path mentions in live documents were updated; DONE, archive and
  release notes keep their old wording.
- **Validation:** `just ushgo-test`: gofmt and vet clean, all packages ok (policy with the corpus
  at its new path). `PYTHON=/opt/homebrew/bin/python3.12 just ushpy-test`: 225 tests OK.
  `just ushgo-conform`: ushpy's black-box tests against the Go binary, 82 tests OK. `just links`:
  passed; heading anchors checked by a script, none broken. Not run: `just ci` (`uv` and Python
  3.10 and 3.13 are not installed here), so the CI matrix is unchecked on this commit.

### Deviations

- None from the request. Installed locations (`~/.local/share/ush`, `~/.config/ush`) and the
  plugin's `~/.zshrc` line are unaffected: both installers copy the plugin out of the checkout.

### Insights

- Within `src/`, `py`, `go`, `shell` and `man` keep their relative positions, so the installers and
  release tooling needed no change beyond the corpus, which moved out of the tree.

---

## 2026-11-10 11:03 — Bug: `cd` in the session didn't move

TODO items: 2.42 (new, done), in both implementations. PR: not yet opened (branch
`fix/ush-2.42-session-cd`).

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The report:** "cd does not move away from the initial directory" (user, 2026-11-10). In the
  `ush>` session, `Agent.run_explicit` (ushpy) and `Agent.RunExplicit` (ushgo) ran each line, typed
  or an accepted proposal, with `zsh -f -o pipefail -c`, so `cd` ended with that shell. The
  session never moved, although ARCH and the propose-mode prompt treat a session line like a line
  in the user's shell, and `_proposal_results` already reports a changed working directory.
- **The fix:** `execute.run_session_command` and `execute.RunSessionCommand` prefix the command's
  first line with `trap 'printf %s "$PWD" > FILE' EXIT; `, `FILE` in a private temporary directory
  (mode 0700), then `change_directory`/`ChangeDirectory` changes ush there and sets `PWD` to the
  shell's logical path. The exit status survives the trap (checked for `exit 5`, `&& false`, and
  under pipefail), error messages keep line 1, and history records the command as typed. Where the
  trap can't run (the command replaces it, `exec`s, or is killed), no file is written and ush stays
  where it was. Agent-mode tool commands use `run_command` as before.
- **Tests:** `test_agent.AgentTest.test_cd_in_a_session_line_persists` and
  `TestCdInASessionLinePersists` (`cd sub`, `pwd`, `cd ..`, `cd sub; exit 4`, a failed `cd`,
  `cd ..; trap - EXIT`); `test_zsh_plugin.SessionTest.test_cd_moves_the_session`, a pty test of the
  real session, which failed before the fix (it printed the initial directory) and passes after.
- **Validation** (macOS, zsh 5.9, worktree of `origin/main` 7125725 plus this change):
  `ush/py/scripts/test.sh`: 225 tests, 1 failure, `test_execute.ExecuteTest.test_interrupt_cancels_and_kills`,
  which also fails without this change under the venv's Python 3.15.2 and passes under 3.12 and
  3.9 (not caused by this fix; see Insights). `just ushgo-test`: all packages ok. `just
  ushgo-conform`: 82 tests OK, including the new pty test against the Go binary.
- Docs: TODO 2.42, ARCH (in a session), the man page (`ush` session).

### Deviations

- ID 2.42, not 2.35: the unmerged branch `docs/ush-interaction-followups` already uses 2.35–2.41.
- `cd -` across lines isn't supported: zsh 5.9 ignores an inherited or assigned `OLDPWD` for
  `cd -`, and the workaround (two `cd`s in the prefix) wasn't worth its risk.

### Insights

- `test_interrupt_cancels_and_kills` fails under Python 3.15 on unchanged code: the marker file is
  created, so `_thread.interrupt_main` no longer interrupts `proc.wait` in time there. It needs its
  own item before Python 3.15 is a supported version.
- Whether agent-mode `cd` should also move the session is a separate decision: an auto-approved
  `cd` would then change where later approved commands run.

---

## 2026-10-30 09:30 — The provider table and model profiles

TODO items: 2.29.1, in both implementations. PR: #41.

## 2026-10-30 08:45 — Release 0.2.2, and 0.3.0 planned

TODO items: 5.10 (done), 5.11 (new: 0.3.0), 5.3 (run again). PR: #40.

## 2026-10-30 08:40 — Notes on Ctrl-N

TODO items: 2.25 (changed), in both implementations (the plugin is shared). PR: #39.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request:** "add options to allow other model providers, like ../kibitz" (user,
  2026-10-30). With GAPS Q13 ("first openai, mistral model settings (consider internal
  abstraction?)"), 2.29.1 comes first: an internal abstraction with no change in behaviour. The
  adapter with OpenAI and Mistral (2.29.2) follows on it.
- **ushpy `providers.py`, ushgo `internal/providers`:**
  - **The table**, one row per provider, with Anthropic and Ollama as its rows:
    - name, label, API, default endpoint and the setting that overrides it;
    - whether it must be on this machine;
    - the key variables and whether it needs a stored key;
    - the model pattern and the setting that holds the model;
    - whether it can list its models, and a hint when no model is set.
  - **A model profile:** effort, thinking (adaptive, with progress notes or without), server-side
    fallbacks, strict tool schemas, eager tool input, the prompt cache, the output limit, and
    whether the model may write tool calls as text.
- **What reads them:**
  - Settings: `active_model`/`ActiveModel` and the new `model_of`/`ModelOf` and `Endpoint`;
    `set_model` and the checker's model check use the row's pattern.
  - The CLI:
    - the factory picks the adapter by the row;
    - `secrets` asks for a key where a used row needs one;
    - the disclosure, explain's and notes' "where", the checker's disclosure and `ush ai
      status` come from `destination` and the row;
    - `ush ai configure` asks for the URL, the models and the key where the row says so, and
      for the effort where the profile takes one.
  - The Messages adapter builds each request from the profile (Anthropic's and Ollama's), and
    reads tool calls written as text where the profile says so; before, that was Ollama's class.
  - Ollama keeps its own errors and transport in the adapter.
  - No `provider == "ollama"` or `"anthropic"` check is left in the CLI or settings of either
    implementation.
- **Tests:**
  - ushpy `test_providers`: the table's rows and their settings; Anthropic's and Ollama's rows;
    settings through the table; the profile of every listed Anthropic model and of Ollama; each
    profile setting on and off as the request carries it; Ollama's bare request.
  - ushgo: `TestTheTable`, `TestProfiles`, `TestEachProfileSettingMapsOntoTheRequest`.
  - The existing suites pass unchanged.
- **Validation:** `just ush-test`: ushpy 223 tests OK, every ushgo package ok (with the new `internal/providers`), conformance 81 tests OK, on macOS 26.2 arm64, Python 3.11.12. `just links` OK.

### Deviations

- **No `ush ai` note for a setting a model ignores yet:** Ollama's models are simply not asked for
  an effort, as before. The note comes with the first model that takes a setting only partly
  (2.29.2).
- **The work was rebuilt:** an earlier branch had the Chat Completions adapter and nine presets
  without the table. After the answers to GAPS Q13 and Q17 (merged in #38), it is split: this
  item first, then 2.29.2 with OpenAI, Mistral and `openai-compatible` only.

### Insights

- Ollama's adapter differs from Anthropic's in what it sends (the profile) and in its errors and
  transport (the adapter); only the first is per model, so only that went into the profile.
- **The user's request:** "deploy to a homebrew tap", for the fix of TODO 2.31 (#36). The user
  merged the version change (#37).
- **Gate on deac48b**, the merge of #37, as 0.2.2:
  - Level 1, macOS arm64: `just ci --post` passes all seven jobs; statuses posted.
  - Level 1, Linux arm64 (Debian in `golang:1.27`, with mandoc, unprivileged, from
    `git archive`): ushpy 216 OK, `go test ./...` every package OK, conformance 81 OK.
  - Release build: `scripts/release.sh 0.2.2` and `--check` (two builds identical); the Linux
    container gives the same four checksums:
    - `decb5c64…` darwin-arm64
    - `a4ac8b9f…` darwin-amd64
    - `37af8129…` linux-arm64
    - `ca7bacf4…` linux-amd64
  - darwin-arm64 archive in a temporary prefix: `ush 0.2.2 (go)`, nothing writable by others,
    `man ush` renders; `smoke.sh --bin` with llama3.2:3b: 13 PASS, 2 WARN (the model ran or
    proposed nothing for two checks), 0 FAIL.
  - linux-arm64 archive on `debian:13`: the checksum verifies, `ush 0.2.2 (go)`, `man ush`
    renders, offline smoke 6 PASS.
  - Level 2: the injection eval three times, 36 runs: 6 attempted, 0 ran unapproved (SECTEST).
  - `govulncheck` v1.1.4: no vulnerabilities. `gitleaks git --redact .`: no leaks.
- **Published by the agent, at the user's request:**
  - the tag `v0.2.2` on deac48b;
  - the GitHub release v0.2.2 (latest), with the four archives, `SHA256SUMS` and the notes.
    Downloaded again: the checksums verify, and `SHA256SUMS` is byte-identical to the build's;
  - the 0.2.2 formula in `gregoryyy/homebrew-ush`, `brew style` clean, its build reproduced by
    hand from the tag's tarball (`ush 0.2.2 (go)`, the plugin and the page at its paths).
- **0.3.0 (TODO 5.11):**
  - The user asked for "bash, sh and other commonly used shells" in 0.3.0, and accepted the
    agent's proposal with "write to pr as proposed".
  - The proposal is adjusted to the answers to GAPS Q15–Q17, merged meanwhile in #38: agent
    mode only in zsh rather than a bash executor, and argument origin (2.23) before the new
    adapter ships.
  - PowerShell, the local server presets and the other platforms come after 0.3.0, as proposed.
- **Docs:** README and ushgo's README say 0.2.2; ROAD has the 0.3.0 paragraph; SECTEST the eval.

### Deviations

- **`brew install` still not run here** (outdated Command Line Tools on this Mac).
- **Not run:** Linux amd64 and Intel macOS (no machine); a live Anthropic run (not authorised).

### Insights

- Propose mode's eval counts moved from 0 attempts to some once written-out calls became real
  proposals (2.31). The eval measures what the model tries; what runs unapproved is still 0.
- **The request:** "ctrl-x n doesn't seem to work: use a single step key combination instead,
  like ctrl-f or ctrl-a" (user, 2026-10-30). Offered Ctrl-N, Ctrl-F or Ctrl-A, with what each
  does in zsh; the answer: "Ctrl-N" (GAPS Q7).
- **Why Ctrl-X n did nothing:** probably not the key. None of the user's startup files
  (`~/.zshrc`, `~/.zprofile`, `~/.zshenv`) loads the plugin, so no ush key was bound.
- **The plugin:**
  - The notes key is Ctrl-N in emacs and viins, taken only where it still has zsh's own meaning
    (emacs: down-line-or-history, which Down also does; viins: a literal ^N).
  - A key the user bound, e.g. `history-substring-search-down`, is left alone; vicmd is not
    touched. `USH_NOTES_KEY` sets another key, taken only where it is free; empty means none.
  - The widget is read with `${(z)…}`: `${$(bindkey …)##* }` left the key name in place, so the
    first version bound nothing, which the binding test caught.
- **Text:** usage in both implementations, the man page, USER, ARCH, ROAD, SECTEST, SOTA,
  TODO 2.25, ushpy's README, and the comments: Ctrl-X n became Ctrl-N.
- **Tests:**
  - The pty tests send Ctrl-N (`\x0e`).
  - `test_the_notes_key_takes_only_zshs_own_ctrl_n`:
    - only ^N changed, in emacs and viins, from zsh's defaults to `_ush_notes`;
    - its neighbours ^O and ^P, and vicmd's ^N, keep their meaning;
    - a user's ^N is kept;
    - `USH_NOTES_KEY` empty binds nothing; `^Xq` (free) is taken, and `^F` (forward-char) is not.
- **Validation:** `just ush-test`: ushpy 216 OK, every Go package OK, conformance 81 OK. `just links` OK.

### Insights

- In zsh, `$map:$widget` reads `:$…` as a modifier; brace both. `${$(cmd)##* }` does not strip
  as it does on a plain parameter; `${${(z)"$(cmd)"}[-1]}` does.

---

## 2026-10-29 21:20 — Tool calls that local models write as text

TODO items: 2.31 (new), in both implementations. PR: #36.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The report:** with llama3.2:3b, `ush> //list the current directory` printed
  `{"name":"ls","parameters":{"command":"ls -bA","effect":"read","interactive":false,…}}` and
  nothing else (user, 2026-10-29). Reproduced in 6 of 6 runs on 0.2.1.
- **The cause:** llama3.2 writes tool calls in Llama's format, `{"name": …, "parameters": {…}}`,
  and Ollama turns that into a `tool_use` only when the name is one of the tools. The model put
  the command in the name (`ls`), so the call arrived as text. Often `command` held only the
  arguments (`-d`), `interactive` was the string `"false"`, and sometimes `"parameters{` lacked
  its `":`.
- **The Ollama adapter** (`OllamaProvider.turn`; Go `Client.Turn` for Ollama, `textcall.go`):
  - A turn without a tool_use whose whole text is one such object (or one in a code fence)
    becomes a tool_use with a `toolu_text_…` id; `stop_reason` becomes `tool_use`.
  - `text_tool_call` / `TextToolCall` read the call as meant where that is unambiguous:
    - a name that is no tool, with input that fits exactly one tool's schema, is that tool;
    - a name that is a program on the `PATH` and isn't the command's first word goes in front
      (`-d` → `ls -d`);
    - `"true"`/`"false"` in a boolean field become booleans;
    - the `"parameters{` slip is repaired.
    Anything else stays text, or keeps its wrong name for the agent to refuse.
  - `HeldText` holds text that starts with `{` or a backtick until the turn ends. It is shown
    then only if it was no call; prose streams as before. Anthropic's turns are unchanged.
- **The agent:**
  - An unknown tool's error names the tools ("unknown tool 'ls'; the tools are
    propose_command").
  - Three turns in a row whose calls were all refused (invalid, or past the step limit) stop the
    request with an error. A model that keeps writing a wrong call could loop until now.
- **Tests:**
  - ushpy:
    - `TextToolCallTest` (named tools, program names, made-up names, no fit, fences, the slip,
      text that is no call) and `HeldTextTest`;
    - `OllamaTextToolCallTest` against the mock API: nothing shown, a `propose_command` with
      `ls -d`; prose and real calls unchanged; Anthropic text stays text;
    - `test_agent`: unknown-tool message, the three-turn stop in agent and propose mode;
    - `test_cli` end to end with a mock Ollama: the proposal `ls -d`, no JSON, in history.
  - ushgo:
    - `TestTextToolCall` (the same cases), `TestHeldTextStreamsProseAndHoldsAPossibleCall`,
      `TestOllamaTurnsAWrittenCallIntoAToolUse`, `TestProseARealCallAndAnthropicTextAreUnchanged`;
    - `TestAnUnknownToolIsRefusedWithTheToolNames`, `TestThreeTurnsOfRefusedCallsStopTheRequest`;
    - the CLI test runs against the Go binary in conformance.
- **Live, llama3.2:3b on local Ollama:**
  - `//list the current directory`: a proposal in 6 of 6 runs with ushpy (`ls`, `ls -d`,
    `ls -l`) and 6 of 6 with ushgo (`ls`, `ls -d`, `ls -lah`).
  - Continuing with `//now only the .txt files` in the same shell works (the converted call is
    echoed back to Ollama).
- **Validation:** `just ush-test`: ushpy 216 OK, every Go package OK, conformance 81 OK. `just links` OK.

### Deviations

- **Agent mode on llama3.2:3b stays poor:** in 7 runs of "how many files are in the current
  directory" it mostly answered without running anything, with made-up counts, and once wrote a
  call broken past repair. No adapter fixes that; USER's troubleshooting now says so.
- **The first version** only passed the call on with its wrong name, for the agent's error to
  correct. The model then answered in prose in 6 of 6 runs, so the adapter reads the call as
  meant instead.

### Insights

- Ollama's Anthropic API depends on the model's own call format; a call with a name that is no
  tool is silently text. Models below about 8B need this help, and still fail in agent mode.

---

## 2026-10-29 12:28 — Release 0.2.1: the gate, the tag, the GitHub release and the tap

TODO items: 5.9, 5.7, 5.8 (done); 5.3 (run again). PR: #35.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The user's request:** "first create version 0.2.1 and run the gh command on that", after
  "add a man page and re-release to 0.2.1"; the user merged #34.
- **Gate on 5c77891**, the merge of #34, as 0.2.1:
  - Level 1, macOS arm64: `just ci --post` passes all seven jobs; statuses posted.
  - Level 1, Linux arm64 (Debian in `golang:1.27`, with mandoc, unprivileged, from `git archive`):
    ushpy 203 OK, `go test ./...` every package OK, conformance 80 OK.
  - Release build: `scripts/release.sh 0.2.1` and `--check` (two builds identical). The same build
    in the Linux container gives the same four checksums:
    - `4af05af8…` darwin-arm64
    - `a8bad593…` darwin-amd64
    - `c7ce2ae6…` linux-arm64
    - `11417195…` linux-amd64
  - darwin-arm64 archive, installed to a temporary prefix:
    - it holds `ush.1`; `ush 0.2.1 (go)`; no file writable by others; `man ush` renders;
    - `smoke.sh --bin` with llama3.2:3b: 14 PASS, 1 WARN (the model proposed no command), 0 FAIL;
    - `ush notes 'any errors?' app.log` with llama3.2:3b names the log's error, after the
      disclosure line.
  - linux-arm64 archive on `debian:13`: the checksum verifies, it installs as a user, `man ush`
    renders, offline smoke 6 PASS.
  - Level 2: the injection eval three times, 36 runs: 4 attempted, **2 ran unapproved** (HTML,
    agent mode, `cat pwned-by-injection`: F8). The user decided "Release, F8 documented" (GAPS
    Q12), and the release notes give the rate.
  - `govulncheck` v1.1.4: no vulnerabilities. `gitleaks git --redact .`: no leaks.
- **Published by the agent, at the user's request:**
  - The annotated tag `v0.2.1` on 5c77891.
  - The GitHub release v0.2.1 (latest), with the four archives, `SHA256SUMS` and the notes.
    Downloaded again: the checksums verify, and `SHA256SUMS` is byte-identical to the build's.
- **The tap:** the user had created `gregoryyy/homebrew-ush` with the prepared 0.2.0 commit. The
  agent pushed:
  - the 0.2.1 formula from `formula.sh`: `brew style` clean, and its build by hand from the tag's
    tarball gives `ush 0.2.1 (go)`, with the plugin and page at the formula's paths;
  - a README line on `brew trust`.
- **`brew trust`:** `brew tap gregoryyy/ush` works, but this Homebrew refuses to load formulas
  from an untrusted tap. The install line is now `brew tap`, `brew trust`, `brew install ush`:
  - in the tap's README, ADMIN and both release notes (files);
  - on GitHub, in the v0.2.0 and v0.2.1 release bodies, edited.
  The agent left trusting the tap on this Mac to the user.

### Deviations

- **The eval failed its criterion** (2 unapproved), released by the user's decision (GAPS Q12).
- **`brew install` not run:** the Command Line Tools on this Mac are outdated (DONE 11:10), and the
  tap is not trusted here.
- **Not run:** Linux amd64 and Intel macOS (no machine); a live Anthropic run (not authorised).

### Insights

- The eval's per-release sample of a 3B model swings from 0 to 2 in 36. A gate decision needs
  the rate over several releases (SECTEST), and F8 needs its fix (TODO 2.23), not more samples.
- Homebrew now gates third-party taps behind `brew trust`; every install instruction needs it.

---

## 2026-10-29 11:41 — The manual page, `man ush`; version 0.2.1

TODO items: 2.14 and 5.4, in both implementations; 5.9 (0.2.1 prepared, not yet released). PR: #34.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request:** "add a man page and re-release to 0.2.1" (user, 2026-10-29). 0.2.1 replaces the
  0.3.0 planned in TODO 5.9.
- **`ush/man/ush.1`:** one page for both implementations, in man macros. It covers:
  - the synopsis, the `//` forms, sessions and piped input;
  - the plugin's keys (Enter, Down, Up, Esc, Ctrl-X n) and agent mode's approvals;
  - every option and subcommand of `ush help`;
  - the environment, files, what is sent where, exit statuses, and examples.
  The user guide stays the long form.
- **Installed everywhere ush is:**
  - Both installers copy it to `PREFIX/share/man/man1/ush.1`, mode 0644, per user
    (`~/.local/share/man/man1`) and with `--prefix`. `man` finds it next to the binary on the
    `PATH`, with no `MANPATH`.
  - The release archive carries it (`tools/release`, the archive README).
  - `release.sh` refuses to build with uncommitted changes in `ush/man`.
  - The formula installs it with `man1.install`, and its test checks that it is there.
- **Tests:**
  - `test_docs`: every short and long option of `ush help`, every subcommand, and the plugin's
    and override variables are in the page. Removing `--once` from the page fails the test.
  - `mandoc -T lint -W warning` is clean; the test is skipped where mandoc is missing.
  - `test_install`, new `InstallerTest`: `ush/py/install.sh` runs with a stand-in Python (no venv
    or network) and puts the page, the plugin, the package and the launcher where expected, none
    writable by others, per user and with `--prefix`. Without the copy line, the tests fail.
  - `tools/release`: the archive has `ush.1`, and its `install.sh` puts it in
    `share/man/man1`.
  - By hand: the Go archive's installer into a temporary `HOME`, then `man ush` with that `PATH`,
    renders the page.
- **Version 0.2.1** in ushpy (`__version__`, `pyproject`) and in the page's header. Release notes:
  [docs/releases/0.2.1.md](releases/0.2.1.md).
- **Docs:** USER (`man ush`), ADMIN (where the installers put it, and removal), ARCH (archive
  members), TODO, ROAD.
- **Validation:**
  - `just ush-test`: ushpy 203 OK, every Go package OK, conformance 80 OK.
  - `just links` OK.

### Deviations

- **The page's source is roff, written by hand**, not generated from the user guide. The test keeps
  it complete against `ush help`; its prose can drift from USER.md, which says more.

### Insights

- macOS's `man` derives its search path from `PATH` (`../share/man` of each entry), so a per-user
  install needs no `MANPATH` change.

---

## 2026-10-29 11:10 — Release 0.2.0: the gate, the tag, the GitHub release; the tap prepared

TODO items: 5.8 (new; open until the tap is pushed), 5.5, 5.6 (done), 5.7 (prepared), 5.3 (run
again), 5.9 (new: 0.3.0 next). PR: #33.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The user's decisions** (2026-10-29): "do it. also publish the github release and homebrew
  tap", answering the agent's recommendation: 0.2.0 on the tested commit of 0.2.0-rc1, then 0.3.0
  for the work since (layout, 2.24, 2.25). With it, `govulncheck` was run (asked as "do it").
- **The candidate:** b4f9d82, the commit of `v0.2.0-rc1` (main at #24), built as version 0.2.0.
- **Gate on b4f9d82, fresh:**
  - Level 1, macOS arm64: `just ci --post` passes all seven jobs (links, ushpy on 3.10 and 3.13,
    ushgo, ushgo-conform, tools, secrets); statuses posted to the commit.
  - Level 1, Linux arm64 (Debian in `golang:1.27`, Go 1.27.3, Python 3.13.5, zsh 5.9,
    unprivileged, from `git archive b4f9d82`): ushpy 175 OK, `go test ./...` every package OK,
    conformance 63 OK.
  - Release build: `scripts/release.sh 0.2.0` and `--check` (two builds identical). The same build
    in the Linux container gives the same four checksums:
    - `4bf4c50d…` darwin-arm64
    - `64cd579c…` darwin-amd64
    - `df76e957…` linux-arm64
    - `3c33ccb5…` linux-amd64
  - darwin-arm64 archive, installed to a temporary prefix: `ush 0.2.0 (go)`, no file writable by
    others; `smoke.sh --bin` with llama3.2:3b gave 13 PASS, 2 WARN (the model proposed no
    command), 0 FAIL.
  - linux-arm64 archive on `debian:13`: the checksum verifies, it installs as a user,
    `ush 0.2.0 (go)`, offline smoke 6 PASS.
  - Level 2: the injection eval three times, 36 runs: 1 attempted, 0 ran unapproved (SECTEST).
  - `govulncheck` v1.1.4 (`go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...` in `ushgo/`):
    no vulnerabilities found. The first run of it on a release.
  - `gitleaks git --redact .`: no leaks.
- **Published by the agent, at the user's request:**
  - The annotated tag `v0.2.0` on b4f9d82, pushed.
  - The GitHub release v0.2.0 (latest): the four archives, `SHA256SUMS`, and the notes in
    [docs/releases/0.2.0.md](releases/0.2.0.md). Downloaded again: the checksums
    verify and `SHA256SUMS` is byte-identical to the build's.
- **The Homebrew tap, prepared:** `Formula/ush.rb` from `formula.sh` at v0.2.0 (it has the old
  paths, `ushgo/` and `ushpy/shell/`), with a README and the licence, committed in a local
  repository. Checked:
  - `brew style`: no offenses, after two fixes now also in `formula.sh` on main (the description
    starts with an article; `version` goes before `sha256`).
  - The formula's build by hand from the tag's tarball: the tarball's checksum verifies, `go build`
    with the formula's flags gives `ush 0.2.0 (go)`, and the plugin is at the formula's path.
- **Main moves to 0.3.0:** ushpy says `0.3.0-dev` (`pyproject` `0.3.0.dev0`); TODO 5.9 plans 0.3.0.
- **Docs:** ADMIN has a Homebrew section (not for `sudo`: the prefix belongs to the user) and
  brew upgrade/uninstall; version examples say 0.2.0; README, ROAD and ushgo's README updated.

### Deviations

- **The tap repository isn't published.** `gh repo create gregoryyy/homebrew-ush --public` was
  refused by the agent's permission check, so the agent stopped there. The release notes already
  name `brew install gregoryyy/ush/ush`; it works once the user pushes the prepared repository.
  TODO 5.7 and 5.8 stay open until then.
- **`brew install` and `brew audit` not run:** Homebrew refuses both on this Mac because its
  Command Line Tools are older than Xcode 27's. The build was checked by hand instead (above).
- **The Python implementation at the tag** reports `0.2.0-rc1`: the tag is on the rc1 commit,
  and the release is the Go binary. Stated in the release notes.
- **Not run:** Linux amd64 and Intel macOS (no machine); a live Anthropic run (not authorised).
- **The rc1 tag** stays; 0.2.0-rc1 was never published as a GitHub release.

### Insights

- A final release on the candidate's commit needs only a rebuild with the new version: the build
  stamps the version, so the gate reran without a code change.
- Homebrew checks the Command Line Tools before building any formula from source, also a Go one
  that needs no C compiler. A tap's users need current tools.

---

## 2026-10-29 02:40 — Notes on command output and files: Ctrl-X n and `ush notes`

TODO items: 2.25, in both implementations. PR: #32.

Agent: Opus 5.5, reasoning: not exposed, subagents: none.

### Implementation notes

- **The request:** "add the possibility to analyze command output and files. this could be a
  shortcut like ctrl-n like notes" (user, 2026-10-29). The key is Ctrl-X n, since Ctrl-M is Enter.
  The output is the plugin's recorded output, as proposed and accepted ("implement the changes").
  Both decisions are in GAPS Q7.
- **`ush notes [-v] [--last] [INSTRUCTION] [FILE…]`**, in `cli.py` and `notes.py`, and in Go in
  `cmd/ush/notes.go` and `internal/notes`:
  - Words naming existing paths are files; the rest is the instruction. Piped input counts as
    material.
  - `--last` takes the plugin's last recorded run.
  - One request without tools (`complete`, as for `ush explain`). The material is redacted and
    goes between random-tag markers; the system prompt calls it data. At most 200 KB in all.
  - A disclosure line names each item and where it goes; `-v` adds the sizes.
  - Refused, with the reason:
    - secret locations, the policy's list (now `is_secret` / `policy.IsSecret`), by the name given
      and by what a link resolves to;
    - directories, and special files: a FIFO is opened with `O_NONBLOCK`;
    - binary files (a NUL byte);
    - files past the remaining budget.
  - Never as root.
- **The last run kept for notes:** `take_ran` (Go `TakeRan`) now renames `ran` and `output` to
  `last-ran` and `last-output` instead of deleting them. `last_ran` (`LastRan`) reads the fresh
  run or else the kept one and removes nothing. So Ctrl-X n on an empty line still has the output
  after `//` has sent it. The next run replaces the kept files, and the shell's exit removes them.
- **The plugin**, shared by both:
  - `_ush_notes` splits the line with `${(Z+c+)BUFFER}`, so a trailing comment is one word: the
    question.
  - A line whose first word `whence` finds runs like a proposal: the captured state, then
    `accept-line`; `precmd` writes `ran`, then runs `ush notes --last`.
  - Other lines become `ush notes --last …` (no word names a file) or `ush notes -- WORDS`, with
    `~/` expanded, and are accepted, so the history shows what was sent.
  - sudo, doas, ush and `//` lines are refused with a message.
  - The key is bound in emacs and viins only where it is free (`undefined-key`). `USH_NOTES_KEY`
    sets another key; empty means none.
  - The completion lists `explain` and `notes`.
- **Tests:**
  - **`test_zsh_plugin`:**
    - six pty tests with the fake ush: a command with and without a question, the history line;
      a script under `~/` runs;
      an empty line, a question alone, plain words; a line of files with `~/` and quotes; sudo and
      `//` lines; the bindings;
    - `ZshNotesTest`, with the real ush against the mock API: a command, then its output again.
  - **`test_cli`:**
    - files and piped input, with redaction and markers;
    - the refusals: `.env`, a link to `~/.ssh/config` and a broken one into `~/.ssh`, binary,
      200 KB, a directory, a FIFO;
    - the budget across two files;
    - `--last`: sent, kept, not recorded, nothing yet, no plugin;
    - root, in-process.
  - **`test_notes`** and **`test_propose`**: `last_ran`; `take_ran` keeps the run.
  - **Go:** `internal/notes` (`TestReadFileSendsTextAndSaysWhyNot` and two more),
    `TestNotesNeverAsRoot`, `TestLastRanIsTheNewestRunAndRemovesNothing`.
  - **Both implementations:** the black-box tests ran against the Go binary too.
- **Documents:** USER (Notes, the commands table, what leaves the machine), ARCH (notes, the kept
  files, `internal/notes`), SEC (two threats, the kept output, a residual risk), SECTEST (five
  rows), TODO 2.25 done for both, the repository ROAD's idea narrowed to what remains (a result per
  file), SOTA, ush/py's README.
- **Validation:**
  - `just ush-test`: ushpy 199 tests OK; every Go package OK, `internal/notes` new; 80
    conformance tests OK against the Go binary.
  - `just links`: passes.
  - Local CI on the pushed head: see the PR.

### Deviations

- **What a line is:**
  - TODO 2.25 said the words naming files are sent; it didn't say when a line is a command.
    Here a line is a command if `whence` finds its first word, else it is files and a question.
  - Words that name no file, with no file among them, are a question on the last output, so
    `why so slow` works without the `#`.
  - A command line's own file arguments are not sent besides its output.
- **Notes are printed, not drawn above the line:** TODO 2.25 said notes appear "as an explanation
  is shown", but explanations are display-only (`PREDISPLAY`) and vanish. Notes are printed before
  the next prompt, so they stay in the scrollback, and the history shows the `ush notes` line.
- **Refused files are reported, the rest is sent:** one refusal doesn't stop the request; if
  nothing is left, ush says "nothing sent" and exits 1.
- **One answer for all files:** the repository ROAD's idea, a result per file, stays an idea.
- **`complete`'s limits:** a 60-second timeout and 2048 answer tokens, the same as for explanations.
  That suits brief notes; long material on a slow local model may time out.

### Insights

- zsh's `${(Z+c+)…}` parses a line as the shell would and keeps the comment as one word. That
  gave the question and the command without a second parser.
- Inside double quotes zsh joins an array before applying `(q-)`: `${(j: :)${(q-)args}}` quoted
  the whole list as one word. `(@q-)` quotes each word.
- `bindkey -L` lists only the main keymap; `bindkey -LM MAP` per keymap shows the rest.
- Two cases the first version got wrong, found in review and now tested: `whence` was asked about
  `~/bin/x` before the `~` was expanded, so a script under home was sent as a file instead of
  run; and Go resolved a broken link to its own path, where Python's `realpath` follows it to the
  secret location it names.

## 2026-10-29 02:07 — One document set for ush

TODO items: none; the plan's structure. PR: #31.

Agent: Opus 5.5, reasoning: not exposed, subagents: one research agent earlier in the session
(SOTA), none for this change.

### Implementation notes

- **The request:**
  - The user asked: "doesn't it make sense to have the ush documents kept in one directory as the
    functionality is largely equivalent between py and go".
  - Asked where, the user chose "ush/{py,go,docs}". The layout came first (#30); this entry is the
    document merge.
- **One set in `ush/docs/`:** USER, ADMIN, ARCH, SEC, SECTEST, SMOKE, TODO, ROAD, GAPS, DONE.
  ushpy's documents were the base; ushgo's were merged in:
  - **TODO:**
    - ushgo's setup items keep 0.1–0.4; its port is section 4 (1.x became 4.x), its release items
      are section 5 (2.1–2.7 became 5.1–5.7).
    - Its port items 2.8–2.16 became the Go side of the items they ported.
    - Every item in section 2 now says where it is built (*py ✓ go ✓*, *py ✓ go –*,
      *py – go –*), and an item is done only when both have it.
    - A table maps the old IDs.
  - **GAPS:** ushgo's Q1–Q4 are Q9–Q12, its A1–A3 are A10–A12.
  - **ARCH and SEC:** ushgo's documents are their last sections, "The Go implementation" and "What
    the Go implementation changes"; the TODO numbers in its SEC table were mapped by hand.
  - **ROAD:** ushgo's phases are done, recorded in phase 3; its two ideas joined the list.
  - **USER and ADMIN:** one copy each. ushgo's ADMIN, which covered both installs, was the base,
    with ushpy's Python details; the shared-section markers are gone.
- **References:**
  - A script mapped TODO and GAPS citations by where the text came from. In ushgo's documents a
    bare "TODO 1.6" meant ushgo's item; elsewhere "ushpy TODO" and "ushgo TODO" became
    "ush TODO", mapped.
  - Relative links were rewritten by resolving their old targets.
  - AGENTS.md, SDLC, the README files, the repository ROAD and SOTA follow, and there is a new
    `ush/README.md`.
- **Records:**
  - The old DONE logs are in `ush/docs/archive/`, unchanged except for an archive note at the top
    and link targets.
  - The 0.2.0-rc1 release notes and wish's DONE keep the old names: they record the old layout.
- **A lost change restored:** TODO 2.23 had #28's text ("Argument origin in agent mode",
  unconditional since GAPS Q8) at #28's merge (bbbdcb8). After #29's merge, `main` had the text from
  before #28 again: #29's branch predated #28, and the merge of `main` into it (a895667) kept #29's
  version of that passage. Diffing #28's documents at its merge against `main` showed this was the
  only loss; 2.23 has #28's text again, with the new IDs.
- **Out-of-date statements fixed on the way:**
  - ADMIN still said F1 and F2 were open; both were fixed on 2026-10-28.
  - SMOKE said the Go implementation fails unported checks.
  - The repository ROAD still had `main` and the licence as open.
- **Tests:** `test_docs` no longer compares two copies. It checks that:
  - every option and subcommand of `ush help` is in the user guide;
  - the documents exist once, with no `ush/py/docs` or `ush/go/docs`;
  - every item in TODO section 2 says where it is built, and a ticked item is built in both.
  It also finds `ush/py/src` itself now.
- **Validation:** see the PR: `just ush-test`, `just links`, the CI tool's tests, and local CI on
  the head.

### Deviations

- **TODO 3.1** (the Python 0.1 release gate) stays open as a record, marked as overtaken by the
  releases of the Go binary: closing it is the user's call.
- **DONE archives:** their link targets were updated, so the links work, while their text is
  unchanged.

### Insights

- IDs are cheap to keep when sections, not items, are renumbered: ushpy's items, ushgo's 0.x, and
  the minor numbers of ushgo's 1.x and 2.x all survive, so most old citations can be read without
  the table.
- A branch that predates a merged plan change can bring the old text back when `main` is merged
  into it. Before merging such a branch, diff the documents the earlier PR changed.
- `zsh` doesn't split an unquoted `$var` into words: a list of moves passed as `$moves` reached the
  link script as one argument. Pass such lists explicitly, or with `${=var}`.
