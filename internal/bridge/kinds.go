package bridge

import (
	"context"
	"encoding/binary"
	"fmt"
	"unicode/utf8"

	"github.com/oioio-space/inkwell/internal/transport"
)

// Kind says what an [Envelope] carries. It extends [transport.Kind]: the
// first three values are the transport's, and the others are the write
// enforcement of the webview (see [Attester]).
type Kind uint8

// The kinds of envelope. Towards Go the webview sends Message and Attest;
// towards the webview Go sends all the others.
const (
	// PeerUp reports that Peer is connected and allowed.
	PeerUp = Kind(transport.PeerUp)
	// PeerDown reports that Peer is gone.
	PeerDown = Kind(transport.PeerDown)
	// Message carries opaque bytes to or from Peer.
	Message = Kind(transport.Message)
	// Attest (webview to Go) asks the device to sign the heads of a
	// document. Peer is ignored; Payload is a [Heads] encoding.
	Attest Kind = 4
	// Attested (Go to webview) reports that Peer, a current writer, signed
	// the heads of a document. Payload is a [Heads] encoding. It reaches the
	// webview before the repo frames that Peer sent after the attestation.
	Attested Kind = 5
	// Revoked (Go to webview) reports that Peer is no longer a writer: its
	// attestations are void.
	Revoked Kind = 6
	// Sealed (Go to webview) reports that the member list is full: this
	// device signs no more, and the user must create a new project. Peer is
	// the local device.
	Sealed Kind = 7
	// Roles (Go to webview) reports the member list: the role and the
	// suspension of every device, so the webview knows who the admins are.
	// Peer is the local device; Payload is a [MemberRole] list, see
	// [EncodeRoles]. It is sent whenever the roles change.
	Roles Kind = 8
	// Suspended (Go to webview) reports a change of this device's own
	// suspension: Peer is the admin who made it, and Payload is one byte, 1 if
	// this device was suspended and 0 if it was let back.
	Suspended Kind = 9

	lastKind = Suspended
)

// String returns the name of k.
func (k Kind) String() string {
	switch k {
	case PeerUp:
		return "PeerUp"
	case PeerDown:
		return "PeerDown"
	case Message:
		return "Message"
	case Attest:
		return "Attest"
	case Attested:
		return "Attested"
	case Revoked:
		return "Revoked"
	case Sealed:
		return "Sealed"
	case Roles:
		return "Roles"
	case Suspended:
		return "Suspended"
	}
	return fmt.Sprintf("Kind(%d)", uint8(k))
}

// Limits of a [Heads] encoding; they match the attestation limits of package
// trust.
const (
	// HashSize is the size of a change hash.
	HashSize = 32
	// MaxDoc bounds Heads.Doc, in bytes.
	MaxDoc = 256
	// MaxHeads bounds the number of heads.
	MaxHeads = 64
)

// Heads names the heads of one document: the payload of Attest and Attested.
// Wire format: len(doc) u16 big endian | doc | heads, each [HashSize] bytes.
type Heads struct {
	// Doc is the document ID.
	Doc string
	// Heads are the change hashes; the order is kept.
	Heads [][HashSize]byte
}

func (h Heads) validate() error {
	if h.Doc == "" || len(h.Doc) > MaxDoc || !utf8.ValidString(h.Doc) || len(h.Heads) > MaxHeads {
		return fmt.Errorf("%w: heads fields", ErrMalformed)
	}
	return nil
}

// EncodeHeads serializes h. It returns an error wrapping [ErrMalformed] if h
// breaks the limits.
func EncodeHeads(h Heads) ([]byte, error) {
	if err := h.validate(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, 2+len(h.Doc)+HashSize*len(h.Heads))
	b = binary.BigEndian.AppendUint16(b, uint16(len(h.Doc))) // #nosec G115 -- validate bounds it by MaxDoc.
	b = append(b, h.Doc...)
	for _, x := range h.Heads {
		b = append(b, x[:]...)
	}
	return b, nil
}

// DecodeHeads parses an encoding of [EncodeHeads], strictly. It returns an
// error wrapping [ErrMalformed] for a truncated or oversized encoding.
func DecodeHeads(b []byte) (Heads, error) {
	if len(b) < 2 {
		return Heads{}, fmt.Errorf("%w: truncated heads", ErrMalformed)
	}
	n := int(binary.BigEndian.Uint16(b))
	rest := b[2:]
	if n > len(rest) {
		return Heads{}, fmt.Errorf("%w: truncated document", ErrMalformed)
	}
	h := Heads{Doc: string(rest[:n])}
	rest = rest[n:]
	if len(rest)%HashSize != 0 {
		return Heads{}, fmt.Errorf("%w: heads of %d bytes", ErrMalformed, len(rest))
	}
	if cnt := len(rest) / HashSize; cnt > 0 {
		if cnt > MaxHeads {
			return Heads{}, fmt.Errorf("%w: %d heads", ErrMalformed, cnt)
		}
		h.Heads = make([][HashSize]byte, cnt)
		for i := range h.Heads {
			copy(h.Heads[i][:], rest[i*HashSize:])
		}
	}
	if err := h.validate(); err != nil {
		return Heads{}, err
	}
	return h, nil
}

// MaxMembers bounds the entries of a Roles payload: the member list holds at
// most 1024 ops, so at most that many devices.
const MaxMembers = 1024

// MemberRole is one device in a Roles payload.
type MemberRole struct {
	// Peer is the device.
	Peer transport.PeerID
	// Role is 1 for a reader, 2 for a writer and 3 for an admin (the values of
	// package trust's Role).
	Role uint8
	// Suspended reports whether the device is suspended.
	Suspended bool
}

// flagSuspended is the only flag of a roles entry.
const flagSuspended = 1

// EncodeRoles serializes rs, the payload of a Roles envelope. Wire format:
// count u16 big endian, then for each entry len(peer) u8 | peer | role u8 |
// flags u8 (bit 0: suspended). It returns an error wrapping [ErrMalformed] if
// rs breaks the limits.
func EncodeRoles(rs []MemberRole) ([]byte, error) {
	if len(rs) > MaxMembers {
		return nil, fmt.Errorf("%w: %d members", ErrMalformed, len(rs))
	}
	b := binary.BigEndian.AppendUint16(make([]byte, 0, 2+len(rs)*8), uint16(len(rs))) // #nosec G115 -- bounded by MaxMembers.
	for _, r := range rs {
		if !transport.ValidPeerID(r.Peer) || r.Role < 1 || r.Role > 3 {
			return nil, fmt.Errorf("%w: member entry", ErrMalformed)
		}
		var flags byte
		if r.Suspended {
			flags = flagSuspended
		}
		b = append(b, byte(len(r.Peer))) // #nosec G115 -- ValidPeerID bounds it to 64.
		b = append(b, r.Peer...)
		b = append(b, r.Role, flags)
	}
	return b, nil
}

// DecodeRoles parses an encoding of [EncodeRoles], strictly. It returns an
// error wrapping [ErrMalformed] for a truncated or oversized list, an invalid
// peer, an unknown role or flag, or trailing bytes.
func DecodeRoles(b []byte) ([]MemberRole, error) {
	if len(b) < 2 {
		return nil, fmt.Errorf("%w: truncated roles", ErrMalformed)
	}
	n := int(binary.BigEndian.Uint16(b))
	if n > MaxMembers {
		return nil, fmt.Errorf("%w: %d members", ErrMalformed, n)
	}
	rest := b[2:]
	var out []MemberRole
	if n > 0 {
		out = make([]MemberRole, 0, n)
	}
	for range n {
		if len(rest) < 1 || len(rest) < 1+int(rest[0])+2 {
			return nil, fmt.Errorf("%w: truncated member entry", ErrMalformed)
		}
		pl := int(rest[0])
		r := MemberRole{Peer: transport.PeerID(rest[1 : 1+pl]), Role: rest[1+pl]}
		flags := rest[2+pl]
		rest = rest[3+pl:]
		if !transport.ValidPeerID(r.Peer) || r.Role < 1 || r.Role > 3 || flags&^flagSuspended != 0 {
			return nil, fmt.Errorf("%w: member entry", ErrMalformed)
		}
		r.Suspended = flags&flagSuspended != 0
		out = append(out, r)
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("%w: trailing bytes", ErrMalformed)
	}
	return out, nil
}

// Attester is implemented by a repo transport that controls who may write
// (package app's wiring). A [Router] over such a transport turns the Attest
// envelopes of the webview into calls to Attest and gives a newly attached
// webview the current Notices.
type Attester interface {
	// Attest signs the heads of doc with the device key and gossips them to
	// the members, ahead of the repo frames sent afterwards. It fails if this
	// device may not write, or may no longer sign.
	Attest(ctx context.Context, doc string, heads [][HashSize]byte) error
	// Notices returns the Attested, Sealed, Roles and Suspended envelopes
	// describing the current state, for a webview that just attached. It never blocks on
	// the network.
	Notices() []Envelope
}

// Source is implemented by a repo transport that reports more than
// [transport.Event]s: its envelopes (Attested, Revoked, Sealed, Roles,
// Suspended) are ordered
// with the Message envelopes, so an attestation precedes the changes it
// covers. [Router.Run] reads it instead of Recv.
type Source interface {
	// RecvEnvelope blocks for the next envelope, like [transport.Transport.Recv].
	RecvEnvelope(ctx context.Context) (Envelope, error)
}
