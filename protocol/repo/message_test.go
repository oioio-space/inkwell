// SPDX-License-Identifier: MPL-2.0

package repo

import (
	"bytes"
	"strings"
	"testing"
)

// Spec 16.2: encode integers in minimal CBOR form.
func TestEncodeIntegers(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []byte
	}{
		{"uint64 zero", uint64(0), []byte{0x00}},
		{"uint64 small", uint64(23), []byte{0x17}},
		{"uint64 1-byte", uint64(24), []byte{0x18, 0x18}},
		{"uint64 2-byte", uint64(256), []byte{0x19, 0x01, 0x00}},
		{"uint64 4-byte", uint64(65536), []byte{0x1a, 0x00, 0x01, 0x00, 0x00}},
		{"uint64 8-byte", uint64(4294967296), []byte{0x1b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}},
		{"int positive", int(42), []byte{0x18, 0x2a}},
		{"int64 positive", int64(42), []byte{0x18, 0x2a}},
		{"int32 positive", int32(42), []byte{0x18, 0x2a}},
		{"uint", uint(42), []byte{0x18, 0x2a}},
		{"uint32", uint32(42), []byte{0x18, 0x2a}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.input)
			if err != nil {
				t.Fatalf("Encode(%v) error: %v", tc.input, err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("Encode(%v) = %x, want %x", tc.input, got, tc.want)
			}
		})
	}
}

// Spec 9.5 / §9.7 G1: present() validates ephemeral __present payload.
func TestPresent(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]any
		expect bool
	}{
		{
			name: "state valid",
			input: map[string]any{
				"t":      "state",
				"epoch":  uint64(1),
				"screen": "show",
				"seq":    uint64(1),
				"slide":  uint64(1),
				"step":   uint64(1),
			},
			expect: true,
		},
		{
			name: "state invalid screen",
			input: map[string]any{
				"t":      "state",
				"epoch":  uint64(1),
				"screen": "invalid",
				"seq":    uint64(1),
				"slide":  uint64(1),
				"step":   uint64(1),
			},
			expect: false,
		},
		{
			name: "state slide zero",
			input: map[string]any{
				"t":      "state",
				"epoch":  uint64(1),
				"screen": "show",
				"seq":    uint64(1),
				"slide":  uint64(0),
				"step":   uint64(1),
			},
			expect: false,
		},
		{
			name: "laser valid with x,y",
			input: map[string]any{
				"t":     "laser",
				"epoch": uint64(1),
				"x":     uint64(100),
				"y":     uint64(200),
			},
			expect: true,
		},
		{
			name: "laser valid nil x,y",
			input: map[string]any{
				"t":     "laser",
				"epoch": uint64(1),
				"x":     nil,
				"y":     nil,
			},
			expect: true,
		},
		{
			name: "laser x only",
			input: map[string]any{
				"t":     "laser",
				"epoch": uint64(1),
				"x":     uint64(100),
				"y":     nil,
			},
			expect: false,
		},
		{
			name: "laser x too large",
			input: map[string]any{
				"t":     "laser",
				"epoch": uint64(1),
				"x":     uint64(20000),
				"y":     uint64(100),
			},
			expect: false,
		},
		{
			name: "take valid",
			input: map[string]any{
				"t":     "take",
				"epoch": uint64(1),
			},
			expect: true,
		},
		{
			name: "end valid",
			input: map[string]any{
				"t":     "end",
				"epoch": uint64(1),
			},
			expect: true,
		},
		{
			name: "hello valid",
			input: map[string]any{
				"t": "hello",
			},
			expect: true,
		},
		{
			name: "ask valid",
			input: map[string]any{
				"t": "ask",
			},
			expect: true,
		},
		{
			name: "give valid",
			input: map[string]any{
				"t":     "give",
				"epoch": uint64(1),
				"to":    "12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5pV",
			},
			expect: true,
		},
		{
			name: "give invalid peer",
			input: map[string]any{
				"t":     "give",
				"epoch": uint64(1),
				"to":    "invalid peer with spaces",
			},
			expect: false,
		},
		{
			name: "deny valid",
			input: map[string]any{
				"t":  "deny",
				"to": "12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5pV",
			},
			expect: true,
		},
		{
			name: "unknown type",
			input: map[string]any{
				"t": "unknown",
			},
			expect: false,
		},
		{
			name: "extra key",
			input: map[string]any{
				"t":      "state",
				"epoch":  uint64(1),
				"screen": "show",
				"seq":    uint64(1),
				"slide":  uint64(1),
				"step":   uint64(1),
				"extra":  "key",
			},
			expect: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := present(tc.input)
			if got != tc.expect {
				t.Fatalf("present(%v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

// Spec 9.2.1 item 6: closed() ensures map has exactly the given keys.
func TestClosed(t *testing.T) {
	tests := []struct {
		name   string
		m      map[string]any
		keys   []string
		expect bool
	}{
		{"exact match", map[string]any{"a": 1, "b": 2}, []string{"a", "b"}, true},
		{"extra key", map[string]any{"a": 1, "b": 2, "c": 3}, []string{"a", "b"}, false},
		{"missing key", map[string]any{"a": 1}, []string{"a", "b"}, false},
		{"empty", map[string]any{}, []string{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := closed(tc.m, tc.keys...)
			if got != tc.expect {
				t.Fatalf("closed(%v, %v) = %v, want %v", tc.m, tc.keys, got, tc.expect)
			}
		})
	}
}

// Spec 9.3: epoch() accepts uint64 ≤ 2^31.
func TestEpoch(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect bool
	}{
		{"valid uint64", uint64(1), true},
		{"max valid", uint64(2147483648), true},
		{"too large", uint64(2147483649), false},
		{"zero", uint64(0), true},
		{"wrong type", "1", false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := epoch(tc.input)
			if got != tc.expect {
				t.Fatalf("epoch(%v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

// Spec 9.2.1 item 5: uintAtMost() checks uint64 ≤ maxVal.
func TestUintAtMost(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		max    uint64
		expect bool
	}{
		{"valid", uint64(5), 10, true},
		{"at max", uint64(10), 10, true},
		{"over max", uint64(11), 10, false},
		{"wrong type", "5", 10, false},
		{"nil", nil, 10, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := uintAtMost(tc.input, tc.max)
			if got != tc.expect {
				t.Fatalf("uintAtMost(%v, %v) = %v, want %v", tc.input, tc.max, got, tc.expect)
			}
		})
	}
}

// Spec 9.5: nonZero() checks uint64 ≥ 1.
func TestNonZero(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect bool
	}{
		{"positive", uint64(1), true},
		{"zero", uint64(0), false},
		{"wrong type", "1", false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nonZero(tc.input)
			if got != tc.expect {
				t.Fatalf("nonZero(%v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

// Spec 9.5 / §9.7 G1: screen() validates screen values.
func TestScreen(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect bool
	}{
		{"show", "show", true},
		{"black", "black", true},
		{"white", "white", true},
		{"invalid", "invalid", false},
		{"wrong type", 1, false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := screen(tc.input)
			if got != tc.expect {
				t.Fatalf("screen(%v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

// Spec 9.2.3: peer() validates peer ID format (base58btc multihash).
func TestPeer(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect bool
	}{
		{"valid peer", "12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5pV", true},
		{"valid with bang", "12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5p!", true},
		{"too long", "12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5pV12345678901234", false},
		{"wrong type", 1, false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := peer(tc.input)
			if got != tc.expect {
				t.Fatalf("peer(%v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

// Spec 9.5: channel() validates channel name/anchor/head length limits.
func TestChannel(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  any
		expect bool
	}{
		{"name valid", "name", "validname", true},
		{"name too long", "name", strings.Repeat("a", 129), false},
		{"anchor valid", "anchor", "validanchor", true},
		{"head valid", "head", "validhead", true},
		{"unknown key", "unknown", "value", false},
		{"wrong type", "name", 123, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := channel(tc.key, tc.value)
			if got != tc.expect {
				t.Fatalf("channel(%q, %v) = %v, want %v", tc.key, tc.value, got, tc.expect)
			}
		})
	}
}
