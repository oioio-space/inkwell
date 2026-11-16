# HP1n - red witnesses

Recorded 2026-11-16 at step 1, before any production code, the command run exactly as written in `gates/HP1n.toml`
inside an outer `heavy.sh --exclusive` (the wait for the machine stayed outside the cap; scratchpad log
`hp1/witness-HP1n-netsim.log`). This is the second round: Codex's freeze review (B5) found that the second `run.sh`
wiped `.task/netsim` (scripts/netsim/run.sh, `rm -rf "$state"`) before scenario 9's results were read; each run's result
files are now copied to `.task/hp1n-results` right after it, and only the copies are read. The first round (exit 1,
315 s) is superseded.

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| netsim | 240 + 1 500 + 360 s | 1 | 317 s | scenarios 1-4 ran, then `netsim: no scenario 9` (run.sh exit 2, before its summary); 6 and 7 on seed 20772 `PASS` (41 s, 103 s; 7's C2 known-failure), their copies `6-s20772.json` and `7-s20772.json` in `.task/hp1n-results`; `HP1n netsim: exit 2 (1-4, 9) and 0 (6, 7), ['9-s20771.json missing', '9-s20772.json missing', '9-s20773.json missing']` | yes: scenario 9 is not written yet; the regression part is green and read from its copies |

Sum of the caps 2 100 s ≤ 2 700 s. Scenario 9 is new: its cap (with 1-4, about 170 s) assumes up to about 400 s per
run including the stack's start; the smoke measures it, and a longer run moves 1-4 elsewhere, never a seed away.
