#!/usr/bin/env bash
# Adapted from ../orgfab.ai/tools/pdfs.sh. Explicit paths use the calling directory;
# without paths, build README.md, wish/, and ush/ from the repository root.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
paths=("$@")
if [ "${#paths[@]}" -eq 0 ]; then
    cd "$(dirname "$here")"
    paths=(README.md wish ush)
fi
command -v git >/dev/null || { echo "ERROR: git is required" >&2; exit 1; }
filelist="$(mktemp)"
trap 'rm -f "$filelist"' EXIT
# Capture first so a Git enumeration failure cannot become a successful empty build.
git ls-files --cached --others --exclude-standard -z -- "${paths[@]}" > "$filelist"
fail=0
while IFS= read -r -d '' file; do
    case "$file" in *.md) ;; *) continue ;; esac
    case "${file##*/}" in AGENTS.md|CLAUDE.md) continue ;; esac
    [ -f "$file" ] || continue
    echo "── $file"
    "$here/pdf.sh" "$file" || { echo "FAILED: $file" >&2; fail=1; }
done < "$filelist"
exit "$fail"
