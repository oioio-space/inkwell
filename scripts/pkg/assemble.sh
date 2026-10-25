#!/usr/bin/env bash
# Assembles the release assets of both architectures (ADR 0025): copies the archives, packages and
# SBOMs that the per-architecture builds produced into OUT_DIR, then writes
#   checksums.txt   SHA-256 of every asset (this is what cosign signs)
#   sizes.txt       "<asset> <bytes>" per executable-bearing asset, for the size watch (PKG-12)
#
# Usage: assemble.sh IN_DIR OUT_DIR       (IN_DIR holds the downloaded per-architecture artifacts)
set -euo pipefail

in="${1:?usage: assemble.sh IN_DIR OUT_DIR}"
out="${2:?usage: assemble.sh IN_DIR OUT_DIR}"

mkdir -p "$out"
count=0
while IFS= read -r -d '' f; do
  cp "$f" "$out/"
  count=$((count + 1))
done < <(find "$in" -type f \( -name '*.deb' -o -name '*.rpm' -o -name '*.tar.gz' -o -name '*.spdx.json' -o -name 'inkwell-signing-key.asc' \) -print0)

[[ $count -gt 0 ]] || { echo "assemble.sh: no asset found in $in" >&2; exit 1; }
for kind in '*.deb' '*.rpm' '*.tar.gz'; do
  n="$(find "$out" -maxdepth 1 -name "$kind" | wc -l)"
  [[ $n -eq 2 ]] || { echo "assemble.sh: expected 2 $kind (amd64 and arm64), found $n" >&2; exit 1; }
done
# Each archive and package has its SBOM.
for f in "$out"/*.deb "$out"/*.rpm "$out"/*.tar.gz; do
  [[ -s "$f.spdx.json" ]] || { echo "assemble.sh: no SBOM for $(basename "$f")" >&2; exit 1; }
done

(cd "$out" && find . -maxdepth 1 -type f ! -name 'checksums.txt' ! -name 'sizes.txt' -printf '%f\n' | LC_ALL=C sort | xargs sha256sum >checksums.txt)
(cd "$out" && for f in *.deb *.rpm *.tar.gz; do printf '%s %s\n' "$f" "$(stat -c %s "$f")"; done >sizes.txt)

echo "assembled $count files in $out:"
cat "$out/sizes.txt"
