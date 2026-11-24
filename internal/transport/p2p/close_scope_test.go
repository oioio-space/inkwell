package p2p_test

// Block PU1d (spec §8.7.4 « Portée d'un échec », rules P-16 and P-17; plan docs/plans/2026-11-24-pu1d-close-scope.md;
// model docs/protocol/tla/CloseScope.tla; gate gates/PU1d.toml, criterion `go`). A failed session opening (a refusal,
// a violation, the deadline) closes the connection the opening names from its start, and only it (P-16); any other
// connection still open then opens the session, with no connectedness event (P-17). Each scene puts one peer identity
// on two connections: two bare hosts with the same key at two addresses (or a member's relay circuit and a bare host
// with its key), each doing what the scene makes it do. A failure reads its rule's text.

import (
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

const (
	p16Other    = "P-16 other connection closed"
	p16Unnamed  = "P-16 unnamed opening"
	p16Loop     = "P-16 refusal loop"
	p17Stranded = "P-17 stranded"
)

// One peer identity (key 3) at two public addresses: two connections to b.
const (
	firstEnd, secondEnd = 13, 14
	firstIP, secondIP   = "1.2.13.1", "1.2.14.1"
)

// dialNode connects the bare host h to the node b; b holds an inbound connection from h's address.
func dialNode(t *testing.T, h host.Host, b *simNode) {
	t.Helper()
	if err := h.Connect(t.Context(), peer.AddrInfo{ID: libp2pID(t, b), Addrs: b.Host().Addrs()}); err != nil {
		t.Fatalf("connect %s to b: %v", h.Addrs(), err)
	}
}

// connAt returns b's open connection to p whose remote address is on ip (an /ip4/ip/ component), or nil.
func connAt(b *simNode, p peer.ID, ip string) network.Conn {
	for _, c := range b.Host().Network().ConnsToPeer(p) {
		if !c.IsClosed() && strings.Contains(c.RemoteMultiaddr().String(), "/ip4/"+ip+"/") {
			return c
		}
	}
	return nil
}

// sessionOn reports whether b's session stream to p runs on c.
func sessionOn(b *simNode, p transport.PeerID, c network.Conn) bool {
	s := b.OutStream(p)
	return s != nil && c != nil && s.Conn() == c
}

// gated makes h handle sync streams with f once gate is closed (or the test ends), counting them in n.
func gated(t *testing.T, h host.Host, gate <-chan struct{}, n *atomic.Int32, f func(network.Stream)) {
	h.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
		n.Add(1)
		select {
		case <-gate:
		case <-t.Context().Done():
			_ = s.Reset()
			return
		}
		f(s)
	})
}

// refuse resets the stream without a byte (a peer that does not admit us, §8.7.3 step 2).
func refuse(s network.Stream) { _ = s.Reset() }

// events counts b's PeerUp and PeerDown about p.
func events(r *recorder, p transport.PeerID) (ups, downs int) {
	return len(r.of(transport.PeerUp, p)), len(r.of(transport.PeerDown, p))
}

// connectedness counts the connectedness events b's libp2p emits about p from now on.
func connectedness(t *testing.T, b *simNode, p peer.ID) *atomic.Int32 {
	t.Helper()
	sub, err := b.Host().EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		t.Fatal(err)
	}
	var n atomic.Int32
	go func() {
		for e := range sub.Out() {
			if e.(event.EvtPeerConnectednessChanged).Peer == p {
				n.Add(1)
			}
		}
	}()
	t.Cleanup(func() { _ = sub.Close() })
	return &n
}

// TestFailedOpeningClosesOnlyItsConnection: P-16. b's opening runs on its only connection; a second connection of
// the same peer arrives; the first refuses the stream. b closes the refused connection, keeps the other, and the
// session opens there (P-17): one PeerUp, no PeerDown.
func TestFailedOpeningClosesOnlyItsConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(21)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		refuser, healthy := nn.rawHost(t, 3, firstEnd), nn.rawHost(t, 3, secondEnd)
		gate := make(chan struct{})
		var refused atomic.Int32
		gated(t, refuser, gate, &refused, refuse)
		serve(healthy, p2p.ProtocolSync, ackByte)
		p, name := refuser.ID(), nameOf(refuser)
		rb := record(t, b)
		dialNode(t, refuser, b)
		time.Sleep(time.Second)
		dialNode(t, healthy, b)
		time.Sleep(time.Second)
		if refused.Load() != 1 || connAt(b, p, secondIP) == nil {
			t.Fatalf("fixture: %d openings on the first connection (want 1, in progress), second connection open: %v", refused.Load(), connAt(b, p, secondIP) != nil)
		}
		close(gate)
		time.Sleep(2 * time.Second)
		other := connAt(b, p, secondIP)
		if other == nil {
			t.Errorf("%s: the refusal of the opening on %s closed the connection from %s too", p16Other, firstIP, secondIP)
		}
		if connAt(b, p, firstIP) != nil {
			t.Errorf("P-16 refused connection kept: the connection from %s is still open 2 s after it refused the opening", firstIP)
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 || !sessionOn(b, name, other) {
			t.Errorf("%s: %d PeerUp, %d PeerDown, session on the remaining connection: %v; want 1, 0, true", p16Other, ups, downs, sessionOn(b, name, other))
		}
	})
}

// TestDeadlineBeforeReturnClosesNamedConnection: P-16, review finding 1. b negotiates its sync streams; its opening
// runs on a connection whose responder never echoes, so NewStream never returns; a second connection arrives. At the
// 15 s deadline the opening, which named its connection from its start, closes that one alone and the session opens
// on the other.
func TestDeadlineBeforeReturnClosesNamedConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(22)
		var o p2p.Options
		p2p.SetExtra(&o, negotiating(t))
		b := nn.nodeAt(t, 2, true, o)
		mute, healthy := nn.rawHost(t, 3, firstEnd), nn.rawHost(t, 3, secondEnd)
		slowSync(mute, syncack.Budget+time.Second, 0) // the echo comes after the budget
		serve(healthy, p2p.ProtocolSync, ackByte)
		p, name := mute.ID(), nameOf(mute)
		rb := record(t, b)
		start := time.Now()
		dialNode(t, mute, b)
		time.Sleep(time.Second)
		dialNode(t, healthy, b)
		time.Sleep(time.Until(start.Add(syncack.Budget - time.Second)))
		if connAt(b, p, firstIP) == nil || connAt(b, p, secondIP) == nil || len(rb.of(transport.PeerUp, name)) != 0 {
			t.Fatalf("fixture: before the deadline both connections must be open and nothing announced (first %v, second %v)", connAt(b, p, firstIP) != nil, connAt(b, p, secondIP) != nil)
		}
		time.Sleep(3 * time.Second) // the deadline, the close, the reopening and its ACK
		other := connAt(b, p, secondIP)
		switch {
		case connAt(b, p, firstIP) != nil:
			t.Errorf("%s: the connection the opening ran on (%s) is still open %v after the opening started, past its 15 s budget", p16Unnamed, firstIP, time.Since(start))
		case other == nil:
			t.Errorf("%s: the deadline of the opening on %s closed the connection from %s, which it did not run on", p16Unnamed, firstIP, secondIP)
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 || !sessionOn(b, name, other) {
			t.Errorf("%s: %d PeerUp, %d PeerDown, session on the remaining connection: %v; want 1, 0, true", p16Unnamed, ups, downs, sessionOn(b, name, other))
		}
	})
}

// TestDeadlineOnStalledCircuitKeepsDirect: P-16. b's opening runs on a relay circuit to a member; the relay stops
// forwarding (the member's side goes dark as the stream arrives); a direct connection of the same member arrives. At
// the deadline b closes the circuit, keeps the direct connection, and the session opens on it.
func TestDeadlineOnStalledCircuitKeepsDirect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(23)
		relays := nn.relay(t, p2p.RelayOptions{})
		b := nn.nodeAt(t, 2, true, p2p.Options{Relays: relays})
		m := nn.rawMember(t, 20, relays, nil)
		m.h.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
			nn.setBlackout(m.end, true) // the relay forwards nothing more to b
			<-t.Context().Done()
			_ = s.Reset()
		})
		direct := nn.rawHost(t, 20, secondEnd) // the member's key at a public address
		serve(direct, p2p.ProtocolSync, ackByte)
		p, name := m.h.ID(), nameOf(m.h)
		rb := record(t, b)
		start := time.Now()
		if err := b.Connect(t.Context(), name, []string{m.addr}); err != nil {
			t.Fatalf("b connects to the member through the relay: %v", err)
		}
		time.Sleep(time.Second)
		dialNode(t, direct, b)
		time.Sleep(time.Until(start.Add(syncack.Budget - time.Second)))
		circuit := b.Host().Network().ConnsToPeer(p)
		if connAt(b, p, secondIP) == nil || len(circuit) < 2 || len(rb.of(transport.PeerUp, name)) != 0 {
			t.Fatalf("fixture: before the deadline the circuit and the direct connection must be open, nothing announced (%d connections)", len(circuit))
		}
		time.Sleep(3 * time.Second)
		other := connAt(b, p, secondIP)
		if other == nil {
			t.Errorf("%s: the deadline of the opening on the stalled circuit closed the direct connection", p16Other)
		}
		for _, c := range b.Host().Network().ConnsToPeer(p) {
			if !c.IsClosed() && c != other {
				t.Errorf("P-16 stalled circuit kept: %s is still open %v after the opening started", c.RemoteMultiaddr(), time.Since(start))
			}
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 || !sessionOn(b, name, other) {
			t.Errorf("%s: %d PeerUp, %d PeerDown, session on the direct connection: %v; want 1, 0, true", p16Other, ups, downs, sessionOn(b, name, other))
		}
	})
}

// TestResetBeforeConnCloseKeepsOther: P-16, review finding 2 (§8.7.3 step 3, which PeerUpAck does not model). The
// responder resets b's stream, then closes its connection 200 ms later: b reads the reset first, on a connection still
// open, as a refusal. It closes that connection only (which its responder was closing anyway): the other stays and
// carries the session.
func TestResetBeforeConnCloseKeepsOther(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(24)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		closing, healthy := nn.rawHost(t, 3, firstEnd), nn.rawHost(t, 3, secondEnd)
		gate := make(chan struct{})
		var opened atomic.Int32
		gated(t, closing, gate, &opened, func(s network.Stream) {
			_ = s.Reset()
			time.Sleep(200 * time.Millisecond) // b reads the reset on a live connection, then the close comes
			_ = s.Conn().Close()
		})
		serve(healthy, p2p.ProtocolSync, ackByte)
		p, name := closing.ID(), nameOf(closing)
		rb := record(t, b)
		dialNode(t, closing, b)
		time.Sleep(time.Second)
		dialNode(t, healthy, b)
		time.Sleep(time.Second)
		if opened.Load() != 1 {
			t.Fatalf("fixture: %d openings on the first connection, want 1 in progress", opened.Load())
		}
		close(gate)
		time.Sleep(2 * time.Second)
		other := connAt(b, p, secondIP)
		if other == nil {
			t.Errorf("%s: a reset that arrived before its connection's close closed the connection from %s", p16Other, secondIP)
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 || !sessionOn(b, name, other) {
			t.Errorf("%s: %d PeerUp, %d PeerDown, session on the remaining connection: %v; want 1, 0, true", p16Other, ups, downs, sessionOn(b, name, other))
		}
	})
}

// TestPeerRefusingEveryConnection: P-16. A peer that does not admit us refuses on both its connections: they close
// one by one, one opening each, and nothing is announced (it never was).
func TestPeerRefusingEveryConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(25)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		first, second := nn.rawHost(t, 3, firstEnd), nn.rawHost(t, 3, secondEnd)
		gate, open := make(chan struct{}), make(chan struct{})
		close(open)
		var n1, n2 atomic.Int32
		gated(t, first, gate, &n1, refuse)
		gated(t, second, open, &n2, refuse)
		p, name := first.ID(), nameOf(first)
		rb := record(t, b)
		dialNode(t, first, b)
		time.Sleep(time.Second)
		dialNode(t, second, b)
		time.Sleep(time.Second)
		close(gate)
		time.Sleep(3 * time.Second)
		if n1.Load() != 1 || n2.Load() != 1 {
			t.Errorf("%s: %d and %d openings on the two connections, want one each", p16Loop, n1.Load(), n2.Load())
		}
		if cs := b.Host().Network().ConnsToPeer(p); len(cs) != 0 {
			t.Errorf("%s: %d connections still open 3 s after the refusals, want none", p16Loop, len(cs))
		}
		if ups, downs := events(rb, name); ups != 0 || downs != 0 {
			t.Errorf("%s: %d PeerUp, %d PeerDown of a peer that refused every opening, want none", p16Loop, ups, downs)
		}
	})
}

// TestByteAfterAckClosesItsConnection: P-16 after the ACK (§8.7.2: a byte after it is a violation). The session runs on
// the first connection; a second one arrives; the first writes a byte after its ACK. b closes the first connection
// only; the peer stays announced and the session reopens on the second (P-17).
func TestByteAfterAckClosesItsConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(26)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		hostile, healthy := nn.rawHost(t, 3, firstEnd), nn.rawHost(t, 3, secondEnd)
		gate := make(chan struct{})
		hostile.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
			_, _ = s.Write([]byte{ackByte})
			select {
			case <-gate:
				_, _ = s.Write([]byte{ackByte})
				<-t.Context().Done()
			case <-t.Context().Done():
			}
			_ = s.Reset()
		})
		serve(healthy, p2p.ProtocolSync, ackByte)
		p, name := hostile.ID(), nameOf(hostile)
		rb := record(t, b)
		dialNode(t, hostile, b)
		time.Sleep(time.Second)
		dialNode(t, healthy, b)
		time.Sleep(time.Second)
		if !sessionOn(b, name, connAt(b, p, firstIP)) || connAt(b, p, secondIP) == nil {
			t.Fatalf("fixture: the session must run on the first connection with the second open")
		}
		close(gate)
		time.Sleep(2 * time.Second)
		other := connAt(b, p, secondIP)
		if other == nil {
			t.Errorf("%s: a byte after the ACK on %s closed the connection from %s", p16Other, firstIP, secondIP)
		}
		if connAt(b, p, firstIP) != nil {
			t.Errorf("P-16 violating connection kept: the connection from %s is still open after a byte after its ACK", firstIP)
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 || !sessionOn(b, name, other) {
			t.Errorf("%s: %d PeerUp, %d PeerDown, session on the remaining connection: %v; want 1, 0 (the peer stays announced), true", p16Other, ups, downs, sessionOn(b, name, other))
		}
	})
}

// TestRemainingConnectionOpens: P-17. The session's connection ends while another one of the same peer stays open:
// libp2p emits no connectedness event (the peer stays connected), so the node must open the session on the remaining
// connection by itself, within one opening.
func TestRemainingConnectionOpens(t *testing.T) {
	scene := func(t *testing.T, seed uint64, end func(t *testing.T, first host.Host, b *simNode, gate chan struct{})) {
		nn := newNatNet(seed)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		first, healthy := nn.rawHost(t, 3, firstEnd), nn.rawHost(t, 3, secondEnd)
		gate := make(chan struct{})
		serve(healthy, p2p.ProtocolSync, ackByte)
		p, name := first.ID(), nameOf(first)
		rb := record(t, b)
		end(t, first, b, gate) // sets first's handler
		dialNode(t, first, b)
		time.Sleep(time.Second)
		dialNode(t, healthy, b)
		time.Sleep(time.Second)
		events0 := connectedness(t, b, p)
		close(gate)
		time.Sleep(time.Second)
		other := connAt(b, p, secondIP)
		if other == nil {
			t.Fatalf("fixture: the remaining connection from %s is closed", secondIP)
		}
		if !sessionOn(b, name, other) {
			t.Errorf("%s: 1 s after the session's connection ended, no session on the remaining connection from %s (stream %v)", p17Stranded, secondIP, b.OutStream(name) != nil)
		}
		if got := events0.Load(); got != 0 {
			t.Errorf("fixture: libp2p emitted %d connectedness events about the peer, want none (it stays connected)", got)
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 { // one PeerUp in all: a reopening announces nothing
			t.Errorf("%s: %d PeerUp, %d PeerDown, want 1, 0", p17Stranded, ups, downs)
		}
	}
	// The remote closes the session's connection after its ACK: the stream is lost with it.
	t.Run("lost", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			scene(t, 27, func(t *testing.T, first host.Host, b *simNode, gate chan struct{}) {
				first.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
					_, _ = s.Write([]byte{ackByte})
					select {
					case <-gate:
						_ = s.Conn().Close()
					case <-t.Context().Done():
					}
					_ = s.Reset()
				})
			})
		})
	})
	// The remote refuses the opening on the first connection: b closes it (P-16).
	t.Run("refused", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			scene(t, 28, func(t *testing.T, first host.Host, _ *simNode, gate chan struct{}) {
				var n atomic.Int32
				gated(t, first, gate, &n, refuse)
			})
		})
	})
}

// TestPreferredCircuitStalledKeepsNewDirect: P-16 on HP1's reopen route (code review finding 1). The session runs on a
// direct connection, moved there from the member's circuit; the direct connection is lost, so the reopening prefers
// the surviving circuit (HP1, the PU1b `drain_replace_reconnect` replay), whose relay then forwards nothing more; a
// new direct connection arrives while that opening holds the peer's only opening. At the deadline the circuit alone is
// closed, the new direct connection stays and carries the session, and the peer stays announced. Before PU1d the
// deadline closed the peer (ClosePeer), the new direct connection with it.
func TestPreferredCircuitStalledKeepsNewDirect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(29)
		relays := nn.relay(t, p2p.RelayOptions{})
		b := nn.nodeAt(t, 2, true, p2p.Options{Relays: relays})
		m := nn.rawMember(t, 20, relays, nil)
		var onCircuit atomic.Int32
		m.h.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
			if onCircuit.Add(1) > 1 { // the reopening after the direct loss
				nn.setBlackout(m.end, true)
				<-t.Context().Done()
				_ = s.Reset()
				return
			}
			_, _ = s.Write([]byte{ackByte})
			_, _ = io.Copy(io.Discard, s)
			_ = s.Reset()
		})
		first, fresh := nn.rawHost(t, 20, secondEnd), nn.rawHost(t, 20, 15) // the member's key at two public addresses
		serve(first, p2p.ProtocolSync, ackByte)
		serve(fresh, p2p.ProtocolSync, ackByte)
		p, name := m.h.ID(), nameOf(m.h)
		rb := record(t, b)
		if err := b.Connect(t.Context(), name, []string{m.addr}); err != nil {
			t.Fatalf("b connects to the member through the relay: %v", err)
		}
		time.Sleep(time.Second)
		dialNode(t, first, b) // the session moves to this direct connection
		time.Sleep(2 * time.Second)
		direct := connAt(b, p, secondIP)
		if !sessionOn(b, name, direct) || len(b.Host().Network().ConnsToPeer(p)) != 2 {
			t.Fatalf("fixture: the session must run on the direct connection, the circuit still open (%d connections)", len(b.Host().Network().ConnsToPeer(p)))
		}
		lost := time.Now()
		_ = first.Network().ClosePeer(b.Host().ID()) // the direct connection is lost: the reopening prefers the circuit
		time.Sleep(time.Second)
		if onCircuit.Load() != 2 {
			t.Fatalf("fixture: %d sync streams on the circuit, want 2 (the session, then the reopening after the direct loss)", onCircuit.Load())
		}
		dialNode(t, fresh, b) // a new direct connection, while the opening on the circuit holds
		time.Sleep(time.Until(lost.Add(syncack.Budget + 2*time.Second)))
		other := connAt(b, p, "1.2.15.1")
		if other == nil {
			t.Errorf("%s: the deadline of the reopening on the stalled circuit closed the new direct connection", p16Other)
		}
		for _, c := range b.Host().Network().ConnsToPeer(p) {
			if !c.IsClosed() && c != other {
				t.Errorf("P-16 stalled circuit kept: %s is still open %v after the direct loss", c.RemoteMultiaddr(), time.Since(lost))
			}
		}
		if ups, downs := events(rb, name); ups != 1 || downs != 0 || !sessionOn(b, name, other) {
			t.Errorf("%s: %d PeerUp, %d PeerDown, session on the new direct connection: %v; want 1, 0 (the peer stays announced), true", p16Other, ups, downs, sessionOn(b, name, other))
		}
	})
}
