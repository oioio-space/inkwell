// Package p2p is a [transport.Transport] over libp2p.
//
// A [Node] listens on QUIC-v1 (TLS 1.3 built in) and on TCP with Noise, as a
// fallback; WebTransport, WebRTC and WebSocket are left out because they
// serve browsers and only add attack surface. Identity is an Ed25519 key, and
// the [transport.PeerID] of a peer is its libp2p peer ID, so a connection is
// authenticated end to end: a peer cannot claim another's ID.
//
// Each side writes on one stream it opened itself and never reads from it
// (a one-way use of libp2p's two-way streams), and reads the streams the
// other side opened (the model of go-libp2p-pubsub). There is
// thus no arbitration when both peers open at once, messages keep their
// order per sender, and QUIC/yamux flow control backpressures a sender whose
// receiver is slow. Payloads travel as length-prefixed frames (see [frame.Write]).
//
// A peer is announced with a PeerUp once the outbound stream is open and
// [Options.Admit] accepts it, and with a single PeerDown once the last
// outbound stream is gone and the peer is no longer connected. When a
// connection is replaced by another one to the same peer, the stream is
// reopened without any event.
//
// # Bulk lane
//
// [Node.Bulk] is a second [transport.Transport] on its own protocol,
// [ProtocolBulk]: the same peers, announced at the same time, but separate
// streams and queues, so that large transfers (package blob) never hold back
// a message of the sync protocol.
//
// # Reachability
//
// With [Options.Relays], a node reserves a slot on a relay (see [RunRelay]),
// which makes it reachable from any network. [Node.Want] keeps chosen peers
// connected: it dials their last known addresses, then the address through the
// relay, which does not depend on the peer's current network, retrying with a
// backoff and at once when the node's own addresses change. A relayed link is
// upgraded to a direct one by hole punching, and the sync stream follows it
// without any event or lost frame ([Node.Links] tells which link a peer is on).
// With [Options.RelayOnly], a node never exposes its address: it accepts and
// dials circuits only.
//
//	n, err := p2p.New(ctx, key, p2p.Options{Relays: []string{relayAddr}})
//	if err != nil { /* … */ }
//	defer n.Close()
//	n.Want(map[transport.PeerID][]string{peer: lastKnownAddrs})
//	err = n.Connect(ctx, peer, addrs) // or dial by hand; addrs come from the peer's Addrs
package p2p

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// ProtocolSync is the libp2p protocol of the unidirectional sync streams.
const ProtocolSync = "/inkwell/sync/1.0.0"

const (
	// memberTag protects the connections of admitted peers from the
	// connection manager.
	memberTag = "member"

	// Resource limits (see newResourceManager).
	maxPeers       = 64
	maxSyncStreams = 4
	memoryLimit    = 256 << 20
	fdLimit        = 1024

	// Connection manager watermarks: above high, connections of
	// unprotected peers are trimmed down to low.
	connLow  = 32
	connHigh = 64

	// writeTimeout is how long a write may go without progress before it
	// fails; it is re-armed each time a chunk of the frame is written, so the
	// time a frame takes is not bounded, only its stalls are.
	writeTimeout = frame.WriteIdle
	// openTimeout bounds opening the outbound stream.
	openTimeout = 15 * time.Second
	// upTimeout bounds how long an inbound stream waits for its peer to be
	// announced, before it is reset.
	upTimeout = 30 * time.Second
	// reorderWait bounds how long a new inbound stream waits for the one it
	// replaces to be read to its end, before it is read anyway. It is long
	// because a slow consumer holds the old stream up, and only a stalled
	// sender should make it give up.
	reorderWait = time.Minute
	// msgOverhead is charged to the byte budget on top of each payload, so
	// floods of tiny frames are bounded too.
	msgOverhead = 64
	// maxBuffered bounds the received bytes waiting for Recv, across peers.
	maxBuffered = 256 << 20
)

// hold is the state of a peer that was reset.
type hold struct {
	until, last time.Time
	backoff     time.Duration
}

const (
	// minResetBackoff is how long a peer that was reset is kept out; it
	// doubles at each reset within resetMemory of the last, up to maxBackoff.
	minResetBackoff = 2 * time.Second
	resetMemory     = 10 * time.Minute
)

// Options tunes a [Node]. The zero value listens on random ports and admits
// every peer.
type Options struct {
	// Admit, if not nil, says whether a peer may connect and be announced.
	// It is called when a connection is authenticated, when a peer connects
	// and for every inbound stream, from several goroutines, so it must be
	// fast, safe for concurrent use and must not call back into the Node.
	// Nil admits everyone, which is for tests only.
	Admit func(transport.PeerID) bool
	// extra is applied after the node's own libp2p options; tests use it to
	// run on a simulated network.
	extra []libp2p.Option
	// rewrite, if not nil, rewrites the advertised addresses; tests use it to
	// give a simulated node its public address.
	rewrite func([]ma.Multiaddr) []ma.Multiaddr
	// Relays are the relays to use, as multiaddrs ending in /p2p/<id>. With
	// at least one, the node reserves a slot on them so that peers can reach
	// it from any network, dials peers through them when it has no better
	// address, and tries to upgrade relayed links to direct ones (hole
	// punching) unless RelayOnly is set.
	Relays []string
	// Port is the UDP port of QUIC and the TCP port of the fallback. Zero
	// picks free ports.
	Port int
	// LAN starts the local network discovery (mDNS, service _inkwell._udp):
	// the node announces its identity and addresses on the local network and
	// lists the other nodes it sees (see [Node.Nearby]). If it cannot start,
	// the node works without it ([Node.LANError]).
	LAN bool
	// RelayOnly hides the node's address from the other people: it refuses
	// every connection that does not go through a relay (except to the
	// relays themselves), never dials a direct address and does not hole
	// punch. It needs at least one relay.
	RelayOnly bool
}

// queued is an event with the byte budget to release once Recv returns it.
type queued struct {
	ev   transport.Event
	cost int
}

// peerState is what a node knows about one libp2p peer. All fields are
// protected by Node.mu, except wsem.
type peerState struct {
	out  network.Stream // the stream we write on; nil while none is open
	wsem chan struct{}  // capacity 1: held while writing to out
	// inTail is closed when the last inbound stream of the peer is done; the
	// next one waits for it to keep the messages in order.
	inTail chan struct{}
	// The bulk lane's counterparts of out, wsem and inTail; see Bulk.
	bulkOut  network.Stream
	bulkSem  chan struct{}
	bulkTail chan struct{}
	name     transport.PeerID
	id       peer.ID
	// connected: libp2p has at least one connection to the peer.
	connected bool
	// admitted: Options.Admit accepted the peer when it connected.
	admitted bool
	// opening: a goroutine is opening the outbound stream.
	opening bool
	// up: PeerUp was announced and PeerDown has not been yet.
	up bool
	// upgrading: a goroutine is moving the outbound stream to a direct
	// connection.
	upgrading bool
}

// Node is a libp2p peer; it implements [transport.Transport].
type Node struct {
	h        host.Host
	ctx      context.Context // canceled by Close
	sub      event.Subscription
	addrSub  event.Subscription
	closeErr error
	cancel   context.CancelFunc
	done     chan struct{}
	wake     chan struct{} // capacity 1: "the queue may be non-empty"
	// chg is closed and replaced whenever a peerState changes, to wake the
	// goroutines that wait for one.
	chg    chan struct{}
	budget *frame.Budget
	peers  map[peer.ID]*peerState
	stop   func() bool // stops the close-on-context-cancel hook; under mu
	// relays are the configured relays, never peers; circuit has the address
	// prefix that leads through each of them. Both are read-only.
	relays    map[peer.ID]peer.AddrInfo
	relayDown map[peer.ID]chan struct{}
	// reserved are the relays that currently hold a slot for this node, under mu.
	reserved map[peer.ID]bool
	notifee  *network.NotifyBundle
	// found are the nodes seen by mDNS; under mu.
	found map[peer.ID]nearbyEntry
	// openTimer closes the stranger window, see SetOpen; under mu.
	openTimer *time.Timer
	// bulk is the bulk lane, once Bulk was called; under mu.
	bulk *Bulk
	// lobby is the handler of lobby requests.
	lobby atomic.Pointer[LobbyHandler]
	// lan is the mDNS service, if started.
	lan io.Closer
	// want is the set of peers to keep connected, and kick is closed and
	// replaced to make their dial loops retry at once. Both under mu.
	want map[peer.ID]*wanted
	held map[peer.ID]hold // the peers that were reset, see Reset; under mu
	kick chan struct{}

	lanErr    error
	openUntil time.Time // under mu

	local   transport.PeerID
	queue   []queued
	circuit []ma.Multiaddr
	opts    Options
	wg      sync.WaitGroup
	// dialWG counts the goroutines that dial, which Close waits for before it
	// closes the host: libp2p does not like to be closed under a dial.
	dialWG sync.WaitGroup

	closeOnce sync.Once
	mu        sync.Mutex
	// closed is set by Close, under mu.
	closed bool
}

var _ transport.Transport = (*Node)(nil)

// New starts a node identified by key. Canceling ctx closes the node.
//
// The node listens on QUIC (IPv4 and IPv6) and TCP at o.Port. Its resources
// are bounded: 64 peers' worth of connections, 4 sync streams per peer, 256
// MiB of memory. It fails if o.Relays has an invalid address, or if
// o.RelayOnly is set without a relay.
func New(ctx context.Context, key ed25519.PrivateKey, o Options) (*Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.Port < 0 || o.Port > 65535 {
		return nil, fmt.Errorf("p2p: invalid port %d", o.Port)
	}
	if o.Port != 0 {
		if err := portFree(o.Port); err != nil {
			return nil, err
		}
	}
	relays, err := parseRelays(o.Relays)
	if err != nil {
		return nil, err
	}
	if o.RelayOnly && len(relays) == 0 {
		return nil, errors.New("p2p: RelayOnly needs at least one relay")
	}
	priv, err := privateKey(key)
	if err != nil {
		return nil, err
	}
	local, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("p2p: peer id: %w", err)
	}
	localName, ok := peerName(local)
	if !ok {
		return nil, fmt.Errorf("p2p: peer id %q is not valid", local)
	}
	rm, err := newResourceManager()
	if err != nil {
		return nil, err
	}
	cm, err := connmgr.NewConnManager(connLow, connHigh)
	if err != nil {
		_ = rm.Close()
		return nil, fmt.Errorf("p2p: connection manager: %w", err)
	}
	base, err := conngater.NewBasicConnectionGater(nil)
	if err != nil {
		_ = rm.Close()
		_ = cm.Close()
		return nil, fmt.Errorf("p2p: connection gater: %w", err)
	}

	n := &Node{
		local:     localName,
		opts:      o,
		done:      make(chan struct{}),
		wake:      make(chan struct{}, 1),
		chg:       make(chan struct{}),
		budget:    frame.NewBudget(maxBuffered),
		peers:     make(map[peer.ID]*peerState),
		want:      make(map[peer.ID]*wanted),
		held:      make(map[peer.ID]hold),
		found:     make(map[peer.ID]nearbyEntry),
		kick:      make(chan struct{}),
		relays:    relays,
		relayDown: make(map[peer.ID]chan struct{}, len(relays)),
		reserved:  make(map[peer.ID]bool, len(relays)),
	}
	n.circuit = circuitPrefixes(relays)
	for id := range relays {
		n.relayDown[id] = make(chan struct{}, 1)
	}
	n.ctx, n.cancel = context.WithCancel(ctx)

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
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", o.Port),
			fmt.Sprintf("/ip6/::/udp/%d/quic-v1", o.Port),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", o.Port),
		),
		libp2p.ResourceManager(rm),
		libp2p.ConnectionManager(cm),
		libp2p.ConnectionGater(gate{BasicConnectionGater: base, admit: n.admitted, relays: relays, relayOnly: o.RelayOnly}),
		libp2p.DisableMetrics(),
	}
	opts = append(opts, reachabilityOptions(relays, o)...)
	h, err := libp2p.New(append(opts, o.extra...)...)
	if err != nil {
		n.cancel()
		_ = rm.Close()
		_ = cm.Close()
		return nil, fmt.Errorf("p2p: start libp2p: %w", err)
	}
	n.h = h

	sub, err := h.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		n.cancel()
		_ = h.Close()
		return nil, fmt.Errorf("p2p: subscribe: %w", err)
	}
	n.sub = sub
	addrSub, err := h.EventBus().Subscribe(new(event.EvtLocalAddressesUpdated))
	if err != nil {
		n.cancel()
		_ = sub.Close()
		_ = h.Close()
		return nil, fmt.Errorf("p2p: subscribe: %w", err)
	}
	n.addrSub = addrSub
	n.notifee = &network.NotifyBundle{
		ConnectedF:    func(_ network.Network, c network.Conn) { n.onConn(c) },
		DisconnectedF: func(_ network.Network, c network.Conn) { n.onDisconn(c) },
	}
	h.Network().Notify(n.notifee)
	h.SetStreamHandler(ProtocolSync, n.handleStream)
	h.SetStreamHandler(ProtocolBulk, n.handleBulkStream)
	h.SetStreamHandler(ProtocolLobby, n.handleLobbyStream)
	n.wg.Go(func() {
		for _, p := range h.Network().Peers() { // connected before the subscription
			n.onConnected(p)
		}
		for ev := range sub.Out() {
			if e, ok := ev.(event.EvtPeerConnectednessChanged); ok {
				n.onConnectedness(e)
			}
		}
	})
	n.wg.Go(func() {
		for range addrSub.Out() { // the network changed: retry now
			n.kickAll()
		}
	})
	for _, ai := range relayList(relays) {
		n.dialWG.Go(func() { n.keepRelay(ai, n.relayDown[ai.ID]) })
	}
	if o.LAN {
		n.lanErr = n.startLAN()
	}
	stop := context.AfterFunc(ctx, func() { _ = n.Close() })
	n.mu.Lock()
	n.stop = stop
	n.mu.Unlock()
	return n, nil
}

// newResourceManager returns the libp2p resource manager with the limits of a
// node: 64 peers (as inbound and outbound connections), 4 sync streams per
// peer and 256 MiB of memory. The other limits are libp2p's, scaled to that
// memory, and the bundled services (identify, ping, relay) keep their own.
func newResourceManager() (network.ResourceManager, error) {
	scaling := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&scaling)
	limits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Memory:        memoryLimit,
			Conns:         2 * maxPeers,
			ConnsInbound:  maxPeers,
			ConnsOutbound: maxPeers,
		},
		ProtocolPeer: map[protocol.ID]rcmgr.ResourceLimits{
			ProtocolSync: {
				Streams:         maxSyncStreams,
				StreamsInbound:  maxSyncStreams,
				StreamsOutbound: maxSyncStreams,
			},
			ProtocolBulk: {
				Streams:         maxBulkStreams,
				StreamsInbound:  maxBulkStreams,
				StreamsOutbound: maxBulkStreams,
			},
			ProtocolLobby: {
				Streams:         maxLobbyStreams,
				StreamsInbound:  maxLobbyStreams,
				StreamsOutbound: maxLobbyStreams,
			},
		},
	}
	cfg := limits.Build(scaling.Scale(memoryLimit, fdLimit))
	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(cfg))
	if err != nil {
		return nil, fmt.Errorf("p2p: resource manager: %w", err)
	}
	return rm, nil
}

// peerName returns the PeerID of p, if it is valid.
func peerName(p peer.ID) (transport.PeerID, bool) {
	name := transport.PeerID(p.String())
	return name, transport.ValidPeerID(name)
}

// admitted reports whether p may connect: Options.Admit accepts it, or the
// window opened by [Node.SetOpen] is open.
func (n *Node) admitted(p peer.ID) bool {
	name, ok := peerName(p)
	if !ok {
		return false
	}
	if n.isHeld(p) {
		return false
	}
	return n.opts.Admit == nil || n.opts.Admit(name) || n.isOpen()
}

// isOpen reports whether the stranger window is open.
func (n *Node) isOpen() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return time.Now().Before(n.openUntil)
}

// SetOpen lets in any authenticated peer, member or not, until the given time,
// instead of only the peers [Options.Admit] accepts: the invitation of a new
// person needs a stranger to connect and speak to the lobby (see
// [Node.HandleLobby]). A zero or past time closes the window, replacing any
// earlier one. When it closes, the connections of the peers that
// Options.Admit does not accept are closed. Without Options.Admit every peer
// is admitted already and the window has no effect.
func (n *Node) SetOpen(until time.Time) {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	if n.openTimer != nil {
		n.openTimer.Stop()
		n.openTimer = nil
	}
	n.openUntil = until
	if d := time.Until(until); d > 0 {
		n.openTimer = time.AfterFunc(d, n.sweep)
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	n.sweep()
}

// sweep closes the connections of the peers that are no longer admitted.
func (n *Node) sweep() {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.wg.Add(1)
	n.mu.Unlock()
	defer n.wg.Done()
	for _, p := range n.h.Network().Peers() {
		if _, relay := n.relays[p]; !relay && !n.admitted(p) {
			_ = n.h.Network().ClosePeer(p)
		}
	}
}

// Local returns the node's identity.
func (n *Node) Local() transport.PeerID { return n.local }

// Addrs returns the addresses the node listens on, each ending with
// /p2p/<id> so that it is enough to dial the node. Addresses of a wildcard
// listener are expanded to each local interface. With relays, the addresses
// through them are included once a slot is reserved, even when the relay is on
// a private or loopback address; in RelayOnly mode they are the only ones, so
// there may be none yet.
func (n *Node) Addrs() []string {
	addrs := n.h.Addrs()
	// AutoRelay only advertises relays on public addresses; a node whose relay
	// is on a LAN (or the loopback interface) is still reachable through it,
	// so the address is built here.
	n.mu.Lock()
	for id := range n.reserved {
		for _, c := range circuitPrefixes(map[peer.ID]peer.AddrInfo{id: n.relays[id]}) {
			if !slices.ContainsFunc(addrs, c.Equal) {
				addrs = append(addrs, c)
			}
		}
	}
	n.mu.Unlock()
	if len(addrs) == 0 { // none yet, in RelayOnly mode before a relay is reserved
		return nil
	}
	mas, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: n.h.ID(), Addrs: addrs})
	if err != nil {
		return nil
	}
	out := make([]string, len(mas))
	for i, ma := range mas {
		out[i] = ma.String()
	}
	return out
}

// Connect connects to peer at addrs and returns once a connection is
// established; the PeerUp follows when the sync stream is open. An address
// may end with /p2p/<id> but need not; one naming another peer is an error.
// It succeeds at once if the peer is already connected.
func (n *Node) Connect(ctx context.Context, to transport.PeerID, addrs []string) error {
	id, ok := parseID(to)
	switch {
	case !ok:
		return fmt.Errorf("p2p: connect: %q is not a peer id", to)
	case id == n.h.ID():
		return errors.New("p2p: connect: cannot connect to ourselves")
	}
	info := peer.AddrInfo{ID: id}
	for _, a := range addrs {
		ai, err := parseAddr(a, id)
		if err != nil {
			return fmt.Errorf("p2p: connect: address %q: %w", a, err)
		}
		info.Addrs = append(info.Addrs, ai.Addrs...)
	}
	if n.isClosed() {
		return transport.ErrClosed
	}
	if err := n.h.Connect(ctx, info); err != nil {
		return fmt.Errorf("p2p: connect to %s: %w", to, err)
	}
	return nil
}

// parseAddr parses a multiaddr that is, or leads to, the peer id.
func parseAddr(s string, id peer.ID) (*peer.AddrInfo, error) {
	ai, err := peer.AddrInfoFromString(s)
	if err != nil {
		// No /p2p/ part: the address is the peer's.
		var err2 error
		if ai, err2 = peer.AddrInfoFromString(strings.TrimSuffix(s, "/") + "/p2p/" + id.String()); err2 != nil {
			return nil, err2
		}
	}
	if ai.ID != id {
		return nil, fmt.Errorf("names peer %s, not %s", ai.ID, id)
	}
	return ai, nil
}

// Drop closes the connections to peer and forgets what is known about it,
// including that it was wanted (see [Node.Want]). It is for revocation: revoke
// the peer in Options.Admit first, or it may reconnect at once. The peer's
// PeerDown follows as for any lost link.
func (n *Node) Drop(who transport.PeerID) {
	id, ok := parseID(who)
	if !ok || id == n.h.ID() {
		return
	}
	n.unwant(id)
	_ = n.h.Network().ClosePeer(id)
	n.h.Peerstore().RemovePeer(id)
	n.h.Peerstore().ClearAddrs(id)
}

// Reset closes the connections to peer like [Node.Drop], but does not ban it:
// the peer stays wanted and known, and may reconnect, from either side, once
// a backoff has passed. The backoff is 2 s, doubling at each reset within ten
// minutes of the last one, up to a minute. It is for a peer that does not read
// what it is sent, such as a slow link; use Drop for a suspended or removed
// member. The peer's PeerDown follows.
func (n *Node) Reset(who transport.PeerID) {
	id, ok := parseID(who)
	if !ok || id == n.h.ID() {
		return
	}
	n.mu.Lock()
	h := n.held[id]
	now := time.Now()
	h.backoff = resetBackoff(h, now)
	h.until, h.last = now.Add(h.backoff), now
	n.held[id] = h
	n.mu.Unlock()
	_ = n.h.Network().ClosePeer(id)
}

// resetBackoff is the backoff of the next reset of a peer that was reset as h.
func resetBackoff(h hold, now time.Time) time.Duration {
	if h.backoff == 0 || now.Sub(h.last) > resetMemory {
		return minResetBackoff
	}
	return min(2*h.backoff, maxBackoff)
}

// isHeld reports whether p was reset and its backoff has not passed.
func (n *Node) isHeld(p peer.ID) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return time.Now().Before(n.held[p].until)
}

// Recv returns the next event, blocking until one arrives, ctx is done or
// the node is closed (then it returns [transport.ErrClosed]).
func (n *Node) Recv(ctx context.Context) (transport.Event, error) {
	for {
		n.mu.Lock()
		if n.closed {
			n.mu.Unlock()
			return transport.Event{}, transport.ErrClosed
		}
		if len(n.queue) > 0 {
			q := n.queue[0]
			n.queue[0] = queued{}
			n.queue = n.queue[1:]
			n.mu.Unlock()
			n.budget.Release(q.cost)
			return q.ev, nil
		}
		n.mu.Unlock()
		select {
		case <-n.wake:
		case <-n.done: // loop back to the closed check
		case <-ctx.Done():
			return transport.Event{}, ctx.Err()
		}
	}
}

// Peers returns the peers announced with a PeerUp and not yet with a
// PeerDown, sorted.
func (n *Node) Peers() []transport.PeerID {
	n.mu.Lock()
	defer n.mu.Unlock()
	peers := make([]transport.PeerID, 0, len(n.peers))
	for _, ps := range n.peers {
		if ps.up {
			peers = append(peers, ps.name)
		}
	}
	slices.Sort(peers)
	return peers
}

// Reserve takes size bytes of the node's receive budget, the one that bounds
// the events waiting for Recv, and reports whether they were free. The mux
// that reassembles fragments above the node charges its partial messages
// here, so that all memory held for incoming data counts against one budget.
// Give them back with [Node.Unreserve].
func (n *Node) Reserve(size int) bool { return n.budget.TryAcquire(size) }

// Unreserve gives back bytes taken with [Node.Reserve].
func (n *Node) Unreserve(size int) { n.budget.Release(size) }

// Close disconnects every peer, stops the node and waits for its goroutines.
// It is idempotent. Connected peers observe a PeerDown; this node's pending
// events are discarded.
func (n *Node) Close() error {
	n.closeOnce.Do(func() {
		n.mu.Lock()
		if n.stop != nil {
			n.stop()
		}
		n.closed = true
		if n.openTimer != nil {
			n.openTimer.Stop()
		}
		n.queue = nil
		if n.bulk != nil {
			n.bulk.queue = nil
			n.bulk.budget.Close()
		}
		n.changedLocked()
		n.mu.Unlock()
		close(n.done)
		n.cancel()
		n.dialWG.Wait()
		n.budget.Close()
		if n.lan != nil {
			_ = n.lan.Close()
		}
		_ = n.sub.Close()
		_ = n.addrSub.Close()
		n.h.Network().StopNotify(n.notifee)
		if err := n.h.Close(); err != nil {
			n.closeErr = fmt.Errorf("p2p: close: %w", err)
		}
	})
	n.wg.Wait()
	return n.closeErr
}

func (n *Node) isClosed() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.closed
}

// pushLocked queues an event for Recv, with the byte budget it holds. The
// announcements of peers are also queued for the bulk lane.
func (n *Node) pushLocked(ev transport.Event, cost int) {
	if n.closed {
		return
	}
	if n.bulk != nil && (ev.Kind == transport.PeerUp || ev.Kind == transport.PeerDown) {
		n.bulk.pushLocked(ev, 0)
	}
	n.queue = append(n.queue, queued{ev: ev, cost: cost})
	select {
	case n.wake <- struct{}{}:
	default:
	}
}

// changedLocked wakes the goroutines waiting for a peerState to change.
func (n *Node) changedLocked() {
	close(n.chg)
	n.chg = make(chan struct{})
}

// ErrPortBusy reports that the port asked for is taken; [New] with Port 0 picks
// a free one instead.
var ErrPortBusy = errors.New("p2p: port busy")

// portFree checks that port can be bound on TCP and UDP (IPv4) before libp2p
// starts: libp2p fails a busy listen only after starting its services, and
// does not stop all of them.
func portFree(port int) error {
	addr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp4", addr)
	if err != nil {
		return fmt.Errorf("%w: tcp %d: %w", ErrPortBusy, port, err)
	}
	_ = l.Close()
	c, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return fmt.Errorf("%w: udp %d: %w", ErrPortBusy, port, err)
	}
	_ = c.Close()
	return nil
}
