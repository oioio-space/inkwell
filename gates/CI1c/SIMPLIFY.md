# CI1c - /simplify review of the block's diff (integrator, 2026-11-14)

Diff read in full: `.github/workflows/ci.yml`, `internal/instance/{runtime_unix.go,wire.go,instance_test.go}`,
`mise.toml`, `scripts/{pinned-path.sh,check-pinned-tools.sh,sec-scan.sh,lint.sh,rtkx.sh,bench.sh,perf/bench.sh}`,
`scripts/netsim/{run.sh,build-iproute2.sh}`, `gates/guard-overrides.json`, the plan and the gate.

| Finding | Decision |
|---|---|
| `wire.go:172`: gosec G115 is a false positive, `check()` bounds `len(r.Args)` to `MaxArgs` (32) before `Encode` builds anything and a test covers 33 arguments | kept as an annotation with the reason, like the four neighbouring lines; no new check (it would duplicate `check`) |
| `privateDir` now refuses a relative path itself, so the `filepath.IsAbs(xdg)` at the call site became redundant | applied: removed at the call site, one rule in one place |
| One sourced file (`pinned-path.sh`) instead of a PATH line in each script | kept: five scripts share it, and the check uses the same file, so the fix and its proof cannot drift apart |
| `check-pinned-tools.sh` runs inside `scan:code` and in `pregate` | kept: `scan:code` is also run alone by the pre-commit hook, where `pregate` does not run |
| `run.sh`: the scenario test was written twice | applied: `impaired_selected` |
| `build-iproute2.sh` builds all of iproute2 to install `tc` only | kept: upstream has no supported `tc`-only target; the build is 6 s locally and cached in CI |
| `lint.sh` keeps its `$GOPATH/bin` fallback after the pinned path | kept: it is the path without mise, and it now comes after the pinned folders |

Not applied, recorded as residuals in the plan: mise tasks and the pre-commit hook that still call `gofumpt`,
`betteralign` and `wails3` by bare name.

Secrets: none. Artefacts: none (the iproute2 build goes to a temporary folder and a cache outside the repository).
