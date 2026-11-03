#!/usr/bin/env bash
# Type checks available targets; the syntax scan includes every build tag.
set -euo pipefail
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
go build -o "$scratch/arch-exec" ./tools/cmd/arch-exec
"$scratch/arch-exec" -syntax .
failed=0
native_arch="$(go env GOARCH)"
for mode in server production; do
  echo "archexec: linux/$native_arch $mode cgo=1"
  GOOS=linux GOARCH="$native_arch" CGO_ENABLED=1 go list -deps -json -tags="$mode" ./internal/... ./cmd/... ./tools/... . | "$scratch/arch-exec" -deps || failed=1
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
  GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go list -deps -json -tags=production "${packages[@]}" | "$scratch/arch-exec" -deps || failed=1
  GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go vet -tags=production -vettool="$scratch/arch-exec" "${packages[@]}" || failed=1
done
exit "$failed"
