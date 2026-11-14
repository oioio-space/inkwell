#!/usr/bin/env bash
# Headless Claude Code billed to OpenRouter (OPENROUTER_API_KEY, mise.local.toml), not the
# subscription. The project's hooks (rtk) apply as in an interactive session.
# Usage: scripts/claude-or.sh -t pro|flash|haiku|sonnet|opus [-e effort] [-b max_usd] [-w workdir] < prompt.md > result.json
# Tiers (cheapest that keeps quality, CLAUDE.md): pro = DeepSeek V4 Pro, code and diagnosis;
# flash = DeepSeek V4.1 Flash, mechanical work; haiku, sonnet; opus = security and final review.
# Free (:free) models failed tool calling through Claude Code (2026-11-08): use them one-shot only.
# -b is Claude Code's own estimate, which overprices non-Anthropic models; watch the real spend on
# https://openrouter.ai/api/v1/credits.
set -euo pipefail
tier=pro
effort=medium
budget=5
dir=$PWD
while getopts "t:e:b:w:" o; do
  case $o in
    t) tier=$OPTARG ;;
    e) effort=$OPTARG ;;
    b) budget=$OPTARG ;;
    w) dir=$OPTARG ;;
    *) echo "usage: $0 -t pro|flash|haiku|sonnet|opus [-e effort] [-b max_usd] [-w dir] < prompt" >&2; exit 2 ;;
  esac
done
case $tier in
  pro) model=deepseek/deepseek-v4-pro ;;
  flash) model=deepseek/deepseek-v4.1-flash ;;
  opus) model=anthropic/claude-opus-5.5 ;;
  sonnet) model=anthropic/claude-sonnet-5.5 ;;
  haiku) model=anthropic/claude-haiku-5.5 ;;
  *) echo "claude-or: unknown tier $tier" >&2; exit 2 ;;
esac
[[ -n ${OPENROUTER_API_KEY:-} ]] || { echo "claude-or: OPENROUTER_API_KEY unset (mise.local.toml)" >&2; exit 3; }
tools='Bash Read Edit Write WebFetch WebSearch'
cd "$dir"
ANTHROPIC_BASE_URL=https://openrouter.ai/api ANTHROPIC_AUTH_TOKEN=$OPENROUTER_API_KEY ANTHROPIC_API_KEY='' \
  exec claude -p --model "$model" --effort "$effort" --max-budget-usd "$budget" \
  --permission-mode acceptEdits --allowedTools "$tools" --output-format json
