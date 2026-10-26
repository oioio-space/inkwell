#!/usr/bin/env bash
# Claude Code PreToolUse hook — enforce the FULL pre-commit review before a commit.
# Blocks until every applicable review dimension has been addressed on the exact
# staged diff. Approval is ONE marker file keyed to the staged-diff hash, so it
# re-arms whenever staging changes; .githooks/post-commit also clears it so each
# new commit re-arms. The sibling commit-*-review.sh hooks inject each dimension's
# detailed checklist; this gate makes ACTING on them mandatory, not advisory —
# i.e. it covers /simplify AND secret/vuln/style/cleanup/ergonomics/docs alike.
# Bypass (use sparingly, e.g. a pure marker/config edit): `git commit --no-verify`.
set -euo pipefail
# shellcheck source=lib/hooklib.sh
source "$(dirname "$0")/lib/hooklib.sh"

cmd="$(hook_cmd)"
hook_is_git_commit "$cmd" || exit 0
case "$cmd" in *--no-verify*) exit 0 ;; esac # explicit bypass

cd "$HOOK_REPO_DIR" || exit 0
marker="$(git rev-parse --git-dir 2>/dev/null || echo .git)/claude-simplify-ok"
staged_hash="$(git diff --cached | sha1sum | cut -d' ' -f1)"

# Already approved for this exact staged content → let the commit through.
[[ -f "$marker" && "$(cat "$marker")" == "$staged_hash" ]] && exit 0

# One short line, Go items only when Go is staged (the full checklists live in the skills).
files="$(git diff --cached --name-only)"
checks="/simplify (reuse, simplification, efficiency, altitude), secrets, no artifacts"
if grep -q '\.go$' <<<"$files"; then
  checks+=", Go style (scripts/style-checklist.md), vulns (vuln-guard)"
  git diff --cached -U0 -- '*.go' | grep -qE '^\+(func|type|var|const) [A-Z]|^\+func \([^)]*\) [A-Z]' &&
    checks+=", exported-API ergonomics"
  grep -v '_test\.go$' <<<"$files" | grep -q '\.go$' && checks+=", README/PROGRESS if user-facing"
fi
hook_emit_deny "Review the staged diff first: ${checks}. Substantial Go → go-reviewer (Sonnet); trivial → inline. Then, in a separate call: git diff --cached | sha1sum | cut -d' ' -f1 > \"$marker\""
