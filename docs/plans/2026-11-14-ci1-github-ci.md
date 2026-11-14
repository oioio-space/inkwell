# CI1c: the GitHub `ci` workflow, second repair (tooling, rigour light)

Date 2026-11-14. Gate: `gates/CI1c.toml` (the name CI1 is taken by the merged block of 2026-11-08, frozen).

## Facts

- The GitHub workflow `ci` has been red since 2026-10-25 while the local pre-push run passed. Commit a479283
  repaired the first three causes; run 43624790761 then showed the next failures: `scan:code` (job `ci`) and the
  netsim step "Impaired network" (scenarios NI-1, NI-2, NI-3). The arm64 job is another block (SB2).
- gosec 2.30.0 (pinned in `mise.toml`) reports G115 at `internal/instance/wire.go:172` and G703 at
  `runtime_unix.go:37`. The local run missed them: under `mise run` / `mise x --` the pinned tool folders are not
  placed before folders already on PATH, and a stale `go install` build in `~/go/bin` shadowed them. Measured:
  `which -a gosec` lists `~/go/bin/gosec` first; `go version -m` gives module v2.29.0 for it and v2.30.0 for the
  pinned one; both print `Version: dev`, so `gosec -version` cannot tell them apart. govulncheck, gofumpt,
  benchstat and wails3 are shadowed the same way (govulncheck: scanner 1.8.0 against pinned 1.9.0).
- `topology.sh` passes `netem ... seed N`. The iproute2 change "tc: support the netem seed parameter" is commit
  a67cf9a2523c (2023-08-28); `tc/q_netem.c` has no `"seed"` at tag v6.5.0 and has it at v6.6.0
  (git.kernel.org, iproute2.git, `plain/tc/q_netem.c?h=<tag>`). Ubuntu 24.04 ships iproute2 6.1.0-1ubuntu6
  (packages.ubuntu.com/noble/iproute2). The developer machine has 7.0.0.

## Decisions

1. `scripts/pinned-path.sh` (puts `mise bin-paths` first) is sourced by `sec-scan.sh`, `lint.sh`, `rtkx.sh`,
   `bench.sh`, `perf/bench.sh`.
2. `scripts/check-pinned-tools.sh` fails when a gate tool resolves elsewhere than `mise which` says, or when gosec's
   embedded module version differs from `mise.toml`; run by `scan:code` and by `pregate` (task `lint:pinned`).
3. The netsim job builds iproute2 6.12.0 from the tarball pinned by SHA-256 (`scripts/netsim/build-iproute2.sh`,
   through `fetch_pinned.py`; the hash equals kernel.org's `sha256sums.asc`), cached by `actions/cache`, and puts its
   `tc` first on PATH. `run.sh` probes `netem ... seed` once and fails with one sentence.
4. G115: `check()` already bounds `len(r.Args)` by MaxArgs (32) before the encoder; annotated, no new check. G703:
   `privateDir` now requires an absolute path (a relative `$TMPDIR` made the socket depend on the working
   directory; test `TestSocketPathFallback`), and carries `#nosec G703` with the reason.

## Set aside

- Deleting the stale tools in `~/go/bin` (the user's home is not ours to clean; the fix must hold anyway).
- Dropping `seed` (it makes the draws replayable, plan C5S harness 1).
- apt backports (no iproute2 6.6+ for noble; a pinned, hash-checked build is reproducible).

## Not in this block

- Job `sidecars-arm64`: block SB2.
- Residual: mise tasks that still resolve Go-installed tools by bare name (`fmt` gofumpt, `betteralign*`, `wails3`
  tasks) and the pre-commit hook. One-line fix: run them as `./scripts/rtkx.sh <tool> ...` (it sources
  `pinned-path.sh`) or add `source scripts/pinned-path.sh` to the hook.
- govulncheck lists GO-2026-5932 (x/crypto openpgp, v0.58.0), not called by the code; to be decided separately.

## To confirm on GitHub

The netsim job builds iproute2 (cold cache, then cache hit), `tc -V` says 6.12.0, NI-1 to NI-3 pass; `scan:code`
is clean in job `ci`; the final results are recorded in `gates/CI1c/GITHUB.md` by the integrator after the push.
