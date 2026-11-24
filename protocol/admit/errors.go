// SPDX-License-Identifier: MPL-2.0

package admit

import "errors"

// Reason is why an opening or a handshake is refused.
type Reason uint8

// The reasons, with the rule each enforces.
const (
	ReasonOrigin  Reason = iota + 1 // RF-3: the origin is at its pending bound
	ReasonWide                      // RF-3: the IPv6 /48 is at its pending bound
	ReasonPending                   // RF-3: the node is at its pending bound
	ReasonRate                      // RF-5: the origin's bucket is empty
	ReasonMembers                   // RF-6: the member has no room left
	ReasonRelays                    // RF-6: the relays have no room left
)

// String is the reason's name, as the vectors write it.
func (r Reason) String() string {
	switch r {
	case ReasonOrigin:
		return "origin"
	case ReasonWide:
		return "wide"
	case ReasonPending:
		return "pending"
	case ReasonRate:
		return "rate"
	case ReasonMembers:
		return "members"
	case ReasonRelays:
		return "relays"
	}
	return "none"
}

// ErrRefused is matched by every [RefusedError].
var ErrRefused = errors.New("admit: refused")

// RefusedError is a refusal, with its reason.
type RefusedError struct{ Reason Reason }

func (e *RefusedError) Error() string { return "admit: refused (" + e.Reason.String() + ")" }

// Is makes errors.Is(err, ErrRefused) hold.
func (e *RefusedError) Is(target error) bool { return target == ErrRefused }

// ReasonOf returns the reason of a refusal, or 0 if err is not one.
func ReasonOf(err error) Reason {
	if e, ok := errors.AsType[*RefusedError](err); ok {
		return e.Reason
	}
	return 0
}
