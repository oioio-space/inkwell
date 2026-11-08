package project

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImmutableProjectionSnapshot(t *testing.T) {
	p := create(t, newBase(t), "Snapshot")
	raw := []byte("#let x = 1\r\n")
	if err := p.root.WriteFile("main.typ", raw, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{".typstdeps/preview/local/1.0.0", "assets"} {
		if err := p.root.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string][]byte{".typstdeps/preview/local/1.0.0/lib.typ": []byte("#let y = 2"), "assets/raw.bin": {0, 1, 2, 255}} {
		if err := p.root.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	target := t.TempDir()
	snap, err := p.SnapshotTo(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(target, "main.typ"))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("raw snapshot %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(target, ".inkwell")); !os.IsNotExist(err) {
		t.Fatal("protected internals copied")
	}
	if _, err := os.Stat(filepath.Join(target, ".typstdeps/preview/local/1.0.0/lib.typ")); err != nil {
		t.Fatal(err)
	}
	if !p.MatchesSnapshot(context.Background(), snap) {
		t.Fatal("fresh snapshot differs")
	}
	if err := p.root.WriteFile("main.typ", []byte("#let different = 2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p.MatchesSnapshot(context.Background(), snap) {
		t.Fatal("changed dependency matched")
	}
	if got, _ := os.ReadFile(filepath.Join(target, "main.typ")); !bytes.Equal(got, raw) {
		t.Fatal("snapshot changed with live sources")
	}
}

func TestSnapshotRefusesLinksAndBounds(t *testing.T) {
	p := create(t, newBase(t), "Snapshot")
	if err := p.root.WriteFile("main.typ", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.root.Symlink("main.typ", "alias.typ"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SnapshotTo(t.Context(), t.TempDir()); err == nil {
		t.Fatal("accepted symbolic link")
	}
	if err := p.root.Remove("alias.typ"); err != nil {
		t.Fatal(err)
	}
	if err := p.root.Link("main.typ", "alias.typ"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SnapshotTo(t.Context(), t.TempDir()); err == nil {
		t.Fatal("accepted hardlink")
	}
	if err := p.root.Remove("alias.typ"); err != nil {
		t.Fatal(err)
	}
	large, err := p.root.OpenFile("large.bin", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(snapshotFileLimit + 1); err != nil {
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SnapshotTo(t.Context(), t.TempDir()); err == nil {
		t.Fatal("accepted oversized file")
	}
}

func TestSnapshotsReuseImmutableContentAndNoticePreservedMtimeEdits(t *testing.T) {
	p := create(t, newBase(t), "Snapshot")
	if err := p.root.WriteFile("main.typ", []byte("#let a = 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, second := t.TempDir(), t.TempDir()
	if _, err := p.SnapshotTo(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SnapshotTo(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	a, _ := os.Stat(filepath.Join(first, "main.typ"))
	b, _ := os.Stat(filepath.Join(second, "main.typ"))
	if !os.SameFile(a, b) {
		t.Fatal("unchanged immutable content copied again")
	}
	original, err := p.root.Stat("main.typ")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.root.WriteFile("main.typ", []byte("#let b = 2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(p.Path(), "main.typ"), original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	third := t.TempDir()
	if _, err := p.SnapshotTo(t.Context(), third); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(third, "main.typ"))
	if err != nil || string(copied) != "#let b = 2" {
		t.Fatalf("preserved mtime concealed changed bytes: %q %v", copied, err)
	}
	if stringMustRead(t, filepath.Join(first, "main.typ")) != "#let a = 1" {
		t.Fatal("reused snapshot was writable through live file")
	}
}

func stringMustRead(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSnapshotBulkIODoesNotTakeProjectWriteMutex(t *testing.T) {
	p := create(t, newBase(t), "Snapshot")
	p.mu.Lock()
	done := make(chan error, 1)
	go func() { _, err := p.SnapshotTo(t.Context(), t.TempDir()); done <- err }()
	select {
	case err := <-done:
		p.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		p.mu.Unlock()
		if err := <-done; err != nil {
			t.Logf("snapshot after releasing write mutex: %v", err)
		}
		t.Fatal("snapshot I/O waited for project write mutex")
	}
}

func TestFailedSnapshotsBoundObjectsAndKeepActiveInputs(t *testing.T) {
	p := create(t, newBase(t), "Snapshot")
	original := []byte("#let original = 1")
	if err := p.root.WriteFile("main.typ", original, 0o600); err != nil {
		t.Fatal(err)
	}
	active := t.TempDir()
	if _, err := p.SnapshotTo(t.Context(), active); err != nil {
		t.Fatal(err)
	}
	late, err := p.root.OpenFile("zz-oversized.bin", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := late.Truncate(snapshotFileLimit + 1); err != nil {
		t.Fatal(err)
	}
	if err := late.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := p.root.WriteFile("main.typ", fmt.Appendf(nil, "#let changed = %d", i), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := p.SnapshotTo(t.Context(), t.TempDir()); err == nil {
			t.Fatal("oversized dependency accepted")
		}
		objects, err := os.ReadDir(p.snapshotObjects)
		if err != nil {
			t.Fatal(err)
		}
		if len(objects) > 1 {
			t.Fatalf("failed snapshot retained %d obsolete objects after edit %d", len(objects), i)
		}
		if len(p.snapshotEntries) > 1 {
			t.Fatalf("failed snapshot retained %d obsolete cache entries", len(p.snapshotEntries))
		}
	}
	if got := stringMustRead(t, filepath.Join(active, "main.typ")); got != string(original) {
		t.Fatalf("active immutable input changed: %q", got)
	}
}
