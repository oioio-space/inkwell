# OI ux-walkthrough (2026-11-13)

Method: skill `ux-walkthrough`, discovery from the screen first, then the plan. Real `inkwell-server` processes (Chromium,
fr-FR), a second launch started as the desktop does (same profile, file as argument), light and dark. Harness:
`frontend/e2e/ux/oi.spec.ts` (captures, `UX_OI_SHOTS=<dir>`), defects: `frontend/e2e/ux/oiDefects.spec.ts` (3 tests, all
red today). Captures: `gates/OI/ux-shots/` (`NN-<row>-<light|dark>.png`). Personas: first-time user, hurried user.

## Blocking (severity 3-4)

None found.

## Defects

| Id | Sev | Screenshot | Repro | Expected |
|----|-----|-----------|-------|----------|
| UX-OI-1 | 2 | `06-burst-{light,dark}` | Three loose files opened at once (select three, « Ouvrir avec Inkwell »; Windows starts one process each): the single toast slot is replaced each time, only « trois.md … » stays; two requests are never reported | one toast naming all, or a count (« 3 fichiers n'ont pas été ouverts »); never a silent replacement. Test `UX-OI-1` |
| UX-OI-2 | 2 | `05-R8-several-{light,dark}` | `a.md` (in the project), `absent.md`, a loose file: toast « « absent.md » est introuvable. Et 1 autre. » | say what the « autre » is (name it, or « 1 autre fichier n'est dans aucun projet »); the shown file is only visible in the sidebar. Test `UX-OI-2` |
| UX-OI-3 | 1 | `04-R19-failed-{light,dark}` | failure toast « Inkwell n'a pas pu ouvrir « .cache.md ». » has `role="status"` while `Toast.svelte` documents `alert` for what failed (Léa, screen reader) | `role="alert"` for failures (R19, R9). Test `UX-OI-3` |
| UX-OI-4 | 1 | `02-R6-loose`, `03-R5-other` | Interim toasts R5/R6 are a dead end: no action, vanish after 7 s; accepted by the user (Q3), lives one block until OI3 | none now; OI3 replaces it. Keep the 7 s but make sure OI3 ships before a release |

## Fixes (2026-11-13)

| Id | Fix | Proof |
|----|-----|-------|
| UX-OI-1 | The requests that end within one burst (800 ms without one ending) give one toast per kind, naming up to three files then « et N autre(s) »: « 3 fichiers ne sont dans aucun projet : « un.md », « deux.md », « trois.md ». … »; several files shown: « N fichiers ouverts : … ». Toasts follow one another instead of replacing each other (`notice.queue`, AdwToastOverlay's queue); same `Toast` component | `oiDefects.spec.ts` UX-OI-1 green; Vitest « says a burst of files of one kind… », « names three files, then counts the others… », « says the files shown by a burst… »; e2e burst spec |
| UX-OI-2 | Go sends a notice for every argument (`launch.Outcome.Notices`, R8 amended), so the window names each one: the failure toast « « absent.md » est introuvable. », then « « brouillon.md » n'est dans aucun projet. … » | `oiDefects.spec.ts` UX-OI-2 green (the second toast follows the first, up to 12 s); Vitest « names every other argument of one request… »; `TestPlanOneToastPerRequest`, `TestResolveSeveralPaths` |
| UX-OI-3 | Failures (R9, R19) go in their own toast with `role="alert"`, each file and why, up to three then « et N autres fichiers non ouverts »; a single failure keeps its sentence | `oiDefects.spec.ts` UX-OI-3 green; Vitest « keeps the sentence of a single failure… », « says every failure of a burst in one alert toast… » |
| UX-OI-4 | Not changed: the interim toasts R5/R6 ship in R1 (decision Q3, `docs/autonomous-decisions.md`); the light single-file window of OI3 follows right after R1 | accepted residual |

## Verified clean (walked, light and dark)

- R3 `01-R3-file`: file of the project shown at once, header subtitle and sidebar selection change, no toast, second launch exits 0 in under 5 s.
- R16 (no file): nothing changes, no toast, no second project (existing test and re-walked).
- R5, R6, R19, R9 missing: wording is plain French, names the file between « », no jargon (« projet », « fichier »; no « pair », « stream »), same pill toast as neighbouring screens (dark pill, close button « Masquer la notification »), contrast fine in both themes.
- 360 px wide: the toast wraps on three lines and stays inside the window (x 16, width 328).
- Toast close button reachable and labelled.

## Not walkable in the web harness (to cover in the SEQ session)

- Raising and focusing the native window (GNOME focus-stealing, activation token), the desktop notifications R11/R17/R18 (« Inkwell ne répond pas »), the file manager's « Ouvrir avec » list (Q2 formats).
- Slow open (« L'ouverture de … prend plus de temps que prévu ») and closing the window right after a request: the bounds are 10 s and 60 s (`BOUNDS`), covered by Vitest only; judged on wording and code: the slow toast disappears after 7 s while the open continues, so a long wait ends with no sign of activity (sev 1, same family as UX-OI-4; ask the user whether a persistent indicator is wanted).

## Out of scope but seen (pre-existing, not OI; the coordinator files them in `docs/exigences.md`)

- A `.md` opened from the file manager shows « Dans cette version, ce fichier s'affiche en lecture seule » (`file.readOnly`): the person who double-clicks `notes.md` cannot edit it. Not caused by OI, but it is what an OI user meets first; worth a line in PROGRESS.
- At 360 px the header bar overlaps (« Fichiers » title under the buttons) : S13 resize, existing.
- Title shows « Sans titre » with the file name as subtitle.

## Raising the window (fix of 2026-11-13, plan §13): to re-check with the installed package, 2 minutes

Not walkable in the web harness; the request itself is proved by `mise run app:activation`. With the new package
installed, Inkwell open on a project, and Files covering its window:

1. In Files, double-click a `.typ` or `.md` of the open project: the Inkwell window comes to the front with the
   file shown; no « Inkwell est prêt » notification; the busy cursor stops at once.
2. Minimise Inkwell, then « Ouvrir avec Inkwell » on another file: the window is restored and in front. If it was
   maximised before, it is still maximised.
3. Put Inkwell on another workspace, start it again from Activities (no file): Inkwell gets the focus (GNOME
   takes you to it or brings it to you); no toast, no notification.
4. Expected exception: `inkwell fichier.typ` typed in a terminal covering Inkwell may still give « Inkwell est
   prêt » (a terminal passes no activation token; GNOME's rule). If so, click the notification and say whether the
   window comes to the front (the click did nothing on 2026-11-13; not explained yet).

## SEQ (session with the user)

done: see « User sessions » below

## User sessions (installed rpm, real GNOME Wayland session)

Verdict: **accepted by the user on 2026-11-14** after the activation fix (purego). Notes as taken, 2026-11-13 (defect found) then 2026-11-14 (re-check):

# OI user session 2026-11-13 (uninstalled build from the OI worktree, isolated profile and documents)
- Step 1, second launch with the project's main.typ from a terminal: the second process exited 0 in 0.07 s; one inkwell left. The window was NOT raised; GNOME showed the notification « io.github.oioio_space.Inkwell — « Inkwell » est prêt » (focus-stealing prevention: a terminal launch carries no XDG activation token). Notification title is the raw app id, not « Inkwell »: the build is not installed, so GNOME finds no .desktop file for the id. Both to re-check with the installed package and a Files launch (bloc 10), which passes an activation token.
- Installed with the user's permission: `pkexec dnf install dist/inkwell-0.0.0-dev.8cacc7a.x86_64.rpm` (OI worktree build 8cacc7a) at 23:15; to uninstall with pkexec at the end. Test file: ~/Téléchargements/brouillon-test-inkwell.md (to delete at the end).
- Step 1 (installed) launch from GNOME Activities: OK — name « Inkwell » and icon found, starts fine and quickly.
- Step 2 (installed) Files › right-click brouillon-test-inkwell.md › Ouvrir avec: « Inkwell » IS listed. Chosen: the window is NOT raised; GNOME still shows the « « Inkwell » est prêt » notification → DEFECT: the activation token passed by Files (XDG_ACTIVATION_TOKEN of the second process) is not used to present the existing window (xdg-activation-v1 / gtk_window_present_with_time). Toast in the window not yet reported.
- Step 2b: clicking GNOME's « est prêt » notification does nothing (window still not raised) → DEFECT (same family: the window never claims activation).
- Step 2c: after Alt+Tab, the toast says brouillon-test-inkwell.md belongs to no project: OK (shown; clarity not commented).
- Step 3 (installed) Files › main.typ › Ouvrir avec Inkwell: only a busy cursor (spinner) and nothing else visible; one inkwell process left (the second handed off and exited). Desktop entry has StartupNotify=true: GNOME waits for the startup-complete/activation that the running instance never sends with the forwarded token → same root cause as step 2.
- Step 3b: after Alt+Tab the window shows main.typ — it already did before (same file), so R3 « file shown, no toast » holds but the change was not observable.
- Cleanup 23:23: inkwell uninstalled with pkexec dnf remove; test file deleted.

## Re-check with the purego rpm, 2026-11-14 (installed 09:46 with pkexec)
- Step 1 (window covered by Files, Ouvrir avec Inkwell on the loose .md): OK — the window came to the front.
- Step 2 (window minimised with Super+H, Ouvrir avec Inkwell): OK — restored and in front. (GNOME shows no minimise button by default: not an Inkwell defect.)
- Step 3 (project main.typ from Files): OK — window in front, no other problem (no « est prêt », no stuck busy cursor reported). Verdict: defect of 2026-11-13 fixed on the real GNOME session.
- Cleanup 09:50: uninstalled with pkexec, test file deleted.
