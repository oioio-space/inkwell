package blob_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"lukechampine.com/blake3/bao"

	"github.com/oioio-space/inkwell/internal/blob"
)

func TestPutGet(t *testing.T) {
	s, dir := openStore(t, "", blob.Options{})
	data := testData(100_000, 1)
	h, err := s.PutBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if h != blob.HashOf(data) {
		t.Errorf("PutBytes hash = %v, want %v", h, blob.HashOf(data))
	}
	if !s.Has(h) {
		t.Error("Has = false after Put")
	}
	if n, err := s.Size(h); err != nil || n != int64(len(data)) {
		t.Errorf("Size = %d, %v; want %d", n, err, len(data))
	}
	if got := s.Used(); got != int64(len(data)) {
		t.Errorf("Used = %d, want %d", got, len(data))
	}
	mustGet(t, s, h, data)

	// Layout: <2 hex>/<64 hex>, read-only, with an outboard tree of the size Bao says.
	hexName := h.String()[len("blake3:"):]
	blobFile := filepath.Join(dir, hexName[:2], hexName)
	info, err := os.Stat(blobFile)
	if err != nil {
		t.Fatalf("blob file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o400 {
		t.Errorf("blob mode = %v, want 0400 (read-only)", info.Mode().Perm())
	}
	tree, err := os.Stat(blobFile + ".obao")
	if err != nil {
		t.Fatalf("outboard file: %v", err)
	}
	if want := int64(bao.EncodedSize(len(data), 4, true)); tree.Size() != want {
		t.Errorf("outboard size = %d, want %d", tree.Size(), want)
	}
	if ratio := float64(tree.Size()) / float64(len(data)); ratio > 0.01 {
		t.Errorf("outboard is %.2f%% of the file, want about 0.4%%", ratio*100)
	}

	// A second Put of the same bytes changes nothing.
	if h2, err := s.PutBytes(data); err != nil || h2 != h || s.Used() != int64(len(data)) {
		t.Errorf("second Put = %v, %v, used %d; want %v and %d", h2, err, s.Used(), h, len(data))
	}
}

func TestPutEmptyAndSmall(t *testing.T) {
	s, _ := openStore(t, "", blob.Options{})
	for _, data := range [][]byte{nil, []byte("x"), testData(blob.GroupSize, 2), testData(blob.GroupSize+1, 3)} {
		h, err := s.PutBytes(data)
		if err != nil {
			t.Fatalf("PutBytes(%d bytes): %v", len(data), err)
		}
		mustGet(t, s, h, data)
	}
}

func TestPutSizeMismatch(t *testing.T) {
	s, _ := openStore(t, "", blob.Options{})
	for name, tc := range map[string]struct {
		data []byte
		size int64
	}{
		"fewer bytes than announced": {[]byte("abc"), 5},
		"more bytes than announced":  {[]byte("abcdef"), 3},
	} {
		if _, err := s.Put(bytes.NewReader(tc.data), tc.size); !errors.Is(err, blob.ErrSizeMismatch) {
			t.Errorf("%s: Put = %v, want ErrSizeMismatch", name, err)
		}
	}
	if s.Used() != 0 {
		t.Errorf("Used = %d after failed Puts, want 0", s.Used())
	}
}

// A blob that rotted on disk is detected by Get, removed, and can be put again.
func TestGetDetectsBitRot(t *testing.T) {
	s, dir := openStore(t, "", blob.Options{})
	data := testData(50_000, 4)
	h, err := s.PutBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	hexName := h.String()[len("blake3:"):]
	file := filepath.Join(dir, hexName[:2], hexName)
	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	rotten := bytes.Clone(data)
	rotten[1234] ^= 1
	if err := os.WriteFile(file, rotten, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(h); !errors.Is(err, blob.ErrCorrupt) {
		t.Fatalf("Get of a rotten blob = %v, want ErrCorrupt", err)
	}
	if s.Has(h) || s.Used() != 0 {
		t.Errorf("rotten blob kept: Has %v, Used %d", s.Has(h), s.Used())
	}
	if _, err := s.Get(h); !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("Get after removal = %v, want ErrNotFound", err)
	}
	if _, err := s.PutBytes(data); err != nil {
		t.Errorf("Put after removal: %v", err)
	}
}

// PRJ-11: the quotas. A file over the limit is refused with a message, a
// project over its limit asks the arrival to confirm, and a full disk is
// refused before anything is written.
func TestQuotas(t *testing.T) {
	t.Run("PRJ-11 a file over 100 MB is refused", func(t *testing.T) {
		s, _ := openStore(t, "", blob.Options{})
		if file, _ := s.Limits(); file != 100<<20 {
			t.Fatalf("default file limit = %d, want 100 MB", file)
		}
		// Refused from the announced size, without reading a byte.
		_, err := s.Put(errReader{}, 100<<20+1)
		if !errors.Is(err, blob.ErrTooLarge) {
			t.Errorf("Put of 100 MB + 1 = %v, want ErrTooLarge", err)
		}
		if err != nil && err.Error() == "" {
			t.Error("the error has no message")
		}
	})
	t.Run("PRJ-11 a file at the limit is accepted", func(t *testing.T) {
		s, _ := openStore(t, "", blob.Options{MaxFile: 1 << 20})
		if _, err := s.PutBytes(testData(1<<20, 5)); err != nil {
			t.Errorf("Put at the limit: %v", err)
		}
		if _, err := s.PutBytes(testData(1<<20+1, 6)); !errors.Is(err, blob.ErrTooLarge) {
			t.Errorf("Put over the limit = %v, want ErrTooLarge", err)
		}
	})
	t.Run("PRJ-11 a project over 2 GB asks for confirmation", func(t *testing.T) {
		s, _ := openStore(t, "", blob.Options{MaxFile: 1 << 20, MaxProject: 3 << 20})
		if _, limit := s.Limits(); limit != 3<<20 {
			t.Fatalf("project limit = %d", limit)
		}
		var refs []blob.Ref
		for i := range 4 {
			d := testData(1<<20, uint64(10+i))
			refs = append(refs, blob.Ref{Hash: blob.HashOf(d), Size: int64(len(d))})
		}
		plan := s.Plan(refs)
		if !plan.NeedsConfirmation || plan.Bytes != 4<<20 || len(plan.Fetch) != 4 || plan.Limit != 3<<20 {
			t.Errorf("Plan = %+v, want 4 MiB to fetch and a confirmation", plan)
		}
		if p := s.Plan(refs[:3]); p.NeedsConfirmation {
			t.Errorf("Plan of exactly the limit = %+v, want no confirmation", p)
		}
		// Local storing past the limit is refused with the sizes, until confirmed.
		for i := range 3 {
			if _, err := s.PutBytes(testData(1<<20, uint64(10+i))); err != nil {
				t.Fatalf("Put %d: %v", i, err)
			}
		}
		_, err := s.PutBytes(testData(1<<20, 13))
		qe, ok := errors.AsType[*blob.QuotaError](err)
		if !ok || !errors.Is(err, blob.ErrProjectQuota) || qe.Need != 4<<20 || qe.Limit != 3<<20 {
			t.Fatalf("Put over the project limit = %v, want a QuotaError of 4 MiB over 3 MiB", err)
		}
		s.Confirm()
		if _, err := s.PutBytes(testData(1<<20, 13)); err != nil {
			t.Errorf("Put after Confirm: %v", err)
		}
		if p := s.Plan(refs); p.NeedsConfirmation {
			t.Errorf("Plan after Confirm = %+v, want no confirmation", p)
		}
	})
	t.Run("PRJ-11 a plan sets aside the files over the limit", func(t *testing.T) {
		s, _ := openStore(t, "", blob.Options{})
		huge := blob.Ref{Hash: blob.HashOf([]byte("huge")), Size: 101 << 20}
		small := blob.Ref{Hash: blob.HashOf([]byte("small")), Size: 10}
		p := s.Plan([]blob.Ref{huge, small, small})
		if len(p.TooLarge) != 1 || p.TooLarge[0] != huge || len(p.Fetch) != 1 || p.Fetch[0] != small {
			t.Errorf("Plan = %+v, want the 101 MB file refused and the small one fetched once", p)
		}
	})
	t.Run("PRJ-11 a full disk is refused", func(t *testing.T) {
		free := uint64(10 << 20)
		s, _ := openStore(t, "", blob.Options{
			Reserve:   4 << 20,
			FreeSpace: func() (uint64, error) { return free, nil },
		})
		if _, err := s.PutBytes(testData(7<<20, 7)); !errors.Is(err, blob.ErrNoSpace) {
			t.Errorf("Put of 7 MiB with 10 MiB free and 4 MiB reserved = %v, want ErrNoSpace", err)
		}
		if _, err := s.PutBytes(testData(5<<20, 8)); err != nil {
			t.Errorf("Put of 5 MiB: %v", err)
		}
	})
	t.Run("an unknown free space does not block", func(t *testing.T) {
		s, _ := openStore(t, "", blob.Options{FreeSpace: func() (uint64, error) { return 0, errors.ErrUnsupported }})
		if _, err := s.PutBytes([]byte("x")); err != nil {
			t.Errorf("Put: %v", err)
		}
	})
}

// errReader fails the test that reads it.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("must not be read") }

// PRJ-13: an image replaced on disk is a new blob; the old one stays until it
// is no longer referenced.
func TestReplacedBinaryIsANewBlob(t *testing.T) {
	t.Run("PRJ-13 a replaced binary is a new hash", runReplacedBinaryIsANewBlob)
}

func runReplacedBinaryIsANewBlob(t *testing.T) {
	s, _ := openStore(t, "", blob.Options{})
	v1, v2 := testData(70_000, 20), testData(70_000, 21)
	h1, err := s.PutBytes(v1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := s.PutBytes(v2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatal("two different files share a hash")
	}
	mustGet(t, s, h1, v1)
	mustGet(t, s, h2, v2)
	// One flipped byte is a different hash too.
	flipped := bytes.Clone(v1)
	flipped[0] ^= 0x80
	if h3, err := s.PutBytes(flipped); err != nil || h3 == h1 {
		t.Errorf("Put of a flipped byte = %v, %v; want a new hash", h3, err)
	}
}

func TestReopenCountsAndCleans(t *testing.T) {
	s, dir := openStore(t, "", blob.Options{})
	h, err := s.PutBytes(testData(5000, 30))
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "partial", "put-stale.tmp")
	if err := os.WriteFile(stale, []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	again, _ := openStore(t, dir, blob.Options{})
	if again.Used() != 5000 || !again.Has(h) {
		t.Errorf("reopened: Used %d, Has %v; want 5000 and true", again.Used(), again.Has(h))
	}
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale temporary file: %v, want removed", err)
	}
	if hs, err := again.List(); err != nil || len(hs) != 1 || hs[0] != h {
		t.Errorf("List = %v, %v; want [%v]", hs, err, h)
	}
}

func TestRemove(t *testing.T) {
	s, dir := openStore(t, "", blob.Options{})
	h, err := s.PutBytes(testData(5000, 31))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(h); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if s.Has(h) || s.Used() != 0 {
		t.Errorf("after Remove: Has %v, Used %d", s.Has(h), s.Used())
	}
	if _, err := os.Stat(filepath.Join(dir, h.String()[len("blake3:"):][:2], h.String()[len("blake3:"):]+".obao")); err == nil {
		t.Error("the outboard file is still there")
	}
	if err := s.Remove(h); err != nil {
		t.Errorf("Remove of a missing blob: %v", err)
	}
}

func TestReadSlice(t *testing.T) {
	s, _ := openStore(t, "", blob.Options{})
	data := testData(5*blob.GroupSize+777, 40)
	h, err := s.PutBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		offset       uint64
		length, want uint32
	}{
		{"the first group", 0, blob.GroupSize, blob.GroupSize},
		{"a middle range", 2 * blob.GroupSize, 2 * blob.GroupSize, 2 * blob.GroupSize},
		{"clamped at the end", 4 * blob.GroupSize, 3 * blob.GroupSize, blob.GroupSize + 777},
		{"the whole file", 0, 8 * blob.GroupSize, uint32(len(data))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sl, err := s.ReadSlice(h, tc.offset, tc.length)
			if err != nil || sl.Length != tc.want || sl.Offset != tc.offset || sl.Hash != h {
				t.Fatalf("ReadSlice = %+v, %v; want %d bytes at %d", sl.Length, err, tc.want, tc.offset)
			}
			got, ok := bao.VerifySlice(sl.Bao, 4, tc.offset, uint64(tc.want), h)
			if !ok || !bytes.Equal(got, data[tc.offset:tc.offset+uint64(tc.want)]) {
				t.Errorf("the slice does not verify or holds other bytes")
			}
		})
	}
	if _, err := s.ReadSlice(h, 6*blob.GroupSize, blob.GroupSize); !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("ReadSlice past the end = %v, want ErrNotFound", err)
	}
	if _, err := s.ReadSlice(h, 1, blob.GroupSize); !errors.Is(err, blob.ErrMalformed) {
		t.Errorf("ReadSlice at a misaligned offset = %v, want ErrMalformed", err)
	}
	if _, err := s.ReadSlice(blob.HashOf([]byte("nope")), 0, blob.GroupSize); !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("ReadSlice of an unknown blob = %v, want ErrNotFound", err)
	}
}

// A missing outboard tree is rebuilt from the file when a slice is served.
func TestReadSliceRebuildsOutboard(t *testing.T) {
	s, dir := openStore(t, "", blob.Options{})
	data := testData(3*blob.GroupSize, 41)
	h, err := s.PutBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	x := h.String()[len("blake3:"):]
	obao := filepath.Join(dir, x[:2], x+".obao")
	if err := os.Chmod(obao, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(obao); err != nil {
		t.Fatal(err)
	}
	sl, err := s.ReadSlice(h, 0, blob.GroupSize)
	if err != nil {
		t.Fatalf("ReadSlice without outboard: %v", err)
	}
	if _, ok := bao.VerifySlice(sl.Bao, 4, 0, blob.GroupSize, h); !ok {
		t.Error("the slice does not verify")
	}
	if _, err := os.Stat(obao); err != nil {
		t.Errorf("outboard not rebuilt: %v", err)
	}
}

// A blob that rotted is never served: it is removed and reported corrupt.
func TestReadSliceRefusesRottenBlob(t *testing.T) {
	s, dir := openStore(t, "", blob.Options{})
	data := testData(3*blob.GroupSize, 42)
	h, err := s.PutBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	x := h.String()[len("blake3:"):]
	file := filepath.Join(dir, x[:2], x)
	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	data[100] ^= 1
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSlice(h, 0, blob.GroupSize); !errors.Is(err, blob.ErrCorrupt) {
		t.Errorf("ReadSlice of a rotten blob = %v, want ErrCorrupt", err)
	}
	if s.Has(h) {
		t.Error("the rotten blob is still stored")
	}
}

func TestGC(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	s, dir := openStore(t, "", blob.Options{Now: clock})
	keep, drop := testData(2000, 50), testData(3000, 51)
	hKeep, _ := s.PutBytes(keep)
	hDrop, _ := s.PutBytes(drop)
	hLate, _ := s.PutBytes(testData(4000, 52))
	referenced := map[blob.Hash]bool{hKeep: true}
	isRef := func(h blob.Hash) bool { return referenced[h] }
	ttl := blob.UnreferencedTTL

	// A partial download of an unreferenced blob that stopped long ago.
	stuck := blob.HashOf([]byte("stuck"))
	stuckFile := filepath.Join(dir, "partial", stuck.String()[len("blake3:"):])
	if err := os.WriteFile(stuckFile, make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stuckFile+".map", []byte("map"), 0o600); err != nil {
		t.Fatal(err)
	}

	if res, err := s.GC(isRef, ttl); err != nil || res.Removed != 0 {
		t.Fatalf("first pass = %+v, %v; want nothing removed (it only marks)", res, err)
	}
	now = now.Add(29 * 24 * time.Hour)
	if res, err := s.GC(isRef, ttl); err != nil || res.Removed != 0 {
		t.Fatalf("pass at 29 days = %+v, %v; want nothing removed", res, err)
	}
	// hLate is referenced again at day 29: its mark goes.
	referenced[hLate] = true
	if res, err := s.GC(isRef, ttl); err != nil || res.Removed != 0 {
		t.Fatalf("pass with a restored file = %+v, %v", res, err)
	}
	delete(referenced, hLate)
	now = now.Add(2 * 24 * time.Hour) // day 31: hDrop is 31 days unreferenced, hLate 2.
	if err := os.Chtimes(stuckFile, now.Add(-31*24*time.Hour), now.Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	res, err := s.GC(isRef, ttl)
	if err != nil {
		t.Fatal(err)
	}
	if s.Has(hDrop) || !s.Has(hKeep) || !s.Has(hLate) {
		t.Errorf("after 31 days: Has(drop) %v, Has(keep) %v, Has(restored then dropped) %v; want false, true, true",
			s.Has(hDrop), s.Has(hKeep), s.Has(hLate))
	}
	if res.Removed != 2 || res.Freed != int64(len(drop))+100 {
		t.Errorf("GC = %+v, want the blob and the abandoned download (%d bytes)", res, len(drop)+100)
	}
	if _, err := os.Stat(stuckFile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the abandoned download is still there: %v", err)
	}
	if _, err := os.Stat(stuckFile + ".map"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the map of the abandoned download is still there: %v", err)
	}
	if s.Used() != int64(len(keep)+4000) {
		t.Errorf("Used = %d after GC, want %d", s.Used(), len(keep)+4000)
	}
	// The marks survive a restart.
	again, _ := openStore(t, dir, blob.Options{Now: clock})
	now = now.Add(30 * 24 * time.Hour)
	if res, err := again.GC(isRef, ttl); err != nil || res.Removed != 1 || again.Has(hLate) {
		t.Errorf("pass after a restart = %+v, %v, Has(late) %v; want it removed", res, err, again.Has(hLate))
	}
}

func TestHashText(t *testing.T) {
	h := blob.HashOf([]byte("hello"))
	text := h.String()
	if len(text) != len("blake3:")+64 {
		t.Fatalf("String() = %q", text)
	}
	back, err := blob.ParseHash(text)
	if err != nil || back != h {
		t.Errorf("ParseHash(%q) = %v, %v", text, back, err)
	}
	for _, bad := range []string{"", "blake3:", "blake3:zz", text[:len(text)-1], "sha256:" + text[7:], "BLAKE3:" + text[7:], text + "0", "blake3:" + upper(text[7:])} {
		if _, err := blob.ParseHash(bad); !errors.Is(err, blob.ErrBadHash) {
			t.Errorf("ParseHash(%q) = %v, want ErrBadHash", bad, err)
		}
	}
}

func upper(s string) string { return string(bytes.ToUpper([]byte(s))) }
