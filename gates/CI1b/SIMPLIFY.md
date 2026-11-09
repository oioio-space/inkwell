# /simplify — block CI1b (netsim only if touched at pre-push, one netsim run on GitHub) (2026-11-09)

Scope: the whole diff of the block — `scripts/netsim/affected.py` and `affected_test.py` (new), `.githooks/pre-push`,
`.github/workflows/ci.yml`, `mise.toml` (`ci:core`, `ci`, `test:py`), `.env.reference` (the selector's test-only `INKWELL_AFFECTED_GOLIST`, found by `lint:env`), `docs/plans/2026-11-08-ci-cd-study.md`. Review by
the main session (reuse, simplification, efficiency, altitude), against the frozen gate `gates/CI1b.toml` and the
study's R2/R3.

## Applied

- Altitude/efficiency: the implementer's hook ran `ci:core` and only then the netsim, serially, adding ~17 min to a
  push that touches the transport (before, `mise run ci` ran the netsim in parallel with the other checks). The hook
  now decides first (0.4 s) and runs `mise run ci` (parallel, unchanged) or `mise run ci:core`. The selector reads the
  hook's stdin itself, so the hook no longer buffers it.
- Simplification: an unused `INPUT_PREFIXES` constant removed (the prefixes are tested inline in `is_input`).

## Declined

- Short-circuiting `go list` for non-Go diffs: a decision costs 0.4 s, measured; not worth a second code path.
- A receipt cache (study R8): a separate block.

## Kept on purpose

- Every failure runs the netsim (unresolvable base, `go list` failure, malformed stdin, crash, missing python): the
  hook treats any exit other than 10 as run.
- A non-test `.go` file in a dependency's directory but excluded by build tags runs the netsim (a tag change can bring
  it in).
- Every package under the repository in `go list -deps` counts, not only the main module's, so the nested `protocol`
  module is covered.

## Evidence

- Gate criteria rule-CI1-unit (28 tests), rule-CI1-real-graph, rule-CI1-wiring, tooling-checks: pass.
- The selector on the CI1b gate commit (pre-push stdin, HEAD~1..HEAD): `netsim: skip (inert: 1 paths)`, exit 10.

## Mutation

No `protocol/*` package is touched; gremlins step 5 does not apply. The selector's rules are each pinned by a named
unit test (one per case of the gate).
