# CI1c - confirmation on GitHub

| Run | Commit | `ci` | `netsim` | `sidecars-arm64` |
|---|---|---|---|---|
| 43620631116 | 04a2782 (before) | `arch`: `frontend/dist` missing | kernel module package not found | sandbox refused (user namespaces) |
| 43624790761 | a479283 | `scan:code`: two gosec findings | `What is "seed"?` (iproute2 6.1.0) | about 30 tests: typst and tinymist die, cgroup, `setrlimit` |
| 43629366160 | 5eeb32d (CI1c) | `scan:code` **passes**; `test:ci` fails in `internal/sidecar` and `internal/typst` (below) | `tc` accepts `seed`; scenario 5 fails on `No distribution data for normal (/usr/lib/tc//normal.dist…)` | unchanged |

What CI1c set out to repair is repaired on GitHub: `scan:code` is clean with the pinned gosec, and the netsim job's
`tc` knows `seed`. Each repair uncovered the next failure, which had never been reached:

- `netsim`: the built `tc` had no distribution tables. Follow-up in the same block: `build-iproute2.sh` installs
  the `*.dist` files and the job exports `TC_LIB_DIR` (checked locally: `netem … distribution normal seed 1` is
  accepted by the built `tc`). To confirm on the next run.
- `ci` → `test:ci`: `TestCgroupSessionScopeDelegated`, `TestCgroupRunLimitsVerifiedBeforeStart` (no delegated user
  scope on the runner), `TestRunNoNetworkFallsBack` and `TestCgroupWorkloadAndDescendantsInside` (the package times
  out after 8 min), `TestHostileCgroupMemoryIsMemoryLimit`, `TestHostileMemoryDeathStartsNoCoreHelper`. These are
  the sandbox on a machine that is not the developer's: block **SB2**, not CI1c. The `ci` job stays red until then.
- `sidecars-arm64`: real arm64 defects, block **SB2**.
