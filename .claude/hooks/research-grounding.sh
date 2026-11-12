#!/usr/bin/env bash
# Claude Code PreToolUse hook (Write|Edit|MultiEdit) — research-first where decisions are written.
#
# Method v2 (docs/plans/2026-11-12-method-v2.md, rule 3): a new technical design decision is recorded in
# docs/plans/ or docs/adr/ with its sources and applicability. This hook fires only when one of those files is
# written and reminds to cite current external sources (state of the art, prior art, official docs) and to go
# beyond them. Non-blocking (exit 0); the model sees additionalContext.
set -euo pipefail
command -v jq >/dev/null 2>&1 || exit 0
path="$(jq -r '.tool_input.file_path // empty' 2>/dev/null || true)"
case "$path" in
  */docs/plans/*|*/docs/adr/*|docs/plans/*|docs/adr/*) ;;
  *) exit 0 ;;
esac
jq -n '{hookSpecificOutput:{hookEventName:"PreToolUse",additionalContext:"Research-first (method v2, rule 3): a design decision written here cites current external sources (WebSearch/WebFetch, scripts/ghx.sh, official docs), says why it applies to this repo, and considers better alternatives."}}'
