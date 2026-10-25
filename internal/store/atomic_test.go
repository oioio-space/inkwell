package store

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Environment contract between the killer test and its re-executed child.
const (
	childEnv    = "INKWELL_STORE_CHILD"
	childDirEnv = "INKWELL_STORE_CHILD_DIR"
	readyLine   = "ready\n"
)

// runChild is the body of the writer subprocess. Mode "stall" stops between
// the synced temporary file and the Rename; mode "loop" rewrites the file
// forever until it is killed.
func runChild(mode, dir string) error {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	switch mode {
	case "stall":
		return writeAtomic(r, "value", bytes.NewReader(bytes.Repeat([]byte("new"), 1<<16)), 0o600, nil, func() {
			_, _ = os.Stdout.WriteString(readyLine)
			select {} // killed by the parent
		})
	case "loop":
		_, _ = os.Stdout.WriteString(readyLine)
		for i := 0; ; i++ {
			payload := bytes.Repeat([]byte{byte('a' + i%26)}, 1<<18)
			if err := WriteAtomic(r, "value", payload, 0o600); err != nil {
				return err
			}
		}
	}
	return errors.New("unknown mode " + mode)
}

// startChild re-executes the test binary as the writer and waits until it
// says it is ready.
func startChild(t *testing.T, mode, dir string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"="+mode, childDirEnv+"="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(readyLine))
	if _, err := out.Read(buf); err != nil || string(buf) != readyLine {
		t.Fatalf("child not ready: %q, %v", buf, err)
	}
	return cmd
}

func kill(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // "signal: killed"
}

func openTemp(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, dir
}

func TestWriteAtomicReplaces(t *testing.T) {
	r, _ := openTemp(t)
	if err := r.Mkdir("d", 0o700); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"one", "two"} {
		if err := WriteAtomic(r, "d/f", []byte(v), 0o600); err != nil {
			t.Fatalf("WriteAtomic(%q): %v", v, err)
		}
		got, err := r.ReadFile("d/f")
		if err != nil || string(got) != v {
			t.Fatalf("read back = %q, %v; want %q", got, err, v)
		}
	}
	info, err := r.Stat("d/f")
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
	entries, err := fs.ReadDir(r.FS(), "d")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries after write, want only the file", len(entries))
	}
}

func TestWriteAtomicMissingDir(t *testing.T) {
	r, _ := openTemp(t)
	if err := WriteAtomic(r, "nope/f", []byte("x"), 0o600); err == nil {
		t.Fatal("WriteAtomic into a missing directory succeeded")
	}
}

// TestKillDuringWrite is Review Focus 5: SIGKILL between the synced temporary
// file and the Rename leaves the previous value, never a truncated one.
func TestKillDuringWrite(t *testing.T) {
	r, dir := openTemp(t)
	if err := WriteAtomic(r, "value", []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	kill(t, startChild(t, "stall", dir))

	got, err := r.ReadFile("value")
	if err != nil || string(got) != "old" {
		t.Fatalf("after kill before rename: %q, %v; want the old value", got, err)
	}
	var leftover int
	entries, _ := fs.ReadDir(r.FS(), ".")
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			leftover++
		}
	}
	if leftover != 1 {
		t.Errorf("%d temporary files left, want exactly the interrupted one", leftover)
	}
}

// TestKillAtRandomTimes kills a writer that never stops, at varying moments:
// whatever is on disk is a complete value, old or new.
func TestKillAtRandomTimes(t *testing.T) {
	r, dir := openTemp(t)
	if err := WriteAtomic(r, "value", bytes.Repeat([]byte{'a'}, 1<<18), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := range 6 {
		cmd := startChild(t, "loop", dir)
		time.Sleep(time.Duration(i*7) * time.Millisecond)
		kill(t, cmd)

		got, err := r.ReadFile("value")
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if len(got) != 1<<18 || len(bytes.Trim(got, string(got[:1]))) != 0 {
			t.Fatalf("round %d: value is torn (len %d)", i, len(got))
		}
	}
}
