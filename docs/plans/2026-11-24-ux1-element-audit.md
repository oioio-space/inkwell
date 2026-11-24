# UX1 element audit (W82): one rule set for every element, and a proof that every element was inspected

Status: **proposal, 2026-11-24**, for the owner. Request W82 (« une interface extrêmement soignée » : every element follows
the written rules and the best rules of the GNOME HIG, Apple's HIG and other strong references; well placed, useful in its
context and in the whole, duplicating nothing, convivial and beautiful), widened the same day (« les meilleurs principes de
l'UX/UI dans leur ensemble, en restant dans le style Adwaita » ; « chaque élément soit inspecté. Chaque élément. »).

Builds on, and does not repeat: `docs/ux-principles.md` (rules 1–10), `docs/engineering-principles.md` §5 (UF-1…UF-12),
`docs/design/spec.md` (P1–P7, A-1…A-22, K-1…K-7, L-1…L-10, N-1, N-2, §4 states and focus return, §6 voice, §9
anti-patterns), the two approved studies `2026-11-14-ux-ui-study.md` (P1–P32) and `2026-11-14-ux-diff-versions-files.md`
(T, D, V, F, O, S, G), and their source digests `docs/ux/sources/2026-11-14-{gnome,craft}.md`. A rule below is kept only if
none of those already states it; where one states part of it, the row says « extends ».

**Precedence** (owner, 2026-11-24): GNOME HIG and libadwaita are the visual language; every other source only fills what
they leave unsaid and never overrides them. The spec's arbitration (P5 and P6 invariant, then P4 > P3 > P2 > P1 > P7) is
unchanged; Adwaita yields only to accessibility, as it already does for the focus ring (spec §2.2).

**Sources read on 2026-11-24** (fetched pages; sections named in each row). GNOME HIG: principles, header bars, buttons,
menus, popovers, dialogs, writing style, accessibility, adaptive (<https://developer.gnome.org/hig/>); libadwaita style
classes (<https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/style-classes.html>); Apple HIG layout, colour,
typography, motion, toolbars, context menus, writing (read through the HIG's JSON data endpoint, same text as
<https://developer.apple.com/design/human-interface-guidelines/>); Fluent 2 layout and motion
(<https://fluent2.microsoft.design/>); Windows dialog controls (<https://learn.microsoft.com/windows/apps/design/controls/dialogs-and-flyouts/dialogs>);
Android canonical layouts, derived from Material 3 (<https://developer.android.com/develop/ui/compose/layouts/adaptive/canonical-layouts>);
NN/g: ten heuristics (updated 2024-01-30), progressive disclosure, visual hierarchy, Gestalt proximity, Fitts's law, icon
usability, empty states, error messages, form design, indicators/validations/notifications, aesthetic-usability effect;
Laws of UX (<https://lawsofux.com/>); Butterick, *Typography in ten minutes*; WCAG 2.2 Understanding 2.5.3 and 3.2.4; APCA
in a nutshell; WAI-ARIA APG keyboard interface; VS Code UX guidelines (status bar, notifications, command palette); iA
Writer focus mode; Obsidian command palette; Notion shortcuts. **Limits:** Material 3 pages render in JavaScript and
returned no text (only the Android page above, and the older material.io density scale through a search snippet); Typora's
help returned 404, so its live-preview flaw rests on one forum quote of Daring Fireball (marked « weak »); Google Docs « Live
edits » comes from search results and Google's 2019 announcement, not a fetched help page; Bringhurst's book was not read
(Butterick stands in).

## 1. The rule set

Numbering continues `docs/ux-principles.md` (rules 1–10): rules **11–58** (53–58, coherence of the whole, are in §2.7). « Proof » names one of the machine checks
EL-1…EL-8 of §2.4 or EL-9…EL-14 of §2.7 (each becomes a UF row of `docs/engineering-principles.md` §5), a lint, or « review » (the element
record of §2.2 carries the verdict, then R14).

### 1.1 Presence: every element earns its place

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 11 | Every visible element serves a named task of its screen in that state; one that serves none goes one level down (menu, palette) | GNOME principles « Make it simple »: avoid showing too many elements at once; NN/g heuristic 8; NN/g progressive disclosure: « disclose everything that users frequently need up front » | W82 « utile dans son contexte »; today nothing asks why an element is there | EL-1 (record field `task`), EL-1b (task-scenario usage), review |
| 12 | **One command, one visible control per screen state.** Keyboard, palette and the macOS menu bar do not count; a context menu repeats only what the main interface has | Apple toolbars, macOS: « it doesn't make sense to provide a toolbar item for every menu item »; Apple context menus: « Always make context menu items available in the main interface, too »; Hick's law | W82 « sans rien dupliquer »; extends P2, K-5 and P12 (which cover verbs, shortcuts and the sync state) to every control | EL-2 |
| 13 | **One fact, one place:** a state (sync, unsaved, role, problems, who is here) is shown by one element per screen state | extends P12 of the W63 study to every state; NN/g indicators are contextual: they sit on the element they describe | three « unsaved » signals and two sync places were measured on 2026-11-14 | EL-2 (`data-state-of`) |
| 14 | Same function, same identity everywhere: one command → one label, one icon, one shortcut on every screen; one icon → one meaning | WCAG 3.2.4 (different icons for one function is a listed failure); Apple colour « Avoid using the same color to mean different things » applied to icons; GNOME buttons: adjacent buttons share a width | extends §8.3 « icônes par idée » from a table to a check | EL-3 |
| 15 | Placement by role in a bar: start = navigation and panes, centre = title and state, end = menus and the openers of the drawer; one primary action at most, at the end | GNOME header bars: three alignment points, menus at the end; Apple toolbars: trailing items remain visible at all widths, « Only specify one primary action » | the header carries title, state, people, Partager, Extensions, menu: their order needs a reason, not habit | EL-4 (order of `data-el` per bar), reference invariants (G3) |
| 16 | Nothing critical lives only in the status bar | Apple layout, macOS: « Avoid placing controls or critical information at the bottom of a window »; VS Code status bar: short labels, no custom colours | the status bar keeps four items (§4.17); this says which may go there | EL-2 (a status-bar command must have another surface) |
| 17 | Fitts: frequent targets large and next to the content they act on; a popover or menu opens at its trigger; a destructive control is never adjacent to a frequent one | NN/g Fitts's law: « make targets big », options « close to the label », overshoot risk | « Retirer » next to « Déconnecter » in the people list is one slip away | EL-4 (gap ≥ one control width or another group), review |

### 1.2 Grouping, alignment, hierarchy

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 18 | **Proximity encodes grouping:** the gap between groups is at least twice the largest gap inside a group (on the 6 px grid: 6 inside, 12–18 between); a label sits nearer its control than any other control | NN/g Gestalt proximity: proximity « can overpower competing visual cues »; Fluent 2 layout: « Space is used to denote groups »; GNOME header bars: group by spacing rather than linked buttons | P2 fixed the grid, not how it is used | EL-4 |
| 19 | Siblings share a leading edge; indentation only for subordination; counters and times use tabular figures | Apple layout « Align elements… use indentation to convey hierarchy »; libadwaita `.numeric` | the version rows, file rows and status bar mix edges today | EL-4 (edges ± 0.5 px), lint (`tabular-nums` on counters) |
| 20 | **Hierarchy budget per view:** at most one suggested or destructive button; at most three text sizes in the chrome (the document excluded); one focal point, checked by the squint (blur) test | GNOME buttons: « Each view should only ever include a single button using either the suggested or destructive styles »; NN/g visual hierarchy: no more than three sizes, the squint test | P1 reduced the scale to five styles; this caps how many meet in one view | EL-5, review (blurred capture) |
| 21 | Buttons side by side have the same width; at most two button widths per window; outside bars a button has an icon or a label, not both | GNOME buttons (« same width », « more than one or two different widths », « either an icon or a label ») | dialog footers have uneven widths; UX-PC1a-7 is uneven wrapping of two buttons | EL-4 |
| 22 | Header-bar buttons are flat, icon-only with a tooltip; text-only, suggested, destructive and linked styles are avoided there (action dialogs excepted) | GNOME header bars (quoted rule, re-read); libadwaita: buttons in toolbars are flat by default | extends P11 (Partager) to every header control | EL-5 (style classes per bar) |
| 23 | Containers keep a comfortable width at any window width: dialogs, preferences, empty states and lists have a maximum width and are centred | GNOME adaptive: « containers that have a maximum width »; Android supporting pane: secondary pane about one third, meaningful only beside the main one | P15 caps the text only; at 1920 px a preferences row runs edge to edge | EL-4 at 1920 and 360 px |

### 1.3 Disclosure: menus, popovers, dialogs

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 24 | **Two levels of disclosure at most:** visible → one gesture (popover, pane, menu) → dialog. Never a popover in a popover, a submenu in a submenu, or a dialog on a dialog | NN/g progressive disclosure: beyond two levels « typically have low usability »; GNOME menus: « Don't nest submenus »; Windows: one dialog per window at a time | D1 removed one stacked modal; this forbids the class | EL-6 |
| 25 | Menus hold 3–12 items (submenus 3–6), grouped by kind, ordered by task or frequency; verbs for commands, adjectives for settings; the standard group last | GNOME menus | extends P14 (main menu order) to every menu, insert and format menus included | EL-5 (registry menu definitions) |
| 26 | A popover covers at most a third of the window, mixes few control types, has a heading only when its purpose is ambiguous, no Close or Done button, Esc closes it | GNOME popovers | the share and people popovers grow with content | EL-4, EL-6 |
| 27 | Context menus: only the most likely commands for that object, the most frequent nearest the pointer; offered on every object kind or none; unavailable items hidden, not dimmed | Apple context menus (relevance, consistency, « Hide unavailable menu items, don't dim them ») | file rows, tabs, version rows, people rows and the text get menus at different times (P27, F3, V4) | EL-5 (each object kind has one), EL-3 |
| 28 | A dialog appears only in immediate answer to a deliberate gesture; it has 1–3 buttons, the affirmative one a specific verb (never « OK »), Enter on it unless none fits, Esc on cancel | GNOME dialogs (« Never pop up a dialog window unexpectedly », specific imperative verb, Enter and Esc) | the question from another person is a banner (L-8); this makes the boundary checkable | EL-6 (dialog opened > 1 s after the last input event fails) |
| 29 | Choices beyond about seven are grouped or searchable | Hick's law; Miller's law, read as chunking (Laws of UX) | font, language and template lists grow (N43) | lint (option groups > 7 without sections or filter) |

### 1.4 Messages, errors, forms

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 30 | The kind of message chooses the component: **indicator** on the element (passive, conditional), **validation** next to the field, **notification** as toast (passive), banner (state) or dialog (only when a response is essential) | NN/g indicators, validations, notifications; GNOME dialogs | extends spec §3.5 (three kinds) with the indicator, the most frequent kind in a collaborative editor (faces, dots, marks) | review, EL-6 |
| 31 | An error sits next to its source, keeps what was typed, says the remedy, never blames (« invalide », « incorrect », « erreur de votre part » banned) | NN/g error messages; Apple writing « Errors »; Windows: no dialog for an error contextual to a place | extends rule 5 (« chaque message dit quoi faire ») with placement and input preservation | lint (i18n words), e2e: value kept and `aria-describedby` after a refused input |
| 32 | The same message is not shown again while its cause has not changed; never a message to promote or to ask for feedback | VS Code notifications (« Send repeated notifications », « Use for promotion » listed as don'ts) | seven banner sources (S2); reconnecting networks repeat | unit test of `ToastHost`/`BannerHost` (same key, same cause ⇒ one) |
| 33 | Forms: one column, label above or beside, never a placeholder as label, field width close to the expected input, optional fields marked « facultatif », the cancel button visibly lighter | NN/g form design | join, share, preferences and templates are forms | lint (placeholder without label), EL-4 (field width), review |
| 34 | Empty states, already P23, also say what will appear here and are never the only place of crucial information | NN/g empty states; Apple writing « Empty States » (« usually temporary ») | completes P23 | review |

### 1.5 Words

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 35 | A setting's label says what it does when on; to send someone to a setting, give a button that opens it, never its path in words | Apple writing « Settings Labels » | « Réglable dans Préférences › Écriture » strings exist | lint: a string containing « › » is rendered inside a link or button (e2e) |
| 36 | Labels short; « … » only when more input follows (spec §6.2); no Latin abbreviation (« ex. », « etc. ») in labels | GNOME writing style | completes spec §6.2 | i18n lint |
| 37 | The accessible name starts with the visible label and stays short; an icon-only control's name is its tooltip's name | WCAG 2.5.3 and its best practice « the text of the label at the start of the name »; GNOME accessibility: names « short and descriptive » | 2.5.3 is open in `coverage.md` (G-31) | EL-3 |

### 1.6 Colour and type

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 38 | **Colour keeps one meaning:** accent = interactive or selected; error tokens = error or destructive; people colours = people only; no colour for decoration | Apple colour (« same color to mean different things »); NN/g visual hierarchy (warm red reserved for warnings); VS Code status bar (no custom colours) | extends P7 (« ni couleur sans rôle ») to a token-to-role map | lint: each colour token allowed only in its role's selectors (extends UF-4) |
| 39 | APCA is reported beside the WCAG ratio for every token pair (Lc 75 body, 60 chrome labels, 45 large, 30 solid icons), as information that chooses between two passing pairs; WCAG 2.2 stays the gate | APCA in a nutshell (candidate method for WCAG 3) | dark-mode pairs that pass 4.5:1 can still read thin | `theme/tokens.test.ts` prints Lc (non-blocking) |
| 40 | No weight under 400; at most two weights in the chrome besides P1's title weight | Apple typography « avoid light font weights », « Minimize the number of typefaces » | Adwaita Sans ships light weights | EL-5 (computed weights) |
| 41 | Reading text (preview, long panels) at 1.4–1.5 line height and 45–90 characters | Butterick: 120–145 % line spacing, 45–90 characters | gives numbers to « interligne augmenté » and joins P15 | EL-4 (computed style) |

### 1.7 Keyboard and assistive technology

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 42 | Inside a composite (toolbar, menu, list), a disabled item stays focusable with `aria-disabled`, so its « why » (spec §4) is reachable by keyboard; one convention everywhere | WAI-ARIA APG keyboard interface (« consistent pattern-based conventions for the focusability of disabled elements ») | the « why » tooltip is mouse-only today | unit tests per composite (UF-12) |
| 43 | **Nothing appears only on hover:** whatever hover reveals, focus reveals too, and it has a keyboard path | NN/g icon usability « Don't rely on hover to reveal text labels »; Notion's hover handles as the counter-example | T1–T3 margin marks, row actions (V4) and the preview pill (P9) are hover-revealed | EL-8 (each hover-revealed element is also shown on focus) |
| 44 | Announcements of others' activity can be turned down to a summary and off, and never flood | Google Docs « Live edits » summary sidebar and its own warning that many suggestions flood the reader (search results; weak) | completes T9 and A-10 | `ui/messages.test.ts`, Orca pass |

### 1.8 Motion and adaptivity

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 45 | Larger surfaces animate longer than small ones (pane ≥ popover ≥ tooltip); exit faster than entry (P5) | Fluent 2 motion: « Give larger elements more time to animate than smaller elements » | P5 gave one curve, not the scale | unit test on duration tokens |
| 46 | What enters from an edge leaves by the same edge | Apple motion, « Providing feedback » | panes, drawer, search bar | review (captures of both transitions) |
| 47 | **Nothing jumps:** resizing across a breakpoint, moving the caret, hovering or receiving text never shifts the text area without an animation | GNOME adaptive: « widgets should not jump around or disappear without an animation »; Typora's markup that « suddenly appears » when the caret enters a link (forum quote, weak) | extends T4 (remote text) and P5 (banners) to local causes | EL-8 (layout-shift observer = 0 on the text column) |

### 1.9 What competing editors teach

| # | Rule | Source | Why for Inkwell | Proof |
|---|---|---|---|---|
| 48 | The palette lists recent commands first and shows every shortcut | Obsidian command palette; VS Code palette guidelines (clear names, categories, never take an existing shortcut) | completes P20 | unit test of the palette ranking |
| 49 | Typewriter scrolling is never on by default and never acts while selecting | iA Writer focus mode: its own warning that centring « might result in the screen jumping » during editing | P10 focus mode | e2e (selection does not scroll) |
| 50 | Icon-only controls are limited to the header bar and toolbars, use only icons of spec §8.3, and each is checked for recognition in R14 (five-second rule) | GNOME header bars (icon-only with tooltips) **over** NN/g icon usability (labels needed), see conflicts | Inkwell's icons are mostly not universal (NN/g's test found a clock for « history » unrecognised by every participant; Inkwell uses `clock` for Versions) | lint (icon-only outside bars), R14 |
| 51 | Beauty is judged after task success, never instead: R14 asks for task completion and SEQ first, aesthetics last | NN/g aesthetic-usability effect: a pretty design « masked its usability issues » | W82 asks « beau »; this keeps it from hiding defects | R14 protocol (§2.6) |
| 52 | Every surface has a « what Inkwell does better » note against two named editors (Google Docs, Obsidian, iA Writer, Typora, Notion, VS Code): kept good patterns cited, known faults avoided | owner's quality bar; the rows above | turns competitive reading into a checked field | EL-1 (record field `compare`) |

### 1.10 Conflicts between guides, and the choice

| Topic | Disagreement | Choice | Reason |
|---|---|---|---|
| Dialog button order | GNOME: « cancel button appears first, before the affirmative »; Apple agrees (affirmative trailing); Windows: « do it » leftmost, Close rightmost | the host system's order (Windows' on Windows, Adwaita's on Linux and macOS), the Adwaita look everywhere (decided 2026-11-24, docs/autonomous-decisions.md) | spec §8.4: the drawing stays Adwaita, what is a reflex follows the system, as the window buttons do; button order is a reflex (Jakob's law) |
| Icon-only vs labels | GNOME header bars and Apple toolbars prefer symbols; NN/g: a label must accompany icons | rule 50 | precedence of GNOME; NN/g's risk is handled by tooltips (P22) and a recognition check |
| Disabled vs hidden | Apple: hide unavailable menu items; spec §4: dim with a « why » | hide in context menus; dim + why in bars and the main menu (rule 42) | transient lists stay short; permanent places must not move under the pointer (rule 47) |
| Grid | Fluent 4 px, Material and SAP 4/8 dp, libadwaita 6 px | 6 px (P2) | precedence of Adwaita |
| Targets | Fluent and Apple 44 px, libadwaita 34 px controls | 34 px, ≥ 24 px, 44 px for touch (P31, A-7) | precedence of Adwaita; WCAG 2.5.8 met |
| Breakpoints | Fluent six classes, Android 600/840 dp, Adwaita 400/550/860 sp | spec §3.4 (A6 plan) | already decided, rounded from Adwaita |
| « You » and full stops | GNOME (English): avoid « you », no period at the end of text | French: vouvoiement and one full sentence in banners and toasts (spec §6.1–6.2); no period on buttons and menu items | French desktop software, GNOME's French translations included, addresses the person as « vous » (to re-check on the translation files); a sentence ends with a period in French |
| Status at the bottom | Apple macOS: no critical information at the bottom | status bar kept, nothing critical only there (rule 16) | GNOME has no such rule; the status bar holds only facts available elsewhere |

## 2. Element-by-element audit method

### 2.1 Two machine lists of elements

The list of elements is **produced by the machine, never written by hand**; the inspection records are checked against it.

1. **Static list** — `scripts/ux_elements.mjs` parses every `frontend/src/**/*.svelte` with the Svelte compiler's `parse`
   and lists each interactive or announcing element: `button`, `a[href]`, `input`, `select`, `textarea`, `[contenteditable]`,
   any `role` of the APG widget set (button, link, menuitem*, tab, switch, checkbox, radio, option, treeitem, slider,
   separator when focusable, combobox, gridcell), any `tabindex`, any element with a pointer or key handler, every
   landmark, `dialog`, `[popover]`, live region and tooltip. Each must carry a **`data-el="<surface>.<name>"`** attribute,
   unique in the source; a component used in several places gets its instance suffix from its parent
   (`data-el={`${parent}.share`}`). A missing or duplicated `data-el` fails the lint (`fe:check`, and `pregate` through the
   check below). Output: `docs/ux/elements/static.json` (generated, committed so a diff shows every new element).
2. **Runtime list** — `frontend/e2e/ux/elements.spec.ts` drives every state of `docs/ux/elements/states.toml` (§2.3) and,
   in each, records every element that is in the accessibility tree and visible: its `data-el`, role and accessible name
   (from `locator.ariaSnapshot()`), bounding box, computed font size and weight, style classes, `data-cmd` (the registry
   command it runs) and `data-state-of` (the fact it shows). An accessible interactive node **without** `data-el` fails
   the spec: this catches elements made at run time (lists, extension contributions, CodeMirror panels). CodeMirror's
   own content nodes are excluded by container, listed once in the states file with the reason. Output:
   `gates/UX1/elements/<state>.json` (raw, ignored) and `docs/ux/elements/runtime.json` (union per `data-el`: states where
   seen, committed).

### 2.2 The inspection record

One file per surface, `docs/ux/inventory/<surface>.toml`, one table per `data-el`:

| Field | Content |
|---|---|
| `surface`, `element`, `role` | from the machine lists (checked equal) |
| `task` | the task(s) of the surface it serves (spec §5 tasks, the five tasks of `docs/ux-principles.md`) — rule 11 |
| `position`, `why` | where it sits (bar and group, order) and the rule that puts it there (15–19) |
| `rules` | the rule ids it must meet (ux-principles 1–52, spec A-/K-/L- ids) and the verdict for each |
| `neighbours` | elements it could duplicate on this and adjacent surfaces (rules 12–14), with the verdict |
| `states` | the states it was seen in (copied by the check from `runtime.json`) and how it looks in each: enabled, disabled and why, loading, error, offline, read-only role |
| `themes` | light, dark, high contrast: capture references and verdict |
| `keyboard` | how it is reached (Tab, arrows, F6 zone, shortcut) and its APG pattern |
| `name` | the screen-reader name and description heard (Orca pass) |
| `compare` | rule 52 note |
| `verdict` | keep, move (where), merge (with what), remove; reviewer and date; the e2e test or lint that now holds each machine-checkable finding |

### 2.3 How the states are enumerated

`docs/ux/elements/states.toml` lists each surface's states, built from three inputs so none is forgotten:
- every state of every XState machine (UF-3), read from the machine definitions: a machine state with no entry in the
  file fails the check (or carries `not_visual = "reason"`);
- every screen of spec §5 (5.1–5.19) and every surface of §4 that exists in code (popovers, dialogs, panes, bars);
- the axes common to all surfaces: content (empty, loaded, error), network (online, offline), people (alone, two:
  a second instance through the existing `instance.ts` helper), role (writing, reading), width (1280, 900, 560, 360 px),
  theme (light, dark, high contrast), motion (normal, reduced).
The full product of the axes is too large; the file holds an **all-pairs** covering of the axis values per surface
(every pair of values appears at least once), plus light and dark for every state, because rule K-3 asks both. The
generator `scripts/ux_states.py` writes the pairs; the spec reads them.

### 2.4 The per-element checks (proposed UF-13…UF-20; the whole: §2.7)

| Check | What fails | Where |
|---|---|---|
| **EL-1 coverage** | a `data-el` in `static.json` or `runtime.json` without a complete record; a record whose element no longer exists; a record field empty or a rule id unknown. **Coverage must be 100 %**, like `scripts/inventory.py` for open items | new check 8 « ui-elements » of `scripts/inventory.py` (fast, static, in `pregate`); runtime half in the UX1 gate and CI e2e |
| EL-1b usage | a default-visible element that no task scenario (`frontend/e2e/**`, UF-5 KLM tasks) ever activates is listed for review (rule 11); not blocking | `elements.spec.ts` report |
| **EL-2 duplicates** | in one state, two visible elements with the same `data-cmd` or the same `data-state-of`, unless the record names the rule that allows it; a status-bar command with no other surface | `elements.spec.ts` |
| **EL-3 identity** | one `data-cmd` with two names, icons or shortcuts across states; one icon for two commands; an accessible name that does not start with the visible label; an icon-only name ≠ its tooltip name | `elements.spec.ts` + registry unit test |
| **EL-4 geometry** | proximity ratio < 2, misaligned leading edges, unequal widths in a button row, popover > 1/3 window, destructive adjacent to frequent, container wider than its maximum, reading line height or measure out of range | `elements.spec.ts` (boxes and computed styles) |
| **EL-5 budgets** | > 1 suggested or destructive button per view; > 3 chrome text sizes; weights outside the set; menus outside 3–12; bars with > 3 groups; non-flat styles in the header | `elements.spec.ts` + registry unit test for menus |
| **EL-6 layering** | a popover opened inside a popover, a dialog over a dialog, a dialog opened more than 1 s after the last user input | a probe in the overlay host, asserted in every spec through the common fixture |
| **EL-7 source lints** | banned words, placeholder-only inputs, icon-only buttons outside bars, colour tokens outside their role, a « › » path outside a link | `fe:check` |
| **EL-8 stability** | a layout shift on the text column during resize across breakpoints, caret moves through links and images, hover, remote text; a hover-revealed element not shown on focus | `elements.spec.ts` (`PerformanceObserver('layout-shift')`, hover vs focus) |

A **new element added later** has no `data-el` (lint fails) or has one with no record (EL-1 fails in `pregate`); a new
state of a machine without an entry fails EL-1 too. Nothing reaches `main` uninspected.

`data-cmd` needs the command half of the EX0 registry (commands, menus, shortcuts; today the registry holds file types,
viewers and grammars): it is the first dependency of UX1.

### 2.5 Judging « useful in its context » and « in the whole »

- **In its context** (rule 11): the record's `task` must name a task of this surface in this state, and EL-1b's usage
  data must not contradict it; an element used by no scenario is moved down a level unless the reviewer writes why
  (rare but essential, for example « Quitter le mode développeur »).
- **In the whole** (rules 12–14, 52): EL-2 and EL-3 over the union of all states; the reviewer then reads the surface's
  records next to its neighbours (the drawer next to the header, the files panel next to tabs) and the coherence map
  (`docs/architecture/coherence.md` §5), and records merges.
- The review is made by Opus or Codex with this rule list as checklist (Sonnet only fills machine fields), as CLAUDE.md
  requires; every finding becomes a failing e2e test named by rule and element (`UXR-20 header single suggested`), then
  the fix, as the `ux-walkthrough` skill already does; measured invariants go to the surface's reference (G3).

### 2.6 Then the R14 session, surface by surface

A surface is « inspected » only when all its records have a verdict, EL-1…EL-8 are green on it, and the owner has seen it
in a short session (R14), after the neighbours sheet of EL-14: the tasks of that surface first, timed, with SEQ; then the five-second recognition of its icons
(rule 50); then light and dark captures next to the reference (UF-8); aesthetics last (rule 51). Notes go to
`docs/ux/sessions/<date>-<surface>.md`, decisions to `docs/autonomous-decisions.md` or the reference.

### 2.7 Coherence of the whole

An element can pass every rule above and still disagree with its neighbours (owner, 2026-11-24: « chaque élément peut
être bien mais pas cohérent avec les autres »). These checks compare elements **with each other, across all surfaces and
states**, from the same machine lists (`static.json`, `runtime.json`) plus the registry, the i18n catalogues and the
Svelte sources. Rules 53–58 state them; EL-9…EL-14 prove them (proposed UF-21…UF-26).

| Rule | Check | What fails | Source | Covers |
|---|---|---|---|---|
| 53. **One command, one identity on every surface**: label, icon, shortcut and tooltip come from the registry entry and nowhere else | **EL-9** registry identity — for every `data-cmd` seen anywhere, the rendered label, icon name, shortcut text and tooltip text equal the registry entry (per system for shortcuts, spec §8.2); a label, icon or tooltip string written in a component for a control that has a `data-cmd` fails the static half | a hand-written « Partager » next to the registry's; a tooltip whose shortcut differs from the menu's | WCAG 3.2.4; Apple writing « Consistency of terms »; NN/g heuristic 4 | P20, P22 (one registry feeds menus, tooltips, palette), K-5, §8.3; extends EL-3 from one surface to all |
| 54. **One concept, one word everywhere** | **EL-10** vocabulary — the glossary (`docs/glossary.md`) and spec §6.4 become a machine table (`docs/ux/vocabulary.toml`: concept, the word, its forms, the banned synonyms); every string of `fr.json` and `en.json` is scanned: a banned word or a known synonym of a concept (« historique » vs « versions », « fusionner » vs « intégrer », three verbs for going back) fails, and two keys whose texts name the same concept with different words are listed | « Fusionner » in a button, « Historique » and « Versions » for one drawer | Apple writing « Create a list of common terms »; NN/g heuristic 4 | P30, §6.4, glossary rule « pair/personne », K-4, H-HON, §4.45 word list |
| 55. **One role, one component**: two banners, two search fields, two empty states, two dialogs, two menus are drawn by the same component | **EL-11** component reuse — a table of roles (`docs/ux/components.toml`: role → the one component, from spec §4) is checked statically: a `.svelte` file that renders the role's markers (for a banner, `role="status"` with an action row; for a dialog, `<dialog>` or `showModal`; for a search field, `type="search"`; for an empty state, a heading + sentence + action in an otherwise empty region) without importing that component fails; at run time, every element whose ARIA role belongs to the table must sit inside the component's root (`data-component`) | a dialog with its own CSS (spec §9), the sheet's own find bar, a hand-made empty card | GNOME HIG UI styling « Keep custom styling to a minimum »; libadwaita style classes (one style per role) | P2 « un besoin = un composant », P3 and S6 (one Dialog, one Button, one reference menu), S7 (sheet on the shell's bars), UX-EX0-3 (one view bar), P23, K-1 |
| 56. **One scale**: every computed spacing, size, radius, font size, weight, line height and duration belongs to the token scale, per role | **EL-12** scale — the runtime walk records computed `margin`, `padding`, `gap`, `font-size`, `font-weight`, `line-height`, `border-radius`, `height` and transition durations of every `data-el`; a value outside the scale (6 px grid, P1 type styles, P3 radii, A-7 heights, P5 durations) fails; then, **per role across surfaces** (all header buttons, all list rows, all dialog titles), values must be equal: an outlier (a 36 px row among 46 px rows, a dialog title at 14.5 px among titles) is listed with its surface | today's 13 font sizes and 709 `px` literals (W63 inventory) | Fluent 2 layout (multiples of the base unit); NN/g visual hierarchy (few sizes) | P1, P2, P3, P5, P6, P31, UF-4 (which checks the source; EL-12 checks the result, so a token misused for another role is caught too) |
| 57. **One screen grammar**: header bar, view bar, tabs, panels, drawer, status bar sit in the same regions with the same heights on every main surface (text, sheet, preview, versions, comparison, presentation setup) | **EL-13** regions — each main surface declares its regions (`data-region="header|viewbar|tabs|left|main|drawer|status"`); at each width of `states.toml`, the check compares region boxes across surfaces: same top, height and side for the same region (± 0.5 px), same order of regions, a region absent only where `states.toml` says why; and the same command keeps the same region across surfaces (EL-9 with geometry) | the text/table switch at the left in one view and the right in the other (UX-EX0-3); the sheet's status bar taller than the shell's | GNOME header bars (three fixed alignment points); Apple toolbars « Keep consistent groupings and placement »; WCAG 3.2.3 consistent navigation | spec §3.1 layout, L-1…L-7, P9, P12, S7, D1 (comparison in place of the editor, same regions), HB1 header variant B, UF-8 and G3 (measured invariants from the reference) |
| 58. **Neighbours are reviewed together** before each R14 session | **EL-14** side-by-side review — the walk renders, for the surface under review and each neighbour (the surfaces reachable in one gesture, from the XState graph), one sheet of captures at the same width and state, light and dark, aligned on their regions; the reviewer (Opus or Codex) records in the surface's inventory file one `coherence` verdict per neighbour; EL-1 fails while a neighbour pair has none | — | GNOME app criteria (dark without stray white surfaces, P32); CLAUDE.md « revue de cohérence visuelle (captures clair et sombre, comparées aux écrans voisins) » | K-3, UF-8, G2, P32, rule 52 |

EL-9 to EL-13 run in the same `elements.spec.ts` pass and in `fe:check` for their static halves; EL-14 is the last step
before R14 (§2.6, which now starts with the neighbours sheet). A seeded defect for each (a hand-written label, a banned
synonym, a hand-rolled dialog, a 7 px gap, a moved view bar, a missing neighbour verdict) must turn its check red in the
UX1-0 gate.

## 3. Order: from the whole to the element (owner, 2026-11-24)

The owner asked for « la plus pratique, la plus soignée, la plus jolie, la plus performante, tout en gardant une
simplicité ». The method is top-down, then back up in a loop, as the established methods do: Jesse James Garrett's five
planes (strategy, scope, structure, skeleton, surface: each plane constrains the next, and polishing the surface first is
the classic error; <https://www.jjg.net/elements/>), the Design Council's Double Diamond (diverge, converge, twice, with
people in each round; <https://www.designcouncil.org.uk/our-resources/the-double-diamond/>), and the design systems of
GNOME, Apple and Material (principles, tokens, components, templates; screens assembled from them, so coherence comes from
construction, not from a check afterwards). Simplicity is the guard at every phase: design by subtraction (GNOME), one
primary action per screen, and each phase ends with what it removed, not only what it added.

| Phase | What is decided, for the whole application | Rules and checks | Output |
|---|---|---|---|
| UX1-0 | The proof harness, first, because every later phase is measured with it | EL-1…EL-14 (§2) | the harness, seeded defects red |
| UX1-A | **Tasks and structure**: the five tasks of `docs/ux-principles.md` and spec §5 ranked by frequency; the map of every screen and how a person moves between them; what must be one gesture away and what may be two | rules 11, 20; Hick, Jakob | `docs/ux/structure.md`, reviewed blind, then with the owner |
| UX1-B | **Screen grammar**: the same regions on every main screen, the hierarchy of actions (one primary per screen, secondary, overflow), the screen templates (document, list, settings, dialog, popover) | rules 12–16, EL-13 | the templates, each with its geometry check |
| UX1-C | **The design system**: Adwaita tokens only, one component per role, the inventory of components, each with its states; duplicates merged, unused ones removed | rules 53–56, EL-9…EL-12 | the component inventory, every surface built from it |
| UX1-D | **Each surface, element by element** (below): the per-element records of §2.2 against rules 11–58 — the final check, not the starting point | EL-1…EL-8, §2.5 | records at 100 %, defects as failing e2e tests then fixes |
| UX1-E | **The loop**: each surface tested with the owner (R14: tasks, time, SEQ, recognition, beauty last), measured (UF-5 effort, PERF budgets), retouched; then the next surface | rule 51, EL-14 | SEQ and timings recorded; a surface is done only when the loop is quiet |

The surfaces of UX1-D, in order (each one UX1 sub-block, committed before the next, method v2 rule 1):

| Step | Surface | What it carries from PROGRESS | Why here |
|---|---|---|---|
| UX1-0 | **The proof harness**: `data-el` lint, `ux_elements.mjs`, `states.toml`, `elements.spec.ts`, inventory check 8, the vocabulary and component tables of §2.7 (EL-1…EL-14), on the shell only; needs EX0's command registry | brings forward the measuring half of G2/G3 (W65) [UX1] | every later step is checked by it; without it « chaque élément » is a promise |
| UX1-1 | Main window chrome: header bar, main menu, status bar | P14, P21, P31; P12-style duplicates removed | seen all the time; the most rules (15, 16, 20, 22, 25) |
| UX1-2 | Text area and the search bar | P19, U14 (SRC-11, SRC-20 out of `fixme`), UX-PC1a-6 (reading position kept) | the first task, the code exists |
| UX1-3 | Messages: banners and toasts | P24, UX-PC1a-3 (banner Tab order), UX-PC1a-7 (button wrap) | rules 28, 30–32 |
| UX1-4 | Files panel, file cards and the text/table switch | UX-EX0-3, UX-EX0-6 | rules 12, 14, 34 |
| UX1-5 | Preferences, with the Fonts tab | N43 (FT-1…14), P29, P30 (i18n lint over all strings) | rules 23, 29, 33, 35 |
| UX1-6 | Share, join and people (popovers and dialogs) | residuals of PC1a/PC1b and the second SI1b walkthrough [UX1] | rules 17, 24, 26, 28 |
| then | preview, sheet, presentation, versions and differences, first launch | W65 (G1, then T, V, D), S13, S1–S9 | their own blocks; the harness already covers them as they are built |

UX1 stays « standard » rigour (no peer bytes): plan ≤ 2 pages per sub-block, gate ≤ 45 min. UX1-0 is « light »
(tooling) but its gate includes a seeded duplicate, a seeded missing record and a seeded new element, each red. UX1-A,
UX1-B and UX1-C are design phases: their outputs are reviewed blind and then by the owner before UX1-D starts.

## 4. Proposed additions to `docs/ux-principles.md` (French, to paste after « Système visuel »)

```markdown
## Chaque élément (W82, 2026-11-24)

Règle d'ensemble : Adwaita et la HIG de GNOME donnent le langage ; les autres guides (Apple, Fluent 2, Material 3,
NN/g, WCAG, APG) ne comblent que ce qu'ils taisent. Sources, raisons et conflits tranchés :
`docs/plans/2026-11-24-ux1-element-audit.md`. Chaque élément de l'interface a une fiche d'inspection, et une machine
vérifie qu'aucun n'en manque (EL-1).

**Présence**
11. Chaque élément visible sert une tâche nommée de son écran ; sinon il descend d'un niveau (menu, palette).
12. Une commande, un seul contrôle visible par état d'écran (clavier, palette et barre de menus de macOS exceptés) ;
    un menu contextuel ne reprend que ce que l'interface offre déjà.
13. Un fait, une seule place : un état (synchronisation, non enregistré, rôle, problèmes, présence) est montré une fois.
14. Même fonction, même identité partout : un nom, une icône, un raccourci ; une icône, un seul sens.
15. Dans une barre : au début la navigation et les panneaux, au centre le titre et l'état, à la fin les menus ; une
    action principale au plus, à la fin.
16. Rien d'essentiel ne vit seulement dans la barre d'état.
17. Les cibles fréquentes sont grandes et près de leur contenu ; un menu s'ouvre à son déclencheur ; une action
    destructrice n'est jamais voisine d'une action fréquente.

**Groupes et hiérarchie**
18. L'écart entre deux groupes vaut au moins deux fois le plus grand écart dans un groupe ; une étiquette est plus près
    de son contrôle que de tout autre.
19. Des éléments frères partagent un bord d'attaque ; le retrait marque la subordination ; compteurs et heures en
    chiffres tabulaires.
20. Par vue : un bouton suggéré ou destructeur au plus, trois tailles de texte au plus dans le cadre, un seul point
    focal (test du flou).
21. Des boutons voisins ont la même largeur, deux largeurs au plus par fenêtre ; hors des barres, icône ou texte, pas
    les deux.
22. Dans la barre d'en-tête, des boutons plats à icône avec infobulle ; ni texte seul, ni style suggéré, destructeur ou
    lié.
23. Dialogues, préférences, écrans vides et listes ont une largeur maximale et se centrent.

**Divulgation**
24. Deux niveaux au plus : visible, puis un geste (popover, panneau, menu), puis un dialogue ; jamais un popover dans
    un popover, un sous-menu dans un sous-menu, un dialogue sur un dialogue.
25. Un menu a 3 à 12 entrées (sous-menu 3 à 6), groupées par sorte, rangées par tâche ou fréquence ; verbes pour les
    commandes, adjectifs pour les réglages ; le groupe standard à la fin.
26. Un popover couvre au plus un tiers de la fenêtre, sans bouton Fermer ; Échap le ferme.
27. Un menu contextuel : seulement les commandes probables pour cet objet, sur chaque sorte d'objet ou aucune ; les
    entrées indisponibles sont masquées, pas grisées.
28. Un dialogue ne s'ouvre qu'en réponse immédiate à un geste ; 1 à 3 boutons, l'affirmatif nommé par un verbe précis
    (jamais « OK »), Entrée sur lui, Échap sur Annuler, Annuler en premier.
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
36. Des libellés courts, sans abréviation latine (« ex. », « etc. »).
37. Le nom lu par le lecteur d'écran commence par le libellé visible ; celui d'un bouton à icône est le nom de son
    infobulle.

**Couleur et texte**
38. Une couleur, un sens : l'accent pour l'interactif et la sélection, les jetons d'erreur pour l'erreur et le
    destructeur, les couleurs des personnes pour les personnes ; aucune couleur décorative.
39. Le contraste APCA est relevé à côté du rapport WCAG, à titre d'information ; WCAG 2.2 reste la règle.
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
48. La palette montre d'abord les commandes récentes, et chaque raccourci.
49. Le défilement « machine à écrire » n'est jamais actif par défaut ni pendant une sélection.
50. Les boutons à icône seule restent dans la barre d'en-tête et les barres d'outils, avec les icônes de la spec §8.3,
    et leur reconnaissance est vérifiée en séance (R14).
51. La beauté se juge après la réussite des tâches, jamais à sa place.
52. Chaque surface a sa note « ce qu'Inkwell fait mieux » face à deux éditeurs nommés.

**Cohérence de l'ensemble** (un élément peut être bon seul et détonner parmi les autres)
53. Une commande a partout le même libellé, la même icône, le même raccourci et la même infobulle, tirés du registre
    et de lui seul.
54. Un concept, un mot, partout : le glossaire et le vocabulaire figé de la spec §6.4 sont vérifiés sur tous les textes ;
    un synonyme est une faute.
55. Un rôle, un composant : deux bandeaux, deux champs de recherche, deux écrans vides, deux dialogues sont dessinés
    par le même composant.
56. Une seule échelle : chaque espacement, taille, rayon, graisse, interligne et durée vient des jetons, et un même rôle
    a les mêmes valeurs sur tous les écrans.
57. Une même grammaire d'écran : barre d'en-tête, barre de vue, onglets, panneaux, tiroir et barre d'état occupent les
    mêmes régions, aux mêmes hauteurs, sur chaque écran principal ; une commande garde sa région d'un écran à l'autre.
58. Avant chaque séance avec l'utilisatrice, l'écran est revu à côté de ses voisins, en clair et en sombre, et la
    cohérence avec chacun est consignée.

Preuves : `docs/engineering-principles.md` §5, UF-13 à UF-26 (vérifications EL-1 à EL-14 du plan).
```
