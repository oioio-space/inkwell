# RT1 — red witnesses (step 1, 2026-11-25, tree `main` 10d8e76 plus the step-1 documents)

Each criterion of `gates/RT1.toml` was run once on this tree, before any code, then the criteria changed by the
pre-freeze review (`REVIEW.md`, round 1: docs, frontend, go, mutants, e2e) were run once more. Each is red for the reason below, and
each can pass with the real tools (where they write, what they produce) once step 2 lands.

| Criterion | Red now because | Turns green when |
|-----------|-----------------|------------------|
| `docs` | ADR 0020's RT1 amendment says « proposé », not « accepté »; `start.rootUnreadable` and `store.openElsewhere` are absent from `fr.json`; the vector `superseded push` is absent from `internal/bridge/testdata/storestream.json`. (Already green: the plan rows, the `candidate.sh` merge condition, the « Conséquence assumée » paragraph, R-RT1-1 under E2E1, R-RT1-2..4 under RT2; rerun after the review: same four reasons, nothing else.) | Step 2 adds the keys and the vector; the amendment is accepted at the merge. |
| `frontend` | Biome, svelte-check and the whole Vitest suite pass (exit 0, nothing failed, 309 s; the given repro `transientLoad.test.ts` was first formatted by Biome, its only errors); the `RT1 …` rule titles are absent (seven; rerun after the review, 311 s: exit 0, nothing failed, the ten titles of the revised gate absent). | The rule tests of A-1..A-4 and C-2 exist and pass. |
| `go` | `go test -race` of `internal/app`, `internal/bridge`, `internal/project` passes (exit 0, nothing failed, 116 s); the `TestRT1…` tests are absent (`TestStoreVectors` passes, without the new vector); rerun after the review with B-1..B-3 in `internal/project` and `TestRT1B3JoinRefusedWhileRootUnknown`: exit 0, nothing failed, 114 s, the five `TestRT1…` absent. | B-1..B-3 (in `internal/project`), the B-3 clause of `Session.stored` and C-1 tests exist and pass. |
| `mutants` | Block RT1 has no entry in `scripts/netsim/mutants.toml`: MR1..MR9, then MR1..MR14 after the review, missing (entries are written at step 2 against the code's anchors, as `mutants.py` expects: a gate is checked once its block has an entry). | Each entry exists with its exact rule title (vitest `name`) or test and `expect` (go), and `mutants.py RT1` catches all fourteen. |
| `e2e` | Run once at 12 repetitions (184 s in all, build included; `--exclusive`): Chromium and WebKit both exit 0, errors 0, 12 expected runs of the original flow, nothing flaky or skipped; `RT1-E2E …` has 0 runs (absent). After the review: a stub of RT1-E2E measured 12.9 s a run (Chromium), 14-15 s (WebKit), and the criterion became RT1-E2E × 15 plus the old flow × 20 per engine, cap 1200 s; rerun (302 s, build cached): the old flow 20 of 20 expected on both engines (71 s Chromium, 221 s WebKit, as estimated); RT1-E2E « No tests found » on both (exit 1, 0 of 15): absent. | `RT1-E2E …` has 15 and the old flow 20 expected runs per engine, nothing flaky or skipped. |
| `simplify-report` | `gates/RT1/SIMPLIFY.md` does not exist. | The `/simplify` pass of step 3 writes it. |

## Why the e2e forces the race

`persistence.spec.ts:37` hits the race only when page 1's 4 s `refresh()` falls inside the fresh page's start load
(1 in 15 at rest). The new `RT1-E2E` test makes it certain: it keeps page 1 issuing store requests while the fresh
page starts, then asserts that page 1 shows `store.openElsewhere` and stops, that the fresh page shows the saved text,
and that `project.json`'s `rootUrl` is the one recorded before. Repetition of the original flow (12 per engine) is the
statistical check; the forced test is the deterministic one.

## Measured facts behind the plan

- automerge-repo 2.5.6, probe in the shape of `frontend/src/store/transientLoad.test.ts` (run under `heavy.sh`,
  removed after): first `find()` with one failing storage read rejects `store stream closed`; a second `find()` of the
  same id in the same Repo did not settle within 1.5 s (`hang`); a `find()` against an empty storage rejects
  `Document automerge:… is unavailable`. Hence A-2 (the adapter resends the read) instead of a retry of `find()`.
