#!/usr/bin/env bash
# Validates what the packages install on a desktop (ADR 0025, PKG-10): the desktop entry, the
# AppStream metadata (as stage.sh generates it), the MIME definition, the manual page and the
# icon set. Needs desktop-file-utils, appstream, libxml2 (xmllint), shared-mime-info and groff.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
app_id="io.github.oioio_space.Inkwell"
fail=0
bad() {
  printf '✗ pkg:lint: %s\n' "$1" >&2
  fail=1
}
need() { command -v "$1" >/dev/null 2>&1 || { printf '✗ pkg:lint: %s is required (%s)\n' "$1" "$2" >&2; exit 2; }; }

need desktop-file-validate "desktop-file-utils"
need appstreamcli "appstream"
need xmllint "libxml2-utils"
need update-mime-database "shared-mime-info"
need groff "groff"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# --- Desktop entry ------------------------------------------------------------------------------
desktop="build/linux/$app_id.desktop"
desktop-file-validate "$desktop" || bad "$desktop is not a valid desktop entry"
for key in 'Name\[fr\]=' 'Comment\[fr\]=' 'Comment=' 'StartupWMClass='"$app_id" 'Icon='"$app_id" \
  'Categories=Office;WordProcessor;TextEditor;' 'MimeType=text/x-typst;text/markdown;text/csv;'; do
  grep -q "^$key" "$desktop" || bad "$desktop lacks $key"
done

# --- AppStream, generated like the package does, for a release and for a pre-release -------------
cat >"$work/CHANGELOG.md" <<'EOF'
# Changelog

## [0.2.0] - 2026-11-02

### Features

- **editor:** Search & replace <across> the project

## [0.1.0] - 2026-10-26

### Bug fixes

- Nothing but a fix
EOF
CHANGELOG="$work/CHANGELOG.md" scripts/pkg/stage.sh 0.2.0 2026-11-02 "$work/stable" >/dev/null
CHANGELOG="$work/CHANGELOG.md" scripts/pkg/stage.sh 0.3.0-rc.1 2026-11-09 "$work/rc" >/dev/null
CHANGELOG=/nonexistent scripts/pkg/stage.sh 0.1.0-rc.1 2026-10-26 "$work/empty" >/dev/null
for dir in stable rc empty; do
  appstreamcli validate --no-net --pedantic "$work/$dir/$app_id.metainfo.xml" >"$work/appstream.log" 2>&1 ||
    { cat "$work/appstream.log" >&2; bad "AppStream metadata ($dir) does not validate"; }
done
grep -q '<li>editor: Search &amp; replace &lt;across&gt; the project</li>' "$work/stable/$app_id.metainfo.xml" ||
  bad "stage.sh does not escape the changelog entries"
[[ "$(grep -c '<release ' "$work/stable/$app_id.metainfo.xml")" == 2 ]] || bad "stage.sh must list both releases"
grep -q 'release version="0.3.0-rc.1" date="2026-11-09" type="development"' "$work/rc/$app_id.metainfo.xml" ||
  bad "stage.sh must add the pre-release on top, as a development release"
# The pre-release comes first.
[[ "$(grep -m1 '<release ' "$work/rc/$app_id.metainfo.xml")" == *'0.3.0-rc.1'* ]] || bad "the newest release must come first"
zcat "$work/stable/inkwell.1.gz" >/dev/null || bad "the manual page is not valid gzip"

# --- MIME definition: shared-mime-info itself must accept it -------------------------------------
xmllint --noout "build/linux/mime/$app_id.xml" || bad "the MIME definition is not well-formed XML"
mkdir -p "$work/mime/packages"
cp "build/linux/mime/$app_id.xml" "$work/mime/packages/"
update-mime-database "$work/mime" >"$work/mime.log" 2>&1 || { cat "$work/mime.log" >&2; bad "update-mime-database rejects the MIME definition"; }
grep -q '^text/x-typst:\*\.typ$' "$work/mime/globs" || bad "*.typ is not mapped to text/x-typst"

# --- Manual page ---------------------------------------------------------------------------------
groff -man -ww -z -Tutf8 build/linux/inkwell.1 2>"$work/groff.log" || true
if [[ -s "$work/groff.log" ]]; then cat "$work/groff.log" >&2; bad "the manual page has groff warnings"; fi

# --- Icons: every size, scalable, symbolic, for the stable and the pre-release (nightly) trees ----
for tree in icons icons-nightly; do
  base="build/linux/$tree/hicolor"
  for size in 16 24 32 48 64 128 256 512; do
    [[ -s "$base/${size}x${size}/apps/$app_id.png" ]] || bad "$base: missing ${size}x${size} icon"
  done
  [[ -s "$base/scalable/apps/$app_id.svg" ]] || bad "$base: missing scalable icon"
  [[ -s "$base/symbolic/apps/$app_id-symbolic.svg" ]] || bad "$base: missing symbolic icon"
done

# --- goreleaser configuration and the release scripts ------------------------------------------
if command -v goreleaser >/dev/null 2>&1; then
  goreleaser check >"$work/goreleaser.log" 2>&1 || { cat "$work/goreleaser.log" >&2; bad ".goreleaser.yaml is not valid"; }
else
  bad "goreleaser is required (mise install)"
fi
scripts/pkg/selftest.sh || fail=1

if ((fail)); then exit 1; fi
echo "✓ pkg:lint: desktop entry, AppStream, MIME, manual page, icons, goreleaser config and release scripts are valid"
