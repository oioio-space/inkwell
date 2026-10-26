package bridge_test

import (
	"errors"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/store"
)

// TestEncodeRangeFitsTheFrameExactly: a range response holds up to the frame
// limit minus the response header, byte for byte, and not one byte more.
func TestEncodeRangeFitsTheFrameExactly(t *testing.T) {
	const limit = 64<<20 - 5 // the wire limit minus the response header
	key := []string{"k", "ab"}
	perEntry := 1 + len(key) + 4 + 1 + 2 // count, lengths, data length, components "k" and "ab"
	first := store.MaxValue
	second := limit - 4 - 2*perEntry - first
	if second < 0 || second > store.MaxValue {
		t.Fatalf("test layout is off: second value of %d bytes", second)
	}
	entries := []store.Entry{{Key: key, Data: make([]byte, first)}, {Key: key, Data: make([]byte, second)}}
	enc, err := bridge.EncodeRange(entries)
	if err != nil || len(enc) != limit {
		t.Fatalf("EncodeRange(exactly the limit) = %d bytes, %v; want %d, nil", len(enc), err, limit)
	}
	back, err := bridge.DecodeRange(enc)
	if err != nil || len(back) != 2 || len(back[0].Data) != first || len(back[1].Data) != second {
		t.Fatalf("DecodeRange of the largest range = %d entries, %v", len(back), err)
	}
	entries[1].Data = make([]byte, second+1)
	if _, err := bridge.EncodeRange(entries); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeRange(one byte over) = %v, want ErrMalformed", err)
	}
	// The count counts: an entry with no data still weighs its key.
	if enc, err := bridge.EncodeRange([]store.Entry{{Key: key}}); err != nil || len(enc) != 4+perEntry {
		t.Errorf("EncodeRange(one empty entry) = %d bytes, %v; want %d", len(enc), err, 4+perEntry)
	}
}
