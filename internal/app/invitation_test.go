package app

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/transport"
)

func TestParseInvitation(t *testing.T) {
	const id = "12D3KooWAbc"
	a1 := "/ip4/10.0.0.2/udp/7419/quic-v1/p2p/" + id
	a2 := "/ip4/10.0.0.2/tcp/7419/p2p/" + id
	circuit := "/ip4/1.2.3.4/udp/1/quic-v1/p2p/RELAY/p2p-circuit/p2p/" + id
	tests := []struct {
		name    string
		text    string
		want    []string
		wantErr bool
	}{
		{name: "one", text: a1, want: []string{a1}},
		{name: "several, odd spacing", text: " " + a1 + "\n" + a2 + "\t", want: []string{a1, a2}},
		{name: "circuit address ends with the target", text: circuit, want: []string{circuit}},
		{name: "empty", text: "", wantErr: true},
		{name: "blank", text: "  \n", wantErr: true},
		{name: "not an address", text: "192.168.1.20:7419", wantErr: true},
		{name: "no identity", text: "/ip4/10.0.0.2/tcp/7419", wantErr: true},
		{name: "empty identity", text: "/ip4/10.0.0.2/tcp/7419/p2p/", wantErr: true},
		{name: "trailing path after identity", text: a1 + "/x", wantErr: true},
		{name: "two identities", text: a1 + " /ip4/10.0.0.3/tcp/1/p2p/other", wantErr: true},
		{name: "non ASCII", text: "/ip4/10.0.0.2/tcp/1/p2p/é", wantErr: true},
		{name: "too many", text: strings.Repeat(a1+" ", maxInvitationAddrs+1), wantErr: true},
		{name: "too long", text: strings.Repeat("a", maxInvitationLen+1), wantErr: true},
		{name: "address too long", text: "/" + strings.Repeat("a", maxAddrLen) + "/p2p/x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer, got, err := parseInvitation(tt.text)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseInvitation(%q) error = %v, wantErr %v", tt.text, err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseInvitation(%q) addrs = %v, want %v", tt.text, got, tt.want)
			}
			if err == nil && peer == "" {
				t.Errorf("parseInvitation(%q) returned no identity", tt.text)
			}
		})
	}
}

func TestInvitationRoundTrip(t *testing.T) {
	var addrs []string
	for i := range maxInvitationAddrs + 10 {
		addrs = append(addrs, "/ip4/10.0.0.1/tcp/"+strconv.Itoa(i+1)+"/p2p/ID")
	}
	peer, got, err := parseInvitation(invitation(addrs))
	if err != nil || peer != "ID" || len(got) != maxInvitationAddrs {
		t.Errorf("round trip = %q, %d addrs, %v; want ID, %d addrs, nil", peer, len(got), err, maxInvitationAddrs)
	}
}

func FuzzParseInvitation(f *testing.F) {
	for _, s := range []string{"", "/ip4/1.2.3.4/tcp/1/p2p/ID", "/p2p/a /p2p/b", "a b", "/p2p//p2p/", "\x00/p2p/x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		peer, addrs, err := parseInvitation(text)
		if err != nil {
			return
		}
		if !transport.ValidPeerID(peer) || len(addrs) == 0 || len(addrs) > maxInvitationAddrs {
			t.Fatalf("parseInvitation(%q) = %q, %d addrs, nil: invalid result", text, peer, len(addrs))
		}
		for _, a := range addrs {
			if !strings.HasSuffix(a, peerSegment+string(peer)) || a[0] != '/' {
				t.Fatalf("parseInvitation(%q): address %q does not end with the identity %q", text, a, peer)
			}
		}
	})
}
