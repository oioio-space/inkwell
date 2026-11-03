package sidecar

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLauncherReadConfiguration(t *testing.T) {
	for _, target := range []string{"/bin/sh", os.Args[0]} {
		c := sandboxConfig{WorkingDir: t.TempDir(), Directory: t.TempDir()}
		if err := restrictReadsConfig(&c, target); err != nil {
			t.Fatal(err)
		}
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatal(err)
		}
		if c.Executable != resolved || !slices.Contains(c.ReadPaths, c.WorkingDir) || !slices.Contains(c.ReadPaths, c.Directory) || !slices.Contains(c.ReadPaths, "/usr/share/fonts") {
			t.Fatalf("bad read authority: %+v", c)
		}
		if target == "/bin/sh" && (!c.Dynamic || len(c.Libraries) == 0) {
			t.Fatal("missing dynamic libraries")
		}
	}
	c := sandboxConfig{ReadPaths: []string{"/custom"}}
	if err := restrictReadsConfig(&c, os.Args[0]); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(c.ReadPaths, "/usr/share/fonts") {
		t.Fatal("custom paths gained default fonts")
	}
	if err := restrictReadsConfig(&c, "/nonexistent/inkwell"); err == nil {
		t.Fatal("missing executable accepted")
	}
	file := filepath.Join(t.TempDir(), "text")
	if err := os.WriteFile(file, []byte("not ELF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restrictReadsConfig(&c, file); err == nil {
		t.Fatal("non ELF accepted")
	}
	if _, err := dynamicLibraries(file, map[string]bool{}); err == nil {
		t.Fatal("non ELF dependencies accepted")
	}
	if paths, err := dynamicLibraries(file, map[string]bool{file: true}); err != nil || len(paths) != 0 {
		t.Fatal("dependency cycle not stopped")
	}
}

func TestLauncherReportAndPrefix(t *testing.T) {
	for _, value := range []string{"", "2", "bad", "7"} {
		t.Setenv(signalReportEnv, value)
		c := sandboxConfig{}
		err := readReportFD(&c)
		if value == "2" || value == "bad" {
			if err == nil {
				t.Fatal("invalid fd accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if value == "7" && (c.ReportFD != 7 || os.Getenv(signalReportEnv) != "") {
			t.Fatal("report descriptor not consumed")
		}
	}
	for _, dir := range []string{"", "/custom/runtime"} {
		t.Setenv("XDG_RUNTIME_DIR", dir)
		path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: "project", Names: []string{"out"}})
		if err != nil || path != "/proc/self/exe" || len(args) != 2 || args[0] != sandboxMode {
			t.Fatalf("%q %q %v", path, args, err)
		}
		var c sandboxConfig
		if err := json.Unmarshal([]byte(args[1]), &c); err != nil {
			t.Fatal(err)
		}
		if c.WorkingDir != "project" || !slices.Equal(c.Names, []string{"out"}) || c.RuntimeDir == "" || (dir != "" && c.RuntimeDir != dir) {
			t.Fatalf("%+v", c)
		}
	}
}

// Security state is per Linux thread. A locked goroutine's thread is destroyed
// on exit, so these checks never confine the test runner or its other threads.
func TestConfinementOnDisposableThread(t *testing.T) {
	project := t.TempDir()
	output := t.TempDir()
	name := filepath.Join(output, "out")
	if err := os.WriteFile(name, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
		if err == nil {
			err = dropCapabilities()
		}
		c := sandboxConfig{WorkingDir: project, Directory: output, Names: []string{"out"}}
		if err == nil {
			err = restrictReadsConfig(&c, "/bin/sh")
		}
		if err == nil {
			err = restrictOutput(c)
		}
		if err == nil {
			err = installSeccomp()
		}
		if err == nil {
			// The installed policy must permit writes to the granted output and deny
			// creating a sibling, beyond the synthetic policy checks.
			var fd int
			fd, err = unix.Open(name, unix.O_WRONLY, 0)
			if err == nil {
				err = unix.Close(fd)
			}
			if err == nil {
				fd, e := unix.Open(filepath.Join(output, "other"), unix.O_CREAT|unix.O_WRONLY, 0o600)
				if e == nil {
					_ = unix.Close(fd)
					err = unix.EACCES
				}
			}
		}
		done <- err
	}()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSandboxEarlyPaths(t *testing.T) {
	t.Setenv(signalReportEnv, "invalid")
	if err := sandboxExec(sandboxConfig{}, nil); err == nil {
		t.Fatal("invalid report fd accepted")
	}
	t.Setenv(signalReportEnv, "")
	if err := sandboxExec(sandboxConfig{Probe: true}, nil); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		err := sandboxExec(sandboxConfig{}, nil)
		if err == nil {
			err = unix.EINVAL
		}
		done <- err
	}()
	if err := <-done; err.Error() != "missing target" {
		t.Fatal(err)
	}
	for _, c := range []sandboxConfig{{}, {WorkingDir: t.TempDir()}} {
		done := make(chan error, 1)
		go func() { runtime.LockOSThread(); done <- sandboxExec(c, []string{"/nonexistent/inkwell"}) }()
		if err := <-done; err == nil {
			t.Fatal("invalid launcher configuration accepted")
		}
	}
}

func TestLauncherMountConstruction(t *testing.T) {
	if os.Getenv("INKWELL_MOUNT_UNIT") == "1" {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		dir := os.Getenv("INKWELL_MOUNT_DIR")
		c := sandboxConfig{RuntimeDir: dir, Overlay: true, Directory: dir, MountBytes: 8192, Names: []string{"out"}, Probe: true}
		if err := sandboxExec(c, nil); err != nil {
			t.Fatal(err)
		}
		var stat unix.Statfs_t
		if err := unix.Statfs(dir, &stat); err != nil || stat.Type != unix.TMPFS_MAGIC {
			t.Fatalf("mount: %+v %v", stat, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "out")); err != nil {
			t.Fatal(err)
		}

		// Exercise the ready/proceed protocol on the real output mount.
		c.Probe = false
		c.WorkingDir = dir
		if err := mountOutput(c); err != nil {
			t.Fatal(err)
		}

		c.WorkingDir = "/nonexistent/inkwell"
		if err := mountOutput(c); err == nil {
			t.Fatal("missing working directory accepted")
		}
		c.Probe = true
		c.Names = []string{"out", "out"}
		if err := mountOutput(c); err == nil {
			t.Fatal("duplicate output accepted")
		}
		if err := mountIPC(sandboxConfig{RuntimeDir: "/nonexistent/inkwell"}); err != nil {
			t.Fatal(err)
		}
		if err := dropCapabilities(); err != nil {
			t.Fatal(err)
		}
		return
	}
	dir := t.TempDir()
	args := []string{"-test.run=^TestLauncherMountConstruction$"}
	if f := flag.Lookup("test.gocoverdir"); f != nil && f.Value.String() != "" {
		args = append(args, "-test.gocoverdir="+f.Value.String())
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "INKWELL_MOUNT_UNIT=1", "INKWELL_MOUNT_DIR="+dir)
	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = readyR.Close() }()
	defer func() { _ = readyW.Close() }()
	proceedR, proceedW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proceedR.Close() }()
	defer func() { _ = proceedW.Close() }()
	cmd.ExtraFiles = []*os.File{readyW, proceedR}
	go func() { var b [1]byte; _, _ = io.ReadFull(readyR, b[:]); _, _ = proceedW.Write([]byte("p")) }()
	setProcAttr(cmd)
	mountNamespace(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mount policy: %v %s", err, out)
	}
}

func TestSandboxExecDeniedTarget(t *testing.T) {
	// A valid ELF without execute permission traverses the whole setup while
	// guaranteeing exec cannot replace the process or run untrusted code.
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { runtime.LockOSThread(); done <- sandboxExec(sandboxConfig{WorkingDir: dir}, []string{target}) }()
	if err := <-done; !errors.Is(err, unix.EACCES) {
		t.Fatalf("exec nonexecutable ELF: %v", err)
	}
}

func TestWorkingDirectoryErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(dir, "missing")} {
		if _, err := workingDirectory(path); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "")
			if key != "HOME" {
				t.Setenv("HOME", "")
			}
			if _, err := workingDirectory(dir); err == nil {
				t.Fatal("missing home accepted")
			}
		})
	}
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(dir, "data"))
	if _, err := workingDirectory(dir); err == nil {
		t.Fatal("XDG search directory ancestor accepted")
	}
}

func TestLandlockRuleFailures(t *testing.T) {
	for _, c := range []sandboxConfig{
		{Directory: t.TempDir(), Names: []string{"missing"}},
		{ReadPaths: []string{"/nonexistent/inkwell", "/dev/null", t.TempDir()}},
	} {
		done := make(chan error, 1)
		go func() {
			runtime.LockOSThread()
			err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
			if err == nil {
				err = restrictOutput(c)
			}
			done <- err
		}()
		err := <-done
		if len(c.Names) > 0 && err == nil {
			t.Fatal("missing output accepted")
		}
		if len(c.Names) == 0 && err != nil {
			t.Fatal(err)
		}
	}
}

func TestSandboxInitArguments(t *testing.T) {
	t.Setenv("INKWELL_TEST_ENV", "value")
	c := sandboxConfig{WorkingDir: "/project", Memory: 123}
	cmd, err := sandboxInitCommand(c, []string{"/target", "a b", ";literal"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != "/proc/self/exe" || len(cmd.Args) != 6 || !slices.Equal(cmd.Args[3:], []string{"/target", "a b", ";literal"}) {
		t.Fatalf("%q", cmd.Args)
	}
	var got sandboxConfig
	if err := json.Unmarshal([]byte(cmd.Args[2]), &got); err != nil {
		t.Fatal(err)
	}
	if got.WorkingDir != c.WorkingDir || got.Memory != c.Memory || !slices.Contains(cmd.Env, "INKWELL_TEST_ENV=value") || cmd.Stdin != os.Stdin || cmd.Stdout != os.Stdout || cmd.Stderr != os.Stderr {
		t.Fatalf("bad init config: %+v", got)
	}
}

func TestSandboxExitStatus(t *testing.T) {
	if got := sandboxExitStatus(unix.WaitStatus(7<<8), 0); got != 7 {
		t.Fatalf("exit=%d", got)
	}
	if got := sandboxExitStatus(unix.WaitStatus(unix.SIGTERM), 0); got != 128+int(unix.SIGTERM) {
		t.Fatalf("signal exit=%d", got)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	if got := sandboxExitStatus(unix.WaitStatus(unix.SIGXFSZ), int(w.Fd())); got != 128+int(unix.SIGXFSZ) {
		t.Fatal(got)
	}
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil || b[0] != byte(unix.SIGXFSZ) {
		t.Fatalf("signal report: %v %v", b, err)
	}
}

func TestInvalidLibraryNames(t *testing.T) {
	data, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("libc.so.6")) {
		t.Fatal("fixture lacks libc dependency")
	}
	for _, name := range []string{"bad/xx.xx", "zzzz.so.6"} {
		path := filepath.Join(t.TempDir(), "target")
		if err := os.WriteFile(path, bytes.ReplaceAll(data, []byte("libc.so.6"), []byte(name)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := dynamicLibraries(path, map[string]bool{}); err == nil {
			t.Fatalf("accepted dependency %s", name)
		}
	}
}

func TestWorkingDirectoryRelativeXDG(t *testing.T) {
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "relative")
			if _, err := workingDirectory(t.TempDir()); err == nil {
				t.Fatal("relative XDG base accepted")
			}
		})
	}
}

func TestSandboxMountErrors(t *testing.T) {
	if err := sandboxExec(sandboxConfig{MountBytes: 1}, nil); err == nil {
		t.Fatal("sub-page mount accepted")
	}
	if err := sandboxExec(sandboxConfig{Overlay: true, RuntimeDir: "/dev/null"}, nil); err == nil {
		t.Fatal("invalid overlay accepted")
	}
}

func TestSymlinkLoopAuthorities(t *testing.T) {
	dir := t.TempDir()
	loop := filepath.Join(dir, "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", loop)
	if _, err := workingDirectory(t.TempDir()); err == nil {
		t.Fatal("unresolvable sensitive directory accepted")
	}
	// A malformed optional read path must fail closed, not be treated as absent.
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
		if err == nil {
			err = restrictOutput(sandboxConfig{ReadPaths: []string{loop}})
		}
		done <- err
	}()
	if err := <-done; !errors.Is(err, unix.ELOOP) {
		t.Fatalf("loop read rule: %v", err)
	}
}
