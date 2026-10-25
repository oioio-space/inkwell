package frame_test

import (
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

func TestBudget(t *testing.T) {
	b := frame.NewBudget(100)
	if !b.Acquire(60) {
		t.Fatal("acquire(60) failed")
	}
	got := make(chan bool)
	go func() { got <- b.Acquire(60) }()
	select {
	case <-got:
		t.Fatal("acquire(60) returned before release")
	case <-time.After(50 * time.Millisecond):
	}
	b.Release(60)
	if !<-got {
		t.Error("acquire(60) failed after release")
	}
	go func() { got <- b.Acquire(100) }()
	time.Sleep(20 * time.Millisecond)
	b.Close()
	if <-got {
		t.Error("acquire succeeded on a closed budget")
	}
	b.Release(0)
}
