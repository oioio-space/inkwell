#!/usr/bin/env bash
# Runs a memory-hungry job (tests, e2e, mutation testing) so that it can never take the machine
# down. Two global out-of-memory kills on 2026-10-26 came from parallel agents: a mutant in
# internal/trust allocated 8.7 GB, and headless Chrome 3.4 GB.
#
#   scripts/heavy.sh <command> [args…]
#
# - At most INKWELL_HEAVY_SLOTS jobs (default 2) run at once on the machine, across every
#   checkout and worktree; the others wait for a free slot.
# - Each job runs in its own cgroup capped at INKWELL_HEAVY_MEM (default 8G) without swap: if it
#   goes past, the kernel kills that job only, and the command fails with exit 137.
# - Nested calls (a heavy task that depends on another) reuse the slot they already hold.
# - In CI, or where there is no user systemd, the command runs as is.
set -euo pipefail

[[ $# -gt 0 ]] || { sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'; exit 2; }

if [[ -n "${CI:-}" || -n "${INKWELL_HEAVY_HELD:-}" ]] || ! systemctl --user show-environment >/dev/null 2>&1; then
  exec "$@"
fi

slots="${INKWELL_HEAVY_SLOTS:-2}"
mem="${INKWELL_HEAVY_MEM:-8G}"
dir="${XDG_RUNTIME_DIR:-/tmp}/inkwell-heavy"
mkdir -p "$dir"

# Take the first free slot; the lock is released when this process exits.
waited=0
while :; do
  for ((i = 0; i < slots; i++)); do
    exec {fd}>"$dir/slot-$i.lock"
    if flock -n "$fd"; then
      export INKWELL_HEAVY_HELD=1
      ((waited)) && echo "heavy.sh: slot $i free, starting" >&2
      systemd-run --user --scope --quiet --collect \
        -p MemoryMax="$mem" -p MemorySwapMax=0 -- "$@"
      exit
    fi
    exec {fd}>&-
  done
  ((waited)) || echo "heavy.sh: $slots heavy jobs already running, waiting for a slot…" >&2
  waited=1
  sleep 5
done
