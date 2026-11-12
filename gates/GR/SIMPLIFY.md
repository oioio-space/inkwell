# /simplify — block GR (`gate.py --resume`)

Diff reviewed: `scripts/gate.py`, `mise.toml` (test:py list), `CLAUDE.md` (one command line),
`gates/guard-overrides.json` (three entries). Lenses: reuse, simplification, efficiency, altitude; invariants of
`docs/plans/2026-11-12-gr-gate-resume.md` kept.

## Applied

- Reuse: the manifest's `versions` and the records' tool versions come from one function, `tool_versions(root)`
  (the manifest used to build the same dict inline, plus the embedded sidecars further down). One snapshot of the
  context per run serves the records and the manifest.
- Robustness: `read_record` requires the exact body fields (`RECORD_FIELDS`), so a self-consistent record with a
  missing field reads as "no intact record" instead of crashing the runner with a `KeyError`.
- Safety: `discard` only removes evidence paths inside `gates/<lot>/`; a record never names a file elsewhere to
  delete.
- `p.name` instead of `str(p.name)`.

## Considered and rejected

- Merging `keep_for_resume` into `reset_evidence(out, keep=…)`: `reset_evidence` removes whole directories
  (`rmtree`, guarded by `gate_reset_test.py`), resume must keep single files inside them; one function with two modes
  would be harder to read than two short ones.
- Using mtimes instead of SHA-256 for the evidence snapshot before and after each criterion: faster on large evidence
  folders, but a rewrite with the same mtime or a clock jump would make a changed file look intact; the digest is
  what the record promises.
- Computing the candidate digest once per run instead of per criterion: measured 0.083 s on this tree (1 700 files),
  negligible beside any criterion, and judging each criterion on the tree it actually sees is what makes
  `test_validity_checked_when_reached` hold.
- argparse for the command line: three accepted forms (`<lot>`, `--resume <lot>`, `<lot> --resume`) and
  `--base-guard`; a five-line `main` keeps exit code 2 for everything else without argparse's own messages.

## Extra mutation check (not part of the gate)

Nine hand mutants of `gate.py`, each run against `scripts/gate_resume_test.py`, all killed: environment check
removed, evidence check removed, exclusive rule removed, criterion-table check removed, runner check removed, tool
check removed, resume cleanup removed, executable bit dropped from the digest, recorded failure retried. The frozen
gate's own mutant (constant `candidate_digest`) is `test_mutant_skipping_digest_check_is_caught`.

## Activation

`gate.sh` runs the merge-base's runner: `mise run gate -- --resume <lot>` works for a branch only once GR is on
`main`; before that, the old runner exits 2 on the flag.
