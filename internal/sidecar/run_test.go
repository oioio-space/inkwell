package sidecar

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func spec(mode string) Spec {
	return Spec{Path: os.Args[0], Env: helperEnv(mode)}
}

func TestRun(t *testing.T) {
	t.Run("SIDE-16 output is captured and the exit code reported", func(t *testing.T) {
		out, err := Run(t.Context(), spec("echo"))
		if err != nil {
			t.Fatalf("Run = %v", err)
		}
		if string(out.Stdout) != "out\n" || string(out.Stderr) != "err\n" || out.ExitCode != 0 {
			t.Errorf("Run = %q %q exit %d, want out/err/0", out.Stdout, out.Stderr, out.ExitCode)
		}

		out, err = Run(t.Context(), spec("exit:3"))
		if err != nil {
			t.Fatalf("Run(exit 3) = %v, want the code in Output, not an error", err)
		}
		if out.ExitCode != 3 {
			t.Errorf("ExitCode = %d, want 3", out.ExitCode)
		}
	})

	t.Run("SIDE-16 output is bounded", func(t *testing.T) {
		s := spec("flood")
		s.MaxOutput = 1024
		out, err := Run(t.Context(), s)
		if err != nil {
			t.Fatalf("Run = %v", err)
		}
		if len(out.Stdout) != 1024 || len(out.Stderr) != 1024 || !out.Truncated {
			t.Errorf("kept %d+%d bytes, Truncated=%v; want 1024+1024 and true", len(out.Stdout), len(out.Stderr), out.Truncated)
		}
		if out.ExitCode != 0 {
			t.Errorf("ExitCode = %d: a flooding child must still be drained, not blocked", out.ExitCode)
		}
	})

	t.Run("the parent's environment and directory are not inherited", func(t *testing.T) {
		t.Setenv("INKWELL_PARENT_SECRET", "hunter2")
		s := spec("env")
		s.Env = append(s.Env, "KEPT=1")
		out, err := Run(t.Context(), s)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(out.Stdout)); got != "KEPT=1" {
			t.Errorf("child environment = %q, want only KEPT=1", got)
		}

		dir := t.TempDir()
		s = spec("pwd")
		s.Dir = dir
		out, err = Run(t.Context(), s)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(out.Stdout)); !strings.HasSuffix(got, dir) {
			t.Errorf("working directory = %q, want %q", got, dir)
		}
	})

	t.Run("stdin is passed", func(t *testing.T) {
		s := spec("cat")
		s.Stdin = strings.NewReader("hello")
		out, err := Run(t.Context(), s)
		if err != nil {
			t.Fatal(err)
		}
		if string(out.Stdout) != "hello" {
			t.Errorf("stdout = %q, want hello", out.Stdout)
		}
	})

	t.Run("a program that cannot start is an error", func(t *testing.T) {
		if _, err := Run(t.Context(), Spec{Path: "/nonexistent/sidecar"}); err == nil {
			t.Fatal("Run(missing binary) = nil, want an error")
		}
		if _, err := Run(t.Context(), Spec{}); err == nil {
			t.Fatal("Run(no path) = nil, want an error")
		}
	})

	t.Run("SIDE-17 a timeout kills the whole process group", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		start := time.Now()
		out, err := Run(ctx, spec("grandchild"))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run = %v, want DeadlineExceeded", err)
		}
		if took := time.Since(start); took > 5*time.Second {
			t.Errorf("Run took %v after a 500ms timeout", took)
		}
		gc := grandchildPID(t, out.Stdout)
		if !waitGone(gc, 3*time.Second) {
			t.Errorf("grandchild %d survived the timeout", gc)
		}
	})

	t.Run("SIDE-18 cancellation returns quickly and stops the child", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(100*time.Millisecond, cancel)
		start := time.Now()
		_, err := Run(ctx, spec("sleep"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run = %v, want Canceled", err)
		}
		if took := time.Since(start); took > 3*time.Second {
			t.Errorf("Run took %v after cancellation", took)
		}
	})

	t.Run("an already cancelled context starts nothing", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := Run(ctx, spec("sleep")); !errors.Is(err, context.Canceled) {
			t.Fatalf("Run = %v, want Canceled", err)
		}
	})
}

func grandchildPID(t *testing.T, stdout []byte) int {
	t.Helper()
	_, rest, ok := strings.Cut(string(stdout), "gc=")
	if !ok {
		t.Fatalf("child did not report a grandchild: %q", stdout)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0]))
	if err != nil {
		t.Fatalf("grandchild pid in %q: %v", stdout, err)
	}
	return pid
}
