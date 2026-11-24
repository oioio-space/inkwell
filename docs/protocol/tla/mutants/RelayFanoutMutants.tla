-------------------------- MODULE RelayFanoutMutants --------------------------
(* Block RF1, README "RelayFanout": definition overrides of RelayFanout's rule operators (each config overrides  *)
(* one, the sound module is never edited), and the witnesses (invariants the sound model must violate: the states *)
(* the properties speak of are reached). An override never calls the operator it replaces.                      *)
EXTENDS RelayFanout

Off == FALSE

\* MF1 (RF-1 and RF-2 off: libp2p's connection limiter as the product runs it before RF1). Every connection is
\* counted under its origin for its whole life, outbound ones included, a circuit under its relay's address: members
\* already connected through the relay (or behind the home router) take the origin's places.
OriginLoadLibp2p(o) == PendOn(o) + EstOn(o)
OutOKLibp2p(m) == Room(m) /\ Total < SysCap /\ OriginLoadLibp2p(Via[m]) < P
\* MF2 (RF-2's outbound rule off): an outbound opening is charged to the pending quota until its handshake.
OutOKCharged(m) == Room(m) /\ Total < SysCap /\ GlobalLoad < Q
\* MF7 (no refund): a member's handshake leaves its token missing.
RefundNone(m) ==
  IF held[m] THEN held' = [held EXCEPT ![m] = FALSE] /\ od' = [od EXCEPT ![Via[m]] = @ + 1] /\ UNCHANGED sd
  ELSE UNCHANGED <<held, sd, od>>
\* MF8 (RF-6 off at the opening): one capacity for members and pending connections (libp2p's single system limit,
\* the product's 128 before RF1).
SharedSysCap == MemberCap
\* MF9 (RF-6 off at the handshake): a member's handshake checked against every connection, pending ones included.
AuthOKShared(m) == est[m] = 2 \/ Total <= MemberCap
\* MF10 (shares off): a third or fourth connection checked against the total only, not the others' unused shares.
RoomNoShare(m) == est[m] < PerCap /\ EstAll < MemberCap
\* MFR1 (review round, roster growth): no shed: a member joins the roster while the surplus is taken, and nothing
\* closes, so its share is not there.
ShedNone(A) == est' = est
\* MF11 (the member capacity off).
RoomNoCap(m) == est[m] < PerCap
\* MF12 (the bucket's refund made blind, `tokens = min(B, tokens + 1)` at every member handshake): a member whose
\* token a period already paid gives back a token that was not its own.
RefundUnheld(m) ==
  IF held[m] THEN held' = [held EXCEPT ![m] = FALSE] /\ UNCHANGED <<sd, od>>
  ELSE /\ sd' = [sd EXCEPT ![Via[m]] = IF @ > 0 THEN @ - 1 ELSE @]
       /\ UNCHANGED <<held, od>>
\* MF13 (round 1's fixed window instead of the bucket): a period pays every debt of the origin at once.
RefillWindow(o) ==
  /\ sd' = [sd EXCEPT ![o] = 0]
  /\ od' = [od EXCEPT ![o] = 0]
  /\ held' = [m \in Members |-> IF Via[m] = o THEN FALSE ELSE held[m]]

\* Witnesses.
NoInRefusal == ev.k # "in"
NoRateRefusal == ~(ev.k = "in" /\ "rate" \in ev.why)
NoFullQuota == Pending < Q
NoPassage == \A m \in Honest : est[m] < 2
NoSurplus == \A b \in Byz : est[b] <= 2
NoByzRefusal == ev.k # "byz"
NoPaidPending == \A m \in Honest : pendM[m] => held[m]
=============================================================================
