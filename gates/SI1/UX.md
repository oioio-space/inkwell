# SI1 UX walkthrough: the honest sync indicator (W33)

Date 2026-11-09. Build: `bin/inkwell-server` (server mode, newer than the SI1 sources), two instances
(Alice and Bob), Chromium 1280x800, light and dark. The screen was read first, then the plan (D-SI1..7).
Captures are in `gates/SI1/ux/`: header, status bar and people popover are cropped (`*-header`,
`*-statusbar`, `people-*`); `*-full.png` are whole windows.

## Verdict: BLOCK (merge)
Two severity-3 defects (UX-SI1-1, UX-SI1-2) break the spec's own no-flicker rule and show a nonsense label.
One more severity-3 candidate (UX-SI1-3) needs a decision. Four follow-ups at severity 2 or lower.

## Scenarios
- S-A, alone: new project. Header and status bar read "Seul" (grey dot). Plain and consistent.
  `alone-light-full.png`. Clean.
- S-B, Alice shares and types, Bob joins: Bob reads "Vérification…" at once, then "Synchronisé" 1 s later;
  Alice reads "Synchronisé". `verification-joiner-light-full.png`, `joined-synced-light-full.png`. Clean,
  except the "?" avatar (UX-SI1-7).
- S-C, both type at once (about 10 keys per second each, 5 s), label sampled every 40 ms: UX-SI1-1, -2.
- S-D, Bob's instance is killed: UX-SI1-3, -4. Popover shows "Hors ligne" (`people-offline-*`).
- S-D2, Bob's instance frozen (a link that stops answering) while Alice types: header "Envoi en cours (1)"
  for 22 s, popover line "Envoi en cours" (`sending-*`, `people-sending-*`). See Not walked for "Lien lent".
- S-E, Bob comes back: restarting his instance does not reunite the two (90 s, both stay "Seul",
  `bob-restarted-seul-light-full.png`); a new invitation does. UX-SI1-5.

## Defects

### UX-SI1-1, severity 3, the label flickers while typing (Nielsen 1, 8; breaks D-SI5)
- Screenshot: `typing-envoi-0-light-full.png`.
- Repro: Alice and Bob type in the same file; watch the header for 5 s.
- Seen on both screens: about 60 label changes in 5 s, one every 40 to 80 ms, between "Envoi en cours (1)",
  "Envoi en cours (0)" and "Pas encore à jour".
- Expected (D-SI5): a steady "Envoi en cours" until 1 s after the last acknowledgement, with no alternation
  between keystrokes. ("Synchronisé" 1.4 s after the last key is right.)

### UX-SI1-2, severity 3, "Envoi en cours (0)" (Nielsen 2, 1; breaks D-SI1, n >= 1)
- Screenshot: `typing-envoi-0-light-full.png` (header reads "Envoi en cours (0)").
- Repro: same as UX-SI1-1; the label shows (0) about 30 times in 5 s.
- Why: "sending to 0 people" contradicts itself; a colleague reads it as a bug.
- Expected: n is never 0 under "Envoi en cours"; show the state that is true instead. Probably one cause
  with UX-SI1-1 (the label follows the instant state, not the held one): fix both together.

### UX-SI1-3, severity 3 if the spec stays as is, else 2: "Seul" in a shared project (Nielsen 1, 2)
- Screenshots: `offline-light-header.png`, `offline-dark-header.png`.
- Repro: share, Bob leaves, Alice keeps typing. Header and status bar read "Seul", also while Alice's new
  text has not reached Bob. The popover says "Hors ligne" for Bob (good, D-SI3) but is two clicks away and
  never mentions the unsent text.
- Why: "Seul" means nobody else is here; Alice believes her work is simply hers. She is misled.
- Expected: keep D-SI3 (no claim about Bob's copy) but do not say "Seul" for a shared project, for example
  "Personne en ligne" or "Hors ligne : Bob". Needs an owner decision (a spec gap, not a code slip).

### UX-SI1-4, severity 2, going offline is noticed 35 s late (Nielsen 1)
- Repro: kill Bob's instance. Alice's header and status bar keep "Synchronisé" and a green "2 personnes"
  for about 35 s, then jump to "Seul".
- Expected: a closed window disappears at once (a goodbye); a dead link may take longer, but the label must
  not claim "Synchronisé" for the whole delay.

### UX-SI1-5, severity 2, "Synchronisé" shown, then retracted (Nielsen 1; D-SI5 spirit)
- Repro: Bob rejoins with a new invitation. Alice reads Synchronisé (1.1 s), Pas encore à jour (2.1 s),
  Synchronisé (3.1 s). Bob reads Pas encore à jour, Synchronisé, Envoi en cours (1), Synchronisé.
- Expected: no "Synchronisé" before the exchange that follows a (re)join is complete.

### UX-SI1-6, severity 2, header and status bar tell different stories (Nielsen 4)
- Repro: in any non-synced state the header reads "Envoi en cours (1)" or "Vérification…" while the
  status bar keeps "2 personnes" with a grey dot (`sending-light-statusbar.png` against the header).
- Expected: the same wording in both places, or a status bar that is plainly a head count.

### UX-SI1-7, severity 1: "?" avatar and a lost name (Nielsen 2)
- A joiner sees a "?" avatar for about 1 s. After Bob's restart his popover lists Alice as "Personne 12D3".
  Older than SI1, but in the popover this block changed.

### UX-SI1-8, severity 1: popover line wraps (Nielsen 8)
- "En ligne · main.typ · Envoi en cours" wraps under the role control at 1280 px (`people-sending-dark.png`).
  Shorten it, or drop the file name.

## What works
- Plain French, no jargon: "Vérification…", "Envoi en cours", "Pas encore à jour", "Synchronisé", "Hors ligne".
  (The older popover button "Passer la main" is unclear; not SI1.)
- Green dot only when synced, grey otherwise. Light and dark follow Adwaita and match the neighbouring
  screens (`synced-*`, `sending-*`, `offline-*`).
- The popover gives each person's own state and "Hors ligne" for an offline person (D-SI2, D-SI3).
- "Synchronisé" never showed while a change was pending on the sender's side; it came 1 s after the last ack.
- After a freeze the popover agrees with the header ("Envoi en cours").

## Not walked
- "Lien lent": a frozen process did not fill the outbox, so no suffix appeared in 22 s. Needs
  `mise run e2e:netsim`. Open question: a change unanswered for more than 2 s showed no warning in my run.
- "Pas encore à jour" and "Vérification…" in dark; "Seul" in dark (read as text or in light only).
- 200 % zoom, forced colours and a screen reader were not checked. Keyboard: the popover opens from the
  avatars, the focus ring is visible and Escape closes it. The status-bar text is not a control (a click does
  nothing; a new user tries it).

## Follow-ups
- Block the merge until UX-SI1-1 and UX-SI1-2 pass a failing-first e2e in `frontend/e2e/ux/`, then are re-walked.
- Owner decision before release: UX-SI1-3.
- In the wave: UX-SI1-4, -5, -6. When touched: UX-SI1-7, -8.
