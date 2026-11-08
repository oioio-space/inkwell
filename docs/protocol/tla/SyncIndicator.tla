--------------------------- MODULE SyncIndicator ---------------------------
(* W33 honest sync indicator (plan 2026-11-08-netsim-impaired-plan.md,     *)
(* "Honest sync indicator"; README.md, SyncIndicator). When may the header *)
(* of THIS device (Me) say « Synchronisé », per peer and globally?          *)
(*                                                                         *)
(* Me and the remote peers run the automerge-repo sync protocol over one   *)
(* ordered stream per direction (sync/1, or one REPO stream per generation *)
(* in v2: plan R15). A document is a set of changes; a sync message        *)
(* carries the sender's heads (abstracted as its whole, causally closed    *)
(* change set) and the changes the sender believes the receiver lacks,     *)
(* minus those already sent (Automerge sentHashes: a lost change is never  *)
(* resent until the sync state is reset, plan R1). A received change is    *)
(* applied only once its causal dependencies are present. Me is the hub:   *)
(* remote peers sync with Me only, so a change of p1 reaches p2 through Me. *)
(*                                                                         *)
(* What the webview can observe, and nothing else, feeds the indicator:    *)
(*  - up[p]       : peer-candidate / peer-disconnected (PeerUp/PeerDown);  *)
(*  - their[Me,p] : heads of the last sync message received from p, as     *)
(*                  WailsNetworkAdapter can decode them (heard = one came); *)
(*  - doc[Me]     : my own heads;                                           *)
(*  - snap[p]     : pendingMsgs of the coalesced Links notification (≤1 Hz, *)
(*                  stale between samples).                                 *)
(* The ground truth (doc[p], in-flight messages) is used only by the        *)
(* properties, never by the indicator.                                      *)
EXTENDS Naturals, FiniteSets, Sequences

CONSTANTS Me,        \* this device (model value)
          Peers,     \* the other members' devices (model values)
          MaxEdits,  \* changes each device makes in a run
          MaxLoss,   \* messages lost in transport per run (each one owes rule L)
          MaxDown,   \* connection losses (PeerDown) per run
          QMax       \* messages in flight per direction (outbox + wire)

Nodes   == {Me} \cup Peers
Changes == Nodes \X (1..MaxEdits)
Links   == ({Me} \X Peers) \cup (Peers \X {Me})   \* <<sender, receiver>>
PeerOf(l) == IF l[1] = Me THEN l[2] ELSE l[1]
Rev(l)    == <<l[2], l[1]>>
PairOf(p) == {<<Me, p>>, <<p, Me>>}
Msgs    == [heads : SUBSET Changes, chg : SUBSET Changes]

VARIABLES doc,       \* doc[n] : changes applied by device n
          deps,      \* deps[c] : what its author had when it made c
          up,        \* up[p] : Me and p see each other (peer announced)
          chan,      \* chan[l] : sync messages in flight on link l
          heard,     \* heard[<<x,y>>] : x received a sync message from y since the last reset
          their,     \* their[<<x,y>>] : heads of the last message x received from y
          sent,      \* sent[<<x,y>>] : changes x already sent to y (sentHashes)
          responded, \* responded[<<x,y>>] : x sent y at least one message since the reset
          lastSent,  \* lastSent[<<x,y>>] : heads x last sent to y (lastSentHeads)
          owed,      \* owed[p] : a transport loss on Me-p awaits its rule L resync
          snap,      \* snap[p] : pendingMsgs towards p in the last Links notification
          losses, downs

vars == <<doc, deps, up, chan, heard, their, sent, responded, lastSent, owed,
          snap, losses, downs>>
syncState == <<heard, their, sent, responded, lastSent>>

TypeOK ==
  /\ doc \in [Nodes -> SUBSET Changes]
  /\ deps \in [Changes -> SUBSET Changes]
  /\ up \in [Peers -> BOOLEAN]
  /\ \A l \in Links : chan[l] \in Seq(Msgs) /\ Len(chan[l]) <= QMax
  /\ heard \in [Links -> BOOLEAN]
  /\ their \in [Links -> SUBSET Changes]
  /\ sent \in [Links -> SUBSET Changes]
  /\ responded \in [Links -> BOOLEAN]
  /\ lastSent \in [Links -> SUBSET Changes]
  /\ owed \in [Peers -> BOOLEAN]
  /\ snap \in [Peers -> 0..QMax]
  /\ losses \in 0..MaxLoss /\ downs \in 0..MaxDown

Init ==
  /\ doc = [n \in Nodes |-> {}]
  /\ deps = [c \in Changes |-> {}]
  /\ up = [p \in Peers |-> TRUE]
  /\ chan = [l \in Links |-> <<>>]
  /\ heard = [l \in Links |-> FALSE]
  /\ their = [l \in Links |-> {}]
  /\ sent = [l \in Links |-> {}]
  /\ responded = [l \in Links |-> FALSE]
  /\ lastSent = [l \in Links |-> {}]
  /\ owed = [p \in Peers |-> FALSE]
  /\ snap = [p \in Peers |-> 0]
  /\ losses = 0 /\ downs = 0

------------------------------------------------------------------------------
\* The environment and the sync protocol.

Own(n) == {c \in doc[n] : c[1] = n}

\* A device edits, connected or not (offline edits).
Edit(n) ==
  LET k == Cardinality(Own(n)) IN
  /\ k < MaxEdits
  /\ deps' = [deps EXCEPT ![<<n, k + 1>>] = doc[n]]
  /\ doc' = [doc EXCEPT ![n] = @ \cup {<<n, k + 1>>}]
  /\ UNCHANGED <<up, chan, syncState, owed, snap, losses, downs>>

\* Changes the sender believes the receiver lacks and has not sent yet.
ToSend(l) == {c \in doc[l[1]] \ sent[l] : c \notin their[l]}

\* generateSyncMessage: a first message after a reset, new heads, or changes to send.
Send(l) ==
  /\ up[PeerOf(l)]
  /\ ~responded[l] \/ lastSent[l] # doc[l[1]] \/ ToSend(l) # {}
  /\ Len(chan[l]) < QMax
  /\ chan' = [chan EXCEPT ![l] = Append(@, [heads |-> doc[l[1]], chg |-> ToSend(l)])]
  /\ sent' = [sent EXCEPT ![l] = @ \cup ToSend(l)]
  /\ responded' = [responded EXCEPT ![l] = TRUE]
  /\ lastSent' = [lastSent EXCEPT ![l] = doc[l[1]]]
  /\ UNCHANGED <<doc, deps, up, heard, their, owed, snap, losses, downs>>

\* The largest subset of S whose dependencies are in D or in it (causally ready).
Ready(D, S) == UNION {X \in SUBSET S : \A c \in X : deps[c] \subseteq D \cup X}

\* receiveSyncMessage at l[2]: apply what is ready, record the sender's heads.
Receive(l) ==
  /\ chan[l] # <<>>
  /\ LET m == Head(chan[l]) IN
     /\ doc' = [doc EXCEPT ![l[2]] = @ \cup Ready(@, m.chg)]
     /\ heard' = [heard EXCEPT ![Rev(l)] = TRUE]
     /\ their' = [their EXCEPT ![Rev(l)] = m.heads]
  /\ chan' = [chan EXCEPT ![l] = Tail(@)]
  /\ UNCHANGED <<deps, up, sent, responded, lastSent, owed, snap, losses, downs>>

\* A transport loss (write given up, outbox overflow): the message is gone, the
\* connection survives. Spec §5.5 rule L: such a loss is never silent.
Lose(l) ==
  /\ losses < MaxLoss
  /\ chan[l] # <<>>
  /\ chan' = [chan EXCEPT ![l] = Tail(@)]
  /\ owed' = [owed EXCEPT ![PeerOf(l)] = TRUE]
  /\ losses' = losses + 1
  /\ UNCHANGED <<doc, deps, up, syncState, snap, downs>>

\* Both repos drop their sync state with p (peer-disconnected, then a fresh one).
Reset(p) ==
  /\ chan' = [l \in Links |-> IF l \in PairOf(p) THEN <<>> ELSE chan[l]]
  /\ heard' = [l \in Links |-> IF l \in PairOf(p) THEN FALSE ELSE heard[l]]
  /\ their' = [l \in Links |-> IF l \in PairOf(p) THEN {} ELSE their[l]]
  /\ sent' = [l \in Links |-> IF l \in PairOf(p) THEN {} ELSE sent[l]]
  /\ responded' = [l \in Links |-> IF l \in PairOf(p) THEN FALSE ELSE responded[l]]
  /\ lastSent' = [l \in Links |-> IF l \in PairOf(p) THEN {} ELSE lastSent[l]]
  /\ owed' = [owed EXCEPT ![p] = FALSE]

\* Spec §5.5 rule L on sync/1 (RL1): the failed send raises PeerDown then PeerUp,
\* so automerge-repo resets its sync state and sends the lost change again.
Resync(p) ==
  /\ up[p] /\ owed[p]
  /\ Reset(p)
  /\ UNCHANGED <<doc, deps, up, snap, losses, downs>>

\* The connection is lost (PeerDown), then comes back (PeerUp, new handshake).
Down(p) ==
  /\ up[p] /\ downs < MaxDown
  /\ up' = [up EXCEPT ![p] = FALSE]
  /\ Reset(p)
  /\ downs' = downs + 1
  /\ UNCHANGED <<doc, deps, snap, losses>>

Up(p) ==
  /\ ~up[p]
  /\ up' = [up EXCEPT ![p] = TRUE]
  /\ UNCHANGED <<doc, deps, chan, syncState, owed, snap, losses, downs>>

\* The coalesced Links notification (plan §7): pendingMsgs towards p, when it changed.
Sample(p) ==
  /\ snap[p] # Len(chan[<<Me, p>>])
  /\ snap' = [snap EXCEPT ![p] = Len(chan[<<Me, p>>])]
  /\ UNCHANGED <<doc, deps, up, chan, syncState, owed, losses, downs>>

Next ==
  \/ \E n \in Nodes : Edit(n)
  \/ \E l \in Links : Send(l) \/ Receive(l) \/ Lose(l)
  \/ \E p \in Peers : Resync(p) \/ Down(p) \/ Up(p) \/ Sample(p)

\* The network is eventually quiet: edits, losses and outages are bounded and get
\* no fairness; the protocol, rule L and reconnection are weakly fair.
Fair ==
  /\ \A l \in Links : WF_vars(Send(l)) /\ WF_vars(Receive(l))
  /\ \A p \in Peers : WF_vars(Resync(p)) /\ WF_vars(Up(p)) /\ WF_vars(Sample(p))

Spec == Init /\ [][Next]_vars /\ Fair

------------------------------------------------------------------------------
\* The indicator: a pure function of what Me observes (rule stated in the plan).
\* Concretely, with myHeads = A.getHeads(doc) and theirHeads from the last sync
\* message received from p: "behind" <=> !A.hasHeads(doc, theirHeads);
\* "synced" <=> sameHeads(myHeads, theirHeads).

\* I know p has a change I lack (« Pas encore à jour »).
MissingIn(p) == heard[<<Me, p>>] /\ ~(their[<<Me, p>>] \subseteq doc[Me])
\* p has not acknowledged every change of mine (« Envoi en cours (n) »).
PendingOut(p) == heard[<<Me, p>>] /\ ~(doc[Me] \subseteq their[<<Me, p>>])

SoundPeerState(p) ==
  CASE ~up[p]               -> "offline"    \* not connected
    [] ~heard[<<Me, p>>]    -> "checking"   \* connected, nothing heard since the (re)connection
    [] MissingIn(p)         -> "behind"
    [] PendingOut(p)        -> "sending"
    [] OTHER                -> "synced"

Connected == {p \in Peers : up[p]}

SoundGlobal(state(_)) ==
  CASE Connected = {}                           -> "offline"   \* alone / connecting
    [] \E p \in Connected : state(p) = "behind"   -> "behind"
    [] \E p \in Connected : state(p) = "sending"  -> "sending"
    [] \E p \in Connected : state(p) = "checking" -> "checking"
    [] OTHER                                      -> "synced"

\* The two names the mutants override (README.md, SyncIndicator mutants).
PeerState(p) == SoundPeerState(p)
Global == SoundGlobal(PeerState)

------------------------------------------------------------------------------
\* Safety, against the ground truth.

\* What a device heard another has, that one has (heads only grow).
TheirSound == \A l \in Links : their[l] \subseteq doc[l[2]]

\* W33 per peer: « Synchronisé » for p only if p is connected, really holds every
\* change of mine, and I hold every change p reported.
PeerHonest ==
  \A p \in Peers : PeerState(p) = "synced" =>
    /\ up[p]
    /\ doc[Me] \subseteq doc[p]
    /\ heard[<<Me, p>>] /\ their[<<Me, p>>] \subseteq doc[Me]

\* W33 globally: « Synchronisé » only with someone connected, and then for every
\* connected member: nothing of mine unacknowledged, no known remote change missing.
GlobalHonest ==
  Global = "synced" =>
    /\ Connected # {}
    /\ \A p \in Connected :
         /\ doc[Me] \subseteq doc[p]
         /\ heard[<<Me, p>>] /\ their[<<Me, p>>] \subseteq doc[Me]

\* Liveness: once the network is quiet and connected, the header reaches « Synchronisé » and stays.
EventuallySynced == <>[](Global = "synced")
==============================================================================
