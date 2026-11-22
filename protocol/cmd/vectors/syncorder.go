// SPDX-License-Identifier: MPL-2.0

package main

// The inbound order of a peer's sync streams (spec §8.7.3, §8.7.4, block PU1b, rules P-8 and P-9). Written from the
// text of the spec, not from any implementation. Status proposed, enforced from step 3 (protocol/syncack implements
// the order and the refusal since PU1b's step 2).

// soScript is a script of the responder's order of one peer (decoder sync_order, §8.7.3): two places; each event
// names a stream and what the order answers.
func soScript(id, desc string, events ...event) vector {
	return vector{
		ID: id, Status: "proposed", Enforce: "step-3", Desc: desc, Decoder: "sync_order", Verdict: "accept", Scope: "transport",
		Context: map[string]any{"places": 2, "reorder_wait_ms": 60000}, Events: events,
	}
}

// soEv: arrive (place | busy), ack (acked), ready (turn | wait), end (leave; wake: the streams whose turn came).
func soEv(op string, at, stream int, expect string, wake ...int) event {
	e := event{"op": op, "at_ms": at, "stream": stream, "expect": expect}
	if op == "end" {
		if wake == nil {
			wake = []int{}
		}
		e["wake"] = wake
	}
	return e
}

func soEnd(at, stream int, delivered bool, wake ...int) event {
	e := soEv("end", at, stream, "leave", wake...)
	e["delivered"] = delivered
	return e
}

// soRefusal is the opener's reading of a reset before the ACK (decoder sync_refusal, §8.7.4): code is the stream
// error code (absent: the stream ended), elapsed the time since the first open, attempt the capacity refusals so far.
func soRefusal(id, desc, role string, code any, elapsed, attempt int, action string, wait int) vector {
	ctx := map[string]any{"role": role, "elapsed_ms": elapsed, "attempt": attempt, "budget_ms": 15000}
	if code != nil {
		ctx["reset_code"] = code
	}
	expect := map[string]any{"action": action}
	if action == "retry" {
		expect["wait_ms"] = wait
	}
	return vector{
		ID: id, Status: "proposed", Enforce: "step-3", Desc: desc, Decoder: "sync_refusal", Verdict: "accept", Scope: "transport",
		Context: ctx, Expect: expect,
	}
}

func syncOrderVectors() []vector {
	const busy = 0x1002
	return []vector{
		soScript("sync-order-at-bound", "Deux places tenues : le troisième flux est refusé pour capacité (P-8).",
			soEv("arrive", 0, 1, "place"), soEv("arrive", 10, 2, "place"), soEv("arrive", 20, 3, "busy")),
		soScript("sync-order-recovery", "Une place rendue (flux lu jusqu'à sa fin) : le flux suivant réveillé, le nouvel essai placé.",
			soEv("arrive", 0, 1, "place"), soEv("ack", 1, 1, "acked"), soEv("arrive", 10, 2, "place"), soEv("ack", 11, 2, "acked"),
			soEv("arrive", 20, 3, "busy"), soEnd(30, 1, true, 2), soEv("arrive", 80, 4, "place"), soEv("ready", 81, 4, "wait")),
		soScript("sync-order-turns", "Le flux de tête lit ; le second attend son tour.",
			soEv("arrive", 0, 1, "place"), soEv("ready", 0, 1, "turn"), soEv("arrive", 5, 2, "place"), soEv("ready", 5, 2, "wait"),
			soEnd(40, 1, true, 2), soEv("ready", 40, 2, "turn")),
		soScript("sync-order-remove-waiting", "Un flux parti sans trame pendant qu'il attendait quitte l'ordre aussitôt ; le suivant attend le flux de tête (P-9).",
			soEv("arrive", 0, 1, "place"), soEv("arrive", 5, 2, "place"), soEnd(10, 2, false), soEv("arrive", 20, 3, "place"),
			soEv("ready", 20, 3, "wait"), soEnd(50, 1, true, 3), soEv("ready", 50, 3, "turn")),
		soScript("sync-order-remove-ahead", "Le flux devant un flux en attente part sans trame : le flux en attente est réveillé et lit (P-9).",
			soEv("arrive", 0, 1, "place"), soEv("arrive", 5, 2, "place"), soEv("ready", 5, 2, "wait"), soEnd(9, 1, false, 2),
			soEv("ready", 9, 2, "turn")),
		soScript("sync-order-byzantine-loop", "Un premier flux tenu ouvert, les suivants réinitialisés pendant leur accusé : jamais plus de deux places, aucune gardée (P-8, P-9).",
			soEv("arrive", 0, 1, "place"), soEv("arrive", 1, 2, "place"), soEnd(2, 2, false), soEv("arrive", 3, 3, "place"),
			soEnd(4, 3, false), soEv("arrive", 5, 4, "place"), soEnd(6, 4, false), soEv("arrive", 7, 5, "place"),
			soEv("arrive", 8, 6, "busy")),
		soScript("sync-order-eof-empty-waiting", "Un flux en attente fermé en écriture par l'ouvreur sans aucune trame quitte l'ordre aussitôt, comme un flux réinitialisé (P-9).",
			soEv("arrive", 0, 1, "place"), soEv("arrive", 5, 2, "place"), soEv("ready", 5, 2, "wait"), soEnd(10, 2, false),
			soEv("arrive", 20, 3, "place"), soEv("ready", 20, 3, "wait")),
		soScript("sync-order-expire", "Attente de son tour : 60 s, puis le flux lit quand même (l'ordre est abandonné).",
			soEv("arrive", 0, 1, "place"), soEv("arrive", 100, 2, "place"), soEv("ready", 60099, 2, "wait"),
			soEv("ready", 60100, 2, "turn")),
		soScript("sync-order-refused-leaves", "Un flux refusé (pair non admis) part sans trame et rend sa place.",
			soEv("arrive", 0, 1, "place"), soEnd(1, 1, false), soEv("arrive", 2, 2, "place"), soEv("ready", 2, 2, "turn")),

		soRefusal("sync-refusal-busy-first", "Réinitialisation 0x1002 avant l'accusé : refus pour capacité, nouvel essai après 50 ms.", "session", busy, 0, 0, "retry", 50),
		soRefusal("sync-refusal-busy-doubled", "Troisième refus pour capacité : attente doublée (400 ms).", "session", busy, 2000, 3, "retry", 400),
		soRefusal("sync-refusal-busy-capped", "Attente plafonnée à 1 s.", "session", busy, 4000, 6, "retry", 1000),
		soRefusal("sync-refusal-busy-in-budget", "Nouvel essai qui tient dans les 15 s de l'ouverture.", "session", busy, 14949, 0, "retry", 50),
		soRefusal("sync-refusal-busy-out-of-budget", "Nouvel essai qui dépasserait les 15 s : ClosePeer (P-5).", "session", busy, 14950, 0, "close_peer", 0),
		soRefusal("sync-refusal-busy-move", "Passage au direct refusé pour capacité : nouvel essai, session inchangée.", "move", busy, 100, 1, "retry", 100),
		soRefusal("sync-refusal-busy-move-out-of-budget", "Passage au direct, budget épuisé : abandon.", "move", busy, 14990, 0, "abandon", 0),
		soRefusal("sync-refusal-other-code", "Réinitialisation d'un autre code : refus, ClosePeer (P-4).", "session", 0, 100, 0, "close_peer", 0),
		soRefusal("sync-refusal-negotiation-code", "Code 0x1001 (négociation échouée) : refus.", "session", 0x1001, 100, 0, "close_peer", 0),
		soRefusal("sync-refusal-eof", "Fin du flux avant l'accusé : refus, ClosePeer.", "session", nil, 100, 0, "close_peer", 0),
		soRefusal("sync-refusal-other-move", "Passage au direct refusé (autre code) : abandon.", "move", 0, 100, 0, "abandon", 0),
	}
}
