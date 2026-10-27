# ushell: ush and wish

Shells that share one command paradigm: ordinary commands run as typed, and a submission starting
with `//` is a natural-language request. The AI turns it into real shell commands that are shown,
checked, and executed. Both use the user's own AI API key, or a local model.

| Project | Where it runs | Status |
| --- | --- | --- |
| [ush](ush/README.md) | zsh on the user's machine, optionally under `sudo` | Version 0.1 implemented; not yet released |
| [ushgo](ushgo/README.md) | The same as ush, as one static Go binary | Port set up; answers no requests yet |
| [wish](wish/README.md) | Browser workspace: files, notebooks, live tools | Design documents; not implemented |

## ush

ush brings `//` to zsh. Ask in plain language; ush investigates with real Unix commands, interprets
the output, and answers, with follow-up questions in the same context. It needs zsh, Python 3.10 or
newer, and an Anthropic API key or a local model in Ollama.

```sh
cd ush && ./install.sh           # installs ~/.local/bin/ush; prints the zsh plugin line
ush ai configure                 # provider, model, and key (macOS keychain or a 0600 file)
ush //find out why my machine does not have enough disk space
```

Read-only commands run immediately; anything else is shown for approval first. See
[ush/README.md](ush/README.md) for the plugin, `sudo` use, local models, and what is sent to the
provider. Its plan and records are in [ush/docs/](ush/docs/): [TODO](ush/docs/TODO.md),
[ROAD](ush/docs/ROAD.md), [ARCH](ush/docs/ARCH.md), [SEC](ush/docs/SEC.md),
[GAPS](ush/docs/GAPS.md), and [DONE](ush/docs/DONE.md).

[ushgo](ushgo/README.md) ports ush to Go, module by module, for a single static binary without an
interpreter for root to trust and without the two-second start. ush stays the reference until ushgo
passes ush's black-box tests ([ushgo TODO](ushgo/docs/TODO.md)).

## wish

wish is the web interactive shell: a browser workspace for files, datasets, notebooks, and live
tools, with `//` requests translated into shell programs that run within the workspace's grants.
It is a design project; nothing runs yet. Phase 1 is a lean browser shell around files, byte/text
pipes, JSON/CSV, and one Python notebook, starting from an evaluation of just-bash. See
[wish/README.md](wish/README.md), the product idea in [IDEA](wish/docs/IDEA.md), and the phases in
[ROAD](wish/docs/ROAD.md).

## How the work is organised

Each project keeps the same documents in its `docs/` folder: TODO (the plan, with stable numbered
items), DONE (the progress log, newest first), ARCH, ROAD, SEC, and GAPS (the questions only the
user can answer). [docs/SDLC.md](docs/SDLC.md) describes the process. Work happens on branches and
merges through human-reviewed GitHub PRs; AI never merges ([CLAUDE.md](CLAUDE.md)). Every feature
needs tests, and every release needs fresh security checks.

```text
CLAUDE.md                 rules for contributors and agents: human merges, tests, security, releases
docs/SDLC.md              the development process shared by all projects
ush/                      zsh tool: README, docs/, src/, tests/, shell/
ushgo/                    Go port of ush: README, docs/, cmd/, scripts/
wish/                     browser workspace: README, docs/
tools/                    PDF build scripts, the Markdown link checker, local CI (ci/)
.github/                  CI workflow (ci.yml), PR template
```

## Checks

```sh
just                      # list recipes
just ush-test             # ush unit, integration, and zsh tests
just ushgo-test           # ushgo: gofmt, go vet, go test
just links                # check relative links in all Markdown files
just ci                   # the CI workflow, locally, on a clean checkout of HEAD
just ush '//…'            # ush from source with the settings in .env (.env.template)
```

CI ([ci.yml](.github/workflows/ci.yml)) runs the link check, the ush suite on
Ubuntu and macOS with Python 3.10 and 3.13, the local CI's own tests, and a gitleaks
secret scan. GitHub Actions doesn't start jobs for this account at the moment, so
`just ci --post` runs the workflow on the developer's machine and reports to the PR
as commit statuses; see [tools/ci/README.md](tools/ci/README.md).

## Document PDFs

The PDF workflow is adapted from `../orgfab.ai`. It requires
[just](https://github.com/casey/just), [Pandoc](https://pandoc.org/), Git, `shasum`,
and Google Chrome or Chromium. On macOS, the scripts detect the standard Chrome
installation; elsewhere they look for a Chrome/Chromium executable on `PATH`.
Set `CHROME_BIN` to select a different browser executable.

```sh
just pdf wish/docs/IDEA.md  # write wish/docs/IDEA.md.pdf
just pdfs                 # build README.md and all Markdown documents in docs/, ush/, ushgo/, and wish/
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
