# CI1c witnesses (2026-11-14)

Light gate written after the code (see the header of `gates/CI1c.toml`). Exit codes are from one run of each
criterion's command on the final tree (`python3 -I` runner over `gates/CI1c.toml`); `simplify-report` is the
integrator's.

| criterion | exit now |
|---|---|
| scan-code | 0 |
| pinned-tools | 0 |
| pinned-tools-red-without-path-fix | 0 (the command expects the failure of the bypassed check, and saw it) |
| instance-tests | 0 (includes `--- PASS: TestSocketPathFallback`, the absolute-path case) |
| seed-probe | 0 (fake tc, iproute2-6.1.0, refused with "does not accept 'netem ... seed'") |
| lint, lint-sh, lint-yaml, lint-actions | 0, 0, 0, 0 |
| simplify-report | not run (file written by the integrator) |

## Red states observed

- scan-code, before: gosec 2.30.0 (the pinned binary, `mise which gosec`) run on an archive of HEAD (`git archive HEAD`,
  the tree before the fix) exited 1 with `wire.go:172 G115 int -> byte` and `runtime_unix.go:37 G703 os.Lstat`. The
  stale `~/go/bin/gosec` (module v2.29.0) on the same archive exited 0 with no finding: that is how the local run
  stayed green.
- pinned-tools, before: the check with the sourcing line deleted (the criterion `pinned-tools-red-without-path-fix`
  does this on the fly) exited 1 with `gosec: runs /home/mathieu/go/bin/gosec, mise pins .../2.30.0/bin/gosec`,
  the same for govulncheck, and `gosec: version 2.29.0, mise.toml pins 2.30.0` (measured during the block).
- instance absolute-path rule, before: `TestSocketPathFallback` with the new case failed first
  (`relative temporary folder used: "rel/inkwell-1000"`), measured during the block, before `privateDir` was changed.
- seed-probe, before: not observed as red on the old `run.sh` (the probe did not exist); the failure of the old
  behaviour is the GitHub log ("What is "seed"?", run 43624790761), not reproduced here.
- Not shown here, to confirm on GitHub: the netsim job with the built iproute2, and scan:code in job `ci`.
