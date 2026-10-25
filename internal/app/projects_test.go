package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oioio-space/inkwell/internal/app"
)

func TestOpenDocsHonoursEnvironment(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs")
	t.Setenv("INKWELL_DOCS", dir)
	root, err := app.OpenDocs()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("docs folder = %v, %v; want it created 0700", info, err)
	}
}

func TestOpenProjectCreatesThenReopens(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()

	first, err := app.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	svc := app.NewProjects(first, nil)
	if got := svc.Current(); got.Title != "Sans titre" || got.RootURL != "" {
		t.Errorf("Current() of a new project = %+v", got)
	}
	if err := svc.SetRootURL("automerge:2akvofn6bxdksK3uVG6qU6m9fyEP"); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := app.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = again.Close() }()
	if got := app.NewProjects(again, nil).Current().RootURL; got != "automerge:2akvofn6bxdksK3uVG6qU6m9fyEP" {
		t.Errorf("RootURL after reopening = %q", got)
	}
	entries, _ := os.ReadDir(root.Name())
	if len(entries) != 1 {
		t.Errorf("%d project folders, want the first one reused", len(entries))
	}
}

func TestSetRootURLRejects(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	p, err := app.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	svc := app.NewProjects(p, nil)
	for _, url := range []string{"", "automerge:", "http://x", "automerge:a/b", "automerge:é", "automerge:" + string(make([]byte, 200))} {
		if err := svc.SetRootURL(url); err == nil {
			t.Errorf("SetRootURL(%q) = nil, want an error", url)
		}
	}
}
