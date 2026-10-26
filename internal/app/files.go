package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport"
)

const (
	// maxFetching bounds the blobs [Files] has asked the service for at once:
	// the service downloads four, and the next ones wait in line.
	maxFetching = 2 * blob.MaxTransfers
	// retryNoPeer is how long a blob that no peer had waits before it is asked
	// for again, unless a peer connects first.
	retryNoPeer = 30 * time.Second
	// retryBroken is how long a blob that failed its checks waits.
	retryBroken = 5 * time.Minute
	// recentFor is how long a blob that was just stored here stays referenced
	// without the index saying so: the time for the index to reach the webview.
	recentFor = time.Hour
	// gcEvery is how often the unreferenced blobs are looked for.
	gcEvery = 24 * time.Hour
)

// failure says until when a blob is left alone after a failed attempt, and
// whether it failed only for lack of a peer that has it.
type failure struct {
	until  time.Time
	noPeer bool
}

// FilesOptions tunes [NewFiles]. Allowed is required.
type FilesOptions struct {
	// Allowed says whether a peer may be served and asked for blobs: an active
	// member of the project, not suspended.
	Allowed func(transport.PeerID) bool
	// Drop, if not nil, closes the connections to a peer that sent forged
	// slices three times.
	Drop func(transport.PeerID)
	// Log receives the diagnostics, never file contents. Nil discards them.
	Log *slog.Logger
	// Blob tunes the blob store: quotas, clock. Its Log defaults to Log.
	Blob blob.Options
	// Timeout bounds the wait for a peer's answer to one request; zero means
	// the blob service's default, 30 s.
	Timeout time.Duration
}

// Files is the multi-file side of the open project (ADR 0023): the
// [bridge.Storage] that the store stream serves, which is the [project.Project]
// plus the blob store, and the machinery that brings the files of the project
// tree in.
//
// The webview materializes the project tree from the index and sends it with
// [Files.SetTree]; Go projects the text files and folders at once. For every
// blob of the tree it asks the connected members, over the bulk lane, for what
// is not in the store, 4 at a time, the smallest first, and writes the visible
// copy once the blob is verified: nothing appears in the folder before. It
// serves its own blobs to the members, only those the project refers to
// ([Files.SetReferenced]).
//
// Create one with [NewFiles] and run it with [Files.Run].
type Files struct {
	*project.Project
	blobs     *blob.Store
	blobsRoot *os.Root
	svc       *blob.Service
	log       *slog.Logger

	mu      sync.Mutex
	refs    map[blob.Hash]struct{}  // what the index refers to, trash and packages included
	wanted  map[blob.Hash]blob.Ref  // the blobs of the tree
	paths   map[blob.Hash][]string  // where the tree puts each
	done    map[blob.Hash]bool      // visible copies written for the current paths
	failed  map[blob.Hash]failure   // blobs not to ask for yet
	refused map[blob.Hash]struct{}  // blobs over the size limit: never asked for
	flying  map[blob.Hash]struct{}  // asked for now
	paused  bool                    // the project is over its quota, waiting for the user
	recent  map[blob.Hash]time.Time // blobs stored here, referenced until the index says so
	known   bool                    // the webview has said what it refers to
	nextGC  time.Time               // when the unreferenced blobs are looked for next
	wake    chan struct{}           // capacity 1: "something changed"
	now     func() time.Time
	closing sync.Once
}

var _ bridge.Storage = (*Files)(nil)

// NewFiles opens the blob store of the project p, in .inkwell/blobs, and the
// pull service over bulk, the bulk lane of the network node. It consumes the
// events of bulk: nothing else may read them. Close the result after [Files.Run]
// has returned, before closing p.
func NewFiles(p *project.Project, bulk transport.Transport, o FilesOptions) (*Files, error) {
	if o.Allowed == nil {
		return nil, errors.New("app: FilesOptions.Allowed is required")
	}
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	root, err := p.OpenDir("blobs")
	if err != nil {
		return nil, fmt.Errorf("opening blob store: %w", err)
	}
	if o.Blob.Log == nil {
		o.Blob.Log = log
	}
	store, err := blob.Open(root, o.Blob)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	f := &Files{
		Project: p, blobs: store, blobsRoot: root, log: log,
		refs:    map[blob.Hash]struct{}{},
		recent:  map[blob.Hash]time.Time{},
		wanted:  map[blob.Hash]blob.Ref{},
		paths:   map[blob.Hash][]string{},
		done:    map[blob.Hash]bool{},
		failed:  map[blob.Hash]failure{},
		refused: map[blob.Hash]struct{}{},
		flying:  map[blob.Hash]struct{}{},
		wake:    make(chan struct{}, 1),
		now:     o.Blob.Now,
	}
	if f.now == nil {
		f.now = time.Now
	}
	f.svc, err = blob.NewService(bulk, store, blob.ServiceOptions{
		Referenced: f.referenced,
		Allowed:    o.Allowed,
		Drop:       o.Drop,
		Log:        log,
		Timeout:    o.Timeout,
	})
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return f, nil
}

// Close releases the blob store. Call it once [Files.Run] has returned.
func (f *Files) Close() error {
	var err error
	f.closing.Do(func() { err = f.blobsRoot.Close() })
	return err
}

// Blobs returns the blob store, for tests and for the shell's own use.
func (f *Files) Blobs() *blob.Store { return f.blobs }

// referenced says whether the project refers to h: the index (the trash and
// the packages with it), or the current tree.
func (f *Files) referenced(h blob.Hash) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.refs[h]; ok {
		return true
	}
	if _, ok := f.wanted[h]; ok {
		return true
	}
	until, ok := f.recent[h]
	return ok && f.now().Before(until)
}

// poke wakes the loop of [Files.Run].
func (f *Files) poke() {
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

// SetTree projects the tree of the webview (see [project.Project.ApplyTree]) and
// starts bringing in the blobs it lists. It returns how many entries were
// refused; a refusal of a hostile name is a security event in the log.
func (f *Files) SetTree(entries []project.TreeEntry) (int, error) {
	res, err := f.ApplyTree(entries)
	for _, rej := range res.Rejected {
		f.log.Warn("tree entry refused", "event", "security", "path", rej.Path, "why", rej.Err)
	}
	f.mu.Lock()
	wanted := make(map[blob.Hash]blob.Ref, len(res.Blobs))
	paths := make(map[blob.Hash][]string, len(res.Blobs))
	for _, e := range res.Blobs {
		h := blob.Hash(e.Hash)
		wanted[h] = blob.Ref{Hash: h, Size: e.Size}
		paths[h] = append(paths[h], e.Path)
	}
	for h, ps := range paths {
		slices.Sort(ps)
		if !slices.Equal(ps, f.paths[h]) {
			delete(f.done, h) // a new place for the file: its visible copy is due
		}
	}
	f.wanted, f.paths = wanted, paths
	f.mu.Unlock()
	f.poke()
	return len(res.Rejected), err
}

// SetReferenced records the hashes the project refers to: only those are served
// and kept. The first call also lets the garbage collection run, since before
// it nothing is known to be referenced.
func (f *Files) SetReferenced(hashes [][bridge.HashSize]byte) error {
	refs := make(map[blob.Hash]struct{}, len(hashes))
	for _, h := range hashes {
		refs[blob.Hash(h)] = struct{}{}
	}
	f.mu.Lock()
	f.refs, f.known = refs, true
	for h := range refs {
		delete(f.recent, h) // the index has caught up
	}
	f.mu.Unlock()
	f.poke()
	return nil
}

// PutBlob stores the project file rel in the blob store and returns its hash
// and size. The file is then referenced at once, so that the members can ask
// for it before the index reaches them. It returns a [*bridge.TooLargeError]
// for a file over the size limit.
func (f *Files) PutBlob(rel string) ([bridge.HashSize]byte, int64, error) {
	file, info, err := f.OpenFile(rel)
	if err != nil {
		return [bridge.HashSize]byte{}, 0, err
	}
	defer func() { _ = file.Close() }() // read-only handle
	sum := sha256.New()
	h, err := f.blobs.Put(io.TeeReader(file, sum), info.Size())
	if errors.Is(err, blob.ErrTooLarge) {
		limit, _ := f.blobs.Limits()
		return [bridge.HashSize]byte{}, 0, &bridge.TooLargeError{Limit: limit}
	}
	if err != nil {
		return [bridge.HashSize]byte{}, 0, err
	}
	// The user asked for this file to join the project: it is Inkwell's from now
	// on, so that the tree, which a peer writes, may list it.
	if err := f.Claim(rel, hex.EncodeToString(sum.Sum(nil))); err != nil {
		return [bridge.HashSize]byte{}, 0, err
	}
	f.mu.Lock()
	f.recent[h] = f.now().Add(recentFor)
	f.mu.Unlock()
	return h, info.Size(), nil
}

// BlobStatus is the progress of the blobs of the project, for the sidebar:
// "12 of 40 files, 3.2 MB".
type BlobStatus struct {
	// Refused lists the paths of the blobs over the size limit, which are not
	// brought in.
	Refused []string
	// Files is the number of blobs the tree lists.
	Files int
	// Stored is how many of them are in the store.
	Stored int
	// Bytes is their total size, and StoredBytes that of the stored ones.
	Bytes, StoredBytes int64
	// Limit is the project quota in bytes.
	Limit int64
	// NeedsConfirmation is true when bringing in the rest would take the
	// project over its quota: nothing more is downloaded until the user
	// confirms with [Files.Confirm].
	NeedsConfirmation bool
}

// Status reports the progress of the blobs of the tree.
func (f *Files) Status() BlobStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := BlobStatus{NeedsConfirmation: f.paused}
	_, st.Limit = f.blobs.Limits()
	for h, ref := range f.wanted {
		st.Files++
		st.Bytes += ref.Size
		if f.blobs.Has(h) {
			st.Stored++
			st.StoredBytes += ref.Size
		}
		if _, refused := f.refused[h]; refused {
			st.Refused = append(st.Refused, f.paths[h]...)
		}
	}
	slices.Sort(st.Refused)
	return st
}

// Confirm accepts a project over its quota, after the user agreed: the blobs
// that were waiting are brought in.
func (f *Files) Confirm() {
	f.blobs.Confirm()
	f.mu.Lock()
	f.paused = false
	f.mu.Unlock()
	f.poke()
}

// Run serves the blobs to the members and brings in those the tree lists, until
// ctx is done. It returns the error that ended the service, ctx's error when ctx
// did, and waits for its fetches.
func (f *Files) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	svcCtx, stop := context.WithCancel(ctx)
	defer stop()
	svcDone := make(chan error, 1)
	wg.Go(func() { svcDone <- f.svc.Run(svcCtx) })
	timer := time.NewTimer(retryNoPeer)
	defer timer.Stop()
	for {
		peers := f.svc.PeersChanged() // before pump, so that no arrival is missed
		f.pump(ctx, &wg)
		f.collect()
		timer.Reset(retryNoPeer)
		select {
		case <-f.wake:
		case <-peers:
			f.retryNoPeer()
		case <-timer.C:
		case err := <-svcDone:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// retryNoPeer forgets that the blobs no peer had were not found: a peer came or
// went, and the next one may have them.
func (f *Files) retryNoPeer() {
	f.mu.Lock()
	defer f.mu.Unlock()
	maps.DeleteFunc(f.failed, func(_ blob.Hash, fl failure) bool { return fl.noPeer })
}

// pump starts the fetches that are due: blobs of the tree whose visible copy is
// not written, the smallest first, a few at a time.
func (f *Files) pump(ctx context.Context, wg *sync.WaitGroup) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.paused {
		return
	}
	now := f.now()
	var due []blob.Ref
	for h, ref := range f.wanted {
		_, refused := f.refused[h]
		_, flying := f.flying[h]
		if !f.done[h] && !refused && !flying && now.After(f.failed[h].until) {
			due = append(due, ref)
		}
	}
	slices.SortFunc(due, func(a, b blob.Ref) int {
		return cmp.Or(cmp.Compare(a.Size, b.Size), slices.Compare(a.Hash[:], b.Hash[:]))
	})
	for _, ref := range due {
		if len(f.flying) >= maxFetching {
			break
		}
		f.flying[ref.Hash] = struct{}{}
		wg.Go(func() { f.fetch(ctx, ref) })
	}
}

// fetch brings in one blob and writes its visible copies.
func (f *Files) fetch(ctx context.Context, ref blob.Ref) {
	err := f.svc.Fetch(ctx, ref)
	if err == nil {
		err = f.materialize(ref.Hash)
	}
	f.mu.Lock()
	delete(f.flying, ref.Hash)
	switch {
	case err == nil:
		f.done[ref.Hash] = true
	case ctx.Err() != nil:
	case errors.Is(err, blob.ErrNoPeer):
		f.failed[ref.Hash] = failure{until: f.now().Add(retryNoPeer), noPeer: true}
	case errors.Is(err, blob.ErrProjectQuota):
		f.paused = true
	case errors.Is(err, blob.ErrTooLarge):
		f.refused[ref.Hash] = struct{}{}
		f.log.Warn("blob over the size limit not brought in", "hash", ref.Hash, "size", ref.Size)
	default:
		f.failed[ref.Hash] = failure{until: f.now().Add(retryBroken)}
		f.log.Warn("bringing in a blob failed", "hash", ref.Hash, "err", err)
	}
	f.mu.Unlock()
	f.poke()
}

// materialize writes the visible copy of the stored blob h at every path the
// tree gives it. A path whose file was edited outside Inkwell is left alone
// and logged: the user decides.
func (f *Files) materialize(h blob.Hash) error {
	var errs []error
	for _, rel := range f.BlobPaths(h) {
		switch err := f.writeCopy(rel, h); {
		case err == nil, errors.Is(err, project.ErrBlobMoved):
		case errors.Is(err, project.ErrExternalEdit):
			f.log.Warn("file edited outside Inkwell, not replaced", "path", rel)
		case errors.Is(err, project.ErrBlobMismatch):
			_ = f.blobs.Remove(h) // rotten on disk: let it be fetched again
			errs = append(errs, blob.ErrCorrupt)
		default:
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// writeCopy streams the stored blob h into the visible file rel; the project
// checks the content against h as it goes, so nothing is held in memory.
func (f *Files) writeCopy(rel string, h blob.Hash) error {
	src, _, err := f.blobs.Reader(h)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }() // read-only handle
	return f.WriteBlob(rel, h, src)
}

// collect removes the blobs that nothing has referenced for 30 days, once a day,
// when the webview has said what is referenced.
func (f *Files) collect() {
	f.mu.Lock()
	now := f.now()
	if !f.known || now.Before(f.nextGC) {
		f.mu.Unlock()
		return
	}
	f.nextGC = now.Add(gcEvery)
	f.mu.Unlock()
	res, err := f.blobs.GC(f.referenced, blob.UnreferencedTTL)
	if err != nil {
		f.log.Warn("blob garbage collection", "err", err)
	}
	if res.Removed > 0 {
		f.log.Info("unreferenced blobs removed", "removed", res.Removed, "freed", res.Freed)
	}
}

// pending returns the hashes of the tree that are not stored, for tests.
func (f *Files) pending() []blob.Hash {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []blob.Hash
	for _, h := range slices.SortedFunc(maps.Keys(f.wanted), func(a, b blob.Hash) int { return slices.Compare(a[:], b[:]) }) {
		if !f.blobs.Has(h) {
			out = append(out, h)
		}
	}
	return out
}
