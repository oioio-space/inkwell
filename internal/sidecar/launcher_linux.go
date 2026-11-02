package sidecar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const sandboxMode = "--inkwell-internal-sandbox-v1"

type sandboxConfig struct {
	WorkingDir   string
	RuntimeDir   string
	Memory, File uint64
	Directory    string
	Names        []string
	MountBytes   int64
	Probe        bool
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
	if c.MountBytes > 0 {
		if err := mountOutput(c); err != nil {
			return err
		}
	}
	if c.Probe {
		return nil
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
	if c.Directory != "" {
		if err := restrictOutput(c); err != nil {
			return err
		}
	}
	for _, limit := range []struct {
		resource int
		value    uint64
	}{
		{unix.RLIMIT_CORE, 0}, {unix.RLIMIT_AS, c.Memory}, {unix.RLIMIT_FSIZE, c.File},
	} {
		if limit.value == 0 && limit.resource == unix.RLIMIT_AS || limit.value == 0 && limit.resource == unix.RLIMIT_FSIZE && c.Directory == "" {
			continue
		}
		if err := unix.Setrlimit(limit.resource, &unix.Rlimit{Cur: limit.value, Max: limit.value}); err != nil {
			return err
		}
	}
	return unix.Exec(args[0], args, os.Environ())
}

func mountOutput(c sandboxConfig) error {
	size := c.MountBytes / int64(os.Getpagesize()) * int64(os.Getpagesize())
	if size <= 0 {
		return errors.New("sidecar: tmpfs size must be at least one page")
	}
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

// landlockPolicy uses only fields and rights supported by the probed kernel.
// https://docs.kernel.org/userspace-api/landlock.html (ABI compatibility).
func landlockPolicy(abi uintptr) (unix.LandlockRulesetAttr, uintptr) {
	attr := unix.LandlockRulesetAttr{Handled_access_fs: outputAccess}
	size := uintptr(8)
	if abi >= 4 {
		attr.Handled_access_net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
		size = 16
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
	for _, name := range c.Names {
		pathFD, err := unix.Open(filepath.Join(c.Directory, name), unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
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
	_, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
