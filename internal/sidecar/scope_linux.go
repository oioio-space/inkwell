package sidecar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

const (
	holderMode        = "--inkwell-internal-sandbox-holder-v1"
	expectedCgroupEnv = "INKWELL_INTERNAL_CGROUP"
	expectedParentEnv = "INKWELL_INTERNAL_PARENT"
)

// SandboxScope serializes setup and versions sessions. Runs own references to
// their verified session; invalidating one never redirects a running workload.
type SandboxScope struct {
	mu      sync.Mutex
	session *scopeSession
	retryAt time.Time
	backoff time.Duration
	logged  bool
	// fault is a per-manager test seam, never process-global.
	fault        func(string) error
	beforeStart  func(*cgroupRun)
	afterStarted func(*cgroupRun)
	afterJob     func(*scopeSession)
	trace        func(string)
	write        func(string, string) error
}
type scopeSession struct {
	path, relative string
	holder         *exec.Cmd
	pipe           *os.File
	pidfd          int
	refs           int
	stale          bool
	next           uint64
}
type cgroupRun struct {
	command        *exec.Cmd
	owner          *SandboxScope
	session        *scopeSession
	path, relative string
	fd             int
	watch          *memoryWatch
	once           sync.Once
}

var appSandbox SandboxScope

func init() {
	if len(os.Args) != 2 || os.Args[1] != holderMode {
		return
	}
	r := os.NewFile(3, "holder-lifetime")
	_, _ = io.Copy(io.Discard, r)
	_ = r.Close()
	// EOF is the app-lifetime barrier. Only our delegated subtree is touched.
	relative, err := selfCgroup()
	if err != nil {
		os.Exit(1)
	}
	path := filepath.Join("/sys/fs/cgroup", relative)
	if filepath.Base(path) != "holder" || !strings.HasPrefix(filepath.Base(filepath.Dir(path)), "inkwell-sandbox-") {
		os.Exit(0)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		os.Exit(1)
	}
	deadline := time.Now().Add(5 * time.Second)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "run-") && entry.IsDir() {
			_ = removeRunCgroup(filepath.Join(filepath.Dir(path), entry.Name()), deadline)
		}
	}
	os.Exit(0)
}

func (m *SandboxScope) step(name string) error {
	if m.fault != nil {
		return m.fault(name)
	}
	return nil
}
func selfCgroup() (string, error) { return procCgroup("self") }

// pidCgroup is the unified (v2) cgroup path of process pid.
func pidCgroup(pid int) (string, error) { return procCgroup(strconv.Itoa(pid)) }

func procCgroup(proc string) (string, error) {
	b, err := os.ReadFile("/proc/" + proc + "/cgroup") // #nosec G304 -- proc is "self" or a decimal PID, never a caller-supplied path.
	return unifiedCgroup(b, err)
}

func unifiedCgroup(b []byte, err error) (string, error) {
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if p, ok := strings.CutPrefix(line, "0::"); ok && strings.HasPrefix(p, "/") && filepath.Clean(p) == p {
			return p, nil
		}
	}
	return "", errors.New("no unified cgroup")
}

func readValue(path string) (string, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- fixed control filename joined only to our own scope path or its kernel-reported ancestor.
	return strings.TrimSpace(string(b)), err
}
func writeValue(path, value string) error { return os.WriteFile(path, []byte(value), 0o600) }
func holderAlive(s *scopeSession) bool {
	if s.pidfd < 0 || s.pidfd > math.MaxInt32 {
		return false
	}
	p := []unix.PollFd{{Fd: int32(s.pidfd), Events: unix.POLLIN}}
	n, err := unix.Poll(p, 0)
	return err == nil && n == 0
}

func (m *SandboxScope) fallback(err error) {
	m.backoff = min(max(m.backoff*2, time.Second), 30*time.Second)
	m.retryAt = time.Now().Add(m.backoff)
	if !m.logged {
		reason := "unavailable"
		if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) {
			reason = "permission"
		}
		slog.Warn("sidecar aggregate memory scope unavailable; using per-process limits", "reason", reason)
		m.logged = true
	}
}

func (m *SandboxScope) prepare(ctx context.Context, spec Spec, cmd *exec.Cmd) *cgroupRun {
	if spec.disableScope || spec.MemoryLimit == 0 && spec.ResidentLimit == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Cloneflags&unix.CLONE_NEWPID == 0 {
		m.fallback(errors.New("PID namespace unavailable"))
		return nil
	}
	if !m.ensureSession(ctx) {
		return nil
	}
	s := m.session
	s.next++
	r := &cgroupRun{
		command:  cmd,
		owner:    m,
		session:  s,
		path:     filepath.Join(s.path, fmt.Sprintf("run-%d", s.next)),
		relative: filepath.Join(s.relative, fmt.Sprintf("run-%d", s.next)),
		fd:       -1,
	}
	fail := func(err error) *cgroupRun {
		_ = removeRunCgroup(r.path, time.Now().Add(5*time.Second))
		m.fallback(err)
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENODEV) {
			s.stale = true
			m.session = nil
			if s.refs == 0 {
				closeSession(s)
			}
		}
		return nil
	}
	if err := m.configureRunLimits(r, spec); err != nil {
		return fail(err)
	}
	fd, err := unix.Open(r.path, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fail(err)
	}
	r.fd = fd
	r.watch = startMemoryWatch(r.path, s.path)
	s.refs++
	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = fd
	expected := r.relative
	if spec.bootstrapExpected != "" {
		expected = spec.bootstrapExpected
	}
	cmd.Env = append(cmd.Env, expectedCgroupEnv+"="+expected, expectedParentEnv+"="+strconv.Itoa(os.Getpid()))
	return r
}

func (m *SandboxScope) configureRunLimits(r *cgroupRun, spec Spec) error {
	if err := m.step("F8"); err != nil {
		return err
	}
	if err := os.Mkdir(r.path, 0o700); err != nil {
		return err
	}
	resident := spec.ResidentLimit
	if resident == 0 {
		resident = spec.MemoryLimit
	}
	pagesize := os.Getpagesize()
	if pagesize <= 0 {
		return errors.New("invalid page size")
	}
	resident = resident / uint64(pagesize) * uint64(pagesize)
	if resident == 0 {
		return errors.New("resident budget below page size")
	}
	limits := []struct{ name, value string }{
		{"memory.max", strconv.FormatUint(resident, 10)},
		{"memory.swap.max", "0"},
		{"memory.oom.group", "1"},
	}
	for _, limit := range limits {
		if err := m.writeLimit(filepath.Join(r.path, limit.name), limit.value); err != nil {
			return err
		}
	}
	if err := m.step("F9"); err != nil {
		return err
	}
	for _, limit := range limits {
		got, err := readValue(filepath.Join(r.path, limit.name))
		if err != nil || got != limit.value {
			return errors.Join(err, errors.New("limit read-back mismatch"))
		}
	}
	return nil
}

func (m *SandboxScope) ensureSession(ctx context.Context) bool {
	if m.session != nil && !holderAlive(m.session) {
		old := m.session
		m.session = nil
		old.stale = true
		if old.refs == 0 {
			closeSession(old)
		}
	}
	if m.session == nil {
		if time.Now().Before(m.retryAt) {
			return false
		}
		s, err := m.setup(ctx)
		if err != nil {
			m.fallback(err)
			return false
		}
		m.session = s
		m.backoff = 0
		m.retryAt = time.Time{}
	}
	return true
}

func (r *cgroupRun) started() {
	if r.fd >= 0 {
		_ = unix.Close(r.fd)
		r.fd = -1
	}
}

func (r *cgroupRun) finish(signal int) MemoryCause {
	cause := MemoryUnknown
	r.once.Do(func() {
		r.started()
		cause = r.watch.finish(signal)
		if r.owner.trace != nil {
			r.owner.trace("counters")
			r.owner.trace("cleanup")
		}
		_ = removeRunCgroup(r.path, time.Now().Add(5*time.Second))
		m := r.owner
		m.mu.Lock()
		defer m.mu.Unlock()
		s := r.session
		s.refs--
		if s.stale && s.refs == 0 {
			closeSession(s)
		}
	})
	return cause
}

func (r *cgroupRun) placementFailed() {
	r.owner.mu.Lock()
	s := r.session
	s.stale = true
	if r.owner.session == s {
		r.owner.session = nil
	}
	r.owner.mu.Unlock()
	r.finish(0)
}

func closeSession(s *scopeSession) { _ = s.pipe.Close(); _ = s.holder.Wait(); _ = unix.Close(s.pidfd) }

func removeRunCgroup(path string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		if err := killRunCgroup(path, func(pid int) error { return killCgroupPID(path, pid) }); err != nil {
			return err
		}
		v, err := readCounters(filepath.Join(path, "cgroup.events"))
		if err != nil {
			return err
		}
		if n, ok := v["populated"]; ok && n == 0 {
			return os.Remove(path)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("cgroup remained populated")
}

type scopeProperty struct {
	Name  string
	Value dbus.Variant
}

func scopeProp(n string, v any) scopeProperty { return scopeProperty{n, dbus.MakeVariant(v)} }
func (m *SandboxScope) setup(parent context.Context) (session *scopeSession, err error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	app, err := selfCgroup()
	if err != nil {
		return nil, err
	}
	user := fmt.Sprintf("user@%d.service", os.Getuid())
	if !strings.Contains(app, "/"+user+"/") {
		return nil, errors.New("app outside user manager")
	}
	slice := filepath.Dir(app)
	for !strings.HasSuffix(filepath.Base(slice), ".slice") && slice != "/" {
		slice = filepath.Dir(slice)
	}
	if !strings.Contains(slice, "/"+user+"/") {
		return nil, errors.New("no shared user slice")
	}
	sweepStaleScopes(filepath.Join("/sys/fs/cgroup", slice))
	if err = m.step("F11"); err != nil {
		return nil, err
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	holder := exec.Command("/proc/self/exe", holderMode)
	holder.ExtraFiles = []*os.File{r}
	holder.Env = []string{}
	pidfd := -1
	holder.SysProcAttr = &syscall.SysProcAttr{PidFD: &pidfd}
	if err = holder.Start(); err != nil {
		_ = w.Close()
		return nil, err
	}
	_ = r.Close()
	s := &scopeSession{holder: holder, pipe: w, pidfd: pidfd}
	defer func() {
		if err != nil {
			closeSession(s)
		}
	}()
	if err = m.step("F1"); err != nil {
		return nil, err
	}
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	manager := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	if err = m.step("F2"); err != nil {
		return nil, err
	}
	if err = conn.AddMatchSignalContext(ctx, dbus.WithMatchInterface("org.freedesktop.systemd1.Manager"), dbus.WithMatchMember("JobRemoved")); err != nil {
		return nil, err
	}
	ch := make(chan *dbus.Signal, 64)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	if err = manager.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.Subscribe", 0).Err; err != nil {
		return nil, err
	}
	var nonce [12]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("inkwell-sandbox-%d-%s.scope", os.Getpid(), hex.EncodeToString(nonce[:]))
	s.relative = filepath.Join(slice, name)
	s.path = filepath.Join("/sys/fs/cgroup", s.relative)
	if holder.Process.Pid <= 0 || holder.Process.Pid > math.MaxInt32 {
		return nil, errors.New("invalid holder PID")
	}
	props := []scopeProperty{
		scopeProp("PIDs", []uint32{uint32(holder.Process.Pid)}),
		scopeProp("Delegate", true),
		scopeProp("Slice", filepath.Base(slice)),
		scopeProp("CollectMode", "inactive-or-failed"),
	}
	props, err = inheritedScopeProperties(app, props)
	if err != nil {
		return nil, err
	}
	if err = m.step("F3"); err != nil {
		return nil, err
	}
	var job dbus.ObjectPath
	if err = manager.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.StartTransientUnit", 0, name, "fail", props, []struct {
		Name       string
		Properties []scopeProperty
	}{}).Store(&job); err != nil {
		return nil, err
	}
	if err := waitJob(ctx, ch, job); err != nil {
		return nil, err
	}
	if m.afterJob != nil {
		m.afterJob(s)
	}
	if err = m.step("F4"); err != nil {
		return nil, err
	}
	if err = m.validateSession(s, app); err != nil {
		return nil, err
	}
	return s, nil
}

func inheritedScopeProperties(app string, props []scopeProperty) ([]scopeProperty, error) {
	for _, v := range []struct{ file, prop string }{{"memory.max", "MemoryMax"}, {"memory.swap.max", "MemorySwapMax"}} {
		limit, err := readValue(filepath.Join("/sys/fs/cgroup", app, v.file))
		if err != nil {
			return nil, err
		}
		if limit != "max" {
			n, err := strconv.ParseUint(limit, 10, 64)
			if err != nil {
				return nil, err
			}
			props = append(props, scopeProp(v.prop, n))
		}
	}
	return props, nil
}

func (m *SandboxScope) validateSession(s *scopeSession, app string) error {
	actual, err := pidCgroup(s.holder.Process.Pid)
	if err != nil || actual != s.relative {
		return errors.New("holder membership mismatch")
	}
	uid := os.Getuid()
	if uid < 0 || uint64(uid) > math.MaxUint32 {
		return errors.New("invalid user ID")
	}
	var st unix.Stat_t
	if err = unix.Stat(s.path, &st); err != nil || st.Uid != uint32(uid) {
		return errors.New("scope ownership mismatch")
	}
	if err = m.step("F5"); err != nil {
		return err
	}
	controllers, err := readValue(filepath.Join(s.path, "cgroup.controllers"))
	if err != nil || !containsWord(controllers, "memory") {
		return errors.New("memory not delegated")
	}
	if err = m.step("F6"); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(s.path, "holder"), 0o700); err != nil {
		return err
	}
	if err = writeValue(filepath.Join(s.path, "holder/cgroup.procs"), strconv.Itoa(s.holder.Process.Pid)); err != nil {
		return err
	}
	actual, err = readValue(filepath.Join(s.path, "holder/cgroup.procs"))
	if err != nil || actual != strconv.Itoa(s.holder.Process.Pid) {
		return errors.New("holder move not verified")
	}
	if actual, err = readValue(filepath.Join(s.path, "cgroup.procs")); err != nil || actual != "" {
		return errors.New("scope has internal processes")
	}
	if err = writeValue(filepath.Join(s.path, "cgroup.subtree_control"), "+memory"); err != nil {
		return err
	}
	actual, err = readValue(filepath.Join(s.path, "cgroup.subtree_control"))
	if err != nil || !containsWord(actual, "memory") {
		return errors.New("memory not enabled")
	}
	if err = m.step("F7"); err != nil {
		return err
	}
	for _, file := range []string{"memory.max", "memory.swap.max"} {
		want, err := readValue(filepath.Join("/sys/fs/cgroup", app, file))
		if err != nil {
			return err
		}
		got, err := readValue(filepath.Join(s.path, file))
		if err != nil || want != "max" && got != want {
			return errors.New("inherited leaf bound mismatch")
		}
	}
	if err = checkCgroupKill(s.path); err != nil {
		return err
	}
	if !holderAlive(s) {
		return errors.New("holder died during setup")
	}
	return nil
}

func containsWord(s, w string) bool {
	for v := range strings.FieldsSeq(s) {
		if v == w {
			return true
		}
	}
	return false
}

func sweepStaleScopes(slice string) {
	entries, err := os.ReadDir(slice)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "inkwell-sandbox-") {
			continue
		}
		parts := strings.Split(entry.Name(), "-")
		if len(parts) != 4 {
			continue
		}
		pid, err := strconv.Atoi(parts[2])
		if err != nil || pid <= 0 {
			continue
		}
		if err = unix.Kill(pid, 0); !errors.Is(err, unix.ESRCH) {
			continue
		}
		root := filepath.Join(slice, entry.Name())
		holder, err := readValue(filepath.Join(root, "holder/cgroup.procs"))
		if err != nil || holder != "" {
			continue
		}
		runs, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		deadline := time.Now().Add(5 * time.Second)
		for _, run := range runs {
			if run.IsDir() && strings.HasPrefix(run.Name(), "run-") {
				_ = removeRunCgroup(filepath.Join(root, run.Name()), deadline)
			}
		}
	}
}

func (m *SandboxScope) writeLimit(path, value string) error {
	if m.write != nil {
		return m.write(path, value)
	}
	return writeValue(path, value)
}

func (r *cgroupRun) start(cmd *exec.Cmd) error {
	defer r.started()
	if r.owner.beforeStart != nil {
		r.owner.beforeStart(r)
	}
	if err := r.owner.step("F10"); err != nil {
		return err
	}
	return cmd.Start()
}

func (r *cgroupRun) afterStart() {
	if r.owner.afterStarted != nil {
		r.owner.afterStarted(r)
	}
}

func waitJob(ctx context.Context, ch <-chan *dbus.Signal, job dbus.ObjectPath) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case signal, ok := <-ch:
			if !ok {
				return errors.New("manager disconnected")
			}
			if signal == nil || signal.Name != "org.freedesktop.systemd1.Manager.JobRemoved" || len(signal.Body) != 4 || signal.Body[1] != job {
				continue
			}
			if signal.Body[3] != "done" {
				return errors.New("scope job failed")
			}
			return nil
		}
	}
}

func (r *cgroupRun) placementError(err error) bool {
	if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EBUSY) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENODEV) {
		return true
	}
	// ENOENT can also name a missing executable. Only a vanished run directory
	// proves that it was the placement that failed.
	if errors.Is(err, unix.ENOENT) {
		_, statErr := os.Stat(r.path)
		return errors.Is(statErr, os.ErrNotExist)
	}
	return false
}

// Pre-5.14 kernels lack cgroup.kill. Our run cgroups are leaves: workloads
// cannot create child cgroups or migrate tasks. Repeat the procs sweep until
// cgroup.events is empty, including children forked during a previous sweep.
func checkCgroupKill(path string) error {
	root, err := os.OpenRoot(path) // #nosec G703 -- path is our own scope/run cgroup; callers construct it under the delegated subtree.
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile("cgroup.kill", os.O_WRONLY, 0)
	if err == nil {
		return f.Close()
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err = root.ReadFile("cgroup.procs")
	return err
}

func killRunCgroup(path string, kill func(int) error) error {
	root, err := os.OpenRoot(path) // #nosec G703 -- path is our own scope/run cgroup; callers construct it under the delegated subtree.
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile("cgroup.kill", os.O_WRONLY, 0)
	if err == nil {
		_, err = f.WriteString("1")
		return errors.Join(err, f.Close())
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := root.ReadFile("cgroup.procs")
	if err != nil {
		return err
	}
	for text := range strings.FieldsSeq(string(b)) {
		pid, err := strconv.Atoi(text)
		if err != nil || pid <= 0 {
			return errors.New("invalid cgroup PID")
		}
		if err := kill(pid); err != nil && !errors.Is(err, unix.ESRCH) {
			return err
		}
	}
	return nil
}

func killCgroupPID(path string, pid int) error {
	// Pin identity before signaling; an exited task must never redirect a kill
	// to a recycled host PID. The holder shares the app's user and permissions.
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(fd) }()
	relative, err := pidCgroup(pid)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if filepath.Join("/sys/fs/cgroup", relative) != path {
		return nil
	}
	return unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0)
}
