--------------------------- MODULE SyncOrderMutants ---------------------------
(* Block PU1b, README "SyncOrder": definition overrides of SyncOrder's rule operators. *)
EXTENDS SyncOrder

\* Q1 (P-9 off: no wake): a removed stream's successor keeps waiting for it.
RewireStale(s) == s
\* Q2 (P-9 off: the place is kept): a stream that delivered nothing leaves only once its turn has come.
EarlyAtTurn(s) == waitFor[s] = 0
\* Q3 (P-8 off): a capacity refusal closes the peer (ClosePeer, as §8.7.4 says of a refusal).
OnBusyKillMut == TRUE
\* Q4 (P-8 off): three places per peer.
PlaceFree3 == Len(list) < 3
\* Q5 (P-9 off for an end of writing): only a reset counts as an end; a stream its opener closed without a frame
\* keeps its place.
EndedByReset(s) == rstd[s]
=============================================================================
