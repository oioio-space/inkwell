---------------------------- MODULE WriteBudget ----------------------------
(* Block WB (docs/plans/2026-11-08-netsim-impaired-plan.md, "Block WB:      *)
(* executable specification"): the write budget of one large message sent  *)
(* to one peer on sync/1 (mux.sendFragmented over frame.WriteProgress).     *)
(*                                                                          *)
(* Time is an ordinary variable, advanced by Tick, with Lamport's upper-    *)
(* bound timers ("Real Time is Really Simple", 2005): Tick is disabled once *)
(* a deadline has expired, so the cut (Fail) happens before time can pass   *)
(* it. A failed write loses the message: spec §5.5 rule L resyncs the peer  *)
(* and the message is sent again from its first byte (sent := 0).          *)
(*                                                                          *)
(* Rule WB (one attempt of a message of S units):                           *)
(*   WB-1 idle bound : the write fails after Idle ticks without progress;  *)
(*   WB-2 total bound: it fails once elapsed >= Idle + S / Floor, Floor     *)
(*        being FloorRate, the slowest link the rule promises to complete.  *)
(*                                                                          *)
(* Scale-free abstraction: one tick = WriteIdle / Idle, one unit = Floor x  *)
(* one tick. An honest link guarantees sustained progress >= Floor (a token *)
(* bucket: sent + Burst >= Floor * elapsed) and never stalls for Idle      *)
(* ticks. A dishonest reader (Honest = FALSE) takes any 0..MaxRate units   *)
(* per tick (it may drip just inside the idle bound), and may stop reading *)
(* forever (Stop, no fairness, as ConnCredit's Stall): weak fairness on     *)
(* Tick is fairness on time, never a promise that such a reader reads.      *)
(* Not modelled: the interactive frames interleaved between fragments (the  *)
(* mux guarantees fragments one frame in four, which FloorRate accounts     *)
(* for), the rule I cadence (it delays a resend, never prevents one),      *)
(* several peers (one sender per peer, each with its own budget).          *)
EXTENDS Naturals

CONSTANTS S,          \* message size, in units
          Idle,       \* WriteIdle, in ticks
          Floor,      \* FloorRate (units / tick): the slowest link promised, = epsilon
          MaxRate,    \* fastest progress per tick the link allows
          Burst,      \* token-bucket slack of an honest link (units)
          MaxMsg,     \* messages sent in one behaviour
          Honest      \* TRUE: sustained progress >= Floor; FALSE: any reader

VARIABLES msg,      \* messages delivered so far
          sent,     \* units of the current attempt written so far
          elapsed,  \* ticks since the current attempt started (saturated at Cap)
          idle,     \* ticks since the last progress (saturated at Cap)
          stopped   \* a dishonest reader stopped reading, for good

vars == <<msg, sent, elapsed, idle, stopped>>

\* An honest link at the floor rate fits the floor budget.
ASSUME /\ Floor >= 1 /\ Idle >= 2 /\ MaxRate >= Floor /\ Burst <= Floor * Idle

Min(a, b) == IF a < b THEN a ELSE b
CeilDiv(a, b) == (a + b - 1) \div b

\* The hold the spec promises against any reader (property HoldBound).
HoldMax == Idle + CeilDiv(S, Floor)
Cap == HoldMax + 1

\* ---- The rule (each name below is what a mutant overrides) ----
Limit == Idle + CeilDiv(S, Floor)                         \* WB-2
IdleExpired == idle >= Idle                               \* WB-1
TotalExpired == elapsed >= Limit                          \* WB-2
Expired == IdleExpired \/ TotalExpired

Init == msg = 0 /\ sent = 0 /\ elapsed = 0 /\ idle = 0 /\ stopped = FALSE

\* The reader may take n units this tick. An honest link keeps a sustained
\* rate >= Floor (token bucket) and never stalls for Idle ticks (WB-1's live link).
\* A stopped reader takes nothing.
Allowed(n) == \/ stopped /\ n = 0
              \/ ~stopped /\ ~Honest
              \/ /\ ~stopped /\ Honest
                 /\ sent + n + Burst >= Floor * (elapsed + 1)
                 /\ n > 0 \/ idle + 1 < Idle

Tick == /\ msg < MaxMsg /\ sent < S
        /\ ~Expired                       \* urgency: an expired deadline fires first
        /\ \E n \in 0..Min(MaxRate, S - sent):
             /\ Allowed(n)
             /\ sent' = sent + n
             /\ idle' = IF n > 0 THEN 0 ELSE Min(idle + 1, Cap)
        /\ elapsed' = Min(elapsed + 1, Cap)
        /\ UNCHANGED <<msg, stopped>>

\* A dishonest reader stops reading for good (no fairness: it may never happen).
Stop == /\ ~Honest /\ ~stopped /\ stopped' = TRUE
        /\ UNCHANGED <<msg, sent, elapsed, idle>>

\* The write is given up: the message is lost, rule L resends it from the start.
Fail == /\ msg < MaxMsg /\ sent < S /\ Expired
        /\ sent' = 0 /\ elapsed' = 0 /\ idle' = 0
        /\ UNCHANGED <<msg, stopped>>

Deliver == /\ msg < MaxMsg /\ sent = S
           /\ msg' = msg + 1
           /\ sent' = 0 /\ elapsed' = 0 /\ idle' = 0
           /\ UNCHANGED stopped

Next == Tick \/ Fail \/ Deliver \/ Stop

\* Time advances (Tick), an expired timer fires (Fail), a written message completes (Deliver).
Spec == Init /\ [][Next]_vars /\ WF_vars(Tick) /\ WF_vars(Fail) /\ WF_vars(Deliver)

TypeOK == /\ msg \in 0..MaxMsg /\ sent \in 0..S /\ elapsed \in 0..Cap /\ idle \in 0..Cap
          /\ stopped \in BOOLEAN /\ (Honest => ~stopped)

\* ---- Properties (README "WriteBudget") ----
\* WB-1: a reader that stops reading is cut within Idle ticks.
StopCut == idle <= Idle
\* WB-2: whatever the reader does, one attempt holds the sender at most Idle + S / Floor.
HoldBound == elapsed <= HoldMax
\* WB-1/WB-2, liveness form: whatever the reader does, every started attempt ends
\* (delivered or cut): the sender is never held forever.
CutEventually == [](elapsed > 0 => <>(elapsed = 0))
\* No resend loop (honest link, sustained progress >= Floor): every message is delivered.
Delivered == <>(msg = MaxMsg)
=============================================================================
