# DONE

Progress log for [TODO](TODO.md), for both implementations of ush, newest first. Each entry starts
with the date and time of day (local time) and a title, then the TODO items and the PR, then the
agent that did the work as `Agent: <model>, reasoning: <level>, subagents: <none or which>.` (or
`Agent: human`), then:
- **Implementation notes:** what was built and where, with the exact validation and its results.
- **Deviations:** what differs from the plan, and why.
- **Insights:** what we learned that matters for later work.

Entries are never rewritten; corrections and later outcomes (merges, releases) are new entries on
top. The format is defined in [SDLC](../../docs/SDLC.md#done-entries). Until 2026-10-29 each
implementation kept its own log: [ushpy](archive/DONE-ushpy.md), [ushgo](archive/DONE-ushgo.md).

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
  [docs/releases/0.2.1.md](../../docs/releases/0.2.1.md).
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
    [docs/releases/0.2.0.md](../../docs/releases/0.2.0.md). Downloaded again: the checksums
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
