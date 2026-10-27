# Local CI

How pull requests are checked while GitHub Actions can't run. GitHub refuses to start Actions jobs
for this account ("recent account payments have failed or your spending limit needs to be
increased"), first seen on PR #1 on 2026-10-26. Until that changes, CI runs on the developer's
machine and reports to GitHub as commit statuses. The design is lakeshore's local CI
(`../lakeshore/utils/ci/`, `../lakeshore/docs/ci/README.md`), extended for a build matrix.

## The workflow

One workflow, [ci.yml](../../.github/workflows/ci.yml), on every PR and on pushes to `main`. It
stays the one definition of CI; the local run reads it.

| Job | What it checks |
| --- | --- |
| `links` | relative links and images in all Markdown files (`tools/check-links.py`) |
| `ush` | the ush suite (`ush/scripts/test.sh`) on Ubuntu and macOS × Python 3.10 and 3.13 |
| `tools` | the local CI's own code (`tools/ci/`): ruff, format, pyright, its tests |
| `secrets` | gitleaks over the history; [.gitleaksignore](../../.gitleaksignore) lists synthetic test tokens |

## How the local run works

`just ci` runs `tools/ci/run.py` (Python 3.13 via uv, PyYAML 6.0.2 as an inline script dependency).
`tools/ci/workflow.py` maps the workflow to local steps; it is pure and tested (`test_workflow.py`).

1. **The commit, not the working copy.** The run reads `ci.yml` as it is in the commit and runs it
   in a fresh worktree of that commit in the temporary directory, removed afterwards (`--keep` keeps
   it). Uncommitted changes can neither make it pass nor break it.
2. **Steps as written.** `run` steps execute with `bash -eo pipefail` in the declared working
   directory, with the workflow's, job's and step's `env`, `RUNNER_TEMP`, `RUNNER_OS`,
   `GITHUB_SHA`, `GITHUB_WORKSPACE` and `GITHUB_STEP_SUMMARY`. `if:` takes `success()`,
   `failure()` and `always()`. The job's `timeout-minutes` applies.
3. **A matrix, by platform.** `strategy.matrix` with lists of values expands into one job per
   entry, named as GitHub names them (`ush (macos-latest, 3.10)`). Matrix values may appear in a
   job's `name` and `runs-on` and a step's `name` and `with`. When the matrix chooses the runner,
   only the entries for this machine's platform run; the others are listed as "not run here".
   A job with a fixed runner (`ubuntu-24.04`) runs on this machine as its stand-in.
4. **Actions mapped.** `actions/checkout` is the worktree. `actions/setup-python` takes an
   installed Python of the asked version, found by `uv python find` (no downloads), and links it
   first on `PATH` as `python` and `python3`. `astral-sh/setup-uv` sets `UV_PYTHON`.
   `gitleaks/gitleaks-action` runs the `gitleaks` binary over the history up to the commit
   (`brew install gitleaks`).
5. **Reduced environment.** Steps see only `PATH`, `HOME`, `USER`, `LOGNAME`, locale and `TMPDIR`,
   so tokens and API keys in the shell (including `ANTHROPIC_API_KEY`) don't reach them.
   `claude`, `codex` and `sudo` are shadowed by commands that fail: CI never calls a live model
   and never uses the real sudo.
6. **Errors instead of drift.** An action without a local equivalent, an expression in a `run`
   step or its `env`, another condition, a matrix with `include`/`exclude`, a `shell` default,
   services or an unknown key stops the run before anything runs.
7. **One run at a time.** Runs wait for each other, also from other clones: the lock is
   `ushell-ci.lock` in the temporary directory. Before starting, the run checks that at least
   1 GB is free.
8. **Stopping cleanly.** Each step runs in its own process group. A timeout, Ctrl-C or a kill
   stops the step with everything it started, removes the worktree, and sets the statuses of
   unfinished jobs to `error`.
9. **Output.** `.ci/<date>-<commit>/` holds a log per job and `summary.md` (jobs, steps, times,
   and the Python versions used). On a failure, the end of the step's log is printed.

## The workflow for a PR

1. **While working:** `just ci` on the last commit, or `just ci --job ush` for one job. Nothing is
   posted.
2. **For the PR, one run per commit:** push, then `just ci --post`, then open the PR. When all jobs
   pass, the run builds the PDFs (`just pdfs`, only the outdated ones); a PDF that fails to build
   fails the run. `--post` first checks that the commit is on GitHub, sets `pending` for every job
   that runs here, then the result: one commit status per job, `local / <job name>`. The PR shows
   them among its checks, next to the failed Actions checks of the same jobs, which never started;
   the prefix keeps the two apart.
3. **A status belongs to one commit:** run `just ci --post` again after every push. The PR text has
   one line that local CI ran: `` **Local CI ran** on <sha> (`just ci --post`): all jobs passed
   on macOS; the Linux matrix entries were not run. ``
4. **Before merging:** every `local / …` status on the head commit is green. A human merges; AI
   never does.

Commit statuses use the GitHub API, not Actions, so they work while Actions doesn't; the token
stays with `gh`. The workflow stays enabled: the refused jobs cost nothing, and they run again on
their own when Actions returns.

| Command | What it does |
| --- | --- |
| `just ci` | All jobs on `HEAD`, then `just pdfs` if they pass |
| `just ci <rev>` | On another commit |
| `just ci --job ush,tools` | Selected jobs (all their matrix entries for this platform), without PDFs |
| `just ci --post` | And set the commit statuses (the commit must be pushed) |
| `just ci --keep` | Keep the worktree for inspection |

## Differences from GitHub Actions

- **Platform:** on a Mac, the `ush` entries for `ubuntu-latest` don't run, and the jobs on a fixed
  `ubuntu-24.04` runner run on macOS instead. A failure only on Linux slips through until Actions
  returns or someone runs `just ci` on Linux.
- **Python:** the machine's installed patch release (e.g. 3.10.18), where GitHub installs the
  latest one. `uv python install 3.10` adds a missing version.
- **Secret scanning:** locally the Homebrew release of gitleaks over the whole history up to the
  commit; on GitHub the pinned action over the PR's commits.
- **Not a sandbox.** Steps run as the user, with the user's files and network. Local CI runs only
  our own branches; the reduced environment keeps tokens out of reach by accident, not against
  hostile code.
- **Statuses are self-reported.** They show that the run passed on that machine for that commit,
  not on an independent runner; the run's `summary.md` and logs are the evidence.

## Files

- `workflow.py`: the workflow mapped to local steps (pure), tests in `test_workflow.py`.
- `run.py`: the runner.
- `ruff.toml`, `pyrightconfig.json`: the checks of the `tools` job.
- `.ci/` at the repository root: logs and summaries; git-ignored.
