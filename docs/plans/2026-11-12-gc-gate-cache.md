# Block GC (tooling): leaf-only session memoisation for frozen gates — measured, then deferred behind GP

Date: 2026-11-12, third revision, after two adversarial reviews by Codex. Rigour: **complete** (it decides which
criteria run). Parked gate: the session scratchpad's `gc/GC.toml`, kept out of `gates/` until it is frozen in one
commit with failing skeletons of `scripts/gate_memo.py` and `scripts/gate_memo_test.py`, so that L1's
`gate-guard.py --self-test` stays green. Decision record to write if GC is built: `docs/adr/0043-gate-leaf-memo.md`.

## 0. Outcome

| Question | Answer |
|---|---|
| What stage 1 is | **Leaf-only session memoisation.** Nested gates keep running as they do today (shell, `mise run gate -- X`), and **every gate occurrence is visited again**. Only leaf criteria classified *deterministic and side-effect-contained* may be memoised, and an attachment requires the whole **state digest** to be unchanged since the end of the first run |
| Honest saving on the recorded closures | **≈ 0 min** for RL1, C5S and RC1 (section 5). Every repeat of a memoisable leaf (`mise run tla` repeats 13 times in one RC1 run) is separated from its first run by a criterion that writes checkout state, such as netsim results, the vitest cache, perf data or builds |
| Potential once criteria are contained | ≈ 105–130 min per RC1 run, 32–40 min per C5S run, 8–10 min per RL1 chain (the `mise run tla` repeats), **if** criteria write only to runner-owned evidence or outside the checkout, and builds are reproducible |
| Recommendation | **Do GP first** (gate profiling and containment): it attacks C5S's e2e-C5 (164.9 min) and mutants (60.6 min) directly, and its containment step is what makes GC pay off. Build GC after GP, with this gate. Cheap meanwhile: new gates name only the maximal gates of their regression closure (RC1 visits RL1 five times) |
| Persistent cache | Stage 2, only if logs measured after GC justify it. Any key would drop HEAD and the checkout path when the content is identical |

## 1. Measured problem (recorded `duration_seconds`, perf = `exclusive = true`)

RL1 chain = 51.2 min (perf 24.5), measured from:

- L0: 19.1 min, of which perf 18.8;
- L1 1.9, L2 0.3;
- L3: 5.8, of which perf 5.7;
- P2 3.5;
- P3 10.5, of which P-TLA = `mise run tla` 10.0;
- RL1 10.1, of which model-RepoGen = `mise run tla` 5.3.

SP1b's own criteria take 35.5 min. C5S's own take 248.2 min, of which **e2e-C5 164.9** and **mutants 60.6**.
`mise run tla` with these exact bytes is the command of 14 criteria in one RC1 run's closure:

- RL1 and P3 in each of the 5 RL1 visits (RL1 directly, and via WB, SP1b, C5S, and C5S→SP1b): 10;
- P3 via NI0: 1;
- SP1b ×2: 2;
- WB: 1.

C5S's own `timeout 900 mise run tla` has different bytes, so it is a different trial. The latest durations of
`mise run tla` are 8.1–10.0 min.

## 2. Prior art (kept from the previous revision, condensed)

How each paradigm is judged:

- **Sound vs undeclared inputs**: does a read nobody declared still invalidate the result?
- **Cost here**: what adopting it would take in this repository.
- **Fit with frozen gates**: criteria are arbitrary shell commands, frozen, never rewritten into build rules.

| Paradigm | Sound vs undeclared inputs | Cost here | Fit with frozen gates |
|---|---|---|---|
| Hermetic action cache (Bazel, Buck2 under remote execution) | Only under an enforcing sandbox ([cache hit](https://bazel.virtuslab.com/book/2~4~3/), [Buck2 RE](https://buck2.build/docs/users/remote_execution)) | Very high | Poor |
| Input-addressed derivations (Nix) | Only for pure builds ([Nix](https://nix.dev/manual/nix/2.33/store/derivation/outputs/input-address)) | Very high | Poor |
| Task-graph hashing (Turborepo, Nx) | No: undeclared env vars and inputs give stale hits ([turbo](https://turborepo.com/docs/reference/configuration), [env](https://computingforgeeks.com/turborepo-cache-misses-environment-variables/)) | Low | Poor |
| Runtime-recorded reads (Go testlog, Buck2 dep files, redo) | Partial: Go uses mtime, ignores files outside the module, and goes stale on edits during the run ([cmd/go](https://pkg.go.dev/cmd/go#hdr-Test_packages), [#26790](https://golang.org/issue/26790), [dep files](https://buck2.build/docs/rule_authors/dep_files), [redo](https://redo.readthedocs.io/en/latest/)) | Low for Go only | Partial |
| Tracing (tup, Shake fsatrace and rattle, strace) | Only if the trace is complete ([fsatrace](https://github.com/jacereda/fsatrace), [tup](https://gittup.org/blog/2014/05/7-clobber-builds-part-2---fixing-missing-dependencies), [GHC #16400](https://gitlab.haskell.org/ghc/ghc/issues/16400)) | Medium to high | Good |
| **Session memoisation with a whole-state check** (each node once per build, as in Make, Shake, or Bazel within one build) | Yes, if reuse requires the **entire** readable state to be unchanged; no input declaration needed | Low | Good |

Recommendation: the last row, restricted to leaves (Codex's increment). The declared walk
(`gate-guard.dependencies()`) is used only to **classify** leaves. It never decides an attachment.

## 3. The rule

**Leaf classes**. The class comes from the criterion's command and its dependency closure (mise tasks closed over
`depends`, scripts naming scripts):

| Class | Examples | Memoised |
|---|---|---|
| contained-deterministic | `mise run tla`, `mise run alloy`, `mise run arch`, `bash scripts/arch-check.sh`, `mise run lint`, `python3 scripts/check-no-file-query.py`, spec checks made only of `test`/`grep`/`sha256sum -c` | **yes** |
| test-runner | `go test` (any flags), vitest, Python unittest, Playwright | no: scheduling-dependent and often state-writing; seeded is not deterministic under real scheduling |
| stochastic | `-race`, `-fuzz`, `-count=` with N > 1, `--repeat-each`, `-shuffle`, unseeded | no |
| netsim / e2e / mutation | `mise run e2e:*`, `netsim:mutants`, gremlins, mutant loops | no |
| perf | `exclusive = true`, or `heavy.sh --exclusive` in the closure | no |
| external | network-reading tools (`govulncheck`, `grype`, `curl`, `wget`, `gh `, `pnpm install` without `--offline`) | no |
| nested gate | closure invokes the gate runner | never as a criterion; the nested gate is visited again and decides for its own leaves |

The allowlist lives in `scripts/gate_memo.py` (gate machinery, guarded) and is frozen in the gate
(`[policy] memoised_commands`). A command matches only **byte for byte**. Anything not listed is not memoised.

**Trial identity** = SHA-256 of canonical JSON {`command` bytes, cwd relative to the root, execution environment}.

- The execution environment is the full environment minus a frozen **metadata set**: `INKWELL_GATE_CRITERION`,
  `INKWELL_PERF_OUT`, `INKWELL_GATE_SESSION`, `INKWELL_GATE_MEMO`, `SHLVL`, `PWD`, `OLDPWD`, `_`, `__MISE_*`.
- `PATH` is deduplicated, keeping each first occurrence.
- A leaf is memoisable only if its dependency closure names none of the metadata variables, checked statically.
- A command repeated within one gate is an explicit repetition and is never attached.

**State digest**. The digest covers every path under the checkout root, except `.git/` and the exact set `R` that
the **runner itself** writes:

- `gates/<lot>.json` for each lot visited;
- `gates/<lot>/<criterion>.log`;
- `gates/<lot>/base-guard.log`;
- `gates/<lot>/memo.json`;
- the session's timeline file.

Each path enters as kind, mode, content SHA-256 or link target. `.gitignore` is irrelevant to the sets: tracked,
untracked and ignored files all count. Files that criteria write into `gates/<lot>/` (measurements, Playwright
results, `exclusive.jsonl`) are state, not `R`.

**Attachment**. At a later occurrence of a memoised trial, it attaches only if all hold:

- the trial passed at its first run, in this session;
- the current state digest equals the digest taken at the end of that run;
- no symlink under the root points outside it or is absolute (checked once per session; otherwise memoisation is
  off for the session);
- the mode is `on`.

Then its **full evidence bundle** is copied: every file the first run created or modified outside `R`, plus its log,
with their SHA-256 values. A failed first run is never attached as a pass and never retried to find a green; later
occurrences run again, and any failure stands.

**Session**. The outermost runner creates `INKWELL_GATE_SESSION` and the record (owner pid, its start time,
`boot_id`) in `$XDG_RUNTIME_DIR/inkwell-gate/<session>/` (tmpfs). It removes it in its `finally`. A nested run
accepts the session only if the owner is a live ancestor with the same start time.

**Escape hatch**. `INKWELL_GATE_MEMO=on|off`; `off` is today's runner exactly. Any other value exits with code 2. The
mode is inherited by nested gates.

## 4. Codex's findings, one by one

| # | Finding | Status under the leaf-only rule | Proof (gate criterion) |
|---|---|---|---|
| 1 Critical | Reusing a nested gate skips its independent trials | **Fixed.** No gate-level reuse; every occurrence is visited again; stochastic, network, mutation, netsim, perf, repeats and test runners are never memoised | `nesting`: diamond DAG with stochastic descendants |
| 2 High | Attachment checked earlier writes, not current reads | **Fixed.** Attachment needs the whole state digest unchanged since the first run, whatever was written or read | `state`: read-after-replace fixture |
| 3 High | The digest included runner-written tracked files | **Fixed.** The runner-owned set `R` is listed exactly and the digest is independent of `.gitignore`. Criterion-written files in `gates/<lot>/` stay in the digest | `state`: runner-written manifest fixture |
| 4 High | `INKWELL_GATE_CRITERION` in the identity prevents merging | **Fixed.** Identity uses the execution environment minus the frozen metadata set; a leaf whose closure reads a metadata variable is not memoisable | `identity`: criterion-id env fixture |
| 5 High | External symlink targets are not frozen | **Fixed** by prohibition: any external or absolute symlink turns memoisation off for the session | `symlink` |
| 6 High | gate.sh protocol selection: a candidate could pick its own machinery | **Fixed.** The `gate` mise task runs the **merge-base's** `gate.sh` (`git show <base>:scripts/gate.sh`), so the selection logic is the base's. A frozen selection matrix (base protocol × base module present) decides the runner and the memo mode. Adversarial nested tests: a working-tree `gate_memo.py`, `gate.sh` or runner environment variable never changes what runs | `protocol` |
| 7 High | Attached perf evidence needs the full bundle | **Not applicable to perf**: perf is never memoised, and each occurrence measures again under `heavy.sh --exclusive`. The bundle rule is kept for leaves: every created or modified file outside `R`, not only the log | `bundle`: perf-bundle fixture (perf runs again) and leaf multi-file bundle |
| 8 Medium | Criteria did not falsify the boundaries; the count was wrong | **Fixed.** One criterion per boundary with named fixtures. The previous draft had 18 criteria (17 new + regression), not 16. This one has **17: 16 new + regression** | all |

## 5. The honest saving, measured with this rule

The method: walk each recorded closure in the runner's order (depth-first, file order). Mark as **state-writing**
every criterion whose class writes checkout state. The evidence is in the c5s-wt worktree during its running chain,
2026-11-12, files newer than the gate files:

- `testdata/perf/generated/` (perf);
- `frontend/test-results/.vitest-cache` and `.vite/vitest` (vitest);
- `frontend/perf-dist/`;
- `bin/` and `frontend/dist/` (`e2e:build`);
- `.task/netsim/results/` (netsim);
- TLC traces.

A memoised trial attaches only if no state-writing criterion ran since its first run's end.

| Closure | Memoisable repeats | Attachable under the rule | Saving |
|---|---|---|---|
| RL1 chain | `mise run tla` ×2 (RL1, P3) | 0: `netsim:mutants`, NI-2 and netsim run in between | **0 min** |
| C5S run | `mise run tla` ×5 (RL1 chain ×2, SP1b) | 0: netsim, e2e and vitest run between every pair | **0 min** |
| RC1 run | `mise run tla` ×14, `mise run arch` ×2 | 0: each repeat follows netsim, vitest or perf | **0 min** |

**If containment came first** (GP: criteria write their results only to runner-owned evidence or outside the
checkout, the vitest cache moves out, builds are reproducible), the same rule would attach 13 `mise run tla` runs per
RC1 run, ≈ 105–130 min, 4 per C5S run, ≈ 32–40 min, and 1 per RL1 chain, ≈ 8–10 min. Relaxing the whole-state check
instead would need the trial's read set: tracing (GC2) or declarations, which section 2 rejects.

## 6. The next lever: GP (gate profiling and containment)

1. **Profile inside the long trials**: e2e-C5 (164.9 min, `NETSIM_REPEAT=10` × 3 seeds), mutants (60.6 min, 11
   mutants × netsim), SP1b `netsim-regression` (24.5 min). Split each into setup (browser and app start), rebuilds
   (`e2e:build` repeated by every `mise run e2e:netsim*` through `depends`), and scenario time, from the netsim
   runner's per-scenario logs, the Playwright JSON reporter, and per-mutant timings. Measure, then remove duplicated
   setup and rebuilds within one criterion; a build is reused only if it is byte-identical.
2. **Containment**: each criterion writes results into its runner-owned evidence directory or outside the checkout.
   The vitest cache, `.task/netsim/results` and the perf data move. This is also what makes GC worth doing.
3. **Shared caches across worktrees**: one GOCACHE, GOMODCACHE, pnpm store and Playwright browser cache for every
   worktree; mutable outputs (`bin/`, `frontend/dist/`, test results, `gates/<lot>/`) stay per worktree.
4. **Gate hygiene, for new gates only** (frozen gates do not change): a regression criterion names only the maximal
   gates of its closure. The `new-gate` skill and a check in `gate-guard.py`'s self-test refuse a named gate that is
   already in another named gate's closure. RC1 would visit RL1 3 times, not 5.

## 7. If GC is built after GP: steps

1. Skeletons, ADR 0043; freeze the parked gate.
2. Classifier and the allowlist (`leaf`).
3. Trial identity (`identity`).
4. State digest and `R` (`state`, `symlink`).
5. Session, attachment, bundle, failure (`bundle`, `failure`, `session`).
6. The `gate` task runs the base's `gate.sh`, and the selection matrix (`protocol`).
7. Model and mutants (`model`, `mutants`, `nesting`); `integration`; `saving` re-measures section 5 from the frozen
   closure snapshot `scripts/testdata/gate_memo/closures-2026-11-12.json` (durations and classes), so its figures
   cannot drift.
8. `/simplify`, then `mise run done -- GC`.

## Sources

- [Bazel cache hit](https://bazel.virtuslab.com/book/2~4~3/), [Bazel hermeticity](https://bazel.build/basics/hermeticity),
  [Bazel test caching](https://bazel.virtuslab.com/book/1~2~1/), [test tags](https://docs.aspect.build/cli/help/tags).
- Buck2: [RE](https://buck2.build/docs/users/remote_execution), [dep files](https://buck2.build/docs/rule_authors/dep_files).
- [Nix input-addressing](https://nix.dev/manual/nix/2.33/store/derivation/outputs/input-address).
- [turbo.json](https://turborepo.com/docs/reference/configuration),
  [Turborepo env misses](https://computingforgeeks.com/turborepo-cache-misses-environment-variables/).
- Go: [cmd/go](https://pkg.go.dev/cmd/go#hdr-Test_packages),
  [test.go](https://github.com/golang/go/blob/master/src/cmd/go/internal/test/test.go),
  [#26790](https://golang.org/issue/26790).
- [fsatrace](https://github.com/jacereda/fsatrace),
  [tup](https://gittup.org/blog/2014/05/7-clobber-builds-part-2---fixing-missing-dependencies),
  [GHC #16400](https://gitlab.haskell.org/ghc/ghc/issues/16400), [redo](https://redo.readthedocs.io/en/latest/).
- In repository: `scripts/gate.py`, `scripts/gate.sh`, `scripts/gate-guard.py`, `gates/*.toml`, `gates/*.json`,
  `docs/adr/0038-paired-performance.md`.
