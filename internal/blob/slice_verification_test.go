package blob

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"lukechampine.com/blake3/bao"
)

func TestCheckSliceVerification(t *testing.T) {
	data := bytes.Repeat([]byte{42}, 3*GroupSize+7)
	tree, h := bao.EncodeBuf(data, groupLog, true)
	for _, offset := range []uint64{0, GroupSize, 3 * GroupSize} {
		n := min(uint64(GroupSize), uint64(len(data))-offset)
		var encoded bytes.Buffer
		if err := bao.ExtractSlice(&encoded, bytes.NewReader(data[offset:offset+n]), bytes.NewReader(tree), groupLog, offset, n); err != nil {
			t.Fatal(err)
		}
		m := Slice{Hash: h, Offset: offset, Length: uint32(n), Bao: encoded.Bytes()}
		got, size, final, err := checkSlice(m, h)
		if err != nil || !bytes.Equal(got, data[offset:offset+n]) || size != uint64(len(data)) || final != (offset+n == size) {
			t.Fatalf("checkSlice(offset=%d) = %d bytes, %d, %v, %v", offset, len(got), size, final, err)
		}
		for _, kind := range []string{"trailing", "truncated", "corrupt"} {
			bad := m
			bad.Bao = bytes.Clone(m.Bao)
			switch kind {
			case "trailing":
				bad.Bao = append(bad.Bao, 0)
			case "truncated":
				bad.Bao = bad.Bao[:len(bad.Bao)-1]
			case "corrupt":
				bad.Bao[len(bad.Bao)-1] ^= 1
			}
			if _, _, _, err := checkSlice(bad, h); err == nil {
				t.Fatalf("checkSlice(offset=%d) accepted %s encoding", offset, kind)
			}
		}
	}
}

// Count actual reads so this test detects a return to scanning the full tree.
type countedOutboard struct {
	*bytes.Reader
	read int
}

func (r *countedOutboard) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestExtractSliceSkipsUnrelatedParents(t *testing.T) {
	data := bytes.Repeat([]byte{42}, 257*GroupSize+7)
	tree, h := bao.EncodeBuf(data, groupLog, true)
	name := filepath.Join(t.TempDir(), "blob")
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	for _, offset := range []uint64{0, 128 * GroupSize, 257 * GroupSize} {
		r := &countedOutboard{Reader: bytes.NewReader(tree)}
		n := min(uint64(GroupSize), uint64(len(data))-offset)
		var encoded bytes.Buffer
		if err := extractSlice(&encoded, f, r, uint64(len(data)), offset, n); err != nil {
			t.Fatal(err)
		}
		got, ok := bao.VerifySlice(encoded.Bytes(), groupLog, offset, n, h)
		if !ok || !bytes.Equal(got, data[offset:offset+n]) {
			t.Fatalf("extracted slice at %d does not verify", offset)
		}
		if r.read >= len(tree) {
			t.Errorf("read %d outboard bytes at offset %d, want less than the full tree (including buffered read-ahead)", r.read, offset)
		}
	}
}
