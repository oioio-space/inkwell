# inkwell UI inventory for the UX/UI study (W63) - 2026-11-14

Method: read-only. Docs read (ux-principles, glossary, 2026-11-09 ux-research, 2026-11-13 ux-ui-review, report-and-header-design,
si1c-header, c5s-ui, coherence §5, engineering-principles §5, OI and SI1b UX reports, exigences UI rows, PROGRESS table), code
read (App.svelte, ui/**, theme/**, preview, present, i18n), 6 screenshots viewed. Counts come from grep over `frontend/src/**/*.svelte|css`
(tests excluded) and are marked "static" when they count template occurrences, not rendered nodes. Nothing was run or launched, so
anything about runtime behaviour is "not verified". Paths are relative to `/home/mathieu/GolandProjects/inkwell/frontend/src` unless they start with `gates/`, `docs/`.

Existing, larger sources the study should read before proposing: `docs/design/spec.md` (4114 lines, N17 full design spec with
component/token spec §2-§5, states, E-/L-/T- rules), `docs/design/S13-spec.md` (CSD window, page-fit preview, icons), `docs/design/icon-audit.md`.

---------------------------------------------------------------------------------------------------------------------

## 1. Screen map

Window: Wails, 1280x800, **min 720x480** (`cmd/inkwell/main.go:163-167`), system title bar still present (CSD/frameless = N49, planned,
not done: no `Frameless`, no `wails-draggable` in `src`). One window = `App.svelte` (`.window` flex row: Sidebar | `.column` | HistoryDrawer).
Layout stack in `.column` (App.svelte:370-480), top to bottom: HeaderBar 47px, FormatBar 46px (only for `main.typ` of a Typst/Markdown file),
[banners, any number at once], `<main>` (source pane | preview pane), StatusBar 36px.

| Surface | File | Contents | Controls (static count) |
|---|---|---|---|
| Header bar | `ui/shell/HeaderBar.svelte` | left: sidebar toggle, sync status (dot + word, `role=status`); centre: `<h1>` constant "Sans titre" (`App.svelte:436`, `app.title`) + subtitle = current file (+ unsaved dot); right: preview toggle (eye), Present (conditional), avatar stack button, "Partager" (blue), history toggle (branch glyph), main menu | 7 buttons + 1 menu button (+ status span) |
| Main menu (popover) | HeaderBar.svelte:135-215 | Barre de mise en forme (check), Aperçu (check), Présenter / depuis cette diapositive / vue public (3), Exporter en PDF…, separator, Préférences, À propos | 6 to 9 items |
| Format bar | `ui/toolbar/FormatBar.svelte`, `spec.ts`, `BarButton.svelte`, `layout.ts` | heading menu, 13 toggles in 4 groups (B I U S / link code codeblock ∑ / ul ol task quote / table), Insert menu, "⋯" overflow computed by JS width (`layout.ts visibleCount`) | up to 13 + 2 menus + overflow; visible only if `main.typ` and bar-language (`App.svelte:54-58`), so absent on every other `.typ` |
| Insert menu | `ui/toolbar/InsertMenu.svelte` (382 lines) | image/PDF page, figure, include, attach, csv table, data, bibliography, link, cite, ref, footnote, outline (TOC), page break; sub-pickers with "Retour" | 19 `insert.*` keys; 6 `<button` static |
| Heading menu | `ui/toolbar/HeadingMenu.svelte` | Paragraphe, H1-H4 with syntax hints | 5 items |
| Sidebar | `ui/shell/Sidebar.svelte` | fixed 264px; own 47px header "Fichiers" (h2, centred); tree of project files, folders fold, "principal" tag; read-only list | 1 button per file/folder; no add, rename, delete, search |
| Source pane | `ui/Editor.svelte`, `editor/setup.ts` | CodeMirror 6, 14px mono, line-height 1.75, line numbers, wrap, active line tint, remote carets with name flags, placeholder "Commencez à écrire…" | - |
| Read-only text / info card / image | `ui/ReadOnlyText.svelte`, `App.svelte:419-436` | `.md` and other non-main files are read-only (`file.readOnly` banner text), picture viewer, "Aucun aperçu pour ce type de fichier." | - |
| Spreadsheet (csv/tsv/txt-as-table) | `ui/sheet/Sheet.svelte` (739), `Grid.svelte` (1030), `SheetStatus.svelte`, `FindBar.svelte`, `FilterMenu.svelte`, `ContextMenu.svelte`, `PasteDialog.svelte` | own toolbar (undo/redo, row/col ops, sort, filter, layout), formula/name bar, grid, own find bar, own status bar (selection totals, dialect, zoom - / 100 % / +), own context menu, "Voir le texte" | Sheet 8, SheetStatus 10 + 2 menus, FindBar 7, Filter 5 (static) |
| Preview pane | `preview/Preview.svelte` | own tool row (zoom out, "60 %" = fit toggle, zoom in, jump-to-cursor), "Mise à jour…" busy text, page images (white sheet), stale/failed/starting states | 5 buttons |
| Status bar | `ui/shell/StatusBar.svelte` | left: dot + "Seul" / "N personnes"; right: problems (tick "Aucun problème" or counts, opens list popover), "Page X sur Y", "Ligne, colonne", "N mots" (stats popover), "100 %" text-zoom menu, encoding menu (if unusual), language | 3 menu buttons (problems, words, zoom) + encoding + 3 static spans |
| Popovers | `ui/popover/MenuButton.svelte` (+ `StatsPopover`, `ZoomMenu`, `EncodingMenu`) | Adwaita arrow popover; Up/Down/Home/End/Esc; auto placement | used by 9 call sites |
| Share dialog | `ui/ShareDialog.svelte` | name field, "Inviter", invitation text + copy, join field + "Rejoindre", error line, trust note | 5 buttons, 3 inputs |
| People dialog | `ui/people/PeopleDialog.svelte` (361) | per person: avatar, name, state line, role select, "Déconnecter" / "Laisser revenir", "Passer la main", inline `.ask` confirm | 6 buttons + select (static) |
| Preferences | `ui/prefs/Preferences.svelte` | 2 groups only: file saving (auto / Ctrl+S, delay select), outside-edit policy (3-way select) | 2 buttons, 2 selects |
| About | `ui/AboutDialog.svelte` | name, tagline, version, build, licence, offline-typesetting isolation status | 1 button |
| History drawer ("Relire") | `ui/history/HistoryDrawer.svelte` (369), `HistoryRow.svelte`, `HistoryBars.svelte`, `DiffView.svelte`, `DiffInline.svelte`, `ApplyDialog.svelte` | right drawer `min(380px, 42vw)`, tabs Historique / Commentaires / Activité (last two are stubs: `history.comments.empty` "pas encore disponibles"), 4 filters, who-wrote-what, named versions, variants, compare, revert, in-text change marks, diff dialog, apply dialog | Drawer 13 buttons + 7 inputs, DiffView 7, Bars 4, Apply 2 (static) |
| Banners | `ui/Banner.svelte` (tones info, warning only), `SealedBanner`, `AccessBanner`, `ProjectionBanner`, `ExternalBanner`, `PresentBanner`, `HistoryBars`, inline `limit.slow` | 7 sources mounted side by side in `App.svelte:386-400`, no host, no priority | - |
| Toasts | `ui/Toast.svelte`, `NoticeToast.svelte`, `notice.svelte.ts`, `ExternalChangeToast.svelte` | dark pill, bottom, 7 s, queue only in `notice`; second host for external changes | close 32px |
| Presentation | `present/PresentHost.svelte`, `PresenterView.svelte` (423), `SlideStage.svelte`, `PublicWindow.svelte`, `PublicOverlay.svelte`, `PresentBanner.svelte` | full-window presenter view (slide, next, notes, timer, clock, pace, laser, black/white screen, follow/take control), separate public window (`?view=public`) | Presenter 14 buttons |
| Search (project-wide + replace) | `ui/search/SearchPanel.svelte` (769), `SearchField`, `ResultsTree`; `editor/search/**` | full panel, regex/case/word chips, replace with preview, results tree | **built and tested but not mounted in the app** (see §5) |
| Problems list | `language/Problems.svelte`, popover in StatusBar | cf. status bar | - |
| First launch | Go side (`internal/app docs.go:86-104` per the 2026-11-13 review, not re-read) | untitled project opens on `main.typ`, placeholder text; no welcome, no name question | - |
| Empty states | none as a component | text-only: `file.info`, `file.loading`, `preview.fail.*`, `present.empty`, `history.*.empty`, `problems.empty` | no `StatusPage`, no spinner (preview uses a shimmer skeleton) |

Pane logic: `.side-source` min 280px, `.side-preview` min 260 / max 55%, container query 560px makes the preview **cover** the source
(`App.svelte:615-622`). Sidebar never collapses automatically.

Screenshots (viewed): `gates/SI1/ux/joined-synced-light-full.png`, `typing-envoi-0-light-full.png` (full window, light),
`gates/SI1/ux/people-synced-dark.png`, `sending-dark-header.png`, `offline-light-statusbar.png`,
`gates/L2/playwright-results/files-FM-OPEN-1-.../files-readonly-dark.png` (csv in dark), `.playwright-mcp/mock-v2.png` (mocks A, B, 360px).
All are Chromium harness captures at 1280x800 or smaller, not the native WebKitGTK window.

---------------------------------------------------------------------------------------------------------------------

## 2. Design-token inventory

Source: `theme/tokens.css` (185 lines), `theme/adwaita.css` (275 lines). Three tiers declared (primitives `--p-*`, semantic, component), only
`adwaita.css` is "component" tier; components use semantic tokens plus many raw values.

**Colour (semantic, `light-dark()`):** surfaces `--window-bg` (#fafafb / #222226), `--view-bg` (#fff / #1d1d20), `--headerbar-bg` (#fff / #2e2e32),
`--sidebar-bg` (#ebebed / #2e2e32), `--side-bg-2`, `--card-bg`, `--popover-bg`, `--tooltip-bg` (rgb(0 0 6/80%)). Text `--fg` (rgb(0 0 6/80%) / #fff),
`--fg-dim` (55%), lines `--border` 12%, `--shade`, `--card-shade`. Interaction `--btn-bg` 10%, `--hover-bg` 7%, `--selected-bg` 10%, `--accent-bg` #3584e4,
`--suggested-bg` #1c71d8 (white 4.77:1), `--accent` text blue #0461be / #81d0ff, `--warning`, `--error`, `--destructive`, `--success` #2ec27e (no dark variant),
`--error-bg`, `--warning-bg`, `--info-bg`, diff add/del bg+fg, 10 syntax colours `--hl-*`, 7 file-type marks `--mark-*`, 8 Okabe-Ito people `--person-1..8`.
Theme: `color-scheme: light dark`, system-driven, `data-theme` hook exists in CSS only: **no UI toggles it** (grep `data-theme|setTheme|prefs.theme`: nothing in code).
High contrast: one `@media (prefers-contrast: more)` block (tokens.css:139). Forced colours: no `forced-colors` rule (grep, none).

**Typography:** `--font-ui: "Adwaita Sans", "Inter", system-ui, sans-serif`; `--font-mono: "Adwaita Mono", "JetBrains Mono", ui-monospace`; `--font-serif: "Source Serif 4", Georgia`.
No `@font-face` anywhere and CSP `font-src 'self'` (`index.html`): the Adwaita/Inter faces render only when installed on the machine (the screenshots look Inter-like). Base `14px/1.45`
(`adwaita.css:106`), `font-feature-settings: "cv05","cv08"`. **No type-scale tokens**: `--text-small` is referenced once (`language/Problems.svelte:25`) and is not defined anywhere (grep); the only other `--text-*` variable is `--text-zoom`, set at runtime by `ui/history/HistoryBars.svelte:95`.
Distinct sizes actually declared (font/font-size, static): 11, 11.5, 12 (11x), 12.5 (17x), 13 (30x), 13.5, 14 (13x), 14.5 (8x), 15 (12x), 16, 17, 18, 26 = 13 values; weights 400/600/700/800 (7/15/32/1).
Titles: header h1, sidebar h2, dialog h2 and h3 are all `700 14.5px` (HeaderBar.svelte:257, Sidebar.svelte:90, Preferences.svelte:127,141) - no larger heading style exists except About 800 26px.

**Spacing:** `--space-1..4` = 4 / 8 / 12 / 18 px. Used only in 5 files (PeopleDialog, ShareDialog, SearchField, SearchPanel, ResultsTree): 48 of 271 padding/margin/gap declarations (18 %).
Most frequent literal: 6px (55x), then 8 (39), 4 (28), 10 (26), 12 (21), 2 (15), 16 (9), 20 (9), 22 (5); off-grid odd values 3px (5), 5px (4), 7px (2), 1px (3). 709 `px` literals in total (ux-research counted 627 on 2026-11-09).
Fixed metrics: header 47, format bar 46, status bar 36, sidebar 264 + rows 36, preview tool row ~40, dialogs `min(480|520px, 100vw - 32px)`, drawer `min(380px, 42vw)`.

**Radii:** tokens `--radius` 12, `--radius-button` 9, `--radius-small` 6, `--radius-window` 15. Literal/other in components: 50% (17x), 9px (6x, duplicating `--radius-button`), 999px (4x), 99px (3x), 5px, 2px, 11px, 8px, 7px, 4px, 3px, `var(--radius-card, 12px)` (2x, token undefined).
Dialogs use two radii: `--radius` 12 (Share, People, Diff, Apply, Paste) vs `--radius-window` 15 (Preferences, About).

**Shadows:** `--shadow-popover`, `--shadow-card`; 13 `box-shadow` declarations without any token. Dialog shadow `0 12px 32px /25%` (Share, People, Diff, Apply) vs `0 12px 40px /30%` (About, Preferences); backdrop `rgb(0 0 0/30%)` in 5 dialogs, `rgb(0 0 6/35%)` in PasteDialog.
Z-index: 1, 2, 3, 4, 5, 6, 20, 30, 40, 50, 60 literals (no tokens; popover 20, tooltip 30, toast 40).

**Motion:** one token `--duration: 200ms` (0ms under `prefers-reduced-motion: reduce`, tokens.css:145-149); used 4 times, all `transform`/`background` transitions (button background `adwaita.css:20`, sidebar chevron rotate `Sidebar.svelte:149`, 2 search chevrons).
Other motion: sync dot `pulse 1.2s infinite` (HeaderBar:294, stopped under reduced motion), preview skeleton `shimmer 1.4s linear infinite` (Preview:299, stopped under reduced motion; note ux-research said "no skeleton shimmer"), sheet `fresh 4s` flash (Grid:891, stopped), preview `scrollTo` smooth (JS, honours reduced motion, Preview.svelte ~77).
**No enter/exit animation on dialogs, popovers, toasts, banners, drawer open/close, sidebar toggle, preview toggle** (grep `transition|animation|@starting-style|@keyframes`: only the 7 hits above). Easing: none defined (default `ease`).

**Focus ring:** `:focus-visible { outline: 2px solid color-mix(accent-bg 75%, transparent); outline-offset: 2px }` (tokens.css `--focus-ring`, adwaita.css:~110). `.mi` uses offset -2. Review (2026-11-13) measured 2.34-2.95:1 on four surfaces, below 3:1 (WCAG 1.4.11), planned fix = opaque `--accent`. `outline: none` appears 6 times (adwaita.css:114,146; ResultsTree:162; Grid:730,971; PresenterView:240), some with replacement ring.

**Targets:** `--target-min` 44 (used only by dialogs: Share 5, People 5, Prefs 3, About 2, Paste 1), `--target-bar` 34 (header `.fb`, `.btn`, `.mi`, avatar button). Smaller: status-bar `.fb` 28px high, toggle-group buttons 28px (36 in Prefs), banner button 32, toast buttons 32, avatars 28, sidebar rows 36, tree chevron 12, dialog close visible disc 26 inside a 44px hit area.

**Raw colours bypassing tokens** (hex or `rgb(`, outside tokens.css): 53 occurrences in 18 files (static): `present/SlideStage.svelte` 8, `ui/people/PeopleDialog.svelte` 6, `ui/Toast.svelte` 5,
`theme/adwaita.css` 5 (shadows, kbd), `present/PublicWindow.svelte` 5, `preview/Preview.svelte` 4 (white page, `#55555a`, shimmer `#f4f4f6/#fafafb` - light-only, so the skeleton is light in dark mode: not verified visually),
2 each in Share/Search/Prefs/PeopleStack/DiffView/Apply/About/PublicOverlay, 1 each in Paste, Grid (`color:#000`), SearchField, HistoryDrawer. Examples: Toast.svelte:49-51,59,63; SearchPanel.svelte:696 (`#e01b24`); PeopleDialog.svelte:267,271,347,350; PeopleStack.svelte:70,74.
**Latent contrast defect:** `ui/search/SearchPanel.svelte:610-613` `.btn.suggested { background: var(--accent); color: var(--accent-fg) }`: in dark `--accent` is #81d0ff so white-on-#81d0ff = **1.69:1** (computed here); light is 6.07:1. Panel is unmounted today.

Lint status: the UF-4 "no raw value" lint does **not exist** (grep in `scripts/`, `mise.toml`: no hit; only in docs). `tokens.test.ts` checks syntax-colour contrast only.

---------------------------------------------------------------------------------------------------------------------

## 3. Already known defects and already decided changes

Status codes: open / planned (block) / done / accepted.

Header and shell
- Title is the constant "Sans titre", project cannot be named/shown (G1) - planned HB1a (pill with file name) and lot "Première minute et secours" (W40 R4); open.
- Sync state is a label, not a control, no popover (G2, "a new user tries it") - planned: HB1a pill with "who does what" popover, absorbs C5S-UI link line; open.
- Header text overlaps title at <=700px, 360px (UX-SI1b-2, UX-OI "360 px overlap", W53) - planned HB1a/HB1b (SI1c header scope replaced, `gates/SI1c.toml` never frozen); open.
- Sync label blinks 1/s while two people type (UX-SI1b-1) - planned HB1a carrying SI1-C (single "syncing" state); open. "Seul" while undelivered text (UX-SI1-3) - same, SI1-F; open.
- Header too thick, wanted like JetBrains bar (W52), islands/cards look for panes (user 2026-11-13) - decided in `docs/plans/2026-11-13-report-and-header-design.md` §3; planned HB1a; open.
- Header variant B decided: sidebar toggle, versions, export, central pill (file + sync + faces), segmented Texte / Les deux / Aperçu, report flag, menu; "Partager" stays blue suggested - planned HB1a; open.
- Narrow window: title+state on two lines, flag + menu only, bottom view-switcher (mock `.playwright-mcp/mock-v2.png`), sidebar closed below breakpoint - planned HB1b; open.
- "Relire" uses a Git-branch glyph (jargon by image) (ux-ui-review §2) - open, not assigned (HB1a may replace icon: not stated).
- Window decorations / CSD (N49), app icon visible (N57), window controls - planned lot S13; open.
- Sidebar fixed 264px never an overlay (G11, R10 four window states spec §3.4) - planned W40 R10 (lot "Écrire et retrouver"); open. Note native min width is 720px (main.go), so 360px behaviours are harness-only unless MinWidth changes (not stated in any doc I read).
- Sidebar: own header bar with project name, "+", search, versions list below files (decided in header design §3; functions belong to G-0b, W59/W59b big-folder file manager) - planned; open.
- Preview page not the size of the writing area (W54, N50, N54 PDF-viewer behaviours, N55 sync) - planned HB1a (default fit) and lot S13; open.

Feedback and messaging
- 7 banner sources stack, no host; Banner lacks error tone (G3, R2) - planned "BannerHost + ToastHost" after SI1 (R1/R2); open.
- Toasts: several hosts, no queue/fade, raw colours (G4); queue exists for `notice` (OI) only - partly done; open.
- Interim toasts are dead ends (OI-4), accepted until OI3; slow-open toast vanishes after 7 s - accepted.
- Export error toast has no action although F8/`jumpToProblem` exist (ux-ui-review item 7) - proposed, not scheduled; open.
- Technical `{reason}` text inside sentences, 4 keys (G12, UX-S7-4) - open (ux-ui-review item 3 proposes i18n tests UI-11/UI-12).
- Banner "Liste de membres saturée - créez un nouveau projet" asks for a command that does not exist (ux-ui-review §2) - open.
- Arrivals: no toast when someone joins; invitation single-use not said (ux-ui-review item 6) - proposed; open.
- Reversible actions ask confirmation (`people.suspend.ask`) against rule 6 - minor, open.

First minute
- No welcome, no name/colour, no sample project, no Projects screen, joining hidden in Share, may orphan the joiner's text (G6, R4, ux-ui-review §1, items 1, 2, 9) - planned lot "Première minute et secours"; open. Generated names "Personne 12D3" / machine name (UX-S7-3, class G) - open.
- Share dialog mixes invite and join, no role choice, no fingerprint compare, error far from field (G7, R5) - planned UF2 (statechart-first); open. W58 (invite refused on foreign project, raw error) - block 11, blocking R1; planned.

Keyboard, commands, tokens
- No command registry, palette, shortcuts list, Ctrl+, , F10 (G5, R3); `ux-principles.md` rule 3 already promises Ctrl/Cmd+Shift+P and menu "Rechercher une commande…" - planned W40 R3 (lot "Écrire et retrouver"); open. Ctrl+Shift+S is reserved for the report tool (RP1 design).
- Token set is a subset of spec §2 (types, durations, eases, z, bar heights, pill radius) (G10, R6); 627-709 raw px - planned UF1 + UF-4 lint; open. Reduced-motion should keep fade token (G14) - open.
- `title=` tooltips everywhere, no Tooltip component (G8, R7); 49 `title=` in 18 `.svelte` files (re-counted: 49 / 18); avatar names by mouse hover only; `<ul>` inside `<button>` (G9) - planned UF1; open.
- Focus ring 2.34-2.95:1; dim text 4.26:1 on selected row; carets of 4 of 8 person colours <3:1 (ux-ui-review §2) - proposed (tokens.test.ts extension) and CP1 for carets; open.
- axe in only 2 of 17 Playwright specs; no hit-test invariant; ARIA snapshots and APG contracts missing (UF-1, UF-2, UF-10..12) - planned UF1/UF2; open. Only 13 visual baselines (UF-8) - open.
- Zoom 200 %, forced colours, screen reader "not walked" in the last three walks (OI, SI1, SI1b) - open, process item.
- No empty/loading component (G13, R9) - open.

Content and wording
- Glossary leaks: "fusion" family in 6 keys, `history.merge` "Fusionner" (glossary lists fusionner as never to say; word is "intégrer") - proposed (item 3); open. "vue public" should be "vue publique" (UX-S11-4) - open (re-grep of `fr.json`: `present.showPublic` still "vue public").
- "Passer la main" ambiguous (UX-S7-7), "Envoi en cours" under a person (UX-SI1b-3) - open, wording settled in the C5S-UI session.
- Link level in people panel (W33 C5S-UI) - planned after PR1; design done: one dim line, words not glyphs, poor only emphasised.
- Presentation: footer shows keys to followers (S11-5), "Suivre/Revenir" two names (S11-6), black screen with no word (S11-7), 200 %/360px stripped slide (S11-8), laser icon same as cursor target (S11-9) - listed Défauts restants (severity 1-2); status not verified in code.

Diff, history, collaboration
- Diff "horrible" (W55), version tree ugly and not synced (W56), who-does-what attribution unclear (W57) - to place; reported via RP1 first; open.
- Collaboration cues (named carets, follow, jump) CP0-CP5 - PR1 in progress, rest after R1.
- Report tool (W51: flag in every window, Ctrl+Shift+S, freeze + point/draw, manager, zip export) - designed, RP1 to plan; open.
- Tabs for documents (N19, coherence §6.1), drawer with 3 destinations Rechercher / Relire / IA (§6.2), left panel two tabs Fichiers + Sommaire (§6.3), status bar max 4 visible items (§6.5) - decided, not done.
- Focus / typewriter / full-screen modes per person (N63) - decided, not done. Typora-like live markup rendering (N53/N64) - decided, not done.
- PDF-viewer behaviours for preview (N54: page-by-page, continuous, fit) - decided, not done.
- Unsaved/save indicator: done ("Non enregistré" in status bar + dot in header).

Done and valid (do not re-propose): honest sync words from a TLA+ model (SI1/SI1b), people dialog with roles, per-person colours Okabe-Ito, Adwaita popovers with arrow, native `<dialog>` modals with 44px close targets, text zoom per project (50-300 %), word/char stats popover, encoding menu, format bar with overflow menu, preview zoom/fit/jump, bidirectional source-preview jump, external-edit banner/toast, presentation mode with presenter/public windows, spreadsheet view, find bar inside the sheet, French+English catalogs complete (770 keys each, no diff), dark mode, reduced motion on existing animations, OI desktop-open toasts.

---------------------------------------------------------------------------------------------------------------------

## 4. Observations from code and screenshots (new or not found in the docs read)

Tags: [code] file:line, [img] screenshot. "NK" = not recorded as known in docs I read (I may have missed one).

Hierarchy, density, layout
1. Three stacked bars above the text: header 47px + format bar 46px + preview tool row ~40px, with the bar toolbars each carrying a bottom hairline. In [img] `gates/SI1/ux/joined-synced-light-full.png` that is ~130px of chrome before line 1, plus 36px status bar; the doc area starts at y=94 and the preview tools row sits level with the editor's first line. The header design (HB1a) only rebuilds the top bar; the format-bar and preview-tool rows are untouched in the plan. NK.
2. Sync state is shown twice at once: header word and status-bar word/dot ([img] joined-synced-light-full: "Synchronisé" top left, "2 personnes" dot bottom left). They use different vocabulary (state word vs people count). Header design moves the state into the pill but does not say whether the status-bar dot stays. NK (partly G2).
3. Two unrelated zooms visible at once: preview tool row "60 %" (fit toggle, `Preview.svelte`) and status-bar "100 %" (text zoom, `ZoomMenu`); the sheet has a third (`SheetStatus` "- 100 % +"). Labels "Agrandir/Réduire" are shared (`preview.zoomIn` and `zoom.in` both "Agrandir"). [img] joined-synced-light-full, files-readonly-dark. NK.
4. Text zoom keys (Ctrl+= / - / 0 and Ctrl+wheel) are implemented in `ui/history/HistoryBars.svelte:110-125`, a history component, not in the shell; shortcut labels in `ZoomMenu.svelte:8` are a hard-coded `KEYS` object. Works only because HistoryBars is always mounted. NK (architecture smell, affects discoverability).
5. Flat typographic hierarchy: title, sidebar heading, dialog h2/h3 are all 14.5px/700 (`HeaderBar:257`, `Sidebar:90`, `Preferences:127,141`). Body 14, notes 13, captions 12-12.5. The only larger heading is About (26px). Section headings in Preferences (h3 14.5/700, margin 20px 4px 8px) are the same weight as the dialog title. NK.
6. The editor text fills the full pane width with no measure limit (content padding `20px 28px 20px 0`, `editor/setup.ts:42`), line-height 1.75, mono 14px; on a wide window source lines run long next to a 55%-max preview. The source is code-like for Markdown prose too. NK (HIG/Apple-style "document" reading width is in ux-principles "Lecture longue" for preview only).
7. Sidebar header "Fichiers" is centred bold text in a 47px band with no control; the rest of the header band aligns with the main header only by height, there is no divider between sidebar header and sidebar body ([img] joined-synced-light-full). The sidebar has no empty/loading state (`Sidebar.svelte` renders an empty `<ul>`), no create/rename/delete/import (grep `newFile|renameFile|deleteFile|createFile` in src: none; fr.json has no "Nouveau", "Renommer", "Importer"). Known as W59/W59b for scale, but file creation basics are not listed as a defect. NK.
8. Dialog family differs: padding 18 (Share) vs `0 22px 22px` (Prefs), widths 480 vs 520, radii 12 vs 15, shadows 32/25 vs 40/30, title band 56px in Prefs vs inline in Share. 6 dialogs are distinct styles (Share, People, Prefs, About, Diff, Apply, Paste). NK.
9. Button style families: global `.fb` (12 files, 45 uses), `.btn` (15 files, 50), `.sug` (9 files, 13), `.tg`, plus local re-definitions: `SearchPanel.svelte:589-613` redefines `.btn`/`.btn.flat`/`.btn.suggested` with `cursor: pointer` (global uses `cursor: default`) and other radius literal; `.chip` is defined separately in SearchField, DiffInline, DiffView, HistoryDrawer; `.danger` only in PeopleDialog:345; PeopleStack has its own button. Header "Partager" is the only blue button; there is no destructive style in the global sheet. NK.
10. Icon system: single source `ui/icons/paths.ts` (67 entries, 16x16, stroke 1.6) - consistent. But rendered sizes mix 16 (default), 14, 13, 12 (`size={12}` 11x, `13` 6x, `14` 4x) and the format bar mixes SVG icons with typeset serif glyphs (B I U S ∑ at 17px serif, `BarButton.svelte:59`), and "branch" is used for Relire. [img] joined-synced-light-full shows glyph/icon mix in the format bar. N56 (one source) is met, uniform optical size is not. NK.
11. Hover/pressed states are one flat tint; no pressed state, no `:active` rule found (grep `:active` in src: not run, "not verified"); disabled = 45 % opacity (`.fb/.btn` 0.45, `.mi` 0.5, Prefs 0.45) - 2 values. NK.

Feedback and state
12. No transitions on any overlay (see §2): popovers, dialogs, toasts, banners, drawer and sidebar/preview toggles appear instantly; layout shifts when a banner appears (banners are in-flow between header and `<main>`, so the text jumps down by ~45px). `Banner.svelte` min-height 45px. NK (G14 is only about reduced motion).
13. Document title uses `● ` prefix for unsaved (`App.svelte:87`) while the in-app dot is a separate 8px circle in the header subtitle and a text "Non enregistré" in the status bar: three signals for one state. NK minor.
14. Preview empty/error states are plain text on the page background (`preview.fail.*`), no icon, no action (HIG placeholders: heading, guidance, action - cited in ux-ui-review but not applied). Known as G13/R9, listed for completeness.
15. In csv read-only view the info banner (`file.readOnly`: "Dans cette version, ce fichier s'affiche en lecture seule") is a full-width blue strip shown for every `.md`/`.csv` opened - [img] files-readonly-dark; the first file type a user double-clicks (md) is read-only (`gates/OI/UX.md` "Out of scope but seen"). Known as W-md-open-readonly accepted limit until G-0b; the wording "Dans cette version" is the only explanation.

Keyboard and access
16. Global shortcuts that exist (verified in code): F8 / Shift+F8 next/previous problem, Ctrl+S save, F5 present, Shift+F5 present from this slide, Ctrl+Alt+J jump to preview (`App.svelte:onKey`); Ctrl+Shift+T toggle format bar; Ctrl+= / - / 0 text zoom; format keys Ctrl+B/I/U/Shift+X/E/Shift+E/K/M, Ctrl+1..4, Ctrl+Alt+0 (`editor/format/keymap.ts`); Ctrl+Z (history). Menu items showing a shortcut: toolbar toggle, Present x2 only. No shortcut and none shown for: toggle sidebar, toggle preview, history, export, preferences, share. NK for the list; G5 knows the gap.
17. Sidebar rows, format-bar buttons and header buttons are each a tab stop (no roving tabindex); `role="toolbar"` on the format bar and the sheet toolbar with zero keydown handlers (`FormatBar.svelte`, `BarButton.svelte`: 0 `keydown`; APG toolbar expects arrow-key navigation). Sidebar is a nav+ul of buttons, no `role=tree` and no arrow navigation; Space opens without moving focus. DOM order puts the sidebar before the header, so Tab starts in the file list. UF-12 (APG contracts) plans this; the concrete instances are NK.
18. Tooltips: format bar has a rich tooltip (`.tipwrap/.tip`: label, syntax, shortcut), every header/preview/status control uses native `title` (49 `title=` in 18 files). Two tooltip mechanisms, one with shortcut hints, one without. G8 known; the two-mechanism split NK.
19. `aria-keyshortcuts` and `accesskey`: 0 uses (grep). No F10/main-menu key.
20. Target sizes below 24px (literal height/width 10-23px): tree chevron 12, popover arrow 14, notes/format-bar separators, sheet 20px elements (`Sheet.svelte:655`, `Grid.svelte:837`), status chips are 28px high (WCAG 2.5.8 minimum 24 OK; ux-principles rule 9 says 44px, met only in dialogs). Header controls 34px, status bar 28px, toast 32px: the "44px targets" rule is not what the shell does. NK as a measured gap vs rule 9.
21. Contrast: header `--fg-dim` ok per review. Computed here: light `--fg-dim` 55 % on `--sidebar-bg` = 4.52:1 (borderline); dark `--accent` as background with white text 1.69:1 in SearchPanel.suggested (unmounted); `--success` has no light/dark split (#2ec27e is 2.30:1 on white per review). Status-bar items use `opacity:.8` on `--fg` (`StatusBar.svelte:~199`), not recomputed.

Wording (French, `i18n/fr.json`; 770 keys, en parity exact)
22. Typography: 123 keys use the straight apostrophe, 1 uses the typographic one (`menu.about` "À propos d’Inkwell") - inconsistent in one menu. 70 keys put an ordinary space before ":" and 2 before "?"; 0 use a non-breaking (U+00A0/U+202F) space as French typography requires. 20 use "…", 0 use "...". 37 keys use « ». NK.
23. Vocabulary drift: close buttons are "Fermer" (4 keys), "Fermer les préférences", "Fermer la fenêtre À propos", "Fermer la recherche", "Fermer la vue public", and the toast's "Masquer la notification"; "Annuler" means cancel dialog (3), undo (3) and "Annuler cette modification" (history undo); `history.restore` "Rétablir" vs `history.revert` "Revenir à cette version" vs glossary (restore) - three verbs for going back; "Relire" (button/title) vs "Historique" (tab, `history.title`) vs "versions" for one drawer; "Enregistré/écrire sur le disque" in Prefs vs "Non enregistré" vs "PDF enregistré". Glossary forbids "fusionner" but `history.merge` = "Fusionner". "admin" (lowercase noun, English) in 8 keys and "Écriture/Lecture" role names vs sentences "vous ne pouvez plus modifier": the glossary lists write/read/admin so it is decided; mixed use of "écrire/modifier/écriture" is not. Partly known (jargon section of ux-ui-review); the verb-drift lists are NK.
24. Unlabelled-by-text controls: header icon-only buttons (sidebar, preview, present, history, menu) rely on `title` + aria-label; the header word set mixes states ("Seul", "Connexion…") and the status bar says "Seul"/"2 personnes".
25. Tone: sentences end with periods in banners but not in labels; "rien n'est perdu" is promised in 3 texts (`status.offline`, `prefs.save.note`, `history.revert.hint`) while C19 "honest states" says never to promise beyond what is acknowledged. Minor; decided in coherence §6.16 only as a principle.

Narrow width
26. HeaderBar has **no** media/container query (grep list of queries in src: StatusBar 900 and 640, PresenterView 760, App 560, tokens `prefers-contrast`); `.hb` is `1fr auto 1fr` with nowrap children (`HeaderBar.svelte:220-253`); the only adaptive behaviours: status bar hides page/lang under 640px and hides the problems text under 900px (viewport media, not container), format bar overflows into "⋯", preview covers source under 560px. Sidebar stays 264px at any width. All known as G11/SI1-E/HB1b; the concrete "only 4 breakpoints, none for header" count is a measured fact.
27. Native min width is 720px, the 360px behaviours documented in reviews were produced in the Chromium harness; a real window cannot be narrower than 720 unless `MinWidth` changes. Which of the 360px work is therefore user-visible on desktop is not stated anywhere I read. Not verified whether Wails enforces MinWidth on the user's WM.

Motion
28. Only decorative animation present: pulsing sync dot and preview shimmer (infinite). `docs/plans/2026-11-09-ux-research.md` says T-11 forbids infinite animation and "not recommended: skeleton shimmer", but `Preview.svelte:299` ships one. Contradiction with a recorded decision. NK.

---------------------------------------------------------------------------------------------------------------------

## 5. What does not exist (each verified by grep over `frontend/src` unless stated)

| Item | Verdict | Evidence |
|---|---|---|
| Onboarding / welcome / first-run tour | absent | no `welcome|onboard|bienvenue` UI key in fr.json (matches in `net/admission.ts` are protocol terms); planned W40 R4 |
| Command palette / command registry | absent | no `Palette|CommandRegistry`; keys handled ad hoc in `App.svelte onKey`; planned R3 |
| Shortcuts window / cheat sheet | absent | no `ShortcutsDialog|Raccourcis clavier` |
| Focus / zen / typewriter mode | absent | no `typewriter|zenMode|focusMode|machine à écrire` (N63 decided, not built) |
| Outline / document structure panel | absent | only the Typst `#outline()` inserter in InsertMenu; "Sommaire" tab is spec only (coherence §6.3) |
| Find in the text editor (Ctrl+F) and replace | **built but not wired** | `editor/search/**` (keymap Mod-f, Mod-h, Mod-Shift-f, F3, F4) and `ui/search/SearchPanel.svelte` (769 lines) have no consumer in `App.svelte`/`Editor.svelte`/`setup.ts` (grep `SearchPanel|searchKeymap|openSearchPanel`: only inside their own folders); `setup.ts` uses `defaultKeymap` only. Only the spreadsheet has a working find bar (`FindBar.svelte`). Not verified: whether tinymist/CodeMirror default maps give a native Ctrl+F in WebKitGTK (CodeMirror's `defaultKeymap` does not include search). Search i18n (56 keys) exists. |
| Zoom | exists (text zoom 50-300 % per project, status bar menu; preview zoom/fit; sheet zoom) | `store/zoom.svelte.ts`, `ZoomMenu.svelte`, `Preview.svelte` |
| Word / character count | exists | `StatsPopover.svelte` (words, characters, sentences, paragraphs, minutes) |
| Export dialog / options | absent | single menu item "Exporter en PDF…" -> native save dialog (`preview/actions.ts`); no page range/format/options; no print (`window.print|Imprimer`: none) |
| Theme switch (light/dark/system) and font/size/language prefs | absent in UI | Preferences has only 2 groups; no `setTheme|prefs.theme`; language from `navigator.language` (`i18n/index.ts:9`), no switch |
| Project name / rename / Projects screen | absent | no `projectTitle|renameProject`; title constant |
| Create / rename / delete / import file in sidebar, drag-and-drop of files/images | absent | no matching functions; no `ondrop`/`DataTransfer`/`clipboardData` handling outside CodeMirror's `dropCursor` |
| Sidebar context menu | absent | `oncontextmenu` only in the sheet grid |
| Tabs / multiple open documents | absent | no `role="tab"` outside the history drawer; one document at a time (open notices say so) |
| Undo/redo buttons in the text UI | absent | only the sheet toolbar has them; Ctrl+Z in editor |
| Spell-check / grammar | absent in editor | `spellcheck` appears only on inputs |
| Tooltip component | absent | `title=` x49 + the format-bar's own `.tip` |
| `StatusPage`/empty-state component, spinner | absent | preview shimmer only |
| Dialog/popover/toast transitions | absent | see §2 |
| Window controls / CSD / window drag region | absent | no `Frameless`, no `wails-draggable` |
| Report tool / flag | absent | RP1 design only |
| Comments, mentions, Activity feed | stubs | drawer tabs show "pas encore disponibles" / "Rien à signaler" |
| Forced-colours / `prefers-contrast` beyond tokens | partial | one `prefers-contrast` block, no `forced-colors` |
| Mobile/touch behaviour | not verified (desktop only; W34 is a study) |
| Sync-state popover, link-quality display | absent | `links.of()` has no caller (C5S-UI S4); `PeopleDialog` shows only online/offline |
| Per-person caret follow / jump | absent (CP3, after R1) |

---------------------------------------------------------------------------------------------------------------------

## 6. Not verified / caveats

- I did not run the app: animation behaviour, real fonts, WebKitGTK rendering, the dark-mode look of the preview shimmer, `:active` states, native-window minimum width enforcement.
- Counts are static greps (a `<button` inside `{#each}` counts once). `fr.json` string statistics were computed with a script over the 770 keys.
- `docs/plans/2026-11-14-ux-ui-study.md` (referenced by W63 in exigences.md:82) does not exist yet in the repo: this inventory is its input.
- Walkthrough docs `docs/ux/walkthrough-2026-10-26-*.md` and the S7/S11 defects were read via their headings only; their fix status is as the ux-ui-review states it, not re-verified in code except `present.showPublic` (still "vue public").
- Screenshots are Chromium harness captures; light/dark pairs viewed only for the people dialog and header.
