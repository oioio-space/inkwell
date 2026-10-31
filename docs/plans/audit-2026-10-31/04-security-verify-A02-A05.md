# Adversarial verification of Codex findings A02–A05 (inkwell @ 17603f5)

Rubric: docs/plans/2026-10-31-architecture-audit.md:34-37. Proof tests live ONLY in the scratch copy
`$S/repo` (git archive of HEAD): `internal/app/zz_audit_a03_test.go`, `internal/app/zz_audit_a05_test.go`,
`frontend/src/net/zz_audit.test.ts`. Run: `scripts/heavy.sh go test ./internal/app -run TestAudit -v`,
`NODE_OPTIONS=--expose-gc vitest run src/net/zz_audit.test.ts`. Nothing written to the repo.

## Who can reach the webview's `Kind.Message` path (shared by A02/A04)
- Go relays a repo frame to the webview only if `isActive(peer)` (internal/app/wiring.go:364, :528) =
  peer in `w.full`, set by `activate` (:473-489) when `Admission.Admit` (:54-56: role > 0) and not
  `restricted` (:466-469: not suspended). => **any current, non-suspended member, READER included**.
  Go does not inspect the payload (opaque bytes, ADR 0011). Size cap: transport.MaxFrame ≈ 64 MiB
  (internal/transport/transport.go:36); the TS envelope caps only peer id (64) and attestation docs (256).
- Adapter then requires handshake + `senderId === from` + `targetId === me` (WailsNetworkAdapter.ts:300-327),
  all trivially satisfiable by that member.

## A02 — CBOR null/undefined → TypeError  — **PARTIAL**, severity **Mineur** (not Critique)
- True: `decode(payload)` (WailsNetworkAdapter.ts:295) returns `null` for `f6` / `undefined` for `f7`;
  `msg.senderId` (:302) throws TypeError. Reproduced (zz_audit.test.ts "A02"): throws, `onDrop` not called.
- Refuting the impact: the throw is the first property access, so no state is mutated; the production
  socket is `WailsSocket extends EventTarget` (node_modules/@wailsio/runtime/dist/stream.js:171) and
  delivers via `dispatchEvent` (:345) — listener exceptions are reported, not propagated, so the poll loop
  and later frames are unaffected (test: the same peer's next `arrive` is processed normally). No global
  error handler in src/ escalates it. Net effect = frame dropped (as intended) + console error; diagnostics
  bypassed. Other shapes (number, array, string) fail the sender check; malformed Repo fields are
  re-validated by automerge-repo `isRepoMessage` (NetworkSubsystem.js:48).
- Attacker: any active member (reader included). No crash, no bypass → letter-of-the-rule gap in
  engineering-principles §4 "décodage strict" without measured impact → Mineur.
- Fix: a type guard right after decode:
  `if (typeof msg !== "object" || msg === null || typeof msg.senderId !== "string" || typeof msg.type !== "string") { onDrop("malformed payload"); return; }`
  and, for sync/request, `typeof documentId === "string" && isValidDocumentId(documentId) && data instanceof Uint8Array`
  (this guard is also the cheapest first layer for A04).
- Test: deliver payloads `f6`, `f7`, `encode(1)`, `encode([])`, `encode({type:"sync",senderId:p,targetId:me,documentId:42,data:"x"})`
  → `expect(() => deliver(...)).not.toThrow()` and drops = ["malformed payload", …].

## A03 — writer grows the Go attestation store without bound — **CONFIRMED**, severity **Majeur** (Codex: Critique)
- Path: `onAttest` (attest.go:228) accepts frames from any active peer → `offerAttest` (:256) →
  `List.Verify` (internal/trust/attest.go:128-149: project, field bounds MaxDoc 256 / MaxHeads 64,
  Ed25519 signature, signer role ≥ writer, not suspended) → `att.add` (:42-51) keeps one entry per
  (doc, signer) with **no per-signer or global cap**. The relaying peer may be a reader, but the
  **signer must be a current non-suspended writer** (a reader cannot forge).
- Amplification: each new entry is notified to the webview (:263-265, which creates a DocCoverage per doc,
  coverage.ts:157/257), gossiped to every active peer (:266-271), replayed in full to every newly linked
  peer (`sendAttestations` :276-280, wiring.go:486) and on webview attach (`Notices`, wiring.go:671).
  Not persisted, but re-gossiped by others after a restart. No transport rate limit (mux only bounds the
  queue, mux.go:59-63).
- Proof: zz_audit_a03_test.go — a writer attests `maxPending+1` = 10001 distinct docs; peer A holds
  10001 attestations (PASS). Contrast: the analogous pre-admission maps ARE capped (`keep` :145-149,
  `hold` :290-307).
- Why Majeur not Critique: needs a writer (ADR 0030 does treat a malicious current writer as in scope),
  each item ≤ 4 KiB (MaxAttestationSize), no crash shown, admin recovery exists (remove/suspend prunes,
  wiring.go:424). It violates §4 "taille bornée" with measurable unbounded growth → Majeur.
- Fix: per-signer cap in `attestStore.add`: when `k` is new and the signer already has
  `maxPending` (= project.MaxNodes, a project cannot have more text documents) entries, refuse
  (return an `ErrTooManyDocs`, do not relay, optionally `w.drop(from)` when from != ""); replacing an existing
  (doc, signer) stays allowed. Global bound then = members × MaxNodes. Mirror the cap in `Coverage`
  (see A04). Test: writer attests MaxNodes+1 distinct docs → `len(A.Attestations())` for that signer
  == MaxNodes and the last Attest is not relayed; a fresh Seq for an already-held doc is still accepted.

## A04 — webview Coverage keeps state per arbitrary documentId — **CONFIRMED**, severity **Critique**
- Path: any active member (reader) → handshake → `{type:"sync"|"request", targetId: me, documentId: <any
  string, unbounded length>, data}` → `#enforce` (WailsNetworkAdapter.ts:333-343) → `Coverage.filter`.
  After `decodeSyncMessage` (coverage.ts:188) and before any coverage/role decision, `#doc(doc)` (:193,
  :257-264) creates a DocCoverage; `#counted`/`#grown` get a key per doc (:240, :242); `#rejected` is capped
  per doc only (1024, :78-89) so it is unbounded across docs; and when the message must be rewritten
  (any uncovered change — exactly what a reader sends) the full rewritten message, including the
  peer-controlled `need`/`have` lists, is retained in `#lastRewritten[from\0doc]` (:247-253).
  `forgetPeer` (:167-171) clears only `#lastRewritten`, and only on PeerDown; `#docs/#grown/#counted` are
  never pruned.
- Proof: zz_audit.test.ts "A04" — a reader with **no attestation** sends 300 sync messages (~0.96 MB each:
  one forged change + 30 000 `need` hashes) with distinct documentIds → **~279 MiB retained** after GC,
  `stats.removed = 300`. With MaxFrame ≈ 64 MiB per frame, a few dozen frames suffice to exhaust a
  WebKit renderer.
- Downstream, not fixed by Coverage alone: the Repo is built with `sharePolicy: async () => true`
  (frontend/src/workspace/workspace.svelte.ts:166), so automerge-repo also records and `repo.find()`s
  every arbitrary valid documentId (CollectionSynchronizer.js:90-106) and asks generous peers for it
  (mesh amplification).
- Severity: lowest-privilege member (reader — the very role ADR 0018 defends against) causes unbounded
  memory growth with hundreds of MiB demonstrated → ASVS L2 resource-exhaustion flaw → Critique per rubric.
- Fix (minimal, in the adapter before Coverage and before the Repo): (1) A02 type guard incl.
  `isValidDocumentId`; (2) do not create per-doc state for a sender who cannot write: in `filter`, look up
  `#docs.get(doc)` and, when absent, treat every change as uncovered WITHOUT inserting (a doc only gets
  state through `attested`, which comes from Go-verified writer attestations — bounded once A03 is fixed);
  only count/remember for docs that exist; (3) bound `#lastRewritten` (store a 32-byte digest instead of
  the message, or one entry per peer) and clear all per-peer state on PeerDown; (4) bound `#docs` to
  MAX_NODES (10 000) with LRU eviction of docs with no attested heads; (5) replace `sharePolicy: () => true`
  by a policy that only shares/finds documents reachable from the project index.
- Test: as the proof test but assert retained memory < 10 MiB and that `Coverage` has no entry for the
  300 unattested doc ids (expose a `docCount` getter for tests); plus: after `attested("w", d, h)` the
  doc is tracked and a writer's covered change still passes (non-regression of existing coverage tests).

## A05 — `replace` deletes the old trust store before saving the new one — **CONFIRMED**, severity **Critique**
- Path: only caller `invites.adopt` (internal/app/invites.go:361) during a join to another project.
  `replace` (persist.go:132-142) runs `clear()` (removes every .op/.rcv/.tmp, :145-159) then saves op by op;
  any `save` error (ENOSPC, EIO, permission) returns with the old ops already gone and no rollback. A crash
  in the same window has the same effect (the comment at invites.go:360 "an interruption leaves a valid,
  partial list" is only true once the new genesis is on disk and says nothing of the old list). Not
  remotely triggerable (host-supplied ops are verified and size-bounded); trigger = local I/O fault/crash.
- After the error `adopt` returns "join: …", the device keeps running on the OLD list in memory, so the
  user believes nothing changed; the next start loads the damaged directory (`load`, persist.go:51-71):
  with no genesis it silently mints a fresh self-signed genesis (`trust.Genesis` is deterministic,
  op.go:396-401).
- Proof: zz_audit_a05_test.go (failure injected on the first save via a directory at the target name):
  * creator of A with one extra member: before A (2 ops) → after restart same project id, **1 op** (the
    other member is gone; Admission then refuses them, so the list cannot re-sync);
  * guest of A (genesis by someone else): before A (2 ops) → after restart a **different, self-owned
    project** (1 op). Both PASS = loss confirmed.
- Severity: demonstrated loss of the signed member list → Critique per rubric (likelihood low: needs an
  I/O fault or crash mid-join).
- Fix: stage then swap. Write all new ops into `.inkwell/trust.next/` (WriteAtomic + dir fsync); then
  `Rename(trust, trust.prev)`, `Rename(trust.next, trust)`, `RemoveAll(trust.prev)` via the parent
  `os.Root`; on any error before the swap remove `trust.next` and leave `trust` intact. `openTrust` recovers:
  `trust` missing + `trust.prev` present → rename back; stray `trust.next` → remove. (Alternative if
  directory swap is undesirable: save the new ops first — content-addressed names do not collide — then
  delete only the old ones, plus an atomically written `project` marker naming the current genesis that
  `load` uses to pick the genesis and ignore foreign ops.)
- Tests: (1) injected failure on the k-th save for every k → reopen yields the OLD project with all its
  ops; (2) simulated crash after each rename step (hook like `writeAtomic`'s `beforeRename`) → reopen
  yields either the complete old or the complete new list, never a partial/fresh one; (3) existing
  `TestReplaceSwitchesTheStoredProject` still passes.
