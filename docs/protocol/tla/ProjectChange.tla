---------------------------- MODULE ProjectChange ----------------------------
(***************************************************************************)
(* Block PC1 (ADR 0043 part 2): the project of a running session never     *)
(* changes. A take-over (« Reprendre ce document ici ») or an accepted     *)
(* invitation into another project ends in a restart: the new member list  *)
(* is written while nothing of the old session runs, and the next process  *)
(* image loads it from disk.                                               *)
(* Plan: docs/plans/2026-11-14-pc1-project-change-by-restart.md, rules     *)
(* N-5..N-14 (take-over, restart) and J-1..J-5 (join).                     *)
(* Mutants and witnesses: mutants/ProjectChangeMutants.tla (EXTENDS this   *)
(* module); configurations: README.md, section ProjectChange.              *)
(*                                                                         *)
(* Actors:                                                                 *)
(*   the Go runtime of each process image (phase, proj, wiring, buf, req,  *)
(*     wstep): the wiring with its 64-event buffer, the trust store and    *)
(*     the invitation host; it shuts down in the order of N-13, writes,    *)
(*     passes the termination barrier of N-14, then restarts (N-12);       *)
(*   the page (ph, pst, cov, cont, proot): the adapter's coverage, the     *)
(*     continuations automerge-repo holds, the root it opened. It is the   *)
(*     server build's page, a browser tab that outlives the process; the   *)
(*     desktop webview, which ends with the process, is the special case   *)
(*     where the reload is immediate;                                      *)
(*   the restart: Platform "exec" (Linux: execve of /proc/self/exe, same   *)
(*     process, every other thread ends, descriptors close-on-exec) or     *)
(*     "spawn" (macOS, Windows, N35: a new process that claims the         *)
(*     profile lock like any launch, ADR 0037, while the old one exits);   *)
(*   the environment: one peer that attests under whatever project the     *)
(*     wiring runs, the person (request, quit, relaunch), a member that    *)
(*     gives the root document after a join, children (tinymist, typst, the*)
(*     preview server), crashes and failed steps.                          *)
(*                                                                         *)
(* Durable (the folder; survives a crash and a restart): trust, prev,      *)
(* next (opStore.replace: the list, the set-aside one, the staged one; each*)
(* rename is followed by a directory sync, so each step is one durable     *)
(* step), pj (the pending-join record, a field of project.json), root      *)
(* (project.json rootUrl). Everything else is memory: a crash or a restart *)
(* drops it (Raft's TLA+ Restart: "it loses everything but" stable         *)
(* storage). Projects are "old" (the one the folder had) and "new"; roots  *)
(* are R0 (the folder's document) and R1 (the inviter's document).         *)
(***************************************************************************)
EXTENDS Naturals, Sequences, FiniteSets

CONSTANTS
  Platform,    \* "exec" or "spawn"
  Kinds,       \* requests a behaviour may make: a subset of {"takeover", "join"}
  MaxImg,      \* process images in one behaviour (first, restarted, one relaunch)
  MaxCrashes,  \* crashes of any process at any step
  MaxErrors    \* failed steps: a write step or the execution of the restart

ASSUME Platform \in {"exec", "spawn"}
ASSUME Kinds \subseteq {"takeover", "join"} /\ Kinds # {}
ASSUME MaxImg \in Nat /\ MaxImg >= 2
ASSUME MaxCrashes \in Nat /\ MaxErrors \in Nat

Img == 1..MaxImg
Alive == {"wait", "load", "run", "leaving", "closed", "stopped", "exiting"}
Active == Alive \ {"wait"}          \* a process that holds or uses the profile
Ended == {"none", "gone", "crashed", "failed"}

VARIABLES
  trust, prev, next, pj, root,                                     \* durable
  phase, proj, wiring, buf, req, wstep, flag, byRestart, notice,   \* each image's Go runtime
  lock, children,                                                  \* kernel: profile lock, live children by image
  ph, pst, cov, cont, proot,                                       \* the page
  kind, rk, top,                                                   \* the request, the newest image
  requests, restarts, consumed, committed, polluted, bad, cut, crashes, errors, intr  \* history

disk == <<trust, prev, next, pj, root>>
goVars == <<phase, proj, wiring, buf, req, wstep, flag, byRestart, notice>>
kernel == <<lock, children>>
page == <<ph, pst, cov, cont, proot>>
env == <<kind, rk, top>>
hist == <<requests, restarts, consumed, committed, polluted, bad, cut, crashes, errors, intr>>
vars == <<disk, goVars, kernel, page, env, hist>>

(***************************************************************************)
(* RULES. Each is one decision of the plan; a mutant configuration         *)
(* overrides exactly one of them ("Rule <- Mutant").                       *)
(***************************************************************************)
\* N-11: the project of a running session is never changed in place.
LiveChange == FALSE
\* N-13: the write starts only once the wiring, the lobby, the invitation host and any pending join have ended.
WriteOK(i) == ~wiring[i]
\* N-5, J-1: the write's steps; a join writes its pending-join record before the new list is staged.
StepsOf(k) == IF k = "join" THEN <<"record", "stage", "aside", "install", "cleanup">>
                            ELSE <<"stage", "aside", "install", "cleanup">>
\* N-12: a restart only for a request this image recorded, never because the image was started by one.
RestartWanted(i) == req[i] # "none"
\* N-12: a restart only after stopAll returned, so after the write's outcome is on disk.
StoppedOK(i) == phase[i] = "stopped"
\* N-14: the termination barrier: no child of the image is alive.
BarrierOK(i) == i \notin children
\* N-12, N35: an image starts its work only once it holds the profile lock (ADR 0037, D-LOCK).
MayStart(j) == lock = 0
\* N-9: once the page asked, or lost its image, it applies and persists nothing until it reloads.
PageGate == TRUE
\* N-9: the page reloads only on another startup id.
ReloadOn(j) == j # ph
\* J-1: consuming the record removes it in the same atomic rewrite of project.json that records the root.
ClearRecord == "none"
\* N-7: recovery at open (recoverSwap): a missing list is restored from the set-aside one.
RecoveredTrust == IF trust = "none" /\ prev = "old" THEN "old" ELSE trust
\* N-12: the notice follows the standing actually loaded, not the request.
NoticeOf(t) == IF t = "new" THEN "done" ELSE "failed"

(***************************************************************************)
(* Helpers                                                                 *)
(***************************************************************************)
Steps(i) == StepsOf(req[i])
WriteDone(i) == req[i] = "none" \/ wstep[i] > Len(Steps(i))
\* Every step before the install rename rolls back on an error; after it, errCommitted.
BeforeInstall(i) == \A n \in 1..Len(Steps(i)) : Steps(i)[n] = "install" => wstep[i] <= n
\* The root a project opens: a take-over keeps the folder's document, a join opens the inviter's.
RootOf(p) == IF p = "new" /\ kind = "join" THEN "R1" ELSE "R0"
\* What the page opens after loading image j: while a join waits for its root document, nothing.
RootChoice(j) == IF pj # "none" /\ proj[j] = "new" THEN "none" ELSE root
\* The list the next open loads, by the specified recovery (used by the properties, never overridden).
Recovered == IF trust = "none" /\ prev = "old" THEN "old" ELSE trust

TypeOK ==
  /\ trust \in {"old", "new", "none"} /\ prev \in {"old", "none"} /\ next \in {"new", "none"}
  /\ pj \in {"none", "known", "unknown"} /\ root \in {"R0", "R1"}
  /\ phase \in [Img -> Alive \cup Ended]
  /\ proj \in [Img -> {"old", "new", "none"}]
  /\ wiring \in [Img -> BOOLEAN] /\ buf \in [Img -> SUBSET {"old", "new"}]
  /\ req \in [Img -> {"none", "takeover", "join"}] /\ wstep \in [Img -> 1..6]
  /\ flag \in [Img -> BOOLEAN] /\ byRestart \in [Img -> BOOLEAN]
  /\ notice \in [Img -> {"none", "done", "failed", "joined", "joinFailed"}]
  /\ lock \in 0..MaxImg /\ children \subseteq Img
  /\ ph \in Img /\ pst \in {"live", "waiting"} /\ cov \subseteq {"old", "new"} /\ cont \subseteq {"old", "new"}
  /\ proot \in {"none", "R0", "R1"}
  /\ kind \in {"none", "takeover", "join"} /\ rk \in BOOLEAN /\ top \in Img

Init ==
  /\ trust = "old" /\ prev = "none" /\ next = "none" /\ pj = "none" /\ root = "R0"
  /\ phase = [j \in Img |-> IF j = 1 THEN "run" ELSE "none"]
  /\ proj = [j \in Img |-> IF j = 1 THEN "old" ELSE "none"]
  /\ wiring = [j \in Img |-> j = 1]
  /\ buf = [j \in Img |-> {}]
  /\ req = [j \in Img |-> "none"]
  /\ wstep = [j \in Img |-> 1]
  /\ flag = [j \in Img |-> FALSE]
  /\ byRestart = [j \in Img |-> FALSE]
  /\ notice = [j \in Img |-> "none"]
  /\ lock = 1 /\ children = {}
  /\ ph = 1 /\ pst = "live" /\ cov = {} /\ cont = {} /\ proot = "R0"
  /\ kind = "none" /\ rk = FALSE /\ top = 1
  /\ requests = 0 /\ restarts = 0 /\ consumed = 0
  /\ committed = FALSE /\ polluted = FALSE /\ bad = FALSE /\ cut = FALSE
  /\ crashes = 0 /\ errors = 0 /\ intr = FALSE

(***************************************************************************)
(* The session and the peer (environment, no fairness)                     *)
(***************************************************************************)
\* The peer attests a head; the wiring validates it under the project it runs and buffers the event.
PeerAttest(i) ==
  /\ wiring[i] /\ proj[i] \notin buf[i]
  /\ buf' = [buf EXCEPT ![i] = @ \cup {proj[i]}]
  /\ UNCHANGED <<disk, phase, proj, wiring, req, wstep, flag, byRestart, notice, kernel, page, env, hist>>

\* The bridge hands a buffered Attested to the page bound to that image: coverage admits it.
Deliver(i) ==
  /\ pst = "live" /\ ph = i
  /\ \E e \in buf[i] : buf' = [buf EXCEPT ![i] = @ \ {e}] /\ cov' = cov \cup {e}
  /\ UNCHANGED <<disk, phase, proj, wiring, req, wstep, flag, byRestart, notice, kernel, ph, pst, cont, proot, env, hist>>

\* A frame of a head attested under p is admitted and handed to automerge-repo: a pending continuation.
Frame ==
  /\ pst = "live"
  /\ \E p \in cov \ cont : cont' = cont \cup {p}
  /\ UNCHANGED <<disk, goVars, kernel, ph, pst, cov, proot, env, hist>>

\* A continuation applies and persists its change through the image that answers; bad records a change
\* authorised under one project that reached an image running another.
Apply ==
  /\ \E p \in cont :
       /\ \/ pst = "live" /\ bad' = (bad \/ p # proj[ph])
          \/ pst = "waiting" /\ ~PageGate
             /\ \E a \in Img : phase[a] = "run" /\ bad' = (bad \/ p # proj[a])
       /\ cont' = cont \ {p}
  /\ UNCHANGED <<disk, goVars, kernel, ph, pst, cov, proot, env, requests, restarts, consumed, committed,
                 polluted, cut, crashes, errors, intr>>

\* The wiring saves an op of its project to .inkwell/trust; an op of the old project in the installed new
\* list is old state that the next image would load.
WiringSave(i) ==
  /\ wiring[i] /\ trust = "new" /\ proj[i] = "old" /\ ~polluted
  /\ polluted' = TRUE
  /\ UNCHANGED <<disk, goVars, kernel, page, env, requests, restarts, consumed, committed, bad, cut, crashes,
                 errors, intr>>

\* A child (language service, typst, the preview server) is started.
SpawnChild(i) ==
  /\ phase[i] = "run" /\ i \notin children
  /\ children' = children \cup {i}
  /\ UNCHANGED <<disk, goVars, lock, page, env, hist>>

(***************************************************************************)
(* The person                                                              *)
(***************************************************************************)
\* N-5, N-9, J-1: the request. Go records it in memory (a take-over's genesis, or a join's Welcome: the
\* list, the inviter, the root when the genesis carries one) and quits; it writes nothing. The page shows
\* « Redémarrage… », closes its bridge and from now on applies nothing (PageGate).
Request(i) ==
  /\ ~LiveChange
  /\ phase[i] = "run" /\ ph = i /\ pst = "live" /\ requests = 0 /\ proj[i] = "old" /\ top < MaxImg
  /\ \E k \in Kinds, r \in BOOLEAN :
       /\ (k = "takeover" => ~r)
       /\ kind' = k /\ rk' = r /\ req' = [req EXCEPT ![i] = k]
  /\ phase' = [phase EXCEPT ![i] = "leaving"]
  /\ pst' = "waiting"
  /\ requests' = 1
  /\ UNCHANGED <<disk, proj, wiring, buf, wstep, flag, byRestart, notice, kernel, ph, cov, cont, proot, top,
                 restarts, consumed, committed, polluted, bad, cut, crashes, errors, intr>>

\* The rejected design (ADR 0043, « Considered options »): adopt the new project in the running session,
\* store it, reset the adapter when the call returns; the 64-event buffer is not cleared.
LiveAdopt(i) ==
  /\ LiveChange
  /\ phase[i] = "run" /\ ph = i /\ pst = "live" /\ requests = 0 /\ proj[i] = "old"
  /\ kind' = "takeover" /\ requests' = 1
  /\ proj' = [proj EXCEPT ![i] = "new"]
  /\ trust' = "new" /\ committed' = TRUE
  /\ cov' = {} /\ cont' = {}
  /\ UNCHANGED <<prev, next, pj, root, phase, wiring, buf, req, wstep, flag, byRestart, notice, kernel, ph, pst,
                 proot, rk, top, restarts, consumed, polluted, bad, cut, crashes, errors, intr>>

\* The person closes the window.
Quit(i) ==
  /\ phase[i] = "run" /\ ph = i /\ pst = "live"
  /\ phase' = [phase EXCEPT ![i] = "leaving"]
  /\ pst' = "waiting"
  /\ intr' = (intr \/ requests = 1)
  /\ UNCHANGED <<disk, proj, wiring, buf, req, wstep, flag, byRestart, notice, kernel, ph, cov, cont, proot, env,
                 requests, restarts, consumed, committed, polluted, bad, cut, crashes, errors>>

\* The person launches Inkwell again once no process of the profile is left (after a crash or a failed
\* execution, R-9).
Launch ==
  /\ \A i \in 1..top : phase[i] \in Ended
  /\ top < MaxImg
  /\ top' = top + 1
  /\ phase' = [phase EXCEPT ![top + 1] = "wait"]
  /\ UNCHANGED <<disk, proj, wiring, buf, req, wstep, flag, byRestart, notice, kernel, page, kind, rk, hist>>

(***************************************************************************)
(* The Go runtime: shutdown (N-13), write (N-5, N-7, J-1), barrier (N-14), *)
(* restart (N-12), start (N-12, J-1, J-4)                                  *)
(***************************************************************************)
CloseWiring(i) ==
  /\ phase[i] = "leaving" /\ wiring[i]
  /\ wiring' = [wiring EXCEPT ![i] = FALSE]
  /\ buf' = [buf EXCEPT ![i] = {}]
  /\ phase' = [phase EXCEPT ![i] = "closed"]
  /\ UNCHANGED <<disk, proj, req, wstep, flag, byRestart, notice, kernel, page, env, hist>>

DoStep(s) ==
  CASE s = "record"  -> pj' = (IF rk THEN "known" ELSE "unknown") /\ UNCHANGED <<trust, prev, next, committed>>
    [] s = "stage"   -> next' = "new" /\ UNCHANGED <<trust, prev, pj, committed>>
    [] s = "aside"   -> trust' = "none" /\ prev' = "old" /\ UNCHANGED <<next, pj, committed>>
    [] s = "install" -> trust' = next /\ next' = "none" /\ committed' = (committed \/ next = "new")
                        /\ UNCHANGED <<prev, pj>>
    [] s = "cleanup" -> prev' = "none" /\ UNCHANGED <<trust, next, pj, committed>>

WriteStep(i) ==
  /\ phase[i] \in {"leaving", "closed"} /\ ~WriteDone(i) /\ WriteOK(i)
  /\ DoStep(Steps(i)[wstep[i]])
  /\ wstep' = [wstep EXCEPT ![i] = @ + 1]
  /\ UNCHANGED <<root, phase, proj, wiring, buf, req, flag, byRestart, notice, kernel, page, env,
                 requests, restarts, consumed, polluted, bad, cut, crashes, errors, intr>>

\* A step fails. Before the install rename the replacement rolls back (the rollback itself may fail,
\* leaving the set-aside list for the next open, N-7) and the staged list is discarded; after it, the
\* new list is in place (errCommitted) and the next open discards what is left. The write ends either way.
WriteError(i) ==
  /\ phase[i] \in {"leaving", "closed"} /\ ~WriteDone(i) /\ WriteOK(i) /\ errors < MaxErrors
  /\ IF BeforeInstall(i)
       THEN \E rolledBack \in BOOLEAN :
              /\ trust' = IF trust = "none" /\ rolledBack THEN "old" ELSE trust
              /\ prev' = IF trust = "none" /\ rolledBack THEN "none" ELSE prev
              /\ next' = "none"
       ELSE UNCHANGED <<trust, prev, next>>
  /\ wstep' = [wstep EXCEPT ![i] = Len(Steps(i)) + 1]
  /\ errors' = errors + 1 /\ intr' = TRUE
  /\ UNCHANGED <<pj, root, phase, proj, wiring, buf, req, flag, byRestart, notice, kernel, page, env,
                 requests, restarts, consumed, committed, polluted, bad, cut, crashes>>

\* stopAll returns: the network, the project and the stores are closed. On exec the profile lock is
\* released (closeInstance, and close-on-exec); a spawning image keeps it until it exits (D-HOLD).
StopAll(i) ==
  /\ phase[i] = "closed" /\ WriteDone(i)
  /\ phase' = [phase EXCEPT ![i] = "stopped"]
  /\ lock' = IF Platform = "exec" /\ lock = i THEN 0 ELSE lock
  /\ UNCHANGED <<disk, proj, wiring, buf, req, wstep, flag, byRestart, notice, children, page, env, hist>>

\* Children end once the shutdown asked them to.
ChildEnd(i) ==
  /\ i \in children /\ phase[i] \in {"closed", "stopped", "exiting", "gone"}
  /\ children' = children \ {i}
  /\ UNCHANGED <<disk, goVars, lock, page, env, hist>>

RestartGuard(i) ==
  /\ phase[i] \in {"closed", "stopped"} /\ StoppedOK(i) /\ BarrierOK(i) /\ RestartWanted(i) /\ top < MaxImg

\* N-12: the restart. exec: the same process becomes image j (every thread of i ends, so does its
\* memory); spawn: a new process j that must claim the profile lock, while i exits. j carries one fact,
\* INKWELL_AFTER_TAKEOVER (flag), and nothing else.
Restart(i) ==
  /\ RestartGuard(i)
  /\ LET j == top + 1 IN
       /\ top' = j
       /\ phase' = [phase EXCEPT ![i] = IF Platform = "exec" THEN "gone" ELSE "exiting", ![j] = "wait"]
       /\ byRestart' = [byRestart EXCEPT ![j] = TRUE]
       /\ flag' = [flag EXCEPT ![j] = (req[i] = "takeover")]
  /\ wiring' = [wiring EXCEPT ![i] = FALSE] /\ buf' = [buf EXCEPT ![i] = {}]
  /\ lock' = IF Platform = "exec" /\ lock = i THEN 0 ELSE lock
  /\ restarts' = restarts + 1
  /\ cut' = (cut \/ ~WriteDone(i))
  /\ UNCHANGED <<disk, proj, req, wstep, notice, children, page, kind, rk, requests, consumed, committed,
                 polluted, bad, crashes, errors, intr>>

\* N-12: the execution fails: the runtime is already closed; the process notifies and exits.
ExecFail(i) ==
  /\ RestartGuard(i) /\ errors < MaxErrors
  /\ phase' = [phase EXCEPT ![i] = "failed"]
  /\ lock' = IF lock = i THEN 0 ELSE lock
  /\ errors' = errors + 1 /\ intr' = TRUE
  /\ UNCHANGED <<disk, proj, wiring, buf, req, wstep, flag, byRestart, notice, children, page, env,
                 requests, restarts, consumed, committed, polluted, bad, cut, crashes>>

\* The process ends: after a quit with no request, or a spawning image once it spawned.
Exit(i) ==
  /\ \/ phase[i] = "stopped" /\ ~RestartWanted(i)
     \/ phase[i] = "exiting"
  /\ i \notin children
  /\ phase' = [phase EXCEPT ![i] = "gone"]
  /\ lock' = IF lock = i THEN 0 ELSE lock
  /\ UNCHANGED <<disk, proj, wiring, buf, req, wstep, flag, byRestart, notice, children, page, env, hist>>

\* A new image claims the profile lock (ADR 0037: the claim comes before anything else).
Start(j) ==
  /\ phase[j] = "wait" /\ MayStart(j)
  /\ phase' = [phase EXCEPT ![j] = "load"]
  /\ lock' = IF lock = 0 THEN j ELSE lock
  /\ UNCHANGED <<disk, proj, wiring, buf, req, wstep, flag, byRestart, notice, children, page, env, hist>>

\* The start: recover the list (N-7), load it, drop the pending-join record of a join that did not
\* install (J-4), choose the notice from what loaded (N-12) and read the variable once, then open the
\* wiring. A record of a join that installed stays until its root document is held (RootArrives).
\* Recovery is several idempotent durable steps; a crash inside it is the same as a crash before it, so
\* it is one step here.
Load(j) ==
  /\ phase[j] = "load"
  /\ LET t == RecoveredTrust
         failedJoin == pj # "none" /\ t = "old"
     IN /\ trust' = t /\ prev' = "none" /\ next' = "none"
        /\ proj' = [proj EXCEPT ![j] = t]
        /\ pj' = IF failedJoin THEN "none" ELSE pj
        /\ notice' = [notice EXCEPT ![j] = IF failedJoin THEN "joinFailed"
                                           ELSE IF flag[j] THEN NoticeOf(t) ELSE "none"]
  /\ flag' = [flag EXCEPT ![j] = FALSE]
  /\ phase' = [phase EXCEPT ![j] = "run"]
  /\ wiring' = [wiring EXCEPT ![j] = TRUE]
  /\ UNCHANGED <<root, buf, req, wstep, byRestart, kernel, page, env, requests, restarts, consumed, committed,
                 polluted, bad, cut, crashes, errors, intr>>

\* Any process dies at any step: its memory goes, its children with it (Pdeathsig and the PID namespace,
\* ADR 0042 A9), the kernel releases its lock; the page loses its image.
Crash(i) ==
  /\ phase[i] \in Alive /\ crashes < MaxCrashes
  /\ phase' = [phase EXCEPT ![i] = "crashed"]
  /\ wiring' = [wiring EXCEPT ![i] = FALSE] /\ buf' = [buf EXCEPT ![i] = {}]
  /\ children' = children \ {i}
  /\ lock' = IF lock = i THEN 0 ELSE lock
  /\ pst' = IF ph = i THEN "waiting" ELSE pst
  /\ crashes' = crashes + 1 /\ intr' = (intr \/ requests = 1)
  /\ UNCHANGED <<disk, proj, req, wstep, flag, byRestart, notice, ph, cov, cont, proot, env,
                 requests, restarts, consumed, committed, polluted, bad, cut, errors>>

(***************************************************************************)
(* The page                                                                *)
(***************************************************************************)
\* N-9: the page polls the startup id; on an image that answers with another id it reloads: a fresh
\* heap, an empty coverage, no continuation; then it opens the root Go names (J-2: none while a join
\* waits for its root, never a fresh document).
Reload(j) ==
  /\ pst = "waiting" /\ phase[j] = "run" /\ ReloadOn(j)
  /\ ph' = j /\ pst' = "live" /\ cov' = {} /\ cont' = {} /\ proot' = RootChoice(j)
  /\ UNCHANGED <<disk, goVars, kernel, env, hist>>

\* J-1, J-2: after a join, a member of the new project (the inviter, or any member) is online: it
\* announces the root when the genesis carries none ("unknown"), and the page finds the root document
\* and stores it; then Go consumes the record in one atomic rewrite of project.json (rootUrl set, record
\* removed). Until then no root is recorded, so no start can replace it by a fresh document.
RootArrives(j) ==
  /\ phase[j] = "run" /\ ph = j /\ pst = "live" /\ proot = "none" /\ pj # "none" /\ proj[j] = "new"
  /\ proot' = "R1" /\ root' = "R1" /\ pj' = ClearRecord /\ consumed' = consumed + 1
  /\ notice' = [notice EXCEPT ![j] = "joined"]
  /\ UNCHANGED <<trust, prev, next, phase, proj, wiring, buf, req, wstep, flag, byRestart, kernel, ph, pst, cov,
                 cont, env, requests, restarts, committed, polluted, bad, cut, crashes, errors, intr>>

(***************************************************************************)
(* Specification                                                           *)
(***************************************************************************)
GoStep == \E i \in Img : CloseWiring(i) \/ WriteStep(i) \/ StopAll(i) \/ ChildEnd(i) \/ Restart(i)
                         \/ Exit(i) \/ Start(i) \/ Load(i)
PageStep == \E j \in Img : Reload(j)
RootStep == \E j \in Img : RootArrives(j)

Next ==
  \/ GoStep \/ PageStep \/ RootStep
  \/ \E i \in Img : PeerAttest(i) \/ Deliver(i) \/ WiringSave(i) \/ SpawnChild(i) \/ Request(i) \/ LiveAdopt(i)
                    \/ Quit(i) \/ WriteError(i) \/ ExecFail(i) \/ Crash(i)
  \/ Frame \/ Apply \/ Launch

\* Fairness, three weak terms (grouped: see README, OISingleInstance, on TLC's DNF blow-up):
\*  F1 every live process keeps running its own steps (the steps of one image are sequential, those of
\*     two images are finite, so weak fairness on the group is weak fairness on each);
\*  F2 a waiting page keeps polling and reloads;
\*  F3 a member of the new project is eventually online and gives the root document (an assumption:
\*     with every member gone for good the record waits, J-2, and no property but this liveness needs it).
\* None on the person (request, quit, launch), the peer, children, crashes or failed steps.
Spec == Init /\ [][Next]_vars /\ WF_vars(GoStep) /\ WF_vars(PageStep) /\ WF_vars(RootStep)

(***************************************************************************)
(* PROPERTIES                                                              *)
(***************************************************************************)
\* N-11, N-13, N-14: nothing of the old session or the old project is observable once the project
\* changed: a page bound to an image holds coverage and continuations of that image's project only and
\* opens its project's root; no change authorised under one project reaches an image running another;
\* no op of the old project is in the installed new list; no child of an older image runs beside a
\* newer one.
NoOldStateAfterChange ==
  /\ ~bad /\ ~polluted
  /\ pst = "live" => (cov \cup cont \subseteq {proj[ph]} /\ proot \in {"none", RootOf(proj[ph])})
  /\ \A j \in Img : phase[j] = "run" => children \subseteq {j}

\* N-7, J-4: nothing of the new project that is durable is lost: once installed the new list stays the
\* list, and the joined root, once recorded, stays recorded.
NewStateSurvives ==
  /\ committed => trust = "new"
  /\ consumed > 0 => root = "R1"

\* N-12: at most one restart per request (no restart loop).
RestartOnce == restarts <= requests

\* N-7, J-1, J-4: whatever crashed, the folder opens, as the old project or the new one, never broken:
\* a complete list is always recoverable, and with no record left the recorded root is the project's.
OpensAfterCrash ==
  /\ trust # "none" \/ prev = "old"
  /\ pj = "none" => root = RootOf(Recovered)
  /\ Recovered = "old" => root = "R0"

\* N-12, N-13: the restart happens only once the write is finished and on disk (history: no restart
\* ever cut a write short).
WriteBeforeRestart == ~cut

\* J-1, J-2: the pending-join record is consumed at most once, and the « joined » notice shows once.
JoinOnce ==
  /\ consumed <= 1
  /\ Cardinality({j \in Img : notice[j] = "joined"}) <= 1

\* N-12, N35, ADR 0037: one process works on the profile at a time; a restarted image starts its work
\* only when the previous one has ended.
OneImage == Cardinality({j \in Img : phase[j] \in Active}) <= 1

\* N-12, J-2: the notice tells what actually loaded (the three outcomes of the write, errCommitted included).
NoticeMatchesStanding ==
  \A j \in Img :
    /\ notice[j] \in {"done", "joined"} => proj[j] = "new"
    /\ notice[j] \in {"failed", "joinFailed"} => proj[j] = "old"

\* N-9, N-12, J-2 (liveness): a requested change ends in a running image of the new project, the page
\* reloaded on it with the project's root and the notice, unless a crash, a failed step or a quit
\* interrupted it.
Settled ==
  \E j \in Img : /\ phase[j] = "run" /\ ph = j /\ pst = "live" /\ proj[j] = "new"
                 /\ proot = RootOf("new") /\ notice[j] \in {"done", "joined"}
ChangeCompletes == (requests = 1) ~> (Settled \/ intr)

=============================================================================
