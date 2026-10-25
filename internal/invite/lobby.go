package invite

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/trust"
)

// Limits of the lobby protocol, which runs one request and one reply on a
// stream (see p2p.Node.HandleLobby).
const (
	// MaxRequest bounds an encoded [Request]; every request is far smaller.
	MaxRequest = 4 << 10
	// MaxReply bounds an encoded [Reply]. A Welcome carries the whole member
	// list, so it is larger than a request; a list whose Welcome would not
	// fit cannot be joined by invitation (the host answers [ReasonTooLarge]).
	MaxReply = 256 << 10

	// maxFieldLen bounds the strings of a message (a one-byte length).
	maxFieldLen = 255
	// maxWelcomeOps bounds the ops of a Welcome: a member list holds at most
	// 1024 ops.
	maxWelcomeOps = 1024
)

// RequestKind says what a [Request] asks for.
type RequestKind byte

// The requests a stranger can make of a host.
const (
	// KindHello asks who the host is. Served only while it is visible.
	KindHello RequestKind = iota + 1
	// KindAsk asks the host's people to let the stranger join. Served only
	// while the host is visible.
	KindAsk
	// KindRedeem presents the secret of a ticket.
	KindRedeem
)

// Request is a message from a would-be member to a host.
type Request struct {
	// Name is how the stranger wants to be called; Ask and Redeem.
	Name string
	// Project and Secret identify the ticket; Redeem.
	Project trust.ProjectID
	Secret  [16]byte
	// Kind says what is asked.
	Kind RequestKind
}

// Encode returns the wire form of r.
func (r Request) Encode() []byte {
	b := []byte{byte(r.Kind)}
	switch r.Kind {
	case KindAsk:
		b = appendStr(b, r.Name)
	case KindRedeem:
		b = append(b, r.Project[:]...)
		b = append(b, r.Secret[:]...)
		b = appendStr(b, r.Name)
	}
	return b
}

// DecodeRequest parses a request received from another machine. It is strict:
// the whole input must be consumed.
func DecodeRequest(b []byte) (Request, error) {
	if len(b) > MaxRequest {
		return Request{}, fmt.Errorf("%w: request too long", ErrMalformed)
	}
	r := &reader{b: b}
	req := Request{Kind: RequestKind(r.u8())}
	switch req.Kind {
	case KindHello:
	case KindAsk:
		req.Name = r.str()
	case KindRedeem:
		copy(req.Project[:], r.take(len(req.Project)))
		copy(req.Secret[:], r.take(len(req.Secret)))
		req.Name = r.str()
	default:
		return Request{}, fmt.Errorf("%w: request kind %d", ErrMalformed, req.Kind)
	}
	if err := r.done(); err != nil {
		return Request{}, err
	}
	return req, nil
}

// ReplyKind says what a [Reply] is.
type ReplyKind byte

// The replies of a host.
const (
	// ReplyHello tells who the host is.
	ReplyHello ReplyKind = iota + 1
	// ReplyWelcome admits the requester and gives it the project.
	ReplyWelcome
	// ReplyRefused says no, for one [Reason].
	ReplyRefused
)

// Reason tells why a request was refused. It is deliberately coarse: a ticket
// that is wrong, used, expired or for another project is always [ReasonInvalid].
type Reason byte

// The reasons of a [ReplyRefused].
const (
	// ReasonInvalid: the ticket cannot be used.
	ReasonInvalid Reason = iota + 1
	// ReasonDeclined: a person of the project said no, or did not answer.
	ReasonDeclined
	// ReasonBusy: too many requests are waiting.
	ReasonBusy
	// ReasonFailed: the host could not admit the requester.
	ReasonFailed
	// ReasonTooLarge: the member list is too large to send.
	ReasonTooLarge
)

// Reply is a message from a host to a would-be member.
type Reply struct {
	// Machine and Title describe the host; Hello. They come from another
	// machine: show them through identity.CleanName.
	Machine string
	Title   string
	// RootURL and Ops are the project; Welcome. Ops are in application order.
	RootURL string
	Ops     []trust.Op
	// Kind says what the reply is.
	Kind ReplyKind
	// Reason is set for Refused.
	Reason Reason
}

// Encode returns the wire form of r.
func (r Reply) Encode() []byte {
	b := []byte{byte(r.Kind)}
	switch r.Kind {
	case ReplyHello:
		b = appendStr(b, r.Machine)
		b = appendStr(b, r.Title)
	case ReplyWelcome:
		url := r.RootURL[:min(len(r.RootURL), trust.MaxRootURL)]
		b = binary.BigEndian.AppendUint16(b, uint16(len(url))) // #nosec G115 -- bounded by MaxRootURL.
		b = append(b, url...)
		b = binary.BigEndian.AppendUint16(b, uint16(len(r.Ops))) // #nosec G115 -- a list holds at most maxWelcomeOps ops.
		for _, op := range r.Ops {
			enc := trust.Encode(op)
			b = binary.BigEndian.AppendUint32(b, uint32(len(enc))) // #nosec G115 -- an op is at most MaxOpSize.
			b = append(b, enc...)
		}
	case ReplyRefused:
		b = append(b, byte(r.Reason))
	}
	return b
}

// DecodeReply parses a reply received from another machine. It is strict:
// every op must decode canonically and the whole input must be consumed.
func DecodeReply(b []byte) (Reply, error) {
	if len(b) > MaxReply {
		return Reply{}, fmt.Errorf("%w: reply too long", ErrMalformed)
	}
	r := &reader{b: b}
	rep := Reply{Kind: ReplyKind(r.u8())}
	switch rep.Kind {
	case ReplyHello:
		rep.Machine = r.str()
		rep.Title = r.str()
	case ReplyWelcome:
		url := r.take(r.u16())
		if len(url) > trust.MaxRootURL || !utf8.Valid(url) {
			return Reply{}, fmt.Errorf("%w: root url", ErrMalformed)
		}
		rep.RootURL = string(url)
		n := r.u16()
		if n > maxWelcomeOps {
			return Reply{}, fmt.Errorf("%w: %d ops", ErrMalformed, n)
		}
		for range n {
			enc := r.take(r.u32())
			if r.err != nil {
				break
			}
			op, err := trust.Decode(enc)
			if err != nil {
				return Reply{}, fmt.Errorf("%w: op: %w", ErrMalformed, err)
			}
			rep.Ops = append(rep.Ops, op)
		}
	case ReplyRefused:
		rep.Reason = Reason(r.u8())
		if rep.Reason < ReasonInvalid || rep.Reason > ReasonTooLarge {
			return Reply{}, fmt.Errorf("%w: reason %d", ErrMalformed, rep.Reason)
		}
	default:
		return Reply{}, fmt.Errorf("%w: reply kind %d", ErrMalformed, rep.Kind)
	}
	if err := r.done(); err != nil {
		return Reply{}, err
	}
	return rep, nil
}

// appendStr appends s with a one-byte length, cut to maxFieldLen bytes at a
// rune boundary.
func appendStr(b []byte, s string) []byte {
	if len(s) > maxFieldLen {
		s = s[:maxFieldLen]
		for len(s) > 0 && !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	b = append(b, byte(len(s))) // #nosec G115 -- cut to maxFieldLen (255) above.
	return append(b, s...)
}

// reader reads big-endian fields and remembers the first overrun.
type reader struct {
	err error
	b   []byte
}

func (r *reader) take(n int) []byte {
	if r.err != nil || n > len(r.b) {
		if r.err == nil {
			r.err = fmt.Errorf("%w: truncated", ErrMalformed)
		}
		return make([]byte, min(n, 8)) // callers read at most a few bytes of a failed take
	}
	out := r.b[:n:n]
	r.b = r.b[n:]
	return out
}

func (r *reader) u8() byte { return r.take(1)[0] }

func (r *reader) u16() int { return int(binary.BigEndian.Uint16(r.take(2))) }

func (r *reader) u32() int {
	v := binary.BigEndian.Uint32(r.take(4))
	if v > MaxReply {
		if r.err == nil {
			r.err = fmt.Errorf("%w: length", ErrMalformed)
		}
		return 0
	}
	return int(v)
}

func (r *reader) u64() uint64 { return binary.BigEndian.Uint64(r.take(8)) }

// str reads a string with a one-byte length that must be valid UTF-8.
func (r *reader) str() string {
	s := r.take(int(r.u8()))
	if r.err == nil && !utf8.Valid(s) {
		r.err = fmt.Errorf("%w: not UTF-8", ErrMalformed)
	}
	return string(s)
}

// done returns the first error of the reader, or one if bytes are left.
func (r *reader) done() error {
	if r.err != nil {
		return r.err
	}
	if len(r.b) != 0 {
		return fmt.Errorf("%w: %d trailing bytes", ErrMalformed, len(r.b))
	}
	return nil
}
