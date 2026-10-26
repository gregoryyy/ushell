# ush implementation notes

Append a dated entry for every completed TODO milestone, using its ush `x.x` ID.
Record what actually exists, exact validation, deviations, learnings, and branch,
commit, and PR status. Do not rewrite earlier entries; append corrections and
merge/release outcomes as new entries. All work follows [CLAUDE.md](../CLAUDE.md).

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
- Docs: [README.md](README.md), [DESIGN.md](docs/DESIGN.md), [SEC.md](docs/SEC.md),
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
