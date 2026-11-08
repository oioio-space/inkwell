#!/usr/bin/env bash
# Hosted open-weight models (Groq, Cloudflare, OpenRouter) for MECHANICAL text work, so Claude's
# context and tokens go to design, code and review. Never used for security,
# architecture or code review judgement (CLAUDE.md routing table).
#
#   scripts/llm.sh commit-msg          # Conventional Commit draft from the staged diff
#   scripts/llm.sh triage < log.txt    # failing items only, from a long CI/test log
#   scripts/llm.sh translate en < fr.json   # i18n JSON translation draft (keys kept)
#   scripts/llm.sh summarize 15 < report    # a long report or log in at most N lines
#   scripts/llm.sh prose < text.md          # French copy-edit: plain, short, same meaning
#   scripts/llm.sh check                    # exit 0 when a provider answers, 3 otherwise
#
# Provider order (LLM_PROVIDERS): Groq (GROQ_API_KEY) -> Cloudflare Workers AI
# (CLOUDFLARE_API_TOKEN + CLOUDFLARE_ACCOUNT_ID) -> OpenRouter free -> OpenRouter paid
# (OPENROUTER_API_KEY). A missing key, a 429 or a 5xx falls through to the next. Keys go in
# mise.local.toml [env] — gitignored — or your shell profile, never in the repo.
# NEVER send secrets or security-sensitive material through this script: free tiers may retain or
# train on prompts (only the OpenRouter paid hop enforces zero data retention and skips
# fp4-quantised endpoints). Writing tasks (translate, prose) use OpenRouter paid only.
# Exit 3 when no provider answers: callers fall back to their Claude tier.
# Model choice and evidence: docs/adr/0013-*.md.
set -euo pipefail

: "${LLM_MECHANICAL_MODEL:=deepseek/deepseek-v4.1-flash-1017}"
: "${LLM_WRITING_MODEL:=mistralai/mistral-large-4-0}"
: "${LLM_MAX_INPUT_BYTES:=200000}"
: "${LLM_PROVIDERS:=groq cloudflare openrouter-free openrouter}"
: "${LLM_FREE_MODEL:=openai/gpt-oss-120b}"                   # Groq as is, Cloudflare as @cf/<model>
: "${LLM_OPENROUTER_FREE_MODEL:=openai/gpt-oss-120b:free}"
: "${LLM_GROQ_MAX_INPUT_BYTES:=16000}"                       # Groq allows ~8000 tokens/min: larger inputs skip it
: "${LLM_CF_MAX_INPUT_BYTES:=60000}"
: "${LLM_API_ROOT_GROQ:=https://api.groq.com/openai/v1}"
: "${LLM_API_ROOT_CLOUDFLARE:=https://api.cloudflare.com/client/v4}"
: "${LLM_API_ROOT_OPENROUTER:=https://openrouter.ai/api/v1}"

# post URL AUTH_HEADER JSON → prints the response body on HTTP 200, returns 1 otherwise (429, 5xx,
# network error) so the caller falls through. The key is passed through a header file, not argv.
post() {
  local url="$1" auth="$2" json="$3" body code
  body="$(mktemp)"
  code="$(curl -sS --max-time 120 -o "$body" -w '%{http_code}' "$url" \
    -H @<(printf '%s\n' "$auth") -H "Content-Type: application/json" -H "X-Title: inkwell-dev" \
    --data-binary "$json" 2>/dev/null)" || code=000
  if [[ "$code" == 200 ]]; then
    cat "$body"
    rm -f "$body"
    return 0
  fi
  rm -f "$body"
  echo "llm.sh: HTTP $code, trying the next provider." >&2
  return 1
}

# chat_body SYSTEM INPUT [MODEL] → OpenAI-style chat JSON (no model field when MODEL is empty).
chat_body() {
  jq -n --arg model "${3:-}" --arg system "$1" --arg input "$2" '{
      temperature: 0.2,
      messages: [{role: "system", content: $system}, {role: "user", content: $input}]
    } + (if $model == "" then {} else {model: $model} end)'
}

# try PROVIDER MODEL SYSTEM INPUT → completion on stdout; returns 1 when unavailable or empty.
try() {
  local provider="$1" model="$2" system="$3" input="$4" resp text
  case "$provider" in
    groq)
      [[ -n "${GROQ_API_KEY:-}" && ${#input} -le $LLM_GROQ_MAX_INPUT_BYTES ]] || return 1
      resp="$(post "$LLM_API_ROOT_GROQ/chat/completions" "Authorization: Bearer $GROQ_API_KEY" \
        "$(chat_body "$system" "$input" "$LLM_FREE_MODEL")")" || return 1
      text="$(jq -er '.choices[0].message.content' <<<"$resp")" || return 1
      ;;
    cloudflare)
      [[ -n "${CLOUDFLARE_API_TOKEN:-}" && -n "${CLOUDFLARE_ACCOUNT_ID:-}" && ${#input} -le $LLM_CF_MAX_INPUT_BYTES ]] || return 1
      resp="$(post "$LLM_API_ROOT_CLOUDFLARE/accounts/$CLOUDFLARE_ACCOUNT_ID/ai/run/@cf/$LLM_FREE_MODEL" \
        "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "$(chat_body "$system" "$input")")" || return 1
      text="$(jq -er '.result.choices[0].message.content // .result.response' <<<"$resp")" || return 1
      ;;
    openrouter-free)
      [[ -n "${OPENROUTER_API_KEY:-}" ]] || return 1
      resp="$(post "$LLM_API_ROOT_OPENROUTER/chat/completions" "Authorization: Bearer $OPENROUTER_API_KEY" \
        "$(chat_body "$system" "$input" "$LLM_OPENROUTER_FREE_MODEL")")" || return 1
      text="$(jq -er '.choices[0].message.content' <<<"$resp")" || return 1
      ;;
    openrouter)
      [[ -n "${OPENROUTER_API_KEY:-}" ]] || return 1
      resp="$(post "$LLM_API_ROOT_OPENROUTER/chat/completions" "Authorization: Bearer $OPENROUTER_API_KEY" \
        "$(chat_body "$system" "$input" "$model" |
          jq '. + {provider: {zdr: true, quantizations: ["fp8", "bf16"], require_parameters: true}}')")" || return 1
      text="$(jq -er '.choices[0].message.content' <<<"$resp")" || return 1
      ;;
    *) return 1 ;;
  esac
  [[ -n "$text" ]] || return 1
  printf '%s\n' "$text"
}

# ask MODEL SYSTEM_PROMPT [writing] < input → prints the completion text; exit 3 when no provider
# answers. "writing" (copy-edit, translation) skips the free chain: OpenRouter paid only (ADR 0013).
ask() {
  local model="$1" system="$2" kind="${3:-}" input provider
  input="$(head -c "$LLM_MAX_INPUT_BYTES")"
  for provider in $LLM_PROVIDERS; do
    if [[ "$kind" == writing && "$provider" != openrouter ]]; then continue; fi
    if try "$provider" "$model" "$system" "$input"; then return 0; fi
  done
  echo "llm.sh: no provider answered (GROQ_API_KEY, CLOUDFLARE_API_TOKEN + CLOUDFLARE_ACCOUNT_ID, OPENROUTER_API_KEY): fall back to the Claude tier." >&2
  exit 3
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
      "Translate the values of this i18n JSON into ${lang}. Keep keys, placeholders like {name} and JSON structure identical. Plain, friendly UI tone, no jargon. Output only JSON." writing
    ;;
  summarize)
    lines="${2:-15}"
    ask "$LLM_MECHANICAL_MODEL" \
      "Summarize this text in at most ${lines} lines. Keep every number, file path, test name, failure and decision exactly; drop narration. Output only the summary."
    ;;
  prose)
    ask "$LLM_WRITING_MODEL" \
      "Copy-edit this French text: plain words, short sentences, active voice, no jargon, same meaning and same Markdown structure. Do not add content. Output only the text." writing
    ;;
  check)
    [[ "$(echo ping | ask "$LLM_MECHANICAL_MODEL" "Answer with the single word: pong" 2>/dev/null)" == *pong* ]] || exit 3
    ;;
  *)
    sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
    exit 2
    ;;
esac
