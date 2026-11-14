-------------------- MODULE OISingleInstanceMutants --------------------
(* MUTANTS and WITNESSES (README.md, OISingleInstance): broken rules of     *)
(* plan docs/plans/2026-11-11-oi-single-instance-open.md section 4.1 that   *)
(* the model must reject, and reachability witnesses. Each mutant config    *)
(* overrides exactly one rule of OISingleInstance with one of these         *)
(* ("Rule <- Mutant"); the sound module is never edited.                    *)
EXTENDS OISingleInstance

NoLockLead(p) == sock.state # "live"    \* NoLock: "no socket answers, so I am first"
EarlyIdentity == TRUE                   \* EarlyIdentity: identity opened before the lock
AckNoQueue(p, q) == inbox[p]            \* AckNoQueue: acknowledged, never queued
FireAndForget == FALSE                  \* FireAndForget: exit 0 right after connecting (Wails macOS)
ReleaseEarly == TRUE                    \* LockReleasedEarly: unlock when closing starts
KeepLockOnCrash(p) == lock              \* CrashKeepsLock: a pid/O_EXCL file, stale after a crash
GiveUpClosing == TRUE                   \* GiveUpOnClosing: "closing" read as "already running"
SecondUnlinksStale == TRUE              \* SecondUnlinks: a second launch removes a "stale" socket
DialAgain == "dial"                     \* NoRetryLock: retries only reconnect
NoDrain(p) == {}                        \* NoDrain: queued, never read by the window
CloseAnyTime(p) == TRUE                 \* AckDropOnClose: the window closes with acknowledged requests not applied
\* EarlyDequeue: close once every request was delivered to the window, not once handled
QueueEmpty(p) == \A q \in inbox[p] \ done[p] : sentOn[p][q] # 0
\* StaleReportAccepted: a report frees the request whatever generation it answers
StaleReport(p, m) == m[1] \in inbox[p] \ done[p]
\* ResetAllOnDisconnect: the end of any stream gives back every request, the new stream's too
ResetAll(p, g) == [q \in Launch |-> 0]

(* WITNESSES (anti-vacuity): each invariant must be VIOLATED by the sound   *)
(* model, proving the behaviour is reachable.                               *)
NoHandoff == \A q \in Launch : pc[q] # "exit0"
NoGiveUp == \A q \in Launch : pc[q] # "gaveUp"
NoTakeover == ~\E p, q \in Launch : p # q /\ pc[p] = "crashed" /\ opened[p] # {} /\ pc[q] = "serve"
NoHandover == ~\E p, q \in Launch : p # q /\ pc[p] = "done" /\ pc[q] = "serve"
\* A report of a superseded stream arrives while the new stream holds the same request.
NoStaleReport == ~\E p, q \in Launch : <<q, 1>> \in reports[p] /\ sentOn[p][q] = 2

=======================================================================
