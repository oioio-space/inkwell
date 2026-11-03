#!/usr/bin/env bash
# Runs a memory-hungry job (tests, e2e, mutation testing) so that it can never take the machine
# down. Two global out-of-memory kills on 2026-10-26 came from parallel agents: a mutant in
# internal/trust allocated 8.7 GB, and headless Chrome 3.4 GB.
#
#   scripts/heavy.sh [--exclusive] <command> [args…]
#
# - At most INKWELL_HEAVY_SLOTS jobs (default 2) run at once on the machine, across every
#   checkout and worktree; the others wait for a free slot.
# - Each job runs in its own cgroup capped at INKWELL_HEAVY_MEM (default 8G) without swap: if it
#   goes past, the kernel kills that job only, and the command fails with exit 137.
# - Nested calls (a heavy task that depends on another) reuse the slot they already hold.
# - In CI, or where there is no user systemd, there is no cgroup cap; slots still apply.
# - --exclusive is for wall-clock performance measurements: it takes EVERY slot (in order, so two
#   exclusive jobs cannot deadlock), so no other heavy job runs meanwhile. It then waits until
#   the 1-minute load average is at most INKWELL_PERF_MAX_LOAD (default: CPUs / 4, i.e. the
#   machine is at least three quarters idle), for at most INKWELL_PERF_LOAD_WAIT seconds
#   (default 600); past that it refuses to measure and exits 75. The threshold of the
#   measurement itself is never touched. With INKWELL_PERF_OUT set, it appends
#   {"criterion","exclusive","slots","load_before","load_after","max_load","waited_seconds",
#   "exit_code"} to $INKWELL_PERF_OUT/exclusive.jsonl (criterion from INKWELL_GATE_CRITERION).
#   An exclusive job cannot start inside a shared slot: that would wait for itself.
set -euo pipefail

exclusive=0
if [[ ${1:-} == --exclusive ]]; then
  exclusive=1
  shift
fi
[[ $# -gt 0 ]] || { sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'; exit 2; }

# /tmp is a RAM-backed tmpfs: Go's link and test scratch goes to disk instead, and scratch
# left by killed jobs (go-build*, gremlins-*) is removed once it is an hour old.
if [[ -z "${CI:-}" ]]; then
  export GOTMPDIR="${GOTMPDIR:-${XDG_CACHE_HOME:-$HOME/.cache}/inkwell/gotmp}"
  mkdir -p "$GOTMPDIR"
  find /tmp "$GOTMPDIR" -maxdepth 1 -mindepth 1 -user "$(id -u)" \
    \( -name 'go-build*' -o -name 'gremlins-*' \) -mmin +60 -exec rm -rf {} + 2>/dev/null || true
fi

if [[ -n "${INKWELL_HEAVY_HELD:-}" ]]; then
  if ((exclusive)) && [[ ${INKWELL_HEAVY_SLOT:-} != all ]]; then
    echo "heavy.sh: --exclusive inside a shared heavy slot would wait for itself" >&2
    exit 2
  fi
  exec "$@"
fi

slots="${INKWELL_HEAVY_SLOTS:-2}"
mem="${INKWELL_HEAVY_MEM:-8G}"
dir="${XDG_RUNTIME_DIR:-/tmp}/inkwell-heavy"
mkdir -p "$dir"

# capped runs the command in its own memory-capped cgroup where user systemd exists.
capped() {
  if [[ -n "${CI:-}" ]] || ! systemctl --user show-environment >/dev/null 2>&1; then
    "$@"
  else
    systemd-run --user --scope --quiet --collect -p MemoryMax="$mem" -p MemorySwapMax=0 -- "$@"
  fi
}

if ((exclusive)); then
  for ((i = 0; i < slots; i++)); do
    exec {fd}>"$dir/slot-$i.lock"
    if ! flock -n "$fd"; then
      echo "heavy.sh: exclusive measurement waiting for slot $i…" >&2
      flock "$fd"
    fi
  done
  export INKWELL_HEAVY_HELD=1 INKWELL_HEAVY_SLOT=all
  max_load="${INKWELL_PERF_MAX_LOAD:-$(awk -v n="$(nproc)" 'BEGIN { print n / 4 }')}"
  load_wait="${INKWELL_PERF_LOAD_WAIT:-600}"
  begin=$SECONDS
  until awk -v m="$max_load" '{ exit !($1 <= m) }' /proc/loadavg; do
    if ((SECONDS - begin >= load_wait)); then
      echo "heavy.sh: load average $(cut -d' ' -f1 /proc/loadavg) still above $max_load after ${load_wait}s; refusing to measure" >&2
      exit 75
    fi
    sleep 5
  done
  waited=$((SECONDS - begin))
  before=$(cut -d' ' -f1-3 /proc/loadavg)
  set +e
  capped "$@"
  status=$?
  set -e
  after=$(cut -d' ' -f1-3 /proc/loadavg)
  if [[ -n "${INKWELL_PERF_OUT:-}" ]]; then
    mkdir -p "$INKWELL_PERF_OUT"
    printf '{"criterion":"%s","exclusive":true,"slots":%d,"load_before":[%s],"load_after":[%s],"max_load":%s,"waited_seconds":%d,"exit_code":%d}\n' \
      "${INKWELL_GATE_CRITERION:-}" "$slots" "${before// /,}" "${after// /,}" "$max_load" "$waited" "$status" \
      >>"$INKWELL_PERF_OUT/exclusive.jsonl"
  fi
  exit "$status"
fi

# Take the first free slot; the lock is released when this process exits.
waited=0
while :; do
  for ((i = 0; i < slots; i++)); do
    exec {fd}>"$dir/slot-$i.lock"
    if flock -n "$fd"; then
      export INKWELL_HEAVY_HELD=1 INKWELL_HEAVY_SLOT="$i"
      ((waited)) && echo "heavy.sh: slot $i free, starting" >&2
      capped "$@"
      exit
    fi
    exec {fd}>&-
  done
  ((waited)) || echo "heavy.sh: $slots heavy jobs already running, waiting for a slot…" >&2
  waited=1
  sleep 5
done
