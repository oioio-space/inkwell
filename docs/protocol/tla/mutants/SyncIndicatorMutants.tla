------------------------ MODULE SyncIndicatorMutants ------------------------
(* MUTANTS (README.md, SyncIndicator): broken indicator rules that the model *)
(* must reject. Each mutant config overrides one name of SyncIndicator with  *)
(* one of these (TLC definition override, "Name <- Mutant"); the sound model *)
(* itself is never edited.                                                   *)
EXTENDS SyncIndicator

\* M-PeerCount: today's statusOf, « Synchronisé » as soon as the peer is connected.
PeerCountState(p) == IF up[p] THEN "synced" ELSE "offline"

\* M-IgnoreRemote: only my side is checked; the peer's reported heads are ignored.
IgnoreRemoteState(p) ==
  CASE ~up[p]                                    -> "offline"
    [] ~heard[<<Me, p>>]                         -> "checking"
    [] ~(doc[Me] \subseteq their[<<Me, p>>])     -> "sending"
    [] OTHER                                     -> "synced"

\* M-SentIsAcked: what I sent counts as acknowledged (lastSentHeads, not theirHeads).
SentIsAckedState(p) ==
  CASE ~up[p]                                    -> "offline"
    [] ~heard[<<Me, p>>]                         -> "checking"
    [] MissingIn(p)                              -> "behind"
    [] ~(doc[Me] \subseteq lastSent[<<Me, p>>])  -> "sending"
    [] OTHER                                     -> "synced"

\* M-QueueOnly: my side read from the bridge queue (Links pendingMsgs = 0).
QueueOnlyState(p) ==
  CASE ~up[p]                                    -> "offline"
    [] ~heard[<<Me, p>>]                         -> "checking"
    [] MissingIn(p)                              -> "behind"
    [] snap[p] > 0                               -> "sending"
    [] OTHER                                     -> "synced"

\* M-EveryVacuous: peers.every(isSynced), true when nobody is connected.
EveryGlobal ==
  IF \A p \in Connected : PeerState(p) = "synced" THEN "synced"
  ELSE SoundGlobal(PeerState)

\* M-AnyPeer: peers.some(isSynced), one synced peer is enough.
AnyGlobal ==
  IF \E p \in Connected : PeerState(p) = "synced" THEN "synced"
  ELSE SoundGlobal(PeerState)

\* M-NoRuleL: a transport loss raises no PeerDown/PeerUp (sync/1 before RL1).
NoResync(p) == FALSE
==============================================================================
