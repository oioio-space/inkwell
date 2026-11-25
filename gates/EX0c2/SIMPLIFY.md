# EX0c2 - /simplify review

Reviewer: Claude Opus 5.5 (block agent), from four parallel review passes run by Sonnet subagents: reuse,
simplification, efficiency and altitude, on the whole uncommitted diff of the block (2026-11-25). I judged each
finding against the frozen gate `gates/EX0c2.toml`, the plan and the invariants, and applied the fixes by hand. A
move changes no behaviour, so a finding whose fix would change what the person sees was set aside.

## Applied

- **One build of a browser test's copy.** `run_playwright` repeated `run_e2e`'s frontend build and its two
  `go build` loops. Both now call `build_tree` (`scripts/netsim/mutants.py`). It also takes the `-tags` with each
  binary rather than testing the name. The Playwright run goes through `heavy.sh` with a `timeout`, as the other
  kinds do. `mutants_test.py` follows the shared helper.
- **The context menu draws an item once.** `LocationMenuAt.svelte` had the same `CommandMenuItem` and lead three
  times (core groups, flat extensions, the submenu). There is now one snippet, and the wrapper around `onclose` is
  gone.
- **The tabs' keys are a table.** `PanelSide.svelte` used a four-level ternary. It is now `MOVES`, from key to
  index (Left and Right wrap, Home, End). MV3 was re-anchored on it.
- **A panel's focusable check watches its own children only.** The `reachable` action observed the whole subtree,
  so it ran a `querySelector` at every change inside a long list. It now observes the panel's direct children: a
  body adds its first content once.
- **The sides come from `SIDES`.** `panelSelection.svelte.ts` looped over a hard-coded `["start", "end"]`.
- **One view bar.** `ViewBar.svelte` drew two bars with the same CSS. It now draws one bar that holds whichever
  widgets apply: the offer and the default viewer never show together, since a text offered as a table is shown
  by its default viewer.
- **The grid's page actions are required.** `GridTarget`'s `copy`, `paste`, `filter` and `autofit` were optional and
  called with `?.`, which made a silent no-op possible. Every caller gives all four.

## Set aside

- **A shared overflow component for the formatting bar and the grid's bar.** Both now use `fitCount`. The
  formatting bar keeps its own frame because its heading menu and Insert are fixed widgets measured in `BAR`, and
  merging the frames would move UI-05's and UI-11's pixels. → UX1-D, the bar design (with arrow roving in toolbars,
  EX0c1's decision).
- **Shared roving-menu keys between `MenuButton` and `LocationMenuAt`.** The two behave differently: one is a
  popover with a trigger, the other a menu at a point with a submenu. A shared helper is a refactor of
  `ui/popover/`, outside this diff. → UX1-D, which moves the six context menus onto `LocationMenuAt`.
- **`panelEntries.ts` beside `statusEntries.ts`.** The two could become one eager-entry module. The frozen lint names
  `extensions/panelEntries.ts`, and EX0c1's status module is outside the diff. → PF3, the start chunk budget, which
  owns what the first screen imports.
- **A manifest field for eager or idle loading, instead of the file names `panels.ts` and `drawer.ts`.** Today the
  left side is the only eager side and P-4 tests it. A field would be an untested switch before a plugin can
  contribute a panel. → EXT1.
- **Panel ids as constants (`PANEL`), and the drawer's initial selection.** A « Show » command per panel (VS Code
  `workbench.view.*`) would let the core run a command instead. Those commands are new palette entries, which is a
  behaviour change. → UX1-D (plan §5, out of scope).
- **The per-side class and test id in `PanelSide.svelte`.** These keep today's CSS and the `history-drawer` test id
  (review rule: preserved names and testids). The width belongs in `SIDES` once a third side exists. → UX1-D.
- **The history strip writes `unseen` with `setLive`, rather than being a surface.** The strip lives as long as the
  window. Making it a surface would also change where Ctrl+Z goes from the strip's buttons. → UX1-D.
- **The default viewer, the table offer, the colour and the diff mode as commands.** The plan and the review keep
  them as value picks, as EX0c1 did with the encoding. Making them commands adds palette entries. → UX1-D.
- **Per-button context copies (`availability` and `isOn`, each reading the surface's state).** This is the same
  mechanism and cost as EX0c1's `context()`, now with one small `state()` object per call. The gate's PERF-START
  passes, and typing does not run it. → EX0b3, with the palette's PERF-PALETTE that measures `context()`.
- **Hidden panels stay mounted.** This is required: every tab's `aria-controls` must name an element (review
  minor 6).
- **The prefetch at idle.** It is required by review minor 5 (no empty frame, a first opening within 100 ms).
- **The main-view focus selector in two places** (`PanelSide.svelte`, `Files.svelte`). Both were moved unchanged
  from App.svelte. A `focusShownView()` belongs to the session, which EX0c3 touches. → EX0c3.

## Verified

- **Departures from the plan, and the lint's literals.**
  - The width overflow is one function (`fitCount`), not one component. Plan §2 now says so.
  - In `DiffView.svelte`, `go` was renamed `stepChange`. The frozen lint's `DRAWS` reads `go(1)` and `go(-1)` as
    the old buttons, and the surface's handler map held the same text.
  - The grid's bar decides the focus by its own state: the grid takes its focus back unless the filter menu is now
    open. The plan's « except `tables.filter` » would otherwise need a literal command id in `Sheet.svelte`, which
    the lint forbids.
  - « Suppr » no longer shows on « Effacer le contenu », because a bare key is no chord (EX0b1). Plan §3 says so.
    Cut, copy and paste show their chords as `delegated: "system"`, which is the browser's own clipboard event,
    not a widget's keymap.
  - The ⋯ menu of the grid's bar is named « Plus d'actions », as the formatting bar's is.
- **Existing tests changed, and why:**
  - **`ux/w58a.spec.ts:276,310,381`:** « Afficher Relire » became the « Relire » toggle (EX0b1/EX0c1). This named
    edit was decided on 2026-11-25.
  - **`ui/history/history.test.ts`:** HIS-09 and the writer test render the drawer as the end side of the point
    (`PanelSide`, its bodies loaded), since `HistoryDrawer.svelte` is gone.
  - **`extensions/commands.test.ts`:** C-7 has rows for the 19 grid commands and `app.mark-seen`, and the grid
    surface's target gives the four page actions. In C-7 and C-8, the diff surfaces now say `changes` (the next
    difference's new `when`).
  - **`ui/command/locations.test.ts`:** L-1's reserved-location case moved from `grid/context` (now hosted) to
    `insert` (EX0c3), and it checks that the five EX0c2 locations are hosted. Its source helper gives a `panels`
    entry file.
  - **`extensions/manifest.test.ts`:** a valid vector's entries are files of its folder, for the panels vector.
  - **`extensions/testdata/manifests/invalid/reserved.reserved-location.json`:** now names `insert`.
- **Measured.**
  - The first `import()` of a chunk larger than the drawer: at most 5.4 ms in Chromium and 8 ms in WebKit
    (`WITNESSES.md`).
  - MV13: caught, 7 s (`mutants.py EX0c2 MV13`, copy built, EX0c2-E1 failed with « stray »).
  - The replayed suites before the drawer's split: 31 passed, and W58a-UX-1e passed after its selector edit.

## Review (blind code review, 2026-11-25: « accept with fixes », no major)

The review found parity with HEAD, no duplicate label, justified guard overrides, and changed tests that were
adapted rather than weakened. I wrote each fix below test-first: the named test failed, then passed.
`gates/EX0c2.toml` is untouched.

1. **A panel whose body fails to load no longer blocks its side.**
   - Cause: `sideReady` waited for every body, and the registry kept the rejected promise.
   - Fix: a failed panel counts as done and its tab says so (`panel.unavailable`). The failure is announced in the
     toast (`notice`, `panel.failed`), and `loadSide`'s cache entry is dropped.
   - The registry's `load` no longer keeps a failed load, so the next opening tries again. A side's load is now
     cached per registry.
   - Test: « a panel that fails to load counts as done, says so, and is tried again at the next opening ».
2. **The ⋯ menu of the grid's bar gives the grid its focus back.**
   - Cause: `CommandMenuItem` closed the menu with `refocus` (true for a toggle, so `session.focus()` went to the
     text), and `after` was never called.
   - Fix: the overflow items close with `refocus={false}`, and a new `onrun` calls `after(id)` once the menu has
     left the page. `tables.unfreeze` from ⋯ now focuses the grid.
   - Test: « an item run from the bar's ⋯ menu closes it, then gives the grid its focus back ».
3. **The drawer's prefetch waits for the editor.** `prefetchSide` starts its idle fetch only once `.cm-content` is in
   the page (PERF-START's mark), with a `MutationObserver` that disconnects itself when that happens, as plan §1
   says. Test: « the drawer's code is fetched only once the editor shows ».
4. **F7 and Shift+F7 with no change are taken again, and they say why.**
   - Cause: a surface's chord whose `when` failed went on to the page. This was meant for the text, whose keymap
     its languages extend (Ctrl+U in Markdown is CodeMirror's).
   - Fix: only an `editor` chord goes on. A grid's or a diff's chord is the application's, because their own keys
     are a closed table (OWNED_KEYS). It is taken and its `unavailable` is said, as HEAD took F7, and F7 no longer
     reaches the webview's caret browsing.
   - Test: « F7 where there is no change to go to is taken and says why ».
   - Same rule in the grid: Ctrl+D or Ctrl+R typed in a cell (`!field` fails) is taken and says « Pas pendant la
     saisie dans une cellule », where before it went on to the field and the webview (Ctrl+R reloaded the page).
5. **The playwright kind's caps sit under the gate's 240 s.**
   - Caps: `PLAYWRIGHT_CAPS` gives 50 s for the frontend, 50 s for each Go binary and 80 s for the test (75 s for
     the inner `timeout`), 230 s in all.
   - Signals: `mutants.py` now handles SIGTERM and SIGINT (`install_signals`, `on_signal`). The handler kills every
     running child's process group (`CHILDREN`, kept by `sh`) and exits, so each `TemporaryDirectory` removes its
     copy of the tree.
   - Tests: `PlaywrightCapsAndSignalsTest` (the caps, the kill and the unwinding, the handler's installation).
6. **The selected drawer tab's highlight is kept.** HEAD's `HistoryDrawer.svelte:365-368` had the same rule
   (`.tabs button[aria-selected="true"]`, `--popover-bg` and its shadow), so it was moved, not added. There is no
   visual change.
7. **Greyed context-menu items are focusable.** HEAD's `ContextMenu` used `disabled`, which skipped a greyed item.
   `CommandMenuItem` uses `aria-disabled`, so a greyed item can now be focused and says its reason when used. This is
   APG-conformant: the menu pattern lets a disabled item take the focus. Items hidden by their `when` (the
   read-only ones) stay out.
8. **Ctrl+Shift+H opens the drawer.** The new assertion in EX0c2-E2 failed first.
   - Cause, from before EX0c2 (reproduced on HEAD's `keys.ts` in a scratch test): with Shift held and the key typing
     its own base letter (« h », Caps Lock on, or Playwright's `Shift+h`), `eventChord` tried Ctrl+h before
     Ctrl+Shift+h. Ctrl+h is Replace (a grid's chord), so the drawer never opened.
   - Fix: Shift that typed nothing is part of the chord, so Ctrl+Shift+h comes first (`keys.ts`, `capsShift`).
     Caps Lock without Shift is unchanged, and CodeMirror's equivalence tests (`keytables.test.ts`) stay green.
   - Tests: « Caps Lock on: Ctrl+Shift with a lowercase letter is the Shift chord even when the plain one is bound »
     (unit), and the E2 assertion.
