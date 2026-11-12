# /simplify — block LF0a3 (incremental highlighting, fixed at the cause) (2026-11-10)

Scope:
- the pinned patches `frontend/patches/codemirror-lang-typst@0.6.0.patch` (3 521 lines),
  `@lezer__markdown@1.8.1.patch` (154) and `@lezer__common@1.5.4.patch` (150);
- `patchedDependencies` in `frontend/pnpm-workspace.yaml`, where pnpm writes it;
- `@lezer/common` 1.5.4, added as an exact dev dependency so the tests can import it;
- the corrected comment in `frontend/src/editor/highlight.ts`;
- the tests: `highlight.equivalence.test.ts`, `highlight.perf.test.ts`, `highlight.testkit.ts`, and the Typst corpus
  in `src/editor/testdata/highlight/`;
- the study doc `docs/plans/2026-11-10-typst-highlight-root-cause.md`.

Review by the main session, against gate `gates/LF0a3.toml` and the golden rule (causes, not symptoms).

## Causes fixed

- **Typst** (`codemirror-lang-typst` 0.6.0): the hand-written parser ignored the old tree on every edit and reparsed
  the whole text in one blocking step. The patch:
  - reuses the old tree around the edit, and reads a text window instead of the whole file;
  - suspends the parse every 8 ms, even inside an element;
  - builds a large tree in slices.

  A keystroke now costs 0.6–1.9 ms at 2, 10 and 50 MB, against 0.1–3.4 s before; 99.6–99.98 % of the tree is reused.
- **Markdown** (`@lezer/markdown`): three causes, two of them already in the original package:
  - the reuse size limit was computed once per call;
  - blocks ending in a list were refused;
  - the GFM table check read one line beyond what reuse allowed.

  A keystroke at 50 MB now costs about 1–3 ms, against 21–97 ms.
- **Wrong colours** (`@lezer/common`): after moving to a sibling stored in a buffer, the cursor read that sibling as the
  parent. This bug was already visible in today's colours (`#foo` in Typst took the function colour). A second fix
  slices the nested-language walk, which froze opening for 125 ms.

## Kept on purpose

- **No size cap**: highlighting stays on at every size, as the gate checks.
- **The size of the Typst patch**: it is close to a rewrite of the parser, which now advances in steps. It is
  accepted as a pinned, tested patch with an upstream route, for two reasons:
  - the equivalence test (painted spans, tags and tree, ≥ 50 000 edits per language over ≥ 6 seeds, 0 differences,
    the window-guard mutant caught) is what holds it;
  - block LF0b replaces this parser with one whose state is explicit and whose node storage is compact.

  Until then, an upgrade of codemirror-lang-typst must re-run the equivalence test.

## Corpus and oracle, re-reviewed (2026-11-12)

New docs made the Markdown random-edit test fail twice: `2026-11-09-c5-sound.md` (seed 4, direct #18) and
`2026-11-11-sandbox-crash-reports.md` (seed 6, editor #11, which depends on timing).

- **Cause.** Neither failure comes from the patches. In both, an inserted `<script>` that is never closed turns the
  rest of the document into JavaScript full of syntax errors. `@lezer/lr` 1.4.11 then parses that region with
  `@lezer/javascript` 1.5.6, and recovers from the errors differently in two situations:
  - after an edit;
  - in a parse resumed from a partial tree, which CodeMirror makes when a parse runs out of its time budget.

  In both situations the result differs from a full parse. The evidence, replayed:
  - the same difference appears with `@lezer/markdown` and `@lezer/common` both reverse-patched to their originals;
  - it appears with `@lezer/javascript` alone on that region, with no Markdown and no `parseMixed`;
  - a parse stopped and resumed with no edit at all also differs (1 stop point in 51 on that document).

  `@lezer/lr` reuses any cached node whose type the current state accepts, even one that contains error recovery.
  A full parse instead recovers from the stack it has at that point. Shrinking by line and by character stops at
  about 4 KB, and a random search over small inputs found nothing. So this is a property of upstream recovery,
  which needs a long run of errors, and not a bug of LF0a3. Nothing changes in the patches.
- **Minimal cases, now fixed.** `src/editor/testdata/highlight/markdown-cases.json` holds both cases: one edit, and
  one resume at `stopAt`. Each records where it was found. The corpus test runs them on every run.
- **Oracle corrected.** `compare()` was too strict on one point: it required the unpatched nested parsers to be
  stable across incremental parses where they recover from errors, which upstream does not provide. It now
  compares the following exactly:
  - the nested trees: language, range, and clean or recovered;
  - inside a recovered nested tree, everything before its first error node;
  - everything else, unchanged.

  Only what reaches past the first error of a recovered nested tree is skipped. The test "LF0 upstream LR recovery is
  not incremental-stable" proves the exception is needed and narrow. On each fixed case it checks that:
  - the exact spans differ;
  - the nested JavaScript differs on its own.

  If an upgrade makes upstream stable, that test fails and the exception goes.
- **Corpus pinned.** `repoMarkdown()` no longer walks `docs/`. It reads the paths listed in
  `src/editor/testdata/highlight/markdown-corpus.txt`: the 194 tracked documents at pin time, in the former order.
  A new document joins only when added there, and a listed one that is gone fails the test.

## Follow-ups (owned elsewhere)

- **LF0b, the unclosed delimiter**: a `$`, `#[`, fence or `/*` left open still pauses at 10 and 50 MB. V8's GC
  stops for 190–210 ms on a 1.3–1.7 GB heap, because every node of the open element is a JS object rebuilt on each
  keystroke. LF0b owns both its no-freeze requirement and its 16 ms viewport colours.
- **Upstream**: three issue or PR texts are ready in the study doc. Publishing them needs the user's consent.
- **Robustness**: the parser reads documents that come from peers. A patched parser that loops forever would freeze
  the editor, and the random-edit equivalence test also acts as a fuzzer. LF0b adds a dedicated fuzz target with a
  time bound per edit.

## Evidence

- Gate criteria `equivalence`, `pinned-no-cap` and `regression` (`fe:check`, 1 243 tests) pass. In `no-freeze`, 15 of
  the 15 cases this gate keeps pass, at 2, 10 and 50 MB.
- The tests were red before the patches.
