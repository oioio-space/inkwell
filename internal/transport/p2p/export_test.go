package p2p

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-yamux/v5"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
)

// SetExtra makes New apply opts after its own, so tests can swap the network
// (for example for a simulated one).
func SetExtra(o *Options, opts ...libp2p.Option) { o.extra = opts }

// SetRewrite makes New rewrite the addresses the node advertises through f,
// so a simulated node behind a NAT advertises its public address.
func SetRewrite(o *Options, f func([]ma.Multiaddr) []ma.Multiaddr) { o.rewrite = f }

// SetOnDial makes the node tell f of each dial attempt of a wanted peer, with its addresses.
func SetOnDial(o *Options, f func(transport.PeerID, []string)) {
	o.onDial = func(id peer.ID, addrs []ma.Multiaddr) {
		name, _ := peerName(id)
		out := make([]string, len(addrs))
		for i, a := range addrs {
			out[i] = a.String()
		}
		f(name, out)
	}
}

// SetRelayExtra makes RunRelay apply opts after its own, as SetExtra does for
// a node.
func SetRelayExtra(o *RelayOptions, opts ...libp2p.Option) { o.extra = opts }

// Host returns the underlying libp2p host.
func (n *Node) Host() host.Host { return n.h }

// LibP2PID converts a PeerID to the libp2p form; ok is false if it is not
// the canonical encoding of a libp2p identity.
func LibP2PID(id transport.PeerID) (peer.ID, bool) { return parseID(id) }

// ResetOut resets the outbound stream to peer, as a remote that refuses it
// would. It reports whether there was one.
func (n *Node) ResetOut(id transport.PeerID) bool {
	p, ok := parseID(id)
	if !ok {
		return false
	}
	n.mu.Lock()
	ps := n.peers[p]
	if ps == nil || ps.out == nil {
		n.mu.Unlock()
		return false
	}
	s := ps.out
	n.mu.Unlock()
	_ = s.Reset()
	return true
}

// OutDirect reports whether the outbound stream to peer runs on a direct
// connection; ok is false if there is no such stream.
func (n *Node) OutDirect(id transport.PeerID) (direct, ok bool) {
	p, valid := parseID(id)
	if !valid {
		return false, false
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	ps := n.peers[p]
	if ps == nil || ps.out == nil {
		return false, false
	}
	return !isCircuit(ps.out.Conn().RemoteMultiaddr()), true
}

// MuxConfig is the yamux configuration the node and the relay pass to libp2p.
func MuxConfig() *yamux.Config { return muxConfig() }

// The muxer's constants, for the tests that derive their scenarios from them.
const (
	MuxWriteTimeout = muxWriteTimeout
	MuxKeepAlive    = muxKeepAlive
	StreamWindow    = streamWindow
)

// StallReading makes the node's application stop reading: it takes the whole
// receive budget, so the readers of its inbound streams wait for a frame's
// bytes while the connection and its keepalives carry on. It panics if the
// budget was not entirely free. ResumeReading undoes it.
func (n *Node) StallReading() {
	if !n.budget.TryAcquire(maxBuffered) {
		panic("p2p: StallReading: the receive budget is not free")
	}
}

// ResumeReading gives back the budget taken by StallReading.
func (n *Node) ResumeReading() { n.budget.Release(maxBuffered) }

// RelayViews is how many times the link sampler looked at the probe of the connection to the
// relay, since the node started.
func (n *Node) RelayViews() uint64 {
	var total uint64
	for _, c := range n.h.Network().Conns() {
		if _, ok := n.relays[c.RemotePeer()]; !ok {
			continue
		}
		if pc := n.probes.of(c.ID()); pc != nil {
			total += pc.views.Load()
		}
	}
	return total
}

// MemBook is a [BookStore] in memory, safe for concurrent use.
type MemBook struct {
	mu        sync.Mutex
	data, bad []byte
	reason    string // of the last quarantine
	saves     int
	fails     int // saves still to fail, see FailSaves
}

// FailSaves makes the next n saves fail, as a full disk does.
func (m *MemBook) FailSaves(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fails = n
}

// Load implements BookStore.
func (m *MemBook) Load() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return bytes.Clone(m.data), nil
}

// Save implements BookStore.
func (m *MemBook) Save(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fails > 0 {
		m.fails--
		return errors.New("disk full")
	}
	m.data = bytes.Clone(data)
	m.saves++
	return nil
}

// Quarantine implements BookStore.
func (m *MemBook) Quarantine(reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bad, m.data, m.reason = m.data, nil, reason
	return nil
}

// Set replaces the stored book.
func (m *MemBook) Set(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = bytes.Clone(data)
}

// Data returns the stored book.
func (m *MemBook) Data() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return bytes.Clone(m.data)
}

// Saves returns how many times the book was saved.
func (m *MemBook) Saves() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saves
}

// BookAddrs returns the addresses the address book holds for id, and when it was last seen.
func (n *Node) BookAddrs(id transport.PeerID) (addrs []string, seen time.Time) {
	p, ok := parseID(id)
	if !ok {
		return nil, time.Time{}
	}
	for _, e := range n.book.snapshot() {
		if e.ID == p.String() {
			return e.Addrs, time.Unix(e.Seen, 0)
		}
	}
	return nil, time.Time{}
}

// SightLAN hands info to the node as its mDNS service does when it sees a node.
func (n *Node) SightLAN(info peer.AddrInfo) { (*lanNotifee)(n).HandlePeerFound(info) }

// BookFile is the stored form of an address book holding, for each peer, its addresses seen at
// seen.
func BookFile(seen time.Time, peers map[transport.PeerID][]string) []byte {
	entries := make(map[peer.ID]bookEntry, len(peers))
	for id, addrs := range peers {
		p, _ := parseID(id)
		e := bookEntry{seen: seen.Unix(), in: []string{defaultScope}}
		for _, a := range addrs {
			e.addrs = append(e.addrs, ma.StringCast(a))
		}
		entries[p] = e
	}
	data, skipped, err := encodeBook(entries)
	if err != nil || skipped > 0 {
		panic(fmt.Sprint(skipped, err))
	}
	return data
}

// snapshot returns the entries of the book, sorted by peer, for the tests.
func (b *addrBook) snapshot() []bookPeer {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]bookPeer, 0, len(b.peers))
	for id, e := range b.peers {
		p := bookPeer{ID: id.String(), Seen: max(e.seen, e.lanSeen)}
		for _, a := range slices.Concat(e.addrs, e.lan) {
			p.Addrs = append(p.Addrs, a.String())
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(x, y bookPeer) int { return cmp.Compare(x.ID, y.ID) })
	return out
}

// discard is a logger for the tests that do not look at the logs.
var discard = slog.New(slog.DiscardHandler)
