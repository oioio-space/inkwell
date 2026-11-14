# shellcheck shell=bash
# Source me: puts the mise-pinned tools first on PATH, whatever the order of the caller's PATH.
#
# Why: `mise run` / `mise x --` add the pinned tool folders to PATH but do not move them ahead of
# folders already there, and a stale `go install` build in $GOPATH/bin (gosec 2.29.0, "Version:
# dev") shadowed the pinned gosec 2.30.0 locally while the GitHub job ran the pinned one: two
# findings passed the local gate for weeks. A script that calls a pinned tool by bare name sources
# this file first. Without mise (a plain shell) PATH is left alone.
if command -v mise >/dev/null 2>&1; then
  pinned_paths="$(mise bin-paths 2>/dev/null | paste -sd: -)"
  [[ -z "$pinned_paths" ]] || export PATH="$pinned_paths:$PATH"
  unset pinned_paths
fi
