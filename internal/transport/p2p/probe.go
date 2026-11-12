package p2p

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	msmux "github.com/multiformats/go-multistream"
	"github.com/quic-go/quic-go"

	"github.com/oioio-space/inkwell/internal/transport"
	wire "github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

// The link probe (spec §8.5, ADR 0036) on libp2p: one [linkprobe.Host] per QUIC
// connection, the rules of the sans-I/O reference, and here only the mapping of
// its events and actions to libp2p streams and quic-go's counters. Nothing in
// this file decides what the probe sends or accepts.

// ProtocolProbe is the libp2p protocol of the link probe.
const ProtocolProbe = linkprobe.ProtocolID

// probeReadSize is the size of the buffer a probe stream is read into. The
// reference host takes any chunking, so it only has to be small.
const probeReadSize = 512

// probeRoles says what a connection's peer is to this endpoint (spec §8.5.1). It
// is asked at each tick and at each opened stream, so a member removed meanwhile
// stops being probed and answered.
type probeRoles func(c network.Conn) linkprobe.Roles

// probeSet runs the probe on every raw QUIC connection of a libp2p host. The
// connection sampler of a [Node] reads it; a relay only answers.
type probeSet struct {
	roles probeRoles
	t0    time.Time // the origin of the host's monotonic clock
	ctx   context.Context

	mu     sync.Mutex
	conns  map[string]*probeConn // by network.Conn.ID
	closed bool
	wg     sync.WaitGroup

	nextStream atomic.Uint64

	// afterNewStream, if not nil, runs in an outbound attempt once its stream is open and before the
	// negotiation: a test holds the attempt there to cancel it meanwhile.
	afterNewStream func()
}

// newProbeSet returns a probe set that stops with ctx or [probeSet.close].
func newProbeSet(ctx context.Context, h host.Host, roles probeRoles) *probeSet {
	ps := &probeSet{roles: roles, t0: time.Now(), ctx: ctx, conns: make(map[string]*probeConn)}
	h.SetStreamHandler(ProtocolProbe, ps.handle)
	return ps
}

// now is the monotonic clock of every Host of the set.
func (ps *probeSet) now() time.Duration { return time.Since(ps.t0) }

// spawn runs f on a goroutine that close waits for; it does nothing once the
// set is closed.
func (ps *probeSet) spawn(f func()) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return
	}
	ps.wg.Go(f)
}

// close stops every connection's probe and waits for its goroutines.
func (ps *probeSet) close() {
	ps.mu.Lock()
	ps.closed = true
	conns := make([]*probeConn, 0, len(ps.conns))
	for _, pc := range ps.conns {
		conns = append(conns, pc)
	}
	ps.mu.Unlock()
	for _, pc := range conns {
		pc.stop()
	}
	ps.wg.Wait()
}

// attach starts the probe of c if it is a raw QUIC connection: a TCP connection
// and a circuit never carry it (spec §8.5.1).
func (ps *probeSet) attach(c network.Conn) {
	if isCircuit(c.RemoteMultiaddr()) {
		return
	}
	var qc *quic.Conn
	if !c.As(&qc) || qc == nil {
		return
	}
	roles := ps.roles(c) // before our lock: it may take the node's
	ps.mu.Lock()
	if ps.closed || ps.conns[c.ID()] != nil {
		ps.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ps.ctx)
	pc := &probeConn{
		ps: ps, c: c, qc: qc, ctx: ctx, cancel: cancel, roles: roles,
		ins: make(map[uint64]*probeStream),
	}
	pc.host = linkprobe.NewHost(linkprobe.QUIC, roles, ps.now())
	ps.conns[c.ID()] = pc
	ps.wg.Go(pc.run)
	ps.mu.Unlock()
}

// detach stops the probe of c, which is gone.
func (ps *probeSet) detach(c network.Conn) {
	ps.mu.Lock()
	pc := ps.conns[c.ID()]
	delete(ps.conns, c.ID())
	ps.mu.Unlock()
	if pc != nil {
		pc.stop()
	}
}

// forget unregisters pc, unless another probe took its place, and stops it.
func (ps *probeSet) forget(pc *probeConn) {
	ps.mu.Lock()
	if ps.conns[pc.c.ID()] == pc {
		delete(ps.conns, pc.c.ID())
	}
	ps.mu.Unlock()
	pc.stop()
}

// of returns the probe of the connection id, or nil, also once the set is closed.
func (ps *probeSet) of(id string) *probeConn {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return nil
	}
	return ps.conns[id]
}

// handle is the stream handler of the probe protocol: libp2p calls it when the
// negotiation of an inbound stream is finished, which is the instant the
// responder accepts the stream (spec §8.5.6, ouvert_à).
func (ps *probeSet) handle(s network.Stream) {
	pc := ps.of(s.Conn().ID())
	if pc == nil || pc.ctx.Err() != nil { // a connection the probe does not run on, or a set that closes: refuse, unread (spec §8.5.1)
		_ = s.Reset()
		return
	}
	pc.serve(s)
}

// probeConn is the probe of one QUIC connection.
type probeConn struct {
	ps     *probeSet
	c      network.Conn
	qc     *quic.Conn
	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex // guards everything below; the Host is not safe for concurrent use
	host  *linkprobe.Host
	roles linkprobe.Roles
	// attempt numbers the outbound attempts: OpenOut hands out the next, ResetOut ends the current
	// one by taking the next, so an attempt that finds another number is stale and resets what it
	// opened, whatever the order of events.
	attempt uint64
	neg     network.Stream // the outbound stream being negotiated
	out     *probeStream   // the outbound stream, once negotiated
	ins     map[uint64]*probeStream

	views atomic.Uint64 // how many times the sampler looked at the connection, for the tests
}

// probeStream is a stream and its one-message writer.
type probeStream struct {
	s network.Stream
	w *msgWriter
}

// newStream wraps s and starts its writer.
func (pc *probeConn) newStream(s network.Stream) *probeStream {
	w := &msgWriter{s: s, ch: make(chan writeOp, 2), end: make(chan struct{})}
	pc.ps.spawn(w.run)
	return &probeStream{s: s, w: w}
}

// run is the prober's timer: one tick per interval, with the endpoint's jitter
// (spec §8.5.5 step 2). It ends with the probe or with the connection, and then
// unregisters the probe: a connection that ended, and whose detach came and went,
// before attach inserted it is not probed until shutdown.
func (pc *probeConn) run() {
	defer pc.ps.forget(pc)
	for {
		d := wire.LinkProbeInterval + time.Duration((rand.Float64()*0.2-0.1)*float64(wire.LinkProbeInterval)) // #nosec G404 -- timer jitter, not a secret.
		t := time.NewTimer(d)
		select {
		case <-t.C:
		case <-pc.ctx.Done():
			t.Stop()
			return
		case <-pc.qc.Context().Done():
			t.Stop()
			return
		}
		pc.tick()
	}
}

// tick gives the Host its timer, with the counter and the writer state.
func (pc *probeConn) tick() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	now := pc.ps.now()
	pc.syncRoles(now)
	sent := pc.qc.ConnectionStats().PacketsSent
	pc.do(pc.host.Tick(now, sent, pc.outFree()))
}

// syncRoles tells the Host when the peer's roles changed; the caller holds pc.mu.
func (pc *probeConn) syncRoles(now time.Duration) {
	if r := pc.ps.roles(pc.c); r != pc.roles {
		pc.roles = r
		pc.do(pc.host.SetRoles(now, r))
	}
}

func (pc *probeConn) outFree() bool { return pc.out != nil && pc.out.w.free() }

// stop ends the probe of the connection: every stream is reset.
func (pc *probeConn) stop() {
	pc.cancel()
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if pc.neg != nil {
		_ = pc.neg.Reset()
		pc.neg = nil
	}
	if pc.out != nil {
		_ = pc.out.s.Reset()
		pc.out = nil
	}
	for _, in := range pc.ins {
		_ = in.s.Reset()
	}
}

// view returns what the probe shows of the connection at this instant; the sampler looks once per
// connection and step, which is how often the level may be read (spec §8.5.5 step 6). The level
// is the connection's: the caller drops it on the hop to a relay, whose level is not the link's
// (Annex A.1), keeping only the figure as a diagnostic.
func (pc *probeConn) view() probeView {
	pc.views.Add(1)
	pc.mu.Lock()
	defer pc.mu.Unlock()
	now := pc.ps.now()
	v := probeView{loss: pc.host.Loss(now), level: transport.Quality(pc.host.Level(now))}
	v.hasLoss = v.loss != linkprobe.Unknown
	if rtt, dev, age, ok := pc.host.RTT(now); ok && rtt > 0 {
		v.rtt, v.rttDev, v.rttAge, v.hasRTT = rtt, dev, age, true
	}
	return v
}

// do carries out the Host's actions; the caller holds pc.mu.
func (pc *probeConn) do(acts []linkprobe.Action) {
	for _, a := range acts {
		switch a.Kind {
		case linkprobe.OpenOut:
			pc.attempt++
			att := pc.attempt
			pc.ps.spawn(func() { pc.openOut(att) })
		case linkprobe.WriteOut:
			if pc.out != nil {
				pc.out.w.put(a.Data)
			}
		case linkprobe.FinOut:
			if pc.out != nil {
				pc.out.w.fin()
			}
		case linkprobe.ResetOut:
			pc.attempt++ // an attempt in flight is over
			if pc.neg != nil {
				_ = pc.neg.Reset()
				pc.neg = nil
			}
			if pc.out != nil {
				_ = pc.out.s.Reset()
				pc.out = nil
			}
		case linkprobe.WriteIn:
			if in := pc.ins[a.Stream]; in != nil {
				in.w.put(a.Data)
			}
		case linkprobe.FinIn:
			if in := pc.ins[a.Stream]; in != nil {
				in.w.fin()
			}
		case linkprobe.ResetIn, linkprobe.RefuseIn:
			if in := pc.ins[a.Stream]; in != nil {
				_ = in.s.Reset()
			}
		}
	}
}

// openOut opens the outbound stream on the measured connection and negotiates
// it to the end before the first PROBE: a lazy negotiation would only show a
// refusal at the first read (spec §8.5.3).
func (pc *probeConn) openOut(att uint64) {
	ctx, cancel := context.WithTimeout(pc.ctx, wire.LinkProbeNegotiationTimeout)
	defer cancel()
	s, err := pc.c.NewStream(ctx)
	if err != nil {
		pc.negotiated(att, nil, err)
		return
	}
	if hook := pc.ps.afterNewStream; hook != nil {
		hook()
	}
	pc.mu.Lock()
	if pc.attempt != att { // the Host gave up on this attempt while the stream was opening
		pc.mu.Unlock()
		_ = s.Reset()
		return
	}
	pc.neg = s
	pc.mu.Unlock()
	stop := context.AfterFunc(ctx, func() { _ = s.Reset() }) // the 10 s of the negotiation
	err = msmux.SelectProtoOrFail(ProtocolProbe, s)
	if !stop() && err == nil { // the context ended first: the stream was reset
		err = ctx.Err()
	}
	if err == nil {
		err = s.SetProtocol(protocol.ID(ProtocolProbe)) // the resource manager's limit applies here
	}
	pc.negotiated(att, s, err)
}

// negotiated reports the end of the negotiation of attempt att to the Host: refused is the
// multistream "na", any other error an ordinary failure. A stale attempt reports nothing.
func (pc *probeConn) negotiated(att uint64, s network.Stream, err error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	now := pc.ps.now()
	if pc.attempt != att { // ResetOut ended it meanwhile: the Host knows
		if s != nil {
			_ = s.Reset()
		}
		return
	}
	pc.neg = nil
	switch {
	case err == nil:
		if pc.out != nil { // never two outbound streams: the older one goes
			_ = pc.out.s.Reset()
		}
		pc.out = pc.newStream(s)
		ps := pc.out
		pc.ps.spawn(func() { pc.readOut(ps) })
		sent := pc.qc.ConnectionStats().PacketsSent
		pc.do(pc.host.Negotiated(now, true, sent, ps.w.free()))
	case errors.Is(err, msmux.ErrNotSupported[string]{}):
		_ = s.Reset()
		pc.do(pc.host.Negotiated(now, false, 0, false))
	default:
		if s != nil {
			_ = s.Reset()
		}
		pc.do(pc.host.OutboundEnded(now))
	}
}

// readOut reads the outbound stream to its end: it is always read, whether its
// writer is busy or not (spec §8.5.3), and the writer is another goroutine.
func (pc *probeConn) readOut(ps *probeStream) {
	buf := make([]byte, probeReadSize)
	for {
		n, err := ps.s.Read(buf)
		if n > 0 {
			st := pc.qc.ConnectionStats() // right after the read call (spec §8.5.5 step 3)
			pto := linkprobe.PTO(st.SmoothedRTT, st.MeanDeviation, -1)
			pc.mu.Lock()
			if pc.out == ps {
				pc.do(pc.host.OutboundData(pc.ps.now(), buf[:n], st.PacketsSent, pto))
			}
			pc.mu.Unlock()
		}
		if err != nil {
			pc.mu.Lock()
			if pc.out == ps {
				pc.out = nil
				pc.do(pc.host.OutboundEnded(pc.ps.now()))
			}
			pc.mu.Unlock()
			ps.w.stop()
			_ = ps.s.Reset()
			return
		}
	}
}

// serve is the responder on one inbound stream.
func (pc *probeConn) serve(s network.Stream) {
	id := pc.ps.nextStream.Add(1)
	in := pc.newStream(s)
	pc.mu.Lock()
	now := pc.ps.now() // after the negotiation: ouvert_à
	pc.syncRoles(now)
	pc.ins[id] = in
	acts := pc.host.InboundOpened(now, id)
	pc.do(acts)
	pc.mu.Unlock()
	defer func() {
		pc.mu.Lock()
		delete(pc.ins, id)
		pc.mu.Unlock()
		in.w.stop() // what the writer holds, the FIN included, goes out first
		t := time.NewTimer(wire.LinkProbeNegotiationTimeout)
		defer t.Stop()
		select {
		case <-in.w.end:
		case <-t.C:
		case <-pc.ctx.Done():
		}
		_ = s.Reset()
	}()
	buf := make([]byte, probeReadSize)
	for {
		n, err := s.Read(buf)
		if n > 0 {
			received := pc.qc.ConnectionStats().PacketsReceived // right after the read call (spec §8.5.6)
			pc.mu.Lock()
			pc.do(pc.host.InboundData(pc.ps.now(), id, buf[:n], received, in.w.free()))
			pc.mu.Unlock()
		}
		if err != nil {
			pc.mu.Lock()
			pc.do(pc.host.InboundEnded(pc.ps.now(), id, errors.Is(err, io.EOF)))
			pc.mu.Unlock()
			return
		}
	}
}

// writeOp is one thing for a writer to do: write a message, or close the stream
// for writing.
type writeOp struct {
	data []byte
	fin  bool
}

// msgWriter is the one-message writer of spec §8.5.3: a message is handed to the
// transport whole, the writer is free again once the transport accepted it, and
// nothing is queued behind a busy writer: the Host skips its PROBE or drops its
// ECHO. The writing is on its own goroutine because a stream write may block on
// flow control, and the reading must never wait for it.
type msgWriter struct {
	s    network.Stream
	busy atomic.Bool
	ch   chan writeOp // one message and one FIN at most
	once sync.Once
	end  chan struct{} // closed when the writer goroutine ends
}

// free reports whether the writer holds no message.
func (w *msgWriter) free() bool { return !w.busy.Load() }

// put hands a message to the writer, which the caller found free.
func (w *msgWriter) put(b []byte) {
	w.busy.Store(true)
	w.ch <- writeOp{data: b}
}

// fin closes the stream for writing once the message in the writer is out.
func (w *msgWriter) fin() { w.ch <- writeOp{fin: true} }

// stop ends the writer: it finishes what it holds.
func (w *msgWriter) stop() { w.once.Do(func() { close(w.ch) }) }

func (w *msgWriter) run() {
	defer close(w.end)
	for op := range w.ch {
		if op.fin {
			_ = w.s.CloseWrite()
			continue
		}
		if _, err := w.s.Write(op.data); err != nil {
			return // the stream is dead and the reader will see it; busy stays set
		}
		w.busy.Store(false)
	}
}

// probeRoles says what the peer of c is to this node (spec §8.5.1): a member of
// the project (Options.Admit; nil admits everyone, for tests) is probed and
// answered, a relay the node uses is probed, and anybody else, a guest of the
// lobby included, neither. The stranger window of [Node.SetOpen] makes no role.
func (n *Node) probeRoles(c network.Conn) linkprobe.Roles {
	var r linkprobe.Roles
	id := c.RemotePeer()
	if _, ok := n.relays[id]; ok {
		r |= linkprobe.Relay
	}
	if name, ok := peerName(id); ok && !n.isHeld(id) && (n.opts.Admit == nil || n.opts.Admit(name)) {
		r |= linkprobe.Member
	}
	return r
}
