# Syntax highlighting on large files: root cause (Typst) and per-language check

Date: 2026-11-10. Follows `2026-11-09-large-files.md` §4.2 (LF) and `2026-11-09-ide-large-file-practices.md`.

**Versions:** `codemirror-lang-typst` 0.6.0 (`/lezer` entry), `@codemirror/lang-markdown` 6.5.2, `@lezer/markdown`
1.8.1, `@codemirror/language` 6.13.1, `@codemirror/state` 6.7.6, `@lezer/common` 1.5.4, `@lezer/highlight` 1.2.5.

**Method:** Node 24.22, headless, under `heavy.sh`. Scripts, outside the repo, in `$SCRATCH/typst-lezer/`:
`patch.mjs`, `incremental.js`, `equiv.mjs`, `targeted.mjs`, `bench.mjs`, `langbench.mjs`, `mdprof.mjs`.

**Headline figures:** one keystroke through `EditorState.update` (synchronous, main thread), p50, edit at
10 / 50 / 90 % of the file. "Reused" is the share of characters whose subtrees are the same objects in the
old and the new tree (mounted trees included). Node is about 3× slower than LF's Chromium (128 ms vs 49 ms
at 2 MB), so read ratios, not absolutes.

## 1. What is highlighted (wiring in `frontend/src/editor/highlight.ts`)

| Language | Parser | Nested languages |
|---|---|---|
| Typst | `typst_lezer().language`, no `codeLanguages` | none: raw blocks only get `monospace` |
| Markdown | `markdown({ base: markdownLanguage })` (GFM), no `codeLanguages` | fences: none. **HTML** blocks and inline tags always go through `@lezer/html` (`parseCode({ htmlParser })`, `lang-markdown` l. 422), and HTML nests CSS and JS |

`plain`, CSV (a grid view) and the problem marks are not syntax highlighting.

## 2. Typst: the cause

`typst_lezer` is not an LR grammar: there are no generated tables, no external tokenizer and no
`contextTracker`. It is a hand-written recursive-descent CST parser (`TypstLexer`, `TypstCSTParser`; the
`@lezer/markdown` approach). It emits one buffer through `Tree.build`, which balances it correctly (2 038 top
children at 2 MB). The fault is in `TypstPartialParse.advance()`, `dist/lezer.js` l. 4504–4557:

- **The fragments are ignored.** The previous tree is reused only when nothing changed
  (`fragment.from === 0 && fragment.to >= end && tree.length === length`). Any edit re-reads
  `input.read(0, length)` and reparses everything.
- **`advance()` is one shot.** There is no time budget: the 20 ms `Work.Apply` and the worker slices never
  apply.
- **Why the cost grows with distance.** On an incomplete tree, CodeMirror calls `stopAt(viewport.to)`, so the
  parse runs from 0 to the viewport. On a complete tree, the whole file is reparsed inside `state.update`.

Measured: **0.00 % of characters reused** after any keystroke (2 and 10 MB, all positions). The other suspects
were ruled out: no context hash and no lookahead are involved, the top rule does split, and the package does
not rebuild the tree on change.

## 3. Typst: the proof patch (scratch copy of `dist/lezer.js`, ≈ 240 lines)

**`TypstCSTParser` can start at any offset.** A `parseTop(hook)` loop is the document loop of
`parseMarkupExpressions(["End"], true)`, with a hook before each top-level expression that starts at a line
start with no open bracket. That is the parser's initial state, so a parse can **restart** there.

**`TypstPartialParse` is incremental:**
- **Chunks.** The tree is a list of anonymous chunks (≥ 4 Ki chars) cut at restart points.
- **Before the edit.** Chunks are reused from the `from = 0` fragment, up to the start of the last chunk that
  ends ≥ 64 chars before the change.
- **After the edit.** The parse resynchronises at the first restart point that maps to an old chunk start in
  the unchanged tail fragment, with its line start inside the fragment. The rest of the old tree is reused.
- **Text window.** The parse reads a window (64 Ki), never the whole file: `sliceString` alone costs 18.7 ms
  at 50 MB. A token lexed within 64 chars of the window end invalidates the window. It reopens at the last
  emitted chunk, or reads the rest of the file at once when no chunk could be emitted.
- **Time budget.** `advance()` returns every 128 Ki chars. Multi-range (mixed-language) parses keep the
  original path.

## 4. Typst: measurements, original → patched

| Size | original (complete tree: full reparse) | patched | patched: reused / reparsed |
|---|---|---|---|
| 2 MB | 106 / 109 / 111 ms | **1.6 / 1.8 / 0.9 ms** | 99.59 % / 8.3 Ki chars |
| 10 MB | 554 / 528 / 513 ms | **1.9 / 1.3 / 1.2 ms** | 99.92 % / 8.3 Ki |
| 50 MB | 3 438 ms (at 50 %) | **0.9 / 0.8 / 0.6 ms** | 99.98 % / 8.3 Ki |

**Viewport case (LF's browser case).** The parse stops at the edit + 3 000 chars. Original: 15 / 71 / 117 ms
at 2 MB, 68 / 345 / 610 ms at 10 MB and 375 / 1 783 / 3 059 ms at 50 MB. That is linear in the position:
LF's symptom, reproduced.

**First parse at 50 MB.** Original: 3 562 ms in one `advance()` call, a 3.5 s long task. Patched: 2 890 to
3 297 ms over 379 calls, the longest 24 to 30 ms.

**Parsing from a JS string.** Fed a JS string, the patched parse costs 18 ms at 50 MB, because V8 flattens a
concatenated string on first access. CodeMirror's rope (`Text`) avoids this.

## 5. Typst: same output

**Corpus.** 227 real files: the 223 files of `typst/typst` `tests/suite` (main, 1.5 MB, error cases
included), the repo's `testdata/perf/lsp/*.typ`, and all of them concatenated.

**Check.** For each file: a fresh parse, then 20 random Typst-significant edits (`[ ] $ * _` and backticks,
`#let`, `= `, list markers, comments, quotes, deletions). The edits are chained directly
(`parse(text, applyChanges(…))`) and through `EditorState` (partial parses, `stopAt`, pauses). After each
step, the per-node tags (`getStyleTags` on the node) and the named-node structure are compared with a full
parse by the original.

**Final patch.** Seeds 1, 5, 6, 7 and 8 at chunk/window/slice 64/256/512, and seed 9 at
4 Ki/64 Ki/128 Ki: 54 720 edits, **0 differences**.

**A bug the test caught.** Seed 5 broke a first version: a string that runs past the window was truncated,
and error recovery carried the damage to a later point that looked valid. The guard now covers every token
lexed since the window opened. Without that guard, seed 5 fails twice, so the mutant is killed.

**Targeted test.** An indentation edit before a resync point kills M2 (resync without the line-start check).
The "one chunk back" rule has no counterexample at top level, because the 64-char margin already covers it.
It stays as a guard: its cost is ≤ 1 chunk.

**Found on the way: a Lezer bug, to report upstream.**
- **Symptom.** `highlightTree` span lists differ in 0 to 28 runs per seed, for both parsers. In the
  original, `#foo` gets the `FuncCall/Ident` colour without being a call.
- **Cause.** `TreeCursor.sibling()` does not reset `_tree` when it moves from a `Tree` child into a
  `TreeBuffer`, so `matchContext` reads the previous sibling as the parent.
- **Where.** `@lezer/common` 1.5.4 and `main` (`src/tree.ts`, `matchContext`). The wrong colour depends on
  the buffer layout.

## 6. Typst: the remaining limit

Incrementality needs top-level restart points. An **unclosed delimiter** (`$`, `#[`, a backtick fence, `/*`)
puts the rest of the file inside one element: the suite corpus has no restart point after its 521 Ki-th char
(`math-unclosed`). Every edit then reparses the tail.

Measured, `suite` 10 MB, at 50 %: original 379 ms, patched 413 ms. That is the same cost. It lasts until the
person closes the delimiter. The fix is second-level restart points, inside `Markup` and `Math`, keyed by the
same state. Until then, LF's 512 Ki cap and time-budget kill switch stay for this case.

## 7. Markdown (GFM) and its nested languages

| File | 2 MB | 10 MB | 50 MB | reused |
|---|---|---|---|---|
| Markdown (repo `docs/**/*.md`, repeated) | 3.9 / 3.0 / 3.1 ms | 4.9 / 16.2 / 18.7 ms | **21.5 / 58.6 / 97.0 ms** | 99.65–99.94 % |
| Markdown + one HTML block in 8 (`<style>`, `<script>`, inline tags) | 6.8 / 3.9 / 4.3 ms | 3.1 / 7.9 / 9.6 ms (quiet machine) | **13.0 / 42.5 / 60.8 ms** | 99.51–99.94 % |

- **Reuse works.** No Markdown language loses fragment reuse. A keystroke outside HTML creates no new nested
  tree: 0 mounted trees new after edits.
- **But the cost grows with position.** It passes one 16 ms frame from about 10 MB at 90 %, and reaches
  ≈ 100 ms at 50 MB. The parse is O(nodes), not O(edit).
- **Cause, measured on the parser alone at 10 MB, edit at 50 / 90 %:**
  - bare `@lezer/markdown`: 2.6 / 3.5 ms. It re-walks the fragment from 0 (`takeNodes`, `balanceRange`).
  - the app's parser, wrapped by `parseCode` (`parseMixed` for HTML): 5.3 / 6.0 ms.
    `MixedParse.startInner` and `nextChild` walk the whole outer tree after each parse.
  - The remaining gap up to the `EditorState` figure is not attributed; profile it in the browser.
  - One 1.2 s "rest of the parse" outlier in the HTML case did not reproduce on a quiet machine
    (load noise).
- **Root-fix routes:**
  - (i) Upstream (`@lezer/common` `parseMixed`, `@lezer/markdown` `takeNodes`): skip subtrees reused
    from fragments instead of walking them. The 1.5.2 changelog already touched this ("fail to reuse
    unchanged inner parses").
  - (ii) If HTML inside Markdown need not be highlighted: `new LanguageSupport(markdownLanguage)` without
    `parseCode`. That halves the cost but does not fix the walk.
- **Nested code: none.** Neither fences nor Typst raw blocks have a nested parser wired. Adding
  `codeLanguages` later must come with its own PERF case.

## 8. PERF cases for gate LF1 (one per highlighted language)

Run them in LF's Chromium harness, on files of 2, 10 and 50 MB. Each case types at 10, 50 and 90 %. Each
asserts: keystroke p95 ≤ 16 ms, no long task, ≥ 99 % of characters reused, first parse without any
slice > 50 ms.

| Case | Content | Today (Node) |
|---|---|---|
| PERF-HL-TYPST | well-formed Typst (`testdata/perf/lsp` repeated) | fails (0 % reused) → passes with the patch |
| PERF-HL-TYPST-OPEN | Typst with an unclosed `$` early on | asserts the cap or kill switch engages, cost ≤ original |
| PERF-HL-MD | GFM Markdown | passes at 2 MB, fails from 10 MB at 90 % |
| PERF-HL-MD-HTML | Markdown with HTML blocks (nested HTML, CSS, JS) | as PERF-HL-MD; plus the edit inside a `<script>` |
| PERF-HL-NESTED | fences and raw blocks | asserts no nested parser is mounted (guard for future `codeLanguages`) |

## 9. Typst options

Figures per keystroke at 2 / 10 / 50 MB; "est." means not measured here.

| Option | Per keystroke | Cost | Risk |
|---|---|---|---|
| **This patch** (upstream PR, pinned local patch meanwhile) | 1.6 / 1.9 / 0.9 ms (measured) | S, ≈ 240 lines; harness exists | low: same parser, same output proven; one maintainer |
| (a) Our own Lezer LR grammar | ~1–3 ms est. | L: three modes, newline/column rules, contextual `else`/`.` need external tokenizers and a context tracker; the package author rejected it | high: fidelity to `typst-syntax`, a grammar to update per Typst release |
| (b) Go PEG (Pigeon) over the Wails bridge | ≥ today est.: full reparse (no incremental mode, O(n) packrat memo) + O(n) tokens over the Stream | M–L | high: latency, memory, highlighting leaves the webview. Incremental packrat exists (Dubroy & Warth, SLE 2017; Ohm.js), not in Pigeon |
| (c) tree-sitter-typst, web-tree-sitter in a Worker | ~1–5 ms est. + round trip (stale colours meanwhile) | M–L | medium: grammar lags Typst (`uben0` 2025-04, `SeniorMars` fork 2026-07); WASM size |
| (d) `typst-syntax` WASM in a Worker | ~1–5 ms est. + round trip | M–L | medium: the package's `typst()` already ships it (359 KB, `edit_many` + `highlight_range`), but on the main thread, decorations rebuilt over the whole buffer |

**The CSP is not the obstacle that `highlight.ts` claims.** `frontend/index.html` has
`script-src 'self' 'wasm-unsafe-eval'` (Automerge; ADR 0028). The comment "our CSP forbids WASM" is wrong.

## 10. Recommendation

**Typst: fix the package at its root.**
1. Open a PR to `kxxt/codemirror-lang-typst`. It is Apache-2.0 and maintained (0.6.0 on 2026-08-09, issue #5
   fixed). The PR carries the incremental `TypstPartialParse` in `src/lezer.ts` and the equivalence harness
   as a test.
2. Until it lands, apply a `pnpm patch` (`patchedDependencies`) pinned to 0.6.0. It fails to apply on any
   version bump.

Why: it removes the cause, it is the simplest option and the only one measured end to end, and it keeps the
parser, tags, folding and linter. (a) and (b) cost a new grammar and its fidelity risk for no gain; (b) also
puts the bridge on every keystroke. (c) and (d) remain the route if exact `typst-syntax` fidelity is ever
needed; prefer (d), the official parser.

**Markdown:** route (i), upstream issues with these profiles, while PERF-HL-MD caps highlighting above the
size where it fails.

**Follow-ups:** second-level restart points (§6); report the Lezer `TreeCursor` bug with its repro (dumped
file plus `dbg.mjs`); correct the CSP comment in `highlight.ts`.
