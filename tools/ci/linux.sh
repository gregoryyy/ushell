#!/bin/sh
# Local CI on Linux (TODO 6.5): the workflow's ubuntu-latest matrix entries in a container, by the
# same tools/ci/run.py that `just ci` runs on macOS. See tools/ci/README.md.
#
#   tools/ci/linux.sh [REV] [--post] [--job ushpy,…] [--platform linux/amd64] [--keep]
#
# REV (default HEAD) must be on GitHub: the container clones it from there, so it tests exactly
# the pushed commit, not this working copy. --post sets the `local / … (ubuntu-latest …)` statuses
# with this machine's `gh` token, passed to the container for that run only. The default jobs are
# those with an Ubuntu matrix entry (ushpy, ushgo, ushgo-conform); links, tools and secrets have
# a fixed runner and run in `just ci`. --platform linux/amd64 runs GitHub's architecture through
# emulation (Colima: `colima start --vz-rosetta`); the default is the machine's own. Needs Docker
# (`brew install colima docker`, then `colima start`). Logs: .ci/linux-<date>-<commit>/.
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
rev=HEAD post= jobs=ushpy,ushgo,ushgo-conform platform= keep=
while [ $# -gt 0 ]; do
    case "$1" in
        --post) post=--post; shift ;;
        --job) jobs=${2:?--job needs job IDs}; shift 2 ;;
        --platform) platform=${2:?--platform needs e.g. linux/amd64}; shift 2 ;;
        --keep) keep=--keep; shift ;;
        -*) echo "linux.sh: unknown option $1" >&2; exit 2 ;;
        *) rev=$1; shift ;;
    esac
done
command -v docker >/dev/null || { echo "linux.sh: needs docker (brew install colima docker; colima start)" >&2; exit 2; }
docker info >/dev/null 2>&1 || { echo "linux.sh: Docker is not running (colima start)" >&2; exit 2; }

sha=$(git -C "$root" rev-parse --verify "$rev^{commit}")
short=$(printf %.7s "$sha")
[ -n "$(git -C "$root" branch -r --contains "$sha")" ] ||
    { echo "linux.sh: $short is not on GitHub yet; push it first" >&2; exit 2; }
url=$(git -C "$root" remote get-url origin)
case "$url" in git@github.com:*) url=https://github.com/${url#git@github.com:} ;; esac

image=ushell-ci-linux:$(git -C "$root" hash-object "$root/tools/ci/linux/Dockerfile" | cut -c1-12)
set -- ${platform:+--platform "$platform"}
if [ -n "$platform" ]; then image=$image-$(echo "$platform" | tr / -); fi
docker image inspect "$image" >/dev/null 2>&1 ||
    docker build "$@" -t "$image" "$root/tools/ci/linux"

out="$root/.ci/linux-$(date +%Y-%m-%d-%H%M)-$short"
mkdir -p "$out"
token=
if [ -n "$post" ]; then token=$(gh auth token); fi
# The token reaches run.py, which posts the statuses; run.py gives the steps a reduced environment
# without it (README, "Reduced environment").
GH_TOKEN=$token docker run --rm --init ${platform:+--platform "$platform"} \
    -e GH_TOKEN -v "$out:/out" "$image" bash -euo pipefail -c '
        git clone --quiet --filter=blob:none "$1" ushell
        cd ushell
        git checkout --quiet --detach "$2"
        status=0
        uv run --quiet --script tools/ci/run.py "$2" --job "$3" $4 $5 || status=$?
        cp -R .ci/. /out/ 2>/dev/null || true
        exit $status' sh "$url" "$sha" "$jobs" "$post" "$keep"
