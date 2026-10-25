package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

const (
	// joinWindow bounds how long this device lets a stranger connect while it
	// joins: the host it contacts is a stranger to it until it is admitted.
	joinWindow = 2 * time.Minute
	// redeemTimeout bounds redeeming a ticket, from the connection to the Welcome.
	redeemTimeout = 30 * time.Second
	// codeSetupTimeout bounds how long Invite waits for a Wormhole code.
	codeSetupTimeout = 8 * time.Second
	// fetchTimeout bounds how long Join waits for a ticket behind a code.
	fetchTimeout = 30 * time.Second
	// probeTimeout bounds the greeting of one nearby device.
	probeTimeout = 4 * time.Second
	// maxProbes bounds how many nearby devices are greeted at once.
	maxProbes = 8
	// fingerprintLang is the language of the words shown to compare keys.
	fingerprintLang = "fr"
)

// carrier carries a ticket as a short code; [invite.Wormhole] implements it.
type carrier interface {
	Offer(ctx context.Context, t invite.Ticket) (code string, done <-chan error, err error)
	Fetch(ctx context.Context, code string) (invite.Ticket, error)
}

// lobbyNode is what the invitations need from the network node.
type lobbyNode interface {
	Network
	HandleLobby(h p2p.LobbyHandler)
	AskLobby(ctx context.Context, to transport.PeerID, req []byte) ([]byte, error)
	SetOpen(until time.Time)
	Nearby() []p2p.Nearby
	Drop(who transport.PeerID)
}

// Invitation is what [Session.Invite] returns: the two ways to give a ticket.
type Invitation struct {
	// Expires is when the invitation stops working.
	Expires time.Time `json:"expires"`
	// Code is the short code to say aloud or type, empty if it could not be
	// made (see CodeError).
	Code string `json:"code"`
	// Text is the invitation to copy and paste where there is no Internet.
	Text string `json:"text"`
	// CodeError says why there is no Code.
	CodeError string `json:"codeError,omitzero"`
}

// NearbyPerson is a visible device found on the local network.
type NearbyPerson struct {
	// Peer identifies the device, for [Session.Ask].
	Peer string `json:"peer"`
	// Machine is the name of the machine.
	Machine string `json:"machine"`
	// Title is the title of the project it offers.
	Title string `json:"title"`
}

// JoinRequest is a request to join that waits for an answer.
type JoinRequest struct {
	// ID names the request in [Session.Answer].
	ID int `json:"id"`
	// Peer identifies the device.
	Peer string `json:"peer"`
	// Name is the name the person gave.
	Name string `json:"name"`
	// Fingerprint is the four words to compare aloud with the person.
	Fingerprint string `json:"fingerprint"`
}

// invites runs the invitations of a runtime: the tickets this device issues
// and serves, and the projects it joins. The project it works on changes when
// it joins another one.
type invites struct {
	node    lobbyNode
	wiring  *Wiring
	dev     *identity.Device
	carrier carrier
	log     *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	now     func() time.Time
	store   *opStore
	list    *trust.List
	host    *invite.Host
	machine string
	wg      sync.WaitGroup

	// joining serializes the joins.
	joining sync.Mutex

	// mu guards the fields below and every call to node.SetOpen, so that the
	// last window computed is the one set.
	mu        sync.Mutex
	hostUntil time.Time
	joinUntil time.Time
	joins     int
}

// newInvites starts the invitations of the project of list, stored in store.
// store is closed by [invites.close].
func newInvites(ctx context.Context, node lobbyNode, w *Wiring, dev *identity.Device, list *trust.List, store *opStore, c carrier, log *slog.Logger) *invites {
	ctx, cancel := context.WithCancel(ctx)
	inv := &invites{
		node: node, wiring: w, dev: dev, carrier: c, log: log,
		ctx: ctx, cancel: cancel, now: time.Now, store: store, list: list,
		machine: identity.MachineName(),
	}
	inv.host = inv.newHost(list)
	node.HandleLobby(inv.serve)
	return inv
}

func (inv *invites) newHost(list *trust.List) *invite.Host {
	return invite.NewHost(invite.HostConfig{
		List:    list,
		Issue:   inv.wiring.IssueWith,
		KeyOf:   p2p.KeyFromID,
		Addrs:   inv.addrs,
		OnOpen:  inv.hostOpen,
		Now:     inv.now,
		Self:    inv.node.Local(),
		SelfKey: inv.dev.Public(),
		Machine: inv.machine,
		Lang:    fingerprintLang,
	})
}

func (inv *invites) currentHost() *invite.Host {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return inv.host
}

func (inv *invites) currentList() *trust.List {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return inv.list
}

// serve is the lobby handler of the node.
func (inv *invites) serve(ctx context.Context, from transport.PeerID, req []byte) ([]byte, bool) {
	return inv.currentHost().Serve(ctx, from, req)
}

// hostOpen records until when the host serves strangers.
func (inv *invites) hostOpen(until time.Time) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	inv.hostUntil = until
	inv.node.SetOpen(inv.windowLocked())
}

// windowLocked returns until when strangers may connect.
func (inv *invites) windowLocked() time.Time {
	if inv.hostUntil.After(inv.joinUntil) {
		return inv.hostUntil
	}
	return inv.joinUntil
}

// openForJoin lets the device it contacts connect, until the returned function
// is called. The window also ends by itself after joinWindow.
func (inv *invites) openForJoin() (closeWindow func()) {
	inv.mu.Lock()
	inv.joins++
	inv.joinUntil = inv.now().Add(joinWindow)
	inv.node.SetOpen(inv.windowLocked())
	inv.mu.Unlock()
	return func() {
		inv.mu.Lock()
		defer inv.mu.Unlock()
		if inv.joins--; inv.joins == 0 {
			inv.joinUntil = time.Time{}
		}
		inv.node.SetOpen(inv.windowLocked())
	}
}

// addrs returns the addresses to put in a ticket, the most useful first: on
// the network (QUIC before TCP), then on this machine, then through a relay.
func (inv *invites) addrs() []string {
	addrs := slices.Clone(inv.node.Addrs())
	rank := func(a string) int {
		r := 0
		switch {
		case strings.Contains(a, "/p2p-circuit"):
			r += 4
		case strings.HasPrefix(a, "/ip4/127.") || strings.HasPrefix(a, "/ip6/::1/"):
			r += 2
		}
		if !strings.Contains(a, "/quic-v1") {
			r++
		}
		return r
	}
	slices.SortStableFunc(addrs, func(a, b string) int { return cmp.Compare(rank(a), rank(b)) })
	return slices.Compact(addrs)
}

// invite issues a ticket and offers it as a code.
func (inv *invites) invite(role trust.Role, person *trust.Hash) (Invitation, error) {
	t, err := inv.currentHost().Invite(role, person)
	if err != nil {
		return Invitation{}, fmt.Errorf("invite: %w", err)
	}
	out := Invitation{Expires: t.Expires, Text: t.Text()}
	if inv.carrier == nil {
		out.CodeError = "codes are turned off"
		return out, nil
	}
	code, err := inv.offer(t)
	if err != nil {
		out.CodeError = err.Error()
		return out, nil
	}
	out.Code = code
	return out, nil
}

// offer publishes t as a code, which stays valid until t expires or is
// received.
func (inv *invites) offer(t invite.Ticket) (string, error) {
	ctx, cancel := context.WithDeadline(inv.ctx, t.Expires)
	type result struct {
		done <-chan error
		err  error
		code string
	}
	res := make(chan result, 1)
	inv.wg.Go(func() {
		code, done, err := inv.carrier.Offer(ctx, t)
		res <- result{done, err, code}
		if err != nil {
			cancel()
			return
		}
		if err := <-done; err != nil { // nil once received
			inv.log.Debug("code offer ended", "err", err)
		}
		cancel()
	})
	select {
	case r := <-res:
		return r.code, r.err
	case <-time.After(codeSetupTimeout):
		cancel()
		return "", errors.New("the code server did not answer")
	}
}

// ticket reads what a person gave to join: an invitation text or a code.
func (inv *invites) ticket(ctx context.Context, codeOrText string) (invite.Ticket, error) {
	s := strings.TrimSpace(codeOrText)
	if !invite.LooksLikeCode(s) {
		return invite.ParseText(s)
	}
	if inv.carrier == nil {
		return invite.Ticket{}, errors.New("codes are turned off")
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	return inv.carrier.Fetch(ctx, s)
}

// join redeems the ticket and moves to its project.
func (inv *invites) join(ctx context.Context, codeOrText string) (transport.PeerID, error) {
	t, err := inv.ticket(ctx, codeOrText)
	if err != nil {
		return "", err
	}
	inv.joining.Lock()
	defer inv.joining.Unlock()
	switch {
	case t.Host == inv.node.Local():
		return "", fmt.Errorf("%w: this is your own invitation", invite.ErrMalformed)
	case t.Expired(inv.now()):
		return "", invite.ErrExpired
	}
	defer inv.openForJoin()()
	ctx, cancel := context.WithTimeout(ctx, redeemTimeout)
	defer cancel()
	if err := inv.node.Connect(ctx, t.Host, t.Addrs); err != nil {
		return "", fmt.Errorf("join: %w", err)
	}
	joined, err := inv.joiner(identity.MachineName()).Redeem(ctx, t, inv.now())
	if err != nil {
		inv.node.Drop(t.Host)
		return "", fmt.Errorf("join: %w", err)
	}
	if err := inv.adopt(joined); err != nil {
		inv.node.Drop(t.Host)
		return "", err
	}
	return t.Host, nil
}

// ask asks the nearby device peer, which is visible, to let this one join,
// and waits for a person to answer.
func (inv *invites) ask(ctx context.Context, peer transport.PeerID, name string) (transport.PeerID, error) {
	nearby := inv.node.Nearby()
	i := slices.IndexFunc(nearby, func(n p2p.Nearby) bool { return n.ID == peer })
	if i < 0 {
		return "", errors.New("join: this device is not nearby anymore")
	}
	addrs := nearby[i].Addrs
	inv.joining.Lock()
	defer inv.joining.Unlock()
	defer inv.openForJoin()()
	ctx, cancel := context.WithTimeout(ctx, invite.AskTimeout+redeemTimeout)
	defer cancel()
	if err := inv.node.Connect(ctx, peer, addrs); err != nil {
		return "", fmt.Errorf("join: %w", err)
	}
	joined, err := inv.joiner(cmp.Or(identity.CleanName(name), identity.MachineName())).Ask(ctx, peer)
	if err != nil {
		inv.node.Drop(peer)
		return "", fmt.Errorf("join: %w", err)
	}
	if err := inv.adopt(joined); err != nil {
		inv.node.Drop(peer)
		return "", err
	}
	return peer, nil
}

func (inv *invites) joiner(name string) invite.Joiner {
	return invite.Joiner{Lobby: inv.node, KeyOf: p2p.KeyFromID, Name: name, Self: inv.dev.Public()}
}

// adopt moves this device to the project it joined: it stores its member
// list in place of the previous one, and gives it to the wiring.
func (inv *invites) adopt(j *invite.Joined) error {
	if j.List.Project() == inv.currentList().Project() {
		return nil // already there; the member lists converge by themselves
	}
	// The genesis op goes first: an interruption leaves a valid, partial list.
	if err := inv.store.replace(j.List.Since(nil)); err != nil {
		return fmt.Errorf("join: %w", err)
	}
	if err := inv.wiring.SwitchProject(j.List, inv.store.save); err != nil {
		return fmt.Errorf("join: %w", err)
	}
	inv.mu.Lock()
	inv.list = j.List
	inv.host = inv.newHost(j.List)
	inv.hostUntil = time.Time{}
	inv.mu.Unlock()
	return nil
}

// nearby greets the visible devices of the local network.
func (inv *invites) nearby(ctx context.Context) []NearbyPerson {
	found := inv.node.Nearby()
	found = found[:min(len(found), maxProbes)]
	if len(found) == 0 {
		return nil
	}
	defer inv.openForJoin()()
	var (
		mu  sync.Mutex
		out []NearbyPerson
		wg  sync.WaitGroup
	)
	for _, n := range found {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			if err := inv.node.Connect(ctx, n.ID, n.Addrs); err != nil {
				return
			}
			h, err := inv.joiner("").Hello(ctx, n.ID)
			if err != nil {
				inv.node.Drop(n.ID) // not visible, or not an inkwell that answers
				return
			}
			mu.Lock()
			out = append(out, NearbyPerson{Peer: string(n.ID), Machine: h.Machine, Title: h.Title})
			mu.Unlock()
		})
	}
	wg.Wait()
	slices.SortFunc(out, func(a, b NearbyPerson) int { return cmp.Compare(a.Peer, b.Peer) })
	return out
}

// myRoleAndPerson returns this device's role and person in the project.
func (inv *invites) myRoleAndPerson() (trust.Role, trust.Hash, error) {
	for _, m := range inv.currentList().Members() {
		if m.Pub.Equal(inv.dev.Public()) {
			return m.Role, m.Person, nil
		}
	}
	return 0, trust.Hash{}, trust.ErrNotAdmin
}

// close stops the invitations and releases what they hold.
func (inv *invites) close() error {
	inv.node.HandleLobby(nil)
	inv.cancel()
	inv.wg.Wait()
	return inv.store.close()
}
