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

// errNoProject is returned by the sessions that are not tied to a project.
var errNoProject = errors.New("no project to invite to")

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
	inv *invites
}

// NewSession returns a session over n. It cannot invite or join; [Start]
// makes the one that can.
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

// parseRole converts a role name of the frontend.
func parseRole(s string) (trust.Role, error) {
	switch s {
	case "reader":
		return trust.RoleReader, nil
	case "writer":
		return trust.RoleWriter, nil
	case "admin":
		return trust.RoleAdmin, nil
	}
	return 0, fmt.Errorf("unknown role %q", s)
}

// Invite issues an invitation for a new person with role ("reader",
// "writer" or "admin"), valid for 15 minutes and for one person. It carries
// the same ticket as a short code (when the code server answers) and as a
// text to paste. Only an admin can invite.
func (s *Session) Invite(role string) (Invitation, error) {
	if s.inv == nil {
		return Invitation{}, errNoProject
	}
	r, err := parseRole(role)
	if err != nil {
		return Invitation{}, err
	}
	return s.inv.invite(r, nil)
}

// InviteDevice is like [Session.Invite] for another device of the same person
// as this one, which gets this device's role. Only an admin can invite.
func (s *Session) InviteDevice() (Invitation, error) {
	if s.inv == nil {
		return Invitation{}, errNoProject
	}
	role, person, err := s.inv.myRoleAndPerson()
	if err != nil {
		return Invitation{}, err
	}
	return s.inv.invite(role, &person)
}

// Join uses an invitation, a code or a text, to join the project of the person
// who made it, and returns that person's device identity, so that the caller
// can open the project. The project then replaces the one this device had.
func (s *Session) Join(ctx context.Context, codeOrText string) (string, error) {
	if s.inv == nil {
		return "", errNoProject
	}
	peer, err := s.inv.join(ctx, codeOrText)
	return string(peer), err
}

// SetVisible opens (or closes) the window "visible to join", for ten minutes:
// the devices of the local network see this one and may ask to join; requests
// wait in [Session.Requests]. Outside the window this device is not visible.
func (s *Session) SetVisible(on bool) error {
	if s.inv == nil {
		return errNoProject
	}
	s.inv.currentHost().SetVisible(on)
	return nil
}

// Visible reports whether the window "visible to join" is open.
func (s *Session) Visible() bool { return s.inv != nil && s.inv.currentHost().Visible() }

// VisibleUntil returns when the window "visible to join" ends; the zero time
// when it is closed.
func (s *Session) VisibleUntil() time.Time {
	if s.inv == nil {
		return time.Time{}
	}
	h := s.inv.currentHost()
	if !h.Visible() {
		return time.Time{}
	}
	return h.VisibleUntil()
}

// Nearby returns the visible devices of the local network, which offer a
// project to join (see [Session.Ask]).
func (s *Session) Nearby(ctx context.Context) []NearbyPerson {
	if s.inv == nil {
		return nil
	}
	return s.inv.nearby(ctx)
}

// Ask asks the nearby device peer (see [Session.Nearby]) to let this device
// join its project, under name, and waits for a person to answer. It returns
// the identity of that device, like [Session.Join].
func (s *Session) Ask(ctx context.Context, peer, name string) (string, error) {
	if s.inv == nil {
		return "", errNoProject
	}
	p, err := s.inv.ask(ctx, transport.PeerID(peer), name)
	return string(p), err
}

// Requests returns the requests to join that wait for an answer.
func (s *Session) Requests() []JoinRequest {
	if s.inv == nil {
		return nil
	}
	pending := s.inv.currentHost().Pending()
	out := make([]JoinRequest, len(pending))
	for i, p := range pending {
		out[i] = JoinRequest{ID: p.ID, Peer: string(p.Peer), Name: p.Name, Fingerprint: p.Fingerprint}
	}
	return out
}

// Answer answers the request id: with a role ("reader", "writer" or "admin")
// to let the person in with it, or with "refuse".
func (s *Session) Answer(id int, decision string) error {
	if s.inv == nil {
		return errNoProject
	}
	if decision == "refuse" {
		return s.inv.currentHost().Answer(id, false, 0)
	}
	r, err := parseRole(decision)
	if err != nil {
		return err
	}
	return s.inv.currentHost().Answer(id, true, r)
}
