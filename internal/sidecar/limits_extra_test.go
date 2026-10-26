package sidecar

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCappedKeepsExactlyMax(t *testing.T) {
	for _, tc := range []struct {
		name    string
		writes  []string
		want    string
		dropped bool
	}{
		{"nothing", nil, "", false},
		{"under", []string{"abc"}, "abc", false},
		{"exactly the cap", []string{"abcde"}, "abcde", false},
		{"one byte over", []string{"abcdef"}, "abcde", true},
		{"two writes reaching the cap", []string{"abc", "de"}, "abcde", false},
		{"two writes one byte over", []string{"abc", "def"}, "abcde", true},
		{"a write after the cap", []string{"abcde", "f"}, "abcde", true},
		{"an empty write after the cap", []string{"abcde", ""}, "abcde", false},
		{"far over", []string{strings.Repeat("x", 1000)}, "xxxxx", true},
	} {
		c := &capped{max: 5}
		for _, w := range tc.writes {
			// The child must never see a short write: it would block or fail.
			if n, err := c.Write([]byte(w)); n != len(w) || err != nil {
				t.Errorf("%s: Write(%q) = %d, %v; want a full write", tc.name, w, n, err)
			}
		}
		if string(c.buf) != tc.want || c.dropped != tc.dropped {
			t.Errorf("%s: kept %q dropped=%t; want %q dropped=%t", tc.name, c.buf, c.dropped, tc.want, tc.dropped)
		}
	}
}

func TestDirBytes(t *testing.T) {
	dir := t.TempDir()
	if got := dirBytes(dir); got != 0 {
		t.Errorf("dirBytes(empty) = %d, want 0", got)
	}
	for name, n := range map[string]int{"a": 10, "sub/b": 20, "sub/deep/c": 30} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := dirBytes(dir); got != 60 {
		t.Errorf("dirBytes = %d, want 60 (folders themselves do not count)", got)
	}
	if got := dirBytes(filepath.Join(dir, "missing")); got != 0 {
		t.Errorf("dirBytes(missing) = %d, want 0", got)
	}
}

// TestWithLimitsRoundsUp: a limit is never lowered by rounding to the shell's
// blocks, and an unset one is not applied.
func TestWithLimitsRoundsUp(t *testing.T) {
	path, args, err := withLimits(Spec{Path: "/bin/true", Args: []string{"x"}})
	if err != nil || path != "/bin/true" || !slices.Equal(args, []string{"x"}) {
		t.Fatalf("withLimits(no limits) = %q, %q, %v; want the program as is", path, args, err)
	}
	for _, tc := range []struct {
		spec Spec
		want []string // lines of the script
		not  string
	}{
		{Spec{MemoryLimit: 1}, []string{"ulimit -v 1 || exit 126"}, "ulimit -f"},
		{Spec{MemoryLimit: 1024}, []string{"ulimit -v 1 || exit 126"}, "ulimit -f"},
		{Spec{MemoryLimit: 1025}, []string{"ulimit -v 2 || exit 126"}, "ulimit -f"},
		{Spec{FileSizeLimit: 1}, []string{"ulimit -f 1 || exit 126"}, "ulimit -v"},
		{Spec{FileSizeLimit: 512}, []string{"ulimit -f 1 || exit 126"}, "ulimit -v"},
		{Spec{FileSizeLimit: 513}, []string{"ulimit -f 2 || exit 126"}, "ulimit -v"},
		{Spec{MemoryLimit: 2048, FileSizeLimit: 1024}, []string{"ulimit -v 2 || exit 126", "ulimit -f 2 || exit 126"}, ""},
	} {
		tc.spec.Path, tc.spec.Args = "/bin/prog", []string{"a", "b"}
		path, args, err := withLimits(tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		if path != shell || len(args) < 6 || args[0] != "-c" || !slices.Equal(args[3:], []string{"/bin/prog", "a", "b"}) {
			t.Errorf("withLimits(%+v) = %q %q; want the shell running the program with its arguments", tc.spec, path, args)
			continue
		}
		script := args[1]
		for _, line := range tc.want {
			if !strings.Contains(script, line+"\n") {
				t.Errorf("withLimits(%+v): script %q lacks %q", tc.spec, script, line)
			}
		}
		if tc.not != "" && strings.Contains(script, tc.not) {
			t.Errorf("withLimits(%+v): script %q applies a limit that was not asked for", tc.spec, script)
		}
		if !strings.HasSuffix(script, `exec "$@"`) {
			t.Errorf("withLimits(%+v): script %q does not end by running the program", tc.spec, script)
		}
	}
}

func TestSupervisorDefaultsAreExact(t *testing.T) {
	d := SupervisorConfig{}.withDefaults()
	if d.Backoff.Initial != defaultInitialBackoff || d.Backoff.Max != defaultMaxBackoff || d.Backoff.Factor != 2 ||
		d.StableAfter != 30*time.Second || d.StopGrace != 3*time.Second || d.MaxFailures != 10 {
		t.Errorf("zero config defaults = %+v", d)
	}
	for _, tc := range []struct {
		name string
		in   SupervisorConfig
		got  func(SupervisorConfig) any
		want any
	}{
		{"max below initial", SupervisorConfig{Backoff: Backoff{Initial: 50 * time.Millisecond, Max: 5 * time.Millisecond}}, func(c SupervisorConfig) any { return c.Backoff.Max }, 50 * time.Millisecond},
		{"max kept", SupervisorConfig{Backoff: Backoff{Initial: 5 * time.Millisecond, Max: 50 * time.Millisecond}}, func(c SupervisorConfig) any { return c.Backoff.Max }, 50 * time.Millisecond},
		{"negative initial", SupervisorConfig{Backoff: Backoff{Initial: -1}}, func(c SupervisorConfig) any { return c.Backoff.Initial }, defaultInitialBackoff},
		{"negative max", SupervisorConfig{Backoff: Backoff{Initial: time.Second, Max: -1}}, func(c SupervisorConfig) any { return c.Backoff.Max }, defaultMaxBackoff},
		{"factor 1 is kept", SupervisorConfig{Backoff: Backoff{Factor: 1}}, func(c SupervisorConfig) any { return c.Backoff.Factor }, 1.0},
		{"factor under 1 is replaced", SupervisorConfig{Backoff: Backoff{Factor: 0.5}}, func(c SupervisorConfig) any { return c.Backoff.Factor }, 2.0},
		{"factor 3 is kept", SupervisorConfig{Backoff: Backoff{Factor: 3}}, func(c SupervisorConfig) any { return c.Backoff.Factor }, 3.0},
		{"negative max failures means never", SupervisorConfig{MaxFailures: -1}, func(c SupervisorConfig) any { return c.MaxFailures }, -1},
		{"one failure", SupervisorConfig{MaxFailures: 1}, func(c SupervisorConfig) any { return c.MaxFailures }, 1},
		{"negative stable", SupervisorConfig{StableAfter: -1}, func(c SupervisorConfig) any { return c.StableAfter }, 30 * time.Second},
		{"negative grace", SupervisorConfig{StopGrace: -1}, func(c SupervisorConfig) any { return c.StopGrace }, 3 * time.Second},
	} {
		if got := tc.got(tc.in.withDefaults()); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSupervisorBackoffShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    Backoff
		want []time.Duration
	}{
		{"constant with factor 1", Backoff{Initial: 5 * time.Millisecond, Max: 50 * time.Millisecond, Factor: 1}, []time.Duration{5, 5, 5, 5}},
		{"tripling", Backoff{Initial: 2 * time.Millisecond, Max: 50 * time.Millisecond, Factor: 3}, []time.Duration{2, 6, 18, 50}},
		{"zero factor doubles", Backoff{Initial: 3 * time.Millisecond, Max: 50 * time.Millisecond}, []time.Duration{3, 6, 12, 24}},
		{"cap below the start", Backoff{Initial: 20 * time.Millisecond, Max: 5 * time.Millisecond}, []time.Duration{20, 20, 20, 20}},
	} {
		rec := &recorder{}
		var calls atomic.Int32
		_, _, wait := start(t, SupervisorConfig{Name: "shape", Command: script(&calls, "exit:1"), Backoff: tc.b, MaxFailures: 5, OnEvent: rec.on})
		if err := wait(); !errors.Is(err, ErrGaveUp) {
			t.Fatalf("%s: Run = %v, want ErrGaveUp", tc.name, err)
		}
		want := make([]time.Duration, len(tc.want))
		for i, d := range tc.want {
			want[i] = d * time.Millisecond
		}
		if got := delaysOf(rec.ofKind(EventBackoff)); !slices.Equal(got, want) {
			t.Errorf("%s: delays = %v, want %v", tc.name, got, want)
		}
	}
}

// TestSupervisorGivingUp: the default is ten consecutive failures, one gives up
// at once, and a negative number never gives up.
func TestSupervisorGivingUp(t *testing.T) {
	tiny := Backoff{Initial: time.Millisecond, Max: time.Millisecond, Factor: 1}
	var calls atomic.Int32
	rec := &recorder{}
	_, _, wait := start(t, SupervisorConfig{Name: "default", Command: script(&calls, "exit:1"), Backoff: tiny, OnEvent: rec.on})
	if err := wait(); !errors.Is(err, ErrGaveUp) || !strings.Contains(err.Error(), "10 consecutive failures") {
		t.Fatalf("Run = %v, want ErrGaveUp after 10 failures", err)
	}
	if calls.Load() != 10 || len(rec.ofKind(EventBackoff)) != 9 {
		t.Errorf("%d launches and %d pauses, want 10 and 9", calls.Load(), len(rec.ofKind(EventBackoff)))
	}

	calls.Store(0)
	rec = &recorder{}
	sup, _, wait := start(t, SupervisorConfig{Name: "one", Command: script(&calls, "exit:1"), Backoff: tiny, MaxFailures: 1, OnEvent: rec.on})
	if err := wait(); !errors.Is(err, ErrGaveUp) {
		t.Fatalf("Run = %v, want ErrGaveUp", err)
	}
	if calls.Load() != 1 || len(rec.ofKind(EventBackoff)) != 0 || sup.Restarts() != 0 {
		t.Errorf("%d launches, %d pauses, %d restarts; want 1, 0, 0", calls.Load(), len(rec.ofKind(EventBackoff)), sup.Restarts())
	}

	calls.Store(0)
	rec = &recorder{}
	_, stop, _ := start(t, SupervisorConfig{Name: "never", Command: script(&calls, "exit:1"), Backoff: tiny, MaxFailures: -1, OnEvent: rec.on})
	rec.waitKind(t, EventExit, 25)
	if err := stop(); err != nil {
		t.Errorf("Run = %v, want nil: a supervisor told never to give up stops only when cancelled", err)
	}
	if len(rec.ofKind(EventGiveUp)) != 0 {
		t.Error("a give-up event with MaxFailures < 0")
	}
}

// TestSupervisorStableRunForgivesFailures: a launch that runs for StableAfter
// starts the count over, so a program that keeps crashing only after a good
// run is never given up on.
func TestSupervisorStableRunForgivesFailures(t *testing.T) {
	rec := &recorder{}
	var calls atomic.Int32
	cfg := SupervisorConfig{
		Name: "stable", Command: script(&calls, "sleepms:150"), MaxFailures: 2, StableAfter: 80 * time.Millisecond,
		Backoff: Backoff{Initial: time.Millisecond, Max: time.Millisecond, Factor: 1}, OnEvent: rec.on,
	}
	_, stop, _ := start(t, cfg)
	rec.waitKind(t, EventExit, 4)
	if err := stop(); err != nil {
		t.Errorf("Run = %v, want nil: each 150 ms run is stable and forgives the failure before it", err)
	}

	// Without the forgiveness (StableAfter beyond the runs) the same program is given up on.
	calls.Store(0)
	cfg.StableAfter = time.Minute
	cfg.OnEvent = nil
	_, _, wait := start(t, cfg)
	if err := wait(); !errors.Is(err, ErrGaveUp) || calls.Load() != 2 {
		t.Errorf("Run = %v after %d launches, want ErrGaveUp after 2", err, calls.Load())
	}
}
