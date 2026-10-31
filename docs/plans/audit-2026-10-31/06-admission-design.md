# Design v3: admission boundary and per-writer document chains (full Byzantine model)

Author: architect (Opus), 2026-10-31. History: v1 → Codex review 1
([07-codex-design-review-1.md](07-codex-design-review-1.md)) → v2 → Codex review 2
([08-codex-design-review-2.md](08-codex-design-review-2.md)) → **v3** (this file). User decisions
N7 (full Byzantine threat model) and N8 (merge Stage 1 now; S1 stays Critique until the ADR 0030
guard) are in `docs/exigences.md`. Decision record: [ADR 0031](../../adr/0031-frontiere-admission-chaines-documents.md)
(proposed).

Evidence (repo paths): [04-security-verify-A02-A05.md](04-security-verify-A02-A05.md) (A04),
[05-review-stage1-candidate.md](05-review-stage1-candidate.md) (P1-P5, R1, ZZ-2),
[09-admission-measurements.md](09-admission-measurements.md) (scripts and outputs: reconnect cost,
presence sizes, memory per document and per unavailable handle). Library line numbers refer to
`frontend/node_modules/@automerge/automerge-repo/dist` (2.5.6) and `@automerge/automerge` 3.5.0.

Rules: ASVS 5.0 V2.4.1; `docs/engineering-principles.md` §4; CLAUDE.md "une seule source de vérité
par chose"; ADR 0002/0011 (Go sees opaque bytes); ADR 0017/0021 (seniority, suspension); ADR 0018
(coverage); ADR 0030 §2 (causal standing), §3 (nothing hidden except by decision or trash), §4-5
(guard, fairness); N7. Severity per `docs/plans/2026-10-31-architecture-audit.md`.

Stage map: **Stage 1** (frozen, being implemented) bounds readers and unattested peers and keeps
R1/ZZ-2. **Stage 2** = ADR 0030 structural guard. **Stage 3** = this file's Byzantine design (§2-§7).
Every Stage 1 residual S1-R1..R5 is closed by Stage 2 or 3 (§7).

## Stage 1 — frozen scope (to merge now; decision N8, 2026-10-31)

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

### Stage 1 residual risks (graded per rubric)

| ID | Residual | Grade | Why | Closed by |
|---|---|---|---|---|
| S1-R1 | a writer's forged change panics Automerge, splits replicas, or makes a document fail save → load (ADR 0030:23, 36-46), now also spread by the loader's pull of listed variants | **Critique** | violates ADR 0030's guard requirement; crash/data loss documented by its probes; pre-existing on `main`, not widened | Stage 2 (ADR 0030 guard) |
| S1-R2 | a writer fills the 256 interim variant slots for the session, or deletes rows so later peers do not admit them | **Critique** | under N7 a writer must never hinder others; displacement denies new variants to honest writers | Stage 3 (per-writer slots, signed) |
| S1-R3 | a writer lists arbitrary or foreign document ids, which every peer then tries to fetch | **Majeur** | bounded (≤ 256 per session, loader ≤ 4 in flight), no data loss, but wasted work and no provenance | Stage 3 |
| S1-R4 | admitted documents stay resident; bytes unbounded (a writer can list 256 large documents) | **Majeur** | measured 56-310 KiB per small document, more for large ones; same as `main` | Stage 3 residency policy |
| S1-R5 | peak RSS while decoding a frame up to MaxFrame (64 MiB) from any member | **Majeur** | transient, per frame; not measured end to end | Stage 3 measurements + optional Go per-peer byte budget |
| S1-R6 | readers and unattested peers | bounded (P1, P2, P3, P5, S4 closed) | tests 2-7 | — |

## 0. Grounding (what the installed library does)

| Where (`automerge-repo/dist`) | Behaviour | Consequence |
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

Measurements (09): reconnect with fresh state costs ≈ 1.25 B and ≈ 2 µs per change per document for
a single-actor append workload (148 B vs 12 623 B at 10 000 changes); a loaded Automerge value is
56-310 KiB RSS; an **unavailable `DocHandle` is ≈ 28 KiB heap / ≈ 66 KiB RSS**; presence snapshot
265 B.

## 1. Threat model N7 and which layer answers what

Attackers: any current member, including a **malicious invited writer**, a writer who is later
removed or suspended, and several colluding writers. Readers and unattested peers must be strictly
bounded; a writer must never hinder others (availability, visibility of their documents, their
quota, their CPU/memory share), and every effect a writer has must be **attributable** to it and
**bounded per writer**.

| Layer | Answers | Stage |
|---|---|---|
| Admission boundary (adapter) | which messages may make the Repo hold state, and how much, per peer | 1, extended in 3 |
| Coverage T10 (ADR 0018) | who wrote a change (current writer attestation) | exists |
| Structural guard G1-G10 (ADR 0030 §4) | whether a change is well formed (no panic, no divergence, save → load) | 2 |
| Anchor rule G11 (this design) | whether a change belongs to a listed document's lineage | 3 (in the guard module) |
| Document chains (`internal/listing`) | which documents belong to the project, per writer, attributable, lifetime-bounded | 3 |
| Decision journal (ADR 0030 §2) | purge, quota grants, re-listing | with ADR 0030 |

What no layer prevents, by design: a current writer editing the **content** of shared documents
(that is what writing is). It is attributable (history), reversible (versions, revert), and stopped
by removing the writer. This is not "hindering" in the N7 sense; it is stated so nobody assumes
otherwise.

## 2. The admitted set: one chain of listing ops per writer, document ids derived from the slot

### 2.1 Alternatives (the design changed again: v2 tokens → v3 chains)

| Option | Lifetime ids per writer under equivocation | Cross-writer aliasing | Provenance of content | Revocation | Cost |
|---|---|---|---|---|---|
| v2: numbered tokens naming any doc id | **unbounded** (each conflicting token at slot n admits a new id; every token retained) | **possible** (two writers list the same id) | none (any id) | EverWriter: accepts post-removal issuance | small |
| (b) id = H(project, writer, kind, n) only | 1 per slot | impossible | **squattable**: anyone can pre-create the id with rival history whose root keys win the conflict | needs causality | small |
| (c) put listings in the ADR 0030 decision journal | bounded | impossible | as (d) | causal | a writer could fill the shared 8 192-op seal and freeze admin decisions → hinders others: rejected |
| (d) actor ↔ key binding, attribute each `meta.variants` op | bounded with add-only schema | impossible | needs per-actor rules | causal | large (ADR "acteurs liés aux clés", every honest client's actors) |
| **(e) per-writer chain of listing ops; id = H(project, writer, kind, position, anchor); anchor rule G11 (chosen)** | **≤ Q_w + 1, ever** | **impossible** (writer key in the id) | **anchored** (G11) | **causal, ADR 0030 style** (§3) | moderate; reuses `trust`/`policy` patterns |

### 2.2 The listing op (`internal/listing`, Go)

Canonical binary, domain `inkwell/listing-op/v1`, Ed25519 over the **full tuple**:

| Field | Content |
|---|---|
| `Project` | ProjectID (32 B) |
| `Prev` | hash of the author's previous listing op, or empty for position 1 |
| `Members` | member-list frontier seen by the author, non-empty, ≤ 64, strictly increasing |
| `Position` | 1 + position of `Prev` (so the chain is linear and numbered) |
| `Kind` | `variant` (now); `node` reserved for the ADR 0023 index, with a 16-byte `Node` field, **not implemented until needed** |
| `Anchor` | hash of the anchor change of the new document (32 B), see §2.3 |
| `Author`, `Sig` | device key, signature |

Size ≤ 64·32 + 160 B ≈ 2.2 KiB (typical ≈ 300 B). `Decode` strict, golden vectors shared with TS,
`FuzzDecode`. **Document id** (never transmitted, always recomputed):
`DocId = BLAKE3("inkwell/doc/v1" ‖ Project ‖ Author ‖ Kind ‖ Position ‖ Anchor)[:16]`, rendered with
automerge-repo's `binaryToDocumentId`. Hence: a writer cannot name another writer's document
(aliasing, review 2 point 4), cannot name a foreign or arbitrary id (provenance, point 5), and each
position yields one id.

**Admission of an op** (pure function of the op and its causal past, like `List.integrate`):
L1 canonical and signed for the project; L2 `Prev` and `Members` known, else wait in a buffer
bounded per author (≤ 4 ops, standing writers only, as `trust`); L3 `Position` exact; L4 the author
is a writer and not suspended in the member list folded on `past(Members)` (ADR 0030 A4 analogue
with role ≥ writer); L5 `Position ≤ Q_w` (§2.5).

**Equivocation.** Two admitted ops of the same author with the same `Position` (different hashes)
form a proof. Rule E: the author's chain is **frozen at that position**: positions ≥ k of that author
are excluded everywhere; the proof (two ops) is kept and gossiped, the UI names the person, the
escalation is removal (as ADR 0030 G8). Retention: per author ≤ Q_w ops on its chain + 1 conflicting
op; ops arriving after a proof for positions ≥ k are not stored. So the ids an author can ever get
admitted on any peer are ≤ Q_w + 1 (the +1 is the conflicting branch seen by some peer before the
proof reached it), across restarts (ops are persisted), however often it equivocates.

### 2.3 Creating a document and the anchor rule (G11)

Fork flow (variant): the webview builds the copy (`A.clone(A.view(doc, at))`), then makes one
**anchor change** on it (`meta.origin = {from, by}`; its deps are the copied heads, so its hash
commits to the whole copied history). It calls the bound method `Session.List("variant", anchor)`;
Go checks the role and the quota, assigns the next position, signs, **persists, then gossips**, and
returns `DocId`. The webview then `repo.import(bytes, {docId})` (`Repo.js:656-664`) and writes the
presentation entry in `meta.variants`. An import from elsewhere follows the same flow: bytes are
copied into a **new** listed document; a foreign id is never reused.

**G11 (anchor), in the guard module (Stage 3, uses ADR 0030's ancestry index).** For a document
admitted through a listing op with anchor `a`: the anchor must be held before anything else is
accepted (G2 waits for it); then a change `c` is admitted only if `c ∈ past(a)` or `a ∈ before(c)`.
A rival history (a squatter's change concurrent with the anchor, or a fresh root-level genesis) is
refused everywhere, deterministically (function of `c`, `before(c)` and the fixed `a`). Cost: one
vector-clock comparison per change.

### 2.4 The admitted set

`Admitted = {RootDoc} ∪ {IndexG (ADR 0030)} ∪ {DocId(o) : o standing (§3), position below the
author's freeze point} − {d : purged (§4)}`.
Union of signed objects and permanent decisions: every peer converges to the same set; nothing is
truncated; one writer's ops can only add or freeze **its own** ids.

Go owns it (`internal/listing` verifies, persists `.inkwell/listing/<hash>.lop`, syncs on the trust
channel with a type byte, motif `trust.Sync`); `internal/app` composes `listing`, `trust` and
`policy` (`listing` imports `trust` only; no import of `policy`, review 2 point 15). The webview
receives the set by `Listed` notices (with `(ProjectID, epoch)`), uses it for admission, sharing and
the attestation live set. `meta.variants` is presentation only; the history panel shows a row for
**every admitted document**, falling back to "Variante de <personne> (sans nom)" when no
`meta.variants` entry names it, so deleting presentation metadata never hides content.

### 2.5 Quotas

`Q_w = 32 + 32 · grants(w)` variant positions per writer, for the project's life. `grants(w)` =
number of standing admin decisions `quota/<key>` in the ADR 0030 journal (each grants 32; journal
capped at 8 192 ops). 32 is an assumption (no usage data), recorded in `docs/open-questions.md`; the
UI refuses the 33rd variant with "limite de variantes atteinte — une personne qui administre peut en
accorder d'autres" and shows admins a grant action. Raising the base later is compatible.

## 3. Revocation: causal standing of listing ops (review 2, point 12)

Alternatives:
- (A) **Causal standing, as ADR 0030 D1 (chosen).** Member ops that cut, suspend or record the leave
  of a person y carry `SeenListing(y)`: the hash of y's chain tip that the author held (0 or 1 hash;
  2 if y has equivocated). This rides on ADR 0030's version-2 member ops (`Seen`), as a typed entry.
  A listing op o of y **stands** iff (a) L4 holds at `past(Members(o))`, and (b) for every standing
  cut, suspension or leave m of y: either m ∈ `past(Members(o))` (then (a) already decides, including
  a later let-back, ADR 0021), or o is at or before `SeenListing(m)` on y's chain (witnessed before
  the cut). Otherwise o is concurrent to the cut: **dead**. A removed writer cannot issue new
  documents by naming an old frontier (backdating), and what its remover had seen stays.
- (B) EverWriter plus quota: a removed writer may still use its unused positions; its changes are
  uncovered (attestations revoked), so the documents stay empty; harm ≤ (Q_w − used) unavailable
  ids. Cheaper (no member-op change) but contradicts N7 ("post-cut issuance accepted") and review 2.

Dead listings: their documents leave the admitted set; they are **not deleted** (others may have
edited them before learning of the cut). Holders show them as "à confirmer"; an admin decision
`doc/<id> = kept` re-admits one (ADR 0030 journal, standing rules apply). Folding is pure and
identical everywhere (`listing` reads `trust.CutsOn(key)` with `Seen`, as `policy` does).

## 4. Purge, tombstones, replay, disk GC, quota exhaustion (review 2, point 13)

- **Mapping.** A variant is addressed by its `DocId` directly: decision entry `doc/<DocId>` with
  values `purged` or `kept` (new entry type in the ADR 0030 journal; entries are typed and bounded
  there). Future `node` listings carry `Node`, and the id derivation includes it, so
  `node/<id> = purged` maps to the documents whose listing names that node: authenticated, no
  Automerge reading.
- **Tombstone.** Decisions are permanent (grow-only journal), so a purge is a tombstone. Admission
  is `standing − purged`: a replayed listing op has the same hash (no effect), and no new op can
  derive the same `DocId` (position consumed, anchor fixed), so **resurrection is impossible**.
- **Disk GC.** On a standing purge, Go deletes `repo/<DocId>/` under the store root and the store
  stream refuses any later write under that prefix (Go-side tombstone list fed by `policy`); the
  webview receives `Listed` without the id, unloads the handle (patch P1, §5.3) and drops queued
  messages. The listing op itself is kept (it numbers the chain).
- **Quota.** A purge does **not** return the position (lifetime quota); exhaustion is handled by
  grants (§2.5). A hostile writer exhausts only its own quota.

## 5. Boundary in Stage 3 (deltas over Stage 1)

### 5.1 Bootstrap and hold (review 2, point 2)
A message for an admitted id with no handle starts **one** bootstrap `find`
(`allowableStates: ["ready","requesting","unavailable"]`), ≤ 4 in flight, ≤ 32 non-ready handles at
once; messages for that id are held (Stage 1 caps) and drained, in order, as soon as the handle is in
one of those three states. After every `await` the loader rechecks: epoch unchanged, id still
admitted, peer still announced; otherwise it drops the held messages. No speculative pull: holders
push admitted documents on connection (`announce` true), so a document is only loaded when a peer
actually speaks about it.

### 5.2 Fair capacity and bounded processing (review 2, point 14)
- **Reserved, not shared**: per peer hold ≤ 8 MiB + one frame (Stage 1), no global pool.
- **Per-peer scheduler**: delivered messages go to per-peer queues drained **round robin**, one
  message per turn, yielding to the event loop every 8 ms; the same queues feed the guard
  (ADR 0030 §5, one mechanism, not two).
- **Work budget**: per peer, a bucket of payload bytes processed, 16 MiB/s, burst one MaxFrame;
  guard work units charge the same bucket. A slow bucket delays only that peer.
- **Overflow** → reset that peer with a cooldown of 30 s doubling to 10 min, **kept per peer id across
  reconnects** (map pruned when the person leaves the member list). The reset is not "fresh":
  `endSync` keeps `#syncStates` (`DocSynchronizer.js:217-222`), so sync resumes from retained shared
  heads; that retention is ≤ lifetime peers × admitted.
- **Peak RSS (S1-R5)**: Go's router (`internal/bridge/router.go`) lets at most **one frame > 1 MiB per
  peer and two globally** be in delivery to the webview at a time (semaphore on the outgoing Wails
  stream, schema-agnostic, ADR 0011 intact). The webview decodes one frame at a time (single
  thread). Target: peak RSS ≤ baseline + 2 × (MaxFrame × 2) ≈ baseline + 256 MiB, **measured**.

### 5.3 Residency (review 2, point 9)
- **Patch P1 (adopted; the gap is measured in 09)**: `removeFromCache` on a non-ready handle throws
  and keeps it. A pnpm patch (`frontend/patches/@automerge__automerge-repo@2.5.6.patch`, ≤ 15 lines
  in `dist/Repo.js`) makes it delete the handle, its progress, save function and synchronizer when
  `handle.doc()` is unavailable, without throwing. Contract test guards it; upstream PR filed;
  `pnpm install` fails if it no longer applies. Maintenance: ≈ 1 rebase per upstream release
  (2 releases in Oct 2026).
- **Policy, unconditional**: resident = open documents ∪ an LRU of non-open documents bounded by
  **count ≤ 16 and stored bytes ≤ 32 MiB** (stored size = bytes the storage adapter last saved or
  loaded for that id, tracked in `GoStorageAdapter`); eviction = `repo.flush([id])` then
  `removeFromCache`, never waiting for "synced with every holder". Non-ready handles: ≤ 32 at once,
  evicted after 60 s or when over the cap. A document evicted and spoken about again is reloaded by
  the Repo; reload churn is charged to the speaking peer's work budget.
- Bytes are bounded by the stored-bytes budget, not by the count (review 2: count alone never bounds
  bytes). The memory/stored factor is measured on full `DocHandle` + synchronizer, not on bare values.

### 5.4 Epochs, refusals (review 2, point 6 amendment)
Every delayed callback (loader continuation, hold drain, `Listed`, `AttestRefused`, cooldown timer,
re-arrive pending slot) carries the epoch at scheduling time and does nothing if it changed. Test:
switch projects while each of them is pending.

### 5.5 Sessions (review 2, point 8 amendment)
Session keys in the Repo are ≤ **lifetime peers × adapter instances**. The workspace attaches exactly
one adapter per Repo, and `WailsNetworkAdapter.connect` refuses a second `connect` for another Repo
peer id (assertion + test), so the bound is lifetime peers (members).

## 6. Numeric bounds (Stage 3)

| State | Bound | Basis |
|---|---|---|
| Ids a writer can ever get admitted | ≤ Q_w + 1, Q_w = 32 + 32·grants | chain positions + one equivocation branch |
| Admitted ids | ≤ 2 + Σ_w (Q_w + 1) | categories: root, index, variants (nodes later, own Q) |
| Listing ops retained | ≤ Σ_w (Q_w + 1) ops, ≤ 2.2 KiB each; pending buffer ≤ 4 per standing writer | E rule |
| Repo bookkeeping per id (`#hasRequested`, synchronizer, retained sync states) | ≤ admitted ids; sync states ≤ lifetime peers × admitted | only admitted ids reach the Repo |
| Resident non-open documents | ≤ 16 and ≤ 32 MiB stored bytes | §5.3 |
| Non-ready handles | ≤ 32 at once (≈ 2 MiB at 66 KiB) | measured |
| Hold | 4 per (peer, doc); 8 MiB + 1 frame per peer; no shared pool | Stage 1 |
| Work | 16 MiB/s per peer, burst 1 frame; round robin | §5.2 |
| Large frames in delivery | 1 per peer, 2 globally | §5.2 |
| Peak RSS over baseline | ≤ 256 MiB (to be measured) | §5.2 |
| Ephemeral | 64/s, burst 128, 1 KiB, schema | Stage 1 |
| Session keys | ≤ lifetime peers | §5.5 |
| Cooldown map | ≤ current members | §5.2 |
| Sync-state files | 0 | Stage 1 |
| Go attestations | per signer ≤ admitted; total ≤ writers × admitted × 4 KiB | quadratic in writers: tracked risk |

## 7. Stages and closure of the Stage 1 residuals

| Residual (grade at Stage 1) | Closed by | How |
|---|---|---|
| S1-R1 writer panic, divergence, unreloadable document (Critique) | **Stage 2** | ADR 0030 guard G1-G10 + containment; S1 may be declared closed only then (N8) |
| S1-R2 slot exhaustion, rows hidden by deletion (Critique) | Stage 3 | per-writer chains (§2), no shared slots, fallback rows (§2.4) |
| S1-R3 arbitrary or foreign ids (Majeur) | Stage 3 | derived ids, full signed tuple, G11 anchor (§2.2-2.3) |
| S1-R4 residency and bytes (Majeur) | Stage 3 | patch P1 + count and byte budgets (§5.3) |
| S1-R5 peak RSS on large frames (Majeur) | Stage 3 | Go large-frame semaphore + measured peak (§5.2) |

Order: Stage 2 before Stage 3 (G11 lives in the guard module; the residency policy reloads documents,
which must pass the guard). Stage 3 internal order: `listing` (Go) → member-op `SeenListing` (with
ADR 0030's v2 member ops) → bridge kinds → webview set/history fallback → patch P1 + residency →
scheduler and work budget → Go frame semaphore.

## 8. Stage 3 TDD plan (Stage 1's is above; Stage 2 = ADR 0030's T1-T7)

1. `listing`: golden vectors; `FuzzDecode`; L1-L5; `DocId` derivation vectors shared with TS.
2. **Equivocation flood across restarts**: a writer forks its chain at every position, 10 000 ops,
   over 3 restarts and 3 replicas in random orders → retained ops ≤ Q_w + 1, admitted ids ever ≤
   Q_w + 1 on each replica, the freeze point identical everywhere, the proof gossiped.
3. **Cross-writer aliasing**: W cannot produce an op whose `DocId` equals H's (property test over
   random tuples); W's equivocation never changes H's admitted ids.
4. **Fairness**: W uses its 32 positions and floods invalid ops; honest H creates 5 variants
   concurrently → all 5 admitted everywhere; W deletes H's `meta.variants` entries → H's rows still
   shown (fallback); H's quota untouched.
5. **Revocation**: W removed; W's op naming an old frontier after the cut → dead on all replicas in
   all orders; an op the remover had seen (`SeenListing`) → standing; suspension then let-back →
   later ops standing; dead documents shown "à confirmer", re-admitted by `doc/<id> = kept`.
6. **Provenance and G11**: a foreign doc id cannot be admitted; a squatter's rival root history for a
   listed id is refused by G11 on 3 replicas; an import becomes a new listed id.
7. **Purge**: `doc/<id> = purged` → id leaves the set, `repo/<id>/` removed, store refuses writes
   under it, replayed listing op has no effect, quota position not returned, a grant adds 32.
8. **Bootstrap**: unknown handle + held messages → one `find`, drain at `requesting` and at
   `unavailable`; cancellation after `await` (peer down, epoch change, id purged).
9. **Residency**: 200 admitted documents spoken about in turn → resident non-open ≤ 16 and ≤ 32 MiB
   stored; unavailable handles evicted (patch P1 contract test); RSS measured on full handles and
   synchronizers.
10. **Multi-peer fairness**: 3 hostile peers overflow, 1 honest peer typing → honest p99 delivery
    latency ≤ 2 × its baseline (real clock, as ADR 0030 T5); cooldown kept across reconnects.
11. **Peak RSS**: 3 peers each sending 64 MiB frames → peak ≤ baseline + 256 MiB, recovery to
    baseline + 10 % within 30 s.
12. **Reconnect benchmark** (review 2, point 11): real `Repo` over simnet with coverage, guard and
    transport, multi-actor random edits, 1 000/10 000/100 000 changes, ≥ 10 runs, median and IQR,
    `isEphemeral` vs persisted (patched throttle per document, for the comparison only). Keep
    `isEphemeral` unless the p50 reconnect of a 100 000-change document under the ADR 0028 WAN
    profile exceeds 1 s.
13. Epochs: every delayed callback of §5.4 with a project switch in flight.

## 9. Residual risks after Stage 3

| Risk | Grade | Note |
|---|---|---|
| A writer edits shared content maliciously | not a finding (product behaviour) | attributable, reversible, removal stops it |
| Automerge `(actor, seq)` equivocation | as ADR 0030 | proof and escalation there; actor ↔ key ADR pending |
| Dead listings hide honest edits until an admin re-lists | Mineur | recoverable by `doc/<id> = kept` |
| Attestation memory quadratic in writers | Majeur if W grows (watch) | measure with ADR 0028 endurance before raising Q |
| Patch P1 maintenance | Mineur | contract test, upstream PR |
| Quota 32 is an assumption | — | open question, compatible to raise |

## 10. Where to enforce (boundary alternatives, from v2, amended)

| Option | P1 | P2 | P3 | backpressure | R1 | §4 shapes | ADR 0011 | Maintenance |
|---|---|---|---|---|---|---|---|---|
| A. Repo `shareConfig` only (candidate fix) | no | no | no | no | breaks | no | ok | none |
| B. Go parses the cbor-x repo wrapper and filters | partial | yes | yes | yes | needs the set anyway | duplicated | **violates** | Go follows automerge-repo's wire schema |
| C. pnpm patch of automerge-repo alone | yes | yes | no | if patched | no | no | ok | patch per upgrade |
| D (v1). boundary + session rewrite + storageId pinned to peer id | yes | yes | GC races pending saves | no | yes | yes | ok | none |
| E (Codex). boundary + `isEphemeral` + small patch | yes | yes | by construction | with patch | yes | yes | ok | patch |
| **F (chosen)**: boundary (shapes, membership, ephemeral rewrite, hold, scheduler) + `isEphemeral` + **only** patch P1 (`removeFromCache` on non-ready handles), because the gap is measured (09) | yes | yes | by construction | in the adapter | yes | yes | ok | one ≤ 15-line patch, contract-tested |

Reconnect cost of `isEphemeral` (09, single-actor append workload, so indicative only):
+1.25 B and ≈ +2 µs per change per document per reconnect (12 623 B vs 148 B at 10 000 changes).
Persisted state is lossy anyway in 2.5.6 (one throttle per storageId cancels other documents' saves,
`throttle.js:80-104`). The decision is re-checked by the repeated real-Repo benchmark (§8, test 12).
Other bookkeeping the patch E would prune (`#ephemeralSessionCounts`, `#hasRequested`,
`#pendingSyncMessages`) is already bounded by F (rewrite, lifetime-bounded admitted set, hold).

## Changes after review 2 (Codex, [08-codex-design-review-2.md](08-codex-design-review-2.md))

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

Kept for history; where v3 changed the answer, review 2's table below prevails.

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
