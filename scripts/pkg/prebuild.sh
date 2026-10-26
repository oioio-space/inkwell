#!/usr/bin/env bash
# goreleaser "before" hook (.goreleaser.yaml): everything the executable and the packages need that
# goreleaser does not build itself (ADR 0025).
#   1. the frontend, which the executable embeds (go:embed frontend/dist);
#   2. the Typst sidecars, when the project has the task (internal/sidecar fetches them at build
#      time and embeds them per architecture): `mise run sidecar:fetch`;
#   3. the staged package files (manual page, AppStream with release notes).
#
# Usage: prebuild.sh VERSION COMMIT_DATE      (COMMIT_DATE is RFC 3339; only the day is used)
set -euo pipefail

version="${1:?usage: prebuild.sh VERSION COMMIT_DATE}"
commit_date="${2:?usage: prebuild.sh VERSION COMMIT_DATE}"

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

mise run fe:build

# The hook for the sidecars: present once internal/sidecar lands. It must fetch the binaries of the
# architecture this machine builds for, verify their SHA-256 against the committed hashes (ADR 0008)
# and leave them where go:embed picks them up.
if mise tasks ls 2>/dev/null | grep -q '^sidecar:fetch[[:space:]]'; then
  mise run sidecar:fetch
else
  echo "prebuild: no sidecar:fetch task yet, building without sidecars" >&2
fi

scripts/pkg/stage.sh "$version" "${commit_date%%T*}"
