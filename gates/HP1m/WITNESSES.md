# HP1m - red witnesses

Recorded 2026-11-16 at step 1, before any production code, the command run exactly as written in `gates/HP1m.toml`
(scratchpad log `hp1/witness-HP1m-mutants.log`).

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| mutants | 1 800 s | 1 | 0.4 s | `FileNotFoundError: … /tree/protocol/directup/policy.go` on MH1, the first mutant: its file is not written yet | yes (not a catch, not a pass) |

Sum of the caps 1 800 s ≤ 2 700 s. Estimate before the code: 6 pure mutants of about 15 s and 16 p2p mutants of about
60 s (a copy of the tree and a build without -race), about 1 050 s; the smoke measures it.
