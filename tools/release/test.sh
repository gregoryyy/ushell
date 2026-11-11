#!/bin/sh
# Format, vet and test the release tool: `just ushgo-test` and the CI job ushgo run this. The tests
# build the four archives of the ush module in src/go/ and install one.
set -eu
cd "$(dirname "$0")"
export GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
    echo "not gofmt-formatted: $unformatted" >&2
    exit 1
fi
go vet ./...
go test ./...
