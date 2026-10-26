package p2p

import (
	"context"
	"fmt"
	"slices"
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
	if _, relay := n.relays[p]; relay {
		return // a relay is infrastructure, never a peer
	}
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
		ps = &peerState{id: p, name: name, wsem: make(chan struct{}, 1), bulkSem: make(chan struct{}, 1)}
		n.peers[p] = ps
	}
	ps.connected, ps.admitted = true, admitted
	n.startOpenLocked(ps)
	n.changedLocked()
}

func (n *Node) onDisconnected(p peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ps := n.peers[p]; ps != nil {
		ps.connected = false
		n.settleLocked(ps)
	}
	n.changedLocked()
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
	n.wg.Go(func() { n.measureRTT(ps.id) })
	n.mu.Unlock()
	// A direct connection may have been made while the stream was opening.
	n.maybeUpgrade(ps)
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
		if ps.bulkOut != nil {
			_ = ps.bulkOut.Reset()
			ps.bulkOut = nil
		}
		n.h.ConnManager().Unprotect(ps.id, memberTag)
		n.pushLocked(transport.Event{Kind: transport.PeerDown, Peer: ps.name}, 0)
	}
	n.changedLocked()
}

// handleBulkStream serves a bulk stream opened by a remote, or resets it if
// the bulk lane is not in use.
func (n *Node) handleBulkStream(s network.Stream) {
	n.mu.Lock()
	b := n.bulk
	n.mu.Unlock()
	if b == nil {
		_ = s.Reset()
		return
	}
	b.handle(s)
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
	// A stream replaced by an upgrade is read to its end before the next one,
	// so that the messages of a peer keep their order.
	n.mu.Lock()
	prev, mine := ps.inTail, make(chan struct{})
	ps.inTail = mine
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
// serialised and a write fails only after writeTimeout without progress, so
// a large frame on a slow link completes; canceling ctx interrupts a write in
// progress. A failed or interrupted write leaves the stream in an unknown
// state, so it is replaced by a new one: the peer stays up and later sends
// work, but the interrupted payload is lost.
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

	err = frame.WriteProgress(ctx, s, writeTimeout, payload)
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

// onConn handles a new libp2p connection of any kind. A relay that connects
// again means the network is back, so the wanted peers are retried at once; a
// direct connection to a peer whose stream is relayed upgrades the stream.
func (n *Node) onConn(c network.Conn) {
	p := c.RemotePeer()
	if _, relay := n.relays[p]; relay {
		n.kickAll()
		return
	}
	if isCircuit(c.RemoteMultiaddr()) {
		return
	}
	n.mu.Lock()
	ps := n.peers[p]
	n.mu.Unlock()
	if ps != nil {
		n.maybeUpgrade(ps)
	}
}

// onDisconn handles the loss of a libp2p connection: if it was the last one to
// a relay, the relay's keeper is told.
func (n *Node) onDisconn(c network.Conn) {
	p := c.RemotePeer()
	down, relay := n.relayDown[p]
	if relay && n.h.Network().Connectedness(p) == network.NotConnected {
		select {
		case down <- struct{}{}:
		default:
		}
	}
}

// maybeUpgrade starts moving the outbound stream of ps to a direct connection
// if it runs through a relay and a direct connection exists.
func (n *Node) maybeUpgrade(ps *peerState) {
	direct := slices.ContainsFunc(n.h.Network().ConnsToPeer(ps.id), func(c network.Conn) bool {
		return !isCircuit(c.RemoteMultiaddr())
	})
	if !direct {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed || ps.upgrading || ps.out == nil || !isCircuit(ps.out.Conn().RemoteMultiaddr()) {
		return
	}
	ps.upgrading = true
	n.wg.Go(func() { n.upgrade(ps) })
}

// upgrade replaces the relayed outbound stream of ps by one on a direct
// connection, without any event and without losing or reordering a frame:
// writers are paused for the swap, the old stream is closed for writing so
// that the peer reads it to its end, and the peer reads the new stream only
// after that (see handleStream).
func (n *Node) upgrade(ps *peerState) {
	defer func() {
		n.mu.Lock()
		ps.upgrading = false
		n.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(n.ctx, openTimeout)
	defer cancel()
	ctx = network.WithAllowLimitedConn(ctx, "inkwell sync")
	ctx = network.WithNoDial(ctx, "the peer is connected")
	ctx = network.WithForceDirectDial(ctx, "upgrade to a direct connection")
	s, err := n.h.NewStream(ctx, ps.id, ProtocolSync)
	if err != nil {
		return
	}
	select {
	case ps.wsem <- struct{}{}:
	case <-n.ctx.Done():
		_ = s.Reset()
		return
	}
	defer func() { <-ps.wsem }()

	n.mu.Lock()
	old := ps.out
	if n.closed || !ps.connected || old == nil || !isCircuit(old.Conn().RemoteMultiaddr()) {
		n.mu.Unlock()
		_ = s.Reset()
		return
	}
	ps.out = s
	n.changedLocked()
	n.wg.Go(func() { n.watchOut(ps, s) })
	n.wg.Go(func() { n.measureRTT(ps.id) })
	n.mu.Unlock()

	// Everything written to old so far reaches the peer before its EOF. The
	// peer resets the stream once it has read it, which ends watchOut(old).
	_ = old.SetWriteDeadline(time.Now().Add(writeTimeout))
	_ = old.CloseWrite()
}
