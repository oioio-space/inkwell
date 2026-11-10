------------------------- MODULE LinkProbeMutants -------------------------
(* MUTANTS (README.md, LinkProbe): broken rules of spec §8.5.5 and §8.5.6    *)
(* that the model must reject. Each mutant config overrides one rule of       *)
(* LinkProbe with one of these ("Name <- Mutant"); the sound module is never  *)
(* edited.                                                                     *)
EXTENDS LinkProbe

AcceptLate(age) == TRUE                 \* no late rule: a held PROBE's stale count is taken
NeverOnTime(age) == age < 0             \* a PTO too small for the path: every echo is late
NoSentBound(x) == TRUE                  \* a forged count above what was sent is taken
StrictSentBound(x) == x < sent          \* over-strict: an honest peer that got everything is a violation
NoMonotone(x) == TRUE                   \* a forged decreasing count is taken
NoClamp(dR, dS) == dR                   \* loss computed without the clamp: negative when reordered
NoPendingBound(p) == p                  \* outstanding PROBEs never forgotten
KeepAcrossRestart == samples            \* samples survive a new connection
OpenOnce == FALSE                       \* a stream that ended never reopens
StopAndWait == pending = <<>>           \* a PROBE waits for the ECHO of the previous one
AnyStream(e) == TRUE                    \* the prober takes an ECHO of an ended stream
AnyDropFault(n) == TRUE                 \* a dropped PROBE counts as a fault (a flood limit)
UnlimitedCredit == TRUE                 \* no credit: every PROBE is echoed
NoRefusal == r.count = 0                \* a new stream is accepted during the refusal
AnyCount == r.refuse = 0                \* a second stream is accepted while one is open
LongRefusal(viol) == B + 1              \* a refusal after an end longer than the prober's backoff
NoSeqBound(q) == TRUE                   \* no bound on the PROBE rate
NoDropBand(q) == q <= r.age + 2         \* every PROBE above the rate bound is a fault (review 3, F6)
DataOnly == g.rdata                     \* the responder counts data packets, not QUIC packets
=============================================================================
