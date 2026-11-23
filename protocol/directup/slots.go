// SPDX-License-Identifier: MPL-2.0

package directup

import (
	"slices"
	"time"
)

// Slots are the node-wide punch slots (H-5, H-16): every hole-punch round with a peer, whoever initiates,
// and every call of the node's own upgrader, needs its peer to hold one. A peer asks with [Slots.Ask]
// and holds one ticket in a FIFO queue until it is granted; a grant takes a token of the node-wide
// admission bucket, lasts a lease of 75 s at most, and allows 12 non-relay destinations, all the
// peer's rounds and calls together. Peers are named by any string the caller chooses. Slots is not
// safe for concurrent use.
type Slots struct {
	node  Bucket
	max   int
	held  map[string]*grant
	queue []string
	// seen holds the queued peers and when each last asked or gave up.
	seen map[string]time.Time
	// grants counts the grants made.
	grants int
}

// grant is a slot held by a peer.
type grant struct {
	until     time.Time
	allowance int
	// users count the asks that were granted and not released yet; a grant made to a peer whose round
	// gave up waiting has none, and is kept for the peer until its lease ends.
	users int
}

// NewSlots returns the slots of a node started at now: 4 slots, a full admission bucket.
func NewSlots(now time.Time) *Slots {
	return &Slots{
		node: NewBucket(NodeBurst, NodeRefill, now),
		max:  MaxSlots,
		held: make(map[string]*grant),
		seen: make(map[string]time.Time),
	}
}

// Ask asks a slot for p at now. It reports true if p holds one (it may share a grant it already holds);
// each true must be matched by a [Slots.Release]. Otherwise p keeps its ticket, or takes one at the end
// of the queue, and should ask again when the slots change.
func (s *Slots) Ask(p string, now time.Time) bool {
	s.Expire(now)
	if g := s.held[p]; g != nil {
		g.users++
		return true
	}
	if _, ok := s.seen[p]; !ok {
		s.queue = append(s.queue, p)
	}
	s.seen[p] = now
	s.grantAll(now)
	if g := s.held[p]; g != nil {
		g.users++
		return true
	}
	return false
}

// grantAll grants slots to the head of the queue while it can.
func (s *Slots) grantAll(now time.Time) {
	for s.grantHead(now) {
	}
}

// grantHead grants a slot to the first ticket, if a slot is free and the admission bucket has a token.
func (s *Slots) grantHead(now time.Time) bool {
	if len(s.queue) == 0 {
		return false
	}
	if len(s.held) >= s.max {
		return false
	}
	if !s.node.Take(now) {
		return false
	}
	p := s.queue[0]
	s.queue = s.queue[1:]
	delete(s.seen, p)
	s.held[p] = &grant{until: now.Add(Lease), allowance: GrantAllowance}
	s.grants++
	return true
}

// Grants returns the number of grants made since the start: each took a node-wide admission token.
func (s *Slots) Grants() int { return s.grants }

// Release ends one granted ask of p: the grant ends with its last one (early release).
func (s *Slots) Release(p string, now time.Time) {
	g := s.held[p]
	if g == nil {
		return
	}
	g.users = max(g.users-1, 0)
	if g.users == 0 {
		delete(s.held, p)
	}
	s.Expire(now)
}

// GiveUp records that the round of p waiting for a slot stopped waiting at now: its ticket keeps its
// place, and a grant made to it is kept for p until the lease ends.
func (s *Slots) GiveUp(p string, now time.Time) {
	if _, ok := s.seen[p]; ok {
		s.seen[p] = now
	}
}

// Drop removes the ticket of p, which is no longer wanted or relayed. A grant it holds ends with its
// lease or its release.
func (s *Slots) Drop(p string) { s.drop(p) }

func (s *Slots) drop(p string) {
	if _, ok := s.seen[p]; !ok {
		return
	}
	delete(s.seen, p)
	s.queue = slices.DeleteFunc(s.queue, func(q string) bool { return q == p })
}

// Expire ends the leases due by now and drops the tickets without a round for 20 min, then grants what
// it can.
func (s *Slots) Expire(now time.Time) {
	for p, g := range s.held {
		if !now.Before(g.until) {
			delete(s.held, p)
		}
	}
	for p, at := range s.seen {
		if now.Sub(at) >= TicketIdle {
			s.drop(p)
		}
	}
	s.grantAll(now)
}

// Holds reports whether p holds a slot at now.
func (s *Slots) Holds(p string, now time.Time) bool {
	s.Expire(now)
	return s.held[p] != nil
}

// Until returns when the grant p holds at now ends, and whether it holds one.
func (s *Slots) Until(p string, now time.Time) (time.Time, bool) {
	s.Expire(now)
	if g := s.held[p]; g != nil {
		return g.until, true
	}
	return time.Time{}, false
}

// Spend draws one non-relay destination for p from its grant's allowance; false if p holds no grant
// or its allowance is spent (H-16).
func (s *Slots) Spend(p string, now time.Time) bool {
	s.Expire(now)
	g := s.held[p]
	if g == nil {
		return false
	}
	if g.allowance == 0 {
		return false
	}
	g.allowance--
	return true
}

// Queued reports whether p holds a ticket.
func (s *Slots) Queued(p string) bool {
	_, ok := s.seen[p]
	return ok
}

// Held returns the number of slots held at now.
func (s *Slots) Held(now time.Time) int {
	s.Expire(now)
	return len(s.held)
}

// Next returns when the slots may change by themselves after now (a lease ends, an admission token
// comes back for a waiting ticket, a ticket expires), or the zero time if they will not.
func (s *Slots) Next(now time.Time) time.Time {
	s.Expire(now)
	var next time.Time
	soonest := func(t time.Time) {
		if next.IsZero() || t.Before(next) {
			next = t
		}
	}
	for _, g := range s.held {
		soonest(g.until)
	}
	for _, at := range s.seen {
		soonest(at.Add(TicketIdle))
	}
	if len(s.queue) > 0 && len(s.held) < s.max {
		soonest(s.node.Next(now))
	}
	return next
}
