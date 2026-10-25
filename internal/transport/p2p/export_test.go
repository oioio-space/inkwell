package p2p

import (
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
)

// SetExtra makes New apply opts after its own, so tests can swap the network
// (for example for a simulated one).
func SetExtra(o *Options, opts ...libp2p.Option) { o.extra = opts }

// SetRewrite makes New rewrite the addresses the node advertises through f,
// so a simulated node behind a NAT advertises its public address.
func SetRewrite(o *Options, f func([]ma.Multiaddr) []ma.Multiaddr) { o.rewrite = f }

// SetRelayExtra makes RunRelay apply opts after its own, as SetExtra does for
// a node.
func SetRelayExtra(o *RelayOptions, opts ...libp2p.Option) { o.extra = opts }

// Host returns the underlying libp2p host.
func (n *Node) Host() host.Host { return n.h }

// LibP2PID converts a PeerID to the libp2p form; ok is false if it is not
// the canonical encoding of a libp2p identity.
func LibP2PID(id transport.PeerID) (peer.ID, bool) { return parseID(id) }

// ResetOut resets the outbound stream to peer, as a remote that refuses it
// would. It reports whether there was one.
func (n *Node) ResetOut(id transport.PeerID) bool {
	p, ok := parseID(id)
	if !ok {
		return false
	}
	n.mu.Lock()
	ps := n.peers[p]
	if ps == nil || ps.out == nil {
		n.mu.Unlock()
		return false
	}
	s := ps.out
	n.mu.Unlock()
	_ = s.Reset()
	return true
}

// OutDirect reports whether the outbound stream to peer runs on a direct
// connection; ok is false if there is no such stream.
func (n *Node) OutDirect(id transport.PeerID) (direct, ok bool) {
	p, valid := parseID(id)
	if !valid {
		return false, false
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	ps := n.peers[p]
	if ps == nil || ps.out == nil {
		return false, false
	}
	return !isCircuit(ps.out.Conn().RemoteMultiaddr()), true
}
