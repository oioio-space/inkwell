package mux

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
)

// waiter takes a turn on g as soon as it is granted and records its name.
func waiter(g *gate, lo bool, name string, order *[]string) {
	go func() {
		if err := g.acquire(context.Background(), lo); err != nil {
			return
		}
		*order = append(*order, name)
		g.release()
	}()
}

// TestGateServesInteractiveFirstButNeverStarvesBackground: while background
// fragments wait, interactive frames get three turns in four.
func TestGateServesInteractiveFirstButNeverStarvesBackground(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := newGate()
		if err := g.acquire(t.Context(), false); err != nil { // the turn in progress
			t.Fatal(err)
		}
		var order []string
		waiter(g, true, "lo1", &order)
		synctest.Wait()
		waiter(g, true, "lo2", &order)
		synctest.Wait()
		for _, n := range []string{"hi1", "hi2", "hi3", "hi4", "hi5", "hi6", "hi7"} {
			waiter(g, false, n, &order)
			synctest.Wait()
		}
		g.release()
		synctest.Wait()
		want := []string{"hi1", "hi2", "hi3", "lo1", "hi4", "hi5", "hi6", "lo2", "hi7"}
		if !slices.Equal(order, want) {
			t.Errorf("turns went to %v, want %v", order, want)
		}
	})
}

// TestGateWithoutBackgroundIsFIFO: interactive frames alone keep their order,
// and the quota of background turns does not build up while none waits.
func TestGateWithoutBackgroundIsFIFO(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := newGate()
		if err := g.acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
			waiter(g, false, n, &order)
			synctest.Wait()
		}
		g.release()
		synctest.Wait()
		if want := []string{"a", "b", "c", "d", "e", "f"}; !slices.Equal(order, want) {
			t.Errorf("order = %v, want %v", order, want)
		}
		// A fragment arriving after those: it waits for its own three turns, not
		// fewer because of the frames served before it.
		if err := g.acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
		order = nil
		waiter(g, true, "lo", &order)
		synctest.Wait()
		for _, n := range []string{"h1", "h2", "h3", "h4"} {
			waiter(g, false, n, &order)
			synctest.Wait()
		}
		g.release()
		synctest.Wait()
		if want := []string{"h1", "h2", "h3", "lo", "h4"}; !slices.Equal(order, want) {
			t.Errorf("order = %v, want %v", order, want)
		}
	})
}

// TestGateCanceledWaiterLeavesTheQueue: a waiter whose context ends is
// removed and the turn goes to the next one; a turn granted at the same moment
// is passed on rather than lost.
func TestGateCanceledWaiterLeavesTheQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := newGate()
		if err := g.acquire(t.Context(), false); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		errc := make(chan error, 1)
		go func() { errc <- g.acquire(ctx, false) }()
		synctest.Wait()
		var order []string
		waiter(g, false, "next", &order)
		synctest.Wait()
		cancel()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Errorf("canceled acquire = %v, want context.Canceled", err)
		}
		g.release()
		synctest.Wait()
		if !slices.Equal(order, []string{"next"}) {
			t.Errorf("after the cancellation the turn went to %v, want next", order)
		}
		// The gate is free again: a new frame takes its turn at once.
		if err := g.acquire(t.Context(), true); err != nil {
			t.Errorf("acquire on a free gate = %v", err)
		}
		g.release()
	})
}
