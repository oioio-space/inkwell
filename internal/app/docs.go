package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oioio-space/inkwell/internal/project"
)

// docsEnv overrides the folder holding the user's projects; end-to-end tests
// give each instance its own.
const docsEnv = "INKWELL_DOCS"

// OpenDocs opens the folder that holds the user's projects: $INKWELL_DOCS or,
// by default, "Inkwell" in the user's Documents folder. It is created, private
// to the user, when missing, without asking. The caller closes the result.
func OpenDocs() (*os.Root, error) {
	dir := os.Getenv(docsEnv)
	if dir == "" {
		docs, err := documentsDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(docs, "Inkwell")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", dir, err)
	}
	return root, nil
}

// documentsDir returns the user's Documents folder: $XDG_DOCUMENTS_DIR, else
// the XDG_DOCUMENTS_DIR of the user-dirs.dirs file, else ~/Documents (which
// is where macOS and Windows keep it).
func documentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	if dir := os.Getenv("XDG_DOCUMENTS_DIR"); filepath.IsAbs(dir) {
		return dir, nil
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		// #nosec G304 -- fixed file name in the user's configuration directory.
		if b, err := os.ReadFile(filepath.Join(cfg, "user-dirs.dirs")); err == nil {
			if dir := parseUserDirs(string(b), home); dir != "" {
				return dir, nil
			}
		}
	}
	return filepath.Join(home, "Documents"), nil
}

// parseUserDirs returns the XDG_DOCUMENTS_DIR of the user-dirs.dirs content,
// with "$HOME" expanded to home, or "" if it is missing or not absolute.
func parseUserDirs(content, home string) string {
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		value, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "XDG_DOCUMENTS_DIR=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		if rest, ok := strings.CutPrefix(value, "$HOME"); ok {
			value = home + rest
		}
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
	}
	return ""
}

// OpenProject opens the most recently used project of docs, or creates an
// untitled one when there is none, without asking. The caller closes it.
func OpenProject(docs *os.Root) (*project.Project, error) {
	dir, err := project.Latest(docs)
	if errors.Is(err, project.ErrNoProject) {
		return project.Create(docs, project.DefaultTitle, "")
	}
	if err != nil {
		return nil, err
	}
	return project.Open(docs, dir)
}
