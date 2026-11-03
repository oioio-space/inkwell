package sidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	protected := []string{home, config, cache, filepath.Join(home, ".local", "share"), filepath.Join(home, ".local", "state"), os.Getenv("XDG_DATA_HOME"), os.Getenv("XDG_STATE_HOME"), os.Getenv("XDG_RUNTIME_DIR"), "/etc", "/usr", "/var", "/proc", "/sys", "/dev", "/boot", "/run", os.Getenv("INKWELL_HOME"), os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME")}
	if dirs := os.Getenv("XDG_DATA_DIRS"); dirs != "" {
		protected = append(protected, filepath.SplitList(dirs)...)
	}
	if dirs := os.Getenv("XDG_CONFIG_DIRS"); dirs != "" {
		protected = append(protected, filepath.SplitList(dirs)...)
	}
	if filepath.Dir(path) == path {
		return "", errors.New("sidecar: root directory refused")
	}
	for _, sensitive := range protected {
		if sensitive == "" {
			continue
		}
		sensitive, err = filepath.Abs(sensitive)
		if err != nil {
			return "", err
		}
		// Resolve existing symlink prefixes even when the config directory is absent.
		tail := ""
		for {
			resolved, resolveErr := filepath.EvalSymlinks(sensitive)
			if resolveErr == nil {
				sensitive = filepath.Join(resolved, tail)
				break
			}
			if !os.IsNotExist(resolveErr) {
				return "", resolveErr
			}
			parent := filepath.Dir(sensitive)
			if parent == sensitive {
				return "", resolveErr
			}
			tail = filepath.Join(filepath.Base(sensitive), tail)
			sensitive = parent
		}
		rel, err := filepath.Rel(path, sensitive)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return "", fmt.Errorf("sidecar: directory contains home, system or XDG base directory")
		}
	}
	return path, nil
}
