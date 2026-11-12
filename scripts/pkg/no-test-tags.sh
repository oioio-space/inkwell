#!/usr/bin/env bash
# Gate AC-14 (docs/architecture/decisions.md, AD-22): a published executable was not built with a
# test tag. `-tags server` (the headless end-to-end build: a local server, the netsim filler) and
# `-tags faultinject` must never reach a release (INV-5: no local server in production).
#
# Usage: no-test-tags.sh EXECUTABLE...   (reads the build settings with `go version -m`)
set -euo pipefail

((${#} > 0)) || {
  echo "usage: no-test-tags.sh EXECUTABLE..." >&2
  exit 2
}
fail=0
for bin in "$@"; do
  info="$(go version -m "$bin")" || {
    echo "✗ no-test-tags: cannot read the build settings of $bin" >&2
    fail=1
    continue
  }
  tags="$(sed -n 's/^[[:space:]]*build[[:space:]]*-tags=\(.*\)$/\1/p' <<<"$info")"
  for t in server faultinject; do
    if [[ ",$tags," == *",$t,"* ]]; then
      echo "✗ no-test-tags: $bin was built with -tags $t (tags: $tags)" >&2
      fail=1
    fi
  done
done
((fail == 0)) || exit 1
echo "✓ no-test-tags: no executable carries a test tag"
