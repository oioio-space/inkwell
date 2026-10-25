// Package bridge links the webview to the [transport.Transport].
//
// The webview speaks to Go over one Wails stream, [StreamName], whose frames
// are binary [Envelope]s. A [Router] attaches the current stream connection to
// the transport: frames from the webview are decoded and sent to the addressed
// peer, and transport events are encoded and pushed to the webview. Payloads
// are opaque; the bridge never interprets them.
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
// Towards Go only Kind [transport.Message] is accepted and Peer is the
// destination. Towards the webview all kinds are sent and Peer is the peer
// the event is about (the sender of a Message).
type Envelope struct {
	// Payload is the opaque message bytes; empty unless Kind is Message.
	Payload []byte
	// Peer is the remote peer.
	Peer transport.PeerID
	// Kind says what the envelope reports.
	Kind transport.Kind
}

// validate checks the invariants shared by Encode and Decode.
func (e Envelope) validate() error {
	switch {
	case e.Kind < transport.PeerUp || e.Kind > transport.Message:
		return fmt.Errorf("%w: kind %d", ErrMalformed, e.Kind)
	case !transport.ValidPeerID(e.Peer):
		return fmt.Errorf("%w: invalid peer", ErrMalformed)
	case e.Kind != transport.Message && len(e.Payload) > 0:
		return fmt.Errorf("%w: payload on %v", ErrMalformed, e.Kind)
	case len(e.Payload) > transport.MaxFrame:
		return fmt.Errorf("%w: payload of %d bytes", ErrMalformed, len(e.Payload))
	}
	return nil
}

// Encode serializes e into a freshly allocated slice the caller may hand over
// (Wails' Send takes ownership). It returns an error wrapping [ErrMalformed]
// if e is not a valid envelope.
func Encode(e Envelope) ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, headerLen+len(e.Peer)+len(e.Payload))
	b = append(b, version, byte(e.Kind), byte(len(e.Peer)))
	b = append(b, e.Peer...)
	b = append(b, e.Payload...)
	return b, nil
}

// Decode parses an envelope received from the webview. The returned Payload
// aliases b. It returns an error wrapping [ErrMalformed] for an unknown
// version or kind, a truncated or invalid peer, a payload on a non-Message
// kind, or an oversized frame.
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
		Kind:    transport.Kind(b[1]),
		Peer:    transport.PeerID(b[headerLen : headerLen+peerLen]),
		Payload: b[headerLen+peerLen:],
	}
	if err := e.validate(); err != nil {
		return Envelope{}, err
	}
	return e, nil
}
