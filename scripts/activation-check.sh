#!/usr/bin/env bash
# Native check of the activation token (block OI, plan docs/plans/2026-11-11-oi-single-instance-open.md
# section 13): a second launch started as a desktop starts it, with $XDG_ACTIVATION_TOKEN, makes the
# running Inkwell ask the compositor to activate its window WITH THAT TOKEN, first. Without it GNOME
# refuses the focus and only says « "Inkwell" est prêt » (defect of 2026-11-13).
#
#   scripts/activation-check.sh [inkwell binary]      (default bin/inkwell: mise run app:build)
#
# The real desktop binary runs under a headless Mutter on a private session bus, with its profile,
# documents and XDG folders in a scratch folder: nothing of the person's session or profile is
# touched. The proof is the Wayland request itself, read from the primary's WAYLAND_DEBUG trace:
# xdg_activation_v1.activate("<the forwarded token>", <the window's surface>). What the compositor
# does with a token it issued (raise, end of the busy cursor) is Mutter's side and is checked on a
# real GNOME session.
set -euo pipefail

token="oi-forwarded-token-42"

if [[ ${1:-} == --inner ]]; then
  bin=$2 out=$3
  mutter --headless --wayland --no-x11 --virtual-monitor 1280x800 --wayland-display wl-oi >"$out/mutter.log" 2>&1 &
  mutter_pid=$!
  primary_pid=
  # shellcheck disable=SC2317,SC2329 # called by the trap
  stop() { kill "${primary_pid:-$mutter_pid}" "$mutter_pid" 2>/dev/null || true; }
  trap stop EXIT
  wait_for() { # <tries of 0.1 s> <command…>
    local tries=$1 _
    shift
    for _ in $(seq "$tries"); do
      if "$@" 2>/dev/null; then return 0; fi
      sleep 0.1
    done
    return 1
  }
  wait_for 100 test -S "$XDG_RUNTIME_DIR/wl-oi" || { echo "✗ activation: Mutter did not start (see $out/mutter.log)"; exit 2; }
  export WAYLAND_DISPLAY=wl-oi GDK_BACKEND=wayland
  WAYLAND_DEBUG=1 "$bin" >"$out/primary.log" 2>&1 &
  primary_pid=$!
  # The primary raises its window for its own request once the webview reads the open stream: from
  # then on a forwarded request is delivered at once.
  wait_for 300 grep -q 'xdg_activation_v1#[0-9]*\.activate(' "$out/primary.log" ||
    { echo "✗ activation: the primary never presented its window (see $out/primary.log)"; exit 2; }
  sleep 2 # the startup's own presentations are over
  before=$(wc -l <"$out/primary.log")
  code=0
  XDG_ACTIVATION_TOKEN=$token timeout 5 "$bin" >"$out/second.log" 2>&1 || code=$?
  [[ $code -eq 0 ]] || { echo "✗ activation: the second launch ended with $code within 5 s, want 0 (see $out/second.log)"; exit 1; }
  requests() { tail -n "+$((before + 1))" "$out/primary.log" | grep -o 'xdg_activation_v1#[0-9]*\.activate("[^"]*"' || true; }
  # shellcheck disable=SC2317,SC2329 # called through wait_for
  requested() { [[ -n $(requests) ]]; }
  wait_for 50 requested || true
  first=$(requests | head -n 1)
  requests >"$out/requests.txt"
  if [[ $first != *"activate(\"$token\"" ]]; then
    echo "✗ activation: after the second launch the primary's first activation request is « ${first:-none} », want the forwarded token « $token »"
    exit 1
  fi
  echo "✓ activation: the running Inkwell activates its window with the second launch's token: $first)"
  exit 0
fi

for tool in mutter dbus-run-session; do
  command -v "$tool" >/dev/null 2>&1 || { echo "✗ activation: $tool is required (a GNOME desktop has both)" >&2; exit 2; }
done
bin=$(realpath "${1:-bin/inkwell}")
[[ -x $bin ]] || { echo "✗ activation: no desktop binary at $bin (mise run app:build)" >&2; exit 2; }
out=$(mktemp -d "${TMPDIR:-/tmp}/oi-act.XXXXXX") # short: a Wayland socket path holds 108 bytes at most
trap 'sleep 0.5; rm -rf "$out" 2>/dev/null || true' EXIT
mkdir -m 700 "$out/rt"
mkdir -p "$out/home" "$out/docs"
status=0
env -i PATH="$PATH" LANG="${LANG:-C.UTF-8}" HOME="$out/home" XDG_RUNTIME_DIR="$out/rt" \
  XDG_DATA_HOME="$out/home/data" XDG_CACHE_HOME="$out/home/cache" XDG_CONFIG_HOME="$out/home/config" \
  XDG_STATE_HOME="$out/home/state" XDG_DOCUMENTS_DIR="$out/docs" INKWELL_HOME="$out/home/inkwell" \
  LIBGL_ALWAYS_SOFTWARE=1 \
  dbus-run-session -- "$0" --inner "$bin" "$out" 2>"$out/session.log" || status=$?
if [[ $status -ne 0 ]]; then
  echo "--- activation requests after the second launch:" >&2
  cat "$out/requests.txt" >&2 2>/dev/null || true
  tail -n 5 "$out/second.log" "$out/mutter.log" >&2 2>/dev/null || true
fi
exit "$status"
