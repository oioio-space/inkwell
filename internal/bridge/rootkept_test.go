package bridge_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
)

// RT1 rule C-1 (ADR 0020, amendment of 2026-11-25): a stream replaced by a newer page is told so before it is
// closed, and the newer page is not.
func TestRT1C1SupersededToldBeforeClose(t *testing.T) {
	st, _ := newFakeStorage(t)
	srv := bridge.NewStoreServer(st, st, st, quietLogger())
	old := newFakeConn(t)
	oldDone := wait(func() { srv.Serve(old) })
	oc := &storeClient{t: t, conn: old}
	if got := oc.call(bridge.OpLoad, []string{"x"}, ""); got.Status != bridge.StatusNotFound {
		t.Fatalf("first connection not served: %+v", got)
	}

	fresh := newFakeConn(t)
	freshDone := wait(func() { srv.Serve(fresh) })
	// The fake conn refuses a Send once closed: a frame received proves it was sent before the close.
	select {
	case b := <-old.sent:
		got, err := bridge.DecodeStoreResponse(b)
		if err != nil || got.ID != 0 || got.Status != bridge.StatusSuperseded || len(got.Payload) != 0 {
			t.Errorf("C-1 the replaced page got %+v, %v; want the superseded notice (id 0, no payload)", got, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("C-1 the replaced page was closed without the superseded notice")
	}
	<-oldDone
	if old.Context().Err() == nil {
		t.Error("C-1 the replaced page's stream is still open")
	}
	c := &storeClient{t: t, conn: fresh}
	if got := c.call(bridge.OpLoad, []string{"x"}, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("C-1 the newer page is served after its own requests only, got %+v", got)
	}
	fresh.close()
	<-freshDone
}

// RT1 review m2: a third page C arrives while page B still waits for page A's request in flight, longer than C
// would wait for B's turn. B is a live page and
// must be told it is superseded, not closed silently once C's wait for B's turn runs out.
func TestRT1C1ThirdPageTellsTheWaitingOne(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, _ := newFakeStorage(t)
		kv := newHoldKV(st)
		srv := bridge.NewStoreServer(kv, st, st, quietLogger())
		a, aDone := served(t, srv)
		a.send(bridge.OpSave, []string{"hold"}, "a")
		<-kv.entered // A is in the middle of a write
		b, bDone := served(t, srv)
		synctest.Wait() // B attached and waits for A
		c, cDone := served(t, srv)
		time.Sleep(2 * time.Second) // A's write outlasts any bounded wait for B's turn
		kv.unhold()
		synctest.Wait()
		told := func(name string, conn *fakeConn) bool {
			for {
				select {
				case f := <-conn.sent:
					if r, err := bridge.DecodeStoreResponse(f); err == nil && r.ID == 0 && r.Status == bridge.StatusSuperseded {
						return true
					}
				default:
					return false
				}
			}
		}
		if !told("A", a.conn) {
			t.Error("C-1 page A was closed without the superseded notice")
		}
		if !told("B", b.conn) {
			t.Error("C-1 page B, replaced while it waited for A, was closed without the superseded notice")
		}
		<-aDone
		<-bDone
		if got := c.call(bridge.OpLoad, []string{"hold"}, ""); got.Status != bridge.StatusOK || string(got.Payload) != "a" {
			t.Errorf("C-1 page C is served after A's write: %+v", got)
		}
		c.conn.close()
		<-cDone
	})
}

// RT1 review m5 (residual R-RT1-5): a retired page stuck in a Send (it reads nothing, its buffer is full) holds its
// turn; the newer page waits for it at most the bound, then closes it without the notice and is served.
func TestRT1C1StuckPageClosedAfterBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, _ := newFakeStorage(t)
		srv := bridge.NewStoreServer(st, st, st, quietLogger())
		old, oldDone := served(t, srv)
		for range cap(old.conn.sent) + 1 { // the last reply blocks in Send, holding the turn
			old.send(bridge.OpLoad, []string{"absent"}, "")
		}
		synctest.Wait()
		start := time.Now()
		fresh, freshDone := served(t, srv)
		got := fresh.call(bridge.OpLoad, []string{"absent"}, "")
		if waited := time.Since(start); got.Status != bridge.StatusNotFound || waited < time.Second || waited > 2*time.Second {
			t.Errorf("C-1 the newer page was served after %v with %+v; want served after the 1 s bound", waited, got)
		}
		<-oldDone
		for len(old.conn.sent) > 0 {
			if r, err := bridge.DecodeStoreResponse(<-old.conn.sent); err == nil && r.Status == bridge.StatusSuperseded {
				t.Error("C-1 a stuck page cannot have taken the notice")
			}
		}
		fresh.conn.close()
		<-freshDone
	})
}
