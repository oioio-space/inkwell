// SPDX-License-Identifier: MPL-2.0

package syncack

import (
	"fmt"
	"slices"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// The bounds of the inbound order and of the refusal for capacity (spec §8.7.3, §8.7.4, §16.1, block PU1b).
const (
	// Places is how many inbound sync streams of one peer hold a place in its order (rule P-8).
	Places = transport.SyncInboundPlaces
	// BusyCode is the stream error code of a refusal for capacity: go-libp2p's StreamResourceLimitExceeded, which
	// its resource manager uses for its own per-peer stream limits.
	BusyCode uint32 = transport.SyncBusyCode
	// RetryMin is the opener's first wait after a refusal for capacity, doubled at each one, up to RetryMax.
	RetryMin = transport.SyncBusyRetryMin
	// RetryMax caps that wait.
	RetryMax = transport.SyncBusyRetryMax
	// ReorderWait bounds how long a stream waits for its turn before it is read anyway (rule P-9).
	ReorderWait = transport.SyncReorderWait
)

// Order is the order of one peer's inbound sync streams (rules P-8, P-9): the streams holding a place, in arrival
// order. A stream takes its place on arrival, before its ACK; its turn comes when no stream ahead of it holds a
// place. A stream leaves when it ends — at once, wherever it stands, if it delivered nothing; once read to its end
// if it did — and nothing waits on behalf of a stream that left. Ids are the caller's, unique per order. The zero
// value is an empty order waiting ReorderWait. It is not safe for concurrent use.
type Order struct {
	// Wait, if not zero, replaces ReorderWait (an implementation's own bound, kept equal to it).
	Wait time.Duration
	list []place
}

func (o *Order) wait() time.Duration {
	if o.Wait != 0 {
		return o.Wait
	}
	return ReorderWait
}

type place struct {
	id uint64
	at time.Duration // arrival, on the caller's clock
}

// Arrive gives stream id a place, last, at time at; false means the peer holds every place: the stream is refused
// for capacity (reset with BusyCode).
func (o *Order) Arrive(id uint64, at time.Duration) bool {
	if len(o.list) >= Places {
		return false
	}
	o.list = append(o.list, place{id: id, at: at})
	return true
}

// Ready reports whether stream id may deliver its frames at time at: its turn has come (no stream ahead of it
// holds a place), or it has waited ReorderWait for it (the order is then given up).
func (o *Order) Ready(id uint64, at time.Duration) bool {
	switch i := o.index(id); {
	case i < 0:
		return false
	case i == 0:
		return true
	default:
		return at-o.list[i].at >= o.wait()
	}
}

// WaitLeft is how long stream id still waits, at time at, before it is read anyway; 0 once ready or unknown.
func (o *Order) WaitLeft(id uint64, at time.Duration) time.Duration {
	i := o.index(id)
	if i <= 0 {
		return 0
	}
	return max(0, o.list[i].at+o.wait()-at)
}

// Leave removes stream id from the order, wherever it stands, and returns the streams whose turn came with it
// (the successors to wake). An id that holds no place changes nothing.
func (o *Order) Leave(id uint64) (woken []uint64) {
	i := o.index(id)
	if i < 0 {
		return nil
	}
	o.list = slices.Delete(o.list, i, i+1)
	if i == 0 && len(o.list) > 0 {
		return []uint64{o.list[0].id}
	}
	return nil
}

// Holds reports whether stream id holds a place.
func (o *Order) Holds(id uint64) bool { return o.index(id) >= 0 }

// Len is how many places are held.
func (o *Order) Len() int { return len(o.list) }

func (o *Order) index(id uint64) int {
	return slices.IndexFunc(o.list, func(p place) bool { return p.id == id })
}

// Action is what the opener does on a refusal before the ACK (spec §8.7.4).
type Action uint8

const (
	// Retry is to open again after the wait OnRefusal returns; the session and the connections are kept.
	Retry Action = iota
	// ClosePeerAction is to reset the stream and close the connections to the peer (a session opening).
	ClosePeerAction
	// AbandonAction is to reset the stream; the session stays where it is (a move).
	AbandonAction
)

var actionNames = [...]string{"retry", "close_peer", "abandon"}

// String returns the action's name in the vectors.
func (a Action) String() string {
	if int(a) < len(actionNames) {
		return actionNames[a]
	}
	return fmt.Sprintf("action(%d)", a)
}

// OnRefusal is the opener's decision on a refusal before the ACK of an opening of role (spec §8.7.4, rule P-8):
// capacity tells whether the reset carried BusyCode, elapsed is the time since the opening's first open, attempt the
// refusals for capacity before this one. A refusal for capacity is retried after RetryMin doubled per attempt, at
// most RetryMax, if the retry still falls within Budget; otherwise, and on any other refusal, a session opening
// closes the peer and a move is abandoned.
func OnRefusal(role Role, capacity bool, elapsed time.Duration, attempt int) (Action, time.Duration) {
	if capacity {
		wait := RetryMin
		for range attempt {
			wait = min(2*wait, RetryMax)
		}
		if elapsed+wait < Budget {
			return Retry, wait
		}
	}
	if role == RoleMove {
		return AbandonAction, 0
	}
	return ClosePeerAction, 0
}
