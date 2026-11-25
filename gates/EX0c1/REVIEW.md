# EX0c1 - blind pre-freeze review (Opus): « freeze after fixes »

Reviewed: `docs/plans/2026-11-25-ex0c-locations.md` and `gates/EX0c1.toml` (step 1 draft, 2026-11-25). The design is
confirmed: toolbars are `menus` locations, and the header stays closed to non-essential manifests. Each finding was
applied as listed below. Only the changed criteria were rerun (`gates/EX0c1/WITNESSES.md`, round 2).

## Major

| # | Finding | Change |
|---|---|---|
| 1 | The table is not closed: EX0c2 and EX0c3 locations have no groups, and `main` lacks spec §4.31 row 2's groups | Plan §1: every group of `grid/toolbar`, `grid/context`, `view`, `history/toolbar`, `diff/toolbar`, `insert` and the six context menus is written and marked `reserved → <block>`. `main` is now view, insert, export, **project**, document, extensions, **app**. Lint: `LOCATIONS` in `manifest.ts` declares all 19 locations and a reserved mark, and `main` has `insert`, `export`, `extensions` |
| 2 | Header: the fixed widgets have no exact slot, the groups have no appearance, and the testids are unaccounted for | Plan §1: `header/start` = **panels**, ‹sync state›; `header/end` = **view**, **document**, ‹people›, **review**, ‹main menu›. Appearance belongs to the group: `icon`, `label`, or `label suggested` (class `sug`, Share). The header commands get their icons (lint). Plan §2: `present-start` and `history-toggle` are removed, and their two tests select `[data-cmd=…]` |
| 3 | Status contract: no item label, and no narrow-width rule | `status` gains `label` (used by the ⋯ menu and the tooltip « X : … »). Plan §1 lists the nine items with side, group and exact `when`, plus each group's rule: problems text hides at a window width ≤ 900 px (`@media`); preview and type hide at a bar width ≤ 640 px (`@container`); extensions go into ⋯ (today's `StatusBar.svelte:248,254`). L-4 covers the rules, and E2 checks the layout at 640 px |
| 4 | Preview zoom loses its own accessible names and the fit percent | New commands `typst.preview-zoom-out` and `-zoom-in`, with their own labels `preview.zoomOut` and `preview.zoomIn`, replace the delegated `app.zoom-*`. ‹fit level› is a fixed widget that runs `typst.preview-fit` and shows the percent (`Preview.svelte:117`). L-8 and ML8 are reworded (`typst.preview-zoom-in` runs the text zoom) |
| 5 | The replay set misses what the bars touch | Added to `e2e`: UI-06 (axe, both themes), UI-11 (screenshots, describe title matched), PRS-06/31/33/37 (`present.spec.ts`), UX-L2-1 (`ux/l2.spec.ts:58-70`), SHEET-FREEZE (the grid's zoom, `sheet.spec.ts:498`) and JOIN-3 (header helpers in `e2e/helpers.ts`). The cap was re-measured (witnesses, round 2) |
| 6 | E1 only checks `[data-cmd]`, so a hard-coded button without it would pass | Plan §3 and gate: E1 checks that every button and menu item of each region is either a resolved item or a named fixed widget |

## Minor

| Finding | Change |
|---|---|
| The lint misses the brace and const variants | `place` matches `id={"…"}` and template literals. New: in placement files (`App.svelte`, `ui/shell`, `ui/toolbar`, `ui/popover`, `preview`), any string literal that equals a manifest command id, or `<ext>.${`, is flagged (12 sites today) |
| gate:114 does not name the status ids; gate:112/116 test `when` by substring | The nine status ids are checked with exact side, group and `when` plus a label. `typst.present`'s `when` must equal `canPresent && fileType == typst` |
| L-1 must read every vector | Plan L-1: the test reads every `invalid/` vector and checks its reason code |
| ML9 | `menu(loc)` must return the same array on each call; the mutant builds a new one |
| Format bar: protected group, keyboard, pressed state | Plan §1: none of its groups is protected. Plan §2: `aria-pressed` comes from the command's `toggled` state, which the text surface reports. The keyboard stays as it is today. The finding asked to keep roving focus with the arrows, but `shell.spec.ts:51-53` (UI-03) shows Tab moving through the bar button by button, and `FormatBar.svelte` has no arrow handling. So nothing roves today; adding APG roving focus is left to UX1-D |
| Plan §5 too long | Shortened to the list of sites |
