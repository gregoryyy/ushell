#!/bin/sh
# Build the release archives of the Go ush into ush/go/dist/ (tools/release): one per platform,
# with SHA256SUMS. The timestamps inside are the commit time, so the same commit and version give
# the same bytes on any machine. Building is local; publishing a release is a human's step.
#
#   scripts/release.sh 0.1.0            build into dist/
#   scripts/release.sh 0.1.0 --check    build twice and compare, keep nothing
set -eu
cd "$(dirname "$0")/.."
version=${1:?usage: scripts/release.sh VERSION [--check]}
shift
epoch=$(git log -1 --format=%ct)
if [ -n "$(git status --porcelain -- . ../shell ../man ../../LICENSE)" ]; then
    echo "release.sh: uncommitted changes in ush/go/, the plugin or the manual page; the build would not match the commit" >&2
    [ "${1:-}" = --check ] || exit 1
fi
export GOTOOLCHAIN=local GOFLAGS=-mod=readonly
if [ "${1:-}" = --check ]; then
    exec go run ./tools/release -version "$version" -epoch "$epoch" -check
fi
rm -rf dist
go run ./tools/release -version "$version" -epoch "$epoch" -out dist
ls -l dist
