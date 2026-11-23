package p2p_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// Block HP1 (plan docs/plans/2026-11-16-hp1-direct-upgrade.md, §4): a relayed link to a wanted peer keeps
// trying to become direct. These tests run in sim_test.go's simnet inside testing/synctest (controlled time,
// real libp2p hole punching, the NAT firewall of go-libp2p's own tests), or with a fake puncher where the
// rule is about our scheduling; a restart starts a node's identity again on its network. Each names its
// rule (H-n) at the start of every failure text, so the Go mutants (scripts/netsim/mutants.toml, block HP1)
// are caught by their `expect`.

// waitDirect waits until both a and b see their link as direct, and returns when; it fails after d.
func waitDirect(t *testing.T, a, b *simNode, d time.Duration, rule string) time.Time {
	t.Helper()
	deadline := time.Now().Add(d)
	for linkTo(a, b.Local()) != transport.LinkDirect || linkTo(b, a.Local()) != transport.LinkDirect {
		if time.Now().After(deadline) {
			t.Fatalf("%s: links after %v: a->b %v, b->a %v; a: %s", rule, d, linkTo(a, b.Local()), linkTo(b, a.Local()), a.UpgradeDebug(b.Local()))
		}
		time.Sleep(100 * time.Millisecond)
	}
	return time.Now()
}

// H-1: a RelayOnly node, and a node without relays, make no upgrade call.
func TestUpgradeRelayOnlyMakesNoCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		var calls atomic.Int64
		count := func(context.Context, transport.PeerID) error { calls.Add(1); return errors.New("fake") }
		oa := p2p.Options{Relays: relays, RelayOnly: true}
		p2p.SetPuncher(&oa, count)
		a := nn.node(t, 1, oa)
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		wantEachOther(a, b)
		expectUp(t, a, b)
		time.Sleep(10 * time.Minute)
		if n := calls.Load(); n != 0 {
			t.Fatalf("H-1 a RelayOnly node made %d upgrade calls", n)
		}
		if l := linkTo(a, b.Local()); l != transport.LinkRelayed {
			t.Fatalf("H-1 the RelayOnly link is %v", l)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(2)
		var calls atomic.Int64
		o := p2p.Options{}
		p2p.SetPuncher(&o, func(context.Context, transport.PeerID) error { calls.Add(1); return nil })
		a := nn.nodeAt(t, 1, true, o)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		a.Want(map[transport.PeerID][]string{b.Local(): b.Addrs()})
		expectUp(t, a, b)
		time.Sleep(10 * time.Minute)
		if n := calls.Load(); n != 0 {
			t.Fatalf("H-1 a node without relays made %d upgrade calls", n)
		}
	})
}

// H-2: b restarts and dials within the 250 ms window before its /libp2p/dcutr handler exists: direct within
// 5 s simulated after b's handler is registered (red on main: relayed for ever).
func TestUpgradeAfterRestartBeforeHandler(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		o := p2p.Options{Relays: relays}
		a := nn.node(t, 1, o)
		b := nn.node(t, 2, o)
		wantEachOther(a, b)
		expectUp(t, a, b)
		waitDirect(t, a, b, time.Minute, "H-2 first link")

		nn.blockDirect(true) // the old direct connection dies with b; the new one must be punched
		b = nn.restart(t, b, 2, o, a)
		ready := dcutrReady(t, b.Host())
		expectEventWithin(t, b, transport.PeerUp, a.Local(), time.Minute) // the new process is linked again
		var at time.Time
		select {
		case at = <-ready:
		case <-time.After(time.Minute):
			t.Fatal("H-2 b's handler never registered")
		}
		nn.blockDirect(false)
		if done := waitDirect(t, a, b, 5*time.Second+time.Until(at), "H-2 direct after b's handler"); done.Sub(at) > 5*time.Second {
			t.Fatalf("H-2 direct %v after b's handler, want ≤ 5 s", done.Sub(at))
		}
	})
}

// H-2: with a fake readiness of the node's own handler, no call is observed before it.
func TestUpgradeNotBeforeLocalReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(3)
		relays := nn.relay(t, p2p.RelayOptions{})
		var ready atomic.Bool
		var early, late atomic.Int64
		oa := p2p.Options{Relays: relays}
		p2p.SetLocalReady(&oa, ready.Load)
		p2p.SetPuncher(&oa, func(context.Context, transport.PeerID) error {
			if !ready.Load() {
				early.Add(1)
			} else {
				late.Add(1)
			}
			return errors.New("fake")
		})
		nn.blockDirect(true)
		a := nn.node(t, 1, oa)
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		b.Want(map[transport.PeerID][]string{a.Local(): nil}) // b dials: a holds the inbound circuit
		expectUp(t, b, a)
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		time.Sleep(2 * time.Minute)
		if n := early.Load(); n != 0 {
			t.Fatalf("H-2 call before local ready: %d calls", n)
		}
		ready.Store(true)
		em, err := a.Host().EventBus().Emitter(new(event.EvtLocalProtocolsUpdated))
		if err != nil {
			t.Fatal(err)
		}
		_ = em.Emit(event.EvtLocalProtocolsUpdated{Added: []protocol.ID{holepunch.Protocol}})
		_ = em.Close()
		time.Sleep(2 * time.Second)
		if late.Load() == 0 {
			t.Fatalf("H-2 no call once ready")
		}
	})
}

// H-3: two circuits (both ends dialled at once): nobody punches until one is closed (H-15), then direct is
// reached; a call is never made without an inbound circuit.
func TestUpgradeTwoCircuits(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		var la, lb callLog
		var a, b *simNode
		oa, ob := p2p.Options{Relays: relays}, p2p.Options{Relays: relays}
		p2p.SetPuncher(&oa, realPuncher(&la, &a))
		p2p.SetPuncher(&ob, realPuncher(&lb, &b))
		a = nn.node(t, 1, oa)
		b = nn.node(t, 2, ob)
		wantEachOther(a, b) // both dial the other's circuit at once
		expectUp(t, a, b)
		waitDirect(t, a, b, time.Minute, "H-3 direct")
		for name, l := range map[string]*callLog{"a": &la, "b": &lb} {
			l.mu.Lock()
			bad := l.noInbound
			l.mu.Unlock()
			if bad > 0 {
				t.Fatalf("H-3 punch without an inbound circuit: %s made %d calls with outbound circuits only", name, bad)
			}
		}
	})
}

// H-1, H-3 (integrator decision 2026-11-16): the upgrader exists for every admitted member the node holds
// a circuit with, not only for the peers it wants. a wants b, b does not want a: b holds the inbound
// circuit, so b is the one to punch. libp2p's one-shot fails while the direct path is blocked; once it
// opens, b's upgrader punches for a and the link becomes direct within the slow ladder's first step
// (60 s · 1.25) and a call. Every call of b is made with an inbound circuit.
func TestUpgradeMemberNotWanted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(22)
		relays := nn.relay(t, p2p.RelayOptions{})
		var l callLog
		var b *simNode
		ob := p2p.Options{Relays: relays}
		p2p.SetPuncher(&ob, realPuncher(&l, &b))
		nn.blockDirect(true)
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b = nn.node(t, 2, ob)
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		expectUp(t, a, b)
		time.Sleep(30 * time.Second) // libp2p's one-shot and b's first call fail on the blocked path
		nn.blockDirect(false)
		waitDirect(t, a, b, time.Minute*5/4+30*time.Second, "H-1 direct for a member b does not want")
		l.mu.Lock()
		defer l.mu.Unlock()
		if len(l.starts) == 0 || l.noInbound > 0 {
			t.Fatalf("H-1 b's calls for a member it does not want: %d, %d without an inbound circuit", len(l.starts), l.noInbound)
		}
	})
}

// H-4: a fake puncher counts concurrent calls per peer under -race: never more than one, a cancelled one
// included until it returns (the fake returns 3 s after its context ends).
func TestUpgradeOneCallInFlight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(4)
		relays := nn.relay(t, p2p.RelayOptions{})
		var l callLog
		var a *simNode
		oa := p2p.Options{Relays: relays}
		// The upgrader alone: libp2p's one-shot is off on both nodes, so no other DCUtR call to the peer runs, whose
		// rounds would be booked to the fake call (a Spent call, the slow ladder; with H-20 each new circuit's one-shot
		// runs).
		p2p.SetNoAutoPunch(&oa)
		p2p.SetPuncher(&oa, func(ctx context.Context, _ transport.PeerID) error {
			l.begin(true)
			defer l.end(nil)
			select {
			case <-ctx.Done():
				time.Sleep(3 * time.Second) // a slow unwinding
				return ctx.Err()
			case <-time.After(20 * time.Second):
				return errors.New("fake: no punch")
			}
		})
		// Every call of the upgrader, punch or direct dial: after a hangup the new circuit may be a's own (a wants b
		// too), and with an outbound circuit only the call is a direct dial (H-3), which the fake puncher never sees.
		var calls atomic.Int64
		p2p.SetOnUpgradeCall(&oa, func(transport.PeerID, string) { calls.Add(1) })
		nn.blockDirect(true)
		a = nn.node(t, 1, oa)
		ob := p2p.Options{Relays: relays}
		p2p.SetNoAutoPunch(&ob)
		b := nn.node(t, 2, ob)
		b.Want(map[transport.PeerID][]string{a.Local(): nil})
		expectUp(t, b, a)
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		for range 6 { // new circuits: each a new epoch that cancels the call in flight
			time.Sleep(7 * time.Second)
			b.Hangup(a.Local())
			time.Sleep(3 * time.Second)
		}
		time.Sleep(time.Minute)
		l.mu.Lock()
		got, n := l.maxFlight, int(calls.Load())
		l.mu.Unlock()
		if got > 1 {
			t.Fatalf("H-4 calls in flight: %d at once to one peer", got)
		}
		if n < 3 {
			t.Fatalf("H-4 only %d calls: the upgrader stopped", n)
		}
		_ = a.Close()
		time.Sleep(5 * time.Second) // the last fake call unwinds
	})
}

// H-5: 10 relayed wanted peers and a blocking fake puncher: at most 4 slots held; a peer asking in a loop
// never delays a waiting one by more than one lease; a holder that never finishes loses its slot at 75 s.
func TestUpgradeSlotsFIFO(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(5)
		relays := nn.relay(t, p2p.RelayOptions{})
		var mu sync.Mutex
		inFlight, maxFlight := 0, 0
		first := map[transport.PeerID]time.Time{}
		oa := p2p.Options{Relays: relays}
		// Ten relayed links on one node: libp2p's limit of 8 connections per IPv4 address counts every circuit
		// under the relay's address. Lifted here for the test only (block RF1 fixes the product; this proves no
		// product capacity).
		p2p.LiftRelayIPLimit(&oa, relayIP, 64)
		p2p.SetPuncher(&oa, func(ctx context.Context, to transport.PeerID) error {
			mu.Lock()
			inFlight++
			maxFlight = max(maxFlight, inFlight)
			if _, ok := first[to]; !ok {
				first[to] = time.Now()
			}
			mu.Unlock()
			defer func() { mu.Lock(); inFlight--; mu.Unlock() }()
			<-ctx.Done() // never finishes on its own
			return ctx.Err()
		})
		nn.blockDirect(true)
		a := nn.node(t, 1, oa)
		want := map[transport.PeerID][]string{}
		var peers []*simNode
		for i := range 10 {
			p := nn.node(t, byte(10+i), p2p.Options{Relays: relays})
			p.Want(map[transport.PeerID][]string{a.Local(): nil}) // each dials: a holds inbound circuits
			want[p.Local()] = nil
			peers = append(peers, p)
		}
		// Every peer up over the circuit it dialled before a wants them: a then holds inbound circuits only.
		for up := map[transport.PeerID]bool{}; len(up) < len(peers); {
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			ev, err := a.Recv(ctx)
			cancel()
			if err != nil {
				var missing []string
				for _, p := range peers {
					if up[p.Local()] {
						in, out := a.Circuits(p.Local())
						pin, pout := p.Circuits(a.Local())
						t.Logf("up %s: at a in %d out %d, at the peer in %d out %d", p.Local(), in, out, pin, pout)
					}
					if !up[p.Local()] {
						in, out := a.Circuits(p.Local())
						missing = append(missing, fmt.Sprintf("%s (at a: circuits in %d out %d; %s; at the peer: %s)", p.Local(), in, out, a.UpgradeDebug(p.Local()), p.UpgradeDebug(a.Local())))
					}
				}
				t.Fatalf("H-5 setup: %d of %d peers up after a minute without an event; missing: %v", len(up), len(peers), missing)
			}
			if ev.Kind == transport.PeerUp {
				up[ev.Peer] = true
			}
		}
		start := time.Now()
		a.Want(want)
		maxHeld := 0
		for time.Since(start) < 10*time.Minute {
			held, _ := a.SlotsHeld()
			maxHeld = max(maxHeld, held)
			time.Sleep(time.Second)
		}
		mu.Lock()
		defer mu.Unlock()
		if maxHeld > 4 || maxFlight > 4 {
			t.Fatalf("H-5 slots held: %d at most (calls in flight %d), want ≤ 4", maxHeld, maxFlight)
		}
		// Ten peers, four slots, leases of 75 s, the node's 8 + 1 per 120 s admissions: the last peer's first
		// call comes within 8 grants, then 2 more refills: ≤ 75 s · 2 + 120 s · 2 + slack.
		for _, p := range peers {
			at, ok := first[p.Local()]
			if !ok {
				t.Fatalf("H-5 slots held: %s never got a slot in 10 min: %s", p.Local(), a.UpgradeDebug(p.Local()))
			}
			if d := at.Sub(start); d > 75*time.Second*2+240*time.Second+30*time.Second {
				t.Fatalf("H-5 slots held: %s waited %v for its first slot", p.Local(), d)
			}
		}
	})
}

// H-6: a call in flight towards the old process of a peer that restarts: direct within 3 s after the new
// circuit (not 5 s: the old call is cancelled, D1), and the old call's result changes nothing.
func TestUpgradeAfterRestartDuringAttempt(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		o := p2p.Options{Relays: relays}
		oa := o
		started, finishOld := make(chan struct{}), make(chan struct{}, 1)
		defer close(finishOld)
		var a *simNode
		var calls atomic.Int64
		p2p.SetPuncher(&oa, func(ctx context.Context, to transport.PeerID) error {
			if calls.Add(1) == 1 {
				close(started)
				<-ctx.Done()
				<-finishOld
				// A stale service-closed result would stop the replacement epoch's scheduler.
				return holepunch.ErrClosed
			}
			// Keep the one-shot from making the link direct before the scheduler acts.
			nn.blockDirect(false)
			return a.RealPunch(ctx, to)
		})
		nn.blockDirect(true)
		a = nn.node(t, 1, oa)
		b := nn.node(t, 2, o)
		b.Want(map[transport.PeerID][]string{a.Local(): nil}) // b alone dials a circuit
		expectUp(t, b, a)
		select {
		case <-started:
		case <-time.After(time.Minute):
			t.Fatal("H-6 no call started on the original circuit")
		}
		time.Sleep(1500 * time.Millisecond)

		connections := watchConns(t, a, b)
		b = nn.restart(t, b, 2, o, a)
		deadline := time.Now().Add(time.Minute)
		expectEventWithin(t, b, transport.PeerUp, a.Local(), time.Until(deadline))
		for {
			in, _ := a.Circuits(b.Local())
			direct, open := a.OutDirect(b.Local())
			if in > 0 && open && !direct {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("H-6 b's new process never came back over the relay")
			}
			time.Sleep(10 * time.Millisecond)
		}
		synctest.Wait() // apply the replacement circuit event before releasing the old result
		connections.mu.Lock()
		if len(connections.conns) == 0 {
			connections.mu.Unlock()
			t.Fatalf("H-6 b's new process never came back over the relay")
		}
		circuit := connections.conns[0].at
		connections.mu.Unlock()
		finishOld <- struct{}{}
		if done := waitDirect(t, a, b, time.Minute, "H-6 direct after the new circuit"); done.Sub(circuit) > 3*time.Second {
			t.Fatalf("H-6 direct after the new circuit: %v, want ≤ 3 s", done.Sub(circuit))
		}
	})
}

// H-7: a hopeless pair (direct paths blocked) on controlled time: Spent outcomes climb the slow ladder; the
// calls of the first hour are within the slow ladder's count (t = 0, 1, 3, 7, 15, 31, 47 min ± 25 %).
func TestUpgradeHopelessIsCalm(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(7)
		relays := nn.relay(t, p2p.RelayOptions{})
		var l callLog
		var a *simNode
		oa := p2p.Options{Relays: relays}
		p2p.SetPuncher(&oa, realPuncher(&l, &a))
		nn.blockDirect(true)
		a = nn.node(t, 1, oa)
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		b.Want(map[transport.PeerID][]string{a.Local(): nil})
		expectUp(t, b, a)
		a.NoteActivity()
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		for range 12 { // an hour, the app in use
			time.Sleep(5 * time.Minute)
			a.NoteActivity()
		}
		if n := l.count() - l.busy; n > 8 || n < 5 {
			t.Fatalf("H-7 class: %d non-Busy calls in an hour for a hopeless pair, want the slow ladder's 5 to 8", n)
		}
	})
}

// H-10: direct path blocked for 20 simulated min, then open: direct within 16 min · 1.25 + 30 s after.
func TestUpgradeAfterLongBlock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(10)
		relays := nn.relay(t, p2p.RelayOptions{})
		o := p2p.Options{Relays: relays}
		nn.blockDirect(true)
		a := nn.node(t, 1, o)
		b := nn.node(t, 2, o)
		wantEachOther(a, b)
		expectUp(t, a, b)
		for range 4 { // twenty minutes of use, the direct path blocked
			time.Sleep(5 * time.Minute)
			a.NoteActivity()
			b.NoteActivity()
		}
		nn.blockDirect(false)
		lifted := time.Now()
		deadline := lifted.Add(16*time.Minute*5/4 + 30*time.Second)
		for linkTo(a, b.Local()) != transport.LinkDirect || linkTo(b, a.Local()) != transport.LinkDirect {
			if time.Now().After(deadline) {
				t.Fatalf("H-10 direct after the block lifts: still %v / %v after %v; a: %s", linkTo(a, b.Local()), linkTo(b, a.Local()), time.Since(lifted), a.UpgradeDebug(b.Local()))
			}
			time.Sleep(time.Second)
			if time.Since(lifted)%(4*time.Minute) < time.Second {
				a.NoteActivity()
				b.NoteActivity()
			}
		}
	})
}

// H-11: a member reconnects every 10 s for one simulated hour: our non-Busy calls ≤ 12.
func TestUpgradeByzantineReconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(11)
		relays := nn.relay(t, p2p.RelayOptions{})
		var l callLog
		oa := p2p.Options{Relays: relays}
		p2p.SetOnUpgradeCall(&oa, func(transport.PeerID, string) { l.begin(true); l.end(nil) })
		nn.blockDirect(true)
		a := nn.node(t, 1, oa)
		m := nn.rawMember(t, 20, relays, nil)
		a.Want(map[transport.PeerID][]string{m.id: {m.addr}})
		start := time.Now()
		for time.Since(start) < time.Hour {
			time.Sleep(10 * time.Second)
			a.NoteActivity()
			_ = m.h.Network().ClosePeer(a.Host().ID()) // the member drops; a's dial loop makes a new circuit
		}
		if n := l.maxInWindow(time.Hour) - l.busy; n > 12 {
			t.Fatalf("H-11 calls in an hour: %d non-Busy calls to a member that reconnects every 10 s, want ≤ 12", n)
		}
		if l.count() < 4 {
			t.Fatalf("H-11 only %d calls: the upgrader did not act", l.count())
		}
	})
}

// H-12: a raw libp2p member opens /libp2p/dcutr in a loop: rounds that get an address ≤ 9 + t/100 s; a
// stranger: 0, our public addresses are never sent to it, and it never gets a ticket in the slot queue (H-5).
// Both roles: one stranger opens /libp2p/dcutr in the loop (the node answers), another dials a circuit to the
// node, whose one-shot then initiates (its CONNECT, carrying our addresses, would go out before it reads any).
func TestUpgradeGateRefusesSpam(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(12)
		relays := nn.relay(t, p2p.RelayOptions{})
		var strangers sync.Map // transport.PeerID -> struct{}
		admit := func(id transport.PeerID) bool { _, s := strangers.Load(id); return !s }
		nn.blockDirect(true)
		a := nn.node(t, 1, p2p.Options{Relays: relays, Admit: admit})
		m := nn.rawMember(t, 20, relays, nil)
		s := nn.rawMember(t, 21, relays, nil)
		s2 := nn.rawMember(t, 22, relays, nil)
		strangers.Store(s.id, struct{}{})
		strangers.Store(s2.id, struct{}{})
		s2.answer()
		a.Want(map[transport.PeerID][]string{m.id: {m.addr}})
		a.SetOpen(time.Now().Add(2 * time.Hour)) // an invitation window: the strangers are let in, not wanted
		if err := a.Connect(t.Context(), s.id, []string{s.addr}); err != nil {
			t.Fatalf("connect to the stranger: %v", err)
		}
		ai := fmt.Sprintf("%s/p2p-circuit/p2p/%s", relays[0], a.Host().ID())
		deadline := time.Now().Add(time.Minute)
		for err := s2.dial(t.Context(), ai); err != nil; err = s2.dial(t.Context(), ai) {
			if time.Now().After(deadline) {
				t.Fatalf("the stranger's dial: %v", err)
			}
			time.Sleep(time.Second)
		}
		time.Sleep(time.Minute) // the node's one-shot rounds with s2, which dialled it
		deadline = time.Now().Add(time.Minute)
		for m.h.Network().Connectedness(a.Host().ID()) == network.NotConnected {
			if time.Now().After(deadline) {
				t.Fatal("a never dialled the member")
			}
			time.Sleep(100 * time.Millisecond)
		}
		start := time.Now()
		addr := []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", m.end.external, simPort))}
		saddr := []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", s.end.external, simPort))}
		for time.Since(start) < time.Hour {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			_ = m.exchange(ctx, a.Local(), addr, false)
			_ = s.exchange(ctx, a.Local(), saddr, false)
			cancel()
			// Addresses first: a round admitted for a stranger also leaves it a ticket or a slot.
			if n := s.connects.Load() + s2.connects.Load(); n > 0 {
				t.Fatalf("H-12 rounds with a stranger: %d CONNECTs with our addresses", n)
			}
			if a.PunchTicket(s.id) || a.PunchTicket(s2.id) {
				t.Fatalf("H-12 ticket for a stranger: it holds a ticket or a slot")
			}
			if got, limit := m.connects.Load(), int64(9+time.Since(start)/(100*time.Second)); got > limit {
				t.Fatalf("H-12 rounds for a member in a loop: %d after %v, want ≤ %d", got, time.Since(start), limit)
			}
			time.Sleep(time.Second)
		}
		if m.connects.Load() < 9 {
			t.Fatalf("H-12 the member got only %d rounds in an hour: the budget refused too much", m.connects.Load())
		}
	})
}

// H-12, H-16: a member's identify lists 20 public addresses. libp2p's one-shot first direct dial, made before any
// round while the member holds no grant, asks one on demand through the FIFO: with every slot held, the FIFO grants
// none and the dial is refused (no packet to the member), the ticket kept; with a slot free, the grant admits the
// member's ≤ 3 selected announced addresses, each counted in the destination bucket. A stranger with a public
// address: none dialled. a's own upgrader makes no call (a fake puncher): what is dialled is libp2p's.
func TestUpgradeDialsOnlyAdmitted(t *testing.T) {
	// A name is refused before selection: the node never resolves a name a member chose (libp2p/go-libp2p#3547).
	named := []ma.Multiaddr{
		ma.StringCast("/dns4/victim.example/udp/9000/quic-v1"), ma.StringCast("/dns/victim.example/tcp/9000"),
		ma.StringCast("/dns6/victim.example/udp/9000/quic-v1"), ma.StringCast("/ip4/5.1.1.1/udp/9000/quic-v1"),
	}
	if got := p2p.SelectAddrs(named); len(got) != 1 || !got[0].Equal(named[3]) {
		t.Fatalf("H-12 a DNS name selected: %v, want only %v", got, named[3])
	}
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(13)
		relays := nn.relay(t, p2p.RelayOptions{})
		var stranger atomic.Value
		admit := func(id transport.PeerID) bool { s, _ := stranger.Load().(transport.PeerID); return id != s }
		nn.blockDirect(true)
		var mu sync.Mutex
		var a *simNode
		var aEnd atomic.Pointer[natEnd] // a's end, for the tap: packets are routed while a starts
		dialled := map[string]int{}     // non-relay destination -> packets a sent to it
		nn.tap = func(src *natEnd, to string) {
			if src == aEnd.Load() && !strings.HasPrefix(to, relayIP+":") {
				mu.Lock()
				dialled[to]++
				mu.Unlock()
			}
		}
		dialledOf := func(addrs []ma.Multiaddr) (n int) {
			mu.Lock()
			defer mu.Unlock()
			for _, x := range addrs {
				ip, err := x.ValueForProtocol(ma.P_IP4)
				if err != nil {
					ip, _ = x.ValueForProtocol(ma.P_IP6)
					ip = "[" + ip + "]"
				}
				for to := range dialled {
					if strings.HasPrefix(to, ip+":") {
						n++
					}
				}
			}
			return n
		}
		m := nn.rawMember(t, 20, relays, publicAddrs(1, 20))
		m2 := nn.rawMember(t, 22, relays, publicAddrs(3, 20))
		s := nn.rawMember(t, 21, relays, publicAddrs(2, 3))
		stranger.Store(s.id)
		oa := p2p.Options{Relays: relays, Admit: admit}
		p2p.SetPuncher(&oa, func(context.Context, transport.PeerID) error { return errors.New("no call of ours") })
		a = nn.node(t, 1, oa)
		aEnd.Store(a.end)
		a.SetOpen(time.Now().Add(time.Hour))
		raw := func(r *rawMember) {
			ai := fmt.Sprintf("%s/p2p-circuit/p2p/%s", relays[0], a.Host().ID())
			start := time.Now()
			for err := r.dial(t.Context(), ai); err != nil; err = r.dial(t.Context(), ai) {
				if time.Since(start) > time.Minute {
					t.Fatalf("raw dial: %v", err)
				}
				time.Sleep(time.Second) // a has no reservation on the relay yet
			}
		}

		// Every slot held: the member's circuit makes libp2p's one-shot dial it directly; no grant, none given.
		release := a.FillSlots(4)
		raw(m)
		time.Sleep(5 * time.Second)
		if n := dialledOf(publicAddrs(1, 20)); n > 0 {
			t.Fatalf("H-16 first direct dial without a grant: %d of the member's addresses dialled while the FIFO granted none", n)
		}
		if !a.PunchTicket(m.id) {
			t.Fatalf("H-16 the refused dial lost the member's ticket")
		}
		release()

		// A slot free: the one-shot's first direct dial of another member gets its grant on demand.
		raw(m2)
		raw(s)
		time.Sleep(2 * time.Second)
		if n := dialledOf(publicAddrs(3, 20)); n == 0 || n > 3 {
			t.Fatalf("H-16 on-demand grant: %d of the member's announced addresses dialled at its first direct dial, want 1 to 3", n)
		}
		time.Sleep(time.Minute) // the one-shots' rounds
		if n := dialledOf(publicAddrs(2, 3)); n > 0 {
			t.Fatalf("H-12 dialled a stranger: %d of its addresses", n)
		}
		mu.Lock()
		defer mu.Unlock()
		total := 0
		for _, r := range []*rawMember{m, m2} {
			_, dests := a.PunchCounts(r.id)
			if dests > 30+3*2 {
				t.Fatalf("H-12 destinations: %d admitted for a member in about a minute, beyond the bucket", dests)
			}
			total += dests
		}
		if distinct := len(dialled); distinct > total {
			t.Fatalf("H-16 first direct dial without a grant: %d destinations dialled, %d admitted by the gater", distinct, total)
		}
	})
}

// H-13: a direct connection exists but the move of the session onto it failed (its stream open timed out,
// as on a lossy path or a stale connection, go-libp2p #3420): the next call succeeds without moving the
// session; after T_settle the upgrader nudges once and the link becomes direct.
func TestUpgradeStaleDirectSettles(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		// The direct connection is stale: every move onto it fails until 10 s after it exists, so that a second
		// direct connection of a simultaneous punch cannot move the session either, and only the settle check (H-13:
		// 15 s after the call that succeeded) moves it.
		var failUntil atomic.Int64
		failUntil.Store(math.MaxInt64)
		oa := p2p.Options{Relays: relays}
		p2p.SetFailUpgrade(&oa, func() bool { return time.Now().UnixNano() < failUntil.Load() })
		nn.blockDirect(true)
		a := nn.node(t, 1, oa)
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		b.Want(map[transport.PeerID][]string{a.Local(): nil})
		expectUp(t, b, a)
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		time.Sleep(2 * time.Second)
		nn.blockDirect(false)
		deadline := time.Now().Add(time.Minute)
		for !a.HasDirect(b.Local()) {
			if time.Now().After(deadline) {
				t.Fatalf("H-13 no direct connection was made: %s", a.UpgradeDebug(b.Local()))
			}
			time.Sleep(100 * time.Millisecond)
		}
		made := time.Now()
		failUntil.Store(made.Add(10 * time.Second).UnixNano())
		for {
			if direct, ok := a.OutDirect(b.Local()); ok && direct {
				break
			}
			if time.Since(made) > 15*time.Second+30*time.Second+5*time.Second {
				t.Fatalf("H-13 direct after settle: a's session still relayed %v after the direct connection: %s", time.Since(made), a.UpgradeDebug(b.Local()))
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

// H-14: Close with a call in flight returns within 1 s, and nothing is left running once the call unwinds
// (synctest fails a test whose goroutines outlive it).
func TestUpgradeCloseDuringAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(14)
		relays := nn.relay(t, p2p.RelayOptions{})
		started := make(chan struct{}, 1)
		oa := p2p.Options{Relays: relays}
		p2p.SetPuncher(&oa, func(ctx context.Context, _ transport.PeerID) error {
			select {
			case started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			time.Sleep(5 * time.Second) // a call that unwinds slowly
			return ctx.Err()
		})
		nn.blockDirect(true)
		a := nn.node(t, 1, oa)
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		// Keep the member loop in charge of the in-flight call through shutdown.
		b.Want(map[transport.PeerID][]string{a.Local(): nil})
		expectUp(t, b, a)
		select {
		case <-started:
		case <-time.After(time.Minute):
			t.Fatal("H-14 no call started")
		}
		begin := time.Now()
		_ = a.Close()
		if d := time.Since(begin); d > time.Second {
			t.Fatalf("H-14 Close returned after %v with a call in flight, want ≤ 1 s", d)
		}
		time.Sleep(10 * time.Second) // the call unwinds; nothing else may be left
	})
}

// H-15 (adopted, B1): with two circuits to the peer our upgrader opens no DCUtR stream, and the circuit the higher
// PeerID dialled is closed within 5 s of the session leaving it. For information only, both ends initiating at
// offsets 0, 0.5, 1, 2 and 3 s, 20 seeds each, it logs one line per offset,
// "H-15 crossed offset=<d> runs=20 overlap=<n> bothfail=<n>", which the gate copies to gates/HP1/CROSSED.md.
func TestUpgradeCrossedExchanges(t *testing.T) {
	t.Run("oracle", func(t *testing.T) {
		forSeeds(t, func(t *testing.T, nn *natNet) {
			relays := nn.relay(t, p2p.RelayOptions{})
			var la, lb callLog
			var a, b *simNode
			oa, ob := p2p.Options{Relays: relays}, p2p.Options{Relays: relays}
			var two atomic.Int64 // calls made while two circuits were held
			wrap := func(l *callLog, n **simNode) func(context.Context, transport.PeerID) error {
				punch := realPuncher(l, n)
				return func(ctx context.Context, to transport.PeerID) error {
					if in, out := (*n).Circuits(to); in > 0 && out > 0 {
						two.Add(1)
					}
					return punch(ctx, to)
				}
			}
			p2p.SetPuncher(&oa, wrap(&la, &a))
			p2p.SetPuncher(&ob, wrap(&lb, &b))
			nn.blockDirect(true) // circuits only: the duplicate must go, not be outlived by a direct link
			a = nn.node(t, 1, oa)
			b = nn.node(t, 2, ob)
			wantEachOther(a, b)
			expectUp(t, a, b)
			// Within 5 s of the sessions settling, one circuit is left (on both sides).
			deadline := time.Now().Add(20 * time.Second)
			for {
				ai, ao := a.Circuits(b.Local())
				bi, bo := b.Circuits(a.Local())
				if ai+ao == 1 && bi+bo == 1 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("H-15 circuit not closed: a holds %d in / %d out, b %d / %d; a: %s", ai, ao, bi, bo, a.UpgradeDebug(b.Local()))
				}
				time.Sleep(100 * time.Millisecond)
			}
			if n := two.Load(); n > 0 {
				t.Fatalf("H-15 %d calls made while two circuits were held", n)
			}
			nn.blockDirect(false)
			waitDirect(t, a, b, time.Minute, "H-15 direct after the duplicate closed")
		})
	})
	// The measurement, for information: both ends call DCUtR over two circuits at once (bypassing the policy).
	for _, off := range []time.Duration{0, 500 * time.Millisecond, time.Second, 2 * time.Second, 3 * time.Second} {
		var overlap, bothFail atomic.Int64
		for seed := range 20 {
			synctest.Test(t, func(t *testing.T) {
				nn := newNatNet(uint64(1000 + seed))
				relays := nn.relay(t, p2p.RelayOptions{})
				noop := func(context.Context, transport.PeerID) error { return errors.New("measured apart") }
				oa, ob := p2p.Options{Relays: relays}, p2p.Options{Relays: relays}
				p2p.SetPuncher(&oa, noop)
				p2p.SetPuncher(&ob, noop)
				nn.blockDirect(true)
				a := nn.node(t, 1, oa)
				b := nn.node(t, 2, ob)
				_ = a.Connect(t.Context(), b.Local(), []string{relays[0] + "/p2p-circuit"})
				_ = b.Connect(t.Context(), a.Local(), []string{relays[0] + "/p2p-circuit"})
				time.Sleep(2 * time.Second)
				nn.blockDirect(false)
				type span struct {
					from, to time.Time
					err      error
				}
				var sa, sb span
				var wg sync.WaitGroup
				wg.Go(func() { sa.from = time.Now(); sa.err = a.RealPunch(t.Context(), b.Local()); sa.to = time.Now() })
				wg.Go(func() {
					time.Sleep(off)
					sb.from = time.Now()
					sb.err = b.RealPunch(t.Context(), a.Local())
					sb.to = time.Now()
				})
				wg.Wait()
				if sa.from.Before(sb.to) && sb.from.Before(sa.to) {
					overlap.Add(1)
				}
				if sa.err != nil && sb.err != nil {
					bothFail.Add(1)
				}
				_ = a.Close()
				_ = b.Close()
			})
		}
		t.Logf("H-15 crossed offset=%v runs=20 overlap=%d bothfail=%d", off, overlap.Load(), bothFail.Load())
	}
}

// H-16 (B3), the worst case the plan's bounds assume, for one simulated hour, with the node's own upgrader quiet
// (the members announce no public address, so the node, which dialled them, has no Dial to make and no inbound
// circuit to punch over: every grant goes to a member's rounds). Each Byzantine member (raw libp2p) that the node
// dials through the relay spends its round bucket as fast as it can, its rounds chained on each grant (libp2p's
// per-peer limit of DCUtR streams allows two at once), each CONNECT carrying twenty public addresses never sent before: its rounds wait in the FIFO, share the
// member's grant when it comes (or the grant held for it, B6) and each offer two new destinations (the simnet carries
// QUIC only, so the selection keeps the first QUIC/IPv4 and QUIC/IPv6), so that every grant can spend its allowance
// of 12 and beyond. (Members
// that reconnect only lose rounds: budgets across reconnections are TestUpgradeByzantineReconnect's.) Counts the destinations actually
// dialled (gater admissions, and distinct simnet destinations of the node's packets other than the relay) and
// the packets, against the bounds: grants ≤ 8 + t/120 s; destinations ≤ 456 per node and ≤ 174 per member;
// packets ≤ 500 per destination and bytes ≤ 15.5 MB (at 1 500 B a packet). "coalition" is the plan's ten members;
// "member" is one member alone, which gets every grant, so its own buckets bind.
func TestUpgradeNodeTraffic(t *testing.T) {
	t.Run("coalition", func(t *testing.T) { nodeTraffic(t, 10) })
	t.Run("member", func(t *testing.T) { nodeTraffic(t, 1) })
}

// burstAddrs returns twenty public QUIC addresses (IPv4 and IPv6 in turn) that no other call returns: k numbers
// the burst's exchange.
func burstAddrs(k int) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, 20)
	for i := range 20 {
		j := k*20 + i
		if i%2 == 0 {
			out = append(out, ma.StringCast(fmt.Sprintf("/ip4/6.%d.%d.%d/udp/9000/quic-v1", j>>16&255, j>>8&255, j&255)))
		} else {
			out = append(out, ma.StringCast(fmt.Sprintf("/ip6/2001:db9:%x::%x/udp/9000/quic-v1", j>>16, j&0xffff)))
		}
	}
	return out
}

// sleepCtx sleeps d, or until ctx is done; it reports whether ctx is still live.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

func nodeTraffic(t *testing.T, members int) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(16)
		relays := nn.relay(t, p2p.RelayOptions{})
		nn.blockDirect(true)
		var mu sync.Mutex
		var a *simNode
		var aEnd atomic.Pointer[natEnd] // a's end, for the tap: packets are routed while a starts
		packets := 0
		dests := map[string]int{} // non-relay destination -> packets a sent to it
		nn.tap = func(src *natEnd, to string) {
			if src == aEnd.Load() && !strings.HasPrefix(to, relayIP+":") {
				mu.Lock()
				packets++
				dests[to]++
				mu.Unlock()
			}
		}
		var calls callLog
		oa := p2p.Options{Relays: relays}
		p2p.SetPuncher(&oa, realPuncher(&calls, &a))
		// Ten relayed links on one node: libp2p's limit of 8 connections per IPv4 address counts every circuit
		// under the relay's address. Lifted here for the test only (block RF1 fixes the product; this proves no
		// product capacity).
		p2p.LiftRelayIPLimit(&oa, relayIP, 64)
		a = nn.node(t, 1, oa)
		aEnd.Store(a.end)
		want := map[transport.PeerID][]string{}
		var ms []*rawMember
		for i := range members {
			m := nn.rawMember(t, byte(30+i), relays, []ma.Multiaddr{}) // announces no public address
			ms = append(ms, m)
			want[m.id] = []string{m.addr}
		}
		a.Want(want)
		start := time.Now()
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()
		var next atomic.Int64 // numbers the exchanges: every CONNECT carries new addresses
		for i, m := range ms {
			// The member spends its round bucket (9, refill 1 per 100 s: it knows the spec) as fast as it can: libp2p's
			// default limit of DCUtR streams per peer (2, limits.go) refuses a third while the node's two handlers run
			// (each until its dial ends, ≈ 5 s), so once granted it chains its rounds, one always in flight, on the
			// grant (or the grant held for it, B6) until the allowance is spent. A stream refused at once is tried
			// again a second later.
			var tmu sync.Mutex
			tokens, at := 9.0, time.Now()
			take := func() time.Duration { // 0 if a token was taken, else how long until the next one
				tmu.Lock()
				defer tmu.Unlock()
				tokens = min(9, tokens+time.Since(at).Seconds()/100)
				at = time.Now()
				if tokens >= 1 {
					tokens--
					return 0
				}
				return time.Duration((1 - tokens) * 100 * float64(time.Second))
			}
			give := func() { tmu.Lock(); tokens++; tmu.Unlock() } // the round was not admitted
			round := func() bool {                                // one exchange; true if the node admitted it (its CONNECT carried our addresses)
				if take() > 0 {
					return false
				}
				began := time.Now()
				if err := m.exchange(ctx, a.Local(), burstAddrs(int(next.Add(1))), false); err != nil {
					give()
					if time.Since(began) < time.Second {
						sleepCtx(ctx, time.Second) // refused at once: the per-peer stream limit, or not connected
					}
					return false
				}
				return true
			}
			wg.Go(func() {
				sleepCtx(ctx, time.Duration(i)*6*time.Second) // the members spread over the minute
				for ctx.Err() == nil {
					if d := take(); d > 0 {
						sleepCtx(ctx, d)
						continue
					}
					give()
					if !round() { // waits in the FIFO for the grant (≤ 20 s), or is refused
						continue
					}
					// Granted: chain five more rounds 3 s apart, so that one is always in flight on the node (each
					// holds its handler ≈ 5 s, two at most) and the grant stays in use until its allowance of 12
					// (six rounds of two destinations) is spent.
					var chain sync.WaitGroup
					for range 5 {
						if !sleepCtx(ctx, 3*time.Second) {
							break
						}
						chain.Go(func() { round() })
					}
					chain.Wait()
				}
			})
		}
		grants := 0 // at the end of the hour: the bursts still running after it are not counted
		for ctx.Err() == nil {
			time.Sleep(time.Minute)
			a.NoteActivity()
			_, grants = a.SlotsHeld()
			if limit := 8 + int(time.Since(start)/(120*time.Second)); grants > limit {
				t.Fatalf("H-16 admissions: %d grants after %v, want ≤ %d", grants, time.Since(start), limit)
			}
			if el := time.Since(start); el%(10*time.Minute) < time.Minute {
				admitted, connected, rounds := 0, 0, 0
				for _, m := range ms {
					r, d := a.PunchCounts(m.id)
					admitted += d
					rounds += r
					if m.h.Network().Connectedness(a.Host().ID()) != network.NotConnected {
						connected++
					}
				}
				mu.Lock()
				t.Logf("H-16 progress: %v simulated, %d grants, %d rounds, %d destinations admitted, %d distinct dialled, %d packets; %d members connected", el.Round(time.Minute), grants, rounds, admitted, len(dests), packets, connected)
				mu.Unlock()
			}
		}
		total, most, rounds, connects := 0, 0, 0, int64(0)
		for _, m := range ms {
			r, d := a.PunchCounts(m.id)
			total += d
			rounds += r
			most = max(most, d)
			connects += m.connects.Load()
			if d > 174 {
				t.Fatalf("H-16 destinations: %d admitted for one member in an hour, want ≤ 174", d)
			}
		}
		mu.Lock()
		perDest := 0
		for _, n := range dests {
			perDest = max(perDest, n)
		}
		t.Logf("H-16 measured in an hour (%d members): %d grants (≤ 38), %d rounds admitted, %d CONNECTs with our addresses, %d calls of ours; %d destinations admitted (≤ 456), %d distinct dialled, the most for one member %d (≤ 174); %d packets ≤ %d B (≤ 15.5 MB), the most to one destination %d (≤ 500)",
			members, grants, rounds, connects, calls.count(), total, len(dests), most, packets, packets*1500, perDest)
		distinct, sent := len(dests), packets
		mu.Unlock()
		cancel()
		wg.Wait()
		if grants > 38 {
			t.Fatalf("H-16 admissions: %d grants in an hour, want ≤ 38", grants)
		}
		// Not vacuous: the worst case is built when the bound that binds is reached, the node's admissions (every
		// grant spending most of its allowance) for the coalition, the member's round bucket (45) for one member.
		if members > 1 && (grants < 30 || total < 10*grants) {
			t.Fatalf("H-16 worst case not built: %d grants, %d destinations admitted (want ≥ 30 grants, ≥ 10 each)", grants, total)
		}
		if members == 1 && rounds < 40 {
			t.Fatalf("H-16 worst case not built: the member got %d rounds, want ≥ 40 of its 45", rounds)
		}
		if total > 456 || distinct > 456 {
			t.Fatalf("H-16 destinations: %d admitted, %d distinct dialled in an hour, want ≤ 456", total, distinct)
		}
		if distinct > total {
			t.Fatalf("H-16 destinations: %d distinct dialled, only %d admitted by the gater", distinct, total)
		}
		if sent*1500 > 15_500_000 || perDest > 500 {
			t.Fatalf("H-16 destinations: %d packets to non-relay addresses (≤ %d B, the most to one %d), want ≤ 15.5 MB and ≤ 500 each", sent, sent*1500, perDest)
		}
	})
}

// H-5 (B6): an answerer round waits behind other peers in the slot queue, gives up at 20 s and comes back: it keeps
// its ticket's place, and a grant made to it is held for its peer for the lease; direct is reached. R, a raw member
// that announces no public address (so a has no Dial to make: a only answers), initiates DCUtR rounds every 2 s as
// libp2p's initiator does; a's four slots are held in turn by eight members whose rounds stall until a's stream
// deadline, so R's rounds wait in the FIFO and give up. After each give-up R's ticket must still be queued (the
// place kept); the test counts the give-ups, and fails if there was none (the queue not exercised).
func TestUpgradeAnswererBehindQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(17)
		relays := nn.relay(t, p2p.RelayOptions{})
		nn.blockDirect(true)
		oa := p2p.Options{Relays: relays}
		p2p.LiftRelayIPLimit(&oa, relayIP, 64) // nine relayed links (block RF1 fixes the product's limit)
		a := nn.node(t, 1, oa)
		want := map[transport.PeerID][]string{}
		var hogs []*rawMember
		for i := range 8 {
			h := nn.rawMember(t, byte(40+i), relays, []ma.Multiaddr{})
			hogs = append(hogs, h)
			want[h.id] = []string{h.addr}
		}
		r := nn.rawMember(t, 50, relays, []ma.Multiaddr{})
		want[r.id] = []string{r.addr}
		a.Want(want) // a dials everyone through the relay: each member holds the inbound circuit and initiates
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		deadline := time.Now().Add(time.Minute)
		for r.h.Network().Connectedness(a.Host().ID()) == network.NotConnected {
			if time.Now().After(deadline) {
				t.Fatal("a never dialled R")
			}
			time.Sleep(100 * time.Millisecond)
		}
		var wg sync.WaitGroup
		for _, h := range hogs {
			addrs := []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", h.end.external, simPort))}
			wg.Go(func() {
				for ctx.Err() == nil {
					_ = h.exchange(ctx, a.Local(), addrs, true) // holds a slot until a's stream deadline
					sleepCtx(ctx, time.Second)                  // a refused round returns at once: no busy loop
				}
			})
		}
		time.Sleep(5 * time.Second)
		if held, _ := a.SlotsHeld(); held < 4 {
			t.Fatalf("H-5 setup: the members hold %d of a's 4 slots", held)
		}
		nn.blockDirect(false) // the hogs' addresses lead nowhere useful; R's direct path is open
		start := time.Now()
		gaveUp := 0
		for !a.HasDirect(r.id) {
			if time.Since(start) > 30*time.Minute {
				t.Fatalf("H-5 place lost: R not direct after 30 min behind members that keep asking (%d rounds gave up): %s", gaveUp, a.UpgradeDebug(r.id))
			}
			began := time.Now()
			if r.punch(ctx, a.Local()) {
				break
			}
			if time.Since(began) >= 19*time.Second { // a's round waited in the FIFO and gave up (20 s)
				gaveUp++
				if !a.PunchTicket(r.id) {
					t.Fatalf("H-5 place lost: R's round gave up after %v and R no longer holds a ticket", time.Since(began))
				}
			}
			time.Sleep(2 * time.Second)
		}
		if gaveUp == 0 {
			t.Fatalf("H-5 queue not exercised: R went direct after %v without any round waiting in the FIFO", time.Since(start))
		}
		rounds, _ := a.PunchCounts(r.id)
		t.Logf("H-5 R direct after %v: %d of its rounds gave up in the FIFO first, %d admitted", time.Since(start), gaveUp, rounds)
		cancel()
		wg.Wait()
	})
}

// H-17: idle for 30 simulated min: no ladder-driven call; on activity, a call within 1 s; an event-driven
// call (new circuit) runs at once while idle.
func TestUpgradeIdleNoLadder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(18)
		relays := nn.relay(t, p2p.RelayOptions{})
		var callsA, callsB callLog
		var a, b *simNode
		oa, ob := p2p.Options{Relays: relays}, p2p.Options{Relays: relays}
		p2p.SetPuncher(&oa, realPuncher(&callsA, &a))
		p2p.SetPuncher(&ob, realPuncher(&callsB, &b))
		nn.blockDirect(true)
		a = nn.node(t, 1, oa)
		b = nn.node(t, 2, ob)
		a.NoteActivity() // the app tracks activity from its first input on
		b.NoteActivity()
		b.Want(map[transport.PeerID][]string{a.Local(): nil})
		expectUp(t, b, a)
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		time.Sleep(6 * time.Minute) // past the active window: idle now
		idleFrom, beforeA, beforeB := time.Now(), callsA.count(), callsB.count()
		time.Sleep(30 * time.Minute)
		if n := (callsA.count() - beforeA) + (callsB.count() - beforeB); n > 0 {
			t.Fatalf("H-17 ladder call while idle: %d calls in 30 idle minutes", n)
		}
		a.NoteActivity()
		time.Sleep(time.Second)
		if callsA.count() == beforeA {
			t.Fatalf("H-17 no call within 1 s of activity (idle since %v)", time.Since(idleFrom))
		}
		// While both are idle again, either node can receive the new circuit and punch.
		time.Sleep(6 * time.Minute)
		n0 := callsA.count() + callsB.count()
		b.Hangup(a.Local())
		deadline := time.Now().Add(30 * time.Second)
		for callsA.count()+callsB.count() == n0 {
			if time.Now().After(deadline) {
				t.Fatalf("H-17 an event-driven call did not run while idle")
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

// H-18: a member's unreachable addresses cannot shut the node's honest UDP dials. libp2p's black hole detector
// (on by default) refuses every new UDP dial once fewer than 5 of the last 100 succeeded, with no time expiry and
// whoever chose the addresses; the product turns it off for UDP and IPv6, the budgets bounding the wasted
// attempts. Here the node fails 120 direct dials to public addresses nobody answers, then dials a public peer:
// the dial must go through.
func TestUpgradeUnreachableAddrsKeepUDPDials(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(19)
		a := nn.node(t, 1, p2p.Options{})
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		var wg sync.WaitGroup
		for i := range 120 {
			id := idOf(key(byte(100 + i)))
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				_ = a.Connect(ctx, id, []string{fmt.Sprintf("/ip4/6.1.%d.%d/udp/9000/quic-v1", i/200, i%200+1)})
			})
		}
		wg.Wait()
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		if err := a.Connect(ctx, b.Local(), b.Addrs()); err != nil {
			t.Fatalf("H-18 honest UDP dial after 120 failed ones: %v", err)
		}
	})
}
