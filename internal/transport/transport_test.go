package transport_test

import (
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/transport"
)

func TestValidPeerID(t *testing.T) {
	tests := []struct {
		name string
		id   transport.PeerID
		want bool
	}{
		{name: "empty", id: "", want: false},
		{name: "too long", id: transport.PeerID(strings.Repeat("a", 65)), want: false},
		{name: "max length", id: transport.PeerID(strings.Repeat("a", 64)), want: true},
		{name: "control byte", id: "a\x00", want: false},
		{name: "space", id: "a b", want: false},
		{name: "non ascii", id: "é", want: false},
		{name: "base32", id: "k7f3", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transport.ValidPeerID(tt.id); got != tt.want {
				t.Errorf("ValidPeerID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	for k, want := range map[transport.Kind]string{
		transport.PeerUp: "PeerUp", transport.PeerDown: "PeerDown", transport.Message: "Message", 0: "Kind(?)",
	} {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
