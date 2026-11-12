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

// Quality is the level of the link the person is shown: the probe's, unknown while it has no figure
// and always on a relayed link, whose end-to-end quality nothing measures (spec Annex A.1). A message
// that waits a long time to be sent is another signal, [PeerLink.OldestPending], which the sync
// indicator shows as « Lien lent » (ADR 0028 D8); it never changes the level.
func (p PeerLink) Quality() transport.Quality {
	if p.Kind == transport.LinkRelayed {
		return transport.QualityUnknown
	}
	return p.ProbeLevel
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
