package bridge

import (
	"slices"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	p "github.com/oioio-space/inkwell/protocol/bridge"
)

// The wire values of the path and the grades are the transport's own, so they
// convert directly.
var (
	_ = [1]struct{}{}[uint8(transport.LinkLAN)-p.PathLAN]
	_ = [1]struct{}{}[uint8(transport.LinkDirect)-p.PathDirect]
	_ = [1]struct{}{}[uint8(transport.LinkRelayed)-p.PathRelayed]
	_ = [1]struct{}{}[uint8(transport.QualityUnknown)-p.QualityUnknown]
	_ = [1]struct{}{}[uint8(transport.QualityGood)-p.QualityGood]
	_ = [1]struct{}{}[uint8(transport.QualityFair)-p.QualityFair]
	_ = [1]struct{}{}[uint8(transport.QualityPoor)-p.QualityPoor]
	_ = [1]struct{}{}[uint8(transport.LossPath)-p.LossScopePath]
	_ = [1]struct{}{}[uint8(transport.LossFirstHop)-p.LossScopeFirstHop]
)

// linkEntry converts a [PeerLink] to its wire form, instants as ages at now.
func linkEntry(l PeerLink, now time.Time) LinkEntry {
	return p.LinkOf(p.Measures{
		Peer: l.Peer, Path: uint8(l.Kind), Quality: uint8(l.Quality()), LossScope: uint8(l.LossScope),
		PathSince: l.PathSince, UpSince: l.UpSince,
		RTT: l.RTT, RTTVar: l.RTTVar, RTTAge: l.RTTAge, LossPct: l.LossPct,
		LastDown: l.LastDown, OldestPending: l.OldestPending,
		Upgrades: l.Upgrades, Fallbacks: l.Fallbacks, Downs10m: l.Downs10m, DownsTotal: l.DownsTotal,
		Resyncs10m: l.Resyncs10m, Resyncs: l.Resyncs, PendingMsgs: l.PendingMsgs, PendingBytes: l.PendingBytes,
		SendFailures: l.SendFailures, Resets: l.Resets,
	}, now)
}

// linkEntries converts ls, which must be sorted by peer, to wire entries; the
// first [MaxLinks] are kept.
func linkEntries(ls []PeerLink, now time.Time) []LinkEntry {
	ls = ls[:min(len(ls), MaxLinks)]
	es := make([]LinkEntry, 0, len(ls))
	for _, l := range ls {
		if transport.ValidPeerID(l.Peer) {
			es = append(es, linkEntry(l, now))
		}
	}
	return es
}

// stableLinks returns es with their ages reduced to set or unset, the form in
// which two states are compared.
func stableLinks(es []LinkEntry) []LinkEntry {
	out := make([]LinkEntry, len(es))
	for i, e := range es {
		out[i] = e.Stable()
	}
	return out
}

// LinksPushPeriod is the shortest time between two Links notifications.
const LinksPushPeriod = time.Second

// linksEnvelope encodes the Links envelope of es.
func (r *Router) linksEnvelope(es []LinkEntry) (Envelope, bool) {
	payload, err := EncodeLinks(es)
	if err != nil {
		r.log.Error("bridge: encode links", "err", err)
		return Envelope{}, false
	}
	return Envelope{Kind: Links, Peer: r.t.Local(), Payload: payload}, true
}

// pushLinks sends a Links notification to s each period if the links changed
// since last (stable form), until s ends. Links opens no epoch: rule T does
// not apply.
func (r *Router) pushLinks(s *session, last []LinkEntry) {
	tick := time.NewTicker(r.linksPeriod)
	defer tick.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-tick.C:
		}
		es := linkEntries(r.Links(), time.Now())
		if st := stableLinks(es); !slices.Equal(st, last) {
			last = st
			if e, ok := r.linksEnvelope(es); ok {
				r.send(s, e)
			}
		}
	}
}
