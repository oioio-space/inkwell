package mux

import (
	"cmp"
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/oioio-space/inkwell/internal/transport"
)

// flushAt is how many sent bytes a send buffer holds back before it adds them
// to its link: the hot path of a small Send then costs no atomic operation,
// one in about every 500 for frames of 256 bytes, one in eight for 16 KiB.
const flushAt = 128 << 10

// LinkTraffic is what crossed the link to one peer since the [Split], in
// bytes on the wire (channel byte and fragment headers included).
type LinkTraffic struct {
	Peer transport.PeerID
	// Sent lags behind by what a send buffer holds back (see [Traffic]).
	Sent     uint64
	Received uint64
	// Opened counts the PeerUp events of the peer: the connections opened.
	Opened uint64
}

// link holds the counters of one peer: recv and opened are written by the
// reader goroutine only, sent by the send buffers when they flush, rarely
// (see flushAt).
type link struct {
	sent, recv, opened atomic.Uint64
}

// links is the set of links of a muxer, summed only when read. Each read
// starts a new epoch: a send buffer adds what it holds back at its first Send
// of a new epoch, so a count lags one read behind at most, or flushAt bytes.
type links struct {
	m     sync.Map // transport.PeerID → *link
	epoch atomic.Uint32
}

// of returns the link of peer, creating it.
func (ls *links) of(peer transport.PeerID) *link {
	if l, ok := ls.m.Load(peer); ok {
		return l.(*link)
	}
	l, _ := ls.m.LoadOrStore(peer, new(link))
	return l.(*link)
}

// sendBuf is a pooled send buffer and the sent bytes it holds back for one
// link. Whoever took it from the pool owns it, so its fields are plain.
type sendBuf struct {
	b   []byte
	acc *pending
}

// pending is the part of a sendBuf that survives it: a cleanup adds what it
// holds back to its link when the pool lets the buffer go.
type pending struct {
	links *links // of the muxer
	peer  transport.PeerID
	at    *byte // where peer's bytes are
	l     *link
	n     uint64
	epoch uint32 // of links when n started
}

func (p *pending) flush() {
	if p.n > 0 {
		p.l.sent.Add(p.n)
		p.n = 0
	}
}

// add counts n bytes sent to peer: they are held back until the buffer
// changes peer or holds flushAt bytes. Send checks [pending.ready] itself
// first, which is inlined, and calls add only when it is false.
func (p *pending) add(peer transport.PeerID, n int) {
	if !p.ready(peer) {
		p.switchTo(peer)
	}
	p.n += uint64(n) // #nosec G115 -- a frame length, never negative.
}

// ready reports whether p has room, in the current epoch, and was last used
// for this very string: the common case compares where the two strings point,
// not their bytes.
func (p *pending) ready(peer transport.PeerID) bool {
	return p.n < flushAt && p.at == unsafe.StringData(string(peer)) && len(p.peer) == len(peer) && // #nosec G103 -- read only: same data pointer and length is the same immutable string; p.peer keeps it alive.
		p.epoch == p.links.epoch.Load()
}

// switchTo adds what p holds back to its link when p is full, the links were
// read since, or the peer is another, and points p at peer.
func (p *pending) switchTo(peer transport.PeerID) {
	epoch := p.links.epoch.Load()
	if p.peer != peer || p.l == nil {
		p.flush()
		p.l = p.links.of(peer)
	} else if p.n >= flushAt || p.epoch != epoch {
		p.flush()
	}
	// The same bytes, maybe elsewhere: the next Send compares pointers.
	p.peer, p.at, p.epoch = peer, unsafe.StringData(string(peer)), epoch // #nosec G103 -- read only, compared by ready; never written through.
}

// newSendBuf returns an empty send buffer whose held-back bytes reach their
// link even once the pool has dropped it.
func newSendBuf(ls *links) *sendBuf {
	bp := &sendBuf{acc: &pending{links: ls}}
	runtime.AddCleanup(bp, (*pending).flush, bp.acc)
	return bp
}

// recvLink returns the link of peer for the reader goroutine, which keeps
// its own map of them so that a frame costs no shared lookup.
func (m *muxer) recvLink(peer transport.PeerID) *link {
	l := m.recvLinks[peer]
	if l == nil {
		l = m.links.of(peer)
		m.recvLinks[peer] = l
	}
	return l
}

// Traffic returns, sorted by peer, the bytes sent to and received from each
// peer by every sub-transport of the same [Split] as t, or by a [Counted]
// lane, and the connections opened; nil for another transport. The counters are
// per link and summed here only: no counter is shared by all links on the
// send path. A sent byte is counted once the Send that carried it returned
// without error, and a send buffer holds back at most 128 KiB plus one frame:
// it adds them at its first Send after this read, or when the pool lets it go.
func Traffic(t transport.Transport) []LinkTraffic {
	var ls *links
	switch t := t.(type) {
	case *sub:
		ls = &t.m.links
	case *counted:
		ls = &t.links
	default:
		return nil
	}
	ls.epoch.Add(1)
	var out []LinkTraffic
	ls.m.Range(func(k, v any) bool {
		l := v.(*link)
		out = append(out, LinkTraffic{
			Peer:     k.(transport.PeerID),
			Sent:     l.sent.Load(),
			Received: l.recv.Load(),
			Opened:   l.opened.Load(),
		})
		return true
	})
	slices.SortFunc(out, func(a, b LinkTraffic) int { return cmp.Compare(a.Peer, b.Peer) })
	return out
}

// Counted returns t with its bytes counted per link, read with [Traffic],
// for a lane that carries no channels: the bulk lane of the blobs. Each frame
// costs a lookup of its link and one atomic add, small beside a chunk's
// copy and disk write. The caller must not use t directly afterwards.
func Counted(t transport.Transport) transport.Transport { return &counted{Transport: t} }

type counted struct {
	transport.Transport
	links links
}

func (c *counted) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	err := c.Transport.Send(ctx, to, payload)
	if err == nil {
		c.links.of(to).sent.Add(uint64(len(payload))) // #nosec G115 -- a length.
	}
	return err
}

func (c *counted) Recv(ctx context.Context) (transport.Event, error) {
	ev, err := c.Transport.Recv(ctx)
	switch {
	case err != nil:
	case ev.Kind == transport.Message:
		c.links.of(ev.Peer).recv.Add(uint64(len(ev.Payload))) // #nosec G115 -- a length.
	case ev.Kind == transport.PeerUp:
		c.links.of(ev.Peer).opened.Add(1)
	}
	return ev, err
}
