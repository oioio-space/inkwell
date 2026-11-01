#!/usr/bin/env bash
# Architecture gate (docs/architecture.md, "Règles de dépendance"). Fails when:
#   - a package outside internal/transport/p2p or protocol/libp2p imports libp2p directly
#     (reaching it through p2p is the point, so only direct imports count);
#   - a package outside internal/invite imports wormhole-william;
#   - any package depends, even transitively, on a Go automerge package (the CRDT
#     lives in the webview; Go only moves opaque bytes);
#   - internal/sidecar/fetch (the build-time downloader) is imported by anything
#     but cmd/sidecar-fetch, so no download code ships in the executable;
#   - internal/sidecar or internal/typst imports net, net/http or crypto/tls:
#     Typst and its sidecars never go to the network from Inkwell's side.
set -euo pipefail

for tool in git go grep; do
  command -v "$tool" >/dev/null 2>&1 || { printf 'Required tool missing: %s\n' "$tool" >&2; exit 1; }
done

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

# Each core root gets its own dependency closure, with no exceptions.
roots="$(go list ./internal/trust/... ./internal/identity/... ./internal/transport/...)"
while IFS= read -r root; do
  [[ "$root" != "$module/internal/transport/p2p" && "$root" != "$module/internal/transport/p2p/"* ]] || continue
  deps="$(go list -deps "$root")"
  while IFS= read -r dep; do
    case "$root" in
      "$module/internal/trust"|"$module/internal/trust/"*|"$module/internal/identity"|"$module/internal/identity/"*)
        case "$dep" in
          "$module/internal/transport"|"$module/internal/transport/"*)
            report "$root reaches $dep (trust and identity must not reach transport)" ;;
        esac ;;
      *)
        case "$dep" in
          "$module/internal/transport"|"$module/internal/transport/"*) ;;
          "$module/internal/"*) report "$root reaches $dep (transport must be independent)" ;;
        esac ;;
    esac
  done <<< "$deps"
done <<< "$roots"

# Direct imports, tests included, of every package of the module.
direct="$(go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...)"
while read -r pkg imports; do
  for imp in $imports; do
    case "$imp" in
      github.com/libp2p/*)
        [[ "$pkg" == "$module/internal/transport/p2p" || "$pkg" == "$module/protocol/libp2p" || "$pkg" == "$module/protocol/libp2p/"* ]] ||
          report "$pkg imports $imp (libp2p belongs to internal/transport/p2p or protocol/libp2p)"
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

# ADR 0034 / spec §1.6: the independently buildable pure module has no app dependency.
protocol_direct="$(cd protocol && go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...)"
while read -r pkg imports; do
  for imp in $imports; do
    case "$imp" in
      "$module/internal"|"$module/internal/"*|"$module/cmd"|"$module/cmd/"*)
        report "$pkg imports application package $imp" ;;
      github.com/libp2p/*|github.com/wailsapp/*|*automerge*)
        report "$pkg imports forbidden pure-module dependency $imp" ;;
    esac
  done
done <<< "$protocol_direct"
# Former codec owners may forward APIs, but may not retain serialization code.
# This checks implementation primitives, not filenames, so renaming a copy cannot evade it.
if grep -rnE --include='*.go' --exclude='*_test.go' '(encoding/(binary|base32)|crypto/ed25519|func .*encodeBody|func .*decodeMessage)' internal/trust; then
  report "internal/trust contains a private codec implementation"
else
  status=$?; [[ $status -eq 1 ]] || exit "$status"
fi
if grep -rnE --include='*.go' --exclude='*_test.go' '^func (Encode|Decode|HashOf|ParseHash|ParseText|DecodeRequest|DecodeReply)\(' internal/blob internal/invite; then
  report "an application package implements an extracted codec instead of forwarding it"
else
  status=$?; [[ $status -eq 1 ]] || exit "$status"
fi
for codec in internal/blob/wire.go internal/blob/hash.go internal/invite/ticket.go internal/invite/lobby.go; do
  if [[ -f "$codec" ]]; then report "$codec duplicates an extracted protocol codec"; fi
done

if [[ $fail -ne 0 ]]; then
  exit 1
fi
echo "✓ architecture: dependency rules hold"
