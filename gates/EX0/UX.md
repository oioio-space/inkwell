# EX0 — UX walkthrough (2026-11-24)

Severity scale of the brief: 1 blocking .. 4 cosmetic. Verdict: **fix first** (no severity-1 defect; two severity-2 defects, each with a failing e2e test, fixes proposed
not applied). Everything below was walked in the real app (e2e server build of the tree, chromium 1280x760, fr-FR, light
and dark; fresh temporary INKWELL_HOME and INKWELL_DOCS, checked non-empty). Screenshots: `/tmp/claude-1000/-home-mathieu-GolandProjects-inkwell/61fa9e48-30a6-4e0a-bf70-5c7eb177fa69/scratchpad/shots/`.
Single-user scenarios only (no multi-user step in EX0). Heavy work went through `scripts/heavy.sh`.

## What works (walked end to end)

- S1 project of 10 files: icons are right (typst, markdown, csv grid, svg, text); README and LICENSE start generic and
  turn to the text icon once opened (01-tree, 02-icons, 03-*). The 50 MB zip and a 3 MB unknown binary show a card in
  ~65 ms (no read of the whole file visible). main.typ opens in ~40 ms, three runs, no grid or Markdown chunk needed.
- S2 data.csv: table opens, "Voir le texte" top right, "Voir le tableau" back, light and dark consistent (04/05, -d-).
  Text to table keeps the line (text line 4 to row 4).
- S3 a tabular .txt gets "Ouvrir comme tableau ?"; main.typ and liste.txt do not. The choice is kept after a reload,
  per project, on this device (localStorage `inkwell.openWith`).
- S4 failed grid load (chunk blocked): one notice "Cet affichage n'a pas pu s'ouvrir : le fichier s'affiche tel quel.",
  ~7 s, the text shown, no second notice on reopening, no second fetch (13-fail-data.csv, 14-fail-again).
- Wrong content: a .txt holding binary shows "Ce fichier ne contient pas du texte…" (16-faux.txt); empty and ragged CSVs open.

## Findings

| Id | Sev | Finding | Screenshot |
|---|---|---|---|
| UX-EX0-1 | 2 | Table to text drops the caret at line 1 col 1: select Lyon (row 3), "Voir le texte", status says "Ligne 1, colonne 1". The other way round is correct. A person on a long CSV loses her place. | 05-l-as-text |
| UX-EX0-2 | 2 | Grid chunk fails on a tabular .txt: "Ouvrir comme tableau ?" is still shown, and pressing it does nothing, with no notice (dead button, heuristics 1 and 9). | 15-fail-offer-click |
| UX-EX0-3 | 4 | The switch does not look or sit the same in both views: "Voir le texte" has an icon, right-aligned in the sheet toolbar; "Voir le tableau" and "Ouvrir comme tableau ?" are bare left-aligned words with no icon (they read as a title until hovered). Heuristic 4. | 04-l-table-edit-try vs 05-l-as-text, 08 |
| UX-EX0-4 | 3 | After "Entrée" on the switch, keyboard focus falls on `body`: Enter cannot toggle back; one more Tab lands in the editor. Léa/keyboard. (The toggle predates EX0.) | 12b-after-enter |
| UX-EX0-5 | 3 | Memory is uneven: "Ouvrir comme tableau" is remembered, "Voir le texte" is not (a CSV left in text reopens as a table; tableau.txt reopens as a table after being viewed as text). Say which in the ADR, or remember both. | s5/s11 logs |
| UX-EX0-6 | 3 | Cards are dead ends: "archive.zip / Aucun aperçu pour ce type de fichier." gives no size, no type, no next step; "fausse.png" says only "Ce fichier ne peut pas être ouvert." (why? what now?); faux.txt keeps a text icon on a non-text file. Heuristics 6, 9. | 03-05-archive.zip, 16-fausse.png.png |
| UX-EX0-7 | 3 | Read-only text: "Dans cette version, ce fichier s'affiche en lecture seule" on every non-main file (G-0a, not EX0): the persona task "edit a cell as text, switch back" cannot be done at all; W79 is satisfied as « voir » only. Empty table still says "Tapez dans une cellule pour commencer" while read-only (misleading). | 03-02-data.csv, 16-vide.csv |

UX-EX0-7 is a known limit (G-0b), not an EX0 regression; the decision to merge EX0 with W79 marked "voir" only is the
integrator's. It also means "edit as text then switch back" (task 2) is untestable until G-0b.

## Failing e2e tests (written first, both red on the tree)

`frontend/e2e/ux/ex0.spec.ts`: `UX-EX0-1` (status must read "Ligne 3, colonne N" after the switch), `UX-EX0-2` (the offer is
gone or a notice explains) — run: `cd frontend && ../scripts/heavy.sh ./node_modules/.bin/playwright test e2e/ux/ex0.spec.ts`.

## Proposed fixes (not applied)

- UX-EX0-1: the grid already hands its active row to the text view the other way; do the same in `Sheet.svelte` (the
  `data-sheet="text"` handler, ~l.543): pass the active cell's (row, col) to the text editor and set the selection to
  the offset of that field (line = row, column = sum of the previous fields' lengths + delimiters, using `options.delimiter`).
- UX-EX0-2: have `offersTable` (`workspace/viewChoice.svelte.ts`) return false once the viewer's load is known failed,
  or make `openAsTable` (`App.svelte` ~l.590) show the same failure notice; the failed state is already memoized.
- UX-EX0-3: one `.viewbar` component for both views, right-aligned, icon + label, same as the sheet's.
- UX-EX0-4: after the switch, focus the new view's switch (`tick()` then focus).
- UX-EX0-5: decision to record in ADR 0027 (open question 2).
- UX-EX0-6: show size and type on the card, "Ouvrir avec une autre application" when available; say why for a bad image.

Light and dark compared on the neighbouring screens: no contrast or theme defect found. Not covered: WebKit, 200 % zoom,
forced colours, a real viewer-load failure on a slow network (simulated by blocking the grid chunk).

## Answers (2026-11-24, implementer)

- UX-EX0-1: fixed. The read-only text view took no caret position; `createReadOnlyEditor` and `ReadOnlyText` now take
  `at`, so the caret lands on the table's active cell, as the editor of the main document already did.
- UX-EX0-2: fixed. A grid that failed to load this session is no longer offered (`offersTable` asks
  `registry.hasFailed`); the first failure gives the one notice, now a `status` toast.
- UX-EX0-3: → UX1 (PROGRESS.md, UX1 line), tagged [UX-EX0-3].
- UX-EX0-4: fixed. After a switch of view, the keyboard goes to the view just shown (`focusShown` in `App.svelte`).
- UX-EX0-5: fixed. The view chosen for a file is remembered like its type: device-local, per project
  (`PerFile` in `workspace/viewChoice.svelte.ts`, key `inkwell.viewWith`).
- UX-EX0-6: → UX1 (PROGRESS.md, UX1 line), tagged [UX-EX0-6].
- UX-EX0-7: → G-0b, user's call. Nothing changed.
