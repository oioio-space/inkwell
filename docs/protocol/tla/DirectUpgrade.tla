--------------------------- MODULE DirectUpgrade ---------------------------
(* Block HP1 (docs/plans/2026-11-16-hp1-direct-upgrade.md, rules H-1..H-19):  *)
(* a relayed link to a wanted peer keeps trying to become direct.             *)
(*                                                                            *)
(* Two nodes. L, the local node, is modelled in full: its upgrader (the       *)
(* policy of protocol/directup: states, ladders, epochs, budgets, activity),  *)
(* its calls to a cancellable DirectConnect (D1: Punch, with the direct dial  *)
(* libp2p makes first when the peerstore holds a public address) or to a      *)
(* forced direct dial (Dial), libp2p's one-shot on a new inbound circuit      *)
(* (Auto), and the answerer side of R's exchanges. Every round, whatever its  *)
(* origin, passes two choke points of L: the hole-punch address filter (the   *)
(* gate: wanted, round bucket, FIFO admission to a node-wide slot with a      *)
(* lease) and the connection gater (InterceptAddrDial: wanted, a bucket of    *)
(* non-relay destinations), since libp2p's Connect dials every address its    *)
(* peerstore holds. R is honest (its upgrader under the same role rule, paced *)
(* by time, and its one-shot) or Byzantine. Other peers compete for the slots *)
(* and may hold them as long as their lease allows.                           *)
(*                                                                            *)
(* Circuits are named by their dialler: "LR" (L dialled: outbound for L,      *)
(* inbound for R), "RL". A DCUtR stream rides the circuit its sender's        *)
(* NewStream picks (chL, chR); the answerer resets a stream that arrives on   *)
(* its inbound circuit (svc.go handleNewStream). Both ends may durably pick   *)
(* their own inbound circuit (Alloy BothAccepted): two exchanges then run at  *)
(* once; with CrossCancel, punch phases that overlap may both fail.           *)
(*                                                                            *)
(* Time: Tick is one unit; due times are countdowns, `since` the time since   *)
(* the last call started (capped), buckets refill on one free-running clock,  *)
(* leases count down; Tick waits while a granted holder of R's lease is at    *)
(* its last unit with work in progress that fits in the lease by libp2p's own *)
(* deadlines (an upper-bound timer). Ladders and caps are scaled.             *)
(*                                                                            *)
(* Budgets are checked against reference buckets driven only by the clock    *)
(* and by what really happens: a run conforms to a (sigma, rho) bound in      *)
(* every window iff its reference never goes negative.                       *)
(*                                                                            *)
(* Block HP1c (docs/plans/2026-11-23-hp1-reformalize.md, H-20, P-15): R may   *)
(* crash. Its circuits stay open at L, dead (ghosts in cr.ghost, open for L,  *)
(* nothing they carry arrives), and a new circuit comes in either direction.  *)
(* L's rounds open on their cause circuit (H-20); every older circuit proves  *)
(* it lives (P-15): a live one answers and stays, a ghost is closed. With     *)
(* ProveOlder = FALSE and MaxCrashes = 0 (every block HP1 and HP1r          *)
(* configuration) no circuit is a ghost and the proofs are not modelled: each *)
(* would be answered and change nothing but the wait of H-15, a stutter.      *)
(*                                                                            *)
(* Each rule is a named operator, so that a mutant overrides exactly one      *)
(* (mutants/DirectUpgradeMutants.tla); the sound module is never edited.      *)
EXTENDS Naturals, Integers, FiniteSets, Sequences

CONSTANTS
  Byz,           \* R is a Byzantine member (safety only)
  TwoCirc,       \* both directions may be dialled: two circuits at once
  Asym,          \* the two ends may pick different circuits
  DialDirs,      \* directions a new circuit may take
  MaxDials,      \* new circuits (dial loops, a restarted R, Byzantine reconnects): E7
  MaxRestarts,   \* honest restarts of R's process (#3420)
  MaxRelayLost,  \* the relay drops every circuit
  MaxCrashes,    \* R crashes and comes back on a new circuit; the old ones stay open at L, dead (HP1c)
  ProveOlder,    \* P-15 is modelled: every older circuit proves it lives when a new one comes (HP1c)
  MaxDirectLost, \* the direct connection is lost, the session falls back to a circuit
  MaxWantFlips,  \* wanted / not wanted (membership, invitation window)
  MaxCloses,     \* the direct path closes without any event (it reopens under fairness)
  MaxRepicks,    \* NewStream's choice changes (stream counts change)
  MaxLostPush,   \* identify pushes lost while the circuit lives
  MaxIdles,      \* the app goes idle or to sleep (it wakes under fairness)
  MaxRounds,     \* rounds per DirectConnect (3 in libp2p)
  TMin, TBusy, Off, FastBase, JCap, SlowBase, KCap,
  ACap, APeriod, \* per-peer attempt bucket; the node-wide admission bucket refills on the same period
  NCap,          \* node-wide admissions (leases granted, all peers, all origins)
  RCap, RPeriod, \* per-peer round bucket (the gate)
  DCap,          \* per-peer bucket of non-relay dial destinations (the gater), refilled every RPeriod
  K,             \* node-wide slots, granted FIFO, one ticket per peer
  TLease,        \* a slot's lease: libp2p's deadlines end the holder's work within it
  OPeers,        \* the other peers that compete for the slots
  MaxOthers,     \* tickets the other peers may take
  OthersForever, \* the other peers ask for slots without end (a Byzantine member)
  CrossCancel,   \* overlapping punch phases of two exchanges of the pair may both fail
  LAllow,        \* non-relay destinations one slot grant allows (every origin of the peer, while it holds it)
  WantedInit, LowerInit, PublicInit  \* initial wanted, L's PeerID lower, R public: subsets of BOOLEAN

ASSUME /\ APeriod % RPeriod = 0 /\ MaxRounds >= 1 /\ TMin >= 1 /\ TBusy >= 1 /\ TLease >= 1
       /\ ACap >= 1 /\ RCap >= 1 /\ DCap >= 1 /\ NCap >= 1 /\ K >= 1 /\ LAllow >= 1 /\ DialDirs \subseteq {"LR", "RL"}
       /\ "r" \notin OPeers

Dirs == {"LR", "RL"}
Peers == {"r"} \cup OPeers
In(s) == IF s = "L" THEN "RL" ELSE "LR"            \* the circuit side s sees as inbound
Out(s) == IF s = "L" THEN "LR" ELSE "RL"
Min(a, b) == IF a < b THEN a ELSE b
Max(a, b) == IF a > b THEN a ELSE b
IncTo(a, c) == Min(a + 1, c)
Fast(j) == FastBase * 2^j                           \* H-8: 1 s * 2^j, cap 30 s (scaled)
Slow(k) == SlowBase * 2^k                           \* H-8: 60 s * 2^k, cap 16 min (scaled)
NoCall == [act |-> "none", stale |-> FALSE, spent |-> FALSE, rounds |-> 0, res |-> "none", pre |-> FALSE]
Cnt0 == [dials |-> 0, restarts |-> 0, rlost |-> 0, dlost |-> 0, wflips |-> 0, closes |-> 0,
         repicks |-> 0, lostpush |-> 0, idles |-> 0, crashes |-> 0]
\* HP1c: ghost, circuits dead at R that L still holds open (by direction); pl, pg: P-15 proofs pending on an older
\* live circuit, on a ghost; the rest are ghost flags of the properties and witnesses.
Cr0 == [ghost |-> {}, pl |-> {}, pg |-> {}, ghostRound |-> FALSE, besideGhost |-> FALSE, liveClosed |-> FALSE,
        answered |-> FALSE, proofClosed |-> FALSE, liveResolved |-> FALSE]
ASSUME MaxCrashes > 0 => ProveOlder                 \* a ghost is always under proof (P-15)

VARIABLES
  wanted, circuits, chL, chR, direct, lReady, rReady, rKnown, idDue, punchable, llower, rPublic,
  active,      \* the app is active (D4); idle or asleep otherwise
  autoL,       \* L's libp2p one-shot: -1 none, -2 its first direct dial pending, else rounds done
  lPunch,      \* an L-initiated punch phase in progress: "none", "call", "auto"
  rAuto,       \* R's libp2p one-shot on its new inbound circuit is pending
  rPend,       \* an R-initiated exchange waits in L's gate (FilterLocal) for R's lease
  rPunch,      \* an R-initiated punch phase in progress (L as answerer dials)
  rRest,       \* R's last exchange just ended: an honest R starts the next one after some time
  crossed,     \* two punch phases of the pair overlapped since both were last idle
  pol,         \* L's policy: [st, wait, j, k, ev] (ev: the due time comes from an event, not a ladder)
  call,        \* L's call for R (NoCall: none)
  ncalls,      \* L's calls for R in flight (H-4)
  lastCls,     \* L's last scheduling step: "wait" (queued for a slot), "busy", "other", "none" (L2)
  attTok, refAtt, roundTok, refRound, dTok, refDest, nTok, refN, clk, since,
  lease,       \* [p \in Peers |-> remaining lease of p's node-wide slot; 0: none]
  allow,       \* [p \in Peers |-> destinations p's current grant still allows]
  queue,       \* peers waiting for a slot, in arrival order, one ticket per peer
  othersLeft,  \* tickets the other peers may still take
  cnt, g,
  cr           \* HP1c: ghosts, P-15 proofs and their flags (Cr0)

env0 == <<wanted, circuits, chL, chR, lReady, rReady, rKnown, idDue, punchable, llower, rPublic, active>>
envc == <<env0, cr>>
opv == <<autoL, lPunch, rAuto, rPend, rPunch, rRest, crossed>>
budv == <<attTok, refAtt, roundTok, refRound, dTok, refDest, nTok, refN, allow>>
timev == <<clk, since>>
slotv == <<ncalls, lease, queue, othersLeft>>
vars == <<envc, direct, opv, pol, call, lastCls, budv, timev, slotv, cnt, g>>

Lower(s) == IF s = "L" THEN llower ELSE ~llower
\* The circuits side s sees open: L cannot tell a ghost from a live circuit until P-15 closes it; R, back from its
\* crash, knows only its live ones.
Sees(s) == IF s = "L" THEN circuits \cup cr.ghost ELSE circuits

\* ---- The rules (overridden one at a time by the mutants) ----
WantedOK == wanted                       \* H-1: the upgrader acts only for a wanted peer
NotDirect == ~direct                     \* H-1, H-14: only while the session is on a circuit
HasRole(s) == In(s) \in Sees(s)          \* H-3: Punch only over an inbound circuit
TieOK(s) == TRUE                         \* H-3, §3.4: both sides holding an inbound circuit are eligible
LReadyOK == lReady                       \* H-2: our handler registered (localReady)
RemoteGuard == FALSE                     \* H-2: remoteReady is a trigger, not a guard (the poll calls)
OneCallOK == call.act = "none"           \* H-4: the next call starts after the result
SlotFree == Cardinality({p \in Peers : lease[p] > 0}) < K   \* H-5: at most K slots on the node
GrantOrder(p) == Head(queue) = p         \* H-5: slots granted FIFO, one ticket per peer
NodeTokOK == nTok >= 1                   \* H-16: a grant takes a node-wide admission token
Expire == TRUE                           \* H-5: a lease ends at its deadline, whatever its holder does
ApplyStale == FALSE                      \* H-6: a result applies only to its epoch
EpochAtt(a) == a                         \* H-9, H-11: no event refills the attempt bucket
EpochRounds(r) == r                      \* H-9, H-12: no event refills the round bucket
BusyCharge(a) == a                       \* H-7: Busy takes no token
Retry == TRUE                            \* H-8: a failed call is retried on a ladder
StopAfter(k) == FALSE                    \* H-10: no terminal hopeless state
GateWanted == wanted                     \* H-12: no address to or from a peer that is not wanted
GateBudget(origin) == roundTok >= 1      \* H-12: every round, both roles, libp2p's own included
GaterWanted == wanted                    \* H-12: no non-relay dial to a peer that is not wanted
GaterBudget(origin) == dTok >= 1         \* H-12: every non-relay destination, every origin
GaterLease == lease["r"] > 0 /\ allow["r"] >= 1   \* H-16: a destination only under a grant, within its allowance
OAllowOK(o) == allow[o] >= 1             \* H-16: the same for the other peers' rounds (several may share a grant)
TicketOK == wanted                       \* H-1, H-5: a peer that is not wanted takes no ticket
DropTicket == ~wanted \/ circuits = {} \/ direct   \* H-5: an abandoned wait keeps the ticket's place
CloseDup == TRUE                         \* H-15: with two circuits nobody initiates
ResolveOK == TRUE                        \* H-15: the circuit the higher PeerID dialled is closed
ActiveOK == active \/ pol.ev             \* H-17: ladders run only while active; events call at once
\* H-19: a direct connection is kept only if, once registered, the node holds a circuit to the peer (or wants it,
\* or connects to it on request: both outside the model, so the model is the conservative case); one that lands
\* after the last circuit is gone (the late end of any attempt: our call, the one-shot, the answerer) is closed.
LandOK == circuits # {}
\* H-20 (HP1c): a round L initiates opens on the circuit that causes it (the one-shot: the new inbound circuit; a
\* call: the epoch's), never on the one libp2p picks; a crash brings a new circuit and a new epoch in one step, so
\* that circuit is live: no round of L rides a ghost (libp2p prefers the older circuit, which carries more streams).
RoundGhost(origin) == FALSE
\* P-15 (HP1c): an older circuit that answers its proof stays open, and nothing changes.
KeepAnswered == TRUE
\* H-15 (HP1c): L settles a duplicate only once the P-15 proofs are done, so a ghost is never taken for one.
ResolveWaits == cr.pl = {} /\ cr.pg = {}
Resume(p) == IF p.st = "sched" THEN [p EXCEPT !.wait = 0] ELSE p   \* H-17: resume at once on activity

\* ---- The observation ----
Eligible(s) == HasRole(s) /\ TieOK(s) /\ (CloseDup => Sees(s) # Dirs)
DialRole == Sees("L") = {"LR"} /\ rPublic         \* H-3: Dial only with outbound only and a public address
LAct == IF Eligible("L") THEN "punch" ELSE IF DialRole THEN "dial" ELSE "none"
Ready == LReadyOK /\ LAct # "none" /\ ((RemoteGuard /\ LAct = "punch") => rKnown)
Acts == WantedOK /\ circuits # {} /\ NotDirect
Target == IF Acts THEN (IF Ready THEN "sched" ELSE "idle")
          ELSE IF direct /\ WantedOK THEN "direct" ELSE "off"
Offset == IF Sees("L") = Dirs /\ ~llower THEN Off ELSE 0  \* §3.3 T_offset: the higher PeerID, both inbound
Due0 == Max(0, TMin - since) + Offset                     \* max(now, lastStart + T_min)
Gate(origin) == GateWanted /\ GateBudget(origin)
Gater(origin) == GaterWanted /\ GaterLease /\ GaterBudget(origin)
Fresh == call.act # "none" /\ ~call.stale
InQ(p) == \E i \in 1..Len(queue) : queue[i] = p
Drop(q, p) == SelectSeq(q, LAMBDA x : x # p)
CanTake(p) == queue # <<>> /\ SlotFree /\ GrantOrder(p)
Ended(c) == [c EXCEPT !.res = IF c.spent THEN "spent" ELSE "unspent"]
\* L's upgrader wants a slot for R; R's lease is wanted by any of the pair's rounds at L.
WantsSlot == pol.st = "sched" /\ pol.wait = 0 /\ OneCallOK /\ Target = "sched" /\ attTok >= 1 /\ ActiveOK
RWants == WantsSlot \/ autoL >= 0 \/ rPend
\* An honest R may start an exchange now (its upgrader under the same rule, or its one-shot).
RArriveOK == ~rPend /\ ~rPunch /\ ~rRest /\ rReady /\ lReady /\ circuits # {} /\ ~direct
             /\ chR = In("R") /\ (rAuto \/ Eligible("R"))
\* Work of the pair at L that libp2p's own deadlines end within R's lease (Tick waits for it at the last unit);
\* an honest R refused for lack of a slot comes back within the hold (H-5: its fast ladder, its token refill).
RWork == lPunch # "none" \/ rPunch \/ WantsSlot
         \/ (~Byz /\ (call.res = "run" \/ autoL # -1 \/ rPend \/ RArriveOK))

\* The policy's reaction to a change of what it observes (after the environment's new values are set).
\* c: the call record after the action's own effect; ep: a new circuit (E7, H-6). D1: a new epoch, an
\* unwant, Close or a direct session cancels a call that has not reached its punch phase.
React(c, ep) ==
  LET t == Target'
      stop == t \in {"off", "direct"}
      trig == ep \/ (rKnown' /\ ~rKnown) \/ (lReady' /\ ~lReady) \/ (LAct' # LAct)
      due == Due0'
  IN /\ call' = IF c.act # "none" /\ (ep \/ stop)
                  THEN [c EXCEPT !.stale = TRUE, !.res = IF c.res = "run" THEN Ended(c).res ELSE c.res]
                  ELSE c
     /\ lastCls' = IF stop THEN "none" ELSE lastCls
     /\ pol' = CASE stop -> [st |-> t, wait |-> 0, j |-> 0, k |-> 0, ev |-> FALSE]
                 [] ep \/ pol.st \in {"off", "direct"} ->              \* E1, E7, fallback: ladders reset
                      [st |-> t, wait |-> IF t = "sched" THEN due ELSE 0, j |-> 0, k |-> 0, ev |-> t = "sched"]
                 [] pol.st \in {"inflight", "rest"} -> pol             \* the call stays in flight (H-4)
                 [] t = "idle" -> [pol EXCEPT !.st = "idle", !.wait = 0, !.ev = FALSE]
                 [] pol.st = "idle" \/ trig -> [pol EXCEPT !.st = "sched", !.wait = due, !.ev = TRUE]
                 [] OTHER -> pol

\* A round through the gate (r) and/or a non-relay destination through the gater (d); land: a direct
\* connection lands in this step (the ghost lateLand records one without a circuit, H-19).
SpendL(r, d, land) ==
  /\ roundTok' = IF r THEN Max(0, roundTok - 1) ELSE roundTok
  /\ refRound' = IF r THEN refRound - 1 ELSE refRound
  /\ dTok' = IF d THEN Max(0, dTok - 1) ELSE dTok
  /\ refDest' = IF d THEN refDest - 1 ELSE refDest
  /\ allow' = IF d THEN [allow EXCEPT !["r"] = Max(0, @ - 1)] ELSE allow
  /\ g' = [g EXCEPT !.unwantedRound = @ \/ (r /\ ~wanted), !.unwantedDial = @ \/ (d /\ ~wanted),
                    !.unallowedDial = @ \/ (d /\ (lease["r"] = 0 \/ allow["r"] = 0)),
                    !.lateLand = @ \/ (land /\ circuits = {})]
  /\ UNCHANGED <<attTok, refAtt, nTok, refN>>
Spend(r, d) == SpendL(r, d, FALSE)
\* A direct connection lands in a step that spends nothing (the end of a punch phase).
Landed == g' = [g EXCEPT !.lateLand = @ \/ circuits = {}]

Init ==
  /\ wanted \in WantedInit
  /\ circuits = {} /\ chL = "none" /\ chR = "none" /\ direct = FALSE
  /\ lReady \in BOOLEAN /\ rReady \in BOOLEAN /\ rKnown = FALSE /\ idDue = FALSE
  /\ punchable \in BOOLEAN /\ llower \in LowerInit /\ rPublic \in PublicInit /\ active = TRUE
  /\ autoL = -1 /\ lPunch = "none" /\ rAuto = FALSE /\ rPend = FALSE /\ rPunch = FALSE /\ rRest = FALSE
  /\ crossed = FALSE
  /\ pol = [st |-> "off", wait |-> 0, j |-> 0, k |-> 0, ev |-> FALSE]
  /\ call = NoCall /\ ncalls = 0 /\ lastCls = "none"
  /\ attTok = ACap /\ refAtt = ACap /\ roundTok = RCap /\ refRound = RCap /\ dTok = DCap /\ refDest = DCap
  /\ nTok = NCap /\ refN = NCap /\ clk = 0 /\ since = TMin
  /\ lease = [p \in Peers |-> 0] /\ allow = [p \in Peers |-> 0] /\ queue = <<>> /\ othersLeft = MaxOthers
  /\ cr = Cr0
  /\ cnt = Cnt0 /\ g = [staleTouched |-> FALSE, unwantedRound |-> FALSE, unwantedDial |-> FALSE,
                        idleCall |-> FALSE, unallowedDial |-> FALSE, ticketUnwanted |-> FALSE,
                        lateLand |-> FALSE]

\* ---- Environment ----

\* A new circuit (E7). Identify runs on it; libp2p's one-shot starts at the receiver if its hole puncher
\* exists (svc.go: only once ready), with a direct dial first when the peerstore holds a public address.
\* P-15 (HP1c): every older circuit still open proves it lives (a ghost is already proving, since its crash).
Dial(d) ==
  /\ d \in DialDirs /\ cnt.dials < MaxDials
  /\ LET cs == IF TwoCirc THEN circuits \cup {d} ELSE {d} IN
       /\ circuits' = cs
       /\ \E a \in cs : /\ chL' = a
                        /\ IF Asym THEN chR' \in cs ELSE chR' = a
       /\ cr' = IF ProveOlder THEN [cr EXCEPT !.pl = @ \cup ((cs \cap circuits) \ {d})] ELSE cr
  /\ idDue' = TRUE /\ rKnown' = FALSE
  /\ autoL' = IF d = "RL" /\ lReady /\ autoL = -1 /\ call.res \notin {"run", "punch"}   \* else ErrHolePunchActive
                THEN (IF rPublic THEN -2 ELSE 0) ELSE autoL
  /\ rAuto' = ((d = "LR" /\ rReady /\ ~Byz) \/ rAuto)
  /\ attTok' = EpochAtt(attTok) /\ roundTok' = EpochRounds(roundTok)
  /\ cnt' = [cnt EXCEPT !.dials = @ + 1]
  /\ UNCHANGED <<wanted, direct, lReady, rReady, punchable, llower, rPublic, active, lPunch, rPend, rPunch,
                 rRest, crossed, refAtt, refRound, dTok, refDest, nTok, refN, allow, timev, slotv, g>>
  /\ React(call, TRUE)

\* An honest restart closes R's circuits (L learns it); the model has none while L still holds a ghost.
RRestart ==
  /\ ~Byz /\ cnt.restarts < MaxRestarts /\ (circuits # {} \/ direct) /\ cr.ghost = {}
  /\ circuits' = {} /\ chL' = "none" /\ chR' = "none" /\ direct' = FALSE
  /\ rReady' = FALSE /\ rKnown' = FALSE /\ idDue' = FALSE /\ rAuto' = FALSE /\ rPend' = FALSE
  /\ cnt' = [cnt EXCEPT !.restarts = @ + 1] /\ cr' = [cr EXCEPT !.pl = {}]
  /\ UNCHANGED <<wanted, lReady, punchable, llower, rPublic, active, autoL, lPunch, rPunch, rRest, crossed,
                 budv, timev, slotv, g>>
  /\ React(call, FALSE)

\* HP1c: R crashes without closing anything and comes back on a new circuit d, in the same direction (R dials
\* again) or the other (L does). Every circuit L held stays open at L, dead (a ghost), until P-15 closes it; the
\* new circuit is a new epoch (E7) and starts libp2p's one-shot at L as Dial does. R's new process registers its
\* handler later (RReady). The pair has no exchange in progress (a crash during one ends it, as CanFail does).
RCrash(d) ==
  /\ cnt.crashes < MaxCrashes /\ ~Byz /\ d \in DialDirs
  /\ circuits # {} /\ ~direct /\ lPunch = "none" /\ autoL = -1 /\ ~rPend /\ ~rPunch
  /\ circuits' = {d} /\ chL' = d /\ chR' = d
  /\ cr' = [cr EXCEPT !.ghost = @ \cup circuits, !.pg = @ \cup circuits, !.pl = {}]
  /\ rReady' = FALSE /\ rKnown' = FALSE /\ idDue' = TRUE /\ rAuto' = FALSE
  /\ autoL' = IF d = "RL" /\ lReady /\ call.res \notin {"run", "punch"} THEN (IF rPublic THEN -2 ELSE 0) ELSE -1
  /\ attTok' = EpochAtt(attTok) /\ roundTok' = EpochRounds(roundTok)
  /\ cnt' = [cnt EXCEPT !.crashes = @ + 1]
  /\ UNCHANGED <<wanted, direct, lReady, punchable, llower, rPublic, active, lPunch, rPend, rPunch, rRest, crossed,
                 refAtt, refRound, dTok, refDest, nTok, refN, allow, timev, slotv, g>>
  /\ React(call, TRUE)

\* P-15 (HP1c) on an older live circuit: R answers within T_vie; the circuit stays open and nothing changes.
ProveAnswered(d) ==
  /\ d \in cr.pl
  /\ IF KeepAnswered
       THEN /\ cr' = [cr EXCEPT !.pl = @ \ {d}, !.answered = TRUE]
            /\ UNCHANGED <<env0, direct, opv, pol, call, lastCls, budv, timev, slotv, cnt, g>>
       ELSE LET rest == circuits \ {d}
                keep(ch) == IF ch \in rest THEN ch ELSE IF rest = {} THEN "none" ELSE CHOOSE x \in rest : TRUE
            IN /\ circuits' = rest /\ chL' = keep(chL) /\ chR' = keep(chR)
               /\ cr' = [cr EXCEPT !.pl = @ \ {d}, !.liveClosed = TRUE]
               /\ UNCHANGED <<wanted, lReady, rReady, rKnown, idDue, punchable, llower, rPublic, active,
                              direct, opv, budv, timev, slotv, cnt, g>>
               /\ React(call, FALSE)

\* P-15 (HP1c) on a ghost: no byte within T_vie, and the newer circuit carries an acknowledged sync stream (the
\* session's ACK, outside this model, comes: R is honest): the ghost is closed and L sees it gone.
ProveClose(d) ==
  /\ d \in cr.pg
  /\ cr' = [cr EXCEPT !.ghost = @ \ {d}, !.pg = @ \ {d}, !.proofClosed = TRUE]
  /\ UNCHANGED <<env0, direct, opv, budv, timev, slotv, cnt, g>>
  /\ React(call, FALSE)

\* The relay drops every circuit, the dead ones included.
RelayLost ==
  /\ cnt.rlost < MaxRelayLost /\ circuits # {}
  /\ circuits' = {} /\ chL' = "none" /\ chR' = "none" /\ rKnown' = FALSE /\ idDue' = FALSE
  /\ rAuto' = FALSE /\ rPend' = FALSE
  /\ cnt' = [cnt EXCEPT !.rlost = @ + 1] /\ cr' = [cr EXCEPT !.ghost = {}, !.pl = {}, !.pg = {}]
  /\ UNCHANGED <<wanted, direct, lReady, rReady, punchable, llower, rPublic, active, autoL, lPunch, rPunch,
                 rRest, crossed, budv, timev, slotv, g>>
  /\ React(call, FALSE)

DirectLost ==
  /\ direct /\ cnt.dlost < MaxDirectLost
  /\ direct' = FALSE /\ cnt' = [cnt EXCEPT !.dlost = @ + 1]
  /\ UNCHANGED <<envc, opv, budv, timev, slotv, g>>
  /\ React(call, FALSE)

WantFlip ==
  /\ cnt.wflips < MaxWantFlips
  /\ wanted' = ~wanted /\ cnt' = [cnt EXCEPT !.wflips = @ + 1]
  /\ UNCHANGED <<cr, circuits, chL, chR, lReady, rReady, rKnown, idDue, punchable, llower, rPublic, active,
                 direct, opv, budv, timev, slotv, g>>
  /\ React(call, FALSE)

Close ==
  /\ punchable /\ cnt.closes < MaxCloses
  /\ punchable' = FALSE /\ cnt' = [cnt EXCEPT !.closes = @ + 1]
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rReady, rKnown, idDue, llower, rPublic, active,
                 direct, opv, pol, call, lastCls, budv, timev, slotv, g>>
Open ==
  /\ ~punchable /\ punchable' = TRUE
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rReady, rKnown, idDue, llower, rPublic, active,
                 direct, opv, pol, call, lastCls, budv, timev, slotv, cnt, g>>

\* NewStream's choice changes with the stream counts; invisible to the policy.
Repick ==
  /\ circuits # {} /\ cnt.repicks < MaxRepicks
  /\ \E a \in circuits : /\ chL' = a
                         /\ IF Asym THEN chR' \in circuits ELSE chR' = a
  /\ cnt' = [cnt EXCEPT !.repicks = @ + 1]
  /\ UNCHANGED <<cr, wanted, circuits, direct, lReady, rReady, rKnown, idDue, punchable, llower, rPublic, active,
                 opv, pol, call, lastCls, budv, timev, slotv, g>>
\* Both ends on their own outbound circuit needs unequal stream counts (DirectUpgradeRoles.als,
\* SomeEligibleAccepted), which last only while an opening is in flight. No other pick is ever aligned.
Settle ==
  /\ Asym /\ chL = Out("L") /\ chR = Out("R")
  /\ \E a \in circuits : chL' = a /\ chR' = a
  /\ UNCHANGED <<cr, wanted, circuits, direct, lReady, rReady, rKnown, idDue, punchable, llower, rPublic, active,
                 opv, pol, call, lastCls, budv, timev, slotv, cnt, g>>
\* H-15: once both circuits exist, the one the higher PeerID dialled is closed (once the session is off it). L sees
\* a ghost as a circuit; HP1c: it waits for the P-15 proofs, so a ghost (under proof until it is closed, TypeOK) is
\* never taken for a duplicate, and once they are done L's view is the live circuits (ResolveWaits). Without that
\* wait (M33) the circuit kept may be a ghost: the live one is closed and L is left on the dead one (S21). With it,
\* no ghost is left (TypeOK), keep is live and left = {keep}.
Resolve ==
  /\ CloseDup /\ ResolveOK /\ Sees("L") = Dirs /\ ResolveWaits
  /\ LET keep == IF llower THEN "LR" ELSE "RL"
         left == circuits \cap {keep}
         ch == IF left = {} THEN "none" ELSE keep IN
       /\ circuits' = left /\ chL' = ch /\ chR' = ch
       /\ cr' = [cr EXCEPT !.liveResolved = @ \/ (left = {} /\ circuits # {})]
  /\ rAuto' = (rAuto /\ ~llower) /\ rPend' = (rPend /\ ~llower)
  /\ autoL' = IF llower THEN -1 ELSE autoL
  /\ UNCHANGED <<wanted, direct, lReady, rReady, rKnown, idDue, punchable, llower, rPublic, active,
                 lPunch, rPunch, rRest, crossed, budv, timev, slotv, cnt, g>>
  /\ React(call, FALSE)

LReady ==
  /\ ~lReady /\ lReady' = TRUE
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, rReady, rKnown, idDue, punchable, llower, rPublic, active,
                 direct, opv, budv, timev, slotv, cnt, g>>
  /\ React(call, FALSE)

\* R registers its handler; identify pushes the change once; its stream may fail (MaxLostPush).
RReady ==
  /\ ~rReady /\ rReady' = TRUE
  /\ \/ /\ idDue' = (idDue \/ circuits # {}) /\ UNCHANGED cnt
     \/ /\ circuits # {} /\ cnt.lostpush < MaxLostPush
        /\ idDue' = idDue /\ cnt' = [cnt EXCEPT !.lostpush = @ + 1]
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rKnown, punchable, llower, rPublic, active,
                 direct, opv, pol, call, lastCls, budv, timev, slotv, g>>

Learn ==
  /\ circuits # {} /\ idDue
  /\ rKnown' = rReady /\ idDue' = FALSE
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rReady, punchable, llower, rPublic, active,
                 direct, opv, budv, timev, slotv, cnt, g>>
  /\ React(call, FALSE)

ByzToggle ==
  /\ Byz /\ circuits # {}
  /\ rKnown' = ~rKnown
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rReady, idDue, punchable, llower, rPublic, active,
                 direct, opv, budv, timev, slotv, cnt, g>>
  /\ React(call, FALSE)

\* D4: the app goes idle or to sleep; it becomes active again (the person returns), at once resuming a due.
Idle ==
  /\ active /\ cnt.idles < MaxIdles
  /\ active' = FALSE /\ cnt' = [cnt EXCEPT !.idles = @ + 1]
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rReady, rKnown, idDue, punchable, llower, rPublic,
                 direct, opv, pol, call, lastCls, budv, timev, slotv, g>>
Wake ==
  /\ ~active /\ active' = TRUE /\ pol' = Resume(pol)
  /\ UNCHANGED <<cr, wanted, circuits, chL, chR, lReady, rReady, rKnown, idDue, punchable, llower, rPublic,
                 direct, opv, call, lastCls, budv, timev, slotv, cnt, g>>

\* ---- Node-wide slots (H-5, H-16): FIFO tickets, one per peer; leases end at their deadline ----
RWait ==
  /\ TicketOK /\ RWants /\ lease["r"] = 0 /\ ~InQ("r")
  /\ queue' = Append(queue, "r") /\ lastCls' = IF WantsSlot THEN "wait" ELSE lastCls
  /\ g' = [g EXCEPT !.ticketUnwanted = @ \/ ~wanted]
  /\ UNCHANGED <<envc, direct, opv, pol, call, ncalls, lease, othersLeft, budv, timev, cnt>>
\* A ticket leaves the queue only when its peer stops being wanted or relayed: a round that gave up waiting
\* (the filter's 20 s) keeps the ticket's place, and the peer's next round finds it (B6).
LeaveQueue ==
  /\ InQ("r") /\ DropTicket
  /\ queue' = Drop(queue, "r") /\ lastCls' = IF lastCls = "wait" THEN "none" ELSE lastCls
  /\ UNCHANGED <<envc, direct, opv, pol, call, ncalls, lease, othersLeft, budv, timev, cnt, g>>
OWait(o) ==
  /\ lease[o] = 0 /\ ~InQ(o) /\ othersLeft > 0
  /\ queue' = Append(queue, o) /\ othersLeft' = IF OthersForever THEN othersLeft ELSE othersLeft - 1
  /\ UNCHANGED <<envc, direct, opv, pol, call, lastCls, ncalls, lease, budv, timev, cnt, g>>
\* The semaphore hands a freed slot to the head of the queue, with its destination allowance; a ticket whose
\* round gave up gets it too, held for its peer until the lease ends.
Grant(p) ==
  /\ InQ(p) /\ CanTake(p) /\ NodeTokOK /\ (p = "r" => TicketOK)
  /\ lease' = [lease EXCEPT ![p] = TLease] /\ allow' = [allow EXCEPT ![p] = LAllow] /\ queue' = Drop(queue, p)
  /\ nTok' = Max(0, nTok - 1) /\ refN' = refN - 1
  /\ g' = [g EXCEPT !.ticketUnwanted = @ \/ (p = "r" /\ ~wanted)]
  /\ UNCHANGED <<envc, direct, opv, pol, call, lastCls, ncalls, othersLeft, attTok, refAtt, roundTok,
                 refRound, dTok, refDest, timev, cnt>>
\* A holder may give its slot back early; nothing forces it to (its lease ends it).
ODone(o) ==
  /\ lease[o] > 0 /\ lease' = [lease EXCEPT ![o] = 0] /\ allow' = [allow EXCEPT ![o] = 0]
  /\ UNCHANGED <<envc, direct, opv, pol, call, lastCls, ncalls, queue, othersLeft, attTok, refAtt, roundTok,
                 refRound, dTok, refDest, nTok, refN, timev, cnt, g>>
\* Another peer's rounds dial under its grant; several rounds may share one grant (D3, B3), never its allowance.
ODial(o) ==
  /\ lease[o] > 0 /\ OAllowOK(o)
  /\ allow' = [allow EXCEPT ![o] = Max(0, @ - 1)]
  /\ g' = [g EXCEPT !.unallowedDial = @ \/ allow[o] = 0]
  /\ UNCHANGED <<envc, direct, opv, pol, call, lastCls, slotv, attTok, refAtt, roundTok, refRound, dTok, refDest,
                 nTok, refN, timev, cnt>>
\* libp2p's goroutines give up waiting in the filter (20 s): R's exchange and L's one-shot. The ticket is taken
\* as the round enters the filter, so a round only gives up once it holds one.
RGiveUp ==
  /\ rPend /\ lease["r"] = 0 /\ InQ("r")
  /\ rPend' = FALSE /\ rRest' = TRUE
  /\ UNCHANGED <<envc, direct, autoL, lPunch, rAuto, rPunch, crossed, pol, call, lastCls, budv, slotv, timev, cnt, g>>
AutoGiveUp ==
  /\ autoL >= 0 /\ lease["r"] = 0 /\ InQ("r")
  /\ autoL' = -1
  /\ UNCHANGED <<envc, direct, lPunch, rAuto, rPend, rPunch, rRest, crossed, pol, call, lastCls, budv, slotv, timev,
                 cnt, g>>

\* ---- L's upgrader and its calls ----

\* Due, ready, a token, active (or an event), R's slot held: a call, or Busy (libp2p's own call holds the
\* per-peer lock, holepuncher.go beginDirectConnect).
Launch ==
  /\ WantsSlot /\ lease["r"] > 0
  /\ since' = 0
  /\ IF LAct = "punch" /\ autoL # -1
       THEN /\ pol' = [pol EXCEPT !.wait = TBusy]
            /\ attTok' = BusyCharge(attTok) /\ lastCls' = "busy"
            /\ UNCHANGED <<call, ncalls, refAtt, g>>
       ELSE /\ call' = [act |-> LAct, stale |-> FALSE, spent |-> FALSE, rounds |-> 0, res |-> "run",
                        pre |-> LAct = "punch" /\ rPublic]
            /\ ncalls' = ncalls + 1
            /\ pol' = [pol EXCEPT !.st = "inflight"]
            /\ attTok' = attTok - 1 /\ refAtt' = refAtt - 1 /\ lastCls' = "other"
            /\ g' = [g EXCEPT !.idleCall = @ \/ (~active /\ ~pol.ev)]
  /\ UNCHANGED <<envc, direct, opv, roundTok, refRound, dTok, refDest, nTok, refN, allow, clk, lease, queue,
                 othersLeft, cnt>>

\* DirectConnect's first step: a direct dial to every peerstore address (holepuncher.go:113), before any
\* hole-punch filter; only the gater bounds its destinations.
PreDial ==
  /\ call.res = "run" /\ call.pre
  /\ IF Gater("pre")
       THEN \/ /\ punchable /\ LandOK /\ direct' = TRUE /\ SpendL(FALSE, TRUE, TRUE)
               /\ UNCHANGED <<envc, opv, slotv, timev, cnt>>
               /\ React([call EXCEPT !.pre = FALSE, !.spent = TRUE, !.res = "success"], FALSE)
            \/ /\ ~punchable \/ Byz \/ ~LandOK
               /\ Spend(FALSE, TRUE)
               /\ call' = [call EXCEPT !.pre = FALSE, !.spent = TRUE]
               /\ UNCHANGED <<envc, direct, opv, pol, lastCls, slotv, timev, cnt>>
       ELSE /\ call' = [call EXCEPT !.pre = FALSE]
            /\ UNCHANGED <<envc, direct, opv, pol, lastCls, budv, slotv, timev, cnt, g>>

\* A round of L's Punch: the stream rides chL; R resets it unless L sees chL as inbound. The gate gives
\* the round's addresses; the gater lets the punch phase dial; without both, no packet leaves. A round on a
\* ghost gets no answer (H-20 forbids it: RoundGhost); cr records it, and a round beside one (the witness).
RoundOn(origin) == cr' = [cr EXCEPT !.ghostRound = @ \/ RoundGhost(origin), !.besideGhost = @ \/ In("L") \in cr.ghost]
RoundStart ==
  /\ call.res = "run" /\ call.act = "punch" /\ ~call.pre /\ lReady /\ lease["r"] > 0 /\ lPunch = "none"
  /\ RoundOn("ours")
  /\ LET acc == ~RoundGhost("ours") /\ chL = In("L") /\ circuits # {} /\ (rReady \/ Byz)
         c1 == [call EXCEPT !.spent = TRUE, !.rounds = @ + 1]
     IN \/ /\ ~acc \/ Byz \/ ~Gate("ours")
           /\ call' = Ended(call)
           /\ UNCHANGED <<env0, direct, opv, pol, lastCls, budv, slotv, timev, cnt, g>>
        \/ /\ acc /\ Gate("ours")
           /\ IF Gater("ours")
                THEN /\ Spend(TRUE, TRUE)
                     /\ lPunch' = "call" /\ crossed' = (crossed \/ rPunch)
                     /\ call' = [call EXCEPT !.spent = TRUE, !.res = "punch"]
                ELSE /\ Spend(TRUE, FALSE) /\ UNCHANGED <<lPunch, crossed>>
                     /\ call' = IF c1.rounds = MaxRounds THEN Ended(c1) ELSE c1
           /\ UNCHANGED <<env0, direct, autoL, rAuto, rPend, rPunch, rRest, pol, lastCls, slotv, timev, cnt>>

\* The end of a punch phase. It may fail while the path is open only for a Byzantine R or, with
\* CrossCancel, after an overlap with the pair's other exchange.
CanFail == ~punchable \/ Byz \/ (CrossCancel /\ crossed)
RoundEnd ==
  /\ lPunch = "call"
  /\ lPunch' = "none" /\ crossed' = (crossed /\ rPunch)
  /\ LET c1 == [call EXCEPT !.rounds = @ + 1] IN
     \/ /\ punchable /\ LandOK /\ direct' = TRUE /\ Landed
        /\ UNCHANGED <<envc, autoL, rAuto, rPend, rPunch, rRest, budv, slotv, timev, cnt>>
        /\ React([c1 EXCEPT !.res = "success"], FALSE)
     \/ /\ CanFail \/ ~LandOK
        /\ call' = IF c1.rounds >= MaxRounds \/ call.stale THEN Ended(c1) ELSE [c1 EXCEPT !.res = "run"]
        /\ UNCHANGED <<envc, direct, autoL, rAuto, rPend, rPunch, rRest, pol, lastCls, budv, slotv, timev,
                       cnt, g>>

\* L's Dial to R's public announced addresses (WithForceDirectDial), through the gater.
DialStep ==
  /\ call.res = "run" /\ call.act = "dial"
  /\ IF Gater("ours")
       THEN \/ /\ punchable /\ LandOK /\ direct' = TRUE /\ SpendL(FALSE, TRUE, TRUE)
               /\ UNCHANGED <<envc, opv, slotv, timev, cnt>>
               /\ React([call EXCEPT !.res = "success", !.spent = TRUE], FALSE)
            \/ /\ ~punchable \/ Byz \/ ~LandOK
               /\ Spend(FALSE, TRUE)
               /\ call' = [call EXCEPT !.res = "spent", !.spent = TRUE]
               /\ UNCHANGED <<envc, direct, opv, pol, lastCls, slotv, timev, cnt>>
       ELSE /\ call' = Ended(call)
            /\ UNCHANGED <<envc, direct, opv, pol, lastCls, budv, slotv, timev, cnt, g>>

\* The actor takes the call's result (H-6, H-7, H-8, H-10).
Applied(res) ==
  LET nx == IF Target = "sched" THEN "sched" ELSE "idle" IN
  CASE res = "success" -> [st |-> "direct", wait |-> 0, j |-> 0, k |-> 0, ev |-> FALSE]
    [] ~Retry \/ (res = "spent" /\ StopAfter(pol.k)) -> [pol EXCEPT !.st = "rest", !.wait = 0]
    [] res = "spent" -> [st |-> nx, wait |-> IF nx = "sched" THEN Slow(pol.k) + Offset ELSE 0,
                         j |-> pol.j, k |-> IncTo(pol.k, KCap), ev |-> FALSE]
    [] OTHER -> [st |-> nx, wait |-> IF nx = "sched" THEN Fast(pol.j) + Offset ELSE 0,
                 j |-> IncTo(pol.j, JCap), k |-> pol.k, ev |-> FALSE]

Complete ==
  /\ call.act # "none" /\ call.res \notin {"run", "punch", "none"}
  /\ call' = NoCall /\ ncalls' = ncalls - 1
  /\ IF call.stale /\ ~ApplyStale
       THEN UNCHANGED <<pol, lastCls, g>>
       ELSE /\ pol' = Applied(call.res)
            /\ lastCls' = lastCls
            /\ g' = [g EXCEPT !.staleTouched = @ \/ (call.stale /\ pol' # pol)]
  /\ UNCHANGED <<envc, direct, opv, budv, timev, lease, queue, othersLeft, cnt>>

\* libp2p's one-shot at L: its direct dial first (as PreDial), then up to MaxRounds rounds. R holding no
\* grant, the gater asks one for it once, through the FIFO (Slots.Ask: a ticket at the tail, only the head is
\* served; no bypass): granted, the dial goes on under it (the grant is made in this step, the dial takes the
\* next); not granted, the dial is refused and the ticket kept (§3.3a, step 2).
AskNow == GaterWanted /\ TicketOK /\ lease["r"] = 0 /\ GaterBudget("auto")
AskGranted == AskNow /\ SlotFree /\ NodeTokOK /\ (queue = <<>> \/ GrantOrder("r"))
AutoPre ==
  /\ autoL = -2
  /\ CASE Gater("auto") ->
            \/ /\ punchable /\ LandOK /\ direct' = TRUE /\ autoL' = -1 /\ SpendL(FALSE, TRUE, TRUE)
               /\ UNCHANGED <<envc, lPunch, rAuto, rPend, rPunch, rRest, crossed, slotv, timev, cnt>>
               /\ React(call, FALSE)
            \/ /\ ~punchable \/ Byz \/ ~LandOK
               /\ autoL' = 0 /\ Spend(FALSE, TRUE)
               /\ UNCHANGED <<envc, direct, lPunch, rAuto, rPend, rPunch, rRest, crossed, pol, call,
                              lastCls, slotv, timev, cnt>>
       [] AskGranted ->
            /\ lease' = [lease EXCEPT !["r"] = TLease] /\ allow' = [allow EXCEPT !["r"] = LAllow]
            /\ queue' = Drop(queue, "r") /\ nTok' = Max(0, nTok - 1) /\ refN' = refN - 1
            /\ UNCHANGED <<envc, direct, opv, pol, call, lastCls, ncalls, othersLeft, attTok, refAtt, roundTok,
                           refRound, dTok, refDest, timev, cnt, g>>
       [] OTHER ->
            /\ autoL' = 0
            /\ queue' = IF AskNow /\ ~InQ("r") THEN Append(queue, "r") ELSE queue
            /\ UNCHANGED <<envc, direct, lPunch, rAuto, rPend, rPunch, rRest, crossed, pol, call, lastCls,
                           budv, ncalls, lease, othersLeft, timev, cnt, g>>
AutoStart ==
  /\ autoL >= 0 /\ lReady /\ lPunch = "none" /\ (lease["r"] > 0 \/ ~GateWanted)   \* a stranger: refused at once
  /\ RoundOn("auto")
  /\ LET acc == ~RoundGhost("auto") /\ chL = In("L") /\ circuits # {} /\ (rReady \/ Byz) IN
     \/ /\ ~acc \/ Byz \/ ~Gate("auto")
        /\ autoL' = -1
        /\ UNCHANGED <<env0, direct, lPunch, rAuto, rPend, rPunch, rRest, crossed, pol, call, lastCls,
                       budv, slotv, timev, cnt, g>>
     \/ /\ acc /\ Gate("auto") /\ lease["r"] > 0
        /\ IF Gater("auto")
             THEN /\ Spend(TRUE, TRUE) /\ lPunch' = "auto" /\ crossed' = (crossed \/ rPunch)
                  /\ UNCHANGED autoL
             ELSE /\ Spend(TRUE, FALSE) /\ UNCHANGED <<lPunch, crossed>>
                  /\ autoL' = IF autoL + 1 = MaxRounds THEN -1 ELSE autoL + 1
        /\ UNCHANGED <<env0, direct, rAuto, rPend, rPunch, rRest, pol, call, lastCls, slotv, timev, cnt>>
AutoEnd ==
  /\ lPunch = "auto"
  /\ lPunch' = "none" /\ crossed' = (crossed /\ rPunch)
  /\ \/ /\ punchable /\ LandOK /\ direct' = TRUE /\ autoL' = -1 /\ Landed
        /\ UNCHANGED <<envc, rAuto, rPend, rPunch, rRest, budv, slotv, timev, cnt>>
        /\ React(call, FALSE)
     \/ /\ CanFail \/ ~LandOK
        /\ autoL' = IF autoL + 1 >= MaxRounds THEN -1 ELSE autoL + 1
        /\ UNCHANGED <<envc, direct, rAuto, rPend, rPunch, rRest, pol, call, lastCls, budv, slotv, timev,
                       cnt, g>>

\* ---- R's exchanges: L answers; each waits in L's gate for R's slot, then L dials as A ----

\* An honest R starts an exchange (its upgrader under the same rule, or its one-shot), paced by time.
RArrive ==
  /\ ~Byz /\ RArriveOK
  /\ rPend' = TRUE /\ rAuto' = FALSE
  /\ UNCHANGED <<envc, direct, autoL, lPunch, rPunch, rRest, crossed, pol, call, lastCls, budv, slotv, timev,
                 cnt, g>>
\* A Byzantine R starts exchanges at will, on a circuit L sees as outbound (else L resets the stream).
ByzInit ==
  /\ Byz /\ ~rPend /\ ~rPunch /\ "LR" \in circuits /\ lReady
  /\ rPend' = TRUE
  /\ UNCHANGED <<envc, direct, autoL, lPunch, rAuto, rPunch, rRest, crossed, pol, call, lastCls, budv, slotv,
                 timev, cnt, g>>
RStart ==
  /\ rPend /\ (lease["r"] > 0 \/ ~GateWanted)                   \* a stranger: refused at once, no ticket
  /\ rPend' = FALSE
  /\ \/ /\ ~Gate("peer")
        /\ rRest' = TRUE
        /\ UNCHANGED <<rPunch, crossed, budv, g>>
     \/ /\ Gate("peer") /\ lease["r"] > 0 /\ Gater("peer")
        /\ Spend(TRUE, TRUE) /\ rPunch' = TRUE /\ crossed' = (crossed \/ lPunch # "none")
        /\ UNCHANGED rRest
     \/ /\ Gate("peer") /\ lease["r"] > 0 /\ (~Gater("peer") \/ Byz)   \* no destination, or no SYNC
        /\ Spend(TRUE, FALSE) /\ rRest' = TRUE
        /\ UNCHANGED <<rPunch, crossed>>
  /\ UNCHANGED <<envc, direct, autoL, lPunch, rAuto, pol, call, lastCls, slotv, timev, cnt>>
REnd ==
  /\ rPunch
  /\ rPunch' = FALSE /\ rRest' = TRUE /\ crossed' = (crossed /\ lPunch # "none")
  /\ \/ /\ punchable /\ LandOK /\ direct' = TRUE /\ Landed
        /\ UNCHANGED <<envc, autoL, lPunch, rAuto, rPend, budv, slotv, timev, cnt>>
        /\ React(call, FALSE)
     \/ /\ CanFail \/ ~LandOK
        /\ UNCHANGED <<envc, direct, autoL, lPunch, rAuto, rPend, pol, call, lastCls, budv, slotv, timev,
                       cnt, g>>

\* ---- Time ----
Tick ==
  /\ ~(lease["r"] = 1 /\ RWork)
  /\ clk' = (clk + 1) % APeriod
  /\ since' = IncTo(since, TMin)
  /\ pol' = IF pol.st = "sched" /\ pol.wait > 0 THEN [pol EXCEPT !.wait = @ - 1] ELSE pol
  /\ LET ra == clk' = 0
         rr == clk' % RPeriod = 0
         l2 == [p \in Peers |-> IF Expire /\ lease[p] > 0 THEN lease[p] - 1 ELSE lease[p]]
         lost == lease["r"] > 0 /\ l2["r"] = 0                  \* R's lease ends: D1 cancels what is left
     IN /\ attTok' = IF ra THEN IncTo(attTok, ACap) ELSE attTok
        /\ refAtt' = IF ra THEN Min(refAtt + 1, ACap) ELSE refAtt
        /\ nTok' = IF ra THEN IncTo(nTok, NCap) ELSE nTok
        /\ refN' = IF ra THEN Min(refN + 1, NCap) ELSE refN
        /\ roundTok' = IF rr THEN IncTo(roundTok, RCap) ELSE roundTok
        /\ refRound' = IF rr THEN Min(refRound + 1, RCap) ELSE refRound
        /\ dTok' = IF rr THEN IncTo(dTok, DCap) ELSE dTok
        /\ refDest' = IF rr THEN Min(refDest + 1, DCap) ELSE refDest
        /\ lease' = l2
        /\ allow' = [p \in Peers |-> IF l2[p] = 0 THEN 0 ELSE allow[p]]
        /\ call' = IF lost /\ call.res = "run" THEN Ended(call) ELSE call
        /\ autoL' = IF lost /\ autoL >= 0 THEN -1 ELSE autoL
        /\ rPend' = (rPend /\ ~lost)
        /\ rRest' = FALSE
  /\ UNCHANGED <<envc, direct, lPunch, rAuto, rPunch, crossed, lastCls, ncalls, queue, othersLeft, cnt, g>>

Next ==
  \/ \E d \in Dirs : Dial(d)
  \/ \E d \in Dirs : RCrash(d) \/ ProveAnswered(d) \/ ProveClose(d)
  \/ RRestart \/ RelayLost \/ DirectLost \/ WantFlip \/ Close \/ Open \/ Repick \/ Settle \/ Resolve
  \/ LReady \/ RReady \/ Learn \/ ByzToggle \/ Idle \/ Wake
  \/ RWait \/ LeaveQueue \/ RGiveUp \/ AutoGiveUp \/ \E o \in OPeers : OWait(o) \/ ODone(o) \/ ODial(o)
  \/ \E p \in Peers : Grant(p)
  \/ Launch \/ PreDial \/ RoundStart \/ RoundEnd \/ DialStep \/ Complete \/ AutoPre \/ AutoStart \/ AutoEnd
  \/ RArrive \/ ByzInit \/ RStart \/ REnd
  \/ Tick

\* Weakly fair: time, every step of a running call, one-shot or exchange, L's actor, identify, both handlers
\* registering, an honest R starting exchanges, Settle (unequal counts are transient), Resolve, the app
\* waking, the queue (joining, leaving, the semaphore's hand-over), the P-15 proofs (HP1c). Not fair: dials,
\* restarts, crashes, losses, wanted flips, closes, pick changes, idling, other peers asking for or giving back a
\* slot, a Byzantine R.
Fair ==
  /\ WF_vars(Tick) /\ WF_vars(Launch) /\ WF_vars(PreDial) /\ WF_vars(RoundStart) /\ WF_vars(RoundEnd)
  /\ WF_vars(DialStep) /\ WF_vars(Complete) /\ WF_vars(AutoPre) /\ WF_vars(AutoStart) /\ WF_vars(AutoEnd)
  /\ WF_vars(RArrive) /\ WF_vars(RStart) /\ WF_vars(REnd) /\ WF_vars(Learn) /\ WF_vars(LReady)
  /\ WF_vars(RReady) /\ WF_vars(Open) /\ WF_vars(Settle) /\ WF_vars(Resolve) /\ WF_vars(Wake)
  /\ WF_vars(RWait) /\ WF_vars(LeaveQueue) /\ \A p \in Peers : WF_vars(Grant(p))
  /\ IF ProveOlder THEN \A d \in Dirs : WF_vars(ProveAnswered(d)) /\ WF_vars(ProveClose(d)) ELSE TRUE   \* HP1c: P-15 ends
Spec == Init /\ [][Next]_vars /\ Fair

\* ---- Properties ----
TypeOK ==
  /\ circuits \subseteq Dirs /\ chL \in Dirs \cup {"none"} /\ chR \in Dirs \cup {"none"}
  /\ pol.st \in {"off", "idle", "sched", "inflight", "rest", "direct"}
  /\ pol.wait \in 0..(Slow(KCap) + Off) /\ pol.j \in 0..JCap /\ pol.k \in 0..KCap
  /\ autoL \in -2..(MaxRounds - 1) /\ lPunch \in {"none", "call", "auto"}
  /\ attTok \in 0..ACap /\ roundTok \in 0..RCap /\ dTok \in 0..DCap /\ nTok \in 0..NCap
  /\ \A p \in Peers : lease[p] \in 0..TLease /\ allow[p] \in 0..LAllow
  /\ cr.ghost \subseteq cr.pg /\ cr.pg \subseteq Dirs /\ cr.pl \subseteq circuits   \* HP1c: every ghost is under proof

OneCall == ncalls <= 1                                        \* S1, H-4
NoLaunchUnwanted == Fresh => wanted                           \* S2, H-1, H-14
NoLaunchDirect == Fresh => ~direct                            \* S3, H-14
RoleOK == Fresh => IF call.act = "punch" THEN In("L") \in circuits
                   ELSE circuits = {"LR"} /\ rPublic          \* S4, H-3
RoundBudget == refRound >= 0                                  \* S5, H-12: rounds, both roles, every origin
NoRoundUnwanted == ~g.unwantedRound                           \* S6, H-12
NoLaunchUnready == Fresh => lReady                            \* S7, H-2
StaleIgnored == ~g.staleTouched                               \* S8, H-6
GlobalCap == Cardinality({p \in Peers : lease[p] > 0}) <= K   \* S9, H-5: slots, every origin
AttemptBudget == refAtt >= 0                                  \* S10, H-11
BusyFree == attTok >= refAtt                                  \* S11, H-7
DestBudget == refDest >= 0                                    \* S12, H-12: non-relay destinations, every origin
NoDialUnwanted == ~g.unwantedDial                             \* S13, H-12: no direct path to a stranger
NoLadderCallIdle == ~g.idleCall                               \* S14, H-17: ladders sleep while idle
AdmissionBudget == refN >= 0                                  \* S15, H-16: node-wide admissions
NoUnallowedDial == ~g.unallowedDial                           \* S16, H-16: every destination, every origin, under a grant's allowance
NoTicketUnwanted == ~g.ticketUnwanted                         \* S17, H-1, H-5: a peer not wanted takes no ticket
LinkBound == ~g.lateLand                                      \* S18, H-19: no direct connection kept without a circuit
RoundOnLive == ~cr.ghostRound                                 \* S19, H-20: no round of L starts on a ghost
ProofKeepsLive == ~cr.liveClosed                              \* S20, P-15: a live circuit that answers is never closed
ResolveKeepsLive == ~cr.liveResolved                          \* S21, H-15, P-15: L never closes its live circuit for a ghost

\* L1: a wanted relayed link becomes direct, unless it stops being wanted or relayed.
Upgrades == (wanted /\ circuits # {} /\ ~direct) ~> (direct \/ ~wanted \/ circuits = {})
\* L2: waiting for a slot or Busy does not repeat forever, whatever the other peers do.
BusyEnds == []<>(lastCls \notin {"busy", "wait"})
\* L3 (H-15): two circuits do not last: the one the higher PeerID dialled is closed.
DupResolves == (circuits = Dirs) ~> (circuits # Dirs)
=============================================================================
