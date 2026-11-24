// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"maps"
	"slices"
)

// The admission corpus (spec §16.1 rows « connexions », « connexions non vérifiées », « connexions par IP
// (membres) »; block RF1, rules RF-1 to RF-6). Each vector is an event script on one node's ledger, with the
// bounds written out in its context from the text of §16.1, not from any implementation: a bound, the
// bound + 1 and the recovery per rule. Origins are "relay:<PeerID>" (a circuit, RF-1) or "ip:<address>" (a
// direct connection, charged to its IPv4 /32 or IPv6 /56 and /48); an outbound opening has no origin.
// Status proposed, enforced from step 3, scope transport.

const (
	adEnforce = "step-3"
	adRelay   = "relay:12D3KooWRelayA"
	adRelayB  = "relay:12D3KooWRelayB"
	adHome    = "ip:2.2.9.1"
)

// adBounds are §16.1's bounds for a node; zero is no bound.
func adBounds() map[string]any {
	return map[string]any{
		"per_relay": 16, "per_ip": 8, "per_ipv6_48": 64, "pending": 48, "deadline_ms": 15000,
		"burst": 16, "refill_ms": 5000, "members": 128, "share": 2, "per_member": 4, "relays": 8, "per_relay_conns": 2,
	}
}

// adScript is one script: the bounds (§16.1's unless changed), the roster the shares are reserved for, and
// the events.
type adScript struct {
	id, desc string
	change   map[string]any
	roster   []string
	events   []event
}

func (s adScript) vector() vector {
	b := adBounds()
	maps.Copy(b, s.change)
	ctx := map[string]any{"bounds": b}
	if s.roster != nil {
		ctx["roster"] = s.roster
	}
	return vector{
		ID: s.id, Status: "proposed", Enforce: adEnforce, Desc: s.desc, Decoder: "admit_ledger", Verdict: "accept",
		Scope: "transport", Context: ctx, Events: s.events,
	}
}

// adOpen opens connection conn at at ms: inbound from origin, or outbound if origin is "".
func adOpen(at int, conn, origin, expect string) event {
	e := event{"op": "open", "at_ms": at, "conn": conn, "dir": "in", "origin": origin, "expect": expect}
	if origin == "" {
		e["dir"] = "out"
		delete(e, "origin")
	}
	return e
}

// adID is the handshake of conn: what it proved its peer to be (member, relay or stranger).
func adID(at int, conn, peer, class, expect string) event {
	return event{"op": "identify", "at_ms": at, "conn": conn, "peer": peer, "class": class, "expect": expect}
}

// adClose closes conn at 0 ms.
func adClose(conn string) event { return event{"op": "close", "at_ms": 0, "conn": conn} }

// adCheck checks the counts at at ms: pending in all, pending and tokens of origin, member connections.
func adCheck(at int, origin string, pending, onOrigin, tokens, members int) event {
	return event{
		"op": "check", "at_ms": at, "origin": origin,
		"expect": map[string]any{"pending": pending, "origin_pending": onOrigin, "tokens": tokens, "members": members},
	}
}

// adFill opens n inbound connections <prefix><i> from origin at 0 ms, all admitted.
func adFill(prefix string, n int, origin string) []event {
	var es []event
	for i := range n {
		es = append(es, adOpen(0, fmt.Sprintf("%s%d", prefix, i), origin, "ok"))
	}
	return es
}

func adCat(parts ...[]event) []event {
	var es []event
	for _, p := range parts {
		es = append(es, p...)
	}
	return es
}

func adIP(i int) string { return fmt.Sprintf("ip:3.3.%d.%d", i/256, i%256) }

func admitVectors() []vector {
	members := make([]string, 64)
	for i := range members {
		members[i] = fmt.Sprintf("m%02d", i)
	}
	// 128 member connections: two per roster member, outbound.
	var full []event
	for i := range 128 {
		c := fmt.Sprintf("o%d", i)
		full = append(full, adOpen(0, c, "", "ok"), adID(0, c, members[i/2], "member", "ok"))
	}
	// Nine members behind one home router and 63 behind one relay, all inbound.
	var nine, relayed []event
	for i := range 9 {
		c := fmt.Sprintf("h%d", i)
		nine = append(nine, adOpen(i*10, c, adHome, "ok"), adCheck(i*10, adHome, 1, 1, 15, i), adID(i*10, c, members[i], "member", "ok"))
	}
	for i := range 63 {
		c := fmt.Sprintf("r%d", i)
		relayed = append(relayed, adOpen(0, c, adRelay, "ok"), adID(0, c, members[i], "member", "ok"))
	}
	// 48 pending from 12 IPv4 origins.
	var pending48 []event
	for i := range 48 {
		pending48 = append(pending48, adOpen(0, fmt.Sprintf("p%d", i), adIP(i/4), "ok"))
	}
	// 64 per IPv6 /48: eight /56s of eight each (the global bound lifted).
	var wide []event
	for i := range 8 {
		o := fmt.Sprintf("ip:2001:db8:1:%x00::1", i)
		wide = append(wide, adFill(fmt.Sprintf("w%d-", i), 8, o)...)
	}
	vs := []adScript{
		{
			id: "admit-relay-origin-bound", desc: "16 connexions non vérifiées par relais (RF-3) ; la 17e est refusée (origin) ; une fermée et un jeton plus tard (5 s, celui de l'étrangère n'est pas rendu), la suivante passe.",
			events: adCat(adFill("c", 16, adRelay), []event{adOpen(0, "x", adRelay, "origin"), adOpen(0, "y", adRelayB, "ok"), adClose("c0"), adOpen(0, "z", adRelay, "rate"), adOpen(5000, "z2", adRelay, "ok"), adCheck(5000, adRelay, 17, 16, 0, 0)}),
		},
		{
			id: "admit-ip-origin-bound", desc: "8 connexions non vérifiées par adresse IPv4 /32 (RF-3) ; la 9e est refusée ; une fermée, la suivante passe 5 s plus tard.",
			events: adCat(adFill("c", 8, adHome), []event{adOpen(0, "x", "ip:2.2.9.1", "origin"), adOpen(0, "y", "ip:2.2.9.2", "ok"), adClose("c3"), adOpen(5000, "z", adHome, "ok")}),
		},
		{
			id: "admit-ipv6-origin", desc: "Une adresse IPv6 est comptée à son /56 (8) et à son /48 (64) : huit /56 pleins remplissent le /48, le neuvième /56 est refusé (wide).",
			change: map[string]any{"pending": 0},
			events: adCat(wide, []event{adOpen(0, "x", "ip:2001:db8:1:700::2", "origin"), adOpen(0, "y", "ip:2001:db8:1:ff00::1", "wide"), adOpen(0, "z", "ip:2001:db8:2::1", "ok")}),
		},
		{
			id: "admit-global-bound", desc: "48 connexions non vérifiées en tout (RF-3), sur douze origines ; la 49e est refusée quelle que soit son origine (pending) ; une fermée, la suivante passe.",
			events: adCat(pending48, []event{adOpen(0, "x", adIP(999), "pending"), adOpen(0, "y", adRelay, "pending"), adClose("p7"), adOpen(0, "z", adRelay, "ok"), adOpen(0, "w", adRelayB, "pending"), adCheck(0, adRelay, 48, 1, 15, 0)}),
		},
		{
			id: "admit-deadline", desc: "Une connexion non vérifiée tient sa place 15 s au plus (RF-4) : à 14 999 ms l'origine est encore pleine, à 15 000 ms ses places sont libres.",
			events: adCat(adFill("c", 8, adHome), []event{adCheck(14999, adHome, 8, 8, 10, 0), adOpen(14999, "x", adHome, "origin"), adCheck(15000, adHome, 0, 0, 11, 0), adOpen(15000, "y", adHome, "ok")}),
		},
		{
			id: "admit-rate", desc: "Seau de 16 jetons par origine, 1 de plus par 5 s (RF-5) : 16 admissions (fermées aussitôt), la 17e est refusée (rate) jusqu'à 5 s ; un autre relais a son propre seau.",
			events: func() []event {
				var es []event
				for i := range 16 {
					c := fmt.Sprintf("c%d", i)
					es = append(es, adOpen(0, c, adRelay, "ok"), adClose(c))
				}
				return append(es, adOpen(0, "x", adRelay, "rate"), adOpen(0, "y", adRelayB, "ok"), adOpen(4999, "x2", adRelay, "rate"), adOpen(5000, "x3", adRelay, "ok"), adOpen(5000, "x4", adRelay, "rate"), adCheck(20000, adRelay, 0, 0, 3, 0), adCheck(100000, adRelay, 0, 0, 16, 0))
			}(),
		},
		{
			id: "admit-outbound-uncharged", desc: "Une connexion sortante n'est comptée à aucune origine (RF-1, RF-2) : origine pleine et seau vide, le nœud ouvre encore vers un membre.",
			events: func() []event {
				var es []event
				for i := range 16 {
					c := fmt.Sprintf("c%d", i)
					es = append(es, adOpen(0, c, adRelay, "ok"))
				}
				return append(es, adOpen(0, "x", adRelay, "origin"), adOpen(0, "o", "", "ok"), adID(0, "o", "m00", "member", "ok"), adCheck(0, adRelay, 16, 16, 0, 1))
			}(),
		},
		{
			id: "admit-members-behind-one-ip", desc: "Neuf membres derrière une même box (RF-2) : chacun quitte le compte de l'adresse à sa poignée de main, son jeton rendu ; tous passent.",
			events: append(nine, adCheck(100, adHome, 0, 0, 16, 9)),
		},
		{
			id: "admit-members-behind-one-relay", desc: "63 membres derrière un même relais (RF-2) : aucun ne reste compté au relais.",
			events: append(relayed, adCheck(0, adRelay, 0, 0, 16, 63)),
		},
		{
			id: "admit-stranger-stays-counted", desc: "Un étranger identifié reste non vérifié jusqu'à sa fermeture (RF-3) ; un relais configuré sort des comptes comme un membre (RF-2).",
			events: []event{adOpen(0, "s", adHome, "ok"), adID(0, "s", "stranger", "stranger", "ok"), adCheck(0, adHome, 1, 1, 15, 0), adOpen(0, "r", adHome, "ok"), adID(0, "r", "relay1", "relay", "ok"), adCheck(0, adHome, 1, 1, 15, 0), adClose("s"), adCheck(0, adHome, 0, 0, 15, 0)},
		},
		{
			id: "admit-refund-unpaid-only", desc: "Le jeton d'un membre lui est rendu à sa poignée de main s'il est encore dû, jamais un jeton qu'une période a payé (RF-5) : la plus ancienne dette est payée d'abord.",
			events: []event{
				adOpen(0, "a", adRelay, "ok"), adOpen(0, "b", adRelay, "ok"), adCheck(0, adRelay, 2, 2, 14, 0),
				adCheck(5000, adRelay, 2, 2, 15, 0), adID(5000, "a", "m00", "member", "ok"), adCheck(5000, adRelay, 1, 1, 15, 1),
				adID(5000, "b", "m01", "member", "ok"), adCheck(5000, adRelay, 0, 0, 16, 2),
			},
		},
		{
			id: "admit-member-share", desc: "128 connexions de membres, 2 réservées à chaque membre de la liste (RF-6) : la 129e est refusée (members), même pour un membre hors liste ; une fermée, le membre retrouve sa part.",
			roster: members,
			events: append(full, adOpen(0, "x", "", "ok"), adID(0, "x", "m00", "member", "members"), adOpen(0, "y", "", "ok"), adID(0, "y", "outsider", "member", "members"),
				adClose("o127"), adOpen(0, "z", "", "ok"), adID(0, "z", "m63", "member", "ok"), adCheck(0, adRelay, 0, 0, 16, 128)),
		},
		{
			id: "admit-no-third-from-shares", desc: "Toutes les parts dues : un membre a ses deux connexions et pas de troisième (RF-6, une troisième seulement sur le reste).",
			roster: members,
			events: []event{adOpen(0, "a", "", "ok"), adID(0, "a", "m00", "member", "ok"), adOpen(0, "b", "", "ok"), adID(0, "b", "m00", "member", "ok"), adOpen(0, "c", "", "ok"), adID(0, "c", "m00", "member", "members")},
		},
		{
			id: "admit-surplus-to-four", desc: "Une liste de quatre membres : le reste permet à un membre une troisième et une quatrième connexion, jamais une cinquième (RF-6).",
			roster: members[:4],
			events: func() []event {
				var es []event
				for i, want := range []string{"ok", "ok", "ok", "ok", "members"} {
					c := fmt.Sprintf("c%d", i)
					es = append(es, adOpen(0, c, "", "ok"), adID(0, c, "m00", "member", want))
				}
				return es
			}(),
		},
		{
			id: "admit-relays", desc: "Relais configurés : 2 connexions chacun, 4 relais au plus (RF-6) ; une fermée, le relais en retrouve une.",
			events: func() []event {
				var es []event
				for r := range 4 {
					for k := range 2 {
						c := fmt.Sprintf("r%d-%d", r, k)
						es = append(es, adOpen(0, c, "", "ok"), adID(0, c, fmt.Sprintf("relay%d", r), "relay", "ok"))
					}
				}
				return append(es, adOpen(0, "x", "", "ok"), adID(0, "x", "relay0", "relay", "relays"), adOpen(0, "y", "", "ok"), adID(0, "y", "relay9", "relay", "relays"),
					adClose("r0-1"), adOpen(0, "z", "", "ok"), adID(0, "z", "relay0", "relay", "ok"))
			}(),
		},
		{
			id: "admit-pending-and-members-coexist", desc: "48 connexions non vérifiées et 128 de membres coexistent (RF-6) : ni les unes ni les autres ne prennent la place des autres.",
			events: func() []event {
				es := slices.Clone(pending48)
				for i := range 128 {
					c := fmt.Sprintf("o%d", i)
					es = append(es, adOpen(0, c, "", "ok"), adID(0, c, fmt.Sprintf("m%02d", i/2), "member", "ok"))
				}
				return append(es, adCheck(0, adIP(0), 48, 4, 12, 128))
			}(),
		},
	}
	out := make([]vector, len(vs))
	for i, s := range vs {
		out[i] = s.vector()
	}
	return out
}
