#!/usr/bin/env bash
# Size watch (PKG-12): compares the sizes of this release (sizes.txt, written by assemble.sh) with
# the previous release's and warns about any asset that grew by more than THRESHOLD percent
# (default 10). Assets are matched by kind and architecture, not by name (the version is in it).
# A warning is a GitHub annotation in CI; the exit status is 0 unless STRICT=1.
#
# Usage: sizes.sh OLD_SIZES NEW_SIZES
set -euo pipefail

old="${1:?usage: sizes.sh OLD_SIZES NEW_SIZES}"
new="${2:?usage: sizes.sh OLD_SIZES NEW_SIZES}"
threshold="${THRESHOLD:-10}"

# "inkwell_0.1.0_amd64.deb 123" -> "deb-amd64 123" ; the key survives version changes.
key() {
  awk '{
    name = $1
    kind = (name ~ /\.deb$/) ? "deb" : (name ~ /\.rpm$/) ? "rpm" : "tar.gz"
    arch = (name ~ /(amd64|x86_64)/) ? "amd64" : "arm64"
    print kind "-" arch, $2
  }' "$1"
}

declare -A before
while read -r k size; do before[$k]=$size; done < <(key "$old")

status=0
while read -r k size; do
  prev="${before[$k]:-}"
  [[ -n $prev && $prev -gt 0 ]] || { echo "sizes: $k = $size bytes (no previous release)"; continue; }
  growth=$(((size - prev) * 100 / prev))
  line="sizes: $k = $size bytes, was $prev (${growth}%)"
  if ((growth > threshold)); then
    echo "::warning title=Package size::$line, above +${threshold}%"
    status=1
  else
    echo "$line"
  fi
done < <(key "$new")

[[ ${STRICT:-} == 1 ]] && exit "$status"
exit 0
