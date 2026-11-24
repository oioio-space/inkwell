# EX0 — /simplify review of the slice-1 diff

Reviewer: Claude Opus 5.5 (implementing session, 2026-11-24). This is a self-review. The review that
counts for merging (Opus at the integrator, or Codex) is still owed.

Scope: the whole diff of block EX0 slice 1 against `bae2aa5`. That is the registry (`frontend/src/extensions/`), the
nine built-ins (`frontend/src/ext/`), the 15 consumers migrated, the Go file list v2 (`internal/project`,
`internal/bridge`, `internal/app/image.go`), the mutants MX1–MX14 and `frontend/e2e/ex0.spec.ts`. The checklist
covers reuse, simplification, efficiency and altitude, within the invariants, ADR 0027 and the frozen gate.

## Applied

- **Claims checked in one pass, with no side effect.** `buildRegistry` used to detect a claim made twice in one
  manifest through `seen.add` inside a `find` predicate. It now reads `claims.indexOf(claim) !== i`, with no
  mutable set.
- **Startup entries load once, from the page's start.** `activateStartup` used to load the startup viewers and
  grammars itself. Now `preloadStartup` (in `extensions/loader.ts`, called by `main.ts`) starts those imports
  next to the App chunk, and `activateStartup` only builds the startup grammars' languages. This removes a second
  load path. It also keeps Vite's preload helper in the entry closure. Before, the helper was split into a
  1519-byte chunk with no sourcemap, which criterion `chunks` refused.
- **The read-only editor lost an unused option.** `createReadOnlyEditor` no longer takes `lang`; only the
  grammar's `support` mattered. It gives `setSupport` back, so a grammar that loads late colours the text in place.
- **One name per entry chunk.** The two grammar entries `grammar.ts` became `typst.ts` and `markdown.ts`, so
  each lazy chunk is named after its entry. EX0-E1 reads the chunks from the page's requests by those names.
- **The `.md` wording suffix became the language itself.** `InsertMenu` built `.md` by hand. The keys are now
  `insert.csvTable.markdown*` and the suffix is `` `.${lang}` ``, so there is no extension literal in a consumer.
- **The `Lang` type lives where the formatting language is decided.** It moved to `editor/format/support.ts`
  next to `formatLanguage`. `lang.ts` re-exports it, so the ten importers did not change.
- **Defect found by FM-OPEN-1 and fixed at its root.** `workspace.#readText` returned the string `"binary"` as
  its sentinel. `typeof read === "string"` therefore took binary bytes for text: `l2.bin` opened in the editor
  instead of the file card. The text now comes back as `{ text }`, so the sentinel can no longer collide with a
  text. A new Vitest test in `workspace.test.ts`, « how a file opens (ADR 0027 rule 6) », covers it: unknown bytes
  open as the card, a CSV whose bytes are not text gets its notice, and an image is never read as text.
- **A manifest vector that Biome cannot parse moved inline.** The trailing-comma "not JSON" case is now an
  inline assertion of R-2 instead of a `.json` vector.

## Set aside

- **`ViewerContext` (the non-native facade) is declared but not used yet.** ADR 0027, « Never exposed », asks
  for the type now, so that a built-in cannot widen it unnoticed. Its first consumer is the plugin host (EXT1).
- **`viewChoice.openAsTable` names the viewer id `tables.grid`.** Only the test hooks use it: the e2e
  `__inkwellTestTable` and the native close walk. The person's own way in is the later « suggest » point, so a
  generic lookup would be speculative.
- **`App.svelte` hides the status bar and the preview for « a text shown by another viewer than the editor ».**
  That rule (`ownView`) stands in for the `status` point of EX0c, which absorbs it.
- **The remaining type-id reads are listed for EX0c or K-7.** They are the format bar, Insert, word count, Typst
  status and the LSP id. Criterion `no-lists` lists each of these sites. Moving them belongs to the next slice.
- **Go's `openHook` is a package variable checked against nil on every content open.** One nil check per file
  read is negligible. An injected opener would thread a parameter through `readLimited`, `tree.go` and
  `snapshot.go` for a test-only need.
- **mutation \<frontend\>**: no `protocol/*` package is touched, so gremlins does not apply (definition of done,
  step 5). Each rule test is instead guarded by its own mutant, MX1–MX14, and all 14 are caught.

## Verified

- `no-lists`: 0 sites. `docs`: pass. `chunks`: pass. The start closure holds no source of
  `@codemirror/lang-markdown`, `@lezer/markdown`, `src/ui/sheet/` or a lazy built-in's `native/` folder, and every
  lazy entry is a dynamic chunk of its own.
- `frontend`: Biome clean; svelte-check 0 errors and 0 warnings (798 files); Vitest 1430 passed, 0 failed, 1 todo;
  the 13 named rule tests passed.
- `go`: `internal/project` and `internal/bridge` pass, including `TestEX0FileListV2` and `TestEX0ListReadsNoContent`.
  `TestProjectTextReportsBinaryContent` (415 for bytes that are not text) passes in `internal/app`.
- `mutants`: `netsim mutants: all 14 caught`, each one by the exact title of its rule's test (MX13 by
  `TestEX0FileListV2` with « R-9 »). `python3 scripts/netsim/mutants.py --check`: 0 problems.
- Invariants. Go carries no type table and no kind on the wire. The CRDT is untouched: the file list was never
  persisted or sent to a peer (ADR 0027, « The file list Go sends is local »). The CSP is unchanged: no `eval` and
  no `new Function`, because the validator is hand-written. Manifests are read as `?raw` text through the one
  parser. Every parsed value is a frozen null-prototype object, and prototype keys are refused at any depth.

## Code review round (blind Opus, 2026-11-24)

The 13 findings of gates/EX0/CODE-REVIEW.md were fixed at their root, each with a test seen failing first (details there); no reuse or simplification finding was rejected.
