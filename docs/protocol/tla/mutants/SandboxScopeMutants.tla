------------------------- MODULE SandboxScopeMutants -------------------------
(* MUTANTS and WITNESSES (README.md, SandboxScope): broken rules of ADR 0042  *)
(* (amendment A1-A4, A8) that the model must reject, and reachability        *)
(* witnesses. Each mutant config overrides exactly one rule of SandboxScope  *)
(* with one of these ("Rule <- Mutant"); the sound module is never edited.   *)
EXTENDS SandboxScope

NotBorn == FALSE                    \* NotBornInside: PIDs of a running launcher handed to systemd (ADR point 1)
NoFirstCheck == FALSE               \* ExecBeforeCheck: the membership check comes after the bootstrap's work
EarlyCleanup == TRUE                \* CleanupBeforeCounters: cgroup.kill and rmdir before reading the counters
UnreadZero(o) ==                    \* UnreadAsZero: a failed read or an overflow counts as zero
  IF o.bad THEN "none" ELSE SoundCause(o)
KillIsLimit(o) ==                   \* VictimAsLimit: any local oom_kill is the limit (no local oom check)
  IF o.bad THEN "unknown" ELSE IF o.kill = "none" THEN "none" ELSE "local-limit"
CountersOnly(o) ==                  \* MixedHistoryAsLimit: oom >= 1 and oom_kill >= 1, the timeline ignored
  IF o.bad THEN "unknown"
  ELSE IF o.kill = "none" THEN "none"
  ELSE IF (o.hist # "none" \/ o.kill = "same") /\ ~o.oth /\ ~o.anc THEN "local-limit"
  ELSE "victim"
ContinueOnFail == "continue"        \* ContinueAfterPartialSetup: a refused or ignored limit write is not acted on
OtherSlice == "other"               \* SessionUnbounded: the scope is not placed in the app's slice
JobDone(u, b, m, w) == u # "fail"   \* JobDoneIsConfined: job result "done" taken as confinement
Raised(mode) ==                     \* RaiseAddressLimit: RLIMIT_AS raised "so that the cgroup fires first"
  IF mode = "cgroup" THEN "raised" ELSE "limit"
KeepContainment == "cgroup"         \* HolderDeathEscapes: the retry after a dead session keeps Containment cgroup
NoReap == FALSE                     \* OrphanAfterAppKill: the holder ignores EOF
NoPdeathsig == FALSE                \* NoPdeathsig: the namespace init does not die with the app (A9, H6)
NoCgroupPidNs == FALSE              \* NoPidNamespace: a cgroup launch path without PID namespace (A9, H7)
ErrorRetry == "error"               \* ErrorOnFallback: a placement error returns an error instead of the fallback
WaitScope == "wait"                 \* WaitForScope: a failed setup makes runs wait for a session

(* WITNESSES (anti-vacuity): each invariant must be VIOLATED by the sound    *)
(* model, proving the behaviour is reachable.                                *)
NoLocalLimit == \A r \in Runs : R[r].cause # "local-limit"
NoAddressDeath == \A r \in Runs : R[r].truth # "address"
NoFallback == \A r \in Runs : R[r].res # "rlimit"
NoVictim == \A r \in Runs : R[r].cause # "victim"
\* A non-fatal local oom, then a kill by someone else, read as victim (A8, M6).
NoMixedHistory == ~\E r \in Runs : R[r].hist # "none" /\ R[r].truth = "victim" /\ R[r].cause = "victim"
\* A run ends confined in its cgroup although its session's holder was killed meanwhile (H3).
NoHolderDeath == ~\E r \in Runs : R[r].res = "cgroup" /\ HolderKilled(r)

(* RESIDUAL WITNESS (README.md, SandboxScope, residual RS6): must be         *)
(* VIOLATED by the sound model, so that the README never claims a guarantee  *)
(* the design does not give.                                                 *)
\* A run waits for R7 (outer dead, app alive) while its run cgroup is already gone with its collected
\* scope: its read fails, so its cause is "unknown" (never zero). In this state nothing but a collection
\* removes a run cgroup (R8 comes after R7, the holder reaps only after the app died).
NoCollectedBeforeRead ==
  ~\E r \in Runs : app /\ R[r].mode = "cgroup" /\ R[r].pc = "wait" /\ R[r].rcg = "none" /\ ~R[r].ref
=============================================================================
