# EX0c2: the `panels` point, the grid's bar and context menu, the view, history and difference bars

Rigour: standard (no peer bytes, no trust state, no sandbox boundary). Follows EX0c1 (plan
`docs/plans/2026-11-25-ex0c-locations.md` §5, which lists these sites; `LOCATIONS` reserves their locations). Gate
`gates/EX0c2.toml`, witnesses `gates/EX0c2/WITNESSES.md`; ADR 0027 gains « Panels, the grid and the secondary bars
(EX0c2) » with the code. **Not split:** the touched suites replay in 1.5 min (witnesses), so one gate stays well under
45 min; the grid and the panels share the hosts (menu at a point, width overflow, the resolver).

## Prior art (read 2026-11-25) and what is better here

VS Code `viewsContainers` (`activitybar`, `panel`: `id`, `title`, `icon`) hold `views` (`id`, `name`, `when`, `icon`),
a view's code activates when it opens (`onView:`); `submenus` hang a menu off an item; every view gets a « Show » command
([points](https://code.visualstudio.com/api/references/contribution-points)). GNOME HIG: a utility pane goes left if
it affects the main view, right if subordinate, toggled with a shortcut, overlapping when narrow
([utility panes](https://developer.gnome.org/hig/patterns/containers/utility-panes.html)); a view switcher holds about 3
to 5 views named by nouns ([view switchers](https://developer.gnome.org/hig/patterns/nav/view-switchers.html)). WAI-ARIA
APG [tabs](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/): roving focus, arrows wrap, Home/End, automatic activation
only when a panel shows without delay, `aria-controls`, `tabpanel` labelled by its tab, `tabindex="0"` on a panel without
a focusable first child; APG [menu](https://www.w3.org/WAI/ARIA/apg/patterns/menubar/): focus on the first item, Escape
returns focus to the opener, a submenu item has `aria-haspopup` and `aria-expanded`, Right opens it, Left closes it.
**Better here:** a side's order is the closed resolver's (core first, never a contributed priority); bodies of a side
load together the first time it opens, so automatic activation never waits on a chunk, and nothing loads at start but
the left side shown on the first screen (EX0c1 review: lazy chunks at start cost a request each); a context menu is a
location, so the palette, the bar and the menu show one name and one chord per action.

## 1. The `panels` point (same validator, bounds and null-prototype rules as ADR 0027)

- **Fields:** `id` (`<extension>.<name>`, a claim like a command id), `side` (`start`, `end`), `order` (0–999), `when`?
  (hides the tab), `icon` (icon set name, for where its name does not fit), `label` (i18n key: the tab's name and the
  panel's accessible name), `entry` (a `native/` module whose default export is the body; `builtin` only, `ui.native`,
  as status entries, until EXT1).
- **Sides** are a closed table `SIDES` (`extensions/manifest.ts`): `start` (label `sidebar.label`, toggle `app.sidebar`),
  `end` (label `review.title`, toggle `app.review`); the toggles stay commands (their chords unchanged; GNOME's F9 is a UX1-D question).
  Order in a side: the core's, then `order`, extension id, panel id (L-2's `placed`).
- **Host** `ui/command/PanelSide.svelte`: an `aside` named by its side (`data-testid="history-drawer"` kept on `end`).
  One panel shown: its body, no tab list (nothing to switch). Two or more: an APG tab list named by its side's label
  (`aria-label`) — `role=tab`, `aria-selected`, `aria-controls`, roving `tabindex` (selected 0, others -1), Left/Right
  wrap, Home/End, automatic activation; every panel of the side is mounted once loaded, the others `hidden`, so each
  `aria-controls` names an element; a body is `role=tabpanel`, `aria-labelledby` its tab, `tabindex="0"` when its first
  child is not focusable. A side remembers its selected panel for the session; a hidden selected panel falls to the
  first shown.
- **Lazy body:** `start` bodies are imported with the first screen (`extensions/panelEntries.ts`, an eager glob as
  `statusEntries.ts`), since the files are shown at start; `end` bodies never at start (the drawer starts closed,
  `history.svelte.ts:89`): they are fetched together at idle after the first paint (`requestIdleCallback` once
  `.cm-content` shows, after PERF-START's mark), or at the side's first open if that comes sooner. A side opens only
  once its bodies are in, so no frame shows an empty panel. Budget of that first open: 100 ms. Measured at step 1 on
  the built dist, a first `import()` of the grid's chunk (120 KB, larger than the drawer's sources, 72 KB): at most
  5.4 ms in Chromium, 8 ms in the WebKit container (5 starts each, `gates/EX0c2/WITNESSES.md`).
- **Focus:** opening a side by its toggle or pointer leaves the focus where it is (today); a tab chosen with the keys
  takes it (APG); closing a side that holds the focus gives it to `returnTarget()` (EX0b1) or the shown view, never to
  `body`. `openPanel(id)` (shell) opens the side and selects the panel: problems' `show()` and `history.show()` use it.
- **Built-ins:** `app.files` (start, the file tree, `Sidebar.svelte` body), `app.problems`, `app.history`,
  `app.comments`, `app.activity` (end, 10–40, today's tabs and names). `DiffView` and `ApplyDialog` leave the drawer for
  a host mounted with the shell (a dialog never waits on a body).

## 2. Locations (the table loses what is a value picker, as EX0c1's encoding: VS Code's « Reopen With » is a pick)

| Location | Groups, in order (protected in bold) | Overflow | Today |
|---|---|---|---|
| `grid/toolbar` | **history**, structure, sort (sort and filter, today one group), freeze, extensions, ‹show text› | ⋯ by width | `Sheet.svelte:254-330,530-561` |
| `grid/context` | clipboard, structure, sort, freeze, extensions | « Extensions » submenu past 8 | `Sheet.svelte:384-469,642-644` |
| `view` | ‹default› (the type's default viewer, from the `viewers` point, its label, `data-viewer`, offered only while another viewer is shown: today's rule, `App.svelte:646`), ‹offer› (« Ouvrir comme tableau ? »), extensions | — | `App.svelte:639-651` |
| `history/toolbar` | ‹since›, ‹legend›, ‹colour› (radio pick), seen, extensions | — | `HistoryBars.svelte:139-157` |
| `diff/toolbar` | ‹mode› (radio pick, text only), navigate, extensions; ‹close› stays the dialog's header | — | `DiffView.svelte:175-191` |

The width overflow moves from `FormatBar` into `LocationBar` (the EX0c1 residual, taken here [EX0c2]), used by both bars. The context-menu host
(`ui/command/LocationMenuAt.svelte`, from `ui/sheet/ContextMenu.svelte`) draws any `overflow: "submenu"` location at a
point: separators between groups, `CommandMenuItem`s (a toggle as `menuitemcheckbox`), APG keys, focus on the first
item, Escape back to the opener; past 8 extension items, one « Extensions » item with `aria-haspopup="menu"` and
`aria-expanded` opens them as a sibling menu (Right or Enter in, Left or Escape out), order kept. The six reserved
context menus reuse it at UX1-D.

## 3. Commands (one name and one chord per action: no two palette labels equal, EX0c1 review)

Grid, `ext/tables` (the viewer `tables.grid` declares each, its surface handles them; chords of keys the grid owns are
`delegated: "widget"`, shown never captured). T = toolbar, C = context menu. New context keys (each read by a built-in):
`gridLocked`, `gridFiltered`, `gridFrozen` (the grid surface reports them), `unseen` (marks not yet seen), `changes`
(the open difference is a text with a change). `readOnly` is the targeted surface's own: the grid reports its
controller's (`sheet.svelte.ts:194`: the reader role, a lock, a file too big). Levels as today: read-only is on the
command's `when` (the bar greys) and on the context menu item's `when` (the menu hides); `gridLocked` only on the
command's `when` (greys in both).

| Command (label key) | Where | Command `when` (greys) / item `when` (hides) | Toggle / chord |
|---|---|---|---|
| `app.undo`, `app.redo` (existing) | T history | `!readOnly` / — | — |
| `tables.cut`, `tables.copy`, `tables.paste`, `tables.paste-values` (`sheet.cut` …) | C clipboard | `!readOnly` / C `!readOnly` (not copy) | Ctrl+X, C, V, Ctrl+Shift+V |
| `tables.insert-row-above`, `-below` (`sheet.insertRowAbove`, `…Below`) | T (above), C | `!readOnly && !gridLocked` / C `!readOnly` | — |
| `tables.insert-col-left`, `-right` (`sheet.insertColLeft`, `…Right`) | T (left), C | `!readOnly` / C `!readOnly` | — |
| `tables.delete-rows`, `tables.delete-cols` (`sheet.deleteRows`, `sheet.deleteCols`) | T, C | `!readOnly` / C `!readOnly` | — |
| `tables.duplicate-row`, `tables.clear` (`sheet.duplicateRow`, `sheet.clear`) | C | `!readOnly` (+ `!gridLocked`) / C `!readOnly` | clear: Delete |
| `tables.sort-asc`, `tables.sort-desc` | T sort, C sort | — | — |
| `tables.filter` (`sheet.filter`: the active column's filter menu) | T sort | — | `gridFiltered` |
| `tables.filter-by-value` | C sort | — | — |
| `tables.freeze` (`sheet.freeze`: freezes at the selection, always) | T freeze (item `!gridFrozen`), C freeze | — | — |
| `tables.unfreeze` (`sheet.unfreeze`) | T freeze (item `gridFrozen`), C freeze | `gridFrozen` / C `gridFrozen` | `gridFrozen` (pressed, as today) |
| `tables.autofit` | C freeze | — | — |

History and differences, `ext/app`: `app.mark-seen` (« Marquer tout comme vu », `cmd.mark-seen`, `when: unseen`,
history/toolbar seen); `app.previous-change`, `app.next-change` (existing, now `when: changes` with a reason;
diff/toolbar navigate, label look). Names: the commands take the context menu's precise names, so the toolbar's
« Insérer une ligne », « Supprimer la ligne », « Insérer une colonne », « Supprimer la colonne » read « … au-dessus »,
« Supprimer les lignes », « … à gauche », « Supprimer les colonnes » (SHEET-ROWS `:304`, SHEET-MU-02 `:958`,
SHEET-FREEZE and SHEET-BIG `:684`, a substring match, pass unchanged); the context menu's « Figer jusqu'ici » reads
« Figer jusqu'à la sélection »; « Changement précédent / suivant » read « Différence précédente / suivante ». Chords:
the context menu shows each command's own chord (`CommandMenuItem`); Ctrl++ and Ctrl+- belong to `tables.insert-cells`
and `tables.delete-cells` (insert or delete rows, or columns when whole columns are selected), so they leave
« Insérer une ligne au-dessus » and « Supprimer les lignes » and stay in the palette and the shortcuts window.
`ownView` (`App.svelte:99,718,730`) becomes a viewer field `ownStatus` (`tables.grid`: draws its own status bar).

## 4. Rules (test titles in the gate; each has a mutant caught by that title only, `scripts/netsim/mutants.toml`)

| Rule | What | Mutant |
|---|---|---|
| P-1 | `panels` validate; the test reads every `invalid/` panel vector and its reason: unknown side, duplicate id, a plugin entry, no label | MV1 an unknown side accepted |
| P-2 | a side's panels: core first, then order, extension, id, whatever the build order; one panel, no tab list | MV2 ties in build order |
| P-3 | the tab list is APG's: named, roles, every `aria-controls` target mounted, `labelledby`, roving tabindex, arrows wrap, Home/End, automatic activation | MV3 ArrowRight does not wrap |
| P-4 | bodies: none at start but `start`'s, `end`'s at idle after the first paint; a side opens once loaded, never empty; a tab's `when` hides it, the selection falls back | MV4 the end side imported at start |
| P-5 | focus kept on open, returned on close, `openPanel` selects (problems, history entry) | MV5 close leaves focus on body |
| P-6 | grid bar = `grid/toolbar`, greyed by the commands' `when` on the grid's own state (a file too big included), pressed by their toggles; the focus back on the grid after a click, except `tables.filter` (to its filter menu); ‹show text›; ⋯ by width shared with the format bar, nothing lost | MV6 `gridLocked` ignored; MV12 `readOnly` read from the workspace |
| P-7 | grid menu = `grid/context`: item `when` hides, a toggle is `menuitemcheckbox`; keys: Up/Down wrap, Home/End, Enter and Space run, Escape and Tab close with the focus back on the grid | MV7 item `when` not read |
| P-8 | « Extensions » past 8: 8 inline, 9 one submenu item, Right in, Left out, order kept | MV8 9 drawn inline |
| P-9 | view bar = `view` around ‹default› and ‹offer›, no type id: the default viewer only while another shows, so a grid has no view bar; `ownStatus` from the viewer | MV9 the default viewer offered while it is shown |
| P-10 | history strip and difference bar from their locations; `unseen`, `changes`; no two palette labels equal | MV10 `changes` true for a table |
| P-11 | cost: panels and locations indexed once at build (`panels(side)` the same array), a closed side imports nothing | MV11 `panels(side)` rebuilt |

End to end, Chromium and WebKit: `EX0c2-E1` in the grid's bar, its context menu, the view bar, the history strip, the
difference bar and both sides' tab lists, every button, menu item and tab is its location's or point's resolved item
in order, or a fixed widget whose `data-fixed` is in a closed list (`show-text`, `default-viewer`, `offer-table`,
`since`, `legend`, `colour`, `diff-mode`); any other control fails with « stray », and the gate's MV13 (a button added
to the grid's bar; `scripts/netsim/mutants.toml`, new kind `playwright` with `spec`, `grep`, `project`, implemented in
`mutants.py` with its `--check` and a `test:py` test at step 2 [EX0c2]) must make it fail; read-only greys and hides; `EX0c2-E2` the
drawer's tabs by keyboard (arrows, Home/End, focus back on close), its code fetched at idle, no frame with an empty
panel, its first open within 100 ms, and the grid menu by keyboard (Shift+F10, arrows, Escape). Replayed (Chromium):
EX0c1-E1, EX0b1-E1, EX0b1-E2, EX0b2-E1, EX0b2-E3, W58a-UX-1e, UI-03, UI-05, UI-06, UI-11, FM-OPEN-1, ER-1–4, B2 gutter, B2 variants,
UX-EX0-1/2/4/5, EX0-E1, SHEET-ROWS, SHEET-COLS, CSV-11, SHEET-FREEZE, CSV-20 (with its screenshots), SHEET-BIG,
SHEET-MU-02; WebKit: UI-05. W58a-UX-1e (run with `W58A_UX=1`) is red on `main` d2ea1c9 for a stale name:
it clicks « Afficher Relire » (`ux/w58a.spec.ts:381`, also `:276,310`), which EX0b1/EX0c1 made the toggle « Relire »
(pressed); accepted as a named test edit of step 2 (decision of 2026-11-25): its three « Afficher Relire » selectors
(`:276,310,381`) become the « Relire » toggle [EX0c2]. Residual → E2E1: a suite gated behind an environment variable
(`W58A_UX`) rotted on `main` unseen; list the e2e suites gated by an environment variable that CI never runs, and run
or retire each.

## 5. Performance, gate, risks, out of scope

Start: PERF-START paired against the merge-base (ADR 0038), a guard; `end` bodies leave the start closure, `start`'s
stay in it. Gate (≤ 45 min): `docs`, `no-hard-locations`, `frontend`, `mutants`, `e2e`, `e2e-webkit`, `perf-start`,
`simplify-report`, plus `e2e-mutant` (MV13). Risks: the first open of the drawer before the idle fetch (E2, budget
100 ms, measured below 10 ms at step 1), the grid losing its focus after a bar click (P-6), UI-05 when the overflow
moves. A move changes no behaviour; when in doubt today's stays (review, `gates/EX0c2/REVIEW.md`). **Out of scope:** a « Show »
command per panel and a panel's own toolbar (`view/title`, UX1-D); the « Sommaire » panel (UX1); the variant banner's
action (banners are not locations); the history rows' actions (W65); `status` as two `LOCATIONS` entries (set aside
again: its groups carry narrow-width rules a location has not); arrow roving inside toolbars (UX1-D, EX0c1 decision).

## Sites (file:line at `main` d2ea1c9)

- Grid: `ui/sheet/Sheet.svelte:8` (`shortcutOf` import), `:245-330` (`Btn`, `groups`), `:332-382` (menu clipboard into
  the handlers), `:384-469` (`menuItems`, `shortcutOf` `:413,436`), `:530-561` (toolbar), `:642-644` (`ContextMenu`);
  `ui/sheet/ContextMenu.svelte:1-112`; `ui/sheet/menu.ts:1-10`; `ui/sheet/Grid.svelte:430-447,498-502,672` (opens the
  location at the pointer or the cell); `ui/sheet/commands.ts:16-56` (handlers); `ext/tables/inkwell-plugin.json:44-66` (viewer), `:69` (commands).
- Panels: `ui/shell/Sidebar.svelte:30-34`; `App.svelte:612-614,744`; `ui/history/HistoryDrawer.svelte:47,82-90`
  (tabs), `:91-219` (bodies), `:222-223` (dialogs); `ui/history/history.svelte.ts:46,90,463`;
  `language/problems.svelte.ts:176-179`.
- View bar: `App.svelte:122-142,639-651`; `ownView` `App.svelte:99,718,730`.
- History and differences: `ui/history/HistoryBars.svelte:139-157`; `ui/history/DiffView.svelte:150-153,175-191`.
- Hosts: `ui/command/LocationBar.svelte`, `FormatBar.svelte` (overflow); `extensions/manifest.ts:153-184` (`LOCATIONS`
  rows), `extensions/when.ts:43-60`.
