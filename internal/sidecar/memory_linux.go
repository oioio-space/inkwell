package sidecar

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Samples retain monotonic time. A failed read/overflow poisons the entire watch.
type memorySample struct {
	at               time.Time
	oom, kills, high uint64
}

// memoryHistory is the run's counter timeline. bounded records that the
// watchdog killed the run itself once it went over memory.high (ADR 0044 1g).
type memoryHistory struct {
	samples                      []memorySample
	globalDelta                  uint64
	ancestorOOM, failed, bounded bool
}

func (h memoryHistory) cause(signal int) MemoryCause {
	if h.failed || len(h.samples) == 0 {
		return MemoryUnknown
	}
	for i := 1; i < len(h.samples); i++ {
		if h.samples[i].oom < h.samples[i-1].oom || h.samples[i].kills < h.samples[i-1].kills {
			return MemoryUnknown
		}
	}
	first, last := h.samples[0], h.samples[len(h.samples)-1]
	if last.kills < first.kills || last.oom < first.oom {
		return MemoryUnknown
	}
	kills := last.kills - first.kills
	if kills == 0 {
		switch {
		case !h.bounded:
			return MemoryNone
		case signal == int(unix.SIGKILL):
			return MemoryLocalLimit
		default:
			return MemoryUnknown
		}
	}
	if signal != int(unix.SIGKILL) || h.ancestorOOM || h.globalDelta != kills {
		return MemoryVictim
	}
	var local time.Time
	prev := first
	for _, s := range h.samples[1:] {
		if s.oom < prev.oom || s.kills < prev.kills {
			return MemoryUnknown
		}
		if s.oom > prev.oom {
			local = s.at
		}
		if s.kills > prev.kills {
			if local.IsZero() || s.at.Sub(local) > time.Second {
				return MemoryVictim
			}
			return MemoryLocalLimit
		}
		prev = s
	}
	return MemoryVictim
}

func readCounters(path string) (map[string]uint64, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- fixed counter name under our own cgroup/ancestor path, or fixed /proc/vmstat.
	if err != nil {
		return nil, err
	}
	values := map[string]uint64{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, errors.New("invalid counter record")
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			return nil, err
		}
		if _, ok := values[f[0]]; ok {
			return nil, errors.New("duplicate counter")
		}
		values[f[0]] = n
	}
	return values, nil
}

func readMemorySample(path string) (memorySample, error) {
	v, err := readCounters(filepath.Join(path, "memory.events.local"))
	// "high" is read when present: without it the watchdog never fires and
	// the memory.max backstop is what bounds the run.
	s := memorySample{at: time.Now(), oom: v["oom"], kills: v["oom_kill"], high: v["high"]}
	if _, ok := v["oom"]; !ok {
		err = errors.Join(err, errors.New("missing oom"))
	}
	if _, ok := v["oom_kill"]; !ok {
		err = errors.Join(err, errors.New("missing oom_kill"))
	}
	return s, err
}

func globalKills() (uint64, error) {
	v, err := readCounters("/proc/vmstat")
	n, ok := v["oom_kill"]
	if !ok {
		err = errors.Join(err, errors.New("missing global oom_kill"))
	}
	return n, err
}

type memoryWatch struct {
	beforeRead  func()      // test seam at the final counter read, before cleanup
	disarmed    atomic.Bool // test seam: no watchdog kill, the memory.max backstop acts (R-7)
	path        string
	fd          int
	done, ended chan struct{}
	once        sync.Once
	h           memoryHistory
	global      uint64
	ancestors   map[string]uint64
	bound       uint64    // memory.high of the run; zero leaves only the backstop
	over        time.Time // first check, since the last one at or under the bound, that found unreclaimable memory above it
}

// boundGrace is how long the run's unreclaimable memory must stay above
// memory.high before the watchdog kills: it absorbs the moment between the
// reads of memory.current and memory.stat (ADR 0044 point 1g).
const boundGrace = 100 * time.Millisecond

// unreclaimable is the part of a run's charge that reclaim cannot give back
// without swap and without the run's cooperation: memory.current less the
// clean, evictable page cache of files. memory.stat "file" holds tmpfs and
// shared memory ("shmem"), dirty and writeback pages, which wait on writes,
// and mlocked file pages ("unevictable"; it holds mlocked anonymous pages
// too, which are then counted twice, on the side of the bound): all stay
// counted. Clean mapped file pages are left out: reclaim unmaps them itself
// (measured 2026-11-16: a run mapping a 512 MiB file under a 128 MiB bound
// keeps 18 MiB unreclaimable). The page cache alone is not a bound violation,
// even while it stays above memory.high: pages under read I/O cannot be
// reclaimed until the I/O completes, and on storage at 200 MB/s or less a
// plain read stays above the bound for 85 ms to 200 ms at a time (measured
// 2026-11-16, ADR 0044 point 1g). What is left out is still charged: memory.max
// stays its hard backstop.
func unreclaimable(current uint64, stat map[string]uint64) (uint64, error) {
	var v [5]uint64
	for i, name := range [...]string{"file", "shmem", "file_dirty", "file_writeback", "unevictable"} {
		n, ok := stat[name]
		if !ok {
			return 0, fmt.Errorf("memory.stat has no %s", name)
		}
		v[i] = n
	}
	file, kept := v[0], v[1]+v[2]+v[3]+v[4]
	clean := file - min(kept, file)
	return current - min(clean, current), nil
}

// usage reads the run's unreclaimable memory now.
func (w *memoryWatch) usage() (uint64, error) {
	v, err := readValue(filepath.Join(w.path, "memory.current"))
	if err != nil {
		return 0, err
	}
	current, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, err
	}
	stat, err := readCounters(filepath.Join(w.path, "memory.stat"))
	if err != nil {
		return 0, err
	}
	return unreclaimable(current, stat)
}

// watchCounterFile opens a non-blocking inotify queue on the counter file;
// a variable so that tests can make inotify fail.
var watchCounterFile = func(file string) (int, error) {
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return -1, err
	}
	if _, err := unix.InotifyAddWatch(fd, file, unix.IN_MODIFY|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

func startMemoryWatch(path, scope string) *memoryWatch {
	w := &memoryWatch{
		path:      path,
		fd:        -1,
		done:      make(chan struct{}),
		ended:     make(chan struct{}),
		ancestors: map[string]uint64{},
	}
	var err error
	if w.fd, err = watchCounterFile(filepath.Join(path, "memory.events.local")); err != nil {
		w.fd, w.h.failed = -1, true
	}
	w.global, err = globalKills()
	w.h.failed = w.h.failed || err != nil
	for p := scope; ; p = filepath.Dir(p) {
		s, err := readMemorySample(p)
		w.h.failed = w.h.failed || err != nil
		w.ancestors[p] = s.oom
		if filepath.Base(p) == fmt.Sprintf("user@%d.service", os.Getuid()) {
			break
		}
		if p == "/sys/fs/cgroup" || p == "/" {
			w.h.failed = true
			break
		}
	}
	if v, err := readValue(filepath.Join(path, "memory.high")); err == nil {
		w.bound, _ = strconv.ParseUint(v, 10, 64) // "max" or unreadable: no watchdog, the backstop bounds the run
	}
	w.sample()
	go func() {
		defer close(w.ended)
		for {
			select {
			case <-w.done:
				w.drain()
				return
			default:
			}
			w.wait(10 * time.Millisecond)
			if w.fd < 0 {
				// No inotify queue: sample at the same cadence, so that the
				// watchdog still acts and memory.max stays the backstop (S-11).
				w.sample()
			} else {
				w.drain()
			}
			w.enforce(time.Now())
		}
	}()
	return w
}

// wait blocks until the counters change or d passes: the watchdog must act
// on the first memory.high event, before the run reaches memory.max.
func (w *memoryWatch) wait(d time.Duration) {
	if w.fd < 0 || w.fd > math.MaxInt32 {
		time.Sleep(d)
		return
	}
	p := []unix.PollFd{{Fd: int32(w.fd), Events: unix.POLLIN}}
	_, _ = unix.Poll(p, int(d.Milliseconds()))
}

func (w *memoryWatch) sample() {
	s, err := readMemorySample(w.path)
	w.h.failed = w.h.failed || err != nil
	if len(w.h.samples) < 4096 {
		w.h.samples = append(w.h.samples, s)
	} else {
		w.h.failed = true
		w.h.samples[len(w.h.samples)-1] = s
	}
}

// enforce is the watchdog of ADR 0044 point 1g, checked at most 10 ms apart
// while the run lives: memory.high throttles and reclaims but never invokes
// the OOM killer (kernel cgroup-v2 documentation, memory.high). Once the run
// has had a "high" event, inkwell kills the whole run cgroup itself when
// its unreclaimable memory stays above the bound for boundGrace, so a run
// that only filled the page cache is not stopped. A kill by inkwell is not an OOM kill:
// the session scope's Result stays "success" and the desktop raises no memory
// alarm. If the kill fails, memory.max above the bound is the backstop
// (residual R-7).
func (w *memoryWatch) enforce(now time.Time) {
	n := len(w.h.samples)
	if w.h.bounded || w.bound == 0 || w.disarmed.Load() || n == 0 || w.h.samples[n-1].high <= w.h.samples[0].high {
		return
	}
	used, err := w.usage()
	if err != nil || used <= w.bound {
		w.over = time.Time{}
		return
	}
	if w.over.IsZero() {
		w.over = now
	}
	if now.Sub(w.over) < boundGrace {
		return
	}
	if err := killRunCgroup(w.path, func(pid int) error { return killCgroupPID(w.path, pid) }); err == nil {
		w.h.bounded = true
	}
}

func (w *memoryWatch) drain() {
	if w.fd < 0 {
		return
	}
	var buf [4096]byte
	for {
		n, err := unix.Read(w.fd, buf[:])
		if errors.Is(err, unix.EAGAIN) {
			return
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil || n == 0 {
			w.h.failed = true
			return
		}
		for off := 0; off+unix.SizeofInotifyEvent <= n; {
			ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off])) // #nosec G103 -- kernel inotify ABI header; loop bounds ensure the full header is in the read buffer.
			if ev.Mask&(unix.IN_Q_OVERFLOW|unix.IN_IGNORED|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF) != 0 {
				w.h.failed = true
			}
			off += unix.SizeofInotifyEvent + int(ev.Len)
		}
		w.sample()
	}
}

func (w *memoryWatch) finish(signal int) MemoryCause {
	w.once.Do(func() {
		close(w.done)
		<-w.ended
		if w.fd >= 0 {
			_ = unix.Close(w.fd)
		}
		if w.beforeRead != nil {
			w.beforeRead()
		}
		w.sample()
		// Read both local and hierarchical counters before ANY cgroup cleanup. The
		// hierarchical values are not kept: only their readability is evidence here.
		if _, err := readCounters(filepath.Join(w.path, "memory.events")); err != nil {
			w.h.failed = true
		}
		g, err := globalKills()
		if err != nil || g < w.global {
			w.h.failed = true
		} else {
			w.h.globalDelta = g - w.global
		}
		for p, initial := range w.ancestors {
			s, err := readMemorySample(p)
			if err != nil || s.oom < initial {
				w.h.failed = true
			}
			w.h.ancestorOOM = w.h.ancestorOOM || s.oom > initial
		}
	})
	return w.h.cause(signal)
}
