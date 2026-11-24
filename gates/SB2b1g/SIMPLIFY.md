# SB2b1g — /simplify review of the block's diff

Diff : 6a73087..working tree (internal/sidecar/memory_linux.go, memory_rules_linux_test.go, bound_linux_test.go,
helper_signal_linux_test.go, helper_test.go; scripts/netsim/mutants.toml; docs/adr/0044-sandbox-tiers.md;
docs/plans/2026-11-14-sb2-sandbox-tiers.md, docs/plans/2026-11-24-sb2b-1g-dirty-pages.md). Checklists: go-style-guide,
use-modern-go, vuln-guard, secret-guard, helper-ergonomics (no exported API changed).

## Findings

| # | Lens | Finding | Outcome |
|---|------|---------|---------|
| 1 | Reuse | The old `dirty` helper and the new `helddirty` one would duplicate the write loop | Applied: both go through `writeDirtyHolding(path, mib, hold)` (hold 0 = the former `dirty`) |
| 2 | Reuse | A second `throttleWrites` would copy `throttleReads` | Applied: `throttleIO(scope, device, key, limit)` serves `rbps` (page-cache test) and `wbps` (dirty test) |
| 3 | Reuse | `rssAnon` parsed a `/proc/self/status` "NNN kB" field the same way `raceAddressReserve` parses `VmPeak` | Applied: shared `statusBytes(status, key)` |
| 4 | Simplification | `unreclaimable` still read `file_dirty` and `file_writeback` | Applied: only `file`, `shmem` and `unevictable` are read and required (S-11a); a stat without dirty counters is complete (test row) |
| 5 | Altitude | The witness's held memory had to be sized against the bound with the target's own runtime memory included; a fixed mmap of 80 MiB, then of 104 MiB touched by Go stores, was either too slack (red 0/3 and 1/3 under the former rule) or grew with the race detector's shadow | Applied: the target keeps its resident anonymous memory at 108 MiB from `RssAnon`, faulted in by `MADV_POPULATE_WRITE` and given back as the runtime grows; red 5/5 under the former rule, green 5/5 under S-11a |
| 6 | Altitude | Under `-race` the run's launcher (the race-built test binary) alone adds tens of MiB of anonymous memory to the run cgroup (measured: 127 MiB anon with nothing written), so no hold fits under the 128 MiB bound | Applied: `held_heap_slow_storage` skips under the race detector, with that reason; the gate's `witness` and mutant MD2 run without `-race` and allow no skip. Not a weakened test: a new case, the rule proven where the measurement is meaningful |
| 7 | Efficiency | `writeDirtyHolding` reads `/proc/self/status` once per MiB written (64 reads) | Kept: negligible beside the 8 MB/s write, and it is what keeps the hold exact |
| 8 | Correctness of the step-2 edit | A `sed` of the 96 → 104 MiB sizing also hit the `resident` scope-app helper's 96 MiB mapping | Fixed before the criteria's regression run: that helper is back to 96 MiB (no diff left there) |
| 9 | Docs | The plan's §6 and the ADR witness sentence named the step-1 sizing (80/96 MiB) | Applied: both state the implemented sizing and the race skip |

## Dismissed

- Mutant ids MD1-MD3 also exist in block PU1d: the ids are frozen by the gate; `scripts/netsim/mutants.py` selects by
  block, and ids were already shared across blocks (MW3, MW4, MP1-MP3). Reported to the integrator, not renamed.
- Restoring SB2a1's MS1-MS22 (deleted by a541db5): outside this block (plan §6); MD4, MD5 and MD7 cover the S-11 ones.

## Mutation

No `protocol/*` package is touched: no gremlins run (definition of done, step 5). The block's Go mutants MD1-MD7 are the
gate's `mutants` criterion.
