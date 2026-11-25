# UX1 — the interface plan: from the whole to every element (W82, W83, W84)

Status: **proposal, 2026-11-24**, decided by the integrator and recorded (CLAUDE.md « Autonomie »); the owner sees
its results in her R14 sessions, by wave (CLAUDE.md, step 4). One plan for block UX1 (point 4 of `PROGRESS.md`, « UX1 et W65 »),
written in one piece from the requests of that evening (W82, W83, W84; owner: « je veux un plan très cohérent, suivable
et maintenable dans le temps »). It holds the goal, the method, the proposed rules, the proof of inspection and the
order. It holds no state (that is `PROGRESS.md`) and no decision record (that is `docs/autonomous-decisions.md`, §7).
§8 says where each piece goes and how this plan is retired.

## 1. Goal and principles

### 1.1 The goal

- **W82, extremely polished.** Every element of the interface follows the written rules and the best principles of
  interface design taken as a whole, in the Adwaita style: well placed, useful in its context and in the whole,
  duplicating nothing, convivial and beautiful, for itself and in its screen. Nothing is left to chance: every size,
  gap, colour, word, order, icon, state and motion is justified by a sourced rule (§3) or a recorded decision (§7), and
  every element is inspected, **every one**, with a machine proof that none was missed (§4).
- **W84, every function under an apparent simplicity** (the governing principle; it is R1's « beau, simple en apparence
  et complet »). Inkwell absorbs the complexity rather than handing it to the person (Tesler's law,
  <https://lawsofux.com/teslers-law/>: good defaults, automation); the screen shows the task of the moment and the rest
  is one gesture away, two levels at most (progressive disclosure,
  <https://www.nngroup.com/articles/progressive-disclosure/>: rules 11 and 24); every function is findable by name in
  the command palette without taking screen space (spec §4.24, built by EX0b); advanced functions appear where they
  serve. It can grow while still looking simple, because growth goes into reserved places (W83).
- **W83, extensibility designed in.** A function added later, or forgotten, or brought by a plugin, enters the
  interface easily and meets every rule by construction: §5.

### 1.2 What this plan builds on, and does not repeat

`docs/ux-principles.md` (rules 1–10, visual system), `docs/glossary.md`, `docs/engineering-principles.md` §5 (UF-1…UF-12),
`docs/design/spec.md` (rév. 7: P1–P7, the component rules of §4, the screens of §5, voice §6, accessibility §7, K-1…K-5
and the command registry §8, anti-patterns §9), `docs/design/coverage.md` (the standards grid), the two studies the
owner approved on 2026-11-14, `2026-11-14-ux-ui-study.md` (P1–P32 except P26) and `2026-11-14-ux-diff-versions-files.md`
(T, D, V, F, O, S, G), their source digests `docs/ux/sources/2026-11-14-{gnome,craft}.md`, the references of
`docs/design/reference/`, and the contribution registry of EX0 (`2026-11-24-ex0-contribution-registry.md`, ADR 0027).

Where two of them differ, the later owner decision wins: the approved studies amend the spec on their points (P11 makes
« Partager » a flat button with a label, against H-2's suggested button; P12 puts the sync state in the central pill,
against §4.1's left side), and `docs/ux-principles.md` wins over the spec (spec's own README). UX1-C writes these
amendments into the spec, so one text remains.

### 1.3 Precedence of the guides (owner, 2026-11-24)

This orders the **guides** against each other; it does not change the order of the spec's principles (§1.4). The
GNOME HIG and libadwaita are the visual language. Every other source (Apple HIG, Fluent 2, Material 3, NN/g, Laws of
UX, WCAG 2.2, WAI-ARIA APG, interface writing, the reference editors) only fills what they leave unsaid and never
overrides them. Among the guides, Adwaita yields only to accessibility (as for the focus ring, spec §2.2) and to the
system's reflexes the spec gives to the host (window buttons, §8.4; dialog button order, spec §4.8). Inside the spec,
fidelity to Adwaita is P7 and still yields to P1–P6 by the arbitration of §1.4: a guide never decides against a
principle.

### 1.4 When the principles pull apart

The spec's arbitration is unchanged and comes first: **P5 and P6 are invariants** (no jargon; accessible to everyone),
then **P4 > P3 > P2 > P1 > P7**. The three requests sit inside it rather than beside it:

| Tension | Who wins | Why, and the guard |
|---|---|---|
| polish (W82) or simplicity (W84) wants a control that another wants hidden | simplicity: the function goes one level down (menu, palette) rather than adding a visible control (rules 11, 12, 24) | P1 « le texte d'abord » ranks above P7; the guard is UF-6: every command stays in the palette and at most two steps from a menu, so simplicity never makes a function unreachable |
| a placement that suits one surface but has no slot in the screen grammar (W83) | the grammar's slot: the function adapts, or UX1-B amends the grammar for every surface at once | P2 « un seul langage » ranks above P1 and P7 |
| Adwaita or another guide | Adwaita, except accessibility (P6) and the host's reflexes (§1.3) | conflicts settled one by one in §3.11 |
| beauty or task success | task success first; beauty is judged after it, never instead (rule 51) | the owner session (UX1-E) asks tasks, time and SEQ before aesthetics |

## 2. Method: from the whole to the element, then the loop

### 2.1 Why top-down

The owner asked for « la plus pratique, la plus soignée, la plus jolie, la plus performante, tout en gardant une
simplicité ». Established methods go from the whole to the detail, then loop: Jesse James Garrett's five planes
(strategy, scope, structure, skeleton, surface; each plane constrains the next, and polishing the surface first is the
classic error; <https://www.jjg.net/elements/>); the Design Council's Double Diamond (diverge then converge, twice, with
people in each round; <https://www.designcouncil.org.uk/our-resources/the-double-diamond/>); the design systems of
GNOME, Apple and Material (principles, tokens, components, templates; screens are assembled from them, so coherence
comes from construction, not from a check afterwards). Simplicity is the guard of every phase: design by subtraction
(GNOME HIG principles, « Make it simple »), one primary action per screen, and **each phase lists what it removed**, not
only what it added.

### 2.2 The phases

UX1-0 measures, A to C decide the whole, D inspects each element against it, E closes the loop with the owner. Each
phase is committed before the next starts (method v2, rule 1).

**UX1-0 — the proof harness** (light rigour; first, because every later phase is measured with it; decision « UX1
question 4 »).
- Inputs: EX0b's command registry (`data-cmd` needs it), the state sources of §4.3 that exist when it starts, the
  i18n catalogues, the glossary and spec §6.4, spec §4.
- Decides: the element id scheme (`data-el`), the state list, the record format, the checks EL-1…EL-14 (§4).
- Outputs: `scripts/ux_elements.mjs`, `scripts/ux_states.py`, `frontend/e2e/ux/elements.spec.ts`, check 8 « ui-elements »
  of `scripts/inventory.py`, `docs/ux/elements/` (static and runtime lists, states), `docs/ux/vocabulary.toml`,
  `docs/ux/components.toml`; the common e2e fixture with UF-1 (axe-core) and UF-2 (`elementFromPoint`), taken from
  block UF1 because every EL check runs through it; the measuring half of W65's guards G2 and G3 comes forward into it.
- Checks: each EL check turns red on its seeded defect (§4.7).
- **Measures UX1's cost** (owner decision, 2026-11-24): on the shell, the count of elements and states, the share of
  record fields the machine fills, and the reviewer minutes per element on the deviations reported; from these, the
  time budget of each UX1-D sub-block, written in its plan; a sub-block over its budget is re-split (method v2,
  rule 4), never reviewed faster.
- Exit: gate `gates/UX1-0.toml` green on the shell, every seeded defect red, the cost measured.
- Removes: nothing on screen; the hand-written element lists of `docs/design/inventory.md` stop being maintained (§8).
- Owner review: none for the tooling; its first coverage report opens the UX1-A review.

**UX1-A — tasks and structure** (Garrett's structure plane).
- Inputs: the five tasks of `docs/ux-principles.md`, the screens of spec §5, the XState graph, the UF-5 budgets, the
  two studies, the registry's command list.
- Decides: the tasks ranked by frequency; the map of every screen and how a person moves between them; for each command,
  whether it is visible, one gesture away or two (rules 11, 20, 24; Hick's and Jakob's laws).
- Outputs: `docs/ux/structure.md` (task ranking, screen map) and **the function tree** `docs/ux/functions.toml` (W85,
  owner, 2026-11-24): every function of Inkwell — done, planned or after R1, plugins included — grouped by task, each with its
  screen, its slot and rank (primary, secondary, overflow, palette only), its PROGRESS block, its requirement ids and its
  state; reserved branches and slots for what will come, so a new idea is placed before it is built (W83). It is the one
  list of functions: `coherence.md` §5 lots and spec §5 tasks point to it instead of listing functions again.
- Checks: every screen of spec §5 and every machine state is on the map; UF-6 holds for every command; the tree is
  checked like the inventory: every requirement row of `docs/exigences.md` that asks for a function maps to a node, every
  command of the registry (EX0b) has its node and slot, a node without a block or a slot fails, and a new requirement is
  recorded with its node (check 9 « functions » of `scripts/inventory.py`, in `pregate`, written in UX1-A).
- Exit: map reviewed blind (Opus, plus Gemini when its quota allows: §2.3); the integrator decides and records; the
  owner sees it in the wave's R14 session.
- Removes: duplicate paths to one place, commands visible without a frequent task (listed).
- Owner review (R14, by wave): the map and the disclosure levels.

**UX1-B — screen grammar and templates** (skeleton plane).
- Inputs: `structure.md`, spec §3.1 zones and L-1…L-7, header variant B (`2026-11-13-report-and-header-design.md`, HB1),
  P9, P12, P17, P18, the reserved slots of spec §4.31, the references of `docs/design/reference/`.
- Decides: the regions every main screen shares (header, view bar, tabs, left, main, drawer, status); the hierarchy of
  actions (one primary, secondary, overflow ⋯); the templates (document, list, settings, dialog, popover) with their
  reserved slots for functions added afterwards and for extensions (§5).
- Outputs: spec §3 amended with the region table and templates; one reference per template in
  `docs/design/reference/`; the measured invariants the checks read (UF-8, G3).
- Checks: EL-13 (regions), EL-4 and EL-5 on the templates.
- Exit: every main surface declares its regions; EL-13 green, or each gap listed as a finding of its UX1-D step.
- Removes: per-surface bars that repeat the shell's (the sheet's own toolbar, status bar and find bar: S7), a view
  switch that changes place between views (UX-EX0-3).
- Owner review (R14, by wave): the templates, light and dark, at the four widths.

**UX1-C — the design system** (surface plane, as a system).
- Inputs: spec §2 (tokens) and §4 (components), `docs/design/tokens.json`, `frontend/src/theme/tokens.css`, the study
  proposals P1–P6, P15, P22, P23, P28, S2, S6.
- Decides: Adwaita tokens only, on one scale per role (rule 53); one component per role (spec P2, K-1), each with all its
  states (enabled, disabled with why, busy, error, offline, read-only role), keyboard pattern and accessible name.
- Outputs: tokens in `tokens.json` and `tokens.css`; the components spec §4 marks « à créer » (Dialog, Tooltip,
  Banner and BannerHost, Toast and ToastHost, StatusPage); `docs/ux/components.toml` (role → component);
  spec §4 amended where the studies and the decisions of §7 changed it (§4.1, §4.4, §4.8, §4.9).
- Checks: EL-9…EL-12; UF-4 on the sources (its lint is built here, taken from block UF1).
- Exit: every surface draws each role with its component; no value outside the scale.
- Removes: the seven dialog copies, local button variants, the 13 font sizes and 709 `px` literals measured by the W63
  inventory, components left unused.
- Owner review (R14, by wave): the component sheet, light and dark.

**UX1-D — each surface, element by element** (the final check, not the starting point). One sub-block per surface, in
the order of §6.
- Inputs: phases A–C, the harness, the surface's records.
- Decides: for each element, keep, move, merge or remove (§4.2 `verdict`). The machine checks run on every element;
  the human review reads only the deviations they report and the fields no machine can fill (§4.2), never every
  element by hand (owner decision, 2026-11-24).
- Outputs: records at 100 % for the surface; each machine-checkable finding as a failing e2e test named by rule and
  element (`UXR-20 header single suggested`), then the fix, as the `ux-walkthrough` skill does.
- Checks: EL-1…EL-14 green on the surface; the `ux-walkthrough` pass.
- Exit: every record of the surface has a verdict and every check is green.
- Removes: what the verdicts remove (each listed in the sub-block's `SIMPLIFY.md`).
- Owner review: UX1-E.

**UX1-E — the loop with the owner** (Double Diamond's second round).
- Inputs: the surface after UX1-D, its neighbours sheet (EL-14), its reference.
- Steps: the surface's tasks first, timed, with SEQ (R14 of `2026-11-09-ux-research.md`); then the five-second
  recognition of its icon-only controls (rule 50); then light and dark captures next to the reference (UF-8);
  aesthetics last (rule 51); effort (UF-5) and the PERF budgets measured.
- Outputs: `docs/ux/sessions/<date>-<surface>.md`; decisions to `docs/autonomous-decisions.md` or the reference.
- Sessions are grouped by wave (CLAUDE.md, « Définition de fini », step 4): one session covers the surfaces finished
  since the last; the next surface does not wait for it, and a pending session blocks R1, not `main`.
- Exit: the loop is quiet (no new finding in a session for the surface).
- Removes: what the session shows to be unused or misunderstood.
- Owner review: this phase is the review.

### 2.3 Rigour and reviews

UX1 is « standard » (no peer bytes): each UX1-D sub-block has a plan of two pages at most and a gate of 45 min at most;
UX1-0 is « light » (tooling), with seeded defects in its gate. A, B and C are design phases whose outputs are reviewed
blind before UX1-D starts. As standard blocks, their reviews are a blind Opus review with checklist plus Gemini when
its quota allows, never OpenRouter (method v2, « When Codex is absent »); never Sonnet alone; Sonnet fills machine
fields only. The integrator decides on each review; the owner's R14 sessions review by wave, they are not an exit.

## 3. The rule set

Numbering continues `docs/ux-principles.md` (rules 1–10): rules **11–54**. **Each rule has one wording, the French
text of §3.12**; the tables below keep only its number, source, reason and proof. A rule is kept only if none of the sources of
§1.2 already states it; where one states part of it, the row says « extends » and keeps only the new part. « Proof »
names a check of §4 (each becomes a UF row of `docs/engineering-principles.md` §5), a lint, or « review » (the element
record carries the verdict, then UX1-E).

**Removed as duplicates in this rewrite.** The draft's « one command, one identity » is rule 14 (now with its registry
source); « one concept, one word » is P30 and spec §6.4; « one role, one component » is spec P2 and K-1; « neighbours
reviewed together » is spec K-3. They are no longer rules here: their checks (EL-9, EL-10, EL-11, EL-14) stay in §4.5.
Rules 20, 25, 26, 28 and 36 lost the parts spec B-1, §4.2, §4.7, §4.8 (D-1, B-2) and §6.2 already state. Rules 22 and 50
were rewritten to agree with the decision on icon-only buttons (§7). The draft's coherence rules « one scale » and « one
screen grammar » became 53 and 54.

**Sources read on 2026-11-24** (fetched pages; sections named in each row). GNOME HIG: principles, header bars, buttons,
menus, popovers, dialogs, writing style, accessibility, adaptive (<https://developer.gnome.org/hig/>); libadwaita style
classes (<https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/style-classes.html>); Apple HIG layout, colour,
typography, motion, toolbars, context menus, writing (read through the HIG's JSON data endpoint, same text as
<https://developer.apple.com/design/human-interface-guidelines/>); Fluent 2 layout and motion
(<https://fluent2.microsoft.design/>); Windows dialog controls
(<https://learn.microsoft.com/windows/apps/design/controls/dialogs-and-flyouts/dialogs>); Android canonical layouts,
derived from Material 3 (<https://developer.android.com/develop/ui/compose/layouts/adaptive/canonical-layouts>); NN/g:
ten heuristics (updated 2024-01-30), progressive disclosure, visual hierarchy, Gestalt proximity, Fitts's law, icon
usability, empty states, error messages, form design, indicators/validations/notifications, aesthetic-usability effect;
Laws of UX (<https://lawsofux.com/>); Butterick, *Typography in ten minutes*; WCAG 2.2 Understanding 2.5.3 and 3.2.4;
APCA in a nutshell; WAI-ARIA APG keyboard interface; VS Code UX guidelines (status bar, notifications, command palette);
iA Writer focus mode; Obsidian command palette; Notion shortcuts. **Limits:** Material 3 pages render in JavaScript and
returned no text (only the Android page above, and the older material.io density scale through a search snippet);
Typora's help returned 404, so its live-preview flaw rests on one forum quote of Daring Fireball (marked « weak »);
Google Docs « Live edits » comes from search results and Google's 2019 announcement, not a fetched help page;
Bringhurst's book was not read (Butterick stands in).

### 3.1 Presence: every element earns its place

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 11 | GNOME principles « Make it simple »; NN/g heuristic 8; NN/g progressive disclosure: « disclose everything that users frequently need up front » | W82 « utile dans son contexte », W84; today nothing asks why an element is there | EL-1 (record field `task`), EL-1b, review |
| 12 | Apple toolbars, macOS: « it doesn't make sense to provide a toolbar item for every menu item »; Apple context menus: « Always make context menu items available in the main interface, too »; Hick's law | W82 « sans rien dupliquer »; extends P2, K-5 and P12 (verbs, shortcuts, sync state) to every control | EL-2 |
| 13 | extends P12 to every state; NN/g: indicators sit on the element they describe | three « unsaved » signals and two sync places measured on 2026-11-14 | EL-2 (`data-state-of`) |
| 14 | WCAG 3.2.4 (different icons for one function is a listed failure); Apple writing « Consistency of terms »; NN/g heuristic 4 | extends §8.3 « icônes par idée » and K-5 from tables to checks; P20 and P22 make the registry feed menus, tooltips and palette | EL-3 (per surface), EL-9 (across surfaces) |
| 15 | GNOME header bars: three alignment points, menus at the end; Apple toolbars: trailing items remain visible at all widths, « Only specify one primary action » | extends spec §4.1 (header anatomy) to every bar; the header's order needs a reason, not habit | EL-4 (order of `data-el` per bar), G3 invariants |
| 16 | Apple layout, macOS: « Avoid placing controls or critical information at the bottom of a window »; VS Code status bar | the status bar keeps four items (§4.17); this says which may go there | EL-2 (a status-bar command must have another surface) |
| 17 | NN/g Fitts's law: « make targets big », options « close to the label », overshoot risk | « Retirer » next to « Déconnecter » in the people list is one slip away | EL-4 (gap ≥ one control width or another group), review |

### 3.2 Grouping, alignment, hierarchy

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 18 | NN/g Gestalt proximity; Fluent 2 layout: « Space is used to denote groups »; GNOME header bars: group by spacing rather than linked buttons | P2 fixed the grid, not how it is used | EL-4 |
| 19 | Apple layout « Align elements… use indentation to convey hierarchy »; libadwaita `.numeric` | the version rows, file rows and status bar mix edges today | EL-4 (edges ± 0.5 px), lint (`tabular-nums` on counters) |
| 20 | NN/g visual hierarchy: no more than three sizes, the squint test | P1 reduced the scale to five styles; this caps how many meet in one view | EL-5, review (blurred capture) |
| 21 | GNOME buttons (« same width », « more than one or two different widths », « either an icon or a label ») | dialog footers have uneven widths; UX-PC1a-7 is uneven wrapping of two buttons | EL-4 |
| 22 | GNOME header bars; libadwaita: buttons in toolbars are flat by default | amends H-2's suggested « Partager » as P11 decided | EL-5 (style classes per bar) |
| 23 | GNOME adaptive: « containers that have a maximum width »; Android supporting pane: secondary pane about one third | P15 caps the text only; at 1920 px a preferences row runs edge to edge | EL-4 at 1920 and 360 px |

### 3.3 Disclosure: menus, popovers, dialogs

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 24 | NN/g progressive disclosure: beyond two levels « typically have low usability »; GNOME menus: « Don't nest submenus »; Windows: one dialog per window at a time | W84; D1 removed one stacked modal; this forbids the class | EL-6 |
| 25 | GNOME menus | extends P14 (main menu order) to every menu, insert and format menus included | EL-5 (registry menu definitions) |
| 26 | GNOME popovers | the share and people popovers grow with content | EL-4, review |
| 27 | Apple context menus (relevance, consistency, « Hide unavailable menu items, don't dim them ») | file rows, tabs, version rows, people rows and the text get menus at different times (P27, F3, V4) | EL-5 (each object kind has one), EL-3 |
| 28 | GNOME dialogs; Windows dialog controls | makes D-1 checkable: the question from another person is a banner (L-8) | EL-6 (a dialog opened > 1 s after the last input event fails, unless listed) |
| 29 | Hick's law; Miller's law, read as chunking (Laws of UX) | font, language and template lists grow (N43) | lint (option groups > 7 without sections or filter) |

### 3.4 Messages, errors, forms

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 30 | NN/g indicators, validations, notifications; GNOME dialogs | extends spec §3.5 (three kinds) with the indicator, the most frequent kind in a collaborative editor (faces, dots, marks) | review, EL-6 |
| 31 | NN/g error messages; Apple writing « Errors »; Windows: no dialog for an error contextual to a place | extends rule 5 with placement and input preservation | lint (i18n words), e2e: value kept and `aria-describedby` after a refused input |
| 32 | VS Code notifications (« Send repeated notifications », « Use for promotion » listed as don'ts) | seven banner sources (S2); reconnecting networks repeat | unit test of `ToastHost`/`BannerHost` (same key, same cause ⇒ one) |
| 33 | NN/g form design | join, share, preferences and templates are forms | lint (placeholder without label), EL-4 (field width), review |
| 34 | NN/g empty states; Apple writing « Empty States » | completes P23 | review |

### 3.5 Words

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 35 | Apple writing « Settings Labels » | « Réglable dans Préférences › Écriture » strings exist | EL-7 (a « › » path outside a link or button) |
| 36 | GNOME writing style; W3C COGA « Making Content Usable » (Working Group Note, 2021-04-29), objective 3, patterns 4.4.1 « Use Clear Words », 4.4.2, 4.4.3, 4.4.5 (<https://www.w3.org/TR/coga-usable/>, read 2026-11-24) | completes spec §6.2; P5 bans jargon but says nothing of sentence form | i18n lint (sentence length, double negatives), review |
| 37 | WCAG 2.5.3 and its best practice « the text of the label at the start of the name »; GNOME accessibility | 2.5.3 is open in `coverage.md` (G-31) | EL-3 |

### 3.6 Colour and type

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 38 | Apple colour (« same color to mean different things »); NN/g visual hierarchy; VS Code status bar (no custom colours) | extends P7 (« ni couleur sans rôle ») to a token-to-role map | EL-7 (each colour token only in its role's selectors; extends UF-4) |
| 39 | APCA in a nutshell (candidate method for WCAG 3) | dark-mode pairs that pass 4.5:1 can still read thin | `theme/tokens.test.ts` prints Lc (non-blocking) |
| 40 | Apple typography « avoid light font weights », « Minimize the number of typefaces » | Adwaita Sans ships light weights | EL-5 (computed weights) |
| 41 | Butterick: 120–145 % line spacing, 45–90 characters | gives numbers to « interligne augmenté » and joins P15 | EL-4 (computed style) |

### 3.7 Keyboard and assistive technology

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 42 | WAI-ARIA APG keyboard interface (« consistent pattern-based conventions for the focusability of disabled elements ») | the « why » tooltip is mouse-only today | unit tests per composite (UF-12) |
| 43 | NN/g icon usability « Don't rely on hover to reveal text labels »; Notion's hover handles as the counter-example | T1–T3 margin marks, row actions (V4) and the preview pill (P9) are hover-revealed | EL-8 |
| 44 | Google Docs « Live edits » summary sidebar and its own warning that many suggestions flood the reader (search results; weak) | completes T9 and A-10 | `ui/messages.test.ts`, Orca pass |

### 3.8 Motion and adaptivity

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 45 | Fluent 2 motion: « Give larger elements more time to animate than smaller elements » | P5 gave one curve, not the scale | unit test on duration tokens |
| 46 | Apple motion, « Providing feedback » | panes, drawer, search bar | review (captures of both transitions) |
| 47 | GNOME adaptive: « widgets should not jump around or disappear without an animation »; Typora's markup that « suddenly appears » when the caret enters a link (forum quote, weak) | extends T4 (remote text) and P5 (banners) to local causes | EL-8 |

### 3.9 What competing editors teach

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 48 | Obsidian command palette; VS Code palette guidelines | completes P20 and W84's « trouvable par son nom » | unit test of the palette ranking |
| 49 | iA Writer focus mode: its own warning that centring « might result in the screen jumping » | P10 focus mode | e2e (selection does not scroll) |
| 50 | NN/g icon usability (labels needed, a handful of universal icons) within GNOME header bars and Apple toolbars (icon buttons with tooltips) | NN/g's test found a clock for « history » unrecognised by every participant; Inkwell uses `clock` for Versions | EL-7 (icon-only outside bars or outside the universal list), UX1-E |
| 51 | NN/g aesthetic-usability effect: a pretty design « masked its usability issues » | W82 asks « beau »; this keeps it from hiding defects | UX1-E protocol |
| 52 | owner's quality bar; the rows above | turns competitive reading into a checked field | EL-1 (record field `compare`) |

### 3.10 Coherence of the whole

An element can pass every rule above and still disagree with its neighbours (owner, 2026-11-24: « chaque élément peut
être bien mais pas cohérent avec les autres »). Most coherence properties are already rules (14, P30 and §6.4, P2 and
K-1, K-3) and get their checks in §4.5; two are new:

| # | Source | Why for Inkwell | Proof |
|---|---|---|---|
| 53 | Fluent 2 layout (multiples of the base unit); NN/g visual hierarchy (few sizes) | 13 font sizes and 709 `px` literals today (W63 inventory); UF-4 checks the source, this checks the result, so a token used for another role is caught | EL-12 |
| 54 | GNOME header bars (three fixed alignment points); Apple toolbars « Keep consistent groupings and placement »; WCAG 3.2.3 | the text/table switch at the left in one view and the right in the other (UX-EX0-3); the sheet's status bar taller than the shell's; extends spec §3.1 and L-1…L-7 | EL-13 |

### 3.11 Conflicts between guides, and the choice

| Topic | Disagreement | Choice | Reason |
|---|---|---|---|
| Dialog button order | GNOME and Apple: cancel first, affirmative trailing; Windows: « do it » leftmost | the host system's order, the Adwaita look everywhere (decision « UX1 question 1 ») | spec §8.4: the drawing stays Adwaita, a reflex follows the system, as the window buttons do (Jakob's law); written in spec §4.8 and §4.9 (2026-11-24), its one home |
| Icon-only vs labels | GNOME header bars and Apple toolbars prefer symbols; NN/g: a label must accompany icons | rule 50 (decision « UX1 question 2 ») | precedence of GNOME for the bars; NN/g's risk handled by the universal list, tooltips (P22) and the recognition test |
| Disabled vs hidden | Apple: hide unavailable menu items; spec §4: dim with a « why » | hidden in context menus; dimmed with the why in bars and the main menu (rules 27, 42; decision « UX1 question 3 ») | transient lists stay short; permanent places must not move under the pointer (rule 47) |
| Grid | Fluent 4 px, Material and SAP 4/8 dp, libadwaita 6 px | 6 px (P2) | precedence of Adwaita |
| Targets | Fluent and Apple 44 px, libadwaita 34 px controls | spec §2.5 and A-7 (their one home; P31 added in UX1-C) | precedence of Adwaita; WCAG 2.5.8 met |
| Breakpoints | Fluent six classes, Android 600/840 dp, Adwaita 400/550/860 sp | spec §3.4 | already decided, rounded from Adwaita |
| « You » and full stops | GNOME (English): avoid « you », no period at the end of text | French: vouvoiement and one full sentence in banners and toasts (spec §6.1–6.2); no period on buttons and menu items | French desktop software addresses the person as « vous » (re-read on GNOME's French translation files in UX1-5 → UX1); a sentence ends with a period in French |
| Status at the bottom | Apple macOS: no critical information at the bottom | status bar kept, nothing critical only there (rule 16) | GNOME has no such rule; the status bar holds only facts available elsewhere |

### 3.12 The one wording of rules 11–54 (French, pasted after « Système visuel » of `docs/ux-principles.md` when UX1-0's gate is frozen)

```markdown
## Chaque élément (W82, 2026-11-24)

Règle d'ensemble : Adwaita et la HIG de GNOME donnent le langage ; les autres guides (Apple, Fluent 2, Material 3,
NN/g, WCAG, APG) ne comblent que ce qu'ils taisent. Sources, raisons et conflits tranchés :
`docs/plans/2026-11-24-ux1-interface-plan.md` (§3). Chaque élément de l'interface a une fiche d'inspection, et une
machine vérifie qu'aucun n'en manque (EL-1).

**Présence**
11. Chaque élément visible sert une tâche nommée de son écran ; sinon il descend d'un niveau (menu, palette).
12. Une commande, un seul contrôle visible par état d'écran (clavier, palette et barre de menus de macOS exceptés) ;
    un menu contextuel ne reprend que ce que l'interface offre déjà.
13. Un fait, une seule place : un état (synchronisation, non enregistré, rôle, problèmes, présence) est montré par un
    seul élément par état d'écran.
14. Même fonction, même identité partout : un libellé, une icône, un raccourci et une infobulle, tirés du registre des
    commandes et de lui seul ; une icône, un seul sens.
15. Dans une barre : au début la navigation et les panneaux, au centre le titre et l'état, à la fin les menus et ce qui
    ouvre le tiroir ; une action principale au plus, à la fin.
16. Rien d'essentiel ne vit seulement dans la barre d'état.
17. Les cibles fréquentes sont grandes et près de leur contenu ; un menu s'ouvre à son déclencheur ; une action
    destructrice n'est jamais voisine d'une action fréquente.

**Groupes et hiérarchie**
18. L'écart entre deux groupes vaut au moins deux fois le plus grand écart dans un groupe (grille de 6 px : 6 dans un
    groupe, 12 à 18 entre groupes) ; une étiquette est plus près de son contrôle que de tout autre.
19. Des éléments frères partagent un bord d'attaque ; le retrait marque la subordination ; compteurs et heures en
    chiffres tabulaires.
20. Par vue : trois tailles de texte au plus dans le cadre, un seul point focal (test du flou).
21. Des boutons voisins ont la même largeur, deux largeurs au plus par fenêtre ; hors des barres, icône ou texte, pas
    les deux.
22. Dans la barre d'en-tête, des boutons plats ; ni style suggéré, ni destructeur, ni lié (dialogues d'action exceptés) ;
    « Partager » est plat, avec son libellé (P11).
23. Dialogues, préférences, écrans vides et listes ont une largeur maximale et se centrent.

**Divulgation**
24. Deux niveaux au plus : visible, puis un geste (popover, panneau, menu), puis un dialogue ; jamais un popover dans
    un popover, un sous-menu dans un sous-menu, un dialogue sur un dialogue.
25. Un menu a au moins trois entrées (un sous-menu, trois à six), rangées par tâche ou fréquence ; des adjectifs pour
    les réglages ; le groupe standard à la fin.
26. Un popover mêle peu de sortes de contrôles et n'a de titre que si son rôle est ambigu.
27. Un menu contextuel : seulement les commandes probables pour cet objet, la plus fréquente au plus près du pointeur,
    sur chaque sorte d'objet ou aucune ; les entrées indisponibles sont masquées, pas grisées.
28. Un dialogue a un à trois boutons, dans l'ordre du système hôte (spec §4.8). Exception nommée : un dialogue qui
    répond au geste de la personne mais dont l'issue n'est connue qu'ensuite (« Réessayer / Fermer quand même » de WC1
    après un enregistrement manqué à la fermeture) ; chaque exception est listée par son `data-el`.
29. Au-delà d'environ sept choix, des groupes ou un filtre.

**Messages et formulaires**
30. La sorte du message choisit le composant : indicateur sur l'élément, validation près du champ, notification en
    toast, bandeau ou, seulement si une réponse est indispensable, dialogue.
31. Une erreur se place près de sa source, garde ce qui a été saisi, dit le remède, n'accuse jamais (« invalide »,
    « incorrect » proscrits).
32. Un même message ne revient pas tant que sa cause n'a pas changé ; jamais de message de promotion ni de demande d'avis.
33. Formulaires : une colonne, une étiquette visible (jamais le texte indicatif seul), un champ à la taille de la saisie
    attendue, « facultatif » écrit, Annuler plus discret que l'action.
34. Un écran vide dit aussi ce qui apparaîtra ici, et ne porte jamais seul une information essentielle.

**Mots**
35. Un réglage dit ce qu'il fait quand il est activé ; pour envoyer vers un réglage, un bouton qui l'ouvre, jamais son
    chemin en toutes lettres.
36. Des libellés courts, sans abréviation latine (« ex. », « etc. ») ; des messages en langage simple : mots
    courants, une idée par phrase, voix active, ni double négation ni subordonnées enchâssées.
37. Le nom lu par le lecteur d'écran commence par le libellé visible ; celui d'un bouton à icône est le nom de son
    infobulle.

**Couleur et texte**
38. Une couleur, un sens : l'accent pour l'interactif et la sélection, les jetons d'erreur pour l'erreur et le
    destructeur, les couleurs des personnes pour les personnes ; aucune couleur décorative.
39. Le contraste APCA est relevé à côté du rapport WCAG, à titre d'information (Lc 75 pour le texte courant, 60 pour les
    libellés du cadre, 45 pour le grand texte, 30 pour les icônes pleines) ; WCAG 2.2 reste la règle.
40. Aucune graisse sous 400 ; deux graisses au plus dans le cadre, en plus de celle des titres.
41. Texte de lecture : interligne de 1,4 à 1,5, 45 à 90 signes par ligne.

**Clavier et lecteur d'écran**
42. Dans une barre, un menu ou une liste, un élément désactivé reste atteignable (`aria-disabled`) pour que son
    « pourquoi » soit lisible au clavier.
43. Rien n'apparaît seulement au survol : ce que le survol montre, le focus le montre aussi.
44. Les annonces de l'activité des autres se réduisent à un résumé ou se coupent, et ne submergent jamais.

**Mouvement**
45. Une grande surface s'anime plus longtemps qu'une petite ; la sortie est plus rapide que l'entrée.
46. Ce qui entre par un bord sort par le même bord.
47. Rien ne saute : ni le changement de largeur, ni le curseur, ni le survol, ni le texte reçu ne déplacent le texte
    sans animation.

**Leçons des autres éditeurs**
48. Sans saisie, la palette montre d'abord les commandes récentes ; avec une saisie, la qualité de la correspondance
    passe d'abord, la récence départage les égales (EX0b2).
49. Le défilement « machine à écrire » n'est jamais actif par défaut ni pendant une sélection.
50. Un bouton à icône seule n'existe que dans les barres, pour une icône universelle (rechercher, annuler, rétablir,
    fermer, menu), avec infobulle et nom accessible ; toute autre commande montre son libellé ; la reconnaissance est
    vérifiée en séance.
51. La beauté se juge après la réussite des tâches, jamais à sa place.
52. Chaque surface a sa note « ce qu'Inkwell fait mieux » face à deux éditeurs nommés.

**Cohérence de l'ensemble** (un élément peut être bon seul et détonner parmi les autres ; la règle 14, P30, P2 et K-3 de
la spec de design le disent déjà, la machine le vérifie)
53. Une seule échelle : chaque espacement, taille, rayon, graisse, interligne et durée vient des jetons, et un même rôle
    a les mêmes valeurs sur tous les écrans.
54. Une même grammaire d'écran : barre d'en-tête, barre de vue, onglets, panneaux, tiroir et barre d'état occupent les
    mêmes régions, aux mêmes hauteurs, sur chaque écran principal ; une commande garde sa région d'un écran à l'autre.

Preuves : `docs/engineering-principles.md` §5, UF-13 à UF-26 (vérifications EL-1 à EL-14 du plan).
```

## 4. Proof of inspection

### 4.1 Two machine lists of elements

The list of elements is **produced by the machine, never written by hand**; the inspection records are checked against it.

1. **Static list** — `scripts/ux_elements.mjs` parses every `frontend/src/**/*.svelte` with the Svelte compiler's `parse`
   and lists each interactive or announcing element: `button`, `a[href]`, `input`, `select`, `textarea`,
   `[contenteditable]`, any `role` of the APG widget set (button, link, menuitem*, tab, switch, checkbox, radio, option,
   treeitem, slider, separator when focusable, combobox, gridcell), any `tabindex`, any element with a pointer or key
   handler, every landmark, `dialog`, `[popover]`, live region and tooltip. Each must carry a
   **`data-el="<surface>.<name>"`** attribute, unique in the source; a component used in several places gets its
   instance suffix from its parent (`data-el={`${parent}.share`}`). A missing or duplicated `data-el` fails the lint
   (`fe:check`, and `pregate` through EL-1). Output: `docs/ux/elements/static.json` (generated, committed so a diff shows
   every new element).
2. **Runtime list** — `frontend/e2e/ux/elements.spec.ts` drives every state of `docs/ux/elements/states.toml` (§4.3) and,
   in each, records every element that is in the accessibility tree and visible: its `data-el`, role and accessible name
   (from `locator.ariaSnapshot()`), bounding box, computed font size and weight, style classes, `data-cmd` (the registry
   command it runs) and `data-state-of` (the fact it shows). An accessible interactive node **without** `data-el` fails:
   this catches elements made at run time (lists, extension contributions, CodeMirror panels). CodeMirror's own content
   nodes are excluded by container, listed once in the states file with the reason. Output:
   `gates/UX1/elements/<state>.json` (raw, ignored) and `docs/ux/elements/runtime.json` (union per `data-el`: states where
   seen, committed).

### 4.2 The inspection record

One file per surface, `docs/ux/inventory/<surface>.toml`, one table per `data-el`:

| Field | Content |
|---|---|
| `surface`, `element`, `role` | from the machine lists (checked equal) |
| `task` | the task(s) of the surface it serves (`docs/ux/structure.md`, from UX1-A) — rule 11 |
| `position`, `why` | where it sits (region, bar and group, order) and the rule that puts it there (15–19, 54) |
| `rules` | the rule ids it must meet (ux-principles 1–54, spec A-/K-/L- ids) and the verdict for each |
| `neighbours` | elements it could duplicate on this and adjacent surfaces (rules 12–14), with the verdict |
| `states` | the states it was seen in (copied by the check from `runtime.json`) and how it looks in each: enabled, disabled and why, loading, error, offline, read-only role |
| `themes` | light, dark, high contrast: capture references and verdict |
| `keyboard` | how it is reached (Tab, arrows, F6 zone, shortcut) and its APG pattern |
| `name` | the screen-reader name and description heard (Orca pass) |
| `compare` | rule 52 note |
| `coherence` | one verdict per neighbour surface (EL-14) |
| `verdict` | keep, move (where), merge (with what), remove; reviewer and date; the e2e test or lint that now holds each machine-checkable finding |

The generators fill `surface`, `element`, `role`, `states`, `themes` (capture references), `keyboard` and `name` from
the machine lists; a person writes only `task`, `compare` and, for an element a check flags, `position`, `why`,
`neighbours`, `coherence` and the verdict. An element with every check green gets `verdict = "keep"` from the check
run, marked as such, and is not read by hand (owner decision, 2026-11-24; UX1-0 measures what this costs).

### 4.3 How the states are enumerated

`docs/ux/elements/states.toml` lists each surface's states, built from three inputs so none is forgotten:
- the state sources that exist when UX1-0 starts: the screens below, the state enums of the Svelte stores that drive
  what the interface shows as true (sync and save state of SI1b, presence, role, window state of spec §3.4), the
  dialogs and banners the shell opens, and the axes below; no XState machine exists in `frontend/src` today;
- every XState machine (UF-3) as the blocks that build them land (UF2 and after), read from the machine definitions: a
  new machine state with no entry in the file fails EL-1 (or carries `not_visual = "reason"`);
- every screen of spec §5 (5.1–5.19) and every surface of §4 that exists in code (popovers, dialogs, panes, bars);
- the axes common to all surfaces: content (empty, loaded, error), network (online, offline), people (alone, two:
  a second instance through the existing `instance.ts` helper), role (writing, reading), width (1280, 900, 560, 360 px),
  theme (light, dark, high contrast), motion (normal, reduced).

The full product of the axes is too large; the file holds an **all-pairs** covering of the axis values per surface
(every pair of values appears at least once), plus light and dark for every state, because K-3 asks both. The generator
`scripts/ux_states.py` writes the pairs; the spec reads them.

### 4.4 Per-element checks (proposed UF-13…UF-20; EL-1b is a non-blocking report, not a UF row)

| Check | What fails | Where |
|---|---|---|
| **EL-1 coverage** | a `data-el` in `static.json` or `runtime.json` without a complete record; a record whose element no longer exists; a record field empty or a rule id unknown; a machine state without an entry; a neighbour pair without a `coherence` verdict. **Coverage must be 100 %**, like `scripts/inventory.py` for open items | check 8 « ui-elements » of `scripts/inventory.py` (static, in `pregate`); runtime half in the UX1 gates and CI e2e |
| EL-1b usage | a default-visible element that no task scenario (`frontend/e2e/**`, UF-5 KLM tasks) ever activates is listed for review (rule 11); not blocking | `elements.spec.ts` report |
| **EL-2 duplicates** | in one state, two visible elements with the same `data-cmd` or the same `data-state-of`, unless the record names the rule that allows it; a status-bar command with no other surface | `elements.spec.ts` |
| **EL-3 identity on a surface** | an accessible name that does not start with the visible label; an icon-only name ≠ its tooltip name; one `data-cmd` with two names, icons or shortcuts across the states of one surface | `elements.spec.ts` |
| **EL-4 geometry** | proximity ratio < 2, misaligned leading edges, unequal widths in a button row, popover > 1/3 window, destructive adjacent to frequent, container wider than its maximum, reading line height or measure out of range | `elements.spec.ts` (boxes and computed styles) |
| **EL-5 budgets** | > 1 suggested or destructive button per view (B-1); > 3 chrome text sizes; weights outside the set; menus outside 3–12; bars with > 3 groups; non-flat styles in the header | `elements.spec.ts` + registry unit test for menus |
| **EL-6 layering** | a popover opened inside a popover, a dialog over a dialog, a dialog opened more than 1 s after the last user input | a probe in the overlay host, asserted in every spec through the common fixture |
| **EL-7 source lints** | banned words, placeholder-only inputs, icon-only buttons outside bars or outside the universal list, colour tokens outside their role, a « › » path outside a link | `fe:check` |
| **EL-8 stability** | a layout shift on the text column during resize across breakpoints, caret moves through links and images, hover, remote text; a hover-revealed element not shown on focus | `elements.spec.ts` (`PerformanceObserver('layout-shift')`, hover vs focus) |

### 4.5 Coherence checks (proposed UF-21…UF-26)

They compare elements **with each other, across all surfaces and states**, from the same machine lists plus the
registry, the i18n catalogues and the Svelte sources.

| Check | Proves | What fails | Where |
|---|---|---|---|
| **EL-9 registry identity** | rule 14 across surfaces (P20, P22, K-5, §8.3) | for every `data-cmd` seen anywhere, a rendered label, icon name, shortcut text (per system, spec §8.2) or tooltip text that differs from the registry entry; one icon for two commands; a label, icon or tooltip string written in a component for a control that has a `data-cmd` (static half) | `elements.spec.ts`, `fe:check` |
| **EL-10 vocabulary** | P30, spec §6.4, the glossary's « pair / personne », K-4 | every string of `fr.json` and `en.json` is scanned against the glossary and §6.4: a banned word or a known synonym of a concept (« historique » vs « versions », « fusionner » vs « intégrer », three verbs for going back) fails; two keys naming one concept with different words are listed | `fe:check` (reads `docs/ux/vocabulary.toml`, §8) |
| **EL-11 component reuse** | spec P2 and K-1 (S6, S7, UX-EX0-3, P23) | a `.svelte` file that renders a role's markers (banner: `role="status"` with an action row; dialog: `<dialog>` or `showModal`; search field: `type="search"`; empty state: heading, sentence and action in an otherwise empty region) without importing the role's component from `docs/ux/components.toml`; at run time, an element of such a role outside the component's root (`data-component`) | `fe:check`, `elements.spec.ts` |
| **EL-12 scale** | rule 53 (P1, P2, P3, P5, P6, P31; complements UF-4) | a computed `margin`, `padding`, `gap`, `font-size`, `font-weight`, `line-height`, `border-radius`, `height` or transition duration outside the scale; per role across surfaces, an outlier (a 36 px row among 46 px rows) listed with its surface | `elements.spec.ts` |
| **EL-13 regions** | rule 54 (spec §3.1, L-1…L-7, P9, P12, S7, D1, HB1 variant B, UF-8, G3) | each main surface declares `data-region="header|viewbar|tabs|left|main|drawer|status"`; at each width, a region whose top, height or side differs across surfaces (± 0.5 px), a different region order, a region absent without a reason in `states.toml`, a command in another region on another surface | `elements.spec.ts` |
| **EL-14 neighbours** | spec K-3 (and P32, rule 52) | for the surface under review and each neighbour (reachable in one gesture, from the XState graph), one sheet of captures at the same width and state, light and dark, aligned on their regions; EL-1 fails while a neighbour pair has no `coherence` verdict | `elements.spec.ts` renders the sheet; the reviewer records the verdict; last step before UX1-E |

### 4.6 Judging « useful in its context » and « in the whole »

- **In its context** (rule 11): the record's `task` names a task of this surface in this state (from
  `docs/ux/structure.md`), and EL-1b's usage data must not contradict it; an element used by no scenario goes one level
  down unless the reviewer writes why (rare but essential, for example « Quitter le mode développeur »).
- **In the whole** (rules 12–14, 52–54): EL-2, EL-3 and EL-9…EL-13 over the union of all states; the reviewer then reads
  the surface's records next to its neighbours (EL-14) and the product map (`docs/architecture/coherence.md` §4), and
  records merges.
- The review is made by Opus and another family (§2.3) with this rule list as checklist; every finding becomes a failing
  e2e test named by rule and element, then the fix; measured invariants go to the surface's reference (G3).

### 4.7 How a new element is caught, and how the checks are proved

A **new element added later** has no `data-el` (lint fails) or has one with no record (EL-1 fails in `pregate`); a new
state of a machine without an entry fails EL-1 too; an element made at run time without `data-el` fails the runtime walk.
Nothing reaches `main` uninspected. The UX1-0 gate seeds one defect per check (a duplicate command, a missing record, a
new element, a hand-written label, a banned synonym, a hand-rolled dialog, a 7 px gap, a moved view bar, a missing
neighbour verdict) and each must turn its check red.

## 5. Extensibility: how a later function or a plugin enters (W83)

A function added later, a forgotten one, or one brought by a plugin takes the same five steps; none is a special case.

1. **A registry point.** It is declared in the single contribution registry (ADR 0027, spec §4.31, architecture §15.4):
   file types, viewers and grammars (EX0, done); commands, palette and shortcuts (EX0b); menus, toolbars, panels and
   status items (EX0c). Its label, icon, shortcut and tooltip come from that entry (rule 14, EL-9), and it is in the
   palette at once (W84, UF-6).
2. **A reserved slot.** It enters at the lowest disclosure level that serves its task (palette and menu first; rule
   11), and moves up only with a task reason recorded in `docs/ux/structure.md`. The screen grammar of UX1-B reserves its
   places: primary, secondary and overflow ranks per region, and the extension slots of spec §4.31 (extension groups
   after the core's, ⋯ overflow beyond the budget, protected zones of §4.33 closed to it).
3. **Design-system components.** The host draws it with the components of UX1-C (`ui.view`), so tokens, states,
   keyboard pattern and accessible name are inherited; a free view (`ui.canvas`) runs in the frame Inkwell provides,
   with the active theme's tokens and its attribution line.
4. **The harness.** A contribution drawn by the host gets `data-el="<slot>.ext.<extension>.<command>"` from the registry,
   and is covered by the record of its slot template (`<slot>.contribution`): third-party contributions cannot have
   hand-written records, so the slot's record states the rules every instance meets, and EL-2…EL-13 run on every
   instance at run time. A core function gets its own record like any element (EL-1).
5. **Attribution.** A plugin's contribution shows where it comes from by the rule of its point (spec §4.31); this is
   the only difference from a core function.

A forgotten function is found by UX1-A (every command of the registry has a disclosure level on the map) and by UF-6;
a function that fits nowhere means the grammar lacks a slot: UX1-B is amended for every surface, never one surface
alone (§1.4).

## 6. Order of the surfaces

Each row is one UX1 sub-block, committed before the next. « Closes » lists what the sub-block finishes; the state of
each lives in `PROGRESS.md` and `docs/exigences.md`.

| Step | Surface or phase | Closes | PROGRESS block | Why here |
|---|---|---|---|---|
| UX1-0 | proof harness, on the shell | W82 (completeness proof), W83 (new elements caught), measuring half of G2 and G3 (W65, W66) | UX1, after EX0b | every step after it is measured with it |
| UX1-A | tasks and structure | W84 (disclosure levels), UF-5 and UF-6 budgets | UX1 | structure before skeleton |
| UX1-B | screen grammar and templates | P17, UX-EX0-3, S7 (grammar half), W83 (reserved slots) | UX1 | skeleton before surface |
| UX1-C | design system | P1–P6, P15, P22, P23, P28, S2, S6, G2 (references) | UX1 (these proposals were attached to UF1; UF-1 and UF-2 are built in UX1-0, UF-4 in UX1-C, UF1 keeps UF-8) | components before screens |
| UX1-1 | main window chrome: header bar, main menu, status bar | HB1a and HB1b (header variant B; W52, W53; P9, P11, P13, P18), P12, P14, P21, P31, W74 (window buttons), W75 (« Ouvrir un récent ») | UX1, with HB1 | seen all the time; built once as variant B, inspected as built |
| UX1-2 | text area and search bar | P19, U14 (SRC-11, SRC-20 out of `fixme`), P15 (applied), UX-PC1a-6 (reading position kept) | UX1 | the first task; the code exists |
| UX1-3 | messages: banners and toasts | P24, S2 (applied), UX-PC1a-3 (banner Tab order), UX-PC1a-7 (button wrap), the SB2a1 walkthrough findings (UX-SB2a1-6…8: retry without feedback, a toast without its reason, « Aucun problème » beside an error) | UX1 | rules 28, 30–32 |
| UX1-4 | files panel, file cards, view bar | UX-EX0-3, UX-EX0-6, P16 | UX1 | rules 12, 14, 34, 54 |
| UX1-5 | preferences, with the Fonts tab | N43 (FT-1…14), P29, P30 (EL-10 over all strings) | UX1 | rules 23, 29, 33, 35 |
| UX1-6 | share, join and people | the walkthrough findings filed → UX1 by PC1a, PC1b and PC1b2 (UX-PC1b2-3…8 among them), the second SI1b walkthrough | UX1 | rules 17, 24, 26, 28 |
| UX1-7 | first launch and recent projects | S1, W75 (welcome list) | UX1 | the first minute; needs the chrome and messages done |
| — | others' changes in the text, versions and differences, files and tabs | T, V, D, F, O; W55, W56, U11, U12, N39 | W65 | their own block, through UX1-D and UX1-E on each surface as built |
| — | preview, sheet, presentation, problems, export, outline, project search | S3, S4, S5, S8, S9, N44–N57 | S13, SH1, EF1 and the blocks of point 4 | same |
| — | after R1, per the W63 study | P7, P8, P10, P25, P27 | their blocks after R1 | same |

U31 (every function integrated, never bolted on) is the standing rule this plan implements; it is never closed. P20
(palette and shortcuts window) is built by EX0b, before UX1-0.

## 7. Decisions taken and open questions

Decisions, recorded in `docs/autonomous-decisions.md` (not copied here):
- 2026-11-24, « UX1 question 1, dialog button order → UX1 »;
- 2026-11-24, « UX1 question 2, icon-only buttons → UX1 »;
- 2026-11-24, « UX1 question 3, unavailable commands → UX1 »;
- 2026-11-24, « UX1 question 4, order → UX1 »;
- 2026-11-24, « UX1 order: from the whole to the element → UX1 »;
- 2026-11-24, « UX1 plan consolidated → UX1 » (one plan; the duplicates of §3 removed; P1–P6, P15, P22, P23, P28, S6
  to UX1-C; HB1 built as UX1-1; P20 in EX0b).

Owner decisions this plan applies: Adwaita precedence (§1.3, 2026-11-24); the two studies (2026-11-14, P26 excepted).

Amendments the plan makes to other texts, each in its phase: spec §4.1 and H-2 (P11, P12) in UX1-C; spec §3 (regions
and templates) in UX1-B; `docs/ux-principles.md` (§3.12) when UX1-0's gate is frozen. Done on 2026-11-24: spec §4.8 and §4.9
(button order), rules 2 and 9 of `docs/ux-principles.md` (pointers to spec P1 and A-7).

Open questions, each with its block:
- **The interface across versions**: moving a control people know (the old place points to the new one for one
  version), migrating preferences when a setting is renamed, split or removed (never silently reset), and a « what's
  new » that follows rule 32 (once, never promotional). Settled in UX1-B with the templates → UX1.
- **Upkeep of the Adwaita tokens** when libadwaita changes: `tokens.json` records the libadwaita version it was read
  from; each libadwaita release is diffed against it and the change goes through spec §2 → AW1.
- **Right-to-left and other languages**: spec §11 Q7 holds the decision and its reopening trigger → UX1.

## 8. Maintenance

**One home per piece.**

| Piece | Its single home | Updated by, and when |
|---|---|---|
| the rules 11–54 | their one wording is §3.12 (French), pasted into `docs/ux-principles.md`; the tables of §3.1–§3.10 keep only number, source, reason and proof | the integrator, when UX1-0's gate is frozen (the rules get their proofs then) |
| a new rule after that | `docs/ux-principles.md` only, with its source and a proof row in `docs/engineering-principles.md` §5; never in this plan | whoever proposes it, through review |
| the proofs EL-1…EL-14 | `docs/engineering-principles.md` §5 (UF-13…UF-26, one line each), code in the files of §4 | UX1-0, when its gate is frozen |
| tokens | `docs/design/tokens.json` and `frontend/src/theme/tokens.css` (spec §2 describes them) | UX1-C, then any change through the spec |
| components | spec §4 (behaviour), `docs/ux/components.toml` (role → component, read by EL-11) | UX1-C, then each new component |
| words | `docs/glossary.md` and spec §6.4 (the words); `docs/ux/vocabulary.toml` holds only the banned synonyms of each concept, keyed by the glossary entry | with each new concept |
| task ranking and screen map | `docs/ux/structure.md` | UX1-A, then each new function (§5, step 2) |
| the list of functions (W85) | `docs/ux/functions.toml`; `docs/architecture/coherence.md` §4–§5 and spec §5 point to it | UX1-A, then each new requirement (its node is written with its row) |
| light and dark review next to the neighbour screens | the rule: spec K-3; its check: EL-14, UF-26 of `docs/engineering-principles.md` §5; `CLAUDE.md` « Cohérence de l'interface » points to both | — |
| element lists and states | `docs/ux/elements/` (generated, committed) | the generators, at every change |
| element records | `docs/ux/inventory/<surface>.toml` | each UX1-D sub-block's « done », then every change that adds an element (EL-1 in `pregate`) |
| owner sessions | `docs/ux/sessions/<date>-<surface>.md`; the rule (grouped by wave, blocks R1 not `main`): `CLAUDE.md` step 4 | UX1-E |
| decisions | `docs/autonomous-decisions.md` | when taken |
| state of each step | `PROGRESS.md` point 4, `docs/exigences.md` | each sub-block's « done » (step 7 of the definition) |
| this plan | method and order only | when the order changes |

`docs/README.md` gets a row for `docs/ux/elements/`, `docs/ux/inventory/`, `docs/ux/sessions/`,
`docs/ux/structure.md`, `docs/ux/functions.toml`, `docs/ux/vocabulary.toml` and `docs/ux/components.toml` when UX1-0,
UX1-A and UX1-C create them. The hand-written lists of `docs/design/inventory.md` stop being
updated at UX1-0 and the file moves to `docs/archive/` when UX1-7 is done → UX1.

**Retirement.** UX1 ends when UX1-7 has passed UX1-E. Then the status line of this plan becomes « done » with the date;
§3 has already moved to `docs/ux-principles.md`; the harness stays as a permanent guard (EL-1 in `pregate`, EL-2…EL-14 in
`fe:check` and the e2e suites), so the blocks that follow (W65, S13 and the rest of §6) inspect their surfaces with the
same records and checks without this plan.
