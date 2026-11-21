package bridge_test

import (
	"slices"
	"testing"
	"testing/synctest"

	"github.com/oioio-space/inkwell/internal/bridge"
)

type reloadStoreConn struct {
	*fakeConn
	release <-chan struct{}
}

func (c reloadStoreConn) Receive() ([]byte, error) {
	frame, err := c.fakeConn.Receive()
	if err == nil {
		<-c.release // the frame has already left the transport's inbox
	}
	return frame, err
}

// The old stream can return a frame from Receive while its replacement attaches.
// A response to the retired page is immaterial. The new page must see the write.
func TestWC1W5StoreAfterReload(t *testing.T) {
	t.Run("ReceivedBeforeReplacement", testWC1ReceivedBeforeReplacement)
	t.Run("BlockedWriteAndAbandonedReply", testWC1BlockedWriteAndAbandonedReply)
}

func testWC1ReceivedBeforeReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, _ := newFakeStorage(t)
		server := bridge.NewStoreServer(st, st, st, quietLogger())
		old := &storeClient{t: t, conn: newFakeConn(t)}
		release := make(chan struct{})
		oldDone := wait(func() { server.Serve(reloadStoreConn{old.conn, release}) })
		key := []string{"doc", "snapshot", "before-reload"}
		old.send(bridge.OpSave, key, "last keystroke")
		synctest.Wait()

		fresh := &storeClient{t: t, conn: newFakeConn(t)}
		freshDone := wait(func() { server.Serve(fresh.conn) })
		synctest.Wait()

		close(release)
		synctest.Wait()
		old.conn.close()
		<-oldDone

		reply := fresh.call(bridge.OpLoad, key, "")
		fresh.conn.close()
		<-freshDone
		if reply.Status != bridge.StatusOK || string(reply.Payload) != "last keystroke" {
			t.Fatalf("W-5: the replacement page lost a write Go received from the old stream (status %d)", reply.Status)
		}
	})
}

func testWC1BlockedWriteAndAbandonedReply(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, _ := newFakeStorage(t)
		kv := newHoldKV(st)
		server := bridge.NewStoreServer(kv, st, st, quietLogger())
		old, oldDone := served(t, server)
		// Fill the old reply buffer before the held save. No reader will free it.
		for range cap(old.conn.sent) {
			old.send(bridge.OpLoad, []string{"absent"}, "")
		}
		old.send(bridge.OpSave, []string{"hold"}, "old")
		<-kv.entered

		fresh, freshDone := served(t, server)
		fresh.send(bridge.OpLoad, []string{"hold"}, "")
		fresh.send(bridge.OpSave, []string{"hold"}, "new")
		fresh.send(bridge.OpLoad, []string{"hold"}, "")
		synctest.Wait()
		early := len(fresh.conn.sent)
		kv.unhold()
		synctest.Wait()
		select {
		case <-oldDone:
		default:
			t.Error("W-5: retired handler is still waiting for the abandoned page")
			old.conn.close()
			<-oldDone
		}
		if early != 0 {
			t.Errorf("W-5: replacement answered %d requests before the previous write landed", early)
		}
		for i, value := range []string{"old", "", "new"} {
			got := fresh.recv()
			if got.ID != uint32(i+1) || got.Status != bridge.StatusOK || string(got.Payload) != value {
				t.Errorf("W-5: FIFO reply %d = %+v, want %q", i+1, got, value)
			}
		}
		if keys := kv.keys(); !slices.Equal(keys, []string{"hold", "hold"}) {
			t.Errorf("W-5: expected each save to run once, got %v", keys)
		}
		fresh.conn.close()
		<-freshDone
	})
}
