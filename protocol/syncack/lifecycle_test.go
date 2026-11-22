// SPDX-License-Identifier: MPL-2.0

package syncack

import (
	"fmt"
	"testing"
	"time"
)

// lifecycleIn builds a lifecycle in state s, its ACK written at 0 (peer up and turn as given for Waiting and Kept, with
// prefix probed bytes for Kept).
func lifecycleIn(t *testing.T, s InState) *Lifecycle {
	t.Helper()
	l := &Lifecycle{}
	if s == Arrived {
		return l
	}
	l.Arrive(s != Refused)
	switch s {
	case Refused, Accepting:
	case Gone:
		l.Ack(false, 0)
	case Waiting:
		l.Ack(true, 0)
	case Kept:
		l.Ack(true, 0)
		l.Read(1, ReadOK, time.Millisecond)
		l.Read(0, ReadEOF, 250*time.Millisecond)
	case Delivering, Blocked:
		l.Ack(true, 0)
		l.Up()
		l.Turn()
		if s == Blocked {
			l.Budget(false)
		}
	}
	if l.State() != s {
		t.Fatalf("setup: state %v, want %v", l.State(), s)
	}
	return l
}

// TestLifecycle is rules P-10 to P-12 (spec §8.7.3): every state × every observation.
func TestLifecycle(t *testing.T) {
	type obs struct {
		name string
		do   func(l *Lifecycle) bool
	}
	read := func(n int, e ReadEnd, at time.Duration) obs {
		return obs{fmt.Sprintf("read/%d/%d/%v", n, e, at), func(l *Lifecycle) bool { return l.Read(n, e, at) }}
	}
	observations := []obs{
		{"arrive/placed", func(l *Lifecycle) bool { l.Arrive(true); return false }},
		{"arrive/refused", func(l *Lifecycle) bool { l.Arrive(false); return false }},
		{"ack/ok", func(l *Lifecycle) bool { return l.Ack(true, time.Second) }},
		{"ack/failed", func(l *Lifecycle) bool { return l.Ack(false, time.Second) }},
		read(0, ReadOK, time.Second), read(1, ReadOK, time.Second), read(0, ReadDeadline, time.Second),
		read(1, ReadDeadline, time.Second), read(0, ReadDeadline, UpWait), read(0, ReadEOF, time.Second),
		read(1, ReadEOF, time.Second), read(0, ReadReset, time.Second), read(1, ReadReset, time.Second),
		read(0, ReadConnLost, time.Second), read(0, ReadNodeClosed, time.Second),
		{"turn", func(l *Lifecycle) bool { l.Turn(); return false }},
		{"up", func(l *Lifecycle) bool { l.Up(); return false }},
		{"tick/before", func(l *Lifecycle) bool { return l.Tick(UpWait - time.Nanosecond) }},
		{"tick/at", func(l *Lifecycle) bool { return l.Tick(UpWait) }},
		{"budget/room", func(l *Lifecycle) bool { l.Budget(true); return false }},
		{"budget/full", func(l *Lifecycle) bool { l.Budget(false); return false }},
		{"conn_lost/own", func(l *Lifecycle) bool { return l.ConnLost(true) }},
		{"conn_lost/other", func(l *Lifecycle) bool { return l.ConnLost(false) }},
		{"decoded", func(l *Lifecycle) bool { return l.Decoded() }},
	}
	// want[state][observation] = the state after it, "!" marking a leave; an absent observation keeps the state.
	type out struct {
		s     InState
		leave bool
	}
	gone := out{Gone, true}
	want := map[InState]map[string]out{
		Arrived:   {"arrive/placed": {Accepting, false}, "arrive/refused": {Refused, false}},
		Refused:   {},
		Accepting: {"ack/ok": {Waiting, false}, "ack/failed": gone, "conn_lost/own": gone},
		Waiting: {
			"read/1/2/1s": {Kept, false}, "read/0/2/1s": gone, "read/0/1/30s": gone, "tick/at": gone,
			"read/0/3/1s": gone, "read/1/3/1s": gone, "read/0/4/1s": gone, "read/0/5/1s": gone, "conn_lost/own": gone,
		},
		Kept: {"read/0/1/30s": gone, "tick/at": gone, "conn_lost/own": gone},
		Delivering: {
			"budget/full": {Blocked, false}, "read/0/2/1s": gone, "read/1/2/1s": gone, "read/0/3/1s": gone,
			"read/1/3/1s": gone, "read/0/4/1s": gone, "read/0/5/1s": gone, "conn_lost/own": gone,
		},
		Blocked: {"budget/room": {Delivering, false}, "conn_lost/own": gone},
		Gone:    {},
	}
	for s := Arrived; s <= Gone; s++ {
		for _, o := range observations {
			t.Run(s.String()+"/"+o.name, func(t *testing.T) {
				l := lifecycleIn(t, s)
				w, ok := want[s][o.name]
				if !ok {
					w = out{s, false}
				}
				if leave := o.do(l); leave != w.leave || l.State() != w.s {
					t.Errorf("%v then %s: %v (leave %t), want %v (leave %t)", s, o.name, l.State(), leave, w.s, w.leave)
				}
				if l.State() == Gone && l.Prefix() != 0 {
					t.Errorf("a stream gone holds %d probed bytes", l.Prefix())
				}
			})
		}
	}

	// The probed bytes are kept in order before the error is looked at; a kept stream holds them to its turn,
	// waits for its peer's announcement, then delivers and leaves once decoded.
	t.Run("prefix_to_turn", func(t *testing.T) {
		l := lifecycleIn(t, Waiting)
		l.Read(1, ReadDeadline, 250*time.Millisecond)
		l.Read(1, ReadOK, 500*time.Millisecond)
		l.Read(1, ReadEOF, 750*time.Millisecond)
		if l.State() != Kept || l.Prefix() != 3 {
			t.Fatalf("after three probed bytes and an end: %v, prefix %d; want kept, 3", l.State(), l.Prefix())
		}
		if l.Turn(); l.State() != Kept {
			t.Fatalf("turn without the peer up: %v, want kept", l.State())
		}
		if l.Read(0, ReadEOF, time.Second); l.State() != Kept {
			t.Fatalf("a kept stream read again: %v, want kept", l.State())
		}
		if l.Up(); l.State() != Delivering || l.Prefix() != 3 {
			t.Fatalf("turn and peer up: %v, prefix %d; want delivering, 3", l.State(), l.Prefix())
		}
		if !l.Decoded() || l.State() != Gone {
			t.Fatalf("decoded: %v, want gone", l.State())
		}
	})
	// A stream that was not kept goes on reading once its prefix is decoded; the up-wait counts from the ACK.
	t.Run("decoded_goes_on", func(t *testing.T) {
		l := &Lifecycle{}
		l.Arrive(true)
		l.Ack(true, 10*time.Second)
		l.Read(1, ReadOK, 11*time.Second)
		if l.Tick(10*time.Second + UpWait - time.Nanosecond) {
			t.Fatal("the up-wait counted from before the ACK")
		}
		l.Up()
		l.Turn()
		if l.Decoded() || l.State() != Delivering {
			t.Fatalf("decoded, not kept: %v, want delivering", l.State())
		}
		if l.Tick(time.Hour) {
			t.Fatal("a delivering stream left on a tick")
		}
	})
	t.Run("names", func(t *testing.T) {
		if Gone.String() != "gone" || InState(99).String() != "state(99)" || (Gone+1).String() != "state(8)" {
			t.Errorf("names: %q, %q, %q", Gone, InState(99), Gone+1)
		}
		for name, e := range readEndNames {
			if got, ok := ParseReadEnd(name); !ok || got != e {
				t.Errorf("ParseReadEnd(%q) = %v, %t", name, got, ok)
			}
		}
		if _, ok := ParseReadEnd("fin"); ok {
			t.Error(`ParseReadEnd("fin") succeeded`)
		}
	})
}

// TestProbeSchedule is rule P-11: an attempt every ProbeEvery, at most one byte per attempt window, at most ProbeMax kept.
func TestProbeSchedule(t *testing.T) {
	var p ProbeSchedule
	for i := range ProbeMax {
		at := time.Duration(i) * ProbeEvery
		if err := p.Attempt(at); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if err := p.Byte(at); err != nil {
			t.Fatalf("byte %d: %v", i, err)
		}
	}
	if err := p.Attempt(ProbeMax * ProbeEvery); err != nil {
		t.Fatalf("attempt after ProbeMax bytes: %v", err)
	}
	if err := p.Byte(ProbeMax * ProbeEvery); err == nil {
		t.Error("byte ProbeMax+1 accepted")
	}
	var first ProbeSchedule
	if err := first.Attempt(time.Hour); err != nil {
		t.Errorf("a first attempt, whenever it starts: %v", err)
	}
	var late ProbeSchedule
	_ = late.Attempt(0)
	if err := late.Attempt(ProbeEvery + time.Nanosecond); err == nil {
		t.Error("an attempt ProbeEvery+1ns after the previous one accepted")
	}
	var early ProbeSchedule
	_ = early.Attempt(0)
	if err := early.Attempt(ProbeEvery - time.Nanosecond); err == nil {
		t.Error("an attempt before the next window accepted")
	}
	t.Run("one_byte_per_window", func(t *testing.T) {
		var schedule ProbeSchedule
		start := time.Second
		if err := schedule.Attempt(start); err != nil {
			t.Fatal(err)
		}
		if err := schedule.Byte(start + ProbeEvery - time.Nanosecond); err != nil {
			t.Fatalf("first byte near the end of the first window: %v", err)
		}
		if err := schedule.Attempt(start + ProbeEvery); err != nil {
			t.Fatal(err)
		}
		if err := schedule.Byte(start + ProbeEvery); err != nil {
			t.Fatalf("first byte of the next window: %v", err)
		}
		if err := schedule.Byte(start + 2*ProbeEvery - time.Nanosecond); err == nil {
			t.Error("two bytes within one attempt window accepted")
		}
	})
	t.Run("byte_requires_started_window", func(t *testing.T) {
		var schedule ProbeSchedule
		start := time.Second
		if err := schedule.Attempt(start); err != nil {
			t.Fatal(err)
		}
		for _, at := range []time.Duration{start - time.Nanosecond, start + ProbeEvery} {
			if err := schedule.Byte(at); err == nil {
				t.Errorf("byte at %v outside the started window accepted", at)
			}
		}
	})
}

// TestQUICWindow is rule P-13's inequality: at the bound, one byte under, and the node's windows.
func TestQUICWindow(t *testing.T) {
	const w0 = 128 << 10
	for _, c := range []struct {
		conn uint64
		ok   bool
	}{{1 << 20, true}, {873814, true}, {873813, false}, {768 << 10, false}} {
		if got := QUICWindowOK(2, 2, w0, c.conn, 4); got != c.ok {
			t.Errorf("QUICWindowOK(2, 2, %d, %d, 4) = %t, want %t", w0, c.conn, got, c.ok)
		}
	}
	if QUICWindowOK(2, 2, 512<<10, 768<<10, 4) {
		t.Error("quic-go's default windows pass")
	}
	// Exactly at the bound: (2 × 2 + 1) × 3 × 4 = 60 = 3 × 20.
	if !QUICWindowOK(2, 2, 3, 20, 4) || QUICWindowOK(2, 2, 3, 19, 4) {
		t.Error("the bound is not ≤ exactly")
	}
	if QUICWindowOK(2, 2, w0, 1<<20, 0) {
		t.Error("a zero denominator passes")
	}
	if !QUICWindowOK(Places, 2, 128<<10, 1<<20, 4) {
		t.Error("the node's windows fail")
	}
}
