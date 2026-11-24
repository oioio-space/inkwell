#!/usr/bin/env bash
# Port a block worktree's uncommitted work onto the main tree's index (2026-11-24).
# Usage: scripts/port-worktree.sh <worktree> [patch-file]
# The patch is the worktree's tree against the worktree's OWN HEAD, never against a commit of main: a diff from a
# main commit to a worktree that started earlier carries the reversal of every main commit made since (W80, W81 and
# a PROGRESS note were lost that way in 4e9f833, restored in 2a1e286). It is applied on main with --3way, so the main
# commits made meanwhile are kept and true conflicts are shown. The regenerated icons (build/darwin/icons.icns,
# build/windows/icon.ico) are left out, as CLAUDE.md « Pièges connus » asks. The patch is kept in
# ~/inkwell-archive/<date>/ unless a path is given.
set -euo pipefail
wt=${1:-}
[[ $# -le 2 && -n $wt && -d $wt ]] || { echo "usage: $0 <worktree> [patch-file]" >&2; exit 2; }
main=$(git rev-parse --show-toplevel)
wt=$(cd "$wt" && git rev-parse --show-toplevel)
[[ $wt != "$main" ]] || { echo "port-worktree: run it from the main tree, not from the worktree" >&2; exit 2; }
[[ $(git -C "$wt" rev-parse --git-common-dir) -ef $(git -C "$main" rev-parse --git-common-dir) ]] ||
  { echo "port-worktree: $wt is not a worktree of this repository" >&2; exit 2; }
archive="$HOME/inkwell-archive/$(date +%F)"
mkdir -p "$archive"
patch=${2:-$archive/port-$(basename "$wt")-$(date +%H%M%S).patch}  # kept: the archive of what was ported
git -C "$wt" add -A
tree=$(git -C "$wt" write-tree)
git -C "$wt" diff --binary HEAD "$tree" -- . ':!build/darwin/icons.icns' ':!build/windows/icon.ico' > "$patch"
if [[ ! -s $patch ]]; then echo "port-worktree: nothing to port from $wt" >&2; exit 0; fi
echo "port-worktree: $(git -C "$wt" diff --stat HEAD "$tree" | tail -1) from $(git -C "$wt" rev-parse --short HEAD); patch kept at $patch" >&2
status=0
git -C "$main" apply --3way --index "$patch" || status=$?
mapfile -t conflicts < <(git -C "$main" diff --name-only --diff-filter=U)
if [[ ${#conflicts[@]} -gt 0 ]]; then
  echo "port-worktree: conflicts to resolve by union (keep both sides' rows):" >&2
  printf '  %s\n' "${conflicts[@]}" >&2
  exit 1
fi
exit "$status"
