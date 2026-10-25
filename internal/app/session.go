// Package app wires inkwell's services for the desktop shell. It is the only
// package that knows every other one (docs/architecture.md).
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// joinTimeout bounds how long Join waits for the remote handshake.
const joinTimeout = 10 * time.Second

// Network is what a Session needs from the transport: an identity, the
// addresses where it can be reached and a way to reach another peer.
type Network interface {
	Local() transport.PeerID
	// Addrs returns the node's addresses, each ending with /p2p/<id>.
	Addrs() []string
	Connect(ctx context.Context, to transport.PeerID, addrs []string) error
	Peers() []transport.PeerID
}

// Session is the Wails service behind "Partager": invite people or join an
// invitation. Its exported methods are bound to the frontend.
type Session struct {
	net Network
}

// NewSession returns a session over n.
func NewSession(n Network) *Session { return &Session{net: n} }

// LocalPeer returns this machine's peer identity.
func (s *Session) LocalPeer() string { return string(s.net.Local()) }

// Peers returns the connected peers, sorted.
func (s *Session) Peers() []string {
	peers := s.net.Peers()
	out := make([]string, len(peers))
	for i, p := range peers {
		out[i] = string(p)
	}
	return out
}

// Host returns the invitation to give to the people who should join: an
// opaque text that Join accepts. The node already listens (see [StartNode]),
// so calling Host again returns the current addresses.
//
// TODO(Task 7): the node admits every peer that has the invitation; the
// signed member list restricts that.
func (s *Session) Host() (string, error) {
	addrs := s.net.Addrs()
	if len(addrs) == 0 {
		return "", errors.New("host: this machine has no network address")
	}
	return invitation(addrs), nil
}

// Join connects to the peer that made the invitation text and returns its
// identity, so the caller can open that peer's project. Joining a peer that is
// already connected succeeds and returns the same identity.
func (s *Session) Join(ctx context.Context, text string) (string, error) {
	peer, addrs, err := parseInvitation(text)
	if err != nil {
		return "", err
	}
	if peer == s.net.Local() {
		return "", fmt.Errorf("%w: this is your own invitation", errBadInvitation)
	}
	ctx, cancel := context.WithTimeout(ctx, joinTimeout)
	defer cancel()
	if err := s.net.Connect(ctx, peer, addrs); err != nil {
		return "", fmt.Errorf("join: %w", err)
	}
	return string(peer), nil
}
