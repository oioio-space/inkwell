# PC1a UX walkthrough: take over a document by a restart (2026-11-15)

Tester: ux-tester (ux-walkthrough skill). Tree: worktree `inkwell-pc1a`, uncommitted block PC1a. Builds: `bin/inkwell-server`
(`mise run e2e:build`, driven with Playwright/Chromium) and the native desktop `bin/inkwell` (`mise run app:build`, WebKitGTK,
driven through WebKit's remote inspector, once under a headless Mutter, once on the real GNOME session). The interface was
discovered from the screen first; the plan was read afterwards. Captures are in `gates/PC1a/ux/` (light and dark).

## Blocking defects (severity 3 and 4: block the merge and the release)

### UX-PC1a-1, severity 3: on the desktop build the take-over never restarts Inkwell; the application quits after 5 s

- Capture: `ux/12-desktop-restart-page-0.png` (restart page), `ux/11-desktop-dialog.png` (dialog on the desktop), evidence in
  `ux/m4-headless/` and `ux/m4-gnome-session/` (`sampler.log`, `ps-*.txt`, `app-error-lines.txt`).
- Repro: `bin/inkwell` as a second profile (`INKWELL_HOME`, `INKWELL_DOCS`, `XDG_*` isolated) on a folder written by another
  profile, click « Reprendre ce document ici », confirm.
- Observed (both Mutter headless and the real GNOME session, identical): the window stays on « Inkwell redémarre… » and stops
  repainting 27 ms after the confirmation; 5.0 s later the process logs
  `restarting after a project change err="restart: 3 child processes still running: [WebKitNetworkProcess bwrap bwrap]"`,
  prints « Inkwell ne redémarre pas: Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau. » and exits. No new image
  ever starts. Reopening the folder shows the document writable with no banner and no notice (the take-over itself was
  committed), so the person must relaunch by hand and is never told that it worked.
- Expected (N-14, N-12): the barrier holds, the same pid executes the new image, a new startup id, the window comes back
  writable with the notice and the focus in the text.
- Which process outlived the window (M4, below): the three direct children that belong to WebKit, for the whole 5 s of the
  barrier and until the parent exited; they ended 15 ms after the parent did.

### M4, measurement on the desktop build (mandatory)

Process tree of the old image (pid 3672638 in the headless run; names as `ps` shows them):

| moment | direct children of the application | note |
|---|---|---|
| just before the request | WebKitNetworkProcess 3672732; bwrap 3672745 (-> bwrap 3672749 -> xdg-dbus-proxy 3672750); bwrap 3672753 (-> bwrap 3672754 -> WebKitWebProcess 3672755); exe 3672996 (-> tinymist 3673007) | 4 children, 10 processes |
| confirmation (t0) | same | |
| t0 + 16 ms | the `exe` holder and tinymist are gone | the language service ends in time (N-14 holds for it) |
| t0 + 1.5 s, 3.5 s, 4.8 s | WebKitNetworkProcess, bwrap (xdg-dbus-proxy), bwrap (WebKitWebProcess): 3 children, 8 processes | the barrier waits; nothing ends |
| t0 + 5.04 s | the same 3 children; `restart: 3 child processes still running: [3672732 3672745 3672753]` | barrier timeout, restart abandoned |
| t0 + 5.06 s | the application process exits | no exec: the pid does not live on, no new startup id |
| t0 + 5.08 s | the 3 helpers are gone (parent-death) | |
| t0 + 6 s and after (`ps-1-at-8000ms.txt`) | nothing of inkwell left | |

Real GNOME session run: the same three children, the same 5 s, exit at t0 + 5.08 s (`ux/m4-gnome-session/sampler.log`).

Same pid and new startup id: not observable, the restart did not happen (no process with `INKWELL_AFTER_TAKEOVER=1` ever existed).
Orphans from the old image once the old process ended: none left alive after 15 ms (sampler and the listing at 8 s).

orphans: 0

That figure is about leftovers only; it must not be read as a pass: the restart itself failed at the barrier (UX-PC1a-1).
Likely cause to look at, not verified here: WebKit's helper processes are children of the application until the web context is
finalised, which happens at process exit, so a barrier that waits for them before the exec can never be satisfied; the
sandbox holder and tinymist, which the code stops explicitly, do end.

## Other defects

| id | sev | what | capture |
|---|---|---|---|
| UX-PC1a-2 | 2 | Server build, restart fails after the request (provoked without touching code: environment padded with two 100 KB variables, then `prlimit --stack` lowered, so the exec fails with « argument list too long »): the process exits, and the browser tab stays on « Inkwell redémarre… » for good (observed 46 s, no end). No sentence, no timeout, no way out but closing the tab. Expected: after a bound the restart page says that Inkwell stopped and what to do (the Go notification is only a desktop notification). | `ux/30-fail-light-1.png` |
| UX-PC1a-3 | 2 | Keyboard order: the two banner buttons are reached after about 19 Tab stops (preview controls, status bar, file list, header), although the banner sits right under the toolbar. Same for W58a's « Demander une invitation » (inherited); the new action is the one that makes the banner worth reaching. Expected: the banner comes in reading order, before the editor. | `ux/42-keyboard-focus-takeover-button.png` |
| UX-PC1a-4 | 2 | Forced colours (Léa): the banner buttons and the dialog's « Annuler » / « Reprendre ce document ici » lose their button shape; they read as plain text, the suggested action is not distinguishable from Annuler, the dialog has no outline against the page (text of the page shows around it). Probably app-wide, but the confirmation is the one place where the choice matters. | `ux/41-lea-dialog-200pct-forced.png`, `ux/40-lea-banner-200pct-forced.png` |
| UX-PC1a-5 | 2 | Narrow window (360 to 480 px): the file list keeps its width (about 264 px), the header controls overlap, and at 360 px the banner is squeezed to 66 px wide, its text one word per line and its two buttons clipped. Present without the banner too (`ux/43-own-project-360-no-banner-reference.png`): inherited S13 debt, not caused by PC1a, but the banner now carries the only way out of a read-only document. Order at 480-800 px is good (the two buttons stack, « Demander une invitation » first, « Reprendre ce document ici » second, no horizontal scroll). | `ux/02-banner-w360.png`, `ux/02-banner-w480.png` |
| UX-PC1a-6 | 1 | After the restart the caret is at line 1, column 1: typing without clicking prepends to the title line (`Frappe au clavier.Procédure d'accueil`). The reading position the person had is not kept. | `ux/06-after-light.png` |
| UX-PC1a-7 | 1 | At about 1000 px of banner width the two buttons wrap unevenly: « Demander une invitation » sits next to the text, « Reprendre ce document ici » alone below, centred (1100 px browser window: both on one row below the text). | `ux/10-desktop-banner.png` |
| UX-PC1a-8 | 1 | After a desktop failure and a manual relaunch the document is the person's but nothing says so (no « Ce document est maintenant le vôtre… »); the only sentence they saw was the earlier « Inkwell s'est fermé… ». Resolves with UX-PC1a-1. | `ux/13-reopen-after-failed-desktop-restart.png` |

## Scenarios walked

### S-A, Paul (writer, keyboard-heavy) takes over the document of Alice's other device. Server build, light and dark

1. Alice's profile writes two lines and quits; Paul's profile opens the same folder. `ux/01-paul-banner-light.png`: blue read-only
   banner, « Ce document vient d'un autre appareil. Vous pouvez le lire et en copier le texte, mais pas le modifier ni le
   partager. » and two buttons « Demander une invitation » and « Reprendre ce document ici ». Walkthrough: want to (yes, the
   sentence says what is wrong), notice (yes, banner under the toolbar), associate (the label says the goal; « ici » is a little
   vague but the dialog explains), see it worked (yes, see 5).
2. Click: dialog `ux/03-dialog-light.png` / `ux/03-dialog-dark.png`: title « Reprendre ce document sur cet appareil ? », body that says
   Inkwell restarts, what happens to the people who shared it, and that text and files do not change; « Annuler » and the blue
   suggested action « Reprendre ce document ici ». Coherent with the other Adwaita dialogs (rounded card, title, body, two
   buttons, the suggested action on the right, focus on « Annuler », Escape cancels). The confirming button repeats the verb of
   the banner button (good). The body is four sentences: long but each one is needed.
3. Confirm: restart page `ux/05-restart-light-1.png` / `ux/05-restart-dark-0.png`: one centred sentence « Inkwell redémarre… », system
   theme respected (dark stays dark). A blank frame (page colour, about 60-100 ms) shows between the restart page and the application
   (`ux/05-restart-light-2.png`); no light flash in dark mode.
4. The application comes back: `ux/06-after-light.png`, `ux/06-after-dark.png`: no banner, editor writable, same text, preview rendered.
5. Feedback: the toast « Ce document est maintenant le vôtre sur cet appareil. » (role `status`, with a labelled dismiss button) shows
   once, 7 s, bottom centre; focus is in the text (`cm-content`), the toast does not take it. Typing goes into the document
   (`ux/07-typed-light.png`, `ux/07-typed-dark.png`); reloading the page shows no second notice (e2e covers it).
6. Time from confirmation to a writable editor on the server build: 684 ms (light), 466 ms (dark), 1339 ms keyboard run (includes the
   50 ms poll of the script and the page's own wait). Effort (SEQ-style): 2 clicks, no typing, no field to read; keyboard: about 19 Tab,
   Enter, Enter. No step above 1 s without an indicator (the restart page is the indicator).

### S-B, cancel (Paul). Escape or « Annuler »

`ux/04-after-cancel-light.png`: the dialog closes, nothing changed, the banner stays and the focus returns to the button that opened
the dialog (visible focus ring). Tab inside the dialog: Annuler, then the action; a third Tab leaves the dialog for the page
(browser behaviour of a modal; the neighbouring dialogs of the app were not compared one by one).

### S-C, Léa (low vision): 200 % equivalent (550 x 320 px) and forced colours

`ux/40-…` and `ux/41-…`: nothing is cut off or scrolls horizontally inside the dialog (scrollWidth equals clientWidth); the text is
readable. axe (WCAG A and AA): dialog, no violation; banner screen, one `scrollable-region-focusable` on the editor's `.cm-scroller`
(pre-existing, not PC1a). Defects UX-PC1a-4 and UX-PC1a-5.

### S-D, member profile (Alice invites, Paul joins by invitation). Server build

`ux/21-member-paul.png`: Paul, a member, sees no banner and no « Reprendre ce document ici », writes, and Alice sees his text
(`ux/22-alice-sees-paul.png`). The foreign action does not exist for a member. Clean.

### S-E, refusal sentence

The `?takeOver=refused` return of the restart page (what it loads when Go refuses or fails; reached here by opening that address,
the path where Go answers a refusal could not be provoked without editing code): `ux/32-refused-sentence-light.png`, `ux/32-refused-sentence-dark.png`:
the banner is back, the toast says « Le document n'a pas pu être repris. Rien n'a changé. » (role alert, shown once, address cleaned,
focus in the text). Plain French, true in that path. The other failure (restart fails after the write, UX-PC1a-1 and -2) is told
differently (« Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau. », desktop notification only) and there the take-over
did happen, so « Rien n'a changé » would have been false; the two sentences are right to be separate.

### Not walked

- `removed` device (« Vous ne faites plus partie de ce document… » with the same two buttons): no screen of the application can make a
  device removed (« Déconnecter » in the people panel only suspends; a reopened Paul showed no banner), so it was seen only in Vitest.
  One point to decide: a removed person is offered « Reprendre ce document ici ».
- Karim (bad network): a flush that exceeds 5 s (« Le document n'a pas pu être enregistré. Rien n'a changé. ») needs a stalled storage
  stream; not provoked.
- Server-build tab after a failed restart: provoked with an exec failure, see UX-PC1a-2.

## Incident of the tester

A first failure run (`f.mjs`) was started with an empty `INKWELL_DOCS`, so the isolated profile opened the user's real
`~/Documents/Inkwell/Sans titre` and the take-over was confirmed there: the member list was replaced by a throwaway identity's genesis
and archived by copy. It was restored at once from that archive (byte-identical, the operation file's name is its SHA-256, checked),
the new `trust-archive/` and the backup made by that open were removed; `main.typ` and `repo/` were never touched. The user should
check that the document still opens as theirs. This is also a reminder of how easy the action is to trigger on a real folder:
the dialog is the only guard (it was clicked through by a script).

## Verdict

(superseded by the Re-judgement below)

Blocking list: UX-PC1a-1 (desktop take-over fails at the termination barrier: WebKitNetworkProcess, bwrap/xdg-dbus-proxy and
bwrap/WebKitWebProcess outlive the window for the whole 5 s; Inkwell quits instead of restarting). UX-PC1a-2 to -5 are severity 2
and go in the wave; the server-build path (S-A, S-B, S-D) is otherwise clean.

## Fix of UX-PC1a-1 (2026-11-15, by the implementer; to be re-judged by the walkthrough)

This section is a measurement, not a verdict: the findings and the verdict line above are the walkthrough's and are unchanged.

What changed. The termination barrier no longer waits for processes Inkwell does not control. Once `stopAll` returned (its own
services ended, as before) the process makes itself a child subreaper, asks every remaining child to end (SIGTERM), kills it
when it still runs 100 ms later (SIGKILL), reaps, and repeats until it has no child or the 5 s deadline passes; only with no
child does it execute, and past the deadline it fails as before, naming the survivors by pid and name
(`internal/sidecar/endchildren_linux.go`, called by `barrier` in `cmd/inkwell/restart_linux.go`; rule N-14 of the plan, reworded).
Tests: `TestEndChildren` (a child that ignores SIGTERM; one that ends on request and leaves a process behind, reaped through the
subreaper; the deadline failure naming the survivor), `TestRestartBarrier` extended with a helper the session does not control,
mutants MW16 and MW17.

Method: the walkthrough's own (its scripts, reused: WebKit's remote inspector, a 10 ms process sampler, `ps` trees), on the
desktop build `bin/inkwell` rebuilt from this tree (`mise run app:build`), second profile on a folder written by another
profile, every path of the run in a fresh temporary directory (`INKWELL_DOCS`, `INKWELL_HOME`, `XDG_DATA_HOME`,
`XDG_CACHE_HOME`, `XDG_CONFIG_HOME`, checked non-empty before the launch). Once under a headless Mutter on a private bus, once
on the real GNOME session. The startup id is read from the page through the runtime's own call to `Session.StartupID`.
Evidence: `ux/fix-headless/` and `ux/fix-gnome-session/` (`sampler.log`, `drive.out`, `ps-*.txt`, `after.png`; the scripts
`run.sh`, `desk.sh`, `drive.mjs` are in `ux/fix-headless/`).

| | headless Mutter | GNOME session |
|---|---|---|
| pid before / after | 3796775 / 3796775 (same) | 3799124 / 3799124 (same) |
| startup id before | `07e3cbfa1bec13ea891ffb5b39b0d215` | `94b023469f36ed317c21fe4d97f643bb` |
| startup id after | `73763ab2eaad54e81e63534e561a7969` (new) | `fdae299f60b6d73b9ef248f0eec10756` (new) |
| new image (restart marker in the environment of the pid) | 171 ms after the confirmation | 180 ms |
| application back, writable, no banner | at most 1023 ms (150 ms poll) | at most 1024 ms |
| notice | « Ce document est maintenant le vôtre sur cet appareil. », focus in the text | the same |
| `still running` in the application's log | 0 lines | 0 lines |

Process tree of the old image, headless run (time from the confirmation; `sampler.log`, 10 ms period):

| moment | children of the application | note |
|---|---|---|
| before the request | WebKitNetworkProcess 3796872; bwrap 3796887 (-> bwrap 3796888 -> xdg-dbus-proxy 3796889); bwrap 3796892 (-> bwrap 3796893 -> WebKitWebProcess 3796894); exe 3797078 (-> tinymist 3797089) | 4 children, 10 processes |
| + 18 ms | the language service and tinymist are gone; the 3 WebKit children remain | ended by the shutdown, as before |
| + 33 ms (at the barrier) | bwrap 3796888 (-> xdg-dbus-proxy), bwrap 3796893 (-> WebKitWebProcess) | WebKitNetworkProcess and the two outer bwrap ended on SIGTERM; the two inner ones came to the application (subreaper) instead of init |
| + 138 ms | bwrap 3796893 only | the inner bwrap is the init of a pid namespace and takes no SIGTERM from outside: killed after the 100 ms grace, its namespace with it |
| + 154 ms | none | the barrier holds |
| + 171 ms | none; same pid, `INKWELL_AFTER_TAKEOVER=1` in its environment | the new image |
| + 232 ms and after | the new image's own WebKitNetworkProcess, bwrap, bwrap, then its language service | |
| after quitting the new image (`ps-3-after-quit.txt`) | nothing of Inkwell | |

GNOME session run: the same sequence (language service gone at + 38 ms, inner bwrap re-parented at + 56 ms, no child and
the new image at + 180 ms).

Processes of the old image alive outside the application's tree at any sample (`reparented_old_alive`), both runs: none.

orphans: 0 (after the fix: the restart happened, same pid, new startup id, in both runs)

A first run with a 500 ms grace restarted 566 ms after the confirmation: the inner bwrap lived the whole grace. The grace was
lowered to 100 ms for that reason (the helpers have nothing left to save: the page flushed and left its context, N-15).

Also changed after the walkthrough, for re-judging (each with its test):

- UX-PC1a-2: the restart page gives up after 15 s (three times the barrier's 5 s; a restart measured 0.5 to 1.4 s) with nothing
  or the same image answering: « Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau. » (`role="alert"`, key
  `change.restartFailed`, the sentence of rule N-12); it goes on asking and loads the application if an image answers later.
  Vitest « PC1 the restart page gives up when no other image answers », its mutant, « PC1 a restart within the bound never shows the failure ».
- UX-PC1a-8: a change whose restart failed leaves a mark in the profile folder (`restart-failed`, the project folder); the next
  launch that opens that folder says « Ce document est maintenant le vôtre sur cet appareil. » (or the failure sentence, from
  the standing it loaded), once. `TestRestartFailedThenLaunchedByHandSaysOnce`.
- UX-PC1a-4: in forced colours `.btn` and `.sug` keep a border, the suggested action keeps a fill in the system's highlight
  colours, and the take-over dialog has an edge. UX-PC1a-5: a banner action never leaves the banner (its label wraps, a long
  word breaks). e2e « PC1-E2E the banner's actions stay whole in a narrow window and keep a shape in forced colours » (360 and
  480 px; red before at 360 px). The squeezed banner itself at 360 px (the file list keeps its width) is S13's and unchanged.
- UX-PC1a-3 (tab order), -6 and -7: not changed; UX-PC1a-3 is a residual in `SIMPLIFY.md`.

## Re-judgement (2026-11-15, same tester, rebuilt binaries `e2e:build` and `app:build` at 10:29)

Done again by me, not taken from the fixer's evidence. Every launch used fresh temporary folders (docs, home, data, cache), checked
non-empty before launch; every mutter, server and app I started was stopped (checked with `ps`).

- Desktop take-over, headless Mutter, two runs (`ux/rejudge/sampler-light.log`, `sampler-dark.log`; the Mutter theme was dark in
  both, so the desktop pair is not a light/dark comparison): the old image's 3 WebKit children and tinymist end within about 140 ms
  of the confirmation, the barrier holds, the same pid carries `INKWELL_AFTER_TAKEOVER=1` 150 ms after the confirmation, the new
  image starts its own helpers, and the window is back and writable 683 ms (light run) and 665 ms (second run) after the
  confirmation (including my 100 ms poll). The notice shows once, the focus is in the text, no banner (`ux/rejudge/d-after-light.png`,
  `d-after-dark.png`; banner and dialog: `d-banner-*.png`, `d-dialog-*.png`). `reparented_old_alive` is empty in every sample of both runs.

orphans: 0

- Server build, failed restart (exec made to fail as before): the restart page now says « Inkwell s'est fermé sans pouvoir
  redémarrer. Ouvrez-le de nouveau. » at 15.2 s (role alert; `ux/rejudge/fail-light-147.png`); the log says `argument list too long`.
  UX-PC1a-1 and UX-PC1a-2 are closed.
- Forced colours (550 px): buttons have borders, the suggested action is filled, the dialog has an edge
  (`ux/rejudge/forced-colors-w550-dialog.png`). UX-PC1a-4 closed.
- Dialog and banner, 1100 px and 360 px, light and dark: `ux/rejudge/w1100-*`, `w360-*`. The dialog fits at 360 px. At 360 px the
  banner is still squeezed to about 66 px by the fixed-width file list: its buttons now stay inside the banner but their labels break
  mid-word (« Dem / ander une invit… ») and the banner is taller than the visible area. UX-PC1a-5 is therefore only partly closed.

Residuals, severity 2, none blocking (S13 layout debt and tab order, not caused by this block): UX-PC1a-3 (banner buttons about 19
Tab stops in), UX-PC1a-5 (banner at 360-480 px with the fixed-width file list). Severity 1: UX-PC1a-6 (caret at line 1 after the
restart), UX-PC1a-7 (uneven wrap near 1000 px). Not walked: removed device, Karim's flush timeout. No severity-3 or 4 defect remains.

Verdict: ship
