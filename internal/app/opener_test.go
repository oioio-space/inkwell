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
	}, func(string) error { return nil })
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

// A file from another person is never run: only types made to be viewed go to the system's
// program for them; everything else is shown in the file manager (security audit, HIGH).
func TestOpenWithSystemRunsNothingExecutable(t *testing.T) {
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
	var opened, revealed []string
	svc.SetOpener(
		func(path string) error { opened = append(opened, filepath.Base(path)); return nil },
		func(path string) error { revealed = append(revealed, filepath.Base(path)); return nil },
	)
	viewable := []string{"a.pdf", "B.PNG", "c.jpg", "d.woff2", "e.docx", "f.xlsx", "g.odt", "dir.v1.pdf"}
	risky := []string{"a.exe", "b.bat", "c.lnk", "d.hta", "e.scr", "f.msi", "g.jar", "h.docm", "i.svg", "j.sh", "k.desktop", "noext", "l.pdf.exe", "m.cmd", "n.ps1", "o.js", "p.url"}
	for _, name := range append(append([]string{}, viewable...), risky...) {
		if err := os.WriteFile(filepath.Join(root.Name(), p.Dir(), name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := svc.OpenWithSystem(name); err != nil {
			t.Fatalf("OpenWithSystem(%q): %v", name, err)
		}
	}
	if len(opened) != len(viewable) || len(revealed) != len(risky) {
		t.Fatalf("opened %v, revealed %v", opened, revealed)
	}
	for i, name := range viewable {
		if opened[i] != name {
			t.Errorf("%q was not opened (opened %v)", name, opened)
		}
	}
	for i, name := range risky {
		if revealed[i] != name {
			t.Errorf("%q was not only revealed (revealed %v)", name, revealed)
		}
	}
}
