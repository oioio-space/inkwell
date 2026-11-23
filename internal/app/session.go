// Package app wires inkwell's services for the desktop shell. It is the only
// package that knows every other one (docs/architecture.md).
package app

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/oioio-space/inkwell/internal/project"
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
	chg *change // the project change of this image; nil without a project
	// proj is the open project; nil without one.
	proj *project.Project
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

// The standing of this device in the member list of the open project, see
// [Session.Standing].
const (
	// StandingMember: the list gives this device a role.
	StandingMember = "member"
	// StandingRemoved: the list holds an op about this device, but gives it no role.
	StandingRemoved = "removed"
	// StandingNotNamed: the list never names this device, as when the folder was
	// made by another profile.
	StandingNotNamed = "notNamed"
)

// standingOf computes the standing of pub in list.
func standingOf(list *trust.List, pub ed25519.PublicKey) string {
	switch {
	case list.Role(pub) != 0:
		return StandingMember
	case slices.ContainsFunc(list.Since(nil), func(o trust.Op) bool { return pub.Equal(o.Subject) }):
		return StandingRemoved
	}
	return StandingNotNamed
}

// Standing reports whether this device is in the member list of the open
// project: "member", "removed" or "notNamed". It follows the list; the webview asks again
// whenever it receives the roles event. A session without project is "notNamed".
func (s *Session) Standing() string {
	if s.inv == nil {
		return StandingNotNamed
	}
	// The list of the wiring, not the one of the invitations: the wiring follows
	// a new project before it announces the roles, the invitations only after.
	return standingOf(s.inv.wiring.list(), s.inv.dev.Public())
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
// text to paste. Only an admin can invite: for anyone else the result has
// [Invitation.Refused] and no error.
func (s *Session) Invite(role string) (Invitation, error) {
	if err := s.leaving(); err != nil {
		return Invitation{}, err
	}
	if key := s.inv.refusal(); key != "" {
		return Invitation{Refused: key}, nil
	}
	r, err := parseRole(role)
	if err != nil {
		return Invitation{}, err
	}
	return s.inv.invite(r, nil)
}

// InviteDevice is like [Session.Invite] for another device of the same person
// as this one, which gets this device's role. Only an admin can invite, as for
// [Session.Invite].
func (s *Session) InviteDevice() (Invitation, error) {
	if err := s.leaving(); err != nil {
		return Invitation{}, err
	}
	if key := s.inv.refusal(); key != "" {
		return Invitation{Refused: key}, nil
	}
	role, person, err := s.inv.myRoleAndPerson()
	if err != nil {
		return Invitation{}, err
	}
	return s.inv.invite(role, &person)
}

// InvitationState says whether text, the text of an invitation this device made
// ([Invitation.Text]), still works: [InvitationOpen] while this device holds its ticket,
// [InvitationExpired] past its end, [InvitationUsed] otherwise (someone redeemed it, or it was
// withdrawn). Only Go knows: a ticket is single-use and is taken before the person is admitted,
// whether or not that person then connects (rule J-8). A text that is not an invitation is an
// error.
func (s *Session) InvitationState(text string) (string, error) {
	if s.inv == nil {
		return "", errNoProject
	}
	return s.inv.state(text)
}

// Join uses an invitation, a code or a text, to join the project of the person who made it. For
// another project than the open one, the verified answer is only held in memory: the result says
// Restart, nothing is written, and the project of the running session never changes; the page
// leaves its context and calls [Session.FinishJoin] (rule J-1). For the open project the result
// says no restart and nothing happens (rule J-5).
func (s *Session) Join(ctx context.Context, codeOrText string) (JoinResult, error) {
	if err := s.leaving(); err != nil {
		return JoinResult{}, err
	}
	return s.inv.join(ctx, codeOrText)
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
// join its project, under name, and waits for a person to answer. Its result
// is the one of [Session.Join].
func (s *Session) Ask(ctx context.Context, peer, name string) (JoinResult, error) {
	if err := s.leaving(); err != nil {
		return JoinResult{}, err
	}
	return s.inv.ask(ctx, transport.PeerID(peer), name)
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
