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
)

var _ transport.LinkSource = (*Node)(nil)

// peerLink is the metrics state of one peer, under Node.lmu. It outlives the
// peer's PeerDown so that the outage counters survive a reconnection; the
// sampler forgets it once the peer has been gone for [transport.DownsWindow].
type peerLink struct {
	rtt      rttEstimator
	loss     lossWindow
	path     pathTrack
	stab     stability
	scope    transport.LossScope // what loss measures
	connID   string              // the connection loss is read from
	quicRTT  bool                // the RTT comes from QUIC, so pings are not folded in
	lastPing time.Time           // when the last ping started
	pinging  bool
}

// linkSample is what one look at the connections to a peer shows. It is plain
// values, taken without any lock, and applied to a peerLink under Node.lmu.
type linkSample struct {
	kind transport.LinkKind // zero: no connection
	// The remaining fields are meaningful only if quic is set.
	quic       bool
	scope      transport.LossScope
	connID     string
	sent, lost uint64
	// srtt and dev are the QUIC round trip, only if rttFromQUIC is set.
	srtt, dev   time.Duration
	rttFromQUIC bool
}

// The lock order is Node.mu, then Node.lmu. lmu is a leaf: code under it never
// takes mu and never calls into libp2p or QUIC; probeLink does that before.

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
}

// Links returns the link to each peer announced with a PeerUp, sorted by peer.
// It only reads what the sampler measured once a second, so it is cheap to
// poll, but a relayed link upgraded by hole punching shows up within a second
// or so. No address is part of a Link.
func (n *Node) Links() []transport.Link {
	ids := n.upPeers()
	now := time.Now()
	links := make([]transport.Link, 0, len(ids))
	n.lmu.Lock()
	defer n.lmu.Unlock()
	for _, id := range ids {
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
		if v, ok := pl.loss.pct(now); ok {
			l.LossPct, l.LossScope = &v, pl.scope
		}
		links = append(links, l)
	}
	slices.SortFunc(links, func(a, b transport.Link) int { return cmp.Compare(a.Peer, b.Peer) })
	return links
}

// upPeers returns the peers announced with a PeerUp.
func (n *Node) upPeers() []peer.ID {
	n.mu.Lock()
	defer n.mu.Unlock()
	var ids []peer.ID
	for _, ps := range n.peers {
		if ps.up {
			ids = append(ids, ps.id)
		}
	}
	return ids
}

// probeLink reads the connections to id. It takes no lock of ours. The best
// direct connection tells the path; behind a relay only the hop to the relay
// can be seen.
func (n *Node) probeLink(id peer.ID) linkSample {
	conns := n.h.Network().ConnsToPeer(id)
	s := linkSample{kind: bestKind(conns)}
	if s.kind == 0 {
		return s
	}
	var c network.Conn
	if s.kind != transport.LinkRelayed {
		for _, d := range conns {
			if isCircuit(d.RemoteMultiaddr()) {
				continue
			}
			if st, ok := quicStats(d); ok {
				c, s.scope = d, transport.LossPath
				s.sent, s.lost = st.PacketsSent, st.PacketsLost
				s.srtt, s.dev, s.rttFromQUIC = st.SmoothedRTT, st.MeanDeviation, true
				break
			}
		}
	} else if rc := n.relayConn(conns); rc != nil {
		if st, ok := quicStats(rc); ok {
			c, s.scope = rc, transport.LossFirstHop
			s.sent, s.lost = st.PacketsSent, st.PacketsLost
		}
	}
	if c != nil {
		s.quic, s.connID = true, c.ID()
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
	if s.rttFromQUIC {
		pl.rtt.set(now, s.srtt, s.dev)
	}
	if s.quic {
		// Counters of another connection, or of another scope, do not continue
		// these: start the window again.
		if s.connID != pl.connID || s.scope != pl.scope {
			pl.loss = lossWindow{}
			pl.connID, pl.scope = s.connID, s.scope
		}
		pl.loss.observe(now, s.sent, s.lost)
	}
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
			ids := n.upPeers()
			samples := make([]linkSample, len(ids))
			for i, id := range ids {
				samples[i] = n.probeLink(id)
			}
			now := time.Now()
			n.lmu.Lock()
			for i, id := range ids {
				if pl := n.stats[id]; pl != nil { // gone since upPeers: leave it
					n.applyLinkLocked(id, pl, now, samples[i])
				}
			}
			n.forgetLinksLocked(ids, now)
			n.lmu.Unlock()
		case <-n.ctx.Done():
			return
		}
	}
}

// forgetLinksLocked drops the state of the peers that are not in up and have
// been gone for longer than the window of the outage counters. The caller
// holds n.lmu.
func (n *Node) forgetLinksLocked(up []peer.ID, now time.Time) {
	maps.DeleteFunc(n.stats, func(id peer.ID, pl *peerLink) bool {
		return !slices.Contains(up, id) && now.Sub(pl.stab.seen) > transport.DownsWindow
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
