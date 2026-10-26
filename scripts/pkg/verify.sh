#!/usr/bin/env bash
# Verifies a downloaded release the way a user would (PKG-11, ADR 0025):
#   1. the checksums match the files;
#   2. checksums.txt.sigstore.json is a valid cosign bundle for checksums.txt, made by this
#      repository's release workflow (keyless: the identity is the workflow, not a key);
#   3. every archive and package has a parseable SPDX SBOM.
# When the release was built unsigned (a dry run), step 2 is reported as skipped.
#
# Usage: verify.sh DIR [TAG]        TAG (e.g. v0.1.0-rc.1) is required to check a keyless signature:
#                                   the signing identity is pinned to that tag, to a push event and
#                                   to this repository
# Environment: COSIGN_PUBLIC_KEY=path checks a bundle made with a key instead (local tests);
#              REPO=owner/name overrides the repository the identity must come from.
set -euo pipefail

dir="${1:?usage: verify.sh DIR [TAG]}"
tag="${2:-}"
repo="${REPO:-oioio-space/inkwell}"
cd "$dir"
fail=0
bad() {
  printf '✗ verify: %s\n' "$1" >&2
  fail=1
}

# 1. checksums
if sha256sum --check --strict --quiet checksums.txt; then
  echo "✓ checksums: $(wc -l <checksums.txt) files match"
else bad "the checksums do not match"; fi

# 2. signature
if [[ -f checksums.txt.sigstore.json ]]; then
  command -v cosign >/dev/null 2>&1 || { bad "cosign is required to verify the signature (mise install cosign)"; exit 1; }
  if [[ -n ${COSIGN_PUBLIC_KEY:-} ]]; then
    ident=(--key "$COSIGN_PUBLIC_KEY" --insecure-ignore-tlog=true) # a key made offline is not in the log
    signer="the key $COSIGN_PUBLIC_KEY"
  else
    [[ -n $tag ]] || { bad "the tag is required to verify the signature (verify.sh DIR vX.Y.Z)"; exit 1; }
    ident=(--certificate-identity "https://github.com/$repo/.github/workflows/release.yml@refs/tags/$tag"
      --certificate-oidc-issuer https://token.actions.githubusercontent.com
      --certificate-github-workflow-trigger push
      --certificate-github-workflow-repository "$repo"
      --certificate-github-workflow-ref "refs/tags/$tag")
    signer="the release workflow of $repo"
  fi
  if cosign verify-blob "${ident[@]}" --bundle checksums.txt.sigstore.json checksums.txt >/dev/null 2>&1; then
    echo "✓ cosign: checksums.txt is signed by $signer"
  else bad "cosign does not validate checksums.txt.sigstore.json"; fi
else
  echo "- cosign: no checksums.txt.sigstore.json (an unsigned dry run?)"
fi

# 3. SBOMs
for f in *.deb *.rpm *.tar.gz; do
  [[ -e $f ]] || continue
  if [[ ! -s $f.spdx.json ]] || ! grep -q '"spdxVersion"' "$f.spdx.json"; then bad "no valid SPDX SBOM for $f"; fi
done
((fail)) || echo "✓ sbom: every archive and package has its SPDX document"

exit "$fail"
