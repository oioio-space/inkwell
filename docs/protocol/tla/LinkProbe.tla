---------------------------- MODULE LinkProbe ----------------------------
(* Spec §8.5, /inkwell/linkprobe/1.0.0 (ADR 0036): one direction of one QUIC   *)
(* connection, over several probe streams. The prober P sends data packets and  *)
(* PROBEs on its stream; the responder R counts the packets of P it receives    *)
(* and answers PROBEs from a credit kept per connection, refuses streams after  *)
(* a violation or too soon after the previous one, and bounds the PROBE rate.   *)
(* P keeps outstanding PROBEs and accepted samples and computes the figure.     *)
(*                                                                              *)
(* Time: ages in ticks (one tick = E = 500 ms, the cost of one echo, the        *)
(* fastest PROBE rate), each capped where larger values change nothing, so the  *)
(* state space is finite while time is unbounded. Honest = TRUE: R answers      *)
(* truthfully; the network delivers every item within D ticks except during a   *)
(* stall (everything held, then a burst); data packets may be lost or overtake  *)
(* PROBEs. Honest = FALSE: R is Byzantine and the network has no deadline.      *)
(* Greedy = TRUE: P ignores the spacing and the backoff, and tries to open a    *)
(* second stream while one is open (a flooding peer). AccDelay > 0: R accepts  *)
(* a stream up to AccDelay ticks after P opened it and started to write.       *)
(*                                                                              *)
(* The rules of §8.5.5 and §8.5.6 are named operators, so that each mutant      *)
(* overrides exactly one of them (mutants/LinkProbeMutants.tla).                *)
EXTENDS Naturals, Integers, Sequences

CONSTANTS
  MaxProbes,   \* PROBEs per stream
  MaxStreams,  \* probe streams per connection
  MaxData,     \* data packets per connection
  MaxOut,      \* MaxOutstanding (§8.5.7), scaled
  MaxSamples,  \* MaxSamples, scaled
  PTO,         \* the late bound, in ticks
  W,           \* Window, in ticks
  NMin,        \* MinPackets, scaled
  D,           \* honest network out of a stall: delivered within D ticks
  B,           \* Backoff, in ticks
  Gap,         \* the responder's refusal after a stream's end (B/2), in ticks
  CreditMax,   \* the responder's credit, in echoes (1500 ms / 500 ms = 3 in the spec)
  AccDelay,    \* ticks the responder may take to accept a stream after P opened it (review 3, F6)
  DropMax,     \* seq above the rate bound by at most this: dropped without fault (SeqDropMax, scaled)
  MaxStalls,   \* network stalls (honest network)
  MaxEnds,     \* streams ended by the transport
  MaxRestarts, \* connections after the first
  Honest,
  Greedy

ASSUME 2 * D <= PTO /\ W >= 1 /\ NMin >= 1 /\ B >= 1 /\ Gap <= B /\ CreditMax >= 1
       /\ AccDelay <= DropMax + 1  \* the spec's ceiling: one PROBE per E of the negotiation timeout

IncTo(a, c) == IF a < c THEN a + 1 ELSE c
AgeTo(s, c) == [j \in 1..Len(s) |-> [s[j] EXCEPT !.age = IncTo(@, c)]]
Dec(x) == IF x > 0 THEN x - 1 ELSE 0
Max(a, b) == IF a > b THEN a ELSE b
RMax == MaxStreams * MaxProbes + MaxData + 1
ElapsedCap == MaxStreams * MaxProbes          \* EchoBound holds trivially beyond
OpenAgeCap == MaxProbes + 2                    \* the seq bound cannot bind beyond

VARIABLES
  epoch, sent, recv, data, ndata,
  sid,        \* streams opened on this connection
  out,        \* P's open stream (0: none)
  wait,       \* ticks before P may open a stream
  toR,        \* PROBEs in flight: <<[sid, seq, s, age]>>
  echoes,     \* ECHOes in flight: <<[sid, seq, r, ps, age]>> (ps: ghost, the s of the PROBE answered)
  probes, pending, lastEchoed, lastR, samples, ticked,
  r,          \* R: [in |-> open inbound stream (0: none), last |-> its last seq, age |-> ticks since
              \*   it was accepted, credit, refuse |-> ticks before a new stream is accepted,
              \*   count |-> inbound streams held open, reset |-> a reset not yet seen by P,
              \*   acc |-> ticks before R accepts the open stream (it reads nothing until then)]
  echoesSent, elapsed, stalled, stalls, ends,
  g           \* ghosts: [violP, violR, psent, pdel, crossed, lost, rdata, acceptRefused, refusedHonest]

vars == <<epoch, sent, recv, data, ndata, sid, out, wait, toR, echoes, probes, pending, lastEchoed,
          lastR, samples, ticked, r, echoesSent, elapsed, stalled, stalls, ends, g>>
pvars == <<probes, pending, lastEchoed, lastR, samples>>

\* ---- The rules (overridden one at a time by the mutants) ----
LateOK(age) == age <= PTO                       \* §8.5.5 step 3.6
SentOK(x) == x <= sent                          \* §8.5.5 step 3
MonoOK(x) == x >= lastR                         \* §8.5.5 step 3
Clamp(dR, dS) == IF dR < dS THEN dR ELSE dS     \* §8.5.5 step 5
KeepPending(p) == IF Len(p) > MaxOut THEN Tail(p) ELSE p
KeepSamples(s) == IF Len(s) > MaxSamples THEN Tail(s) ELSE s
RestartSamples == <<>>                          \* nothing passes from one connection to the next
MayOpen == TRUE                                 \* a stream reopens after the backoff
MayProbe == TRUE                                \* PROBEs never wait for an ECHO
SameStream(e) == e.sid = out                    \* §8.5.3: bytes of an ended stream have no effect
OpenCredit == r.credit                          \* §8.5.6: the credit is per connection
DropFault(n) == FALSE                           \* §8.5.6 step 4: a dropped PROBE is never a fault
Accepts == r.count = 0 /\ r.refuse = 0          \* §8.5.6 step 1: one stream; refusal after an end
EndRefusal(viol) == IF viol THEN B ELSE Gap     \* §8.5.6 steps 1 and 5: B after a violation, B/2 after any end
CreditOK == r.credit >= 1                       \* §8.5.6 step 3: an echo spends credit
SeqOK(q) == q <= r.age + 2 + DropMax            \* §8.5.6 step 3: above, a flood: violation
InBound(q) == q <= r.age + 2                    \* §8.5.6 step 3: seq <= elapsed / E + 2, else dropped
Counted == recv + 1                             \* §8.5.6: every packet of P, the PROBE's included

\* A Byzantine responder's seq: the right one, one behind (a replay) or one ahead (never sent);
\* its count: 0 (below an earlier one), the truth, or more than was ever sent (RMax).
Forged(q) == {x \in {q - 1, q, q + 1} : x >= 1}
Remove(s, i) == [j \in 1..(Len(s) - 1) |-> IF j < i THEN s[j] ELSE s[j + 1]]
\* A PROBE of the stream R has not accepted yet: R reads nothing of it until it does.
Unaccepted(h) == h.sid = r.in /\ r.acc > 0

R0 == [in |-> 0, last |-> 0, age |-> 0, credit |-> CreditMax, refuse |-> 0, count |-> 0, reset |-> FALSE,
       acc |-> 0]
G0 == [violP |-> FALSE, violR |-> FALSE, psent |-> 0, pdel |-> 0, crossed |-> FALSE, lost |-> FALSE,
       rdata |-> 0, acceptRefused |-> FALSE, refusedHonest |-> FALSE]

Init ==
  /\ epoch = 0 /\ sent = 0 /\ recv = 0 /\ data = <<>> /\ ndata = 0
  /\ sid = 0 /\ out = 0 /\ wait = 0 /\ toR = <<>> /\ echoes = <<>>
  /\ probes = 0 /\ pending = <<>> /\ lastEchoed = 0 /\ lastR = 0 /\ samples = <<>> /\ ticked = FALSE
  /\ r = R0 /\ echoesSent = 0 /\ elapsed = 0
  /\ stalled = FALSE /\ stalls = 0 /\ ends = 0 /\ g = G0

CanOpen == out = 0 /\ (wait = 0 \/ Greedy) /\ sid < MaxStreams /\ (sid = 0 \/ MayOpen)

\* R's side of a stream end (any end): the refusal of §8.5.6 step 1.
REnd(rr, viol) == [rr EXCEPT !.in = 0, !.count = 0, !.refuse = Max(rr.refuse, EndRefusal(viol))]

\* P opens a stream (the negotiation is not modelled: corpus linkprobe-host-*); R accepts it or
\* refuses it unread, which P sees as the end of the stream.
Open ==
  /\ CanOpen
  /\ sid' = sid + 1
  /\ IF Accepts
       THEN /\ out' = sid + 1 /\ wait' = wait
            /\ \E a \in 0..AccDelay :
                 r' = [r EXCEPT !.in = sid + 1, !.last = 0, !.age = 0, !.count = 1, !.credit = OpenCredit,
                                !.acc = a]
            /\ probes' = 0 /\ pending' = <<>> /\ lastEchoed' = 0
            /\ g' = [g EXCEPT !.acceptRefused = @ \/ r.refuse > 0]
       ELSE /\ out' = 0 /\ wait' = B /\ r' = r /\ UNCHANGED <<probes, pending, lastEchoed>>
            /\ g' = [g EXCEPT !.refusedHonest = @ \/ ~Greedy]
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, toR, echoes, lastR, samples, ticked, echoesSent,
                 elapsed, stalled, stalls, ends>>

\* A flooding P opens a second stream while one is open: R must refuse it.
OpenAgain ==
  /\ Greedy /\ out /= 0 /\ sid < MaxStreams
  /\ sid' = sid + 1
  /\ IF Accepts
       THEN r' = [r EXCEPT !.count = @ + 1, !.in = sid + 1, !.last = 0, !.age = 0, !.acc = 0]
       ELSE r' = r
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, out, wait, toR, echoes, probes, pending, lastEchoed,
                 lastR, samples, ticked, echoesSent, elapsed, stalled, stalls, ends, g>>

SendProbe ==
  /\ out /= 0 /\ probes < MaxProbes /\ (~ticked \/ Greedy) /\ MayProbe
  /\ probes' = probes + 1
  /\ pending' = KeepPending(Append(pending, [seq |-> probes + 1, age |-> 0, s |-> sent]))
  /\ sent' = sent + 1
  /\ toR' = Append(toR, [sid |-> out, seq |-> probes + 1, s |-> sent, age |-> 0])
  /\ ticked' = TRUE
  /\ g' = [g EXCEPT !.psent = @ + 1]
  /\ UNCHANGED <<epoch, recv, data, ndata, sid, out, wait, echoes, lastEchoed, lastR, samples, r,
                 echoesSent, elapsed, stalled, stalls, ends>>

SendData ==
  /\ ndata < MaxData
  /\ ndata' = ndata + 1 /\ sent' = sent + 1 /\ data' = Append(data, [age |-> 0, pb |-> g.psent])
  /\ UNCHANGED <<epoch, recv, sid, out, wait, toR, echoes, probes, pending, lastEchoed, lastR,
                 samples, ticked, r, echoesSent, elapsed, stalled, stalls, ends, g>>

\* A data packet arrives; it crossed a PROBE if the PROBEs delivered so far are not exactly
\* those sent before it.
DeliverData(i) ==
  /\ ~stalled /\ i \in 1..Len(data)
  /\ recv' = recv + 1 /\ data' = Remove(data, i)
  /\ g' = [g EXCEPT !.rdata = @ + 1, !.crossed = @ \/ g.pdel /= data[i].pb]
  /\ UNCHANGED <<epoch, sent, ndata, sid, out, wait, toR, echoes, probes, pending, lastEchoed,
                 lastR, samples, ticked, r, echoesSent, elapsed, stalled, stalls, ends>>

LoseData(i) ==
  /\ i \in 1..Len(data)
  /\ data' = Remove(data, i) /\ g' = [g EXCEPT !.lost = TRUE]
  /\ UNCHANGED <<epoch, sent, recv, ndata, sid, out, wait, toR, echoes, probes, pending, lastEchoed,
                 lastR, samples, ticked, r, echoesSent, elapsed, stalled, stalls, ends>>

\* R reads the head PROBE (its packet is counted first: recv + 1).
DeliverProbe ==
  /\ ~stalled /\ toR /= <<>> /\ ~Unaccepted(Head(toR))
  /\ LET h == Head(toR)
         gd == [g EXCEPT !.pdel = @ + 1]
     IN
     /\ recv' = recv + 1 /\ toR' = Tail(toR)
     /\ IF h.sid /= r.in
          THEN UNCHANGED <<echoes, r, echoesSent>> /\ g' = gd                       \* ended stream
        ELSE IF ~Honest
          THEN /\ \/ \E q \in Forged(h.seq), x \in {0, recv + 1, RMax} :
                       echoes' = Append(echoes, [sid |-> h.sid, seq |-> q, r |-> x, ps |-> h.s, age |-> 0])
                  \/ echoes' = echoes
               /\ UNCHANGED <<r, echoesSent>> /\ g' = gd
        ELSE IF h.seq <= r.last \/ ~SeqOK(h.seq)
          THEN /\ r' = [REnd(r, TRUE) EXCEPT !.reset = TRUE]
               /\ g' = [gd EXCEPT !.violR = TRUE]
               /\ UNCHANGED <<echoes, echoesSent>>
        ELSE IF InBound(h.seq) /\ CreditOK
          THEN /\ echoes' = Append(echoes, [sid |-> h.sid, seq |-> h.seq, r |-> Counted, ps |-> h.s, age |-> 0])
               /\ r' = [r EXCEPT !.credit = IF @ > 0 THEN @ - 1 ELSE 0, !.last = h.seq]
               /\ echoesSent' = echoesSent + 1 /\ g' = gd
        ELSE /\ r' = IF DropFault(1) THEN [REnd(r, TRUE) EXCEPT !.reset = TRUE] ELSE [r EXCEPT !.last = h.seq]
             /\ g' = IF DropFault(1) THEN [gd EXCEPT !.violR = TRUE] ELSE gd
             /\ UNCHANGED <<echoes, echoesSent>>
  /\ UNCHANGED <<epoch, sent, data, ndata, sid, out, wait, probes, pending, lastEchoed, lastR, samples,
                 ticked, elapsed, stalled, stalls, ends>>

\* P's side of the end of its stream.
PEnd(viol) ==
  /\ out' = 0 /\ pending' = <<>> /\ wait' = B
  /\ IF viol THEN samples' = <<>> /\ lastR' = 0 ELSE UNCHANGED <<samples, lastR>>

\* P reads the head ECHO (§8.5.5 step 3).
ReceiveEcho ==
  /\ ~stalled /\ echoes /= <<>>
  /\ echoes' = Tail(echoes)
  /\ LET e == Head(echoes) IN
     IF ~SameStream(e) \/ out = 0
       THEN UNCHANGED <<out, pending, wait, r, samples, lastR, lastEchoed, g>>
     ELSE IF e.seq > probes \/ e.seq <= lastEchoed \/ ~SentOK(e.r) \/ ~MonoOK(e.r)
       THEN PEnd(TRUE) /\ r' = REnd(r, FALSE) /\ g' = [g EXCEPT !.violP = TRUE] /\ UNCHANGED lastEchoed
     ELSE LET rest == SelectSeq(pending, LAMBDA p : p.seq >= e.seq) IN
          /\ lastEchoed' = e.seq /\ lastR' = e.r
          /\ IF rest = <<>> \/ Head(rest).seq /= e.seq
               THEN pending' = rest /\ UNCHANGED samples                              \* stale
               ELSE /\ pending' = Tail(rest)
                    /\ IF LateOK(Head(rest).age)
                         THEN samples' = KeepSamples(Append(samples,
                                [age |-> Head(rest).age, s |-> Head(rest).s, r |-> e.r,
                                 rtt |-> Head(rest).age, ep |-> epoch, ps |-> e.ps]))   \* accepted
                         ELSE UNCHANGED samples                                       \* late
          /\ UNCHANGED <<out, wait, r, g>>
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, sid, toR, probes, ticked, echoesSent, elapsed,
                 stalled, stalls, ends>>

\* P sees the reset R raised (§8.5.6 step 5): the end of its stream.
SeeReset ==
  /\ r.reset /\ r' = [r EXCEPT !.reset = FALSE]
  /\ IF out /= 0 THEN PEnd(FALSE) ELSE UNCHANGED <<out, pending, wait, samples, lastR>>
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, sid, toR, echoes, probes, lastEchoed, ticked,
                 echoesSent, elapsed, stalled, stalls, ends, g>>

\* The transport ends the stream (reset, FIN, error): the samples stay.
EndStream ==
  /\ out /= 0 /\ ends < MaxEnds
  /\ ends' = ends + 1 /\ PEnd(FALSE) /\ r' = REnd(r, FALSE)
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, sid, toR, echoes, probes, lastEchoed, ticked,
                 echoesSent, elapsed, stalled, stalls, g>>

Stall ==
  /\ Honest /\ ~stalled /\ stalls < MaxStalls
  /\ stalled' = TRUE /\ stalls' = stalls + 1
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, sid, out, wait, toR, echoes, probes, pending,
                 lastEchoed, lastR, samples, ticked, r, echoesSent, elapsed, ends, g>>

Unstall ==
  /\ stalled /\ stalled' = FALSE
  /\ UNCHANGED <<epoch, sent, recv, data, ndata, sid, out, wait, toR, echoes, probes, pending,
                 lastEchoed, lastR, samples, ticked, r, echoesSent, elapsed, stalls, ends, g>>

Due == \/ \E i \in 1..Len(toR) : toR[i].age >= D /\ ~Unaccepted(toR[i])
       \/ \E i \in 1..Len(echoes) : echoes[i].age >= D
       \/ \E i \in 1..Len(data) : data[i].age >= D

\* The prober's timer (§8.5.5): one PROBE per interval while it can send one, and a stream
\* opened as soon as it may be; a reset R raised is seen before time passes.
ProbeDue == \/ out /= 0 /\ probes < MaxProbes /\ ~ticked /\ MayProbe
            \/ CanOpen
            \/ r.reset

Tick ==
  /\ ~ProbeDue
  /\ (Honest /\ ~stalled) => ~Due
  /\ toR' = AgeTo(toR, D) /\ echoes' = AgeTo(echoes, D) /\ data' = AgeTo(data, D)
  /\ pending' = AgeTo(pending, PTO + 1) /\ samples' = AgeTo(samples, 2 * W + 1)
  /\ wait' = Dec(wait)
  \* The responder's credit, refusal and stream age; the elapsed time only matters to EchoBound.
  /\ r' = [r EXCEPT !.credit = IF Honest /\ @ < CreditMax THEN @ + 1 ELSE @,
                    !.refuse = Dec(@), !.acc = Dec(@),
                    !.age = IF r.acc > 0 THEN 0 ELSE IncTo(@, OpenAgeCap)]  \* age counts from acceptance
  /\ elapsed' = IF Honest /\ elapsed < ElapsedCap THEN elapsed + 1 ELSE elapsed
  /\ ticked' = FALSE
  /\ UNCHANGED <<epoch, sent, recv, ndata, sid, out, probes, lastEchoed, lastR, echoesSent,
                 stalled, stalls, ends, g>>

\* A new connection (§8.5.5, §8.5.6): its counters and both machines start afresh.
Restart ==
  /\ epoch < MaxRestarts
  /\ epoch' = epoch + 1 /\ sent' = 0 /\ recv' = 0 /\ data' = <<>> /\ ndata' = 0
  /\ sid' = 0 /\ out' = 0 /\ wait' = 0 /\ toR' = <<>> /\ echoes' = <<>>
  /\ probes' = 0 /\ pending' = <<>> /\ lastEchoed' = 0 /\ lastR' = 0
  /\ samples' = RestartSamples /\ ticked' = FALSE
  /\ r' = R0 /\ echoesSent' = 0 /\ elapsed' = 0 /\ g' = G0
  /\ UNCHANGED <<stalled, stalls, ends>>

Next ==
  \/ Open \/ OpenAgain \/ SendProbe \/ SendData \/ DeliverProbe \/ ReceiveEcho \/ SeeReset
  \/ EndStream \/ Tick \/ Stall \/ Unstall \/ Restart
  \/ \E i \in 1..MaxData : DeliverData(i) \/ LoseData(i)

Spec == Init /\ [][Next]_vars
  /\ WF_vars(Open) /\ WF_vars(SendProbe) /\ WF_vars(DeliverProbe) /\ WF_vars(ReceiveEcho)
  /\ WF_vars(SeeReset) /\ WF_vars(Tick) /\ WF_vars(Unstall)
  /\ WF_vars(\E i \in 1..MaxData : DeliverData(i))

\* ---- The figure (§8.5.5 step 5) ----
Newest == samples[Len(samples)]
InWin == {i \in 1..Len(samples) : samples[i].age - Newest.age <= W}
Oldest == CHOOSE i \in InWin : \A j \in InWin : i <= j
DS == Newest.s - samples[Oldest].s
DR == Newest.r - samples[Oldest].r
Known == Len(samples) >= 2 /\ Newest.age <= W /\ Oldest < Len(samples)
         /\ samples[Oldest].age - Newest.age >= W \div 2 /\ DS >= NMin
Figure == ((DS - Clamp(DR, DS)) * 1000 + DS \div 2) \div DS

\* ---- Properties ----
TypeOK ==
  /\ epoch \in 0..MaxRestarts /\ sid \in 0..MaxStreams /\ out \in 0..MaxStreams
  /\ probes \in 0..MaxProbes /\ r.credit \in 0..CreditMax /\ stalled \in BOOLEAN

Bounded == Len(pending) <= MaxOut /\ Len(samples) <= MaxSamples           \* §8.5.7
SampleBounded == \A i \in 1..Len(samples) : samples[i].r <= sent /\ samples[i].s <= sent
NoLateSample == \A i \in 1..Len(samples) : samples[i].rtt <= PTO
Monotone == \A i, j \in 1..Len(samples) :
              i < j => samples[i].s <= samples[j].s /\ samples[i].r <= samples[j].r
CurrentConnection == \A i \in 1..Len(samples) : samples[i].ep = epoch
FigureInRange == Known => Figure \in 0..1000
\* Honest responder: a sample pairs an ECHO with the very PROBE it answers (never one of an
\* ended stream); no side raises a violation and an honest prober is never refused, stalls
\* included; the credit is per connection; and with no loss and nothing crossing a PROBE the
\* figure is exactly 0 (§8.5.5 « Exactitude »).
SampleMatches == Honest => \A i \in 1..Len(samples) : samples[i].ps = samples[i].s
HonestNoViolation == (Honest /\ ~Greedy) => ~g.violP /\ ~g.violR /\ ~g.refusedHonest
EchoBound == Honest => echoesSent <= CreditMax + elapsed
\* A data packet still in flight that was sent before a PROBE already delivered will cross it.
CrossingInFlight == \E i \in 1..Len(data) : data[i].pb < g.pdel
Exact == (Honest /\ ~g.lost /\ ~g.crossed /\ ~CrossingInFlight /\ Known) => Figure = 0
\* Any responder: it never holds two inbound streams, and never accepts one while refusing.
OneInbound == r.count <= 1
BanHolds == ~g.acceptRefused
\* Honest responder: it never reads a seq more than DropMax above the rate bound (a flood is cut);
\* PROBEs written before it accepted the stream are dropped, never a fault (HonestNoViolation).
RateBound == Honest => (r.in /= 0 => r.last <= r.age + 2 + DropMax)

\* Honest responder: once no stall, stream end or restart remains (the bounded stand-in for « after
\* any finite number of them ») and a fresh stream is open, the figure becomes known; and a stream
\* that ended always reopens.
CleanStart == epoch = MaxRestarts /\ stalls = MaxStalls /\ ends = MaxEnds /\ ~stalled
              /\ out /= 0 /\ probes = 0
Live == [](CleanStart => <>Known)
Reopens == [](out = 0 /\ sid < MaxStreams => <>(out /= 0))
\* Whatever the responder does, an open stream sends all its PROBEs or ends: P never waits on R.
NeverBlocked == [](out /= 0 => <>(out = 0 \/ probes = MaxProbes))
=============================================================================
