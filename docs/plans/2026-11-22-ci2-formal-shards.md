# CI2 — Run formal checks on separate GitHub runners

Before PU1, the formal job took 39:23, 30:26 and 38:18 in runs 45005007973, 45022971739 and 44888290738. The sequential TLC runs exhausted the 45-minute job after PU1 added SyncOrder and InboundLifecycle. Alloy had already completed. Split TLC by model family so each configuration, mutant, and witness still runs in full, on a runner with the existing worker, memory and timeout settings. [Cancelled run 45030441084](https://github.com/oioio-space/inkwell/actions/runs/45030441084)

## Measurements

The table sums the gaps between consecutive TLC verdict timestamps in the `formal` log, starting with the TLC task launch. These wall times include startup and per-run harness overhead. The final gap ends at cancellation, 66.8 seconds after the second InboundLifecycle verdict. Fourteen InboundLifecycle entries have no completed verdict in this run.

| Family | Completed runs | Seconds | Shard |
|---|---:|---:|---|
| RepoGen | 4 | 12.3 | 1/3 |
| ConnCredit | 5 | 9.4 | 1/3 |
| WriteBudget | 6 | 4.4 | 1/3 |
| OISingleInstance | 23 | 302.6 | 1/3 |
| SyncOrder | 12 | 28.4 | 1/3 |
| InboundLifecycle | 2 of 16 | ≥368.7 | 1/3 |
| SendPath | 17 | 79.9 | 2/3 |
| SP1 | 21 | 389.6 | 2/3 |
| SandboxScope | 25 | 227.4 | 2/3 |
| ProjectChange | 23 | 246.0 | 2/3 |
| SyncIndicator | 9 | 119.7 | 3/3 |
| LinkProbe | 23 | 275.0 | 3/3 |
| DirectUpgrade | 44 | 442.9 | 3/3 |
| PeerUpAck | 19 | 190.5 | 3/3 |
| Alloy | All | 34.25 | Alloy |

TLC shard totals from the log are ≥12:06, 15:43 and 17:08. Shard 1 passed all 66 entries locally in 16:09. Its final 14 InboundLifecycle entries took 5:46 locally, giving a mixed-run estimate of 16:45 for shard 1 when added to the GitHub timings through the second InboundLifecycle verdict. The estimate excludes runner setup and awaits a hosted rerun. Shards 2 and 3 contain 86 and 95 entries. HP1 later adds its three HP1r runs (link lost, late landing, witness survives) to shard 3/3 with the DirectUpgrade family they extend; the partition test caught them as unassigned. Block HP1c adds 17 runs, 194 s locally (2026-11-23, 2 workers): its
seven DirectUpgrade crash runs (`HP1c[!S]*`, about 50 s) go to shard 1/3 and its ten Supersede runs (`HP1cSupersede*`,
about 150 s) to shard 3/3. From the last green run's shard times (45087731662: 14:19, 15:58, 13:21) that gives
about 15:09 and 15:51, and shard 2/3 stays the longest.

## Design and acceptance

A matrix starts three TLC shards and one Alloy job. Each has a 45-minute timeout and runs with mise tool installation disabled (CI1/CI1b). The `formal` job requires a successful matrix result and an explicit success output from each leg. A leg that never starts supplies no output. The runner omits the empty outputs from its other legs. This preserves the check name for GitHub and for exact job-name lookups in `scripts/gh-verdict.py`. Its frozen callers currently ask for other jobs. GitHub's classic branch protection API reported that `main` was unprotected, and the branch rules API returned no required status check rule. [Matrix jobs](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/run-job-variations), [matrix job outputs](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#using-job-outputs-in-a-matrix-job), [runner output handling](https://github.com/actions/runner/blob/main/src/Runner.Worker/JobExtension.cs#L733-L755), [required checks and skipped jobs](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks#handling-skipped-but-required-checks)

The TLA+ script keeps the same run list and uses `TLA_SHARD` only when supplied. Shard selection fails for an unassigned family. The Python tooling suite compares the full run list with the multiset of lists emitted by the dispatcher for the workflow's matrix and rejects duplicates or empty shards. `mise run formal` with no shard or inner-loop filter runs all checks, including Alloy.

## Base guard

| Guard ID | Reason |
|---|---|
| `8418ebaf38c587103c7f8db4108caadcd3c1b081729b0eb48b7b53a288dbe3c7` | The timeout scanner flags the extra timeout line for the new 5-minute summary job. Every formal worker has the original 45-minute limit. |
| `3177912c2e823ca534985f31d7f0cc2e2f9a79d403165d5102921b15953e22fc` | The `test:py` task adds the partition tests to its existing test list. |
| `ff357fb48b908270365f2e2ba5f0b9ffd9e41b792bdb95e9f2ff50429a421f6c` | The TLA+ harness adds opt-in dispatch and listing. The full run list, assertions, TLC arguments and mutant verdicts are preserved. |
