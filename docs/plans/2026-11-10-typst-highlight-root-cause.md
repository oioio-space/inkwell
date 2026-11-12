# Syntax highlighting on large files: root causes and fixes (blocks LF0a2, LF0b)

Date: 2026-11-10. Follows `2026-11-09-large-files.md` §4.2 (LF). Gate: `gates/LF0a2.toml`.

**Versions:** `codemirror-lang-typst` 0.6.0, `@lezer/markdown` 1.8.1, `@lezer/common` 1.5.4, `@codemirror/language`
6.13.1, `@codemirror/lang-markdown` 6.5.2. The three packages are fixed by pinned patches in `frontend/patches/`
(`patchedDependencies` in `frontend/pnpm-workspace.yaml`).

**Tests:** `frontend/src/editor/highlight.equivalence.test.ts` and `highlight.perf.test.ts`.

## 1. What is highlighted (`frontend/src/editor/highlight.ts`)

| Language | Parser | Nested |
|---|---|---|
| Typst | `typst_lezer().language` | none (raw blocks only get `monospace`) |
| Markdown (GFM) | `markdown({ base: markdownLanguage })` | HTML blocks and tags through `@lezer/html` (always wired by `lang-markdown`); fences: none |

## 2. Typst: cause

`typst_lezer` is a hand-written recursive-descent parser (`TypstCSTParser`), not an LR grammar. Its
`TypstPartialParse.advance()` (dist l. 4504–4557) had three faults:
- **It ignored the fragments.** The old tree was reused only when nothing had changed. Measured: 0.00 % of
  characters reused after a keystroke.
- **It parsed everything in one call.** CodeMirror's time budget never applied.
- **The cost grew with position.** The parse restarted from 0 up to the viewport (LF §4.2).

## 3. Typst: fix (`patches/codemirror-lang-typst@0.6.0.patch`)

1. **Chunks.** The tree is a list of anonymous chunks (≥ 4 Ki chars), cut at top-level restart points: a line
   start with no open bracket, which is the parser's initial state.
   - Chunks before the edit are reused, up to the last chunk ending ≥ 64 chars before the change.
   - The parse resynchronises at the first restart point that maps to an old chunk start in the unchanged
     tail, with its line start inside the fragment, and reuses the rest.
2. **Text window.** The parse reads a window (64 Ki), never the whole file.
   - A token lexed within 64 chars of the window end invalidates the window. It reopens at the last committed
     chunk, or reads the rest at once.
   - Without this guard, a string running past the window is truncated and corrupts later chunks. The
     equivalence test found this case, and the mutant test now kills it.
3. **Suspendable parse.**
   - The CST parser's 53 token-consuming methods are generators, produced by a mechanical transform:
     `this.m(…)` becomes `(yield* this.m(…))`, and the callbacks of `withNewlineMode`/`enterMode` become
     generator functions.
   - `eat()` yields when the 8 ms slice is over, even deep inside an element.
   - `advance()` returns `null` with the parse suspended. `parsedPos` is the last committed chunk, so
     CodeMirror's `takeTree()` gets its tree at once.
   - Chunks or elements over 64 Ki are built child by child over several calls, balanced 32 per level, never
     in one `Tree.build`.
   - Cost of the generators: about +20 % on a first parse (10 MB: 855 ms instead of 700 ms).
4. **Tuning per parser instance.** `TypstLezerParser.tune({chunk, window, sliceMs, big})` is used only by
   the tests. Multi-range (mixed-language) parses keep the package's original one-shot path. That path is
   also the reference in the tests.

**Limit (block LF0b).** After an unclosed `$`, `#[`, fence or `/*`, no restart point remains. The tail is
parsed again after each keystroke, in slices and without freezing, but the viewport waits for it. §9.

## 4. Markdown: causes and fixes (`patches/@lezer__markdown@1.8.1.patch`, 3 changes in `FragmentCursor`)

Profile at 50 MB, edit at 90 %: `toTree` → `balance` and `parseMixed` `startInner` → `hasNode`, both
O(blocks). The Document got 17 780 children per keystroke at 10 MB: reuse went block by block. Counted
refusals of anonymous nodes: 6 929 for size, 7 for crossing the change.

1. **`maxAnonSize` was computed once per `takeNodes` call** (`(lineStart - block.from) >> 2`). A call that starts
   near the block's start scans to the change with a limit of about 0, so every anonymous node is "too big".
   The comment's intent (¼ of the block's size so far) is now measured at each node. Reuse grows
   geometrically: 6 Document children instead of 17 780.
2. **Anonymous nodes ending in a list or indented code were refused.** They are now taken like the named
   `NotLast` block: reuse may stop after them only once a following node is taken.
   - A first version also took nodes ending in a non-block (the `> ` mark of the next line). It stopped
     mid-line, and the random-edit test caught it.
   - Hence the three-way `lastBlock`: not a block → descend, as before; `NotLast` → take, no stop;
     block → take, may stop.
3. **Pre-existing bug: GFM tables.** It is present in the unpatched 1.8.1 (minimal case: `<div>` inserted into
   a table's delimiter row).
   - A paragraph's end was decided by looking at the next line and at the one after it (`peekLine`, table
     detection).
   - `fragmentEnd` kept only one unchanged line after reused content, so a changed delimiter row left the
     paragraph wrongly cut.
   - Fix: when the fragment ends at a change, `moveTo` drops one more line.

p50 per keystroke through `EditorState` (Node), edit at 10 / 50 / 90 %:

| | 10 MB before → after | 50 MB before → after |
|---|---|---|
| Markdown | 4.9 / 16.2 / 18.7 → 1.2 / 0.9 / 0.8 ms | 21.5 / 58.6 / 97.0 → 3.0 / 1.1 / 1.3 ms |
| Markdown + HTML | 3.1 / 7.9 / 9.6 → 0.9 / 0.9 / 0.8 ms | 13.0 / 42.5 / 60.8 → 1.9 / 1.1 / 1.5 ms |

## 5. `@lezer/common` (`patches/@lezer__common@1.5.4.patch`, 2 changes)

### 5.1 The cursor defect behind wrong colours

**Defect.**
- In buffer mode, `TreeCursor._tree` must be the buffer's parent. The constructor and `enterChild` keep that,
  and `matchContext` relies on it.
- But `sibling()` → `yield(node)` can move from a `Tree` child into a `TreeBuffer` sibling. It then sets
  `this.buffer` and leaves `_tree` on that previous sibling.
- `matchContext`, which `highlightTree` uses for every `A/B` style rule, then reads the sibling as the parent.
- Present in 1.5.4 and on `main`.

**Today's colours are affected, with the original parsers.**
- **Typst:** `#foo` after a call got the function colour (`FuncCall/Ident`) without being a call (suite
  `text/space.typ`).
- **Markdown:** the random-edit test failed on the unpatched packages; an incremental reparse and a fresh
  one paint differently.

**Fix (1 line).** `yield()` sets `this._tree = node.context.parent` when it enters a buffer.

**Reproduction.** `Top(A(B), B)` with the second `B` in a `TreeBuffer`, and `styleTags({"A/B": keyword})`.
- Before the fix: both Bs are painted; `cursor.matchContext(["A"])` gives `true`, `node.matchContext` gives
  `false`.
- After the fix: only the first B is painted.

### 5.2 `parseMixed`'s walk in slices

After a full parse, `MixedParse.startInner` walked the whole tree for nested regions in one `advance()`:
125 ms at 50 MB, a freeze on opening.
- **Change:** the walk keeps its state in `this.scan`, checks an 8 ms deadline every 1 024 nodes, and
  resumes on the next call.
- **Parse position:** `parsedPos` reports the walk's position, so CodeMirror's `takeTree` stops there.

## 6. Measurements (gate `no-freeze`, Node 24, `INKWELL_PERF=1`; 2, 10, 50 MB; keystrokes at 10/50/90 %)

Passing: 16 of 18 cases.
- PERF-HL-TYPST, -TYPST-OPEN, -MD, -MD-HTML and -NESTED pass at 2, 10 and 50 MB: no parse step and no
  keystroke over 50 ms, viewport colours p95 ≤ 16 ms, and the whole file coloured.
- The patched Typst parser costs 0.6–1.9 ms per keystroke at every size, with 99.6–99.98 % of characters
  reused (original: 106–3 438 ms, 0 % reused).

Failing: PERF-HL-TYPST-UNCLOSED at 10 MB (one 60 ms step) and at 50 MB (295 ms). It passes at 2 MB.
- **Cause, measured with `--trace-gc`.** Each slow step is a V8 Mark-Compact pause of 190–210 ms, on a heap of
  1.3–1.7 GB. After an unclosed `$`, the tail is one element of 49.5 MB. The CST parser holds every node of
  an unfinished element as a JS object with a children array (about 100 bytes each, millions of them), and
  builds them all again on every keystroke.
- **Not caused by the slicing.** Between pauses, the steps stay at the 8 ms slice (plus about 19 ms once, to
  read the rest of the file).
- **The fix is a compact node representation in the parser**: a flat post-order buffer, Lezer's own format.
  It belongs to the parser rewrite of block LF0b (§9), not to a patch.

## 7. Same output (gate `equivalence`)

**Red first.** Before the patches, the tests were red: `tune` missing; the Markdown random edits differed; the
TYPST perf cases failed with steps of 154–454 ms.

**After the patches: all 5 named tests pass.**
- **Typst.** The suite corpus (223 files of typst/typst `tests/suite`, commit `84cca40`) plus the repo's
  `.typ` files: the patched parser against the original one-shot path. Two tunings: the default one, and a
  small one (chunk 64, window 256, slice 0 ms, so a yield every 128 tokens; big 300).
- **Markdown.** The repo's 178 `docs/**/*.md`: an incremental parse against a fresh one.
- **Random edits.** At least 50 000 per language, over at least 6 seeds, chained directly and through
  `EditorState`.
- **What is compared.** The painted `highlightTree` spans (ranges and the app's classes), the per-node tags
  and the structure: 0 differences.
- **Mutant.** Removing the window guard is caught by a deterministic string-past-the-window case.

## 8. Upstream texts (prepared; opened by the coordinator with the user's consent)

**(a) PR to kxxt/codemirror-lang-typst — "Make the Lezer parser incremental and time-sliced"**

> `typst_lezer()` reparses the whole document on every edit. `TypstPartialParse` ignores `fragments`, and its
> `advance()` does the full parse in one call, so CodeMirror's time budget never applies. On a 10 MB file a
> keystroke costs ~530 ms (2 MB: ~110 ms). Inside the viewport, CodeMirror stops the parse at `viewport.to`,
> and the cost grows linearly with the edit's position.
>
> This PR keeps the CST parser and its output unchanged and changes how it is driven:
> - **Chunked tree.** The tree is a list of anonymous chunks cut where the top-level loop is in its initial
>   state (line start, no open bracket). Chunks before the edit are reused from the fragments, and the parse
>   resynchronises with the old tree after the edit.
> - **Text window.** The parse reads a text window, invalidated when a token comes near its end.
> - **Generators.** `TypstCSTParser`'s token-consuming methods become generators, so `advance()` returns after
>   a few ms, even inside an unclosed element. Big chunks are built over several calls.
>
> Results: 2 / 10 / 50 MB keystroke 0.6–1.9 ms (was 0.1–3.4 s); no `advance()` over 50 ms; 99.9 % of the tree
> reused. Equivalence test: the typst test suite plus 50 000 random edits over 6 seeds give the same tree,
> tags and painted spans as the one-shot parse. That parse is kept for multi-range parses.
>
> Known limit: after an unclosed delimiter there is no restart point, so the tail is reparsed (sliced).
> From about 10 MB, the per-node objects of that single open element cause GC pauses over 50 ms. A compact
> node buffer in `TypstCSTParser` is the follow-up.

**(b) PR to lezer-parser/markdown — "Incremental reuse: per-node size limit, anonymous NotLast nodes, table
lookahead"**

> Three fixes in `FragmentCursor`, each with a test comparing an incremental parse against a fresh one:
> 1. `maxAnonSize` is computed once per `takeNodes` call. A call starting near the block's start then refuses
>    every anonymous node, and reuse proceeds block by block. On a 50 MB document every edit costs O(blocks),
>    about 100 ms: the Document is re-balanced and `parseMixed` walks every block. Computing the limit at each
>    node keeps the ¼ rule and makes the reuse geometric.
> 2. Anonymous nodes ending in a list or indented code are refused outright. Take them like the named
>    `NotLast` block: reuse stops after them only once a following node is taken. Nodes ending in a
>    non-block are still descended into.
> 3. A table's header line is accepted only when `peekLine()` sees the delimiter row, so a paragraph's end
>    depends on two lines after it. `moveTo` keeps one unchanged line after reused content. Repro: a paragraph,
>    a table header, a delimiter row; insert `<div>` into the delimiter row. The incremental parse cuts the
>    paragraph; the fresh one continues it. When the fragment ends at a change, drop one more line.
>
> Verified on 178 Markdown documents and on more than 50 000 random edits over 6 seeds.

**(c) Issue + PR to lezer-parser/common — "TreeCursor.matchContext after sibling() into a buffer; slice
parseMixed's walk"**

> 1. **`TreeCursor.yield` leaves `_tree` behind when it moves into a `BufferNode`.** After `nextSibling()`
>    from a Tree child to a TreeBuffer sibling, `matchContext` (used by `highlightTree` for `parent/child`
>    rules) matches against the previous sibling. Repro: `Top(A(B), B)` with the second B in a buffer and
>    `styleTags({"A/B": tags.keyword})`; both Bs get painted. Fix: `this._tree = node.context.parent` in
>    `yield` for buffer nodes. The colours then no longer depend on where buffers are cut.
> 2. **`MixedParse.startInner` walks the whole base tree in one `advance()`.** That is O(nodes) in one call:
>    125 ms on a 50 MB Markdown document. Keep its state on the parse, check a deadline every 1 024 nodes,
>    resume on the next `advance()`, and report the walk's position as `parsedPos`.

## 9. Block LF0b: colours within 16 ms after an unclosed delimiter

The need is reuse *inside* an open element, which needs parse state as data that can be compared and
resumed. The candidates, each with fidelity (same colours as Typst), inner reuse, effort and risk:

**(1) Explicit-stack rewrite of `TypstCSTParser`.** The parse state becomes a stack of frames with
checkpoints inside open elements, and the nodes go to a flat buffer.
- **Fidelity:** high (same rules, and the current tests hold).
- **Inner reuse:** yes, by design.
- **Effort:** L, about 1 200 lines rewritten.
- **Risk:** medium. This is a fork of the package's parser, and the fork must follow every Typst release.

**(2) Our own Lezer LR grammar.** Modes, newline and column rules, and contextual `else`/`.` would need
external tokenizers and a context tracker.
- **Fidelity:** medium; it diverges from `typst-syntax` on edge cases.
- **Inner reuse:** yes, natively.
- **Effort:** L.
- **Risk:** high. This is the route the package author rejected.

**(3) tree-sitter-typst in a Worker (web-tree-sitter).**
- **Fidelity:** medium. The grammar lags Typst: `uben0` last pushed in 2025-04; the `SeniorMars` fork is from
  2026-07.
- **Inner reuse:** yes (GLR, error recovery).
- **Effort:** M–L, for the Worker protocol, the query-to-tag mapping, and painting from an async tree.
- **Risk:** medium.

**(4) `typst-syntax` (WASM) in a Worker.**
- **Fidelity:** exact; it is the official parser.
- **Inner reuse:** partial. Its reparser handles markup blocks; for an unclosed `$` it falls back to a full
  reparse.
- **Effort:** M–L.
- **Risk:** medium (async colours, WASM size).

**All options need a compact node representation** (§6: GC pauses of 190–210 ms at 50 MB). (2) and (3)
have one by design; (1) must add one: a post-order buffer, as Lezer builds.

**Recommendation: (1).**
- It is the only option that keeps exact colours and adds inner reuse.
- Its correctness is checked by the harness that already exists: the equivalence corpus, 50 000 edits and
  the mutants.
- It stays in the webview and is synchronous with CodeMirror.
- Proposing it upstream makes it the package's own parser rather than a fork. (4) is exact but does not give
  inner reuse for math; (2) and (3) trade fidelity for it.

## 10. PERF cases (gate LF1, measured here in Node)

- PERF-HL-TYPST, PERF-HL-TYPST-OPEN, PERF-HL-TYPST-UNCLOSED, PERF-HL-MD, PERF-HL-MD-HTML and PERF-HL-NESTED,
  at 2, 10 and 50 MB. Each case: no parse step and no keystroke over 50 ms, viewport colours p95 ≤ 16 ms
  (not for UNCLOSED: LF0b), and the whole file eventually coloured.
- PERF-HL-NESTED also asserts that no nested parser is mounted in a fence or a raw block.
