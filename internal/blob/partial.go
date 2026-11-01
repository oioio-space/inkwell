package blob

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"math/bits"
	"os"

	"github.com/oioio-space/inkwell/internal/store"
)

// syncEvery is how many bytes are received between two flushes of a download
// to disk: the data is synced, then the map of verified groups saved. A crash
// loses at most that much progress, never a group the map claims.
const syncEvery = 8 << 20

// mapMagic starts the map file of a download.
const mapMagic = "IBM1"

// mapHeader is the length of the map file's fixed part: magic, the size of
// the file, and a CRC-32 of the groups bitmap.
const mapHeader = len(mapMagic) + 8 + 4

// download is a blob being received. Its data lives in partial/<hash>, sparse
// until filled, and the groups already verified in partial/<hash>.map, so that
// a download which stops, by a cut link or a closed laptop, resumes without
// asking again for what is verified. Only the goroutine that downloads uses it.
type download struct {
	s        *Store
	f        *os.File
	dataName string
	mapName  string
	bitmap   []byte // one bit per group, set once the group is verified
	size     int64
	unsynced int64 // bytes written since the last flush
	groups   int
	have     int // groups verified
	h        Hash
}

// groupsOf returns the number of groups of a file of size bytes.
func groupsOf(size int64) int { return int((size + GroupSize - 1) / GroupSize) }

// openDownload opens the download of the blob h of size bytes, resuming the
// one on disk if its map is intact and agrees on h and the size, and starting
// over otherwise.
func (s *Store) openDownload(h Hash, size int64) (*download, error) {
	d := &download{
		s: s, h: h, size: size, groups: groupsOf(size),
		dataName: partialDir + "/" + h.Hex(),
	}
	d.mapName = d.dataName + ".map"
	if d.resume() {
		return d, nil
	}
	d.removeFiles()
	f, err := s.root.OpenFile(d.dataName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("blob: create download: %w", err)
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("blob: allocate download: %w", err)
	}
	d.f, d.bitmap, d.have = f, make([]byte, (d.groups+7)/8), 0
	return d, nil
}

// resume loads the map and the data file of an earlier run, and reports whether
// both are usable.
func (d *download) resume() bool {
	raw, err := d.s.root.ReadFile(d.mapName)
	if err != nil {
		return false
	}
	bitmap, ok := parseMap(raw, d.size, d.groups)
	if !ok {
		d.s.log.Warn("blob: download map unusable, starting over", "hash", d.h.Short())
		return false
	}
	f, err := d.s.root.OpenFile(d.dataName, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	if info, err := f.Stat(); err != nil || info.Size() != d.size {
		_ = f.Close()
		return false
	}
	d.f, d.bitmap = f, bitmap
	for _, b := range bitmap {
		d.have += bits.OnesCount8(b)
	}
	return true
}

// parseMap checks a map file against the size and number of groups it must
// describe, and returns its bitmap.
func parseMap(raw []byte, size int64, groups int) ([]byte, bool) {
	if len(raw) != mapHeader+(groups+7)/8 || string(raw[:len(mapMagic)]) != mapMagic {
		return nil, false
	}
	bitmap := raw[mapHeader:]
	if binary.BigEndian.Uint64(raw[len(mapMagic):]) != uint64(size) || // #nosec G115 -- size is not negative.
		binary.BigEndian.Uint32(raw[len(mapMagic)+8:]) != crc32.ChecksumIEEE(bitmap) {
		return nil, false
	}
	if rest := groups % 8; rest != 0 && bitmap[len(bitmap)-1]>>rest != 0 {
		return nil, false // bits for groups that do not exist
	}
	return bitmap, true
}

// encodeMap serializes the map of d.
func (d *download) encodeMap() []byte {
	b := make([]byte, 0, mapHeader+len(d.bitmap))
	b = append(b, mapMagic...)
	b = binary.BigEndian.AppendUint64(b, uint64(d.size)) // #nosec G115 -- size is not negative.
	b = binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(d.bitmap))
	return append(b, d.bitmap...)
}

func (d *download) has(group int) bool { return d.bitmap[group/8]&(1<<(group%8)) != 0 }

// complete reports whether every group is verified.
func (d *download) complete() bool { return d.have == d.groups }

// next returns the first run of groups still to receive, at most [MaxSlice]
// bytes long. ok is false when there is none.
func (d *download) next() (offset uint64, length uint32, ok bool) {
	first := 0
	for first < d.groups && d.has(first) {
		first++
	}
	if first == d.groups {
		return 0, 0, false
	}
	n := 1
	for first+n < d.groups && n < MaxSlice/GroupSize && !d.has(first+n) {
		n++
	}
	return uint64(first) * GroupSize, uint32(n) * GroupSize, true // #nosec G115 -- n ≤ MaxSlice/GroupSize.
}

// covers reports whether every group of [offset, offset+length) is verified.
func (d *download) covers(offset uint64, length uint32) bool {
	first := int(offset / GroupSize) // #nosec G115 -- offset ≤ the size of the file.
	for g := first; g < min(d.groups, first+groupsOf(int64(length))); g++ {
		if !d.has(g) {
			return false
		}
	}
	return true
}

// put stores the verified bytes data, which start at offset, a multiple of the
// group size, and ends at the end of a group or of the file. It flushes the
// download every [syncEvery] bytes.
func (d *download) put(offset uint64, data []byte) error {
	if _, err := d.f.WriteAt(data, int64(offset)); err != nil { // #nosec G115 -- offset ≤ the size of the file.
		return fmt.Errorf("blob: write download: %w", err)
	}
	first := int(offset / GroupSize) // #nosec G115 -- offset ≤ the size of the file.
	for g := first; g < first+groupsOf(int64(len(data))); g++ {
		if !d.has(g) {
			d.bitmap[g/8] |= 1 << (g % 8)
			d.have++
		}
	}
	d.s.written(d.h.Hex(), min(d.size, int64(d.have)*GroupSize))
	d.unsynced += int64(len(data))
	if d.unsynced >= syncEvery {
		return d.flush()
	}
	return nil
}

// flush makes the data written so far durable, then saves the map. The order
// keeps the map from ever claiming a group the disk does not have.
func (d *download) flush() error {
	if err := d.f.Sync(); err != nil {
		return fmt.Errorf("blob: sync download: %w", err)
	}
	if err := store.WriteAtomic(d.s.root, d.mapName, d.encodeMap(), 0o600); err != nil {
		return fmt.Errorf("blob: save download map: %w", err)
	}
	d.unsynced = 0
	return nil
}

// suspend flushes the download and closes its file, keeping both on disk for a
// later resume.
func (d *download) suspend() error {
	err := d.flush()
	if cerr := d.f.Close(); err == nil {
		err = cerr
	}
	return err
}

// finish checks the whole file against the hash and, if it matches, stores it
// under the hash and ends the download. A file that does not match is
// deleted, and [ErrCorrupt] returned: the map claimed groups that the disk
// does not hold, or a peer got a forgery past the slice check.
func (d *download) finish() error {
	if err := d.f.Sync(); err != nil {
		return fmt.Errorf("blob: sync download: %w", err)
	}
	obao := d.dataName + obaoSuffix + tmpSuffix
	got, err := d.s.outboard(d.f, d.size, obao)
	if err != nil {
		return err
	}
	if got != d.h {
		d.discard()
		_ = d.s.root.Remove(obao)
		return ErrCorrupt
	}
	if err := d.f.Close(); err != nil {
		return fmt.Errorf("blob: close download: %w", err)
	}
	if err := d.s.install(d.h, d.size, d.dataName, obao); err != nil {
		return err
	}
	_ = d.s.root.Remove(d.mapName)
	return nil
}

// discard deletes the download from disk.
func (d *download) discard() {
	if d.f != nil {
		_ = d.f.Close()
	}
	d.removeFiles()
}

func (d *download) removeFiles() {
	for _, name := range []string{d.dataName, d.mapName} {
		if err := d.s.root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			d.s.log.Warn("blob: remove download", "name", name, "err", err)
		}
	}
}
