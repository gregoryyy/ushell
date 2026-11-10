# wish

wish is the **web intelligent shell**. Its working space is a **website**: the current page's
semantic structure is the current directory, the site's pages are a directory tree, and the
hyperlink graph has a namespace of its own. An agent crawls by content, deciding which links lead
to the information it is after and recording why. What it captures with Playwright becomes a
semantic overlay (references to where things are, optionally the data itself) that you can
browse across sites, question with structured retrieval, and use to research evidence and trends
for a claim on additional sites, with every answer explained down to its evidence, the path that reached it, and what wasn't seen.

This directory contains design documents; nothing is implemented. wish is open source (MIT) and
built on open components only. It will be developed as a separate project in its own repository
(decided 2026-11-10; [GAPS](docs/GAPS.md) Q13, [TODO](docs/TODO.md) 0.11); until the move, its
documents live here.

- [IDEA.md](docs/IDEA.md): the idea, use cases and command examples.
- [ROAD.md](docs/ROAD.md): phase 1 (one site), phase 2 (content-driven crawling, overlay), phase 3
  (evidence, trends, export).
- [ARCH.md](docs/ARCH.md): technical architecture and decisions.
- [SEC.md](docs/SEC.md): security profile: navigation side effects, sessions, injection, disclosure.
- [SOTA.md](docs/SOTA.md): research and comparisons with existing tools.
- [WORKSPACE.md](docs/WORKSPACE.md): the browser-workspace concept of 2026-10-14, now the substrate
  (shell, files, notebook, `//`).
- [TODO.md](docs/TODO.md): numbered implementation milestones and acceptance criteria.
- [GAPS.md](docs/GAPS.md): questions for the user and assumptions to check.
- [DONE.md](docs/DONE.md): progress log, newest first.

The local command-line counterpart, [ush](../ush/py/README.md), applies the same `//` paradigm to a
zsh session on the user's own machine.
