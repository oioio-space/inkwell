package bridge_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// holdKV is a [bridge.KV] whose Save of the key "hold" blocks until released, so that a test
// controls the moment a handler is in flight.
type holdKV struct {
	bridge.KV
	entered chan struct{} // closed when the held Save is entered
	release chan struct{} // closed by unhold to let it return
	once    sync.Once
	unheld  sync.Once
	mu      sync.Mutex
	saved   []string // keys saved, in order
}

func newHoldKV(kv bridge.KV) *holdKV {
	return &holdKV{KV: kv, entered: make(chan struct{}), release: make(chan struct{})}
}

func (h *holdKV) Save(key []string, data []byte) error {
	if slices.Equal(key, []string{"hold"}) {
		h.once.Do(func() { close(h.entered) })
		<-h.release
	}
	err := h.KV.Save(key, data)
	h.mu.Lock()
	h.saved = append(h.saved, key[0])
	h.mu.Unlock()
	return err
}

// unhold lets the held Save return; it may be called more than once.
func (h *holdKV) unhold() { h.unheld.Do(func() { close(h.release) }) }

func (h *holdKV) keys() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.saved)
}

// served starts srv on a new connection and returns its client and a channel closed when Serve
// returned.
func served(t *testing.T, srv *bridge.StoreServer) (*storeClient, <-chan struct{}) {
	t.Helper()
	c := &storeClient{t: t, conn: newFakeConn(t)}
	return c, wait(func() { srv.Serve(c.conn) })
}

func ended(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("N-16 streams closed for good: %s still served", what)
	}
}

// TestChangeRequestClosesBridgeStreamsForGood: the drain of a change request ends the open store
// and net streams (their Serve returns, the webview gets its end-of-stream), and every stream
// opened after it is refused at once, until the process ends (rule N-16).
func TestChangeRequestClosesBridgeStreamsForGood(t *testing.T) {
	d := bridge.NewDrain()
	st, _ := newFakeStorage(t)
	srv := bridge.NewStoreServer(st, st, st, quietLogger())
	srv.UseDrain(d)
	router := newRouter(memnet.New(1, memnet.Faults{}).Join("a"), quietLogger())
	router.UseDrain(d)

	store, storeDone := served(t, srv)
	if r := store.call(bridge.OpSave, []string{"before"}, "x"); r.Status != bridge.StatusOK {
		t.Fatalf("save before the drain: %+v", r)
	}
	net := newFakeConn(t)
	netDone := wait(func() { router.Serve(net) })

	if err := d.Drain(t.Context()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	ended(t, storeDone, "the store stream")
	ended(t, netDone, "the net stream")
	if net.ctx.Err() == nil || store.conn.ctx.Err() == nil {
		t.Error("N-16 streams closed for good: a stream was not closed (no end-of-stream for the page)")
	}

	for name, serve := range map[string]func(bridge.Conn){"store": srv.Serve, "net": router.Serve} {
		c := newFakeConn(t)
		done := wait(func() { serve(c) })
		ended(t, done, "a "+name+" stream opened after the drain")
		if c.ctx.Err() == nil {
			t.Errorf("N-16 streams closed for good: a %s stream opened after the drain was not closed", name)
		}
	}
	if _, err := st.Load([]string{"before"}); err != nil {
		t.Errorf("the save taken before the drain is lost: %v", err)
	}
}

// TestStorageWriteBeforeRequestLandsOrIsRefused: a storage write sent just before the change
// request, held at each point (not yet taken by the handler, in the handler, after its reply),
// has either landed in the storage, with its reply sent before the drain returns, or been
// refused; never absent without a refusal; and nothing is written once the drain returned, when
// the request goes on to its write (rule N-16, NoLostWrite of ProjectChange.tla).
func TestStorageWriteBeforeRequestLandsOrIsRefused(t *testing.T) {
	for _, point := range []string{"not taken", "in the handler", "after the reply"} {
		t.Run(point, func(t *testing.T) {
			st, _ := newFakeStorage(t)
			kv := newHoldKV(st)
			d := bridge.NewDrain()
			srv := bridge.NewStoreServer(kv, st, st, quietLogger())
			srv.UseDrain(d)
			c, done := served(t, srv)
			t.Cleanup(func() { kv.unhold(); c.conn.close(); <-done })

			var write uint32
			switch point {
			case "not taken": // the handler is busy with « hold »; « write » waits in the stream
				c.send(bridge.OpSave, []string{"hold"}, "h")
				<-kv.entered
				write = c.send(bridge.OpSave, []string{"write"}, "w")
			case "in the handler":
				write = c.send(bridge.OpSave, []string{"hold"}, "w")
				<-kv.entered
			case "after the reply":
				kv.unhold()
				write = c.send(bridge.OpSave, []string{"write"}, "w")
				if r := c.recv(); r.ID != write || r.Status != bridge.StatusOK {
					t.Fatalf("reply before the request: %+v", r)
				}
			}

			drained := make(chan error, 1)
			go func() { drained <- d.Drain(t.Context()) }()
			if point != "after the reply" {
				// The drain must wait for the handler in flight.
				select {
				case err := <-drained:
					t.Fatalf("N-16 no write before the drain: Drain returned (%v) with a handler in flight", err)
				case <-time.After(50 * time.Millisecond):
				}
				kv.unhold()
			}
			if err := <-drained; err != nil {
				t.Fatalf("Drain: %v", err)
			}
			atDrain := kv.keys() // what the request's write would find: nothing may follow it

			// The replies the page got before its end-of-stream.
			replies := map[uint32]bridge.StoreResponse{}
			for {
				select {
				case b := <-c.conn.sent:
					r, err := bridge.DecodeStoreResponse(b)
					if err != nil {
						t.Fatal(err)
					}
					replies[r.ID] = r
					continue
				default:
				}
				break
			}
			if point == "after the reply" {
				replies[write] = bridge.StoreResponse{ID: write, Status: bridge.StatusOK}
			}

			key := "write"
			if point == "in the handler" {
				key = "hold"
			}
			landed := slices.Contains(atDrain, key)
			r, answered := replies[write]
			switch {
			case landed && (!answered || r.Status != bridge.StatusOK):
				t.Errorf("N-16 no lost write: %q landed without its reply before the drain returned (reply %+v, %v)", key, r, answered)
			case !landed && answered && r.Status == bridge.StatusOK:
				t.Errorf("N-16 no lost write: %q answered OK but absent from the storage", key)
			case !landed && answered && (r.Status != bridge.StatusError || string(r.Payload) != bridge.ReasonLeaving):
				t.Errorf("N-16 no lost write: %q answered %+v, want refused %q", key, r, bridge.ReasonLeaving)
			}
			// Not answered and not landed: the stream ended with it pending, which the page reports
			// as refused (its adapter fails every pending request at the end-of-stream).

			// A write sent after the drain is never applied.
			c.send(bridge.OpSave, []string{"late"}, "l")
			time.Sleep(20 * time.Millisecond)
			if got := kv.keys(); !slices.Equal(got, atDrain) {
				t.Errorf("N-16 no write after the drain: storage writes %v after the drain, %v at it", got, atDrain)
			}
		})
	}
}

// TestDrainTimesOutOnAStuckHandler: past its deadline the drain fails, and the streams stay
// closed and refused all the same.
func TestDrainTimesOutOnAStuckHandler(t *testing.T) {
	d := bridge.NewDrain()
	if !d.Begin() {
		t.Fatal("Begin refused before any drain")
	}
	defer d.End()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := d.Drain(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain with a stuck handler = %v, want the deadline", err)
	}
	if d.Begin() {
		t.Error("N-16 streams closed for good: a request is taken after a failed drain")
	}
}
