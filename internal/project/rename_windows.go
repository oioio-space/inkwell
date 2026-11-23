//go:build windows

package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// renameNative moves src to dst with MoveFileEx, which without
// MOVEFILE_REPLACE_EXISTING fails if dst exists. It works on paths, so it is
// as safe as the checks that precede it.
func (p *Project) renameNative(src, dst string) error { return renameChild(p.root, src, dst) }

// renameChild is [RenameNoReplace] on checked names, and the move of [Project.renameNative]:
// MoveFileEx without MOVEFILE_REPLACE_EXISTING, on the paths of src and dst in base.
func renameChild(base *os.Root, src, dst string) error {
	from, err := windows.UTF16PtrFromString(filepath.Join(base.Name(), filepath.FromSlash(src)))
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(filepath.Join(base.Name(), filepath.FromSlash(dst)))
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(from, to, 0)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
		return fs.ErrExist
	}
	return err
}
