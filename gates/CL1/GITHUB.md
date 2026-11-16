# CL1 - the netsim job on GitHub after the merge

Status: open (no run counted yet; the block is not merged).

This record is the acceptance of CL1 that the local gate cannot give: the defect shows on GitHub's runners and not on
the development machine (`WITNESSES.md`, ten local runs of ten passing on the unchanged tree).

## What is counted

- Workflow `ci` (`.github/workflows/ci.yml`), job `netsim`, step « Impaired network (bad Wi-Fi, blackout, slow relay,
  three-device mesh) » (`mise run e2e:netsim:impaired`).
- Runs of event `push` on `main` whose head commit is CL1's merge commit or a descendant of it, in the order GitHub
  lists them (`scripts/ghx.sh run list --workflow ci --branch main`). First commit counted: `<merge commit, to fill>`.
- Attempt 1 of each run only. A re-run (attempt 2 or more) is not a new draw; if any attempt of a counted run fails C5
  of the editor variant, the run counts as a failure.
- A run is **measured** when its log has the line `netsim 8-editor-s<seed> C5 pass` or `… C5 fail`. A run cancelled
  (the workflow cancels a run when a newer push arrives) or that dies before that line is listed below with its
  reason and does not count, either way.
- The record closes at **ten measured runs over at least three distinct seeds**. The CI seed is the day number
  (`scripts/netsim/run.sh:175`), so three seeds mean runs on three different UTC days at least.
- For each measured run, also copy the line `netsim 8-editor-s<seed> claims written, ms from the quiescence baseline:
  …` (printed since CL1): three devices, three negative numbers. This is the cause observed, not inferred.

## Verdict

- **Accepted:** ten measured runs, three seeds or more, C5 of the editor variant passing in all, and every copied
  claims line showing each device's claim before the baseline. Then `Status: closed (accepted, <date>)`.
- **Reopened:** one C5 failure of the editor variant among them. The claims line of that run says which: a claim
  after the baseline (the fix does not hold on that runner), or none (the cause was not the claim, or not it alone).

## What ten green runs are worth

Before the change, of the 12 runs that measured it: 7 failures (seed 20771: 4 of 8; seed 20772: 3 of 4; ids in
`WITNESSES.md`). Both verdicts occur under each seed, so the seed does not decide the verdict and runs of one day are
not copies of one another; nothing shows they are independent either.

- If runs are independent draws: ten passes by chance, (5/12)^10 = 0.00016; at the one-sided 95 % lower bound of the
  failure rate (Clopper-Pearson for 7 of 12, 0.315), 0.685^10 = 0.023.
- If runs of one seed were fully dependent (one draw per seed): three seeds, (5/12)^3 = 0.072; at the bound, 0.32.
  On that reading the count alone proves little: the claims lines carry the proof.

## Runs

| # | Run id | Date (UTC) | Head commit | Seed | Attempt | 8-editor C5 (per pair) | Claims, ms from baseline | Counted |
|---|---|---|---|---|---|---|---|---|
