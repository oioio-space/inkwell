package bridge_test

import (
	"bytes"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
)

// The sync point travels with the write that records the hash, and is read
// back by its own op: the webview asks for it when it opens a file.
func TestStoreServerSyncPoint(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()
	key := []string{"main.typ"}

	if got := c.call(bridge.OpReadSyncPoint, key, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("ReadSyncPoint(none) = %d, want NotFound", got.Status)
	}
	point := []byte(`{"heads":["a"],"fingerprint":1}`)
	data, err := bridge.EncodeWriteAt(point, []byte("= Hi"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.call(bridge.OpWriteProjectionAt, key, string(data)); got.Status != bridge.StatusOK {
		t.Fatalf("WriteProjectionAt = %d %s", got.Status, got.Payload)
	}
	if got := c.call(bridge.OpReadProjection, key, ""); string(got.Payload) != "= Hi" {
		t.Errorf("text after WriteProjectionAt = %q", got.Payload)
	}
	if got := c.call(bridge.OpReadSyncPoint, key, ""); got.Status != bridge.StatusOK || !bytes.Equal(got.Payload, point) {
		t.Errorf("ReadSyncPoint = %d %q, want OK %q", got.Status, got.Payload, point)
	}

	next := []byte(`{"heads":["b"],"fingerprint":2}`)
	if got := c.call(bridge.OpAcknowledge, key, string(next)); got.Status != bridge.StatusOK {
		t.Fatalf("Acknowledge with a point = %d %s", got.Status, got.Payload)
	}
	if got := c.call(bridge.OpReadSyncPoint, key, ""); !bytes.Equal(got.Payload, next) {
		t.Errorf("ReadSyncPoint after Acknowledge = %q, want %q", got.Payload, next)
	}
	if got := c.call(bridge.OpAcknowledge, key, ""); got.Status != bridge.StatusOK {
		t.Fatalf("Acknowledge = %d", got.Status)
	}
	if got := c.call(bridge.OpReadSyncPoint, key, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("ReadSyncPoint after a plain Acknowledge = %d, want NotFound", got.Status)
	}

	// A frame whose point is longer than the frame is refused, not guessed at.
	if got := c.call(bridge.OpWriteProjectionAt, key, "\xff\xffxy"); got.Status != bridge.StatusError {
		t.Errorf("WriteProjectionAt(bad framing) = %d, want Error", got.Status)
	}
}

func TestWriteAtRoundTrip(t *testing.T) {
	for _, tc := range []struct{ point, text string }{{"", ""}, {"{}", "x"}, {"", "text"}, {`{"a":1}`, ""}} {
		b, err := bridge.EncodeWriteAt([]byte(tc.point), []byte(tc.text))
		if err != nil {
			t.Fatal(err)
		}
		point, text, err := bridge.DecodeWriteAt(b)
		if err != nil || string(point) != tc.point || string(text) != tc.text {
			t.Errorf("DecodeWriteAt(EncodeWriteAt(%q, %q)) = %q, %q, %v", tc.point, tc.text, point, text, err)
		}
	}
	if _, err := bridge.EncodeWriteAt(make([]byte, 1<<16), nil); err == nil {
		t.Error("EncodeWriteAt(point of 64 KiB) succeeded")
	}
	if _, _, err := bridge.DecodeWriteAt([]byte{0}); err == nil {
		t.Error("DecodeWriteAt(truncated) succeeded")
	}
}

func TestStoreServerSetSyncPoint(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()
	key := []string{"main.typ"}
	point := `{"heads":["a"],"fingerprint":1}`
	if got := c.call(bridge.OpSetSyncPoint, key, point); got.Status != bridge.StatusOK {
		t.Fatalf("SetSyncPoint = %d %s", got.Status, got.Payload)
	}
	if got := c.call(bridge.OpReadSyncPoint, key, ""); string(got.Payload) != point {
		t.Errorf("ReadSyncPoint = %q, want %q", got.Payload, point)
	}
	if got := c.call(bridge.OpSetSyncPoint, key, ""); got.Status != bridge.StatusError {
		t.Errorf("SetSyncPoint(empty) = %d, want Error", got.Status)
	}
}
