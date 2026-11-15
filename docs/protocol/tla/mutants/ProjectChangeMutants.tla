------------------------ MODULE ProjectChangeMutants ------------------------
(* MUTANTS and WITNESSES (README.md, ProjectChange): broken rules of the    *)
(* plan docs/plans/2026-11-14-pc1-project-change-by-restart.md that the     *)
(* model must reject, and reachability witnesses. Each mutant config        *)
(* overrides exactly one rule of ProjectChange with one of these            *)
(* ("Rule <- Mutant"); the sound module is never edited.                    *)
EXTENDS ProjectChange

\* LiveAdopt: the rejected design, the project adopted in the running session, adapter reset at the reply.
Yes == TRUE
\* RestartBeforeDurable: restart once the wiring is closed, before stopAll returned (MW8).
AfterWiring(i) == ~wiring[i]
\* RestartTwice: the restarted image restarts again on a quit (INKWELL_AFTER_TAKEOVER taken as a request).
AfterChangeToo(i) == req[i] # "none" \/ byRestart[i]
\* RequestBeforeNavigation: the request is sent while the page's context lives (no navigation first).
No == FALSE
\* JoinNoRestart: a join is written but only a take-over restarts.
TakeOverOnly(i) == req[i] = "takeover"
\* JoinTwice: consuming the record leaves it in project.json.
KeepRecord == pj
\* WriteBeforeWiringStopped: the write starts while the wiring runs (MW10).
Anytime(i) == TRUE
\* WriteBeforeDrain: the member-list write starts once the wiring stopped but before Go drained the bridge
\* streams (N-16): a storage write of the page still in its handler lands after the write began.
WriteBeforeDrain(i) == ~wiring[i]
\* NoBarrier: a child of the old image outlives the restart (MW11).
NoBarrier(i) == TRUE
\* SpawnNoWait: the spawned image starts its work without claiming the profile lock (N35).
NoWait(j) == TRUE
\* JoinRecordAfterInstall: the new list is installed before the pending-join record is written.
RecordLast(k) == IF k = "join" THEN <<"stage", "aside", "install", "cleanup", "record">>
                               ELSE <<"stage", "aside", "install", "cleanup">>
\* RecoverPrefersAside: at open, the set-aside list wins over the installed one.
PreferAside == IF prev = "old" THEN "old" ELSE trust
\* CarriedOutcome: the notice follows the request carried across the restart, not what loaded.
AlwaysDone(t) == "done"
\* PageNeverReloads: the page reloads only on the startup id it already knows (N-9's in-test mutant).
SameId(j) == j = ph

(* WITNESSES (anti-vacuity): each invariant must be VIOLATED by the sound   *)
(* model, proving the behaviour is reachable.                               *)
NoTakeOverDone == \A j \in Img : notice[j] # "done"
NoJoinAnnounced == ~(consumed > 0 /\ ~rk)
NoCrashAfterInstall == ~\E j \in Img : ~byRestart[j] /\ j > 1 /\ phase[j] = "run" /\ proj[j] = "new"
NoRefusedWrite == sw # "refused"
NoFailedNotice == \A j \in Img : notice[j] \notin {"failed", "joinFailed"}

=============================================================================
