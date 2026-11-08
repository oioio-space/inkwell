#!/usr/bin/env bash
# Event stream for a long task's log, meant as the command of a Claude Code Monitor
# (CLAUDE.md, « Tâches longues »): one line per event, never the raw log.
#
#   scripts/watch-log.sh <log> [stall-seconds] [extra-progress-regex]
#
# Failure signatures: upper-case markers plus the bare word « fail(ed) » (gate.sh prints « <criterion> fail »);
# never the generic word « Error », which passing tests print on purpose (refused connections).
# Emits: progress lines (done: / gate / step markers, mutation scores, test summaries, plus the
# optional extra regex), every failure or terminal signature (ERROR, FAIL, panic, Killed, OOM,
# exit codes, timeouts, void measurements), a count every 50 mutants, and « stalled: no output for
# N s » when the log stops moving (default 300 s) — silence is reported, never taken for progress.
# It exits when the log contains a final « exit N » line (the launcher's `; echo "exit $?"`).
set -uo pipefail
log="${1:?usage: watch-log.sh <log> [stall-seconds] [extra-progress-regex]}"
stall="${2:-300}"
extra="${3:-}"
progress='(^done: |\[gate\]|^ok |^--- FAIL|Test efficacy|Killed: [0-9]+, Lived|Tests +[0-9]+|Test Files|Finished in|✓|✗)'
failure='(ERROR|FAIL|(^| )fail(ed)?( |$)|panic:|Killed process|OOM|oom-kill|exited [0-9]+|exit status [1-9]|timed out|void|Traceback|fatal error)'

until [[ -f $log ]]; do sleep 1; done
mutants=0
while true; do
  IFS= read -r -t "$stall" line
  rc=$?
  if ((rc > 128)); then
    echo "stalled: no output for ${stall} s in $(basename "$log")"
    continue
  elif ((rc != 0)); then
    break
  fi
  case "$line" in
    *KILLED* | *LIVED* | *"TIMED OUT"* | *"NOT COVERED"*)
      mutants=$((mutants + 1))
      [[ $line == *LIVED* || $line == *"TIMED OUT"* ]] && echo "${line:0:160}"
      ((mutants % 50 == 0)) && echo "progress: $mutants mutants judged"
      ;;
    *)
      if [[ $line =~ $failure || $line =~ $progress || (-n $extra && $line =~ $extra) ]]; then
        echo "${line:0:200}"
      fi
      ;;
  esac
  [[ $line =~ ^exit\ [0-9]+$ ]] && { echo "finished: $line"; exit 0; }
done < <(tail -n +1 -F "$log" 2>/dev/null)
