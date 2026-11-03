package sidecar

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// See Documentation/filesystems/tmpfs (size= and nr_inodes=),
// https://docs.kernel.org/filesystems/tmpfs.html, and user_namespaces(7) /
// mount_namespaces(7). The probe includes the mount: namespace creation alone
// does not detect all AppArmor, container and sysctl restrictions.
// A command failure does not reliably distinguish unsupported namespaces from
// temporary resource exhaustion. Cache success only; retry every failed probe.
var probeOutputMount = cachedMountProbe(func() bool {
	dir, err := os.MkdirTemp("", "sidecar-mount-probe-")
	if err != nil {
		return false
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path, args, err := sandboxPrefix(sandboxConfig{Directory: dir, MountBytes: 4096, Probe: true, Overlay: true})
	if err != nil {
		return false
	}
	cmd := exec.Command(path, args...) // #nosec G204 -- re-exec our own binary.
	setProcAttr(cmd)
	mountNamespace(cmd)
	cmd.Env = []string{}
	return cmd.Run() == nil
})

func cachedMountProbe(probe func() bool) func() bool {
	var mu sync.Mutex
	var supported bool
	return func() bool {
		mu.Lock()
		defer mu.Unlock()
		if !supported {
			supported = probe()
		}
		return supported
	}
}

type directoryMount struct {
	spec                                     Spec
	ready, readyChild, proceed, proceedChild *os.File
	root                                     *os.Root
}

func newDirectoryMount(s Spec) (*directoryMount, error) {
	if s.WatchDir == "" || s.MaxDirBytes <= 0 || s.MaxDirBytes < int64(os.Getpagesize()) || !probeOutputMount() {
		return nil, nil
	}
	entries, err := os.ReadDir(s.WatchDir)
	if err != nil {
		return nil, err
	}
	if len(entries) != 0 || len(s.WriteFiles) == 0 {
		return nil, errors.New("sidecar: bounded directory must be empty")
	}
	if err := validateOutputNames(s.WriteFiles); err != nil {
		return nil, err
	}
	m := &directoryMount{spec: s}
	m.ready, m.readyChild, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	m.proceedChild, m.proceed, err = os.Pipe()
	if err != nil {
		m.close()
		return nil, err
	}
	return m, nil
}

func (m *directoryMount) configure(cmd *exec.Cmd) error {
	dir, err := filepath.Abs(m.spec.WatchDir)
	if err != nil {
		return err
	}
	cwd, err := workingDirectory(cmd.Dir)
	if err != nil {
		return err
	}
	path, args, err := sandboxPrefix(sandboxConfig{ReadPaths: m.spec.ReadPaths, WorkingDir: cwd, Directory: dir, Names: m.spec.WriteFiles, Overlay: true, MountBytes: m.spec.MaxDirBytes, File: directoryFileCap(m.spec, 1), Memory: m.spec.MemoryLimit})
	if err != nil {
		return err
	}
	cmd.Args = append(append([]string{path}, args...), cmd.Args...)
	cmd.Path = path
	mountNamespace(cmd)
	cmd.ExtraFiles = []*os.File{m.readyChild, m.proceedChild}
	return nil
}

func (m *directoryMount) open(pid int) error {
	_ = m.readyChild.Close()
	_ = m.proceedChild.Close()
	var b [1]byte
	if _, err := io.ReadFull(m.ready, b[:]); err != nil {
		return fmt.Errorf("sidecar: mount handshake: %w", err)
	}
	dir, err := filepath.Abs(m.spec.WatchDir)
	if err != nil {
		return err
	}
	// Pin the private mount before exec; it survives child exit through this fd.
	m.root, err = os.OpenRoot(fmt.Sprintf("/proc/%d/root%s", pid, dir))
	if err != nil {
		return fmt.Errorf("sidecar: pin output mount: %w", err)
	}
	_, err = m.proceed.Write([]byte("go\n"))
	return err
}

func (m *directoryMount) export() (err error) {
	var created []string
	defer func() {
		if err != nil {
			for _, path := range created {
				err = errors.Join(err, os.Remove(path))
			}
		}
	}()
	entries, err := fs.ReadDir(m.root.FS(), ".")
	if err != nil {
		return err
	}
	var total int64
	// tmpfs limits allocation, not sparse logical lengths. Verify the complete
	// snapshot before exporting anything, then bound every copy independently.
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > m.spec.MaxDirBytes-total {
			return errors.New("sidecar: invalid or oversized output")
		}
		total += info.Size()
	}
	remaining := m.spec.MaxDirBytes
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Size() == 0 {
			continue
		}
		in, err := m.root.Open(e.Name())
		if err != nil {
			return err
		}
		path := filepath.Join(m.spec.WatchDir, e.Name())
		copyErr := exportFile(path, in, info.Size(), remaining)
		err = errors.Join(copyErr, in.Close())
		if copyErr == nil {
			created = append(created, path)
		}
		if err != nil {
			return err
		}
		remaining -= info.Size()
	}
	return nil
}

// exportFile removes its destination on any copy, size or close failure.
func exportFile(path string, in io.Reader, size, remaining int64) (err error) {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- path is the caller-private WatchDir joined with a validated simple name; O_EXCL.
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
		if err != nil {
			err = errors.Join(err, os.Remove(path))
		}
	}()
	n, err := io.Copy(out, io.LimitReader(in, remaining))
	if n != size {
		err = errors.Join(err, fmt.Errorf("sidecar: output size changed: copied %d bytes, expected %d", n, size))
	}
	return err
}

func (m *directoryMount) close() {
	for _, f := range []*os.File{m.ready, m.readyChild, m.proceed, m.proceedChild} {
		if f != nil {
			_ = f.Close()
		}
	}
	if m.root != nil {
		_ = m.root.Close()
	}
}

func (m *directoryMount) limitHit(out Output) bool {
	if out.ExitCode == 0 {
		return false
	}
	f, err := m.root.Open(".")
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	var stat unix.Statfs_t
	if unix.Fstatfs(int(f.Fd()), &stat) == nil && stat.Bavail == 0 {
		return true
	}
	limit := directoryFileCap(m.spec, 1)
	for _, name := range m.spec.WriteFiles {
		info, err := m.root.Stat(name)
		if err == nil && info.Size() >= 0 && uint64(info.Size()) >= limit { // #nosec G115 -- guarded by Size() >= 0
			return true
		}
	}
	return false
}

// Mount privileges exist only in the child's private user namespace.
func mountNamespace(cmd *exec.Cmd) {
	a := cmd.SysProcAttr
	a.Cloneflags |= syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNS
	a.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}}
	a.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}}
	a.GidMappingsEnableSetgroups = false
}
