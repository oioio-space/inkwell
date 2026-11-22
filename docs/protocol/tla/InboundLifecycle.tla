--------------------------- MODULE InboundLifecycle ---------------------------
(***************************************************************************)
(* Block PU1c (plan docs/plans/2026-11-17-pu1c-inbound-lifecycle.md, spec  *)
(* §8.1, §8.7.3, §16.1): the life of one peer's inbound streams on BOTH    *)
(* ordered lanes at once (sync and bulk), sharing one QUIC connection's    *)
(* receive credit. Each stream has one reader for its whole life:          *)
(*                                                                         *)
(*   arrived -> refused | accepting (the ACK, both lanes) -> waiting       *)
(*   waiting -> kept (ended with a probed prefix) | delivering | gone      *)
(*   delivering <-> blocked (the lane's receive budget is full)            *)
(*                                                                         *)
(* Waiting, the reader only probes (one unit at a time, at most ProbeMax   *)
(* units kept, never charged to the budget): a pending probe sees an end   *)
(* with nothing unread at once; a reset wins over buffered data (both      *)
(* muxers), so the next probe sees it; an end behind unread bytes (FIN, or *)
(* RESET_STREAM_AT's reliable bytes) is seen at the stream's turn. Any end *)
(* with a non-empty probed prefix keeps the place (kept) and goes to the   *)
(* decoder at the turn: whole frames before a truncated last one are       *)
(* delivered. A blocked reader is freed by room (its consumer), or by the  *)
(* loss of its own connection (or the node's close), never by its own      *)
(* reset (the explicit resource limit of §8.7.3).                          *)
(*                                                                         *)
(* Credit (rule P-13): a stream holds at most Win units of the             *)
(* connection's credit, the connection ConnWindow units; a unit the reader *)
(* reads gives its credit back. The streams not read (accepting, waiting,  *)
(* kept, blocked) of both lanes are at most Cap * 2 - 1 besides a head, so *)
(* ConnWindow = Cap * 2 * Win leaves every head Win units however full     *)
(* they are (the window update lag theta is folded into the units: the     *)
(* spec's inequality divides by 1 - theta).                                *)
(*                                                                         *)
(* Openers: honest per lane in HonestLanes (one session stream; a reopen  *)
(* swaps to the new stream after its ACK and closes the old one for       *)
(* writing; the late-handler case: streams arrive before PeerUp);         *)
(* Byzantine per lane in Byz (opens, writes whole or partial frames, FIN,  *)
(* reset, RESET_STREAM_AT, holds); Greedy streams are filled to their      *)
(* window by any opener (the credit-exhaustion case). Consumers drain a    *)
(* lane's budget fairly only for lanes in FairConsumers.                   *)
(*                                                                         *)
(* Rule operators, overridden by the mutants (mutants/InboundLifecycle-    *)
(* Mutants.tla):                                                           *)
(*   PlaceAtArrival(l)  P-6/P-14: the place is taken on arrival;           *)
(*   EarlyOK(s)         P-9: an ended stream with nothing delivered leaves *)
(*                      at once, wherever it stands;                       *)
(*   ProbeOK(s)         P-11: the probe goes on after a unit was taken;    *)
(*   WaitCharges        P-12: FALSE, a waiting stream never takes budget;  *)
(*   KeepPrefix(s)      P-11: any non-empty prefix is kept to its turn;    *)
(*   CancelOnLoss       P-12: a blocked reader is freed by its connection's*)
(*                      loss;                                              *)
(*   ConnWindow         P-13: Cap * 2 * Win.                               *)
(***************************************************************************)
EXTENDS Naturals, Sequences, FiniteSets

CONSTANTS
  Sync, Bulk,     \* the stream ids of each lane (disjoint), in the honest opener's order
  Cap,            \* places per peer and per lane (2)
  Win,            \* a stream's receive window, in units
  Budget,         \* each lane's receive budget, in frames (unconsumed deliveries)
  ProbeMax,       \* units a waiting stream may take by probing
  MaxFrames,      \* frames each lane's opener writes over the run (honest and Byzantine)
  MaxMoves,       \* honest reopens per lane
  HonestLanes,    \* lanes whose opener is honest
  Byz,            \* lanes whose opener is Byzantine
  Greedy,         \* streams any opener fills to their window (junk frames)
  FairConsumers,  \* lanes whose consumer drains the budget fairly
  Stall,          \* TRUE: a waiting stream may give the order up (reorderWait)
  MaxDrops        \* losses of the connection over the run

Lanes == {"sync", "bulk"}
S == Sync \cup Bulk
Lane(s) == IF s \in Sync THEN "sync" ELSE "bulk"
LaneIds(l) == IF l = "sync" THEN Sync ELSE Bulk

VARIABLES
  st,        \* each stream's state (see States)
  q,         \* units received and not read (they hold credit): << [f |-> n, w |-> whole?] >>
  pre,       \* units taken by the probe while waiting (in memory, given to the decoder at the turn)
  fin,       \* the opener closed the stream for writing
  rstd,      \* the stream was reset (abrupt: its unread units are dropped), or its connection lost
  rat,       \* RESET_STREAM_AT: reset after the units already sent (they are delivered first)
  part,      \* the opener wrote a partial (truncated) last frame: nothing whole may follow
  ackd,      \* the ACK was written
  ch,        \* budget units charged by the stream for frames not handed to the consumer (WaitCharges only)
  got,       \* whole frames delivered from the stream
  wholeW,    \* whole frames written on the stream
  list,      \* list[l]: the streams of lane l holding a place, in arrival order
  waitFor,   \* the stream a placed stream waits for (0: none)
  dlv,       \* dlv[l]: frame numbers delivered to lane l's consumer, in order
  used,      \* used[l]: lane l's budget held by unconsumed deliveries and charges
  up,        \* the peer is announced (PeerUp)
  alive,     \* the connection is up
  expired,   \* a stream gave the order up (Stall)
  cur,       \* honest opener: cur[l], the session stream of lane l (0: none)
  pend,      \* honest opener: pend[l], the stream opened and not yet ACKed (0: none)
  nf,        \* nf[l]: next frame number of lane l
  moves,     \* moves[l]: honest reopens so far
  drops,     \* connection losses so far
  keptSeen, blockedSeen, truncSeen, exhaustedFirst  \* witnesses

vars == <<st, q, pre, fin, rstd, rat, part, ackd, ch, got, wholeW, list, waitFor, dlv, used, up, alive,
          expired, cur, pend, nf, moves, drops, keptSeen, blockedSeen, truncSeen, exhaustedFirst>>
wit == <<keptSeen, blockedSeen, truncSeen, exhaustedFirst>>
opener == <<cur, pend, moves>>

States == {"idle", "arrived", "refused", "accepting", "waiting", "kept", "delivering", "blocked", "gone"}
Open == {"arrived", "accepting", "waiting", "kept", "delivering", "blocked"}
NotRead == {"arrived", "accepting", "waiting", "kept"}
InList(s) == \E i \in 1..Len(list[Lane(s)]) : list[Lane(s)][i] = s
Junk == [f |-> 0, w |-> TRUE]

\* The connection credit a stream holds: its unread units, plus, until its turn, the units its probe took (read, but
\* their credit is not given back: no window update below a quarter of a window).
Held(x) == Len(q[x]) + (IF st[x] \in {"accepting", "waiting", "kept"} THEN Len(pre[x]) ELSE 0)
RECURSIVE SumQ(_)
SumQ(T) == IF T = {} THEN 0 ELSE LET x == CHOOSE x \in T : TRUE IN Held(x) + SumQ(T \ {x})
RECURSIVE SumChSet(_)
SumChSet(T) == IF T = {} THEN 0 ELSE LET x == CHOOSE x \in T : TRUE IN ch[x] + SumChSet(T \ {x})
SumCh(l) == SumChSet(LaneIds(l))

\* Rule operators.
PlaceAtArrival(l) == TRUE
EarlyOK(s) == TRUE
ProbeOK(s) == TRUE
WaitCharges == FALSE
KeepPrefix(s) == TRUE
CancelOnLoss == TRUE
ConnWindow == Cap * 2 * Win

\* Credit: the stream's window (the probe's units are read but give no stream credit back: no window update below a
\* quarter of the window) and the connection's.
StreamRoom(s) == Len(q[s]) + (IF st[s] \in {"delivering", "blocked"} THEN 0 ELSE Len(pre[s])) < Win
ConnRoom == SumQ(S) < ConnWindow
Room(s) == StreamRoom(s) /\ ConnRoom
Writable(s) == st[s] \in Open \cup {"refused"} /\ ~fin[s] /\ ~rstd[s] /\ ~rat[s] /\ ~part[s] /\ alive

TypeOK ==
  /\ st \in [S -> States]
  /\ waitFor \in [S -> S \cup {0}]
  /\ used \in [Lanes -> 0..Budget]
  /\ \A s \in S : Len(pre[s]) <= ProbeMax /\ Len(q[s]) <= Win

Init ==
  /\ st = [s \in S |-> "idle"]
  /\ q = [s \in S |-> <<>>] /\ pre = [s \in S |-> <<>>]
  /\ fin = [s \in S |-> FALSE] /\ rstd = [s \in S |-> FALSE] /\ rat = [s \in S |-> FALSE]
  /\ part = [s \in S |-> FALSE] /\ ackd = [s \in S |-> FALSE] /\ ch = [s \in S |-> 0]
  /\ got = [s \in S |-> 0] /\ wholeW = [s \in S |-> 0]
  /\ list = [l \in Lanes |-> <<>>] /\ waitFor = [s \in S |-> 0]
  /\ dlv = [l \in Lanes |-> <<>>] /\ used = [l \in Lanes |-> 0]
  /\ up = FALSE /\ alive = TRUE /\ expired = FALSE
  /\ cur = [l \in Lanes |-> 0] /\ pend = [l \in Lanes |-> 0]
  /\ nf = [l \in Lanes |-> 1] /\ moves = [l \in Lanes |-> 0] /\ drops = 0
  /\ keptSeen = FALSE /\ blockedSeen = FALSE /\ truncSeen = FALSE /\ exhaustedFirst = FALSE

-----------------------------------------------------------------------------
\* The reader of each stream.

\* Both lanes write the acceptance ACK (P-14: bulk/2.0.0 too; without it two handlers may start in either order and
\* an honest reopen is not ordered: the first run of this model found dlv[bulk] = <<2, 1>>).
FirstState(l) == "accepting"

Place(s) ==
  /\ waitFor' = [waitFor EXCEPT ![s] = IF list[Lane(s)] = <<>> THEN 0 ELSE list[Lane(s)][Len(list[Lane(s)])]]
  /\ list' = [list EXCEPT ![Lane(s)] = Append(@, s)]

\* The stream leaves its lane's order (P-9): its successor now waits for the one ahead of it (the wake). The reader
\* resets it: its unread units are dropped (credit returned), its charges released.
Remove(s) ==
  /\ list' = [list EXCEPT ![Lane(s)] = SelectSeq(@, LAMBDA t : t # s)]
  /\ waitFor' = [t \in S |-> IF t = s THEN 0 ELSE IF waitFor[t] = s THEN waitFor[s] ELSE waitFor[t]]
  /\ st' = [st EXCEPT ![s] = "gone"]
  /\ q' = [q EXCEPT ![s] = <<>>] /\ pre' = [pre EXCEPT ![s] = <<>>]
  /\ ch' = [ch EXCEPT ![s] = 0]
  /\ used' = [used EXCEPT ![Lane(s)] = @ - ch[s]]

Keep == <<fin, rstd, rat, part, ackd, got, wholeW, dlv, up, alive, expired, nf, drops>>

Arrive(s) ==
  /\ st[s] = "arrived"
  /\ IF rstd[s] THEN /\ st' = [st EXCEPT ![s] = "gone"] /\ q' = [q EXCEPT ![s] = <<>>]
                     /\ UNCHANGED <<list, waitFor>>
     ELSE IF ~PlaceAtArrival(Lane(s))
          THEN /\ st' = [st EXCEPT ![s] = FirstState(Lane(s))] \* placed later (the mutants' late place)
               /\ UNCHANGED <<q, list, waitFor>>
     ELSE IF Len(list[Lane(s)]) < Cap
          THEN /\ st' = [st EXCEPT ![s] = FirstState(Lane(s))]
               /\ Place(s) /\ UNCHANGED q
          ELSE /\ st' = [st EXCEPT ![s] = "refused"] \* reset with the capacity code; its units are released by Release
               /\ UNCHANGED <<q, list, waitFor>>
  /\ UNCHANGED <<pre, ch, used>> /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED wit

\* A refused stream's received units hold credit until its reset is processed (the opener's RESET_STREAM answers
\* our STOP_SENDING; quic-go then abandons them): a step of its own, not part of the refusal.
Release(s) ==
  /\ st[s] = "refused"
  /\ st' = [st EXCEPT ![s] = "gone"] /\ q' = [q EXCEPT ![s] = <<>>]
  /\ rstd' = [rstd EXCEPT ![s] = TRUE] \* the refusal is a reset (code 0x1002): its units are dropped, not delivered
  /\ UNCHANGED <<pre, ch, used, list, waitFor, fin, rat, part, ackd, got, wholeW, dlv, up, alive, expired, nf, drops>>
  /\ UNCHANGED opener /\ UNCHANGED wit

\* The mutants' late place: taken once the peer is up (the old up-wait).
LatePlace(s) ==
  /\ ~PlaceAtArrival(Lane(s)) /\ st[s] \in {"accepting", "waiting", "kept"} /\ ~InList(s) /\ up /\ ~rstd[s]
  /\ IF Len(list[Lane(s)]) < Cap
     THEN /\ Place(s) /\ UNCHANGED <<st, q, pre, ch, used>>
     ELSE /\ Remove(s)
  /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED wit

\* The ACK (both lanes); its write fails on a reset.
Ack(s) ==
  /\ st[s] = "accepting"
  /\ IF rstd[s] /\ EarlyOK(s)
     THEN Remove(s) /\ UNCHANGED ackd
     ELSE /\ ~rstd[s]
          /\ st' = [st EXCEPT ![s] = "waiting"] /\ ackd' = [ackd EXCEPT ![s] = TRUE]
          /\ UNCHANGED <<q, pre, ch, used, list, waitFor>>
  /\ UNCHANGED <<fin, rstd, rat, part, got, wholeW, dlv, up, alive, expired, nf, drops>>
  /\ UNCHANGED opener /\ UNCHANGED wit

EndedEmpty(s) == (fin[s] \/ rat[s]) /\ q[s] = <<>>

\* Waiting, the reader probes: a reset is seen (it wins over unread units); else a unit is taken (at most ProbeMax);
\* else an end with nothing unread is seen: an empty prefix leaves at once, a non-empty one is kept to its turn.
Probe(s) ==
  /\ st[s] = "waiting" /\ ProbeOK(s)
  /\ \/ /\ rstd[s] /\ EarlyOK(s)
        /\ Remove(s) /\ UNCHANGED <<keptSeen>>
     \/ /\ ~rstd[s] /\ q[s] # <<>> /\ Len(pre[s]) < ProbeMax
        /\ ~(WaitCharges /\ Head(q[s]).w /\ used[Lane(s)] = Budget)
        /\ pre' = [pre EXCEPT ![s] = Append(@, Head(q[s]))]
        /\ q' = [q EXCEPT ![s] = Tail(@)]
        /\ IF WaitCharges /\ Head(q[s]).w \* the old design: a frame read while waiting is charged to the budget
           THEN /\ ch' = [ch EXCEPT ![s] = @ + 1] /\ used' = [used EXCEPT ![Lane(s)] = @ + 1]
           ELSE UNCHANGED <<ch, used>>
        /\ UNCHANGED <<st, list, waitFor, keptSeen>>
     \/ /\ ~rstd[s] /\ EndedEmpty(s) /\ pre[s] = <<>> /\ EarlyOK(s)
        /\ Remove(s) /\ UNCHANGED <<keptSeen>>
     \/ /\ ~rstd[s] /\ EndedEmpty(s) /\ pre[s] # <<>>
        /\ IF KeepPrefix(s)
           THEN /\ st' = [st EXCEPT ![s] = "kept"] /\ keptSeen' = TRUE
                /\ UNCHANGED <<q, pre, ch, used, list, waitFor>>
           ELSE Remove(s) /\ UNCHANGED keptSeen
  /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED <<blockedSeen, truncSeen, exhaustedFirst>>

\* The turn: nobody ahead holds a place (or the order was given up), and the peer is up.
Turn(s) ==
  /\ st[s] \in {"waiting", "kept"} /\ InList(s) /\ waitFor[s] = 0 /\ up
  /\ st' = [st EXCEPT ![s] = "delivering"]
  /\ used' = [used EXCEPT ![Lane(s)] = @ - ch[s]] /\ ch' = [ch EXCEPT ![s] = 0] \* charged frames go to the decoder
  /\ UNCHANGED <<q, pre, list, waitFor>> /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED wit

Expire(s) ==
  /\ Stall /\ st[s] \in {"waiting", "kept"} /\ waitFor[s] # 0
  /\ waitFor' = [waitFor EXCEPT ![s] = 0] /\ expired' = TRUE
  /\ UNCHANGED <<st, q, pre, fin, rstd, rat, part, ackd, ch, got, wholeW, list, dlv, used, up, alive, nf, drops>>
  /\ UNCHANGED opener /\ UNCHANGED wit

Src(s) == pre[s] \o q[s]
Ends(s) == fin[s] \/ rat[s] \/ rstd[s]

\* Delivering: the decoder reads the prefix, then the stream; each whole frame is charged to the lane's budget as it
\* is handed over; a full budget blocks; a truncated last frame ends the stream (nothing more is delivered).
Read(s) ==
  /\ st[s] = "delivering"
  /\ LET src == Src(s) IN
     \/ /\ src # <<>> /\ Head(src).w /\ used[Lane(s)] < Budget
        /\ dlv' = [dlv EXCEPT ![Lane(s)] = IF Head(src).f > 0 THEN Append(@, Head(src).f) ELSE @]
        /\ used' = [used EXCEPT ![Lane(s)] = @ + 1]
        /\ got' = [got EXCEPT ![s] = @ + (IF Head(src).f > 0 THEN 1 ELSE 0)]
        /\ IF pre[s] # <<>> THEN /\ pre' = [pre EXCEPT ![s] = Tail(@)] /\ UNCHANGED q
                            ELSE /\ q' = [q EXCEPT ![s] = Tail(@)] /\ UNCHANGED pre
        /\ UNCHANGED <<st, ch, list, waitFor, blockedSeen, truncSeen>>
     \/ /\ src # <<>> /\ Head(src).w /\ used[Lane(s)] = Budget
        /\ st' = [st EXCEPT ![s] = "blocked"] /\ blockedSeen' = TRUE
        /\ UNCHANGED <<q, pre, ch, list, waitFor, dlv, used, got, truncSeen>>
     \/ /\ src # <<>> /\ ~Head(src).w /\ Ends(s)
        /\ Remove(s) /\ truncSeen' = (truncSeen \/ got[s] > 0)
        /\ UNCHANGED <<dlv, got, blockedSeen>>
     \/ /\ src = <<>> /\ Ends(s)
        /\ Remove(s) /\ UNCHANGED <<dlv, got, blockedSeen, truncSeen>>
  /\ UNCHANGED <<fin, rstd, rat, part, ackd, wholeW, up, alive, expired, nf, drops>>
  /\ UNCHANGED opener /\ UNCHANGED <<keptSeen, exhaustedFirst>>

Unblock(s) ==
  /\ st[s] = "blocked" /\ used[Lane(s)] < Budget
  /\ st' = [st EXCEPT ![s] = "delivering"]
  /\ UNCHANGED <<q, pre, ch, list, waitFor, used>> /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED wit

\* A blocked reader is freed by the loss of its own connection (AcquireCtx), never by its own reset.
CancelLoss(s) ==
  /\ st[s] = "blocked" /\ ~alive /\ CancelOnLoss
  /\ Remove(s) /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED wit

Reader(s) == Arrive(s) \/ Release(s) \/ LatePlace(s) \/ Ack(s) \/ Probe(s) \/ Turn(s) \/ Expire(s) \/ Read(s) \/ Unblock(s)
             \/ CancelLoss(s)

Consume(l) ==
  /\ used[l] > 0 /\ used[l] > SumCh(l)
  /\ used' = [used EXCEPT ![l] = @ - 1]
  /\ UNCHANGED <<st, q, pre, ch, list, waitFor>> /\ UNCHANGED Keep /\ UNCHANGED opener /\ UNCHANGED wit

Announce ==
  /\ ~up /\ alive /\ up' = TRUE
  /\ UNCHANGED <<st, q, pre, fin, rstd, rat, part, ackd, ch, got, wholeW, list, waitFor, dlv, used, alive,
                 expired, nf, drops>>
  /\ UNCHANGED opener /\ UNCHANGED wit

\* The connection is lost: every open stream is reset (its unread units dropped).
Drop ==
  /\ alive /\ drops < MaxDrops
  /\ alive' = FALSE /\ drops' = drops + 1
  /\ rstd' = [s \in S |-> rstd[s] \/ st[s] \in Open]
  /\ q' = [s \in S |-> IF st[s] \in Open THEN <<>> ELSE q[s]]
  /\ UNCHANGED <<st, pre, fin, rat, part, ackd, ch, got, wholeW, list, waitFor, dlv, used, up, expired, nf>>
  /\ UNCHANGED opener /\ UNCHANGED wit

-----------------------------------------------------------------------------
\* Openers.

Write1(s, unit) ==
  /\ q' = [q EXCEPT ![s] = Append(@, unit)]
  /\ wholeW' = [wholeW EXCEPT ![s] = @ + (IF unit.w /\ unit.f > 0 THEN 1 ELSE 0)]

OpUnchanged == UNCHANGED <<pre, ch, got, list, waitFor, dlv, used, up, alive, expired, drops>>

FreshHonest(l) == {s \in LaneIds(l) : st[s] = "idle" /\ s \notin Greedy}
NextHonest(l) == CHOOSE s \in FreshHonest(l) : \A t \in FreshHonest(l) : s <= t

HOpen(l) ==
  /\ l \in HonestLanes /\ alive /\ cur[l] = 0 /\ pend[l] = 0 /\ FreshHonest(l) # {}
  /\ st' = [st EXCEPT ![NextHonest(l)] = "arrived"] /\ cur' = [cur EXCEPT ![l] = NextHonest(l)]
  /\ UNCHANGED <<q, fin, rstd, rat, part, ackd, wholeW, nf, pend, moves>> /\ OpUnchanged /\ UNCHANGED wit

\* A reopen (write loss, or a move): the new stream is opened, and the session swaps to it after its ACK (P-6).
HReopen(l) ==
  /\ l \in HonestLanes /\ alive /\ cur[l] # 0 /\ ackd[cur[l]] /\ pend[l] = 0 /\ moves[l] < MaxMoves
  /\ FreshHonest(l) # {}
  /\ st' = [st EXCEPT ![NextHonest(l)] = "arrived"] /\ moves' = [moves EXCEPT ![l] = @ + 1]
  /\ pend' = [pend EXCEPT ![l] = NextHonest(l)] /\ UNCHANGED <<cur, fin>>
  /\ UNCHANGED <<q, rstd, rat, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED wit

HSwap(l) ==
  /\ l \in HonestLanes /\ pend[l] # 0 /\ ackd[pend[l]]
  /\ fin' = [fin EXCEPT ![cur[l]] = TRUE]
  /\ cur' = [cur EXCEPT ![l] = pend[l]] /\ pend' = [pend EXCEPT ![l] = 0]
  /\ UNCHANGED <<st, q, rstd, rat, part, ackd, wholeW, nf, moves>> /\ OpUnchanged /\ UNCHANGED wit

HWrite(l) ==
  /\ l \in HonestLanes /\ cur[l] # 0 /\ nf[l] <= MaxFrames
  /\ Writable(cur[l]) /\ ackd[cur[l]] /\ Room(cur[l])
  /\ Write1(cur[l], [f |-> nf[l], w |-> TRUE]) /\ nf' = [nf EXCEPT ![l] = @ + 1]
  /\ UNCHANGED <<st, fin, rstd, rat, part, ackd>> /\ OpUnchanged /\ UNCHANGED opener /\ UNCHANGED wit

\* The session ends once every frame is written: the session stream is closed for writing.
HFinish(l) ==
  /\ l \in HonestLanes /\ cur[l] # 0 /\ pend[l] = 0 /\ nf[l] > MaxFrames /\ ~fin[cur[l]]
  /\ fin' = [fin EXCEPT ![cur[l]] = TRUE]
  /\ UNCHANGED <<st, q, rstd, rat, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED opener /\ UNCHANGED wit

\* A Greedy stream (the peer's own opener, honest: it writes after the ACK; a connection carries one peer, so a
\* Byzantine peer filling its streams before the ACK starves only itself) is opened once the lane's honest session stream has its place, then filled.
GOpen(s) ==
  /\ s \in Greedy /\ st[s] = "idle" /\ alive
  /\ \/ Lane(s) \notin HonestLanes
     \/ \E t \in LaneIds(Lane(s)) : t \notin Greedy /\ InList(t)
  /\ st' = [st EXCEPT ![s] = "arrived"]
  /\ UNCHANGED <<q, fin, rstd, rat, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED opener
  /\ UNCHANGED wit

GWrite(s) ==
  /\ s \in Greedy /\ Writable(s) /\ ackd[s] /\ Room(s) \* an honest peer's opener: after the ACK
  /\ Write1(s, Junk)
  /\ exhaustedFirst' = (exhaustedFirst \/ (SumQ(S) + 1 = ConnWindow - Win
                                            /\ \A l \in HonestLanes : nf[l] = 1))
  /\ UNCHANGED <<st, fin, rstd, rat, part, ackd, nf>> /\ OpUnchanged /\ UNCHANGED opener
  /\ UNCHANGED <<keptSeen, blockedSeen, truncSeen>>

BOpen(s) ==
  /\ Lane(s) \in Byz /\ s \notin Greedy /\ st[s] = "idle" /\ alive
  /\ st' = [st EXCEPT ![s] = "arrived"]
  /\ UNCHANGED <<q, fin, rstd, rat, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED opener
  /\ UNCHANGED wit

BWrite(s, whole) ==
  /\ Lane(s) \in Byz /\ s \notin Greedy /\ Writable(s) /\ Room(s) /\ nf[Lane(s)] <= MaxFrames
  /\ Write1(s, [f |-> nf[Lane(s)], w |-> whole]) /\ nf' = [nf EXCEPT ![Lane(s)] = @ + 1]
  /\ part' = [part EXCEPT ![s] = ~whole]
  /\ UNCHANGED <<st, fin, rstd, rat, ackd>> /\ OpUnchanged /\ UNCHANGED opener /\ UNCHANGED wit

BFin(s) ==
  /\ Lane(s) \in Byz /\ s \notin Greedy /\ st[s] \in Open /\ ~fin[s] /\ ~rstd[s] /\ ~rat[s]
  /\ fin' = [fin EXCEPT ![s] = TRUE]
  /\ UNCHANGED <<st, q, rstd, rat, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED opener
  /\ UNCHANGED wit

BReset(s) ==
  /\ Lane(s) \in Byz /\ s \notin Greedy /\ st[s] \in Open /\ ~fin[s] /\ ~rstd[s] /\ ~rat[s]
  /\ rstd' = [rstd EXCEPT ![s] = TRUE] /\ q' = [q EXCEPT ![s] = <<>>]
  /\ UNCHANGED <<st, fin, rat, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED opener
  /\ UNCHANGED wit

BResetAt(s) ==
  /\ Lane(s) \in Byz /\ s \notin Greedy /\ st[s] \in Open /\ ~fin[s] /\ ~rstd[s] /\ ~rat[s]
  /\ rat' = [rat EXCEPT ![s] = TRUE]
  /\ UNCHANGED <<st, q, fin, rstd, part, ackd, wholeW, nf>> /\ OpUnchanged /\ UNCHANGED opener
  /\ UNCHANGED wit

-----------------------------------------------------------------------------

Next ==
  \/ \E s \in S : Reader(s)
  \/ \E l \in Lanes : Consume(l) \/ HOpen(l) \/ HReopen(l) \/ HSwap(l) \/ HWrite(l) \/ HFinish(l)
  \/ Announce \/ Drop
  \/ \E s \in S : GOpen(s) \/ GWrite(s) \/ BOpen(s) \/ BFin(s) \/ BReset(s) \/ BResetAt(s)
                  \/ \E w \in BOOLEAN : BWrite(s, w)

Fairness ==
  /\ \A s \in S : /\ WF_vars(Arrive(s)) /\ WF_vars(Release(s)) /\ WF_vars(LatePlace(s)) /\ WF_vars(Ack(s)) /\ WF_vars(Probe(s))
                  /\ WF_vars(Turn(s)) /\ WF_vars(Read(s)) /\ WF_vars(Unblock(s)) /\ WF_vars(CancelLoss(s))
                  /\ WF_vars(GOpen(s)) /\ WF_vars(GWrite(s))
  /\ \A l \in FairConsumers : WF_vars(Consume(l))
  /\ \A l \in Lanes : WF_vars(HOpen(l)) /\ WF_vars(HSwap(l)) /\ WF_vars(HWrite(l))
  /\ WF_vars(Announce)

Spec == Init /\ [][Next]_vars /\ Fairness

-----------------------------------------------------------------------------
\* Properties.

\* P-8/P-14: at most Cap places per lane (whatever the rule operators say).
Places == \A l \in Lanes : Len(list[l]) <= 2
\* P-6/P-14: an honest lane's frames reach its consumer in the order they were written (no order given up).
Order == ~expired => \A l \in HonestLanes :
           LET d == SelectSeq(dlv[l], LAMBDA f : f > 0) IN \A i, j \in 1..Len(d) : i < j => d[i] < d[j]
\* P-13: on an honest peer's connection a stream arriving, accepting or refused holds no credit (its opener writes
\* only after the ACK, P-4/P-14): the « + 1 » of the spec's inequality is a margin, not a stream's due.
ArrivingHoldsNothing == \A s \in S : Lane(s) \notin Byz /\ st[s] \in {"arrived", "accepting", "refused"} => Held(s) = 0
\* P-12: a stream holds budget only from its turn (no waiting stream holds the budget).
WaitNoBudget == \A s \in S : ch[s] > 0 => st[s] \in {"delivering", "blocked"}
\* P-12: a stream that left returned every reservation.
ReservationsReturned == \A s \in S : st[s] \in {"gone", "refused"} => ch[s] = 0
\* P-11: a stream that ended without an abrupt reset (FIN, or RESET_STREAM_AT after its units) and left delivered
\* every whole frame written on it: whole frames before a truncated last one are never thrown away.
FinFramesDelivered == \A s \in S : (st[s] = "gone" /\ ~rstd[s] /\ (fin[s] \/ rat[s])) => got[s] = wholeW[s]
\* P-9/P-11: a stream that ended before its turn with nothing to read (nothing unread, nothing probed) leaves its
\* place. A reset stream with a probed prefix may reach its turn first: the prefix goes to the decoder, and a stalled
\* consumer then holds it (P-12's stated limit); NoBlindWait covers it while it waits.
EmptyLeaves == \A s \in S :
                 (InList(s) /\ st[s] \in {"accepting", "waiting"} /\ pre[s] = <<>> /\ (rstd[s] \/ EndedEmpty(s)))
                   ~> ~InList(s)
\* P-11: no wait is blind to its stream's reset or its connection's loss, even after the probe took units.
NoBlindWait == \A s \in S : (st[s] \in {"accepting", "waiting"} /\ rstd[s]) ~> (st[s] \notin {"accepting", "waiting"})
\* P-12: a reader blocked on the budget is freed by its connection's loss.
BlockedFreed == \A s \in S : (st[s] = "blocked" /\ ~alive) ~> (st[s] = "gone")
\* P-11/P-13: every frame an honest lane writes reaches its consumer; with Greedy streams of both lanes filled to
\* their window (and a blocked head on the other lane), this is « the head progresses under exhausted waiting credit ».
AllDelivered == <>[](\A l \in HonestLanes : nf[l] = MaxFrames + 1 /\ Len(SelectSeq(dlv[l], LAMBDA f : f > 0)) = MaxFrames)
HeadProgresses == AllDelivered

\* Witnesses (invariants the sound model must violate).
NoKept == ~keptSeen
NoBlocked == ~blockedSeen
NoTruncatedTail == ~truncSeen
NoCreditExhaustedFirst == ~exhaustedFirst
=============================================================================
