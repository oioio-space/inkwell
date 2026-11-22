------------------------ MODULE InboundLifecycleMutants ------------------------
(* Block PU1c, README "InboundLifecycle": definition overrides of InboundLifecycle's rule operators, one per past     *)
(* finding (QC1-QC6), plus the bulk lane's old chain (QC7) and a connection window too small (QC8).                *)
EXTENDS InboundLifecycle

\* QC1 (finding 1, PU1 round 1): the sync place is taken after the up-wait, so a late handler loses its rank.
PlaceSyncLate(l) == l # "sync"
\* QC2 (finding 2, PU1b code round 1): a stream that ended without a frame waits for its predecessor before leaving.
EarlyAtTurn(s) == waitFor[s] = 0
\* QC3 (finding 3, PU1b code round 2): the probe goes blind once it took a unit (the one-shot Peek watcher).
ProbeOnce(s) == pre[s] = <<>>
\* QC4 (finding 5, PU1b code round 3): a waiting stream reads its frames and charges them to the shared budget.
WaitChargesMut == TRUE
\* QC5 (lifecycle review): only a prefix without a truncated last frame is kept; else the stream leaves at once and
\* the whole frames before the truncated one are thrown away.
KeepWholeOnly(s) == pre[s][Len(pre[s])].w
\* QC6 (lifecycle review): the budget wait is not cancelled by the loss of the stream's connection.
CancelOnLossMut == FALSE
\* QC7 (bulkTail, bulk.go before PU1c): the bulk place is taken after the up-wait and a failed bulk stream waits for
\* its predecessor.
PlaceBulkLate(l) == l # "bulk"
EarlyBulkAtTurn(s) == Lane(s) # "bulk" \/ waitFor[s] = 0
\* QC8 (P-13 off): the connection window leaves the head nothing once the other streams hold their windows.
ConnWindowSmall == (Cap * 2 - 1) * Win
=============================================================================
