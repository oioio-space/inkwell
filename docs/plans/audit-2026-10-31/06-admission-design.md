# Design v5: admission boundary and per-writer document chains (full Byzantine model)

Original author: architect (Opus); revision 5: Codex, 2026-10-31. History: v1 → Codex review 1
([07-codex-design-review-1.md](07-codex-design-review-1.md)) → v2 → review 2
([08-codex-design-review-2.md](08-codex-design-review-2.md)) → v3 → review 3
([10-codex-design-review-3.md](10-codex-design-review-3.md)) → v4 → review 4
([13-codex-design-review-4.md](13-codex-design-review-4.md)) → **v5** → **revision 6: "Périmètre final"
(top section; it prevails over v5 where they differ, and the v5 paragraphs it replaces are marked → F-…)**. The point-by-point
answer to review 4 is "Changes after review 4" below. N9 ratifies D-1..D-6 (§11);
the ADR itself remains proposed pending acceptance. Historical change tables are not normative; v5 prevails. User decisions N7 (full Byzantine threat model) and N8 (merge
Stage 1 now; S1 stays Critique until the ADR 0030 guard) are in `docs/exigences.md`. Decision
record: [ADR 0031](../../adr/0031-frontiere-admission-chaines-documents.md) (proposed).

Evidence (repo paths): [04-security-verify-A02-A05.md](04-security-verify-A02-A05.md) (A04),
[05-review-stage1-candidate.md](05-review-stage1-candidate.md) (P1-P5, R1, ZZ-2),
[09-admission-measurements.md](09-admission-measurements.md) (scripts and outputs: reconnect cost,
presence sizes, memory per document and per unavailable handle; v4 probes: a sync message is one
indivisible bundle, Automerge's hash-committed `author` field). Library line numbers refer to
`frontend/node_modules/@automerge/automerge-repo/dist` (2.5.6) and `@automerge/automerge` 3.5.0.

Rules: ASVS 5.0 V2.4.1; `docs/engineering-principles.md` §4; CLAUDE.md "une seule source de vérité
par chose"; ADR 0002/0011 (Go sees opaque bytes); ADR 0017/0021 (seniority, suspension); ADR 0018
(coverage); ADR 0028 (D1 growth caps, D2 Worker, D5 fragments, D9 unloading); ADR 0030 §2 (causal
standing), §3 (nothing hidden except by decision or trash), §4-5 (guard, fairness); N7. Severity per
`docs/plans/2026-10-31-architecture-audit.md` (Critique: invariant/ADR/ASVS L2 or demonstrated
loss or crash; Majeur: measured impact; Mineur: no measured impact).

Stage map: **Stage 1** merged (`ad2cf86`, `f2f1460`): bounds readers and unattested peers and keeps
R1/ZZ-2. **Stage 2** = ADR 0030 (structural guard **and** decision journal) plus the additions this
design requires of it (§7.1: actor-author binding G12, joint causal past, post-erasure absorbing certificate,
acknowledgements, fair scheduler, member-gated peer table). **Stage 3** = this file's Byzantine
admission (§2-§6). Every Stage 1 residual is mapped to its closing stage in §7.2; what stays open
after Stage 3 is graded in §9.

## Périmètre final : faisable maintenant / recherche ouverte (revision 6)

Architect (Opus), 2026-10-31: an independent review of v5 (Codex), checked against review 4 ([13](13-codex-design-review-4.md)),
the sourced survey ([14](14-research-byzantine-solutions.md), rows 1-6), ADR 0030/0031, the
audit rubric and the merged Stage 1 code. **This section prevails over §1-§11 where they differ**;
the paragraphs it replaces carry a "→ F-…" marker. "Achievable now" means a known mechanism,
buildable with the installed libraries under these assumptions: Ed25519/BLAKE3 hold, honest members
eventually exchange what they hold, and standing admins are trusted (ADR 0017, §1). It is not a claim
of implementation. User rule (2026-10-31): go as far as is achievable now, and grade the rest.

**Probe, 2026-10-31** (Node 22, automerge 3.5.0 from `frontend/node_modules`, script outside the repo):
(1) applying a second change with an `(actor, seq)` already held throws
`duplicate seq 1 found for actor …`, so the split is real and G8 must run before apply;
(2) `applyChanges(init(), held \ E)` over a filtered change set rebuilds the same state from the same
set (0.7 ms for the toy case), and the rival branch then applies cleanly, so arbitration by rebuild
is feasible; (3) `load(bytes, {actor})` then `change` gives that actor at `seq 1` with `author`
undefined: an actor-only factory works and the `author` field is not needed.

### F.0 Review of v5: kept, corrected, simplified

| ID | v5 text | Verdict | Final contract | Guarantee |
|---|---|---|---|---|
| F-C1 | §5.2: ADR 0028 D1 growth cap "not found in `coverage.ts` on `main`" | **wrong** | the cap exists (`frontend/src/net/coverage.ts:311-330`, commit `7972fdd`); keep it and add the history limits of F-C2 | fact |
| F-C2 | §5.2: ≤ 65,536 ops per job | **refuses honest edits** | `store/chunked.ts:44-64` batches small edits into one change and never splits `remove`: deleting a 2 Mi text is one change of 2 Mi `del` ops. **Local change factory splits every change at ≤ 65,536 ops** (inserts, deletes, batches, every document kind). Guard: one change ≤ 65,536 ops, one job = whole changes ≤ 16 MiB decompressed. Changes in `past(L_d)` (D-6 legacy) are exempt from the op cap and run under the 256 MiB Worker ceiling | ISO 25010 adequacy; same per-job memory bound |
| F-C3 | §2.4: admitted set has no term for pre-upgrade ids | **gap** | add `∪ {d : legacy/<d> standing}`: existing variants keep their random ids (they cannot match the derived formula); admins are trusted and the journal bounds them (8 192 ops) | removes the "migration covers every variant" hand-wave of §7.3 |
| F-S1 | §7.1 G12: `author` field, descriptor in `message`, `docNamespace`, two-clone factory, seq=1 check + causal binding | **simplify** | **ChangeSig** (M1): signed tuple, actor derived from (project, key, epoch), each change checked on its own | the Automerge hash commits actor and seq; only the key that derives the actor can sign; probe (3). Rival genesis, re-encoding and relay labelling fail as in v5, with one rule instead of three |
| F-S2 | §2.2 contribution receipts `inkwell/contribution/v1` | **drop** | adoption is triggered locally by this device's first ChangeSig in a foreign listed document (M2) | no peer verifies a receipt (v5: "No assertion about H follows"); the persisted `adopt` op is the idempotency record |
| F-S3 | §2.2 L3: `Slot` = prefix variant count + 1; prefix proofs fetched and retained for adoptions | **simplify** | L3: `Position` exact; `1 ≤ Slot ≤ Q(o)` and no earlier op on the chain with that Slot. Adoption checks the embedded e with L1, L4 and issuance-L5 only; no prefix fetch or retention | own ids ≤ distinct slots ≤ Q*; adoptions ≤ 1 per (adopter, author, slot): the bounds in §6 are unchanged |
| F-S4 | §4 phases 3-4: coordinator, prepare, locks, votes, abort, certificate | **simplify** | **FinalAck finality** (M8): per-device durable commitment after the window, required set fixed by the purge, pure monotone predicate | an honest admin cannot sign a restore after its FinalAck (v5's lock, without an abort path); no coordinator to recover |
| F-S5 | §7.1, §9: "no deterministic rule without consensus" for self-equivocation | **partly wrong** | true for a *leaderless* rule (Kleppmann 2022 §2.4). Standing admins are already trusted arbiters (ADR 0017), so an admin **arbitration** decision plus a deterministic rebuild (probe 2) is achievable now (M9, ERA's arbiter model, 14 rows 4-5) | turns a permanent split into convergence after one decision; the window stays a residual (F.2-R1) |
| F-A1 | §5.2: DRR charges the delivering peer only | **improve** | after decoding, guard and apply cost is charged to the ChangeSig author; transfer stays charged to the peer (14 row 3) | an honest relay no longer pays for W's work |
| F-A2 | §9: "paginated sender protocol" named but not specified | **specify** | **history pages** (M5) | only an activation blocker in v5 |
| — | §1, Stage 1, §2.1 option (g), §2.3 G11, §2.5, §3, §5.1, §5.3-§5.6, §6, §8 | **kept** | — | review 4 closures stand |

### F.1 Achievable now: mechanism, components, tests, stage

Each test row in the tables names threshold / one past / recovery, per §8.2.

**M1 — ChangeSig: actor bound to a key, every change signed by its author** (14 row 1: SSB feeds,
Nostr NIP-01; Stage 2).
- `actor = BLAKE3("inkwell/actor/v2" ‖ ProjectID ‖ key ‖ epoch)[:16]`. `epoch` is a random 16 B value
  issued and persisted by Go at handle creation (`NewEpoch`). There is one epoch per handle generation,
  and none is ever reused. A session that writes nothing leaves no actor in the document, and Go prunes
  unused epochs at startup.
- `ChangeSig = Ed25519(key, "inkwell/change/v1" ‖ ProjectID ‖ key ‖ epoch ‖ seq:u64 ‖ hash(c))`.
  On the wire it is 152 B. Go stores ≈ 104 B per change, with key and epoch kept once per actor
  under `repo/<doc>/sigs/`.
- Only the local factory's outbound path calls the bridge kind `Sign`. It sends one batch per outgoing
  sync message, in the slot used today by `#attest` (`WailsNetworkAdapter.ts:277`). No receive or relay
  path calls it.
- Go verifies signatures, batched per frame, before forwarding. It sends the verified tuples alongside
  the frames (kind `Authored`) and never decodes Automerge (ADR 0011). The webview guard decodes the
  header, recomputes the hash, and checks the actor derivation and the seq.
- Authorization stays T10: heads attested by current writers (ADR 0018). Authorship is ChangeSig.
  Both are required.
- Exceptions: the byte-exact deterministic genesis G (ADR 0030 §3) and `past(L_d)` (D-6).
- Two ChangeSigs with the same (key, epoch, seq) and different hashes are a ≈ 300 B equivocation proof.
  Go can check it without Automerge, like Hypercore's two signed roots or a CT split view
  (RFC 9162 §11.3).
- Components: `internal/trust` (`ChangeSig` beside `Attestation`, strict `Decode`, `FuzzDecode`,
  golden vectors shared with TS), `internal/app` (sign, verify, persist, gossip),
  `internal/bridge` (`Sign`, `Authored`, `NewEpoch`), `frontend/src/net/authorship.ts`, and the actor
  factory in the automerge-repo patch (`create`, `load`, `clone`, `import`, hydration).
- Tests: A15 rewritten as follows.
  - W's seq=1 under H's actor, in both orders → refused.
  - W re-encodes a change, or forges H's actor and author while H relays and attests it → refused.
  - Exact G is accepted; a modified G is refused.
  - Legacy past is accepted; an unsigned descendant is refused.
  - Run on 3 replicas in every order.
  - B18: unsigned-change wait queue of 4 per author (the 5th evicts that author's oldest; drains when
    the tuple arrives). Verification throughput is measured on a 100 000-change join and the budget is
    set from that measurement.

**M2 — Contribution = accepted signed change; adoption at first local co-edit** (14 row 2; Stage 3).
- Trigger: this device's first ChangeSig in a document listed by another author.
- Go persists `adopt(e)` before the change is published. After a crash, the change waits in the local
  journal and the adoption is retried idempotently.
- Relays never trigger it, by construction: they never produce a ChangeSig.
- Tests:
  - A3.
  - Relay-only of 1 000 documents → 0 adoptions.
  - Restart between adoption persistence and publication → one adoption, no lost edit.
  - B6.

**M3 — Per-writer listing chains, derived ids, G11, causal revocation, quotas on a frontier** (v5
§2-§3 with F-S3 and F-C3; 14 row 5: causal revocation is achievable, ADR 0017 seniority is kept for
the admin duel). `internal/listing` (imports `trust` only); `internal/app` composes `listing`, `trust`
and `policy`. Tests A1, A2, A4-A8, B5-B7. Stage 3.

**M4 — Structural guard G1-G10, per-change/per-job limits, Worker, permits, DRR** (v5 §5.2 with
F-C1, F-C2 and F-A1; ADR 0030 §4-5; 14 row 3: bounds before decoding, isolation).
- Locally, `store/chunked.ts` splits deletions and batches at 65,536 ops.
- `frontend/src/net/guard.ts` and `ancestry.ts` run in a Worker (256 MiB, kill then join before
  `Released`).
- A streaming header parser stops at op 65,537 and at 16 MiB decompressed.
- Tests:
  - B11-B14.
  - A 2 Mi-character honest delete lands as ≥ 32 changes and reloads.
  - A remote 65,537-op change is refused before WASM.
  - A legacy 2 Mi-op change loads under the Worker ceiling.
  - ADR 0030 oracle: no exception, save → load passes.
- Stage 2.

**M5 — History pages for large histories** (F-A2; Stage 2).
- When a receiver holds no copy of an admitted document, or refuses a sync message over the job bound
  with a recoverable error, it sends `history-request {doc, haveHeads}`. This is a new adapter type,
  validated by `Admission`, and only offered to peers that advertise the capability.
- The sender answers with topologically ordered `history-page` messages from
  `A.getChanges(A.view(doc, haveHeads ∩ known), doc)`. Each page is whole changes ≤ 16 MiB
  decompressed (F-C2 guarantees a change fits).
- The receiver runs each page through the guard, applies it, persists it, then requests the next.
  Ordinary sync then finds equal heads.
- Tests:
  - 200 MiB of history with zero net text growth arrives in pages within the §5.2 peak-RSS formula.
  - Restart mid-transfer resumes from the persisted heads.
  - A peer without the capability gets an explicit recoverable error, never a crash.
- This closes v5 §9 "oversized history sync" once those tests pass.

**M6 — Residency and lifecycle patch** (v5 §5.3, kept). M1's actor factory and M9's rebuild use the
same suspend/drain/generation machinery. Tests B8, B9, B16. Stage 2.

**M7 — Storage lanes and pagination** (v5 §5.1, kept). `internal/bridge` store stream v2 and
`GoStorageAdapter.ts`. Tests A10, B10, plus an H-1 probe (save then load > 64 MiB). Closes H-1.
Stage 2.

**M8 — Purge finality by FinalAck** (F-S4; 14 row 6: ERA finality, Autobase `signedLength`;
Stage 2).
- The logical purge stays reversible, as in v5 §4 phases 1-2: ≥ 30 days per generation g, durable,
  and a clock rollback only delays it.
- After the window, each admin device whose user has not restored first **persists, then publishes**
  `FinalAck{d, g, hash(p), MemberFrontier, DecisionFrontier, Sig}`. From then on that device refuses
  to sign `kept`/`trashed` for (d, g), and the UI says "effacement définitif engagé".
- Required set R(p) = the admin devices standing in the member fold at `past(Members(p))`. It is fixed
  by p.
- **Final(d, g)** holds iff every r ∈ R(p) has a held FinalAck and p is the winning `doc/d` value in
  the fold over the union of those FinalAcks' decision frontiers. This is a pure function of signed ops
  and monotone: an ack is never withdrawn and its frontiers are fixed.
- Once Final holds:
  - a restore of (d, g) that is not in the past of some FinalAck is ignored everywhere, because a
    finalised effect is not reverted (ERA);
  - replay or adoption of d is refused;
  - each holder writes an erase intent, fences, drains, removes the bytes, then marks done. All of it
    is crash-resumable.
- An absent device is removed, then the purge is re-issued: new generation, new window. There is no
  coordinator, prepare, vote, abort or lock recovery.
- Components: `internal/policy` (`FinalAck`, ≤ 96 (d, g) entries per op,
  `.inkwell/policy/final/`), the erase intent in `internal/store`, blob GC in `internal/blob`.
- Tests (A9 rewritten):
  - restore on day 29 recovers identical bytes and heads;
  - a FinalAck before day 30 is refused, also under clock rollback;
  - after its own FinalAck a device refuses to restore;
  - a pre-ack restore keeps Final false in every order;
  - a restore by an admin promoted after p, concurrent with Final, is overridden identically on
    3 replicas;
  - crash between intent and removal resumes;
  - a late save is refused;
  - an absent admin blocks; removing it and re-issuing unblocks;
  - blob GC follows Final.
- B15': ≤ |R(p)| FinalAcks per (d, g); a duplicate is ignored; one naming unheld ops counts once they
  are held.

**M9 — Equivocation: detection, containment, admin arbitration with deterministic rebuild** (F-S5;
14 row 4: detection by signed heads, ERA arbiter; Stage 3).
- **Detection.**
  - Guard G8 plus the M1 proof.
  - Go gossips proofs on the trust channel, ≤ 1 per (key, epoch).
  - On connect, peers exchange their proof sets before repository data, a split-view check after
    RFC 9162 §11.3.
- **Containment** (local, monotone):
  - stop relaying that actor at seq ≥ k;
  - refuse its not-yet-held changes at seq ≥ k;
  - the UI names the person and offers removal (ADR 0021).
- **Arbitration.**
  - An admin decision `equiv/<actor>/<k> = <hash>` goes in the ADR 0030 journal: D1 standing,
    D2 ≺-last when contested. The UI suggests the branch that more member devices have built on.
  - Every replica computes E = changes of that actor at seq ≥ k on a losing branch, plus all their
    descendants.
  - It rebuilds `applyChanges(init(), held \ E)` (probe 2) through M6 suspend/drain and a new handle
    generation with **fresh epochs**, so no honest actor reuses a seq.
  - E goes to a local quarantine (Go, opaque, D-2 30 days).
- **Recovery.**
  - Each losing branch that holds non-W changes is admitted as
    `RecoveryId = BLAKE3("inkwell/recovery/v1" ‖ d ‖ hash_lose)[:16]`, with G11 anchor `hash_lose`.
    It is quota-neutral and enters the admitted set by the decision, like `kept`.
  - At most one per member device per proof, since G8 lets a device hold only one branch.
- Tests (A16):
  - W sends c1 to R1 and c2 to R2; H builds on c1, J on c2; the admin picks c1.
  - On 3 replicas in every order: identical heads, J's changes intact in `RecoveryId`, no exception,
    and J's next edit uses a new actor.
  - Contested decisions converge to the same ≺-last choice.
  - The deciding admin cut concurrently → no data loss.
  - Re-equivocation inside the recovery branch → its own proof.
- Leaderless rules compared and rejected:
  - excluding every branch at seq ≥ k: an early equivocation would evict all honest work built
    since then;
  - lowest hash wins: W picks or grinds the winner;
  - most witnesses wins: colluders vote and the result is non-monotone;
  - first seen (G8 alone): order-dependent, which is today's split.

**M10 — D-3 detection and host disk floor** (14 row 3, within D-3; Stage 2).
- Meter: the guard measures decompressed bytes and ops accepted per author per day, and Go persists
  the counter. Admins are alerted at 256 MiB/day/author through the existing banner, and they suspend
  (ADR 0021).
- Floor: when free space on the project volume falls below max(1 GiB, 5 %), Go sends `DiskLow`. The
  adapter then pauses remote ingest for **every** peer (hold, then reset with a "disk" cooldown).
  Local edits keep saving into the reserve, and the alert names the top authors. Nothing is deleted
  and no author is singled out, so this is not a cumulative per-author block (D-3).
- Tests (B19): at the reserve + 1 B ingest continues; 1 B below it pauses ingest while a local edit
  saves and reloads; freeing space resumes ingest within one poll; the alert fires at the threshold and
  not one below it, once per author per day.

**Kept from v5 unchanged:** M11 member-gated peer table and explicit disconnect on removal (§5.6,
A14, Stage 2); M12 transport fairness, with per-peer reassembly and spill, router queues, credits and
P = 2 permits (§5.2, B12-B14, Stage 2; `internal/transport/mux` is a hot path, so benchstat applies);
M13 epochs and one adapter per Repo (§5.4-5.5, A12-A13).

### F.2 Open research: graded residuals and the mitigation we implement

| # | Residual | Grade (rubric) | Mitigation implemented | Why full resolution is not achievable now |
|---|---|---|---|---|
| R1 | W equivocates on **its own** `(actor, seq)`: replicas disagree until an admin arbitrates; honest work built on the losing branch leaves the main document (preserved in `RecoveryId`, not lost) | **Critique** (N7 / ADR 0031 determinism, during the window) — needs explicit acceptance | M1 compact proofs; M9 gossip-first detection, containment, alert, removal, admin arbitration, deterministic rebuild, recovery document | Automerge op ids are `(counter, actor)` (probe 1). A leaderless fix needs hash-based op ids (Kleppmann 2022 §3.3), a different CRDT encoding that Automerge 3.5 lacks and that has no demonstrated algorithm (14 row 4). The other route, finality before apply, needs Byzantine reliable broadcast with > 2/3 correct and quorum votes (Kleppmann §2.3.2), which contradicts offline editing (ERA §6) |
| R2 | Cumulative disk growth through churn, by a standing writer | **Critique** (ASVS 5.0 V2.4.1) — accepted only by D-3 | F-C2 per-change/per-job bounds, M4 fairness, M10 meter + alert + host floor (no corruption; sync pauses instead), admin suspension | **Not a research gap**: a per-member cumulative quota with admin grants (like D-1) closes it (14 row 3); **user decision D-3** excludes it. Only the absolute "never hinder" with unbounded history on finite disk is open |
| R3 | Erasure at a hostile or never-returning holder | outside the rubric (no rule can be met) | M8 on every honest device; UI: "effacé chez les personnes de confiance ; une copie déjà prise ne se reprend pas" | MLS (RFC 9420) and p2panda access-control exclude a global erasure guarantee (14 row 6) |
| R4 | Erasure waits indefinitely while a device of R(p) is absent | **Mineur** (delay, no loss) | removal + re-issue (M8) | finality without an availability assumption is impossible (ERA §5-6) |
| R5 | Concurrent admin decisions (duel): no neutral, immediate offline finality | **Mineur** | ADR 0017/0021 seniority, deterministic; ADR 0030 D2 "contesté" shown in the UI | neutrality + offline finality + no arbiter is open (ERA §6; MLS §14 leaves it to the application) |
| R6 | A restore by an admin promoted after p, concurrent with Final, is overridden | **Mineur** (documented, consistent with D-4) | UI explains the override | it follows from fixing R(p) without a coordinator (F-S4) |
| R7 | A standing admin arbitrates or purges maliciously | not a finding (ADR 0017, §1) | attributable signed decision | trusted role by design |
| R8 | Adoption/attestation storage quadratic in writers; ChangeSig ≈ 104 B per change and join-time verification | **Mineur** until measured (Majeur if a measured budget is exceeded) | B18 measurement; bounded by Q* and §6 | measurement, not research |
| R9 | Indivisible apply S_max | **Mineur** if B11 passes, otherwise Critique | per-change/per-job bounds, Worker, two-Repo escalation (§5.2) | measurement gate |

Activation blockers (gates, not residuals): pre-upgrade co-edits stay **Critique** until step 3.4,
and H-1 stays a hypothesis until step 2.1.

### F.3 Final ordered plan (Stages 2 and 3)

**Rules for every step:**
- Each step merges alone, with its tests.
- A feature stays behind a flag until its activation step.
- Every merge runs compatibility tests plus **ZZ-1/ZZ-2**.
- Heavy tests go through `scripts/heavy.sh`; `mux`, `frame` and `blob` changes are proven with
  benchstat.
- ADR 0030 §3 (index genesis) and §7 (B4b migration) follow ADR 0030's own track and are not
  re-planned here.

| Step | Content | Tests | Effect at merge |
|---|---|---|---|
| 2.0 | M11: member-gated router and peer table, explicit disconnect on removal | A14 | active; closes S1-R6 |
| 2.1 | M7: store protocol v2 (request ids, lanes, Cancel/Released, pages ≤ 1 MiB) | A10 (store side), B10, H-1 probe | active (transparent); closes H-1 |
| 2.2 | M12: per-peer reassembly + spill, router queues, DRR slices, credits | B13, B14, benchstat | active; S1-R5 measured |
| 2.3 | `internal/policy` journal (ADR 0030 §2), member ops v2 (`Seen`, `SeenListing` encoding), joint causal past; entries `doc/`, `quota/`, `legacy/`, `equiv/` decoded and folded, inert; `Decide`/`Policy` bridge | fold property in every order, FuzzDecode, golden vectors, A9 (a)(b)(g) | inert |
| 2.4 | M6: automerge-repo lifecycle + factory patch (generation checks, non-ready teardown, ready-only flush, actor factory **flag off**), suspend/drain residency | contract tests on 2.5.6, B8, B9, B16 | residency active |
| 2.5a | F-C2: local factory splits changes at 65,536 ops (`store/chunked.ts`, sheet, merge) | 2 Mi delete and paste round-trip | active |
| 2.5b | M4: guard G1-G10 in a Worker, header parser limits, permits Cancel/Released, webview DRR (keyed by peer) | B11, B12, F-C2 refusal tests, ADR 0030 oracle | active; S1-R1 closed except R1 |
| 2.5c | M5: history pages (capability-negotiated) | M5 tests | active |
| 2.6 | M1: ChangeSig, `Sign`/`Authored`/`NewEpoch`, guard check, G/legacy exceptions, G8 proofs + gossip; F-A1 author-charged DRR. Activation needs a `legacy/<d>` for every existing document and the capability on every peer | A15, B18 | flag `authored-changes`, on once preconditions pass |
| 2.7 | M8: FinalAck, Final fold, erase intent, blob GC | A9 rewritten, B15' | active |
| 2.8 | M10: per-author meter, alert, disk floor | B19 | active |
| 3.1 | `internal/listing` (variant/adopt, F-S3, rule E, issuance-L5, SeenListing fold) | A1, A2, A4, A6, A7, B5, B6 | Go only, inert |
| 3.2 | `List`/`Listed` bridge; Go admitted set (with F-C3) in **shadow** beside `scope.ts`; loader on 2.1 lanes | A10, A13, B7, shadow divergence counter | shadow |
| 3.3 | Migration: `legacy/<d>` for every pre-upgrade document (D-6), version negotiation, read-only adapter for old peers | mixed-version refusal, legacy admission | inert |
| 3.4 | **Single activation**: derived-id creation, Go admitted set authoritative, G11, M2 adoption at first co-edit, history rows | A3, A5, A8, ZZ-2 | active |
| 3.5 | M9: `equiv/` arbitration, rebuild, quarantine, `RecoveryId` | A16 | active |
| 3.6 | Retire `meta.variants` authority (presentation only); final gate A11 reconnect benchmark; audit S1 closed except accepted R1/R2 | A11, full §8 | done |

### F.4 What the user must decide

1. **Accept ADR 0030** (status `proposed`). Stage 2 is built on it, including the amendments listed in
   ADR 0031: `Seen`/`SeenListing`, joint past, entries `doc/`, `quota/`, `legacy/`, `equiv/`, FinalAck,
   ChangeSig in G8, guard limits.
2. **Accept ADR 0031 revision 6** (status `proposée`). This includes the simplifications F-S1..F-S5
   and the new admin duty of arbitrating a split (`equiv/`).
3. **R1**: accept the self-equivocation window as an explicit Critique residual (admin arbitration
   bounds it). No achievable alternative exists today.
4. **R2**: either accept cumulative disk growth as a Critique residual under D-3, or amend D-3 to a
   per-writer cumulative quota with admin grants, the only known closing mechanism.

D-1..D-6 are not reopened.

Sources added in revision 6: [14](14-research-byzantine-solutions.md) (rows 1-6, all sources dated
there); Kleppmann, [*Making CRDTs Byzantine Fault Tolerant*](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf)
(PaPoC 2022) §2.3.1, §2.3.2, §2.4 and §3.3 (re-read 2026-10-31); ERA v2
([arXiv 2601.22963](https://arxiv.org/html/2601.22963v2), 2026); RFC 9162 §11.3; Hypercore fork
proofs ([hypercore-next#76](https://github.com/hypercore-protocol/hypercore-next/issues/76));
the 2026-10-31 automerge 3.5.0 probe above.

## Stage 1 — scope as merged (decision N8, 2026-10-31)

**Status after Stage 1: S1 stays Critique** (ADR 0030 guard missing; see S1-R1 below). Nothing in
Stage 1 may be described as "S1 closed". Threat model N7 (full Byzantine) is the target of Stages 2-3;
Stage 1 delivers the hard bounds against **readers and unattested peers**, and keeps R1/ZZ-2 working.

### What Stage 1 implements (files)

| File | Change |
|---|---|
| `frontend/src/net/admission.ts` (new) | strict decoder `parsePeerMessage` + `class Admission` (per-peer session slot, ephemeral bucket, re-arrive bucket and pending slot, hold queue, reset cooldown); pure apart from an injected clock |
| `frontend/src/net/scope.ts` (new) | interim `AdmittedSet`: root ∪ `root.meta.variants[].url` (valid Automerge URL, id ≤ 64 chars), **session-sticky** (an id once admitted stays admitted until restart or project switch), ≤ 256 distinct variant ids per session; `epoch` incremented on root switch; `onChange` |
| `frontend/src/net/sharing.ts` | `shareAdmitted(set)`: `announce = access = set.has(id)` (replaces `shareHeld` and `() => true`) |
| `frontend/src/workspace/loader.ts` (new) | bounded bootstrap loader: `repo.find(url, {allowableStates: ["ready","requesting","unavailable"]})`, ≤ 4 in flight, FIFO of ≤ 257 ids, called (a) when an id enters the set, (b) when a message for an admitted id without a handle arrives; after every `await`, rechecks the set's epoch and the id's admission before acting |
| `frontend/src/net/WailsNetworkAdapter.ts` | calls `Admission` before Coverage; overrides peer metadata; throttles re-arrives; drops outbound ephemeral relays; `isPeerMessage` moves to `admission.ts` |
| `frontend/src/net/coverage.ts` | candidate fix kept (no per-document state for unattested ids, digest instead of message, caps) |
| `frontend/src/workspace/workspace.svelte.ts` | `new Repo({…, isEphemeral: true, shareConfig: shareAdmitted(set)})`; `open()` sets the root in the set **before** `repo.find`; root `change` → `set.refresh()`; `set.onChange` → `repo.shareConfigChanged()` + loader |
| Go `internal/app` (open path) + `internal/store` | one-time removal of every `<repo>/<doc>/sync-state/` directory **before the store stream serves the webview**; idempotent; logs a count |
| Go worktree agent-a55a2bd8 | **do not wire** `files.SetOnTree(w.SetDocuments)` (node.go:200): feeding the attestation live set from the disk tree would park variant attestations (R1 on the Go side). Keep the per-signer cap. |
| `frontend/e2e/history.spec.ts` | ZZ-1 and ZZ-2 permanent |

### Message types validated (anything else dropped, counted by reason, never logged with bytes)

| type | rule |
|---|---|
| `arrive`, `welcome` | payload ≤ 1 KiB; `senderId` string ≤ 64 = authenticated `from`; `peerMetadata` plain object or absent, **ignored**; `projectUrl` absent or string ≤ 80 passing `isValidAutomergeUrl` |
| `sync`, `request` | `senderId` = `from`, `targetId` = me (strings ≤ 64); peer announced; `documentId` string 1-64 **∈ admitted set**; `data` Uint8Array |
| `ephemeral` | same + `sessionId` `/^[0-9a-z]{1,16}$/`, `count` safe integer ≥ 1, `data` ≤ 1 KiB decoding to the presence schema (`__presence.type` ∈ heartbeat/goodbye/update/snapshot; channels ⊆ name ≤ 128 chars, anchor ≤ 64, head ≤ 64) |
| `doc-unavailable` | `senderId`, `targetId`, `documentId` ∈ admitted set |
| `remote-subscription-change`, `remote-heads-changed`, unknown | dropped (gossip is off) |

Objects must have `Object.prototype` as prototype; fields read with `Object.hasOwn`; the delivered
message is rebuilt from validated fields only.

### Bounds (numbers)

| What | Bound |
|---|---|
| Admitted ids per session | 1 root + ≤ 256 variants (sticky, distinct, per session) |
| `#hasRequested` keys, handles, synchronizers reachable from peers | ≤ 257 per session (only admitted ids reach the Repo) |
| Ephemeral rate | per peer bucket 128, refill 64/s; reset on PeerDown and on an accepted re-arrive |
| Ephemeral session keys in the Repo | `sessionId` rewritten to one random value per adapter instance, `count` to a per-instance counter → keys ≤ lifetime peers × adapter instances; the workspace creates **one** adapter per Repo, so ≤ lifetime peers (authenticated members) |
| Presence state | ≤ 3 channels of ≤ 128/64/64 chars per peer per document |
| Re-arrive | per peer bucket 3, refill 1 per 20 s, one pending slot (latest wins) |
| Hold queue (messages for an admitted id whose handle is not ready/requesting/unavailable) | ≤ 4 messages per (peer, doc); ≤ 8 MiB per peer, except one single message up to MaxFrame when the peer's queue is empty (honest first sync of a large document); **no shared global pool** (no peer can use another's capacity); overflow → drop that peer's queue, `peer-disconnected`, re-handshake not before a **cooldown of 30 s, doubling to 10 min, kept per peer id across reconnects** (map ≤ lifetime peers); PeerDown, re-arrive or epoch change → queue dropped |
| Sync-state files | **0 written** (all peers `isEphemeral`, no `storageId`); old ones removed by Go before attach |
| Coverage per-document state | ≤ admitted ∪ attested |

### `isEphemeral` / `storageId`

The adapter emits `peer-candidate` with `peerMetadata = {isEphemeral: true}` for every peer (the
peer's own `storageId` is discarded), and the Repo is created with `isEphemeral: true`. Then
`Repo.js:250-252` never saves, `CollectionSynchronizer.js:40-43` never loads, `Repo.js:157-160` never
calls `setSyncInfo`. No sync-state save is ever scheduled, so the Go cleanup (which runs before any
Repo exists) cannot be undone by a delayed throttled save (`throttle.js:80-104`).

### How R1 / ZZ-2 keeps passing without listing tokens

Two independent paths, both limited to the admitted set:
1. **Push from the creator.** `repo.import` registers the variant synchronously (`Repo.js:656-663`)
   and `addDocument` evaluates `announce` **before** `fork` writes `meta.variants`, so the first
   answer is "no". `set.onChange` (root changed, variant listed) calls `repo.shareConfigChanged()`,
   which re-evaluates sharing (`CollectionSynchronizer.js:177-201`) and starts the sync of the variant
   with every peer.
2. **Pull by everyone else.** When the root listing reaches Bob, the variant enters Bob's set and the
   loader `find`s it (≤ 4 in flight), so Bob fetches and stores it while Carol is online.
Exposure check: `main` (`sharePolicy: () => true`) already accepts and stores every document any
member pushes; Stage 1 fetches a strict subset (listed ids only), so it widens nothing.

### Explicitly not claimed by Stage 1

- Protection against a **malicious writer**: panics, divergence, unreloadable documents (ADR 0030 §4
  guard absent); displacement or exhaustion of the 256 interim variant slots; hiding a variant row by
  deleting its `meta.variants` entry (the document stays stored and session-sticky, but a peer that
  starts later will not admit it); listing foreign or arbitrary document ids.
- Any bound on **bytes** of admitted documents or on resident memory (all fetched documents stay
  loaded, as on `main`).
- Provenance of variant URLs, fairness among writers, lifetime bounds across restarts (the per-session
  cap resets at restart).
- Peak RSS during decoding of large frames (Go's MaxFrame is the only bound).

### Stage 1 tests (all must fail before the change where a bug is claimed)

1. Contract tests on automerge-repo 2.5.6: session dedup key/count; `hasRequested` before `access`;
   `isEphemeral` skips save/load/`setSyncInfo`; `#pendingSyncMessages` while loading; `addDocument`
   announces before `fork`'s change; ephemeral relay.
2. Strict decoder table (null, undefined, number, array, unknown and `remote-*` types, extra keys,
   `__proto__`, long ids, non-Uint8Array data, bad count/session, oversized handshake and ephemeral)
   + fast-check "never throws".
3. P2: 50 000 `request`s with invented ids → `access` spy 0 calls, `repo.handles` empty, heap < 4 MiB.
4. Interim churn: a writer lists/unlists 10 000 variant ids → ≤ 256 distinct ids reach `access`;
   a removed entry stays admitted until restart; project switch → epoch change drops held messages.
5. P1: 1 MiB session ids dropped; 10 000 distinct valid sessions → one session seen by the Repo
   (spy), heap < 2 MiB; lower count dropped; restarted remote session accepted (asymmetric
   reconnect); 1 000 messages in 1 s → ≤ 192 delivered; bucket reset on PeerDown/re-arrive; unknown
   channel dropped; no outbound relay.
6. P3: 40 re-arrives with 40 storageIds → no `sync-state` key saved, also with a storage whose `save`
   is delayed 1 s; re-arrive throttle (3, then 1/20 s, last pending processed); Go
   `TestOpenRemovesSyncStates` (before attach, idempotent).
7. Hold queue and loader: storage `load` blocked + 10 000 × 64 KiB sync messages for an admitted id
   without handle → loader starts one `find`, held ≤ 4 and ≤ 8 MiB, peer reset once, cooldown kept
   across a reconnect; a second honest peer's messages for another document are still delivered
   (no shared pool); unblock → drains at `requesting`; PeerDown while blocked → nothing delivered
   after the `await` (cancellation rechecked).
8. R1: unit (three repos, simnet: creator pushes after `shareConfigChanged`; a peer pulls via the
   loader) and e2e ZZ-1 + **ZZ-2** in `history.spec.ts` (wait on a test hook exposing the variant's
   heads at Bob, no sleep). Fails on the candidate fix, passes here.
9. Existing e2e (join, collab, history, offline, persistence) and `coverage.test.ts` unchanged.

### Stage 1 residual risks (regraded per rubric in v4)

| ID | Residual | Grade | Rule and evidence | Closed by |
|---|---|---|---|---|
| S1-R1 | a writer's forged change panics Automerge, splits replicas, or makes a document fail save → load, now also spread by the loader's pull of listed variants | **Critique** | ADR 0030 §4 guard absent; crash/data loss demonstrated by ADR 0030's probes (`0030-decisions-signees-garde-structurelle.md:31-46`) | Stage 2 (G1-G10) — except self-equivocation, §9 |
| S1-R2 | a writer fills the 256 interim variant slots, or deletes rows so later peers do not admit them | **Critique** | ASVS 5.0 V2.4.1 (quota exhaustion) and N7; `frontend/src/net/scope.ts:36-53` (shared cap 257, first come) | Stage 3 (§2) |
| S1-R3 | a writer lists arbitrary or foreign ids, which every peer then tries to fetch | **Majeur** | engineering-principles §4 and N7; measured impact: ≤ 256 unavailable handles × 66 KiB RSS ≈ 16.5 MiB per session (09, `handles.mjs`), plus futile fetches | Stage 3 (§2.2-2.3) |
| S1-R4 | admitted documents stay resident; bytes unbounded (a writer can list 256 large documents) | **Critique** (was Majeur) | ASVS 5.0 V2.4.1 ("overuse of costly resources": no limit at all); S1 itself was graded Critique on the same rule; 56-310 KiB per *small* document measured (09) | Stage 3 (§5.3) |
| S1-R5 | peak RSS while decoding frames up to MaxFrame (64 MiB) from several members | **Mineur until measured** (was Majeur) | a limit exists (MaxFrame per frame; mux reassembly 128 MiB, `internal/transport/mux/fragment.go:26-28`); impact not measured, so not Majeur under the rubric; regraded by test B12 | Stage 2 (§5.2) |
| S1-R6 | 1 024 peer ids that drain their presence budget occupy every entry of the admission table: a new honest member is refused for up to ≈ 33 min | **Critique** (audit plan: Majeur) | ASVS 5.0 V2.4.1 (denial of service to a member); `frontend/src/net/admission.ts:106,127-148`; strangers reach the webview during an open invitation: `internal/transport/p2p/node.go:476` admits any authenticated peer while `isOpen`, and `internal/bridge/router.go:80-93` forwards every envelope. The audit table should be updated to Critique | Stage 2, step 2.0 (§5.6) |
| — | readers and unattested peers (P1, P2, P3, P5, S4) | closed by Stage 1 | Stage 1 tests 2-7 | — |

## 0. Grounding (what the installed library and code do)

| Where | Behaviour | Consequence |
|---|---|---|
| `network/NetworkSubsystem.js:47-63` | unknown type → `JSON.stringify(msg)` in a log template; ephemeral → `#ephemeralSessionCounts[`${senderId}:${sessionId}`]`, never pruned | P5; P1 |
| `synchronizer/CollectionSynchronizer.js:87-96` | `#hasRequested` recorded **before** `shareConfig.access`; `removePeer` (`:156-165`) keeps keys | lifetime state per id that reaches the Repo |
| `CollectionSynchronizer.js:106-111` | the Repo itself calls `repo.find` when a message for an unknown document arrives | holding such messages in the adapter blocks that initialisation (review 2, point 2) |
| `Repo.js:236-243` | `synchronizer.receiveMessage(…).catch(…)`: fire-and-forget | no backpressure |
| `synchronizer/DocSynchronizer.js:259-266`, `:74-95` | `#pendingSyncMessages` while the handle loads; callbacks while a sync state loads | unbounded under blocked storage |
| `DocSynchronizer.js:217-222` `endSync` | deletes the peer and its status, **keeps** `#syncStates[peer]` | a "reset" resumes from retained shared heads; retained ≤ lifetime peers × documents |
| `Repo.js:128-131`, `:246-258`; `helpers/throttle.js:80-104` | peer metadata taken as sent; one throttle per storageId shared by all documents (a pending save is cancelled by the next document's) | P3; persisted sync state is lossy; GC races delayed saves |
| `Repo.js:656-664` | `import(bytes, {docId})` loads into a handle with **that** id | project-derived ids are possible without a fork of the library |
| `Repo.js:715-737` `removeFromCache` | on a non-ready handle, `handle.doc()` **throws** "DocHandle is not ready"; the handle stays cached (measured, 09) | unavailable/requesting handles cannot be evicted with the public API |
| `DocSynchronizer.js:240-257` | ephemerals re-emitted to other peers with the original sender | relays dropped by our sender check |
| `presence/*` | channel names peer-chosen; prune every 15 s, TTL 45 s; a talking peer is never pruned | bound by schema |
| `Repo.js:477-480` | `#loadDocumentWithProgress` awaits `storageSubsystem.loadDoc` **before** racing the abort signal | a `find` cannot be cancelled while storage is slow; only the caller's wait can be abandoned |
| `Repo.js:425-436`, `:451`, `:467` | the progress cache deduplicates loads per document id | abandoning a wait never multiplies loads: ≤ 1 underlying load per id |
| `Repo.js:499`, `:512`, `:536` | registration with the synchronizer and the `ready` notice run in the load continuation; `find` registers immediately | once a handle is observed `ready` in a later macrotask, no late re-registration can follow |
| `Repo.js:706` | `flush` calls `handle.doc()` | throws on a non-ready handle: callers must filter |
| `Repo.js:721` | `removeFromCache` waits for ready/unloaded/deleted/unavailable | on a loading/requesting handle it waits up to the 60 s handle timeout (`DocHandle.js:34`), then `doc()` is empty and nothing is removed (09) |
| `storage/StorageSubsystem.js:153-164`, `:215-239` | a save may compact: it writes a snapshot and **deletes** the chunks it knew | a stale throttled save after eviction and reload could delete newer chunks; eviction must be quiescent (§5.3) |
| Automerge 3.5 sync (09, `split.mjs`) | missing history travels as **one compressed bundle** per message | applying a message is indivisible; preemption happens between messages, never inside |
| Automerge 3.5 `decodeChange(c).author` (09, `author2.mjs`) | per-change `author`, hash-committed; `changeAt` keeps it on a 21-byte actor | an actor ↔ key binding can use `author` without touching actor bytes (G12, §7.1) |
| `frontend/src/store/GoStorageAdapter.ts:57`; `internal/bridge/storestream.go:293`, `envelope.go:35` | the store stream answers one request at a time; a `LoadRange` answer must fit one 64 MiB frame | head-of-line between loads; a document stored above 64 MiB cannot load (hypothesis H-1, §9) |
| `internal/transport/mux/fragment.go:26-28`, `:229-231`, `:275-277` | reassembly: one partial message per peer, a **shared** 128 MiB budget; when short, the message that progressed least recently is dropped; memory is released at hand-off, before the router and the webview | a fast hostile sender can evict a slow honest one; delivered bytes are no longer counted (§5.2) |
| `internal/app/attest.go:116-160` | the device signs heads per document it changed | Go knows, without reading Automerge, which documents this device contributed to (auto-adopt, §2.2) |

Measurements (09): reconnect with fresh state costs ≈ 1.25 B and ≈ 2 µs per change per document for
a single-actor append workload (148 B vs 12 623 B at 10 000 changes); a loaded Automerge value is
56-310 KiB RSS; an **unavailable `DocHandle` is ≈ 28 KiB heap / ≈ 66 KiB RSS**; presence snapshot
265 B. ADR 0028 (`0028-performance-huge-files-and-wan.md:28-29`): loading 1 Mio of text takes
0.48 s, 5 Mio 2.6 s on the main thread.

## 1. Threat model N7 and which layer answers what

Attackers: any current member, including a **malicious invited writer**, a writer who is later
removed or suspended, and several colluding writers. Readers and unattested peers must be strictly
bounded; a writer must never hinder others (availability, visibility of their documents, their
quota, their CPU/memory share), and every effect a writer has must be **attributable** to it and
**bounded per writer**. Admins are trusted for decisions (ADR 0017): a malicious admin can purge or
keep anything by a decision, so §4 protects against honest concurrency, revoked admins and
writers, not against a standing malicious admin.

| Layer | Answers | Stage |
|---|---|---|
| Peer table gated by membership (§5.6) | which peers may hold any state in the webview | 2 |
| Admission boundary (adapter) | which messages may make the Repo hold state, and how much, per peer | 1, extended in 2-3 |
| Coverage T10 (ADR 0018), by author key with G12 | authorisation/coverage of changes, not proof of local editing | exists; G12 in 2 |
| Structural guard G1-G10 (ADR 0030 §4) | whether a change is well formed (no panic, no divergence, save → load) | 2 |
| Fair scheduler (§5.2, shared with ADR 0030 §5) | whose work runs next, how long, how much memory | 2 |
| Anchor rule G11 (§2.3) | whether a change belongs to a listed document's lineage | 3 (in the guard module) |
| Document chains (`internal/listing`) | which documents belong to the project, per writer, attributable, lifetime-bounded | 3 |
| Decision journal (ADR 0030 §2) + purge stability (§4) | purge, quota grants, `kept`; when bytes may be erased | 2 |

What no layer prevents, by design: a current writer editing the **content** of shared documents
(that is what writing is). It is attributable (history), reversible (versions, revert), and stopped
by removing the writer. This is not "hindering" in the N7 sense; it is stated so nobody assumes
otherwise. Its cumulative disk cost is a user decision (§11, D-3).

## 2. The admitted set: one chain of listing ops per writer, document ids derived from the slot

### 2.1 Alternatives

| Option | Lifetime ids per writer under equivocation | Cross-writer aliasing | Provenance of content | Others' co-edits under the author's equivocation | Cost |
|---|---|---|---|---|---|
| v2: numbered tokens naming any doc id | **unbounded** | **possible** | none | n/a | small |
| (b) id = H(project, writer, kind, n) only, no anchor | 1 per slot | impossible | **squattable** (rival history) | lost | small |
| (c) listings in the ADR 0030 decision journal | bounded | impossible | as (e) | kept | a writer could seal the shared 8 192-op journal: rejected |
| (d) actor ↔ key binding, attribute each `meta.variants` op | bounded with add-only schema | impossible | per-actor rules | depends | large |
| (e) v3: per-writer chain; id = H(project, writer, kind, position, anchor); G11; freeze on equivocation | ≤ Q_w + 1 | impossible | anchored | **lost** (review 3, point 1: the freeze excludes them) | moderate |
| (f) Codex review 3: id = H(project, writer, kind, position) **without** anchor; anchor signed and imposed by G11 | 1 per slot | impossible | anchored **only without equivocation**: two ops at one slot give one id with two anchors; which one G11 enforces depends on arrival order (the freeze rule retains the first conflicting op each peer saw) → replicas disagree on the document's admissible lineage | lost | moderate |
| **(g) v5 = (e) + `adopt` ops + `Decisions` frontier (chosen)** | own ≤ Q*_w; others may keep ≤ 1 branch per (adopter, slot) | impossible | anchored; one anchor per id, so G11 is a function of the id | **kept**: every co-editor adopts automatically | (e) + one op kind |

(f) is simpler only when nobody equivocates; under N7 it makes G11 order-dependent, so the anchor
stays in the id. (g) adds one op kind and no new mechanism: an adoption is a listing op on the
adopter's own chain, so revocation, freeze and retention rules apply to it unchanged.

### 2.2 The listing op (`internal/listing`, Go)

Canonical binary, domain `inkwell/listing-op/v1`, Ed25519 over the **full tuple**:

| Field | Content |
|---|---|
| `Project` | ProjectID (32 B) |
| `Prev` | hash of the author's previous listing op, or empty for position 1 |
| `Members` | member-list frontier seen by the author, non-empty, ≤ 64, strictly increasing |
| `Decisions` | decision-journal frontier seen by the author (ADR 0030 §2), ≤ 64, strictly increasing, may be empty (review 3, point 12) |
| `Position` | 1 + position of `Prev` (chain sequence, including adoptions) |
| `Slot` | variant ordinal = 1 + number of variants in the prefix; absent on adopt |
| `Kind` | `variant`, `adopt`; `node` reserved for the ADR 0023 index (16-byte `Node` field), **not implemented until needed** |
| `Anchor` | `variant`: hash of the anchor change of the new document (32 B), §2.3 |
| `Adopted` | `adopt`: the complete canonical bytes of another author's `variant` op (≤ 4.4 KiB) |
| `Author`, `Sig` | device key, signature |

Size ≤ 2·64·32 + 200 B ≈ 4.3 KiB for a variant, ≤ 8.7 KiB for an adoption (typical ≈ 300 B /
600 B). `Decode` strict, golden vectors shared with TS, `FuzzDecode`. **Document id** (never
transmitted, always recomputed from a `variant` op, embedded or not):
`DocId = BLAKE3("inkwell/doc/v1" ‖ Project ‖ Author ‖ Kind ‖ Slot ‖ Anchor)[:16]`, rendered with
automerge-repo's `binaryToDocumentId`. A writer cannot name another writer's document (aliasing),
a foreign or arbitrary id (provenance), and each (author, variant slot, anchor) yields one id.

→ F-S3 (no prefix arithmetic). **Separate slot from chain position (v5).** Interleaved adoptions must not move a writer’s
variant quota into arbitrarily many document positions under equivocation. Hashing chain Position
(v4) is rejected for that reason; hash the signed variant ordinal Slot instead. An embedded variant
requires a verified causal prefix proving its ordinal and issuance quota (L2 fetches hashes, L3
sequence/Slot arithmetic applies), but NOT current standing or absence of equivocation of that
prefix. No unproved arbitrary Slot is accepted. Adoption deduplicates (foreign author, Slot),
not chain Position. Freeze still uses chain Position. Retain referenced prefix proof hashes/ops
under the adopter’s lifetime budget, deduplicated; branches are bounded by adopted slots × bounded
prefix length. The draft wire format is not activated, so this changes no deployed ids.

**Admission of an op** (pure function of the op and its causal past, like `List.integrate`):
- L1 canonical, signed, for this project.
- L2 `Prev`, `Members` and `Decisions` held; otherwise wait in a buffer **per author**, ≤ 4 ops,
  standing writers only; a fifth evicts that author's oldest, never another author's.
- L3 `Position` exact; for variant, `Slot` equals its prefix variant count + 1 (canonical u64).
- L4 the author is a writer, not suspended, in the member list folded on `past(Members)` (ADR 0030 A4
  analogue with role ≥ writer).
- L5 `variant`: the number of `variant` ops on the chain up to and including o is ≤ `Q(o)` (§2.5).
  `adopt`: the embedded op e passes L1, L4 and issuance-L5 at its own frontiers (L2 waits for them),
  `e.Author ≠ o.Author`, `e.Kind = variant`, and no earlier op on o's chain adopts the same
  `(e.Author, e.Slot)`. Prefix validity/Slot arithmetic is checked for e, but current chain standing is **not**:
  adoption must work precisely when e’s chain is frozen or dead.

**Equivocation (rule E).** Two admitted ops of the same author with the same `Position` (different
hashes) form a proof. The author's chain is **frozen** at the smallest such position k: its own ops
at positions ≥ k stop standing everywhere; the proof (two ops) is kept and gossiped, the UI names
the person, the escalation is removal (as ADR 0030 G8). Retention per author: its chain up to k−1,
plus the two ops of the proof; ops arriving later at positions ≥ k are not stored — except as the
`Adopted` payload of another writer's standing adoption, which carries its own copy.

→ F-S2 (receipts dropped; M2). **Contribution is not coverage (review 4, #1).** `attestHeads` (`attest.go:124`) and
`WailsNetworkAdapter.ts:277,292` sign heads after relay too. They MUST NOT populate `contrib`.
A contribution receipt is a separate domain `inkwell/contribution/v1`, signed by the local key,
containing Project, id, local change hash and covered heads. It is emitted only after a local
`change`/merge command produces a nonempty delta, G12 verifies that delta's author is this key,
and its bytes are durably saved. Receive/load/relay/empty change never emits it. Go stores at most
one receipt per (device, foreign author, slot), with the embedded variant op; no text in it.
Go remains opaque to Automerge; the trusted local webview checks the delta, Go signs the typed
receipt. A Byzantine device can claim its own contribution, but only consumes its own bounded
adoption slots. No assertion about H follows from W's receipt.

→ F-S1 (ChangeSig tuple, M1). **Cryptographic authorship is also distinct from relay coverage.** Every new non-genesis/non-legacy
change c needs `AuthorshipSig = Ed25519(key, "inkwell/authored-change/v1" || ProjectID || hash(c))`.
Only the local change factory may request this signature, after G12 self-check; a receive/relay path
NEVER signs it. Hash(c) commits author, actor, descriptor, deps and operations. Guard requires this
signature from c.author before accepting c, even if relay coverage T10 is complete. Otherwise H's
automatic relay attestation would let W label its forged actor/author as H and obtain H's coverage.
Authorship signatures travel as an opaque sidecar on the trust channel, bounded by the same per-job
bytes/ops and missing-proof queue (4 per author); Go signs hashes, never decodes Automerge. Persist
96 bytes (hash+signature) per authored change alongside history; fetch missing proofs before guard,
not via an unbounded in-memory index. This lifetime disk cost is part of D-3's explicit residual.
Contribution receipts are first-co-edit bookkeeping, not a substitute for every-change signatures.


**Adoption persisted at first real co-edit (chosen in v5).** For a standing writer H's first local
contribution to a foreign listed document, persist `adopt(e)` and the contribution receipt before
acknowledging the edit as durable or publishing it. A crash before adoption keeps the edit in a
local recovery journal; retry the same adoption idempotently on restart, then publish. No dependency
on detecting disappearance, no temporary admission gap for a completed co-edit. Adoption is automatic
(N9 D-5), quota-neutral for H, and never overrides a current logical purge or an erasure certificate.
L5 for an embedded op uses **issuance-L5**: fold the joint causal closure of e's signed frontiers,
count only grants valid in that closure. This is independent of later cuts. Ordinary variant standing
still uses current standing grants (§2.5). Thus an op issued with a valid grant can be adopted when
that grant later dies, while a 33rd op issued before any grant cannot. No reliance on a peer's claim
that it once admitted e; e's frontiers and grant signatures are the proof. L2 fetches this proof.

**Cost comparison (32 slots/writer, typical adoption 600 B; worst 8.7 KiB).** Deferred adoption
(v4) stores receipts for every co-edit and detects loss on every fold; eager adoption stores the same
maximum number of receipts plus one op earlier. If all W writers co-edit every other's 32 variants,
both eventually require `32 W(W−1)` adoptions: W=10 → 2,880 (1.73 MB typical / 25.1 MB worst);
W=100 → 316,800 (190 MB / 2.76 GB). With no orphaning deferred stores 0 adoption ops, eager stores
that maximum; both retain contribution evidence. At 10% co-edit density these op/byte figures divide
by ten. Eager removes a loss detector, its race and its confirmation window; selected for that
simplification, without claiming less storage. Copying into H's quota is rejected (penalises H).

**Lifetime bounds.** A grant later withdrawn cannot shrink history already adopted. Define
`Q*_w = 32 + 32 × distinct causally valid grants ever issued to w` (bounded by the decision-journal
limit, ADR 0030); current `Q(o)` is not a lifetime adoption bound. One adoption per
(adopter, foreign author, slot), so ≤ Σ Q*_v per adopter, ≤ `(W−1) Σ Q*_v` globally.
L5 forbids adopting one's own op, duplicate foreign slots or nesting an adoption. Eager adoptions consume
chain positions too: retention bound is variant ops + adoption ops + two equivocation-proof ops,
not Q alone. A colluder can preserve one rival branch per foreign slot, attributable to it.

### 2.3 Creating a document and the anchor rule (G11)

Fork flow (variant, `frontend/src/history/versions.ts:170-198`): the webview builds the copy
(`A.clone(A.view(doc, at))`; the copied history is required because variants merge back into the
main document), then makes one **anchor change** on it (`meta.origin = {from, by}`; its deps are the
copied heads, so its hash commits to the whole copied history). It calls the bound method
`Session.List("variant", anchor)`; Go checks the role and `Q`, assigns the next position, signs,
**persists, then gossips**, and returns `DocId`. The webview then `repo.import(bytes, {docId})`
(`Repo.js:656-664`) and writes the presentation entry in `meta.variants`. An import from elsewhere
follows the same flow: bytes are copied into a **new** listed document; a foreign id is never reused.

**G11 (anchor), in the guard module (Stage 3, uses ADR 0030's ancestry index).** For a document d
admitted through a `variant` op with anchor a (one anchor per id, §2.1):
1. **Nothing of d is accepted before a.** While d holds no anchor, a message for d is processed as
   one batch (it is one bundle anyway, 09 `split.mjs`): decode C = its changes; require a ∈ C;
   mark `past(a)` by walking deps from a **inside C** (a Merkle walk: each dep hash must be in C, or
   the batch is refused); every c ∈ C must be marked or have a ∈ before(c); then G1-G10 run over C in
   topological order. One failure refuses the whole message (ADR 0018 fragment rule). A batch
   without a (fragmented or hostile delivery) waits in the sender's G2 quota (ADR 0030 §5) and is
   dropped when evicted; honest holders always hold a (they could not accept d otherwise), and sync
   resends it.
2. **The anchor itself** is accepted iff `hash(a) = Anchor(o)` (signed by o's author) and a passes
   G1-G10 with `before(a) = past(a)` from step 1. No other property of a is required: its author
   chose it and signed it.
3. **After a is held**, a change c is admitted only if `c ∈ past(a)` (already held: nothing new can
   enter) or `a ∈ before(c)` (one vector-clock comparison). A rival parent is refused transitively:
   c with deps {x, r}, r rival, waits on r (G2), r is refused by G11, so c is refused.
4. **A descendant using another member's actor** passes G11 (it descends from a); it is refused by
   G12 (Stage 2, §7.1), and only by G12. Before G12 it is the ADR 0030 §4 residual.

G11 is a function of c, `before(c)` and the fixed a, so it is identical everywhere.

### 2.4 The admitted set

`Admitted = {RootDoc} ∪ {IndexG (ADR 0030)}`
`∪ {DocId(o) : o standing variant op (§3), Position(o) < freeze point of its author}`
`∪ {DocId(e) : e adopted by a standing adopt op}`
`∪ {d : doc/d = kept standing, d derived from a held variant op}`
`− {d : doc/d = purged standing (§4, logical phase)}`.
→ F-C3: add `∪ {d : legacy/<d> standing}`, for pre-upgrade ids.

Every term is a function of signed ops and decisions; every peer converges to the same set; no
writer's ops can remove another writer's id (only its own, and co-edited ones survive by adoption).
`kept` can only name an id derived from a held signed op, so an admin cannot admit a foreign id.

Go owns it (`internal/listing` verifies, persists `.inkwell/listing/<hash>.lop`, syncs on the trust
channel with a type byte, motif `trust.Sync`); `internal/app` composes `listing`, `trust` and
`policy` (`listing` imports `trust` only). The webview receives the set by `Listed` notices (with
`(ProjectID, epoch)`), uses it for admission, sharing, loading and the attestation live set.
`meta.variants` is presentation only; the history panel shows a row for **every admitted document**
("Variante de <personne> (sans nom)" when no entry names it), plus rows "à confirmer" for held
documents that left the set without a purge.

### 2.5 Quotas, with a causal frontier of decisions (review 3, point 12)

`Q(o) = 32 + 32 · |G(o)|`, where `G(o)` = the `quota/<o.Author>` decisions that are **in
past(Decisions(o))** and **standing in the decision fold** (ADR 0030 D1). Consequences:
- A grant issued after o never validates o: a 33rd variant signed before its grant is dead for
  ever, on every replica (no dependence on "currently standing" at reception time).
- A grant in o's past that later dies (its admin's cut was concurrent, D1) takes o's standing with
  it, on every replica, as soon as the cut is held: re-evaluation is the pure fold over the held op
  sets, recomputed on every change of `trust`, `policy` or `listing`; same sets → same admitted set
  (convergence as ADR 0017/0030). Co-edited documents of o survive by adoption (§2.2).
- L5 never reads local time or arrival order.

32 is an assumption (no usage data), recorded in `docs/open-questions.md`; the UI refuses the 33rd
variant with "limite de variantes atteinte — une personne qui administre peut en accorder d'autres"
and shows admins a grant action. Raising the base later is compatible (§11, D-1).

## 3. Revocation: causal standing of listing ops

Alternatives:
- (A) **Causal standing, as ADR 0030 D1 (chosen).** Member ops that cut, suspend or record the leave
  of a person y carry `SeenListing(y)`: the hash of y's chain tip that the author held (0 or 1 hash;
  2 if y has equivocated). This rides on ADR 0030's version-2 member ops (`Seen`), as a typed entry.
  A listing op o of y **stands** iff (a) L4 holds at `past(Members(o))`, (b) for every standing
  cut, suspension or leave m of y: either m ∈ `past(Members(o))` (then (a) decides, including a
  later let-back, ADR 0021), or o is at or before `SeenListing(m)` on y's chain, (c) L5 holds with
  the standing grants of §2.5, and (d) o is below y's freeze point (rule E). Otherwise o is dead.
- (B) EverWriter plus quota: contradicts N7 ("post-cut issuance accepted") and review 2.

Dead listings: their documents leave the admitted set; they are **not deleted**. Holders show them
"à confirmer"; a co-editor's automatic adoption (§2.2) or an admin decision `doc/<id> = kept`
re-admits them. Folding is pure and identical everywhere (`listing` reads `trust.CutsOn(key)` with
`Seen`, as `policy` does), and is recomputed whenever `trust`, `policy` or `listing` changes.

## 4. Purge: logical tombstone first, disk erasure only after stability (review 3, point 13)

**The problem.** In ADR 0030 a `purged` decision is not permanent: D1 kills it when a cut of its
author is concurrent (`0030-decisions-signees-garde-structurelle.md:153-159`), and D2 lets a
concurrent `kept` win (`:165`) and a later decision overwrite it. v3 erased `repo/<DocId>/` as soon
as the purge stood locally, so an annulment arriving later found nothing to restore.

**Alternatives.**
- (P-a) Erase on local standing (v3): data loss on late annulment. Rejected.
- (P-b) Never erase (purge = hide): safe, but contradicts the user's intent of a purge and ADR 0023
  blob GC; disk grows for ever. Rejected.
- (P-c) Fixed delay (erase T after local standing): a cut held offline longer than T still annuls
  after erasure; T trades safety for space without a bound. Rejected as the safety mechanism, kept
  as the mandatory N9 30-day recovery window, not proof of safety.
- (P-d) **Two phases, erasure gated by causal stability (chosen)**, the CRDT notion of a causally
  stable operation (Baquero, Almeida, Shoker, *Pure operation-based replicated data types*, 2017):
  an operation is stable when no concurrent operation can still be delivered.

**Semantics (amendments to ADR 0030, required in Stage 2, §7.1).**
- **Reversible logical purge.** Until physical erasure, sequential `kept`/`trashed` can
  restore the document; concurrency keeps D2 (`kept` > `trashed` > `purged`). Only an
  **erasure certificate**, issued after durable removal, is absorbing (D-4), not a purge decision.
- **Joint causal past.** "o ∈ past(x)" follows links in both DAGs transitively: member ops reach
  decisions through `Seen`, decisions reach member ops through `Members`. A cut whose author's
  promotion op had seen p has seen p. (ADR 0030 D1 currently tests `o ∈ past(Seen(m))` only.)
- → F-S4 (FinalAck, M8). **Acknowledgement.** Each admin device publishes `Ack = {Author, MemberFrontier, DecisionFrontier,
  Sig}`, latest-wins per device, each new one including the previous (monotone), on the trust channel,
  persisted `.inkwell/policy/acks/<key>` (≤ 1 per admin device). An `Ack` counts only once every op
  its frontiers name is held. An honest client emits one when it holds a standing purge it has not
  acknowledged (debounced 60 s).

**Phases (Go, `internal/policy` + store).**
1. Logical purge: remove admission, suspend inputs and drain saves (§5.3); keep bytes and allow
   restore by standing `kept`/`trashed`. A restored generation cancels all old purge timers/Acks.
2. Recovery window: **30 days minimum** from durable first receipt of this purge generation on the
   erasing device, persisted across restart (clock rollback delays, never accelerates). Every standing
   admin device must acknowledge the current joint frontier including p. An absent/hostile device
   blocks indefinitely until removed: 30 days is NOT an erasure deadline. A new purge after restore
   starts a new generation and a new 30-day window.
3. → F-S4 (FinalAck, M8) replaces phases 3-4. Erasure barrier: simple Acks are insufficient now that sequential restoration is allowed.
   After the window, an admin coordinates a per-id, per-generation prepare on the trust channel.
   Each standing admin device durably locks that key, supplies its complete member/decision frontier,
   and votes only if p still wins; the coordinator fetches/folds their union. Votes bind the same
   membership frontier, p and union frontier. A changed membership or restore invalidates prepare;
   repeat, or abort and unlock. A locked honest device queues restore requests and requests abort;
   it cannot issue unseen `kept` or conflicting prepare votes. The lock also defers
   member promotions/cuts/suspensions that would change the participating admin set; request abort
   before issuing such operations. Pre-vote operations are included in the union, so no honest
   device can leave a hidden concurrent restoration/membership mutation behind its vote. Commit vs abort is durably serialized
   by the coordinator's store command; a crash requires recovery of that outcome before unlocking.
   No timeout unlock (would permit a stale commit). Coordinator loss can block erasure, not editing
   of other documents; recover its durable outcome before unlocking. Do not claim that simply
   removing a coordinator safely invalidates an unseen commit: administrative recovery requires
   its outcome or a replacement agreement that fences the old coordinator before any new vote.
   Restore remains available until the local erase transaction starts; if racing a commit, it either
   aborts before erasure and restores, or receives the terminal erasure result. A remote UI shows
   restoration pending until abort is confirmed, never claims success before that confirmation.
4. At erasure commit, suspend/drain all local work for id, durably write an **erase intent** with the
   unanimous prepare proof, fence writes, remove directory/blobs and SyncDir. A crash resumes the
   intent before serving storage; never publishes a certificate while bytes still need erasure.
   Only after durable removal publish the signed **definitive erasure certificate** (id, generation,
   purge, prepare proof, eraser, signature). Other holders first drain and erase their own copy;
   their completion acknowledgements distinguish project-wide erasure from this device's completion.
   The intent fences late saves locally; the verified certificate prevents replay/adoption from
   re-admitting this id globally. Certificate validity uses the unanimously bound member frontier; an honest prepare cannot
   straddle an unacknowledged membership change. Once erasure is certified it is terminal even if
   a later cut targets the eraser (N9 D-4); ordinary D1 revocation cannot resurrect erased bytes.

**Alternatives after review 4.** Absorption at logical purge is simpler but violates N9's recovery
window. Delay plus ordinary Acks permits a restore signed after an Ack to arrive after deletion.
The selected short per-key prepare barrier closes that race for honest admins; it is not a consensus
solution against malicious admins (trusted decisions, §1). Its locks and outcome recovery are required
contracts, not existing implementation. Restoring on **day 29 even when all Acks arrived on day 1**
must recover identical bytes/heads and cancel erasure after day 30. Restore after certificate is
refused; an absent admin blocks erasure for years if necessary. The same protocol gates blob GC.
Listing positions are never returned. Replay cannot undo a certificate. Store fencing is shared with
§5.3, so a throttled/late save cannot recreate an erased directory.

## 5. Boundary in Stages 2-3 (deltas over Stage 1)

### 5.1 Bootstrap loader: one lane per owner, no shared slots (review 3, point 2)

Stage 1's loader (`frontend/src/workspace/loader.ts:4-48`) has one FIFO of ≤ 257 ids and 4 shared
slots: four ids whose storage load is slow occupy every slot and H's document waits (review 3).

**Owner of an id**: `project` for the root and the index; `local` for documents this person opens;
otherwise the author of the standing `variant` op, or, for an adopted document, its adopter.
For lane bounds W counts lifetime writer-device identities including retired/cancel-pending owners
(not just current membership), bounded by the 1,024-op member journal. A removed identity’s lane
retains its one slot until Released; membership churn never resets it. At an epoch switch teardown
old resources before creating replacement resources, rather than multiplying abandoned epochs.

**Rules (v5).** One FIFO per owner, deduplicated per id. A load transaction owns a lane
lease from scheduling through **underlying completion or confirmed cancellation**, not through a
10 s `Promise.race`. At most one outstanding storage request (queued OR executing) per lane;
a multipart transaction requests its next page only after its previous response. No second find is
started while an abandoned transaction still holds the lease. At 10 s detach the UI waiter, mark
cancel-pending, keep the lease and bounded handle; peer/admission/epoch removal does the same.
Retry only after `Released(lane, generation)`; late results are discarded by generation checks.
Ownership is fixed for the lease lifetime; opening an id or changing adopter cannot migrate a stuck
request into another lane. Distinct abandoned slots ≤ lifetime owners with work, one per lane,
and the old slot must be released before that owner gets a new one.

**Actual storage scheduling required.** `GoStorageAdapter.ts:57` and Go currently process requests
sequentially. Add lane/request/generation and Cancel/Released to the opaque store protocol, with
one queued request per lane; Go dispatches round robin, one bounded page per turn, preserving FIFO
for saves/loads of the same id and fencing erasure after its saves. Paginate LoadRange (≤ 1 MiB
response/page) before enabling this path. A blocked syscall must not hold the dispatcher: one worker
per lane (≤ writers + 2), independent responses demultiplexed by request id, no global worker pool
that W can occupy. Cancellation removes queued work immediately; an executing uninterruptible I/O
keeps that lane's lease until completion. H's worker dispatches within one dispatcher turn even if
all W workers are stuck; no wall-clock disk-latency guarantee is claimed for failed shared hardware.

Alternatives: releasing only the JS wait accumulates requests (rejected); keeping the lease alone
bounds backlog but the sequential Go worker still blocks H (rejected); lane workers plus bounded
pages and real request scheduling are selected. Go still sees opaque bytes/keys, no Automerge.
Non-ready abandoned handles require the lifecycle cancellation contract in §5.3; they are NOT bounded
by the currently admitted set. B8/B10 include repeated deadlines, admission removal and late resume.

### 5.2 Fair scheduling, preemption, and bounded queues (review 3, point 14)

Preemption is only possible **between** messages: an Automerge sync message is one indivisible
bundle (09, `split.mjs`). The design therefore (1) keeps transfer time from holding any shared
resource, (2) makes every queue per peer and bounded, (3) shares the few global resources by FIFO
turns with short, deadline-bounded holds, (4) bounds and charges the indivisible slice.

**Go, transport and router (Stage 2).**
- **Reassembly without a shared pool.** Today a 128 MiB budget is shared and the least recently
  progressing message is dropped (`mux/fragment.go:26-28,229-231`), so a fast hostile sender evicts a
  slow honest one. Change: each member peer has a **reserved** in-memory reassembly of 4 MiB; a
  message above 4 MiB is reassembled into a per-peer temporary file (`os.Root` in the app cache,
  ≤ 1 per peer, ≤ MaxFrame, removed on completion, drop, PeerDown or `partialTTL`). No peer's
  reassembly can evict another's. Disk use ≤ members × MaxFrame, attributable per peer.
- **Per-peer inbound queues in the router** (today `Router.Run` forwards each envelope in arrival
  order, `router.go:80-93`): small frames (≤ 1 MiB) ≤ 8 MiB per peer; at most one completed large
  frame per peer (on disk). Overflow → that peer is reset (`Resetter`, `router.go:55-59`) with the
  cooldown of §5.2 "Overflow"; nobody else's queue is touched.
- **Delivery to the webview by deficit round robin** (Shreedhar and Varghese, 1996) over peers, in
  slices of ≤ 1 MiB (`DataPart` envelopes, reassembled per peer in the webview), quantum 256 KiB:
  the single Wails stream is never held by one 64 MiB write.
- **Credits.** The webview grants each peer 2 MiB of small-frame credit and returns it as its
  scheduler consumes; Go never sends beyond credit (receiver-driven flow control, as QUIC stream
  credit, RFC 9000 §4).
- **Large-frame permits: P = 2 live allocations**, FIFO, ≤ 1 per peer. Tokens include project,
  epoch and unique generation. States: free → delivering/processing → cancelling → released.
  `Done` is accepted only after guard/apply ends and buffers are freed. PeerDown, 30 s expiry,
  admission removal and epoch change send Cancel and stop slices, but **do not return the token**.
  The owner cancels queued callbacks, waits for running work or terminates its isolated Worker,
  drops buffers/reassembly, then sends `Released(token)`. Go reuses capacity only after this ack;
  stale Done/Released cannot free a newer token. Worker death/join is release confirmation, not a
  timer. An uninterruptible main-thread apply retains its token until return; therefore bounded
  wait guarantees require the Worker and its bounded termination, delivered before activation.
  Test k colluders: H is dispatched within ⌈k/2⌉ completed cancellation/service turns, each ≤30 s
  plus measured Worker termination/join time. A failed release blocks that slot (observable error),
  never permits a third live allocation. The same cancellation accounting covers small guard jobs.

**Webview (Stage 2, one mechanism shared with the guard, ADR 0030 §5).**
- Per-peer queues, bounded by the credit (2 MiB small) plus ≤ 1 large frame (permit holders only);
  inside a peer, one lane per document served round robin, so W's content relayed by H does not
  block H's own documents in H's queue.
- **DRR over peers on measured cost**: each step (guard work units, then the apply) is timed;
  quantum 8 ms; the measured cost is charged to the peer's deficit, so a peer that just used a long
  indivisible apply is skipped until it has repaid it. Work-conserving: a lone peer gets 100 %.
- **Two classes** (as ADR 0028 D5): interactive (≤ 64 KiB) first; background guaranteed one turn in
  four.
- **Guard work is preemptible** (it is our code: yield every 8 ms between changes); **the apply is
  not**: S_max must be measured under both byte/operation and Worker limits below. ADR 0028 D1
  alone supplies only a text ceiling:
  as a display-size limit a peer fragment is capped at the 512 Ki "Grand" ceiling
  (`0028-performance-huge-files-and-wan.md:129`, target ≤ 100 ms of freeze), after it at 4 Mi, off
  the display thread. → F-C1: the D1 growth cap **exists** (`frontend/src/net/coverage.ts:311-330`, commit `7972fdd`);
  v5 said it was missing.
- → F-C2 (cap per change; the local factory splits) and F-A2/M5 (history pages). **History/operation bound independent of text growth.** `coverage.ts:311,413` text-growth
  accounting is insufficient. Before WASM decode, bound decompressed change bytes to 16 MiB per
  job and counted operations to 65,536 per job (streaming parser, stop at +1); compressed bytes
  remain ≤ MaxFrame. Bound retained job state to this allocation, including a million insert/delete
  or overwrite ops with zero net text growth. Large honest histories use paginated application
  bundles prepared by an upgraded sender, not splitting an existing sync message blindly; until
  that protocol exists, oversized sync is refused with a recoverable explicit error and remains
  Critique for availability (§9), not a claim of transparent support. Worker linear memory is
  capped at 256 MiB; kill/join on overflow/deadline before releasing the permit. Each accepted
  chunk is persisted before the next; per-id saves serialized. These are per-job bounds, never
  lifetime rejection of a writer's history: N9 D-3 still means detect/alert at 256 MiB/day,
  no automatic suspension or cumulative quota. Cumulative disk remains an explicit residual.
- **Overflow** → reset that peer with a cooldown of 30 s doubling to 10 min, kept per member device
  across reconnects (map ≤ member devices, pruned when the device leaves the list). The reset is not
  "fresh": `endSync` keeps `#syncStates` (`DocSynchronizer.js:217-222`); retention ≤ lifetime peers ×
  admitted.
- **Rejected alternatives**: a per-peer token bucket at 16 MiB/s (v3; not work-conserving, and does
  nothing for an indivisible apply); weighted fair queuing (O(log n), no gain at a few peers);
  splitting sync messages (impossible: one bundle); a two-Repo split, open documents vs the rest
  (strong isolation, but a second Repo and document migration; kept as the escalation if test B11
  fails after the Worker).

**Peak RSS (S1-R5).** Webview: P × (MaxFrame × k_dec + 256 MiB Worker allocation) +
256 MiB resident-document Worker + members × (2 MiB credit + 1 MiB slice) + baseline.
Use one resident Worker and at most P transient processing Workers; small jobs share these slots
through the interactive scheduler. No additional Worker per queued peer. k_dec (JS copies/decode
outside Worker memory) is measured by B12, not assumed; include Worker process overhead in baseline; Go: members × (4 MiB reassembly +
8 MiB small queue) + P × MaxFrame (read back from disk for delivery). Every term is either per peer
(attributable) or shared by fair turns.

### 5.3 Residency with suspension, save drain and lifecycle patch (review 4, point 9)

v3 adopted a pnpm patch (P1) so that `removeFromCache` would evict non-ready handles. Review 3 showed
that a patch limited to `removeFromCache` is insufficient: `removeFromCache` waits outside
loading/requesting (`Repo.js:721`), `flush` throws on a non-ready handle (`Repo.js:706`), and a late
load re-registers the synchronizer (`Repo.js:499`). An exact patch would need four hunks (abortable
storage wait, identity check after every `await` in `#loadDocumentWithProgress`, non-ready branch in
`removeFromCache`, ready filter in `flush`) and a lifecycle contract test per upstream release.

**Alternatives.** (R-b, v4) wait for natural one-second idle and keep non-ready handles:
continuous writes prevent eviction and revoked handles accumulate. Rejected. (R-a, v5) explicit
lifecycle cancellation plus save drain: selected. Worker recycling alone isolates memory but cannot
prove persistence without the same drain. A maintained minimal automerge-repo patch is now required;
no "no patch" claim. Required integration points (contract tests against pinned 2.5.6):
- Every handle has a generation and a lifecycle lease. On eviction/removal suspend **all** inputs
  (network, local edits, projection/import, loader) for that id; mark evicting synchronously before
  awaiting anything. New input stays in its peer's bounded queue or is retried after reload; do not
  reopen until teardown finishes. Select the LRU even under continuous writes.
- Cancel throttled save callbacks, await every active save/compaction, flush the final frozen heads,
  then await a per-id storage drain barrier. Snapshot saves/removes are FIFO for that id; no stale
  callback can compact newer chunks after the barrier. Save failure retains the dirty handle and
  reports failure; no silent unload. Memory overflow suspends admission of more work in that lane
  or recycles its Worker after durable recovery, never discards unsaved edits.
- Patch `#loadDocumentWithProgress` after every await (including Repo.js:499 registration) to check
  generation/identity/admission; a cancelled load never registers a synchronizer or drains messages.
  Patch `removeFromCache` to terminate non-ready handles without waiting/flush; `flush` ready-only;
  unregister synchronizers, timers/listeners and bookkeeping for that generation. Confirm release
  only when storage, callbacks and save work are drained or cancelled. Late storage responses are
  ignored by request generation. Ready handles follow the same teardown, with final flush first.
- Non-open resident ready docs: ≤16 and calibrated Σ numOps budget; reserve room for one bounded
  incoming job before processing it. Worker 256 MiB ceiling plus termination bounds transient
  memory; no claim that 16 arbitrary documents bounds bytes. Dirty-save failures are bounded by
  the same Worker allocation and an explicit paused error state.
- Non-ready handles: one per active lane lease, including **cancel-pending removed ids**, ≤ W+2;
  one retired epoch may remain only until confirmed teardown; do not create a replacement Repo or
  reuse its lane resources before release. Metadata for removed ids is purged after drain; durable
  listing/proof retention has the separate lifetime bounds of §2.2. B8 tests thousands of removed
  ids while storage is stuck: one abandoned handle per lane, no repeated allocation.

An unavailable/failed find must tear down its non-ready handle before returning its lane lease;
otherwise completed-but-unavailable finds would accumulate. Retry with per-id backoff in the same
bounded lane, triggered by a holder announcement or user demand, not a tight polling loop.

Suspension creates quiescence despite continuous arrivals; it is not an idle heuristic. After drain,
reload resumes in the fair scheduler if still admitted; otherwise only an explicit restoration can
re-admit. A 60 s invisible timeout initiates cancellation, never promises completion of stuck I/O.

### 5.4 Epochs, refusals

Every delayed callback (loader continuation, hold drain, `Listed`, `AttestRefused`, cooldown timer,
re-arrive pending slot, `Done`, purge notices) carries the epoch at scheduling time and does nothing
if it changed. Refusal notices already in transit when the project changes are dropped by the same
check (review 3, point 6). Test A13.

### 5.5 Sessions: exactly one adapter per Repo (review 3, point 8)

Session keys in the Repo are ≤ lifetime peers × adapter instances. Enforcement by **object
identity**, not peer id: a module-level `WeakMap<Repo, WailsNetworkAdapter>`; `connect` throws if the
Repo already has a different adapter, and the workspace asserts `repo.networkSubsystem.adapters.length
=== 1` after construction. Test A12 builds two adapters on one Repo (second throws) and two Repos with
the same peer id (both allowed).

### 5.6 Peer table bounded by membership (S1-R6)

Stage 1 keeps ≤ 1 024 peer entries and evicts only fully refilled ones
(`frontend/src/net/admission.ts:106,127-148`), so 1 024 strangers admitted during an invitation
window (`internal/transport/p2p/node.go:476`) lock out a new member for ≈ 33 min.
- **Go (router)**: forward `Data`, `PeerUp` and `PeerDown` to the webview only for peers whose
  device is a current member in the `trust` fold; the invitation lobby uses the ADR 0032 channel,
  not the repo channel (to verify in step 2.0, test A14).
- **Removal**: member-set updates explicitly disconnect the removed peer in Repo and cancel its
  queued/active work; do not depend on a PeerDown that the router now filters out. Keep only its
  cancellation lease until confirmed release.
- **Webview**: `Admission` creates entries only for member devices (Go sends the member device set
  with the epoch); an entry and its cooldown are removed when the device leaves the list. Table size
  ≤ member devices; no eviction rule needed. Member devices are added by admins only (attributable,
  ≤ the 1 024-op list).
- Alternative rejected: reserve the 1 024 entries by class (members never evicted by strangers).
  It keeps a capacity number that has no meaning and still lets strangers hold state.

## 6. Numeric bounds (Stages 2-3)

| State | Bound | Basis | Test |
|---|---|---|---|
| Ids a writer W can ever get admitted from its own chain | ≤ Q*_W (+1 transiently, before the proof arrives) | positions, rule E | A2 |
| Ids kept alive by adoption | ≤ 1 per (adopter, author, slot); per adopter ≤ Σ_{v≠adopter} Q*_v | L5 adopt | A3, B6 |
| Admitted ids | ≤ 2 + Σ_w Q*_w + adoptions + `kept` | §2.4 | A2-A7 |
| Listing ops retained | per author: variants + adoptions below freeze + 2 proof ops; ≤8.7 KiB each | rule E | A2 |
| Listing pending buffer | ≤ 4 per standing author, evicting only that author's | L2 | B5 |
| Repo bookkeeping per id (`#hasRequested`, synchronizer, sync states) | active + cancel-pending generations; sync states ≤ lifetime peers × lifetime listed ids | only admitted ids reach the Repo | Stage 1 test 3 |
| Non-ready handles | ≤ W+2 active/cancel-pending lane leases (≈66 KiB each before payload) | §5.1 | B8 |
| Loads in flight | ≤1 underlying request per lane until Released; UI wait ≤10 s; lanes ≤W+2 | §5.1 | B10 |
| Resident non-open documents | interim: ≤ 16 and Σ numOps ≤ calibrated budget; with the Worker: WASM budget + recycling | §5.3 | B9 |
| Hold (Stage 1) | 4 per (peer, doc); 8 MiB + 1 frame per peer; no shared pool | Stage 1 | B3, B4 |
| Go reassembly | 4 MiB in memory per member peer; above, one temp file ≤ MaxFrame per peer | §5.2 | B13 |
| Go router queue | ≤ 8 MiB small + 1 large (on disk) per peer | §5.2 | B14 |
| Webview queue | ≤ 2 MiB credit + 1 large (permit) per peer | §5.2 | B13 |
| Large frames in delivery/processing | P=2 live allocations, ≤1/peer; 30 s initiates Cancel, reuse only after Released | §5.2 | B12 |
| Indivisible apply S_max | per-job bytes/ops plus Worker 256 MiB ceiling; deadline/termination measured; text cap alone insufficient | §5.2, ADR 0028 D2 | B11 |
| Scheduler | DRR, quantum 8 ms of measured cost; interactive first, background ≥ 1 in 4 | §5.2 | B11 |
| Peak RSS over baseline | webview P × (MaxFrame × k_dec + 256 MiB) + 256 MiB resident Worker + members × 3 MiB; Go members × 12 MiB + P × MaxFrame | §5.2 | B12 |
| Ephemeral | 64/s, burst 128, 1 KiB, schema | Stage 1 | B1 |
| Re-arrive | 3, then 1 per 20 s, one pending | Stage 1 | B2 |
| Session keys | ≤ lifetime peers (one adapter per Repo, by identity) | §5.5 | A12 |
| Peer table, cooldown map | ≤ member devices | §5.6 | A14 |
| Sync-state files | 0 | Stage 1 | Stage 1 test 6 |
| Go attestations | per signer ≤ admitted; total ≤ writers × admitted × 4 KiB | S3 | B7 |
| Purge acknowledgements | ≤ 1 per admin device (latest wins, monotone) | §4 | B15 |

## 7. Stages, what Stage 2 must add, and closure of the residuals

### 7.1 What Stage 2 (ADR 0030) must add for N7 (review 3, point 3)

ADR 0030 as written admits a writer publishing two contents for one `(actor, seq)`, **including the
actor of another member**, which splits another person's document
(`0030-decisions-signees-garde-structurelle.md:333-341`). Stage 2 must add:
1. → superseded by F-S1 / M1 (ChangeSig). **G12, verifiable actor ↔ key from seq=1.** A first-seen binding is insufficient: W can
   re-encode its first change with H's actor (review 4 Node probe). Each actor session carries in
   its first change's canonical `message` a descriptor `{v:1, project, docNamespace, epoch}`.
   `author = hex(Ed25519 public key)` (32 bytes, not an ambiguous display fingerprint);
   `baseActor = BLAKE3("inkwell/actor/v1" || ProjectID || authorBytes || docNamespace || epoch)[:16]`.
   Fields are fixed-size 32/32/32/16 bytes with domain separation; epoch is a fresh random 128-bit
   nonce persisted before use, never reused for a restarted writing session. `docNamespace` is a
   preallocated random 32-byte creation namespace committed in the anchor's message; root uses
   ProjectID, index uses its deterministic creation context. This avoids circular dependence on
   DocId (which includes anchor hash). Copies/forks preserve original changes' namespaces and
   descriptors; their new changes use the new anchor's namespace. Merges keep those bindings,
   and G11 limits the accepted histories; do not reinterpret copied changes under the target id.
   Guard checks canonical descriptor, project, author key/membership/coverage AuthorshipSig (not relay T10), and the separate AuthorshipSig (§2.2), then
   recomputes baseActor on **every seq=1** before recording anything. Later seq require that exact
   descriptor/binding in their causal past (G3). `changeAt` generates a prefixed actor (09); native
   changeAt is disabled for v5 writes: implement edit-at-version via view → fresh session clone.
   No arbitrary prefixed actor exception. W changing author to W cannot match H's actor; W claiming
   author H cannot supply H's signature. Two concurrent genèses must pass these checks independently.
   T10 relays remain coverage, never contribution receipts or AuthorshipSig.
   G12 waits for the real authorship sidecar; relay-signed heads cannot satisfy it.

   **Proven writable load path (Automerge 3.5.0).** `load(bytes,{author})` ignores author without
   patchCallback (AM:496–511); experiments also show loadIncremental may reset the actor/author.
   Moreover setting author can randomise the actor. Use, in order:
   `raw = A.load(bytes); authored = A.clone(raw,{author}); writable = A.clone(authored,{actor:baseActor})`.
   For create use `A.clone(A.init({author}),{actor:baseActor})`; for view/fork use the same two clone
   steps. Free temporary documents. Only the final writable is installed in the handle, never raw.
   Patch Repo create/clone/import and storage hydration/DocHandle replacement to call this factory;
   a wrapper in application code alone does not cover Repo's internal loading. Before publishing
   each local delta assert decoded actor, author, descriptor and G12; fail closed on mismatch.
   Probe run 2026-10-31, Node 22.23.1, installed 3.5.0, from `frontend`:
   ```js
   import * as A from "@automerge/automerge";
   const actor="ab".repeat(16), author="cd".repeat(32);
   const bytes=A.save(A.change(A.init(), d=>{d.x=1}));
   const bad=A.change(A.load(bytes,{actor,author}),d=>{d.x=2});
   const raw=A.load(bytes), authored=A.clone(raw,{author});
   const good=A.change(A.clone(authored,{actor}),d=>{d.x=2});
   for (const d of [bad,good]) {
     const c=A.decodeChange(A.getLastLocalChange(d));
     console.log(c.actor,c.author,c.seq);
   }
   ```
   Output: bad actor `ab`×16, author undefined, seq 1; good same actor, author `cd`×32, seq 1.
   This proves the factory path, not integration with Repo; A15 must cover reload/import/fork and
   the next delta in real Repo before activation.

   **Deterministic genesis exception (ADR 0030 §3, :182).** Only the byte-for-byte locally
   recomputed index G (fixed project-derived a0, seq/startOp=1, time=0, deps=[], exact schema and
   RootURL) is exempt from author/T10/actor derivation. Check hash and full canonical bytes;
   never allow another unsigned change under a0, an empty-author descendant or a rival genesis.
   This exception is separate from legacy history and consumes no contribution/adoption evidence.
   Alternatives: first-seen actor→author mapping (v4) accepts both rival genèses and is rejected;
   actor=hash(key,doc) without epoch reuses seq after reload and is rejected; hash with persisted
   fresh session nonce and seq=1 descriptor is selected. Author-signature coverage is still required:
   derivation alone authenticates no bytes.
2. **Legacy history.** Changes without `author` are admitted only in `past(L_d)`, L_d being the heads
   of document d recorded once, at upgrade, in an admin decision `legacy/<d>` (pre-1.0 "version
   commune" clause of ADR 0030). User decision D-6.
3. **Joint causal past, post-erasure absorbing certificate, acknowledgements, two-phase erasure** (§4), for `doc/` and
   `node/` entries and for ADR 0023 blob GC.
4. **Fair scheduler and Go delivery** (§5.2), as the single mechanism behind ADR 0030 §5, and the
   ADR 0028 D1 growth cap it relies on.
5. **Member-gated peer table** (§5.6, S1-R6).

→ F-S5 / M9 and F.2-R1 (admin arbitration). **What stays open after Stage 2 and 3 — and stays Critique:** W equivocating on **its own**
`(actor, seq)` splits any document W writes in, H's included. Without consensus no guard can choose
deterministically (Kleppmann 2022, §2.3.1). With G12 the proof is non-repudiable (two attestations
signed by W); honest peers stop relaying W's changes after the proof (monotone, affects only W); the
UI names W; an admin removes W; H recovers by copying the document at a version before the proof into
a new variant. Stages 1→2→3 are a mitigation authorised by N8, **not** a Byzantine closure of this
case (§9).

### 7.2 Residual → closing step

| Residual (grade) | Closed by | How |
|---|---|---|
| S1-R1 writer panic, divergence, unreloadable document (Critique) | Stage 2, steps 2.2-2.3 | G1-G10 + containment; G12 for foreign actors; self-equivocation stays Critique (§7.1) |
| S1-R2 slot exhaustion, rows hidden by deletion (Critique) | Stage 3, steps 3.1-3.6 | per-writer chains, no shared slots, fallback rows, adoption |
| S1-R3 arbitrary or foreign ids (Majeur) | Stage 3, steps 3.1, 3.4-3.5 | derived ids, full signed tuple, G11 |
| S1-R4 residency and bytes (Critique) | Stage 3 activation gate | suspend/drain eviction + cancelled-generation teardown + Worker ceiling |
| S1-R5 peak RSS on large frames (Mineur until measured) | Stage 2, step 2.5 | per-peer reassembly + spill, router queues, fair permits; measured by B12 |
| S1-R6 sybil ids fill the peer table (Critique) | Stage 2, step 2.0 | member-gated router and peer table |

### 7.3 Ordered implementation plan

→ superseded by F.3.

Stage 2 (dependencies before consumers):
- 2.0 Membership-gated router/peer removal (A14), explicit Repo disconnect even when the device
  is no longer eligible for forwarded PeerDown; cancel queues and start release protocol.
- 2.1 `internal/session` → policy journal, v2 Seen/joint past; implement `legacy/<d>` signed
  decisions and capture/fetch legacy heads; purge recovery/barrier/certificates (A9, B15).
- 2.2 G1-G10 containment; history/operation parser limits, Worker termination, permit Cancel/Released
  and scheduler/store lane workers + pagination (B10-B14). Neither text growth nor JS timeout is
  sufficient. Keep feature inactive until all dependency contracts pass.
- 2.3 G12 factory + exact deterministic G exception + coverage by key; enable only after 2.1 legacy
  snapshots are present and 2.2 Worker/cancellation is operational (A15).

Stage 3:
- 3.1 listing variant/adopt decoder, issuance-L5 vs standing-L5, lifetime bounds, SeenListing.
- 3.2 bridge and contribution receipts, save-drain/generation patch, loader lane leases and tests;
  compute new admitted set in shadow mode; legacy meta.variants remains authoritative meanwhile.
- 3.3 **One activation gate** for derived-id creation, new admission, G11, contribution persistence
  and automatic adoption. Before switching, admin legacy decisions/listing migration cover every
  existing variant and any co-edit requiring adoption; peers negotiate support, reject a mixed
  write session explicitly (older peers may stay read-only through a compatibility adapter).
  Never create an id its receivers cannot admit or retire rows before adoption can preserve them.
- 3.4 Retire meta.variants authority only after successful activation and convergent migration;
  keep presentation/fallback rows. Residency hard bounds and H-1 pagination tests are prerequisites,
  not post-activation repairs. Run A11 reconnect benchmark at final gate.

Every merge, including inactive preparatory changes: compatibility tests plus **ZZ-2** (and ZZ-1).
No partial merge activates a dependent feature. Stage numbers in older review tables are historical.

## 8. Tests

### 8.1 Scenario tests (Stage 2 additions and Stage 3)

- **A1** `listing`: golden vectors; `FuzzDecode`; L1-L5; `DocId` vectors shared with TS.
- **A2** Equivocation flood across restarts: a writer forks its chain at every position, 10 000 ops,
  3 restarts, 3 replicas, random orders → retained ops ≤ chain + 2, admitted own ids ≤ Q_w, freeze
  point identical everywhere, proof gossiped.
- **A3** Equivocation after honest contribution (review 3, point 1): W lists d (position 2); H edits
  d (local contribution receipt and persisted adoption); W sends a conflicting position-2 op to R3 and a conflicting position-1 op to R2 →
  after convergence, on all 3 replicas, d is admitted (adopted by H), H's changes are present, W is
  frozen at 1, W's documents nobody co-edited are "à confirmer" on holders, H's quota is unchanged,
  H signed exactly one adoption; a second adoption of the same key is refused.
- **A4** Cross-writer aliasing: no W op derives H's `DocId` (property over random tuples); W's
  equivocation never changes H's ids.
- **A5** Creation fairness: W uses its 32 positions and floods invalid ops; H creates 5 variants
  concurrently → all admitted everywhere; W deletes H's `meta.variants` rows → rows still shown.
- **A6** Revocation: W removed; W's op naming an old frontier after the cut → dead everywhere, in
  all orders; an op the remover had seen (`SeenListing`) → standing; suspension then let-back →
  later ops standing; a dead document co-edited by H → adopted; one nobody edited → "à confirmer",
  re-admitted by `doc/<id> = kept`.
- **A7** Quota frontier (review 3, point 12): 33rd variant signed before the grant → dead even after
  the grant arrives; signed after it (grant in `Decisions`) → standing; the granting admin cut
  concurrently → positions 33+ dead on every replica, the co-edited one adopted; grant seen by the cut
  → standing; let-back of the admin concurrent with the listing; 3 replicas × 100 random delivery
  orders → identical admitted sets.
- **A8** G11 (review 3, point 5): ancestors without the anchor → waiting, then dropped at eviction,
  then accepted when resent with it; anchor with a dep missing from the batch → refused (Merkle walk);
  rival root genesis → refused; rival parent → descendant refused transitively; batch mixing the
  lineage and a rival branch → refused entirely; descendant on another member's actor → refused with
  G12, accepted without it (documents the ADR 0030 residual); 3 replicas, all orders.
- **A9** Purge (review 3, point 13): (a) rogue admin purges, a concurrent cut arrives after the
  tombstone → the document returns with its bytes, never erased; (b) concurrent `kept` → returns;
  (c) sequential `kept` on day 29, with all Acks on day 1 → restores identical heads/bytes; old timers never erase; after certificate → refused; (d) restart after the tombstone, after
  stability, and between the erased-list append and the removal → no premature erasure, removal
  idempotent; (e) a save queued before erasure completes; after erasure a late save is refused and the
  directory is not recreated; (f) an absent admin device blocks erasure; removing it before prepare unblocks; coordinator failure during prepare keeps locks until outcome recovery;
  (g) an admin promoted after all acknowledgements cuts the purger with a stale `Seen` → purge stands
  (joint past); (h) replayed listing op or adoption → no re-admission; (i) blob GC follows the same
  phases.
- **A10** Loader (review 3, point 2): 40 hostile lanes with storage `load` blocked → H's lane starts
  its underlying storage dispatch within one turn; 100 deadlines do not release the lease or
  allocate a second request. Admission removed while blocked, then late response: zero registration,
  delivery or new save, Released permits retry only for a still-admitted id.
- **A11** Reconnect benchmark: real `Repo` over simnet with coverage, guard and transport, multi-actor
  random edits, 1 000/10 000/100 000 changes, ≥ 10 runs, median and IQR, `isEphemeral` vs persisted;
  keep `isEphemeral` unless the p50 reconnect of a 100 000-change document under the ADR 0028 WAN
  profile exceeds 1 s.
- **A12** One adapter per Repo by identity (§5.5).
- **A13** Epochs: every delayed callback of §5.4, and refusal notices in transit, with a project switch
  in flight.
- **A14** S1-R6: 2 000 stranger ids during an open invitation → 0 webview entries; a new member is
  admitted at once; removing a device frees its entry/cooldown, explicitly disconnects it from Repo and cancels/drains its work despite filtered PeerDown; late resume delivers nothing; the invitation still completes.
- **A15** G12: two concurrent seq=1 genèses, one W change re-encoded under H actor,
  delivered in both orders: W refused, H accepted; repeat create/load/import/clone and next edit
  through real Repo factory; W forges H actor+author and H relays/attests it →
  still refused without H AuthorshipSig; relayed-only heads never issue that signature; exact deterministic G accepted, modified G or unsigned descendant refused; W's self-equivocation
  produces a proof with two W attestations, W's later changes are no longer relayed, the split is
  documented (residual).

### 8.2 One test per bound: threshold, overflow, recovery (review 3, point 10)

| # | Bound | At the threshold | One past it | Recovery |
|---|---|---|---|---|
| B1 | ephemeral bucket 128, 64/s | 128 in one tick delivered | 129th dropped | +1 s → 64 more delivered; PeerDown resets to 128 |
| B2 | re-arrive 3, 1 per 20 s, 1 pending | 3 accepted | 4th pending, 5th replaces it | at +20 s the pending one is processed once |
| B3 | hold 4 per (peer, doc) | 4 held | 5th → that peer's queue dropped, cooldown 30 s; another peer's held messages intact | after 30 s the handshake is accepted and messages drain |
| B4 | hold 8 MiB per peer (+1 frame when empty) | 8 MiB held; one 64 MiB frame alone held | +1 B → reset | cooldown 30, 60 … 600 s (cap), kept across reconnect, pruned when the device leaves |
| B5 | listing buffer 4 per author | 4 waiting | 5th evicts that author's oldest; other authors untouched | when `Prev` arrives, drained in order |
| B6 | adoption 1 per (adopter, author, slot) | first adoption stands | second, same key → refused | — |
| B7 | attestations per signer ≤ admitted | stored | non-admitted doc → `AttestRefused` | retried and accepted once the doc is admitted |
| B8 | non-ready active + abandoned handles ≤W+2 | 300 absent ids, W lanes → ≤W+2 active/cancel-pending handles, measured RSS per lease | 50 000 invented ids → 0 handles | remove admission, repeat 1,000 ids then unblock: no stale registration; Released permits next queued id |
| B9 | resident non-open: 16, op budget | 16 resident | 17th → suspend LRU inputs, drain saves, evict; continuous writes are suspended and drained before eviction; open documents have a separate Worker budget | evicted, reloaded, changed, wait 2 × debounce, reload from storage → every change present; RSS per ready handle with synchronizer and 1-3 peers' sync states measured |
| B10 | loader: 1 in flight per lane, 10 s wait | 1 in flight | a 2nd id of the same lane waits | 100 repeated deadlines leave one underlying request/handle per lane; remove admission while blocked; H dispatches in one turn; unblock W late → no registration/delivery/save, Released then permits retry |
| B11 | DRR quantum 8 ms, classes, S_max | 2 backlogged peers: shares 45-55 % over 10 s (real clock) | a 400 ms apply → that peer skipped until repaid | lone peer gets 100 %; interactive p99 ≤ 2 × baseline + S_max with 3 hostile peers sending small frames continuously and large frames |
| B12 | permits P = 2, 1 per peer, 30 s | 2 peers served | 3rd waits; FIFO; one peer never holds 2 | at PeerDown/30 s mark cancelling; guard deliberately suspended: no third allocation until Released after Worker join; late Done ignored; peak RSS ≤ §5.2 formula, including processing/resident Workers, back to baseline + 10 % within 30 s after confirmed release, not merely expiry |
| B13 | Go reassembly 4 MiB / spill; credit 2 MiB | 4 MiB in memory | 5 MiB → temp file; fast hostile and slow honest senders both complete; Go never sends beyond credit (spy) | temp file removed on completion, drop, PeerDown, 30 s TTL |
| B14 | router queue 8 MiB small | 8 MiB queued | +1 B → that peer reset only | cooldown shared with the webview map |
| B15 | acknowledgements | `Ack` counted when its frontiers are held | an older `Ack` → ignored; one naming unheld ops → not counted | counted when they arrive |
| B16 | handle timeout 60 s; unload of invisible documents 60 s (ADR 0028 D9) | at 59 s still loaded | at 61 s teardown initiated, unload after drain; non-ready cancel-pending retains its lane until Released | late completion ignored after removal; reopen only after teardown if admitted |
| B17 | reconnect cost | = A11 | — | — |

## 9. Residual risks and explicit activation blockers

→ superseded by F.2.

| Risk | Grade | Note |
|---|---|---|
| W's own `(actor, seq)` equivocation splits documents W writes in, H's included | **Critique** (review 3, point 3) | no deterministic rule without consensus; proof, no relay, removal, recovery by copy (§7.1); an ADR must decide whether to accept it or add a stabilisation step |
| H-1: a document stored above 64 MiB cannot load (`LoadRange` in one frame, `storestream.go:293`) | hypothesis; Critique if demonstrated (data unavailable) | pagination now required before activation (§5.1/7.3); probe >64 MiB and reload |
| Cumulative history/disk growth by churn | **Critique**, ASVS V2.4.1 residual under N9 D-3 | alerts do not bound disk; per-job ops/Worker bounds do not solve lifetime disk exhaustion; no Byzantine closure claim |
| Oversized history sync until paginated sender protocol and Repo factory integration pass | **Critique**, availability/ADR 0030 | explicit activation blocker, B11/B12/A15; not solved by text caps |
| Indivisible apply S_max | Mineur only if bounded B11 passes; Critique if no enforceable bound (ASVS V2.4.1) | escalation: two Repos (§5.2) |
| Pre-upgrade co-edits not yet migrated | Critique until activation gate | migration/adoption before retiring legacy authority |
| Erasure blocked by absent admin or prepared coordinator failure | Mineur (disk/recovery availability) | remove an absent device before prepare; prepared coordinator requires durable outcome recovery; no unsafe timer unlock |
| Malicious standing admin annuls or erases | not a finding under N7 (admin power, ADR 0017) | UI "contenu effacé" |
| Attestation/adoption memory quadratic in writers | Mineur until measured; Majeur only on measured budget overrun | measure with ADR 0028 endurance before raising Q |
| Quota 32 is a ratified initial default | — | D-1, benchmark before changing |

## 10. Where to enforce (boundary alternatives, amended in v5)

| Option | P1 | P2 | P3 | backpressure | R1 | §4 shapes | ADR 0011 | Maintenance |
|---|---|---|---|---|---|---|---|---|
| A. Repo `shareConfig` only (candidate fix) | no | no | no | no | breaks | no | ok | none |
| B. Go parses the cbor-x repo wrapper and filters | partial | yes | yes | yes | needs the set anyway | duplicated | **violates** | Go follows automerge-repo's wire schema |
| C. pnpm patch of automerge-repo alone | yes | yes | no | if patched | no | no | ok | patch per upgrade |
| D (v1). boundary + session rewrite + storageId pinned | yes | yes | GC races pending saves | no | yes | yes | ok | none |
| E (Codex 1). boundary + `isEphemeral` + small patch | yes | yes | by construction | with patch | yes | yes | ok | patch |
| F (v3). E with patch P1 only | yes | yes | by construction | in the adapter | yes | yes | ok | patch insufficient as described (review 3, point 9) |
| G (v4, rejected): boundary (shapes, membership, ephemeral rewrite, hold, scheduler) + `isEphemeral` + **no patch** (ready-only eviction, non-ready handles bounded by the admitted set) | yes | yes | by construction | in the adapter and Go | yes | yes | ok | insufficient under continuous writes |

**H (v5, chosen)**: boundary + isEphemeral + lifecycle/factory patch; suspended-input save drain,
confirmed cancellation, Worker memory bound and real store lanes. Tests B8–B16/A15 are activation
gates; no blanket Byzantine guarantee before those pass.

Reconnect cost of `isEphemeral` (09, single-actor append workload, indicative only): +1.25 B and
≈ +2 µs per change per document per reconnect. Persisted state is lossy anyway in 2.5.6 (one
throttle per storageId cancels other documents' saves, `throttle.js:80-104`). Re-checked by A11.

## 11. Ratified user decisions (N9); ADR acceptance still pending

D-1: base quota 32 variants/writer (grants in steps of 32). D-2: recoverable for 30 days minimum,
then erase only after every standing admin device acknowledges and the barrier completes; no maximum
when one is absent. D-3: detect history growth and alert admins, no automatic cumulative blocking.
D-4: irreversible **once erased**, not at logical purge. D-5: automatic co-editor adoption; v5
persists it at first real co-edit as the equivalent preservation mechanism. D-6: one signed admin
`legacy/<d>` decision per pre-upgrade document; no project reset. These choices are ratified, not
questions to reopen. Only formal acceptance of ADR 0031 and treatment of the explicitly Critique
self-equivocation residual need a user decision (→ F.4, which adds ADR 0030 and R2); implementation/measurement gates remain mandatory.

Sources carried from v4 (no new external-source claim): Kleppmann, *Making CRDTs Byzantine Fault Tolerant* (PaPoC 2022, §2.3.1, §3.4);
Baquero, Almeida, Shoker, [*Pure Operation-Based Replicated Data Types*](https://arxiv.org/abs/1710.04469)
(causal stability); Shreedhar and Varghese, [*Efficient Fair Queuing Using Deficit Round Robin*](https://iqua.ece.toronto.edu/baochun/ece1771f/papers/deficitrr.pdf)
(IEEE/ACM ToN 1996); [RFC 9000 §4](https://www.rfc-editor.org/rfc/rfc9000#section-4) (receiver credit);
automerge 3.5.0 `implementation.d.ts:84-85,387-390,627` (author, stats).

## Changes after review 4 (Codex, [13-codex-design-review-4.md](13-codex-design-review-4.md))

CLOSED below means **design contract**, not implementation or test execution. Audit rubric unchanged;
N7 remains full Byzantine and N9 decisions are preserved. Any implementation/measurement gate failing
keeps the associated risk open; no claim that S1 or Byzantine safety is implemented.

| Review # | v5 disposition | Normative contract and verification |
|---|---|---|
| 1 | CLOSED design | §2.2 separate contribution receipt/relay coverage; eager adoption; signed variant Slot distinct from chain Position; issuance-L5 survives later grant removal, pre-grant op still invalid; A3/A7 plus relay-only → zero receipts/adoptions |
| 2 | CLOSED design | §5.1 lease holds through underlying Released; one storage request/lane, real Go workers/pages; B10 repeated timeouts, then H, then late resume |
| 3 | CLOSED design, integration gated | §7.1 derivation recomputed at seq=1, author-key AuthorshipSig distinct from relay T10, canonical namespace/epoch; A15 two concurrent genèses W/H with re-encoded H actor in both delivery orders → W refused, H accepted |
| 4,5,6,7,8,11,12 | CLOSED retained | signed tuples, G11 ancestors, epochs/refusals, zero sync-state, adapter identity, repeated reconnect bench and causal grants retained |
| 9 | CLOSED design | §5.3 suspend every input despite continuous writes, cancel throttle, drain compaction/flush; generation patch prevents late registration; B8/B9 removal + stuck I/O + unblock |
| 10 | CLOSED design | B8/B10/B12/B16 assert beyond deadlines, admission removal, late resume and baseline recovery; 60 s is cancellation initiation for non-ready, not fictional unload |
| 13 | CLOSED design | §4 reversible until erase, ≥30 days, per-generation barrier, certificate only after removal; A9 restore day29 even after early Acks; years-offline admin blocks |
| 14 | CLOSED design, protocol gated | §5.2 Cancel/Released/Worker join before permit reuse; B12 suspended guard past30 s + PeerDown + late Done; zero-growth 65,536 ops pass, +1 rejected pre-WASM; 16 MiB decompressed threshold/+1; 256 MiB Worker kill then join; cumulative disk residual explicitly Critique |
| 15 / order | CLOSED design | §7.3 dependencies first, atomic activation, ZZ-2 each merge; §11 and ADR ratified D-1..D-6 with status proposed; quantified eager/deferred comparison §2.2 |
| Automerge blocker | CLOSED factory probe, integration unimplemented | §7.1 two-clone writable load path proven on installed version (direct load fails); real Repo contract mandatory, deterministic G exact exception; self-equivocation stays Critique |
| S1-R6 follow-up | CLOSED design | A14 explicit disconnect/cancel on member removal despite PeerDown filtering |

Additional contract checks: restart between contribution save/adoption persistence/publication (no
lost edit or duplicate adoption); quota grant removal in all A7 orders; continuous inbound/local
writes through B9 teardown (final heads survive reload); admission removal during storage await,
compaction and guard yields (no late delivery, resurrection or new save); cancellation at 30 s and
31/60/120 s keeps ≤2 live allocations; after unblocking, counters return to zero and H progresses.
All thresholds in §5 are versioned operational defaults to benchmark, not measurements already made.

## Changes after review 3 (Codex, [10-codex-design-review-3.md](10-codex-design-review-3.md))

| # | Verdict in review 3 | Amendment in v4 | Evidence / test |
|---|---|---|---|
| 1 | PARTIAL, Critique: W's equivocation makes co-edited variants disappear | `adopt` listing ops: a document stays admitted while a standing writer's chain adopts it; Go adopts automatically from `contrib` (`attest.go:124`); adopter's quota untouched; purge still wins (§2.2) | A3, B6 |
| 2 | PARTIAL, Critique: 4 loads / 32 handles shared | one lane per owner, ≤ 1 in flight per lane, no global cap; per-load deadline releases the wait; underlying loads deduplicated per id (`Repo.js:425-436,477-480`); the 32-handle cap removed: non-ready handles ≤ admitted ids (§5.1) | A10, B8, B10 |
| 3 | PARTIAL, Critique: ADR 0030 still admits `(actor, seq)` spoofing; Majeur grades unmeasured | Stage 2 must add G12 (author binding, coverage by author key), legacy decision (§7.1); W's own equivocation kept **Critique** after Stage 3 (§7.1, §9); Stage 1 residuals regraded per rubric (S1-R4 Critique, S1-R5 Mineur until measured, S1-R3 Majeur with measured impact, S1-R6 Critique) | A15; 09 `author2.mjs` |
| 4 | CLOSED | kept: full signed tuple, derived ids; aliasing tests | A4 |
| 5 | PARTIAL, Mineur: G11 for the anchor and its ancestors | nothing accepted before the anchor; batch Merkle walk from the anchor; anchor accepted iff its hash is the signed one and it passes G1-G10; rival parent refused transitively; actor usurpation refused by G12 only (§2.3) | A8 |
| 6 | CLOSED; amend: refusals in transit | epoch check covers refusal notices and `Done` (§5.4) | A13 |
| 7 | CLOSED | unchanged (sync states); document saves: ready-only quiescent eviction (§5.3) | Stage 1 test 6, B9 |
| 8 | CLOSED; amend: really prevent several adapters | `WeakMap<Repo, adapter>` identity check + `adapters.length === 1` (§5.5) | A12 |
| 9 | PARTIAL, Critique: P1 insufficient (`Repo.js:721/706/499`); stored bytes ≠ memory; disk history | patch dropped (option G); never evict non-ready handles; eviction protocol for ready ones that avoids all three lines and the compaction race (`StorageSubsystem.js:215-239`); memory budget on ops (interim) then Worker WASM memory (ADR 0028 D2/D9); disk history → D-3 and H-1 (§5.3, §9) | B8, B9 |
| 10 | PARTIAL, Mineur: a concrete test per bound | table 8.2: threshold, overflow and recovery for every bound, including the 8 ms quantum, 60 s timeouts, announcement buffer, attestation cap, and RSS of ready handles with synchronizers | B1-B17 |
| 11 | CLOSED | unchanged; repeated benchmark | A11 |
| 12 | PARTIAL, Critique: L5 on "currently standing" grants | `Decisions` frontier in every listing op; L5 counts grants in `past(Decisions(o))` that stand in the fold; re-evaluated by the pure fold; co-edited documents adopted when a grant dies (§2.5) | A7 |
| 13 | OPEN, Critique: erasure before annulment | logical tombstone, then erasure only when **stable**: every standing admin device's `Ack` contains the purge in its joint causal past, plus optional grace; post-erasure absorbing certificate; joint causal past; erased list before removal; proof of sufficiency (§4) | A9, B15 |
| 14 | PARTIAL, Critique: no preemption of large frames, unbounded scheduler queues, two peers on two permits | network time decoupled from memory (per-peer reassembly, spill to disk); per-peer bounded queues in Go and the webview; DRR on measured cost; permits FIFO, ≤ 1 per peer, held only for local processing, 30 s deadline; S_max bounded by ADR 0028 D1-D2; peak RSS formula measured (§5.2) | B11-B14 |
| 15 | PARTIAL, Mineur: ADR proposed; exact patch; compare anchor-less ids | ADR 0031 updated, still proposed; no patch to specify; option (f) anchor-less id compared and rejected: G11 would become order-dependent under equivocation (§2.1) | — |
| S1-R6 | sybil ids fill the 1 024-entry table | member-gated router and peer table (§5.6) | A14 |

## Changes after review 2 (Codex, [08-codex-design-review-2.md](08-codex-design-review-2.md))

Kept for history; where v4 changed the answer, the table "Changes after review 3" above prevails.

| # | Verdict in review 2 | Amendment in v3 |
|---|---|---|
| 1 | OPEN, Critique: slots do not bound lifetime ids under equivocation; evidence retention | ids derived from (project, writer, kind, position, anchor); per-writer linear chain; rule E freezes the chain at the first equivocation; retention ≤ Q_w + 1 ops; lifetime ids ≤ Q_w + 1 (§2.2); test 2 across restarts |
| 2 | PARTIAL, Critique: holding unknown documents blocks `Repo.find` | bootstrap `find` on the first held message, drain at ready/requesting/unavailable, recheck after every `await`; no speculative pull (§5.1); test 8 (Stage 1 test 7 too) |
| 3 | PARTIAL, Critique: grade Stage 1 residual | S1-R1 graded Critique; S1 cannot be declared closed before Stage 2 (Stage 1 section, §7) |
| 4 | PARTIAL, Critique: cross-writer aliasing | writer key inside the derived id: aliasing impossible; test 3 |
| 5 | PARTIAL, Critique: provenance | project-derived ids, full canonical tuple signed, imports copied into new listed ids, G11 anchor against squatting (§2.2-2.3); test 6 |
| 6 | CLOSED; amend: epoch checks on delayed callbacks | §5.4; test 13 |
| 7 | CLOSED; amend: keep delayed-save contract test, cleanup before attach | kept in Stage 1 tests 1 and 6 |
| 8 | CLOSED; amend: lifetime peers × adapter instances | one adapter per Repo enforced by assertion (§5.5) |
| 9 | PARTIAL, Critique: residency (`removeFromCache` on non-ready handles; bytes) | measured (09: throws, stays cached, ≈ 66 KiB RSS each); patch P1 adopted; unconditional LRU with count and **stored-bytes** budgets; ≤ 32 non-ready handles (§5.3); test 9 |
| 10 | PARTIAL, Mineur: missing tests | equivocation flood, suspended-writer ops, bootstrap, unavailable eviction, multi-peer fairness, RSS on full handles: tests 2, 5, 8, 9, 10, 11 |
| 11 | PARTIAL, Mineur: reconnect numbers workload-specific | labelled indicative (09); repeated real-Repo benchmark with a keep/revert threshold (test 12) |
| 12 | OPEN, Critique: EverWriter accepts post-cut issuance | causal standing with `SeenListing` on cut/suspension/leave member ops, ADR 0030 D1 analogue; dead listings kept on disk, re-admittable by decision (§3); test 5 |
| 13 | OPEN, Critique: purge mapping, tombstones, replay, disk GC, quota | `doc/<DocId>` decisions (and `node` mapping through the listing tuple later); permanent tombstones; replay impossible by derivation; Go deletes and refuses writes under the prefix; positions not returned, admin grants (§4, §2.5); test 7 |
| 14 | PARTIAL, Critique: fair capacity, cooldown, processing, peak RSS | reserved per-peer capacity, round-robin scheduler shared with the guard, per-peer work budget, cooldown per peer id across reconnects, Go large-frame semaphore, measured peak RSS (§5.2); tests 10, 11 |
| 15 | CLOSED conditionally | ADR 0031 drafted (proposed); composition in `internal/app`; `node` kind reserved, not implemented; fallback history rows (§2.4) |

## Changes after review 1 (Codex, [07-codex-design-review-1.md](07-codex-design-review-1.md); v2)

Kept for history; where v3 or v4 changed the answer, the later tables above prevail.

| # | Point | Verdict | Amendment |
|---|---|---|---|
| 1 | caps bound simultaneous, not lifetime ids | **accepted** (confirmed: `CollectionSynchronizer.js:87-95`, keys never deleted) | admitted set is a grow-only signed set with per-writer numbered tokens (`N ≤ Q` ever): lifetime-bounded by construction (§2); interim per-session cap 256 (§8); churn test across restarts (test 4, 10) |
| 2 | no backpressure (hypothesis) | **confirmed** (`Repo.js:236-243`; `DocSynchronizer.js:259-266` unbounded `#pendingSyncMessages`; `:74-95` callbacks) | adapter hold queue with per-(peer, doc), per-peer and global caps, cancel on PeerDown/re-arrive, peer reset on overflow; `isEphemeral` removes storage-wait callbacks (§3.5-3.6); test 7 |
| 3 | guard deferred though writers can panic Automerge today | **accepted** | §5: coverage table admission / coverage / guard; guard is Stage 2, prerequisite of the pull replicator and of declaring S1 closed; exposure argument for merging Stage 1 first |
| 4 | one writer can occupy caps; truncation ≠ fairness | **accepted** | per-writer quotas attributable by Ed25519 signature, numbered, verified against the member list (`EverWriter`); no global truncation; equivocation rule; fairness test 10 |
| 5 | URL syntax ≠ provenance; eager fetch of anything referenced | **accepted** | listing signed over ProjectID; foreign documents never admitted; import = copy into a new listed document (§2) |
| 6 | Scope before Attest; epochs; attest marked sent without acceptance | **accepted** | `Scope` kind dropped: Go owns the set; `List` returns after the store update; `Listed`/`Notices` carry (ProjectID, epoch); `AttestRefused` clears `#attested` (§2 items 4-5); test 11 |
| 7 | GC of sync-state races delayed saves | **confirmed** (`Repo.js:254-258`, `throttle.js:80-104`; also found: the shared throttle drops other documents' saves) | no sync state is ever saved (`isEphemeral`), cleanup in Go before any Repo exists (§3.5); test 6 with delayed storage |
| 8 | constant session ids need lifetime-monotonic counts; storage loss, asymmetric reconnect | **accepted** | session random per adapter instance, counter per instance; no pinned storage id at all; tests 5 and 6 |
| 9 | tree cap ≠ document cap; TTL overstated | **accepted** | per-category budgets (§4: 2 + W·(32 + 2 048)); TTL wording corrected (≤ ~60 s for silent peers, never for talking ones; bound comes from the schema) |
| 10 | missing tests; 256 and byte limits unjustified | **accepted** | tests 4-7, 10, 12 added; sizes measured (265 B snapshot → 1 KiB limit; handshake ≈ 150 B → 1 KiB; 56–310 KiB per resident doc → 16 resident); 32 variants flagged as an assumption with a compatible way to raise it |
| 11 | simpler alternative (boundary + `isEphemeral` + small patch) | **adopted in part** | `isEphemeral` adopted (measured reconnect cost §0, §1.1); patch kept as fallback gated by contract tests, upstream PRs filed instead; comparison in §1.1 |


## Décisions de l'utilisatrice (2026-10-31)

D-1 : 32 variantes de départ par personne qui écrit. D-2 : délai de 30 jours avant effacement des données
purgées. D-3 : détecter et alerter, sans blocage automatique. D-4 : purge irréversible une fois effacée.
D-5 : adoption automatique par les co-éditeurs. D-6 : une décision admin signée par document pour
l'historique antérieur. Registre : `docs/exigences.md` N9.
