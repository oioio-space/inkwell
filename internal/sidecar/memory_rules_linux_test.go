package sidecar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These tests prove the memory attribution rules of ADR 0042 (rows M4, M8, M9
// and R7 of docs/plans/2026-11-11-sandbox-crash-reports.md) on ordinary files:
// a cause is evidence, so whatever cannot be read gives "unknown", never zero.

func counterFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.events.local")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// R7: a counter file is read whole or not at all; a malformed, negative or
// repeated record is a failed read, not a zero.
func TestCounterFileReadWholeOrRefused(t *testing.T) {
	got, err := readCounters(counterFile(t, "low 0\nhigh 3\nmax 12\noom 2\noom_kill 1\noom_group_kill 1\n"))
	if err != nil || len(got) != 6 || got["oom"] != 2 || got["oom_kill"] != 1 || got["max"] != 12 {
		t.Fatalf("readCounters = %v, %v", got, err)
	}
	for name, content := range map[string]string{
		"name without a value": "oom 1\noom_kill\n",
		"three fields":         "oom 1 2\n",
		"not a number":         "oom many\n",
		"negative":             "oom_kill -1\n",
		"beyond 64 bits":       "oom 18446744073709551616\n",
		"repeated counter":     "oom 1\noom_kill 0\noom 0\n",
		"empty file":           "",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := readCounters(counterFile(t, content)); err == nil {
				t.Fatalf("readCounters accepted %q as %v", content, got)
			}
		})
	}
	if _, err := readCounters(filepath.Join(t.TempDir(), "memory.events.local")); !os.IsNotExist(err) {
		t.Fatalf("missing counter file: %v", err)
	}
}

// R7 and M4: a sample needs both oom and oom_kill; a file without one of them
// is unreadable evidence, not "no kill".
func TestMemorySampleNeedsBothCounters(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		ok            bool
		oom, kills    uint64
	}{
		{"both", "low 0\noom 4\noom_kill 3\n", true, 4, 3},
		{"oom_kill missing", "low 0\noom 4\n", false, 0, 0},
		{"oom missing", "low 0\noom_kill 3\n", false, 0, 0},
		{"malformed", "oom 4\noom_kill three\n", false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := readMemorySample(filepath.Dir(counterFile(t, tc.content)))
			if (err == nil) != tc.ok {
				t.Fatalf("readMemorySample = %+v, %v", s, err)
			}
			if tc.ok && (s.oom != tc.oom || s.kills != tc.kills || s.at.IsZero()) {
				t.Fatalf("sample = %+v, want oom %d, kills %d and a time", s, tc.oom, tc.kills)
			}
		})
	}
}

// M4: kernel event counters only grow, so a timeline in which one goes back
// (a recreated or foreign cgroup) is not evidence of anything.
func TestMemoryCauseRefusesRegressingCounters(t *testing.T) {
	now := time.Now()
	at := func(ms int) time.Time { return now.Add(time.Duration(ms) * time.Millisecond) }
	for name, samples := range map[string][]memorySample{
		"oom goes back":              {{at: at(0), oom: 2}, {at: at(1), oom: 1}, {at: at(2), oom: 3, kills: 1}},
		"oom_kill goes back":         {{at: at(0), kills: 2}, {at: at(1), oom: 1, kills: 1}, {at: at(2), oom: 1, kills: 3}},
		"ends below the start":       {{at: at(0), oom: 1, kills: 1}, {at: at(1), oom: 1, kills: 0}},
		"oom goes back, no kill":     {{at: at(0), oom: 2}, {at: at(1), oom: 1}, {at: at(2), oom: 2}},
		"oom_kill dips, no net kill": {{at: at(0), kills: 2}, {at: at(1), kills: 1}, {at: at(2), kills: 2}},
		"no sample was taken":        {},
		"regression then a kill":     {{at: at(0), oom: 5}, {at: at(1), oom: 0}, {at: at(2), oom: 6, kills: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			h := memoryHistory{samples: samples, globalDelta: 1}
			if got := h.cause(int(unix.SIGKILL)); got != MemoryUnknown {
				t.Fatalf("cause = %s, want unknown", got)
			}
		})
	}
}

// M4 and M8: a local OOM kill is attributed to the run's own limit only when
// every ancestor up to user@<uid>.service could be read; a watch that reaches
// the root without it has no such evidence and answers "unknown".
func TestMemoryWatchWithoutAncestorEvidenceIsUnknown(t *testing.T) {
	dir := filepath.Dir(counterFile(t, "oom 1\noom_kill 1\n"))
	if err := os.WriteFile(filepath.Join(dir, "memory.events"), []byte("oom 1\noom_kill 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := startMemoryWatch(dir, dir)
	if got := w.finish(int(unix.SIGKILL)); got != MemoryUnknown {
		t.Fatalf("cause outside a user manager cgroup = %s, want unknown", got)
	}
	if got := w.finish(0); got != MemoryUnknown {
		t.Fatalf("second finish = %s, want the same poisoned answer", got)
	}

	missing := filepath.Join(t.TempDir(), "run-1")
	if got := startMemoryWatch(missing, missing).finish(int(unix.SIGKILL)); got != MemoryUnknown {
		t.Fatalf("cause of a run whose counters do not exist = %s, want unknown", got)
	}
}

// M9: the timeline is bounded; once it is full the watch can no longer order
// the events, so the whole attribution is poisoned rather than guessed.
func TestMemoryTimelineOverflowPoisons(t *testing.T) {
	dir := filepath.Dir(counterFile(t, "oom 0\noom_kill 0\n"))
	w := &memoryWatch{path: dir, fd: -1}
	w.h.samples = make([]memorySample, 4095)
	w.sample()
	if w.h.failed || len(w.h.samples) != 4096 || w.h.cause(0) != MemoryNone {
		t.Fatalf("timeline of %d samples: failed=%v cause=%s", len(w.h.samples), w.h.failed, w.h.cause(0))
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.events.local"), []byte("oom 1\noom_kill 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.sample()
	last := w.h.samples[len(w.h.samples)-1]
	if !w.h.failed || len(w.h.samples) != 4096 || last.kills != 1 {
		t.Fatalf("overflow: failed=%v, %d samples, last %+v; want a poisoned, bounded timeline holding the latest read", w.h.failed, len(w.h.samples), last)
	}
	if got := w.h.cause(int(unix.SIGKILL)); got != MemoryUnknown {
		t.Fatalf("cause after an overflow = %s, want unknown", got)
	}
}

// M9: a failed read of the event queue, or the loss of the watched file,
// during the watch poisons the attribution; an event read in order does not.
func TestMemoryWatchFailurePoisons(t *testing.T) {
	watch := func(t *testing.T) (*memoryWatch, string) {
		t.Helper()
		file := counterFile(t, "oom 0\noom_kill 0\n")
		fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = unix.Close(fd) })
		if _, err := unix.InotifyAddWatch(fd, file, unix.IN_MODIFY|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF); err != nil {
			t.Fatal(err)
		}
		return &memoryWatch{path: filepath.Dir(file), fd: fd}, file
	}
	t.Run("event read in order", func(t *testing.T) {
		w, file := watch(t)
		w.drain()
		if w.h.failed || len(w.h.samples) != 0 {
			t.Fatalf("idle queue: failed=%v, %d samples", w.h.failed, len(w.h.samples))
		}
		if err := os.WriteFile(file, []byte("oom 1\noom_kill 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		w.drain()
		if w.h.failed || len(w.h.samples) == 0 || w.h.samples[len(w.h.samples)-1].kills != 1 {
			t.Fatalf("after a counter change: failed=%v samples=%+v", w.h.failed, w.h.samples)
		}
	})
	t.Run("watched file removed", func(t *testing.T) {
		w, file := watch(t)
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		w.drain()
		if !w.h.failed {
			t.Fatal("the loss of memory.events.local did not poison the watch")
		}
	})
	t.Run("event queue unreadable", func(t *testing.T) {
		r, wr, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close(); _ = wr.Close() }()
		// Reading the write end of a pipe fails with EBADF, like a lost queue.
		w := &memoryWatch{path: t.TempDir(), fd: int(wr.Fd())}
		w.drain()
		if !w.h.failed || w.h.cause(int(unix.SIGKILL)) != MemoryUnknown {
			t.Fatalf("failed queue read: failed=%v cause=%s", w.h.failed, w.h.cause(int(unix.SIGKILL)))
		}
	})
	t.Run("event queue closed", func(t *testing.T) {
		r, wr, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close() }()
		_ = wr.Close() // a read that returns 0 bytes: the queue is gone
		w := &memoryWatch{path: t.TempDir(), fd: int(r.Fd())}
		w.drain()
		if !w.h.failed {
			t.Fatal("an ended event queue did not poison the watch")
		}
	})
}
