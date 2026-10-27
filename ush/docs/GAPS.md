# Gaps

Open questions and explicit assumptions for ush. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted because they seemed clear enough
not to ask; check them and correct any that are wrong. The rules for this file are in
[SDLC](../../docs/SDLC.md#work-items). Status: 2026-10-27 14:52.

- Answers carry date, time and author: `**Answer** (2026-10-27 15:10, Gregor Heinrich): …`. Short
  answers are fine. A changed answer goes on top; earlier ones stay below it, struck through.
- Once answered, the decision moves into the document it affects, and the entry gets a
  `**Resolved**` line saying where. Resolved entries stay as a record.
- A question that blocks a TODO item is marked **blocking**.

## Questions

### Q1. Which licence? — blocking for TODO 3.1

The repository has no licence file, so nobody else may use, change or redistribute ush. A release
needs one. ush's dependencies (the Anthropic SDK and its tree) are MIT, Apache-2.0, BSD-3-Clause
and PSF-2.0, so any of the common choices is compatible.

- **Proposed:** Apache-2.0 for the whole repository (permissive, with a patent grant). MIT if you
  prefer the shortest licence; GPL-3.0-or-later if derived tools must stay open.

### Q2. How is ush distributed? — blocking for TODO 3.1

`install.sh` copies ush into a venv today, per user or root-owned. Other channels need their own
packaging and maintenance.

- **Proposed:** `install.sh` from a tagged release for 0.1; a Homebrew tap later, once ushgo
  ([ROAD](ROAD.md) phase 3) shows whether the Python or the Go implementation is the one to ship.

### Q3. Which platforms does a release support?

The suite runs on macOS locally; the Linux matrix entries in CI don't run while GitHub Actions
is unavailable ([tools/ci/README.md](../../tools/ci/README.md)).

- **Proposed:** macOS 14 or newer and Ubuntu 22.04 or newer, Python 3.10–3.13, zsh 5.8 or newer.
  A release needs one recorded run on Linux, by hand if Actions is still unavailable.

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | One person uses ush per account; under `sudo`, the invoking user (`SUDO_USER`) owns the key and settings. | [SEC](SEC.md), `config.py` | |
| A2 | Users accept that redacted command output goes to the selected provider, as the banner says before each request. | [SEC](SEC.md), README | |
| A3 | zsh is the only supported shell; bash and fish users quote requests or don't use the plugin. | [ARCH](ARCH.md), plugin | |
| A4 | Anthropic's default model stays `claude-opus-5-5` with effort `medium`, even though it is the most expensive choice. | `config.py`, README | |
