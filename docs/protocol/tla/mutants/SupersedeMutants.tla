--------------------------- MODULE SupersedeMutants ---------------------------
(* Block HP1c, README "Supersede": definition overrides of Supersede's rule operators (rule P-15), and the     *)
(* witnesses (invariants the sound model must violate: the states the properties speak of are reached).       *)
EXTENDS Supersede

\* MS1 (P-15 off): a newer connection proves nothing; a dead older one lasts until the idle timeout.
NoProofMut == FALSE
\* MS2: the proof's timer closes the older connection whatever the answer.
CloseOnTimerMut(n, c) == proof[n][c].st \in {"silent", "answered"}
\* MS3: the close does not wait for an acknowledged stream on a newer connection (no fallback while it awaits).
IgnoreAckWaitMut(n, c) == TRUE
\* MS4: a silent connection is closed even when it is the newest or the only one left. Since block HP1c2 (review 3b)
\* the close re-checks that a newer connection, usable and still judged alive, is acknowledged (AckWaitOK): that
\* implies a newer usable one, so FallbackKept implies NeverCloseSole and dropping SoleOK alone is equivalent. MS4 now
\* mirrors the reference decision's mutant MP2 (the newest is judged like the older ones): SoleOK is dropped and the
\* connection's own acknowledged stream counts, as the newest's own ACK does in syncack.Supersede under MP2.
ProveSoleMut(n, c) == TRUE
OwnAckMut(n, c) == AckedOn(n, c) \/ NewerAcked(n, c)
\* MS5 (block HP1c review 3b): the exemption is lifted by any newer acknowledged connection, a silent one included
\* (the ACK of a dead middle connection lets the oldest close while the newest awaits its first ACK).
BackwardAckMut(n, c) == proof[n][c].ack /\ AnyNewerAcked(n, c)

\* Witnesses.
NoProofClose == ~sv.closed
NoAnswer == \A n \in Nodes, c \in Conns : proof[n][c].st # "answered"
\* A silent proof waits while the newer connection's opening awaits its ACK (the fallback exemption's case).
NoFallbackWait ==
  ~\E n \in Nodes, c, c2 \in Conns :
     /\ proof[n][c].st = "silent" /\ ~proof[n][c].ack /\ Usable(n, c) /\ Usable(n, c2) /\ Newer(n, c2, c)
     /\ \/ out[n].st \in Opening /\ out[n].c = c2
        \/ nxt[n].st \in Opening /\ nxt[n].c = c2
=============================================================================
