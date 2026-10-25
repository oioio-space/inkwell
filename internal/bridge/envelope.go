// Package bridge links the webview to the [transport.Transport].
//
// The webview speaks to Go over one Wails stream, [StreamName], whose frames
// are binary [Envelope]s. A [Router] attaches the current stream connection to
// the transport: frames from the webview are decoded and sent to the addressed
// peer, and transport events are encoded and pushed to the webview. The
// payloads of messages are opaque; the bridge never interprets them. The
// webview also asks the device to attest the heads of a document, and learns
// who attested what, through the [Attester] and [Source] of the transport: Go
// signs, the webview alone can read the documents (see [Kind]).
//
// Every frame from the webview is untrusted and decoded strictly.
//
//	r := bridge.NewRouter(t, log)
//	go r.Run(ctx)       // single consumer of t.Recv, for the app lifetime
//	r.Serve(conn)       // per stream connection; blocks until it closes
package bridge

import (
	"errors"
	"fmt"

	"github.com/oioio-space/inkwell/internal/transport"
)

// StreamName is the name of the Wails stream the webview opens.
const StreamName = "net"

// Wire format: version u8 | kind u8 | len(peer) u8 | peer | payload.
const (
	version   = 1
	headerLen = 3
	// maxWire bounds a whole envelope: the Wails stream frame limit, which
	// transport.MaxFrame leaves room for the header and the longest peer ID.
	maxWire = 64 << 20
)

// ErrMalformed is wrapped by the errors [Encode] and [Decode] return for an
// envelope that is not valid.
var ErrMalformed = errors.New("bridge: malformed envelope")

// Envelope is one bridge frame.
//
// Towards Go only [Message] and [Attest] are accepted; Peer is the destination
// of a Message. Towards the webview all kinds are sent and Peer is the peer
// the event is about (the sender of a Message, the signer of an Attested).
type Envelope struct {
	// Peer is the remote peer.
	Peer transport.PeerID
	// Payload is the opaque message bytes of a Message, the [Heads] encoding
	// of an Attest or Attested, the [EncodeRoles] encoding of a Roles, or the
	// byte of a Suspended; empty for the other kinds.
	Payload []byte
	// Kind says what the envelope reports.
	Kind Kind
}

// validate checks the invariants shared by Encode and Decode.
func (e Envelope) validate() error {
	switch {
	case e.Kind < PeerUp || e.Kind > lastKind:
		return fmt.Errorf("%w: kind %d", ErrMalformed, e.Kind)
	case !transport.ValidPeerID(e.Peer):
		return fmt.Errorf("%w: invalid peer", ErrMalformed)
	case !carriesPayload(e.Kind) && len(e.Payload) > 0:
		return fmt.Errorf("%w: payload on %v", ErrMalformed, e.Kind)
	case len(e.Payload) > transport.MaxFrame:
		return fmt.Errorf("%w: payload of %d bytes", ErrMalformed, len(e.Payload))
	case e.Kind == Attest || e.Kind == Attested:
		if _, err := DecodeHeads(e.Payload); err != nil {
			return err
		}
	case e.Kind == Roles:
		if _, err := DecodeRoles(e.Payload); err != nil {
			return err
		}
	case e.Kind == Suspended && (len(e.Payload) != 1 || e.Payload[0] > 1):
		return fmt.Errorf("%w: suspended payload", ErrMalformed)
	}
	return nil
}

// carriesPayload reports whether envelopes of kind k have a payload.
func carriesPayload(k Kind) bool {
	switch k {
	case Message, Attest, Attested, Roles, Suspended:
		return true
	}
	return false
}

// Encode serializes e into a freshly allocated slice the caller may hand over
// (Wails' Send takes ownership). It returns an error wrapping [ErrMalformed]
// if e is not a valid envelope.
func Encode(e Envelope) ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, headerLen+len(e.Peer)+len(e.Payload))
	b = append(b, version, byte(e.Kind), byte(len(e.Peer))) // #nosec G115 -- validate bounds Kind ≤ lastKind and len(Peer) ≤ 64.
	b = append(b, e.Peer...)
	b = append(b, e.Payload...)
	return b, nil
}

// Decode parses an envelope received from the webview. The returned Payload
// aliases b. It returns an error wrapping [ErrMalformed] for an unknown
// version or kind, a truncated or invalid peer, a payload on a kind that carries
// none or an invalid [Heads] one, or an oversized frame.
func Decode(b []byte) (Envelope, error) {
	if len(b) > maxWire {
		return Envelope{}, fmt.Errorf("%w: %d bytes", ErrMalformed, len(b))
	}
	if len(b) < headerLen {
		return Envelope{}, fmt.Errorf("%w: truncated header", ErrMalformed)
	}
	if b[0] != version {
		return Envelope{}, fmt.Errorf("%w: version %d", ErrMalformed, b[0])
	}
	peerLen := int(b[2])
	if len(b) < headerLen+peerLen {
		return Envelope{}, fmt.Errorf("%w: truncated peer", ErrMalformed)
	}
	e := Envelope{
		Kind:    Kind(b[1]),
		Peer:    transport.PeerID(b[headerLen : headerLen+peerLen]),
		Payload: b[headerLen+peerLen:],
	}
	if err := e.validate(); err != nil {
		return Envelope{}, err
	}
	return e, nil
}
