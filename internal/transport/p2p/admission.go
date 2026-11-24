package p2p

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/x/rate"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/admit"
)

// admission is the node's resource manager: libp2p's, wrapped in the admission layer of spec §16.1 (block
// RF1, rules RF-1 to RF-6), whose counts are kept by the pure ledger of protocol/admit. Every method of
// network.ResourceManager is forwarded to libp2p's (the embedded interface), except two:
//
//   - OpenConnection charges an inbound connection to its origin before its handshake (RF-1, RF-3, RF-5), and
//     its scope's SetPeer tells the ledger what the handshake proved (RF-2, RF-6);
//   - VerifySourceAddress keeps libp2p's rule on the layer's own limiter (RF-6): libp2p's own derives from its
//     per-IP limiter, which admission switches off (resourceManagerWith).
type admission struct {
	network.ResourceManager
	// classify says what a peer is to the node: a member, a configured relay or a stranger. Set before the
	// host starts.
	classify func(peer.ID) admit.Class
	// observe, if not nil, is told of each opening and handshake (tests).
	observe func(admitEvent)
	verify  *rate.Limiter
	start   time.Time // the ledger's clock: time since start, monotonic

	mu     sync.Mutex
	ledger *admit.Ledger
}

// admitEvent is an opening (Peer empty) or a handshake the ledger decided on, for the tests.
type admitEvent struct {
	Dir     network.Direction
	Origin  string
	Peer    peer.ID
	Refused string // the reason, "libp2p" for a refusal of libp2p's own limits, or empty
	Pending int    // the origin's pending connections after the opening
}

func newAdmission(inner network.ResourceManager, cfg admit.Config) *admission {
	return &admission{
		ResourceManager: inner,
		classify:        func(peer.ID) admit.Class { return admit.Stranger },
		ledger:          admit.New(cfg),
		verify:          newVerifyLimiter(),
		start:           time.Now(),
	}
}

// now is the ledger's clock.
func (a *admission) now() admit.Nanos { return admit.Nanos(time.Since(a.start)) }

// nodeAdmitConfig is spec §16.1's admission for a node (RF-3 to RF-6).
func nodeAdmitConfig() admit.Config {
	return admit.Config{
		PerRelay: protocol.MaxPendingPerRelay, PerIP: protocol.MaxPendingPerIP,
		PerWide: protocol.MaxPendingPerIPv6WidePrefix, Pending: protocol.MaxPending,
		Deadline: admit.Nanos(protocol.PendingDeadline),
		Burst:    protocol.AdmissionBurst, Refill: admit.Nanos(protocol.AdmissionRefill),
		Members: protocol.MaxMemberConnections, Share: protocol.MemberShareConnections,
		PerMember: protocol.MaxConnectionsPerPeer,
		Relays:    protocol.MaxRelays * protocol.ConnectionsPerRelay, PerRelayConns: protocol.ConnectionsPerRelay,
	}
}

// relayAdmitConfig is an inkwell relay's (RF-7): 64 connections per IP address until a peer of its Allow list
// proves itself, no deadline (a client of an open relay stays counted while it is connected), and the bucket.
func relayAdmitConfig() admit.Config {
	return admit.Config{
		PerIP: protocol.RelayConnectionsPerIP, Burst: protocol.AdmissionBurst, Refill: admit.Nanos(protocol.AdmissionRefill),
	}
}

// newVerifyLimiter is libp2p's QUIC source address rule (go-libp2p rcmgr, conn_limiter.go): an address is
// verified (a Retry) beyond a burst of 4 handshakes per IPv4 /32 or IPv6 /56 (32 per /48), refilled 1 per
// 10 s, never on loopback.
func newVerifyLimiter() *rate.Limiter {
	const rps = 0.1
	burst := protocol.MaxPendingPerIP / 2
	return &rate.Limiter{
		NetworkPrefixLimits: []rate.PrefixLimit{
			{Prefix: netip.MustParsePrefix("127.0.0.0/8")}, {Prefix: netip.MustParsePrefix("::1/128")},
		},
		SubnetRateLimiter: rate.SubnetLimiter{
			IPv4SubnetLimits: []rate.SubnetLimit{{PrefixLength: protocol.PendingIPv4PrefixBits, RPS: rps, Burst: burst}},
			IPv6SubnetLimits: []rate.SubnetLimit{
				{PrefixLength: protocol.PendingIPv6PrefixBits, RPS: rps, Burst: burst},
				{PrefixLength: protocol.PendingIPv6WidePrefixBits, RPS: rps, Burst: protocol.MaxPendingPerIPv6WidePrefix / 2},
			},
			GracePeriod: time.Minute,
		},
	}
}

// setRoster names the members whose shares the ledger reserves (RF-6), and returns, one entry per
// connection, the peers of the surplus connections to close for those shares to fit again.
func (a *admission) setRoster(ids []peer.ID) []peer.ID {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = string(id)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []peer.ID
	for _, c := range a.ledger.SetRoster(s) {
		out = append(out, peer.ID(a.ledger.Peer(c)))
	}
	return out
}

// OpenConnection implements network.ResourceManager: libp2p's limits first, then the ledger's. An inbound
// connection is charged to its origin (RF-1); an outbound one to none.
func (a *admission) OpenConnection(dir network.Direction, usefd bool, endpoint ma.Multiaddr) (network.ConnManagementScope, error) {
	o := originOf(endpoint)
	inner, err := a.ResourceManager.OpenConnection(dir, usefd, endpoint)
	if err != nil {
		if a.observe != nil {
			a.observe(admitEvent{Dir: dir, Origin: o.String(), Refused: "libp2p"})
		}
		return nil, err
	}
	d := admit.Outbound
	if dir == network.DirInbound {
		d = admit.Inbound
	}
	a.mu.Lock()
	now := a.now()
	id, err := a.ledger.Open(now, d, o)
	if a.observe != nil {
		ev := admitEvent{Dir: dir, Origin: o.String(), Refused: reasonText(err), Pending: a.ledger.Stats(now).PerOrigin[o]}
		defer a.observe(ev) // after the unlock
	}
	a.mu.Unlock()
	if err != nil {
		inner.Done()
		return nil, fmt.Errorf("p2p: connection from %s refused: %w", o, err)
	}
	return &admittedScope{ConnManagementScope: inner, a: a, id: id, dir: dir}, nil
}

// VerifySourceAddress implements network.ResourceManager on the layer's own limiter (RF-6).
func (a *admission) VerifySourceAddress(addr net.Addr) bool {
	ap, err := netip.ParseAddrPort(addr.String())
	if err != nil {
		return true
	}
	return !a.verify.Allow(ap.Addr())
}

func reasonText(err error) string {
	if err == nil {
		return ""
	}
	return admit.ReasonOf(err).String()
}

// admittedScope is a connection's scope: libp2p's, with the ledger told of its handshake and of its end.
type admittedScope struct {
	network.ConnManagementScope
	a    *admission
	id   admit.ID
	dir  network.Direction
	done sync.Once
}

// SetPeer implements network.ConnManagementScope: libp2p's per-peer limits, then what the handshake proved to
// the ledger (RF-2, RF-6). An inbound connection reaches it after its handshake; an outbound one with the
// dialled peer, which the handshake confirms.
func (s *admittedScope) SetPeer(p peer.ID) error {
	if err := s.ConnManagementScope.SetPeer(p); err != nil {
		return err
	}
	class := s.a.classify(p)
	s.a.mu.Lock()
	err := s.a.ledger.Identify(s.a.now(), s.id, string(p), class)
	s.a.mu.Unlock()
	if s.a.observe != nil {
		s.a.observe(admitEvent{Dir: s.dir, Peer: p, Refused: reasonText(err)})
	}
	if err != nil {
		return fmt.Errorf("p2p: connection of %s refused: %w", p, err)
	}
	return nil
}

// Done implements network.ResourceScopeSpan: the ledger frees what the connection held, once.
func (s *admittedScope) Done() {
	s.done.Do(func() {
		s.a.mu.Lock()
		s.a.ledger.Close(s.a.now(), s.id)
		s.a.mu.Unlock()
		s.ConnManagementScope.Done()
	})
}

// originOf is what an inbound connection at endpoint is charged to (RF-1): for a circuit, its relay, named by
// the relay's authenticated PeerID (the /p2p/<relay> before /p2p-circuit), never by the relay's IP;
// otherwise its IPv4 /32 or IPv6 /56. An endpoint with neither shares one origin with every other such.
func originOf(endpoint ma.Multiaddr) admit.Origin {
	if relay, ok := circuitRelay(endpoint); ok {
		return admit.RelayOrigin(relay)
	}
	ip, err := manet.ToIP(endpoint)
	if err != nil {
		return admit.Origin{}
	}
	a, _ := netip.AddrFromSlice(ip)
	return admit.IPOrigin(a)
}

// circuitRelay returns the PeerID of the relay of a circuit address: the last /p2p before /p2p-circuit.
func circuitRelay(a ma.Multiaddr) (string, bool) {
	var relay string
	for _, c := range a {
		switch c.Code() {
		case ma.P_P2P:
			relay = c.Value()
		case ma.P_CIRCUIT:
			return relay, relay != ""
		}
	}
	return "", false
}

// classOf is what p is to the node (RF-2): a configured relay, a member (the membership list, never the
// invitation window), or a stranger.
func (n *Node) classOf(p peer.ID) admit.Class {
	switch {
	case n.relays[p].ID != "":
		return admit.Relay
	case n.member(p):
		return admit.Member
	}
	return admit.Stranger
}

// shedSurplus closes, for each entry of peers, that peer's newest open connection not closed yet: the surplus
// connections a roster change leaves above the members' room (RF-6, SetRoster).
func (n *Node) shedSurplus(peers []peer.ID) {
	shed := map[network.Conn]bool{}
	for _, p := range peers {
		var newest network.Conn
		for _, c := range n.h.Network().ConnsToPeer(p) {
			if !shed[c] && !c.IsClosed() && (newest == nil || c.Stat().Opened.After(newest.Stat().Opened)) {
				newest = c
			}
		}
		if newest != nil {
			shed[newest] = true
			_ = newest.Close()
		}
	}
}
