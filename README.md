# ushell: ush and wish

Shells that share one command paradigm: ordinary commands run as typed, and a submission starting
with `//` is a natural-language request. The AI turns it into real shell commands that are shown,
checked, and executed. Both use the user's own AI API key, or a local model.

| Project | What | Status |
| --- | --- | --- |
| **ush** | The `//` shell for zsh on your machine, optionally under `sudo`; one command, two implementations | |
| ↳ [ushpy](ushpy/README.md) | ush in Python: the experimental implementation, where features are tried first | Complete; not yet released |
| ↳ [ushgo](ushgo/README.md) | ush in Go: the compiled deliverable, one static binary | Ported, at parity with ushpy; 0.2.0-rc1 ships it |
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

`ush` is one command with two implementations that must behave the same: Python in `ushpy/` and Go
in `ushgo/`. `ush --version` says which one you have (`(python)` or `(go)`). Features land in Python
first and are then ported to Go; releases ship the Go binary once it passes the same black-box tests
([SDLC](docs/SDLC.md#ush-one-command-two-implementations)). Until then, install the Python one.

```sh
cd ushpy && ./install.sh         # installs ~/.local/bin/ush; offers to add the plugin to ~/.zshrc
ush ai configure                 # mode, provider, model, key (macOS keychain or a 0600 file)
// find out why my machine does not have enough disk space      # in a new terminal
```

To check an installed `ush` of either implementation, run the user smoke test
([SMOKE](ushpy/docs/SMOKE.md)): `ushpy/scripts/smoke.sh` offline, then with
`--provider ollama --model …` on a local model, or `--bin PATH` for another binary.

### ushpy: Python

The reference for behaviour and the place to try features: zsh plugin, `sudo` use, local models,
the checker model, and what is sent to the provider are described in
[ushpy/README.md](ushpy/README.md). Its plan and records are in [ushpy/docs/](ushpy/docs/):
[TODO](ushpy/docs/TODO.md), [ROAD](ushpy/docs/ROAD.md), [ARCH](ushpy/docs/ARCH.md),
[SEC](ushpy/docs/SEC.md), [SECTEST](ushpy/docs/SECTEST.md), [GAPS](ushpy/docs/GAPS.md), and
[DONE](ushpy/docs/DONE.md). It needs Python 3.10 or newer.

### ushgo: Go

The version users will install: one static binary, with no interpreter or installed packages for
root to trust under `sudo`, and no two-second start while a Python SDK loads. It has every feature
of ushpy, and ushpy's black-box tests pass against it (`just ushgo-conform`).

```sh
just ushgo-test                                  # gofmt, go vet, go test (Go 1.27: brew install go)
cd ushgo && go build -o ush ./cmd/ush            # writes ./ush
./ush --version                                  # ush 0.0.0-dev (go)
just ushgo-conform                               # ushpy's black-box tests against the Go binary
```

The [user guide](ushgo/docs/USER.md) and the [admin guide](ushgo/docs/ADMIN.md) describe the `ush`
command; ushpy has the same guides ([USER](ushpy/docs/USER.md), [ADMIN](ushpy/docs/ADMIN.md)). See [ushgo/README.md](ushgo/README.md) and its plan in
[ushgo/docs/](ushgo/docs/):
[TODO](ushgo/docs/TODO.md), [ROAD](ushgo/docs/ROAD.md), [ARCH](ushgo/docs/ARCH.md),
[SEC](ushgo/docs/SEC.md), [GAPS](ushgo/docs/GAPS.md), and [DONE](ushgo/docs/DONE.md).

## wish

wish is the web interactive shell: a browser workspace for files, datasets, notebooks, and live
tools, with `//` requests translated into shell programs that run within the workspace's grants.
It is a design project; nothing runs yet. Phase 1 is a lean browser shell around files, byte/text
pipes, JSON/CSV, and one Python notebook, starting from an evaluation of just-bash. See
[wish/README.md](wish/README.md), the product idea in [IDEA](wish/docs/IDEA.md), and the phases in
[ROAD](wish/docs/ROAD.md).

## How the work is organised

Each project (ushpy, ushgo, wish) keeps the same documents in its `docs/` folder: TODO (the plan,
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
ushpy/                    ush in Python, experimental: README, docs/, src/, tests/, shell/
ushgo/                    ush in Go, the deliverable: README, docs/, cmd/ush/, scripts/
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
just pdfs                 # build README.md and all Markdown documents in docs/, ushpy/, ushgo/, and wish/
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
