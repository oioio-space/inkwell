// SPDX-License-Identifier: MPL-2.0

// Package syncack is the acceptance ACK of the existing sync session, protocol /inkwell/sync/2.0.0
// (spec §8.7, block PU1): the responder of a sync stream writes one byte, 0x01, once its own
// checks accept the stream, and the opener announces the peer only after reading it. The package is
// pure: the codec, the responder's decision (§8.7.3) and the opener's machine (§8.7.5). Time is an
// argument; nothing here reads a clock, opens a stream or touches a connection.
package syncack

import (
	"errors"
	"fmt"
	"io"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// ProtocolID is the libp2p protocol of the sync streams that carry the ACK (§8.7.8, registry §20.6).
const ProtocolID = "/inkwell/sync/2.0.0"

// Ack is the acceptance byte, the only byte a responder ever writes on a sync stream (§8.7.2).
const Ack byte = 0x01

// Budget is the single deadline of an opening (§8.7.4): opening the stream and reading its ACK
// share it, counted from the open.
const Budget = transport.SyncAckTimeout

var (
	// ErrRefused is a stream that ended, or was reset, before its first byte: the peer refused it.
	ErrRefused = errors.New("syncack: the stream ended before its acceptance")
	// ErrViolation is a first byte other than Ack: the peer broke the protocol.
	ErrViolation = errors.New("syncack: the first byte is not the acceptance")
	// ErrOrder is an opener event that cannot happen in the machine's state (§8.7.5).
	ErrOrder = errors.New("syncack: event out of order")
)

// ReadAck reads exactly one byte from r: nil if it is Ack, an error wrapping ErrViolation if it is
// another value, and one wrapping ErrRefused (and the cause) if r ends or fails first. It never
// reads past that byte; any later byte is the caller's to judge (a violation, §8.7.2).
func ReadAck(r io.Reader) error {
	var b [transport.SyncAckLen]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return fmt.Errorf("%w: %w", ErrRefused, err)
	}
	if b[0] != Ack {
		return fmt.Errorf("%w: 0x%02x", ErrViolation, b[0])
	}
	return nil
}

// WriteAck writes the ACK to w.
func WriteAck(w io.Writer) error {
	_, err := w.Write([]byte{Ack})
	return err
}

// Responder is what the responder knows when a sync stream arrives (§8.7.3), checked in this
// order: the node is closed; the peer is admitted (Admit accepts it and it is not held); the
// stream's connection is closed or closing.
type Responder struct {
	NodeClosed bool
	Admitted   bool
	ConnClosed bool
}

// Accept reports whether the responder writes the ACK; otherwise it resets the stream without
// writing anything (rule P-2).
func (r Responder) Accept() bool { return !r.NodeClosed && r.Admitted && !r.ConnClosed }

// Role is what an opening stream is for (§8.7.5).
type Role uint8

const (
	// RoleSession is the stream that carries the session: the peer's first stream, or one reopened.
	RoleSession Role = iota
	// RoleMove is a stream on a direct connection that is to replace a relayed session stream.
	RoleMove
)

// ParseRole reads the name of a role in the vectors: "session" or "move".
func ParseRole(s string) (Role, bool) {
	switch s {
	case "session":
		return RoleSession, true
	case "move":
		return RoleMove, true
	}
	return 0, false
}

// Op is an input of the opener's machine.
type Op uint8

// The inputs, named in the vectors as ParseOp reads them.
const (
	OpOpen       Op = iota // the stream starts opening
	OpReturn               // the opening returned a stream
	OpByte                 // a byte arrived on the stream
	OpEOF                  // the stream ended
	OpReset                // the stream was reset
	OpConnClosed           // the stream's connection is gone
	OpDeadline             // the budget ran out
)

var opNames = map[string]Op{
	"open": OpOpen, "return": OpReturn, "byte": OpByte, "eof": OpEOF, "reset": OpReset,
	"conn_closed": OpConnClosed, "deadline": OpDeadline,
}

// ParseOp reads the name of an input in the vectors.
func ParseOp(s string) (Op, bool) {
	op, ok := opNames[s]
	return op, ok
}

// Event is one input at a time since an origin common to the opening's events.
type Event struct {
	Op   Op
	At   time.Duration
	Byte byte // the byte of OpByte
}

// Outcome is what the opener does on an input.
type Outcome uint8

const (
	// None is nothing to do.
	None Outcome = iota
	// PeerUp announces the peer (and the stream carries the session).
	PeerUp
	// CloseConn resets the stream and closes the connection it runs on, and only it, with no event (spec
	// §8.7.4, rule P-16): the opening names that connection from its start, so the close always knows it.
	CloseConn
	// Lost is the connection's loss, handled as any other (PeerDown if announced).
	Lost
	// Swap replaces the session's stream by the move's, with no event.
	Swap
	// Abandon resets the move's stream; the session stays where it is, with no event.
	Abandon
)

var outcomeNames = [...]string{"none", "peer_up", "close_conn", "lost", "swap", "abandon"}

// String returns the outcome's name in the vectors.
func (o Outcome) String() string {
	if int(o) < len(outcomeNames) {
		return outcomeNames[o]
	}
	return fmt.Sprintf("outcome(%d)", o)
}

type state uint8

const (
	stNew     state = iota // not opened yet
	stOpening              // opening, no stream returned yet
	stWaiting              // stream returned, no ACK yet
	stOpen                 // ACK read
	stDone                 // ended
)

// Opener is the machine of one opening stream (§8.7.4, §8.7.5): it starts with OpOpen, and the
// opening must end, ACK read, before Budget has passed since then, whatever is in flight. It is the
// reference model the vectors are checked against; a node may follow it with its own control flow
// (inkwell's does, its arms proven by internal/transport/p2p's TestPeerUpAfterAck).
type Opener struct {
	role      Role
	announced bool
	st        state
	openAt    time.Duration
	last      time.Duration
}

// NewOpener returns the machine of an opening of role; announced tells whether the peer is
// already announced (a stream reopened, or a move).
func NewOpener(role Role, announced bool) *Opener {
	return &Opener{role: role, announced: announced}
}

// Step applies e. It returns ErrOrder for an event the machine cannot see in its state: anything
// before OpOpen or after the end, a byte before the return, a second return, time going back, a
// deadline before its time.
func (o *Opener) Step(e Event) (Outcome, error) {
	if o.st == stDone || (o.st == stNew) != (e.Op == OpOpen) || e.At < o.last {
		return None, o.order(e)
	}
	o.last = e.At
	if o.st == stNew {
		o.st, o.openAt = stOpening, e.At
		return None, nil
	}
	if o.st != stOpen && e.At >= o.openAt+Budget { // the deadline fired first
		return o.end(o.refused())
	}
	switch e.Op {
	case OpReturn:
		if o.st == stOpening {
			o.st = stWaiting
			return None, nil
		}
	case OpByte:
		switch {
		case o.st == stOpen: // the responder writes nothing after the ACK
			return o.end(CloseConn)
		case o.st == stWaiting && e.Byte == Ack:
			o.st = stOpen
			return o.accepted(), nil
		case o.st == stWaiting:
			return o.end(o.refused())
		}
	case OpEOF, OpReset:
		if o.st == stOpen { // the remote ended our stream on a live connection: it refuses us
			return o.end(CloseConn)
		}
		return o.end(o.refused())
	case OpConnClosed:
		if o.role == RoleMove && o.st != stOpen {
			return o.end(Abandon)
		}
		return o.end(Lost)
	case OpDeadline:
		if o.st == stOpen {
			return None, nil
		}
	}
	return None, o.order(e)
}

func (o *Opener) order(e Event) error {
	return fmt.Errorf("%w: %d at %v in state %d", ErrOrder, e.Op, e.At, o.st)
}

func (o *Opener) end(out Outcome) (Outcome, error) {
	o.st = stDone
	return out, nil
}

// refused is the outcome of an opening that fails: a refusal, a violation or the deadline (P-16: its
// connection is closed, not the peer).
func (o *Opener) refused() Outcome {
	if o.role == RoleMove {
		return Abandon
	}
	return CloseConn
}

// accepted is the outcome of the ACK.
func (o *Opener) accepted() Outcome {
	switch {
	case o.role == RoleMove:
		return Swap
	case o.announced:
		return None
	}
	return PeerUp
}
