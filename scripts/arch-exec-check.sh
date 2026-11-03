#!/usr/bin/env bash
# Our own code starts no process outside internal/sidecar (tools/archexec).
# Type checks every available build configuration; the syntax scan includes
# every build tag. Dependencies are not scanned (N42, clarified 2026-11-03).
set -euo pipefail
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
go build -o "$scratch/arch-exec" ./tools/cmd/arch-exec
"$scratch/arch-exec" -syntax .
failed=0
native_arch="$(go env GOARCH)"
for mode in server production; do
  echo "archexec: linux/$native_arch $mode cgo=1"
  GOOS=linux GOARCH="$native_arch" CGO_ENABLED=1 go vet -tags="$mode" -vettool="$scratch/arch-exec" ./internal/... ./cmd/... ./tools/... . || failed=1
done
for target in darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  echo "archexec: $target production cgo=0"
  # Wails' darwin application package requires cgo even for its Go files.
  # Its composition root is syntax-scanned now; full typing is deferred to N35.
  packages=(./internal/... ./cmd/... ./tools/... .)
  if [[ "$target" == darwin/* ]]; then
    package_list="$(GOOS=darwin GOARCH="${target#*/}" CGO_ENABLED=0 go list -tags=production ./internal/... ./cmd/... ./tools/... .)"
    mapfile -t packages <<< "$(grep -v '/cmd/inkwell$' <<< "$package_list")"
    echo "deferred: darwin cmd/inkwell full Wails typing requires native cgo (N35)"
  fi
  GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go vet -tags=production -vettool="$scratch/arch-exec" "${packages[@]}" || failed=1
done
exit "$failed"
