#!/usr/bin/env bash
# PreToolUse(Bash): refuses any test or mutation run that is not memory-capped.
# Three global out-of-memory kills (2026-10-26 twice, 2026-11-08: an uncapped Python test looping
# to 21 GB) took the whole session down. A rule in CLAUDE.md was not enough; this makes it structural.
# Allowed: scripts/heavy.sh, scripts/memcap.sh, systemd-run (own cgroup), or a `mise run` task
# (every test task goes through heavy.sh — keep it that way).
set -euo pipefail
cmd="$(jq -r '.tool_input.command // empty')"
[[ -n $cmd ]] || exit 0
# A runner counts only in command position (start, or after ; & | ( $( ), optionally behind
# timeout / env / VAR=value / rtk / mise x --, so words inside echo or printf text do not match.
runner='(^|[;&|(]|\$\()[[:space:]]*((timeout( -[a-zA-Z]+)* [^ ]+|env|[A-Za-z_][A-Za-z0-9_]*=[^ ]*|rtk|mise x --|pnpm( exec)?|npx|cd [^;&|]+&&)[[:space:]]+)*((go|rtk go) test|gotestsum|python3? (-I )?-m (unittest|pytest)|pytest|vitest|\./node_modules/\.bin/vitest|pnpm( [^;&|]*)? (run )?(test|check)\b|npm (run )?test|playwright test|gremlins|go run [^;&|]*gremlins|stryker|python3? [^;&|]*_test\.py)'
# Quoted text (grep patterns, echo/printf messages) is not run: strip it before matching.
# Here-document bodies (<<EOF … EOF) are data, not commands: drop them before matching.
strip_heredocs() { awk '{ if (t != "") { if ($0 == t) t = ""; next } print; if (match($0, /<<-?[[:space:]]*["'"'"']?[A-Za-z_][A-Za-z0-9_]*["'"'"']?/)) { t = substr($0, RSTART, RLENGTH); sub(/^<<-?[[:space:]]*/, "", t); gsub(/["'"'"']/, "", t) } }'; }
bare="$(strip_heredocs <<<"$cmd" | sed -E "s/'[^']*'//g; s/\"[^\"]*\"//g")"
bare="${bare//$'\n'/;}" # a new line starts a new command, like ;
[[ $bare =~ $runner ]] || exit 0
[[ $cmd =~ (heavy\.sh|memcap\.sh|systemd-run) ]] && exit 0
cat >&2 <<'MSG'
heavy-guard: test run without a memory cap. Wrap it so a runaway test kills itself, not the session:
  ./scripts/heavy.sh <command>        (or a `mise run` test task, which already does)
  timeout 300 systemd-run --user --scope -q -p MemoryMax=4G -p MemorySwapMax=0 <command>
MSG
exit 2
