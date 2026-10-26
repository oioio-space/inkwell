package mux

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

const (
	// fragChannel is the reserved wire byte of a fragment frame; it cannot be
	// the channel of a sub-transport.
	fragChannel = 0xFF
	// fragHeader is the size of a fragment header: the fragChannel byte, the
	// channel, then the message id, the total length and the offset of this
	// fragment, each four bytes big-endian.
	fragHeader = 2 + 3*4
	// fragSize is the payload of one fragment. It is the unit of preemption:
	// an interactive message waits at most one fragment, 0.26 s on a link of
	// 1 Mbit/s. fragHeader+fragSize fits a pooled send buffer.
	fragSize = 32 << 10
	// maxReassembly bounds the bytes held in partial messages, across peers,
	// when the transport has no budget of its own (see [Reserver]).
	maxReassembly = 128 << 20
	// partialTTL is how long a partial message may go without a new fragment.
	// It is above the 20 s a sender waits for progress (frame.WriteIdle).
	partialTTL = 30 * time.Second
	// backgroundShare is the "one in n" turn the background class is
	// guaranteed while interactive messages wait: 25 % of the frames.
	backgroundShare = 4
)

// Reserver is implemented by a transport that has a budget for the memory of
// incoming data, as the p2p node has. If the transport given to [Split] is
// one, the partial messages are charged to that budget, so that everything a
// peer can make this process hold counts against one limit.
type Reserver interface {
	// Reserve takes size bytes of the budget and reports whether they were free.
	Reserve(size int) bool
	// Unreserve gives back bytes taken with Reserve.
	Unreserve(size int)
}

// A fragment must fit a pooled send buffer; this fails to compile otherwise.
const _ = uint(maxPooled - fragHeader - fragSize)

// gate serialises the frames one peer receives, serving interactive frames
// before the fragments of a large message but never starving the latter.
// It exists only while a large message to the peer is being sent.
type gate struct {
	big    chan struct{} // capacity 1: held for a whole large message
	hi, lo []chan struct{}
	users  atomic.Int32
	mu     sync.Mutex
	streak int // interactive turns granted since the last background turn
	held   bool
}

func newGate() *gate { return &gate{big: make(chan struct{}, 1)} }

// acquire waits for the turn of a frame of class lo (a fragment) or not.
func (g *gate) acquire(ctx context.Context, lo bool) error {
	g.mu.Lock()
	if !g.held {
		g.held = true
		g.mu.Unlock()
		return nil
	}
	w := make(chan struct{})
	if lo {
		g.lo = append(g.lo, w)
	} else {
		g.hi = append(g.hi, w)
	}
	g.mu.Unlock()
	select {
	case <-w:
		return nil
	case <-ctx.Done():
		g.mu.Lock()
		defer g.mu.Unlock()
		select {
		case <-w: // granted meanwhile: pass the turn on
			g.next()
		default:
			g.lo = remove(g.lo, w)
			g.hi = remove(g.hi, w)
		}
		return ctx.Err()
	}
}

// release gives the turn to the next waiter, if any.
func (g *gate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next()
}

// next hands the turn over; g.mu is held.
func (g *gate) next() {
	var w chan struct{}
	switch {
	case len(g.lo) > 0 && (len(g.hi) == 0 || g.streak >= backgroundShare-1):
		w, g.lo, g.streak = g.lo[0], g.lo[1:], 0
	case len(g.hi) > 0:
		w, g.hi = g.hi[0], g.hi[1:]
		if len(g.lo) > 0 {
			g.streak++
		}
	default:
		g.held = false
		return
	}
	close(w)
}

func remove(s []chan struct{}, w chan struct{}) []chan struct{} {
	for i, x := range s {
		if x == w {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// lease returns the gate of peer to if a large message is being sent to it,
// counting the caller as a user, else nil.
func (m *muxer) lease(to transport.PeerID) *gate {
	if m.ngates.Load() == 0 { // the common case, without a lock
		return nil
	}
	m.gmu.RLock()
	defer m.gmu.RUnlock()
	g := m.gates[to]
	if g != nil {
		g.users.Add(1)
	}
	return g
}

// leaseNew is lease that creates the gate.
func (m *muxer) leaseNew(to transport.PeerID) *gate {
	m.gmu.Lock()
	defer m.gmu.Unlock()
	g := m.gates[to]
	if g == nil {
		g = newGate()
		m.gates[to] = g
		m.ngates.Add(1)
	}
	g.users.Add(1)
	return g
}

// unlease drops a user of g, and the gate when nobody uses it.
func (m *muxer) unlease(to transport.PeerID, g *gate) {
	if g.users.Add(-1) > 0 {
		return
	}
	m.gmu.Lock()
	defer m.gmu.Unlock()
	if g.users.Load() == 0 && m.gates[to] == g {
		delete(m.gates, to)
		m.ngates.Add(-1)
	}
}

// sendFragmented sends payload to peer to as fragments of fragSize, one large
// message at a time per peer, each fragment taking its turn after the
// interactive frames that wait.
func (s *sub) sendFragmented(ctx context.Context, to transport.PeerID, payload []byte) error {
	m := s.m
	g := m.leaseNew(to)
	defer m.unlease(to, g)
	select {
	case g.big <- struct{}{}:
		defer func() { <-g.big }()
	case <-ctx.Done():
		return ctx.Err()
	}
	// The whole message has a deadline, however steadily the receiver makes
	// progress: a drip-feed must not keep the send, and its buffers, alive.
	ctx, cancel := context.WithTimeout(ctx, frame.WriteIdle+time.Duration(len(payload))*time.Second/frame.MinRate)
	defer cancel()
	id := m.msgID.Add(1)
	bp := m.getBuf()
	defer m.bufs.Put(bp)
	for off := 0; off < len(payload); off += fragSize {
		chunk := payload[off:min(off+fragSize, len(payload))]
		buf := append(append((*bp)[:0], fragChannel), s.ch)
		buf = binary.BigEndian.AppendUint32(buf, id)
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(payload))) // #nosec G115 -- ≤ MaxFrame.
		buf = binary.BigEndian.AppendUint32(buf, uint32(off))          // #nosec G115 -- < len(payload).
		buf = append(buf, chunk...)
		*bp = buf
		if err := g.acquire(ctx, true); err != nil {
			return err
		}
		err := m.t.Send(ctx, to, buf)
		g.release()
		if err != nil {
			return err
		}
	}
	return nil
}

// partial is a message being reassembled.
type partial struct {
	last  time.Time // of the latest fragment
	buf   []byte
	id    uint32
	total int
	ch    byte
}

// reassemble handles one fragment frame (starting with fragChannel) from
// peer. It runs on the reader goroutine only. A peer sends one large message
// at a time, in order, so a fragment that does not continue the message in
// progress ends it: the message is dropped, and the sender's next one starts
// afresh. That is also the quota of a peer: one message, at most MaxFrame.
// Memory follows the bytes that really arrived, never the announced total, is
// charged by capacity, and when the budget is short the message that made
// progress least recently is dropped, which is never the one being received.
// It reports false if the muxer stopped.
func (m *muxer) reassemble(peer transport.PeerID, p []byte) bool {
	if len(p) < fragHeader {
		return true
	}
	ch := p[1]
	id := binary.BigEndian.Uint32(p[2:])
	total := int(binary.BigEndian.Uint32(p[6:]))
	off := int(binary.BigEndian.Uint32(p[10:]))
	data := p[fragHeader:]
	s := m.subs[ch]
	cur := m.partials[peer]
	if off == 0 { // a new message ends the one in progress
		m.drop(peer)
		cur = nil
	}
	if s == nil || total <= 0 || total > transport.MaxFrame-Overhead || len(data) == 0 || len(data) > total-off || off < 0 {
		m.drop(peer)
		return true
	}
	switch {
	case cur == nil:
		if off != 0 { // the continuation of a message we dropped
			return true
		}
		cur = &partial{id: id, total: total, ch: ch}
		m.partials[peer] = cur
	case cur.id != id || cur.ch != ch || cur.total != total:
		m.drop(peer)
		return true
	case off+len(data) <= len(cur.buf): // a duplicate: the link repeated it
		return true
	case off != len(cur.buf): // overlaps what we have, or leaves a gap
		m.drop(peer)
		return true
	}
	if !m.grow(peer, cur, len(data)) {
		m.drop(peer)
		return true
	}
	cur.buf = append(cur.buf, data...)
	cur.last = time.Now()
	if len(cur.buf) < total {
		return true
	}
	delete(m.partials, peer)
	m.release(cap(cur.buf))
	return s.q.push(m.ctx, transport.Event{Kind: transport.Message, Peer: peer, Payload: cur.buf})
}

// drop discards the message in progress from peer, giving its memory back.
func (m *muxer) drop(peer transport.PeerID) {
	if cur := m.partials[peer]; cur != nil {
		m.release(cap(cur.buf))
		delete(m.partials, peer)
	}
}

// expire drops the messages that made no progress for partialTTL.
func (m *muxer) expire() {
	now := time.Now()
	for peer, cur := range m.partials {
		if now.Sub(cur.last) > partialTTL {
			m.drop(peer)
		}
	}
}

// grow makes room in cur for n more bytes, at most cur.total in all, doubling
// so that reassembly stays linear. It charges the new capacity first, dropping
// the stalest other messages if the budget is short, and reports whether it
// succeeded.
func (m *muxer) grow(peer transport.PeerID, cur *partial, n int) bool {
	if cap(cur.buf)-len(cur.buf) >= n {
		return true
	}
	size := min(cur.total, max(2*cap(cur.buf), len(cur.buf)+n, fragSize))
	extra := size - cap(cur.buf)
	for !m.charge(extra) {
		if !m.evictStalest(peer) {
			return false
		}
	}
	bigger := make([]byte, len(cur.buf), size)
	copy(bigger, cur.buf)
	cur.buf = bigger
	return true
}

// charge takes n bytes of the budget, reporting whether they were free.
func (m *muxer) charge(n int) bool {
	if m.held+n > maxReassembly {
		return false
	}
	if m.res != nil && !m.res.Reserve(n) {
		return false
	}
	m.held += n
	return true
}

// release gives back n bytes taken by charge.
func (m *muxer) release(n int) {
	m.held -= n
	if m.res != nil {
		m.res.Unreserve(n)
	}
}

// evictStalest drops the message that made progress least recently, except
// keep's, and reports whether there was one.
func (m *muxer) evictStalest(keep transport.PeerID) bool {
	var victim transport.PeerID
	var oldest time.Time
	found := false
	for peer, cur := range m.partials {
		if peer != keep && (!found || cur.last.Before(oldest)) {
			victim, oldest, found = peer, cur.last, true
		}
	}
	if found {
		m.drop(victim)
	}
	return found
}

// recv is Recv on t, which also drops the stale partial messages: it
// wakes up for that when there are some and nothing arrives.
func (m *muxer) recv() (transport.Event, error) {
	for {
		ctx, cancel := m.ctx, context.CancelFunc(func() {})
		if len(m.partials) > 0 {
			m.expire()
			ctx, cancel = context.WithTimeout(m.ctx, partialTTL)
		}
		ev, err := m.t.Recv(ctx)
		cancel()
		if err != nil && m.ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		return ev, err
	}
}
