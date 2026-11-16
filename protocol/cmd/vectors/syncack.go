// SPDX-License-Identifier: MPL-2.0

package main

// The sync acceptance ACK corpus (spec §8.7, block PU1). The ACK is one byte, 0x01 (§8.7.2); the
// bytes and decisions are written here from the text of §8.7, not from any implementation.
// Status proposed with no enforce at step 1 of PU1; step 2 sets enforce once protocol/syncack exists.

const syncAck = "01"

// saRead is an opener's reading of its outbound stream (§8.7.2): the bytes read before the stream
// ended or the reading stopped; result is the verdict on the first byte, after on any later byte.
func saRead(id, desc, input, result, after string) vector {
	verdict := "reject"
	if result == "accepted" && after == "none" {
		verdict = "accept"
	}
	return vector{
		ID: id, Status: "proposed", Desc: desc, Decoder: "sync_ack", Input: input, Verdict: verdict,
		Scope: "transport", Expect: map[string]any{"result": result, "after": after},
	}
}

// saResponder is the responder's decision (§8.7.3) for one state, checked in the rule's order.
func saResponder(id, desc string, closed, admitted, connClosed bool, action string) vector {
	verdict := "reject"
	if action == "ack" {
		verdict = "accept"
	}
	return vector{
		ID: id, Status: "proposed", Desc: desc, Decoder: "sync_ack_responder", Verdict: verdict, Scope: "transport",
		Context: map[string]any{"node_closed": closed, "admitted": admitted, "conn_closed": connClosed},
		Expect:  map[string]any{"action": action},
	}
}

// saOpener is an event script of the opener's machine (§8.7.4, §8.7.5). The deadline is 15 000 ms
// after "open"; an input at or past it finds the deadline already fired.
func saOpener(id, desc, role string, up bool, events ...event) vector {
	return vector{
		ID: id, Status: "proposed", Desc: desc, Decoder: "sync_ack_opener", Verdict: "accept", Scope: "transport",
		Context: map[string]any{"role": role, "peer_announced": up, "deadline_ms": 15000}, Events: events,
	}
}

func saEv(op string, at int, expect string) event {
	return event{"op": op, "at_ms": at, "expect": expect}
}

func saByte(at int, hex, expect string) event {
	return event{"op": "byte", "at_ms": at, "byte_hex": hex, "expect": expect}
}

func syncAckVectors() []vector {
	vs := []vector{
		saRead("sync-ack-ok", "L'accusé 0x01, rien après : accepté.", syncAck, "accepted", "none"),
		saRead("sync-ack-eof", "Fin du flux avant tout octet : refus (P-4).", "", "refused", "none"),
		saRead("sync-ack-zero", "0x00 n'est pas l'accusé : violation.", "00", "violation", "none"),
		saRead("sync-ack-two", "0x02 : violation (aucune valeur ignorable).", "02", "violation", "none"),
		saRead("sync-ack-7f", "0x7F : violation.", "7f", "violation", "none"),
		saRead("sync-ack-ff", "0xFF : violation.", "ff", "violation", "none"),
		saRead("sync-ack-twice", "Un second 0x01 après l'accusé : violation (le répondeur n'écrit qu'un octet).", "0101", "accepted", "violation"),
		saRead("sync-ack-trailing", "Accusé puis donnée : violation.", "0100", "accepted", "violation"),
		saRead("sync-ack-frame-first", "Une trame sync (longueur, données) à la place de l'accusé : violation.", "0300616263", "violation", "none"),

		saResponder("sync-ack-responder-ok", "Nœud ouvert, pair admis, connexion gardée : accusé.", false, true, false, "ack"),
		saResponder("sync-ack-responder-closed", "Nœud fermé : réinitialisation sans octet (étape 1).", true, true, false, "reset"),
		saResponder("sync-ack-responder-not-admitted", "Pair non admis ou tenu : réinitialisation sans octet (étape 2).", false, false, false, "reset"),
		saResponder("sync-ack-responder-conn-closed", "Connexion fermée ou en fermeture (H-19, Drop) : réinitialisation sans octet (étape 3).", false, true, true, "reset"),
		saResponder("sync-ack-responder-all-bad", "Tout faux : réinitialisation.", true, false, true, "reset"),
		saResponder("sync-ack-responder-closed-conn-not-admitted", "Pair non admis sur une connexion fermée : réinitialisation.", false, false, true, "reset"),

		saOpener("sync-ack-opener-up", "Session, retour de l'ouverture : rien ; accusé : PeerUp (P-1).", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saByte(100, syncAck, "peer_up")),
		saOpener("sync-ack-opener-negotiated", "Ouverture négociée : le retour arrive après un aller-retour, PeerUp attend toujours l'accusé.", "session", false,
			saEv("open", 0, "none"), saEv("return", 100, "none"), saByte(100, syncAck, "peer_up")),
		saOpener("sync-ack-opener-eof", "Fin du flux sans accusé : ClosePeer, aucun PeerUp (H-19 : connexion fermée par le pair).", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("eof", 40, "close_peer")),
		saOpener("sync-ack-opener-reset", "Réinitialisation sans accusé (pair qui ne nous admet pas) : ClosePeer, aucun PeerUp.", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("reset", 100, "close_peer")),
		saOpener("sync-ack-opener-conn-lost", "Connexion perdue avant l'accusé : aucun événement.", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("conn_closed", 30, "lost")),
		saOpener("sync-ack-opener-bad-byte", "Octet faux : violation, ClosePeer, aucun PeerUp.", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saByte(100, "02", "close_peer")),
		saOpener("sync-ack-opener-byte-after", "Accusé puis un octet : PeerUp puis violation, ClosePeer.", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saByte(100, syncAck, "peer_up"), saByte(200, syncAck, "close_peer")),
		saOpener("sync-ack-opener-just-in-time", "Accusé à 14 999 ms : PeerUp.", "session", false,
			saEv("open", 0, "none"), saEv("return", 10, "none"), saByte(14999, syncAck, "peer_up")),
		saOpener("sync-ack-opener-deadline", "Accusé à 15 000 ms : le délai est déjà écoulé, ClosePeer (P-5).", "session", false,
			saEv("open", 0, "none"), saEv("return", 10, "none"), saByte(15000, syncAck, "close_peer")),
		saOpener("sync-ack-opener-slow-negotiation", "Négociation lente (retour à 10 s) puis accusé à 15 s : un seul délai pour l'ouverture et l'accusé, ClosePeer (P-5).", "session", false,
			saEv("open", 0, "none"), saEv("return", 10000, "none"), saByte(15000, syncAck, "close_peer")),
		saOpener("sync-ack-opener-slow-negotiation-in-time", "Négociation lente (retour à 10 s), accusé à 14 999 ms : PeerUp.", "session", false,
			saEv("open", 0, "none"), saEv("return", 10000, "none"), saByte(14999, syncAck, "peer_up")),
		saOpener("sync-ack-opener-deadline-before-return", "Ouverture sans retour pendant 15 s : ClosePeer.", "session", false,
			saEv("open", 0, "none"), saEv("deadline", 15000, "close_peer")),
		saOpener("sync-ack-opener-silent", "Pair muet après le retour : ClosePeer à 15 s, aucun PeerUp.", "session", false,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("deadline", 15000, "close_peer")),
		saOpener("sync-ack-opener-reopen", "Flux rouvert alors que le pair est annoncé : l'accusé ne produit aucun nouvel événement (P-6).", "session", true,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saByte(100, syncAck, "none")),
		saOpener("sync-ack-opener-reopen-refused", "Flux rouvert refusé : ClosePeer (PeerDown suivra la fermeture).", "session", true,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("reset", 100, "close_peer")),
		saOpener("sync-ack-opener-move", "Passage au direct : le remplacement attend l'accusé du nouveau flux (P-6).", "move", true,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saByte(60, syncAck, "swap")),
		saOpener("sync-ack-opener-move-eof", "Passage refusé (connexion directe fermée par le pair) : abandon, session inchangée, aucun événement.", "move", true,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("eof", 30, "abandon")),
		saOpener("sync-ack-opener-move-bad-byte", "Passage, octet faux : abandon.", "move", true,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saByte(60, "00", "abandon")),
		saOpener("sync-ack-opener-move-deadline", "Passage sans accusé en 15 s : abandon.", "move", true,
			saEv("open", 0, "none"), saEv("return", 0, "none"), saEv("deadline", 15000, "abandon")),
	}
	return vs
}
