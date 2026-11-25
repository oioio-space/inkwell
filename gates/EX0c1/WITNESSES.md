# EX0c1 - red witnesses

Recorded 2026-11-25 at step 1, before any production change. Tree: the worktree `inkwell-ex0c` at `main` 4147534, plus
the plan `docs/plans/2026-11-25-ex0c-locations.md`, this draft gate and the PROGRESS lines EX0c1, EX0c2 and EX0c3,
which replace the single EX0c line. Each command was run once, exactly as `gates/EX0c1.toml` writes it, from the
worktree root and through `sh -c`, which is how `scripts/gate.py` runs it. The logs are in the session scratchpad
(`witness-EX0c1-<id>.log`). The machine was otherwise idle: no other heavy job ran, and the load average stayed between
0.33 and 1.5.

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| docs | 1 | < 1 s | It found no EX0c1 section in ADR 0027. Spec §4.31 does not name `header/start`, `header/end`, `format`, `format/heading`, `status/zoom` or `preview/toolbar`, and W83 does not name EX0c1. It did find the plan's rows L-1..L-9, its §5 and the three PROGRESS lines | yes (the missing parts are written with the code) |
| no-hard-locations | 1 | 0.2 s | 71 problems, all at plan §2 sites. Commands placed in markup: `HeaderBar.svelte:41,47,57,59,62,63`, `ZoomMenu.svelte:21-23`, `HeadingMenu.svelte:42,53`, `StatusBar.svelte:99`. Bars and menus outside a host: `HeaderBar.svelte:64`, `FormatBar.svelte:69,90`, `HeadingMenu.svelte:28`, `ZoomMenu.svelte:17`, `StatusBar.svelte:99,112,170`. The bar tables are still present in `ui/toolbar/spec.ts` (9 lines), `BarButton.svelte:6,22`, `FormatBar.svelte:14,45`, `HeadingMenu.svelte:7,24,51,59` and `App.svelte:60,150`. Preview.svelte still draws 3 tools and StatusBar.svelte 6 items. All six locations are empty, the 13 `format.*` commands have no icon, the status point is empty, and Present is not limited to Typst. Type ids remain at `App.svelte:156` and `StatusBar.svelte:98`. No EX0c2 or EX0c3 file was flagged, nor the K-7 line `App.svelte:379` | yes. One plan site was missing (`BarButton.svelte`) and has been added to plan §2 |
| frontend | 1 | 307 s | Biome clean. svelte-check: 846 files, 0 errors, 0 warnings. Vitest: 1480 passed, 0 failed, 1 todo. The 9 rule tests L-1..L-9 are `absent` | yes (307 s of the 480 s cap) |
| mutants | 1 | < 1 s | ML1..ML9 are missing. There is no `*location*` vector in `valid/`, and none of `*unknown-location*`, `*foreign-group*`, `*protected-group*`, `*plugin-status-entry*` or `*reserved-location*` in `invalid/`. The runner was not reached | yes (the entries come at step 2). `mutants.py --check` with this gate present: 313 entries, 0 problems |
| e2e | 1 | 30 s | `e2e:build` done (7 s). In Chromium, the 12 replays passed the first time (22 s): EX0b1-E2, UI-02, UI-03, UI-04, UI-05, UI-07, ER-1–4, PRV-100, PRV-101, PRV-105, PRV-107, « B2 text zoom ». EX0c1-E1 and E2 are `absent` | yes (30 s of 560 s) |
| e2e-webkit | 1 | 10 s | `mise run e2e:webkit`, WebKit in the Playwright container: UI-05 passed the first time (3 s). EX0c1-E1 and E2 are `absent` | yes (10 s of 420 s) |
| perf-start | 0 | 115 s | PERF-START `pass` on its first attempt, paired against the merge-base 4147534: ratio 1.0016, bounds 0.990–1.013, half-width 1.12 %. Absolute budget `pass` (median bound 254 ms ≤ 350, p95 bound 267 ms ≤ 400) | yes: it is a regression guard and passes on the unchanged tree |
| simplify-report | 1 | < 1 s | `gates/EX0c1/SIMPLIFY.md` absent | yes |

7 of 8 criteria are red for the right reason. `perf-start` is a regression guard, so it is green on the unchanged tree.

## Round 2 (after the blind review, `gates/EX0c1/REVIEW.md`): changed criteria only

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| no-hard-locations | 1 | 0.2 s | 117 problems: round 1's 71 (the preview location now asks for `typst.preview-zoom-*`) plus 12 command-id literals in placement files (`HeaderBar.svelte:41,47,57,59,62,63`, `ZoomMenu.svelte:21-23`, `StatusBar.svelte:99`, `HeadingMenu.svelte:42`, `spec.ts:38`), all 19 locations missing from `LOCATIONS` plus no reserved mark and no `main` groups `insert`/`export`/`extensions`, 7 more bar commands with no icon, the nine status items absent (exact side, group, `when`, label), and Present's `when` not exactly `canPresent && fileType == typst` | yes. The location check first matched `"format"` and `"view"` as category words; it now requires a `<loc>: {` entry, so all 19 are reported |
| e2e | 1 | 78 s | `e2e:build` 4 s; Chromium: 23 tests passed the first time in 73 s (EX0b1-E2, UI-02…UI-07 with UI-06 in both themes, UI-11 ×2, ER-1–4, PRV-100/101/105/107, « B2 text zoom », PRS-06, PRS-31, PRS-33, PRS-37 ×2, UX-L2-1, SHEET-FREEZE, JOIN-3); EX0c1-E1 and E2 `absent` | yes. The checker first missed SHEET-FREEZE, whose title sits in a `describe`: it now matches a test's own title or its describe path, replayed on the same report |

`docs`, `frontend`, `mutants`, `e2e-webkit`, `perf-start` and `simplify-report` did not change, so they were not rerun.

## Caps (rule G-1)

The sum of the `timeout` caps is 30 + 30 + 480 + (20 + 300) + (200 + 360) + 420 + 600 + 30 = 2470 s, which is at most
2700 s; `gate.deadline_problem` returns None. The measured total is about 515 s (round 2's e2e takes 78 s of its 560 s), with `frontend` taking 307 s of it. The
caps leave room for the code that adds the nine rule tests and the two e2e scenarios.

## Before the freeze

- A blind review (Opus, plus Gemini if its quota allows) of the plan and of this gate (method v2), then the freeze.
- The rule tests, mutants, vectors and the scenarios `frontend/e2e/ex0c1.spec.ts` are written at step 2. Their titles
  are fixed by this gate.
