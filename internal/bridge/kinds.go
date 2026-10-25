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

	lastKind = Sealed
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

// Attester is implemented by a repo transport that controls who may write
// (package app's wiring). A [Router] over such a transport turns the Attest
// envelopes of the webview into calls to Attest and gives a newly attached
// webview the current Notices.
type Attester interface {
	// Attest signs the heads of doc with the device key and gossips them to
	// the members, ahead of the repo frames sent afterwards. It fails if this
	// device may not write, or may no longer sign.
	Attest(ctx context.Context, doc string, heads [][HashSize]byte) error
	// Notices returns the Attested and Sealed envelopes describing the
	// current state, for a webview that just attached. It never blocks on
	// the network.
	Notices() []Envelope
}

// Source is implemented by a repo transport that reports more than
// [transport.Event]s: its envelopes (Attested, Revoked, Sealed) are ordered
// with the Message envelopes, so an attestation precedes the changes it
// covers. [Router.Run] reads it instead of Recv.
type Source interface {
	// RecvEnvelope blocks for the next envelope, like [transport.Transport.Recv].
	RecvEnvelope(ctx context.Context) (Envelope, error)
}
