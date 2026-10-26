package p2p

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// ProtocolBulk is the libp2p protocol of the bulk streams, which carry the
// binary files of a project (package blob). It is a second protocol, hence a
// second QUIC stream per direction: a hundred megabytes in flight never delays
// a keystroke of the sync protocol, because QUIC schedules streams
// independently and the sync stream has no byte in front of it to wait for.
const ProtocolBulk = "/inkwell/bulk/1.0.0"

const (
	// maxBulkStreams bounds the bulk streams of one peer: one in use, one
	// being closed after the link moved to a direct connection.
	maxBulkStreams = 4
	// bulkBuffered bounds the received bytes waiting for Bulk.Recv, across
	// peers: one largest frame.
	bulkBuffered = transport.MaxFrame + msgOverhead
)

// Bulk is the bulk lane of a [Node]: a [transport.Transport] over the second
// protocol, [ProtocolBulk]. Get it with [Node.Bulk].
//
// It speaks to the same peers as the node, and announces them with the same
// PeerUp and PeerDown, at the same moment: a peer of the node is a peer of its
// bulk lane, and the admission is the node's. A Send opens the outbound stream
// of the peer on first use, so an idle lane costs nothing. The stream follows
// the link as the node's sync stream does: a relayed link that gets a direct
// connection moves its bulk stream to it, and a failed write is replaced by a
// new stream and loses only the payload it was writing.
//
// Bulk shares its lifetime with the node: Close closes the node.
type Bulk struct {
	n      *Node
	budget *frame.Budget
	wake   chan struct{} // capacity 1: "the queue may be non-empty"
	queue  []queued      // under Node.mu
}

var _ transport.Transport = (*Bulk)(nil)

// Bulk returns the bulk lane of the node, creating it on the first call. Call
// it before the first peer connects to see every PeerUp, or at any time: the
// peers already up are announced to the new lane. A bulk stream that a peer
// opens before the lane exists is reset.
func (n *Node) Bulk() *Bulk {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.bulk == nil {
		b := &Bulk{n: n, budget: frame.NewBudget(bulkBuffered), wake: make(chan struct{}, 1)}
		if n.closed {
			b.budget.Close()
		}
		n.bulk = b
		for _, ps := range n.peers {
			if ps.up {
				b.pushLocked(transport.Event{Kind: transport.PeerUp, Peer: ps.name}, 0)
			}
		}
	}
	return n.bulk
}

// pushLocked queues an event for Recv, with the byte budget it holds.
func (b *Bulk) pushLocked(ev transport.Event, cost int) {
	if b.n.closed {
		return
	}
	b.queue = append(b.queue, queued{ev: ev, cost: cost})
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Local returns the node's identity.
func (b *Bulk) Local() transport.PeerID { return b.n.local }

// Peers returns the peers of the node, sorted.
func (b *Bulk) Peers() []transport.PeerID { return b.n.Peers() }

// Close closes the node, and with it the lane.
func (b *Bulk) Close() error { return b.n.Close() }

// Recv returns the next event of the lane, blocking until one arrives, ctx is
// done or the node is closed (then it returns [transport.ErrClosed]).
func (b *Bulk) Recv(ctx context.Context) (transport.Event, error) {
	n := b.n
	for {
		n.mu.Lock()
		if n.closed {
			n.mu.Unlock()
			return transport.Event{}, transport.ErrClosed
		}
		if len(b.queue) > 0 {
			q := b.queue[0]
			b.queue[0] = queued{}
			b.queue = b.queue[1:]
			n.mu.Unlock()
			b.budget.Release(q.cost)
			return q.ev, nil
		}
		n.mu.Unlock()
		select {
		case <-b.wake:
		case <-n.done: // loop back to the closed check
		case <-ctx.Done():
			return transport.Event{}, ctx.Err()
		}
	}
}

// handle serves a bulk stream opened by a remote: it delivers its frames as
// Messages for as long as it lasts. Streams of the same peer are read one
// after the other, so that its messages keep their order.
func (b *Bulk) handle(s network.Stream) {
	n := b.n
	defer func() { _ = s.Reset() }()
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.wg.Add(1)
	n.mu.Unlock()
	defer n.wg.Done()

	p := s.Conn().RemotePeer()
	if !n.admitted(p) {
		return
	}
	ctx, cancel := context.WithTimeout(n.ctx, upTimeout)
	defer cancel()
	ps, ok := n.waitUp(ctx, p)
	if !ok {
		return
	}
	n.mu.Lock()
	prev, mine := ps.bulkTail, make(chan struct{})
	ps.bulkTail = mine
	n.mu.Unlock()
	defer close(mine)
	if prev != nil {
		wctx, wcancel := context.WithTimeout(n.ctx, reorderWait)
		select {
		case <-prev:
		case <-wctx.Done():
		}
		wcancel()
	}
	for {
		var cost int
		payload, err := frame.ReadReserve(s, transport.MaxFrame, func(size int) error {
			cost = min(size+msgOverhead, bulkBuffered)
			if !b.budget.Acquire(cost) {
				cost = 0
				return transport.ErrClosed
			}
			return nil
		})
		if err != nil {
			b.budget.Release(cost)
			return
		}
		n.mu.Lock()
		if n.peers[p] == ps && ps.up {
			b.pushLocked(transport.Event{Kind: transport.Message, Peer: ps.name, Payload: payload}, cost)
		} else {
			b.budget.Release(cost)
		}
		n.mu.Unlock()
	}
}

// Send writes payload to peer to as one frame on the bulk stream, with the
// guarantees of [Node.Send]: writes to one peer are serialised and bounded by
// 10 s plus one second per MiB, canceling ctx interrupts a write, and a failed
// write replaces the stream and loses only the interrupted payload. It returns
// [transport.ErrUnknownPeer] for a peer that is not up on the node.
func (b *Bulk) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	n := b.n
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.isClosed() {
		return transport.ErrClosed
	}
	if len(payload) > transport.MaxFrame {
		return transport.ErrFrameTooLarge
	}
	p, ok := parseID(to)
	if !ok {
		return transport.ErrUnknownPeer
	}
	ps, err := n.upPeer(p)
	if err != nil {
		return err
	}
	select {
	case ps.bulkSem <- struct{}{}:
		defer func() { <-ps.bulkSem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	s, err := b.stream(ctx, ps)
	if err != nil {
		return err
	}
	err = frame.WriteProgress(ctx, s, writeTimeout, frame.MinRate, payload)
	if err != nil {
		n.mu.Lock()
		if ps.bulkOut == s {
			ps.bulkOut = nil
		}
		n.mu.Unlock()
		_ = s.Reset()
		if cerr := ctx.Err(); cerr != nil {
			err = cerr
		}
		return fmt.Errorf("p2p: bulk send to %s: %w", to, err)
	}
	return nil
}

// upPeer returns the peer state of p if it is announced up.
func (n *Node) upPeer(p peer.ID) (*peerState, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	ps := n.peers[p]
	switch {
	case n.closed:
		return nil, transport.ErrClosed
	case ps == nil || !ps.up:
		return nil, transport.ErrUnknownPeer
	}
	return ps, nil
}

// stream returns the bulk stream to ps, opening it if there is none or if the
// current one runs through a relay while a direct connection exists. The
// caller holds ps.bulkSem, so no two goroutines open one at the same time.
func (b *Bulk) stream(ctx context.Context, ps *peerState) (network.Stream, error) {
	n := b.n
	n.mu.Lock()
	cur := ps.bulkOut
	n.mu.Unlock()
	direct := slices.ContainsFunc(n.h.Network().ConnsToPeer(ps.id), func(c network.Conn) bool {
		return !isCircuit(c.RemoteMultiaddr())
	})
	if cur != nil && (!direct || !isCircuit(cur.Conn().RemoteMultiaddr())) {
		return cur, nil
	}
	octx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	octx = network.WithAllowLimitedConn(octx, "inkwell bulk")
	octx = network.WithNoDial(octx, "the peer is connected")
	if direct {
		octx = network.WithForceDirectDial(octx, "bulk on a direct connection")
	}
	s, err := n.h.NewStream(octx, ps.id, ProtocolBulk)
	if err != nil {
		return nil, fmt.Errorf("p2p: open bulk stream to %s: %w", ps.name, err)
	}
	n.mu.Lock()
	if n.closed || !ps.up {
		closed := n.closed
		n.mu.Unlock()
		_ = s.Reset()
		if closed {
			return nil, transport.ErrClosed
		}
		return nil, transport.ErrUnknownPeer
	}
	old := ps.bulkOut
	ps.bulkOut = s
	n.mu.Unlock()
	if old != nil {
		// Everything written to old so far reaches the peer before its EOF;
		// the peer resets it once read, and reads the new stream after it.
		_ = old.SetWriteDeadline(time.Now().Add(writeTimeout))
		_ = old.CloseWrite()
	}
	return s, nil
}
