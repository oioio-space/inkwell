---------------------------- MODULE ConnCredit ----------------------------
(* One direction (sender s -> receiver r) of ONE QUIC connection (spec.md   *)
(* §5.1, §10.7). Every stream shares the connection receive window         *)
(* (RFC 9000 §4.1): unread bytes of any stream hold connection credit.     *)
(* Cancelling a stream (STOP_SENDING / CancelRead) does NOT return its      *)
(* credit at once: quic-go returns it when the final size is known, i.e.    *)
(* when the peer's RESET_STREAM (or FIN) arrives (RFC 9000 §3.5, §4.5) —    *)
(* modelled by the deferred step ResetArrives. A Byzantine peer may never   *)
(* send it: the receiver then tears the session down (Teardown, 10 s).      *)
(* The webview may stop consuming forever (Stuck). Property: every wanted   *)
(* CONTROL frame is eventually read, or the session is torn down.          *)
(* Rules: CONTROL always drained; at most one cancelled stream pending;     *)
(* LARGE opened only after LARGE_REPLY (read into a reservation); on a      *)
(* generation change only REPO is cancelled — an inbound LARGE of the old    *)
(* generation is read to its end and dropped, so it never holds credit      *)
(* (spec §5.1 c): Switch therefore touches REPO only, by design.            *)
EXTENDS Naturals

CONSTANTS Wc, Ws, QMax, MaxRepo, MaxLarge, MaxCtl, MaxSwitch,
          PullLarge,     \* rule: LARGE only after LARGE_REPLY
          Honest,        \* the peer answers STOP_SENDING with RESET_STREAM
          TeardownRule   \* rule: no RESET within 10 s -> close the session

VARIABLES repo, pending, large, lstate, ctl, q, stuck, torn,
          nRepo, nLarge, nCtlSent, nCtlRead, nSwitch

vars == <<repo, pending, large, lstate, ctl, q, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>

Credit == Wc - (repo + pending + large + ctl)

Init == repo = 0 /\ pending = 0 /\ large = 0 /\ lstate = "none" /\ ctl = 0 /\ q = 0
        /\ stuck = FALSE /\ torn = FALSE /\ nRepo = 0 /\ nLarge = 0 /\ nCtlSent = 0
        /\ nCtlRead = 0 /\ nSwitch = 0

Live == ~torn

SendRepo == /\ Live /\ nRepo < MaxRepo /\ repo < Ws /\ Credit >= 1
            /\ repo' = repo + 1 /\ nRepo' = nRepo + 1
            /\ UNCHANGED <<pending, large, lstate, ctl, q, stuck, torn, nLarge, nCtlSent, nCtlRead, nSwitch>>
ReadRepo == /\ Live /\ repo > 0 /\ q < QMax
            /\ repo' = repo - 1 /\ q' = q + 1
            /\ UNCHANGED <<pending, large, lstate, ctl, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Deliver  == /\ Live /\ q > 0 /\ ~stuck /\ q' = q - 1
            /\ UNCHANGED <<repo, pending, large, lstate, ctl, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Stall    == /\ ~stuck /\ stuck' = TRUE
            /\ UNCHANGED <<repo, pending, large, lstate, ctl, q, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
\* Generation change: the current REPO stream is cancelled; its unread data
\* keeps holding credit (pending) until the peer's RESET_STREAM. At most one
\* cancelled stream may be pending (rule), so the switch waits for it.
Switch   == /\ Live /\ nSwitch < MaxSwitch /\ pending = 0
            /\ pending' = repo /\ repo' = 0 /\ nSwitch' = nSwitch + 1
            /\ UNCHANGED <<large, lstate, ctl, q, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead>>
ResetArrives == /\ Live /\ Honest /\ pending > 0 /\ pending' = 0
                /\ UNCHANGED <<repo, large, lstate, ctl, q, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Teardown == /\ Live /\ TeardownRule /\ pending > 0 /\ torn' = TRUE
            /\ UNCHANGED <<repo, pending, large, lstate, ctl, q, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Announce == /\ Live /\ lstate = "none" /\ lstate' = IF PullLarge THEN "announced" ELSE "open"
            /\ UNCHANGED <<repo, pending, large, ctl, q, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Pull     == /\ Live /\ lstate = "announced" /\ lstate' = "open"
            /\ UNCHANGED <<repo, pending, large, ctl, q, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
SendLarge == /\ Live /\ lstate = "open" /\ nLarge < MaxLarge /\ large < Ws /\ Credit >= 1
             /\ large' = large + 1 /\ nLarge' = nLarge + 1
             /\ UNCHANGED <<repo, pending, lstate, ctl, q, stuck, torn, nRepo, nCtlSent, nCtlRead, nSwitch>>
ReadLarge == /\ Live /\ large > 0 /\ (PullLarge \/ ~stuck)
             /\ large' = large - 1
             /\ lstate' = IF large = 1 /\ nLarge = MaxLarge THEN "done" ELSE lstate
             /\ UNCHANGED <<repo, pending, ctl, q, stuck, torn, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
SendCtl  == /\ Live /\ nCtlSent < MaxCtl /\ Credit >= 1
            /\ ctl' = ctl + 1 /\ nCtlSent' = nCtlSent + 1
            /\ UNCHANGED <<repo, pending, large, lstate, q, stuck, torn, nRepo, nLarge, nCtlRead, nSwitch>>
ReadCtl  == /\ Live /\ ctl > 0 /\ ctl' = ctl - 1 /\ nCtlRead' = nCtlRead + 1
            /\ UNCHANGED <<repo, pending, large, lstate, q, stuck, torn, nRepo, nLarge, nCtlSent, nSwitch>>

Next == SendRepo \/ ReadRepo \/ Deliver \/ Stall \/ Switch \/ ResetArrives \/ Teardown
        \/ Announce \/ Pull \/ SendLarge \/ ReadLarge \/ SendCtl \/ ReadCtl

\* No fairness for un-stalling the webview; Teardown is weakly fair (the 10 s
\* timer fires); ResetArrives is weakly fair for an honest peer.
Spec == Init /\ [][Next]_vars /\ WF_vars(SendCtl) /\ WF_vars(ReadCtl) /\ WF_vars(ReadRepo)
        /\ WF_vars(Pull) /\ WF_vars(ReadLarge) /\ WF_vars(SendLarge) /\ WF_vars(Deliver)
        /\ WF_vars(ResetArrives) /\ WF_vars(Teardown)

CreditOK == Credit >= 0
OnePending == pending <= Ws
ControlLive == <>(nCtlRead = MaxCtl \/ torn)
=============================================================================
