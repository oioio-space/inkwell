package sidecar

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recorder collects supervisor events.
type recorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recorder) on(e Event) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recorder) all() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

func (r *recorder) ofKind(k EventKind) []Event {
	var out []Event
	for _, e := range r.all() {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

// waitKind waits until n events of kind k were recorded and returns them.
func (r *recorder) waitKind(t *testing.T, k EventKind, n int) []Event {
	t.Helper()
	// Generous: the first launch of a real sidecar extracts 77 MB, which the
	// race detector makes slow.
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if got := r.ofKind(k); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %v events, have %v", n, k, r.all())
	return nil
}

// script returns a Command that launches the helper in the given modes, one
// per launch, then keeps launching the last one. It counts its calls.
func script(calls *atomic.Int32, modes ...string) func(context.Context) (*exec.Cmd, error) {
	return func(context.Context) (*exec.Cmd, error) {
		i := int(calls.Add(1)) - 1
		return helperCmd(modes[min(i, len(modes)-1)]), nil
	}
}

type result struct {
	err  error
	done chan struct{}
}

// start runs the supervisor and returns a stop function that cancels it and
// waits for Run to return.
func start(t *testing.T, cfg SupervisorConfig) (sup *Supervisor, stop, wait func() error) {
	t.Helper()
	sup = NewSupervisor(cfg)
	ctx, cancel := context.WithCancel(t.Context())
	res := &result{done: make(chan struct{})}
	go func() {
		res.err = sup.Run(ctx)
		close(res.done)
	}()
	wait = func() error {
		select {
		case <-res.done:
		case <-time.After(10 * time.Second):
			t.Fatal("Run did not return")
		}
		return res.err
	}
	stop = func() error {
		cancel()
		return wait()
	}
	t.Cleanup(func() { _ = stop() })
	return sup, stop, wait
}

var quick = Backoff{Initial: 5 * time.Millisecond, Max: 20 * time.Millisecond, Factor: 2}

func TestSupervisor(t *testing.T) {
	t.Run("SIDE-20 runs until cancelled, then the child is gone", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		_, stop, _ := start(t, SupervisorConfig{Name: "sleeper", Command: script(&calls, "sleep"), Backoff: quick, OnEvent: rec.on})
		pid := rec.waitKind(t, EventStart, 1)[0].PID
		if !alive(pid) {
			t.Fatalf("child %d is not running", pid)
		}
		if err := stop(); err != nil {
			t.Errorf("Run = %v after cancellation, want nil", err)
		}
		if !waitGone(pid, 3*time.Second) {
			t.Errorf("child %d survived the cancellation", pid)
		}
	})

	t.Run("SIDE-21 a crash is followed by a restart", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		sup, _, _ := start(t, SupervisorConfig{Name: "crashy", Command: script(&calls, "exit:7", "sleep"), Backoff: quick, OnEvent: rec.on})
		starts := rec.waitKind(t, EventStart, 2)
		if starts[0].PID == starts[1].PID {
			t.Errorf("restart reused pid %d", starts[0].PID)
		}
		exits := rec.waitKind(t, EventExit, 1)
		if exits[0].ExitCode != 7 {
			t.Errorf("first exit code = %d, want 7", exits[0].ExitCode)
		}
		if got := sup.Restarts(); got != 1 {
			t.Errorf("Restarts() = %d, want 1", got)
		}
	})

	t.Run("SIDE-22 backoff grows, is capped and resets after a stable run", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		cfg := SupervisorConfig{
			Name:        "flaky",
			Command:     script(&calls, "exit:1", "exit:1", "sleepms:150", "exit:1", "sleep"),
			Backoff:     Backoff{Initial: 20 * time.Millisecond, Max: 80 * time.Millisecond, Factor: 2},
			StableAfter: 100 * time.Millisecond,
			OnEvent:     rec.on,
		}
		start(t, cfg) // stopped by the cleanup
		delays := delaysOf(rec.waitKind(t, EventBackoff, 4)[:4])
		want := []time.Duration{20 * time.Millisecond, 40 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
		if fmt.Sprint(delays) != fmt.Sprint(want) {
			t.Errorf("backoff delays = %v, want %v (the 150ms run is stable and resets the delay)", delays, want)
		}
	})

	t.Run("SIDE-22 the delay never exceeds the cap", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		cfg := SupervisorConfig{
			Name: "capped", Command: script(&calls, "exit:1"), OnEvent: rec.on, MaxFailures: 6,
			Backoff: Backoff{Initial: 10 * time.Millisecond, Max: 40 * time.Millisecond, Factor: 2},
		}
		_, _, wait := start(t, cfg)
		if err := wait(); !errors.Is(err, ErrGaveUp) {
			t.Fatalf("Run = %v, want ErrGaveUp", err)
		}
		delays := delaysOf(rec.ofKind(EventBackoff))
		want := []time.Duration{10, 20, 40, 40, 40}
		for i := range want {
			want[i] *= time.Millisecond
		}
		if fmt.Sprint(delays) != fmt.Sprint(want) {
			t.Errorf("delays = %v, want %v", delays, want)
		}
	})

	t.Run("SIDE-23 gives up after N consecutive failures", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		_, _, wait := start(t, SupervisorConfig{Name: "dead", Command: script(&calls, "exit:1"), Backoff: quick, MaxFailures: 3, OnEvent: rec.on})
		err := wait()
		if !errors.Is(err, ErrGaveUp) {
			t.Fatalf("Run = %v, want ErrGaveUp", err)
		}
		if got := calls.Load(); got != 3 {
			t.Errorf("launched %d times, want 3", got)
		}
		if got := len(rec.ofKind(EventGiveUp)); got != 1 {
			t.Errorf("%d give-up events, want 1", got)
		}
	})

	t.Run("SIDE-24 a failed integrity check launches nothing and counts as a failure", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		cfg := SupervisorConfig{
			Name: "tampered", Backoff: quick, MaxFailures: 2, OnEvent: rec.on,
			Command: func(context.Context) (*exec.Cmd, error) {
				calls.Add(1)
				return nil, fmt.Errorf("verify: %w", ErrIntegrity)
			},
		}
		_, _, wait := start(t, cfg)
		err := wait()
		if !errors.Is(err, ErrGaveUp) || !errors.Is(err, ErrIntegrity) {
			t.Fatalf("Run = %v, want ErrGaveUp wrapping ErrIntegrity", err)
		}
		if got := len(rec.ofKind(EventStart)); got != 0 {
			t.Errorf("%d processes were started although verification failed", got)
		}
		if got := len(rec.ofKind(EventPrepareFailed)); got != 2 || calls.Load() != 2 {
			t.Errorf("%d prepare-failed events and %d calls, want 2 and 2", got, calls.Load())
		}
	})

	t.Run("SIDE-25 Command runs again before every launch", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		start(t, SupervisorConfig{Name: "reverify", Command: script(&calls, "exit:1", "exit:1", "exit:1", "sleep"), Backoff: quick, OnEvent: rec.on})
		rec.waitKind(t, EventStart, 4)
		if got := calls.Load(); got != 4 {
			t.Errorf("Command called %d times for 4 launches, want 4", got)
		}
	})

	t.Run("SIDE-27 the whole process group stops with the supervisor", func(t *testing.T) {
		out := &syncBuffer{}
		cfg := SupervisorConfig{
			Name: "tree", Backoff: quick,
			Command: func(context.Context) (*exec.Cmd, error) {
				c := helperCmd("grandchild")
				c.Stdout = out
				return c, nil
			},
		}
		rec := &recorder{}
		cfg.OnEvent = rec.on
		_, stop, _ := start(t, cfg)
		pid := rec.waitKind(t, EventStart, 1)[0].PID
		gc := waitGrandchild(t, out)
		if err := stop(); err != nil {
			t.Errorf("Run = %v", err)
		}
		if !waitGone(pid, 3*time.Second) || !waitGone(gc, 3*time.Second) {
			t.Errorf("child %d or grandchild %d survived", pid, gc)
		}
	})

	t.Run("a child that ignores SIGTERM is killed after the grace period", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		_, stop, _ := start(t, SupervisorConfig{
			Name: "stubborn", Command: script(&calls, "ignoreterm"), Backoff: quick,
			StopGrace: 200 * time.Millisecond, OnEvent: rec.on,
		})
		pid := rec.waitKind(t, EventStart, 1)[0].PID
		time.Sleep(100 * time.Millisecond) // let it install the handler
		begin := time.Now()
		if err := stop(); err != nil {
			t.Errorf("Run = %v", err)
		}
		if took := time.Since(begin); took > 5*time.Second {
			t.Errorf("stopping took %v", took)
		}
		if !waitGone(pid, 3*time.Second) {
			t.Errorf("child %d survived SIGKILL", pid)
		}
	})

	t.Run("SIDE-28 events come in order", func(t *testing.T) {
		rec := &recorder{}
		var calls atomic.Int32
		_, stop, _ := start(t, SupervisorConfig{Name: "ordered", Command: script(&calls, "exit:2", "sleep"), Backoff: quick, OnEvent: rec.on})
		rec.waitKind(t, EventStart, 2)
		_ = stop()
		var kinds []string
		for _, e := range rec.all() {
			if e.Sidecar != "ordered" {
				t.Errorf("event %v names sidecar %q, want ordered", e, e.Sidecar)
			}
			kinds = append(kinds, e.Kind.String())
		}
		got := strings.Join(kinds, ",")
		if !strings.HasPrefix(got, "start,exit,backoff,start,exit") {
			t.Errorf("events = %s, want start,exit,backoff,start,exit...", got)
		}
	})

	t.Run("an already cancelled context launches nothing", func(t *testing.T) {
		var calls atomic.Int32
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		sup := NewSupervisor(SupervisorConfig{Name: "late", Command: script(&calls, "sleep")})
		if err := sup.Run(ctx); err != nil {
			t.Errorf("Run = %v, want nil", err)
		}
		if calls.Load() != 0 {
			t.Errorf("Command called %d times on a cancelled context", calls.Load())
		}
	})

	t.Run("a program that cannot start counts as a failure", func(t *testing.T) {
		rec := &recorder{}
		cfg := SupervisorConfig{
			Name: "ghost", Backoff: quick, MaxFailures: 2, OnEvent: rec.on,
			Command: func(context.Context) (*exec.Cmd, error) {
				return exec.Command("/nonexistent/sidecar"), nil
			},
		}
		_, _, wait := start(t, cfg)
		if err := wait(); !errors.Is(err, ErrGaveUp) {
			t.Fatalf("Run = %v, want ErrGaveUp", err)
		}
		if got := len(rec.ofKind(EventStartFailed)); got != 2 {
			t.Errorf("%d start-failed events, want 2", got)
		}
	})

	t.Run("a nil Command is refused", func(t *testing.T) {
		if err := NewSupervisor(SupervisorConfig{Name: "x"}).Run(t.Context()); err == nil {
			t.Fatal("Run without Command = nil, want an error")
		}
	})

	t.Run("defaults fill the zero configuration", func(t *testing.T) {
		cfg := SupervisorConfig{}.withDefaults()
		if cfg.Backoff.Initial <= 0 || cfg.Backoff.Max < cfg.Backoff.Initial || cfg.Backoff.Factor < 1 ||
			cfg.StableAfter <= 0 || cfg.MaxFailures <= 0 || cfg.StopGrace <= 0 {
			t.Errorf("defaults = %+v, want positive values", cfg)
		}
	})
}

func TestEventKindString(t *testing.T) {
	for k, want := range map[EventKind]string{
		EventStart: "start", EventExit: "exit", EventBackoff: "backoff",
		EventGiveUp: "give-up", EventPrepareFailed: "prepare-failed", EventStartFailed: "start-failed",
		EventKind(99): "EventKind(99)",
	} {
		if got := k.String(); got != want {
			t.Errorf("EventKind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}

// SIDE-26: the supervising process is killed without a chance to clean up; the
// kernel must take the child with it.
func TestSupervisorNoOrphanAfterParentDies(t *testing.T) {
	if !orphanSafe {
		t.Skip("this platform has no parent-death signal (see docs/plans/2026-10-25-etape-3-typst.md, D4)")
	}
	parent := exec.CommandContext(t.Context(), os.Args[0]) // #nosec G204 -- re-executes the test binary
	parent.Env = helperEnv("supervise-forever")
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		_ = parent.Process.Kill()
		_ = parent.Wait()
		t.Fatalf("reading the child pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "child=")))
	if err != nil {
		t.Fatalf("child pid in %q: %v", line, err)
	}
	if !alive(pid) {
		t.Fatalf("child %d is not running before the parent dies", pid)
	}

	if err := parent.Process.Kill(); err != nil { // SIGKILL: no deferred cleanup runs
		t.Fatal(err)
	}
	_ = parent.Wait()
	if !waitGone(pid, 5*time.Second) {
		_ = syscallKill(pid)
		t.Fatalf("child %d is still running after its supervisor was killed: orphan", pid)
	}
}

// superviseForever is the body of the "supervise-forever" helper: it runs a
// sleeping child under a Supervisor, reports its pid, and never returns.
func superviseForever() int {
	sup := NewSupervisor(SupervisorConfig{
		Name:    "orphan-test",
		Command: func(context.Context) (*exec.Cmd, error) { return helperCmd("sleep"), nil },
		OnEvent: func(e Event) {
			if e.Kind == EventStart {
				fmt.Printf("child=%d\n", e.PID)
			}
		},
	})
	_ = sup.Run(context.Background())
	return 0
}

func waitDone(t *testing.T, stop func() error) error {
	t.Helper()
	// Run returns by itself when it gives up; stop only joins it.
	return stop()
}

func delaysOf(events []Event) []time.Duration {
	out := make([]time.Duration, len(events))
	for i, e := range events {
		out[i] = e.Delay
	}
	return out
}

// syncBuffer is a bytes.Buffer safe for the exec copying goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitGrandchild(t *testing.T, b *syncBuffer) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := b.String(); strings.Contains(s, "gc=") && strings.Contains(s, "\n") {
			return grandchildPID(t, []byte(s))
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the child never reported its grandchild")
	return 0
}
