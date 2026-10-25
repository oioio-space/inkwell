# Liste de membres — algorithme de repli v2 (après audit)

Conception de l'agent architect (Opus) en réponse à l'audit de sécurité du 25/10/2026 ; source de vérité pour l'implémentation de `internal/trust`.

## Approach

Prior art:
- **Matrix room v12:** creators get infinite power because "access control requires a hierarchy".
- **Kleppmann's seniority:** the more senior admin's ops apply first.
- **p2panda-auth "strong removal":** a removed member's concurrent ops stay invalid even after a re-add.
- **Keyhive:** decides which revocations can act without letting revocations judge each other, which keeps the evaluation acyclic. It accepts that backdating can't be detected (its threat item T5). It lets mutual revocations both stand, which we reject.
- **Radicle:** avoids concurrency with delegate quorums, which is not workable offline.

The improvement here: ranks are static, and a rank can only come from an op of a key that is already ranked. So the fold is a single Dijkstra-style sweep over keys in rank order, with no fixpoint. Every decision about key y reads only decisions about keys senior to y and y's own earlier ops.

## (a) Replacement for the rules part of `internal/trust/doc.go`

This replaces everything from "Ops are stored whatever their effect" through the end of "# Fold rules":

```
// # Admission
//
// [List.Apply] stores an op only if it is canonical and signed for the project,
// its parents are stored (otherwise it waits in a bounded buffer that refuses
// rather than evicts), its Lamport is 1 + the maximum of its parents', and its
// author was granted admin in the op's causal past: the creator, or the subject
// of an Add or SetRole with Role admin among the op's ancestors, effective or
// not ([ErrUnknownAuthor] otherwise). Every op kind is admin-only, so readers
// and writers cannot store anything. Each check reads only the op and its
// ancestors, so every replica reaches the same verdict.
//
// A list holding 1024 ops is sealed: it stores nothing more, and Issue, Apply
// and Verify fail with [ErrTooManyOps]. Role and Members report nobody, so
// the list fails closed. Sealing depends only on the set the replicas exchange:
// if it reaches 1024 ops every replica seals, otherwise every replica ends up
// holding the whole set.
//
// # Fold rules (the specification)
//
// The membership is a function of the stored set. ≺ orders ops by Lamport,
// author key, then hash, and extends causality. pos(o) is o's index in ≺.
// "o sees p" means p is a strict ancestor of o. Two ops are concurrent when
// neither sees the other. A role op on x is an Add, SetRole or Remove whose
// subject is x, or Genesis for the creator. It is a grant if it sets Role
// admin, and a cut if it is a Remove or sets a lower role. Levels go
// Remove < reader < writer < admin.
//
//  1. Seniority. The creator has rank 0. Any other key's rank is pos(g) of the
//     ≺-earliest surviving (rule 4) standing grant g on it. A key without one
//     is unranked. A smaller rank is senior. Ranks are distinct and are kept
//     across demotion.
//  2. Reach. A role op by y on another key x counts only if x is unranked or
//     junior to y. An op on a senior is ignored: no one can cut, demote,
//     re-promote or rename a senior. A key may act on itself, except that the
//     creator's cuts of itself are ignored, so the creator is admin forever.
//  3. Standing. An op o by y stands if and only if:
//     (a) y is admin at o: among the role ops on y that o sees and that survive,
//         the ≺-latest is a grant (Genesis for the creator);
//     (b) no standing cut on y by another key is concurrent to o; and
//     (c) rule 2 allows it.
//     An op that does not stand has no effect at all. By (b), what a revoked
//     admin signs without having seen its cut is dead, even if it is later
//     re-admitted. By (a), what it signs after seeing the cut is dead unless a
//     senior re-granted it in between.
//  4. Survival. A standing role op on x by another key is dropped if a
//     concurrent standing role op on x by another key sets a lower level, so a
//     revocation beats a concurrent grant. All other role ops survive. A key's
//     ops on itself always survive and drop nothing.
//  5. Result. x's role is the one set by the ≺-latest surviving role op on x.
//     If that op is a Remove, or there is none, x is not a member. Add and
//     SetRole both set the role, and the Name when it is non-empty. Person comes
//     from the ≺-latest surviving Add (that Add's hash if Person is zero).
//     Members are listed in ≺ order of that Add, and Genesis comes first.
//
// Each rule about y's ops reads only ops by keys senior to y and y's own
// earlier ops. A grant's position is greater than its author's rank, because
// the author's own grant is one of its ancestors. So one sweep over keys in
// rank order decides everything. Cuts only go to juniors, so two admins cannot
// cut each other and no cycle of removals exists. The creator keeps at least
// one admin in place. The fold costs O(n²/64 + n·e), where e is the largest
// number of role ops on one key.
```

## (b) Pseudo-code

**Fold** (it replaces `pass`, `authorized` and `addVoids`):

```
fold(S):
  nodes := sort S by ≺;  anc[i] := OR over parents p of (anc[p] ∪ {p})     // bitsets, n ≤ 1024
  conc(i,j) := i≠j ∧ ¬anc[i][j] ∧ ¬anc[j][i]
  on[x] := role ops on x in ≺ order;  by[y] := ops by y in ≺ order;  C := creator
  stands[] := false;  rank := {};  heap := {(0, C)}
  while heap not empty:
    (r, y) := popMin(heap); if y ∈ rank: continue
    E    := [i ∈ on[y] : author(i) ≠ y ∧ stands[i]]           // authors are all senior to y
    surv := survivors(E)
    if y ≠ C:
      g := min{pos(i) : i ∈ surv, grant(i)}; if none: continue // never ranked
      if g > r: push(heap, (g, y)); continue                   // its earliest grant was dropped: re-queue
    rank[y] := r;  cuts := [i ∈ E : cut(i)];  self := []
    for i in by[y]:                                            // ≺ order, so y's earlier ops are decided
      if ∃k ∈ cuts: conc(i,k): continue                        // 3b: backdated or concurrent, so killed
      last := ≺-max{j ∈ surv ∪ self : anc[i][j]}
      if y ≠ C ∧ (last = none ∨ ¬grant(last)): continue        // 3a
      if role(i):
        x := subject(i)
        if x = y: { if y = C ∧ cut(i): continue; self += i }
        else if x ∈ rank: continue                             // rule 2: x was popped first, so x is senior
      stands[i] := true
      if role(i) ∧ subject(i) ≠ y ∧ grant(i): push(heap, (pos(i), subject(i)))
  for each subject x:                                          // rule 5
    E := [i ∈ on[x] : stands[i] ∧ (author(i) = x ∨ x ∉ rank ∨ rank[author(i)] < rank[x])]
    last := ≺-max(survivors(E without x's self-ops) ∪ x's self-ops)
    if last ≠ none ∧ kind(last) ≠ Remove: add member (role/name/person per rule 5)
survivors(E) := {i ∈ E : ¬∃j ∈ E, conc(i,j) ∧ level(j) < level(i)}
```

Why this is correct:
- **Termination and cost.** Each key is popped at most 1 + (number of grants on it) times, and re-queuing only increases a key's priority. The heap costs O(n log n), the ancestor bitsets O(n²/64), and the survival and kill scans O(Σ|E_y|·ops(y)) ≤ n·e. At the cap that is about 1M bit-tests, a few milliseconds.
- **Dijkstra invariant.** When y is popped, every key with a smaller final rank has already been popped. The positions are distinct and every grant's position is greater than its author's rank.
- **Stickiness.** Let k be a standing cut on x. Take an op o by x that does not see k:
  - If o is concurrent to k, it is killed by rule 3b.
  - Otherwise k sees o, which means the remover saw it, so it stays.

  Now take an op o that does see k. It stands only if the ≺-latest surviving role op g on x in its past is a grant with g after k. If g were concurrent to k, rule 4 would drop it; it cannot be an ancestor of k. So g sees k: it is a re-admission, and rule 2 means it came from a senior.

**`List.Apply` admission:**

```
apply(op):
  validate → ErrMalformed;  h := hash;  if stored(h) ∨ pending(h): return nil
  if Genesis ∧ h ≠ id → ErrWrongProject;  verify sig → ErrBadSignature
  if len(ops) ≥ maxOps → ErrTooManyOps                         // sealed
  if ¬allParentsStored: if len(pending) ≥ maxPending → ErrTooManyOps   // refuse, never evict
                        pending += (op, h, gen); return nil
  integrate(op, h)?;  drain pending whose parents are all stored (stop once sealed, drop failures)
integrate(op, h):
  if ¬Genesis: Lamport = 1+max(parents) (guard MaxUint64) else ErrBadLamport
               everAdmin(op.Author, op.Prev) else ErrUnknownAuthor  // ancestor search with early exit: creator,
                                                                    // or subject of an Add/SetRole with Role=Admin
  store; update heads; view = nil
```

`Sync.Tick` calls `l.prunePending()`, which drops entries more than one tick old (two generations).

**`IssueWith`:**
1. If sealed, return `ErrTooManyOps`.
2. If the signer is not admin, return `ErrNotAdmin`.
3. A cut aimed at the creator returns `ErrLastAdmin`.
4. An op on a senior key returns the new `ErrOutranked`.
5. Prev is all heads if there are ≤ MaxPrev of them. Otherwise it is the head covering the issuer's ≺-latest surviving grant, plus the MaxPrev−1 ≺-greatest other heads, sorted by hash. Covering that grant also covers every cut on the issuer, so the op is guaranteed to stand.

`ErrTooManyHeads` is never returned again.

**Sync:**
- A Have gets back Ops (`Since`, which only counts hashes we know). It never gets a Have back.
- Each peer gets at most 2 Have answers per Tick; the budget resets on Tick and `OnPeerUp`.
- A sealed list neither announces on Tick nor on `OnPeerUp`, but still answers.
- Ops received from one peer are relayed to the others, as now.
- Convergence relies on each side's own Tick to pull what it lacks.

**Public API changes:**
- Added: `List.Sealed() bool`, `ErrOutranked`.
- Removed: `ErrTooManyHeads`. No callers outside `internal/trust` (grep).
- Same signatures, changed meaning:
  - `ErrUnknownAuthor` now means "never granted admin in the causal past".
  - `ErrLastAdmin` now means "the creator cannot be removed or demoted".
  - Apply refuses ops when the pending buffer is full instead of evicting old ones.
  - `Role`, `Members` and `Verify` fail closed when the list is sealed.
  - Add of an existing member now sets its role and name (it used to do nothing).
  - SetRole on a non-member now admits it. Only an admin can do this, so it grants nothing beyond what an Add would.

Everything else is unchanged.

## Why each PoC is defeated

| PoC | Outcome under the design |
|---|---|
| TwoDevicesEscapeRevocation | m1 and m2 target a, who is senior, so rule 2 ignores the ops; they are also killed by rule 3b. `Issue(m1)` returns `ErrNotAdmin`, so the PoC hits `t.Fatal` and the exploit fails. |
| Resurrect2 / ExAdminResurrects | Same reasons. a stays admin; z and m stay removed. |
| ExAdminStickyVoid | m's `Remove(w)` is concurrent to `Remove(m)`, so rule 3b kills it. There are no "voids" left to stick; w stays a writer. |
| Ring | c→a is a cut on a senior, ignored by rule 2. b→c is killed by a→b (rule 3b). Result: a and c admin, b removed. |
| DemotedAdminRepromotes | The whole chain is concurrent to the demotion and is killed. x never gets a rank. |
| ReaderFreezesHeads / FillsCap | The reader's ops are refused at Apply with `ErrUnknownAuthor`. Issue caps Prev anyway. |
| SyncStorm | There is no Have-in-reply-to-Have, and the per-peer budget bounds traffic. **This PoC must be rewritten:** `la.Len()` can no longer reach `maxOps`, so its loop never ends. Flood with an ex-admin instead. |
| Cascade | One sweep; milliseconds. All S_i are removed and all C_i are admins. |
| Pending eviction | The buffer refuses when full and expires entries on Tick. |

## (c) Regression tests (`internal/trust`)

**Fold:**
1. `TestExAdminBackdatedOpsKilled`: PoC ExAdminStickyVoid, both rounds; w stays a writer.
2. `TestExAdminCannotResurrect`: PoC 4 plus Resurrect2; exactly one member is left.
3. `TestRevokedDevicesCannotRescueEachOther`: PoC 6; ends with `ErrNotAdmin`.
4. `TestRemovalRing`: a and c admin, b removed.
5. `TestDemotedAdminCannotRepromote`: m is a reader; x is not a member.
6. `TestJuniorCannotCutSenior`: a forged cut has no effect; Issue returns `ErrOutranked`.
7. `TestCreatorPermanent`: Issue returns `ErrLastAdmin`; a forged self-cut has no effect.
8. `TestRepromotionOnlyBySenior`: a grant by a junior does nothing; one by a senior works.
9. `TestConcurrentOpsStayKilledAfterReadd`: the p2panda semantics.
10. `TestCascadeSingleSweep`: PoC Cascade at k=250; checks the roles and that the fold takes under 100 ms.
11. Keep the existing named scenarios; their expectations are unchanged: `ConcurrentRevocation`, `TwoAdminsRemoveEachOther`, `Antedating`, `ReaddAfterRemove`, `ReaddConcurrentWithRemoveIsVoid`, `LastAdminCannotLeave`, `MembersOrderAndNames`.

**Admission and cap:**
12. `TestNonAdminOpsRefused`: reader and writer ops return `ErrUnknownAuthor`; Len and heads are unchanged.
13. `TestIssueCapsPrev`: with 100 admin-made heads, Issue succeeds with `len(Prev)==MaxPrev` and the new op stands.
14. `TestSealIsOrderIndependent`: ex-admin flood plus a legitimate Remove, delivered in two orders; after exchange both replicas are `Sealed()`, report Role 0, and Issue/Verify return `ErrTooManyOps`.
15. `TestTooManyOps`: update it to also assert `Sealed()`.
16. `TestPendingFullRefusesAndExpires`.

**Sync:**
17. `TestSyncNoAnnounceStorm`: diverged replicas, one `OnPeerUp` each; the queue drains in ≤ 6 messages and the replicas converge.
18. `TestSyncHaveNeverAnsweredWithHave`.
19. `TestSyncHaveBudgetPerTick`: 10 empty Haves get at most 2 answers.
20. `TestSyncSealedDoesNotAnnounce`.
21. `TestSyncConverges` (existing).

**Property tests:**
22. `TestFoldOrderIndependent`: change genDAG so authors include granted admins and revoked ones (to exercise backdating); also compare `Sealed`.
23. `TestFoldMatchesReference`: rapid comparison against a naive O(n³) implementation written literally from the rules (no bitsets, no heap).
24. `TestFoldInvariants` (rapid): the creator is admin; every standing cut goes from a senior to a junior; no standing op is concurrent to a standing senior cut on its author; every standing op's author is admin at that op.

**Benchmark:**
25. `BenchmarkFoldAtCap`: chain, cascade and wide-fan shapes.

## Decisions for you

- **An ex-admin can seal the project.** A compromised ex-admin key can still store ops by backdating them; Keyhive says such ops can't be told apart from genuinely late ones. With 1024 of them it seals the list. That is a denial of service that fails closed; recovery means creating a new project. A v2 could add equivocation "burning".
- **The creator can never be revoked.** A lost or stolen creator device means a new project, as in Matrix v12. This and strict seniority replace ADR 0017's "older admin wins"; that ADR needs an amendment.
- **Seniority survives demotion.** Only keys senior to a former admin can manage it; the creator always can.

Sources:
- [Keyhive convergent capabilities (design doc)](https://github.com/inkandswitch/keyhive/blob/main/design/convergent_capabilities.md)
- [Keyhive threat model, T5 back-dating](https://github.com/inkandswitch/keyhive/blob/main/design/threat_model.md)
- [Keyhive notebook 01](https://www.inkandswitch.com/keyhive/notebook/01/)
- [p2panda-auth docs](https://docs.rs/p2panda-auth)
- [Matrix Project Hydra: improving state res](https://www2.matrix.org/blog/2025/08/project-hydra-improving-state-res)
- [Matrix spec v1.16 changelog](https://spec.matrix.org/unstable/changelog/v1.16/)
- [Matrix v1.16 release](https://matrix.org/blog/2025/09/17/matrix-v1.16-release/)
- [ERA: Epoch-Resolved Arbitration for Duelling Admins (arXiv 2601.22963)](https://arxiv.org/html/2601.22963v1)
- [Kleppmann PaPoC 2025 keynote](https://martin.kleppmann.com/2025/03/31/papoc-keynote-byzantine.html) and [slides](https://speakerdeck.com/ept/byzantine-eventual-consistency-and-local-first-access-control)
- [Kleppmann, "Making CRDTs Byzantine Fault Tolerant" (PaPoC 2022)](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf) (cited from memory, not re-fetched)
- [Radicle protocol guide](https://radicle.xyz/guides/protocol)
- [Radicle delegates and thresholds (HackMD)](https://hackmd.io/@radicle/SyrkZVxC9)
- [Jacob et al., formal verification of local-first access control (arXiv 2604.23560)](https://arxiv.org/abs/2604.23560)

## Amendement (25/10/2026, demande utilisateur) — passation du créateur

Le créateur peut se retirer ou se rétrograder **lui-même** si au moins un autre admin existe
dans le passé causal de l'opération ; sinon l'opération est ignorée (jamais zéro admin).
Personne d'autre ne peut le couper. Son rang 0 subsiste (personne ne peut agir sur lui).
Tests : `TestCreatorHandsOverToAnotherAdmin`, `TestCreatorCannotLeaveAlone`,
`TestOthersCannotRemoveCreator`, et modèle de référence/invariants étendus.
