package invite_test

import (
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

const testHost transport.PeerID = "12D3KooWExampleHostPeerIdentityForTicketTests1234"

func sampleTicket() invite.Ticket {
	person := trust.Hash(sha256.Sum256([]byte("person")))
	return invite.Ticket{
		Project: trust.ProjectID(sha256.Sum256([]byte("project"))),
		Host:    testHost,
		Addrs: []string{
			"/ip4/192.168.1.20/udp/7419/quic-v1/p2p/" + string(testHost),
			"/ip4/192.168.1.20/tcp/7419/p2p/" + string(testHost),
		},
		Secret:  [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Role:    trust.RoleWriter,
		Person:  &person,
		Expires: time.Unix(1_800_000_000, 0).UTC(),
	}
}

func TestTicketTextRoundTrip(t *testing.T) {
	want := sampleTicket()
	text := want.Text()
	if !strings.HasPrefix(text, "inkwell1:") {
		t.Fatalf("Text() = %q, want the inkwell1: prefix", text)
	}
	got, err := invite.ParseText(text)
	if err != nil {
		t.Fatalf("ParseText(Text()) = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseText(Text()) mismatch (-want +got):\n%s", diff)
	}

	// Forgiving of what a paste does to text: case, wrapping, surrounding spaces.
	wrapped := "  " + strings.ToLower(text[:20]) + "\n" + text[20:] + " \n"
	if got, err := invite.ParseText(wrapped); err != nil || !cmp.Equal(want, got) {
		t.Errorf("ParseText(pasted) = %v, %v; want the same ticket", got, err)
	}

	want.Person = nil
	if got, err := invite.ParseText(want.Text()); err != nil || got.Person != nil {
		t.Errorf("ticket without person parsed as %+v, %v", got, err)
	}
}

func TestTicketTextBounded(t *testing.T) {
	tk := sampleTicket()
	for i := range 40 {
		tk.Addrs = append(tk.Addrs, "/ip4/10.0.0.1/udp/"+strings.Repeat("9", i%5+1)+
			"/quic-v1/p2p-circuit/"+strings.Repeat("x", 80)+"/p2p/"+string(testHost))
	}
	text := tk.Text()
	if len(text) > invite.MaxTextLen {
		t.Fatalf("Text() is %d bytes, want <= %d", len(text), invite.MaxTextLen)
	}
	got, err := invite.ParseText(text)
	if err != nil {
		t.Fatalf("ParseText(long ticket) = %v", err)
	}
	if len(got.Addrs) == 0 || got.Addrs[0] != tk.Addrs[0] {
		t.Errorf("addresses = %v, want the first ones kept", got.Addrs)
	}
}

func TestParseTextRejects(t *testing.T) {
	good := sampleTicket().Text()
	body := good[len("inkwell1:"):]
	for name, text := range map[string]string{
		"empty":        "",
		"no prefix":    body,
		"wrong prefix": "inkwell2:" + body,
		"bad base32":   "inkwell1:!!!!",
		"truncated":    good[:len(good)-9],
		"trailing":     good + "AAAA",
		"too long":     "inkwell1:" + strings.Repeat("A", invite.MaxTextLen),
		"multiaddr":    "/ip4/1.2.3.4/tcp/1/p2p/" + string(testHost),
	} {
		if _, err := invite.ParseText(text); err == nil {
			t.Errorf("ParseText(%s) = nil, want an error", name)
		}
	}
}

func TestTicketExpired(t *testing.T) {
	tk := sampleTicket()
	if tk.Expired(tk.Expires.Add(-time.Second)) || !tk.Expired(tk.Expires.Add(time.Second)) {
		t.Error("Expired() does not follow Expires")
	}
}

func FuzzParseText(f *testing.F) {
	f.Add(sampleTicket().Text())
	f.Add("inkwell1:")
	f.Add("")
	f.Add("inkwell1:AAAAAAAA")
	f.Fuzz(func(t *testing.T, s string) {
		tk, err := invite.ParseText(s)
		if err != nil {
			return
		}
		if text := tk.Text(); len(text) > invite.MaxTextLen {
			t.Fatalf("Text() of a parsed ticket is %d bytes", len(text))
		}
		again, err := invite.ParseText(tk.Text())
		if err != nil || !cmp.Equal(tk, again) {
			t.Fatalf("ParseText(Text()) = %+v, %v; want %+v", again, err, tk)
		}
	})
}
