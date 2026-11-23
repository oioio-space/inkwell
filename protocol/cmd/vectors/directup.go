// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"strings"
)

// duEnforce is the migration step from which runners enforce the direct-upgrade vectors (spec §8.6):
// the bounds are already those of the transport.
const duEnforce = "step-3"

// directupVectors are the cases of spec §8.6 « Montée en direct »: the class of a call, the role of
// each side, the two retry ladders, the four buckets and the address selection. Every expectation is
// written from the tables of the spec, not computed by protocol/directup.
func directupVectors() []vector {
	var v []vector
	add := func(id, desc, decoder string, ctx, expect map[string]any) {
		v = append(v, vector{
			ID: "directup-" + id, Status: "proposed", Enforce: duEnforce, Scope: "transport",
			Desc: desc, Decoder: decoder, Context: ctx, Verdict: "accept", Expect: expect,
		})
	}

	// The class of a call (§8.6, table « classe d'un appel »), by what was observed.
	for _, c := range []struct {
		rounds          int
		dialled, direct bool
		outcome, class  string
	}{
		{0, false, true, "ok", "success"},
		{3, true, true, "other", "success"},
		{0, false, true, "busy", "success"},
		{0, false, false, "busy", "busy"},
		{1, false, false, "other", "spent"},
		{0, true, false, "other", "spent"},
		{3, false, false, "ok", "spent"},
		{0, false, false, "other", "unspent"},
		{0, false, false, "ok", "unspent"},
		{2, true, true, "closed", "closed"},
		{0, false, false, "closed", "closed"},
	} {
		add(fmt.Sprintf("class-r%d-d%v-c%v-%s", c.rounds, c.dialled, c.direct, c.outcome),
			fmt.Sprintf("Classe d'un appel : %d ronde(s), numérotation %v, direct après %v, issue %s : %s.", c.rounds, c.dialled, c.direct, c.outcome, c.class),
			"directup_classify",
			map[string]any{"rounds": c.rounds, "dialled": c.dialled, "direct": c.direct, "outcome": c.outcome},
			map[string]any{"class": c.class})
	}

	// Who initiates (§8.6, rôles): Punch only over an inbound circuit, Dial with an outbound one alone and
	// a public announced address, nobody with two circuits.
	for _, c := range []struct {
		in, out, public bool
		action          string
	}{
		{true, false, false, "punch"},
		{true, false, true, "punch"},
		{false, true, false, "none"},
		{false, true, true, "dial"},
		{true, true, false, "none"},
		{true, true, true, "none"},
		{false, false, true, "none"},
	} {
		add(fmt.Sprintf("role-in%v-out%v-pub%v", c.in, c.out, c.public),
			fmt.Sprintf("Rôle : circuit entrant %v, sortant %v, adresse publique annoncée %v : %s.", c.in, c.out, c.public, c.action),
			"directup_role",
			map[string]any{"in": c.in, "out": c.out, "remote_public": c.public},
			map[string]any{"action": c.action})
	}

	// The ladders (§8.6, minuteries), without jitter: the wait after the n-th call of a class in a row.
	for _, l := range []struct {
		class string
		waits []int
	}{
		{"unspent", []int{1000, 2000, 4000, 8000, 16000, 30000, 30000}},
		{"spent", []int{60000, 120000, 240000, 480000, 960000, 960000, 960000, 960000}},
	} {
		class := l.class
		for n, w := range l.waits {
			add(fmt.Sprintf("ladder-%s-%d", class, n+1),
				fmt.Sprintf("Échelle %s : après le %de appel de cette classe, attente de %d ms (sans gigue).", class, n+1, w),
				"directup_ladder",
				map[string]any{"class": class, "calls": n + 1},
				map[string]any{"wait_ms": w})
		}
	}

	// The buckets (§8.6 and §16): takes at a steady period over an hour, and a burst at once.
	for _, c := range []struct {
		bucket   string
		periodMs int
		windowS  int
		granted  int
	}{
		{"attempts", 1000, 3600, 12},
		{"attempts", 0, 0, 4},
		{"rounds", 1000, 3600, 45},
		{"rounds", 0, 0, 9},
		{"dests", 250, 3600, 174},
		{"dests", 0, 0, 30},
		{"node", 1000, 3600, 38},
		{"node", 0, 0, 8},
	} {
		id, desc := fmt.Sprintf("bucket-%s-burst", c.bucket), fmt.Sprintf("%s bucket grants %d of 100 simultaneous requests.", c.bucket, c.granted)
		if c.periodMs > 0 {
			id = fmt.Sprintf("bucket-%s-hour", c.bucket)
			desc = fmt.Sprintf("%s bucket receives one request every %d ms for an hour and grants %d.", c.bucket, c.periodMs, c.granted)
		}
		add(id, desc, "directup_bucket",
			map[string]any{"bucket": c.bucket, "period_ms": c.periodMs, "window_s": c.windowS},
			map[string]any{"granted": c.granted})
	}

	// The address selection (§8.6): at most 3 public non-circuit addresses with an IP component (no DNS
	// name), the first QUIC over IPv4, the first QUIC over IPv6, the first TCP over IPv4.
	for _, c := range []struct {
		addrs    []string
		selected []int
	}{
		{nil, []int{}},
		{[]string{"quic4-private", "quic4-circuit"}, []int{}},
		{[]string{"quic4", "quic4", "quic4", "quic6", "quic6", "tcp4", "tcp4"}, []int{0, 3, 5}},
		{[]string{"quic4-circuit", "tcp4", "quic4-private", "quic6", "quic4"}, []int{1, 3, 4}},
		{[]string{"tcp6", "quic4"}, []int{1}},
		{[]string{"quic6-private", "tcp4-circuit", "tcp4"}, []int{2}},
		{[]string{"quic4-dns", "quic6", "tcp4-dns"}, []int{1}}, // a name, not an IP: refused before selection
	} {
		id := "select-empty"
		if len(c.addrs) > 0 {
			id = "select-" + strings.Join(c.addrs, "-")
		}
		add(id, fmt.Sprintf("Sélection d'adresses : %v donne les indices %v.", c.addrs, c.selected), "directup_select",
			map[string]any{"addrs": append([]string{}, c.addrs...)},
			map[string]any{"selected": c.selected})
	}
	return v
}
