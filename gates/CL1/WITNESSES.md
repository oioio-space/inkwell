# CL1 - red witnesses before the freeze (2026-11-15; worktree `cl1` on main 73d0f4b, no production change)

Each criterion's command was run once where the machine allowed; it must fail for the reason its rule names.

| Criterion | Exit | The failing line | Right reason |
|---|---|---|---|
| docs | 1 | `CL1 docs: ['exigences W32 does not name CL1']` (the four plan rows and the four unchanged texts of scenario 8's C5 are found) | yes: the registry is updated at step 7 |
| frontend | 1 | `CL1 frontend: exit 1, failed 6, missing ['CL1 C-1 …', 'CL1 C-2 …', 'CL1 C-4 …', 'CL1 mutant C-1 …', 'CL1 mutant C-2 …', 'CL1 mutant C-4 …']` (detail below) | yes for the three rules; the three mutants: see below |
| netsim | 0 | not red, by nature: ten runs of ten pass on the unchanged tree (table below). The criterion as first drafted (ten runs as statistical evidence) proved nothing here and was reduced to one run per seed, kept as a non-regression check only | no red witness is possible on this machine (plan, R-1 confirmed) |
| regression | 1 | `Test Files  1 failed \| 7 passed (8)`, `Tests  6 failed \| 63 passed (69)`: the six failures are those of `claim.test.ts` below; Biome clean, svelte-check `724 FILES 0 ERRORS 0 WARNINGS`. (A first attempt at 21:27 exited 124 without a slot, behind another session's exclusive measurement; this one ran under `heavy.sh --exclusive`.) | yes |
| simplify-report | 1 | (file absent) | yes |

## The frontend criterion, test by test (`frontend/src/ui/history/claim.test.ts`; after the blind review: 19 tests, 16 red, 3 green)

| Test (`CL1 …`) | On 73d0f4b | The failure |
|---|---|---|
| `C-1 a change from another device does not re-arm the claim timer` | red | `C-1 a change from another device re-armed the claim timer: expected [] to deeply equal [ 2000 ]` |
| `C-1 every way this device writes arms the claim` | green | kept behaviour (today every change arms); it bites through the mutant « a flag around change » |
| `C-2 a device that made no edit since its last claim sends no claim` | red | `C-2 a device that never wrote claimed: expected 1 to be +0` (the timer assertion, later in the oracle, is not reached today) |
| `C-3 the claim lands CLAIM_PAUSE_MS after the last local edit` | green | a kept rule |
| `C-4 three devices: no claim traffic in the quiescence window of scenario 8` | red | `C-4 claim traffic in the quiescence window: expected [ '0@6420', '0@6440', '1@6420', …(3) ] to deeply equal []` |
| `C-5 a claim owed when the document or the page is left is written` | red | `C-5 the claim owed was dropped when the document was left: expected [] to deeply equal [ 500 ]` |
| `C-5 a claim that could not be written is retried` | red | `C-5 the claim owed was not retried: expected [] to deeply equal [ 4000 ]` |
| `C-5 a claim owed is found again when the document is opened` | red | `C-5 the claim owed was not found again at the opening: expected +0 to be 1` |
| `C-6 the actor a clone gives this device is claimed too` | red | `C-6 the actor after a clone was not claimed: expected +0 to be 1` (its pin passed: the clone has another actor) |
| `mutant C-3 a longer pause is caught` | green | caught today with C-3's text |
| `mutant C-2 arming at the opening is caught` | red | `C-1 a change from another device re-armed the claim timer`: the mutant test first requires C-1's oracle to pass on the mutant |
| the eight other mutants (`C-1 arming on every change`, `C-1 a flag around change`, `C-4 arming on every change`, `dropping the owed claim at detach`, `… when hidden`, `no retry`, `nothing owed at the opening`, `the actor cached at the opening`) | red | `Error: mutant cl1mutant-…: anchor found 0 times in …/history.svelte.ts`: each edits a line of the fix (plan, planned lines), which does not exist yet. Red because the code is absent, not because a rule is seen broken |

## What the failing GitHub run's artefacts say (run `8-editor-s20772`, C5 measured 3, 2, 3, 2, 3, 3)

- `nt-fail-trace/tl.txt`: last keystrokes at 36.84 s (c030), 37.60 s (b030), 38.35 s (a030); convergence read at
  39.20 s (0.0 s); baseline at 42.22 s; end of the window at 72.27 s. A claim re-armed once is due at 42.35 s or later.
- No claim change is visible: `alice.log`, `bob.log`, `carol.log` have no line per document change (`grep -ci claim`: 0)
  and the Playwright traces record no sync message. The cause is shown by the tests above and by this timeline, not
  observed at message level.
- Local, same day: see the next section.
  (plan, residual R-1). → CL1

## The netsim witness on the unchanged tree (21:44 to 22:06, `heavy.sh --exclusive`, strict, exit 0)

Command of the first draft: scenario 8, editor variant, seeds 20771 and 20772, `NETSIM_REPEAT=5`. C1 to C5 pass in
all ten; C5's measure per ordered pair (bound 2):

| Run | Seconds | C5 per ordered pair |
|---|---|---|
| 8-editor-s20771-r1 | 139 | 0 0 0 0 0 0 |
| 8-editor-s20771-r2 | 129 | 2 1 2 2 2 2 |
| 8-editor-s20771-r3 | 131 | 0 0 0 0 0 0 |
| 8-editor-s20771-r4 | 139 | 0 0 0 0 0 0 |
| 8-editor-s20771-r5 | 130 | 0 1 0 0 0 0 |
| 8-editor-s20772-r1 | 130 | 1 1 1 1 1 1 |
| 8-editor-s20772-r2 | 131 | 0 0 0 0 0 0 |
| 8-editor-s20772-r3 | 130 | 0 0 0 0 0 0 |
| 8-editor-s20772-r4 | 130 | 1 0 0 0 0 0 |
| 8-editor-s20772-r5 | 129 | 2 2 2 2 2 2 |

Zero failure in ten: the defect GitHub shows in 8 runs of 15 does not show here (R-1 confirmed). Traffic in the window
does occur locally (four runs of ten, two of them at the bound), it never passes 2. Why the criterion is kept all the
same, reduced to one run per seed (cap 360 s): the change touches when a device writes into the shared document, and
one real three-device exchange per seed shows it breaks no clause; it is not presented as proof of the fix. That proof
is the `frontend` criterion and the next ten GitHub runs of `main` (`gates/CL1/GITHUB.md`, plan « What proves the
fix »). The reduced command was not run (same scenario, same checker with `k = 1`; it passes `bash -n`).

`mise run pregate` on this tree (21:31, with the plan, the test file and the gate draft present): exit 0,
`gate guard: pass`, no guard id raised.
## The GitHub runs since 2026-11-14 (workflow `ci`, job `netsim`, event `push` on `main`; logs in `nsjobs/`)

Fifteen runs; twelve measured C5 of the editor variant: **7 failures** (seed 20771: 4 of 8; seed 20772: 3 of 4). The
reader variant passes C5 in all twelve. The first report of this block said « 8 of 15 »: the logs give 7 of 12.

| Run id | Created (UTC) | Seed | 8-editor C5 | Per ordered pair |
|---|---|---|---|---|
| 43575366495 | 11-14 08:20 | - | not measured (the log has no netsim scenario line) | - |
| 43577199269 | 11-14 08:43 | - | not measured (same) | - |
| 43620631116 | 11-14 17:21 | - | not measured (same) | - |
| 43624790761 | 11-14 18:07 | 20771 | pass | 1 1 0 0 0 0 |
| 43629366160 | 11-14 18:56 | 20771 | pass | 1 2 0 0 0 0 |
| 43633549985 | 11-14 19:42 | 20771 | pass | 2 2 1 0 0 0 |
| 43638847179 | 11-14 20:41 | 20771 | fail | 2 3 2 3 2 2 |
| 43642240432 | 11-14 21:19 | 20771 | fail | 3 2 3 2 2 2 |
| 43645254814 | 11-14 21:54 | 20771 | pass | 0 0 0 0 0 0 |
| 43647599752 | 11-14 22:20 | 20771 | fail | 2 3 2 2 2 2 |
| 43650598956 | 11-14 22:55 | 20771 | fail | 2 3 2 3 2 2 |
| 43673422313 | 11-15 03:40 | 20772 | fail | 2 2 3 2 2 3 |
| 43677087944 | 11-15 04:30 | 20772 | fail | 2 2 3 2 3 2 |
| 43694919823 | 11-15 08:31 | 20772 | fail | 1 2 2 3 2 2 |
| 43703418610 | 11-15 10:18 | 20772 | pass | 2 1 2 2 2 2 |

Upstream line: [DocHandle.ts#L268-L269 at v2.5.6](https://github.com/automerge/automerge-repo/blob/v2.5.6/packages/automerge-repo/src/DocHandle.ts#L268-L269)
(`// TODO: pass along the source (load/change/network)`, `patchInfo: { before, after, source: "change" }`; line
numbers read from the raw file on 2026-11-16).

F2, measured (Automerge 3.5.0, `node`, 2026-11-16): `A.getActorId(doc)` = `97b0cbf7…`, `A.getActorId(A.clone(doc))` =
`0acbf6ae…`; the change made after the clone is written by `0acbf6ae…`. A clone changes the actor.

Seen in passing, not part of this block: that push failed on `8-reader-s20772` (C1, C2, C3 fail, 326 s).

## Re-witness after the blind review (2026-11-16, 01:40 to 01:45, each step under `heavy.sh --exclusive`)

These rows replace the first table for the criteria the review changed.

| Criterion | Exit | Seconds | The failing line | Right reason |
|---|---|---|---|---|
| `mise run pregate` | 0 | 10 | `gate guard: pass`, no guard id raised | - |
| docs | 1 | 0 | `CL1 docs: ['exigences W32 does not name CL1']` (rows C-1..C-6, `GITHUB.md` open, the typists' period and factor, settle, window and bound of scenario 8 are found) | yes |
| frontend | 1 | 3 (bindings included) | `CL1 frontend: exit 1, failed 16, missing [...]` (table above) | yes; nine mutants red on a missing anchor or on C-1 |
| regression | 1 | 41 | `Test Files  1 failed \| 11 passed (12)`: only `claim.test.ts` fails. Twelve files, not nine: this worktree still carries PC1b, whose test files differ from `main` | yes |
| `mise run e2e:build` | 0 | 4 | warm caches; the cold duration is not measured, the 240 s cap is PC1b's | - |
| netsim (reduced: one strict run per seed) | 1 | 264 (2 x 129 s per run) | `CL1 netsim: exit 0, ['8-editor-s20771-r1.json: C-4 claims not exposed by the build (window.__inkwellClaims)', '8-editor-s20772-r1.json: …']`; C1 to C5 pass in both runs | red because the observation hook does not exist yet (step 2 adds it before the fix, then this criterion is witnessed again on real claim times) |

All criteria use the shared `heavy.sh` in the gate; `--exclusive` was only this witness's way to queue behind other
sessions' jobs.
