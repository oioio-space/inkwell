package sidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// workingDirectory validates the explicit read authority before any launch.
func workingDirectory(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("sidecar: explicit project directory required")
	}
	path, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("sidecar: project directory is not a directory")
	}
	secret, broad, err := protectedDirs()
	if err != nil {
		return "", err
	}
	if err := checkWorkdir(path, secret, broad); err != nil {
		return "", err
	}
	return path, nil
}

// protectedDirs returns two resolved lists. A project may never be inside a
// secret directory: keys, the user's configuration, data and state, Inkwell's
// identity, and the kernel's pseudo-filesystems. A project may sit inside a
// broad directory but never contain one: home, caches, the runtime directory
// and the system tree, whose /run/media holds removable drives and /var/home
// the homes of image-based systems (Fedora Silverblue).
func protectedDirs() (secret, broad []string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, nil, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, nil, err
	}
	secret = []string{
		filepath.Join(home, ".ssh"), filepath.Join(home, ".gnupg"),
		config, os.Getenv("XDG_CONFIG_HOME"), os.Getenv("APPDATA"),
		filepath.Join(home, ".local", "share"), os.Getenv("XDG_DATA_HOME"),
		filepath.Join(home, ".local", "state"), os.Getenv("XDG_STATE_HOME"),
		os.Getenv("INKWELL_HOME"),
		"/proc", "/sys", "/dev",
	}
	broad = []string{
		home, cache, os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("LOCALAPPDATA"),
		"/etc", "/usr", "/var", "/boot", "/run",
	}
	broad = append(broad, filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))...)
	broad = append(broad, filepath.SplitList(os.Getenv("XDG_CONFIG_DIRS"))...)
	if secret, err = resolveAll(secret); err != nil {
		return nil, nil, err
	}
	if broad, err = resolveAll(broad); err != nil {
		return nil, nil, err
	}
	return secret, broad, nil
}

// resolveAll makes each non-empty path absolute and resolves its symbolic links.
func resolveAll(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		p, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if p, err = resolveProtected(p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// checkWorkdir applies the policy of [protectedDirs] to the resolved path.
func checkWorkdir(path string, secret, broad []string) error {
	if filepath.Dir(path) == path {
		return errors.New("sidecar: root directory refused")
	}
	for _, sensitive := range secret {
		if within(sensitive, path) {
			return fmt.Errorf("sidecar: directory %q is inside protected directory %q", path, sensitive)
		}
	}
	for _, sensitive := range slices.Concat(secret, broad) {
		if within(path, sensitive) {
			return errors.New("sidecar: directory contains home, system or XDG base directory")
		}
	}
	return nil
}

func within(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Resolve existing prefixes even when the sensitive directory is absent.
func resolveProtected(path string) (string, error) {
	tail := ""
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(resolved, tail), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		tail = filepath.Join(filepath.Base(path), tail)
		path = parent
	}
}
