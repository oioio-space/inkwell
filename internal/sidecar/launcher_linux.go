package sidecar

import (
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const sandboxMode = "--inkwell-internal-sandbox-v1"

type sandboxConfig struct {
	RequireDeath bool
	Cgroup       bool
	ReportFD     int
	Libraries    []string
	WorkingDir   string
	RuntimeDir   string
	Memory, File uint64
	Directory    string
	Names        []string
	MountBytes   int64
	ReadPaths    []string
	Executable   string
	Dynamic      bool
	Probe        bool
	Overlay      bool
}

// init dispatches only our explicit hidden child mode, before application startup.
// The locked thread carries Landlock and capabilities through exec; no untrusted
// code runs in this Go runtime before that exec.
func init() {
	if len(os.Args) < 3 || os.Args[1] != sandboxMode {
		return
	}
	runtime.LockOSThread()
	var c sandboxConfig
	err := json.Unmarshal([]byte(os.Args[2]), &c)
	if err == nil {
		err = sandboxExec(c, os.Args[3:])
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "sidecar: sandbox:", err)
		if c.Probe && definitiveRefusal(err) {
			os.Exit(probeUnsupportedExit)
		}
		os.Exit(125)
	}
	os.Exit(0)
}

func sandboxPrefix(c sandboxConfig) (string, []string, error) {
	c.RuntimeDir = os.Getenv("XDG_RUNTIME_DIR")
	if c.RuntimeDir == "" {
		c.RuntimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	data, err := json.Marshal(c)
	return "/proc/self/exe", []string{sandboxMode, string(data)}, err
}

func sandboxExec(c sandboxConfig, args []string) error {
	if err := readReportFD(&c); err != nil {
		return err
	}
	if expected := os.Getenv(expectedCgroupEnv); expected != "" {
		c.Cgroup = true
		actual, err := selfCgroup()
		if err != nil || actual != expected {
			if c.ReportFD >= 3 {
				_, _ = unix.Write(c.ReportFD, []byte{bootstrapRefusalReport})
			}
			return errors.New("bootstrap cgroup membership refused")
		}
		_ = os.Unsetenv(expectedCgroupEnv)
	}
	if parent := os.Getenv(expectedParentEnv); parent != "" {
		if err := armNamespaceParentDeath(parent); err != nil {
			return err
		}
		_ = os.Unsetenv(expectedParentEnv)
	}
	if c.Overlay {
		if err := mountIPC(c); err != nil {
			return err
		}
	}
	if c.MountBytes > 0 {
		if err := mountOutput(c); err != nil {
			return err
		}
	}
	if c.Probe {
		return nil
	}
	if err := unix.CloseRange(3, ^uint(0), unix.CLOSE_RANGE_CLOEXEC); err != nil {
		return err
	}
	if unix.Getpid() == 1 {
		hostProc, err := os.OpenRoot("/proc")
		if err != nil {
			return err
		}
		defer func() { _ = hostProc.Close() }()
		if err := unix.Mount("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
			return err
		}
		c.Overlay, c.MountBytes = false, 0
		return sandboxInitWithProc(c, args, hostProc)
	}
	if len(args) == 0 {
		return errors.New("missing target")
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if err := dropCapabilities(); err != nil {
		return err
	}
	if c.WorkingDir == "" {
		return errors.New("explicit project directory required")
	}
	if err := restrictReadsConfig(&c, args[0]); err != nil {
		return err
	}
	if err := restrictOutput(c); err != nil {
		return fmt.Errorf("mandatory Landlock unavailable; refusing peer document: %w", err)
	}

	return limitAndExec(c, args)
}

// limitAndExec applies the resource limits and seccomp, then execs args[0].
func limitAndExec(c sandboxConfig, args []string) error {
	// Everything that allocates happens before RLIMIT_AS: this Go runtime
	// shares the limit until exec, and a heap growth past it is a fatal
	// "runtime: out of memory" that looks like a typst limit hit (TYP-16
	// failed about 2% of runs). GC is off for the same reason, and the exec
	// is a raw execve on pointers prepared here (syscall.Exec allocates).
	prog, err := seccompProgram()
	if err != nil {
		return fmt.Errorf("mandatory seccomp unavailable; refusing peer document: %w", err)
	}
	path, err := unix.BytePtrFromString(args[0])
	if err != nil {
		return err
	}
	argv, err := syscall.SlicePtrFromStrings(args)
	if err != nil {
		return err
	}
	envp, err := syscall.SlicePtrFromStrings(os.Environ())
	if err != nil {
		return err
	}
	// execve(2) resets caught signals to their default action but keeps ignored
	// ones ignored: the target starts with SIGXFSZ ignored, so a write past
	// RLIMIT_FSIZE fails with EFBIG instead of killing it, and with no other
	// signal ignored, whatever started the app (nohup ignores SIGHUP). The byte
	// cap stays the kernel's (ADR 0042 A6).
	signal.Ignore(syscall.SIGXFSZ)
	if err := resetIgnoredSignals(); err != nil {
		return err
	}
	type rlimit struct {
		resource int
		value    unix.Rlimit
	}
	limits := make([]rlimit, 0, 3) // built now: Setrlimit's argument must not be allocated after RLIMIT_AS
	// RLIMIT_CORE {1,1} is the kernel's pipe-helper sentinel (fs/coredump.c,
	// coredump_pipe): a core-class death starts no crash helper such as
	// systemd-coredump, and 1 byte is below a core file's minimum. Max = 1, so
	// the target cannot raise it. Socket core_pattern modes ignore it (RS1).
	limits = append(limits, rlimit{unix.RLIMIT_CORE, unix.Rlimit{Cur: 1, Max: 1}})
	if c.Memory > 0 {
		limits = append(limits, rlimit{unix.RLIMIT_AS, unix.Rlimit{Cur: c.Memory, Max: c.Memory}})
	}
	if c.File > 0 || c.Directory != "" {
		limits = append(limits, rlimit{unix.RLIMIT_FSIZE, unix.Rlimit{Cur: c.File, Max: c.File}})
	}
	debug.SetGCPercent(-1)
	for i := range limits {
		if err := unix.Setrlimit(limits[i].resource, &limits[i].value); err != nil {
			return err
		}
	}
	if err := installSeccompProgram(prog); err != nil {
		return fmt.Errorf("mandatory seccomp unavailable; refusing peer document: %w", err)
	}
	_, _, errno := unix.RawSyscall(unix.SYS_EXECVE, uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&argv[0])), uintptr(unsafe.Pointer(&envp[0]))) // #nosec G103 -- NUL-terminated pointers prepared above; execve returns only on failure.
	runtime.KeepAlive(path)
	runtime.KeepAlive(argv)
	runtime.KeepAlive(envp)
	return errno
}

// resetIgnoredSignals restores the default action of every signal ignored in
// this process except SIGXFSZ, so that exec gives the target no other ignored
// signal.
func resetIgnoredSignals() error {
	for sig := uintptr(1); sig <= 64; sig++ {
		if sig == uintptr(unix.SIGXFSZ) || sig == uintptr(unix.SIGKILL) || sig == uintptr(unix.SIGSTOP) {
			continue
		}
		// The kernel's struct sigaction: handler, flags, restorer, mask.
		var action [4]uint64
		if _, _, errno := unix.RawSyscall6(unix.SYS_RT_SIGACTION, sig, 0, uintptr(unsafe.Pointer(&action)), 8, 0, 0); errno != 0 { // #nosec G103 -- the kernel fills the old action synchronously.
			return fmt.Errorf("sidecar: read the action of signal %d: %w", sig, errno)
		}
		if action[0] != 1 { // SIG_IGN
			continue
		}
		// All zero: SIG_DFL.
		action = [4]uint64{}
		if _, _, errno := unix.RawSyscall6(unix.SYS_RT_SIGACTION, sig, uintptr(unsafe.Pointer(&action)), 0, 8, 0, 0); errno != 0 { // #nosec G103 -- the kernel copies the action synchronously.
			return fmt.Errorf("sidecar: reset signal %d: %w", sig, errno)
		}
	}
	return nil
}

func mountOutput(c sandboxConfig) error {
	size := c.MountBytes / int64(os.Getpagesize()) * int64(os.Getpagesize())
	if size <= 0 {
		return errors.New("sidecar: tmpfs size must be at least one page")
	}
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return err
	}
	options := fmt.Sprintf("size=%d,nr_inodes=%d,mode=0700", size, 2*len(c.Names)+1)
	if err := unix.Mount("tmpfs", c.Directory, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, options); err != nil {
		return err
	}
	for _, name := range c.Names {
		f, err := os.OpenFile(filepath.Join(c.Directory, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- private tmpfs and validated simple output name, O_EXCL.
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	if c.Probe {
		return nil
	}
	if err := unix.Chdir(c.WorkingDir); err != nil {
		return err
	}
	ready, proceed := os.NewFile(3, "ready"), os.NewFile(4, "proceed")
	defer func() { _ = ready.Close(); _ = proceed.Close() }()
	if _, err := ready.Write([]byte("r")); err != nil {
		return err
	}
	var b [1]byte
	_, err := io.ReadFull(proceed, b[:])
	return err
}

func mountIPC(c sandboxConfig) error {
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return err
	}
	for _, path := range []string{c.RuntimeDir, "/tmp/.X11-unix"} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := unix.Mount("tmpfs", path, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "size=4096,nr_inodes=1,mode=0700"); err != nil {
			return err
		}
	}
	return nil
}

func dropCapabilities() error {
	// An ordinary caller lacks CAP_SETPCAP: its bounding set cannot be dropped,
	// but no_new_privs and empty effective/permitted/inheritable sets still apply.
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{}
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return err
	}
	if data[0].Effective&(1<<unix.CAP_SETPCAP) != 0 {
		for capID := uintptr(0); ; capID++ {
			if _, err := unix.PrctlRetInt(unix.PR_CAPBSET_READ, capID, 0, 0, 0); errors.Is(err, unix.EINVAL) {
				break
			} else if err != nil {
				return err
			}
			if err := unix.Prctl(unix.PR_CAPBSET_DROP, capID, 0, 0, 0); err != nil {
				return err
			}
		}
		// NOROOT and NO_SETUID_FIXUP, each locked; root cannot regain caps at exec.
		if err := unix.Prctl(unix.PR_SET_SECUREBITS, 15, 0, 0, 0); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_CLEAR_ALL, 0, 0, 0); err != nil {
		return err
	}
	data = [2]unix.CapUserData{}
	return unix.Capset(&hdr, &data[0])
}

// Base filesystem write policy requires ABI >= 3; newer ABIs add restrictions.
const outputAccess = uint64(unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE | unix.LANDLOCK_ACCESS_FS_MAKE_CHAR | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_MAKE_REG | unix.LANDLOCK_ACCESS_FS_MAKE_SOCK | unix.LANDLOCK_ACCESS_FS_MAKE_FIFO | unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK | unix.LANDLOCK_ACCESS_FS_MAKE_SYM | unix.LANDLOCK_ACCESS_FS_REFER | unix.LANDLOCK_ACCESS_FS_TRUNCATE)

// UDP rights added by ABI 10; x/sys v0.49 does not yet name them.
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/landlock.h
const (
	landlockAccessNetBindUDP        = 1 << 2
	landlockAccessNetConnectSendUDP = 1 << 3
)

// landlockPolicy uses only fields and rights supported by the probed kernel.
// https://docs.kernel.org/userspace-api/landlock.html (ABI compatibility).
func landlockPolicy(abi uintptr) (unix.LandlockRulesetAttr, uintptr) {
	attr := unix.LandlockRulesetAttr{Handled_access_fs: outputAccess | readAccess}
	size := uintptr(8)
	if abi >= 4 {
		attr.Handled_access_net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
		size = 16
	}
	if abi >= 10 {
		attr.Handled_access_net |= landlockAccessNetBindUDP | landlockAccessNetConnectSendUDP
	}
	if abi >= 5 {
		attr.Handled_access_fs |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}
	if abi >= 6 {
		attr.Scoped = unix.LANDLOCK_SCOPE_SIGNAL | unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET
		size = 24
	}
	if abi >= 9 {
		attr.Handled_access_fs |= unix.LANDLOCK_ACCESS_FS_RESOLVE_UNIX
	}
	return attr, size
}

func restrictOutput(c sandboxConfig) error {
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return errno
	}
	if abi < 3 {
		return errors.New("sidecar: directory bound requires Landlock ABI >= 3")
	}
	attr, size := landlockPolicy(abi)
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), size, 0) // #nosec G103 -- kernel reads this ABI-sized struct synchronously.
	if errno != 0 {
		return errno
	}
	defer func() { _ = unix.Close(int(fd)) }()
	paths := []string{"/dev/null"}
	for _, name := range c.Names {
		paths = append(paths, filepath.Join(c.Directory, name))
	}
	for _, path := range paths {
		pathFD, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		rule := unix.LandlockPathBeneathAttr{Allowed_access: unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_TRUNCATE, Parent_fd: int32(pathFD)} // #nosec G115 -- Linux file descriptors are nonnegative signed ints fitting int32.
		_, _, errno = unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, fd, unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0)                 // #nosec G103 -- kernel reads the path rule synchronously.
		_ = unix.Close(pathFD)
		if errno != 0 {
			return errno
		}
	}
	for _, path := range c.ReadPaths {
		rights := uint64(unix.LANDLOCK_ACCESS_FS_READ_FILE)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			rights |= unix.LANDLOCK_ACCESS_FS_READ_DIR
		}
		if path == c.Executable || slices.Contains(c.Libraries, path) {
			rights |= unix.LANDLOCK_ACCESS_FS_EXECUTE
		}
		pathFD, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		rule := unix.LandlockPathBeneathAttr{Allowed_access: rights, Parent_fd: int32(pathFD)}                                                   // #nosec G115 -- Linux fd fits int32.
		_, _, ruleErr := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, fd, unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0) // #nosec G103 -- synchronous kernel copy.
		_ = unix.Close(pathFD)
		if ruleErr != 0 {
			return ruleErr
		}
	}
	_, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

const readAccess = uint64(unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR | unix.LANDLOCK_ACCESS_FS_EXECUTE)

func restrictReadsConfig(c *sandboxConfig, target string) error {
	path, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	c.Executable = path
	if len(c.ReadPaths) == 0 {
		c.ReadPaths = append(c.ReadPaths, "/usr/share/fonts", "/usr/local/share/fonts")
	}
	c.ReadPaths = append(c.ReadPaths, []string{c.WorkingDir, path, "/dev/null", "/dev/urandom", "/proc/self", "/proc/thread-self", "/proc/meminfo", "/proc/cpuinfo", "/etc/ssl/certs", "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"}...)
	if c.Directory != "" {
		c.ReadPaths = append(c.ReadPaths, c.Directory)
	}
	binary, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = binary.Close() }()
	for _, prog := range binary.Progs {
		if prog.Type == elf.PT_INTERP {
			c.Dynamic = true
		}
	}
	if c.Dynamic {
		c.Libraries, err = dynamicLibraries(path, make(map[string]bool))
		if err != nil {
			return err
		}
		c.ReadPaths = append(c.ReadPaths, c.Libraries...)
		c.ReadPaths = append(c.ReadPaths, "/etc/ld.so.cache")
	}
	return nil
}

// PID 1 remains a small trusted init; only the exec child handles documents.
// pidfd receives the child's pidfd, for its core-dump record.
func sandboxInitCommand(c sandboxConfig, args []string, pidfd *int) (*exec.Cmd, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("/proc/self/exe", append([]string{sandboxMode, string(data)}, args...)...) // #nosec G204 G702 -- fixed own launcher path and serialized policy.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{PidFD: pidfd}
	return cmd, nil
}

func sandboxInit(c sandboxConfig, args []string) error {
	root, err := os.OpenRoot("/proc")
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return sandboxInitWithProc(c, args, root)
}

func sandboxInitWithProc(c sandboxConfig, args []string, hostProc *os.Root) error {
	status, err := runSandboxInit(c, args, hostProc)
	if err != nil {
		return err
	}
	os.Exit(status)
	return nil
}

func runSandboxInit(c sandboxConfig, args []string, hostProc *os.Root) (int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pidfd := -1
	cmd, err := sandboxInitCommand(c, args, &pidfd)
	if err != nil {
		return 0, err
	}
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go func() {
		for sig := range signals {
			_ = cmd.Process.Signal(sig)
		}
	}()
	death, err := targetHostIdentity(hostProc, cmd.Process.Pid, c.RequireDeath)
	if err != nil {
		return 0, err
	}

	return reapSandboxTarget(c, cmd, hostProc, death, pidfd)
}

func reapSandboxTarget(c sandboxConfig, cmd *exec.Cmd, hostProc *os.Root, death *SandboxDeath, pidfd int) (int, error) {
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_ALL, 0, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		waited := siginfoChildPID(&info)
		if waited == cmd.Process.Pid {
			death, err = refreshTargetIdentity(hostProc, death, c.RequireDeath)
			if err != nil {
				return 0, err
			}
		}
		core := coreDumpOf(pidfd)
		var status unix.WaitStatus
		pid, err := unix.Wait4(waited, &status, 0, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if pid == cmd.Process.Pid {

			if status.Signaled() && c.ReportFD >= 3 && death != nil {
				sig := status.Signal()
				if sig < 1 || sig > 64 || core > CoreDumpStarted {
					return 0, errors.New("invalid signal report")
				}
				b, _ := json.Marshal(death)
				_, _ = unix.Write(c.ReportFD, append([]byte{byte(sig), byte(core)}, b...))
				return 128 + int(status.Signal()), nil
			}
			return sandboxExitStatus(status, c.ReportFD, core), nil
		}
	}
}

func refreshTargetIdentity(hostProc *os.Root, death *SandboxDeath, required bool) (*SandboxDeath, error) {
	// Read identity after exec and death, before reap. Never infer comm
	// from the executable basename: the target can change its own name.
	if hostProc != nil && death != nil {
		b, err := hostProc.ReadFile(strconv.Itoa(death.PID) + "/status")
		if err != nil {
			if required {
				return nil, err
			}
			death = nil
		} else {
			d := identityFromStatus(b)
			death = &d
		}
	}
	return death, nil
}

// Identity is evidence only; CONFIG_PROC_CHILDREN is optional for normal runs.
func targetHostIdentity(root *os.Root, pid int, required bool) (*SandboxDeath, error) {
	if root != nil {
		b, err := root.ReadFile("thread-self/children")
		if err == nil {
			for host := range strings.FieldsSeq(string(b)) {
				status, err := root.ReadFile(host + "/status")
				if err != nil {
					continue
				}
				d := identityFromStatus(status)
				if len(d.NSPID) > 0 && d.NSPID[len(d.NSPID)-1] == pid {
					return &d, nil
				}
			}
		}
	}
	if required {
		return nil, errors.New("target host identity unavailable")
	}
	return nil, nil
}

// Resolve DT_NEEDED without executing ldd or trusting environment search paths.
func dynamicLibraries(path string, seen map[string]bool) ([]string, error) {
	if seen[path] {
		return nil, nil
	}
	seen[path] = true
	binary, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = binary.Close() }()
	var paths []string
	for _, prog := range binary.Progs {
		if prog.Type == elf.PT_INTERP {
			data, err := io.ReadAll(prog.Open())
			if err != nil {
				return nil, err
			}
			paths = append(paths, string(data[:len(data)-1]))
		}
	}
	names, err := binary.ImportedLibraries()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if filepath.Base(name) != name || name == "." || name == ".." {
			return nil, fmt.Errorf("invalid ELF library name %q", name)
		}
		found := false
		for _, dir := range []string{"/lib64", "/lib", "/usr/lib64", "/usr/lib", "/lib/x86_64-linux-gnu", "/lib/aarch64-linux-gnu"} {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err != nil { // #nosec G703 -- validated basename joined to fixed system library directory; ELF comes from verified sidecar.
				continue
			}
			paths = append(paths, candidate)
			dependencies, err := dynamicLibraries(candidate, seen)
			if err != nil {
				return nil, err
			}
			paths = append(paths, dependencies...)
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("missing dynamic dependency %s", name)
		}
	}
	return paths, nil
}

func readReportFD(c *sandboxConfig) error {
	if value := os.Getenv(signalReportEnv); value != "" {
		fd, err := strconv.Atoi(value)
		if err != nil || fd < 3 {
			return errors.New("invalid signal report descriptor")
		}
		c.ReportFD = fd
		if err := os.Unsetenv(signalReportEnv); err != nil {
			return err
		}
	}

	return nil
}

// sandboxExitStatus preserves signal death, and whether it started a core
// dump, across the PID namespace boundary.
func sandboxExitStatus(status unix.WaitStatus, reportFD int, core CoreDump) int {
	if status.Signaled() {
		if reportFD >= 3 {
			_, _ = unix.Write(reportFD, []byte{byte(status.Signal()), byte(core)}) // #nosec G115 -- Linux signal numbers are 1..64, so they fit in a byte
		}
		return 128 + int(status.Signal())
	}
	return status.ExitStatus()
}

// Read the original host procfs before mounting the namespace procfs: getppid
// returns zero for an external parent in a new PID namespace and cannot check
// the parent-death race. The kernel's PPid field can.
func armNamespaceParentDeath(expected string) error {
	if err := unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0); err != nil {
		return err
	}
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if p, ok := strings.CutPrefix(line, "PPid:"); ok {
			if strings.TrimSpace(p) == expected {
				return nil
			}
			return errors.New("launching parent died")
		}
	}
	return errors.New("parent identity unavailable")
}
