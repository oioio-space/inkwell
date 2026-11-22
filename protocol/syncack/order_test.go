// SPDX-License-Identifier: MPL-2.0

package syncack_test

import (
	"slices"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol/syncack"
)

// P-8, P-9 (spec §8.7.3): the order of one peer's inbound sync streams.
func TestOrder(t *testing.T) {
	t.Run("two places", func(t *testing.T) {
		var o syncack.Order
		if !o.Arrive(1, 0) || !o.Arrive(2, 0) {
			t.Fatal("the first two streams were not placed")
		}
		if o.Arrive(3, 0) || o.Len() != 2 || syncack.Places != 2 {
			t.Fatalf("a third stream was placed (len %d, places %d)", o.Len(), syncack.Places)
		}
		if o.Ready(3, time.Hour) || o.Holds(3) || !o.Holds(2) {
			t.Error("a refused stream has a turn")
		}
	})
	t.Run("turns and wake", func(t *testing.T) {
		var o syncack.Order
		o.Arrive(1, 0)
		o.Arrive(2, 0)
		if !o.Ready(1, 0) || o.Ready(2, 0) {
			t.Fatalf("ready: 1 %v, 2 %v; want the head only", o.Ready(1, 0), o.Ready(2, 0))
		}
		if woken := o.Leave(1); !slices.Equal(woken, []uint64{2}) || !o.Ready(2, 0) {
			t.Fatalf("Leave(head) woke %v, want [2]", woken)
		}
		if woken := o.Leave(2); len(woken) != 0 || o.Len() != 0 {
			t.Fatalf("Leave(last) woke %v, len %d", woken, o.Len())
		}
	})
	t.Run("leave while waiting", func(t *testing.T) {
		var o syncack.Order
		o.Arrive(1, 0)
		o.Arrive(2, 0)
		if woken := o.Leave(2); len(woken) != 0 || o.Len() != 1 || !o.Ready(1, 0) {
			t.Fatalf("Leave(waiting) woke %v, len %d", woken, o.Len())
		}
		if !o.Arrive(3, 0) || o.Ready(3, 0) {
			t.Fatal("the place given back was not taken, or the new stream skipped the head")
		}
		if woken := o.Leave(1); !slices.Equal(woken, []uint64{3}) {
			t.Fatalf("Leave(head) woke %v, want [3]", woken)
		}
	})
	t.Run("leave unknown", func(t *testing.T) {
		var o syncack.Order
		o.Arrive(1, 0)
		if woken := o.Leave(7); len(woken) != 0 || o.Len() != 1 {
			t.Fatalf("Leave(unknown) woke %v, len %d", woken, o.Len())
		}
	})
	t.Run("reorder wait", func(t *testing.T) {
		var o syncack.Order
		o.Arrive(1, 0)
		o.Arrive(2, 100*time.Millisecond)
		if o.Ready(2, 100*time.Millisecond+syncack.ReorderWait-time.Millisecond) {
			t.Error("ready before its 60 s")
		}
		if !o.Ready(2, 100*time.Millisecond+syncack.ReorderWait) || syncack.ReorderWait != time.Minute {
			t.Error("not ready after its 60 s")
		}
		if !o.Ready(1, 0) {
			t.Error("the head is not ready")
		}
		if got := o.WaitLeft(2, 100*time.Millisecond+time.Second); got != syncack.ReorderWait-time.Second {
			t.Errorf("WaitLeft = %v, want %v", got, syncack.ReorderWait-time.Second)
		}
		if got := o.WaitLeft(2, time.Hour); got != 0 {
			t.Errorf("WaitLeft past the wait = %v, want 0", got)
		}
		if got := o.WaitLeft(1, 0); got != 0 || !o.Holds(1) {
			t.Errorf("WaitLeft(head) = %v (holds %v), want 0", got, o.Holds(1))
		}
		if got := o.WaitLeft(9, 0); got != 0 {
			t.Errorf("WaitLeft(unknown) = %v, want 0", got)
		}
		short := syncack.Order{Wait: time.Second}
		short.Arrive(1, 0)
		short.Arrive(2, 0)
		if short.Ready(2, time.Second-1) || !short.Ready(2, time.Second) || short.WaitLeft(2, 0) != time.Second {
			t.Error("Order.Wait does not replace ReorderWait")
		}
	})
}

// P-8 (spec §8.7.4): the opener's reading of a refusal.
func TestRefusal(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	for _, c := range []struct {
		name     string
		role     syncack.Role
		capacity bool
		elapsed  time.Duration
		attempt  int
		want     syncack.Action
		wait     time.Duration
	}{
		{"first", syncack.RoleSession, true, 0, 0, syncack.Retry, ms(50)},
		{"doubled", syncack.RoleSession, true, ms(2000), 3, syncack.Retry, ms(400)},
		{"capped", syncack.RoleSession, true, ms(4000), 5, syncack.Retry, time.Second},
		{"capped far", syncack.RoleSession, true, ms(4000), 60, syncack.Retry, time.Second},
		{"in budget", syncack.RoleSession, true, ms(14949), 0, syncack.Retry, ms(50)},
		{"out of budget", syncack.RoleSession, true, ms(14950), 0, syncack.ClosePeerAction, 0},
		{"move", syncack.RoleMove, true, ms(100), 1, syncack.Retry, ms(100)},
		{"move out of budget", syncack.RoleMove, true, ms(14990), 0, syncack.AbandonAction, 0},
		{"refusal", syncack.RoleSession, false, 0, 0, syncack.ClosePeerAction, 0},
		{"refusal move", syncack.RoleMove, false, 0, 0, syncack.AbandonAction, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, wait := syncack.OnRefusal(c.role, c.capacity, c.elapsed, c.attempt)
			if got != c.want || wait != c.wait {
				t.Errorf("OnRefusal = %v, %v; want %v, %v", got, wait, c.want, c.wait)
			}
		})
	}
	if syncack.BusyCode != 0x1002 {
		t.Errorf("BusyCode = %#x, want 0x1002", syncack.BusyCode)
	}
	for a, want := range map[syncack.Action]string{syncack.Retry: "retry", syncack.ClosePeerAction: "close_peer", syncack.AbandonAction: "abandon", syncack.Action(3): "action(3)", syncack.Action(9): "action(9)"} {
		if a.String() != want {
			t.Errorf("Action(%d).String() = %q, want %q", a, a.String(), want)
		}
	}
}
