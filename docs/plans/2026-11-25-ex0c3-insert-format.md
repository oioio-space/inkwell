# EX0c3: the Insert menu as the `insert` location, the formatting language as a format entry of a grammar

Rigour: standard (no peer bytes, no trust state, no sandbox boundary). Follows EX0c2 (plan
`docs/plans/2026-11-25-ex0c-locations.md` §5 lists these sites; `LOCATIONS` reserves `insert` for this block). Gate
`gates/EX0c3.toml`, witnesses `gates/EX0c3/WITNESSES.md`; ADR 0027 gains « The Insert menu and the formatting language
(EX0c3) » with the code. **Not split:** 25 sites in 14 files, the touched suites replay in about 2 min (witnesses).
A move changes no behaviour; when in doubt today's stays (EX0c2 review). The one declared change is in §2.

## Prior art (read 2026-11-25) and what is better here

VS Code splits a language in three contributions: `languages` (`id`, `aliases`, `extensions`, …, `configuration`, a
declarative `language-configuration.json`: comments, brackets, surrounding pairs, folding, word pattern, on-enter
rules), `grammars` (`language`, `scopeName`, `path`, `embeddedLanguages`) and `snippets` (`language`, `path`): what a
language writes on insertion is data keyed by the language, never a test in the editor
([points](https://code.visualstudio.com/api/references/contribution-points),
[language configuration](https://code.visualstudio.com/api/language-extensions/language-configuration-guide)). Menus:
`group@order`, `submenus`; items are commands. Today inkwell has the grammar point (ADR 0027: `grammars[]` `id`,
`names`, `entry`) but the formatting language is the closed union `Lang = "typst" | "markdown" | "plain"`
(`editor/format/support.ts:7`) and eleven sites compare it to a literal. **Better here:** the formatting language is one
entry of its grammar (as `configuration` is one file of its language), so a language added later gets the bar, the
Insert menu, the cleanup and the word count by contributing it, and the core holds no table keyed by a language name;
the file types an item takes are manifest data (as `snippets` are files of the manifest), never code; a snippet's
fixed text cannot take a picked path or a CSV's rows, so the renderer is code of the entry, `builtin` only until EXT1.

## 1. The format entry (grammar point; same validator, bounds and null-prototype rules as ADR 0027)

`grammars[].label` (required, i18n key: the language's name, today's `lang.typst`, `lang.markdown`; VS Code's
`aliases`), so a grammar without formatting still has a name (the status bar's language, the bar's « Mise en forme
Typst »); a file with no grammar reads « Texte brut ». `grammars[].format` (optional) holds formatting only: `entry` (a
`native/` module, `builtin` only, `ui.native`, as status and panel entries), `accepts` (`{item: [type id]}`: the file
types an Insert item lists; an item absent takes every file), `readers` (`{type id: name}`: Typst's data reader of a
file type; `read` otherwise). A grammar without `format` has no bar.
The built-ins' entries load with the first screen (`extensions/formatEntries.ts`, an eager glob as
`statusEntries.ts`): today `typst.ts` and `markdown.ts` are static imports of the start closure, so the start keeps
its cost. `registry.format(grammar)` returns the same frozen object at each call (indexed once at build).

The module's default export keeps the type name `Lang` (now an object; `undefined` is plain text) and holds what
today's code keys by the name:

| Field | Read by | Today |
|---|---|---|
| `syntax: LangSyntax` (marks, blocks, inserts, scan, classify, prefix, hints; hints gain the Insert items' syntax) | the bar (`formats`, hints, pressed), the heading menu, `active.ts`, `toggleMark`, `setBlock` | `support.ts:10-19`, `typst.ts:165`, `markdown.ts:141` |
| `emptyPairs: RegExp[]` | the convergence cleanup | `cleanup.ts:11-14` |
| `quote?: { wrap, unwrap }` (a call around quoted lines) | `setBlock` | `commands.ts:381,396` (`lang === "typst"`) |
| `strip(text)` | words and stats | `words.ts:151-155` |
| `render(item, args, ctx)`, `precondition?` | the insert commands | `insert.ts:78-200,228` |
| `describe?: {item: key}`, `note?: key` | the Insert menu (Markdown's « Copie figée », its note) | `InsertMenu.svelte:133-159,282-285` |

Context gains `inserts` (a set over the Insert items, from `syntax.inserts`), read by the items' `when`; `formats`
is unchanged. The Typst and Markdown tables move to `ext/typst/native/format.ts` and `ext/markdown/native/format.ts`;
`editor/format/` keeps the algorithms, which take a `Lang`, never a name. **Test edits are proved mechanical**
(review): `scripts/codemods/ex0c3_type_literals.py` rewrites 17 test files (about 325 lines: `"typst"`, `"markdown"`, `"plain"` become
the testkit's `TYPST`, `MARKDOWN`, `PLAIN`; the unions `TestLang`, `AnyLang`, `LangArg`); the gate's `mechanical`
criterion reverses them and must find the merge-base text (outside strings and comments, formatting-blind), the same
test titles per file and at least 1513 passed. Three named edits only, declared in the gate: L-1's reserved case
leaves `insert` for `editor/context` (`locations.test.ts`), C-7 gains one row per new command (`commands.test.ts`),
`context.test.ts`'s plain `lang` becomes `undefined`. A new test goes in a new file.

## 2. The `insert` location and its commands

`LOCATIONS.insert` loses `reserved` and its placeholder groups (`blocks`, `data`: never hosted) for today's:
**files**, **refs**, **layout**, extensions, each with a `caption` (`GroupSpec` gains it: `insert.group.*`, today's
captions; GNOME and APG menus allow a group heading). Thirteen commands in `ext/format`, labels today's `insert.<item>`,
tooltips today's `.desc`, keywords `kw.insert` (« insérer ajouter », « insert add »): `format.insert-image`, `-figure`, `-include`, `-attach`, `-csv-table`, `-data`,
`-bibliography`, `-file-link` (files), `-cite`, `-ref`, `-footnote` (refs), `-outline`, `-pagebreak` (layout); command
`when` `<item> in inserts && !readOnly`, reason `cmd.format.unavailable` (« Indisponible pour ce fichier ou en
lecture seule »), item `when` `<item> in inserts` (`-file-link`: `link in inserts && !(attach in inserts)`, today's
substitute, `InsertMenu.svelte:144`). A big file is read-only (ADR 0028 « ouvert en lecture seule »): the context's
`readOnly` says so (`App.svelte:339`), so the reason is the real one, never a bar's. Host `ui/command/InsertMenu.svelte` (from
`ui/toolbar/`): trigger, name « Insérer un fichier, une référence, une note… », `data-insert`, syntax column (the
format's hints) and steps kept; **declared change:** an item's accessible name is its command's label (today the
button's whole text), its description and syntax move to `aria-describedby`, still shown; its fixed widgets carry `data-fixed` from a closed list (`insert-back`, `insert-file`,
`insert-key`, `insert-keyform`, `insert-note`, `insert-nobib`, `insert-add-bib`). The text surface handles the commands:
an item that needs nothing inserts (one undo step, as today); one that needs a file or a key opens the Insert menu at
its step (`insertMenu.open(item)`), the picker listing the `accepts` types sorted by path, the citation keys read from
the files `accepts.bibliography` lists (`:202`). **Declared change:** the thirteen commands enter the palette (a
location's item is a command, §4.31); run from there, one with a step opens the menu at that step, showing the bar
first if the person hid it (its setting, as `app.toolbar` does). No two palette labels equal in French or English,
case-insensitive (checked: « Lien vers un fichier » ≠ « Lien »). `insert.csvTable.markdown` (the same text as
`insert.csvTable`) is deleted; Markdown's description goes through `describe` (`insert.csvTable.markdown.desc`).

## 3. The other type-id reads (K-7 left: `language/client.ts`, the Typst engine's LSP `languageId`)

- `setup.ts:78` (Typst's writing help): K-7's, given when the language service serves the type
  (`client.ts` exports `LANGUAGE_TYPE`; `App.svelte:396` and the editor read it), so `App.svelte` leaves the list too.
- `problems.svelte.ts:148,159,195` (`source: "csv"`): the source is `viewer` (a viewer's own problems, today only
  the grid's, `Sheet.svelte:119`), which opens in the text view at the line, as today.
- `insert.ts:63` (`READERS`), `InsertMenu.svelte:53,78,93,101,109` (types): the manifests' `readers`, `accepts`.
- `FormatBar.svelte:62`, `ext/app/native/status/Language.svelte:9` (`lang.${…}`): the grammar's `label`, else `lang.plain`.
- `App.svelte:109,158,338,653` (`formatLanguage`, `lang !== "plain"`): `registry.format(opening.type.grammar)`.

## 4. Rules (test titles in the gate; each has a mutant caught by that title only, `scripts/netsim/mutants.toml`)

| Rule | What | Mutant |
|---|---|---|
| F-1 | `format` and a grammar's `label` validate; the test reads every `invalid/` format vector and its reason: a plugin entry, a grammar without label, an `accepts` key that is no Insert item, a `readers` value that is no identifier | MI1 a plugin format entry accepted |
| F-2 | the formatting language is the grammar's entry: none is plain (no bar, « Texte brut »); `formats` and `inserts` from it; built-in entries on the first screen; `registry.format` the same object | MI2 a grammar without `format` gets Typst's |
| F-3 | a language's own behaviour comes from its entry: quote wrapping only with `quote`, cleanup pairs, word stripping | MI3 `setBlock` wraps a quote without `quote` |
| F-4 | Insert menu = `insert` location: groups in order with captions, items hidden by `when` on `inserts` (the file link only without attach), syntax from hints, Markdown's description and note | MI4 the file link shown when `attach` is there |
| F-5 | an item's run: nothing inserts at once; a file lists the `accepts` types by path; a key gives the form and the keys; cite without bibliography gives the step; from the palette, the menu at its step | MI5 the picker ignores `accepts` |
| F-6 | type ids only at K-7: the writing help only for `LANGUAGE_TYPE`, a viewer's problem opens in the text view, Typst's data reader from `readers` | MI6 a viewer's problem opens in its viewer; MI9 the writing help for every type |
| F-7 | cost: entries and the location indexed once at build, no format chunk at start beyond today's | MI7 `registry.format` builds a new object at each call |

End to end, Chromium and WebKit: `EX0c3-E1` the Insert menu for `main.typ` and a Markdown note, every menu item is the
location's resolved item in order or a fixed widget of the closed list, else « stray » (MI8, kind `playwright`: a
hard-coded `menuitem` in the host, must fail it); its trigger and menu keep their names, each item is named by its
label and described by its description and syntax; a CSV has no bar; the palette
lists « Note de bas de page » and opens « Image ou page de PDF » at its picker. `EX0c3-E2` insertions: Typst footnote
and page break, an image picker listing only pictures, a Markdown CSV frozen copy, cite with no bibliography; the
status bar's language and words for both. Replayed (Chromium): EX0c2-E1, EX0c1-E1, EX0c1-E2, EX0b1-E1, EX0b1-E2,
EX0b2-E1, EX0b2-E3, UI-02, UI-03, UI-05, UI-06, UI-11, UI-12, UX-S7-1, FMT-29, N29, UX-L3-4, CSV-EDGE-04, EX0-E1,
UX-L2-1; WebKit: UI-05. None is gated by an environment variable (checked, witnesses). `e2e:webkit` stops with
« container not ready » when its container does not answer in 60 s, instead of running the tests against nothing.

**Step-2 item EX0c1-E2 [EX0c3]** (red in the replay set, 2 of 2; green alone, 8 of 8; it stays in the set). Likely
a read race at `ex0c1.spec.ts:170-172`: the poll ends while `text` is still false during the on-demand load of the
Markdown grammar, and `items()` and `resolved()` are read at different moments. First record the bar's sequence of
items with a `MutationObserver` across the switch to the note, and prove which it is: a short loading state, then the
test polls until both reads agree (a test fix, named in the commit); items that vanish and come back on a switch from
text to text, then it is a flicker of the product, fixed in the status bar, the test unchanged.

## 5. Performance, gate, risks, out of scope

Start: PERF-START paired against the merge-base (ADR 0038), a guard; the format entries replace static imports of the
same size. Gate (≤ 45 min): `docs`, `no-hard-locations` (EX0c2's lint, InsertMenu off its allow list, the type-id
lint with K-7 alone, object keys named after a type caught too), `lint` (`mise run lint`), `test-py` (this block adds
MI8 to `scripts/`), `frontend`, `mechanical`, `mutants`, `e2e-mutant`, `e2e`, `e2e-webkit`, `perf-start`, `simplify-report`. Risks:
the bar before the entry is there (eager, F-2), the test edits hiding a change (`mechanical`),
UI-11's Insert screenshot (names and captions kept). **Out of scope:** `focusShownView()` (the EX0c2 residual: the
session is not touched here → UX1-D); the `main.typ` name literals (`App.svelte:396`, `problems.svelte.ts:142`, a
project rule, not a type id); `insert.empty.*` keys; icons named after types (`ui/icons/paths.ts:81-82`, the
manifests' `icon`); the `main` menu's `insert` group (spec §4.31 row 2, empty → UX1-D); plugin format entries (EXT1).

## Sites (file:line at `main` af69c6f)

- Format language: `editor/format/support.ts:7-27,30-45`, `lang.ts:1`, `typst.ts:147-204`, `markdown.ts:139-184`,
  `cleanup.ts:11-17,36`, `commands.ts:205,361,381,396`, `active.ts:14,35`, `handlers.ts:45-57`, `insert.ts:35-44,63-71,78-237`;
  `editor/stats/words.ts:151-159`, `stats.ts:34-35`; `editor/setup.ts:76-79,95,125`; `extensions/context.ts:32`;
  `extensions/manifest.ts:278-285` (`GrammarContribution`), `:189-192` (`insert`); `extensions/when.ts:47`;
  `ext/typst/inkwell-plugin.json:17-24`, `ext/markdown/inkwell-plugin.json:18-25`.
- Insert menu: `ui/toolbar/InsertMenu.svelte:38-159,202,216,233-237,278,282`; `ui/command/FormatBar.svelte:62,100`;
  `ext/format/inkwell-plugin.json:146` (menus).
- Other reads: `App.svelte:109,158,338,339,396,653`; `ext/app/native/status/Language.svelte:9`;
  `language/problems.svelte.ts:148,159,195`; `ui/sheet/Sheet.svelte:119`; `language/client.ts:191` (K-7, kept).
- Tooling: `mise.toml` (`e2e:webkit`'s wait, review); `scripts/codemods/ex0c3_type_literals.py`.
