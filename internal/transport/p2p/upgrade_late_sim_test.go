package p2p_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// Block HP1r (rule H-19, plan docs/plans/2026-11-16-hp1-direct-upgrade.md §4, spec §8.6 « Connexion tardive »): a
// node keeps an outbound direct connection only if, once the swarm registered it, the node holds a circuit to the
// peer, wants it, or is connecting to it on request. The closed arms fail with « H-19 late link kept », the kept
// arms with « H-19 kept link closed » (mutant MR1 of scripts/netsim/mutants.toml expects the first).

// lateDelay is the one-way latency added to the direct path between the two nodes: an attempt's direct dial
// takes a few of them, so it is still in flight when the circuit it started on goes.
const lateDelay = 150 * time.Millisecond

// offsetDelay is the direct path's added latency of closed_drop_offsets: short, so that the landing falls among
// the offsets of the Drop and races the circuit's removal.
const offsetDelay = 5 * time.Millisecond

// renewDelay is that of kept_renewed_circuit: long enough for the old attempt to land after b's new call
// through the relay has returned.
const renewDelay = 600 * time.Millisecond

// timedEvent is an event of a node's transport, with when it was received.
type timedEvent struct {
	transport.Event
	at time.Time
}

// eventLog records every event a node delivers, from a goroutine that reads them until the test ends.
type eventLog struct {
	mu  sync.Mutex
	evs []timedEvent
}

func watchEvents(t *testing.T, n *simNode) *eventLog {
	t.Helper()
	l := &eventLog{}
	go func() {
		for {
			ev, err := n.Recv(t.Context())
			if err != nil {
				return
			}
			l.mu.Lock()
			l.evs = append(l.evs, timedEvent{Event: ev, at: time.Now()})
			l.mu.Unlock()
		}
	}()
	return l
}

// count returns how many events of kind from peer were received after since.
func (l *eventLog) count(kind transport.Kind, peer transport.PeerID, since time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := 0
	for _, e := range l.evs {
		if e.Kind == kind && e.Peer == peer && e.at.After(since) {
			k++
		}
	}
	return k
}

// waitFor waits until an event of kind from peer was received after since; it fails after d.
func (l *eventLog) waitFor(t *testing.T, kind transport.Kind, peer transport.PeerID, since time.Time, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for l.count(kind, peer, since) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no %v from %s within %v", kind, peer, d)
		}
		time.Sleep(time.Millisecond)
	}
}

// connRec is a connection a node's swarm registered to the peer watched.
type connRec struct {
	c        network.Conn
	at       time.Time
	closedAt time.Time
}

// connLog records the connections between a node and one peer, as the swarm registers and closes them.
type connLog struct {
	mu    sync.Mutex
	conns []*connRec
}

func watchConns(t *testing.T, n, to *simNode) *connLog {
	t.Helper()
	p, _ := p2p.LibP2PID(to.Local())
	l := &connLog{}
	nb := &network.NotifyBundle{
		ConnectedF: func(_ network.Network, c network.Conn) {
			if c.RemotePeer() != p {
				return
			}
			l.mu.Lock()
			l.conns = append(l.conns, &connRec{c: c, at: time.Now()})
			l.mu.Unlock()
		},
		DisconnectedF: func(_ network.Network, c network.Conn) {
			l.mu.Lock()
			defer l.mu.Unlock()
			for _, r := range l.conns {
				if r.c == c && r.closedAt.IsZero() {
					r.closedAt = time.Now()
				}
			}
		},
	}
	n.Host().Network().Notify(nb)
	t.Cleanup(func() { n.Host().Network().StopNotify(nb) })
	return l
}

// direct returns the direct connections registered after since, outbound only if out.
func (l *connLog) direct(since time.Time, out bool) []*connRec {
	l.mu.Lock()
	defer l.mu.Unlock()
	var rs []*connRec
	for _, r := range l.conns {
		if isCircuitAddr(r.c.RemoteMultiaddr()) || r.at.Before(since) {
			continue
		}
		if out && r.c.Stat().Direction != network.DirOutbound {
			continue
		}
		rs = append(rs, &connRec{c: r.c, at: r.at, closedAt: r.closedAt})
	}
	return rs
}

// conns describes the connections from n to peer.
func conns(n *simNode, to transport.PeerID) []string {
	p, _ := p2p.LibP2PID(to)
	var out []string
	for _, c := range n.Host().Network().ConnsToPeer(p) {
		if !c.IsClosed() {
			out = append(out, fmt.Sprintf("%s %s", c.Stat().Direction, c.RemoteMultiaddr()))
		}
	}
	return out
}

// waitNoCircuit waits until n holds no circuit to peer; it fails after d.
func waitNoCircuit(t *testing.T, n *simNode, to transport.PeerID, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		in, out := n.Circuits(to)
		if in+out == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s still holds %d+%d circuits to %s after %v", n.Local(), in, out, to, d)
		}
		time.Sleep(time.Millisecond)
	}
}

// expectNoLateLink checks, a second after since, that neither node holds a connection to the other and that
// neither announced the other since: the late end of an upgrade attempt was closed.
func expectNoLateLink(t *testing.T, a, b *simNode, ea, eb *eventLog, since time.Time, what string) {
	t.Helper()
	time.Sleep(time.Until(since.Add(time.Second)))
	if ca, cb := conns(a, b.Local()), conns(b, a.Local()); len(ca)+len(cb) > 0 {
		t.Errorf("H-19 late link kept (%s): a->b %v, b->a %v one second after; b: %s", what, ca, cb, b.UpgradeDebug(a.Local()))
	}
	if ua, ub := ea.count(transport.PeerUp, b.Local(), since), eb.count(transport.PeerUp, a.Local(), since); ua+ub > 0 {
		t.Errorf("H-19 late link kept (%s): PeerUp at a %d, at b %d within one second", what, ua, ub)
	}
}

// expectKept checks that the outbound direct connections n registered to peer after since are still open,
// that there is one, and that n's session to peer is on a direct connection.
func expectKept(t *testing.T, n *simNode, to transport.PeerID, l *connLog, since time.Time, what string) {
	t.Helper()
	rs := l.direct(since, true)
	if len(rs) == 0 {
		t.Fatalf("H-19 kept link closed (%s): no outbound direct connection registered; %s", what, n.UpgradeDebug(to))
	}
	for _, r := range rs {
		if r.c.IsClosed() {
			t.Errorf("H-19 kept link closed (%s): %s registered at %v, closed %v later", what, r.c.RemoteMultiaddr(), r.at.Format("15:04:05.000"), r.closedAt.Sub(r.at))
		}
	}
	if direct, ok := n.OutDirect(to); !ok || !direct {
		t.Errorf("H-19 kept link closed (%s): the session is not on a direct connection (ok %v, direct %v); %s", what, ok, direct, n.UpgradeDebug(to))
	}
}

// waitRelayAddr waits until n has an address through the relay; it fails after wait.
func waitRelayAddr(t *testing.T, n *simNode) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for len(relayAddrs(n)) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%s has no address through the relay after %v: %v", n.Local(), wait, n.Addrs())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// relayAddrs returns n's addresses through the relay.
func relayAddrs(n *simNode) []string {
	var out []string
	for _, a := range n.Addrs() {
		if isCircuitAddr(ma.StringCast(a)) {
			out = append(out, a)
		}
	}
	return out
}

// dropScene is the scene of the closed arms: a wants b, b holds the inbound circuit and punches (libp2p's
// one-shot and its upgrader); a drops b, revoked, and admits it again (see drop).
type dropScene struct {
	nn      *natNet
	a, b    *simNode
	ea, eb  *eventLog
	revoked atomic.Bool
}

func newDropScene(t *testing.T, seed uint64, delay time.Duration, ob p2p.Options) *dropScene {
	t.Helper()
	sc := &dropScene{nn: newNatNet(seed)}
	sc.nn.setDirectDelay(delay)
	relays := sc.nn.relay(t, p2p.RelayOptions{})
	bID := idOf(key(2))
	sc.a = sc.nn.node(t, 1, p2p.Options{Relays: relays, Admit: func(id transport.PeerID) bool { return !sc.revoked.Load() || id != bID }})
	ob.Relays = relays
	sc.b = sc.nn.node(t, 2, ob)
	sc.ea, sc.eb = watchEvents(t, sc.a), watchEvents(t, sc.b)
	return sc
}

// drop revokes b at a, drops it, waits until b holds no connection to a, admits b again and returns when.
// The wait is the rule's stated limit (plan row H-19, « Residual »): a connection b registers between a's Drop
// and its own removal of the circuit passes b's check, and only a's Admit refuses it; a re-admits b once that
// is settled, and a landing after it is H-19's to close.
func (sc *dropScene) drop(t *testing.T) time.Time {
	t.Helper()
	sc.revoked.Store(true)
	sc.a.Drop(sc.b.Local())
	waitNoCircuit(t, sc.b, sc.a.Local(), wait)
	deadline := time.Now().Add(wait)
	for len(conns(sc.b, sc.a.Local())) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("H-19 late link kept (b revoked): b->a %v after %v", conns(sc.b, sc.a.Local()), wait)
		}
		time.Sleep(time.Millisecond)
	}
	sc.revoked.Store(false)
	return time.Now()
}

// TestUpgradeLateLinkClosed: H-19 (plan row, spec §8.6). The direct path between the two nodes is slow
// (lateDelay), so that an upgrade attempt's direct dial is in flight when the circuit it started on goes.
func TestUpgradeLateLinkClosed(t *testing.T) {
	t.Run("closed_dropped_side", func(t *testing.T) {
		// The late connection is the dropped node's (b's one-shot): b no longer holds a circuit, does not want a.
		synctest.Test(t, func(t *testing.T) {
			sc := newDropScene(t, 1, lateDelay, p2p.Options{})
			start := time.Now()
			sc.a.Want(map[transport.PeerID][]string{sc.b.Local(): nil})
			sc.ea.waitFor(t, transport.PeerUp, sc.b.Local(), start, wait)
			sc.eb.waitFor(t, transport.PeerUp, sc.a.Local(), start, wait)
			expectNoLateLink(t, sc.a, sc.b, sc.ea, sc.eb, sc.drop(t), "dropped side")
		})
	})

	t.Run("closed_dropping_side", func(t *testing.T) {
		// The late connection is the dropping node's: b connects to a through the relay and wants nothing, so a
		// holds the inbound circuit and punches; a drops b (for a fault: b stays admitted) and closes it.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(2)
			nn.setDirectDelay(lateDelay)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.node(t, 1, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			ea, eb := watchEvents(t, a), watchEvents(t, b)
			waitRelayAddr(t, a)
			start := time.Now()
			if err := b.Connect(t.Context(), a.Local(), relayAddrs(a)); err != nil {
				t.Fatalf("Connect through the relay: %v", err)
			}
			ea.waitFor(t, transport.PeerUp, b.Local(), start, wait)
			eb.waitFor(t, transport.PeerUp, a.Local(), start, wait)
			if in, _ := a.Circuits(b.Local()); in == 0 {
				t.Fatalf("a holds no inbound circuit from b: %v", conns(a, b.Local()))
			}
			a.Drop(b.Local())
			waitNoCircuit(t, a, b.Local(), wait)
			waitNoCircuit(t, b, a.Local(), wait)
			expectNoLateLink(t, a, b, ea, eb, time.Now(), "dropping side")
		})
	})

	t.Run("closed_drop_offsets", func(t *testing.T) {
		// The Drop at offsets of b's punch: the landing races the circuit's removal.
		for _, off := range []time.Duration{0, 5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond} {
			synctest.Test(t, func(t *testing.T) {
				punched := make(chan struct{})
				var once sync.Once
				var ob p2p.Options
				p2p.SetOnUpgradeCall(&ob, func(_ transport.PeerID, action string) {
					if action == "punch" {
						once.Do(func() { close(punched) })
					}
				})
				sc := newDropScene(t, 3, offsetDelay, ob)
				bc := watchConns(t, sc.b, sc.a)
				sc.a.Want(map[transport.PeerID][]string{sc.b.Local(): nil})
				select {
				case <-punched:
				case <-time.After(wait):
					t.Fatal("b never punched")
				}
				at := time.Now()
				time.Sleep(off)
				dropAt := time.Now()
				back := sc.drop(t)
				what := fmt.Sprintf("Drop %v after the punch", off)
				expectNoLateLink(t, sc.a, sc.b, sc.ea, sc.eb, back, what)
				for _, r := range bc.direct(at, false) {
					life := "never closed"
					if !r.closedAt.IsZero() {
						life = fmt.Sprintf("closed %v later", r.closedAt.Sub(r.at))
					}
					t.Logf("%s: b's direct connection %s registered %v after the punch (Drop at %v), %s", what, r.c.Stat().Direction, r.at.Sub(at), dropAt.Sub(at), life)
				}
			})
		}
	})

	t.Run("closed_after_failed_connect", func(t *testing.T) {
		// A Connect that fails, and one cancelled, leave nothing behind: the late landing is still closed.
		synctest.Test(t, func(t *testing.T) {
			sc := newDropScene(t, 4, lateDelay, p2p.Options{})
			waitRelayAddr(t, sc.a)
			// First the one that fails (b's peerstore knows no other address of a yet), then the cancelled one.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := sc.b.Connect(ctx, sc.a.Local(), []string{"/ip4/9.9.9.9/udp/8000/quic-v1"}); err == nil {
				t.Fatal("a Connect to an address nobody listens on succeeded")
			}
			ctx, cancel = context.WithCancel(t.Context())
			cancel()
			if err := sc.b.Connect(ctx, sc.a.Local(), relayAddrs(sc.a)); err == nil {
				t.Fatal("a cancelled Connect succeeded")
			}
			start := time.Now()
			sc.a.Want(map[transport.PeerID][]string{sc.b.Local(): nil})
			sc.ea.waitFor(t, transport.PeerUp, sc.b.Local(), start, wait)
			sc.eb.waitFor(t, transport.PeerUp, sc.a.Local(), start, wait)
			expectNoLateLink(t, sc.a, sc.b, sc.ea, sc.eb, sc.drop(t), "after a failed and a cancelled Connect")
		})
	})

	t.Run("closed_answerer_dial", func(t *testing.T) {
		// HP1r2: the late connection is the answerer's. a wants b and dials its circuit, so b holds the inbound one
		// and initiates; a answers. a's upgrader is off, so a never dials b directly (H-3) and b's own first direct
		// dial finds a's NAT closed: the link needs a round. When a has read b's SYNC and starts its punch dial (on the
		// punch service's context: no circuit's close ends it, unlike b's call, H-20), a drops b. The dial lands
		// after the Drop, delayed by the direct path: H-19 closes it, and neither node announces the other.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(5)
			nn.setDirectDelay(lateDelay)
			relays := nn.relay(t, p2p.RelayOptions{})
			bID := idOf(key(2))
			var revoked atomic.Bool
			dialing := make(chan struct{})
			var once sync.Once
			oa := p2p.Options{Relays: relays, Admit: func(id transport.PeerID) bool { return !revoked.Load() || id != bID }}
			p2p.SetLocalReady(&oa, func() bool { return false })
			p2p.SetOnAnswerDial(&oa, func(transport.PeerID) { once.Do(func() { close(dialing) }) })
			sc := &dropScene{nn: nn}
			sc.a = nn.node(t, 1, oa)
			sc.b = nn.node(t, 2, p2p.Options{Relays: relays})
			sc.ea, sc.eb = watchEvents(t, sc.a), watchEvents(t, sc.b)
			start := time.Now()
			sc.a.Want(map[transport.PeerID][]string{sc.b.Local(): nil})
			sc.ea.waitFor(t, transport.PeerUp, sc.b.Local(), start, wait)
			sc.eb.waitFor(t, transport.PeerUp, sc.a.Local(), start, wait)
			select {
			case <-dialing:
			case <-time.After(time.Minute):
				t.Fatal("fixture: a never answered a round of b's")
			}
			time.Sleep(10 * time.Millisecond) // the answerer's dial is in flight: its packets take lateDelay each way
			revoked.Store(true)
			back := func() time.Time {
				sc.a.Drop(sc.b.Local())
				waitNoCircuit(t, sc.b, sc.a.Local(), wait)
				revoked.Store(false)
				return time.Now()
			}()
			expectNoLateLink(t, sc.a, sc.b, sc.ea, sc.eb, back, "answerer's dial")
		})
	})

	t.Run("kept_wanted", func(t *testing.T) {
		// The same late landing at b, but b wants a: it is kept, and once a admits b again it carries the session.
		synctest.Test(t, func(t *testing.T) {
			sc := newDropScene(t, 5, lateDelay, p2p.Options{})
			bc := watchConns(t, sc.b, sc.a)
			start := time.Now()
			sc.a.Want(map[transport.PeerID][]string{sc.b.Local(): nil})
			sc.ea.waitFor(t, transport.PeerUp, sc.b.Local(), start, wait)
			sc.eb.waitFor(t, transport.PeerUp, sc.a.Local(), start, wait)
			sc.b.Want(map[transport.PeerID][]string{sc.a.Local(): nil})
			back := sc.drop(t)
			sc.eb.waitFor(t, transport.PeerUp, sc.a.Local(), back, wait)
			time.Sleep(5 * time.Second)
			expectKept(t, sc.b, sc.a.Local(), bc, back, "wanted")
		})
	})

	t.Run("kept_live_circuit", func(t *testing.T) {
		// The normal upgrade: the direct connection lands while the circuit is held.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(6)
			nn.setDirectDelay(lateDelay)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.node(t, 1, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			bc := watchConns(t, b, a)
			start := time.Now()
			a.Want(map[transport.PeerID][]string{b.Local(): nil})
			waitDirect(t, a, b, time.Minute, "H-19 kept link closed (live circuit)")
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, start, "live circuit")
		})
	})

	t.Run("kept_renewed_circuit", func(t *testing.T) {
		// b's attempt started on a circuit that closes; b connects to a through the relay again before the
		// attempt lands (H-6): the landing is the new circuit's upgrade and is kept.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(7)
			nn.setDirectDelay(renewDelay)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.node(t, 1, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			eb := watchEvents(t, b)
			bc := watchConns(t, b, a)
			waitRelayAddr(t, a)
			start := time.Now()
			a.Want(map[transport.PeerID][]string{b.Local(): nil})
			eb.waitFor(t, transport.PeerUp, a.Local(), start, wait)
			old := closeCircuits(b, a.Local())
			waitNoCircuit(t, b, a.Local(), wait)
			if err := b.Connect(t.Context(), a.Local(), relayAddrs(a)); err != nil {
				t.Fatalf("Connect through the relay again: %v", err)
			}
			renewed := time.Now() // the call is over: the new circuit is the only reason left
			if rs := bc.direct(start, false); len(rs) > 0 {
				t.Fatalf("b's direct connection landed %v after the old circuit closed, before the new circuit's call ended (%v): the scene is not the one tested", rs[0].at.Sub(old), renewed.Sub(old))
			}
			waitDirect(t, a, b, time.Minute, "H-19 kept link closed (renewed circuit)")
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, old, "renewed circuit")
		})
	})

	t.Run("kept_two_circuits", func(t *testing.T) {
		// Two circuits (both dial at once), then b stops wanting a: one circuit is closed (H-15), the landing
		// is kept for the other.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(8)
			nn.setDirectDelay(lateDelay)
			nn.blockDirect(true)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.node(t, 1, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			// Start both dials with reserved circuit addresses and keep the
			// direct path blocked until the two circuits are observed.
			waitRelayAddr(t, a)
			waitRelayAddr(t, b)
			bc := watchConns(t, b, a)
			start := time.Now()
			a.Want(map[transport.PeerID][]string{b.Local(): relayAddrs(b)})
			b.Want(map[transport.PeerID][]string{a.Local(): relayAddrs(a)})
			deadline := time.Now().Add(wait)
			for in, out := b.Circuits(a.Local()); in+out < 2; in, out = b.Circuits(a.Local()) {
				if time.Now().After(deadline) || len(bc.direct(start, false)) > 0 {
					t.Fatalf("b never held two circuits to a: %v", conns(b, a.Local()))
				}
				time.Sleep(time.Millisecond)
			}
			b.Want(nil)
			nn.blockDirect(false)
			waitDirect(t, a, b, time.Minute, "H-19 kept link closed (two circuits)")
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, start, "two circuits")
		})
	})

	t.Run("kept_invitation_connect", func(t *testing.T) {
		// b connects directly to a, which it does not want (an invitation): the connection lands during the call.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(9)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.nodeAt(t, 1, true, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			bc := watchConns(t, b, a)
			start := time.Now()
			if err := b.Connect(t.Context(), a.Local(), []string{directAddr(a)}); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, start, "invitation Connect")
		})
	})

	t.Run("kept_dial_loop", func(t *testing.T) {
		// b's dial loop dials a, which it wants, at its direct address.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(10)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.nodeAt(t, 1, true, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			eb := watchEvents(t, b)
			bc := watchConns(t, b, a)
			start := time.Now()
			b.Want(map[transport.PeerID][]string{a.Local(): {directAddr(a)}})
			eb.waitFor(t, transport.PeerUp, a.Local(), start, wait)
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, start, "dial loop")
		})
	})

	t.Run("kept_lan_sighting", func(t *testing.T) {
		// b wants a with no address and no relay: a sighting on the local network makes its dial loop dial it.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(11)
			a := nn.nodeOn(t, 1, &natEnd{sent: make(map[string]struct{}), internal: "192.168.1.1", external: "192.168.1.1", public: true}, p2p.Options{})
			b := nn.node(t, 2, p2p.Options{})
			eb := watchEvents(t, b)
			bc := watchConns(t, b, a)
			start := time.Now()
			b.Want(map[transport.PeerID][]string{a.Local(): nil})
			time.Sleep(3 * time.Second) // the first attempt finds no address
			id, _ := p2p.LibP2PID(a.Local())
			b.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast(directAddr(a))}})
			eb.waitFor(t, transport.PeerUp, a.Local(), start, wait)
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, start, "LAN sighting")
		})
	})

	t.Run("kept_upgrade_outbound_circuit_closed", func(t *testing.T) {
		// A one-sided upgrade: a wants b, b does not want a. b's direct connection registers outbound while the
		// circuit is held, the session moves onto it, then the last circuit closes: the link stays.
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(12)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.node(t, 1, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			ea, eb := watchEvents(t, a), watchEvents(t, b)
			bc := watchConns(t, b, a)
			start := time.Now()
			a.Want(map[transport.PeerID][]string{b.Local(): nil})
			waitDirect(t, a, b, time.Minute, "H-19 kept link closed (one-sided upgrade)")
			if len(bc.direct(start, true)) == 0 {
				t.Fatalf("b's direct connection is not outbound: %v", conns(b, a.Local()))
			}
			closed := closeCircuits(b, a.Local())
			waitNoCircuit(t, a, b.Local(), wait)
			time.Sleep(5 * time.Second)
			expectKept(t, b, a.Local(), bc, start, "one-sided upgrade, its last circuit closed")
			if d := ea.count(transport.PeerDown, b.Local(), closed) + eb.count(transport.PeerDown, a.Local(), closed); d > 0 {
				t.Errorf("H-19 kept link closed (one-sided upgrade, its last circuit closed): %d PeerDown after the circuit closed", d)
			}
		})
	})
}

// closeCircuits closes n's circuits to peer and returns when.
func closeCircuits(n *simNode, to transport.PeerID) time.Time {
	p, _ := p2p.LibP2PID(to)
	for _, c := range n.Host().Network().ConnsToPeer(p) {
		if isCircuitAddr(c.RemoteMultiaddr()) {
			_ = c.Close()
		}
	}
	return time.Now()
}
