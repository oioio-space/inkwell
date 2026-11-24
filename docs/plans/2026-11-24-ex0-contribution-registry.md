# EX0, slice 1: the contribution registry and the file-type point

Rigour: standard (no peer bytes, no trust state, no sandbox boundary). Decision and sources: ADR 0027. Requirements: U26,
N20 (registry part), N21 and N24 (base), W72, W79. Gate: `gates/EX0.toml`, witnesses `gates/EX0/WITNESSES.md`, reviews
`gates/EX0/SPEC-REVIEW.md` and `REVIEW-2.md`. Next slices, each with its own gate: commands, palette, shortcuts (EX0b);
menus, toolbars, panels, status items, and the type-id reads they absorb (EX0c); the grammars of AD-54's closed list,
new pinned packages (EX0d). Out of scope: plugin host, run-time capabilities, the « Extensions » page, a general « suggest » point (the « looks
tabular » offer stays, through the registry, off the start path: code review finding 2), per-file choice in the project index (ADR 0023).

## Steps (each in TDD)

1. `frontend/src/extensions/`: `manifest.ts` (JSON text → strict validator, bounds, no regex, no `__proto__`/
   `constructor`/`prototype` key, null-prototype frozen result, entry paths confined to the folder), `registry.ts`
   (build, conflicts, frozen snapshot, `resolve`, `viewersFor`, `grammarFor`, `codeLanguages()`), `loader.ts`
   (manifests by `import.meta.glob("../ext/*/inkwell-plugin.json", { query: "?raw", import: "default", eager: true })`;
   entries by a lazy glob of `../ext/*/native/*.ts`, one memoized `import()` each), `context.ts` (viewer contexts).
2. Built-ins in `frontend/src/ext/`. Essential, with code: `text` (type, text editor, startup), `files` (`binary`, file
   card), `typst` (type, grammar, startup), `markdown` (type, grammar, lazy), `tables` (`csv`, `tsv`, `psv` with
   `options.delimiter`; grid, lazy), `images` (png, jpeg, gif, webp, svg, bmp, storage `blob`; image viewer, lazy).
   Data only: `bibtex`, `pdf` (blob, file card), `data` (json, yaml, toml, xml as text, no grammar until EX0d).
3. Migrate, then delete: `langOf`/`Lang`, `isTabular`, `dialect.ts`'s extension switch, `FileIcon`'s table, the kind
   lists of `workspace.svelte.ts`, `search/project.ts`, `SearchPanel.svelte`, `InsertMenu.svelte`, `fakeStore.ts`, the
   table test of `DiffView.svelte`, the extension table of `format/insert.ts`, the `switch` of `highlight.ts`, the
   static `markdown()` and `Sheet` imports, and `App.svelte`'s `if` chain (the `looksTabular` offer becomes `offersTable`, from the registry). Type-id reads stay
   only at the sites the `no-lists` criterion lists (format bar, Insert, word count, Typst status and presentation:
   EX0c; the LSP language id: K-7).
4. Go and the bridge: `project.File` loses `Kind`; `extKinds`, `kindOf`, `looksLikeText` go; `ReadImage`'s name check
   moves before the read into `internal/app/image.go` (its only caller, which holds `imageTypes`). Version byte, one
   constant per payload: `fileListVersion = 2` for `EncodeFileList` (`files.go`) and `encodeFileList`/`decodeFileList`
   (`store/files.ts`); `fileInfoVersion = 1` for `EncodeFileInfo`/`DecodeFileInfo` (`files.go:88`) and
   `encodeFileInfo`/`decodeFileInfo`; `treeVersion = 1` for `DecodeTree`, `EncodeTreeResult`, `DecodeReferenced`,
   `EncodeBlobInfo` (`tree.go:67,123,129,147`) and `store/tree.ts` (`VERSION`). Vectors in
   `internal/bridge/testdata/storestream.json`: « file list payload » and the `ListFiles` response change, the others
   stay byte for byte (both sides read the same file).
5. `scripts/netsim/mutants.py`: a `vitest` entry may carry `name`, the exact title of its test, and is then caught only
   when that test fails (done at review round 2, tested in `mutants_test.py`; entries without `name`, block SI1, keep
   the file-wide rule). EX0's entries MX1..MX14 are written with the code.
6. Docs: spec MF-14 and §4.4 amended (Go embeds nothing; `src/filetypes` merged into `src/extensions`; FE-3 carve-out),
   AD-51 superseded, `docs/exigences.md` (U26, N20, W79), `PROGRESS.md` (EX0b, EX0c, EX0d).

## Rules and tests (exact names; each Vitest rule has its mutant, caught by that title only)

| Rule | What | Test | Mutant |
|---|---|---|---|
| R-1 | built from data alone, no entry imported, snapshot frozen, built-ins valid | `EX0 R-1 the registry is built from manifests without running code` | MX1 build imports an entry |
| R-2 | malformed manifests refused with their reason; ≥ 1 vector in `testdata/manifests/valid/` and `invalid/` | `EX0 R-2 a malformed manifest is refused with its reason` | MX10 unknown keys accepted |
| R-3 | a taken extension, whole name or id refused, both named, either order | `EX0 R-3 a claim already taken is refused in either order` | MX3 the later claim overwrites |
| R-4 | name order (choice, whole name, longest suffix, shebang) and content-class outcomes of ADR 0027 rule 6 | `EX0 R-4 resolution follows the order of ADR 0027` | MX4 suffix before whole name |
| R-5 | origin stamped by the loader; `ui.native` only built-in and essential | `EX0 R-5 ui.native is refused to any origin but built-in` | MX5 origin read from the manifest |
| R-6 | entry loaded once for concurrent opens; failed load falls back with a notice | `EX0 R-6 a viewer loads on first open, once, and falls back on failure` | MX7 loader not memoized |
| R-7 | data-only extensions disabled: their types vanish, their files open by content | `EX0 R-7 the essential extensions alone open every file` | MX9 disabled types kept |
| R-8 | one folder adds type, icon, viewer, grammar; Markdown fences use the registry | `EX0 R-8 one folder adds a type and its grammar` | MX11 `codeLanguages` ignores the registry |
| R-9 | file list v2 without kind; other payloads unchanged; listing opens no file | `EX0 R-9 the file list v2 decodes the shared vectors`; Go `TestEX0FileListV2`, `TestEX0ListReadsNoContent` | MX12 decoder accepts v1; MX13 (go) encoder writes v1 |
| R-10 | no lazy source in the start chunks | criterion `chunks` (sourcemaps) | — |
| R-11 | opening budgets | `EX0-E1 entries load on demand: main.typ loads none, a CSV its grid, a Markdown file its grammar`, `EX0-E2 PERF-EXT-VIEWER-CSV opens a 1 MiB table within budget` | — |
| R-12 | prototype keys refused at any depth; null-prototype result | `EX0 R-12 a manifest cannot reach the object prototype` | MX2 `__proto__` accepted |
| R-13 | entries confined to their folder (`..`, `/`, `\`, `C:`, scheme, empty segment refused) | `EX0 R-13 an entry cannot leave its extension's folder` | MX6 `..` accepted |
| R-14 | a type without a viewer opens in the essential viewer of its storage | `EX0 R-14 a type without a viewer opens in the essential viewer of its storage` | MX8 no fallback |
| R-15 | CSV offers grid and text (W79) from the manifests, no viewer code loaded to offer them | `EX0 R-15 a table offers its grid and its text without running viewer code` | MX14 text alternative dropped |

## Performance budget

- Start, unchanged or better: **PERF-START** (`scripts/perf/baseline.py`, paired against the merge-base, ADR 0038). The
  start keeps its eager editor and Typst grammar and loses Markdown, the grid and the image viewer (R-10).
- Registry ≤ 10 ms (§15.4): by counts (no entry imported, one pass over the manifests), not by a clock.
- `PERF-EXT-VIEWER-CSV` (§15.4 viewers row), `frontend/e2e/ex0.spec.ts`: activation and first render only, in an app
  already running. Before each run the page is reloaded and `main.typ` is editable (the grid's chunk is not loaded);
  the test clicks `table-1mib.csv` (1 048 576 bytes, 10 columns, fixed seed) in the file panel; the app marks
  `ex0:open` when the panel's handler starts and `ex0:first-render` after the frame in which the grid's first visible
  rows are in the DOM; the test reads the measure by `performance.getEntriesByName`. One warm-up, then 10 runs; p95 is
  the nearest-rank ⌈0.95 × 10⌉ = 10th of the sorted values: the slowest of 10 must be ≤ 200 ms. Whole run under
  `heavy.sh --exclusive`.

## Gate criteria (`gates/EX0.toml`, max_minutes 45)

`docs`, `no-lists` (type ids and extensions outside the manifests, except listed sites), `frontend` (Biome,
svelte-check, Vitest; the 13 Vitest rule names), `go`, `chunks` (R-10 on sourcemaps), `mutants` (MX1..MX14, each with
`name`, all caught), `e2e`, `perf-start`, `simplify-report`.

## Risks

Opening a file breaks: FM-OPEN and two CSV scenarios in the gate, WebKit in CI on push. A lazy module pulled into the
start by a static import: `chunks` reads sources. Wire skew: per-payload version bytes refuse a mix.
