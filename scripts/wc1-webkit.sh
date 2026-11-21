#!/usr/bin/env bash
# WC1 browser and native desktop evidence. The frozen gate owns the wall-clock cap and heavy slot.
set -euo pipefail

out="${1:?absolute evidence directory required}"
[[ "$out" = /* && -d "$out" ]] || { echo "Expected an existing absolute evidence directory" >&2; exit 2; }
cd "$(dirname "$0")/.."

browser_walk() {
    local browser="$1" fixture
    fixture="$(mktemp -d "$out/$browser.XXXXXX")"
    mkdir -p "$fixture/docs" "$fixture/profiles"
    (
        cd frontend
        local port
        for port in $(node --input-type=module -e 'import {ALICE_PORT, BOB_PORT} from "./e2e/ports.ts"; console.log(ALICE_PORT, BOB_PORT)'); do
            mkdir -p "$fixture/docs/$port" "$fixture/profiles/$port"
            touch "$fixture/docs/$port/.fixture" "$fixture/profiles/$port/.fixture"
        done
        export INKWELL_E2E_DOCS="$fixture/docs" INKWELL_E2E_PROFILE="$fixture/profiles"
        export PLAYWRIGHT_JSON_OUTPUT_NAME="$out/browser-$browser.json"
        local args=(e2e/save.spec.ts e2e/sheet.spec.ts --grep 'SAV-03b|SHEET-MU-01b' --retries=0 --reporter=json)
        if [[ "$browser" == webkit && -z "${CI:-}" ]]; then
            # Build Chromium for the next leg while the task runs WebKit in Podman.
            INKWELL_E2E_BROWSER=chromium mise run e2e:webkit -- "${args[@]}"
        else
            INKWELL_E2E_BROWSER="$browser" INKWELL_E2E_WS='' ./node_modules/.bin/playwright test "${args[@]}"
        fi
    )
}

native_walk() (
    for tool in dbus-run-session dbus-daemon mutter gdbus go; do
        command -v "$tool" >/dev/null || { echo "WC1 native: missing $tool on PATH" >&2; exit 2; }
    done

    # Keep the Wayland, D-Bus and fixture sockets within the Unix address limit.
    runtime="$(mktemp -d /tmp/inkwell-wc1.XXXXXX)"
    trap 'rm -rf -- "$runtime"' EXIT

    # The walk's services register directly. Omit service search directories
    # so the private bus cannot activate the host's portal or accessibility services.
    cat >"$runtime/bus.conf" <<WC1_BUS
<busconfig>
  <type>session</type>
  <listen>unix:path=$runtime/bus</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*"/>
    <allow receive_sender="*"/>
    <allow own="*"/>
  </policy>
</busconfig>
WC1_BUS

    unset DISPLAY WAYLAND_SOCKET DBUS_SESSION_BUS_ADDRESS AT_SPI_BUS_ADDRESS
    unset XDG_ACTIVATION_TOKEN DESKTOP_STARTUP_ID SESSION_MANAGER
    export XDG_RUNTIME_DIR="$runtime" WAYLAND_DISPLAY=wc1-test
    export XDG_CONFIG_HOME="$runtime/config" XDG_CACHE_HOME="$runtime/cache"
    export XDG_STATE_HOME="$runtime/state" XDG_DATA_HOME="$runtime/data"
    export GDK_BACKEND=wayland GSK_RENDERER=cairo LIBGL_ALWAYS_SOFTWARE=1
    export GTK_A11Y=none NO_AT_BRIDGE=1 GSETTINGS_BACKEND=memory
    mkdir -p "$XDG_STATE_HOME"

    dbus-run-session --config-file "$runtime/bus.conf" -- bash -s -- "$out" <<'WC1_SESSION'
set -euo pipefail
out="$1"
if ! gdbus call --session --timeout 3 --dest org.freedesktop.DBus \
    --object-path /org/freedesktop/DBus --method org.freedesktop.DBus.GetId >/dev/null; then
    echo "WC1 native: private D-Bus session is unreachable" >&2
    exit 1
fi

mutter --wayland --headless --no-x11 --virtual-monitor 1280x800 \
    --wayland-display "$WAYLAND_DISPLAY" >"$out/mutter.log" 2>&1 &
compositor=$!
trap 'kill "$compositor" 2>/dev/null || true; wait "$compositor" 2>/dev/null || true' EXIT
if ! gdbus wait --timeout 10 --session org.gnome.Mutter.RemoteDesktop ||
    ! test -S "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY" || ! kill -0 "$compositor" 2>/dev/null; then
    echo "WC1 native: headless Mutter did not provide RemoteDesktop and its Wayland socket ($out/mutter.log)" >&2
    exit 1
fi

status=0
WC1_NATIVE_OUT="$out" go test -tags wc1native -count=1 -json \
    -run "^TestWC1Native(Close|Recovery)$" ./cmd/inkwell >"$out/native.jsonl" || status=$?
if ! kill -0 "$compositor" 2>/dev/null; then
    echo "WC1 native: headless Mutter exited during the GTK walk ($out/mutter.log)" >&2
    exit 1
fi
if ((status)); then
    echo "WC1 native: GTK walk exited $status ($out/native.jsonl)" >&2
fi
exit "$status"
WC1_SESSION
)

if [[ -n "${CI:-}" ]]; then
    # GitHub's Ubuntu runner uses the host Playwright browsers and their apt dependencies.
    INKWELL_E2E_BROWSER=chromium mise run e2e:build
    (cd frontend && ./node_modules/.bin/playwright install --with-deps webkit)
fi
browser_walk webkit
browser_walk chromium
python3 -I - "$out" <<'PY'
import json
from pathlib import Path
import sys

out = Path(sys.argv[1])
engines = [json.loads((out / ("browser-" + name + ".json")).read_text()) for name in ("webkit", "chromium")]
report = {
    "suites": [suite for engine in engines for suite in engine["suites"]],
    "errors": [error for engine in engines for error in engine["errors"]],
}
(out / "webkit.json").write_text(json.dumps(report) + "\n")
PY
native_walk
