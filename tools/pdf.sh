#!/usr/bin/env bash
# Adapted from ../orgfab.ai/tools/pdf.sh: Markdown -> embedded HTML -> Chrome PDF.
# Output keeps the source's full name (file.md.pdf); builds are incremental.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(dirname "$here")"
css="$here/pdf-style.css"
stampdir="$root/.pdfstamps"

if [ "$#" -ne 1 ]; then
    echo "usage: tools/pdf.sh <file.md>" >&2
    exit 2
fi
src="$1"
case "$src" in
    *.md) ;;
    *) echo "ERROR: not a .md file: $src" >&2; exit 2 ;;
esac
if [ ! -f "$src" ]; then
    echo "ERROR: Markdown file does not exist: $src" >&2
    exit 2
fi
dir="$(cd "$(dirname "$src")" && pwd)"
base="$(basename "$src")"
src="$dir/$base"
out="$src.pdf"

command -v shasum >/dev/null || { echo "ERROR: shasum is required" >&2; exit 1; }
mkdir -p "$stampdir"
# Include the renderer itself so changes to conversion options trigger a rebuild.
# As in the reference tooling, embedded images are not part of the content stamp.
hash="$(cat "$src" "$css" "$here/pdf.sh" | shasum -a 256 | cut -d' ' -f1)"
stamp="$stampdir/$(printf '%s' "$out" | shasum -a 256 | cut -c1-16).sha"
if [ -f "$out" ]; then
    outhash="$(shasum -a 256 "$out" | cut -d' ' -f1)"
    if [ -f "$stamp" ] && [ "$(cat "$stamp")" = "$hash $outhash" ]; then
        echo "up to date: $out"
        exit 0
    fi
    # Adopt an existing newer PDF only when there is no previous content stamp.
    if [ ! -f "$stamp" ] && [ "$out" -nt "$src" ] && [ "$out" -nt "$css" ] && [ "$out" -nt "$here/pdf.sh" ]; then
        printf '%s %s\n' "$hash" "$outhash" > "$stamp"
        echo "up to date: $out"
        exit 0
    fi
fi

command -v pandoc >/dev/null || { echo "ERROR: pandoc is required" >&2; exit 1; }
chrome="${CHROME_BIN:-}"
if [ -z "$chrome" ] && [ -x "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]; then
    chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
fi
if [ -z "$chrome" ]; then
    for candidate in google-chrome google-chrome-stable chromium chromium-browser; do
        if command -v "$candidate" >/dev/null 2>&1; then
            chrome="$(command -v "$candidate")"
            break
        fi
    done
elif [ ! -x "$chrome" ] && command -v "$chrome" >/dev/null 2>&1; then
    chrome="$(command -v "$chrome")"
fi
if [ -z "$chrome" ] || [ ! -x "$chrome" ]; then
    echo "ERROR: Chrome/Chromium was not found; set CHROME_BIN to its executable" >&2
    exit 1
fi

tmp="$(mktemp -d)"
pid=""
stop_chrome() {
    if [ -n "$pid" ]; then
        kill "$pid" 2>/dev/null || true
        for _ in $(seq 1 20); do
            kill -0 "$pid" 2>/dev/null || break
            sleep 0.1
        done
        kill -9 "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
        pid=""
    fi
}
cleanup() {
    stop_chrome
    rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

pandoc "$src" -o "$tmp/doc.html" --standalone --embed-resources --mathml \
    --resource-path "$dir" --css "$css" --metadata title-prefix="" \
    --metadata pagetitle="$base"

# Some Chrome versions finish printing without exiting. Watch for a stable PDF,
# terminate the isolated browser, and retain the old output if rendering fails.
"$chrome" --headless --disable-gpu \
    --no-first-run --no-default-browser-check --disable-extensions \
    --virtual-time-budget=15000 --timeout=20000 \
    --user-data-dir="$tmp/chrome-profile" --no-pdf-header-footer \
    --print-to-pdf="$tmp/out.pdf" "$tmp/doc.html" >"$tmp/chrome.log" 2>&1 &
pid=$!
done=""
for _ in $(seq 1 120); do
    if ! kill -0 "$pid" 2>/dev/null; then
        done=1
        break
    fi
    if [ -s "$tmp/out.pdf" ]; then
        s1="$(wc -c < "$tmp/out.pdf")"
        sleep 0.5
        s2="$(wc -c < "$tmp/out.pdf" 2>/dev/null || echo 0)"
        if [ "$s1" = "$s2" ]; then
            done=1
            break
        fi
    fi
    sleep 0.5
done
stop_chrome
if [ -z "$done" ]; then
    echo "ERROR: Chrome timed out after 60 s" >&2
    cat "$tmp/chrome.log" >&2
    exit 1
fi
if [ ! -s "$tmp/out.pdf" ] || [ "$(head -c 5 "$tmp/out.pdf")" != "%PDF-" ]; then
    echo "ERROR: Chrome produced no valid PDF" >&2
    cat "$tmp/chrome.log" >&2
    exit 1
fi
mv "$tmp/out.pdf" "$out"
printf '%s %s\n' "$hash" "$(shasum -a 256 "$out" | cut -d' ' -f1)" > "$stamp"
echo "wrote $out"
