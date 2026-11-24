----------------------------- MODULE CloseScope -----------------------------
(***************************************************************************)
(* Block PU1d (plan docs/plans/2026-11-24-pu1d-close-scope.md, spec        *)
(* §8.7.4, rules P-16 and P-17): the scope of what a failed sync opening   *)
(* closes, which connection an opening names, and who starts an opening.   *)
(*                                                                         *)
(* PeerUpAck (block PU1, frozen by gates/PU1b.toml and gates/PU1c.toml)    *)
(* already closes only the connection a failed opening ran on (CloseC in   *)
(* RecvReset and Timeout; its comments still say « ClosePeer », written    *)
(* before PU1d) and already lets any usable connection start an opening    *)
(* (StartOpen), on a connection it picks (StartOpen's \E c). This module   *)
(* extends it without editing it, as Supersede does: every action runs     *)
(* unchanged (Keep), except the two session-stream failure branches,       *)
(* restated with CloseNamed, and StartOpen, guarded by OpenGuard.          *)
(*                                                                         *)
(* Rule operators, overridden by the mutants (mutants/CloseScopeMutants):  *)
(*   FailClose(n, c)  P-16: a refusal or the deadline of n's session       *)
(*                    opening closes c, the connection the stream ran on,  *)
(*                    and nothing else (PeerUpAck's CloseC);               *)
(*   Names(st)        P-16: the connection the node names for its opening  *)
(*                    st, from its start: st.c (it opens on a connection   *)
(*                    it chose, network.Conn.NewStream). The mutant names  *)
(*                    it only when NewStream returns (host.NewStream: the  *)
(*                    swarm chooses, and a failure before the return names *)
(*                    no connection); with no name, the node cannot close  *)
(*                    a connection and closes them all (CloseAll, the code *)
(*                    before PU1d);                                        *)
(*   OpenGuard(n)     P-17: TRUE, any usable connection starts the opening *)
(*                    of a peer with no session stream and none opening,   *)
(*                    whether or not a connectedness event was emitted.    *)
(*                                                                         *)
(* evt[n] is libp2p's connectedness event, abstracted: set when n goes     *)
(* from no usable connection to one, cleared when it has none or when an   *)
(* opening consumes it. It is kept until consumed, which is kinder to the  *)
(* mutant than libp2p (an event that finds an opening in progress is lost  *)
(* in inkwell's onConnected): a mutant caught here is caught a fortiori.   *)
(*                                                                         *)
(* Not modelled (review finding 2):                                        *)
(*   - the responder's reset on a connection it is closing (§8.7.3 step    *)
(*     3): PeerUpAck's Decide sends nothing once the connection is not     *)
(*     usable for the responder, so a reset that reaches the opener before *)
(*     it learns the close never occurs here. Under P-16 such a reset,     *)
(*     read as a refusal, closes the same connection the close takes, so   *)
(*     S6 is unaffected; the Go test TestResetBeforeConnCloseKeepsOther    *)
(*     (step 2) covers that interleaving on controlled time;               *)
(*   - violations (a wrong byte, a byte after the ACK): same rule, carried *)
(*     by the vectors;                                                     *)
(*   - with no name (mutant only), the node's view that the connection is  *)
(*     closed (out[n].c \in seen[n]) is kept as PeerUpAck's: openingLost   *)
(*     reads it from the connections present at the start.                 *)
(***************************************************************************)
EXTENDS PeerUpAck

CONSTANTS
  Moves,         \* TRUE: PeerUpAck's move (relay to direct) is explored; its failures close nothing either way
  CrashBy        \* the nodes whose side of a connection may close at will (PeerUpAck's Close: a crash, a dead link)

VARIABLES
  evt,  \* [Nodes -> BOOLEAN]: a connectedness event pending, not yet consumed by an opening
  gh    \* ghost flags of the properties and witnesses

scv == <<evt, gh>>
allv == <<vars, scv>>

HasConn(n) == \E c \in Conns : Usable(n, c)
\* The peer's acknowledged session stream runs on x (the peer read n's ACK of it).
PeerSessionOn(n, x) == out[Other(n)].st = "open" /\ out[Other(n)].c = x

\* ---- Rule operators (the mutants override them) ----
FailClose(n, c) == CloseC(n, c)
Names(st) == st.c
OpenGuard(n) == TRUE

\* The connection n names for its session opening.
Named(n) == Names(out[n])
\* What a node that names no connection can do: close every connection to the peer (ClosePeer), c included.
CloseAll(n, c) ==
  /\ closedBy' = [x \in Conns |-> IF x = c \/ Usable(n, x) THEN closedBy[x] \cup {n} ELSE closedBy[x]]
  /\ seen' = [seen EXCEPT ![n] = @ \cup {c} \cup {x \in Conns : Usable(n, x)}]
\* The close of a failed session opening: the named connection (P-16), or, unnamed, all of them.
CloseNamed(n) == IF Named(n) = NoConn THEN CloseAll(n, out[n].c) ELSE FailClose(n, Named(n))

TypeS ==
  /\ evt \in [Nodes -> BOOLEAN]
  /\ gh \in [collat: BOOLEAN, beside: BOOLEAN, quiet: BOOLEAN, neg: BOOLEAN]

InitS ==
  /\ Init
  /\ evt = [n \in Nodes |-> FALSE]
  /\ gh = [collat |-> FALSE, beside |-> FALSE, quiet |-> FALSE, neg |-> FALSE]

\* The connectedness event after a step: raised on the edge from no usable connection to one, dropped with the last.
EvtNext == [m \in Nodes |-> IF ~HasConn(m) /\ HasConn(m)' THEN TRUE ELSE IF ~HasConn(m)' THEN FALSE ELSE evt[m]]

Keep(A) == A /\ evt' = EvtNext /\ UNCHANGED gh

\* After a failure of n's session opening on f: collat, it closed another connection carrying the peer's acknowledged
\* session; beside, the peer's acknowledged session ran on another connection n held (the case S6 speaks of); neg, the
\* failure came before NewStream returned (the case S7 speaks of).
AfterFail(n, f) ==
  /\ evt' = EvtNext
  /\ gh' = [gh EXCEPT
              !.collat = @ \/ \E x \in Conns \ {f} : x \notin seen[n] /\ x \in seen'[n] /\ PeerSessionOn(n, x),
              !.beside = @ \/ \E x \in Conns \ {f} : Usable(n, x) /\ PeerSessionOn(n, x),
              !.neg = @ \/ out[n].st = "neg"]

\* PeerUpAck's RecvReset, session branch (a refusal), with CloseNamed for CloseC.
RecvResetSession(n) ==
  \E m \in msgs :
    /\ m.t = "reset" /\ m.to = n /\ Deliverable(m) /\ EchoGone(m)
    /\ out[n].st \in Opening /\ out[n].s = m.s
    /\ msgs' = msgs \ {m}
    /\ out' = [out EXCEPT ![n] = NoStream]
    /\ CloseNamed(n)
    /\ UNCHANGED <<live, reg, admit, cached, nxt, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>
    /\ AfterFail(n, m.c)
\* Its other branches (a move's reset, a stale reset) are exactly the steps of RecvReset that leave out unchanged.
RecvResetS(n) == RecvResetSession(n) \/ Keep(RecvReset(n) /\ out' = out)

\* PeerUpAck's Timeout, session branch, with CloseNamed for CloseC.
TimeoutSession(n) ==
  /\ out[n].st \in Opening /\ TimeoutReady(n, out[n])
  /\ out' = [out EXCEPT ![n] = NoStream]
  /\ lateFire' = (lateFire \/ ~Silenced(n, out[n]))
  /\ IF out[n].c \in seen[n] THEN UNCHANGED <<closedBy, seen>> ELSE CloseNamed(n)
  /\ UNCHANGED <<live, reg, admit, cached, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves>>
  /\ AfterFail(n, out[n].c)
\* The move's deadline is exactly the step of Timeout that leaves out unchanged.
TimeoutS(n) == TimeoutSession(n) \/ Keep(Timeout(n) /\ out' = out)

\* An opening starts (P-17); quiet: it started with no connectedness event pending.
StartOpenS(n) ==
  /\ OpenGuard(n)
  /\ StartOpen(n)
  /\ evt' = [evt EXCEPT ![n] = FALSE]
  /\ gh' = [gh EXCEPT !.quiet = @ \/ ~evt[n]]

NextS ==
  \/ \E c \in Conns : Keep(Dial(c))
  \/ \E n \in Nodes, c \in Conns :
       Keep(Register(n, c)) \/ Keep(LearnClose(n, c)) \/ (n \in CrashBy /\ Keep(Close(n, c)))
  \/ \E n \in Nodes :
       \/ Keep(Drop(n)) \/ Keep(Readmit(n)) \/ StartOpenS(n) \/ (Moves /\ Keep(Move(n)))
       \/ Keep(RecvProto(n)) \/ Keep(RecvAck(n)) \/ RecvResetS(n) \/ Keep(Tick(n)) \/ TimeoutS(n)
       \/ Keep(StreamLost(n)) \/ Keep(Down(n))
       \/ Keep(Accept(n)) \/ Keep(Swallow(n)) \/ Keep(Decide(n))
  \/ Keep(DropStale)

FairS ==
  /\ \A c \in Conns : WF_allv(Keep(Dial(c)))
  /\ \A n \in Nodes, c \in Conns : WF_allv(Keep(Register(n, c))) /\ WF_allv(Keep(LearnClose(n, c)))
  /\ \A n \in Nodes :
       /\ WF_allv(StartOpenS(n)) /\ WF_allv(Keep(RecvProto(n))) /\ WF_allv(Keep(RecvAck(n))) /\ WF_allv(RecvResetS(n))
       /\ WF_allv(TimeoutS(n)) /\ WF_allv(Keep(StreamLost(n))) /\ WF_allv(Keep(Down(n)))
       /\ WF_allv(Keep(Accept(n))) /\ WF_allv(Keep(Decide(n)))
       /\ IF TickFair THEN WF_allv(Keep(Tick(n))) ELSE TRUE
  /\ WF_allv(Keep(DropStale))

SpecS == InitS /\ [][NextS]_allv /\ FairS

\* ---- Properties ----
\* S6 (P-16, §8.7.5 « deux ouvertures croisées sont indépendantes »): a failed session opening of n never closes another
\* connection that carries the peer's acknowledged session.
CrossedIndependent == ~gh.collat
\* S7 (P-16, review finding 1): an opening in flight, NewStream not returned included, always has a named connection,
\* the one it runs on.
OpeningNamed == \A n \in Nodes : out[n].st \in Opening => Named(n) = out[n].c
\* L3 (P-17): a node that admits its peer, holds a usable connection to it and has no session stream and none opening
\* does not stay so (otherwise every send to the peer waits for ever).
Stranded(n) == out[n].st = "none" /\ nxt[n].st = "none" /\ admit[n] /\ opens[n] < MaxOpens /\ HasConn(n)
NeverStranded == \A n \in Nodes : [](Stranded(n) => <>~Stranded(n))
=============================================================================
