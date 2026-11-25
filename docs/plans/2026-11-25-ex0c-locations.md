# EX0c: where contributions show — menu locations, bars, panels and status items

Rigour: standard (no peer bytes, no trust state, no sandbox boundary). Builds on ADR 0027 (EX0, EX0b1, OBS1 sections),
which gains « Menu locations and status items (EX0c1) » with the code. Applies spec §4.31 rows 2, 3, 4, 5 and
architecture §15.13 (« one form: `{command, location, group, order, when}` »). **Split at step 1** (inventory below:
17 sites, about 90 placed elements, too many for one gate of 45 min and one plan of 2 pages): **EX0c1** (this plan's
§1–§4, gate `gates/EX0c1.toml`, witnesses `gates/EX0c1/WITNESSES.md`) fixes the closed table of every location, so
UX1-B reserves against it, and migrates the shell's bars; **EX0c2** the grid's bar and context menu, the panels, the
view bar and the history bars; **EX0c3** the Insert menu and the last type-id reads (§5). Each takes its own gate.

## Prior art (read 2026-11-25) and what is better here

VS Code: toolbars are menu locations (`editor/title`, `view/title`), the `navigation` group first, `group@order`, the
other groups sorted by name; `submenus` hang a menu off an item; `viewsContainers` (`activitybar`, `panel`) hold `views`
with `when`; status items are API only (`createStatusBarItem(id, alignment, priority)`), workspace items left, contextual
right, « limit the number of items » ([points](https://code.visualstudio.com/api/references/contribution-points),
[status bar](https://code.visualstudio.com/api/ux-guidelines/status-bar)). GNOME HIG: few header bar controls, main
actions at the start, menus at the end, content follows the view ([header bars](https://developer.gnome.org/hig/patterns/containers/header-bars.html));
utility panes left (affect the view) or right (subordinate) ([panes](https://developer.gnome.org/hig/patterns/containers/utility-panes.html)).
Eclipse `menuContributions` (`?after=`), JetBrains `add-to-group` and GMenu sections are §15.13's other sources.
**Better:** groups are a closed table per location with fixed order, never sorted by name nor by a numeric priority a
contribution picks (VS Code's `priority` lets anyone go first); a protected group refuses at build; a toolbar item is a
command and a place, its icon, label and chord its command's (one identity, EX0b1); status items keep a fixed bar height,
so a lazy item never moves the layout.

## 1. The points (same validator, bounds and null-prototype rules as ADR 0027)

- **`menus`** keeps its shape (`command`, `location`, `group`, `order` 0–999, `when`), its `location` and `group` now
  read from one closed table, `LOCATIONS` (`extensions/manifest.ts`): each location lists its groups in fixed order,
  which are protected (built-in essential manifests only, §15.13 « intouchable »), whether it has an overflow, and the
  block that hosts it. A location without a host yet is `reserved`: a built-in item there is refused (nothing built-in
  vanishes); another origin's item is kept inactive and listed (ND-3). Toolbars are locations (VS Code), not a point.
- **`status`** (new; the core's items are code, so there is no host-drawn text before EXT1): `id`
  (`<extension>.<name>`), `label` (i18n key: the item's name in the ⋯ menu and the start of its tooltip « X : … »),
  `side` (`start`, `end`), `group`, `order`, `when`?, `command`? (the item's click), `entry` (a `native/` module whose
  default export is the item's component; `builtin` only, `ui.native`, as command entries).
- **`commands`** gains nothing; the commands placed in a bar get their `icon` (§8.3): the `format.*` ones (B, I, U,
  S, ∑ become glyph entries of the icon set), `app.sidebar` `side`, `app.preview` `eye`, `typst.present` `present`,
  `app.review` `branch`, so no bar keeps a table of its own. New: `typst.preview-zoom-out`, `-zoom-in` (their own
  labels, `preview.zoomOut`, `preview.zoomIn`, icons `minus`, `plus`) and `typst.preview-fit` (toggled).
- **Appearance** is the group's, never the item's: `icon` (the command's icon, its label as accessible name and
  tooltip « Label (chord) »), `label` (text), `label suggested` (text with the `sug` class, Adwaita's suggested action).

| Location | Groups, in order (protected in bold) | Overflow | Host |
|---|---|---|---|
| `main` | view, insert, export, **project**, document, extensions, **app** (spec §4.31 row 2; GNOME keeps Preferences, Shortcuts, About last) | — | EX0b1, groups EX0c1 |
| `header/start` | **panels** (icon), then ‹sync state› | — | EX0c1 |
| `header/end` | **view** (icon), ‹people›, **document** (label suggested), **review** (icon), ‹main menu› (today's order, kept) | — | EX0c1 |
| `format` | ‹heading menu›, marks, inline, blocks, objects, extensions (all icon; none protected: no Undo, Redo or save state here), ‹Insert› | ⋯ by width | EX0c1 |
| `format/heading` | paragraph, headings (label) | — | EX0c1 |
| `status/zoom` | zoom (label) | — | EX0c1 |
| `preview/toolbar` | zoom-out (icon), ‹fit level›, zoom-in (icon), navigate (icon), extensions | — | EX0c1 |
| `grid/toolbar` | **history** (undo, redo, §4.31 row 3), structure, sort, filter, freeze, extensions, ‹show text› | ⋯ | reserved → EX0c2 |
| `grid/context` | clipboard, structure, sort, freeze, extensions (« Extensions » submenu past 8) | — | reserved → EX0c2 |
| `view` | viewers (the type's alternatives), extensions | — | reserved → EX0c2 |
| `history/toolbar` | navigate, colour, extensions | — | reserved → EX0c2 |
| `diff/toolbar` | **close**, mode, navigate, extensions | — | reserved → EX0c2 |
| `insert` | blocks, files, data, extensions | — | reserved → EX0c3 |
| `editor/context`, `files/context`, `tabs/context`, `preview/context`, `people/context`, `link/context` | each: core, extensions (« Extensions » submenu past 8) | — | reserved → UX1-D |

‹…› is a fixed widget of the host, at that exact place; it carries `data-el` (UX1-0) or, when it runs a command
(‹fit level› runs `typst.preview-fit` and shows the percent, `Preview.svelte:117`), that command's `cmdAttrs`.

**Status point** (`ext/app` unless said; exact `when`): start **presence**: `app.presence`. End, in this order:
problems: `typst.problems` (`ext/typst`, `fileType == typst`; at ≤ 900 px window, `@media`, its text hides and the
counts stay); preview: `typst.page` (`ext/typst`, `main && previewOpen`; hidden at ≤ 640 px bar, `@container`);
**save**: `app.unsaved` (never hidden); caret: `app.position`, `app.words` (`text`); file: `app.zoom`, `app.encoding`;
type: `app.language` (`text`; hidden at ≤ 640 px bar); extensions: ≤ 3 shown, the rest and all of them at ≤ 640 px in
⋯ (by `label`). Those are today's rules (`StatusBar.svelte:248,254`), now per group.

**Order** (§15.13): the location's groups at their fixed places; in a group, `order`, then extension id, then command
id; an extension group after the core's. **When**: an item's `when` hides it; its command's `when` greys it with the
reason (EX0b1); both read the live context, never an extension. The context gains `text` (the open file shows text: caret,
words, language) and `main` (the open file is the main document), and `canPresent` loses its type test; a key no
built-in reads is refused (EX0b1 rule).

## 2. Migration (EX0c1; each site deleted once its items are on the points)

| Site | Becomes |
|---|---|
| `ui/shell/HeaderBar.svelte:47,57-63` (5 commands placed in markup; `:41` `availability("typst.present")`) | `header/start` `app.sidebar`; `header/end` `app.preview`, `typst.present` (`when: canPresent && fileType == typst`, in `ext/typst`), `app.share`, `app.review`; fixed widgets at their slots: sync state `:48-50`, title, `PeopleStack` `:61`, main menu `:64`. The testids `present-start` and `history-toggle` go: their two tests (`ex0b1.spec.ts`, `history.spec.ts`) select `[data-cmd="typst.present"]` and `[data-cmd="app.review"]` |
| `ui/toolbar/spec.ts:10-135` (`BarId`, `BAR_ITEMS`, `itemsFor`), `FormatBar.svelte:44-105`, `BarButton.svelte:6,22`, `layout.ts:15` | `format` items of `ext/format`, `when: "<feature> in formats"`; the syntax hints move into the language's syntax tables (`editor/format/typst.ts`, `markdown.ts`); one bar host with the overflow (`visibleCount` kept); keyboard as today (UI-03, `shell.spec.ts:51-53`); `aria-pressed` from the command's `toggled` state, which the text surface reports from the marks and block at the caret |
| `ui/toolbar/HeadingMenu.svelte:40-62` (`format.paragraph`, `format.{h}`), `spec.ts:137-147` | `format/heading`; the trigger still shows the level at the caret |
| `ui/popover/ZoomMenu.svelte:21-23` | `status/zoom` |
| `ui/shell/StatusBar.svelte:91-181` (9 items: people, problems, page, unsaved, position, words, zoom, encoding, language) | the nine `status` items above; the bar draws the point and its narrow-width rules |
| `preview/Preview.svelte:110-135` (zoom −, fit, +, jump to cursor) | `preview/toolbar`: `typst.preview-zoom-out`, ‹fit level›, `typst.preview-zoom-in` (own names: the text's « Zoom » is another thing), `typst.jump-to-preview` |
| type-id reads `App.svelte:150-152` (`barLang`, `hasBar`), `:156` (`lang === "typst"`), `StatusBar.svelte:98`, `ui/toolbar/spec.ts:150` | `hasToolbar` = some `format` item shows; the `typst` literals move into `ext/typst`'s `when` |

Kept as widgets (a value picker or a data list is not a command, as VS Code's encoding quick pick): `EncodingMenu.svelte:25`,
`SheetStatus.svelte:81,103`, the problems list `StatusBar.svelte:112-158` (moves inside `typst.problems`), its
Shift+click and context gesture `:99`, `StatsPopover`, the grid's `FilterMenu`. K-7 (the closed Typst engine):
`language/client.ts`, `App.svelte:379` (`language.open` for a Typst file).

## 3. Rules (test titles in the gate; each has a mutant caught by that title only, `scripts/netsim/mutants.toml`)

| Rule | What | Mutant |
|---|---|---|
| L-1 | `menus` and `status` validate against `LOCATIONS`; the test reads every `invalid/` vector and checks its reason code: unknown location, a group of another location, a protected group or `header/*` from a non-essential manifest, a `plugin` status entry, a built-in item on a reserved location | ML1 a group of another location accepted |
| L-2 | fixed groups, then `order`, extension id, command id; an extension group after the core's, whatever the build order | ML2 ties left in build order |
| L-3 | item `when` hides, command `when` greys with its reason, both on the live context; `text` and `canPresent` read | ML3 an item shown with a false `when` |
| L-4 | overflow by width keeps the order and loses nothing; status extensions past 3 go to ⋯; the narrow-width rules per group | ML4 the last item dropped |
| L-5 | header bar from `header/start` and `header/end` around its fixed widgets; Present only for a Typst main document | ML5 `header/end` groups reversed |
| L-6 | formatting bar = `format` items whose `when` holds; no item, no bar (`hasToolbar`); heading menu from `format/heading`; hints from the syntax tables | ML6 `hasToolbar` true with no item |
| L-7 | status bar from the point by side and group, the core's first; `typst.problems` only for a Typst file | ML7 `side` ignored |
| L-8 | preview tools and zoom menu from their locations, each with its own name; the preview's zoom acts on the preview, the text's on the text | ML8 `typst.preview-zoom-in` runs the text zoom |
| L-9 | items indexed once per location at build (`menu(loc)` returns the same array at each call); drawing calls no extension; status entries counted (start closure) | ML9 `menu(loc)` builds a new array at each call |

End to end, Chromium and WebKit: `EX0c1-E1` in the header bar, the formatting bar and the preview tools, every
button and menu item is either its location's resolved item, in order, or a named fixed widget, so a hard-coded button
without `data-cmd` fails (a Markdown file has no underline, a CSV no bar, 800 px fills ⋯); `EX0c1-E2` the status
bar's items for `main.typ`, a note and a picture, its layout at 640 px, and the zoom menu steps. Replayed (Chromium):
EX0b1-E2, UI-02…UI-07 (UI-06: axe in both themes), UI-11 (screenshots), ER-1–4, PRV-100, PRV-101, PRV-105, PRV-107,
« B2 text zoom », PRS-06, PRS-31, PRS-33, PRS-37 (`present.spec.ts`), UX-L2-1 (`ux/l2.spec.ts:58-70`), SHEET-FREEZE
(the grid's zoom, `sheet.spec.ts:498`), JOIN-3 (the header helpers of `e2e/helpers.ts`); WebKit: UI-05.

## 4. Performance, gate, risks

Start: PERF-START paired against `main` (ADR 0038), a regression guard; the built-ins' status entries are imported
with the first screen, never as chunks of their own, and the manifests ship without their JSON whitespace (Vite plugin
`src/build/compactManifests.ts`, code review 2026-11-25); the bar's height is fixed. Gate (max 45 min): `docs`, `no-hard-locations` (no command placed
in markup, no bar, tab list or context menu outside its host, no bar table, the points filled, EX0's type-id lint with
its list narrowed), `frontend`, `mutants`, `e2e`, `e2e-webkit`, `perf-start`, `simplify-report`. Risks: a lazy status
item flashing in (fixed height, L-9), the overflow at narrow widths (UI-05 on both engines), the problems counter's
composite name (ER-1–4).
Format bar focus: moved with Tab, as today (UI-03, `e2e/shell.spec.ts:51-53`); arrow-key roving focus inside a toolbar (WAI-ARIA toolbar pattern) is a change of behaviour, not a move, and goes to UX1-D with the rest of the bar design (decision of 2026-11-25, `docs/autonomous-decisions.md`).

## 5. EX0c2 and EX0c3 (own plans and gates at their step 1): sites

- **EX0c2**: `Sheet.svelte:254-330,384-470,530-560,643`, `ContextMenu.svelte:70`, `menu.ts`, `Grid.svelte:672`,
  `Sheet.svelte:413,436` (`shortcutOf`); a `panels` point (`viewsContainers`): `Sidebar.svelte:30-34`,
  `HistoryDrawer.svelte:47,83-86`, `App.svelte:605-607`; `App.svelte:633-645` (view bar); `HistoryBars.svelte:136-156`;
  `DiffView.svelte:175-188`; `App.svelte:100,711,724` (`ownView`).
- **EX0c3**: `InsertMenu.svelte:157,202,216,233,259,278,282`; `editor/format/insert.ts:66`; `support.ts:12-26`;
  `format/commands.ts:381,396`; `stats/words.ts:152-153`; `editor/setup.ts:78`; `language/problems.svelte.ts:159,195`.

## 6. Out of scope, and why

Showing « par X » and reordering or hiding an extension's item (no plugin origin before EXT1; the resolver keeps each
item's origin for it); the header's « Extensions » button and pinning (§15.13, EXT1); host-drawn status text for plugins
(EXT1, rule V values); the search panel (unmounted, UX1-2); a person's row menu (`people/context`, UX1-D); the history
drawer's row actions (redesigned by W65); the macOS menu bar (UX1-1); dialogs, popovers and windows of extensions (§15.13,
EXT1).
