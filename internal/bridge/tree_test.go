package bridge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
)

// treeVector is a golden case about the tree payloads of testdata/storestream.json.
type treeVector struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Hash    string `json:"hash"`
	Entries []struct {
		Path string `json:"path"`
		Type string `json:"type"`
		URL  string `json:"url"`
		Hash string `json:"hash"`
		Size int64  `json:"size"`
		Main bool   `json:"main"`
	} `json:"entries"`
	Hashes   []string `json:"hashes"`
	Size     int64    `json:"size"`
	Rejected int      `json:"rejected"`
}

var nodeTypes = map[string]project.NodeType{"dir": project.NodeDir, "text": project.NodeText, "blob": project.NodeBlob}

func hashFromHex(t *testing.T, s string) [bridge.HashSize]byte {
	t.Helper()
	var h [bridge.HashSize]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(h) {
		t.Fatalf("hash %q: %v", s, err)
	}
	copy(h[:], b)
	return h
}

// checkTreeVector checks the golden vector v, a tree, tree result, referenced
// list or blob info payload, against the encoders and decoders.
func checkTreeVector(t *testing.T, raw []byte, v storeVector, wire []byte) {
	t.Helper()
	var all []treeVector
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	var tv treeVector
	for _, c := range all {
		if c.Name == v.Name {
			tv = c
		}
	}
	switch v.Type {
	case "tree":
		var entries []project.TreeEntry
		for _, e := range tv.Entries {
			te := project.TreeEntry{Path: e.Path, Type: nodeTypes[e.Type], URL: e.URL, Size: e.Size, Main: e.Main}
			if e.Hash != "" {
				te.Hash = hashFromHex(t, e.Hash)
			}
			entries = append(entries, te)
		}
		got, err := bridge.EncodeTree(entries)
		if err != nil || !bytes.Equal(got, wire) {
			t.Errorf("EncodeTree = %x, %v; want %x", got, err, wire)
		}
		dec, err := bridge.DecodeTree(wire)
		if err != nil || len(dec) != len(entries) {
			t.Fatalf("DecodeTree = %+v, %v; want %+v", dec, err, entries)
		}
		for i := range dec {
			if dec[i] != entries[i] {
				t.Errorf("DecodeTree entry %d = %+v, want %+v", i, dec[i], entries[i])
			}
		}
	case "treeresult":
		if got := bridge.EncodeTreeResult(tv.Rejected); !bytes.Equal(got, wire) {
			t.Errorf("EncodeTreeResult = %x, want %x", got, wire)
		}
		if n, err := bridge.DecodeTreeResult(wire); err != nil || n != tv.Rejected {
			t.Errorf("DecodeTreeResult = %d, %v; want %d", n, err, tv.Rejected)
		}
	case "referenced":
		var hashes [][bridge.HashSize]byte
		for _, h := range tv.Hashes {
			hashes = append(hashes, hashFromHex(t, h))
		}
		got, err := bridge.EncodeReferenced(hashes)
		if err != nil || !bytes.Equal(got, wire) {
			t.Errorf("EncodeReferenced = %x, %v; want %x", got, err, wire)
		}
		dec, err := bridge.DecodeReferenced(wire)
		if err != nil || len(dec) != len(hashes) || dec[0] != hashes[0] || dec[1] != hashes[1] {
			t.Errorf("DecodeReferenced = %x, %v; want %x", dec, err, hashes)
		}
	case "blobinfo":
		h := hashFromHex(t, tv.Hash)
		if got := bridge.EncodeBlobInfo(h, tv.Size); !bytes.Equal(got, wire) {
			t.Errorf("EncodeBlobInfo = %x, want %x", got, wire)
		}
		if gh, gs, err := bridge.DecodeBlobInfo(wire); err != nil || gh != h || gs != tv.Size {
			t.Errorf("DecodeBlobInfo = %x, %d, %v; want %x, %d", gh, gs, err, h, tv.Size)
		}
	}
}

func goodTree() []project.TreeEntry {
	return []project.TreeEntry{
		{Path: "main.typ", Type: project.NodeText, URL: "automerge:abc", Main: true},
		{Path: "img", Type: project.NodeDir},
		{Path: "img/a.png", Type: project.NodeBlob, Hash: sha256.Sum256([]byte("a")), Size: 1},
	}
}

func TestDecodeTreeRejects(t *testing.T) {
	good, err := bridge.EncodeTree(goodTree())
	if err != nil {
		t.Fatal(err)
	}
	u32 := func(n uint32) []byte { return binary.BigEndian.AppendUint32(nil, n) }
	one := func(typ, flags byte, size uint64, path string, ref []byte) []byte {
		b := append([]byte{1}, u32(1)...)
		b = append(b, typ, flags)
		b = binary.BigEndian.AppendUint64(b, size)
		b = binary.BigEndian.AppendUint16(b, uint16(len(path)))
		b = append(b, path...)
		b = append(b, byte(len(ref)))
		return append(b, ref...)
	}
	hash := bytes.Repeat([]byte{1}, 32)
	tests := []struct {
		name string
		wire []byte
	}{
		{"empty", nil},
		{"short", []byte{1, 0, 0}},
		{"bad version", append([]byte{2}, good[1:]...)},
		{"too many entries", append([]byte{1}, u32(project.MaxNodes+1)...)},
		{"count without entries", append([]byte{1}, u32(2)...)},
		{"truncated", good[:len(good)-1]},
		{"trailing bytes", append(bytes.Clone(good), 0)},
		{"bad type", one(3, 0, 0, "a", nil)},
		{"bad flags", one(1, 2, 0, "a", []byte("automerge:x"))},
		{"size over int64", one(1, 0, 1<<63, "a", []byte("automerge:x"))},
		{"empty path", one(1, 0, 0, "", []byte("automerge:x"))},
		{"path not UTF-8", one(1, 0, 0, "\xff", []byte("automerge:x"))},
		{"path over the limit", one(1, 0, 0, strings.Repeat("a", project.MaxPathLen+1), []byte("automerge:x"))},
		{"text without a URL", one(1, 0, 0, "a", nil)},
		{"text with a bad URL", one(1, 0, 0, "a", []byte{0xff})},
		{"blob with a short hash", one(2, 0, 5, "a", hash[:31])},
		{"blob with a long hash", one(2, 0, 5, "a", append(hash, 1))},
		{"folder with a reference", one(0, 0, 0, "a", hash)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := bridge.DecodeTree(tc.wire); !errors.Is(err, bridge.ErrMalformed) {
				t.Errorf("DecodeTree = %+v, %v; want ErrMalformed", got, err)
			}
		})
	}
}

func TestEncodeTreeRejects(t *testing.T) {
	for name, e := range map[string]project.TreeEntry{
		"bad type":        {Path: "a", Type: 9},
		"negative size":   {Path: "a", Type: project.NodeBlob, Size: -1},
		"empty path":      {Type: project.NodeDir},
		"long URL":        {Path: "a", Type: project.NodeText, URL: strings.Repeat("u", 256)},
		"URL on a folder": {Path: "a", Type: project.NodeDir, URL: "x"},
		"hash on a text":  {Path: "a", Type: project.NodeText, URL: "x", Hash: [32]byte{1}},
	} {
		if _, err := bridge.EncodeTree([]project.TreeEntry{e}); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("%s: EncodeTree = %v, want ErrMalformed", name, err)
		}
	}
}

// A hostile name is not the decoder's business: it comes through for
// ApplyTree to refuse and count.
func TestDecodeTreeKeepsHostileNames(t *testing.T) {
	in := []project.TreeEntry{{Path: "../x\x00", Type: project.NodeText, URL: "automerge:x"}}
	wire, err := bridge.EncodeTree(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := bridge.DecodeTree(wire)
	if err != nil || len(out) != 1 || out[0].Path != in[0].Path {
		t.Errorf("DecodeTree = %+v, %v; want the entry unchanged", out, err)
	}
}

func TestDecodeReferencedRejects(t *testing.T) {
	for name, wire := range map[string][]byte{
		"empty":           nil,
		"bad version":     {2, 0, 0, 0, 0},
		"count too high":  {1, 0, 0, 0, 2, 1, 2, 3},
		"count too low":   append([]byte{1, 0, 0, 0, 0}, 1),
		"huge count":      {1, 0xff, 0xff, 0xff, 0xff},
		"truncated hash":  append([]byte{1, 0, 0, 0, 1}, make([]byte, 31)...),
		"trailing in one": append([]byte{1, 0, 0, 0, 1}, make([]byte, 33)...),
	} {
		if got, err := bridge.DecodeReferenced(wire); !errors.Is(err, bridge.ErrMalformed) {
			t.Errorf("%s: DecodeReferenced = %v, %v; want ErrMalformed", name, got, err)
		}
	}
	if _, _, err := bridge.DecodeBlobInfo([]byte{1, 2}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeBlobInfo(short) = %v, want ErrMalformed", err)
	}
	if _, err := bridge.DecodeTreeResult([]byte{1}); !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("DecodeTreeResult(short) = %v, want ErrMalformed", err)
	}
}

// FuzzDecodeTree: the decoder never panics on the webview's bytes, and what it
// accepts encodes back to the same bytes.
func FuzzDecodeTree(f *testing.F) {
	good, err := bridge.EncodeTree(goodTree())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(good)
	f.Add([]byte{1, 0, 0, 0, 0})
	f.Add([]byte{1, 0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		entries, err := bridge.DecodeTree(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeTree(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		again, err := bridge.EncodeTree(entries)
		if err != nil || !bytes.Equal(again, b) {
			t.Fatalf("round trip of %x gave %x, %v", b, again, err)
		}
	})
}

func FuzzDecodeReferenced(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 0})
	f.Add(append([]byte{1, 0, 0, 0, 1}, make([]byte, 32)...))
	f.Add([]byte{1, 0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, b []byte) {
		hashes, err := bridge.DecodeReferenced(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeReferenced(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		again, err := bridge.EncodeReferenced(hashes)
		if err != nil || !bytes.Equal(again, b) {
			t.Fatalf("round trip of %x gave %x, %v", b, again, err)
		}
	})
}

// The store stream carries the tree and the referenced hashes to the storage,
// and answers with the number of refusals.
func TestStoreServerTreeAndReferenced(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	tree := append(goodTree(), project.TreeEntry{Path: "../evil.typ", Type: project.NodeText, URL: "automerge:e"})
	wire, err := bridge.EncodeTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	resp := c.call(bridge.OpTree, nil, string(wire))
	if resp.Status != bridge.StatusOK {
		t.Fatalf("Tree status = %d (%s)", resp.Status, resp.Payload)
	}
	if n, err := bridge.DecodeTreeResult(resp.Payload); err != nil || n != 1 {
		t.Errorf("Tree result = %d, %v; want 1 refused", n, err)
	}
	if len(st.trees) != 1 || len(st.trees[0]) != 4 {
		t.Errorf("storage got %d trees, first of %d entries; want one of 4", len(st.trees), len(st.trees[0]))
	}

	refs, _ := bridge.EncodeReferenced([][bridge.HashSize]byte{{1}, {2}})
	if resp := c.call(bridge.OpReferenced, nil, string(refs)); resp.Status != bridge.StatusOK || len(resp.Payload) != 0 {
		t.Errorf("Referenced = %d %x, want OK and no payload", resp.Status, resp.Payload)
	}
	if len(st.referenced) != 2 || st.referenced[1][0] != 2 {
		t.Errorf("storage referenced = %x", st.referenced)
	}

	// Garbage data is an error, and the stream goes on.
	if resp := c.call(bridge.OpTree, nil, "garbage"); resp.Status != bridge.StatusError {
		t.Errorf("Tree(garbage) status = %d, want Error", resp.Status)
	}
	if resp := c.call(bridge.OpReferenced, nil, "\x01\x00\x00\x00\x09"); resp.Status != bridge.StatusError {
		t.Errorf("Referenced(garbage) status = %d, want Error", resp.Status)
	}
	if len(st.trees) != 1 {
		t.Error("a garbage tree reached the storage")
	}
}

// Projection of a nested file, and PutBlob with its answers.
func TestStoreServerPathOperations(t *testing.T) {
	st, _ := newFakeStorage(t)
	_, c, stop := startStore(t, st)
	defer stop()

	key := []string{"chapters", "résumé 1.typ"}
	if resp := c.call(bridge.OpWriteProjection, key, "= One\n"); resp.Status != bridge.StatusOK {
		t.Fatalf("WriteProjection status = %d (%s)", resp.Status, resp.Payload)
	}
	if got := c.call(bridge.OpReadProjection, key, ""); got.Status != bridge.StatusOK || string(got.Payload) != "= One\n" {
		t.Errorf("ReadProjection = %d %q", got.Status, got.Payload)
	}
	if got := st.files["chapters/résumé 1.typ"]; string(got) != "= One\n" {
		t.Errorf("storage got the path %q, want components joined with a slash", slices.Sorted(maps.Keys(st.files)))
	}

	resp := c.call(bridge.OpPutBlob, []string{"img", "logo.png"}, "")
	h, size, err := bridge.DecodeBlobInfo(resp.Payload)
	if resp.Status != bridge.StatusOK || err != nil || h != sha256.Sum256([]byte("img/logo.png")) || size != int64(len("img/logo.png")) {
		t.Errorf("PutBlob = %d %x, %v", resp.Status, resp.Payload, err)
	}
	// PRJ-11: a file over the limit is refused with the limit, so that the interface can say it.
	big := c.call(bridge.OpPutBlob, []string{"huge.bin"}, "")
	if big.Status != bridge.StatusTooLarge || binary.BigEndian.Uint64(big.Payload) != 100<<20 {
		t.Errorf("PutBlob(huge) = %d %x, want TooLarge with the 100 MB limit", big.Status, big.Payload)
	}
	if gone := c.call(bridge.OpPutBlob, []string{"missing.bin"}, ""); gone.Status != bridge.StatusNotFound {
		t.Errorf("PutBlob(missing) status = %d, want NotFound", gone.Status)
	}
}
