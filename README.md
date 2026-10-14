# wish

wish is the web interactive shell: a browser workspace for files, datasets,
notebooks, live tools, and local or explicitly attached server resources.
The project currently contains design documents and document build tooling.

Phase 1 starts with a lean browser shell: ordinary files, byte/text pipes, JSON/CSV,
a Python notebook, and `//` commands using the user's key. just-bash is the first
shell candidate to evaluate. Phase 2 adds tools and remote resources where useful;
typed pipelines and public resource handles remain deferred.

- [IDEA.md](docs/IDEA.md): product definition and command examples.
- [PHASES.md](docs/PHASES.md): lean phase 1 and optional phase 2 ideas.
- [ARCH.md](docs/ARCH.md): technical architecture.
- [SEC.md](docs/SEC.md): security profile and browser obstacles.
- [SOTA.md](docs/SOTA.md): research and comparisons with existing tools.
- [TODO.md](TODO.md): numbered implementation milestones and acceptance criteria.
- [DONE.md](DONE.md): implementation notes, validation, deviations, and learnings.
- [CLAUDE.md](CLAUDE.md): contribution, testing, security, and human-merge rules.

Work follows the plan on feature branches, with implementation notes appended to
DONE. Changes merge through human-reviewed GitHub PRs; AI never merges. Every
feature needs tests, and every release needs fresh security checks.

## Document PDFs

The PDF workflow is adapted from `../orgfab.ai`. It requires
[just](https://github.com/casey/just), [Pandoc](https://pandoc.org/), Git, `shasum`,
and Google Chrome or Chromium. On macOS, the scripts detect the standard Chrome
installation; elsewhere they look for a Chrome/Chromium executable on `PATH`.
Set `CHROME_BIN` to select a different browser executable.

```sh
just                      # list recipes
just pdf docs/IDEA.md      # write docs/IDEA.md.pdf
just pdfs                 # build README.md and all Markdown documents in docs/
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
