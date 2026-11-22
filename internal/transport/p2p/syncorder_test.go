package p2p_test

// Block PU1b (spec §8.7.3, §8.7.4, plan docs/plans/2026-11-16-pu1b-inbound-order.md, gate gates/PU1b.toml): the
// order of a peer's inbound sync streams. Two places per peer, shared by its connections and generations; a stream
// that resets or closes with no unread or probed bytes leaves at once and wakes its successors. A further stream is refused for capacity (reset
// with code 0x1002) and the opener retries within its 15 s budget, keeping its session.

import (
	"errors"
	"io"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

const (
	pu1bOver  = "PU1b over capacity"
	pu1bKept  = "PU1b place kept"
	pu1bTurn  = "PU1b turn missed"
	pu1bOrder = "PU1b order broken"
	pu1bLost  = "PU1b session lost"
	busyCode  = network.StreamErrorCode(0x1002) // spec §8.7.3, written from the text
)

// isBusy reports whether err is a reset of the remote with the capacity code.
func isBusy(err error) bool {
	var se *network.StreamError
	return errors.As(err, &se) && se.Remote && se.ErrorCode == busyCode
}

// rawStream is a sync stream a raw opener opened and whose ACK it read.
type rawStream struct{ network.Stream }

// openAcked opens a sync stream from h to p and reads its first byte: the stream, or the error (a reset with the
// capacity code is a refusal for capacity).
func openAcked(t *testing.T, h host.Host, p peer.ID) (*rawStream, error) {
	t.Helper()
	s, err := openAckedOn(t, h, p, p2p.ProtocolSync)
	if err != nil {
		return nil, err
	}
	return &rawStream{s}, nil
}

// mustAck is openAcked that fails the test.
func mustAck(t *testing.T, h host.Host, p peer.ID) *rawStream {
	t.Helper()
	s, err := openAcked(t, h, p)
	if err != nil {
		t.Fatalf("open a sync stream: %v", err)
	}
	return s
}

// within waits until cond holds, or fails with what after d.
func within(t *testing.T, d time.Duration, cond func() bool, what string) {
	t.Helper()
	for end := time.Now().Add(d); !cond(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("%s (after %v)", what, d)
		}
	}
}

// orderScene is a Node b and a raw opener (key 3) connected, b announcing the raw host (which acknowledges b's own
// stream).
type orderScene struct {
	nn  *natNet
	b   *simNode
	raw host.Host
	bID peer.ID
	rb  *recorder
}

func newOrderScene(t *testing.T, seed uint64) *orderScene {
	t.Helper()
	nn := newNatNet(seed)
	b := nn.nodeAt(t, 2, true, p2p.Options{})
	raw := nn.rawHost(t, 3, 13)
	serve(raw, p2p.ProtocolSync, ackByte)
	sc := &orderScene{nn: nn, b: b, raw: raw, bID: libp2pID(t, b)}
	if err := raw.Connect(t.Context(), peer.AddrInfo{ID: sc.bID, Addrs: b.Host().Addrs()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	expectEventWithin(t, b, transport.PeerUp, nameOf(raw), wait)
	sc.rb = record(t, b)
	return sc
}

func (sc *orderScene) places() int { return sc.b.InboundPlaces("sync", nameOf(sc.raw)) }

// rawResponder is a raw host that refuses its first `busy` sync streams for capacity (all if busy < 0), then
// acknowledges.
func (nn *natNet) rawResponder(t *testing.T, busy int) host.Host {
	t.Helper()
	raw := nn.rawHost(t, 3, 13)
	var n atomic.Int32
	raw.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
		if busy < 0 || int(n.Add(1)) <= busy {
			_ = s.ResetWithError(busyCode)
			return
		}
		_, _ = s.Write([]byte{ackByte})
		buf := make([]byte, 64)
		for {
			if _, err := s.Read(buf); err != nil {
				break
			}
		}
		_ = s.Reset()
	})
	return raw
}

func TestSyncOrder(t *testing.T) {
	// P-8: two streams of one peer hold places.
	t.Run("at_bound", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newOrderScene(t, 1)
			s1, s2 := mustAck(t, sc.raw, sc.bID), mustAck(t, sc.raw, sc.bID)
			defer func() { _ = s1.Reset() }()
			defer func() { _ = s2.Reset() }()
			within(t, time.Second, func() bool { return sc.places() == 2 }, "two acknowledged streams do not hold two places")
		})
	})

	// P-8: a third stream is refused for capacity: reset with 0x1002, no byte, the connection and session kept.
	t.Run("bound_plus_one", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newOrderScene(t, 2)
			s1, s2 := mustAck(t, sc.raw, sc.bID), mustAck(t, sc.raw, sc.bID)
			defer func() { _ = s1.Reset() }()
			defer func() { _ = s2.Reset() }()
			s3, err := openAcked(t, sc.raw, sc.bID)
			if err == nil {
				_ = s3.Reset()
				t.Fatalf("%s: a third stream of one peer was acknowledged (places %d)", pu1bOver, sc.places())
			}
			if !isBusy(err) {
				t.Fatalf("the third stream ended with %v, want a reset with code 0x1002", err)
			}
			time.Sleep(time.Second)
			if got := sc.places(); got != 2 {
				t.Errorf("%s: %d places after the refusal, want 2", pu1bOver, got)
			}
			if got := sc.b.BusyRefusals(); got != 1 {
				t.Errorf("BusyRefusals = %d, want 1", got)
			}
			if len(sc.raw.Network().ConnsToPeer(sc.bID)) == 0 || len(sc.rb.others()) > 0 {
				t.Errorf("%s: the refusal closed the connection or changed the session: events %v", pu1bLost, sc.rb.others())
			}
		})
	})

	// P-8: the place given back is taken again; and a Node opener refused twice for capacity retries (50 then 100 ms)
	// and announces the peer, never closing it.
	t.Run("recovery", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newOrderScene(t, 3)
			s1, s2 := mustAck(t, sc.raw, sc.bID), mustAck(t, sc.raw, sc.bID)
			defer func() { _ = s2.Reset() }()
			if _, err := openAcked(t, sc.raw, sc.bID); !isBusy(err) {
				t.Fatalf("a third stream: %v, want a refusal for capacity", err)
			}
			_ = s1.CloseWrite() // read to its end: its place is given back
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1bKept+": the drained stream kept its place")
			s4, err := openAcked(t, sc.raw, sc.bID)
			if err != nil {
				t.Fatalf("%s: the retry after a place was given back: %v", pu1bKept, err)
			}
			_ = s4.Reset()
		})
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(4)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawResponder(t, 2)
			life := watchConn(b.Host(), raw.ID())
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(2 * time.Second)
			ups := rb.of(transport.PeerUp, nameOf(raw))
			up, down := life.times()
			switch {
			case len(ups) != 1 || !down.IsZero():
				t.Fatalf("%s: %d PeerUp, connection closed %v after it opened; want one PeerUp, the connection kept",
					pu1bLost, len(ups), down.Sub(up))
			case ups[0].at.Sub(up) < 150*time.Millisecond:
				t.Errorf("PeerUp %v after the connection, want after the waits of 50 and 100 ms", ups[0].at.Sub(up))
			case b.BusyRetries() != 2:
				t.Errorf("BusyRetries = %d, want 2", b.BusyRetries())
			}
		})
	})

	// P-8 with P-5: a remote that always refuses for capacity is retried until the budget runs out, then closed.
	t.Run("busy_forever", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(5)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawResponder(t, -1)
			life := watchConn(b.Host(), raw.ID())
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(16 * time.Second)
			noUp(t, rb, b.Local(), nameOf(raw), "which always refused for capacity")
			if b.BusyRetries() < 2 {
				t.Errorf("%s: %d refusals for capacity read, want them retried", pu1bLost, b.BusyRetries())
			}
			closedAtBudget(t, life, "always busy")
		})
	})

	// P-8: go-libp2p's own resource manager refuses the stream during its negotiation, with the same code.
	t.Run("resource_manager_refusal", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(6)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			scaling := rcmgr.DefaultLimits
			libp2p.SetDefaultServiceLimits(&scaling)
			limits := rcmgr.PartialLimitConfig{ProtocolPeer: map[protocol.ID]rcmgr.ResourceLimits{
				p2p.ProtocolSync: {Streams: rcmgr.BlockAllLimit, StreamsInbound: rcmgr.BlockAllLimit, StreamsOutbound: rcmgr.BlockAllLimit},
			}}
			rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits.Build(scaling.AutoScale())))
			if err != nil {
				t.Fatal(err)
			}
			raw := nn.rawHostWith(t, 3, 13, libp2p.ResourceManager(rm))
			serve(raw, p2p.ProtocolSync, ackByte) // never reached: the resource manager resets first
			life := watchConn(b.Host(), raw.ID())
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(16 * time.Second)
			noUp(t, rb, b.Local(), nameOf(raw), "whose resource manager refuses every sync stream")
			if b.BusyRetries() < 2 {
				t.Errorf("%s: %d refusals for capacity read from the resource manager, want them retried", pu1bLost, b.BusyRetries())
			}
			closedAtBudget(t, life, "resource manager refusal")
		})
	})

	// P-8, P-9: a Byzantine opener holds a first stream open and resets (or ends without a frame) 50 following ones
	// right after their ACK: never more than two places, none kept, and no goroutine per failure.
	t.Run("byzantine_reset_loop", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newOrderScene(t, 7)
			s1 := mustAck(t, sc.raw, sc.bID)
			defer func() { _ = s1.Reset() }()
			time.Sleep(time.Second)
			before := runtime.NumGoroutine()
			for i := range 50 {
				s, err := openAcked(t, sc.raw, sc.bID)
				for isBusy(err) { // the previous one has not left yet: retry, as an opener would
					time.Sleep(50 * time.Millisecond)
					s, err = openAcked(t, sc.raw, sc.bID)
				}
				if err != nil {
					t.Fatalf("stream %d: %v", i, err)
				}
				if i%2 == 0 {
					_ = s.Reset()
				} else {
					_ = s.CloseWrite()
				}
				if got := sc.places(); got > 2 {
					t.Fatalf("%s: %d places after %d streams", pu1bOver, got, i+1)
				}
			}
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1bKept+": the failed streams kept places")
			time.Sleep(time.Second)
			if after := runtime.NumGoroutine(); after > before+8 {
				t.Errorf("%s: %d goroutines after 50 failed streams, %d before", pu1bKept, after, before)
			}
		})
	})

	// P-9: a waiting stream that resets or closes for writing without sending a byte frees its place.
	// When an empty head leaves, its waiting successor takes its turn.
	t.Run("removal_while_waiting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newOrderScene(t, 8)
			head := mustAck(t, sc.raw, sc.bID) // held open, no frame
			s2 := mustAck(t, sc.raw, sc.bID)
			_ = s2.Reset()
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1bKept+": a waiting stream reset kept its place")
			s3 := mustAck(t, sc.raw, sc.bID)
			_ = s3.CloseWrite()
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1bKept+": a waiting stream closed without a byte kept its place")
			s4 := mustAck(t, sc.raw, sc.bID)
			defer func() { _ = s4.Reset() }()
			if err := frame.Write(s4, []byte("four")); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			if got := sc.rb.of(transport.Message, nameOf(sc.raw)); len(got) != 0 {
				t.Fatalf("%s: the waiting stream delivered before its turn: %v", pu1bOrder, got)
			}
			_ = head.Reset()
			within(t, time.Second, func() bool { return len(sc.rb.of(transport.Message, nameOf(sc.raw))) == 1 },
				pu1bTurn+": the waiting stream did not read when the head left without a frame")
		})
	})

	// P-11: a waiting stream resets within the probe bound even after sending a byte. A FIN after
	// probed bytes keeps the place until its turn. The decoder discards a truncated frame and
	// delivers a complete frame.
	t.Run("byte_then_end_while_waiting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newOrderScene(t, 11)
			head := mustAck(t, sc.raw, sc.bID)
			s2 := mustAck(t, sc.raw, sc.bID)
			if _, err := s2.Write([]byte{0}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			_ = s2.Reset()
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1bKept+": a waiting stream reset after one byte kept its place")
			s3 := mustAck(t, sc.raw, sc.bID)
			if _, err := s3.Write([]byte{0}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			_ = s3.CloseWrite()
			time.Sleep(time.Second)
			if got := sc.places(); got != 2 {
				t.Fatalf("%s: %d places: a waiting stream that ended after a probed byte left before its turn (P-11)", pu1bTurn, got)
			}
			_ = head.Reset()
			within(t, time.Second, func() bool { return sc.places() == 0 }, pu1bKept+": the kept stream did not leave at its turn")
			if got := sc.rb.of(transport.Message, nameOf(sc.raw)); len(got) != 0 {
				t.Fatalf("a truncated frame was delivered: %v", got)
			}
			head2 := mustAck(t, sc.raw, sc.bID)
			s4 := mustAck(t, sc.raw, sc.bID)
			defer func() { _ = s4.Reset() }()
			if err := frame.Write(s4, []byte("whole")); err != nil {
				t.Fatal(err)
			}
			_ = s4.CloseWrite()
			time.Sleep(time.Second)
			if got := sc.places(); got != 2 {
				t.Fatalf("%s: %d places: a waiting stream with a whole frame left before its turn", pu1bTurn, got)
			}
			_ = head2.Reset()
			within(t, time.Second, func() bool { return len(sc.rb.of(transport.Message, nameOf(sc.raw))) == 1 },
				pu1bTurn+": the whole frame of a stream that ended while waiting was not delivered at its turn")
		})
	})

	// P-8, P-9: drain + replacement + reconnect. b's handlers of a's relayed and direct streams are held after their
	// ACK; the direct connection is closed; a's reopen on the circuit needs a third place: refused for capacity, then
	// placed once the lost stream leaves. No event either side; the frames b delivers keep their order.
	t.Run("drain_replace_reconnect", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			release := make(chan struct{})
			var held atomic.Int32
			sc := newMoveScene(t, 9, func(o *p2p.Options) {
				p2p.SetAfterAck(o, func(network.Conn) {
					if held.Add(1) <= 2 {
						<-release
					}
				})
			})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			for deadline := time.Now().Add(2 * time.Minute); ; time.Sleep(50 * time.Millisecond) {
				if direct, ok := sc.a.OutDirect(sc.b.Local()); ok && direct {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("a's session never moved to the direct connection")
				}
				sc.send(t)
			}
			sc.send(t)
			bID := libp2pID(t, sc.b)
			for _, c := range sc.a.Host().Network().ConnsToPeer(bID) {
				if isDirect(c) {
					_ = c.Close()
				}
			}
			within(t, 10*time.Second, func() bool { return sc.b.BusyRefusals() >= 1 },
				pu1bOver+": the reopen on the circuit was never refused for capacity (or was given a third place)")
			close(release)
			for range 10 {
				sc.send(t)
				time.Sleep(50 * time.Millisecond)
			}
			time.Sleep(5 * time.Second)
			if evs := append(sc.ra.others(), sc.rb.others()...); len(evs) > 0 {
				t.Fatalf("%s: events %v", pu1bLost, evs)
			}
			got := sc.rb.of(transport.Message, sc.a.Local())
			for i := 1; i < len(got); i++ {
				if string(got[i].Payload) <= string(got[i-1].Payload) {
					t.Fatalf("%s: frame % x after % x", pu1bOrder, got[i].Payload, got[i-1].Payload)
				}
			}
			if len(got) == 0 {
				t.Fatalf("%s: b delivered nothing", pu1bTurn)
			}
		})
	})

	// P-8 across generations: a and b go down and up again while b's handler of a's old stream still holds a place;
	// a's new stream waits behind it, which leaves without a frame when released.
	t.Run("generation_change", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(10)
			release := make(chan struct{})
			var held atomic.Int32
			ob := p2p.Options{}
			p2p.SetAfterAck(&ob, func(network.Conn) {
				if held.Add(1) == 1 {
					<-release
				}
			})
			a := nn.nodeAt(t, 1, true, p2p.Options{})
			b := nn.nodeAt(t, 2, true, ob)
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			expectUp(t, a, b)
			ra, rb := record(t, a), record(t, b)
			before := runtime.NumGoroutine()
			_ = a.Host().Network().ClosePeer(libp2pID(t, b))
			within(t, 10*time.Second, func() bool {
				return len(ra.of(transport.PeerDown, b.Local())) == 1 && len(rb.of(transport.PeerDown, a.Local())) == 1
			}, "no PeerDown on both sides")
			if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
				t.Fatalf("reconnect: %v", err)
			}
			within(t, 10*time.Second, func() bool {
				return len(ra.of(transport.PeerUp, b.Local())) == 1 && len(rb.of(transport.PeerUp, a.Local())) == 1
			}, pu1bLost+": no PeerUp on both sides of the new generation")
			if got := b.InboundPlaces("sync", a.Local()); got != 2 {
				t.Fatalf("places %d while the old handler holds one, want 2 (shared across generations)", got)
			}
			var mu sync.Mutex
			var sent [][]byte
			for i := range 5 {
				p := []byte{byte('a' + i)}
				if err := a.Send(t.Context(), b.Local(), p); err != nil {
					t.Fatalf("Send: %v", err)
				}
				mu.Lock()
				sent = append(sent, p)
				mu.Unlock()
			}
			time.Sleep(200 * time.Millisecond)
			close(release)
			within(t, 5*time.Second, func() bool { return len(rb.of(transport.Message, a.Local())) == 5 },
				pu1bTurn+": the new generation's frames were not delivered once the old handler left")
			got := rb.of(transport.Message, a.Local())
			for i, m := range got {
				if !slices.Equal(m.Payload, sent[i]) {
					t.Fatalf("%s: frame %d is %q, want %q", pu1bOrder, i, m.Payload, sent[i])
				}
			}
			within(t, time.Second, func() bool { return b.InboundPlaces("sync", a.Local()) == 1 }, pu1bKept+": the old handler kept its place")
			if n := len(ra.of(transport.PeerDown, b.Local())) + len(rb.of(transport.PeerDown, a.Local())); n != 2 {
				t.Errorf("%d PeerDown in all, want one per side", n)
			}
			time.Sleep(time.Second)
			if after := runtime.NumGoroutine(); after > before+8 {
				t.Errorf("%s: %d goroutines after the generation change, %d before", pu1bKept, after, before)
			}
		})
	})
}

// P-4 with the connection's loss (Codex, code review of PU1b): how an outbound stream's end is read. A byte after
// the ACK, or a reset by the remote, is a refusal even if the connection is closed by the time it is looked at (the
// remote may reset then close one connection while another survives); only the connection's own close error, or a
// local end on a closed connection, is the connection lost.
func TestOutEnded(t *testing.T) {
	remoteReset := &network.StreamError{ErrorCode: 0, Remote: true}
	connLost := &network.ConnError{Remote: true}
	for _, c := range []struct {
		name       string
		n          int
		err        error
		connClosed bool
		want       bool // the connection is lost (reopen), not a refusal
	}{
		{"byte after the ACK", 1, nil, false, false},
		{"byte after the ACK, connection closed", 1, nil, true, false},
		{"remote reset", 0, remoteReset, false, false},
		{"remote reset then the connection closed", 0, remoteReset, true, false},
		{"the connection's close", 0, connLost, false, true},
		{"the connection's close, marked", 0, connLost, true, true},
		{"local reset on a closed connection", 0, &network.StreamError{Remote: false}, true, true},
		{"end on a live connection", 0, io.EOF, false, false},
		{"end on a closed connection", 0, io.EOF, true, true},
	} {
		if got := p2p.OutEndedByConn(c.n, c.err, c.connClosed); got != c.want {
			t.Errorf("%s: connection lost = %v, want %v (%s)", c.name, got, c.want, pu1bLost)
		}
	}
}
