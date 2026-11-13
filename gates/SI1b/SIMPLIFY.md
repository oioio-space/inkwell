# /simplify — block SI1b (honest sync indicator, W33; supersedes SI1) (2026-11-09)

Scope: the whole diff of the block:
- `frontend/src/net/syncIndicator.ts`, `syncLabel.ts` and their tests (`syncIndicator.test.ts`, `.model.test.ts`);
- `WailsNetworkAdapter.ts` (heard heads per peer and document, the scope clear);
- `workspace.svelte.ts` (`attach`, `refreshSync`, `statusOf`) and `observe.ts` (`__inkwellLinks.shown`);
- the header, the status bar and the people popover;
- the i18n keys;
- `frontend/e2e/netsim.spec.ts` (clause SI1);
- `scripts/netsim/mutants.py` (kind `vitest`) and `mutants.toml` (12 SI1 mutants);
- the spec amendment SI1-A;
- `.env.reference`: `SI1_SEED`, the model test's replay seed, found by `lint:env`.

Review by the main session (reuse, simplification, efficiency, altitude), against the frozen gate `gates/SI1.toml`, the
spec, D-SI1..7 and the interface rules adopted the same day (`docs/plans/2026-11-09-ui-formal.md`).

## Applied

- **Altitude (decision of the main session):** the implementer's 20 s `UNANSWERED_MS` drop of unanswered documents is
  removed. It was a timer outside the spec and the model, and it could claim « Synchronisé » early. It is replaced by
  the spec amendment SI1-A, from a traced root cause: a first sync message for Bob's pre-join root, dropped by Alice
  outside her scope. Only documents of the current admitted scope count, and a scope change clears the heard heads.
  Pinned by the tests `SI1-A …` and the mutants SI1-ScopeKept and SI1-NoLocalRefresh.
- **Rule compliance:** the new `title=` on the status bar's people count is removed (R7: no `title=` tooltips;
  the header already shows the label). The status bar's `sending`/`slow` props, now unused, are gone too.
- **Simplification:** the people popover computed each person's sync line twice per row; it is now one `{@const}`.

## Declined

- **Reusing the per-peer states for the n of « Envoi en cours (n) » instead of `sendingCount(observed)`:** it would
  save one `peerSyncState` call per peer. But the mutant SI1-CountMessages targets `sendingCount`, so the shipped
  path must stay the proven one. The cost is a few microseconds per refresh.
- **Hooking handles through a Repo event:** automerge-repo 2.5.6 emits none for new handles. `refreshSync` hooks
  each handle it meets once (a `WeakSet`). A handle created later is hooked on its first sync send, which notifies.

## Root causes found by the gate (kept as evidence)

- **NI-2, seed 1, first run:** 3 headers said « Synchronisé » while 1-3 of the viewer's own tokens were missing at the
  other side. The indicator listened to a Repo `document` event that this automerge-repo never emits, so a local edit
  during a blackout never recomputed it. A debug probe showed the tokens were in Bob's Automerge document and absent
  from Alice's text. Fixed by the `heads-changed` hook; pinned by `SI1-A a local edit makes a synced peer sending`
  (through the real workspace wiring) and the mutant SI1-NoLocalRefresh.
- **`pair()` stuck at « Vérification… »:** see SI1-A above. No content leaves for an out-of-scope document
  (`shareAdmitted` limits announce and access; the dropped message held no changes).

## Evidence

- `rule-SI1-tests`: all nine named tests pass, plus the three SI1-A tests. The model-based test ran 200 runs, and 3
  more runs with random seeds.
- `rule-SI1-mutants`: 12 of 12 caught. `rule-SI1-wiring` passes. `fe:check`: 111 files, 1236 tests. After the
  cleanups: svelte-check 0 errors, Biome clean.

## Mutation

No `protocol/*` package is touched; gremlins does not apply. The rules are proven by the 12 code mutants and the 7
TLA+ mutants of SyncIndicator.tla.

## Gate run evidence: NI-SI1 (3 seeds) and the C5 replay

- **Clause SI1, scenarios 5 and 6, seeds 1-3:** pass on every run, 0 lies (290-292 and 94 header samples a run).
- **Scenario 5, seed 2: C5 failed once** (« reported loss follows the injected loss »). P3, the clean phase, measured
  0.7 % for a 0.5 % bound. SI1 touches no Go code and no link metric.
- **Replay, same tree, same seed, twice:** C5 passed both times, with P3 at 0.2 % and 0.3 %. The same seed passed and
  failed on the same commit, so C5's P3 bound is proven marginal. It is not caused by SI1.
- **Follow-up, outside this block:** the P3 samples fall from 1.8 % and 1.6 % to 0-0.7 %. This suggests that the
  clean phase still counts loss from the previous phase's 30 s window; it is a hypothesis to check. Filed for the
  netsim work (W32). The bound is not changed.

## SI1b: the display rule after the blocked walkthrough

- **Why:** the SI1 walkthrough (`gates/SI1/UX.md`, kept as history) blocked the merge. Two people typing made the
  header flicker about 60 times in 5 s, and it showed « Envoi en cours (0) ». D-SI5 is replaced by DS-1..DS-5 (spec
  amendment SI1-B) in a pure `displayStep`, proven by a fast-check property over random timed sequences (600 runs) and
  by the mutants SI1-NoMinShow and SI1-LiveCount. 14 of 14 mutants are caught.
- **Applied in review:** the implementer had cut the popover's sync line with an ellipsis and a `title=` tooltip. That
  is against R7 (no `title=`), and the ellipsis hid information. The person's sync state is now on its own line. The
  affected test now selects that line; its assertion is unchanged.
- **Second walkthrough** (`gates/SI1b/UX.md`): « Verdict: ship with filed follow-ups ». The flicker is down to 9-10
  label changes in 8 s of two-person typing; « (0) » is never shown.
- **Filed for the next block** (R1, the clickable sync state, where the labels are revisited):
  - UX-SI1b-1: sending and behind alternate once a second while both type;
  - UX-SI1-5: « Synchronisé » shown, withdrawn and shown again on a first join;
  - UX-SI1b-2: checking and sending share the same blue;
  - UX-SI1b-3: the popover and the header can disagree for about a second;
  - UX-SI1-7: the « ? » avatar.

## C5 A/B (2026-11-09): SI1 does not cause the C5 failures

- **Setup:** scenario 5, seeds 2 and 3, interleaved and two runs each. A is HEAD 132b7c8 without SI1, in a separate
  worktree; B is the tree with SI1.
- **Result:** A failed C5 once in 4 runs (P1 at 0.5 %, under its 1 % floor: the other direction). B passed 4 of 4.
- **Reading:** C5 is marginal in both directions, independently of SI1. The two B failures of the gate run came while
  the large-file measurements loaded the machine. Follow-up (W32, outside this block): C5's loss estimate rests on
  too few packets per 30 s window. It needs a statistically sound estimator; the bounds are not changed.

## Re-validation on main after C5S, RC1 and LF0a3 (2026-11-13)

The block was re-applied on main (cherry-pick of 60fa7a2; conflicts in `docs/objectif-en-cours.md`,
`gates/guard-overrides.json`, `scripts/netsim/mutants.toml`, `.env.reference` and `docs/exigences.md` resolved by
keeping both sides, main's text first). Diff re-reviewed for reuse and simplification: nothing new to apply. One
cleanup: `syncedSince` in `syncIndicator.test.ts` was written but never read (Biome `noUnusedVariables`); removed with
its two assignments, the DS-1..DS-5 property test is unchanged. Criteria re-run one by one with their frozen
commands: model-SyncIndicator, rule-SI1-tests, rule-SI1-wiring, rule-SI1-mutants (14 of 14 caught), NI-SI1
(scenarios 5 and 6, seeds 1-3, clause SI1 pass), base-guard.
