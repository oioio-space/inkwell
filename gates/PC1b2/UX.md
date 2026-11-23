# PC1b2 UX walkthrough: joining by invitation restarts Inkwell on the joined document (2026-11-23)

Tester: ux-tester (ux-walkthrough skill). Tree: worktree `inkwell-pc1b2`, rebased on WC1 (window close) and HP1; builds made
that day (`mise run e2e:build`, `mise run app:build`). Two drivers, as in `gates/PC1b/UX.md`:

- server build `bin/inkwell-server` under Playwright/Chromium, 2 instances with a real invitation between them (light and dark);
- desktop build `bin/inkwell` (WebKitGTK 2.54, GTK 4.24) in a headless Mutter on a private bus, driven through WebKit's remote
  inspector, **and recorded as video frames through Mutter's ScreenCast** (15 fps, `scripts/rec.py`) so that the native window,
  which the inspector cannot see, is judged as the person sees it. The webview follows the host theme (dark) on the desktop; the
  light desktop captures force `data-theme="light"` on the root (the same tokens), and say so below.

Every launch had `INKWELL_DOCS`, `INKWELL_HOME` and the XDG directories on fresh temporary directories under the scratchpad,
checked non-empty and confirmed from `/proc/<pid>/environ` before use (`scripts/h.mjs`, `scripts/dlib.mjs`); no process of
the session is left (checked after every run). The screen was discovered first; the plan, the catalogue and the code were read
afterwards. Captures: `gates/PC1b2/ux/` (git-ignored like the earlier walkthroughs: `/gates/*/*`), scripts: `gates/PC1b2/ux/scripts/`.

## Blocking defects (severity 3 and 4)

None. The two defects of the first walkthrough that blocked (UX-PC1b-1: dead-end waiting page, UX-PC1b-2: nothing said about the
kept document) are closed in this pass (see "Previous defects").

## Defects

| id | sev | what | capture | failing e2e test |
|---|---|---|---|---|
| UX-PC1b2-1 | 2 | After the guest cancels (« Annuler » then « Invitation annulée. Pour rejoindre ce document plus tard, demandez une nouvelle invitation. »), the inviter reopens « Partager » and her dialog still offers the **spent** invitation (field filled, « Copier l'invitation »), with nothing saying it is used up. Giving it again to the guest gives « Impossible de rejoindre. Vérifiez l'invitation et que l'autre personne a cliqué sur « Inviter ». » (reproduced 4 times of 4 with the shown invitation; works at once with a freshly generated one). The guest's screen tells her to ask for a new invitation; the inviter's screen does not tell the inviter that one is needed. Heuristics 1, 5, 9. The e2e helper `freshInvitation` already documents that « the dialog keeps showing the last one »: a known behaviour, but the cancel path makes it likely (one cancelled question = one spent invitation). Expected: after the invitation is used or cancelled, the inviter's dialog clears it or says « Cette invitation a déjà servi » and shows only « Inviter ». | `ux/a13-alice-reopens-share-light.png`, `ux/07-retry-after-cancel-light.png` | `frontend/e2e/ux/pc1b2.spec.ts` « UX-PC1b2-1 … » (red: `Expected: not "inkwell1:…"`) |
| UX-PC1b2-2 | 2 | After the join, the application has **no way to the document the person had before**. The question promised « Votre document actuel ne change pas : il reste dans « Sans titre » », the hamburger menu has no « Ouvrir », list or previous-document entry (`ux/j25-menu-after-join-light.png`), a relaunch opens the joined document (`ux/d33-…`). The only way back is « Revenir à mon document » **while the join is still waiting**, so a person whose join succeeded at once (the normal case: inviter online) can reach their old text only through the file manager, knowing the folder name told once in a question. The text is safe on disk (`Sans titre` beside `Sans titre (2)`, checked) and nothing is lost, which keeps this at 2; it is the second half of UX-PC1b-2 that the question only softened. Expected: a document list or « Ouvrir… » in the main menu (a block of its own: no project list exists in the application). Needs a PROGRESS block id before merge (inventory rule). | `ux/j25-menu-after-join-light.png` | `frontend/e2e/ux/pc1b2.spec.ts` « UX-PC1b2-2 … » (red: no menu item) |
| UX-PC1b2-3 | 1 | « Rejoindre » looks enabled with an empty field and does nothing when clicked (no message). | `ux/08-rejoindre-empty-light.png` | no (severity 1) → UX1 |
| UX-PC1b2-4 | 1 | The title in the question, « Rejoindre « Sans titre » ? », and « il reste dans « Sans titre » » name the two documents the same way (no title can be given to a document in the interface): the person cannot tell which is which by the names; only « (2) » differs. Right data, weak wording. | `ux/05-question-light.png` | no → UX1 |
| UX-PC1b2-5 | 1 | When the inviter is reachable (the normal case) the guest sees, for about 0.3 s, the waiting page « En attente de la personne qui vous a invité… / Revenir à mon document » before the document opens, then a brief « ? » avatar for the inviter and « Vérification… ». A flash of the page that means « it may take long ». | `ux/rec-first-dark-contact-sheet.jpg` (frames 12 to 15), `ux/rec-first-dark-08-763ms.jpg` | no → UX1 |
| UX-PC1b2-6 | 1 | The waiting page keeps the full chrome of a document that is not open: « Partager » enabled, file list « main.typ / principal », header title « Sans titre / main.typ », footer « Aucun problème · Ligne 1, colonne 1 · … · 100 % · Typst » (a lone « … » where the word count is). The status says « Connexion… » (good) but the rest is the old document's. Carried over from UX-PC1b-4 (left unchanged by decision). | `ux/d20-waiting-light.png`, `ux/d20-waiting-dark.png` | no → UX1 |
| UX-PC1b2-7 | 1 | « Revenir à mon document » ends with the previous document and no sentence (« Vous êtes revenu à votre document. »): the person chose it and sees the text, so nothing is misleading; a notice would match « Vous avez rejoint ce document. ». | `ux/d22-returned-dark.png` | no → UX1 |
| UX-PC1b2-8 | 1 | Server build: about 0.3 s of empty page (background colour only) between « Inkwell redémarre… » and the application (`ux/j10-after-confirm-dark-01-345ms.png`). On the desktop the window itself disappears for about 0.5 s (see scenario 3), expected for a restart. | `ux/j10-after-confirm-dark-01-345ms.png` | no → UX1 |

## Follow-up of the defects (integrator, 2026-11-23)

- **UX-PC1b2-1: fixed in this block** (rule J-8 of `docs/plans/2026-11-15-pc1b-join.md`). Root cause: the inviter's page guessed
  that an invitation was used when a new person connected (`workspace.peerUp`); a guest who redeems the ticket and then cancels
  never connects, so the guess never fired, while Go had taken the ticket at the admit (J-1). Go now answers what became of the
  invitation (`Session.InvitationState`: open, used, expired, from `invite.Host.Outstanding`), the page asks it when « Partager »
  opens, when the member list changes and when someone connects, and a used or expired invitation is never offered to copy. The
  red test passes, unchanged. → PC1b
- **UX-PC1b2-2: fixed in this block** (rule J-9). The way back no longer ends with the wait: `CompleteJoin` keeps the record's
  `previous` in `project.json`, `ReturnToPrevious` accepts it after the join, and the main menu offers « Revenir à mon document »
  (the waiting page's wording) first among its actions at every start of the joined folder. The test's pattern now also accepts
  that wording, and the test goes further: it clicks the item and waits for Bob's previous text. → PC1b
- **Found by the gate's `regression` run (2026-11-23)**: `history.spec.ts` ZZ-2 hung in `freshInvitation(alice)`: the trace shows
  `Query count` of the « Invitation » field returning it, then `Get input value` waiting 87 s for a field that was gone. With J-8
  the dialog first showed the old invitation, then Go's answer (`used`: Bob had redeemed it) replaced it a few milliseconds
  later; the person would have seen the spent invitation flash by. Fixed in the product: at each opening the invitation's row
  shows only once Go has answered (Vitest « J-8 nothing is offered before Go answers », with its in-test mutant). The same run
  showed the two `pc1b2.spec.ts` tests depending on the order of the specs (the shared Bob was already in Alice's document,
  where a join asks nothing, J-5): their guest is now a fresh profile with a document of his own, as in the walkthrough;
  the assertions are unchanged. → PC1b
- **UX-PC1b2-3 to UX-PC1b2-8 (severity 1)**: recorded for block UX1 (interface gains, `PROGRESS.md` « Cap à long terme »), as
  the residuals of the earlier walkthroughs of PC1a and PC1b. → UX1

## Previous defects (gates/PC1b/UX.md), re-walked

| id | state |
|---|---|
| UX-PC1b-1 (3) dead-end waiting page | **closed.** The waiting page has the sentence in the text colour, « Le document s'ouvrira dès que cette personne sera connectée. Vous pouvez fermer Inkwell : l'attente reprendra à la prochaine ouverture. » and « Revenir à mon document » (`ux/d20-*`). The wait comes back after a relaunch with the inviter still frozen (`ux/d21-*`). « Revenir à mon document » restarts on the previous document, writable, text intact (`ux/d22-*`), and the previous document survives a further relaunch (`ux/d23-*`). Light and dark. |
| UX-PC1b-2 (3) the kept text is neither said nor reachable | **half closed.** Said before the click (hint under « Rejoindre ») and in the question; reachable while waiting; no way back after a successful join: UX-PC1b2-2 above. |
| UX-PC1b-3 (2) sentence names a folder that is not the one created | **closed.** The question names « Sans titre (2) » and the folder created is `Sans titre (2)` (desktop, server, three runs). |
| UX-PC1b-4 (2) waiting screen contradicts itself | **mostly closed.** No « Ce projet n'a pas encore de document », status « Connexion… »; the leftover chrome is UX-PC1b2-6. |
| UX-PC1b-5 (2) no state while connecting | **closed.** « Connexion à la personne qui vous invite… », button disabled and dimmed, status « Connexion… » in the header and the status bar (`ux/04-connexion-*-30ms.png`). On a LAN the answer comes in about 80 ms, so the state is seen for one or two frames. The slow case (inviter frozen at the click) was not re-walked. |
| UX-PC1b-6 (1), -7 (1) | closed (no restart sentence after a failed attempt is not re-walked in this pass; the waiting sentence is in the text colour). |

## Scenarios walked

### 1. The question before the restart. Server build, light and dark; desktop, dark and light

1. Paul writes a page (`ux/01-paul-own-*.png`), opens « Partager » (`ux/02-share-dialog-*.png`), types his name, pastes Alice's invitation.
   Under « Rejoindre » the hint reads « Inkwell va redémarrer pour ouvrir ce document. Votre document actuel est conservé. » (`ux/03-pasted-hint-*.png`):
   said **before** the click (J-2).
2. « Rejoindre »: the button is disabled and the card says « Connexion à la personne qui vous invite… »; header and footer say « Connexion… » (`ux/04-connexion-light-30ms.png`).
   The question follows after 80 to 100 ms on this machine (desktop and server): « Rejoindre « Sans titre » ? », « Inkwell va redémarrer
   pour ouvrir ce document dans un nouveau dossier, « Sans titre (2) », à côté du vôtre. Votre document actuel ne change pas : il reste dans
   « Sans titre ». », **« Annuler » | « Redémarrer et ouvrir »** (`ux/05-question-light.png`, `-dark.png`, desktop `ux/d04-question-*`).
   - Right title: the inviter's (« Sans titre »: UX-PC1b2-4). Right folder: the first free name, and it is the one created. Right current document: named in the
     second sentence. **At a first launch** (empty default document) the second sentence is absent: no folder of its own is named
     (`ux/d04-question-dark-first.png`), as specified.
   - Focus is on « Annuler » (the safe choice), with a visible ring; Tab goes « Annuler » then « Redémarrer et ouvrir », then leaves the page (modal);
     Escape = « Annuler ». Dialog has `aria-labelledby` and `aria-describedby`; axe (WCAG A/AA) on the question and on the whole page: no violation.
     At 640 x 400 (200 % of 1280 x 800) and at 360 px the question fits (nothing cut, no horizontal scroll), forced colours legible (`ux/09-*`, `ux/09b-*`).
   - The hint under « Rejoindre » stays visible, dimmed, behind the question (two statements of the same sentence): cosmetic.
   - Cognitive walkthrough: want to do it (yes, the person pasted an invitation); notice (the question is the only live thing, modal);
     associate (« Redémarrer et ouvrir » says what happens); see it worked (the next screens). All yes. Heuristics 1, 3, 5, 6 met.
3. Cancel: « Annuler » gives the card « Invitation annulée. Pour rejoindre ce document plus tard, demandez une nouvelle invitation. » and an **empty field**
   (`ux/06-annule-light.png`, `-dark.png`, desktop `ux/d05-annule-light.png`). Plain, accurate, with the next step. The catch is on the other side: UX-PC1b2-1.
   A fresh invitation after the cancel brings the question again (desktop and server).

### 2. « Redémarrer et ouvrir ». Desktop, native window, recorded at 15 fps (`ux/rec-first-dark-contact-sheet.jpg`)

Timeline of one join with the inviter online (frame = 66 ms; the click is between frames 0 and 1):

| frame | what the person sees |
|---|---|
| 0 to 1 | the question |
| 2 | the window fades (closing animation) |
| 3 to 7 | **no window**, the desktop background, about 0.33 s |
| 8 to 10 | an empty Inkwell window, about 0.2 s |
| 11 | the shell with an empty editor, « Préparation de l'aperçu… » |
| 12 to 15 | the waiting page, about 0.27 s (UX-PC1b2-5) |
| 16 on | the joined text, writable, « Vous avez rejoint ce document. » |

From the click to the joined text: about 1.0 s of video (16 frames) on this machine. By polling: the old page answers « Inkwell redémarre… » 37 ms
after the click, the restart marker appears in the process environment (the execution in place, same pid, new startup id) at **733 to 738 ms**, the new image
answers writable with Alice's text by **1.47 s** at the latest (first read at 1.44 s after a driver pause that cannot be shorter: connecting WebKit's inspector
sooner crashes WebKit itself, `RemoteInspectorClient::inspect`, found with gdb: a driver artefact, not Inkwell). Server build: confirmation to a writable editor
in 585 ms, 750 ms, 759 ms (dark, light, dark at a first launch).

- **« Enregistrement… » (the native banner of WC1) was not seen**, neither by the inspector (it is a native GTK widget, outside the web view) nor in the 15 fps
  frames: the save finished within one frame, below the banner's own deadline. It is asserted by WC1's native tests, not by this walkthrough; a slow save was not provoked.
- **« Inkwell redémarre… » was seen in the page** (37 ms after the click, server build `ux/j10-after-confirm-light-00-84ms.png`) **but not on the desktop screen**: the window
  closes within about 130 ms, so the person on the desktop sees the window vanish and come back (frames 2 to 10), not the sentence. The question announced
  « Inkwell va redémarrer », so this is expected, and short (about 0.5 s without a window); heuristic 1 is met by the question, not by a progress sentence. Severity 1, folded into UX-PC1b2-8.
- No second Inkwell window or orphan process: the restart is in place (same pid), and nothing of the session is left after the run.

### 3. The waiting page, return, relaunch. Desktop, dark and light

The inviter is frozen (SIGSTOP) just before « Redémarrer et ouvrir ».
1. The window comes back on the waiting page after the same 0.74 s: « En attente de la personne qui vous a invité pour ouvrir le document… », the second sentence, « Revenir à mon document »,
   status « Connexion… » (`ux/d20-waiting-dark.png`, `-light.png`). Text colour, sentences and the button are legible and consistent with the neighbouring Adwaita pages. Leftover chrome: UX-PC1b2-6.
   The named wait (« En attente de {name} … ») cannot be reached: the inviter's name comes from the member list, which an unreachable inviter has not sent (unchanged from UX-PC1b-7).
2. Quit and relaunch (inviter still frozen): the same page again (`ux/d21-*`), with its button.
3. « Revenir à mon document »: Inkwell restarts (the exec at 2.2 s in this path, document visible and writable **3.1 s** after the click, longer than the join's 0.74 s;
   the wait for the end of the helpers is part of it; no sentence meanwhile on the desktop) on the previous document, « Notes de Paul sur le bureau », status « Seul » (`ux/d22-*`).
4. Relaunch: still the previous document (`ux/d23-*`), the join is gone, the waiting page does not come back. **Survives a relaunch.**
5. Inviter resumed afterwards: the guest does not rejoin by itself (correct: the person chose to return).

### 4. The joined document. Desktop and server, light and dark

- Writable at once; focus is in the text; cursors and names of both people (`ux/d30-*`, `ux/j20-*`).
- « Vous avez rejoint ce document. » shown **once**: present at the first read, gone after 8 s (`ux/d31-*`, `ux/j21-*`), not shown again after a relaunch (`ux/d33-*`).
  Dismissable with its « × ». When the folder name differs from the one announced the longer sentence exists (Vitest; not provoked here).
- The other person's screen: while Paul is on the question or has cancelled, Alice's screen is unchanged (« Seul », no sign: `ux/a10-*` to `ux/a12-*`); after the join she sees
  « 2 personnes », Paul's cursor and his text arriving (`ux/d32-alice-sees-*`, `ux/j30-alice-sees-*`). Paul's typing reached Alice in every run (7 of 7).
- Folders on disk after each join: `Sans titre` (kept) and `Sans titre (2)` (joined).

### 5. A first launch that joins at once

An empty profile, one pasted invitation, one confirmation: **1.47 s** (desktop, dark, to a writable editor showing Alice's text; ≤ 2.3 s in a first run with a longer driver pause);
server build 759 ms. The question names no own folder (`ux/d04-question-dark-first.png`). Effort: 3 clicks (« Partager », paste, « Rejoindre ») plus the confirmation. Clean.

### 6. Consistency with neighbouring screens

- Vocabulary: « personne », « document », « dossier », « invitation »; no « pair », no « projet » on the new screens, no English left. Verbs match: « Rejoindre » gives « Vous avez rejoint ce document. ».
  « Annuler » is used for the dialog's own cancel, as in the take-over dialog (« Reprendre ce document sur cet appareil ? » with a body, « Annuler » and its action).
- Components: the question is the take-over dialog's pattern (modal, title as a question, two buttons, safe default focused), the toast is PC1a's, the waiting page reuses the file-view
  card style; Adwaita tokens in light and dark; no new surface. The restart page is the same one as the take-over's (« Inkwell redémarre… » centred).
- Keyboard: the question and « Revenir à mon document » are reachable with Tab; the waiting page has no focus target of its own on arrival (focus stays on `body`): a keyboard-only person must Tab to the button (not provoked in the desktop driver).

## Not walked

- The slow case: inviter frozen at the click (the error before 30 s), and the J-4 notice « L'invitation n'a pas pu être terminée. Rien n'a changé. » (needs a failed write).
- A slow save to see « Enregistrement… » and the native « Réessayer / Fermer quand même » dialog (WC1's native tests own them).
- The real GNOME session (headless Mutter only); a light **system** theme on the desktop (the light desktop captures force `data-theme`); WebKit under Playwright (not runnable on this Fedora).
- Long, Japanese and Devanagari titles (no title can be given in the interface; Go tests).

## Method notes (for the integrator)

- The gate's own native test `TestWC1NativeClose/PC1b-join`, run in this session through the WC1 script restricted to that subtest, timed out at « gtk-ready page-ready » here (`typst: sidecar: namespace probe inconclusive`);
  the desktop walk above uses its own private-bus Mutter, where the application ran and restarted. This is not a finding about the join, and the gate's criterion must be run on the integrating machine.
- `bin/inkwell` and `bin/inkwell-server` were rebuilt from this tree before the walk; `app:build` rewrote `build/darwin/icons.icns` and `build/windows/icon.ico`, restored afterwards.

## Result

No severity 3 or 4 defect: nothing blocks the merge. Two severity 2 defects (UX-PC1b2-1, UX-PC1b2-2) go in the wave and each needs a PROGRESS block id (inventory rule); their
tests in `frontend/e2e/ux/pc1b2.spec.ts` are **red on purpose**: the integrator either fixes the product before the merge or marks them `test.fixme` with the block id, because `mise run e2e`
runs every spec of that folder. The six severity 1 defects are listed above.

Verdict: ship
