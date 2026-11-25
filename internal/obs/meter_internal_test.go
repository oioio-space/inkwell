package obs

import (
	"testing"
	"time"
)

// TestOBS1Z1AddIsFree proves rule Z-1: an instrument's Add allocates nothing
// and takes no lock, a nil instrument included.
func TestOBS1Z1AddIsFree(t *testing.T) {
	var m Meter
	c := m.Counter(Descriptor{ID: "test.events", Scope: ScopeApp, Label: "test.events", Max: 1 << 40, Values: []string{"syntax", "timeout"}})
	v := m.Volume(Descriptor{ID: "test.bytes", Unit: "bytes", Scope: ScopeApp, Label: "test.bytes", Max: 1 << 40})
	d := m.Duration(Descriptor{ID: "test.time", Scope: ScopeApp, Label: "test.time", Max: 1 << 40})
	var nilCounter *Counter
	var nilVolume *Volume
	var nilDuration *Duration
	adds := map[string]func(){
		"Counter.Add":      func() { c.Add(1) },
		"Counter.AddBy":    func() { c.AddBy("timeout", 1) },
		"Volume.Add":       func() { v.Add(512) },
		"Duration.Observe": func() { d.Observe(3 * time.Millisecond) },
		"nil Counter":      func() { nilCounter.Add(1); nilCounter.AddBy("timeout", 1) },
		"nil Volume":       func() { nilVolume.Add(1) },
		"nil Duration":     func() { nilDuration.Observe(time.Second) },
	}
	for name, add := range adds {
		if n := testing.AllocsPerRun(1000, add); n != 0 {
			t.Errorf("%s allocates %v times per call, want 0", name, n)
		}
	}
	// With the Meter's lock held elsewhere, every Add still returns: none takes it.
	m.mu.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, add := range adds {
			add()
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("an Add waits for the Meter's lock")
	}
	m.mu.Unlock()
	got := map[string]Delta{}
	for _, x := range m.drain() {
		got[x.M+"/"+x.By] = x
	}
	const calls = 1001 + 1 // AllocsPerRun's runs and its warm-up, then the run under the lock
	want := map[string]Delta{
		"test.events/":        {}, // a metric split by values counts nothing without one
		"test.events/syntax":  {},
		"test.events/timeout": {M: "test.events", By: "timeout", V: calls},
		"test.bytes/":         {M: "test.bytes", V: 512 * calls},
		"test.time/":          {M: "test.time", V: 3 * calls, N: calls},
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("drained %s = %+v, want %+v", k, got[k], w)
		}
	}
	if c.in.sums[1].Load() != 0 || v.in.sums[0].Load() != 0 {
		t.Error("drain leaves the instruments' sums")
	}
}
