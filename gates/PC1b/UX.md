# PC1b UX walkthrough: join a document by a restart (2026-11-15)

Tester: ux-tester (ux-walkthrough skill). Tree: worktree `inkwell-pc1b`, uncommitted block PC1b. Builds: `bin/inkwell-server`
(Playwright/Chromium, 2 to 3 instances with real invitations between them) and the native desktop `bin/inkwell` (WebKitGTK under a
headless Mutter on a private bus, driven through WebKit's remote inspector, as in `gates/PC1a/UX.md`). Every launch had its
documents, profile and XDG directories set to fresh temporary directories, checked non-empty before the launch and confirmed
from `/proc/<pid>/environ`; every process started was stopped (leftover check: none). The screen was discovered first; the
plan, the code and the wording catalogue were read afterwards. Captures are in `gates/PC1b/ux/` (light and dark), the scripts
in `gates/PC1b/ux/scripts/`.

## Blocking defects (severity 3 and 4: block the merge and the release)

### UX-PC1b-1, severity 3: the waiting page is a dead end, and it comes back at every relaunch

- Capture: `ux/10-waiting-light.png`, `ux/10-waiting-dark.png`, `ux/12-relaunch-light.png`, `ux/13-waiting-partager-dark.png`.
- Repro: Paul (own text) pastes Alice's invitation, « Rejoindre »; Alice's process is frozen (SIGSTOP) the moment Paul's page
  reaches the restart page. Paul's Inkwell restarts and shows « En attente de la personne qui vous a invité pour ouvrir le
  document… » where the editor was. Observed for 24 s: nothing changes, no end. Quit Paul and relaunch Inkwell on the same
  profile (Alice still frozen): the same waiting page again (`ux/12-relaunch-light.png`).
- What the person has: no « Annuler », no link back to their previous document, no sentence about how long it may last or what
  to do (« Ouvrez-le de nouveau », « Demandez une nouvelle invitation »), and the application has no project list to open the
  other folder from. « Partager » is still enabled and says « Seul un admin peut inviter quelqu'un. » (`ux/13`), which does not
  help; pasting another invitation is the only action that does something. The menu, the file list and the preview pane look
  alive but nothing in them leads out.
- Cognitive walkthrough: will the person know what to do? No (nothing is offered). Will they see a way back? No.
  Heuristics violated: 3 (user control and freedom), 9 (recover from errors), 1 (no end, no estimate).
- Why severity 3 and not "by design": R-10 (no end by design) is a decision about the protocol, not about what the person
  can do meanwhile. The data is safe (the previous folder is intact, checked on disk), but the colleague who relaunches
  Inkwell while the inviter is offline (the normal case of Karim on a bad network, or Alice closing her laptop) cannot write
  anywhere, in this or in the previous document, until Alice is back, and has no outside-the-app way to know that. Recovery
  worked when the inviter came back (`ux/15-joined-after-wait-dark.png`: the document opened 737 ms after SIGCONT, writable,
  « Vous avez rejoint ce document. », Paul's typing reached Alice), so the state is a wait, not a loss.
- Expected: at least a sentence under the wait that says the document will open when the person is reachable and that nothing is
  lost, and an exit: « Annuler » on the waiting page (drops the held join and opens the previous document), or a way to open the
  previous document; the same exit must exist after a relaunch.

### UX-PC1b-2, severity 3: the person cannot learn, before or after, that their text is kept or where it is, and cannot reopen it from the application

- Capture: `ux/01-paul-before-rejoindre-light.png`, `ux/01-paul-before-rejoindre-dark.png`, `ux/03-joined-light.png`,
  `ux/05-paul-menu-after-light.png`, `ux/32-relaunch-after-join-dark.png`.
- Observed: before « Rejoindre » the dialog says only « Inkwell va redémarrer pour ouvrir ce document. », in small grey text
  under the button (`ux/01-…`). The second J-2 sentence (« Il s'ouvrira dans un nouveau dossier, « {folder} », à côté de celui-ci ;
  le document ouvert maintenant ne change pas. ») is shown only after the click: its value comes from the answer of `Session.Join`
  (`workspace.joinFolder`), and the page leaves for the restart page about 40 ms later. Captured through a mutation observer, it
  existed on the screen for tens of milliseconds and was never readable. So the person never reads that their current text is
  kept nor where the new document goes, which is exactly what J-2 promises under « Rejoindre ».
- After the restart the previous document is not findable from the application: the menu has no project list or « Ouvrir »
  (`ux/05-…`), the header says « Sans titre » for both the old and the new document (every document is created with that
  title; see UX-PC1b-4), and a relaunch opens the joined document (`ux/32-…`). The old text is only on disk (folder `Sans
  titre`, beside `Sans titre (2)`); the application never says where the documents folder is. A colleague who wrote a page
  before accepting the invitation sees it vanish and cannot get it back without the file manager and knowledge of the path.
- Cognitive walkthrough: step « Rejoindre »: will the person know that their page is kept? No. After the restart, will
  they see what became of it? No. Heuristics violated: 1, 5 (error prevention: the fear of losing the text is real), 6, 10.
- Answer to the brief's question 1: no, they do not understand before clicking; yes, the text really is kept on disk
  (`Sans titre` and `Sans titre (2)` both present in the documents folder, scenario 1); no, they cannot find it again by the
  application.
- Expected: the sentence that the current text stays where it is must be readable before the click (under « Rejoindre », a
  single static sentence is enough: « Le document ouvert maintenant reste dans son dossier. »), and after the restart the
  notice or the document menu must give the person a way to the previous document (a list, or at least its place).

## Other defects

| id | sev | what | capture |
|---|---|---|---|
| UX-PC1b-3 | 2 | The sentence names a folder that is not the one created. With the person's own document in `Sans titre`, the sentence read « …un nouveau dossier, « Sans titre », à côté de celui-ci… » while the folder created was `Sans titre (2)` (observed with a mutation observer, then `ls` of the documents folder; same on the desktop build). The joined title is always `Sans titre` in this build (see UX-PC1b-4), so in the default case where the person already has a document the sentence is wrong every time (and the next join gives `(3)`). Expected: the page names the folder Go actually created (the answer of the join carries it), or does not name one. | observed text in the run log of `scripts/e7.mjs`; folders in `ux/scripts/d1.mjs` run |
| UX-PC1b-4 | 2 | While waiting, the screen contradicts itself and does not say which document it is: the preview pane reads « Ce projet n'a pas encore de document à afficher. » (« projet » is not the vocabulary of the interface, and it says the opposite of the wait sentence), the status says « Seul » (server) or « Vérification… » with « 2 personnes » (desktop), the full toolbar and « Partager » stay active, the header still says « Sans titre » (the same as the old document). Expected: one coherent state: the preview pane shows nothing or the wait sentence, the status says that the person is waiting for the document. | `ux/10-waiting-light.png`, `ux/52-desktop-waiting.png` |
| UX-PC1b-5 | 2 | Inviter frozen at the moment of the click (the answer never comes): « Rejoindre » stays disabled with no message and no progress for more than 4 s (`ux/42-…`), and the error « Impossible de rejoindre. Vérifiez l'invitation et que l'autre personne a cliqué sur « Inviter ». » comes some time before 30 s (`ux/43-…`; the exact time was not measured). More than 1 s without an indicator. Pre-existing wording and behaviour, but it is the first screen of the new restart flow. Expected: a visible « Connexion… » state on the button, then the error. | `ux/42-alice-frozen-at-click-4s-light.png`, `ux/43-alice-frozen-at-click-30s-light.png` |
| UX-PC1b-6 | 1 | After a failed attempt (garbage text, used invitation) « Inkwell va redémarrer pour ouvrir ce document. » stays in the card right above « Impossible de rejoindre… »: two opposite statements. | `ux/40-garbage-light.png`, `ux/41-used-ticket-light.png` |
| UX-PC1b-7 | 1 | The named wait (« En attente de {name} … ») was never reached: the name comes from the member list, which an unreachable inviter has not delivered, so the unreachable case (the only case where the sentence stays) always reads « la personne qui vous a invité ». Not wrong, but the named sentence would be shown only for a fraction of a second. The wait sentence is dim grey on the page background (dark mode included) and takes no focus (`activeElement` is `body`), though it is a `role="status"`. | `ux/10-waiting-dark.png` |

## Scenarios walked

### 1. Paul (own text) joins Alice's document. Server build, light and dark

1. Paul writes « Notes de Paul… »; « Partager » opens « Écrire à plusieurs » (`ux/01-…-light.png`, `-dark.png`). Alice made an invitation in her own
   instance (`ux/a-invite-light.png`). Paul pastes it: the card shows « Rejoindre » and the grey note (see UX-PC1b-2).
2. « Rejoindre »: the dialog closes and the page shows the restart page, one centred sentence « Inkwell redémarre… », in dark too
   (`ux/02-after-rejoindre-light-0.png`, `ux/02-restart-page-dark.png`). Same page and wording as PC1a's take-over: coherent.
   It appeared about 83 ms after the click (measured by polling every 40 ms).
3. The application comes back: the joined document in about 0.6 s, the toast « Vous avez rejoint ce document. » (same toast
   pattern as PC1a, with its dismiss button, role `status`, still there after 6 s), Alice's caret visible, status « Synchronisé »
   a second later (`ux/02-after-rejoindre-light-3.png` to `-6.png`, `ux/03-joined-light.png`). The editor is writable; typing in it
   reached Alice (`ux/15-joined-after-wait-dark.png`, run of scenario 3). Focus is in the text (`cm-content`) after the restart; the
   toast does not take it. For a few hundred milliseconds the preview pane shows « Préparation de l'aperçu… » then « Ce projet n'a
   pas encore de document à afficher. » while the editor already has the text (`ux/02-after-rejoindre-light-3.png`, `-4.png`).
4. The previous document stays in its folder (`Sans titre`); the new one is `Sans titre (2)`. The application gives no way to the
   old one (UX-PC1b-2). A relaunch opens the joined document.
5. Time « Rejoindre » to a writable editor showing the joined text (polling 25 ms), server build: 835 ms (own text, light), 780 ms,
   732 ms, 741 ms (three further runs, same path); about 6 s of toast lifetime. Effort: 3 clicks (Partager, paste, Rejoindre),
   no further step.

### 2. First launch, joining at once. Server build, dark, and desktop build

`ux/30-first-launch-before-dark.png`, `ux/31-first-launch-joined-dark.png`: a profile with the default empty document joins the
invitation straight away: 1026 ms to a writable editor (dark run), same screens as above, the empty first document stays in its
folder (`Sans titre`) and the joined one is `Sans titre (2)`. A relaunch opens the joined document (`ux/32-relaunch-after-join-dark.png`).
Clean apart from UX-PC1b-2 (here the previous document is empty, so the loss is not felt).

### 3. The inviter goes away between « Rejoindre » and the restart. Server build, light and dark

Alice's process stopped (SIGSTOP) as soon as Paul's page reached `restart.html`.
- Waiting page: `ux/10-waiting-light.png`, `ux/10-waiting-dark.png` (see UX-PC1b-1 and -4). After 24 s unchanged
  (`ux/11-waiting-24s-light.png`); the pid is the same, the restart marker is present.
- Relaunch of Paul on the same profile with Alice still frozen: the same waiting page (`ux/12-relaunch-light.png`). Alice resumed
  (SIGCONT): the join completed 737 ms later on the page that was already open, writable, with « Vous avez rejoint ce document. »
  and Paul's text reached Alice. So the wait is a real wait, not a failure. Severity judged honestly in UX-PC1b-1.
- « Voir les personnes » and « Partager » stay usable during the wait (`ux/13`, `ux/14-waiting-people-dark.png`), but offer no exit.

### 4. Title already taken, long title, non-Latin title

- The collision case is the default case: both Alice's and Paul's documents are titled `Sans titre`; the sentence says
  « Sans titre » and the folder is `Sans titre (2)` (UX-PC1b-3).
- Long title, Japanese title (« 手順書：新入社員向け ») and Devanagari title (« हिंदी दस्तावेज़ नमस्ते »): not walked as a person.
  The interface has no way to give a document a title (no rename of the document anywhere; checked in the header, the
  menu and the dialogs), the title is signed at creation as `Sans titre`, and editing `title` in the inviter's `project.json`
  before the launch changed neither the joined title nor the folder (the joined folder was still `Sans titre (2)` in all three
  runs), so the name rules of J-6 for such titles cannot be exercised through the application. They stay Go tests.

### 5. A failed join

- Reachable without touching product code: garbage text, an invitation already used by another person, an inviter that does not
  answer. The sentence is the existing « Impossible de rejoindre. Vérifiez l'invitation et que l'autre personne a cliqué sur
  « Inviter ». » in the warm colour of the other errors, readable, plain French (`ux/40-garbage-light.png`, `ux/41-used-ticket-light.png`). Defects
  UX-PC1b-5 and -6.
- « L'invitation n'a pas pu être terminée. Rien n'a changé. » (J-4: the failure or crash after the held join, before the install,
  on the next start): not walked. No legitimate way to provoke it from the screen (it needs a failed flush within the shutdown
  or a crash between the record and the install); it exists only in Vitest and Go tests.

### Desktop build (native window, headless Mutter), scenario 1 replayed

`ux/50-desktop-paul-own-text.png`, `ux/51-desktop-before-rejoindre.png`, `ux/52-desktop-waiting.png`, `ux/53-desktop-joined.png`
(the system theme of that session is dark; no light desktop capture). Paul's own text, the invitation of Alice (server build,
real p2p on one machine), « Rejoindre »:

| | result |
|---|---|
| pid before / after | 689471 / 689471 (same; the process name changes to `exe`, the restart's re-execution) |
| startup id before / after | `084ab132a8f503911707ccd754eda279` / `9f7308152e36e328f5e20e8387a12806` (new) |
| restart marker in the environment of the pid | present after, absent before |
| waiting sentence on screen | 638 ms after the click, then replaced |
| « Rejoindre » to a writable editor showing Alice's text | 774 ms (poll of about 60 ms plus the inspector connection) |
| notice, focus | « Vous avez rejoint ce document. », focus in the text |
| folders | `Sans titre`, `Sans titre (2)` |
| processes of the old image after the restart | none outside the new tree (`scripts/desktop-ps-after.txt`: 10 lines, the new image's own children); the old image's children ended during the restart (`ps` at 500 ms shows only the new image, `exe`, with its own new WebKit and sandbox processes) |

orphans: 0. The PC1a defect (WebKit children blocking the barrier) did not recur.

### Wording, coherence, accessibility

- Wording: plain French, no jargon in the new sentences (« redémarrer », « dossier », « document »; « pair » appears nowhere). « Ce projet n'a pas
  encore de document à afficher. » is the one leftover use of « projet » (existing string, new context: UX-PC1b-4). « Rejoindre »
  produces « Vous avez rejoint ce document. » (consistent verb).
- Coherence: the restart page, the toast, the dialog card are PC1a's and the neighbouring Adwaita dialogs'; the waiting
  sentence reuses the file-view card style. The one new surface (the waiting page) has no action at all, unlike the PC1a banner
  which always carries its buttons.
- Keyboard: not walked in full. Focus after the restart is in the text (observed on server and desktop); on the waiting page it
  is `body`.
- Dark: restart page, waiting page, dialog, toast and joined document are legible and consistent in dark (`ux/02-restart-page-dark.png`,
  `ux/10-waiting-dark.png`, `ux/13-…`, `ux/15-…`).

### Not walked

- J-4 sentence « L'invitation n'a pas pu être terminée. Rien n'a changé. » (above).
- Titles with a long, Japanese or Devanagari name (above).
- The named wait « En attente de {name} … » (UX-PC1b-7).
- axe (WCAG), 200 % zoom, forced colours, 360 px width: not run for this block.
- Light capture of the desktop build; the real GNOME session (only the headless Mutter was used).
- Exact time of the error when the inviter is frozen at the click (between 4 s and 30 s).

First-pass walkthrough result (superseded by the Re-judgement below): fix. Blocking: UX-PC1b-1 (waiting page with no exit, back at every relaunch), UX-PC1b-2 (the kept text and the place of the previous document are never readable or reachable). Severity 2 and 1 defects (UX-PC1b-3 to -7) go in the wave.

## Fixes after the walkthrough (2026-11-15, for the re-judgement)

The findings above are left as written. What changed since, by the integrator's decisions D1 to D3 (plan
`docs/plans/2026-11-15-pc1b-join.md`, row J-2 and the wording catalogue; `docs/autonomous-decisions.md`). No capture was
retaken here: the screens below are to be walked again.

| defect | what changed | where to look |
|---|---|---|
| UX-PC1b-2 (3) | Before the click, under « Rejoindre »: « Inkwell va redémarrer pour ouvrir ce document. Votre document actuel est conservé. ». « Rejoindre » no longer restarts anything: once the invitation is verified a question stays until the person chooses, in the take-over's dialog pattern: « Rejoindre « {title} » ? », « Inkwell va redémarrer pour ouvrir ce document dans un nouveau dossier, « {folder} », à côté du vôtre. Votre document actuel ne change pas : il reste dans « {current} ». » (the second sentence is left out when the open document is empty, as on a first launch), « Annuler » and « Redémarrer et ouvrir » (suggested). « Annuler » drops the invitation, nothing is written, the dialog « Écrire à plusieurs » is still there with the invitation text and « Rejoindre » works again. Not done: a list of documents in the application (the previous one is still reached by the folder named in the question, or by « Revenir à mon document » while the join waits) | the dialog after « Rejoindre », light and dark, narrow window; « Annuler » then « Rejoindre » again; a first launch |
| UX-PC1b-3 (2) | `{folder}` is the name about to be made: the first free one (« Sans titre (2) » when « Sans titre » exists), computed when the invitation is verified and checked again when the folder is made. If the name was taken in between, the notice after the join says « Vous avez rejoint ce document. Il est dans le dossier « {folder} ». » | the question, then the documents folder |
| UX-PC1b-1 (3) | The waiting page now holds: the waiting sentence (in the text colour, no longer dimmed); « Le document s'ouvrira dès que cette personne sera connectée. Vous pouvez fermer Inkwell : l'attente reprendra à la prochaine ouverture. »; and the action « Revenir à mon document » when the document open before the join is still in its folder. The action restarts Inkwell on that document (« Inkwell redémarre… », another startup id), writable, with no notice; later launches open it too. The join keeps waiting in its own folder and resumes if that folder is opened. A refused return (the folder is gone) says « Impossible de revenir à votre document. Rien n'a changé. ». Not done: an « Annuler » of the join itself on the waiting page (the inviter's list already names this device) | the waiting page with the inviter frozen, light and dark; the action; a relaunch after it |
| UX-PC1b-4 (2) | While a join waits the preview pane is not shown (« Ce projet n'a pas encore de document à afficher. » is gone from that screen) and the status reads « Connexion… » in the header and in the status bar, never « Seul ». Unchanged: the header title, the toolbar and « Partager » | the same screen |
| UX-PC1b-5 (2) | While « Rejoindre » waits for the answer the button is disabled and marked busy, and the card says « Connexion à la personne qui vous invite… » (announced as a status). The time before the error is unchanged | the inviter frozen at the click |
| UX-PC1b-6 (1) | After a failed attempt the restart sentence is not shown: only the error | garbage text, a used invitation |
| UX-PC1b-7 (1) | The waiting sentence is in the text colour. The named sentence still shows only when the member list names the inviter. Focus on the waiting page is unchanged | the waiting page |

Checks that hold these: Vitest « PC1 join says that Inkwell restarts », « PC1 a join waits for the confirmation, and
« Annuler » drops it », « PC1 the confirmation of a join: « Annuler », and a first launch names no folder of its own »,
« PC1 the waiting page says what to do and offers the way back », « PC1 the restart page asks Go for the return, and a
refused return says so », « PC1 the joined notice names the folder when it is not the one announced »; Go
`TestJoinNamesTheFolderCreated`, `TestReturnToPreviousSurvivesARelaunch`, `TestReturnToPreviousRefuses`; Playwright
« PC1-E2E a waiting join returns to the previous document » and the extra confirmation step in the three join scenarios.
The verdict line above is the walkthrough's and stays until the screens are judged again.

After the re-judgement below (2026-11-15), not walked again:

- **RJ-1, fixed.** Cause: an invitation is used once; the person who invites admitted this device before the question was
  shown, and « Annuler » drops what was received, so the same text is refused as not valid (Go `TestJoinCancelledInvitationIsUsed`).
  After « Annuler » (and when the edits could not be saved in time) the field is now emptied and the card says, where the
  restart sentence was and with no error: « Invitation annulée. Pour rejoindre ce document plus tard, demandez une nouvelle
  invitation. », until the next « Rejoindre ». Vitest « PC1 a cancelled invitation leaves the field, and the page asks for a
  new one » and its mutant. Where to look: « Annuler » in the question, light and dark.
- **RJ-2, open, residual for the interface block UX1** (`gates/PC1b/SIMPLIFY.md`, « After the re-judgement »): proposed line
  after the return, « L'invitation reste en attente dans le dossier « {folder} ». »; it needs Go (the start after a return
  is silent and knows no folder name), so it was not done as a page change.

## Re-judgement (2026-11-15, same day, fresh binaries)

Tester: ux-tester. `mise run e2e:build` and `mise run app:build` were re-run from this tree before the walk (`bin/inkwell-server`,
`bin/inkwell` rebuilt). Same method, fresh temporary directories for documents, profile and XDG, confirmed from `/proc/<pid>/environ`
(the desktop run too); every process started was stopped (leftover check: none). The coordinator's claims were not trusted; each was
checked on screen. Captures: `ux/rejudge/` (light and dark; scripts in `ux/rejudge/scripts/`). The machine was loaded (load average
about 23): timings are upper bounds.

### 1. Own text, join, cancel, confirm (Paul; server build, light and dark)

- `ux/rejudge/r1-01-before-light.png`: under « Rejoindre » one note, « Inkwell va redémarrer pour ouvrir ce document. Votre document actuel
  est conservé. » (readable, small grey, before the click: the first half of UX-PC1b-2 is closed).
- `r1-02-confirm-light.png`, `r1-02-confirm-dark.png`: « Rejoindre « Sans titre » ? » / « Inkwell va redémarrer pour ouvrir ce document dans un nouveau
  dossier, « Sans titre (2) », à côté du vôtre. Votre document actuel ne change pas : il reste dans « Sans titre ». » / « Annuler » (focused),
  « Redémarrer et ouvrir » (suggested). It stays until chosen (checked 1.5 s later), the folder named is the one created (`ls` of the documents
  folder after the join: `Sans titre`, `Sans titre (2)`), nothing is written before the choice (folders unchanged after the question). Everything
  the person needs is there: that Inkwell restarts, where the new document goes, that the current one is kept and where. Same dialog pattern as
  PC1a's take-over. Walkthrough answers: want (yes), notice (yes), associate (yes), see what changed (yes). UX-PC1b-2 and UX-PC1b-3: closed.
  Residual: the question and the header both say « Sans titre » for two different documents (every document has that title), so
  « il reste dans « Sans titre » » is exact but does not tell them apart by title; the folder name is the only handle.
- « Annuler » (`r1-03-cancelled-light.png`): the question closes, « Écrire à plusieurs » is back with the invitation still in the field, the text is intact,
  same process, no folder made. **Defect RJ-1 (severity 2):** a second « Rejoindre » with the same invitation does not work: the card answers in about
  60 ms « Impossible de rejoindre. Vérifiez l'invitation et que l'autre personne a cliqué sur « Inviter ». » (`r1-04-retry-same-invitation-light.png`, `-dark.png`),
  although the dialog kept the text as if it could be reused (the fix note says « Rejoindre » works again; it works only with a fresh invitation: the inviter
  clicked « Inviter » again and the new text opened the question at once, `r1-04-confirm2-light.png`). The existing error sentence points at the right remedy, hence
  severity 2; expected: clear the field on « Annuler », or say that the invitation was used and a new one is needed.
- Confirm: restart page « Inkwell redémarre… », the joined document writable after 704 ms (light) and 725 ms (dark) from « Redémarrer et ouvrir »; notice
  « Vous avez rejoint ce document. »; focus in the text (`r1-05-joined-light.png`, `-dark.png`).

### 2. Inviter away: waiting page, « Revenir à mon document », relaunch, inviter back (light and dark)

- Alice frozen when Paul's page reaches the restart page. Waiting page (`r2-01-waiting-light.png`, `r2-01-waiting-dark.png`): « En attente de la personne qui vous a invité pour
  ouvrir le document… » in the text colour, « Le document s'ouvrira dès que cette personne sera connectée. Vous pouvez fermer Inkwell : l'attente reprendra à la
  prochaine ouverture. », the button « Revenir à mon document », status « Connexion… » (header and status bar), no preview pane, no « Ce projet n'a pas encore… ».
  The contradictions are gone (UX-PC1b-4 closed but for the header title, still « Sans titre »). Focus stays on `body` (sev 1, unchanged).
- Relaunch while waiting: the same waiting page returns, as the sentence says, and when Alice came back the joined document opened 339 ms later
  (`r4-01-relaunch-while-waiting-light.png`). The sentence is true.
- « Revenir à mon document »: « Inkwell redémarre… », then the previous text back and writable after 412 ms (light) and 1207 ms (dark, loaded machine), no notice, focus in the
  text, typing works (`r2-02-back-light.png`). A relaunch of Inkwell opens that previous document with the new typing (`r2-03-relaunch-light.png`). UX-PC1b-1: closed
  (the dead end has an exit, the exit survives a relaunch, the previous text is intact).
- The joined folder after the return, honestly: it stays in `Sans titre (2)` and is findable again only by starting Inkwell **on that folder** (file manager, no Inkwell
  running): with Alice back it then joined in 471 ms (`r2-05-launch-on-joined-folder-dark.png`) and later launches opened it. With Inkwell already running,
  « Ouvrir avec » on that folder only shows « « Sans titre (2) » fait partie d'un autre projet. Inkwell n'ouvre qu'un projet à la fois pour l'instant. »
  (`r2-04-after-open-with-light.png`, `r2-04-open-with-running-dark.png`). **Defect RJ-2 (severity 2):** after « Revenir à mon document » nothing in the application says that
  the join is still pending in `Sans titre (2)` nor how to resume it, the wait page's « l'attente reprendra à la prochaine ouverture » is no longer true once the person went back
  (a plain relaunch opens the previous document), the app has no project list, and the toast above uses « projet ». The person is not stuck (their document is back and writable,
  and a new invitation from Alice gives a new join, in the application, though it leaves the unfinished folder behind), so severity 2, not 3. Expected: a notice after
  the return (« L'invitation reste dans le dossier « Sans titre (2) ». ») or the folder offered where « Ouvrir » would be.

### 3. First launch, joining at once (server build, light and dark)

`r3-01-first-launch-confirm-light.png`, `-dark.png`: « Rejoindre « Sans titre » ? » / « …dans un nouveau dossier, « Sans titre (2) », à côté du vôtre. » (no « reste dans » sentence: the
open document is empty, as designed) / the same two buttons. Confirm to writable: 1031 ms (light), 2453 ms (dark, loaded machine; a repeat in light 1031 ms); notice, focus in the text,
folders `Sans titre` (empty) and `Sans titre (2)` (`r3-02-first-launch-joined-light.png`, `-dark.png`). Clean.

### 4. Desktop build (native window, headless Mutter), scenario 1 replayed

`r5-50-desktop-paul-own-text.png`, `r5-51-desktop-before-rejoindre.png`, `r5-51b-desktop-confirm.png` (the question, focus ring on « Annuler »), `r5-52-desktop-waiting.png`,
`r5-53-desktop-joined.png`. The system theme of that session is dark; no light desktop capture.

| | result |
|---|---|
| pid before / after | 1286267 / 1286267 (same) |
| startup id before / after | `c0f4e58444b6e22b539ffb9fbce43480` / `90d020c6e6be565ad1e5ae7d728a4eb0` (new) |
| restart marker in the environment of the pid | present after |
| question before the restart | shown and held (startup id unchanged, only `Sans titre` in the documents folder) |
| waiting sentence seen | 908 ms after the confirmation, with its second sentence and « Revenir à mon document » |
| confirmation to a writable editor showing Alice's text | 1182 ms (loaded machine, polling 60 ms) |
| notice, focus | « Vous avez rejoint ce document. », focus in the text |
| folders | `Sans titre`, `Sans titre (2)` |
| processes of the old image after the restart | none outside the new tree (`ux/rejudge/scripts/desktop-ps-after.txt`); leftover check at the end: none |

orphans: 0.

### Defects after the re-judgement

| id | sev | state |
|---|---|---|
| UX-PC1b-1 | 3 | closed: exit on the waiting page, survives a relaunch, previous text intact |
| UX-PC1b-2 | 3 | closed: the question is readable before and after the click, names the folder created and where the current document stays |
| UX-PC1b-3, -4, -5, -6 | 2, 2, 2, 1 | closed (the sentence names the created folder; no contradicting text while waiting; the busy sentence on the button was visible in the waiting run but its time was not re-measured; no restart note next to an error) |
| UX-PC1b-7 | 1 | open: waiting page takes no focus (`body`); named wait still unreachable |
| RJ-1 | 2 | open: same invitation after « Annuler » fails; field not cleared |
| RJ-2 | 2 | open: after « Revenir à mon document » no notice about the pending folder, the folder is reachable only by launching Inkwell on it; « projet » in the refusal |
| RJ-3 | 1 | open: header, question and file list show « Sans titre » for both documents |

Not walked again: J-4 sentence « L'invitation n'a pas pu être terminée… » (no legitimate trigger), long and non-Latin titles (no way to set a title), axe, 200 % zoom,
forced colours, 360 px width, the keyboard pass, the real GNOME session, a light desktop capture. No severity 3 or 4 defect remains; the severity 2 and 1 ones go in the wave.

Verdict: ship
