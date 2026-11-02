package sidecar

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// TestWithLimitsExactBytes checks the policy passed to the pre-exec launcher.
func TestWithLimitsExactBytes(t *testing.T) {
	path, args, err := withLimits(Spec{Path: "target", Args: []string{"a"}})
	if err != nil || path != "target" || !slices.Equal(args, []string{"a"}) {
		t.Fatalf("unset limits: %q %q %v", path, args, err)
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
	if probeOutputMount() && !strings.Contains(string(out.Stdout), "CapBnd:\t0000000000000000") {
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
	os.Exit(0)
}

func confinementAttack() error {
	pid, err := strconv.Atoi(os.Getenv("INKWELL_SIBLING"))
	if err != nil {
		return err
	}
	switch os.Getenv("INKWELL_ATTACK") {
	case "signal":
		if err := unix.Kill(pid, 0); !errors.Is(err, unix.EPERM) {
			return fmt.Errorf("kill sibling: %w", err)
		}
	case "bus":
		fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
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
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	for _, tc := range []struct {
		name string
		abi  uintptr
	}{{"signal", 6}, {"bus", 9}} {
		t.Run(tc.name, func(t *testing.T) {
			if errno != 0 || abi < tc.abi {
				t.Skipf("requires Landlock ABI >= %d; probed %d (%v)", tc.abi, abi, errno)
			}
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
			path, args, err := sandboxPrefix(sandboxConfig{Directory: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			args = append(args, os.Args[0], "-test.run=^TestConfinementHelper$")
			cmd := exec.Command(path, args...)
			cmd.Env = []string{"INKWELL_ATTACK=" + tc.name, fmt.Sprintf("INKWELL_SIBLING=%d", sibling.Process.Pid), "INKWELL_BUS=" + bus, "XDG_RUNTIME_DIR=" + runtimeDir}
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
	for abi := uintptr(3); abi <= 9; abi++ {
		attr, size := landlockPolicy(abi)
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
	path, args, err := sandboxPrefix(sandboxConfig{Directory: dir, Names: []string{"out"}})
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
