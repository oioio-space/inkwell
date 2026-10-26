#!/usr/bin/env bash
# SessionStart: says whether the hosted open-weight models answer, and what to hand them
# (CLAUDE.md routing, ADR 0013). Never blocks the session.
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
err=$(echo ping | timeout 20 mise exec -- scripts/llm.sh summarize 1 2>&1 >/dev/null)
case $? in
  0) state="OpenRouter answers" ;;
  3) state="OpenRouter key missing (mise.local.toml): fall back to Haiku" ;;
  *) case "$err" in
       *402*) state="OpenRouter credit exhausted (HTTP 402): tell the user once, fall back to Haiku" ;;
       *401*) state="OpenRouter key rejected (HTTP 401): tell the user once, fall back to Haiku" ;;
       *) state="OpenRouter unreachable: fall back to Haiku" ;;
     esac ;;
esac
cat <<MSG
OPENROUTER DELEGATION CHECK: ${state}.
At the start of this session, list the pending mechanical text work and delegate it to scripts/llm.sh,
not to Claude:
- commit-msg: draft commit messages;
- triage: long test or CI logs;
- summarize N: long agent reports or logs;
- translate en: i18n drafts;
- prose: French copy-edits of docs and the register.
Never use it for code, review, security, design or UX judgement.
MSG
