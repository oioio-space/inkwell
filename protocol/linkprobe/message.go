// SPDX-License-Identifier: MPL-2.0

// Package linkprobe is protocol /inkwell/linkprobe/1.0.0 (spec §8.5, ADR 0036): the receiver
// of a QUIC connection tells the sender how many of its packets arrived, so the sender knows
// the share the network lost, without counting spurious loss declarations or ACK-only packets
// the way a sender's own loss detection does. The package is pure: messages, the prober and
// responder state machines, and the bounds of §8.5.7. Time is an argument; nothing here
// reads a clock, a socket or a counter.
package linkprobe

import (
	"encoding/binary"
	"errors"
	"fmt"

	transport "github.com/oioio-space/inkwell/protocol"
)

// ProtocolID is the libp2p protocol identifier, negotiated by multistream-select (§8.5.9).
const ProtocolID = "/inkwell/linkprobe/1.0.0"

// Message types (§8.5.4, registry §20.8).
const (
	TypeProbe uint8 = 0x01 // prober to responder: seq u32
	TypeEcho  uint8 = 0x02 // responder to prober: seq u32, received u64
)

// Lengths (§8.5.4): len counts the type byte and the body.
const (
	MaxLen   = transport.LinkProbeMaxLen // largest len of any message
	ProbeLen = 5
	EchoLen  = 13
)

var (
	// ErrMalformed is a message the spec refuses: the stream is reset (§8.5.8).
	ErrMalformed = errors.New("linkprobe: malformed message")
	// ErrShort means the bytes hold the start of a message only: read more (at most MaxLen + 1).
	ErrShort = errors.New("linkprobe: incomplete message")
)

// Message is one decoded message. Seq and Received are set by their types; Body holds the body
// of an ignorable type (0x40-0x7F), which the receiver skips; it aliases the decoded bytes.
type Message struct {
	Type     uint8
	Seq      uint32
	Received uint64
	Body     []byte
}

// Ignorable reports whether the type is in the ignorable range 0x40-0x7F (§8.5.4).
func (m Message) Ignorable() bool { return ignorable(m.Type) }

func ignorable(t uint8) bool { return t >= 0x40 && t <= 0x7f }

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrMalformed}, args...)...)
}

// Decode reads the message at the head of b and returns it with the number of bytes it took.
// It refuses a length out of 1..MaxLen as soon as it reads the length byte, a type of the
// critical range (0x01-0x3F) other than PROBE and ECHO, a reserved type (0x00, 0x80-0xFF), a
// known type whose length is not its fixed one, and a seq of 0. Received is not bounded here:
// the prober's rule of §8.5.5 judges it.
func Decode(b []byte) (Message, int, error) {
	if len(b) == 0 {
		return Message{}, 0, ErrShort
	}
	n := int(b[0])
	if n == 0 || n > MaxLen {
		return Message{}, 0, malformed("len %d", n)
	}
	if len(b) < 1+n {
		return Message{}, 0, ErrShort
	}
	t, body := b[1], b[2:1+n]
	m := Message{Type: t}
	switch {
	case t == TypeProbe:
		if n != ProbeLen {
			return Message{}, 0, malformed("PROBE of len %d", n)
		}
		m.Seq = binary.BigEndian.Uint32(body)
	case t == TypeEcho:
		if n != EchoLen {
			return Message{}, 0, malformed("ECHO of len %d", n)
		}
		m.Seq = binary.BigEndian.Uint32(body)
		m.Received = binary.BigEndian.Uint64(body[4:])
	case ignorable(t):
		m.Body = body // a sub-slice of b: no copy (§8.5.4, review 2 F1)
		return m, 1 + n, nil
	default:
		return Message{}, 0, malformed("type 0x%02x", t)
	}
	if m.Seq == 0 {
		return Message{}, 0, malformed("seq 0")
	}
	return m, 1 + n, nil
}

// Encode writes m in its unique encoding; it refuses what Decode refuses.
func Encode(m Message) ([]byte, error) {
	switch {
	case m.Type == TypeProbe && m.Seq != 0:
		return binary.BigEndian.AppendUint32([]byte{ProbeLen, TypeProbe}, m.Seq), nil
	case m.Type == TypeEcho && m.Seq != 0:
		b := binary.BigEndian.AppendUint32([]byte{EchoLen, TypeEcho}, m.Seq)
		return binary.BigEndian.AppendUint64(b, m.Received), nil
	case ignorable(m.Type) && len(m.Body) < MaxLen:
		return append([]byte{byte(1 + len(m.Body)), m.Type}, m.Body...), nil // #nosec G115 -- len(Body) < MaxLen.
	default:
		return nil, malformed("cannot encode type 0x%02x seq %d body %d", m.Type, m.Seq, len(m.Body))
	}
}
