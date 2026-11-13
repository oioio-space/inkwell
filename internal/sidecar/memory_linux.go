package sidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Samples retain monotonic time. A failed read/overflow poisons the entire watch.
type memorySample struct {
	at         time.Time
	oom, kills uint64
}
type memoryHistory struct {
	samples             []memorySample
	globalDelta         uint64
	ancestorOOM, failed bool
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
		return MemoryNone
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
	s := memorySample{at: time.Now(), oom: v["oom"], kills: v["oom_kill"]}
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
	beforeRead  func() // test seam at the final counter read, before cleanup
	path        string
	fd          int
	done, ended chan struct{}
	once        sync.Once
	h           memoryHistory
	global      uint64
	ancestors   map[string]uint64
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
	w.fd, err = unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		w.h.failed = true
	}
	if w.fd >= 0 {
		if _, err = unix.InotifyAddWatch(w.fd, filepath.Join(path, "memory.events.local"), unix.IN_MODIFY|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF); err != nil {
			w.h.failed = true
		}
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
	w.sample()
	go func() {
		defer close(w.ended)
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-w.done:
				w.drain()
				return
			case <-tick.C:
				w.drain()
			}
		}
	}()
	return w
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
