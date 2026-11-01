package invite_test

import (
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

func TestSmallOrderRedeemPreservesTicket(t *testing.T) {
	p := newProject(t)
	ticket, err := p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	bad := transport.PeerID("01" + strings.Repeat("00", 31))
	req := invite.Request{Kind: invite.KindRedeem, Project: ticket.Project, Secret: ticket.Secret, Name: "bad"}
	data, ok := p.host.Serve(t.Context(), bad, req.Encode())
	if !ok {
		t.Fatal("no refusal")
	}
	rep, err := invite.DecodeReply(data)
	if err != nil || rep.Kind == invite.ReplyWelcome {
		t.Fatal("accepted small-order key", err)
	}
	if len(p.list.Members()) != 1 {
		t.Fatal("changed membership")
	}
	bob := testDevice(t)
	if _, err := p.joiner(bob, "Bob").Redeem(t.Context(), ticket, p.clock.Now()); err != nil {
		t.Fatal("consumed ticket", err)
	}
	p.host.SetVisible(true)
	if _, ok := p.host.Serve(t.Context(), bad, (invite.Request{Kind: invite.KindAsk, Name: "bad"}).Encode()); ok {
		t.Fatal("accepted lobby ask")
	}
	if len(p.host.Pending()) != 0 {
		t.Fatal("queued invalid key")
	}
}

func TestSmallOrderWelcomeHost(t *testing.T) {
	p := newProject(t)
	bob := testDevice(t)
	if _, err := p.list.Issue(p.admin, trust.OpAdd, bob.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	ticket, err := p.host.Invite(trust.RoleWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	reply := invite.Reply{Kind: invite.ReplyWelcome, RootURL: "automerge:root", Ops: p.list.Since(nil)}.Encode()
	j := invite.Joiner{Lobby: forged(reply), Self: bob.Public(), KeyOf: func(transport.PeerID) (ed25519.PublicKey, error) {
		key := make([]byte, 32)
		key[0] = 1
		return key, nil
	}}
	if joined, err := j.Redeem(t.Context(), ticket, p.clock.Now()); err == nil || joined != nil {
		t.Fatal("accepted small-order host")
	}
}
