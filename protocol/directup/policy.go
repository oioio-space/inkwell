// SPDX-License-Identifier: MPL-2.0

package directup

import "time"

// Obs is what the transport observes of one peer when it steps its [Policy].
type Obs struct {
	// Enabled: the node has relays and is not RelayOnly, so it may hole punch at all.
	Enabled bool
	// Wanted: the peer is one the node keeps connected (a member).
	Wanted bool
	// Relayed: the session (the outbound sync stream) runs on a circuit. Direct: on another connection.
	// A stale direct connection to a dead process does not make the session direct.
	Relayed, Direct bool
	// In and Out: the node holds an inbound, an outbound circuit to the peer.
	In, Out bool
	// LocalReady: the node's own /libp2p/dcutr handler is registered.
	LocalReady bool
	// RemotePublic: the peer announced a public non-circuit address (identify).
	RemotePublic bool
	// Idle: the app is not active (no input for 5 min, or the machine asleep).
	Idle bool
}

// Event is something the transport saw that may make a call worth trying sooner (H-9).
type Event uint8

// The events. E1 (the session up on a circuit) is not sent: [Policy.Step] sees it.
const (
	E2      Event = 2 // identify completed with the peer
	E3      Event = 3 // the peer's protocols now list /libp2p/dcutr (identify push)
	E4      Event = 4 // the node's own /libp2p/dcutr handler was registered
	E7      Event = 7 // a new circuit connection to the peer (it restarted or moved): a new epoch
	E8      Event = 8 // the node's non-circuit addresses changed
	EActive Event = 9 // the app became active again (H-17)
)

func (e Event) String() string {
	switch e {
	case E2:
		return "E2"
	case E3:
		return "E3"
	case E4:
		return "E4"
	case E7:
		return "E7"
	case E8:
		return "E8"
	case EActive:
		return "active"
	}
	return "unknown"
}

// Decision is what the transport does after a step or a result.
type Decision struct {
	// Action, if not None, is the call to start now, once the peer holds a node-wide slot; the
	// transport then calls [Policy.Start].
	Action Action
	// Epoch is the epoch the call belongs to.
	Epoch uint64
	// FromLadder: the call is due to a retry ladder, not to an event (H-17).
	FromLadder bool
	// Wake, if not zero, is when to step again without an event.
	Wake time.Time
	// FreeSlot: release the slot the call held.
	FreeSlot bool
	// Nudge: the call succeeded but the session stayed relayed: move it to the direct connection (H-13).
	Nudge bool
	// Cancel: cancel the call in flight (a new epoch, unwanted, direct, closed).
	Cancel bool
}

// Policy decides, for one wanted peer, when to call DCUtR again (plan §3.2). It is not safe for
// concurrent use; the transport steps it from one goroutine per peer.
type Policy struct {
	jitter   func(time.Duration) time.Duration
	attempts Bucket
	// epoch counts the circuits to the peer (H-6); call is the epoch of the call in flight.
	epoch, call uint64
	// fast and slow are the steps of the two ladders (H-8).
	fast, slow int
	// due is when the next call may start; fromLadder says whether a ladder set it.
	due        time.Time
	fromLadder bool
	lastStart  time.Time
	inFlight   bool
	// stopped: the call in flight was cancelled by a stop (unwanted, direct, no relay, disabled); its result
	// only frees the slot, even if the policy acts again before it returns (H-14).
	stopped bool
	// active: the policy acted at the last step (wanted, enabled, relayed).
	active bool
	// settle, if not zero, is when a call that succeeded must have moved the session (H-13).
	settle time.Time
	// closed: the hole-punch service is closing; nothing starts any more.
	closed bool
}

// NewPolicy returns the policy of a peer first seen at now. jitter spreads the ladders' waits; nil is
// [Jitter].
func NewPolicy(now time.Time, jitter func(time.Duration) time.Duration) *Policy {
	if jitter == nil {
		jitter = Jitter
	}
	return &Policy{jitter: jitter, attempts: NewBucket(AttemptBurst, AttemptRefill, now), epoch: 1}
}

// Step decides what to do at now, given o: start a call, wake later, or wait for an event.
func (p *Policy) Step(o Obs, now time.Time) Decision {
	if !o.Wanted || !o.Relayed || !o.Enabled {
		p.active, p.settle, p.stopped = false, time.Time{}, p.inFlight
		return Decision{Cancel: p.inFlight}
	}
	if !p.active { // the session is on a circuit again (E1, or a fallback from direct): budgets kept
		p.active = true
		p.fast, p.slow = 0, 0
		p.due, p.fromLadder = p.soonest(now), false
	}
	if p.closed {
		return Decision{}
	}
	if p.inFlight {
		return Decision{Cancel: p.call != p.epoch}
	}
	if !p.settle.IsZero() {
		if now.Before(p.settle) {
			return Decision{Wake: p.settle}
		}
		p.settle = time.Time{}
		p.unspent(now) // classed Unspent (H-13)
		return Decision{Nudge: true, Wake: p.due}
	}
	if !o.LocalReady {
		return Decision{}
	}
	act := Role(o)
	if act == None {
		return Decision{}
	}
	if now.Before(p.due) {
		return Decision{Wake: p.due}
	}
	d := Decision{Action: act, Epoch: p.epoch, FromLadder: p.fromLadder}
	if o.Idle && d.FromLadder {
		return Decision{}
	}
	if next := p.attempts.Next(now); next.After(now) {
		return Decision{Wake: next}
	}
	return d
}

// Start records that the call [Policy.Step] decided starts at now, and returns its epoch. It takes an
// attempt token, given back if the call turns out Busy.
func (p *Policy) Start(now time.Time) uint64 {
	p.attempts.Take(now)
	p.inFlight, p.call, p.lastStart = true, p.epoch, now
	return p.epoch
}

// Result applies the class of the call of epoch that returned at now.
func (p *Policy) Result(epoch uint64, c Class, now time.Time) Decision {
	stopped := p.stopped
	p.inFlight, p.stopped = false, false
	if epoch != p.epoch {
		return Decision{FreeSlot: true}
	}
	if stopped || p.closed {
		return Decision{FreeSlot: true}
	}
	switch c {
	case Success:
		p.fast, p.slow = 0, 0
		p.settle = now.Add(TSettle)
		return Decision{FreeSlot: true, Wake: p.settle}
	case Busy:
		p.attempts.Give()
		p.due, p.fromLadder = now.Add(p.jitter(TBusy)), true
	case Spent:
		p.due, p.fromLadder = now.Add(p.jitter(min(SlowBase<<p.slow, SlowCap))), true
		p.slow = min(p.slow+1, slowSteps)
	case Unspent:
		p.unspent(now)
	case Closed:
		p.closed = true
		return Decision{FreeSlot: true}
	}
	return Decision{FreeSlot: true, Wake: p.due}
}

// unspent schedules the next call on the fast ladder.
func (p *Policy) unspent(now time.Time) {
	p.due, p.fromLadder = now.Add(p.jitter(min(FastBase<<p.fast, FastCap))), true
	p.fast = min(p.fast+1, fastSteps)
}

// Event applies e, seen at now. No event refills a budget (H-9).
func (p *Policy) Event(e Event, now time.Time) {
	switch e {
	case E7, E8:
		p.slow = 0
		if e == E7 {
			p.newEpoch(now)
			return
		}
		p.advance(now)
	case E2, E3, E4:
		p.advance(now)
	}
}

// newEpoch starts a new epoch: the peer restarted or moved (H-6). The call in flight, if any, is
// cancelled by the next step and its result discarded.
func (p *Policy) newEpoch(now time.Time) {
	p.epoch++
	p.fast = 0
	p.settle = time.Time{}
	p.advance(now)
}

// advance brings the due time forward to the soonest a call may start, never later (H-9); the call is
// then due to an event, so it runs even while the app is idle (H-17).
func (p *Policy) advance(now time.Time) {
	if s := p.soonest(now); p.due.IsZero() || s.Before(p.due) || p.due.Before(p.lastStart.Add(TMin)) {
		p.due = s
	}
	p.fromLadder = false
}

// soonest is max(now, lastStart + T_min).
func (p *Policy) soonest(now time.Time) time.Time {
	if s := p.lastStart.Add(TMin); !p.lastStart.IsZero() && s.After(now) {
		return s
	}
	return now
}

// Epoch returns the current epoch.
func (p *Policy) Epoch() uint64 { return p.epoch }

// InFlight reports whether a call has started and not returned, a cancelled one included (H-4).
func (p *Policy) InFlight() bool { return p.inFlight }

// Due returns when the next call may start.
func (p *Policy) Due() time.Time { return p.due }

// Rested reports whether the policy carries nothing a new one would not: no call in flight and a full
// attempt budget. The transport keeps a peer's policy across want, un-want and reconnections (H-11)
// and may drop it only then.
func (p *Policy) Rested(now time.Time) bool { return !p.inFlight && p.attempts.Full(now) }
