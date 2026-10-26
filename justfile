# Document tooling and checks. Run `just` to list the available recipes.

[private]
default:
    @just --list

# Build FILE.md.pdf beside a Markdown file, resolving FILE from the calling directory
[positional-arguments]
pdf file:
    @cd {{quote(invocation_directory())}} && {{quote(justfile_directory() + "/tools/pdf.sh")}} "$@"

# Build PDFs under PATHS (default README.md, wish/, and ush/), skipping unchanged documents
[positional-arguments]
pdfs *paths:
    @cd {{quote(invocation_directory())}} && {{quote(justfile_directory() + "/tools/pdfs.sh")}} "$@"

# Run the ush test suite (unit, mock-provider integration, and zsh plugin tests)
ush-test:
    @cd ush && ./scripts/test.sh

# Check relative links and images in all Markdown files
links:
    @python3 tools/check-links.py

# Local CI (tools/ci/README.md): the jobs of .github/workflows/ci.yml on a clean worktree of a
# commit, while GitHub Actions doesn't start jobs. E.g. `just ci`, `just ci --job ush`, `just ci
# --post` (commit statuses on the PR), `just ci <rev>`. Logs and summary in .ci/<date>-<commit>/.
# A run of all jobs that passes then builds the PDFs (`just pdfs`); a PDF that fails fails the run.
ci *ARGS:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{justfile_directory()}}"
    unset VIRTUAL_ENV
    uv run --quiet --script tools/ci/run.py {{ARGS}}
    case " {{ARGS}} " in *" --job"*) ;; *) just pdfs ;; esac
