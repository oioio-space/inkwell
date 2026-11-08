#!/usr/bin/env bash
# One line per model source: what is live-readable (1-token probes, OpenRouter key/credits) plus the
# known renewals. Keys come from the environment (mise.local.toml [env]); they are never printed.
#
#   mise run quotas
#
# Renewal dates that change (Codex, Antigravity) live in scripts/quotas.toml.
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${QUOTAS_FILE:=$here/quotas.toml}"
: "${GEMINI_MODEL:=gemini-3.8-flash}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
msg='{"messages":[{"role":"user","content":"hi"}],"max_tokens":1}'

# probe URL AUTH_HEADER JSON → prints the HTTP status; response headers in $tmp/h, body in $tmp/b.
probe() {
  curl -sS --max-time 30 -D "$tmp/h" -o "$tmp/b" -w '%{http_code}' "$1" \
    -H @<(printf '%s\n' "$2") -H "Content-Type: application/json" --data-binary "$3" 2>"$tmp/e" || true
}
header() { grep -i "^$1:" "$tmp/h" 2>/dev/null | head -1 | cut -d: -f2- | tr -d ' \r'; }
verdict() {
  case "$1" in
    200) echo ok ;;
    429) echo "quota exhausted (429)" ;;
    503) echo "high demand (503)" ;;
    000) echo "no answer (timeout or unreachable)" ;;
    *) echo "HTTP $1" ;;
  esac
}

# Groq
if [[ -z "${GROQ_API_KEY:-}" ]]; then
  echo "groq        not configured"
else
  code="$(probe https://api.groq.com/openai/v1/chat/completions "Authorization: Bearer $GROQ_API_KEY" \
    "$(jq -n --argjson m "$msg" '$m + {model: "openai/gpt-oss-120b"}')")"
  echo "groq        $(verdict "$code"); requests left $(header x-ratelimit-remaining-requests)/$(header x-ratelimit-limit-requests)" \
    "(reset $(header x-ratelimit-reset-requests)); tokens left $(header x-ratelimit-remaining-tokens)/$(header x-ratelimit-limit-tokens)" \
    "(reset $(header x-ratelimit-reset-tokens))"
fi

# OpenRouter
if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  echo "openrouter  not configured"
else
  auth="Authorization: Bearer $OPENROUTER_API_KEY"
  key="$(curl -sS --max-time 30 https://openrouter.ai/api/v1/key -H @<(printf '%s\n' "$auth") 2>/dev/null)"
  cred="$(curl -sS --max-time 30 https://openrouter.ai/api/v1/credits -H @<(printf '%s\n' "$auth") 2>/dev/null)"
  echo "openrouter  key usage \$$(jq -r '.data.usage // "?"' <<<"$key" 2>/dev/null)" \
    "(limit $(jq -r '.data.limit // "none"' <<<"$key" 2>/dev/null), free tier $(jq -r '.data.is_free_tier // "?"' <<<"$key" 2>/dev/null));" \
    "credits used \$$(jq -r '.data.total_usage // "?"' <<<"$cred" 2>/dev/null) of \$$(jq -r '.data.total_credits // "?"' <<<"$cred" 2>/dev/null)" \
    "(left \$$(jq -r 'if .data then (.data.total_credits - .data.total_usage) else "?" end' <<<"$cred" 2>/dev/null))"
fi

# Cloudflare Workers AI
if [[ -z "${CLOUDFLARE_API_TOKEN:-}" || -z "${CLOUDFLARE_ACCOUNT_ID:-}" ]]; then
  echo "cloudflare  not configured"
else
  code="$(probe "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/ai/run/@cf/openai/gpt-oss-120b" \
    "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "$msg")"
  echo "cloudflare  $(verdict "$code") (no counter exposed; free allowance resets 00:00 UTC)"
fi

# Gemini
if [[ -z "${GEMINI_API_KEY:-}" ]]; then
  echo "gemini      not configured"
else
  code="$(probe "https://generativelanguage.googleapis.com/v1beta/models/$GEMINI_MODEL:generateContent" \
    "x-goog-api-key: $GEMINI_API_KEY" \
    '{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":1}}')"
  echo "gemini      $GEMINI_MODEL $(verdict "$code") $(tr -d "\n" <"$tmp/e" | cut -c1-120)"
fi

# Known renewals (static)
echo "renewal     gemini daily reset at midnight Pacific (09:00 Paris)"
echo "renewal     cloudflare daily reset at 00:00 UTC"
if [[ -r "$QUOTAS_FILE" ]]; then
  python3 -I - "$QUOTAS_FILE" <<'PY'
import sys, tomllib
with open(sys.argv[1], "rb") as f:
    data = tomllib.load(f)
for name, entry in data.get("renewals", {}).items():
    print(f"renewal     {name} {entry.get('when', '?')}" + (f" ({entry['note']})" if entry.get("note") else ""))
PY
else
  echo "renewal     $QUOTAS_FILE missing"
fi
