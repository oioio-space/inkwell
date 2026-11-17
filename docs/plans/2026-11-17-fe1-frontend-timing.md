# FE1 — four frontend tests that measured the runner, not the code

Rigour: light (tests of `frontend/src`; one production change: a work counter in `history/pieces.ts`, read only by
tests). Precedent: TC1 (`2026-11-16-tc1-fold-cost-by-counts.md`), cost asserted by counts.

## Symptom

CI run 43979066870, job `ci`, `fe:check` (4-vCPU `ubuntu-latest`; `fe:check` ran from 23:37 to 23:53 while
`test:ci`, the Go suites, ran from 23:33 to 23:48 in the same job): 4 failures out of 1378 tests.

| Test | CI | Here, idle (20 threads) |
|---|---|---|
| `track.test.ts` DIF-05 large document | `expected 32821 to be less than 20000` (ms, `Tracker.update`), test 46.8 s | 3.7 s, update 2.56 s |
| `track.test.ts` no quadratic cost | timeout 120 s (195.8 s) | 24.7 s |
| `admission-adapter.test.ts` 3 invented ids | timeout 5 s (5.95 s) | 0.85 s |
| `admission.test.ts` 2 safe snapshot values | timeout 5 s (6.39 s) | 0.35 s |

Same class, passed narrowly: `admission.test.ts` « 3 P2 drops 50,000 invented ids », 4.35 s on CI for 0.85 s here.
Fixed with the adapter test (same cause).

No coverage build is involved: `fe:check` runs `vitest run` without coverage. The cause common to all is CPU
contention (Go suites and three Vitest workers on four vCPU): 5 to 15 times slower than here. What each test did
with that time is below.

## Causes, measured (`performance.now()` probes, removed)

- **No quadratic cost.** Of 24.7 s, the fixture took 24.4 s: `A.updateText` of the 30 000-line rewrite 20.5 s, the
  merge 3.8 s; `blameOf`, the code under test, 0.25 s. Automerge's cost of the edit grows with the number of edits
  and with the text length (one change of N splices into « ligne N » lines: 3 750 → 70 ms, 7 500 → 145 ms,
  15 000 → 1.0 s, 30 000 → 3.5 s; `updateText` 0.4, 1.0, 4.4, 17.3 s). The 4 s bound timed `blameOf` only, but the
  fixture alone blew the 120 s cap.
- **DIF-05.** The update spent 2.50 of its 2.56 s in `A.diff`, about 150 ms for each of the 17 sessions of the
  1 M-character two-author document (a diff between historical heads; a diff ending at the current heads costs
  < 1 ms). The tracker's own work is about 60 ms. The 20 s bound measured Automerge under load.
- **Invented ids (adapter).** Of 1.2 s for 50 000 messages, the test's input took 1.05 s (`encodeCBOR`, about 21 µs
  a small map: every map key and value gets its own 256-byte `Writer`), the adapter 0.11 s (2.2 µs a message).
- **Safe snapshot values.** 275 to 330 ms is fast-check building, once per process, its table for
  `fc.string({ unit: "binary" })` (`getOrCreateStringUnitInstance`, cached; a second build costs 0.03 ms). The
  100 runs cost 4 to 43 ms over 60 seeds. Whichever test of the file builds that arbitrary first pays it.

## Product or test

Test, in all four. Product observations, with numbers, for the plans they belong to:
- `store/merge.ts` uses `A.updateText` for an external rewrite whose regions are all under 64 Ki: an external tool
  that touches each of 30 000 lines of a 300 KB file costs about 17 s at idle on this machine (Automerge, superlinear
  in the number of edits). Residual R-FE1-1 for the external-edit plan (ADR 0028): measure it in the product path and
  bound it (for example the line-region splices of the locked-zone path cost 3.5 s, still superlinear).
- DIF-05 costs one historical `A.diff` per session, each O(document): 150 ms at 1 M characters here, so a 5 MB
  document with many sessions costs seconds to minutes on its first analysis. It runs in a Worker
  (`analysis.worker.ts`), so the page is not frozen; the slices yield between sessions, never inside one diff.
  Residual R-FE1-2 for the history plan: a budget at 5 MB stated by sessions × diff.
- `encodeCBOR` at about 21 µs a small map is not the cause of any failure; noted for the protocol package.

## Fix (no bound raised, no run or input removed, no timeout lengthened)

| Test | Defect class it keeps | Assertion now | Hand mutant → caught by |
|---|---|---|---|
| no quadratic cost | following one session's N scattered edits walks the text (or every piece) per edit | `pieceWork.finds === 1 + 30 000` and `pieceWork.steps ≤ workBound(30 001)` = 99 663 682; measured 46 067 177; text, 60 001 spans | MT1 blocks never split (flat list): 1 800 150 007 steps; MT3 one `toArray` per inserted run: 947 803 663 steps |
| DIF-05 | each session followed once; slices yield; hover exact | `finds === 17` (one per run typed), `steps ≤ workBound(17)` (measured 629), `ticks > 0`, the hover and tiling checks unchanged | MT2 the sessions followed again from the first at each one (same result): `expected 137 to be 17` |
| invented ids (adapter) | no invented id gets past the scope check to Repo access, handles or the repo's messages | 50 000 distinct ids, their CBOR encoded once by `encodeCBOR` and the digits rewritten in a copy (`inventedIds`, `net/testkit.ts`), checked byte for byte against `encodeCBOR` for four ids; the 50 000 are dropped as « invalid peer message », none reaches the repo, then the same request for the admitted document does (control) | MA1 the scope check removed (`admission.ts`, `c.admitted.has` → admit): `expected undefined to be 50000` |
| P2 50 000 invented ids (`admission.test.ts`) | `parsePeerMessage` drops every invented id | the same 50 000 ids through `inventedIds`, the admitted ones counted: 0 | MA1: `expected 50000 to be +0` |
| safe snapshot values | an admitted snapshot carries only safe keys and values | unchanged, 100 runs, random seed; the binary-string arbitrary is built once at the top of the file, as fast-check caches it anyway | MA2 the name check dropped (`validPresenceName` → any string): red |

**The adapter test was vacuous before FE1.** MA1, and even the scope and « announced » checks removed together, left
the old test green: constructing the `Repo` reconnected the adapter under a random peer id, so every message (whose
`targetId` is « me ») was refused on its target before its document id was read, and an empty `data` would have been
dropped further on by the coverage filter (`sync message dropped`, measured). The test now gives the repo the peer id
« me », sends a real sync message (`A.generateSyncMessage`), counts the drops by reason, and ends with the admitted
control; MA1 turns it red. A strengthening, not a weakening: every old assertion is kept.

`PieceWork` (`history/pieces.ts`): two counters, `finds` (places looked up) and `steps` (blocks and pieces walked,
copied or shifted, counted at their upper bound in `locate`), incremented by the list itself and shared with its
copies; a `Tracker` owns one (`tracker.work`) and nothing in the application reads it. A first version kept one
module-wide counter: under the harshest contention below, DIF-05 timed out while its update went on yielding, and its
finds leaked into the next test (`expected 30002 to be 30001`). Counted per tracker, a leftover cannot reach another.
The P2 test counts the ids admitted and asserts once (`admitted === 0`): 50 000 `expect` calls were most of its
remaining cost. Alternatives set aside: process CPU time (still load-dependent), fake timers (they control timers, not
CPU), a smaller fixture (would hide the flat-list defect: it shows only past a few blocks of 512 pieces), a hook in the
`Tracker` API (an API for a test). The fixture of « no quadratic cost » now types lines of one character and makes the
30 000 edits as splices in one change (0.67 s instead of 24 s): the list follows runs and pieces, not characters.

Sources: Vitest's `testTimeout` is a wall-clock cap per test (https://vitest.dev/config/#testtimeout), so a fixed
amount of work fails it on any machine slow enough; fast-check caches each string unit after its first build
(`getOrCreateStringUnitInstance`, `node_modules/fast-check/lib/fast-check.js`, 4.10.2); counting operations instead of
timing them is the Go standard library's practice (`testing.AllocsPerRun`, `sort`'s `TestCountSortOps`), cited by TC1.

Follow-up, not taken here: in CI, `fe:check` shares four vCPU with `test:ci` (`mise.toml`, task `ci`); serialising them,
or giving Vitest a fixed `maxWorkers`, removes the load itself. It changes the CI's length and is a decision for the CI
plan.

## Proof

Contention: the three files run by Vitest pinned to two CPUs with N busy loops pinned to the same two
(`taskset -c 0,1`, under `scripts/heavy.sh`). N = 4 is calibrated on the CI: it reproduces DIF-05's failure with the
same text (`expected 37010.93 to be less than 20000`, CI 32821) and slows the unchanged P2 test 9.3× (CI 5.1×); N = 6
is harsher than the CI (P2 13.8×).

| Run | DIF-05 | no quadratic | adapter ids | P2 ids | snapshot values |
|---|---|---|---|---|---|
| before, N = 4 | red, 37 011 ms > 20 000 | red, timeout 120 s (316 s) | red, timeout (7.9 s) | red, timeout (7.9 s) | green (passed after a warm-up) |
| before, N = 6 | red, timeout 60 s | red, timeout (452 s) | red, timeout (9.9 s) | red, timeout (11.7 s) | red, timeout (5.9 s) |
| after, N = 4, three runs | green, 51.6 / 52.7 / 52.6 s | green, 12.0 / 12.3 / 11.0 s | green, 3.9 / 3.6 / 3.7 s | green, 2.0 / 1.9 / 2.0 s | green, 0.25 / 0.34 / 0.24 s |
| after, N = 6 | red, timeout 60 s (62.9 s) | green, 20.8 s | red, timeout (5.3 s) | green, 2.7 s | green, 0.36 s |
| after, idle | 4.0 s | 0.77 s | 0.40 s | 0.21 s | 0.015 s |

Hand mutants (each applied alone, the file restored after): MT1, MT2, MT3, MA1 (adapter and P2), MA2, all red with
the texts in the table above.

**What is left, said plainly.** At N = 6, two tests still pass their cap, and no test change can fix that without
weakening one. DIF-05 is the product's own work: 17 historical `A.diff` calls on a 1 M-character document, 2.5 s idle
here, 32.8 s on the CI runner, 52 s at N = 4, past the 60 s cap at N = 6. The adapter test's 50 000 messages cost
0.1 s in the adapter itself; the rest of its 0.4 s is the Repo's start and stop. Both have their margin only at the
CI's measured level of load (DIF-05: 46.8 s on the CI against 60 s). The cause of that load is the CI's job layout
(follow-up above), and DIF-05's cost per session is residual R-FE1-2.
