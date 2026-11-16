# HP1n2 - red witnesses

Recorded 2026-11-16 at step 1, before any production code, the commands run exactly as written in
`gates/HP1n2.toml` inside an outer `heavy.sh --exclusive` (scratchpad logs `hp1/witness-HP1n2-<id>.log`).

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| netsim | 240 + 1 300 s | 0 | 814 s | the six runs `PASS`, every clause: editor 125, 125, 123 s; reader 144, 145, 145 s (seeds 20771, 20772, 20773) | passes by design: without PC1b on this tree no device restarts in scenario 8, so this is HP1's non-regression on the scenario; the gate is red through `netsim-mutants` |
| netsim-mutants | 900 s | 1 | 3 s | `mutant MN1: anchor in internal/transport/p2p/dial.go found 0 times …`, `netsim mutants: not caught: MN1` | yes: the upgrader's call in the dial loop is step 2's line |

Sum of the caps 2 440 s ≤ 2 700 s.
