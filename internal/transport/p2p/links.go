package p2p

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/quic-go/quic-go"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

var _ transport.LinkSource = (*Node)(nil)

// peerLink is the metrics state of one peer, under Node.lmu. It outlives the
// peer's PeerDown so that the outage counters survive a reconnection; the
// sampler forgets it once the peer has been gone for [transport.DownsWindow].
type peerLink struct {
	rtt      rttEstimator
	path     pathTrack
	stab     stability
	probe    probeView           // what the link probe showed at the last step
	scope    transport.LossScope // what the figure measures
	quicRTT  bool                // the RTT comes from QUIC, so pings are not folded in
	lastPing time.Time           // when the last ping started
	pinging  bool
}

// probeView is what the link probe of a connection showed at one sampler step.
type probeView struct {
	level   transport.Quality
	loss    uint16
	hasLoss bool
	// rtt, rttDev and rttAge are the window's round trip (Annex A.1 « Sens de rttMs et
	// rttVarMs »), only if hasRTT is set, which is when the figure is known.
	rtt, rttDev, rttAge time.Duration
	hasRTT              bool
}

// LinkStep is what one step of the link sampler saw of a link: the level and the
// figure of its probe, known or not. A diagnostics build records every step, so
// that a test can replay the level from the figures (plan C5S, R1).
type LinkStep struct {
	Peer transport.PeerID
	At   time.Time
	// Kind is the path the link is on; the level of a relayed link is unknown
	// whatever its first hop's figure says.
	Kind transport.LinkKind
	// Level is the quality shown for the link, [transport.QualityUnknown]
	// when the probe has no figure or the link is relayed.
	Level transport.Quality
	// LossPermille is the probe's figure, [LossUnknown] while it is unknown.
	LossPermille uint16
}

// LossUnknown is the LossPermille of a step whose figure is unknown (Annex A.1).
const LossUnknown = linkprobe.Unknown

// step returns what the sampler saw of the peer id at now; the caller holds n.lmu.
func (pl *peerLink) step(id peer.ID, now time.Time) LinkStep {
	name, _ := peerName(id)
	st := LinkStep{Peer: name, At: now, Kind: pl.path.kind, Level: pl.probe.level, LossPermille: LossUnknown}
	if pl.probe.hasLoss {
		st.LossPermille = pl.probe.loss
	}
	return st
}

// linkSample is what one look at the connections to a peer shows. It is plain
// values, taken without any lock, and applied to a peerLink under Node.lmu.
type linkSample struct {
	id peer.ID // the peer it is of
	// out is the stream the peer's session ran on when the sample was taken: a sample
	// of an ended session, or of a session moved to another stream since, is stale.
	out  network.Stream
	kind transport.LinkKind // zero: no connection
	// The remaining fields are meaningful only if quic is set.
	quic  bool
	scope transport.LossScope
	probe probeView
	// srtt and dev are the QUIC round trip, only if rttFromQUIC is set.
	srtt, dev   time.Duration
	rttFromQUIC bool
}

// The lock order is Node.mu, then Node.lmu. lmu is a leaf: code under it never
// takes mu and never calls into libp2p or QUIC; probeLink does that before, under
// neither.

// linkUp records a PeerUp, with a first look at the connection so that Links
// has the path as soon as the peer is announced. The caller may hold n.mu.
func (n *Node) linkUp(id peer.ID, s linkSample) {
	now := time.Now()
	n.lmu.Lock()
	defer n.lmu.Unlock()
	pl := n.linkOf(id)
	pl.stab.up(now)
	n.applyLinkLocked(id, pl, now, s)
}

// linkOf returns the metrics state of id, creating it; the caller holds n.lmu.
func (n *Node) linkOf(id peer.ID) *peerLink {
	pl := n.stats[id]
	if pl == nil {
		pl = new(peerLink)
		n.stats[id] = pl
	}
	return pl
}

// linkDown records a PeerDown for the stability figures: a lost connection, or
// with resync a session restarted over a healthy one (spec rule L).
func (n *Node) linkDown(id peer.ID, resync bool) {
	n.lmu.Lock()
	defer n.lmu.Unlock()
	pl := n.linkOf(id)
	if resync {
		pl.stab.resync(time.Now())
		return
	}
	pl.stab.down(time.Now())
	pl.probe = probeView{} // the connection is gone with its probe: what it showed is not the link's
}

// Links returns the link to each peer announced with a PeerUp, sorted by peer.
// It only reads what the sampler measured once a second, so it is cheap to
// poll, but a relayed link upgraded by hole punching shows up within a second
// or so. No address is part of a Link.
func (n *Node) Links() []transport.Link {
	ups := n.upLinks()
	now := time.Now()
	links := make([]transport.Link, 0, len(ups))
	n.lmu.Lock()
	defer n.lmu.Unlock()
	for _, u := range ups {
		id := u.id
		name, ok := peerName(id)
		pl := n.stats[id]
		if !ok || pl == nil || pl.path.kind == 0 {
			continue
		}
		rtt, dev, age := pl.rtt.get(now)
		l := transport.Link{
			Peer: name, Kind: pl.path.kind, PathSince: pl.path.since,
			Upgrades: pl.path.upgrades, Fallbacks: pl.path.fallbacks,
			RTT: rtt, RTTVar: dev, RTTAge: age,
			UpSince: pl.stab.upSince, Downs10m: pl.stab.downsWithin(now),
			DownsTotal: pl.stab.total, LastDown: pl.stab.last,
			Resyncs10m: pl.stab.resyncsWithin(now), Resyncs: pl.stab.resyncTotal,
		}
		l.ProbeLevel = pl.probe.level
		if pl.probe.hasLoss {
			l.ProbeLoss, l.LossScope = new(pl.probe.loss), pl.scope
		}
		links = append(links, l)
	}
	slices.SortFunc(links, func(a, b transport.Link) int { return cmp.Compare(a.Peer, b.Peer) })
	return links
}

// upLink is a peer announced with a PeerUp, and the stream its session runs on at
// that instant: nil while a lost stream is being replaced.
type upLink struct {
	id  peer.ID
	out network.Stream
}

// upLinks returns the peers announced with a PeerUp.
func (n *Node) upLinks() []upLink {
	n.mu.Lock()
	defer n.mu.Unlock()
	var ups []upLink
	for _, ps := range n.peers {
		if ps.up {
			ups = append(ups, upLink{id: ps.id, out: ps.out})
		}
	}
	return ups
}

// stepViews holds what the probes showed in one sampler step, by connection: the peers behind one
// relay share its connection, which is looked at once. A nil map looks every time.
type stepViews map[string]probeView

func (v stepViews) of(id string, pc *probeConn) probeView {
	if pv, ok := v[id]; ok {
		return pv
	}
	pv := pc.view()
	if v != nil {
		v[id] = pv
	}
	return pv
}

// probeLink reads the connections to id, whose session runs on the stream out (nil:
// none at the moment). It takes no lock of ours. The best connection tells the
// path; the figures are those of the connection that carries the session (Annex
// A.1), and behind a relay only the hop to the relay can be seen. A session on a
// connection the probe does not run on (TCP, or a circuit while a direct
// connection waits for the session to move to it) has no figure.
func (n *Node) probeLink(id peer.ID, out network.Stream, views stepViews) linkSample {
	conns := n.h.Network().ConnsToPeer(id)
	s := linkSample{id: id, out: out, kind: bestKind(conns)}
	if s.kind == 0 || out == nil {
		return s
	}
	var c network.Conn
	switch sc, relayed := out.Conn(), s.kind == transport.LinkRelayed; {
	case !relayed && !isCircuit(sc.RemoteMultiaddr()):
		if st, ok := quicStats(sc); ok {
			c, s.scope = sc, transport.LossPath
			s.srtt, s.dev, s.rttFromQUIC = st.SmoothedRTT, st.MeanDeviation, true
		}
	case relayed:
		if rc := n.relayConn([]network.Conn{sc}); rc != nil {
			if _, ok := quicStats(rc); ok {
				c, s.scope = rc, transport.LossFirstHop
			}
		}
	}
	if c == nil {
		return s
	}
	s.quic = true
	if pc := n.probes.of(c.ID()); pc != nil {
		s.probe = views.of(c.ID(), pc)
		if s.kind == transport.LinkRelayed {
			s.probe.level = transport.QualityUnknown // the hop to the relay is not the link
		}
	}
	return s
}

// applyLinkLocked folds a sample taken before now into pl and starts a ping if
// a relayed link is due one. The caller holds n.lmu.
func (n *Node) applyLinkLocked(id peer.ID, pl *peerLink, now time.Time, s linkSample) {
	if s.kind == 0 {
		return
	}
	pl.path.observe(now, s.kind)
	pl.quicRTT = s.rttFromQUIC
	switch {
	case s.kind != transport.LinkRelayed && s.probe.hasRTT:
		// The probe measures every ECHO; QUIC's smoothed RTT samples only the newly largest
		// acknowledged packet, which under reordering favours the fast ones (RFC 9002 §5.1).
		pl.rtt.set(now.Add(-s.probe.rttAge), s.probe.rtt, s.probe.rttDev)
	case s.rttFromQUIC:
		pl.rtt.set(now, s.srtt, s.dev)
	}
	// A new connection has a probe of its own, which starts unknown: the view
	// is the connection's, whatever the last one showed.
	pl.probe, pl.scope = s.probe, s.scope
	if s.kind == transport.LinkRelayed && !pl.pinging && now.Sub(pl.lastPing) >= pingEvery {
		pl.lastPing = now
		n.wg.Go(func() { n.measureRTT(id) })
	}
}

// relayConn returns our connection to the relay that carries the circuit
// connection in conns, or nil.
func (n *Node) relayConn(conns []network.Conn) network.Conn {
	for _, c := range conns {
		if !isCircuit(c.RemoteMultiaddr()) {
			continue
		}
		v, err := c.RemoteMultiaddr().ValueForProtocol(ma.P_P2P)
		if err != nil {
			continue
		}
		rid, err := peer.Decode(v)
		if err != nil {
			continue
		}
		if rcs := n.h.Network().ConnsToPeer(rid); len(rcs) > 0 {
			return rcs[0]
		}
	}
	return nil
}

// quicStats returns the QUIC statistics of c, if it is a QUIC connection.
func quicStats(c network.Conn) (quic.ConnectionStats, bool) {
	var qc *quic.Conn
	if !c.As(&qc) || qc == nil {
		return quic.ConnectionStats{}, false
	}
	return qc.ConnectionStats(), true
}

// sampleLinks refreshes the figures of every link once a second. It is the
// only writer of the loss window and the path history, so they do not depend
// on anyone polling Links.
func (n *Node) sampleLinks() {
	t := time.NewTicker(sampleEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			steps := n.applyLinks(n.readLinks(make(stepViews)))
			if n.opts.OnLinkStep != nil {
				for _, st := range steps {
					n.opts.OnLinkStep(st)
				}
			}
		case <-n.ctx.Done():
			return
		}
	}
}

// readLinks is the first half of a sampler step: it looks at the links of the peers that are up,
// without any lock of ours, through views.
func (n *Node) readLinks(views stepViews) []linkSample {
	ups := n.upLinks()
	samples := make([]linkSample, len(ups))
	for i, u := range ups {
		samples[i] = n.probeLink(u.id, u.out, views)
	}
	return samples
}

// applyLinks is the second half of a sampler step: it folds the samples into the links' state and
// returns what the step saw of each link it updated. A sample whose session ended or moved to
// another stream since it was read is dropped: the next step sees the new one.
func (n *Node) applyLinks(samples []linkSample) []LinkStep {
	now := time.Now()
	var steps []LinkStep
	n.mu.Lock() // the session a sample was read for is still the peer's (lock order: mu, then lmu)
	defer n.mu.Unlock()
	n.lmu.Lock()
	defer n.lmu.Unlock()
	for _, s := range samples {
		ps, pl := n.peers[s.id], n.stats[s.id]
		if ps == nil || !ps.up || ps.out != s.out || pl == nil {
			continue
		}
		n.applyLinkLocked(s.id, pl, now, s)
		steps = append(steps, pl.step(s.id, now))
	}
	n.forgetLinksLocked(samples, now)
	return steps
}

// forgetLinksLocked drops the state of the peers that no sample of up is of and have
// been gone for longer than the window of the outage counters. The caller
// holds n.lmu.
func (n *Node) forgetLinksLocked(up []linkSample, now time.Time) {
	maps.DeleteFunc(n.stats, func(id peer.ID, pl *peerLink) bool {
		return !slices.ContainsFunc(up, func(s linkSample) bool { return s.id == id }) && now.Sub(pl.stab.seen) > transport.DownsWindow
	})
}

// bestKind returns the best kind among conns, or zero if there is none.
func bestKind(conns []network.Conn) transport.LinkKind {
	var best transport.LinkKind
	for _, c := range conns {
		kind := kindOf(c.RemoteMultiaddr())
		if best == 0 || kind < best {
			best = kind
		}
	}
	return best
}

// kindOf returns the kind of a connection to the remote address a.
func kindOf(a ma.Multiaddr) transport.LinkKind {
	switch {
	case isCircuit(a):
		return transport.LinkRelayed
	case manet.IsPrivateAddr(a) || manet.IsIPLoopback(a) || manet.IsIP6LinkLocal(a):
		return transport.LinkLAN
	default:
		return transport.LinkDirect
	}
}

// measureRTT pings id once and, unless QUIC already tells the RTT, folds the
// result into the link's estimator. It does nothing if the peer is unknown or
// a ping is already running.
func (n *Node) measureRTT(id peer.ID) {
	if !n.claimPing(id) {
		return
	}
	ctx, cancel := context.WithTimeout(n.ctx, pingTimeout)
	defer cancel()
	var res ping.Result
	select {
	case res = <-ping.Ping(ctx, n.h, id):
	case <-ctx.Done():
	}
	n.lmu.Lock()
	defer n.lmu.Unlock()
	pl := n.stats[id]
	if pl == nil {
		return
	}
	pl.pinging = false
	if res.Error == nil && res.RTT > 0 && !pl.quicRTT {
		pl.rtt.sample(time.Now(), res.RTT)
	}
}

// claimPing marks a ping of id as running, if id is known and none is.
func (n *Node) claimPing(id peer.ID) bool {
	n.lmu.Lock()
	defer n.lmu.Unlock()
	pl := n.stats[id]
	if pl == nil || pl.pinging {
		return false
	}
	pl.pinging, pl.lastPing = true, time.Now()
	return true
}

const (
	// pingTimeout bounds the round-trip measurement of a link.
	pingTimeout = 5 * time.Second
	// pingEvery is how often a relayed link, whose RTT QUIC cannot tell, is
	// pinged end to end.
	pingEvery = 10 * time.Second
	// sampleEvery is the period of the link sampler.
	sampleEvery = time.Second
)
