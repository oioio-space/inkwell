# Block GR — `gate.py --resume`, per criterion, on the same candidate

Date: 2026-11-12. Rigour: standard (tooling; the runner judges every later block, so every input class has a test
and the digest check has a mutant). Decision: `docs/autonomous-decisions.md`, row « process (after Codex) », item 4.
Frozen gate: `gates/GR.toml`; tests: `scripts/gate_resume_test.py`.

## Need

A full gate runs for hours; an interruption (crash, OOM of one step, reboot) today costs the whole run again.
`--resume` re-uses every criterion already finished **on the same candidate**, and nothing else.

## Rule

- Usage: `gate.py [--resume] <lot>` (flag before or after the lot), `gate.py --base-guard` unchanged; any other
  argument list exits 2. `mise run gate -- --resume <lot>` passes the flag through `gate.sh` (`"$@"`, unchanged).
- **Records, always written.** Every run, with or without the flag, writes one record per finished criterion,
  atomically (temporary file + `os.replace`) **after** its command exits: `gates/<lot>/resume/<id>.json`, body
  plus `sha256` of the canonical JSON body (`sort_keys`, compact separators). Body: `lot`, `id`, the criterion's
  TOML table, the manifest entry (verdict, exit code, duration, log), the evidence (`{path: sha256}`: the log and
  every file of `gates/<lot>/` the criterion created or changed, `resume/`, `*.md` and `exclusive.jsonl` excepted),
  the candidate digest taken just before the command, the runner identity (SHA-256 of the running `gate.py` and of
  its `perf/archive.py`), the tool versions (the manifest's `versions`: go, node, typst, tinymist, os, embedded
  sidecars) and the environment (every `INKWELL_*` variable except `INKWELL_GATE_RUNNER`, which names the merge-base
  and not the runner's content). No record is written when the digest cannot be computed (not a git work tree).
- **Candidate digest** (`candidate_digest(root, lot)`): SHA-256 over the sorted `(path, mode, content SHA-256)` of
  `git ls-files --cached --others --exclude-standard` (a deleted tracked file and a symlink's target count), excluding
  the gate's own outputs only: `gates/<lot>.json` and the files of `gates/<lot>/` that `reset_evidence` clears. The
  block's versioned reports (`gates/<lot>/*.md`, e.g. SIMPLIFY.md) **are** inputs: `simplify-report` reads one.
  Ignored files (`node_modules`, build outputs, `.typstdeps/`) are not in the digest; they are pinned elsewhere
  (sidecar SHA-256, lock files) or covered by the tool versions — a stated residual, as in the decision.
- **Without `--resume`**: unchanged — evidence reset, every criterion runs, same verdicts; the only addition is the
  record files (listed in the manifest's `sha256` like all evidence). The manifest has no `resume` key.
- **With `--resume`**: no `reset_evidence`; instead every file of `gates/<lot>/` is removed except the `*.md`
  reports, the intact records and their evidence (stale evidence is never hashed); `exclusive.jsonl` is removed.
  The base guard and the deferral checks always run again (cheap; the base guard still stops the gate when it
  fails). Each criterion is judged **when reached**, in order (an earlier re-run can invalidate a later record):
  re-used only if its record parses, its checksum holds, its `lot`/`id` match, its TOML table equals the current one,
  the digest computed now equals the recorded one, runner identity, tool versions and environment are equal, and
  every evidence file exists with its recorded SHA-256. Otherwise the record and its evidence are deleted and the
  criterion runs; the first mismatch is the reason. A re-used entry carries `"resumed": true`.
- **A valid failure stays blocking**: a recorded `fail` is re-used as `fail`, never retried by `--resume` (retry =
  change the candidate, or run without the flag).
- **Interrupted criterion** = no record (records are written only after the command exits) ⇒ re-run.
- **Native performance: always re-run.** A criterion that declares `exclusive = true`, or whose record shows an
  exclusive measurement step (`scripts/heavy.sh --exclusive`), is never re-used. Its validity depends on machine
  conditions at measurement time (idle host, no concurrent heavy job, cadence) that cannot be re-observed afterwards;
  re-validating them means measuring again, so the simpler rule is also the only sound one.
- Manifest under `--resume`: `resume = {"reused": [ids], "rerun": {id: reason}}`.
- Not in scope: a cache across candidates, a partial resume inside a criterion, impact selection.

## Activation

`gate.sh` runs the **merge-base's** runner. Until GR is merged into `main`, `mise run gate -- --resume <lot>` runs
the old runner, which exits 2 on the unknown argument; the feature takes effect for a branch once its merge-base
contains GR. GR's own gate therefore tests the working-tree runner directly (in-process and `python3 -I
scripts/gate.py`), never through `gate.sh`.

## Proof (gates/GR.toml)

`spec` (this plan, CLAUDE.md mentions the flag) → `unit` (the 21 named tests of `scripts/gate_resume_test.py`, among
them a mutant `candidate_digest` returning a constant, which must re-use a changed candidate while the real one
re-runs) → `regression` (`gate-selftest.py`, `gate-guard.py --self-test`, `gate_reset_test.py`) →
`test-py-inclusion` (`gate_resume_test.py` added to `test:py` at implementation; during the freeze it is simply
not listed there, so CI stays green) → `tooling-checks` → `simplify-report`. Guarded files (`scripts/gate.py`, the
new test, `mise.toml`) get reviewed overrides in `gates/guard-overrides.json`, decision: this plan.
