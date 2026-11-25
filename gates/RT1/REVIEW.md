# RT1 — reviews

## Round 1 — blind pre-freeze review (Opus, standard tier), 2026-11-25: « freeze after fixes »

Every finding was applied; none was declined.

| # | Finding | Change |
|---|---------|--------|
| 1 (major) | The e2e cap was set on an estimate of the old flow only. | Measured a stub of RT1-E2E (same steps, failing at its last assertion) 5 times per engine: 12.9 s a run (Chromium), 14-15 s (WebKit); old flow 3.6 s / 11 s (from the first run). The e2e criterion now runs RT1-E2E 15 times and the old flow 20 times per engine, in separate `-g` runs, each with its exact count of expected results; cap 1200 s against ≈ 940 s estimated (build ≤ 180 s included). The other caps were rebalanced (mutants 800 s for 14 mutants); total 2700 s = `max_minutes` 45. Stub removed after the measurement. |
| 2 (major) | C-2 changes every page of the `server` build; the local gate cannot show the suites that open several pages or reload. | Plan §2 « Merge condition » and the gate header: a green GitHub run of `scripts/candidate.sh RT1` (`ci-e2e` and `e2e:netsim:impaired`) before the merge; the ADR amendment repeats it; the `docs` criterion checks the plan states it. |
| 3 (major) | A-1 had no test for a rejected `RootState` call. | A-1 now covers it; test « RT1 A-1 a RootState call that rejects … » and mutant MR10 (`.catch(() => "absent")`). |
| 4 | A-3 contradicted `open()`, which always calls `SetRootURL`. | A-3 now asserts the call order: `ReplaceMissingRoot(saved, fresh)` resolves before any `SetRootURL`; `open()` keeps recording, B-1 lets the same URL through. Test title changed to « … before any SetRootURL ». |
| 5 | The guard belonged where the root is written. | B-1, B-2, B-3 live in `internal/project` (`project.SetRootURL`, `project.go:304`, `ReplaceMissingRoot`, `RootState`); `Projects` only forwards. The Go tests of B-1..B-3 moved to `internal/project`. |
| 6 | Empty and invalid saved roots were undefined. | A-1 and B-3: an empty or invalid saved root is `absent`; empty → `SetRootURL` (first root), invalid → `ReplaceMissingRoot`. Test « RT1 A-1 an empty or invalid saved root is absent … ». |
| 7 | A-4, C-2 « says once », B-3 on `Session.stored`, A-2 « writes never resent » had no proof that bites. | Mutants MR11 (A-4), MR12 (C-2 says once), MR13 with `TestRT1B3JoinRefusedWhileRootUnknown` (`internal/app`, `join.go:518`), MR14 with « RT1 A-2 a write cut by a stream close is never sent again ». |
| 8 | RT1-E2E's role was implicit, and page 1 reconnecting afterwards went unchecked. | E-1 states that RT1-E2E proves C-1 and C-2 end to end, and that page 1 opens no `store` stream after the notice (its `POST /wails/stream/send` naming `store`, counted as in `e2e/bridgeTap.ts`). |
| 9 | Two residuals were missing. | Filed under a new block **RT2** in `PROGRESS.md` (point 1, after RT1): R-RT1-2, a stored but corrupt document locks every start; R-RT1-3, the join retry loop (`workspace.svelte.ts:681-688`) calls `find()` again after a failed read and hangs. The peers residual (formerly « → PC1b », a merged block) became R-RT1-4 under RT2. The `docs` criterion checks all three. |
| 10 | The ADR amendment hid a cost. | ADR 0020 amendment, « Conséquence assumée » : a superseded page's unsaved edits are dropped, in the `server` build only; checked by `docs`. |
