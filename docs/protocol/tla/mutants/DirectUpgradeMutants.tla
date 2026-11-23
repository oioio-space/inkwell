------------------------ MODULE DirectUpgradeMutants ------------------------
(* MUTANTS, WITNESSES and RESIDUALS (README.md, DirectUpgrade): broken rules *)
(* of plan docs/plans/2026-11-16-hp1-direct-upgrade.md (H-1..H-14) that the  *)
(* model must reject. Each mutant config overrides exactly one rule of       *)
(* DirectUpgrade ("Rule <- Mutant") with the constants of the configuration  *)
(* its README row names; the sound module is never edited.                  *)
EXTENDS DirectUpgrade

\* M1 (H-8): no retry, libp2p's one-shot behaviour: a failed call ends the upgrader for the epoch.
NoRetry == FALSE
\* M2 (H-6): a result of an older epoch is applied.
StaleApplied == TRUE
\* M3 (H-7): Busy takes an attempt token.
BusyTakesToken(a) == a - 1
\* M4 (H-1): no wanted check.
AnyPeer == TRUE
\* M5 (H-3): the side that only dialled punches too.
AnyCircuit(s) == circuits # {}
\* M6 (H-3, §3.4): only the lower PeerID initiates.
LowerOnly(s) == Lower(s)
\* M7 (H-9): a new circuit (epoch) refills both buckets.
RefillAttempts(a) == ACap
RefillRounds(r) == RCap
\* M8 (H-12): the round budget in the scheduler only: the gate lets libp2p's and the peer's rounds through.
SchedulerOnly(origin) == origin = "ours" => roundTok >= 1
\* M9 (H-2): launch before our handler is registered.
NoLocalReady == TRUE
\* M10 (H-10): a hopeless state: stop once the slow ladder reached its cap and failed again.
StopAtCap(k) == k = KCap
\* M11 (H-5): no node-wide cap.
NoCap == TRUE
\* M12 (H-14): launch while the session is direct.
IgnoreDirect == TRUE
\* M13 (H-4): launch while a call is in flight (after E7, before the old call's result).
AnyTime == TRUE
\* M14 (H-12): the gate does not check that the peer is wanted.
GateAnyPeer == TRUE
\* M15 (H-5): no deadline: a slot is held until its holder gives it back.
Never == FALSE
\* M17 (H-2): the old guard: Punch waits for remoteReady (a lost push leaves the only initiator NotReady).
GuardOn == TRUE
\* M18 (H-5): node-wide slots granted in any order: a peer that keeps calling takes every free slot.
AnyOrder(p) == TRUE
\* M19 (H-12): DirectConnect's first direct dial passes the gater without spending a destination.
PreDialFree(origin) == origin = "pre" \/ dTok >= 1
\* M20 (H-12): the gater lets a peer that is not wanted be dialled directly (libp2p's one-shot to a stranger).
GaterAnyPeer == TRUE
\* M21 (H-17): the ladders keep calling while the app is idle.
AlwaysActive == TRUE
\* M22 (H-17): no resume: once idle, ladder calls stay suspended after the app is active again.
StaySuspended == (active /\ cnt.idles = 0) \/ pol.ev
\* M23 (H-16): a slot granted without a node-wide admission token.
NoNodeBudget == TRUE
\* M24 (H-15 off): with two circuits both sides stay eligible (with CrossCancel: the old residual).
Disabled == FALSE
\* M25 (H-15): the duplicate circuit is never closed (ResolveOK <- Disabled).
\* M26 (H-16, B3): another peer's rounds sharing one grant dial beyond its allowance.
AnyAllow(o) == TRUE
\* M27 (H-16, B3): the gater lets a destination through without a grant (libp2p's first direct dial).
NoLeaseNeeded == TRUE
\* M28 (H-5, B4): a peer that is not wanted takes a ticket.
AnyTicket == TRUE
\* M29 (H-5, B6): a round that gave up waiting loses its ticket's place (re-registers at the tail).
AbandonDrops == ~RWants \/ ~wanted \/ circuits = {} \/ direct
\* M30 (H-19, block HP1r): a direct connection that lands after the last circuit is gone is kept.
LandAny == TRUE
\* M31 (H-20, block HP1c): L's round rides the connection libp2p picks, the older circuit (more streams), which is a
\* ghost after R's crash.
RoundAnyConn(origin) == In("L") \in cr.ghost
\* M32 (P-15, block HP1c): the proof's timer closes the older circuit whatever the answer.
CloseOnTimer == FALSE
\* M33 (H-15, P-15, block HP1c): L settles a duplicate without waiting for the P-15 proofs: a ghost is taken for
\* one of the two circuits, and the circuit H-15 keeps may be the dead one.
ResolveNoWait == TRUE

\* Witnesses (anti-vacuity): invariants the sound model must violate.
NoDirect == ~direct                                               \* a link becomes direct
NoStaleResult == ~(call.stale /\ call.res \notin {"run", "none"})  \* a result of an older epoch arrives
NoBusy == lastCls # "busy"                                         \* a call is Busy
NoSlowStep == pol.k = 0                                            \* the slow ladder is used (a Spent call)
NoWaitBehind == ~(InQ("r") /\ \E o \in OPeers : lease[o] > 0)      \* L waits for a slot another peer holds
NoCross == ~crossed                                                \* two punch phases of the pair overlap
NoDirectAfterRelayLost == ~(direct /\ circuits = {} /\ cnt.rlost > 0)  \* HP1r: an upgraded session outlives its circuit
NoRoundBesideGhost == ~cr.besideGhost                            \* HP1c: a round of L starts while a ghost shares its direction
NoProofAnswered == ~cr.answered                                    \* HP1c: an older live circuit answers its P-15 proof
NoProofClose == ~cr.proofClosed                                    \* HP1c: P-15 closes a ghost
NoGateRefusal == ~(Byz /\ "LR" \in circuits /\ lReady /\ wanted /\ roundTok = 0)  \* the gate's budget binds a flood
=============================================================================
