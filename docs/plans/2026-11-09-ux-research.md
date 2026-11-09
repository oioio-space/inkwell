# UX research: state of the art, formal methods, gaps (2026-11-09)

User request (2026-11-09): the interface must be at the best level: beautiful, practical, simple and friendly,
following Adwaita; simple to look at while offering advanced features, never restricting the person; grounded in
current practice and in formal methods. This note builds on what is already decided: `docs/ux-principles.md`,
`docs/design/spec.md` (N17), `docs/plans/2026-11-09-ui-formal.md` (UF-1 to UF-8) and
`docs/engineering-principles.md` §5. It does not repeat them. It adds sources, a verdict on each formal method, the
gaps between the spec and the code, and ranked recommendations.

Method: web sources fetched or searched on 2026-11-09 (sources marked "(search)" were seen as search results, not
fetched). Gaps come from reading `frontend/src/App.svelte`, `frontend/src/ui/**` and `frontend/src/theme/*.css` in
the working tree of 2026-11-09 (uncommitted SI1 changes included).

---

## 1. State of the art

### 1.1 GNOME HIG and libadwaita (1.8, and 1.9 since March 2026)

- **Breakpoints.** `AdwBreakpoint` (since 1.4) sits on the window. `AdwBreakpointBin` handles one adaptive section.
  Conditions are written in `sp` so that they follow the Large Text setting. View switchers collapse at 400, 450 or
  550 sp for 2, 3 or 4 pages. The minimum window is 360 × 200 px.
  https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/migrating-to-breakpoints.html
- **Split views.** `AdwNavigationSplitView` is for sidebar + content and collapses into a stack.
  `AdwOverlaySplitView` is for a utility pane (`sidebar`, `show-sidebar`, `pin-sidebar`). `AdwBottomSheet` replaces
  bottom flaps. Same URL.
- **Sidebar.** There is no `AdwSidebar` in 1.8 (https://nyaa.place/blog/libadwaita-1-8). It arrived in **1.9**, with
  `AdwSidebarSection` and `AdwViewSwitcherSidebar`, as the adaptive replacement for `GtkStackSidebar`
  (https://nyaa.place/blog/libadwaita-1-9, https://www.phoronix.com/news/GNOME-AdwSidebar (search)).
- **Reduced motion.** In 1.9, most animated widgets respect reduced motion by **replacing slides and scales with
  crossfades** instead of removing all motion (https://nyaa.place/blog/libadwaita-1-9). This confirms the spec's
  `--duration-fade` (§2.8, T-11).
- **1.8 additions.** `AdwShortcutsDialog` replaces `GtkShortcutsWindow`. An app that ships a shortcuts resource gets an
  `app.shortcuts` action, with `AdwShortcutLabel` keycaps. `.document` uses a separate document font (Adwaita Sans
  12 pt) with a taller line height, and `.body` also gains line height. One CSS file covers light/dark and high
  contrast through media queries. Preferences search covers `AdwPreferencesRow` items only. Dialog shadows were
  reduced. https://nyaa.place/blog/libadwaita-1-8
- **Toasts.** A toast is for an event, mostly the result of the person's own action. The main use is undo after a
  destructive action. Bottom centre, short title. Toasts are not for ongoing state, and carry no button by default.
  https://developer.gnome.org/hig/patterns/feedback/toasts.html. The API has a `timeout`, a `priority` (HIGH for
  undo), one action, and a close button. A visible toast is updated by re-adding it.
  https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.Toast.html
- **Banners.** For a state that lasts or is tied to a place, e.g. "Working offline" or read-only. A banner never
  auto-hides and has a short, factual title. https://developer.gnome.org/hig/patterns/feedback/banners.html
- **Empty states.** `AdwStatusPage`. The illustrated style is for an empty main view at first use, the symbolic
  style for places that are usually full (search, folders). The heading always points to a next step.
  https://developer.gnome.org/hig/patterns/feedback/placeholders.html
- **Spinners.** Only for operations of about 3 s or more, after a delay when the duration varies. A progress bar if
  the task may exceed 30 s. One spinner at a time, with a label.
  https://developer.gnome.org/hig/patterns/feedback/spinners.html
- **Principles.** Design for people, make it simple (one purpose per view, reveal complexity gradually), reduce
  effort, be considerate (undo rather than confirm). https://developer.gnome.org/hig/principles.html
- **GNOME Text Editor**, the closest native analogue, never asks to save on quit. It restores the session and its
  drafts, always restores after a crash, and autosaves only when something changed. A user-facing switch that lost
  text was reverted as a data-loss bug.
  https://gitlab.gnome.org/GNOME/gnome-text-editor/-/issues/403,
  https://lists.gnome.org/archives/commits-list/2022-June/msg06464.html,
  https://mail.gnome.org/archives/commits-list/2021-July/msg07556.html

### 1.2 Collaborative and local-first editors

- **Offline honesty.** The local-first essay asks that apps "function normally regardless of network status". It
  says a peer-to-peer system is never fully online or offline, and leaves *how to show it* open.
  https://www.inkandswitch.com/essay/local-first/
- **Figma** shows an offline icon with a tooltip and a banner. "Tap to review" lists files with unsynced changes
  (https://help.figma.com/hc/en-us/articles/360040328553 (search)). Figma has no full offline mode, and users report
  false "offline" states (https://forum.figma.com/t/any-fixes-for-why-figma-keeps-showing-me-as-offline/51472
  (search)). A status word that lies costs trust, which is the point of inkwell's H-HON rule and SI1.
- **Presence.** In Figma, clicking an avatar follows that person, and "Spotlight me" asks the others to follow. "We
  won't tell someone if you follow them." There have been long-running requests for an invisible mode and for an
  opt-out of observation (https://help.figma.com/hc/en-us/articles/360040322673,
  https://forum.figma.com/t/disable-observe-mode/647?page=4). In Zed, joining a project starts by following the
  inviter. You can follow in one pane and roam in another. Channels have Guest, Member and Admin roles
  (https://zed.dev/docs/collaboration/channels (search)). inkwell's « Me rendre invisible » and « Suivre une
  personne » (spec §4.47) already answer the Figma complaints.
- **Drafts instead of conflicts.** In Upwelling, every edit lives in a titled draft (layer), private or shared. A
  merge rebases the other drafts, so divergences show *before* the final merge. Merged layers are immutable. History
  shows a deletion as a marker, with the text on hover (https://www.inkandswitch.com/upwelling/). In Peritext, bold
  grows at the edges but links and comments do not, and overlapping comments are kept
  (https://www.inkandswitch.com/peritext/). These match inkwell's « variante » and « divergence » (glossary,
  spec §4.22).
- **Access.** Keyhive builds access on capabilities and delegation without a central guard. Revocation needs to
  know who was authorised at which point in history (https://www.inkandswitch.com/keyhive/notebook/ (search)).
- **History.** In Google Docs, "Last edit" opens history, and a blue dot marks changes since your last visit. Named
  versions are kept apart (40 per document). Restore and "Make a copy" are separate actions
  (https://support.google.com/docs/answer/190843). Overleaf has "All history" and "Labels" ("Label this version"),
  but free history is 24 h only. Track changes is paid, and a review mode can track everyone's or only your own edits
  (https://docs.overleaf.com/collaborating/track-changes, https://sharelatex.com/learn/latex/Using_the_History_feature
  (search)).
- **typst.app** has review-only share links (comment, no edit) and **no built-in version history**. Git sync is the
  only path (https://typst.app/docs/web-app/comments/ (search),
  https://forum.typst.app/t/will-changes-be-tracked-in-typst-app/1309 (search)). inkwell's « Relire › Versions »
  without Git is a real advantage, if it stays simple.
- **Power that looks simple.**
  - NN/g: defer rare features to a secondary level, at most 2 levels (3 at the very most), one clearly labelled path
    to the advanced options. Decide by frequency of use and task analysis, not by guessing.
    https://www.nngroup.com/articles/progressive-disclosure/
  - Command palettes: Linear opens one with Cmd/Ctrl+K, which runs any action or jumps anywhere. Context menus teach
    the shortcuts, and `?` lists them all (https://linear.app/enablement/guide/navigating-linear (search)).
    Superhuman: a palette that shows each shortcut teaches it over time
    (https://blog.superhuman.com/how-to-build-a-remarkable-command-palette/). Cmd+K (Linear, Slack) and Cmd+Shift+P
    (VS Code, Sublime) are both conventions. inkwell keeps Ctrl+K for links (decision C14), so Ctrl+Shift+P is
    consistent.
- **Empty states** should show system status, help learning, and give a direct path to the key task
  (https://www.nngroup.com/videos/empty-states-in-application-design-guidelines/ (search)).

### 1.3 Beauty and feel

- **Response limits.** 0.1 s feels instantaneous. 1 s keeps the flow of thought, and beyond it the system must
  show it is working. 10 s is the limit of attention, beyond which you show percent-done and a way to stop. For waits
  of 2 to 10 s a small changing counter often beats a progress bar.
  https://www.nngroup.com/articles/response-times-3-important-limits/
- **Skeletons.** None below 1 s. From 2 to 10 s, a spinner for one component and a skeleton for a full page. Beyond
  10 s, a progress bar. Skeletons are static (shimmer distracts and hurts accessibility) and never replace real
  speed. https://www.nngroup.com/articles/skeleton-screens/
- **Doherty threshold.** About 400 ms, from 1982 IBM work. Past it, acknowledge the action
  (https://lawsofux.com/doherty-threshold/ (search)).
- **Line length.** 50–75 characters for body text, and WCAG 1.4.8 (AAA) caps it at 80
  (https://baymard.com/blog/line-length-readability (search)). The evidence for an "optimum" is mixed
  (https://news.ycombinator.com/item?id=5193835 (search)). For inkwell, the native choice is the 1.8 `.document`
  style.
- **Motion.** WCAG 2.3.3 (AAA) targets non-essential, interaction-triggered motion, and instant appearance does not
  count as animation (https://www.w3.org/WAI/WCAG21/Understanding/animation-from-interactions.html). The technique is
  `prefers-reduced-motion` (https://w3.org/WAI/WCAG22/Techniques/css/C39 (search)). Animations run on the clock,
  not on frames (NN/g, above). libadwaita 1.9 turns motion into crossfades rather than nothing (§1.1).
- **Dark mode.** Dark grey, not black, so that elevation stays visible
  (https://m2.material.io/design/color/dark-theme (search)). Adwaita's own dark palette (`#222226`, `#1d1d20`)
  already follows this.
- **Errors.** Show the error near its cause and never by colour alone. Match prominence to severity. Don't show an
  error too early. Be plain and specific, suggest the fix, and keep the person's input.
  https://www.nngroup.com/articles/error-message-guidelines/

---

## 2. Formal and measurable methods: verdicts

UF-1 to UF-8 stand. The table says, for each method, whether it adds something beyond them and how it becomes a
test. "New" rules are proposals numbered UF-9 onwards; they are not adopted until the user agrees.

| Method | What it gives | Verdict | Executable form |
|---|---|---|---|
| **KLM / GOMS** (Card, Moran, Newell) | expert task time from operator counts | **Keep as UF-5**, but use it for **relative** budgets only | Absolute predictions are unreliable: CogTool missed the measured 95 % CI in 9 of 12 tasks on a real interface (https://digitalcommons.lsu.edu/gradschool_theses/1401 (search)). So UF-5 freezes counts and the KLM sum per task, and fails on an increase. It never claims "share takes 8 s". |
| **CogTool / Cogulator** | storyboard + demo → ACT-R/KLM model | **Do not adopt the tool.** Borrow its idea: test replay → model, which is CogTool-Helper (https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/8903690 (search)) | UF-5's fixture *is* that idea. Operator values can be tuned as in Cogulator (https://cogulator.io (search)). |
| **Fitts's law** | pointing time from distance and width | **Adopt inside UF-5** | Replace the constant P = 1.1 s with P = a + b·log2(D/W + 1), using the bounding boxes Playwright already has. A change that moves « Partager » further away or makes it smaller becomes visible. Screen edges count as infinite width (https://nngroup.com/articles/fitts-law). |
| **Hick–Hyman law** | decision time grows with log(choices) | **Do not model.** It does not hold for unordered menus (https://www.medien.ifi.lmu.de/lehre/ss11/id/IxD-SS11-06-laws-of-IxD.pdf (search)) | Covered by UF-6's budget of visible controls, plus a cap on top-level items per menu. |
| **Cognitive walkthrough** (Wharton; streamlined, Spencer 2000) | per step: will they know what to do? will they know it worked? | **Keep in `ux-walkthrough`.** Make its second question executable as **UF-9** | **UF-9, feedback invariant.** After each action in an e2e path, a visible or announced change (DOM mutation in the target's region, `aria-live`, focus move) happens within 100 ms. If the result takes longer than 1 s, a progress cue appears. Measured with `MutationObserver` + `performance.now()` in the shared fixture. Source of the two questions: https://en.wikipedia.org/wiki/Cognitive_walkthrough, NN/g limits above. |
| **Statecharts** (Harel; XState v5) | flows as hierarchical machines, generated paths | **Keep as UF-3**; add one property, **UF-10** | **UF-10, mode visibility.** For every pair of states of a machine whose allowed events differ, the ARIA snapshot (below) differs too. A person can always tell which mode they are in. This is a property PVSio-web checks on medical devices ("visibility of mode"), computed here over the `xstate/graph` states. |
| **PVS / PVSio-web** | theorem-proved UI models, FDA medical devices (https://qmro.qmul.ac.uk/xmlui/handle/123456789/24773) | **Do not adopt the tool.** PVS is a heavy prover and the payoff is for safety-critical devices | Borrow its *properties*: mode visibility (UF-10), consistency (same key, same effect, UF-12 below), undo (UF-7). |
| **ICO / Petri nets** (PetShop) | concurrent UI models, invariant analysis (https://www.irit.fr/ICS/softwares/petshop/ico.html) | **Do not adopt.** The tooling is academic, and the concurrency inkwell cares about (several people) is already in TLA+ | None. TLA+ (SI1) plays this role. |
| **TLA+ of what the UI claims** | sync indicator honesty (SI1) | **Keep; extend** to the message hosts (R2) | The priority rules L-8/L-8b/L-9 become a pure TS function plus a fast-check property (below). TLA+ only if a timing race appears. |
| **SUS** | 10-item usability score, average 68 (https://measuringu.com/ux-benchmarks/) | **Adopt** for in-person tests (rule 10, no telemetry) | One SUS per release session. The goal is ≥ 80 (grade A range). It is recorded in the release UX report, not gated by CI. |
| **SEQ** | 7-point ease after each task, average 5.3–5.6 (https://measuringu.com/seq10/) | **Adopt**: the cheapest per-task signal for the five tasks | After each of the five tasks. Any task < 5.5 opens a defect with its walkthrough step. |
| **UEQ-S** | 8 items: pragmatic + hedonic quality (https://scitepress.org/Papers/2018/71883/pdf/index.html) | **Adopt.** It is the only standard measure of "beautiful and pleasant" (hedonic) | Same session. The hedonic scale is the metric for "belle"; it must not fall between releases. |
| **Task success and time** | completion rate, time on task | **Adopt** with SEQ | Same session. Success is binary per task; time is compared with the UF-5 KLM time to find where people hesitate (the M operator). |
| **WCAG 2.2 AA via axe** | about 57 % of issues by volume, per Deque's own figure; independent estimates are lower (https://www.deque.com/blog/automated-testing-study-identifies-57-percent-of-digital-accessibility-issues) | **UF-1 is necessary, not enough** | Add UF-11 and UF-12 below for what axe cannot see. |
| **WAI-ARIA APG patterns** | keyboard and role contract per widget | **Adopt as UF-12** | **UF-12, widget contracts.** A table maps each widget (menu button, dialog, combobox palette, tabs, grid, switch, toolbar) to its APG keys. A generated Playwright test presses each key and checks the expected focus and `aria-*` state. https://www.w3.org/WAI/ARIA/apg/patterns/ |
| **ARIA snapshots** | accessibility-tree baseline (Playwright ≥ 1.49; repo has 1.64.1) | **Adopt as UF-11**, next to UF-8 | **UF-11.** `toMatchAriaSnapshot` per view and per statechart state: names, roles and states don't drift. It is cheaper and less flaky than pixels, and it feeds UF-10. https://playwright.dev/docs/aria-snapshots |
| **WCAG 2.2 new criteria** 2.4.11, 2.5.7, 2.5.8, 3.3.7, 3.3.8 | focus not obscured, drag alternative, 24 px targets, no re-entry, accessible authentication (https://www.w3.org/TR/WCAG22/) | **Adopt in the UF-2 fixture** | The focused element's centre is not covered by a toast or banner (2.4.11, the same `elementFromPoint` as UF-2). Every target ≥ 24 px, bars ≥ 34, dialogs ≥ 44 (spec §2.5). Each drag (panels, tabs, sheet) has a named alternative. |
| **Reflow and text spacing** (1.4.10, 1.4.12) | no horizontal scroll at 320 CSS px; spacing override | **Adopt** (spec L-5, T-9 already state it) | A viewport of 320 px and a 400 % zoom plus the 1.4.12 stylesheet. Assert `scrollWidth ≤ clientWidth` and no clipped text. |
| **Screen reader automation** (Guidepup) | real VoiceOver/NVDA, or a virtual reader (https://github.com/guidepup/guidepup) | **Partial.** There is no Orca driver, so not on Linux CI | Use `@guidepup/virtual-screen-reader` in Vitest for the announcement order of `status`/`alert` regions (sync, toasts). Real readers only in the release walkthrough on macOS/Windows. |

---

## 3. Gaps in inkwell (code of 2026-11-09)

The spec (N17) is thorough and mostly agrees with §1. The gaps are between the spec and the code.

**G1. The title is the app name, not the project.** `App.svelte` passes `title={t("app.title")}` to `HeaderBar`.
Spec §4.1 H-1 says the project name, with `<h1>` = project.

**G2. The sync state is not a button and has no popover.** `ui/shell/HeaderBar.svelte` renders
`<span class="status" role="status">` with a dot and a word. Spec §4.15 makes it a button whose popover says who
received what, through what (direct or relay), what is waiting, and every pending message (L-8b). Today
« Envoi en cours (3) » cannot be explained, and a question hidden behind a banner (L-8b) has nowhere to go.

**G3. Banners stack; there is no `BannerHost`.** `App.svelte` mounts `SealedBanner`, `AccessBanner`,
`ProjectionBanner`, `<Banner>` for `limit.slow`, `ExternalBanner`, `PresentBanner` and `HistoryBars` side by side,
so several can show at once. This breaks L-8 ("au plus un bandeau visible", priority order). `ui/Banner.svelte` has
only `info | warning` tones; spec §5.16 needs `error` too.

**G4. Toasts have several sources and no queue.** `NoticeToast` and `ExternalChangeToast` are both mounted in
`App.svelte`; the sheet has its own (spec L-9 cites `Sheet.svelte`). `ui/Toast.svelte` has no timeout, no priority,
no queue and no entry or exit fade. It also holds raw colours (`rgb(0 0 6 / 84%)`, `#fff`), which UF-4 already
lists.

**G5. No command registry, no palette, no shortcuts dialog.** Keys are handled ad hoc in `App.svelte` `onKey` (F8,
Ctrl+S, F5, Ctrl+Alt+J, Ctrl+Shift+T). There is no Ctrl+Shift+P (spec §4.24), no Ctrl+, (spec §5.13), no F10 for the
main menu (spec §4.1), and no shortcuts list (`AdwShortcutsDialog` pattern, Ctrl+?). The main menu in `HeaderBar`
has no « Rechercher une commande… » and no « Projets… ». UF-6 needs an enumerable registry, and none exists yet.

**G6. No first minute.** `i18n/fr.json` has no welcome string. The name is asked inside the share dialog
(`ShareDialog.svelte`, `share.name`), and there is no colour choice and no Projects screen. Spec §5.1–5.2 define
the welcome `StatusPage`, the sample project, and « Rejoindre avec une invitation » on the Projects screen.

**G7. The share dialog mixes inviting and joining, and lacks the spec's steps.** `ShareDialog.svelte` has name,
« Inviter », the raw invitation in a read-only input, then a join form, with the error at the bottom
(`<p class="error" role="alert">`) far from the field. Missing, per spec §5.3–5.4: the role choice (Écriture /
Lecture), validity in plain words, comparing the 4-word fingerprint, and per-error actions (« Réessayer »). The
error sits away from its cause, against NN/g.

**G8. No tooltip component; `title=` is everywhere.** `title=` appears in 18 `.svelte` files (`HeaderBar`,
`PeopleStack`, …). Spec H-3 requires the §4.6 Tooltip. A person's name on an avatar is reachable only by mouse
hover through `title` (`PeopleStack.svelte`), not by keyboard or touch. This weakens the rule "jamais la couleur
seule".

**G9. Invalid markup in the avatar stack.** `PeopleStack.svelte` puts a `<ul>` with `<li aria-label>` inside a
`<button>`. A button only allows phrasing content
(https://html.spec.whatwg.org/multipage/form-elements.html#the-button-element), and the list's labels are flattened
into the button's name. axe does not flag it.

**G10. The token set is a subset of the spec.** `theme/tokens.css` has one duration (`--duration`), spacing
`--space-1..4`, and no typography, z-index, bar-height, pill radius, toast or dialog shadow tokens. Spec §2.3–2.8
define `--text-caption … --text-document`, `--duration-fast/-fade/-slow`, eases, `--space-0..6`, `--z-*`,
`--radius-pill`, `--bar-*`. Only 2 uses of `var(--text-small)` exist, and there are 627 raw `px` literals in
`.svelte` files (e.g. `HeaderBar` `h1 { font: 700 14.5px/1.25 }`, `small { font-size: 12px }`, `height: 47px`).
UF-4's lint would have no tokens to point to.

**G11. Adaptive layout covers one state of four.** There is one container query in `App.svelte` (`max-width:
560px`, the preview covers the source), two in `StatusBar.svelte` (900, 640) and one in `PresenterView.svelte`
(760). Spec §3.4 defines Large ≥ 1200, Medium 900–1199 (drawer overlays the preview), Narrow 560–899 (sidebar *and*
preview as overlays) and Very narrow 360–559 (header reduced to files, title, Partager, menu). `Sidebar` is never an
overlay.

**G12. Technical reasons inside sentences.** `fr.json`: `error.host` « Impossible d'inviter : {reason} »,
`people.error` « Impossible : {reason} », `history.error` « … ({reason}) », `projection.failed.reason`. This breaks
spec E-2 (the reason goes under « Détails ») and NN/g's "plain and specific".

**G13. No empty or loading component.** There is no `StatusPage` and no spinner. Loading is the text `file.loading`
in a `div.file-view role=status` (`App.svelte`). Spec §4.20 lists nine empty states, and §4.21 sets the wait rules.

**G14. Motion has no fade token.** Under reduced motion `tokens.css` sets `--duration: 0ms`, and nothing keeps
the fades of T-11. The libadwaita 1.9 crossfade approach (§1.1) is the target. The `connecting` pulse in
`HeaderBar` stops correctly, so that part is fine.

What is already strong: honest sync words driven by a TLA+ model (SI1), Okabe-Ito person colours with checked
contrast (`PeopleStack.svelte`, `tokens.test.ts`), high-contrast and reduced-motion media queries, the native
dialogs (`<dialog>` + `showModal`), and an Adwaita palette in three token tiers.

---

## 4. Ranked recommendations

Ranked by value for the person ÷ cost. **Covered** means the proof already exists in UF-1 to UF-8 or SI1; **new**
means it needs one of the proposed UF-9 to UF-12 or a test named here. Cost: S ≤ 1 day, M ≤ 3 days, L > 3 days.

**R1. Make the sync state a button with its popover (G2).**
- Value: the most-watched word in a local-first app becomes explainable: who received what, and through what. It
  is also the landing place of L-8b (no question is ever invisible).
- Cost M. Risk: low, the model exists.
- Proof: the SI1 model test (covered) plus an XState machine of the popover (UF-3, covered), UF-11 ARIA snapshot per
  sync state (new), and UF-2 hit-test (covered).

**R2. One `BannerHost` and one `ToastHost` (G3, G4).**
- Value: calm. One banner, the most important; one toast with « Annuler »; nothing hidden.
- Cost M. Risk: medium, since seven banner sources move.
- Proof (new): the priority rules as one pure function `pick(messages) → {banner, extra}`, with fast-check
  properties:
  - at most one banner;
  - a critical state is never hidden by a question;
  - if a question exists, it is either the banner or counted in « +1 question »;
  - at most one toast; a replaced toast stays recoverable (TO-1).
- Mutants: swap two priorities, drop the « +1 » counter. UF-7 (covered) checks that undo toasts follow destructive
  actions.

**R3. A command registry, then the palette, the shortcuts dialog, and the missing keys (G5).**
- Value: "never restrict" made real. Every command is one search away and shows its shortcut, which teaches it
  (Linear, Superhuman). F10, Ctrl+, and Ctrl+? follow GNOME.
- Cost L (registry M, palette M). Risk: medium, since key handlers move out of `App.svelte` and `sheet/keys.ts`.
- Proof:
  - UF-6 enumeration (covered once the registry exists);
  - UF-12 APG combobox contract for the palette (new);
  - a registry test that every shortcut is unique per platform (spec §8.2) and that `onKey` dispatches only
    through the registry (new).

**R4. The first minute: welcome, name and colour, sample project, a Projects screen with « Rejoindre » (G6).**
- Value: the first thing everyone sees. The name leaves the share dialog. No question about the network before
  « Partager » (rule 8).
- Cost M. Risk: low.
- Proof:
  - a UF-3 onboarding machine (empty name, too long, abandon, start) (covered);
  - a UF-5 budget "launch → first keystroke visible in the preview" (covered);
  - UF-9 feedback invariant on « Commencer » (new);
  - SEQ on "create" in person (new, §2).

**R5. Rebuild « Partager » to spec §5.3–5.4 (G7).**
- Value: the most stateful flow, and the first contact between two people. It needs the role, plain validity, a
  fingerprint comparison rather than retyping (WCAG 3.3.8), and errors placed at their cause with their next action.
- Cost M. Risk: medium (security wording; protocol states).
- Proof: this is UF2's first target (UF-3, UF-5, UF-7, covered). Add UF-10 mode visibility (new): « Connexion à
  Marie… », « Comparer » and « Réception » must have distinct ARIA snapshots.

**R6. Complete the tokens before the UF-4 lint (G10, G14).**
- Value: the Adwaita rhythm: one type scale (caption 12 / body 14 / heading / title-4 / title-2 / title-1 /
  document 16 / 1.6), the 2 px grid of §2.4, and motion with fades kept under reduced motion. Beauty comes from
  consistency, not decoration.
- Cost S–M. Risk: low (visual baselines catch drift).
- Proof:
  - UF-4 lint (covered), made possible by this;
  - a `tokens.test.ts` extension that every token in spec §2 exists and that, under `reduce`, only
    `--duration-fade` stays non-zero (new);
  - UF-8 baselines (covered).

**R7. A `Tooltip` component; no `title=`; fix the avatar markup (G8, G9).**
- Value: names and shortcuts become reachable by keyboard and touch, and the colour never stands alone.
- Cost S. Risk: low.
- Proof (new):
  - the UF-4 lint also refuses `title=` in `.svelte`;
  - `html-validate` in `fe:check` refuses non-phrasing content inside `<button>` (https://html-validate.org/rules/);
  - UF-12 tooltip contract: shown on focus, Escape hides it, `aria-describedby`.

**R8. UF-9: the feedback invariant (§2).**
- Value: "will they know it worked?" for every click, every time. It turns NN/g's 0.1 / 1 / 10 s and spec
  §4.21/W-1 into a check. It also guards PERF-1 (typing never blocked).
- Cost S (one fixture hook). Risk: CI timing noise, so allow 100 ms for the cue (not the result) and run it with
  motion reduced.
- Proof (new): a mutant that delays a toast by 500 ms, or removes `role=status` from a progress text, turns red.

**R9. Messages say what is kept and what to do; reasons go under « Détails » (G12, G13).**
- Value: friendly errors (spec E-2/E-3, NN/g), and empty states that point to the next step (HIG placeholders).
- Cost S (strings + `StatusPage` component). Risk: low.
- Proof (new), as i18n tests:
  - no `{reason}` placeholder in a sentence key, only in `*.details` keys;
  - every `error.*` key has a matching action key;
  - every empty state in spec §4.20 exists as a `StatusPage` with a title (axe and UF-11 cover the structure).

**R10. The four window states of spec §3.4 (G11).**
- Value: a window that works half-screen and on small laptops. The sidebar and preview become overlays as
  `AdwOverlaySplitView` would make them.
- Cost M. Risk: medium (layout).
- Proof:
  - a Playwright viewport matrix of 1280, 1000, 700, 360 and 320 (zoom 400 %);
  - UF-2 hit-test (covered) and UF-8 baselines per state (covered);
  - a reflow assertion `scrollWidth ≤ clientWidth` and the 1.4.12 stylesheet (new).
- Use container queries in `px` scaled by the text-size setting, as the `sp` units of AdwBreakpoint do.

**R11. UF-11 ARIA snapshots and UF-12 APG contracts (§2).**
- Value: screen-reader users get the same stability that UF-8 gives sighted users. Automation catches about half
  of accessibility issues, and these add the half that is structure and keyboard.
- Cost M. Risk: low (YAML snapshots, partial matching).
- Proof: these are themselves the proofs. Mutants: rename a button, drop `aria-expanded`, break Escape in a menu.

**R12. UF-2 extended: focus not obscured, target sizes, drag alternatives (WCAG 2.4.11, 2.5.8, 2.5.7).**
- Value: the toast and banners never hide the focus, and every target can be hit.
- Cost S (same fixture as UF-2). Risk: low.
- Proof (extends UF-2): a mutant toast placed over the focused status-bar item is caught, and so is a 20 px icon
  button.

**R13. Fitts in UF-5 (§2).**
- Value: the budget sees distance and size, not just the number of clicks.
- Cost S. Risk: low (relative only).
- Proof (extends UF-5): a mutant that shrinks « Partager » or moves it into the menu raises the KLM time and is red.

**R14. In-person metrics each release: SEQ per task, SUS and UEQ-S per session, task success and time (§2).**
- Value: the only measure of "pratique, simple, conviviale" and of the hedonic "belle" that does not rely on our
  own judgment. It needs no telemetry (rule 10).
- Cost S per session. Risk: small samples, so report the numbers and do not gate CI on them.
- Proof: the release UX report holds the scores. A release is not published with a task below SEQ 5.5 that has no
  filed defect.

**Not recommended:**
- PVS/PVSio-web and ICO/PetShop as tools (their properties are taken into UF-10 and UF-12).
- Hick's-law modelling.
- Skeleton shimmer: NN/g advises static skeletons, and T-11 forbids infinite animation.
- Lowering the spec's 300 ms spinner delay. The HIG's "about 3 s" is about *whether* to show a spinner; spec §4.21
  (after 300 ms) and W-1 (explain beyond 3 s) already reconcile the HIG with NN/g's 1 s.

**Order in the global plan.**
- R6, R7, R12 and R11 fit block **UF1** (same fixture, same lint).
- R2 and R1 go together, right after SI1: the host is where the popover lists the messages.
- R5 and R4 belong to block **UF2** (statecharts first).
- R3 goes with the palette work (UF-6).
- R10 is a block of its own, after UF1 has baselines.
- R8, R9 and R13 are small and go with whichever block touches their files.
- R14 goes with each release walkthrough.

Each of them needs a line in `docs/exigences.md` and its place in `docs/architecture/coherence.md` §5 when adopted.
