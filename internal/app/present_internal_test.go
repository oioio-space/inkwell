package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRelayBoundsAWindowQueueByBytes(t *testing.T) {
	h := NewPresent(nil)
	from := &presentConn{out: make(chan []byte, presentQueue)}
	slow := &presentConn{out: make(chan []byte, presentQueue)} // nobody drains it
	h.conns[from], h.conns[slow] = struct{}{}, struct{}{}
	frame := []byte(`{"type":"page","pad":"` + strings.Repeat("x", 1<<20) + `"}`)
	for range 100 {
		h.relay(from, frame)
	}
	if got := slow.queued.Load(); got > maxPresentQueueBytes {
		t.Errorf("queued %d bytes, over the bound %d", got, maxPresentQueueBytes)
	}
	if n := len(slow.out); n < 20 || n > 33 {
		t.Errorf("%d frames queued, want about 32 (the bound is on bytes, not frames)", n)
	}
}

func TestRelayGivesTheOnlyRecipientTheFrameItself(t *testing.T) {
	h := NewPresent(nil)
	from := &presentConn{out: make(chan []byte, presentQueue)}
	to := &presentConn{out: make(chan []byte, presentQueue)}
	h.conns[from], h.conns[to] = struct{}{}, struct{}{}
	frame := []byte(`{"type":"state"}`)
	h.relay(from, frame)
	if got := <-to.out; &got[0] != &frame[0] {
		t.Error("a single recipient got a copy: the frame is the hub's to hand over")
	}
}

func FuzzCheckPresentFrame(f *testing.F) {
	f.Add([]byte(`{"type":"state","slide":3}`))
	f.Add([]byte(`{"type":"roster","windows":2}`))
	f.Add([]byte(`{"type":"a b"}`))
	f.Add([]byte(`[1,2]`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if checkPresentFrame(b) != nil {
			return
		}
		// Whatever passes is a JSON object with a short lowercase type.
		var head struct{ Type string }
		if err := json.Unmarshal(b, &head); err != nil {
			t.Fatalf("accepted %q, which does not decode: %v", b, err)
		}
		if n := len(head.Type); n == 0 || n > maxPresentType {
			t.Fatalf("accepted %q with a type of %d bytes", b, n)
		}
		for _, c := range []byte(head.Type) {
			if c < 'a' || c > 'z' {
				t.Fatalf("accepted %q with type %q", b, head.Type)
			}
		}
	})
}
