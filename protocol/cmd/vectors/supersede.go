// SPDX-License-Identifier: MPL-2.0

package main

// A superseded connection proves that it lives (spec §8.7.4, block HP1c, rule P-15). Written from the text of the
// spec, not from any implementation. Status proposed, enforced from step 3 (protocol/syncack.Supersede is the
// reference decision since HP1c's step 1).

// spConn is one open connection to the peer: its registration time, whether it carries an acknowledged sync
// stream, and its proof (nil: none).
type spConn struct {
	id, opened int
	acked      bool
	proof      map[string]any
}

func spProof(started, forMs int, heard bool) map[string]any {
	return map[string]any{"started_ms": started, "for_ms": forMs, "heard": heard}
}

// spCase is rule P-15 at now over conns (decoder sync_supersede): the proofs to start, the connections to close, the
// proofs to drop, the next deadline (0: none) and each connection's proof state.
func spCase(id, desc string, now int, conns []spConn, prove, closes, drop []int, wake int, states map[string]string) vector {
	cs := make([]map[string]any, 0, len(conns))
	for _, c := range conns {
		m := map[string]any{"id": c.id, "opened_ms": c.opened, "acked": c.acked}
		if c.proof != nil {
			m["proof"] = c.proof
		}
		cs = append(cs, m)
	}
	list := func(xs []int) []int {
		if xs == nil {
			return []int{}
		}
		return xs
	}
	return vector{
		ID: id, Status: "proposed", Enforce: "step-3", Desc: desc, Decoder: "sync_supersede", Verdict: "accept",
		Scope: "transport", Context: map[string]any{"now_ms": now, "proof_window_ms": 5000, "conns": cs},
		Expect: map[string]any{"prove": list(prove), "close": list(closes), "drop": list(drop), "wake_ms": wake, "states": states},
	}
}

func supersedeVectors() []vector {
	return []vector{
		spCase("supersede_newer_proves_older", "Une connexion plus récente s'enregistre : l'ancienne doit prouver qu'elle vit (un ping de 32 octets).",
			1000, []spConn{{id: 1, opened: 0}, {id: 2, opened: 1000}}, []int{1}, nil, nil, 0,
			map[string]string{"1": "none", "2": "none"}),
		spCase("supersede_pending_waits", "Aucun octet depuis 4,999 s : la preuve attend encore, échéance à 5 s.",
			5999, []spConn{{id: 1, opened: 0, proof: spProof(1000, 1000, false)}, {id: 2, opened: 1000, acked: true}}, nil, nil, nil, 6000,
			map[string]string{"1": "pending", "2": "none"}),
		spCase("supersede_silent_closed_after_newer_ack", "T_vie écoulé sans octet et la nouvelle connexion porte un flux sync accusé : l'ancienne est fermée.",
			6000, []spConn{{id: 1, opened: 0, proof: spProof(1000, 1000, false)}, {id: 2, opened: 1000, acked: true}}, nil, []int{1}, nil, 0,
			map[string]string{"1": "silent", "2": "none"}),
		spCase("supersede_newer_unacked_keeps_older", "Une connexion directe plus récente attend l'accusé de son flux, le circuit plus ancien est muet : il reste ouvert, chemin de repli (exemption).",
			10000, []spConn{{id: 1, opened: 0, proof: spProof(1000, 1000, false)}, {id: 2, opened: 1000}}, nil, nil, nil, 0,
			map[string]string{"1": "silent", "2": "none"}),
		spCase("supersede_answered_keeps_older", "L'ancienne connexion a répondu dans T_vie : elle reste ouverte, rien ne change, même la nouvelle accusée.",
			20000, []spConn{{id: 1, opened: 0, proof: spProof(1000, 1000, true)}, {id: 2, opened: 1000, acked: true}}, nil, nil, nil, 0,
			map[string]string{"1": "answered", "2": "none"}),
		spCase("supersede_ack_on_older_does_not_count", "Seule l'ancienne connexion porte un flux accusé : la muette reste ouverte tant que la nouvelle n'en porte pas.",
			9000, []spConn{{id: 1, opened: 0, acked: true, proof: spProof(1000, 1000, false)}, {id: 2, opened: 1000}}, nil, nil, nil, 0,
			map[string]string{"1": "silent", "2": "none"}),
		spCase("supersede_sole_never_closed", "La nouvelle connexion s'est fermée : la seule connexion restante n'est jamais fermée ; sa preuve est abandonnée.",
			9000, []spConn{{id: 1, opened: 0, acked: true, proof: spProof(1000, 1000, false)}}, nil, nil, []int{1}, 0,
			map[string]string{"1": "silent"}),
		spCase("supersede_newest_proof_dropped", "La plus récente s'est fermée : la preuve de la suivante, devenue la plus récente, est abandonnée ; la plus ancienne, muette, est fermée (une plus récente est accusée).",
			9000, []spConn{{id: 1, opened: 0, proof: spProof(3000, 3000, false)}, {id: 2, opened: 1000, acked: true, proof: spProof(3000, 3000, false)}}, nil, []int{1}, []int{2}, 0,
			map[string]string{"1": "silent", "2": "silent"}),
		spCase("supersede_middle_acked", "Trois connexions : la plus ancienne, muette, est fermée dès qu'une plus récente, même intermédiaire, est accusée.",
			9000, []spConn{{id: 1, opened: 0, proof: spProof(2000, 2000, false)}, {id: 2, opened: 1000, acked: true, proof: spProof(2000, 2000, true)}, {id: 3, opened: 2000}}, nil, []int{1}, nil, 0,
			map[string]string{"1": "silent", "2": "answered", "3": "none"}),
		spCase("supersede_dead_middle_acked_keeps_older", "Trois connexions : l'intermédiaire a été accusée mais sa preuve est muette, elle ne compte pas ; la plus récente attend son accusé, la plus ancienne reste le chemin de repli.",
			9000, []spConn{{id: 1, opened: 0, proof: spProof(2000, 2000, false)}, {id: 2, opened: 1000, acked: true, proof: spProof(2000, 2000, false)}, {id: 3, opened: 2000}}, nil, nil, nil, 0,
			map[string]string{"1": "silent", "2": "silent", "3": "none"}),
		spCase("supersede_answered_reproved", "Une connexion prouvée pour une ancienne venue doit prouver de nouveau quand une plus récente arrive.",
			9000, []spConn{{id: 1, opened: 0, proof: spProof(1000, 1000, true)}, {id: 2, opened: 1000}, {id: 3, opened: 9000}}, []int{1, 2}, nil, nil, 0,
			map[string]string{"1": "answered", "2": "none", "3": "none"}),
		spCase("supersede_pending_not_restarted", "Au plus une preuve en cours par connexion : une nouvelle venue ne relance pas la preuve en cours.",
			3000, []spConn{{id: 1, opened: 0, proof: spProof(1000, 1000, false)}, {id: 2, opened: 1000}, {id: 3, opened: 3000}}, []int{2}, nil, nil, 6000,
			map[string]string{"1": "pending", "2": "none", "3": "none"}),
		spCase("supersede_loop", "Un pair qui ouvre des connexions en boucle : un seul ping par connexion plus ancienne, aucune fermeture sans preuve muette.",
			4000, []spConn{{id: 1, opened: 0}, {id: 2, opened: 1000}, {id: 3, opened: 2000}, {id: 4, opened: 3000}, {id: 5, opened: 4000, acked: true}}, []int{1, 2, 3, 4}, nil, nil, 0,
			map[string]string{"1": "none", "2": "none", "3": "none", "4": "none", "5": "none"}),
		spCase("supersede_same_time_by_id", "Deux connexions enregistrées au même instant : l'ordre est celui des identifiants, la plus grande est la plus récente.",
			0, []spConn{{id: 9, opened: 0}, {id: 4, opened: 0}}, []int{4}, nil, nil, 0,
			map[string]string{"4": "none", "9": "none"}),
		spCase("supersede_single", "Une seule connexion, sans preuve : rien à faire.",
			0, []spConn{{id: 1, opened: 0, acked: true}}, nil, nil, nil, 0,
			map[string]string{"1": "none"}),
	}
}
