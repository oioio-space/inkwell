# W58a UX walkthrough

Block W58a (read-only foreign project), worktree `inkwell-w58`, HEAD 26b11c0 plus the uncommitted implementation.
Method: `ux-walkthrough` (cognitive walkthrough, Nielsen heuristics). The screen was explored first; the plan was read
afterwards to check what is missing. Real app: `bin/inkwell-server` rebuilt from the working tree (e2e build, observe
mode), Chromium, French locale, two real profiles on one project folder (the helpers of `foreign-project.spec.ts`).

Captures are in `gates/W58a/ux/` (58 PNG files). That folder is git-ignored except `*.md`: the captures are evidence kept
locally, not committed. Drive file: `frontend/e2e/ux/w58a.spec.ts` (skipped unless `W58A_UX=1`; observations go to
`gates/W58a/ux/observations.log`).

## Blocking findings (top)

- **UX-1-1, severity 3 (major), remains.** The persona "Alice reinstalled her computer and kept her Documents" reads
  her own document, is told she cannot edit or share, and is offered one action that sends her to "the person who shares
  this document". Alice is alone: nobody can invite her. She can copy the text and export a PDF, but cannot continue
  editing her own file from Inkwell, and there is no way out on screen. The plan names this as its stated limit, to be
  closed by PC1. Under the rule (severity 3 blocks a merge or a release) this blocks a release of W58a without PC1, or
  needs the owner's explicit acceptance of the limit.
- No severity 4. One severity 2 was fixed (UX-2-1); one severity 2 remains and is not a UI matter (UX-2-2).

## Scenario 1: "Alice reinstalled her computer" (read-only, banner, share dialog)

| Step | What she sees | Capture |
|------|---------------|---------|
| Opens her document | Her text, a blue banner under the format bar, greyed format bar | `s1-banner-full-light.png`, `s1-banner-full-dark.png` |
| Narrowest native width (720 px) | Banner wraps on two lines, action centred below, nothing cut | `s1-banner-720-light.png`, `s1-banner-720-dark.png` |
| Selects and copies | Ctrl+A then Ctrl+C gives the whole text; mouse drag then Ctrl+C gives the dragged part | `s1-selected-light.png` |
| Types, formats | `x`, Enter, Backspace, Delete, Ctrl+B, Ctrl+I, Ctrl+Z, Ctrl+V, Ctrl+X, Tab and an inserted text: the document stays byte-identical | (log) |
| Format bar, Insérer | Greyed and `inert`: not focusable, not clickable | `s1-insert-menu-light.png` |
| History | Entries show; "Créer une variante ici", "Nommer cette version", "Revenir à cette version" disabled | `s1-history-entry-light.png` |
| Other doors | Right-click on the file: no menu. Title: not editable. People: lists the original author as Admin, offline | `s1-people-light.png` |
| Partager | "Ce document n'est pas à vous sur cet appareil : vous ne pouvez pas inviter." next to a disabled "Inviter"; the join part is offered | `s1-share-nonmember-light.png`, `-dark.png`, `s1-share-nonmember-720-dark.png` |
| "Demander une invitation" | Same dialog with "Demandez à la personne qui partage ce document de vous inviter, puis collez son invitation ici." | `s2-ask-dialog-light.png`, `-dark.png`, `s2-ask-dialog-720-dark.png` |

Cognitive walkthrough:
1. Will she want to do the step? Yes: the banner names what she cannot do and offers one action.
2. Will she notice the control? Yes: the action sits in the banner, same row, contrast checked in both themes.
3. Will she associate it with her goal? For Bob (asks someone to invite him): yes. For Alice alone: no, see UX-1-1.
4. Will she see that it worked? The dialog opens at once with a sentence that matches the action.

Keyboard: Tab reaches "Demander une invitation" after the header controls (it follows "Menu principal" in the
document order); Enter opens the dialog; Esc closes it; focus returns to the banner button (`s1-focus-ask-light.png`,
`s1-focus-ask-dark.png`, visible ring). Opened through "Partager", Esc returns focus to "Partager". The editor exposes
`aria-readonly="true"`; the locked format bar is `inert`, so a screen reader does not walk through dead buttons.
Accessibility: axe (WCAG 2 A and AA) reports zero violations on the page and on the open dialog, light and dark.
Forced colors: the action keeps a visible border (`s1-forced-colors-light.png`). 200 % zoom (640 x 400 CSS px): the
banner wraps in four lines and nothing in the banner is cut (`s1-zoom200-light.png`).
No take-over remains: no "Reprendre" button, no confirmation, no second action in the banner or the dialog (the test
`W58a-E2E` asserts zero "Reprendre ce document ici" and the full page text has none).

Findings:

- **UX-1-1 (3), remains**: see above. Evidence: `s1-banner-full-light.png`, `s2-ask-dialog-light.png`. Repro: a profile
  on a project folder whose members do not name it, alone. Expected: an action that works without another person (PC1),
  or, if the block ships first, a sentence that does not promise someone will answer. The banner sentence itself is
  honest and clear; the problem is the missing way out.
- **UX-1-2 (1)**: three history actions are disabled with no reason next to them (heuristic 1, 9); the banner above says
  why in general. Evidence: `s1-history-entry-light.png`. Expected: a short reason on hover or under the group.
- **UX-1-3 (1)**: opened from the banner, the dialog puts focus on "Fermer", not on the paste field, so Bob needs two
  Tab presses before he can paste. Expected: focus on "Collez l'invitation ici" when the dialog is opened by
  « Demander une invitation ».
- **UX-1-4 (1)**: "Partager" stays the strongest (blue) control for a person who cannot share. The dialog explains
  at once, so no task fails. The owner's mock-up proposal P11 (no blue background for "Partager",
  `docs/design/reference/2026-11-14-w63-mockups.html`) would reduce this: no conflict, nothing to redesign here.
- **UX-1-5 (1)**: the banner arrives after the text and pushes it down by 45 px (compare `s1-selected-light.png` with
  `s2-joined-light.png`). The owner's study already lists this (P5, S2 in the same mock-ups): not specific to W58a.
- Pre-existing, outside this block (not fixed): the header title ("Sans titre / main.typ") collides with the header buttons
  at 720 px and at 200 % zoom, for a member as well (`s3-member-720-dark.png`, `s1-zoom200-light.png`): scenario S13.
  At 720 px the editor pane is not shown, only the preview, so "copy the text" there goes through the preview; I did not
  verify whether the pane can be brought back at that width. Ctrl+F does nothing (P19 of the same mock-ups).

## Scenario 2: "Bob received a copied folder and asks for an invitation"

Steps: banner, "Demander une invitation", rename, paste the first profile's invitation, "Rejoindre".
Captures: `s2-ask-dialog-*.png`, `s2-pasted-light.png`, `s2-pasted-dark.png`, `s2-joining-*.png` (frames in the first
400 ms), `s2-joined-light.png`, `s2-joined-dark.png`, `s2-bob-wrote-light.png`, `s2-bob-wrote-dark.png`,
`s2-bob-share-after-join-*.png`, and the other person's screen `s2-host-people-light.png`, `s2-host-people-dark.png`.

Result when the list arrives at once: the dialog closes about 100 ms after the click, the banner goes and the editor is
writable in 62 to 135 ms (median about 70), without a reload; Bob types, the text reaches the first profile
(30 s poll, passes); the first profile's people list shows Bob as writer with "À jour" and "Déconnecter / Passer la main"
controls; Bob's own dialog now says "Seul un admin peut inviter quelqu'un." (the writer wording, different from the
non-member one). After a reload the banner stays away. The existing `foreign-project.spec.ts` passes (2 of 2).

- **UX-2-1 (2), FIXED.** Before the fix, in a copy of the project folder the banner stayed for about 14 s after a
  successful join: header "Synchronisé", "2 personnes", dialog closed, banner still saying "Ce document vient d'un autre
  appareil" with the invitation button she had just used, and a dead editor. Measured with an in-page observer, joins
  into a copy: banner and lock gone after 13.6 s and 14.4 s in 2 of the joins of the first runs (the other joins: 62 to
  135 ms). Screenshot of the state after the fix: `s2-waiting-banner-1.png` and `s2-waiting-banner-1-plus2s.png`
  ("Invitation acceptée. Vous pourrez écrire dès que la liste des personnes de ce document sera arrivée.", no button).
  Heuristics 1 (status) and 9. Failing test first: `frontend/src/ui/foreignJoinWait.test.ts`, test « UX-W58a-2 says the
  invitation is accepted and drops the action, then goes away » (failed on the old banner with `undefined` key, passes
  after). Fix, in the block's files only: `workspace.listPending` (set after a join that does not yet name this device,
  cleared when the standing becomes `member` or the join fails), `AccessBanner.svelte` (waiting sentence, no action),
  catalogue key `foreign.joinWaiting` in `fr.json` and `en.json`. Re-walked: 37 joins in 4 runs after the fix, 4 of them slow; the 3 slow
  joins of the run that captured the screen all showed the waiting sentence with zero "Demander une invitation" button
  at 400 ms and again 2 s later, then the normal editor (the fourth was in a run without that capture); every fast join
  unchanged. Closed.
- **UX-2-2 (2), remains, not a UI defect.** The slow case is a constant 14.3 s (13.9 to 14.4 s), which matches
  `defaultTick = 15 * time.Second` in `internal/app/wiring.go` ("how often the member list frontier is announced
  again"): when the join targets a copy of the same project, `adopt` returns early (`already there; the member lists
  converge by themselves`), and the list that names the joiner arrives at the next announcement. It happened in 6 of 50
  joins over the runs. On a bad network (Karim) it will be longer. The waiting sentence makes it honest; the cause is
  better fixed by announcing the frontier when a peer connects (trust sync, "complete" rigour class, so not touched here).
  Needs a follow-up item.
- **UX-2-3 (1)**: after joining, the pasted invitation stays in the field of Bob's dialog and "Rejoindre" stays
  available (`s2-bob-share-after-join-light.png`). Not checked against `main`; probably pre-existing.

## Scenario 3: a member's normal first launch

Fresh profile, then 6 page loads, measured in the page from navigation start with `requestAnimationFrame`, "Partager"
clicked as soon as it exists:

| Measure | min | median | max |
|---------|-----|--------|-----|
| Editor shown (text visible) | 301 ms | 307 ms | 317 ms |
| Editor editable | 301 ms | 313 ms | 341 ms |
| Format bar active | 301 ms | 313 ms | 341 ms |
| « Inviter » shown (with « Vérification… ») | 181 ms | 191 ms | 195 ms |
| « Inviter » enabled | 227 ms | 313 ms | 341 ms |

The read-only state was visible in 1 load of 6, for 34 ms (307 to 341 ms), with the text already shown; in the other 5
the editor appeared already editable. The foreign banner appeared in 0 loads of 6. "Vérification…" is seen in the
dialog only if "Partager" is clicked within the first 300 ms. Captures: `s3-early-0.png`, `s3-early-1.png` (the dialog
there is opened by the probe), `s3-member-first-screen-light.png`, `s3-member-first-screen-dark.png`,
`s3-member-720-dark.png`. The member's screen has no banner, a normal format bar and the placeholder. Verdict: the lock
is invisible for a member; the figures match the plan (323 to 359 ms).

## Heuristics and consistency summary

- Visibility of status: good (banner, dialog sentences, "Vérification…"); the join gap was the exception (UX-2-1, fixed).
- Match with the real world: "appareil", "personne", "invitation", no Go text, no jargon; fine.
- User control: Esc and "Fermer" in the dialog, focus returns; fine. No way out for a lone person (UX-1-1).
- Consistency: banner, dialog rows and buttons are the existing Adwaita components; light and dark checked against the
  neighbouring screens. The owner's mock-ups (`docs/design/reference/2026-11-14-w63-mockups.html`,
  `2026-11-14-w65-mockups.html`) have no banner or dialog for this case; the conflicts are only the ones noted in UX-1-4
  and UX-1-5.
- Error prevention: the editor, format bar, history actions and the "Inviter" controls are locked, nothing is lost.

## Tests run

- `vitest run src/ui src/workspace src/net src/i18n`: 42 files, 531 tests, all pass (through `scripts/heavy.sh`).
- `playwright test e2e/foreign-project.spec.ts`: 2 passed. `e2e/ux/w58a.spec.ts` (W58A_UX=1): each of its 10 tests ran at least once.
- Biome on the touched frontend files: no error (the exploratory spec has non-null-assertion warnings only).
  `tsc --noEmit` shows two pre-existing errors in `src/ui/search/SearchPanel.test.ts`, not in this block.

## Files changed by this walkthrough

- `frontend/src/workspace/workspace.svelte.ts` (`listPending`, set in `join`, cleared in `refreshStanding`)
- `frontend/src/ui/people/AccessBanner.svelte` (waiting sentence, no action while waiting)
- `frontend/src/i18n/fr.json`, `frontend/src/i18n/en.json` (`foreign.joinWaiting`)
- `frontend/src/ui/foreignJoinWait.test.ts` (new, the failing test first)
- `frontend/e2e/ux/w58a.spec.ts` (new, the walkthrough drive file, skipped by default)
- `bin/inkwell-server` and `frontend/dist` were rebuilt (untracked build output).

No gate file, no `gates/*.toml`, no commit, no stash, no `gate`, `done` or `ci`.

## What remains

1. UX-1-1 (severity 3): the lone-person dead end, closed only by PC1 or by an explicit owner decision.
2. UX-2-2 (severity 2): the 15 s announcement tick behind the slow list arrival; needs a Go follow-up.
3. Severity 1: UX-1-2, UX-1-3, UX-1-5, UX-2-3, and the pre-existing header collision at 720 px.
4. The short session with the owner in front of the application (the definition of « fini », step 4) is not replaced
   by this walkthrough.

## Owner's decision (2026-11-14, 23:18)

Asked in the session with the facts above (UX-1-1: the person alone reads, copies and exports her text but cannot
take the document back; today on `main` she believes she is writing while nothing leaves, and « Inviter » fails on
raw Go text), the owner chose « Intégrer, limite acceptée »: W58a enters `main` as a step, not as a published
version; UX-1-1 is accepted until block PC1 (project change by restart), which closes it and is mandatory before R1
— R1 is not released without PC1. UX-2-2 (a join waits for the inviter's 15 s member-list announcement) goes to block
IV1. With UX-1-1 accepted by the owner and no other severity ≥ 3, the walkthrough's rule allows shipping the step.

Verdict: ship
