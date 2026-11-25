# EX0c2 - blind pre-freeze review: « freeze after fixes »

Reviewed: `docs/plans/2026-11-25-ex0c2-panels-grid.md` and `gates/EX0c2.toml` (step 1 draft, 2026-11-25). Every finding
was applied as listed. The rule the review set for this block: a move changes no behaviour, so when in doubt today's
behaviour stays. The changed criteria were rerun once each (`gates/EX0c2/WITNESSES.md`, round 2).

## Major

| # | Finding | Change |
|---|---|---|
| 1 | Freeze as one toggle with one label changes the bar (SHEET-FREEZE reads « Libérer les volets », pressed) and the menu (refreezing elsewhere took one click) | Two commands. `tables.freeze` (« Figer jusqu'à la sélection ») always freezes at the selection, with no toggle and no `when`. `tables.unfreeze` (« Libérer les volets ») has `when: gridFrozen` and `toggled: gridFrozen`, so it shows pressed as today. In the bar, the items `!gridFrozen` / `gridFrozen` show one of them, as today; the menu has both. SHEET-FREEZE passes unchanged. Lint: no toggle and no `when` on freeze, the exact `when` on unfreeze, both items in both locations |
| 2 | Names: the plan let the toolbar's short names win | The commands take the menu's precise names (`sheet.insertRowAbove`, `sheet.insertColLeft`, `sheet.deleteRows`, `sheet.deleteCols`). SHEET-ROWS `:304` and SHEET-MU-02 `:958` pass unchanged, and so does SHEET-BIG `:684` (a substring match). The context menu shows each command's own chord. Ctrl++ and Ctrl+- belong to `tables.insert-cells` and `tables.delete-cells` (`sheet.svelte.ts:668-677`: rows, or columns when whole columns are selected), so both leave « Insérer une ligne au-dessus » and « Supprimer les lignes », and stay in the palette and the shortcuts window. Declared in plan §3. Lint: the labels |
| 3 | `readOnly` from the workspace would miss a grid that is read-only on its own | The grid surface reports its controller's `readOnly` (`sheet.svelte.ts:194`: the reader role, a lock, a file too big). P-6 covers the too-big case. New mutant MV12 (P-6): `readOnly` read from the workspace. Levels, as today: read-only is on the command's `when` (the bar greys) and on the menu item's `when` (the menu hides); `gridLocked` is on the command's `when` only (greys in both). Lint: the context items' `when` is `!readOnly` |
| 4 | The view bar offered « Voir le texte » under a grid, a new bar that changes the CSV-20 screenshots | Today's rule (`App.svelte:646`): ‹default› offers the type's default viewer only while another viewer is shown. No bar under a grid. MV9 is now « the default viewer offered while it is shown », and P-9 is reworded |

## Minor

| # | Finding | Change |
|---|---|---|
| 5 | Loading at first open could show an empty panel; the 100 ms was not measured; the plan and the gate disagreed | The drawer's code is fetched at idle after the first paint (after PERF-START's mark), or at first open if that comes sooner. A side opens only once its bodies are in, so E2 asserts no frame with an empty panel. Measured on the built dist: a first `import()` of the grid's chunk (120 KB, larger than the drawer) takes at most 5.4 ms in Chromium and 8 ms in the WebKit container (witnesses). Plan §1 and §4 and the gate's `e2e` comment now say the same |
| 6 | The tab list had no name, and `aria-controls` pointed at unmounted panels | The tab list is named by its side's label. Every panel of a side is mounted once loaded, the others `hidden`. P-7 names its keys: Up/Down wrap, Home/End, Enter and Space, Escape and Tab |
| 7 | Focus after a bar click | P-6: the focus goes back to the grid after a click, except `tables.filter`, which moves it to the filter menu (today's `Sheet.svelte:545`) |
| 8 | The replay set missed suites that the sides and the palette touch | Added W58a-UX-1e (`ux/w58a.spec.ts:341,382`), EX0b2-E1 and EX0b2-E3. The cap was re-measured (witnesses, round 2). W58a-UX-1e runs only with `W58A_UX` set, so the gate now sets `W58A_UX=1`. On `main` it is red for a stale name (« Afficher Relire », `:381`, which EX0b1/EX0c1 turned into the toggle « Relire »). Decision of 2026-11-25: its three selectors (`:276,310,381`) become the « Relire » toggle as a named test edit of step 2. A residual goes to E2E1: list the e2e suites gated by an environment variable that CI never runs |
| 9 | E1 had no closed list of fixed widgets, and no proof that it catches a stray control | E1 accepts only `data-fixed` from a closed list (`show-text`, `default-viewer`, `offer-table`, `since`, `legend`, `colour`, `diff-mode`), and fails with « stray » otherwise. New criterion `e2e-mutant`: MV13 adds a button to the grid's bar, and E1 must fail. Decision of 2026-11-25 (no second registry): MV13 is an entry of `scripts/netsim/mutants.toml` with a new kind `playwright` (`spec`, `grep`, `project`). The criterion runs `mutants.py EX0c2 MV13`, which mutates a throwaway copy of the tree. The kind, its `--check` support and a `test:py` test are EX0c2 step 2. A first draft kept MV13 in its own JSON file (never created), which was dropped |
| 10 | « Tout est vu » is not a command's name | `app.mark-seen` is « Marquer tout comme vu » (`cmd.mark-seen`; lint checks the French text) |

## Deadline

The caps add up to 30 + 30 + 480 + (20 + 400) + (20 + 120 + 120) + (200 + 480) + 300 + 400 + 30 = 2630 s ≤ 2700 s.
`gate.deadline_problem` counts 2390 s, because the two inner caps of `e2e-mutant` are argument lists of a Python call,
which its parser does not read. The true sum is the one above. `e2e-webkit` goes from 420 to 300 s (measured 11 s) and
`perf-start` from 600 to 400 s (measured 115 s) to make room.
