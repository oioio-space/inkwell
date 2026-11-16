package sidecar

import (
	"fmt"
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

// S-11 (ADR 0044 point 1g): without inotify the watchdog still acts, on
// memory.events.local sampled at the same 10 ms cadence, and kills only once
// memory.current has stayed above memory.high for boundGrace; memory.max stays
// the backstop, never the normal path. The cgroup is ordinary files here, so
// the kill shows as the write of cgroup.kill.
func TestMemoryWatchdogWithoutInotify(t *testing.T) {
	watch := watchCounterFile
	t.Cleanup(func() { watchCounterFile = watch })
	watchCounterFile = func(string) (int, error) { return -1, unix.EMFILE }

	dir := filepath.Dir(counterFile(t, "high 0\noom 0\noom_kill 0\n"))
	for name, value := range map[string]string{"memory.high": "100\n", "memory.current": "200\n", "memory.stat": "anon 200\nfile 0\nshmem 0\nfile_dirty 0\nfile_writeback 0\nunevictable 0\n", "cgroup.kill": ""} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	w := startMemoryWatch(dir, dir)
	over := time.Now()
	if err := os.WriteFile(filepath.Join(dir, "memory.events.local"), []byte("high 1\noom 0\noom_kill 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var killed time.Time
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if b, _ := os.ReadFile(filepath.Join(dir, "cgroup.kill")); string(b) == "1" {
			killed = time.Now()
			break
		}
	}
	w.finish(int(unix.SIGKILL))
	if killed.IsZero() || !w.h.bounded {
		t.Fatalf("no watchdog kill without inotify (bounded=%v): memory.max would be the normal path", w.h.bounded)
	}
	if took := killed.Sub(over); took < boundGrace {
		t.Fatalf("killed %v after the bound was passed, before its %v grace", took, boundGrace)
	}
}

// S-11 (ADR 0044 point 1g): the bound is on what reclaim cannot give back.
// memory.current counts the page cache, which stays above memory.high for
// 85 ms to 200 ms at a time while a plain read waits on storage at 200 MB/s
// or less (measured 2026-11-16): file pages are not the run's to answer for,
// but tmpfs and shared memory (shmem, inside "file") and the rest are.
func TestUnreclaimableLeavesOutOnlyFileCache(t *testing.T) {
	const mib = 1 << 20
	for _, c := range []struct {
		name               string
		current, file, shm uint64
		want               uint64
	}{
		{"page cache", 200 * mib, 190 * mib, 0, 10 * mib},
		{"tmpfs", 200 * mib, 190 * mib, 190 * mib, 200 * mib},
		{"anonymous", 200 * mib, 0, 0, 200 * mib},
		{"part tmpfs", 200 * mib, 150 * mib, 50 * mib, 100 * mib},
		{"stat ahead of current", 100 * mib, 150 * mib, 0, 0},
	} {
		got, err := unreclaimable(c.current, map[string]uint64{"anon": 1, "file": c.file, "shmem": c.shm, "file_dirty": 0, "file_writeback": 0, "file_mapped": 0, "unevictable": 0})
		if err != nil || got != c.want {
			t.Errorf("S-11 page cache: %s: unreclaimable = %d, %v; want %d", c.name, got, err, c.want)
		}
	}
	// Only clean, evictable file pages are left out: dirty and writeback pages
	// wait on the run's own writes, unevictable ones (mlock) never go. Clean
	// mapped pages are reclaimed by unmapping them, without the run.
	for _, c := range []struct {
		name                            string
		dirty, writeback, unevict, mapd uint64
		want                            uint64
	}{
		{"dirty", 100 * mib, 0, 0, 0, 110 * mib},
		{"writeback", 0, 100 * mib, 0, 0, 110 * mib},
		{"mlocked file mapping", 0, 0, 100 * mib, 100 * mib, 110 * mib},
		{"clean mapped", 0, 0, 0, 190 * mib, 10 * mib},
		{"all three", 60 * mib, 60 * mib, 60 * mib, 0, 190 * mib},
		{"more kept than cache", 190 * mib, 190 * mib, 0, 0, 200 * mib},
	} {
		stat := map[string]uint64{"file": 190 * mib, "shmem": 0, "file_dirty": c.dirty, "file_writeback": c.writeback, "file_mapped": c.mapd, "unevictable": c.unevict}
		got, err := unreclaimable(200*mib, stat)
		if err != nil || got != c.want {
			t.Errorf("S-11 page cache: %s: unreclaimable = %d, %v; want %d", c.name, got, err, c.want)
		}
	}
	for _, stat := range []map[string]uint64{{"file": 1}, {"shmem": 1}, {}, {"file": 1, "shmem": 0, "file_dirty": 0, "file_writeback": 0}, {"file": 1, "shmem": 0, "unevictable": 0}} {
		if _, err := unreclaimable(1, stat); err == nil {
			t.Errorf("S-11 page cache: memory.stat %v without every counter gave a usage", stat)
		}
	}
}

// S-11: past memory.high for longer than boundGrace, a run held up only by
// its page cache is not killed; the same charge in tmpfs is.
func TestMemoryWatchdogSparesFileCache(t *testing.T) {
	const mib = 1 << 20
	for _, c := range []struct {
		name string
		stat string
		kill bool
	}{
		{"page cache", fmt.Sprintf("anon %d\nfile %d\nshmem 0\nfile_mapped %d\nfile_dirty 0\nfile_writeback 0\nunevictable 0\n", 10*mib, 190*mib, 190*mib), false},
		{"tmpfs", fmt.Sprintf("anon %d\nfile %d\nshmem %d\nfile_mapped 0\nfile_dirty 0\nfile_writeback 0\nunevictable 0\n", 10*mib, 190*mib, 190*mib), true},
		{"dirty file", fmt.Sprintf("anon %d\nfile %d\nshmem 0\nfile_mapped 0\nfile_dirty %d\nfile_writeback 0\nunevictable 0\n", 10*mib, 190*mib, 190*mib), true},
		{"mlocked file", fmt.Sprintf("anon %d\nfile %d\nshmem 0\nfile_mapped %d\nfile_dirty 0\nfile_writeback 0\nunevictable %d\n", 10*mib, 190*mib, 190*mib, 190*mib), true},
		{"no stat", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Dir(counterFile(t, "high 0\noom 0\noom_kill 0\n"))
			files := map[string]string{"memory.high": fmt.Sprint(128*mib, "\n"), "memory.current": fmt.Sprint(200*mib, "\n"), "cgroup.kill": ""}
			if c.stat != "" {
				files["memory.stat"] = c.stat
			}
			for name, value := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			w := startMemoryWatch(dir, dir)
			if err := os.WriteFile(filepath.Join(dir, "memory.events.local"), []byte("high 1\noom 0\noom_kill 0\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			killed := false
			for deadline := time.Now().Add(10 * boundGrace); time.Now().Before(deadline) && !killed; time.Sleep(time.Millisecond) {
				b, _ := os.ReadFile(filepath.Join(dir, "cgroup.kill"))
				killed = string(b) == "1"
			}
			w.finish(int(unix.SIGKILL))
			if killed != c.kill || w.h.bounded != c.kill {
				t.Fatalf("S-11 page cache: killed=%v bounded=%v after %v above memory.high, want %v", killed, w.h.bounded, 10*boundGrace, c.kill)
			}
		})
	}
}
