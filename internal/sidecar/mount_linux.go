package sidecar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// See Documentation/filesystems/tmpfs (size= and nr_inodes=),
// https://docs.kernel.org/filesystems/tmpfs.html, and user_namespaces(7) /
// mount_namespaces(7). The probe includes the mount: namespace creation alone
// does not detect all AppArmor, container and sysctl restrictions.
// Spec §9.11, refuse rather than weaken: only a DEFINITIVE negative result
// selects the launch without namespaces, and it is cached. Definitive means
// clone/unshare (or the id-map setup) refused with EPERM or EINVAL (sysctl
// kernel.unprivileged_userns_clone=0, no CONFIG_USER_NS), or with ENOSPC while
// user.max_user_namespaces is 0 (ENOSPC under a positive limit only means the
// limit is reached for now: transient), or the probe child itself got one of those errors setting
// up its mounts (AppArmor's restricted unprivileged namespaces) and said so
// with probeUnsupportedExit. That fallback keeps no_new_privs, the dropped
// capabilities, the resource limits and the two mandatory layers, Landlock and
// seccomp, but has no user, mount, PID or IPC namespace and no IPC overlay.
//
// Anything else (the probe outlives probeTimeout, a crash, a fork failure on
// EAGAIN) is inconclusive: the launch is refused with ErrProbeInconclusive and
// nothing is cached; the probe runs again once a backoff has elapsed, and
// launches before then are refused with the same retryable error.
const (
	probeTimeout    = 5 * time.Second
	probeBackoffMin = time.Second
	probeBackoffMax = 30 * time.Second
	// probeUnsupportedExit is the probe child's exit status for a definitive refusal.
	probeUnsupportedExit = 120
)

var probeOutputMount = cachedProbe(func(ctx context.Context) (bool, error) {
	dir, err := os.MkdirTemp("", "sidecar-mount-probe-")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path, args, err := sandboxPrefix(sandboxConfig{Directory: dir, MountBytes: 4096, Probe: true, Overlay: true})
	if err != nil {
		return false, err
	}
	return runMountProbe(ctx, []string{}, path, args...)
}, probeTimeout, time.Now)

// definitiveRefusal reports whether err is a kernel or policy refusal of
// unprivileged namespaces rather than a transient failure.
func definitiveRefusal(err error) bool {
	if errors.Is(err, unix.ENOSPC) {
		limit, readErr := userNamespaceLimit()
		return readErr == nil && limit == 0
	}
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EINVAL)
}

// userNamespaceLimit reads user.max_user_namespaces; tests replace it.
var userNamespaceLimit = func() (int, error) {
	data, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// runMountProbe runs one probe child in the sandbox's mount namespaces.
func runMountProbe(ctx context.Context, env []string, path string, args ...string) (bool, error) {
	return runNamespaceProbe(ctx, mountNamespace, env, path, args...)
}

// runNamespaceProbe runs one probe child in the namespaces that namespaces
// requests; ctx kills it. It returns (true, nil) on success, (false, nil) on a
// definitive refusal and an error when the result is inconclusive.
func runNamespaceProbe(ctx context.Context, namespaces func(*exec.Cmd), env []string, path string, args ...string) (bool, error) {
	cmd := exec.CommandContext(ctx, path, args...) // #nosec G204 -- re-exec our own binary.
	setProcAttr(cmd)
	namespaces(cmd)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, exited := errors.AsType[*exec.ExitError](err)
	switch {
	case err == nil:
		return true, nil
	case ctx.Err() != nil:
		return false, ctx.Err()
	case exited && exitErr.ExitCode() == probeUnsupportedExit:
		return false, nil // the child's mounts were refused
	case !exited && definitiveRefusal(err):
		return false, nil // Start itself was refused: clone or the id maps
	}
	if detail := strings.TrimSpace(stderr.String()); detail != "" {
		return false, fmt.Errorf("%w: %s", err, detail)
	}
	return false, err
}

// cachedProbe serialises probe calls and bounds each to timeout. A
// conclusive answer is cached for good; an inconclusive one is returned as
// ErrProbeInconclusive, retried once a backoff measured on now has elapsed.
func cachedProbe(probe func(context.Context) (bool, error), timeout time.Duration, now func() time.Time) func() (bool, error) {
	var mu sync.Mutex
	var decided, supported bool
	var lastErr error
	var retryAt time.Time
	var backoff time.Duration
	return func() (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		if decided {
			return supported, nil
		}
		if lastErr != nil && now().Before(retryAt) {
			return false, lastErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ok, err := probe(ctx)
		if err == nil {
			decided, supported = true, ok
			return ok, nil
		}
		backoff = min(max(2*backoff, probeBackoffMin), probeBackoffMax)
		retryAt = now().Add(backoff)
		lastErr = fmt.Errorf("%w: %w", ErrProbeInconclusive, err)
		return false, lastErr
	}
}

type directoryMount struct {
	spec                                     Spec
	ready, readyChild, proceed, proceedChild *os.File
	root                                     *os.Root
}

func newDirectoryMount(s Spec) (*directoryMount, error) {
	if s.WatchDir == "" || s.MaxDirBytes <= 0 || s.MaxDirBytes < int64(os.Getpagesize()) {
		return nil, nil
	}
	if supported, err := probeOutputMount(); err != nil || !supported {
		return nil, err
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
	path, args, err := sandboxPrefix(sandboxConfig{
		RequireDeath: m.spec.DeathTest != "",
		ReadPaths:    m.spec.ReadPaths,
		WorkingDir:   cwd,
		Directory:    dir,
		Names:        m.spec.WriteFiles,
		Overlay:      true,
		MountBytes:   m.spec.MaxDirBytes,
		File:         directoryFileCap(m.spec, 1),
		Memory:       m.spec.MemoryLimit,
	})
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
