package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// Lifecycle of a peer. libp2p reports connectedness changes; from them and
// from the outbound stream a peerState derives the events:
//
//   - connected and admitted: the outbound stream is opened (openOut);
//   - stream open: PeerUp, once;
//   - stream lost while connected: it is reopened silently when the loss comes
//     from our side or from the connection (a Send that failed, a connection
//     replaced by another), and the peer is disconnected when the remote
//     reset a stream of a healthy connection, since it refuses us;
//   - no stream and not connected: PeerDown, once, and the state is forgotten.

// onConnectedness handles a libp2p connectedness change.
func (n *Node) onConnectedness(e event.EvtPeerConnectednessChanged) {
	switch e.Connectedness {
	case network.Connected, network.Limited:
		n.onConnected(e.Peer)
	case network.NotConnected:
		n.onDisconnected(e.Peer)
	}
}

func (n *Node) onConnected(p peer.ID) {
	name, ok := peerName(p)
	if !ok {
		return
	}
	admitted := n.admitted(p)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return
	}
	ps := n.peers[p]
	if ps == nil {
		ps = &peerState{id: p, name: name, wsem: make(chan struct{}, 1)}
		n.peers[p] = ps
	}
	ps.connected, ps.admitted = true, admitted
	n.startOpenLocked(ps)
}

func (n *Node) onDisconnected(p peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ps := n.peers[p]; ps != nil {
		ps.connected = false
		n.settleLocked(ps)
	}
}

// startOpenLocked starts opening the outbound stream of ps if it needs one.
func (n *Node) startOpenLocked(ps *peerState) {
	if n.closed || ps.out != nil || ps.opening || !ps.connected || !ps.admitted {
		return
	}
	ps.opening = true
	n.wg.Go(func() { n.openOut(ps) })
}

// openOut opens the outbound stream of ps, then announces the peer if it is
// new. If it cannot, the connection is useless for syncing and is closed.
func (n *Node) openOut(ps *peerState) {
	ctx, cancel := context.WithTimeout(n.ctx, openTimeout)
	defer cancel()
	ctx = network.WithAllowLimitedConn(ctx, "inkwell sync")
	ctx = network.WithNoDial(ctx, "the peer is connected")
	s, err := n.h.NewStream(ctx, ps.id, ProtocolSync)

	n.mu.Lock()
	ps.opening = false
	if err == nil && (n.closed || !ps.connected) {
		_ = s.Reset()
		err = transport.ErrClosed
	}
	if err != nil {
		closePeer := ps.connected && !n.closed
		n.settleLocked(ps)
		n.mu.Unlock()
		if closePeer {
			_ = n.h.Network().ClosePeer(ps.id)
		}
		return
	}
	ps.out = s
	if !ps.up {
		ps.up = true
		n.h.ConnManager().Protect(ps.id, memberTag)
		n.pushLocked(transport.Event{Kind: transport.PeerUp, Peer: ps.name}, 0)
	}
	n.changedLocked()
	n.wg.Go(func() { n.watchOut(ps, s) })
	n.mu.Unlock()
}

// watchOut returns when the outbound stream s ends, which is the only way to
// learn that the remote reset it: nothing is ever read from it.
func (n *Node) watchOut(ps *peerState, s network.Stream) {
	var b [1]byte
	// The remote never writes on this stream; if it does, it is hostile.
	_, _ = s.Read(b[:])
	connGone := s.Conn().IsClosed()
	n.outLost(ps, s, connGone)
	_ = s.Reset()
}

// outLost retires the outbound stream s of ps. With reopen, a new one is
// opened if the peer is still connected; without, the peer is disconnected,
// since a stream reset on a healthy connection means the remote refuses us.
func (n *Node) outLost(ps *peerState, s network.Stream, reopen bool) {
	n.mu.Lock()
	if ps.out != s {
		n.mu.Unlock()
		return
	}
	ps.out = nil
	n.changedLocked()
	closePeer := false
	if ps.connected && !n.closed {
		if reopen {
			n.startOpenLocked(ps)
		} else {
			closePeer = true
		}
	}
	n.settleLocked(ps)
	n.mu.Unlock()
	if closePeer {
		_ = n.h.Network().ClosePeer(ps.id)
	}
}

// settleLocked forgets ps, with a PeerDown if it was announced, once it has
// no stream, no stream being opened and no connection.
func (n *Node) settleLocked(ps *peerState) {
	if ps.out != nil || ps.opening || ps.connected {
		return
	}
	if n.peers[ps.id] == ps {
		delete(n.peers, ps.id)
	}
	if ps.up {
		ps.up = false
		n.h.ConnManager().Unprotect(ps.id, memberTag)
		n.pushLocked(transport.Event{Kind: transport.PeerDown, Peer: ps.name}, 0)
	}
	n.changedLocked()
}

// handleStream serves a stream opened by a remote: it delivers its frames as
// Messages for as long as it lasts.
func (n *Node) handleStream(s network.Stream) {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		_ = s.Reset()
		return
	}
	n.wg.Add(1)
	n.mu.Unlock()
	defer n.wg.Done()
	defer func() { _ = s.Reset() }()

	p := s.Conn().RemotePeer()
	if !n.admitted(p) {
		return
	}
	// Messages come after the peer's PeerUp, which needs our own stream open.
	ctx, cancel := context.WithTimeout(n.ctx, upTimeout)
	defer cancel()
	ps, ok := n.waitUp(ctx, p)
	if !ok {
		return
	}
	for {
		var cost int
		payload, err := frame.ReadReserve(s, transport.MaxFrame, func(size int) error {
			cost = min(size+msgOverhead, maxBuffered)
			if !n.budget.Acquire(cost) {
				cost = 0
				return transport.ErrClosed
			}
			return nil
		})
		if err != nil {
			n.budget.Release(cost)
			return
		}
		n.mu.Lock()
		if n.peers[p] == ps && ps.up {
			n.pushLocked(transport.Event{Kind: transport.Message, Peer: ps.name, Payload: payload}, cost)
		} else {
			n.budget.Release(cost)
		}
		n.mu.Unlock()
	}
}

// waitUp waits until p is announced, or ctx is done.
func (n *Node) waitUp(ctx context.Context, p peer.ID) (*peerState, bool) {
	for {
		n.mu.Lock()
		ps := n.peers[p]
		if ps != nil && ps.up {
			n.mu.Unlock()
			return ps, true
		}
		chg, closed := n.chg, n.closed
		n.mu.Unlock()
		if closed {
			return nil, false
		}
		select {
		case <-chg:
		case <-ctx.Done():
			return nil, false
		}
	}
}

// outStream returns the announced peer's outbound stream, waiting while it is
// being replaced. It returns [transport.ErrUnknownPeer] if the peer is not
// announced (any more) and [transport.ErrClosed] after Close.
func (n *Node) outStream(ctx context.Context, p peer.ID) (*peerState, network.Stream, error) {
	for {
		n.mu.Lock()
		ps := n.peers[p]
		switch {
		case n.closed:
			n.mu.Unlock()
			return nil, nil, transport.ErrClosed
		case ps == nil || !ps.up:
			n.mu.Unlock()
			return nil, nil, transport.ErrUnknownPeer
		case ps.out != nil:
			s := ps.out
			n.mu.Unlock()
			return ps, s, nil
		}
		chg := n.chg
		n.mu.Unlock()
		select {
		case <-chg:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
}

// Send writes payload to peer to as one frame. Writes to one peer are
// serialised and each is bounded by 10 s plus one second per MiB; canceling
// ctx interrupts a write in progress. A failed or interrupted write leaves
// the stream in an unknown state, so it is replaced by a new one: the peer
// stays up and later sends work, but the interrupted payload is lost.
func (n *Node) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
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
	ps, _, err := n.outStream(ctx, p)
	if err != nil {
		return err
	}
	select {
	case ps.wsem <- struct{}{}:
		defer func() { <-ps.wsem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	// The stream may have been replaced while waiting for the other writers.
	ps, s, err := n.outStream(ctx, p)
	if err != nil {
		return err
	}

	timeout := writeTimeout + time.Duration(len(payload))*time.Second/writeRate
	if err := s.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return n.sendFailed(ctx, ps, s, to, err)
	}
	stop := context.AfterFunc(ctx, func() { _ = s.SetWriteDeadline(time.Now()) })
	err = frame.Write(s, payload)
	stop()
	if err != nil {
		return n.sendFailed(ctx, ps, s, to, err)
	}
	return nil
}

// sendFailed replaces the stream s after a failed write. It retires s before
// resetting it, so that watchOut sees a stream we gave up, not a refusal.
func (n *Node) sendFailed(ctx context.Context, ps *peerState, s network.Stream, to transport.PeerID, err error) error {
	n.outLost(ps, s, true)
	_ = s.Reset()
	if cerr := ctx.Err(); cerr != nil {
		err = cerr
	}
	return fmt.Errorf("p2p: send to %s: %w", to, err)
}
