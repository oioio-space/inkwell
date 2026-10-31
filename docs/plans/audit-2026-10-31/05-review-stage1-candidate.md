# Review of the S1/S4 fix (worktree agent-ade848f3cc140536b, base 17603f5)

Paths are relative to `<wt>/frontend/src`. automerge-repo 2.5.6 (dist/). Proof files (throwaway, scratchpad only):
- `$S/rv/frontend/src/net/zz_review.test.ts` (R1 to R3), `zz_review2.test.ts` (R5): vitest, real `Repo` + `WailsNetworkAdapter` + `shareHeld`.
- `$S/rv/frontend/e2e/zz-variant.spec.ts` + `pw-review.config.ts`: Playwright, run against the fix build and against a HEAD (pre-fix) build.
  Logs: `$S/e2e-review.log`, `$S/e2e-review2.log`, `$S/e2e-zz.log` (fix), `$S/e2e-zz-base.log` (pre-fix), `$S/e2e-zz2.log` (fix, rerun).
  ($S = dossier de travail temporaire de la session du 2026-10-31, non conservé ; le test ZZ-2 est repris dans `frontend/e2e/history.spec.ts` par l'étape 1)

## 1. Adversarial review

### What the fix does close
- Coverage creates no per-document state for unattested ids (filter uses a throwaway DocCoverage); `#lastRewritten` is
  now a digest, capped at 256 per peer. The 279 MiB A04 reproduction no longer works through Coverage.
- `shareHeld` stops `repo.find()` on invented ids: `repo.handles` stays empty (checked in R1: handles=0 after 50,000 requests).

### P1: ephemeral messages bypass everything. CRITIQUE (the S1 class is still open)
`#enforce` passes every non-sync/request message through unchanged (net/WailsNetworkAdapter.ts:342), and `isPeerMessage`
returns true for every other type without checking it (:396). automerge-repo `NetworkSubsystem` then does
`this.#ephemeralSessionCounts[`${senderId}:${sessionId}`] = msg.count` (NetworkSubsystem.js:54-57). The map is never
pruned, and both the key (sessionId) and the value (count) are any CBOR value the peer chooses.
Proof R2: a reader with no attestation sends 100 ephemeral frames, each with a distinct 1 MiB sessionId. **About 114 MiB
is retained after GC.** The control (the same sessionId every time) retains 16 MiB. The cost is about 1:1 per frame, up
to MaxFrame (64 MiB). This is the same impact as A04.
Fix: in `isPeerMessage`, allow only `arrive | welcome | sync | request | ephemeral | doc-unavailable`. For an ephemeral
message require `sessionId` to be a string of 64 characters or fewer, `count` to be a safe integer, `documentId` to be a
valid id, and `data` to be a Uint8Array. Accept only one sessionId per peer per link: keep the last one in the adapter
and drop any other until PeerDown or a re-arrive. Without that rule, short distinct sessionIds still grow the map at
roughly 100 B per message. Also drop ephemeral messages for documents that are not held (no need to wake the Repo).

### P2: `CollectionSynchronizer.#hasRequested` grows with invented ids. MAJEUR
The Repo records `#hasRequested.set(documentId, Set{sender})` before the access check
(CollectionSynchronizer.js:83-89). `removePeer` only empties the Sets; the keys stay. Proof R1: 50,000 request messages
of about 200 B, each with a distinct invented id of 128 characters or fewer, retain **15.5 MiB (326 B per message)**.
The control (one repeated id) retains 1.2 MiB. The growth is unbounded, but tied to the attacker's bandwidth (about
1.5 times the bytes sent), so it is less severe than P1.
Fix: in the adapter, drop sync/request messages (and ephemeral and doc-unavailable) whose documentId the share predicate
rejects, before `emit("message")`. Optionally answer `doc-unavailable` from the adapter. No record is needed: once the
document is held, `announce` returns true and the Repo starts the sync itself.

### P3: `storageId` cycling leaves persistent sync-state on disk. MAJEUR (already present before this fix, not caused by it)
`peerMetadata` from the handshake is not validated (:370). A reader that sends `arrive` again gets "peer-disconnected"
followed by a new "peer-candidate" (:317). It does this with a new `storageId` each time, then sends one sync for a held
document. Each cycle leaves:
- a new `[doc, "sync-state", storageId]` file on disk (StorageSubsystem.saveSyncState, never collected);
- a new `#throttledSaveSyncStateHandlers` entry and a new `DocHandle.#syncInfoByStorageId` entry.
Each re-arrive also re-attests every document (`#reattest`), which Go then gossips. Proof R5: 40 re-arrives give
**40 persisted sync-state records**. The Go KV validates key components, so there is no path traversal.
Fix: validate `peerMetadata` (storageId a string of 64 characters or fewer, base58; isEphemeral a boolean). Pin a
peer's storageId for the life of its identity: refuse a change, or treat a peer that changes it as ephemeral. Rate-limit
re-arrives per peer.

### P4: prototype keys pass `shareHeld`. MINEUR
`doc in repo.handles` (workspace/workspace.svelte.ts:169) is true for "constructor", "toString" and similar keys, because
`#handleCache` is a plain `{}`. In practice the later `find()` throws (the id is not valid base58check) and the error is
caught, so there is no state and no crash. It still lets these ids pass the access check (R3).
Fix: use `Object.hasOwn(repo.handles, doc)` and require `isValidDocumentId(doc)` in `isPeerMessage` (the current rule
only checks a length of 128 or fewer).

### P5: unknown message types go through `JSON.stringify`. MINEUR
`isPeerMessage` lets unknown types through. NetworkSubsystem then runs
`this.#log(`invalid message: ${JSON.stringify(msg)}`)` (NetworkSubsystem.js:49). The template string is built even when
debug logging is off. A 64 MiB `Uint8Array` serialises to several hundred MiB of transient string: a CPU and peak-memory
spike per frame. This is fixed by the type whitelist in P1.

### Caps (256 per peer and 4,096 digests overall; 10,000 docs overall and 1,024 per signer)
- There is no eviction of tracked documents, so a malicious writer cannot push out legitimate ones. Good.
- Filling the 10,000-document global cap at 1,024 per signer takes 10 or more colluding writer identities. After that,
  documents that are new in this session (for example a new variant) are refused, and their changes are stripped, until
  restart or revocation. With fewer than 10 writers the per-signer cap prevents this. MINEUR. Revocation releases the
  documents (`revoked` deletes documents left with no attestation).
- The webview cap is 1,024 per signer while Go `project.MaxNodes` is 10,000. A03 (the Go-side cap) is not fixed in this
  diff, so the Go store and the gossip are still unbounded for a writer. Keep both caps the same once A03 lands.
- `revoked()` also deletes `#grown` and `#counted` for documents it drops, which resets the growth budget of a document
  when it is attested again. MINEUR, and only an admin action can trigger it.
- The global loop in `#repeated` stops on `other === peer` (coverage.ts:259). If the current peer is the oldest entry,
  nothing is evicted and the total can go past 4,096. It stays bounded at 256 × the number of active peers (members only,
  identities checked by Go). MINEUR.
- `digest` is FNV, which is not cryptographic. A collision only drops the sender's own message under its own
  (peer, doc) key, so it is not exploitable against others. OK.

### S4 validator coverage. PARTIAL
| Message type | Validated by `isPeerMessage` |
|---|---|
| sync / request | Yes (documentId: a string of 1 to 128 characters; data: a Uint8Array). |
| arrive / welcome | Only `senderId` and `type`. `peerMetadata` is not checked (P3). `projectUrl` is checked later by `isValidAutomergeUrl`. |
| ephemeral | No: sessionId, count, documentId and data are unchecked (P1). |
| doc-unavailable | No: documentId is unchecked. Harmless today (the access check runs first). |
| remote-subscription-change / remote-heads-changed | Pass through, then ignored (gossip is off). |
| unknown types | Pass through (P5). |
`targetId` is not type-checked, but it is compared with `===`, so that is OK.

## 2. Functional regression

How the project's documents travel:
- The project is one root document. The joiner learns its URL from the host's handshake (`projectUrl`) and calls
  `open()`, which calls `repo.find()`. That puts the root in `repo.handles` before the request is sent. The host holds
  the root, so `access` is true and the host serves it. **Joining works.**
- Variants are documents created after joining (`repo.import` in history/versions.ts:183). Their URL is written into
  `meta.variants` of the root document. Other people only `find()` a variant on demand (history.svelte.ts:547).

What changed:
- Before the fix (`sharePolicy` returned true), the creator announced the new document and every peer fetched and stored
  it at once.
- With the fix, every other peer answers `doc-unavailable`. A variant then exists only on the creator and on whoever has
  opened it. **If the creator goes offline before another person opens the variant, that person cannot get it.** This
  is a regression in offline-first availability, and it violates "joining = receiving everything" (N5 / JOIN-3).

e2e results:
- Existing specs on the fix build: join (JOIN-3), collab, history (including "B2 variants"), offline (relay killed and
  restarted), persistence: **7 out of 7 passed**.
- No existing e2e covers "a document created after joining reaches another person". In "B2 variants", Bob only sees the
  variant's row; the merge happens in the root document.
- Throwaway `zz-variant.spec.ts`:
  - ZZ-1 (the creator stays online; Bob opens the variant): passes before and after the fix.
  - ZZ-2 (Carol joins, creates a variant and types in it; her row reaches Bob; Carol stops; Bob opens the variant):
    **passes before the fix** (`e2e-zz-base.log`) and **fails after it, twice in a row**. Bob's editor shows only
    "= Commun", without "Variante de Carole".
- Severity: MAJEUR (a functional regression with measured impact, matching the A04 rubric spirit); the fix must not
  merge as is.

Recommended fix: share the set of documents reachable from the project, not "documents already held".
- The predicate is: id is the root, OR id is listed in `root.meta.variants[].url` (capped, for example to 256 entries,
  ignoring the rest), OR id is held. Use `Object.hasOwn`.
- Proactively `find()` every listed variant URL so documents are again replicated eagerly and persisted.
- Use the same predicate in the adapter's pre-Repo drop (P2), so invented ids still create no state anywhere.

Add an e2e test like ZZ-2 to `history.spec.ts` or `join.spec.ts`.

The Go-side unit tests were not rerun. Frontend unit tests pass on the fix: 150 out of 150 in `src/net` and
`src/workspace`. The one failing suite is an artifact of my scratch copy (`envelope.test.ts` needs `internal/` testdata).
