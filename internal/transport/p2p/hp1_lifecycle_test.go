package p2p

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/protocol/directup"
)

func TestReopenChoosesLiveConnectionAfterReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		lost := &lifecycleConn{closed: true}
		circuit := &lifecycleConn{addr: ma.StringCast("/p2p-circuit")}
		fresh := &lifecycleConn{}
		nw := &lifecycleNetwork{conns: []network.Conn{lost, circuit}}
		opened := make(chan struct{})
		n := &Node{ctx: ctx, proofs: newProofSet(), chg: make(chan struct{}), h: lifecycleHost{
			openRecordingHost: openRecordingHost{open: func(ctx context.Context) (network.Stream, error) {
				close(opened)
				<-ctx.Done()
				return nil, ctx.Err()
			}},
			nw: nw,
		}}
		defer func() {
			n.mu.Lock()
			n.closed = true
			n.mu.Unlock()
			cancel()
			n.wg.Wait()
		}()
		s := &lifecycleStream{conn: lost, reset: func() {
			circuit.closed = true
			nw.conns = []network.Conn{fresh}
		}}
		ps := &peerState{connected: true, admitted: true, up: true, out: s}

		n.watchOut(ps, s)
		synctest.Wait()
		if circuit.opens != 0 || nw.closes != 0 {
			t.Fatalf("dead circuit opens = %d, peer closes = %d with a fresh direct link", circuit.opens, nw.closes)
		}
		select {
		case <-opened:
		default:
			t.Fatal("reopen did not use the live connection")
		}
	})
}

func TestRetiringSyncStreamReleasesAtOrderDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, aRemote := net.Pipe()
		b, bRemote := net.Pipe()
		old := &lifecycleStream{pipe: a, conn: &lifecycleConn{}}
		next := &lifecycleStream{pipe: b, conn: &lifecycleConn{}}
		ps := &peerState{connected: true, out: old, wsem: make(chan struct{}, 1)}
		n := &Node{ctx: t.Context(), proofs: newProofSet(), chg: make(chan struct{}), h: lifecycleHost{nw: &lifecycleNetwork{}}}
		n.wg.Go(func() { n.watchOut(ps, old) })
		defer func() {
			n.mu.Lock()
			n.closed = true
			n.mu.Unlock()
			_ = a.Close()
			_ = b.Close()
			_ = aRemote.Close()
			_ = bRemote.Close()
			n.wg.Wait()
		}()

		n.swapOut(ps, old, next)
		time.Sleep(reorderWait - time.Nanosecond)
		synctest.Wait()
		n.mu.Lock()
		draining := ps.retiring == old
		n.mu.Unlock()
		if !draining {
			t.Fatal("old stream lost its drain window")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		n.mu.Lock()
		moved := ps.retiring == nil && ps.out == next
		n.mu.Unlock()
		if !moved {
			t.Fatal("silent predecessor still blocks moves at the sync ordering deadline")
		}
	})
}

// H-6, H-14, H-17: a new circuit that comes while the loop driving the upgrader is ending (its member loop
// saw the old circuit close and decided to end; the new circuit's memberUp starts the next loop, and its E7
// reaches the upgrader, before the old loop stops it) is still a new epoch for the next loop: the old call,
// stopped in flight, is stale, and the next loop punches over the new circuit at once, whatever the
// activity. The interleaving is the one the locks allow: the old loop's end (memberLoop) and its stop
// (undrive, upgradeStop) are separate sections, and memberUp, upgraderLocked and upEvent run between them.
func TestUpgradeNewCircuitWhileLoopStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const p = peer.ID("peer")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		oldCircuit := &upConn{lifecycleConn{addr: ma.StringCast("/p2p-circuit")}}
		nw := &lifecycleNetwork{conns: []network.Conn{oldCircuit}}
		hold := make(chan struct{})
		var calls []directup.Action
		n := &Node{
			proofs: newProofSet(),
			ctx:    ctx, h: lifecycleHost{nw: nw}, born: time.Now(),
			ups:   map[peer.ID]*upgrader{},
			peers: map[peer.ID]*peerState{p: {out: &lifecycleStream{conn: oldCircuit}}},
			pgate: newPunchGate(func(peer.ID) bool { return true }, time.Now()),
		}
		n.opts.localReady = func() bool { return true }
		n.opts.onCall = func(_ peer.ID, a directup.Action) { calls = append(calls, a) }
		n.opts.punch = func(ctx context.Context, _ peer.ID) error {
			<-ctx.Done()
			if len(calls) == 1 {
				<-hold // the first call, cancelled, returns late (libp2p tears its exchange down)
			}
			return ctx.Err()
		}
		n.lastActivity.Store(1) // the person last typed when the node started

		// Loop A drives the upgrader and starts a punch over the old circuit.
		ctxA, endA := context.WithCancel(ctx)
		n.mu.Lock()
		wA := &wanted{id: p, up: n.upgraderLocked(p)}
		n.mu.Unlock()
		if !n.drive(ctxA, wA) {
			t.Fatal("loop A did not get the upgrader")
		}
		n.upgradeStep(ctxA, wA)
		if len(calls) != 1 {
			t.Fatalf("H-6 setup: %d calls, want one in flight", len(calls))
		}

		// Loop A has decided to end. The new circuit comes: memberUp starts loop B, and E7 is recorded.
		time.Sleep(directup.ActiveWindow + time.Minute) // the app goes idle meanwhile
		endA()
		newCircuit := &upConn{lifecycleConn{addr: ma.StringCast("/p2p-circuit")}}
		nw.conns = []network.Conn{newCircuit}
		n.peers[p].out = &lifecycleStream{conn: newCircuit}
		ctxB, endB := context.WithCancel(ctx)
		defer endB()
		n.mu.Lock()
		wB := &wanted{id: p, up: n.upgraderLocked(p)}
		n.mu.Unlock()
		owned := make(chan bool, 1)
		go func() { owned <- n.drive(ctxB, wB) }()
		n.upEvent(p, directup.E7)

		// Loop A stops the upgrader; loop B takes it and steps while the old call is still returning.
		n.undrive(wA, true)
		if !<-owned {
			t.Fatal("loop B did not get the upgrader")
		}
		n.upgradeStep(ctxB, wB)
		close(hold)
		synctest.Wait() // the old call's result is in
		time.Sleep(time.Second)
		n.upgradeStep(ctxB, wB)
		if len(calls) != 2 {
			t.Fatalf("H-6 no punch over the new circuit after the old call returned (calls %v): the stopped "+
				"call's result was applied to the new circuit's epoch, a ladder call the idle app holds back (H-17)", calls)
		}
		cancel() // ends the second call
		synctest.Wait()
		n.undrive(wB, true)
	})
}

// upConn is a circuit the peer dialled to the node: the side that punches (H-3).
type upConn struct{ lifecycleConn }

func (c *upConn) Stat() network.ConnStats {
	return network.ConnStats{Direction: network.DirInbound}
}

func TestNoteActivityLocksOnlyOnIdleTransition(t *testing.T) {
	n := &Node{born: time.Now()}
	n.NoteActivity()
	n.mu.Lock()
	started, noted := make(chan struct{}), make(chan struct{})
	go func() {
		close(started)
		n.NoteActivity()
		close(noted)
	}()
	<-started
	select {
	case <-noted:
	case <-time.After(time.Second):
		t.Error("active input waited for the node mutex")
	}
	n.mu.Unlock()
	<-noted

	synctest.Test(t, func(t *testing.T) {
		u := newUpgrader(time.Now())
		u.drivers = 1
		n := &Node{born: time.Now(), ups: map[peer.ID]*upgrader{"peer": u}}
		n.NoteActivity()

		time.Sleep(directup.ActiveWindow + time.Nanosecond)
		for range 2 {
			go n.NoteActivity()
		}
		synctest.Wait()
		if len(u.events) != 1 || u.events[0] != directup.EActive {
			t.Fatalf("idle-to-active events = %v, want one", u.events)
		}
	})
}

type lifecycleHost struct {
	openRecordingHost
	nw *lifecycleNetwork
}

var _ host.Host = lifecycleHost{}

func (h lifecycleHost) Network() network.Network { return h.nw }

type lifecycleNetwork struct {
	network.Network
	conns  []network.Conn
	closes int
}

func (n *lifecycleNetwork) ConnsToPeer(peer.ID) []network.Conn { return n.conns }
func (n *lifecycleNetwork) ClosePeer(peer.ID) error {
	n.closes++
	return nil
}

type lifecycleConn struct {
	network.Conn
	addr   ma.Multiaddr
	closed bool
	opens  int
}

func (c *lifecycleConn) RemoteMultiaddr() ma.Multiaddr { return c.addr }
func (c *lifecycleConn) IsClosed() bool                { return c.closed }
func (c *lifecycleConn) NewStream(context.Context) (network.Stream, error) {
	c.opens++
	return nil, errors.New("connection closed")
}

type lifecycleStream struct {
	network.Stream
	conn  network.Conn
	pipe  net.Conn
	reset func()
}

func (s *lifecycleStream) Conn() network.Conn { return s.conn }

func (s *lifecycleStream) Read(b []byte) (int, error) {
	if s.pipe == nil {
		return 0, io.EOF
	}
	return s.pipe.Read(b)
}
func (s *lifecycleStream) SetReadDeadline(at time.Time) error { return s.pipe.SetReadDeadline(at) }
func (s *lifecycleStream) SetWriteDeadline(time.Time) error   { return nil }
func (s *lifecycleStream) CloseWrite() error                  { return nil }
func (s *lifecycleStream) Reset() error {
	if s.reset != nil {
		s.reset()
	}
	if s.pipe != nil {
		return s.pipe.Close()
	}
	return nil
}

func (s *lifecycleStream) Close() error { return s.Reset() }
