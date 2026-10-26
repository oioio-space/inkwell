package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oioio-space/inkwell/internal/app"
)

func TestOpenWithSystemHandsOverOnlyAProjectFile(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	p, err := app.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	svc := app.NewProjects(p, nil)
	if err := svc.OpenWithSystem("a.pdf"); err == nil {
		t.Error("opening without an opener succeeded")
	}
	var opened []string
	svc.SetOpener(func(path string) error {
		opened = append(opened, path)
		return nil
	})
	if err := os.WriteFile(filepath.Join(root.Name(), p.Dir(), "a.pdf"), []byte("%PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.OpenWithSystem("a.pdf"); err != nil || len(opened) != 1 || filepath.Base(opened[0]) != "a.pdf" {
		t.Errorf("OpenWithSystem = %v, opened %v", err, opened)
	}
	for _, rel := range []string{"../a.pdf", "missing.pdf", ".inkwell/project.json", "/etc/passwd"} {
		if err := svc.OpenWithSystem(rel); err == nil {
			t.Errorf("OpenWithSystem(%q) was accepted", rel)
		}
	}
	if len(opened) != 1 {
		t.Errorf("something outside the project reached the system: %v", opened)
	}
}
