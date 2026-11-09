# CI/CD study: faster, cheaper, no weaker (2026-11-08)

Request (exigence): "study a qualitative, optimized, fast and performant CI/CD". Read-only study; nothing was
changed. Every number below is either **[M] measured** (log evidence, path given) or **[E] estimated**
(method given). No gate, threshold, mutation score, netsim scenario or native-perf run is weakened by any
recommendation; where a check moves, it moves earlier or runs in parallel, never away.

## 1. Baseline (evidence)

Sources: `/tmp/inkwell-push.log` (pre-push `mise run ci`, 23:15-23:19 on 2026-11-08, still running when read),
scratchpad `push-rl1*.log`, `done-rl1-full*.log`, `done-P3.log`, `netsim.log`, `ni*-diag.log`; `mise.toml`,
`.github/workflows/ci.yml`, `release.yml`, `scripts/{heavy,lint,cover,deadcode,sec-scan}.sh`.
Machine: 20 CPUs, 30 GB RAM, warm Go cache (34 GB in `~/.cache/go-build`, never trimmed by us).

`mise run ci` = 18 tasks in `depends`, run by mise in parallel (default 4 jobs), then `e2e:netsim:impaired`
which `wait_for`s eight of them (to avoid the `frontend/dist` race).

| Task | Duration [M] (warm) | On critical path? | Duplicated / note |
|------|--------------------|-------------------|-------------------|
| test:py, lint:env, lint:claude-md, lint:sh/yaml/actions, pkg:lint, scan:secrets | 0.1-2 s each | no | gitleaks also runs in pre-commit (staged) |
| sidecar:fetch, fe:install, fe:bindings | 1.0 s, 0.13 s, 2.7 s | start of chain | `fe:bindings` + `sidecar:fetch` re-run as dependency of 5 tasks (mise dedups per run, not per process: each manual `mise run` repeats them, ~0.8 s) |
| lint (vet + golangci-lint + build) | 9.5 s warm (3.7 s in `done`) | no | `go vet` + `go build` repeat what test/lint compile; same lint ran in pre-commit (staged) |
| deadcode | 6.3 s | no | `go run x/tools/deadcode` compiles the tool each time |
| scan:code (gosec + govulncheck), sbom, scan:sbom (grype) | 6.5 s, 6.6 s, 3.8 s | no | pre-commit runs gosec/govulncheck on staged Go as well |
| arch | 20.6 s | no | |
| tla / alloy (formal) | 40.2 s / 26.4 s | no (parallel) | run again in `done` step 4 (9.4 s, cached) |
| test:ci (Go, -short, -coverpkg=./..., atomic) | 27.8 s (cache hit; cold much longer) | **yes** (gates cover:check) | `-coverpkg=./...` makes every package's cache key depend on all packages: any edit invalidates every test result (see R1) |
| cover:check | 0.17 s | after test:ci | falls back to re-running the whole test suite if `coverage.out` is missing |
| fe:check (svelte-check, Biome, Vitest) | 40-80 s | **yes** among the parallel set | also run by `done` (42 s) and by the `ci` of GitHub |
| e2e:build (vite + 2 go builds + Playwright browser check) | 1.5-3.7 s warm, 12.9 s once | **yes**, serialized behind 8 tasks | second vite build of the same frontend as `fe:build` / `app:build` |
| e2e:netsim:impaired (scenarios 5-8, seeds) | ~17 min (ci.yml comment; scenario 6 alone 1.2 min, 46 s, 32 s; failing diagnoses 2.5-3 min each) | **yes, dominant** | runs in the local push, in the GitHub `ci` job (via `mise run ci`) AND again in the GitHub `netsim` job |
| e2e:netsim (scenarios 1-4) | 154.5 s [M] (`netsim.log`) | in GitHub `netsim` only | |

Measured end-to-end times of whole attempts:

| Run | Wall time [M] | Outcome |
|-----|---------------|---------|
| push attempt 23:11:50 -> 23:14:06 (`push-rl1d.log`) | 2 min 16 s | failed at `cover:check`: `internal/transport` below 85 % |
| push 23:15:35 -> >=23:18:43, still in scenario 6 of 8 | >3 min and ~17 min expected | impaired scenario 5 failed once, retries in progress |
| `mise run done -- RL1` (`done-rl1-full6.log`) | 33.7 min (22:31 -> 23:05) | green, mostly mutation + native gate |
| `done` before that (`full4`, `full5`) | 35 min, 11 min | failed at step 7 (gate), wasted 35 min |
| P3 `done` (`done-P3.log`) | 35 min | failed at step 7 |

**Baseline wall time of one pre-push: ~2.3 min of checks (warm, parallel) + ~17 min of impaired netsim =
~19-20 min [M for the first part, documented for the second]; a failure found at cover:check costs ~2.3 min,
but a failure found in step 7 of `done` costs 35 min.** GitHub: one `ci` job runs everything serially
(`apt-get`, mise install, `sidecar:trace`, `mise run ci`, `app:build`, `e2e`) with a 45 min timeout, plus a
50 min `netsim` job that repeats the impaired suite, plus an arm64 job. The GitHub critical path is not
measured here (no run logs fetched; no `gh` call made during the full CI). [E] 40-60 min.

## 2. Problems found, with evidence

P1. **The impaired netsim suite is 85 % of local CI time and is the last step.** `mise.toml` `ci` depends on it;
`e2e:build` `wait_for`s eight tasks, so the browser suite cannot overlap anything (it exists because of the
`frontend/dist` race, comment in `mise.toml`). Cause is structural, not the scenarios.
P2. **Cheap failures arrive late relative to their cost.** Coverage (`internal/transport` < 85 %) was only
discovered by the push, after the commit; `deadcode` and the `frontend/dist` embed race likewise. The same checks
exist in `done` but `done` skips them or runs them in a different order.
P3. **`-coverpkg=./...` defeats Go's test cache granularity.** Every test binary links all packages, so any
code edit changes every cache key. Observed: `test:ci` is 27.8 s only when nothing changed.
P4. **Duplicated work.** (a) `netsim:impaired` runs twice on GitHub (`ci` job and `netsim` job). (b) lint, scan,
fe:check and formal run in pre-commit, `done`, pre-push and GitHub. (c) vite builds the frontend three times
(`fe:build`, `app:build`, `e2e:build`) with different flags. (d) `sidecar:fetch` + `fe:bindings` (0.8 s) in every
task invocation.
P5. **Serialization by `heavy.sh`.** Two slots, 8 GB cgroup each; the suites that dominate (`test:ci`,
`fe:check`, netsim, `test:py`, alloy) all need a slot, so at most 2 of them overlap even with 20 idle CPUs.
It protects against the 3 OOMs of Oct-Nov; the cap is right, the *slot count of 2 for small jobs* is not
(`fe:check` ~1-2 GB, alloy ~2 GB).
P6. **GitHub job is one serial 45 min box.** No sharding, no matrix, no `needs:` fan-out; a lint error is
reported after install + sidecar trace + toolchain restore. `fail-fast` ordering absent: cheap checks do not
gate expensive ones.
P7. **Cache keys are coarse.** Go cache key `hashFiles('**/go.sum')` only: build cache stored once per
dependency change, restored but never refreshed (actions/cache does not overwrite a hit); Playwright browsers
not cached at all (`playwright install --with-deps` each run); `~/.cache/go-build` locally is 34 GB, unmanaged.
P8. **Flaky vs real failures are indistinguishable.** NI-1/NI-2 scenario failures tonight (`ni2-diag`,
`rl1-ni2`, `ni3-diag`) were each 2.5-3 min to fail and were investigated by hand; Playwright `retries: 1` in CI
hides them. No record of pass/fail per scenario and seed over time.
P9. **Stale comment / drift.** `.githooks/pre-commit` header says it runs tests; `lint.sh` says it does not.
Cheap to fix, but proves hook text is not checked (`lint:claude-md` covers only CLAUDE.md).

## 3. State of the art (sources)

- Go test result caching keys on package inputs; `-coverpkg` and `-count=1` defeat it
  (https://pkg.go.dev/cmd/go#hdr-Testing_flags, https://go.dev/doc/go1.10#test).
- Affected-package selection by `go list -deps` + reverse dependency walk: DigitalOcean GTA
  (https://digitalocean.com/blog/gta-detecting-affected-dependent-go-packages), cascade
  (https://pkg.go.dev/github.com/geomyidia/cascade), engineered.at write-up
  (https://engineered.at/articles/testing-only-changed-go-packages). They help most on a runner with an empty
  cache; with a warm cache `go test` already skips unchanged packages. Test-impact selection must always fall
  back to the full suite on `go.mod`/`mise.toml`/script changes.
- setup-go caches GOCACHE but does not refresh it unless the key includes sources (ADR:
  https://github.com/actions/setup-go/blob/main/docs/adrs/0000-caching-dependencies.md).
- golangci-lint-action `only-new-issues`/`--new-from-rev` (https://github.com/golangci/golangci-lint-action);
  issue #820 warns two PRs can each pass and fail together after merge, so use it only on the PR check, with the
  full run on `main` and in the merge queue.
- Playwright sharding with blob reports and `merge-reports` (https://playwright.dev/docs/test-sharding);
  browser cache path `~/.cache/ms-playwright` keyed on the Playwright version (not re-verified in this study).
- Merge queue: `merge_group` trigger, concurrency group `workflow-ref` is safe because queue runs have their
  own ref (community notes; not confirmed on GitHub docs here, https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#merge_group).
- mise `sources`/`outputs` on tasks skip a task when its inputs are unchanged
  (https://mise.jdx.dev/tasks/task-configuration.html); none of our 60 tasks sets `sources`.
- gotestsum `--rerun-fails` and JUnit output for flaky tracking (https://github.com/gotestyourself/gotestsum).
- Not found / not verified: concrete "Go 1.27 build-cache features" (no source found; repo is on 1.27.3, `GOCACHEPROG`
  remote cache protocol exists since Go 1.21 and is the supported path to a shared cache).

Beyond the state of the art, for this repo: a **content-keyed receipt** per check (hash of inputs -> pass),
stored in `$GIT_DIR` locally and in `actions/cache` on GitHub, so a check that already passed on the exact
tree is never rerun, anywhere. Same idea as the `claude-simplify-ok` marker, applied to every gate.

## 4. Recommendations (ranked)

Gain = wall time of a pre-push on a typical diff unless stated. "Prove" always compares before/after with
`hyperfine --runs 3 'mise run ci'` (machine at rest) plus a red-team: a seeded regression per moved check must
still fail.

**R1. Split the impaired netsim out of the blocking path and run it in parallel with everything, not after.**
Remove the `wait_for` by building the e2e frontend into its own directory (`VITE --outDir .e2e-dist`) and
embedding via a build tag or copy-on-build in `e2e:build` so no Go task sees an empty `dist`; the race
disappears at the root. Netsim (heavy slot 1) then overlaps `test:ci` + `fe:check` (slot 2).
Gain [E]: ~17 min -> max(17, 3) = still 17 min of netsim but starting at t=0.3 min instead of t=2.5 min:
saves ~2 min. Real gain is R2. Cost: medium (embedding path). Risk: low; proof: dist-race reproduction test
(run `go build ./...` in a loop while `e2e:build` runs; zero embed errors in 20 runs).
**R2. Netsim impaired in the pre-push only for a diff that touches its inputs; always in GitHub and `done`.**
Inputs = `internal/transport/**`, `internal/sync*`, `internal/bridge/**`, `scripts/netsim/**`, `protocol/**`,
frontend sync code, `mise.toml`, `go.mod`. Rule: unknown path => run. This is a *selection*, with a content
receipt (R8) so a later push of the same tree is free. Gain [E]: ~17 min saved on ~70 % of pushes (docs, UI,
tooling); the check still runs on GitHub on every push to `main`, on every `done`, and in the merge queue.
Risk: a missed dependency edge; mitigation = `go list -deps` of the netsim binaries computes the input set
instead of a hand list. Prove: mutation test in `netsim:mutants` still runs; a seeded transport change must trigger.
**R3. Remove the duplicate impaired run on GitHub** (`ci` job calls `mise run ci` which includes it, and `netsim`
job repeats it). Make `ci` exclude it (`CI_SKIP_NETSIM=1` gives a `ci:core` task) and let the `netsim` job own
scenarios 1-8 in parallel with `ci`. Gain [M-based]: -17 min of the serial `ci` job; GitHub wall time becomes
max(ci, netsim). Cost: low (10 lines). Risk: none, nothing is removed. Prove: workflow run times before/after.
**Status (2026-11-09): R2 and R3 are done in block CI1** (frozen gate `gates/CI1.toml`). `scripts/netsim/affected.py`
computes the input set from `go list -deps` of the netsim binaries (no hand list) and the pre-push hook runs the
impaired netsim only when the pushed diff touches it; `ci` = `ci:core` + `e2e:netsim:impaired`, GitHub's `ci` job runs
`ci:core` and its `netsim` job owns scenarios 1-8 once, in parallel.
**R4. Shift coverage left: `mise run ci:fast` in the pre-commit for the touched packages.**
`go test -short -cover` of the packages touched by the staged diff (+ reverse deps from `go list -deps`) against
the per-package floor in `cover.sh` (85 %). The coverage failure tonight cost one push (2.3 min) and an
interruption; it would be caught in ~10-30 s at commit. Also run `deadcode` and `arch` there (6 s + 21 s, only
if `.go` is staged; run them in parallel, budget < 30 s total with warm caches). Cost: medium; risk: pre-commit
over 30 s (mitigation: cap, then fall back to pre-push). Prove: time 20 real commits from git log.
**R5. Replace the repo-wide `-coverpkg=./...` in test:ci with per-package coverage plus one merged cross-package
run only on GitHub.** Per-package `-cover` keeps the test cache valid per package: edit one package, retest only
it and its reverse deps. The 85 % per-package rule is then measured on each package's own tests; cross-package
coverage (which the repo counts today) is kept as the GitHub-only full run, so the gate is unchanged where it
is authoritative. Gain [E]: `test:ci` from full rebuild (cold ~minutes) to seconds-to-tens-of-seconds on a one-package edit.
Risk: medium: the current gate counts coverage obtained from other packages' tests; to be decided by an ADR
before use; do not change until a diff of per-package numbers is produced (`go tool cover -func` old vs new).
**R6. Raise `heavy.sh` slots by weight, not count.** Declare a weight per job (`fe:check` 2 GB, alloy 2 GB,
`test:py` 1 GB, `test:ci` 6 GB, netsim 5 GB, native-perf = exclusive) and admit jobs while the sum of caps is
<= 16 GB and load permits; the 8 GB per-job cgroup limit stays per job. Gain [E]: 30-45 s on the checks phase.
Risk: OOM regression (3 prior incidents); mitigation: keep total <= 16 GB of 30 GB, `--exclusive` unchanged.
Prove: `systemd-cgls`/`peak memory` log across 10 runs; zero 137.
**R7. GitHub job graph with fail-fast ordering and caches.**
Stage 0 (30 s): `lint:*`, `lint:env`, `arch`, gitleaks, `actionlint`, `go vet` => `needs` of all heavy jobs.
Stage 1 (parallel jobs): `test` (Go, matrix by package group, 2-3 shards via `go list` + `gotestsum`),
`fe` (fe:check + Vitest), `scan` (gosec, govulncheck, sbom, grype), `formal` (tla/alloy), `e2e` (Playwright
sharded 2x with blob reports), `netsim`, `arm64`. Stage 2: `coverage` (merge profiles, gate), `package`. Cache:
`actions/cache` with key `go-${hashFiles(go.sum)}-${hashFiles('**/*.go')}` and restore-keys `go-${go.sum}-`
(GOCACHE refreshed per commit as the setup-go ADR describes), `~/.cache/ms-playwright` keyed on the Playwright
version, `~/.cache/typst` etc. keyed on `manifest.json` (already). Concurrency cancel is already in `ci.yml`;
add `merge_group:` trigger for a merge queue so `main` only receives green trees. Gain [E]: GitHub wall time 40-60 min -> ~20-25 min;
cold start of Go build -2-4 min with refreshed key. Cost: medium-high (rewrite ci.yml). Risk: more runner minutes;
mitigation: a stage-0 failure cancels the rest. Prove: three consecutive GitHub runs, compare `gh run list --json durationMs`.
**R8. Check receipts** (`scripts/receipt.sh <check> <inputs...>`): hash of check name + tool versions + input
files; result stored in `.git/ci-receipts/` and uploaded as an `actions/cache` entry on GitHub. `ci`, `done`,
pre-push and pre-commit consult it. Directly removes P4(b): lint, scan, formal and fe:check pass once per tree.
Gain [E]: `done` + push on an unchanged tree: from 33-35 min + 20 min to the cost of the missing checks.
Risk: stale receipt hides a failure; mitigation: inputs include the tool pin and the script itself; receipts expire at 7 days;
any `--no-receipt` flag is ignored in CI. Prove: seeded failure per check must invalidate its receipt.
**R9. Make `done` fail fast.** Move step 7 (native perf gate, 35 min wasted twice tonight) before mutation
only when its preconditions (machine at rest, `exclusive.jsonl`) can be evaluated cheaply; otherwise order the
cheap preconditions of step 7 (does the gate file parse, does the native binary build, are slots free) before step 5.
Gain [M]: tonight's two failures cost 35 + 35 min. The step order in CLAUDE.md ("last because longest") stays;
only a cheap pre-check is added. Cost: low.
**R10. Flaky-test quarantine with evidence.** `gotestsum --rerun-fails=1 --junitfile` for Go; for netsim record
`{scenario, seed, result, duration}` in `gates/netsim-history.jsonl` per run; a scenario is called flaky only
after the same seed passes and fails on the same commit (rerun proof), never on a single failure. Quarantine =
an entry in the history file with owner and expiry, and the test still *runs* and reports, only not blocking the
merge queue. Gain: investigation time (tonight ~2 h across NI-1/NI-2/NI-3). Risk: hiding real races; mitigation:
expiry 7 days, quarantine never applies to protocol/security tests.
**R11. Small wins.** (a) `mise` `sources`/`outputs` for `fe:bindings`, `sidecar:fetch`, `deadcode`-tool build, `sbom`
(~3-5 s per run). (b) `go run x/tools/deadcode` -> `go build` once into `$XDG_CACHE_HOME` pinned by version (~4 s).
(c) `go clean -cache` trim of `~/.cache/go-build` by age (34 GB; disk, not time). (d) Fix the pre-commit header.
(e) default `NETSIM_SEEDS` is the day number: keep it (reproducible) but record it in the history file.

## 5. Target pipeline

| Stage | Where | Budget | Content |
|-------|-------|--------|---------|
| pre-commit | local, staged diff | < 30 s | gitleaks staged, gosec/govulncheck staged, gofmt/vet/golangci-lint on touched packages, coverage floor and `deadcode`/`arch` on touched packages (R4), `/simplify` attestation, receipts consulted |
| pre-push | local | < 5 min typical | `ci:core` in parallel (lint, scans, formal, fe:check, Go tests, cover:check), netsim impaired only if its input set changed (R2), all receipt-skipped when the tree already passed |
| GitHub PR | Actions | ~10 min | stage 0 cheap gates then test/fe/scan/formal/e2e shards in parallel; cancel superseded runs |
| merge queue / `main` | Actions | ~20-25 min | everything above + netsim 1-8 + arm64 + package build, always full (no selection) |
| `done` (block) | local, rest of the machine | as today | adds the cheap step-7 preconditions first (R9) |
| release | `release.yml` | as today | no cache (deliberate), unchanged |

## 6. What must not be cut

- Frozen gates (`gates/*.toml`), mutation (>= 95 % on `protocol/*`), TLA+/Alloy with mutants, fuzzing of decoders.
- Netsim scenarios 1-8 and `netsim:mutants`: only *selected out of a local push* when provably unaffected (R2); full on GitHub and in `done`.
- Native perf gate: machine at rest, `--exclusive`, load threshold untouched; never moved to a shared runner, never run concurrently with other work.
- `gitleaks` over all history on GitHub, `grype --fail-on high`, release without cache.
- Memory caps per job; the slot admission change (R6) keeps them.
- Playwright `retries: 1` in CI stays; flakiness is tracked (R10), not hidden by raising retries.

## 7. Order of execution and proof plan

1. R3 (duplicate on GitHub), R11, R9: hours, no risk. 2. R8 + R4: removes repeated checks and shifts coverage left.
3. R1 + R2: removes the dominant 17 min. 4. R6, R5 (ADR first). 5. R7 (rewrite ci.yml) with R10.
Each step ends with: `hyperfine --runs 3 --warmup 1 'mise run ci'` before and after on a quiet machine, a seeded
regression that the moved check must still catch, and an entry in `docs/exigences.md` and `PROGRESS.md`.
Limits of this study: GitHub run durations were not fetched (no `gh` call during the full local CI); the 17 min
of netsim comes from the comment in `ci.yml` and scenario-level logs, not a timestamped full run; the push log has
no timestamps, so per-task wall times are task-reported durations, not offsets.
