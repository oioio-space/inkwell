# OBS1 - red witnesses

Recorded 2026-11-24/25 at step 1, before any production change, in the worktree `inkwell-obs1` (branch `obs1` from
`main` 525dc3c, fast-forwarded to ae99621 then ae37c44) holding only the plan, this draft gate and its docs. Round 1 is
the first draft; round 2 follows the blind Opus review (`REVIEW.md`): every criterion whose command changed was run
again on ae37c44; `bench`, `perf-start` and `simplify-report` are unchanged since round 1. Each command was run as written in
`gates/OBS1.toml`, from the worktree root, through `sh -c` as `scripts/gate.py` does. Logs are in the session
scratchpad, `witness-OBS1-<id>.log`. Exclusive criteria ran only when no other `heavy.sh --exclusive` job was
on the machine (checked with `pgrep`).

| Criterion | Round | Exit | Time | Output line read | Right reason? |
|---|---|---|---|---|---|
| docs | 2 | 1 | < 1 s | `OBS1 docs: ['ADR 0027 does not name the `metrics` point and the OBS1 plan']`; the plan's 22 rule rows, spec O-4, W86 and the OBS2 line found | yes (the ADR amendment is written at step 2) |
| frontend | 2 | 1 | 314 s | Biome clean, svelte-check `800 FILES 0 ERRORS 0 WARNINGS`, Vitest 1435 passed, 0 failed, 1 todo; `OBS1 frontend: exit 1, rule tests {… the 9 names (M-1..M-3, V-1, T-1, T-2, P-3, Z-2, Z-4) 'absent'}, failed []` | yes (round 1: 310 s; cap 420 s) |
| go | 2 | 1 | 73 s | `OBS1 go: exit 1, named {... all 13 'absent'}, failed ['./internal/obs/....None']`: `./internal/obs/...` does not exist; `internal/app` and `internal/transport/...` were built and run | yes (cap 180 s; the new tests add little) |
| privacy | 2 | 1 | < 1 s | seeds drawn and written (`seeds.json`: text, keystrokes, file name, path, 28-char document id, peer id, count 829511); then `stat …/internal/obs: directory not found` / `FAIL ./internal/obs [setup failed]`; the scan is not reached | yes (no package, no seeded session). `forms()` self-tested apart: a seed embedded at any offset in base64 (standard and URL), UTF-16 LE/BE and percent-encoded text is found in 300 of 300 random cases |
| local-only | 2 | 1 | < 1 s | `go list ./internal/obs/... failed: … no such file or directory`, `frontend/src/obs has no module`, `frontend/src/ext/activity has no module` | yes. Checked: `go list -deps ./internal/store` reaches no network or inkwell package; the import scanner reads type-only, named, multi-line, side-effect, dynamic, re-export and namespace imports correctly on a synthetic file, and `editor/setup.ts` (the planned importer) holds only a type import of Automerge |
| mutants | 2 | 1 | < 1 s | `OBS1 mutants: no named mutant for rules ['M-1', 'M-2', 'M-3', 'P-3', 'T-1', 'T-2', 'V-1', 'Z-2', 'Z-4', 'go P-1' … 'go Z-5']`: the 9 frontend titles are read from the `frontend` criterion itself; the runner is not reached | yes. `mutants.py --check`: 0 problem(s) with this gate naming block OBS1 |
| bench | 1 | 0 | 866 s | `internal/blob`, `internal/transport/frame`, `internal/transport/mux`, `internal/app`: each `informational pass`; `regression-guard passed` | green by design: a regression guard on an unchanged tree, every package informational (nothing touched). `frame` dominates (10 benchmarks, 200 samples per side) |
| perf-start | 1 | 0 | 117 s | PERF-START `pass`: paired against the merge-base ae99621 (PF2 driver), 80 pairs, ratio upper bound 1.0043 < 1.10, budget `pass` | green by design: a regression guard on an unchanged tree (no other exclusive job, waited 0 s; cap 300 s) |
| simplify-report | 1 | 1 | < 1 s | `OBS1 /simplify review evidence is absent` | yes |

## Wrong-reason runs, corrected

- `frontend` and `perf-start`, first run: exit 1 in 0 s. `mise ERROR No version is set for shim: biome`, and
  `scripts/perf/automerge.cjs` failed to build its corpus: the new worktree had no `frontend/node_modules` and mise
  had not installed biome. Second `frontend` run (23 s): svelte-check could not find
  `frontend/bindings/…/internal/app/index`, because the Wails bindings had never been generated. That is setup, not the
  rule: `mise install`, `pnpm install --frozen-lockfile` and `mise run fe:bindings`; then the worktree was
  fast-forwarded to `main` ae99621 (PF2) and both were run again (rows above). `scripts/worktree.sh` does none of these
  three steps; fixed on `main` by ae37c44 (CI3: a new worktree gets its tools, frontend packages and Wails bindings).

## Caps (rule G-1)

`bench` measured 866 s with every package informational; a confirmation session (run only on a significant worsening)
re-measures the package, so its cap is 1200 s, and a timeout fails closed. The other caps come down to fit:
`frontend` 420 s (EX0 measured 308 s), `mutants` 300 s, `perf-start` 300 s (EX0 measured 126 s and 138 s).
Round 2 adds two 10 s and 30 s steps to `privacy` (seed draw, scan): 30 + 420 + 180 + (10 + 120 + 30) + 30 + (20 + 300)
+ 1200 + 300 + 30 = 2670 s <= 2700 s; `deadline_problem` returns `None`.
