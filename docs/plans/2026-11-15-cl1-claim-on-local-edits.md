# CL1 - the history claim arms on local edits only

Status: **draft, step 1, after the blind review** (2026-11-16). Rigour: **standard**. Gate: `gates/CL1.toml` (draft).
Serves W32 (the impaired network tests stay a trustworthy gate); keeps U12 (« Qui a écrit quoi »: each actor named).

## The defect

Netsim scenario 8 (NI-4), editor variant, fails C5 « <= 2 sync messages per ordered pair in 30 s once converged » in 7
of the 12 GitHub runs that measured it since 2026-11-14 (ids in `gates/CL1/WITNESSES.md`). The history panel writes a
« claim » change (`history/people.ts:81-97`) `CLAIM_PAUSE_MS` = 2 s after the last `change` event of the handle
(`ui/history/history.svelte.ts:188-191`), guarded by `patchInfo.source === "change"`. The guard is always true:
automerge-repo 2.5.6 emits `change` from one function for a local change, a peer's change and a load, with
`source: "change"` under « TODO: pass along the source (load/change/network) »
([DocHandle.ts#L268-L269, v2.5.6](https://github.com/automerge/automerge-repo/blob/v2.5.6/packages/automerge-repo/src/DocHandle.ts#L268-L269),
`#checkForChanges`; installed `dist/DocHandle.js:207-215`). So a peer's change re-arms this device's timer, a device
that never wrote claims, and with three writers the first claim re-arms the two others: claims land 4 s and more
after the last keystroke, inside the window (tests red on `73d0f4b`; the failing run fits: baseline 42.22 s, a
re-armed claim due at 42.35 s). **Not observed yet:** the claim changes in a real run (see « Observed, not inferred »).

## Options

- **A. Arm from the write paths** (`history.edited()` in each writer): six places, one not ours; a missed place leaves
  an actor unnamed, silently.
- **B. Arm when the event brings a change this device's actor wrote (chosen):** `A.getChangesMetaSince(after,
  A.getHeads(before))` (Automerge 3.5.0, `implementation.d.ts:99`) has a change whose `actor` is the actor of `after`.
  One place; it reads the document, not the callers.
- Rejected: a flag set around `handle.change`. The editor and the merge write through `handle.update` around
  `A.changeAt` (`editor/amSync.ts:222-232`, `store/merge.ts:354-358`), which it would miss (mutant « a flag around
  change »), and it needs a synchronous emission that upstream `main` has rewritten.

The claim's own change counts as a write of this actor (no exception to keep right): the timer it arms finds the claim
already right and writes nothing (`people.ts:88`).

**The actor is read at each event, never cached** (F2, measured: `A.clone` of a document with actor `97b0…` yields
`0acb…`, and the next change is written under `0acb…`; the editor's recovery hands the handle such a clone,
`amSync.ts:242`). When the actor of `after` differs from `myActor`, it replaces it, is remembered (`rememberActor`)
and the « claimed » state is reset.

**A claim owed is never lost** (F1). Arming on own writes only removes today's accidental rescue (any later change
re-armed a dropped claim). Loss paths: `detach()` clears the timer when another document is opened within 2 s; the
window closes within 2 s (the next session writes under another actor, so that one can never be claimed again);
`#claim` returns early when the device cannot write at the pause (rights unknown, no identity yet). Mechanism: one
state, « owed » (this actor wrote and its claim is not right yet), settled at three moments and rebuilt at a fourth:

```ts
const actor = A.getActorId(patchInfo.after);              // C-6
if (this.#wroteHere(patchInfo)) {                          // C-1: sets owed, arms the pause
this.#settle("detach");   this.#settle("hidden");          // C-5: written now if the device may write
this.#armClaim(); // owed and not written: retried         // C-5: at the pause, every CLAIM_PAUSE_MS while attached
this.#owedFrom(result);                                    // C-5: first analysis of an opening: sessions, no claim
```

**At the opening (integrator's decision, 2026-11-16, a precision of C-2 and C-5; U12 wins).** A claim is owed when
the document already holds a change written by the current actor (the actor id of this page's life) that no claim
names: the device that has just created the document (the creation is a change of its actor and an entry of the
history, measured), a change made before the history followed the document, a claim that could not be written when
the document was left. It is read once, from the first analysis of the opening (`#owedFrom`: a session of this actor,
no claim of it), and written a pause later. A device whose current actor wrote nothing owes nothing: reading or
receiving never claims. The tested devices therefore receive the document from a device that plays no part, except
where a test asks for the creator (`C-5 the device that created a document is named at its opening`, and C-4's
model, where the first device created the document as in scenario 8).

Flushing alone misses a device that cannot write when it leaves; rebuilding at the opening alone misses the closed
window. The retry is a timer of this device only (no change wakes it), alive only while a claim is owed. Residual
R-2: owed, unable to write, and the session ends: that actor stays « Quelqu'un ». The mutants anchor on these lines
(the anchors follow the code if step 2 writes it otherwise; the gate names titles). → W65

## Rules

Tests: `frontend/src/ui/history/claim.test.ts`: real `HistoryState`s on real `DocHandle`s, edits written as the
editor writes them (`update` around `changeAt`), changes carried as bytes through `DocHandle.update`, fake timers.
Titles are prefixed `CL1 `; mutants are in-test copies (`src/mutant.ts`), titled `CL1 mutant … is caught`.

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| C-1 | The claim timer arms, and re-arms, on a change this device's actor wrote, by whatever call (`change`, `update` around `changeAt`), and on nothing else. | `C-1 a change from another device does not re-arm the claim timer`; `C-1 every way this device writes arms the claim` | `C-1 arming on every change` -> `C-1 a change from another device re-armed the claim timer`; `C-1 a flag around change` (passes the first oracle when the local edit is written through `change`) -> `C-1 an edit written through update and changeAt did not arm the claim` |
| C-2 | A device that made no edit since its last claim (or ever) sends no claim and arms no timer, whatever it receives. | `C-2 a device that made no edit since its last claim sends no claim` (nothing written in 10 s of a peer's changes; after its own claim, a peer's change leaves no timer waiting 1 s later) | `C-2 arming at the opening` -> `C-2 a device that never wrote claimed`; `C-2 owing a claim at every opening` -> `C-2 a device that wrote nothing claimed at the opening` |
| C-3 | Kept: the claim is written `CLAIM_PAUSE_MS` after the last local edit, never while the person types; readers and devices of unknown rights write nothing (`history.test.ts`). | `C-3 the claim lands CLAIM_PAUSE_MS after the last local edit` (green before the change: a kept rule) | `C-3 a longer pause` -> same text |
| C-4 | In scenario 8 the bound of C5 stays **2**, never loosened; settle 3 s, window 30 s, typists' period unchanged (pinned by `docs`). Last claim due 2 s after the last keystroke, baseline at least 0.8 s + 3 s after it: 1.8 s of margin. | Model: `C-4 three devices: no claim traffic in the quiescence window of scenario 8`. Real stack: the `netsim` criterion reads each device's claim before the baseline. | `C-4 arming on every change` -> `C-4 claim traffic in the quiescence window` |
| C-5 | A claim owed is never lost while the session lasts: written at once when the document or the page is left, retried every pause while it cannot be written, found again at the next opening. | `C-5 a claim owed when the document or the page is left is written`; `C-5 a claim that could not be written is retried`; `C-5 a claim owed is found again when the document is opened`; `C-5 the device that created a document is named at its opening` (one claim, a pause after the opening, without an edit; the device that received the document writes nothing) | `dropping the owed claim at detach` -> `C-5 the claim owed was dropped when the document was left`; `dropping the owed claim when hidden` -> `… when the page was hidden`; `no retry` -> `C-5 the claim owed was not retried`; `nothing owed at the opening` -> `C-5 the claim owed was not found again at the opening`; `the creator not named at the opening` -> `C-5 the creator of the document was not named at its opening` |
| C-6 | « This device's actor » is the actor of the document now: after a clone, the new actor is claimed too. | `C-6 the actor a clone gives this device is claimed too` (its first assertion pins that a clone changes the actor) | `the actor cached at the opening` -> `C-6 the actor after a clone was not claimed` |

## Observed, not inferred

Scenario 8 now records, in its result and in the job's log, when each device wrote its claim against the quiescence
baseline (`claims`, `netsim <run> claims written, ms from the quiescence baseline: …`), read from
`window.__inkwellClaims(): { actor, mine, at }[]`. That hook does not exist yet (`claims: null`, a report, never a
clause, so other blocks' runs are unaffected). Step 2 adds it to `workspace/observe.ts` (instrumented builds only)
**before** the fix and runs the `netsim` criterion once: the first real observation of the cause, expected red on a
late claim. The criterion then requires one claim per device, each before the baseline.

## What proves the fix

**R-1, confirmed.** The defect does not show on this machine: ten strict runs of ten pass on the unchanged tree
(`WITNESSES.md`). The local `netsim` criterion (one run per seed, 20771 and 20772) is a non-regression check, plus the
observation above. Proof before the merge: the deterministic tests and mutants (`frontend`). **Acceptance after the
merge:** `gates/CL1/GITHUB.md` (workflow `ci`, job `netsim`; ten measured runs over three seeds or more, hence three
UTC days; reruns and unmeasured runs not counted; one failure reopens). Ten passes by chance: (5/12)^10 = 0.00016 if
runs are independent, 0.023 at the 95 % lower bound of the rate (0.315); (5/12)^3 = 0.072 if one seed is one draw,
which is why the claims lines are copied too. `gates/deferred.jsonl` does not fit this obligation: its rows are
closed by the next passing local run of a lot (`scripts/gate.py`, `LEDGER`), which is not this evidence; the record
is `GITHUB.md`, whose `Status: open` the `docs` criterion checks, and a line of `PROGRESS.md` until it closes. → CL1

Budget: `max_minutes = 20`; caps: frontend 30 + 120 s, netsim 240 + 360 s, regression 30 + 180 s; 960 s in all.
