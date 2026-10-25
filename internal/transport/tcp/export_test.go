package tcp

import "github.com/oioio-space/inkwell/internal/transport"

// ConnCount returns how many live connections claim peer.
func (n *Node) ConnCount(peer transport.PeerID) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ps := n.peers[peer]; ps != nil {
		return len(ps.conns)
	}
	return 0
}
