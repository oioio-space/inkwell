//go:build unix

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotStartupCleanupPreservesActiveOwners(t *testing.T) {
	base := t.TempDir()
	old, lease, err := newSnapshotSession(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "leftover"), []byte("snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	active, activeLease, err := newSnapshotSession(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = activeLease.Close() }()
	if _, err := os.Stat(old); err != nil {
		t.Fatal("active owner removed", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	third, thirdLease, err := newSnapshotSession(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = thirdLease.Close() }()
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("stale owner contents survived startup")
	}
	for _, path := range []string{active, third} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("live owner removed", err)
		}
	}
}
