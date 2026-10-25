#!/usr/bin/env bash
# Network-simulation scenarios: two real inkwell instances behind NATs, a public relay,
# driven by Playwright (frontend/e2e/netsim.spec.ts). No root: the script re-executes
# itself in new user, network and mount namespaces (`unshare -Urnm`), where it is root
# of a private network stack. Needs: unshare, ip (iproute2), nft (nftables), socat, curl.
# Binaries are built beforehand by `mise run e2e:netsim`.
#
# Without the privileges to do so (no user namespaces, tool missing) it skips with the
# reason, except when CI is set, where it fails: the CI job must really run it.
#
# Scenarios (topology.sh has the picture):
#   1  cone NAT       direct link after hole punching
#   2  symmetric NAT  the link stays relayed
#   3  alice offline 60 s while both type; back; converge
#   4  alice moves from nat-a to nat-c (new public address); converge
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$here/../.." && pwd)

if [[ -z ${NETSIM_INNER:-} ]]; then
  missing=()
  for tool in unshare ip nft socat curl; do
    command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
  done
  reason=""
  if ((${#missing[@]})); then
    reason="missing tools: ${missing[*]} (Fedora: sudo dnf install iproute nftables socat curl; Debian/Ubuntu: sudo apt-get install iproute2 nftables socat curl)"
  elif [[ $(id -u) != 0 ]] && ! unshare -Urnm true 2>/dev/null; then
    reason="cannot create user namespaces (unshare -Urnm failed); on Ubuntu 24.04: sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0; elsewhere check user.max_user_namespaces"
  elif ! unshare -Urnm nft list ruleset >/dev/null 2>&1; then
    reason="nftables does not work in a user namespace (kernel module nf_tables not loaded: sudo modprobe nf_tables)"
  fi
  if [[ -n $reason ]]; then
    if [[ -n ${CI:-} ]]; then
      echo "netsim: FAILED, $reason" >&2
      exit 1
    fi
    echo "netsim: SKIPPED, $reason" >&2
    exit 0
  fi
  export NETSIM_INNER=1
  if [[ $(id -u) == 0 ]]; then
    exec unshare -nm --fork "$0" "$@"
  fi
  exec unshare -Urnm --fork "$0" "$@"
fi

# From here on we are root of private user, network and mount namespaces.
mount -t tmpfs tmpfs /run
mkdir -p /run/netns

topology=$here/topology.sh
state=$repo/.task/netsim
out=$state/results
relay_bin=$repo/bin/inkwell-relay
app_bin=$repo/bin/inkwell-server
for bin in "$relay_bin" "$app_bin"; do
  [[ -x $bin ]] || {
    echo "netsim: $bin is missing, run: mise run e2e:netsim" >&2
    exit 1
  }
done
rm -rf "$state"
mkdir -p "$out" /run/netsim

declare -A MODE=([1]=cone [2]=symmetric [3]=cone [4]=cone)
declare -A TITLE=(
  [1]="cone NAT: direct link after hole punching"
  [2]="symmetric NAT: link stays relayed"
  [3]="alice offline 60 s, both type, converge"
  [4]="alice moves nat-a to nat-c, converge"
)
declare -A PORT=([alice]=18081 [bob]=18082)
pids=()

stop_stack() {
  if ((${#pids[@]})); then
    kill "${pids[@]}" 2>/dev/null || true
    wait "${pids[@]}" 2>/dev/null || true
  fi
  pids=()
  "$topology" down
}
trap stop_stack EXIT

# wait_http PORT: waits until the instance answers on the test runner's side of the bridge.
wait_http() {
  local port=$1 i
  for ((i = 0; i < 150; i++)); do
    if curl -fs "http://127.0.0.1:$port/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done
  echo "netsim: nothing answers on 127.0.0.1:$port (see $state)" >&2
  return 1
}

# start_stack N: topology, relay, the two instances and the unix sockets bridging their
# HTTP ports to the test runner's loopback. Nothing reaches an instance except through
# these bridges and the NATs.
start_stack() {
  local n=$1 dir=$state/$1 who relay_addr relay_ip
  mkdir -p "$dir"
  "$topology" up "${MODE[$n]}"
  relay_ip=$("$topology" relay-ip)

  ip netns exec relay "$relay_bin" --key "$dir/relay.key" --print-addr \
    >"$dir/relay.addrs" 2>"$dir/relay.log" &
  pids+=($!)
  for _ in $(seq 100); do
    relay_addr=$(grep -m1 "^/ip4/${relay_ip//./\\.}/udp/.*/quic-v1/p2p/" "$dir/relay.addrs" 2>/dev/null || true)
    [[ -n $relay_addr ]] && break
    sleep 0.1
  done
  [[ -n $relay_addr ]] || {
    echo "netsim: the relay did not start (see $dir/relay.log)" >&2
    return 1
  }

  for who in alice bob; do
    ip netns exec "$who" env \
      INKWELL_SERVER_PORT="${PORT[$who]}" \
      INKWELL_HOME="$dir/$who" \
      INKWELL_DOCS="$dir/$who-docs" \
      INKWELL_RELAYS="$relay_addr" \
      "$app_bin" >"$dir/$who.log" 2>&1 &
    pids+=($!)
    ip netns exec "$who" socat "UNIX-LISTEN:/run/netsim/$who.sock,fork" "TCP:127.0.0.1:${PORT[$who]}" &
    pids+=($!)
    socat "TCP-LISTEN:${PORT[$who]},bind=127.0.0.1,reuseaddr,fork" "UNIX-CONNECT:/run/netsim/$who.sock" &
    pids+=($!)
  done
  wait_http "${PORT[alice]}"
  wait_http "${PORT[bob]}"
}

scenarios=${NETSIM_SCENARIOS:-1 2 3 4} # e.g. NETSIM_SCENARIOS="1 2" to run some
declare -A RESULT DETAIL
failed=0
for n in $scenarios; do
  echo "netsim: scenario $n, ${TITLE[$n]}" >&2
  start=$SECONDS
  status=PASS
  if start_stack "$n"; then
    (
      cd "$repo/frontend"
      NETSIM_SCENARIO=$n NETSIM_OUT=$out NETSIM_TOPOLOGY=$topology \
        pnpm exec playwright test -c playwright.netsim.config.ts --grep "scenario $n:"
    ) || status=FAIL
  else
    status=FAIL
  fi
  RESULT[$n]=$status
  DETAIL[$n]=$(cat "$out/$n.txt" 2>/dev/null || echo "-")
  [[ $status == PASS ]] || failed=1
  DETAIL[$n]+=" ($((SECONDS - start)) s)"
  stop_stack
done

echo
echo "netsim results"
printf '  %-3s %-42s %-5s %s\n' "#" "scenario" "" "observed"
for n in $scenarios; do
  printf '  %-3s %-42s %-5s %s\n' "$n" "${TITLE[$n]}" "${RESULT[$n]}" "${DETAIL[$n]}"
done
exit "$failed"
