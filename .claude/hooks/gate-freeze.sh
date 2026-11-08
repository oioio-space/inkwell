#!/usr/bin/env bash
# PreToolUse(Write|Edit|MultiEdit|Bash): a gate gates/<bloc>.toml is frozen once committed — it is
# written before the code (definition of « fini », step 1) and never weakened to let code pass.
# Refuses an edit of a committed gate, and a shell command that writes one (sed -i, >, tee, mv, rm).
# A gate changes only when the user asks for it: they edit it themselves, or say so explicitly.
set -euo pipefail
input="$(cat)"
root="$(git -C "${CLAUDE_PROJECT_DIR:-.}" rev-parse --show-toplevel 2>/dev/null)" || exit 0
tool="$(jq -r '.tool_name // empty' <<<"$input")"

frozen() { # $1: path, absolute or relative to the root
  local rel="${1#"$root"/}"
  [[ $rel =~ ^gates/[^/]+\.toml$ ]] && git -C "$root" cat-file -e "HEAD:$rel" 2>/dev/null
}

refuse() {
  printf 'gate-freeze: %s is a committed gate, frozen (CLAUDE.md, « Définition de fini », step 1).\n' "$1" >&2
  printf 'Never weaken a gate to make code pass. If the gate itself is wrong, stop and ask the user.\n' >&2
  exit 2
}

if [[ $tool == Bash ]]; then
  cmd="$(jq -r '.tool_input.command // empty' <<<"$input")"
  # Quoted text (messages, patterns) is not a write target: strip it before matching.
  cmd="$(sed -E "s/'[^']*'//g; s/\"[^\"]*\"//g" <<<"$cmd")"
  while read -r path; do
    [[ -n $path ]] && frozen "$path" && refuse "$path"
  done < <(grep -oE '(sed -i[^;&|]*|>>?|tee( -a)?|mv|rm( -f)?|cp)[[:space:]]+[^;&|]*gates/[^[:space:];&|]+\.toml' <<<"$cmd" |
    grep -oE 'gates/[^[:space:];&|]+\.toml' || true)
  exit 0
fi

path="$(jq -r '.tool_input.file_path // empty' <<<"$input")"
[[ -n $path ]] && frozen "$path" && refuse "$path"
exit 0
