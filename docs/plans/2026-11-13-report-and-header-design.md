# Report tool and header bar B — design (W51, W52, W53, W57)

Status: design from the brainstorming of 2026-11-13 with the user; to plan (`superpowers:writing-plans`) as two blocks,
**RP1** (report tool) then **HB1** (header bar B, absorbs SI1c). Mockups: `.superpowers/brainstorm/*/content/adwaita-v2.html`
(local, not versioned; variant B chosen).

## 1. Intent and why

The user meets many defects (overlapping buttons, preview not the height of the writing area, ugly diff, history tree
neither pretty nor synced, no clear "who does what") and has no cheap way to show them: today a defect reaches Claude only
through a 20-minute live session or a sentence in chat, and the velocity review (`docs/reviews/2026-11-13-velocity-review.md`
§3, change 3) measured 18 to 35 h of waiting per UI block on that session. The report tool turns every moment of use into
a precise, reproducible defect report. The header bar is the most visible "not polished" surface, and the place where the
report flag lives.

## 2. Part 1 — report tool (RP1)

### Decided with the user
- A flag button « Signaler » is always visible in the header bar, in every window.
- A **keyboard shortcut** starts reporting without closing anything, so popovers, menus, dialogs and toasts can be shown
  while open (the user: "s'il y a des pop-up… il faut que ça marche aussi"). Full-window dialogs also carry the flag.
- Reporting mode: the screen is frozen as it is; the person points at **one or several elements** (hover outline, click)
  and/or draws **one or several rectangles**; each mark is numbered (①, ②…).
- A screenshot is taken with the marks drawn on it.
- A **manager** (a GNOME window: list on the left, detail on the right) where the person can edit, add information by hand,
  create a report without pointing, and delete reports. Kind: « Défaut » or « Amélioration ».
- **Per person, local only**: reports stay on that person's machine; nothing is shared with the project's other people or
  sent anywhere. The person alone decides to **export**.
- Export: one file the person hands to Claude, who files each item in `docs/exigences.md`.

### Claude's calls (to check)
- Shortcut `Ctrl+Shift+S` (« signaler »), registered at the window level so it works over any popover; checked against
  existing shortcuts (none may move).
- Each report also records, automatically: screen/view, open file path (not its content), window size, theme, app version
  and commit, sync state, and for each pointed element a stable description (role, accessible name, i18n key when known,
  CSS selector, bounding box). This is what makes a report reproducible.
- States: open / exported / fixed (the person sets "fixed"); export can take all reports or a selection.
- Export format: a `.zip` with `reports.json` (versioned schema), `reports.md` (readable) and one PNG per report.
  A small importer (`scripts/import-reports.py`) turns it into `docs/exigences.md` rows.
- Privacy: screenshots may show document text; the manager shows each image before export and lets the person crop or
  drop it; export never includes the document itself.
- Storage: one directory per person under the app's data directory (`internal/store` conventions, atomic writes),
  bounded (e.g. 500 reports, 20 MB of images, oldest-first warning), never in the project folder (would sync).
- Capture technique, to settle by a spike at plan time: (a) native WebKitGTK `webkit_web_view_get_snapshot` through
  cgo on Linux (exact pixels, includes canvases and popovers) with the existing per-OS build split; (b) DOM
  serialization in the webview (modern-screenshot / snapDOM, SVG `foreignObject`) as fallback, known weak on canvases
  and some CSS. Recommendation: (a) on Linux, (b) elsewhere until measured.
- Integration: the flag sits in the header bar; the manager opens from the main menu (« Signalements ») and from the flag
  badge; no new component family beyond the Adwaita patterns already in the app (dialogs, list rows, toasts).

### Non-goals / anti-goals
- No upload, no telemetry, no account, no automatic sending: anything that leaves the machine without the person's export
  is a failure even if it works.
- Not a chat or a shared tracker between people.
- Must not change the app's behaviour while reporting (no state mutation, no closed popover, no lost typing).

## 3. Part 2 — header bar B (HB1, absorbs SI1c)

### Decided with the user
- GNOME/Adwaita style, "simple appearance answering every function elegantly"; variant **B** of the mockups.
- Two-pane window like Files / Text Editor: the sidebar has its own header bar (project name, « + », search); below it
  files, then the latest versions with their author.
- Content header bar (Adwaita metrics: 47 px, flat 34 px buttons), left to right: show/hide sidebar, versions, export;
  a **central pill** with file name, sync state and the faces of present people (one click opens "who does what":
  « Alice · vous — écrit dans main.typ, ligne 6 », « Bob — écrit ligne 3 · bon lien »); a segmented « Texte / Les deux /
  Aperçu »; the report flag; the main menu.
- Each remote caret carries a name flag in the text (W57, "who writes now").
- The preview **page itself** (the white sheet, not only the pane) is sized to the writing area by default, so text and
  page are balanced (W54, user 2026-11-13: « la feuille blanche fasse la même taille que la zone d'écriture »).
- Narrow window (360 px): title + state on two lines, flag and menu only; views move to a bottom view-switcher bar, as
  adaptive GNOME apps do. Nothing may overlap (W53).

- Inside the window, the user likes the JetBrains « islands » look (photo of GoLand, 2026-11-13): each pane (sidebar,
  editor, preview, bottom panel) is its own card with slightly rounded corners, separated by a thin gap of the window
  background, rather than panes joined by hairlines. Kept within Adwaita: card radius and colours from the Adwaita
  tokens, no new palette.

- « Partager » (invite) stays a visible suggested (blue) button at the end of the bar (Claude's call, user: « trouve le
  meilleur design »; `docs/autonomous-decisions.md`).

### Claude's calls (to check)
- HB1 replaces SI1c's header layout scope: SI1c's other items (one steady wording while two people type, « Seul » when my
  text has not left) move into HB1; `gates/SI1c.toml` (never frozen) is replaced by `gates/HB1.toml`. The UX review's
  finding that SI1c's 360 px criterion was unsatisfiable with a 264 px sidebar is solved by the narrow layout (sidebar
  starts closed below the adaptive breakpoint).
- The pill reuses the existing sync indicator and people stack logic (`HeaderBar.svelte`, `PeopleStack.svelte`,
  `PeopleDialog.svelte`); PR1 (presence correctness) and C5S-UI (link level) plug into the same popover.
- Breakpoints follow libadwaita's adaptive guidance (`AdwBreakpoint`), values measured on our layout.

### Non-goals
- No new visual language: Adwaita tokens already in `frontend/src` only; no dark/light divergence.
- The diff and history tree redesign (W55, W56) are separate blocks, reported through RP1 first.

## 4. Order
RP1 first (it accelerates every other UI fix), then HB1, then the defects RP1 collects (diff, history tree, preview).
PR1 continues in parallel (implemented in a worktree, gate criteria running; not on `main`).

Base trees (Claude's call): RP1 starts from `main` once SI1b is committed and touches `HeaderBar.svelte` only to add the
flag (interim placement); HB1 starts once SI1b and PR1 are on `main`, and C5S-UI's link line moves into HB1's
"who does what" popover (its plan is amended then).

## 4b. Builder check (2026-11-13), Claude's calls
- HB1 is split to respect the standard cap: **HB1a** (header bar B and the pill, carrying SI1c's rules SI1-C, D, F
  unchanged) and **HB1b** (narrow layout: two-line title, bottom view switcher, replacing SI1-E's "move to the main
  menu below 520 px").
- Long states (« Envoi en cours (n) », « Lien lent », « Modifications non envoyées ») show in full in the pill; on the
  narrow two-line title the second line ellipsizes and the full text is in the pill's popover and the accessible name.
- The sidebar's « + », search and versions list are layout slots in HB1; their functions belong to later blocks
  (file creation with G-0b, search = the existing find panel, versions list with the history redesign W56).
- « In every window » means the main window and its dialogs and popovers, presenter mode included through the shortcut;
  the public audience window (projected) carries no flag.

## 5. Left to the builder
Component split, file names, test layers (cheapest first, per CLAUDE.md), exact breakpoint values, icon names from the
Adwaita symbolic set, schema field names.

## 6. Sources
- GNOME HIG, header bars and adaptive layouts: https://developer.gnome.org/hig/patterns/containers/header-bars.html,
  https://developer.gnome.org/hig/guidelines/adaptive.html
- WebKitGTK snapshot API: https://webkitgtk.org/reference/webkitgtk/stable/method.WebView.get_snapshot.html
- DOM capture approaches (SVG foreignObject, limits): https://engineering.monday.com/?p=11028
- Visual feedback tools for agents (element picker, selector + bounding box + screenshot, Markdown export, overlay in
  shadow DOM): https://spaceparrots.de/en/tools/nit, https://www.producthunt.com/p/agentecho/agentecho
- Wails v3 window API (no capture API): https://v3.wails.io/reference/window/
