package p2p

import (
	"cmp"
	"context"
	"crypto/ed25519"
	"fmt"
	"slices"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
)

// DefaultRelayPort is the port a relay listens on, over UDP (QUIC) and TCP,
// unless [RelayOptions.Listen] says otherwise.
const DefaultRelayPort = 7419

// Limits of the relay service. The libp2p defaults (2 minutes and 128 KiB per
// relayed connection) suit hole punching, not synchronising, so a circuit is
// unlimited here; the relay is bounded by how many it serves instead.
const (
	relayMaxReservations      = 256
	relayMaxCircuits          = 16
	relayMaxReservationsPerIP = 8
	// A reservation is a connection that must stay open, so the connection
	// manager is given room for all of them.
	relayConnLow  = 600
	relayConnHigh = 1024
)

// Tuning of the client side, for a small private network where a relay that
// does not answer is more likely to be briefly unreachable than gone.
const (
	// relayRetry is how long AutoRelay waits before asking a relay that
	// refused a reservation again; the default is an hour.
	relayRetry = 10 * time.Second
	// relayLookup spaces AutoRelay's looks for a relay to reserve on.
	relayLookup = 2 * time.Second
	// reserveTimeout bounds one attempt at a reservation, which includes
	// connecting to the relay.
	reserveTimeout = 15 * time.Second
	// relayMaxBackoff bounds the wait between two attempts at a relay, much
	// shorter than for a peer: a relay is one UDP handshake away, and a node
	// that cannot be reached through its relay cannot be reached at all.
	relayMaxBackoff = 10 * time.Second
	// minRefresh and maxRefresh bound the time between two reservations
	// that succeed.
	minRefresh = 10 * time.Second
	maxRefresh = 30 * time.Minute
)

// RelayOptions tunes [RunRelay]. The zero value listens on
// [DefaultRelayPort] and serves everyone.
type RelayOptions struct {
	// OnReady, if not nil, is called once the relay listens, with its
	// addresses, each ending with /p2p/<id>: what a node passes in
	// [Options.Relays].
	OnReady func(addrs []string)
	// extra is applied after the relay's own libp2p options; tests use it to
	// run on a simulated network.
	extra []libp2p.Option
	// Listen are the multiaddrs to listen on. Empty means QUIC on IPv4 and
	// IPv6 and TCP, all at [DefaultRelayPort].
	Listen []string
	// Allow, if not empty, restricts the relay to these peers: only they
	// may reserve a slot, and a circuit needs both ends to be listed. Empty
	// serves everyone.
	Allow []transport.PeerID
}

// RunRelay runs a circuit relay identified by key until ctx is done, and
// returns nil then. It relays opaque, end-to-end encrypted bytes between
// peers that cannot reach each other: it sees who talks to whom, never what.
//
// Circuits are not limited in time or volume; the relay serves 256 slots, 16
// circuits per peer and 8 slots per IP address.
func RunRelay(ctx context.Context, key ed25519.PrivateKey, o RelayOptions) error {
	priv, err := privateKey(key)
	if err != nil {
		return err
	}
	acl, err := newAllowList(o.Allow)
	if err != nil {
		return err
	}
	base, err := conngater.NewBasicConnectionGater(nil)
	if err != nil {
		return fmt.Errorf("p2p: relay connection gater: %w", err)
	}
	cm, err := connmgr.NewConnManager(relayConnLow, relayConnHigh)
	if err != nil {
		return fmt.Errorf("p2p: connection manager: %w", err)
	}
	listen := o.Listen
	if len(listen) == 0 {
		listen = []string{
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", DefaultRelayPort),
			fmt.Sprintf("/ip6/::/udp/%d/quic-v1", DefaultRelayPort),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", DefaultRelayPort),
		}
	}
	opts := []libp2p.Option{
		libp2p.Identity(priv),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
		// No SO_REUSEPORT: with it, a second Inkwell on the machine binds the same TCP port and
		// the kernel spreads incoming connections between the two, so handshakes reach the
		// wrong process. A busy port must fail the bind, so the caller falls back to a free one.
		libp2p.Transport(tcp.NewTCPTransport, tcp.DisableReuseport()),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DefaultMuxers,
		libp2p.ListenAddrStrings(listen...),
		libp2p.ConnectionManager(cm),
		libp2p.ConnectionGater(gate{BasicConnectionGater: base, admit: func(peer.ID) bool { return true }}),
		// A relay is a public server: no relay of its own, no reachability probe.
		libp2p.DisableRelay(),
		libp2p.ForceReachabilityPublic(),
		libp2p.DisableMetrics(),
	}
	h, err := libp2p.New(append(opts, o.extra...)...)
	if err != nil {
		_ = cm.Close()
		return fmt.Errorf("p2p: start relay: %w", err)
	}
	defer func() { _ = h.Close() }()

	res := relay.DefaultResources()
	res.Limit = nil
	res.MaxReservations = relayMaxReservations
	res.MaxCircuits = relayMaxCircuits
	res.MaxReservationsPerIP = relayMaxReservationsPerIP
	ropts := []relay.Option{relay.WithResources(res)}
	if acl != nil {
		ropts = append(ropts, relay.WithACL(acl))
	}
	svc, err := relay.New(h, ropts...)
	if err != nil {
		return fmt.Errorf("p2p: start relay service: %w", err)
	}
	defer func() { _ = svc.Close() }()

	if o.OnReady != nil {
		addrs, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()})
		if err != nil {
			return fmt.Errorf("p2p: relay addresses: %w", err)
		}
		out := make([]string, len(addrs))
		for i, a := range addrs {
			out[i] = a.String()
		}
		o.OnReady(out)
	}
	<-ctx.Done()
	return nil
}

// allowList is the relay's access control: the listed peers only.
type allowList map[peer.ID]struct{}

// newAllowList returns the ACL for ids, or nil, no restriction, if there are none.
func newAllowList(ids []transport.PeerID) (allowList, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	l := make(allowList, len(ids))
	for _, id := range ids {
		p, ok := parseID(id)
		if !ok {
			return nil, fmt.Errorf("p2p: allowed peer %q is not a peer id", id)
		}
		l[p] = struct{}{}
	}
	return l, nil
}

func (l allowList) has(p peer.ID) bool {
	_, ok := l[p]
	return ok
}

// AllowReserve implements [relay.ACLFilter].
func (l allowList) AllowReserve(p peer.ID, _ ma.Multiaddr) bool { return l.has(p) }

// AllowConnect implements [relay.ACLFilter].
func (l allowList) AllowConnect(src peer.ID, _ ma.Multiaddr, dest peer.ID) bool {
	return l.has(src) && l.has(dest)
}

// privateKey converts key to the libp2p form.
func privateKey(key ed25519.PrivateKey) (crypto.PrivKey, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("p2p: private key is %d bytes, want %d", len(key), ed25519.PrivateKeySize)
	}
	priv, err := crypto.UnmarshalEd25519PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("p2p: private key: %w", err)
	}
	return priv, nil
}

// parseRelays parses the relays of Options, merging the addresses of a relay
// given more than once.
func parseRelays(addrs []string) (map[peer.ID]peer.AddrInfo, error) {
	relays := make(map[peer.ID]peer.AddrInfo, len(addrs))
	for _, s := range addrs {
		ai, err := peer.AddrInfoFromString(s)
		if err != nil {
			return nil, fmt.Errorf("p2p: relay %q: %w", s, err)
		}
		if len(ai.Addrs) == 0 {
			return nil, fmt.Errorf("p2p: relay %q has no address to dial", s)
		}
		cur := relays[ai.ID]
		cur.ID = ai.ID
		cur.Addrs = append(cur.Addrs, ai.Addrs...)
		relays[ai.ID] = cur
	}
	return relays, nil
}

// relayList returns the relays sorted by ID, so that everything derived from
// them is stable.
func relayList(relays map[peer.ID]peer.AddrInfo) []peer.AddrInfo {
	out := make([]peer.AddrInfo, 0, len(relays))
	for _, ai := range relays {
		out = append(out, ai)
	}
	slices.SortFunc(out, func(a, b peer.AddrInfo) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// circuitPrefixes returns, for each address of each relay, the multiaddr
// <relay address>/p2p/<relay>/p2p-circuit: a peer reachable through that relay
// is dialed at this prefix, whatever network it is on.
func circuitPrefixes(relays map[peer.ID]peer.AddrInfo) []ma.Multiaddr {
	var out []ma.Multiaddr
	for _, ai := range relayList(relays) {
		suffix := ma.StringCast("/p2p/" + ai.ID.String() + "/p2p-circuit")
		for _, a := range ai.Addrs {
			out = append(out, a.Encapsulate(suffix))
		}
	}
	return out
}

// reachabilityOptions returns the libp2p options that make the node reachable
// through relays, none if there are no relays.
//
// A desktop is nearly always behind a NAT, and AutoNAT v2 needs several public
// servers that a small private network lacks, so the node believes it is
// private and reserves a slot on the relays at once. Port mapping stays off.
func reachabilityOptions(relays map[peer.ID]peer.AddrInfo, o Options) []libp2p.Option {
	if len(relays) == 0 {
		return nil
	}
	opts := []libp2p.Option{
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(
			relayList(relays),
			autorelay.WithBackoff(relayRetry),
			autorelay.WithMinInterval(relayLookup),
		),
		libp2p.ForceReachabilityPrivate(),
	}
	if !o.RelayOnly {
		opts = append(opts, libp2p.EnableHolePunching())
	}
	if o.RelayOnly || o.rewrite != nil {
		opts = append(opts, libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
			if o.rewrite != nil {
				addrs = o.rewrite(addrs)
			}
			if o.RelayOnly {
				addrs = slices.DeleteFunc(slices.Clone(addrs), func(a ma.Multiaddr) bool { return !isCircuit(a) })
			}
			return addrs
		}))
	}
	return opts
}

// keepRelay keeps a connection to the relay and a reservation on it until the
// node is closed: the reservation is what lets peers reach the node through
// the relay, from any network. AutoRelay does the same, but its retries after
// a long outage are slow and not tied to the network coming back; this loop
// retries every few seconds, at once when the node's own addresses change, and
// makes the wanted peers retry as soon as a reservation is back.
//
// down is signaled when the connection to the relay is lost.
func (n *Node) keepRelay(ai peer.AddrInfo, down <-chan struct{}) {
	backoff := minBackoff
	healthy := false
	for {
		n.mu.Lock()
		kick := n.kick
		n.mu.Unlock()

		n.forgetDialFailures(ai.ID)
		res, ok, err := n.reserve(ai)
		if !ok {
			return
		}

		var wait time.Duration
		n.setReserved(ai.ID, err == nil)
		if err != nil {
			healthy = false
			wait = jitter(backoff)
			backoff = min(backoff*2, relayMaxBackoff)
		} else {
			if !healthy {
				n.kickAll() // peers wanted through this relay are reachable now
			}
			healthy, backoff = true, minBackoff
			wait = max(minRefresh, min(time.Until(res.Expiration)/2, maxRefresh))
		}

		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-kick:
			backoff = minBackoff
		case <-down:
			healthy = false
			n.setReserved(ai.ID, false)
		case <-n.ctx.Done():
			t.Stop()
			return
		}
		t.Stop()
	}
}

// setReserved records whether relay holds a slot for this node.
func (n *Node) setReserved(relay peer.ID, held bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if held {
		n.reserved[relay] = true
	} else {
		delete(n.reserved, relay)
	}
}

// reserve asks the relay for a slot. It returns with ok false if the node is
// closed meanwhile: client.Reserve does not give up on its own when its context
// ends, only when the host closes, so it is left to finish in the background.
func (n *Node) reserve(ai peer.AddrInfo) (*client.Reservation, bool, error) {
	type result struct {
		res *client.Reservation
		err error
	}
	ctx, cancel := context.WithTimeout(n.ctx, reserveTimeout)
	defer cancel()
	ch := make(chan result, 1)
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return nil, false, nil
	}
	n.wg.Go(func() {
		res, err := client.Reserve(ctx, n.h, ai)
		ch <- result{res, err}
	})
	n.mu.Unlock()
	select {
	case r := <-ch:
		return r.res, true, r.err
	case <-n.ctx.Done():
		return nil, false, nil
	}
}
