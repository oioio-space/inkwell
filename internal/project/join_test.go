package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests of what a join by restart needs from a project folder (plan
// docs/plans/2026-11-15-pc1b-join.md, rules J-1 and J-6).

func TestStagingHoldsThePendingJoinUntilCompleted(t *testing.T) {
	dir := t.TempDir()
	base, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	const id = "000102030405060708090a0b0c0d0e0f"
	p, err := CreateStaging(base, id, "Roman")
	if err != nil {
		t.Fatal(err)
	}
	if p.Dir() != StagingPrefix+id {
		t.Errorf("CreateStaging folder = %q, want %q", p.Dir(), StagingPrefix+id)
	}
	if _, err := CreateStaging(base, id, "Roman"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("J-6 exclusive create: a second CreateStaging of the same id = %v, want it refused as existing", err)
	}
	if _, ok := p.PendingJoin(); ok {
		t.Error("a new staging project already holds a record")
	}
	rec := PendingJoin{V: 1, ID: id, Project: "p1", Inviter: "peer", Addrs: []string{"/ip4/127.0.0.1/tcp/1"}, Root: "", Previous: "Mon livre", Folder: "Roman"}
	if err := p.SetPendingJoin(rec); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, StagingPrefix+id, inkwellDir, manifestFile))
	if err != nil || !strings.Contains(string(raw), `"pendingJoin"`) {
		t.Fatalf("J-1 the record is the field pendingJoin of project.json: %v\n%s", err, raw)
	}

	p, err = Open(base, StagingPrefix+id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	got, ok := p.PendingJoin()
	if !ok || got.ID != rec.ID || got.Project != rec.Project || got.Inviter != rec.Inviter || len(got.Addrs) != 1 || got.Previous != rec.Previous || got.Folder != rec.Folder {
		t.Fatalf("PendingJoin after a reopen = %+v, %v; want %+v", got, ok, rec)
	}
	if err := p.SetRootURL("automerge:fresh"); !errors.Is(err, ErrPendingJoin) {
		t.Errorf("J-1 no root is recorded before the consumption: SetRootURL with a record = %v, want refused", err)
	}
	if p.Meta().RootURL != "" {
		t.Errorf("J-1 no root is recorded before the consumption: rootUrl = %q", p.Meta().RootURL)
	}
	if err := p.CompleteJoin("automerge:joined"); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.PendingJoin(); ok || p.Meta().RootURL != "automerge:joined" {
		t.Errorf("J-1 consumed once: after CompleteJoin the record is kept (%v) or the root is %q", ok, p.Meta().RootURL)
	}
	raw, err = os.ReadFile(filepath.Join(dir, StagingPrefix+id, inkwellDir, manifestFile))
	if err != nil || strings.Contains(string(raw), "pendingJoin") {
		t.Errorf("J-1 consumed once: project.json still holds the record: %v\n%s", err, raw)
	}
}

func TestLatestIgnoresAStagingFolder(t *testing.T) {
	base, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	own, err := Create(base, "Mon livre", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = own.Close() }()
	staged, err := CreateStaging(base, "ffffffffffffffffffffffffffffffff", "Roman") // the newest
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = staged.Close() }()
	if got, err := Latest(base); err != nil || got != own.Dir() {
		t.Errorf("J-4 a staging folder is never opened: Latest = %q, %v; want %q", got, err, own.Dir())
	}
}

func TestRenameNoReplaceNeverReplaces(t *testing.T) {
	dir := t.TempDir()
	base, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	mk := func(name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mk("src/inside")
	mk("folder")
	mk("elsewhere")
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	for _, taken := range []string{"folder", "file", "link"} {
		if err := RenameNoReplace(base, "src", taken); !errors.Is(err, fs.ErrExist) {
			t.Errorf("J-6 never replaces: RenameNoReplace onto the existing %s = %v, want fs.ErrExist", taken, err)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "elsewhere")); len(entries) != 0 {
		t.Errorf("J-6 never replaces: the symbolic link was followed: %v", entries)
	}
	for _, bad := range []string{"../out", "a/b", `a\b`, ".", "..", ""} {
		if err := RenameNoReplace(base, "src", bad); err == nil || errors.Is(err, fs.ErrExist) {
			t.Errorf("J-6 a new direct child only: RenameNoReplace to %q = %v, want refused", bad, err)
		}
	}
	if err := RenameNoReplace(base, "src", "free"); err != nil {
		t.Fatalf("RenameNoReplace to a free name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "free", "inside")); err != nil {
		t.Errorf("the folder did not move whole: %v", err)
	}
}

// TestPreferMakesAProjectTheLatest: a return to the previous document survives a relaunch (plan
// docs/plans/2026-11-15-pc1b-join.md, rule J-2): the folder the person returned to is the one
// [Latest] names afterwards, whatever the clock said when the other folders were written.
func TestPreferMakesAProjectTheLatest(t *testing.T) {
	dir := t.TempDir()
	base, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	var names []string
	for _, title := range []string{"Mon livre", "Roman"} {
		p, err := Create(base, title, "")
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, p.Dir())
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// The joined folder was written by a clock ahead of this one.
	ahead := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, names[1], inkwellDir, manifestFile), ahead, ahead); err != nil {
		t.Fatal(err)
	}
	if latest, err := Latest(base); err != nil || latest != names[1] {
		t.Fatalf("Latest before the return = %q, %v; want %q", latest, err, names[1])
	}
	if !IsProject(base, names[0]) || IsProject(base, "absent") || IsProject(base, "../"+filepath.Base(dir)+"/"+names[0]) {
		t.Errorf("IsProject: a project %v, a missing folder %v, a path out of the base %v; want true, false, false",
			IsProject(base, names[0]), IsProject(base, "absent"), IsProject(base, "../"+filepath.Base(dir)+"/"+names[0]))
	}
	if err := Prefer(base, names[0]); err != nil {
		t.Fatal(err)
	}
	if latest, err := Latest(base); err != nil || latest != names[0] {
		t.Errorf("J-2 a return to the previous document survives a relaunch: Latest = %q, %v; want %q", latest, err, names[0])
	}
	if err := Prefer(base, "absent"); !errors.Is(err, ErrNoProject) {
		t.Errorf("Prefer of a folder that is no project = %v, want ErrNoProject", err)
	}
}
