# /simplify — PU1c

Diff : 76200b1.. (main; the worktree's PU1 + PU1b + PU1c step-2 code rebased onto it by patch,
`pu1/pre-step2.patch`, nothing lost but `scripts/tla-check.sh`, where main's `metaroot` change is kept). The block's
own files: `internal/transport/p2p/inbound.go` (new: the lifecycle both lanes share), `stream.go` (`handleStream` =
`serveInbound`, `openAcked` for both lanes), `bulk.go` (`/inkwell/bulk/2.0.0`, `handle` = `serveInbound`, the opener
reads the ACK; `bulkTail` gone), `node.go` (`quicReuse`, `connLost`, the lanes, seams), `export_test.go`,
`inbound_test.go` (new), `sim_test.go` and `contract_test.go` (a node takes the simulated socket in its own QUIC
manager), `syncorder_test.go`; `internal/transport/frame/budget.go` (`AcquireCtx`, `Held`) and its tests and
benchmark; `protocol/syncack/lifecycle.go` and `lifecycle_test.go` (new), `protocol/conformance/inbound.go` (new),
`protocol/cmd/vectors/inbound.go` and `vectors/inbound.json` (enforce `step-3` only); `scripts/netsim/mutants.toml`
(MC1–MC11; MQ1, MQ2, MQ5 re-anchored); the plans' two inventory tags (`mise run inventory`); `go.mod` (temporary local
replace); the go-libp2p fork change, `docs/plans/2026-11-17-pu1c-go-libp2p-fork.patch` (applies on a pristine
`cae49c803966`, result identical to the clone `~/GolandProjects/go-libp2p-pu1c`). Reviewers: four read-only /simplify
agents (reuse, simplification, efficiency, altitude; Sonnet / medium), triaged and applied by the step-2 session
(Opus). The review that counts for the merge is the integrator's (Opus or Codex).

## Appliqué

- `syncack.BulkProtocolID` removed (dead: `ProtocolBulk` must stay a literal, frozen arch criterion).
- `Lifecycle.Up`, `Turn` and `advance` return nothing (their « leave » was always false).
- `deliver`: the reserve closure is built once per stream, not per frame; the prefix reader only when bytes were
  probed (`src = s` otherwise); `setProbed(…, 0)` only when a byte was probed (one lock less per stream).
- `frame.Budget`: the wake channel exists only while someone waits, so an uncontended `Release` allocates nothing
  (the cond-based Budget did not either). Paired, interleaved benchmark of main's Budget against this one
  (`BenchmarkBudget`, 10 × 2 runs, benchstat, `bench-budget-old.txt` / `bench-budget-new.txt`): uncontended
  20.94 → 20.13 ns (−3.85 %, p = 0.001), hand-off 49.88 → 48.51 ns (−2.74 %, p = 0.015), 0 B and 0 allocs both.
- Test helper: `openAcked` is `openAckedOn(…, ProtocolSync)` (one copy).

## Écarté (avec la raison)

- `syncack.ProbeEvery`/`ProbeMax` aliasing `protocol.InboundProbe*`: the package's style (`Places`, `BusyCode`,
  `ReorderWait` alias `protocol` the same way); p2p uses `protocol`'s names because the frozen arch criterion asks for
  them.
- One lane-aware hook pair instead of `beforeAck`/`afterAck` (sync) beside `ackHold`/`afterAckLane` (both lanes): the
  sync-only seams are PU1's and PU1b's (their arms use them); folding them rewrites frozen-era tests for no behaviour.
- `quicReuse` repeats libp2p's default `ConnContext`/`VerifySourceAddress` wiring (altitude: a fork option on the
  default constructor would avoid the copy): the frozen arch criterion requires the node to configure its QUIC
  transport through `libp2p.QUICReuse(` with the fork's quicreuse option; the copy is eleven lines, commented « as
  libp2p's default ». Flagged for LP1 (fork upgrade).
- One `testHooks` struct, a `lanes` map, `maxCost` from the budget, a `sleepCtx` helper shared with `openAcked`, one
  reused timer in `await`, one context per connection instead of per delivering stream, an interface instead of
  `pushLocked`: each saves a few lines or one allocation per stream (per 250 ms while waiting), never per frame.
- A FIFO waiter list in `Budget` (`x/sync/semaphore`'s shape): the broadcast wakes at most the readers blocked on one
  lane's budget, which only a stalled consumer produces; not a hot path.
- The sim's QUIC socket wiring duplicated between `sim_test.go` and `contract_test.go` (two fakes of different
  simulators).

Lint (after the review): `await` lost its unused lane argument (MC1 now reads `s.Protocol() == ProtocolBulk`, MC4
charges `n.budget`; both caught again in the final pass); the generator's `every` lost its constant start and uses
`maps.Copy` (corpus byte-identical); a French « initiales » in a vector description is marked for misspell.

## mutation protocol/syncack

gremlins v0.6.0, 4 workers, coefficient 10: killed 95, lived 0, efficacy 100 % (`pu1/c-gremlins3.log`). First pass
90.91 % (9 lived): `InState.String`'s bound, the first `Attempt` (`attempts > 0`), the exact P-13 bound (`<=`) and a
byte schedule not starting at 0 are now tested; the four survivors in error messages were removed by printing the
times instead of their difference. « Not covered » (11) are `case` conditions, gremlins' blind spot, as in PU1/PU1b.

## Vérifié

Red first: the gate's witnesses (`gates/PU1c/WITNESSES.md`, step 1). On the final tree: every criterion of
`gates/PU1c.toml`, `gates/PU1b.toml` and `gates/PU1.toml` once (`pu1/c-all2.log`, TLA 16 + 12 + 19 ✓ with
`--exclusive`; the criteria touched by the lint fixes again in `pu1/c-final.log`); MC1–MC11, MQ1–MQ5, MP1–MP3
caught (MP1 and MP2 re-anchored: `openOnce`'s condition and the Responder now in `inbound.go`); consumers with -race
(`./internal/transport/...`, `./internal/app/...`, `./internal/bridge/...`, `protocol/...`: all ok, `pu1/c-rest.log`);
`mise run lint` 0 issues; `mise run inventory` 0; the fork's own test (`quicreuse/initial_window_test.go`) ok. Not run:
`gates/PU1n.toml` (netsim, the integrator's, exclusive machine).

For the integrator: (1) `go.mod` carries a temporary absolute replace to the local fork clone: push
`docs/plans/2026-11-17-pu1c-go-libp2p-fork.patch` to `oioio-space/go-libp2p` and replace it with the new
pseudo-version; (2) `TestSyncOrder/byte_then_end_while_waiting` follows P-11 now (see `gates/PU1b/SIMPLIFY.md`);
(3) one red `drain_replace_reconnect` in the first full pass, not reproduced in 15 runs since.

## Review follow-up (Codex, 2026-11-21)

- [PU1c] P-11. The frozen `reference` criterion and §8.7.3 say one byte per period. The frozen corpus still accepts the
  250 ms grid, rejects a 251 ms gap, and rejects the 125 ms sequence with 300 bytes. The consumption-to-consumption
  minimum was in the reference code and its test. `ProbeSchedule` now checks one byte per started 250 ms window.
  The red test rejects an early attempt and exposes the old rejection of adjacent-window bytes 1 ns apart.
  A 60 s interval contains 240 windows, or 241 starts including the turn boundary as the corpus does, below the
  256-byte cap. Each read covers one window and reset takes precedence over buffered bytes on both muxers, so a
  reset after the current read is observed by the next attempt within 250 ms. The transport already sets its
  probe deadline from the attempt grid. Spec, gate and vectors are byte-identical to the pre-review candidate.
- [PU1c] P-13. The fork option now opens and charges the inbound resource scope inside `ConnContext`. The authenticated
  connection reuses that scope. A failed reservation releases it and refuses the connection. Cancellation before
  acceptance and the libp2p close path share a once-only release after a successful reservation. The red witness
  lets the former accept-time implementation complete its QUIC handshake with less than W available. The new
  witness requires QUIC `CONNECTION_REFUSED` and checks that the failed reservation changed no memory. The fork
  test exercises release before `Accept` and a second close. The transport enables only native QUIC and TCP.
  [quic-go v0.64.1 server connection creation](https://github.com/quic-go/quic-go/blob/v0.64.1/server.go#L790)
- [PU1c] The root dead-code check starts from the application commands. The ten named `syncack` symbols are
  called by the public conformance runner, so the baseline records those symbols following the `linkprobe` entry.

## Resumed verification (Codex, 2026-11-21)

All eleven frozen PU1c criteria passed in the diagnostic replay on `f5d2b55`. The runner reused the intact vectors
and reference results after interruption. The exclusive TLA criterion verified sixteen expected outcomes. The
Go mutant criterion caught MC1–MC11 and the re-anchored MQ1, MQ2 and MQ5 with their specified diagnostics. The
transport, app, bridge, blob and protocol suites passed under the race detector. The base guard still blocks
acceptance of the three blocks. The final `pregate` also stopped in inventory on WebKit CI run 44933423541 on `main`.

The final patch adds the reference test for a byte outside a started window to the product code verified by that
replay. The race run of `protocol/syncack` passed with the additional test. A mutation admitting a byte exactly at
the start of an unstarted window failed that test. A mutation admitting a second byte in the same window failed
`one_byte_per_window` on its duplicate-byte assertion. Gremlins killed 97 of 98 covered mutations (98.98% test
efficacy). The survivor changes the reported byte count in the limit error. Both variants reject the same byte
and leave the counter and buffer unchanged.
