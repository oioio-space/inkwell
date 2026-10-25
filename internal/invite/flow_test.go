package invite_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

func wantRefused(t *testing.T, err error, reason invite.Reason) {
	t.Helper()
	re, ok := errors.AsType[*invite.RefusedError](err)
	if !ok || re.Reason != reason {
		t.Errorf("error = %v, want a refusal for reason %d", err, reason)
	}
}

// TestRedeemOnceExpiresWrongProject is Review Focus 6: a replayed, expired or
// foreign ticket is refused, and a refusal consumes nothing.
func TestRedeemOnceExpiresWrongProject(t *testing.T) {
	p := newProject(t)
	bob, carol := testDevice(t), testDevice(t)

	tk, err := p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Another project, or another secret, is refused and leaves the ticket usable.
	other := tk
	other.Project[0] ^= 1
	if _, err := p.joiner(bob, "Bob").Redeem(t.Context(), other, p.clock.Now()); err == nil {
		t.Error("Redeem(ticket of another project) = nil")
	} else {
		wantRefused(t, err, invite.ReasonInvalid)
	}
	wrong := tk
	wrong.Secret[15] ^= 1
	_, err = p.joiner(bob, "Bob").Redeem(t.Context(), wrong, p.clock.Now())
	wantRefused(t, err, invite.ReasonInvalid)
	if p.list.Role(bob.Public()) != 0 {
		t.Fatal("a refused ticket made Bob a member")
	}

	// The right ticket works, once.
	joined, err := p.joiner(bob, "Bob").Redeem(t.Context(), tk, p.clock.Now())
	if err != nil {
		t.Fatalf("Redeem() = %v", err)
	}
	if got := p.list.Role(bob.Public()); got != trust.RoleWriter {
		t.Errorf("Bob's role = %v, want writer", got)
	}
	if joined.List.Project() != p.list.Project() || joined.List.Role(bob.Public()) != trust.RoleWriter ||
		joined.RootURL != "automerge:root" || joined.Host != peerOf(p.admin) {
		t.Errorf("Joined = %+v, want the project, Bob as writer, the root url and the host", joined)
	}
	_, err = p.joiner(carol, "Carol").Redeem(t.Context(), tk, p.clock.Now())
	wantRefused(t, err, invite.ReasonInvalid)
	if p.list.Role(carol.Public()) != 0 {
		t.Error("a replayed ticket made Carol a member")
	}
	if !p.openUntil().IsZero() {
		t.Errorf("window still open until %v after the only ticket was used", p.openUntil())
	}

	// An expired ticket is refused by the host even if the joiner does not check.
	tk2, err := p.host.Invite(trust.RoleReader, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.clock.Advance(invite.TicketTTL + time.Second)
	req := invite.Request{Kind: invite.KindRedeem, Project: tk2.Project, Secret: tk2.Secret, Name: "Carol"}
	b, ok := p.host.Serve(t.Context(), peerOf(carol), req.Encode())
	if !ok {
		t.Fatal("Serve(Redeem) gave no answer")
	}
	if rep, err := invite.DecodeReply(b); err != nil || rep.Kind != invite.ReplyRefused || rep.Reason != invite.ReasonInvalid {
		t.Errorf("Serve(expired) = %+v, %v; want refused", rep, err)
	}
	if _, err := p.joiner(carol, "Carol").Redeem(t.Context(), tk2, p.clock.Now()); !errors.Is(err, invite.ErrExpired) {
		t.Errorf("Redeem(expired) = %v, want ErrExpired", err)
	}
	if p.list.Role(carol.Public()) != 0 {
		t.Error("an expired ticket made Carol a member")
	}
}

func TestRedeemConcurrentlyAdmitsOne(t *testing.T) {
	p := newProject(t)
	tk, err := p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		dev := testDevice(t)
		wg.Go(func() {
			if _, err := p.joiner(dev, "x").Redeem(context.Background(), tk, p.clock.Now()); err == nil {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Errorf("%d devices redeemed one ticket, want exactly 1", wins.Load())
	}
}

func TestOnlyAdminInvites(t *testing.T) {
	p := newProject(t)
	bob := testDevice(t)
	if _, err := p.list.Issue(p.admin, trust.OpAdd, bob.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	bobHost := projectOf(t, bob, p.list)
	if _, err := bobHost.host.Invite(trust.RoleReader, nil); !errors.Is(err, trust.ErrNotAdmin) {
		t.Errorf("Invite() by a writer = %v, want ErrNotAdmin", err)
	}
	if _, err := p.host.Invite(trust.Role(9), nil); err == nil {
		t.Error("Invite(invalid role) = nil")
	}
}

// TestHelloOnlyWhileVisible: outside the window, a stranger gets nothing
// (the stream is reset); only a ticket holder is served.
func TestHelloOnlyWhileVisible(t *testing.T) {
	p := newProject(t)
	stranger := testDevice(t)
	hello := invite.Request{Kind: invite.KindHello}.Encode()
	ask := invite.Request{Kind: invite.KindAsk, Name: "Marie"}.Encode()
	serve := func(req []byte) bool {
		_, ok := p.host.Serve(t.Context(), peerOf(stranger), req)
		return ok
	}

	if serve(hello) || serve(ask) {
		t.Fatal("a host that is not visible answered Hello or Ask")
	}
	if !p.openUntil().IsZero() {
		t.Errorf("window open until %v with nothing to offer", p.openUntil())
	}

	p.host.SetVisible(true)
	if !p.host.Visible() {
		t.Fatal("Visible() = false after SetVisible(true)")
	}
	if want := p.clock.Now().Add(invite.VisibleFor); !p.openUntil().Equal(want) {
		t.Errorf("window open until %v, want %v", p.openUntil(), want)
	}
	h, err := p.joiner(stranger, "Marie").Hello(t.Context(), peerOf(p.admin))
	if err != nil || h.Machine != "portable" || h.Title != "Mon livre" {
		t.Errorf("Hello() = %+v, %v; want portable / Mon livre", h, err)
	}

	p.clock.Advance(invite.VisibleFor + time.Second)
	if p.host.Visible() || serve(hello) {
		t.Error("the host still answers after the window")
	}

	p.host.SetVisible(true)
	p.host.SetVisible(false)
	if serve(hello) {
		t.Error("the host answers after SetVisible(false)")
	}
	if !p.openUntil().IsZero() {
		t.Errorf("window open until %v after SetVisible(false)", p.openUntil())
	}

	// A ticket holder is served whether or not the host is visible, and the
	// window follows the ticket.
	tk, err := p.host.Invite(trust.RoleReader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !p.openUntil().Equal(tk.Expires) {
		t.Errorf("window open until %v, want the ticket's expiry %v", p.openUntil(), tk.Expires)
	}
	if serve(hello) {
		t.Error("an outstanding ticket made the host answer Hello")
	}
	if _, err := p.joiner(stranger, "Marie").Redeem(t.Context(), tk, p.clock.Now()); err != nil {
		t.Errorf("Redeem() while not visible = %v", err)
	}
}

func waitPending(t *testing.T, h *invite.Host, n int) []invite.Pending {
	t.Helper()
	for range 2000 {
		if got := h.Pending(); len(got) == n {
			return got
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Pending() never reached %d requests", n)
	return nil
}

func TestLanAskAcceptReject(t *testing.T) {
	p := newProject(t)
	marie, noe, zoe := testDevice(t), testDevice(t), testDevice(t)
	p.host.SetVisible(true)

	type result struct {
		joined *invite.Joined
		err    error
	}
	ask := func(j invite.Joiner) chan result {
		c := make(chan result, 1)
		go func() {
			joined, err := j.Ask(context.Background(), peerOf(p.admin))
			c <- result{joined, err}
		}()
		return c
	}

	// Accepted as a reader.
	r1 := ask(p.joiner(marie, "Marie\u202e"))
	pend := waitPending(t, p.host, 1)
	if pend[0].Peer != peerOf(marie) || pend[0].Name != "Marie" || pend[0].Fingerprint == "" {
		t.Errorf("Pending() = %+v, want Marie with a fingerprint", pend)
	}
	if err := p.host.Answer(pend[0].ID, true, trust.RoleReader); err != nil {
		t.Fatal(err)
	}
	if res := <-r1; res.err != nil || res.joined.List.Role(marie.Public()) != trust.RoleReader {
		t.Errorf("Ask() accepted = %+v, %v; want a reader", res.joined, res.err)
	}
	if p.list.Role(marie.Public()) != trust.RoleReader {
		t.Errorf("host sees Marie as %v, want reader", p.list.Role(marie.Public()))
	}

	// Refused.
	r2 := ask(p.joiner(noe, "Noé"))
	pend = waitPending(t, p.host, 1)
	if err := p.host.Answer(pend[0].ID, false, 0); err != nil {
		t.Fatal(err)
	}
	res := <-r2
	wantRefused(t, res.err, invite.ReasonDeclined)
	if p.list.Role(noe.Public()) != 0 {
		t.Error("a refused request made Noé a member")
	}
	if err := p.host.Answer(pend[0].ID, true, trust.RoleWriter); !errors.Is(err, invite.ErrNoRequest) {
		t.Errorf("Answer(finished request) = %v, want ErrNoRequest", err)
	}

	// Closing the window declines what waits.
	r3 := ask(p.joiner(zoe, "Zoé"))
	waitPending(t, p.host, 1)
	p.host.SetVisible(false)
	wantRefused(t, (<-r3).err, invite.ReasonDeclined)
	waitPending(t, p.host, 0)

	// Not visible: no request at all.
	if _, err := p.joiner(zoe, "Zoé").Ask(t.Context(), peerOf(p.admin)); err == nil {
		t.Error("Ask() on a host that is not visible = nil")
	}
}

func TestAskBounded(t *testing.T) {
	p := newProject(t)
	p.host.SetVisible(true)
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	devs := make([]transport.PeerID, 0, 12)
	for range 12 {
		devs = append(devs, peerOf(testDevice(t)))
	}
	var busy atomic.Int32
	for _, id := range devs {
		wg.Go(func() {
			req := invite.Request{Kind: invite.KindAsk, Name: "x"}.Encode()
			b, ok := p.host.Serve(ctx, id, req)
			if !ok {
				return
			}
			if rep, err := invite.DecodeReply(b); err == nil && rep.Reason == invite.ReasonBusy {
				busy.Add(1)
			}
		})
	}
	waitPending(t, p.host, 8)
	for busy.Load() < 4 { // the other four are refused at once
		time.Sleep(time.Millisecond)
	}
	// A second request from a peer that is already waiting is busy too.
	if b, ok := p.host.Serve(ctx, devs[0], invite.Request{Kind: invite.KindAsk}.Encode()); ok {
		if rep, err := invite.DecodeReply(b); err != nil || rep.Reason != invite.ReasonBusy {
			t.Errorf("second Ask from the same peer = %+v, %v; want busy", rep, err)
		}
	}
	cancel()
	wg.Wait()
}

// TestDeviceTicketKeepsPerson: a ticket for another device of a person adds
// the device to that person instead of creating a new one.
func TestDeviceTicketKeepsPerson(t *testing.T) {
	p := newProject(t)
	mine := p.list.Members()[0].Person

	tk, err := p.host.Invite(trust.RoleAdmin, &mine)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := invite.ParseText(tk.Text())
	if err != nil || parsed.Person == nil || *parsed.Person != mine {
		t.Fatalf("ParseText(Text()).Person = %v, %v; want %v", parsed.Person, err, mine)
	}
	second := testDevice(t)
	joined, err := p.joiner(second, "portable").Redeem(t.Context(), parsed, p.clock.Now())
	if err != nil {
		t.Fatalf("Redeem() = %v", err)
	}
	for name, l := range map[string]*trust.List{"host": p.list, "device": joined.List} {
		var got *trust.Member
		for _, m := range l.Members() {
			if string(m.Pub) == string(second.Public()) {
				got = &m
			}
		}
		if got == nil || got.Person != mine || got.Role != trust.RoleAdmin {
			t.Errorf("on %s, the new device is %+v; want person %x as admin", name, got, mine[:4])
		}
	}

	// A new person gets a person of their own.
	tk, err = p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	third := testDevice(t)
	if _, err := p.joiner(third, "x").Redeem(t.Context(), tk, p.clock.Now()); err != nil {
		t.Fatal(err)
	}
	for _, m := range p.list.Members() {
		if string(m.Pub) == string(third.Public()) && m.Person == mine {
			t.Error("a ticket without person put the device on an existing person")
		}
	}

	unknown := trust.Hash{1}
	if _, err := p.host.Invite(trust.RoleWriter, &unknown); !errors.Is(err, invite.ErrUnknownPerson) {
		t.Errorf("Invite(unknown person) = %v, want ErrUnknownPerson", err)
	}
}

// forged is a Lobby that answers with a fixed reply, as a dishonest host would.
type forged []byte

func (f forged) AskLobby(context.Context, transport.PeerID, []byte) ([]byte, error) { return f, nil }

func TestJoinerVerifiesProject(t *testing.T) {
	p := newProject(t)
	bob := testDevice(t)
	tk, err := p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A project that is not the ticket's, with Bob admitted and the host an admin.
	evil := newProject(t)
	if _, err := evil.list.Issue(evil.admin, trust.OpAdd, bob.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	other := invite.Reply{Kind: invite.ReplyWelcome, RootURL: "automerge:root", Ops: evil.list.Since(nil)}.Encode()
	// The right project, without Bob in it.
	without := invite.Reply{Kind: invite.ReplyWelcome, RootURL: "automerge:root", Ops: p.list.Since(nil)}.Encode()
	// The right ops with a root url the genesis does not state.
	swapped := invite.Reply{Kind: invite.ReplyWelcome, RootURL: "automerge:evil", Ops: p.list.Since(nil)}.Encode()
	// Ops missing their parents.
	var headless []trust.Op
	if _, err := p.list.Issue(p.admin, trust.OpAdd, bob.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	headless = p.list.Since(nil)[1:]
	missing := invite.Reply{Kind: invite.ReplyWelcome, RootURL: "automerge:root", Ops: headless}.Encode()

	for name, reply := range map[string][]byte{
		"another project":    other,
		"without the joiner": without,
		"root url":           swapped,
		"no genesis":         missing,
		"hello":              invite.Reply{Kind: invite.ReplyHello}.Encode(),
		"garbage":            {0xff},
	} {
		j := invite.Joiner{Lobby: forged(reply), KeyOf: keyOf, Name: "Bob", Self: bob.Public()}
		if _, err := j.Redeem(t.Context(), tk, p.clock.Now()); err == nil {
			t.Errorf("Redeem(%s) = nil, want an error", name)
		}
	}
}
