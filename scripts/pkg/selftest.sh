#!/usr/bin/env bash
# Tests of the release scripts themselves (notes, assemble, sizes, verify, on-main, release-guard), on fake assets, with a
# throwaway cosign key. Run by `mise run pkg:lint`. No network.
set -euo pipefail
# A git hook runs this with GIT_DIR and friends set: unset them, or the throwaway repos below
# would commit into the real one.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
bad() {
  printf '✗ selftest: %s\n' "$1" >&2
  fail=1
}
expect() { # expect DESCRIPTION CMD...: the command must succeed
  local what="$1"
  shift
  "$@" >"$work/out" 2>&1 || { cat "$work/out" >&2; bad "$what"; }
}
refuse() { # refuse DESCRIPTION CMD...: the command must fail
  local what="$1"
  shift
  if "$@" >"$work/out" 2>&1; then bad "$what"; fi
}

# --- notes.sh ---------------------------------------------------------------------------------
cat >"$work/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

### Features

- **x:** Not released

## [0.2.0-rc.1] - 2026-11-02

### Features

- **editor:** Search

### Bug fixes

- Fix a thing

## [0.1.0] - 2026-10-26

- Old
EOF
got="$(CHANGELOG="$work/CHANGELOG.md" scripts/pkg/notes.sh 0.2.0-rc.1)"
[[ $got == $'### Features\n\n- **editor:** Search\n\n### Bug fixes\n\n- Fix a thing' ]] || bad "notes.sh returns the wrong section: $got"
refuse "notes.sh must fail on a version the changelog does not have" env CHANGELOG="$work/CHANGELOG.md" scripts/pkg/notes.sh 9.9.9
refuse "notes.sh must not match a prefix of another version" env CHANGELOG="$work/CHANGELOG.md" scripts/pkg/notes.sh 0.2.0

# --- assemble.sh, verify.sh -------------------------------------------------------------------
for arch in amd64 arm64; do
  mkdir -p "$work/in/dist-$arch"
  rpm_arch=x86_64
  [[ $arch == arm64 ]] && rpm_arch=aarch64
  for f in "inkwell_0.2.0_$arch.deb" "inkwell-0.2.0.$rpm_arch.rpm" "inkwell_0.2.0_linux_$arch.tar.gz"; do
    head -c 1000 /dev/urandom >"$work/in/dist-$arch/$f"
    echo '{"spdxVersion":"SPDX-2.3"}' >"$work/in/dist-$arch/$f.spdx.json"
  done
done
expect "assemble.sh assembles both architectures" scripts/pkg/assemble.sh "$work/in" "$work/out-dir" 0.2.0
[[ "$(wc -l <"$work/out-dir/checksums.txt")" -eq 12 ]] || bad "checksums.txt should list 12 files"
expect "verify.sh accepts the intact release" scripts/pkg/verify.sh "$work/out-dir"
rm -rf "$work/wrong"
refuse "assemble.sh must refuse assets of another version" scripts/pkg/assemble.sh "$work/in" "$work/wrong" 0.3.0
if command -v cosign >/dev/null 2>&1; then
  export COSIGN_PASSWORD="" # the throwaway key has no passphrase
  (cd "$work/out-dir" && cosign generate-key-pair >/dev/null 2>&1 &&
    cosign sign-blob --yes --key cosign.key --use-signing-config=false --tlog-upload=false \
      --bundle checksums.txt.sigstore.json checksums.txt >/dev/null 2>&1 && mv cosign.pub ../cosign.pub && rm -f cosign.key) ||
    bad "could not make a cosign test signature"
  expect "verify.sh validates a cosign bundle" env COSIGN_PUBLIC_KEY="$work/cosign.pub" scripts/pkg/verify.sh "$work/out-dir"
  refuse "verify.sh must require the tag to check a keyless signature" scripts/pkg/verify.sh "$work/out-dir"
  echo tampered >>"$work/out-dir/checksums.txt"
  refuse "verify.sh must reject a tampered checksums.txt" env COSIGN_PUBLIC_KEY="$work/cosign.pub" scripts/pkg/verify.sh "$work/out-dir"
else
  echo "selftest: cosign not installed, signature checks skipped" >&2
fi
rm -rf "$work/out-dir"
rm "$work/in/dist-arm64/inkwell_0.2.0_arm64.deb.spdx.json"
refuse "assemble.sh must refuse a package without SBOM" scripts/pkg/assemble.sh "$work/in" "$work/out-dir" 0.2.0

# --- sizes.sh ---------------------------------------------------------------------------------
printf 'inkwell_0.1.0_amd64.deb 1000\ninkwell-0.1.0.x86_64.rpm 1000\ninkwell_0.1.0_linux_amd64.tar.gz 1000\n' >"$work/old.txt"
printf 'inkwell_0.2.0_amd64.deb 1050\ninkwell-0.2.0.x86_64.rpm 1300\ninkwell_0.2.0_linux_arm64.tar.gz 5\n' >"$work/new.txt"
scripts/pkg/sizes.sh "$work/old.txt" "$work/new.txt" >"$work/sizes.out"
grep -q '::warning.*rpm-amd64' "$work/sizes.out" || bad "sizes.sh must warn about the rpm that grew 30 percent"
grep -q '::warning.*deb-amd64' "$work/sizes.out" && bad "sizes.sh must not warn about +5 percent"
grep -q 'tar.gz-arm64.*no previous release' "$work/sizes.out" || bad "sizes.sh must say when there is nothing to compare"
refuse "STRICT=1 sizes.sh must fail on growth" env STRICT=1 scripts/pkg/sizes.sh "$work/old.txt" "$work/new.txt"

# --- on-main.sh: only commits of main are released ---------------------------------------------
repo="$work/repo"
git init -q -b main "$repo"
git -C "$repo" -c user.name=t -c user.email=t@invalid commit -q --allow-empty -m on-main
on_main="$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" switch -q -c feature
git -C "$repo" -c user.name=t -c user.email=t@invalid commit -q --allow-empty -m off-main
off_main="$(git -C "$repo" rev-parse HEAD)"
onmain="$PWD/scripts/pkg/on-main.sh"
(cd "$repo" && expect "on-main.sh accepts a commit of main" "$onmain" "$on_main" main)
(cd "$repo" && refuse "on-main.sh must refuse a commit that is not on main" "$onmain" "$off_main" main)
(cd "$repo" && refuse "on-main.sh must refuse when main is unknown" "$onmain" "$on_main" nonexistent)

# --- release-guard.sh: a published release is immutable (fake gh) -----------------------------
mkdir -p "$work/bin"
cat >"$work/bin/gh" <<'EOF2'
#!/usr/bin/env bash
case "$FAKE_RELEASE" in
none) echo "release not found" >&2; exit 1 ;;
draft) echo true ;;
published) echo false ;;
*) echo "HTTP 401" >&2; exit 1 ;;
esac
EOF2
chmod +x "$work/bin/gh"
guard() { PATH="$work/bin:$PATH" FAKE_RELEASE="$1" scripts/pkg/release-guard.sh v0.2.0; }
[[ "$(guard none)" == create ]] || bad "release-guard.sh must say create when there is no release"
[[ "$(guard draft)" == draft ]] || bad "release-guard.sh must say draft for a draft"
refuse "release-guard.sh must refuse a published release" guard published
refuse "release-guard.sh must refuse when gh fails for another reason" guard error

if ((fail)); then exit 1; fi
echo "✓ selftest: notes, assemble, verify, sizes, on-main and release-guard behave"
