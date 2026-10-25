package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// newBase returns a root standing for <Documents>/Inkwell.
func newBase(t *testing.T) *os.Root {
	t.Helper()
	base, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	return base
}

func create(t *testing.T, base *os.Root, title string) *Project {
	t.Helper()
	p, err := Create(base, title, "automerge:abc")
	if err != nil {
		t.Fatalf("Create(%q): %v", title, err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestCreateOpenRoundTrip(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "Mon rapport")

	if got := p.Dir(); got != "Mon rapport" {
		t.Errorf("Dir = %q, want %q", got, "Mon rapport")
	}
	meta := p.Meta()
	if meta.Title != "Mon rapport" || meta.RootURL != "automerge:abc" || len(meta.ID) < 16 {
		t.Errorf("Meta = %+v", meta)
	}
	for name, mode := range map[string]fs.FileMode{
		"Mon rapport":                       0o700,
		"Mon rapport/.inkwell":              0o700,
		"Mon rapport/.inkwell/project.json": 0o600,
		"Mon rapport/.inkwell/repo":         0o700,
		"Mon rapport/.inkwell/quarantine":   0o700,
	} {
		info, err := base.Stat(name)
		if err != nil {
			t.Errorf("Stat(%s): %v", name, err)
			continue
		}
		if info.Mode().Perm() != mode {
			t.Errorf("%s mode = %o, want %o", name, info.Mode().Perm(), mode)
		}
	}

	if err := p.Save([]string{"doc", "k"}, []byte("v")); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	q, err := Open(base, "Mon rapport")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if q.Meta() != meta {
		t.Errorf("reopened Meta = %+v, want %+v", q.Meta(), meta)
	}
	if got, err := q.Load([]string{"doc", "k"}); err != nil || string(got) != "v" {
		t.Errorf("stored value after reopen = %q, %v", got, err)
	}
}

func TestCreateFolderNames(t *testing.T) {
	base := newBase(t)
	for _, tc := range []struct{ title, dir string }{
		{"Rapport", "Rapport"},
		{"Rapport", "Rapport (2)"},
		{"Rapport", "Rapport (3)"},
		{`a/b\c:d*e?"f<g>h|i`, "a-b-c-d-e--f-g-h-i"},
		{"", "Sans titre"},
		{"  ..  ", "Sans titre (2)"},
		{"../../etc", "-..-etc"},
		{"Été\x00\n", "Été-"},
		{strings.Repeat("é", 200), strings.Repeat("é", maxFolderRunes)},
	} {
		p := create(t, base, tc.title)
		if p.Dir() != tc.dir {
			t.Errorf("Create(%q).Dir = %q, want %q", tc.title, p.Dir(), tc.dir)
		}
	}
	entries, err := fs.ReadDir(base.FS(), ".")
	if err != nil || len(entries) != 9 {
		t.Errorf("%d folders under the base, want 9 (%v)", len(entries), err)
	}
}

func TestCreateKeepsTitle(t *testing.T) {
	base := newBase(t)
	for title, want := range map[string]string{"  Hello / world  ": "Hello / world", "": DefaultTitle, "  ": DefaultTitle} {
		if got := create(t, base, title).Meta().Title; got != want {
			t.Errorf("Create(%q).Meta.Title = %q, want %q", title, got, want)
		}
	}
}

func TestOpenRejects(t *testing.T) {
	base := newBase(t)
	if _, err := Open(base, "missing"); err == nil {
		t.Error("Open(missing) succeeded")
	}
	if err := base.Mkdir("plain", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(base, "plain"); err == nil {
		t.Error("Open(a folder that is not a project) succeeded")
	}
	if _, err := Open(base, "../plain"); err == nil {
		t.Error("Open(../plain) escaped the base")
	}
	p := create(t, base, "p")
	if err := p.inkwell.WriteFile("project.json", []byte(`{"id":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(base, "p"); err == nil {
		t.Error("Open(project without id) succeeded")
	}
	if err := p.inkwell.WriteFile("project.json", []byte(`not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(base, "p"); err == nil {
		t.Error("Open(garbage project.json) succeeded")
	}
}

func TestLatest(t *testing.T) {
	base := newBase(t)
	if _, err := Latest(base); !errors.Is(err, ErrNoProject) {
		t.Fatalf("Latest(empty) = %v, want ErrNoProject", err)
	}
	if err := base.Mkdir("not a project", 0o700); err != nil {
		t.Fatal(err)
	}
	old := create(t, base, "old")
	fresh := create(t, base, "fresh")
	past := time.Now().Add(-time.Hour)
	if err := base.Chtimes("old/.inkwell/project.json", past, past); err != nil {
		t.Fatal(err)
	}
	if got, err := Latest(base); err != nil || got != fresh.Dir() {
		t.Errorf("Latest = %q, %v; want %q", got, err, fresh.Dir())
	}
	// Changing a project makes it the latest.
	recent := time.Now().Add(-30 * time.Minute)
	if err := base.Chtimes("fresh/.inkwell/project.json", recent, recent); err != nil {
		t.Fatal(err)
	}
	if err := old.SetRootURL("automerge:new"); err != nil {
		t.Fatal(err)
	}
	if got, err := Latest(base); err != nil || got != old.Dir() {
		t.Errorf("Latest after SetRootURL = %q, %v; want %q", got, err, old.Dir())
	}
}

func TestSetRootURL(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.SetRootURL("automerge:other"); err != nil {
		t.Fatal(err)
	}
	q, err := Open(base, "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if got := q.Meta().RootURL; got != "automerge:other" {
		t.Errorf("RootURL after reopen = %q", got)
	}
}

func TestWriteProjection(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")

	for _, text := range []string{"= Titre\n", "= Titre\nSuite\n", ""} {
		if err := p.WriteProjection(MainFile, []byte(text)); err != nil {
			t.Fatalf("WriteProjection(%q): %v", text, err)
		}
		got, err := p.ReadProjection(MainFile)
		if err != nil || string(got) != text {
			t.Fatalf("ReadProjection = %q, %v; want %q", got, err, text)
		}
	}
	onDisk, err := base.ReadFile("p/main.typ")
	if err != nil || len(onDisk) != 0 {
		t.Errorf("main.typ on disk = %q, %v", onDisk, err)
	}
	if _, err := p.ReadProjection("other.typ"); !errors.Is(err, ErrNotOwned) {
		t.Errorf("ReadProjection(not owned) = %v, want ErrNotOwned", err)
	}
}

func TestWriteProjectionNotOwned(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	for _, rel := range []string{"other.typ", "../x", "sub/main.typ", "", ".inkwell/project.json", "/etc/passwd"} {
		if err := p.WriteProjection(rel, []byte("x")); !errors.Is(err, ErrNotOwned) {
			t.Errorf("WriteProjection(%q) = %v, want ErrNotOwned", rel, err)
		}
	}
	if _, err := base.Stat("p/other.typ"); err == nil {
		t.Error("a file that is not owned was created")
	}
}

// TestProjectionNeverClobbersUnseenEdit is the ownership rule: Inkwell does
// not overwrite a change it was not told about, until it is acknowledged.
func TestProjectionNeverClobbersUnseenEdit(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("mine")); err != nil {
		t.Fatal(err)
	}
	if err := base.WriteFile("p/main.typ", []byte("theirs"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection(MainFile, []byte("mine 2")); !errors.Is(err, ErrExternalEdit) {
		t.Fatalf("WriteProjection over an unseen edit = %v, want ErrExternalEdit", err)
	}
	if got, _ := base.ReadFile("p/main.typ"); string(got) != "theirs" {
		t.Errorf("main.typ = %q: the external edit was lost", got)
	}
	if err := p.Acknowledge(MainFile); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection(MainFile, []byte("mine 3")); err != nil {
		t.Fatalf("WriteProjection after Acknowledge: %v", err)
	}
	if got, _ := base.ReadFile("p/main.typ"); string(got) != "mine 3" {
		t.Errorf("main.typ = %q after acknowledged edit", got)
	}
	if err := p.Acknowledge("nope"); !errors.Is(err, ErrNotOwned) {
		t.Errorf("Acknowledge(not owned) = %v, want ErrNotOwned", err)
	}
}

// TestRecordedHashSurvivesRestart: an edit made while Inkwell was closed is
// still an external edit when the project is reopened.
func TestRecordedHashSurvivesRestart(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("mine")); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := base.WriteFile("p/main.typ", []byte("edited offline"), 0o600); err != nil {
		t.Fatal(err)
	}
	q, err := Open(base, "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if err := q.WriteProjection(MainFile, []byte("mine")); !errors.Is(err, ErrExternalEdit) {
		t.Errorf("WriteProjection after offline edit = %v, want ErrExternalEdit", err)
	}
}

func TestProjectionSymlinkEscape(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Symlink(outside, "p/main.typ"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := p.ReadProjection(MainFile); err == nil {
		t.Error("ReadProjection followed a link out of the project")
	}
	_ = p.WriteProjection(MainFile, []byte("pwned"))
	if got, _ := os.ReadFile(outside); string(got) != "secret" {
		t.Errorf("file outside the project was modified: %q", got)
	}
}

func TestOpenDir(t *testing.T) {
	p := create(t, newBase(t), "Mon rapport")
	r, err := p.OpenDir("trust")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	info, err := os.Stat(r.Name())
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("OpenDir(trust) = %v, %v; want a private directory", info, err)
	}
	if want := filepath.Join(".inkwell", "trust"); !strings.HasSuffix(r.Name(), want) {
		t.Errorf("OpenDir(trust) opened %s, want it below %s", r.Name(), want)
	}
	again, err := p.OpenDir("trust") // already there
	if err != nil {
		t.Fatalf("second OpenDir: %v", err)
	}
	_ = again.Close()
	if _, err := p.OpenDir("../escape"); err == nil {
		t.Error("OpenDir(../escape) = nil error, want a refusal")
	}
}
