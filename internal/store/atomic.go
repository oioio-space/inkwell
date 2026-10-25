package store

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"runtime"
)

// WriteAtomic replaces the file name under r with data so that a crash at any
// point leaves either the previous content or the new one, never a mix.
//
// It writes a uniquely named temporary file next to name (O_EXCL), syncs and
// closes it, renames it over name, then syncs the directory so the rename
// itself survives power loss. name uses forward slashes and its directory must
// exist. A temporary file left by a killed process is harmless; [Open]
// removes it.
func WriteAtomic(r *os.Root, name string, data []byte, perm fs.FileMode) error {
	return writeAtomic(r, name, bytes.NewReader(data), perm, nil, nil)
}

// WriteAtomicFrom is [WriteAtomic] for content that is too big to hold in
// memory: it streams src into the temporary file. check, if not nil, runs once
// src is drained and before the file is synced and renamed; an error from it
// removes the temporary file and is returned, so that content can be verified
// as it goes by.
func WriteAtomicFrom(r *os.Root, name string, src io.Reader, perm fs.FileMode, check func() error) error {
	return writeAtomic(r, name, src, perm, check, nil)
}

// writeAtomic is [WriteAtomic] with a hook run after the temporary file is
// durable and before the Rename; tests use it to stop a process there.
func writeAtomic(r *os.Root, name string, src io.Reader, perm fs.FileMode, check func() error, beforeRename func()) (err error) {
	tmp := name + "." + rand.Text() + tmpSuffix
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("store: create %s: %w", tmp, err)
	}
	defer func() {
		if err != nil {
			_ = r.Remove(tmp) // best effort: the write already failed
		}
	}()
	if _, err = io.Copy(f, src); err == nil && check != nil {
		err = check()
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("store: write %s: %w", tmp, err)
	}
	if beforeRename != nil {
		beforeRename()
	}
	if err = r.Rename(tmp, name); err != nil {
		return fmt.Errorf("store: rename %s: %w", tmp, err)
	}
	if err = syncDir(r, path.Dir(name)); err != nil {
		return fmt.Errorf("store: sync directory of %s: %w", name, err)
	}
	return nil
}

// syncDir flushes directory dir so a rename in it is durable. Windows cannot
// open a directory for syncing; NTFS journals metadata, so it is skipped there.
func syncDir(r *os.Root, dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := r.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	return err
}
