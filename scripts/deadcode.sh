#!/usr/bin/env bash
# Audit lot 1: exported functions unreachable from production command roots.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
# Pin to the same x/tools version as go.mod; never silently upgrade the analyzer. Raised from v0.52.0 to
# v0.52.1-0.20261106145633-d407941a339a with block LP1 (2026-11-24): libp2p v0.52.0 requires it.
version="$(go list -m -f '{{.Version}}' golang.org/x/tools)"
[[ "$version" == v0.52.1-0.20261106145633-d407941a339a ]] || { echo "deadcode: expected x/tools v0.52.1-0.20261106145633-d407941a339a, got $version" >&2; exit 1; }
raw="$(mktemp)"
trap 'rm -f "$raw"' EXIT
go run golang.org/x/tools/cmd/deadcode -json ./cmd/... > "$raw"
# Methods are represented as (Receiver).Method; keep exported names only.
violations="$(jq -r --rawfile baseline scripts/deadcode-baseline.txt '
  ($baseline | split("\n") | map(select(length > 0 and (startswith("#") | not)) | gsub("^\\s+|\\s+$"; ""))) as $known
  | [.[]? | .Path as $pkg | .Funcs[]
     | select(.Name | split(".") | last | test("^\\p{Lu}"))
     | $pkg + "." + .Name]
  | unique - $known | .[]
' "$raw")"
if [[ -n "$violations" ]]; then
  while IFS= read -r name; do
    printf '✗ new unreachable exported function: %s\n' "$name"
  done <<< "$violations"
  exit 1
fi
echo "✓ deadcode: no new unreachable exported functions"
