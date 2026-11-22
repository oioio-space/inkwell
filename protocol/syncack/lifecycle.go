// SPDX-License-Identifier: MPL-2.0

package syncack

import (
	"fmt"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// The bounds of an inbound stream's lifecycle, on the sync and bulk lanes alike (spec §8.1, §8.7.3, §16.1, block
// PU1c, rules P-10 to P-14).
const (
	// ProbeEvery bounds the time between the starts of two probe reads of a waiting stream (rule P-11), and so the
	// time to see a reset, a turn or the peer's announcement.
	ProbeEvery = transport.InboundProbeEvery
	// ProbeMax is how many bytes a waiting stream may take by probing, in all (rule P-11).
	ProbeMax = transport.InboundProbeMax
	// UpWait bounds how long an accepted stream waits for its peer's announcement, counted from its ACK (rule P-10).
	UpWait = 30 * time.Second
)

// InState is the state of an inbound stream (rule P-10).
type InState uint8

const (
	// Arrived is a stream that has not asked for its place yet.
	Arrived InState = iota
	// Refused is a stream that found no place: it is reset with BusyCode, no byte read, nothing kept.
	Refused
	// Accepting is a placed stream whose ACK is being written.
	Accepting
	// Waiting is an acknowledged stream waiting for its turn and its peer's announcement, probing.
	Waiting
	// Kept is a waiting stream that ended after a non-empty probed prefix: it keeps its place to its turn.
	Kept
	// Delivering is a stream whose frames are read and handed to the consumer.
	Delivering
	// Blocked is a delivering stream waiting for room in its lane's receive budget.
	Blocked
	// Gone is a stream that left its place (or never had one).
	Gone
)

var inStateNames = [...]string{"arrived", "refused", "accepting", "waiting", "kept", "delivering", "blocked", "gone"}

// String returns the state's name in the vectors.
func (s InState) String() string {
	if int(s) < len(inStateNames) {
		return inStateNames[s]
	}
	return fmt.Sprintf("state(%d)", s)
}

// ReadEnd is how a read of an inbound stream ended, besides the bytes it returned.
type ReadEnd uint8

const (
	// ReadOK is a read that returned with no error.
	ReadOK ReadEnd = iota
	// ReadDeadline is the reader's own deadline: never an end, the reader re-evaluates (rule P-11).
	ReadDeadline
	// ReadEOF is the opener's end of writing, with nothing unread behind it.
	ReadEOF
	// ReadReset is a reset of the stream.
	ReadReset
	// ReadConnLost is the loss of the stream's connection.
	ReadConnLost
	// ReadNodeClosed is the local node's close.
	ReadNodeClosed
)

var readEndNames = map[string]ReadEnd{
	"none": ReadOK, "deadline": ReadDeadline, "eof": ReadEOF, "reset": ReadReset, "conn_lost": ReadConnLost,
	"node_closed": ReadNodeClosed,
}

// ParseReadEnd returns the ReadEnd named s in the vectors.
func ParseReadEnd(s string) (ReadEnd, bool) {
	e, ok := readEndNames[s]
	return e, ok
}

// Lifecycle is the state machine of one inbound stream of an ordered lane (spec §8.7.3, rules P-10 to P-12): the
// reader's view, driven by what it observes. Each method returns whether the stream gave its place back at that
// observation. Times are on the caller's clock. The zero value is an Arrived stream. It is not safe for concurrent
// use.
//
// The turn (no stream ahead in the lane's order, or the order's wait spent, rule P-9) is the caller's to tell, from
// Order; so is the peer's announcement. The stream delivers once both came. Its budget is taken only from then
// (rule P-12): nothing here before Delivering touches it.
type Lifecycle struct {
	state    InState
	prefix   int           // probed bytes held
	ackAt    time.Duration // when the ACK was written
	up, turn bool
	ended    bool // kept: the stream ended behind its probed prefix
}

// State is the stream's state.
func (l *Lifecycle) State() InState { return l.state }

// Prefix is how many probed bytes the stream holds, given to the decoder first at its turn.
func (l *Lifecycle) Prefix() int { return l.prefix }

// Arrive is the stream's arrival: placed tells whether it got a place in its lane's order (rule P-8).
func (l *Lifecycle) Arrive(placed bool) {
	if l.state != Arrived {
		return
	}
	l.state = Refused
	if placed {
		l.state = Accepting
	}
}

// Ack is the end of the ACK's write at time at: ok if it was written. A stream whose ACK fails leaves at once.
func (l *Lifecycle) Ack(ok bool, at time.Duration) (leave bool) {
	if l.state != Accepting {
		return false
	}
	if !ok {
		return l.leave()
	}
	l.state, l.ackAt = Waiting, at
	l.advance()
	return false
}

// Up is the peer's announcement.
func (l *Lifecycle) Up() {
	l.up = true
	l.advance()
}

// Turn is the stream's turn in its lane's order.
func (l *Lifecycle) Turn() {
	l.turn = true
	l.advance()
}

// Read is a read that returned n bytes and ended with end, at time at. While waiting it is a probe: the n bytes
// are kept, in order, before end is looked at (rule P-11); a deadline is never an end, only the moment to check
// the wait for the peer's announcement; an end of writing keeps the stream to its turn when it holds bytes and lets
// it leave at once when it holds none; a reset, a lost connection or the node's close end it. Delivering, any end
// ends it (the decoder has the bytes).
func (l *Lifecycle) Read(n int, end ReadEnd, at time.Duration) (leave bool) {
	switch l.state {
	case Waiting:
		l.prefix += n
		switch end {
		case ReadOK:
			return false
		case ReadDeadline:
			return l.Tick(at)
		case ReadEOF:
			if l.prefix == 0 {
				return l.leave()
			}
			l.state, l.ended = Kept, true
			return false
		}
		return l.leave()
	case Kept:
		return l.Tick(at)
	case Delivering:
		if end == ReadOK || end == ReadDeadline {
			return false
		}
		return l.leave()
	}
	return false
}

// Tick re-evaluates a waiting or kept stream at time at: one whose peer was not announced within UpWait of its ACK
// leaves.
func (l *Lifecycle) Tick(at time.Duration) (leave bool) {
	if (l.state == Waiting || l.state == Kept) && !l.up && at-l.ackAt >= UpWait {
		return l.leave()
	}
	return false
}

// Budget is the answer of the lane's receive budget to a delivering stream's reservation: room, or none yet (the
// stream is then blocked until room is freed, its own connection is lost or the node closes).
func (l *Lifecycle) Budget(room bool) {
	switch {
	case l.state == Delivering && !room:
		l.state = Blocked
	case l.state == Blocked && room:
		l.state = Delivering
	}
}

// ConnLost is the loss of a connection of the stream's peer: own tells whether it is the stream's own. Only its
// own ends the stream, wherever it stands (a blocked reservation is cancelled, rule P-12).
func (l *Lifecycle) ConnLost(own bool) (leave bool) {
	if !own || l.state == Arrived || l.state == Refused || l.state == Gone {
		return false
	}
	return l.leave()
}

// Decoded is the decoder's report on a delivering stream once its probed prefix is decoded: the whole frames are
// delivered and a truncated last frame dropped. A stream that was kept (it ended behind its prefix) then leaves; any
// other goes on reading.
func (l *Lifecycle) Decoded() (leave bool) {
	if l.state != Delivering || !l.ended {
		return false
	}
	return l.leave()
}

// advance moves a waiting or kept stream to Delivering once both its turn and its peer's announcement came.
func (l *Lifecycle) advance() {
	if (l.state == Waiting || l.state == Kept) && l.up && l.turn {
		l.state = Delivering
	}
}

func (l *Lifecycle) leave() bool {
	l.state, l.prefix = Gone, 0
	return true
}

// ProbeSchedule checks a waiting stream's probe (rule P-11): attempts start ProbeEvery apart, each attempt window
// holds at most one byte, and at most ProbeMax bytes are kept. Attempt times and byte times are each in order on
// the caller's clock. The zero value has seen nothing.
type ProbeSchedule struct {
	start          time.Duration
	attempts       int
	bytes          int
	lastByteWindow int
}

// Attempt records a probe read starting at at. The first attempt sets the grid origin.
func (p *ProbeSchedule) Attempt(at time.Duration) error {
	if p.attempts == 0 {
		p.start = at
	}
	want := p.start + time.Duration(p.attempts)*ProbeEvery
	if at != want {
		return fmt.Errorf("syncack: probe attempt at %v, want %v", at, want)
	}
	p.attempts++
	return nil
}

// Byte records a byte consumed in a started attempt's window, including its start and excluding the next start.
func (p *ProbeSchedule) Byte(at time.Duration) error {
	offset := at - p.start
	window := int(offset / ProbeEvery)
	if offset < 0 || window >= p.attempts {
		return fmt.Errorf("syncack: probed byte at %v outside a started attempt window", at)
	}
	if p.bytes > 0 && window <= p.lastByteWindow {
		return fmt.Errorf("syncack: probed byte at %v repeats or precedes consumed window %d", at, p.lastByteWindow)
	}
	if p.bytes == ProbeMax {
		return fmt.Errorf("syncack: %d probed bytes kept, more than %d", p.bytes+1, ProbeMax)
	}
	p.bytes++
	p.lastByteWindow = window
	return nil
}

// QUICWindowOK is rule P-13 (§8.7.3, §16.1): (places × lanes + 1) × stream ≤ (1 − 1/den) × conn, so that the streams
// a node leaves unread on one QUIC connection, each holding at most its initial stream window, never take the
// credit a head needs before the connection window is updated (quic-go updates it once 1/den of it was read).
func QUICWindowOK(places, lanes, stream, conn, den uint64) bool {
	return den > 0 && (places*lanes+1)*stream*den <= (den-1)*conn
}
