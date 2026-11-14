# UX/UI improvement study (W63): apparent simplicity, maximum usefulness, beauty

Status: **decided by the owner on 2026-11-14: every proposal is kept except P26** (short verification code, not kept).
Each kept proposal joins an existing block
(HB1a/HB1b, UF1/UF2, « Écrire et retrouver », « Première minute et secours », navigation in the project), never an isolated
task. Request: W63 (`docs/exigences.md`), which extends W40. Earlier work this builds on and does not repeat:
`docs/plans/2026-11-09-ux-research.md` (R1–R14), `docs/reviews/2026-11-13-ux-ui-review.md`,
`docs/plans/2026-11-13-report-and-header-design.md` (header variant B, chosen).

Brief (owner, 2026-11-14): « beau, simple, et pourtant très fonctionnel et complet » ; « simplicité apparente, utilité
maximale » ; the spirit of Apple products, in the Adwaita style, following the GNOME guide.

## 1. Method and limits

Three inputs, each kept as a dated reference under `docs/ux/sources/`:

| Input | File | Limits |
|---|---|---|
| Inventory of the current interface (code, tokens, 6 screenshots) | `2026-11-14-inventory.md` | static reading; the application was not run; counts are greps |
| GNOME HIG, libadwaita 1.10 (GNOME 51), exemplary apps, GNOME app criteria | `2026-11-14-gnome.md` | quotes went through a page summariser; the header-bar button rule (P11) was re-read word for word; only Text Editor and Files were read from their UI sources; designers' blogs not obtained |
| Apple HIG, NN/g, WCAG 2.2, writing and collaboration tools, local-first research | `2026-11-14-craft.md` | ISO 9241-110, Bringhurst's book, some WCAG pages and Pages/Notes activity pages not read: marked unverified there |

Every proposal states the fact observed in inkwell, the change, the reason and its source. A source marked « (unverified) »
was not read on a fetched page and must be re-read before the proposal becomes a rule.

## 2. The idea in one picture

Apple's and GNOME's guides agree on the mechanism behind « simple yet complete »: **progressive disclosure** — show what
is needed now, keep the rest one gesture away ([NN/g](https://www.nngroup.com/articles/progressive-disclosure/); GNOME HIG
principle 2 « reveal complexity gradually », <https://developer.gnome.org/hig/principles.html>). For inkwell that gives three
levels, and each function gets exactly one:

1. **Always visible**: the text, one bar, one state. Nothing else by default.
2. **One gesture away**: files (F9), the preview, people, versions, search — panes and popovers that slide in and out.
3. **One shortcut away**: everything else, through the command palette, which also teaches the shortcuts.

Beauty then comes from regularity, not decoration: one type scale, one spacing grid, one radius family, one motion curve,
colour reserved for the accent, the states and the people.

## 3. What the inventory measured

| Fact | Evidence |
|---|---|
| ~130 px of bars above the first line (header 47, format bar 46, preview tools ~40), plus a 36 px status bar | `App.svelte:370-480`, `gates/SI1/ux/joined-synced-light-full.png` |
| 13 font sizes; window title, sidebar title and dialog titles all `700 14.5px`; no type-scale token | `HeaderBar.svelte:257`, `Sidebar.svelte:90`, `Preferences.svelte:127` |
| Spacing tokens used in 48 of 271 declarations (18 %); 709 `px` literals; 53 raw colours in 18 files | greps over `frontend/src` |
| 7 dialog variants (two radii, two shadows, two paddings); button families redefined locally | `ShareDialog`, `Preferences`, `SearchPanel.svelte:589-613` |
| No enter or exit transition on any dialog, popover, toast, banner or pane; a banner pushes the text down by ~45 px | only 7 `transition`/`animation` hits; `Banner.svelte` |
| Two infinite animations (sync dot pulse, preview shimmer) against the recorded « no skeleton shimmer » | `HeaderBar.svelte:294`, `Preview.svelte:299` |
| Sync state shown twice with two vocabularies; three zooms visible; three signals for « unsaved » | header + status bar; `Preview`, `ZoomMenu`, `SheetStatus` |
| Find and replace is written and tested (769 lines) but mounted nowhere: Ctrl+F does nothing in the text | `ui/search/SearchPanel.svelte`, `editor/search/**`; no consumer in `App.svelte`, `Editor.svelte`, `setup.ts` |
| Adwaita Sans and Adwaita Mono are named but not shipped (no `@font-face`, CSP `font-src 'self'`) | `theme/tokens.css`, `index.html` |
| Shortcuts exist for formatting only; none for sidebar, preview, versions, share, preferences; two use Alt | `App.svelte onKey`, `editor/format/keymap.ts` |
| Native minimum width 720 px: a half-tiled window on a 1366 px screen (683 px) is below it, and the 360 px work was only seen in the Chromium harness | `cmd/inkwell/main.go:163-167` |
| French strings: 123 straight apostrophes for 1 typographic; no no-break space before « : » and « ? »; three verbs for « go back to a version »; three names for one drawer | `i18n/fr.json` |

## 4. Proposals

Effort: S (hours), M (days), L (a block). « Home » is the existing block that would carry it.

### A. Beauty: one visual system

| # | Change | Why (source) | Effort, home |
|---|---|---|---|
| P1 | Five text styles instead of 13 sizes, taken from libadwaita: title (136 %, 800), subtitle (118 %, 700), heading (100 %, 700), body (100 %, 400, line 1.4), caption (82 %). Dialog titles become real titles | HIG typography: « minimize the number of font sizes and weights », <https://developer.gnome.org/hig/guidelines/typography.html>; values from libadwaita `_labels.scss`. A flat hierarchy is what makes the dialogs look unfinished | M, UF1 |
| P2 | Spacing on Adwaita's grid of 6 (3, 6, 12, 18, 24) instead of today's 4/8/12/18 tokens; 6 px is already the most frequent literal (55 uses). Enforced by the planned UF-4 lint | libadwaita SCSS (6 px spacing, 12 and 24 px margins, <https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.10/adaptive-layouts.html>); regular rhythm reads as care | M, UF1 |
| P3 | One radius family (15 window, dialog, popover; 12 card; 9 control), one overlay shadow, one `Dialog` and one `Button` component replacing the 7 variants | libadwaita SCSS values; HIG UI styling: « Keep custom styling to a minimum » | M, UF1 |
| P4 | Ship Adwaita Sans and Adwaita Mono in the binary so every machine renders the same letters | GNOME 48 made them the system fonts (<https://release.gnome.org/48/developers/>); today the look depends on what is installed, which breaks « self-contained ». Licence to confirm before shipping (expected SIL OFL) | S, UF1 |
| P5 | Motion with a purpose: one curve (200 ms, `cubic-bezier(.25,.46,.45,.94)`), overlays fade and rise in 200 ms and leave in 150 ms, panes slide without bounce, a banner never makes the text jump; reduced motion keeps a short fade; the two infinite animations go | HIG adaptive: « Widgets should not jump around or vanish without an animation »; NN/g 100–500 ms, exit faster than entry (<https://www.nngroup.com/articles/animation-duration/>); libadwaita uses 200 ms and critically damped springs; GNOME 50 reduced motion asks for gentler alternatives, not none | M, UF1 |
| P6 | Icons at one optical size (16 px, 2 px stroke), the format bar's serif letters B I U S drawn as icons like the rest | HIG UI icons: 16 × 16, « Avoid other sizes to prevent fuzzy rendering », 2 px strokes, <https://developer.gnome.org/hig/guidelines/ui-icons.html>; today 12, 13, 14 and 16 px mixed at 1.6 px | S, UF1 |
| P7 | Preferences gains « Apparence » (Clair, Sombre, Système), language and text size, laid out as boxed lists | HIG UI styling: per-app style preferences « are mainly useful for text editors and apps used for long periods »; today 2 groups only, no theme or language choice | M, « Première minute » |
| P8 | Follow the system accent colour | libadwaita: apps follow the system accent by default (GNOME 47+). Later: needs the desktop portal read from Go, and a check against the 8 people colours | M, after R1 |

### B. Apparent simplicity: less chrome, one place per thing

| # | Change | Why (source) | Effort, home |
|---|---|---|---|
| P9 | One bar above the text by default. The preview's tool row becomes a small pill laid over the bottom of the preview, shown on pointer move or focus; the format bar stays (owner requirement) and keeps its toggle. Chrome above line 1: 130 → 93 px, 47 px with the format bar hidden | Apple toolbars: reduce backgrounds, every item deliberate; HIG overlaid controls (<https://developer.gnome.org/hig/patterns/controls/overlaid.html>); the header redesign (HB1a) treats only the first of the three bars | M, HB1a |
| P10 | Focus mode: text only, bars return on pointer move to the top edge or Esc (already decided as N63: schedule it) | iA Writer, Apostrophe focus mode; Apple: hide chrome on demand « with a reliable way back » | M, « Écrire et retrouver » |
| P11 | « Partager » becomes a flat button with icon and label instead of the blue button. This corrects the 2026-11-13 decision, whose citation was inexact | HIG header bars, re-read word for word: buttons with « the suggested and destructive action styles » « should generally be avoided for primary window header bars, since it leads to a complex and inconsistent visual appearance » (<https://developer.gnome.org/hig/patterns/containers/header-bars.html>). Apple still wants Share easy to find in the toolbar: icon + label keeps that | S, HB1a |
| P12 | One state, one place: the sync state lives only in the central pill; the status bar loses its people dot and keeps at most four items (problems, position, words, language); « unsaved » has one signal | Apple feedback: status in place, passive; Ink & Switch: peer-to-peer is « never fully online or offline » (<https://www.inkandswitch.com/essay/local-first/>); coherence §6.5 already caps the status bar at four | S, HB1a |
| P13 | One zoom: a « − 100 % + » row in the main menu for the text (as GNOME Text Editor), the preview fits its width by default and keeps its overlay pill | Text Editor's primary menu read from its source; three zooms with the same labels today | S, HB1a |
| P14 | Main menu in the HIG order, ending with Préférences, Raccourcis clavier, À propos d'Inkwell; F10 opens it | HIG menus, <https://developer.gnome.org/hig/patterns/controls/menus.html> | S, HB1a |
| P15 | Reading width: the text column is capped near 80–90 characters and centred when the pane is wider | Butterick: 45–90 characters (<https://practicaltypography.com/line-length.html>); WCAG 1.4.8: no more than 80; today lines run the full pane | S, UF1 |
| P16 | Sidebar by the numbers: 25 % of the width between 180 and 280 px, neutral selection, 36 px rows, F9 toggles it, it overlays the text below the breakpoint | libadwaita split views (25 %, 180–280); HIG utility panes and keyboard reference (F9) | S, HB1b |
| P17 | Make narrow real: minimum width 360 px instead of 720, so half-tiling works and HB1b's narrow layout is reachable | HIG adaptive: support 1024 × 600, phone-capable apps down to 360; 1366 / 2 = 683 < 720 | S, HB1b |
| P18 | One window bar: draw the header as the title bar (client-side decorations, N49) together with HB1a rather than later; today the system title bar sits above the header | every GNOME app; it removes one more bar (~37 px) and is the largest single step toward « looks native » | L, HB1a + S13 |

### C. Maximum usefulness: nothing missing, nothing in the way

| # | Change | Why (source) | Effort, home |
|---|---|---|---|
| P19 | Connect find and replace: Ctrl+F, Ctrl+H, Ctrl+G; a search bar slides under the header; fix its dark-mode button (1.69:1) and its private button styles | HIG search and standard shortcuts (<https://developer.gnome.org/hig/reference/keyboard.html>); the code exists and is tested: the highest value for the lowest cost in this study | S, « Écrire et retrouver » |
| P20 | Command palette and shortcuts window from one command registry (already planned as R3): the registry also feeds menu labels and tooltips, so a shortcut is written once and shown everywhere | Linear, Zed, VS Code; libadwaita `AdwShortcutsDialog` (1.8), opened by Ctrl+?; coherence C14 | L, « Écrire et retrouver » |
| P21 | Standard shortcuts for the frequent gestures: F9 files, F10 menu, Ctrl+, preferences, Ctrl+? shortcuts, and replacements for the two Alt chords | HIG keyboard: use standard shortcuts, no Alt in shortcuts (it conflicts with access keys) | S, with P20 |
| P22 | One tooltip component: name and shortcut, on every icon button | HIG: « Primary window header bar controls should all have tooltips »; GNOME app criteria: image buttons have a tooltip; today 49 native `title=` and one rich tooltip | M, UF1 |
| P23 | Empty and error states as a component: heading, one sentence, one action (empty sidebar, no preview, failed export with « Voir le problème ») | HIG placeholders, <https://developer.gnome.org/hig/patterns/feedback/placeholders.html>; Apple: errors explain what to do next | M, « Première minute » |
| P24 | Undo instead of asking: reversible actions act at once and offer « Annuler » in a toast (5 s, one button); alerts only for irreversible loss | HIG principle 4: « Allow destructive actions to be undone instead of asking for confirmation »; toasts default to 5 s (libadwaita `adw-toast.c`); today 7 s and a confirmation on a reversible action | S, BannerHost/ToastHost |
| P25 | Collaboration in two controls: the share popover opens with one sentence saying who can do what, then the invitation; the people popover lists people, then « Gérer »; clicking a face follows that person until the next key press | Apple « Collaboration and sharing » (<https://developer.apple.com/design/human-interface-guidelines/collaboration-and-sharing>); Zed's follow mode; CP3 and UF2 already hold parts of this | M, UF2 + CP3 |
| P26 | Short verification code when someone joins, the long fingerprint kept as a detail | Turner et al., ARES 2023, 162 participants: missed near-matches rise from 13.2 % (one line) to 43.5 % (three lines) | M, UF2 |
| P27 | Direct manipulation of files: drop an image on the text to insert it, drop files on the sidebar to add them, context menu on a file | Apple drag and drop; HIG pointer and touch (context menu as the secondary action); W62 covers the file operations, not the drop on the text | M, navigation in the project |
| P28 | Waiting rule: nothing under 1 s, a quiet « Mise à jour… » from 1 to 3 s, a spinner beyond, a progress bar beyond 30 s; never a shimmer | NN/g limits 0.1 s, 1 s, 10 s (<https://www.nngroup.com/articles/response-times-3-important-limits/>); HIG spinners: indicate progress beyond about 3 s, a bar beyond 30 s | S, UF1 |

### D. Words and proof

| # | Change | Why (source) | Effort, home |
|---|---|---|---|
| P29 | French typography in the interface, checked by a test: typographic apostrophe, narrow no-break space before « : » « ? » « ! », « … » | HIG typography lists the characters to use; today 123 keys against 1 | S, i18n tests |
| P30 | One word per thing: one verb for going back to a version, one name for the versions drawer, « Fermer » everywhere, « intégrer » instead of « fusionner » | `docs/glossary.md`; HIG writing style: short, consistent labels | S, i18n tests |
| P31 | Rule 9 of `docs/ux-principles.md` says 44 px targets, the shell uses 34 px, which is libadwaita's control height: write the rule as it should be (34 px controls, never under 24 px, 44 px for touch layouts) | libadwaita `_buttons.scss`; WCAG 2.2 criterion 2.5.8 (24 px) | S, UF1 |
| P32 | A release checklist taken from GNOME's app criteria, added to the `ux-walkthrough` skill: keyboard only, high contrast, large text, screen reader, a tooltip on every icon button, no stray white surface in dark | <https://gitlab.gnome.org/Teams/Releng/AppOrganization/-/raw/main/AppCriteria.md> | S, tooling |

## 5. Suggested order

1. **Now, small and visible** (hours each, no new design): P19 find and replace, P24 undo toasts, P29–P31 words and rule,
   P14 and P21 menu order and shortcuts.
2. **With the header block HB1a/HB1b**, since they touch the same files: P9, P11–P13, P16–P18.
3. **With UF1 (tokens and components)**: P1–P6, P15, P22, P28 — this is where « beautiful » is won, and the lint keeps it.
4. **With the blocks already planned after R1**: P7, P10, P20, P23, P25–P27; P8 last.

## 6. Not proposed

A floating format toolbar replacing the bar (the owner asked for a per-format bar; hiding it would cost discoverability),
a second accent or gradients (HIG: classes and colours only for their purpose), skeleton screens (recorded decision), an
onboarding tour (Apple and GNOME both prefer a usable first screen with a good empty state: P23).

## 7. Owner's remarks on the mock-ups

- 2026-11-14: the « Texte / Les deux / Aperçu » switcher must keep one line per label and the height of the other header
  controls (34 px); a label that wraps (« Les deux » on two lines in the first mock-up) is a defect. HB1a carries this as
  a test: the switcher's height equals the header buttons' at every supported width.
