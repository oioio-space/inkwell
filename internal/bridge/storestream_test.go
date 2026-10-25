package bridge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
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
	Files      []struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
		Size int64  `json:"size"`
		Main bool   `json:"main"`
	} `json:"files"`
	Info *struct {
		Encoding string `json:"encoding"`
		EOL      string `json:"eol"`
		BOM      bool   `json:"bom"`
		Mixed    bool   `json:"mixed"`
	} `json:"info"`
	Entries []struct {
		Key     []string `json:"key"`
		DataHex string   `json:"dataHex"`
	} `json:"entries"`
	ID uint32 `json:"id"`
}

var storeOps = map[string]bridge.StoreOp{
	"Load": bridge.OpLoad, "Save": bridge.OpSave, "Remove": bridge.OpRemove,
	"LoadRange": bridge.OpLoadRange, "RemoveRange": bridge.OpRemoveRange,
	"WriteProjection": bridge.OpWriteProjection, "ReadProjection": bridge.OpReadProjection,
	"Acknowledge": bridge.OpAcknowledge, "ListFiles": bridge.OpListFiles, "FileInfo": bridge.OpFileInfo,
	"Tree": bridge.OpTree, "Referenced": bridge.OpReferenced, "PutBlob": bridge.OpPutBlob,
}

var fileKinds = map[string]project.Kind{
	"typst": project.KindTypst, "markdown": project.KindMarkdown, "text": project.KindText,
	"csv": project.KindCSV, "bib": project.KindBib, "image": project.KindImage, "pdf": project.KindPDF,
	"data": project.KindData, "other": project.KindOther,
}

var eols = map[string]project.EOL{"LF": project.LF, "CRLF": project.CRLF, "CR": project.CR}

var storeStatuses = map[string]bridge.StoreStatus{
	"OK": bridge.StatusOK, "NotFound": bridge.StatusNotFound, "Corrupt": bridge.StatusCorrupt,
	"Error": bridge.StatusError, "External": bridge.StatusExternal,
	"Binary": bridge.StatusBinary, "InvalidText": bridge.StatusInvalidText, "TooLarge": bridge.StatusTooLarge,
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
	if len(vectors) < 24 {
		t.Fatalf("got %d vectors, want at least 24", len(vectors))
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
			case "filelist":
				var files []project.File
				for _, f := range v.Files {
					files = append(files, project.File{Path: f.Path, Kind: fileKinds[f.Kind], Size: f.Size, Main: f.Main})
				}
				got, err := bridge.EncodeFileList(files)
				if err != nil || !bytes.Equal(got, wire) {
					t.Errorf("EncodeFileList = %x, %v; want %x", got, err, wire)
				}
				dec, err := bridge.DecodeFileList(wire)
				if err != nil || !slices.Equal(dec, files) {
					t.Errorf("DecodeFileList = %+v, %v; want %+v", dec, err, files)
				}
			case "fileinfo":
				info := bridge.FileInfo{Encoding: v.Info.Encoding, EOL: eols[v.Info.EOL], BOM: v.Info.BOM, Mixed: v.Info.Mixed}
				if got := bridge.EncodeFileInfo(info); !bytes.Equal(got, wire) {
					t.Errorf("EncodeFileInfo = %x, want %x", got, wire)
				}
				dec, err := bridge.DecodeFileInfo(wire)
				if err != nil || dec != info {
					t.Errorf("DecodeFileInfo = %+v, %v; want %+v", dec, err, info)
				}
			case "tree", "treeresult", "referenced", "blobinfo":
				checkTreeVector(t, raw, v, wire)
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
		{name: "unknown op 14", wire: []byte{0, 0, 0, 5, 14, 1, 1, 'a'}, id: 5},
		{name: "list files with a key", wire: []byte{0, 0, 0, 5, 9, 1, 1, 'a'}, id: 5},
		{name: "data on list files", wire: []byte{0, 0, 0, 5, 9, 0, 1}, id: 5},
		{name: "file info without name", wire: []byte{0, 0, 0, 5, 10, 0}, id: 5},
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
		{name: "projection with a parent component", wire: []byte{0, 0, 0, 5, 7, 2, 2, '.', '.', 1, 'b'}, id: 5},
		{name: "projection of a reserved name", wire: []byte{0, 0, 0, 5, 7, 2, 1, 'a', 3, 'n', 'u', 'l'}, id: 5},
		{name: "projection under .inkwell", wire: []byte{0, 0, 0, 5, 6, 2, 8, '.', 'i', 'n', 'k', 'w', 'e', 'l', 'l', 1, 'x'}, id: 5},
		{name: "projection with an empty component", wire: []byte{0, 0, 0, 5, 7, 2, 1, 'a', 0}, id: 5},
		{name: "put blob without a path", wire: []byte{0, 0, 0, 5, 13, 0}, id: 5},
		{name: "tree with a key", wire: []byte{0, 0, 0, 5, 11, 1, 1, 'a'}, id: 5},
		{name: "referenced with a key", wire: []byte{0, 0, 0, 5, 12, 1, 1, 'a'}, id: 5},
		{name: "data on put blob", wire: []byte{0, 0, 0, 5, 13, 1, 1, 'a', 9}, id: 5},
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
	files      map[string][]byte
	acked      []string
	trees      [][]project.TreeEntry
	referenced [][bridge.HashSize]byte
	mu         sync.Mutex
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

func (s *fakeStorage) ReadText(rel string) (project.Text, error) {
	switch rel {
	case "binary.typ":
		return project.Text{}, fmt.Errorf("project: %s: %w", rel, project.ErrBinary)
	case "bad.typ":
		return project.Text{}, fmt.Errorf("project: %s: %w", rel, &project.InvalidUTF8Error{Line: 7})
	}
	body, err := s.ReadProjection(rel)
	if err != nil {
		return project.Text{}, err
	}
	return project.Text{Body: string(body), Encoding: project.EncodingWindows1252, EOL: project.Mixed, Dominant: project.CRLF}, nil
}

func (s *fakeStorage) List() ([]project.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []project.File
	for _, name := range slices.Sorted(maps.Keys(s.files)) {
		out = append(out, project.File{Path: name, Kind: project.KindTypst, Size: int64(len(s.files[name])), Main: name == "main.typ"})
	}
	return out, nil
}

func (s *fakeStorage) SetTree(entries []project.TreeEntry) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trees = append(s.trees, slices.Clone(entries))
	rejected := 0
	for _, e := range entries {
		if project.ValidatePath(e.Path) != nil {
			rejected++
		}
	}
	return rejected, nil
}

func (s *fakeStorage) SetReferenced(hashes [][bridge.HashSize]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.referenced = slices.Clone(hashes)
	return nil
}

func (s *fakeStorage) PutBlob(rel string) ([bridge.HashSize]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch rel {
	case "huge.bin":
		return [bridge.HashSize]byte{}, 0, fmt.Errorf("put: %w", &bridge.TooLargeError{Limit: 100 << 20})
	case "missing.bin":
		return [bridge.HashSize]byte{}, 0, &fs.PathError{Op: "open", Path: rel, Err: fs.ErrNotExist}
	}
	return sha256.Sum256([]byte(rel)), int64(len(rel)), nil
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

func TestStoreServerFiles(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	c.call(bridge.OpWriteProjection, []string{"main.typ"}, "= Hi")
	got := c.call(bridge.OpListFiles, nil, "")
	files, err := bridge.DecodeFileList(got.Payload)
	if got.Status != bridge.StatusOK || err != nil || len(files) != 1 || files[0].Path != "main.typ" || !files[0].Main || files[0].Size != 4 {
		t.Errorf("ListFiles = %d %+v, %v", got.Status, files, err)
	}

	got = c.call(bridge.OpFileInfo, []string{"main.typ"}, "")
	info, err := bridge.DecodeFileInfo(got.Payload)
	want := bridge.FileInfo{Encoding: project.EncodingWindows1252, EOL: project.CRLF, Mixed: true}
	if got.Status != bridge.StatusOK || err != nil || info != want {
		t.Errorf("FileInfo = %d %+v, %v; want %+v", got.Status, info, err, want)
	}

	if got := c.call(bridge.OpFileInfo, []string{"absent.typ"}, ""); got.Status != bridge.StatusNotFound {
		t.Errorf("FileInfo(absent) = %d, want NotFound", got.Status)
	}
	if got := c.call(bridge.OpFileInfo, []string{"binary.typ"}, ""); got.Status != bridge.StatusBinary {
		t.Errorf("FileInfo(binary) = %d, want Binary", got.Status)
	}
	got = c.call(bridge.OpFileInfo, []string{"bad.typ"}, "")
	if got.Status != bridge.StatusInvalidText || !bytes.Equal(got.Payload, []byte{0, 0, 0, 7}) {
		t.Errorf("FileInfo(invalid) = %d %x, want InvalidText with line 7", got.Status, got.Payload)
	}
}

func TestDecodeFilesRejects(t *testing.T) {
	for name, wire := range map[string][]byte{
		"empty":               nil,
		"bad version":         {2, 0, 0, 0, 0},
		"count without entry": {1, 0, 0, 0, 1},
		"bad kind":            {1, 0, 0, 0, 1, 99, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 'a'},
		"bad flags":           {1, 0, 0, 0, 1, 1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 1, 'a'},
		"empty path":          {1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"path past the end":   {1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 'a'},
		"invalid UTF-8 path":  {1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0xff},
		"size over int64":     {1, 0, 0, 0, 1, 1, 0, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 1, 'a'},
		"trailing bytes":      {1, 0, 0, 0, 0, 9},
	} {
		if _, err := bridge.DecodeFileList(wire); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeFileList(%s) error = %v, want ErrMalformed", name, err)
		}
	}
	for name, wire := range map[string][]byte{
		"empty":        nil,
		"bad version":  {2, 0, 0, 1, 'a'},
		"bad flags":    {1, 4, 0, 1, 'a'},
		"mixed as eol": {1, 0, 3, 1, 'a'},
		"short name":   {1, 0, 0, 5, 'a'},
		"long name":    {1, 0, 0, 1, 'a', 'b'},
	} {
		if _, err := bridge.DecodeFileInfo(wire); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("DecodeFileInfo(%s) error = %v, want ErrMalformed", name, err)
		}
	}
	if _, err := bridge.EncodeFileList([]project.File{{Path: ""}}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeFileList(empty path) error = %v", err)
	}
	if _, err := bridge.EncodeFileList([]project.File{{Path: "a", Size: -1}}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("EncodeFileList(negative size) error = %v", err)
	}
}

func FuzzDecodeFiles(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 4, 0, 1, 'a'}, true)
	f.Add([]byte{1, 3, 1, 3, 'a', 'b', 'c'}, false)
	f.Fuzz(func(t *testing.T, b []byte, list bool) {
		if list {
			files, err := bridge.DecodeFileList(b)
			if err != nil {
				if !errors.Is(err, bridge.ErrMalformed) {
					t.Fatalf("DecodeFileList(%x) error %v is not ErrMalformed", b, err)
				}
				return
			}
			if again, err := bridge.EncodeFileList(files); err != nil || !bytes.Equal(again, b) {
				t.Fatalf("file list round trip of %x gave %x, %v", b, again, err)
			}
			return
		}
		info, err := bridge.DecodeFileInfo(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeFileInfo(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		if again := bridge.EncodeFileInfo(info); !bytes.Equal(again, b) {
			t.Fatalf("file info round trip of %x gave %x", b, again)
		}
	})
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
