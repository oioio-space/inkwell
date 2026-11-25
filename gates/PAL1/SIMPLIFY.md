# PAL1 /simplify review

Reviewer: Claude Opus, 2026-11-25, over the whole diff of the block against `main` 4147534 (EX0b2): `frontend/src/extensions/commands.ts`,
`frontend/src/ui/command/Palette.svelte`, `frontend/src/ui/command/palette.test.ts`, `frontend/e2e/helpers.ts`,
`frontend/e2e/ex0.spec.ts`, `frontend/e2e/ex0b2.spec.ts`, `docs/architecture/spec.md`. Checklists: reuse,
simplification, efficiency, altitude. go-style-guide and helper-ergonomics do not apply (no Go).

## Symptom

`main`'s ci-e2e went red after EX0b2 (run 45601671496, commit 4147534, WebKit). EX0b2-E2 PERF-PALETTE failed on both
attempts. Its slowest warm opens were 67 and 75 ms against 50 ms (opens 35-75 ms, one-letter queries 6-16 ms). The
gate had measured 33-35 ms locally.

## Root cause, measured

Each palette opening reads availability for every command (P-5). `runnable(id, target)` called `availability()`.
Both rebuilt the context per command (`context()` = `{ ...live, focus, field }`, a copy of the Svelte `$state` proxy
through its `ownKeys`/`get` traps) and walked the DOM up to the surface twice. That context is the same for the whole
opening, so the cost was O(commands × live keys) where O(commands + keys) suffices.
- WebKit phase marks (instrumentation, removed): the availability loop took 5-9 ms of an 11-17 ms warm open locally.
- Chromium CPU profile at 4× throttling, 11 opens: `context()` and the proxy traps took about 160 of the palette
  script's 248 ms (65 %).
- CI: queries, which also rank 1 000 commands and draw 50 rows, took 10-15 ms against 50-75 ms for opens, so the
  extra cost is opening-only work. Playwright tracing on or off changes nothing (measured).
- Vitest: 1 055 contexts built for one opening over the e2e fixture (old behaviour), 1 after the fix.

## Applied

- Efficiency, at the root: `runnableOn(target)` reads the target's surface and the context once and returns the
  check per command. The palette calls it once per opening. `runnable()` had no other caller, so it was removed, not
  kept as a wrapper. `availability()` is unchanged: buttons and menu items read it in a `$derived`, which must track
  `live`.
- Simplification: the old `runnable` computed its reason through two branches (`when` false, or no handler) that both
  ended in « `unavailable` read, else the general reason ». It is now one expression.
- Regression test `PAL1 one opening over 1 000 commands builds the context once, not once per command`
  (`palette.test.ts`) counts `commandStats.contexts`, the counter `commandStats.lookups` already lived beside. It
  fails on the old behaviour (`expected 1055 to be 1`) and passes after the fix.
- Measured after the fix: WebKit warm opens 7-17 ms (was 11-17 ms), availability loop 0-1 ms; Chromium at 4×
  throttling 18-24 ms (was 33-44 ms).
- Convention, reuse: the decision of b240aeb (2026-11-24) was coded inline in `ex0.spec.ts` only. That decision says a
  wall-clock e2e budget is judged where the run holds the machine (`INKWELL_HEAVY_SLOT=all`) and is reported as
  `perf-unjudged` elsewhere. EX0b2-E2 did not follow it. It is now one helper, `judgeBudget` (`e2e/helpers.ts`), used
  by EX0-E2 and EX0b2-E2. Nothing is weakened: gates/EX0b2.toml's `e2e` and `e2e-webkit` criteria run under
  `heavy.sh --exclusive`, where the three budgets still fail the test. `docs/architecture/spec.md` §12.3 said budgets
  are measured on CI machines, which contradicted that decision. It now states it and cites b240aeb.

## Set aside

- Per-row work at opening (`shortcutOf`, `labelOf` and the category for 50 rows) plus the empty-query rank: about
  5 ms in WebKit, proportional to what is drawn. Kept.
- A slowed reproduction under a podman CPU quota (`--cpus` 0.5 and 1) is invalid as a measure: every stall is a
  multiple of the 100 ms CFS period. Not used as evidence.
- Proof on GitHub's runner needs a candidate run (`scripts/candidate.sh`, a push), which is the integrating session's
  job.

## Residuals

- UX-S11-2 (flaky in the same run, passed on retry) is new: it is absent from the ~37 earlier ci-e2e runs that were
  searched. It fails in `open()` (`helpers.ts:64`) right after `page.goto`, before any key. The trace shows the
  header and preview drawn but the source pane empty: the document never reached the page, and the store stream's
  poll stayed silent for 5.8 s at load. EX0b2's key listener changes run on keydown and focusin only. Suspect: RT1's
  turn-taking, where a new page waits for the previous page's turn (`storestream.go`). Unproven → RT2.
- PRV-105 (flaky, line 176, « Ligne 8, colonne 29 ») was already flaky before EX0b2: same failure in run 45523876285
  (OBS1 freeze) → E2E1.
- EX0b1-E1 failed once in three under `--repeat-each 3` at `ex0b1.spec.ts:112` (« Changement 1 sur 1 »). The
  instances are shared and their history grows across repeats, while the test takes the `.first()` « Bob » moment. It
  does not open the palette → E2E1.
