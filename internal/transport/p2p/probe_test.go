package p2p_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/quic-go/quic-go"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// The link probe (spec §8.5, ADR 0036, block C5S) on the simulated network: a
// real node pair, a seeded latency per packet, and the network's own count of
// what it drops as the truth the probe is judged against.

// pathWiFi is netsim's `wifi` profile at one hop: the Gilbert-Elliott chain of
// burstyWiFi with a bad state twice as likely, 3.2 % loss per packet, which is
// the 3.6 % of the two netem hops of the path, well inside the fair band
// (2.5 % to 15 %) so that the level asked of the test does not depend on the
// seed.
var pathWiFi = impairment{enterBad: 0.02, leaveBad: 0.30, lossBad: 0.50, lossGood: 0.001}

// toleranceMille is the accuracy the figure keeps in every modelled condition
// (ADR 0036 « Levels »: within 15 permille at the 99.9th percentile).
const toleranceMille = 15

// directPair returns two public nodes connected to each other, announced.
func directPair(t *testing.T, nn *natNet, oa, ob p2p.Options) (a, b *simNode) {
	t.Helper()
	a = nn.nodeAt(t, 1, true, oa)
	b = nn.nodeAt(t, 2, true, ob)
	if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	expectUp(t, a, b)
	return a, b
}

// typing makes from send a small frame to to every period, 1000 bytes each, and
// drains what to receives, until the returned stop is called. At 10 ms it puts
// about 100 packets a second on the link each way.
func typing(t *testing.T, from, to *simNode, every time.Duration, size int) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			if _, err := to.Recv(ctx); err != nil {
				return
			}
		}
	})
	wg.Go(func() {
		payload := make([]byte, size)
		for {
			if err := from.Send(ctx, to.Local(), payload); err != nil {
				return
			}
			select {
			case <-time.After(every):
			case <-ctx.Done():
				return
			}
		}
	})
	return func() {
		cancel()
		wg.Wait()
	}
}

// both is typing in both directions.
func both(t *testing.T, a, b *simNode, every time.Duration) (stop func()) {
	s1, s2 := typing(t, a, b, every, 1000), typing(t, b, a, every, 1000)
	return func() { s1(); s2() }
}

// quicOf returns quic-go's connection from n to peer.
func quicOf(t *testing.T, n *simNode, peer transport.PeerID) *quic.Conn {
	t.Helper()
	id, ok := p2p.LibP2PID(peer)
	if !ok {
		t.Fatalf("%q is not a peer id", peer)
	}
	for _, c := range n.Host().Network().ConnsToPeer(id) {
		var qc *quic.Conn
		if c.As(&qc) && qc != nil {
			return qc
		}
	}
	t.Fatalf("%s: no QUIC connection to %s", n.Local(), peer)
	return nil
}

// truth is the share of the packets a sent that the network did not deliver
// between its snapshots from and to, in permille.
func truth(from, to [2]uint64) int {
	sent, dropped := to[0]-from[0], to[1]-from[1]
	if sent == 0 {
		return 0
	}
	return int((1000*dropped + sent/2) / sent)
}

// observe runs the network for seconds, recording once a second what a's end
// sent and the network dropped; it returns the snapshots.
func observe(nn *natNet, a *simNode, seconds int) [][2]uint64 {
	snaps := make([][2]uint64, 0, seconds)
	for range seconds {
		time.Sleep(time.Second)
		s, d := nn.counts(a.end)
		snaps = append(snaps, [2]uint64{s, d})
	}
	return snaps
}

func absInt(x int) int { return max(x, -x) }

// figure formats a probe figure for a failure message.
func figure(p *uint16) string {
	if p == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d permille", *p)
}

// TestLinkLevelBurstyWiFi is the accuracy claim of ADR 0036 on the real stack:
// under bursty loss the level is fair, and the figure is within 15 permille of
// what the network dropped over its window.
func TestLinkLevelBurstyWiFi(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		nn.setImpairment(pathWiFi)
		a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
		stop := both(t, a, b, 10*time.Millisecond)
		defer stop()

		snaps := observe(nn, a, 80)
		l := linkOf(t, a, b.Local())
		n := len(snaps)
		// The figure ends up to two seconds before it is read and spans 30.
		want := truth(snaps[n-33], snaps[n-2])
		if l.ProbeLoss == nil {
			t.Fatalf("figure unknown after %d s of traffic (truth %d permille)", n, want)
		}
		if want < 15 {
			t.Fatalf("the network dropped %d permille over the window: the profile is not impairing", want)
		}
		if got := int(*l.ProbeLoss); absInt(got-want) > toleranceMille {
			t.Errorf("figure = %d permille, the network dropped %d: off by more than %d", got, want, toleranceMille)
		}
		if l.ProbeLevel != transport.QualityFair {
			t.Errorf("level = %v at %d permille, want fair", l.ProbeLevel, *l.ProbeLoss)
		}
		if l.LossScope != transport.LossPath {
			t.Errorf("scope = %v, want path", l.LossScope)
		}
	})
}

// TestLinkLevelIgnoresReordering is the anti-vacuity of the probe: with no loss
// at all and packets overtaking each other several places within quic-go's
// tolerance (less than the ACK loop, spec §8.5.2; netsim's P4), quic-go declares
// several percent of its packets lost (its PacketsLost counts a sender's guesses, RFC 9002
// §6.1), but the link is good and the figure is about zero. (Reordering beyond
// about one round trip is another matter: quic-go's receiver then drops the late
// packet as a probable duplicate, before PacketsReceived counts it, and the
// probe counts it as a loss, as spec §8.5.2 defines; TestLinkProbeReceivedCounter pins it.)
func TestLinkLevelIgnoresReordering(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		nn.setJitter(32 * time.Millisecond) // packets overtake each other several places, within quic-go's tolerance (netsim P4)
		a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
		stop := both(t, a, b, 10*time.Millisecond)
		defer stop()

		snaps := observe(nn, a, 80)
		l := linkOf(t, a, b.Local())
		st := quicOf(t, a, b.Local()).ConnectionStats()
		if st.PacketsLost == 0 {
			t.Fatalf("quic-go declared no packet lost under reordering: the test proves nothing")
		}
		if n := len(snaps); snaps[n-1][1] != 0 {
			t.Fatalf("the network dropped %d packets with no impairment", snaps[n-1][1])
		}
		if l.ProbeLoss == nil {
			t.Fatalf("figure unknown after 80 s of traffic")
		}
		if *l.ProbeLoss > toleranceMille {
			t.Errorf("figure = %d permille on a link that drops nothing (quic-go declared %d of %d lost)", *l.ProbeLoss, st.PacketsLost, st.PacketsSent)
		}
		if l.ProbeLevel != transport.QualityGood {
			t.Errorf("level = %v with %d of %d packets declared lost by quic-go, want good", l.ProbeLevel, st.PacketsLost, st.PacketsSent)
		}
		// The declarations must be enough to make a figure built from them fair (the
		// midpoint is 10 permille, the entry from unknown is placed against it),
		// or the test could not tell the probe from them.
		if declared := int(1000 * st.PacketsLost / st.PacketsSent); declared < 20 {
			t.Errorf("quic-go declared %d permille lost: too few to tell the probe from its declarations", declared)
		}
	})
}

// probeEchoes replaces the probe responder of n by a scripted one that echoes
// every PROBE with the connection's received count, after lag(); with lie set it
// reports only half of the packets that arrived since lie was set. It returns
// the number of echoes it delayed.
func probeEchoes(ctx context.Context, n *simNode, lag func() time.Duration, lie *atomic.Bool) *atomic.Int64 {
	var delayed atomic.Int64
	n.Host().SetStreamHandler(p2p.ProtocolProbe, func(s network.Stream) {
		defer func() { _ = s.Reset() }()
		var qc *quic.Conn
		if !s.Conn().As(&qc) {
			return
		}
		var base, floor uint64
		lying := false
		for {
			var msg [6]byte // len 5, type 1 (PROBE), seq u32
			if _, err := io.ReadFull(s, msg[:]); err != nil {
				return
			}
			r := qc.ConnectionStats().PacketsReceived
			switch {
			case lie.Load() && !lying:
				lying, base = true, r
			case lying:
				r = base + (r-base)/2
			}
			r = max(r, floor)
			floor = r
			if d := lag(); d > 0 {
				delayed.Add(1)
				select { // the test ending must not leave this goroutine asleep in the bubble
				case <-time.After(d):
				case <-ctx.Done():
					return
				}
			}
			echo := make([]byte, 14) // len 13, type 2 (ECHO), seq u32, received u64
			echo[0], echo[1] = 13, 2
			copy(echo[2:6], msg[2:6])
			binary.BigEndian.PutUint64(echo[6:], r)
			if _, err := s.Write(echo); err != nil {
				return
			}
		}
	})
	return &delayed
}

// TestLinkLevelLateEchoDiscarded: an echo that arrives more than one PTO after
// its PROBE is dropped (spec §8.5.5 step 3.6), because the PROBE waited and
// packets crossed it. A responder that answers 400 ms late and understates what
// it received would make the link poor if its echoes counted; discarded, they
// leave the figure unknown once the last good sample is 30 s old.
func TestLinkLevelLateEchoDiscarded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(3)
		a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
		var late, lying atomic.Bool
		delayed := probeEchoes(t.Context(), b, func() time.Duration {
			if late.Load() {
				return 400 * time.Millisecond
			}
			return 0
		}, &lying)
		stop := both(t, a, b, 10*time.Millisecond)
		defer stop()

		time.Sleep(50 * time.Second)
		if l := linkOf(t, a, b.Local()); l.ProbeLoss == nil || l.ProbeLevel != transport.QualityGood {
			t.Fatalf("before the late echoes: level %v, figure %s, want good and known", l.ProbeLevel, figure(l.ProbeLoss))
		}
		late.Store(true)
		lying.Store(true)
		for range 45 {
			time.Sleep(time.Second)
			if l := linkOf(t, a, b.Local()); l.ProbeLevel == transport.QualityPoor || (l.ProbeLoss != nil && *l.ProbeLoss > 100) {
				t.Fatalf("a late echo moved the figure to %s (level %v)", figure(l.ProbeLoss), l.ProbeLevel)
			}
		}
		if n := delayed.Load(); n < 30 {
			t.Fatalf("the scripted responder delayed %d echoes, want at least 30", n)
		}
		if l := linkOf(t, a, b.Local()); l.ProbeLoss != nil || l.ProbeLevel != transport.QualityUnknown {
			t.Errorf("after 45 s of late echoes: level %v, figure %s, want unknown (no accepted echo in 30 s)", l.ProbeLevel, figure(l.ProbeLoss))
		}
	})
}

// TestLinkLevelRestartsUnknown: all the probe's state belongs to one connection
// (spec §8.5.5): a new connection to the same peer starts at unknown, whatever
// the old one had shown, and learns its own level again.
func TestLinkLevelRestartsUnknown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(4)
		a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
		stop := both(t, a, b, 10*time.Millisecond)

		time.Sleep(50 * time.Second)
		if l := linkOf(t, a, b.Local()); l.ProbeLevel != transport.QualityGood || l.ProbeLoss == nil {
			t.Fatalf("before the restart: level %v figure %s, want good and known", l.ProbeLevel, figure(l.ProbeLoss))
		}
		stop() // the traffic's readers would take the events
		id, _ := p2p.LibP2PID(b.Local())
		if err := a.Host().Network().ClosePeer(id); err != nil {
			t.Fatalf("ClosePeer: %v", err)
		}
		expectEventWithin(t, a, transport.PeerDown, b.Local(), wait)
		if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
			t.Fatalf("Connect again: %v", err)
		}
		expectEventWithin(t, a, transport.PeerUp, b.Local(), wait)
		stop = both(t, a, b, 10*time.Millisecond)
		defer stop()
		for range 10 { // the figure needs 15 s and 200 packets
			time.Sleep(time.Second)
			if l := linkOf(t, a, b.Local()); l.ProbeLevel != transport.QualityUnknown || l.ProbeLoss != nil {
				t.Fatalf("after the new connection: level %v figure %s, want unknown", l.ProbeLevel, figure(l.ProbeLoss))
			}
		}
		time.Sleep(50 * time.Second)
		if l := linkOf(t, a, b.Local()); l.ProbeLevel != transport.QualityGood {
			t.Errorf("a minute after the new connection: level %v, want good again", l.ProbeLevel)
		}
	})
}

// TestLinkLevelRelayedUnknown: the probe measures one connection, so behind a
// relay it measures the first hop only (Annex A.1): the figure is kept as a
// diagnostic with scope firstHop, and the level, which would mislead as the
// quality of the link, stays unknown.
func TestLinkLevelRelayedUnknown(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		nn.blockDirect(true)
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		expectUp(t, a, b)
		stop := both(t, a, b, 10*time.Millisecond)
		defer stop()

		for range 70 {
			time.Sleep(time.Second)
			if l := linkOf(t, a, b.Local()); l.ProbeLevel != transport.QualityUnknown {
				t.Fatalf("relayed link shows level %v, want unknown", l.ProbeLevel)
			}
		}
		l := linkOf(t, a, b.Local())
		if l.Kind != transport.LinkRelayed {
			t.Fatalf("link = %v, want relayed", l.Kind)
		}
		if l.ProbeLoss == nil || l.LossScope != transport.LossFirstHop {
			t.Errorf("relayed figure = %s scope %v, want a first-hop figure (the relay answers its clients)", figure(l.ProbeLoss), l.LossScope)
		}
	})
}

// TestLinkProbeAlwaysRead: the probe streams are read whatever the writers do
// (spec §8.5.3): while a bulk transfer fills the link, the probe keeps getting
// its echoes, the figure follows the connection's own counters, and a link that
// loses next to nothing stays good. (The simulated receive queue drops a few
// packets under load, as a real bottleneck would: the truth here is what the
// connection itself counted, not the network's drops.)
func TestLinkProbeAlwaysRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(5)
		a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
		qa, qb := quicOf(t, a, b.Local()), quicOf(t, b, a.Local())
		stopT := both(t, a, b, 10*time.Millisecond)
		defer stopT()
		stopB := typing(t, a, b, 5*time.Millisecond, 1200)
		defer stopB()

		var snaps [][2]uint64
		for range 80 {
			time.Sleep(time.Second)
			snaps = append(snaps, [2]uint64{qa.ConnectionStats().PacketsSent, qa.ConnectionStats().PacketsSent - qb.ConnectionStats().PacketsReceived})
		}
		n := len(snaps)
		want := truth(snaps[n-33], snaps[n-2])
		l := linkOf(t, a, b.Local())
		if l.ProbeLoss == nil {
			t.Fatalf("figure unknown after 80 s under a bulk transfer: the probe's echoes were not read")
		}
		if got := int(*l.ProbeLoss); absInt(got-want) > toleranceMille {
			t.Errorf("figure = %d permille, the connection's counters say %d", got, want)
		}
		if l.ProbeLevel != transport.QualityGood {
			t.Errorf("bulk transfer: level %v at %d permille (counters %d), want good", l.ProbeLevel, *l.ProbeLoss, want)
		}
		in, out := probeStreams(t, a, b.Local())
		if in != 1 || out != 1 {
			t.Errorf("probe streams on the connection = %d inbound, %d outbound, want 1 and 1", in, out)
		}
	})
}

// probeStreams counts the open probe streams of n's connection to peer.
func probeStreams(t *testing.T, n *simNode, peer transport.PeerID) (in, out int) {
	t.Helper()
	id, _ := p2p.LibP2PID(peer)
	for _, c := range n.Host().Network().ConnsToPeer(id) {
		for _, s := range c.GetStreams() {
			if s.Protocol() != p2p.ProtocolProbe {
				continue
			}
			if s.Stat().Direction == network.DirInbound {
				in++
			} else {
				out++
			}
		}
	}
	return in, out
}

// TestLinkProbeRoles: the peer's roles decide, on the real connection (spec
// §8.5.1). Members probe and answer each other; a guest of the lobby is neither
// probed nor answered; a member removed meanwhile has its probe streams reset.
func TestLinkProbeRoles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(6)
		var member atomic.Bool
		member.Store(true)
		a := nn.nodeAt(t, 1, true, p2p.Options{})
		b := nn.nodeAt(t, 2, true, p2p.Options{Admit: func(id transport.PeerID) bool { return id == a.Local() && member.Load() }})
		if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		expectUp(t, a, b)
		time.Sleep(5 * time.Second)
		if in, out := probeStreams(t, a, b.Local()); in != 1 || out != 1 {
			t.Errorf("members: a has %d inbound and %d outbound probe streams, want 1 and 1", in, out)
		}
		if in, out := probeStreams(t, b, a.Local()); in != 1 || out != 1 {
			t.Errorf("members: b has %d inbound and %d outbound probe streams, want 1 and 1", in, out)
		}

		member.Store(false) // a is removed from b's project
		time.Sleep(3 * time.Second)
		if in, out := probeStreams(t, b, a.Local()); in != 0 || out != 0 {
			t.Errorf("removed: b has %d inbound and %d outbound probe streams, want none", in, out)
		}
		if in, out := probeStreams(t, a, b.Local()); in != 0 || out != 0 {
			t.Errorf("removed: a has %d inbound and %d outbound probe streams, want b's resets to have ended them", in, out)
		}
	})

	synctest.Test(t, func(t *testing.T) { // a guest: admitted only by the stranger window
		nn := newNatNet(7)
		a := nn.nodeAt(t, 1, true, p2p.Options{})
		b := nn.nodeAt(t, 2, true, p2p.Options{Admit: func(transport.PeerID) bool { return false }})
		b.SetOpen(time.Now().Add(time.Hour))
		if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		time.Sleep(20 * time.Second)
		if in, out := probeStreams(t, b, a.Local()); in != 0 || out != 0 {
			t.Errorf("guest: b has %d inbound and %d outbound probe streams, want none", in, out)
		}
		if in, out := probeStreams(t, a, b.Local()); in != 0 || out != 0 {
			t.Errorf("guest: a has %d inbound and %d outbound probe streams, want none (b refuses a guest)", in, out)
		}
	})
}

// TestLinkProbeReceivedCounter pins what the probe relies on in quic-go
// (spec §8.5.6, informative; to be redone at each upgrade): PacketsReceived
// counts packets once, a duplicate not at all, and it is read right after the
// stream data is handed over, which leaves it at most one packet behind the
// packet that carried the data.
func TestLinkProbeReceivedCounter(t *testing.T) {
	t.Run("late packets", latePacketDiscard)
	for _, tc := range []struct {
		name string
		im   impairment
	}{{"clean", impairment{}}, {"every packet duplicated", impairment{dup: 1}}} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				nn := newNatNet(8)
				a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
				qb := quicOf(t, b, a.Local())
				type reading struct{ before, atRead, settled uint64 }
				readings := make(chan reading, 64)
				var before atomic.Uint64
				b.Host().SetStreamHandler("/inkwell/test/count", func(s network.Stream) {
					defer func() { _ = s.Reset() }()
					var one [1]byte
					for {
						if _, err := io.ReadFull(s, one[:]); err != nil {
							return
						}
						r := reading{before: before.Load(), atRead: qb.ConnectionStats().PacketsReceived}
						time.Sleep(time.Microsecond) // every goroutine blocked: the connection's own work is done
						r.settled = qb.ConnectionStats().PacketsReceived
						readings <- r
					}
				})
				id, _ := p2p.LibP2PID(b.Local())
				s, err := a.Host().NewStream(t.Context(), id, "/inkwell/test/count")
				if err != nil {
					t.Fatalf("NewStream: %v", err)
				}
				defer func() { _ = s.Reset() }()
				nn.setImpairment(tc.im)
				time.Sleep(3 * time.Second) // the handshake and the first probes are over
				clean := 0
				for range 40 {
					time.Sleep(237 * time.Millisecond)
					before.Store(qb.ConnectionStats().PacketsReceived)
					if _, err := s.Write([]byte{1}); err != nil {
						t.Fatalf("Write: %v", err)
					}
					r := <-readings
					if r.settled-r.before != 1 {
						continue // another packet arrived meanwhile (a probe, an ACK): not an isolated one
					}
					clean++
					if r.atRead != r.before && r.atRead != r.settled {
						t.Errorf("PacketsReceived at the read = %d, want %d (not yet counted) or %d (counted)", r.atRead, r.before, r.settled)
					}
				}
				if clean < 25 {
					t.Errorf("only %d of 40 data packets arrived alone, want at least 25: a duplicate or a stray packet was counted", clean)
				}
			})
		})
	}
}

// latePacketDiscard (a subtest of TestLinkProbeReceivedCounter) pins, for each quic-go upgrade, the other half of spec §8.5.2:
// a packet that arrives more than about one ACK loop after a later one is discarded by the
// receiver as a possible duplicate and never counted, though the network delivered it; a shorter
// reordering loses nothing. The network drops nothing in both cases.
func latePacketDiscard(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hold     time.Duration
		min, max float64 // the share of a's packets that b did not count
	}{
		{"within the ACK loop", 6 * time.Millisecond, 0, 0.01},
		{"beyond the ACK loop", 60 * time.Millisecond, 0.08, 0.30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				nn := newNatNet(8)
				nn.setReorder(0.2, tc.hold)
				a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
				stop := both(t, a, b, 10*time.Millisecond)
				time.Sleep(40 * time.Second)
				stop()
				time.Sleep(5 * time.Second)
				sent, dropped := nn.counts(a.end)
				if dropped != 0 {
					t.Fatalf("the network dropped %d packets", dropped)
				}
				qa, qb := quicOf(t, a, b.Local()).ConnectionStats(), quicOf(t, b, a.Local()).ConnectionStats()
				if got := float64(qa.PacketsSent-qb.PacketsReceived) / float64(qa.PacketsSent); got < tc.min || got > tc.max {
					t.Errorf("b did not count %.1f %% of a's %d packets (network sent %d), want %.0f to %.0f %%", 100*got, qa.PacketsSent, sent, 100*tc.min, 100*tc.max)
				}
			})
		})
	}
}

// oneWayMedianRTT returns the median round trip of a path whose one-way latency is baseDelay plus
// extra and a uniform draw below jitter (natNet.RecvPacket): the
// true round trip the probe is judged against, drawn from a fixed seed.
func oneWayMedianRTT(extra, jitter time.Duration) time.Duration {
	rng := rand.New(rand.NewPCG(7, 7)) // #nosec G404 -- a model of the network, not a secret.
	oneWay := func() time.Duration {
		d := baseDelay + extra + time.Duration(rng.Int64N(int64(jitter)))
		return d
	}
	rtts := make([]time.Duration, 20001)
	for i := range rtts {
		rtts[i] = oneWay() + oneWay()
	}
	slices.Sort(rtts)
	return rtts[len(rtts)/2]
}

// TestLinkRTTFromProbe is spec Annex A.1 « Sens de rttMs et rttVarMs »: on a direct path whose
// probe figure is known, the link's round trip is the probe's, which measures every ECHO, within
// [0.9 R, 1.2 R] of the true median R under reordering and dense traffic, where quic-go's smoothed
// round trip (RFC 9002 §5.1: one sample per newly largest acknowledged packet) reads low (the
// anti-vacuity); a relayed link, whose probe sees only the first hop, keeps the end-to-end ping.
func TestLinkRTTFromProbe(t *testing.T) {
	t.Run("direct", func(t *testing.T) {
		// The bias is a statistical property of quic-go's estimator, which wanders sample by
		// sample: it is judged on the median over the seeds, the probe's bound on every seed.
		var mu sync.Mutex
		var ratios []float64
		forSeeds(t, func(t *testing.T, nn *natNet) {
			// About 60 ms of round trip and 24 ms of jitter against packets 4 ms apart: they
			// overtake each other, and the bias shows (netsim P1).
			const extra, jitter = 20 * time.Millisecond, 24 * time.Millisecond
			nn.setLatency(extra)
			nn.setJitter(jitter)
			r := oneWayMedianRTT(extra, jitter)
			a, b := directPair(t, nn, p2p.Options{}, p2p.Options{})
			stop := both(t, a, b, 8*time.Millisecond)
			defer stop()
			time.Sleep(60 * time.Second)

			l := linkOf(t, a, b.Local())
			if l.ProbeLoss == nil {
				t.Fatalf("figure unknown after 60 s of traffic")
			}
			if lo, hi := r*9/10, r*12/10; l.RTT < lo || l.RTT > hi {
				t.Errorf("RTT = %v, want within [%v, %v] (true median %v)", l.RTT, lo, hi, r)
			}
			if l.RTTVar <= 0 || l.RTTVar > r {
				t.Errorf("RTTVar = %v, want within (0, %v]", l.RTTVar, r)
			}
			if l.RTTAge < 0 || l.RTTAge > 3*time.Second {
				t.Errorf("RTTAge = %v, want the age of a sample at most a probe interval old", l.RTTAge)
			}
			sr := quicOf(t, a, b.Local()).ConnectionStats().SmoothedRTT
			mu.Lock()
			ratios = append(ratios, float64(sr)/float64(r))
			mu.Unlock()
		})
		slices.Sort(ratios)
		if len(ratios) == 0 {
			t.Fatal("no seed ran")
		}
		if m := ratios[(len(ratios)-1)/2]; m >= 0.95 {
			t.Errorf("quic-go SmoothedRTT / R: median %.3f over the seeds %.3f, want below 0.95: the profile does not show the bias", m, ratios)
		}
	})
	t.Run("relayed keeps ping", func(t *testing.T) {
		forSeeds(t, func(t *testing.T, nn *natNet) {
			nn.setLatency(20 * time.Millisecond)
			nn.setJitter(24 * time.Millisecond)
			nn.blockDirect(true)
			relays := nn.relay(t, p2p.RelayOptions{})
			a := nn.node(t, 1, p2p.Options{Relays: relays})
			b := nn.node(t, 2, p2p.Options{Relays: relays})
			a.Want(map[transport.PeerID][]string{b.Local(): nil})
			expectUp(t, a, b)
			stop := both(t, a, b, 10*time.Millisecond)
			defer stop()
			time.Sleep(60 * time.Second)

			l := linkOf(t, a, b.Local())
			if l.Kind != transport.LinkRelayed {
				t.Fatalf("kind = %v, want relayed", l.Kind)
			}
			// The first hop's round trip is one path's; the ping crosses two paths each way.
			hop := oneWayMedianRTT(20*time.Millisecond, 24*time.Millisecond)
			if l.RTT < hop*3/2 {
				t.Errorf("relayed RTT = %v, want the end-to-end ping, well above the first hop's %v", l.RTT, hop)
			}
		})
	})
}

// TestLinkLevelLogsEverySample: the sampler reports every step of every link,
// the unknown ones included, once a second, so that a diagnostics build can log
// them all and a test can replay the level from the figures (plan C5S, R1).
func TestLinkLevelLogsEverySample(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(9)
		var mu sync.Mutex
		var steps []p2p.LinkStep
		a, b := directPair(t, nn, p2p.Options{OnLinkStep: func(s p2p.LinkStep) {
			mu.Lock()
			steps = append(steps, s)
			mu.Unlock()
		}}, p2p.Options{})
		stop := both(t, a, b, 10*time.Millisecond)
		defer stop()
		time.Sleep(60 * time.Second)

		mu.Lock()
		defer mu.Unlock()
		if len(steps) < 55 {
			t.Fatalf("%d steps in 60 s, want one a second", len(steps))
		}
		for i, s := range steps {
			if s.Peer != b.Local() {
				t.Fatalf("step %d is for %s, want %s", i, s.Peer, b.Local())
			}
			if i > 0 && s.At.Sub(steps[i-1].At) != time.Second {
				t.Errorf("steps %d and %d are %v apart, want 1s: a sampler step was not reported", i-1, i, s.At.Sub(steps[i-1].At))
			}
		}
		if first := steps[0]; first.Level != transport.QualityUnknown || first.LossPermille != p2p.LossUnknown {
			t.Errorf("first step = %+v, want unknown (the figure needs 15 s)", first)
		}
		last, l := steps[len(steps)-1], linkOf(t, a, b.Local())
		if l.ProbeLoss == nil || last.LossPermille != *l.ProbeLoss || last.Level != l.ProbeLevel {
			t.Errorf("last step = %+v, Links() shows level %v figure %s: the log is not what is shown", last, l.ProbeLevel, figure(l.ProbeLoss))
		}
	})
}

// TestLinkProbeSharedRelayHopLookedAtOnce: the peers reached through one relay share the
// connection to it, and the sampler looks at its probe once per step, not once per peer.
func TestLinkProbeSharedRelayHopLookedAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(11)
		nn.blockDirect(true)
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		c := nn.node(t, 3, p2p.Options{Relays: relays})
		a.Want(map[transport.PeerID][]string{b.Local(): nil, c.Local(): nil})
		for range 2 {
			ev := recvWithin(t, a, 30*time.Second)
			if ev.Kind != transport.PeerUp {
				t.Fatalf("event %v, want PeerUp", ev.Kind)
			}
		}
		before := a.RelayViews()
		time.Sleep(20 * time.Second)
		if got := a.RelayViews() - before; got > 21 {
			t.Errorf("the relay hop was looked at %d times in 20 sampler steps, want about once per step", got)
		}
	})
}
