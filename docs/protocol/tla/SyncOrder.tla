------------------------------ MODULE SyncOrder ------------------------------
(***************************************************************************)
(* Block PU1b (plan docs/plans/2026-11-16-pu1b-inbound-order.md, spec      *)
(* §8.7.3, §8.7.4): the responder's order of one peer's inbound sync       *)
(* streams. Each stream takes a place in the peer's order on arrival,      *)
(* before its ACK (P-6); at most Cap places per peer, shared by all its    *)
(* connections and generations (P-8): a further stream is refused for      *)
(* capacity (reset with the capacity code, nothing else) and the opener    *)
(* retries after a backoff, within its opening's budget, without touching  *)
(* its session. A stream delivers frames only once every stream ahead of   *)
(* it has left (waitFor = 0); a stream leaves when it ends: at once,       *)
(* wherever it stands, if it delivered nothing (refused, reset, or ended   *)
(* by its opener without a frame), and its successor then waits for the    *)
(* one ahead of it (the wake, P-9); a stream that delivered frames leaves  *)
(* once read to its end.                                                   *)
(*                                                                         *)
(* The opener is honest (one session stream cur, a move from the relayed  *)
(* connection 1 to the direct connection 2 that swaps after the ACK and    *)
(* closes the old stream for writing, the direct connection dropping, a    *)
(* reopen on connection 1: drain + replacement + reconnect may need a      *)
(* third place) or Byzantine (opens, writes, ends, resets and holds any    *)
(* stream at will). Stall lets a waiting stream give the order up          *)
(* (reorderWait, 60 s), used with a Byzantine head never read to its end.  *)
(*                                                                         *)
(* Time (P-5 with P-8): an opening has a clock (spent, ticks since its     *)
(* first open, retries included); a retry after a capacity refusal needs  *)
(* a tick first (the backoff); at spent = D the opening's budget is out:   *)
(* a session opening closes the peer (closedBy = "budget": every stream    *)
(* of the peer is reset), a move is abandoned. HonestTiming = TRUE is the  *)
(* assumption « the budget is longer than any honest drain »: time passes  *)
(* only while neither the responder nor a due retry can act.               *)
(*                                                                         *)
(* Rule operators, overridden by the mutants (mutants/SyncOrderMutants):   *)
(*   PlaceFree   P-8: Len(list) < Cap (Cap = 2);                           *)
(*   EarlyOK     P-9: a stream that delivered nothing leaves at once;      *)
(*   Ended       P-9: it ended: reset, or closed for writing with nothing  *)
(*               left to read;                                             *)
(*   Rewire      P-9: removal wakes the successor (it waits for the one    *)
(*               ahead of the removed stream);                             *)
(*   OnBusyKill  P-8: FALSE, a capacity refusal never closes the peer.     *)
(***************************************************************************)
EXTENDS Naturals, Sequences, FiniteSets

CONSTANTS
  Streams,      \* stream ids, in the opener's order, e.g. 1..3
  Cap,          \* places per peer (2)
  MaxFrames,    \* frames the opener writes over the run
  Byzantine,    \* TRUE: the opener is Byzantine (see the header)
  Stall,        \* TRUE: a waiting stream may give the order up (reorderWait)
  MaxDrops,     \* drops of the direct connection over the run (honest opener)
  D,            \* the opening's budget, in ticks (15 s)
  HonestTiming  \* TRUE: time passes only while nothing else can act (see the header)

VARIABLES
  rst,        \* each stream: idle, sent, placed, acked, gone; busy (refused for capacity), then wait (the
              \* opener's backoff), due (the backoff passed: retry)
  conn,       \* the stream's connection (1 relayed, 2 direct)
  list,       \* the peer's order: streams holding a place, in arrival order
  waitFor,    \* the stream a placed stream waits for (0: none, its turn has come)
  buf,        \* frames written and not yet read
  fin,        \* the opener closed the stream for writing
  rstd,       \* the stream was reset (by the opener or its connection's loss); its bytes are gone
  deliv,      \* the stream delivered at least one frame
  delivered,  \* the frames delivered to the application, in order
  alive,      \* alive[c]: connection c is up
  cur,        \* opener: the session's stream (0: none)
  pend,       \* opener: the stream it is opening (0: none)
  role,       \* opener: "session" or "move" for pend
  nextS,      \* opener: next stream id
  nextF,      \* opener: next frame number
  drops,      \* drops so far
  closedBy,   \* "none", or why the opener closed the peer (ClosePeer): "budget", "busy"
  spent,      \* ticks since the current opening's first open
  expired,    \* a stream gave the order up (Stall)
  removedWaiting, \* witness: a stream left while its successor was waiting for it
  emptyEof    \* witness: a waiting stream ended by its opener without a frame left the order

vars == <<rst, conn, list, waitFor, buf, fin, rstd, deliv, delivered, alive, cur, pend, role, nextS, nextF,
          drops, closedBy, spent, expired, removedWaiting, emptyEof>>
opener == <<cur, pend, role, nextS, nextF, drops, closedBy, spent>>

States == {"idle", "sent", "busy", "wait", "due", "placed", "acked", "gone"}
Refused == {"busy", "wait", "due"}
InList(s) == \E i \in 1..Len(list) : list[i] = s
Held == {s \in Streams : rst[s] \in {"placed", "acked"}}
Up == closedBy = "none"

\* Rule operators (P-8, P-9).
PlaceFree == Len(list) < Cap
EarlyOK(s) == TRUE
Ended(s) == rstd[s] \/ (fin[s] /\ buf[s] = <<>>)
Rewire(s) == waitFor[s]
OnBusyKill == FALSE

TypeOK ==
  /\ rst \in [Streams -> States]
  /\ conn \in [Streams -> {0, 1, 2}]
  /\ waitFor \in [Streams -> Streams \cup {0}]
  /\ cur \in Streams \cup {0} /\ pend \in Streams \cup {0}
  /\ closedBy \in {"none", "budget", "busy"} /\ spent \in 0..D

Init ==
  /\ rst = [s \in Streams |-> "idle"]
  /\ conn = [s \in Streams |-> 0]
  /\ list = <<>>
  /\ waitFor = [s \in Streams |-> 0]
  /\ buf = [s \in Streams |-> <<>>]
  /\ fin = [s \in Streams |-> FALSE]
  /\ rstd = [s \in Streams |-> FALSE]
  /\ deliv = [s \in Streams |-> FALSE]
  /\ delivered = <<>>
  /\ alive = [c \in {1, 2} |-> TRUE]
  /\ cur = 0 /\ pend = 0 /\ role = "session"
  /\ nextS = 1 /\ nextF = 1
  /\ drops = 0 /\ closedBy = "none" /\ spent = 0
  /\ expired = FALSE /\ removedWaiting = FALSE /\ emptyEof = FALSE

-----------------------------------------------------------------------------
\* The responder.

Arrive(s) ==
  /\ rst[s] = "sent"
  /\ IF rstd[s] THEN /\ rst' = [rst EXCEPT ![s] = "gone"]
                     /\ UNCHANGED <<list, waitFor>>
     ELSE IF PlaceFree
          THEN /\ rst' = [rst EXCEPT ![s] = "placed"]
               /\ waitFor' = [waitFor EXCEPT ![s] = IF list = <<>> THEN 0 ELSE list[Len(list)]]
               /\ list' = Append(list, s)
          ELSE /\ rst' = [rst EXCEPT ![s] = "busy"] \* P-8: reset with the capacity code
               /\ UNCHANGED <<list, waitFor>>
  /\ UNCHANGED <<conn, buf, fin, rstd, deliv, delivered, alive, expired, removedWaiting, emptyEof>>
  /\ UNCHANGED opener

Ack(s) ==
  /\ rst[s] = "placed" /\ ~rstd[s]
  /\ rst' = [rst EXCEPT ![s] = "acked"]
  /\ UNCHANGED <<conn, list, waitFor, buf, fin, rstd, deliv, delivered, alive, expired, removedWaiting, emptyEof>>
  /\ UNCHANGED opener

Read(s) ==
  /\ rst[s] = "acked" /\ waitFor[s] = 0 /\ buf[s] # <<>>
  /\ delivered' = Append(delivered, Head(buf[s]))
  /\ buf' = [buf EXCEPT ![s] = Tail(buf[s])]
  /\ deliv' = [deliv EXCEPT ![s] = TRUE]
  /\ UNCHANGED <<rst, conn, list, waitFor, fin, rstd, alive, expired, removedWaiting, emptyEof>>
  /\ UNCHANGED opener

\* The stream leaves the order: its successor now waits for the one ahead of it (the wake).
Remove(s) ==
  /\ list' = SelectSeq(list, LAMBDA t : t # s)
  /\ rst' = [rst EXCEPT ![s] = "gone"]
  /\ waitFor' = [t \in Streams |-> IF t = s THEN 0 ELSE IF waitFor[t] = s THEN Rewire(s) ELSE waitFor[t]]
  /\ removedWaiting' = (removedWaiting \/ \E t \in Streams : waitFor[t] = s /\ rst[t] = "acked")

\* Read to its end (end of stream, or a reset once its turn came).
Finish(s) ==
  /\ rst[s] = "acked" /\ waitFor[s] = 0 /\ buf[s] = <<>> /\ (fin[s] \/ rstd[s])
  /\ Remove(s)
  /\ UNCHANGED <<conn, buf, fin, rstd, deliv, delivered, alive, expired, emptyEof>>
  /\ UNCHANGED opener

\* Ended without delivering a frame (reset, or closed for writing with nothing to read): leaves at once, wherever it
\* stands.
EarlyEnd(s) ==
  /\ rst[s] \in {"placed", "acked"} /\ ~deliv[s] /\ Ended(s) /\ EarlyOK(s)
  /\ Remove(s)
  /\ emptyEof' = (emptyEof \/ (~rstd[s] /\ waitFor[s] # 0))
  /\ UNCHANGED <<conn, buf, fin, rstd, deliv, delivered, alive, expired>>
  /\ UNCHANGED opener

\* reorderWait: a stream that waited too long reads anyway (the order is given up).
Expire(s) ==
  /\ Stall /\ rst[s] = "acked" /\ waitFor[s] # 0
  /\ waitFor' = [waitFor EXCEPT ![s] = 0]
  /\ expired' = TRUE
  /\ UNCHANGED <<rst, conn, list, buf, fin, rstd, deliv, delivered, alive, removedWaiting, emptyEof>>
  /\ UNCHANGED opener

Responder(s) == Arrive(s) \/ Ack(s) \/ Read(s) \/ Finish(s) \/ EarlyEnd(s) \/ Expire(s)

-----------------------------------------------------------------------------
\* The honest opener.

Send(s, c) ==
  /\ rst' = [rst EXCEPT ![s] = "sent"]
  /\ conn' = [conn EXCEPT ![s] = c]

Unchanged == UNCHANGED <<list, waitFor, deliv, delivered, expired, removedWaiting, emptyEof>>

OpenSession ==
  /\ ~Byzantine /\ Up /\ cur = 0 /\ pend = 0 /\ nextS \in Streams
  /\ \E c \in {1, 2} : alive[c] /\ Send(nextS, c)
  /\ pend' = nextS /\ role' = "session" /\ nextS' = nextS + 1 /\ spent' = 0
  /\ Unchanged /\ UNCHANGED <<buf, fin, rstd, alive, cur, nextF, drops, closedBy>>

OpenMove ==
  /\ ~Byzantine /\ Up /\ cur # 0 /\ pend = 0 /\ conn[cur] = 1 /\ alive[2] /\ nextS \in Streams
  /\ Send(nextS, 2)
  /\ pend' = nextS /\ role' = "move" /\ nextS' = nextS + 1 /\ spent' = 0
  /\ Unchanged /\ UNCHANGED <<buf, fin, rstd, alive, cur, nextF, drops, closedBy>>

OnAck ==
  /\ ~Byzantine /\ Up /\ pend # 0 /\ rst[pend] = "acked" /\ ~rstd[pend]
  /\ IF role = "move" THEN /\ fin' = [fin EXCEPT ![cur] = TRUE] \* the old stream is closed for writing
                      ELSE UNCHANGED fin
  /\ cur' = pend /\ pend' = 0
  /\ Unchanged /\ UNCHANGED <<rst, conn, buf, rstd, alive, role, nextS, nextF, drops, closedBy, spent>>

\* A capacity refusal: wait the backoff, then retry the same opening; never ClosePeer (P-8).
OnBusy ==
  /\ ~Byzantine /\ Up /\ pend # 0 /\ rst[pend] = "busy"
  /\ IF OnBusyKill THEN /\ closedBy' = "busy"
                        /\ rstd' = [s \in Streams |-> rstd[s] \/ rst[s] # "idle"]
                        /\ buf' = [s \in Streams |-> <<>>]
                        /\ UNCHANGED rst
                   ELSE /\ rst' = [rst EXCEPT ![pend] = "wait"]
                        /\ UNCHANGED <<closedBy, rstd, buf>>
  /\ Unchanged /\ UNCHANGED <<conn, fin, alive, cur, pend, role, nextS, nextF, drops, spent>>

\* The backoff has passed (a tick since the refusal): the opening is sent again.
Retry ==
  /\ ~Byzantine /\ Up /\ pend # 0 /\ rst[pend] = "due"
  /\ rst' = [rst EXCEPT ![pend] = "sent"]
  /\ Unchanged /\ UNCHANGED <<conn, buf, fin, rstd, alive, cur, pend, role, nextS, nextF, drops, closedBy, spent>>

\* Time passes for the opening (the backoff, the budget). HonestTiming: only while nothing else can act.
Tick ==
  /\ ~Byzantine /\ Up /\ pend # 0 /\ spent < D
  /\ HonestTiming => ~ENABLED (Retry \/ \E s \in Streams : Responder(s))
  /\ spent' = spent + 1
  /\ rst' = IF rst[pend] = "wait" THEN [rst EXCEPT ![pend] = "due"] ELSE rst
  /\ Unchanged /\ UNCHANGED <<conn, buf, fin, rstd, alive, cur, pend, role, nextS, nextF, drops, closedBy>>

\* The budget is out (P-5): a session opening closes the peer (every stream of the peer reset), a move is abandoned.
OutOfBudget ==
  /\ ~Byzantine /\ Up /\ pend # 0 /\ spent = D /\ rst[pend] # "acked"
  /\ IF role = "session"
     THEN /\ closedBy' = "budget"
          /\ rstd' = [s \in Streams |-> rstd[s] \/ rst[s] # "idle"]
          /\ buf' = [s \in Streams |-> <<>>]
          /\ cur' = 0
     ELSE /\ rstd' = [rstd EXCEPT ![pend] = TRUE]
          /\ buf' = [buf EXCEPT ![pend] = <<>>]
          /\ UNCHANGED <<closedBy, cur>>
  /\ rst' = IF rst[pend] \in Refused THEN [rst EXCEPT ![pend] = "gone"] ELSE rst
  /\ pend' = 0
  /\ Unchanged /\ UNCHANGED <<conn, fin, alive, role, nextS, nextF, drops, spent>>

Write ==
  /\ ~Byzantine /\ Up /\ cur # 0 /\ nextF <= MaxFrames
  /\ buf' = [buf EXCEPT ![cur] = Append(buf[cur], nextF)]
  /\ nextF' = nextF + 1
  /\ Unchanged /\ UNCHANGED <<rst, conn, fin, rstd, alive, cur, pend, role, nextS, drops, closedBy, spent>>

\* The direct connection drops: its streams are reset (their bytes lost); the opener reopens on the relay.
Drop ==
  /\ ~Byzantine /\ Up /\ drops < MaxDrops /\ alive[2]
  /\ alive' = [alive EXCEPT ![2] = FALSE]
  /\ rstd' = [s \in Streams |-> rstd[s] \/ (conn[s] = 2 /\ rst[s] # "idle")]
  /\ buf' = [s \in Streams |-> IF conn[s] = 2 THEN <<>> ELSE buf[s]]
  /\ cur' = IF cur # 0 /\ conn[cur] = 2 THEN 0 ELSE cur
  /\ pend' = IF pend # 0 /\ conn[pend] = 2 THEN 0 ELSE pend
  /\ drops' = drops + 1
  /\ Unchanged /\ UNCHANGED <<rst, conn, fin, role, nextS, nextF, closedBy, spent>>

-----------------------------------------------------------------------------
\* The Byzantine opener.

BOpen ==
  /\ Byzantine /\ nextS \in Streams
  /\ Send(nextS, 1) /\ nextS' = nextS + 1
  /\ Unchanged /\ UNCHANGED <<buf, fin, rstd, alive, cur, pend, role, nextF, drops, closedBy, spent>>

BRetry(s) ==
  /\ Byzantine /\ rst[s] \in Refused
  /\ rst' = [rst EXCEPT ![s] = "sent"]
  /\ Unchanged /\ UNCHANGED <<conn, buf, fin, rstd, alive>> /\ UNCHANGED opener

BReset(s) ==
  /\ Byzantine /\ rst[s] \in {"sent", "placed", "acked"} /\ ~rstd[s]
  /\ rstd' = [rstd EXCEPT ![s] = TRUE]
  /\ buf' = [buf EXCEPT ![s] = <<>>]
  /\ Unchanged /\ UNCHANGED <<rst, conn, fin, alive>> /\ UNCHANGED opener

BWrite(s) ==
  /\ Byzantine /\ rst[s] \in {"placed", "acked"} /\ ~rstd[s] /\ ~fin[s] /\ nextF <= MaxFrames
  /\ buf' = [buf EXCEPT ![s] = Append(buf[s], nextF)]
  /\ nextF' = nextF + 1
  /\ Unchanged /\ UNCHANGED <<rst, conn, fin, rstd, alive, cur, pend, role, nextS, drops, closedBy, spent>>

BFin(s) ==
  /\ Byzantine /\ rst[s] \in {"placed", "acked"} /\ ~fin[s]
  /\ fin' = [fin EXCEPT ![s] = TRUE]
  /\ Unchanged /\ UNCHANGED <<rst, conn, buf, rstd, alive>> /\ UNCHANGED opener

-----------------------------------------------------------------------------

Next ==
  \/ \E s \in Streams : Responder(s)
  \/ OpenSession \/ OpenMove \/ OnAck \/ OnBusy \/ Retry \/ Tick \/ OutOfBudget \/ Write \/ Drop \/ BOpen
  \/ \E s \in Streams : BRetry(s) \/ BReset(s) \/ BWrite(s) \/ BFin(s)

Fairness ==
  /\ \A s \in Streams : WF_vars(Arrive(s)) /\ WF_vars(Ack(s)) /\ WF_vars(Read(s)) /\ WF_vars(Finish(s))
                        /\ WF_vars(EarlyEnd(s))
  /\ WF_vars(OpenSession) /\ WF_vars(OnAck) /\ WF_vars(OnBusy) /\ WF_vars(Retry) /\ WF_vars(Tick)
  /\ WF_vars(OutOfBudget)

Spec == Init /\ [][Next]_vars /\ Fairness

-----------------------------------------------------------------------------
\* Properties.

\* P-8: at most two places per peer (whatever the rule operator says).
Places == Len(list) <= 2 /\ Cardinality(Held) <= 2
\* P-6/P-9: the frames are delivered in the order the honest opener wrote them (no order given up).
Order == ~expired => \A i \in 1..Len(delivered) : \A j \in 1..Len(delivered) : i < j => delivered[i] < delivered[j]
\* P-8: a capacity refusal never closes the peer (only the budget's end does, P-5).
SessionKept == closedBy # "busy"
\* P-5 with P-8: an opening, retries included, never outlives its budget.
Budget == spent <= D
\* P-9: a stream that ended (reset, or closed for writing with nothing left) without delivering a frame does not
\* keep its place.
EmptyLeaves == \A s \in Streams :
                 (InList(s) /\ ~deliv[s] /\ (rstd[s] \/ (fin[s] /\ buf[s] = <<>>))) ~> ~InList(s)
\* Liveness (honest): every frame still in a stream is eventually delivered, and the session comes back.
AllDelivered == <>[](\A s \in Streams : buf[s] = <<>>)
SessionResumes == (cur = 0) ~> (cur # 0 \/ nextS \notin Streams)

\* Witnesses (invariants the sound model must violate).
NoBusy == \A s \in Streams : rst[s] \notin Refused
NoRemovalWhileWaiting == ~removedWaiting
NoEmptyEof == ~emptyEof
NoBudgetOut == closedBy # "budget"
=============================================================================
