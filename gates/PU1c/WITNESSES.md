# PU1c - red witnesses

Recorded 2026-11-17 at step 1 of PU1c, before any PU1c production code, on the worktree `inkwell-pu1` (PU1 + PU1b
step-2 code, uncommitted, on `main` c426b8e). Round 2, after Codex's first review (DO NOT FREEZE): every criterion
re-run on the amended texts (raw logs `pu1/pu1c-red4.log`, TLC `pu1/il-16.log`, logs
`~/.cache/inkwell/tla/logs/InboundLifecycle*.log`).

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| vectors | 90 s | 1 | 4 s | `enforce None, want 'step-3'` for the 33 inbound.json vectors; `future (not required)` (no reference decoder); the three digests match (inbound.json's is round 2's: 128 KiB / 1 MiB windows); generator matches | yes |
| reference | 60 s | 1 | 1 s | `missing or not passing: TestLifecycle`, `TestProbeSchedule`, `TestQUICWindow` | yes |
| tla | 560 s | 0 | 292 s (4 workers, idle machine) | 16 ✓, 0 ✗ (the sixteen expected lines); structure check | passes with the real tools |
| go | 450 s | 1 | 6 s | `missing or not passing: TestInboundLifecycle`, `TestInboundTransport` and every arm, `immediate_when_first_and_up`, `quic_window_reserved`, `refused_stream_bytes_released` included | yes |
| inherited | 120 s | 0 | 16 s | PU1's and PU1b's arms pass on the current code | non-red by design |
| regression | 270 s | 0 | 96 s | transport, p2p (92 s), bridge, blob ok | non-red by design |
| arch | 30 s | 1 | < 1 s | no `inbound.go`; the handler's helper goroutine; `bulkTail`; ProtocolBulk not 2.0.0; no protocol constants used; no `QUICReuse(`; no `AcquireCtx`; the fork has neither the window option nor the initial reservation | yes |
| docs, current tree | 30 s | 0 | < 1 s | round 2's documents | passes |
| docs, on HEAD's texts | 30 s | 1 | < 1 s | 83 `lacks` lines | yes |
| mutant | 690 s | 1 | < 1 s | `PU1c mutant entries differ from the table: ['MC1', …, 'MC11']` | yes: step 2 writes them |
| frozen-untouched | 30 s | 0 | < 1 s | the 19 frozen files match | passes (a guard) |
| limits | 30 s | 0 | < 1 s | `TestLimitsMatchSpecification` ok (QUIC row 128 KiB / 1 MiB, the assertions hold) | passes |

Round 3 (Codex's second review, the freeze): the go criterion's bounds made the spec's (≤ 250 ms on synctest/simnet
time, no 300 ms slack), P-13 states that a Byzantine lane starves its own connection's heads without a time bound.
Re-run (`pu1/pu1c-red5.log`): go exit 1 in 2 s (same reason: no TestInboundLifecycle/TestInboundTransport arm), docs
exit 0, docs on HEAD's texts exit 1 (83 `lacks`), limits exit 0.

Total of the caps 2360 s ≤ 2700 s. The tla cap is compute + wait: 292 s measured × 1.5 = 440 s, plus
`INKWELL_PERF_LOAD_WAIT` = 120 s (during a gate no other heavy job runs, so the exclusive slot wait is the idle wait).

## The model, with the real tools (round 2)

Sixteen runs, 292 s: Honest (both lanes, `ArrivingHoldsNothing`), Byzantine (one lane, two streams, liveness),
ByzantineSafety (three streams, a refused stream written into, released by its own step `Release`), Credit (both
lanes, 34 s for its three runs). Found while amending, and fixed: a refused stream released into `gone` was not marked
reset, so `FinFramesDelivered` counted its dropped frame (the refusal is a reset). A direct model of two Byzantine
lanes on one credit did not finish (≥ 700 000 states, still growing), and an environment process taking any credit
added states but no behaviour. The composition is therefore argued by over-approximation in plan §6: the lanes share
only the credit, the credit guards only writes, and a Byzantine opener's writes are unfair choices. Round 1's
findings stay recorded in the plan: bulk needs the ACK, and the premise of `EmptyLeaves`.

`gates/PU1n.toml` (netsim) is not part of this gate: it runs once, exclusive, on the candidate carrying PU1, PU1b and
PU1c.
