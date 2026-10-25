package bridge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/store"
)

// storeVector is one golden case of testdata/storestream.json, shared with
// the TypeScript implementation.
type storeVector struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Op         string `json:"op"`
	Status     string `json:"status"`
	DataHex    string `json:"dataHex"`
	PayloadHex string `json:"payloadHex"`
	WireHex    string `json:"wireHex"`
	Key        []string
	Entries    []struct {
		Key     []string `json:"key"`
		DataHex string   `json:"dataHex"`
	} `json:"entries"`
	ID uint32 `json:"id"`
}

var storeOps = map[string]bridge.StoreOp{
	"Load": bridge.OpLoad, "Save": bridge.OpSave, "Remove": bridge.OpRemove,
	"LoadRange": bridge.OpLoadRange, "RemoveRange": bridge.OpRemoveRange,
	"WriteProjection": bridge.OpWriteProjection, "ReadProjection": bridge.OpReadProjection,
	"Acknowledge": bridge.OpAcknowledge,
}

var storeStatuses = map[string]bridge.StoreStatus{
	"OK": bridge.StatusOK, "NotFound": bridge.StatusNotFound, "Corrupt": bridge.StatusCorrupt,
	"Error": bridge.StatusError, "External": bridge.StatusExternal,
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStoreVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/storestream.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []storeVector
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) < 8 {
		t.Fatalf("got %d vectors, want at least 8", len(vectors))
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			wire := unhex(t, v.WireHex)
			switch v.Type {
			case "request":
				req := bridge.StoreRequest{ID: v.ID, Op: storeOps[v.Op], Key: v.Key, Data: unhex(t, v.DataHex)}
				got, err := bridge.EncodeStoreRequest(req)
				if err != nil || !bytes.Equal(got, wire) {
					t.Errorf("EncodeStoreRequest = %x, %v; want %x", got, err, wire)
				}
				dec, err := bridge.DecodeStoreRequest(wire)
				if err != nil {
					t.Fatalf("DecodeStoreRequest(%x): %v", wire, err)
				}
				if dec.ID != req.ID || dec.Op != req.Op || !slices.Equal(dec.Key, req.Key) || !bytes.Equal(dec.Data, req.Data) {
					t.Errorf("DecodeStoreRequest = %+v, want %+v", dec, req)
				}
			case "response":
				resp := bridge.StoreResponse{ID: v.ID, Status: storeStatuses[v.Status], Payload: unhex(t, v.PayloadHex)}
				got, err := bridge.EncodeStoreResponse(resp)
				if err != nil || !bytes.Equal(got, wire) {
					t.Errorf("EncodeStoreResponse = %x, %v; want %x", got, err, wire)
				}
				dec, err := bridge.DecodeStoreResponse(wire)
				if err != nil || dec.ID != resp.ID || dec.Status != resp.Status || !bytes.Equal(dec.Payload, resp.Payload) {
					t.Errorf("DecodeStoreResponse = %+v, %v; want %+v", dec, err, resp)
				}
			case "range":
				var entries []store.Entry
				for _, e := range v.Entries {
					entries = append(entries, store.Entry{Key: e.Key, Data: unhex(t, e.DataHex)})
				}
				got, err := bridge.EncodeRange(entries)
				if err != nil || !bytes.Equal(got, wire) {
					t.Errorf("EncodeRange = %x, %v; want %x", got, err, wire)
				}
				dec, err := bridge.DecodeRange(wire)
				if err != nil || len(dec) != len(entries) || !slices.Equal(dec[0].Key, entries[0].Key) || !bytes.Equal(dec[0].Data, entries[0].Data) {
					t.Errorf("DecodeRange = %+v, %v; want %+v", dec, err, entries)
				}
			default:
				t.Fatalf("unknown vector type %q", v.Type)
			}
		})
	}
}

func TestDecodeStoreRequestRejects(t *testing.T) {
	longComp := append([]byte{0, 0, 0, 1, byte(bridge.OpLoad), 1, 129}, strings.Repeat("a", 129)...)
	tests := []struct {
		name string
		wire []byte
		id   uint32 // id still reported
	}{
		{name: "empty"},
		{name: "short header", wire: []byte{0, 0, 0, 1, 1}},
		{name: "id zero", wire: []byte{0, 0, 0, 0, 1, 1, 1, 'a'}},
		{name: "unknown op 0", wire: []byte{0, 0, 0, 5, 0, 1, 1, 'a'}, id: 5},
		{name: "unknown op 9", wire: []byte{0, 0, 0, 5, 9, 1, 1, 'a'}, id: 5},
		{name: "no key count", wire: []byte{0, 0, 0, 5, 1}},
		{name: "five components", wire: []byte{0, 0, 0, 5, 1, 5, 1, 'a', 1, 'b', 1, 'c', 1, 'd', 1, 'e'}, id: 5},
		{name: "truncated component", wire: []byte{0, 0, 0, 5, 1, 1, 4, 'a'}, id: 5},
		{name: "empty component", wire: []byte{0, 0, 0, 5, 1, 1, 0}, id: 5},
		{name: "component too long", wire: longComp, id: 1},
		{name: "dot dot", wire: []byte{0, 0, 0, 5, 1, 1, 2, '.', '.'}, id: 5},
		{name: "slash", wire: []byte{0, 0, 0, 5, 1, 1, 3, 'a', '/', 'b'}, id: 5},
		{name: "load without key", wire: []byte{0, 0, 0, 5, 1, 0}, id: 5},
		{name: "remove range without key", wire: []byte{0, 0, 0, 5, 5, 0}, id: 5},
		{name: "data on load", wire: []byte{0, 0, 0, 5, 1, 1, 1, 'a', 9}, id: 5},
		{name: "data on load range", wire: []byte{0, 0, 0, 5, 4, 0, 9}, id: 5},
		{name: "projection with two components", wire: []byte{0, 0, 0, 5, 7, 2, 1, 'a', 1, 'b'}, id: 5},
		{name: "projection without name", wire: []byte{0, 0, 0, 5, 7, 0}, id: 5},
		{name: "oversized", wire: append([]byte{0, 0, 0, 5, 2, 1, 1, 'a'}, make([]byte, store.MaxValue+1)...), id: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := bridge.DecodeStoreRequest(tt.wire)
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeStoreRequest(%x) error = %v, want ErrMalformed", tt.wire[:min(len(tt.wire), 16)], err)
			}
			if req.ID != tt.id {
				t.Errorf("reported id = %d, want %d", req.ID, tt.id)
			}
		})
	}
}

func TestEncodeStoreRequestRejects(t *testing.T) {
	for name, req := range map[string]bridge.StoreRequest{
		"id zero":   {Op: bridge.OpLoad, Key: []string{"a"}},
		"bad op":    {ID: 1, Op: 99, Key: []string{"a"}},
		"bad key":   {ID: 1, Op: bridge.OpLoad, Key: []string{"a/b"}},
		"data":      {ID: 1, Op: bridge.OpRemove, Key: []string{"a"}, Data: []byte{1}},
		"too large": {ID: 1, Op: bridge.OpSave, Key: []string{"a"}, Data: make([]byte, store.MaxValue+1)},
	} {
		if _, err := bridge.EncodeStoreRequest(req); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("%s: EncodeStoreRequest error = %v, want ErrMalformed", name, err)
		}
	}
	if _, err := bridge.EncodeStoreResponse(bridge.StoreResponse{Status: 99}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeStoreResponse(status 99) error = %v", err)
	}
	if _, err := bridge.DecodeStoreResponse([]byte{0, 0, 0, 1, 99}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeStoreResponse(status 99) error = %v", err)
	}
	if _, err := bridge.DecodeStoreResponse([]byte{0, 0}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeStoreResponse(short) error = %v", err)
	}
}

func TestDecodeRangeRejects(t *testing.T) {
	for name, wire := range map[string][]byte{
		"empty":              nil,
		"count without data": {0, 0, 0, 1},
		"truncated key":      {0, 0, 0, 1, 2, 1, 'a'},
		"truncated length":   {0, 0, 0, 1, 1, 1, 'a', 0, 0},
		"data past the end":  {0, 0, 0, 1, 1, 1, 'a', 0, 0, 0, 9, 1},
		"huge data length":   {0, 0, 0, 1, 1, 1, 'a', 0xff, 0xff, 0xff, 0xff},
		"trailing bytes":     {0, 0, 0, 0, 7},
	} {
		if _, err := bridge.DecodeRange(wire); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("%s: DecodeRange error = %v, want ErrMalformed", name, err)
		}
	}
}

func FuzzStoreRequest(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, 1, 2, 3, 'd', 'o', 'c', 1, 'k'})
	f.Add([]byte{0, 0, 1, 2, 2, 1, 1, 'a', 0, 0xff})
	f.Add([]byte{0, 0, 0, 7, 4, 0})
	f.Add([]byte{0, 0, 0, 3, 6, 1, 8, 'm', 'a', 'i', 'n', '.', 't', 'y', 'p', 1})
	f.Add([]byte{0, 0, 0, 0, 1, 1, 1, 'a'})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		req, err := bridge.DecodeStoreRequest(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeStoreRequest(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		again, err := bridge.EncodeStoreRequest(req)
		if err != nil {
			t.Fatalf("EncodeStoreRequest(DecodeStoreRequest(%x)): %v", b, err)
		}
		if !bytes.Equal(again, b) {
			t.Fatalf("round trip of %x gave %x", b, again)
		}
	})
}

func FuzzDecodeRange(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, 2, 1, 'd', 1, 'k', 0, 0, 0, 1, 'v'})
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, b []byte) {
		entries, err := bridge.DecodeRange(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeRange(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		again, err := bridge.EncodeRange(entries)
		if err != nil || !bytes.Equal(again, b) {
			t.Fatalf("round trip of %x gave %x, %v", b, again, err)
		}
	})
}

// fakeStorage is a real [store.KV] plus an in-memory set of project files.
type fakeStorage struct {
	*store.KV
	files map[string][]byte
	acked []string
	mu    sync.Mutex
}

func newFakeStorage(t *testing.T) (*fakeStorage, *os.Root) {
	t.Helper()
	r, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	kv, err := store.Open(r)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeStorage{KV: kv, files: map[string][]byte{}}, r
}

func (s *fakeStorage) WriteProjection(rel string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rel == "locked.typ" {
		return errors.New("edited outside Inkwell")
	}
	s.files[rel] = bytes.Clone(data)
	return nil
}

func (s *fakeStorage) ReadProjection(rel string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.files[rel]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: rel, Err: fs.ErrNotExist}
	}
	return data, nil
}

func (s *fakeStorage) Acknowledge(rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acked = append(s.acked, rel)
	return nil
}

// storeClient plays the webview on the store stream.
type storeClient struct {
	t    *testing.T
	conn *fakeConn
	next uint32
}

func (c *storeClient) send(op bridge.StoreOp, key []string, data string) uint32 {
	c.t.Helper()
	c.next++
	b, err := bridge.EncodeStoreRequest(bridge.StoreRequest{ID: c.next, Op: op, Key: key, Data: []byte(data)})
	if err != nil {
		c.t.Fatal(err)
	}
	c.conn.push(b)
	return c.next
}

func (c *storeClient) recv() bridge.StoreResponse {
	c.t.Helper()
	resp, err := bridge.DecodeStoreResponse(<-c.conn.sent)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp
}

// call sends a request and returns its response.
func (c *storeClient) call(op bridge.StoreOp, key []string, data string) bridge.StoreResponse {
	c.t.Helper()
	id := c.send(op, key, data)
	resp := c.recv()
	if resp.ID != id {
		c.t.Fatalf("response id = %d, want %d", resp.ID, id)
	}
	return resp
}

func startStore(t *testing.T, st bridge.Storage) (*bridge.StoreServer, *storeClient, func()) {
	t.Helper()
	srv := bridge.NewStoreServer(st, quietLogger())
	c := &storeClient{t: t, conn: newFakeConn(t)}
	done := wait(func() { srv.Serve(c.conn) })
	return srv, c, func() { c.conn.close(); <-done }
}

func TestStoreServerOperations(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	key := []string{"doc", "snapshot", "h"}
	if got := c.call(bridge.OpLoad, key, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("Load(absent) status = %d, want NotFound", got.Status)
	}
	if got := c.call(bridge.OpSave, key, "hello"); got.Status != bridge.StatusOK {
		t.Fatalf("Save status = %d (%s)", got.Status, got.Payload)
	}
	if got := c.call(bridge.OpLoad, key, ""); got.Status != bridge.StatusOK || string(got.Payload) != "hello" {
		t.Errorf("Load = %d %q, want OK hello", got.Status, got.Payload)
	}
	inc := sha256.Sum256([]byte("chunk"))
	c.call(bridge.OpSave, []string{"doc", "incremental", hex.EncodeToString(inc[:])}, "chunk")

	got := c.call(bridge.OpLoadRange, []string{"doc"}, "")
	entries, err := bridge.DecodeRange(got.Payload)
	if got.Status != bridge.StatusOK || err != nil || len(entries) != 2 {
		t.Fatalf("LoadRange = %d, %d entries, %v; want 2 entries", got.Status, len(entries), err)
	}
	if got := c.call(bridge.OpRemoveRange, []string{"doc", "incremental"}, ""); got.Status != bridge.StatusOK {
		t.Errorf("RemoveRange status = %d", got.Status)
	}
	if got := c.call(bridge.OpRemove, key, ""); got.Status != bridge.StatusOK {
		t.Errorf("Remove status = %d", got.Status)
	}
	if got := c.call(bridge.OpLoadRange, nil, ""); string(got.Payload) != "\x00\x00\x00\x00" {
		t.Errorf("LoadRange(all) after removals = %x, want an empty range", got.Payload)
	}

	// Storage rejections become StatusError with a reason, not a closed stream.
	bad := c.call(bridge.OpSave, []string{"doc", "incremental", "nothash"}, "chunk")
	if bad.Status != bridge.StatusError || len(bad.Payload) == 0 {
		t.Errorf("Save(bad incremental key) = %d %q, want Error with a reason", bad.Status, bad.Payload)
	}
}

func TestStoreServerProjection(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	if got := c.call(bridge.OpReadProjection, []string{"main.typ"}, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("ReadProjection(absent) = %d, want NotFound", got.Status)
	}
	if got := c.call(bridge.OpWriteProjection, []string{"main.typ"}, "= Hi"); got.Status != bridge.StatusOK {
		t.Fatalf("WriteProjection = %d %s", got.Status, got.Payload)
	}
	if got := c.call(bridge.OpReadProjection, []string{"main.typ"}, ""); string(got.Payload) != "= Hi" {
		t.Errorf("ReadProjection = %q", got.Payload)
	}
	if got := c.call(bridge.OpWriteProjection, []string{"locked.typ"}, "x"); got.Status != bridge.StatusError {
		t.Errorf("WriteProjection(locked) = %d, want Error", got.Status)
	}
	if got := c.call(bridge.OpAcknowledge, []string{"main.typ"}, ""); got.Status != bridge.StatusOK {
		t.Errorf("Acknowledge = %d", got.Status)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !slices.Equal(st.acked, []string{"main.typ"}) {
		t.Errorf("acknowledged = %v", st.acked)
	}
}

// TestStoreServerSequential: requests are answered in order, so a Load queued
// right behind a Save sees it.
func TestStoreServerSequential(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	key := []string{"k"}
	var ids []uint32
	for _, v := range []string{"one", "two", "three"} {
		ids = append(ids, c.send(bridge.OpSave, key, v), c.send(bridge.OpLoad, key, ""))
	}
	for i, id := range ids {
		resp := c.recv()
		if resp.ID != id {
			t.Fatalf("response %d has id %d, want %d (out of order)", i, resp.ID, id)
		}
		if i%2 == 1 && string(resp.Payload) != []string{"one", "two", "three"}[i/2] {
			t.Errorf("Load #%d = %q: read-after-write broken", i/2, resp.Payload)
		}
	}
}

func TestStoreServerReportsCorruption(t *testing.T) {
	st, root := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	key := []string{"d", "snapshot", "h"}
	c.call(bridge.OpSave, key, "value")
	name := "repo/d/snapshot/h~"
	b, err := root.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)-1] ^= 1
	if err := root.WriteFile(name, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.call(bridge.OpLoad, key, ""); got.Status != bridge.StatusCorrupt || len(got.Payload) != 0 {
		t.Errorf("Load(corrupted) = %d %q, want Corrupt without payload", got.Status, got.Payload)
	}
	if got := c.call(bridge.OpLoad, key, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("Load after quarantine = %d, want NotFound", got.Status)
	}
}

func TestStoreServerSkipsMalformed(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	c.conn.push([]byte{1, 2}) // no id: dropped silently
	c.conn.push([]byte{0, 0, 0, 77, 1, 1, 2, '.', '.'})
	if got := c.recv(); got.ID != 77 || got.Status != bridge.StatusError {
		t.Errorf("answer to a bad key = %+v, want Error for id 77", got)
	}
	if got := c.call(bridge.OpLoad, []string{"x"}, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("server did not recover after malformed frames: %d", got.Status)
	}
}

func TestStoreServerExternalNotices(t *testing.T) {
	st, _ := newFakeStorage(t)
	srv := bridge.NewStoreServer(st, quietLogger())

	srv.NotifyExternal("main.typ") // no webview yet: remembered

	conn := newFakeConn(t)
	c := &storeClient{t: t, conn: conn}
	done := wait(func() { srv.Serve(conn) })
	if got := c.recv(); got.ID != 0 || got.Status != bridge.StatusExternal || string(got.Payload) != "main.typ" {
		t.Fatalf("pending notice on attach = %+v", got)
	}

	srv.NotifyExternal("main.typ") // live push
	if got := c.recv(); got.Status != bridge.StatusExternal || string(got.Payload) != "main.typ" {
		t.Fatalf("live notice = %+v", got)
	}

	// A reloaded page gets the notice again until it acknowledges.
	conn.close()
	<-done
	conn2 := newFakeConn(t)
	c2 := &storeClient{t: t, conn: conn2}
	done2 := wait(func() { srv.Serve(conn2) })
	if got := c2.recv(); got.Status != bridge.StatusExternal {
		t.Fatalf("notice after reload = %+v", got)
	}
	if got := c2.call(bridge.OpAcknowledge, []string{"main.typ"}, ""); got.Status != bridge.StatusOK {
		t.Fatalf("Acknowledge = %d", got.Status)
	}
	conn2.close()
	<-done2

	conn3 := newFakeConn(t)
	c3 := &storeClient{t: t, conn: conn3}
	done3 := wait(func() { srv.Serve(conn3) })
	if got := c3.call(bridge.OpLoad, []string{"x"}, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("first answer after the acknowledged notice = %+v, want only the response", got)
	}
	conn3.close()
	<-done3
}

func TestStoreServerReplacesConnection(t *testing.T) {
	st, _ := newFakeStorage(t)
	srv := bridge.NewStoreServer(st, quietLogger())
	old := newFakeConn(t)
	oldDone := wait(func() { srv.Serve(old) })
	oc := &storeClient{t: t, conn: old}
	if got := oc.call(bridge.OpLoad, []string{"x"}, ""); got.Status != bridge.StatusNotFound {
		t.Fatalf("first connection not served: %+v", got)
	}

	fresh := newFakeConn(t)
	c := &storeClient{t: t, conn: fresh}
	freshDone := wait(func() { srv.Serve(fresh) })
	// Once the new connection is answered it is the attached one. The replaced
	// connection's loop then ends at its next frame.
	if got := c.call(bridge.OpLoad, []string{"x"}, ""); got.Status != bridge.StatusNotFound {
		t.Fatalf("new connection not served: %+v", got)
	}
	old.push([]byte{0, 0, 0, 1, 1, 1, 1, 'a'})
	<-oldDone
	fresh.close()
	<-freshDone
}

func TestStoreServerRangeTooLarge(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()
	chunk := strings.Repeat("x", 20<<20)
	for _, k := range []string{"a", "b", "c", "d"} {
		if got := c.call(bridge.OpSave, []string{"doc", k}, chunk); got.Status != bridge.StatusOK {
			t.Fatalf("Save %s = %d", k, got.Status)
		}
	}
	// 80 MiB cannot travel in one 64 MiB frame: the request fails cleanly.
	if got := c.call(bridge.OpLoadRange, []string{"doc"}, ""); got.Status != bridge.StatusError {
		t.Errorf("LoadRange(80 MiB) = %d, want Error", got.Status)
	}
	if got := c.call(bridge.OpLoad, []string{"doc", "a"}, ""); got.Status != bridge.StatusOK || len(got.Payload) != len(chunk) {
		t.Errorf("server unusable after an oversized range: %d, %d bytes", got.Status, len(got.Payload))
	}
}
