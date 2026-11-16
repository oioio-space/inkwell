# PU1 - red witnesses

Recorded 2026-11-16 at step 1, before any production code, on the worktree `inkwell-pu1` (branch `pu1` from `main`
6c5cbf9). Each criterion of `gates/PU1.toml` was run with its own command (a small runner that reads the TOML and runs
each `command` with bash). Rounds: (1) first draft; (2) after the integrator: PU1 lands first and depends on no HP1
artefact, `TestPeerUpLatency` is a paired measurement, mutants carry `clause` for main's `mutants.py`; (3) after
Codex's first review (DO NOT FREEZE): the ACK's limit against a Byzantine peer stated (spec §8.7.7, plan), one absolute
budget for NewStream and the ACK (spec §8.7.4, P-5), the model's clock and stuck responder, acceptance remembered by
connection and stream, mutants M6-M8, the arm `slow_negotiation_late_ack`, and an executed prototype of the latency
measurement. Rows marked (3) were re-run on round 3's text. The machine was shared with other sessions (an exclusive
netsim run, coverage tests): waits for a `heavy.sh` slot are inside the caps. Raw logs: session scratchpad
`pu1/crit*.log`, `pu1/tla7.log`, `pu1/proto.log`; TLC logs `~/.cache/inkwell/tla/logs/PeerUpAck*.log`.

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| vectors (3) | 240 s | 1 | 1 s | `enforce None, want 'step-3'` for every vector; `TestCorpus/syncack.json/<id>: future (not required)` for all 34 (the reference has no `sync_ack*` decoder: `ErrUnsupported`); the digest matches; `TestGenerateMatchesCommittedCorpus` passes | yes |
| fuzz (3) | 60 + 180 s | 1 | < 1 s | `stat …/protocol/syncack: directory not found` | yes: the reference package does not exist |
| tla, on `main` | 600 s | 1 | — | `scripts/tla-check.sh` at `main` has no `PeerUpAck` run (`grep -c PeerUpAck` = 0): `missing: ✓ PeerUpAckHonest…` | yes: the model is step 1's |
| tla (3), current tree, as written (`heavy.sh --exclusive`) | 600 s | 0 | 109 s | the nineteen expected lines, 19 ✓, 0 ✗, and the structure check (sound rule operators, `Budget`, `NoUpFromStuck`) | passes with the real tools |
| go (3) | 300 s | 1 | 2 s | `missing or not passing: TestPeerUpAfterAck`, `TestPeerUpMoveAfterAck`, `TestPeerUpLatency` and their 15 arms, `slow_negotiation_late_ack` included (`[no tests to run]`) | yes |
| regression | 480 s | 0 | 73 s | `ok internal/transport/p2p 71.2s`, `ok internal/bridge 1.3s` on `main`'s code | non-red by design (a regression bound): passes with the real tools |
| mutant (3) | 30 + 30 + 480 s | 1 | < 1 s | `PU1 mutant entries differ from the table: ['MP1', 'MP2', 'MP3', …]` (no PU1 entry in `scripts/netsim/mutants.toml`) | yes: step 2 writes them |
| arch (3) | 30 s | 1 | < 1 s | `does not import protocol/syncack`, `/inkwell/sync/1.0.0 still in the product code`, `ProtocolSync is not /inkwell/sync/2.0.0` | yes |
| docs (3), on `main`'s texts (spec, CHANGES, TLA README from `main`, empty plan) | 30 s | 1 | < 1 s | 47 `lacks` lines: plan rows, `spec §8.7 lacks …` (the Byzantine limit and the single budget included), §20.6, §16.1, CHANGES, TLA README (S4, M6-M8), `plan lacks the paired latency measurement` | yes |
| docs (3), current tree | 30 s | 0 | < 1 s | step 1's documents; §8.7 names no HP1 rule | passes |

`gates/PU1n.toml` (netsim) was **not** run at step 1 (no netsim now; it needs the machine alone). It characterises
real timings of scenarios 1-4 (strict) and 5, 8 (characterize); scenario 9 is HP1's and not on `main`. It is expected
green on `main`, and red when a run leaves no result file or a clause fails. To be run once, exclusive, on the step-2
candidate.

## Latency prototype (Codex round 1, executed on main's code)

`gates/PU1/latency_proto_test.go.txt`, copied into `internal/transport/p2p` only for the run, then removed (no test is
added to the tree at step 1). Two raw libp2p hosts on the package's simnet (`natNet`) inside `testing/synctest`,
public ends, jitter 1 ns, one-way 50 ms (RTT 100 ms); the responder writes one byte (the stand-in ACK) as soon as it
accepts the stream; in one run, the delay from `NewStream` to its return (today's PeerUp) and to that byte (PU1's
PeerUp), on the cached path (identify filled the peerstore) and then on the negotiated one (`RemoveProtocols`).
Result (`pu1/proto.log`, `heavy.sh`, exit 0):

    one-way 50ms, RTT 100ms; cached: return 0s, ACK 100ms, paired difference 100ms;
    negotiated: return 100ms, ACK 100ms, paired difference 0s
    --- PASS: TestPU1LatencyPrototype (0.01s)

So the measurement is exact on controlled time: the cached path pays exactly one RTT (the frozen bound R to R + 2 ms
holds with 2 ms to spare), the negotiated path pays nothing (the byte rides the multistream echo; the frozen bound is
0 to R + 2 ms). Step 2's `TestPeerUpLatency` makes the same pairing on the real Node through a test-only seam.

## Can pass, with the real tools

- **tla** (round 3, `pu1/tla7.log`, 107 s; as written 109 s): Honest 168 distinct states (liveness), Late 4 262 146
  (any timing, invariants, 56 s), LateLive 24 528 (liveness), Silent 569 (liveness), Stuck 2 071 (b's handler never
  decides, any timing, time fair: S5, S4, L2), Byzantine 1 621 892 (invariants, 20 s), Reuse 374 200 (three streams,
  any timing, invariants, 5 s). M1-M8 each caught by the property the README names, the four witnesses reached:
  M6 (deadline guarded by silence) leaves an opening for ever against the stuck handler (L2), M7 (acceptance by
  connection) lets a late ACK of an abandoned move promote the next stream on that connection (S2), M8 (clock restarted
  at NewStream's return) lets the negotiated path exceed the budget (S4); `PeerUpAckWitnessLateFire` shows a deadline
  firing while the stream's bytes are in flight or its handler is deciding. Byzantine runs with honest timing: with
  any timing it passed 9 million states without ending (stopped); any timing is Late's, Stuck's and Reuse's.
  M1's shortest counterexample (depth 4): `a` registers connection 1, `b` has not, `a` (cached path) announces `b` on
  NewStream's return: `reg[b][1] = "no"` violates S3; the refused-at-registration trace is reached in the sound model
  by `PeerUpAckWitnessLazyLate` and blocked there by the ACK.
- **vectors**: `protocol/cmd/vectors` writes `syncack.json` (34 vectors: 9 `sync_ack`, 6 `sync_ack_responder`,
  19 `sync_ack_opener`, two of them the slow negotiation and late ACK, scope `transport`); `TestGenerateMatchesCommittedCorpus` and `TestCorpus` pass on the tree
  (proposed vectors are not required, so the CI stays green). Step 2 sets `enforce: "step-3"` in the generator only
  (the digest ignores `status` and `enforce`).
- **go**: the parser read the real events (package `[no tests to run]`); subtests report as `TestPeerUpAfterAck/<arm>`.
- **Caps** (`gate.deadline_problem` returns none): PU1 2 460 s, PU1n 2 320 s, each ≤ 2 700 s.
