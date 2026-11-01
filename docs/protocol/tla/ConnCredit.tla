---------------------------- MODULE ConnCredit ----------------------------
(* One direction (sender s -> receiver r) of ONE QUIC connection (spec.md   *)
(* §10.2, §10.7). Every stream shares the connection receive window        *)
(* (RFC 9000 §4.1): unread bytes of any stream hold connection credit until *)
(* the application reads them. Units are abstract (1 unit = 1 frame).       *)
(* The receiver's webview may stop consuming forever (Stuck). The property *)
(* checked: CONTROL frames are still delivered (control never waits behind *)
(* data), because (a) CONTROL and PRESENCE are always drained, (b) the old *)
(* REPO stream is cancelled at once on a generation change (CancelRead     *)
(* returns its credit), (c) a LARGE stream is opened only after the        *)
(* receiver's LARGE_PULL, so it is always read into a reservation, and     *)
(* (d) Ws <= Wc - 1, so the one stream that may stay unread (REPO) cannot  *)
(* take the last unit. Constants PullLarge and CancelOld turn rules (b)    *)
(* and (c) off to show they are necessary.                                 *)
EXTENDS Naturals

CONSTANTS Wc, Ws, QMax, MaxRepo, MaxLarge, MaxCtl, MaxSwitch, PullLarge, CancelOld

VARIABLES repo,      \* unread units of the current REPO stream (in the transport)
          old,       \* unread units of the previous REPO stream
          large,     \* unread units of the LARGE stream
          lstate,    \* "none" | "announced" | "open" | "done"
          ctl,       \* unread CONTROL units
          q,         \* receiver REPO queue (towards the webview)
          stuck,     \* the webview stopped consuming (forever)
          nRepo, nLarge, nCtlSent, nCtlRead, nSwitch

vars == <<repo, old, large, lstate, ctl, q, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>

Credit == Wc - (repo + old + large + ctl)

Init == repo = 0 /\ old = 0 /\ large = 0 /\ lstate = "none" /\ ctl = 0 /\ q = 0
        /\ stuck = FALSE /\ nRepo = 0 /\ nLarge = 0 /\ nCtlSent = 0 /\ nCtlRead = 0 /\ nSwitch = 0

SendRepo == /\ nRepo < MaxRepo /\ repo < Ws /\ Credit >= 1
            /\ repo' = repo + 1 /\ nRepo' = nRepo + 1
            /\ UNCHANGED <<old, large, lstate, ctl, q, stuck, nLarge, nCtlSent, nCtlRead, nSwitch>>
ReadRepo == /\ repo > 0 /\ q < QMax
            /\ repo' = repo - 1 /\ q' = q + 1
            /\ UNCHANGED <<old, large, lstate, ctl, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Deliver  == /\ q > 0 /\ ~stuck /\ q' = q - 1
            /\ UNCHANGED <<repo, old, large, lstate, ctl, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
Stall    == /\ ~stuck /\ stuck' = TRUE
            /\ UNCHANGED <<repo, old, large, lstate, ctl, q, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
\* Generation change: a new REPO stream; the old one is cancelled at once (rule)
\* or, in the faulty variant, left unread.
Switch   == /\ nSwitch < MaxSwitch
            /\ old' = IF CancelOld THEN 0 ELSE old + repo
            /\ repo' = 0 /\ nSwitch' = nSwitch + 1
            /\ UNCHANGED <<large, lstate, ctl, q, stuck, nRepo, nLarge, nCtlSent, nCtlRead>>
Announce == /\ lstate = "none" /\ lstate' = IF PullLarge THEN "announced" ELSE "open"
            /\ UNCHANGED <<repo, old, large, ctl, q, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
\* The receiver reserves memory and sends LARGE_PULL (its reservation does not
\* depend on the webview); only then does the sender open the stream.
Pull     == /\ lstate = "announced" /\ lstate' = "open"
            /\ UNCHANGED <<repo, old, large, ctl, q, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
SendLarge == /\ lstate = "open" /\ nLarge < MaxLarge /\ large < Ws /\ Credit >= 1
             /\ large' = large + 1 /\ nLarge' = nLarge + 1
             /\ UNCHANGED <<repo, old, lstate, ctl, q, stuck, nRepo, nCtlSent, nCtlRead, nSwitch>>
\* Pulled: read into the reservation, always. Not pulled (faulty variant): the
\* receiver reads it only when it can deliver, i.e. not while stuck.
ReadLarge == /\ large > 0 /\ (PullLarge \/ ~stuck)
             /\ large' = large - 1
             /\ lstate' = IF large = 1 /\ nLarge = MaxLarge THEN "done" ELSE lstate
             /\ UNCHANGED <<repo, old, ctl, q, stuck, nRepo, nLarge, nCtlSent, nCtlRead, nSwitch>>
SendCtl  == /\ nCtlSent < MaxCtl /\ Credit >= 1
            /\ ctl' = ctl + 1 /\ nCtlSent' = nCtlSent + 1
            /\ UNCHANGED <<repo, old, large, lstate, q, stuck, nRepo, nLarge, nCtlRead, nSwitch>>
ReadCtl  == /\ ctl > 0 /\ ctl' = ctl - 1 /\ nCtlRead' = nCtlRead + 1
            /\ UNCHANGED <<repo, old, large, lstate, q, stuck, nRepo, nLarge, nCtlSent, nSwitch>>

Next == SendRepo \/ ReadRepo \/ Deliver \/ Stall \/ Switch \/ Announce \/ Pull
        \/ SendLarge \/ ReadLarge \/ SendCtl \/ ReadCtl

\* Fairness for everything the receiver and sender must do; none for the
\* webview un-stalling (it may stay stuck) and none for Deliver once stuck.
Spec == Init /\ [][Next]_vars /\ WF_vars(SendCtl) /\ WF_vars(ReadCtl) /\ WF_vars(ReadRepo)
        /\ WF_vars(Pull) /\ WF_vars(ReadLarge) /\ WF_vars(SendLarge) /\ WF_vars(Deliver)

CreditOK == Credit >= 0
WindowRule == Ws <= Wc - 1
\* Control is never starved: every control frame the sender wants to send is
\* eventually read, whatever the data streams and the webview do.
ControlLive == <>(nCtlRead = MaxCtl)
=============================================================================
