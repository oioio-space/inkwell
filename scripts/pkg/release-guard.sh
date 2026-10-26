#!/usr/bin/env bash
# Decides what the release workflow may do with the GitHub Release of TAG, and prints it:
#   create   there is no release yet
#   draft    a draft exists: its assets may be replaced
# A published release is immutable: the script fails, and nothing is uploaded to it.
#
# Usage: release-guard.sh TAG        (needs gh and a token that can read the releases)
set -euo pipefail

tag="${1:?usage: release-guard.sh TAG}"

if ! out="$(gh release view "$tag" --json isDraft --jq .isDraft 2>&1)"; then
  if [[ $out == *"release not found"* ]]; then
    echo create
    exit 0
  fi
  echo "release-guard.sh: cannot read the release $tag: $out" >&2
  exit 1
fi
if [[ $out == true ]]; then
  echo draft
else
  echo "release-guard.sh: $tag is already published and releases are immutable: cut a new version" >&2
  exit 1
fi
