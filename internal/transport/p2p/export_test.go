package p2p

import (
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/oioio-space/inkwell/internal/transport"
)

// SetExtra makes New apply opts after its own, so tests can swap the network
// (for example for a simulated one).
func SetExtra(o *Options, opts ...libp2p.Option) { o.extra = opts }

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
