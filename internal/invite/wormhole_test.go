package invite_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/psanford/wormhole-william/rendezvous/rendezvousservertest"

	"github.com/oioio-space/inkwell/internal/invite"
)

func TestWormholeRoundTrip(t *testing.T) {
	srv := rendezvousservertest.NewServer()
	t.Cleanup(srv.Close)
	w := invite.Wormhole{RendezvousURL: srv.WebSocketURL()}

	ctx, cancel := withTimeout(t)
	defer cancel()
	want := sampleTicket()
	code, done, err := w.Offer(ctx, want)
	if err != nil {
		t.Fatalf("Offer() = %v", err)
	}
	if !invite.LooksLikeCode(code) {
		t.Errorf("Offer() code = %q, want a wormhole code", code)
	}
	got, err := w.Fetch(ctx, code)
	if err != nil {
		t.Fatalf("Fetch(%q) = %v", code, err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Fetch() mismatch (-want +got):\n%s", diff)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("done = %v, want nil once received", err)
		}
	case <-ctx.Done():
		t.Fatal("the offer never completed")
	}
}

func TestWormholeWrongCodeAndCancel(t *testing.T) {
	srv := rendezvousservertest.NewServer()
	t.Cleanup(srv.Close)
	w := invite.Wormhole{RendezvousURL: srv.WebSocketURL()}

	ctx, cancel := withTimeout(t)
	defer cancel()
	if _, err := w.Fetch(ctx, "not a code"); err == nil {
		t.Error("Fetch(not a code) = nil")
	}

	offerCtx, withdraw := withTimeout(t)
	_, done, err := w.Offer(offerCtx, sampleTicket())
	if err != nil {
		t.Fatal(err)
	}
	withdraw()
	select {
	case err := <-done:
		if err == nil {
			t.Error("done = nil after the offer was withdrawn")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("done never fired after cancel")
	}
}

func TestLooksLikeCode(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"7-guitarist-revenge", true},
		{" 12-a-b-c ", true},
		{"inkwell1:ABCD", false},
		{"guitarist-revenge", false},
		{"7-", false},
		{"", false},
		{"/ip4/1.2.3.4", false},
	}
	for _, tt := range tests {
		if got := invite.LooksLikeCode(tt.in); got != tt.want {
			t.Errorf("LooksLikeCode(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
