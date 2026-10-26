package mux

import (
	"context"
	"encoding/binary"
	"sync"
	"sync/atomic"

	"github.com/oioio-space/inkwell/internal/transport"
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
	// maxReassembly bounds the bytes held in partial messages, across peers.
	maxReassembly = 128 << 20
	// backgroundShare is the "one in n" turn the background class is
	// guaranteed while interactive messages wait: 25 % of the frames.
	backgroundShare = 4
)

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

// partial is a message being reassembled from fragments.
type partial struct {
	buf   []byte
	id    uint32
	total int
	ch    byte
}

// reassemble handles one fragment frame (starting with fragChannel) from
// peer. It runs on the reader goroutine only. A peer sends one large message
// at a time, in order, so a fragment that does not continue the message in
// progress ends it: the message is dropped, and the sender's next one starts
// afresh. Memory follows the bytes that really arrived, never the announced
// total. It reports false if the muxer stopped.
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
	if m.held+len(data) > maxReassembly {
		m.drop(peer)
		return true
	}
	cur.buf = grow(cur.buf, len(data), total)
	cur.buf = append(cur.buf, data...)
	m.held += len(data)
	if len(cur.buf) < total {
		return true
	}
	delete(m.partials, peer)
	m.held -= total
	return s.q.push(m.ctx, transport.Event{Kind: transport.Message, Peer: peer, Payload: cur.buf})
}

// drop discards the message in progress from peer.
func (m *muxer) drop(peer transport.PeerID) {
	if cur := m.partials[peer]; cur != nil {
		m.held -= len(cur.buf)
		delete(m.partials, peer)
	}
}

// grow makes room in buf for n more bytes, at most limit in all, doubling so
// that reassembly stays linear.
func grow(buf []byte, n, limit int) []byte {
	if cap(buf)-len(buf) >= n {
		return buf
	}
	bigger := make([]byte, len(buf), min(limit, max(2*cap(buf), len(buf)+n, fragSize)))
	copy(bigger, buf)
	return bigger
}
