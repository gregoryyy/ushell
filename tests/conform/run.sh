#!/bin/sh
# Conformance (ushgo TODO 1.10): ushpy's black-box tests against the Go binary, next to the policy
# corpus both implementations must satisfy (policy.json). They start ush as
# a program (USH_TEST_BIN): the command line against a mock Messages API, the approval prompt and
# sessions in a pty, the zsh plugin, the security tests that run ush, and the smoke test. The
# same tests pass for the Python implementation, so the two behave the same.
# `just ushgo-conform` and the CI job ushgo-conform run this; it needs Go 1.27, Python 3.10+, zsh.
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/ush-conform.XXXXXX")
trap 'rm -rf "$work"' EXIT
(cd "$root/src/go" && GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0 go build -o "$work/ush" ./cmd/ush)
cd "$root/src/py"
python=${PYTHON:-python3}
if [ ! -x .venv/bin/python ]; then
    "$python" -m venv .venv
    # A failed install leaves no half-built venv that a later run would take as ready.
    .venv/bin/pip install -q --disable-pip-version-check --require-hashes -r requirements.lock ||
        { rm -rf .venv; exit 1; }
fi
"$work/ush" --version
cd tests
USH_TEST_BIN="$work/ush" ../.venv/bin/python -m unittest test_cli test_security test_smoke test_zsh_plugin test_models "$@"
