package bridge

import (
	"cmp"
	"slices"
	"sync/atomic"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// peerCounts are the failures of the sends to one peer (plan W33, §7).
type peerCounts struct {
	// sendFailures counts writes given up, the detector of a broken stream;
	// resets counts the peer resets after its queue overflowed.
	sendFailures, resets atomic.Uint64
}

// countsOf returns the counters of peer, creating them.
func (r *Router) countsOf(peer transport.PeerID) *peerCounts {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.counts[peer]
	if c == nil {
		c = new(peerCounts)
		r.counts[peer] = c
	}
	return c
}

// SlowSend is how long a message may wait to be sent before the link is called
// slow (« Lien lent », ADR 0028 D8); VerySlowSend makes it poor.
const (
	SlowSend     = 2 * time.Second
	VerySlowSend = 10 * time.Second
)

// PeerLink is the transport's [transport.Link] to a peer with what the router
// holds back for it. No network address is part of it.
type PeerLink struct {
	transport.Link
	// PendingMsgs and PendingBytes are what waits to be sent to the peer,
	// the message being sent included (« Envoi en cours (n) »).
	PendingMsgs, PendingBytes int
	// OldestPending is how long the message being sent has waited, or zero.
	OldestPending time.Duration
	// SendFailures counts writes given up and Resets the peer resets after
	// a queue overflow, since the router started.
	SendFailures, Resets uint64
}

// Quality is the transport's grade, lowered to fair when a message has waited
// more than [SlowSend], and to poor after [VerySlowSend].
func (p PeerLink) Quality() transport.Quality {
	q := p.Link.Quality()
	var slow transport.Quality
	switch {
	case p.OldestPending >= VerySlowSend:
		slow = transport.QualityPoor
	case p.OldestPending >= SlowSend:
		slow = transport.QualityFair
	default:
		return q
	}
	return max(q, slow)
}

// Links returns the state of the link to each peer, sorted by peer: the
// transport's figures, if it implements [transport.LinkSource] (otherwise only
// the connected peers, with no figure), merged with the router's own.
func (r *Router) Links() []PeerLink {
	var base []transport.Link
	if src, ok := r.t.(transport.LinkSource); ok {
		base = src.Links()
	} else {
		for _, p := range r.t.Peers() {
			base = append(base, transport.Link{Peer: p})
		}
	}
	// One look at the router's state: the session and the failure counters.
	r.mu.Lock()
	s := r.cur
	counts := make([]*peerCounts, len(base))
	for i, l := range base {
		counts[i] = r.counts[l.Peer]
	}
	r.mu.Unlock()

	out := make([]PeerLink, 0, len(base))
	if s != nil {
		s.omu.Lock()
	}
	for i, l := range base {
		pl := PeerLink{Link: l}
		if s != nil {
			if o := s.out[l.Peer]; o != nil {
				pl.PendingMsgs = int(max(o.msgs.Load(), 0))
				pl.PendingBytes = int(max(o.bytes.Load(), 0))
				if h := o.head.Load(); h != 0 {
					pl.OldestPending = max(time.Duration(r.stamp()-h), 0)
				}
			}
		}
		if c := counts[i]; c != nil {
			pl.SendFailures, pl.Resets = c.sendFailures.Load(), c.resets.Load()
		}
		out = append(out, pl)
	}
	if s != nil {
		s.omu.Unlock()
	}
	slices.SortFunc(out, func(a, b PeerLink) int { return cmp.Compare(a.Peer, b.Peer) })
	return out
}
