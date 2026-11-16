# PU1b - red witnesses

Recorded 2026-11-16 at step 1 of PU1b, before any PU1b production code, on the worktree `inkwell-pu1` (branch `pu1`
rebased onto `main` 3ac908e, carrying PU1's step-2 code with its round-1 fix: the place reserved before the ACK, the
`inTail` chain). Each criterion of `gates/PU1b.toml` was run with its own command (scratchpad runner reading the TOML;
raw log `pu1/pu1b-red.log`; TLC logs `~/.cache/inkwell/tla/logs/SyncOrder*.log`). The machine was shared with other
sessions; slot waits are inside the caps. Round 2 (after Codex's first review, DO NOT FREEZE): every criterion re-run
on the amended texts (raw log `pu1/pu1b-red2.log`); the table below is round 2's.

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| vectors | 240 s | 1 | 1 s | (20 vectors, one added: an empty end of writing while waiting) `enforce None, want 'step-3'` for the syncorder vectors; `TestCorpus/syncorder.json/<id>: future (not required)` (no `sync_order`/`sync_refusal` decoder: `ErrUnsupported`); both digests match; `TestGenerateMatchesCommittedCorpus` passes | yes |
| reference | 120 s | 1 | < 1 s | `missing or not passing: TestOrder`, `TestRefusal` (the reference does not exist) | yes |
| tla, current tree (`heavy.sh --exclusive`) | 300 s | 0 | 19 s | the twelve expected lines, 12 ✓, 0 ✗, the structure check (`Ended`, `Budget`, `Tick`, `OutOfBudget` included) | passes with the real tools |
| tla, on `main` | 300 s | 1 | — | `scripts/tla-check.sh` at `main` has no `SyncOrder` run: `missing: ✓ SyncOrderHonest…` | yes: the model is step 1's |
| go | 300 s | 1 | 3 s | `missing or not passing: TestSyncOrder` and its nine arms (`resource_manager_refusal`, `generation_change` added) (`[no tests to run]`); `TestPeerUpMoveAfterAck/move_keeps_order_across_late_handler` passes (PU1's fix) | yes |
| regression | 480 s | 0 | 55 s | `ok internal/transport/p2p 54.2s`, `ok internal/bridge` | non-red by design (a regression bound) |
| arch | 30 s | 1 | < 1 s | `the chain of PU1's round 1 (inTail) is still there`, `does not use the reference's order or refusal` | yes: the finding of Codex's round 2 |
| docs, current tree | 30 s | 0 | < 1 s | step 1's documents | passes |
| docs, on `main`'s texts (spec, CHANGES, TLA README; empty plan) | 30 s | 1 | < 1 s | 45 `lacks` lines (the scope of the closes and the empty end of writing included): plan rows P-8, P-9, the §8.7 sentences, §16.1, CHANGES, TLA README | yes |
| mutant | 30 + 30 + 600 s | 1 | < 1 s | `PU1b mutant entries differ from the table: ['MQ1', 'MQ2', 'MQ3', 'MQ4', 'MQ5', …]` (no PU1b entry) | yes: step 2 writes them |
| pu1-model-untouched | 30 s | 0 | < 1 s | `PeerUpAck.tla`, its mutants module, `gates/PU1.toml`, `gates/PU1n.toml` identical to c7c90c7 | passes (a guard) |
| limits | 30 s | 0 | < 1 s | `TestLimitsMatchSpecification` passes with the new §16.1 row and its five constants | passes (written with the row, so `main` stays green) |

## The model, with the real tools

`INKWELL_TLA_ONLY='^SyncOrder' ./scripts/tla-check.sh`, 19.2 s for the twelve runs: Honest 2 560 distinct states
(honest timing, D = 3; liveness checked; the capacity refusal, its backoff and retry reached: `SyncOrderWitnessBusy`),
AnyTiming 2 715 (D = 2, invariants; the budget runs out during a capacity refusal and the session opening closes the
peer: `SyncOrderWitnessBudgetOut`; `SessionKept` and `Budget` hold), Byzantine 39 421 (three streams, any opens,
writes, ends, resets and holds, Stall; liveness `EmptyLeaves`, now covering an empty end of writing). Mutants: Q1
NoWake breaks `AllDelivered` (the successor of a drained stream waits for it for ever), Q2 KeepEmpty breaks
`EmptyLeaves` (a stream reset behind a head held open keeps its place: the finding of Codex's code round 2), Q3
ClosePeerOnBusy breaks `SessionKept`, Q4 Bound3 breaks `Places`, Q5 EofKeeps breaks `EmptyLeaves` (Codex's PU1b round
1: its counterexample ends with `list = <<2, 1>>`, stream 1 closed for writing, nothing buffered, never delivered,
waiting for stream 2, which delivered and is held open); `SyncOrderWitnessRemoveWaiting` and
`SyncOrderWitnessEmptyEof` show a stream leaving while its successor waits, and a waiting stream ended without a frame
leaving. A first Byzantine
configuration with four streams and two frames took 7 min (2.3 million states) for the same verdicts; three streams
and one frame keep every mutant caught (each needs at most three streams) and fit the gate.

## Current code against the new rules (what the step-2 arms will show)

Codex's round 2, read on the code: `leave` (stream.go) waits without bound for the stream ahead of a stream that left
before its turn; a Byzantine opener that holds a first stream open and resets the following ones keeps one goroutine
and one place per failure. Arms `byzantine_reset_loop` (InboundPlaces, goroutine count), `removal_while_waiting` (the
place freed within 1 s, not 60 s) and `bound_plus_one` (no capacity refusal today: a third stream is placed) are
expected red on it once written; they are not written at step 1 (no test is added before the freeze).

`gates/PU1n.toml` (netsim) is not part of this gate: it runs once, exclusive, on the candidate that carries PU1 and
PU1b (plan §5).
