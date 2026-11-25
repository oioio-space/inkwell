# EX0b2 review, step 1 (blind Opus review, 2026-11-25; standard tier)

Verdict: accept with corrections, 4 major. Every finding applied at its root, in the plan and the gate (answers in WITNESSES.md):
1. availability read from the palette's field → computed once at opening against the target, cached while open (P-5, MP5; `evalWhen` counted);
2. a detached opener or a menu item → the menu's return target, the last focused surface (P-7, MP7; E1 runs menu → palette → « gras » → Enter);
3. the bound could be dodged by built-in search → criterion `ranker-lint` on rank.ts; MP8's witness query is 64 characters « a…ab »; E2 times 1 000 fixture commands (e2e build only, absent from production by `chunks`) in Chromium and WebKit within §15.4;
4. status outside the modal dialog → a live region inside it; the reason tied to its option by `aria-describedby` (P-5, P-6);
5. every listed command runs or says why: the validator already refuses `when` without `unavailable` (vector kept, tested in P-5); the census found `app.return` (condition on its menu item only) and the eight surface-only commands without a reason: both fixed in the plan;
6. French word boundaries (space ' ’ - / : ( )), NFKD, « … » and guillemets folded; class 3 skips label words, each used once (P-1, P-2);
7. a 50-row window around the active row, PageDown reaches every match (P-6, MP6);
8. built-in keywords are i18n keys read by `t()`, third-party ones literal in every language (plan, P-2);
9. « Aucune commande », status debounced 300 ms (P-6); 10. rows prevent `mousedown`; each dialog's own command focuses its field, the other replaces it (P-7);
12. `docs` also checks ux-principles rule 3 and spec §4.26 (`app.shortcuts`, AdwShortcutsDialog).
Integrator's decisions: typo tolerance → EX0b3 (PROGRESS line; from OBS1's « no result » count; never scattered letters); recents = palette runs only; the window gains an « Écrire » section of labelled editor and grid keys (P-9).
Rerun once: `docs`, `ranker-lint` (new), `chunks`, all red for the right reason; the other commands are unchanged.
