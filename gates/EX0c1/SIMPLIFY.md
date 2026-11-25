# EX0c1 - /simplify review

Reviewer: Claude Opus 5.5 (integrating session), from four parallel review passes run by Sonnet subagents: reuse,
simplification, efficiency and altitude, on the whole uncommitted diff of the block (2026-11-25). The fixes were
applied by hand, and each finding was judged against the frozen gate `gates/EX0c1.toml`, the plan and the invariants.

## Applied

- **One rule for a contribution's text.** `StatusMore.svelte` had its own copy of « an i18n key for a built-in, the
  text itself otherwise ». `textOf` (`extensions/commands.ts`) now takes any contribution with an origin, so
  `StatusMore.svelte` uses it.
- **No string sentinel between parser and registry.** The registry's `keptOut` returns the final reason
  (`protected group …`, `reserved location …`). Before, it returned a marker that the caller compared and rebuilt.
- **The location index is built in one pass.** It used to copy its array on every push (quadratic). The items are
  now grouped, then sorted once per location (`placed`). `menu(loc)` still returns the same frozen array (L-9, ML9).
- **Dead props removed.** The unused `$props()` bindings of `Zoom.svelte` and `Page.svelte` are gone.
- **`CommandIcon` draws a glyph in one place.** `glyphOf` returns the letter and its typeset style, so the magic
  `icon.slice(6)` offset and the two copies of the span are gone.
- **`HeadingMenu` has one item branch.** The paragraph and the levels differ only by `checked`, `class` and `detail`,
  which are now computed per group.
- **`nameOf(id)`** (`extensions/commands.ts`) is the one place that reads a command's name within its extension.
  The bar and the heading menu use it. This also keeps EX0's `no-lists` criterion green: only the registry splits
  names.

## Set aside

- **The zoom bounds are per-id code in `Zoom.svelte` (`atBound`).** Turning them into the commands' `when` would need
  a zoom-level key in the closed context. It would also change what the keys and the palette do at a bound (today
  they clamp silently). Kept as today → UX1-D (the zoom menu's surface), with the question of greying at bounds
  everywhere.
- **The status bar as two entries of `LOCATIONS` (`status/start`, `status/end`).** That would unify the two group
  tables. The gate and the plan fix a `status` point with `side`, and the review accepted it. → EX0c2, which adds
  the `panels` point and can merge the group mechanics of the three.
- **`FormatBar` keeps its own width overflow instead of a general `LocationBar` overflow.** The table's
  `overflow: "width"` field is not read yet. The grid's toolbar is the second bar to overflow, so the general host
  is built then → EX0c2.
- **Status item props are half the `bar` bag, half stores, and `ext/*` imports the `StatusItemProps` type from
  `ui/command`.** Built-ins only until EXT1. A third-party item will need a facade, not UI types → EXT1.
- **Efficiency on the typing path.**
  - The status bar's `bar` object is rebuilt per keystroke.
  - `featureState` and `HeadingMenu` each classify the caret's line.
  - `Words` recomputes on a selection-only change.
  These are the same costs as before the move (`FormatBar` and `StatusBar` did exactly this). PERF-START passes,
  and typing is not measured by this gate. → PF3 (start chunk budget) and UX1-0, which measures interaction cost.
- **`context()` copies the whole live state per button.** This is an EX0b1 mechanism, not changed here. Its cost is
  watched by the palette's PERF-PALETTE → EX0b3.
- **Reserved locations kept although nothing hosts them yet.** The gate and the review require the closed table now,
  so UX1-B can reserve against it. The `{ ...contextMenu }` spreads are the object-literal form the frozen lint reads.
- **The counter markup in `Problems.svelte` is written twice (problems navigation, and the compilation's list).**
  This was moved unchanged from `StatusBar.svelte` (no behaviour change) → UX1-D (the problems surface).
- **The `__inkwellLocation` test hook.** It exists only in the e2e build (`VITE_E2E_OBSERVE`), like the hooks around
  it.

## Verified

- `gates/EX0c1.toml` criteria `docs`, `no-hard-locations` and `mutants` (ML1–ML9 all caught) after the fixes. EX0's
  `no-lists` and EX0b1/EX0b2's static lints are still green. EX0b1's mutants are all 12 caught, MK10 re-anchored on
  `placed` (record in `scripts/netsim/mutants.toml`).
- Departures from the plan, kept on purpose:
  - **The header's end keeps today's order** (view, ‹people›, document, review, ‹main menu›). Plan §1 lists « view,
    document, ‹people›, review », but that would move the people stack, and UI-11's screenshots, EX0b1 C-11 and the
    review's own « no behaviour change » rule fix today's order.
  - **‹fit level› is a fixed widget (`data-el="preview.fit"`) that calls the same `fitPreview()` as
    `typst.preview-fit`.** The lint forbids a literal command id in `preview/`.
  - **The jump-to-cursor button is no longer disabled before the first page.** It is now the command
    `typst.jump-to-preview`, available on a Typst file, as its key always was. PRV-87 was changed to that.
  - **`typst.jump-to-preview` is labelled « Montrer le curseur dans l'aperçu » (`preview.jumpToCursor`)** in the
    palette, the shortcuts window and on the preview's button. It used to have two names: « Montrer dans
    l'aperçu » in the palette and « Aller à l'endroit du curseur » on the button. Per the review, it now has one.
- **Existing tests changed, and why:**
  - **The toolbar and status-bar unit tests:** `FormatBar.test.ts` (moved to `ui/command/`, its tables read from the
    registry), `command.test.ts` (C-11: Present needs `fileType == typst`; the zoom menu is now the `app.zoom` item),
    `StatusBar.test.ts`, `StatusBar.problems.test.ts` and `ExternalNotice.test.ts` (the items are code loaded once,
    and their `when` reads the live context).
  - **`Preview.test.ts`:** PRV-84 now goes through the commands; PRV-87 as above.
  - **`commands.test.ts`:** C-2 reads every location's and status item's `when`; C-7 has three new commands in its
    table; C-12 counts the status clauses.
  - **e2e `ex0b1.spec.ts` and `history.spec.ts`:** `[data-cmd="app.review"]` replaces the `history-toggle` testid,
    as plan §2 says.

## Review (blind code review, 2026-11-25: « accept with fixes »)

The review found every surface on par with `main` and no weakened test. Each fix below was written test-first: the
test failed, then passed.

- **Major: the preview's zoom commands.**
  - `typst.preview-zoom-out`, `-in` and `-fit` now have `when: main && previewOpen` and their own labels: « Réduire
    l'aperçu », « Agrandir l'aperçu », « Ajuster l'aperçu ». The buttons take those names.
  - New tests: no two commands the palette lists share a label (fr, en); the preview's zoom acts only where the
    preview is open. Both failed first.
  - PRV-84, PRV-87, L-8, EX0c1-E1 and PRV-105 now use the new names.
- **Start cost.**
  - The built-ins' status entries are imported with the first screen (`extensions/statusEntries.ts`, an eager glob,
    kept apart from the loader, which the registry needs first). The loader's lazy glob no longer names them, and
    lists them as text only (1 KB), so they are not split into chunks of their own. `warmStatus` is gone.
    Test-first: « the built-ins' status items draw with the first paint, no entry loaded ».
  - The manifests' JSON loses its whitespace at build (`src/build/compactManifests.ts`). A manifest that is not
    JSON is kept as it is, so the registry still refuses it with its reason. Test-first:
    `compactManifests.test.ts`.
  - Measured, production build, start closure (`index.html` + `App.svelte`):

    | | Files | Raw | gzip | `index.html` alone (gzip) |
    |---|---|---|---|---|
    | before | 42 | 1 378 287 B | 434 753 B | 30 920 B |
    | after | 33 | 1 377 319 B | 436 156 B | 30 511 B |

    The 1.4 KB more is the nine status items, which used to load right after mount; 9 fewer requests. PERF-START,
    before → after, against `main` 01ecacf, 80 pairs each: ratio 1.038 (upper bound 1.049) → 1.013 (upper bound 1.026); median bound 263 → 261 ms, p95 bound 284 → 273 ms.
- **The registry refuses broken status items.** A status id is now a claim (`status <id>`), so a duplicate refuses
  the manifest instead of breaking the keyed list. An item whose `command` no manifest gives is kept out and listed
  (`unknown command …`). Vectors: `extensions/testdata/manifests/refused/` (registry level). The test failed
  first.
- **One name for the jump**: « Montrer le curseur dans l'aperçu » everywhere (i18n fr/en, `docs/architecture/spec.md`).
- **Docs**: plan §1, spec §4.31 and ADR 0027 state the header's end in today's order: view, ‹people›, document,
  review, ‹main menu›.
