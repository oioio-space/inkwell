---------------------------- MODULE SendPath ----------------------------
(* The send path between two peers, end to end (W38,                        *)
(* docs/plans/2026-11-09-send-path-design.md §3-§4, "SP1: executable        *)
(* specification"). One sender (Alice) writes one object of S units to one *)
(* reader (Bob) over a relayed circuit, while she types. One unit = C = 8   *)
(* KiB, the credit quantum; one tick = the time the bottleneck needs for    *)
(* one unit (C / R). Time is a variable, with Lamport's upper-bound timers  *)
(* ("Real Time is Really Simple", 2005): Tick is disabled while an urgent   *)
(* step is enabled (a write that can proceed, a frame at the head of a      *)
(* queue, an honest read, an expired timer), so no timer can be overtaken.  *)
(*                                                                          *)
(* The path, in the order a byte travels:                                   *)
(*   stream window  yamux (or QUIC) stream window Wt of the object's stream,*)
(*                  reopened by the reader's updates in quanta Uq (yamux:   *)
(*                  Wt/2, go-yamux util.go GrowTo);                         *)
(*   sq             the session's send queue (yamux sendCh): a Write returns*)
(*                  once its frames are queued here;                        *)
(*   hop            Alice -> relay: the relay's receive window H on her     *)
(*                  circuit stream, reopened in quanta Uh as the relay      *)
(*                  forwards (QUIC: 25 %); the first hop is fast;           *)
(*   rb             the relay's forward buffer (bytes waiting for the       *)
(*                  bottleneck: RC3's bufferbloat);                         *)
(*   nq             the bottleneck queue (netem on the relay's eth0), of    *)
(*                  Qn data units, SHARED by both directions: Bob's window  *)
(*                  updates and credits wait in it behind Alice's data. Its *)
(*                  delay, Qn ticks when full, is the contract's D.         *)
(* Small frames (typed units, RST, announce, updates, credits) cost no      *)
(* service time but keep their place in every queue.                        *)
(*                                                                          *)
(* The muxer's liveness timers, each its own cut of the WHOLE session (every *)
(* stream, the interactive lane too: a connection loss, rule L for the      *)
(* object, an outage for the typing; E1 and E1b, 2026-11-09):               *)
(*   write valve  go-yamux Config.ConnectionWriteTimeout (10 s; session.go  *)
(*                sendLoop): the send loop has a frame and the hop has no   *)
(*                room for Tw ticks;                                        *)
(*   keepalive    every Ka ticks (KeepAliveInterval, 30 s) Alice pings; the *)
(*                ping must be flushed, then answered, each within Tk ticks *)
(*                (go-yamux uses ConnectionWriteTimeout for both: Ping and  *)
(*                keepalive). The ping is put at the head of the session    *)
(*                queue (yamux's best case), but on the wire it queues      *)
(*                behind every byte already in the relay; the pong queues   *)
(*                in the shared bottleneck. Bob's own ping is symmetric     *)
(*                (his pong to Alice queues behind her data) and omitted.   *)
(*                                                                          *)
(* The rule set is chosen by constants, so that the v2 target (rules SP-1.. *)
(* SP-8) and the v1 interim of block SP1 are checked against the SAME path: *)
(*   v2: Credit (SP-2), Measure = "consumed" (SP-1) with quantum P and timer *)
(*       Tp (SP-4), Call = 1 (C-sized writes, SP-7), Resume (SP-5);         *)
(*   v1: no credit, Measure = "write" (progress = a Write call returned,    *)
(*       timer Tp = WriteIdle per call), Call = one 32 KiB fragment, the    *)
(*       WB-2 total budget Total, no resumption, and Shared: a typed frame  *)
(*       waits for the fragment Write in progress (one stream, RC5).        *)
(* Loss handling is spec §5.5 rule L: a cut or a lost connection changes    *)
(* the epoch and tells the webview; the next epoch announces the object     *)
(* again, the reader answers with its offset (LARGE_REPLY, SP-5; v1: 0).    *)
(*                                                                          *)
(* Not modelled (README "SendPath"): the receiver-side SP-4 timer (a writer *)
(* that drips), several objects or lanes (bulk, REPO bursts: one object     *)
(* stream per session here), rate changes and cross traffic (the honest     *)
(* link is the contract's worst case: rate exactly F, queue full to D),     *)
(* yamux window auto-tuning (Wt is the window in effect, a constant), the   *)
(* transport's own retransmissions, Byzantine window updates, the rule I    *)
(* cadence, CONTROL (ConnCredit's ControlLive).                             *)
EXTENDS Naturals, Sequences, FiniteSets

CONSTANTS
  S,         \* object size, in units
  Wt,        \* stream window of the transport (units)
  Uq,        \* its update quantum (yamux Wt/2, QUIC Wt/4)
  H,         \* hop window: the relay's receive window on Alice's circuit stream
  Uh,        \* its update quantum
  Qn,        \* bottleneck queue, in data units (= the one-way delay D, in ticks)
  Tw,        \* muxer safety valve (ConnectionWriteTimeout), in ticks
  Credit,    \* TRUE: SP-2 receiver credit
  Win,       \* the window an honest receiver grants (SP-2), units
  Measure,   \* "consumed" (SP-1) | "write" (v1: the Write call returned)
  Call,      \* units per Write call (v2: 1 = C; v1: one 32 KiB fragment)
  P,         \* SP-4 progress quantum, units ("consumed" only)
  Tp,        \* SP-4 progress timer T_p; v1: WriteIdle, per Write call
  Total,     \* v1 WB-2 total budget per attempt, ticks; 0 = none (v2)
  Resume,    \* SP-5: resume at the reader's offset
  Shared,    \* v1: typed frames wait for the bulk Write call in progress
  Honest,    \* TRUE: the reader reads at once and tells the truth
  MaxLoss,   \* connection losses the environment may cause
  MaxEpoch,  \* attempts explored (epochs of the object)
  MaxForge,  \* forged credits a Byzantine reader may send
  MaxInt,    \* typed units
  Lmax,      \* InteractiveBound, ticks
  Tpart,     \* v1 receiver: a partial message is dropped after Tpart ticks without a
             \* fragment (mux partialTTL); 0 = none (v2 has no partial messages)
  Blk,       \* one silence of the bottleneck, ticks (blackout + the QUIC probe backoff
             \* that follows it, NI-2); 0 = none
  Ka,        \* muxer keepalive interval, ticks; 0 = keepalive disabled
  Tk         \* muxer ping timeouts (flush, then answer), ticks

ASSUME /\ S >= 1 /\ Wt >= 1 /\ Uq \in 1..Wt /\ H >= 1 /\ Uh \in 1..H /\ Qn >= 1
       /\ Tw >= 1 /\ Win >= 1 /\ Measure \in {"consumed", "write"} /\ Call >= 1
       /\ P >= 1 /\ Tp >= 1 /\ Total \in Nat /\ MaxEpoch >= 1
       /\ Credit \in BOOLEAN /\ Resume \in BOOLEAN /\ Shared \in BOOLEAN /\ Honest \in BOOLEAN
       /\ Tpart \in Nat /\ Blk \in Nat /\ Ka \in Nat /\ Tk >= 1

VARIABLES
  sq, rb, nq,                                        \* queues (see the header)
  st, sent, tLim, cSeen, armed, pt, el, inCall, att, \* sender, current attempt
  hcred, hfwd, mt,                                   \* hop credit, forwarded since its last update, valve
  got, rcons, lastU, ropen,                          \* reader
  ist, iage, nI,                                     \* the typed unit
  cuts, losses, tears, notices, totSent, forged, sAge, \* history (ghosts)
  bo, boUsed, gap, rdrops,                           \* blackout left, used; reader's partial: idle, drops
  ka, pst, page, kfails                              \* keepalive: since last pong, ping state, its age, failures

qv == <<sq, rb, nq>>
sv == <<st, sent, tLim, cSeen, armed, pt, el, inCall, att>>
hv == <<hcred, hfwd, mt>>
rv == <<got, rcons, lastU, ropen>>
iv == <<ist, iage, nI>>
gv == <<cuts, losses, tears, notices, totSent, forged, sAge>>
bv == <<bo, boUsed, gap, rdrops>>
kv == <<ka, pst, page, kfails>>
vars == <<qv, sv, hv, rv, iv, gv, bv, kv>>

Min(a, b) == IF a < b THEN a ELSE b
Max(a, b) == IF a > b THEN a ELSE b
IsData(x) == x[1] = "d"
NData(s) == Cardinality({i \in 1..Len(s) : s[i][1] = "d"})
Frame(k) == <<k, 0>>

\* Bounds the properties promise (README "SendPath").
HoldMax == IF Total > 0 THEN Total ELSE (S \div P + 1) * Tp   \* SP-4 (v1: WB-2)
StopMax == Tp + Qn                                            \* SP-4: last progress + its report
CapT == Tp + 1
CapE == HoldMax + 1
CapS == StopMax + 1
CapI == Lmax + 1
CapW == Tw + 1
CapK == Tk + 1

\* ---- The rule (each name below is what a mutant of SendPathMutants overrides) ----
Quantum == P                                     \* SP-4 quantum
CreditOK(v) == v <= sent                         \* SP-8 #large-credit-beyond-sent
TotalBound == Total                              \* v1 WB-2
Expired == pt >= Tp \/ (TotalBound > 0 /\ el >= TotalBound)   \* SP-4 (v1: WB-1, WB-2)
RuleL == TRUE                                    \* §5.5: a loss changes the epoch, the webview is told

delivered == rcons = S
\* Something is under way (the keepalive alone does not keep time running).
Busy0 == st \in {"wait", "open"} \/ sq # <<>> \/ rb # <<>> \/ nq # <<>> \/ ist \in {"local", "net"}

Init == /\ sq = <<>> /\ rb = <<>> /\ nq = <<>>
        /\ st = "open" /\ sent = 0 /\ tLim = Wt /\ cSeen = 0 /\ armed = 0 /\ pt = 0 /\ el = 0
        /\ inCall = FALSE /\ att = 1
        /\ hcred = H /\ hfwd = 0 /\ mt = 0
        /\ got = 0 /\ rcons = 0 /\ lastU = 0 /\ ropen = TRUE
        /\ ist = "none" /\ iage = 0 /\ nI = 0
        /\ cuts = 0 /\ losses = 0 /\ tears = 0 /\ notices = 0 /\ totSent = 0 /\ forged = 0 /\ sAge = 0
        /\ bo = 0 /\ boUsed = FALSE /\ gap = 0 /\ rdrops = 0
        /\ ka \in 0..Ka /\ pst = "idle" /\ page = 0 /\ kfails = 0   \* the keepalive in any phase

\* ---------------------------- Sender (Alice) ----------------------------
\* A Write call starts: v2 only within the credit (SP-2, SP-7); v1 at once. A
\* typed frame waiting for the mux gate goes first (interactive priority).
CanStart == /\ st = "open" /\ ~inCall /\ sent < S
            /\ ~Credit \/ sent < cSeen + Win
            /\ ~(Shared /\ ist = "local")
StartCall == /\ CanStart /\ inCall' = TRUE /\ UNCHANGED <<bv, kv>>
             /\ UNCHANGED <<qv, st, sent, tLim, cSeen, armed, pt, el, att, hv, rv, iv, gv>>

\* One unit of the call passes the stream window into the session queue. The call
\* returns with its last unit: v1 progress (Measure "write") re-arms the timer then.
CanWrite == st = "open" /\ inCall /\ sent < tLim
Write == /\ CanWrite /\ UNCHANGED <<bv, kv>>
         /\ LET ret == (sent + 1) % Call = 0 \/ sent + 1 = S
                wr  == Measure = "write" /\ ret
            IN /\ sent' = sent + 1
               /\ totSent' = totSent + 1
               /\ sq' = Append(sq, Frame("d"))
               /\ inCall' = ~ret
               /\ pt' = IF wr THEN 0 ELSE pt
               /\ armed' = IF wr THEN sent + 1 ELSE armed
               /\ st' = IF Measure = "write" /\ sent + 1 = S THEN "done" ELSE st
         /\ UNCHANGED <<rb, nq, tLim, cSeen, el, att, hv, rv, iv, cuts, losses, tears, notices, forged, sAge>>

\* A typed frame enters the session queue. v2: its own stream, never paced by the
\* object. v1 (Shared): one stream, so it waits for the Write call in progress and
\* for one byte of that stream's window.
IntFree == ~Shared \/ (~inCall /\ (st # "open" \/ sent < tLim))
CanInt == ist = "local" /\ IntFree
SendInt == /\ CanInt /\ sq' = Append(sq, Frame("i")) /\ ist' = "net" /\ UNCHANGED <<bv, kv>>
           /\ UNCHANGED <<rb, nq, sv, hv, rv, iage, nI, gv>>

\* The session's send loop writes its head frame into the hop, if the hop has room.
CanPush == sq # <<>> /\ hcred > 0
Push == /\ CanPush /\ UNCHANGED bv
        /\ rb' = Append(rb, Head(sq)) /\ sq' = Tail(sq)
        /\ hcred' = IF IsData(Head(sq)) THEN hcred - 1 ELSE hcred
        /\ mt' = 0
        /\ IF Head(sq)[1] = "p" THEN pst' = "sent" /\ page' = 0 ELSE UNCHANGED <<pst, page>>
        /\ UNCHANGED <<nq, sv, hfwd, rv, iv, gv, ka, kfails>>

\* SP-4 cut (v1: WriteIdle per call, WB-2 total): the stream is reset in band (the
\* RST queues behind the data already written: RC3). Rule L: new epoch, webview told,
\* the object announced again.
CanCut == st = "open" /\ Expired
Cut == /\ CanCut /\ UNCHANGED <<bv, kv>>
       /\ cuts' = cuts + 1 /\ inCall' = FALSE
       /\ IF RuleL
            THEN /\ notices' = notices + 1
                 /\ IF att < MaxEpoch
                      THEN st' = "wait" /\ sq' = sq \o <<Frame("rst"), Frame("a")>>
                      ELSE st' = "over" /\ sq' = Append(sq, Frame("rst"))
            ELSE /\ notices' = notices                \* the object is silently dropped
                 /\ st' = "over" /\ sq' = Append(sq, Frame("rst"))
       /\ UNCHANGED <<rb, nq, sent, tLim, cSeen, armed, pt, el, att, hv, rv, iv,
                      losses, tears, totSent, forged, sAge>>

\* The session is gone (environment loss, or the muxer's valve): every queue and
\* stream is dropped, a typed frame in flight is sent again on the next session,
\* and rule L applies to the object (unless it was already delivered).
\* The webview is always told (PeerDown, then PeerUp on the next session). An object
\* that is finished stays so; v1's "done" (last Write returned) is not finished.
Finished == delivered \/ st = "over" \/ (st = "done" /\ Measure = "consumed")
Reconnect ==
  /\ UNCHANGED bv
  /\ ka' = 0 /\ pst' = "idle" /\ page' = 0
  /\ LET nst == IF Finished THEN st ELSE IF att < MaxEpoch THEN "wait" ELSE "over"
     IN /\ st' = nst
        /\ sq' = IF nst = "wait" THEN <<Frame("a")>> ELSE <<>>
  /\ rb' = <<>> /\ nq' = <<>>
  /\ notices' = notices + 1
  /\ inCall' = FALSE
  /\ hcred' = H /\ hfwd' = 0 /\ mt' = 0
  /\ ropen' = FALSE
  /\ ist' = IF ist = "net" THEN "local" ELSE ist
  /\ UNCHANGED <<sent, tLim, cSeen, armed, pt, el, att, got, rcons, lastU, iage, nI,
                 cuts, totSent, forged, sAge>>

\* Muxer valve: the send loop has been blocked on the hop for Tw ticks.
CanTear == mt >= Tw
Teardown == /\ CanTear /\ tears' = tears + 1 /\ Reconnect /\ UNCHANGED <<losses, kfails>>

\* Keepalive: the ping is due (urgent: a timer); its flush or its answer is late.
CanPing == Ka > 0 /\ pst = "idle" /\ ka >= Ka /\ Busy0
PingFire == /\ CanPing
            /\ sq' = <<Frame("p")>> \o sq /\ pst' = "queued" /\ page' = 0
            /\ UNCHANGED <<rb, nq, sv, hv, rv, iv, gv, bv, ka, kfails>>
CanKaFail == pst # "idle" /\ page >= Tk
KaTear == /\ CanKaFail /\ kfails' = kfails + 1 /\ Reconnect /\ UNCHANGED <<losses, tears>>

\* Environment: the connection is lost (no fairness; at most MaxLoss times). Not while
\* a typed unit is pending: its latency is measured on a live connection (the valve's
\* teardown, a cut of the path's own making, is not excluded).
Lose == /\ losses < MaxLoss /\ st \in {"wait", "open"} /\ ~delivered /\ ist \notin {"local", "net"}
        /\ losses' = losses + 1 /\ Reconnect /\ UNCHANGED <<tears, kfails>>

\* ---------------------------- Relay ----------------------------
\* The relay moves its head frame into the bottleneck queue when it has room for
\* data; forwarding data reopens the hop in quanta Uh (hop window H).
CanAdmit == rb # <<>> /\ (~IsData(Head(rb)) \/ NData(nq) < Qn)
Admit == /\ CanAdmit /\ UNCHANGED <<bv, kv>>
         /\ nq' = Append(nq, Head(rb)) /\ rb' = Tail(rb)
         /\ IF IsData(Head(rb)) /\ hfwd + 1 >= Uh
              THEN hcred' = H - NData(Tail(rb)) /\ hfwd' = 0
              ELSE /\ hcred' = hcred
                   /\ hfwd' = IF IsData(Head(rb)) THEN hfwd + 1 ELSE hfwd
         /\ UNCHANGED <<sq, sv, mt, rv, iv, gv>>

\* ------------------- Small frames at the head of the bottleneck -------------------
CtlHead(k) == bo = 0 /\ nq # <<>> /\ Head(nq)[1] = k
HeadV == Head(nq)[2]

ArriveI == /\ CtlHead("i") /\ nq' = Tail(nq) /\ ist' = "done" /\ UNCHANGED <<bv, kv>>
           /\ UNCHANGED <<sq, rb, sv, hv, rv, iage, nI, gv>>
ArriveRst == /\ CtlHead("rst") /\ nq' = Tail(nq) /\ ropen' = FALSE /\ UNCHANGED <<bv, kv>>
             /\ UNCHANGED <<sq, rb, sv, hv, got, rcons, lastU, iv, gv>>
\* The reader answers an announce (LARGE_REPLY) with its offset: SP-5 its prefix
\* (a Byzantine reader may state less), v1 0 (the partial message is dropped).
Offsets == IF ~Resume THEN {0} ELSE IF Honest THEN {got} ELSE 0..got
ArriveA == /\ CtlHead("a") /\ UNCHANGED <<bv, kv>>
           /\ \E off \in Offsets :
                /\ nq' = Append(Tail(nq), <<"r", off>>)
                /\ got' = off /\ rcons' = Min(rcons, off) /\ lastU' = Min(rcons, off)
           /\ ropen' = TRUE
           /\ UNCHANGED <<sq, rb, sv, hv, iv, gv>>
\* A stream window update (only the current stream's count: older ones arrive
\* before the LARGE_REPLY that opens the next stream, so st = "wait" drops them).
ArriveU == /\ CtlHead("u") /\ nq' = Tail(nq) /\ UNCHANGED <<bv, kv>>
           /\ tLim' = IF st = "open" THEN Max(tLim, HeadV + Wt) ELSE tLim
           /\ UNCHANGED <<sq, rb, st, sent, cSeen, armed, pt, el, inCall, att, hv, rv, iv, gv>>
\* SP-2 credit: monotone, never beyond what was sent (SP-8); SP-1/SP-4: each P
\* units of reported consumption re-arm the timer.
ArriveC == /\ CtlHead("c") /\ nq' = Tail(nq) /\ UNCHANGED <<bv, kv>>
           /\ LET v  == HeadV
                  ok == st = "open" /\ Credit /\ CreditOK(v) /\ v > cSeen
                  re == ok /\ Measure = "consumed" /\ v - armed >= Quantum
              IN /\ cSeen' = IF ok THEN v ELSE cSeen
                 /\ armed' = IF re THEN v ELSE armed
                 /\ pt' = IF re THEN 0 ELSE pt
                 /\ st' = IF ok /\ v >= S THEN "done" ELSE st
           /\ UNCHANGED <<sq, rb, sent, tLim, el, inCall, att, hv, rv, iv, gv>>
\* LARGE_REPLY: the next attempt opens at the stated offset, with a fresh stream.
ArriveR == /\ CtlHead("r") /\ nq' = Tail(nq) /\ UNCHANGED <<bv, kv>>
           /\ IF st = "wait"
                THEN LET off == HeadV IN
                     /\ st' = IF off >= S THEN "done" ELSE "open"
                     /\ sent' = off /\ cSeen' = off /\ tLim' = off + Wt /\ armed' = off
                     /\ pt' = 0 /\ el' = 0 /\ inCall' = FALSE /\ att' = att + 1 /\ sAge' = 0
                ELSE UNCHANGED <<sv, sAge>>
           /\ UNCHANGED <<sq, rb, hv, rv, iv, cuts, losses, tears, notices, totSent, forged>>

\* Bob's muxer answers a ping at once; the pong joins the shared bottleneck queue.
ArriveP == /\ CtlHead("p") /\ nq' = Append(Tail(nq), Frame("q"))
           /\ UNCHANGED <<sq, rb, sv, hv, rv, iv, gv, bv, kv>>
ArriveQ == /\ CtlHead("q") /\ nq' = Tail(nq)
           /\ ka' = 0 /\ pst' = "idle" /\ page' = 0
           /\ UNCHANGED <<sq, rb, sv, hv, rv, iv, gv, bv, kfails>>

CanCtl == bo = 0 /\ nq # <<>> /\ ~IsData(Head(nq))
ServeCtl == ArriveI \/ ArriveRst \/ ArriveA \/ ArriveU \/ ArriveC \/ ArriveR \/ ArriveP \/ ArriveQ

\* ---------------------------- Reader (Bob) ----------------------------
\* The application reads one unit; the transport then reports it (window update
\* once Uq units are read since the last one, written first), and so does SP-2
\* (one credit per C). An honest reader reads at once; any other one when it likes.
Consume == /\ rcons < got /\ UNCHANGED <<bv, kv>>
           /\ LET c  == rcons + 1
                  up == c - lastU >= Uq
              IN /\ rcons' = c
                 /\ lastU' = IF up THEN c ELSE lastU
                 /\ nq' = nq \o (IF up THEN <<<<"u", c>>>> ELSE <<>>)
                             \o (IF Credit THEN <<<<"c", c>>>> ELSE <<>>)
           /\ sAge' = 0
           /\ UNCHANGED <<sq, rb, sv, hv, got, ropen, iv, cuts, losses, tears, notices, totSent, forged>>

\* A Byzantine reader forges a credit (any value).
Forge == /\ ~Honest /\ forged < MaxForge /\ UNCHANGED <<bv, kv>>
         /\ \E v \in 0..S : nq' = Append(nq, <<"c", v>>)
         /\ forged' = forged + 1 /\ sAge' = 0
         /\ UNCHANGED <<sq, rb, sv, hv, rv, iv, cuts, losses, tears, notices, totSent>>

\* v1 reader: a partial message without a new fragment for Tpart ticks is dropped
\* (mux partialTTL), and so are its continuations (fragment.go reassemble: « the
\* continuation of a message we dropped »). Nothing tells the sender.
Partial == Tpart > 0 /\ ropen /\ 0 < got /\ got < S
CanDrop == Partial /\ gap >= Tpart
PartialDrop == /\ CanDrop
               /\ got' = 0 /\ rcons' = 0 /\ ropen' = FALSE /\ gap' = 0 /\ rdrops' = rdrops + 1
               /\ UNCHANGED <<qv, sv, hv, lastU, iv, gv, bo, boUsed, kv>>

\* Environment: the bottleneck goes silent for Blk ticks, once, during the transfer.
Blackout == /\ Blk > 0 /\ ~boUsed /\ st = "open"
            /\ bo' = Blk /\ boUsed' = TRUE
            /\ UNCHANGED <<qv, sv, hv, rv, iv, gv, gap, rdrops, kv>>

\* ---------------------------- Typing ----------------------------
Type == /\ ist = "none" /\ nI < MaxInt /\ ~delivered /\ st # "over" /\ UNCHANGED <<bv, kv>>
        /\ ist' = "local" /\ iage' = 0 /\ nI' = nI + 1
        /\ UNCHANGED <<qv, sv, hv, rv, gv>>

\* ---------------------------- Time ----------------------------
Urgent == \/ CanStart \/ CanWrite \/ CanInt \/ CanPush \/ CanAdmit \/ CanCtl
          \/ (Honest /\ rcons < got) \/ CanCut \/ CanTear \/ CanDrop \/ CanPing \/ CanKaFail
Busy == Busy0 \/ pst # "idle"

\* One tick: the bottleneck delivers its head data unit; timers advance.
Tick == /\ Busy /\ ~Urgent
        /\ IF bo = 0 /\ nq # <<>> /\ IsData(Head(nq))
             THEN /\ nq' = Tail(nq) /\ got' = IF ropen THEN got + 1 ELSE got
                  /\ gap' = 0
             ELSE /\ UNCHANGED <<nq, got>>
                  /\ gap' = IF Partial THEN Min(gap + 1, Tpart) ELSE 0
        /\ bo' = IF bo > 0 THEN bo - 1 ELSE 0
        /\ ka' = IF Ka > 0 /\ pst = "idle" THEN Min(ka + 1, Ka) ELSE ka
        /\ page' = IF pst # "idle" THEN Min(page + 1, CapK) ELSE 0
        /\ pt' = IF st = "open" THEN Min(pt + 1, CapT) ELSE pt
        /\ el' = IF st = "open" THEN Min(el + 1, CapE) ELSE el
        /\ sAge' = IF st = "open" THEN Min(sAge + 1, CapS) ELSE sAge
        /\ iage' = IF ist \in {"local", "net"} THEN Min(iage + 1, CapI) ELSE iage
        /\ mt' = IF sq # <<>> THEN Min(mt + 1, CapW) ELSE 0   \* not Urgent: sq blocked on the hop
        /\ UNCHANGED <<sq, rb, st, sent, tLim, cSeen, armed, inCall, att, hcred, hfwd,
                       rcons, lastU, ropen, ist, nI, cuts, losses, tears, notices, totSent, forged,
                       boUsed, rdrops, pst, kfails>>

Next == \/ StartCall \/ Write \/ SendInt \/ Push \/ Cut \/ Teardown \/ Lose \/ PingFire \/ KaTear
        \/ Admit \/ ServeCtl \/ Consume \/ Forge \/ Type \/ Tick \/ PartialDrop \/ Blackout

\* Fairness on time and on every urgent step; none on a Byzantine reader's reads,
\* on typing, on losses or on forging.
Spec == /\ Init /\ [][Next]_vars
        /\ WF_vars(Tick) /\ WF_vars(StartCall) /\ WF_vars(Write) /\ WF_vars(SendInt)
        /\ WF_vars(Push) /\ WF_vars(Cut) /\ WF_vars(Teardown) /\ WF_vars(Admit)
        /\ WF_vars(ServeCtl) /\ WF_vars(Honest /\ Consume) /\ WF_vars(PartialDrop)
        /\ WF_vars(PingFire) /\ WF_vars(KaTear)

TypeOK == /\ st \in {"wait", "open", "done", "over"} /\ sent \in 0..S /\ cSeen \in 0..S
          /\ got \in 0..S /\ rcons \in 0..got /\ ist \in {"none", "local", "net", "done"}
          /\ pt \in 0..CapT /\ el \in 0..CapE /\ sAge \in 0..CapS /\ iage \in 0..CapI
          /\ mt \in 0..CapW /\ hcred \in 0..H /\ att \in 1..MaxEpoch
          /\ pst \in {"idle", "queued", "sent"} /\ page \in 0..CapK

\* ---- Properties (README "SendPath"; design §4) ----
\* SP-4 derivation, SP-6: an honest link inside the contract is never cut.
NoFalseCut == cuts = 0
\* SP-3 extended to the hop, and SP-9: an honest link inside the contract never loses
\* its session to the muxer's own timers (write valve, keepalive).
NoSessionLoss == tears = 0 /\ kfails = 0
\* SP-3: the object's Write is never blocked by the transport window.
WindowNeverBinds == ~(st = "open" /\ inCall /\ sent >= tLim)
\* SP-7: a typed unit reaches the reader within Lmax ticks.
InteractiveBound == iage <= Lmax
\* SP-4: a reader that stops (reads and reports nothing) is cut within T_p of its
\* last report's arrival.
StopCut == st = "open" => sAge <= StopMax
\* SP-4 (v1: WB-2): whatever the reader does, one attempt holds the lane at most HoldMax.
HoldBound == el <= HoldMax
\* SP-5, SP-6: each lost attempt costs at most one window of resent units.
BoundedWaste == totSent <= S + (cuts + losses + tears + kfails) * Win
\* SP-8: the sender never believes more was consumed than it sent.
CreditSane == cSeen <= sent
\* §5.5 rule L: every lost attempt changed the epoch and told the webview.
NoSilentLoss == notices = cuts + losses + tears + kfails
\* SP-6 (liveness): honest link => delivered, in one attempt per connection loss.
DeliveredHonest == <>(delivered /\ att <= 1 + losses + tears + kfails)
\* §5.5 rule L, v1 (block SP1): the reader never drops a partial message that the
\* sender goes on writing without telling anyone (a silent loss).
NoSilentDrop == rdrops <= notices
\* SP-4 (liveness): whatever the reader does, every open attempt ends.
CutEventually == [](st = "open" => <>(st # "open"))
=============================================================================
