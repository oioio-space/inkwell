# EX0b2 /simplify review

Reviewer: Claude Opus (implementing session), 2026-11-25, over the whole diff of the block against `main` 8f5edf8
(registry changes in `frontend/src/extensions/`, `frontend/src/ui/command/`, `ext/app`, i18n, spec, e2e, mutants).
Checklists: reuse, simplification, efficiency, altitude; go-style-guide and helper-ergonomics do not apply (no Go).

## Applied

- Reuse: the palette and the shortcuts window each had their own « give the focus back » and « run this block's two
  commands on their chord » code; both now live once in `dialogKeys.ts` (`giveFocusBack`, `runOwnChord`), which
  reads the chords from the registry (`chordsOf`, `eventChord`), never a literal. Not in `dialogs.svelte.ts`: that
  module is in the start chunk, and a value import from the lazy dialogs made it a shared chunk and split Vite's
  preload helper into a start chunk without a sourcemap (criterion `chunks` caught it).
- Reuse: `commands.ts` gained `textOf` (a command's text read as an i18n key for a built-in) and `labelOf` and
  `availability` use it instead of repeating the origin test; the shortcuts window reads keywords with it.
- Altitude: the dialogs open through the shell (`Shell.palette`, `Shell.shortcuts`, given by `dialogShell`) instead of
  `ext/app` importing the dialogs module: the module stays in the start chunk only, so Vite's preload helper is no
  shared chunk of its own (the first draft failed criterion `chunks` for that), and C-7 checks each entry's shell call.
- Simplification: `runnable()` builds on `availability()` (its `when` and reason) and adds only the handler check
  and the general reason, rather than a second evaluation path.
- Efficiency: the order is one pass into buckets over the label-ordered entries (no sort per keystroke), and
  availability, recents and folding are computed once per opening or per registry and language.
- Tests: the fixture PRNG uses `Math.imul` and the high bits (the first draft's low bits cycled and starved some words).

## Set aside

- A shared dialog stylesheet: every dialog of the application styles itself (Preferences, About, Share); the two
  new ones follow that pattern; a common dialog component is a wider change than this block → UX1 (sub-block UX1-0, K-3 review).
- `entries.ts` keeps its own `textOf(c, text, language)`: it reads a given language (P-1 folds per language), which
  `commands.textOf` (the interface's language) cannot; merging them would widen the registry's API for one caller.
- Typo tolerance in the ranker: out of the block by decision (plan, review question 1) → EX0b3.
- The fixture commands of `e2eMany.ts` are greyed in the palette (data-only, no handler); they exist only to time
  the ranking (E2), so their reason (« Cette commande n'agit pas ici ») is the general one.

## Verified

- No chord literal and no hand-made shortcut label: EX0b1's `no-hard-keys` over the new files (green).
- `rank.ts` compares text only in its counted loops (`ranker-lint` green); `fold.ts` holds the regular expressions.
- Mutants MP1..MP10 each caught by their rule's test (`mutants.py EX0b2`: 10 of 10).
- The fixture is absent from production in any form: no `e2e-many` in any `.js` of the `chunks` build (the
  conditional on `VITE_E2E_OBSERVE` is folded and `e2eMany.ts` tree-shaken). Finding on the frozen gate: the
  minifier writes the fixture's prefixes as template literals (`` `e2e-many.` `` in the e2e build), so the byte
  pattern `"e2e-many.` of criterion `chunks` would not see a leak; the next gate of the family checks the bare
  name instead → EX0b3.
- Residuals: the two items set aside and the gate finding above, each with its block (→ UX1, → EX0b3).

## Code review (blind, « accept with fixes »)

The changed tests were judged adapted, not weakened. Five findings, each fixed in TDD (a test that failed first):
1. MAJOR: the active row was never scrolled into view (off screen after about 13 ↓). Now an effect calls
   `scrollIntoView({ block: "nearest" })` on the active option. P-6 asserts it after 15 ↓, PageDown and the last
   row; E1 checks `toBeInViewport` after 15 ↓ (added with the fix, not run red first).
2. Enter and Escape during an input method's composition acted on the palette. Both dialogs now ignore keys with
   `isComposing` or keyCode 229 (`composing()` in `dialogKeys.ts`, as `keys.ts` reads events). Tested in P-7.
3. The same reason twice was not said twice. The status is emptied, then filled on the next tick. Tested in P-5.
4. A dialog whose chunk was still loading opened after Escape, and a slow earlier request could replace a later
   one. `openDialog` now takes a request token: Escape during the load, a newer request or `closeDialog` drops
   it. Escape is heard through the one key listener (`onEscape` in `commands.ts`), not a second window
   listener, which `no-hard-keys` refuses (the first fix was caught by it). New test « EX0b2 P-10b » (`dialogsLoad.test.ts`, own file: P-10 counts the first load of each chunk).
5. The shortcuts window's search said nothing. It now has its own debounced `status` (« 3 raccourcis », « Aucun
   raccourci »), using the palette's `sayLater` (moved to `dialogKeys.ts` and shared). Tested in P-9.
