# SB2a1 - red witnesses before the freeze (2026-11-14, main ceefcff + the frozen specification)

Each criterion's command was run once before any code of the block, on this machine. The four constraints added at the
freeze (S-8 fonts N43, S-9 protected projects N37, S-10 program descriptors for plugins, S-11 no desktop OOM alarm W73)
changed the `docs` and `mutants` commands: both were run again afterwards (results below are the second runs).

| Criterion | Exit | Reason read in the output |
|---|---|---|
| docs | 1 | the catalogue keys `about.isolation.reduced`, `preview.fail.sandbox`, `preview.fail.sandbox.details`, `preview.fail.protected` are absent from `fr.json` and `en.json`; `about.network.proxy` is still there; ADR 0044 is not accepted. Probe run 4 is recorded and every rule id has its plan row (those checks pass) |
| go | 2 | the test run completed (internal/language, internal/typst, internal/sidecar/fetch pass; internal/sidecar failed on `TestResidentBudgetKillsWithSIGKILL/{resident,resident-tree}` and `TestCgroupSessionSharesSliceBound`, run while the pre-push CI held the other heavy slot — load-sensitive cgroup tests, pre-existing, outside this witness); then `scripts/gojson_check.py` does not exist (« can't open file »): the checker and the inventory's named tests are the block's work |
| mutants | 1 | entries MS1–MS19 of block `SB2a1` are absent from `scripts/netsim/mutants.toml` |
| frontend | 1 | Vitest ran (exit 0, 0 failed); the three named tests « SB2a1 about shows the tier: namespaces », « …: reduced », « SB2a1 a refused sandbox shows its sentence and Détails » are missing. A first run had timed out (exit 1 after 600 s) waiting for a heavy slot behind the pre-push CI; this row is the run on a free machine |
| checkers | 1 | `scripts/gh_verdict_test.py` and `scripts/gojson_check_test.py` do not exist (unittest: « ModuleNotFoundError: No module named scripts/gh_verdict_test », 2 errors) |
| github | 2 | `scripts/gh-verdict.py` does not exist (« can't open file »); `gates/SB2a1/github.json` does not exist either |
| regression | 0 | green by construction before the change (no-regression guard): `e2e/preview.spec.ts` 9 passed on a free machine; a first run had timed out (124) waiting for a slot |
| ux-walkthrough | 1 | `gates/SB2a1/UX.md` absent |
| simplify-report | 1 | `gates/SB2a1/SIMPLIFY.md` absent |

The global cap (45 min) and the smoke are measured on the candidate, before acceptance (gate header); the estimate is
43 min (plan, « Durations of SB2a1 »).
