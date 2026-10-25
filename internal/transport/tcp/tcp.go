// Package tcp is a [transport.Transport] over plain TCP, for stage 1.
//
// SECURITY: this transport is temporarily unauthenticated and unencrypted.
// Any host that can reach the listening port can join and read or inject
// every payload, and a peer's identity is only what it claims. Use it on a
// trusted local network only; stage 2 replaces it with libp2p and Noise.
//
// Each connection starts with a handshake frame "INKW" | version (1 byte,
// currently 1) | peer ID, sent by both sides. A peer is announced with a
// PeerUp only once its handshake is valid, and with exactly one PeerDown when
// its link is lost. Frames are a 4-byte big-endian length followed by the
// payload (see [frame.Write] and [frame.Read]), bounded by [transport.MaxFrame].
//
// When two peers connect to each other at the same time, both connections
// complete and each side keeps the one initiated by the smaller PeerID. The
// side that initiated the losing connection closes it, so the other side
// never sees its peer flap.
//
//	n := tcp.New(tcp.NewPeerID(), tcp.Options{})
//	defer n.Close()
//	addr, err := n.Listen(ctx, "127.0.0.1:0")
//	// elsewhere: peer, err := other.Dial(ctx, addr.String())
package tcp

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

const (
	protoMagic   = "INKW"
	protoVersion = 1
	// maxHello bounds a handshake frame: magic, version and the longest peer ID.
	maxHello = len(protoMagic) + 1 + 64
	// msgOverhead is charged to the byte budget on top of each payload, so
	// floods of tiny frames are bounded too.
	msgOverhead = 64
	// maxConnsPerPeer bounds the simultaneous connections claiming one PeerID.
	maxConnsPerPeer = 4
	// writeRate scales the write deadline: each Send gets WriteTimeout plus
	// one second per writeRate bytes.
	writeRate = 1 << 20

	defaultHandshakeTimeout = 5 * time.Second
	defaultWriteTimeout     = 10 * time.Second
	defaultMaxPending       = 16
	defaultMaxPeers         = 64
	defaultMaxBuffered      = 256 << 20

	acceptBackoffMin = 5 * time.Millisecond
	acceptBackoffMax = time.Second
)

// Options tunes a [Node]. The zero value selects the defaults.
type Options struct {
	// HandshakeTimeout bounds the whole handshake of a connection. Zero means 5 s.
	HandshakeTimeout time.Duration
	// WriteTimeout bounds each Send on the wire, plus one second per MiB of
	// payload. Zero means 10 s.
	WriteTimeout time.Duration
	// MaxPendingHandshakes bounds the inbound connections that have not
	// finished their handshake; further ones are closed at once. Zero means 16.
	MaxPendingHandshakes int
	// MaxPeers bounds the connected peers; connections of further peers are
	// refused. Zero means 64.
	MaxPeers int
	// MaxBufferedBytes bounds the received payload bytes (plus a small
	// per-message overhead) waiting for Recv, across all peers. When it is
	// exhausted, readers stop reading and TCP backpressures the senders. A
	// frame larger than the budget is charged the whole budget. Zero means
	// 256 MiB.
	MaxBufferedBytes int
}

// Stats counts events useful to spot misbehaving or hostile peers.
type Stats struct {
	// Replaced counts the times a new connection took over the link of a peer
	// that already had one.
	Replaced uint64
	// FailedHandshakes counts inbound connections whose handshake failed.
	FailedHandshakes uint64
}

var (
	errHandshake    = errors.New("tcp: handshake failed")
	errTooManyPeers = errors.New("tcp: too many peers")
)

// NewPeerID returns a fresh random PeerID: 16 bytes from crypto/rand, in
// lowercase unpadded base32.
func NewPeerID() transport.PeerID {
	var b [16]byte
	_, _ = rand.Read(b[:]) // never fails since Go 1.24: it crashes the program instead.
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return transport.PeerID(strings.ToLower(enc))
}

// conn is one handshaken connection to a peer.
type conn struct {
	nc       net.Conn
	peer     transport.PeerID
	wmu      sync.Mutex // serialises writes: frames must not interleave
	outbound bool       // we dialed it
}

// peerConns are the live connections that claim one PeerID. Exactly one is
// the link: the one Send writes to and Messages are delivered from.
type peerConns struct {
	link  *conn
	conns []*conn
}

// queued is an event with the byte budget to release once Recv returns it.
type queued struct {
	ev   transport.Event
	cost int
}

// Node is a TCP peer; it implements [transport.Transport].
type Node struct {
	done   chan struct{} // closed by Close
	wake   chan struct{} // capacity 1: "the queue may be non-empty"
	budget *frame.Budget
	peers  map[transport.PeerID]*peerConns
	raw    map[net.Conn]struct{} // every open connection, handshaken or not
	local  transport.PeerID
	lns    []net.Listener
	queue  []queued
	opts   Options
	wg     sync.WaitGroup
	// pending counts inbound connections still handshaking.
	pending          int
	replaced         atomic.Uint64
	failedHandshakes atomic.Uint64
	mu               sync.Mutex
	// closed is set by Close, under mu.
	closed bool
}

var _ transport.Transport = (*Node)(nil)

// New returns a node identified as local. It panics if local is not a valid
// PeerID, which is a programming error; use [NewPeerID] for a fresh one.
func New(local transport.PeerID, o Options) *Node {
	if !transport.ValidPeerID(local) {
		panic(fmt.Sprintf("tcp: invalid peer id %q", local))
	}
	o.HandshakeTimeout = cmp.Or(o.HandshakeTimeout, defaultHandshakeTimeout)
	o.WriteTimeout = cmp.Or(o.WriteTimeout, defaultWriteTimeout)
	o.MaxPendingHandshakes = cmp.Or(o.MaxPendingHandshakes, defaultMaxPending)
	o.MaxPeers = cmp.Or(o.MaxPeers, defaultMaxPeers)
	o.MaxBufferedBytes = cmp.Or(o.MaxBufferedBytes, defaultMaxBuffered)
	return &Node{
		local:  local,
		opts:   o,
		done:   make(chan struct{}),
		wake:   make(chan struct{}, 1),
		budget: frame.NewBudget(o.MaxBufferedBytes),
		peers:  make(map[transport.PeerID]*peerConns),
		raw:    make(map[net.Conn]struct{}),
	}
}

// Stats returns a snapshot of the node's counters.
func (n *Node) Stats() Stats {
	return Stats{Replaced: n.replaced.Load(), FailedHandshakes: n.failedHandshakes.Load()}
}

// Local returns the node's identity.
func (n *Node) Local() transport.PeerID { return n.local }

// Listen accepts connections on addr (for example "127.0.0.1:0") until ctx
// is done or the node is closed. It returns the address actually bound.
func (n *Node) Listen(ctx context.Context, addr string) (net.Addr, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp: listen: %w", err)
	}
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		_ = ln.Close()
		return nil, transport.ErrClosed
	}
	n.lns = append(n.lns, ln)
	n.wg.Go(func() { n.acceptLoop(ctx, ln) })
	n.mu.Unlock()
	return ln.Addr(), nil
}

func (n *Node) acceptLoop(ctx context.Context, ln net.Listener) {
	stop := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stop()
	var backoff time.Duration
	for {
		nc, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			// Transient failure (for example out of file descriptors):
			// back off and retry, like net/http.Server.Serve.
			backoff = min(max(2*backoff, acceptBackoffMin), acceptBackoffMax)
			select {
			case <-time.After(backoff):
				continue
			case <-n.done:
				return
			case <-ctx.Done():
				return
			}
		}
		backoff = 0
		if !n.admit(nc) {
			continue
		}
		n.wg.Go(func() {
			defer n.endPending()
			peer, err := n.handshake(ctx, nc)
			if err != nil {
				n.failedHandshakes.Add(1)
				n.discard(nc)
				return
			}
			_ = n.register(nc, peer, false)
		})
	}
}

// admit records an inbound connection as pending. It closes nc and reports
// false if the node is closed or too many handshakes are already pending.
func (n *Node) admit(nc net.Conn) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed || n.pending >= n.opts.MaxPendingHandshakes {
		_ = nc.Close()
		return false
	}
	n.pending++
	n.raw[nc] = struct{}{}
	return true
}

func (n *Node) endPending() {
	n.mu.Lock()
	n.pending--
	n.mu.Unlock()
}

// Dial connects to the node listening on addr and returns its PeerID once
// the handshake succeeded. If a link to that peer already exists, the better
// of the two connections is kept and Dial still succeeds.
func (n *Node) Dial(ctx context.Context, addr string) (transport.PeerID, error) {
	n.mu.Lock()
	closed := n.closed
	n.mu.Unlock()
	if closed {
		return "", transport.ErrClosed
	}
	var d net.Dialer
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("tcp: dial: %w", err)
	}
	if !n.track(nc) {
		return "", transport.ErrClosed
	}
	peer, err := n.handshake(ctx, nc)
	if err != nil {
		n.discard(nc)
		return "", err
	}
	if err := n.register(nc, peer, true); err != nil {
		return "", err
	}
	return peer, nil
}

// track records nc so Close can reach it. It closes nc and reports false if
// the node is already closed.
func (n *Node) track(nc net.Conn) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		_ = nc.Close()
		return false
	}
	n.raw[nc] = struct{}{}
	return true
}

// discard closes a connection that never completed its handshake.
func (n *Node) discard(nc net.Conn) {
	_ = nc.Close()
	n.mu.Lock()
	delete(n.raw, nc)
	n.mu.Unlock()
}

// handshake exchanges hello frames on nc and returns the remote's PeerID.
func (n *Node) handshake(ctx context.Context, nc net.Conn) (transport.PeerID, error) {
	if err := nc.SetDeadline(time.Now().Add(n.opts.HandshakeTimeout)); err != nil {
		return "", fmt.Errorf("%w: %w", errHandshake, err)
	}
	stop := context.AfterFunc(ctx, func() { _ = nc.Close() })
	defer stop()

	hello := append([]byte(protoMagic), protoVersion)
	hello = append(hello, n.local...)
	if err := frame.Write(nc, hello); err != nil {
		return "", fmt.Errorf("%w: %w", errHandshake, err)
	}
	got, err := frame.Read(nc, maxHello)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errHandshake, err)
	}
	rest, ok := bytes.CutPrefix(got, []byte(protoMagic))
	if !ok || len(rest) == 0 {
		return "", fmt.Errorf("%w: bad magic", errHandshake)
	}
	if rest[0] != protoVersion {
		return "", fmt.Errorf("%w: unsupported version %d", errHandshake, rest[0])
	}
	peer := transport.PeerID(rest[1:])
	if !transport.ValidPeerID(peer) {
		return "", fmt.Errorf("%w: invalid peer id", errHandshake)
	}
	if peer == n.local {
		return "", fmt.Errorf("%w: peer is ourselves", errHandshake)
	}
	if err := nc.SetDeadline(time.Time{}); err != nil {
		return "", fmt.Errorf("%w: %w", errHandshake, err)
	}
	return peer, nil
}

// initiator returns the PeerID that opened c.
func (n *Node) initiator(c *conn) transport.PeerID {
	if c.outbound {
		return n.local
	}
	return c.peer
}

// register makes a handshaken connection the link to peer, or keeps it as a
// spare if the existing link wins. The link is the connection initiated by
// the smaller PeerID; on a tie (same initiator, a redial) the existing link
// stays, and a spare takes over only if it ends. A redial of a peer we
// already dialed is closed at once.
func (n *Node) register(nc net.Conn, peer transport.PeerID, outbound bool) error {
	c := &conn{nc: nc, peer: peer, outbound: outbound}
	n.mu.Lock()
	defer n.mu.Unlock()
	ps := n.peers[peer]
	var err error
	switch {
	case n.closed:
		err = transport.ErrClosed
	case ps == nil && len(n.peers) >= n.opts.MaxPeers, ps != nil && len(ps.conns) >= maxConnsPerPeer:
		err = errTooManyPeers
	}
	if err != nil {
		_ = nc.Close()
		delete(n.raw, nc)
		return err
	}
	switch {
	case ps != nil && outbound && ps.link.outbound:
		// Redial of a peer we already dialed: the existing link stays and
		// the remote just sees a spare connection end.
		_ = nc.Close()
		delete(n.raw, nc)
		return nil
	case ps == nil:
		n.peers[peer] = &peerConns{link: c, conns: []*conn{c}}
		n.pushLocked(transport.Event{Kind: transport.PeerUp, Peer: peer}, 0)
	case n.initiator(c) < n.initiator(ps.link):
		ps.conns = append(ps.conns, c)
		n.replaced.Add(1)
		old := ps.link
		ps.link = c
		n.dropLocked(old)
	default:
		ps.conns = append(ps.conns, c)
		n.dropLocked(c)
	}
	n.wg.Go(func() { n.readLoop(c) })
	return nil
}

// dropLocked retires a losing connection. Only the side that initiated it
// closes it, and only after a grace period of HandshakeTimeout: the remote
// may still be using it as its link until its own handshake on the winning
// connection completes, and closing it earlier would make the remote report
// a spurious PeerDown. The other side keeps reading it until the initiator
// closes it; its frames are ignored while it is not the link.
func (n *Node) dropLocked(c *conn) {
	if c.outbound {
		time.AfterFunc(n.opts.HandshakeTimeout, func() { _ = c.nc.Close() })
	}
}

// removeLocked forgets the ended connection c. If it was the link and a
// spare survives, the spare becomes the link and the peer stays up; the
// peer goes down only when its last connection ends.
func (n *Node) removeLocked(c *conn) {
	ps := n.peers[c.peer]
	if ps == nil {
		return
	}
	if i := slices.Index(ps.conns, c); i >= 0 {
		ps.conns = slices.Delete(ps.conns, i, i+1)
	}
	switch {
	case len(ps.conns) == 0:
		delete(n.peers, c.peer)
		n.pushLocked(transport.Event{Kind: transport.PeerDown, Peer: c.peer}, 0)
	case ps.link == c:
		ps.link = slices.MinFunc(ps.conns, func(x, y *conn) int {
			return cmp.Compare(n.initiator(x), n.initiator(y))
		})
	}
}

// readLoop delivers the frames of c while it is the link, until the
// connection ends.
func (n *Node) readLoop(c *conn) {
	defer func() {
		_ = c.nc.Close()
		n.mu.Lock()
		delete(n.raw, c.nc)
		n.removeLocked(c)
		n.mu.Unlock()
	}()
	for {
		var cost int
		payload, err := frame.ReadReserve(c.nc, transport.MaxFrame, func(size int) error {
			cost = min(size+msgOverhead, n.opts.MaxBufferedBytes)
			if !n.budget.Acquire(cost) {
				cost = 0
				return transport.ErrClosed
			}
			return nil
		})
		if err != nil {
			n.budget.Release(cost)
			return
		}
		n.mu.Lock()
		if ps := n.peers[c.peer]; ps != nil && ps.link == c && !n.closed {
			n.pushLocked(transport.Event{Kind: transport.Message, Peer: c.peer, Payload: payload}, cost)
		} else {
			n.budget.Release(cost)
		}
		n.mu.Unlock()
	}
}

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

// Send writes payload to peer to as one frame. Writes to one peer are
// serialised and bounded by [Options.WriteTimeout] plus one second per MiB;
// canceling ctx interrupts a write in progress. A failed or interrupted write
// closes the link, since the stream is then in an unknown state.
func (n *Node) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n.mu.Lock()
	closed := n.closed
	var c *conn
	if ps := n.peers[to]; ps != nil {
		c = ps.link
	}
	n.mu.Unlock()
	switch {
	case closed:
		return transport.ErrClosed
	case len(payload) > transport.MaxFrame:
		return transport.ErrFrameTooLarge
	case c == nil:
		return transport.ErrUnknownPeer
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	timeout := n.opts.WriteTimeout + time.Duration(len(payload))*time.Second/writeRate
	if err := c.nc.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		_ = c.nc.Close()
		return fmt.Errorf("tcp: send: %w", err)
	}
	stop := context.AfterFunc(ctx, func() { _ = c.nc.SetWriteDeadline(time.Now()) })
	err := frame.Write(c.nc, payload)
	stop()
	if err != nil {
		_ = c.nc.Close()
		if cerr := ctx.Err(); cerr != nil {
			err = cerr
		}
		return fmt.Errorf("tcp: send to %s: %w", to, err)
	}
	return nil
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

// Peers returns the currently connected peers, sorted.
func (n *Node) Peers() []transport.PeerID {
	n.mu.Lock()
	defer n.mu.Unlock()
	peers := make([]transport.PeerID, 0, len(n.peers))
	for p := range n.peers {
		peers = append(peers, p)
	}
	slices.Sort(peers)
	return peers
}

// Close stops listening, closes every connection and waits for the node's
// goroutines. It is idempotent. Connected peers observe a PeerDown on their
// side; this node's pending events are discarded.
func (n *Node) Close() error {
	n.mu.Lock()
	if !n.closed {
		n.closed = true
		close(n.done)
		n.budget.Close()
		n.queue = nil
		for _, ln := range n.lns {
			_ = ln.Close()
		}
		for nc := range n.raw {
			_ = nc.Close()
		}
	}
	n.mu.Unlock()
	n.wg.Wait()
	return nil
}
