package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/trust"
)

var errInjected = errors.New("injected failure")

// twoProjects returns the store of a project of a device (two ops: its genesis
// and one more member), that project's list, the member list of another
// project to replace it with, and the project folder.
func twoProjects(t *testing.T) (st *opStore, old, repl *trust.List, folder string) {
	t.Helper()
	proj, folder := newProjectFolder(t)
	dev, member, admin := newDevice(t), newDevice(t), newDevice(t)
	old, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	op, err := old.Issue(dev, trust.OpAdd, member.Public(), trust.RoleWriter, "member")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.save(op); err != nil {
		t.Fatal(err)
	}
	repl = newProject(t, admin)
	if _, err := repl.Issue(admin, trust.OpAdd, dev.Public(), trust.RoleWriter, "me"); err != nil {
		t.Fatal(err)
	}
	return st, old, repl, folder
}

// reopenCopy opens the member list of a copy of folder, as a restart after a
// crash would find the disk.
func reopenCopy(t *testing.T, folder string) *trust.List {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, filepath.Base(folder))
	if err := os.CopyFS(dir, os.DirFS(folder)); err != nil {
		t.Fatal(err)
	}
	base, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	p, err := project.Open(base, filepath.Base(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	list, st, err := openTrust(p, newDevice(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	return list
}

// replaceSteps runs a successful replace and returns the steps it went through.
func replaceSteps(t *testing.T) []string {
	t.Helper()
	st, _, repl, _ := twoProjects(t)
	t.Cleanup(func() { _ = st.close() })
	var steps []string
	st.step = func(name string) error { steps = append(steps, name); return nil }
	if err := st.replace(repl.Since(nil)); err != nil {
		t.Fatal(err)
	}
	return steps
}

func TestReplaceFailureAtAnyStepIsEitherRolledBackOrCommitted(t *testing.T) {
	steps := replaceSteps(t)
	commit := slices.Index(steps, stepInstalled)
	if commit < 0 {
		t.Fatalf("steps %v have no %s", steps, stepInstalled)
	}
	for k, name := range steps {
		committed := k >= commit
		t.Run(fmt.Sprintf("%d-%s", k, name), func(t *testing.T) {
			st, old, repl, folder := twoProjects(t)
			t.Cleanup(func() { _ = st.close() })
			n := 0
			st.step = func(string) error {
				if n++; n == k+1 {
					return errInjected
				}
				return nil
			}
			err := st.replace(repl.Since(nil))
			if !errors.Is(err, errInjected) {
				t.Fatalf("replace() = %v, want the injected failure", err)
			}
			if got := errors.Is(err, errCommitted); got != committed {
				t.Fatalf("errors.Is(replace() = %v, errCommitted) = %v, want %v", err, got, committed)
			}
			want := old
			if committed {
				want = repl // what the caller must adopt: the disk has it
			}
			if got := reopenCopy(t, folder); got.Project() != want.Project() || got.Len() != want.Len() {
				t.Errorf("after a failure at %s: project %v with %d ops, want %v with %d", name, got.Project(), got.Len(), want.Project(), want.Len())
			}
			// The store keeps working, and the replacement can be made again.
			st.step = nil
			if err := st.replace(repl.Since(nil)); err != nil {
				t.Errorf("replace() after the failure = %v", err)
			}
			if got := reopenCopy(t, folder); got.Project() != repl.Project() || got.Len() != repl.Len() {
				t.Errorf("after the retry: project %v with %d ops, want %v with %d", got.Project(), got.Len(), repl.Project(), repl.Len())
			}
		})
	}
}

func TestReplaceCrashAtAnyStepLeavesACompleteList(t *testing.T) {
	for k, name := range replaceSteps(t) {
		t.Run(fmt.Sprintf("%d-%s", k, name), func(t *testing.T) {
			st, old, repl, folder := twoProjects(t)
			t.Cleanup(func() { _ = st.close() })
			n := 0
			var snap *trust.List
			st.step = func(string) error {
				if n++; n == k+1 {
					snap = reopenCopy(t, folder) // the disk as a crash here leaves it
				}
				return nil
			}
			if err := st.replace(repl.Since(nil)); err != nil {
				t.Fatal(err)
			}
			switch {
			case snap == nil:
				t.Fatalf("step %d never ran", k)
			case snap.Project() == old.Project() && snap.Len() == old.Len():
			case snap.Project() == repl.Project() && snap.Len() == repl.Len():
			default:
				t.Errorf("crash at %s: project %v with %d ops, want the complete old (%d) or new (%d) list", name, snap.Project(), snap.Len(), old.Len(), repl.Len())
			}
		})
	}
}

func TestOpenTrustDiscardsAnIncompleteStage(t *testing.T) {
	proj, folder := newProjectFolder(t)
	dev := newDevice(t)
	list, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	junk := filepath.Join(folder, ".inkwell", "trust.next")
	if err := os.Mkdir(junk, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junk, "x.op"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	again, st2, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.close() })
	if again.Project() != list.Project() {
		t.Errorf("project = %v, want %v", again.Project(), list.Project())
	}
	if _, err := os.Stat(junk); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("trust.next after open: %v, want it removed", err)
	}
}

// interruptedSwap builds the disk of an interrupted swap: the member list of
// folder set aside as trust.prev, and a trust.next holding
// content.
func interruptedSwap(t *testing.T, content string, ops ...trust.Op) (*project.Project, string, *trust.List) {
	t.Helper()
	proj, folder := newProjectFolder(t)
	dev := newDevice(t)
	list, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	inkwell := filepath.Join(folder, ".inkwell")
	if err := os.Rename(filepath.Join(inkwell, "trust"), filepath.Join(inkwell, "trust.prev")); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(inkwell, "trust.next")
	if err := os.Mkdir(next, 0o700); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(next, "x.op"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, op := range ops {
		if err := os.WriteFile(filepath.Join(next, op.Hash().String()+".op"), trust.Encode(op), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return proj, folder, list
}

func TestOpenTrustDoesNotPromoteAnInvalidStage(t *testing.T) {
	proj, _, list := interruptedSwap(t, "garbage")
	again, st, err := openTrust(proj, newDevice(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.close() })
	if again.Project() != list.Project() {
		t.Errorf("project = %v, want the set-aside one %v", again.Project(), list.Project())
	}
}

func TestOpenTrustFailsClosedOnAnOrphanedStage(t *testing.T) {
	proj, folder, _ := interruptedSwap(t, "garbage")
	if err := os.RemoveAll(filepath.Join(folder, ".inkwell", "trust.prev")); err != nil {
		t.Fatal(err)
	}
	if _, st, err := openTrust(proj, newDevice(t)); !errors.Is(err, errOrphanedStage) {
		if err == nil {
			_ = st.close()
		}
		t.Errorf("openTrust() = %v, want errOrphanedStage", err)
	}
}

func TestOpenTrustDoesNotPromoteAStageWithMissingParents(t *testing.T) {
	admin, a, b := newDevice(t), newDevice(t), newDevice(t)
	other := newProject(t, admin)
	if _, err := other.Issue(admin, trust.OpAdd, a.Public(), trust.RoleWriter, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Issue(admin, trust.OpAdd, b.Public(), trust.RoleWriter, "b"); err != nil {
		t.Fatal(err)
	}
	all := other.Since(nil)
	if len(all) != 3 {
		t.Fatalf("Since(nil) has %d ops, want 3", len(all))
	}
	// The genesis and the last op, without the one in between: it applies
	// without an error but waits for its parent.
	proj, _, list := interruptedSwap(t, "", all[0], all[2])
	again, st, err := openTrust(proj, newDevice(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.close() })
	if again.Project() != list.Project() {
		t.Errorf("project = %v, want the set-aside one %v", again.Project(), list.Project())
	}
}

// TestReplaceIsRaceFreeWithSavesAndReads parks a save and a read at the
// critical point of the swap, trust closed and not yet reopened, and checks
// that neither sees the store half-way, nor races with the swap. The hook gives
// them time to run: without the lock they do so concurrently with the write
// of root, which the race detector reports; with it they wait for the swap.
func TestReplaceIsRaceFreeWithSavesAndReads(t *testing.T) {
	st, old, repl, _ := twoProjects(t)
	t.Cleanup(func() { _ = st.close() })
	op := old.Since(nil)[0]
	var wg sync.WaitGroup
	var saveErr error
	st.step = func(name string) error {
		if name != stepReopen {
			return nil
		}
		wg.Go(func() { saveErr = st.save(op) })
		wg.Go(func() { _ = st.receivedAt(op.Hash()) })
		time.Sleep(50 * time.Millisecond)
		return nil
	}
	if err := st.replace(repl.Since(nil)); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if saveErr != nil {
		t.Errorf("save() during the swap = %v, want it to wait for the swap", saveErr)
	}
}

// TestSaveReopensAClosedStore checks that a store whose reopening failed
// reopens once the directory is back.
func TestSaveReopensAClosedStore(t *testing.T) {
	st, old, repl, _ := twoProjects(t)
	t.Cleanup(func() { _ = st.close() })
	st.step = func(name string) error {
		if name == stepReopen {
			return errInjected
		}
		return nil
	}
	if err := st.replace(repl.Since(nil)); !errors.Is(err, errInjected) {
		t.Fatalf("replace() = %v, want the injected failure", err)
	}
	st.step = nil
	op := old.Since(nil)[0]
	if err := st.save(op); err != nil {
		t.Fatalf("save() after a failed reopen = %v", err)
	}
	if st.receivedAt(op.Hash()).IsZero() {
		t.Error("receivedAt() is zero after the save")
	}
}

// TestOpenTrustPrefersTheSetAsideListToAValidStage checks the documented rule:
// trust missing means the install rename never happened, so the previous list
// wins even when the stage is whole.
func TestOpenTrustPrefersTheSetAsideListToAValidStage(t *testing.T) {
	admin := newDevice(t)
	other := newProject(t, admin)
	proj, _, list := interruptedSwap(t, "", other.Since(nil)...)
	again, st, err := openTrust(proj, newDevice(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.close() })
	if again.Project() != list.Project() {
		t.Errorf("project = %v, want the set-aside one %v", again.Project(), list.Project())
	}
}

// TestReplaceAfterAFailedRollbackKeepsAList checks that when the rollback
// itself fails, the set-aside list, the only complete one, survives the next
// replace instead of being deleted with the stage.
func TestReplaceAfterAFailedRollbackKeepsAList(t *testing.T) {
	st, old, repl, folder := twoProjects(t)
	t.Cleanup(func() { _ = st.close() })
	st.step = func(name string) error {
		if name == stepSwap || name == stepRollback {
			return errInjected
		}
		return nil
	}
	if err := st.replace(repl.Since(nil)); !errors.Is(err, errInjected) {
		t.Fatalf("replace() = %v, want the injected failure", err)
	}
	if got := reopenCopy(t, folder); got.Project() != old.Project() {
		t.Fatalf("after the failed rollback: project %v, want the old one %v", got.Project(), old.Project())
	}
	st.step = nil
	if err := st.replace(repl.Since(nil)); err != nil {
		t.Fatalf("replace() after the failed rollback = %v", err)
	}
	if got := reopenCopy(t, folder); got.Project() != repl.Project() {
		t.Errorf("project %v, want the new one %v", got.Project(), repl.Project())
	}
}
