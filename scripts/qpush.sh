#!/usr/bin/env bash
# Quiet push: the pre-push hook runs the full CI; keep its log out of the terminal and
# print only the verdict and the failing lines (token economy).
set -uo pipefail
log="${TMPDIR:-/tmp}/inkwell-push.log"
# Seconds, before the minutes of the pre-push CI: the cheap checks fail first.
if ! mise run pregate >"$log" 2>&1; then echo "pregate FAILED — $log"; grep -E "FAIL|✗|ERROR|missing|unreachable" "$log" | head -15; exit 1; fi
if git push -q "$@" >"$log" 2>&1; then echo "push ok"; exit 0; fi
echo "push FAILED — $log"; grep -E "FAIL|✗|ERROR|--- FAIL|G[0-9]{3} " "$log" | head -15; exit 1
