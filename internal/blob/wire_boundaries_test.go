package blob_test

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/blob"
)

const (
	maxBaoForTest    = blob.MaxSlice + 16<<10 // the largest slice encoding Decode takes
	maxOffsetForTest = 1 << 40
)

// TestWireLimitsAreExact: each bound of the frame format accepts its limit and
// refuses one step beyond, in both directions (Encode and Decode agree).
func TestWireLimitsAreExact(t *testing.T) {
	h := hashOfByte(3)
	for _, tc := range []struct {
		name string
		msg  blob.Message
		ok   bool
	}{
		{"want of one group", blob.Want{Hash: h, Length: blob.GroupSize}, true},
		{"want of MaxSlice", blob.Want{Hash: h, Length: blob.MaxSlice}, true},
		{"want of MaxSlice plus a group", blob.Want{Hash: h, Length: blob.MaxSlice + blob.GroupSize}, false},
		{"want of zero", blob.Want{Hash: h}, false},
		{"want of half a group", blob.Want{Hash: h, Length: blob.GroupSize / 2}, false},
		{"want at the last allowed offset", blob.Want{Hash: h, Offset: maxOffsetForTest, Length: blob.GroupSize}, true},
		{"want one group past the last offset", blob.Want{Hash: h, Offset: maxOffsetForTest + blob.GroupSize, Length: blob.GroupSize}, false},
		{"want at an unaligned offset", blob.Want{Hash: h, Offset: blob.GroupSize + 1, Length: blob.GroupSize}, false},

		{"slice of one byte", blob.Slice{Hash: h, Length: 1, Bao: make([]byte, 8+1)}, true},
		{"slice with an unaligned length", blob.Slice{Hash: h, Length: blob.GroupSize + 7, Bao: make([]byte, 8+blob.GroupSize+7)}, true},
		{"slice of MaxSlice", blob.Slice{Hash: h, Length: blob.MaxSlice, Bao: make([]byte, 8+blob.MaxSlice)}, true},
		{"slice of MaxSlice plus one", blob.Slice{Hash: h, Length: blob.MaxSlice + 1, Bao: make([]byte, 8+blob.MaxSlice+1)}, false},
		{"slice with exactly its data", blob.Slice{Hash: h, Length: 100, Bao: make([]byte, 8+100)}, true},
		{"slice one byte short", blob.Slice{Hash: h, Length: 100, Bao: make([]byte, 8+100-1)}, false},
		{"slice with the largest encoding", blob.Slice{Hash: h, Length: blob.MaxSlice, Bao: make([]byte, maxBaoForTest)}, true},
		{"slice with an encoding one byte past", blob.Slice{Hash: h, Length: blob.MaxSlice, Bao: make([]byte, maxBaoForTest+1)}, false},
		{"slice at the last allowed offset", blob.Slice{Hash: h, Offset: maxOffsetForTest, Length: 1, Bao: make([]byte, 9)}, true},
		{"slice past the last offset", blob.Slice{Hash: h, Offset: maxOffsetForTest + blob.GroupSize, Length: 1, Bao: make([]byte, 9)}, false},
		{"slice at an unaligned offset", blob.Slice{Hash: h, Offset: 5, Length: 1, Bao: make([]byte, 9)}, false},
		{"slice of zero length", blob.Slice{Hash: h, Length: 0, Bao: make([]byte, 8)}, false},

		{"busy of zero", blob.Busy{Hash: h}, true},
		{"busy of one millisecond", blob.Busy{Hash: h, Retry: time.Millisecond}, true},
		{"busy of a minute", blob.Busy{Hash: h, Retry: time.Minute}, true},
		{"busy of a minute and a millisecond", blob.Busy{Hash: h, Retry: time.Minute + time.Millisecond}, false},
		{"busy of a fraction of a millisecond", blob.Busy{Hash: h, Retry: time.Millisecond + time.Microsecond}, false},
		{"busy in the past", blob.Busy{Hash: h, Retry: -time.Millisecond}, false},
		{"missing", blob.Missing{Hash: h}, true},
	} {
		enc, err := blob.Encode(tc.msg)
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, blob.ErrMalformed)) {
			t.Errorf("%s: Encode error = %v, want accepted=%t", tc.name, err, tc.ok)
			continue
		}
		if !tc.ok {
			continue
		}
		back, err := blob.Decode(enc)
		if err != nil || !cmp.Equal(back, tc.msg) {
			t.Errorf("%s: Decode(Encode(m)) = %+v, %v; want m back", tc.name, back, err)
		}
	}
}

// TestDecodeLimitsOnTheWire builds the frames by hand, as a hostile peer
// would, to hit each limit without going through Encode's own checks.
func TestDecodeLimitsOnTheWire(t *testing.T) {
	h := hashOfByte(4)
	busy := func(ms uint32) []byte {
		return binary.BigEndian.AppendUint32(append([]byte{4}, h[:]...), ms)
	}
	for _, tc := range []struct {
		ms uint32
		ok bool
	}{{0, true}, {1, true}, {60000, true}, {60001, false}, {1 << 31, false}, {1<<32 - 1, false}} {
		m, err := blob.Decode(busy(tc.ms))
		if tc.ok != (err == nil) {
			t.Errorf("Decode(busy %d ms) = %+v, %v; want accepted=%t", tc.ms, m, err, tc.ok)
		}
		if tc.ok && m.(blob.Busy).Retry != time.Duration(tc.ms)*time.Millisecond {
			t.Errorf("Decode(busy %d ms).Retry = %v", tc.ms, m.(blob.Busy).Retry)
		}
	}

	slice := func(offset uint64, length uint32, bao int) []byte {
		b := append([]byte{2}, h[:]...)
		b = binary.BigEndian.AppendUint64(b, offset)
		b = binary.BigEndian.AppendUint32(b, length)
		return append(b, make([]byte, bao)...)
	}
	for _, tc := range []struct {
		name string
		wire []byte
		ok   bool
	}{
		{"exact", slice(0, 10, 18), true},
		{"one short", slice(0, 10, 17), false},
		{"the header only", slice(0, 10, 0), false},
		{"largest", slice(0, blob.MaxSlice, maxBaoForTest), true},
		{"one over the largest", slice(0, blob.MaxSlice, maxBaoForTest+1), false},
		{"length 2^32-1", slice(0, 1<<32-1, 100), false},
		{"offset 2^64-GroupSize", slice(1<<64-blob.GroupSize, 1, 9), false},
	} {
		if _, err := blob.Decode(tc.wire); tc.ok != (err == nil) {
			t.Errorf("Decode(slice %s) = %v, want accepted=%t", tc.name, err, tc.ok)
		}
	}

	// A frame of one byte: the kind alone is never enough.
	for kind := range byte(7) {
		if m, err := blob.Decode([]byte{kind}); !errors.Is(err, blob.ErrMalformed) {
			t.Errorf("Decode(kind %d alone) = %+v, %v; want ErrMalformed", kind, m, err)
		}
	}
}
