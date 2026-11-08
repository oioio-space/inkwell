#!/usr/bin/env bash
# Network-simulation scenarios: two real inkwell instances behind NATs, a public relay,
# driven by Playwright (frontend/e2e/netsim.spec.ts). No root: the script re-executes
# itself in new user, network and mount namespaces (`unshare -Urnm`), where it is root
# of a private network stack. Needs: unshare, ip (iproute2), nft (nftables), socat, curl.
# Binaries are built beforehand by `mise run e2e:netsim`.
#
# Without the privileges to do so (no user namespaces, tool missing) it skips with the
# reason, except when CI is set, where it fails: the CI job must really run it. A missing netem
# (impaired scenarios 5-8) always fails: those scenarios are part of `mise run ci`.
#
# Scenarios (topology.sh has the picture):
#   1  cone NAT       direct link after hole punching
#   2  symmetric NAT  the link stays relayed
#   3  alice offline 60 s while both type; back; converge
#   4  alice moves from nat-a to nat-c (new public address); converge
# Impaired network (docs/plans/2026-11-08-netsim-impaired-plan.md; netem, seeded):
#   5  NI-1  bad Wi-Fi that changes: converge, stay up, link metrics match the injected truth
#   6  NI-2  25 s blackout during a 100 000-character paste (characterization: C2 known failure)
#   7  NI-3  8 KiB/s relay: 300 000-character paste, typing elsewhere, link stays up (C1 known failure)
#   8  NI-4  three devices, alice-bob down: converge through carol (editor pass, then reader pass)
#
# Variables: NETSIM_SCENARIOS ("1 2 3 4"), NETSIM_SEEDS (impaired scenarios run once per seed;
# default: one seed derived from the date), NETSIM_EXPECT (characterize, the default: the declared
# known failures are reported, not blocking; strict: they block), NETSIM_BIN_DIR (binaries to run;
# the mutant runner points it at a mutated build), NETSIM_VARIANTS (scenario 8: editor reader).
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
  elif case " ${NETSIM_SCENARIOS:-1 2 3 4} " in *" "[5-8]" "*) true ;; *) false ;; esac &&
    ! { command -v tc >/dev/null 2>&1 && unshare -Urn tc qdisc add dev lo root netem delay 1ms 2>/dev/null; }; then
    # The impaired scenarios are part of `mise run ci`: a missing netem is never a skip, in CI or not.
    echo "netsim: FAILED, netem is not available (tc missing, or kernel module sch_netem not loaded: sudo modprobe sch_netem; a user namespace cannot load it; Fedora: sudo dnf install iproute-tc; CI: the job loads it before mise run ci)" >&2
    exit 1
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
bin_dir=${NETSIM_BIN_DIR:-$repo/bin}
relay_bin=$bin_dir/inkwell-relay
app_bin=$bin_dir/inkwell-server
for bin in "$relay_bin" "$app_bin"; do
  [[ -x $bin ]] || {
    echo "netsim: $bin is missing, run: mise run e2e:netsim" >&2
    exit 1
  }
done
rm -rf "$state"
mkdir -p "$out" /run/netsim

declare -A MODE=([1]=cone [2]=symmetric [3]=cone [4]=cone [5]=cone [6]=cone [7]=symmetric [8]=mesh)
declare -A TITLE=(
  [1]="cone NAT: direct link after hole punching"
  [2]="symmetric NAT: link stays relayed"
  [3]="alice offline 60 s, both type, converge"
  [4]="alice moves nat-a to nat-c, converge"
  [5]="NI-1 bad Wi-Fi that changes"
  [6]="NI-2 25 s blackout during a large paste"
  [7]="NI-3 8 KiB/s relay, large paste"
  [8]="NI-4 three devices, alice-bob down"
)
declare -A PORT=([alice]=18081 [bob]=18082 [carol]=18083)
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

# start_stack N RUN: topology, relay, the instances (alice and bob; carol too in mesh mode) and
# the unix sockets bridging their HTTP ports to the test runner's loopback. Nothing reaches an
# instance except through these bridges and the NATs. RUN names the state directory.
start_stack() {
  local n=$1 dir=$state/$2 who relay_addr relay_ip people="alice bob"
  [[ ${MODE[$n]} == mesh ]] && people="alice bob carol"
  # start_stack runs in an `if`, where set -e is off: every step that can fail returns explicitly.
  mkdir -p "$dir" || return 1
  "$topology" up "${MODE[$n]}" || return 1
  relay_ip=$("$topology" relay-ip) || return 1

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

  for who in $people; do
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
  for who in $people; do wait_http "${PORT[$who]}" || return 1; done
}

scenarios=${NETSIM_SCENARIOS:-1 2 3 4} # e.g. NETSIM_SCENARIOS="1 2" to run some
# The CI seed rotates daily and is recorded in every results file.
seeds=${NETSIM_SEEDS:-$(($(date +%s) / 86400))}
export NETSIM_EXPECT=${NETSIM_EXPECT:-characterize}
declare -A RESULT DETAIL TITLE_OF
runs=()
failed=0
for n in $scenarios; do
  [[ -n ${MODE[$n]:-} ]] || {
    echo "netsim: no scenario $n" >&2
    exit 2
  }
  variants=("")
  seed_list=("")
  if ((n >= 5)); then read -r -a seed_list <<<"$seeds"; fi
  if ((n == 8)); then read -r -a variants <<<"${NETSIM_VARIANTS:-editor reader}"; fi
  for seed in "${seed_list[@]}"; do
    for variant in "${variants[@]}"; do
      run=$n${variant:+-$variant}${seed:+-s$seed}
      runs+=("$run")
      TITLE_OF[$run]="${TITLE[$n]}${variant:+ ($variant)}${seed:+ seed $seed}"
      echo "netsim: scenario $run, ${TITLE_OF[$run]}" >&2
      start=$SECONDS
      status=PASS
      if start_stack "$n" "$run"; then
        (
          cd "$repo/frontend"
          NETSIM_SCENARIO=$n NETSIM_RUN=$run NETSIM_VARIANT=$variant NETSIM_SEED=${seed:-0} \
            NETSIM_OUT=$out NETSIM_TOPOLOGY=$topology \
            pnpm exec playwright test -c playwright.netsim.config.ts --grep "scenario $n:"
        ) || status=FAIL
      else
        status=FAIL
      fi
      RESULT[$run]=$status
      DETAIL[$run]=$(cat "$out/$run.txt" 2>/dev/null || echo "-")
      [[ $status == PASS ]] || failed=1
      DETAIL[$run]+=" ($((SECONDS - start)) s)"
      stop_stack
    done
  done
done

# clauses FILE: "C1 pass C2 known-failure ..." from the per-clause results of one run.
clauses() {
  python3 -I - "$1" <<'PY' 2>/dev/null || echo "-"
import json, sys
d = json.load(open(sys.argv[1]))
print(" ".join(f"{c['id']}={c['status']}{'(XPASS)' if c.get('xpass') else ''}" for c in d["clauses"]))
PY
}

echo
echo "netsim results"
printf '  %-14s %-52s %-5s %s\n' "run" "scenario" "" "observed"
for run in "${runs[@]}"; do
  printf '  %-14s %-52s %-5s %s\n' "$run" "${TITLE_OF[$run]}" "${RESULT[$run]}" "${DETAIL[$run]}"
  [[ -f $out/$run.json ]] && printf '  %-14s clauses: %s\n' "" "$(clauses "$out/$run.json")"
done
exit "$failed"
