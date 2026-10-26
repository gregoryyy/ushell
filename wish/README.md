# wish

wish is the web interactive shell: a browser workspace for files, datasets,
notebooks, live tools, and local or explicitly attached server resources.
This directory currently contains its design documents; runtime commands are not
implemented yet.

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

The local command-line counterpart, [ush](../ush/README.md), applies the same `//`
paradigm to a zsh session on the user's own machine.
