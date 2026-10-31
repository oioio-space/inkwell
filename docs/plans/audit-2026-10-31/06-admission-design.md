# Design v2: admission boundary before the Repo + a signed, per-writer document listing

Author: architect (Opus), 2026-10-31. **v2 after Codex's adversarial review** (the 11 points are
mapped in the last section, "Changes after review"). Basis: automerge-repo **2.5.6** and
automerge **3.5.0** as installed; upstream `main` checked 2026-10-31; evidence `review-s1.md`,
`security-verify.md`; candidate fix (worktree agent-ade848f3…); Go live set (worktree
agent-a55a2bd8…, branch fix/trust-atomic-save). Measurements are in `scratchpad/bench-sync/`
(`reconnect.mjs`, `sizes.mjs`, `mem2.mjs`; command:
`node [--expose-gc] <script> <frontend>/node_modules/@automerge/automerge/dist/mjs/entrypoints/fullfat_node.js`).

Rules applied: ASVS 5.0 V2.4.1 (resource exhaustion); engineering-principles §4 (strict decoding,
bounded sizes); CLAUDE.md "une seule source de vérité par chose"; ADR 0002/0011 (Go sees opaque
bytes, never Automerge); ADR 0018 (coverage); ADR 0030 §3 (no node hidden except by decision or
trash), §4 (structural guard before the Repo); ISO/IEC 25010 functional suitability (R1).
Severity per `docs/plans/2026-10-31-architecture-audit.md`.

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

## 0. What the installed library does (grounding, file:line in `@automerge/automerge-repo/dist`)

| Where | Behaviour | Consequence |
|---|---|---|
| `network/NetworkSubsystem.js:47-63` | unknown type → `JSON.stringify(msg)` in a log template (built even with debug off); ephemeral → `#ephemeralSessionCounts[`${senderId}:${sessionId}`]`, never pruned | P5; P1 |
| `synchronizer/CollectionSynchronizer.js:87-95` then `:96` | `#hasRequested.set(documentId, …)` **before** `shareConfig.access`; `removePeer` (`:156-165`) empties Sets, keeps keys | P2: no share policy can stop it; keys are **lifetime**, not current, state |
| `Repo.js:236-243` | `synchronizer.receiveMessage(message).catch(…)`: fire-and-forget, no backpressure | review point 2 |
| `synchronizer/DocSynchronizer.js:259-266` | sync messages for a handle still loading go to `#pendingSyncMessages` (unbounded) | blocked storage + flood = unbounded retained bytes |
| `synchronizer/DocSynchronizer.js:74-95` + `CollectionSynchronizer.js:36-45` | callbacks pile up in `#pendingSyncStateCallbacks[peer]` while a sync state loads from storage; load skipped when the peer `isEphemeral` | blocked storage = unbounded callbacks |
| `Repo.js:128-131`, `:246-258`; `StorageSubsystem.js:252` | peer metadata taken as sent; sync state saved under `[doc,"sync-state",storageId]` through **one** `asyncThrottle` per storageId (default 100 ms) | P3; and see next row |
| `helpers/throttle.js:80-104` | each call `clearTimeout`s the pending one: with one throttle per storageId shared by all documents, the save of doc A is **dropped** when doc B's arrives within 100 ms; a delayed save fires after any external delete | persisted sync state is lossy across documents today; GC races with pending saves (point 7) |
| `synchronizer/DocSynchronizer.js:240-257` | received ephemerals re-emitted to other peers with the original sender/session/count | relays reach us with `senderId ≠ from`; our sender check drops them (wasted bandwidth) |
| `presence/PeerPresenceInfo.js:42-58`, `Presence.js:71-75`, `:212-221`; `constants.js` | `update` merges `{[channel]: value}`; channel is peer-chosen; prune runs every heartbeat (15 s) with TTL 45 s, so a silent peer lives up to ~60 s, and a talking peer is **never** pruned | the bound must come from a schema, not from the TTL |
| `Repo.js:55` | gossip of remote heads off by default | `remote-*` types are useless → drop |
| `Repo.js:715-737` | `removeFromCache` (hidden API) unloads a handle and its synchronizer | used for bounded residency |

Upstream (2026-10-31): v2.6.0-alpha.5, v3.0.0-experimental.2. `main` still keys session counts
without pruning and still records `hasRequested` before access. Issue
**automerge/automerge-repo#745** (open): ephemeral `senderId` is not attributable to a connection;
PR #742 changed ephemeral relay in 2.6.

Repo facts: presence is used (`editor/presence.ts`, `ui/sheet/presence.ts`; channels `name`,
`anchor`, `head`; two broadcasts per selection change, no throttle). Nothing reads `storageId` or
`getSyncInfo`. Variants are `repo.import` **full copies** listed in the root's
`meta.variants[].url` (`history/versions.ts:183-197`); any writer can also delete list items. On
`main` the webview never calls `Files.SetTree`, so the Go live set of agent-a55a2bd8 is dormant.
ADR 0030's guard is **proposed, not implemented**: a writer can today make Automerge panic or make
a document unloadable after save (ADR 0030:23, 36-46).

Measurements (automerge 3.5.0, Node 22):
- **Reconnect cost per document**, persisted sync state vs fresh (`isEphemeral`) state, both peers
  already in sync: 1 000 changes 148 B vs 1 373 B; 10 000 changes 148 B / 1.5 ms vs 12 623 B /
  7.6 ms; 100 000 changes 148 B / 38 ms vs 125 125 B / 220 ms (≈ 1.25 B and ≈ 2 µs per change,
  2 messages either way; with 10 new changes: 1.5 KB vs 251 KB at 100 000).
- **Resident memory per loaded document**: ≈ 56 KiB (empty) to ≈ 310 KiB (2 000 changes), RSS.
  10 000 resident documents would cost 0.5 to 3 GiB: residency must be bounded independently of
  the listing.
- **Presence payloads** (CBOR): snapshot with a 128-character name and two real cursors (37 chars)
  = 265 B; update = 89 B.

## 1. Alternatives

### 1.1 Where to enforce (boundary)

| Option | P1 | P2 | P3 | backpressure | R1 | §4 shapes | ADR 0011 | Maintenance |
|---|---|---|---|---|---|---|---|---|
| A. Repo `shareConfig` only (candidate fix) | no | no | no | no | breaks | no | ok | none |
| B. Go parses the cbor-x repo wrapper and filters | partial | yes | yes | yes | needs the set anyway | duplicated | **violates** | Go follows automerge-repo's wire schema (changes in 2.6/3.0) |
| C. pnpm patch of automerge-repo alone | yes | yes | no | yes if patched | no | no | ok | patch rebased per upgrade |
| D (v1). adapter boundary + session rewrite + storageId pinned to peer id | yes | yes | yes, but GC races pending saves (point 7) | **no** (point 2) | yes | yes | ok | none |
| **E (Codex). adapter boundary + `isEphemeral` peers + small audited patch** | yes | yes | **yes, by construction** | with patch | yes | yes | ok | patch |
| **F (chosen) = E without the patch**: boundary (shapes, membership, ephemeral rewrite, **hold queue**) + `isEphemeral` for every peer; patch only if a contract test proves a gap | yes | yes | yes, by construction | yes, in the adapter | yes | yes | ok | none (contract tests are the tripwire) |

E vs F, numerically:
- **Reconnect cost of `isEphemeral`** (shared by E and F): +1.25 B and +2 µs per change per
  document per peer per reconnect. A project of 1 root + 20 variants at 10 000 changes each:
  ≈ 265 KB and ≈ 160 ms per reconnect, against ≈ 3 KB and ≈ 30 ms persisted. A 100 000-change
  root: +125 KB and +180 ms. Acceptable for a desktop on a WAN (ADR 0028 tiers are about
  multi-MB documents; there the change count, not the bloom filter, dominates). Persisted state
  also brings less than it seems: the shared throttle drops saves across documents
  (`throttle.js:80-104`), so a multi-document burst keeps only the last document's state.
- **What the patch would add over F**: pruning of `#ephemeralSessionCounts` and `#hasRequested`
  keys, and a cap on `#pendingSyncMessages`. Under F these are already bounded: only admitted ids
  reach the Repo (§2 makes the admitted set lifetime-bounded), session keys are rewritten to one
  per peer per adapter instance, and the adapter holds messages for loading handles. The patch is
  ~25 lines of `dist/` JS in 3 files, re-applied at each of the ~1 upgrade per month the project
  saw in Sept–Oct 2026 (2.6.0-alpha.3 → .5, 3.0.0-experimental.1 → .2); `pnpm install` fails
  loudly when it no longer applies. Cost is small but non-zero, and the benefit is redundancy only.
- **Decision**: F. Keep the patch as a ready fallback (step 1's contract tests decide), and file
  upstream PRs (record `hasRequested` after `access`; prune session counts on
  `peer-disconnected`; bound `#pendingSyncMessages`), referencing #745.

### 1.2 What decides which documents belong to the project (the admitted set)

| Option | Deterministic | Attributable | Fair (no displacement) | Lifetime-bounded under churn | Project-bound | Go can use it without Automerge |
|---|---|---|---|---|---|---|
| a. root `meta.variants` order, global cap (v1) | yes | no | **no** (one writer fills 256) | **no** (delete/re-add) | no (any URL) | no |
| b. project tree (`SetOnTree`) | yes | no | no | no | no | yes, but excludes variants (R1) |
| c. member list ops (new kind) | yes | yes | yes | yes | yes | yes, but rejected by ADR 0030 O3 (seals person management, Welcome budget) |
| d. actor ↔ key binding, attribute each listing op to its author | yes | yes | yes | needs add-only schema | yes | no (needs Automerge) and a further ADR (ADR 0030 "acteurs liés aux clés") |
| **e. signed listing tokens, numbered per writer, grow-only set kept by Go (chosen)** | **yes** | **yes** (Ed25519) | **yes** (own quota only) | **yes** (`n ≤ Q` per writer, ever) | **yes** (signed over ProjectID) | **yes** |

## 2. The admitted set: a signed, per-writer, grow-only listing (`internal/listing`)

**Object.** `Listing{Project, Doc, Kind, N, Signer, Sig}`, canonical binary, domain `listing-v1`,
signed over `ProjectID`; `Doc` = 16-byte Automerge document id; `Kind` ∈ {variant} now,
{node} with the ADR 0023/0030 index later; `N` ∈ [1, Q(Kind)] is the writer's counter for that
kind. ≤ 160 bytes. `Decode` strict, golden vectors, `FuzzDecode`.

**Validity** (pure function of the listing and the member list, like `List.Verify`):
- signature valid, project matches, `N` in range;
- the signer **ever** held writer rights in the list (`EverWriter`, analogue of ADR 0030
  `EverAdmin`), so revoking a person does not hide what they had listed; a signer **removed before
  any writer right** never counts;
- equivocation: two valid listings with the same (Signer, Kind, N) and different `Doc` → the lower
  `Doc` (bytes) wins everywhere; the proof is logged and shown, as G8 in ADR 0030.

**Admitted set** = root document (`RootURL`, from the genesis/meta) ∪ index G (when ADR 0030 lands)
∪ `{Doc : valid Listing}` − `{Doc : purged by an admin decision}` (ADR 0030 journal, when it lands).
It is a **grow-only set of signed objects** (a G-Set): union is commutative, so every peer converges
to the same set; nothing is ordered, so nothing is truncated or displaced. One writer can fill only
its own `Q` slots, ever: delete/re-add churn cannot exceed `Q` per writer per kind for the life of
the project, across restarts (point 1).

**Bounds.** `Q(variant) = 32` per writer, `Q(node) = 2 048` per writer (set by the ADR 0023 lot).
Admitted ≤ 2 + W·(32 + 2 048) with W = writers ever (≤ 1 023: the member list seals at 1 024 ops).
Honest practice (a few writers) = a few thousand ids; worst case is linear in writers that admins
added. 32 is an **assumption** (no usage data exists): it is a constant in the token's validity,
raising it later is backward compatible (old tokens stay valid), and the UI refuses the 33rd variant
with "limite de variantes atteinte" (entry in `docs/open-questions.md` for the user).

**Where it lives (single source of truth).** In Go, `internal/listing` (store, verification, gossip),
persisted as `.inkwell/listing/<hash>.lst` through the store (bounded: ≤ W·2 080 × 160 B ≈ 330 KiB per
writer), replicated like attestations on the trust channel with a type byte (same pattern as ADR 0030
§2). `meta.variants` in the root stays **presentation** (name, from, state, keyed by URL); the history
panel shows a variant only if its document is admitted. Go never reads Automerge (ADR 0011 intact).

**Lifecycle of a variant.**
- *Create*: `fork` first calls the bound method `Session.List(doc, kind)` (await). Go checks this
  device is a writer, takes its next `N` (refuses past `Q`: `ErrQuota`), signs, stores, gossips, and
  emits `Listed` to the webview **before returning**. Then `repo.import` + the `meta.variants` entry.
- *Learn*: a peer's Go verifies, stores, gossips, emits `Listed`; its adapter admits the id and the
  replicator fetches it.
- *Leave*: only an admin `purged` decision (ADR 0030) removes it; deleting the entry from
  `meta.variants` only hides the row (presentation), never the document. This matches ADR 0030 §3:
  "seule une décision ou une corbeille cache un nœud".
- *Foreign documents* (point 5): **not allowed**. A URL pasted from another project has no listing
  signed for this ProjectID, so it is not admitted, not fetched, not shared. Importing content from
  elsewhere = copy into a new document (`repo.import`), which then gets its own listing.
- *Legacy variants* (created before this change: an entry in `meta.variants` with no listing): at
  upgrade, the device that created it (its actor made the entry, in its own history) signs a listing
  for it; any other such entry is shown "à confirmer" and fetched only from peers that push it (as on
  `main`), never pulled. Pre-1.0, acceptable.

**Go changes** (supersede the v1 `Scope` kind, which is dropped):
1. New package `internal/listing` (types, `Verify`, store, `Sync` on the trust channel) depending on
   `trust` only (`policy` precedent); `arch-check.sh` gets the edge.
2. `internal/app`: `attestStore.setLive` is fed by `listing` (root ∪ listed − purged) instead of
   `Files.SetOnTree`. **Remove** `files.SetOnTree(w.SetDocuments)` (`node.go:200`) and
   `Files.onTree`/`treeDocuments` in the agent-a55a2bd8 worktree **before it merges**: feeding the
   live set from the disk tree recreates R1. Until `listing` lands, `liveKnown` stays false
   (per-signer cap only), which is the current, bounded behaviour.
3. Per-signer attestation cap = |admitted| (an honest writer attests every admitted document it
   holds); global attestation budget ≤ W × |admitted| × 4 KiB (`MaxAttestationSize`), quadratic in
   writers: flagged risk, acceptable for W in the tens, revisit with ADR 0028 data.
4. Bridge: `Listed` (Go → webview, `Notices` replays the full set at attach, with a project
   epoch) and `AttestRefused{doc, reason}` (Go → webview). `router.go:191` `attest` currently only
   logs a refusal; it must emit `AttestRefused` so the adapter clears `#attested[doc]`
   (`WailsNetworkAdapter.ts:288-289` marks it as attested before any acceptance) and retries after
   the next `Listed`.
5. Epochs (point 6): `Listed` and `Notices` carry `(ProjectID, epoch)`; the adapter discards
   notices of another project and attests only documents of the current epoch's set. Because the
   set is computed and owned by Go and `List` returns only after the store is updated, there is no
   "Scope applied after Attest" window; the router loop stays FIFO (`router.go:175-192`).

**Trust store reopen (kept from v1, still required).** After a failed reopen in `swap()`, `s.root`
keeps a closed handle: every later save fails until restart; locally authored ops are not durable.
And `adopt` runs `replace` on the invites goroutine while the Wiring goroutine saves: a data race
on `s.root`, and old-project ops can land in the new `trust/`. Fix: `s.root` nil on failure, lazy
`rootLocked()` reopen under a mutex; run `replace` + `SwitchProject` as one step on the Wiring
goroutine. Tests: failed reopen then successful save; `-race` adopt vs saves; no foreign ops.

## 3. The admission boundary (frontend)

### 3.1 Receive chain

```
Go frame → decodeEnvelope → Kind.Message → cbor decode
 → admission (net/admission.ts)
     1 shape  : type whitelist, exact fields and bounds, fresh object
     2 sender : senderId === from; targetId === me; announced (non-handshake)
     3 member : documentId ∈ admitted (Listed set), else drop silently
     4 policy : handshake throttle + metadata override; ephemeral rewrite/rate/schema
     5 hold   : if the document's handle is not ready, queue (bounded) instead of delivering
 → Coverage T10 (ADR 0018)
 → structural guard G1–G10 (ADR 0030 §4)    ← prerequisite, see §5
 → emit("message") → Repo
outbound: drop ephemerals whose senderId ≠ me (relays)
```

### 3.2 Layout and types

- `frontend/src/net/admission.ts`: `parsePeerMessage(raw)` (pure strict decoder) and
  `class Admission { admit(from, msg, now); hold/release; forget(peer); stats }`.
- `frontend/src/net/scope.ts`: `class AdmittedSet` fed only by `Listed` notices (+ root); `has(id)`.
  No derivation from Automerge any more (Go owns it).
- `frontend/src/net/sharing.ts`: `shareAdmitted(set)`: `announce = access = set.has(id)`.
- `frontend/src/workspace/replicate.ts`: bounded pull replicator (§3.6).
- `WailsNetworkAdapter.ts`: transport, handshake, attest; delegates to `Admission`; handles
  `Listed`, `AttestRefused`.

### 3.3 Shapes (after decode; anything else dropped and counted)

| type | fields and bounds |
|---|---|
| `arrive`, `welcome` | `senderId` str ≤ 64; `peerMetadata` plain object or absent, **content ignored**; `projectUrl` absent or str ≤ 80 and `isValidAutomergeUrl`; payload ≤ 1 KiB (measured honest ≈ 150 B) |
| `sync`, `request` | `senderId`, `targetId` str ≤ 64; `documentId` str 1–64, admitted; `data` Uint8Array (≤ MaxFrame by Go) |
| `ephemeral` | as above + `sessionId` `/^[0-9a-z]{1,16}$/`; `count` safe integer ≥ 1; `data` ≤ **1 KiB** (measured snapshot 265 B: 3.8× margin) and presence schema (§3.4) |
| `doc-unavailable` | `senderId`, `targetId`, `documentId` admitted |
| other (`remote-*`, unknown) | dropped |

Plain object = prototype `Object.prototype`; fields via `Object.hasOwn`; the delivered message is
rebuilt from validated fields only.

### 3.4 Ephemeral messages (kept for presence, bounded)

- **Session rewrite, monotonic for the Repo's life (point 8).** Per peer, one slot
  `{realSessionId, lastCount}`; accept `count > lastCount` on the same session, or a different
  session (the slot is replaced, never added). Deliver with `sessionId = this.#session` (random,
  generated per **adapter instance**) and `count = ++this.#seq` (per instance, starts at 0). Keys in
  `#ephemeralSessionCounts` = peers × adapter instances on that Repo; the workspace creates one
  adapter per Repo, so = peers. A recreated adapter has a new session, so its counter starting low
  is never compared with the old one.
- **Rate**: per-peer bucket, capacity 128, refill 64/s. Basis: 2 updates per selection change;
  key repeat 25–33 Hz → ≤ 66/s for one document. Plus the recommended sender throttle (trailing
  edge, 20 Hz) in both presence modules → honest ≤ 40/s. The bucket **resets on PeerDown and on an
  accepted re-arrive** (tested, point 10).
- **Size**: 1 KiB. **Schema**: `{__presence: m}`, `m.type` ∈ {heartbeat, goodbye, update,
  snapshot}; channels ⊆ {name ≤ 128 chars, anchor ≤ 64, head ≤ 64} (cursor measured 37). Retained
  presence state per peer per document ≤ 3 short strings; the TTL only matters for silent peers
  (≤ ~60 s, not 45 s: prune runs every 15 s).
- **No relays out** (they are dropped by every receiver's sender check). Revisit if #745 lands.

### 3.5 Handshake and sync state

- `peer-candidate` is emitted with `peerMetadata = {isEphemeral: true}` for every peer, whatever it
  sent; the Repo is also created with `isEphemeral: true` (so honest peers do not persist about us).
  Then `Repo.#saveSyncState` returns at once (`Repo.js:250-252`), `onLoadSyncState` returns at once
  (`CollectionSynchronizer.js:40-43`), `setSyncInfo` is skipped (`Repo.js:157-160`). No sync-state
  file is ever written, so **nothing can resurrect** (point 7), no storageId is pinned (point 8's
  storage half disappears), and no callback waits on storage (half of point 2).
- **One-time cleanup**, in Go, at project open, **before the webview attaches** (no Repo exists, so
  no pending save can exist): remove every `<repo>/*/sync-state/` directory under the store root
  (`internal/store`, `os.Root`, idempotent, logged with a count).
- **Re-arrive throttle**: per peer, capacity 3, refill 1 per 20 s, one pending slot (latest wins,
  processed on refill). A processed re-arrive clears the peer's slot, buckets and hold queue.

### 3.6 Backpressure (point 2) and bounded residency

- **Hold queue.** The adapter delivers a `sync`/`request`/`ephemeral` only if the document's handle
  is `ready`, `requesting` or `unavailable` (predicate injected by the workspace from
  `repo.handles`). Otherwise it queues: ≤ 4 messages per (peer, document), ≤ MaxFrame + 1 MiB per
  peer, ≤ 2 × MaxFrame for all peers. Overflow for a peer → drop its queue and **reset the peer**
  (`peer-disconnected`, then re-handshake through the throttle); its Automerge sync restarts fresh
  (cost measured in §0). PeerDown or re-arrive → drop the peer's queue (cancellation). When the handle
  becomes ready, the queue drains in order. Honest steady state: queue empty (Automerge's sync is
  request/response: ≤ 2 outstanding per peer per document).
  Effect: `#pendingSyncMessages` never receives a message (the handle is never loading when we
  deliver), and each delivered message costs the Repo O(1) awaits (`access` resolves at once, `find`
  on a ready handle resolves at once).
- **Replicator (pull, bounded residency).** For each admitted id: `find` with
  `allowableStates: ["ready","unavailable"]`, at most **4 in flight**; when synced with every connected
  holder and flushed, `removeFromCache` unless the document is open (resident non-open documents ≤
  **16**, i.e. ≤ 5 MiB at the measured 310 KiB). An admitted document that a peer later pushes is
  loaded on demand by the Repo (access true), then unloaded again by the same rule. Measured cost
  justifies it: 10 000 resident documents would need 0.5–3 GiB.

## 4. Numeric bounds

| State | Bound | Basis |
|---|---|---|
| Variants admitted | ≤ 32 per writer **ever** (token `N`) | grow-only, numbered tokens; assumption flagged |
| Text nodes (ADR 0023 lot) | ≤ 2 048 per writer ever | same mechanism; replaces a global truncation |
| Admitted ids | ≤ 2 + W·2 080 | sum of categories (point 9) |
| `#hasRequested` keys, handles, synchronizers, stored documents | ≤ admitted, for the project's life | only admitted ids reach the Repo; admitted is grow-only and quota-bounded |
| Resident documents not open | ≤ 16 (≈ 5 MiB) | measured 56–310 KiB each |
| `#ephemeralSessionCounts` | ≤ peers × adapter instances | rewrite |
| Presence state | ≤ 3 channels, ≤ 256 chars total, per peer per document | schema |
| Ephemeral | 64/s, burst 128, 1 KiB | key repeat; measured 265 B |
| Handshake | 1 KiB; 3 burst, 1 per 20 s, 1 pending | §3.5 |
| Hold queue | 4 per (peer, doc); MaxFrame + 1 MiB per peer; 2 × MaxFrame total | §3.6 |
| Sync-state files | 0 | `isEphemeral` |
| Listing store | ≤ W·2 080 × 160 B | token size |
| Go attestations | ≤ W × admitted (per-signer cap = admitted) | quadratic in W: risk |

## 5. The structural guard is a prerequisite (point 3)

What admission covers: **who** may make the Repo hold state, and **how much** (any member,
readers included). What the guard (ADR 0030 §4, G1–G10) covers: **whether a writer's change is
well formed** (no panic, no divergence, save → load passes). Coverage (ADR 0018) covers **who wrote**.
None of the three replaces another.

Exposure analysis: `main` accepts and stores any document any member pushes
(`sharePolicy: () => true`), with no guard. This design pushes and pulls only admitted documents, so
the exposure to a malicious writer's poisoned document is a strict subset of `main`'s for the
push path; but the **pull replicator** makes every peer fetch every listed document without anyone
opening it, which spreads a poisoned variant faster. Hence the ordering:
- Stage 1 (admission bounds, push replication of admitted ids): may merge before the guard; it
  removes reader-level exhaustion and does not widen exposure.
- **Stage 2 = ADR 0030 guard, G1–G9 per change + G10 for whole documents + the PanicError
  containment ("trou de la garde": drop, reload from the Go store, log)**, with T1–T4. Required
  before Stage 3 and before any release that claims S1 closed.
- Stage 3 (signed listing, pull replicator, bounded residency) only after Stage 2.

## 6. ADR: required (ADR 0031)

Amends ADR 0018 (ephemeral messages are bounded and schema-restricted, not merely "allowed for
readers"), ADR 0030 (receive chain order; the listing is a third signed log next to members and
decisions), ADR 0023 (text-node admission by listing with per-writer quotas instead of a global
truncation). Draft decision text:

> **Décision.**
> 1. **Frontière d'admission.** Tout message d'un pair passe, dans l'adaptateur réseau, par :
>    décodage strict → admission → couverture (ADR 0018) → garde structurelle (ADR 0030) → `Repo`.
>    Seuls `arrive`, `welcome`, `sync`, `request`, `ephemeral`, `doc-unavailable` passent, champ par
>    champ, bornés, et le message est reconstruit. Un document hors de l'ensemble admis est jeté sans
>    réponse.
> 2. **Ensemble admis = annonces signées.** Un document appartient au projet s'il est la racine,
>    l'index, ou s'il porte une **annonce** (`Listing`) signée pour le projet par une personne qui a
>    eu le droit d'écrire, numérotée de 1 à Q par personne et par sorte (32 variantes, 2 048 nœuds).
>    L'ensemble ne fait que croître ; seule une décision d'admin de purge en retire un document.
>    Go vérifie, garde (`internal/listing`), diffuse, et le transmet à la webview (`Listed`). La liste
>    `meta.variants` ne sert plus qu'à l'affichage. Un document d'un autre projet n'est jamais admis.
> 3. **Éphémères.** Présence seulement (schéma `name`, `anchor`, `head`), ≤ 1 Kio, 64/s (rafale 128)
>    par pair ; session et compteur réécrits par l'adaptateur ; aucun relais sortant.
> 4. **État de synchro.** Tout pair est traité comme éphémère : aucun état de synchro n'est
>    enregistré ; ceux d'avant sont effacés par Go à l'ouverture.
> 5. **Contre-pression.** Les messages d'un document pas encore prêt attendent dans une file bornée
>    par pair ; au-delà, le lien avec ce pair repart de zéro.
> 6. **Réplication.** Chaque document admis est récupéré dès qu'il est connu, au plus 4 à la fois, et
>    au plus 16 documents non ouverts restent en mémoire.
> 7. **Ordre.** La garde de l'ADR 0030 précède la réplication active (étape 3).
>
> **Options écartées.** Politique de partage seule ; filtrage par Go du format d'automerge-repo
> (ADR 0002, 0011) ; liste `meta.variants` tronquée (déplaçable par un seul écrivain, non
> attribuable) ; arborescence disque (exclut les variantes) ; op de la liste de membres (ADR 0030 O3) ;
> correctif local d'automerge-repo (gardé en repli si un test de contrat le justifie).
>
> **Confirmation.** Tests de contrat sur automerge-repo épinglé ; tests de bornes ; fuzz du décodeur
> d'admission et de `listing.Decode` ; e2e « une variante survit au départ de sa créatrice ».

Also: `docs/architecture.md` (package `listing`, kinds `Listed`, `AttestRefused`),
`docs/exigences.md`, `docs/open-questions.md` (Q = 32), `PROGRESS.md`; upstream PRs.

## 7. Ordered TDD plan

Vitest in `frontend/src/net` (memory checks with `NODE_OPTIONS=--expose-gc`, always paired with a
structural count via spies); Go via `scripts/heavy.sh`. Each step: failing test, code, review.

**Stage 1: admission bounds (frontend; merges first)**
1. Contract tests on automerge-repo 2.5.6 (`net/automerge-contract.test.ts`): session dedup key and
   count rule; `hasRequested` before `access`; `isEphemeral` skips save and load; `#pendingSyncMessages`
   used while loading; ephemeral relay. They decide whether the fallback patch is ever needed.
2. Strict decoder: table of bad payloads (`f6`, `f7`, number, array, unknown and `remote-*` types,
   extra keys, `__proto__`, long ids, wrong `data`, bad `count`/`sessionId`) → dropped with a reason,
   no throw; fast-check: never throws, accepted inputs round-trip.
3. P2: 50 000 `request`s with invented ids → `shareConfig.access` spy **0 calls**, `repo.handles`
   empty, heap < 4 MiB (was 15.5 MiB).
4. **Admitted-id churn (points 1, 10)**: a Stage-1 interim set (root + `meta.variants`) with a
   per-session cap of 256 distinct ids ever admitted; a writer lists and unlists 10 000 ids → access
   called for ≤ 256 distinct ids, handles ≤ 256; repeated across 3 Repo restarts on the same store →
   stored documents ≤ 256 per restart (interim residual, closed in Stage 3, see §8).
5. P1: 100 ephemerals with 1 MiB sessionIds → dropped; 10 000 valid ephemerals with distinct
   sessions → the Repo sees the adapter's single session (spy); heap < 2 MiB (was ~114 MiB); lower
   count dropped; restarted session accepted; 1 000 messages in 1 s → ≤ 192 delivered (fake timers);
   bucket refilled after PeerDown and after re-arrive; unknown channel dropped; relays not sent.
   **Asymmetric reconnection**: B's webview restarts (new session, count 1) while A's link stays up →
   A delivers B's messages; **adapter recreation** on the same Repo → new session, delivered.
6. P3 and point 7: 40 re-arrives with 40 storageIds → **no** `sync-state` key written (fake storage
   spy); with a storage whose `save` is delayed 1 s, no sync-state key appears after the test either;
   Go: `TestOpenRemovesSyncStates` (dirs removed before attach, idempotent). **Remote storage loss**:
   B wipes its store and reconnects → full resync, A's doc unchanged.
7. Backpressure (point 2): storage `load` blocked forever + 10 000 sync messages of 64 KiB for an
   admitted document → held ≤ 4, retained heap < MaxFrame + 1 MiB, peer reset once; unblock → drains
   in order; PeerDown during the block → queue dropped (spy), no delivery afterwards.
8. R1 by push: `shareAdmitted` with the interim set; **ZZ-2 permanent e2e** in
   `frontend/e2e/history.spec.ts` ("une variante créée après l'arrivée survit au départ de sa
   créatrice", wait on a test hook exposing the variant's heads at Bob, never a sleep) + ZZ-1. Fails on
   the candidate fix, passes here.

**Stage 2: ADR 0030 structural guard** (its own plan; T1–T4 of ADR 0030 are its tests).

**Stage 3: signed listing (Go + frontend), after Stage 2**
9. `internal/listing`: golden vectors, `FuzzDecode`, `Verify` (wrong project, `N` > Q, never-writer,
   ever-writer after revocation still valid, equivocation → lower Doc wins on 3 replicas in all
   delivery orders), store persistence round trip, `Sync` convergence on netsim.
10. **Fairness (point 4)**: writer W signs 32 variant listings + tries a 33rd (`ErrQuota`); W forges
    `N` = 1..10 000 with its key → only 32 valid; meanwhile honest H creates 5 variants concurrently
    → all 5 admitted on every replica, none of W's displaces them; W deletes H's `meta.variants`
    entries → H's documents stay admitted (row hidden only); revoking W keeps W's 32 listed (no
    hiding), and blocks new ones.
11. Go wiring: `setLive` fed by listing; `TestTreeDoesNotFeedLiveSet`; `AttestRefused` emitted for a
    non-admitted document and the adapter retries after `Listed` (TS order test on the fake socket);
    `Notices` of another project's epoch ignored.
12. Replicator: ≤ 4 in flight, ≤ 16 resident non-open (spy on `removeFromCache`), a listed id held by
    nobody stays `unavailable` without retries, a later push makes it ready; heap with 200 admitted
    documents < 16 × 310 KiB + margin.
13. Trust store reopen fixes (Go, may land in the current S2/S3 lot).
14. Docs and reviews: ADR 0031 (scribe on §6), `security-auditor` (Opus, high) re-running R1/R2/R5
    harnesses, `go-reviewer` on the Go lots.

Routing: Stage 1 one Sonnet/medium frontend agent; Stage 3 `go-dev` (Sonnet/medium) then the
frontend agent; Stage 2 per ADR 0030's own plan.

## 8. Residual risks (with severity per rubric)

- **Between Stage 1 and Stage 3** the admitted set is the interim root listing with a per-session cap
  of 256 distinct ids: a hostile writer can exhaust the cap for the session (no new variants admitted
  until restart) and hide rows by deleting list entries. **Majeur, temporary**, bounded, recovery =
  remove the writer; closed by Stage 3 (tests 10). Not worse than `main`, which has no bound at all.
- Attestation memory is quadratic in writers (≤ W × admitted × 4 KiB): fine for tens of writers;
  measure with ADR 0028's endurance runs before raising Q.
- Q = 32 variants per writer is an assumption; raising it is compatible.
- automerge-repo upgrades (2.6 changes ephemerals): contract tests must pass before bumping; fallback
  patch ready.
- `isEphemeral` adds ≈ 1.25 B and ≈ 2 µs per change per document per reconnect (measured).

## Changes after review (Codex, 11 points)

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
