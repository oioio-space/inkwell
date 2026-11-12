package p2p

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
)

const (
	// minBackoff and maxBackoff bound the wait between two dial attempts to
	// a wanted peer, which doubles after each failure.
	minBackoff = time.Second
	maxBackoff = time.Minute
	// stableAfter is how long a connection must last for the next loss to be
	// retried at once rather than after the backoff: a link that drops as
	// soon as it is made (a peer that refuses us) must not be redialed in a
	// tight loop.
	stableAfter = 30 * time.Second
	// knownDialTimeout bounds an attempt at the last known addresses of a
	// peer, which may be stale; circuitDialTimeout bounds the one through
	// the relays, which needs two hops and a reservation on the far side.
	knownDialTimeout   = 10 * time.Second
	circuitDialTimeout = 20 * time.Second
	// wakeEvery bounds how often mDNS sightings of a wanted peer wake its dial
	// loop, and wakeBurst with wakeEvery the wakes of all peers together (a
	// token bucket): anyone on the local network can announce a sighting, so
	// sightings must not make the node dial in a loop.
	wakeEvery = knownDialTimeout
	wakeBurst = 4
)

// wanted is a peer the node keeps connected.
type wanted struct {
	cancel context.CancelFunc
	id     peer.ID
	// known are the last known addresses of the peer, under Node.mu.
	known []ma.Multiaddr
	// wake (capacity 1) ends the loop's pause, without resetting its backoff: the peer was seen on
	// the local network. woken is when it last was, under Node.mu (see wakeEvery).
	wake  chan struct{}
	woken time.Time
}

var errNoAddress = errors.New("no address to dial")

// Want sets the peers to keep connected, with the addresses they were last
// seen at (as returned by their [Node.Addrs]; they may be stale or empty).
// Peers absent from the map are no longer wanted, but their connections are
// left alone; use [Node.Drop] to close one.
//
// For each wanted peer that is not connected, the node dials its known
// addresses, then the ones its address book holds (what the peer announced
// when it was last connected, kept across restarts with [Options.Book]), then
// the address through each relay,
// /<relay>/p2p/<relay id>/p2p-circuit/p2p/<peer>. The latter does not depend
// on the peer's current network, so it finds a peer that came back from
// another one. A failed attempt is retried after one second, then two, doubling
// up to a minute, with some jitter; a change of the node's own addresses (it
// moved to another network) or a relay becoming reachable again retries at
// once. In RelayOnly mode only addresses through a relay are dialed.
//
// The wanted peers that are connected have their addresses recorded in the
// address book, if [Options.Admit] accepts them: a peer that became a member
// while connected (an invitation) is found again after a restart.
//
// Invalid peers and addresses are ignored. Want does not block.
func (n *Node) Want(peers map[transport.PeerID][]string) {
	parsed := make(map[peer.ID][]ma.Multiaddr, len(peers))
	for name, addrs := range peers {
		id, ok := parseID(name)
		if !ok || id == n.h.ID() {
			continue
		}
		parsed[id] = nil
		for _, a := range addrs {
			if ai, err := parseAddr(a, id); err == nil {
				parsed[id] = append(parsed[id], ai.Addrs...)
			}
		}
	}

	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	for id, w := range n.want {
		if _, ok := parsed[id]; !ok {
			w.cancel()
			delete(n.want, id)
		}
	}
	for id, known := range parsed {
		if w := n.want[id]; w != nil {
			w.known = known
			continue
		}
		delete(n.held, id) // wanted anew (a new pairing): an earlier reset no longer keeps it out
		ctx, cancel := context.WithCancel(n.ctx)
		w := &wanted{id: id, known: known, cancel: cancel, wake: make(chan struct{}, 1)}
		n.want[id] = w
		n.dialWG.Go(func() { n.dialLoop(ctx, w) })
	}
	n.kickLocked()
	var up []peer.ID
	for id := range parsed {
		if ps := n.peers[id]; ps != nil && ps.up {
			up = append(up, id)
		}
	}
	n.mu.Unlock()
	for _, id := range up { // outside mu: Admit and the peerstore are called
		n.remember(id)
	}
}

// unwant stops keeping id connected.
func (n *Node) unwant(id peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if w := n.want[id]; w != nil {
		w.cancel()
		delete(n.want, id)
	}
}

// kickAll makes every dial loop retry now.
func (n *Node) kickAll() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.kickLocked()
}

func (n *Node) kickLocked() {
	if n.closed {
		return
	}
	close(n.kick)
	n.kick = make(chan struct{})
}

// connected reports whether libp2p has a connection to id.
func (n *Node) connected(id peer.ID) bool {
	return n.h.Network().Connectedness(id) != network.NotConnected
}

// dialLoop keeps w connected until ctx is done.
func (n *Node) dialLoop(ctx context.Context, w *wanted) {
	backoff := minBackoff
	var since time.Time // when the current connection was seen, if there is one
	// due is when the pending pause ends, kept across wakes: sightings, which anyone on the
	// local network can forge, never postpone the next normal attempt.
	var due time.Time
	var lanOnly, ok bool
	for {
		n.mu.Lock()
		chg, kick := n.chg, n.kick
		n.mu.Unlock()

		if n.connected(w.id) {
			if since.IsZero() {
				since = time.Now()
			}
			select {
			case <-chg:
				continue
			case <-ctx.Done():
				return
			}
		}
		if !since.IsZero() {
			// The connection was lost. A long one is replaced at once.
			stable := time.Since(since) >= stableAfter
			since = time.Time{}
			if stable {
				backoff = minBackoff
			} else if lanOnly, ok = n.pause(ctx, kick, w.wake, &backoff, &due); !ok {
				return
			}
			continue
		}

		dial := n.dialOnce
		if lanOnly { // woken by a sighting: its addresses only, no relay
			dial, lanOnly = n.dialLAN, false
		}
		if err := dial(ctx, w); err == nil {
			// A peer that refuses us may drop the connection at once: count
			// it as made, so that its loss waits for the backoff.
			since, due = time.Now(), time.Time{}
			continue
		}
		if lanOnly, ok = n.pause(ctx, kick, w.wake, &backoff, &due); !ok {
			return
		}
	}
}

// pause waits until *due, which it sets to the backoff from now, with jitter,
// if it is zero; then it doubles the backoff and clears *due. A kick ends the
// wait at once, resets the backoff and clears *due; a wake ends it and reports
// woken, leaving the backoff and *due as they are, so that the next pause ends
// at the same time. ok is false if ctx is done.
func (n *Node) pause(ctx context.Context, kick, wake <-chan struct{}, backoff *time.Duration, due *time.Time) (woken, ok bool) {
	if due.IsZero() {
		*due = time.Now().Add(jitter(*backoff))
	}
	t := time.NewTimer(time.Until(*due))
	defer t.Stop()
	select {
	case <-t.C:
		*backoff = min(*backoff*2, maxBackoff)
	case <-kick:
		*backoff = minBackoff
	case <-wake:
		return true, true
	case <-ctx.Done():
		return false, false
	}
	*due = time.Time{}
	return false, true
}

// jitter spreads d by up to a quarter either way, so that peers that lost a
// relay together do not retry together.
func jitter(d time.Duration) time.Duration {
	if d < 4 {
		return d
	}
	return d - d/4 + rand.N(d/2) // #nosec G404 -- spreading retries; nothing secret is drawn here.
}

// dialOnce makes one round of attempts at w: its known addresses, then the
// other ones of the address book, the authenticated ones and, apart, after
// them, those mDNS gave, then the relays. It returns nil once connected. In
// RelayOnly mode only circuits are dialed, so not the book.
func (n *Node) dialOnce(ctx context.Context, w *wanted) error {
	n.mu.Lock()
	known := slices.Clone(w.known)
	n.mu.Unlock()
	var book, lan []ma.Multiaddr
	if n.opts.RelayOnly {
		known = slices.DeleteFunc(known, func(a ma.Multiaddr) bool { return !isCircuit(a) })
	} else {
		book, lan = n.book.dialAddrs(w.id)
		book = slices.DeleteFunc(book, func(a ma.Multiaddr) bool { return slices.ContainsFunc(known, a.Equal) })
		lan = slices.DeleteFunc(lan, func(a ma.Multiaddr) bool { return slices.ContainsFunc(known, a.Equal) })
	}

	var errs []error
	if len(known) > 0 {
		err := n.dial(ctx, w.id, known, knownDialTimeout)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	for _, addrs := range [][]ma.Multiaddr{book, lan} { // the same bound: last known addresses too
		if len(addrs) == 0 {
			continue
		}
		err := n.dial(ctx, w.id, addrs, knownDialTimeout)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	if len(n.circuit) > 0 {
		err := n.dial(ctx, w.id, n.circuit, circuitDialTimeout)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return errNoAddress
	}
	return errors.Join(errs...)
}

// dialLAN is the attempt of a dial loop woken by an mDNS sighting: the
// addresses the local network gave, and nothing else (no relay).
func (n *Node) dialLAN(ctx context.Context, w *wanted) error {
	if n.opts.RelayOnly {
		return errNoAddress
	}
	_, lan := n.book.dialAddrs(w.id)
	if len(lan) == 0 {
		return errNoAddress
	}
	return n.dial(ctx, w.id, lan, knownDialTimeout)
}

// forgetDialFailures makes the swarm try id again: it refuses to dial an
// address that failed recently, for longer after each failure, up to five
// minutes, whereas this node has its own schedule of retries.
func (n *Node) forgetDialFailures(id peer.ID) {
	if s, ok := n.h.Network().(interface{ Backoff() *swarm.DialBackoff }); ok {
		s.Backoff().Clear(id)
	}
}

// dial connects to id at addrs, and at them only: libp2p dials every address
// its peerstore holds for a peer (earlier attempts leave theirs for minutes), so
// they are cleared first, or an attempt at mDNS hints would dial the
// authenticated addresses too and the order of the attempts would mean nothing.
// It is called for a peer that is not connected, whose peerstore addresses are
// stale.
func (n *Node) dial(ctx context.Context, id peer.ID, addrs []ma.Multiaddr, timeout time.Duration) error {
	if n.opts.onDial != nil {
		n.opts.onDial(id, slices.Clone(addrs))
	}
	n.h.Peerstore().ClearAddrs(id)
	n.forgetDialFailures(id)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return n.h.Connect(ctx, peer.AddrInfo{ID: id, Addrs: addrs})
}
