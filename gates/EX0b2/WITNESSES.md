# EX0b2 - red witnesses

Recorded 2026-11-25 at step 1, before any production change, on the worktree `inkwell-ex0b2` at `main` 700face (EX0b1)
plus the plan `docs/plans/2026-11-25-ex0b2-palette.md`, the pointer in §7 of the EX0b plan, the PROGRESS lines EX0b2 and
EX0b3 and this draft gate. Each command was run as written in `gates/EX0b2.toml`, from the worktree root, through
`sh -c` as `scripts/gate.py` does. Logs are in the session scratchpad (`witness-EX0b2-<id>.log`). Before each exclusive
criterion, `pgrep -af 'heavy.sh --exclusive'` showed no other exclusive run (OBS1 is implementing in another worktree).
Round 1 is the first draft; round 2 reran, once each, the criteria the blind review changed (`docs`, `ranker-lint`,
`chunks`); the others' commands are unchanged since round 1.

| Criterion | Round | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| docs | 2 | 1 | < 1 s | spec §8.2 rows « Palette » and « Raccourcis » not « fait (`app.palette`) » / « fait (`app.shortcuts`) »; §4.24 still « à créer »; §4.26 does not name `app.shortcuts` and AdwShortcutsDialog. Found: plan rows P-1..P-10 at 107 lines (≤ 110), §7's pointer, the PROGRESS EX0b2 line, « → EX0b3 » and its line, ux-principles rule 3, rule 48 | yes (spec written with the code). EX0b1's `docs` is still green after the §7 and PROGRESS edits |
| no-hard-keys | 1 | 0 | < 1 s | EX0b1's criterion read from `gates/EX0b1.toml` and run: green on EX0b1's merged tree | green by design: a regression guard over every `.ts` and `.svelte`, the new components included |
| ranker-lint | 2 | 1 | < 1 s | `frontend/src/ui/command/rank.ts` is absent | yes. Tool checked on a probe copy: it flags `indexOf`, `startsWith`, a regex literal, `new RegExp`, `/x/.test`, and passes a comment naming `indexOf` and a division |
| chunks | 2 | 1 | 2 s | a real Vite build (593 modules): `Palette.svelte` and `ShortcutsWindow.svelte` are not dynamic chunks of their own; no file holds `"e2e-many.` | yes. Detection checked: the App chunk's sourcemap lists `../../../../frontend/src/ui/command/CommandButton.svelte`, so a statically imported palette would be caught |
| frontend | 1 | 1 | 333 s | Biome clean (521 files), svelte-check `0 ERRORS 0 WARNINGS` (814 files), Vitest 1447 passed, 0 failed, 1 todo; the ten rule names P-1..P-10 `absent` | yes (333 s of 480 s, load 1.5–1.9) |
| mutants | 1 | 1 | < 1 s | MP1..MP10 missing; runner not reached. `mutants.py --check`: 267 entries, 0 problems | yes (entries written at step 2) |
| e2e | 1 | 1 | 24 s | `e2e:build` done (6.5 s, warm); Chromium: EX0b1-E1, EX0b1-E2, UI-02, UI-03, SAV-03b passed first time (5 expected, 0 unexpected); EX0b2-E1, E2, E3 `absent` | yes |
| e2e-webkit | 1 | 1 | 14 s | « No tests found » (`ex0b2.spec.ts` does not exist), EX0b2-E1 and E2 `absent` | yes. Tool checked apart: the same command on `ex0b1.spec.ts --grep EX0b1-E2` ran in Playwright's WebKit container (project `webkit`), 1 expected, the JSON report written through `PLAYWRIGHT_JSON_OUTPUT_NAME` |
| perf-start | 1 | 0 | 115 s | PERF-START `pass`, paired against the merge-base 700face, 80 pairs, ratio 1.0008, bounds 0.989–1.013 (half-width 1.21 %), contaminated 0; budget `pass` (median bound 253 ms ≤ 350, p95 bound 268 ms ≤ 400) | green by design: a regression guard on the unchanged tree |
| simplify-report | 1 | 1 | < 1 s | `gates/EX0b2/SIMPLIFY.md` absent | yes |

Red for the right reason: 8 of 10; `no-hard-keys` and `perf-start` are regression guards, green on the unchanged tree.

## Caps (rule G-1)

- Measured: `frontend` 333 s (cap 480); `e2e` 24 s with a warm build for 5 tests (caps 200 + 300: the EX0b2 scenarios
  add 21 timed opens and queries over 1 000 fixture commands); `e2e-webkit` 14 s with no test, about 25 s for one test
  with the container start (cap 420); `perf-start` 115 s (cap 600); `chunks` 2 s build (cap 120 + 30).
- `mutants`: no EX0b2 entry before step 2; EX0b1's 12 single-file Vitest entries are the model; cap 300 s.
- The caps add up to 2590 s within `max_minutes = 45`; `scripts/gate.py`'s `deadline_problem` returns `None`.

## What each named test must assert (checked at the code review and by the mutants)

- **P-1** [MP1]: fold on a table: « Écrire », « ÉCRIRE », « ecrire » equal; « cœur » = « coeur », « Æ » = « ae »,
  « Straße » = « strasse »; « Rechercher une commande… » = « rechercher une commande... »; « « Gras » » = « gras »;
  « l’aperçu », « mise-en-page », « a/b », « x:y », « (z) » split into words at ’ - / : ( ); with a counter, a
  registry's labels and keywords are folded once, a second query and 100 keystrokes fold none, a language change once.
- **P-2** [MP2]: at least 24 rows (query, label, keywords → class or none): equal, prefix, « pan gau » and « panneau
  gauche » (« de » skipped) → class 3, « gau pan » → none, « gauche gauche » on « Panneau de gauche » → none (a word used
  once), « de gauche » → class 4, « pdg » → none, « apercu » after ’ → class 3; a built-in keyword key read by `t()` in
  French and in English; a third-party literal keyword found in both languages; an empty query → no class; 65
  characters cut at 64, 9 words cut at 8.
- **P-3** [MP3]: with recents [B, A] and a query where A is a prefix match and B a one-piece match, A is above B; two
  prefix matches order by recency, then by `Intl.Collator("fr")`; without a query recents first in recency order, then
  by label; every unavailable row below every available one.
- **P-4** [MP4]: palette runs A, B, A store [A, B]; a key or menu run stores nothing; 60 runs keep 50; an unknown id
  dropped; `"x"`, `{}`, a list of numbers read as none; a `localStorage` throwing on read and write leaves no recents.
- **P-5** [MP5]: opened with the focus in a Typst editor, `typst.signature-help` (`focus == editor && fileType ==
  typst`) is available (MP5 evaluates against the palette's field, `focus == other`, and greys it); opened from a
  Markdown file, `typst.present` is `aria-disabled`, its reason in the element its `aria-describedby` names; Enter and a
  click call `announce` with it and never `run`, the palette stays open; `app.find` opened from the editor is greyed
  with its reason, from a grid it is available; every built-in command with a `when` has an `unavailable`, every
  surface-only one has one, `app.return` has `when: canReturn`; the vector `missing-key.command-when-unavailable` is
  refused; an `evalWhen` counter: ≤ N at opening, 0 more over 20 keystrokes.
- **P-6** [MP6]: `role="combobox"`, `aria-expanded="true"`, `aria-controls` = the listbox id, `aria-autocomplete="list"`,
  `aria-activedescendant` = the row with `aria-selected="true"` after ↓, ↑, PageDown, PageUp; 120 commands: at most 50
  `option` rows drawn, PageDown repeated reaches the 120th, which is then drawn and active (MP6 cuts at the first 50);
  the `status` region is inside the dialog, says « 120 commandes », « 1 commande », « Aucune commande » and the English
  forms, once 300 ms after the last keystroke (fake timers: 5 fast keystrokes, one announcement).
- **P-7** [MP7]: focus in a mounted editor, open, Enter: a recorder shows the focus back on the editor before
  `run(id, editor)`; opened from a main-menu item (then detached), the target is the menu's return target, the editor
  (MP7 keeps the detached item); a `mousedown` on a row leaves the focus in the field; Escape restores the focus and
  records no run; `app.palette` is not listed; `app.palette` while open focuses the field and selects the query;
  `app.shortcuts` while open closes the palette and opens the window; a registry chord runs nothing behind the dialog.
- **P-8** [MP8]: 1 000 seeded synthetic commands; queries « a », « pan gau », « de la page » and 63 « a » then « b »
  (labels of « a » runs): `rankStats.comparisons` ≤ 4 × words × Σ(label + keyword lengths), `rankStats.selection` ≤
  12 × 1 000, rows ≤ 50, `rankStats.folds` = 0. MP8 (a naive search restarting at each position) exceeds the bound on
  the 64-character query only.
- **P-9** [MP9]: on linux, mac and win, the command rows equal the resolution's active chords of that system, each
  once, in `CATEGORIES` sections, rendered by `renderChord`; « Écrire » holds exactly the `keytables.ts` entries with a
  label key (none a bare arrow, Home, End, PageUp or PageDown), each marked « géré par l'éditeur »; a stored binding of
  hers marked « personnalisé », the default it replaced absent, `null` removes the row; a `delegated: system` chord
  marked; the search « zoom » leaves only the zoom rows.
- **P-10** [MP10]: the plan's chords on the three systems and the main-menu items (`project` 90, `app` 15) with their
  labels; building the registry and `activateStartup()` call neither component's loader; two runs load each once;
  `app.shortcuts` mounts the window, `app.palette` the palette.
- **EX0b2-E1**, **E2**, **E3**: as in the plan §2; E2 reads `performance.getEntriesByName` for its two marks over the
  1 000 fixture commands, prints the eleven open times and ten query times, and asserts the bounds.

## Open questions of round 1, decided by the integrator after the review

1. Typo tolerance: not in EX0b2; a residual → EX0b3, judged from OBS1's count of queries with no result; never a
   scattered-letter class. 2. Recents: palette runs only. 3. The window has an « Écrire » section with the labelled
   editor and grid keys, marked like delegated ones, no bare cursor movement. 4. PERF-PALETTE times 1 000 fixture
   commands in both engines (review finding 3).
