-------------------------- MODULE OISingleInstance --------------------------
(***************************************************************************)
(* Block OI (W41, OI2): one running Inkwell per profile, and the hand-off  *)
(* of a second launch's arguments to it.                                   *)
(* Plan: docs/plans/2026-11-11-oi-single-instance-open.md, section 6.      *)
(*                                                                         *)
(* Mutants and witnesses: mutants/OISingleInstanceMutants.tla (EXTENDS     *)
(* this module); configurations: README.md, section OISingleInstance.      *)
(*                                                                         *)
(* Each element of Launch is one process started on the same profile      *)
(* directory. The kernel objects are abstracted:                           *)
(*   lock  - the exclusive flock (LockFileEx on Windows) on                *)
(*           <profile>/instance.lock; the kernel releases it when its      *)
(*           holder dies, which Crash models (rule ReleaseOnCrash);        *)
(*   sock  - the Unix-domain socket file: absent, stale (a file with no    *)
(*           listener: connect fails) or live (bound and listening; a      *)
(*           connect succeeds and waits in the backlog even before the     *)
(*           owner accepts);                                               *)
(*   inbox - the requests a primary has queued (a history: it never       *)
(*           shrinks, also after the primary died);                        *)
(*   opened - what a primary has applied (shown to the person): its own    *)
(*           arguments and the queued requests ("applied": shown, or said  *)
(*           why not, in the window). A request is acknowledged once       *)
(*           received and queued by a primary ready to show it (contract   *)
(*           simplified 2026-11-12); a crash of that primary before it     *)
(*           applied the request loses it (accepted residual). A failed    *)
(*           start is a Crash here: nothing was queued.                    *)
(*   gen, sentOn, win, reports, done, dying - the open stream between the  *)
(*           primary's Go side and its window, one owner at a time by      *)
(*           generation (amendment 2026-11-13): a new connection           *)
(*           supersedes the old one; Go keeps each request until the       *)
(*           window of the generation holding it reports it handled.       *)
(* Time is abstract: a client's deadline (5 s, plan section 4.3) is the    *)
(* Timeout action, always allowed while no reply arrived; giving up is     *)
(* only allowed while the lock holder is hung (GiveUp), which is what the  *)
(* 5 s deadline is for: a healthy primary binds in microseconds and        *)
(* answers in milliseconds.                                                *)
(***************************************************************************)
EXTENDS Naturals, FiniteSets

CONSTANTS
  Launch,      \* the launches on one profile, model values, e.g. {p1, p2, p3}
  None,        \* model value: no process
  MaxCrashes,  \* crashes allowed in one behaviour
  Hangs,       \* BOOLEAN: may a primary stop serving while it keeps the lock
  MaxGen       \* connections of the open stream in one primary's life (1: no reconnection)

ASSUME None \notin Launch
ASSUME MaxCrashes \in Nat
ASSUME Hangs \in BOOLEAN
ASSUME MaxGen \in Nat \ {0}

VARIABLES pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck,
  gen, sentOn, win, reports, done, dying

\* The open stream between a primary's Go side and its window: the generation of its current
\* connection (0: none yet), the generation each request was delivered to (0: not delivered, or given
\* back), what the current window holds, the handled reports on their way to Go with the generation
\* of the delivery they answer, what Go removed from its queue, the superseded streams not ended yet.
wv == <<gen, sentOn, win, reports, done, dying>>

vars == <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck, wv>>

Absent == [state |-> "absent", owner |-> None]
Stale == [state |-> "stale", owner |-> None]
Live(p) == [state |-> "live", owner |-> p]

PrimaryStates == {"unlink", "bind", "own", "serve", "closing", "closed"}
SecondStates == {"dial", "unlinkStale", "wait", "backoff"}
Final == {"done", "exit0", "gaveUp", "crashed"}
States == {"start"} \cup PrimaryStates \cup SecondStates \cup Final
\* A launch has settled when it serves (primary), is shutting down after serving, or has ended.
Settled == Final \cup {"serve", "closing", "closed"}
Replies == {"none", "ack", "closing", "lost"}

(***************************************************************************)
(* RULES. Each is one decision of the plan; a mutant configuration         *)
(* overrides exactly one of them ("Rule <- Mutant").                       *)
(***************************************************************************)
\* D-LOCK: the primary is whoever takes the profile lock, never "whoever finds no socket".
MayLead(p) == lock = None
\* D-ORDER: the identity directory is opened only after the lock is held.
IdentityAtStart == FALSE
\* D-ACK: a request is acknowledged once received and queued; from then on it is the primary's.
Enqueue(p, q) == inbox[p] \cup {q}
\* D-HOLD: the lock is never released explicitly; the process exit releases it, after the identity is closed.
ReleaseOnClose == FALSE
\* D-KERNEL: a kernel lock, released when its holder dies (no pid file, no staleness heuristics).
ReleaseOnCrash(p) == IF lock = p THEN None ELSE lock
\* D-ACK: a second launch exits 0 only after reading the acknowledgement.
WaitsForReply == TRUE
\* D-CLOSING: "closing" means "wait for the lock and become the primary", never "give up".
GiveUpOnClosing == FALSE
\* D-UNLINK: only the lock holder removes a socket file.
UnlinksStale == FALSE
\* D-RETRY: every retry starts by trying the lock again.
AfterBackoff == "start"
\* D-DRAIN: the window keeps applying every request it holds.
ToApply(p) == win[p]
\* D-DRAIN at close: the process closes only once Go's queue is empty, i.e. the window reported
\* handled (shown, or said why not) everything received, not merely once everything was delivered.
DrainedBeforeClose(p) == inbox[p] \ done[p] = {}
\* D-OWNER: a handled report frees a request only from the generation that currently holds it; a
\* report of a superseded stream, or about a request not held, is ignored.
AcceptsReport(p, m) == m[1] \in inbox[p] \ done[p] /\ sentOn[p][m[1]] = m[2]
\* D-OWNER: when a stream of generation g ends, only the requests delivered to g are given back.
GiveBack(p, g) == [q \in Launch |-> IF sentOn[p][q] = g THEN 0 ELSE sentOn[p][q]]
EndGiveBack(p, g) == GiveBack(p, g)

TypeOK ==
  /\ pc \in [Launch -> States]
  /\ lock \in Launch \cup {None}
  /\ sock \in {Absent, Stale} \cup {Live(p) : p \in Launch}
  /\ inbox \in [Launch -> SUBSET Launch]
  /\ conn \in [Launch -> Launch \cup {None}]
  /\ reply \in [Launch -> Replies]
  /\ opened \in [Launch -> SUBSET Launch]
  /\ identity \subseteq Launch
  /\ crashes \in 0..MaxCrashes
  /\ hung \subseteq Launch
  /\ gen \in [Launch -> 0..MaxGen]
  /\ sentOn \in [Launch -> [Launch -> 0..MaxGen]]
  /\ win \in [Launch -> SUBSET Launch]
  /\ reports \in [Launch -> SUBSET (Launch \X (1..MaxGen))]
  /\ done \in [Launch -> SUBSET Launch]
  /\ dying \in [Launch -> SUBSET (1..MaxGen)]
  /\ stuck \subseteq Launch
  /\ why \in [Launch -> {"none", "hung", "other"}]

Init ==
  /\ pc = [p \in Launch |-> "start"]
  /\ lock = None
  /\ sock \in {Absent, Stale}          \* a previous run may have crashed and left its socket file
  /\ inbox = [p \in Launch |-> {}]
  /\ conn = [p \in Launch |-> None]
  /\ reply = [p \in Launch |-> "none"]
  /\ opened = [p \in Launch |-> {}]
  /\ identity = {}
  /\ crashes = 0
  /\ hung = {}
  /\ gen = [p \in Launch |-> 0]
  /\ sentOn = [p \in Launch |-> [q \in Launch |-> 0]]
  /\ win = [p \in Launch |-> {}]
  /\ reports = [p \in Launch |-> {}]
  /\ done = [p \in Launch |-> {}]
  /\ dying = [p \in Launch |-> {}]
  /\ stuck = {}
  /\ why = [p \in Launch |-> "none"]

Listening(p) ==
  /\ sock = Live(p)
  /\ pc[p] \in {"own", "serve"}
  /\ p \notin hung

Waiting(q, p) == pc[q] = "wait" /\ conn[q] = p /\ reply[q] = "none"

\* Every connection still waiting on p is reset when p goes away.
Reset(p) == [q \in Launch |-> IF Waiting(q, p) THEN "lost" ELSE reply[q]]

(* ---- every launch starts here, and comes back here after each failed try ---- *)
TryLock(p) ==
  /\ pc[p] = "start"
  /\ IF MayLead(p)
       THEN /\ lock' = p
            /\ pc' = [pc EXCEPT ![p] = "unlink"]
       ELSE /\ UNCHANGED lock
            /\ pc' = [pc EXCEPT ![p] = "dial"]
  /\ identity' = IF IdentityAtStart THEN identity \cup {p} ELSE identity
  /\ UNCHANGED <<sock, inbox, conn, reply, opened, crashes, hung, why, wv, stuck>>

(* ---- primary ---- *)
\* Holding the lock, remove whatever socket file a crashed predecessor left.
Unlink(p) ==
  /\ pc[p] = "unlink"
  /\ sock' = Absent
  /\ pc' = [pc EXCEPT ![p] = "bind"]
  /\ UNCHANGED <<lock, inbox, conn, reply, opened, identity, crashes, hung, why, wv, stuck>>

\* Listen before opening anything else, so that a second launch never waits on a slow start.
Bind(p) ==
  /\ pc[p] = "bind"
  /\ sock' = Live(p)
  /\ pc' = [pc EXCEPT ![p] = "own"]
  /\ UNCHANGED <<lock, inbox, conn, reply, opened, identity, crashes, hung, why, wv, stuck>>

\* Open the identity, the project and the network; the primary's own arguments are applied.
OpenIdentity(p) ==
  /\ pc[p] = "own"
  /\ p \notin hung
  /\ identity' = identity \cup {p}
  /\ opened' = [opened EXCEPT ![p] = @ \cup {p}]
  /\ pc' = [pc EXCEPT ![p] = "serve"]
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, crashes, hung, why, wv, stuck>>

\* Read one request, queue it, acknowledge it. Only a primary ready to show a file queues ("serve":
\* identity, project and network up); while it opens ("own") its launches are told "busy", which
\* the per-try Timeout stands for.
Accept(p, q) ==
  /\ Listening(p)
  /\ pc[p] = "serve"
  /\ Waiting(q, p)
  /\ inbox' = [inbox EXCEPT ![p] = Enqueue(p, q)]
  /\ reply' = [reply EXCEPT ![q] = "ack"]
  /\ UNCHANGED <<pc, lock, sock, conn, opened, identity, crashes, hung, why, wv, stuck>>

(* ---- the open stream between Go and the window (ADR 0009: a Wails stream, no port) ---- *)
\* The window connects to the open stream once the primary serves (or while it closes: a window
\* closed before its page connected still gets what was acknowledged, within the bounded wait).
Connect(p) ==
  /\ pc[p] \in {"serve", "closing"}
  /\ gen[p] = 0
  /\ p \notin hung
  /\ gen' = [gen EXCEPT ![p] = 1]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck,
                 sentOn, win, reports, done, dying>>

\* The window connects again (a page reload, a dropped stream; not fair): the new connection
\* supersedes the old one, which Go closes. What the old one was given and had not reported is given
\* back, to be sent on the new one; the window starts from a fresh state (what the old one held is
\* discarded); its reports still on their way may arrive later. The old stream's handler returns
\* later (EndStream). A closing window does not reload.
Reconnect(p) ==
  /\ pc[p] = "serve"
  /\ gen[p] \in 1..(MaxGen - 1)
  /\ p \notin hung \cup stuck
  /\ gen' = [gen EXCEPT ![p] = @ + 1]
  /\ sentOn' = [sentOn EXCEPT ![p] = GiveBack(p, gen[p])]
  /\ win' = [win EXCEPT ![p] = {}]
  /\ dying' = [dying EXCEPT ![p] = @ \cup {gen[p]}]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck,
                 reports, done>>

\* The handler of a superseded stream returns (its deferred clean-up), possibly after the new stream
\* got requests.
EndStream(p) ==
  /\ p \notin hung
  /\ \E g \in dying[p] :
       /\ dying' = [dying EXCEPT ![p] = @ \ {g}]
       /\ sentOn' = [sentOn EXCEPT ![p] = EndGiveBack(p, g)]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck,
                 gen, win, reports, done>>

\* Go sends a queued request not delivered to the current window; it keeps it (and its slot) until
\* the window reports it handled.
Send(p) ==
  /\ pc[p] \in {"serve", "closing"}
  /\ gen[p] > 0
  /\ p \notin hung \cup stuck
  /\ \E q \in inbox[p] \ done[p] :
       /\ sentOn[p][q] = 0
       /\ sentOn' = [sentOn EXCEPT ![p][q] = gen[p]]
       /\ win' = [win EXCEPT ![p] = @ \cup {q}]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck,
                 gen, reports, done, dying>>

\* The window handles a request it holds (shows the file, or its toast) and reports it, with the
\* generation of its stream; while the process closes it still does (the drain).
Apply(p) ==
  /\ pc[p] \in {"serve", "closing"}
  /\ p \notin hung \cup stuck
  /\ \E q \in ToApply(p) :
       /\ opened' = [opened EXCEPT ![p] = @ \cup {q}]
       /\ win' = [win EXCEPT ![p] = @ \ {q}]
       /\ reports' = [reports EXCEPT ![p] = @ \cup {<<q, gen[p]>>}]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, identity, crashes, hung, why, stuck,
                 gen, sentOn, done, dying>>

\* Go reads a handled report: it removes the request only if it accepts the report (rule D-OWNER).
Deliver(p) ==
  /\ pc[p] \in {"serve", "closing"}
  /\ p \notin hung
  /\ \E m \in reports[p] :
       /\ reports' = [reports EXCEPT ![p] = @ \ {m}]
       /\ done' = IF AcceptsReport(p, m) THEN [done EXCEPT ![p] = @ \cup {m[1]}] ELSE done
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, stuck,
                 gen, sentOn, win, dying>>

\* The person closes the window (not fair: a primary may serve forever): no request is acknowledged
\* from now on; the window still handles what it was given.
CloseStart(p) ==
  /\ pc[p] = "serve"
  /\ p \notin hung
  /\ pc' = [pc EXCEPT ![p] = "closing"]
  /\ lock' = IF ReleaseOnClose THEN None ELSE lock
  /\ UNCHANGED <<sock, inbox, conn, reply, opened, identity, crashes, hung, why, wv, stuck>>

\* While closing, the listener stays open and answers "closing" to every new request.
\* The launch reads "closing" in the same step: it backs off to wait for the lock (rule D-CLOSING).
ReplyClosing(p, q) ==
  /\ pc[p] = "closing"
  /\ sock = Live(p)
  /\ Waiting(q, p)
  /\ conn' = [conn EXCEPT ![q] = None]
  /\ IF GiveUpOnClosing
       THEN /\ pc' = [pc EXCEPT ![q] = "gaveUp"]
            /\ why' = [why EXCEPT ![q] = "other"]
            /\ identity' = identity \ {q}
       ELSE /\ pc' = [pc EXCEPT ![q] = "backoff"]
            /\ UNCHANGED <<why, identity>>
  /\ UNCHANGED <<lock, sock, inbox, reply, opened, crashes, hung, wv, stuck>>

\* The process closes once the window handled everything, or once the bounded wait (2 s) gave up on a
\* window that cannot answer (stuck: the residual).
CloseIdentity(p) ==
  /\ pc[p] = "closing"
  /\ DrainedBeforeClose(p) \/ p \in stuck
  /\ identity' = identity \ {p}
  /\ pc' = [pc EXCEPT ![p] = "closed"]
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, opened, crashes, hung, why, wv, stuck>>

\* Fault: the window stops answering (it never reports handled any more); not fair.
Stuck(p) ==
  /\ pc[p] \in {"serve", "closing"}
  /\ p \notin stuck
  /\ stuck' = stuck \cup {p}
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, wv>>

\* Process exit: the kernel releases the lock; the socket file may stay (worst case: stale).
Exit(p) ==
  /\ pc[p] = "closed"
  /\ pc' = [pc EXCEPT ![p] = "done"]
  /\ lock' = IF lock = p THEN None ELSE lock
  /\ sock' = IF sock = Live(p) THEN Stale ELSE sock
  /\ reply' = Reset(p)
  /\ UNCHANGED <<inbox, conn, opened, identity, crashes, hung, why, wv, stuck>>

(* ---- second launch ---- *)
Dial(q) ==
  /\ pc[q] = "dial"
  /\ IF sock.state = "live"
       THEN /\ conn' = [conn EXCEPT ![q] = sock.owner]
            /\ reply' = [reply EXCEPT ![q] = "none"]
            /\ pc' = [pc EXCEPT ![q] = IF WaitsForReply THEN "wait" ELSE "exit0"]
       ELSE /\ UNCHANGED <<conn, reply>>
            /\ pc' = [pc EXCEPT ![q] = IF UnlinksStale THEN "unlinkStale" ELSE "backoff"]
  /\ UNCHANGED <<lock, sock, inbox, opened, identity, crashes, hung, why, wv, stuck>>

\* Only reachable under the mutant SecondUnlinks: removes whatever is there by then.
UnlinkStale(q) ==
  /\ pc[q] = "unlinkStale"
  /\ sock' = Absent
  /\ pc' = [pc EXCEPT ![q] = "backoff"]
  /\ UNCHANGED <<lock, inbox, conn, reply, opened, identity, crashes, hung, why, wv, stuck>>

Answer(q) ==
  /\ pc[q] = "wait"
  /\ reply[q] # "none"
  /\ conn' = [conn EXCEPT ![q] = None]
  /\ CASE reply[q] = "ack" ->
            /\ pc' = [pc EXCEPT ![q] = "exit0"]
            /\ identity' = identity \ {q}
            /\ UNCHANGED why
       [] reply[q] = "closing" /\ GiveUpOnClosing ->
            /\ pc' = [pc EXCEPT ![q] = "gaveUp"]
            /\ why' = [why EXCEPT ![q] = "other"]
            /\ identity' = identity \ {q}
       [] OTHER ->
            /\ pc' = [pc EXCEPT ![q] = "backoff"]
            /\ UNCHANGED <<why, identity>>
  /\ reply' = [reply EXCEPT ![q] = "none"]   \* read: no stale value distinguishes states
  /\ UNCHANGED <<lock, sock, inbox, opened, crashes, hung, wv, stuck>>

\* The per-try deadline: no answer yet (slow or hung server, or the real primary's "busy" after its
\* hold, while the window has not applied the request; the retry waits on the same request).
Timeout(q) ==
  /\ pc[q] = "wait"
  /\ reply[q] = "none"
  /\ conn' = [conn EXCEPT ![q] = None]
  /\ pc' = [pc EXCEPT ![q] = "backoff"]
  /\ UNCHANGED <<lock, sock, inbox, reply, opened, identity, crashes, hung, why, wv, stuck>>

Backoff(q) ==
  /\ pc[q] = "backoff"
  /\ pc' = [pc EXCEPT ![q] = AfterBackoff]
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why, wv, stuck>>

\* The 5 s deadline: only while the lock holder does not answer.
GiveUp(q) ==
  /\ pc[q] = "backoff"
  /\ lock \in hung
  /\ pc' = [pc EXCEPT ![q] = "gaveUp"]
  /\ why' = [why EXCEPT ![q] = "hung"]
  /\ identity' = identity \ {q}
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, opened, crashes, hung, wv, stuck>>

(* ---- faults ---- *)
Hang(p) ==
  /\ Hangs
  /\ pc[p] \in {"own", "serve"}
  /\ p \notin hung
  /\ hung' = hung \cup {p}
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, why, wv, stuck>>

Crash(p) ==
  /\ crashes < MaxCrashes
  /\ pc[p] \notin Final
  /\ pc' = [pc EXCEPT ![p] = "crashed"]
  /\ crashes' = crashes + 1
  /\ lock' = ReleaseOnCrash(p)
  /\ identity' = identity \ {p}
  /\ sock' = IF sock = Live(p) THEN Stale ELSE sock
  /\ hung' = hung \ {p}
  /\ reply' = Reset(p)
  /\ conn' = [conn EXCEPT ![p] = None]
  /\ UNCHANGED <<inbox, opened, why, wv, stuck>>

Next ==
  \E p \in Launch :
    \/ TryLock(p) \/ Unlink(p) \/ Bind(p) \/ OpenIdentity(p)
    \/ Connect(p) \/ Reconnect(p) \/ EndStream(p) \/ Send(p) \/ Apply(p) \/ Deliver(p)
    \/ CloseStart(p) \/ CloseIdentity(p) \/ Exit(p)
    \/ Dial(p) \/ UnlinkStale(p) \/ Answer(p) \/ Timeout(p) \/ Backoff(p) \/ GiveUp(p)
    \/ Hang(p) \/ Crash(p) \/ Stuck(p)
    \/ \E q \in Launch : Accept(p, q) \/ ReplyClosing(p, q)

(***************************************************************************)
(* FAIRNESS: three assumptions per launch, each a fact of the real system  *)
(* (README.md, OISingleInstance, "Fairness"). No fairness on CloseStart    *)
(* (the person decides), Hang or Crash (faults).                           *)
(*                                                                         *)
(* Few terms over grouped actions, not one per action: TLC puts           *)
(* Fairness /\ ~Property in disjunctive normal form, and every SF term     *)
(* (<>[]~ENABLED A \/ []<>A) doubles it. One SF per (p, q) pair and per    *)
(* GiveUp was 21 SF terms for 3 launches, i.e. 2^21 disjuncts per branch:  *)
(* TLC ran out of memory in LNConj.toDNF before exploring a state.         *)
(***************************************************************************)
\* The launch's own program steps. They are enabled in disjoint control states, and each one,
\* once enabled, stays enabled until the launch takes a step, so weak fairness on their
\* disjunction is weak fairness on each of them.
Step(p) ==
  \/ TryLock(p) \/ Unlink(p) \/ Bind(p) \/ OpenIdentity(p)
  \/ Connect(p) \/ Send(p) \/ Apply(p) \/ Deliver(p)
  \/ CloseIdentity(p) \/ Exit(p)
  \/ Dial(p) \/ UnlinkStale(p) \/ Answer(p) \/ Timeout(p) \/ Backoff(p)

\* Launch q's connection gets its answer from the primary it reached (conn[q] is that one).
Answered(q) == \E p \in Launch : Accept(p, q) \/ ReplyClosing(p, q)

Fairness ==
  \A p \in Launch :
    \* A1: the operating system keeps running every live process that is not hung.
    /\ WF_vars(Step(p))
    \* A2: a primary that keeps answering answers a client that keeps reconnecting (strong:
    \*     the client's per-try timeout disables the answer between tries).
    /\ SF_vars(Answered(p))
    \* A3: a client whose retries keep meeting a hung lock holder reaches its 5 s deadline
    \*     (strong: each retry disables it until the next back-off).
    /\ SF_vars(GiveUp(p))

Spec == Init /\ [][Next]_vars /\ Fairness

(***************************************************************************)
(* PROPERTIES                                                              *)
(***************************************************************************)
\* S1: never two processes on one identity key (the defect this block removes).
MutualExclusion == Cardinality(identity) <= 1
\* S2: the identity is open only by the lock holder.
IdentityNeedsLock == \A p \in identity : lock = p
\* S3: a live socket belongs to the lock holder.
OnlyHolderListens == sock.state = "live" => sock.owner = lock
\* S4: a second launch that exits 0 was queued by a primary.
ExitOkImpliesQueued == \A q \in Launch : pc[q] = "exit0" => \E p \in Launch : q \in inbox[p]
\* S5: a launch gives up only because the holder does not answer.
GaveUpJustified == \A q \in Launch : why[q] \in {"none", "hung"}
\* S6: one owner at a time: whatever the current window holds, Go holds too, as delivered to that
\* window's generation (so Go's in-flight count is the window's, and nothing it holds is sent twice).
SlotsAgree ==
  \A p, q \in Launch :
    q \in win[p] => q \in inbox[p] \ done[p] /\ sentOn[p][q] = gen[p]
\* S7: Go removes a request only once a window has shown it (or said why not).
RemovedWasShown == \A p \in Launch : done[p] \subseteq opened[p]

\* L1: every launch settles: it serves, or hands off, or gives up on a hung holder, or crashed.
Settles == \A p \in Launch : (pc[p] = "start") ~> (pc[p] \in Settled \/ p \in hung)
\* L2: a request a primary received is eventually handled by its window (shown, or said why not)
\* unless that primary crashes, hangs, or its window stops answering (the bounded wait at close then
\* gives up: the residual); no escape for a primary that closes (rule D-DRAIN at close).
AckedApplied ==
  \A p, q \in Launch :
    (q \in inbox[p]) ~> (q \in opened[p] \/ pc[p] = "crashed" \/ p \in hung \/ p \in stuck)
\* L3: no slot stays taken: every request a primary received leaves Go's queue, whatever the
\* reconnections, unless that primary crashes, hangs, or its window stops answering.
NoStrandedSlot ==
  \A p, q \in Launch :
    (q \in inbox[p]) ~> (q \in done[p] \/ pc[p] = "crashed" \/ p \in hung \/ p \in stuck)
=============================================================================
