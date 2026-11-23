// SPDX-License-Identifier: MPL-2.0

package directup_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol/directup"
)

// The pure policy of block HP1 (plan docs/plans/2026-11-16-hp1-direct-upgrade.md, §3 and §4): Policy,
// Bucket, Slots, Classify, the address selection and the budgets of the two choke points, on a clock passed
// in. Each test names its rule (H-n) at the start of every failure text, so the Go mutants of
// scripts/netsim/mutants.toml (block HP1) are caught by their `expect`.

var t0 = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// noJitter keeps every due time exact.
func noJitter(d time.Duration) time.Duration { return d }

// relayed is the observation of a wanted peer whose session runs on the inbound circuit, ready.
func relayed() directup.Obs {
	return directup.Obs{Enabled: true, Wanted: true, Relayed: true, In: true, LocalReady: true}
}

// callAt steps p at now and reports the action it starts (None if none), starting it.
func callAt(p *directup.Policy, o directup.Obs, now time.Time) (directup.Action, uint64) {
	d := p.Step(o, now)
	if d.Action == directup.None {
		return directup.None, 0
	}
	return d.Action, p.Start(now)
}

// H-1: no decision for a peer that is not wanted, whose session is direct, with no relay, or in RelayOnly.
func TestPolicyActsOnlyWhenWantedAndRelayed(t *testing.T) {
	for name, o := range map[string]directup.Obs{
		"not wanted": {Enabled: true, Relayed: true, In: true, LocalReady: true},
		"direct":     {Enabled: true, Wanted: true, Direct: true, In: true, LocalReady: true},
		"disabled":   {Wanted: true, Relayed: true, In: true, LocalReady: true},
		"no session": {Enabled: true, Wanted: true, In: true, LocalReady: true},
	} {
		p := directup.NewPolicy(t0, noJitter)
		for i := range 100 {
			now := t0.Add(time.Duration(i) * time.Minute)
			if d := p.Step(o, now); d.Action != directup.None || !d.Wake.IsZero() {
				if name == "not wanted" {
					t.Fatalf("H-1 decision for a peer not wanted: %+v at +%d min", d, i)
				}
				t.Fatalf("H-1 decision for a peer %s: %+v at +%d min", name, d, i)
			}
		}
	}
	p := directup.NewPolicy(t0, noJitter)
	if a, _ := callAt(p, relayed(), t0); a != directup.Punch {
		t.Fatalf("H-1 a wanted relayed peer with an inbound circuit: action %v, want punch at once", a)
	}
}

// H-2: no call before localReady; remoteReady (E2, E3) advances the due time but is not a guard: without it
// the fast-ladder poll still calls, class Unspent, one attempt token.
func TestPolicyLocalReadyIsAGuardRemoteReadyATrigger(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	o := relayed()
	o.LocalReady = false
	for i := range 60 {
		if d := p.Step(o, t0.Add(time.Duration(i)*time.Second)); d.Action != directup.None {
			t.Fatalf("H-2 call before local ready: %+v at +%d s", d, i)
		}
	}
	// Our handler is registered (E4): a call at once, though the peer's support is unknown.
	now := t0.Add(time.Minute)
	p.Event(directup.E4, now)
	a, e := callAt(p, relayed(), now)
	if a != directup.Punch {
		t.Fatalf("H-2 after E4 the action is %v, want punch (remoteReady is not a guard)", a)
	}
	// The peer had no handler: the call is Unspent; the fast ladder polls again 1 s later.
	p.Result(e, directup.Unspent, now)
	if a, _ := callAt(p, relayed(), now.Add(999*time.Millisecond)); a != directup.None {
		t.Fatalf("H-2 the fast-ladder poll called before 1 s")
	}
	if a, e = callAt(p, relayed(), now.Add(time.Second)); a != directup.Punch {
		t.Fatalf("H-2 the fast-ladder poll did not call at 1 s without remoteReady: %v", a)
	}
	// E3 (the peer's push lists /libp2p/dcutr) during the next wait advances the due time to T_min after the
	// last start.
	p.Result(e, directup.Unspent, now.Add(time.Second))
	p.Event(directup.E3, now.Add(1200*time.Millisecond))
	if a, _ := callAt(p, relayed(), now.Add(2*time.Second)); a != directup.Punch {
		t.Fatalf("H-2 E3 did not advance the due time to lastStart + T_min")
	}
}

// H-3: Punch only with an inbound circuit; Dial only with outbound only and a public announced address;
// two circuits: nobody initiates (H-15, adopted).
func TestPolicyRoles(t *testing.T) {
	for _, c := range []struct {
		in, out, public bool
		want            directup.Action
	}{
		{in: true, want: directup.Punch},
		{in: true, public: true, want: directup.Punch},
		{out: true, want: directup.None},
		{out: true, public: true, want: directup.Dial},
		{in: true, out: true, want: directup.None},
		{in: true, out: true, public: true, want: directup.None},
		{public: true, want: directup.None},
	} {
		o := directup.Obs{Enabled: true, Wanted: true, Relayed: true, LocalReady: true, In: c.in, Out: c.out, RemotePublic: c.public}
		got := directup.Role(o)
		if got == directup.Punch && !c.in {
			t.Errorf("H-3 punch without an inbound circuit (in %v, out %v, public %v)", c.in, c.out, c.public)
		}
		if got != c.want {
			t.Errorf("H-3 role(in %v, out %v, public %v) = %v, want %v", c.in, c.out, c.public, got, c.want)
		}
		p := directup.NewPolicy(t0, noJitter)
		if a, _ := callAt(p, o, t0); a != c.want {
			t.Errorf("H-3 policy action(in %v, out %v, public %v) = %v, want %v", c.in, c.out, c.public, a, c.want)
		}
	}
}

// H-4: one call in flight per peer, a cancelled one included until its result returns.
func TestPolicyOneCallInFlight(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	_, e := callAt(p, relayed(), t0)
	for i := 1; i < 120; i++ {
		now := t0.Add(time.Duration(i) * time.Second)
		if i == 10 {
			p.Event(directup.E7, now) // a new epoch: the call is cancelled but still in flight
		}
		if i == 20 {
			p.Event(directup.E8, now)
		}
		if d := p.Step(relayed(), now); d.Action != directup.None {
			t.Fatalf("H-4 calls in flight: a second call at +%d s while the first has not returned", i)
		}
	}
	if !p.InFlight() {
		t.Fatalf("H-4 the policy forgot its call in flight")
	}
	p.Result(e, directup.Unspent, t0.Add(2*time.Minute))
	if a, _ := callAt(p, relayed(), t0.Add(2*time.Minute)); a != directup.Punch {
		t.Fatalf("H-4 no call after the result of the cancelled one: %v", a)
	}
}

// H-5: at most 4 node-wide slots, granted FIFO, one ticket per peer, a lease of 75 s; 10 peers.
func TestSlotsFIFOWithLease(t *testing.T) {
	s := directup.NewSlots(t0)
	peers := make([]string, 10)
	for i := range peers {
		peers[i] = fmt.Sprintf("p%d", i)
		if granted := s.Ask(peers[i], t0); granted != (i < directup.MaxSlots) {
			t.Fatalf("H-5 slots held: peer %d granted %v with %d held", i, granted, s.Held(t0))
		}
	}
	if s.Held(t0) != directup.MaxSlots {
		t.Fatalf("H-5 slots held = %d, want %d", s.Held(t0), directup.MaxSlots)
	}
	// p0 keeps asking: it shares its grant, it never takes a second slot, and its ticket keeps no place.
	for range 5 {
		if !s.Ask("p0", t0) {
			t.Fatalf("H-5 p0 lost its grant while asking again")
		}
		s.Release("p0", t0)
	}
	// p0 and p1 release: the next in FIFO order are p4 and p5, not a peer that asks later.
	now := t0.Add(time.Second)
	s.Release("p0", now)
	s.Release("p1", now)
	if s.Ask("p0", now) {
		t.Fatalf("H-5 p0 asked again and took a slot before p4..p9 that wait")
	}
	for i, want := range map[int]bool{4: true, 5: true, 6: false, 9: false} {
		if got := s.Ask(peers[i], now); got != want {
			t.Fatalf("H-5 FIFO: peer %d granted %v, want %v", i, got, want)
		}
	}
	// A holder that never finishes loses its slot at 75 s, whatever it does.
	if !s.Holds("p2", t0.Add(directup.Lease-time.Millisecond)) {
		t.Fatalf("H-5 p2's lease ended before 75 s")
	}
	if s.Holds("p2", t0.Add(directup.Lease)) {
		t.Fatalf("H-5 slots held: p2 still holds its slot at 75 s")
	}
	// A round that gave up keeps its ticket's place; a grant made to it then is held for the lease.
	s2 := directup.NewSlots(t0)
	for i := range directup.MaxSlots {
		s2.Ask(peers[i], t0)
	}
	s2.Ask("late", t0)
	s2.GiveUp("late", t0.Add(directup.RoundWait))
	s2.Ask("later", t0.Add(directup.RoundWait))
	s2.Release("p0", t0.Add(30*time.Second))
	if !s2.Holds("late", t0.Add(30*time.Second)) {
		t.Fatalf("H-5 place lost: the peer whose round gave up was not granted first")
	}
	if s2.Holds("later", t0.Add(31*time.Second)) || !s2.Holds("late", t0.Add(30*time.Second+directup.Lease-time.Millisecond)) {
		t.Fatalf("H-5 the grant to a peer whose round gave up was not held for it")
	}
	if !s2.Ask("late", t0.Add(60*time.Second)) {
		t.Fatalf("H-5 the peer that came back did not find its grant")
	}
	// A ticket without any round for 20 min is dropped; Drop removes one at once.
	s3 := directup.NewSlots(t0)
	for i := range directup.MaxSlots {
		s3.Ask(peers[i], t0)
	}
	s3.Ask("idle", t0)
	s3.Ask("gone", t0)
	s3.Drop("gone")
	if s3.Queued("gone") || !s3.Queued("idle") {
		t.Fatalf("H-5 Drop: queued gone %v, idle %v", s3.Queued("gone"), s3.Queued("idle"))
	}
	s3.Expire(t0.Add(directup.TicketIdle))
	if s3.Queued("idle") {
		t.Fatalf("H-5 a ticket idle for 20 min was kept")
	}
}

// H-6: a new circuit is a new epoch (ladders reset, due at max(now, lastStart + T_min), the call in flight
// cancelled); a result of an older epoch frees the slot only.
func TestPolicyEpoch(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	_, e := callAt(p, relayed(), t0)
	now := t0.Add(400 * time.Millisecond)
	p.Event(directup.E7, now)
	if p.Epoch() == e {
		t.Fatalf("H-6 a new circuit did not start a new epoch")
	}
	if d := p.Step(relayed(), now); !d.Cancel {
		t.Fatalf("H-6 the call of the old epoch is not cancelled: %+v", d)
	}
	// The old call's result (it punched: Spent) frees the slot and changes nothing else.
	later := t0.Add(600 * time.Millisecond)
	if d := p.Result(e, directup.Spent, later); !d.FreeSlot || d.Action != directup.None {
		t.Fatalf("H-6 the old result: %+v, want the slot freed only", d)
	}
	if a, _ := callAt(p, relayed(), t0.Add(999*time.Millisecond)); a != directup.None {
		t.Fatalf("H-6 a call before lastStart + T_min")
	}
	a, e2 := callAt(p, relayed(), t0.Add(time.Second))
	if a != directup.Punch {
		t.Fatalf("H-6 direct after the new circuit: no call at lastStart + T_min (the stale Spent applied?)")
	}
	if e2 == e {
		t.Fatalf("H-6 the new call carries the old epoch")
	}
	// Ladders reset by the epoch: after Spent then E7, the next Unspent waits 1 s, not 2 s.
	p.Result(e2, directup.Unspent, t0.Add(2*time.Second))
	p.Event(directup.E7, t0.Add(2*time.Second))
	if a, _ := callAt(p, relayed(), t0.Add(3*time.Second)); a != directup.Punch {
		t.Fatalf("H-6 the epoch did not reset the ladders")
	}
}

// H-7: the classification table (vectors): Success, Busy (no token), Spent, Unspent, ErrClosed = stop.
func TestClassify(t *testing.T) {
	for _, c := range []struct {
		rounds          int
		dialled, direct bool
		out             directup.Outcome
		want            directup.Class
	}{
		{0, false, true, directup.OK, directup.Success},
		{3, true, true, directup.ErrOther, directup.Success},
		{0, false, false, directup.ErrBusy, directup.Busy},
		{0, false, true, directup.ErrBusy, directup.Success},
		{1, false, false, directup.ErrOther, directup.Spent},
		{0, true, false, directup.ErrOther, directup.Spent},
		{3, false, false, directup.OK, directup.Spent},
		{0, false, false, directup.ErrOther, directup.Unspent},
		{0, false, false, directup.OK, directup.Unspent},
		{2, true, true, directup.ErrClosed, directup.Closed},
		{0, false, false, directup.ErrClosed, directup.Closed},
	} {
		if got := directup.Classify(c.rounds, c.dialled, c.direct, c.out); got != c.want {
			t.Errorf("H-7 class(rounds %d, dialled %v, direct %v, %v) = %v, want %v", c.rounds, c.dialled, c.direct, c.out, got, c.want)
		}
	}
	// Busy takes no attempt token.
	p := directup.NewPolicy(t0, noJitter)
	now := t0
	for i := range 20 {
		a, e := callAt(p, relayed(), now)
		if a == directup.None {
			t.Fatalf("H-7 Busy took an attempt token: no call at try %d", i)
		}
		p.Result(e, directup.Busy, now)
		now = now.Add(directup.TBusy)
	}
}

// H-8: fast 1 s·2^j cap 30 s; slow 60 s·2^k cap 16 min; ± 25 % jitter (injected); Success clears them.
func TestPolicyLadders(t *testing.T) {
	// Without the attempt bucket in the way (an hour between calls): each wait follows its ladder.
	check := func(c directup.Class, want []time.Duration, rule string) {
		t.Helper()
		p := directup.NewPolicy(t0, noJitter)
		now := t0
		for i, w := range want {
			_, e := callAt(p, relayed(), now)
			p.Result(e, c, now)
			due := p.Due()
			if got := due.Sub(now); got != w {
				t.Fatalf("H-8 %s: wait %d = %v, want %v", rule, i, got, w)
			}
			now = due.Add(time.Hour) // far enough for the attempt bucket to refill
		}
	}
	s := time.Second
	check(directup.Unspent, []time.Duration{1 * s, 2 * s, 4 * s, 8 * s, 16 * s, 30 * s, 30 * s}, "fast ladder")
	m := time.Minute
	check(directup.Spent, []time.Duration{1 * m, 2 * m, 4 * m, 8 * m, 16 * m, 16 * m, 16 * m, 16 * m}, "slow ladder")

	// Jitter is applied to every ladder wait, within ± 25 %.
	for _, j := range []func(time.Duration) time.Duration{
		func(d time.Duration) time.Duration { return d - d/4 },
		func(d time.Duration) time.Duration { return d + d/4 - 1 },
	} {
		p := directup.NewPolicy(t0, j)
		_, e := callAt(p, relayed(), t0)
		p.Result(e, directup.Spent, t0)
		if got := p.Due().Sub(t0); got != j(time.Minute) {
			t.Fatalf("H-8 slow ladder: jittered wait %v, want %v", got, j(time.Minute))
		}
	}
	if j := directup.Jitter(time.Minute); j < 45*time.Second || j >= 75*time.Second {
		t.Fatalf("H-8 Jitter(1 min) = %v, outside ± 25 %%", j)
	}

	// Success clears both ladders: Spent, Spent, Success; the session stays relayed, so after T_settle the
	// call is classed Unspent; the next Spent waits 1 min again, not 4 min.
	p := directup.NewPolicy(t0, noJitter)
	now := t0
	for _, c := range []directup.Class{directup.Spent, directup.Spent, directup.Success} {
		_, e := callAt(p, relayed(), now)
		p.Result(e, c, now)
		now = now.Add(time.Hour)
	}
	if d := p.Step(relayed(), now); !d.Nudge {
		t.Fatalf("H-8 no settle after Success: %+v", d)
	}
	now = p.Due()
	_, e := callAt(p, relayed(), now)
	p.Result(e, directup.Spent, now)
	if got := p.Due().Sub(now); got != time.Minute {
		t.Fatalf("H-8 slow ladder after Success: wait %v, want 1 min", got)
	}
}

// H-9: E3, E4, E7, E8 advance the due time to max(now, lastStart + T_min); E7 and E8 reset the slow ladder;
// no event refills a budget; relay-address-only changes are ignored (the transport never reports them as E8).
func TestPolicyEvents(t *testing.T) {
	for _, ev := range []directup.Event{directup.E2, directup.E3, directup.E4, directup.E7, directup.E8} {
		p := directup.NewPolicy(t0, noJitter)
		_, e := callAt(p, relayed(), t0)
		p.Result(e, directup.Spent, t0) // due in 1 min
		p.Event(ev, t0.Add(100*time.Millisecond))
		if got := p.Due(); !got.Equal(t0.Add(directup.TMin)) {
			t.Fatalf("H-9 %v: due %v after the start, want lastStart + T_min", ev, got.Sub(t0))
		}
		p.Event(ev, t0.Add(10*time.Second))
		if got := p.Due(); !got.Equal(t0.Add(directup.TMin)) {
			t.Fatalf("H-9 %v: a later event postponed the due time to %v", ev, got.Sub(t0))
		}
	}
	// E7 and E8 reset the slow ladder; E3 does not.
	for ev, reset := range map[directup.Event]bool{directup.E7: true, directup.E8: true, directup.E3: false} {
		p := directup.NewPolicy(t0, noJitter)
		now := t0
		for range 3 {
			_, e := callAt(p, relayed(), now)
			p.Result(e, directup.Spent, now)
			now = now.Add(time.Hour)
		}
		p.Event(ev, now)
		_, e := callAt(p, relayed(), now.Add(time.Second))
		p.Result(e, directup.Spent, now.Add(time.Second))
		got := p.Due().Sub(now.Add(time.Second))
		if want := map[bool]time.Duration{true: time.Minute, false: 8 * time.Minute}[reset]; got != want {
			t.Fatalf("H-9 %v: slow wait %v, want %v", ev, got, want)
		}
	}
	// No event refills the attempt bucket: empty it, then every event: no call before a token comes back.
	p := directup.NewPolicy(t0, noJitter)
	now := t0
	for range directup.AttemptBurst {
		_, e := callAt(p, relayed(), now)
		p.Result(e, directup.Unspent, now)
		p.Event(directup.E3, now.Add(directup.TMin))
		now = now.Add(directup.TMin)
	}
	for _, ev := range []directup.Event{directup.E2, directup.E3, directup.E4, directup.E7, directup.E8, directup.EActive} {
		p.Event(ev, now)
		if a, _ := callAt(p, relayed(), now); a != directup.None {
			t.Fatalf("H-9 an event refilled a budget: %v gave a call with the attempt bucket empty", ev)
		}
	}
}

// H-10: no terminal state: at the cap, one call per 16 min ± 25 % while wanted and relayed.
func TestPolicyNoHopelessState(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	now := t0
	calls := 0
	for now.Before(t0.Add(24 * time.Hour)) {
		if a, e := callAt(p, relayed(), now); a != directup.None {
			calls++
			p.Result(e, directup.Spent, now)
		}
		d := p.Step(relayed(), now)
		if d.Wake.IsZero() {
			t.Fatalf("H-10 the policy stopped after %d calls (no wake)", calls)
		}
		if gap := d.Wake.Sub(now); gap > directup.SlowCap*5/4 {
			t.Fatalf("H-10 a wait of %v after %d calls, beyond 16 min · 1.25", gap, calls)
		}
		now = d.Wake
	}
	if want := 24*60/16 - 2; calls < want {
		t.Fatalf("H-10 %d calls in 24 h, want at least %d (one per 16 min at the cap)", calls, want)
	}
}

// H-11: ≤ 4 + 8 = 12 non-Busy calls per peer in any hour, kept across epochs, reconnections and un-want.
func TestAttemptBudget(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	var starts []time.Time
	o := relayed()
	for i := range 4 * 3600 {
		now := t0.Add(time.Duration(i) * time.Second)
		if i%10 == 0 { // a member that reconnects every 10 s
			p.Event(directup.E7, now)
		}
		if i%600 == 300 { // and is un-wanted then wanted again
			nw := o
			nw.Wanted = false
			p.Step(nw, now)
		}
		if a, e := callAt(p, o, now); a != directup.None {
			starts = append(starts, now)
			p.Result(e, directup.Unspent, now.Add(500*time.Millisecond))
		}
	}
	for i := range starts {
		n := 0
		for j := i; j < len(starts) && starts[j].Sub(starts[i]) < time.Hour; j++ {
			n++
		}
		if n > directup.AttemptBurst+8 {
			t.Fatalf("H-11 calls in an hour: %d from +%v (want ≤ 12)", n, starts[i].Sub(t0))
		}
	}
	if len(starts) < 3*8 {
		t.Fatalf("H-11 only %d calls in 4 h, want the sustained 8 an hour", len(starts))
	}
	// The bucket itself: 4 at once, then 1 per 7.5 min.
	b := directup.NewBucket(directup.AttemptBurst, directup.AttemptRefill, t0)
	for i := range 4 {
		if !b.Take(t0) {
			t.Fatalf("H-11 bucket: take %d refused", i)
		}
	}
	if b.Take(t0.Add(directup.AttemptRefill - time.Nanosecond)) {
		t.Fatalf("H-11 bucket: a fifth token before 7.5 min")
	}
	if !b.Take(t0.Add(directup.AttemptRefill)) {
		t.Fatalf("H-11 bucket: no token at 7.5 min")
	}
	// The transport keeps a peer's policy until it is rested: no call in flight, the attempt budget full.
	r := directup.NewPolicy(t0, noJitter)
	if !r.Rested(t0) {
		t.Fatalf("H-11 rested: a new policy is not rested")
	}
	_, e := callAt(r, o, t0)
	if r.Rested(t0.Add(time.Hour)) {
		t.Fatalf("H-11 rested: rested with a call in flight")
	}
	r.Result(e, directup.Unspent, t0.Add(time.Second))
	if r.Rested(t0.Add(directup.AttemptRefill - time.Nanosecond)) {
		t.Fatalf("H-11 rested: rested before its attempt token came back")
	}
	if !r.Rested(t0.Add(directup.AttemptRefill)) {
		t.Fatalf("H-11 rested: not rested once its attempt budget is full again")
	}
}

// H-12: the budgets of the two choke points (rounds: 9 then 1 per 100 s; destinations: 30 then 1 per 25 s,
// each drawn from a held grant's allowance) and the address selection (≤ 3 public non-circuit addresses:
// first QUIC/IPv4, first QUIC/IPv6, first TCP/IPv4). The checks of « wanted » are the transport's
// (internal/transport/p2p: TestUpgradeGateRefusesSpam, TestUpgradeDialsOnlyAdmitted).
func TestAddrFilterAndGater(t *testing.T) {
	b := directup.NewBudgets(t0)
	rounds := 0
	for i := range 3600 {
		if b.Rounds.Take(t0.Add(time.Duration(i) * time.Second)) {
			rounds++
		}
	}
	if rounds > directup.RoundBurst+36 || rounds < directup.RoundBurst+35 {
		t.Fatalf("H-12 rounds in an hour = %d, want 45 (9 + 36)", rounds)
	}
	dests := 0
	for i := range 3600 * 4 {
		if b.Dests.Take(t0.Add(time.Duration(i) * 250 * time.Millisecond)) {
			dests++
		}
	}
	if dests > directup.DestBurst+144 || dests < directup.DestBurst+143 {
		t.Fatalf("H-12 destinations in an hour = %d, want 174 (30 + 144)", dests)
	}

	q4 := directup.Addr{QUIC: true, Public: true}
	q6 := directup.Addr{QUIC: true, IPv6: true, Public: true}
	t4 := directup.Addr{TCP: true, Public: true}
	priv := directup.Addr{QUIC: true}
	circ := directup.Addr{QUIC: true, Public: true, Circuit: true}
	for _, c := range []struct {
		in   []directup.Addr
		want []int
	}{
		{nil, nil},
		{[]directup.Addr{priv, circ}, nil},
		{[]directup.Addr{q4, q4, q4, q6, q6, t4, t4}, []int{0, 3, 5}},
		{[]directup.Addr{circ, t4, priv, q6, q4}, []int{1, 3, 4}},
		{[]directup.Addr{{TCP: true, IPv6: true, Public: true}, q4}, []int{1}},
		// A name (/dns4/…/quic-v1) is public to manet but carries no IP: refused before selection.
		{[]directup.Addr{{QUIC: true, Public: true, Name: true}, q6, {TCP: true, Public: true, Name: true}}, []int{1}},
	} {
		got := directup.Select(c.in)
		if len(got) > directup.MaxAddrs || fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("H-12 Select(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	// Twenty public addresses: three.
	var many []directup.Addr
	for range 20 {
		many = append(many, q4, q6, t4)
	}
	if got := directup.Select(many); len(got) != 3 {
		t.Fatalf("H-12 Select(60 public) = %d addresses, want 3", len(got))
	}

	// A destination needs a held grant: none outside one, at most its allowance inside.
	s := directup.NewSlots(t0)
	if s.Spend("p", t0) {
		t.Fatalf("H-12 a destination without a grant")
	}
	s.Ask("p", t0)
	n := 0
	for s.Spend("p", t0) {
		n++
		if n > 100 {
			break
		}
	}
	if n != directup.GrantAllowance {
		t.Fatalf("H-12 destinations under one grant = %d, want %d", n, directup.GrantAllowance)
	}
}

// H-13: after Success, if the session is not direct within T_settle, nudge once and class Unspent.
func TestPolicySettle(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	_, e := callAt(p, relayed(), t0)
	d := p.Result(e, directup.Success, t0.Add(time.Second))
	if !d.FreeSlot {
		t.Fatalf("H-13 Success did not free the slot")
	}
	end := t0.Add(time.Second + directup.TSettle)
	for now := t0.Add(time.Second); now.Before(end); now = now.Add(time.Second) {
		if d := p.Step(relayed(), now); d.Action != directup.None || d.Nudge {
			t.Fatalf("H-13 while settling: %+v at %v", d, now.Sub(t0))
		}
	}
	d = p.Step(relayed(), end)
	if !d.Nudge {
		t.Fatalf("H-13 direct after settle: no nudge at T_settle (%+v)", d)
	}
	if d := p.Step(relayed(), end); d.Nudge {
		t.Fatalf("H-13 nudged twice")
	}
	// Classed Unspent: the next call 1 s later.
	if a, _ := callAt(p, relayed(), end.Add(999*time.Millisecond)); a != directup.None {
		t.Fatalf("H-13 a call before the fast ladder's 1 s")
	}
	if a, _ := callAt(p, relayed(), end.Add(time.Second)); a != directup.Punch {
		t.Fatalf("H-13 no call 1 s after the settle (Unspent)")
	}
	// The session moved within T_settle: Direct, no nudge.
	p = directup.NewPolicy(t0, noJitter)
	_, e = callAt(p, relayed(), t0)
	p.Result(e, directup.Success, t0)
	o := relayed()
	o.Relayed, o.Direct = false, true
	if d := p.Step(o, t0.Add(time.Second)); d.Nudge || d.Action != directup.None {
		t.Fatalf("H-13 direct within T_settle: %+v", d)
	}
}

// H-14: direct stops the policy; a fallback to a circuit restarts it with budgets kept; unwant or close:
// nothing new starts, the call in flight is cancelled.
func TestPolicyDirectUnwantClose(t *testing.T) {
	direct := relayed()
	direct.Relayed, direct.Direct = false, true
	p := directup.NewPolicy(t0, noJitter)
	now := t0
	for range directup.AttemptBurst { // empty the attempt bucket
		_, e := callAt(p, relayed(), now)
		p.Result(e, directup.Unspent, now)
		now = now.Add(directup.FastCap)
	}
	for i := range 100 {
		if d := p.Step(direct, now.Add(time.Duration(i)*time.Second)); d.Action != directup.None || !d.Wake.IsZero() {
			t.Fatalf("H-14 a decision while direct: %+v", d)
		}
	}
	now = now.Add(100 * time.Second)
	// Fallback: restarted at once, but with the bucket as it was (100 s refill nothing yet past 7.5 min).
	if a, _ := callAt(p, relayed(), now); a != directup.None {
		t.Fatalf("H-14 the fallback refilled the attempt bucket")
	}
	if d := p.Step(relayed(), now); d.Wake.IsZero() {
		t.Fatalf("H-14 the fallback did not restart the policy")
	}
	// Unwant with a call in flight: cancelled, its result discarded, nothing new starts.
	p = directup.NewPolicy(t0, noJitter)
	_, e := callAt(p, relayed(), t0)
	off := relayed()
	off.Wanted = false
	if d := p.Step(off, t0.Add(time.Second)); !d.Cancel {
		t.Fatalf("H-14 unwant did not cancel the call in flight: %+v", d)
	}
	if d := p.Result(e, directup.Spent, t0.Add(2*time.Second)); d.Action != directup.None || !d.FreeSlot {
		t.Fatalf("H-14 the result after unwant: %+v", d)
	}
	for i := range 60 {
		if a, _ := callAt(p, off, t0.Add(time.Duration(i)*time.Second)); a != directup.None {
			t.Fatalf("H-14 a call after unwant")
		}
	}
	// Unwant with a call in flight, then wanted again before its result: the call stays stopped, its result
	// discarded; the restarted policy calls at lastStart + T_min, an event's due time (H-17: idle or not), not
	// after the slow ladder of the stopped call's Spent. The transport restarts the policy so whenever a new loop
	// drives it before the cancelled call returns, the new circuit's E7 lost or not.
	p = directup.NewPolicy(t0, noJitter)
	_, e = callAt(p, relayed(), t0)
	p.Step(off, t0.Add(100*time.Millisecond))
	if d := p.Step(relayed(), t0.Add(200*time.Millisecond)); d.Action != directup.None {
		t.Fatalf("H-14 a call while the stopped one is in flight: %+v", d)
	}
	if d := p.Result(e, directup.Spent, t0.Add(300*time.Millisecond)); d.Action != directup.None || !d.FreeSlot || !d.Wake.IsZero() {
		t.Fatalf("H-14 the stopped call's result after the restart: %+v, want the slot freed only", d)
	}
	idle := relayed()
	idle.Idle = true
	if a, _ := callAt(p, idle, t0.Add(time.Second)); a != directup.Punch {
		t.Fatalf("H-14 the restarted policy took the stopped call's result: no call at lastStart + T_min")
	}
	// Closed: the policy stops.
	p = directup.NewPolicy(t0, noJitter)
	_, e = callAt(p, relayed(), t0)
	p.Result(e, directup.Closed, t0)
	for i := range 60 {
		if a, _ := callAt(p, relayed(), t0.Add(time.Duration(i)*time.Minute)); a != directup.None {
			t.Fatalf("H-14 a call after the service closed")
		}
	}
}

// H-16: every slot grant takes one token of a node bucket of 8, refill 1 per 120 s (≤ 38 in any hour), FIFO, and
// allows 12 non-relay destinations for its peer while held; no non-relay destination outside a grant.
func TestNodeAdmissions(t *testing.T) {
	s := directup.NewSlots(t0)
	var grants []time.Time
	dests := 0
	for i := range 2 * 3600 {
		now := t0.Add(time.Duration(i) * time.Second)
		for k := range 20 { // 20 members asking without end, each releasing at once
			p := fmt.Sprintf("m%d", k)
			if s.Ask(p, now) {
				grants = append(grants, now)
				for s.Spend(p, now) {
					dests++
				}
				s.Release(p, now)
			}
		}
	}
	for i := range grants {
		n := 0
		for j := i; j < len(grants) && grants[j].Sub(grants[i]) < time.Hour; j++ {
			n++
		}
		if n > directup.NodeBurst+30 {
			t.Fatalf("H-16 admissions in an hour: %d from +%v, want ≤ 38", n, grants[i].Sub(t0))
		}
	}
	if len(grants) < 2*30 {
		t.Fatalf("H-16 admissions in 2 h: %d, want the sustained 30 an hour", len(grants))
	}
	if dests != len(grants)*directup.GrantAllowance {
		t.Fatalf("H-16 destinations = %d for %d grants, want %d each", dests, len(grants), directup.GrantAllowance)
	}
	// FIFO: each member granted in turn.
	s = directup.NewSlots(t0)
	var order []string
	for i := range 3600 {
		now := t0.Add(time.Duration(i) * time.Second)
		for k := range 6 {
			p := fmt.Sprintf("m%d", k)
			if s.Ask(p, now) {
				order = append(order, p)
				s.Release(p, now)
			}
		}
	}
	for i := 6; i < len(order); i++ {
		if order[i] != order[i-6] {
			t.Fatalf("H-16 admissions out of turn: %v", order[:i+1])
		}
	}
}

// H-17: ladder-driven calls only while active (input within 5 min, not asleep), suspended when idle and due
// at once on activity; event-driven calls at once whatever the activity.
func TestPolicyActivity(t *testing.T) {
	if directup.InputIdle(directup.ActiveWindow, false) || !directup.InputIdle(directup.ActiveWindow+time.Nanosecond, false) {
		t.Fatalf("H-17 the active window is not 5 min")
	}
	if !directup.InputIdle(0, true) {
		t.Fatalf("H-17 a machine asleep is not idle")
	}

	idle := relayed()
	idle.Idle = true
	p := directup.NewPolicy(t0, noJitter)
	_, e := callAt(p, relayed(), t0)
	p.Result(e, directup.Spent, t0) // ladder: due in 1 min
	for i := range 30 * 60 {
		if a, _ := callAt(p, idle, t0.Add(time.Duration(i)*time.Second)); a != directup.None {
			t.Fatalf("H-17 ladder call while idle at +%d s", i)
		}
	}
	now := t0.Add(30 * time.Minute)
	if a, _ := callAt(p, relayed(), now); a != directup.Punch {
		t.Fatalf("H-17 no call at once on activity")
	}
	// An event-driven call (a new circuit) runs at once while idle.
	p = directup.NewPolicy(t0, noJitter)
	_, e = callAt(p, relayed(), t0)
	p.Result(e, directup.Spent, t0)
	p.Event(directup.E7, t0.Add(10*time.Second))
	if a, _ := callAt(p, idle, t0.Add(10*time.Second)); a != directup.Punch {
		t.Fatalf("H-17 an event-driven call did not run while idle")
	}
	// The first relayed session (E1) is an event too.
	p = directup.NewPolicy(t0, noJitter)
	if a, _ := callAt(p, idle, t0); a != directup.Punch {
		t.Fatalf("H-17 the first session's call did not run while idle")
	}
}

// H-11, H-12: a bucket without refill (the zero refill period) gives its burst and then none, and reports a
// token available never later than now while it has one.
func TestBucketWithoutRefill(t *testing.T) {
	b := directup.NewBucket(2, 0, t0)
	for i := range 2 {
		if !b.Take(t0) {
			t.Fatalf("H-11 bucket without refill: take %d refused", i)
		}
	}
	if b.Take(t0.Add(time.Hour)) || b.Tokens(t0.Add(time.Hour)) != 0 {
		t.Fatalf("H-11 bucket without refill: a token came back")
	}
	if next := b.Next(t0.Add(time.Hour)); !next.Equal(t0.Add(time.Hour)) {
		t.Fatalf("H-11 bucket without refill: next token at %v, want now (it never refills)", next)
	}
}

// H-8: the jitter keeps a wait within ± 25 %, really spreads it, and leaves the shortest waits alone.
func TestJitter(t *testing.T) {
	for d := range time.Duration(4) {
		if got := directup.Jitter(d); got != d {
			t.Fatalf("H-8 jitter of %v: %v, want it unchanged", d, got)
		}
	}
	seen := map[time.Duration]bool{}
	for range 200 {
		got := directup.Jitter(4)
		if got < 3 || got > 5 {
			t.Fatalf("H-8 jitter of 4 ns: %v, beyond ± 25 %%", got)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Fatalf("H-8 jitter of 4 ns: always %v, not spread", seen)
	}
}

// H-6: each new circuit is a new, higher epoch.
func TestPolicyEpochCounts(t *testing.T) {
	p := directup.NewPolicy(t0, noJitter)
	for i := range 3 {
		before := p.Epoch()
		p.Event(directup.E7, t0.Add(time.Duration(i)*time.Second))
		if p.Epoch() != before+1 {
			t.Fatalf("H-6 epoch after a new circuit: %d, want %d", p.Epoch(), before+1)
		}
	}
}

// H-5, H-16: a dropped ticket leaves the queue alone and the others keep their order; a ticket without a round
// for 20 min is dropped exactly then; every grant is counted.
func TestSlotsDropAndIdle(t *testing.T) {
	s := directup.NewSlots(t0)
	for i := range directup.MaxSlots {
		if !s.Ask(fmt.Sprintf("h%d", i), t0) {
			t.Fatalf("H-5 slots: holder %d not granted", i)
		}
	}
	// a, b, fifteen others, then c: the admissions refill too slowly for c to be granted within 20 min.
	queued := []string{"a", "b"}
	for i := range 15 {
		queued = append(queued, fmt.Sprintf("x%d", i))
	}
	for _, p := range append(queued, "c") {
		if s.Ask(p, t0) {
			t.Fatalf("H-5 slots: %s granted beyond the cap", p)
		}
	}
	if until, ok := s.Until("h1", t0); !ok || !until.Equal(t0.Add(directup.Lease)) {
		t.Fatalf("H-5 slots: h1's grant ends at %v (held %v), want %v", until, ok, t0.Add(directup.Lease))
	}
	if _, ok := s.Until("a", t0); ok {
		t.Fatalf("H-5 slots: a waiting ticket reported as a grant")
	}
	s.Drop("b")
	if !s.Queued("a") || s.Queued("b") || !s.Queued("c") {
		t.Fatalf("H-5 slots: dropping b changed the other tickets (a %v, b %v, c %v)", s.Queued("a"), s.Queued("b"), s.Queued("c"))
	}
	now := t0.Add(time.Second)
	s.Release("h0", now)
	if !s.Holds("a", now) || s.Holds("c", now) {
		t.Fatalf("H-5 slots: after a drop, the freed slot did not go to the head (a %v, c %v)", s.Holds("a", now), s.Holds("c", now))
	}
	if got := s.Grants(); got != directup.MaxSlots+1 {
		t.Fatalf("H-16 admissions: %d grants counted, want %d", got, directup.MaxSlots+1)
	}
	if !s.Queued("c") {
		t.Fatalf("H-5 slots: c's ticket lost before 20 min")
	}
	s.Expire(t0.Add(directup.TicketIdle - time.Nanosecond))
	if !s.Queued("c") {
		t.Fatalf("H-5 slots: c's ticket dropped before 20 min without a round")
	}
	s.Expire(t0.Add(directup.TicketIdle))
	if s.Queued("c") {
		t.Fatalf("H-5 slots: c's ticket kept after 20 min without a round")
	}
}
