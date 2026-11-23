package p2p

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/oioio-space/inkwell/protocol/directup"
)

// punchGate holds the two choke points every hole punch goes through, whoever initiates (plan §3.3a,
// rules H-5, H-12, H-16): the DCUtR address filter, called in both roles of every round,
// and the connection gater's check of every non-relay destination of a peer reachable only through a
// relay. A round takes a node-wide slot (FIFO, leased) and a token of its peer's round bucket; a
// destination needs its peer to hold a slot, and takes a token of the peer's destination bucket and one
// of the grant's allowance. A peer that is not wanted gets nothing: no address, no ticket, no dial.
type punchGate struct {
	// net is the host's network, set once the host exists; wanted is Node.member (Options.Admit): a
	// stranger, a non-member or a peer admitted only by the invitation window, is never wanted. The
	// node never calls the gate under Node.mu.
	net    atomic.Value // networkPeers
	wanted func(peer.ID) bool
	// announced returns the addresses a peer announced through identify (Node.announced): those the
	// gater's on-demand grant admits.
	announced func(peer.ID) []ma.Multiaddr

	mu    sync.Mutex
	slots *directup.Slots
	peers map[peer.ID]*punchPeer
	// changed is closed and replaced whenever the slots may have changed, to wake the rounds and the
	// upgraders that wait for one.
	changed chan struct{}
	closed  chan struct{}
}

// networkPeers is the part of the host's network the gate reads.
type networkPeers interface {
	ConnsToPeer(peer.ID) []network.Conn
}

// punchPeer is what the gate knows of one peer.
type punchPeer struct {
	budgets directup.Budgets
	// admitted are the non-relay addresses the peer's current grant may dial: those its rounds' filter
	// gave, and those the node's own call chose from what the peer announced.
	admitted   []ma.Multiaddr
	grantUntil time.Time
	// rounds counts the rounds admitted with the peer, dests its destinations let through, since the
	// gate first saw it: the upgrader classes its calls by their growth (H-7).
	rounds, dests int
	// demand is owned by the connect call that may make a preliminary dial.
	demand *punchDemand
}

type punchDemand struct {
	dialing bool
	until   time.Time
}

// A round owns one reference to the lease under which it was admitted.
type punchRound struct {
	until time.Time
	local []ma.Multiaddr
}

func newPunchGate(wanted func(peer.ID) bool, now time.Time) *punchGate {
	return &punchGate{
		wanted:  wanted,
		slots:   directup.NewSlots(now),
		peers:   make(map[peer.ID]*punchPeer),
		changed: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

// peerLocked returns the gate's state of p, made on first use.
func (g *punchGate) peerLocked(p peer.ID, now time.Time) *punchPeer {
	pp := g.peers[p]
	if pp == nil {
		pp = &punchPeer{budgets: directup.NewBudgets(now)}
		g.peers[p] = pp
	}
	return pp
}

// signalLocked wakes everything that waits for the slots to change.
func (g *punchGate) signalLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

// beginRound admits one exchange. A round giving up its 20 s wait keeps its
// FIFO ticket (H-5). The caller releases an admitted round on every exit.
func (g *punchGate) beginRound(ctx context.Context, p peer.ID, addrs []ma.Multiaddr) *punchRound {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if ctx.Err() != nil {
		return nil
	}
	if !g.wanted(p) {
		return nil
	}
	if g.peerLocked(p, now).budgets.Rounds.Tokens(now) == 0 {
		return nil
	}
	deadline := now.Add(directup.RoundWait)
	until := g.askLocked(p, now)
	for until.IsZero() {
		if !g.waitLocked(ctx, deadline, now) {
			g.slots.GiveUp(string(p), time.Now())
			return nil
		}
		now = time.Now()
		if !g.wanted(p) || g.peerLocked(p, now).budgets.Rounds.Tokens(now) == 0 {
			return nil
		}
		until = g.askLocked(p, now)
	}
	pp := g.peers[p]
	pp.budgets.Rounds.Take(now)
	pp.rounds++
	return &punchRound{until: until, local: selectAddrs(addrs)}
}

// filterRemote admits the addresses conveyed by this round under its own lease.
func (g *punchGate) filterRemote(p peer.ID, round *punchRound, addrs []ma.Multiaddr) []ma.Multiaddr {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	pp := g.peers[p]
	if !g.wanted(p) || !now.Before(round.until) || pp == nil || pp.grantUntil != round.until {
		return nil
	}
	out := selectAddrs(addrs)
	pp.admitted = append(pp.admitted, out...)
	return out
}

// askLocked retains one reference to p's current grant. A grant awarded
// asynchronously from the FIFO also starts with an empty address set.
func (g *punchGate) askLocked(p peer.ID, now time.Time) time.Time {
	if !g.slots.Ask(string(p), now) {
		return time.Time{}
	}
	until, _ := g.slots.Until(string(p), now)
	pp := g.peerLocked(p, now)
	if pp.grantUntil != until {
		pp.grantUntil = until
		pp.admitted = nil
		g.signalLocked()
	}
	return until
}

// waitLocked waits, mu released, until the slots change, the next lease or token is due, or deadline;
// it reports false at the deadline or once the gate is closed.
func (g *punchGate) waitLocked(ctx context.Context, deadline, now time.Time) bool {
	wake := deadline
	if next := g.slots.Next(now); !next.IsZero() && next.Before(wake) {
		wake = next
	}
	changed := g.changed
	g.mu.Unlock()
	t := time.NewTimer(time.Until(wake))
	select {
	case <-changed:
	case <-t.C:
	case <-g.closed:
	case <-ctx.Done():
	}
	t.Stop()
	g.mu.Lock()
	select {
	case <-g.closed:
		return false
	default:
	}
	return ctx.Err() == nil && time.Now().Before(deadline)
}

// ask retains a grant for the upgrader's call without waiting.
func (g *punchGate) ask(p peer.ID, announced []ma.Multiaddr) time.Time {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.wanted(p) {
		return time.Time{}
	}
	until := g.askLocked(p, now)
	if !until.IsZero() {
		pp := g.peers[p]
		pp.admitted = append(pp.admitted, selectAddrs(announced)...)
	}
	return until
}

func (g *punchGate) release(p peer.ID, until time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.releaseLocked(p, until, time.Now())
}

// While a reference is live, its grant cannot end early. After its deadline
// it no longer owns a reference, even if the same peer has a fresh grant.
func (g *punchGate) releaseLocked(p peer.ID, until, now time.Time) {
	pp := g.peers[p]
	if !now.Before(until) || pp == nil || pp.grantUntil != until {
		return
	}
	g.slots.Release(string(p), now)
	if !g.slots.Holds(string(p), now) {
		pp.grantUntil, pp.admitted = time.Time{}, nil
	}
	g.signalLocked()
}

// drop removes the ticket of p, which is no longer wanted or relayed.
func (g *punchGate) drop(p peer.ID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.slots.Drop(string(p))
	now := time.Now()
	if pp := g.peers[p]; pp != nil && pp.demand == nil && pp.budgets.Full(now) && !g.slots.Holds(string(p), now) {
		delete(g.peers, p) // full budgets carry nothing
	}
	g.signalLocked()
}

// next returns when the slots may change by themselves, and the channel closed when they change.
func (g *punchGate) next() (time.Time, <-chan struct{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.slots.Next(time.Now()), g.changed
}

// counts returns the rounds admitted with p and the destinations let through to it so far.
func (g *punchGate) counts(p peer.ID) (rounds, dests int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if pp := g.peers[p]; pp != nil {
		return pp.rounds, pp.dests
	}
	return 0, 0
}

// admitDial decides on a non-relay destination a of p, a wanted peer reachable only through a relay
// (the gater's choke point, H-12, H-16): only an admitted address of a held grant, within the peer's
// destination bucket and the grant's allowance.
//
// During the preliminary dial only, the call may acquire a grant on demand
// through the same FIFO. It retains that reference through all its rounds.
func (g *punchGate) admitDial(p peer.ID, a ma.Multiaddr) bool {
	var announced []ma.Multiaddr
	if g.announced != nil {
		announced = g.announced(p) // outside mu: it takes Node.mu
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.slots.Holds(string(p), now) && !g.askOnDemandLocked(p, announced, now) {
		return false
	}
	pp := g.peerLocked(p, now)
	if pp.demand != nil && pp.demand.dialing && pp.demand.until.IsZero() {
		g.askOnDemandLocked(p, announced, now)
	}
	if !now.Before(pp.grantUntil) || !slices.ContainsFunc(pp.admitted, a.Equal) || pp.budgets.Dests.Tokens(now) == 0 {
		return false
	}
	if !g.slots.Spend(string(p), now) {
		return false
	}
	pp.budgets.Dests.Take(now)
	pp.dests++
	return true
}

func (g *punchGate) beginDemand(p peer.ID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peerLocked(p, time.Now()).demand = &punchDemand{dialing: true}
}

func (g *punchGate) endPreliminaryDial(p peer.ID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peers[p].demand.dialing = false
}

func (g *punchGate) endDemand(p peer.ID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	pp := g.peers[p]
	g.releaseLocked(p, pp.demand.until, time.Now())
	pp.demand = nil
}

// askOnDemandLocked grants (or retains) a slot once per preliminary dial.
// A refusal leaves the peer's FIFO ticket in place.
func (g *punchGate) askOnDemandLocked(p peer.ID, announced []ma.Multiaddr, now time.Time) bool {
	pp := g.peers[p]
	if pp == nil || pp.demand == nil || !pp.demand.dialing || !pp.demand.until.IsZero() || pp.budgets.Dests.Tokens(now) == 0 {
		return false
	}
	until := g.askLocked(p, now)
	if until.IsZero() {
		return false
	}
	pp.admitted = append(pp.admitted, selectAddrs(announced)...)
	pp.demand.until = until
	return true
}

// relayedOnly reports whether the node holds connections to p and all of them are relay circuits.
func (g *punchGate) relayedOnly(p peer.ID) bool {
	nw, _ := g.net.Load().(networkPeers)
	if nw == nil {
		return false
	}
	conns := nw.ConnsToPeer(p)
	for _, c := range conns {
		if !isCircuit(c.RemoteMultiaddr()) {
			return false
		}
	}
	return len(conns) > 0
}

// close wakes every waiting round for good.
func (g *punchGate) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.closed:
	default:
		close(g.closed)
	}
}

// selectAddrs keeps at most three of addrs, as [directup.Select] says.
func selectAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	kinds := make([]directup.Addr, len(addrs))
	for i, a := range addrs {
		kinds[i] = addrKind(a)
	}
	idx := directup.Select(kinds)
	if len(idx) == 0 {
		return nil
	}
	out := make([]ma.Multiaddr, len(idx))
	for i, j := range idx {
		out[i] = addrs[j]
	}
	return out
}

// addrKind describes a for the selection: an address without an IP4 or IP6 component (dns, dns4, dns6,
// dnsaddr) is a name, refused before selection (H-12), though manet counts it as public.
func addrKind(a ma.Multiaddr) directup.Addr {
	k := directup.Addr{Circuit: isCircuit(a), Public: manet.IsPublicAddr(a), Name: true}
	ma.ForEach(a, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_IP4:
			k.Name = false
		case ma.P_IP6:
			k.IPv6, k.Name = true, false
		case ma.P_QUIC_V1:
			k.QUIC = true
		case ma.P_TCP:
			k.TCP = true
		}
		return true
	})
	return k
}
