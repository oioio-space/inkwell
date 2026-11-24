// SPDX-License-Identifier: MPL-2.0

package admit

import "net/netip"

// The prefixes of an IP origin (spec §16.1, « connexions non vérifiées »; pinned to protocol/limits.go by
// TestPrefixesMatchSpecification: the package imports nothing that does I/O, the protocol root included).
const (
	IPv4Bits     = 32 // protocol.PendingIPv4PrefixBits
	IPv6Bits     = 56 // protocol.PendingIPv6PrefixBits
	IPv6WideBits = 48 // protocol.PendingIPv6WidePrefixBits
)

// Origin is what an inbound connection is charged to before its handshake (RF-1): a relay, named by its
// authenticated PeerID, or an IP prefix. The zero Origin is no origin: an outbound connection's.
type Origin struct {
	// Relay is the PeerID of the relay of a circuit; empty for a direct connection.
	Relay string
	// Net is the IPv4 /32 or the IPv6 /56 of a direct connection.
	Net netip.Prefix
	// Wide is the IPv6 /48 of a direct connection, counted as well (§16.1: 64 per /48); zero for IPv4.
	Wide netip.Prefix
}

// RelayOrigin is the origin of a circuit through the relay whose PeerID is id (RF-1): never the relay's IP
// address, which every circuit through it shares.
func RelayOrigin(id string) Origin { return Origin{Relay: id} }

// IPOrigin is the origin of a direct connection from a (RF-1): its IPv4 /32 or its IPv6 /56 and /48. An
// IPv4-mapped IPv6 address is the IPv4 one. An invalid address is no origin.
func IPOrigin(a netip.Addr) Origin {
	a = a.Unmap()
	switch {
	case a.Is4():
		return Origin{Net: netip.PrefixFrom(a, IPv4Bits).Masked()}
	case a.Is6():
		return Origin{
			Net:  netip.PrefixFrom(a, IPv6Bits).Masked(),
			Wide: netip.PrefixFrom(a, IPv6WideBits).Masked(),
		}
	}
	return Origin{}
}

// IsZero reports whether o is no origin.
func (o Origin) IsZero() bool { return o == Origin{} }

// String is "relay:<PeerID>", "ip:<prefix>" or "none".
func (o Origin) String() string {
	switch {
	case o.Relay != "":
		return "relay:" + o.Relay
	case o.Net.IsValid():
		return "ip:" + o.Net.String()
	}
	return "none"
}
