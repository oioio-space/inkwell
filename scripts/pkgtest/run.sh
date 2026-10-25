#!/usr/bin/env bash
# Install tests of the packages in dist/ on fresh distribution containers (podman), PKG-01…PKG-11:
#   debian:13 and ubuntu:24.04 (.deb), fedora:44 (.rpm).
# Each distribution goes through: tools -> install (+checks) -> smoke (offline, headless) ->
# uninstall -> upgrade from a previous package. The in-container checks are in inside.sh.
#
# Environment:
#   PKG_DIST=dist             where the packages are
#   PKG_DISTROS="debian:13 ubuntu:24.04 fedora:44"   which distributions (space separated)
#   PKG_ARCH=native           or arm64/amd64: a foreign architecture needs qemu-user-static
#                             (binfmt) AND packages built for it (cgo: only a native build works),
#                             else the run is skipped with the reason
#   PKG_PREV_DIST=            directory of the PREVIOUS release's packages for the upgrade test;
#                             unset: a package of version 0.0.0-a.1 is built from this tree (cached
#                             in .task/pkgtest, PKG_REBUILD_PREV=1 forces)
#   PKG_SIGN=1                rebuild with a throwaway OpenPGP key and check the signatures too
#   PKG_PUBKEY=public.asc     the packages are already signed: check them against this public key
#   PKG_KEEP=1                keep the containers' images (inspect with `podman run -it IMAGE`)
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
root="$PWD"
dist="${PKG_DIST:-dist}"
work="$root/.task/pkgtest"
mkdir -p "$work"

native="$(uname -m)"
case "${PKG_ARCH:-native}" in
native) [[ $native == x86_64 ]] && want=amd64 || want=arm64 ;;
amd64 | x86_64) want=amd64 ;;
arm64 | aarch64) want=arm64 ;;
*) echo "pkg:test: unknown PKG_ARCH=$PKG_ARCH" >&2; exit 64 ;;
esac
[[ $want == amd64 ]] && rpm_arch=x86_64 || rpm_arch=aarch64
[[ $native == x86_64 ]] && native_arch=amd64 || native_arch=arm64

command -v podman >/dev/null 2>&1 || { echo "pkg:test: podman is required" >&2; exit 2; }

if [[ $want != "$native_arch" ]]; then
  if [[ ! -e /proc/sys/fs/binfmt_misc/qemu-$([[ $want == arm64 ]] && echo aarch64 || echo x86_64) ]]; then
    echo "skip: PKG_ARCH=$want on a $native_arch machine needs qemu-user-static (binfmt) and none is registered"
    echo "      (install qemu-user-static; the packages themselves must be built on a $want machine: cgo)"
    exit 0
  fi
fi

# --- the packages -----------------------------------------------------------------------------
deb="$(find "$dist" -maxdepth 1 -name "inkwell_*_${want}.deb" | head -1)"
rpm="$(find "$dist" -maxdepth 1 -name "inkwell-*.${rpm_arch}.rpm" | head -1)"
if [[ -z $deb || -z $rpm ]]; then
  echo "pkg:test: no $want .deb/.rpm in $dist/ — run \`mise run pkg:build\` on a $want machine first" >&2
  exit 2
fi
version="$(basename "$deb" .deb)"
version="${version#inkwell_}"
version="${version%_"${want}"}"

stage() { # stage DIR DEB RPM [PUBKEY]: the directory mounted in the containers
  rm -rf "$1" && mkdir -p "$1"
  cp "$2" "$3" "$1/"
  [[ -z ${4:-} ]] || cp "$4" "$1/public.asc"
}

signed=0
pubkey=""
if [[ ${PKG_SIGN:-} == 1 ]]; then
  command -v gpg >/dev/null 2>&1 || { echo "pkg:test: PKG_SIGN=1 needs gpg" >&2; exit 2; }
  gnupg="$work/gnupg"
  rm -rf "$gnupg" && mkdir -p "$gnupg" && chmod 700 "$gnupg"
  GNUPGHOME="$gnupg" gpg --batch --quiet --passphrase '' --quick-generate-key 'Inkwell pkg:test <pkgtest@invalid>' rsa3072 sign 1d
  GNUPGHOME="$gnupg" gpg --batch --quiet --armor --export-secret-keys >"$work/secret.asc"
  GNUPGHOME="$gnupg" gpg --batch --quiet --armor --export >"$work/public.asc"
  export NFPM_KEY_FILE="$work/secret.asc" NFPM_PASSPHRASE=""
  echo "pkg:test: rebuilding $version signed with a throwaway key"
  PKG_VERSION="$version" scripts/pkg/build.sh >"$work/build-signed.log" 2>&1 || { tail -20 "$work/build-signed.log"; exit 1; }
  signed=1
  pubkey="$work/public.asc"
elif [[ -n ${PKG_PUBKEY:-} ]]; then
  signed=1 # signed elsewhere (the release workflow): check the signatures with this public key
  pubkey="$PKG_PUBKEY"
fi

cur="$work/cur-$want"
stage "$cur" "$deb" "$rpm" "$pubkey"

# --- the previous release, for the upgrade ----------------------------------------------------
prev="$work/prev-$want"
if [[ -z ${PKG_PREV_DIST:-} ]]; then
  raw="$work/prev-raw-$want"
  if [[ ! -f $raw/.built || ${PKG_REBUILD_PREV:-} == 1 || $signed == 1 ]]; then
    if [[ $want != "$native_arch" ]]; then
      echo "pkg:test: the previous package for $want must come from PKG_PREV_DIST (cgo: no cross build)" >&2
      exit 2
    fi
    echo "pkg:test: building the previous package (0.0.0-a.1) for the upgrade test"
    rm -rf "$raw"
    scripts/pkg/build-prev.sh "$raw"
    touch "$raw/.built"
  fi
  PKG_PREV_DIST="$raw"
fi
stage "$prev" "$(find "$PKG_PREV_DIST" -maxdepth 1 -name "inkwell_*_${want}.deb" | head -1)" \
  "$(find "$PKG_PREV_DIST" -maxdepth 1 -name "inkwell-*.${rpm_arch}.rpm" | head -1)" "$pubkey"
prev_deb="$(basename "$(find "$prev" -maxdepth 1 -name '*.deb' | head -1)")"
prev_rpm="$(basename "$(find "$prev" -maxdepth 1 -name '*.rpm' | head -1)")"

# --- one distribution -------------------------------------------------------------------------
podman_arch=()
[[ $want != "$native_arch" ]] && podman_arch=(--arch "$want")
failed=()

run_distro() {
  local distro="$1" image family pkg prevpkg
  case "$distro" in
  debian:* | ubuntu:*)
    image="docker.io/library/$distro" family=deb pkg="$(basename "$deb")" prevpkg="$prev_deb"
    ;;
  fedora:*)
    image="registry.fedoraproject.org/fedora:${distro#fedora:}" family=rpm pkg="$(basename "$rpm")" prevpkg="$prev_rpm"
    ;;
  *)
    echo "pkg:test: unknown distribution $distro" >&2
    return 1
    ;;
  esac
  local slug="${distro//[:.]/-}-$want" tag log
  tag="localhost/inkwell-pkgtest-$slug"
  log="$work/$slug.log"
  local envs=(-e "PKG_FILE=$pkg" -e "EXPECT_VERSION=$version" -e "PKG_PREV_FILE=$prevpkg" -e "PKG_SIGNED=$signed")
  local mounts=(-v "$cur:/pkg:ro,z" -v "$prev:/prev:ro,z" -v "$root/scripts/pkgtest:/t:ro,z")

  # phase NAME FROM_IMAGE COMMIT_AS(or "-") [podman options...]: runs `inside.sh NAME` in a
  # throwaway container started from FROM_IMAGE; the container becomes image COMMIT_AS.
  phase() {
    local name="$1" from="$2" commit="$3" c="inkwell-pkgtest-$slug-$1" rc=0
    shift 3
    podman rm -f "$c" >/dev/null 2>&1 || true
    printf '\n== %s / %s ==\n' "$distro" "$name" >>"$log"
    podman run --name "$c" "${podman_arch[@]}" "$@" "${envs[@]}" "${mounts[@]}" "$from" bash /t/inside.sh "$name" >>"$log" 2>&1 || rc=$?
    if [[ $commit != - ]]; then podman commit -q "$c" "$commit" >/dev/null; fi
    podman rm -f "$c" >/dev/null 2>&1 || true
    return "$rc"
  }

  : >"$log"
  local rc=0
  echo "── $distro ($family, $want) ──"
  if phase tools "$image" "$tag:tools"; then
    # The installation commits its container: the offline smoke test and the uninstallation start
    # from it, even when one of its checks failed (they are independent findings).
    phase install "$tag:tools" "$tag:installed" || rc=1
    phase smoke "$tag:installed" - --network none || rc=1
    phase uninstall "$tag:installed" - || rc=1
    phase upgrade "$tag:tools" - || rc=1
  else
    rc=1
  fi
  grep -E '^(ok|FAIL|skip) ' "$log" | sed 's/^/   /'
  if [[ ${PKG_KEEP:-} != 1 ]]; then
    podman rmi -f "$tag:tools" "$tag:installed" >/dev/null 2>&1 || true
  fi
  [[ $rc -eq 0 ]] || failed+=("$distro")
}

for distro in ${PKG_DISTROS:-debian:13 ubuntu:24.04 fedora:44}; do
  run_distro "$distro"
done

echo
if ((${#failed[@]})); then
  echo "✗ pkg:test: failed on: ${failed[*]} (logs in $work/*.log)"
  exit 1
fi
echo "✓ pkg:test: $version on ${PKG_DISTROS:-debian:13 ubuntu:24.04 fedora:44} ($want)"
