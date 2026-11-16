--------------------------- MODULE PeerUpAckMutants ---------------------------
(* Block PU1, README "PeerUpAck": definition overrides of PeerUpAck's rule operators, and the        *)
(* witnesses (invariants the sound model must violate: the states the properties speak of are reached). *)
EXTENDS PeerUpAck

\* M1 (P-1 off, today's code): PeerUp when NewStream returns.
ReturnOpensMut == TRUE
\* M2 (P-2 off): the responder ACKs before its admission and retention checks.
AckAlways(m, c) == TRUE
\* M3 (P-3 off): the responder ACKs only once it has announced the opener itself: each waits for the other.
AckAfterUp(m, c) == admit[m] /\ Usable(m, c) /\ up[m]
\* M4 (P-6 off): the move swaps streams when NewStream returns, before the peer's ACK.
MoveAtReturnMut == TRUE
\* M5 (P-2's retention re-check off): ACK while admitted, whether or not the connection is still held.
AckAdmittedOnly(m, c) == admit[m]

\* M6 (P-5 off: the old guard): the deadline fires only once the stream is silent: a stuck handler holds it for ever.
TimeoutWhenSilent(n, st) == st.age = D /\ Silenced(n, st)
\* M7 (P-4 off): an ACK counts for any stream of its connection (acceptance by connection only).
AckSameConn(st, m) == st.c = m.c
\* M8 (P-5 off): NewStream and the ACK get a budget each (the clock restarts when NewStream returns).
ResetOnReturnMut == TRUE

\* Witnesses.
NotLazyLate == ~\E n \in Nodes : out[n].st = "await" /\ out[n].c # NoConn /\ reg[Other(n)][out[n].c] = "refused"
NotBothUp == ~(up["a"] /\ up["b"])
NoMove == moves = 0
NoLateFire == ~lateFire
=============================================================================
