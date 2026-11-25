# RT1 — a project's root is kept unless storage proves it absent

Status: **gate frozen 2026-11-25 (`2eacd24`), step 2 implemented; merge after a green candidate run (§2)**. Rigour: standard (local data of the person, no peer bytes; one live page).
Gate `gates/RT1.toml`, witnesses `gates/RT1/WITNESSES.md`. Place: `PROGRESS.md`, « Cap à long terme », point 1, before
any other block. Store-stream rule: amendment « RT1 » of ADR 0020.

## 1. The defect, proven

CI run 45542633229 (`ci-e2e`, WebKit, `frontend/e2e/persistence.spec.ts:37`), reproduced at rest 1 time in 15:

1. Page 1 re-runs `refresh()` every 4 s (`frontend/src/App.svelte:582`), a store request (file list).
2. The fresh page opens its store stream; `StoreServer.Serve` attaches it and closes page 1's (`internal/bridge/
   storestream.go:419`). Page 1's next request reconnects (`GoStorageAdapter.#connect`) and in turn closes the fresh
   page's stream, its start load still in flight (`store stream closed`; the poll then sees Wails' 410).
3. automerge-repo 2.5.6 `findWithProgress` turns any storage exception into state `failed` and `find()` rejects
   (`dist/Repo.js:392-527`). Only a storage that answers « nothing » and no peer gives `unavailable` (« Document … is
   unavailable »). Measured 2026-11-25 (probe in the shape of `frontend/src/store/transientLoad.test.ts`): a second
   `find()` of the same id **in the same Repo never settles** (the failed progress is cached with the handle still
   `loading`), so a retry must not go through `find()` again.
4. `openStart()` (`frontend/src/workspace/workspace.svelte.ts:642-650`) reads any rejection as « gone », creates a
   document and `open()` records it with `Projects.SetRootURL` (`internal/app/projects.go:63`), which overwrites any
   root: the saved document is orphaned in `.inkwell/repo/`.

Root cause: a transient read failure and a proven absence are the same signal to the start, and nothing below it
refuses the overwrite. Two live pages on one store are the trigger.

**Why two pages.** In the desktop app only the main window runs `App.svelte` and the store stream: the public window
(`/?view=public`, `cmd/inkwell/windows_desktop.go`) mounts `PublicWindow.svelte` and opens only `present`
(`frontend/src/main.ts`); `restart.html` opens no stream. A reload in a window is retired by Wails itself: page
generations per window, an older generation gets 410 and closes its sockets (Wails v3.0.0
`pkg/application/stream_transport.go:147-210`, `@wailsio/runtime` `stream.js:55-75,740`). Two live store pages exist
only in the `server` build (each browser context is its own Wails session): the e2e and netsim suites, and a person
who opens a second tab. The fix still holds for the desktop: a Go storage read error reaches the same branch.

## 2. Rules

| Id | Rule | Proof |
|----|------|-------|
| A-1 | `openStart` creates a document only when `Projects.RootState()` answers `absent`; `stored`, `unknown` or a rejected `RootState` call never creates nor records a root. An empty saved root is `absent` and is recorded by `SetRootURL` (first root); an invalid saved URL is `absent` and is replaced by A-3. | Vitest |
| A-2 | `GoStorageAdapter` sends again, once, on the next stream, a **read** (`load`, `loadRange`) cut by a stream close; a **write** is never sent again (today's `onLeaving` rule). Reads are idempotent; automerge-repo leaves retry to the adapter (PR #679). | Vitest |
| A-3 | The absent path awaits `Projects.ReplaceMissingRoot(saved, fresh)` before `open(fresh)`, so before any `SetRootURL` (which `open()` keeps calling; B-1 lets the same URL through). | Vitest (call order) |
| A-4 | A start that cannot open a stored root writes nothing, keeps the editor locked and says so (`notice`, role `alert`, key `start.rootUnreadable`: nothing was changed, restart Inkwell). | Vitest |
| B-1 | `project.SetRootURL(url)` (`internal/project/project.go:304`) records a root only when none is recorded or `url` equals it; otherwise `ErrRootRecorded`, `project.json` untouched. `Projects.SetRootURL` only forwards. | Go |
| B-2 | `project.ReplaceMissingRoot(old, new)` replaces, under the project's lock, only when the recorded root is `old` and its state is `absent`. | Go |
| B-3 | `project.RootState(url)` → `stored` (a `snapshot` or `incremental` chunk under its id), `absent` (none, or `url` empty or not a valid root URL) or `unknown` (a `LoadRange` error: corrupt, I/O; or quarantine holds a chunk of it, its file name keeping the document id), never `absent` on an error. `Session.stored` (`internal/app/join.go:518`, CompleteJoin) uses it: `unknown` refuses the join. `Projects.RootState()` is the binding. | Go |
| C-1 | `StoreServer.Serve`, replacing a live stream, first pushes `StatusSuperseded` (id 0, status 9, empty payload) to it, then closes it. Shared vector `superseded push` (status `Superseded`, wire `0000000009`) in `internal/bridge/testdata/storestream.json`, read by `TestStoreVectors` and `protocol.test.ts`. | Go + vector |
| C-2 | An adapter that receives `StatusSuperseded` is terminal: pending and later requests fail with `StoreSupersededError`, it never reconnects, and the page shows `store.openElsewhere` once, however many requests fail after. Only one live page owns the store. | Vitest |
| E-1 | `RT1-E2E` proves C-1 and C-2 end to end: page 1 keeps issuing store requests while a fresh page starts; page 1 shows `store.openElsewhere` and opens no store stream afterwards (its `POST /wails/stream/send` naming `store`, counted as in `e2e/bridgeTap.ts`), the fresh page shows the saved text, and `project.json`'s `rootUrl` is unchanged. The original persistence flow is repeated under WebKit and Chromium. | e2e |

The check and the write of B-2 run under the project's lock; the only writer of chunks is the live page, which is the
caller, so the proof cannot go stale between them (one page owns the store, C-2).

**Merge condition.** C-2 changes how every page of the `server` build behaves when a second page or a reload arrives
(the `ex0`, `save`, `project-change`, `history`, `preview` and netsim suites open several pages or reload). RT1 is merged
only after a green GitHub run of its candidate, `scripts/candidate.sh RT1` (`ci-e2e` and `e2e:netsim:impaired`).

## 3. Alternatives weighed

- **Retry `find()` with back-off in `openStart`**: refuted by the measurement in §1.3 (never settles in the same Repo).
- **Read the error text** (« unavailable » vs other): automerge-repo has no typed error yet (`DocumentLoadFailedError`
  is only announced in PR #679); a page also cannot see Go's storage. Go, which owns the storage, decides (B-2, B-3).
- **Oldest stream wins** (refuse a new stream while one is live): breaks a reload in the `server` build, where the old
  page may still be attached when the new one connects. Newest wins, the loser is told (C-1) and stays down (C-2).
- **Web Locks / BroadcastChannel between tabs** (MDN Web Locks API; automerge-repo's `BroadcastChannelNetworkAdapter`):
  only for same-origin tabs of one browser, not two Playwright contexts nor two engines; Go is the one arbiter.

## 4. Mutants (written at step 2 with the code, `scripts/netsim/mutants.toml`, block RT1)

| Id | Kind | Mutation | Caught by |
|----|------|----------|-----------|
| MR1 | vitest | `openStart` creates on any rejection (state not asked) | A-1 stored |
| MR2 | vitest | `unknown` treated as `absent` | A-1 unknown |
| MR3 | vitest | an interrupted read is not sent again | A-2 read |
| MR4 | vitest | the absent path calls `SetRootURL`, not `ReplaceMissingRoot` | A-3 |
| MR5 | vitest | the adapter reconnects after `StatusSuperseded` | C-2 never reconnects |
| MR6 | go | `SetRootURL` accepts another root | `TestRT1B1SetRootURLNeverReplaces` |
| MR7 | go | `ReplaceMissingRoot` skips the state check | `TestRT1B2ReplaceMissingRootOnlyWhenAbsent` |
| MR8 | go | `RootState` maps a `LoadRange` error to `absent` | `TestRT1B3UnknownIsNeverAbsent` |
| MR9 | go | `Serve` closes the old stream without the notice | `TestRT1C1SupersededToldBeforeClose` |
| MR10 | vitest | the `RootState` call's rejection read as absent (`.catch(() => "absent")`) | A-1 RootState rejects |
| MR11 | vitest | the unreadable-root notice is not shown | A-4 |
| MR12 | vitest | `store.openElsewhere` shown on every failed request | C-2 says once |
| MR13 | go | `Session.stored` reads `unknown` as stored | `TestRT1B3JoinRefusedWhileRootUnknown` |
| MR14 | vitest | an interrupted write is sent again too | A-2 write |

## 5. Out of scope, residuals

- **R-RT1-1 → E2E1**: W58a-E2E under WebKit (`frontend/e2e/foreign-project.spec.ts:106`, passed on retry): the local
  caret jumped to 0 as the host's remote caret appeared. Suspect `frontend/src/editor/remoteCursors.ts`, unproven
  (0 in 16 at rest); a separate defect, not a store-stream effect.
- **R-RT1-2 → RT2**: a corrupt document with no healthy backup stays locked and is reported, never replaced: its
  damaged chunks go to `quarantine/` under a name that keeps the document id, and `RootState` answers `unknown`
  while quarantine holds it (review M1), so every start says `start.rootUnreadable` (A-4). RT2 gives the person a
  way forward inside Inkwell (restore from quarantine or from a peer, or start anew on request).
- **R-RT1-3 → RT2**: the join retry loop (`frontend/src/workspace/workspace.svelte.ts:681-688`) calls `find()` again
  after a failed read, which never settles in the same Repo (§1.3); A-2 makes a cut read rare, not impossible.
- **R-RT1-4 → RT2**: a root absent locally may still live at peers of a joined project; B-2 replaces it as today.
  Whether a joined project should wait for its peers instead follows ADR 0043 (PC1, merged).

## Sources (read 2026-11-25)

automerge-repo 2.5.6 `dist/Repo.js` (`findWithProgress`, `find`); automerge-repo PR #679, storage error handling and
the adapter contract, <https://github.com/automerge/automerge-repo/pull/679>; issue #153, unavailable documents,
<https://github.com/automerge/automerge-repo/issues/153>; Automerge storage docs,
<https://automerge.org/docs/reference/repositories/storage/>; Wails v3 streams and windows,
<https://v3.wails.io/llms.txt>, source `pkg/application/stream_transport.go` (v3.0.0); MDN Web Locks API,
<https://developer.mozilla.org/en-US/docs/Web/API/Web_Locks_API>.
