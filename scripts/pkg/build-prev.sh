#!/usr/bin/env bash
# Builds a throwaway "previous" package (version 0.0.0-a.1, older than any real one) from this tree
# into OUT_DIR, for the upgrade test (PKG-08) when there is no previous release to download. The
# current dist/ is set aside and restored, whatever happens.
#
# Usage: build-prev.sh OUT_DIR
set -euo pipefail

out="${1:?usage: build-prev.sh OUT_DIR}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

aside="$(mktemp -d)"
restore() {
  rm -rf dist
  [[ ! -d $aside/dist ]] || mv "$aside/dist" dist
  rm -rf "$aside"
}
trap restore EXIT
[[ ! -d dist ]] || mv dist "$aside/dist"

PKG_VERSION=0.0.0-a.1 scripts/pkg/build.sh >"$aside/build.log" 2>&1 || { tail -30 "$aside/build.log" >&2; exit 1; }
mkdir -p "$out"
find dist -maxdepth 1 \( -name '*.deb' -o -name '*.rpm' \) -exec cp {} "$out/" \;
echo "build-prev: $(find "$out" -maxdepth 1 -type f | wc -l) packages of 0.0.0-a.1 in $out"
