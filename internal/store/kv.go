// Package store keeps automerge-repo's chunks on disk: an atomic, verified
// key-value store under an [os.Root].
//
// Every value is written with [WriteAtomic] behind a header,
//
//	"IKV1" | SHA-256(data) | data
//
// so a bit flip or truncation is detected on load. A damaged file is moved to
// quarantine/ and replaced by the latest intact backup when there is one (see
// [KV.Backup]). For automerge-repo's "incremental" chunks the last key
// component is also the SHA-256 of the data (its keyHash convention), which
// is checked too.
//
// Keys are 1 to 4 components matching ^[A-Za-z0-9._-]{1,128}$ (never "." or
// ".."); they map to files below repo/, so no key can name a path outside the
// root. The store never logs or returns document bytes in errors.
//
//	kv, err := store.Open(root)
//	err = kv.Save([]string{docID, "snapshot", heads}, chunk)
//	chunk, err = kv.Load([]string{docID, "snapshot", heads})
package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// MaxDepth is the maximum number of components in a key.
	MaxDepth = 4
	// MaxComponent is the maximum length in bytes of one key component.
	MaxComponent = 128
	// MaxValue is the maximum size in bytes of a stored value.
	MaxValue = 32 << 20

	repoDir       = "repo"
	quarantineDir = "quarantine"
	backupsDir    = "backups"
	// leafSuffix marks a file as the value of a key, so that a key and a key
	// it prefixes ([a] and [a, b]) can coexist: "a~" is a file, "a/" a directory.
	// "~" is not a valid key character, hence never ambiguous.
	leafSuffix = "~"
	tmpSuffix  = ".tmp"
	// backupLayout is RFC 3339 in basic form: Windows forbids ':' in names, and
	// the layout still sorts chronologically.
	backupLayout = "20060102T150405Z"

	magic     = "IKV1"
	headerLen = len(magic) + sha256.Size
)

var (
	// ErrNotFound is returned when a key has no value.
	ErrNotFound = errors.New("store: not found")
	// ErrCorrupt is returned when a stored value fails verification and no
	// intact backup exists. The damaged file has been quarantined.
	ErrCorrupt = errors.New("store: corrupt value")
	// ErrBadKey is returned for a key that is empty, too deep or has an
	// invalid component.
	ErrBadKey = errors.New("store: invalid key")
	// ErrTooLarge is returned for a value over [MaxValue].
	ErrTooLarge = errors.New("store: value too large")
	// ErrKeyHash is returned when an "incremental" key is not the SHA-256 of
	// its data.
	ErrKeyHash = errors.New("store: incremental key does not hash its data")
)

// Entry is one key and its value, as returned by [KV.LoadRange].
type Entry struct {
	// Data is the verified value.
	Data []byte
	// Key is the full key.
	Key []string
}

// KV is the verified key-value store. Methods are safe for concurrent use.
// Create one with [Open].
type KV struct {
	r  *os.Root
	mu sync.Mutex
}

// Open returns the store living in r, creating repo/ and quarantine/ (0o700)
// and removing temporary files left by interrupted writes. The caller keeps
// ownership of r and closes it after the store is no longer used.
func Open(r *os.Root) (*KV, error) {
	for _, d := range []string{repoDir, quarantineDir} {
		if err := r.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("store: create %s: %w", d, err)
		}
	}
	err := fs.WalkDir(r.FS(), repoDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && strings.HasSuffix(p, tmpSuffix) {
			return r.Remove(p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: sweep %s: %w", repoDir, err)
	}
	return &KV{r: r}, nil
}

// ValidKey reports whether key is a valid full key: 1 to [MaxDepth]
// components. The error wraps [ErrBadKey].
func ValidKey(key []string) error {
	if len(key) == 0 {
		return fmt.Errorf("%w: empty", ErrBadKey)
	}
	return ValidPrefix(key)
}

// ValidPrefix is like [ValidKey] but also accepts the empty prefix.
func ValidPrefix(prefix []string) error {
	if len(prefix) > MaxDepth {
		return fmt.Errorf("%w: %d components, max %d", ErrBadKey, len(prefix), MaxDepth)
	}
	for _, c := range prefix {
		if !validComponent(c) {
			return fmt.Errorf("%w: component of %d bytes", ErrBadKey, len(c))
		}
	}
	return nil
}

func validComponent(c string) bool {
	if c == "" || len(c) > MaxComponent || c == "." || c == ".." {
		return false
	}
	for i := range len(c) {
		b := c[i]
		ok := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' ||
			b == '.' || b == '_' || b == '-'
		if !ok {
			return false
		}
	}
	return true
}

// dirPath is the directory holding the keys below prefix, relative to the root.
func dirPath(prefix []string) string {
	return path.Join(append([]string{repoDir}, prefix...)...)
}

// filePath is the file holding the value of key, relative to the root.
func filePath(key []string) string {
	return dirPath(key) + leafSuffix
}

// isIncremental reports whether key names an automerge-repo incremental chunk,
// whose last component is the SHA-256 of the data.
func isIncremental(key []string) bool {
	return len(key) >= 2 && key[len(key)-2] == "incremental"
}

func encodeValue(data []byte) []byte {
	sum := sha256.Sum256(data)
	b := make([]byte, 0, headerLen+len(data))
	b = append(b, magic...)
	b = append(b, sum[:]...)
	return append(b, data...)
}

// checkSave validates the key and value of a Save.
func checkSave(key []string, data []byte) error {
	if err := ValidKey(key); err != nil {
		return err
	}
	if len(data) > MaxValue {
		return fmt.Errorf("%w: %d bytes", ErrTooLarge, len(data))
	}
	if isIncremental(key) {
		sum := sha256.Sum256(data)
		if key[len(key)-1] != hex.EncodeToString(sum[:]) {
			return ErrKeyHash
		}
	}
	return nil
}

// Save stores data under key, atomically replacing any previous value.
func (k *KV) Save(key []string, data []byte) error {
	if err := checkSave(key, data); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.write(key, data)
}

func (k *KV) write(key []string, data []byte) error {
	name := filePath(key)
	if err := k.r.MkdirAll(path.Dir(name), 0o700); err != nil {
		return fmt.Errorf("store: save %v: %w", key, err)
	}
	if err := WriteAtomic(k.r, name, encodeValue(data), 0o600); err != nil {
		return fmt.Errorf("store: save %v: %w", key, err)
	}
	return nil
}

// Load returns the value of key. It returns [ErrNotFound] if there is none.
// A value that fails verification is quarantined; Load then returns the
// latest intact backup of it, or an error wrapping [ErrCorrupt] if there is
// none (the next Load reports [ErrNotFound]).
func (k *KV) Load(key []string) ([]byte, error) {
	if err := ValidKey(key); err != nil {
		return nil, err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.load(key)
}

func (k *KV) load(key []string) ([]byte, error) {
	name := filePath(key)
	data, err := k.read(name, key)
	switch {
	case err == nil:
		return data, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, ErrNotFound
	case errors.Is(err, ErrCorrupt):
		return k.recover(name, key, err)
	}
	return nil, fmt.Errorf("store: load %v: %w", key, err)
}

// read returns the verified data of the file name, which holds key.
func (k *KV) read(name string, key []string) ([]byte, error) {
	f, err := k.r.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only handle
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(MaxValue+headerLen) {
		return nil, fmt.Errorf("%w: not a regular file of sane size", ErrCorrupt)
	}
	buf, err := io.ReadAll(io.LimitReader(f, info.Size()+1))
	if err != nil {
		return nil, err
	}
	if len(buf) < headerLen || string(buf[:len(magic)]) != magic {
		return nil, fmt.Errorf("%w: bad header", ErrCorrupt)
	}
	data := buf[headerLen:]
	sum := sha256.Sum256(data)
	if !bytes.Equal(sum[:], buf[len(magic):headerLen]) {
		return nil, fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
	}
	if isIncremental(key) && key[len(key)-1] != hex.EncodeToString(sum[:]) {
		return nil, fmt.Errorf("%w: incremental key does not hash its data", ErrCorrupt)
	}
	return data, nil
}

// recover quarantines the damaged file name and restores key from the latest
// intact backup. cause is the verification error.
func (k *KV) recover(name string, key []string, cause error) ([]byte, error) {
	if err := k.quarantine(name, key); err != nil {
		return nil, fmt.Errorf("store: quarantine %v: %w", key, errors.Join(err, cause))
	}
	rel := strings.TrimPrefix(name, repoDir+"/")
	backups, _ := k.dirNames(backupsDir) // no backups yet is not an error
	for _, b := range slices.Backward(backups) {
		data, err := k.read(path.Join(backupsDir, b, rel), key)
		if err != nil {
			continue // missing from, or damaged in, this backup: try the older one
		}
		if err := k.write(key, data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, fmt.Errorf("store: %v quarantined, no intact backup: %w", key, cause)
}

func (k *KV) quarantine(name string, key []string) error {
	if err := k.r.MkdirAll(quarantineDir, 0o700); err != nil {
		return err
	}
	last := key[len(key)-1]
	target := path.Join(quarantineDir, rand.Text()[:8]+"-"+last[:min(len(last), 64)])
	return k.r.Rename(name, target)
}

// dirNames returns the sorted names of the entries of directory dir.
func (k *KV) dirNames(dir string) ([]string, error) {
	d, err := k.r.Open(dir)
	if err != nil {
		return nil, err
	}
	names, err := d.Readdirnames(-1)
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	slices.Sort(names)
	return names, err
}

// Remove deletes the value of key. Removing an absent key is not an error.
func (k *KV) Remove(key []string) error {
	if err := ValidKey(key); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.r.Remove(filePath(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: remove %v: %w", key, err)
	}
	return nil
}

// RemoveRange deletes every value whose key starts with prefix, which must
// not be empty. Removing an absent range is not an error.
func (k *KV) RemoveRange(prefix []string) error {
	if err := ValidKey(prefix); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.r.Remove(filePath(prefix)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: remove range %v: %w", prefix, err)
	}
	if err := k.r.RemoveAll(dirPath(prefix)); err != nil {
		return fmt.Errorf("store: remove range %v: %w", prefix, err)
	}
	return nil
}

// LoadRange returns every value whose key starts with prefix (the prefix
// itself included), ordered by key. The empty prefix matches everything.
//
// Values that fail verification are quarantined and recovered like in
// [KV.Load]; those without an intact backup are left out and the returned
// error wraps [ErrCorrupt] alongside the entries that were read.
func (k *KV) LoadRange(prefix []string) ([]Entry, error) {
	if err := ValidPrefix(prefix); err != nil {
		return nil, err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []Entry
	var corrupt error
	if len(prefix) > 0 {
		if err := k.collect(prefix, &out, &corrupt); err != nil {
			return nil, err
		}
	}
	if err := k.walk(dirPath(prefix), prefix, &out, &corrupt); err != nil {
		return nil, err
	}
	return out, corrupt
}

// collect appends the value of key to out. A missing value is skipped and a
// corrupt one recorded in corrupt.
func (k *KV) collect(key []string, out *[]Entry, corrupt *error) error {
	data, err := k.load(key)
	switch {
	case err == nil:
		*out = append(*out, Entry{Key: slices.Clone(key), Data: data})
	case errors.Is(err, ErrNotFound):
	case errors.Is(err, ErrCorrupt):
		*corrupt = err
	default:
		return err
	}
	return nil
}

// walk collects the values in directory dir, whose keys start with key.
// Symbolic links and files that are not store values are ignored.
func (k *KV) walk(dir string, key []string, out *[]Entry, corrupt *error) error {
	d, err := k.r.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: list %v: %w", key, err)
	}
	entries, err := d.ReadDir(-1)
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("store: list %v: %w", key, err)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir():
			if len(key)+1 >= MaxDepth || !validComponent(name) {
				continue
			}
			if err := k.walk(path.Join(dir, name), append(slices.Clone(key), name), out, corrupt); err != nil {
				return err
			}
		case e.Type().IsRegular() && strings.HasSuffix(name, leafSuffix):
			comp := strings.TrimSuffix(name, leafSuffix)
			if !validComponent(comp) {
				continue
			}
			if err := k.collect(append(slices.Clone(key), comp), out, corrupt); err != nil {
				return err
			}
		}
	}
	return nil
}

// Backup snapshots every value into backups/<now>/ (UTC, RFC 3339 basic
// form) with hard links, which costs no data copy: writes replace files
// atomically, so a linked backup keeps the content it had. It then deletes
// the oldest backups, keeping the keep newest (at least 1).
func (k *KV) Backup(now time.Time, keep int) error {
	if keep < 1 {
		return fmt.Errorf("store: backup: keep must be at least 1, got %d", keep)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	dest := path.Join(backupsDir, now.UTC().Format(backupLayout))
	err := fs.WalkDir(k.r.FS(), repoDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := path.Join(dest, strings.TrimPrefix(strings.TrimPrefix(p, repoDir), "/"))
		switch {
		case d.IsDir():
			return k.r.MkdirAll(target, 0o700)
		case d.Type().IsRegular() && !strings.HasSuffix(p, tmpSuffix):
			err := k.r.Link(p, target)
			if errors.Is(err, fs.ErrExist) || errors.Is(err, fs.ErrNotExist) {
				return nil // already in this backup, or removed meanwhile
			}
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: backup: %w", err)
	}
	names, err := k.dirNames(backupsDir)
	if err != nil {
		return fmt.Errorf("store: backup: %w", err)
	}
	for _, old := range names[:max(0, len(names)-keep)] {
		if err := k.r.RemoveAll(path.Join(backupsDir, old)); err != nil {
			return fmt.Errorf("store: prune backup %s: %w", old, err)
		}
	}
	return nil
}
