package invite

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

const (
	// VisibleFor is how long "visible to join" lasts, in the manner of
	// AirDrop's "Everyone for 10 minutes".
	VisibleFor = 10 * time.Minute
	// AskTimeout is how long a request to join waits for an answer.
	AskTimeout = 2 * time.Minute

	maxTickets = 16
	maxAsks    = 8
)

// Errors returned by this package, besides [ErrMalformed]. Use [errors.Is].
var (
	// ErrExpired is returned for a ticket past its expiry.
	ErrExpired = errors.New("invite: ticket expired")
	// ErrNoAddress is returned when the host has no address to put in a ticket
	// yet (for instance a relay-only device before its relay reservation).
	ErrNoAddress = errors.New("invite: this device has no address yet, try again in a moment (is the relay reachable?)")
	// ErrTooMany is returned when too many invitations are open at once.
	ErrTooMany = errors.New("invite: too many open invitations")
	// ErrUnknownPerson is returned for a ticket meant for a person who is not a member.
	ErrUnknownPerson = errors.New("invite: unknown person")
	// ErrNoRequest is returned by [Host.Answer] for a request that is not waiting.
	ErrNoRequest = errors.New("invite: no such request")
	// ErrBadProject is returned when what a host sends is not a valid
	// project, or not the one the ticket is for.
	ErrBadProject = errors.New("invite: the host sent an invalid project")
)

// RefusedError is returned to a would-be member whose request was refused.
type RefusedError struct {
	Reason Reason
}

func (e *RefusedError) Error() string {
	switch e.Reason {
	case ReasonDeclined:
		return "invite: the request was declined"
	case ReasonBusy:
		return "invite: the host is busy"
	case ReasonFailed:
		return "invite: the host could not add you"
	case ReasonTooLarge:
		return "invite: the project is too large to join by invitation"
	default:
		return "invite: the invitation is not valid"
	}
}

// Pending is a request to join that waits for an answer; see [Host.Pending].
type Pending struct {
	// Name is the name the stranger gave, cleaned for display.
	Name string
	// Fingerprint is the stranger's four-word fingerprint, to compare aloud.
	Fingerprint string
	// Peer is the stranger's identity.
	Peer transport.PeerID
	// ID names the request in [Host.Answer].
	ID int
}

type decision struct {
	accept bool
	role   trust.Role
}

type pendingAsk struct {
	answer chan decision
	Pending
}

// HostConfig is what a [Host] needs from the application. Every field but
// OnOpen, Now and Lang is required.
type HostConfig struct {
	// List is the member list of the project.
	List *trust.List
	// Issue signs and applies an op for this device (an Add, for the host).
	Issue func(trust.Op) (trust.Op, error)
	// KeyOf returns the key of a peer identity.
	KeyOf func(transport.PeerID) (ed25519.PublicKey, error)
	// Addrs returns the addresses where this device can be reached.
	Addrs func() []string
	// OnOpen, if not nil, is told until when strangers may connect: while a
	// ticket is outstanding or the host is visible. A zero time means never.
	OnOpen func(until time.Time)
	// Now returns the current time; the default is [time.Now].
	Now func() time.Time
	// Self is the identity of this device and SelfKey its key.
	Self transport.PeerID
	// Machine is the name of this machine, as shown in Hello.
	Machine string
	// Lang is the language ("fr" or "en") of the fingerprints shown.
	Lang    string
	SelfKey ed25519.PublicKey
}

// Host is the side of a device that takes people in: it issues tickets, serves
// the lobby protocol and answers requests to join. It is safe for concurrent
// use.
//
// A ticket is single-use and expires after [TicketTTL]; the secret is compared
// in constant time and nothing about a failed attempt is kept or logged.
// While the host is not visible only [KindRedeem] is served: any other request
// gets no answer at all.
type Host struct {
	cfg     HostConfig
	asks    map[int]*pendingAsk
	tickets []Ticket
	visible time.Time
	nextID  int
	mu      sync.Mutex
}

// NewHost returns the host of the project of cfg.List.
func NewHost(cfg HostConfig) *Host {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Host{cfg: cfg, asks: make(map[int]*pendingAsk)}
}

// Invite issues a ticket for the role, valid for [TicketTTL]. A non-nil person
// makes it a ticket for another device of that person. Only an admin can issue
// a ticket ([trust.ErrNotAdmin] otherwise).
func (h *Host) Invite(role trust.Role, person *trust.Hash) (Ticket, error) {
	if h.cfg.List.Role(h.cfg.SelfKey) != trust.RoleAdmin {
		return Ticket{}, fmt.Errorf("invite: %w", trust.ErrNotAdmin)
	}
	if !role.Valid() {
		return Ticket{}, fmt.Errorf("invite: role %d: %w", role, ErrMalformed)
	}
	if person != nil && !slices.ContainsFunc(h.cfg.List.Members(), func(m trust.Member) bool { return m.Person == *person }) {
		return Ticket{}, ErrUnknownPerson
	}
	addrs := slices.Clone(h.cfg.Addrs())
	if len(addrs) == 0 {
		return Ticket{}, ErrNoAddress
	}
	t := Ticket{
		Project: h.cfg.List.Project(),
		Host:    h.cfg.Self,
		Addrs:   addrs,
		Role:    role,
		Person:  person,
	}
	if _, err := rand.Read(t.Secret[:]); err != nil {
		return Ticket{}, fmt.Errorf("invite: random secret: %w", err)
	}
	now := h.cfg.Now()
	t.Expires = now.Add(TicketTTL).Truncate(time.Second).UTC()

	h.mu.Lock()
	h.purgeLocked(now)
	if len(h.tickets) >= maxTickets {
		h.mu.Unlock()
		return Ticket{}, ErrTooMany
	}
	h.tickets = append(h.tickets, t)
	h.mu.Unlock()
	h.opened()
	return t, nil
}

// Cancel withdraws the ticket t, if it is still outstanding.
func (h *Host) Cancel(t Ticket) {
	h.mu.Lock()
	h.tickets = slices.DeleteFunc(h.tickets, func(o Ticket) bool {
		return subtle.ConstantTimeCompare(o.Secret[:], t.Secret[:]) == 1
	})
	h.mu.Unlock()
	h.opened()
}

// SetVisible opens or closes the window "visible to join": for [VisibleFor],
// the host answers Hello and takes requests to join. Closing it declines the
// requests that wait.
func (h *Host) SetVisible(on bool) {
	var declined []*pendingAsk
	h.mu.Lock()
	if on {
		h.visible = h.cfg.Now().Add(VisibleFor)
	} else {
		h.visible = time.Time{}
		declined = slices.Collect(maps.Values(h.asks))
	}
	h.mu.Unlock()
	for _, a := range declined {
		select {
		case a.answer <- decision{}:
		default: // already answered
		}
	}
	h.opened()
}

// Visible reports whether the window "visible to join" is open.
func (h *Host) Visible() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg.Now().Before(h.visible)
}

// VisibleUntil returns when the window "visible to join" closes; it is in the
// past or zero when the window is closed.
func (h *Host) VisibleUntil() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.visible
}

// Pending returns the requests that wait for an answer, oldest first.
func (h *Host) Pending() []Pending {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Pending, 0, len(h.asks))
	for _, a := range h.asks {
		out = append(out, a.Pending)
	}
	slices.SortFunc(out, func(a, b Pending) int { return a.ID - b.ID })
	return out
}

// Answer accepts the request id with role, or refuses it.
func (h *Host) Answer(id int, accept bool, role trust.Role) error {
	if accept && !role.Valid() {
		return fmt.Errorf("invite: role %d: %w", role, ErrMalformed)
	}
	h.mu.Lock()
	a, ok := h.asks[id]
	h.mu.Unlock()
	if !ok {
		return ErrNoRequest
	}
	select {
	case a.answer <- decision{accept: accept, role: role}:
		return nil
	default:
		return ErrNoRequest // already answered
	}
}

// opened tells OnOpen until when strangers may connect.
func (h *Host) opened() {
	if h.cfg.OnOpen == nil {
		return
	}
	now := h.cfg.Now()
	h.mu.Lock()
	h.purgeLocked(now)
	until := h.visible
	for _, t := range h.tickets {
		if t.Expires.After(until) {
			until = t.Expires
		}
	}
	h.mu.Unlock()
	if !until.After(now) {
		until = time.Time{}
	}
	h.cfg.OnOpen(until)
}

// purgeLocked forgets the expired tickets.
func (h *Host) purgeLocked(now time.Time) {
	h.tickets = slices.DeleteFunc(h.tickets, func(t Ticket) bool { return t.Expired(now) })
}

// Serve answers one lobby request from peer from, for p2p.Node.HandleLobby. It
// returns false to give no answer at all (the stream is reset): for a request
// that does not decode, for Hello and Ask while the host is not visible, and
// when ctx ends while a request waits.
func (h *Host) Serve(ctx context.Context, from transport.PeerID, req []byte) ([]byte, bool) {
	r, err := DecodeRequest(req)
	if err != nil {
		return nil, false
	}
	switch r.Kind {
	case KindHello:
		if !h.Visible() {
			return nil, false
		}
		return Reply{Kind: ReplyHello, Machine: h.cfg.Machine, Title: genesis(h.cfg.List.Since(nil)).Name}.Encode(), true
	case KindAsk:
		if !h.Visible() {
			return nil, false
		}
		return h.ask(ctx, from, r.Name)
	case KindRedeem:
		return h.redeem(from, r), true
	}
	return nil, false
}

func refuse(why Reason) []byte { return Reply{Kind: ReplyRefused, Reason: why}.Encode() }

// redeem handles a Redeem request.
func (h *Host) redeem(from transport.PeerID, r Request) []byte {
	key, err := h.cfg.KeyOf(from)
	if err != nil || identity.ValidatePublicKey(key) != nil || r.Project != h.cfg.List.Project() {
		return refuse(ReasonInvalid)
	}
	t, ok := h.take(r.Secret)
	if !ok {
		return refuse(ReasonInvalid)
	}
	// The window may close with the ticket, and closing it drops the strangers
	// that are not members: that must wait until the requester is one.
	defer h.opened()
	return h.admit(key, t.Role, t.Person, r.Name)
}

// take removes and returns the unexpired ticket whose secret is secret. The
// secret is compared with every ticket, in constant time.
func (h *Host) take(secret [16]byte) (Ticket, bool) {
	now := h.cfg.Now()
	h.mu.Lock()
	h.purgeLocked(now)
	found := -1
	for i, t := range h.tickets {
		if subtle.ConstantTimeCompare(t.Secret[:], secret[:]) == 1 {
			found = i
		}
	}
	var t Ticket
	if found >= 0 {
		t = h.tickets[found]
		h.tickets = slices.Delete(h.tickets, found, found+1)
	}
	h.mu.Unlock()
	return t, found >= 0
}

// ask handles an Ask request: it waits for a person to answer.
func (h *Host) ask(ctx context.Context, from transport.PeerID, name string) ([]byte, bool) {
	key, err := h.cfg.KeyOf(from)
	if err != nil || identity.ValidatePublicKey(key) != nil {
		return nil, false
	}
	if h.cfg.List.Role(key) > 0 {
		return h.admit(key, 0, nil, name), true // already a member: just the project
	}
	a := h.addAsk(from, key, name)
	if a == nil {
		return refuse(ReasonBusy), true
	}
	defer func() {
		h.mu.Lock()
		delete(h.asks, a.ID)
		h.mu.Unlock()
	}()
	timer := time.NewTimer(AskTimeout)
	defer timer.Stop()
	select {
	case d := <-a.answer:
		if !d.accept {
			return refuse(ReasonDeclined), true
		}
		return h.admit(key, d.role, nil, name), true
	case <-timer.C:
		return refuse(ReasonDeclined), true
	case <-ctx.Done():
		return nil, false
	}
}

// addAsk registers a waiting request, or returns nil if there are too many or
// if from already has one.
func (h *Host) addAsk(from transport.PeerID, key ed25519.PublicKey, name string) *pendingAsk {
	fp := identity.Fingerprint(key, h.cfg.Lang)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.asks) >= maxAsks {
		return nil
	}
	for _, a := range h.asks {
		if a.Peer == from {
			return nil
		}
	}
	h.nextID++
	a := &pendingAsk{
		Pending: Pending{ID: h.nextID, Peer: from, Name: identity.CleanName(name), Fingerprint: strings.Join(fp[:], " · ")},
		answer:  make(chan decision, 1),
	}
	h.asks[a.ID] = a
	return a
}

// admit makes key a member with role, unless it already is one, and returns
// the Welcome reply.
func (h *Host) admit(key ed25519.PublicKey, role trust.Role, person *trust.Hash, name string) []byte {
	if h.cfg.List.Role(key) == 0 {
		tmpl := trust.Op{Kind: trust.OpAdd, Subject: key, Role: role, Name: identity.CleanName(name)}
		if person != nil {
			tmpl.Person = *person
		}
		if _, err := h.cfg.Issue(tmpl); err != nil {
			return refuse(ReasonFailed)
		}
	}
	ops := h.cfg.List.Since(nil)
	b := Reply{Kind: ReplyWelcome, RootURL: genesis(ops).RootURL, Ops: ops}.Encode()
	if len(b) > MaxReply {
		return refuse(ReasonTooLarge)
	}
	return b
}

// genesis returns the genesis op among ops, or the zero op if there is none.
func genesis(ops []trust.Op) trust.Op {
	if i := slices.IndexFunc(ops, isGenesis); i >= 0 {
		return ops[i]
	}
	return trust.Op{}
}

func isGenesis(op trust.Op) bool { return op.Kind == trust.OpGenesis }

// Lobby is how a would-be member talks to a host it is connected to; it is
// implemented by p2p.Node.
type Lobby interface {
	AskLobby(ctx context.Context, to transport.PeerID, req []byte) ([]byte, error)
}

// Joiner is the side of a device that wants to join a project.
type Joiner struct {
	// Lobby reaches the host.
	Lobby Lobby
	// KeyOf returns the key of a peer identity.
	KeyOf func(transport.PeerID) (ed25519.PublicKey, error)
	// Name is how this device wants to be called.
	Name string
	// Self is this device's key.
	Self ed25519.PublicKey
}

// Joined is a project this device was admitted to, verified.
type Joined struct {
	// List is the member list, checked up to the genesis.
	List *trust.List
	// Host is the device that admitted this one.
	Host transport.PeerID
	// RootURL is the document URL of the project, as its genesis states.
	RootURL string
}

// Hello is what a visible host says about itself.
type Hello struct {
	// Machine and Title are cleaned for display.
	Machine, Title string
}

// Hello asks the connected host who it is. A host that is not visible does
// not answer, which is an error here.
func (j Joiner) Hello(ctx context.Context, host transport.PeerID) (Hello, error) {
	rep, err := j.exchange(ctx, host, Request{Kind: KindHello})
	if err != nil {
		return Hello{}, err
	}
	if rep.Kind != ReplyHello {
		return Hello{}, fmt.Errorf("%w: unexpected reply", ErrMalformed)
	}
	return Hello{Machine: identity.CleanName(rep.Machine), Title: identity.CleanName(rep.Title)}, nil
}

// Redeem presents ticket t to its host, which must be connected, and checks
// what comes back: the member list must lead from the project's genesis (the
// project of the ticket) to a membership of this device, with the host as
// admin.
func (j Joiner) Redeem(ctx context.Context, t Ticket, now time.Time) (*Joined, error) {
	if t.Expired(now) {
		return nil, ErrExpired
	}
	rep, err := j.exchange(ctx, t.Host, Request{Kind: KindRedeem, Project: t.Project, Secret: t.Secret, Name: j.Name})
	if err != nil {
		return nil, err
	}
	return j.verify(rep, t.Host, &t.Project, t.Person)
}

// Ask asks the connected host to let this device join, and waits for a person
// of the project to answer. The project is not known in advance, so it is
// only checked to be coherent: the host's own genesis certifies it, and the
// person who accepted compared fingerprints.
func (j Joiner) Ask(ctx context.Context, host transport.PeerID) (*Joined, error) {
	rep, err := j.exchange(ctx, host, Request{Kind: KindAsk, Name: j.Name})
	if err != nil {
		return nil, err
	}
	return j.verify(rep, host, nil, nil)
}

func (j Joiner) exchange(ctx context.Context, host transport.PeerID, req Request) (Reply, error) {
	b, err := j.Lobby.AskLobby(ctx, host, req.Encode())
	if err != nil {
		return Reply{}, fmt.Errorf("invite: asking %s: %w", host, err)
	}
	rep, err := DecodeReply(b)
	if err != nil {
		return Reply{}, err
	}
	if rep.Kind == ReplyRefused {
		return Reply{}, &RefusedError{Reason: rep.Reason}
	}
	return rep, nil
}

// verify checks a Welcome and builds the member list from it.
func (j Joiner) verify(rep Reply, host transport.PeerID, project *trust.ProjectID, person *trust.Hash) (*Joined, error) {
	if rep.Kind != ReplyWelcome {
		return nil, fmt.Errorf("%w: unexpected reply", ErrMalformed)
	}
	hostKey, err := j.KeyOf(host)
	if err != nil {
		return nil, fmt.Errorf("invite: host key: %w", err)
	}
	if err := identity.ValidatePublicKey(hostKey); err != nil {
		return nil, err
	}
	i := slices.IndexFunc(rep.Ops, isGenesis)
	if i < 0 {
		return nil, fmt.Errorf("%w: no genesis", ErrBadProject)
	}
	genesis := rep.Ops[i]
	id := trust.ProjectID(genesis.Hash())
	if project != nil && id != *project {
		return nil, fmt.Errorf("%w: another project", ErrBadProject)
	}
	list := trust.New(id)
	if _, err := list.ApplyAll(rep.Ops); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadProject, err)
	}
	switch {
	case list.Pending() > 0 || list.Len() != len(rep.Ops):
		return nil, fmt.Errorf("%w: incomplete or repeated ops", ErrBadProject)
	case rep.RootURL != genesis.RootURL:
		return nil, fmt.Errorf("%w: root url", ErrBadProject)
	case list.Role(hostKey) != trust.RoleAdmin:
		return nil, fmt.Errorf("%w: the host is not an admin", ErrBadProject)
	case list.Role(j.Self) == 0:
		return nil, fmt.Errorf("%w: this device is not a member", ErrBadProject)
	}
	if person != nil {
		ok := slices.ContainsFunc(list.Members(), func(m trust.Member) bool {
			return m.Person == *person && string(m.Pub) == string(j.Self)
		})
		if !ok {
			return nil, fmt.Errorf("%w: wrong person", ErrBadProject)
		}
	}
	return &Joined{List: list, Host: host, RootURL: genesis.RootURL}, nil
}
