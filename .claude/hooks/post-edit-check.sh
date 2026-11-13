#!/usr/bin/env bash
# PostToolUse(Write|Edit|MultiEdit): checks the edited file at once, instead of at pre-commit.
# - .go: gofmt, then go vet on its package (its own module: root or protocol/);
# - .ts / .svelte / .js under frontend/: Biome;
# - CLAUDE.md, .claude/agents|skills|hooks: scripts/check_claude_md.py (rule « Cohérence de ce fichier »).
# A problem is reported back to Claude (exit 2) so it is fixed in the same step.
set -euo pipefail
input="$(cat)"
path="$(jq -r '.tool_input.file_path // empty' <<<"$input")"
[[ -n $path && -f $path ]] || exit 0
# The edited file's own checkout (a linked worktree has its own module root), not the session's.
root="$(git -C "$(dirname "$path")" rev-parse --show-toplevel 2>/dev/null)" || exit 0
rel="${path#"$root"/}"
out=""

case "$rel" in
  *.go)
    out="$(gofmt -l "$path" | sed 's/$/: not gofmt-clean (run gofumpt -w or mise run fmt)/')"
    if [[ -z $out ]]; then
      mod="$root"
      [[ $rel == protocol/* ]] && mod="$root/protocol"
      pkg="./$(dirname "${path#"$mod"/}")"
      out="$(cd "$mod" && timeout 120 go vet "$pkg" 2>&1)" || true
    fi
    ;;
  frontend/*.ts | frontend/*.svelte | frontend/*.js)
    biome="$root/frontend/node_modules/.bin/biome"
    [[ -x $biome ]] && { out="$(cd "$root/frontend" && timeout 60 "$biome" check "${path#"$root"/frontend/}" 2>&1)" || true; } \
      && [[ $out == *"Checked 1 file"* && $out != *error* ]] && out=""
    ;;
  CLAUDE.md | .claude/agents/* | .claude/skills/* | .claude/hooks/* | .claude/settings.json)
    out="$(timeout 60 python3 "$root/scripts/check_claude_md.py" "$root" 2>&1)" || true
    ;;
esac

[[ -z $out ]] && exit 0
printf 'post-edit-check: %s\n%s\n' "$rel" "$out" >&2
exit 2
