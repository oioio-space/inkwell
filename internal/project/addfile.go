package project

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/oioio-space/inkwell/internal/store"
)

// ErrExists is returned by [Project.AddFile] when something is already at the
// path.
var ErrExists = errors.New("project: file exists")

// AddFile writes data as the new file rel of the project folder: a file the
// user dropped on the window, whose bytes only the webview has. It never
// replaces anything, so a file the user already had there is safe; it creates
// the folders above it, one at a time, refusing a symbolic link on the way;
// and nothing appears at rel before the whole content is on disk.
//
// The file is not Inkwell's yet: [Project.Claim] (through the blob store's
// PutBlob) makes it so. It returns [ErrExists] if there is already a file or a
// folder at rel, an [ErrInvalidName] for a path that is not portable, and
// [ErrLink] if rel, or a folder above it, is a symbolic link.
func (p *Project) AddFile(rel string, data []byte) error {
	if err := ValidatePath(rel); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.lstatPath(rel); err == nil {
		return fmt.Errorf("%w: %q", ErrExists, rel)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := p.mkdir(path.Dir(rel)); err != nil {
		return err
	}
	tmp := rel + "." + rand.Text() + tmpSuffix
	if err := store.WriteAtomic(p.root, tmp, data, 0o600); err != nil {
		return fmt.Errorf("project: add %s: %w", rel, err)
	}
	if err := p.renameNoReplace(tmp, rel); err != nil {
		_ = p.root.Remove(tmp) // best effort: the file was never in place
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %q", ErrExists, rel)
		}
		return fmt.Errorf("project: add %s: %w", rel, err)
	}
	return nil
}

// FilePath returns the absolute path of the regular file rel of the project, for
// handing it to the system (to open it in the program the user chose for that
// kind of file). The path is checked as any other ([ValidatePath]), and the
// file must be a plain file with one name, reached through no link, so that
// the system never opens something other than the project's own file.
func (p *Project) FilePath(rel string) (string, error) {
	if err := ValidatePath(rel); err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	exists, err := p.regular(rel)
	switch {
	case err != nil:
		return "", err
	case !exists:
		return "", fmt.Errorf("project: %q: %w", rel, fs.ErrNotExist)
	}
	return filepath.Join(p.root.Name(), filepath.FromSlash(rel)), nil
}
