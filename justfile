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
