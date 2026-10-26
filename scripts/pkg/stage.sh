#!/usr/bin/env bash
# Prepares the files that goreleaser puts in the packages and that are not static (ADR 0025):
#   <out>/inkwell.1.gz                                    the manual page, compressed
#   <out>/io.github.oioio_space.Inkwell.metainfo.xml      AppStream, release notes from CHANGELOG.md
#
# Usage: stage.sh VERSION YYYY-MM-DD [OUT_DIR]      (OUT_DIR defaults to .task/stage)
# Environment: CHANGELOG overrides the changelog that is read (tests).
set -euo pipefail

version="${1:?usage: stage.sh VERSION YYYY-MM-DD [OUT_DIR]}"
date="${2:?usage: stage.sh VERSION YYYY-MM-DD [OUT_DIR]}"
out="${3:-.task/stage}"

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
changelog="${CHANGELOG:-CHANGELOG.md}"
[[ -f "$changelog" ]] || changelog=/dev/null
app_id="io.github.oioio_space.Inkwell"
keep=5 # releases listed in AppStream: the latest ones

mkdir -p "$out"
# -n: no name or time in the header, so the package is reproducible.
gzip -9n <build/linux/inkwell.1 >"$out/inkwell.1.gz"

# The version being built goes on top when the changelog has no section for it yet (a snapshot,
# or a tag cut before the changelog was regenerated): AppStream wants the newest release first.
listed=0
if grep -qF "## [$version]" "$changelog"; then listed=1; fi

# One <release> per "## [x.y.z] - date" section of the changelog, newest first.
releases="$(mktemp)"
trap 'rm -f "$releases"' EXIT
awk -v keep="$keep" -v cur="$version" -v curdate="$date" -v listed="$listed" '
  function esc(s) { gsub(/&/, "\\&amp;", s); gsub(/</, "\\&lt;", s); gsub(/>/, "\\&gt;", s); gsub(/"/, "\\&quot;", s); return s }
  function kind(v) { return v ~ /-/ ? "development" : "stable" }
  function open(v, d) {
    printf "    <release version=\"%s\" date=\"%s\" type=\"%s\">\n      <description>\n", esc(v), esc(d), kind(v)
  }
  function shut() {
    if (!inrel) return
    if (items) printf "        </ul>\n"
    else printf "        <p>Maintenance release.</p>\n"
    printf "      </description>\n    </release>\n"
    inrel = 0
  }
  BEGIN {
    n = 0; inrel = 0; items = 0; skip = 1
    if (!listed) {
      open(cur, curdate)
      printf "        <p>%s</p>\n      </description>\n    </release>\n", (cur ~ /-/ ? "Pre-release build." : "Maintenance release.")
      n = 1
    }
  }
  /^## \[/ {
    shut()
    v = $0; sub(/^## \[/, "", v); sub(/\].*/, "", v)
    d = $0; sub(/^.*\] - /, "", d)
    if (v == "Unreleased" || n >= keep) { skip = 1; next }
    skip = 0; n++
    open(v, d); inrel = 1; items = 0
    next
  }
  /^- / && !skip {
    t = $0; sub(/^- /, "", t); gsub(/\*\*/, "", t)
    if (!items) printf "        <ul>\n"
    items = 1
    printf "          <li>%s</li>\n", esc(t)
  }
  END { shut() }
' "$changelog" >"$releases"

awk 'FNR == NR { rel = rel $0 "\n"; next } /^@RELEASES@$/ { printf "%s", rel; next } { print }' \
  "$releases" "build/linux/$app_id.metainfo.xml.in" >"$out/$app_id.metainfo.xml"

echo "staged $out ($version, $date)"
