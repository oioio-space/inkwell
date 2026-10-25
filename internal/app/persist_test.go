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
