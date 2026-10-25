package app

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// startLoopbackRelay runs a relay on the loopback until the test ends and
// returns its QUIC address.
func startLoopbackRelay(t *testing.T) string {
	t.Helper()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan []string, 1)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- p2p.RunRelay(ctx, key, p2p.RelayOptions{
			Listen:  []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
			OnReady: func(addrs []string) { ready <- addrs },
		})
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("RunRelay returned %v after cancel, want nil", err)
		}
	})
	select {
	case addrs := <-ready:
		return addrs[0]
	case err := <-done:
		t.Fatalf("RunRelay: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("relay not ready")
	}
	return ""
}

// TestRelayOnlyTicketJoin: a host and a joiner that only talk through a relay
// redeem a ticket, and Invite refuses while the host has no address.
func TestRelayOnlyTicketJoin(t *testing.T) {
	t.Setenv(relaysEnv, startLoopbackRelay(t))
	t.Setenv(netEnv, relayOnly)
	alice, _ := startRuntime(t, nil, false)
	bob, _ := startRuntime(t, nil, false)

	var text string
	waitUntil(t, "an invitation with an address", func() bool {
		inv, err := alice.Session.Invite("writer")
		if err != nil {
			if !errors.Is(err, invite.ErrNoAddress) {
				t.Fatalf("Invite() = %v, want nil or ErrNoAddress", err)
			}
			return false
		}
		text = inv.Text
		return true
	})
	tk, err := invite.ParseText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(tk.Addrs) == 0 || !strings.Contains(tk.Addrs[0], "/p2p-circuit") {
		t.Fatalf("ticket addrs = %v, want a circuit address", tk.Addrs)
	}
	if _, err := bob.Session.Join(t.Context(), text); err != nil {
		t.Fatalf("Join() = %v", err)
	}
	if got := alice.role(bob); got == 0 {
		t.Errorf("role of bob in alice's list = %v, want a member", got)
	}
}
