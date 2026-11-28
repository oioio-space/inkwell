# EX0c3 - /simplify review of the block's diff

Reviewer: Claude Opus 5.5 (the step-2 implementer session, reviewing its own diff before the gate run). The blind
review of the code after the smoke (method v2) is still the integrator's to run.

Scope: the whole diff of the block against `main` 99ff3f9 (EX0c3t frozen): `editor/format/*`, the two format entries
`ext/{typst,markdown}/native/format.ts`, `extensions/{manifest,registry,loader,formatEntries,when,commands,index}.ts`,
the manifests, `ui/command/{InsertMenu,FormatBar,HeadingMenu}.svelte`, `App.svelte`, `editor/setup.ts`,
`ui/Editor.svelte`, `language/{client,problems}`, `ui/sheet/Sheet.svelte`, the status bar, the vectors, the rule
tests, `e2e/ex0c3.spec.ts` and `scripts/netsim/mutants.toml`. Checklists: `go-style-guide` (no Go touched),
`vuln-guard`, `secret-guard`, `simplify`, `helper-ergonomics`.

## Applied

- `InsertMenu.svelte`: an item's label read with `labelOf(command)`, the registry's own helper, instead of
  `textOf(command, command.label)`.
- `loader.ts`: the format entries' file names come from the eager glob's keys (`FORMAT_FILES`), not from a second
  `?raw` glob, which would have put both modules' text (about 20 kB) into the start bundle for a name check.
- `registry.ts`: the formats are built only when a format lookup is given (`opts.formatEntry`); test registries
  built without it no longer print a warning per grammar.
- The Typst quote call (`wrapQuotes`, `closingBracket`) moved with its language into its entry (`quote`), so
  `setBlock` holds no language test; the renderers moved likewise, `insert.ts` keeps only placement, preconditions
  and the step requests.
- `words.ts` keeps the two strip algorithms; the entries point at them (`strip`), so no text moves for nothing and
  `stripMarkup` is one line.
- `editor/csv/typst.ts` imports `typstString` from Typst's entry instead of keeping a second copy.
- The loader comment rewrapped (it had been cut in two by an edit).

## Set aside

- One table for the bar's hints and the Insert items' syntax (plan §1, « hints gain the Insert items' syntax »): the
  bar's `link` hint and the Insert menu's file link differ (`[texte](url)` and `[](…)`), so a shared table would
  change what one of them shows. `insertSyntax` stays its own field (docs/autonomous-decisions.md, 2026-11-28).
- An entry module in `ext/format` for the step commands (the ext/app pattern): native code needs an essential
  extension, and making `format` essential breaks C-4 (it disables `format`). The text surface handles all thirteen;
  a step item asks the shell (`Shell.step`), which C-7 asserts row by row.
- `kindHint` (the empty picker's message, a switch on the item) stays in the host: the `insert.empty.*` keys are out
  of scope (plan §5). → UX1-D.
- Each Insert item's name is its label alone (plan §2): its description and syntax sit beside the item's button in
  the row (the row is its hit area), since axe's label-in-name rule (UI-06) refuses a name that leaves out text
  inside the control.
- `INSERT_INPUT` lists `table` and `formula` (the bar's insertions) as « nothing »: kept so the table is total over
  `InsertItem`, which the type checker then guards.

## Verified

- No behaviour change beyond the plan: the same items in the same order per language (FormatBar UI-01 tests pass
  unchanged in substance), the same text per insertion (insert.test, C-7), the same cleanup, words and writing
  help (commands.prop, stats, keytables census), the same problems jump for the grid.
- Declared changes only: items named by their label and described by `aria-describedby`; the thirteen commands in
  the palette; a big file read-only in the context; `ext/text` takes the thirteen commands (`DELEGABLE`).
- Security: the `format` entry is refused to any origin but a built-in (F-1, MI1); `accepts` keys are a closed list,
  `readers` values an identifier (no path or expression reaches a Typst call), objects null-prototype and frozen.
  No secret, no new dependency, no network, no `eval`.
- Cost: the format entries were static imports of the start closure before; now an eager glob of the same two
  modules (PERF-START is the gate's guard).

## Code review (blind, « accept with fixes », no major)

- Applied: a step asked for from the palette is dropped when the file changes or the bar hides before a menu
  took it (`noteInsertHost`, fed by App.svelte), so no menu opens later at a stale step; the bar the shell shows
  for the step keeps it. Test « EX0c3 review a step asked for one file or one bar is dropped… », red first.
- Not applied, the gate forbids it: `format.insert-file-link`'s `when` gaining `&& !(attach in inserts)`.
  `no-hard-locations` requires every insert command's `when` to be exactly `<item> in inserts && !readOnly`
  (EX0c3t line 197), and moving the rule into `inserts` would leave MI4 uncaught. In a .typ file the palette
  offers « Lien vers un fichier », which works: it writes `#link(…)` after its file step. Removing that entry
  needs a gate that names this `when`. → UX1-D.
- Not applied, no site: the palette shows a command's label and, when greyed, its reason; it never shows a
  `tooltip` (`Palette.svelte:198-199`; `tooltip` is read only by the Insert menu). Markdown's « Copie figée » is
  already its menu's description through `describe`, and a note opens read-only, so its palette entry is greyed
  with its reason.

## Lesson

- A mechanical criterion is frozen only after its codemod has run against the implemented contract: EX0c3's
  codemod was proved reversible against the merge-base, but three of its outputs broke once `Lang` was an object
  (a constant read as a type id in `fileType`, a cast to `string`, a default parameter that turns `undefined` into
  Typst), so the gate was superseded three times (EX0c3r, EX0c3s, EX0c3t). Run the codemod's output through the
  type checker and the touched tests on the new contract before the freeze.

## Residuals

- The Insert popover can run under the status bar when it is taller than the space below the bar (seen at the e2e
  window size: the last item is covered, so E2 runs the items from the keyboard). The host placement is today's
  (inside the bar), so the defect predates EX0c3. → UX1-D, the bar's design.
- A Markdown note is a read-only projection without a bar, so its Insert menu (frozen CSV copy, note) cannot be
  opened end to end; F-4 and F-5 prove it in the component, E1 and E2 check the note's status bar only. → UX1-D.
