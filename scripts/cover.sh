#!/usr/bin/env bash
# Audit M4: statement-weighted coverage per package, after merging profile blocks.
#   scripts/cover.sh          # (re)generate coverage.out + check every package
#   scripts/cover.sh --check  # reuse coverage.out (or generate) + check every package
#
# COVER_MIN defaults to 85; exemptions are listed explicitly below.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

# In --check mode reuse coverage.out if a prior task (test:ci) already produced it;
# otherwise generate it. Plain mode always regenerates.
if [[ "${1:-}" != "--check" || ! -f coverage.out ]]; then
  go test -short -timeout=8m -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
fi

# The nested protocol module is not traversed by the root module's ./....
# Record its own tests as well as coverage contributed by root-module tests.
if [[ "${1:-}" != "--check" || ! -f coverage.protocol.out ]]; then
  (cd protocol && go test -short -timeout=8m -covermode=atomic -coverpkg=./... -coverprofile=../coverage.protocol.out ./...)
fi

# Explicit exemptions only: main wiring covered by e2e, and a test helper.
# cmd/sidecar-fetch, cmd/inkwell-relay and internal/sidecar/fetch are NOT exempt.
module="$(go list -m)"
min="${COVER_MIN:-85}"
awk -v module="$module" -v min="$min" '
$1 == "mode:" { next }
{
  block=$1
  weights[block]=$2
  counts[block]+=$3
}
END {
  # -coverpkg=./... repeats blocks across test binaries: merge counts first.
  for (block in weights) {
    pkg=block
    sub(/:[0-9]+[.][0-9]+,[0-9]+[.][0-9]+$/, "", pkg)
    sub(/\/[^/]+$/, "", pkg)
    statements[pkg]+=weights[block]
    if (counts[block] > 0) covered[pkg]+=weights[block]
  }
  for (pkg in statements) {
    # The vector generator is offline fixture tooling, not shipped runtime code.
    if (pkg == module "/protocol/cmd/vectors") continue
    if (pkg == module "/cmd/inkwell" || pkg == module "/internal/transport/transporttest") continue
    if (!statements[pkg]) continue
    all_statements+=statements[pkg]; all_covered+=covered[pkg]
    pct=100*covered[pkg]/statements[pkg]
    printf "%s: %.1f%% (minimum %s%%)\n", pkg, pct, min
    if (pct < min) { failed=1; printf "✗ coverage: %s is below %s%%\n", pkg, min > "/dev/stderr" }
  }
  if (all_statements) printf "Total coverage (after exemptions): %.1f%%\n", 100*all_covered/all_statements
  if (!all_statements) { print "✗ coverage: empty profile" > "/dev/stderr"; failed=1 }
  exit failed
}' coverage.out coverage.protocol.out
