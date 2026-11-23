package p2p_test

// Block PU1c (spec §8.1, §8.7.3, §16.1, plan docs/plans/2026-11-17-pu1c-inbound-lifecycle.md, gate gates/PU1c.toml):
// one lifecycle for every inbound stream of the two ordered lanes, sync and bulk. Every time bound is checked on the
// controlled time of testing/synctest and the simulated network, with no slack beyond the network's own one-way
// latency (the jitter is off: a packet takes baseDelay).

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/quic-go/quic-go"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

const (
	pu1cKept    = "PU1c place kept"
	pu1cBlind   = "PU1c blind wait"
	pu1cLate    = "PU1c late wake"
	pu1cBudget  = "PU1c budget taken"
	pu1cLost    = "PU1c frames lost"
	pu1cOrder   = "PU1c order broken"
	pu1cBlocked = "PU1c blocked forever"
	pu1cStarved = "PU1c head starved"
	pu1cOver    = "PU1c over capacity"
	pu1cMem     = "PU1c memory unaccounted"

	// The bounds of the spec (§8.7.3, §16.1), written from its text.
	probeEvery    = 250 * time.Millisecond
	probeMax      = 256
	reorderWaitPU = time.Minute
	upWait        = 30 * time.Second
	streamWindow0 = 128 << 10
	connWindow0   = 1 << 20

	// oneWay is the latency of a packet on the simulated network with no jitter: a bound on b's side is measured
	// from the moment the packet reaches b.
	oneWay = baseDelay
)

// lanes are the two ordered lanes.
var lanes = []struct {
	name  string
	proto protocol.ID
}{{"sync", p2p.ProtocolSync}, {"bulk", p2p.ProtocolBulk}}

// laneScene is a Node b and a raw opener (key 3) connected on the simulated network with no jitter, b announcing
// the raw host; rec records b's events of the lane under test, ev b's node events.
type laneScene struct {
	nn      *natNet
	b       *simNode
	raw     host.Host
	bID     peer.ID
	lane    string
	proto   protocol.ID
	rec, ev *recorder
}

func newLaneScene(t *testing.T, seed uint64, lane string, configure ...func(*p2p.Options)) *laneScene {
	t.Helper()
	nn := newNatNet(seed)
	nn.setJitter(1)
	var o p2p.Options
	for _, f := range configure {
		f(&o)
	}
	b := nn.nodeAt(t, 2, true, o)
	bulk := b.Bulk()
	raw := nn.rawHost(t, 3, 13)
	serve(raw, p2p.ProtocolSync, ackByte)
	sc := &laneScene{nn: nn, b: b, raw: raw, bID: libp2pID(t, b), lane: lane, proto: p2p.ProtocolSync}
	if lane == "bulk" {
		sc.proto = p2p.ProtocolBulk
	}
	if err := raw.Connect(t.Context(), peer.AddrInfo{ID: sc.bID, Addrs: b.Host().Addrs()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	expectEventWithin(t, b, transport.PeerUp, nameOf(raw), wait)
	sc.ev = record(t, b)
	sc.rec = sc.ev
	if lane == "bulk" {
		sc.rec = record(t, bulk)
	}
	return sc
}

func (sc *laneScene) peer() transport.PeerID { return nameOf(sc.raw) }
func (sc *laneScene) places() int            { return sc.b.InboundPlaces(sc.lane, sc.peer()) }
func (sc *laneScene) msgs() []recorded       { return sc.rec.of(transport.Message, sc.peer()) }

// open opens an acknowledged stream of the lane from the raw opener.
func (sc *laneScene) open(t *testing.T) network.Stream {
	t.Helper()
	s, err := openAckedOn(t, sc.raw, sc.bID, sc.proto)
	if err != nil {
		t.Fatalf("open a %s stream: %v", sc.lane, err)
	}
	return s
}

// openAckedOn is openAcked on protocol proto.
func openAckedOn(t *testing.T, h host.Host, p peer.ID, proto protocol.ID) (network.Stream, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s, err := h.NewStream(ctx, p, proto)
	if err != nil {
		return nil, err
	}
	_ = s.SetReadDeadline(time.Now().Add(5 * time.Second))
	var b [1]byte
	if _, err := s.Read(b[:]); err != nil {
		_ = s.Reset()
		return nil, err
	}
	_ = s.SetReadDeadline(time.Time{})
	if b[0] != ackByte {
		_ = s.Reset()
		return nil, errors.New("not the ACK")
	}
	return s, nil
}

// frames writes each payload as a frame on s.
func frames(t *testing.T, s network.Stream, payloads ...[]byte) {
	t.Helper()
	for _, p := range payloads {
		if err := frame.Write(s, p); err != nil {
			t.Fatalf("write a frame: %v", err)
		}
	}
}

// numbered returns n payloads of size bytes, each starting with its number.
func numbered(n, size int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = binary.BigEndian.AppendUint32(make([]byte, 0, size), uint32(i))
		out[i] = append(out[i], bytes.Repeat([]byte{byte(i)}, size-4)...)
	}
	return out
}

// inOrder fails with pu1cOrder or pu1cLost unless got are exactly want, in order.
func inOrder(t *testing.T, got []recorded, want [][]byte, what string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %s: %d frames delivered, want %d", pu1cLost, what, len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i].Payload, want[i]) {
			t.Fatalf("%s: %s: frame %d is not the %d-th written", pu1cOrder, what, i, i)
		}
	}
}

// firstTrue polls cond every millisecond for d and returns how long it took to hold; ok is false if it never did.
func firstTrue(d time.Duration, cond func() bool) (time.Duration, bool) {
	start := time.Now()
	for !cond() {
		if time.Since(start) > d {
			return 0, false
		}
		time.Sleep(time.Millisecond)
	}
	return time.Since(start), true
}

// endKinds are the ends of the matrix besides the kept stream's.
var endKinds = []string{"reset", "fin", "conn_lost", "node_close"}

// end applies kind to s; for node_close it returns a channel closed once b's Close returned.
func (sc *laneScene) end(kind string, s network.Stream) <-chan struct{} {
	closed := make(chan struct{})
	switch kind {
	case "reset":
		_ = s.Reset()
	case "fin":
		_ = s.CloseWrite()
	case "conn_lost":
		_ = sc.raw.Network().ClosePeer(sc.bID)
	case "node_close":
		go func() {
			_ = sc.b.Close()
			close(closed)
		}()
	}
	return closed
}

// freed fails with pu1cKept unless the place of the stream that ended with kind is given back within d: left
// places remain (none after the connection's loss), or b's Close returns.
func (sc *laneScene) freed(t *testing.T, kind string, left int, d time.Duration, closed <-chan struct{}) {
	t.Helper()
	switch kind {
	case "node_close":
		select {
		case <-closed:
		case <-time.After(d):
			t.Fatalf("%s: b's Close did not return within %v", pu1cKept, d)
		}
		if got := sc.places(); got != 0 {
			t.Fatalf("%s: %d places after b's Close", pu1cKept, got)
		}
	case "conn_lost":
		within(t, d, func() bool { return sc.places() == 0 }, pu1cKept+": a place outlived its connection")
	default:
		within(t, d, func() bool { return sc.places() == left }, pu1cKept+": the stream that ended ("+kind+") kept its place")
	}
}

// noSessionChange fails unless b kept its session with the raw opener: no event but Messages, a connection.
func (sc *laneScene) noSessionChange(t *testing.T, what string) {
	t.Helper()
	if evs := sc.ev.others(); len(evs) > 0 || len(sc.raw.Network().ConnsToPeer(sc.bID)) == 0 {
		t.Errorf("%s: the session changed: events %v, %d connections", what, evs, len(sc.raw.Network().ConnsToPeer(sc.bID)))
	}
}

// holdAck returns an option holding, in its ACK write, the first inbound stream of lane, and the channels on which
// the held stream is handed and released.
func holdAck(lane string) (func(*p2p.Options), <-chan network.Stream, chan struct{}) {
	held := make(chan network.Stream, 1)
	release := make(chan struct{})
	var once atomic.Bool
	return func(o *p2p.Options) {
		p2p.SetAckHold(o, func(l string, s network.Stream) {
			if l == lane && once.CompareAndSwap(false, true) {
				held <- s
				<-release
			}
		})
	}, held, release
}

func TestInboundLifecycle(t *testing.T) {
	seed := uint64(100)
	next := func() uint64 { seed++; return seed }
	for _, ln := range lanes {
		lane := ln.name
		// P-10: held in its ACK write, the stream sees its end through that write: the place is freed within 1 s of
		// the write's end (a closed write side lets the ACK through; the probe then sees the end at once), no event.
		for _, kind := range endKinds {
			s0 := next()
			t.Run(fmt.Sprintf("matrix/%s/accepting/%s", lane, kind), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					hold, held, release := holdAck(lane)
					sc := newLaneScene(t, s0, lane, hold)
					defer func() {
						select {
						case <-release:
						default:
							close(release)
						}
					}()
					s, err := sc.raw.NewStream(t.Context(), sc.bID, sc.proto)
					if err != nil {
						t.Fatal(err)
					}
					go func() { _, _ = s.Read(make([]byte, 1)) }() // negotiates the protocol
					<-held
					within(t, time.Second, func() bool { return sc.places() == 1 }, "the accepting stream holds no place")
					closed := sc.end(kind, s)
					time.Sleep(50 * time.Millisecond) // the end reaches b
					close(release)
					sc.freed(t, kind, 0, time.Second, closed)
					if got := sc.msgs(); len(got) > 0 {
						t.Errorf("%s: an accepting stream delivered %d frames", pu1cKept, len(got))
					}
					if kind == "reset" || kind == "fin" {
						sc.noSessionChange(t, "an accepting stream's end")
					}
				})
			})
		}
		// P-11: a waiting stream with nothing unread sees its end at once; it leaves within 1 s.
		for _, kind := range endKinds {
			s0 := next()
			t.Run(fmt.Sprintf("matrix/%s/waiting_empty/%s", lane, kind), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, lane)
					head := sc.open(t)
					defer func() { _ = head.Reset() }()
					s := sc.open(t)
					within(t, time.Second, func() bool { return sc.places() == 2 }, "two streams do not hold two places")
					closed := sc.end(kind, s)
					sc.freed(t, kind, 1, time.Second, closed)
				})
			})
		}
		// P-11: a waiting stream with 64 KiB unread: a reset is seen within 250 ms of its arrival (it wins over the
		// buffered data); an end of writing behind the bytes keeps the stream to its turn, then every frame is
		// delivered; a lost connection or b's close frees it within 1 s.
		for _, kind := range endKinds {
			s0 := next()
			t.Run(fmt.Sprintf("matrix/%s/waiting_bytes/%s", lane, kind), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, lane)
					head := sc.open(t)
					defer func() { _ = head.Reset() }()
					s := sc.open(t)
					want := numbered(16, 4<<10)
					frames(t, s, want...)
					within(t, time.Second, func() bool { return sc.b.ProbedBytes(sc.peer()) > 0 }, pu1cBlind+": the waiting stream was not probed")
					closed := sc.end(kind, s)
					switch kind {
					case "reset":
						if took, ok := firstTrue(time.Second, func() bool { return sc.places() == 1 }); !ok || took > probeEvery+oneWay {
							t.Fatalf("%s: a reset behind 64 KiB unread seen after %v (ok %t), want ≤ %v of its arrival", pu1cBlind, took-oneWay, ok, probeEvery)
						}
					case "fin":
						time.Sleep(2 * time.Second)
						if got := sc.places(); got != 2 {
							t.Fatalf("%s: a stream ended behind unread bytes left before its turn (%d places)", pu1cLost, got)
						}
						_ = head.Reset()
						within(t, time.Second, func() bool { return len(sc.msgs()) == len(want) && sc.places() == 0 },
							pu1cLost+": the frames of a stream ended behind unread bytes were not all delivered at its turn")
						inOrder(t, sc.msgs(), want, "waiting_bytes/fin")
					default:
						sc.freed(t, kind, 1, time.Second, closed)
					}
				})
			})
		}
		// P-11: kept — one whole frame and 3 bytes of the next, then FIN, all probed while waiting.
		for _, kind := range []string{"at_turn", "conn_lost", "node_close", "reorder_wait"} {
			s0 := next()
			t.Run(fmt.Sprintf("matrix/%s/kept/%s", lane, kind), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, lane)
					head := sc.open(t)
					defer func() { _ = head.Reset() }()
					s := sc.open(t)
					arrived := time.Now()
					if _, err := s.Write([]byte{0, 0, 0, 1, 'k', 0, 0, 0}); err != nil {
						t.Fatal(err)
					}
					_ = s.CloseWrite()
					within(t, 3*time.Second, func() bool { return sc.b.ProbedBytes(sc.peer()) == 8 }, pu1cBlind+": the 8 bytes were not all probed")
					time.Sleep(time.Second)
					if got := sc.places(); got != 2 {
						t.Fatalf("%s: a kept stream left before its turn (%d places)", pu1cLost, got)
					}
					switch kind {
					case "at_turn":
						_ = head.Reset()
						within(t, time.Second, func() bool { return len(sc.msgs()) == 1 && sc.places() == 0 },
							pu1cLost+": the kept stream's whole frame was not delivered at its turn, or it kept its place")
						inOrder(t, sc.msgs(), [][]byte{{'k'}}, "kept/at_turn")
						sc.noSessionChange(t, "a kept stream's truncated tail")
					case "conn_lost":
						sc.end(kind, s)
						within(t, upWait+time.Second, func() bool { return sc.places() == 0 }, pu1cKept+": the kept stream outlived its connection")
						if got := sc.msgs(); len(got) > 0 {
							t.Fatalf("%s: a kept stream delivered %d frames after PeerDown", pu1cLost, len(got))
						}
					case "node_close":
						sc.freed(t, kind, 0, time.Second, sc.end(kind, s))
					case "reorder_wait":
						time.Sleep(time.Until(arrived.Add(reorderWaitPU - time.Second)))
						if got := sc.msgs(); len(got) > 0 {
							t.Fatalf("%s: the kept stream delivered before its reorder wait, behind a head held open", pu1cOrder)
						}
						if took, ok := firstTrue(2*time.Second, func() bool { return len(sc.msgs()) == 1 }); !ok || took > time.Second+probeEvery {
							t.Fatalf("%s: the kept stream's frame not delivered at its reorder wait (ok %t, %v late)", pu1cLate, ok, took-time.Second)
						}
					}
				})
			})
		}
		// P-10: a delivering stream ends: freed within 1 s, the frames delivered before stay delivered.
		for _, kind := range endKinds {
			s0 := next()
			t.Run(fmt.Sprintf("matrix/%s/delivering/%s", lane, kind), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, lane)
					s := sc.open(t)
					want := numbered(3, 64)
					frames(t, s, want...)
					within(t, time.Second, func() bool { return len(sc.msgs()) == 3 }, pu1cLost+": the head did not deliver")
					closed := sc.end(kind, s)
					sc.freed(t, kind, 0, time.Second, closed)
					inOrder(t, sc.msgs(), want, "delivering/"+kind)
				})
			})
		}
		// P-12: blocked on the lane's budget (its consumer stalled).
		for _, kind := range endKinds {
			s0 := next()
			t.Run(fmt.Sprintf("matrix/%s/blocked/%s", lane, kind), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, lane)
					base := sc.b.BufferedBytes(lane)
					if !sc.b.StallLane(lane) {
						t.Fatalf("%s: the lane's budget is not free (%d bytes taken)", pu1cBudget, base)
					}
					stalled := sc.b.BufferedBytes(lane)
					resumed := false
					defer func() {
						if !resumed && kind != "node_close" {
							sc.b.ResumeLane(lane)
						}
					}()
					s := sc.open(t)
					want := numbered(3, 1<<10)
					frames(t, s, want[0])
					time.Sleep(time.Second)
					if len(sc.msgs()) != 0 || sc.places() != 1 {
						t.Fatalf("not blocked: %d frames, %d places", len(sc.msgs()), sc.places())
					}
					switch kind {
					case "reset":
						_ = s.Reset()
						time.Sleep(2 * time.Second)
						if got := sc.places(); got != 1 {
							t.Logf("the reset of a blocked stream was seen before room (%d places)", got)
						}
						sc.b.ResumeLane(lane)
						resumed = true
						within(t, time.Second, func() bool { return sc.places() == 0 }, pu1cBlocked+": the blocked stream reset was not freed once room came")
						within(t, time.Second, func() bool { return sc.b.BufferedBytes(lane) == base }, pu1cBudget+": a reservation was not returned")
					case "fin":
						frames(t, s, want[1:]...)
						_ = s.CloseWrite()
						sc.b.ResumeLane(lane)
						resumed = true
						within(t, time.Second, func() bool { return len(sc.msgs()) == 3 && sc.places() == 0 }, pu1cLost+": the rest was not delivered after room")
						inOrder(t, sc.msgs(), want, "blocked/fin")
					case "conn_lost":
						sc.end(kind, s)
						within(t, time.Second, func() bool { return sc.places() == 0 }, pu1cBlocked+": a reader blocked on the budget outlived its connection")
						if got := sc.b.BufferedBytes(lane); got != stalled {
							t.Fatalf("%s: %d bytes of the budget taken after the loss, want %d (the stall's)", pu1cBudget, got, stalled)
						}
					case "node_close":
						closed := sc.end(kind, s)
						select {
						case <-closed:
						case <-time.After(time.Second):
							t.Fatalf("%s: b's Close did not return within 1 s with a reader blocked on the budget", pu1cBlocked)
						}
					}
				})
			})
		}
	}

	// P-11: each lane: 10 MiB pushed into a waiting stream for 60 s: at most 256 bytes probed, the lane's budget
	// untouched, and the opener's writes held by flow control without failing.
	t.Run("probe_bound", func(t *testing.T) {
		for _, ln := range lanes {
			s0 := next()
			t.Run(ln.name, func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, ln.name)
					head := sc.open(t)
					defer func() { _ = head.Reset() }()
					s := sc.open(t)
					base := sc.b.BufferedBytes(ln.name)
					werr := make(chan error, 1)
					go func() {
						for _, p := range numbered(160, 64<<10) {
							if err := frame.Write(s, p); err != nil {
								werr <- err
								return
							}
						}
						werr <- nil
					}()
					for end := time.Now().Add(reorderWaitPU - 2*time.Second); time.Now().Before(end); time.Sleep(probeEvery) {
						if got := sc.b.ProbedBytes(sc.peer()); got > probeMax {
							t.Fatalf("%s: %d bytes probed, more than %d", pu1cBlind, got, probeMax)
						}
						if got := sc.b.BufferedBytes(ln.name); got != base {
							t.Fatalf("%s: a waiting stream holds %d bytes of the budget", pu1cBudget, got-base)
						}
					}
					select {
					case err := <-werr:
						t.Fatalf("the opener's write ended before 60 s: %v", err)
					default:
					}
					_ = s.Reset()
				})
			})
		}
	})

	// P-11: a reset behind unread bytes, at seven phases of the probe period, is seen within 250 ms each time.
	t.Run("probe_period", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "sync")
			head := sc.open(t)
			defer func() { _ = head.Reset() }()
			for k := range 7 {
				s := sc.open(t)
				frames(t, s, numbered(4, 1<<10)...)
				// A byte probed is the start of an attempt (the data is there): the reset comes k/7 of a period later.
				within(t, time.Second, func() bool { return sc.b.ProbedBytes(sc.peer()) > 0 }, pu1cBlind+": not probed")
				time.Sleep(time.Duration(k) * probeEvery / 7)
				_ = s.Reset()
				if took, ok := firstTrue(time.Second, func() bool { return sc.places() == 1 }); !ok || took > probeEvery+oneWay {
					t.Fatalf("%s: at phase %d/7 the reset was seen %v after its arrival (ok %t), want ≤ %v", pu1cBlind, k, took-oneWay, ok, probeEvery)
				}
			}
		})
	})

	// P-11 (Codex): a read that returns a byte and an error keeps the byte, in order, before the error is handled.
	t.Run("probe_bytes_before_error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "sync")
			const proto = "/inkwell-test/scripted/1"
			sc.b.Host().SetStreamHandler(proto, func(s network.Stream) {
				deadline := os.ErrDeadlineExceeded
				sc.b.ServeSync(&scriptedStream{Stream: s, reads: []scriptRead{
					{0, deadline}, {0, nil}, {0, deadline}, {1, nil}, {'x', io.EOF},
				}})
			})
			head := sc.open(t)
			s, err := openAckedOn(t, sc.raw, sc.bID, proto)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Reset() }()
			within(t, 3*time.Second, func() bool { return sc.b.ProbedBytes(sc.peer()) == 5 }, pu1cLost+": the bytes returned with an error were not kept")
			_ = head.Reset()
			within(t, time.Second, func() bool { return len(sc.msgs()) == 1 }, pu1cLost+": the frame whose bytes came with errors was not delivered")
			inOrder(t, sc.msgs(), [][]byte{{'x'}}, "probe_bytes_before_error")
		})
	})

	// P-10: a turn and a PeerUp are seen within 250 ms.
	t.Run("wake_within_bound", func(t *testing.T) {
		t.Run("turn", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				sc := newLaneScene(t, next(), "sync")
				head := sc.open(t)
				s := sc.open(t)
				defer func() { _ = s.Reset() }()
				frames(t, s, []byte("next"))
				time.Sleep(time.Second)
				_ = head.Reset()
				if took, ok := firstTrue(time.Second, func() bool { return len(sc.msgs()) == 1 }); !ok || took > probeEvery+oneWay {
					t.Fatalf("%s: the turn was seen %v after the head's reset reached b (ok %t), want ≤ %v", pu1cLate, took-oneWay, ok, probeEvery)
				}
			})
		})
		t.Run("peer_up", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				nn := newNatNet(next())
				nn.setJitter(1)
				b := nn.nodeAt(t, 2, true, p2p.Options{})
				raw := nn.rawHost(t, 3, 13)
				release := make(chan struct{})
				raw.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
					<-release
					_, _ = s.Write([]byte{ackByte})
					_, _ = io.Copy(io.Discard, s)
				})
				defer func() {
					select {
					case <-release:
					default:
						close(release)
					}
				}()
				rb := record(t, b)
				bID := libp2pID(t, b)
				if err := raw.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()}); err != nil {
					t.Fatal(err)
				}
				s, err := openAckedOn(t, raw, bID, p2p.ProtocolSync)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = s.Reset() }()
				frames(t, s, []byte("first"))
				time.Sleep(time.Second)
				if len(rb.of(transport.Message, nameOf(raw))) != 0 {
					t.Fatal("a frame delivered before the peer was announced")
				}
				close(release)
				within(t, 5*time.Second, func() bool { return len(rb.of(transport.Message, nameOf(raw))) == 1 }, pu1cLate+": no frame after PeerUp")
				up, msg := rb.of(transport.PeerUp, nameOf(raw)), rb.of(transport.Message, nameOf(raw))
				if len(up) != 1 || msg[0].at.Sub(up[0].at) > probeEvery {
					t.Fatalf("%s: the PeerUp was seen %v after it (%d PeerUp), want ≤ %v", pu1cLate, msg[0].at.Sub(up[0].at), len(up), probeEvery)
				}
			})
		})
	})

	// P-10 (Codex): the head leaves, or PeerUp arrives, exactly at k × 250 ms of the successor's wait, k = 1..8: the
	// successor's frames intact, in order (no deadline left armed into its delivery).
	t.Run("interrupt_during_transition", func(t *testing.T) {
		for k := 1; k <= 8; k++ {
			s0 := next()
			t.Run(fmt.Sprintf("head/%d", k), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, s0, "sync")
					head := sc.open(t)
					s := sc.open(t)
					defer func() { _ = s.Reset() }()
					acked := time.Now()
					want := numbered(40, 8<<10)
					go func() {
						for _, p := range want {
							if frame.Write(s, p) != nil {
								return
							}
							select {
							case <-time.After(20 * time.Millisecond):
							case <-t.Context().Done():
								return
							}
						}
					}()
					time.Sleep(time.Until(acked.Add(time.Duration(k)*probeEvery - oneWay)))
					_ = head.Reset()
					within(t, 5*time.Second, func() bool { return len(sc.msgs()) == len(want) }, pu1cLost+": the successor's frames were cut")
					inOrder(t, sc.msgs(), want, fmt.Sprintf("head leaves at %d × 250 ms", k))
				})
			})
		}
	})

	// The same with PeerUp: the raw opener holds b's own sync stream unacknowledged until k × 250 ms after b
	// acknowledged the raw opener's stream, which is first in its order.
	t.Run("interrupt_during_transition", func(t *testing.T) {
		for k := 1; k <= 8; k++ {
			s0 := next()
			t.Run(fmt.Sprintf("up/%d", k), func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					nn := newNatNet(s0)
					nn.setJitter(1)
					b := nn.nodeAt(t, 2, true, p2p.Options{})
					raw := nn.rawHost(t, 3, 13)
					release := make(chan struct{})
					raw.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
						<-release
						_, _ = s.Write([]byte{ackByte})
						_, _ = io.Copy(io.Discard, s)
					})
					defer func() {
						select {
						case <-release:
						default:
							close(release)
						}
					}()
					rb := record(t, b)
					bID := libp2pID(t, b)
					if err := raw.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()}); err != nil {
						t.Fatal(err)
					}
					s, err := openAckedOn(t, raw, bID, p2p.ProtocolSync)
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = s.Reset() }()
					acked := time.Now()
					want := numbered(40, 8<<10)
					go func() {
						for _, p := range want {
							if frame.Write(s, p) != nil {
								return
							}
							select {
							case <-time.After(20 * time.Millisecond):
							case <-t.Context().Done():
								return
							}
						}
					}()
					// b's PeerUp follows the ACK it reads one network trip after the raw opener writes it.
					time.Sleep(time.Until(acked.Add(time.Duration(k)*probeEvery - 2*oneWay)))
					close(release)
					within(t, 5*time.Second, func() bool { return len(rb.of(transport.Message, nameOf(raw))) == len(want) }, pu1cLost+": the frames were cut")
					inOrder(t, rb.of(transport.Message, nameOf(raw)), want, fmt.Sprintf("PeerUp at %d × 250 ms", k))
				})
			})
		}
	})

	// P-10: a stream that arrives first with its peer announced delivers its first frame with no added delay.
	t.Run("immediate_when_first_and_up", func(t *testing.T) {
		for _, ln := range lanes {
			t.Run(ln.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, next(), ln.name)
					s := sc.open(t)
					defer func() { _ = s.Reset() }()
					sent := time.Now()
					frames(t, s, []byte("now"))
					within(t, time.Second, func() bool { return len(sc.msgs()) == 1 }, pu1cLate+": nothing delivered")
					if d := sc.msgs()[0].at.Sub(sent); d > oneWay {
						t.Fatalf("%s: the first frame of a head already up was delivered %v after it reached b, want 0", pu1cLate, d-oneWay)
					}
				})
			})
		}
	})

	// P-11 (Codex): two whole frames, then a truncated third and FIN, all while waiting: the two are delivered at
	// its turn, the stream leaves, no ClosePeer.
	t.Run("valid_prefix_then_truncated", func(t *testing.T) {
		for _, ln := range lanes {
			t.Run(ln.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, next(), ln.name)
					head := sc.open(t)
					s := sc.open(t)
					if _, err := s.Write([]byte{0, 0, 0, 1, 'a', 0, 0, 0, 1, 'b', 0, 0, 0, 5, 'c'}); err != nil {
						t.Fatal(err)
					}
					_ = s.CloseWrite()
					time.Sleep(5 * time.Second)
					_ = head.Reset()
					within(t, time.Second, func() bool { return len(sc.msgs()) == 2 && sc.places() == 0 },
						pu1cLost+": the whole frames before a truncated tail were not delivered, or the stream kept its place")
					inOrder(t, sc.msgs(), [][]byte{{'a'}, {'b'}}, "valid_prefix_then_truncated")
					sc.noSessionChange(t, "a truncated tail")
				})
			})
		}
	})

	// P-13: on QUIC, an honest peer's opener fills a waiting sync stream, a waiting and a blocked bulk stream to their
	// window before the head writes: the head still delivers 4 MiB in 10 s; with quic-go's default windows it does
	// not (the witness: the arm discriminates).
	credit := func(t *testing.T, seed uint64, configure ...func(*p2p.Options)) int {
		sc := newLaneScene(t, seed, "sync", configure...)
		if !sc.b.StallLane("bulk") {
			t.Fatalf("%s: the bulk budget is not free", pu1cBudget)
		}
		defer sc.b.ResumeLane("bulk")
		head := sc.open(t)
		defer func() { _ = head.Reset() }()
		fill := func(s network.Stream) {
			go func() {
				for _, p := range numbered(16, 64<<10) {
					if frame.Write(s, p) != nil {
						return
					}
				}
			}()
		}
		fill(sc.open(t))
		bh, err := openAckedOn(t, sc.raw, sc.bID, p2p.ProtocolBulk)
		if err != nil {
			t.Fatal(err)
		}
		fill(bh)
		bw, err := openAckedOn(t, sc.raw, sc.bID, p2p.ProtocolBulk)
		if err != nil {
			t.Fatal(err)
		}
		fill(bw)
		time.Sleep(2 * time.Second)
		go func() {
			for _, p := range numbered(64, 64<<10) {
				if frame.Write(head, p) != nil {
					return
				}
			}
		}()
		time.Sleep(10 * time.Second)
		total := 0
		for _, m := range sc.msgs() {
			total += len(m.Payload)
		}
		return total
	}
	t.Run("credit_exhausted_before_head", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			if got := credit(t, next()); got < 4<<20 {
				t.Fatalf("%s: the head delivered %d bytes in 10 s while the other streams of both lanes held their windows, want 4 MiB", pu1cStarved, got)
			}
		})
	})
	t.Run("credit_witness_default_window", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			got := credit(t, next(), func(o *p2p.Options) { p2p.SetQUICWindows(o, 512<<10, 768<<10) })
			if got >= 1<<20 {
				t.Fatalf("the witness does not discriminate: with quic-go's default windows the head delivered %d bytes", got)
			}
		})
	})

	// P-13: every QUIC connection's resource scope holds the initial connection window from its start and returns it
	// at close; with less than that left in the system scope, the connection is refused.
	t.Run("quic_window_reserved", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(next())
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			bID := libp2pID(t, b)
			rm := b.Host().Network().ResourceManager()
			system := func() int64 {
				var m int64
				_ = rm.ViewSystem(func(s network.ResourceScope) error { m = s.Stat().Memory; return nil })
				return m
			}
			m0 := system()
			for i, dir := range []string{"inbound", "outbound"} {
				raw := nn.rawHost(t, 3+byte(i), 13+byte(i))
				var err error
				if dir == "inbound" {
					err = raw.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()})
				} else {
					// Use the explicit connection path so H-19 retains this outbound connection.
					err = b.Connect(t.Context(), nameOf(raw), addrStrings(raw))
				}
				if err != nil {
					t.Fatalf("%s connection: %v", dir, err)
				}
				conns := b.Host().Network().ConnsToPeer(raw.ID())
				if len(conns) == 0 {
					t.Fatalf("no %s connection", dir)
				}
				for _, c := range conns {
					if got := c.Scope().Stat().Memory; got < connWindow0 {
						t.Fatalf("%s: the %s QUIC connection's scope holds %d bytes, want ≥ %d", pu1cMem, dir, got, connWindow0)
					}
				}
				_ = b.Host().Network().ClosePeer(raw.ID())
				within(t, time.Second, func() bool { return system() == m0 }, pu1cMem+": the initial window was not returned at close")
			}
			// Less than the window left: the connection is refused.
			const memoryLimit = 256 << 20 // the node's (§16.1)
			fill := int(memoryLimit - m0 - connWindow0 + 1)
			if err := rm.ViewSystem(func(s network.ResourceScope) error { return s.ReserveMemory(fill, network.ReservationPriorityAlways) }); err != nil {
				t.Fatalf("reserve all but less than the window: %v", err)
			}
			defer func() {
				_ = rm.ViewSystem(func(s network.ResourceScope) error { s.ReleaseMemory(fill); return nil })
			}()
			fresh := nn.rawHost(t, 5, 15) // No existing connection can satisfy Connect during the refusal check.
			// QUIC ConnectionRefused here comes from ConnContext, before the handshake can advertise credit.
			err := fresh.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()})
			if refused, ok := errors.AsType[*quic.TransportError](err); !ok || refused.ErrorCode != quic.ConnectionRefused {
				t.Fatalf("%s: initial window must be refused before the inbound handshake, got %v", pu1cMem, err)
			}
			if got := len(b.Host().Network().ConnsToPeer(fresh.ID())); got != 0 {
				t.Fatalf("%s: %d inbound connections accepted with less than their initial window left in the system scope", pu1cMem, got)
			}
			if got := system(); got != m0+int64(fill) {
				t.Fatalf("%s: refused inbound connection changed memory to %d, want %d", pu1cMem, got, m0+int64(fill))
			}
			err = b.Connect(t.Context(), nameOf(fresh), addrStrings(fresh))
			if !errors.Is(err, network.ErrResourceLimitExceeded) {
				t.Fatalf("%s: outbound initial window must be refused by the resource manager, got %v", pu1cMem, err)
			}
		})
	})

	// P-13: a raw opener writes 128 KiB into each of eight streams refused for capacity: their credit comes back
	// once their resets are processed, and the head on the connection still delivers.
	t.Run("refused_stream_bytes_released", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "sync")
			head := sc.open(t)
			defer func() { _ = head.Reset() }()
			waiting := sc.open(t)
			defer func() { _ = waiting.Reset() }()
			for i := range 8 {
				s, err := sc.raw.NewStream(t.Context(), sc.bID, p2p.ProtocolSync)
				if err != nil {
					t.Fatal(err)
				}
				go func() { _, _ = s.Write(make([]byte, streamWindow0)) }()
				within(t, time.Second, func() bool { return sc.b.BusyRefusals() == uint64(i+1) }, "the third stream was not refused for capacity")
			}
			time.Sleep(time.Second)
			want := numbered(32, 64<<10)
			go func() { frames(t, head, want...) }()
			within(t, 10*time.Second, func() bool { return len(sc.msgs()) == len(want) },
				pu1cStarved+": the head starved after streams refused with data in flight")
		})
	})

	// P-10: each placed waiting stream adds at most one goroutine (the handler's).
	t.Run("one_goroutine_per_place", func(t *testing.T) {
		for _, ln := range lanes {
			t.Run(ln.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, next(), ln.name)
					head := sc.open(t)
					defer func() { _ = head.Reset() }()
					time.Sleep(time.Second)
					before := runtime.NumGoroutine()
					s := sc.open(t)
					defer func() { _ = s.Reset() }()
					frames(t, s, []byte("w"))
					time.Sleep(time.Second)
					if after := runtime.NumGoroutine(); after > before+1 {
						t.Fatalf("%s: a waiting stream added %d goroutines", pu1cKept, after-before)
					}
				})
			})
		}
	})

	// P-12: two connections to one peer: the loss of the other leaves a reader blocked on the budget, the loss of its
	// own frees it.
	t.Run("blocked_exact_connection", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newMoveScene(t, next(), func(*p2p.Options) {})
			for deadline := time.Now().Add(2 * time.Minute); ; time.Sleep(50 * time.Millisecond) {
				if direct, ok := sc.a.OutDirect(sc.b.Local()); ok && direct {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("a's session never moved to the direct connection")
				}
				sc.send(t)
			}
			sc.check(t)
			places := func() int { return sc.b.InboundPlaces("sync", sc.a.Local()) }
			within(t, 5*time.Second, func() bool { return places() == 1 }, "the moved session holds more than one place")
			if !sc.b.StallLane("sync") {
				t.Fatalf("%s: the sync budget is not free", pu1cBudget)
			}
			defer sc.b.ResumeLane("sync")
			sc.send(t)
			time.Sleep(time.Second)
			aID := libp2pID(t, sc.a)
			var own, other []network.Conn
			for _, c := range sc.b.Host().Network().ConnsToPeer(aID) {
				if isDirect(c) {
					own = append(own, c)
				} else {
					other = append(other, c)
				}
			}
			if len(own) == 0 || len(other) == 0 {
				t.Fatalf("want a direct and a relayed connection, got %d and %d", len(own), len(other))
			}
			for _, c := range other {
				_ = c.Close()
			}
			time.Sleep(2 * time.Second)
			if got := places(); got != 1 {
				t.Fatalf("%s: the loss of another connection freed the blocked reader (%d places)", pu1cBlocked, got)
			}
			for _, c := range own {
				_ = c.Close()
			}
			if _, ok := firstTrue(time.Second, func() bool { return places() == 0 }); !ok {
				t.Fatalf("%s: the reader blocked on the budget was not freed within 1 s of its own connection's loss", pu1cBlocked)
			}
		})
	})

	// P-8 to P-12 against a Byzantine opener: each lane, 50 open-reset loops and data floods into waiting streams:
	// never more than two places, goroutines back to their count, the budget untouched.
	t.Run("byzantine_floods", func(t *testing.T) {
		for _, ln := range lanes {
			t.Run(ln.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					sc := newLaneScene(t, next(), ln.name)
					head := sc.open(t)
					defer func() { _ = head.Reset() }()
					time.Sleep(time.Second)
					before, base := runtime.NumGoroutine(), sc.b.BufferedBytes(ln.name)
					for i := range 50 {
						s, err := openAckedOn(t, sc.raw, sc.bID, sc.proto)
						for isBusy(err) {
							time.Sleep(50 * time.Millisecond)
							s, err = openAckedOn(t, sc.raw, sc.bID, sc.proto)
						}
						if err != nil {
							t.Fatalf("stream %d: %v", i, err)
						}
						if i%2 == 1 {
							go func() { _, _ = s.Write(make([]byte, 96<<10)) }()
							time.Sleep(10 * time.Millisecond)
						}
						_ = s.Reset()
						if got := sc.places(); got > 2 {
							t.Fatalf("%s: %d places after %d streams", pu1cOver, got, i+1)
						}
					}
					within(t, time.Second, func() bool { return sc.places() == 1 }, pu1cKept+": the flooded streams kept places")
					time.Sleep(time.Second)
					if after := runtime.NumGoroutine(); after > before+8 {
						t.Errorf("%s: %d goroutines after the floods, %d before", pu1cKept, after, before)
					}
					if got := sc.b.BufferedBytes(ln.name); got != base {
						t.Errorf("%s: %d bytes of the budget taken after the floods", pu1cBudget, got-base)
					}
				})
			})
		}
	})

	// P-14: the bulk opener reads the ACK before its first frame.
	t.Run("bulk_ack", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "bulk")
			early := make(chan error, 1)
			got := make(chan []byte, 1)
			sc.raw.SetStreamHandler(p2p.ProtocolBulk, func(s network.Stream) {
				_ = s.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := s.Read(make([]byte, 1))
				if n > 0 || !isTimeout(err) {
					early <- fmt.Errorf("the opener wrote before the ACK (%d bytes): %w", n, err)
					_ = s.Reset()
					return
				}
				early <- nil
				_ = s.SetReadDeadline(time.Time{})
				_, _ = s.Write([]byte{ackByte})
				p, err := frame.Read(s, transport.MaxFrame)
				if err == nil {
					got <- p
				}
				_, _ = io.Copy(io.Discard, s)
			})
			if err := sc.b.Bulk().Send(t.Context(), sc.peer(), []byte("blob")); err != nil {
				t.Fatalf("bulk Send: %v", err)
			}
			if err := <-early; err != nil {
				t.Fatal(err)
			}
			if p := <-got; string(p) != "blob" {
				t.Fatalf("the responder read %q", p)
			}
		})
	})

	// P-6 on bulk (P-14): b's handler of the old bulk stream, held after its ACK, keeps its rank across a move to
	// the direct connection: b's bulk frames keep a's order.
	t.Run("bulk_order_across_late_handler", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			release := make(chan struct{})
			var held atomic.Int32
			sc := newMoveScene(t, next(), func(o *p2p.Options) {
				p2p.SetAfterAckLane(o, func(lane string, _ network.Conn) {
					if lane == "bulk" && held.Add(1) == 1 {
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
			ab, bb := sc.a.Bulk(), sc.b.Bulk()
			rbb := record(t, bb)
			var want [][]byte
			send := func() {
				p := binary.BigEndian.AppendUint32(nil, uint32(len(want)))
				if err := ab.Send(t.Context(), sc.b.Local(), p); err != nil {
					t.Fatalf("bulk Send: %v", err)
				}
				want = append(want, p)
			}
			send() // on the circuit, its handler held
			for deadline := time.Now().Add(2 * time.Minute); ; time.Sleep(50 * time.Millisecond) {
				if direct, ok := sc.a.OutDirect(sc.b.Local()); ok && direct {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("no direct connection")
				}
				sc.send(t)
			}
			for range 5 {
				send() // on the direct connection: a new bulk stream
			}
			time.Sleep(time.Second)
			if got := rbb.of(transport.Message, sc.a.Local()); len(got) > 0 {
				t.Fatalf("%s: the new bulk stream delivered before the held old one: %v", pu1cOrder, got)
			}
			close(release)
			within(t, 5*time.Second, func() bool { return len(rbb.of(transport.Message, sc.a.Local())) == len(want) }, pu1cLost+": bulk frames lost across the move")
			inOrder(t, rbb.of(transport.Message, sc.a.Local()), want, "bulk across a move")
		})
	})

	// P-14: a third bulk stream is reset with 0x1002; the Node opener refused for capacity retries and its Send
	// succeeds; no ClosePeer, no event.
	t.Run("bulk_capacity", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "bulk")
			s1, s2 := sc.open(t), sc.open(t)
			defer func() { _ = s1.Reset() }()
			defer func() { _ = s2.Reset() }()
			if _, err := openAckedOn(t, sc.raw, sc.bID, p2p.ProtocolBulk); !isBusy(err) {
				t.Fatalf("%s: a third bulk stream: %v, want a reset with code 0x1002 (places %d)", pu1cOver, err, sc.places())
			}
			if got := sc.places(); got != 2 {
				t.Fatalf("%s: %d bulk places", pu1cOver, got)
			}
			var refused atomic.Int32
			got := make(chan []byte, 1)
			sc.raw.SetStreamHandler(p2p.ProtocolBulk, func(s network.Stream) {
				if refused.Add(1) <= 2 {
					_ = s.ResetWithError(busyCode)
					return
				}
				_, _ = s.Write([]byte{ackByte})
				if p, err := frame.Read(s, transport.MaxFrame); err == nil {
					got <- p
				}
				_, _ = io.Copy(io.Discard, s)
			})
			retries := sc.b.BusyRetries()
			if err := sc.b.Bulk().Send(t.Context(), sc.peer(), []byte("after")); err != nil {
				t.Fatalf("%s: a bulk Send refused twice for capacity failed: %v", pu1cLost, err)
			}
			if p := <-got; string(p) != "after" {
				t.Fatalf("the responder read %q", p)
			}
			if r := sc.b.BusyRetries() - retries; r != 2 {
				t.Errorf("BusyRetries grew by %d, want 2", r)
			}
			sc.noSessionChange(t, "bulk refusals for capacity")
		})
	})

	// P-9 on bulk (P-14): a waiting bulk stream that ends without a byte frees its place at once; the one behind a
	// head that leaves without a frame reads at once.
	t.Run("bulk_removal_while_waiting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "bulk")
			head := sc.open(t)
			s2 := sc.open(t)
			_ = s2.Reset()
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1cKept+": a waiting bulk stream reset kept its place")
			s3 := sc.open(t)
			_ = s3.CloseWrite()
			within(t, time.Second, func() bool { return sc.places() == 1 }, pu1cKept+": a waiting bulk stream closed without a byte kept its place")
			s4 := sc.open(t)
			defer func() { _ = s4.Reset() }()
			frames(t, s4, []byte("four"))
			time.Sleep(100 * time.Millisecond)
			if len(sc.msgs()) != 0 {
				t.Fatalf("%s: the waiting bulk stream delivered before its turn", pu1cOrder)
			}
			_ = head.Reset()
			within(t, probeEvery+oneWay+10*time.Millisecond, func() bool { return len(sc.msgs()) == 1 }, pu1cLate+": the waiting bulk stream did not read when the head left")
		})
	})

	// P-14: /inkwell/bulk/1.0.0 is neither served nor opened.
	t.Run("old_bulk_refused", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sc := newLaneScene(t, next(), "bulk")
			const old = "/inkwell/bulk/1.0.0"
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if s, err := sc.raw.NewStream(ctx, sc.bID, old); err == nil {
				_ = s.SetReadDeadline(time.Now().Add(time.Second))
				if _, err := s.Read(make([]byte, 1)); err == nil || isTimeout(err) {
					t.Fatalf("b serves %s", old)
				}
			}
			var opened atomic.Bool
			sc.raw.RemoveStreamHandler(p2p.ProtocolBulk)
			sc.raw.SetStreamHandler(old, func(s network.Stream) { opened.Store(true); _ = s.Reset() })
			if err := sc.b.Bulk().Send(t.Context(), sc.peer(), []byte("x")); err == nil {
				t.Fatal("a bulk Send to a peer serving only 1.0.0 succeeded")
			}
			if opened.Load() {
				t.Fatalf("b opened %s", old)
			}
		})
	})
}

// scriptRead is one read of a scriptedStream: one byte and an error.
type scriptRead struct {
	b   byte
	err error
}

// scriptedStream is a real inbound stream whose first reads are scripted: each returns one byte and an error, as
// a transport may (a read that returns n > 0 and an error); then it reads as an ended stream.
type scriptedStream struct {
	network.Stream
	mu    sync.Mutex
	reads []scriptRead
}

func (s *scriptedStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.reads) == 0 || len(p) == 0 {
		return 0, io.EOF
	}
	r := s.reads[0]
	s.reads = s.reads[1:]
	p[0] = r.b
	return 1, r.err
}

// TestInboundTransport checks the transport facts the lifecycle stands on (plan §2), on QUIC and on yamux through
// a relay circuit: a deadline interrupts a pending read, a reset wins over buffered data, data comes before EOF.
func TestInboundTransport(t *testing.T) {
	const proto = "/inkwell-test/transport/1"
	pair := func(t *testing.T, seed uint64, mux string) (network.Stream, <-chan network.Stream) {
		t.Helper()
		var a, b *simNode
		if mux == "quic" {
			nn := newNatNet(seed)
			a, b = nn.nodeAt(t, 1, true, p2p.Options{}), nn.nodeAt(t, 2, true, p2p.Options{})
			if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
				t.Fatal(err)
			}
			expectUp(t, a, b)
		} else {
			nn := newNatNet(seed)
			nn.blockDirect(true)
			relays := nn.relay(t, p2p.RelayOptions{})
			a = nn.node(t, 1, p2p.Options{Relays: relays})
			b = nn.node(t, 2, p2p.Options{Relays: relays})
			a.Want(map[transport.PeerID][]string{b.Local(): nil})
			expectUp(t, a, b)
		}
		in := make(chan network.Stream, 1)
		b.Host().SetStreamHandler(proto, func(s network.Stream) { in <- s; <-t.Context().Done() })
		ctx := network.WithAllowLimitedConn(t.Context(), "test")
		s, err := a.Host().NewStream(ctx, libp2pID(t, b), proto)
		if err != nil {
			t.Fatal(err)
		}
		if mux == "yamux" && !isCircuit(s.Conn()) {
			t.Fatal("the stream does not run through the relay")
		}
		if _, err := s.Write([]byte{1}); err != nil { // the opener's first byte negotiates the protocol
			t.Fatal(err)
		}
		return s, in
	}
	recvFirst := func(t *testing.T, in <-chan network.Stream) network.Stream {
		r := <-in
		if _, err := io.ReadFull(r, make([]byte, 1)); err != nil {
			t.Fatal(err)
		}
		return r
	}
	seed := uint64(300)
	for _, mux := range []string{"quic", "yamux"} {
		seed++
		s0 := seed
		t.Run("deadline_interrupts_pending_read/"+mux, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w, in := pair(t, s0, mux)
				defer func() { _ = w.Reset() }()
				r := recvFirst(t, in)
				done := make(chan error, 1)
				go func() { _, err := r.Read(make([]byte, 1)); done <- err }()
				time.Sleep(100 * time.Millisecond)
				start := time.Now()
				_ = r.SetReadDeadline(time.Now())
				if err := <-done; !isTimeout(err) || time.Since(start) > 0 {
					t.Fatalf("a deadline set on a pending read: %v after %v, want a timeout at once", err, time.Since(start))
				}
			})
		})
		seed++
		s1 := seed
		t.Run("reset_wins_over_buffered_data/"+mux, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w, in := pair(t, s1, mux)
				r := recvFirst(t, in)
				if _, err := w.Write(make([]byte, 1<<10)); err != nil {
					t.Fatal(err)
				}
				time.Sleep(time.Second)
				_ = w.Reset()
				time.Sleep(time.Second)
				if n, err := r.Read(make([]byte, 1)); err == nil || n > 0 {
					t.Fatalf("a read after a reset behind 1 KiB buffered returned %d bytes, %v; want the reset first", n, err)
				}
			})
		})
		seed++
		s2 := seed
		t.Run("data_before_eof/"+mux, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w, in := pair(t, s2, mux)
				defer func() { _ = w.Reset() }()
				r := recvFirst(t, in)
				data := bytes.Repeat([]byte{7}, 1<<10)
				if _, err := w.Write(data); err != nil {
					t.Fatal(err)
				}
				_ = w.CloseWrite()
				got, err := io.ReadAll(r)
				if err != nil || !slices.Equal(got, data) {
					t.Fatalf("read %d bytes, %v; want the 1 KiB, then EOF", len(got), err)
				}
			})
		})
	}
}

// isCircuit reports whether c runs through a relay.
func isCircuit(c network.Conn) bool { return !isDirect(c) }
