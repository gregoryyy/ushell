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
