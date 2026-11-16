------------------------------ MODULE PeerUpAck ------------------------------
(***************************************************************************)
(* Block PU1 (plan docs/plans/2026-11-16-pu1-peerup-after-acceptance.md,   *)
(* spec §8.7): a node announces a peer up only after the peer's            *)
(* acceptance (an ACK the peer writes on this very stream after its own    *)
(* admission and connection checks), never on the return of libp2p's      *)
(* NewStream, which on the cached-protocol path (optimistic                *)
(* multistream-select) proves nothing about the peer.                      *)
(*                                                                         *)
(* Two nodes a and b, symmetric. Connections are created on the wire       *)
(* (Dial) and registered by each side on its own time; at registration a   *)
(* side may close a connection at once (a late link it does not keep, or   *)
(* Admit refusing it): the other side may register it before it learns of  *)
(* the close (the trace found in block HP1r, reproducible on main). Each   *)
(* node opens one outbound stream carrying its session (out) and, while    *)
(* up, may move it to another connection (nxt, the relay-to-direct         *)
(* upgrade). Messages: "open" (the stream's multistream header), "proto"   *)
(* (the responder's multistream echo, libp2p-level only), "ack" (P-2),     *)
(* "reset".                                                                *)
(*                                                                         *)
(* Time (P-5): each opening stream has a clock (age, ticks since it was    *)
(* opened, NewStream and ACK together); at age = D the deadline fires,     *)
(* whatever is in flight. HonestTiming = TRUE is the environment           *)
(* assumption « the deadline is longer than any honest exchange »: the     *)
(* clock advances only while nothing of the stream is in flight and the    *)
(* responder is not deciding. HonestTiming = FALSE: the clock advances at  *)
(* any moment (slow negotiation, late ACK, a responder stuck in its        *)
(* handler: StuckNodes never decide, and fairness never forces them to).   *)
(*                                                                         *)
(* Rule operators, overridden by the mutants (mutants/PeerUpAckMutants):   *)
(*   AckReady       P-2: the responder ACKs only when it admits the peer   *)
(*                  and still holds the connection, never waiting for its  *)
(*                  own PeerUp (P-3, no deadlock);                         *)
(*   ReturnOpens    P-1: FALSE, NewStream's return is not an acceptance;   *)
(*   MoveAtReturn   P-6: FALSE, the move swaps streams only after the ACK; *)
(*   AckMatches     P-4: an ACK counts only for the stream it arrived on;  *)
(*   TimeoutReady   P-5: the deadline fires at age = D, unconditionally;   *)
(*   ResetOnReturn  P-5: FALSE, NewStream and the ACK share one budget.    *)
(***************************************************************************)
EXTENDS Naturals, FiniteSets

CONSTANTS
  Conns,         \* connection ids, e.g. {1, 2}
  RefuseOK,      \* connections a node may close at its registration (a late link it does not keep, Admit refusal)
  MaxCloses,     \* closes of a usable connection at will (Drop of one link, ClosePeer) over the run
  MaxFlips,      \* admission changes (Drop with ClosePeer, re-admission) over the run
  CachedInit,    \* possible initial values of cached: the peerstore lists the peer's protocol
  AdmitInit,     \* possible initial values of admit
  Silent,        \* TRUE: a node may accept a stream at the libp2p level and never answer (Byzantine)
  StuckNodes,    \* nodes whose handler accepts a stream and never decides (not even a reset)
  MaxOpens,      \* streams a node opens over the run (bound)
  D,             \* the opening deadline, in ticks (15 s)
  HonestTiming,  \* TRUE: the clock advances only while the stream is silent (see the header)
  TickFair       \* TRUE: time passes (weak fairness of Tick)

Nodes == {"a", "b"}
Other(n) == IF n = "a" THEN "b" ELSE "a"
NoConn == 0
NoStream == [st |-> "none", c |-> NoConn, s |-> 0, age |-> 0, tot |-> 0]
NoUp == [c |-> NoConn, s |-> 0]
Opening == {"neg", "await"}

VARIABLES
  live,      \* connections that exist on the wire
  closedBy,  \* [Conns -> SUBSET Nodes]: who closed each connection
  seen,      \* [Nodes -> SUBSET Conns]: connections a node knows closed (its own closes included)
  reg,       \* [Nodes -> [Conns -> {"no", "yes", "refused"}]]: registration of each connection
  admit,     \* [Nodes -> BOOLEAN]: the node admits its peer
  cached,    \* [Nodes -> BOOLEAN]: libp2p's lazy path (NewStream returns with no byte from the peer)
  out,       \* [Nodes -> stream]: the outbound sync stream that carries the session
  nxt,       \* [Nodes -> stream]: the replacement stream of a move
  msgs,      \* in-flight messages [t, by, to, c, s, ok]; by = the stream's opener, s its id
  pending,   \* [Nodes -> SUBSET [c, s]]: inbound streams whose handler has not decided yet
  up,        \* [Nodes -> BOOLEAN]: PeerUp announced, no PeerDown since
  upOn,      \* [Nodes -> [c, s]]: the stream whose ACK announced the peer (ghost)
  acc,       \* [Nodes -> SUBSET [c, s]]: streams (of the peer) the node sent a sound ACK on (ghost)
  opens,     \* [Nodes -> Nat]: streams opened, also their ids
  closes, flips, moves,
  lateFire   \* ghost: a deadline fired while something of its stream was in flight or being decided

vars == <<live, closedBy, seen, reg, admit, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

Usable(n, c) == c \in live /\ reg[n][c] = "yes" /\ c \notin seen[n]
Id(st) == [c |-> st.c, s |-> st.s]

\* The stream st of n is silent: nothing of it in flight, its handler not deciding.
Silenced(n, st) ==
  /\ ~\E m \in msgs : m.by = n /\ m.s = st.s
  /\ Id(st) \notin pending[Other(n)]

\* ---- Rule operators (the mutants override them) ----
AckReady(m, c) == admit[m] /\ Usable(m, c)
ReturnOpens == FALSE
MoveAtReturn == FALSE
AckMatches(st, m) == st.s = m.s
TimeoutReady(n, st) == st.age = D
ResetOnReturn == FALSE

Ids == [c: Conns, s: 1..MaxOpens]
Streams == [st: {"none", "neg", "await", "open"}, c: Conns \cup {NoConn}, s: 0..MaxOpens, age: 0..D, tot: 0..(2 * D)]
Msgs == [t: {"open", "proto", "ack", "reset"}, by: Nodes, to: Nodes, c: Conns, s: 1..MaxOpens, ok: BOOLEAN]
\* ok: for "ack", the ACK was sound (admitted, connection held; ghost); for "open", the opener negotiates
\* (waits for the echo); the echo is only modelled where it is awaited (on the cached path it changes nothing).

TypeOK ==
  /\ live \subseteq Conns
  /\ closedBy \in [Conns -> SUBSET Nodes]
  /\ seen \in [Nodes -> SUBSET Conns]
  /\ reg \in [Nodes -> [Conns -> {"no", "yes", "refused"}]]
  /\ admit \in [Nodes -> BOOLEAN] /\ cached \in [Nodes -> BOOLEAN]
  /\ out \in [Nodes -> Streams] /\ nxt \in [Nodes -> Streams]
  /\ msgs \subseteq Msgs
  /\ pending \in [Nodes -> SUBSET Ids]
  /\ up \in [Nodes -> BOOLEAN] /\ upOn \in [Nodes -> Ids \cup {NoUp}]
  /\ acc \in [Nodes -> SUBSET Ids]
  /\ opens \in [Nodes -> 0..MaxOpens]
  /\ closes \in 0..MaxCloses /\ flips \in 0..MaxFlips /\ moves \in Nat /\ lateFire \in BOOLEAN

Init ==
  /\ live = {}
  /\ closedBy = [c \in Conns |-> {}]
  /\ seen = [n \in Nodes |-> {}]
  /\ reg = [n \in Nodes |-> [c \in Conns |-> "no"]]
  /\ admit \in [Nodes -> AdmitInit]
  /\ cached \in [Nodes -> CachedInit]
  /\ out = [n \in Nodes |-> NoStream]
  /\ nxt = [n \in Nodes |-> NoStream]
  /\ msgs = {}
  /\ pending = [n \in Nodes |-> {}]
  /\ up = [n \in Nodes |-> FALSE]
  /\ upOn = [n \in Nodes |-> NoUp]
  /\ acc = [n \in Nodes |-> {}]
  /\ opens = [n \in Nodes |-> 0]
  /\ closes = 0 /\ flips = 0 /\ moves = 0 /\ lateFire = FALSE

CloseC(n, c) ==
  /\ closedBy' = [closedBy EXCEPT ![c] = @ \cup {n}]
  /\ seen' = [seen EXCEPT ![n] = @ \cup {c}]

\* ---- Connections ----
Dial(c) ==
  /\ c \notin live
  /\ live' = live \cup {c}
  /\ UNCHANGED <<closedBy, seen, reg, admit, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* A side registers a connection it has not learned closed. It may close it at once (a late link, Admit):
\* the stream handlers never run on it, so this side never ACKs on it.
Register(n, c) ==
  /\ c \in live /\ reg[n][c] = "no" /\ c \notin seen[n]
  /\ \/ /\ reg' = [reg EXCEPT ![n][c] = "yes"]
        /\ UNCHANGED <<closedBy, seen>>
     \/ /\ c \in RefuseOK
        /\ reg' = [reg EXCEPT ![n][c] = "refused"]
        /\ CloseC(n, c)
  /\ UNCHANGED <<live, admit, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* The close of a connection reaches the other side, some time later.
LearnClose(n, c) ==
  /\ closedBy[c] # {} /\ c \notin seen[n]
  /\ seen' = [seen EXCEPT ![n] = @ \cup {c}]
  /\ UNCHANGED <<live, closedBy, reg, admit, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

Close(n, c) ==
  /\ Usable(n, c) /\ closes < MaxCloses
  /\ closes' = closes + 1
  /\ CloseC(n, c)
  /\ UNCHANGED <<live, reg, admit, cached, out, nxt, msgs, pending, up, upOn, acc, opens, flips, moves, lateFire>>

\* Drop: the node stops admitting its peer and closes every connection to it (ClosePeer).
Drop(n) ==
  /\ admit[n] /\ flips < MaxFlips
  /\ admit' = [admit EXCEPT ![n] = FALSE]
  /\ flips' = flips + 1
  /\ closedBy' = [c \in Conns |-> IF Usable(n, c) THEN closedBy[c] \cup {n} ELSE closedBy[c]]
  /\ seen' = [seen EXCEPT ![n] = @ \cup {c \in Conns : Usable(n, c)}]
  /\ UNCHANGED <<live, reg, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, moves, lateFire>>

Readmit(n) ==
  /\ ~admit[n] /\ flips < MaxFlips
  /\ admit' = [admit EXCEPT ![n] = TRUE]
  /\ flips' = flips + 1
  /\ UNCHANGED <<live, closedBy, seen, reg, cached, out, nxt, msgs, pending, up, upOn, acc, opens, closes, moves, lateFire>>

\* ---- Opener ----
\* NewStream returned on stream st of the session (at once on the cached path, after the echo otherwise).
\* The clock keeps running: one budget for NewStream and the ACK (P-5), unless ResetOnReturn (mutant).
Returned(st, state) == [st |-> state, c |-> st.c, s |-> st.s, age |-> IF ResetOnReturn THEN 0 ELSE st.age, tot |-> st.tot]

Return(n, st) ==
  /\ out' = [out EXCEPT ![n] = Returned(st, IF ReturnOpens THEN "open" ELSE "await")]
  /\ IF ReturnOpens /\ ~up[n]
       THEN /\ up' = [up EXCEPT ![n] = TRUE]
            /\ upOn' = [upOn EXCEPT ![n] = Id(st)]
       ELSE UNCHANGED <<up, upOn>>

\* NewStream returned on a move's replacement stream st.
MoveReturn(n, st) ==
  IF MoveAtReturn
    THEN /\ out' = [out EXCEPT ![n] = [st |-> "open", c |-> st.c, s |-> st.s, age |-> 0, tot |-> 0]]
         /\ nxt' = [nxt EXCEPT ![n] = NoStream]
         /\ moves' = moves + 1
    ELSE /\ nxt' = [nxt EXCEPT ![n] = Returned(st, "await")]
         /\ UNCHANGED <<out, moves>>

NewSt(c, s) == [st |-> "neg", c |-> c, s |-> s, age |-> 0, tot |-> 0]

StartOpen(n) ==
  /\ out[n].st = "none" /\ nxt[n].st = "none" /\ admit[n] /\ opens[n] < MaxOpens
  /\ \E c \in Conns :
       /\ Usable(n, c)
       /\ LET s == opens[n] + 1 IN
            /\ opens' = [opens EXCEPT ![n] = s]
            /\ msgs' = msgs \cup {[t |-> "open", by |-> n, to |-> Other(n), c |-> c, s |-> s, ok |-> ~cached[n]]}
            /\ IF cached[n]
                 THEN Return(n, NewSt(c, s))
                 ELSE /\ out' = [out EXCEPT ![n] = NewSt(c, s)]
                      /\ UNCHANGED <<up, upOn>>
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, nxt, pending, acc, closes, flips, moves, lateFire>>

\* The move (the relay-to-direct upgrade): while up, the session's stream is replaced by one on another connection.
Move(n) ==
  /\ up[n] /\ out[n].st = "open" /\ nxt[n].st = "none" /\ opens[n] < MaxOpens
  /\ \E c \in Conns \ {out[n].c} :
       /\ Usable(n, c)
       /\ LET s == opens[n] + 1 IN
            /\ opens' = [opens EXCEPT ![n] = s]
            /\ msgs' = msgs \cup {[t |-> "open", by |-> n, to |-> Other(n), c |-> c, s |-> s, ok |-> ~cached[n]]}
            /\ IF cached[n]
                 THEN MoveReturn(n, NewSt(c, s))
                 ELSE /\ nxt' = [nxt EXCEPT ![n] = NewSt(c, s)]
                      /\ UNCHANGED <<out, moves>>
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, pending, up, upOn, acc, closes, flips, lateFire>>

Deliverable(m) == m.c \notin seen[m.to]
Echo(m) == IF m.ok THEN {[t |-> "proto", by |-> m.by, to |-> m.by, c |-> m.c, s |-> m.s, ok |-> TRUE]} ELSE {}
\* A stream's bytes arrive in order: the echo before the ACK or the reset.
EchoGone(m) == ~\E e \in msgs : e.t = "proto" /\ e.by = m.by /\ e.s = m.s

RecvProto(n) ==
  \E m \in msgs :
    /\ m.t = "proto" /\ m.to = n /\ Deliverable(m)
    /\ msgs' = msgs \ {m}
    /\ IF out[n].st = "neg" /\ out[n].s = m.s
         THEN Return(n, out[n]) /\ UNCHANGED <<nxt, moves>>
         ELSE IF nxt[n].st = "neg" /\ nxt[n].s = m.s
           THEN MoveReturn(n, nxt[n]) /\ UNCHANGED <<up, upOn>>
           ELSE UNCHANGED <<out, nxt, up, upOn, moves>>
    /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, pending, acc, opens, closes, flips, lateFire>>

RecvAck(n) ==
  \E m \in msgs :
    /\ m.t = "ack" /\ m.to = n /\ Deliverable(m) /\ EchoGone(m)
    /\ msgs' = msgs \ {m}
    /\ IF out[n].st = "await" /\ AckMatches(out[n], m)
         THEN /\ out' = [out EXCEPT ![n] = [st |-> "open", c |-> out[n].c, s |-> out[n].s, age |-> 0, tot |-> 0]]
              /\ IF up[n] THEN UNCHANGED <<up, upOn>>
                 ELSE /\ up' = [up EXCEPT ![n] = TRUE]
                      /\ upOn' = [upOn EXCEPT ![n] = Id(out[n])]
              /\ UNCHANGED <<nxt, moves>>
         ELSE IF nxt[n].st = "await" /\ AckMatches(nxt[n], m)
           THEN /\ out' = [out EXCEPT ![n] = [st |-> "open", c |-> nxt[n].c, s |-> nxt[n].s, age |-> 0, tot |-> 0]]
                /\ nxt' = [nxt EXCEPT ![n] = NoStream]
                /\ moves' = moves + 1
                /\ UNCHANGED <<up, upOn>>
           ELSE UNCHANGED <<out, nxt, up, upOn, moves>>
    /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, pending, acc, opens, closes, flips, lateFire>>

\* A reset of the session's stream is a refusal: the opener closes the connection (ClosePeer).
\* A reset of a move's stream only abandons the move: the session stays where it is, no event.
RecvReset(n) ==
  \E m \in msgs :
    /\ m.t = "reset" /\ m.to = n /\ Deliverable(m) /\ EchoGone(m)
    /\ msgs' = msgs \ {m}
    /\ IF out[n].st \in Opening /\ out[n].s = m.s
         THEN /\ out' = [out EXCEPT ![n] = NoStream]
              /\ CloseC(n, m.c)
              /\ UNCHANGED nxt
         ELSE IF nxt[n].st \in Opening /\ nxt[n].s = m.s
           THEN /\ nxt' = [nxt EXCEPT ![n] = NoStream]
                /\ UNCHANGED <<out, closedBy, seen>>
           ELSE UNCHANGED <<out, nxt, closedBy, seen>>
    /\ UNCHANGED <<live, reg, admit, cached, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* Time passes for an opening stream (P-5). Under HonestTiming only while the stream is silent.
TickOK(n, st) == st.st \in Opening /\ st.age < D /\ (HonestTiming => Silenced(n, st))
Ticked(st) == [st EXCEPT !.age = @ + 1, !.tot = @ + 1]
Tick(n) ==
  /\ \/ /\ TickOK(n, out[n])
        /\ out' = [out EXCEPT ![n] = Ticked(out[n])]
        /\ UNCHANGED nxt
     \/ /\ TickOK(n, nxt[n])
        /\ nxt' = [nxt EXCEPT ![n] = Ticked(nxt[n])]
        /\ UNCHANGED out
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, msgs, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* The deadline fires at age = D whatever is in flight (P-5). The session's stream: ClosePeer. A move's stream:
\* abandoned. Late bytes of the stream are then stale (its id is gone).
Timeout(n) ==
  \/ /\ out[n].st \in Opening /\ TimeoutReady(n, out[n])
     /\ out' = [out EXCEPT ![n] = NoStream]
     /\ lateFire' = (lateFire \/ ~Silenced(n, out[n]))
     /\ IF out[n].c \in seen[n] THEN UNCHANGED <<closedBy, seen>> ELSE CloseC(n, out[n].c)
     /\ UNCHANGED <<live, reg, admit, cached, nxt, msgs, pending, up, upOn, acc, opens, closes, flips, moves>>
  \/ /\ nxt[n].st \in Opening /\ TimeoutReady(n, nxt[n])
     /\ nxt' = [nxt EXCEPT ![n] = NoStream]
     /\ lateFire' = (lateFire \/ ~Silenced(n, nxt[n]))
     /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, out, msgs, pending, up, upOn, acc, opens, closes, flips, moves>>

\* A stream whose connection is known closed ends.
StreamLost(n) ==
  /\ \/ /\ out[n].st # "none" /\ out[n].c \in seen[n]
        /\ out' = [out EXCEPT ![n] = NoStream]
        /\ UNCHANGED nxt
     \/ /\ nxt[n].st # "none" /\ nxt[n].c \in seen[n]
        /\ nxt' = [nxt EXCEPT ![n] = NoStream]
        /\ UNCHANGED out
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, msgs, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* PeerDown: no stream, none opening, no usable connection.
Down(n) ==
  /\ up[n] /\ out[n].st = "none" /\ nxt[n].st = "none"
  /\ ~\E c \in Conns : Usable(n, c)
  /\ up' = [up EXCEPT ![n] = FALSE]
  /\ upOn' = [upOn EXCEPT ![n] = NoUp]
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, out, nxt, msgs, pending, acc, opens, closes, flips, moves, lateFire>>

\* ---- Responder ----
\* libp2p accepts the stream on a registered connection the node holds (multistream echo); the
\* handler then decides (Decide). A connection closed at registration never runs a handler.
Accept(r) ==
  \E m \in msgs :
    /\ m.t = "open" /\ m.to = r /\ reg[r][m.c] = "yes" /\ m.c \notin seen[r]
    /\ msgs' = (msgs \ {m}) \cup Echo(m)
    /\ pending' = [pending EXCEPT ![r] = @ \cup {[c |-> m.c, s |-> m.s]}]
    /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, out, nxt, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* A Byzantine responder: libp2p echoes the protocol, the handler never answers.
Swallow(r) ==
  /\ Silent
  /\ \E m \in msgs :
       /\ m.t = "open" /\ m.to = r /\ reg[r][m.c] = "yes" /\ m.c \notin seen[r]
       /\ msgs' = (msgs \ {m}) \cup Echo(m)
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, out, nxt, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

\* P-2: ACK once admitted and the connection held; P-3: never waits for the node's own PeerUp.
\* Not admitted: reset. Connection gone: nothing reaches the opener. A stuck node never decides.
Decide(r) ==
  /\ r \notin StuckNodes
  /\ \E p \in pending[r] :
       /\ AckReady(r, p.c) \/ ~admit[r] \/ ~Usable(r, p.c)
       /\ pending' = [pending EXCEPT ![r] = @ \ {p}]
       /\ IF AckReady(r, p.c)
            THEN LET ok == admit[r] /\ Usable(r, p.c) IN
                 /\ msgs' = msgs \cup {[t |-> "ack", by |-> Other(r), to |-> Other(r), c |-> p.c, s |-> p.s, ok |-> ok]}
                 /\ acc' = IF ok THEN [acc EXCEPT ![r] = @ \cup {p}] ELSE acc
            ELSE IF Usable(r, p.c)
              THEN /\ msgs' = msgs \cup {[t |-> "reset", by |-> Other(r), to |-> Other(r), c |-> p.c, s |-> p.s, ok |-> TRUE]}
                   /\ UNCHANGED acc
              ELSE UNCHANGED <<msgs, acc>>
  /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, out, nxt, up, upOn, opens, closes, flips, moves, lateFire>>

\* Bytes for a connection the receiver knows closed never arrive; bytes of a stream the opener
\* abandoned (deadline, move given up) are discarded.
Stale(m) ==
  \/ m.c \in seen[m.to]
  \/ m.t \in {"proto", "ack", "reset"} /\ m.s # out[m.to].s /\ m.s # nxt[m.to].s
DropStale ==
  \E m \in msgs :
    /\ Stale(m)
    /\ msgs' = msgs \ {m}
    /\ UNCHANGED <<live, closedBy, seen, reg, admit, cached, out, nxt, pending, up, upOn, acc, opens, closes, flips, moves, lateFire>>

Next ==
  \/ \E c \in Conns : Dial(c)
  \/ \E n \in Nodes, c \in Conns : Register(n, c) \/ LearnClose(n, c) \/ Close(n, c)
  \/ \E n \in Nodes :
       \/ Drop(n) \/ Readmit(n) \/ StartOpen(n) \/ Move(n)
       \/ RecvProto(n) \/ RecvAck(n) \/ RecvReset(n) \/ Tick(n) \/ Timeout(n) \/ StreamLost(n) \/ Down(n)
       \/ Accept(n) \/ Swallow(n) \/ Decide(n)
  \/ DropStale

Fairness ==
  /\ \A c \in Conns : WF_vars(Dial(c))
  /\ \A n \in Nodes, c \in Conns : WF_vars(Register(n, c)) /\ WF_vars(LearnClose(n, c))
  /\ \A n \in Nodes :
       /\ WF_vars(StartOpen(n)) /\ WF_vars(RecvProto(n)) /\ WF_vars(RecvAck(n)) /\ WF_vars(RecvReset(n))
       /\ WF_vars(Timeout(n)) /\ WF_vars(StreamLost(n)) /\ WF_vars(Down(n))
       /\ WF_vars(Accept(n)) /\ WF_vars(Decide(n))
       /\ IF TickFair THEN WF_vars(Tick(n)) ELSE TRUE
  /\ WF_vars(DropStale)

Spec == Init /\ [][Next]_vars /\ Fairness

\* ---- Properties ----
\* S1 (P-1, P-2, P-4): a PeerUp is backed by the peer's sound ACK of the very stream that announced it.
UpAfterAccept == \A n \in Nodes : up[n] => upOn[n] \in acc[Other(n)]
\* S2 (P-6, P-4): the session's stream is only ever a stream the peer accepted (moves included).
SessionOnAccepted == \A n \in Nodes : out[n].st = "open" => Id(out[n]) \in acc[Other(n)]
\* S3: a connection the peer closed at registration, or has not registered, never announces it
\* (« no PeerUp on either side » for a connection closed at once).
NoUpOnRefused == \A n \in Nodes : up[n] => reg[Other(n)][upOn[n].c] = "yes"
\* S4 (P-5): an opening stream, NewStream and ACK together, never outlives the deadline.
Budget == \A n \in Nodes : out[n].tot <= D /\ nxt[n].tot <= D
\* S5 (P-2, a responder stuck in its handler): it never gets its opener announced.
NoUpFromStuck == \A n \in Nodes : Other(n) \in StuckNodes => ~up[n]

\* L1 (P-3, honest configuration): both sides come up; nobody waits for the other's PeerUp.
BothUp == <>(up["a"] /\ up["b"])
\* L2 (P-5): an opening stream always resolves (ACK, reset, connection lost or deadline).
OpenResolves == \A n \in Nodes : [](out[n].st \in Opening => <>(out[n].st \notin Opening))
MoveResolves == \A n \in Nodes : [](nxt[n].st \in Opening => <>(nxt[n].st \notin Opening))
=============================================================================
