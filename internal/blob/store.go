// Package blob is the store of a project's binary files, and the protocol
// that moves them between peers (ADR 0023).
//
// A file is named by its BLAKE3 hash. The store keeps it under an [os.Root],
//
//	blobs/
//	  ab/abcd…ef           the file, read-only
//	  ab/abcd…ef.obao      its Bao outboard tree, about 0.4 % of the file
//	  partial/<hash>       a download in progress
//	  partial/<hash>.map   which groups of it are verified
//
// The outboard tree cuts the file into groups of 16 KiB whose hashes lead to
// the file's hash. With it, a peer can serve any range of the file as a slice
// that the receiver checks against the hash alone: a forged byte is caught
// within 16 KiB, and a download that stops resumes without asking again for
// what was verified.
//
// A [Service] pulls blobs from the peers of a transport (the bulk lane of the
// network node) and serves them to the members, only the hashes the project
// references. The store itself has no network: [Store.Put], [Store.Get] and
// the quotas are usable alone.
//
// The visible copy of a blob in the project folder is not this package's
// business, and is never a hard link to the store: an external edit would
// corrupt it.
package blob

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"lukechampine.com/blake3"
	"lukechampine.com/blake3/bao"
)

// Defaults of [Options].
const (
	// DefaultMaxFile is the largest file accepted: 100 MB.
	DefaultMaxFile = 100 << 20
	// DefaultMaxProject is the total above which an arrival must confirm: 2 GB.
	DefaultMaxProject = 2 << 30
	// defaultReserve is the free space always left on the disk.
	defaultReserve = 64 << 20
	// UnreferencedTTL is how long a blob that nothing references is kept.
	UnreferencedTTL = 30 * 24 * time.Hour

	// maxParents is more than the length header and the parent hashes of a
	// slice take, for the files this package accepts.
	maxParents = 8 + 64*64

	partialDir = "partial"
	obaoSuffix = ".obao"
	tmpSuffix  = ".tmp"
)

var (
	// ErrNotFound is returned for a blob that the store does not hold.
	ErrNotFound = errors.New("blob: not found")
	// ErrTooLarge is returned for a file over the limit of [Options.MaxFile].
	ErrTooLarge = errors.New("blob: file over the size limit")
	// ErrProjectQuota is returned, wrapped in a [*QuotaError], when storing a
	// blob would take the project over [Options.MaxProject] and the user has
	// not confirmed (see [Store.Confirm]).
	ErrProjectQuota = errors.New("blob: project over its size limit")
	// ErrNoSpace is returned when the disk lacks the room for a file.
	ErrNoSpace = errors.New("blob: not enough disk space")
	// ErrCorrupt is returned for a blob whose content does not match its hash;
	// the damaged copy has been removed.
	ErrCorrupt = errors.New("blob: content does not match its hash")
	// ErrSizeMismatch is returned when a file is not the size announced for it.
	ErrSizeMismatch = errors.New("blob: size does not match the announced size")
)

// QuotaError says by how much a blob would exceed the project quota. It
// wraps [ErrProjectQuota].
type QuotaError struct {
	// Need is the total the project would take once the blob is stored.
	Need int64
	// Limit is the project quota.
	Limit int64
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("blob: project would take %d bytes, over its limit of %d", e.Need, e.Limit)
}

// Is reports that a QuotaError is an [ErrProjectQuota].
func (e *QuotaError) Is(target error) bool { return target == ErrProjectQuota }

// Options tunes a [Store]. The zero value is valid.
type Options struct {
	// Log receives the diagnostics. Nil discards them.
	Log *slog.Logger
	// FreeSpace reports the free bytes of the disk holding the store. Nil
	// asks the operating system; an error from it disables the check.
	FreeSpace func() (uint64, error)
	// Now is the clock of the garbage collection; nil means time.Now.
	Now func() time.Time
	// MaxFile is the largest file, in bytes; zero means [DefaultMaxFile].
	MaxFile int64
	// MaxProject is the size of the store above which new blobs need the
	// user's confirmation; zero means [DefaultMaxProject].
	MaxProject int64
	// Reserve is the free space left on the disk; zero means 64 MiB.
	Reserve int64
}

// Store is the content-addressed store of one project. Create it with [Open];
// its methods are safe for concurrent use.
type Store struct {
	root *os.Root
	log  *slog.Logger
	opt  Options
	// used is the size of the stored blobs, under mu; holds is the size
	// promised to the files being received, by key: a download keeps its hold
	// while its partial file is on disk, so that stopped downloads still count.
	used      int64
	holds     map[string]int64
	mu        sync.Mutex
	confirmed bool
}

// Open opens the store in root, which it creates the layout in. The caller
// closes root after the store is no longer used.
func Open(root *os.Root, o Options) (*Store, error) {
	if o.MaxFile <= 0 {
		o.MaxFile = DefaultMaxFile
	}
	if o.MaxProject <= 0 {
		o.MaxProject = DefaultMaxProject
	}
	if o.Reserve <= 0 {
		o.Reserve = defaultReserve
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.FreeSpace == nil {
		o.FreeSpace = func() (uint64, error) { return freeSpace(root.Name()) }
	}
	s := &Store{root: root, opt: o, log: o.Log, holds: map[string]int64{}}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	if err := root.MkdirAll(partialDir, 0o700); err != nil {
		return nil, fmt.Errorf("blob: create %s: %w", partialDir, err)
	}
	if err := s.scan(); err != nil {
		return nil, err
	}
	return s, nil
}

// scan adds up the stored blobs and removes the temporary files that a killed
// process left.
func (s *Store) scan() error {
	dirs, err := fs.ReadDir(s.root.FS(), ".")
	if err != nil {
		return fmt.Errorf("blob: list store: %w", err)
	}
	for _, d := range dirs {
		if !d.IsDir() && strings.HasSuffix(d.Name(), tmpSuffix) {
			_ = s.root.Remove(d.Name()) // best effort: a stale temporary file
		}
		if !d.IsDir() || d.Name() == partialDir {
			continue
		}
		entries, err := fs.ReadDir(s.root.FS(), d.Name())
		if err != nil {
			return fmt.Errorf("blob: list %s: %w", d.Name(), err)
		}
		for _, e := range entries {
			name := d.Name() + "/" + e.Name()
			switch {
			case strings.HasSuffix(e.Name(), tmpSuffix):
				_ = s.root.Remove(name) // best effort: a stale temporary file
			case isHashName(e.Name()):
				if info, err := e.Info(); err == nil && info.Mode().IsRegular() {
					s.used += info.Size()
				}
			}
		}
	}
	partials, err := fs.ReadDir(s.root.FS(), partialDir)
	if err != nil {
		return fmt.Errorf("blob: list %s: %w", partialDir, err)
	}
	for _, e := range partials {
		if strings.HasSuffix(e.Name(), tmpSuffix) {
			_ = s.root.Remove(partialDir + "/" + e.Name())
			continue
		}
		if info, err := e.Info(); err == nil && isHashName(e.Name()) && info.Mode().IsRegular() {
			s.holds[e.Name()] = info.Size() // a download that was stopped: it keeps its share
		}
	}
	return nil
}

// isHashName reports whether name is 64 lower-case hex digits.
func isHashName(name string) bool {
	if len(name) != 2*HashSize {
		return false
	}
	for i := range len(name) {
		if c := name[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// blobPath returns the path of the blob h below the root.
func blobPath(h Hash) string {
	x := h.hex()
	return x[:2] + "/" + x
}

// Used returns the total size of the stored blobs, in bytes.
func (s *Store) Used() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.used
}

// Limits returns the size limit of a file and of the project.
func (s *Store) Limits() (file, project int64) { return s.opt.MaxFile, s.opt.MaxProject }

// Has reports whether the blob h is stored.
func (s *Store) Has(h Hash) bool {
	info, err := s.root.Stat(blobPath(h))
	return err == nil && info.Mode().IsRegular()
}

// Size returns the size of the stored blob h, or [ErrNotFound].
func (s *Store) Size(h Hash) (int64, error) {
	info, err := s.root.Stat(blobPath(h))
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("blob: stat %s: %w", h.short(), err)
	}
	return info.Size(), nil
}

// Confirm records that the user accepts a project over [Options.MaxProject]:
// the quota no longer refuses arrivals until the store is opened again.
func (s *Store) Confirm() {
	s.mu.Lock()
	s.confirmed = true
	s.mu.Unlock()
}

// Ref is a blob a project refers to: its hash and the size the index gives.
type Ref struct {
	// Hash names the blob.
	Hash Hash
	// Size is the length in bytes announced by the index. It is checked
	// against the first slice that arrives, never trusted.
	Size int64
}

// Plan is what bringing the missing blobs of a project in would cost.
type Plan struct {
	// Fetch lists the blobs to download, without duplicates.
	Fetch []Ref
	// TooLarge lists the blobs over the per-file limit: they are refused.
	TooLarge []Ref
	// Bytes is the total size of Fetch.
	Bytes int64
	// Used is what the store holds already.
	Used int64
	// Limit is the project quota.
	Limit int64
	// NeedsConfirmation is true when Used + Bytes is over Limit and the user
	// has not confirmed: the arrival asks before it downloads (PRJ-11).
	NeedsConfirmation bool
}

// Plan sorts refs into what to download and what to refuse, and tells whether
// the user must confirm first. It changes nothing.
func (s *Store) Plan(refs []Ref) Plan {
	s.mu.Lock()
	p := Plan{Used: s.used, Limit: s.opt.MaxProject}
	confirmed := s.confirmed
	s.mu.Unlock()
	seen := make(map[Hash]bool, len(refs))
	for _, r := range refs {
		switch {
		case seen[r.Hash] || s.Has(r.Hash):
		case r.Size < 0 || r.Size > s.opt.MaxFile:
			p.TooLarge = append(p.TooLarge, r)
		default:
			p.Fetch = append(p.Fetch, r)
			p.Bytes += r.Size
		}
		seen[r.Hash] = true
	}
	p.NeedsConfirmation = !confirmed && p.Used+p.Bytes > p.Limit
	return p
}

// hold checks that a file of size bytes may be received and sets that room
// aside under key until [Store.unhold]. The room counts against the project
// quota and the free space, with every other hold, so that downloads that stop
// half way keep their share and cannot add up past the quota. Holding again
// under the same key replaces the hold. It returns [ErrTooLarge], [*QuotaError]
// or [ErrNoSpace].
func (s *Store) hold(key string, size int64) error {
	if size < 0 || size > s.opt.MaxFile {
		return fmt.Errorf("%w: %d bytes, limit %d", ErrTooLarge, size, s.opt.MaxFile)
	}
	s.mu.Lock()
	others := int64(0)
	for k, n := range s.holds {
		if k != key {
			others += n
		}
	}
	if need := s.used + others + size; need > s.opt.MaxProject && !s.confirmed {
		s.mu.Unlock()
		return &QuotaError{Need: need, Limit: s.opt.MaxProject}
	}
	s.mu.Unlock()
	if free, ferr := s.opt.FreeSpace(); ferr != nil {
		s.log.Debug("blob: free space unknown", "err", ferr)
	} else if need := uint64(size) + uint64(bao.EncodedSize(int(size), groupLog, true)) + uint64(s.opt.Reserve) + uint64(others); free < need { // #nosec G115 -- sizes are within [0, MaxFile] and the holds are sums of them.
		return fmt.Errorf("%w: %d bytes free, %d needed", ErrNoSpace, free, need)
	}
	s.mu.Lock()
	s.holds[key] = size
	s.mu.Unlock()
	return nil
}

// unhold gives back the room held under key.
func (s *Store) unhold(key string) {
	s.mu.Lock()
	delete(s.holds, key)
	s.mu.Unlock()
}

// settle gives back the hold of the blob h unless a partial download of it is
// on disk: the file is stored, or its download was discarded.
func (s *Store) settle(h Hash) {
	if _, err := s.root.Lstat(partialDir + "/" + h.hex()); err != nil {
		s.unhold(h.hex())
	}
}

// PutBytes stores data and returns its hash. See [Store.Put].
func (s *Store) PutBytes(data []byte) (Hash, error) {
	return s.Put(bytes.NewReader(data), int64(len(data)))
}

// Put reads exactly size bytes from r, stores them and returns their hash. A
// blob already stored is not written again. It returns [ErrTooLarge] for a
// size over the per-file limit, [ErrSizeMismatch] if r holds more or less
// than size, [*QuotaError] and [ErrNoSpace] (see [Store.Plan] and
// [Store.Confirm] for the project quota).
func (s *Store) Put(r io.Reader, size int64) (Hash, error) {
	id := "put-" + rand.Text()
	if err := s.hold(id, size); err != nil {
		return Hash{}, err
	}
	defer s.unhold(id)
	name := partialDir + "/" + id
	f, err := s.root.OpenFile(name+tmpSuffix, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Hash{}, fmt.Errorf("blob: create temporary file: %w", err)
	}
	defer func() { _ = s.root.Remove(name + tmpSuffix) }() // gone once renamed
	n, err := io.Copy(f, io.LimitReader(r, size+1))
	if err == nil && n != size {
		err = fmt.Errorf("%w: %d bytes read, %d announced", ErrSizeMismatch, n, size)
	}
	if err != nil {
		_ = f.Close()
		return Hash{}, err
	}
	h, err := s.seal(f, size, name)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return h, err
}

// seal computes the hash and the outboard tree of the size bytes of f, which
// is a temporary file named tmp+tmpSuffix, and stores both under the hash.
func (s *Store) seal(f *os.File, size int64, tmp string) (Hash, error) {
	if err := f.Sync(); err != nil {
		return Hash{}, fmt.Errorf("blob: sync temporary file: %w", err)
	}
	h, err := s.outboard(f, size, tmp+obaoSuffix+tmpSuffix)
	if err != nil {
		return Hash{}, err
	}
	if err := s.install(h, size, tmp+tmpSuffix, tmp+obaoSuffix+tmpSuffix); err != nil {
		_ = s.root.Remove(tmp + obaoSuffix + tmpSuffix)
		return Hash{}, err
	}
	return h, nil
}

// outboard hashes the size bytes of f, writes their outboard tree to the file
// name and returns the hash.
func (s *Store) outboard(f *os.File, size int64, name string) (Hash, error) {
	out, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return Hash{}, fmt.Errorf("blob: create outboard: %w", err)
	}
	root, err := bao.Encode(out, io.NewSectionReader(f, 0, size), size, groupLog, true)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = s.root.Remove(name)
		return Hash{}, fmt.Errorf("blob: hash file: %w", err)
	}
	return root, nil
}

// install moves the files blobTmp and obaoTmp, which hold the verified blob h
// of size bytes, to their place in the store, outboard first so that a stored
// blob can always be served. If h is stored already they are removed.
func (s *Store) install(h Hash, size int64, blobTmp, obaoTmp string) error {
	final := blobPath(h)
	if s.Has(h) {
		_ = s.root.Remove(blobTmp)
		_ = s.root.Remove(obaoTmp)
		return nil
	}
	if err := s.root.MkdirAll(final[:2], 0o700); err != nil {
		return fmt.Errorf("blob: create %s: %w", final[:2], err)
	}
	if err := s.root.Chmod(obaoTmp, 0o400); err != nil {
		return fmt.Errorf("blob: protect outboard: %w", err)
	}
	if err := s.root.Chmod(blobTmp, 0o400); err != nil {
		return fmt.Errorf("blob: protect blob: %w", err)
	}
	if err := s.root.Rename(obaoTmp, final+obaoSuffix); err != nil {
		return fmt.Errorf("blob: store outboard: %w", err)
	}
	if err := s.root.Rename(blobTmp, final); err != nil {
		return fmt.Errorf("blob: store blob: %w", err)
	}
	s.mu.Lock()
	s.used += size
	s.mu.Unlock()
	return nil
}

// Get returns the content of the blob h after checking it against h. A blob
// that fails the check is removed and [ErrCorrupt] returned, so that it can be
// fetched again. It returns [ErrNotFound] for a blob that is not stored.
func (s *Store) Get(h Hash) ([]byte, error) {
	f, err := s.root.Open(blobPath(h))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("blob: open %s: %w", h.short(), err)
	}
	defer func() { _ = f.Close() }() // read-only handle
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("blob: stat %s: %w", h.short(), err)
	}
	if info.Size() > s.opt.MaxFile {
		_ = s.Remove(h)
		return nil, fmt.Errorf("%w: %d bytes", ErrCorrupt, info.Size())
	}
	data, err := io.ReadAll(io.LimitReader(f, info.Size()+1))
	if err != nil {
		return nil, fmt.Errorf("blob: read %s: %w", h.short(), err)
	}
	if blake3.Sum256(data) != h {
		_ = s.Remove(h)
		s.log.Warn("blob: stored file does not match its hash, removed", "hash", h.short())
		return nil, ErrCorrupt
	}
	return data, nil
}

// Reader opens the stored blob h for reading and returns it with its size, to
// stream a 100 MB file without holding it in memory. The content is not checked
// here: the caller hashes it as it reads, against h. The caller closes the file.
// It returns [ErrNotFound] for a blob that is not stored.
func (s *Store) Reader(h Hash) (*os.File, int64, error) {
	f, err := s.root.Open(blobPath(h))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, fmt.Errorf("blob: open %s: %w", h.short(), err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("blob: stat %s: %w", h.short(), err)
	}
	return f, info.Size(), nil
}

// Remove deletes the blob h and its outboard tree. Removing a blob that is
// not stored succeeds.
func (s *Store) Remove(h Hash) error {
	final := blobPath(h)
	size, err := s.Size(h)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range []string{final + obaoSuffix, final} {
		// A stored file is read-only, which Windows refuses to delete.
		if err := s.root.Chmod(name, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
		if err := s.root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("blob: remove %s: %w", h.short(), err)
	}
	s.mu.Lock()
	s.used -= size
	s.mu.Unlock()
	return nil
}

// ReadSlice returns the Bao slice encoding of the bytes [offset,
// offset+length) of the blob h, clamped to the end of the file. offset and
// length must be multiples of [GroupSize] and length at most [MaxSlice]. The
// slice is checked against h before it is returned: a blob that has rotted on
// disk is removed and [ErrCorrupt] returned, instead of being served. It
// returns [ErrNotFound] for a blob that is not stored.
func (s *Store) ReadSlice(h Hash, offset uint64, length uint32) (Slice, error) {
	if err := checkRange(offset, length, true); err != nil {
		return Slice{}, err
	}
	f, err := s.root.Open(blobPath(h))
	if errors.Is(err, fs.ErrNotExist) {
		return Slice{}, ErrNotFound
	}
	if err != nil {
		return Slice{}, fmt.Errorf("blob: open %s: %w", h.short(), err)
	}
	defer func() { _ = f.Close() }() // read-only handle
	info, err := f.Stat()
	if err != nil {
		return Slice{}, fmt.Errorf("blob: stat %s: %w", h.short(), err)
	}
	size := info.Size()
	if offset >= uint64(size) { // #nosec G115 -- a file size is not negative.
		return Slice{}, fmt.Errorf("%w: offset %d of a file of %d bytes", ErrNotFound, offset, size)
	}
	n := uint32(min(int64(length), size-int64(offset))) // #nosec G115 -- offset < size, and length is at most MaxSlice.
	tree, err := s.tree(h, f, size)
	if err != nil {
		return Slice{}, err
	}
	var buf bytes.Buffer
	buf.Grow(int(n) + maxParents)                                                                                                                       // the data, the header and the parent hashes: no regrowth
	if err := bao.ExtractSlice(&buf, io.NewSectionReader(f, int64(offset), int64(n)), bytes.NewReader(tree), groupLog, offset, uint64(n)); err != nil { // #nosec G115 -- offset ≤ size.
		return Slice{}, fmt.Errorf("blob: extract slice of %s: %w", h.short(), err)
	}
	if _, ok := bao.VerifySlice(buf.Bytes(), groupLog, offset, uint64(n), h); !ok {
		_ = s.Remove(h)
		s.log.Warn("blob: stored file does not match its hash, removed", "hash", h.short())
		return Slice{}, ErrCorrupt
	}
	return Slice{Hash: h, Offset: offset, Length: n, Bao: buf.Bytes()}, nil
}

// tree returns the outboard tree of the blob h, stored in f with size bytes.
// A missing or damaged tree is rebuilt from the file, which also checks the
// file against its hash.
func (s *Store) tree(h Hash, f *os.File, size int64) ([]byte, error) {
	final := blobPath(h) + obaoSuffix
	want := bao.EncodedSize(int(size), groupLog, true)
	if tree, err := s.root.ReadFile(final); err == nil && len(tree) == want {
		return tree, nil
	}
	tmp := partialDir + "/rebuild-" + rand.Text() + obaoSuffix + tmpSuffix
	got, err := s.outboard(f, size, tmp)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.root.Remove(tmp) }() // gone once renamed
	if got != h {
		_ = s.Remove(h)
		return nil, ErrCorrupt
	}
	tree, err := s.root.ReadFile(tmp)
	if err != nil {
		return nil, fmt.Errorf("blob: read rebuilt outboard: %w", err)
	}
	if err := s.root.Chmod(tmp, 0o400); err == nil {
		_ = s.root.Remove(final) // read-only, and now wrong
		_ = s.root.Rename(tmp, final)
	}
	return tree, nil
}

// List returns the hashes of the stored blobs, sorted.
func (s *Store) List() ([]Hash, error) {
	dirs, err := fs.ReadDir(s.root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("blob: list store: %w", err)
	}
	var out []Hash
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == partialDir {
			continue
		}
		entries, err := fs.ReadDir(s.root.FS(), d.Name())
		if err != nil {
			return nil, fmt.Errorf("blob: list %s: %w", d.Name(), err)
		}
		for _, e := range entries {
			if h, ok := hashFromName(e.Name()); ok && e.Type().IsRegular() {
				out = append(out, h)
			}
		}
	}
	slices.SortFunc(out, func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
	return out, nil
}

// hashFromName parses the 64 hex digits of a blob's file name.
func hashFromName(name string) (Hash, bool) {
	if !isHashName(name) {
		return Hash{}, false
	}
	h, err := ParseHash(hashPrefix + name)
	return h, err == nil
}
