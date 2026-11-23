package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/trust"
)

// The take-over of a document whose member list does not name this device (plan
// docs/plans/2026-11-15-pc1a-take-over.md, rules N-5..N-8; ADR 0043, part 2): the request only
// signs a genesis in memory; the old list is archived and replaced by it in [Runtime.Close], and
// the next process image loads it.

const (
	// archiveDir, in .inkwell, keeps a copy of each list a take-over replaced, in a folder named
	// after the UTC time of the take-over: never deleted, never rotated.
	archiveDir = trustDir + "-archive"
	// partialExt marks an archive being written: it is renamed once whole.
	partialExt = ".partial"
	// archiveTime names an archive; it sorts by time and has no character a file system refuses.
	archiveTime = "20060102T150405.000000000Z"

	// Names of the steps of [opStore.archive], given to the test hook before each, as those of
	// [opStore.replace] are.
	stepArchive        = "archive"         // before each file copied
	stepArchiveInstall = "archive-install" // before the rename of the whole copy
)

// TakeOverResult is the answer of [Session.TakeOver].
type TakeOverResult struct {
	// Restarting is set when the take-over was recorded: Inkwell quits and restarts on the same
	// folder, where this device is then the first admin.
	Restarting bool `json:"restarting"`
}

// TakeOver asks for the document to become this device's: when the member list does not name it
// as a member, a genesis signed by this device is checked in memory, the bridge is drained, and the
// application is asked to quit, then to restart on the same folder; the answer is then
// Restarting. Nothing is written here and the running session keeps its project: the list is
// archived and replaced while the runtime closes (rule N-5), and only the next image loads it
// (rule N-11). For a member it answers not Restarting and does nothing (rule N-8). Once a change
// was requested, and where Inkwell cannot restart ([Session.CanTakeOver]), it is refused.
func (s *Session) TakeOver(ctx context.Context) (TakeOverResult, error) {
	if err := s.leaving(); err != nil {
		return TakeOverResult{}, err
	}
	if s.Standing() == StandingMember {
		return TakeOverResult{}, nil
	}
	if !s.CanTakeOver() {
		return TakeOverResult{}, errNoRestart
	}
	write, err := s.inv.takeOver(s.proj.Meta())
	if err != nil {
		return TakeOverResult{}, err
	}
	if err := s.requestChange(ctx, ChangeTakeOver, write); err != nil {
		return TakeOverResult{}, err
	}
	return TakeOverResult{Restarting: true}, nil
}

// takeOverGenesis returns the genesis of the project a take-over of the project of m by dev
// makes: signed by dev, titled like the project, with its recorded root document URL when valid,
// so that its id differs from the one of a project dev created with the same title (rule N-8). It
// is checked as a peer would check it.
func takeOverGenesis(dev trust.Signer, m project.Meta) (trust.Op, error) {
	root := m.RootURL
	if !validRootURL(root) {
		root = ""
	}
	g, id := trust.Genesis(dev, m.Title, root)
	if err := trust.New(id).Apply(g); err != nil {
		return trust.Op{}, fmt.Errorf("take-over: %w", err)
	}
	return g, nil
}

// finishTakeOver writes a take-over on st: it archives the stored list by copy, then replaces it
// with the genesis g. An error leaves the old list, which the next image loads; a failure after
// the replacement took place ([errCommitted]) is only logged, since the next image loads the new
// list (rule N-12).
func finishTakeOver(st *opStore, g trust.Op, at time.Time, log *slog.Logger) error {
	if err := st.archive(at); err != nil {
		return fmt.Errorf("take-over: archiving the member list: %w", err)
	}
	err := st.replace([]trust.Op{g})
	if errors.Is(err, errCommitted) {
		log.Warn("member list taken over, with a follow-up failure", "err", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("take-over: %w", err)
	}
	return nil
}

// archive copies the stored list to .inkwell/trust-archive/<UTC time of at>/: written as a
// sibling with [partialExt], renamed once whole and durable (rule N-7). The list itself is never
// touched.
func (s *opStore) archive(at time.Time) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	name := path.Join(archiveDir, at.UTC().Format(archiveTime))
	partial := name + partialExt
	if err := s.parent.MkdirAll(partial, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", partial, err)
	}
	dst, err := s.parent.OpenRoot(partial)
	if err != nil {
		return fmt.Errorf("opening %s: %w", partial, err)
	}
	s.mu.RLock()
	if err = errClosedStore; s.root != nil { // closed again since ensureOpen, as in [opStore.save]
		err = s.copyOps(dst)
	}
	s.mu.RUnlock()
	if err = errors.Join(err, dst.Close()); err != nil {
		return err
	}
	if err := errors.Join(store.SyncDir(s.parent, partial), s.hook(stepArchiveInstall)); err != nil {
		return err
	}
	if err := s.parent.Rename(partial, name); err != nil {
		return fmt.Errorf("installing the archive: %w", err)
	}
	return store.SyncDir(s.parent, archiveDir)
}

// copyOps copies the files of the list (ops and their arrival notes) to dst; the caller holds
// s.mu for reading.
func (s *opStore) copyOps(dst *os.Root) error {
	entries, err := s.entries()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasSuffix(e.Name(), tmpExt) {
			continue
		}
		if err := s.hook(stepArchive); err != nil {
			return err
		}
		b, err := s.root.ReadFile(e.Name())
		if err != nil {
			return fmt.Errorf("archiving %s: %w", e.Name(), err)
		}
		if err := store.WriteAtomic(dst, e.Name(), b, 0o600); err != nil {
			return fmt.Errorf("archiving %s: %w", e.Name(), err)
		}
	}
	return nil
}
