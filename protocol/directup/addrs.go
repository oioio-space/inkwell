// SPDX-License-Identifier: MPL-2.0

package directup

import "time"

// Addr describes an address a round could use, as far as the selection cares.
type Addr struct {
	QUIC, TCP bool // its transport
	IPv6      bool
	Public    bool // a public IP address
	Circuit   bool // through a relay
	// Name: the address names its host (dns, dns4, dns6, dnsaddr) instead of carrying an IP4 or IP6
	// component. It is refused before selection, whatever its transport: a peer would make the node
	// resolve a name of its choosing (as go-libp2p's open PR #3547 does for hole punching).
	Name bool
}

// Select returns the indices of the addresses a round may use, at most [MaxAddrs], in order: the first
// public QUIC address over IPv4, the first over IPv6, the first public TCP address over IPv4 (H-12). The
// others, circuits, private addresses and names (DNS) included, are dropped: a CONNECT may carry about a
// hundred.
func Select(addrs []Addr) []int {
	var out []int
	var q4, q6, t4 bool
	for i, a := range addrs {
		if !a.Public || a.Circuit || a.Name {
			continue
		}
		switch {
		case a.QUIC && !a.IPv6 && !q4:
			q4 = true
		case a.QUIC && a.IPv6 && !q6:
			q6 = true
		case a.TCP && !a.IPv6 && !t4:
			t4 = true
		default:
			continue
		}
		out = append(out, i)
	}
	return out
}

// InputIdle reports whether elapsed time since the last input or sleep makes
// the app idle (H-17).
func InputIdle(elapsed time.Duration, asleep bool) bool {
	return asleep || elapsed > ActiveWindow
}
