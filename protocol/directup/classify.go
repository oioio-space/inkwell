// SPDX-License-Identifier: MPL-2.0

package directup

import (
	"math/rand/v2"
	"time"
)

// Action is what a call does (H-3).
type Action uint8

// The actions.
const (
	None  Action = iota
	Punch        // DCUtR over an inbound circuit: DirectConnect
	Dial         // a direct dial to the peer's announced public addresses, over an outbound circuit
)

func (a Action) String() string {
	switch a {
	case Punch:
		return "punch"
	case Dial:
		return "dial"
	}
	return "none"
}

// Role returns the action the side of o may take (H-3, H-15): Punch only over an inbound circuit,
// Dial only with an outbound circuit alone and a public announced address, and nothing with two
// circuits: the duplicate is closed first, so that one circuit gives one initiator.
func Role(o Obs) Action {
	switch {
	case o.In && o.Out:
		return None
	case o.In:
		return Punch
	case o.Out && o.RemotePublic:
		return Dial
	}
	return None
}

// Outcome is how a call returned.
type Outcome uint8

// The outcomes.
const (
	OK        Outcome = iota // no error
	ErrBusy                  // another attempt to the peer was active (holepunch.ErrHolePunchActive)
	ErrClosed                // the hole-punch service is closing
	ErrOther                 // any other error, a canceled call included
)

// Class is the class of a finished call (H-7), decided by what was observed, never by error texts.
type Class uint8

// The classes.
const (
	Success Class = iota + 1 // a direct connection to the peer exists after the call
	Busy                     // another attempt was active: no token, ladders unchanged
	Spent                    // a round passed the address filter or a dial reached the network, in vain
	Unspent                  // nothing was sent to the network
	Closed                   // the service is closing: stop
)

func (c Class) String() string {
	switch c {
	case Success:
		return "success"
	case Busy:
		return "busy"
	case Spent:
		return "spent"
	case Unspent:
		return "unspent"
	case Closed:
		return "closed"
	}
	return "unknown"
}

// Classify classes a call from what it observed: rounds is the number of rounds with the peer that
// passed the address filter while the call ran, dialled whether a non-relay dial to it passed the gater,
// direct whether a direct connection to it exists once the call returned.
func Classify(rounds int, dialled, direct bool, out Outcome) Class {
	switch {
	case out == ErrClosed:
		return Closed
	case direct:
		return Success
	case out == ErrBusy:
		return Busy
	case rounds > 0 || dialled:
		return Spent
	}
	return Unspent
}

// Jitter spreads d by up to a quarter either way, so that peers that lost a path together do not retry
// together. It is the jitter the transport gives [NewPolicy].
func Jitter(d time.Duration) time.Duration {
	if d < 4 {
		return d
	}
	return d - d/4 + rand.N(d/2) // #nosec G404 -- spreading retries; nothing secret is drawn here.
}
