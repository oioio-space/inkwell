package main

import (
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/instance"
)

// TestSupersedeGivesBackOnlyOldGeneration: rule D-OWNER (plan, amendment 2026-11-13; model
// OISingleInstance, Reconnect and EndStream). A new connection supersedes the old one: what the old
// one was given and had not reported is sent again on the new one, in order; a report of the old
// generation is ignored; the late end of the old stream gives back nothing the new one holds.
func TestSupersedeGivesBackOnlyOldGeneration(t *testing.T) {
	q := newOpenQueue()
	q.open()
	for i := range 3 {
		q.push(instance.Request{Cwd: fmt.Sprint(i)})
	}
	g1 := q.connect()
	var old []*openEntry
	for range 3 {
		e, ok := q.take(g1)
		if !ok {
			t.Fatal("take refused on the current generation")
		}
		old = append(old, e)
	}
	if !q.handled(g1, old[0].seq) {
		t.Fatal("a report of the generation holding the request was ignored")
	}
	g2 := q.connect()
	if _, ok := q.take(g1); ok {
		t.Fatal("a superseded stream was given a request")
	}
	again, ok := q.take(g2)
	if !ok || again.seq != old[1].seq {
		t.Fatalf("take on the new stream = %v, %v; want the oldest unhandled of the old one", again, ok)
	}
	if q.handled(g1, old[1].seq) || q.handled(g1, old[2].seq) {
		t.Error("a report of a superseded stream removed a request")
	}
	if q.handled(g2, old[0].seq) || q.handled(g2, 999) {
		t.Error("a report about a request not held removed something")
	}
	q.release(g1) // the old stream's handler returns late
	next, ok := q.take(g2)
	if !ok || next.seq != old[2].seq {
		t.Fatalf("after the old stream ended, take = %v, %v; want the last unhandled, once", next, ok)
	}
	if _, ok := q.take(g2); ok {
		t.Error("the end of a superseded stream gave back what the new one holds: sent twice")
	}
	if !q.handled(g2, old[1].seq) || !q.handled(g2, old[2].seq) || q.pending() != 0 {
		t.Errorf("reports of the current stream not taken: %d left", q.pending())
	}
	q.push(instance.Request{Cwd: "late"})
	q.release(g2) // the current stream ends: what it holds goes back, for the next connection
	g3 := q.connect()
	if e, ok := q.take(g3); !ok || e.req.Cwd != "late" {
		t.Errorf("after the current stream ended, take = %v, %v; want the request it held", e, ok)
	}
}

// ownerReport is a handled report on its way from a window to Go.
type ownerReport struct{ gen, seq uint64 }

// ownerEntry is a request of the reference queue: its number and the generation holding it.
type ownerEntry struct{ seq, gen uint64 }

// ownerModel is the reference of the open queue, computed from a history alone (plan rule D-OWNER,
// model OISingleInstance): a launch is admitted while fewer than openQueueSize are held and numbered
// in admission order; each connection is the next generation and gives back what the previous one
// held; a stream's end gives back what it held; the current generation gets the oldest request not
// delivered (FIFO) while fewer than maxInFlight are in flight; a report frees a request only from
// the generation holding it.
type ownerModel struct {
	items    []ownerEntry
	seq, gen uint64
}

func (m *ownerModel) push() (uint64, bool) {
	if len(m.items) >= openQueueSize {
		return 0, false
	}
	m.seq++
	m.items = append(m.items, ownerEntry{seq: m.seq})
	return m.seq, true
}

func (m *ownerModel) giveBack(g uint64) {
	for i := range m.items {
		if m.items[i].gen == g {
			m.items[i].gen = 0
		}
	}
}

func (m *ownerModel) connect() uint64 {
	m.giveBack(m.gen)
	m.gen++
	return m.gen
}

func (m *ownerModel) take(g uint64) (uint64, bool) {
	if g != m.gen {
		return 0, false
	}
	inFlight := 0
	for _, e := range m.items {
		if e.gen != 0 {
			inFlight++
		}
	}
	if inFlight >= maxInFlight {
		return 0, false
	}
	for i := range m.items {
		if m.items[i].gen == 0 {
			m.items[i].gen = g
			return m.items[i].seq, true
		}
	}
	return 0, false
}

func (m *ownerModel) report(g, seq uint64) bool {
	for i, e := range m.items {
		if e.seq == seq && e.gen == g && g != 0 {
			m.items = slices.Delete(m.items, i, i+1)
			return true
		}
	}
	return false
}

// queueState is the open queue's requests, in order, with the generation holding each.
func queueState(q *openQueue) []ownerEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]ownerEntry, 0, len(q.items))
	for _, e := range q.items {
		out = append(out, ownerEntry{e.seq, e.gen})
	}
	return out
}

// TestOpenQueueOwnerHistories (rule 16, property): over generated histories of launches (some
// filling the queue), connections, ends of streams (late ones included), deliveries, reports (stale,
// duplicate and forged ones included), the open queue does exactly what the reference model computed
// from the history alone does: the same admissions and refusals, the same numbers, the same delivery
// order per generation, the same give-backs and the same accepted reports, compared at every step.
// The window side checks one owner at a time (model invariant SlotsAgree): what the current window
// holds, Go holds as delivered to it. And no slot stays taken (model property NoStrandedSlot): the
// history ends by draining, half of the time on the current connection without any reconnection
// (which would give back, and so hide, what an earlier step stranded).
func TestOpenQueueOwnerHistories(t *testing.T) {
	var refused, stale, plainDrains int
	for seed := range uint64(400) {
		r := rand.New(rand.NewPCG(seed, 0x4f49)) // #nosec G404 -- reproducible test histories
		q := newOpenQueue()
		q.open()
		var m ownerModel
		var cur uint64            // the current window's generation, 0 when none
		live := []uint64{}        // streams whose handler has not returned
		held := map[uint64]bool{} // what the current window holds (got, not reported)
		var reports []ownerReport // on their way, any order
		fail := func(step int, format string, args ...any) {
			t.Helper()
			t.Fatalf("seed %d step %d: %s", seed, step, fmt.Sprintf(format, args...))
		}
		report := func(step int, rep ownerReport) {
			t.Helper()
			want := m.report(rep.gen, rep.seq)
			if got := q.handled(rep.gen, rep.seq); got != want {
				fail(step, "report %+v accepted %v, the reference %v", rep, got, want)
			}
			if !want && slices.ContainsFunc(m.items, func(e ownerEntry) bool { return e.seq == rep.seq }) {
				stale++
			}
		}
		take := func(step int) bool {
			t.Helper()
			want, wantOK := m.take(cur)
			e, ok := q.take(cur)
			if ok != wantOK || (ok && e.seq != want) {
				fail(step, "take on generation %d = %v (%v), the reference %d (%v)", cur, e, ok, want, wantOK)
			}
			if ok {
				if held[e.seq] {
					fail(step, "request %d sent twice to one window", e.seq)
				}
				held[e.seq] = true
			}
			return ok
		}
		pushWeight := int(seed%3) * 6 // a third of the histories mostly launch, and fill the queue
		for step := range 300 {
			op := r.IntN(7 + pushWeight)
			switch {
			case op == 0 || op >= 7:
				wantSeq, want := m.push()
				got := q.push(instance.Request{})
				if got != want {
					fail(step, "push admitted %v, the reference %v (%d held)", got, want, len(m.items))
				}
				if !want {
					refused++
				} else if st := queueState(q); st[len(st)-1].seq != wantSeq {
					fail(step, "push numbered %d, the reference %d", st[len(st)-1].seq, wantSeq)
				}
			case op == 1:
				want := m.connect()
				if cur = q.connect(); cur != want {
					fail(step, "connect = generation %d, the reference %d", cur, want)
				}
				live = append(live, cur)
				clear(held) // a fresh window; the old one's reports stay on their way
			case op == 2:
				if len(live) == 0 {
					continue
				}
				i := r.IntN(len(live))
				g := live[i]
				live = slices.Delete(live, i, i+1)
				m.giveBack(g)
				q.release(g)
				if g == cur {
					cur = 0
					clear(held)
				}
			case op == 3 || op == 4:
				if cur != 0 {
					take(step)
				}
			case op == 5:
				if len(held) == 0 {
					continue
				}
				s := slices.Sorted(maps.Keys(held))[r.IntN(len(held))]
				delete(held, s)
				reports = append(reports, ownerReport{cur, s})
			case op == 6:
				var rep ownerReport
				if len(reports) > 0 && r.IntN(4) > 0 {
					i := r.IntN(len(reports))
					rep = reports[i]
					if r.IntN(3) > 0 {
						reports = slices.Delete(reports, i, i+1) // else it comes again (a duplicate)
					}
				} else {
					rep = ownerReport{uint64(r.IntN(4)), uint64(r.IntN(80))} // about nothing held, or another generation
					if rep.gen == cur && held[rep.seq] {
						continue // the window reports only what it handled (its stream has no other sender, ADR 0009)
					}
				}
				report(step, rep)
			}
			if got := queueState(q); !slices.Equal(got, m.items) {
				fail(step, "queue %v, the reference %v", got, m.items)
			}
			for s := range held {
				if !slices.Contains(m.items, ownerEntry{s, cur}) {
					fail(step, "the window holds %d, which Go does not hold as delivered to generation %d (SlotsAgree)", s, cur)
				}
			}
		}
		// No slot stays taken: the reports on their way arrive, then the window handles what it holds
		// and what it gets.
		for _, rep := range reports {
			report(999, rep)
		}
		if seed%2 == 0 && cur != 0 {
			plainDrains++ // on the current connection: no reconnection gives anything back
		} else {
			want := m.connect()
			if cur = q.connect(); cur != want {
				t.Fatalf("seed %d: final connect = generation %d, the reference %d", seed, cur, want)
			}
			clear(held)
		}
		for step := 1000; ; step++ {
			for _, s := range slices.Sorted(maps.Keys(held)) {
				report(step, ownerReport{cur, s})
				delete(held, s)
			}
			if !take(step) && len(held) == 0 {
				break
			}
		}
		if n := q.pending(); n != 0 || len(m.items) != 0 {
			t.Fatalf("seed %d: %d requests stranded (the reference %d) after the window handled what it got", seed, n, len(m.items))
		}
	}
	if refused == 0 || stale == 0 || plainDrains == 0 {
		t.Errorf("histories too weak: %d refusals, %d ignored reports of held requests, %d drains without reconnection", refused, stale, plainDrains)
	}
}

// TestOpenerSupersedes: a new connection of the window supersedes the old one, which Go closes; the
// requests the old one had not reported go to the new one, once.
func TestOpenerSupersedes(t *testing.T) {
	_, cur := projectDocs(t)
	q := newOpenQueue()
	q.open()
	o := &opener{queue: q, env: openEnv(cur), raise: func(string) {}, log: slog.New(slog.DiscardHandler)}
	q.push(instance.Request{Cwd: cur, Args: []string{"ch/a.typ"}})
	q.push(instance.Request{Cwd: cur, Args: []string{"ch/a.typ"}})
	w1 := newFakeWindow(t.Context())
	done1 := make(chan struct{})
	go func() { o.Serve(w1); close(done1) }()
	w1.waitFrames(t, 2)
	w1.acks <- []byte(`{"handled":1}`)
	for end := time.Now().Add(5 * time.Second); q.pending() != 1; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the report of the current stream was not taken")
		}
	}
	w2 := newFakeWindow(t.Context())
	done2 := make(chan struct{})
	go func() { o.Serve(w2); close(done2) }()
	select {
	case <-done1:
	case <-time.After(5 * time.Second):
		t.Fatal("the superseded stream was not closed")
	}
	if !w1.isClosed() {
		t.Error("Go did not close the superseded stream")
	}
	q.push(instance.Request{Cwd: cur, Args: []string{"ch/a.typ"}})
	w2.waitFrames(t, 2)
	time.Sleep(50 * time.Millisecond) // a duplicate, if any, would be sent by now
	w2.mu.Lock()
	got := slices.Clone(w2.sent)
	w2.mu.Unlock()
	want := []string{`{"id":2,"show":"ch/a.typ"}`, `{"id":3,"show":"ch/a.typ"}`}
	if !slices.Equal(got, want) {
		t.Errorf("the new stream got %q, want %q", got, want)
	}
	w2.acks <- []byte(`{"handled":2}`)
	w2.acks <- []byte(`{"handled":3}`)
	if left := q.waitHandled(2 * time.Second); left != 0 {
		t.Errorf("%d requests left after the new stream reported them", left)
	}
	_ = w2.Close()
	<-done2
}
