#!/bin/sh
# Run the ush test suite in ush/.venv, created from the hashed lock file if missing.
# Tests use fake providers, a local mock API, and temporary directories; they never
# call a paid provider. Extra arguments go to `unittest discover` (e.g. -k policy).
set -eu
cd "$(dirname "$0")/.."
python=${PYTHON:-python3}
if [ ! -x .venv/bin/python ]; then
    "$python" -c 'import sys; sys.exit(sys.version_info < (3, 10))' ||
        { echo "ush needs Python 3.10 or newer; set PYTHON" >&2; exit 1; }
    "$python" -m venv .venv
    .venv/bin/pip install -q --disable-pip-version-check --require-hashes -r requirements.lock
fi
cd tests
exec ../.venv/bin/python -m unittest discover "$@"
