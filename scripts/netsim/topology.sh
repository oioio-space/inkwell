#!/usr/bin/env bash
# Network-namespace topology for the netsim scenarios (see run.sh, docs/relay.md).
#
#   wan 11.0.0.0/24: bridge br-wan in the current namespace (the test runner)
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
# Must run inside user+net+mount namespaces (run.sh sets them up); touches nothing else.
#
# Usage: topology.sh up cone|symmetric
#        topology.sh down
#        topology.sh alice-detach
#        topology.sh alice-attach nat-a|nat-c
#        topology.sh relay-ip                              # prints the relay's wan address
#        topology.sh counter nat-a|nat-b|nat-c direct|egress   # prints a packet count
set -euo pipefail

WAN=11.0.0
declare -A WAN_HOST=([relay]=10 [nat-a]=2 [nat-b]=3 [nat-c]=4)
declare -A LAN=([nat-a]=10.1.0 [nat-b]=10.2.0 [nat-c]=10.3.0)
NAMESPACES=(relay nat-a nat-b nat-c alice bob)

# lan NAT HOST HOST_IF: a point-to-point link from the NAT's eth1 to HOST_IF.
lan() {
  local nat=$1 host=$2 ifname=$3
  ip -n "$nat" link add eth1 type veth peer name "$ifname" netns "$host"
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
    *)
      echo "topology: unknown NAT mode '$mode' (cone or symmetric)" >&2
      exit 2
      ;;
  esac
  ip link set lo up
  ip link add br-wan type bridge
  ip addr add "$WAN.1/24" dev br-wan
  ip link set br-wan up
  local ns
  for ns in "${NAMESPACES[@]}"; do
    ip netns add "$ns"
    ip -n "$ns" link set lo up
  done
  for ns in "${!WAN_HOST[@]}"; do
    ip link add "w-$ns" type veth peer name eth0 netns "$ns"
    ip link set "w-$ns" master br-wan up
    ip -n "$ns" addr add "$WAN.${WAN_HOST[$ns]}/24" dev eth0
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
  local ns
  for ns in "${NAMESPACES[@]}"; do
    ip netns del "$ns" 2>/dev/null || true
  done
  ip link del br-wan 2>/dev/null || true
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
  relay-ip) echo "$WAN.${WAN_HOST[relay]}" ;;
  counter) counter "${2:?usage: topology.sh counter NAT direct|egress}" "${3:?usage: topology.sh counter NAT direct|egress}" ;;
  *)
    sed -n '2,/^set -e/p' "$0" | sed '$d' >&2
    exit 2
    ;;
esac
