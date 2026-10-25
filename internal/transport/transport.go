// Package transport defines how inkwell moves opaque bytes between peers.
//
// Everything that crosses the network goes through a [Transport]: the bridge
// hands it the frames automerge-repo produces in the webview, and the
// implementation delivers them to the addressed peer. A transport never
// interprets a payload; it only knows peers and bytes.
//
// Implementations live in sub-packages: memnet (in-memory, fault-injecting,
// for tests and simulation), tcp (stage 1) and p2p (libp2p, stage 2).
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
)

// MaxFrame is the largest payload a transport accepts or delivers, in bytes.
//
// Wails rejects stream frames above 64 MiB, and the bridge wraps each payload
// in an envelope that adds a 3-byte header and a peer ID of at most
// maxPeerIDLen (64) bytes. MaxFrame is 64 MiB minus those 67 bytes, so a
// payload that crossed the network always fits one bridge frame.
const MaxFrame = 64<<20 - 3 - maxPeerIDLen

// maxPeerIDLen bounds a [PeerID] so it fits a one-byte length prefix.
const maxPeerIDLen = 64

// PeerID identifies a peer. It is opaque to callers and is valid when
// [ValidPeerID] reports true.
type PeerID string

// ValidPeerID reports whether id is 1 to 64 bytes of printable,
// non-space ASCII. Anything a remote peer claims as its identity must pass
// this check before it is used as a map key, logged or displayed.
func ValidPeerID(id PeerID) bool {
	if len(id) == 0 || len(id) > maxPeerIDLen {
		return false
	}
	for i := range len(id) {
		if c := id[i]; c <= ' ' || c > '~' {
			return false
		}
	}
	return true
}

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
	// Payload is the received bytes; set only when Kind is Message. The
	// receiver owns it.
	Payload []byte
	// Peer is the peer the event is about: the sender for a Message.
	Peer PeerID
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
