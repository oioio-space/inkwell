# W58 - red witnesses before the freeze (2026-11-14, main 47b15ee + spec)

Each criterion's command was run once before any code of the block; it must fail for the reason its rule names.

| Criterion | Exit | Reason read in the output |
|---|---|---|
| docs | 1 | the eleven catalogue keys of the plan are absent from `fr.json`; `error.host` still carries `{reason}` |
| go | 1 (10 s) | `missing or not passing: TestStandingFromList` (the named tests do not exist yet) |
| mutant-tool | 1 | `scripts/netsim/mutants_test.py` does not exist yet (unittest: errors=1) |
| frontend | 1 (5 s) | Vitest exit 0, zero failed, every named « W58 … » test missing: the analyser refuses a run without them |
| ux-walkthrough | 1 | `gates/W58/UX.md` absent |
| simplify-report | 1 | `gates/W58/SIMPLIFY.md` absent |
| mutants, e2e, regression | not run | they need the block's entries (`MW1`..`MW7`), `e2e/foreign-project.spec.ts` and a built candidate; `regression` is green by construction before the change. Their duration is measured at the smoke, on an idle machine, and the gate is re-cut (never weakened) if the whole exceeds 45 min |
