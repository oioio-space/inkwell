package sidecar

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FINDING 4: the limits are in force from the first instruction of the
// program, not applied to a process that is already running.
func TestRunLimitsHoldFromStartup(t *testing.T) {
	s := spec("rlimits")
	s.MemoryLimit = 3 << 30
	s.FileSizeLimit = 5 << 20
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 {
		t.Fatalf("Run = %v exit %d stderr %q", err, out.ExitCode, out.Stderr)
	}
	// ulimit counts the file size in blocks of 512 or 1024 bytes, by shell.
	var as, fsize int64
	if _, err := fmt.Sscanf(string(out.Stdout), "as=%d fsize=%d", &as, &fsize); err != nil {
		t.Fatalf("limits %q: %v", out.Stdout, err)
	}
	if as != 3<<30 {
		t.Errorf("address space limit = %d, want %d", as, int64(3<<30))
	}
	if fsize < 5<<20 || fsize > 10<<20 {
		t.Errorf("file size limit = %d, want 5 MiB (up to 2x with a 1 KiB-block shell)", fsize)
	}
}

func TestRunFileSizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big")
	s := spec("writefile:" + path + ",32")
	s.FileSizeLimit = 2 << 20
	out, err := Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode == 0 {
		t.Errorf("a 32 MiB write got through a 2 MiB limit: %q", out.Stdout)
	}
	if info, err := os.Stat(path); err != nil || info.Size() > 4<<20 {
		t.Errorf("file = %v, %v; want at most the limit", info, err)
	}
}

// FINDING 2: a directory that grows past its bound gets the process killed.
func TestRunDirectoryBound(t *testing.T) {
	dir := t.TempDir()
	s := spec("flooddir:" + dir)
	s.WatchDir, s.MaxDirBytes = dir, 8<<20
	begin := time.Now()
	out, err := Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OutputLimit {
		t.Errorf("OutputLimit = false after the child wrote past the bound (exit %d)", out.ExitCode)
	}
	if took := time.Since(begin); took > 10*time.Second {
		t.Errorf("took %v", took)
	}
	if got := dirBytes(dir); got > 128<<20 {
		t.Errorf("directory holds %d MiB, want the overshoot bounded", got>>20)
	}
}

func TestRunDirectoryWithinBound(t *testing.T) {
	dir := t.TempDir()
	s := spec("writefile:" + filepath.Join(dir, "ok") + ",1")
	s.WatchDir, s.MaxDirBytes = dir, 8<<20
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || out.OutputLimit {
		t.Fatalf("Run = %v exit %d limit %v, want a clean run", err, out.ExitCode, out.OutputLimit)
	}
}

// FINDING 3: the child has no network, not even loopback.
func TestRunNoNetwork(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	addr := ln.Addr().String()

	out, err := Run(t.Context(), spec("dial:"+addr))
	if err != nil || strings.TrimSpace(string(out.Stdout)) != "connected" {
		t.Fatalf("without isolation: %v %q, want the loopback to answer", err, out.Stdout)
	}

	s := spec("dial:" + addr)
	s.NoNetwork = true
	out, err = Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("network isolation available: %v", NetworkIsolation())
	if NetworkIsolation() {
		if !out.Isolated || strings.TrimSpace(string(out.Stdout)) != "unreachable" {
			t.Errorf("isolated=%v output %q; want a private network where the loopback is unreachable", out.Isolated, out.Stdout)
		}
	} else if out.Isolated {
		t.Error("Isolated = true although isolation is unavailable")
	}
}

func TestRunNoNetworkFallsBack(t *testing.T) {
	old := isolationOverride
	isolationOverride = func() bool { return false }
	t.Cleanup(func() { isolationOverride = old })
	s := spec("echo")
	s.NoNetwork = true
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || out.Isolated {
		t.Fatalf("Run = %v exit %d isolated %v, want it to run without isolation", err, out.ExitCode, out.Isolated)
	}
}
