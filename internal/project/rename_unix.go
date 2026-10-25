//go:build linux || darwin

package project

import (
	"errors"
	"io/fs"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

// renameNative moves src to dst, both relative to the project folder, in one
// system call that fails if dst exists: renameat2 with RENAME_NOREPLACE on
// Linux, renameatx_np with RENAME_EXCL on macOS. The two folders are opened
// through the project root, which keeps them inside it. It returns
// [errNoNative] where the file system cannot do it.
func (p *Project) renameNative(src, dst string) error {
	sd, err := p.openDir(path.Dir(src))
	if err != nil {
		return err
	}
	defer func() { _ = sd.Close() }() // read-only handle
	dd, err := p.openDir(path.Dir(dst))
	if err != nil {
		return err
	}
	defer func() { _ = dd.Close() }()                                           // read-only handle
	err = noReplace(int(sd.Fd()), path.Base(src), int(dd.Fd()), path.Base(dst)) // #nosec G115 -- a file descriptor fits an int.
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EEXIST):
		return fs.ErrExist
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOTSUP):
		return errNoNative
	}
	return err
}

// openDir opens the folder dir of the project for use as a file descriptor.
func (p *Project) openDir(dir string) (*os.File, error) {
	f, err := p.root.Open(dir)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.IsDir() {
		_ = f.Close()
		return nil, errors.Join(err, errNoNative)
	}
	return f, nil
}
