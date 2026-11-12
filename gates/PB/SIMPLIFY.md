# /simplify — block PB (paired performance comparison, ADR 0038) (2026-11-12)

Scope: the whole PB diff.

- `scripts/perf`: `paired.py`, `refbuild.py`, `pbmodel.py`, `baseline.py`, `start.cjs`, `type_native.py`,
  `native_input.py`, their tests, and `testdata/` (fake server and quantile vector).
- `internal/nativeperf` (`verdict.go`, `service.go`, tests), `cmd/inkwell/main_perf_native.go` and
  `frontend/src/perf/native.ts`.
- `scripts/gate-selftest.py`, `mise.toml` (`test:py`), `gates/guard-overrides.json` and the docs rows
  (`engineering-principles.md` §3, `coherence.md` §5).

Two free models wrote part of the code; it was read against the plan, and the tests it relies on were re-run.

## Findings, applied

- **Reuse: one rule, two consumers.** `pbmodel.py` imports the verdict, the Walsh bounds and the order statistic from
  `paired.py`, so the model that proves the rule and the gate that applies it cannot drift. The old private copy of the
  rule in `pbmodel.py` was removed (112 lines).
- **Reuse: one build vector.** `baseline.py` builds the change by `refbuild.build_server`, the function that builds the
  reference, instead of its own command list: both sides go through the same command vector (`-trimpath` included).
- **Reuse: one cooling and one attempt count.** `type_native.py` takes `paired.cool` and `paired.ATTEMPTS`; the native
  retry loop is the same decision table as the startup one (only an invalid run is retried).
- **Reuse: the sample sizes** are named once per language (`WARM_COUNT` and `COLD_REPLICATES`; `WarmSamples` and
  `ColdReplicates`) and every check that said `30` reads them. The Go and Python order-statistic indexes read one
  shared vector, `scripts/perf/testdata/quantile-index.json`.
- **Simplification: no verdict by hand.** The old `start_verdict` (absolute limit) is gone; `start_record` only
  assembles what `run_criterion` returned.
- **Altitude: an environment fault is one type.** `EnvironmentFault` (kind, message) carries the focus loss, the lost
  delivery, the failed activation and the permission dialog to the exit status 75 and `driver.json`; the Go verifier
  maps a coded fault to `invalid`. Nothing sniffs message text except the one `[fault:<code>]` tag.
- **Guard overrides.** Two mistyped entries (ids with a wrong digit, matching no finding) were removed and an existing
  entry that had been overwritten (`f22057e3`, kernel-ABI skips) was restored; one entry per real finding was added,
  each naming what changed and the test that proves it.
- **Docs.** The two `spec` criterion pointers that were missing (principles §3, coherence §5) were added as one row
  each, in the table they belong to.

## Considered, discarded

- Moving `STARTUP_MEDIAN_MS` and `STARTUP_P95_MS` above `startup_budget` in `paired.py`: they are module constants
  resolved at call time, so it is correct as is; editing the file changes its guard identity for no gain.
- Merging the Go `quantileUpperIndex` and the Python `quantile_upper_index`: two languages, one per runtime; the shared
  vector test keeps them equal.
- Caching the built change binary like the reference: it changes with every edit, the build is the same function and
  the cost is the incremental Go and Vite build.
- A generic retry helper for `paired.attempt` and `type_native.execute`: two loops of ten lines with different
  triggers (a verdict versus an exit status) and different fresh-state steps; a helper would need both as callbacks.
- `start.cjs` records per-slot load and pressure; they are diagnostics only and never enter a verdict (kept, cheap).

## Efficiency

- The signed-rank null is counted once per `n` (`functools.cache`, integers) and the Walsh averages are sorted once per
  verdict (3 240 values for 80 pairs). The model run (about one minute) stays out of `test:py` for that reason, like
  `paired_e2e_test.py`.
- The reference is built once per commit and toolchain (lock, atomic publication, six entries kept); the change is built
  once per run. Cooling waits for the load to fall instead of a fixed sleep.

---

# /simplify — block PB, amendment: per-sample interference (2026-11-12)

Scope: the diff of this amendment only: `scripts/perf/cpu.cjs` (new), `start.cjs`, `paired.py`, `baseline.py`,
`pbmodel.py`, their tests (`cpu_test.py`, `paired_test.py`, `paired_e2e_test.py`, `pbmodel_test.py`), the archive
`docs/perf/2026-11-12-start-bg/`, plan section 18 and the ADR 0038 amendment.

## Findings, applied

- **Reuse: one rule, three consumers.** `invalid_start`, `run_interfered` and the four constants live in `paired.py`;
  the plan sent to the driver (`paired.plan`) carries them, and `pbmodel.py` imports the same functions, so the
  simulated rule and the gate's rule cannot drift (the JavaScript driver reads them from the plan, never its own copy).
- **Reuse: the existing fault channel.** The new stop is one more `fault.kind` (`interference`) of the record the driver
  already writes; `observation` and `classify` map it to an invalid attempt, so the three-attempt retry and the cooling are
  the existing ones (`paired.attempt`). No second retry loop.
- **Simplification: one definition of "quiet".** The wait before a retry reuses `cpu.background` and the same bound; no
  second threshold.
- **Altitude: the measured tree is "the driver and its descendants"**, not a list of pids to track (the browser, its
  renderer, the application): nothing to enumerate and nothing the driver spawns can be missed.
- **Efficiency.** Two `/proc` scans per start, outside the timed window (the opening snapshot reads the machine counter
  last, the closing one first); a retry waits for quiet instead of burning tries inside the same burst, which keeps the
  invalid count near the number of bursts met (29 of 164 in the first real run) instead of three times that.
- **Existing tests.** The timing tests of the driver pass `bg_share_max=None`: they judge durations, and an ambient load
  above the bound would otherwise fail them for a reason that is not theirs. No test, threshold or delay was weakened;
  the new tests of the bound run against a real burner process.

## Considered, discarded

- Deriving the bound from a quiet level measured inside each run (robust to a busier host). Data from one host cannot
  validate it; filed as plan follow-up F6, not done.
- A finer observable (peak share over 50 ms sub-windows), which would separate edge overlaps of a peak from shoulders:
  the written decision is the share over the window, and the model reproduces the archive with it.
- Counting only the first try of each slot against the 20 % cap: a slot that is invalid twice is two lost starts; the
  cap counts all of them, which is the stricter reading.
- `for (;;)` instead of `do { ... } while (true)` in `start.cjs`: cosmetic, and it would re-key the guard entry.

## Mutation

The model's three mutants (no invalidation, invalidation on wall time, no 20 % cap) are rejected by
`pbmodel_test.Interference.test_every_mutant_is_rejected`. `cpu.cjs` and `start.cjs` are JavaScript, outside the gremlins
scope (`protocol/*`).

---

# /simplify — block PB, review fixes (2026-11-12)

Scope: the diff answering the independent review ("fix first"): `scripts/perf/cpu.cjs` (rewritten), `start.cjs`,
`paired.py`, `pbmodel.py`, `refbuild.py`, `native_input.py`, `testdata/slot-rule.json` (new), their tests,
`internal/nativeperf/verdict.go` and tests, `frontend/src/perf/native.ts` and test, plan sections 7, 8 and 18, the
ADR 0038 amendment.

## Findings, applied

- **Altitude: the measured set is a cgroup, not a list of processes.** The process-tree reader of `cpu.cjs`
  (`parseProcStat`, `treeCpu`, `treeDelta`, `readTable`, about 50 lines) is gone: one `cpu.stat` read of the run's
  cgroup replaces the scan of every `/proc/<pid>/stat`, and it cannot miss a detached helper or a reaped child. The
  launcher is two shell words (`echo $$ > …; exec`), not a wrapper binary.
- **Reuse: one per-start rule.** `paired.slot_step` is the decision; the model runs it (`pbmodel.driver_step` only adds
  the duration argument the wall-time mutant needs) and the driver's `slotStep` is checked against the same vector
  (`testdata/slot-rule.json`), so the model, the gate and the driver cannot drift. The driver's quiet wait reuses
  `slotStep` instead of a second comparison with the bound.
- **Root cause of the first gate run's invalid attempts (2026-11-12).** Chromium moves its browser process into its own
  systemd scope, out of the run's cgroup; instead of a Chromium switch (none is documented for it) or a tolerance on the
  share, the driver enforces the invariant of the measured set (`cpu.adopt`: every descendant of the driver is in the run's
  cgroup, moved back and recorded otherwise), and its failure is the fault `accounting`.
- **Reuse: one /proc reader.** `load1`, `pressure` and `clockTicks` moved from `start.cjs` into `cpu.cjs`; the driver
  reads nothing from `/proc` itself.
- **Reuse: one fault rule for the native verdict.** `Validate` and `Merge` go through one `validate(runs, faults)`;
  the `invalidReasons` helper and the second `Invalid(...)` call in `Merge` are gone. A reason is recorded by `fail`
  (genuine) or `missing` (absent evidence); the decision needs one boolean, not a set.
- **Simplification: the side test is one table.** `fisher_two_sided` takes the four counts and compares integer
  weights (exact, no tolerance); the driver record already holds every discarded and kept start, so no new field is
  written for it.
- **Simplification: refbuild's vector has no conditional step.** `unless_exists` and its branch are removed: both
  sides run every step.
- **Efficiency.** The cache key's `go env -json` runs in a scratch directory holding the reference's `go.mod`, so a hit
  needs no worktree checkout. The cgroup read is one small file per snapshot, outside the timed window as before.

## Considered, discarded

- `systemd-run --user --scope` per start: a D-Bus round trip inside the timed window, and outside `heavy.sh` it would
  move the application out of the memory cap. The driver's own cgroup (a user-owned scope under `heavy.sh`) is used.
- The negative-hypergeometric conditional test, exact for the retry-until-valid design: it needs the per-side number
  of kept starts fixed, which the cap breaks; Fisher's test is conservative here and the model measures its false
  alarm (7 in 30000).
- Making the side test one-sided (B discarded more): the review asks two-sided, and a side-dependent discard in either
  direction breaks the label independence the paired verdict assumes.
- An allowlist of `go env` keys for the cache key: a new key that changes the binary would be missed; the denylist
  names only location and download keys.

## Mutation

`pbmodel_test.Interference.test_every_mutant_is_rejected` rejects five mutants (no invalidation, invalidation on wall
time, process-tree accounting, no cap, no side test), each with disjoint 95 % bounds against the rule. `cpu.cjs` and
`start.cjs` are JavaScript and `scripts/perf` Python, outside the gremlins scope (`protocol/*`).
