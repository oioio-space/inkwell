# UX/UI study, part 2 (W65): others' changes in the text, differences, versions, files, tabs, and the surfaces nobody designed

Status: **decided by the owner on 2026-11-14: everything is kept** (T, D, V, F, O, S, G), with one change: she keeps
the git-like tree of the versions, so V1 and V2 were redrawn as a tree (below) and await her view of the new mock-up. Same line as part 1 (`2026-11-14-ux-ui-study.md`, W63):
apparent simplicity, maximum usefulness, beauty, Adwaita. Groups W55 (diff), W56 (version tree), W57 (who did what),
W59–W62 (files, tabs). Kept proposals become references in `docs/design/reference/` and are held by the W66 rule
(`docs/engineering-principles.md` §5, UF-8).

Inputs, kept under `docs/ux/sources/`: `2026-11-14-inventory-2.md` (what exists, data available, coverage of every
surface, what enforces a design today; static reading, nothing was run; **no screenshot of the diff or of the versions
drawer exists in the repository**, so the « today » mock-ups of those two are rebuilt from the code) and
`2026-11-14-practice-2.md` (current practice with sources; no controlled study was found on prose diffs, on tabs for
writers or on live cursors in prose; items not read on a fetched page are marked there).

## 1. Facts that shape the design

| Fact | Evidence |
|---|---|
| History, differences and « who wrote what » are computed in the webview from the Automerge document of `main.typ` only; there is no go-git projection and no per-file history yet | `ui/history/HistoryBars.svelte:22-26`; no `go-git` in `go.mod`; G-0b not done |
| Text arriving from another person shows **no mark at all**; a deletion leaves no trace; nothing says that someone writes above or below the view; no code or test keeps the reading position stable | `editor/amPatches.ts`, `editor/remoteCursors.ts`; no scroll code in `amSync.ts` |
| Arriving patches carry exact ranges and their source: an arrival mark needs only an author lookup | `editor/amPatches.ts`, `patchInfo.source` |
| The diff is a 1100 × 760 modal with two engines: word-level with authors and moves when inline, CodeMirror's line diff when side by side; restoring opens a second modal on top; unchanged stretches fold but cannot be opened; only « X against now » can be compared | `ui/history/DiffView.svelte`, `DiffInline.svelte`, `ApplyDialog.svelte`, `history/layout.ts` |
| The versions list is a lane graph drawn row by row (64 px rows, 16 px per lane) in a 380 px drawer; lanes take their colours from the people palette; any filter hides the named versions; actions sit in a card under the list | `ui/history/HistoryRow.svelte:88-114`, `history.svelte.ts:352-358` |
| Named versions and variants are shared (e2e HIS-06). What differs per person: the grouping into sessions (my own actors, my last visit kept locally) and perhaps the row order (each replica's apply order, not verified). The failing case behind « not synchronised » (W56) is written down nowhere | `history/log.ts:40`, `store/visit.ts:32-38`, `history/graph.ts:50-51` |
| A one-sentence summary of each change already exists (« Marie a ajouté 2 phrases dans « Intro » ») and is used only inside the diff | `history/summary.ts` |
| The files panel is a flat list of buttons, 264 px: no tree keyboard, no filter, no operation, no virtualisation; the size is transported and never shown; operations need the files index writer (ADR 0023) and G-0b | `ui/shell/Sidebar.svelte` |
| The interface says « Fusionner », « fusionnée » and draws a branch for the versions button, against the glossary and the design spec (a clock) | `i18n/fr.json`, `HeaderBar.svelte:126-133` |
| 13 visual baselines exist (shell and people dialog); axe runs in 2 of 17 specs; no raw-value lint; `block_done.py` does not check that the `ux-walkthrough` report exists although the rules require it | `frontend/e2e/**-snapshots`, `scripts/block_done.py` |

## 2. Proposals

Effort S/M/L as in part 1. « Needs » names what must exist first.

### T. Other people's changes in the text area

| # | Change | Why (source) | Effort, needs |
|---|---|---|---|
| T1 | Text someone else just wrote gets a light tint in their colour that fades in about 3 s, and a 2 px bar in the margin that stays until I have seen it; the text itself stays clean | Word « Simple Markup » (margin bar, clean text) and Upwelling (light background for insertions), <https://www.inkandswitch.com/upwelling/>; awareness should arise passively from the work (Dourish and Bellotti 1992, abstract only) | M |
| T2 | A deletion by someone else leaves a small margin mark; hovering or focusing it shows what was removed and by whom | Upwelling and Patchwork: hover or marker rather than struck text | M |
| T3 | When someone writes outside my view, a quiet chip on that edge says so (« Bob écrit plus haut ») and jumps there on click | no source found for this pattern: a design of ours, to be tested in a session | M, presence must carry « is writing » (CP1) |
| T4 | My reading line never moves when text arrives above it; a test proves it | Apple and GNOME both ask that content never jump; today nothing guarantees it | S |
| T5 | Caret flags: the name shows when the person arrives or moves, then folds into the caret after 2 s; text colour chosen by luminance; every caret at 3:1 | WCAG 1.4.11 and 1.4.1 (<https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html>); four of the eight carets are under 3:1 today, and the flag text is a hard-coded black | S |
| T6 | « Depuis votre dernière visite » becomes margin bars plus one chip (« 12 changements depuis hier 17:42 · Voir ») instead of a strip that pushes the text | part 1, P5 and P12: nothing jumps, one place per state | S |
| T7 | « Qui a écrit quoi » is a view one switches on from the central pill (and a shortcut), off by default | iA Writer authorship, <https://ia.net/writer/support/editor/authorship>; today it hides in the versions drawer | S |
| T8 | « Écriture calme »: hide others' carets and marks while I write, remembered | requested of Figma, Miro and Lucid by their users (search snippets only); fits focus mode (P10) | S |
| T9 | A live log of others' changes for screen readers, polite, grouped by person | Google Docs offers one (search snippet only); WCAG 4.1.3 status messages | M |

### D. Differences

| # | Change | Why (source) | Effort, needs |
|---|---|---|---|
| D1 | The comparison opens in place of the editor, not in a modal; the preview stays beside it; Esc returns to writing | HIG: prefer views inside the window to new windows (<https://developer.gnome.org/hig/guidelines/navigation.html>); two stacked modals today | M |
| D2 | One engine: word-level with semantic cleanup for both the single-column and the side-by-side view | Fraser, « Diff Strategies » and diff-match-patch semantic cleanup (<https://neil.fraser.name/writing/diff/>): character and line diffs of prose read as noise | M |
| D3 | Quiet by default: added text tinted and underlined, removed text as a margin mark that opens on demand; a four-way switch Simple / Tout / Avant / Après | Word's four markup views; Upwelling: heavy strike-through makes prose unreadable; WCAG 1.4.1: never colour alone | M |
| D4 | A replaced phrase reads « avant → après » in the margin; a moved paragraph is one move, not a deletion and an insertion | Patchwork notebook (<https://www.inkandswitch.com/patchwork/notebook/>); MediaWiki moved-paragraph marks; the move detection already exists | S |
| D5 | The sentence that sums up the change first, then a count with previous and next, a minibar showing where the changes fall, and folded stretches that open | Patchwork (summary, minibar), VS Code (collapse unchanged regions), Word (next and previous change) | M |
| D6 | Compare any two moments (« De » and « À »), not only against now | Overleaf history compares any two points; already in the design spec §4.22, not built | M |
| D7 | Go back on one passage only (« Rétablir ce passage ») | Word accept or reject per change; today only a whole session can be undone | M |
| D8 | Screen readers hear « ajouté » and « supprimé » | WCAG 1.3.1; absent today | S |
| D9 | Later: the two typeset pages side by side with the changed zones marked | Figma and PDF comparison tools; needs a compile at a past version | L, after R1 |

### V. Versions

| # | Change | Why (source) | Effort, needs |
|---|---|---|---|
| V1 | **The tree is kept and redrawn** (owner, 2026-11-14; the first proposal replaced it by a timeline): one continuous trunk for the main text, one curved branch per variant in a branch colour that is not a person's colour, faces as the nodes of writing sessions, a diamond for a named version, the variant's name on a chip at the tip of its branch; one drawing behind the rows instead of one fragment per row; a fixed graph column so the text stays aligned (at most three lanes drawn, the rest behind a count); 46 px rows with the summary sentence and the time; day separators | the owner's choice; what made today's tree hard to read is measured in the inventory: per-row fragments, lanes in people colours, a text column that shifts with the lanes, no labels, 64 px rows. Clean lane drawing follows the practice of Git clients (Fork, Sublime Merge, GitKraken: not read on a fetched page) | M |
| V2 | Variants keep their name, « Variante » (owner, 2026-11-14: « une branche n'est pas un brouillon » — a variant is another wording of the text written beside the main text, not an unfinished one; no git word unless it is the clearer one), and **are the branches of the tree**. Vocabulary of the surface: « texte principal » (trunk), « variante » (branch), « version nommée » (diamond), « intégrer » (bring a variant into the main text), « abandonner » / « rétablir »; selecting a draft's tip offers Ouvrir, Comparer, Intégrer, Abandonner on its row; an integrated draft folds into one row with a small loop on the trunk and unfolds on click; hovering a branch dims the others | Upwelling: « draft » is the word writers understand; Patchwork: a merged branch shows as one history item; glossary: « intégrer », never « fusionner » | M |
| V3 | Filters as chips (« Versions nommées », a person, a period) that never hide the named versions | Google Docs « only show named versions »; today any filter removes them | S |
| V4 | Actions on the row itself; going back acts at once, creates a new moment and offers « Annuler » in a toast | Figma, Notion, Pages: restoring never destroys; part 1, P24 | S |
| V5 | Selecting a moment shows the text as it was, read-only, with one bar: « Texte du 12 novembre, 17:42 · Revenir à maintenant · Restaurer » | Pages « Browse All Versions »: preview before restoring; in the design spec, not built | M |
| V6 | A name and an optional note for a version | Overleaf labels; the spec's HI-3 | S |
| V7 | The same list for everyone: sessions and order computed from shared data only, proved by a two-instance test; the W56 case is reproduced first | W56; today the grouping depends on who looks | M, a reproduction |
| V8 | One name, « Versions », a clock on the button, a shortcut, and the three latest moments at the bottom of the files panel with « Tout voir » | part 1, P30; header design of 2026-11-13 | S |
| V9 | **Several people work together inside a variant** (owner, 2026-11-14, W67; already N46 and VA-3 of the spec, `docs/architecture/spec.md:1595-1602`): everyone sees every variant and joins one in one gesture; the faces of the people inside show at the tip of its branch; inside, everything works as in the main text (live text, carets, arrival marks T1–T2, preview, problems, sync state of that variant); the central pill always says where one writes (« Variante « Intro courte » » in the branch colour); people on the main text are told who is in which variant and never see a misplaced caret; when someone integrates a variant while others write in it, they are told and brought back to the main text; a variant can be updated from the main text and integrated again (VA-4) | today another person can open a variant and read its text (e2e ZZ-1, ZZ-2), but presence is hard-wired to `main.typ` (`editor/presence.ts:30`, `:47`), nothing carries the variant, no test has two people writing in one variant, the preview and the problems stay on the main document (`language/problems.svelte.ts:48`), a second integration does nothing (`history/versions.ts:243-244`), and `editVariant` has no role check (`history.svelte.ts:601`). Patchwork and Figma branching treat a branch as a shared place | L, presence carries the document (CP1); whether Go attests variant documents is to verify first |

### F. Files

| # | Change | Why (source) | Effort, needs |
|---|---|---|---|
| F1 | A real tree: arrow keys, Home and End, type-ahead, folders that fold, a virtualised list | WAI-ARIA Authoring Practices, Tree View (<https://www.w3.org/WAI/ARIA/apg/patterns/treeview/>); the owner's large Typst folders (W59) | M |
| F2 | A filter field above the tree (the ancestors of a match stay visible) and a quick switcher, Ctrl+P, that finds a file or creates it | VS Code tree filter and Quick Open; Obsidian quick switcher; JetBrains speed search (the reference the owner named) | M |
| F3 | Everything a file manager does: new file and folder, import, rename in place (F2), move by dragging, delete with « Annuler » and a project trash kept 30 days; every operation is shared | W62; HIG: undo instead of confirming; Zed's guests can only delete for good in a shared project, so the recoverable trash must be designed in | L, files index writer (ADR 0023), G-0b |
| F4 | Each row tells what matters: type icon, the main document's mark, the faces of the people in that file, a dot when others changed it since my last look, a problem count that also shows on the parent folder | Zed (status propagates up), Figma and Live Share (presence in the tree); only the name and a « principal » label today | M, presence carries the file (it does) |
| F5 | Single-child folders fold into one row; « Montrer le fichier ouvert » | Zed and JetBrains compact folders, JetBrains « Select Opened File » | S |
| F6 | Empty and receiving states (« Ce dossier est vide », « En cours de réception… ») | part 1, P23 | S |
| F7 | Drop files from the desktop to add them | part 1, P27 | M |

### O. Tabs

| # | Change | Why (source) | Effort, needs |
|---|---|---|---|
| O1 | A tab bar that appears only when a second file is open | HIG tabs (<https://developer.gnome.org/hig/patterns/nav/tabs.html>) and AdwTabBar autohide: one bar less for the person who writes one document | M, G-0b |
| O2 | A single click previews a file in a reusable tab (italic title); a double click or an edit keeps it; tabs can be pinned | VS Code preview editors; libadwaita pinned tabs | M |
| O3 | Ctrl+Tab in most-recently-used order, Ctrl+W, Ctrl+Maj+T to reopen, Alt+1…9 | HIG keyboard reference; VS Code | S |
| O4 | A tab shows what the tree shows: unsaved dot, a face when someone else is in the file, a problem mark | consistency with F4 | S |

Evidence for or against tabs for writers is anecdotal; they stay hidden for a one-document project (O1), so they cost
nothing to those who do not need them.

### S. Surfaces nobody had designed

| # | Change | Why (source) | Effort, needs |
|---|---|---|---|
| S1 | First launch and project chooser: one calm window — name and colour, then Nouveau document, Ouvrir, Rejoindre avec une invitation, and the recent documents | Apple launching guidance, GNOME Builder's greeter; the first minute of every new person has no design today | M, « Première minute » |
| S2 | One banner at a time, by priority, the others behind a count; undo toasts | HIG banners and toasts; seven banner sources stack today | M, BannerHost |
| S3 | Problems: a count, a list in plain words, a click goes to the line, F8 for the next | VS Code Problems; Overleaf's simplified log | S |
| S4 | Search and replace in the whole project: results by file, a preview, changes one can untick before applying | VS Code refactor preview | M, G-0b |
| S5 | Export: one small dialog (format, pages, « Ouvrir après l'export ») that remembers the last choice | HIG action dialogs; no source read on export specifically | S |
| S6 | One reference dialog and one reference menu, from which the seven dialogs and the format and insert menus are redrawn | part 1, P3 | M, UF1 |
| S7 | The spreadsheet's own toolbar, status bar, find bar and menu aligned on the shell's | the only surface outside the W63 study | M |
| S8 | Outline panel (« Sommaire ») as the second tab of the left panel | coherence §6.3; Google Docs outline, Word navigation pane | M |
| S9 | Presentation views checked against the reference and given baselines | walkthrough defects S11-5…9 still listed | S |

### G. Keeping the designs (W66)

| # | Change | Why | Effort |
|---|---|---|---|
| G1 | `block_done.py` refuses an interface block without its `ux-walkthrough` report and without the comparison to the reference | the rule exists in writing only | S |
| G2 | Reference captures per surface, light and dark, for every surface of this study as it is built; a raw-value lint (UF-4) | 13 baselines today, none for the diff, the versions, the files panel or the text with others' changes | M, UF1 |
| G3 | Measured invariants per reference (bar heights, labels on one line, order and presence of controls) in each block's tests | catches what pixels miss when content changes | S per block |

## 3. Suggested order

1. Guards first (G1–G3, with UF1), as decided for W66.
2. The text area (T1, T2, T4, T5, T6) with PR1 and CP1: it is the surface people look at all day, and the data is there.
3. Versions and differences together (V1–V5, V8, D1–D5, D8), in the « Relire » block; V7 after reproducing W56.
4. Files and tabs (F1–F7, O1–O4) with « navigation in the project », after G-0b; F1, F2, F4–F6 do not need G-0b and can
   come earlier.
5. S1–S9 with their blocks; D6, D7, D9, T3, T8, T9, V6 after R1.
