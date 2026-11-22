// SPDX-License-Identifier: MPL-2.0

package main

import "maps"

// The lifecycle of an inbound stream on the two ordered lanes, sync and bulk (spec §8.1, §8.7.3, §16.1, block PU1c,
// rules P-10 to P-14). Written from the text of the spec, not from any implementation. Status proposed, enforced from
// step 3 (CurrentStep when PU1c's reference, protocol/syncack's Lifecycle, ProbeSchedule and QUICWindowOK, landed).

// ilScript is a script of one inbound stream's reader (decoder inbound_lifecycle, §8.7.3): each event is what the
// reader observes, and expect is the state it is in afterwards; leave tells it gave its place back at that event,
// prefix how many probed bytes it holds, deliver how many whole frames it handed to the consumer at that event.
func ilScript(id, lane, desc string, events ...event) vector {
	return vector{
		ID: id, Status: "proposed", Enforce: "step-3", Desc: desc, Decoder: "inbound_lifecycle", Verdict: "accept", Scope: "transport",
		Context: map[string]any{"lane": lane, "places": 2, "probe_every_ms": 250, "probe_max": 256, "reorder_wait_ms": 60000, "up_wait_ms": 30000},
		Events:  events,
	}
}

// ilEv is one observation: op is arrive (placed), ack (ok), read (n bytes and an error: none, deadline, eof, reset,
// conn_lost, node_closed), turn, up, budget (room), decode (whole frames, truncated tail); expect the state after it.
func ilEv(op string, at int, expect string, fields map[string]any) event {
	e := event{"op": op, "at_ms": at, "expect": expect}
	maps.Copy(e, fields)
	return e
}

// ilRead is a probe Read of at most one byte that returned n bytes and err.
func ilRead(at, n int, err, expect string, prefix int, leave bool) event {
	return ilEv("read", at, expect, map[string]any{"n": n, "err": err, "prefix": prefix, "leave": leave})
}

// ilProbe is the probe schedule (decoder inbound_probe, §8.7.3, rule P-11): attempts are the start times of the
// probe's reads, bytes the times at which the probe took a byte; the reader is valid when no gap between two
// attempts exceeds probe_every_ms and it holds at most probe_max bytes.
func ilProbe(id, desc string, attempts, bytes []int, verdict string) vector {
	return vector{
		ID: id, Status: "proposed", Enforce: "step-3", Desc: desc, Decoder: "inbound_probe", Verdict: verdict, Scope: "transport",
		Context: map[string]any{"probe_every_ms": 250, "probe_max": 256, "attempts_ms": attempts, "bytes_ms": bytes},
	}
}

// ilWindow is the QUIC connection window rule (decoder quic_window, §16.1, rule P-13): (places × lanes + 1) ×
// stream_window ≤ (1 − 1/4) × conn_window.
func ilWindow(id, desc string, stream, conn int, verdict string) vector {
	return vector{
		ID: id, Status: "proposed", Enforce: "step-3", Desc: desc, Decoder: "quic_window", Verdict: verdict, Scope: "transport",
		Context: map[string]any{"places": 2, "lanes": 2, "stream_window": stream, "update_threshold_den": 4, "conn_window": conn},
	}
}

// every is n times from 0, step apart.
func every(step, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i * step
	}
	return out
}

func inboundVectors() []vector {
	none := map[string]any{}
	placed := map[string]any{"placed": true}
	vs := []vector{}
	for _, lane := range []string{"sync", "bulk"} {
		p := "inbound-" + lane + "-"
		vs = append(vs,
			ilScript(p+"refused", lane, "Pas de place : réinitialisé avec 0x1002, aucun octet lu, rien gardé (P-8, P-14).",
				ilEv("arrive", 0, "refused", map[string]any{"placed": false})),
			ilScript(p+"ack-fails", lane, "L'accusé ne s'écrit pas (flux réinitialisé pendant l'acceptation) : la place est rendue aussitôt (P-9, P-14).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 3, "gone", map[string]any{"ok": false, "leave": true})),
			ilScript(p+"waiting-reset-empty", lane, "En attente sans octet non lu : la lecture en cours voit la réinitialisation aussitôt (P-11).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilRead(40, 0, "reset", "gone", 0, true)),
			ilScript(p+"waiting-eof-empty", lane, "Fermé en écriture sans aucun octet : quitte l'ordre aussitôt (P-9, P-11).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilRead(40, 0, "eof", "gone", 0, true)),
			ilScript(p+"waiting-deadline-reevaluates", lane, "L'échéance périodique n'est pas une fin : le lecteur réévalue son tour et relit (P-11).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilRead(251, 0, "deadline", "waiting", 0, false), ilRead(501, 0, "deadline", "waiting", 0, false)),
			ilScript(p+"waiting-byte-then-reset", lane, "Un octet sondé, puis la réinitialisation (qui passe devant les octets en tampon) : vue à la sonde suivante, au plus 250 ms après (P-11).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilRead(10, 1, "none", "waiting", 1, false), ilRead(260, 0, "reset", "gone", 0, true)),
			ilScript(p+"waiting-n-with-error", lane, "Une lecture qui rend un octet et une erreur : l'octet est gardé (dans l'ordre) avant de traiter l'erreur (P-11).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilRead(10, 1, "deadline", "waiting", 1, false), ilRead(260, 1, "eof", "kept", 2, false)),
			ilScript(p+"kept-prefix-at-turn", lane, "Fin après des octets sondés : la place est gardée jusqu'au tour, puis le décodeur livre les trames entières et jette la dernière, tronquée (P-11).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilRead(10, 1, "none", "waiting", 1, false), ilRead(260, 0, "eof", "kept", 1, false),
				ilEv("up", 300, "kept", none), ilEv("turn", 900, "delivering", none),
				ilEv("decode", 901, "gone", map[string]any{"deliver": 1, "truncated": true, "leave": true})),
			ilScript(p+"turn-needs-up", lane, "Le tour venu, le flux attend encore l'annonce du pair (30 s au plus après l'accusé), puis lit (P-10).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilEv("turn", 5, "waiting", none), ilEv("up", 120, "delivering", none)),
			ilScript(p+"up-wait-expires", lane, "Pas d'annonce dans les 30 s après l'accusé : le flux est réinitialisé et rend sa place (P-10).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilEv("turn", 5, "waiting", none), ilRead(30001, 0, "deadline", "gone", 0, true)),
			ilScript(p+"blocked-own-reset", lane, "Bloqué sur le budget : sa propre réinitialisation n'est vue qu'une fois de la place rendue (limite de ressources), puis il part et rend tout (P-12).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilEv("up", 2, "waiting", none), ilEv("turn", 2, "delivering", none),
				ilEv("budget", 3, "blocked", map[string]any{"room": false}), ilEv("reset_unseen", 4, "blocked", none),
				ilEv("budget", 5000, "delivering", map[string]any{"room": true}), ilRead(5000, 0, "reset", "gone", 0, true)),
			ilScript(p+"blocked-conn-lost", lane, "Bloqué sur le budget : la perte de sa connexion le libère aussitôt, réservations rendues (P-12).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilEv("up", 2, "waiting", none), ilEv("turn", 2, "delivering", none),
				ilEv("budget", 3, "blocked", map[string]any{"room": false}),
				ilEv("conn_lost", 10, "gone", map[string]any{"own_conn": true, "leave": true})),
			ilScript(p+"blocked-other-conn-lost", lane, "Bloqué sur le budget : la perte d'une autre connexion du même pair ne le libère pas (P-12).",
				ilEv("arrive", 0, "accepting", placed), ilEv("ack", 1, "waiting", map[string]any{"ok": true}),
				ilEv("up", 2, "waiting", none), ilEv("turn", 2, "delivering", none),
				ilEv("budget", 3, "blocked", map[string]any{"room": false}),
				ilEv("conn_lost", 10, "blocked", map[string]any{"own_conn": false})),
		)
	}
	vs = append(vs,
		ilProbe("inbound-probe-period", "Une sonde toutes les 250 ms au plus, un octet au plus par période : borne tenue (P-11).",
			every(250, 241), every(250, 241), "accept"),
		ilProbe("inbound-probe-late", "Un essai 251 ms après le précédent : la borne de 250 ms n'est plus tenue (P-11).",
			[]int{0, 250, 501}, []int{}, "reject"),
		ilProbe("inbound-probe-too-many", "Deux octets dans une même période : plus d'un octet par période, et au-delà de 256 octets gardés (P-11).",
			every(125, 300), every(125, 300), "reject"),
		ilWindow("inbound-quic-window-product", "Fenêtres du nœud, 128 Kio par flux et 1 Mio par connexion : (2 × 2 + 1) × 128 Kio = 640 Kio ≤ 3/4 × 1 Mio (P-13).", 128<<10, 1<<20, "accept"),
		ilWindow("inbound-quic-window-default", "Fenêtres initiales de quic-go (512 Kio par flux, 768 Kio par connexion) : les flux en attente des deux voies peuvent affamer la tête (P-13).", 512<<10, 768<<10, "reject"), //nolint:misspell // French: « initiales »
		ilWindow("inbound-quic-window-bound", "873 814 octets : juste assez ((2 × 2 + 1) × 128 Kio ≤ 3/4 × W) (P-13).", 128<<10, 873814, "accept"),
		ilWindow("inbound-quic-window-bound-minus-one", "873 813 octets : un octet de moins, refusé (P-13).", 128<<10, 873813, "reject"),
	)
	return vs
}
