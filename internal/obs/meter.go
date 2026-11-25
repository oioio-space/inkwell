package obs

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Meter holds the instruments Go declares in code, the single source of its
// own descriptors. An instrument's Add is called inline, allocates nothing and
// takes no lock (rule Z-1); a nil instrument, or one declared on a nil Meter,
// does nothing. A [Store] drains a Meter at each flush.
type Meter struct {
	mu  sync.Mutex
	ins []*instrument
}

// instrument is the state shared by the three kinds: one sum and one count of
// measures per declared value (index 0 when the metric has none).
type instrument struct {
	d     Descriptor
	sums  []atomic.Uint64
	ns    []atomic.Uint64
	index map[string]int // a value's slot; read only once declared
}

// Counter counts events.
type Counter struct{ in *instrument }

// Volume counts an amount, in bytes or words.
type Volume struct{ in *instrument }

// Duration sums measured times, in milliseconds, and counts them.
type Duration struct{ in *instrument }

// declare adds an instrument for d, which must be a valid application metric
// of kind and not taken: a declaration in code that breaks this is a
// programming error, so it panics. On a nil Meter it returns nil, an
// instrument that counts nothing.
func (m *Meter) declare(d Descriptor, kind string) *instrument {
	if err := d.Validate(); err != nil || d.Kind != kind || d.Scope != ScopeApp {
		panic(fmt.Sprintf("obs: bad declaration of %s: kind %s, scope %s, %v", d.ID, d.Kind, d.Scope, err))
	}
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if slices.ContainsFunc(m.ins, func(in *instrument) bool { return in.d.ID == d.ID }) {
		panic("obs: metric " + d.ID + " declared twice")
	}
	slots := max(len(d.Values), 1)
	in := &instrument{d: d, sums: make([]atomic.Uint64, slots), ns: make([]atomic.Uint64, slots), index: map[string]int{}}
	for i, v := range d.Values {
		in.index[v] = i
	}
	m.ins = append(m.ins, in)
	return in
}

// Counter declares a counter (unit count).
func (m *Meter) Counter(d Descriptor) *Counter {
	d.Kind, d.Unit = KindCounter, "count"
	return &Counter{m.declare(d, KindCounter)}
}

// Volume declares a volume; d.Unit is bytes or words.
func (m *Meter) Volume(d Descriptor) *Volume {
	d.Kind = KindVolume
	return &Volume{m.declare(d, KindVolume)}
}

// Duration declares a duration (unit ms).
func (m *Meter) Duration(d Descriptor) *Duration {
	d.Kind, d.Unit = KindDuration, "ms"
	return &Duration{m.declare(d, KindDuration)}
}

// Descriptors returns what the Meter declares, in declaration order.
func (m *Meter) Descriptors() []Descriptor {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Descriptor, len(m.ins))
	for i, in := range m.ins {
		out[i] = in.d
	}
	return out
}

// slot is the index of value by, or -1 for a value not declared.
func (in *instrument) slot(by string) int {
	if by == "" && len(in.d.Values) == 0 {
		return 0
	}
	if i, ok := in.index[by]; ok {
		return i
	}
	return -1
}

func (in *instrument) add(by string, v, n uint64) {
	if in == nil {
		return
	}
	if i := in.slot(by); i >= 0 {
		in.sums[i].Add(v)
		in.ns[i].Add(n)
	}
}

// Add counts n events.
func (c *Counter) Add(n uint64) {
	if c != nil {
		c.in.add("", n, 0)
	}
}

// AddBy counts n events of a declared value; another value is ignored.
func (c *Counter) AddBy(by string, n uint64) {
	if c != nil {
		c.in.add(by, n, 0)
	}
}

// Add adds n bytes or words.
func (v *Volume) Add(n uint64) {
	if v != nil {
		v.in.add("", n, 0)
	}
}

// AddBy adds n bytes or words of a declared value; another value is ignored.
func (v *Volume) AddBy(by string, n uint64) {
	if v != nil {
		v.in.add(by, n, 0)
	}
}

// Observe adds one measured time, rounded down to the millisecond.
func (d *Duration) Observe(t time.Duration) {
	if d != nil && t >= 0 {
		d.in.add("", uint64(t.Milliseconds()), 1) // #nosec G115 -- t >= 0 checked just above.
	}
}

// drain returns the deltas the Meter's instruments hold, and zeroes them.
// A sum above the bound of one delta is split into several deltas.
func (m *Meter) drain() []Delta {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	ins := slices.Clone(m.ins)
	m.mu.Unlock()
	var out []Delta
	for _, in := range ins {
		for i := range in.sums {
			v, n := in.sums[i].Swap(0), in.ns[i].Swap(0)
			if v == 0 && n == 0 {
				continue
			}
			by := ""
			if len(in.d.Values) > 0 {
				by = in.d.Values[i]
			}
			for first := true; first || v > 0; first = false {
				part := min(v, in.d.Max)
				out = append(out, Delta{M: in.d.ID, By: by, V: float64(part), N: float64(n)})
				v, n = v-part, 0
			}
		}
	}
	return out
}
