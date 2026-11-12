// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/binary"
	"fmt"
)

// The linkprobe corpus (spec §8.5). The bytes are assembled here from the layout of §8.5.4,
// and every verdict and expected answer was worked out by hand from the rules of §8.5.3 to
// §8.5.8: none comes from the codec, so the corpus is an oracle for it, not a copy of it.

const lpEnforce = "step-3" // CurrentStep when the probe was specified (block C5S)

// lpPTO is the PTO every script runs with, in milliseconds.
const lpPTO = 200

// lpProbe is PROBE: len 5, type 0x01, seq u32.
func lpProbe(seq uint32) []byte {
	return binary.BigEndian.AppendUint32([]byte{0x05, 0x01}, seq)
}

// lpEcho is ECHO: len 13, type 0x02, seq u32, received u64.
func lpEcho(seq uint32, received uint64) []byte {
	return binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint32([]byte{0x0d, 0x02}, seq), received)
}

// lpFrame is any message: len u8 = 1 + len(body), type, body.
func lpFrame(typ byte, body []byte) []byte {
	return append([]byte{u8(1 + len(body)), typ}, body...)
}

// ---- prober script events (§8.5.5) ----

func lpOpen(at int) event { return event{"op": "open", "at_ms": at, "expect": "ok"} }

func lpOpenRefused(at int) event { return event{"op": "open", "at_ms": at, "expect": "backoff"} }

func lpSend(at int, sent uint64, seq uint32) event {
	return event{"op": "probe", "at_ms": at, "sent": sent, "writable": true, "expect": hx(lpProbe(seq))}
}

func lpSendRefused(at int, sent uint64, writable bool, why string) event {
	return event{"op": "probe", "at_ms": at, "sent": sent, "writable": writable, "expect": why}
}

func lpRecv(at int, msg []byte, sentNow uint64, outcome string) event {
	return event{"op": "receive", "at_ms": at, "message_hex": hx(msg), "sent_now": sentNow, "pto_ms": lpPTO, "expect": outcome}
}

func lpLoss(at int, permille int) event { return event{"op": "loss", "at_ms": at, "expect": permille} }

// lpLevel is the level of §8.5.5 step 6 at that instant (it also advances its hysteresis).
func lpLevel(at int, level string) event { return event{"op": "level", "at_ms": at, "expect": level} }

// lpSlowPTO is the PTO of the round-trip level script: round trips up to 2.6 s are on time.
const lpSlowPTO = 3000

// lpAcceptSlow is an ECHO accepted on a path whose PTO is lpSlowPTO.
func lpAcceptSlow(at int, msg []byte, sentNow uint64) event {
	return event{"op": "receive", "at_ms": at, "message_hex": hx(msg), "sent_now": sentNow, "pto_ms": lpSlowPTO, "expect": "accepted"}
}

// ---- responder script events (§8.5.6) ----

func lpStream(at int, action, outcome string) event {
	return event{"op": "stream", "at_ms": at, "action": action, "expect": outcome}
}

func lpAnswer(at int, msg []byte, received uint64, writable bool, outcome string) event {
	return event{"op": "receive", "at_ms": at, "message_hex": hx(msg), "received": received, "writable": writable, "expect": outcome}
}

func lpEchoOf(seq uint32, received uint64) string { return hx(lpEcho(seq, received)) }

// ---- host script events (§8.5.3 to §8.5.6, sans I/O) ----

type acts []string

func lpTick(at int, sent uint64, writable bool, a ...string) event {
	return event{"op": "tick", "at_ms": at, "sent": sent, "writable": writable, "expect": append(acts{}, a...)}
}

func lpNego(at int, ok bool, sent uint64, writable bool, a ...string) event {
	return event{"op": "negotiated", "at_ms": at, "ok": ok, "sent": sent, "writable": writable, "expect": append(acts{}, a...)}
}

func lpOutData(at int, data []byte, sentNow uint64, a ...string) event {
	return event{"op": "outbound_data", "at_ms": at, "data_hex": hx(data), "sent_now": sentNow, "pto_ms": lpPTO, "expect": append(acts{}, a...)}
}

func lpOutEnded(at int) event { return event{"op": "outbound_ended", "at_ms": at, "expect": acts{}} }

func lpInOpened(at int, stream int, a ...string) event {
	return event{"op": "inbound_opened", "at_ms": at, "stream": stream, "expect": append(acts{}, a...)}
}

func lpInData(at, stream int, data []byte, received uint64, writable bool, a ...string) event {
	return event{"op": "inbound_data", "at_ms": at, "stream": stream, "data_hex": hx(data), "received": received, "writable": writable, "expect": append(acts{}, a...)}
}

func lpInEnded(at, stream int, fin bool, a ...string) event {
	return event{"op": "inbound_ended", "at_ms": at, "stream": stream, "fin": fin, "expect": append(acts{}, a...)}
}

func lpRoles(at int, roles []string, a ...string) event {
	return event{"op": "roles", "at_ms": at, "roles": roles, "expect": append(acts{}, a...)}
}

func lpHostLoss(at int, permille int) event {
	return event{"op": "loss", "at_ms": at, "expect": permille}
}

func wout(b []byte) string            { return "write_out:" + hx(b) }
func win(stream int, b []byte) string { return fmt.Sprintf("write_in:%d:%s", stream, hx(b)) }
func refuse(stream int) string        { return fmt.Sprintf("refuse_in:%d", stream) }
func resetIn(stream int) string       { return fmt.Sprintf("reset_in:%d", stream) }

func cat(bs ...[]byte) []byte {
	var out []byte
	for _, b := range bs {
		out = append(out, b...)
	}
	return out
}

func linkprobeVectors() []vector {
	var v []vector
	msg := func(id, desc string, in []byte, verdict string, expect map[string]any) {
		v = append(v, vector{
			ID: id, Status: "proposed", Enforce: lpEnforce, Scope: "transport", Desc: desc,
			Decoder: "linkprobe_message", Input: hx(in), Verdict: verdict, Expect: expect,
		})
	}
	msg("linkprobe-probe", "PROBE seq 1 : len 5, type 0x01, seq u32.", lpProbe(1), "accept", map[string]any{"type": 1, "seq": 1})
	msg("linkprobe-probe-seq-max", "PROBE seq 2^32 − 1, la plus grande.", lpProbe(0xffffffff), "accept", map[string]any{"type": 1, "seq": 4294967295})
	msg("linkprobe-echo", "ECHO seq 7, received 1000 : len 13, type 0x02, seq u32, received u64.", lpEcho(7, 1000), "accept", map[string]any{"type": 2, "seq": 7, "received": 1000})
	msg("linkprobe-echo-received-max", "ECHO received 2^64 − 1 : toute valeur u64 se décode (la règle du §8.5.5 en juge).", lpEcho(1, 0xffffffffffffffff), "accept", map[string]any{"type": 2, "seq": 1, "received": uint64(0xffffffffffffffff)})
	msg("linkprobe-ignorable", "Type 0x4A (ignorable, GREASE), corps de 2 octets : sauté.", lpFrame(0x4a, []byte{0xaa, 0xbb}), "ignore", map[string]any{"type": 0x4a})
	msg("linkprobe-ignorable-empty", "Type 0x40, corps vide (len 1) : sauté.", lpFrame(0x40, nil), "ignore", map[string]any{"type": 0x40})
	msg("linkprobe-ignorable-max", "Type 0x7F, corps de 63 octets (len 64, la borne) : sauté.", lpFrame(0x7f, make([]byte, 63)), "ignore", map[string]any{"type": 0x7f})
	msg("linkprobe-probe-seq-zero", "PROBE seq 0 : hors domaine (seq ≥ 1).", lpProbe(0), "reject", nil)
	msg("linkprobe-echo-seq-zero", "ECHO seq 0 : hors domaine.", lpEcho(0, 5), "reject", nil)
	msg("linkprobe-len-zero", "len 0 : refusé dès le premier octet.", []byte{0x00}, "reject", nil)
	msg("linkprobe-len-only", "len 65 seul, sans aucun corps : refusé dès le premier octet, sans attendre la suite.", []byte{0x41}, "reject", nil)
	msg("linkprobe-len-too-large", "len 65 suivi de 65 octets : refusé.", append([]byte{0x41, 0x4a}, make([]byte, 64)...), "reject", nil)
	msg("linkprobe-len-max-byte", "len 0xFF : refusé dès le premier octet.", []byte{0xff}, "reject", nil)
	msg("linkprobe-probe-long", "PROBE de len 6 : longueur fixe 5.", lpFrame(0x01, []byte{0, 0, 0, 1, 0}), "reject", nil)
	msg("linkprobe-probe-short", "PROBE de len 4 : longueur fixe 5.", lpFrame(0x01, []byte{0, 0, 1}), "reject", nil)
	msg("linkprobe-echo-short", "ECHO de len 12 : longueur fixe 13.", lpFrame(0x02, make([]byte, 11)), "reject", nil)
	msg("linkprobe-echo-long", "ECHO de len 14 : longueur fixe 13.", lpFrame(0x02, cat(make([]byte, 3), []byte{1}, make([]byte, 9))), "reject", nil)
	msg("linkprobe-head", "len 5 seul : incomplet.", []byte{0x05}, "incomplete", nil)
	msg("linkprobe-truncated", "PROBE tronqué : len 5 annoncé, 2 octets de corps : incomplet (si le flux finit là, c'est une fin de flux, §8.5.8).", []byte{0x05, 0x01, 0x00, 0x00}, "incomplete", nil)
	msg("linkprobe-trailing", "Un PROBE suivi d'un octet : un vecteur de message porte exactement un message.", append(lpProbe(1), 0x00), "reject", nil)
	msg("linkprobe-type-zero", "Type 0x00 : réservé.", lpFrame(0x00, nil), "reject", nil)
	msg("linkprobe-critical-unknown", "Type 0x03 (critique, non attribué) : refusé.", lpFrame(0x03, nil), "reject", nil)
	msg("linkprobe-critical-last", "Type 0x3F (critique, non attribué) : refusé.", lpFrame(0x3f, []byte{0x00}), "reject", nil)
	msg("linkprobe-reserved-major", "Type 0x80 (réservé à une majeure) : refusé.", lpFrame(0x80, nil), "reject", nil)
	msg("linkprobe-reserved-ff", "Type 0xFF : refusé.", lpFrame(0xff, nil), "reject", nil)

	script := func(id, decoder, desc string, ctx map[string]any, events ...event) {
		v = append(v, vector{
			ID: id, Status: "proposed", Enforce: lpEnforce, Scope: "transport", Desc: desc,
			Decoder: decoder, Context: ctx, Verdict: "accept", Events: events,
		})
	}
	prober := func(id, desc string, events ...event) { script(id, "linkprobe_prober", desc, nil, events...) }

	// ---- prober (§8.5.5); sent is the endpoint's packet counter when it writes the probe ----
	prober("linkprobe-prober-basic",
		"Trois échantillons à 15 s d'écart : perte (400 − 384) / 400 = 40 ‰, puis 0 ‰ sur la fenêtre ; inconnue dès que le dernier échantillon a plus de 30 s.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(40, lpEcho(1, 0), 10, "accepted"), lpLoss(40, 65535),
		lpSend(15000, 400, 2), lpRecv(15040, lpEcho(2, 384), 410, "accepted"), lpLoss(15040, 40),
		lpSend(30000, 800, 3), lpRecv(30040, lpEcho(3, 800), 810, "accepted"), lpLoss(30040, 0),
		lpLoss(60000, 0), lpLoss(60001, 65535))
	prober("linkprobe-prober-min-packets",
		"Moins de 200 paquets envoyés entre les deux échantillons : inconnue ; à 200, (20 × 1000 + 100) / 200 = 100 ‰.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 199, 2), lpRecv(15010, lpEcho(2, 160), 200, "accepted"), lpLoss(15010, 65535),
		lpSend(16000, 200, 3), lpRecv(16010, lpEcho(3, 180), 201, "accepted"), lpLoss(16010, 100))
	prober("linkprobe-prober-min-span",
		"Les deux échantillons d'un chiffre sont à au moins 15 s l'un de l'autre : à 14,999 s d'écart, inconnu ; à 15 s, (400 − 360) / 400 = 100 ‰.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(14999, 400, 2), lpRecv(15009, lpEcho(2, 360), 401, "accepted"), lpLoss(15009, 65535),
		event{"op": "restart", "at_ms": 15100},
		lpOpen(15100),
		lpSend(15100, 0, 1), lpRecv(15110, lpEcho(1, 0), 1, "accepted"),
		lpSend(30100, 400, 2), lpRecv(30110, lpEcho(2, 360), 401, "accepted"), lpLoss(30110, 100))
	prober("linkprobe-prober-min-span-late",
		"L'écart minimal se mesure entre les deux échantillons, pas depuis l'ouverture : à 20 s puis 30 s (10 s d'écart), inconnu, même si les instants sont grands.",
		lpOpen(0),
		lpSend(20000, 0, 1), lpRecv(20010, lpEcho(1, 0), 1, "accepted"),
		lpSend(30000, 400, 2), lpRecv(30010, lpEcho(2, 360), 401, "accepted"), lpLoss(30010, 65535))
	prober("linkprobe-prober-window",
		"La fenêtre part du plus ancien échantillon à au plus 30 s du plus récent : à 31 s, l'échantillon de 0 s sort ; (1000 − 950) / 1000 = 50 ‰ (75 ‰ s'il restait).",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(1000, 1000, 2), lpRecv(1010, lpEcho(2, 900), 1001, "accepted"),
		lpSend(31000, 2000, 3), lpRecv(31010, lpEcho(3, 1850), 2001, "accepted"), lpLoss(31010, 50))
	prober("linkprobe-prober-rounding",
		"Arrondi au plus proche, la moitié vers le haut : 1 perdu sur 2000 = 0,5 ‰ → 1 ‰ ; 1 sur 2001 → 0 ‰.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 2000, 2), lpRecv(15010, lpEcho(2, 1999), 2001, "accepted"), lpLoss(15010, 1),
		lpSend(16000, 2001, 3), lpRecv(16010, lpEcho(3, 2000), 2002, "accepted"), lpLoss(16010, 0))
	prober("linkprobe-prober-more-received",
		"Le pair a compté plus de paquets que l'intervalle n'en a envoyé (un paquet envoyé avant la première sonde et arrivé après elle) : perte 0, jamais négative.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 3, "accepted"),
		lpSend(15000, 400, 2), lpRecv(15010, lpEcho(2, 403), 404, "accepted"), lpLoss(15010, 0))
	// Level (step 6): a sample every 15 s, 1000 packets apart, so the figure at the k-th is
	// (lost over the last two intervals) / 2000, or lost / 1000 at the first.
	levelSteps := func(lost []uint64, levels []string) []event {
		es := []event{lpOpen(0), lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted")}
		var got uint64
		for k, l := range lost {
			at, sent := 15000*(k+1), uint64(1000*(k+1))
			got += 1000 - l
			es = append(es, lpSend(at, sent, uint32(k+2)), lpRecv(at+10, lpEcho(uint32(k+2), got), sent+1, "accepted"), lpLevel(at+10, levels[k]))
		}
		return es
	}
	prober("linkprobe-prober-level",
		"Niveau (étape 6) : après « inconnu », placé au milieu (9 ‰ → bon) ; bon monte à 25 ‰ (moyen), moyen reste à 25 puis à 5 ‰ et redescend sous 5 ‰ (bon) ; bon saute à mauvais à 150 ‰, y reste à 150, à 50 et à 99 ‰, redescend à moyen sous 50 ‰ (49), puis à bon à 0 ‰ ; plus de 30 s sans échantillon : inconnu, et l'hystérésis est oubliée (20 ‰ donne alors moyen, pas bon).",
		append(levelSteps(
			[]uint64{9, 41, 9, 0, 0, 299, 0, 100, 98, 0, 0},
			[]string{"good", "fair", "fair", "fair", "good", "poor", "poor", "poor", "poor", "fair", "good"}),
			lpLevel(210011, "unknown"),
			lpSend(220000, 20000, 13), lpRecv(220010, lpEcho(13, 18882), 20001, "accepted"),
			lpSend(235000, 21000, 14), lpRecv(235010, lpEcho(14, 19862), 21001, "accepted"), lpLevel(235010, "fair"))...)
	prober("linkprobe-prober-level-rtt",
		"Le niveau est le pire de celui de la perte et de celui de l'aller-retour (médiane basse des échantillons de la fenêtre), sans perte ici : 500 ms → moyen ; médiane 500 parmi 500, 500, 2600 → moyen ; 2600 → mauvais, et il le reste tant que la médiane l'est ; quand elle tombe à 300 ms (sous 400), bon.",
		lpOpen(0),
		lpSend(0, 0, 1), lpAcceptSlow(500, lpEcho(1, 0), 1),
		lpSend(15000, 300, 2), lpAcceptSlow(15500, lpEcho(2, 300), 301), lpLevel(15500, "fair"),
		lpSend(20000, 400, 3), lpAcceptSlow(22600, lpEcho(3, 400), 401), lpLevel(22600, "fair"),
		lpSend(31000, 600, 4), lpAcceptSlow(33600, lpEcho(4, 600), 601), lpLevel(33600, "poor"),
		lpSend(40000, 800, 5), lpAcceptSlow(42600, lpEcho(5, 800), 801), lpLevel(42600, "poor"),
		lpSend(52000, 1000, 6), lpAcceptSlow(52300, lpEcho(6, 1000), 1001), lpLevel(52300, "poor"),
		lpSend(53000, 1100, 7), lpAcceptSlow(53300, lpEcho(7, 1100), 1101), lpLevel(53300, "good"))
	prober("linkprobe-prober-late",
		"Un écho reçu plus d'un PTO après sa sonde est tardif, donc jeté ; à exactement un PTO, il est accepté ; (360 − 352) / 360 → 22 ‰.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(201, lpEcho(1, 0), 10, "late"),
		lpSend(1000, 40, 2), lpRecv(1200, lpEcho(2, 40), 80, "accepted"),
		lpSend(16000, 400, 3), lpRecv(16010, lpEcho(3, 392), 401, "accepted"), lpLoss(16010, 22))
	stale := func(last event) []event {
		es := []event{lpOpen(0)}
		for i := range 9 {
			es = append(es, lpSend(i*1000, uint64(i)*10, uint32(i+1)))
		}
		return append(es, last)
	}
	prober("linkprobe-prober-stale",
		"Au plus 8 sondes en attente : la 9e oublie la 1re, dont l'écho est périmé (jeté, sans faute) ; les sondes 2 à 8 sans écho sortent avec l'écho de la 9e ; un écho d'un seq déjà dépassé est une violation.",
		append(stale(lpRecv(8010, lpEcho(1, 0), 90, "stale")),
			lpRecv(8020, lpEcho(9, 80), 90, "accepted"),
			lpRecv(8030, lpEcho(5, 80), 90, "violation"))...)
	prober("linkprobe-prober-stale-above-sent",
		"L'ordre de l'étape 3 : un écho périmé dont received dépasse ce qui a été envoyé est une violation (la règle sur received passe avant celle de l'attente).",
		stale(lpRecv(8010, lpEcho(1, 1000), 90, "violation"))...)
	prober("linkprobe-prober-spacing",
		"Deux sondes à moins de 500 ms : la seconde est refusée, que l'écrivain soit libre ou non (trop tôt passe avant occupé) ; à 500 ms, elle part. Sans flux ouvert, aucune sonde.",
		lpSendRefused(0, 0, true, "no_stream"),
		lpOpen(0),
		lpSend(0, 0, 1), lpSendRefused(499, 5, true, "too_soon"), lpSendRefused(499, 5, false, "too_soon"),
		lpSend(500, 5, 2))
	prober("linkprobe-prober-blocked",
		"Écrivain occupé : la sonde est sautée, sans seq, sans attente, sans toucher à l'espacement ; rien n'est mis en file : la suivante porte le seq suivant.",
		lpOpen(0),
		lpSendRefused(0, 0, false, "skipped"),
		lpSend(0, 0, 1),
		lpSendRefused(1000, 10, false, "skipped"),
		lpSendRefused(1300, 13, false, "skipped"),
		lpSend(1500, 15, 2))
	prober("linkprobe-prober-unknown-seq",
		"Un écho d'un seq jamais envoyé : violation ; le flux est réinitialisé, les échantillons oubliés, rouverture après 60 s ; ce qui serait encore lu du flux fini est sans effet.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 400, 2), lpRecv(15010, lpEcho(2, 360), 401, "accepted"), lpLoss(15010, 100),
		lpRecv(15020, lpEcho(3, 380), 401, "violation"),
		lpLoss(15020, 65535),
		lpSendRefused(16000, 480, true, "no_stream"),
		lpRecv(16010, lpEcho(4, 400), 481, "discarded"),
		lpOpenRefused(75019),
		lpOpen(75020),
		lpSend(75020, 800, 1))
	prober("linkprobe-prober-duplicate",
		"Deux échos du même seq : violation au second.",
		lpOpen(0), lpSend(0, 0, 1),
		lpRecv(10, lpEcho(1, 0), 1, "accepted"), lpRecv(20, lpEcho(1, 0), 1, "violation"))
	prober("linkprobe-prober-received-decreases",
		"received plus petit qu'un écho précédent : violation (le compteur ne décroît jamais).",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 5), 10, "accepted"),
		lpSend(1000, 10, 2), lpRecv(1010, lpEcho(2, 4), 20, "violation"))
	prober("linkprobe-prober-received-equals-sent",
		"received égal à sent(C) à cet instant : accepté (la borne est r ≤ envoyés).",
		lpOpen(0), lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 10), 10, "accepted"))
	prober("linkprobe-prober-violation-forgets-received",
		"Après une violation, dernier_r repart de 0 : sur le flux rouvert, un received plus petit que celui d'avant est accepté.",
		lpOpen(0), lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 100), 200, "accepted"),
		lpRecv(20, lpEcho(5, 100), 200, "violation"),
		lpOpen(60020), lpSend(60020, 300, 1), lpRecv(60030, lpEcho(1, 50), 301, "accepted"))
	prober("linkprobe-prober-end-keeps-received",
		"Après une fin ordinaire, dernier_r reste : sur le flux rouvert, un received plus petit que celui d'avant est une violation.",
		lpOpen(0), lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 100), 200, "accepted"),
		event{"op": "ended", "at_ms": 20},
		lpOpen(60020), lpSend(60020, 300, 1), lpRecv(60030, lpEcho(1, 50), 301, "violation"))
	prober("linkprobe-prober-end-clears-pending",
		"La fin d'un flux vide attente : sur le flux rouvert, l'écho du seq 1 se rapporte au nouveau PROBE seq 1 (accepté), pas à celui du flux fini (il serait tardif).",
		lpOpen(0), lpSend(0, 0, 1),
		event{"op": "ended", "at_ms": 10},
		lpOpen(60010), lpSend(60010, 500, 1), lpRecv(60020, lpEcho(1, 400), 501, "accepted"))
	prober("linkprobe-prober-open-twice",
		"Ouvrir quand un flux est déjà ouvert : refusé (already_open), l'état ne change pas.",
		lpOpen(0), lpSend(0, 0, 1),
		event{"op": "open", "at_ms": 100, "expect": "already_open"},
		lpRecv(110, lpEcho(1, 0), 1, "accepted"))
	prober("linkprobe-prober-level-short-unknown",
		"L'hystérésis survit à un passage par « inconnu » d'au plus 30 s depuis le dernier chiffre connu : moyen à 20 ‰, puis inconnu (50 paquets en 25 s), puis 2 perdus sur 300, 7 ‰, 29 s après le dernier chiffre connu : moyen (7 ≥ 5) ; oubliée, elle aurait donné bon (7 < 10).",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 1000, 2), lpRecv(15010, lpEcho(2, 980), 1001, "accepted"), lpLevel(15010, "fair"),
		lpSend(40000, 1050, 3), lpRecv(40010, lpEcho(3, 1030), 1051, "accepted"), lpLevel(40010, "unknown"),
		lpSend(44000, 1300, 4), lpRecv(44010, lpEcho(4, 1278), 1301, "accepted"), lpLevel(44010, "fair"))
	prober("linkprobe-prober-level-unknown-boundary",
		"L'hystérésis est oubliée **plus** de 30 s après le dernier chiffre connu : à 30 s tout juste, elle est gardée (7 ‰ reste moyen).",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 1000, 2), lpRecv(15010, lpEcho(2, 980), 1001, "accepted"), lpLevel(15010, "fair"),
		lpSend(40000, 1050, 3), lpRecv(40010, lpEcho(3, 1030), 1051, "accepted"), lpLevel(40010, "unknown"),
		lpSend(45000, 1300, 4), lpRecv(45010, lpEcho(4, 1278), 1301, "accepted"), lpLevel(45010, "fair"))
	stalePlus := func(last ...event) []event {
		es := []event{lpOpen(0)}
		for i := range 9 {
			es = append(es, lpSend(i*1000, uint64(i)*10, uint32(i+1)))
		}
		return append(es, last...)
	}
	prober("linkprobe-prober-ignorable-stale",
		"Le budget des ignorables compte tout ECHO lu sans violation, un écho périmé compris : après l'écho périmé du seq 1, deux ignorables sont permis, le troisième est une violation.",
		stalePlus(lpRecv(8010, lpEcho(1, 0), 90, "stale"),
			lpRecv(8020, lpFrame(0x4a, nil), 90, "ignored"),
			lpRecv(8030, lpFrame(0x5a, nil), 90, "ignored"),
			lpRecv(8040, lpFrame(0x6a, nil), 90, "violation"))...)
	prober("linkprobe-prober-received-above-sent",
		"received plus grand que ce que le sondeur a envoyé à cet instant : violation (le pair ne peut pas avoir reçu plus).",
		lpOpen(0), lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 11), 10, "violation"))
	prober("linkprobe-prober-stale-decreasing",
		"Même un écho tardif doit respecter la monotonie : received plus petit qu'avant est une violation.",
		lpOpen(0), lpSend(0, 0, 1), lpSend(1000, 10, 2),
		lpRecv(1500, lpEcho(1, 8), 20, "late"), lpRecv(1510, lpEcho(2, 7), 20, "violation"))
	prober("linkprobe-prober-wrong-type",
		"Une sonde reçue par le sondeur (mauvais sens) : violation.",
		lpOpen(0), lpSend(0, 0, 1), lpRecv(10, lpProbe(1), 1, "violation"))
	prober("linkprobe-prober-ignorable-bound",
		"Au plus un ignorable par ECHO lu, plus un, par flux : le premier est sauté, le second avant tout écho est une violation.",
		lpOpen(0), lpSend(0, 0, 1),
		lpRecv(100, lpFrame(0x4a, []byte{1}), 1, "ignored"),
		lpRecv(101, lpFrame(0x5a, nil), 1, "violation"))
	prober("linkprobe-prober-ignorable-per-echo",
		"Après un écho, un ignorable de plus est permis ; au-delà, violation.",
		lpOpen(0), lpSend(0, 0, 1),
		lpRecv(100, lpFrame(0x4a, nil), 1, "ignored"),
		lpRecv(200, lpEcho(1, 0), 1, "accepted"),
		lpRecv(201, lpFrame(0x6a, nil), 1, "ignored"),
		lpRecv(202, lpFrame(0x7a, nil), 1, "violation"))
	prober("linkprobe-prober-ended",
		"Flux terminé (réinitialisé ou fermé par le pair) : les sondes en attente sont oubliées, ce qui serait encore lu est sans effet, les échantillons de la connexion restent, rouverture après 60 s.",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 400, 2), lpRecv(15010, lpEcho(2, 396), 401, "accepted"),
		lpSend(16000, 800, 3),
		event{"op": "ended", "at_ms": 16005},
		lpRecv(16006, lpEcho(3, 800), 801, "discarded"),
		lpLoss(16006, 10),
		lpSendRefused(16500, 810, true, "no_stream"),
		lpOpenRefused(76004), lpOpen(76005), lpSend(76005, 1200, 1))
	prober("linkprobe-prober-reopen-spacing",
		"L'espacement de 500 ms est par connexion, d'un flux à l'autre ; l'attente de 60 s avant de rouvrir le satisfait toujours : la première sonde du nouveau flux part aussitôt, seq 1.",
		lpOpen(0), lpSend(0, 0, 1),
		event{"op": "ended", "at_ms": 100},
		lpOpenRefused(60099), lpOpen(60100), lpSend(60100, 5, 1))
	prober("linkprobe-prober-restart",
		"Nouvelle connexion : rien ne passe d'une connexion à l'autre (échantillons, flux, attente).",
		lpOpen(0),
		lpSend(0, 0, 1), lpRecv(10, lpEcho(1, 0), 1, "accepted"),
		lpSend(15000, 400, 2), lpRecv(15010, lpEcho(2, 396), 401, "accepted"), lpLoss(15010, 10),
		event{"op": "restart", "at_ms": 15500},
		lpLoss(15500, 65535), lpSendRefused(15500, 0, true, "no_stream"),
		lpOpen(15500), lpSend(15500, 0, 1))

	// ---- responder (§8.5.6), one connection established at the first event ----
	responder := func(id, desc string, events ...event) { script(id, "linkprobe_responder", desc, nil, events...) }
	at := func(t int, from uint32, n int, outcome func(seq uint32) string) []event {
		var es []event
		for i := range n {
			seq := from + uint32(i)
			es = append(es, lpAnswer(t, lpProbe(seq), uint64(seq), true, outcome(seq)))
		}
		return es
	}
	echoed := func(seq uint32) string { return lpEchoOf(seq, uint64(seq)) }
	dropped := func(uint32) string { return "dropped" }
	responder("linkprobe-responder-basic",
		"Chaque sonde reçoit un écho de son seq et du compteur received ; des seq qui sautent sont permis.",
		lpStream(0, "open", "ok"),
		lpAnswer(10000, lpProbe(1), 10, true, lpEchoOf(1, 10)),
		lpAnswer(11000, lpProbe(2), 20, true, lpEchoOf(2, 20)),
		lpAnswer(12000, lpProbe(9), 31, true, lpEchoOf(9, 31)))
	responder("linkprobe-responder-bucket",
		"Crédit de 1500 ms au départ, 500 ms par écho, regagné avec le temps, plafonné à 1500 ms : trois échos d'un coup, puis un par 500 ms ; une sonde sans crédit est jetée sans écho.",
		append(append(append([]event{lpStream(0, "open", "ok")}, at(10000, 1, 3, echoed)...),
			lpAnswer(10000, lpProbe(4), 4, true, "dropped"),
			lpAnswer(10500, lpProbe(5), 5, true, echoed(5)),
			lpAnswer(10600, lpProbe(6), 6, true, "dropped"),
			lpAnswer(11000, lpProbe(7), 7, true, echoed(7))),
			append(at(20000, 8, 3, echoed), lpAnswer(20000, lpProbe(11), 11, true, "dropped"))...)...)
	responder("linkprobe-responder-drops-free",
		"Une sonde jetée n'est jamais une faute : 197 jetées d'un coup après 100 s (le réseau les avait retenues) n'empêchent pas l'écho suivant ; seq reste sous la borne (100 s / 500 ms + 2 = 202).",
		append(append(append([]event{lpStream(0, "open", "ok")}, at(100000, 1, 3, echoed)...), at(100000, 4, 197, dropped)...),
			lpAnswer(100500, lpProbe(201), 201, true, echoed(201)))...)
	responder("linkprobe-responder-stall",
		"Un blocage du réseau de 30 s puis 60 sondes délivrées ensemble : sous la borne de seq (30 s / 500 ms + 2 = 62), aucune faute.",
		append(append([]event{lpStream(0, "open", "ok")}, at(30000, 1, 3, echoed)...), at(30000, 4, 57, dropped)...)...)
	responder("linkprobe-responder-seq-bound",
		"Borne de seq : ⌊(maintenant − ouvert_à) / 500 ms⌋ + 2. Au-dessus, de 20 au plus (sondes écrites avant que le répondeur n'accepte le flux) : jetées sans faute ; au-delà de 22 à l'ouverture, une inondation : violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(0, lpProbe(1), 1, true, echoed(1)),
		lpAnswer(0, lpProbe(2), 2, true, echoed(2)),
		lpAnswer(0, lpProbe(3), 3, true, "dropped"),
		lpAnswer(0, lpProbe(22), 4, true, "dropped"),
		lpAnswer(0, lpProbe(23), 5, true, "violation"))
	responder("linkprobe-responder-accept-delay",
		"Le flux est accepté 2 s après que le sondeur a commencé : les sondes 1 à 5 arrivent d'un coup à l'acceptation ; 1 et 2 ont un écho, 3 à 5 sont jetées sans faute ; 2 s plus tard la borne est 6 et la sonde 6 a son écho.",
		lpStream(0, "open", "ok"),
		lpAnswer(0, lpProbe(1), 1, true, echoed(1)),
		lpAnswer(0, lpProbe(2), 2, true, echoed(2)),
		lpAnswer(0, lpProbe(3), 3, true, "dropped"),
		lpAnswer(0, lpProbe(4), 4, true, "dropped"),
		lpAnswer(0, lpProbe(5), 5, true, "dropped"),
		lpAnswer(2000, lpProbe(6), 6, true, echoed(6)))
	responder("linkprobe-responder-seq-from-accept",
		"La borne de seq part de l'acceptation du flux, pas de l'établissement de la connexion : connexion à 0, flux accepté à 100 s, seq 23 à 100 s dépasse 2 + 20 : violation.",
		lpStream(0, "end", "ok"),
		lpStream(100000, "open", "ok"),
		lpAnswer(100000, lpProbe(23), 1, true, "violation"))
	responder("linkprobe-responder-seq-skew",
		"La borne compte le temps écoulé avec une marge de 1/64 contre les horloges : à 32 s, ⌊(32000 + 500) / 500⌋ + 2 = 67 ; seq 67 a son écho.",
		lpStream(0, "open", "ok"),
		lpAnswer(0, lpProbe(1), 1, true, echoed(1)),
		lpAnswer(32000, lpProbe(67), 2, true, lpEchoOf(67, 2)))
	responder("linkprobe-responder-drop-updates-seq",
		"Une sonde jetée au-dessus de la borne avance dernier_seq : le même seq relu ensuite est une violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(0, lpProbe(3), 1, true, "dropped"),
		lpAnswer(1000, lpProbe(3), 2, true, "violation"))
	responder("linkprobe-responder-ignorable-dropped",
		"Le budget des ignorables compte toute sonde lue sans violation, une sonde jetée comprise : après une sonde jetée, deux ignorables sont permis, le troisième est une violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(0, lpProbe(3), 1, true, "dropped"),
		lpAnswer(1, lpFrame(0x4a, nil), 1, true, "ignored"),
		lpAnswer(2, lpFrame(0x5a, nil), 1, true, "ignored"),
		lpAnswer(3, lpFrame(0x6a, nil), 1, true, "violation"))
	responder("linkprobe-responder-credit-exact",
		"Crédit exact : trois échos d'un coup à 10 s vident le crédit ; à 10,499 s il vaut 499 ms : sonde jetée ; à 10,5 s, 500 ms : écho.",
		append(append([]event{lpStream(0, "open", "ok")}, at(10000, 1, 3, echoed)...),
			lpAnswer(10499, lpProbe(4), 4, true, "dropped"),
			lpAnswer(10500, lpProbe(5), 5, true, echoed(5)))...)
	responder("linkprobe-responder-end-without-stream",
		"Une fin de flux sans flux ouvert est sans effet : aucun refus n'en découle.",
		lpStream(0, "end", "ok"),
		lpStream(1, "open", "ok"))
	responder("linkprobe-responder-reopen-gap",
		"Un nouveau flux moins de 30 s après la fin du précédent, quelle que soit la fin, est refusé.",
		lpStream(0, "open", "ok"),
		lpStream(10000, "end", "ok"),
		lpStream(39999, "open", "refused"),
		lpStream(40000, "open", "ok"))
	responder("linkprobe-responder-blocked",
		"Écrivain occupé : la sonde est jetée sans dépenser de crédit ; aucun écho n'est mis en file.",
		lpStream(0, "open", "ok"),
		lpAnswer(10000, lpProbe(1), 1, false, "dropped"),
		lpAnswer(10000, lpProbe(2), 2, true, echoed(2)),
		lpAnswer(10000, lpProbe(3), 3, true, echoed(3)),
		lpAnswer(10000, lpProbe(4), 4, true, echoed(4)),
		lpAnswer(10000, lpProbe(5), 5, true, "dropped"))
	responder("linkprobe-responder-second-stream",
		"Un second flux pendant que le premier est ouvert est refusé ; le premier continue.",
		lpStream(0, "open", "ok"),
		lpStream(10, "open", "refused"),
		lpAnswer(20, lpProbe(1), 1, true, echoed(1)))
	responder("linkprobe-responder-replay",
		"Un seq qui ne croît pas (rejeu, désordre) : violation ; le flux est fini, la suite est sans effet ; tout nouveau flux de la connexion est refusé pendant 60 s.",
		lpStream(0, "open", "ok"),
		lpAnswer(10000, lpProbe(5), 1, true, lpEchoOf(5, 1)),
		lpAnswer(11000, lpProbe(5), 2, true, "violation"),
		lpAnswer(11010, lpProbe(6), 3, true, "discarded"),
		lpStream(11020, "open", "refused"),
		lpStream(70999, "open", "refused"),
		lpStream(71000, "open", "ok"),
		lpAnswer(71000, lpProbe(1), 4, true, lpEchoOf(1, 4)))
	responder("linkprobe-responder-reorder",
		"seq 3 après seq 5 : violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(10000, lpProbe(5), 1, true, lpEchoOf(5, 1)),
		lpAnswer(11000, lpProbe(3), 2, true, "violation"))
	responder("linkprobe-responder-wrong-type",
		"Un écho reçu par le répondeur (mauvais sens) : violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(0, lpEcho(1, 1), 1, true, "violation"))
	responder("linkprobe-responder-ignorable-bound",
		"Au plus un ignorable par PROBE lu, plus un, par flux : le premier est sauté, le second avant toute sonde est une violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(10, lpFrame(0x4a, nil), 0, true, "ignored"),
		lpAnswer(20, lpFrame(0x5a, []byte{1}), 0, true, "violation"))
	responder("linkprobe-responder-ignorable-per-probe",
		"Après une sonde, un ignorable de plus est permis ; au-delà, violation.",
		lpStream(0, "open", "ok"),
		lpAnswer(10, lpFrame(0x4a, nil), 0, true, "ignored"),
		lpAnswer(10000, lpProbe(1), 7, true, lpEchoOf(1, 7)),
		lpAnswer(10001, lpFrame(0x6a, nil), 0, true, "ignored"),
		lpAnswer(10002, lpFrame(0x7a, nil), 0, true, "violation"))
	responder("linkprobe-responder-ended",
		"Après la fin du flux, ce qui serait encore lu est sans effet.",
		lpStream(0, "open", "ok"),
		lpStream(10, "end", "ok"),
		lpAnswer(20, lpProbe(1), 1, true, "discarded"))

	// ---- host (§8.5.3 to §8.5.6): events of one connection, actions expected ----
	host := func(id, desc, transport string, roles []string, events ...event) {
		script(id, "linkprobe_host", desc, map[string]any{"transport": transport, "roles": append([]string{}, roles...)}, events...)
	}
	member := []string{"member"}
	for _, tr := range []string{"tcp", "circuit", "webtransport"} {
		host("linkprobe-host-"+tr,
			"Hors d'une connexion QUIC brute (v1 ou v2), la sonde n'existe pas : jamais de flux ouvert, un flux du pair est refusé.",
			tr, member,
			lpTick(0, 0, true), lpInOpened(0, 1, refuse(1)), lpTick(1000, 10, true))
	}
	host("linkprobe-host-guest",
		"Un invité du salon (aucun rôle) : ni sonde ni réponse.",
		"quic", nil,
		lpTick(0, 0, true), lpInOpened(0, 1, refuse(1)))
	host("linkprobe-host-relay",
		"Le relais qu'on utilise : on le sonde (premier saut), on ne lui répond pas.",
		"quic", []string{"relay"},
		lpTick(0, 0, true, "open_out"), lpInOpened(0, 1, refuse(1)))
	host("linkprobe-host-relay-client",
		"Vu du relais, un pair qui a une réservation ou un circuit actif : on lui répond, on ne le sonde pas ; le répondeur n'ouvre jamais de flux.",
		"quic", []string{"relay_client"},
		lpTick(0, 0, true), lpInOpened(0, 1), lpInData(10, 1, lpProbe(1), 3, true, win(1, lpEcho(1, 3))), lpTick(1000, 5, true))
	host("linkprobe-host-two-roles",
		"Deux rôles : leur union (relais utilisé et client de ce relais) : on sonde et on répond.",
		"quic", []string{"relay", "relay_client"},
		lpTick(0, 0, true, "open_out"), lpInOpened(0, 1))
	host("linkprobe-host-roles-change",
		"Un rôle qui change : le flux que les nouveaux rôles excluent est réinitialisé, et le rôle est réévalué à chaque ouverture.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpNego(0, true, 0, true, wout(lpProbe(1))),
		lpInOpened(10, 1),
		lpRoles(100, []string{"relay"}, resetIn(1)),
		lpRoles(200, nil, "reset_out"),
		lpTick(70000, 10, true),
		lpInOpened(70001, 2, refuse(2)))
	host("linkprobe-host-negotiate",
		"Le sondeur ouvre, attend la fin de la négociation (pas de PROBE pendant), puis envoie la première sonde aussitôt.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpTick(1000, 5, true),
		lpNego(1100, true, 6, true, wout(lpProbe(1))),
		lpOutData(1150, lpEcho(1, 6), 7),
		lpTick(1600, 20, true, wout(lpProbe(2))),
		lpOutData(1650, lpEcho(2, 19), 21),
		lpHostLoss(1650, 65535))
	host("linkprobe-host-unsupported",
		"Le pair refuse l'identifiant (na) : plus jamais de sonde sur cette connexion ; la réponse à ses sondes continue.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpNego(10, false, 0, true),
		lpTick(1000, 5, true), lpTick(70000, 900, true), lpTick(700000, 9000, true),
		lpInOpened(700001, 1), lpInData(700002, 1, lpProbe(1), 50, true, win(1, lpEcho(1, 50))))
	host("linkprobe-host-open-failed",
		"Une autre fin de la négociation (flux fini avant) : rouverture après 60 s.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpOutEnded(10),
		lpTick(1000, 5, true), lpTick(60009, 5, true), lpTick(60010, 5, true, "open_out"))
	host("linkprobe-host-negotiation-timeout",
		"Un pair muet pendant la négociation : au bout de 10 s, le flux est réinitialisé comme un autre échec ; rouverture 60 s plus tard.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpTick(9999, 5, true),
		lpTick(10000, 5, true, "reset_out"),
		lpTick(69999, 5, true),
		lpTick(70000, 5, true, "open_out"))
	host("linkprobe-host-close-negotiating",
		"Cesser de mesurer pendant la négociation : le FIN part dès qu'elle réussit, aucune sonde ; la connexion n'est plus sondée.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		event{"op": "close", "at_ms": 10, "expect": acts{}},
		lpNego(20, true, 0, true, "fin_out"),
		lpOutData(30, lpEcho(1, 0), 1),
		lpOutEnded(40),
		lpTick(60020, 5, true),
		lpTick(600000, 5, true))
	host("linkprobe-host-negotiated-unasked",
		"Une fin de négociation sans négociation en cours est sans effet.",
		"quic", member,
		lpNego(0, true, 0, true),
		lpTick(10, 0, true, "open_out"))
	ok2 := func(more ...event) []event {
		return append([]event{
			lpTick(0, 0, true, "open_out"),
			lpNego(0, true, 0, true, wout(lpProbe(1))),
			lpOutData(10, lpEcho(1, 0), 1),
			lpTick(15000, 400, true, wout(lpProbe(2))),
		}, more...)
	}
	host("linkprobe-host-chunks",
		"Un écho coupé en deux lectures est reçu entier ; un message mal formé réinitialise le flux sortant.",
		"quic", member,
		ok2(
			lpOutData(15010, lpEcho(2, 360)[:5], 401),
			lpOutData(15020, lpEcho(2, 360)[5:], 401),
			lpHostLoss(15020, 100),
			event{"op": "level", "at_ms": 15020, "expect": "poor"},
			lpOutData(15030, []byte{0x00}, 401, "reset_out"),
			lpHostLoss(15030, 65535))...)
	host("linkprobe-host-truncated-end",
		"Un message tronqué puis la fin du flux : simple fin de flux, les échantillons restent.",
		"quic", member,
		ok2(
			lpOutData(15010, lpEcho(2, 360), 401),
			lpTick(16000, 800, true, wout(lpProbe(3))),
			lpOutData(16010, lpEcho(3, 720)[:7], 801),
			lpOutEnded(16020),
			lpHostLoss(16020, 100),
			lpOutData(16030, lpEcho(3, 720)[7:], 801),
			lpHostLoss(16030, 100),
			lpTick(17000, 900, true),
			lpTick(76020, 1000, true, "open_out"))...)
	host("linkprobe-host-close",
		"Le sondeur cesse de mesurer : FIN ; il lit et jette jusqu'à la fin du pair, sans violation ; il ne rouvre plus de flux sur cette connexion, et répond toujours aux sondes du pair.",
		"quic", member,
		ok2(
			event{"op": "close", "at_ms": 15005, "expect": acts{"fin_out"}},
			lpOutData(15010, lpEcho(2, 360), 401),
			lpOutData(15020, []byte{0x00}, 401),
			lpOutEnded(15030),
			lpTick(16000, 500, true),
			lpTick(75005, 600, true),
			lpInOpened(75006, 1),
			lpInData(75007, 1, lpProbe(1), 9, true, win(1, lpEcho(1, 9))))...)
	host("linkprobe-host-closing-timeout",
		"Un pair qui ne ferme jamais son sens après le FIN du sondeur : réinitialisation au bout de 60 s ; pas de rouverture.",
		"quic", member,
		ok2(
			event{"op": "close", "at_ms": 15005, "expect": acts{"fin_out"}},
			lpOutData(30000, lpEcho(2, 360), 401),
			lpTick(75004, 600, true),
			lpTick(75005, 600, true, "reset_out"),
			lpTick(75006, 600, true))...)
	host("linkprobe-host-closing-junk",
		"Après son FIN, le sondeur lit et jette au plus 1024 octets ; au-delà, il réinitialise le flux (un pair qui en ferait un tuyau à déchets).",
		"quic", member,
		ok2(
			event{"op": "close", "at_ms": 15005, "expect": acts{"fin_out"}},
			lpOutData(15010, make([]byte, 1000), 401),
			lpOutData(15020, make([]byte, 24), 401),
			lpOutData(15030, []byte{0x00}, 401, "reset_out"),
			lpOutData(15040, make([]byte, 100), 401))...)
	host("linkprobe-host-blocked",
		"Écrivain occupé : la sonde est sautée (la première, puis une autre), la suivante part avec le seq suivant ; côté répondeur, la sonde est jetée sans dépenser de crédit.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpNego(0, true, 0, false),
		lpTick(1000, 10, true, wout(lpProbe(1))),
		lpTick(1500, 15, false),
		lpTick(2500, 25, true, wout(lpProbe(2))),
		lpInOpened(2600, 1),
		lpInData(4600, 1, lpProbe(1), 3, false),
		lpInData(4600, 1, lpProbe(2), 4, true, win(1, lpEcho(2, 4))),
		lpInData(4600, 1, lpProbe(3), 5, true, win(1, lpEcho(3, 5))),
		lpInData(4600, 1, lpProbe(4), 6, true, win(1, lpEcho(4, 6))),
		lpInData(4600, 1, lpProbe(5), 7, true))
	host("linkprobe-host-one-message",
		"Deux sondes lues d'un coup : un seul écho part (l'écrivain ne tient qu'un message, au plus un ECHO par lecture) ; aucun écho n'est retardé ni groupé.",
		"quic", member,
		lpInOpened(0, 1),
		lpInData(600, 1, cat(lpProbe(1), lpProbe(2)), 3, true, win(1, lpEcho(1, 3))),
		lpInData(700, 1, lpProbe(3), 4, true, win(1, lpEcho(3, 4))))
	host("linkprobe-host-second-inbound",
		"Un second flux entrant pendant que le premier est ouvert est refusé ; ses événements suivants n'ont aucun effet ; le premier continue.",
		"quic", member,
		lpInOpened(0, 1), lpInOpened(10, 2, refuse(2)),
		lpInData(15, 2, lpProbe(1), 2, true),
		lpInEnded(20, 2, false),
		lpInData(30, 1, lpProbe(1), 3, true, win(1, lpEcho(1, 3))))
	host("linkprobe-host-roles-refusal",
		"Un changement de rôle qui réinitialise le flux entrant est une fin de flux : un nouveau flux est refusé pendant 30 s, même quand les rôles le permettent à nouveau.",
		"quic", member,
		lpInOpened(0, 1),
		lpRoles(100, []string{"relay"}, resetIn(1)),
		lpRoles(200, member),
		lpInOpened(1000, 2, refuse(2)),
		lpInOpened(30100, 3))
	host("linkprobe-host-inbound-fin",
		"Le sondeur ferme son sens (FIN) : le répondeur ferme le sien ; ce qui suivrait est sans effet ; un nouveau flux moins de 30 s après est refusé, puis accepté.",
		"quic", member,
		lpInOpened(0, 1),
		lpInData(10, 1, lpProbe(1), 3, true, win(1, lpEcho(1, 3))),
		lpInEnded(20, 1, true, "fin_in:1"),
		lpInData(30, 1, lpProbe(2), 4, true),
		lpInOpened(40, 2, refuse(2)),
		lpInOpened(30020, 3),
		lpInData(30030, 3, lpProbe(1), 5, true, win(3, lpEcho(1, 5))))
	host("linkprobe-host-ban",
		"Une violation du côté répondeur : le flux est réinitialisé et tout nouveau flux de sonde de la connexion est refusé, sans être lu, pendant 60 s.",
		"quic", member,
		lpInOpened(0, 7),
		lpInData(10, 7, lpEcho(1, 1), 3, true, resetIn(7)),
		lpInOpened(20, 2, refuse(2)),
		lpInOpened(60009, 3, refuse(3)),
		lpInOpened(60010, 4),
		lpInData(60020, 4, lpProbe(1), 9, true, win(4, lpEcho(1, 9))))
	host("linkprobe-host-inbound-malformed",
		"Un message mal formé sur le flux entrant est une violation du côté répondeur.",
		"quic", member,
		lpInOpened(0, 1),
		lpInData(10, 1, []byte{0x41}, 3, true, resetIn(1)),
		lpInOpened(20, 2, refuse(2)))
	host("linkprobe-host-peer-reset-refusal",
		"Une réinitialisation du flux entrant par le pair est une fin comme une autre : un nouveau flux est refusé pendant 30 s.",
		"quic", member,
		lpInOpened(0, 1),
		lpInEnded(10, 1, false),
		lpInOpened(20, 2, refuse(2)),
		lpInOpened(30010, 3))
	host("linkprobe-host-close-idle",
		"Cesser de mesurer sans flux ouvert : aucune action, et la connexion n'est plus sondée.",
		"quic", member,
		event{"op": "close", "at_ms": 0, "expect": acts{}},
		lpTick(10, 0, true),
		lpTick(600000, 0, true))
	host("linkprobe-host-roles-negotiating",
		"Un rôle qui change pendant la négociation : le flux est réinitialisé, c'est un autre échec : rouverture après 60 s.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpRoles(10, nil, "reset_out"),
		lpRoles(20, member),
		lpTick(30, 0, true),
		lpTick(60010, 0, true, "open_out"))
	host("linkprobe-host-roles-open",
		"Un rôle qui change sur un flux ouvert : le flux est réinitialisé, c'est une fin de flux : rouverture après 60 s.",
		"quic", member,
		lpTick(0, 0, true, "open_out"),
		lpNego(0, true, 0, true, wout(lpProbe(1))),
		lpRoles(100, nil, "reset_out"),
		lpRoles(200, member),
		lpTick(1000, 10, true),
		lpTick(60100, 10, true, "open_out"))
	host("linkprobe-host-inbound-flood",
		"Trois sondes lues d'un coup 10 ms après l'ouverture : écho de la première seule (un par lecture), la troisième dépasse la borne de seq (2) et est jetée ; seq 23 dépasse la borne de plus de 20 : réinitialisation.",
		"quic", member,
		lpInOpened(0, 1),
		lpInData(10, 1, cat(lpProbe(1), lpProbe(2), lpProbe(3)), 3, true, win(1, lpEcho(1, 3))),
		lpInData(20, 1, lpProbe(23), 4, true, resetIn(1)))
	host("linkprobe-host-inbound-ignorables",
		"Deux ignorables avant toute sonde : le second est une violation (un par message, plus un).",
		"quic", member,
		lpInOpened(0, 1),
		lpInData(10, 1, cat(lpFrame(0x4a, nil), lpFrame(0x5a, nil)), 3, true, resetIn(1)))

	// ---- PTO (§8.5.5 step 3.6) ----
	pto := func(id, desc string, srtt, rttvar int, mad any, want int) {
		v = append(v, vector{
			ID: id, Status: "proposed", Enforce: lpEnforce, Scope: "transport", Desc: desc,
			Decoder: "linkprobe_pto", Context: map[string]any{"srtt_us": srtt, "rttvar_us": rttvar, "max_ack_delay_us": mad},
			Verdict: "accept", Expect: map[string]any{"pto_ms": want},
		})
	}
	pto("linkprobe-pto", "50 ms + 4 × 2 ms + 25 ms = 83 ms.", 50000, 2000, 25000, 83)
	pto("linkprobe-pto-granularity", "4 × rttvar sous 1 ms : 1 ms (kGranularity) ; 50 + 1 + 25 = 76 ms.", 50000, 100, 25000, 76)
	pto("linkprobe-pto-granularity-zero", "rttvar nul : kGranularity seul, 1 ms ; 50 + 1 + 25 = 76 ms (sans lui, 75).", 50000, 0, 25000, 76)
	pto("linkprobe-pto-no-max-ack-delay", "max_ack_delay non exposé (null) : 26 ms, ce qu'annonce quic-go ; 50 + 8 + 26 = 84 ms.", 50000, 2000, nil, 84)
	pto("linkprobe-pto-round-up", "50,3 + 8 + 25 = 83,3 ms : arrondi vers le haut, 84 ms.", 50300, 2000, 25000, 84)
	pto("linkprobe-pto-zero-max-ack-delay", "max_ack_delay exposé à 0 : 10 + 4 + 0 = 14 ms.", 10000, 1000, 0, 14)
	// ---- level (§8.5.5 step 6): one step at each threshold and one below ----
	level := func(kind, from string, value int64, want string) {
		v = append(v, vector{
			ID: fmt.Sprintf("linkprobe-level-%s-%s-%d", kind, from, value), Status: "proposed", Enforce: lpEnforce, Scope: "transport",
			Desc:    fmt.Sprintf("Niveau, %s : depuis %s, %d donne %s.", kind, from, value, want),
			Decoder: "linkprobe_level", Context: map[string]any{"kind": kind, "from": from, "value": value},
			Verdict: "accept", Expect: map[string]any{"level": want},
		})
	}
	for _, c := range []struct {
		kind, from string
		value      int64
		want       string
	}{
		{"loss", "unknown", 9, "good"},
		{"loss", "unknown", 10, "fair"},
		{"loss", "unknown", 99, "fair"},
		{"loss", "unknown", 100, "poor"},
		{"loss", "good", 24, "good"},
		{"loss", "good", 25, "fair"},
		{"loss", "good", 149, "fair"},
		{"loss", "good", 150, "poor"},
		{"loss", "fair", 5, "fair"},
		{"loss", "fair", 4, "good"},
		{"loss", "fair", 149, "fair"},
		{"loss", "fair", 150, "poor"},
		{"loss", "poor", 50, "poor"},
		{"loss", "poor", 49, "fair"},
		{"loss", "poor", 5, "fair"},
		{"loss", "poor", 4, "good"},
		{"rtt", "unknown", 499, "good"},
		{"rtt", "unknown", 500, "fair"},
		{"rtt", "unknown", 1999, "fair"},
		{"rtt", "unknown", 2000, "poor"},
		{"rtt", "good", 624, "good"},
		{"rtt", "good", 625, "fair"},
		{"rtt", "good", 2499, "fair"},
		{"rtt", "good", 2500, "poor"},
		{"rtt", "fair", 400, "fair"},
		{"rtt", "fair", 399, "good"},
		{"rtt", "fair", 2499, "fair"},
		{"rtt", "fair", 2500, "poor"},
		{"rtt", "poor", 1600, "poor"},
		{"rtt", "poor", 1599, "fair"},
		{"rtt", "poor", 400, "fair"},
		{"rtt", "poor", 399, "good"},
	} {
		level(c.kind, c.from, c.value, c.want)
	}
	return v
}
