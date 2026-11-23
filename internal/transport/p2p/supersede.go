package p2p

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multistream"

	"github.com/oioio-space/inkwell/protocol/syncack"
)

// Superseded connections (spec §8.7.4, rule P-15, block HP1c, plan docs/plans/2026-11-23-hp1-reformalize.md §2).
// An honest peer dials only a peer it is not connected to, so a new connection to a peer says that an older one
// may have died without a close: a crashed peer's circuit stays open at the relay for QUIC's 30 s idle timeout,
// and libp2p keeps preferring it. When a connection to a peer registers and is kept (H-19), every older one
// still open proves it lives: a ping of syncack.ProofLen bytes on that very connection, answered within T_vie
// (syncack.ProofWindow). A silent one is closed once a newer connection carries an acknowledged sync stream;
// its streams then end as streams lost with their connection (the session reopens elsewhere, no event). The
// decision is syncack.Supersede's; this file only keeps its inputs and runs what it decides.

// proofSet is the state rule P-15 decides on: per peer, its registered open connections. mu is a leaf: nothing
// under it takes Node.mu or calls into libp2p.
type proofSet struct {
	mu    sync.Mutex
	peers map[peer.ID]*peerConns
	next  uint64 // the ID of the next connection registered
}

// peerConns are the registered open connections to one peer, and the timer of its next pending proof.
type peerConns struct {
	conns map[network.Conn]*connProof
	timer *time.Timer
}

// connProof is what P-15 knows of one connection: syncack's view of it, and the proof in flight, if any.
type connProof struct {
	sc syncack.Conn
	// run identifies the proof in flight; stop ends it. Both nil once it is dropped or none was started.
	run  *struct{}
	stop context.CancelFunc
}

func newProofSet() *proofSet { return &proofSet{peers: make(map[peer.ID]*peerConns)} }

// proofStart is a proof the node starts: on c, ending at deadline.
type proofStart struct {
	c        network.Conn
	ctx      context.Context
	stop     context.CancelFunc // releases ctx once the proof is over
	run      *struct{}
	deadline time.Time
}

// supersedes reports whether P-15 applies to the connections of p: a relay is infrastructure, never a peer.
func (n *Node) supersedes(p peer.ID) bool {
	_, relay := n.relays[p]
	return !relay
}

// supersedeRegister records c, which the swarm registered and the node keeps (H-19), then applies P-15 to its
// peer: the older connections prove they live.
func (n *Node) supersedeRegister(c network.Conn) {
	p := c.RemotePeer()
	if !n.supersedes(p) {
		return
	}
	ps := n.proofs
	ps.mu.Lock()
	pc := ps.peers[p]
	if pc == nil {
		pc = &peerConns{conns: make(map[network.Conn]*connProof)}
		ps.peers[p] = pc
	}
	if pc.conns[c] == nil && !c.IsClosed() {
		ps.next++
		pc.conns[c] = &connProof{sc: syncack.Conn{ID: ps.next, Opened: n.since()}}
	}
	ps.mu.Unlock()
	n.supersedeStep(p)
}

// supersedeForget forgets c, closed, and applies P-15 again: a proof on the newest connection left is dropped.
func (n *Node) supersedeForget(c network.Conn) {
	p := c.RemotePeer()
	ps := n.proofs
	ps.mu.Lock()
	pc := ps.peers[p]
	if pc == nil {
		ps.mu.Unlock()
		return
	}
	if cp := pc.conns[c]; cp != nil {
		cp.end()
		delete(pc.conns, c)
	}
	if len(pc.conns) == 0 {
		if pc.timer != nil {
			pc.timer.Stop()
		}
		delete(ps.peers, p)
	}
	ps.mu.Unlock()
	n.supersedeStep(p)
}

// supersedeAcked records that c carries a sync stream acknowledged in either direction (§8.7.3, §8.7.4): a
// silent older connection may now be closed.
func (n *Node) supersedeAcked(c network.Conn) {
	p := c.RemotePeer()
	ps := n.proofs
	ps.mu.Lock()
	var changed bool
	if pc := ps.peers[p]; pc != nil {
		if cp := pc.conns[c]; cp != nil && !cp.sc.Acked {
			cp.sc.Acked, changed = true, true
		}
	}
	ps.mu.Unlock()
	if changed {
		n.supersedeStep(p)
	}
}

// proving reports whether a proof of P-15 is in progress on a connection to p (pending: within T_vie, no
// verdict yet). H-15 waits for its end before it settles two circuits, and then leaves a silent one out (silent):
// a dead circuit is no duplicate (spec §8.6).
func (n *Node) proving(p peer.ID) bool {
	ps := n.proofs
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if pc := ps.peers[p]; pc != nil {
		now := n.since()
		for _, cp := range pc.conns {
			if cp.sc.State(now) == syncack.Pending {
				return true
			}
		}
	}
	return false
}

// end drops the proof in flight on the connection, if any; under proofSet.mu.
func (cp *connProof) end() {
	if cp.stop != nil {
		cp.stop()
	}
	cp.run, cp.stop = nil, nil
	cp.sc.Proving, cp.sc.Heard, cp.sc.Unjudged = false, false, false
}

// supersedeStep applies P-15 to the connections of p now: it records what syncack.Supersede decides under
// proofSet.mu, then, without it, starts the proofs and closes the connections.
func (n *Node) supersedeStep(p peer.ID) {
	ps := n.proofs
	ps.mu.Lock()
	pc := ps.peers[p]
	if pc == nil {
		ps.mu.Unlock()
		return
	}
	now := n.since() // under mu: the steps of a peer see time in the order they decide
	conns, byID, newest := pc.open()
	d := syncack.Supersede(now, conns)
	for _, id := range d.Drop {
		pc.conns[byID[id]].end()
	}
	starts := make([]proofStart, 0, len(d.Prove))
	for _, id := range d.Prove {
		c := byID[id]
		cp := pc.conns[c]
		cp.end()
		ctx, stop := context.WithCancel(n.ctx)
		cp.run, cp.stop = new(struct{}), stop
		cp.sc.Proving, cp.sc.Started, cp.sc.For = true, now, newest
		starts = append(starts, proofStart{c: c, ctx: ctx, stop: stop, run: cp.run, deadline: n.born.Add(now + syncack.ProofWindow)})
	}
	closes := make([]network.Conn, 0, len(d.Close))
	for _, id := range d.Close {
		closes = append(closes, byID[id])
	}
	if pc.timer != nil {
		pc.timer.Stop()
		pc.timer = nil
	}
	if d.Wake > 0 && n.ctx.Err() == nil { // no timer outlives Close
		pc.timer = time.AfterFunc(d.Wake-now, func() { n.supersedeStep(p) })
	}
	ps.mu.Unlock()

	if len(starts) > 0 {
		n.mu.Lock()
		closed := n.closed
		for _, s := range starts {
			if closed {
				s.stop()
				continue
			}
			n.wg.Go(func() { n.prove(p, s) })
		}
		n.mu.Unlock()
	}
	for _, c := range closes {
		if n.superseded(p, c) {
			_ = c.Close() // its streams end as lost with their connection (§8.7.5)
		}
	}
}

// open returns P-15's view of the registered connections libp2p still holds open, by ID, and the newest one's
// registration time; under proofSet.mu. A connection libp2p closed is left out at once, before its notification
// reaches the node: the decision is made against the connections as they are.
func (pc *peerConns) open() ([]syncack.Conn, map[uint64]network.Conn, time.Duration) {
	conns := make([]syncack.Conn, 0, len(pc.conns))
	byID := make(map[uint64]network.Conn, len(pc.conns))
	var newest time.Duration
	for c, cp := range pc.conns {
		if c.IsClosed() {
			continue
		}
		conns = append(conns, cp.sc)
		byID[cp.sc.ID] = c
		newest = max(newest, cp.sc.Opened)
	}
	return conns, byID, newest
}

// superseded decides again, right before c is closed, that P-15 closes it now: a newer acknowledged connection
// may have been lost since the decision, and then c is the fallback, perhaps the only connection left.
func (n *Node) superseded(p peer.ID, c network.Conn) bool {
	ps := n.proofs
	ps.mu.Lock()
	defer ps.mu.Unlock()
	pc := ps.peers[p]
	if pc == nil || pc.conns[c] == nil {
		return false
	}
	conns, _, _ := pc.open()
	return slices.Contains(syncack.Supersede(n.since(), conns).Close, pc.conns[c].sc.ID)
}

// proofOutcome is how a proof ended at this node.
type proofOutcome uint8

const (
	// proofNoByte: no byte of the peer before the window ended (or the proof was dropped, or its connection
	// closed). Nothing is recorded: at Started + T_vie, syncack.Supersede reads the proof silent.
	proofNoByte proofOutcome = iota
	// proofHeard: a byte of the peer, the connection lives.
	proofHeard
	// proofUnjudged: the proof failed here before the window ended, without reaching the peer (the node's own
	// resource manager refused the stream, for one): no verdict, never the peer's silence.
	proofUnjudged
)

// prove runs the proof s on its connection, records its outcome and applies P-15 again.
func (n *Node) prove(p peer.ID, s proofStart) {
	defer s.stop()
	ctx, cancel := context.WithDeadline(s.ctx, s.deadline)
	defer cancel()
	out := proveLive(ctx, s.c)
	if n.opts.onProof != nil {
		n.opts.onProof(s.c, out == proofHeard)
	}
	ps := n.proofs
	ps.mu.Lock()
	var changed bool
	if pc := ps.peers[p]; pc != nil && out != proofNoByte {
		if cp := pc.conns[s.c]; cp != nil && cp.run == s.run {
			cp.sc.Heard, cp.sc.Unjudged = out == proofHeard, out == proofUnjudged
			cp.run, cp.stop, changed = nil, nil, true // the proof is over; its record stays for the decision
		}
	}
	ps.mu.Unlock()
	if changed {
		n.supersedeStep(p)
	}
}

// proveLive opens a /ipfs/ping/1.0.0 stream on c, writes a ping of syncack.ProofLen bytes and reports how the
// proof ended before ctx did. Any byte of the peer is an answer (P-15): its reply to the protocol negotiation, a
// refusal of the protocol, or a reset of the stream. The end of ctx (the window, or the proof dropped) or the
// close of c is no byte; any other failure is the node's own and gives no verdict.
func proveLive(ctx context.Context, c network.Conn) proofOutcome {
	ctx = network.WithAllowLimitedConn(ctx, "liveness proof")
	s, err := newStreamOn(ctx, c, ping.ID)
	if err != nil {
		switch {
		case peerAnswered(err):
			return proofHeard
		case ctx.Err() != nil || c.IsClosed():
			return proofNoByte
		}
		return proofUnjudged
	}
	// The negotiation's reply came from the peer: c lives. The ping /ipfs/ping/1.0.0 expects is still written,
	// and the stream closed, without waiting for its echo.
	defer func() { _ = s.Close() }()
	buf := make([]byte, syncack.ProofLen)
	_, _ = rand.Read(buf)
	if dl, ok := ctx.Deadline(); ok {
		_ = s.SetWriteDeadline(dl)
	}
	_, _ = s.Write(buf)
	return proofHeard
}

// peerAnswered reports whether err, the failure of a stream's negotiation, came from the peer: it refused the
// protocol, or reset the stream.
func peerAnswered(err error) bool {
	if _, ok := errors.AsType[multistream.ErrNotSupported[protocol.ID]](err); ok {
		return true
	}
	se, ok := errors.AsType[*network.StreamError](err)
	return ok && se.Remote
}

// close stops the timers of P-15 at Close; the proofs in flight end with the node's context.
func (ps *proofSet) close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, pc := range ps.peers {
		if pc.timer != nil {
			pc.timer.Stop()
		}
	}
}

// silent reports whether c's proof (c a connection to p) ended with no answer within T_vie: P-15 judged it dead, so H-15 does not
// count it as one of two circuits (a dead circuit is no duplicate).
func (n *Node) silent(p peer.ID, c network.Conn) bool {
	ps := n.proofs
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if pc := ps.peers[p]; pc != nil {
		if cp := pc.conns[c]; cp != nil {
			return cp.sc.State(n.since()) == syncack.Silent
		}
	}
	return false
}
