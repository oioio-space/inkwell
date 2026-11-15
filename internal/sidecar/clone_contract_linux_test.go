package sidecar

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestCloneThreeFallback(t *testing.T) {
	for _, name := range []string{Tinymist, Typst} {
		t.Run(name, func(t *testing.T) {
			if !requireLaunchFor(t, "S-6 clone3") {
				return
			}
			requireEmbedded(t)
			// A raw call from a confined child distinguishes ENOSYS from EPERM even
			// on a libc that has cached the older clone entry point.
			s := spec("sb2-clone3")
			s.tier = tierF
			raw, err := Run(t.Context(), s)
			if err != nil || raw.ExitCode != 0 || string(raw.Stdout) != "clone3: ENOSYS\n" {
				t.Fatalf("S-6 clone3: raw result %+v %v", raw, err)
			}
			binary, err := (&Store{Dir: sharedCache}).Resolve(name)
			if err != nil {
				t.Fatalf("S-6 clone3: resolve: %v", err)
			}
			dir := t.TempDir()
			args := []string{"lsp"}
			if name == Typst {
				if err := os.WriteFile(filepath.Join(dir, "main.typ"), bytes.Repeat([]byte("Hello world.\n\n"), 3000), 0o600); err != nil {
					t.Fatalf("S-6 clone3: input: %v", err)
				}
				args = []string{"compile", "--creation-timestamp", "0", "main.typ", "out.pdf"}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			// Observe the target PID after the verified report, while the real binary
			// runs. A version command is insufficient: it need not create a thread.
			var cmd *exec.Cmd
			var mu sync.Mutex
			maxThreads := 0
			var libc string
			observed := make(chan struct{})
			stop := make(chan struct{})
			var observing atomic.Bool
			stopObservation := sync.OnceFunc(func() { close(stop) })
			defer func() {
				stopObservation()
				if observing.Load() {
					<-observed
				}
			}()
			sandboxed := Spec{Path: binary, Dir: dir, Args: args, tier: tierF, ReadPaths: []string{}}
			if name == Typst {
				sandboxed.WatchDir = t.TempDir()
				args[len(args)-1] = filepath.Join(sandboxed.WatchDir, "out.pdf")
				sandboxed.Args = args
				sandboxed.WriteFiles = []string{"out.pdf"}
				sandboxed.MaxDirBytes = 32 << 20
			}
			sandboxed.rewriteLauncher = func(c *exec.Cmd) { cmd = c }
			sandboxed.afterReport = func() {
				observing.Store(true)
				go func() {
					defer close(observed)
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-stop:
							return
						case <-ticker.C:
							exe, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", cmd.Process.Pid))
							if exe != binary {
								continue
							}
							status, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", cmd.Process.Pid))
							for line := range strings.SplitSeq(string(status), "\n") {
								if v, ok := strings.CutPrefix(line, "Threads:"); ok {
									n, _ := strconv.Atoi(strings.TrimSpace(v))
									mu.Lock()
									maxThreads = max(maxThreads, n)
									mu.Unlock()
								}
							}
							maps, _ := os.ReadFile(fmt.Sprintf("/proc/%d/maps", cmd.Process.Pid))
							for line := range strings.SplitSeq(string(maps), "\n") {
								if strings.Contains(line, "libc.so") {
									mu.Lock()
									libc = line
									mu.Unlock()
								}
							}
						}
					}
				}()
			}
			var got, baseline []byte
			if name == Tinymist {
				got = cloneLSPInitialize(ctx, t, binary, dir, &sandboxed)
				baseline = cloneLSPInitialize(ctx, t, binary, dir, nil)
			} else {
				out, err := Run(ctx, sandboxed)
				if err != nil || out.ExitCode != 0 {
					t.Fatalf("S-6 clone3: compile: %+v %v", out, err)
				}
				var e error
				got, e = os.ReadFile(args[len(args)-1])
				if e != nil {
					t.Fatalf("S-6 clone3: PDF: %v", e)
				}
				plain := exec.CommandContext(ctx, binary, args...)
				plain.Dir = dir
				plain.Env = []string{}
				if data, e := plain.CombinedOutput(); e != nil {
					t.Fatalf("S-6 clone3: unrestricted compile: %s %v", data, e)
				}
				baseline, e = os.ReadFile(args[len(args)-1])
				if e != nil {
					t.Fatalf("S-6 clone3: baseline PDF: %v", e)
				}
			}
			stopObservation()
			<-observed
			mu.Lock()
			threads, loaded := maxThreads, libc
			mu.Unlock()
			if threads <= 1 || !bytes.Equal(got, baseline) {
				t.Fatalf("S-6 clone3: %s threads=%d results equal=%t", name, threads, bytes.Equal(got, baseline))
			}
			version := exec.CommandContext(ctx, "getconf", "GNU_LIBC_VERSION")
			glibc, e := version.Output()
			if e != nil {
				t.Fatalf("S-6 clone3: glibc version: %v", e)
			}
			t.Logf("S-6 clone3: %s; loaded libc: %s; threads=%d", strings.TrimSpace(string(glibc)), loaded, threads)
			if name == Tinymist && loaded == "" {
				t.Fatal("S-6 clone3: loaded glibc not observed")
			}
		})
	}
}

// cloneLSPInitialize compares actual capabilities, keeping the server alive
// until its answer so the observer can count its threads and loaded libc.
func cloneLSPInitialize(ctx context.Context, t *testing.T, binary, dir string, specification *Spec) []byte {
	t.Helper()
	in, out, done := startLSP(ctx, binary, dir, specification)
	defer func() { _ = in.Close() }()
	defer func() { _ = out.Close() }()
	writeFrame := func(body string) {
		t.Helper()
		if err := writeLSP(ctx, in, []byte(body)); err != nil {
			t.Fatalf("S-6 clone3: LSP write: %v", err)
		}
	}
	writeFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}`)
	r := bufio.NewReader(out)
	result := make(chan []byte, 1)
	go func() {
		for {
			b, err := readLSP(r)
			if err != nil {
				result <- nil
				return
			}
			if bytes.Contains(b, []byte(`"id":1`)) {
				result <- b
				return
			}
		}
	}()
	var response []byte
	select {
	case response = <-result:
	case <-ctx.Done():
		t.Fatal("S-6 clone3: initialize deadline")
	}
	if !bytes.Contains(response, []byte(`"capabilities"`)) {
		t.Fatalf("S-6 clone3: bad initialize %q", response)
	}
	// Consume the shutdown response while preserving backpressure on neither
	// stream; exit is queued after shutdown on the same ordered input.
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, r); close(drained) }()
	writeFrame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	writeFrame(`{"jsonrpc":"2.0","id":99,"method":"shutdown","params":null}`)
	writeFrame(`{"jsonrpc":"2.0","method":"exit","params":null}`)
	_ = in.Close()
	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("S-6 clone3: LSP exit: %v", e)
		}
	case <-ctx.Done():
		t.Fatal("S-6 clone3: LSP exit deadline")
	}
	select {
	case <-drained:
	case <-ctx.Done():
		t.Fatal("S-6 clone3: output drain deadline")
	}
	var value any
	if err := json.Unmarshal(response, &value); err != nil {
		t.Fatalf("S-6 clone3: initialize JSON: %v", err)
	}
	if response, ok := value.(map[string]any); ok {
		sortLSPCommands(response["result"])
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("S-6 clone3: canonical JSON: %v", err)
	}
	return canonical
}

// cloneThreeHelper makes the raw call, in a child: libc would translate
// clone3 into the old clone call and hide the filter's answer.
func cloneThreeHelper() int {
	var args [11]uint64
	_, _, errno := unix.Syscall(unix.SYS_CLONE3, uintptr(unsafe.Pointer(&args[0])), uintptr(len(args)*8), 0)
	fmt.Println("clone3:", unix.ErrnoName(errno))
	if errno != unix.ENOSYS {
		return 3
	}
	return 0
}

// Only command ordering is unspecified; preserve every capability and command.
func sortLSPCommands(result any) {
	r, _ := result.(map[string]any)
	capabilities, _ := r["capabilities"].(map[string]any)
	provider, _ := capabilities["executeCommandProvider"].(map[string]any)
	commands, _ := provider["commands"].([]any)
	slices.SortFunc(commands, func(a, b any) int { return strings.Compare(a.(string), b.(string)) })
}
