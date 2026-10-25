package p2p

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/oioio-space/inkwell/internal/transport"
)

// LinkKind says how well a peer is connected.
type LinkKind uint8

const (
	// LinkLAN is a direct connection on the local network.
	LinkLAN LinkKind = iota + 1
	// LinkDirect is a direct connection over the internet.
	LinkDirect
	// LinkRelayed is a connection through a relay: slower, and the relay
	// sees who talks to whom, but it works from any network.
	LinkRelayed
)

// String returns the kind's name, for logs.
func (k LinkKind) String() string {
	switch k {
	case LinkLAN:
		return "LinkLAN"
	case LinkDirect:
		return "LinkDirect"
	case LinkRelayed:
		return "LinkRelayed"
	default:
		return "LinkKind(?)"
	}
}

// Link describes the connection to a peer.
type Link struct {
	// Peer is the peer at the other end.
	Peer transport.PeerID
	// RTT is the last round-trip time measured, or zero if none was yet.
	RTT time.Duration
	// Kind is the best connection the peer is reached on.
	Kind LinkKind
}

// Links returns the link to each peer announced with a PeerUp, sorted by peer.
// A relayed link is upgraded to a direct one by hole punching, without any
// event, so poll Links to follow it.
func (n *Node) Links() []Link {
	n.mu.Lock()
	var ids []peer.ID
	for _, ps := range n.peers {
		if ps.up {
			ids = append(ids, ps.id)
		}
	}
	n.mu.Unlock()

	links := make([]Link, 0, len(ids))
	for _, id := range ids {
		name, ok := peerName(id)
		kind := bestKind(n.h.Network().ConnsToPeer(id))
		if !ok || kind == 0 {
			continue
		}
		links = append(links, Link{Peer: name, Kind: kind, RTT: n.h.Peerstore().LatencyEWMA(id)})
	}
	slices.SortFunc(links, func(a, b Link) int { return cmp.Compare(a.Peer, b.Peer) })
	return links
}

// bestKind returns the best kind among conns, or zero if there is none.
func bestKind(conns []network.Conn) LinkKind {
	var best LinkKind
	for _, c := range conns {
		kind := kindOf(c.RemoteMultiaddr())
		if best == 0 || kind < best {
			best = kind
		}
	}
	return best
}

// kindOf returns the kind of a connection to the remote address a.
func kindOf(a ma.Multiaddr) LinkKind {
	switch {
	case isCircuit(a):
		return LinkRelayed
	case manet.IsPrivateAddr(a) || manet.IsIPLoopback(a) || manet.IsIP6LinkLocal(a):
		return LinkLAN
	default:
		return LinkDirect
	}
}

// measureRTT pings id once; libp2p keeps the result for [Node.Links].
func (n *Node) measureRTT(id peer.ID) {
	ctx, cancel := context.WithTimeout(n.ctx, pingTimeout)
	defer cancel()
	select {
	case <-ping.Ping(ctx, n.h, id):
	case <-ctx.Done():
	}
}

// pingTimeout bounds the round-trip measurement of a link.
const pingTimeout = 5 * time.Second
