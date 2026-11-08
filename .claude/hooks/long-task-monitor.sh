#!/usr/bin/env bash
# PreToolUse(Bash): a command sent to the background is a long task, and a long task is always
# monitored (CLAUDE.md, « Tâches longues »). The hook reminds Claude, at the moment it launches
# one, to write the output to a log and arm a Monitor on it that reports progress and EVERY
# terminal state — success, failure, OOM/137, panic, stall — instead of waiting blind.
set -euo pipefail
# shellcheck source=lib/hooklib.sh
source "$(dirname "$0")/lib/hooklib.sh"
input="${HOOK_INPUT:-}"
[[ "$(jq -r '.tool_input.run_in_background // false' <<<"$input")" == true ]] || exit 0

hook_emit_context "LONG TASK (CLAUDE.md « Tâches longues ») — this command runs in the background, so monitor it:
1. its output goes to a log in the scratchpad (cmd > LOG 2>&1; echo \"exit \$?\" >> LOG);
2. right after launching, arm Monitor with command \"scripts/watch-log.sh LOG [stall-seconds]\"
   (progress, every failure signature, silence, final exit), a specific description and
   timeout_ms 1800000, re-armed at each expiry until the task ends;
3. tell the user what runs and how it is watched; report each step; act at once on a failure."
