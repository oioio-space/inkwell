# HP1 - /simplify

Diff : c560c9a..(working tree of the block, untracked HP1 files included: `protocol/directup/`,
`internal/transport/p2p/upgrade*.go`, `punchgate.go`, `protocol/cmd/vectors/directup.go`,
`protocol/conformance/directup.go`, the TLA+/Alloy models and the netsim scripts)

Review made on 2026-11-16 (step 2, session 4) by four parallel reviewers (reuse, simplification, efficiency,
altitude; Sonnet, first pass), read and sorted by the session (Opus). It is a first pass: the review that counts
for the merge is Codex's or Opus's (CLAUDE.md, « Sonnet ne fait pas foi »).

## Applied

- `punchgate.go`: the `onAsk` test hook was dead (set by no test): removed with its nil check.
- `punchgate.go`: `containsAddr` re-implemented `slices.ContainsFunc(as, a.Equal)`, the idiom of `addrbook.go`
  and `node.go`: replaced, helper removed.

Applied earlier in the session, as root-cause fixes rather than review findings (plan H-1, §3.3a, H-18):
the member-to-dial-loop hand-over keeps the call in flight and the ticket; a loop that is ending never steps;
calls live on the node's context; the on-demand grant serves libp2p's whole one-shot (one node token); the
black hole detector is off.

## Set aside (behaviour or scope; recorded for the merge review)

- Reuse: `directup.Jitter` duplicates `dial.go`'s `jitter`. The transport could call `directup.Jitter`, but
  that touches `dial.go` and `relay.go` callers outside the block's rules: to do in a later tidy-up.
- Reuse / simplification: the circuit-split loop (`observe`, `resolveDuplicate`, `circuitTo`, `hasDirect`, the
  test `Circuits`) is written several times, and `cmpConn` should be `cmp.Or`. A `circuitConns(p)` helper is
  the simpler form. Set aside because it changes the H-3/H-15 code paths after their tests and mutants were
  measured: to do with the next change to those paths, with the sim tests and MH3/MH15 rerun.
- Simplification: `upgrader.queued` is derivable from `slotCh != nil`. Same reason.
- Simplification: `memberDrive.handover` could be a context cancel cause (`context.WithCancelCause`). It is
  equivalent but new today; kept as written until the hand-over is reviewed.
- Simplification: `drive`/`undrive`/`handOver` share a release step; `demand`/`demandRounds` could be one
  `*onDemand`. These are cosmetic, kept as written.
- Efficiency: `observe` runs `selectAddrs(n.announced(p))` and `localReady` (a copy of the protocol list) on
  every step, and `admitDial` reads `announced` on every gater call. A cached flag would be cheaper. At ≤ 64
  peers these cost microseconds per step, and none is on a CLAUDE.md hot path, so no change was made without
  a benchstat measurement.
- Efficiency: `pp.admitted` can hold duplicates within one grant. It is bounded by the lease and the round
  budget.
- Altitude: a single upgrader loop per member (no hand-over) is the deeper form of the member/dial-loop
  ownership. It is a redesign of H-1's mechanism and needs a plan amendment: deferred to the code review after
  the smoke (integrator, 2026-11-16). → HP1
- Altitude: turning off libp2p's initiator one-shot would remove the on-demand grant (`demand`,
  `oneShotRounds = 3`, which mirrors libp2p's `maxRetries`). This would change the TLA model (`AutoPre`) and the
  frozen gates: deferred to the code review after the smoke (integrator, 2026-11-16). → HP1
- Altitude: `LiftRelayIPLimit` and the simnet's IPv6 socket are harness exceptions. RF1 fixes the first in the
  product (obligation already in the plan). The second keeps the simnet faithful to quicreuse.
- Altitude: the test hooks in `Options` (`punch`, `onCall`, `localReady`, `failUpgrade`, `rcmgr`) could be one
  `hooks` struct. This is cosmetic, kept.
- Frontend `netsim.spec.ts`: the log-path reads and the downs-delta computation are repeated; a `readLog`
  helper and a `downsDelta` helper would remove the repeats. Set aside: e2e tooling outside the Go review.

## Mutation protocol/directup

gremlins on `./directup` (smoke, session 4): 53 killed, 2 lived, 21 not covered, efficacy 96.36 % (≥ 95 %).
The two survivors are equivalent:

- `bucket.go:24:31` (`b.tokens >= b.burst` mutated to `>`): when the bucket is full both forms leave `tokens`
  unchanged; only `at` moves forward, and the next `Take` or `Tokens` call resets `at` to its own `now`. No
  observable difference.
- `bucket.go:27:43` (`n >= 0` boundary): the branch adds `n` tokens, and for `n = 0` it adds nothing. Both forms
  give the same state.

## Port onto PU1 (2026-11-22)

The direct move and the move to the retained circuit now share the sync
opener's acceptance check, Busy retry policy and original opening budget.
A pending move can replace only the stream it started from, preserving
arrival order if a concurrent reopen replaces that stream first. Both routes
use the existing inbound lifecycle. The raw HP1 member
acknowledges the sync stream so the DCUtR tests exercise an accepted session.
The HP1r two-circuit fixture waits for both relay reservations and keeps the
direct path blocked until both circuits have been observed. Its assertions
and time bounds are unchanged.
The earlier review above records the source tree; the port is based on
`a541db5` and needs the final review on that diff.

### Port checks

- The local go-libp2p clone at `~/.cache/inkwell/go-libp2p-hp1m` is on
  `inkwell/hp1-pu1c` at `6de4a5e`, with the `c37a183f` changes staged for
  the integrator. Holepunch, QUIC and quicreuse passed under `-race`.
- Build, vet and lint passed with that combined fork. The untracked
  verification modfile and workspace were kept outside this checkout.
- The protocol policy, conformance suite and vector generator passed under
  `-race`. The regenerated direct-upgrade and sync-ACK corpora match the
  candidate files.
- The complete `TestUpgrade*` run passed under `-race` in 175.189 s
  (270 s cap). The repaired two-circuit fixture passed 20 repetitions and
  the idle-ladder test passed 15 repetitions.
- `mise run formal` passed all 250 TLA+ runs and the Alloy suite. The
  frozen HP1 and HP1r checks for verdicts and model structure passed against
  the 44 HP1 and three HP1r verdicts from that log.
- `TestWantStopsAndDrop` passed 40 of 40 repetitions.
- Biome and Svelte type checking passed, and Vitest passed 1,383 tests
  with one existing TODO. The netsim scenarios were reserved for the integrator.
- The root transport consumers passed under `-race`, including the desktop
  command with a private writable IPC directory. The in-test package loader
  passed with the combined-fork workspace inherited by its Go subprocess.
  All 137 top-level p2p tests and fuzz seed suites passed across
  `final-race-early.log`, `final-race-late.log` and `remaining-race.log` in
  `~/.cache/inkwell/hp1m-run`. The resumed batches used the shared heavy-job
  locks with read-only file descriptors because the sandbox protects the
  lock directory from writes.
- PU1b's `drain_replace_reconnect` passed 80/80 on `a541db5` and 9/80 on
  the initial port. The trace showed the reopen selecting another direct
  connection that was closing. The pinned reopen on the surviving circuit
  passed 80/80 under `-race`; the inherited PU1 tests passed as well.
- HP1, HP1r, PU1, PU1b and PU1c mutation anchors all apply. HP1r's docs
  criterion passes; HP1's only docs failure is ADR 0045's proposed status.
  The mutant runner's 11 Python unit tests passed.

### Pregate guard findings

| Guard ID | Change and retained evidence |
|---|---|
| `7d115488ff92a841ccb2f58afe358e8612470ae6c99314ee1e2c20ca1cbf2fce` | Adds the archived 600 s Playwright cap for scenario 9. Existing scenarios retain their caps. C1 and C2 keep their separate 30 s waits. |
| `fe77e923f52141c1e727e6021d340c4a931e677ddffa527f94667e203ad42e55` | The simnet delivery callback captures the selected destination socket before releasing the simulator lock. It keeps the same delay calculation; the H-19 fixtures can add latency between NATed peers. |
| `ac421e2f9ae3835176a9b5263baa384a09392baa475cc041c1e40bdaf65be1ef` | The duplicated packet uses the same captured socket. Its scheduling expression still uses the existing duplicate delay. |
| `f326e7d354e0fe62e8adc52b16fe44992e71c8aa25041d37fd8e9915ee960f98` | Go mutants can carry `expect` without `clause`. The runner reports that label instead of throwing after a result; it retains the verdict and matching rules. |
| `eb44f5f12eed537d81f95fb9dc21f60b25020e5c4e113916764079f0035218c4` | Adds scenario 9 process restart and PID cleanup to the harness. Existing scenario selections, prerequisites and qdisc truth checks are preserved. |
| `9501f711acfc9535e5aad1ced075b3e11d8c0bd73432187528c422d4dc2ad825` | Adds `HP1rLinkLost`, `HP1rLateLand` and `HP1rWitnessSurvives` and assigns them to CI2 shard 3/3 with DirectUpgrade. The original 44 DirectUpgrade runs and all PU1 runs stay registered. (The row's earlier ID, `6284940e…`, was from before the CI2 port; it is no longer in `gates/guard-overrides.json`.) |

### Frozen gates

Run HP1, HP1m, HP1n, HP1n2, HP1n3 and HP1r in that order on one final
candidate. HP1's Go test descriptions predate PU1. Observations of link kind
at PeerUp now occur after the acceptance RTT, and the H-15 stream move
requires an ACK. HP1r's absence-of-PeerUp assertion keeps its original
meaning and time bound. PU1 supplies the acceptance proof the pre-PU1
closed-during-registration trace was missing. HP1n and HP1n2 should repeat
their red witnesses, especially MN1: a direct link can be observed before
the first ACK, so a pre-PU1 observation order is not evidence on this tree.
The frozen commands retain their acceptance assertions.

HP1's docs criterion requires the integrator's acceptance of ADR 0045.
Scenario 9's archived C1 starts its 30 s wait after the page reports
readiness. Its frozen description starts the clock at process restart.
Scenario 5's C3a uses a count at baseline. On this candidate Bob does not
restart, so it still requires zero outages through baseline. When PC1b
introduces a restart there, the count alone cannot place an outage inside
that restart's window. → HP1, PC1b

### DNS scope

The H-12 address filter rejects named addresses in DCUtR rounds. The fork's
swarm resolves peerstore addresses before the connection gater runs, so a name
learned through identify can reach the resolver during a preliminary direct
dial. The DCUtR filter and its vectors cover addresses exchanged in the round.
[Pinned swarm dial path](https://github.com/oioio-space/go-libp2p/blob/6de4a5e06d73b04f26d4464730d9806431fec2f8/p2p/net/swarm/swarm_dial.go#L293-L306) → LP1

## Post-port root-cause review (2026-11-22)

| Finding | Regression | Baseline failure |
|---|---|---|
| 1 | `TestPunchCancelledRoundReturnsGrant` | Cancellation during the half-RTT wait left one grant held. |
| 2 | `TestPunchRefusedAnswerDoesNotReleaseAnotherRound` | A refused answer changed the active round's held slots from one to zero. |
| 3 | `TestReopenChoosesLiveConnectionAfterReset` | Recovery opened on the dead circuit and called ClosePeer with a live direct connection. |
| 4 | `TestRetiringSyncStreamReleasesAtOrderDeadline` | The predecessor still blocked moves at the P-9 ordering deadline. |
| 5 | `TestNoteActivityLocksOnlyOnIdleTransition` | Active input waited on Node.mu. |

All five HP1 witnesses passed with the race detector after the fixes. The
regression suite also exercised FIFO waits, two circuits, dial budgets,
and stranger refusal. The transport owns round grants through their
deferred releases and keeps libp2p's protobuf, size bound, and resource
service. The preliminary dial's grant belongs to the entire connect call.
The sync opener selects the recovery route at opening time, and the
predecessor closes gracefully after the existing ordering window. A
single atomic timestamp update emits EActive when input resumes.
[Root causes and pinned sources](../../docs/plans/2026-11-16-hp1-direct-upgrade.md#post-port-review-on-the-published-fork) [HP1]

The base guard reported the six findings already listed above. Mutation
edits still apply for HP1, HP1r, PU1, PU1b and PU1c.

The final full race run passed for p2p (542.137 s) and bridge (10.061 s).
The H-17 policy test passed under the race detector. Pregate passed its
dependencies and exited 1 at the base guard with the six existing findings.
The final frozen campaign and protocol mutation run remain with HP1. [HP1]

## W77 idle upgrade and race runtime

The staged idle witness failed 5 of 50 isolated race runs. In both instrumented
failures, `a` redialed after `b.Hangup` and `b` punched. The assertion watching
`a` missed the new-circuit event. Both nodes now
track activity from startup. The test checks both punch logs over 30 idle
minutes, `a`'s log within one second of its activity, and either log within
30 seconds of reconnecting while both nodes are idle. All three fatal messages
are identical to the staged version. The same isolated race sample passed
50 of 50 runs after the fix (240.955 s).

All 21 `TestUpgrade*` tests passed in 123.069 s with the frozen 270 s Go
timeout. `TestOpenOutConnectionLossPreservesReplacement` passed under `-race`
with the ported `startOpenLocked(ps, false)` call. The Go mutant runner killed
MH17 with `H-17 ladder call while idle`. All 24 HP1 mutation entries and HP1r's
MR1 anchor apply. `python3 scripts/gate.py --base-guard` passed with the same
six existing overrides and zero new alerts.

The full race JSON trace reached Go's 600 s package timeout after 130
top-level tests passed. The four longest completed tests took 423.62 s
(70.6% of that budget). The main-line race replay finished in 568.305 s.
The same four tests took 430.07 s there, leaving only 31.695 s of total
package headroom before adding HP1.

| Test | Wall time | Seed runs |
|---|---:|---:|
| `TestLinkRTTFromProbe` | 167.38 s | 40 |
| `TestLinkLevelRelayedUnknown` | 120.80 s | 20 |
| `TestLinkLevelIgnoresReordering` | 71.61 s | 20 |
| `TestLinkLevelBurstyWiFi` | 63.83 s | 20 |

All 100 seed runs already use `forSeeds` with a `synctest` bubble per seed.
They simulate 60–80 s of bidirectional traffic at 8–10 ms intervals per seed
through QUIC. The fake clock advances between packet, timer and stream work,
which still executes under the race detector. `TestUpgradeByzantineReconnect`
had been running for only 1 s when the package alarm fired and passed in
3.50 s in the earlier targeted race run. The timings identify no wall-clock
sleep to convert in the dominant tests. Their seed counts, traffic and
sampling windows are part of the coverage. [Go synctest documentation](https://pkg.go.dev/testing/synctest)

The longest completed loopback test was `TestLinkProbeStaleOpenAttempt` at 4.18 s.
It uses real loopback sockets, waits for the next probe tick and observes a
2 s quiet window. `TestSendHoldsTheWriterLockOfTheStreamItWritesOn` took
1.04 s and `TestBulkLoopback` took 1.38 s, both on real loopback sockets.
Those three tests contributed 6.60 s. The probe and writer-lock witnesses
are named in the frozen HP1 regression criterion, which exercises their
transport behavior on loopback.

The root-module `test:ci` command uses `-short -timeout=8m -covermode=atomic
-coverpkg=./...` for p2p. The nested `protocol` module has a separate race run.
The candidate's p2p package passed with the root CI flags in 100.331 s
(141 tests and two fuzz seed suites). The same four probe tests took 55.76 s
in aggregate. GitHub's `ci` job has a 35-minute cap. The main-line job in
run 45067621739 took 17m28s and its p2p package took 273.871 s, ending on a
`TestOpenWindow` assertion. The measured runtimes fit both limits. This
candidate's CI timing was measured locally at package scope. [GitHub run 45067621739](https://github.com/oioio-space/inkwell/actions/runs/45067621739)

Local evidence is under `~/.cache/inkwell/codex-w77/` (`b-before-alone.log`,
`b-trace.log`, `a-full.log`, `b-full.log`) and
`~/.cache/inkwell/hp1-w77-final.0Z6EnJ/` (`gate-final.log`, `replacement.log`,
`mh17.log`, `idle-50.log`, `ci-p2p.log`, `base-guard-final.log`).

## W77 HP1m Go mutants

Codex, 2026-11-22. The selected HP1m mutations ran sequentially with the
frozen runner and its timeouts. The sandbox disallowed registering a
worktree, so the archives were reconstructed from Git exports. The older
HP1 source checkout was also copied for comparison.

| Source | MH6 | MH13 | MH15 |
|---|---|---|---|
| Pre-port HP1 source checkout on `c560c9a`, fork `c37a183f` | Passed | Ticket failure | Passed |
| `hp1-final.patch` on `a541db5`, before the `openOut` port | Passed | Ticket failure | Passed |
| `hp1-final2.patch` on `21eb2e9`, before the `openOut` port | Passed | Ticket failure | Caught |
| Staged HP1 on `731886c`, after the `openOut` port | Passed | Ticket failure | Caught in replay, passed in frozen gate |

- **MH6 (a).** Removing the epoch check in `Policy.Result` lets an old
  completion change the current policy. The instrumented original run
  applied zero completions from an older epoch across all 20 seeds. One
  trace had an immediate Busy result at epoch 2, and one-shot traffic could
  also make the link direct. The revised fixture holds the old call until
  the replacement circuit has a sync stream, then releases its
  service-closed result. The replacement scheduler must act before the test
  enables direct traffic. All 20 mutant seeds failed with
  `H-6 direct after the new circuit`, with both links still relayed.
- **MH13 (a, corrected by Claude the same day; the earlier "b" verdict was
  wrong).** Removing the first membership check in `beginRound` lets a
  stranger take a grant. On the responder path `filterRemote` still checks
  membership before the reply CONNECT, so the old fixture saw zero
  address-bearing replies and failed only on the ticket. On the initiator
  path, `beginRound` is the only guard: `exchange` writes our CONNECT before
  it reads anything. `autoPunch` takes that path for every inbound circuit,
  and a stranger let in by the invitation window can open one. So the mutant
  is not equivalent, and the old fixture never had a stranger that dialled
  the node. The fixture now adds a second stranger, `s2`, that dials a
  circuit to the node and answers its rounds with `rawMember.answer`, which
  counts the node's CONNECT. The address assertion runs first, because an
  admitted round also leaves a ticket or a slot. The ticket assertion also
  covers `s2`. The test name, both messages and the single-edit mutant are
  unchanged; no compound mutant is needed. Replays on a copy: MH13 failed
  10/10 with `H-12 rounds with a stranger: 1 CONNECTs with our addresses`,
  and MH22 failed 10/10 with `H-12 ticket for a stranger`.
- **MH15 (a).** The mutant waits on the call's `done` after cancelling it
  in `abort`. A close during handover can leave the member loop handing off
  and the dial loop exiting without ownership. A controlled replay of
  that schedule passed with the mutant. Close took 1 ns and the fake call
  unwound five seconds later. Keeping the member loop as the sole driver
  made the mutant fail with
  `H-14 Close returned after 5s with a call in flight, want ≤ 1 s`.

`python3 scripts/gate.py --base-guard` passed with the six existing overrides
listed above and zero new alerts. Go vet and the HP1 mutation anchor check
also passed. All three named tests passed unmutated under the race detector
in 19.504 s on the revised candidate.

Claude's follow-up after the MH13 fixture fix (scratch logs under → HP1
`/tmp/claude-1000/…/scratchpad/`, not kept): the frozen runner
`mutants.py HP1 MH6 MH13 MH15 MH22` reports all four caught, each with its own
message. Replays on a copy of the tree: MH6 failed 100/100 seed runs (5 × 20
seeds) with `H-6 direct after the new circuit`. MH15 failed 20/20 with
`H-14 Close returned after 5s`. MH13 and MH22 each failed 10/10, as above.
Unmutated, the three named tests passed `-race -count=20` in 304.979 s. The
gate's sim command (`-race -run '^TestUpgrade'`) passed in 95.843 s. The base
guard passed with the six overrides above and no new alert. Codex's MH6 and
MH15 fixtures are kept: each is now the only path to its mutated line, and both
are deterministic under synctest. No frozen gate needs a superseding gate.

Raw output is under `~/.cache/inkwell/hp1m-mutant-audit.bwwoZO/`.
The historical verdicts are in `pre-port.log`, `archive.log`, `archive2.log`,
and `current.log`. See `handover-results/` for the forced schedule,
`revised-mutants.log` and `revised-results/` for MH6 and MH15, and
`mh13-control-results/` and `mh13-compound-results/` for MH13. The race
run is in `revised-green.log`.

## W77 stopped call across loops

A reviewer suspected a race in `upgradeStop`, which clears `u.events`. The race
is real, and it is one path to a broader defect.

**Reproduction.** `TestUpgradeNewCircuitWhileLoopStops` (internal, synctest)
replays this interleaving, which the locks allow:

1. Member loop A has a punch in flight over the old circuit and has decided to
   end (`memberLoop` deletes `n.mup[p]` under `mu`).
2. The new circuit's `memberUp` starts loop B (`upgraderLocked`, `drivers` = 2),
   and `upEvent(E7)` queues E7.
3. A's `undrive` runs `upgradeStop`, which clears E7.
4. B restarts the policy before the cancelled call returns.
5. The call's result lands in the same epoch and sets a ladder. The idle app
   holds that ladder call back (H-17), so the link stays relayed.

Before the fix the test failed 50 of 50 runs (`-race -count=50`). As an
experiment, keeping the events instead of clearing them made it pass.

**Root cause.** The same result also lands without any E7. The new pure check in
`TestPolicyDirectUnwantClose` was red before the fix: stop, restart, then a
Spent result woke the policy at +60 s. Every loop change goes through
`upgradeStop`'s `Step(Obs{})`, and so does a dial-to-member transition. The Go
policy discarded a stopped call's result only if it was still inactive when the
result came. The plan's state table says « an InFlight call is cancelled, its
result discarded ». The TLA+ model agrees: `React` marks the call stale on a
stop, `StaleIgnored` holds, and mutant M2 `ApplyStale` breaks it. So the model
already covers the case; no model change is needed.

**Fix (`protocol/directup/policy.go`).** A stopping step marks the call in
flight `stopped`, and `Result` then only frees the slot. This replaces the
`!p.active` test. The two are equivalent wherever the old one held: a call
starts only while active, and leaving active with a call in flight now sets
`stopped`. `upgradeStop` still clears the queue; its comment now says why
dropping E7 there loses nothing.

**Checks after the fix:**

- New test: 50 of 50 passed.
- Gate sim command (`^TestUpgrade`): 22 of 22 passed in 124.4 s at rest
  (`heavy.sh --exclusive`). An earlier run hit the 270 s deadline inside
  `TestUpgradeGateRefusesSpam`. It was mid-loop at 48 s, during another agent's
  24-worker CPU burn (load 27); at rest it took 17.5 s.
- `protocol/directup` and `protocol/conformance`: pass.
- gremlins `./directup`: 53 killed, 2 lived (the two bucket survivors above),
  efficacy 96.36 %.
- `gate.py --base-guard`: pass, the six overrides above, no new alert.

**HP1 mutants.** 21 of the 22 Go mutants (MH1..MH22, the set `gates/HP1m.toml` runs) are caught. MH6 is not.

MH6 deletes the epoch check in `Result`. Its named test,
`TestUpgradeAfterRestartDuringAttempt`, restarts b. The old circuit closes, the
member loop stops, and the call becomes `stopped`. On that path the stale result
is now discarded by both guards, as in the model (`ep \/ stop`), so MH6 is
equivalent there. The epoch check is the only guard when E7 arrives with no stop
(a new circuit while the session stays relayed). There MH6 is caught by the
pure `TestPolicyEpoch`, with the same message `H-6 direct after the new
circuit` (run by hand on a copy of the edit).

The frozen entry's `test` field is not changed here: re-pointing it, or a
fixture with E7 and no stop, is the integrator's decision (→ HP1).

MN1 and MN2 (e2e) failed at `go build` of the rsync copy (`error obtaining VCS
status: exit status 128`), before any test ran. That failure does not depend on
this diff.

They are the command `./scripts/heavy.sh go build -tags server -o <copy>/bin/inkwell-server ./cmd/inkwell`, run
by `run_e2e` in the rsync copy of the tree. `gates/HP1m.toml` does not run them; `gates/HP1n2.toml` (MN1) and
`gates/HP1n3.toml` (MN2) do, under `heavy.sh --exclusive`.

**Integrator's option for MH6: a restart with no stop (tried, then dropped).** The ordering does occur in the
product. When b crashes, it sends no QUIC close. The relay keeps b's old connection, and so a's old circuit, until
it times out, while b's new process (on another endpoint) brings a new circuit. a's member loop still has a
circuit, so it never stops. E7 is applied, the old call is cancelled, and only the epoch discards its result.

In that ordering, though, H-6's 3 s bound does not hold, and the reason is not the policy. A `crashed` subtest
(old endpoint blacked out, `crashRestart` on endpoint 3), traced call by call, showed the following:

- a's outbound sync stream stays on the dead old circuit.
- libp2p's one-shot DCUtR, started for the new circuit, opens its stream on the dead old circuit too.
- That one-shot stays « active », so every call of ours is Busy (`ErrHolePunchActive`).

Direct came 33.8–38.9 s after the new circuit, on all 20 seeds; that is the old connection's timeout. Closing the
old process just after the new session (before or after the old result) still gave 6.8 s: Busy, then `T_busy`.

The graceful restart passes for the same reason in reverse. Its call 2 is Busy too, and the link goes direct only
because the test's puncher lifts the direct-path block while libp2p's own one-shot is still running. Keeping
≤ 3 s in a crash would mean choreographing around libp2p's one-shot, so the subtest was dropped and the test
files are unchanged.

New defect (→ HP1, W77 crash restart): after a peer crashes and restarts, the node's session stream and DCUtR
stay on the dead circuit until the relay's timeout (about 35 s), although a live circuit to the new process
exists. Its root (stream and connection choice, `stream.go` and libp2p's `NewStream`) is outside this diff.

## HP1c step 1 (H-20, P-15): specification and models (2026-11-23)

Spec §8.6 (H-20; H-15 waits for P-15), §8.7.4 (P-15 with the fallback exemption), §8.7.5 (row « connexion du flux
fermée faute de preuve »), §8.7.9 and §16 (row « connexion supplantée »); pure decision `protocol/syncack.Supersede`
with `vectors/supersede.json` (14 vectors, decoder `sync_supersede`), `FuzzSupersede`; models `DirectUpgrade.tla`
(S19, S20, M31, M32, three witnesses) and the new `Supersede.tla` (MS1-MS4, three witnesses). Where this departs
from the plan, and why: `docs/autonomous-decisions.md`, row « HP1c step 1 ».

**Found on the way (root cause, fixed):** `TestLimitsMatchSpecification` (protocol) was red on this worktree before
any HP1c change. HP1's staged diff added four §16 rows (« montée en direct … ») without their entries in
`protocol/testdata/limits.json`, the reviewed fixture the test binds row by row to `limits.go`. HP1's bounds are
`protocol/directup` constants, not `limits.go` ones, so the four rows enter the fixture with no constant, as four
qualitative rows already do. `gates/PU1c.toml`'s `limits` criterion runs this test.

**Base guard** (`python3 scripts/gate.py --base-guard`, after `git add -A`): the five HP1 overrides above, and one
new alert:

| Guard ID | Reason |
|---|---|
| `88ea645962f0094d9c6a877492df20b62fad218803f627f901b45d91a499973e` (replaces `50e72a02…`, the same file before M33) | `scripts/tla-check.sh` registers block HP1c's 18 TLC runs (M33 `HP1cResolveNoWait` added on 2026-11-23, see below) (named `HP1c*`, outside `^DirectUpgrade`, `^HP1r` and `^PeerUpAck`, so HP1's 44, HP1r's 3 and PU1's 19 frozen counts stay exact) and assigns them to CI2 shards (`HP1c[!S]*` on 1/3, `HP1cSupersede*` on 3/3, plan 2026-11-22-ci2-formal-shards.md); no existing run, shard assignment, assertion, TLC argument or verdict rule changes. |

### HP1c step 1, gates (2026-11-23)

- Model mutant M33 `HP1cResolveNoWait` (`ResolveWaits <- ResolveNoWait`, H-15 waits for P-15's proofs) and
  invariant S21 `ResolveKeepsLive`, listed in `DirectUpgradeCrash.cfg` and `DirectUpgradeProofTwoCircuits.cfg`: the
  guard of `Resolve` had no mutant. `Resolve` now keeps `circuits \cap {keep}`, identical under the guard (no ghost is
  left, so keep is live); the sound state spaces are unchanged (Crash 317,878/80,974, ProofTwoCircuits
  155,160/42,140). `INKWELL_TLA_ONLY='^HP1c' tla-check.sh`: 18 ✓, exit 0; M33's counterexample is Dial(LR),
  RCrash(RL), Resolve keeping the ghost LR. `tla_shards_test.py` passes (`HP1cResolveNoWait` on shard 1/3).
- Gates written: `gates/HP1b.toml` (supersedes HP1: MH6 repointed to the pure `TestPolicyEpoch`, same text; the → HP1
  block-HP1 entry of `scripts/netsim/mutants.toml` changed in place, so HP1m runs the pure mutant and HP1b's
  `mutant-tool` table checks it; HP1's own `mutant-tool` now fails by design, HP1 is history), `gates/HP1c.toml`,
  `gates/HP1n4.toml`. Mutant entries MP1-MP4 (block HP1c, anchors in `protocol/syncack/supersede.go`) and MN3 (block
  HP1n4); MH23 is frozen in HP1c's table only (its anchor is step 2's H-20 code; `mutants.py` has no deferred anchors).
- Base guard after `git add -A`: the five HP1 overrides above and one new alert, `88ea6459…` (`scripts/tla-check.sh`, reason in the
  table above). It is recorded in `gates/guard-overrides.json` (decision: the plan; the guard accepts only a `docs/` path), so that
  `mise run gate -- HP1b` can start; `gate.py --base-guard` then passes.

## HP1c step 2 (H-20, P-15): code (2026-11-23)

Plan `docs/plans/2026-11-23-hp1-reformalize.md` §2; decisions in `docs/autonomous-decisions.md`, rows « HP1c step 2 ».

- **H-20** (`dcutr.go`, `upgrade.go`): `punchOn` binds each DCUtR call to one circuit. Its rounds open on that
  circuit with `newStreamOn`, and the call ends when the circuit closes. The one-shot uses the circuit that
  caused it. A policy call uses the circuit recorded with E7 (`upEpoch`, `roundCircuit`), or, once that one is
  closed, the newest open inbound circuit. `circuitTo` and `cmpConn` keep the newest connection. A call waits for
  identify on its circuit before its preliminary direct dial (own root cause, decision row).
- **P-15** (`supersede.go`): the node keeps `syncack.Conn` per registered connection and only runs what
  `syncack.Supersede` decides. The proof is a `/ipfs/ping/1.0.0` stream on the chosen connection, bounded by
  `T_vie`. H-15 (`resolveDuplicate`) waits while a proof is pending or silent. The ping streams per peer are
  raised to `maxPeerConns` (own root cause, decision row; `TestMaxPeerConnsIsLibp2pLimit`).
- **Netsim** (`scripts/netsim/run.sh`, `frontend/e2e/netsim.spec.ts`): `NETSIM_VARIANTS=crash` for scenario 9
  writes `9-crash-s<seed>`, and without a variant the names are unchanged (`9-s<seed>`; dry selection checked).
  `restart-bob.sh crash` sends SIGKILL, then holds UDP 7419 with a placeholder. Clauses C1c, C2c and C3c are
  counted from bob's page readiness (15 s each). A miss is returned as data, never thrown (`mutants.py` counts a
  thrown error as a harness error). Not run here: it needs the machine alone (HP1n4).

**Red before the code** (the tree with the test seams `SetOnRound`/`SetOnProof`/`OutStream` only, seeds 0-1).
`TestUpgradeAfterCrashRestart` failed with `H-20 round on a dead circuit`, `H-6 direct after the new circuit`,
`P-15 dead connection kept` and `P-15 frames held`. `TestSupersedeKeepsLiveOlder/answered` failed with `P-15 live
connection not proved`, and `/fallback` with `P-15 dead connection kept` (the circuit open 1 s after the ACKs).
**Green after:** the HP1c `go` criterion by hand (`-race`, both tests, 40/40 seeds and both subtests, 46 passing
events); `mutants` (table, `--check`, MH23 and MP1-MP4 all caught by their text); `pure` (vectors, digest, 15 s
fuzz); `mutation` (gremlins `protocol/syncack`, 96.6 %: 113 killed, 4 lived, all in step 1's pure decision).

**Base guard:** two new alerts, each replacing an HP1 entry for the same file:

| Guard ID | Reason |
|---|---|
| `2080020ca018132adfc19bec82bf2923970977fa29e4e22efc9517cc93fbd6a9` (replaces `7d115488…`) | `netsim.spec.ts`: scenario 9's archived 600 s Playwright cap now also covers the crash variant (C1c, C2c and C3c, 15 s each); other caps unchanged. |
| `cea36854ce39c8e8eec86e1de0fd34852f411181c660e9fba158c5789df12d5f` (replaces `eb44f5f1…`) | `run.sh`: the crash variant of scenario 9 (SIGKILL, placeholder on the old UDP port); the graceful restart, the run names without a variant and the existing selection are unchanged. |

### HP1c step 2, cross-family code review (Gemini, 2026-11-23)

| Finding | Verdict | Evidence (red before) | Fix (green after) |
|---|---|---|---|
| 1 [Blocker] a local proof failure read as silence | real | `TestSupersedeLocalFailureNoVerdict`: « P-15 live connection closed on a local failure »; pure `TestSupersedeKeepsUnjudged` red on the old switch | `proofUnjudged` gives the pure state `Void`, so the connection is kept. The ping budget is derived from the rules (`pingStreams`); the wire is unchanged |
| 2 [Major] `proving` stuck after a silent proof | real (liveness of H-15) | `TestSupersedeSilentProofEnds`: « H-15 waits for a finished proof » | `proving` means a pending proof only; `silent` circuits are left out of H-15 and of `observe` |
| 3a [Major] a close executed after the newer connection was lost | real (TOCTOU) | `TestSupersedeCloseRechecksNewer`: « P-15 sole connection closed » | decided on libp2p's open connections; `superseded` re-decides right before each `Close` |
| 3b [Major] backward propagation of `newerAcked` | real, reproduced | `Supersede(8 s; C1 silent, C2 silent acked, C3 newest unacked)` closes C1 | **not changed**: frozen corpus digest and TLA run list; see `docs/autonomous-decisions.md`, row « HP1c review 3b » |
| 4 [Minor] churn | not a defect on its own (it needed 1) | `TestSupersedeChurnOneProofPerConnection` green before and after | no change: one proof per connection, a pending one never restarted |

Also found: hand-built test `Node`s had no P-15 state, which the new code paths dereference; the fixtures now get `newProofSet()`, and `silent` takes the peer it already knows instead of asking a fake connection.

### HP1c2 step 1, review finding 3b (2026-11-23)

The rule, not just the code: P-15's fallback exemption is lifted only by a newer connection still judged alive
(acknowledged, its proof not silent; the newest, its proof dropped, is never judged). Spec §8.7.4 (French sentence
« Seule une connexion plus récente encore jugée vivante lève cette attente »).
- Vector `supersede_dead_middle_acked_keeps_older` (15th, generator `protocol/cmd/vectors/supersede.go`, status
  `proposed`). It is red on the step-1 decision for the right reason: `TestSupersede` and `TestCorpus` give
  `expectation close: got [1], want []`. The pure Go decision changes at the code step, after the freeze.
- `Supersede.tla`: `NewerAcked` excludes a silent connection, `AckWaitOK` re-checks it at the close, and
  `FallbackKept` reads it then. Mutant MS5 `HP1cSupersedeBackwardAck` (`AckWaitOK <- BackwardAckMut`) runs on
  AnyTiming's constants with `Conns = {1, 2, 3}` and is caught by `FallbackKept` (32 s). It is registered in
  `tla-check.sh` (CI2 shard 3/3, pattern `HP1cSupersede*`). Residual → HP1: the sound rule is not model-checked on
  three connections. An exhaustive AnyTiming run on three had not finished after 15 min, past the gate's 1000 s budget,
  so it was stopped and `SupersedeAnyTiming.cfg` is back to two connections.
- Gate `gates/HP1c2.toml` supersedes `gates/HP1c.toml` (method v2 rule 9). The two files differ only in the header,
  `raw_results` (`gates/HP1c2/*`), the corpus (15 vectors, digest `75cf22a6…`, the new vector required) and the TLA run
  list (19 ✓, `SupersedeBackwardAck FallbackKept`). The caps are unchanged (2520 s ≤ 2700 s, G-1).
- **MS4 redefined (coordinator's decision A, HP1c2 not yet frozen).** With the 3a/3b close re-check, `AckWaitOK`
  requires a newer connection that is usable, still judged alive and acknowledged. That implies a newer usable one,
  so `FallbackKept` ⇒ `NeverCloseSole`, and MS4 as frozen in HP1c (`SoleOK <- ProveSoleMut` alone) became an
  equivalent mutant: TLC reported « mutant passed ». MS4 now mirrors the reference decision's MP2 (the newest is judged
  like the older ones, its own ACK counting: `SoleOK <- ProveSoleMut`, `AckWaitOK <- OwnAckMut`), and `NeverCloseSole`
  catches it in 2 s.
- **Three connections, sound.** No exhaustive run fits. Honest on three: 11.1 M distinct states after 330 s, queue
  growing. Honest with one crashing side: 6.6 M after 300 s. AnyTiming with no close: 6.8 M after 300 s. So
  `HP1cSupersedeThree` uses TLC's simulation mode on AnyTiming's constants with `Conns = {1, 2, 3}` (seed 20771,
  depth 60, 100,000 traces per worker): no error, 5,979,513 states, 52 s. The same simulation catches
  `HP1cSupersedeBackwardAck` in 3 s (6,823 traces), and under Honest bounds with one close the 3b case is not even
  reachable (both older connections would have to die). The new `simulate` mode in `tla-check.sh` puts the run on CI2
  shard 3/3 (`HP1cSupersede*`). The exhaustive check on three connections stays a residual → HP1.

### HP1c2 code step, review 3b (2026-11-23, after HP1c2 was frozen on main, e29bbe6)

`protocol/syncack/supersede.go`: `newerAcked` grows only by a newer connection that is acknowledged and not silent
(`c.Acked && st != Silent`); the newest still counts by its own ACK, since it is never judged. The node's close
re-check (`superseded`) calls the same decision, so the two cannot diverge. The 15th vector,
`supersede_dead_middle_acked_keeps_older`, goes from red (`close: got [1], want []`) to green; the 14 others are
unchanged. MP1-MP4's anchors are untouched (`closes[i] = newerAcked` included).

### HP1b `regression`: TestUpgradeDialsOnlyAdmitted (2026-11-23)

« H-16 on-demand grant: 0 of the member's announced addresses dialled » failed in 14 of 200 runs of the test alone
without `-race` (0 of 200 with `-race`, whose scheduling hides it). The cause predates the HP1c code: in a scratch
copy, reverting tonight's changes one by one or all together still failed (9 to 21 of 200).

Root cause, traced in the punch gate: the one-shot's preliminary dial starts when libp2p's `IdentifyWait` ends. The
punch gate admits the dial's destinations from the node's own record of the peer's identify (`n.heard`), which
`onIdentified` writes from identify's event, asynchronously and after that wait. The first destination then asked
the on-demand grant with `announced 0`. The grant is taken once per preliminary dial, so it admitted no address,
and every later destination was refused.

Fix (product, `dcutr.go`): `punchOn` starts the call only once the node has handled identify on the call's
circuit (`identifiedOn`: closed by the completed event of that connection, or by a failed identify with the peer),
in place of the libp2p wait in `connect`. 0 of 200 failures without `-race` and 0 of 200 with it. Entries are
dropped with the connection (`identifyForget`), and none is made for a closed one.

### HP1b `regression`: TestUpgradeOneCallInFlight (2026-11-23, found by the rerun)

« H-4 only 2 calls: the upgrader stopped » failed in 29 of 200 runs (3 of 200 with all of tonight's changes reverted,
1 of 200 with only H-20's binding of a call to its circuit reverted). Root cause, traced: the test's fake puncher did
not take the DCUtR service's one-call-per-peer place. With H-20, a call ends when its circuit closes, so each of the
test's seven circuits starts its own libp2p one-shot. Those one-shots ran their rounds during the fake call; the
rounds were booked to it, so it was classed Spent and the policy moved to its 60 s ladder. A real call and a one-shot
exclude each other (H-4), so the node cannot reach that state. Before H-20, the first, unbound one-shot never ended
and kept every later one Busy, hiding the overlap. Fixture fix: the fake call runs through `Node.AsCall`, which is
the service's exclusion. The residual (about 1 in 200) had a second, older cause: with an outbound circuit only, the
upgrader's call is a direct dial (H-3), which the fake puncher never saw. The « upgrader stopped » check now counts
every upgrader call (`SetOnUpgradeCall`), while the in-flight check still counts the fake calls. Neither assertion is
weakened: H-4 counts calls, punch or dial. 0 of 400 without `-race`, 0 of 200 with it. [done]

### HP1m mutants MH4 and MH14 survived the fixtures above (2026-11-23), corrected

- **MH4** (the policy launches a call while one is in flight) survived `Node.AsCall`. Under the service's exclusion,
  each new circuit's one-shot (H-20) holds the service, so every fake call returns Busy at once and two calls never
  overlap. A fixture that enforces the exclusion hides the product's own H-4 check. `AsCall` is removed. The test
  now isolates the upgrader: libp2p's one-shot is off on both nodes (`SetNoAutoPunch`, a test seam in
  `autoPunch`), so no other DCUtR call runs whose rounds could be booked to the fake call. The fake call is again the
  plain one, counted where the upgrader starts it. MH4 is caught. « the upgrader stopped » still counts every
  upgrader call (`SetOnUpgradeCall`).
- **MH14** (no settle check) survived because the test's power depended on punch timing. Measured with the mutant:
  6 of 20 seeds caught it on the pre-tonight tree (rebuilt from `77122d2` and
  `~/inkwell-archive/2026-11-22/hp1-final5.patch`); 0 or 1 caught it on the current tree; no single revert of
  tonight's changes restored 6. Cause, traced: only the first move failed, and a second direct connection of a
  simultaneous punch then moved the session (`onConn` → `maybeUpgrade`, MOVED 46 ms after the failed one), so the
  settle nudge was never needed. Fixture: the stale direct connection now fails every move until 10 s after it
  exists, as a stale connection does, and only H-13's settle (15 s) can move the session. The assertion and its
  text are unchanged. MH14 is caught.
- **MH23** (HP1c2: rounds through `host.NewStream`) survived after the H-16 identify fix. Measured: no round ran after
  the crash in any of the 40 seeds, so no round could land on the dead circuit. The one-shot's direct dial now gets a
  grant with b's new addresses, and b's own H-3 dial had already opened its NAT, so the link went direct without a
  punch. Before the fix, the race gave that grant no address, which is what forced the rounds. Fixture: a's punch
  slots are held by other peers when b's new process comes back (`FillSlots(4)`), and freed 500 ms after its
  circuit. The direct dial is then refused with no packet (H-16), the rounds wait for a slot and run on the new
  circuit. Rounds now run in 28 of 40 seeds. MH23 and MP1 are caught; the test's bounds and texts are unchanged.

### HP1r mutant MR1 survives: an equivalent mutant on the test's paths (2026-11-23), gate defect, not changed

MR1 removes H-19's late-link check (`if n.lateLink(c) {` → `if false {`). With it applied, `TestUpgradeLateLinkClosed`
shows no « H-19 late link kept » on the current tree (0 catches). With only H-20's binding of a call to its circuit
reverted (`punchOn` without `connContext`), it shows 5. Since H-20, a DCUtR call ends when its circuit closes. The
one-shot started on the dropped circuit is cancelled, and its delayed direct dial with it, before the dial lands, so
the closed arms never produce a late link. No revert of the identify wait, of P-15 or of the newest-connection
choice changes that (0 catches each). H-19's guard is now redundant on these paths. It still guards late links that
no bound call can cancel. The clearest one is the answerer's punch dial: `answer` runs its round on the service's
context, bound to no circuit, and dials once the initiator's SYNC is read. Proposal (needs a superseding gate, since
HP1r freezes the test's arms): a closed arm whose Drop lands after the SYNC, while the answerer's dial, delayed by
`setDirectDelay`, is in flight, with MR1 caught there by the same text. → HP1
- **HP1r2** (2026-11-23) supersedes HP1r (method v2 rule 9). `diff gates/HP1r.toml gates/HP1r2.toml` shows only:
  the header (why, the evidence table above), `raw_results` (`gates/HP1r2/*`), and the new closed arm
  `closed_answerer_dial` in criterion `go` (its description and its place in the arm list). The caps are unchanged
  (2190 s ≤ 2700 s; `gate.py`'s G-1 check: ok). Red for the right reason, before the arm's code: HP1r2's `go`
  criterion run verbatim fails with only `missing or not passing: TestUpgradeLateLinkClosed/closed_answerer_dial`;
  every other arm and the 40 `TestWantStopsAndDrop` runs pass. The arm's test code comes after the freeze. → HP1
- **HP1r2 code step** (after the freeze on main, 5d6c325): arm `closed_answerer_dial` of `TestUpgradeLateLinkClosed`.
  a wants b and dials its circuit, so b holds the inbound circuit and initiates; a answers. a's upgrader is off
  (`SetLocalReady` false), so a never dials b directly and the link needs a round. A new test hook (`onAnswerDial`,
  called where the answerer starts its punch dial) triggers a's Drop 10 ms later, while that dial (on the punch
  service's context, bound to no circuit) is in flight on the 150 ms direct delay. With MR1 applied, only this arm
  fails, with « H-19 late link kept (answerer's dial) »: a's outbound late connection is kept and a announces b. MR1
  is caught.

### HP1n4 mutant MN3 not caught: C1c and C2c start their clock at page readiness (2026-11-23), gate defect

`gates/HP1n4/netsim-mutants.log`: « mutant MN3 -> C2c: NOT CAUGHT (C2c is pass: 0.7 s after bob's page was ready) ».
Cause: `crashClauses` (`frontend/e2e/netsim.spec.ts`, ~1321-1323) counts C1c and C2c from `tReady`, taken only after
bob's page shows « Synchronisé » (a wait of up to 60 s). Under MN3 (P-15 off) the ~30 s spent on the dead connection
is absorbed by that wait. The dead connection is the fresh direct QUIC one: bob is SIGKILLed 0.8 s after the pair
went direct (`bob.log`: LinkDirect at t=1795422444628, restart at 1795422445383), the socat placeholder holds it
silent, and it dies by quic-go's 30 s idle timeout.

| Run | tReady - tRestart | C1c (from tReady) | C2c (from tReady) | Source |
|---|---|---|---|---|
| seed 20771, unmutated | 6.6 s | 0.1 s | 0.7 s | `gates/HP1n4/netsim.log` line 117 |
| seed 20772, unmutated | 6.2 s | 0.7 s | 0.7 s | line 143 |
| seed 20773, unmutated | 6.1 s | 0.7 s | 0.7 s | line 167 |
| seed 20772, MN3 | **31.2 s** | 0.1 s | 0.7 s | `.task/netsim/results/9-crash-s20772.json`, `.task/netsim/9-crash-s20772/` |

The discriminating number is tReady - tRestart, which no clause bounds. In the MN3 run, bob's link reads LinkRelayed
every second from 1.0 s to 29.0 s after the restart. It first reads LinkDirect at 30.0 s (t=1795422475424), before
the page readiness (31.2 s) and the convergence (31.9 s).

- **HP1n5** (2026-11-23) supersedes HP1n4 (method v2 rule 9). C1d and C2d start at `tRestart` (when
  `restart-bob.sh`'s kill and respawn return), with « Synchronisé » part of what they measure, 15 s each. Measured
  from the restart: unmutated, direct at 6.7, 6.9, 6.8 s and converged at 7.3, 6.9, 6.8 s; MN3, 31.3 s and 31.9 s.
  C3c is unchanged. Seeds, variant and order are kept: HP1n5 replaces HP1n4, last. MN3 now expects **C1d**: its own
  observable (bob's link direct) is 30 s late with or without the readiness term. C2d's texts are typed only once
  the page is ready, so its lateness is inherited from readiness. MN3 keeps block `HP1n4` in `mutants.toml` (as MH6
  keeps block HP1 for HP1b); its tuple is now `("e2e", 9, "crash", "C1d")`. HP1n4's own table check no longer
  passes, and HP1n4 is not run again. Caps 240 + 1300 + 30 + 30 + 900 = 2500 s ≤ 2700 s (`gate.py`
  `deadline_problem`: none).
- **Red for the right reason**, before the spec change: HP1n5's `netsim` parser, run verbatim on HP1n4's three
  strict result copies (`.task/hp1n4-results/`), exits 1 with only « missing ['C1d', 'C2d'], red [] » for each seed.
  Its table check passes, and `mutants.py --check HP1n4` reports « edits apply ». `netsim.spec.ts` changes after the
  freeze. A miss there, « Synchronisé » not shown in time included, is returned as data, never thrown.
- HP1n's C1 has the same clock defect (archived C1 counted from readiness, its text from restart, § « Port onto
  PU1 »). It is filed in the plan's §6 → PC1b.
- **HP1n5 code step** (after the freeze on main, 3d1a5da; `gates/HP1n5.toml` is byte-identical to it):
  `crashClauses` in `frontend/e2e/netsim.spec.ts` now takes `tRestart` only. It polls bob's status for
  « Synchronisé » within the 15 s from the restart (a non-throwing `until`). Once the page is ready it types on both
  sides, then waits for the direct link (C1d) and the two-way convergence (C2d) within what is left of the same 15 s.
  A miss, readiness included, is a measured `fail` (« « Synchronisé » not shown 15 s after bob's restart », or the
  path or « not converged »), never an exception, so `mutants.py` counts it as a catch. C3c is unchanged. C1c and
  C2c are removed: only HP1n4 read them, and HP1n4 is superseded (HP1n, HP1n2 and HP1n3 read C1, C2, C3, C3a and
  C4-C6). The graceful variant still waits 60 s for « Synchronisé » before C1 and C2, as HP1n freezes.
  Checks: Biome clean; `playwright --list` lists the 9 scenarios; no type error in the spec (`tsc` on it alone);
  HP1n5's `netsim` parser on dry outputs: exit 0 with C1d/C2d/C3c passing, exit 1 on a C1d miss (« red ["C1d fail:
  …"] ») and on a non-zero run.sh exit. Netsim not run here (the integrator runs the chain on the machine alone).
  **Base guard:** one new alert, `6989446145e9…` « frontend/e2e/netsim.spec.ts: timeout increased:
  test.setTimeout(#_#) », the same 600 s archived cap as entry `2080020c`, whose fingerprint the edit changed.
  It is recorded in `gates/guard-overrides.json` (decision `docs/objectif-en-cours.md`). `gate.py --base-guard`:
  pass.
