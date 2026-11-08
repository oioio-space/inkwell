// SPDX-License-Identifier: MPL-2.0

package bridge_test

import (
	"math"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol/bridge"
)

var now = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// Spec Annex A.1, Links: instants are ages in ms, loss is in permille.
func TestLinksFigures(t *testing.T) {
	got := bridge.LinkOf(bridge.Measures{
		Peer: "peer-a", Path: bridge.PathRelayed, Quality: bridge.QualityFair, LossScope: bridge.LossScopeFirstHop,
		PathSince: now.Add(-61 * time.Second), UpSince: now.Add(-2 * time.Minute),
		RTT: 180 * time.Millisecond, RTTVar: 25 * time.Millisecond, RTTAge: 900 * time.Millisecond,
		LossPct: new(4.25), LastDown: 8 * time.Second, OldestPending: 150 * time.Millisecond,
		Upgrades: 2, Fallbacks: 1, Downs10m: 1, DownsTotal: 4, Resyncs10m: 1, Resyncs: 3,
		PendingMsgs: 2, PendingBytes: 4096, SendFailures: 5, Resets: 1,
	}, now)
	want := bridge.Link{
		Peer: "peer-a", Path: 3, Quality: 2, PathAgeMs: 61000, Upgrades: 2, Fallbacks: 1,
		RTTMs: 180, RTTVarMs: 25, RTTAgeMs: 900, LossPermille: 43, LossScope: 2,
		UpMs: 120000, Downs10m: 1, DownsTotal: 4, LastDownMs: 8000, Resyncs10m: 1, Resyncs: 3,
		PendingMsgs: 2, PendingBytes: 4096, OldestPendingMs: 150, SendFailures: 5, Resets: 1,
	}
	if got != want {
		t.Errorf("link = %+v\nwant   %+v", got, want)
	}
}

// Spec Annex A.1, Links: loss 0xFFFF and scope none while fewer than 50 packets
// were sent; rttMs 0 means no measure, so a real sub-millisecond RTT is 1.
func TestLinksUnmeasured(t *testing.T) {
	got := bridge.LinkOf(bridge.Measures{Peer: "p", Path: bridge.PathLAN, LossScope: bridge.LossScopePath, PathSince: now}, now)
	if got.LossPermille != bridge.LossUnknown || got.LossScope != 0 || got.RTTMs != 0 || got.UpMs != 0 {
		t.Errorf("unmeasured link = %+v", got)
	}
	got = bridge.LinkOf(bridge.Measures{Peer: "p", RTT: 200 * time.Microsecond}, now)
	if got.RTTMs != 1 {
		t.Errorf("RTTMs = %d, want 1", got.RTTMs)
	}
}

// Spec Annex A.1, Links: a value out of range saturates at the maximum of its
// type, never truncated; a loss saturates one below the « unknown » sentinel.
func TestLinksSaturation(t *testing.T) {
	huge := time.Duration(math.MaxInt64)
	got := bridge.LinkOf(bridge.Measures{
		Peer: "p", Path: bridge.PathDirect, Quality: bridge.QualityPoor, LossScope: bridge.LossScopePath,
		PathSince: now.Add(-huge), UpSince: now.Add(-huge),
		RTT: huge, RTTVar: huge, RTTAge: huge, LossPct: new(1e9), LastDown: huge, OldestPending: huge,
		Upgrades: 1 << 20, Fallbacks: math.MaxInt, Downs10m: 1 << 17, DownsTotal: math.MaxInt,
		Resyncs10m: 70000, Resyncs: math.MaxInt, PendingMsgs: math.MaxInt, PendingBytes: math.MaxInt,
		SendFailures: math.MaxUint64, Resets: 1 << 40,
	}, now)
	const m32, m16 = math.MaxUint32, math.MaxUint16
	want := bridge.Link{
		Peer: "p", Path: 2, Quality: 3, PathAgeMs: m32, Upgrades: m16, Fallbacks: m16,
		RTTMs: m32, RTTVarMs: m32, RTTAgeMs: m32, LossPermille: m16 - 1, LossScope: 1,
		UpMs: m32, Downs10m: m16, DownsTotal: m32, LastDownMs: m32, Resyncs10m: m16, Resyncs: m32,
		PendingMsgs: m32, PendingBytes: m32, OldestPendingMs: m32, SendFailures: m32, Resets: m32,
	}
	if got != want {
		t.Errorf("link = %+v\nwant   %+v", got, want)
	}
}

// Spec Annex A.1, coalescence of Links: ages count as set or unset only.
func TestLinksStable(t *testing.T) {
	a := bridge.Link{Peer: "p", PathAgeMs: 5, RTTAgeMs: 900, UpMs: 7000, OldestPendingMs: 30, RTTMs: 4}
	b := bridge.Link{Peer: "p", PathAgeMs: 60000, RTTAgeMs: 1, UpMs: 1, OldestPendingMs: 31000, RTTMs: 4}
	if a.Stable() != b.Stable() {
		t.Errorf("ages that are all set differ: %+v vs %+v", a.Stable(), b.Stable())
	}
	b.UpMs = 0
	if a.Stable() == b.Stable() {
		t.Error("a link that went down looks unchanged")
	}
}

// Spec Annex A.1, Links: the payload size follows the field list, the figures
// being LinkFixed bytes after each peer.
func TestLinksLayout(t *testing.T) {
	b, err := bridge.EncodeLinks([]bridge.Link{{Peer: "ab", Quality: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if want := 2 + 1 + 2 + bridge.LinkFixed; len(b) != want {
		t.Errorf("payload of %d bytes, want %d", len(b), want)
	}
}

// Spec Annex A.1, Links: a value saturates exactly past the maximum of its
// type: the maximum itself and the value before it are kept.
func TestLinksSaturationBoundary(t *testing.T) {
	ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
	for _, c := range []struct {
		name string
		d    time.Duration
		want uint32
	}{
		{"max-1", ms(math.MaxUint32 - 1), math.MaxUint32 - 1},
		{"max", ms(math.MaxUint32), math.MaxUint32},
		{"max+1", ms(math.MaxUint32 + 1), math.MaxUint32},
		{"max+1ns", ms(math.MaxUint32) + 1, math.MaxUint32},
		{"zero", 0, 0},
		{"negative", -time.Second, 0},
	} {
		if got := bridge.LinkOf(bridge.Measures{Peer: "p", RTT: c.d}, now).RTTMs; got != c.want {
			t.Errorf("RTT %s: %d, want %d", c.name, got, c.want)
		}
	}
	for _, c := range []struct {
		n              int
		want32, want16 uint32
	}{
		{-1, 0, 0},
		{0, 0, 0},
		{1, 1, 1},
		{math.MaxUint16 - 1, math.MaxUint16 - 1, math.MaxUint16 - 1},
		{math.MaxUint16, math.MaxUint16, math.MaxUint16},
		{math.MaxUint16 + 1, math.MaxUint16 + 1, math.MaxUint16},
		{math.MaxUint32 - 1, math.MaxUint32 - 1, math.MaxUint16},
		{math.MaxUint32, math.MaxUint32, math.MaxUint16},
		{math.MaxUint32 + 1, math.MaxUint32, math.MaxUint16},
	} {
		l := bridge.LinkOf(bridge.Measures{Peer: "p", Upgrades: c.n, DownsTotal: c.n}, now)
		if uint32(l.Upgrades) != c.want16 || l.DownsTotal != c.want32 {
			t.Errorf("count %d: u16 %d u32 %d, want %d %d", c.n, l.Upgrades, l.DownsTotal, c.want16, c.want32)
		}
	}
	if l := bridge.LinkOf(bridge.Measures{Peer: "p", SendFailures: math.MaxUint32, Resets: math.MaxUint32 + 1}, now); l.SendFailures != math.MaxUint32 || l.Resets != math.MaxUint32 {
		t.Errorf("uint64 counts: %d %d", l.SendFailures, l.Resets)
	}
}

// Spec Annex A.1, Links: a positive duration is at least 1 ms, rounded up, so
// rttMs 0 only ever means « no measure ».
func TestLinksSubMillisecondRTT(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want uint32
	}{
		{1, 1}, {time.Millisecond - 1, 1}, {time.Millisecond, 1}, {time.Millisecond + 1, 2}, {2 * time.Millisecond, 2},
	} {
		if got := bridge.LinkOf(bridge.Measures{Peer: "p", RTT: c.d}, now).RTTMs; got != c.want {
			t.Errorf("RTT %v: %d, want %d", c.d, got, c.want)
		}
	}
}

// Spec Annex A.1, Links: loss in permille, saturating at 0xFFFE since 0xFFFF
// means unknown; no figure, or NaN, is 0xFFFF with scope none.
func TestLinksLossPermille(t *testing.T) {
	for _, c := range []struct {
		pct  float64
		want uint16
	}{
		{0, 0},
		{0.04, 0},
		{0.05, 1},
		{0.1, 1},
		{100, 1000},
		{-5, 0},
		{6553.3, 65533},
		{6553.4, 65534},
		{6553.5, 65534},
		{6553.6, 65534},
		{1e12, 65534},
	} {
		l := bridge.LinkOf(bridge.Measures{Peer: "p", LossPct: &c.pct, LossScope: bridge.LossScopeFirstHop}, now)
		if l.LossPermille != c.want || l.LossScope != bridge.LossScopeFirstHop {
			t.Errorf("loss %v%%: %d scope %d, want %d", c.pct, l.LossPermille, l.LossScope, c.want)
		}
	}
	nan := math.NaN()
	for _, p := range []*float64{nil, &nan} {
		l := bridge.LinkOf(bridge.Measures{Peer: "p", LossPct: p, LossScope: bridge.LossScopePath}, now)
		if l.LossPermille != bridge.LossUnknown || l.LossScope != bridge.LossScopeNone {
			t.Errorf("unknown loss: %d scope %d", l.LossPermille, l.LossScope)
		}
	}
}

// Spec Annex A.1, Links: an instant is an age, and no instant is age 0.
func TestLinksAges(t *testing.T) {
	l := bridge.LinkOf(bridge.Measures{Peer: "p", PathSince: now.Add(-1500 * time.Millisecond), UpSince: now}, now)
	if l.PathAgeMs != 1500 || l.UpMs != 0 {
		t.Errorf("ages = %d, %d", l.PathAgeMs, l.UpMs)
	}
	if l := bridge.LinkOf(bridge.Measures{Peer: "p", PathSince: now.Add(time.Hour)}, now); l.PathAgeMs != 0 {
		t.Errorf("an instant in the future is age %d", l.PathAgeMs)
	}
}
