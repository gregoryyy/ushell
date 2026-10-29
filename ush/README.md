# ush

`ush` brings `//` to zsh: ask in plain language at the prompt, and ush proposes the shell command
that answers it, on your command line. You run it, and `//` continues with its output. It is one
command with two implementations that behave the same.

| Directory | What |
| --- | --- |
| [go/](go/README.md) | The Go implementation, the deliverable: one static binary, no dependencies; releases ship it |
| [py/](py/README.md) | The Python implementation, where features are tried first |
| [shell/](shell/) | The zsh plugin and the installers' `~/.zshrc` step, used by both |
| [conform/](conform/) | `policy.json`: the policy verdicts both implementations must give |
| [docs/](docs/) | The documents, for both |

## Documents

- [USER](docs/USER.md) and [ADMIN](docs/ADMIN.md): using ush, and installing and running it for
  others.
- [ARCH](docs/ARCH.md): how it is built, in Python and in Go.
- [SEC](docs/SEC.md) and [SECTEST](docs/SECTEST.md): the threat model, the security tests and
  findings.
- [SMOKE](docs/SMOKE.md): the user smoke test for an installed `ush` of either implementation.
- [TODO](docs/TODO.md), [ROAD](docs/ROAD.md), [GAPS](docs/GAPS.md) and [DONE](docs/DONE.md): the
  plan, the phases, the open questions and the progress log. Each TODO item says where it is
  built: in Python, in Go, or in both.

Until 2026-10-29 each implementation kept its own documents; their logs are in
[docs/archive/](docs/archive/), and [TODO](docs/TODO.md#old-ids) maps the old item numbers.

## Test

```sh
just ush-test       # from the repository root: ushpy-test, ushgo-test, and ushgo-conform
```

The process is the repository's [SDLC](../docs/SDLC.md#ush-one-command-two-implementations); the
rules in [AGENTS.md](../AGENTS.md) take precedence.
