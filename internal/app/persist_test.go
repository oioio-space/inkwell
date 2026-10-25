package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/trust"
)

func newProjectFolder(t *testing.T) (*project.Project, string) {
	t.Helper()
	dir := t.TempDir()
	base, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	p, err := project.Create(base, "Mon livre", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, filepath.Join(dir, p.Dir())
}

func TestOpenTrustCreatesOnceAndReloads(t *testing.T) {
	proj, folder := newProjectFolder(t)
	dev, guest := newDevice(t), newDevice(t)

	list, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	op, err := list.Issue(dev, trust.OpAdd, guest.Public(), trust.RoleWriter, "guest")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.save(op); err != nil {
		t.Fatal(err)
	}
	// A save interrupted before its rename leaves a temporary file behind.
	if err := store.WriteAtomic(st.root, "junk.op.tmp", []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}

	again, st2, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.close() })
	if again.Project() != list.Project() || again.Len() != 2 || again.Role(guest.Public()) != trust.RoleWriter {
		t.Errorf("reloaded project %v with %d ops, guest %v; want %v, 2 ops, writer",
			again.Project(), again.Len(), again.Role(guest.Public()), list.Project())
	}
	if got := again.Role(dev.Public()); got != trust.RoleAdmin {
		t.Errorf("creator role = %v, want admin", got)
	}

	// The ops live in the project folder, and the interrupted save is swept.
	entries, err := os.ReadDir(filepath.Join(folder, ".inkwell", trustDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Errorf("trust directory holds %d files, want the 2 ops and when they arrived: %v", len(entries), entries)
	}
	info, err := os.Stat(filepath.Join(folder, ".inkwell", trustDir, op.Hash().String()+opExt))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("op file = %v, %v; want mode 0600", info, err)
	}
}

func TestOpenTrustRejectsForeignOps(t *testing.T) {
	proj, _ := newProjectFolder(t)
	dev, other := newDevice(t), newDevice(t)
	_, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	// An op of another project, dropped into the folder.
	foreign := newProject(t, other)
	op, err := foreign.Issue(other, trust.OpAdd, dev.Public(), trust.RoleReader, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.save(op); err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	if _, st2, err := openTrust(proj, dev); err == nil {
		_ = st2.close()
		t.Error("openTrust accepted an op of another project")
	}
}

func TestArrivalIsNotedOnceAndRenewedByJoining(t *testing.T) {
	proj, _ := newProjectFolder(t)
	dev, guest := newDevice(t), newDevice(t)
	list, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.close() })
	op, err := list.Issue(dev, trust.OpAdd, guest.Public(), trust.RoleWriter, "guest")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.save(op); err != nil {
		t.Fatal(err)
	}
	first := st.receivedAt(op.Hash())
	if first.IsZero() {
		t.Fatal("receivedAt() is zero after save")
	}
	time.Sleep(20 * time.Millisecond)
	if err := st.save(op); err != nil {
		t.Fatal(err)
	}
	if got := st.receivedAt(op.Hash()); !got.Equal(first) {
		t.Errorf("receivedAt() = %v after saving again, want the first arrival %v", got, first)
	}

	// Joining another project stores its ops as they arrive now.
	if err := st.replace(list.Since(nil)); err != nil {
		t.Fatal(err)
	}
	if got := st.receivedAt(op.Hash()); !got.After(first) {
		t.Errorf("receivedAt() = %v after joining, want after %v", got, first)
	}
	if got := st.receivedAt(trust.Hash{1}); !got.IsZero() {
		t.Errorf("receivedAt(unknown op) = %v, want zero", got)
	}
}

func TestReplaceSwitchesTheStoredProject(t *testing.T) {
	proj, _ := newProjectFolder(t)
	dev, admin := newDevice(t), newDevice(t)

	_, st, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	other := newProject(t, admin)
	if _, err := other.Issue(admin, trust.OpAdd, dev.Public(), trust.RoleWriter, "me"); err != nil {
		t.Fatal(err)
	}
	if err := st.replace(other.Since(nil)); err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}

	again, st2, err := openTrust(proj, dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.close() })
	if again.Project() != other.Project() || again.Len() != 2 {
		t.Errorf("reopened project %v with %d ops, want the replacement %v with 2", again.Project(), again.Len(), other.Project())
	}
}
