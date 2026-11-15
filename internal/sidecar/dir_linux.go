package sidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

// directoryLimits prepares an empty, caller-owned private directory. The caller
// must not mutate it concurrently, including through another Run.
func directoryLimits(spec Spec, plan launchPlan) (Spec, []string, error) {
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
	if err := validateOutputNames(spec.WriteFiles); err != nil {
		return spec, nil, err
	}
	if spec.slotsReady {
		// The placement fallback (plan T-6): the slots of the first attempt
		// are reused, never created twice; they must be exactly ours, empty.
		if err := ownEmptySlots(dir, spec.WriteFiles); err != nil {
			return spec, nil, err
		}
	} else if err := createSlots(root, dir, spec.WriteFiles); err != nil {
		return spec, nil, err
	}
	c := baseConfig(spec, plan)
	c.Directory, c.Names, c.File = dir, spec.WriteFiles, directoryFileCap(spec, len(spec.WriteFiles))
	path, args, err := sandboxPrefix(c)
	spec.FileSizeLimit, spec.MemoryLimit = 0, 0
	return spec, append([]string{path}, args...), err
}

// createSlots creates each output slot in the empty folder dir.
func createSlots(root *os.Root, dir string, names []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		return errors.New("sidecar: bounded directory must be empty and readable")
	}
	for _, name := range names {
		f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("sidecar: prepare output: %w", err)
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// ownEmptySlots checks that dir holds exactly the empty slots names.
func ownEmptySlots(dir string, names []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(names) {
		return errors.New("sidecar: the output slots changed before the retry")
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() != 0 || !slices.Contains(names, e.Name()) {
			return errors.New("sidecar: the output slots changed before the retry")
		}
	}
	return nil
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
