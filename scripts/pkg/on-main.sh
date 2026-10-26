#!/usr/bin/env bash
# Refuses to release a commit that is not in the history of main (ADR 0025): a tag cut on a branch
# that was never reviewed and merged must not publish anything.
#
# Usage: on-main.sh COMMIT [MAIN_REF]        MAIN_REF defaults to origin/main
set -euo pipefail

commit="${1:?usage: on-main.sh COMMIT [MAIN_REF]}"
main_ref="${2:-origin/main}"

git rev-parse --verify --quiet "$main_ref^{commit}" >/dev/null ||
  { echo "on-main.sh: $main_ref does not exist (fetch the full history)" >&2; exit 1; }
git merge-base --is-ancestor "$commit" "$main_ref" ||
  { echo "on-main.sh: $commit is not on $main_ref: merge it before tagging" >&2; exit 1; }
