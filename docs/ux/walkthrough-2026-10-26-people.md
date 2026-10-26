# UX walkthrough S7 — people (2026-10-26)

Scope: B3b people management (dialog « Personnes », bouton « Voir les personnes », bannière d'accès,
éditeur en lecture seule). Three real instances (Alice admin et créatrice, Paul écrivain, Léa
lectrice), Chromium, 1100x640, light and dark, 550x320 (200 %), 360 px, forced-colors, keyboard.
Interface discovered from the screen first; the code and `people.spec.ts` read afterwards.
Screenshots: `docs/ux/screens/s7-*.png`.

Already accepted, not re-reported: machine name for a person never seen present; « Hors ligne »
for a person not directly connected (so Léa sees « 2 personnes » when three are in the project);
no free-text reason with a disconnection.

## Blocking (severity 3 and 4)

- **UX-S7-4 (3)**, fixed, see below. No severity 4.

## Walkthrough

| Step | Four questions | Verdict |
|------|----------------|---------|
| Alice finds the people list: the avatar stack is a button, hover highlights it, « Voir les personnes » | notice yes, associate yes | OK |
| Dialog: one row per person, colour + initial + name, « En ligne », role, actions | clear, light and dark | OK |
| Change Paul's role (select « Rôle de Paul ») | applied at once, toast, Paul's banner within 2 s | OK, toast wording was terse (UX-S7-6) |
| Disconnect Paul | inline question that says what happens and that it is reversible; red confirm; toast; row turns « Déconnecté par … » with « Laisser revenir » | OK, but « par Un admin » (UX-S7-2) |
| Paul, disconnected | header « Déconnecté » (not « Seul »), banner says who, when, how to come back; editor read-only; his own dialog has no control and says why | good; toolbar still offered (UX-S7-1) |
| Alice lets him back | toast, Paul's banner gone in under 1 s, typing works, Alice and Léa get it | OK, name regressed to the machine name (UX-S7-3) |
| Léa, reader | banner « Vous lisez ce document… », cannot type, dialog without any control | OK, but « Inviter » offered then refused with a technical error (UX-S7-4) |
| Keyboard | Enter on the stack opens the dialog, focus on « Fermer », Tab through every control, Escape closes and returns focus to the stack, focus ring visible | OK |
| 200 % zoom (550x320), 360 px | dialog scrolls inside itself, no horizontal scroll, buttons wrap under the name | OK |
| Forced colours | all text and controls visible; row separators and status dot disappear | acceptable (UX-S7-9, severity 1) |

## Defects

### UX-S7-1 (severity 2) — a reader or a disconnected person is offered buttons that silently do nothing
- Screenshot: `s7-07-paul-reader-typed.png` (before), `s7-21-after-fix-lea-reader-toolbar.png` (after).
- Repro: Alice sets Paul to « Lecture ». On Paul's screen click « Gras », or « Insérer » then « Saut de page ».
- Seen: the buttons look usable (even show their tooltip), nothing happens, no message.
- Expected: the formatting bar is visibly off (the banner above already says why).
- Fix: `FormatBar.svelte` is `inert` and dimmed while `workspace.access` is set. Test `UX-S7-1`.

### UX-S7-2 (severity 2) — « Déconnecté par Un admin » on the admin's own screen
- Screenshot: `s7-11-alice-disconnected-300ms.png`, after: `s7-21-after-fix-alice-par-vous.png`.
- Repro: Alice disconnects Paul; look at Paul's row.
- Seen: « Déconnecté par Un admin à 18:19 » (and the capital U mid-sentence), although Alice just did it.
- Expected: « Déconnecté par vous à 18:19 ».
- Fix: `PeopleDialog.stateOf` recognises its own device. Test `UX-S7-2`.

### UX-S7-3 (severity 2) — a person let back in is called by the name of their machine
- Screenshot: `s7-13-alice-let-back.png`: toast « fedora.home peut de nouveau modifier », row « fedora.home ».
- Repro: Paul is disconnected for a while and someone else's cursor moves; Alice clicks « Laisser revenir » on « Paul ». Back to « Paul » only when he types.
- Expected: the name Alice has already seen, in the row and in the toast.
- Fix: `workspace.hearNames` remembers every name heard (session), `knownName` is used by the dialog and the banner. Vitest `names (UX-S7-3)`; e2e `UX-S7-3` guards the flow but does not reproduce the loss by itself (needs a third person moving), so the unit test is the real proof.

### UX-S7-4 (severity 3) — a writer or reader can press « Inviter » and gets a technical English error
- Screenshot: `s7-16-lea-invite-click.png`, after: `s7-21-after-fix-paul-share.png`.
- Repro: as Léa (reader) or Paul (writer or disconnected): « Partager », « Inviter ».
- Seen: « Impossible d'inviter : RuntimeError: invite: invite: trust: signer is not an admin ». The person is misled: nothing in the interface said that only an admin invites; the message is jargon and English.
- Expected: the button is off and the line says « Seul un admin peut inviter quelqu'un. »
- Fix: `workspace.mayInvite` (admin, not disconnected; true while Go has not said), `ShareDialog.svelte` disables the button and changes the help text, key `share.host.adminOnly` (fr, en). Test `UX-S7-4`.

### UX-S7-5 (severity 1) — « Déconnecter » / « Passer la main » do not say for whom to a screen reader
- Fixed: `aria-label` « Déconnecter Paul », « Laisser revenir Paul », « Passer la main à Paul ».

### UX-S7-6 (severity 1) — toast « Paul : Lecture » is terse
- Fixed: « Paul a maintenant le rôle Lecture » (`people.done.role`, fr and en).

### UX-S7-7 (severity 1) — « Passer la main » alone is ambiguous
- Left: the confirmation explains it fully (« Léa devient admin et vous passez en écriture… »), so nobody does it by mistake. A tooltip could be added later.

### UX-S7-8 (severity 1) — Paul, disconnected, sees « Seul » in the status bar and Alice « Hors ligne »
- Left: true from his device (he is cut off), the header says « Déconnecté ». Consistent with the accepted « Hors ligne » rule.

### UX-S7-9 (severity 1) — forced colours: no row separators or status dot
- Screenshot: `s7-19-forced-colors.png`. Left (text carries the state).

### Outside S7, noticed
- The « Insérer » menu is taller than a 640 px window and its last items sit under the status bar (S13).
- Share dialog keeps the invitation text a person pasted earlier in « Collez l'invitation… » (cosmetic).
- `people.spec.ts` reference `people-suspended-dark.png` contained the wall-clock time of the disconnection, so the test failed at any other minute: the time is now pinned in the test and the reference regenerated. `people-admin-light.png` depends on Alice's project having no other member (it fails after `join.spec.ts` in the same run, which adds Carole): run order, not UI.

## Status
- Severity 3: UX-S7-4 closed (test red, fix, green, re-walked).
- Severity 2: UX-S7-1, -2, -3 closed. Severity 1: -5, -6 closed; -7, -8, -9 left.
- Vitest 894 passed (one unrelated fast-check CSV case failed once and passed on re-run), biome and svelte-check clean, e2e `ux/s7`, `people`, `collab`, `join`, `format`, `shell` green.
