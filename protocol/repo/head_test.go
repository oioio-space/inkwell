// SPDX-License-Identifier: MPL-2.0

package repo

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// Spec 9.2.1 (canonical heads, RFC 8949 section 3): an unsigned integer is written in the shortest
// head that holds it. Each width is probed at its last value and at the first value of the next
// width, so a shifted bound (< vs <=) in appendHead changes the bytes and fails here.
func TestEncodeHeadUsesTheShortestWidth(t *testing.T) {
	for _, tc := range []struct {
		n    uint64
		want string
	}{
		{23, "17"},                      // last value in the initial byte
		{24, "1818"},                    // first value needing one extra byte
		{255, "18ff"},                   // last one-byte argument
		{256, "190100"},                 // first two-byte argument
		{65535, "19ffff"},               // last two-byte argument
		{65536, "1a00010000"},           // first four-byte argument
		{1<<32 - 1, "1affffffff"},       // last four-byte argument
		{1 << 32, "1b0000000100000000"}, // first eight-byte argument
		{maxSafe, "1b001fffffffffffff"}, // spec bound: 2^53-1 is accepted
	} {
		got, err := encode(nil, tc.n)
		if err != nil {
			t.Fatalf("encode(%d): %v", tc.n, err)
		}
		if want, _ := hex.DecodeString(tc.want); !bytes.Equal(got, want) {
			t.Errorf("encode(%d) = %x, want %s", tc.n, got, tc.want)
		}
	}
}

// Spec 9.2.1 and section 16: no integer above 2^53-1 is ever written (the TypeScript side could not
// read it back exactly).
func TestEncodeRefusesAnIntegerAboveTheSafeBound(t *testing.T) {
	if _, err := encode(nil, uint64(maxSafe+1)); err == nil {
		t.Fatal("encode(2^53) succeeded, want a refusal")
	}
}

// Spec 9.2.1: a text string's head follows the same shortest-width rule for its length.
func TestEncodeTextHeadAtWidthBounds(t *testing.T) {
	for _, tc := range []struct {
		length int
		head   string
	}{{23, "77"}, {24, "7818"}, {255, "78ff"}, {256, "790100"}} {
		got, err := encode(nil, string(bytes.Repeat([]byte("a"), tc.length)))
		if err != nil {
			t.Fatalf("encode(text of %d): %v", tc.length, err)
		}
		if head, _ := hex.DecodeString(tc.head); !bytes.HasPrefix(got, head) {
			t.Errorf("text of %d: head %x, want %s", tc.length, got[:len(head)], tc.head)
		}
	}
}

// Spec 9.2.1: only unsigned integers are written. A signed Go value is accepted from zero
// upward (zero is the boundary) and refused below it, for every signed width the encoder takes.
func TestEncodeSignedIntegersFromZero(t *testing.T) {
	for _, v := range []any{int(0), int64(0), int32(0)} {
		if got, err := encode(nil, v); err != nil || !bytes.Equal(got, []byte{0x00}) {
			t.Errorf("encode(%T 0) = %x, %v; want 00", v, got, err)
		}
	}
	for _, v := range []any{int(-1), int64(-1), int32(-1)} {
		if _, err := encode(nil, v); err == nil {
			t.Errorf("encode(%T -1) succeeded, want a refusal", v)
		}
	}
}

// Section 16: a decoded value holds at most 4096 items (containers count, as do their members).
// 195 arrays of 20 zeros inside one array: 1 + 195*(1+20) = 4096 items, accepted; one more zero
// makes 4097, refused.
func TestDecodeItemsBound(t *testing.T) {
	build := func(extra bool) []byte {
		b := []byte{0x98, 195} // array of 195
		for i := range 195 {
			n := 20
			if extra && i == 0 {
				n = 21
			}
			b = append(b, 0x80|byte(n)) // array of n (n < 24)
			b = append(b, make([]byte, n)...)
		}
		return b
	}
	if _, err := DecodeValue(build(false)); err != nil {
		t.Fatalf("4096 items refused: %v", err)
	}
	if _, err := DecodeValue(build(true)); err == nil || !strings.Contains(err.Error(), "4096 items") {
		t.Fatalf("4097 items: err %v, want the items bound", err)
	}
}

// Spec 9.2.1: a container that announces members the input does not hold is truncated: the decoder
// returns an error and never reads past the end (hostile peer input).
func TestDecodeTruncatedContainersFailCleanly(t *testing.T) {
	for _, in := range [][]byte{
		{0x81}, {0xa1}, {0xa1, 0x61, 'k'}, // [?], {?:?}, {"k": ?}
		{0x19, 0x01}, {0x1a, 0, 0}, {0x1b, 0, 0, 0, 0, 0, 0, 0}, // 2-, 4- and 8-byte arguments cut short
	} {
		if _, err := DecodeValue(in); err == nil {
			t.Errorf("DecodeValue(%x) succeeded, want truncated", in)
		}
	}
}
