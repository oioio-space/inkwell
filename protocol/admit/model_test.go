// SPDX-License-Identifier: MPL-2.0

package admit_test

import (
	"maps"
	"net/netip"
	"slices"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/oioio-space/inkwell/protocol/admit"
)

// small is the model's scale (RelayFanout.cfg): P = 2, Q = 3, B = 2, MemberCap 6, PerCap 4, so that short
// runs reach every bound.
func small() admit.Config {
	return admit.Config{
		PerRelay: 2, PerIP: 2, PerWide: 3, Pending: 3, Deadline: admit.Nanos(15 * time.Second), Burst: 2, Refill: admit.Nanos(5 * time.Second),
		Members: 6, Share: 2, PerMember: 4, Relays: 2, PerRelayConns: 1,
	}
}

var modelOrigins = []admit.Origin{
	admit.RelayOrigin("r1"), admit.RelayOrigin("r2"),
	admit.IPOrigin(netip.MustParseAddr("2.2.9.1")),
	admit.IPOrigin(netip.MustParseAddr("2001:db8:1:100::1")), admit.IPOrigin(netip.MustParseAddr("2001:db8:1:200::1")),
}

var modelPeers = []string{"m1", "m2", "m3", "m4"}

// ref is a reference of RelayFanout's rules, written from the model's operators (PendOn, Pending, EstAll,
// Owed, Room, the bucket as debts paid one per period, oldest first) over a plain list, independently of the
// ledger's maps.
type ref struct {
	cfg    admit.Config
	conns  []*refConn
	debts  map[admit.Origin][]int // per origin, the unpaid admissions, oldest first
	clock  map[admit.Origin]admit.Nanos
	roster []string
	next   int
}

type refConn struct {
	id     int
	in     bool
	origin admit.Origin
	at     admit.Nanos
	kind   string // "opened", "pending", "expired", "member", "relay", "other"
	peer   string
}

func (r *ref) tick(now admit.Nanos) {
	for _, c := range r.conns {
		if c.kind == "pending" && now-c.at >= r.cfg.Deadline {
			c.kind = "expired"
		}
	}
	for o, d := range r.debts {
		for len(d) > 0 && now-r.clock[o] >= r.cfg.Refill {
			d = d[1:]
			r.clock[o] += r.cfg.Refill
		}
		r.debts[o] = d
	}
}

func (r *ref) count(f func(*refConn) bool) int {
	n := 0
	for _, c := range r.conns {
		if f(c) {
			n++
		}
	}
	return n
}

func (r *ref) pendOn(o admit.Origin) int {
	return r.count(func(c *refConn) bool { return c.kind == "pending" && c.origin == o })
}

func (r *ref) est(p string) int {
	return r.count(func(c *refConn) bool { return c.kind == "member" && c.peer == p })
}

func (r *ref) estAll() int { return r.count(func(c *refConn) bool { return c.kind == "member" }) }

func (r *ref) open(now admit.Nanos, in bool, o admit.Origin) (int, admit.Reason) {
	r.tick(now)
	if !in {
		r.next++
		r.conns = append(r.conns, &refConn{id: r.next, at: now, kind: "opened"})
		return r.next, 0
	}
	quota := r.cfg.PerIP
	if o.Relay != "" {
		quota = r.cfg.PerRelay
	}
	wide := r.count(func(c *refConn) bool { return c.kind == "pending" && o.Wide.IsValid() && c.origin.Wide == o.Wide })
	switch {
	case r.pendOn(o) >= quota:
		return 0, admit.ReasonOrigin
	case o.Wide.IsValid() && wide >= r.cfg.PerWide:
		return 0, admit.ReasonWide
	case r.count(func(c *refConn) bool { return c.kind == "pending" }) >= r.cfg.Pending:
		return 0, admit.ReasonPending
	case len(r.debts[o]) >= r.cfg.Burst:
		return 0, admit.ReasonRate
	}
	r.next++
	r.conns = append(r.conns, &refConn{id: r.next, in: true, origin: o, at: now, kind: "pending"})
	if len(r.debts[o]) == 0 {
		r.clock[o] = now
	}
	r.debts[o] = append(r.debts[o], r.next)
	return r.next, 0
}

func (r *ref) find(id int) *refConn {
	for _, c := range r.conns {
		if c.id == id {
			return c
		}
	}
	return nil
}

func (r *ref) identify(now admit.Nanos, id int, p string, class admit.Class) admit.Reason {
	r.tick(now)
	c := r.find(id)
	if c == nil || !slices.Contains([]string{"opened", "pending", "expired"}, c.kind) {
		return 0
	}
	c.peer = p
	if class == admit.Stranger {
		if c.kind == "opened" {
			c.kind = "other"
		}
		return 0
	}
	if i := slices.Index(r.debts[c.origin], id); i >= 0 {
		r.debts[c.origin] = slices.Delete(r.debts[c.origin], i, i+1)
	}
	if class == admit.Relay {
		mine := r.count(func(x *refConn) bool { return x.kind == "relay" && x.peer == p })
		if mine >= r.cfg.PerRelayConns || r.count(func(x *refConn) bool { return x.kind == "relay" }) >= r.cfg.Relays {
			c.kind = "other"
			return admit.ReasonRelays
		}
		c.kind = "relay"
		return 0
	}
	owed := 0
	for _, x := range r.roster {
		if x != p {
			owed += max(r.cfg.Share-r.est(x), 0)
		}
	}
	e := r.est(p)
	room := e < r.cfg.PerMember && r.estAll()+owed < r.cfg.Members
	if slices.Contains(r.roster, p) && e < r.cfg.Share {
		room = r.estAll() < r.cfg.Members
	}
	if !room {
		c.kind = "other"
		return admit.ReasonMembers
	}
	c.kind = "member"
	return 0
}

func (r *ref) close(now admit.Nanos, id int) {
	r.conns = slices.DeleteFunc(r.conns, func(c *refConn) bool { return c.id == id })
	r.tick(now)
}

func (r *ref) stats() (pending int, per map[admit.Origin]int, members int, perMember map[string]int, relays int) {
	per, perMember = map[admit.Origin]int{}, map[string]int{}
	for _, c := range r.conns {
		switch c.kind {
		case "pending":
			pending++
			per[c.origin]++
		case "member":
			members++
			perMember[c.peer]++
		case "relay":
			relays++
		}
	}
	return pending, per, members, perMember, relays
}

// kept is the part of a roster the shares are reserved for: Members / Share members, in sorted order.
func kept(cfg admit.Config, roster []string) []string {
	s := slices.Sorted(slices.Values(roster))
	return s[:min(len(s), cfg.Members/cfg.Share)]
}

// TestLedgerModel checks the ledger against the reference of RelayFanout's rules on random event sequences,
// at the model's scale and at spec §16.1's: every outcome and every count after every event.
func TestLedgerModel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cfg := small()
		if rapid.Bool().Draw(t, "spec scale") {
			cfg = nodeConfig()
		}
		roster := rapid.SliceOfDistinct(rapid.SampledFrom(modelPeers), rapid.ID).Draw(t, "roster")
		l := admit.New(cfg)
		l.SetRoster(roster)
		roster = kept(cfg, roster)
		r := &ref{cfg: cfg, debts: map[admit.Origin][]int{}, clock: map[admit.Origin]admit.Nanos{}, roster: roster}
		ids := map[int]admit.ID{}
		var open []int
		now := t0
		for step := range rapid.IntRange(1, 200).Draw(t, "steps") {
			switch rapid.IntRange(0, 9).Draw(t, "op") {
			case 0, 1, 2:
				in := rapid.IntRange(0, 3).Draw(t, "dir") > 0
				o := rapid.SampledFrom(modelOrigins).Draw(t, "origin")
				dir := admit.Outbound
				if in {
					dir = admit.Inbound
				}
				c, err := l.Open(now, dir, o)
				rid, want := r.open(now, in, o)
				if got := admit.ReasonOf(err); got != want {
					t.Fatalf("step %d: Open(%v, %v) = %v, reference %v", step, in, o, got, want)
				}
				if want == 0 {
					ids[rid] = c
					open = append(open, rid)
				}
			case 3, 4, 5:
				if len(open) == 0 {
					continue
				}
				rid := rapid.SampledFrom(open).Draw(t, "conn")
				p := rapid.SampledFrom(append(slices.Clone(modelPeers), "r1", "r2", "x")).Draw(t, "peer")
				class := rapid.SampledFrom([]admit.Class{admit.Member, admit.Relay, admit.Stranger}).Draw(t, "class")
				err := l.Identify(now, ids[rid], p, class)
				if got, want := admit.ReasonOf(err), r.identify(now, rid, p, class); got != want {
					t.Fatalf("step %d: Identify(%d, %s, %v) = %v, reference %v", step, rid, p, class, got, want)
				}
				if class == admit.Member && l.Owes(now, ids[rid]) {
					t.Fatalf("step %d: S4 a member's token still owed after its handshake", step)
				}
			case 6, 7:
				if len(open) == 0 {
					continue
				}
				i := rapid.IntRange(0, len(open)-1).Draw(t, "close")
				l.Close(now, ids[open[i]])
				r.close(now, open[i])
				open = slices.Delete(open, i, i+1)
			default:
				now = add(now, time.Duration(rapid.IntRange(0, 20000).Draw(t, "ms"))*time.Millisecond)
				r.tick(now)
			}
			s := l.Stats(now)
			pending, per, members, perMember, relays := r.stats()
			if s.Pending != pending || !maps.Equal(s.PerOrigin, per) || s.Members != members || !maps.Equal(s.PerMember, perMember) || s.Relays != relays {
				t.Fatalf("step %d: ledger %+v, reference pending %d %v members %d %v relays %d", step, s, pending, per, members, perMember, relays)
			}
			for _, o := range modelOrigins {
				if got, want := l.Tokens(now, o), cfg.Burst-len(r.debts[o]); got != want {
					t.Fatalf("step %d: tokens of %v = %d, reference %d", step, o, got, want)
				}
			}
		}
	})
}

// FuzzLedger drives the ledger with random event sequences and checks the model's invariants after each
// event: S1 OriginBound, S2 GlobalBound, S3 RateBound, S4 MembersSpendNoRate, S5 MemberCapBound, S9 SharesKept.
func FuzzLedger(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{0x10, 0x20, 0x30, 0x41, 0x52, 0x63, 0x74, 0x85, 0x96, 0xa7, 0xb8, 0xc9, 0xda, 0xeb, 0xfc})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		cfg := small()
		if data[0]&1 == 1 {
			cfg = nodeConfig()
		}
		roster := modelPeers[:int(data[0]>>1)%(len(modelPeers)+1)]
		l := admit.New(cfg)
		l.SetRoster(roster)
		roster = kept(cfg, roster)
		now := t0
		var open []admit.ID
		admitted := map[admit.Origin][]admit.Nanos{} // admissions whose token was not given back
		owner := map[admit.ID]admit.Origin{}
		at := map[admit.ID]admit.Nanos{}
		for i, b := range data[1:] {
			o := modelOrigins[int(b>>4)%len(modelOrigins)]
			switch b & 7 {
			case 0, 1:
				dir := admit.Inbound
				if b&8 != 0 {
					dir = admit.Outbound
				}
				if c, err := l.Open(now, dir, o); err == nil {
					open = append(open, c)
					if dir == admit.Inbound {
						admitted[o] = append(admitted[o], now)
						owner[c], at[c] = o, now
					}
				}
			case 2, 3:
				if len(open) == 0 {
					continue
				}
				c := open[int(b>>3)%len(open)]
				class := []admit.Class{admit.Member, admit.Member, admit.Relay, admit.Stranger}[int(b>>3)%4]
				owed := l.Owes(now, c)
				_ = l.Identify(now, c, modelPeers[int(b>>5)%len(modelPeers)], class)
				if class != admit.Stranger {
					if l.Owes(now, c) {
						t.Fatalf("event %d: S4 a member's token still owed after its handshake", i)
					}
					if o, ok := owner[c]; ok && owed {
						admitted[o] = slices.Delete(admitted[o], slices.Index(admitted[o], at[c]), slices.Index(admitted[o], at[c])+1)
						delete(owner, c)
					}
				}
			case 4:
				if len(open) == 0 {
					continue
				}
				k := int(b>>3) % len(open)
				l.Close(now, open[k])
				open = slices.Delete(open, k, k+1)
			case 5: // the membership changes: the shed connections close, then every share fits (S9)
				roster = kept(cfg, modelPeers[:int(b>>3)%(len(modelPeers)+1)])
				for _, c := range l.SetRoster(roster) {
					l.Close(now, c)
					open = slices.DeleteFunc(open, func(x admit.ID) bool { return x == c })
				}
			default:
				now = add(now, time.Duration(b)*100*time.Millisecond)
			}
			checkInvariants(t, i, l, cfg, roster, now, admitted)
		}
	})
}

func checkInvariants(t *testing.T, i int, l *admit.Ledger, cfg admit.Config, roster []string, now admit.Nanos, admitted map[admit.Origin][]admit.Nanos) {
	t.Helper()
	s := l.Stats(now)
	wide := map[netip.Prefix]int{}
	for o, n := range s.PerOrigin {
		quota := cfg.PerIP
		if o.Relay != "" {
			quota = cfg.PerRelay
		}
		if n > quota {
			t.Fatalf("event %d: S1 %d pending from %v > %d", i, n, o, quota)
		}
		if o.Wide.IsValid() {
			wide[o.Wide] += n
		}
	}
	for w, n := range wide {
		if n > cfg.PerWide {
			t.Fatalf("event %d: S1 %d pending in %v > %d", i, n, w, cfg.PerWide)
		}
	}
	if s.Pending > cfg.Pending {
		t.Fatalf("event %d: S2 %d pending > %d", i, s.Pending, cfg.Pending)
	}
	if s.Members > cfg.Members {
		t.Fatalf("event %d: S5 %d member connections > %d", i, s.Members, cfg.Members)
	}
	shares := s.Members
	for _, x := range roster {
		shares += max(cfg.Share-s.PerMember[x], 0)
	}
	if shares > cfg.Members {
		t.Fatalf("event %d: S9 members %d plus unused shares exceed %d: %v", i, s.Members, cfg.Members, s.PerMember)
	}
	for p, n := range s.PerMember {
		if n > cfg.PerMember {
			t.Fatalf("event %d: RF-6 member %s holds %d > %d", i, p, n, cfg.PerMember)
		}
	}
	// S3: at most B + k admissions in any window that k periods can end in (periods are a refill apart).
	for o, ts := range admitted {
		if tok := l.Tokens(now, o); tok < 0 || tok > cfg.Burst {
			t.Fatalf("event %d: S3 %d tokens for %v", i, tok, o)
		}
		for a := range ts {
			for b := a; b < len(ts); b++ {
				if n, k := b-a+1, int((ts[b]-ts[a])/cfg.Refill)+1; n > cfg.Burst+k {
					t.Fatalf("event %d: S3 %d admissions from %v in %v (%d periods at most), burst %d", i, n, o, time.Duration(ts[b]-ts[a]), k, cfg.Burst)
				}
			}
		}
	}
}
