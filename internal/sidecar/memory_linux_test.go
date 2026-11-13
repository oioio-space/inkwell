package sidecar

import (
	"testing"
	"time"
)

func TestMemoryCauseFromEvents(t *testing.T) {
	for _, row := range []string{"M1", "M2", "M3", "M4", "M6", "M7", "M8", "M9"} {
		t.Run(row, func(t *testing.T) {
			now := time.Now()
			h := memoryHistory{samples: []memorySample{{at: now}, {at: now.Add(time.Millisecond), oom: 1}, {at: now.Add(2 * time.Millisecond), oom: 1, kills: 1}}, globalDelta: 1}
			want := MemoryLocalLimit
			signal := 9
			switch row {
			case "M2":
				h.samples = []memorySample{{at: now}}
				h.globalDelta = 0
				signal = 0
				want = MemoryNone
			case "M3":
				h.samples[1].oom = 0
				h.samples[2].oom = 0
				want = MemoryVictim
			case "M4":
				h.failed = true
				want = MemoryUnknown
			case "M6":
				h.samples[2].at = now.Add(2 * time.Second)
				want = MemoryVictim
			case "M7":
				h.globalDelta = 2
				want = MemoryVictim
			case "M8":
				h.ancestorOOM = true
				want = MemoryVictim
			case "M9":
				h.failed = true
				want = MemoryUnknown
			}
			if got := h.cause(signal); got != want {
				t.Fatalf("%s: %s want %s", row, got, want)
			}
		})
	}
}

func TestMemoryTimelineSameReadAndFailure(t *testing.T) {
	now := time.Now()
	h := memoryHistory{samples: []memorySample{{at: now}, {at: now.Add(time.Millisecond), oom: 1, kills: 1}}, globalDelta: 1}
	if h.cause(9) != MemoryLocalLimit {
		t.Fatal("same-read local OOM not attributed")
	}
	h.failed = true
	if h.cause(9) != MemoryUnknown {
		t.Fatal("failed read or overflow did not poison attribution")
	}
}
