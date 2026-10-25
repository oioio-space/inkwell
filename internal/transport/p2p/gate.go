package p2p

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	ma "github.com/multiformats/go-multiaddr"
)

// gate is the node's connection gater. A peer is let in once it is
// authenticated and Options.Admit accepts it; the configured relays are always
// let in, since the node needs them whoever else is admitted. In RelayOnly
// mode a peer other than a relay must come through a relay circuit, so the
// node never exchanges a packet with its address.
type gate struct {
	*conngater.BasicConnectionGater
	admit     func(peer.ID) bool
	relays    map[peer.ID]peer.AddrInfo
	relayOnly bool
}

func (g gate) isRelay(p peer.ID) bool {
	_, ok := g.relays[p]
	return ok
}

// InterceptAddrDial refuses, in RelayOnly mode, to dial a direct address of
// anyone but a relay.
func (g gate) InterceptAddrDial(p peer.ID, a ma.Multiaddr) bool {
	return !g.relayOnly || g.isRelay(p) || isCircuit(a)
}

// InterceptSecured decides on an authenticated connection.
func (g gate) InterceptSecured(_ network.Direction, p peer.ID, addrs network.ConnMultiaddrs) bool {
	if g.isRelay(p) {
		return true
	}
	if g.relayOnly && !isCircuit(addrs.RemoteMultiaddr()) {
		return false
	}
	return g.admit(p)
}

// isCircuit reports whether a goes through a relay circuit.
func isCircuit(a ma.Multiaddr) bool {
	_, err := a.ValueForProtocol(ma.P_CIRCUIT)
	return err == nil
}
