#!/usr/bin/env bash
# Architecture gate (docs/architecture.md, "Règles de dépendance"). Fails when:
#   - a package outside internal/transport/p2p imports github.com/libp2p directly
#     (reaching it through p2p is the point, so only direct imports count);
#   - a package outside internal/invite imports wormhole-william;
#   - any package depends, even transitively, on a Go automerge package (the CRDT
#     lives in the webview; Go only moves opaque bytes).
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

module="$(go list -m)"
fail=0

report() {
  printf '✗ architecture: %s\n' "$1" >&2
  fail=1
}

# Direct imports, tests included, of every package of the module.
while read -r pkg imports; do
  for imp in $imports; do
    case "$imp" in
      github.com/libp2p/*)
        [[ "$pkg" == "$module/internal/transport/p2p" ]] ||
          report "$pkg imports $imp (libp2p belongs to internal/transport/p2p only)"
        ;;
      github.com/psanford/wormhole-william*)
        [[ "$pkg" == "$module/internal/invite" ]] ||
          report "$pkg imports $imp (wormhole-william belongs to internal/invite only)"
        ;;
    esac
  done
done < <(go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...)

# Any automerge Go package in the full dependency graph, tests included.
automerge="$(go list -deps -test ./... | grep -i 'automerge' || true)"
if [[ -n "$automerge" ]]; then
  report "a Go automerge package is in the dependency graph:
$automerge"
fi

if [[ $fail -ne 0 ]]; then
  exit 1
fi
echo "✓ architecture: dependency rules hold"
