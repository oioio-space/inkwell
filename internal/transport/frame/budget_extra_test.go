package frame_test

import (
	"testing"
	"testing/synctest"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// acquireAsync runs Acquire(n) in a goroutine and reports its result on the
// returned channel.
func acquireAsync(b *frame.Budget, n int) <-chan bool {
	c := make(chan bool, 1)
	go func() { c <- b.Acquire(n) }()
	return c
}

func settled(c <-chan bool) (result, done bool) {
	synctest.Wait()
	select {
	case r := <-c:
		return r, true
	default:
		return false, false
	}
}

// TestBudgetExactAccounting: acquiring exactly what is left succeeds at once,
// one more byte blocks, and the bytes are only available again once released.
func TestBudgetExactAccounting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := frame.NewBudget(100)
		defer b.Close()
		if !b.Acquire(100) {
			t.Fatal("Acquire(100) of a 100-byte budget failed")
		}
		c := acquireAsync(b, 1)
		if _, done := settled(c); done {
			t.Fatal("Acquire(1) returned on an exhausted budget")
		}
		b.Release(1)
		if ok, done := settled(c); !done || !ok {
			t.Fatalf("Acquire(1) after Release(1) = %t, %t; want true, true", ok, done)
		}
		if _, done := settled(acquireAsync(b, 1)); done {
			t.Fatal("budget handed out more than its size")
		}
	})
}

// TestBudgetPartialReleaseKeepsWaiting: a waiter needing more than what was
// released stays blocked until enough comes back.
func TestBudgetPartialReleaseKeepsWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := frame.NewBudget(100)
		defer b.Close()
		b.Acquire(100)
		c := acquireAsync(b, 60)
		b.Release(59)
		if _, done := settled(c); done {
			t.Fatal("Acquire(60) woke with 59 bytes available")
		}
		b.Release(1)
		if ok, done := settled(c); !done || !ok {
			t.Fatalf("Acquire(60) with 60 bytes available = %t, %t", ok, done)
		}
	})
}

func TestBudgetAcquireZero(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := frame.NewBudget(10)
		defer b.Close()
		b.Acquire(10)
		if ok, done := settled(acquireAsync(b, 0)); !done || !ok {
			t.Errorf("Acquire(0) on an exhausted budget = %t, %t; want immediate true", ok, done)
		}
	})
}

// TestBudgetCloseWakesAll: Close releases every waiter with false, and every
// later Acquire fails even when bytes are free.
func TestBudgetCloseWakesAll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := frame.NewBudget(10)
		b.Acquire(10)
		waiters := []<-chan bool{acquireAsync(b, 5), acquireAsync(b, 10)}
		synctest.Wait()
		b.Close()
		for i, c := range waiters {
			if ok, done := settled(c); !done || ok {
				t.Errorf("waiter %d after Close = %t, %t; want false, true", i, ok, done)
			}
		}
		b.Release(10)
		if ok, done := settled(acquireAsync(b, 1)); !done || ok {
			t.Errorf("Acquire on a closed budget with free bytes = %t, %t; want false", ok, done)
		}
		b.Close() // idempotent
	})
}

// TestBudgetWakesEveryFittingWaiter: one Release serving two small waiters
// wakes both (Broadcast, not Signal).
func TestBudgetWakesEveryFittingWaiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := frame.NewBudget(20)
		defer b.Close()
		b.Acquire(20)
		a, c := acquireAsync(b, 10), acquireAsync(b, 10)
		synctest.Wait()
		b.Release(20)
		for i, ch := range []<-chan bool{a, c} {
			if ok, done := settled(ch); !done || !ok {
				t.Errorf("waiter %d = %t, %t; want true after Release(20)", i, ok, done)
			}
		}
	})
}
