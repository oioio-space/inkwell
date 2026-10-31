package app

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/trust"
)

const (
	trustDir = "trust"
	opExt    = ".op"
	rcvExt   = ".rcv" // when this device received the op of the same name.
	tmpExt   = ".tmp"
)

// opStore keeps the ops of one project's member list, one file per op named
// after its hash, in the project's .inkwell/trust directory. Files are
// written with [store.WriteAtomic]. Replacing the whole list swaps a staged
// sibling directory in (see [opStore.replace]), so that the list on disk is
// always a complete one.
type opStore struct {
	parent *os.Root // .inkwell, where trust and its staging siblings live.

	mu   sync.RWMutex // guards root: replace closes and reopens it.
	root *os.Root     // .inkwell/trust; nil while closed, reopened by the next use.
	// step, if not nil, runs before each step of [opStore.replace]; an error
	// aborts it. Tests use it to inject failures and to observe the disk.
	step func(name string) error
}

const (
	nextDir = trustDir + ".next" // the replacement list, being staged.
	prevDir = trustDir + ".prev" // the previous list, set aside during the swap.

	// Names of the steps of [opStore.replace], given to its test hook before
	// each. The new list is committed once the install rename succeeded:
	// stepInstalled is the first step after that.
	stepStage         = "stage"
	stepAside         = "aside"
	stepSyncAside     = "sync-aside"
	stepSwap          = "swap"
	stepInstalled     = "installed"
	stepSyncInstalled = "sync-install"
	stepCleanup       = "cleanup"
	stepSyncCleanup   = "sync-cleanup"
	stepRollback      = "rollback"
	stepReopen        = "reopen"
)

// errCommitted marks the error of [opStore.replace] when the replacement is
// in place anyway: only the follow-up (making it durable, removing the set
// aside list) failed. The caller must go on as if replace had succeeded, since
// the next open loads the new list; any other error means the previous list
// is kept.
var errCommitted = errors.New("replacement committed")

// errOrphanedStage is returned when the member list is gone and only an
// unusable stage is left: nothing safe to load, so the open fails rather than
// minting a new project.
var errOrphanedStage = errors.New("trust store lost: an interrupted replacement left no usable list")

// openTrust loads the member list of project p, or creates it, titled like p
// and signed by s, on the first run. It returns the list, filled with the ops
// found on disk, and the store that keeps it. The caller closes the store.
func openTrust(p *project.Project, s trust.Signer) (*trust.List, *opStore, error) {
	parent, err := p.OpenDir(".")
	if err != nil {
		return nil, nil, err
	}
	if err := recoverSwap(parent); err != nil {
		return nil, nil, errors.Join(err, parent.Close())
	}
	root, err := p.OpenDir(trustDir)
	if err != nil {
		return nil, nil, errors.Join(err, parent.Close())
	}
	st := &opStore{parent: parent, root: root}
	list, err := st.load(s, p.Meta().Title)
	if err != nil {
		return nil, nil, errors.Join(err, st.close())
	}
	return list, st, nil
}

// recoverSwap finishes or undoes a [opStore.replace] that was interrupted, so
// that trust is the one complete list. The commit point of a replacement is
// the install rename, which makes trust exist again: without trust, the swap
// never committed, and the set-aside list is the previous list, whatever the
// stage holds, even a complete one. A stage alone cannot be vouched for and
// fails closed with [errOrphanedStage]. With trust, the stage never took over
// and is dropped, as is the previous list of a swap that did.
func recoverSwap(parent *os.Root) error {
	_, err := parent.Stat(trustDir)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		if err := restoreTrust(parent); err != nil {
			return err
		}
	default:
		return fmt.Errorf("recovering the trust store: %w", err)
	}
	// What the rename made visible must be durable before the fallback goes.
	if err := store.SyncDir(parent, "."); err != nil {
		return fmt.Errorf("syncing the trust store: %w", err)
	}
	for _, d := range []string{nextDir, prevDir} {
		if err := discard(parent, d); err != nil {
			return err
		}
	}
	return nil
}

// discard removes the directory name of parent and makes the removal durable.
func discard(parent *os.Root, name string) error {
	if err := parent.RemoveAll(name); err != nil {
		return fmt.Errorf("removing %s: %w", name, err)
	}
	if err := store.SyncDir(parent, "."); err != nil {
		return fmt.Errorf("syncing the removal of %s: %w", name, err)
	}
	return nil
}

// restoreTrust renames the set-aside list to trust, see [recoverSwap].
func restoreTrust(parent *os.Root) error {
	switch {
	case exists(parent, prevDir):
		if err := parent.Rename(prevDir, trustDir); err != nil {
			return fmt.Errorf("recovering the trust store: %w", err)
		}
		return nil
	case exists(parent, nextDir):
		return errOrphanedStage
	default:
		return nil // a project that never had a member list.
	}
}

func exists(r *os.Root, name string) bool {
	_, err := r.Stat(name)
	return err == nil
}

// load builds the member list from the stored ops. It, and readAll, run in
// [openTrust] before the store is shared, so they read root without s.mu. The project is the one
// whose genesis op is stored; if there is none, load makes one, signed by
// signer and titled title, and saves it.
func (s *opStore) load(signer trust.Signer, title string) (*trust.List, error) {
	ops, err := s.readAll()
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(ops, func(o trust.Op) bool { return o.Kind == trust.OpGenesis }) {
		g, _ := trust.Genesis(signer, title, "")
		if err := s.save(g); err != nil {
			return nil, err
		}
		ops = append(ops, g)
	}
	return buildList(ops)
}

// buildList applies ops, sorted by hash, to a list of the project whose
// genesis op is among them.
func buildList(ops []trust.Op) (*trust.List, error) {
	i := slices.IndexFunc(ops, func(o trust.Op) bool { return o.Kind == trust.OpGenesis })
	if i < 0 {
		return nil, errors.New("stored trust ops have no genesis")
	}
	list := trust.New(trust.ProjectID(ops[i].Hash()))
	// Parents first: the Lamport number of an op exceeds those of its parents.
	slices.SortStableFunc(ops, func(a, b trust.Op) int { return cmp.Compare(a.Lamport, b.Lamport) })
	if _, err := list.ApplyAll(ops); err != nil {
		return nil, fmt.Errorf("applying stored trust ops: %w", err)
	}
	return list, nil
}

// readAll returns the stored ops, sorted by hash so that the genesis op that
// load picks does not depend on the file system. Temporary files left by an
// interrupted save are removed.
func (s *opStore) readAll() ([]trust.Op, error) {
	entries, err := s.entries()
	if err != nil {
		return nil, err
	}
	var ops []trust.Op
	for _, e := range entries {
		switch {
		case e.IsDir():
		case strings.HasSuffix(e.Name(), tmpExt):
			if err := s.root.Remove(e.Name()); err != nil {
				return nil, fmt.Errorf("removing %s: %w", e.Name(), err)
			}
		case strings.HasSuffix(e.Name(), opExt):
			op, err := s.read(e.Name())
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	slices.SortFunc(ops, func(a, b trust.Op) int { return cmp.Compare(a.Hash().String(), b.Hash().String()) })
	return ops, nil
}

// entries lists the trust directory.
func (s *opStore) entries() ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(s.root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("listing trust ops: %w", err)
	}
	return entries, nil
}

func (s *opStore) read(name string) (trust.Op, error) {
	f, err := s.root.Open(name)
	if err != nil {
		return trust.Op{}, fmt.Errorf("reading trust op: %w", err)
	}
	b, err := io.ReadAll(io.LimitReader(f, trust.MaxOpSize+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return trust.Op{}, fmt.Errorf("reading trust op %s: %w", name, err)
	}
	op, err := trust.Decode(b)
	if err != nil {
		return trust.Op{}, fmt.Errorf("decoding trust op %s: %w", name, err)
	}
	return op, nil
}

// replace makes ops, the member list of the project the device joins, the
// saved ones in place of those of the project it leaves. ops come parents
// first, as [trust.List.Since] returns them.
//
// The previous list is never touched until the new one is whole: the ops go
// to a sibling directory; the two directories are then
// exchanged with renames, which are atomic. An error, or a crash, at any
// point leaves one complete list, which [openTrust] settles on the next run;
// after an error the store keeps the previous list, unless the error is
// [errCommitted]: then the replacement is in place and only its follow-up failed.
func (s *opStore) replace(ops []trust.Op) error {
	// A rollback that failed leaves the set-aside list as the only complete
	// one: settle it first, or the swap below would delete it.
	if err := recoverSwap(s.parent); err != nil {
		return err
	}
	if err := s.stage(ops); err != nil {
		return errors.Join(err, discard(s.parent, nextDir))
	}
	return s.swap()
}

func (s *opStore) hook(name string) error {
	if s.step == nil {
		return nil
	}
	return s.step(name)
}

// stage writes ops to nextDir and makes them durable.
func (s *opStore) stage(ops []trust.Op) (err error) {
	if err := s.parent.RemoveAll(nextDir); err != nil {
		return fmt.Errorf("clearing %s: %w", nextDir, err)
	}
	if err := s.parent.Mkdir(nextDir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", nextDir, err)
	}
	next, err := s.parent.OpenRoot(nextDir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", nextDir, err)
	}
	defer func() { err = errors.Join(err, next.Close()) }()
	for _, op := range ops {
		if err := s.hook(stepStage); err != nil {
			return err
		}
		if err := saveIn(next, op); err != nil {
			return err
		}
	}
	return store.SyncDir(s.parent, ".")
}

// sync runs the hook for step name, then flushes the directory entries of
// .inkwell: every rename and removal is made durable before the next step.
func (s *opStore) sync(name string) error {
	if err := s.hook(name); err != nil {
		return err
	}
	return store.SyncDir(s.parent, ".")
}

// rollback puts the set-aside list back as trust. If it fails, prev stays the
// only complete list and the next [opStore.replace] or open recovers it.
func (s *opStore) rollback() error {
	if err := s.hook(stepRollback); err != nil {
		return err
	}
	return errors.Join(s.parent.Rename(prevDir, trustDir), store.SyncDir(s.parent, "."))
}

// swap puts the staged directory in place of trust, and undoes it if it
// cannot. The commit point is the install rename: before it every error rolls
// back; after it the errors are wrapped in [errCommitted].
func (s *opStore) swap() (err error) {
	// Held from the close to the reopening: a save or a read waits, and never
	// sees the closed directory.
	s.mu.Lock()
	defer s.mu.Unlock()
	// An open directory cannot be renamed on every OS.
	if err := closeOnce(s.root); err != nil {
		return fmt.Errorf("closing trust: %w", err)
	}
	s.root = nil
	asideDone, committed := false, false
	defer func() {
		if !committed {
			if asideDone {
				err = errors.Join(err, s.rollback())
			}
			if err != nil {
				err = errors.Join(err, discard(s.parent, nextDir))
			}
		}
		if rerr := s.hook(stepReopen); rerr != nil {
			err = errors.Join(err, rerr)
		} else if root, rerr := s.parent.OpenRoot(trustDir); rerr != nil {
			err = errors.Join(err, fmt.Errorf("reopening trust: %w", rerr))
		} else {
			s.root = root
		}
		// Whatever failed after the install rename, the replacement is in place.
		if committed && err != nil {
			err = fmt.Errorf("%w: %w", errCommitted, err)
		}
	}()
	if err := s.hook(stepAside); err != nil {
		return err
	}
	if err := discard(s.parent, prevDir); err != nil {
		return err
	}
	if err := s.parent.Rename(trustDir, prevDir); err != nil {
		return fmt.Errorf("setting the trust store aside: %w", err)
	}
	asideDone = true
	if err := s.sync(stepSyncAside); err != nil {
		return fmt.Errorf("syncing the trust store: %w", err)
	}
	if err := s.hook(stepSwap); err != nil {
		return err
	}
	if err := s.parent.Rename(nextDir, trustDir); err != nil {
		return fmt.Errorf("installing the new trust store: %w", err)
	}
	committed = true
	if err := s.hook(stepInstalled); err != nil {
		return err
	}
	// Durable before the fallback goes: should the removal below reach the
	// disk first, a crash would leave neither list.
	if err := s.sync(stepSyncInstalled); err != nil {
		return fmt.Errorf("syncing the trust store: %w", err)
	}
	if err := s.hook(stepCleanup); err != nil {
		return err
	}
	if err := s.parent.RemoveAll(prevDir); err != nil {
		return fmt.Errorf("removing %s: %w", prevDir, err) // the next open does.
	}
	return s.sync(stepSyncCleanup)
}

// save writes op, which is then found again by the next load, and notes when
// it was received if that was not noted already.
func (s *opStore) save(op trust.Op) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.root == nil { // closed again since ensureOpen: a replacement is under way.
		return errClosedStore
	}
	return saveIn(s.root, op)
}

// errClosedStore is returned by a save that found the store closed.
var errClosedStore = errors.New("trust store closed")

// ensureOpen reopens the trust directory if a failed replacement left it closed.
func (s *opStore) ensureOpen() error {
	s.mu.RLock()
	open := s.root != nil
	s.mu.RUnlock()
	if open {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != nil {
		return nil
	}
	root, err := s.parent.OpenRoot(trustDir)
	if err != nil {
		return fmt.Errorf("reopening trust: %w", err)
	}
	s.root = root
	return nil
}

// saveIn is [opStore.save] in the directory r.
func saveIn(r *os.Root, op trust.Op) error {
	if err := store.WriteAtomic(r, op.Hash().String()+opExt, trust.Encode(op), 0o600); err != nil {
		return fmt.Errorf("saving trust op: %w", err)
	}
	name := op.Hash().String() + rcvExt
	if _, err := r.Stat(name); err == nil {
		return nil
	}
	stamp := binary.BigEndian.AppendUint64(nil, uint64(time.Now().UnixNano())) // #nosec G115 -- the clock is after 1970.
	if err := store.WriteAtomic(r, name, stamp, 0o600); err != nil {
		return fmt.Errorf("noting the arrival of a trust op: %w", err)
	}
	return nil
}

// receivedAt returns when this device received the op with hash h: the time
// noted when it was saved, which ops do not carry (they have no clock). It is
// the zero time if the op was not saved here.
func (s *opStore) receivedAt(h trust.Hash) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.root == nil {
		return time.Time{}
	}
	b, err := s.root.ReadFile(h.String() + rcvExt)
	if err != nil || len(b) != 8 {
		return time.Time{}
	}
	return time.Unix(0, int64(binary.BigEndian.Uint64(b))) // #nosec G115 -- written from an int64.
}

func (s *opStore) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := closeOnce(s.root)
	s.root = nil
	return errors.Join(err, closeOnce(s.parent))
}

// closeOnce closes r, and is not an error when it is nil or already closed.
func closeOnce(r *os.Root) error {
	if r == nil {
		return nil
	}
	if err := r.Close(); !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}
