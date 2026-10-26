#!/usr/bin/env python3
"""Check that relative Markdown links and images point to existing files.

Usage: tools/check-links.py [PATH ...]   (default: tracked and untracked *.md files)
External URLs and pure #anchors are not checked. Exits 1 if a link is broken.
"""
import re
import subprocess
import sys
from pathlib import Path

LINK = re.compile(r"!?\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
FENCE = re.compile(r"^\s*(```|~~~)")


def markdown_files(paths):
    out = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", *paths],
        check=True, capture_output=True,
    ).stdout.decode()
    return [Path(p) for p in out.split("\0") if p.endswith(".md") and Path(p).is_file()]


def broken_links(path):
    in_fence = False
    for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if FENCE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        line = re.sub(r"`[^`]*`", "", line)  # links inside inline code are examples
        for target in LINK.findall(line):
            if re.match(r"^[a-z][a-z0-9+.-]*:", target, re.I) or target.startswith("#"):
                continue
            file_part = target.split("#", 1)[0]
            if file_part and not (path.parent / file_part).exists():
                yield lineno, target


def main(argv):
    failed = False
    for path in markdown_files(argv or ["."]):
        for lineno, target in broken_links(path):
            print(f"{path}:{lineno}: broken link {target}")
            failed = True
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
