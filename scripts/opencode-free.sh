#!/usr/bin/env bash
# Free agent: opencode with a free model (no credit). gortex is configured in
# ~/.config/opencode/opencode.json (XDG_RUNTIME_DIR passed); opencode has no rtk hook, so the
# prompt is prefixed with the rule. Usage: scripts/opencode-free.sh [-m model] [-w dir] < prompt.md
# Default: OpenRouter free Nemotron Ultra (1000 req/day; run under `mise x --` for OPENROUTER_API_KEY).
# Also working with tools (2026-11-08): openrouter/cohere/north-mini-code:free; OpenCode Zen
# opencode/deepseek-v4.1-flash-free (small free quota). Free endpoints may log prompts: never send
# secrets or security work.
set -euo pipefail
model=openrouter/nvidia/nemotron-3-ultra-550b-a55b:free
dir=$PWD
while getopts "m:w:" o; do
  case $o in
    m) model=$OPTARG ;;
    w) dir=$OPTARG ;;
    *) echo "usage: $0 [-m model] [-w dir] < prompt" >&2; exit 2 ;;
  esac
done
bin=${OPENCODE_BIN:-$HOME/.local/opencode/node_modules/.bin/opencode}
[[ -x $bin ]] || { echo "opencode-free: $bin not found (npm i --prefix ~/.local/opencode opencode-ai)" >&2; exit 3; }
if [[ $model == openrouter/* && -z ${OPENROUTER_API_KEY:-} ]]; then
  echo "opencode-free: OPENROUTER_API_KEY unset; run under \`mise x --\` (key in mise.local.toml)" >&2; exit 3
fi
# shellcheck disable=SC2016 # literal backticks for the model, no expansion wanted
rules='Prefix every shell command with `rtk` (for example `rtk go test ./...`). Navigate code with the gortex tools. Never commit or stash.'
cd "$dir"
# The same skills live in ~/.claude/skills and ~/.agents/skills; read only the cross-agent copy,
# which avoids opencode's "duplicate skill name" warnings without losing a skill.
export OPENCODE_DISABLE_CLAUDE_CODE_SKILLS=1
exec "$bin" run -m "$model" --print-logs "$rules"$'\n\n'"$(cat)"
