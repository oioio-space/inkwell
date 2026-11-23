------------------------------ MODULE Supersede ------------------------------
(***************************************************************************)
(* Block HP1c (plan docs/plans/2026-11-23-hp1-reformalize.md §2, spec      *)
(* §8.7.4, rule P-15): a superseded connection proves that it lives.       *)
(*                                                                         *)
(* PeerUpAck (block PU1, frozen by gates/PU1c.toml) already has the        *)
(* half-open state: a side closes a connection (Close, Drop, a refusal)    *)
(* and the other learns it only later (LearnClose, the 30 s idle timeout); *)
(* until then its session may stay on the dead connection. This module     *)
(* extends it without editing it: every action of PeerUpAck runs unchanged *)
(* (Keep), and P-15 is added around it. When a connection registers and is *)
(* kept, every older connection still usable gets a proof (a ping). The    *)
(* peer answers while it still holds the connection (any byte: an echo, a  *)
(* refusal, a reset); with no answer within T_vie the proof is silent; a   *)
(* silent connection is closed once a newer connection carries an          *)
(* acknowledged sync stream (either direction), never the newest nor the   *)
(* only one; the proof is dropped when no newer connection is left.        *)
(*                                                                         *)
(* Time: HonestTiming (PeerUpAck's) also bounds the answer: TRUE, a peer   *)
(* that holds the connection answers within T_vie (T_vie is twice a bad    *)
(* round trip), so a proof is silent only on a dead connection; FALSE, a   *)
(* proof may be silent at any time (a slow relay). LearnFair: the idle     *)
(* timeout is assumed (weak fairness of LearnClose) or not (P-15 alone     *)
(* must take the session off a dead connection).                           *)
(*                                                                         *)
(* Rule operators, overridden by the mutants (mutants/SupersedeMutants):   *)
(*   StartsProof    P-15: a kept registration proves every older usable    *)
(*                  connection;                                            *)
(*   Closable       only a silent proof closes (an answer keeps it);       *)
(*   AckWaitOK      the close waits for an acknowledged stream on a newer  *)
(*                  connection still judged alive (the fallback exemption, *)
(*                  checked again when the close runs);                    *)
(*   SoleOK         never the newest nor the only connection.              *)
(***************************************************************************)
EXTENDS PeerUpAck

CONSTANTS
  LearnFair,     \* TRUE: the idle timeout is assumed (LearnClose weakly fair)
  Moves,         \* TRUE: PeerUpAck's move (relay to direct) is explored; FALSE: a session changes connection only by
                 \* reopening (StartOpen), as it does after a crash
  CrashBy        \* the nodes whose side of a connection may close at will (PeerUpAck's Close: a crash, a Drop of one link)

VARIABLES
  rank,   \* [Nodes -> [Conns -> Nat]]: registration order at each node (0: not kept)
  proof,  \* [Nodes -> [Conns -> [st, ack]]]: st none | pending | silent | answered; ack: a newer one was acked
  sv      \* ghost flags of the properties and of a witness (a proof closed a connection)

supv == <<rank, proof, sv>>
allv == <<vars, supv>>

PNone == [st |-> "none", ack |-> FALSE]
Active(p) == p.st \in {"pending", "silent"}
\* The peer still holds c: a ping on it gets a byte back.
PeerHolds(n, c) == c \in live /\ c \notin seen[Other(n)]
Newer(n, c2, c) == rank[n][c2] > rank[n][c]
NewerUsable(n, c) == \E c2 \in Conns : Usable(n, c2) /\ Newer(n, c2, c)
\* An acknowledged sync stream on c2, either direction: the node read its ACK, or wrote one for the peer's stream.
AckedOn(n, c2) == (out[n].st = "open" /\ out[n].c = c2) \/ \E p \in acc[n] : p.c = c2
\* A newer connection still judged alive carries an acknowledged sync stream: a silent one (its proof had no
\* answer) does not count, even acked earlier (block HP1c review 3b: with three connections, a dead acked middle one
\* would otherwise lift the oldest one's exemption while the newest awaits its first ACK).
NewerAcked(n, c) == \E c2 \in Conns : Usable(n, c2) /\ Newer(n, c2, c) /\ AckedOn(n, c2) /\ proof[n][c2].st # "silent"
\* The same without the exclusion of a silent connection (mutant BackwardAck: the reference decision before 3b).
AnyNewerAcked(n, c) == \E c2 \in Conns : Usable(n, c2) /\ Newer(n, c2, c) /\ AckedOn(n, c2)
TopRank(n) == LET r == {rank[n][d] : d \in Conns} IN CHOOSE x \in r : \A y \in r : y <= x

\* ---- Rule operators (the mutants override them) ----
StartsProof == TRUE
Closable(n, c) == proof[n][c].st = "silent"
AckWaitOK(n, c) == proof[n][c].ack /\ NewerAcked(n, c)
SoleOK(n, c) == NewerUsable(n, c)

TypeS ==
  /\ rank \in [Nodes -> [Conns -> 0..Cardinality(Conns)]]
  /\ proof \in [Nodes -> [Conns -> [st: {"none", "pending", "silent", "answered"}, ack: BOOLEAN]]]
  /\ sv \in [closed: BOOLEAN, live: BOOLEAN, unacked: BOOLEAN, sole: BOOLEAN]

InitS ==
  /\ Init
  /\ rank = [n \in Nodes |-> [c \in Conns |-> 0]]
  /\ proof = [n \in Nodes |-> [c \in Conns |-> PNone]]
  /\ sv = [closed |-> FALSE, live |-> FALSE, unacked |-> FALSE, sole |-> FALSE]

Keep(A) == A /\ UNCHANGED supv

\* A kept registration ranks the connection newest and proves every older usable one that holds no proof in
\* progress (at most one per connection; an answered one is proved again: a newer connection has come since).
RegisterS(n, c) ==
  /\ Register(n, c)
  /\ IF reg'[n][c] = "yes"
       THEN /\ rank' = [rank EXCEPT ![n][c] = TopRank(n) + 1]
            /\ proof' = [proof EXCEPT ![n] = [d \in Conns |->
                           IF StartsProof /\ d # c /\ Usable(n, d) /\ ~Active(proof[n][d])
                             THEN [st |-> "pending", ack |-> FALSE] ELSE proof[n][d]]]
       ELSE UNCHANGED <<rank, proof>>
  /\ UNCHANGED sv

\* A byte of the peer on c within T_vie: the connection lives; it stays open and nothing changes.
ProveAnswer(n, c) ==
  /\ proof[n][c].st = "pending" /\ Usable(n, c) /\ PeerHolds(n, c)
  /\ proof' = [proof EXCEPT ![n][c].st = "answered"]
  /\ UNCHANGED <<vars, rank, sv>>

\* T_vie passes with no byte. Under HonestTiming only if the peer no longer holds c.
ProveExpire(n, c) ==
  /\ proof[n][c].st = "pending" /\ Usable(n, c) /\ (HonestTiming => ~PeerHolds(n, c))
  /\ proof' = [proof EXCEPT ![n][c].st = "silent"]
  /\ UNCHANGED <<vars, rank, sv>>

\* A newer connection carries an acknowledged sync stream (the later of the two deadlines has come).
ProofAcked(n, c) ==
  /\ proof[n][c].st # "none" /\ ~proof[n][c].ack /\ NewerAcked(n, c)
  /\ proof' = [proof EXCEPT ![n][c].ack = TRUE]
  /\ UNCHANGED <<vars, rank, sv>>

\* The close: its streams end as streams lost with their connection (StreamLost), no ClosePeer, no event.
ProveClose(n, c) ==
  /\ Usable(n, c) /\ Closable(n, c) /\ AckWaitOK(n, c) /\ SoleOK(n, c)
  /\ CloseC(n, c)
  /\ proof' = [proof EXCEPT ![n][c] = PNone]
  /\ sv' = [sv EXCEPT !.closed = TRUE, !.live = @ \/ PeerHolds(n, c), !.unacked = @ \/ ~NewerAcked(n, c),
                      !.sole = @ \/ ~NewerUsable(n, c)]
  /\ UNCHANGED <<live, reg, admit, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves,
                 lateFire, rank>>

\* No newer connection is left (before the ACK, or after it): the proof is dropped and c stays open; a connection
\* that closed ends its proof too.
ProofEnd(n, c) ==
  /\ Active(proof[n][c]) /\ (~Usable(n, c) \/ ~NewerUsable(n, c))
  /\ proof' = [proof EXCEPT ![n][c] = PNone]
  /\ UNCHANGED <<vars, rank, sv>>

NextS ==
  \/ \E c \in Conns : Keep(Dial(c))
  \/ \E n \in Nodes, c \in Conns :
       \/ RegisterS(n, c) \/ Keep(LearnClose(n, c)) \/ (n \in CrashBy /\ Keep(Close(n, c)))
       \/ ProveAnswer(n, c) \/ ProveExpire(n, c) \/ ProofAcked(n, c) \/ ProveClose(n, c) \/ ProofEnd(n, c)
  \/ \E n \in Nodes :
       Keep(\/ Drop(n) \/ Readmit(n) \/ StartOpen(n) \/ (Moves /\ Move(n))
            \/ RecvProto(n) \/ RecvAck(n) \/ RecvReset(n) \/ Tick(n) \/ Timeout(n) \/ StreamLost(n) \/ Down(n)
            \/ Accept(n) \/ Swallow(n) \/ Decide(n))
  \/ Keep(DropStale)

\* PeerUpAck's fairness, on the same actions, the idle timeout as LearnFair says, and every step of a proof.
FairS ==
  /\ \A c \in Conns : WF_allv(Keep(Dial(c)))
  /\ \A n \in Nodes, c \in Conns :
       /\ WF_allv(RegisterS(n, c))
       /\ IF LearnFair THEN WF_allv(Keep(LearnClose(n, c))) ELSE TRUE
       /\ WF_allv(ProveAnswer(n, c)) /\ WF_allv(ProveExpire(n, c)) /\ WF_allv(ProofAcked(n, c))
       /\ WF_allv(ProveClose(n, c)) /\ WF_allv(ProofEnd(n, c))
  /\ \A n \in Nodes :
       /\ WF_allv(Keep(StartOpen(n))) /\ WF_allv(Keep(RecvProto(n))) /\ WF_allv(Keep(RecvAck(n)))
       /\ WF_allv(Keep(RecvReset(n))) /\ WF_allv(Keep(Timeout(n))) /\ WF_allv(Keep(StreamLost(n)))
       /\ WF_allv(Keep(Down(n))) /\ WF_allv(Keep(Accept(n))) /\ WF_allv(Keep(Decide(n)))
       /\ IF TickFair THEN WF_allv(Keep(Tick(n))) ELSE TRUE
  /\ WF_allv(Keep(DropStale))

SpecS == InitS /\ [][NextS]_allv /\ FairS

\* ---- Properties (PeerUpAck's S1-S5 are checked with them) ----
\* P15-1: a superseded connection is always under proof (or proved since the newer came): its life is bounded by
\* T_vie and the newer connection's ACK, not by the idle timeout.
SupersededBounded == \A n \in Nodes, c \in Conns : Usable(n, c) /\ NewerUsable(n, c) => proof[n][c].st # "none"
\* P15-2 (HonestTiming): a connection the peer still holds is never closed by a proof.
LiveKept == ~sv.live
\* P15-3: no proof closes a connection unless a newer one still judged alive carries an acknowledged sync stream
\* when it closes (the fallback).
FallbackKept == ~sv.unacked
\* P15-4: no proof closes the newest connection nor the only one.
NeverCloseSole == ~sv.sole
\* P15-5 (liveness, LearnFair = FALSE): a session on a dead connection under proof leaves it, unless no newer
\* connection is left; checked for the nodes whose peer may crash (CrashBy), the other side being the same.
DeadAt(n, c) == c \in live /\ c \notin seen[n] /\ c \in seen[Other(n)]
SessionLeavesDead ==
  \A n \in {m \in Nodes : Other(m) \in CrashBy}, c \in Conns :
    (out[n].st = "open" /\ out[n].c = c /\ DeadAt(n, c) /\ Active(proof[n][c]))
      ~> (out[n].st # "open" \/ out[n].c # c \/ ~NewerUsable(n, c))
=============================================================================
