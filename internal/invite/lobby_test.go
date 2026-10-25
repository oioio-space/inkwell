package invite_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/trust"
)

func TestLobbyRoundTrip(t *testing.T) {
	p := newProject(t)
	for _, req := range []invite.Request{
		{Kind: invite.KindHello},
		{Kind: invite.KindAsk, Name: "Marie"},
		{Kind: invite.KindRedeem, Project: p.list.Project(), Secret: [16]byte{9}, Name: "Bob"},
	} {
		got, err := invite.DecodeRequest(req.Encode())
		if err != nil || !cmp.Equal(req, got) {
			t.Errorf("DecodeRequest(%+v) = %+v, %v", req, got, err)
		}
	}
	for _, rep := range []invite.Reply{
		{Kind: invite.ReplyHello, Machine: "laptop", Title: "Mon livre"},
		{Kind: invite.ReplyWelcome, RootURL: "automerge:abc", Ops: p.list.Since(nil)},
		{Kind: invite.ReplyRefused, Reason: invite.ReasonDeclined},
	} {
		got, err := invite.DecodeReply(rep.Encode())
		if err != nil || !cmp.Equal(rep, got) {
			t.Errorf("DecodeReply(%+v) = %+v, %v", rep, got, err)
		}
	}
}

func TestLobbyDecodeRejects(t *testing.T) {
	long := bytes.Repeat([]byte{'a'}, invite.MaxRequest)
	for name, b := range map[string][]byte{
		"empty":            nil,
		"unknown kind":     {0x7f},
		"hello trailing":   {byte(invite.KindHello), 0},
		"ask truncated":    {byte(invite.KindAsk), 5, 'a'},
		"ask not UTF-8":    {byte(invite.KindAsk), 1, 0xff},
		"redeem truncated": {byte(invite.KindRedeem), 1, 2, 3},
		"too long":         append([]byte{byte(invite.KindAsk)}, long...),
	} {
		if _, err := invite.DecodeRequest(b); !errors.Is(err, invite.ErrMalformed) {
			t.Errorf("DecodeRequest(%s) = %v, want ErrMalformed", name, err)
		}
	}
	for name, b := range map[string][]byte{
		"empty":         nil,
		"unknown kind":  {0x7f},
		"no reason":     {byte(invite.ReplyRefused)},
		"bad reason":    {byte(invite.ReplyRefused), 99},
		"welcome short": {byte(invite.ReplyWelcome), 0, 0, 0, 1},
		"op garbage":    {byte(invite.ReplyWelcome), 0, 0, 0, 1, 0, 0, 0, 2, 1, 2},
		"huge op":       {byte(invite.ReplyWelcome), 0, 0, 0, 1, 0xff, 0xff, 0xff, 0xff},
		"hello bad":     {byte(invite.ReplyHello), 3, 'a'},
	} {
		if _, err := invite.DecodeReply(b); !errors.Is(err, invite.ErrMalformed) {
			t.Errorf("DecodeReply(%s) = %v, want ErrMalformed", name, err)
		}
	}
}

func FuzzLobbyDecode(f *testing.F) {
	g, _ := trust.Genesis(testDevice(f), "book", "automerge:x")
	f.Add(invite.Request{Kind: invite.KindRedeem, Secret: [16]byte{1}, Name: "Bob"}.Encode())
	f.Add(invite.Request{Kind: invite.KindAsk, Name: "Marie"}.Encode())
	f.Add(invite.Reply{Kind: invite.ReplyWelcome, RootURL: "automerge:x", Ops: []trust.Op{g}}.Encode())
	f.Add(invite.Reply{Kind: invite.ReplyHello, Machine: "m", Title: "t"}.Encode())
	f.Add([]byte{byte(invite.ReplyWelcome), 0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, b []byte) {
		if req, err := invite.DecodeRequest(b); err == nil {
			again, err := invite.DecodeRequest(req.Encode())
			if err != nil || !cmp.Equal(req, again) {
				t.Fatalf("request does not round-trip: %+v then %+v, %v", req, again, err)
			}
		}
		if rep, err := invite.DecodeReply(b); err == nil {
			again, err := invite.DecodeReply(rep.Encode())
			if err != nil || !cmp.Equal(rep, again) {
				t.Fatalf("reply does not round-trip: %+v then %+v, %v", rep, again, err)
			}
		}
	})
}
