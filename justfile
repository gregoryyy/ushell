# Document tooling. Run `just` to list the available recipes.

[private]
default:
    @just --list

# Build FILE.md.pdf beside a Markdown file, resolving FILE from the calling directory
[positional-arguments]
pdf file:
    @cd {{quote(invocation_directory())}} && {{quote(justfile_directory() + "/tools/pdf.sh")}} "$@"

# Build PDFs under PATHS (default README.md and docs/), skipping unchanged documents
[positional-arguments]
pdfs *paths:
    @cd {{quote(invocation_directory())}} && {{quote(justfile_directory() + "/tools/pdfs.sh")}} "$@"
