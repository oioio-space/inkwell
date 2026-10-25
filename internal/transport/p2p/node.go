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
//	n, err := p2p.New(ctx, key, p2p.Options{})
//	if err != nil { /* … */ }
//	defer n.Close()
//	err = n.Connect(ctx, peer, addrs) // addrs come from the peer's Addrs
package p2p

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
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

	// writeTimeout plus one second per writeRate bytes bounds each Send.
	writeTimeout = 10 * time.Second
	writeRate    = 1 << 20
	// openTimeout bounds opening the outbound stream.
	openTimeout = 15 * time.Second
	// upTimeout bounds how long an inbound stream waits for its peer to be
	// announced, before it is reset.
	upTimeout = 30 * time.Second
	// msgOverhead is charged to the byte budget on top of each payload, so
	// floods of tiny frames are bounded too.
	msgOverhead = 64
	// maxBuffered bounds the received bytes waiting for Recv, across peers.
	maxBuffered = 256 << 20
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
	// Port is the UDP port of QUIC and the TCP port of the fallback. Zero
	// picks free ports.
	Port int
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
	name transport.PeerID
	id   peer.ID
	// connected: libp2p has at least one connection to the peer.
	connected bool
	// admitted: Options.Admit accepted the peer when it connected.
	admitted bool
	// opening: a goroutine is opening the outbound stream.
	opening bool
	// up: PeerUp was announced and PeerDown has not been yet.
	up bool
}

// Node is a libp2p peer; it implements [transport.Transport].
type Node struct {
	h      host.Host
	ctx    context.Context // canceled by Close
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{} // capacity 1: "the queue may be non-empty"
	// chg is closed and replaced whenever a peerState changes, to wake the
	// goroutines that wait for one.
	chg    chan struct{}
	budget *frame.Budget
	peers  map[peer.ID]*peerState
	sub    event.Subscription
	stop   func() bool // stops the close-on-context-cancel hook; under mu
	queue  []queued
	opts   Options
	local  transport.PeerID
	wg     sync.WaitGroup

	closeOnce sync.Once
	closeErr  error
	mu        sync.Mutex
	// closed is set by Close, under mu.
	closed bool
}

var _ transport.Transport = (*Node)(nil)

// gate lets the connection gater consult Options.Admit once a peer is
// authenticated; everything else is allowed.
type gate struct {
	*conngater.BasicConnectionGater
	admit func(peer.ID) bool
}

func (g gate) InterceptSecured(_ network.Direction, p peer.ID, _ network.ConnMultiaddrs) bool {
	return g.admit(p)
}

// New starts a node identified by key. Canceling ctx closes the node.
//
// The node listens on QUIC (IPv4 and IPv6) and TCP at o.Port. Its resources
// are bounded: 64 peers' worth of connections, 4 sync streams per peer, 256
// MiB of memory.
func New(ctx context.Context, key ed25519.PrivateKey, o Options) (*Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("p2p: private key is %d bytes, want %d", len(key), ed25519.PrivateKeySize)
	}
	if o.Port < 0 || o.Port > 65535 {
		return nil, fmt.Errorf("p2p: invalid port %d", o.Port)
	}
	priv, err := crypto.UnmarshalEd25519PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("p2p: private key: %w", err)
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
		local:  localName,
		opts:   o,
		done:   make(chan struct{}),
		wake:   make(chan struct{}, 1),
		chg:    make(chan struct{}),
		budget: frame.NewBudget(maxBuffered),
		peers:  make(map[peer.ID]*peerState),
	}
	n.ctx, n.cancel = context.WithCancel(ctx)

	opts := []libp2p.Option{
		libp2p.Identity(priv),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DefaultMuxers,
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", o.Port),
			fmt.Sprintf("/ip6/::/udp/%d/quic-v1", o.Port),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", o.Port),
		),
		libp2p.ResourceManager(rm),
		libp2p.ConnectionManager(cm),
		libp2p.ConnectionGater(gate{BasicConnectionGater: base, admit: n.admitted}),
		libp2p.DisableMetrics(),
	}
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
	h.SetStreamHandler(ProtocolSync, n.handleStream)
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

// admitted reports whether Options.Admit accepts p.
func (n *Node) admitted(p peer.ID) bool {
	if n.opts.Admit == nil {
		return true
	}
	name, ok := peerName(p)
	return ok && n.opts.Admit(name)
}

// Local returns the node's identity.
func (n *Node) Local() transport.PeerID { return n.local }

// Addrs returns the addresses the node listens on, each ending with
// /p2p/<id> so that it is enough to dial the node. Addresses of a wildcard
// listener are expanded to each local interface.
func (n *Node) Addrs() []string {
	mas, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: n.h.ID(), Addrs: n.h.Addrs()})
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

// Drop closes the connections to peer and forgets what is known about it. It
// is for revocation: revoke the peer in Options.Admit first, or it may
// reconnect at once. The peer's PeerDown follows as for any lost link.
func (n *Node) Drop(who transport.PeerID) {
	id, ok := parseID(who)
	if !ok || id == n.h.ID() {
		return
	}
	_ = n.h.Network().ClosePeer(id)
	n.h.Peerstore().RemovePeer(id)
	n.h.Peerstore().ClearAddrs(id)
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
		n.queue = nil
		n.changedLocked()
		n.mu.Unlock()
		close(n.done)
		n.cancel()
		n.budget.Close()
		_ = n.sub.Close()
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

// pushLocked queues an event for Recv, with the byte budget it holds.
func (n *Node) pushLocked(ev transport.Event, cost int) {
	if n.closed {
		return
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
