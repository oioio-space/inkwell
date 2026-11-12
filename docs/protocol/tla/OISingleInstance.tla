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
(*   inbox - the requests a primary has queued and acknowledged (a         *)
(*           history: it never shrinks, also after the primary died);      *)
(*   opened - what a primary has applied (shown to the person): its own    *)
(*           arguments and the queued requests.                            *)
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
  Hangs        \* BOOLEAN: may a primary stop serving while it keeps the lock

ASSUME None \notin Launch
ASSUME MaxCrashes \in Nat
ASSUME Hangs \in BOOLEAN

VARIABLES pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why

vars == <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why>>

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
\* D-ACK: the acknowledgement is sent only once the request is queued.
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
\* D-DRAIN: the window keeps taking every queued request it has not applied yet.
ToApply(p) == inbox[p] \ opened[p]

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
  /\ UNCHANGED <<sock, inbox, conn, reply, opened, crashes, hung, why>>

(* ---- primary ---- *)
\* Holding the lock, remove whatever socket file a crashed predecessor left.
Unlink(p) ==
  /\ pc[p] = "unlink"
  /\ sock' = Absent
  /\ pc' = [pc EXCEPT ![p] = "bind"]
  /\ UNCHANGED <<lock, inbox, conn, reply, opened, identity, crashes, hung, why>>

\* Listen before opening anything else, so that a second launch never waits on a slow start.
Bind(p) ==
  /\ pc[p] = "bind"
  /\ sock' = Live(p)
  /\ pc' = [pc EXCEPT ![p] = "own"]
  /\ UNCHANGED <<lock, inbox, conn, reply, opened, identity, crashes, hung, why>>

\* Open the identity, the project and the network; the primary's own arguments are applied.
OpenIdentity(p) ==
  /\ pc[p] = "own"
  /\ p \notin hung
  /\ identity' = identity \cup {p}
  /\ opened' = [opened EXCEPT ![p] = @ \cup {p}]
  /\ pc' = [pc EXCEPT ![p] = "serve"]
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, crashes, hung, why>>

\* Read one request, queue it, acknowledge it (queued requests wait for the window: "own" accepts too).
Accept(p, q) ==
  /\ Listening(p)
  /\ Waiting(q, p)
  /\ inbox' = [inbox EXCEPT ![p] = Enqueue(p, q)]
  /\ reply' = [reply EXCEPT ![q] = "ack"]
  /\ UNCHANGED <<pc, lock, sock, conn, opened, identity, crashes, hung, why>>

Apply(p) ==
  /\ pc[p] = "serve"
  /\ p \notin hung
  /\ \E q \in ToApply(p) : opened' = [opened EXCEPT ![p] = @ \cup {q}]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, identity, crashes, hung, why>>

\* The person closes the window (not fair: a primary may serve forever).
CloseStart(p) ==
  /\ pc[p] = "serve"
  /\ p \notin hung
  /\ pc' = [pc EXCEPT ![p] = "closing"]
  /\ lock' = IF ReleaseOnClose THEN None ELSE lock
  /\ UNCHANGED <<sock, inbox, conn, reply, opened, identity, crashes, hung, why>>

\* While closing, the listener stays open and answers "closing" to every new request.
ReplyClosing(p, q) ==
  /\ pc[p] = "closing"
  /\ sock = Live(p)
  /\ Waiting(q, p)
  /\ reply' = [reply EXCEPT ![q] = "closing"]
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, opened, identity, crashes, hung, why>>

CloseIdentity(p) ==
  /\ pc[p] = "closing"
  /\ identity' = identity \ {p}
  /\ pc' = [pc EXCEPT ![p] = "closed"]
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, opened, crashes, hung, why>>

\* Process exit: the kernel releases the lock; the socket file may stay (worst case: stale).
Exit(p) ==
  /\ pc[p] = "closed"
  /\ pc' = [pc EXCEPT ![p] = "done"]
  /\ lock' = IF lock = p THEN None ELSE lock
  /\ sock' = IF sock = Live(p) THEN Stale ELSE sock
  /\ reply' = Reset(p)
  /\ UNCHANGED <<inbox, conn, opened, identity, crashes, hung, why>>

(* ---- second launch ---- *)
Dial(q) ==
  /\ pc[q] = "dial"
  /\ IF sock.state = "live"
       THEN /\ conn' = [conn EXCEPT ![q] = sock.owner]
            /\ reply' = [reply EXCEPT ![q] = "none"]
            /\ pc' = [pc EXCEPT ![q] = IF WaitsForReply THEN "wait" ELSE "exit0"]
       ELSE /\ UNCHANGED <<conn, reply>>
            /\ pc' = [pc EXCEPT ![q] = IF UnlinksStale THEN "unlinkStale" ELSE "backoff"]
  /\ UNCHANGED <<lock, sock, inbox, opened, identity, crashes, hung, why>>

\* Only reachable under the mutant SecondUnlinks: removes whatever is there by then.
UnlinkStale(q) ==
  /\ pc[q] = "unlinkStale"
  /\ sock' = Absent
  /\ pc' = [pc EXCEPT ![q] = "backoff"]
  /\ UNCHANGED <<lock, inbox, conn, reply, opened, identity, crashes, hung, why>>

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
  /\ UNCHANGED <<lock, sock, inbox, reply, opened, crashes, hung>>

\* The per-try deadline: no answer yet (slow or hung server).
Timeout(q) ==
  /\ pc[q] = "wait"
  /\ reply[q] = "none"
  /\ conn' = [conn EXCEPT ![q] = None]
  /\ pc' = [pc EXCEPT ![q] = "backoff"]
  /\ UNCHANGED <<lock, sock, inbox, reply, opened, identity, crashes, hung, why>>

Backoff(q) ==
  /\ pc[q] = "backoff"
  /\ pc' = [pc EXCEPT ![q] = AfterBackoff]
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, opened, identity, crashes, hung, why>>

\* The 5 s deadline: only while the lock holder does not answer.
GiveUp(q) ==
  /\ pc[q] = "backoff"
  /\ lock \in hung
  /\ pc' = [pc EXCEPT ![q] = "gaveUp"]
  /\ why' = [why EXCEPT ![q] = "hung"]
  /\ identity' = identity \ {q}
  /\ UNCHANGED <<lock, sock, inbox, conn, reply, opened, crashes, hung>>

(* ---- faults ---- *)
Hang(p) ==
  /\ Hangs
  /\ pc[p] \in {"own", "serve"}
  /\ p \notin hung
  /\ hung' = hung \cup {p}
  /\ UNCHANGED <<pc, lock, sock, inbox, conn, reply, opened, identity, crashes, why>>

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
  /\ UNCHANGED <<inbox, opened, why>>

Next ==
  \E p \in Launch :
    \/ TryLock(p) \/ Unlink(p) \/ Bind(p) \/ OpenIdentity(p) \/ Apply(p)
    \/ CloseStart(p) \/ CloseIdentity(p) \/ Exit(p)
    \/ Dial(p) \/ UnlinkStale(p) \/ Answer(p) \/ Timeout(p) \/ Backoff(p) \/ GiveUp(p)
    \/ Hang(p) \/ Crash(p)
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
  \/ TryLock(p) \/ Unlink(p) \/ Bind(p) \/ OpenIdentity(p) \/ Apply(p)
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
\* S4: a second launch that exits 0 was queued by a primary (no silent loss).
ExitOkImpliesQueued == \A q \in Launch : pc[q] = "exit0" => \E p \in Launch : q \in inbox[p]
\* S5: a launch gives up only because the holder does not answer.
GaveUpJustified == \A q \in Launch : why[q] \in {"none", "hung"}

\* L1: every launch settles: it serves, or hands off, or gives up on a hung holder, or crashed.
Settles == \A p \in Launch : (pc[p] = "start") ~> (pc[p] \in Settled \/ p \in hung)
\* L2: a queued request is applied unless its primary stopped serving (closed, crashed, hung).
AckedApplied ==
  \A p, q \in Launch :
    (q \in inbox[p]) ~> (q \in opened[p] \/ pc[p] \in {"closing", "closed", "done", "crashed"} \/ p \in hung)
=============================================================================
