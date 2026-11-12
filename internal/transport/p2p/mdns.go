package p2p

import (
	"cmp"
	"slices"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"

	"github.com/oioio-space/inkwell/internal/transport"
)

const (
	// MDNSService is the mDNS service name of inkwell nodes.
	MDNSService = "_inkwell._udp"

	// maxNearby bounds the discovered peers kept.
	maxNearby = 64
	// nearbyTTL is how long a discovered peer is listed without being seen again.
	nearbyTTL = 5 * time.Minute
)

// Nearby is a node found on the local network.
type Nearby struct {
	// ID is the identity of the node.
	ID transport.PeerID
	// Addrs are its addresses, each ending with /p2p/<id>.
	Addrs []string
}

type nearbyEntry struct {
	seen time.Time
	info peer.AddrInfo
}

// startLAN starts the mDNS service of the node under the given service name
// ([MDNSService] if empty). The node then announces its
// identity and addresses on the local network and listens for the others
// (the library cannot do one without the other). What an announcement gives
// away is the identity and the addresses of the node and no more; whoever
// connects still has to be admitted (see [Options.Admit] and [Node.SetOpen]).
func (n *Node) startLAN(service string) error {
	svc := mdns.NewMdnsService(n.h, cmp.Or(service, MDNSService), (*lanNotifee)(n))
	if err := svc.Start(); err != nil {
		_ = svc.Close()
		return err
	}
	n.lan = svc
	return nil
}

// lanNotifee receives the nodes found by mDNS.
type lanNotifee Node

// HandlePeerFound implements mdns.Notifee. A member that is seen has the
// addresses it announces recorded in the address book, apart from the
// authenticated ones (anyone can forge an announcement; the handshake of a dial
// checks it), and, if it is wanted and not connected, its dial loop is woken, at
// most every wakeEvery.
func (l *lanNotifee) HandlePeerFound(info peer.AddrInfo) {
	n := (*Node)(l)
	wake := n.rememberLAN(info.ID, info.Addrs) && !n.connected(info.ID)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed || info.ID == n.h.ID() || len(info.Addrs) == 0 {
		return
	}
	now := time.Now()
	if w := n.want[info.ID]; wake && w != nil && now.Sub(w.woken) >= wakeEvery && n.wakeTokenLocked(now) {
		w.woken = now
		select {
		case w.wake <- struct{}{}:
		default:
		}
	}
	n.found[info.ID] = nearbyEntry{info: info, seen: now}
	if len(n.found) <= maxNearby {
		return
	}
	var oldest peer.ID // forget the entry seen longest ago
	for id, e := range n.found {
		if oldest == "" || e.seen.Before(n.found[oldest].seen) {
			oldest = id
		}
	}
	delete(n.found, oldest)
}

// wakeTokenLocked takes a token of the node's bucket of wakes, if there is
// one: wakeBurst at most, one more every wakeEvery.
func (n *Node) wakeTokenLocked(now time.Time) bool {
	if !n.wakeAt.IsZero() {
		n.wakeTokens = min(wakeBurst, n.wakeTokens+now.Sub(n.wakeAt).Seconds()/wakeEvery.Seconds())
	} else {
		n.wakeTokens = wakeBurst
	}
	n.wakeAt = now
	if n.wakeTokens < 1 {
		return false
	}
	n.wakeTokens--
	return true
}

// Nearby returns the nodes seen on the local network in the last five
// minutes, sorted by identity. It is empty unless [Options.LAN] is set.
func (n *Node) Nearby() []Nearby {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	var out []Nearby
	for id, e := range n.found {
		if now.Sub(e.seen) > nearbyTTL {
			delete(n.found, id)
			continue
		}
		name, ok := peerName(id)
		if !ok {
			continue
		}
		addrs, err := peer.AddrInfoToP2pAddrs(&e.info)
		if err != nil {
			continue
		}
		nb := Nearby{ID: name}
		for _, a := range addrs {
			nb.Addrs = append(nb.Addrs, a.String())
		}
		out = append(out, nb)
	}
	slices.SortFunc(out, func(a, b Nearby) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// LANError returns why the local network discovery could not start, or nil.
// The node works without it.
func (n *Node) LANError() error { return n.lanErr }
