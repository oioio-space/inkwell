# HP1n3 - red witnesses

Recorded 2026-11-16 at step 1, before any production code, the commands run exactly as written in
`gates/HP1n3.toml` inside an outer `heavy.sh --exclusive` (scratchpad logs `hp1/witness-HP1n3-<id>.log`).

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| netsim | 240 + 1 500 s | 1 | 991 s | the three runs `PASS` every clause (328, 330, 329 s); `HP1n3 netsim: exit 0, ["5-s20771.json: expect strict, missing ['C3a'], red []", …]` | yes: C3a (plan §7, D6) is not written yet |
| netsim-mutants | 900 s | 1 | 361 s | `mutant MN2 -> C3a: NOT CAUGHT (clause C3a not recorded)`; the mutant does bite: in its run C3 fails, `downsTotal 1, 1` | yes: the clause that must catch it does not exist yet |

Sum of the caps 2 640 s ≤ 2 700 s.
