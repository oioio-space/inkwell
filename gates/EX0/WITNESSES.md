# EX0 - red witnesses

Recorded 2026-11-24 at step 1, before any production change. Round 3 ran on `main` at 60ea6eb (PF1; the worktree was
fast-forwarded after its work was archived), plus the ADR, the plan, this draft gate, the round-2 change of
`scripts/netsim/mutants.py` (and its tests) and the integrator's row W79 of `docs/exigences.md`. Each command was run
as written in `gates/EX0.toml`, from the worktree root, through `sh -c` as `scripts/gate.py` does (`/bin/sh` is bash
here). Logs are in the scratchpad, `witness-EX0-<id>.log`. Every heavy criterion started only once no push CI
(`pre-push`, `mise run ci`) was left on the machine (checked with `pgrep`), and the load average is noted.

Rounds: 1 is the first draft; 2 follows the cross-family review (`SPEC-REVIEW.md`); 3 follows the blind Opus review
(`REVIEW-2.md`). The table gives each criterion's latest run. Every criterion whose command changed in a round was run
again in that round. `go`, `e2e` and `simplify-report` are unchanged since round 2, apart from comments.

| Criterion | Round | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| docs | 3 | 1 | < 1 s | MF-14 still embeds the table in Go, §4.4 still has the `src/filetypes` row, AD-51 not superseded, U26 and N20 do not point at the plan; ADR sections and the 15 plan rows found | yes (plan step 6) |
| no-lists | 3 | 1 | < 1 s | 94 sites: the old lists (`InsertMenu.svelte` 19, `FileIcon.svelte` 12, `workspace.svelte.ts` 10, `lang.ts` 9, `search/project.ts` 9, `App.svelte` 6, `dialect.ts` 5, `store/files.ts` 5, `setup.ts` 4, …), the type ids caught by the new rule (`highlight.ts` switch, `lang.ts` cases, `FileIcon.svelte` values), name splits (`format/insert.ts:62`, `lang.ts:7`), Go (`internal/project/files.go`, `internal/bridge/files.go`) | yes. The rule was checked on the review's three examples (`["csv","tsv"].includes`, `split(".").pop() === "csv"`, `endsWith("csv")`): each is matched; `'tab'` and `type="text"` are not |
| frontend | 3 | 1 | 308 s | Biome clean, svelte-check `0 ERRORS 0 WARNINGS` (775 files), Vitest 1416 passed, 0 failed, 1 todo; the 13 rule names R-1..R-9, R-12..R-15 `absent` | yes (308 s of 480 s, load 0.80) |
| go | 2 | 1 | 99 s | `go test` of `./internal/project` and `./internal/bridge` exit 0; `TestEX0FileListV2`, `TestEX0ListReadsNoContent` `absent` | yes (83 s at rest in round 1; cap 150 s) |
| chunks | 3 | 1 | 2 s | 19 problems: the App chunk holds `src/ui/sheet/` (viewMode, sessions, ContextMenu, FilterMenu, FindBar, keys…), `@lezer/markdown` and `@codemirror/lang-markdown`; no built-in manifest; types csv, markdown, png not contributed | yes: the start chunk really holds what ADR 0027 rule 9 says must leave it. The 670-byte chunk without a sourcemap is Vite's modulepreload polyfill, under the 1 KiB bound |
| mutants | 3 | 1 | < 1 s | MX1..MX14 missing, no vector in `testdata/manifests/valid/` nor `invalid/`; the runner is not reached | yes. `mutants.py --check`: 0 problems with this gate naming block EX0 |
| e2e | 2 | 1 | 209 s | FM-OPEN-1, CSV-DIALECT, CSV-14 pass first time in Chromium; `EX0-E1`, `EX0-E2` not passing (no `ex0.spec.ts`) | yes (209 s with the wait for the exclusive slot, 27 s in round 1; cap 200 + 240 s) |
| perf-start | 3 | 0 | 126 s | PERF-START `pass`: paired against the merge-base 60ea6eb, A/A identity true, ratio upper bound 1.0036 < 1.10, budget `pass` | passes by design: a regression guard of an unchanged tree. It ran once PF1 landed (round 2's red was PF1's defect); cap 600 s |
| simplify-report | 1 | 1 | < 1 s | `gates/EX0/SIMPLIFY.md` absent | yes |

Red for the right reason: 8 of 9; `perf-start` is green by design.

## Caps measured (rule G-1, round 2 finding 9)

- `mutants`: no EX0 entry exists before step 2, so the runner was measured on the 14 `vitest` mutants of block SI1,
  same runner and same copy of the tree: 22 s in all (`netsim mutants: all 14 caught`, load 0.70). EX0 has 13 vitest
  entries and one Go entry (`go test` of `internal/bridge`, about 40 s), so about 80 s is expected; cap 300 s.
- `perf-start`: 138 s, then 126 s (including the merge-base build and 164 starts); cap 600 s.
- The caps add up to 2230 s, within `max_minutes = 45` (2700 s); `deadline_problem` returns `None`.

## Checked to work with the real tools

- `chunks`: Vite 8.3.5 with `--manifest --sourcemap` writes one `.map` per JS chunk, whose `sources` are paths under
  `frontend/node_modules/.pnpm/<pkg>@<version>/node_modules/<pkg>/…` and `frontend/src/…`, so the forbidden list matches
  them. A lazily imported module is keyed by its source path with `isDynamicEntry: true` (`src/App.svelte`).
- `mutants`: `run_vitest` with `name` is covered by six unit tests (`VitestNameTest` in `scripts/netsim/mutants_test.py`,
  29 of 29 pass): only the named title failing is a catch, another failing test or a longer title is not, and an entry
  without `name` keeps the file-wide rule (block SI1, frozen gate `SI1b`, unchanged: its 14 mutants still all caught).

## What each named test must assert (finding 6; checked at the code review and by the mutants)

A rule test that only runs code passes nothing here: each must make these assertions, and the mutant in brackets must
turn exactly this test red, by its title (criterion `mutants`, block EX0, field `name`; entries written at step 2 with the code).

- **R-1** [MX1]: building the registry from the built-in manifests calls a counting fake loader 0 times; every object
  and array of the snapshot is frozen (`Object.isFrozen`, walked to the leaves); every built-in manifest validates.
- **R-2** [MX10]: each vector of `frontend/src/extensions/testdata/manifests/invalid/` (at least one; the gate checks)
  is refused with the reason its file name states (unknown key, list or string over its bound, bad id, a
  pattern-like field), and each one under `valid/` (at least one) is accepted unchanged; MX10 accepts unknown keys.
- **R-3** [MX3]: two manifests that claim `.csv`, then a whole name, then an id: building (A, B) refuses B and names A
  and B; building (B, A) refuses A and names both; the rest of the registry is the same in both orders.
- **R-4** [MX4]: a table of at least 12 rows (name, first line, content class Go reports, person's choice → type
  and viewer) covers each step and its ties: `.tar.gz` before `.gz`, `Makefile` before an extension,
  `#!/usr/bin/env bash`, a `.csv` whose content is binary → the file card with its notice, unknown text → `text`,
  unknown binary → `binary`, `.svg` → the image viewer with no content class asked (blob storage).
- **R-5** [MX5]: `ui.native` is refused for origin `plugin`, for origin `project` and for a non-essential built-in; a
  manifest with an `origin` key is refused (unknown key); a `native/` folder under a manifest without `ui.native` fails
  the build of the registry.
- **R-6** [MX7]: with a deferred fake loader, two concurrent opens call it once and get the same module; a later open [EX0]
  calls it 0 more times; a rejected load returns the essential viewer of the content class and shows exactly one notice.
- **R-7** [MX9]: with `bibtex` and `pdf` disabled, the snapshot has no `bibtex` or `pdf` type, `refs.bib` (text)
  resolves to `text` and opens in the editor, and `a.pdf` (binary) opens as the file card.
- **R-8** [MX11]: a fixture extension (one folder: manifest and one native entry) adds a type, its icon, a viewer and
  a grammar, which the registry lists. Markdown's `codeLanguages` contains the grammar's names. A fenced block with
  that info string gets the grammar's nodes in its syntax tree. MX11 builds `codeLanguages` from a fixed list.
- **R-9** [MX12, MX13]: Vitest decodes every file-list v2 vector of `internal/bridge/testdata/storestream.json`
  (path, size, flags, no kind) and refuses a v1 file-list payload, and the file-info and tree vectors still decode at
  version 1. `TestEX0FileListV2` encodes the same entries to the same bytes. `TestEX0ListReadsNoContent` lists a folder
  holding an unknown-extension file whose mode is 000 and gets it with its size, and the test's counter of file opens
  through `os.Root` stays 0. MX12: the decoder accepts version 1; MX13: the Go encoder writes version 1.
- **R-12** [MX2]: `__proto__`, `constructor` and `prototype` are refused at the top level, in `contributes`, in a file
  type's `options` and inside arrays. After each attempt `({}).polluted` is `undefined`, and in an accepted manifest
  every object has a `null` prototype.
- **R-13** [MX6]: the entries `../x.ts`, `native/../../x.ts`, `/x.ts`, `native\x.ts`, `C:x.ts`, `file:x.ts`, `native//x.ts`
  and the empty entry are refused. `native/grid.ts` is accepted only when it is a glob key of the same folder, and an
  entry of extension A that names a file of extension B is refused.
- **R-14** [MX8]: `bibtex` (no viewer of its own) defaults to the text editor, and a blob type without a viewer defaults
  to the file card. With the fallback removed (MX8), resolution returns no viewer and the test fails.
- **R-15** [MX14]: for `csv` the registry offers the grid (default) and the text editor (alternative) with a
  counting loader at 0 calls. For `typst` (default: the text editor) it offers no alternative. MX14 drops the text
  alternative.
- **EX0-E1**: the requests for `/assets/*.js` show none of the lazy entries' chunks (Vite build manifest) after
  `main.typ` is editable; the grid's chunk appears only after a CSV is opened, and the Markdown grammar's only after
  a `.md` file is opened.
- **EX0-E2**: the scenario of the plan; the test prints the 10 values and fails if the slowest is above 200 ms.
