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
	"github.com/oioio-space/inkwell/protocol/syncack"
)

// ProtocolBulk carries blobs on a separate ordered stream with its own receive
// queue. On QUIC, both lanes share connection credit (spec §8.7.3).
// Since version 2.0.0 (spec §8.1, rule P-14) its streams have the lifecycle of
// the sync streams: the responder's ACK, two places per peer, the order and the
// refusal for capacity; /inkwell/bulk/1.0.0 is neither served nor opened.
const ProtocolBulk = "/inkwell/bulk/2.0.0"

const (
	// maxBulkStreams bounds the bulk streams of one peer for the resource
	// manager, above the lane's two places, which refuse the third (P-14).
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
	lane   *lane
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
		b.lane = &lane{
			name: "bulk", orders: make(map[peer.ID]*inbound), budget: b.budget, maxCost: bulkBuffered,
			pushLocked: func(ps *peerState, payload []byte, cost int) {
				b.pushLocked(transport.Event{Kind: transport.Message, Peer: ps.name, Payload: payload}, cost)
			},
		}
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
// Messages for as long as it lasts, in the order of the peer's bulk streams,
// with the lifecycle of every ordered lane (spec §8.1, rule P-14; see
// serveInbound).
func (b *Bulk) handle(s network.Stream) { b.n.serveInbound(b.lane, s) }

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
	err = frame.WriteProgress(ctx, s, writeTimeout, frame.FloorRate, payload)
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
	// The ACK (rule P-14) orders this stream after the one it replaces: the
	// peer placed it on arrival, before writing the ACK, and nothing is written
	// before the ACK is read. A refusal for capacity is retried within the
	// budget; any other refusal, or the budget's end, fails this Send only.
	octx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	s, _, err := n.openAcked(octx, ps.id, ProtocolBulk, syncack.RoleMove, direct, nil)
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
