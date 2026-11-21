#!/usr/bin/env bash
# Build the test binary and fixtures, then run every confinement test without PATH.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
go test -race -c -o "$scratch/sidecar.test" ./internal/sidecar
mkdir "$scratch/fixtures"
"$scratch/sidecar.test" "--sidecar-test=build-probe-fixtures:$scratch/fixtures"
# Match go test's working directory for package-relative test data.
cd internal/sidecar
PATH="" "$scratch/sidecar.test" -test.v -sidecar-probe-fixtures "$scratch/fixtures"
