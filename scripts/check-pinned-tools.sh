#!/usr/bin/env bash
# Fails when a pinned tool that the gates run by bare name is not the one mise.toml pins:
# a copy found first on PATH (a stale `go install` build in $GOPATH/bin) or a wrong gosec version.
# Run by `mise run lint:pinned` (part of pregate) and by scan:code itself.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
# shellcheck source=scripts/pinned-path.sh
source scripts/pinned-path.sh

command -v mise >/dev/null 2>&1 || { echo "check-pinned-tools: mise is required" >&2; exit 1; }

status=0
for tool in gosec govulncheck gitleaks grype golangci-lint yamlfmt actionlint shellcheck zizmor; do
  want="$(mise which "$tool" 2>/dev/null || true)"
  got="$(command -v "$tool" || true)"
  if [[ -z "$want" || "$got" != "$want" ]]; then
    printf '✗ %s: runs %s, mise pins %s\n' "$tool" "${got:-nothing}" "${want:-nothing}" >&2
    status=1
  fi
done

# gosec prints "Version: dev" when built by `go install`: read the module version it embeds.
pinned="$(sed -n 's|^"go:github.com/securego/gosec/v2/cmd/gosec" *= *"\([^"]*\)".*|\1|p' mise.toml)"
embedded="$(go version -m "$(command -v gosec)" | awk '$1 == "mod" && $2 == "github.com/securego/gosec/v2" {sub(/^v/, "", $3); print $3}')"
if [[ -z "$pinned" || "$embedded" != "$pinned" ]]; then
  printf '✗ gosec: version %s, mise.toml pins %s\n' "${embedded:-unknown}" "${pinned:-nothing}" >&2
  status=1
fi

[[ $status -eq 0 ]] && echo "✓ pinned tools: the ones run are the ones mise.toml pins"
exit "$status"
