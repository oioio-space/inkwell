#!/usr/bin/env bash
# Run declarative lot criteria and retain evidence even when a criterion fails.
#
# The runner is the MERGE-BASE's scripts/gate.py (with its perf/archive.py),
# copied to a temporary directory and run isolated (python3 -I) from there, so a
# branch cannot modify the runner that judges it. INKWELL_GATE_ROOT names this
# checkout; the manifest records the runner ("runner.source").
#
# Bootstrap escape hatch: when the base runner is absent or predates this
# protocol (no "GATE_RUNNER_PROTOCOL = 1" line), the working tree's runner runs
# instead and the manifest says so: runner.source = "working-tree (bootstrap…)".
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
base="$(git merge-base HEAD main)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/scripts/perf"
if git show "$base:scripts/gate.py" >"$tmp/scripts/gate.py" 2>/dev/null &&
  grep -qx 'GATE_RUNNER_PROTOCOL = 1' "$tmp/scripts/gate.py" &&
  git show "$base:scripts/perf/archive.py" >"$tmp/scripts/perf/archive.py" 2>/dev/null; then
  runner="$tmp/scripts/gate.py"
  export INKWELL_GATE_RUNNER="base $base"
else
  runner="$root/scripts/gate.py"
  export INKWELL_GATE_RUNNER="working-tree (bootstrap: no runner protocol at $base)"
fi
status=0
(cd "$tmp" && INKWELL_GATE_ROOT="$root" python3 -I "$runner" "$@") || status=$?
exit "$status"
