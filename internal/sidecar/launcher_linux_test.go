package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TestWithLimitsExactBytes checks the policy passed to the pre-exec launcher.
func TestWithLimitsExactBytes(t *testing.T) {
	path, args, err := withLimits(Spec{Path: "target", Args: []string{"a"}})
	if err != nil || path != "/proc/self/exe" || len(args) != 4 {
		t.Fatalf("unconfined unset limits: %q %q %v", path, args, err)
	}
	for _, value := range []uint64{1, 512, 513, 1024, 1025} {
		s := Spec{Path: "target", Args: []string{"a", "b"}, MemoryLimit: value, FileSizeLimit: value}
		path, args, err := withLimits(s)
		if err != nil {
			t.Fatal(err)
		}
		var c sandboxConfig
		if path == "" || len(args) != 5 || args[0] != sandboxMode {
			t.Fatalf("bad launcher: %q %q", path, args)
		}
		if err := json.Unmarshal([]byte(args[1]), &c); err != nil {
			t.Fatal(err)
		}
		if c.Memory != value || c.File != value || !slices.Equal(args[2:], []string{"target", "a", "b"}) {
			t.Fatalf("wrong limits: %+v %q", c, args)
		}
	}
}

// TestSandboxPrivileges checks kernel state in the final target, after exec.
func TestSandboxPrivileges(t *testing.T) {
	s := spec("sandboxstatus")
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = t.TempDir(), 8192, []string{"out"}
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 {
		t.Fatalf("sandbox: %v %+v", err, out)
	}
	for _, field := range []string{"NoNewPrivs:\t1", "CapInh:\t0000000000000000", "CapPrm:\t0000000000000000", "CapEff:\t0000000000000000", "CapAmb:\t0000000000000000"} {
		if !strings.Contains(string(out.Stdout), field) {
			t.Errorf("missing %s in %s", field, out.Stdout)
		}
	}
	if mountSupported(t) {
		for line := range strings.SplitSeq(string(out.Stdout), "\n") {
			if strings.HasPrefix(line, "NSpid:") && strings.HasSuffix(line, "\t1") {
				t.Errorf("workload incorrectly runs as PID 1: %s", line)
			}
		}
	}
	if mountSupported(t) && !strings.Contains(string(out.Stdout), "CapBnd:\t0000000000000000") {
		t.Errorf("bounding set retained: %s", out.Stdout)
	}
}

// The helper is the final exec target, so checks run after all confinement.
func init() {
	if os.Getenv("INKWELL_ATTACK") == "" {
		return
	}
	err := confinementAttack()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if os.Getenv("INKWELL_ATTACK") == "seccomp" {
		data, readErr := os.ReadFile("/proc/self/status")
		if readErr != nil {
			os.Exit(1)
		}
		_, _ = os.Stdout.Write(data)
		_, _ = fmt.Fprintln(os.Stdout, "seccomp denials verified")
	}
	os.Exit(0)
}

func confinementAttack() error {
	switch os.Getenv("INKWELL_ATTACK") {
	case "init-report":
		fd, err := strconv.Atoi(os.Getenv(signalReportEnv))
		if err != nil {
			return err
		}
		if err := os.Unsetenv(signalReportEnv); err != nil {
			return err
		}
		if err := os.Setenv("INKWELL_ATTACK", "xfsz"); err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return sandboxInit(sandboxConfig{WorkingDir: cwd, ReportFD: fd}, []string{os.Args[0]})
	case "xfsz":
		// Install the kernel default action, bypassing Go's SIGXFSZ handler.
		action := [4]uint64{}
		if _, _, errno := unix.RawSyscall6(unix.SYS_RT_SIGACTION, uintptr(unix.SIGXFSZ), uintptr(unsafe.Pointer(&action)), 0, 8, 0, 0); errno != 0 {
			return errno
		}
		return unix.Tgkill(unix.Getpid(), unix.Gettid(), unix.SIGXFSZ)

	case "fds":
		if _, err := unix.FcntlInt(3, unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
			return fmt.Errorf("inherited fd survived: %w", err)
		}
		return nil
	case "sentinel":
		for _, path := range []string{os.Getenv("INKWELL_SENTINEL"), "/proc/self/root" + os.Getenv("INKWELL_SENTINEL")} {
			if _, err := os.ReadFile(path); !errors.Is(err, os.ErrPermission) {
				return fmt.Errorf("sentinel %s: %w", path, err)
			}
		}
		return nil
	case "home":
		_, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".bash_history"))
		if !errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("home readable: %w", err)
		}
		return nil
	case "shm":
		id, err := strconv.Atoi(os.Getenv("INKWELL_SHMID"))
		if err != nil {
			return err
		}
		_, _, errno := unix.RawSyscall(unix.SYS_SHMAT, uintptr(id), 0, 0) // #nosec G115 -- test shm id.
		if errno != unix.EPERM {
			return fmt.Errorf("shmat: %w", errno)
		}
		return nil
	case "unknown":
		_, _, errno := unix.RawSyscall(10000, 0, 0, 0)
		if errno != unix.EPERM {
			return fmt.Errorf("unknown syscall: %w", errno)
		}
		return nil
	case "controls":
		if _, _, errno := unix.RawSyscall(unix.SYS_FCNTL, 1, unix.F_SETFL, unix.O_ASYNC); errno != unix.EPERM {
			return fmt.Errorf("O_ASYNC: %w", errno)
		}

		for _, command := range []uintptr{unix.F_SETOWN, unix.F_SETOWN_EX, unix.F_SETSIG} {
			_, _, errno := unix.RawSyscall(unix.SYS_FCNTL, 1, command, 0)
			if errno != unix.EPERM {
				return fmt.Errorf("fcntl %d: %w", command, errno)
			}
		}
		for _, command := range []uintptr{0x8901, 0x8902, unix.TIOCSTI, unix.TIOCLINUX} {
			_, _, errno := unix.RawSyscall(unix.SYS_IOCTL, 1, command, 0)
			if errno != unix.EPERM {
				return fmt.Errorf("ioctl %d: %w", command, errno)
			}
		}
		return nil

	case "arch":
		runtime.LockOSThread()
		f := seccompFilter(0, 0)                                   // No valid audit architecture can match.
		prog := unix.SockFprog{Len: uint16(len(f)), Filter: &f[0]} // #nosec G115 -- small fixed test filter.
		if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(&prog)), 0, 0); err != nil {
			return err
		} // #nosec G103 -- kernel copies test filter.
		_ = unix.Getpid()
		return errors.New("audit architecture mismatch survived")
	case "x32":
		_, _, _ = unix.RawSyscall(0x40000000|unix.SYS_GETPID, 0, 0, 0)
		return errors.New("x32 syscall survived")
	}

	if os.Getenv("INKWELL_ATTACK") == "seccomp" {
		for _, nr := range []uintptr{
			unix.SYS_SOCKET, unix.SYS_PTRACE, unix.SYS_PROCESS_VM_READV, unix.SYS_PROCESS_VM_WRITEV,
			unix.SYS_KEXEC_LOAD, unix.SYS_KEXEC_FILE_LOAD, unix.SYS_BPF, unix.SYS_KEYCTL, unix.SYS_ADD_KEY,
			unix.SYS_MOUNT, unix.SYS_UMOUNT2, unix.SYS_UNSHARE, unix.SYS_SETNS, unix.SYS_PERSONALITY,
			unix.SYS_FSOPEN, unix.SYS_FSCONFIG, unix.SYS_FSMOUNT, unix.SYS_FSPICK,
			unix.SYS_OPEN_TREE, unix.SYS_MOVE_MOUNT, unix.SYS_MOUNT_SETATTR, unix.SYS_PIDFD_GETFD,
			unix.SYS_KILL, unix.SYS_TKILL, unix.SYS_PIDFD_SEND_SIGNAL, unix.SYS_IO_URING_SETUP,
		} {
			_, _, errno := unix.RawSyscall6(nr, ^uintptr(0), ^uintptr(0), ^uintptr(0), 0, 0, 0)
			if errno != unix.EPERM {
				return fmt.Errorf("syscall %d: got %w, want EPERM", nr, errno)
			}
		}
		if _, _, errno := unix.RawSyscall6(unix.SYS_CLONE, unix.CLONE_NEWUSER, 0, 0, 0, 0, 0); errno != unix.EPERM {
			return fmt.Errorf("namespace clone: %w", errno)
		}
		if _, _, errno := unix.RawSyscall6(unix.SYS_CLONE, unix.CLONE_PARENT, 0, 0, 0, 0, 0); errno != unix.EPERM {
			return fmt.Errorf("CLONE_PARENT: %w", errno)
		}
		if _, _, errno := unix.RawSyscall6(unix.SYS_CLONE3, 0, 0, 0, 0, 0, 0); errno != unix.ENOSYS {
			return fmt.Errorf("clone3: %w", errno)
		}
		for _, family := range []int{unix.AF_UNIX, unix.AF_INET, unix.AF_INET6, unix.AF_NETLINK, unix.AF_PACKET} {
			fd, err := unix.Socket(family, unix.SOCK_STREAM, 0)
			if err == nil {
				_ = unix.Close(fd)
			}
			if !errors.Is(err, unix.EPERM) {
				return fmt.Errorf("socket family %d: %w", family, err)
			}
		}
		pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
		if err != nil {
			return err
		}
		_ = unix.Close(pair[0])
		_ = unix.Close(pair[1])
		if pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0); !errors.Is(err, unix.EPERM) {
			_ = unix.Close(pair[0])
			_ = unix.Close(pair[1])
			return fmt.Errorf("datagram socketpair: %w", err)
		}
		return nil
	}
	if os.Getenv("INKWELL_ATTACK") == "overlays" {
		for _, path := range []string{filepath.Join(os.Getenv("INKWELL_RUNTIME"), "sentinel"), os.Getenv("INKWELL_X11")} {
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("host sentinel visible: %s: %w", path, err)
			}
			var stat unix.Statfs_t
			if err := unix.Statfs(filepath.Dir(path), &stat); err != nil {
				return err
			}
			if stat.Type != unix.TMPFS_MAGIC {
				return fmt.Errorf("overlay is not tmpfs: %s", path)
			}
		}
		return nil
	}
	pid, err := strconv.Atoi(os.Getenv("INKWELL_SIBLING"))
	if err != nil {
		return err
	}
	switch os.Getenv("INKWELL_ATTACK") {
	case "init-report":
		fd, err := strconv.Atoi(os.Getenv(signalReportEnv))
		if err != nil {
			return err
		}
		if err := os.Unsetenv(signalReportEnv); err != nil {
			return err
		}
		if err := os.Setenv("INKWELL_ATTACK", "xfsz"); err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return sandboxInit(sandboxConfig{WorkingDir: cwd, ReportFD: fd}, []string{os.Args[0]})
	case "xfsz":
		// Install the kernel default action, bypassing Go's SIGXFSZ handler.
		action := [4]uint64{}
		if _, _, errno := unix.RawSyscall6(unix.SYS_RT_SIGACTION, uintptr(unix.SIGXFSZ), uintptr(unsafe.Pointer(&action)), 0, 8, 0, 0); errno != 0 {
			return errno
		}
		return unix.Tgkill(unix.Getpid(), unix.Gettid(), unix.SIGXFSZ)

	case "signal":
		if err := unix.Kill(pid, 0); !errors.Is(err, unix.EPERM) {
			return fmt.Errorf("kill sibling: %w", err)
		}
	case "bus":
		fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
		if errors.Is(err, unix.EPERM) {
			return nil
		}
		if err != nil {
			return err
		}
		defer func() { _ = unix.Close(fd) }()
		if err := unix.Connect(fd, &unix.SockaddrUnix{Name: os.Getenv("INKWELL_BUS")}); !errors.Is(err, unix.EACCES) {
			return fmt.Errorf("connect bus: %w", err)
		}
	}
	return nil
}

func TestSandboxExternalIPC(t *testing.T) {
	for _, name := range []string{"signal", "bus"} {
		t.Run(name, func(t *testing.T) {
			sibling := exec.Command(os.Args[0], "-test.run=^$")
			// Existing helper sleeps until cleanup; it is outside the sandbox domain.
			sibling.Env = helperEnv("sleep")
			if err := sibling.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = sibling.Process.Kill(); _ = sibling.Wait() }()
			if err := unix.Kill(sibling.Process.Pid, 0); err != nil {
				t.Fatal(err)
			}
			runtimeDir := t.TempDir()
			bus := runtimeDir + "/bus"
			listener, err := net.Listen("unix", bus)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = listener.Close() }()
			connection, err := net.Dial("unix", bus)
			if err != nil {
				t.Fatal(err)
			}
			_ = connection.Close()
			path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir(), Directory: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			args = append(args, os.Args[0], "-test.run=^TestConfinementHelper$")
			cmd := exec.Command(path, args...)
			cmd.Env = []string{"INKWELL_ATTACK=" + name, fmt.Sprintf("INKWELL_SIBLING=%d", sibling.Process.Pid), "INKWELL_BUS=" + bus, "XDG_RUNTIME_DIR=" + runtimeDir}
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("sandbox: %v %s", err, output)
			}
		})
	}
}

func TestDirectoryZeroFileCap(t *testing.T) {
	s := Spec{WatchDir: t.TempDir(), MaxDirBytes: 1, WriteFiles: []string{"a", "b"}}
	_, prefix, err := directoryLimits(s)
	if err != nil {
		t.Fatal(err)
	}
	var config sandboxConfig
	if err := json.Unmarshal([]byte(prefix[2]), &config); err != nil {
		t.Fatal(err)
	}
	if config.File != 0 || config.Directory == "" {
		t.Fatalf("zero cap not passed to directory launcher: %+v", config)
	}
}

func TestMountZeroSizeRejected(t *testing.T) {
	for _, size := range []int64{0, 1, int64(os.Getpagesize() - 1)} {
		if err := mountOutput(sandboxConfig{MountBytes: size}); err == nil || !strings.Contains(err.Error(), "at least one page") {
			t.Fatalf("size %d: %v", size, err)
		}
	}
}

func TestLandlockPolicy(t *testing.T) {
	for abi := uintptr(3); abi <= 12; abi++ {
		attr, size := landlockPolicy(abi)
		var wantNet uint64
		if abi >= 4 {
			wantNet = 3
		}
		if abi >= 10 {
			wantNet = 15
		}
		if attr.Handled_access_net != wantNet {
			t.Fatalf("ABI %d: network rights %x, want %x", abi, attr.Handled_access_net, wantNet)
		}
		wantSize := uintptr(8)
		if abi >= 4 {
			wantSize = 16
		}
		if abi >= 6 {
			wantSize = 24
		}
		if size != wantSize || (attr.Handled_access_net != 0) != (abi >= 4) || (attr.Scoped != 0) != (abi >= 6) || (attr.Handled_access_fs&unix.LANDLOCK_ACCESS_FS_IOCTL_DEV != 0) != (abi >= 5) || (attr.Handled_access_fs&unix.LANDLOCK_ACCESS_FS_RESOLVE_UNIX != 0) != (abi >= 9) {
			t.Fatalf("ABI %d: %+v size %d", abi, attr, size)
		}
	}
}

func TestSandboxZeroFileCap(t *testing.T) {
	dir := t.TempDir()
	output := dir + "/out"
	if err := os.WriteFile(output, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir(), Directory: dir, Names: []string{"out"}})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, append(args, os.Args[0])...)
	cmd.Env = helperEnv("writefile:" + output + ",1")
	if data, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("zero cap allowed write: %s", data)
	}
	info, err := os.Stat(output)
	if err != nil || info.Size() != 0 {
		t.Fatalf("zero cap output: %v %v", info, err)
	}
}

// TestEveryInvocationConfined covers unlimited query/eval, limited commands,
// bounded compiles and the long-lived stdio supervisor.
func TestEveryInvocationConfined(t *testing.T) {
	for _, mode := range []string{"unlimited", "limited", "bounded", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			s := spec("sandboxstatus")
			s.Env = append(s.Env, "INKWELL_ATTACK=seccomp")
			switch mode {
			case "limited":
				s.FileSizeLimit = 4096
			case "bounded":
				s.WatchDir, s.MaxDirBytes, s.WriteFiles = t.TempDir(), 8192, []string{"out"}
			case "fallback":
				s.isolationProbe = func() (bool, error) { return false, nil }
			}
			out, err := Run(t.Context(), s)
			if err != nil || out.ExitCode != 0 {
				t.Fatalf("%v: %+v", err, out)
			}
			assertConfined(t, out.Stdout)
		})
	}
	var output bytes.Buffer
	sup := NewSupervisor(SupervisorConfig{Command: func(_ context.Context) (*exec.Cmd, error) {
		cmd := helperCmd("sandboxstatus")
		cmd.Env = append(cmd.Env, "INKWELL_ATTACK=seccomp")
		cmd.Stdout = &output
		return cmd, nil
	}})
	_, _ = sup.runOnce(t.Context())
	assertConfined(t, output.Bytes())
}

func assertConfined(t *testing.T, output []byte) {
	t.Helper()
	for _, marker := range []string{"NoNewPrivs:\t1", "Seccomp:\t2", "seccomp denials verified"} {
		if !strings.Contains(string(output), marker) {
			t.Fatalf("missing %q: %s", marker, output)
		}
	}
}

func TestIPCOverlays(t *testing.T) {
	if !mountSupported(t) {
		t.Fatal("cannot verify IPC overlays: user/mount/PID namespaces unavailable")
	}
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	x11 := "/tmp/.X11-unix"
	if err := os.MkdirAll(x11, 0o700); err != nil {
		t.Fatal(err)
	}
	marker, err := os.CreateTemp(x11, "inkwell-overlay-test-")
	if err != nil {
		t.Fatal(err)
	}
	name := marker.Name()
	_ = marker.Close()
	defer func() { _ = os.Remove(name) }()
	if err := os.WriteFile(filepath.Join(runtimeDir, "sentinel"), []byte("host"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, bounded := range []bool{false, true} {
		s := spec("echo")
		if bounded {
			s.WatchDir, s.MaxDirBytes, s.WriteFiles = t.TempDir(), 8192, []string{"out"}
		}
		s.Env = []string{"INKWELL_ATTACK=overlays", "INKWELL_RUNTIME=" + runtimeDir, "INKWELL_X11=" + name}
		out, err := Run(t.Context(), s)
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("%v: %+v stderr=%s", err, out, out.Stderr)
		}
	}
}

// This invokes the launcher without user, mount, PID or network namespaces.
// The denial probe must still succeed, regardless of Landlock's ABI scopes.
func TestSeccompWithoutNamespaces(t *testing.T) {
	path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, append(args, os.Args[0])...)
	cmd.Env = []string{"INKWELL_ATTACK=seccomp"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestSeccompKillsABIMismatch(t *testing.T) {
	for _, attack := range []string{"arch", "x32"} {
		t.Run(attack, func(t *testing.T) {
			path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(path, append(args, os.Args[0])...)
			cmd.Env = []string{"INKWELL_ATTACK=" + attack}
			out, err := cmd.CombinedOutput()
			if err == nil || exitSignal(cmd.ProcessState) != int(unix.SIGSYS) {
				t.Fatalf("ABI mismatch survived: %v %s", err, out)
			}
		})
	}
}

// Host creates a real, writable SysV segment, then passes its id into the
// namespace-free launcher. Even an otherwise attachable segment must be denied.
func TestSandboxShmAttach(t *testing.T) {
	id, err := unix.SysvShmGet(unix.IPC_PRIVATE, 4096, unix.IPC_CREAT|0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = unix.SysvShmCtl(id, unix.IPC_RMID, nil) }()
	data, err := unix.SysvShmAttach(id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unix.SysvShmDetach(data) }()
	data[0] = 42
	runAttack(t, "shm", "INKWELL_SHMID="+strconv.Itoa(id))
	if data[0] != 42 {
		t.Fatal("host shared memory changed")
	}
}

func TestSandboxHomeRead(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	runAttack(t, "home", "HOME="+home)
}
func TestSandboxSignalControls(t *testing.T) { runAttack(t, "controls") }
func runAttack(t *testing.T, attack string, env ...string) {
	t.Helper()
	path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, append(args, os.Args[0])...)
	cmd.Env = append([]string{"INKWELL_ATTACK=" + attack}, env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v %s", attack, err, output)
	}
}

func TestSandboxInheritedFD(t *testing.T) {
	path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cmd := exec.Command(path, append(args, os.Args[0])...)
	cmd.ExtraFiles = []*os.File{f}
	cmd.Env = []string{"INKWELL_ATTACK=fds"}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, output)
	}
}

func TestSeccompUnknownSyscall(t *testing.T) { runAttack(t, "unknown") }

func TestInitPreservesFileSizeSignal(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"INKWELL_ATTACK=init-report"}
	finish, closeReport, err := signalReport(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer closeReport()
	if err := cmd.Run(); err == nil {
		t.Fatal("signal death succeeded")
	}
	out := Output{ExitCode: cmd.ProcessState.ExitCode()}
	finish(&out)
	if out.Signal != int(unix.SIGXFSZ) || out.ExitCode != -1 {
		t.Fatalf("%+v", out)
	}
}
