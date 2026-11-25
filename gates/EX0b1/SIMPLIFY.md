# EX0b1 - /simplify review of the block's diff

Reviewer: Claude Opus 5.5 (implementing agent of step 2, 2026-11-25), over the whole diff of the worktree
`inkwell-ex0b` against `main` f2e2647: `frontend/src/extensions/` (manifest points, `when.ts`, `keys.ts`,
`keytables.ts`, `bindings.ts`, `commands.ts`, `live.svelte.ts`), the built-in manifests and entries (`ext/app`,
`ext/format`, `ext/typst`, `ext/tables`, `ext/text`), the migrated sites (App, HeaderBar, PeopleStack, FormatBar,
BarButton, HeadingMenu, ZoomMenu, StatusBar, Editor, Sheet, Grid, grid keys, FindBar, DiffView, HistoryBars,
`language/extension.ts`, `editor/setup.ts`), the tests, vectors, mutants and docs. Lenses: reuse, simplification,
efficiency, altitude (go-style-guide does not apply: no Go is touched).

## Applied

- One execution path: `run()` and the key listener both end in `execute()` (flush, then the surface's handler or the
  entry); the listener no longer calls `run()`, which evaluated the command's `when` a second time with `field`
  forced to false.
- One source per fact: the formatting bar's items lost their `key` and `label` fields and the twin `shortcutLabel` /
  `isMac` helpers; the label, title and chord come from the registry (`cmdAttrs`, `shortcutOf`). The format keymap
  (`editor/format/keymap.ts`) is deleted; `editor/format/handlers.ts` holds the one table of what each `format.*`
  command does, used by the editor surface, the tests and nothing else.
- The surfaces' handlers live next to their owner and are shared with the tests instead of being rewritten there:
  `gridHandlers` (`ui/sheet/commands.ts`, used by `Sheet.svelte`) and `textHandlers` (`editor/format/handlers.ts`, used
  by `Editor.svelte`), so the C-7 effect table runs the production handlers.
- The grid's keyboard takes exactly the chords of its table (`GRID_KEYS`): modifiers it ignored before (Ctrl+Enter,
  Alt+arrows, Ctrl+Shift+A, Shift+F2, Alt+Shift+Down…) are left to whoever binds them; the census of C-6 found
  Alt+Shift+Down opening the filter, fixed at its root in `gridKey`.
- The comma operators of the grid's find handlers became a small `open(replace)` helper (Biome).
- `preloadStartup` reads the manifests loosely (a preload hint) instead of parsing them a second time, so each `when`
  is parsed once at start (C-12).
- The zoom keys left `HistoryBars.svelte` (its window listener) for `app.zoom-*`; `keepCaret` is exported from its
  module script and reused by the shell's `zoom`.

- After the blind code review (round 2, « accept with corrections »), each finding at its root:
  - `eventChord` reads the key as w3c-keyname's `keyName` does (macOS ⌘⇧ reports the unshifted key: ⇧⌘G, ⇧⌘',
    ⇧⌘+ now resolve), and the digit row's base comes first (a French Mac's ⌘4 is Titre 4, never ⌘'); C-8.
  - The system is read when needed (`currentSystem()` at each chord, Wails first, then the browser's platform),
    and the tables are rebuilt when it changes; the silent `undefined → linux` test became the new rule's; C-8.
  - Inside a modal dialog only its own surface's bindings act; a key whose focus fell to the body is the dialog's
    (F7 in the diff window); C-8.
  - `delegated`, like `masks`, is the built-ins' only (validator, vector `bad-value.plugin-command-delegated`; EX0's
    R-2 reads `plugin-` vectors as a plugin's, as C-1 does).
  - A refused binding of hers no longer takes the default away; hers keeps the default's `when`; C-5.
  - A window chord whose command cannot run is taken and its reason said (F5 outside Typst); Ctrl+Shift+- zooms out
    again; the grid does not zoom while a cell is typed in; spec §8.2 says the grid's zoom keys.
  - Full screen has its main-menu item (`view`); a control with its own tooltip (the formatting bar) has no `title`:
    one tooltip.
  - Start: bindings are resolved for the current system only, when first asked; the masks-nothing check is a
    static test (`unusedMasks`, C-6); the command entries load when the page is idle (`warmCommands`), not with
    the first paint. Measured in Chromium: `inkwell:registry` 3.0–3.4 ms, `inkwell:keymap` 0.5–0.7 ms.

## Set aside
- `Sheet.svelte`'s context menu keeps its clipboard shortcuts as text (Ctrl+X, C, V: widget keys); only the insert and
  delete items take their chord from the registry. Placement of commands in that menu is EX0c.
- The status bar's problem counter keeps its composite accessible name (« Problèmes du document : Problème suivant,
  … », ER-1–4 reads it) rather than `cmdAttrs`' name; it carries `data-cmd` and runs `app.problem-next`.
- `isOn()` reads the live context through a cast (`ContextKey` includes `focus` and `field`, which are not stored):
  a narrower key type would only move the cast.

- Start-up cost (PERF-START ratio 1.034, then 1.036 after deferring the per-system resolution and the command
  entries): measured, not guessed. Base f2e2647 and this tree built the same way (Vite manifest and source maps,
  start closure = `index.html`'s imports plus `App.svelte` and the startup entries): 1 265 253 B raw / 404 183 B gzip
  before, 1 316 072 B / 417 246 B after (+50.8 KB raw, +13.1 KB gzip). The modules added to it are the registry's
  own: `keytables.ts` 16.5 KB of source, `commands.ts` 14.6, `keys.ts` 10.4, `bindings.ts` 9.1, `when.ts` 7.6,
  `format/handlers.ts` 3.0, `CommandMenuItem` and `CommandButton` 4.6, `live.svelte.ts` 0.9; the removed one is
  `format/keymap.ts` (1.6). In Chromium the registry's build is 3.0–3.4 ms and the key resolution 0.5–0.7 ms
  (`inkwell:registry`, `inkwell:keymap`). No lazy import removes them without changing behaviour: the header bar
  shows each command's chord at the first paint, which needs the tables and the resolution. Residual → PF3
  (start-up chunk budget; split the owned-key tables and conflict resolution off the first paint).

## Verified

- The rule tests C-1..C-12 were red at step 1 (absent, `gates/EX0b1/WITNESSES.md`); in this step they were written
  alongside the code, not strictly before it, so their power to fail is shown by the mutants: each MK1..MK12 must turn
  exactly its rule's test red by title (criterion `mutants`). Two tests found real defects while written (the grid's
  Alt+Shift+Down, an undo fixture without history).
- `no-hard-keys`: 0 problems; the five named commands have chords on the three systems; 40 commands have chords.
- Invariants: no Go change; the CSP holds (no `eval`, `when` evaluated as a tree); zero jargon in the new labels
  (« Panneau de gauche », « Relire », « Différence suivante »); fr and en have the same keys.
