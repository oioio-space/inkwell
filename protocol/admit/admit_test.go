// SPDX-License-Identifier: MPL-2.0

package admit_test

import (
	"fmt"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/admit"
)

const t0 admit.Nanos = 0

// add is n plus d.
func add(n admit.Nanos, d time.Duration) admit.Nanos { return n + admit.Nanos(d) }

// nodeConfig is spec §16.1's configuration for a node, from protocol/limits.go.
func nodeConfig() admit.Config {
	return admit.Config{
		PerRelay: protocol.MaxPendingPerRelay, PerIP: protocol.MaxPendingPerIP,
		PerWide: protocol.MaxPendingPerIPv6WidePrefix, Pending: protocol.MaxPending,
		Deadline: admit.Nanos(protocol.PendingDeadline),
		Burst:    protocol.AdmissionBurst, Refill: admit.Nanos(protocol.AdmissionRefill),
		Members: protocol.MaxMemberConnections, Share: protocol.MemberShareConnections,
		PerMember: protocol.MaxConnectionsPerPeer,
		Relays:    protocol.MaxRelays * protocol.ConnectionsPerRelay, PerRelayConns: protocol.ConnectionsPerRelay,
	}
}

// TestPrefixesMatchSpecification: the ledger's prefixes are protocol/limits.go's (the package cannot import it).
func TestPrefixesMatchSpecification(t *testing.T) {
	if admit.IPv4Bits != protocol.PendingIPv4PrefixBits || admit.IPv6Bits != protocol.PendingIPv6PrefixBits || admit.IPv6WideBits != protocol.PendingIPv6WidePrefixBits {
		t.Fatal("RF-1: the ledger's origin prefixes differ from spec §16.1")
	}
}

var (
	relayA = admit.RelayOrigin("12D3KooWRelayA")
	relayB = admit.RelayOrigin("12D3KooWRelayB")
	home   = admit.IPOrigin(netip.MustParseAddr("2.2.9.1"))
)

func ipOrigin(i int) admit.Origin {
	return admit.IPOrigin(netip.AddrFrom4([4]byte{3, 3, byte(i >> 8), byte(i)}))
}

// open opens an inbound connection from o at now and fails the test with rule if it is refused.
func open(t *testing.T, l *admit.Ledger, now admit.Nanos, o admit.Origin, rule string) admit.ID {
	t.Helper()
	c, err := l.Open(now, admit.Inbound, o)
	if err != nil {
		t.Fatalf("%s: opening from %v refused at %v: %v", rule, o, time.Duration(now-t0), err)
	}
	return c
}

// refused checks that an inbound opening from o at now is refused for want.
func refused(t *testing.T, l *admit.Ledger, now admit.Nanos, o admit.Origin, want admit.Reason, rule string) {
	t.Helper()
	c, err := l.Open(now, admit.Inbound, o)
	if err == nil {
		l.Close(now, c)
		t.Fatalf("%s: opening from %v admitted at %v, want refused (%v)", rule, o, time.Duration(now-t0), want)
	}
	if got := admit.ReasonOf(err); got != want {
		t.Fatalf("%s: opening from %v refused for %v, want %v", rule, o, got, want)
	}
}

func member(t *testing.T, l *admit.Ledger, now admit.Nanos, c admit.ID, p, rule string) {
	t.Helper()
	if err := l.Identify(now, c, p, admit.Member); err != nil {
		t.Fatalf("%s: handshake of member %s refused: %v", rule, p, err)
	}
}

// TestOriginQuotaBound: 16 unverified connections per relay origin and 8 per IPv4 /32 (§16.1, RF-3): the
// next is refused, one closed lets one in.
func TestOriginQuotaBound(t *testing.T) {
	const rule = "RF-3 origin quota"
	for _, tc := range []struct {
		name  string
		o     admit.Origin
		quota int
	}{{"relay", relayA, protocol.MaxPendingPerRelay}, {"ipv4", home, protocol.MaxPendingPerIP}} {
		t.Run(tc.name, func(t *testing.T) {
			l := admit.New(nodeConfig())
			var first admit.ID
			for i := range tc.quota {
				c := open(t, l, t0, tc.o, rule)
				if i == 0 {
					first = c
				}
			}
			refused(t, l, t0, tc.o, admit.ReasonOrigin, rule)
			open(t, l, t0, relayB, rule) // another origin is not affected
			l.Close(t0, first)
			at := add(t0, protocol.AdmissionRefill) // the closed stranger's token is paid by a period, not given back
			open(t, l, at, tc.o, rule)
			if got := l.Stats(at).PerOrigin[tc.o]; got != tc.quota {
				t.Fatalf("%s: %d pending from %v, want %d", rule, got, tc.o, tc.quota)
			}
		})
	}
	// IPv6: 8 per /56, 64 per /48 (the global bound lifted to reach it).
	cfg := nodeConfig()
	cfg.Pending = 0
	l := admit.New(cfg)
	for i := range protocol.MaxPendingPerIPv6WidePrefix / protocol.MaxPendingPerIP {
		o := admit.IPOrigin(netip.MustParseAddr(fmt.Sprintf("2001:db8:1:%x00::1", i)))
		for range protocol.MaxPendingPerIP {
			open(t, l, t0, o, rule)
		}
		refused(t, l, t0, o, admit.ReasonOrigin, rule)
	}
	refused(t, l, t0, admit.IPOrigin(netip.MustParseAddr("2001:db8:1:ff00::1")), admit.ReasonWide, rule)
}

// TestGlobalQuotaBound: 48 unverified connections in all (§16.1, RF-3), over several origins: the 49th is
// refused whatever its origin, one closed lets one in.
func TestGlobalQuotaBound(t *testing.T) {
	const rule = "RF-3 global quota"
	l := admit.New(nodeConfig())
	var held []admit.ID
	for i := range protocol.MaxPending {
		held = append(held, open(t, l, t0, ipOrigin(i/4), rule))
	}
	refused(t, l, t0, ipOrigin(999), admit.ReasonPending, rule)
	refused(t, l, t0, relayA, admit.ReasonPending, rule)
	l.Close(t0, held[7])
	open(t, l, t0, relayA, rule)
	refused(t, l, t0, relayB, admit.ReasonPending, rule)
	if got := l.Stats(t0).Pending; got != protocol.MaxPending {
		t.Fatalf("%s: %d pending, want %d", rule, got, protocol.MaxPending)
	}
}

// TestRateBurstAndRefill: an origin's bucket holds 16 tokens and gains one per 5 s (§16.1, RF-5); a stranger's
// closed connection gives nothing back.
func TestRateBurstAndRefill(t *testing.T) {
	const rule = "RF-5 rate"
	l := admit.New(nodeConfig())
	for range protocol.AdmissionBurst {
		l.Close(t0, open(t, l, t0, relayA, rule))
	}
	refused(t, l, t0, relayA, admit.ReasonRate, rule)
	open(t, l, t0, relayB, rule) // each origin has its own bucket
	almost := add(t0, protocol.AdmissionRefill-time.Millisecond)
	refused(t, l, almost, relayA, admit.ReasonRate, rule)
	at := add(t0, protocol.AdmissionRefill)
	l.Close(at, open(t, l, at, relayA, rule))
	refused(t, l, at, relayA, admit.ReasonRate, rule)
	// Three periods later three tokens, not more.
	at = add(at, 3*protocol.AdmissionRefill)
	if got := l.Tokens(at, relayA); got != 3 {
		t.Fatalf("%s: %d tokens three periods later, want 3", rule, got)
	}
	// A full refill takes 16 periods, and the bucket never holds more than 16.
	at = add(t0, 100*protocol.AdmissionRefill)
	if got := l.Tokens(at, relayA); got != protocol.AdmissionBurst {
		t.Fatalf("%s: %d tokens after a long idle time, want %d", rule, got, protocol.AdmissionBurst)
	}
}

// TestMemberLeavesCounts: a member's handshake frees its origin's place at once (§16.1 « connexions par IP
// (membres) », RF-2), so nine members behind one IPv4 address and 63 behind one relay are all admitted.
func TestMemberLeavesCounts(t *testing.T) {
	const rule = "RF-2 member still counted"
	for _, tc := range []struct {
		o admit.Origin
		n int
	}{{home, 9}, {relayA, protocol.MaxConnectedMembers - 1}} {
		l := admit.New(nodeConfig())
		for i := range tc.n {
			c := open(t, l, t0, tc.o, rule)
			if got := l.Stats(t0).PerOrigin[tc.o]; got != 1 {
				t.Fatalf("%s: %d pending from %v before the handshake of member %d, want 1", rule, got, tc.o, i)
			}
			member(t, l, t0, c, fmt.Sprintf("m%d", i), rule)
			if got := l.Stats(t0).PerOrigin[tc.o]; got != 0 {
				t.Fatalf("%s: %d pending from %v after the handshake of member %d, want 0", rule, got, tc.o, i)
			}
		}
		if s := l.Stats(t0); s.Members != tc.n || s.Pending != 0 {
			t.Fatalf("%s: %d members and %d pending behind %v, want %d and 0", rule, s.Members, s.Pending, tc.o, tc.n)
		}
	}
	// A configured relay's connection leaves the counts as well.
	l := admit.New(nodeConfig())
	c := open(t, l, t0, home, rule)
	if err := l.Identify(t0, c, "relay", admit.Relay); err != nil {
		t.Fatalf("%s: relay's handshake refused: %v", rule, err)
	}
	if s := l.Stats(t0); s.Pending != 0 || s.Relays != 1 {
		t.Fatalf("%s: relay's connection: %d pending, %d relay connections, want 0 and 1", rule, s.Pending, s.Relays)
	}
	// A stranger's does not: it holds its place until it closes.
	c = open(t, l, t0, home, rule)
	if err := l.Identify(t0, c, "stranger", admit.Stranger); err != nil {
		t.Fatalf("stranger's handshake: %v", err)
	}
	if got := l.Stats(t0).PerOrigin[home]; got != 1 {
		t.Fatalf("RF-3: the identified stranger holds %d places, want 1", got)
	}
}

// TestRefundOnlyUnpaid: a member's handshake gives its token back while no period has paid it, and only then
// (§16.1, RF-5): a token a period paid, or a stranger's, never comes back.
func TestRefundOnlyUnpaid(t *testing.T) {
	const rule = "RF-5 refund of a paid token"
	l := admit.New(nodeConfig())
	c := open(t, l, t0, relayA, rule)
	if !l.Owes(t0, c) {
		t.Fatal("RF-5: an admission owes no token")
	}
	member(t, l, t0, c, "m1", rule)
	if got := l.Tokens(t0, relayA); got != protocol.AdmissionBurst {
		t.Fatalf("RF-5 refund: %d tokens after a member's handshake, want %d", got, protocol.AdmissionBurst)
	}
	// A stranger's admission, then a member's: the first period pays the oldest debt, the stranger's.
	s := open(t, l, t0, relayA, rule)
	m := open(t, l, add(t0, time.Second), relayA, rule)
	at := add(t0, protocol.AdmissionRefill)
	if got := l.Tokens(at, relayA); got != protocol.AdmissionBurst-1 {
		t.Fatalf("%s: %d tokens one period later, want %d", rule, got, protocol.AdmissionBurst-1)
	}
	member(t, l, at, m, "m2", rule)
	if got := l.Tokens(at, relayA); got != protocol.AdmissionBurst {
		t.Fatalf("RF-5 refund: %d tokens after the unpaid member's handshake, want %d", got, protocol.AdmissionBurst)
	}
	l.Close(at, s)
	// Two members' admissions, the second paid by a period before its handshake: the refund of the first
	// leaves the bucket one short, never full.
	a := open(t, l, at, relayA, rule)
	b := open(t, l, at, relayA, rule)
	at = add(at, protocol.AdmissionRefill) // pays a's debt, the oldest
	if l.Owes(at, a) || !l.Owes(at, b) {
		t.Fatalf("RF-5: owes a %v, b %v after one period, want false and true", l.Owes(at, a), l.Owes(at, b))
	}
	member(t, l, at, a, "m3", rule)
	if got := l.Tokens(at, relayA); got != protocol.AdmissionBurst-1 {
		t.Fatalf("%s: %d tokens after a paid member's handshake, want %d", rule, got, protocol.AdmissionBurst-1)
	}
	member(t, l, at, b, "m4", rule)
	if got := l.Tokens(at, relayA); got != protocol.AdmissionBurst {
		t.Fatalf("RF-5 refund: %d tokens after the unpaid member's handshake, want %d", got, protocol.AdmissionBurst)
	}
}

// TestPendingDeadline: an unverified connection holds its place for 15 s at most (§16.1, RF-4), not less.
func TestPendingDeadline(t *testing.T) {
	const rule = "RF-4 deadline"
	l := admit.New(nodeConfig())
	for range protocol.MaxPendingPerIP {
		open(t, l, t0, home, rule)
	}
	before := add(t0, protocol.PendingDeadline-time.Millisecond)
	if got := l.Stats(before).PerOrigin[home]; got != protocol.MaxPendingPerIP {
		t.Fatalf("%s: %d places held just before the deadline, want %d", rule, got, protocol.MaxPendingPerIP)
	}
	refused(t, l, before, home, admit.ReasonOrigin, rule)
	at := add(t0, protocol.PendingDeadline)
	if got := l.Stats(at).PerOrigin[home]; got != 0 {
		t.Fatalf("%s: %d places held at the deadline, want 0", rule, got)
	}
	open(t, l, at, home, rule)
}

// TestMemberShareBound: 128 member connections in all, 2 reserved for each roster member; a third or fourth
// comes only from the rest (§16.1, RF-6). The 129th is refused, one closed lets the next in.
func TestMemberShareBound(t *testing.T) {
	const rule = "RF-6 member share"
	n := protocol.MaxConnectedMembers
	roster := make([]string, n)
	for i := range roster {
		roster[i] = fmt.Sprintf("m%02d", i)
	}
	l := admit.New(nodeConfig())
	l.SetRoster(roster)
	out := func(p string) error {
		c, err := l.Open(t0, admit.Outbound, admit.Origin{})
		if err != nil {
			t.Fatalf("outbound opening refused: %v", err)
		}
		return l.Identify(t0, c, p, admit.Member)
	}
	// With every share owed, a member gets its two and no third.
	for range protocol.MemberShareConnections {
		if err := out(roster[0]); err != nil {
			t.Fatalf("%s: share connection refused: %v", rule, err)
		}
	}
	if err := out(roster[0]); admit.ReasonOf(err) != admit.ReasonMembers {
		t.Fatalf("%s: a third connection took another member's share (err %v)", rule, err)
	}
	// With half of the roster gone from the roster, the surplus lets a member reach four, not five.
	l.SetRoster(roster[:n/2])
	for range protocol.MaxConnectionsPerPeer - protocol.MemberShareConnections {
		if err := out(roster[0]); err != nil {
			t.Fatalf("%s: surplus connection refused: %v", rule, err)
		}
	}
	if err := out(roster[0]); admit.ReasonOf(err) != admit.ReasonMembers {
		t.Fatalf("%s: a fifth connection admitted (err %v)", rule, err)
	}
	// Fill up to 128 with everyone's share and the outsiders' surplus; the 129th is refused.
	l = admit.New(nodeConfig())
	l.SetRoster(roster)
	var last admit.ID
	for i := range protocol.MaxMemberConnections {
		c, _ := l.Open(t0, admit.Inbound, admit.Origin{})
		if err := l.Identify(t0, c, roster[i%n], admit.Member); err != nil {
			t.Fatalf("%s: connection %d of %s refused: %v", rule, i, roster[i%n], err)
		}
		last = c
	}
	if got := l.Stats(t0).Members; got != protocol.MaxMemberConnections {
		t.Fatalf("%s: %d member connections, want %d", rule, got, protocol.MaxMemberConnections)
	}
	for _, p := range []string{roster[0], "outsider"} {
		c, _ := l.Open(t0, admit.Inbound, admit.Origin{})
		if err := l.Identify(t0, c, p, admit.Member); admit.ReasonOf(err) != admit.ReasonMembers {
			t.Fatalf("%s: the 129th member connection (of %s) admitted (err %v)", rule, p, err)
		}
		l.Close(t0, c)
	}
	l.Close(t0, last) // roster[n-1] now has one: its share lets its next one in
	c, _ := l.Open(t0, admit.Inbound, admit.Origin{})
	if err := l.Identify(t0, c, roster[n-1], admit.Member); err != nil {
		t.Fatalf("%s: after a close, the member's own share refused: %v", rule, err)
	}
}

// TestPendingNeverUsesMemberRoom: 48 unverified connections and 128 member connections coexist (§16.1, RF-6):
// neither takes the other's room.
func TestPendingNeverUsesMemberRoom(t *testing.T) {
	const rule = "RF-6 shared capacity"
	l := admit.New(nodeConfig())
	for i := range protocol.MaxPending {
		open(t, l, t0, ipOrigin(i), rule)
	}
	for i := range protocol.MaxMemberConnections {
		c, err := l.Open(t0, admit.Outbound, admit.Origin{})
		if err == nil {
			err = l.Identify(t0, c, fmt.Sprintf("m%d", i/2), admit.Member)
		}
		if err != nil {
			t.Fatalf("%s: member connection %d refused with 48 pending: %v", rule, i, err)
		}
	}
	if s := l.Stats(t0); s.Pending != protocol.MaxPending || s.Members != protocol.MaxMemberConnections {
		t.Fatalf("%s: %d pending and %d members, want %d and %d", rule, s.Pending, s.Members, protocol.MaxPending, protocol.MaxMemberConnections)
	}
	// Members arriving inbound still pass: their place is taken back at once.
	l = admit.New(nodeConfig())
	for i := range protocol.MaxPending - 1 {
		open(t, l, t0, ipOrigin(i), rule)
	}
	for i := range protocol.MaxMemberConnections {
		c := open(t, l, t0, relayA, rule)
		member(t, l, t0, c, fmt.Sprintf("m%d", i/2), rule)
	}
}

// Relays: 2 connections each, 4 relays at most (§16.1, RF-6).
func TestRelayConnectionsBound(t *testing.T) {
	l := admit.New(nodeConfig())
	id := func(p string) error {
		c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
		return l.Identify(t0, c, p, admit.Relay)
	}
	for r := range protocol.MaxRelays {
		for range protocol.ConnectionsPerRelay {
			if err := id(fmt.Sprintf("r%d", r)); err != nil {
				t.Fatalf("RF-6 relays: connection to relay %d refused: %v", r, err)
			}
		}
	}
	if err := id("r0"); admit.ReasonOf(err) != admit.ReasonRelays {
		t.Fatalf("RF-6 relays: third connection to a relay: %v", err)
	}
	if err := id("r9"); admit.ReasonOf(err) != admit.ReasonRelays {
		t.Fatalf("RF-6 relays: a fifth relay: %v", err)
	}
}

func TestOriginOf(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"2.2.9.1", "ip:2.2.9.1/32"},
		{"::ffff:2.2.9.1", "ip:2.2.9.1/32"},
		{"2001:db8:1:2345::1", "ip:2001:db8:1:2300::/56"},
	} {
		if got := admit.IPOrigin(netip.MustParseAddr(tc.in)).String(); got != tc.want {
			t.Errorf("IPOrigin(%s) = %s, want %s", tc.in, got, tc.want)
		}
	}
	if got := admit.IPOrigin(netip.MustParseAddr("2001:db8:1:2345::1")).Wide.String(); got != "2001:db8:1::/48" {
		t.Errorf("wide prefix %s", got)
	}
	if !admit.IPOrigin(netip.Addr{}).IsZero() || admit.RelayOrigin("x").String() != "relay:x" || (admit.Origin{}).String() != "none" {
		t.Error("zero or relay origin")
	}
}

// A zero bound is no bound: a zero Config admits everything and keeps no bucket.
func TestZeroConfigUnbounded(t *testing.T) {
	l := admit.New(admit.Config{})
	l.SetRoster([]string{"m1"})
	for i := range 100 {
		c := open(t, l, t0, home, "zero config")
		if err := l.Identify(t0, c, "m1", admit.Member); err != nil {
			t.Fatalf("zero config: member connection %d refused: %v", i, err)
		}
		c = open(t, l, t0, home, "zero config")
		if err := l.Identify(t0, c, "r1", admit.Relay); err != nil {
			t.Fatalf("zero config: relay connection %d refused: %v", i, err)
		}
		open(t, l, t0, home, "zero config")
	}
	if s := l.Stats(add(t0, time.Hour)); s.Pending != 100 || s.Members != 100 || s.Relays != 100 || l.Tokens(t0, home) != 0 {
		t.Fatalf("zero config: %+v, tokens %d: want 100 pending forever, 100 members, 100 relays, no bucket", s, l.Tokens(t0, home))
	}
}

// RF-5 with no refill: the bucket never refills.
func TestRateWithoutRefill(t *testing.T) {
	l := admit.New(admit.Config{Burst: 2})
	open(t, l, t0, home, "RF-5 rate")
	open(t, l, t0, home, "RF-5 rate")
	refused(t, l, add(t0, time.Hour), home, admit.ReasonRate, "RF-5 rate")
}

// RF-6: a member that joins the roster while the members' room is full gets its share as connections close,
// never above the bound.
func TestRosterJoinsWhenFull(t *testing.T) {
	const rule = "RF-6 member share"
	l := admit.New(nodeConfig())
	var held []admit.ID
	for i := range protocol.MaxMemberConnections {
		c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
		member(t, l, t0, c, fmt.Sprintf("x%d", i/4), rule)
		held = append(held, c)
	}
	l.SetRoster([]string{"late"})
	c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
	if err := l.Identify(t0, c, "late", admit.Member); admit.ReasonOf(err) != admit.ReasonMembers {
		t.Fatalf("%s: a 129th connection admitted for a member that joined the roster (err %v)", rule, err)
	}
	l.Close(t0, held[0])
	c, _ = l.Open(t0, admit.Outbound, admit.Origin{})
	member(t, l, t0, c, "late", rule)
}

// RF-6: the shares owed are what each other roster member has not used: a member holding one connection owes
// one, so with 63 members at one each, a member's third connection fits in 128.
func TestOwedCountsPartialShares(t *testing.T) {
	const rule = "RF-6 member share"
	roster := make([]string, protocol.MaxConnectedMembers)
	for i := range roster {
		roster[i] = fmt.Sprintf("m%02d", i)
	}
	l := admit.New(nodeConfig())
	l.SetRoster(roster)
	out := func(p string) error {
		c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
		return l.Identify(t0, c, p, admit.Member)
	}
	for _, p := range roster[1:] {
		if err := out(p); err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
	}
	for range protocol.MemberShareConnections + 1 { // 65 held + 63 owed = 128: no room for the third…
		_ = out(roster[0])
	}
	if got := l.Stats(t0).PerMember[roster[0]]; got != protocol.MemberShareConnections {
		t.Fatalf("%s: member holds %d with 63 shares half used, want %d", rule, got, protocol.MemberShareConnections)
	}
	l.SetRoster(roster[:protocol.MaxConnectedMembers-1]) // …until one member leaves the roster: 65 + 62 < 128
	if err := out(roster[0]); err != nil {
		t.Fatalf("%s: third connection refused with room: %v", rule, err)
	}
}

// RF-6: a relay's 2 connections bound it before the relays' total does, and a closed one is counted back.
func TestRelayConnectionsPerRelay(t *testing.T) {
	l := admit.New(nodeConfig())
	id := func(p string) (admit.ID, error) {
		c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
		return c, l.Identify(t0, c, p, admit.Relay)
	}
	first, _ := id("r0")
	_, _ = id("r0")
	if _, err := id("r0"); admit.ReasonOf(err) != admit.ReasonRelays {
		t.Fatalf("RF-6 relays: a third connection to one relay: %v", err)
	}
	l.Close(t0, first)
	if _, err := id("r0"); err != nil {
		t.Fatalf("RF-6 relays: after a close: %v", err)
	}
	if _, err := id("r0"); admit.ReasonOf(err) != admit.ReasonRelays {
		t.Fatalf("RF-6 relays: a third connection after a close and a reopening: %v", err)
	}
}

// RF-6 with no share: the roster reserves nothing, the bound alone holds.
func TestMembersWithoutShare(t *testing.T) {
	l := admit.New(admit.Config{Members: 4})
	l.SetRoster([]string{"m1", "m2"})
	for i := range 4 {
		c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
		if err := l.Identify(t0, c, "m3", admit.Member); err != nil {
			t.Fatalf("RF-6: connection %d refused with no share: %v", i, err)
		}
	}
	c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
	if err := l.Identify(t0, c, "m1", admit.Member); admit.ReasonOf(err) != admit.ReasonMembers {
		t.Fatalf("RF-6: a fifth member connection admitted (err %v)", err)
	}
}

// Review finding 1 (gates/RF1/CODE-REVIEW.md): 63 Byzantine members each try a third connection; the 64th, honest,
// member still gets its two (RF-6, model SharesKept). The roster is the membership list.
func TestByzantinesCannotTakeShares(t *testing.T) {
	const rule = "RF-6 member share"
	roster := make([]string, protocol.MaxConnectedMembers)
	for i := range roster {
		roster[i] = fmt.Sprintf("m%02d", i)
	}
	for _, inRoster := range []bool{true, false} { // Byzantine members in the roster, or members the roster omits
		l := admit.New(nodeConfig())
		honest := roster[63]
		if inRoster {
			l.SetRoster(roster)
		} else {
			l.SetRoster([]string{honest})
		}
		for _, b := range roster[:63] {
			for range protocol.MemberShareConnections + 1 {
				c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
				_ = l.Identify(t0, c, b, admit.Member)
			}
		}
		for k := range protocol.MemberShareConnections {
			c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
			if err := l.Identify(t0, c, honest, admit.Member); err != nil {
				t.Fatalf("%s (Byzantine members in the roster: %v): the honest member's reserved connection %d refused: %v", rule, inRoster, k+1, err)
			}
		}
	}
}

// Review finding 1, second part: a member that joins the roster after the surplus was taken gets its share at
// once: the roster change names the surplus connections to close, newest member first, so that SharesKept holds
// again (RF-6) once they close.
func TestRosterGrowthShedsSurplus(t *testing.T) {
	const rule = "RF-6 member share"
	cfg := nodeConfig()
	l := admit.New(cfg)
	var roster []string
	for i := range 32 { // 32 members at four connections each: 128, half of it surplus
		p := fmt.Sprintf("m%02d", i)
		roster = append(roster, p)
		for range protocol.MaxConnectionsPerPeer {
			c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
			member(t, l, t0, c, p, rule)
		}
	}
	shed := l.SetRoster(roster)
	if len(shed) != 0 {
		t.Fatalf("%s: a roster that fits sheds %v", rule, shed)
	}
	shed = l.SetRoster(append(roster, "late"))
	if len(shed) != protocol.MemberShareConnections {
		t.Fatalf("%s: a member joining a full roster sheds %d surplus connections (%v), want %d", rule, len(shed), shed, protocol.MemberShareConnections)
	}
	for _, c := range shed {
		l.Close(t0, c)
	}
	for k := range protocol.MemberShareConnections {
		c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
		if err := l.Identify(t0, c, "late", admit.Member); err != nil {
			t.Fatalf("%s: the late member's reserved connection %d refused after the shed: %v", rule, k+1, err)
		}
	}
	s := l.Stats(t0)
	for _, p := range roster {
		if s.PerMember[p] < protocol.MemberShareConnections {
			t.Fatalf("%s: the shed took %s below its share: %v", rule, p, s.PerMember)
		}
	}
}

// The shed takes, one connection at a time, from the member holding the most above its own (its share in the
// roster, nothing outside), ties to the lower name, each member's newest connection first; a zero member bound
// sheds nothing (RF-6, review finding 1).
func TestShedPicksMostAboveNewestFirst(t *testing.T) {
	const rule = "RF-6 member share"
	cfg := admit.Config{Members: 7, Share: 2, PerMember: 4}
	conns := func(l *admit.Ledger, p string, n int) []admit.ID {
		var out []admit.ID
		for range n {
			c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
			member(t, l, t0, c, p, rule)
			out = append(out, c)
		}
		return out
	}
	for range 50 { // map order is random: the choice must not depend on it
		l := admit.New(cfg)
		l.SetRoster([]string{"a", "b"})
		a := conns(l, "a", 4)
		conns(l, "b", 3)
		if shed := l.SetRoster([]string{"a", "b"}); len(shed) != 0 {
			t.Fatalf("%s: a roster that fits sheds %v", rule, shed)
		}
		if shed, want := l.SetRoster([]string{"a", "b", "c"}), []admit.ID{a[3], a[2]}; !slices.Equal(shed, want) {
			t.Fatalf("%s: shed %v, want a's two newest %v (a holds 2 above its share, b 1, then a tie to the lower name)", rule, shed, want)
		}
		// Outside the roster a member's every connection is surplus: it goes before a roster member's share.
		l = admit.New(admit.Config{Members: 4, Share: 2, PerMember: 4})
		l.SetRoster([]string{"a"})
		conns(l, "a", 2)
		d := conns(l, "d", 2)
		if shed, want := l.SetRoster([]string{"a", "c"}), []admit.ID{d[1], d[0]}; !slices.Equal(shed, want) {
			t.Fatalf("%s: shed %v, want the non-roster member's %v, never a roster member's share", rule, shed, want)
		}
	}
	l := admit.New(admit.Config{})
	l.SetRoster([]string{"a"})
	conns(l, "a", 5)
	if shed := l.SetRoster([]string{"a", "b"}); len(shed) != 0 {
		t.Fatalf("%s: no member bound, yet the roster change sheds %v", rule, shed)
	}
}

// The membership bounds the reserve: a roster holds Members / Share = 64 members, in sorted order; a 65th has no
// reserved share, so with every share owed it finds no room (RF-6, review finding 1).
func TestRosterBoundedByMembers(t *testing.T) {
	roster := make([]string, protocol.MaxConnectedMembers+1)
	for i := range roster {
		roster[i] = fmt.Sprintf("m%02d", i)
	}
	l := admit.New(nodeConfig())
	l.SetRoster(roster)
	c, _ := l.Open(t0, admit.Outbound, admit.Origin{})
	if err := l.Identify(t0, c, roster[protocol.MaxConnectedMembers], admit.Member); admit.ReasonOf(err) != admit.ReasonMembers {
		t.Fatalf("RF-6 member share: the 65th member of a roster got a reserved connection (err %v)", err)
	}
	c, _ = l.Open(t0, admit.Outbound, admit.Origin{})
	member(t, l, t0, c, roster[0], "RF-6 member share")
}
