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
)

// wanted is a peer the node keeps connected.
type wanted struct {
	cancel context.CancelFunc
	id     peer.ID
	// known are the last known addresses of the peer, under Node.mu.
	known []ma.Multiaddr
}

var errNoAddress = errors.New("no address to dial")

// Want sets the peers to keep connected, with the addresses they were last
// seen at (as returned by their [Node.Addrs]; they may be stale or empty).
// Peers absent from the map are no longer wanted, but their connections are
// left alone; use [Node.Drop] to close one.
//
// For each wanted peer that is not connected, the node dials its known
// addresses, then the address through each relay,
// /<relay>/p2p/<relay id>/p2p-circuit/p2p/<peer>. The latter does not depend
// on the peer's current network, so it finds a peer that came back from
// another one. A failed attempt is retried after one second, then two, doubling
// up to a minute, with some jitter; a change of the node's own addresses (it
// moved to another network) or a relay becoming reachable again retries at
// once. In RelayOnly mode only addresses through a relay are dialed.
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
	defer n.mu.Unlock()
	if n.closed {
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
		ctx, cancel := context.WithCancel(n.ctx)
		w := &wanted{id: id, known: known, cancel: cancel}
		n.want[id] = w
		n.dialWG.Go(func() { n.dialLoop(ctx, w) })
	}
	n.kickLocked()
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
			} else if !n.pause(ctx, kick, &backoff) {
				return
			}
			continue
		}

		if err := n.dialOnce(ctx, w); err == nil {
			// A peer that refuses us may drop the connection at once: count
			// it as made, so that its loss waits for the backoff.
			since = time.Now()
			continue
		}
		if !n.pause(ctx, kick, &backoff) {
			return
		}
	}
}

// pause waits for the backoff, with jitter, then doubles it. A kick ends the
// wait at once and resets the backoff. It returns false if ctx is done.
func (n *Node) pause(ctx context.Context, kick <-chan struct{}, backoff *time.Duration) bool {
	t := time.NewTimer(jitter(*backoff))
	defer t.Stop()
	select {
	case <-t.C:
		*backoff = min(*backoff*2, maxBackoff)
	case <-kick:
		*backoff = minBackoff
	case <-ctx.Done():
		return false
	}
	return true
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
// relays. It returns nil once connected.
func (n *Node) dialOnce(ctx context.Context, w *wanted) error {
	n.mu.Lock()
	known := slices.Clone(w.known)
	n.mu.Unlock()
	if n.opts.RelayOnly {
		known = slices.DeleteFunc(known, func(a ma.Multiaddr) bool { return !isCircuit(a) })
	}

	var errs []error
	if len(known) > 0 {
		err := n.dial(ctx, w.id, known, knownDialTimeout)
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

// forgetDialFailures makes the swarm try id again: it refuses to dial an
// address that failed recently, for longer after each failure, up to five
// minutes, whereas this node has its own schedule of retries.
func (n *Node) forgetDialFailures(id peer.ID) {
	if s, ok := n.h.Network().(interface{ Backoff() *swarm.DialBackoff }); ok {
		s.Backoff().Clear(id)
	}
}

// dial connects to id at addrs.
func (n *Node) dial(ctx context.Context, id peer.ID, addrs []ma.Multiaddr, timeout time.Duration) error {
	n.forgetDialFailures(id)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return n.h.Connect(ctx, peer.AddrInfo{ID: id, Addrs: addrs})
}
