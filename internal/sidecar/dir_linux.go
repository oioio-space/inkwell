package sidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// directoryLimits prepares an empty, caller-owned private directory. The caller
// must not mutate it concurrently, including through another Run.
func directoryLimits(spec Spec) (Spec, []string, error) {
	if spec.WatchDir == "" || spec.MaxDirBytes <= 0 {
		return spec, nil, nil
	}
	if len(spec.WriteFiles) == 0 {
		return spec, nil, errors.New("sidecar: directory bound requires WriteFiles")
	}
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 || abi < 3 {
		return spec, nil, errors.New("sidecar: directory bound requires Landlock ABI >= 3")
	}

	dir, err := filepath.Abs(spec.WatchDir)
	if err != nil {
		return spec, nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return spec, nil, err
	}
	defer func() { _ = root.Close() }()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		return spec, nil, errors.New("sidecar: bounded directory must be empty and readable")
	}
	if err := validateOutputNames(spec.WriteFiles); err != nil {
		return spec, nil, err
	}
	for _, name := range spec.WriteFiles {
		f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return spec, nil, fmt.Errorf("sidecar: prepare output: %w", err)
		}
		if err := f.Close(); err != nil {
			return spec, nil, err
		}
	}
	overlay, err := probeOutputMount()
	if err != nil {
		return spec, nil, err
	}
	path, args, err := sandboxPrefix(sandboxConfig{
		RequireDeath: spec.DeathTest != "",
		ReadPaths:    spec.ReadPaths,
		WorkingDir:   spec.Dir,
		Overlay:      overlay,
		Directory:    dir,
		Names:        spec.WriteFiles,
		File:         directoryFileCap(spec, len(spec.WriteFiles)),
		Memory:       spec.MemoryLimit,
	})
	spec.FileSizeLimit, spec.MemoryLimit = 0, 0
	return spec, append([]string{path}, args...), err
}

// A failed write may return EFBIG instead of terminating with SIGXFSZ.
func directoryLimitHit(spec Spec, out Output) bool {
	if spec.WatchDir == "" || spec.MaxDirBytes <= 0 || len(spec.WriteFiles) == 0 || out.ExitCode == 0 {
		return false
	}
	capBytes := directoryFileCap(spec, len(spec.WriteFiles))
	for _, name := range spec.WriteFiles {
		info, err := os.Stat(filepath.Join(spec.WatchDir, name))
		if err == nil && info.Size() >= 0 && uint64(info.Size()) >= capBytes { // #nosec G115 -- guarded by Size() >= 0
			return true
		}
	}
	return false
}

func validateOutputNames(names []string) error {
	seen := make(map[string]bool)
	for _, name := range names {
		if name == "." || name == ".." || name == "" || strings.ContainsAny(name, "/\\") || seen[name] {
			return errors.New("sidecar: WriteFiles requires unique simple file names")
		}
		seen[name] = true
	}
	return nil
}

// Remove unused precreated slots, so callers only see produced outputs.
func cleanupEmptyOutputs(s Spec) {
	if s.WatchDir == "" || s.MaxDirBytes <= 0 {
		return
	}
	for _, name := range s.WriteFiles {
		path := filepath.Join(s.WatchDir, name)
		if info, err := os.Stat(path); err == nil && info.Size() == 0 {
			_ = os.Remove(path)
		}
	}
}

// directoryFileCap bounds one file, sharing the budget across slots when the
// filesystem cannot enforce the aggregate limit. Budget and slots are positive.
func directoryFileCap(spec Spec, slots int) uint64 {
	capBytes := uint64(spec.MaxDirBytes / int64(slots)) // #nosec G115 -- positive budget and slots
	if spec.FileSizeLimit > 0 {
		capBytes = min(capBytes, spec.FileSizeLimit)
	}
	return capBytes
}
