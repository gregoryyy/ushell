# ushell: ush, the `//` shell

ush brings `//` to zsh. Ask in plain language; ush proposes the Unix command that answers it and
puts it on your command line. You run it, and `//` continues with its output: the next command, or
the answer. ush itself runs nothing, unless you choose agent mode (`--run`): then it investigates on
its own, running read-only commands at once and showing anything else for approval first. It uses
an Anthropic API key or a local model in Ollama, and an optional second model can check each
proposed command.

On the command line, a proposal waits for you: Enter runs it, Down explains it (what each part
does, which files and network it reads and writes), Esc removes it. `ush explain COMMAND` explains
any command line, and `-v` gives the full explanation.

| Part | What | Status |
| --- | --- | --- |
| [src/go/](src/go/README.md) | ush in Go: the deliverable, one static binary, no dependencies; releases ship it | Shipped in 0.3.0-rc1 (GitHub release, Homebrew) |
| [src/py/](src/py/README.md) | ush in Python: the experimental implementation, where features are tried first | At parity with Go |
| [src/shell/](src/shell/) | The zsh plugin and the installers' `~/.zshrc` step, used by both | |
| [src/man/](src/man/ush.1) | The manual page, for both | |
| [tests/](tests/) | For both: `conform/` (the policy verdicts both must give, and the runner of the black-box tests against Go), `smoke/` (the user smoke test), `security/` (the injection eval) | |
| [docs/](docs/) | The documents, for both | |

`ush` is one command with two implementations that must behave the same. `ush --version` says
which one you have (`(python)` or `(go)`). Features land in Python first and are then ported to Go;
releases ship the Go binary, which passes the same black-box tests
([SDLC](docs/SDLC.md#ush-one-command-two-implementations)). Install a release archive
([ADMIN](docs/ADMIN.md#install)), or from a checkout:

```sh
cd src/py && ./install.sh        # Python; installs ~/.local/bin/ush, offers the plugin for ~/.zshrc
ush ai configure                 # mode, provider, model, key (macOS keychain or a 0600 file)
// find out why my machine does not have enough disk space      # in a new terminal
```

To check an installed `ush` of either implementation, run the user smoke test
([SMOKE](docs/SMOKE.md)): `tests/smoke/smoke.sh` offline, then with
`--provider ollama --model …` on a local model, or `--bin PATH` for another binary.

## Documents

- [USER](docs/USER.md) and [ADMIN](docs/ADMIN.md): using ush, and installing and running it for
  others; the [manual page](src/man/ush.1) says the same in short.
- [ARCH](docs/ARCH.md): how it is built, in Python and in Go.
- [SEC](docs/SEC.md) and [SECTEST](docs/SECTEST.md): the threat model, the security tests and
  findings.
- [SMOKE](docs/SMOKE.md): the user smoke test for an installed `ush` of either implementation.
- [TODO](docs/TODO.md), [ROAD](docs/ROAD.md), [GAPS](docs/GAPS.md) and [DONE](docs/DONE.md): the
  plan, the phases, the open questions and the progress log. Each TODO item says where it is
  built: in Python, in Go, or in both.
- [SOTA](docs/SOTA.md): what exists next to ush, and what sets it apart.
- [SDLC](docs/SDLC.md): the development process; [AGENTS.md](AGENTS.md): the rules that override it.
- [releases/](docs/releases/): release notes.

Until 2026-10-29 each implementation kept its own documents; their logs are in
[docs/archive/](docs/archive/), and [TODO](docs/TODO.md#old-ids) maps the old item numbers. Until
2026-11-10 the documents were in `ush/docs/`, and the sources in `ush/`.

## wish

wish, the web intelligent shell, moved to its own repository,
[gregoryyy/wish](https://github.com/gregoryyy/wish) (private for now), on 2026-11-10, with the history
of its documents. Its documents as they were here are at
[ushell 079f504](https://github.com/gregoryyy/ushell/tree/079f504/wish). ush and wish share the `//`
request contract ([ROAD](docs/ROAD.md) step 5), not code.

## How the work is organised

The documents in `docs/` follow [SDLC](docs/SDLC.md): TODO (the plan, with stable numbered items),
DONE (the progress log, newest first), ARCH, ROAD, SEC, and GAPS (the questions only the user can
answer). Work happens on branches and merges through human-reviewed GitHub PRs; AI never merges
([AGENTS.md](AGENTS.md)). Every feature needs tests, and every release needs fresh security checks.

```text
AGENTS.md                 rules for contributors and agents: human merges, tests, security, releases
CLAUDE.md                 points to AGENTS.md
docs/                     the documents: USER, ADMIN, ARCH, SEC, SECTEST, SMOKE, TODO, ROAD, GAPS,
                          DONE, SOTA, SDLC; releases/ (notes), archive/ (the old DONE logs)
src/go/                   ush in Go, the deliverable: README, cmd/ush/, internal/, scripts/, packaging/
src/py/                   ush in Python, experimental: README, src/, tests/, scripts/, install.sh
src/shell/                the zsh plugin and the installers' ~/.zshrc step, shared by both
src/man/                  the manual page, ush.1
tests/                    for both implementations: conform/ (policy.json, the verdicts both must
                          give; run.sh, ushpy's black-box tests against Go), smoke/ (smoke.sh, the
                          user smoke test), security/ (injection_eval.py)
tools/                    docs/ (link checker, document tests), pdf/ (PDF build), ci/ (local CI),
                          release/ (release archives, Homebrew formula)
.github/                  CI workflow (ci.yml), PR template
```

## Checks

```sh
just                      # list recipes
just ush-test             # both implementations, and ushpy's black-box tests against Go
just ushpy-test           # Python: unit, integration, and zsh tests (Python 3.10 or newer)
just ushgo-test           # Go: gofmt, go vet, go test (Go 1.27)
just links                # check the documents: relative links, tools/docs' tests
just ci                   # the CI workflow, locally, on a clean checkout of HEAD
just ush '//…'            # the Python ush from source with the settings in .env (.env.template)
```

CI ([ci.yml](.github/workflows/ci.yml)) runs the link check, the ushpy suite on
Ubuntu and macOS with Python 3.10 and 3.13, the ushgo suite on Ubuntu and macOS with
Go 1.27, the local CI's own tests, and a gitleaks secret scan. GitHub Actions doesn't
start jobs for this account at the moment, so `just ci --post` runs the workflow on the
developer's machine and reports to the PR as commit statuses; see
[tools/ci/README.md](tools/ci/README.md).

## Document PDFs

The PDF workflow is adapted from `../orgfab.ai`. It requires
[just](https://github.com/casey/just), [Pandoc](https://pandoc.org/), Git, `shasum`,
and Google Chrome or Chromium. On macOS, the scripts detect the standard Chrome
installation; elsewhere they look for a Chrome/Chromium executable on `PATH`.
Set `CHROME_BIN` to select a different browser executable.

```sh
just pdf docs/ARCH.md      # write docs/ARCH.md.pdf
just pdfs                 # build README.md and all Markdown documents in docs/ and src/
just pdfs docs            # build only docs/
just pdfs 'path with spaces'
```

Explicit paths are relative to the directory where the command is called:
`cd docs` followed by `just pdf ARCH.md` or `just pdfs .` works. With no paths,
`just pdfs` always uses the repository defaults. Bulk conversion includes tracked
and untracked files that Git does not ignore, and skips `AGENTS.md` and `CLAUDE.md`.

Each PDF keeps its source's full filename: `file.md` becomes `file.md.pdf`.
Pandoc embeds images and the print stylesheet, and Chrome prints the standalone
HTML with native MathML. Mermaid code blocks currently print as source text.
Generated PDFs and `.pdfstamps/` are ignored by Git.

Incremental builds check the Markdown, stylesheet, renderer script, and output PDF.
Editing an embedded image alone is not detected; remove the corresponding PDF to
force a rebuild. Failed builds leave any previous PDF in place. The scripts use a
temporary browser profile, so they do not depend on the user's browsing session.
