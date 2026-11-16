# /simplify - block CL1 (the history claim arms on local edits only) (2026-11-16)

Scope: the whole diff of the block against `ceedca7`: `frontend/src/ui/history/history.svelte.ts` (rules C-1..C-6),
`frontend/src/workspace/observe.ts` (`observeClaims`, `window.__inkwellClaims`), `frontend/src/ui/history/claim.test.ts`
and scenario 8 of `frontend/e2e/netsim.spec.ts` (both written at step 1, read, not changed at step 2), the registry row
W32, one line of `PROGRESS.md`, rows of `docs/autonomous-decisions.md`, the plan's paragraph « At the opening ». Review by the implementing session
(reuse, simplification, efficiency, altitude) against the frozen gate `gates/CL1.toml` and the plan. It is the
implementer's own pass: the review that counts for the merge is the integrator's.

## Two corrections decided by the integrator (2026-11-16), applied

- **The test `CL1 mutant C-1 a flag around change is caught` could not pass as written:** its first oracle (`c1`)
  wrote the local edit through `update`, which that mutant cannot see, and its second (`c1ways`) required the same
  edit to fail. `c1` now takes the writing call; the mutant test passes `editByChange` to it (what the mutant handles
  rightly) and `c1ways` still fails through `update` and `changeAt`. Title unchanged.
- **U12 wins over the first reading of C-2 (the former residual R-4 is now a rule):** at the opening a claim is owed
  when the document holds a change of the current actor that no claim names (plan, « At the opening »). The creator
  of a document is named a pause after it opens it, without an edit. The test harness made every test's first device
  the creator, so the oracles of C-1, C-2, C-3, C-5 and C-6 now run on devices that received the document; a new test
  `CL1 C-5 the device that created a document is named at its opening` and two new mutants (`the creator not named at
  the opening`, `C-2 owing a claim at every opening`) hold the rule from both sides; C-4's model has its first device
  create the document, as in scenario 8, with the same expectations. 22 tests, all green; no assertion loosened.

## Findings applied

- **Reuse:** the four lines that record the device's actor (`myActor`, `rememberActor`, `myActors`, reset of
  `#claimed`) were needed at the opening and at an actor change (C-6): one method, `#actorIs`, called from both.
- **Reuse:** the change handler is typed with automerge-repo's own `DocHandleChangePayload<HDoc>` instead of a local
  `{ patchInfo: { source: string } }` shape; `observeClaims` uses the same type and the existing `parseAction`.
- **Simplification:** `#settle` kept a `this.#claimTimer = undefined` nobody read: removed. `#owedFrom` read its map
  three times (`has`, `get`, `delete`): two.
- **Correctness found by the pass:** `#settle("hidden")` cleared the pause and did not arm it again when the claim
  could not be written. A window hidden and shown again while the rights are unknown would have stopped retrying,
  against C-5. Only `detach` stops the retry now (its timer is cleared with the rest of the document's state).
- **Altitude:** who wrote a change is read in one private method (`#wroteHere`) of the one module that owns the
  claim; no writer (editor, merge, replace bar, headings rule) knows about it. The observation hook lives in
  `workspace/observe.ts` with the other instrumented-build hooks and computes « mine » from the event itself, so the
  history module only registers it (one line) and hands it nothing.
- **Efficiency, measured:** `#wroteHere` adds `A.getHeads(before)` and `A.getChangesMetaSince` to every `change`
  event, typing included. 7.3 to 9.1 microseconds per event (7 rounds of 2 000 single-change events on a document of
  20 000 changes, Automerge 3.5.0, node 24; script kept in the session scratchpad, not in the tree). Nothing to gain
  by caching.

## Shape against the plan

The six planned lines are written as planned and each mutant is caught with its planned text. `#owedFrom(result)`
reads the first analysis of an opening as the plan says (a session of this actor, no claim of it); the page-life map
of the first version of this step is gone.

## Skipped, with reasons

- **One pass over the event's changes for the rule and for the observation hook:** the hook would then be fed by the
  history module on every build. It runs in instrumented builds only; the second read costs the 8 microseconds above.
- **Stopping the retry while the device only reads:** a demoted or disconnected device is `readOnly`, and both can
  end. The retry is one timer of this page every 2 s, alive only while a claim is owed and the document is open.
- **A claim for the actor a clone replaces:** when the editor's recovery clones the document within the pause, the
  old actor's claim is still owed and can no longer be written (a claim is read from its actor's own change, and the
  device now writes under the new actor). The reading at the next event or at the next opening does not cover it:
  both read the current actor, and writing a claim for the old one would need a change forged under a past actor.
  Listed as R-3.
- **Refactoring `history.svelte.ts` beyond the claim** (the 650-line state class): outside the block.

## Observed on the real stack (netsim criterion, one strict run per seed, `heavy.sh --exclusive`)

Milliseconds from the quiescence baseline at which each device wrote its claim (negative: before it), and C5 per
ordered pair (bound 2):

| Tree | Seed | Device 0 | Device 1 | Device 2 | C5 | Criterion |
|---|---|---|---|---|---|---|
| hook only, before the fix | 20771 | -2149 | -119 | -120 | 0 1 0 0 0 0 | |
| hook only, before the fix | 20772 | -1889 | -1865 | **+175** | 1 1 1 1 1 1 | exit 1: `late ['device 2 at +175 ms']` |
| fixed, first version of step 2 | 20771 | -2111 | -2427 | -2244 | 0 0 0 0 0 0 | |
| fixed, first version of step 2 | 20772 | -1878 | -2438 | -3642 | 0 0 0 0 0 0 | exit 0 |
| final tree (opening rule, all edits) | 20771 | -2126 | -2371 | -35655 | 0 0 0 0 0 0 | |
| final tree (opening rule, all edits) | 20772 | -1886 | -2503 | -36834 | 0 0 0 0 0 0 | exit 0 |

Before the fix the cause is seen for the first time: a device's claim lands 2 s after another device's claim (-119
and -120 ms follow device 0's at -2149 ms), and one lands inside the window. After it every claim is 1.9 s or more
before the baseline and no sync message is counted in the window. On the final tree device 2 (Carol, who created the
shared document) is named a pause after its opening, 36 s before the baseline, and writes nothing more after typing:
the opening rule on the real stack.

**The observation is scoped to the document the page follows now (decision of this step, to confirm).** With the
opening rule a page claims in more than one document: a first run of the criterion on the final tree exited 1 with
`devices [0, 0, 1, 1, 1, 2], late []`, C1 to C5 passing. A diagnostic build (document, reason and visibility in each
record; reverted, sources restored from a backup) showed the three extra claims were all in the pages' own untitled
documents, created at start: Alice's written when she left it for the shared one (`detach`), Bob's at the pause and
again when his name became « Bob ». None is in the measured document and none is late. `window.__inkwellClaims()`
now returns the claims of the followed document only; the frozen criterion (one claim per device) then reads what it
means. A claim in another document inside the window would not show in this list; clause C5 counts its messages.

## Residuals

- **R-1 (plan):** the defect does not show on this machine (ten runs of ten passed on the unchanged tree); the two
  local runs are non-regression and observation. Acceptance is `gates/CL1/GITHUB.md`: open until ten measured GitHub
  runs over three seeds or more.
- **R-2 (plan):** a claim owed, the device unable to write, and the page ends: that actor stays « Quelqu'un ».
- **R-3 (new):** an actor replaced by a clone while its claim is owed is never claimed (see « Skipped »).
- **R-4: closed**, it is the rule « At the opening » of the plan.
- **For the walkthrough (U12):** two instances, A creates a document and only B types: B's history must name A on
  the creation entry within a few seconds. No e2e asserts names across devices.

## Guard ids raised by `mise run pregate` (exit 1 on these two only; `gates/guard-overrides.json` not edited)

Both are on `frontend/src/ui/history/history.svelte.ts` (fingerprints of the final tree); proposed justifications for
the integrator's entries:

- `27f61ff641459ec74b0fc8db0fcd9f4a8b82374bfa64fea17f490f51068ea321` (« timeout increased:
  `clearTimeout(this.#claimTimer);` »): no delay changed. The call that clears the claim timer moved from the change
  handler into `#armClaim` and `#settle`; `CLAIM_PAUSE_MS` is still 2 000 ms, pinned by the test and the mutant of C-3.
- `2b2bae409491f8b8c6f28a25c95ffb85e58f7afc0b49fe60a42066ed04115d03` (« timeout expression removed or rewritten:
  `this.#claimTimer = setTimeout(() => this.#claim(true), CLAIM_PAUSE_MS);` »): the same timer with the same delay,
  now `setTimeout(() => this.#settle("pause"), CLAIM_PAUSE_MS)` in `#armClaim`: at the pause it writes the claim only
  when one is owed and retries while it cannot be written (C-2, C-5).
