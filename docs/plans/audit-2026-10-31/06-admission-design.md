# Design v4: admission boundary and per-writer document chains (full Byzantine model)

Author: architect (Opus), 2026-10-31. History: v1 → Codex review 1
([07-codex-design-review-1.md](07-codex-design-review-1.md)) → v2 → review 2
([08-codex-design-review-2.md](08-codex-design-review-2.md)) → v3 → review 3
([10-codex-design-review-3.md](10-codex-design-review-3.md)) → **v4** (this file). The point-by-point
answer to review 3 is the table "Changes after review 3" at the end; the user decisions this
revision still needs are in §11. User decisions N7 (full Byzantine threat model) and N8 (merge
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
design requires of it (§7.1: actor-author binding G12, joint causal past, absorbing purge,
acknowledgements, fair scheduler, member-gated peer table). **Stage 3** = this file's Byzantine
admission (§2-§6). Every Stage 1 residual is mapped to its closing stage in §7.2; what stays open
after Stage 3 is graded in §9.

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
| Coverage T10 (ADR 0018), by author key with G12 | who wrote a change | exists; G12 in 2 |
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
| **(g) v4 = (e) + `adopt` ops + `Decisions` frontier (chosen)** | own ≤ Q_w; others may keep ≤ 1 branch per (adopter, slot) | impossible | anchored; one anchor per id, so G11 is a function of the id | **kept**: every co-editor adopts automatically | (e) + one op kind |

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
| `Position` | 1 + position of `Prev` (the chain is linear and numbered) |
| `Kind` | `variant`, `adopt`; `node` reserved for the ADR 0023 index (16-byte `Node` field), **not implemented until needed** |
| `Anchor` | `variant`: hash of the anchor change of the new document (32 B), §2.3 |
| `Adopted` | `adopt`: the complete canonical bytes of another author's `variant` op (≤ 4.4 KiB) |
| `Author`, `Sig` | device key, signature |

Size ≤ 2·64·32 + 200 B ≈ 4.3 KiB for a variant, ≤ 8.7 KiB for an adoption (typical ≈ 300 B /
600 B). `Decode` strict, golden vectors shared with TS, `FuzzDecode`. **Document id** (never
transmitted, always recomputed from a `variant` op, embedded or not):
`DocId = BLAKE3("inkwell/doc/v1" ‖ Project ‖ Author ‖ Kind ‖ Position ‖ Anchor)[:16]`, rendered with
automerge-repo's `binaryToDocumentId`. A writer cannot name another writer's document (aliasing),
a foreign or arbitrary id (provenance), and each (author, position, anchor) yields one id.

**Admission of an op** (pure function of the op and its causal past, like `List.integrate`):
- L1 canonical, signed, for this project.
- L2 `Prev`, `Members` and `Decisions` held; otherwise wait in a buffer **per author**, ≤ 4 ops,
  standing writers only; a fifth evicts that author's oldest, never another author's.
- L3 `Position` exact.
- L4 the author is a writer, not suspended, in the member list folded on `past(Members)` (ADR 0030 A4
  analogue with role ≥ writer).
- L5 `variant`: the number of `variant` ops on the chain up to and including o is ≤ `Q(o)` (§2.5).
  `adopt`: the embedded op e passes L1, L4 and L5 at its own frontiers (L2 waits for them),
  `e.Author ≠ o.Author`, `e.Kind = variant`, and no earlier op on o's chain adopts the same
  `(e.Author, e.Position)`. Chain checks (L2-L3) are **not** applied to e: an adoption must work
  precisely when e's chain is frozen or dead.

**Equivocation (rule E).** Two admitted ops of the same author with the same `Position` (different
hashes) form a proof. The author's chain is **frozen** at the smallest such position k: its own ops
at positions ≥ k stop standing everywhere; the proof (two ops) is kept and gossiped, the UI names
the person, the escalation is removal (as ADR 0030 G8). Retention per author: its chain up to k−1,
plus the two ops of the proof; ops arriving later at positions ≥ k are not stored — except as the
`Adopted` payload of another writer's standing adoption, which carries its own copy.

**Adoption (review 3, point 1).** A frozen or dead `variant` op stops standing, but the documents
others co-edited must stay visible and recoverable. Rule: **a document stays admitted while some
standing writer's chain adopts its op.** Go adopts automatically: when the fold makes a document
d leave the admitted set by freeze (rule E), by revocation (§3) or by losing a quota grant (§2.5),
and d ∈ `contrib` (the documents this device signed heads for, recorded by `attestHeads`,
`internal/app/attest.go:124`, persisted in `.inkwell/listing/contrib`, ≤ admitted ids), and this
device is a standing writer, Go signs `adopt(op(d))` once. Consequences:
- No retroactive loss of co-edited content: a document H edited stays admitted on every replica
  once H's adoption arrives; between the proof and the adoption it is shown "à confirmer" by its
  holders (never deleted, §3), then "conservé par <personne>".
- The adopter's quota is **untouched** (L5 counts `variant` ops only), so W cannot spend H's quota
  by making H adopt.
- A purge still wins: `Admitted` subtracts purged ids (§2.4); adoption never resurrects one.
- Bounds: an adopter adopts ≤ 1 op per foreign (author, position), so the admitted ids derived from
  W's signatures are ≤ (k − 1) + (writers − 1) × Q_W, each adoption attributable to its
  adopter, whose own adoptions are ≤ Σ_{v ≠ adopter} Q_v. A colluder can keep one W branch per W
  position alive; it signs for it.
- Alternatives rejected: eager adoption on the first co-edit (no "à confirmer" window, but one op per
  foreign document edited, ≈ W² ops in a busy project); copying the co-edited document into a new
  variant of H (spends H's quota — W would hinder H — and breaks the link for other co-editors).

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
  as an optional grace period.
- (P-d) **Two phases, erasure gated by causal stability (chosen)**, the CRDT notion of a causally
  stable operation (Baquero, Almeida, Shoker, *Pure operation-based replicated data types*, 2017):
  an operation is stable when no concurrent operation can still be delivered.

**Semantics (amendments to ADR 0030, required in Stage 2, §7.1).**
- **Absorbing purge.** For `doc/<id>` and `node/<id>`: a decision whose causal past holds a standing
  `purged` for that key cannot change it (its entry for that key is ignored). Concurrency keeps
  D2's order (`kept` > `trashed` > `purged`, refusal wins). So only an operation **concurrent** to the
  purge p can annul it: a concurrent `kept`/`trashed`, or a concurrent cut of p's author (D1).
- **Joint causal past.** "o ∈ past(x)" follows links in both DAGs transitively: member ops reach
  decisions through `Seen`, decisions reach member ops through `Members`. A cut whose author's
  promotion op had seen p has seen p. (ADR 0030 D1 currently tests `o ∈ past(Seen(m))` only.)
- **Acknowledgement.** Each admin device publishes `Ack = {Author, MemberFrontier, DecisionFrontier,
  Sig}`, latest-wins per device, each new one including the previous (monotone), on the trust channel,
  persisted `.inkwell/policy/acks/<key>` (≤ 1 per admin device). An `Ack` counts only once every op
  its frontiers name is held. An honest client emits one when it holds a standing purge it has not
  acknowledged (debounced 60 s).

**Phases (Go, `internal/policy` + store).**
1. **Logical tombstone** (purge standing locally): the id leaves `Admitted`; the webview drops queued
   messages, flushes and evicts the handle (ready-only rule, §5.3); bytes stay on disk; store writes
   under `repo/<id>/` are still accepted (a save already engaged completes; nothing new comes since
   the id is no longer synced). Annulment → the id returns with its bytes; sync resumes.
2. **Stable**: p standing **and** every admin device standing in the current member fold has a
   counted `Ack` whose joint past contains p **and** the optional grace period T_purge has elapsed
   since p was first held (§11, D-2; 0 is safe).
3. **Erasure** runs as a store-stream command, so it is ordered after any save already queued:
   (a) append `id` to the persistent erased list (`.inkwell/purge/erased`, fsync), from which the store
   refuses every later write under `repo/<id>/`; (b) remove the directory; (c) `SyncDir`. On restart,
   a listed id whose directory still exists is removed again (idempotent).

**Why stability is sufficient (honest admins).** Any operation that could annul p is concurrent to p.
(i) An op authored by a standing admin before its `Ack` is in that `Ack`'s frontiers, hence held and
already folded. (ii) An op authored after its `Ack` has p in its joint past (an honest client's
frontier only grows). (iii) An op of a device that is not standing stands later only if a standing
admin re-admits it (let-back, ADR 0021): ops concurrent to that device's suspension or cut stay dead
after the let-back (ADR 0030 test "décision d'un suspendu (morte même après la reprise)"). (iv) A
future admin's ops contain its promotion, whose author had acknowledged p, hence p (joint past).
A **malicious standing** admin can always author a concurrent op; that is admin power (§1) and the
reason the UI shows "contenu effacé" rather than an empty document if it ever happens.

**Liveness.** An admin device that never comes back blocks erasure (bytes kept, nothing lost); the
UI lists it and an admin can remove the device, after which stability is recomputed.

**Tombstone and replay.** Decisions are permanent (grow-only journal) and a purge is absorbing, so a
replayed listing op (same hash) or adoption cannot re-admit the id, and no new op derives the same
`DocId` (position consumed, anchor fixed). The listing op is kept (it numbers the chain). A purge
does **not** return the position (lifetime quota); exhaustion is handled by grants (§2.5). The same
two phases gate ADR 0023 blob GC for `node/<id> = purged`.

## 5. Boundary in Stages 2-3 (deltas over Stage 1)

### 5.1 Bootstrap loader: one lane per owner, no shared slots (review 3, point 2)

Stage 1's loader (`frontend/src/workspace/loader.ts:4-48`) has one FIFO of ≤ 257 ids and 4 shared
slots: four ids whose storage load is slow occupy every slot and H's document waits (review 3).

**Owner of an id**: `project` for the root and the index; `local` for documents this person opens;
otherwise the author of the standing `variant` op, or, for an adopted document, its adopter.

**Rules.**
- One FIFO per owner, deduplicated per id, bounded by construction (an owner has ≤ its admitted
  ids); lanes are served round robin.
- **≤ 1 load in flight per lane**; no global cap on lanes (a global cap is what colluders saturate).
  In flight ≤ number of lanes with work ≤ writers + 2.
- **Effective cancellation of the wait.** The loader races each `find` against a per-load deadline
  of 10 s and against the epoch/admission/peer re-checks; on any of them it releases the lane. The
  underlying load cannot be cancelled (`Repo.js:477-480`) but is deduplicated per id
  (`Repo.js:425-436`), so abandoned loads are ≤ admitted ids, each ≤ one non-ready handle.
- **Shared storage stream.** Loads still pass the sequential store stream
  (`GoStorageAdapter.ts:57`); with one load per lane, H's load waits behind at most one request of
  each other lane, each ≤ one 64 MiB frame read (`storestream.go:293`). Measured by test B10;
  per-document parallelism in the store stream is the escalation if B10 exceeds its threshold.
- **Non-ready handles** are no longer capped separately: there is one per admitted id at most
  (only admitted ids reach the Repo), so ≤ admitted ids × 66 KiB, and each is attributable to its
  owner's lane.

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
- **Large-frame permits: P = 2, global, fair.** A completed large frame needs a permit to be
  delivered: permits are granted in FIFO order of request, **at most one per peer**, held from the
  first slice until the webview's `Done(peer, seq)` (sent after the guard and the apply, accepted or
  refused), PeerDown, or a 30 s deadline (then the frame is dropped and the peer cooled down). A
  released peer re-enters at the back. Holds cover local delivery and processing only, never
  network transfer (the frame is already on disk), so a slow or stalled sender cannot sit on a
  permit. With k colluders, an honest large frame waits ≤ ⌈k / 2⌉ turns of ≤ 30 s (typically the
  apply time, ≤ S_max below). Codex's case "two peers occupy both permits" is impossible past one
  turn each.

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
  not**: S_max = apply time of the largest admissible message. Its bound comes from ADR 0028 D1:
  before the Worker (ADR 0028 D2) a peer fragment is capped at the 512 Ki "Grand" ceiling
  (`0028-performance-huge-files-and-wan.md:129`, target ≤ 100 ms of freeze), after it at 4 Mi, off
  the display thread. The D1 growth cap is specified in ADR 0028 but not found in
  `frontend/src/net/coverage.ts` on `main`: Stage 2 must land it before the fairness claim holds.
- **Overflow** → reset that peer with a cooldown of 30 s doubling to 10 min, kept per member device
  across reconnects (map ≤ member devices, pruned when the device leaves the list). The reset is not
  "fresh": `endSync` keeps `#syncStates` (`DocSynchronizer.js:217-222`); retention ≤ lifetime peers ×
  admitted.
- **Rejected alternatives**: a per-peer token bucket at 16 MiB/s (v3; not work-conserving, and does
  nothing for an indivisible apply); weighted fair queuing (O(log n), no gain at a few peers);
  splitting sync messages (impossible: one bundle); a two-Repo split, open documents vs the rest
  (strong isolation, but a second Repo and document migration; kept as the escalation if test B11
  fails after the Worker).

**Peak RSS (S1-R5).** Webview: P × MaxFrame × k_dec + members × (2 MiB credit + 1 MiB slice) +
baseline, where k_dec (copies + decode) is measured by test B12; Go: members × (4 MiB reassembly +
8 MiB small queue) + P × MaxFrame (read back from disk for delivery). Every term is either per peer
(attributable) or shared by fair turns.

### 5.3 Residency without patching automerge-repo (review 3, point 9)

v3 adopted a pnpm patch (P1) so that `removeFromCache` would evict non-ready handles. Review 3 showed
that a patch limited to `removeFromCache` is insufficient: `removeFromCache` waits outside
loading/requesting (`Repo.js:721`), `flush` throws on a non-ready handle (`Repo.js:706`), and a late
load re-registers the synchronizer (`Repo.js:499`). An exact patch would need four hunks (abortable
storage wait, identity check after every `await` in `#loadDocumentWithProgress`, non-ready branch in
`removeFromCache`, ready filter in `flush`) and a lifecycle contract test per upstream release.

**Alternatives.**
- (R-a) The exact four-hunk patch above. Works; adds a maintained fork of the load path (ADR 0008
  allows it, at a cost).
- (R-b) **No patch (chosen)**: never evict a non-ready handle; bound them by the admitted set (§5.1);
  evict ready handles only, through the public API, in a quiescent window.
- (R-c) Upstream change first, then (R-b) → (R-a) if it lands: tracked, not relied upon.

**Rules (R-b).**
- Resident = open documents ∪ an LRU of non-open **ready** documents. Interim budget (until the
  Worker): count ≤ 16 and Σ `A.stats(doc).numOps` ≤ an op budget calibrated by test B9 (memory is
  linear in ops, ADR 0028 constat 4, not in stored bytes: 241 B stored gave 309 KiB RSS in 09). With
  the Worker (ADR 0028 D2, D9), the budget becomes the Worker's WASM linear memory, measured
  directly, with recycling (terminate the Worker, reopen visible documents) as the hard bound.
- **Eviction protocol** for a ready, non-open handle: (1) no `heads-changed` for ≥ 1 s and `ready`
  observed in an earlier macrotask (so `Repo.js:499` and `:512` have run); (2) `await
  repo.flush([id])` (ready, so `Repo.js:706` cannot throw); (3) re-check heads unchanged and still
  not open; (4) `await repo.removeFromCache(id)` (ready, so `Repo.js:721` returns at once). The 1 s
  idle window exceeds the save throttle, so no stale throttled save can run after eviction and
  compact away newer chunks (`StorageSubsystem.js:215-239`).
- A document evicted and spoken about again is reloaded by the Repo; reload work is charged to the
  speaking peer in the scheduler (§5.2).
- Non-ready handles: ≤ 1 per admitted id; they become ready when a holder answers, or stay
  unavailable (≈ 66 KiB each, measured) and count in their owner's lane.

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
- **Webview**: `Admission` creates entries only for member devices (Go sends the member device set
  with the epoch); an entry and its cooldown are removed when the device leaves the list. Table size
  ≤ member devices; no eviction rule needed. Member devices are added by admins only (attributable,
  ≤ the 1 024-op list).
- Alternative rejected: reserve the 1 024 entries by class (members never evicted by strangers).
  It keeps a capacity number that has no meaning and still lets strangers hold state.

## 6. Numeric bounds (Stages 2-3)

| State | Bound | Basis | Test |
|---|---|---|---|
| Ids a writer W can ever get admitted from its own chain | ≤ Q_W (+1 transiently, before the proof arrives) | positions, rule E | A2 |
| Ids kept alive by adoption | ≤ 1 per (adopter, author, position); per adopter ≤ Σ_{v≠adopter} Q_v | L5 adopt | A3, B6 |
| Admitted ids | ≤ 2 + Σ_w Q_w + adoptions + `kept` | §2.4 | A2-A7 |
| Listing ops retained | per author: chain below the freeze + 2 (proof); ≤ 8.7 KiB each | rule E | A2 |
| Listing pending buffer | ≤ 4 per standing author, evicting only that author's | L2 | B5 |
| Repo bookkeeping per id (`#hasRequested`, synchronizer, sync states) | ≤ admitted ids; sync states ≤ lifetime peers × admitted | only admitted ids reach the Repo | Stage 1 test 3 |
| Non-ready handles | ≤ admitted ids (≈ 66 KiB each, measured) | §5.1 | B8 |
| Loads in flight | ≤ 1 per lane; lanes ≤ writers + 2; lane wait ≤ 10 s | §5.1 | B10 |
| Resident non-open documents | interim: ≤ 16 and Σ numOps ≤ calibrated budget; with the Worker: WASM budget + recycling | §5.3 | B9 |
| Hold (Stage 1) | 4 per (peer, doc); 8 MiB + 1 frame per peer; no shared pool | Stage 1 | B3, B4 |
| Go reassembly | 4 MiB in memory per member peer; above, one temp file ≤ MaxFrame per peer | §5.2 | B13 |
| Go router queue | ≤ 8 MiB small + 1 large (on disk) per peer | §5.2 | B14 |
| Webview queue | ≤ 2 MiB credit + 1 large (permit) per peer | §5.2 | B13 |
| Large frames in delivery/processing | P = 2 global, ≤ 1 per peer, FIFO, ≤ 30 s hold | §5.2 | B12 |
| Indivisible apply S_max | ≤ 512 Ki fragment before the Worker (target ≤ 100 ms), 4 Mi after, off-thread | ADR 0028 D1-D2 | B11 |
| Scheduler | DRR, quantum 8 ms of measured cost; interactive first, background ≥ 1 in 4 | §5.2 | B11 |
| Peak RSS over baseline | webview P × MaxFrame × k_dec + members × 3 MiB; Go members × 12 MiB + P × MaxFrame | §5.2 | B12 |
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
1. **G12, actor ↔ author binding.** Honest clients create, load and clone every document with
   `author = hex(fingerprint(device key))` (Automerge 3.5 `InitOptions.author`,
   `implementation.d.ts:84-85`; the field is per change and hash-committed, 09 `author2.mjs`). The
   guard requires: (a) every change carries an author that is the fingerprint of a member device;
   (b) all changes of one actor carry the author of that actor's `seq 1` change, which is in
   `before(c)` by G3, so the check is deterministic; (c) coverage T10 counts only an attestation signed
   by the key whose fingerprint is `author(c)` (a missing one makes c wait, never refuses it). This
   closes changes under another member's actor, the split of another's document by reusing its actor,
   and the descendant that usurps an actor (G11 case 4).
2. **Legacy history.** Changes without `author` are admitted only in `past(L_d)`, L_d being the heads
   of document d recorded once, at upgrade, in an admin decision `legacy/<d>` (pre-1.0 "version
   commune" clause of ADR 0030). User decision D-6.
3. **Joint causal past, absorbing purge, acknowledgements, two-phase erasure** (§4), for `doc/` and
   `node/` entries and for ADR 0023 blob GC.
4. **Fair scheduler and Go delivery** (§5.2), as the single mechanism behind ADR 0030 §5, and the
   ADR 0028 D1 growth cap it relies on.
5. **Member-gated peer table** (§5.6, S1-R6).

**What stays open after Stage 2 and 3 — and stays Critique:** W equivocating on **its own**
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
| S1-R4 residency and bytes (Critique) | Stage 3, step 3.7 (interim), ADR 0028 D2/D9 (hard bound) | ready-only eviction, op budget, then WASM budget with recycling |
| S1-R5 peak RSS on large frames (Mineur until measured) | Stage 2, step 2.5 | per-peer reassembly + spill, router queues, fair permits; measured by B12 |
| S1-R6 sybil ids fill the peer table (Critique) | Stage 2, step 2.0 | member-gated router and peer table |

### 7.3 Ordered implementation plan

Stage 2 (ADR 0030, prerequisite of Stage 3):
- 2.0 S1-R6: Go router forwards repo envelopes of member devices only; webview peer table keyed by
  member devices (A14). Independent and small: first.
- 2.1 ADR 0028 D1 peer-growth cap in coverage, if absent (prerequisite of S_max).
- 2.2 Guard G1-G10 + containment (`frontend/src/net/guard.ts`, `ancestry.ts`); ADR 0030 T1-T7.
- 2.3 G12 + coverage by author key + `legacy/<d>` decision (A15).
- 2.4 `internal/session` (audit M5) → `internal/policy` journal, v2 `Seen`, joint causal past,
  absorbing purge, `Ack`, two-phase erasure (A9, B15).
- 2.5 Scheduler: webview DRR shared with the guard; Go per-peer reassembly with spill
  (`internal/transport/mux`, a HOT_PATH: benchstat), router per-peer queues, DRR slices, credits,
  permits (B11-B14).

Stage 3 (this design):
- 3.1 `internal/listing`: ops (`variant`, `adopt`), `Decisions` frontier, L1-L5, rule E, `DocId`
  vectors shared with TS, `FuzzDecode` (A1, A2, A4, A7, B5, B6).
- 3.2 `SeenListing` on member ops (A6).
- 3.3 Bridge kinds `Listed`/`AttestRefused`; Go-owned admitted set replaces `scope.ts`; S3
  attestation cap per signer ≤ admitted (B7).
- 3.4 Fork flow: anchor change, `Session.List`, `import` with `docId`.
- 3.5 G11 in the guard (A8).
- 3.6 Automatic adoption from `contrib`; history fallback rows; "à confirmer" (A3, A5, A6).
- 3.7 Loader lanes; residency (R-b) with the interim op budget; WASM budget once ADR 0028 D2 lands
  (A10, B8-B10, B16).
- 3.8 H-1 test (document stored above 64 MiB), paginated `LoadRange` if confirmed; repeated
  reconnect benchmark (B17).

## 8. Tests

### 8.1 Scenario tests (Stage 2 additions and Stage 3)

- **A1** `listing`: golden vectors; `FuzzDecode`; L1-L5; `DocId` vectors shared with TS.
- **A2** Equivocation flood across restarts: a writer forks its chain at every position, 10 000 ops,
  3 restarts, 3 replicas, random orders → retained ops ≤ chain + 2, admitted own ids ≤ Q_w, freeze
  point identical everywhere, proof gossiped.
- **A3** Equivocation after honest contribution (review 3, point 1): W lists d (position 2); H edits
  d (attests); W sends a conflicting position-2 op to R3 and a conflicting position-1 op to R2 →
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
  (c) sequential `kept` after the purge → ignored (absorbing); (d) restart after the tombstone, after
  stability, and between the erased-list append and the removal → no premature erasure, removal
  idempotent; (e) a save queued before erasure completes; after erasure a late save is refused and the
  directory is not recreated; (f) an absent admin device blocks erasure; removing it unblocks;
  (g) an admin promoted after all acknowledgements cuts the purger with a stale `Seen` → purge stands
  (joint past); (h) replayed listing op or adoption → no re-admission; (i) blob GC follows the same
  phases.
- **A10** Loader (review 3, point 2): 40 hostile lanes with storage `load` blocked → H's lane starts
  its `find` within one turn; lane deadline releases; underlying loads ≤ admitted ids; cancellation
  after `await` (peer down, epoch change, id purged).
- **A11** Reconnect benchmark: real `Repo` over simnet with coverage, guard and transport, multi-actor
  random edits, 1 000/10 000/100 000 changes, ≥ 10 runs, median and IQR, `isEphemeral` vs persisted;
  keep `isEphemeral` unless the p50 reconnect of a 100 000-change document under the ADR 0028 WAN
  profile exceeds 1 s.
- **A12** One adapter per Repo by identity (§5.5).
- **A13** Epochs: every delayed callback of §5.4, and refusal notices in transit, with a project switch
  in flight.
- **A14** S1-R6: 2 000 stranger ids during an open invitation → 0 webview entries; a new member is
  admitted at once; removing a device frees its entry and cooldown; the invitation still completes.
- **A15** G12: changes under another member's actor refused in all orders; W's self-equivocation
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
| B6 | adoption 1 per (adopter, author, position) | first adoption stands | second, same key → refused | — |
| B7 | attestations per signer ≤ admitted | stored | non-admitted doc → `AttestRefused` | retried and accepted once the doc is admitted |
| B8 | non-ready handles ≤ admitted | 300 admitted, absent documents → ≤ 300 handles, RSS ≤ 300 × 70 KiB | 50 000 invented ids → 0 handles | a holder answers → handles become ready |
| B9 | resident non-open: 16, op budget | 16 resident | 17th → LRU evicted after 1 s idle; a document changed within 1 s or open is not | evicted, reloaded, changed, wait 2 × debounce, reload from storage → every change present; RSS per ready handle with synchronizer and 1-3 peers' sync states measured |
| B10 | loader: 1 in flight per lane, 10 s wait | 1 in flight | a 2nd id of the same lane waits | deadline (fake clock) releases the lane; real storage: H's first load starts within (lanes × measured store service time) |
| B11 | DRR quantum 8 ms, classes, S_max | 2 backlogged peers: shares 45-55 % over 10 s (real clock) | a 400 ms apply → that peer skipped until repaid | lone peer gets 100 %; interactive p99 ≤ 2 × baseline + S_max with 3 hostile peers sending small frames continuously and large frames |
| B12 | permits P = 2, 1 per peer, 30 s | 2 peers served | 3rd waits; FIFO; one peer never holds 2 | release on `Done`, PeerDown, and at 30 s without `Done` (a peer that never completes); peak RSS ≤ baseline + P × MaxFrame × k_dec + members × 3 MiB, back to baseline + 10 % in 30 s |
| B13 | Go reassembly 4 MiB / spill; credit 2 MiB | 4 MiB in memory | 5 MiB → temp file; fast hostile and slow honest senders both complete; Go never sends beyond credit (spy) | temp file removed on completion, drop, PeerDown, 30 s TTL |
| B14 | router queue 8 MiB small | 8 MiB queued | +1 B → that peer reset only | cooldown shared with the webview map |
| B15 | acknowledgements | `Ack` counted when its frontiers are held | an older `Ack` → ignored; one naming unheld ops → not counted | counted when they arrive |
| B16 | handle timeout 60 s; unload of invisible documents 60 s (ADR 0028 D9) | at 59 s still loaded | at 61 s unloaded | reopened on demand |
| B17 | reconnect cost | = A11 | — | — |

## 9. Residual risks after Stage 3

| Risk | Grade | Note |
|---|---|---|
| W's own `(actor, seq)` equivocation splits documents W writes in, H's included | **Critique** (review 3, point 3) | no deterministic rule without consensus; proof, no relay, removal, recovery by copy (§7.1); an ADR must decide whether to accept it or add a stabilisation step |
| H-1: a document stored above 64 MiB cannot load (`LoadRange` in one frame, `storestream.go:293`) | hypothesis; Critique if demonstrated (data unavailable) | step 3.8; fix: paginated `LoadRange` |
| Cumulative history growth by writing (insert/delete churn) | Majeur hypothesis, to measure | user decision D-3 |
| Indivisible apply S_max | Mineur if B11 passes; Majeur otherwise | escalation: two Repos (§5.2) |
| "à confirmer" between a freeze/death and the co-editor's adoption | Mineur | nothing deleted; admin `kept` |
| Erasure blocked by an absent admin device | Mineur (disk only) | remove the device |
| Malicious standing admin annuls or erases | not a finding under N7 (admin power, ADR 0017) | UI "contenu effacé" |
| Attestation memory quadratic in writers | Majeur if writers grow (watch) | measure with ADR 0028 endurance before raising Q |
| Quota 32 is an assumption | — | D-1 |

## 10. Where to enforce (boundary alternatives, amended in v4)

| Option | P1 | P2 | P3 | backpressure | R1 | §4 shapes | ADR 0011 | Maintenance |
|---|---|---|---|---|---|---|---|---|
| A. Repo `shareConfig` only (candidate fix) | no | no | no | no | breaks | no | ok | none |
| B. Go parses the cbor-x repo wrapper and filters | partial | yes | yes | yes | needs the set anyway | duplicated | **violates** | Go follows automerge-repo's wire schema |
| C. pnpm patch of automerge-repo alone | yes | yes | no | if patched | no | no | ok | patch per upgrade |
| D (v1). boundary + session rewrite + storageId pinned | yes | yes | GC races pending saves | no | yes | yes | ok | none |
| E (Codex 1). boundary + `isEphemeral` + small patch | yes | yes | by construction | with patch | yes | yes | ok | patch |
| F (v3). E with patch P1 only | yes | yes | by construction | in the adapter | yes | yes | ok | patch insufficient as described (review 3, point 9) |
| **G (v4, chosen)**: boundary (shapes, membership, ephemeral rewrite, hold, scheduler) + `isEphemeral` + **no patch** (ready-only eviction, non-ready handles bounded by the admitted set) | yes | yes | by construction | in the adapter and Go | yes | yes | ok | contract tests only |

Reconnect cost of `isEphemeral` (09, single-actor append workload, indicative only): +1.25 B and
≈ +2 µs per change per document per reconnect. Persisted state is lossy anyway in 2.5.6 (one
throttle per storageId cancels other documents' saves, `throttle.js:80-104`). Re-checked by A11.

## 11. Decisions still needed from the user

- **D-1** Base quota 32 variants per writer and grant size 32 (assumption, `docs/open-questions.md`).
- **D-2** Grace period T_purge before erasure, on top of stability. Proposal: 30 days (a recovery
  window, as common trash bins); 0 is safe with stability.
- **D-3** Cumulative history a writer adds by churn. Recommended: detect and escalate (history bytes
  attributed by G12 per writer and per day, alert at 256 MiB/day naming the person, admin suspends).
  Enforcing would delay honest peers' relays too, because a bundle cannot be split (09).
- **D-4** Purge becomes irreversible (absorbing) for variants and nodes: a later `kept` no longer
  undoes a purge; the trash stays the undoable step. Changes ADR 0030 D2 for sequential decisions.
- **D-5** Automatic adoption of co-edited documents, without asking (recommended), or ask the person.
- **D-6** Legacy author-less history: one admin decision `legacy/<d>` per document at upgrade, or a
  reset of pre-1.0 projects.

Sources used in v4: Kleppmann, *Making CRDTs Byzantine Fault Tolerant* (PaPoC 2022, §2.3.1, §3.4);
Baquero, Almeida, Shoker, [*Pure Operation-Based Replicated Data Types*](https://arxiv.org/abs/1710.04469)
(causal stability); Shreedhar and Varghese, [*Efficient Fair Queuing Using Deficit Round Robin*](https://iqua.ece.toronto.edu/baochun/ece1771f/papers/deficitrr.pdf)
(IEEE/ACM ToN 1996); [RFC 9000 §4](https://www.rfc-editor.org/rfc/rfc9000#section-4) (receiver credit);
automerge 3.5.0 `implementation.d.ts:84-85,387-390,627` (author, stats).

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
| 13 | OPEN, Critique: erasure before annulment | logical tombstone, then erasure only when **stable**: every standing admin device's `Ack` contains the purge in its joint causal past, plus optional grace; absorbing purge; joint causal past; erased list before removal; proof of sufficiency (§4) | A9, B15 |
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
