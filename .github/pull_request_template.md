## What and why

<!-- The problem and the resulting behaviour, and the TODO items it covers (e.g. ush TODO 2.9).
For a stacked PR, name the base branch. -->

## Tests

<!-- Exact commands and results. Name any check that was skipped or couldn't run.
Local CI: **Local CI ran** on <sha> (`just ci --post`): all jobs passed on macOS; the Linux
matrix entries were not run. -->

## Checklist (docs/SDLC.md, definition of done)

- [ ] Works as ARCH describes, or the deviation is recorded in DONE
- [ ] Tests for the observable behaviour and its relevant errors; a bug fix comes with a regression test that fails on the old code
- [ ] `just ci --post` passed on the head commit (while GitHub Actions is unavailable)
- [ ] Docs current: README, ARCH, SEC, ROAD, GAPS as affected; `just links` passes
- [ ] TODO items ticked; DONE entry on top (`## YYYY-MM-DD HH:MM — Title`, with the `Agent:` line)
- [ ] New dependencies pinned, with licence and maturity checked and recorded in ARCH
- [ ] Blocking GAPS questions answered by the user, not assumed

## Security impact (the project's docs/SEC.md)

<!-- None, or what changes. Changes to grants, storage, isolation, egress, credentials or
authorization need a threat-model update and negative tests. -->

- [ ] Model output and command output treated as untrusted; nothing they say widens permissions
- [ ] No keys or tokens in the repository, logs, history, fixtures or requests to other services
- [ ] Nothing leaves the machine without the disclosure saying so

## Remaining limitations

<!-- What doesn't work yet, and where it is tracked. AI never merges: a human reviews and merges. -->
