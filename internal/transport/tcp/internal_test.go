package tcp

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// flakyListener fails its first Accept calls with a non-closed error, then
// blocks until closed.
type flakyListener struct {
	closed  chan struct{}
	once    sync.Once
	calls   atomic.Int32
	failFor int32
}

func (l *flakyListener) Accept() (net.Conn, error) {
	if l.calls.Add(1) <= l.failFor {
		return nil, errors.New("accept: too many open files")
	}
	<-l.closed
	return nil, net.ErrClosed
}
func (l *flakyListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *flakyListener) Addr() net.Addr { return &net.TCPAddr{} }

func TestAcceptLoopRetriesTransientErrors(t *testing.T) {
	n := New("a", Options{})
	t.Cleanup(func() { _ = n.Close() })
	ln := &flakyListener{closed: make(chan struct{}), failFor: 3}
	n.wg.Go(func() { n.acceptLoop(t.Context(), ln) })
	deadline := time.Now().Add(5 * time.Second)
	for ln.calls.Load() < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("Accept called %d times, want the loop to retry past 3 errors", ln.calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
	_ = ln.Close()
}

func TestAcceptLoopStopsOnContext(t *testing.T) {
	n := New("a", Options{})
	ln := &flakyListener{closed: make(chan struct{}), failFor: 1 << 30}
	ctx, cancel := context.WithCancel(t.Context())
	n.wg.Go(func() { n.acceptLoop(ctx, ln) })
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := n.Close(); err != nil { // waits for acceptLoop to return
		t.Fatal(err)
	}
}

func TestBudget(t *testing.T) {
	b := newBudget(100)
	if !b.acquire(60) {
		t.Fatal("acquire(60) failed")
	}
	got := make(chan bool)
	go func() { got <- b.acquire(60) }()
	select {
	case <-got:
		t.Fatal("acquire(60) returned before release")
	case <-time.After(50 * time.Millisecond):
	}
	b.release(60)
	if !<-got {
		t.Error("acquire(60) failed after release")
	}
	go func() { got <- b.acquire(100) }()
	time.Sleep(20 * time.Millisecond)
	b.close()
	if <-got {
		t.Error("acquire succeeded on a closed budget")
	}
	b.release(0)
}
