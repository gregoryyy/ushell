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
