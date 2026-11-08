// SPDX-License-Identifier: MPL-2.0

package repo

import (
	"bytes"
	"maps"
	"strings"
	"testing"
)

// Spec 9.2.1 item 8: depth > 8 is refused (#too-deep).
// Bound: 8, bound+1: 9.
func TestProfileRefusalDepthBounds(t *testing.T) {
	// Depth 8 (maxDepth) - should be accepted
	// 7 nested arrays + uint = depth 8 for the uint
	// [ [ [ [ [ [ [ 0 ] ] ] ] ] ] ] (7 arrays)
	depth8 := []byte{0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x00}
	_, err := DecodeValue(depth8)
	if err != nil {
		t.Fatalf("depth 8 should be accepted: %v", err)
	}

	// Depth 9 - should be refused
	// 8 nested arrays + uint = depth 9 for the uint
	depth9 := []byte{0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x00}
	_, err = DecodeValue(depth9)
	if err == nil {
		t.Fatal("depth 9 should be refused")
	}
	if !strings.Contains(err.Error(), "depth > 8") {
		t.Fatalf("expected 'depth > 8', got: %v", err)
	}
}

// Spec 9.2.1 item 8: more than 4096 items total is refused (#too-many-items).
// Bound: 4096, bound+1: 4097.
// Note: maxItems counts every CBOR item (containers + elements).
// This is tested indirectly via nested structures in vectors.
// A single array cannot exceed maxArray=256, so we test maxItems via nested structures in conformance vectors.

// Spec 9.2.1 item 8: map > 64 pairs is refused (#map-too-big).
// Bound: 64, bound+1: 65.
func TestProfileRefusalMapBounds(t *testing.T) {
	// 64 pairs - should be accepted (unique keys)
	var map64 []byte
	map64 = append(map64, 0xb8, 0x40) // map(64)
	for i := range 64 {
		keyStr := "k" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		keyBytes := []byte(keyStr)
		map64 = append(map64, byte(0x60|len(keyBytes)))
		map64 = append(map64, keyBytes...)
		map64 = append(map64, 0x00) // uint(0) value
	}
	_, err := DecodeValue(map64)
	if err != nil {
		t.Fatalf("64 pairs should be accepted: %v", err)
	}

	// 65 pairs - should be refused
	var map65 []byte
	map65 = append(map65, 0xb8, 0x41) // map(65)
	for i := range 65 {
		keyStr := "k" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		keyBytes := []byte(keyStr)
		map65 = append(map65, byte(0x60|len(keyBytes)))
		map65 = append(map65, keyBytes...)
		map65 = append(map65, 0x00)
	}
	_, err = DecodeValue(map65)
	if err == nil {
		t.Fatal("65 pairs should be refused")
	}
	if !strings.Contains(err.Error(), "map > 64") {
		t.Fatalf("expected 'map > 64', got: %v", err)
	}
}

// Spec 9.2.1 item 8: array > 256 elements is refused (#array-too-big).
// Bound: 256, bound+1: 257.
func TestProfileRefusalArrayBounds(t *testing.T) {
	// 256 elements - should be accepted
	// array(256): 0x99 0x01 0x00 (major 4, ai=25 for 2-byte length, 256=0x0100)
	arr256 := make([]byte, 3+256)
	arr256[0] = 0x99
	arr256[1] = 0x01
	arr256[2] = 0x00
	for i := range 256 {
		arr256[3+i] = 0x00
	}
	_, err := DecodeValue(arr256)
	if err != nil {
		t.Fatalf("256 elements should be accepted: %v", err)
	}

	// 257 elements - should be refused
	// array(257): 0x99 0x01 0x01 (257 = 0x0101)
	arr257 := make([]byte, 3+257)
	arr257[0] = 0x99
	arr257[1] = 0x01
	arr257[2] = 0x01
	for i := range 257 {
		arr257[3+i] = 0x00
	}
	_, err = DecodeValue(arr257)
	if err == nil {
		t.Fatal("257 elements should be refused")
	}
	if !strings.Contains(err.Error(), "array > 256") {
		t.Fatalf("expected 'array > 256', got: %v", err)
	}
}

// Spec 9.2.1 item 5: integer > 2^53-1 is refused (#big-uint).
// Bound: 2^53-1 (9007199254740991), bound+1: 2^53 (9007199254740992).
func TestProfileRefusalMaxSafeBounds(t *testing.T) {
	maxSafe := uint64(1<<53 - 1) // 9007199254740991 = 0x1FFFFFFFFFFFFF

	// maxSafe - should be accepted (encoded as 8-byte uint)
	// 0x1b followed by 8 bytes: 0x00 0x1F 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	maxSafeEnc := []byte{0x1b, 0x00, 0x1f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	_, err := DecodeValue(maxSafeEnc)
	if err != nil {
		t.Fatalf("maxSafe (%d) should be accepted: %v", maxSafe, err)
	}

	// maxSafe + 1 - should be refused
	// 0x20 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	maxSafePlus1Enc := []byte{0x1b, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = DecodeValue(maxSafePlus1Enc)
	if err == nil {
		t.Fatal("maxSafe+1 should be refused")
	}
	if !strings.Contains(err.Error(), "integer above 2^53-1") {
		t.Fatalf("expected 'integer above 2^53-1', got: %v", err)
	}
}

// Spec 9.2.1 item 5: negative integer is refused (#negative-int).
// Bound: any negative integer. Only one case needed.
func TestProfileRefusalNegativeInt(t *testing.T) {
	// negative integer: major 1, ai=0 -> 0x20
	neg := []byte{0x20}
	_, err := DecodeValue(neg)
	if err == nil {
		t.Fatal("negative integer should be refused")
	}
	if !strings.Contains(err.Error(), "negative integer") {
		t.Fatalf("expected 'negative integer', got: %v", err)
	}
}

// Spec 9.2.1 item 5: float (float16/32/64) is refused (#float).
// Bound: any float. Table-driven for all three widths.
func TestProfileRefusalFloatBounds(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"float16 0.0", []byte{0xf9, 0x00, 0x00}},
		{"float32 0.0", []byte{0xfa, 0x00, 0x00, 0x00, 0x00}},
		{"float64 0.0", []byte{0xfb, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeValue(tc.input)
			if err == nil {
				t.Fatalf("%s: float should be refused", tc.name)
			}
			if !strings.Contains(err.Error(), "float") {
				t.Fatalf("%s: expected 'float', got: %v", tc.name, err)
			}
		})
	}
}

// Spec 9.2.1 item 4: simple value other than false/true/null is refused (#simple-value).
func TestProfileRefusalSimpleValue(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"simple value 16", []byte{0xf0}},
		{"simple value 255", []byte{0xff}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeValue(tc.input)
			if err == nil {
				t.Fatalf("%s: should be refused", tc.name)
			}
			if !strings.Contains(err.Error(), "simple value other than false/true/null") {
				t.Fatalf("%s: expected 'simple value other than false/true/null', got: %v", tc.name, err)
			}
		})
	}
}

// Spec 9.2.1 item 2: indefinite length is refused (#indefinite-length).
func TestProfileRefusalIndefiniteLength(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"indefinite array", []byte{0x9f, 0x00, 0xff}},
		{"indefinite map", []byte{0xbf, 0x61, 0x61, 0x00, 0xff}},
		{"indefinite bytes", []byte{0x5f, 0x41, 0x61, 0xff}},
		{"indefinite string", []byte{0x7f, 0x61, 0x61, 0xff}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeValue(tc.input)
			if err == nil {
				t.Fatalf("%s: should be refused", tc.name)
			}
			if !strings.Contains(err.Error(), "indefinite length") {
				t.Fatalf("%s: expected 'indefinite length', got: %v", tc.name, err)
			}
		})
	}
}

// Spec 9.2.1 item 2: reserved additional info (ai=28,29,30) is refused (#reserved-ai).
func TestProfileRefusalReservedAI(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"reserved ai=28", []byte{0x1c}},
		{"reserved ai=29", []byte{0x1d}},
		{"reserved ai=30", []byte{0x1e}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeValue(tc.input)
			if err == nil {
				t.Fatalf("%s: should be refused", tc.name)
			}
			if !strings.Contains(err.Error(), "reserved additional info") {
				t.Fatalf("%s: expected 'reserved additional info', got: %v", tc.name, err)
			}
		})
	}
}

// Spec 9.2.1 item 6: map key not a text string is refused (#non-text-key).
func TestProfileRefusalNonTextKey(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"integer key", []byte{0xa1, 0x00, 0x00}},
		{"bytes key", []byte{0xa1, 0x41, 0x61, 0x00}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeValue(tc.input)
			if err == nil {
				t.Fatalf("%s: should be refused", tc.name)
			}
			if !strings.Contains(err.Error(), "map key not a text string") {
				t.Fatalf("%s: expected 'map key not a text string', got: %v", tc.name, err)
			}
		})
	}
}

// Spec 9.2.1 item 6: duplicate map key (by decoded text) is refused (#duplicate-key).
func TestProfileRefusalDuplicateKey(t *testing.T) {
	// duplicate key "a" with different encodings: 0x61 'a' and 0x78 0x01 'a'
	dupKey := []byte{0xa2, 0x61, 0x61, 0x00, 0x78, 0x01, 0x61, 0x00}
	_, err := DecodeValue(dupKey)
	if err == nil {
		t.Fatal("duplicate key should be refused")
	}
	if !strings.Contains(err.Error(), "duplicate map key") {
		t.Fatalf("expected 'duplicate map key', got: %v", err)
	}
}

// Spec 9.2.1 item 7: invalid UTF-8 is refused (#bad-utf8).
func TestProfileRefusalBadUTF8(t *testing.T) {
	// invalid UTF-8: 0xc0 0x80 (overlong encoding of NUL)
	badUTF8 := []byte{0x62, 0xc0, 0x80}
	_, err := DecodeValue(badUTF8)
	if err == nil {
		t.Fatal("invalid UTF-8 should be refused")
	}
	if !strings.Contains(err.Error(), "invalid UTF-8") && !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("expected 'invalid UTF-8', got: %v", err)
	}
}

// Spec 9.2.1 item 1: trailing bytes after element are refused (#trailing-byte).
func TestProfileRefusalTrailingBytes(t *testing.T) {
	trailing := []byte{0x00, 0x00} // uint(0) + extra byte
	_, err := DecodeValue(trailing)
	if err == nil {
		t.Fatal("trailing bytes should be refused")
	}
	if !strings.Contains(err.Error(), "trailing bytes") {
		t.Fatalf("expected 'trailing bytes', got: %v", err)
	}
}

// Spec 16.2 (webview): encode integer bounds - appendHead length bounds.
// Bound: 23 (direct), 24 (1-byte), 255, 256 (2-byte), 65535, 65536 (4-byte), 4294967295, 4294967296 (8-byte).
func TestEncodeAppendHeadBounds(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		wantHead []byte // major=0
		wantErr  bool
	}{
		{"n=0 (direct)", 0, []byte{0x00}, false},
		{"n=23 (direct max)", 23, []byte{0x17}, false},
		{"n=24 (1-byte)", 24, []byte{0x18, 0x18}, false},
		{"n=255 (1-byte max)", 255, []byte{0x18, 0xff}, false},
		{"n=256 (2-byte)", 256, []byte{0x19, 0x01, 0x00}, false},
		{"n=65535 (2-byte max)", 65535, []byte{0x19, 0xff, 0xff}, false},
		{"n=65536 (4-byte)", 65536, []byte{0x1a, 0x00, 0x01, 0x00, 0x00}, false},
		{"n=4294967295 (4-byte max)", 4294967295, []byte{0x1a, 0xff, 0xff, 0xff, 0xff}, false},
		{"n=4294967296 (8-byte)", 4294967296, []byte{0x1b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}, false},
		{"n=maxSafe (8-byte)", 1<<53 - 1, []byte{0x1b, 0x00, 0x1f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, false},
		{"n=maxSafe+1 (reject)", 1 << 53, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Encode(%d) expected error, got nil", tc.input)
				}
				if !strings.Contains(err.Error(), "integer above 2^53-1") {
					t.Fatalf("expected 'integer above 2^53-1', got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Encode(%d) unexpected error: %v", tc.input, err)
			}
			if !bytes.Equal(got, tc.wantHead) {
				t.Fatalf("Encode(%d) = %x, want %x", tc.input, got, tc.wantHead)
			}
		})
	}
}

// Spec 16.2: encode string/bytes/array/map length bounds (same width transitions).
func TestEncodeContainerLengthBounds(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(t *testing.T, got []byte)
	}{
		{
			name:  "string len 23 (direct max)",
			input: strings.Repeat("a", 23),
			check: func(t *testing.T, got []byte) {
				want := []byte{0x77} // 0x60 | 23
				want = append(want, strings.Repeat("a", 23)...)
				if !bytes.Equal(got, want) {
					t.Fatalf("string len 23: got %x want %x", got, want)
				}
			},
		},
		{
			name:  "string len 24 (1-byte)",
			input: strings.Repeat("a", 24),
			check: func(t *testing.T, got []byte) {
				want := []byte{0x78, 0x18} // 0x60 | 24, then 24
				want = append(want, strings.Repeat("a", 24)...)
				if !bytes.Equal(got, want) {
					t.Fatalf("string len 24: got %x want %x", got, want)
				}
			},
		},
		{
			name:  "bytes len 255 (1-byte max)",
			input: bytes.Repeat([]byte{0x00}, 255),
			check: func(t *testing.T, got []byte) {
				want := []byte{0x58, 0xff} // 0x40 | 24, then 255
				want = append(want, bytes.Repeat([]byte{0x00}, 255)...)
				if !bytes.Equal(got, want) {
					t.Fatalf("bytes len 255: got %x want %x", got, want)
				}
			},
		},
		{
			name:  "bytes len 256 (2-byte)",
			input: bytes.Repeat([]byte{0x00}, 256),
			check: func(t *testing.T, got []byte) {
				want := []byte{0x59, 0x01, 0x00} // 0x40 | 25, then 256
				want = append(want, bytes.Repeat([]byte{0x00}, 256)...)
				if !bytes.Equal(got, want) {
					t.Fatalf("bytes len 256: got %x want %x", got, want)
				}
			},
		},
		{
			name:  "array len 255 (1-byte max)",
			input: make([]any, 255),
			check: func(t *testing.T, got []byte) {
				if len(got) < 2 || got[0] != 0x98 || got[1] != 0xff {
					t.Fatalf("array len 255 head: got %x", got[:2])
				}
			},
		},
		{
			name:  "array len 256 (2-byte)",
			input: make([]any, 256),
			check: func(t *testing.T, got []byte) {
				if len(got) < 3 || got[0] != 0x99 || got[1] != 0x01 || got[2] != 0x00 {
					t.Fatalf("array len 256 head: got %x", got[:3])
				}
			},
		},
		{
			name: "map len 63 (1-byte max for map)",
			input: func() map[string]any {
				m := make(map[string]any, 63)
				for i := range 63 {
					m[string(rune('a'+i))] = uint64(0)
				}
				return m
			}(),
			check: func(t *testing.T, got []byte) {
				if got[0] != 0xb8 || got[1] != 0x3f { // 0xa0 | 24, then 63
					t.Fatalf("map len 63 head: got %x", got[:2])
				}
			},
		},
		{
			name: "map len 64 (1-byte max for profile maxMap)",
			input: func() map[string]any {
				m := make(map[string]any, 64)
				for i := range 64 {
					m[string(rune('a'+i))] = uint64(0)
				}
				return m
			}(),
			check: func(t *testing.T, got []byte) {
				if got[0] != 0xb8 || got[1] != 0x40 {
					t.Fatalf("map len 64 head: got %x", got[:2])
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.input)
			if err != nil {
				t.Fatalf("Encode %s: %v", tc.name, err)
			}
			tc.check(t, got)
		})
	}
}

// Spec 9.6: history-request have array ≤ 256 (maxArray).
// Bound: 256, bound+1: 257.
// Vector: repo.json#history-request.
func TestMessageHistoryRequestHaveBounds(t *testing.T) {
	ctx := testCtx()

	// 256 hashes - should be accepted
	have256 := make([]any, 256)
	for i := range have256 {
		have256[i] = make([]byte, 32)
	}
	m256 := map[string]any{
		"type":       "history-request",
		"have":       have256,
		"max":        uint64(HistoryMax),
		"senderId":   ctx.From,
		"targetId":   ctx.Me,
		"documentId": ctx.Admitted[0],
	}
	_, err := decodeHistoryRequest(m256, Message{})
	if err != nil {
		t.Fatalf("have 256 should be accepted: %v", err)
	}

	// 257 hashes - should be refused
	have257 := make([]any, 257)
	for i := range have257 {
		have257[i] = make([]byte, 32)
	}
	m257 := map[string]any{
		"type":       "history-request",
		"have":       have257,
		"max":        uint64(HistoryMax),
		"senderId":   ctx.From,
		"targetId":   ctx.Me,
		"documentId": ctx.Admitted[0],
	}
	_, err = decodeHistoryRequest(m257, Message{})
	if err == nil {
		t.Fatal("have 257 should be refused")
	}
	if !strings.Contains(err.Error(), "have") {
		t.Fatalf("expected 'have' error, got: %v", err)
	}
}

// Spec 9.6: history-page changes array ≤ 256 (maxArray).
// Bound: 256, bound+1: 257.
func TestMessageHistoryPageChangesBounds(t *testing.T) {
	ctx := testCtx()

	// 256 changes - should be accepted
	changes256 := make([]any, 256)
	for i := range changes256 {
		changes256[i] = []byte{0x01} // minimal change
	}
	m256 := map[string]any{
		"type":       "history-page",
		"changes":    changes256,
		"last":       true,
		"senderId":   ctx.From,
		"targetId":   ctx.Me,
		"documentId": ctx.Admitted[0],
	}
	_, err := decodeHistoryPage(m256, Message{})
	if err != nil {
		t.Fatalf("changes 256 should be accepted: %v", err)
	}

	// 257 changes - should be refused
	changes257 := make([]any, 257)
	for i := range changes257 {
		changes257[i] = []byte{0x01}
	}
	m257 := map[string]any{
		"type":       "history-page",
		"changes":    changes257,
		"last":       true,
		"senderId":   ctx.From,
		"targetId":   ctx.Me,
		"documentId": ctx.Admitted[0],
	}
	_, err = decodeHistoryPage(m257, Message{})
	if err == nil {
		t.Fatal("changes 257 should be refused")
	}
	if !strings.Contains(err.Error(), "changes") {
		t.Fatalf("expected 'changes' error, got: %v", err)
	}

	// Empty changes with last=false - should be refused
	mEmpty := map[string]any{
		"type":       "history-page",
		"changes":    []any{},
		"last":       false,
		"senderId":   ctx.From,
		"targetId":   ctx.Me,
		"documentId": ctx.Admitted[0],
	}
	_, err = decodeHistoryPage(mEmpty, Message{})
	if err == nil {
		t.Fatal("empty changes with last=false should be refused")
	}
	if !strings.Contains(err.Error(), "empty page must be the last") {
		t.Fatalf("expected 'empty page must be the last', got: %v", err)
	}
}

// Spec 9.5: ephemeral data ≤ 1024 bytes.
// Bound: 1024, bound+1: 1025.
// Vector: repo.json#ephemeral-too-big
// The vector only probes 1025; this pins the exact bound (1024 passes the size check).
func TestMessageEphemeralDataBounds(t *testing.T) {
	const tooBig = "ephemeral data > 1024 bytes"
	for _, tc := range []struct {
		size    int
		refused bool
	}{{1024, false}, {1025, true}} {
		m := envelope("ephemeral", map[string]any{
			"sessionId": "s1", "count": uint64(1), "data": make([]byte, tc.size),
		})
		_, err := decodeEphemeral(m, Message{})
		if got := err != nil && strings.Contains(err.Error(), tooBig); got != tc.refused {
			t.Errorf("data of %d bytes: refused for size = %v (err %v), want %v", tc.size, got, err, tc.refused)
		}
	}
}

// Spec 9.2.3 and section 16: an arrive or welcome message is at most 1024 bytes.
func TestMessageHandshakeSizeBound(t *testing.T) {
	const tooBig = "handshake > 1024 bytes"
	for _, tc := range []struct {
		size    int
		refused bool
	}{{1024, false}, {1025, true}} {
		_, err := handshake(envelope("arrive", nil), Message{}, tc.size)
		if got := err != nil && strings.Contains(err.Error(), tooBig); got != tc.refused {
			t.Errorf("handshake of %d bytes: refused for size = %v (err %v), want %v", tc.size, got, err, tc.refused)
		}
	}
}

// Spec 9.2.3: projectUrl ≤ 80 characters.
// Bound: 80, bound+1: 81.
func TestMessageProjectUrlBounds(t *testing.T) {
	ctx := testCtx()

	// 80 chars - should be accepted
	url80 := "automerge:" + strings.Repeat("a", 70) // 10 + 70 = 80
	m80 := map[string]any{
		"type":         "arrive",
		"senderId":     ctx.From,
		"targetId":     ctx.Me,
		"projectUrl":   url80,
		"peerMetadata": map[string]any{},
	}
	_, err := handshake(m80, Message{}, 80) // size ≈ 80
	if err != nil {
		t.Fatalf("projectUrl 80 should be accepted: %v", err)
	}

	// 81 chars - should be refused
	url81 := "automerge:" + strings.Repeat("a", 71) // 10 + 71 = 81
	m81 := map[string]any{
		"type":         "arrive",
		"senderId":     ctx.From,
		"targetId":     ctx.Me,
		"projectUrl":   url81,
		"peerMetadata": map[string]any{},
	}
	_, err = handshake(m81, Message{}, 81)
	if err == nil {
		t.Fatal("projectUrl 81 should be refused")
	}
	if !strings.Contains(err.Error(), "projectUrl") {
		t.Fatalf("expected 'projectUrl' error, got: %v", err)
	}
}

// Spec 9.5: ephemeral __presence state map ≤ 3 entries.
// Bound: 3, bound+1: 4.
func TestMessagePresenceStateBounds(t *testing.T) {
	// Test presence() function directly for state map size
	// state with 3 entries - valid
	state3 := map[string]any{
		"type": "snapshot",
		"state": map[string]any{
			"name":   "test",
			"head":   "test",
			"anchor": "test",
		},
	}
	if !presence(state3) {
		t.Fatal("state with 3 entries should be valid")
	}

	// state with 4 entries - invalid
	state4 := map[string]any{
		"type": "snapshot",
		"state": map[string]any{
			"name":   "test",
			"head":   "test",
			"anchor": "test",
			"extra":  "test",
		},
	}
	if presence(state4) {
		t.Fatal("state with 4 entries should be invalid")
	}
}

// Spec 9.5: channel name ≤ 128, anchor/head ≤ 64.
// Bound: 64/128, bound+1: 65/129.
func TestMessageChannelBounds(t *testing.T) {
	// channel() function tests
	// name: limit 128
	name128 := strings.Repeat("a", 128)
	if !channel("name", name128) {
		t.Fatal("name 128 should be valid")
	}
	name129 := name128 + "a"
	if channel("name", name129) {
		t.Fatal("name 129 should be invalid")
	}

	// anchor/head: limit 64
	anchor64 := strings.Repeat("a", 64)
	if !channel("anchor", anchor64) {
		t.Fatal("anchor 64 should be valid")
	}
	anchor65 := anchor64 + "a"
	if channel("anchor", anchor65) {
		t.Fatal("anchor 65 should be invalid")
	}
}

// Spec 9.2.1: arg() width bounds - ai 24=1byte, 25=2byte, 26=4byte, 27=8byte.
// Bound: ai=24 (1-byte), ai=25 (2-byte), ai=26 (4-byte), ai=27 (8-byte).
func TestDecoderArgWidthBounds(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expect uint64
	}{
		{"ai=24 value 24", []byte{0x18, 0x18}, 24},
		{"ai=24 max value 255", []byte{0x18, 0xff}, 255},
		{"ai=25 value 256", []byte{0x19, 0x01, 0x00}, 256},
		{"ai=25 max value 65535", []byte{0x19, 0xff, 0xff}, 65535},
		{"ai=26 value 65536", []byte{0x1a, 0x00, 0x01, 0x00, 0x00}, 65536},
		{"ai=26 max value 4294967295", []byte{0x1a, 0xff, 0xff, 0xff, 0xff}, 4294967295},
		{"ai=27 value 4294967296", []byte{0x1b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}, 4294967296},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := DecodeValue(tc.input)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if val != tc.expect {
				t.Fatalf("%s: expected %d, got %v", tc.name, tc.expect, val)
			}
		})
	}
}

// Spec 9.2.1 item 5: integer exactly at maxSafe boundary in arg().
func TestDecoderArgMaxSafeBounds(t *testing.T) {
	maxSafe := uint64(1<<53 - 1)
	// Encode maxSafe as 8-byte: 0x1b 0x00 0x1f 0xff 0xff 0xff 0xff 0xff 0xff
	maxSafeEnc := []byte{0x1b, 0x00, 0x1f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	val, err := DecodeValue(maxSafeEnc)
	if err != nil {
		t.Fatalf("maxSafe in arg(): %v", err)
	}
	if val != maxSafe {
		t.Fatalf("expected %d, got %v", maxSafe, val)
	}

	// maxSafe+1 should be refused
	maxSafePlus1Enc := []byte{0x1b, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = DecodeValue(maxSafePlus1Enc)
	if err == nil {
		t.Fatal("maxSafe+1 in arg() should be refused")
	}
	if !strings.Contains(err.Error(), "integer above 2^53-1") {
		t.Fatalf("expected 'integer above 2^53-1', got: %v", err)
	}
}

// Spec 9.2.1 item 6: duplicate key detection at all depths.
func TestDecoderDuplicateKeyAtDepth(t *testing.T) {
	// Duplicate key in nested map
	nestedDup := []byte{
		0xa1,       // map(1)
		0x61, 0x61, // key "a"
		0xa2,       // map(2) as value
		0x61, 0x62, // key "b"
		0x00,       // value 0
		0x61, 0x62, // duplicate key "b"
		0x01, // value 1
	}
	_, err := DecodeValue(nestedDup)
	if err == nil {
		t.Fatal("duplicate key in nested map should be refused")
	}
	if !strings.Contains(err.Error(), "duplicate map key") {
		t.Fatalf("expected 'duplicate map key', got: %v", err)
	}
}

// Spec 9.2.1 item 7: UTF-8 validation at all depths.
func TestDecoderUTF8AtDepth(t *testing.T) {
	// Invalid UTF-8 in nested string
	nestedBadUTF8 := []byte{
		0xa1,       // map(1)
		0x61, 0x61, // key "a"
		0x62, 0xc0, 0x80, // value: invalid UTF-8 (2 bytes)
	}
	_, err := DecodeValue(nestedBadUTF8)
	if err == nil {
		t.Fatal("invalid UTF-8 in nested string should be refused")
	}
	if !strings.Contains(err.Error(), "invalid UTF-8") && !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("expected 'invalid UTF-8', got: %v", err)
	}
}

// Spec 9.2.3: unknown key starting with '!' at any depth is refused (#compat-critical-key).
func TestDecoderCriticalKeyAtDepth(t *testing.T) {
	ctx := testCtx()
	// Critical key in nested map - this is checked by Decode(), not DecodeValue()
	nestedCritical := []byte{
		0xa1,       // map(1)
		0x61, 0x61, // key "a"
		0xa1,             // map(1) as value
		0x62, 0x21, 0x62, // key "!b" (starts with !)
		0x00, // value 0
	}
	_, err := Decode(nestedCritical, ctx)
	if err == nil {
		t.Fatal("critical key in nested map should be refused")
	}
	if !strings.Contains(err.Error(), "must-understand") {
		t.Fatalf("expected 'must-understand', got: %v", err)
	}
}

// testCtx returns a reusable Context for message tests.
func testCtx() Context {
	return Context{
		From:     "12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5pV",
		Me:       "12D3KooWDwTirQce1RRKnasT5fPVFgzXCy6SiRgSwrwPGLC7zE91",
		Admitted: []string{"4NMNnkMhL8jXrdJ9jamS58PAVdXu"},
	}
}

// envelope builds a minimal message map with the given type and extra fields.
func envelope(typ string, extra map[string]any) map[string]any {
	ctx := testCtx()
	m := map[string]any{
		"type":       typ,
		"senderId":   ctx.From,
		"targetId":   ctx.Me,
		"documentId": ctx.Admitted[0],
	}
	maps.Copy(m, extra)
	return m
}
