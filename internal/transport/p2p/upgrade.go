package p2p

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/protocol/directup"
)

// The direct upgrade (block HP1, spec §8.6, ADR 0045): a relayed link to a wanted peer keeps trying to
// become direct. libp2p calls DCUtR once per new relayed connection and never again; a peer that
// restarts, or whose push of /libp2p/dcutr is lost, stays relayed for ever. Each wanted peer's dial loop
// therefore runs an upgrader in its connected branch: it observes the link, asks the pure policy
// (protocol/directup) and runs what it decides, one call at a time, through the DCUtR service. Every round and every non-relay destination, the node's own and libp2p's, passes the punch
// gate (punchgate.go), so what a peer can make the node punch or dial is bounded whoever initiates.

// upgrader is the direct-upgrade state of one member the node holds a circuit with, wanted or not (the
// node keeps it in Node.ups across want, un-want and reconnections, so that a reconnection never refills
// its attempt budget, H-11). Its policy, call and queued are owned by the loop that drives it, which holds
// own: the dial loop of a wanted peer, or the member loop of one the node does not want (memberLoop);
// events and results arrive from other goroutines, under mu, and wake the loop.
type upgrader struct {
	pol  *directup.Policy
	wake chan struct{} // capacity 1: "something to step"
	own  chan struct{} // capacity 1: held by the loop that drives the upgrader
	// drivers counts the loops made to drive it, the one waiting for own included; under Node.mu.
	drivers int

	mu      sync.Mutex
	events  []directup.Event
	results []upResult
	// epochConn is the circuit whose arrival (E7) opened the latest epoch: the calls' rounds open on it (H-20).
	epochConn network.Conn

	call *upCall
	// live is call, for linkLost, which runs outside the loop that owns call.
	live   atomic.Pointer[upCall]
	queued bool // the peer holds a ticket for the node's own call
	// slotCh is closed when the slots change, while the upgrader waits for one; nil otherwise.
	slotCh <-chan struct{}
	timer  *time.Timer
}

// upCall is a call in flight.
type upCall struct {
	cancel          context.CancelFunc
	done            chan struct{}
	epoch           uint64
	rounds0, dests0 int
}

// upResult is how a call returned.
type upResult struct {
	call *upCall
	err  error
}

func newUpgrader(now time.Time) *upgrader {
	return &upgrader{pol: directup.NewPolicy(now, nil), wake: make(chan struct{}, 1), own: make(chan struct{}, 1)}
}

func (u *upgrader) signal() {
	select {
	case u.wake <- struct{}{}:
	default:
	}
}

// event records e for the next step.
func (u *upgrader) event(e directup.Event) {
	u.mu.Lock()
	u.events = append(u.events, e)
	u.mu.Unlock()
	u.signal()
}

// arm makes the loop step again at at, if it is not zero.
func (u *upgrader) arm(at time.Time) {
	if u.timer != nil {
		u.timer.Stop()
		u.timer = nil
	}
	if !at.IsZero() {
		u.timer = time.AfterFunc(time.Until(at), u.signal)
	}
}

// abort cancels the call; it never waits for it (H-14): the call returns at once with its context.
func (u *upCall) abort() {
	u.cancel()
}

// punchEnabled reports whether the node may hole punch at all: it has relays and is not RelayOnly.
func (n *Node) punchEnabled() bool { return n.pgate != nil }

// upgradeStep runs one step of w's upgrader: it applies the results and events that arrived, observes
// the link, and does what the policy decides. It never blocks: the dial loop waits for the upgrader's
// wake, the slots and the peer's changes.
func (n *Node) upgradeStep(ctx context.Context, w *wanted) {
	u := w.up
	if u == nil || !n.punchEnabled() {
		return
	}
	now := time.Now()
	u.mu.Lock()
	events, results := u.events, u.results
	u.events, u.results = nil, nil
	u.mu.Unlock()
	for _, r := range results {
		n.upResult(w, r, now)
	}
	for _, e := range events {
		u.pol.Event(e, now)
	}
	o := n.observe(ctx, w.id)
	if ctx.Err() != nil {
		// The loop is ending: its end decides (upgradeStop cancels the call and drops the ticket; a hand-over
		// to the dial loop keeps both). Stepping now would see the peer unwanted and cancel what the next
		// loop takes over.
		return
	}
	d := u.pol.Step(o, now)
	if d.Cancel && u.call != nil {
		u.call.abort()
	}
	if ps := n.peerState(w.id); d.Nudge && ps != nil {
		n.maybeUpgrade(ps)
	}
	wake := d.Wake
	if o.In && o.Out {
		n.resolveDuplicate(w.id)
		// Nothing signals that the peer moved its session: look again in a second (H-15's 5 s).
		if again := now.Add(dupRecheck); wake.IsZero() || again.Before(wake) {
			wake = again
		}
	}
	u.slotCh = nil
	if d.Action == directup.None {
		if u.queued {
			n.pgate.drop(w.id)
			u.queued = false
		}
		u.arm(wake)
		return
	}
	leaseEnd := n.pgate.ask(w.id, n.announced(w.id))
	if leaseEnd.IsZero() {
		u.queued = true
		next, ch := n.pgate.next()
		u.slotCh = ch
		u.arm(next)
		return
	}
	u.queued = false
	// The call lives on the node's context, not the loop's: a loop that hands the upgrader over (member to
	// dial loop) leaves it running; a loop that stops it cancels it (upgradeStop), and Close ends it.
	n.launch(n.ctx, w, d.Action, u.pol.Start(now), leaseEnd)
	u.arm(time.Time{})
}

// upgradeStop ends w's upgrader when its dial loop ends (unwanted, or the node closing): the call is
// cancelled, never waited for, and its slot and ticket are given back.
func (n *Node) upgradeStop(w *wanted) {
	u := w.up
	if u == nil || !n.punchEnabled() {
		return
	}
	now := time.Now()
	u.arm(time.Time{})
	u.slotCh = nil
	u.queued = false
	u.pol.Step(directup.Obs{}, now) // inactive until the next loop drives it: ladders reset, budgets kept
	if c := u.call; c != nil {
		// Cancelled, not waited for: it stays in flight for the policy until its result, which the next
		// loop applies, so that no call starts before it returns (H-4); it gives its slot back itself.
		c.abort()
	}
	u.mu.Lock()
	// The next loop steps from what it observes. Dropping an E7 here loses nothing: the step above stopped
	// the call in flight, whose result the policy then discards (H-14), and the next loop's first step
	// restarts the policy with its ladders reset and the call due at once (H-6).
	u.events = nil
	u.mu.Unlock()
	n.pgate.drop(w.id)
}

// launch starts the call act to p, of epoch, in a goroutine that only reports its result: Close never
// waits for it (H-14), and it ends with ctx or with its slot's lease.
func (n *Node) launch(ctx context.Context, w *wanted, act directup.Action, epoch uint64, leaseEnd time.Time) {
	u := w.up
	if n.opts.onCall != nil {
		n.opts.onCall(w.id, act)
	}
	// The call ends before its slot's lease (H-5): the slot is back, the call over, when the lease would
	// hand it to another peer, so that no two calls hold one slot. The call gives its slot back when it
	// returns, cancelled or not.
	cctx, cancel := context.WithDeadline(ctx, leaseEnd.Add(-leaseMargin))
	c := &upCall{cancel: cancel, done: make(chan struct{}), epoch: epoch}
	c.rounds0, c.dests0 = n.pgate.counts(w.id)
	u.call = c
	u.live.Store(c)
	go func() {
		defer close(c.done)
		defer cancel()
		var err error
		if act == directup.Dial {
			err = n.dialDirect(cctx, w.id)
		} else {
			err = n.punch(cctx, w.id, n.roundCircuit(w.id, u))
		}
		n.pgate.release(w.id, leaseEnd)
		u.mu.Lock()
		u.results = append(u.results, upResult{call: c, err: err})
		u.mu.Unlock()
		u.signal()
	}()
}

// upResult applies the result of a call.
func (n *Node) upResult(w *wanted, r upResult, now time.Time) {
	u := w.up
	if u.call != r.call {
		return // not the call the upgrader tracks
	}
	u.call = nil
	u.live.CompareAndSwap(r.call, nil)
	rounds, dests := n.pgate.counts(w.id)
	class := directup.Classify(rounds-r.call.rounds0, dests > r.call.dests0, n.hasDirect(w.id), outcome(r.err))
	u.pol.Result(r.call.epoch, class, now) // the call gave its slot back as it returned
}

// outcome sorts the error of a call, by identity, never by its text.
func outcome(err error) directup.Outcome {
	switch {
	case err == nil:
		return directup.OK
	case errors.Is(err, holepunch.ErrHolePunchActive):
		return directup.ErrBusy
	case errors.Is(err, holepunch.ErrClosed):
		return directup.ErrClosed
	}
	return directup.ErrOther
}

// punch calls DCUtR to p through the host's hole-punch service: a direct dial to the public addresses
// the peerstore holds, then up to three rounds over the circuit via (H-20).
func (n *Node) punch(ctx context.Context, p peer.ID, via network.Conn) error {
	if n.opts.punch != nil {
		return n.opts.punch(ctx, p)
	}
	s := n.hps.Load()
	if s == nil {
		return holepunch.ErrClosed
	}
	return n.punchOn(ctx, s, p, via)
}

// roundCircuit is the circuit a call of u's policy to p opens its rounds on (H-20): the inbound circuit that
// opened the current epoch, or, once it is closed, the newest open inbound circuit to p; nil if there is none.
func (n *Node) roundCircuit(p peer.ID, u *upgrader) network.Conn {
	u.mu.Lock()
	c := u.epochConn
	u.mu.Unlock()
	if c != nil && !c.IsClosed() && c.Stat().Direction == network.DirInbound {
		return c
	}
	return n.newestCircuitIn(p)
}

// newestCircuitIn returns the newest open inbound circuit to p, or nil.
func (n *Node) newestCircuitIn(p peer.ID) network.Conn {
	return n.newestConn(p, func(c network.Conn) bool {
		return isCircuit(c.RemoteMultiaddr()) && c.Stat().Direction == network.DirInbound
	})
}

// dialDirect dials p directly at the public addresses it announced, the action of a side that only
// dialled a circuit (H-3): the peer is public, so no punch is needed.
func (n *Node) dialDirect(ctx context.Context, p peer.ID) error {
	addrs := selectAddrs(n.announced(p))
	if len(addrs) == 0 {
		return errNoAddress
	}
	ctx = network.WithForceDirectDial(ctx, "upgrade to a direct connection")
	ctx, cancel := context.WithTimeout(ctx, knownDialTimeout)
	defer cancel()
	return n.h.Connect(ctx, peer.AddrInfo{ID: p, Addrs: addrs})
}

// observe reads the link to p for the policy, holding no lock across libp2p calls.
func (n *Node) observe(ctx context.Context, p peer.ID) directup.Obs {
	o := directup.Obs{Enabled: n.punchEnabled(), Wanted: ctx.Err() == nil}
	n.mu.Lock()
	if ps := n.peers[p]; ps != nil && ps.out != nil {
		relayed := isCircuit(ps.out.Conn().RemoteMultiaddr())
		o.Relayed, o.Direct = relayed, !relayed
	}
	n.mu.Unlock()
	o.Idle = inputWasIdle(n.inputTime(), n.lastActivity.Load())
	for _, c := range n.h.Network().ConnsToPeer(p) {
		if !isCircuit(c.RemoteMultiaddr()) || c.IsClosed() || n.silent(p, c) { // a dead circuit is no duplicate (H-15, P-15)
			continue
		}
		if c.Stat().Direction == network.DirInbound {
			o.In = true
		} else {
			o.Out = true
		}
	}
	o.LocalReady = n.localReady()
	o.RemotePublic = len(selectAddrs(n.announced(p))) > 0
	return o
}

// localReady reports whether the node's own /libp2p/dcutr handler is registered.
func (n *Node) localReady() bool {
	if n.opts.localReady != nil {
		return n.opts.localReady()
	}
	return slices.Contains(n.h.Mux().Protocols(), holepunch.Protocol)
}

// announced returns the addresses p announced through identify.
func (n *Node) announced(p peer.ID) []ma.Multiaddr {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.heard[p])
}

// hasDirect reports whether a direct connection to p exists.
func (n *Node) hasDirect(p peer.ID) bool {
	return slices.ContainsFunc(n.h.Network().ConnsToPeer(p), func(c network.Conn) bool {
		return !isCircuit(c.RemoteMultiaddr()) && !c.IsClosed()
	})
}

// peerState returns what the node knows of p, or nil.
func (n *Node) peerState(p peer.ID) *peerState {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.peers[p]
}

// resolveDuplicate applies H-15 to p, to which the node holds two circuits: the one the higher PeerID
// dialled is closed once the session is off it, so that one circuit gives one initiator. The outbound
// stream of this node is moved off it first; the peer moves its own. The circuit is closed only once
// both session streams are on the other one and it carries nothing but identify: a stream that is
// still being opened on it would otherwise fail, and the peer would drop the link.
func (n *Node) resolveDuplicate(p peer.ID) {
	if n.proving(p) {
		return // a circuit that has not proved it lives is no duplicate (H-15 waits for P-15)
	}
	var in, out network.Conn
	for _, c := range n.h.Network().ConnsToPeer(p) {
		if !isCircuit(c.RemoteMultiaddr()) || c.IsClosed() || n.silent(p, c) { // a dead circuit is no duplicate (H-15, P-15)
			continue
		}
		if c.Stat().Direction == network.DirInbound {
			in = cmpConn(in, c)
		} else {
			out = cmpConn(out, c)
		}
	}
	if in == nil || out == nil {
		return
	}
	dup, keep := in, out
	if n.h.ID() > p {
		dup, keep = out, in
	}
	direct := n.hasDirect(p)
	n.mu.Lock()
	ps := n.peers[p]
	var ourOff bool
	if ps != nil && ps.out != nil && !n.closed {
		old := ps.out
		ourOff = old.Conn() != dup
		// Off the duplicate onto the other circuit, unless a direct connection is there: the session
		// moves to it anyway (maybeUpgrade), and one move at a time.
		if !ourOff && !direct && !ps.upgrading && ps.retiring == nil {
			ps.upgrading = true
			n.wg.Go(func() { n.moveOut(ps, old, keep) })
		}
	}
	n.mu.Unlock()
	if ourOff && peerSessionOff(n.h.Network().ConnsToPeer(p), dup) && idleConn(dup) {
		_ = dup.Close()
	}
}

// cmpConn keeps the newer of two connections (H-20: the older one may be a crashed peer's, dead).
func cmpConn(have, c network.Conn) network.Conn {
	if have == nil || c.Stat().Opened.After(have.Stat().Opened) {
		return c
	}
	return have
}

// peerSessionOff reports whether the peer's session stream (an inbound sync stream) is on one of conns
// other than dup: the peer has opened it, and not on the circuit about to close.
func peerSessionOff(conns []network.Conn, dup network.Conn) bool {
	return slices.ContainsFunc(conns, func(c network.Conn) bool {
		return c != dup && slices.ContainsFunc(c.GetStreams(), func(s network.Stream) bool {
			return s.Protocol() == ProtocolSync && s.Stat().Direction == network.DirInbound
		})
	})
}

// idleConn reports whether c carries nothing but identify: no session stream, no hole-punch exchange,
// and no stream still negotiating its protocol (a session that is being opened on it).
func idleConn(c network.Conn) bool {
	return !slices.ContainsFunc(c.GetStreams(), func(s network.Stream) bool {
		return s.Protocol() != identify.ID && s.Protocol() != identify.IDPush
	})
}

// leaseMargin is how long before its slot's lease a call of the upgrader is cancelled.
const leaseMargin = time.Second

// dupRecheck is how often an upgrader whose peer holds two circuits looks whether the duplicate can be
// closed (H-15).
const dupRecheck = time.Second

// driven returns the upgrader of p if a loop drives it, or nil: events for an upgrader nobody drives
// would only pile up (a peer can make libp2p report them at will).
func (n *Node) driven(p peer.ID) *upgrader {
	n.mu.Lock()
	defer n.mu.Unlock()
	if u := n.ups[p]; u != nil && u.drivers > 0 {
		return u
	}
	return nil
}

// linkLost cancels the call in flight towards p once the node holds no connection to it (H-4): hole punching
// needs the relayed connection it started on, and a call that went on would reopen a link the node has just
// lost or closed (a Drop of a revoked peer). The call still returns, and gives its slot back, before the next
// one starts; a restarted peer's new circuit makes a new call (H-6).
func (n *Node) linkLost(p peer.ID) {
	n.mu.Lock()
	u := n.ups[p]
	n.mu.Unlock()
	if u == nil {
		return
	}
	if c := u.live.Load(); c != nil {
		c.cancel()
	}
}

// upWake makes the upgrader of p, if a loop drives it, step again: a circuit to it closed.
func (n *Node) upWake(p peer.ID) {
	if u := n.driven(p); u != nil {
		u.signal()
	}
}

// upEpoch records E7 for the upgrader of c's peer, if a loop drives it: the new circuit c opens an epoch,
// whose calls open their rounds on it (H-20).
func (n *Node) upEpoch(c network.Conn) {
	if u := n.driven(c.RemotePeer()); u != nil {
		u.mu.Lock()
		u.epochConn = c
		u.mu.Unlock()
		u.event(directup.E7)
	}
}

// upEvent records e for the upgrader of p, if a loop drives it.
func (n *Node) upEvent(p peer.ID, e directup.Event) {
	if u := n.driven(p); u != nil {
		u.event(e)
	}
}

// upEventAll records e for every upgrader a loop drives.
func (n *Node) upEventAll(e directup.Event) {
	n.mu.Lock()
	us := make([]*upgrader, 0, len(n.ups))
	for _, u := range n.ups {
		if u.drivers > 0 {
			us = append(us, u)
		}
	}
	n.mu.Unlock()
	for _, u := range us {
		u.event(e)
	}
}

// upgraderLocked returns the upgrader of p, made on first use, and counts one more loop to drive it; nil
// if the node may not hole punch.
func (n *Node) upgraderLocked(p peer.ID) *upgrader {
	if n.pgate == nil {
		return nil
	}
	u := n.ups[p]
	if u == nil {
		u = newUpgrader(time.Now())
		n.ups[p] = u
	}
	u.drivers++
	return u
}

// drive waits until the loop of w owns its upgrader (the previous loop, cancelled, has stopped it); false
// if ctx ends first. The loop then calls undrive when it ends.
func (n *Node) drive(ctx context.Context, w *wanted) bool {
	if w.up == nil {
		return true
	}
	select {
	case w.up.own <- struct{}{}:
		return true
	case <-ctx.Done():
		n.undrive(w, false)
		return false
	}
}

// undrive ends the loop of w that drove its upgrader (owned), or that never got it: the upgrader is
// stopped, and forgotten once no loop needs it and it carries nothing (Policy.Rested).
func (n *Node) undrive(w *wanted, owned bool) {
	u := w.up
	if u == nil {
		return
	}
	if owned {
		n.upgradeStop(w)
	}
	n.mu.Lock()
	u.drivers--
	if owned && u.drivers == 0 && n.ups[w.id] == u && u.pol.Rested(time.Now()) {
		delete(n.ups, w.id)
	}
	n.mu.Unlock()
	if owned {
		<-u.own
	}
}

// handOver ends the member loop of w that drove its upgrader when the peer's dial loop takes it over: unlike
// undrive, the upgrader is not stopped, so a call in flight goes on (its result is applied by the dial loop,
// H-4) and the ticket keeps its place (H-5); the dial loop then owns it.
func (n *Node) handOver(w *wanted) {
	u := w.up
	if u == nil {
		return
	}
	u.arm(time.Time{})
	n.mu.Lock()
	u.drivers--
	n.mu.Unlock()
	<-u.own
	u.signal() // the dial loop steps from what it observes now
}

// memberDrive is the member loop of one peer the node does not want; under Node.mu.
type memberDrive struct {
	cancel context.CancelFunc
	// again: a circuit came while the loop was deciding to end; it looks again.
	again bool
	// handover: the peer became wanted and its dial loop takes the upgrader over as it is, its call in
	// flight and its ticket's place kept (H-5): the call is not cancelled to be made again on a new grant.
	handover bool
}

// memberUp starts the member loop of p if p is a member the node does not want: the upgrader exists for
// every admitted member the node holds a circuit with (H-1: « wanted » is « member »), since the side
// holding the inbound circuit punches (H-3) whoever wants whom. A stranger (a non-member, or a peer
// admitted only by the invitation window) gets none. Called when a circuit to p appears, and when p is
// no longer wanted.
func (n *Node) memberUp(p peer.ID) {
	if n.pgate == nil || !n.member(p) { // Admit is called outside mu
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, relay := n.relays[p]; relay || n.closed || n.want[p] != nil {
		return
	}
	if d := n.mup[p]; d != nil {
		d.again = true
		if u := n.ups[p]; u != nil {
			u.signal()
		}
		return
	}
	ctx, cancel := context.WithCancel(n.ctx)
	d := &memberDrive{cancel: cancel}
	n.mup[p] = d
	w := &wanted{id: p, up: n.upgraderLocked(p)}
	n.dialWG.Go(func() { n.memberLoop(ctx, w, d) })
}

// memberLoop drives the upgrader of w, a member the node does not want, while it is a member and the
// node holds a circuit to it; it ends when the peer becomes wanted (its dial loop takes the upgrader
// over), at Close, or when the conditions end.
func (n *Node) memberLoop(ctx context.Context, w *wanted, d *memberDrive) {
	defer d.cancel()
	defer n.memberEnd(w.id, d)
	if !n.drive(ctx, w) {
		return
	}
	defer func() {
		n.mu.Lock()
		handover := d.handover
		n.mu.Unlock()
		if handover {
			n.handOver(w)
		} else {
			n.undrive(w, true)
		}
	}()
	for {
		if !n.member(w.id) || n.circuitTo(w.id) == nil {
			// Decided under mu, with the circuit's memberUp: a circuit that comes after starts a new loop.
			n.mu.Lock()
			again := d.again
			d.again = false
			if !again && n.mup[w.id] == d {
				delete(n.mup, w.id)
			}
			n.mu.Unlock()
			if !again {
				return
			}
			continue
		}
		n.mu.Lock()
		chg := n.chg
		n.mu.Unlock()
		n.upgradeStep(ctx, w)
		select {
		case <-chg:
		case <-w.up.wake:
		case <-w.up.slotCh:
		case <-ctx.Done():
			return
		}
	}
}

// memberEnd forgets the member loop d of p, unless a newer one replaced it.
func (n *Node) memberEnd(p peer.ID, d *memberDrive) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.mup[p] == d {
		delete(n.mup, p)
	}
}

// circuitTo returns the newest open relay circuit to p, if one survives (H-20: the oldest may be dead).
func (n *Node) circuitTo(p peer.ID) network.Conn {
	return n.newestConn(p, func(c network.Conn) bool { return isCircuit(c.RemoteMultiaddr()) })
}

// newestConn returns the newest open connection to p that keep accepts, or nil.
func (n *Node) newestConn(p peer.ID, keep func(network.Conn) bool) network.Conn {
	var best network.Conn
	for _, c := range n.h.Network().ConnsToPeer(p) {
		if !c.IsClosed() && keep(c) {
			best = cmpConn(best, c)
		}
	}
	return best
}

// NoteActivity tells the node that the person used the app now (input in its window). The retry
// ladders of the direct upgrade run only while the app is active, within 5 min of the last input; the
// upgrades triggered by events run whatever the activity (spec §8.6, rule H-17). Until the first call
// the node does not track activity and counts as active.
func (n *Node) NoteActivity() {
	at := n.inputTime()
	for last := n.lastActivity.Load(); last < at; last = n.lastActivity.Load() {
		if n.lastActivity.CompareAndSwap(last, at) {
			if inputWasIdle(at, last) {
				n.upEventAll(directup.EActive)
			}
			return
		}
	}
}

func (n *Node) inputTime() int64 { return int64(time.Since(n.born)) + 1 }

func inputWasIdle(at, last int64) bool {
	return last != 0 && directup.InputIdle(time.Duration(at-last), false)
}
