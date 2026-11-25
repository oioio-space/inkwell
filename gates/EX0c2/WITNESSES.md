# EX0c2 - red witnesses

Recorded 2026-11-25 at step 1, before any production change. Tree: the worktree `inkwell-ex0c2` at `main` d2ea1c9, plus
the plan `docs/plans/2026-11-25-ex0c2-panels-grid.md`, this draft gate and the PROGRESS line EX0c2, which now points at
the plan. Each command was run once, exactly as `gates/EX0c2.toml` writes it, from the worktree root and through
`sh -c`, which is how `scripts/gate.py` runs it. The logs are in the session scratchpad (`witness-EX0c2-<id>.log`).
No other heavy job ran; the load average stayed between 1.5 and 1.9.

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| docs | 1 | < 1 s | ADR 0027 has no EX0c2 section; spec §4.31 has no EX0c2 line naming `grid/toolbar`, `grid/context`, `view`, `history/toolbar`, `diff/toolbar` or `panels`; W83 does not name the plan. It found the plan's rows P-1..P-11, its site list and the PROGRESS line | yes (written with the code) |
| no-hard-locations | 1 | 0.2 s | 74 problems, all at plan sites. Surfaces outside a host: `App.svelte:640,647` (view bars), `HistoryDrawer.svelte:82,83,85` (aside, tab list, tabs), `ContextMenu.svelte:70`, `Sheet.svelte:530,643`, `Sidebar.svelte:30`. Placed ids: `Sheet.svelte:413,436` (`shortcutOf`, also caught as command-id literals). Tables: `menuItems` `Sheet.svelte:384,643`, `ownView` `App.svelte:99,718,730`. The old drawings remain in Sheet, App, HistoryDrawer, HistoryBars, DiffView, `history.svelte.ts`, `problems.svelte.ts`, Sidebar; `ContextMenu.svelte` and `menu.ts` still exist; the five locations are still `reserved: "EX0c2"`; no `SIDES`, no `panelEntries.ts`; the five context keys are absent; the four locations lack their commands, the ten bar commands their icons, the two toggles and three `when` clauses are absent; `tables.grid` declares none of the 18 commands and has no `ownStatus`; the five panels are absent. No false positive: the handler maps (`ui/sheet/commands.ts`, `DiffView.svelte:150-153`) are skipped by the handler-key rule. The first draft flagged the registry's own `MenuItem` type (6 lines in `extensions/registry.ts`): `MenuItem` left the pattern, and the file check on `ui/sheet/menu.ts` covers the grid's type | yes |
| frontend | 1 | 312 s | Biome clean; svelte-check 0 errors, 0 warnings; Vitest 1497 passed, 0 failed, 1 todo (tool chain exit 0). The 11 rule tests P-1..P-11 are `absent` | yes (312 s of the 480 s cap) |
| mutants | 1 | < 1 s | MV1..MV11 missing; no `*panel*` vector in `valid/`, none of `*unknown-side*`, `*duplicate-panel*`, `*plugin-panel-entry*`, `*panel-no-label*` in `invalid/` or `refused/`; the runner was not reached. `mutants.py --check` with this gate present: 322 entries, 0 problems | yes (entries come at step 2) |
| e2e | 1 | 100 s | `e2e:build` done; Chromium: 29 tests passed at their first attempt in 93 s, 0 skipped, 0 flaky (EX0c1-E1, EX0b1-E1, EX0b1-E2, UI-03, UI-05, UI-06 ×2, UI-11 ×2, FM-OPEN-1, ER-1–4, B2 gutter, B2 variants, UX-EX0-1/2/4/5, EX0-E1, SHEET-ROWS, SHEET-COLS, CSV-11, SHEET-FREEZE, CSV-20 with its 2 screenshot tests, SHEET-BIG ×2, SHEET-MU-02 ×2). EX0c2-E1 and E2 `absent` | yes (100 s of 620 s) |
| e2e-webkit | 1 | 11 s | WebKit in the Playwright container: UI-05 passed at its first attempt (3 s); EX0c2-E1 and E2 `absent` | yes (11 s of 420 s) |
| perf-start | 0 | 115 s | PERF-START `pass` at its first attempt, paired against the merge-base d2ea1c9: ratio 1.0007, bounds 0.991–1.010, half-width 0.95 %; budget `pass` (median bound 261 ms ≤ 350, p95 bound 274 ms ≤ 400) | yes: a regression guard, green on the unchanged tree |
| simplify-report | 1 | < 1 s | `gates/EX0c2/SIMPLIFY.md` absent | yes |

7 of 8 criteria are red for the right reason; `perf-start` is a guard, green on the unchanged tree.

A probe before the draft, same replay set with `--reporter=list`: 27 passed in 1.5 min; W58a-UX-1d was skipped
(it skips itself in this environment), so it left the replay set, since a skipped test cannot pass « first time ».

## Round 2 (after the blind review, `gates/EX0c2/REVIEW.md`): changed criteria only

Same tree, plus the review's fixes to the plan and the gate. Each changed criterion was run once. The logs are
`witness2-EX0c2-<id>.log`, and `witness3-EX0c2-e2e.log` for the `e2e` rerun with `W58A_UX=1`.

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| no-hard-locations | 1 | 0.2 s | 96 problems: round 1's 74, plus the review's new checks. `tables.unfreeze` is absent from both locations. Freeze has no exact `when` and no toggle split, and neither bar item has a `when`. The seven labels are wrong (`sheet.insertRowAbove`, …, `cmd.mark-seen`), and the French « Marquer tout comme vu » is missing. The eleven context items lack `!readOnly`. Still no false positive: the line hits are the same 15 plan sites | yes |
| frontend | 1 | 309 s | Tool chain exit 0 (Vitest 1497 passed, 0 failed, 1 todo). The 11 rule tests are `absent` under their new titles (P-4, P-6, P-7, P-9 reworded) | yes |
| mutants | 1 | < 1 s | MV1..MV12 missing (MV12: P-6, `readOnly` from the workspace), and the panel vectors too | yes |
| e2e-mutant | 1 | < 1 s | (round 2 version, replaced in round 3) MV13's own JSON file was absent; no build was made and nothing was edited | yes |
| e2e | 1 | 101 s, then 118 s | First run: 31 passed at their first attempt in 97 s, among them EX0b2-E1 (1.2 s) and EX0b2-E3 (0.5 s). W58a-UX-1e was skipped: it runs only with `W58A_UX` set (`ux/w58a.spec.ts:55`), so the gate now sets `W58A_UX=1`. Rerun: 31 passed, and W58a-UX-1e failed after 16 s on `main` itself. It clicks « Afficher Relire » (`:381`), a name that EX0b1/EX0c1 turned into the toggle « Relire » (pressed). It is red for a stale selector, not for EX0c2. Its selector is repointed at step 2 (plan §4 [EX0c2]; the same name is at `:276,310`). EX0c2-E1 and E2 `absent` | yes for E1 and E2. W58a-UX-1e is a red that step 2 fixes as a named test change |
| e2e-webkit | 1 | 11 s | UI-05 passed at its first attempt; E1 and E2 `absent` (cap now 300 s) | yes |
| perf-start | 0 | 111 s | `pass` at its first attempt against d2ea1c9: ratio 0.994, bounds 0.984–1.003, half-width 0.95 %; budget median bound 260 ms, p95 bound 270 ms (cap now 400 s) | yes (guard) |

`docs` and `simplify-report` did not change, so they were not rerun.

**The 100 ms budget of the drawer's first open (review minor 5)**, measured on the built dist (`mise run e2e:build`)
with a temporary probe spec, since deleted. The probe makes a first `import()` of the grid's chunk (120 KB raw, 41 KB
gzip; the drawer's sources weigh 72 KB) after `.cm-content` shows, on 5 fresh pages each. In Chromium: 4.5, 5.1, 5.4,
4.2 and 4.4 ms. In the WebKit container (`mise run e2e:webkit`): 8, 5, 5, 6 and 6 ms. So the worst is 8 ms, more than
ten times under the budget. **The e2e build after a source change** (for `e2e-mutant`) takes 3.2 s (vite 0.9 s), well
under its 120 s cap.

## Round 3 (decisions of 2026-11-25: W58a-UX-1e, MV13 in the one registry): changed criteria only

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| mutants | 1 | < 1 s | MV1..MV12 missing, and the panel vectors too. The runner now names MV1..MV12 only (`mutants.py EX0c2 MV1 … MV12`), and was not reached | yes |
| e2e-mutant | 1 | < 1 s | MV13 is absent from `scripts/netsim/mutants.toml` (it needs kind `playwright`, `spec` `frontend/e2e/ex0c2.spec.ts`, `grep` `EX0c2-E1`, `project` `chromium`, and a stray button); `mutants.py EX0c2 MV13` was not reached. The kind `playwright` does not exist yet in `mutants.py`: today its `validate` refuses any kind but go, vitest and e2e (`mutants.py:208`). The kind, its `--check` support and its `test:py` test are EX0c2 step 2, written with MV13 | yes: red until step 2 adds the kind and the entry |

`mutants.py --check` (whole catalogue): 322 entries (1 retired), 0 problems. No EX0c2 entry exists yet, so the new
kind is not read; an MV13 entry written before the kind would be refused as « unknown kind 'playwright' ». The kind and the entry come together in step 2 → EX0c2.
`mutants.py EX0c2 MV13 --check`: « no mutant for block EX0c2 ». `mise run inventory`: 0 unfiled items (the E2E1
residual is tagged in plan §4). `frontend/e2e/ex0c2.mutant.json` was never created, so there was nothing to delete;
round 2's mention of it is replaced by the registry entry.

## Caps (rule G-1)

After the review: 30 + 30 + 480 + (20 + 400) + (20 + 240) + (200 + 480) + 300 + 400 + 30 = 2630 s ≤ 2700 s, and
`gate.deadline_problem("EX0c2", …)` returns None, counting 2630 s. Since round 3, `e2e-mutant` is one `timeout 240`
around `mutants.py EX0c2 MV13` (build and E1 of the copy). Round 2's version had inner caps that the parser did not
count. The measured total is about 670 s: `frontend` 309 s, `e2e` 118 s, `perf-start` 111 s. The caps leave room for
the 11 rule tests, the 12 mutants (EX0c1 had 9 in 300 s, so 400 s here), MV13, and W58a-UX-1e's full run (its own
timeout is 200 s, within the 480 s of `e2e`).

Round 1 caps, for the record: 30 + 30 + 480 + (20 + 360) + (200 + 420) + 420 + 600 + 30 = 2590 s.

## Before the freeze

- A blind review (Opus, plus Gemini if its quota allows) of the plan and of this gate (method v2), then the freeze.
- The rule tests, mutants, vectors and `frontend/e2e/ex0c2.spec.ts` are written at step 2; their titles are fixed here.
