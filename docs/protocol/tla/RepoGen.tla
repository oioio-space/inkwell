---------------------------- MODULE RepoGen ----------------------------
(* Generations of the repository class (spec.md §7.3), two peers A and B.  *)
(* Each peer owns one outbound REPO stream per generation; the generation  *)
(* travels in the stream preamble. A reset = reset the old stream and open *)
(* a new one with gen+1. Streams are unordered with respect to each other; *)
(* frames inside a stream are ordered. Items read from a stream wait in a  *)
(* bounded inbound queue before an asynchronous delivery to the webview,   *)
(* which checks the generation tag (§7.3 rule T).                          *)
EXTENDS Naturals, Sequences

CONSTANTS MaxGen,      \* last generation explored (u32 bound, §7.3 rule W)
          MaxMsg,      \* messages sent per (peer, generation)
          QMax,        \* inbound queue capacity
          MaxLoss,     \* local losses (queue drop, webview request) per run
          MaxRestrict  \* suspensions (Restreint) per peer

Peers == {"A", "B"}
Other(p) == IF p = "A" THEN "B" ELSE "A"
Gens == 1..MaxGen
NoStream == [st |-> "none", data |-> <<>>]

VARIABLES gen,     \* gen[p] : p's current generation (0 before activation)
          mode,    \* "down" (not Actif) | "resetting" | "synced"
          inG,     \* inG[p] : generation of the inbound stream p reads (0 = none)
          str,     \* str[p][g] : p's outbound stream of generation g
          q,       \* q[p] : inbound queue of [g, n] items awaiting delivery
          sent,    \* sent[p][g] : messages p sent in generation g
          dlv,     \* dlv[p][g] : last message number delivered to p in g
          conf,    \* conf[p] : gen[p] was confirmed by the peer (synced reached in it)
          loss, rstr, viol

vars == <<gen, mode, inG, str, q, sent, dlv, conf, loss, rstr, viol>>

Init ==
  /\ gen = [p \in Peers |-> 0]
  /\ mode = [p \in Peers |-> "down"]
  /\ inG = [p \in Peers |-> 0]
  /\ str = [p \in Peers |-> [g \in Gens |-> NoStream]]
  /\ q = [p \in Peers |-> <<>>]
  /\ sent = [p \in Peers |-> [g \in Gens |-> 0]]
  /\ dlv = [p \in Peers |-> [g \in Gens |-> 0]]
  /\ conf = [p \in Peers |-> FALSE]
  /\ loss = 0
  /\ rstr = [p \in Peers |-> 0]
  /\ viol = "none"

\* Reset p's outbound stream of generation g (RESET_STREAM: data discarded).
ResetOut(s, p, g) ==
  IF g \in Gens /\ s[p][g].st \in {"pending", "open"}
  THEN [s EXCEPT ![p][g] = [st |-> "reset", data |-> <<>>]] ELSE s

\* Rule I (initiate): gen+1, reset the old outbound stream, open the new one,
\* stop reading the old inbound stream. Atomic (one actor step).
\* Rule B (bump): only a CONFIRMED generation is left by gen+1; an unconfirmed
\* one (or none, gen = 0 -> 1 at session start) is reopened as it is.
Next1(p) == IF conf[p] THEN gen[p] + 1 ELSE IF gen[p] = 0 THEN 1 ELSE gen[p]

InitiateStr(p, n) ==
  [ResetOut(str, p, gen[p]) EXCEPT ![p][n] = [st |-> "pending", data |-> <<>>]]

InitiateTo(p, n) ==
  /\ gen' = [gen EXCEPT ![p] = n]
  /\ mode' = [mode EXCEPT ![p] = "resetting"]
  /\ inG' = [inG EXCEPT ![p] = 0]
  /\ conf' = [conf EXCEPT ![p] = FALSE]
  /\ str' = InitiateStr(p, n)

Initiate(p) == InitiateTo(p, gen[p] + 1)   \* from synced: conf[p] holds

\* Activation (session start or resume after Restreint) is an initiation.
Activate(p) ==
  /\ mode[p] = "down" /\ Next1(p) <= MaxGen
  /\ InitiateTo(p, Next1(p))
  /\ UNCHANGED <<q, sent, dlv, loss, rstr, viol>>

\* Local loss while synced: drop the head of the inbound queue AND initiate,
\* in the same step (rule L: no silent loss in the reliable class).
LocalLoss(p) ==
  /\ mode[p] = "synced" /\ q[p] # <<>> /\ loss < MaxLoss /\ gen[p] < MaxGen
  /\ q' = [q EXCEPT ![p] = Tail(@)]
  /\ loss' = loss + 1
  /\ Initiate(p)
  /\ UNCHANGED <<sent, dlv, rstr, viol>>

\* Webview request (or any other local reason) while synced.
Request(p) ==
  /\ mode[p] = "synced" /\ loss < MaxLoss /\ gen[p] < MaxGen
  /\ loss' = loss + 1
  /\ Initiate(p)
  /\ UNCHANGED <<q, sent, dlv, rstr, viol>>

\* Our current outbound stream was reset by the peer while synced: that is a
\* loss (rule L), so initiate.
StreamLost(p) ==
  /\ mode[p] = "synced" /\ str[p][gen[p]].st = "reset" /\ gen[p] < MaxGen
  /\ Initiate(p)
  /\ UNCHANGED <<q, sent, dlv, loss, rstr, viol>>

\* The inbound stream we read was reset by the peer while we are synced and no
\* newer stream replaced it: also a loss (rule L, receiver side).
InboundLost(p) ==
  /\ mode[p] = "synced" /\ inG[p] \in Gens /\ gen[p] < MaxGen
  /\ str[Other(p)][inG[p]].st = "reset"
  /\ Initiate(p)
  /\ UNCHANGED <<q, sent, dlv, loss, rstr, viol>>

\* Resetting and our new stream was refused (peer down or stale): reopen the
\* SAME generation after the rate-limit delay (rule R: retries never bump gen).
Retry(p) ==
  /\ mode[p] = "resetting" /\ str[p][gen[p]].st = "reset"
  /\ str' = [str EXCEPT ![p][gen[p]] = [st |-> "pending", data |-> <<>>]]
  /\ UNCHANGED <<gen, mode, inG, q, sent, dlv, conf, loss, rstr, viol>>

\* p receives the preamble of q's stream of generation g (rule A).
Accept(p, g) ==
  LET s == Other(p) IN
  /\ str[s][g].st = "pending"
  /\ CASE mode[p] = "down" \/ g < gen[p] ->            \* refuse
            /\ str' = [str EXCEPT ![s][g] = [st |-> "reset", data |-> <<>>]]
            /\ UNCHANGED <<gen, mode, inG, conf, viol>>
       [] g = gen[p] /\ mode[p] = "resetting" ->       \* confirmation
            /\ str' = [str EXCEPT ![s][g].st = "open"]
            /\ inG' = [inG EXCEPT ![p] = g]
            /\ mode' = [mode EXCEPT ![p] = "synced"]
            /\ conf' = [conf EXCEPT ![p] = TRUE]
            /\ UNCHANGED <<gen, viol>>
       [] g = gen[p] /\ mode[p] = "synced" ->          \* reopened unconfirmed gen:
            /\ str' = [str EXCEPT ![s][g].st = "open"]     \* replaces the old inbound,
            /\ inG' = [inG EXCEPT ![p] = g]                \* which carried no data
            /\ UNCHANGED <<gen, mode, conf, viol>>
       [] g = gen[p] + 1 ->                            \* follow the peer
            /\ str' = [InitiateStr(p, g) EXCEPT ![s][g].st = "open"]
            /\ inG' = [inG EXCEPT ![p] = g]
            /\ gen' = [gen EXCEPT ![p] = g]
            /\ mode' = [mode EXCEPT ![p] = "synced"]
            /\ conf' = [conf EXCEPT ![p] = TRUE]
            /\ UNCHANGED viol
       [] OTHER ->                                     \* g > gen+1
            /\ viol' = "accept"
            /\ UNCHANGED <<gen, mode, inG, str, conf>>
  /\ UNCHANGED <<q, sent, dlv, loss, rstr>>

Send(p) ==
  /\ mode[p] = "synced" /\ sent[p][gen[p]] < MaxMsg
  /\ str[p][gen[p]].st \in {"pending", "open"}
  /\ sent' = [sent EXCEPT ![p][gen[p]] = @ + 1]
  /\ str' = [str EXCEPT ![p][gen[p]].data = Append(@, sent[p][gen[p]] + 1)]
  /\ UNCHANGED <<gen, mode, inG, q, dlv, conf, loss, rstr, viol>>

\* Read one frame of the current inbound stream, only with queue space
\* (receiver-paced: no space, no read — the transport window holds the sender).
Read(p) ==
  LET s == Other(p) IN LET g == inG[p] IN
  /\ g # 0 /\ str[s][g].st = "open" /\ str[s][g].data # <<>>
  /\ Len(q[p]) < QMax
  /\ q' = [q EXCEPT ![p] = Append(@, [g |-> g, n |-> Head(str[s][g].data)])]
  /\ str' = [str EXCEPT ![s][g].data = Tail(@)]
  /\ UNCHANGED <<gen, mode, inG, sent, dlv, conf, loss, rstr, viol>>

\* Asynchronous delivery to the webview, with the generation check (rule T).
Deliver(p) ==
  /\ q[p] # <<>>
  /\ LET m == Head(q[p]) IN
       IF mode[p] = "synced" /\ m.g = gen[p]
       THEN /\ dlv' = [dlv EXCEPT ![p][m.g] = m.n]
            /\ viol' = IF m.n = dlv[p][m.g] + 1 THEN viol ELSE "gap"
       ELSE UNCHANGED <<dlv, viol>>                    \* older generation: dropped
  /\ q' = [q EXCEPT ![p] = Tail(@)]
  /\ UNCHANGED <<gen, mode, inG, str, sent, conf, loss, rstr>>

\* Suspension (Restreint): leave Actif; reset own outbound and the inbound stream.
Restrict(p) ==
  LET s == Other(p) IN
  /\ mode[p] # "down" /\ rstr[p] < MaxRestrict
  /\ mode' = [mode EXCEPT ![p] = "down"]
  /\ inG' = [inG EXCEPT ![p] = 0]
  /\ str' = [ResetOut(str, p, gen[p]) EXCEPT ![s] =
              IF inG[p] \in Gens /\ str[s][inG[p]].st = "open"
              THEN [@ EXCEPT ![inG[p]] = [st |-> "reset", data |-> <<>>]] ELSE @]
  /\ rstr' = [rstr EXCEPT ![p] = @ + 1]
  /\ UNCHANGED <<gen, q, sent, dlv, conf, loss, viol>>

Next ==
  \E p \in Peers :
    \/ Activate(p) \/ LocalLoss(p) \/ Request(p) \/ StreamLost(p) \/ InboundLost(p) \/ Retry(p)
    \/ Send(p) \/ Read(p) \/ Deliver(p) \/ Restrict(p)
    \/ \E g \in Gens : Accept(p, g)

Fair == \A p \in Peers :
          /\ WF_vars(Activate(p)) /\ WF_vars(StreamLost(p)) /\ WF_vars(InboundLost(p))
          /\ WF_vars(Retry(p))
          /\ WF_vars(Read(p)) /\ WF_vars(Deliver(p))
          /\ \A g \in Gens : WF_vars(Accept(p, g))

Spec == Init /\ [][Next]_vars /\ Fair

------------------------------------------------------------------------------
\* Safety
NoViolation == viol = "none"          \* honest peers never trip rule A, never see a gap
GenGap == (gen["A"] <= gen["B"] + 1) /\ (gen["B"] <= gen["A"] + 1)
SyncedAgree == (mode["A"] = "synced" /\ mode["B"] = "synced") => gen["A"] = gen["B"]
QueueBound == \A p \in Peers : Len(q[p]) <= QMax
\* Within a generation, what p delivered is a prefix of what the other sent.
Prefix == \A p \in Peers : \A g \in Gens : dlv[p][g] <= sent[Other(p)][g]

AtBound == \E p \in Peers : gen[p] = MaxGen
Agreed == mode["A"] = "synced" /\ mode["B"] = "synced" /\ gen["A"] = gen["B"]
Drained == \A p \in Peers :
             /\ q[p] = <<>>
             /\ dlv[p][gen[p]] = sent[Other(p)][gen[p]]
\* No silent loss: a quiescent agreed state delivered everything sent in it.
Quiet == ~ENABLED (\E p \in Peers : Read(p) \/ Deliver(p) \/ \E g \in Gens : Accept(p, g))
NoSilentLoss == (Agreed /\ Quiet /\ \A p \in Peers : str[p][gen[p]].data = <<>>) => Drained

\* Liveness: with finitely many losses and suspensions, both sides end up
\* synced in the same generation, unless the explored u32 bound is reached.
Converges == <>[](Agreed \/ AtBound \/ \E p \in Peers : mode[p] = "down")
==============================================================================
