#!/usr/bin/env bash
# One-shot Claude API call billed to CLAUDE_API_KEY (mise.local.toml), not the subscription quota.
# Usage: scripts/claude-api.sh [-m model] [-t max_tokens] [-e effort] < prompt.md > answer.md
# Default model: claude-opus-5-5 (reviews, security); pass -m claude-sonnet-5-5 for routine work.
set -euo pipefail
model=claude-opus-5-5
max=8000
effort=medium
while getopts "m:t:e:" o; do
  case $o in
    m) model=$OPTARG ;;
    t) max=$OPTARG ;;
    e) effort=$OPTARG ;;
    *) echo "usage: $0 [-m model] [-t max_tokens] [-e effort] < prompt" >&2; exit 2 ;;
  esac
done
key=${CLAUDE_API_KEY:-${ANTHROPIC_API_KEY:-}}
[[ -n $key ]] || { echo "claude-api: CLAUDE_API_KEY unset (mise.local.toml)" >&2; exit 3; }
body=$(jq -Rs --arg m "$model" --argjson t "$max" --arg e "$effort" \
  '{model:$m, max_tokens:$t, output_config:{effort:$e}, messages:[{role:"user", content:.}]}')
resp=$(curl -sS https://api.anthropic.com/v1/messages \
  -H @<(printf 'x-api-key: %s\n' "$key") -H "anthropic-version: 2023-06-01" -H "content-type: application/json" \
  --data-binary @- <<<"$body")
if jq -e '.error' >/dev/null <<<"$resp"; then
  jq -r '"claude-api: " + .error.type + ": " + .error.message' <<<"$resp" >&2
  exit 1
fi
jq -r '"claude-api: stop=" + (.stop_reason // "?") + " in=" + (.usage.input_tokens|tostring) + " out=" + (.usage.output_tokens|tostring)' <<<"$resp" >&2
jq -r '[.content[] | select(.type=="text") | .text] | join("")' <<<"$resp"
