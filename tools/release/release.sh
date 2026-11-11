#!/bin/sh
# Build the release archives of the Go ush into dist/ at the repository root (main.go): one per
# platform, with SHA256SUMS. The timestamps inside are the commit time, so the same commit and
# version give the same bytes on any machine. Building is local; publishing a release is a human's
# step.
#
#   tools/release/release.sh 0.1.0            build into dist/
#   tools/release/release.sh 0.1.0 --check    build twice and compare, keep nothing
set -eu
cd "$(dirname "$0")"
version=${1:?usage: tools/release/release.sh VERSION [--check]}
shift
epoch=$(git log -1 --format=%ct)
if [ -n "$(git status --porcelain -- . ../../src/go ../../src/shell ../../src/man ../../LICENSE)" ]; then
    echo "release.sh: uncommitted changes in src/go/, the plugin, the manual page or the release tool; the build would not match the commit" >&2
    [ "${1:-}" = --check ] || exit 1
fi
export GOTOOLCHAIN=local GOFLAGS=-mod=readonly
if [ "${1:-}" = --check ]; then
    exec go run . -version "$version" -epoch "$epoch" -check
fi
rm -rf ../../dist
go run . -version "$version" -epoch "$epoch" -out ../../dist
ls -l ../../dist
