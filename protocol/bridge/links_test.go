// SPDX-License-Identifier: MPL-2.0

package bridge_test

import (
	"errors"
	"strings"
	"testing"

	transport "github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/bridge"
)

func entries(peers ...string) []bridge.Link {
	out := make([]bridge.Link, len(peers))
	for i, p := range peers {
		out[i] = bridge.Link{Peer: transport.PeerID(p), Path: bridge.PathDirect, Quality: bridge.QualityGood}
	}
	return out
}

func sortedPeers(n int) []bridge.Link {
	out := make([]bridge.Link, n)
	for i := range out {
		out[i] = bridge.Link{Peer: transport.PeerID([]byte{'p', byte('a' + i/26/26%26), byte('a' + i/26%26), byte('a' + i%26)}), Quality: bridge.QualityGood}
	}
	return out
}

func refused(t *testing.T, what string, b []byte) {
	t.Helper()
	if _, err := bridge.DecodeLinks(b); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeLinks(%s) = %v, want ErrMalformed", what, err)
	}
}

func encodeRaw(t *testing.T, ls []bridge.Link) []byte {
	t.Helper()
	b, err := bridge.EncodeLinks(ls)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Annex A.1, Links: n u16 ≤ 1024; a larger n is refused.
func TestLinksCountBound(t *testing.T) {
	b := encodeRaw(t, sortedPeers(bridge.MaxLinks))
	got, err := bridge.DecodeLinks(b)
	if err != nil || len(got) != bridge.MaxLinks {
		t.Fatalf("n = 1024: %d entries, %v", len(got), err)
	}
	if _, err := bridge.EncodeLinks(sortedPeers(bridge.MaxLinks + 1)); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeLinks(1025) = %v", err)
	}
	refused(t, "n 1025", []byte{0x04, 0x01})
	refused(t, "n 65535", []byte{0xff, 0xff})
	refused(t, "no count", nil)
	refused(t, "one byte", []byte{0})
	if got, err := bridge.DecodeLinks([]byte{0, 0}); err != nil || got != nil {
		t.Errorf("n = 0: %v, %v", got, err)
	}
}

// Annex A.1, Links: peer str8 of 1 to 64 characters.
func TestLinksPeerLength(t *testing.T) {
	for _, n := range []int{1, 64} {
		ls := entries(strings.Repeat("p", n))
		got, err := bridge.DecodeLinks(encodeRaw(t, ls))
		if err != nil || len(got) != 1 || got[0].Peer != ls[0].Peer {
			t.Errorf("peer of %d: %v, %v", n, got, err)
		}
	}
	for _, n := range []int{0, 65} {
		if _, err := bridge.EncodeLinks(entries(strings.Repeat("p", n))); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("EncodeLinks(peer of %d) = %v", n, err)
		}
	}
	good := encodeRaw(t, entries("p"))
	zero := append([]byte{0, 1, 0}, good[3+1:]...) // length 0, then the figures
	refused(t, "peer of 0", zero)
	long := append([]byte{0, 1, 65}, append([]byte(strings.Repeat("p", 65)), good[3+1:]...)...)
	refused(t, "peer of 65", long)
	bad := encodeRaw(t, entries("p"))
	bad[3] = ' ' // not a printable, non-space ASCII character
	refused(t, "space in peer", bad)
}

// Annex A.1, Links: path 0 to 3, quality 0 to 3, lossScope 0 to 2; anything else is refused.
func TestLinksDomains(t *testing.T) {
	set := map[string]func(*bridge.Link, uint8){
		"path":      func(l *bridge.Link, v uint8) { l.Path = v },
		"quality":   func(l *bridge.Link, v uint8) { l.Quality = v },
		"lossScope": func(l *bridge.Link, v uint8) { l.LossScope = v },
	}
	hi := map[string]uint8{"path": 3, "quality": 3, "lossScope": 2}
	for name, f := range set {
		for v := uint8(0); v <= hi[name]; v++ {
			l := entries("p")
			f(&l[0], v)
			if got, err := bridge.DecodeLinks(encodeRaw(t, l)); err != nil || len(got) != 1 {
				t.Errorf("%s %d: %v, %v", name, v, got, err)
			}
		}
		l := entries("p")
		f(&l[0], hi[name]+1)
		if _, err := bridge.EncodeLinks(l); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("EncodeLinks(%s %d) = %v", name, hi[name]+1, err)
		}
		// The decoder refuses it too: patch the byte of a valid payload.
		b := encodeRaw(t, entries("p"))
		at := map[string]int{"path": 4, "quality": 5, "lossScope": 4 + 2 + 4 + 2 + 2 + 12 + 2}[name]
		b[at] = hi[name] + 1
		refused(t, name+" out of domain", b)
	}
}

// Annex A.1, Links: peers sorted by peer, no duplicate.
func TestLinksOrder(t *testing.T) {
	for _, ls := range [][]bridge.Link{entries("b", "a"), entries("a", "a"), entries("b", "a", "c")} {
		if _, err := bridge.EncodeLinks(ls); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("EncodeLinks(%v) = %v", ls, err)
		}
	}
	// A decoder sees the same through bytes: swap two valid entries.
	ab := encodeRaw(t, entries("a", "b"))
	entry := 1 + 1 + bridge.LinkFixed
	ba := append([]byte{0, 2}, append(append([]byte{}, ab[2+entry:]...), ab[2:2+entry]...)...)
	refused(t, "descending", ba)
	dup := append([]byte{0, 2}, append(append([]byte{}, ab[2:2+entry]...), ab[2:2+entry]...)...)
	refused(t, "duplicate", dup)
	if got, err := bridge.DecodeLinks(ab); err != nil || len(got) != 2 {
		t.Errorf("ascending: %v, %v", got, err)
	}
	// Prefix order: "a" < "ab".
	if _, err := bridge.EncodeLinks(entries("a", "ab")); err != nil {
		t.Errorf("a, ab: %v", err)
	}
}

// Annex A.1, Links: a payload shorter or longer than its entries is refused.
func TestLinksLength(t *testing.T) {
	b := encodeRaw(t, entries("a", "b"))
	refused(t, "one byte short", b[:len(b)-1])
	refused(t, "one byte extra", append(append([]byte{}, b...), 0))
	for i := range b {
		if _, err := bridge.DecodeLinks(b[:i]); err == nil {
			t.Errorf("prefix of %d bytes accepted", i)
		}
	}
	// A count above the entries present.
	over := append([]byte{}, b...)
	over[1] = 3
	refused(t, "count above entries", over)
	under := append([]byte{}, b...)
	under[1] = 1
	refused(t, "count below entries", under)
}

// Annex A.1, Links: every figure is big endian, in the order of the layout.
func TestLinksRoundTrip(t *testing.T) {
	in := []bridge.Link{{
		Peer: "peer-a", Path: 1, Quality: 2, PathAgeMs: 0x01020304, Upgrades: 0x0506, Fallbacks: 0x0708,
		RTTMs: 0x090a0b0c, RTTVarMs: 0x0d0e0f10, RTTAgeMs: 0x11121314, LossPermille: 0x1516, LossScope: 2,
		UpMs: 0x191a1b1c, Downs10m: 0x1d1e, DownsTotal: 0x1f202122, LastDownMs: 0x23242526, Resyncs10m: 0x2728,
		Resyncs: 0x292a2b2c, PendingMsgs: 0x2d2e2f30, PendingBytes: 0x31323334, OldestPendingMs: 0x35363738,
		SendFailures: 0x393a3b3c, Resets: 0x3d3e3f40,
	}, {Peer: "peer-b", Path: 3, Quality: 0, LossPermille: 0xFFFF}}
	b := encodeRaw(t, in)
	if want := 2 + 2*(1+6+bridge.LinkFixed); len(b) != want {
		t.Fatalf("encoded %d bytes, want %d", len(b), want)
	}
	if b[0] != 0 || b[1] != 2 || b[2] != 6 {
		t.Errorf("header = %x", b[:3])
	}
	out, err := bridge.DecodeLinks(b)
	if err != nil || len(out) != 2 || out[0] != in[0] || out[1] != in[1] {
		t.Fatalf("round trip = %+v, %v", out, err)
	}
	if again := encodeRaw(t, out); string(again) != string(b) {
		t.Error("re-encoding differs")
	}
}

// Annex A.1, Links: a Links envelope carries a valid payload, from the local
// peer, and a payload on a kind without one is refused.
func TestLinksEnvelope(t *testing.T) {
	payload := encodeRaw(t, entries("a"))
	b, err := bridge.Encode(bridge.Envelope{Kind: bridge.Links, Peer: "me", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	e, err := bridge.Decode(b)
	if err != nil || e.Kind != bridge.Links || e.Kind.String() != "Links" {
		t.Fatalf("Decode = %+v, %v", e, err)
	}
	if _, err := bridge.Encode(bridge.Envelope{Kind: bridge.Links, Peer: "me", Payload: payload[:len(payload)-1]}); err == nil {
		t.Error("Links with a truncated payload encoded")
	}
	for _, k := range []bridge.Kind{10, 19, 21} {
		if _, err := bridge.Encode(bridge.Envelope{Kind: k, Peer: "me"}); err == nil {
			t.Errorf("kind %d encoded", k)
		}
	}
}

// Annex A.1 (envelope): a frame is header, peer and payload; one that is
// exactly as long as its header and peer carries an empty payload and is valid.
func TestEnvelopeFraming(t *testing.T) {
	up := []byte{1, byte(bridge.PeerUp), 3, 'a', 'b', 'c'}
	e, err := bridge.Decode(up)
	if err != nil || e.Kind != bridge.PeerUp || e.Peer != "abc" || len(e.Payload) != 0 {
		t.Fatalf("Decode(PeerUp) = %+v, %v", e, err)
	}
	if again, err := bridge.Encode(e); err != nil || string(again) != string(up) {
		t.Errorf("Encode = %x, %v", again, err)
	}
	for i := range up {
		if _, err := bridge.Decode(up[:i]); err == nil {
			t.Errorf("prefix of %d bytes accepted", i)
		}
	}
	if _, err := bridge.Decode([]byte{1, byte(bridge.PeerUp), 0}); err == nil {
		t.Error("empty peer accepted")
	}
	if _, err := bridge.Decode([]byte{2, byte(bridge.PeerUp), 1, 'a'}); err == nil {
		t.Error("version 2 accepted")
	}
	if _, err := bridge.Decode(append(append([]byte{}, up...), 'x')); err == nil {
		t.Error("payload on PeerUp accepted")
	}
}

// Annex A.1 (envelope): the longest frame, a 64-byte peer and the largest
// payload, is valid; one byte more is not.
func TestEnvelopeLargest(t *testing.T) {
	peer := transport.PeerID(strings.Repeat("p", 64))
	e := bridge.Envelope{Kind: bridge.Message, Peer: peer, Payload: make([]byte, transport.MaxFrame)}
	b, err := bridge.Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Decode(b); err != nil {
		t.Errorf("Decode(largest) = %v", err)
	}
	if _, err := bridge.Decode(append(b, 0)); err == nil {
		t.Error("one byte over the largest accepted")
	}
}

// Annex A.1, Roles: n u16 ≤ 1024 of (peer str8, role 1 to 3, flags bit 0).
func TestRolesBoundaries(t *testing.T) {
	role := func(peer string, r, flags byte) []byte {
		return append(append([]byte{byte(len(peer))}, peer...), r, flags)
	}
	one := func(peer string, r, flags byte) []byte { return append([]byte{0, 1}, role(peer, r, flags)...) }
	for _, c := range []struct {
		name string
		b    []byte
		ok   bool
	}{
		{"role 1", one("p", 1, 0), true},
		{"role 3 suspended", one("p", 3, 1), true},
		{"role 0", one("p", 0, 0), false},
		{"role 4", one("p", 4, 0), false},
		{"flag 2", one("p", 1, 2), false},
		{"flags 3", one("p", 1, 3), false},
		{"empty peer", one("", 1, 0), false},
		{"space in peer", one("p q", 1, 0), false},
		{"peer 64", one(strings.Repeat("p", 64), 2, 0), true},
		{"peer 65", one(strings.Repeat("p", 65), 2, 0), false},
		{"empty list", []byte{0, 0}, true},
		{"no count", []byte{0}, false},
		{"n 1025", []byte{0x04, 0x01}, false},
		{"trailing", append(one("p", 1, 0), 0), false},
		{"truncated", one("p", 1, 0)[:4], false},
		{"count above entries", append([]byte{0, 2}, role("p", 1, 0)...), false},
	} {
		got, err := bridge.DecodeRoles(c.b)
		if (err == nil) != c.ok {
			t.Errorf("%s: %v, %v", c.name, got, err)
		}
		if err != nil && !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("%s: %v is not ErrMalformed", c.name, err)
		}
		if err == nil {
			if again, err := bridge.EncodeRoles(got); err != nil || string(again) != string(c.b) {
				t.Errorf("%s: re-encoding = %x, %v", c.name, again, err)
			}
		}
	}
	hi := make([]bridge.MemberRole, bridge.MaxMembers+1)
	for i := range hi {
		hi[i] = bridge.MemberRole{Peer: transport.PeerID([]byte{'p', byte('a' + i/26%26), byte('a' + i%26)}), Role: 1}
	}
	if b, err := bridge.EncodeRoles(hi[:bridge.MaxMembers]); err != nil {
		t.Errorf("EncodeRoles(1024) = %v", err)
	} else if got, err := bridge.DecodeRoles(b); err != nil || len(got) != bridge.MaxMembers || got[1023].Suspended {
		t.Errorf("DecodeRoles(1024) = %d, %v", len(got), err)
	}
	if _, err := bridge.EncodeRoles(hi); err == nil {
		t.Error("EncodeRoles(1025) accepted")
	}
	for _, m := range []bridge.MemberRole{{Peer: "p", Role: 0}, {Peer: "p", Role: 4}, {Peer: "", Role: 1}} {
		if _, err := bridge.EncodeRoles([]bridge.MemberRole{m}); err == nil {
			t.Errorf("EncodeRoles(%+v) accepted", m)
		}
	}
}
