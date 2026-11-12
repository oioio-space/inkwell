---------------------------- MODULE SandboxScope ----------------------------
(***************************************************************************)
(* Block SB1b (W49, ADR 0042 amendment A1-A4 and A8): the memory scope of  *)
(* the sandboxed sidecar. Plan: docs/plans/2026-11-11-sandbox-crash-       *)
(* reports.md, section "Gate SB1b" (rows S, R, M, F, H).                   *)
(*                                                                         *)
(* Mutants and witnesses: mutants/SandboxScopeMutants.tla (EXTENDS this    *)
(* module); configurations: README.md, section SandboxScope.               *)
(*                                                                         *)
(* Not a peer protocol: a local mechanism, kept here because tla-check.sh, *)
(* mise run tla, formal and done check this directory.                     *)
(*                                                                         *)
(* What is modelled. One app session runs the sandboxed launches of Runs   *)
(* (each a compile or a tinymist start, possibly concurrent). The app sets *)
(* up, lazily and under a mutex, a delegated transient scope with a holder *)
(* process (rows S1-S9, one atomic step whose outcomes are the fallback    *)
(* rows F1-F7, F10, F11); session versions 1..MaxSessions. Per run it      *)
(* checks the holder's pidfd (H1), then in one step makes run-<n>, writes  *)
(* and reads back the limits and starts the outer launcher with            *)
(* CLONE_INTO_CGROUP (R1-R5), whose first statement checks its own cgroup  *)
(* (R6); right after Wait it reads the counters (R7), then kills and       *)
(* removes the run cgroup (R8). Kernel facts are kept apart from what      *)
(* the app believes: sc (the scope as it really is), rcg (whether a run    *)
(* cgroup exists and its limits are really in force), loc (where the       *)
(* workload really is), truth (what really ended it).                      *)
(*                                                                         *)
(* The memory cause (A4, A8) is computed from an abstract timeline: kill  *)
(* (how the first local oom_kill rise was seen: in the same read as an     *)
(* oom rise, within W = 1 s of a non-fatal oom event, later, or with no    *)
(* oom rise), hist (a non-fatal local oom event came before), oth (the     *)
(* /proc/vmstat oom_kill delta exceeds the run's own: a kill elsewhere,    *)
(* another run's included), anc (an ancestor's local oom rose), bad        *)
(* (inotify overflow or a failed read). These flags only grow during the   *)
(* watch and are read once, at R7, so each is chosen at the event that     *)
(* sets it (a kill) or at R7 (overflow, failed read, a kill elsewhere):    *)
(* the same outcomes as separate events, with far fewer interleavings.     *)
(*                                                                         *)
(* Faults (Faults = TRUE): a failure at any setup step, a run-level write  *)
(* refused or silently ignored, a launch path that places the bootstrap    *)
(* outside its run cgroup (the code defect R6 exists for), the holder      *)
(* killed at any point, the app SIGKILLed, global and ancestor OOMs, OOM   *)
(* kills elsewhere, non-fatal local oom events, watch overflows and        *)
(* unreadable counters, the app outside the user manager, runs without a   *)
(* PID namespace. Never modelled as faults: the document's own behaviour   *)
(* (reaching the resident limit, mapping past RLIMIT_AS, forking, exiting).*)
(***************************************************************************)
EXTENDS Naturals

CONSTANTS
  Runs,         \* the sandboxed launches of one app session, model values, e.g. {r1, r2}
  MaxSessions,  \* session setups allowed in one behaviour (a new one after a failure or a dead holder)
  Faults        \* BOOLEAN: are environment faults possible

ASSUME MaxSessions \in Nat \ {0}
ASSUME Faults \in BOOLEAN

VARIABLES
  app,     \* the app process is alive
  inUser,  \* kernel fact: the app's cgroup is inside user@<uid>.service (S9; placement permitted)
  sv,      \* version of the current session (0: none yet)
  sst,     \* app's view of session sv: "none", "ready", "failed" (backoff), "stale" (its holder died)
  sc,      \* kernel facts per session version: the scope as it really is
  hold,    \* per session version: the holder "none", "alive", "exited" (by design), "killed" (fault)
  R,       \* per run: a record, see Run0
  over     \* some process mapped past MemoryLimit (A2 broken)

vars == <<app, inUser, sv, sst, sc, hold, R, over>>

Versions == 1..MaxSessions
Choice == IF Faults THEN BOOLEAN ELSE {TRUE}

(***************************************************************************)
(* RULES. Each is one decision of ADR 0042 (amendment); a mutant           *)
(* configuration overrides exactly one of them ("Rule <- Mutant").         *)
(***************************************************************************)
\* A1: the outer launcher is created inside its run cgroup (clone3 CLONE_INTO_CGROUP), never moved in later.
BornInside == TRUE
\* A1, R6: the bootstrap's first statement checks /proc/self/cgroup before any mount, fork or exec.
CheckFirst == TRUE
\* A4, R7-R8: counters are read after Wait and before cgroup.kill and rmdir.
CleanFirst == FALSE
\* A3: any failure of R1-R3 removes the run cgroup and the run uses the fallback.
RunOnFail == "fallback"
\* A8 (aggregate bound): the session scope is created in the slice that contains the app's own cgroup.
ScopeSlice == "app"
\* A3: "done" from the job is a precondition, never the proof: S5-S9 are reads of kernel state.
\* u: the job outcome; b: the inherited bound is in force; m: memory delegated; w: the S6/S7 writes took effect.
Verified(u, b, m, w) == u = "done" /\ b /\ m /\ w /\ inUser
\* A2: RLIMIT_AS is MemoryLimit with or without a cgroup, never raised.
AddressLimit(mode) == "limit"
\* A3, A8 (H2): a placement error (ENOENT at R1, EACCES, EBUSY, ENODEV at clone3) retries the run once in the fallback.
PlacementRetry == "rlimit"
\* A8 (H5): on EOF the holder kills and removes every run-* of its scope, then exits.
ReapOnEOF == TRUE
\* A9 (H6): the namespace init has PR_SET_PDEATHSIG(SIGKILL) tied to the launching app thread (set before
\* exec, getppid re-checked), so it dies with the app.
InitPdeathsig == TRUE
\* A9 (H7): every cgroup launch path runs the target in its own PID namespace; the init's death kills
\* every descendant (pid_namespaces(7)).
CgroupPidNs == TRUE
\* A3: a failed setup selects the fallback (and a retry after a backoff), never a wait.
OnSetupFailed == "fallback"
\* A4, A8: the cause from the timeline read at R7.
SoundCause(o) ==
  IF o.bad THEN "unknown"                                  \* an overflow or a failed read: never zero
  ELSE IF o.kill = "none" THEN "none"
  ELSE IF o.kill \in {"same", "timely"} /\ ~o.oth /\ ~o.anc THEN "local-limit"
  ELSE "victim"
CauseOf(o) == SoundCause(o)

(***************************************************************************)
(* STATE                                                                   *)
(***************************************************************************)
\* pc: "need" (waiting for a session) -> "prep" (holder checked; R1-R5 next) -> "born" (before R6)
\*     -> "exec" (untrusted target) -> "wait" (outer dead) -> R7 -> "clean" -> R8 -> "done";
\*     "boot": a bootstrap that works with no check first (fallback, mutants); "read": R7 after R8
\*     (mutant CleanupBeforeCounters only); "lost" once the app died.
Pcs == {"need", "prep", "born", "boot", "exec", "wait", "read", "clean", "done", "lost"}
Live == {"born", "boot", "exec"}           \* the outer launcher is alive
Run0 == [pc |-> "need", mode |-> "none", ver |-> 0, rcg |-> "none", loc |-> "app", desc |-> "none",
         pidns |-> TRUE, as |-> "limit", hist |-> "none", kill |-> "none", oth |-> FALSE,
         anc |-> FALSE, bad |-> FALSE, truth |-> "none", cause |-> "pending", rd |-> FALSE,
         akill |-> FALSE, ref |-> FALSE, res |-> "none"]

RunType ==
  [pc : Pcs,
   mode : {"none", "cgroup", "rlimit"},    \* Output.Containment as the app will report it
   ver : 0..MaxSessions,                    \* the session the run was verified against
   rcg : {"none", "bare", "limited"},       \* kernel: run-<n> absent, present without its limits, limits in force
   loc : {"app", "run"},                    \* kernel: the cgroup of the workload (outer and descendants)
   desc : {"none", "alive", "dead"},        \* a descendant forked by the target
   pidns : BOOLEAN,                         \* the launch path gives the run a PID namespace
   as : {"limit", "raised"},                \* RLIMIT_AS
   hist : {"none", "seen"},                 \* a non-fatal local oom event was seen before the kill
   kill : {"none", "same", "timely", "late", "bare"},
   oth : BOOLEAN, anc : BOOLEAN, bad : BOOLEAN,
   truth : {"none", "local", "victim", "address"},
   cause : {"pending", "none", "local-limit", "victim", "unknown"},
   rd : BOOLEAN,                            \* R7 done
   akill : BOOLEAN,                         \* R8 done (cgroup.kill, rmdir by the app)
   ref : BOOLEAN,                           \* refused by R6
   res : {"none", "cgroup", "rlimit", "refused", "error"}]

TypeOK ==
  /\ app \in BOOLEAN /\ inUser \in BOOLEAN /\ over \in BOOLEAN
  /\ sv \in 0..MaxSessions
  /\ sst \in {"none", "ready", "failed", "stale"}
  /\ sc \in [Versions -> [alive : BOOLEAN, slice : {"app", "other"}, bound : BOOLEAN, en : BOOLEAN]]
  /\ hold \in [Versions -> {"none", "alive", "exited", "killed"}]
  /\ R \in [Runs -> RunType]

Init ==
  /\ app = TRUE
  /\ inUser \in Choice
  /\ sv = 0
  /\ sst = "none"
  /\ sc = [v \in Versions |-> [alive |-> FALSE, slice |-> "app", bound |-> FALSE, en |-> FALSE]]
  /\ hold = [v \in Versions |-> "none"]
  /\ R = [r \in Runs |-> Run0]
  /\ over = FALSE

Kill(d) == IF d = "alive" THEN "dead" ELSE d
\* Kernel: the workload is in its run cgroup and that cgroup's limits are in force.
RunLimited(o) == o.loc = "run" /\ o.rcg = "limited"
\* The outer launcher dies: its PID namespace, if it has one, ends with it.
OuterDies(o) == [o EXCEPT !.pc = "wait", !.desc = IF o.pidns THEN Kill(o.desc) ELSE o.desc]
\* The app watches the run from R4 to R7 (inotify on memory.events.local, snapshots at R4 and R7).
Watched(q) == R[q].mode = "cgroup" /\ R[q].pc \in {"born", "boot", "exec", "wait", "read", "clean"} /\ ~R[q].rd
\* Every other watched run sees the /proc/vmstat oom_kill rise with no local one.
Others(r) == [q \in Runs |-> IF q # r /\ Watched(q) THEN [R[q] EXCEPT !.oth = TRUE] ELSE R[q]]
\* How the first local oom_kill rise of a victim is seen, after a non-fatal oom event (or none).
VictimKill(h) == CASE h = "within" -> "timely" [] h = "late" -> "late" [] OTHER -> "bare"
Result(o) == IF o.ref THEN "refused" ELSE o.mode
\* A run that ended keeps only what later properties read: every invariant on its cause, counters and
\* order was checked in the state before (each property reads values final by then), so the other
\* fields go back to Run0. This keeps the product of two runs' records small.
Ended(o, pc) == [Run0 EXCEPT !.pc = pc, !.mode = o.mode, !.ver = o.ver, !.rcg = o.rcg, !.loc = o.loc,
                             !.desc = o.desc, !.pidns = o.pidns, !.rd = o.rd, !.akill = o.akill, !.res = o.res]

Fallback(o) == [o EXCEPT !.pc = "boot", !.mode = "rlimit", !.loc = "app", !.rcg = "none",
                         !.as = AddressLimit("rlimit")]

(* ---- session setup, S1-S9 (app, under the session mutex) ---- *)
Setup ==
  /\ app
  /\ sst \in {"none", "stale", "failed"}
  /\ sv < MaxSessions
  /\ \E r \in Runs : R[r].pc = "need"
  /\ LET v == sv + 1 IN
     \E u \in (IF Faults THEN {"done", "fake", "fail"} ELSE {"done"}),  \* S1-S4: F11, F1-F3; "fake": done, holder not moved (F4)
        b \in Choice,                                                   \* S8: inherited bound in place (F7)
        m \in Choice,                                                   \* S5: memory delegated (F5)
        w \in Choice :                                                  \* S6, S7: holder move and +memory took effect (F6)
       LET ok == Verified(u, b, m, w) IN
       /\ sv' = v
       /\ sst' = IF ok THEN "ready" ELSE "failed"
       \* On a failure the app closes the holder's pipe: the holder exits, the empty scope is collected.
       /\ sc' = [sc EXCEPT ![v] = IF ok THEN [alive |-> u = "done", slice |-> ScopeSlice, bound |-> b, en |-> m /\ w]
                                         ELSE [alive |-> FALSE, slice |-> ScopeSlice, bound |-> FALSE, en |-> FALSE]]
       /\ hold' = [hold EXCEPT ![v] = IF ok THEN "alive" ELSE "exited"]
  /\ UNCHANGED <<app, inUser, R, over>>

(* ---- per run, app side ---- *)
\* Before R1: a ready session whose holder is alive (its pidfd is not readable, H1).
Acquire(r) ==
  /\ app
  /\ R[r].pc = "need"
  /\ \/ /\ sst = "ready" /\ hold[sv] = "alive"
        /\ R' = [R EXCEPT ![r].pc = "prep", ![r].ver = sv]
        /\ UNCHANGED sst
     \/ /\ sst = "ready" /\ hold[sv] # "alive"           \* H1: a dead holder invalidates the session
        /\ sst' = "stale"
        /\ UNCHANGED R
     \/ /\ \/ sst = "failed" /\ OnSetupFailed = "fallback"
           \/ sst \in {"none", "stale", "failed"} /\ sv = MaxSessions /\ OnSetupFailed = "fallback"
        /\ R' = [R EXCEPT ![r] = Fallback(@)]
        /\ UNCHANGED sst
  /\ UNCHANGED <<app, inUser, sv, sc, hold, over>>

\* R1-R5 after the pidfd check: mkdir run-<n>; write memory.max, memory.swap.max, memory.oom.group and
\* read them back; open the directory O_PATH; clone3 with CgroupFD. One step: the race the design must
\* survive is between the holder check (Acquire) and clone3, which this keeps; ENOENT at R1 and ENODEV
\* at R5 (the scope collected after its holder died, H2) and EACCES at R5 (F10) are handled alike.
\* Without CLONE_INTO_CGROUP (mutant NotBornInside) the outer launcher starts in the app's cgroup.
Births == IF Faults THEN {"run", "app"} ELSE {"run"}   \* "app": a launch path that forgot the fd (R6's reason)
WriteOutcomes(v) ==
  IF ~sc[v].en THEN {"nowrite"}                       \* memory not enabled for children: no memory.max file
  ELSE IF Faults THEN {"ok", "refused", "ignored"}     \* F8 (write refused), F9 (write silently ignored)
  ELSE {"ok"}

Retry(o) ==
  CASE PlacementRetry = "rlimit" -> Fallback(o)
    [] PlacementRetry = "error" -> Ended([o EXCEPT !.rcg = "none", !.res = "error"], "done")
    [] OTHER -> [Fallback(o) EXCEPT !.mode = PlacementRetry, !.as = AddressLimit(PlacementRetry)]

Place(r) ==
  /\ app
  /\ R[r].pc = "prep"
  /\ LET v == R[r].ver IN
     IF ~sc[v].alive \/ ~inUser
       \* Nothing ran: remove the cgroup, mark the session unavailable, retry once in the fallback.
       THEN /\ sst' = IF v = sv /\ sst = "ready" THEN "failed" ELSE sst
            /\ R' = [R EXCEPT ![r] = Retry(@)]
       ELSE /\ \E out \in WriteOutcomes(v), l \in Births :
                 IF out = "ok" \/ RunOnFail = "continue"
                   THEN R' = [R EXCEPT ![r] = [@ EXCEPT !.rcg = IF out = "ok" THEN "limited" ELSE "bare",
                                                       !.pc = IF BornInside /\ CheckFirst THEN "born" ELSE "boot",
                                                       !.mode = "cgroup",
                                                       !.loc = IF BornInside THEN l ELSE "app",
                                                       !.as = AddressLimit("cgroup")]]
                   \* R3's read-back failed (F8, F9): the run cgroup is removed, the run uses the fallback.
                   ELSE R' = [R EXCEPT ![r] = Fallback(@)]
            /\ UNCHANGED sst
  /\ UNCHANGED <<app, inUser, sv, sc, hold, over>>

\* The app's timeout ends a target that never ends by itself.
Timeout(r) ==
  /\ app
  /\ R[r].pc = "exec"
  /\ R' = [R EXCEPT ![r] = OuterDies(@)]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

\* A fallback run ends at Wait.
Wait(r) ==
  /\ app
  /\ R[r].pc = "wait" /\ R[r].mode = "rlimit"
  /\ R' = [R EXCEPT ![r] = Ended([@ EXCEPT !.res = "rlimit"], "done")]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

\* R7, right after Wait: read memory.events.local, memory.events, the /proc/vmstat and ancestor
\* snapshots; a run cgroup already gone (removed by us or collected with its scope) is a failed read.
\* The flags only grow during the watch and are read here, so two environment facts are chosen here
\* (M7, M9): f, an inotify overflow or a failed read during the watch; e, an OOM kill elsewhere in the
\* system (it only matters when the run had a kill).
Read(r) ==
  /\ app
  /\ R[r].pc = (IF CleanFirst THEN "read" ELSE "wait") /\ R[r].mode = "cgroup"
  /\ \E f \in (IF Faults THEN BOOLEAN ELSE {FALSE}), e \in (IF Faults /\ R[r].kill # "none" THEN BOOLEAN ELSE {FALSE}) :
       LET o == [R[r] EXCEPT !.bad = @ \/ f \/ R[r].rcg = "none", !.oth = @ \/ e] IN
       \* Both orders go on to "clean": R8 next (sound), or Finish (mutant CleanFirst, R8 already done).
       R' = [R EXCEPT ![r] = [o EXCEPT !.cause = CauseOf(o), !.rd = TRUE, !.pc = "clean"]]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

\* R8: cgroup.kill (every process of the run cgroup, descendants included), populated 0, rmdir.
Clean(r) ==
  /\ app
  /\ R[r].pc = (IF CleanFirst THEN "wait" ELSE "clean") /\ R[r].mode = "cgroup"
  /\ LET o == [R[r] EXCEPT !.desc = IF R[r].loc = "run" THEN Kill(@) ELSE @, !.rcg = "none", !.akill = TRUE] IN
     R' = [R EXCEPT ![r] = IF CleanFirst THEN [o EXCEPT !.pc = "read"]
                                         ELSE Ended([o EXCEPT !.res = Result(o)], "done")]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

\* Mutant order only (CleanFirst): the run ends after its late R7.
Finish(r) ==
  /\ app
  /\ CleanFirst
  /\ R[r].pc = "clean" /\ R[r].rd
  /\ R' = [R EXCEPT ![r] = Ended([@ EXCEPT !.res = Result(@)], "done")]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

(* ---- per run, the trusted bootstrap (outer launcher, namespace init, second launcher) ---- *)
\* "born": R6 first: a cgroup other than the expected one exits with the dedicated status (refused,
\* not a fallback); else mounts, namespaces, the second launcher, exec of the target. "boot": the
\* bootstrap works with no check before it (the fallback; the mutants). Under NotBornInside the old
\* design's handshake follows: systemd moves the waiting outer launcher in, then the check runs.
\* At exec the target forks a descendant or not (its own behaviour, chosen here: the descendant
\* lives in the target's cgroup); a cgroup run has a PID namespace (A9), a fallback run may not.
Boot(r) ==
  /\ R[r].pc \in {"born", "boot"}
  /\ LET o == R[r] IN
     CASE o.pc = "born" /\ o.loc # "run" ->
            R' = [R EXCEPT ![r] = [o EXCEPT !.pc = "wait", !.ref = TRUE]]
       [] o.pc = "boot" /\ ~BornInside /\ o.mode = "cgroup" /\ o.loc = "app" ->
            R' = [R EXCEPT ![r] = [o EXCEPT !.loc = "run", !.pc = "born"]]
       [] OTHER ->
            \E d \in {"none", "alive"}, n \in (IF Faults THEN BOOLEAN ELSE {TRUE}) :
              R' = [R EXCEPT ![r] = [o EXCEPT !.pc = "exec", !.desc = d,
                                              !.pidns = IF o.mode = "cgroup" /\ CgroupPidNs THEN TRUE
                                                        ELSE IF d = "alive" THEN n ELSE TRUE]]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

(* ---- per run, the untrusted target and the kernel (never fair) ---- *)
Exit(r) ==
  /\ R[r].pc = "exec"
  /\ R' = [R EXCEPT ![r] = OuterDies(@)]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

\* M2: one mapping past RLIMIT_AS fails (musl SIGSEGV after Rust's abort), unless the limit was raised.
BigMap(r) ==
  /\ R[r].pc = "exec" /\ ~over
  /\ IF R[r].as = "limit"
       THEN /\ R' = [R EXCEPT ![r] = [OuterDies(@) EXCEPT !.truth = "address"]]
            /\ UNCHANGED over
       ELSE /\ over' = TRUE
            /\ UNCHANGED R
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold>>

\* M1: the run's aggregate charge reaches memory.max: local oom and oom_kill rise in the same read,
\* oom.group kills the whole run (SIGKILL).
LocalOom(r) ==
  /\ R[r].pc = "exec" /\ R[r].mode = "cgroup" /\ R[r].loc = "run" /\ R[r].rcg = "limited"
  /\ R' = [Others(r) EXCEPT ![r] = [OuterDies(R[r]) EXCEPT !.desc = Kill(R[r].desc), !.kill = "same",
                                                            !.truth = "local"]]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>

\* M3, M8: a global OOM (a = FALSE) or an ancestor's limit (a = TRUE: the scope's copied bound, the shared
\* slice) picks the run: oom_kill rises, local oom does not; oom.group kills the group. Before it, the
\* run may have had a non-fatal local oom event (A8: the OOM killer found the allocating task exiting
\* or a victim already chosen), within the window W = 1 s of this kill or earlier (M6).
Victim(r) ==
  /\ Faults
  /\ R[r].pc = "exec" /\ R[r].mode = "cgroup"
  /\ \E a \in BOOLEAN, h \in (IF RunLimited(R[r]) THEN {"none", "within", "late"} ELSE {"none"}) :
       R' = [Others(r) EXCEPT ![r] = [OuterDies(R[r]) EXCEPT !.desc = Kill(R[r].desc), !.anc = a,
                                        !.hist = IF h = "none" THEN "none" ELSE "seen",
                                        !.kill = VictimKill(h), !.truth = "victim"]]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, hold, over>>


(* ---- session faults, systemd, and the holder ---- *)
HolderDies(v) ==
  /\ Faults
  /\ hold[v] = "alive"
  /\ hold' = [hold EXCEPT ![v] = "killed"]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, R, over>>

\* systemd stops a scope with no process left and removes its cgroup tree (CollectMode).
Collect(v) ==
  /\ sc[v].alive
  /\ hold[v] # "alive"
  /\ ~\E q \in Runs : R[q].ver = v /\ R[q].loc = "run" /\ (R[q].pc \in Live \/ R[q].desc = "alive")
  /\ sc' = [sc EXCEPT ![v].alive = FALSE]
  /\ R' = [q \in Runs |-> IF R[q].ver = v THEN [R[q] EXCEPT !.rcg = "none"] ELSE R[q]]
  /\ UNCHANGED <<app, inUser, sv, sst, hold, over>>

\* SIGKILL of the app: each outer launcher and namespace init dies by Pdeathsig, its PID namespace (if
\* any) with it. Without the init's Pdeathsig (mutant NoPdeathsig) the workload survives the app.
AppDies ==
  /\ Faults
  /\ app
  /\ app' = FALSE
  /\ R' = [q \in Runs |->
             IF R[q].pc = "done" THEN R[q]
             ELSE Ended([R[q] EXCEPT !.desc = CASE R[q].pc \notin Live -> @
                                                    [] ~InitPdeathsig -> "alive"     \* the init and target live on
                                                    [] R[q].pidns -> Kill(@)
                                                    [] OTHER -> @], "lost")]
  /\ UNCHANGED <<inUser, sv, sst, sc, hold, over>>

\* H5: the holder reads EOF on its pipe: cgroup.kill on every run-* of its scope, populated 0, rmdir, exit.
HolderReap(v) ==
  /\ ~app
  /\ hold[v] = "alive"
  /\ ReapOnEOF
  /\ hold' = [hold EXCEPT ![v] = "exited"]
  /\ R' = [q \in Runs |->
             IF R[q].ver = v /\ R[q].rcg # "none"
               THEN [R[q] EXCEPT !.rcg = "none", !.desc = IF R[q].loc = "run" THEN Kill(R[q].desc) ELSE R[q].desc]
               ELSE R[q]]
  /\ UNCHANGED <<app, inUser, sv, sst, sc, over>>

\* The app's own steps for one run, and the bootstrap's.
AppStep(r) == Acquire(r) \/ Place(r) \/ Timeout(r) \/ Wait(r) \/ Read(r) \/ Clean(r) \/ Finish(r)
BootStep(r) == Boot(r)

Next ==
  \/ Setup \/ AppDies
  \/ \E r \in Runs :
       \/ AppStep(r) \/ BootStep(r)
       \/ Exit(r) \/ BigMap(r) \/ LocalOom(r) \/ Victim(r)
  \/ \E v \in Versions : HolderDies(v) \/ Collect(v) \/ HolderReap(v)

(***************************************************************************)
(* FAIRNESS: four weak terms, one per process (README.md, SandboxScope,    *)
(* "Fairness"). Every fair action can occur only finitely often (each run  *)
(* passes once, setups are bounded, each holder reaps once, each scope is  *)
(* collected once), so weak fairness on each group's disjunction forces    *)
(* every one of its actions that stays enabled. None on the target, the    *)
(* kernel's OOM decisions or faults.                                       *)
(***************************************************************************)
Fairness ==
  \* F1: the app keeps running while alive (its steps, the timeout included, and the session setup).
  /\ WF_vars(Setup \/ \E r \in Runs : AppStep(r))
  \* F2: the trusted bootstrap of every live run keeps running.
  /\ WF_vars(\E r \in Runs : BootStep(r))
  \* F3: a holder that read EOF and is not killed finishes its cleanup (the 5 s bound).
  /\ WF_vars(\E v \in Versions : HolderReap(v))
  \* F4: systemd collects a scope with no process left (CollectMode inactive-or-failed).
  /\ WF_vars(\E v \in Versions : Collect(v))

Spec == Init /\ [][Next]_vars /\ Fairness

(***************************************************************************)
(* PROPERTIES                                                              *)
(***************************************************************************)
\* A1, A3, R3, R6 (S1): every sandbox instruction after the membership check (trusted bootstrap,
\* untrusted target, any descendant) runs in its run cgroup with its limits in force, or the run
\* reports Containment rlimit.
ConfinedFromBirth ==
  \A r \in Runs :
    (R[r].pc \in {"boot", "exec"} \/ R[r].desc = "alive")
      => (R[r].mode = "rlimit" \/ RunLimited(R[r]))
\* A2 (S2): no process maps past MemoryLimit.
AddressBudget == ~over
\* A8 aggregate bound, A3 (S3): a workload in a run cgroup sits in a scope inside the app's own slice
\* with the app's leaf bound in force.
SessionBounded ==
  \A r \in Runs :
    (R[r].loc = "run" /\ R[r].ver \in Versions /\ (R[r].pc \in Live \/ R[r].desc = "alive"))
      => sc[R[r].ver].slice = "app" /\ sc[R[r].ver].bound
\* A4, R7-R8 (S4): the app never kills or removes a run cgroup before reading its counters.
CountersBeforeCleanup == \A r \in Runs : R[r].akill => R[r].rd
\* A4, M9 (S5): a failed read or an overflow is "unknown", never zero.
UnreadNeverZero == \A r \in Runs : (R[r].rd /\ R[r].bad) => R[r].cause = "unknown"
\* Residual RS4 (A8): a global OOM that kills only this run within W of a non-fatal local oom event.
RS4(o) == o.truth = "victim" /\ o.kill = "timely" /\ ~o.oth /\ ~o.anc
\* A4, A8, M1, M3, M6-M8 (S6): only the run's own limit is a memory limit (up to RS4).
LimitIsLocal == \A r \in Runs : R[r].cause = "local-limit" => (R[r].truth = "local" \/ RS4(R[r]))
\* A3, F1-F11, H2 (S7): a setup or placement failure is the fallback, never an error.
FailureFallsBack == \A r \in Runs : R[r].res # "error"

Orphan(r) == R[r].mode = "cgroup" /\ R[r].loc = "run" /\ R[r].desc = "alive"
HolderKilled(r) == R[r].ver \in Versions /\ hold[R[r].ver] = "killed"
\* A8, A9, H4-H7 (L1): after the app dies, no process remains in a run cgroup, whatever the holder did.
NoOrphan == \A r \in Runs : (~app /\ Orphan(r)) ~> ~Orphan(r)
\* A8, A9, H5, H8 (L3): after the app dies, every session scope and every run cgroup is eventually gone.
NoStaleScope == ~app ~> ((\A v \in Versions : ~sc[v].alive) /\ \A r \in Runs : R[r].rcg = "none")
\* A3 (L2): every run ends (confined in a cgroup, in the fallback, or refused by R6), or the app died.
RunsEnd == \A r \in Runs : (R[r].pc = "need") ~> (R[r].pc \in {"done", "lost"})
=============================================================================
