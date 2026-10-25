// Package memnet is an in-memory [transport.Transport] network with seeded
// fault injection: dropped, duplicated and delayed (hence reordered)
// messages, and partitions.
//
// It exists to test the code above the transport without sockets. Delays use
// the standard time functions, so under [testing/synctest] they run on the
// bubble's virtual clock and a given seed replays the same faults.
//
//	n := memnet.New(seed, memnet.Faults{Drop: 0.1, MaxDelay: 50 * time.Millisecond})
//	a, b := n.Join("a"), n.Join("b")
//	n.Link("a", "b")
package memnet

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// Faults configures the misbehaviour of a [Network]. The zero value is a
// perfect network.
type Faults struct {
	// Drop is the probability, in [0, 1], that a message is lost.
	Drop float64
	// Duplicate is the probability, in [0, 1], that a message is delivered twice.
	Duplicate float64
	// MaxDelay bounds the random delivery delay; zero delivers immediately.
	MaxDelay time.Duration
}

// Network connects the [Node]s that joined it.
type Network struct {
	rng     *rand.Rand
	nodes   map[transport.PeerID]*Node
	links   map[link]uint64 // live links → epoch; absent means unlinked
	pending []delivery      // delayed messages, ordered by (due, seq)
	faults  Faults
	epoch   uint64 // bumped by every Link, so a re-link never resurrects old traffic
	seq     uint64 // tie-breaker for deliveries due at the same instant
	mu      sync.Mutex
}

// link is an unordered pair of peers.
type link struct{ a, b transport.PeerID }

func linkOf(a, b transport.PeerID) link {
	if b < a {
		a, b = b, a
	}
	return link{a, b}
}

// delivery is a delayed message, valid only for the link epoch it was sent on.
type delivery struct {
	due   time.Time
	ev    transport.Event
	to    transport.PeerID
	l     link
	epoch uint64
	seq   uint64
}

// New returns an empty network whose faults are drawn from seed.
func New(seed uint64, f Faults) *Network {
	return &Network{
		rng:    rand.New(rand.NewPCG(seed, seed)), // #nosec G404 -- faults must replay from a seed; nothing secret is drawn here.
		nodes:  make(map[transport.PeerID]*Node),
		links:  make(map[link]uint64),
		faults: f,
	}
}

// Join adds a peer to the network. It panics if id is invalid or already
// joined: both are bugs in the test that builds the network.
func (n *Network) Join(id transport.PeerID) *Node {
	if !transport.ValidPeerID(id) {
		panic(fmt.Sprintf("memnet: invalid peer id %q", id))
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, dup := n.nodes[id]; dup {
		panic(fmt.Sprintf("memnet: peer %q joined twice", id))
	}
	node := &Node{net: n, id: id, wake: make(chan struct{}, 1)}
	n.nodes[id] = node
	return node
}

// Link connects a and b, and each observes a PeerUp for the other. Linking an
// already linked pair, or a peer that has not joined or is closed, does
// nothing.
func (n *Network) Link(a, b transport.PeerID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	na, nb := n.nodes[a], n.nodes[b]
	l := linkOf(a, b)
	if _, linked := n.links[l]; a == b || na == nil || nb == nil || na.isClosed() || nb.isClosed() || linked {
		return
	}
	n.epoch++
	n.links[l] = n.epoch
	na.push(transport.Event{Kind: transport.PeerUp, Peer: b})
	nb.push(transport.Event{Kind: transport.PeerUp, Peer: a})
}

// Partition disconnects a and b, and each observes a PeerDown for the other.
// Messages still in flight between them are lost, even if they link again.
func (n *Network) Partition(a, b transport.PeerID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.unlinkLocked(a, b)
}

func (n *Network) unlinkLocked(a, b transport.PeerID) {
	l := linkOf(a, b)
	if _, linked := n.links[l]; !linked {
		return
	}
	delete(n.links, l)
	n.nodes[a].push(transport.Event{Kind: transport.PeerDown, Peer: b})
	n.nodes[b].push(transport.Event{Kind: transport.PeerDown, Peer: a})
}

// send applies the faults to one message from → to. Callers hold no lock.
func (n *Network) send(from, to transport.PeerID, payload []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	l := linkOf(from, to)
	epoch, linked := n.links[l]
	if !linked {
		return transport.ErrUnknownPeer
	}
	if n.rng.Float64() < n.faults.Drop {
		return nil // lost on the wire, as on a real network
	}
	copies := 1
	if n.rng.Float64() < n.faults.Duplicate {
		copies = 2
	}
	for range copies {
		ev := transport.Event{Kind: transport.Message, Peer: from, Payload: bytes.Clone(payload)}
		if n.faults.MaxDelay <= 0 {
			n.nodes[to].push(ev)
			continue
		}
		delay := time.Duration(n.rng.Int64N(int64(n.faults.MaxDelay) + 1))
		n.seq++
		d := delivery{due: time.Now().Add(delay), ev: ev, to: to, l: l, epoch: epoch, seq: n.seq}
		i, _ := slices.BinarySearchFunc(n.pending, d, cmpDelivery)
		n.pending = slices.Insert(n.pending, i, d)
		time.AfterFunc(delay, n.flush)
	}
	return nil
}

func cmpDelivery(x, y delivery) int {
	if c := x.due.Compare(y.due); c != 0 {
		return c
	}
	return cmp.Compare(x.seq, y.seq)
}

// flush delivers every pending message that is due, in (due, seq) order, so
// timers firing at the same instant cannot reorder them. A message whose link
// was broken — or broken and re-made — since it was sent is lost.
func (n *Network) flush() {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	for len(n.pending) > 0 && !n.pending[0].due.After(now) {
		d := n.pending[0]
		n.pending[0] = delivery{}
		n.pending = n.pending[1:]
		if epoch, linked := n.links[d.l]; linked && epoch == d.epoch {
			n.nodes[d.to].push(d.ev)
		}
	}
}

// Node is one peer of a [Network]; it implements [transport.Transport].
type Node struct {
	net    *Network
	wake   chan struct{} // capacity 1: "the queue may be non-empty"
	id     transport.PeerID
	queue  []transport.Event
	mu     sync.Mutex
	closed bool
}

var _ transport.Transport = (*Node)(nil)

// Local returns the node's identity.
func (nd *Node) Local() transport.PeerID { return nd.id }

// Send delivers a copy of payload to peer to, subject to the network faults.
func (nd *Node) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if nd.isClosed() {
		return transport.ErrClosed
	}
	if len(payload) > transport.MaxFrame {
		return transport.ErrFrameTooLarge
	}
	return nd.net.send(nd.id, to, payload)
}

// Recv returns the next event, blocking until one arrives or ctx is done.
func (nd *Node) Recv(ctx context.Context) (transport.Event, error) {
	for {
		nd.mu.Lock()
		if nd.closed {
			nd.mu.Unlock()
			return transport.Event{}, transport.ErrClosed
		}
		if len(nd.queue) > 0 {
			ev := nd.queue[0]
			nd.queue[0] = transport.Event{}
			nd.queue = nd.queue[1:]
			nd.mu.Unlock()
			return ev, nil
		}
		nd.mu.Unlock()
		select {
		case <-nd.wake:
		case <-ctx.Done():
			return transport.Event{}, ctx.Err()
		}
	}
}

// Peers returns the peers currently linked to the node, sorted.
func (nd *Node) Peers() []transport.PeerID {
	nd.net.mu.Lock()
	defer nd.net.mu.Unlock()
	var peers []transport.PeerID
	for l := range nd.net.links {
		switch nd.id {
		case l.a:
			peers = append(peers, l.b)
		case l.b:
			peers = append(peers, l.a)
		}
	}
	slices.Sort(peers)
	return peers
}

// Close unlinks the node from every peer, which each observe a PeerDown,
// and makes Send and Recv return ErrClosed.
func (nd *Node) Close() error {
	nd.net.mu.Lock()
	defer nd.net.mu.Unlock()
	nd.mu.Lock()
	if nd.closed {
		nd.mu.Unlock()
		return nil
	}
	nd.closed = true
	nd.queue = nil
	nd.mu.Unlock()
	for l := range nd.net.links {
		if l.a == nd.id || l.b == nd.id {
			nd.net.unlinkLocked(l.a, l.b)
		}
	}
	nd.signal()
	return nil
}

func (nd *Node) isClosed() bool {
	nd.mu.Lock()
	defer nd.mu.Unlock()
	return nd.closed
}

// push queues ev unless the node is closed.
func (nd *Node) push(ev transport.Event) {
	nd.mu.Lock()
	if !nd.closed {
		nd.queue = append(nd.queue, ev)
	}
	nd.mu.Unlock()
	nd.signal()
}

func (nd *Node) signal() {
	select {
	case nd.wake <- struct{}{}:
	default:
	}
}
