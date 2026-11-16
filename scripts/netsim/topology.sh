#!/usr/bin/env bash
# Network-namespace topology for the netsim scenarios (see run.sh, docs/relay.md).
#
#   wan 11.0.0.0/24: bridge br-wan in its own namespace `wan` (the switch). The test runner's
#   namespace holds only `lo` and never changes during a run: the browser lives there, and an
#   interface appearing or settling (carrier, IPv6 DAD, bridge port state) makes Chromium report
#   ERR_NETWORK_CHANGED in the middle of a page load.
#     relay   11.0.0.10
#     nat-a   11.0.0.2   lan 10.1.0.0/24  -- alice (eth0, 10.1.0.2)
#     nat-b   11.0.0.3   lan 10.2.0.0/24  -- bob   (eth0, 10.2.0.2)
#     nat-c   11.0.0.4   lan 10.3.0.0/24  -- alice (eth1, 10.3.0.2)
#
# alice has a link to nat-a and one to nat-c; only one is attached at a time, so
# moving from one to the other gives her a new public address. Each NAT masquerades
# with nftables. "cone" keeps the source port (hole punching works); "symmetric"
# picks a new random port per flow (it does not). Every NAT also keeps two counters:
#   direct  TCP and UDP packets forwarded into its LAN from another NAT (not the
#           relay): traffic of a direct link, because only a punched hole lets it
#           through (the ICMP errors of failed attempts are not counted)
#   egress  packets coming from its LAN
#
# The wan range is public as far as libp2p is concerned, on purpose: it only hole punches
# between addresses that manet.IsPublicAddr accepts, and that excludes the documentation
# range 203.0.113.0/24. Nothing here is routed outside the namespaces, so the choice
# cannot reach a real host.
#
# mesh mode (scenario 8) adds carol, a desk machine on the wan with no NAT (11.0.0.20), and makes
# nat-a and nat-b symmetric: alice and bob can only reach each other through the relay.
#
# Impairment (docs/plans/2026-11-08-netsim-impaired-plan.md section 3): netem sits on the egress
# of both interfaces of a forwarding namespace (a NAT, or the relay), never on an endpoint.
# `profile NAME` applies a named profile with the seed NETSIM_SEED; `impair` is the raw form.
#
# Must run inside user+net+mount namespaces (run.sh sets them up); touches nothing else.
#
# Usage: topology.sh up cone|symmetric|mesh
#        topology.sh down
#        topology.sh alice-detach
#        topology.sh alice-attach nat-a|nat-c
#        topology.sh relay-ip                              # prints the relay's wan address
#        topology.sh counter nat-a|nat-b|nat-c direct|egress   # prints a packet count
#        topology.sh impair NS DEV NETEM_ARGS...           # tc qdisc add dev DEV root netem ARGS
#        topology.sh clear NS DEV
#        topology.sh qstat NS DEV                          # prints "bytes=N sent=N dropped=N overlimits=N"
#        topology.sh profile wifi|wifi-far|wifi-clean|wifi-entry|wifi-reorder|wifi-edge|blackout|slowrelay [PHASE]   # needs NETSIM_SEED
#        topology.sh unprofile wifi|blackout|slowrelay     # removes the qdiscs of a profile (a wifi-* name: the same four)
#        topology.sh qstats wifi|blackout|slowrelay        # "NS DEV bytes= sent= dropped= overlimits=" lines, in profile_ifaces order
#        topology.sh relay-down | relay-up                 # the relay's network card
#        topology.sh truth-check                           # the qdisc counters count packets, not GSO batches (run after up)
set -euo pipefail

WAN=11.0.0
declare -A WAN_HOST=([relay]=10 [nat-a]=2 [nat-b]=3 [nat-c]=4)
# carol only exists in mesh mode (see up).
CAROL_HOST=20
declare -A LAN=([nat-a]=10.1.0 [nat-b]=10.2.0 [nat-c]=10.3.0)
NAMESPACES=(wan relay nat-a nat-b nat-c alice bob carol)
MESH=0

# segment_early NS DEV: an endpoint's own interface hands the network single packets. quic-go sends a run of
# full-size packets as one UDP GSO batch (TCP does the same with TSO); a veth passes the batch on whole, so the
# netem of the NAT behind it would drop or keep the batch as one skb and count one drop for all its packets
# (plan BK6: C5's truth undercounted a lost batch). With gso_max_segs 1 the kernel segments the batch in
# software as it leaves the endpoint, as a Wi-Fi card does (one frame per datagram). iproute2 rather than
# `ethtool -K DEV tx-udp-segmentation off`: no new tool, it works in the user namespace, and it covers TCP
# too. `truth-check` proves it.
segment_early() {
  ip -n "$1" link set dev "$2" gso_max_segs 1
}

# lan NAT HOST HOST_IF: a point-to-point link from the NAT's eth1 to HOST_IF.
lan() {
  local nat=$1 host=$2 ifname=$3
  ip -n "$nat" link add eth1 type veth peer name "$ifname" netns "$host"
  segment_early "$host" "$ifname"
  ip -n "$nat" addr add "${LAN[$nat]}.1/24" dev eth1
  ip -n "$nat" link set eth1 up
}

# nat NAT RANDOM: forwarding, masquerade and counters in the NAT's namespace.
nat() {
  local nat=$1 random=$2
  ip netns exec "$nat" sysctl -qw net.ipv4.ip_forward=1
  ip netns exec "$nat" nft -f - <<RULES
table ip nat {
  chain post {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "eth0" masquerade $random
  }
}
table ip count {
  counter direct {}
  counter egress {}
  chain watch {
    type filter hook forward priority filter; policy accept;
    iifname "eth0" ip saddr != $WAN.${WAN_HOST[relay]} meta l4proto { tcp, udp } counter name "direct"
    iifname "eth1" counter name "egress"
  }
  # Like a home router: unsolicited packets from the wan are dropped, silently. Were
  # they answered (ICMP unreachable), conntrack would keep an entry for them, and that
  # entry would force the NAT to change the port of the very hole punch it is meant
  # to let through, which would defeat the cone NAT.
  chain guard {
    type filter hook input priority filter; policy accept;
    iifname "eth0" ct state new drop
  }
}
RULES
}

up() {
  local mode=$1 random=""
  case $mode in
    cone) ;;
    symmetric) random=fully-random ;;
    mesh)
      random=fully-random
      MESH=1
      ;;
    *)
      echo "topology: unknown NAT mode '$mode' (cone, symmetric or mesh)" >&2
      exit 2
      ;;
  esac
  ip link set lo up
  local ns
  for ns in "${NAMESPACES[@]}"; do
    [[ $ns == carol && $MESH == 0 ]] && continue
    ip netns add "$ns"
    ip -n "$ns" link set lo up
  done
  ip -n wan link add br-wan type bridge
  ip -n wan addr add "$WAN.1/24" dev br-wan
  ip -n wan link set br-wan up
  local -A hosts=()
  for ns in "${!WAN_HOST[@]}"; do hosts[$ns]=${WAN_HOST[$ns]}; done
  ((MESH)) && hosts[carol]=$CAROL_HOST
  for ns in "${!hosts[@]}"; do
    ip link add "w-$ns" netns wan type veth peer name eth0 netns "$ns"
    ip -n wan link set "w-$ns" master br-wan up
    ip -n "$ns" addr add "$WAN.${hosts[$ns]}/24" dev eth0
    [[ $ns == nat-* ]] || segment_early "$ns" eth0 # the relay and carol are endpoints too
    ip -n "$ns" link set eth0 up
  done
  lan nat-a alice eth0
  lan nat-b bob eth0
  lan nat-c alice eth1
  ip -n bob addr add 10.2.0.2/24 dev eth0
  ip -n bob link set eth0 up
  ip -n bob route add default via 10.2.0.1
  for ns in nat-a nat-b nat-c; do nat "$ns" "$random"; done
  alice_attach nat-a
}

# alice_detach: alice has no network at all (addresses and routes removed).
alice_detach() {
  local ifname
  for ifname in eth0 eth1; do
    ip -n alice addr flush dev "$ifname"
    ip -n alice link set "$ifname" down
  done
}

# alice_attach NAT: alice connects through NAT only.
alice_attach() {
  local nat=$1 ifname
  case $nat in
    nat-a) ifname=eth0 ;;
    nat-c) ifname=eth1 ;;
    *)
      echo "topology: alice has no link to '$nat'" >&2
      exit 2
      ;;
  esac
  alice_detach
  ip -n alice addr add "${LAN[$nat]}.2/24" dev "$ifname"
  ip -n alice link set "$ifname" up
  ip -n alice route add default via "${LAN[$nat]}.1"
}

down() {
  local ns pid
  # A namespace is only freed once its last process exits: a server or socat still running from the
  # previous scenario keeps its veth (w-<ns>) alive, and the next `up` fails with « File exists »
  # (scenario 8 after 7, 2026-11-09). Stop the processes first, then delete everything by name.
  for ns in "${NAMESPACES[@]}"; do
    for pid in $(ip netns pids "$ns" 2>/dev/null); do kill -TERM "$pid" 2>/dev/null || true; done
  done
  sleep 0.5
  for ns in "${NAMESPACES[@]}"; do
    for pid in $(ip netns pids "$ns" 2>/dev/null); do kill -KILL "$pid" 2>/dev/null || true; done
    ip netns del "$ns" 2>/dev/null || true
  done
  # Deleting the namespace `wan` (done above) removes br-wan and the w-<ns> ends with it.
}

# impair NS DEV ARGS...: netem on the egress of DEV. Always `del` then `add`: `replace` keeps the
# options of the previous profile (corrupt survived a change to a profile without it).
impair() {
  local ns=$1 dev=$2
  shift 2
  ip netns exec "$ns" tc qdisc del dev "$dev" root 2>/dev/null || true
  ip netns exec "$ns" tc qdisc add dev "$dev" root netem "$@"
}

clear_qdisc() {
  ip netns exec "$1" tc qdisc del dev "$2" root 2>/dev/null || true
}

# qstat NS DEV: "bytes=<n> sent=<packets> dropped=<n> overlimits=<n>" of the root qdisc, so a scenario can
# prove its impairment really happened (a netem rule that failed to install must turn it red).
qstat() {
  ip netns exec "$1" tc -s qdisc show dev "$2" |
    sed -n 's/.*Sent \([0-9]*\) bytes \([0-9]*\) pkt (dropped \([0-9]*\), overlimits \([0-9]*\).*/bytes=\1 sent=\2 dropped=\3 overlimits=\4/p' |
    head -n1
}

# The interfaces a profile acts on: NS:DEV pairs (plan section 3). Both directions of a host's
# access link are the egress of its NAT's eth0 and eth1.
profile_ifaces() {
  case $1 in
    wifi | wifi-far | wifi-clean | wifi-entry | wifi-reorder | wifi-edge) echo "nat-a:eth0 nat-a:eth1 nat-b:eth0 nat-b:eth1" ;;
    blackout) echo "nat-a:eth0 nat-a:eth1" ;;
    slowrelay) echo "relay:eth0" ;;
    *)
      echo "topology: unknown profile '$1'" >&2
      exit 2
      ;;
  esac
}

# profile NAME [PHASE]: apply a named profile. Every limit is sized explicitly (netem's default of 1000
# packets under `rate` is a queue of minutes). The seed makes the draws replayable, and it is one per
# qdisc and per phase, `seed * 64 + 8 * PHASE + index of the interface in profile_ifaces`: with the same
# seed everywhere, the four Gilbert-Elliott chains made the same draws (nat-a/eth0 and nat-b/eth0 dropped
# exactly 21 and 21 packets) and each phase replayed the previous one's losses (plan C5S, harness 1).
profile() {
  local name=$1 phase=${2:-0} seed=${NETSIM_SEED:-1} args pair i=0
  case $name in
    wifi) args="delay 15ms 5ms distribution normal loss gemodel 1% 30% 50% 0.1% duplicate 0.5% corrupt 0.1% limit 10000" ;;
    wifi-far) args="delay 75ms 10ms distribution normal loss gemodel 1% 30% 50% 0.1% duplicate 0.5% corrupt 0.1% limit 10000" ;;
    wifi-clean) args="delay 15ms 5ms distribution normal limit 10000" ;;
    # P0: independent loss of 1.75 % on the path (0.88 % on each of the two hops), between the good | fair
    # midpoint (1 %) and its rise threshold (2.5 %), so that the first level, entered from unknown, is judged.
    wifi-entry) args="loss random 0.88% limit 10000" ;;
    # P4: no loss; a 40 ms delay with 8 ms of jitter per hop reorders packets several deep at 120 packets/s,
    # but never later than the ACK loop after which the peer's quic-go discards a late packet as a possible
    # duplicate (spec 8.5.2): the level stays good.
    wifi-reorder) args="delay 40ms 8ms distribution normal limit 10000" ;;
    # P5: wifi with a bad state three times less likely: path loss 0.99 %, on the good | fair boundary.
    wifi-edge) args="delay 15ms 5ms distribution normal loss gemodel 0.18% 30% 50% 0.1% duplicate 0.5% corrupt 0.1% limit 10000" ;;
    blackout) args="loss 100% limit 1000" ;;
    slowrelay) args="delay 20ms rate 65536bit limit 24" ;;
    *)
      echo "topology: unknown profile '$name'" >&2
      exit 2
      ;;
  esac
  for pair in $(profile_ifaces "$name"); do
    # shellcheck disable=SC2086 # args is a word list on purpose
    impair "${pair%%:*}" "${pair##*:}" $args seed $((seed * 64 + 8 * phase + i))
    i=$((i + 1))
  done
}

unprofile() {
  local pair
  for pair in $(profile_ifaces "$1"); do clear_qdisc "${pair%%:*}" "${pair##*:}"; done
}

qstats() {
  local pair
  for pair in $(profile_ifaces "$1"); do
    echo "${pair%%:*} ${pair##*:} $(qstat "${pair%%:*}" "${pair##*:}")"
  done
}

# truth_check: the qdisc counters that C5 reads as the truth count packets, not batches (plan BK6). netem
# draws its loss once per skb and adds one to `dropped` for a whole GSO batch, while `Sent ... pkt` counts its
# segments, so a lost batch of k packets reads as one drop. For each of alice and bob, one UDP send of
# GSO_SEGS segments (UDP_SEGMENT) through a `loss 100%` netem on its NAT's wan egress must raise `dropped`
# by at least GSO_SEGS (other packets of the moment only add to it). Run right after `up`, before any profile;
# the relay's address is pinned on the NAT so that no ARP request is lost to the 100 % loss.
GSO_SEGS=10
truth_check() {
  local host nat mac before after
  mac=$(ip -n relay link show eth0 | awk '/link\/ether/ { print $2 }')
  for host in alice:nat-a bob:nat-b; do
    nat=${host##*:} host=${host%%:*}
    ip -n "$nat" neigh replace "$WAN.${WAN_HOST[relay]}" lladdr "$mac" dev eth0 nud permanent
    impair "$nat" eth0 loss 100% limit 1000
    before=$(qstat "$nat" eth0 | sed 's/.*dropped=\([0-9]*\).*/\1/')
    ip netns exec "$host" python3 -I -c '
import socket, sys
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.setsockopt(socket.SOL_UDP, 103, 1200)  # UDP_SEGMENT (linux/udp.h)
s.sendto(b"x" * (int(sys.argv[2]) * 1200), (sys.argv[1], 9))
' "$WAN.${WAN_HOST[relay]}" "$GSO_SEGS"
    sleep 0.2
    after=$(qstat "$nat" eth0 | sed 's/.*dropped=\([0-9]*\).*/\1/')
    clear_qdisc "$nat" eth0
    ip -n "$nat" neigh del "$WAN.${WAN_HOST[relay]}" dev eth0
    if ((after - before < GSO_SEGS)); then
      echo "topology: FAILED truth check: one GSO send of $GSO_SEGS packets from $host raised $nat/eth0 dropped by $((after - before)), not $GSO_SEGS: the qdisc counters undercount lost packets (segmentation must happen before netem)" >&2
      return 1
    fi
  done
}

# counter NAT NAME: the packet count of one of the NAT's counters.
counter() {
  ip netns exec "$1" nft list counter ip count "$2" | awk '/packets/ { print $2 }'
}

case ${1:-} in
  up) up "${2:-cone}" ;;
  down) down ;;
  alice-detach) alice_detach ;;
  alice-attach) alice_attach "${2:?usage: topology.sh alice-attach nat-a|nat-c}" ;;
  impair) impair "${2:?usage: topology.sh impair NS DEV ARGS...}" "${3:?usage: topology.sh impair NS DEV ARGS...}" "${@:4}" ;;
  clear) clear_qdisc "${2:?usage: topology.sh clear NS DEV}" "${3:?usage: topology.sh clear NS DEV}" ;;
  qstat) qstat "${2:?usage: topology.sh qstat NS DEV}" "${3:?usage: topology.sh qstat NS DEV}" ;;
  profile) profile "${2:?usage: topology.sh profile NAME [PHASE]}" "${3:-0}" ;;
  unprofile) unprofile "${2:?usage: topology.sh unprofile NAME}" ;;
  qstats) qstats "${2:?usage: topology.sh qstats NAME}" ;;
  relay-down) ip -n relay link set eth0 down ;;
  relay-up) ip -n relay link set eth0 up ;;
  relay-ip) echo "$WAN.${WAN_HOST[relay]}" ;;
  truth-check) truth_check ;;
  counter) counter "${2:?usage: topology.sh counter NAT direct|egress}" "${3:?usage: topology.sh counter NAT direct|egress}" ;;
  *)
    sed -n '2,/^set -e/p' "$0" | sed '$d' >&2
    exit 2
    ;;
esac
