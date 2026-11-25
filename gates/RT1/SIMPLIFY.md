# RT1 — /simplify and root causes (step 2, 2026-11-25)

## Root cause of each change

The defect (plan §1, CI run 45542633229): a transient read failure and a proven absence were the same signal to the
start, nothing below the start refused to overwrite a root, and two live pages took the store stream from each other.
Each change removes one of those three causes.

| Change | Root cause it removes |
|--------|-----------------------|
| `workspace.svelte.ts` `openStart` asks `Projects.RootState()` after a failed load; creates only on `absent`, through `ReplaceMissingRoot(saved, fresh)` before `open()` (A-1, A-3); otherwise says `start.rootUnreadable` and writes nothing (A-4). | A rejected `find()` was read as « gone ». Only Go, which holds the storage, can tell absence from a cut read. |
| `GoStorageAdapter` resends a read cut by a stream close, once (`StreamClosedError`); writes never (A-2). | automerge-repo 2.5.6 turns one failed storage read into a failed `find()` that never settles on a second try in the same Repo (measured, plan §1.3); the retry belongs to the adapter (PR #679). |
| `project.SetRootURL` refuses another root (`ErrRootRecorded`); `ReplaceMissingRoot` replaces only a root whose state is `absent`; `RootState` is three-way, an error is `unknown` (B-1..B-3); `Projects` forwards; `Session.stored` uses `RootState`. | `SetRootURL` overwrote any root: the last line of defence was missing where the root is written. |
| `StatusSuperseded` (9), vector `superseded push`; `StoreServer.Serve` offers it to the retired page before closing it (`tell`, `TrySend`, bounded wait for the page's turn) (C-1). | The retired page saw a plain close, reconnected on its 4 s refresh and took the stream back. |
| The adapter, told it is superseded, fails every request with `StoreSupersededError`, never reconnects, and the page says `store.openElsewhere` once (`watchSuperseded`) (C-2). | Same: a page must know it lost the store and stay down. |
| Session lock `turn` is a one-slot channel, not a mutex. | `tell` must wait for a reply in progress with a bound: `TestWC1W5StoreAfterReload/BlockedWriteAndAbandonedReply` showed that a blocking notice deadlocks on an abandoned page whose buffer is full. |
| Tests adapted to the new rules, not weakened: `TestLatest` and `TestSetRootURL` (renamed `TestReplaceMissingRootPersists`, review m4) change a root through `ReplaceMissingRoot` (B-1); `protocol.test.ts` and `TestStoreCodecStrictness` move the first unknown status to 10; `projectChange.test.ts` « PC1 a write left unanswered… » lets the cut read go out once more (A-2); `lifecycle.test.ts`'s adapter stub gains `onSuperseded`. | Each encoded the behaviour RT1 changes on purpose. |

## /simplify (four angles: reuse, simplification, efficiency, altitude)

Applied:
- `#call` built the request in both branches of a ternary: one `sent`, the retry chained on reads only (MR14 anchor
  resynced).
- `StreamClosedError` built twice in `onclose`: once.
- The superseded callbacks are cleared once called: the page never reconnects.
- `tell` allocated a timer on every supersede: only when the page's turn is taken.
- The `TrySend` shape was written twice (`tell`, `cmd/inkwell/main.go`): one exported `bridge.TrySender`, checked
  statically against `*application.StreamConn`.

Declined, with the reason:
- *Keep `sync.Mutex` and skip the notice on `TryLock` failure*: a live page in the middle of a reply (its 4 s refresh)
  would miss the notice: exactly the race RT1 closes. The channel lock costs nothing measurable on a sequential stream.
- *One Go call instead of `RootState` then `ReplaceMissingRoot`*: the plan's rules A-1 and A-3 and their frozen tests
  name both; the second check is the guard of B-2, not a duplicate; the path runs only after a failed start.
- *`SetRootURL` of the same URL returns without writing*: `open()` calls it on every start, and that write dates
  `project.json`, which `project.Latest` reads to open the last project: a behaviour change outside RT1.
- *Typed enum for `RootState` over the binding*; *a key-existence probe instead of `LoadRange`*; *compute the state
  outside the project lock*: the path runs once per failed start or completed join; the lock is what makes the check
  and the write one step (B-2). Not worth an API change in this block.
- *One lifecycle event for the adapter (live, leaving, superseded)*; *`TrySend` on `Conn`*: outside this diff (every
  stream server and fake would change).
- *Drop A-2 now that C-2 holds*: a stream also ends when Go drains or closes it, not only when superseded; A-2 is the
  plan's rule.
- The repeated unknown-status assertions of `protocol.test.ts` predate RT1; left as they are.

## Blind code review (2026-11-25, « accept with fixes »): findings and fixes

Each fix went test first (red, then green), except m5, a characterization of the bound already in place.

| Finding | Test (red before the fix) | Fix |
|---------|---------------------------|-----|
| **M1** (major): at the start, the page's `loadRange` quarantines a damaged chunk and answers OK with what is left; `find()` fails, `RootState` then read `absent` and `ReplaceMissingRoot` replaced the root in the same start (a fork from the peers of a joined project). | `TestRT1B3QuarantinedRootIsNeverAbsent` (`internal/project`): load first, then `RootState` twice and `ReplaceMissingRoot`; `TestRT1QuarantineKeepsDocumentID` (`internal/store`); `TestRT1B3JoinRefusedWhileRootUnknown` calls `CompleteJoin` a second time, after the quarantine. | `store.KV.quarantine` names the file `<document id>~<random>-<last>` (`~` is no key character); `KV.Quarantined(id)`; `RootState` answers `unknown` while quarantine holds the id, so `Session.stored` and `ReplaceMissingRoot` refuse too. |
| **m2**: a third page C arriving while B waits for A's request: B held its own turn while waiting, so C's bounded wait ran out and B, a live page, was closed without the notice. | `TestRT1C1ThirdPageTellsTheWaitingOne` (synctest, A's write outlasts the bound). | `Serve` takes the new session's turn only after `<-prev.done`; a push that comes meanwhile may arrive twice, being pending too. |
| **m3**: a refused `ReplaceMissingRoot` left the fresh document orphaned in the store and said nothing. | « RT1 A-3 a refused replacement removes the fresh document and says so ». | The refusal is caught: `repo.delete(fresh)` (automerge-repo removes its chunks), `start.rootUnreadable`, no `open()`. MR4 and MR11 anchors resynced (one `unreadable` helper). |
| **m4**: test names said `SetRootURL` for a `ReplaceMissingRoot`. | — | `TestSetRootURL` → `TestReplaceMissingRootPersists`, messages renamed. |
| **m5**: the stuck-past-the-bound path had no test. | `TestRT1C1StuckPageClosedAfterBound` (synctest): served after 1 s, the stuck page closed without the notice (R-RT1-5). | — (green at once: it documents the bound). |

## Residuals (each with its block)

- **R-RT1-1 → E2E1**: WebKit caret jump in W58a-E2E (`foreign-project.spec.ts:106`), unproven, separate.
- **R-RT1-2 → RT2**: a corrupt document with no healthy backup stays locked and is reported, never replaced (the
  quarantine name keeps the document id; `RootState` answers `unknown` while quarantine holds it; review M1 below).
  RT2 gives the person a way forward inside Inkwell.
- **R-RT1-3 → RT2**: the join retry loop calls `find()` again after a failed read (A-2 makes it rare, not impossible).
- **R-RT1-4 → RT2**: a root absent locally may live at the peers of a joined project.
- **R-RT1-5 → RT2**: a retired page whose Send stays stuck for more than `supersedeWait` (1 s) is closed without the
  notice; if it later reads again it reconnects once, as before RT1. Only a page that stopped reading reaches it.
