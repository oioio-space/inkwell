package p2p

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

// Lifecycle of a peer. libp2p reports connectedness changes; from them and
// from the outbound stream a peerState derives the events:
//
//   - connected and admitted: the outbound stream is opened (openOut);
//   - stream open and acknowledged by the peer (spec §8.7): PeerUp, once;
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
	delete(n.heard, p)
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

// openOut opens the outbound stream of ps and waits for the peer's ACK, then
// announces the peer if it is new (spec §8.7.4). A failed sync opening closes
// the peer unless the failure came from a connection that is already lost.
func (n *Node) openOut(ps *peerState) {
	ctx, cancel := context.WithTimeout(n.ctx, openTimeout)
	defer cancel()
	s, err := n.openSync(ctx, ps.id, false)
	var sample linkSample
	if err == nil {
		sample = n.probeLink(ps.id, s, nil) // before mu: it calls into libp2p
	}

	n.mu.Lock()
	ps.opening = false
	if err == nil && (n.closed || !ps.connected) {
		_ = s.Reset()
		err = transport.ErrClosed
	}
	if err != nil {
		closePeer := ps.connected && !n.closed
		var connErr *network.ConnError
		if closePeer && errors.As(err, &connErr) {
			// The error belongs to a lost connection. A surviving connection
			// may have arrived while ps.opening prevented its own opening.
			closePeer = false
			if slices.ContainsFunc(n.h.Network().ConnsToPeer(ps.id), func(c network.Conn) bool {
				return !c.IsClosed()
			}) {
				n.startOpenLocked(ps)
			}
		}
		n.settleLocked(ps)
		n.mu.Unlock()
		if closePeer {
			_ = n.h.Network().ClosePeer(ps.id)
		}
		return
	}
	ps.out = s
	announced := !ps.up
	if announced {
		ps.up = true
		n.linkUp(ps.id, sample)
		n.h.ConnManager().Protect(ps.id, memberTag)
		n.pushLocked(transport.Event{Kind: transport.PeerUp, Peer: ps.name}, 0)
	}
	n.changedLocked()
	n.wg.Go(func() { n.watchOut(ps, s) })
	if isCircuit(s.Conn().RemoteMultiaddr()) { // QUIC tells the RTT of a direct link
		n.wg.Go(func() { n.measureRTT(ps.id) })
	}
	n.mu.Unlock()
	if announced { // identify has run (NewStream waits for it)
		n.remember(ps.id)
	}
	// A direct connection may have been made while the stream was opening.
	n.maybeUpgrade(ps)
}

// openSync opens a sync stream to p and reads the peer's ACK (spec §8.7.4, rules
// P-1, P-4, P-5); with direct, on a direct connection (the move of upgrade).
func (n *Node) openSync(ctx context.Context, p peer.ID, direct bool) (network.Stream, error) {
	role := syncack.RoleSession
	if direct {
		role = syncack.RoleMove
	}
	return n.openAcked(ctx, p, ProtocolSync, role, direct)
}

// openAcked opens a stream of protocol proto, an ordered lane, to p and reads
// the peer's ACK (spec §8.1, §8.7.4, rules P-4, P-14): opening and ACK share
// ctx's deadline, counted from the open and fired whatever is in flight. A
// stream that is not acknowledged is reset. With direct, the stream is opened on
// a direct connection.
//
// A refusal for capacity (a reset with syncack.BusyCode, rule P-8) is not a
// refusal: the stream is opened again after a backoff, within the same budget,
// the session and the connections untouched; when no retry fits in the budget,
// the opening ends with the budget and the caller acts as on a refusal (spec
// §8.7.4: a session opening closes the peer, a move is abandoned, a bulk send
// fails).
func (n *Node) openAcked(ctx context.Context, p peer.ID, proto protocol.ID, role syncack.Role, direct bool) (network.Stream, error) {
	start := time.Now()
	for attempt := 0; ; attempt++ {
		s, err := n.openOnce(ctx, p, proto, direct)
		if err == nil || !isBusy(err) {
			return s, err
		}
		n.busyRetries.Add(1)
		action, wait := syncack.OnRefusal(role, true, time.Since(start), attempt)
		if action != syncack.Retry { // no retry fits: the opening ends with its budget
			<-ctx.Done()
			return nil, err
		}
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		}
	}
}

// isBusy reports whether err is the remote's refusal for capacity (spec §8.7.4).
func isBusy(err error) bool {
	var se *network.StreamError
	return errors.As(err, &se) && se.Remote && uint32(se.ErrorCode) == syncack.BusyCode
}

// openOnce is one attempt of openAcked.
func (n *Node) openOnce(ctx context.Context, p peer.ID, proto protocol.ID, direct bool) (network.Stream, error) {
	ctx = network.WithAllowLimitedConn(ctx, "inkwell "+string(proto))
	ctx = network.WithNoDial(ctx, "the peer is connected")
	if direct {
		ctx = network.WithForceDirectDial(ctx, "upgrade to a direct connection")
	}
	s, err := n.h.NewStream(ctx, p, proto)
	if err != nil || (n.opts.skipAck && proto == ProtocolSync) {
		return s, err
	}
	// The return of NewStream proves nothing: on the optimistic path no byte of the
	// peer has arrived yet. The deadline resets the stream, which ends the read.
	stop := context.AfterFunc(ctx, func() { _ = s.Reset() })
	err = syncack.ReadAck(s)
	if !stop() && err == nil {
		err = ctx.Err() // the deadline fired as the ACK arrived: too late
	}
	if err != nil {
		_ = s.Reset()
		return nil, fmt.Errorf("p2p: %s stream to %s not accepted: %w", proto, p, err)
	}
	return s, nil
}

// watchOut returns when the outbound stream s ends, which is the only way to
// learn that the remote reset it: nothing is read from it after the ACK.
func (n *Node) watchOut(ps *peerState, s network.Stream) {
	var b [1]byte
	// The remote writes nothing after its ACK; if it does, it is hostile.
	m, err := s.Read(b[:])
	n.outLost(ps, s, outEndedByConn(m, err, s.Conn().IsClosed()), false)
}

// outEndedByConn reports whether an outbound stream that ended with n bytes
// read and err was lost with its connection, rather than refused (spec §8.7.2,
// §8.7.5). What the remote did comes first: a byte after the ACK is hostile and
// a reset by the remote is a refusal, even if the connection is closed by now
// (the remote may reset the stream, then close this connection while another
// survives). Only the connection's own close error, or an end on a connection
// marked closed (connClosed), is the connection lost.
func outEndedByConn(n int, err error, connClosed bool) bool {
	var ce *network.ConnError
	var se *network.StreamError
	switch {
	case n > 0:
		return false
	case errors.As(err, &ce):
		return true
	case errors.As(err, &se) && se.Remote:
		return false
	}
	return connClosed
}

// outLost retires the outbound stream s of ps. With reopen, a new one is
// opened if the peer is still connected; without, the peer is disconnected,
// since a stream reset on a healthy connection means the remote refuses us.
// With resync, a peer that was announced is announced down first and up again
// when the new stream opens, so that the repo layer drops its sync state and
// sends again what the lost stream carried (spec rule L).
func (n *Node) outLost(ps *peerState, s network.Stream, reopen, resync bool) {
	n.mu.Lock()
	if ps.out != s {
		n.mu.Unlock()
		_ = s.Reset()
		return
	}
	ps.out = nil
	n.changedLocked()
	if reopen {
		if resync && ps.connected && !n.closed {
			n.announceDownLocked(ps, true)
		}
		// Keep the state while the stream releases its resource manager slot.
		// A connection event must not start the replacement during that reset.
		ps.opening = true
		n.mu.Unlock()
		_ = s.Reset()
		n.mu.Lock()
		ps.opening = false
		n.startOpenLocked(ps)
	}
	closePeer := !reopen && ps.connected && !n.closed
	n.settleLocked(ps)
	n.mu.Unlock()
	if closePeer {
		_ = n.h.Network().ClosePeer(ps.id)
	}
	if !reopen {
		_ = s.Reset()
	}
}

// settleLocked forgets ps, with a PeerDown if it was announced, once it has
// no stream, no stream being replaced and no connection.
func (n *Node) settleLocked(ps *peerState) {
	if ps.out != nil || ps.opening || ps.connected {
		return
	}
	if n.peers[ps.id] == ps {
		delete(n.peers, ps.id)
	}
	n.announceDownLocked(ps, false)
	n.changedLocked()
}

// announceDownLocked announces ps down if it was announced up, and ends its
// bulk stream. The peer is announced up again when its outbound stream opens.
// With resync the connection survived and only the session restarts (spec rule
// L), which the link figures count apart from an outage.
func (n *Node) announceDownLocked(ps *peerState, resync bool) {
	if !ps.up {
		return
	}
	ps.up = false
	n.linkDown(ps.id, resync)
	if ps.bulkOut != nil {
		_ = ps.bulkOut.Reset()
		ps.bulkOut = nil
	}
	n.h.ConnManager().Unprotect(ps.id, memberTag)
	n.pushLocked(transport.Event{Kind: transport.PeerDown, Peer: ps.name}, 0)
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

// handleStream serves a sync stream opened by a remote: it delivers its frames
// as Messages for as long as it lasts, in the order of the peer's streams, with
// the lifecycle every ordered lane shares (spec §8.7.3, rules P-6, P-8 to P-12;
// see serveInbound).
func (n *Node) handleStream(s network.Stream) { n.serveInbound(n.syncLane, s) }

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
// progress. A failed write leaves the stream in an unknown state and its frame
// lost, so the stream is replaced and the peer is announced down then up again
// (spec rule L), which has the layer above send again what was lost. The
// exception is a write the caller canceled: its frame is lost with no event,
// and the caller that cancels owns that loss (the bridge ends the peer's
// sender and resets the peer, or ends its whole session).
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
	for {
		ps, _, err := n.outStream(ctx, p)
		if err != nil {
			return err
		}
		select {
		case ps.wsem <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		// The peer may have gone and come back while we waited for the other writers: its state
		// is then another one, with its own writer lock, and the lock held is the old one's.
		ps2, s, err := n.outStream(ctx, p)
		if err == nil && ps2 != ps {
			<-ps.wsem
			continue
		}
		if err != nil {
			<-ps.wsem
			return err
		}
		defer func() { <-ps.wsem }()

		finished := watchCancel(ctx)
		err = frame.WriteProgress(ctx, s, writeTimeout, frame.FloorRate, payload)
		if canceled := finished(); err != nil {
			return n.sendFailed(ctx, ps, s, to, err, canceled)
		}
		return nil
	}
}

// watchCancel watches ctx while a write runs. The returned func ends the watch
// and reports whether ctx was canceled before the write returned: a failure is
// then the caller's doing. A failure that comes first (no progress for
// writeTimeout, a dead connection) is not one, even if the caller cancels right
// after. A cancellation always wins when it is what interrupted the write: the
// func waits for the watch's callback if it already started.
func watchCancel(ctx context.Context) (canceled func() bool) {
	const running, returned, interrupted = 0, 1, 2
	var state atomic.Int32
	ran := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(ran)
		state.CompareAndSwap(running, interrupted)
	})
	return func() bool {
		if !stop() {
			<-ran
		}
		return !state.CompareAndSwap(running, returned)
	}
}

// sendFailed replaces the stream s after a failed write. It retires s before
// resetting it and opening a replacement, so that watchOut cannot mistake
// the local reset for a refusal.
// Unless the caller canceled, the frame is lost for good: the peer is
// announced down and up again (spec rule L, §5.5).
func (n *Node) sendFailed(ctx context.Context, ps *peerState, s network.Stream, to transport.PeerID, err error, canceled bool) error {
	n.outLost(ps, s, true, !canceled)
	if canceled {
		err = ctx.Err()
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
// the swap waits for the peer's ACK of the new stream (spec §8.7.4, rule P-6;
// without it the session stays on the circuit), writers are paused for the
// swap, the old stream is closed for writing so that the peer reads it to its
// end, and the peer reads the new stream only after that (see handleStream).
func (n *Node) upgrade(ps *peerState) {
	defer func() {
		n.mu.Lock()
		ps.upgrading = false
		n.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(n.ctx, openTimeout)
	defer cancel()
	s, err := n.openSync(ctx, ps.id, true)
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
