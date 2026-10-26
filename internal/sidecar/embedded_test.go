package sidecar

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests run the real typst and tinymist embedded in this build. They
// need `mise run sidecar:fetch` (the mise test tasks run it first).

func requireEmbedded(t *testing.T) {
	t.Helper()
	for _, name := range []string{Typst, Tinymist} {
		if _, err := fs.Stat(Embedded(), name+".gz"); err != nil {
			t.Fatalf("%s is not embedded in this build (%v): run `mise run sidecar:fetch`", name, err)
		}
	}
}

func TestEmbeddedSidecarsRun(t *testing.T) {
	requireEmbedded(t)
	store := &Store{Dir: sharedCache}

	t.Run("SIDE-02 the pinned typst runs", func(t *testing.T) {
		path, err := store.Resolve(Typst)
		if err != nil {
			t.Fatalf("Resolve(typst) = %v", err)
		}
		out, err := Run(t.Context(), Spec{Path: path, Args: []string{"--version"}})
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("typst --version = %v exit %d stderr %q", err, out.ExitCode, out.Stderr)
		}
		if got := string(out.Stdout); !strings.HasPrefix(got, "typst 0.15.1 ") {
			t.Errorf("typst --version = %q, want the pinned 0.15.1", got)
		}
	})

	t.Run("SIDE-02 the pinned tinymist runs, on the same Typst", func(t *testing.T) {
		path, err := store.Resolve(Tinymist)
		if err != nil {
			t.Fatalf("Resolve(tinymist) = %v", err)
		}
		out, err := Run(t.Context(), Spec{Path: path, Args: []string{"--version"}, Env: []string{"HOME=" + t.TempDir()}})
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("tinymist --version = %v exit %d stderr %q", err, out.ExitCode, out.Stderr)
		}
		if got := string(out.Stdout); !strings.Contains(got, "Typst Version:") || !strings.Contains(got, "0.15.1") {
			t.Errorf("tinymist --version = %q, want it built on Typst 0.15.1", got)
		}
	})

	t.Run("SIDE-04 a tampered real binary is replaced and still runs (PKG-05)", func(t *testing.T) {
		path, err := store.Resolve(Typst)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(path, os.O_WRONLY, 0) // #nosec G302 G304 -- the cached file this test extracted; the test is the attacker
		if err != nil {
			if err := os.Chmod(path, 0o700); err != nil { // #nosec G302 -- the test is the attacker
				t.Fatal(err)
			}
			if f, err = os.OpenFile(path, os.O_WRONLY, 0); err != nil { // #nosec G304 -- see above
				t.Fatal(err)
			}
		}
		if _, err := f.WriteAt([]byte{0xde, 0xad, 0xbe, 0xef}, 4096); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if verify(path, mustArtifact(t, Typst)) == "" {
			t.Fatal("the tampered binary still verifies")
		}

		again, err := store.Resolve(Typst)
		if err != nil || again != path {
			t.Fatalf("Resolve after tampering = %q, %v; want %q", again, err, path)
		}
		out, err := Run(t.Context(), Spec{Path: again, Args: []string{"--version"}})
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("restored typst --version = %v exit %d", err, out.ExitCode)
		}
	})
}

func mustArtifact(t *testing.T, name string) Artifact {
	t.Helper()
	m, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	_, art, err := m.Artifact(name, HostTarget())
	if err != nil {
		t.Fatal(err)
	}
	return art
}

// SIDE-29: the real language server, supervised, answers on stdio.
func TestSupervisedTinymistLSP(t *testing.T) {
	requireEmbedded(t)
	store := &Store{Dir: sharedCache}
	home := t.TempDir()

	var (
		mu     sync.Mutex
		stdin  io.WriteCloser
		stdout *bufio.Reader
	)
	rec := &recorder{}
	sup := NewSupervisor(SupervisorConfig{
		Name:    "tinymist",
		OnEvent: rec.on,
		Command: func(context.Context) (*exec.Cmd, error) {
			path, err := store.Resolve(Tinymist) // verified before every launch
			if err != nil {
				return nil, err
			}
			cmd := exec.Command(path, "lsp") // #nosec G204 -- path from Store.Resolve
			cmd.Env = []string{"HOME=" + home}
			in, err := cmd.StdinPipe()
			if err != nil {
				return nil, err
			}
			out, err := cmd.StdoutPipe()
			if err != nil {
				return nil, err
			}
			mu.Lock()
			stdin, stdout = in, bufio.NewReader(out)
			mu.Unlock()
			return cmd, nil
		},
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- sup.Run(ctx) }()
	stop := sync.OnceFunc(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run = %v after cancellation, want nil", err)
		}
	})
	t.Cleanup(stop)

	pid := rec.waitKind(t, EventStart, 1)[0].PID
	mu.Lock()
	in, out := stdin, stdout
	mu.Unlock()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}`
	if _, err := fmt.Fprintf(in, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		t.Fatal(err)
	}
	reply := make(chan []byte, 1)
	go func() {
		n := 0
		for {
			line, err := out.ReadString('\n')
			if err != nil {
				reply <- nil
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(strings.ToLower(line), "content-length:"); ok {
				n, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(out, buf); err != nil {
			reply <- nil
			return
		}
		reply <- buf
	}()
	var raw []byte
	select {
	case raw = <-reply:
	case <-time.After(30 * time.Second):
		t.Fatal("tinymist did not answer initialize")
	}
	var resp struct {
		Result struct {
			Capabilities map[string]any `json:"capabilities"`
		} `json:"result"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&resp); err != nil {
		t.Fatalf("initialize reply %q: %v", raw, err)
	}
	if _, ok := resp.Result.Capabilities["completionProvider"]; !ok {
		t.Errorf("capabilities = %v, want completionProvider", resp.Result.Capabilities)
	}

	stop()
	if !waitGone(pid, 5*time.Second) {
		t.Errorf("tinymist %d survived the supervisor", pid)
	}
}
