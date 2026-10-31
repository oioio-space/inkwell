package store

import (
	"os"
	"testing"
)

func TestRemoveSyncStatesSkipsLinksAndFiles(t *testing.T) {
	kv, r := openKV(t)
	mustSave(t, kv, []string{"keep", "snapshot", "chunk"}, "document")
	mustSave(t, kv, []string{"old", "sync-state", "peer"}, "obsolete")
	for _, dir := range []string{"repo/linked", "repo/file"} {
		if err := r.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Symlink("../keep", "repo/linked/sync-state"); err != nil {
		t.Fatal(err)
	}
	f, err := r.Create("repo/file/sync-state")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Symlink("keep", "repo/doc-link"); err != nil {
		t.Fatal(err)
	}
	n, err := kv.RemoveSyncStates()
	if err != nil || n != 1 {
		t.Fatalf("removed %d: %v", n, err)
	}
	for _, name := range []string{"repo/linked/sync-state", "repo/file/sync-state", "repo/doc-link", "repo/keep/snapshot/chunk~"} {
		if _, err := r.Lstat(name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Stat("repo/old/sync-state"); !os.IsNotExist(err) {
		t.Fatalf("cache remains: %v", err)
	}
}
