#!/usr/bin/env bash
# Compile with the toolchain available, then run every confinement test without PATH.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
go test -race -c -o "$scratch/sidecar.test" ./internal/sidecar
PATH="" "$scratch/sidecar.test" -test.v
