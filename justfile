# Document tooling and checks. Run `just` to list the available recipes.

[private]
default:
    @just --list

# Build FILE.md.pdf beside a Markdown file, resolving FILE from the calling directory
[positional-arguments]
pdf file:
    @cd {{quote(invocation_directory())}} && {{quote(justfile_directory() + "/tools/pdf/pdf.sh")}} "$@"

# Build PDFs under PATHS (default README.md, docs/, and src/), skipping unchanged documents
[positional-arguments]
pdfs *paths:
    @cd {{quote(invocation_directory())}} && {{quote(justfile_directory() + "/tools/pdf/pdfs.sh")}} "$@"

# Test both implementations of ush: ushpy-test, ushgo-test and ushgo-conform
ush-test: ushpy-test ushgo-test ushgo-conform

# Run the Python ush suite (unit, mock-provider integration, and zsh plugin tests)
ushpy-test:
    @cd src/py && ./scripts/test.sh

# Format-check, vet and test ushgo and the release tool (Go 1.27: brew install go)
ushgo-test:
    @src/go/scripts/test.sh
    @tools/release/test.sh

# ushpy's black-box tests against the Go binary: the two implementations behave the same
ushgo-conform:
    @tests/conform/run.sh

# Build the Go release archives into dist/, e.g. `just ushgo-release 0.1.0` (`--check` to build
# twice and compare). Building only; publishing a release is a human's step.
ushgo-release version *args:
    @tools/release/release.sh {{version}} {{args}}

# Check the documents: relative links and images in all Markdown files, and tools/docs' tests
links:
    @python3 tools/docs/check-links.py
    @python3 -m unittest discover -s tools/docs

# Local CI (tools/ci/README.md): the jobs of .github/workflows/ci.yml on a clean worktree of a
# commit, while GitHub Actions doesn't start jobs. E.g. `just ci`, `just ci --job ushpy`, `just ci
# --post` (commit statuses on the PR), `just ci <rev>`. Logs and summary in .ci/<date>-<commit>/.
# A run of all jobs that passes then builds the PDFs (`just pdfs`); a PDF that fails fails the run.
ci *ARGS:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{justfile_directory()}}"
    unset VIRTUAL_ENV
    uv run --quiet --script tools/ci/run.py {{ARGS}}
    case " {{ARGS}} " in *" --job"*) ;; *) just pdfs ;; esac

# Local CI on Linux (tools/ci/README.md): the workflow's ubuntu-latest entries in a container, on a
# pushed commit, e.g. `just ci-linux`, `just ci-linux --post`, `just ci-linux <rev>`. Needs Docker.
ci-linux *ARGS:
    @tools/ci/linux.sh {{ARGS}}

# Run ush from the source tree with the settings in .env (see .env.template), e.g.
# `just ush '//why is my disk full?'`. Variables already set in the shell win over .env; the file is
# read as KEY=value lines, not executed; a ` # comment` after a value is dropped. Runs the Python
# implementation; needs src/py/.venv (created by `just ushpy-test`).
[positional-arguments]
ush *args:
    #!/usr/bin/env bash
    set -euo pipefail
    root={{quote(justfile_directory())}}
    [ -x "$root/src/py/.venv/bin/python" ] || { echo "src/py/.venv is missing; run: just ushpy-test" >&2; exit 1; }
    if [ -f "$root/.env" ]; then
        while IFS='=' read -r key value || [ -n "$key" ]; do
            [[ $key =~ ^[A-Z_][A-Z0-9_]*$ ]] || continue
            value=$(printf '%s' "$value" | sed -E 's/[[:space:]]+#.*$//; s/[[:space:]]+$//')
            [ -n "${!key+set}" ] || export "$key=$value"
        done < "$root/.env"
    fi
    cd {{quote(invocation_directory())}}
    PYTHONPATH="$root/src/py/src" exec "$root/src/py/.venv/bin/python" -m ush "$@"
