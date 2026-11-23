package p2p_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

// Block HP1c (plan docs/plans/2026-11-23-hp1-reformalize.md §2, gates/HP1c.toml): after a peer crashes, the
// circuit its old process left stays open, dead, for the relay's idle timeout (about 30 s). Rule H-20 opens
// each DCUtR round on the circuit that causes it, never on the swarm's pick (the old circuit, which carries the
// most streams); rule P-15 makes every older connection prove it lives when a newer one registers, and closes
// a silent one once a newer connection carries an acknowledged sync stream. The tests run in sim_test.go's
// simnet inside testing/synctest; each failure text starts with its rule, so the Go mutants of block HP1c
// (scripts/netsim/mutants.toml) are caught by their `expect`.

// tVie is T_vie, the window of a liveness proof (spec §16).
const tVie = syncack.ProofWindow

// roundLog records the connection each DCUtR round of a node opens on (p2p.SetOnRound).
type roundLog struct {
	mu sync.Mutex
	on []network.Conn
	at []time.Time
}

func (l *roundLog) add(c network.Conn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.on, l.at = append(l.on, c), append(l.at, time.Now())
}

// onAny returns how many rounds opened on one of cs after since.
func (l *roundLog) onAny(cs []network.Conn, since time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := 0
	for i, c := range l.on {
		if !l.at[i].Before(since) && slices.Contains(cs, c) {
			k++
		}
	}
	return k
}

// proofLog records the liveness proofs of a node that end (p2p.SetOnProof).
type proofLog struct {
	mu    sync.Mutex
	heard map[network.Conn]int
}

func (l *proofLog) add(c network.Conn, heard bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.heard == nil {
		l.heard = make(map[network.Conn]int)
	}
	if heard {
		l.heard[c]++
	}
}

func (l *proofLog) answered(c network.Conn) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.heard[c]
}

// crashRestart starts the identity of keySeed again on the endpoint of endSeed: another address and port, so
// the relay and the peers see a new circuit. With crash, the old process b is cut off first: its endpoint is
// blacked out, then it is closed, so no close reaches the relay or a peer, as a process that is killed. Without,
// the old process stays up: a second process of the same identity, alive on its old circuit.
func (nn *natNet) crashRestart(t *testing.T, b *simNode, keySeed, endSeed byte, crash bool, o p2p.Options, want ...*simNode) *simNode {
	t.Helper()
	if crash {
		nn.setBlackout(b.end, true)
		_ = b.Close()
	}
	m := nn.nodeOn(t, keySeed, nn.newEnd(endSeed, b.end.public), o)
	for _, w := range want {
		m.Want(map[transport.PeerID][]string{w.Local(): nil})
	}
	return m
}

// openConns returns n's open connections to peer.
func openConns(n *simNode, to transport.PeerID) []network.Conn {
	p, _ := p2p.LibP2PID(to)
	return slices.DeleteFunc(n.Host().Network().ConnsToPeer(p), func(c network.Conn) bool { return c.IsClosed() })
}

// waitCircuits waits until n holds exactly in inbound and out outbound circuits to peer and nothing else.
func waitCircuits(t *testing.T, n *simNode, to transport.PeerID, in, out int) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		i, o := n.Circuits(to)
		if i == in && o == out && len(openConns(n, to)) == in+out {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture: %s holds %v to %s after %v, want %d inbound and %d outbound circuits only", n.Local(), conns(n, to), to, wait, in, out)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitPunchIdle waits until n has no hole-punch call to peer in flight (the attempts of the relayed link before
// the crash have ended: a crash during an exchange is not the scenario, as in DirectUpgrade.tla's RCrash); it
// fails after the slow ladder's first step and the one-shot's attempts, 3 min.
func waitPunchIdle(t *testing.T, n *simNode, to transport.PeerID) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for !n.PunchIdle(to) {
		if time.Now().After(deadline) {
			t.Fatalf("fixture: %s still has a hole-punch call to %s in flight after 3 min: %s", n.Local(), to, n.UpgradeDebug(to))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitNewCircuit waits for the first circuit l registered after since, and returns when it registered.
func waitNewCircuit(t *testing.T, l *connLog, since time.Time) time.Time {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		l.mu.Lock()
		for _, r := range l.conns {
			if !r.at.Before(since) && isCircuitAddr(r.c.RemoteMultiaddr()) {
				l.mu.Unlock()
				return r.at
			}
		}
		l.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("fixture: no new circuit within %v of the restart", wait)
		}
		time.Sleep(time.Millisecond)
	}
}

// firstAfter returns when the first event of kind from peer was received after since, or zero.
func (l *eventLog) firstAfter(kind transport.Kind, peer transport.PeerID, since time.Time) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.evs {
		if e.Kind == kind && e.Peer == peer && e.at.After(since) {
			return e.at
		}
	}
	return time.Time{}
}

// deliveries sends a frame from a to b and from b to a every 250 ms from now until each side has received one,
// or until deadline; it returns when each direction first delivered (zero if it did not). ea and eb record the
// events of a and b.
func deliveries(a, b *simNode, ea, eb *eventLog, deadline time.Time) (ab, ba time.Time) {
	since := time.Now()
	for k := 0; time.Now().Before(deadline); k++ {
		ab, ba = eb.firstAfter(transport.Message, a.Local(), since), ea.firstAfter(transport.Message, b.Local(), since)
		if !ab.IsZero() && !ba.IsZero() {
			return ab, ba
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		if ab.IsZero() {
			_ = a.Send(ctx, b.Local(), fmt.Appendf(nil, "a->b %d", k))
		}
		if ba.IsZero() {
			_ = b.Send(ctx, a.Local(), fmt.Appendf(nil, "b->a %d", k))
		}
		cancel()
		time.Sleep(time.Until(since.Add(time.Duration(k+1) * 250 * time.Millisecond)))
	}
	return eb.firstAfter(transport.Message, a.Local(), since), ea.firstAfter(transport.Message, b.Local(), since)
}

// bothDirect waits until a and b both see their link as direct, until deadline; it returns when, or zero.
func bothDirect(a, b *simNode, deadline time.Time) time.Time {
	for time.Now().Before(deadline) {
		if linkTo(a, b.Local()) == transport.LinkDirect && linkTo(b, a.Local()) == transport.LinkDirect {
			return time.Now()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return time.Time{}
}

// closedBy waits until every connection of cs is closed, until deadline; false if one is still open.
func closedBy(cs []network.Conn, deadline time.Time) bool {
	for {
		if !slices.ContainsFunc(cs, func(c network.Conn) bool { return !c.IsClosed() }) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// H-20, P-15: b crashes (its endpoint blacked out, its process killed) and comes back on another endpoint with
// a new circuit to a, while a still holds the dead circuit of b's old process. In the same direction the old
// and new circuits are both inbound at a; crossed, the old one is a's outbound circuit, so a holds two circuits
// and H-15 waits for P-15's proof before it settles them. a's rounds open on the new circuit (H-20); the link
// is direct within 3 s of the new circuit (T_vie + 3 s crossed); the dead circuit is closed within T_vie + 1 s;
// frames flow both ways within T_vie + 2 s, with no PeerDown on either side. 20 seeds each: 40/40 rejects a
// residual failure rate of 7.2 % or more at 95 % confidence (plan §2, as block HP1r).
func TestUpgradeAfterCrashRestart(t *testing.T) {
	for _, crossed := range []bool{false, true} {
		name := "same-direction"
		if crossed {
			name = "crossed-direction"
		}
		t.Run(name, func(t *testing.T) {
			forSeeds(t, func(t *testing.T, nn *natNet) { crashRestartScene(t, nn, crossed) })
		})
	}
}

func crashRestartScene(t *testing.T, nn *natNet, crossed bool) {
	relays := nn.relay(t, p2p.RelayOptions{})
	o := p2p.Options{Relays: relays}
	oa := o
	var rounds roundLog
	p2p.SetOnRound(&oa, rounds.add)
	nn.blockDirect(true) // relayed until the crash: a's only connection to b is the circuit that dies
	a := nn.node(t, 1, oa)
	b := nn.node(t, 2, o)
	in, out := 1, 0
	if crossed {
		a.Want(map[transport.PeerID][]string{b.Local(): nil}) // a dials: the old circuit is outbound at a
		in, out = 0, 1
	} else {
		b.Want(map[transport.PeerID][]string{a.Local(): nil})
	}
	expectUp(t, a, b)
	waitCircuits(t, a, b.Local(), in, out)
	waitPunchIdle(t, a, b.Local())
	old := openConns(a, b.Local())
	ea := watchEvents(t, a)
	l := watchConns(t, a, b)
	crashed := time.Now()
	nn.blockDirect(false)
	// a's punch slots are busy with other peers when the new process comes back, and free half a second after its
	// circuit: the one-shot's direct dial gets no grant and sends nothing (H-16), so the link becomes direct through
	// DCUtR rounds, the ones H-20 binds to the new circuit. With a slot free, the direct dial alone would succeed
	// (b's new process opened its NAT by its own direct dial, H-3) and no round would run.
	release := a.FillSlots(4)
	b2 := nn.crashRestart(t, b, 2, 3, true, o, a) // the new process dials a: a new circuit, inbound at a
	eb := watchEvents(t, b2)
	t0 := waitNewCircuit(t, l, crashed)
	time.Sleep(time.Until(t0.Add(500 * time.Millisecond)))
	release()

	type both struct{ ab, ba time.Time }
	frames := make(chan both, 1)
	go func() {
		ab, ba := deliveries(a, b2, ea, eb, t0.Add(tVie+2*time.Second))
		frames <- both{ab, ba}
	}()
	bound := 3 * time.Second
	if crossed {
		bound = tVie + 3*time.Second // H-15 waits for P-15's proof of the dead circuit
	}
	if done := bothDirect(a, b2, t0.Add(bound)); done.IsZero() {
		t.Errorf("H-6 direct after the new circuit: links a->b %v, b->a %v %v after it, want direct within %v; a: %s",
			linkTo(a, b2.Local()), linkTo(b2, a.Local()), time.Since(t0), bound, a.UpgradeDebug(b2.Local()))
	}
	if !closedBy(old, t0.Add(tVie+time.Second)) {
		t.Errorf("P-15 dead connection kept: the circuit of b's old process is open %v after the new circuit, want closed within T_vie + 1 s; a: %s",
			time.Since(t0), a.UpgradeDebug(b2.Local()))
	}
	f := <-frames
	if f.ab.IsZero() || f.ba.IsZero() {
		t.Errorf("P-15 frames held: a->b delivered at %v, b->a at %v after the new circuit (zero: not by T_vie + 2 s); a: %s",
			sinceOrZero(f.ab, t0), sinceOrZero(f.ba, t0), a.UpgradeDebug(b2.Local()))
	}
	if da, db := ea.count(transport.PeerDown, b2.Local(), crashed), eb.count(transport.PeerDown, a.Local(), crashed); da+db > 0 {
		t.Errorf("P-15 frames held: PeerDown after the crash, a for b %d (at %v), b for a %d (at %v), want none",
			da, sinceOrZero(ea.firstAfter(transport.PeerDown, b2.Local(), crashed), t0), db, sinceOrZero(eb.firstAfter(transport.PeerDown, a.Local(), crashed), t0))
	}
	if k := rounds.onAny(old, crashed); k > 0 {
		t.Errorf("H-20 round on a dead circuit: %d of a's DCUtR rounds opened on the circuit b's old process left; a: %s",
			k, a.UpgradeDebug(b2.Local()))
	}
}

// sinceOrZero is at - t0, or zero when at is.
func sinceOrZero(at, t0 time.Time) time.Duration {
	if at.IsZero() {
		return 0
	}
	return at.Sub(t0)
}

// P-15: a superseded connection that lives is kept, and a silent one is the fallback until a newer connection
// is acknowledged.
//
//   - answered: b's old process stays up while a second process of its identity brings a second circuit to a.
//     The first circuit answers its proof within T_vie and stays open; a's session is neither reopened nor
//     moved, and a announces nothing.
//   - fallback: b crashes and its new process brings a new circuit, then a direct connection; every ACK of
//     the new process's connections is held 10 s. The dead circuit is silent from T_vie on, but it survives
//     until an ACK, with no PeerDown; it is closed once one is written.
func TestSupersedeKeepsLiveOlder(t *testing.T) {
	t.Run("answered", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) { answeredScene(t, newNatNet(1)) })
	})
	t.Run("fallback", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) { fallbackScene(t, newNatNet(1)) })
	})
}

func answeredScene(t *testing.T, nn *natNet) {
	relays := nn.relay(t, p2p.RelayOptions{})
	o := p2p.Options{Relays: relays}
	oa := o
	var proofs proofLog
	p2p.SetOnProof(&oa, proofs.add)
	nn.blockDirect(true) // circuits only: the second circuit is the only newer connection
	a := nn.node(t, 1, oa)
	b := nn.node(t, 2, o)
	b.Want(map[transport.PeerID][]string{a.Local(): nil})
	expectUp(t, a, b)
	waitCircuits(t, a, b.Local(), 1, 0)
	first := openConns(a, b.Local())[0]
	session := a.OutStream(b.Local())
	ea := watchEvents(t, a)
	l := watchConns(t, a, b)
	since := time.Now()
	b2 := nn.crashRestart(t, b, 2, 3, false, o, a) // b's old process stays up: a second circuit while the first lives
	_ = watchEvents(t, b2)
	t0 := waitNewCircuit(t, l, since)
	time.Sleep(time.Until(t0.Add(tVie + time.Second)))
	if first.IsClosed() {
		t.Errorf("P-15 live connection closed: the first circuit, alive (b's old process answers), is closed %v after the second one arrived; a: %s",
			time.Since(t0), a.UpgradeDebug(b.Local()))
	}
	if now := a.OutStream(b.Local()); now != session {
		t.Errorf("P-15 live connection closed: a's session was reopened or moved (on %v, was on %v)", connOf(now), connOf(session))
	}
	for _, k := range []transport.Kind{transport.PeerDown, transport.PeerUp} {
		if got := ea.count(k, b.Local(), since); got > 0 {
			t.Errorf("P-15 live connection closed: a announced %d %v for b after the second circuit, want none", got, k)
		}
	}
	if proofs.answered(first) == 0 {
		t.Errorf("P-15 live connection not proved: no answered proof of the first circuit within T_vie + 1 s of the second one")
	}
}

func connOf(s network.Stream) string {
	if s == nil {
		return "no stream"
	}
	return s.Conn().RemoteMultiaddr().String()
}

func fallbackScene(t *testing.T, nn *natNet) {
	relays := nn.relay(t, p2p.RelayOptions{})
	o := p2p.Options{Relays: relays}
	oa, ob2 := o, o
	release := make(chan struct{})
	var holding atomic.Bool
	hold := func(lane string, _ network.Stream) {
		if lane != "sync" || !holding.Load() {
			return
		}
		select {
		case <-release:
		case <-t.Context().Done():
		}
	}
	p2p.SetAckHold(&oa, hold)
	p2p.SetAckHold(&ob2, hold)
	nn.blockDirect(true)
	a := nn.node(t, 1, oa)
	b := nn.node(t, 2, o)
	b.Want(map[transport.PeerID][]string{a.Local(): nil})
	expectUp(t, a, b)
	waitCircuits(t, a, b.Local(), 1, 0)
	waitPunchIdle(t, a, b.Local())
	ghost := openConns(a, b.Local())
	ea := watchEvents(t, a)
	l := watchConns(t, a, b)
	crashed := time.Now()
	holding.Store(true) // from now on, the ACKs of a and of b's new process wait for release
	nn.blockDirect(false)
	b2 := nn.crashRestart(t, b, 2, 3, true, ob2, a)
	_ = watchEvents(t, b2)
	t0 := waitNewCircuit(t, l, crashed)
	ack := t0.Add(10 * time.Second)
	time.Sleep(time.Until(ack))
	if !a.HasDirect(b2.Local()) {
		t.Errorf("fixture: no direct connection from a to b's new process within 10 s of its circuit; a: %s", a.UpgradeDebug(b2.Local()))
	}
	if closedBy(ghost, time.Now()) {
		t.Errorf("P-15 fallback closed: the dead circuit was closed before any newer connection's ACK (held %v); a: %s",
			ack.Sub(t0), a.UpgradeDebug(b2.Local()))
	}
	close(release)
	if !closedBy(ghost, time.Now().Add(time.Second)) {
		t.Errorf("P-15 dead connection kept: the dead circuit is open 1 s after the newer connections' ACKs; a: %s", a.UpgradeDebug(b2.Local()))
	}
	if down := ea.count(transport.PeerDown, b2.Local(), crashed); down > 0 {
		t.Errorf("P-15 fallback closed: a announced %d PeerDown for b while the newer connection awaited its ACK", down)
	}
}
