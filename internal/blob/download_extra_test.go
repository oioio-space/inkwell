package blob

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"testing"

	"lukechampine.com/blake3/bao"
)

func TestGroupsOf(t *testing.T) {
	for size, want := range map[int64]int{0: 0, 1: 1, GroupSize - 1: 1, GroupSize: 1, GroupSize + 1: 2, 2 * GroupSize: 2, 3*GroupSize + 5: 4} {
		if got := groupsOf(size); got != want {
			t.Errorf("groupsOf(%d) = %d, want %d", size, got, want)
		}
	}
}

// mapFile builds a map file as [download.encodeMap] writes it.
func mapFile(size int64, bitmap []byte) []byte {
	b := append([]byte(mapMagic), binary.BigEndian.AppendUint64(nil, uint64(size))...)
	b = binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(bitmap))
	return append(b, bitmap...)
}

// TestParseMap: a map is trusted only if every field agrees with the file it
// claims to describe, since it decides which groups are not requested again.
func TestParseMap(t *testing.T) {
	const size = 9 * GroupSize // 9 groups: 2 bytes of bitmap, bit 8 is the last real one
	good := []byte{0xFF, 0x01}
	if got, ok := parseMap(mapFile(size, good), size, 9); !ok || !bytes.Equal(got, good) {
		t.Fatalf("parseMap(valid) = %x, %t; want the bitmap", got, ok)
	}
	for name, tc := range map[string]struct {
		raw    []byte
		size   int64
		groups int
	}{
		"bit of a group that does not exist": {mapFile(size, []byte{0xFF, 0x02}), size, 9},
		"every bit of the spare byte":        {mapFile(size, []byte{0xFF, 0xFF}), size, 9},
		"wrong size in the file":             {mapFile(size+1, good), size, 9},
		"wrong size asked":                   {mapFile(size, good), size + 1, 9},
		"empty":                              {nil, size, 9},
		"truncated header":                   {mapFile(size, good)[:mapHeader-1], size, 9},
		"bitmap too short":                   {mapFile(size, []byte{0xFF}), size, 9},
		"bitmap too long":                    {mapFile(size, []byte{0xFF, 0x01, 0}), size, 9},
	} {
		if got, ok := parseMap(tc.raw, tc.size, tc.groups); ok {
			t.Errorf("parseMap(%s) = %x, true; want refused", name, got)
		}
	}
	bad := mapFile(size, good)
	bad[0] = 'X'
	if _, ok := parseMap(bad, size, 9); ok {
		t.Error("parseMap(wrong magic) accepted")
	}
	bad = mapFile(size, good)
	bad[len(bad)-2] ^= 0x01 // a bit flips in the bitmap, the checksum does not
	if _, ok := parseMap(bad, size, 9); ok {
		t.Error("parseMap(wrong checksum) accepted")
	}
	// A multiple of 8 groups has no spare bits to police.
	if _, ok := parseMap(mapFile(8*GroupSize, []byte{0xFF}), 8*GroupSize, 8); !ok {
		t.Error("parseMap(8 groups, all set) refused")
	}
}

func newTestDownload(t *testing.T, size int64) (*Store, *download) {
	t.Helper()
	s := openInternal(t, Options{FreeSpace: func() (uint64, error) { return 1 << 40, nil }})
	d, err := s.openDownload(HashOf([]byte("x")), size)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.discard)
	return s, d
}

// TestDownloadNextAndCovers: next names the first hole, at most MaxSlice long;
// covers needs every group of the range.
func TestDownloadNextAndCovers(t *testing.T) {
	const groups = 150
	_, d := newTestDownload(t, groups*GroupSize-3) // the last group is short
	if off, n, ok := d.next(); !ok || off != 0 || n != MaxSlice {
		t.Fatalf("next() on an empty download = %d, %d, %t; want 0, MaxSlice", off, n, ok)
	}
	put := func(group int) {
		t.Helper()
		if err := d.put(uint64(group)*GroupSize, make([]byte, GroupSize)); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []int{0, 1, 3} {
		put(g)
	}
	if off, n, ok := d.next(); !ok || off != 2*GroupSize || n != GroupSize {
		t.Errorf("next() with groups 0, 1, 3 = %d, %d, %t; want the hole at group 2", off, n, ok)
	}
	put(2)
	if off, n, ok := d.next(); !ok || off != 4*GroupSize || n != MaxSlice {
		t.Errorf("next() after 0-3 = %d, %d, %t; want group 4 and MaxSlice", off, n, ok)
	}
	// A run stops at the next verified group.
	put(4 + 5)
	if off, n, ok := d.next(); !ok || off != 4*GroupSize || n != 5*GroupSize {
		t.Errorf("next() with a verified group after 5 holes = %d, %d, %t; want 5 groups", off, n, ok)
	}
	// And at the end of the file.
	for g := 4; g < groups; g++ {
		if !d.has(g) {
			put(g)
		}
	}
	if _, _, ok := d.next(); ok || !d.complete() {
		t.Errorf("next() ok = %t, complete = %t on a full download", ok, d.complete())
	}
	if d.have != groups {
		t.Errorf("have = %d, want %d (a group written twice counts once)", d.have, groups)
	}
	put(7)
	if d.have != groups {
		t.Errorf("have = %d after rewriting a group, want %d", d.have, groups)
	}

	_, e := newTestDownload(t, 10*GroupSize)
	for _, g := range []int{2, 3, 4} {
		if err := e.put(uint64(g)*GroupSize, make([]byte, GroupSize)); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		off    uint64
		length uint32
		want   bool
	}{
		{2 * GroupSize, 3 * GroupSize, true},
		{2 * GroupSize, GroupSize, true},
		{3 * GroupSize, GroupSize, true},
		{1 * GroupSize, 2 * GroupSize, false}, // the first group is missing
		{2 * GroupSize, 4 * GroupSize, false}, // the last group is missing
		{0, GroupSize, false},
		{9 * GroupSize, 4 * GroupSize, false}, // asks past the end: group 9 is missing
	} {
		if got := e.covers(tc.off, tc.length); got != tc.want {
			t.Errorf("covers(%d groups from %d, %d) = %t, want %t", tc.length/GroupSize, tc.off/GroupSize, tc.length, got, tc.want)
		}
	}
	// Past the end of the file, asking for what is there is enough.
	for g := range 10 {
		if !e.has(g) {
			if err := e.put(uint64(g)*GroupSize, make([]byte, GroupSize)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !e.covers(9*GroupSize, 4*GroupSize) {
		t.Error("covers(range reaching past the end of a complete file) = false")
	}
}

// TestDownloadFlushesEverySyncEvery: the map reaches the disk after syncEvery
// bytes received, not before, and what it holds is then what resume finds.
func TestDownloadFlushesEverySyncEvery(t *testing.T) {
	const size = 2 * syncEvery
	s, d := newTestDownload(t, size)
	onDisk := func() bool {
		_, err := s.root.Stat(d.mapName)
		return err == nil
	}
	if err := d.put(0, make([]byte, syncEvery-GroupSize)); err != nil {
		t.Fatal(err)
	}
	if onDisk() {
		t.Fatal("the map was saved before syncEvery bytes were received")
	}
	if err := d.put(uint64(syncEvery-GroupSize), make([]byte, GroupSize)); err != nil {
		t.Fatal(err)
	}
	if !onDisk() {
		t.Fatal("the map was not saved after syncEvery bytes")
	}
	if d.unsynced != 0 {
		t.Errorf("unsynced = %d after a flush, want 0", d.unsynced)
	}

	// Resume: the map is trusted when it agrees with the file, with its have count.
	if err := d.f.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := s.openDownload(d.h, size)
	if err != nil {
		t.Fatal(err)
	}
	defer again.discard()
	if want := syncEvery / GroupSize; again.have != want {
		t.Errorf("resumed with %d groups, want %d", again.have, want)
	}
	if off, _, ok := again.next(); !ok || off != syncEvery {
		t.Errorf("resumed download asks from %d, want %d", off, syncEvery)
	}
	if err := again.f.Close(); err != nil {
		t.Fatal(err)
	}

	// The same blob with another size starts over; so does a data file that was cut.
	other, err := s.openDownload(d.h, size+1)
	if err != nil {
		t.Fatal(err)
	}
	if other.have != 0 {
		t.Errorf("a download of another size resumed %d groups", other.have)
	}
	_ = other.f.Close()
}

// TestDownloadWithoutAMapStartsOver: a leftover data file with no map proves
// nothing and is not trusted.
func TestDownloadWithoutAMapStartsOver(t *testing.T) {
	s, d := newTestDownload(t, 3*GroupSize)
	if err := d.put(0, bytes.Repeat([]byte{9}, GroupSize)); err != nil {
		t.Fatal(err)
	}
	if err := d.suspend(); err != nil {
		t.Fatal(err)
	}
	if err := s.root.Remove(d.mapName); err != nil {
		t.Fatal(err)
	}
	again, err := s.openDownload(d.h, 3*GroupSize)
	if err != nil {
		t.Fatal(err)
	}
	defer again.discard()
	if again.have != 0 {
		t.Errorf("resumed %d groups with no map", again.have)
	}
	if data, err := s.root.ReadFile(again.dataName); err != nil || len(data) != 3*GroupSize || data[0] != 0 {
		t.Errorf("the old data survived a restart from scratch: %d bytes, first %d, %v", len(data), data[0], err)
	}
}

// TestHoldBoundaries: every limit of a hold is exact.
func TestHoldBoundaries(t *testing.T) {
	const mib = 1 << 20
	free := uint64(1 << 40)
	s := openInternal(t, Options{
		MaxFile: 4 * mib, MaxProject: 10 * mib, Reserve: mib,
		FreeSpace: func() (uint64, error) { return free, nil },
	})
	for size, wantErr := range map[int64]error{
		0: nil, 4 * mib: nil, 4*mib + 1: ErrTooLarge, -1: ErrTooLarge,
	} {
		err := s.hold("k", size)
		if !errors.Is(err, wantErr) || (wantErr == nil && err != nil) {
			t.Errorf("hold(%d, MaxFile 4 MiB) = %v, want %v", size, err, wantErr)
		}
		s.unhold("k")
	}

	// The project quota: exactly MaxProject fits, one byte more asks for confirmation.
	for _, k := range []string{"a", "b"} {
		if err := s.hold(k, 4*mib); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.hold("c", 2*mib); err != nil {
		t.Errorf("hold reaching exactly MaxProject = %v, want nil", err)
	}
	s.unhold("c")
	var q *QuotaError
	if err := s.hold("c", 2*mib+1); !errors.As(err, &q) || q.Need != 10*mib+1 || q.Limit != 10*mib {
		t.Errorf("hold one byte past MaxProject = %v, want a QuotaError of %d over %d", err, 10*mib+1, 10*mib)
	}
	s.Confirm()
	if err := s.hold("c", 2*mib+1); err != nil {
		t.Errorf("hold past MaxProject after Confirm = %v, want nil", err)
	}
	s.unhold("c")
	s.unhold("a")
	s.unhold("b")

	// Free space: exactly what is needed is enough, one byte less is not, and a
	// refused hold leaves nothing behind.
	const size = 3 * mib
	need := uint64(size) + uint64(bao.EncodedSize(size, groupLog, true)) + uint64(mib)
	free = need
	if err := s.hold("d", size); err != nil {
		t.Errorf("hold with exactly the needed free space = %v, want nil", err)
	}
	s.unhold("d")
	free = need - 1
	if err := s.hold("d", size); !errors.Is(err, ErrNoSpace) {
		t.Errorf("hold one byte short of free space = %v, want ErrNoSpace", err)
	}
	if got := s.holds["d"]; got != 0 {
		t.Errorf("a refused hold left %d bytes held", got)
	}
	// A refused re-hold restores the previous one.
	free = 1 << 40
	if err := s.hold("e", mib); err != nil {
		t.Fatal(err)
	}
	free = 0
	if err := s.hold("e", 2*mib); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("hold with no space = %v", err)
	}
	if got := s.holds["e"]; got != mib {
		t.Errorf("a refused re-hold left %d bytes, want the previous %d", got, mib)
	}
	// An unreadable free space disables the check.
	s.opt.FreeSpace = func() (uint64, error) { return 0, os.ErrPermission }
	if err := s.hold("f", mib); err != nil {
		t.Errorf("hold with unknown free space = %v, want nil", err)
	}
}

func TestOptionDefaults(t *testing.T) {
	s := openInternal(t, Options{})
	if f, p := s.Limits(); f != DefaultMaxFile || p != DefaultMaxProject {
		t.Errorf("Limits() = %d, %d; want the defaults %d, %d", f, p, DefaultMaxFile, DefaultMaxProject)
	}
	if s.opt.Reserve != defaultReserve {
		t.Errorf("Reserve = %d, want the default %d", s.opt.Reserve, defaultReserve)
	}
	neg := openInternal(t, Options{MaxFile: -1, MaxProject: -5, Reserve: -7})
	if f, p := neg.Limits(); f != DefaultMaxFile || p != DefaultMaxProject || neg.opt.Reserve != defaultReserve {
		t.Errorf("negative options = %d, %d, %d; want the defaults", f, p, neg.opt.Reserve)
	}
	kept := openInternal(t, Options{MaxFile: 5, MaxProject: 6, Reserve: 7})
	if f, p := kept.Limits(); f != 5 || p != 6 || kept.opt.Reserve != 7 {
		t.Errorf("explicit options = %d, %d, %d; want 5, 6, 7", f, p, kept.opt.Reserve)
	}
}
