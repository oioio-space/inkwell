#!/usr/bin/env bash
# A block's worktree, usable at once (block CI3a, 2026-11-24): `../inkwell-<block>` on a new branch `<block>` from
# [base] (default main), its mise.toml trusted, the main tree's untracked mise.local.toml (API keys) linked in (a
# symlink, checked to stay ignored by git: a secret is never copied into the tree), and the pinned sidecars fetched
# (`mise run sidecar:fetch`), then the pinned tools, the frontend's packages and the Wails bindings (`mise install`,
# `mise run fe:install`, `mise run fe:bindings`: without them a worktree's frontend criteria failed on a missing biome,
# node_modules and bindings, OBS1 2026-11-25). Prints the path on its last line.
# Usage: scripts/worktree.sh <block> [base]
# Without it a new worktree failed three ways: mise refused its untrusted mise.toml, scripts/claude-or.sh and
# scripts/review-or.py found no OPENROUTER_API_KEY (exit 3), internal/app tests failed on « typst.gz is not in this
# build ». Refuses an existing path or branch; on any later failure removes what it created.
set -euo pipefail
block=${1:-}
base=${2:-main}
[[ $# -le 2 && $block =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || { echo "usage: $0 <block> [base]" >&2; exit 2; }
main=$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")
path=$(dirname "$main")/inkwell-$block
[[ ! -e $path && ! -L $path ]] || { echo "worktree: $path already exists" >&2; exit 1; }
! git -C "$main" show-ref --verify --quiet "refs/heads/$block" || { echo "worktree: branch $block already exists" >&2; exit 1; }
git -C "$main" rev-parse --verify --quiet "$base^{commit}" >/dev/null || { echo "worktree: unknown base $base" >&2; exit 1; }

created=
cleanup() {
  [[ -n $created ]] || return 0
  echo "worktree: failed; removing $path and branch $block" >&2
  git -C "$main" worktree remove --force "$path" || true
  git -C "$main" branch -D "$block" >/dev/null || true
}
trap cleanup EXIT

git -C "$main" worktree add -q -b "$block" "$path" "$base"
created=1
mise trust --quiet "$path/mise.toml"

secrets=$main/mise.local.toml
if [[ -f $secrets ]]; then
  git -C "$path" check-ignore -q mise.local.toml || { echo "worktree: mise.local.toml is not ignored by git in $path; no link made" >&2; exit 1; }
  ln -s "$secrets" "$path/mise.local.toml"
  [[ -z $(git -C "$path" status --porcelain --untracked-files=all -- mise.local.toml) ]] ||
    { echo "worktree: git sees the mise.local.toml link in $path" >&2; exit 1; }
  mise trust --quiet "$path/mise.local.toml"
else
  echo "worktree: no $secrets: no API keys in $path (OpenRouter reviews exit 3)" >&2
fi

(cd "$path" && mise run sidecar:fetch && mise install && mise run fe:install && mise run fe:bindings)
created=
echo "$path"
