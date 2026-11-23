// SPDX-License-Identifier: MPL-2.0

// Package directup is the policy that keeps a relayed link to a wanted peer trying to become direct
// (spec §8.6 « Montée en direct », ADR 0045): when to call DCUtR (/libp2p/dcutr) again, with which role,
// and the budgets that bound what a peer can make this node punch or dial, whoever initiates.
//
// It is pure: no I/O, no goroutine, no clock of its own. The transport (internal/transport/p2p) observes
// libp2p, passes the time in, and does what a [Decision] says. The parts:
//
//   - [Policy], one per wanted peer: its epoch, its two retry ladders, its attempt bucket and its call in
//     flight (rules H-1 to H-4, H-6 to H-11, H-13, H-14, H-17);
//   - [Slots], one per node: the node-wide punch slots, granted in FIFO order with a lease, and the
//     node-wide admission bucket (H-5, H-16);
//   - [Budgets], one per peer at the two choke points, the hole-punch address filter (rounds) and the
//     connection gater (destinations) (H-12);
//   - [Classify] and [Select], the outcome of a call and the addresses a round may use.
package directup

import "time"

// Timers (plan §3.3).
const (
	// TMin is the least spacing of two calls to a peer; an event never calls sooner.
	TMin = time.Second
	// TBusy is the wait after a call that found another attempt to the peer active (libp2p's own).
	TBusy = 2 * time.Second
	// FastBase and FastCap bound the fast ladder, after a call that sent nothing to the network:
	// 1 s · 2^j, up to 30 s (quic-go's idle timeout: a stale connection is gone by then).
	FastBase = time.Second
	FastCap  = 30 * time.Second
	// SlowBase and SlowCap bound the slow ladder, after a call that punched or dialled in vain:
	// 60 s · 2^k, up to 16 min, for ever while the peer is wanted and relayed.
	SlowBase = time.Minute
	SlowCap  = 16 * time.Minute
	// TSettle is how long a call that succeeded may leave the session relayed before the transport is
	// nudged to move it (go-libp2p #3420).
	TSettle = 15 * time.Second
	// ActiveWindow is how long after the last input the app counts as active (H-17).
	ActiveWindow = 5 * time.Minute
)

// The ladders' steps saturate here: 2^5 is past both caps, so the caps decide, and the shift never
// overflows.
const (
	fastSteps = 5
	slowSteps = 5
)

// Budgets (plan §3.3). Each is a token bucket: burst tokens, one more per refill period.
const (
	// AttemptBurst and AttemptRefill: ≤ 4 + 8 = 12 non-Busy calls per peer in any hour.
	AttemptBurst  = 4
	AttemptRefill = 450 * time.Second
	// RoundBurst and RoundRefill: ≤ 9 + 36 = 45 hole-punch rounds per peer in any hour, both roles.
	RoundBurst  = 9
	RoundRefill = 100 * time.Second
	// DestBurst and DestRefill: ≤ 30 + 144 = 174 non-relay destinations per peer in any hour.
	DestBurst  = 30
	DestRefill = 25 * time.Second
	// NodeBurst and NodeRefill: ≤ 8 + 30 = 38 slot grants per node in any hour, all peers together.
	NodeBurst  = 8
	NodeRefill = 120 * time.Second
	// GrantAllowance is how many non-relay destinations one grant allows its peer, all origins together:
	// 3 for DirectConnect's first direct dial and 3 rounds of 3 addresses.
	GrantAllowance = 12
)

// Slots (plan §3.3, H-5).
const (
	// MaxSlots is the number of node-wide punch slots.
	MaxSlots = 4
	// Lease bounds a grant: libp2p's own deadlines (StreamTimeout 60 s, the direct dial's 10 s) end the
	// holder's work first.
	Lease = 75 * time.Second
	// RoundWait is how long a round waits for a slot before it gives up (its ticket keeps its place).
	RoundWait = 20 * time.Second
	// TicketIdle is how long a ticket is kept without any round of its peer.
	TicketIdle = 20 * time.Minute
)

// MaxAddrs is how many addresses a round may use, of the ones a CONNECT carries or a peer announced.
const MaxAddrs = 3
