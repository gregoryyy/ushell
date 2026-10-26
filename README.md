# ushell: wish and ush

Two shells that share one command paradigm: ordinary commands run as typed, and a
submission starting with `//` is a natural-language request. The AI turns it into
real shell commands that are shown, checked, and executed. Both use the user's own
AI API key.

| Project | Where it runs | Status |
| --- | --- | --- |
| [wish](wish/README.md) | Browser workspace: files, notebooks, live tools | Design documents; not implemented |
| [ush](ush/README.md) | zsh on the user's machine, optionally under `sudo` | Version 0.1 implemented; not yet released |

## Quick start: ush

ush needs zsh, Python 3.10 or newer, and an Anthropic API key.

```sh
cd ush && ./install.sh           # installs ~/.local/bin/ush; prints the zsh plugin line
ush ai configure                 # stores the key in the macOS keychain or a 0600 file
ush //find out why my machine does not have enough disk space
```

Read-only commands run immediately; anything else is shown for approval first.
See [ush/README.md](ush/README.md) for the plugin, `sudo` use, and what is sent
to the provider.

## Repository layout

```text
CLAUDE.md                 contribution, testing, security, and human-merge rules
wish/                     browser workspace: README, TODO, DONE, docs/
ush/                      zsh tool: README, TODO, DONE, docs/, src/, tests/, shell/
tools/                    PDF build scripts and the Markdown link checker
.github/workflows/        CI: link check and ush tests on Linux and macOS
```

Each project keeps its own `TODO.md` and `DONE.md` with independent milestone IDs.
Work happens on feature branches, with implementation notes appended to the
project's DONE file. Changes merge through human-reviewed GitHub PRs; AI never
merges. Every feature needs tests, and every release needs fresh security checks.

## Checks

```sh
just                      # list recipes
just ush-test             # ush unit, integration, and zsh tests
just links                # check relative links in all Markdown files
```

CI ([checks.yml](.github/workflows/checks.yml)) runs the link check and the ush
suite on Ubuntu and macOS with Python 3.10 and 3.13 for every pull request.

## Document PDFs

The PDF workflow is adapted from `../orgfab.ai`. It requires
[just](https://github.com/casey/just), [Pandoc](https://pandoc.org/), Git, `shasum`,
and Google Chrome or Chromium. On macOS, the scripts detect the standard Chrome
installation; elsewhere they look for a Chrome/Chromium executable on `PATH`.
Set `CHROME_BIN` to select a different browser executable.

```sh
just pdf wish/docs/IDEA.md  # write wish/docs/IDEA.md.pdf
just pdfs                 # build README.md and all Markdown documents in wish/ and ush/
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
