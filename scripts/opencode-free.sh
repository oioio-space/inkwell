#!/usr/bin/env bash
# Free agent: opencode with a free OpenCode Zen model (no credit). gortex is configured in
# ~/.config/opencode/opencode.json (XDG_RUNTIME_DIR passed); opencode has no rtk hook, so the
# prompt is prefixed with the rule. Usage: scripts/opencode-free.sh [-m model] [-w dir] < prompt.md
# Free models seen working with tools (2026-11-08): deepseek-v4.1-flash-free (default),
# glm-5.3-flash-free. Free endpoints may log prompts: never send secrets or security work.
set -euo pipefail
model=opencode/deepseek-v4.1-flash-free
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
# shellcheck disable=SC2016 # literal backticks for the model, no expansion wanted
rules='Prefix every shell command with `rtk` (for example `rtk go test ./...`). Navigate code with the gortex tools. Never commit or stash.'
cd "$dir"
exec "$bin" run -m "$model" --print-logs "$rules"$'\n\n'"$(cat)"
