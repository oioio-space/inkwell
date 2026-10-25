package project

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

var (
	// ErrLink is returned for a path that goes through a symbolic link: Inkwell
	// reads and writes only real files and folders of the project.
	ErrLink = errors.New("project: path goes through a symbolic link")
	// ErrHardLink is returned for a file that has another name elsewhere: what
	// is read or written there would be read or written under that name too.
	ErrHardLink = errors.New("project: file has several hard links")
	// ErrNotRegular is returned for a path that is not a regular file.
	ErrNotRegular = errors.New("project: not a regular file")
)

// lstatPath walks rel from the project folder, one component at a time, and
// returns the Lstat of the last one. It returns [ErrLink] if any component is a
// symbolic link, and an error wrapping [fs.ErrNotExist] if one is missing.
// [os.Root] already keeps a path from leaving the project; this keeps it from
// being redirected inside it.
func (p *Project) lstatPath(rel string) (fs.FileInfo, error) {
	var (
		info fs.FileInfo
		cur  string
	)
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		cur = path.Join(cur, part)
		var err error
		if info, err = p.root.Lstat(cur); err != nil {
			return nil, err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			return nil, fmt.Errorf("%w: %q", ErrLink, cur)
		case i < len(parts)-1 && !info.IsDir():
			return nil, fmt.Errorf("project: %q is not a folder", cur)
		}
	}
	return info, nil
}

// regular checks that rel is a regular file with one name only, reached through
// no link. exists is false, with no error, when the file or a folder above it is missing.
func (p *Project) regular(rel string) (exists bool, err error) {
	info, err := p.lstatPath(rel)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case !info.Mode().IsRegular():
		return true, fmt.Errorf("%w: %q", ErrNotRegular, rel)
	case hardLinks(info) > 1:
		return true, fmt.Errorf("%w: %q", ErrHardLink, rel)
	}
	return true, nil
}

// mkdir creates the folder dir and those above it, one at a time, refusing a
// symbolic link or a file on the way.
func (p *Project) mkdir(dir string) error {
	if dir == "." || dir == "" {
		return nil
	}
	cur := ""
	for part := range strings.SplitSeq(dir, "/") {
		cur = path.Join(cur, part)
		info, err := p.root.Lstat(cur)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := p.root.Mkdir(cur, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return fmt.Errorf("project: create folder: %w", err)
			}
		case err != nil:
			return fmt.Errorf("project: create folder: %w", err)
		case info.Mode()&fs.ModeSymlink != 0:
			return fmt.Errorf("%w: %q", ErrLink, cur)
		case !info.IsDir():
			return fmt.Errorf("project: %q is not a folder", cur)
		}
	}
	return nil
}

// errNoNative says the system has no single-call rename that refuses to
// replace, or the file system does not support it.
var errNoNative = errors.New("project: no native no-replace rename")

// renameNoReplace moves the file src to dst, which must not exist, without ever
// replacing something that appears there meanwhile. It asks the system for a
// rename that fails if dst exists (renameat2 on Linux, renameatx_np on macOS,
// MoveFileEx on Windows) and, where that is not available, links and unlinks.
func (p *Project) renameNoReplace(src, dst string) error {
	err := p.renameNative(src, dst)
	if !errors.Is(err, errNoNative) {
		return err
	}
	return p.linkThenUnlink(src, dst)
}

// linkThenUnlink links dst to src, which fails if dst exists, then removes src.
// If src cannot be removed, dst is removed again, so that the file is not left
// with two names, which every later write would refuse. Where links are not
// supported it falls back to a check followed by a rename.
func (p *Project) linkThenUnlink(src, dst string) error {
	err := p.root.Link(src, dst)
	switch {
	case err == nil:
		if rerr := p.root.Remove(src); rerr != nil {
			if derr := p.root.Remove(dst); derr != nil {
				return fmt.Errorf("project: move %q: %w", src, errors.Join(rerr, derr))
			}
			return fmt.Errorf("project: move %q: %w", src, rerr)
		}
		return nil
	case errors.Is(err, fs.ErrExist):
		return err
	}
	if _, serr := p.root.Lstat(dst); serr == nil {
		return fs.ErrExist
	}
	return p.root.Rename(src, dst)
}
