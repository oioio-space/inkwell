#!/usr/bin/env bash
# Builds the executable, the tar.gz, the .deb and the .rpm of THIS machine's architecture with
# goreleaser (cgo: native build only, ADR 0003 and 0025), into dist/.
#
#   build.sh            local build: a snapshot, version PKG_VERSION (default 0.0.0-dev.<commit>)
#   build.sh --release  what the release workflow runs on a tag: the version is the tag's, nothing
#                       is published (the workflow assembles both architectures and publishes)
#
# Signing: the .deb and .rpm are signed with the OpenPGP key whose path is in NFPM_KEY_FILE (and
# its passphrase in NFPM_PASSPHRASE); without a key they are built unsigned.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

args=(release --clean)
if [[ "${1:-}" == "--release" ]]; then
  args+=("--skip=publish,announce")
else
  PKG_VERSION="${PKG_VERSION:-0.0.0-dev.$(git rev-parse --short HEAD)}"
  export PKG_VERSION # read by snapshot.version_template in .goreleaser.yaml
  args+=(--snapshot)
fi

if [[ -n "${NFPM_KEY_FILE:-}" ]]; then
  echo "build: signing the packages with ${NFPM_KEY_FILE}"
else
  echo "build: no NFPM_KEY_FILE, the .deb and .rpm are not signed" >&2
fi

exec goreleaser "${args[@]}"
