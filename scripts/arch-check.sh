#!/usr/bin/env bash
# Architecture gate (docs/architecture.md, "Règles de dépendance"). Fails when:
#   - a package outside internal/transport/p2p imports github.com/libp2p directly
#     (reaching it through p2p is the point, so only direct imports count);
#   - a package outside internal/invite imports wormhole-william;
#   - any package depends, even transitively, on a Go automerge package (the CRDT
#     lives in the webview; Go only moves opaque bytes);
#   - internal/sidecar/fetch (the build-time downloader) is imported by anything
#     but cmd/sidecar-fetch, so no download code ships in the executable;
#   - internal/sidecar or internal/typst imports net, net/http or crypto/tls:
#     Typst and its sidecars never go to the network from Inkwell's side.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

module="$(go list -m)"
fail=0
# Capture first: go list failures must fail the gate, not disappear in process substitution.
graph="$(go list -deps -f '{{.ImportPath}} {{join .Imports " "}}' ./...)"

report() {
  printf '✗ architecture: %s\n' "$1" >&2
  fail=1
}

# No internal package may depend directly on the application composition layer.
while read -r pkg imports; do
  for imp in $imports; do
    # cmd/* is the composition root (main); only internal packages are barred from app.
    if [[ "$imp" == "$module/internal/app" && "$pkg" == "$module/internal/"* && "$pkg" != "$imp" ]]; then
      report "$pkg imports $imp (no internal package may import internal/app)"
    fi
  done
done <<< "$graph"

# Each core root gets its own closure: C1 is allowed only for trust roots.
roots="$(go list ./internal/trust/... ./internal/identity/... ./internal/transport/...)"
transport_deps="$(go list -deps "$module/internal/transport")"
while IFS= read -r root; do
  [[ "$root" != "$module/internal/transport/p2p" && "$root" != "$module/internal/transport/p2p/"* ]] || continue
  deps="$(go list -deps "$root")"
  case "$root" in
    "$module/internal/trust"|"$module/internal/trust/"*)
      # Inspect edges outside transport's closure so C1's dependencies are exempt,
      # while any independent route into transport remains forbidden.
      while read -r pkg imports; do
        grep -Fxq "$pkg" <<< "$deps" || continue
        [[ "$pkg" == "$module/internal/trust" || "$pkg" == "$module/internal/trust/"* ]] || {
          if grep -Fxq "$pkg" <<< "$transport_deps"; then continue; fi
        }
        for imp in $imports; do
          case "$imp" in
            "$module/internal/transport"|"$module/internal/transport/"*)
              [[ "$pkg $imp" == "$module/internal/trust $module/internal/transport" ]] ||
                report "$root reaches $imp via $pkg (trust must not reach transport; only C1 is deferred)"
              ;;
          esac
        done
      done <<< "$graph"
      ;;
    *)
      while IFS= read -r dep; do
        case "$root" in
          "$module/internal/identity"|"$module/internal/identity/"*)
            case "$dep" in
              "$module/internal/transport"|"$module/internal/transport/"*)
                report "$root reaches $dep (identity must not reach transport)" ;;
            esac ;;
          *)
            case "$dep" in
              "$module/internal/transport"|"$module/internal/transport/"*) ;;
              "$module/internal/"*) report "$root reaches $dep (transport must be independent)" ;;
            esac ;;
        esac
      done <<< "$deps"
      ;;
  esac
done <<< "$roots"

# Direct imports, tests included, of every package of the module.
direct="$(go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...)"
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
      "$module/internal/sidecar/fetch")
        [[ "$pkg" == "$module/cmd/sidecar-fetch" ]] ||
          report "$pkg imports $imp (the downloader belongs to cmd/sidecar-fetch only; the executable must not download sidecars)"
        ;;
    esac
  done
done <<< "$direct"

# The runtime packages that launch Typst hold no network code of their own
# (non-test imports; their tests use a local listener to prove the point).
network="$(go list -f '{{.ImportPath}} {{join .Imports " "}}' ./internal/sidecar ./internal/typst)"
while read -r pkg imports; do
  for imp in $imports; do
    case "$imp" in
      net | net/http | net/http/* | crypto/tls)
        report "$pkg imports $imp (sidecar and typst must not do network I/O)"
        ;;
    esac
  done
done <<< "$network"

# Any automerge Go package in the full dependency graph, tests included.
all_deps="$(go list -deps -test ./...)"
automerge="$(grep -i automerge <<< "$all_deps" || true)"
if [[ -n "$automerge" ]]; then
  while IFS= read -r dep; do
    report "$dep (Go automerge must not be in the dependency graph)"
  done <<< "$automerge"
fi

if [[ $fail -ne 0 ]]; then
  exit 1
fi
echo "✓ architecture: dependency rules hold"
