package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// sb2Helper runs only in a re-executed child. In particular an unexpectedly
// permitted attack never changes the test runner's resource limits or files.
func sb2Helper(mode, arg string) int {
	switch mode {
	case "sb2-fds":
		// Fstat does not open a directory descriptor or rely on Go's runtime FDs.
		// The one intentional exception is the target's death report pipe.
		report := -1
		if v := os.Getenv(signalReportEnv); v != "" {
			report, _ = strconv.Atoi(v)
		}
		for fd := 3; fd < 256; fd++ {
			var st unix.Stat_t
			if err := unix.Fstat(fd, &st); err == nil && fd != report {
				// The Go runtime may open epoll/eventfd after exec; they cannot be
				// inherited launcher pipes. Check the actual object, not its number.
				link, _ := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
				if link == "anon_inode:[eventpoll]" || link == "anon_inode:[eventfd]" {
					continue
				}
				fmt.Printf("leaked fd %d: %s\n", fd, link)
				return 3
			}
		}
		fmt.Println("descriptors closed")
	case "sb2-attack":
		parts := strings.SplitN(arg, "|", 3)
		if len(parts) != 3 {
			return 2
		}
		attempt, path := parts[0], parts[1]
		pid, _ := strconv.Atoi(parts[2])
		var err error
		switch attempt {
		case "parent-environ", "parent-maps":
			leaf := "environ"
			if attempt == "parent-maps" {
				leaf = "maps"
			}
			var data []byte
			data, err = os.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, leaf))
			if err == nil {
				fmt.Printf("read %q\n", data)
				return 3
			}
		case "proc-self-root":
			var data []byte
			data, err = os.ReadFile("/proc/self/root" + path)
			if err == nil {
				fmt.Printf("read %q\n", data)
				return 3
			}
		case "write-outside":
			err = os.WriteFile(path, []byte("corrupted"), 0o600)
		case "replace-outside":
			err = os.Rename("replacement", path)
		case "signal-parent":
			err = unix.Kill(pid, unix.SIGCONT)
			if !errors.Is(err, unix.EPERM) {
				fmt.Println("kill:", errnoText(err))
				return 3
			}
			err = unix.Tgkill(pid, pid, unix.SIGCONT)
		case "tkill-other-thread":
			_, _, errno := unix.Syscall(unix.SYS_TKILL, uintptr(pid), uintptr(unix.SIGCONT), 0)
			if errno != 0 {
				err = errno
			}
		case "prlimit-parent":
			limit := unix.Rlimit{Cur: 0, Max: 0}
			err = unix.Prlimit(pid, unix.RLIMIT_CORE, &limit, nil)
		case "ptrace-parent":
			err = unix.PtraceAttach(pid)
		case "socket-tcp", "socket-udp", "socket-unix", "socket-abstract":
			// seccomp refuses socket(2) itself, so an abstract address is never
			// reached: socket-abstract is the AF_UNIX attempt under its ADR name.
			domain, kind := unix.AF_INET, unix.SOCK_STREAM
			switch attempt {
			case "socket-udp":
				kind = unix.SOCK_DGRAM
			case "socket-unix", "socket-abstract":
				domain = unix.AF_UNIX
			}
			var fd int
			fd, err = unix.Socket(domain, kind, 0)
			if err == nil {
				_ = unix.Close(fd)
			}
		case "slot-budget":
			f, e := os.OpenFile(path, os.O_WRONLY, 0)
			if e != nil {
				fmt.Println("open:", e)
				return 2
			}
			defer func() { _ = f.Close() }()
			err = f.Truncate(1 << 20)
		case "exec-slot":
			err = unix.Exec(path, []string{path}, []string{})
		default:
			return 2
		}
		// ENOENT is appropriate for a hidden PID namespace, EACCES for Landlock,
		// EPERM for seccomp. An unrelated error does not prove confinement.
		if !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.EACCES) && !errors.Is(err, unix.ENOENT) && !errors.Is(err, unix.EFBIG) {
			fmt.Println("unexpected:", errnoText(err))
			return 3
		}
		fmt.Println("refused")
	}
	return 0
}

func TestEscape(t *testing.T) {
	attempts := []string{"parent-environ", "parent-maps", "proc-self-root", "write-outside", "replace-outside", "slot-budget", "signal-parent", "tkill-other-thread", "prlimit-parent", "socket-tcp", "socket-udp", "socket-unix", "socket-abstract", "ptrace-parent"}
	for _, variant := range []struct {
		name string
		tr   tier
		abi  int
	}{
		{"N", tierN, 0}, {"F", tierF, 0}, {"F-abi3", tierF, 3}, {"F-abi4", tierF, 4},
	} {
		tr := variant.tr
		t.Run(variant.name, func(t *testing.T) {
			for _, attempt := range attempts {
				t.Run(attempt, func(t *testing.T) {
					if !requireTier(t, tr) {
						return
					}
					outside := filepath.Join(t.TempDir(), "secret")
					secret := []byte("SB2a1 outside sentinel")
					if err := os.WriteFile(outside, secret, 0o600); err != nil {
						t.Fatalf("S-4 escape: sentinel: %v", err)
					}
					project := t.TempDir()
					if err := os.WriteFile(filepath.Join(project, "replacement"), []byte("replacement"), 0o600); err != nil {
						t.Fatalf("S-4 escape: replacement: %v", err)
					}
					target := outside
					s := spec("")
					s.Dir = project
					s.tier = tr
					s.landlockABI = variant.abi
					if attempt == "slot-budget" {
						s.WatchDir = t.TempDir()
						s.WriteFiles = []string{"out"}
						s.MaxDirBytes = 8192
						target = filepath.Join(s.WatchDir, "out")
					}
					pid := os.Getpid()
					s.Env = helperEnv("sb2-attack:" + attempt + "|" + target + "|" + strconv.Itoa(pid))
					ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
					defer cancel()
					out, err := Run(ctx, s)
					// In N the bounded mount refuses the growth first: the run ends on its output limit.
					overBudget := attempt == "slot-budget" && tr == tierN && err != nil && out.OutputLimit
					if !overBudget && (err != nil || out.ExitCode != 0 || string(out.Stdout) != "refused\n") {
						t.Fatalf("S-4 escape: %s/%s: %+v %v", tr, attempt, out, err)
					}
					data, e := os.ReadFile(outside)
					if e != nil || string(data) != string(secret) {
						t.Fatalf("S-4 escape: %s changed outside sentinel: %q %v", attempt, data, e)
					}
					if attempt == "slot-budget" {
						if st, e := os.Stat(target); e == nil && st.Size() > s.MaxDirBytes {
							t.Fatalf("S-4 escape: slot grew to %d", st.Size())
						}
					}
				})
			}
			t.Run("exec-slot", func(t *testing.T) { escapeExecSlot(t, tr, variant.abi) })
			t.Run("allocation", func(t *testing.T) { escapeAllocation(t, tr, variant.abi) })
			t.Run("parent-death", func(t *testing.T) { escapeParentDeath(t, tr, variant.abi) })
			t.Run("fork-bomb", func(t *testing.T) {
				if tr == tierF {
					escapeNoProcess(t, variant.abi)
				} else {
					escapeProcessTree(t)
				}
			})
		})
	}
}

func TestTargetCannotLimitAnotherProcess(t *testing.T) {
	for _, tr := range []tier{tierN, tierF} {
		t.Run(tr.String(), func(t *testing.T) {
			if !requireTier(t, tr) {
				return
			}
			var before, after unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_CORE, &before); err != nil {
				t.Fatalf("S-1 prlimit64: %v", err)
			}
			s := spec("sb2-attack:prlimit-parent||" + strconv.Itoa(os.Getpid()))
			s.tier = tr
			out, err := Run(t.Context(), s)
			if e := unix.Getrlimit(unix.RLIMIT_CORE, &after); e != nil {
				t.Fatalf("S-1 prlimit64: %v", e)
			}
			if err != nil || out.ExitCode != 0 || string(out.Stdout) != "refused\n" || before != after {
				t.Fatalf("S-1 prlimit64: parent limit %+v -> %+v output %+v error %v", before, after, out, err)
			}
		})
	}
}
