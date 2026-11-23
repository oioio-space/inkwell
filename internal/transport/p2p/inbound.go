package p2p

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	wire "github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

// The lifecycle of an inbound stream of the two ordered lanes, sync and bulk
// (spec §8.1, §8.7.3, rules P-10 to P-14). A stream has one reader for its
// whole life, the libp2p handler's goroutine:
//
//   - it takes its place in its peer's order of the lane on arrival, before its
//     ACK (P-6), or is refused for capacity: reset with syncack.BusyCode, no byte
//     read (P-8);
//   - it writes the ACK, then waits for its turn (no stream ahead, or
//     reorderWait spent) and its peer's announcement, without reading its
//     frames: it probes one byte at a time, each attempt starting at most
//     InboundProbeEvery after the previous one, which is also the read deadline
//     it sets — nobody else touches it (P-10, P-11). The probed bytes are kept,
//     at most InboundProbeMax, and given to the decoder first. A reset is seen
//     within one period, an end with nothing unread at once: the stream leaves
//     if it holds no byte, keeps its place to its turn if it holds some;
//   - from its turn it clears its deadline and delivers its frames, reserving
//     each one's cost in the lane's receive budget first; blocked on the budget,
//     it is freed by room, by the loss of its own connection or by the node's
//     close (P-12);
//   - wherever it ends, it leaves its place and gives back what it holds.
//
// The flow control of the transport holds the bytes it does not read while it
// waits; on QUIC, the node's initial windows keep credit for a head (P-13).

// lane is one ordered inbound lane: its orders, by peer identity, its receive
// budget and its consumer.
type lane struct {
	name   string
	orders map[peer.ID]*inbound // under Node.mu
	budget *frame.Budget
	// maxCost caps the budget a frame is charged.
	maxCost int
	// pushLocked hands a frame of ps to the lane's consumer, with its charge.
	pushLocked func(ps *peerState, payload []byte, cost int)
}

// inbound is the order of one peer's inbound streams on a lane (spec §8.7.3,
// rules P-8, P-9). It is kept per identity, across the peer's connections and
// states, and forgotten once empty; probed holds the bytes each waiting stream
// probed. Under Node.mu.
type inbound struct {
	order  syncack.Order
	probed map[uint64]int
}

// serveInbound is the lifecycle of the inbound stream s of lane l.
func (n *Node) serveInbound(l *lane, s network.Stream) {
	p := s.Conn().RemotePeer()
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		_ = s.Reset()
		return
	}
	in, id, placed := n.arriveLocked(l, p)
	if !placed {
		n.mu.Unlock()
		n.busyRefusals.Add(1)
		_ = s.ResetWithError(network.StreamErrorCode(syncack.BusyCode))
		return
	}
	n.wg.Add(1)
	n.mu.Unlock()
	defer n.wg.Done()
	defer func() { _ = s.Reset() }()
	defer n.leaveOrder(l, p, in, id)

	var lc syncack.Lifecycle
	lc.Arrive(true)
	if lc.Ack(n.acceptInbound(l, s), n.since()) {
		return
	}
	if l == n.syncLane {
		n.supersedeAcked(s.Conn()) // a silent older connection may now be closed (P-15)
	}
	if n.opts.afterAck != nil && l == n.syncLane {
		n.opts.afterAck(s.Conn())
	}
	if n.opts.afterAckLane != nil {
		n.opts.afterAckLane(l.name, s.Conn())
	}
	ctx, done := n.connContext(s.Conn())
	defer done()
	ps, prefix, ok := n.await(ctx, s, in, id, &lc)
	if !ok {
		return
	}
	n.deliver(ctx, l, s, ps, prefix)
}

// await is the waiting of an accepted stream (rules P-10, P-11): it returns the
// peer's state and the probed bytes once the stream's turn and its peer's
// announcement came, or false once the stream ended or must leave.
func (n *Node) await(ctx context.Context, s network.Stream, in *inbound, id uint64, lc *syncack.Lifecycle) (*peerState, []byte, bool) {
	p := s.Conn().RemotePeer()
	var (
		prefix []byte
		b      [1]byte
		ps     *peerState
	)
	due := time.Now() // start of the next probe attempt
	for sleepUntil(ctx, due) {
		n.mu.Lock()
		closed := n.closed
		if cur := n.peers[p]; cur != nil && cur.up {
			ps = cur
			lc.Up()
		}
		if in.order.Ready(id, n.since()) {
			lc.Turn()
		}
		n.mu.Unlock()
		switch {
		case closed || ctx.Err() != nil || lc.Tick(n.since()):
			return nil, nil, false
		case lc.State() == syncack.Delivering:
			if len(prefix) > 0 {
				n.setProbed(in, id, 0)
			}
			if err := s.SetReadDeadline(time.Time{}); err != nil {
				return nil, nil, false
			}
			return ps, prefix, true
		case lc.State() == syncack.Kept || len(prefix) >= wire.InboundProbeMax:
			// Nothing left to read, or nothing more may be: wait one period.
			due = time.Now().Add(wire.InboundProbeEvery)
			continue
		}
		due = due.Add(wire.InboundProbeEvery)
		if err := s.SetReadDeadline(due); err != nil {
			return nil, nil, false
		}
		m, err := s.Read(b[:])
		prefix = append(prefix, b[:m]...) // the bytes first, then the error (P-11)
		if m > 0 {
			n.setProbed(in, id, len(prefix))
		}
		if lc.Read(m, readEnd(err), n.since()) {
			return nil, nil, false
		}
		if now := time.Now(); now.After(due) { // a slow read: never more than one period between attempts
			due = now
		}
	}
	return nil, nil, false
}

// deliver reads the frames of s, after the probed prefix, and hands them to
// the lane's consumer while ps is the peer's announced state (rule P-12). Each
// frame's cost is reserved before its payload is read, waiting for room until
// the stream's own connection is lost or the node closes.
func (n *Node) deliver(ctx context.Context, l *lane, s network.Stream, ps *peerState, prefix []byte) {
	p := s.Conn().RemotePeer()
	var src io.Reader = s
	if len(prefix) > 0 {
		src = io.MultiReader(bytes.NewReader(prefix), s)
	}
	var cost int
	reserve := func(size int) error {
		c := min(size+msgOverhead, l.maxCost)
		if err := l.budget.AcquireCtx(ctx, c); err != nil {
			return err
		}
		cost = c
		return nil
	}
	for {
		cost = 0
		payload, err := frame.ReadReserve(src, transport.MaxFrame, reserve)
		if err != nil {
			l.budget.Release(cost)
			return
		}
		n.mu.Lock()
		if n.peers[p] == ps && ps.up {
			l.pushLocked(ps, payload, cost)
		} else {
			l.budget.Release(cost)
		}
		n.mu.Unlock()
	}
}

// readEnd classifies how a read of an inbound stream ended.
func readEnd(err error) syncack.ReadEnd {
	var ne net.Error
	var ce *network.ConnError
	switch {
	case err == nil:
		return syncack.ReadOK
	case errors.Is(err, os.ErrDeadlineExceeded) || errors.As(err, &ne) && ne.Timeout():
		return syncack.ReadDeadline
	case errors.Is(err, io.EOF):
		return syncack.ReadEOF
	case errors.As(err, &ce):
		return syncack.ReadConnLost
	}
	return syncack.ReadReset
}

// sleepUntil waits until t or the context ends; it reports false on cancellation.
func sleepUntil(ctx context.Context, t time.Time) bool {
	d := time.Until(t)
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// since is the time since the node started: the clock of the inbound orders.
func (n *Node) since() time.Duration { return time.Since(n.born) }

// arriveLocked gives a new inbound stream of p its place in p's order on lane
// l; placed is false when p holds every place (a refusal for capacity).
func (n *Node) arriveLocked(l *lane, p peer.ID) (in *inbound, id uint64, placed bool) {
	in = l.orders[p]
	if in == nil {
		in = &inbound{order: syncack.Order{Wait: reorderWait}, probed: make(map[uint64]int)}
		l.orders[p] = in
	}
	n.nextIn++
	return in, n.nextIn, in.order.Arrive(n.nextIn, n.since())
}

// leaveOrder removes stream id from p's order on lane l, at once and wherever
// it stands (P-9); the streams whose turn comes with it see it at their next
// probe.
func (n *Node) leaveOrder(l *lane, p peer.ID, in *inbound, id uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	in.order.Leave(id)
	delete(in.probed, id)
	if in.order.Len() == 0 && l.orders[p] == in {
		delete(l.orders, p)
	}
}

// setProbed records how many bytes stream id holds by probing.
func (n *Node) setProbed(in *inbound, id uint64, k int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if k == 0 {
		delete(in.probed, id)
	} else {
		in.probed[id] = k
	}
}

// acceptInbound is the responder's side of spec §8.7.3 (rules P-2, P-3), on
// both lanes (P-14): it writes the ACK on s only if the node is open, the peer
// is admitted and the stream's connection is not closed, and it does so before
// waiting for its own PeerUp of the peer, so that crossed opens never wait on
// each other. It reports whether it wrote the ACK; if not, the caller resets s.
func (n *Node) acceptInbound(l *lane, s network.Stream) bool {
	r := syncack.Responder{
		NodeClosed: n.isClosed(),
		Admitted:   n.admitted(s.Conn().RemotePeer()),
		ConnClosed: s.Conn().IsClosed(),
	}
	if !r.Accept() || (l == n.syncLane && n.opts.beforeAck != nil && !n.opts.beforeAck(s.Conn())) {
		return false
	}
	if n.opts.ackHold != nil {
		n.opts.ackHold(l.name, s)
	}
	if err := s.SetWriteDeadline(time.Now().Add(openTimeout)); err != nil {
		return false
	}
	return syncack.WriteAck(s) == nil
}

// connContext returns a context canceled when the connection c is lost or the
// node closes; done releases it.
func (n *Node) connContext(c network.Conn) (context.Context, func()) {
	ctx, cancel := context.WithCancel(n.ctx)
	n.mu.Lock()
	w := n.connWaits[c]
	if w == nil {
		w = make(map[*context.CancelFunc]struct{})
		n.connWaits[c] = w
	}
	w[&cancel] = struct{}{}
	n.mu.Unlock()
	if c.IsClosed() { // lost before it was registered
		cancel()
	}
	return ctx, func() {
		n.mu.Lock()
		delete(n.connWaits[c], &cancel)
		if len(n.connWaits[c]) == 0 {
			delete(n.connWaits, c)
		}
		n.mu.Unlock()
		cancel()
	}
}

// connLost cancels the contexts of connContext(c).
func (n *Node) connLost(c network.Conn) {
	n.mu.Lock()
	w := n.connWaits[c]
	delete(n.connWaits, c)
	n.mu.Unlock()
	for cancel := range w {
		(*cancel)()
	}
}
