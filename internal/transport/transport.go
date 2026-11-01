// Package transport defines how inkwell moves opaque bytes between peers.
//
// Everything that crosses the network goes through a [Transport]: the bridge
// hands it the frames automerge-repo produces in the webview, and the
// implementation delivers them to the addressed peer. A transport never
// interprets a payload; it only knows peers and bytes.
//
// Implementations live in sub-packages: memnet (in-memory, fault-injecting,
// for tests and simulation), frame (length-prefixed framing) and p2p (libp2p).
//
// Typical use, with a single consumer draining events:
//
//	for {
//		ev, err := t.Recv(ctx)
//		if err != nil {
//			return err
//		}
//		switch ev.Kind {
//		case transport.PeerUp, transport.PeerDown, transport.Message:
//			// …
//		}
//	}
package transport

import (
	"context"
	"errors"

	"github.com/oioio-space/inkwell/protocol"
)

// MaxFrame is the largest payload a transport accepts or delivers, in bytes.
//
// Wails rejects stream frames above 64 MiB, and the bridge wraps each payload
// in an envelope that adds a 3-byte header and a peer ID of at most 64 bytes.
const MaxFrame = protocol.MaxFrame

// PeerID identifies an opaque peer.
type PeerID = protocol.PeerID

// ValidPeerID reports whether a peer identifier is valid.
var ValidPeerID = protocol.ValidPeerID

// Kind tells what an [Event] reports.
type Kind uint8

const (
	// PeerUp reports that a peer became reachable.
	PeerUp Kind = iota + 1
	// PeerDown reports that a previously reachable peer is gone.
	PeerDown
	// Message carries a payload received from a peer.
	Message
)

// String returns the kind's name, for logs.
func (k Kind) String() string {
	switch k {
	case PeerUp:
		return "PeerUp"
	case PeerDown:
		return "PeerDown"
	case Message:
		return "Message"
	default:
		return "Kind(?)"
	}
}

// Event is something that happened on a transport.
type Event struct {
	// Peer is the peer the event is about: the sender for a Message.
	Peer PeerID
	// Payload is the received bytes; set only when Kind is Message. The
	// receiver owns it.
	Payload []byte
	// Kind says what happened.
	Kind Kind
}

// Transport moves opaque payloads between this peer and its connected peers.
//
// Implementations are safe for concurrent use, except that Recv must have a
// single consumer.
type Transport interface {
	// Local returns this peer's identity.
	Local() PeerID
	// Send delivers payload to peer to. Send does not retain payload; the
	// caller may reuse it once Send returns. It returns ErrUnknownPeer if to
	// is not connected, ErrFrameTooLarge if payload exceeds MaxFrame, and
	// ErrClosed after Close.
	Send(ctx context.Context, to PeerID, payload []byte) error
	// Recv returns the next queued event, blocking until one arrives or ctx
	// is done; an already queued event is returned even if ctx is done. It
	// returns ErrClosed once the transport is closed; queued events are
	// discarded.
	Recv(ctx context.Context) (Event, error)
	// Peers returns the currently connected peers, sorted.
	Peers() []PeerID
	// Close disconnects every peer and releases resources. Linked peers
	// observe a PeerDown. Close is idempotent.
	Close() error
}

var (
	// ErrUnknownPeer is returned when sending to a peer that is not connected.
	ErrUnknownPeer = errors.New("transport: unknown peer")
	// ErrClosed is returned by a transport that has been closed.
	ErrClosed = errors.New("transport: closed")
	// ErrFrameTooLarge is returned for a payload larger than MaxFrame.
	ErrFrameTooLarge = errors.New("transport: frame too large")
)
