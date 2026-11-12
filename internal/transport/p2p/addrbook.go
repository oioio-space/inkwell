package p2p

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/oioio-space/inkwell/internal/transport"
)

// The address book is what a device last saw of each other member: the dialable addresses the
// member announced, when, and in which projects it was a member then. It lets a device that
// restarted dial the members again (Node.Want carries no address) and outlives libp2p's peerstore,
// which forgets a peer's addresses 15 minutes after it disconnects. It belongs to the device, not
// to a project. Its file format and bounds are those of docs/plans/2026-11-12-rc1-reconnect.md §3
// and §4, with the changes of gates/RC1/SIMPLIFY.md.

// MaxBookSize bounds the stored address book: a larger one is refused before it is parsed.
const MaxBookSize = 96 << 10

const (
	bookVersion    = 1
	bookMaxPeers   = maxPeers // the member capacity of a project
	bookMaxAddrs   = 8
	bookMaxAddrLen = 128
	// bookMaxLAN bounds the unauthenticated (mDNS) addresses of an entry, in places of their own:
	// they never take nor evict an authenticated one.
	bookMaxLAN = 2
	// bookMaxScopes bounds the projects an entry is kept for; bookMaxScopeLen bounds a name.
	bookMaxScopes   = 8
	bookMaxScopeLen = 64
	// bookMaxAge is how long an entry is kept without being seen again.
	bookMaxAge = 90 * 24 * time.Hour
	// bookFuture is how far in the future a stored seen time may be (clock skew).
	bookFuture = 24 * time.Hour
	// bookSaveEvery is the shortest time between two saves of a changed book; bookSeenEvery is
	// how often a book whose seen times alone changed is saved.
	bookSaveEvery = 10 * time.Second
	bookSeenEvery = time.Hour
	// bookRetryMax caps the delay between two attempts to save after a failure, which starts at
	// bookSaveEvery and doubles.
	bookRetryMax = 10 * time.Minute
	// defaultScope is the project name of a node without Options.Scope.
	defaultScope = "local"
)

// BookStore keeps the address book of a device between runs (see [Options.Book]). Its methods are
// called from the node's goroutines, one at a time.
type BookStore interface {
	// Load returns the stored book, or nil if there is none yet.
	Load() ([]byte, error)
	// Save replaces the stored book with data, atomically: a later Load returns the previous
	// book or data, never a mix.
	Save(data []byte) error
	// Quarantine sets the stored book aside because it was refused, so that it can be examined;
	// the next Load returns none. reason is a category ("size", "syntax", "version", "bounds"),
	// never text of the book, which holds people's addresses.
	Quarantine(reason string) error
}

// bookEntry is what the book holds of one member. Only what came over an authenticated connection
// is stored; what mDNS announced, which anyone on the local network can forge, is kept apart, in
// memory only, in the room the authenticated addresses leave, and dialed after them, so that a
// sighting can neither evict nor outrank an address the member itself announced; they have
// places of their own, so a member whose authenticated addresses all went stale (it moved) is
// still found on the local network.
type bookEntry struct {
	seen  int64          // Unix seconds of the last authenticated announcement (stored)
	addrs []ma.Multiaddr // announced by the member over an authenticated connection (stored)
	in    []string       // the projects (Options.Scope) it was a member of then, latest first (stored)
	// lan are the addresses an mDNS sighting gave, at most bookMaxLAN, and lanSeen its time.
	lan     []ma.Multiaddr
	lanSeen int64
}

// The stored form, version 1.
type (
	bookFile struct {
		V     int        `json:"v"`
		Peers []bookPeer `json:"peers"`
	}
	bookPeer struct {
		ID    string   `json:"id"`
		Seen  int64    `json:"seen"`
		In    []string `json:"in"`
		Addrs []string `json:"addrs"`
	}
)

// bookRefusal is why a stored book was refused: a category for the logs and the quarantine, and
// the detail, which may quote the book and is never logged.
type bookRefusal struct {
	reason string
	err    error
}

func (r *bookRefusal) Error() string { return "p2p: address book (" + r.reason + "): " + r.err.Error() }
func (r *bookRefusal) Unwrap() error { return r.err }

func refuse(reason, format string, args ...any) error {
	return &bookRefusal{reason: reason, err: fmt.Errorf(format, args...)}
}

// decodeBook parses a stored book. The whole book is refused if any rule is broken (size,
// version, unknown or duplicate names, invalid UTF-8, trailing bytes, peer count, peer ID,
// addresses, projects, a negative seen time): it is then not a book this code wrote. The time
// rules (too old, in the future) are applied per entry by newAddrBook, never refuse the book.
func decodeBook(data []byte, self peer.ID) (map[peer.ID]bookEntry, error) {
	if len(data) > MaxBookSize {
		return nil, refuse("size", "%d bytes, more than %d", len(data), MaxBookSize)
	}
	var f bookFile
	if err := json.Unmarshal(data, &f, json.RejectUnknownMembers(true)); err != nil {
		return nil, &bookRefusal{reason: "syntax", err: err}
	}
	if f.V != bookVersion {
		return nil, refuse("version", "version %d, want %d", f.V, bookVersion)
	}
	if len(f.Peers) > bookMaxPeers {
		return nil, refuse("bounds", "%d peers, more than %d", len(f.Peers), bookMaxPeers)
	}
	out := make(map[peer.ID]bookEntry, len(f.Peers))
	for _, p := range f.Peers {
		id, e, err := decodeBookPeer(p, self)
		if err != nil {
			return nil, &bookRefusal{reason: "bounds", err: err}
		}
		if _, dup := out[id]; dup {
			return nil, refuse("bounds", "peer %s twice", p.ID)
		}
		out[id] = e
	}
	return out, nil
}

// decodeBookPeer checks one stored entry.
func decodeBookPeer(p bookPeer, self peer.ID) (peer.ID, bookEntry, error) {
	if _, err := KeyFromID(transport.PeerID(p.ID)); err != nil {
		return "", bookEntry{}, err
	}
	id, _ := parseID(transport.PeerID(p.ID)) // valid: KeyFromID parsed it
	switch {
	case id == self:
		return "", bookEntry{}, errors.New("our own peer id")
	case p.Seen < 0:
		return "", bookEntry{}, fmt.Errorf("peer %s seen at %d", p.ID, p.Seen)
	case len(p.Addrs) == 0 || len(p.Addrs) > bookMaxAddrs:
		return "", bookEntry{}, fmt.Errorf("peer %s has %d addresses, want 1 to %d", p.ID, len(p.Addrs), bookMaxAddrs)
	case len(p.In) == 0 || len(p.In) > bookMaxScopes:
		return "", bookEntry{}, fmt.Errorf("peer %s is in %d projects, want 1 to %d", p.ID, len(p.In), bookMaxScopes)
	}
	e := bookEntry{seen: p.Seen, addrs: make([]ma.Multiaddr, 0, len(p.Addrs))}
	for _, s := range p.In {
		if !validScope(s) || slices.Contains(e.in, s) {
			return "", bookEntry{}, fmt.Errorf("peer %s: project %q invalid or twice", p.ID, s)
		}
		e.in = append(e.in, s)
	}
	for _, s := range p.Addrs {
		a, err := parseBookAddr(s)
		if err != nil {
			return "", bookEntry{}, fmt.Errorf("peer %s: %w", p.ID, err)
		}
		if slices.ContainsFunc(e.addrs, a.Equal) {
			return "", bookEntry{}, fmt.Errorf("peer %s: address %s twice", p.ID, s)
		}
		e.addrs = append(e.addrs, a)
	}
	return id, e, nil
}

// validScope reports whether s may name a project in the book: 1 to 64 ASCII letters, digits,
// '-' or '_'.
func validScope(s string) bool {
	if len(s) == 0 || len(s) > bookMaxScopeLen {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return !asciiAlnum(r) && r != '-' && r != '_' }) < 0
}

// asciiAlnum reports whether r is an ASCII letter or digit.
func asciiAlnum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// parseBookAddr parses a stored address, which must be in canonical form and dialable (see
// bookAddrOK).
func parseBookAddr(s string) (ma.Multiaddr, error) {
	if len(s) > bookMaxAddrLen {
		return nil, fmt.Errorf("address of %d bytes, more than %d", len(s), bookMaxAddrLen)
	}
	a, err := ma.NewMultiaddr(s)
	if err != nil {
		return nil, fmt.Errorf("address %q: %w", s, err)
	}
	if a.String() != s || !bookAddrOK(a) {
		return nil, fmt.Errorf("address %q is not a direct dialable address", s)
	}
	return a, nil
}

// bookAddrOK reports whether a may be kept in the book: a direct address of a peer, an IP (neither
// unspecified nor multicast) or a DNS hostname (ASCII letters, digits and hyphens), then a non-zero
// TCP port or a UDP port with QUIC v1, and nothing else (no relay circuit, no /p2p/ suffix); its
// text, at most 128 bytes, parses back to it, so whatever is recorded can be stored and read back.
func bookAddrOK(a ma.Multiaddr) bool {
	switch {
	case len(a) == 2 && a[1].Code() == ma.P_TCP:
	case len(a) == 3 && a[1].Code() == ma.P_UDP && a[2].Code() == ma.P_QUIC_V1:
	default:
		return false
	}
	if a[1].Value() == "0" {
		return false
	}
	switch a[0].Code() {
	case ma.P_IP4, ma.P_IP6:
		ip := net.IP(a[0].RawValue())
		if ip.IsUnspecified() || ip.IsMulticast() {
			return false
		}
	case ma.P_DNS, ma.P_DNS4, ma.P_DNS6:
		if !validHostname(a[0].Value()) {
			return false
		}
	default:
		return false
	}
	s := a.String()
	back, err := ma.NewMultiaddr(s)
	return len(s) <= bookMaxAddrLen && err == nil && back.Equal(a)
}

// validHostname reports whether h is an ASCII hostname: labels of 1 to 63 letters, digits and
// hyphens, not starting or ending with a hyphen, 253 bytes at most.
func validHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 {
		return false
	}
	for label := range strings.SplitSeq(h, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range []byte(label) {
			if !asciiAlnum(rune(c)) && c != '-' {
				return false
			}
		}
	}
	return true
}

// lanAddrOK reports whether an address an mDNS sighting gave may be kept: one of bookAddrOK on a
// private or link-local IPv4 literal, or a private (unique local) IPv6 one; never loopback, public
// or DNS, which the local network has no business announcing.
func lanAddrOK(a ma.Multiaddr) bool {
	if !bookAddrOK(a) || (a[0].Code() != ma.P_IP4 && a[0].Code() != ma.P_IP6) {
		return false
	}
	ip := net.IP(a[0].RawValue())
	return !ip.IsLoopback() && (ip.IsPrivate() || (ip.To4() != nil && ip.IsLinkLocalUnicast()))
}

// encodeBook is the stored form of entries, sorted by peer ID: the authenticated part of each
// entry kept for at least one project. An entry that cannot be encoded is left out and counted in
// skipped, so that one bad entry never stops the others from being saved.
func encodeBook(entries map[peer.ID]bookEntry) (data []byte, skipped int, err error) {
	f := bookFile{V: bookVersion, Peers: make([]bookPeer, 0, len(entries))}
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		e := entries[id]
		if len(e.addrs) == 0 || len(e.in) == 0 {
			continue // seen by mDNS only: nothing authenticated to keep
		}
		p := bookPeer{ID: id.String(), Seen: e.seen, In: e.in, Addrs: make([]string, len(e.addrs))}
		for i, a := range e.addrs {
			p.Addrs[i] = a.String()
		}
		if _, err := json.Marshal(p); err != nil {
			skipped++
			continue
		}
		f.Peers = append(f.Peers, p)
	}
	data, err = json.Marshal(f)
	return data, skipped, err
}

// addrBook is the in-memory address book of a node, saved to its store, if any, at most every
// bookSaveEvery (see run). It is safe for concurrent use; its lock is a leaf, except that note
// and noteLAN call their membership check under it (it must not call into the node).
type addrBook struct {
	store BookStore // nil: the book lives in memory only
	self  peer.ID
	log   *slog.Logger
	wake  chan struct{} // capacity 1: "the book changed"

	// saveMu serialises a snapshot and its Save, so that saves land in the order of the
	// snapshots: a revocation's save is never overwritten by an older snapshot.
	saveMu sync.Mutex

	mu    sync.Mutex
	peers map[peer.ID]bookEntry
	// changed: the peers or their addresses changed since the last save; seenOnly: only seen
	// times did.
	changed, seenOnly bool
	// hold: the clock disagreed with the stored book at load (see newAddrBook); nothing is pruned,
	// nor saved but a revocation, until a member is seen again, which shows the network, and so
	// the clock, works. revoked: forget removed something since the last save.
	hold, revoked bool
	lastSave      time.Time
}

// newAddrBook returns the book of the node self kept in store (nil for none), loaded at now. A
// stored book that cannot be decoded is quarantined and the book starts empty: starting is never
// blocked by it. Every entry is kept, whatever the open project: the book belongs to the device.
// Entries not seen for more than bookMaxAge are dropped, unless the clock is implausible against
// the book (a seen time beyond now, so it went back; or every entry would go): the book is then
// kept as it is, unsaved but for revocations, until a member is seen (see release).
func newAddrBook(store BookStore, self peer.ID, now time.Time, log *slog.Logger) *addrBook {
	b := &addrBook{store: store, self: self, log: log, wake: make(chan struct{}, 1), peers: make(map[peer.ID]bookEntry)}
	if store == nil {
		return b
	}
	data, err := store.Load()
	if err != nil || len(data) == 0 {
		return b // unreadable or missing: an empty book, the stored one left as it is
	}
	peers, err := decodeBook(data, self)
	if err != nil {
		reason := "syntax"
		if r, ok := errors.AsType[*bookRefusal](err); ok {
			reason = r.reason
		}
		b.log.Warn("address book refused", "reason", reason)
		_ = store.Quarantine(reason) // best effort: the book is refused either way
		return b
	}
	var newest int64
	for _, e := range peers {
		newest = max(newest, e.seen)
	}
	b.peers = peers
	switch {
	case newest > now.Add(bookFuture).Unix() || (len(peers) > 0 && newest < now.Add(-bookMaxAge).Unix()):
		b.hold = true
		b.log.Warn("address book older than the clock allows, or newer: kept unsaved until a member is seen", "entries", len(peers))
	case b.prune(now):
		b.changed = true // the dropped entries leave the disk soon
		b.signal()
	}
	b.log.Info("address book loaded", "entries", len(b.peers))
	return b
}

// signal wakes run.
func (b *addrBook) signal() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// usable returns the addresses of addrs that ok accepts for id, without the /p2p/ suffix naming
// id; one naming another peer is not usable.
func usable(id peer.ID, addrs []ma.Multiaddr, ok func(ma.Multiaddr) bool) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, min(len(addrs), bookMaxAddrs))
	for _, a := range addrs {
		direct, named := peer.SplitAddr(a)
		if direct == nil || (named != "" && named != id) || !ok(direct) {
			continue
		}
		out = append(out, direct)
	}
	return out
}

// merge returns fresh then old, without duplicates or the addresses of not, up to limit.
func merge(fresh, old, not []ma.Multiaddr, limit int) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, limit)
	for _, a := range slices.Concat(fresh, old) {
		if len(out) == limit {
			break
		}
		if !slices.ContainsFunc(out, a.Equal) && !slices.ContainsFunc(not, a.Equal) {
			out = append(out, a)
		}
	}
	return out
}

// note records that id, a member of project scope, announced addrs at now over an authenticated
// connection, freshest first: the usable ones come first, then the entry's earlier ones, up to
// bookMaxAddrs; the mDNS addresses make room. A peer without a usable address is not added. Beyond
// bookMaxPeers an entry is evicted (see evictLocked). member is checked under the book's lock, so
// that a revocation followed by forget cannot be undone by a note decided before it.
func (b *addrBook) note(id peer.ID, addrs []ma.Multiaddr, now time.Time, scope string, member func() bool) {
	fresh := usable(id, addrs, bookAddrOK)
	if id == b.self || !validScope(scope) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !member() {
		return
	}
	old, known := b.peers[id]
	if len(fresh) == 0 && len(old.addrs) == 0 {
		return
	}
	e := old
	e.addrs = merge(fresh, old.addrs, nil, bookMaxAddrs)
	e.seen = max(old.seen, now.Unix())
	e.in = slices.Insert(slices.DeleteFunc(slices.Clone(old.in), func(s string) bool { return s == scope }), 0, scope)
	e.in = e.in[:min(len(e.in), bookMaxScopes)]
	e.lan = merge(nil, old.lan, e.addrs, bookMaxLAN)
	b.peers[id] = e
	if b.hold {
		b.releaseLocked(now)
	}
	switch {
	case !slices.EqualFunc(e.addrs, old.addrs, ma.Multiaddr.Equal) || !slices.Equal(e.in, old.in):
		b.changed = true
		if !known {
			b.evictLocked(id)
		}
		b.signal()
	case e.seen != old.seen:
		b.seenOnly = true
		if now.Sub(b.lastSave) >= bookSeenEvery {
			b.signal()
		}
	}
}

// releaseLocked ends the hold once a member was seen at now (the clock can be trusted again): a
// seen time in the future is brought back to now, and the book is saved, pruned, at the next
// flush.
func (b *addrBook) releaseLocked(now time.Time) {
	b.hold, b.changed = false, true
	for id, e := range b.peers {
		if e.seen > now.Add(bookFuture).Unix() {
			e.seen = now.Unix()
			b.peers[id] = e
		}
	}
	b.signal()
}

// noteLAN records that mDNS announced addrs for id at now. They are unauthenticated: only those of
// lanAddrOK, in at most bookMaxLAN places of their own, and not stored. It reports whether any
// address was usable. member is checked under the book's lock.
func (b *addrBook) noteLAN(id peer.ID, addrs []ma.Multiaddr, now time.Time, member func() bool) bool {
	fresh := usable(id, addrs, lanAddrOK)
	if id == b.self || len(fresh) == 0 {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !member() {
		return false
	}
	e, known := b.peers[id]
	e.lan = merge(fresh, e.lan, e.addrs, bookMaxLAN)
	e.lanSeen = now.Unix()
	b.peers[id] = e
	if !known {
		b.evictLocked(id)
	}
	return true
}

// evictLocked removes entries while the book is over bookMaxPeers: first those that mDNS alone
// gave (keep included), the one seen longest ago; then, among the others, the one whose
// authenticated announcement is the oldest, other than keep.
func (b *addrBook) evictLocked(keep peer.ID) {
	for len(b.peers) > bookMaxPeers {
		victim, ok := b.oldestLocked(func(_ peer.ID, e bookEntry) bool { return len(e.addrs) == 0 }, func(e bookEntry) int64 { return e.lanSeen })
		if !ok {
			victim, _ = b.oldestLocked(func(id peer.ID, _ bookEntry) bool { return id != keep }, func(e bookEntry) int64 { return e.seen })
		}
		delete(b.peers, victim)
	}
}

// oldestLocked returns the eligible entry whose time at is the earliest, if there is one.
func (b *addrBook) oldestLocked(eligible func(peer.ID, bookEntry) bool, at func(bookEntry) int64) (peer.ID, bool) {
	var oldest peer.ID
	found := false
	for id, e := range b.peers {
		if eligible(id, e) && (!found || at(e) < at(b.peers[oldest])) {
			oldest, found = id, true
		}
	}
	return oldest, found
}

// dialAddrs returns the addresses the book holds for id: the authenticated ones, and apart the
// ones mDNS gave, which are dialed after them.
func (b *addrBook) dialAddrs(id peer.ID) (auth, lan []ma.Multiaddr) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.peers[id]
	return slices.Clone(e.addrs), slices.Clone(e.lan)
}

// forget removes project scope from the entry of id, and the entry once it is kept for no project
// any more. It reports whether the stored part changed.
func (b *addrBook) forget(id peer.ID, scope string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.peers[id]
	if !ok || !slices.Contains(e.in, scope) {
		return false
	}
	e.in = slices.DeleteFunc(slices.Clone(e.in), func(s string) bool { return s == scope })
	if len(e.in) == 0 {
		delete(b.peers, id)
	} else {
		b.peers[id] = e
	}
	b.changed, b.revoked = true, true
	return true
}

// prune drops what was not seen for more than bookMaxAge at now (the authenticated part and the
// mDNS part apart), and reports whether the stored part changed. The caller holds mu or owns b.
func (b *addrBook) prune(now time.Time) bool {
	oldest := now.Add(-bookMaxAge).Unix()
	stored := false
	for id, e := range b.peers {
		if e.seen < oldest && len(e.addrs) > 0 {
			e.addrs, e.in, e.seen, stored = nil, nil, 0, true
		}
		if e.lanSeen < oldest {
			e.lan = nil
		}
		if len(e.addrs) == 0 && len(e.lan) == 0 {
			delete(b.peers, id)
		} else {
			b.peers[id] = e
		}
	}
	return stored
}

// flush saves the book if it changed, or, with all, if only its seen times did or the last of
// them was saved bookSeenEvery ago, and reports whether the book is saved (false: the save failed
// and is to be retried, see run). While the book is on hold (see newAddrBook) only a revocation
// is saved: the book as it was loaded, minus what was forgotten, unpruned.
func (b *addrBook) flush(now time.Time, all bool) bool {
	if b.store == nil {
		return true
	}
	b.saveMu.Lock()
	defer b.saveMu.Unlock()
	b.mu.Lock()
	if b.hold && !b.revoked {
		b.mu.Unlock()
		return true
	}
	if !b.hold && b.prune(now) {
		b.changed = true
	}
	if due := b.changed || (b.seenOnly && (all || now.Sub(b.lastSave) >= bookSeenEvery)); !due {
		b.mu.Unlock()
		return true
	}
	data, skipped, err := encodeBook(b.peers)
	b.changed, b.seenOnly, b.revoked, b.lastSave = false, false, false, now
	b.mu.Unlock()
	if skipped > 0 {
		b.log.Warn("address book entries left out of the save", "entries", skipped)
	}
	if err == nil {
		err = b.store.Save(data)
	}
	if err != nil {
		b.log.Warn("saving the address book failed")
		b.mu.Lock()
		b.changed, b.revoked = true, true // retried by run, whatever comes next
		b.mu.Unlock()
		return false
	}
	return true
}

// run saves the book as it changes, at most every bookSaveEvery, until ctx is done; it saves it
// a last time then. A failed save is retried on its own, first after bookSaveEvery, then twice as
// long each time, up to bookRetryMax. Without a store it returns at once.
func (b *addrBook) run(ctx context.Context) {
	if b.store == nil {
		return
	}
	var retry time.Duration // after a failed save: the delay before the next attempt
	for {
		var again <-chan time.Time
		if retry > 0 {
			again = time.After(retry)
		}
		select {
		case <-b.wake:
		case <-again:
		case <-ctx.Done():
			b.flush(time.Now(), true)
			return
		}
		b.mu.Lock()
		wait := time.Until(b.lastSave.Add(bookSaveEvery))
		b.mu.Unlock()
		if wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				b.flush(time.Now(), true)
				return
			}
		}
		if b.flush(time.Now(), false) {
			retry = 0
		} else {
			retry = min(max(2*retry, bookSaveEvery), bookRetryMax)
		}
	}
}

// member reports whether Options.Admit accepts id as a member, not only through the window of
// [Node.SetOpen]: only members are kept in the address book.
func (n *Node) member(id peer.ID) bool {
	name, ok := peerName(id)
	return ok && (n.opts.Admit == nil || n.opts.Admit(name))
}

// scope names the project Options.Admit checks the membership of (see Options.Scope).
func (n *Node) scope() string {
	if n.opts.Scope == nil {
		return defaultScope
	}
	return n.opts.Scope()
}

// onIdentified keeps the addresses a peer announced through identify, over the authenticated
// connection: those of its signed peer record when it sent one, its listen addresses otherwise,
// filtered as identify itself does for the peerstore (reachable only, see reachableFrom). This is
// what the address book records of a member: not the addresses libp2p merely dialed, which its
// peerstore mixes in. They are kept while the peer is connected, so that a peer that becomes a
// member while connected (an invitation) is recorded by the next Want.
func (n *Node) onIdentified(e event.EvtPeerIdentificationCompleted) {
	if _, relay := n.relays[e.Peer]; relay || e.Conn == nil {
		return
	}
	addrs := announced(e)
	if !n.connected(e.Peer) { // gone already: onDisconnected may have run
		return
	}
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.heard[e.Peer] = addrs
	n.mu.Unlock()
	n.remember(e.Peer)
}

// announced returns the addresses identify's event e gives of its peer: those of the signed peer
// record when there is one of the peer (identify checked its signature), its listen addresses
// otherwise, filtered by reachableFrom. e.Conn is not nil.
func announced(e event.EvtPeerIdentificationCompleted) []ma.Multiaddr {
	addrs := e.ListenAddrs
	if e.SignedPeerRecord != nil {
		if r, err := e.SignedPeerRecord.Record(); err == nil {
			if rec, ok := r.(*peer.PeerRecord); ok && rec.PeerID == e.Peer {
				addrs = rec.Addrs
			}
		}
	}
	return reachableFrom(slices.Clone(addrs), e.Conn.RemoteMultiaddr())
}

// reachableFrom keeps the addresses of a peer that a node reaching it at remote can use, as
// libp2p's identify does: all from the loopback interface, no loopback one from a private
// address, public ones only from a public address.
func reachableFrom(addrs []ma.Multiaddr, remote ma.Multiaddr) []ma.Multiaddr {
	switch {
	case manet.IsIPLoopback(remote):
		return addrs
	case manet.IsPrivateAddr(remote):
		return slices.DeleteFunc(addrs, manet.IsIPLoopback)
	case manet.IsPublicAddr(remote):
		return slices.DeleteFunc(addrs, func(a ma.Multiaddr) bool { return !manet.IsPublicAddr(a) })
	}
	return addrs
}

// remember records in the address book the addresses the member id announced (see onIdentified).
// In RelayOnly mode nothing direct is dialed, so nothing is kept. It calls Admit: not under mu.
func (n *Node) remember(id peer.ID) {
	if n.opts.RelayOnly {
		return
	}
	n.mu.Lock()
	addrs := n.heard[id]
	n.mu.Unlock()
	if len(addrs) > 0 {
		n.book.note(id, addrs, time.Now(), n.scope(), func() bool { return n.member(id) })
	}
}

// rememberLAN records what mDNS announced for the member id (unauthenticated, see bookEntry) and
// reports whether an address was usable.
func (n *Node) rememberLAN(id peer.ID, addrs []ma.Multiaddr) bool {
	return !n.opts.RelayOnly && n.book.noteLAN(id, addrs, time.Now(), func() bool { return n.member(id) })
}
