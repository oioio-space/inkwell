# SI1b UX walkthrough: the honest sync indicator after spec amendment SI1-B (W33)

Date 2026-11-13. Tester: ux-tester agent, first-use method (screen first, plan and code afterwards).
Build: `bin/inkwell-server` of 02:30 (newer than every non-test source of the diff), two instances (Alice light,
Bob dark), Chromium 1280x800, 360 px, fresh identity folders. Header label recorded with a MutationObserver
(page clock) on both screens. Captures in `gates/SI1b/ux/`: `*-header`, `*-statusbar` are crops, `*-full.png` whole
windows, `people-*` and `bob-*` show the people popover. Personas: a writer who wants to know whether her text
reached her co-author (Alice, Bob), a hurried user (Paul), plus Karim for the bad link.

Verdict: ship with filed follow-ups

No defect of severity 3 or 4. Five defects of severity 2 or 1 are listed below. Two have a failing e2e test in
`frontend/e2e/ux/si1b.spec.ts` (confirmed red on this build). User session (SEQ): done 2026-11-13, see « User session » below.

## SI1 blockers, re-checked
- Flicker: gone. Two people typing at about 10 keys/s for 8 s: 8 label changes per screen (60 in 5 s before),
  one per second at most (`typing-alice-light-full.png`, `typing-bob-dark-full.png`).
- « Envoi en cours (0) »: never seen, in about 40 s of two-person typing over four runs, nor in any freeze.
- « Synchronisé » is retracted at once when it stops being true and returns 1.0 s after the last acknowledgement
  (measured 991 ms after the last key on both screens, 360 px run).

## Scenarios
- S-A alone: « Seul », grey dot, header and status bar agree (`alone-light-full.png`). Clean.
- S-B Bob joins by invitation: Bob reads « Connexion… », « Vérification… » (blue pulsing dot), then « Synchronisé »
  at 1.7 s; Alice « Vérification… » then « Synchronisé » at 1.1 s (`joining-bob-dark-full.png`,
  `synced-bob-dark-full.png`). Clean.
- S-C both type: UX-SI1b-1. Dots: blue sending, amber behind, green synced in both places, light and dark
  (`behind-light-*`, `behind-dark-*`, `synced-*`).
- S-D Bob unreachable (process frozen) while Alice types: « Envoi en cours (1) » at 90 ms, held for 30 s, then
  « Seul » (`frozen-sending-light-*`). Popover: Bob « En ligne · main.typ » with « Envoi en cours » on its own
  line, then « Hors ligne » once the link is dropped (`people-sending-light-full.png`, `people-sending-dark-full.png`).
  UX-SI1-3 still applies at that moment.
- S-D2 the mirror: Alice frozen, Bob types and sees « Envoi en cours (1) » for 40 s, popover shows Alice « En
  ligne » + « Envoi en cours » (`bob-frozen-alice-sending-dark-full.png`). Bob's text is delivered after the
  resume: « Vérification… » then « Synchronisé » within 3 s, Alice holds Bob's text (`back-online-*`).
- S-E back online: a frozen process that resumes returns to « Synchronisé » in 1 to 3 s without any action.
- S-F 360 px: UX-SI1b-2. Popover is usable at 360 px (`w360-people-dark-full.png`), state line included.
- Wording against `docs/glossary.md` (« synchronisé », « hors ligne » present, no « pair », no « heads ») and
  `docs/ux-principles.md`: plain French, one vocabulary in header, status bar and popover. Clean.

## Defects
### UX-SI1b-1, severity 2: the header blinks once a second for as long as two people type (Nielsen 1, 8; a11y)
- Screenshot: `typing-alice-light-header.png`, `typing-bob-dark-header.png`. Repro: both type 8 s in one file,
  watch the header. Seen on both screens: « Envoi en cours (1) » / « Pas encore à jour » alternating, each shown
  exactly 1.0 s (A: 75, 1151, 2156, 3196, 4223 ms ...). It is what DS-3 prescribes, but a metronome while typing is
  noise, and `role="status"` is a live region, so a screen reader announces every change (Léa).
- Expected: one steady wording while typing (for example the worst state held until the typing stops), at most
  synced -> working -> synced. Test: `frontend/e2e/ux/si1b.spec.ts` UX-SI1b-1 (red: 8 changes, limit 2).

### UX-SI1b-2, severity 2: the status wording collides with the title at 700 px and below (Nielsen 8; S13)
- Screenshot: `w600-sending-light-full.png`, `w360-fresh-light-full.png`, `w360-people-dark-full.png`. Repro:
  shrink the window to 700, 600, 500, 360 px while « Envoi en cours (1) » shows. The label (x 310 to 451) is cut
  by the avatars and overlaps « Sans titre » from 700 px; at 360 px it starts at x 310 and leaves the screen. The
  old « Synchronisé » (x 310 to 386) already collided from 500 px, so SI1b widens the problem, the root is the
  header layout of S13 (the sidebar stays open and pushes the label right). The hurried user on a small window
  reads « Env » or nothing. Test: `si1b.spec.ts` UX-SI1b-2 (red at 700 px).
- Expected: the label stays readable and separate from the title, by hiding the title or truncating it first.

### UX-SI1-3, severity 2 (open, owner decision, deferred by SI1-B): « Seul » while the co-author is offline
- Screenshot: `frozen-sending-dark-full.png` (header « Seul », Alice's last sentence unsent, popover « Hors ligne »).
- The writer asking « did my text reach her? » reads « Seul » as « she left », not as « not delivered ». True, but
  silent about the unsent text. Expected: decide before release (R1, the sync-state popover, is the planned home).

### UX-SI1b-3, severity 1: popover line « Envoi en cours » under a person is ambiguous (Nielsen 2, 6)
- Screenshot: `bob-frozen-alice-sending-dark-full.png`. Under « Alice », « Envoi en cours » can mean she is sending
  or that I am sending to her, next to a green « En ligne » dot. Expected: wording that says whose side, for
  example « Reçoit vos modifications » (to settle with the glossary).

### UX-SI1b-4, severity 1: « Lien lent » not reachable here (follow-up, not a defect)
- A frozen peer (SIGSTOP) and a 6 MB paste never raised « Lien lent » in 14 s (the outbox is not held). The wording
  and the 2 s rule are covered by the unit test; the visual check needs `mise run e2e:netsim` (network namespaces).
  File: capture « Lien lent » light and dark with the netsim before release.

## Not walked
- « Lien lent » on screen (see UX-SI1b-4); 200 % zoom, forced colours, screen reader; keyboard focus order of the
  header status (it is not focusable, by design).

## Checks run
- `mise run fe:check`: exit 0, Biome clean, Vitest 113 files, 1266 tests passed, 1 todo. The log carries
  `ECONNREFUSED :3000` stack traces from the happy-dom teardown; they do not fail the run.
- `frontend/e2e/netsim.spec.ts`: the two `useTemplate` infos were on SI1b lines (clause SI1); fixed with
  `biome check --write --unsafe`.

## User session (2026-11-13, real app, two instances from `mise run app:duo` with isolated documents)

Verdict: **accepted by the user** (« Je valide ») for SI1b's scope: the sync state shown was true at every step. Per-step easiness scores were not given; verdicts recorded as said. Findings outside SI1b's scope are filed in `docs/exigences.md` (W51 to W58 and the rows added on 2026-11-13) and placed in their blocks (HB1a/HB1b, invite W58, share dialog, editor).

- Step 1 alone: OK — both bars « Seul », grey dot.
- Step 2 invite (1st try): BLOCKED — « Impossible d'inviter : RuntimeError: invite: invite: trust: signer is not an admin » (cause: app:duo shared real ~/Documents/Inkwell; W58).
- Step 2 invite (isolated docs): works, verdict « bof » (no score given). Header green « Synchronisé », status bar « 2 personnes ».
  Findings: (a) a « Coller » button for the invitation; (b) invitation should be a few words like Magic Wormhole, not a long text; (c) the random default name should be friendlier; (d) join dialog layout: « Rejoindre » alone at the bottom, « pas très design ».
- W54 precision: the white page itself must match the writing area size.
- Step 3 typing together: text converges both ways (OK). Findings: (e) the remote caret's name label hides the text under it — it should fade (become see-through) so the text behind and the pointer target stay visible; (f) text another person added is not highlighted (only a coloured gutter mark beside it) — highlight the added zone in that person's colour; (g) soft wrap: the user must be able to choose between wrapping long lines and a single line that scrolls to follow the typing (« l'utilisateur devrait avoir le choix »). Sync dot not commented.
- Step 4 cut-off: OK — during the cut the header went to « Seul »; after reopening, the words typed meanwhile arrived and both headers returned to « Synchronisé ». « Envoi en cours (1) » not mentioned by the user. Incident: the first relaunch of the closed instance (Alice) exited silently (cause unknown, no log); the second, logged, stayed up.
- Step 5 narrow window: header bar buttons overlap (known, SI1c/HB1); the « ajouté / modifié / supprimé » legend row wraps, « pas très joli »; the writing area disappears at narrow width; collapsing then reopening the files sidebar swaps the preview out for the writing area, and after widening again the preview stays gone until « Aperçu » is clicked (view state lost on resize).
