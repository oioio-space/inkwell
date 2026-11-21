package bridge_test

import (
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
