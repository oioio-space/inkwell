#!/usr/bin/env bash
# Run the WC1 draft models without changing the formal runner used by frozen gates.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cache="${INKWELL_TLA_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/inkwell/tla}"
jar="$cache/tla2tools.jar"
logs="$cache/logs"
xmx="${INKWELL_TLA_XMX:-2g}"
workers="${INKWELL_TLA_WORKERS:-2}"
heavy="$root/scripts/heavy.sh"

case "${1:-}" in
  sound|mutants) suite="$1" ;;
  *) echo "Usage: $0 sound|mutants" >&2; exit 2 ;;
esac

# Reuse the shared runner's pinned TLC provisioning. No existing model name matches ^$.
INKWELL_TLA_ONLY='^$' "$root/scripts/tla-check.sh"
metaroot="$(mktemp -d -t inkwell-wc1-tla.XXXXXX)"
trap 'rm -rf "$metaroot"' EXIT
trap 'exit 1' HUP INT TERM
cd "$root/docs/protocol/tla"

fail=0
run() {
  local name="$1" cfg="$2" property="${3:-}"
  local out="$logs/$name.log" states="$metaroot/$name" status=0
  "$heavy" java -Xmx"$xmx" -XX:+UseParallelGC -cp "$jar" tlc2.TLC \
    -workers "$workers" -noGenerateSpecTE -cleanup -metadir "$states" \
    -config "$cfg" WindowClose >"$out" 2>&1 || status=$?
  rm -rf "$states"

  if [[ -z "$property" ]]; then
    if ((status == 0)) && grep -q 'No error has been found' "$out" &&
      grep -q 'complete state space' "$out"; then
      printf '✓ %s: no error (exhaustive, liveness checked)\n' "$name"
      return
    fi
  elif ((status == 12 || status == 13)) &&
    grep -Eq "Error: (Invariant|Temporal property) $property (is|was) violated" "$out"; then
    printf '✓ %s: violation detected (%s)\n' "$name" "$property"
    return
  fi

  printf '✗ %s: unexpected TLC result (exit %d)\n' "$name" "$status" >&2
  tail -n 20 "$out" >&2
  fail=1
}

if [[ "$suite" == sound ]]; then
  for name in WindowClose WindowCloseHealthy WindowCloseCrashed; do
    run "$name" "$name.cfg"
  done
else
  while read -r suffix property; do
    name="WindowClose$suffix"
    run "$name" "mutants/$name.cfg" "$property"
  done <<'MUTANTS'
SkipWait NoAcknowledgedKeystrokeLost
TimeoutCloses NoAcknowledgedKeystrokeLost
NoGoodbye GoodbyeBeforeClose
StaleAck CloseWaitedForAck
TypeWhileHeld InputFrozen
AckBeforeWrite AckIsDurable
PartialQuit GroupWaitedForAll
NoWrite HealthyCloseSucceeds
ForceNeedsLive ForceClosesCrashedWindow
ForceWithoutConsent OnlyWithConsent
MUTANTS
fi
exit "$fail"
