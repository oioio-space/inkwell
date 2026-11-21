----------------------------- MODULE WindowClose -----------------------------
EXTENDS Naturals, FiniteSets

CONSTANTS Windows, Attempts, Environment, Fault
VARIABLES win, phase, attempt, targets, expired, permits

vars == <<win, phase, attempt, targets, expired, permits>>
Init ==
    /\ phase = "idle"
    /\ attempt = 0
    /\ targets = {}
    /\ expired = FALSE
    /\ permits = {}
    /\ win = [w \in Windows |-> [
           typed |-> 0, disk |-> 0, live |-> Environment # "crashed",
           held |-> FALSE, cut |-> 0, saved |-> 0, ack |-> 0, goodbye |-> FALSE,
           launchPending |-> TRUE, result |-> "open", closedAttempt |-> 0, consentAttempt |-> 0
       ]]

Active(w) == phase = "drain" /\ w \in targets
OpenWindows == {w \in Windows : win[w].result = "open"}
Ready(w) ==
    LET s == win[w] IN
    s.held /\ s.live /\ s.ack = attempt /\ ~s.launchPending

Type(w) ==
    LET s == win[w] IN
    /\ w \in OpenWindows /\ s.live /\ s.typed < 2
    /\ (~s.held \/ (Fault = "typeWhileHeld" /\ Active(w)))
    /\ win' = [win EXCEPT ![w].typed = @ + 1]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

\* A transaction targets one native window or all open editors on quit.
\* A retry uses the same scope and a new attempt. The save/close ACKs keep
\* their original attempt number so a stale-ACK fault can be observed.
Request(group) ==
    /\ (phase = "idle" \/ (phase = "notice" /\ group = targets))
    /\ group # {} /\ (Cardinality(group) = 1 \/ group = OpenWindows)
    /\ (Environment = "mixed" \/ group = Windows)
    /\ group \subseteq OpenWindows /\ attempt < Attempts
    /\ phase' = "drain" /\ attempt' = attempt + 1 /\ targets' = group
    /\ expired' = FALSE
    /\ win' = [w \in Windows |-> IF w \in group
               THEN [win[w] EXCEPT !.held = FALSE, !.goodbye = FALSE] ELSE win[w]]
    /\ UNCHANGED permits

Prepare(w) ==
    LET s == win[w] IN
    /\ Active(w) /\ s.live /\ ~s.held
    /\ win' = [win EXCEPT ![w].held = TRUE, ![w].cut = s.typed]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

Write(w) ==
    LET s == win[w] IN
    /\ Active(w) /\ s.held /\ s.live /\ s.disk < s.cut /\ Fault # "noWrite"
    /\ win' = [win EXCEPT ![w].disk = s.cut]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

SaveAck(w) ==
    LET s == win[w] IN
    /\ Active(w) /\ s.held /\ s.live /\ s.saved # attempt
    /\ (s.disk = s.cut \/ Fault = "ackBeforeWrite")
    /\ win' = [win EXCEPT ![w].saved = attempt]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

LaunchHandled(w) ==
    LET s == win[w] IN
    /\ Active(w) /\ s.held /\ s.live /\ s.launchPending
    /\ win' = [win EXCEPT ![w].launchPending = FALSE]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

\* Existing two-second OI allowance. This cannot stand in for a save ACK.
LaunchDeadline(w) ==
    /\ Environment = "mixed" /\ Active(w) /\ win[w].launchPending
    /\ win' = [win EXCEPT ![w].launchPending = FALSE]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

Goodbye(w) ==
    LET s == win[w] IN
    /\ Active(w) /\ s.live /\ s.saved = attempt /\ ~s.goodbye
    /\ Fault # "noGoodbye"
    /\ win' = [win EXCEPT ![w].goodbye = TRUE]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

Ack(w) ==
    LET s == win[w] IN
    /\ Active(w) /\ s.live /\ s.saved = attempt /\ s.ack # attempt
    /\ (s.goodbye \/ Fault = "noGoodbye")
    /\ win' = [win EXCEPT ![w].ack = attempt]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

Authorize ==
    /\ phase = "drain" /\ ~expired
    /\ LET ready == {w \in targets : Ready(w)}
           partial == Fault = "partialQuit" /\ Cardinality(targets) > 1 /\ ready # {}
       IN /\ ready = targets \/ partial
             \/ (Fault = "skipWait" /\ \A w \in targets : win[w].held)
             \/ (Fault = "staleAck" /\ \A w \in targets :
                     win[w].held /\ win[w].ack > 0 /\ ~win[w].launchPending)
          /\ permits' = IF partial THEN ready ELSE targets
    /\ phase' = "approved"
    /\ UNCHANGED <<win, attempt, targets, expired>>

Abort ==
    /\ phase' = IF phase \in {"approved", "forced"} THEN "idle" ELSE "notice"
    /\ permits' = {}
    /\ win' = [w \in Windows |-> IF w \in targets \cap OpenWindows
                THEN [win[w] EXCEPT !.held = FALSE, !.saved = 0, !.goodbye = FALSE] ELSE win[w]]
    /\ UNCHANGED <<attempt, targets, expired>>

\* Revocation clears any remaining permits before input can resume,
\* for example when a replacement page becomes editable.
RevokeApproval ==
    /\ Environment = "mixed" /\ phase \in {"approved", "forced"}
    /\ Abort

\* ClockElapsed is the environment crossing the one shared deadline.
\* Healthy runs assume the prerequisites finish within the time budget.
ClockElapsed ==
    /\ Environment = "mixed" /\ phase = "drain" /\ ~expired
    /\ expired' = TRUE
    /\ UNCHANGED <<win, phase, attempt, targets, permits>>

Expire ==
    /\ phase = "drain" /\ expired
    /\ IF Fault = "timeoutCloses"
          THEN /\ phase' = "approved" /\ permits' = targets
               /\ UNCHANGED <<win, attempt, targets, expired>>
          ELSE Abort

SaveFailed ==
    /\ Environment = "mixed" /\ phase = "drain"
    /\ \E w \in targets : win[w].held /\ win[w].disk < win[w].cut
    /\ Abort

Crash(w) ==
    /\ Environment = "mixed" /\ w \in OpenWindows /\ win[w].live
    /\ win' = [win EXCEPT ![w].live = FALSE]
    /\ UNCHANGED <<phase, attempt, targets, expired, permits>>

RendererLost ==
    /\ phase = "drain" /\ \E w \in targets : ~win[w].live
    /\ Abort

\* The person can leave a notice open, retry, or explicitly accept loss.
ChooseForce ==
    /\ phase = "notice"
    /\ phase' = "consented"
    /\ win' = [w \in Windows |-> IF w \in targets
                THEN [win[w] EXCEPT !.consentAttempt = attempt] ELSE win[w]]
    /\ UNCHANGED <<attempt, targets, expired, permits>>

\* The native coordinator freezes input and authorizes force without a page reply.
Force ==
    /\ (phase = "consented" \/ (Fault = "forceWithoutConsent" /\ phase = "notice"))
    /\ (Fault # "forceNeedsLive" \/ \A w \in targets : win[w].live)
    /\ phase' = "forced" /\ permits' = targets
    /\ win' = [w \in Windows |-> IF w \in targets
                THEN [win[w] EXCEPT !.held = TRUE, !.cut = win[w].typed] ELSE win[w]]
    /\ UNCHANGED <<attempt, targets, expired>>

\* The first close event for an approved window consumes its permit,
\* whether it comes from the OS or the coordinator's Close() call.
\* Without a permit, the hook vetoes: Request can start a drain and
\* a duplicate request while draining leaves the state unchanged.
CloseEvent(w) ==
    /\ w \in permits /\ phase \in {"approved", "forced"}
    /\ permits' = permits \ {w}
    /\ phase' = IF permits' = {} THEN "idle" ELSE phase
    /\ win' = [win EXCEPT ![w].result = phase, ![w].closedAttempt = attempt]
    /\ UNCHANGED <<attempt, targets, expired>>

Next ==
    \/ \E group \in SUBSET Windows : Request(group)
    \/ Authorize \/ ClockElapsed \/ Expire \/ SaveFailed \/ RendererLost \/ RevokeApproval
    \/ ChooseForce \/ Force
    \/ \E w \in Windows :
         Type(w) \/ Prepare(w) \/ Write(w) \/ LaunchHandled(w) \/ LaunchDeadline(w)
         \/ SaveAck(w) \/ Goodbye(w) \/ Ack(w) \/ Crash(w) \/ CloseEvent(w)
    \/ UNCHANGED vars

Spec ==
    /\ Init /\ [][Next]_vars
    /\ WF_vars(Authorize) /\ WF_vars(ClockElapsed) /\ WF_vars(Expire)
    /\ WF_vars(RendererLost) /\ WF_vars(Force)
    /\ \A w \in Windows :
        /\ WF_vars(Prepare(w)) /\ WF_vars(Write(w)) /\ WF_vars(LaunchHandled(w))
        /\ WF_vars(SaveAck(w)) /\ WF_vars(Goodbye(w)) /\ WF_vars(Ack(w))
        /\ WF_vars(CloseEvent(w))
    /\ IF Environment = "mixed" THEN TRUE ELSE WF_vars(Request(Windows))
    /\ IF Environment = "crashed" THEN WF_vars(ChooseForce) ELSE TRUE

InputFrozen ==
    \A w \in OpenWindows : win[w].held => win[w].typed = win[w].cut

AckIsDurable ==
    \A w \in targets :
        win[w].held /\ win[w].saved = attempt => win[w].disk = win[w].cut

NoAcknowledgedKeystrokeLost ==
    \A w \in Windows : win[w].result = "approved" => win[w].disk = win[w].typed

CloseWaitedForAck ==
    \A w \in Windows : win[w].result = "approved" =>
        win[w].ack = win[w].closedAttempt /\ ~win[w].launchPending

GoodbyeBeforeClose ==
    \A w \in Windows : win[w].result = "approved" => win[w].goodbye

GroupWaitedForAll ==
    phase = "approved" /\ Cardinality(targets) > 1 =>
        \A w \in targets : win[w].ack = attempt /\ ~win[w].launchPending

OnlyWithConsent ==
    /\ (phase = "forced" => \A w \in permits : win[w].consentAttempt = attempt)
    /\ \A w \in Windows : win[w].result = "forced" =>
        win[w].consentAttempt = win[w].closedAttempt

CloseResolves == (phase = "drain") ~> (phase # "drain")
HealthyCloseSucceeds == <>(\A w \in Windows : win[w].result = "approved")
ForceClosesCrashedWindow == <>(\A w \in Windows : win[w].result = "forced")
=============================================================================
