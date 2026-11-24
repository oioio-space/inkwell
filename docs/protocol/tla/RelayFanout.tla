---------------------------- MODULE RelayFanout ----------------------------
(***************************************************************************)
(* Block RF1 (plan docs/plans/2026-11-24-rf1-relay-fanout.md, spec §16.1   *)
(* rows « connexions », « connexions non vérifiées », « connexions par IP  *)
(* (membres) »; rules RF-1 to RF-6): how a node admits connections before  *)
(* and after their handshake, so that members are never refused because   *)
(* other members share their relay or their IP address, an unknown peer   *)
(* flooding through one origin cannot take what members need, and a       *)
(* Byzantine member cannot take another member's share.                   *)
(*                                                                         *)
(* One node. An origin is what an inbound connection is charged to before *)
(* its handshake: a relay, named by its authenticated PeerID (RF-1), or an *)
(* IP subnet; the model treats them alike (the Go tests check how the      *)
(* origin is derived). An honest member opens an inbound connection from  *)
(* its origin (Via); it is pending until its handshake authenticates it    *)
(* (MemberAuth), then it is a member connection and leaves every pending  *)
(* count (RF-2). The node dials its members (XDial): an outbound           *)
(* connection is a member connection from its start (RF-2); the remote     *)
(* end's own admission is that node's business. A passage (two             *)
(* connections) ends (Settle); member connections close finitely often     *)
(* (Close). A Byzantine member (Byz) opens and closes authenticated        *)
(* connections at will, never fair. A stranger (any number of PeerIDs      *)
(* behind each origin of AttOrigins, never fair) opens pending              *)
(* connections; each ends within the handshake deadline (AttExpire,        *)
(* weakly fair: RF-4), by the deadline or by the gate.                     *)
(*                                                                         *)
(* Bounds: P pending per origin, Q pending in all (RF-3); a token bucket   *)
(* per origin of burst B refilled one token per period (Refill, weakly     *)
(* fair: time passes) (RF-5); MemberCap member connections, of which 2 are *)
(* each member's share and the rest a surplus a member may use up to       *)
(* PerCap while every other member's unused share stays free (RF-6); the   *)
(* node's capacity is MemberCap + Q, so pending connections never use      *)
(* members' room.                                                          *)
(*                                                                         *)
(* The bucket is kept as debts, so that a refund is exact: the tokens      *)
(* missing are the strangers' unpaid admissions (sd), the tokens of        *)
(* members still pending (held), and, under a mutant, member tokens never  *)
(* given back (od). A period pays one unpaid debt, any one (every payment  *)
(* order of a real bucket is explored). A member's handshake removes its   *)
(* own debt if still unpaid, and only then (golang.org/x/time/rate's       *)
(* Reservation.Cancel restores nothing once its time has passed, so the    *)
(* ledger keeps its own bucket). Ghost g is the strangers' admissions      *)
(* minus the periods, from its lowest point: g <= B is « at most B + k     *)
(* stranger admissions in any k periods ».                                 *)
(*                                                                         *)
(* Truth operators (PendOn, Pending, EstAll, Tok, Owed, Why) are never     *)
(* overridden. Rule operators, overridden by                               *)
(* mutants/RelayFanoutMutants.tla: OriginLoad, GlobalLoad, Room, OutOK,    *)
(* AuthOK, SysCap, RefillStep, RefundStep and the switches Expires,        *)
(* UseOrigin, UseGlobal, UseToken.                                         *)
(***************************************************************************)
EXTENDS Integers, FiniteSets

CONSTANTS
  Members,    \* members that want a link with the node
  Byz,        \* the Byzantine ones among them (a subset)
  Origins,    \* origins of a pending inbound connection
  Via,        \* [Members -> Origins]: where an honest member's inbound connections come from
  AttOrigins, \* the origins a stranger floods
  MaxAtt,     \* stranger openings in all (Unbounded: never stops)
  P,          \* pending connections per origin (spec: 16 per relay, 8 per IP)
  Q,          \* pending connections in all (spec: 48)
  B,          \* bucket burst per origin (spec: 16, refilled 1 per 5 s)
  MemberCap,  \* member connections (spec: 128, 2 for each of 63 peers and 2 more)
  PerCap,     \* member connections per member, share and surplus (spec: 4)
  MaxCloses,  \* honest member connections that may close (finite, for liveness)
  Dials,      \* TRUE: the node dials its members; FALSE: inbound only
  Late        \* honest members admitted to the membership only later (review round: roster growth); {} elsewhere

Honest == Members \ Byz

VARIABLES
  pendA,  \* [Origins -> Nat]: stranger connections pending (before the handshake, or identified, not yet closed)
  pendM,  \* [Members -> BOOLEAN]: an honest member's inbound connection before its handshake
  held,   \* [Members -> BOOLEAN]: that connection's token is still an unpaid debt of the bucket
  est,    \* [Members -> 0..PerCap]: a member's member connections (honest: 2 at most, the passage)
  sd,     \* [Origins -> Nat]: strangers' admissions not yet paid by a period
  od,     \* [Origins -> Nat]: member tokens never given back (only under a mutant)
  g,      \* [Origins -> Nat]: ghost, stranger admissions minus periods, from its lowest point
  natt,   \* stranger openings so far (when MaxAtt >= 0)
  closes, \* honest member connections closed so far
  ev,     \* ghost: the last refusal, and why the truth allowed it
  admitted \* the members of the membership list (the roster): the shares are reserved for them

vars == <<pendA, pendM, held, est, sd, od, g, natt, closes, ev, admitted>>

RECURSIVE SumOver(_, _)
SumOver(f, S) == IF S = {} THEN 0 ELSE LET x == CHOOSE y \in S : TRUE IN f[x] + SumOver(f, S \ {x})

\* Truth: what the node holds (never overridden).
PendMOn(o) == Cardinality({m \in Honest : pendM[m] /\ Via[m] = o})
PendOn(o) == pendA[o] + PendMOn(o)
Pending == SumOver(pendA, Origins) + Cardinality({m \in Honest : pendM[m]})
EstAll == SumOver(est, Members)
EstOn(o) == SumOver(est, {m \in Honest : Via[m] = o})
Total == Pending + EstAll
HeldOn(o) == Cardinality({m \in Honest : pendM[m] /\ held[m] /\ Via[m] = o})
Tok(o) == B - sd[o] - od[o] - HeldOn(o)
Short(m) == IF est[m] < 2 THEN 2 - est[m] ELSE 0
\* The shares the other members of the roster have not used yet.
Owed(m) == SumOver([x \in Members |-> Short(x)], admitted \ {m})
\* The reasons the truth gives, now, for refusing a member through origin o.
Why(o) == {r \in {"origin", "global", "rate", "cap"} :
             \/ r = "origin" /\ PendOn(o) >= P
             \/ r = "global" /\ Pending >= Q
             \/ r = "rate" /\ Tok(o) <= 0
             \/ r = "cap" /\ EstAll >= MemberCap}

\* Rule operators (the mutants override them).
OriginLoad(o) == PendOn(o)          \* RF-1, RF-2: pending only; a member leaves the count at its handshake
GlobalLoad == Pending               \* RF-3
SysCap == MemberCap + Q             \* RF-6: pending connections never use members' room
\* RF-6: a member's first two connections are its share; a third or fourth uses the surplus, never another's share.
Room(m) == IF est[m] < 2 THEN EstAll < MemberCap ELSE est[m] < PerCap /\ EstAll + Owed(m) < MemberCap
OutOK(m) == Room(m) /\ Total < SysCap      \* RF-2: an outbound opening is a member connection from its start
AuthOK(m) == est[m] = 2 \/ Room(m)         \* RF-6: a handshake makes a member connection where the member has room
Expires == TRUE                     \* RF-4: a pending connection ends within the handshake deadline
UseOrigin == TRUE                   \* RF-3
UseGlobal == TRUE                   \* RF-3
UseToken == TRUE                    \* RF-5
InOK(o) == /\ UseOrigin => OriginLoad(o) < P
           /\ UseGlobal => GlobalLoad < Q
           /\ UseToken => Tok(o) > 0
           /\ Total < SysCap
\* RF-5: a period pays one unpaid debt of o, any one (or nothing is owed).
RefillStep(o) ==
  \/ /\ sd[o] > 0
     /\ sd' = [sd EXCEPT ![o] = @ - 1] /\ UNCHANGED <<held, od>>
  \/ \E m \in Honest :
       /\ pendM[m] /\ held[m] /\ Via[m] = o
       /\ held' = [held EXCEPT ![m] = FALSE] /\ UNCHANGED <<sd, od>>
  \/ /\ od[o] > 0
     /\ od' = [od EXCEPT ![o] = @ - 1] /\ UNCHANGED <<sd, held>>
  \/ /\ sd[o] = 0 /\ od[o] = 0 /\ HeldOn(o) = 0
     /\ UNCHANGED <<sd, held, od>>
\* RF-5: a member's handshake removes its own debt if a period has not paid it yet, and nothing else.
RefundStep(m) ==
  IF held[m] THEN held' = [held EXCEPT ![m] = FALSE] /\ UNCHANGED <<sd, od>>
  ELSE UNCHANGED <<held, sd, od>>
\* RF-6, roster growth: when the roster becomes A, the node closes connections members hold above their own (their
\* share in the roster, nothing outside), just enough for every roster member's unused share to fit again. Any such
\* choice (the code's: the member most above its own, newest first, is one of them).
Own(x, A) == IF x \in A THEN 2 ELSE 0
ShortIn(e, x, A) == IF x \in A /\ e[x] < 2 THEN 2 - e[x] ELSE 0
Excess(A) == EstAll + SumOver([x \in Members |-> ShortIn(est, x, A)], Members) - MemberCap
Shed(A) == {e \in [Members -> 0..PerCap] :
              /\ \A x \in Members : e[x] <= est[x] /\ (e[x] >= Own(x, A) \/ e[x] = est[x])
              /\ EstAll - SumOver(e, Members) = IF Excess(A) > 0 THEN Excess(A) ELSE 0}
ShedStep(A) == \E e \in Shed(A) : est' = e

\* MaxAtt for a stranger that never stops (a config cannot write -1).
Unbounded == -1
\* The configurations' member origins: every member behind origin 1 (one relay, or one home router).
ViaOne == [m \in Members |-> 1]
\* Members 1 and 2 behind origin 1 (a relay), the others behind origin 2 (a home router).
ViaSplit == [m \in Members |-> IF m <= 2 THEN 1 ELSE 2]

NoEv == [k |-> "none", m |-> CHOOSE x \in Members : TRUE, why |-> {}]
Refusal(k, m, o) == [k |-> k, m |-> m, why |-> Why(o)]

Init ==
  /\ pendA = [o \in Origins |-> 0]
  /\ pendM = [m \in Members |-> FALSE]
  /\ held = [m \in Members |-> FALSE]
  /\ est = [m \in Members |-> 0]
  /\ sd = [o \in Origins |-> 0]
  /\ od = [o \in Origins |-> 0]
  /\ g = [o \in Origins |-> 0]
  /\ natt = 0
  /\ closes = 0
  /\ ev = NoEv
  /\ admitted = Members \ Late

\* An honest member without a connection opens one to the node (it retries while refused).
MemberOpen(m) ==
  /\ m \in Honest /\ m \in admitted /\ est[m] = 0 /\ ~pendM[m]
  /\ IF InOK(Via[m])
       THEN /\ pendM' = [pendM EXCEPT ![m] = TRUE]
            /\ held' = [held EXCEPT ![m] = Tok(Via[m]) > 0]
            /\ ev' = NoEv
            /\ UNCHANGED <<pendA, est, sd, od, g, natt, closes, admitted>>
       ELSE /\ ev' = Refusal("in", m, Via[m])
            /\ UNCHANGED <<pendA, pendM, held, est, sd, od, g, natt, closes, admitted>>

\* Its handshake authenticates it: it leaves the pending counts (RF-2), its token comes back (RF-5).
MemberAuth(m) ==
  /\ pendM[m]
  /\ pendM' = [pendM EXCEPT ![m] = FALSE]
  /\ RefundStep(m)
  /\ IF AuthOK(m)
       THEN /\ est' = [est EXCEPT ![m] = IF @ < 2 THEN @ + 1 ELSE @]
            /\ ev' = NoEv
       ELSE /\ est' = est
            /\ ev' = Refusal("auth", m, Via[m])
  /\ UNCHANGED <<pendA, g, natt, closes, admitted>>

\* The node dials an honest member it has no connection with.
XDial(m) ==
  /\ Dials /\ m \in Honest /\ m \in admitted /\ est[m] = 0
  /\ IF OutOK(m)
       THEN est' = [est EXCEPT ![m] = 1] /\ ev' = NoEv
       ELSE est' = est /\ ev' = Refusal("out", m, Via[m])
  /\ UNCHANGED <<pendA, pendM, held, sd, od, g, natt, closes, admitted>>

\* The passage ends: of a member's two connections, the older one closes (spec §8.7.4, P-15).
Settle(m) ==
  /\ m \in Honest /\ est[m] = 2
  /\ est' = [est EXCEPT ![m] = 1]
  /\ ev' = NoEv
  /\ UNCHANGED <<pendA, pendM, held, sd, od, g, natt, closes, admitted>>

\* An honest member connection closes (finitely often).
Close(m) ==
  /\ m \in Honest /\ est[m] > 0 /\ closes < MaxCloses
  /\ est' = [est EXCEPT ![m] = @ - 1]
  /\ closes' = closes + 1
  /\ ev' = NoEv
  /\ UNCHANGED <<pendA, pendM, held, sd, od, g, natt, admitted>>

\* A Byzantine member opens one more authenticated connection, as many as the rule lets it.
ByzOpen(b) ==
  /\ b \in Byz /\ est[b] < PerCap
  /\ IF OutOK(b)
       THEN est' = [est EXCEPT ![b] = @ + 1] /\ ev' = NoEv
       ELSE est' = est /\ ev' = Refusal("byz", b, Via[b])
  /\ UNCHANGED <<pendA, pendM, held, sd, od, g, natt, closes, admitted>>

ByzClose(b) ==
  /\ b \in Byz /\ est[b] > 0
  /\ est' = [est EXCEPT ![b] = @ - 1]
  /\ ev' = NoEv
  /\ UNCHANGED <<pendA, pendM, held, sd, od, g, natt, closes, admitted>>

\* A stranger opens a pending connection through o (refused openings change nothing).
AttOpen(o) ==
  /\ o \in AttOrigins
  /\ MaxAtt < 0 \/ natt < MaxAtt
  /\ InOK(o)
  /\ pendA' = [pendA EXCEPT ![o] = @ + 1]
  /\ sd' = [sd EXCEPT ![o] = IF Tok(o) > 0 THEN @ + 1 ELSE @]
  /\ g' = [g EXCEPT ![o] = @ + 1]
  /\ natt' = IF MaxAtt < 0 THEN natt ELSE natt + 1
  /\ ev' = NoEv
  /\ UNCHANGED <<pendM, held, est, od, closes, admitted>>

\* The handshake deadline (or the gate, once identified) ends a stranger's connection (RF-4).
AttExpire(o) ==
  /\ Expires /\ pendA[o] > 0
  /\ pendA' = [pendA EXCEPT ![o] = @ - 1]
  /\ ev' = NoEv
  /\ UNCHANGED <<pendM, held, est, sd, od, g, natt, closes, admitted>>

\* One period of origin o's bucket passes.
Refill(o) ==
  /\ RefillStep(o)
  /\ g' = [g EXCEPT ![o] = IF @ > 0 THEN @ - 1 ELSE 0]
  /\ ev' = NoEv
  /\ UNCHANGED <<pendA, pendM, est, natt, closes, admitted>>

\* A member joins the membership after the surplus may have been taken: the roster grows, and the node sheds.
AdmitLate(m) ==
  /\ m \in Late \ admitted
  /\ admitted' = admitted \cup {m}
  /\ ShedStep(admitted \cup {m})
  /\ ev' = NoEv
  /\ UNCHANGED <<pendA, pendM, held, sd, od, g, natt, closes>>

Next ==
  \/ \E m \in Late : AdmitLate(m)
  \/ \E m \in Members : MemberOpen(m) \/ MemberAuth(m) \/ XDial(m) \/ Settle(m) \/ Close(m) \/ ByzOpen(m) \/ ByzClose(m)
  \/ \E o \in Origins : AttOpen(o) \/ AttExpire(o) \/ Refill(o)

Fairness ==
  /\ \A m \in Late : WF_vars(AdmitLate(m))
  /\ \A m \in Honest : WF_vars(MemberOpen(m)) /\ WF_vars(MemberAuth(m)) /\ WF_vars(XDial(m)) /\ WF_vars(Settle(m))
  /\ \A o \in Origins : WF_vars(AttExpire(o)) /\ WF_vars(Refill(o))

Spec == Init /\ [][Next]_vars /\ Fairness

TypeOK ==
  /\ Byz \subseteq Members
  /\ Late \subseteq Members \ Byz /\ admitted \subseteq Members
  /\ pendA \in [Origins -> Nat]
  /\ pendM \in [Members -> BOOLEAN]
  /\ held \in [Members -> BOOLEAN]
  /\ est \in [Members -> 0..PerCap]
  /\ \A m \in Honest : est[m] <= 2
  /\ \A m \in Members : held[m] => pendM[m]
  /\ \A o \in Origins : Tok(o) \in 0..B

\* S1 (RF-3, the threat model): the strangers and pending members of one origin hold at most P places.
OriginBound == \A o \in Origins : PendOn(o) <= P
\* S2 (RF-3): pending connections hold at most Q places in all.
GlobalBound == Pending <= Q
\* S3 (RF-5): at most B + k stranger admissions per origin in any k periods.
RateBound == \A o \in Origins : g[o] <= B
\* S4 (RF-5): members spend no rate: no member token stays missing after its handshake.
MembersSpendNoRate == \A o \in Origins : od[o] = 0
\* S5 (RF-6): member connections stay within the member capacity.
MemberCapBound == EstAll <= MemberCap
\* S9 (RF-6): every member's unused share stays free: a Byzantine member's surplus never takes it.
SharesKept == EstAll + SumOver([x \in Members |-> Short(x)], admitted) <= MemberCap
\* S6 (RF-1, RF-2, §16.1 « connexions par IP (membres) »): a member's inbound opening is refused only for a reason
\* the truth gives (pending load on its origin or in all, the rate, the member capacity), never because other members
\* already hold connections through the same relay or the same address.
InJustified == ev.k = "in" => ev.why # {}
\* S7 (RF-2, RF-6): the node's dial to an honest member is never refused: no stranger and no Byzantine member can
\* starve it.
OutNeverRefused == ev.k # "out"
\* S8 (RF-6): an honest member's authenticated handshake is never refused.
AuthNeverRefused == ev.k # "auth"
\* L1 (RF-2, RF-4): every honest member ends with a connection to the node, and keeps one.
MembersEventuallyUp == <>[](\A m \in Honest : est[m] >= 1)
=============================================================================
