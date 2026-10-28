#!/bin/sh
# Format, vet and test ushgo: `just ushgo-test` and the CI job run this. Go never downloads another
# toolchain or a module here; the port has no dependencies yet.
set -eu
cd "$(dirname "$0")/.."
export GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
    echo "not gofmt-formatted: $unformatted" >&2
    exit 1
fi
go vet ./...
go test ./...
