# ushell: ush and wish

Shells that share one command paradigm: ordinary commands run as typed, and a submission starting
with `//` is a natural-language request. The AI turns it into real shell commands that are shown,
checked, and executed. Both use the user's own AI API key, or a local model.

| Project | What | Status |
| --- | --- | --- |
| [ush](ush/README.md) | The `//` shell for zsh on your machine, optionally under `sudo`; one command, two implementations | 0.2.0-rc1 released |
| ↳ [ush/go](ush/go/README.md) | ush in Go: the compiled deliverable, one static binary | Shipped in 0.2.0-rc1 |
| ↳ [ush/py](ush/py/README.md) | ush in Python: the experimental implementation, where features are tried first | At parity with Go |
| [wish](wish/README.md) | Browser workspace: files, notebooks, live tools | Design documents; not implemented |

What comes next, across the projects, is in the roadmap: [docs/ROAD.md](docs/ROAD.md); what exists
next to them, and what sets them apart, in [docs/SOTA.md](docs/SOTA.md).

## ush

ush brings `//` to zsh. Ask in plain language; ush proposes the Unix command that answers it and
puts it on your command line. You run it, and `//` continues with its output: the next command, or
the answer. ush itself runs nothing, unless you choose agent mode (`--run`): then it investigates on
its own, running read-only commands at once and showing anything else for approval first. It uses
an Anthropic API key or a local model in Ollama, and an optional second model can check each
proposed command.

On the command line, a proposal waits for you: Enter runs it, Down explains it (what each part
does, which files and network it reads and writes), Esc removes it. `ush explain COMMAND` explains
any command line, and `-v` gives the full explanation.

`ush` is one command with two implementations that must behave the same: Python in `ush/py/` and Go
in `ush/go/`. `ush --version` says which one you have (`(python)` or `(go)`). Features land in Python
first and are then ported to Go; releases ship the Go binary, which passes the same black-box tests
([SDLC](docs/SDLC.md#ush-one-command-two-implementations)). Install a release archive
([ADMIN](ush/docs/ADMIN.md#install)), or from a checkout:

```sh
cd ush/py && ./install.sh        # Python; installs ~/.local/bin/ush, offers the plugin for ~/.zshrc
ush ai configure                 # mode, provider, model, key (macOS keychain or a 0600 file)
// find out why my machine does not have enough disk space      # in a new terminal
```

To check an installed `ush` of either implementation, run the user smoke test
([SMOKE](ush/docs/SMOKE.md)): `ush/py/scripts/smoke.sh` offline, then with
`--provider ollama --model …` on a local model, or `--bin PATH` for another binary.

Both share one set of documents in [ush/docs/](ush/docs/): the [user guide](ush/docs/USER.md) and
[admin guide](ush/docs/ADMIN.md), [TODO](ush/docs/TODO.md), [ROAD](ush/docs/ROAD.md),
[ARCH](ush/docs/ARCH.md), [SEC](ush/docs/SEC.md), [SECTEST](ush/docs/SECTEST.md),
[GAPS](ush/docs/GAPS.md) and [DONE](ush/docs/DONE.md). Building and testing each implementation
is in its README: [ush/go](ush/go/README.md) (Go 1.27, no dependencies) and [ush/py](ush/py/README.md)
(Python 3.10 or newer).

```sh
just ush-test                    # both implementations, and ushpy's black-box tests against Go
```

## wish

wish is the web interactive shell: a browser workspace for files, datasets, notebooks, and live
tools, with `//` requests translated into shell programs that run within the workspace's grants.
It is a design project; nothing runs yet. Phase 1 is a lean browser shell around files, byte/text
pipes, JSON/CSV, and one Python notebook, starting from an evaluation of just-bash. See
[wish/README.md](wish/README.md), the product idea in [IDEA](wish/docs/IDEA.md), and the phases in
[ROAD](wish/docs/ROAD.md).

## How the work is organised

Each project (ush, wish) keeps the same documents in its `docs/` folder: TODO (the plan,
with stable numbered items), DONE (the progress log, newest first), ARCH, ROAD, SEC, and GAPS (the
questions only the user can answer). [docs/SDLC.md](docs/SDLC.md) describes the process and
[docs/ROAD.md](docs/ROAD.md) the roadmap across projects. Work happens on branches and
merges through human-reviewed GitHub PRs; AI never merges ([AGENTS.md](AGENTS.md)). Every feature
needs tests, and every release needs fresh security checks.

```text
AGENTS.md                 rules for contributors and agents: human merges, tests, security, releases
CLAUDE.md                 points to AGENTS.md
docs/SDLC.md              the development process shared by all projects
docs/ROAD.md              the roadmap across projects
ush/docs/                 ush's documents, for both implementations; archive/ holds the old DONE logs
ush/go/                   ush in Go, the deliverable: README, cmd/ush/, internal/, scripts/
ush/py/                   ush in Python, experimental: README, src/, tests/, scripts/
ush/shell/                the zsh plugin and the installers' ~/.zshrc step, shared by both
ush/conform/              policy.json, the verdicts both implementations must give
wish/                     browser workspace: README, docs/
tools/                    the Markdown link checker, PDF build scripts (pdf/), local CI (ci/)
.github/                  CI workflow (ci.yml), PR template
```

## Checks

```sh
just                      # list recipes
just ush-test             # both implementations: ushpy-test and ushgo-test
just ushpy-test           # Python: unit, integration, and zsh tests
just ushgo-test           # Go: gofmt, go vet, go test
just links                # check relative links in all Markdown files
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
just pdf wish/docs/IDEA.md  # write wish/docs/IDEA.md.pdf
just pdfs                 # build README.md and all Markdown documents in docs/, ush/py/, ush/go/, and wish/
just pdfs wish/docs       # build only wish/docs/
just pdfs 'path with spaces'
```

Explicit paths are relative to the directory where the command is called:
`cd wish/docs` followed by `just pdf ARCH.md` or `just pdfs .` works. With no paths,
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
