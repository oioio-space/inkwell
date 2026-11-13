# UX/UI review for R1 (2026-11-13)

Scope: the first-use journey of R1, Adwaita/HIG consistency, collaboration cues, defect classes of the walkthroughs,
and at most ten ranked improvements. It builds on `docs/plans/2026-11-09-ux-research.md` (gaps G1-G14, R1-R14) and
`docs/plans/2026-11-10-collab-presence.md`, and does not repeat them: it says what is still open today and what R1 needs.

Method and limits. Nothing was launched or run (a frozen gate holds the machine). Evidence is: code on `main` 0b887fa
(`frontend/src`, `internal/app`), the 13 e2e baselines (`frontend/e2e/*-snapshots/`), the S7 captures
(`docs/ux/screens/`), five walkthrough reports (S7, S11 in `docs/ux/`; OI, SI1, SI1b in the block worktrees), contrast
ratios computed from `frontend/src/theme/tokens.css` (WCAG formula), and web pages fetched today (§6). Click counts are
read from the code, not timed. One point is marked **unverified**: it needs a run.

## 1. First-use journey (install, create, invite, co-edit, export)

| Step | What the person does | Evidence | Where a newcomer stalls |
|---|---|---|---|
| Install | deb/rpm | `pkg:test` PKG-01..11 green (PROGRESS row 10) | not assessed here; the packaged two-person walk is still to do |
| First launch | 0 clicks: an untitled project opens on `main.typ`, « Commencez à écrire… » | `docs.go:86-104`, `s7-03-alice-people-dialog.png` | no welcome, no name asked: `fr.json` has 0 welcome keys; the name becomes « Personne 12D3 » (`workspace.svelte.ts:282`) |
| Create | nothing to do | header title is the constant « Sans titre » (`App.svelte:403`, `:125`) | the project cannot be named, and the main menu (`HeaderBar.svelte`, 6 to 9 items) has no project command; `Projects.Current().Title` exists (`projects.go:56-59`) and is never read |
| Invite | Partager, Inviter, Copier l'invitation, close: 4 clicks, then paste in another app | `ShareDialog.svelte:51-71`, `s7-15-lea-share.png` | the button « Partager » opens « Écrire à plusieurs »; the invitation is an opaque `inkwell1:AEAAF7…` text; nothing says it works once (only a code comment), nor that the other person arrived, once the dialog is closed |
| Join | Partager, paste, Rejoindre: 3 actions | `ShareDialog.svelte:75-90`, `workspace.svelte.ts` `join` | « Rejoindre » is only inside « Partager »: a person holding an invitation sees no entry for it on the window. Joining re-points the only project to the inviter's document (`open`, `workspace.svelte.ts:375`, `project.go:289-299`); what happens to text already typed is **unverified** and has no way back in the interface |
| Co-edit | 0 clicks; named carets, avatar stack, status words | `collab.spec.ts:50-51`, `gates/SI1b/UX.md` | the other person shows as « Personne 12D3 », or as a machine name such as « fedora.home » (`s7-03`), unless they typed a name in the share dialog |
| Export | menu, « Exporter en PDF… », native save dialog | `preview/actions.ts:73-86` | « Le document contient des erreurs : corrigez-les avant d'exporter. » has no action although F8 and `jumpToProblem` exist; no shortcut; not in the preview pane |

Reading: the effort is low (4 + 3 clicks to a shared keystroke). The stalls are discoverability (join), identity (names,
title) and missing next steps, not the number of steps. No walkthrough has yet played this journey end to end: the five
reports cover people management, presenting, opening from the desktop and the sync indicator.

## 2. Adwaita/HIG consistency, jargon, accessibility

| Area | Finding | Evidence | Status |
|---|---|---|---|
| Header bar | constant title; status is a `span role="status"`, not a control (« a new user tries it », SI1 report) | `HeaderBar.svelte:82-88`, `App.svelte:403` | G1, G2 open; layout taken by SI1c |
| Header bar | « Relire » is drawn with a Git branch glyph | `HeaderBar.svelte` `Icon name="branch"`, `window-light-chromium-linux.png` | jargon by image, new |
| Banners | 7 sources mounted side by side, so several can show; HIG: « a single title, which should be short and to the point » | `App.svelte` (Sealed, Access, Projection, `limit.slow`, External, Present, HistoryBars); `people-suspended-dark-chromium-linux.png` is two sentences on two lines | G3 open |
| Banners | « Liste de membres saturée — créez un nouveau projet » asks for an action the interface does not offer | `fr.json` `sealed.message`; no project command (§1) | new |
| Toasts | pill, bottom, centred as the HIG asks; raw colours; two hosts plus the sheet's; the queue exists only in the OI worktree | `Toast.svelte` style block, `App.svelte` (`NoticeToast`, `ExternalChangeToast`) | G4 partly closed by OI |
| Dialogs | native `<dialog>`, boxed lists, 44 px close target, `min(480px, 100vw - 32px)`; usable at 360 px and 200 % (S7 report) | `ShareDialog.svelte`, `PeopleDialog.svelte` | consistent |
| Dialogs | a reversible action asks for confirmation (« Déconnecter … ? Vous pourrez le laisser revenir à tout moment ») against rule 6 | `fr.json` `people.suspend.ask` | minor |
| Adaptive | one container query for the panes (`max-width: 560px`); the sidebar is a fixed 264 px and never an overlay, so a 360 px window leaves 96 px to the header and editor | `App.svelte` style, `Sidebar.svelte:77` | G11 open; SI1c is header only (§5, item 5) |
| Jargon | « fusion » family in 6 keys, where the glossary says « intégrer » and lists « fusionner » as never to say | `fr.json:555, 624, 627, 628, 662, 669` | new |
| Jargon | raw Go error text inside sentences, 4 keys; this path showed « RuntimeError: invite: invite: trust: signer is not an admin » (UX-S7-4) | `fr.json:32, 49, 546, 693` | G12 open |
| Baselines | 13 images in 2 specs; none of the share dialog, a toast or a narrow window; 3 people views exist in one theme only | `frontend/e2e/*-snapshots/` | UF-8 partly met |

Measured contrast (tokens, WCAG 2.2: 4.5:1 text, 3:1 non-text, SC 1.4.3 and 1.4.11):

| Pair | Ratio | Verdict |
|---|---|---|
| `--fg-dim` on header, window, sidebar (light); on the three dark surfaces | 4.74, 4.69, 4.54; 5.24 to 5.93 | pass |
| `--fg-dim` on a selected sidebar row, light (« principal ») | 4.26 | fail by 0.24 |
| `--focus-ring` (accent at 75 %) on white header, light sidebar, dark header, dark window | 2.62, 2.34, 2.63, 2.95 | fail (3:1) |
| the same ring with the opaque `--accent` token | 5.10 to 6.07 light, 7.98 to 9.93 dark | would pass |
| `--success` dot on the white header | 2.30 | below 3:1; the word beside it carries the state |
| person colours as a caret on the light editor: `#f0e442`, `#e69f00`, `#56b4e9`, `#999999` | 1.32, 2.25, 2.31, 2.85 | 4 of 8 below 3:1; the name is always written (`remoteCursors.ts`); planned as CP1 |
| avatar initials on the 8 colours; white on « Partager » | 4.05 to 15.88; 4.77 | pass |

Other accessibility facts: a person's name on an avatar is reachable by mouse `title` only (`PeopleStack.svelte:25,31`),
49 `title=` attributes in 18 components (G8); a `<ul>` sits inside a `<button>` (`PeopleStack.svelte:14-36`, G9).
Keyboard, 200 % zoom and forced colours were walked in S7 and S11; the three reports of November (OI, SI1, SI1b) each
list « 200 % zoom, forced colours, screen reader » under « not walked ».

## 3. Collaboration cues against the state of the art

| Cue | inkwell (R1 with PR1, SI1c, C5S-UI) | Others (fetched today unless noted) | Position |
|---|---|---|---|
| Presence | avatar stack, carets with the name always shown | Zed: each collaborator has a colour, a click on an avatar follows them, joining follows the inviter (zed.dev). Google Docs: caret in the avatar's colour (2026-11-10 study). typst.app documents only « real-time synchronization with your co-authors » and share links | at parity for R1; follow and jump are CP3, after R1 |
| Sync state | words driven by a TLA+ model: Seul, Connexion…, Vérification…, Envoi en cours (n), Pas encore à jour, Synchronisé | Google Docs: a cloud icon « See document status », messages « Checking offline sync status » and « Could not sync document » (support.google.com). HedgeDoc's only strings: « Your editor is out of sync », « You typed something while being offline, but we can't sync you up again. » | ahead in honesty; behind in one point: Google's state is a control that explains itself, ours is a label (G2) |
| Link quality | per person, in words, after C5S-UI | none of the five editors shows it; call apps do (sources in the C5S-UI plan) | ahead |
| Offline | editing continues; « Hors ligne » per person; « Modifications non envoyées » after SI1c | server editors stop or go read-only: Overleaf's open-source editor has a « reconnecting » state and a read-only mode on lost connection (search result, code not documentation). GNOME HIG gives « Working offline » as its banner example | ahead; the HIG place for an ongoing state is a banner, the header word is our choice |
| Arrivals | status and avatar change, no event | HIG: toasts are « best suited to communicating individual events, as opposed to ongoing states » | gap, cheap (item 6) |
| Conflicts | concurrent edits merge silently; « divergence » is stage 4 | Google Docs « Show editors » per range (2026-11-10 study); typst.app has no history | accepted for R1 |

## 4. Defect classes in the five walkthroughs (36 numbered defects, 7 of severity 3 or 4)

SI1 and SI1b cite their Nielsen heuristics; for S7, S11 and OI the class is this review's reading.

| Class (Nielsen) | Count | Defects | Absorbed by | Left |
|---|---|---|---|---|
| A. The state shown is untrue, unstable or invisible (1) | 10 | S7-8, S11-7, S11-10, OI-1, SI1-1, -2, -3, -4, -5, SI1b-1 | SI1b (SI1-1, -2, -5), SI1c (SI1b-1, SI1-3), PR1 (SI1-4, departures), OI (OI-1) | S7-8 |
| B. Wording ambiguous, terse or wrong (2) | 6 | S7-2, -6, -7, S11-4, OI-2, SI1b-3 | fixed in their blocks, except two | « Passer la main » (S7-7), « Envoi en cours » under a person (SI1b-3) |
| E. Assistive technology and focus (a11y) | 5 | S7-5, -9, S11-1, -2, OI-3 | fixed, S7-9 accepted | the three checks not walked since October |
| C. Narrow width and zoom (8) | 4 | S11-3, -8, SI1-8, SI1b-2 (+ the 360 px overlap seen by OI) | SI1c (header), C5S-UI captures at 360 and 600 px | everything but the header: the sidebar (G11) |
| D. A control offered that does nothing, a dead end (5, 3) | 4 | S7-1, -4, S11-5, OI-4 | fixed, OI-4 accepted until OI3 | OI-4, slow-open toast |
| F. One thing, two names, icons or places (4) | 3 | S11-6, -9, SI1-6 | SI1b (SI1-6) | S11-6, -9 |
| G. A person's name lost (2) | 2 | S7-3, SI1-7 | PR1 only in part (one roster, P-3b, P-4) | the generated default name (§1) |
| H. Discoverability, visual weight | 2 | S11-11, -12 | none | minor |

Class A is the largest and is the one the three current blocks treat at the root (model, one roster). Classes G, C
(beyond the header) and the first-use stalls of §1 are in no current block: the improvements below target them.

## 5. Improvements, ranked by impact over cost (S ≤ 1 day, M ≤ 3 days)

| # | When | Improvement and evidence | Concrete change, in an existing place | Proof |
|---|---|---|---|---|
| 1 | R1, M | Joining is hidden and may orphan the joiner's text (§1, `workspace.svelte.ts:375`) | First replay it (walk). Then: a main-menu entry « Rejoindre avec une invitation… » opening the same dialog on its join field; if the current document has text and no other member, the dialog's existing inline question (`.ask` pattern of `PeopleDialog`) says what becomes of it before « Rejoindre » | e2e `join.spec.ts` « R1-J a person who typed, then joins, still reaches that text » (red first); walkthrough criterion in the packaged two-person walk |
| 2 | R1, S | People appear as « Personne 12D3 » or a machine name (§1, class G) | In `ShareDialog`, a generated name is not prefilled: the field is empty, focused and required before « Inviter » and « Rejoindre », with one line « Ce nom est montré aux autres personnes ». No question before the first share (rule 8 kept) | Vitest `ShareDialog` « R1-N a generated name is never announced »; `join.spec.ts` asserts no avatar « ? » and no « Personne » prefix after joining |
| 3 | R1, S | Glossary leaks: 6 « fusion » strings, 4 `{reason}` sentences (§2) | Replace by « intégrer / intégrée »; the reason goes to the log, the sentence keeps what to do. The glossary's « never say » column becomes a test list | `i18n.test.ts` « UI-11 no word of the glossary's forbidden column in fr or en », « UI-12 no `{reason}` in a sentence key »; seeded « fusionner » turns it red |
| 4 | R1, S | Focus ring 2.34 to 2.95:1, dim text 4.26:1 on the selected row (§2) | Two tokens: `--focus-ring` uses the opaque `--accent`; the selected row uses `--fg` for its badge. No component change | `tokens.test.ts` (already checks syntax colours): ring ≥ 3:1 on every surface token, dim text ≥ 4.5:1 on selected rows, light and dark |
| 5 | R1, S, before SI1c is frozen | SI1c E1 asks a correct header at 360 px with the sidebar open: 360 - 264 = 96 px (`Sidebar.svelte:77`, `gates/SI1c.toml:27`), which looks unsatisfiable, and its Q3 excludes collapsing the sidebar. It also keeps « Sans titre » and hides the file name first, so a narrow header names nothing | In the SI1c plan, not a new block: below about 700 px of window the sidebar starts closed (the libadwaita split-view behaviour the plan already cites); the title shows the file name while the project title is the default | SI1c E1 as written, plus « the header text contains the current file name at 360, 500, 700, 1024 » |
| 6 | R1, S, after PR1 | The inviter is not told that the person arrived, nor that an invitation works once (§1, §3) | One toast through the existing `notice` host when a member first connects (« Paul a rejoint le document »); `share.host.help` says « Elle sert une fois » | Vitest on the roster event; `join.spec.ts` « the inviter reads a toast naming the person within 3 s »; PR1's roster is the single source |
| 7 | R1, S | Export refuses with no way to the error (§1) | The existing toast gets its action « Voir » (as `ExternalChangeToast` has), calling `jumpToProblem` on the first problem | Vitest on the notice action; `preview.spec.ts` « export with an error offers Voir and lands on the problem » |
| 8 | R1, zero code | Zoom 200 %, forced colours and screen reader skipped in the last three walks (§2, class E); only 13 baselines (§2) | `gates/<bloc>/UX.md` must carry the three « walked » lines (skill `ux-walkthrough` checklist); add share-dialog and 360 px baselines, light and dark, when SI1c lands | the gate's `ux-walkthrough` criterion also checks the three lines; UF-8 baselines |
| 9 | after R1, M | No first minute, no project name, no Projects screen (G1, G6); the « sealed » banner points to a command that does not exist | Lot « Première minute et secours » (coherence §5, W40 R4): welcome with name and colour, project title shown and renamed in the header, Projects with « Nouveau » and « Rejoindre » | UF-3 onboarding machine, UF-5 budget « launch to first shared keystroke », SEQ « create » and « share » ≥ 5.5 |
| 10 | after R1, M | The sync word cannot be questioned (G2); banners stack (G3); « Envoi en cours » under a person is ambiguous (SI1b-3); interim toasts are dead ends (OI-4); carets below 3:1 and names by `title` only (§2) | Already planned, in this order: sync popover with the banner and toast hosts (2026-11-09 R1, R2), then CP1 (caret tokens) and UF1 (tooltip, avatar markup) | as in those plans; SI1b-3's wording is settled in the C5S-UI user session, which already lists a wording question |

Not proposed: a bar glyph for link quality, auto-follow on join, and a second place for sync state; each would be a
bolted-on feature or contradict a decision already recorded.

## 6. Sources (fetched 2026-11-13)

- GNOME HIG banners: https://developer.gnome.org/hig/patterns/feedback/banners.html (« Banners do not automatically
  hide »; « a single title, which should be short and to the point »; example « Working offline »).
- GNOME HIG toasts: https://developer.gnome.org/hig/patterns/feedback/toasts.html (« best suited to communicating
  individual events, as opposed to ongoing states »; « Toasts shouldn't always include a button »).
- GNOME HIG placeholders: https://developer.gnome.org/hig/patterns/feedback/placeholders.html (heading, guidance, action).
- libadwaita AdwAvatar: https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.Avatar.html (round, initials,
  colour « picked based on the hash » of the text; inkwell's avatars follow it).
- Zed channels: https://zed.dev/docs/collaboration/channels (colours, follow by avatar, join follows the inviter).
- Google Docs offline: https://support.google.com/docs/answer/6388102 (« See document status », sync messages).
- typst.app web app: https://typst.app/docs/web-app/ (says nothing on cursors, avatars or offline).
- HedgeDoc strings: https://raw.githubusercontent.com/hedgedoc/hedgedoc/master/locales/en.json
- Overleaf: search result only (open-source commits « reconnecting », read-only on lost connection); its help page
  https://docs.overleaf.com/troubleshooting-and-support/resolving-pdf-rendering-and-project-loading-problems gives no
  editor wording. Not used as evidence beyond the table's note.
- WCAG 2.2 SC 1.4.3, 1.4.11: https://www.w3.org/TR/WCAG22/ (thresholds; ratios computed here, not fetched).
