#!/usr/bin/env bash
# Hosted open-weight models (OpenRouter) for MECHANICAL text work, so Claude's
# context and tokens go to design, code and review. Never used for security,
# architecture or code review judgement (CLAUDE.md routing table).
#
#   scripts/llm.sh commit-msg          # Conventional Commit draft from the staged diff
#   scripts/llm.sh triage < log.txt    # failing items only, from a long CI/test log
#   scripts/llm.sh translate en < fr.json   # i18n JSON translation draft (keys kept)
#   scripts/llm.sh summarize 15 < report    # a long report or log in at most N lines
#   scripts/llm.sh prose < text.md          # French copy-edit: plain, short, same meaning
#   scripts/llm.sh check                    # exit 0 when OpenRouter answers, 3 otherwise
#
# Needs OPENROUTER_API_KEY (put it in mise.local.toml [env] — gitignored — or your
# shell profile, never in the repo). Exit 3 when unavailable: callers fall back to
# their Claude tier. Every request enforces zero data retention (provider.zdr) and
# skips fp4-quantised endpoints. Model choice and evidence: docs/adr/0013-*.md.
set -euo pipefail

: "${LLM_MECHANICAL_MODEL:=deepseek/deepseek-v4.1-flash-1017}"
: "${LLM_WRITING_MODEL:=mistralai/mistral-large-4-0}"
: "${LLM_MAX_INPUT_BYTES:=200000}"

if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  echo "llm.sh: OPENROUTER_API_KEY not set — fall back to the Claude tier." >&2
  exit 3
fi

# ask MODEL SYSTEM_PROMPT < input  → prints the completion text.
ask() {
  local model="$1" system="$2" input
  input="$(head -c "$LLM_MAX_INPUT_BYTES")"
  jq -n --arg model "$model" --arg system "$system" --arg input "$input" '{
      model: $model,
      temperature: 0.2,
      provider: {zdr: true, quantizations: ["fp8", "bf16"], require_parameters: true},
      messages: [{role: "system", content: $system}, {role: "user", content: $input}]
    }' |
    curl -fsS --max-time 120 https://openrouter.ai/api/v1/chat/completions \
      -H "Authorization: Bearer $OPENROUTER_API_KEY" \
      -H "Content-Type: application/json" \
      -H "X-Title: inkwell-dev" \
      --data-binary @- |
    jq -er '.choices[0].message.content'
}

case "${1:-}" in
  commit-msg)
    git diff --cached --stat --patch | ask "$LLM_MECHANICAL_MODEL" \
      "Write one Conventional Commit message (type(scope): subject ≤ 72 chars, blank line, short body of why/what as bullets) for this staged diff. Output only the message."
    ;;
  triage)
    ask "$LLM_MECHANICAL_MODEL" \
      "From this build/test log, list ONLY the failing items: file:line, test or check name, and the exact error line. No commentary. If nothing failed, output: OK"
    ;;
  translate)
    lang="${2:?usage: llm.sh translate <lang>}"
    ask "$LLM_WRITING_MODEL" \
      "Translate the values of this i18n JSON into ${lang}. Keep keys, placeholders like {name} and JSON structure identical. Plain, friendly UI tone, no jargon. Output only JSON."
    ;;
  summarize)
    lines="${2:-15}"
    ask "$LLM_MECHANICAL_MODEL" \
      "Summarize this text in at most ${lines} lines. Keep every number, file path, test name, failure and decision exactly; drop narration. Output only the summary."
    ;;
  prose)
    ask "$LLM_WRITING_MODEL" \
      "Copy-edit this French text: plain words, short sentences, active voice, no jargon, same meaning and same Markdown structure. Do not add content. Output only the text."
    ;;
  check)
    [[ "$(echo ping | ask "$LLM_MECHANICAL_MODEL" "Answer with the single word: pong" 2>/dev/null)" == *pong* ]] || exit 3
    ;;
  *)
    sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
    exit 2
    ;;
esac
