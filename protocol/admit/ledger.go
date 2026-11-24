// SPDX-License-Identifier: MPL-2.0

package admit

import (
	"cmp"
	"maps"
	"net/netip"
	"slices"
)

// Nanos is a moment or a span on the caller's clock, in nanoseconds: the ledger has no clock of its own, and
// the caller passes a monotonic reading (time.Since of a fixed start, for one).
type Nanos int64

// Config is the set of bounds a [Ledger] keeps. A zero bound is no bound.
type Config struct {
	// PerRelay, PerIP and PerWide bound the unverified connections of one relay origin, one IP origin and
	// one IPv6 /48; Pending bounds them in all (RF-3).
	PerRelay, PerIP, PerWide, Pending int
	// Deadline is how long an unverified connection holds its place at most (RF-4).
	Deadline Nanos
	// Burst is the size of each origin's bucket and Refill the period of one token (RF-5).
	Burst  int
	Refill Nanos
	// Members bounds the member connections in all; Share of them are reserved for each roster member and
	// PerMember bounds one member's (RF-6).
	Members, Share, PerMember int
	// Relays bounds the configured relays' connections in all, PerRelayConns one relay's (RF-6).
	Relays, PerRelayConns int
}

// ID names a connection of a [Ledger].
type ID uint64

// Dir is the direction of a connection.
type Dir uint8

// The directions.
const (
	Inbound Dir = iota + 1
	Outbound
)

// Class is what a connection's handshake proved its peer to be.
type Class uint8

// The classes.
const (
	// Stranger is a peer that is neither a member nor a configured relay: its connection stays unverified
	// until it closes (RF-3).
	Stranger Class = iota + 1
	// Member is a peer of the membership list (not of an invitation window).
	Member
	// Relay is a configured relay.
	Relay
)

// state is where a connection is counted.
type state uint8

const (
	opened  state = iota + 1 // outbound, before its peer is set: counted nowhere
	pending                  // unverified, holding its place
	expired                  // unverified, past its deadline: it holds no place any more (RF-4)
	member                   // a member connection
	relay                    // a configured relay's connection
	other                    // an outbound connection to a stranger, or one refused at its handshake
)

type conn struct {
	origin Origin // RF-1; zero for an outbound connection
	opened Nanos
	state  state
	peer   string
}

// Ledger keeps the counts of one node's connections (RF-1 to RF-6). The zero Ledger is not usable: make one
// with [New].
type Ledger struct {
	cfg     Config
	next    ID
	conns   map[ID]*conn
	pend    map[ID]*conn       // the connections that hold a place: state pending
	buckets map[Origin]*bucket // origins owed a token; a full bucket is dropped
	members map[string]int     // member connections per member
	relays  map[string]int     // connections per configured relay
	nMember int
	nRelay  int
	roster  map[string]bool
	swept   Nanos // the last sweep of the buckets
}

// New returns an empty ledger for cfg.
func New(cfg Config) *Ledger {
	return &Ledger{
		cfg: cfg, conns: map[ID]*conn{}, pend: map[ID]*conn{}, buckets: map[Origin]*bucket{},
		members: map[string]int{}, relays: map[string]int{}, roster: map[string]bool{},
	}
}

// SetRoster names the members the shares are reserved for (RF-6): the membership list, at most
// Members / Share of them, in sorted order. When the roster grows while the surplus is taken, the shares now
// owed no longer fit: it returns the member connections to close so that they fit again (model SharesKept),
// each taken from the member that holds the most above what is its own (its share if it is in the roster,
// nothing otherwise), its newest first. They stay counted until they close.
func (l *Ledger) SetRoster(peers []string) []ID {
	peers = slices.Sorted(slices.Values(peers))
	peers = slices.Compact(peers)
	if l.cfg.Share > 0 { // no member bound (Members 0) keeps no roster
		peers = peers[:min(len(peers), l.cfg.Members/l.cfg.Share)]
	}
	l.roster = make(map[string]bool, len(peers))
	for _, p := range peers {
		l.roster[p] = true
	}
	return l.shed()
}

// shed returns the member connections to close for every roster member's unused share to fit.
func (l *Ledger) shed() []ID {
	if l.cfg.Members <= 0 {
		return nil
	}
	excess := l.nMember + l.owed("") - l.cfg.Members
	held := map[string][]ID{} // each member's connections, newest first
	for c, k := range l.conns {
		if k.state == member {
			held[k.peer] = append(held[k.peer], c)
		}
	}
	for _, cs := range held {
		slices.SortFunc(cs, func(a, b ID) int { return cmp.Or(cmp.Compare(l.conns[b].opened, l.conns[a].opened), cmp.Compare(b, a)) })
	}
	above := func(p string) int {
		if l.roster[p] {
			return len(held[p]) - l.cfg.Share
		}
		return len(held[p])
	}
	var out []ID
	for ; excess > 0; excess-- {
		best := ""
		for p := range held {
			if a := above(p); a > 0 && (best == "" || a > above(best) || (a == above(best) && p < best)) {
				best = p
			}
		}
		if best == "" {
			break
		}
		out = append(out, held[best][0])
		held[best] = held[best][1:]
	}
	return out
}

// Open records the opening of a connection at now. An inbound one is charged to its origin o (RF-1) and
// takes a token from o's bucket (RF-5), or is refused if o, the IPv6 /48 or the node is at its pending bound
// or o's bucket is empty (RF-3). An outbound one is charged to nothing: o is ignored (RF-1, RF-2).
func (l *Ledger) Open(now Nanos, dir Dir, o Origin) (ID, error) {
	l.expire(now)
	if dir != Inbound {
		return l.add(&conn{opened: now, state: opened}), nil
	}
	if r := l.refusal(now, o); r != 0 {
		return 0, &RefusedError{Reason: r}
	}
	c := l.add(&conn{origin: o, opened: now, state: pending})
	l.pend[c] = l.conns[c]
	if l.cfg.Burst > 0 {
		b := l.buckets[o]
		if b == nil {
			b = &bucket{}
			l.buckets[o] = b
		}
		b.take(now, c)
	}
	return c, nil
}

// refusal is why an inbound opening from o is refused at now, or 0.
func (l *Ledger) refusal(now Nanos, o Origin) Reason {
	quota := l.cfg.PerIP
	if o.Relay != "" {
		quota = l.cfg.PerRelay
	}
	switch {
	case quota > 0 && l.pendingOn(o) >= quota:
		return ReasonOrigin
	case l.cfg.PerWide > 0 && o.Wide.IsValid() && l.pendingWide(o.Wide) >= l.cfg.PerWide:
		return ReasonWide
	case l.cfg.Pending > 0 && len(l.pend) >= l.cfg.Pending:
		return ReasonPending
	case l.cfg.Burst > 0 && l.tokens(now, o) <= 0:
		return ReasonRate
	}
	return 0
}

func (l *Ledger) add(c *conn) ID {
	l.next++
	l.conns[l.next] = c
	return l.next
}

// Identify records, at now, what c's handshake proved its peer to be (RF-2). A member's or a configured
// relay's connection leaves every pending count and gets its token back if no period has paid it yet; it is
// then refused if the member, or the relays, have no room left (RF-6). A stranger's inbound connection stays
// unverified; an outbound one to a stranger is counted nowhere. An unknown or already identified c is left
// alone.
func (l *Ledger) Identify(now Nanos, c ID, peer string, class Class) error {
	l.expire(now)
	k := l.conns[c]
	if k == nil || (k.state != opened && k.state != pending && k.state != expired) {
		return nil
	}
	k.peer = peer
	if class == Stranger {
		if k.state == opened {
			k.state = other
		}
		return nil
	}
	l.unpend(c, k)
	if b := l.bucketAt(now, k.origin); b != nil && b.refund(c) && len(b.debts) == 0 {
		delete(l.buckets, k.origin)
	}
	if class == Relay {
		if l.relayRoom(peer) {
			k.state = relay
			l.relays[peer]++
			l.nRelay++
			return nil
		}
		k.state = other
		return &RefusedError{Reason: ReasonRelays}
	}
	if !l.room(peer) {
		k.state = other
		return &RefusedError{Reason: ReasonMembers}
	}
	k.state = member
	l.members[peer]++
	l.nMember++
	return nil
}

// room reports whether member p may have one more member connection (RF-6): a roster member's first Share
// are its own and need only room in all; any other comes from the surplus, what no other roster member's
// unused share holds.
func (l *Ledger) room(p string) bool {
	n := l.members[p]
	if l.cfg.PerMember > 0 && n >= l.cfg.PerMember {
		return false
	}
	if l.cfg.Members <= 0 {
		return true
	}
	if l.roster[p] && n < l.cfg.Share {
		return l.nMember < l.cfg.Members
	}
	return l.nMember+l.owed(p) < l.cfg.Members
}

// owed is the connections the roster members other than p have not used of their shares.
func (l *Ledger) owed(p string) int {
	n := 0
	for x := range l.roster {
		if x != p {
			n += max(l.cfg.Share-l.members[x], 0)
		}
	}
	return n
}

func (l *Ledger) relayRoom(p string) bool {
	return (l.cfg.PerRelayConns <= 0 || l.relays[p] < l.cfg.PerRelayConns) && (l.cfg.Relays <= 0 || l.nRelay < l.cfg.Relays)
}

// Close records that c ended at now: its place, member or relay connection is free again. Its token is not
// given back: a period pays it (RF-5).
func (l *Ledger) Close(now Nanos, c ID) {
	k := l.conns[c]
	if k == nil {
		return
	}
	delete(l.conns, c)
	switch k.state {
	case pending:
		l.unpend(c, k)
	case member:
		l.nMember--
		if l.members[k.peer]--; l.members[k.peer] == 0 {
			delete(l.members, k.peer)
		}
	case relay:
		l.nRelay--
		if l.relays[k.peer]--; l.relays[k.peer] == 0 {
			delete(l.relays, k.peer)
		}
	}
	l.expire(now)
}

func (l *Ledger) unpend(c ID, k *conn) {
	delete(l.pend, c)
	k.state = expired
}

// expire ends the places held past the deadline (RF-4) and drops the buckets that are full again.
func (l *Ledger) expire(now Nanos) {
	if d := l.cfg.Deadline; d > 0 {
		for c, k := range l.pend {
			if now-k.opened >= d {
				l.unpend(c, k)
			}
		}
	}
	// A bucket is paid when it is read (bucketAt); the sweep that drops the full ones runs once a period, so
	// that an opening costs the pending places, not every origin owed a token.
	if now-l.swept >= l.cfg.Refill {
		l.swept = now
		for o := range l.buckets {
			l.bucketAt(now, o)
		}
	}
}

// bucketAt returns o's bucket with the periods ended by now paid, or nil if it is full (and then dropped).
func (l *Ledger) bucketAt(now Nanos, o Origin) *bucket {
	b := l.buckets[o]
	if b == nil {
		return nil
	}
	b.pay(now, l.cfg.Refill)
	if len(b.debts) == 0 {
		delete(l.buckets, o)
		return nil
	}
	return b
}

func (l *Ledger) pendingOn(o Origin) int {
	n := 0
	for _, k := range l.pend {
		if k.origin == o {
			n++
		}
	}
	return n
}

func (l *Ledger) pendingWide(w netip.Prefix) int {
	n := 0
	for _, k := range l.pend {
		if k.origin.Wide == w {
			n++
		}
	}
	return n
}

func (l *Ledger) tokens(now Nanos, o Origin) int {
	b := l.bucketAt(now, o)
	if b == nil {
		return l.cfg.Burst
	}
	return l.cfg.Burst - len(b.debts)
}

// Stats is what a [Ledger] holds at one moment.
type Stats struct {
	// Pending is the unverified connections that hold a place, in all and per origin.
	Pending   int
	PerOrigin map[Origin]int
	// Members is the member connections in all, PerMember per member; Relays the relays' connections.
	Members   int
	PerMember map[string]int
	Relays    int
}

// Stats returns the counts at now.
func (l *Ledger) Stats(now Nanos) Stats {
	l.expire(now)
	s := Stats{Pending: len(l.pend), PerOrigin: map[Origin]int{}, Members: l.nMember, PerMember: maps.Clone(l.members), Relays: l.nRelay}
	for _, k := range l.pend {
		s.PerOrigin[k.origin]++
	}
	return s
}

// Tokens returns the tokens left in o's bucket at now.
func (l *Ledger) Tokens(now Nanos, o Origin) int {
	l.expire(now)
	return l.tokens(now, o)
}

// Peer returns the peer c's handshake named, or "" if there was none (or c is not open).
func (l *Ledger) Peer(c ID) string {
	if k := l.conns[c]; k != nil {
		return k.peer
	}
	return ""
}

// Owes reports whether c's admission token is still owed at now: no period has paid it and no handshake
// gave it back.
func (l *Ledger) Owes(now Nanos, c ID) bool {
	l.expire(now)
	k := l.conns[c]
	if k == nil {
		return false
	}
	b := l.bucketAt(now, k.origin)
	return b != nil && b.owes(c)
}
