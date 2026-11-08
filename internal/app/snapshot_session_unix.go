//go:build unix

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Each process keeps an exclusive lease until exit. Other processes can remove
// only unleased sessions, including their immutable content and abandoned roots.
func newSnapshotSession(base string) (string, *os.File, error) {
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", nil, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := filepath.Join(base, entry.Name())
		if strings.HasPrefix(entry.Name(), "process-") {
			cleanupSnapshotSession(name, entry)
		}
		// Legacy versions had no leases but compiled with a one-minute deadline.
		if strings.HasPrefix(entry.Name(), "compile-") {
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
				_ = os.RemoveAll(name)
			}
		}
	}
	root, err := os.MkdirTemp(base, "process-")
	if err != nil {
		return "", nil, err
	}
	lease, err := openLease(root, os.O_CREATE|os.O_EXCL|os.O_RDWR)
	if err != nil {
		_ = os.RemoveAll(root)
		return "", nil, err
	}
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lease.Close()
		_ = os.RemoveAll(root)
		return "", nil, err
	}
	return root, lease, nil
}

func cleanupSnapshotSession(root string, entry os.DirEntry) {
	lease, err := openLease(root, os.O_RDWR)
	if err != nil {
		// A crash between mkdir and lease creation leaves an unowned directory.
		if errors.Is(err, os.ErrNotExist) {
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
				_ = os.RemoveAll(root)
			}
		}
		return
	}
	defer func() { _ = lease.Close() }()
	if syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
		_ = os.RemoveAll(root)
	}
}

// openLease opens the session's owner.lock through an os.Root, so the open
// cannot leave the session directory (no symlink or ".." escape).
func openLease(root string, flag int) (*os.File, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	return dir.OpenFile("owner.lock", flag, 0o600)
}
