package mux_test

import (
	"bytes"
	"testing"
	"testing/synctest"

	"github.com/oioio-space/inkwell/internal/transport"
)

// TestFragmentsMustAgreeWithTheirMessage: a fragment that continues a message
// in progress must carry the same id, channel and total as the first one.
// Anything else ends the message: it is never completed, and above all never
// completed on another channel or with another length.
func TestFragmentsMustAgreeWithTheirMessage(t *testing.T) {
	for _, tc := range []struct {
		name string
		next []byte // the second fragment, after frag(chRepo, 7, 10, 0, "01234")
	}{
		{"other channel", frag(chTrust, 7, 10, 5, []byte("56789"))},
		{"other total, shorter", frag(chRepo, 7, 8, 5, []byte("567"))},
		{"other total, longer", frag(chRepo, 7, 11, 5, []byte("56789A"))},
		{"other id", frag(chRepo, 8, 10, 5, []byte("56789"))},
	} {
		synctest.Test(t, func(t *testing.T) {
			rig, raws := newHostileRig(t, "x")
			x := raws["x"]
			sendFrag(t, x, frag(chRepo, 7, 10, 0, []byte("01234")))
			sendFrag(t, x, tc.next)
			synctest.Wait()
			for ch, c := range map[string]transport.Transport{"repo": rig.repo, "trust": rig.trust} {
				if got := messages(c); len(got) != 0 {
					t.Errorf("%s: delivered %q on channel %s", tc.name, got, ch)
				}
			}
			if got := rig.res.inUse(); got != 0 {
				t.Errorf("%s: %d bytes still reserved: the inconsistent message was kept", tc.name, got)
			}
		})
	}
}

// TestRepeatedFragmentsDoNotBreakAMessage: a link that repeats a fragment, at
// any point of the message, changes nothing.
func TestRepeatedFragmentsDoNotBreakAMessage(t *testing.T) {
	data := content(9, 3)
	for _, tc := range []struct {
		name  string
		order []int // offsets of the fragments, in the order they arrive
	}{
		{"repeat of the middle one", []int{0, 3, 3, 6}},
		{"repeat of the last but one, twice", []int{0, 3, 3, 3, 6}},
		{"repeat of an earlier one", []int{0, 3, 0, 6}}, // restarts: the message is incomplete, not wrong
		{"in order", []int{0, 3, 6}},
	} {
		synctest.Test(t, func(t *testing.T) {
			rig, raws := newHostileRig(t, "x")
			for _, off := range tc.order {
				sendFrag(t, raws["x"], frag(chRepo, 1, 9, uint32(off), data[off:off+3]))
			}
			synctest.Wait()
			got := messages(rig.repo)
			complete := tc.name != "repeat of an earlier one"
			switch {
			case complete && (len(got) != 1 || !bytes.Equal(got[0], data)):
				t.Errorf("%s: delivered %d messages, want the one intact message", tc.name, len(got))
			case !complete && len(got) != 0:
				t.Errorf("%s: delivered %q from a message that went back to its start", tc.name, got)
			}
		})
	}
}

// TestFragmentOverlapAndGap: a fragment that overlaps what we have, or leaves
// a hole, ends the message.
func TestFragmentOverlapAndGap(t *testing.T) {
	for name, f := range map[string]struct {
		off  uint32
		data string
	}{
		"overlap reaching past what we have": {4, "45"},
		"gap":                                {6, "6"},
		"far gap":                            {9, "9"},
	} {
		synctest.Test(t, func(t *testing.T) {
			rig, raws := newHostileRig(t, "x")
			x := raws["x"]
			sendFrag(t, x, frag(chRepo, 1, 10, 0, []byte("01234")))
			sendFrag(t, x, frag(chRepo, 1, 10, f.off, []byte(f.data)))
			sendFrag(t, x, frag(chRepo, 1, 10, 5, []byte("56789"))) // the right continuation, too late
			synctest.Wait()
			if got := messages(rig.repo); len(got) != 0 {
				t.Errorf("%s: delivered %q", name, got)
			}
		})
	}
}

// TestPeersAreIndependent: fragments of two peers interleave without ever
// mixing; one peer's hostile fragment does not end the other's message.
func TestPeersAreIndependent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x", "y")
		x, y := raws["x"], raws["y"]
		sendFrag(t, x, frag(chRepo, 1, 6, 0, []byte("xxx")))
		sendFrag(t, y, frag(chRepo, 1, 6, 0, []byte("yyy")))
		sendFrag(t, y, frag(chRepo, 99, 6, 3, []byte("zzz"))) // y breaks its own message
		sendFrag(t, x, frag(chRepo, 1, 6, 3, []byte("XXX")))
		sendFrag(t, y, frag(chRepo, 1, 6, 3, []byte("YYY"))) // too late for y
		synctest.Wait()
		got := messages(rig.repo)
		if len(got) != 1 || string(got[0]) != "xxxXXX" {
			t.Errorf("delivered %q, want only x's message", got)
		}
	})
}
