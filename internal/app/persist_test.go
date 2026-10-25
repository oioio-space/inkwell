package app

import (
	"os"
	"testing"

	"github.com/oioio-space/inkwell/internal/trust"
)

func TestOpenProjectCreatesOnceAndReloads(t *testing.T) {
	home, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	dev, guest := newDevice(t), newDevice(t)

	list, st, err := openProject(home, dev, "Mon livre")
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
	if err := writeFileAtomic(st.root, "junk.op.tmp", []byte("partial")); err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}

	again, st2, err := openProject(home, dev, "autre titre")
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
	entries, err := os.ReadDir(home.Name() + "/" + projectsDir)
	if err != nil || len(entries) != 1 {
		t.Errorf("projects = %v, %v; want exactly one", entries, err)
	}
}

func TestCurrentProjectFollowsAdoption(t *testing.T) {
	home, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	dev, admin := newDevice(t), newDevice(t)

	own, st, err := openProject(home, dev, "mine")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}
	other := newProject(t, admin)
	if _, err := other.Issue(admin, trust.OpAdd, dev.Public(), trust.RoleWriter, "me"); err != nil {
		t.Fatal(err)
	}
	st, err = adoptProject(home, other.Project(), other.Since(nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.close(); err != nil {
		t.Fatal(err)
	}

	again, st2, err := openProject(home, dev, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if again.Project() != other.Project() || again.Len() != 2 {
		t.Errorf("reopened project %v with %d ops, want the adopted %v with 2", again.Project(), again.Len(), other.Project())
	}
	if err := st2.close(); err != nil {
		t.Fatal(err)
	}

	// A current file that names nothing falls back to the first project by name.
	if err := writeFileAtomic(home, currentFile, []byte("zz")); err != nil {
		t.Fatal(err)
	}
	want := own.Project()
	if other.Project().String() < want.String() {
		want = other.Project()
	}
	fallback, st3, err := openProject(home, dev, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st3.close() })
	if fallback.Project() != want {
		t.Errorf("fallback project = %v, want %v, the first by name", fallback.Project(), want)
	}
}
