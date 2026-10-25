// Package app wires inkwell's services for the desktop shell. It is the only
// package that knows every other one (docs/architecture.md).
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
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

// Adopter is what joining an invitation needs to make this device a member of
// the host's project ([Wiring.Adopt]).
type Adopter interface {
	Adopt(project trust.ProjectID) error
}

// Session is the Wails service behind "Partager": invite people or join an
// invitation. Its exported methods are bound to the frontend.
type Session struct {
	net     Network
	adm     *Admission
	adopter Adopter
}

// SessionOption configures a [Session].
type SessionOption func(*Session)

// WithAdmission makes Host open the invitation of adm, and Join admit the
// peer joined.
func WithAdmission(adm *Admission) SessionOption { return func(s *Session) { s.adm = adm } }

// WithAdopter makes Join move this device into the project named by the
// invitation.
func WithAdopter(a Adopter) SessionOption { return func(s *Session) { s.adopter = a } }

// NewSession returns a session over n.
func NewSession(n Network, opts ...SessionOption) *Session {
	s := &Session{net: n}
	for _, o := range opts {
		o(s)
	}
	return s
}

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
// The invitation names the project of the host, so that the guest joins it.
// It stays open: whoever holds it can join and becomes a writer.
// TODO(Task 8): single-use tickets replace this.
func (s *Session) Host() (string, error) {
	addrs := s.net.Addrs()
	if len(addrs) == 0 {
		return "", errors.New("host: this machine has no network address")
	}
	text := invitation(addrs)
	if s.adm != nil {
		s.adm.OpenInvitation()
		text = withProject(text, s.adm.current().Project())
	}
	return text, nil
}

// Join connects to the peer that made the invitation text and returns its
// identity, so the caller can open that peer's project. Joining a peer that is
// already connected succeeds and returns the same identity.
//
// The device moves into the project the invitation names, which only works
// while it is alone in a project of its own ([Wiring.Adopt]); otherwise Join
// fails and nothing changes.
func (s *Session) Join(ctx context.Context, text string) (string, error) {
	text, project, err := cutProject(text)
	if err != nil {
		return "", err
	}
	peer, addrs, err := parseInvitation(text)
	if err != nil {
		return "", err
	}
	if peer == s.net.Local() {
		return "", fmt.Errorf("%w: this is your own invitation", errBadInvitation)
	}
	if project != nil && s.adopter != nil {
		if err := s.adopter.Adopt(*project); err != nil {
			return "", fmt.Errorf("join: %w", err)
		}
	}
	if s.adm != nil {
		s.adm.Invite(peer, addrs)
	}
	ctx, cancel := context.WithTimeout(ctx, joinTimeout)
	defer cancel()
	if err := s.net.Connect(ctx, peer, addrs); err != nil {
		return "", fmt.Errorf("join: %w", err)
	}
	return string(peer), nil
}
