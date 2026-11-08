#!/usr/bin/env bash
# Native input collection requires explicit host opt-in; otherwise writes a failed verdict.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"
# Direct host runs retain the same exclusive-run diagnostics as gate execution.
export INKWELL_GATE_CRITERION="${INKWELL_GATE_CRITERION:-PERF-TYPE-native}"
export INKWELL_PERF_OUT="${INKWELL_PERF_OUT:-$root/gates/L3}"
exec "$root/scripts/heavy.sh" --exclusive python3 "$root/scripts/perf/type_native.py" "$@"
