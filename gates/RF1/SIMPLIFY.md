# /simplify — RF1

Diff : 1047d30..the worktree `inkwell-rf1` (main at the frozen gate, then step 1's uncommitted specification and step
2's code).
- Step 1: spec §16.1, `CHANGES.md`, the TLA README, `RelayFanout.tla`, its configurations and mutants,
  `scripts/tla-check.sh`, `protocol/testdata/limits.json`, the plan and its simulations.
- Step 2, production code: `protocol/limits.go` (23 constants; `MaxInboundConnections` and
  `MaxOutboundConnections` dropped), `protocol/admit/` (new pure ledger), `protocol/conformance/admit.go` (decoder
  `admit_ledger`), `protocol/cmd/vectors/admit.go` and `protocol/vectors/admit.json`;
  `internal/transport/p2p/admission.go` (new), `node.go`, `relay.go`, `dial.go`, `addrbook.go`; spec §21 (one
  sentence on the admission vectors).
- Step 2, tests: `protocol/admit/{admit,model,corpus}_test.go`, `internal/transport/p2p/admission_internal_test.go`,
  `fanout_sim_test.go`, the harness (`sim_test.go`: an end may share its external IPv4 with others at its own port),
  `upgrade_sim_test.go` and `export_test.go` (`LiftRelayIPLimit` and its three uses removed).
- Step 2, mutants: `scripts/netsim/mutants.toml` (MG1-MG6, tuples as frozen by the gate).

Reviewer: the step-2 session (Claude Opus), one pass over the whole diff for reuse, simplification, efficiency and
altitude, checklists of the `go-style-guide`, `use-modern-go`, `vuln-guard` and `secret-guard` skills. A second-family
review of the code is still to come (method v2). Every check was run on the final tree.

## Findings, applied

1. **The import check forbids `syscall`, which `time` and the `protocol` root bring in.** The first ledger took
   `time.Time` and read its bounds from `protocol`; `go list -deps ./admit/` then listed `syscall` and criterion
   `admit` failed. Root cause: the gate's import rule (« no I/O ») is checked on the dependency closure, and package
   `time` depends on `syscall`. The ledger now takes an injected integer clock (`admit.Nanos`, a monotonic reading
   the caller passes: `time.Since` of the layer's start in `p2p`), and its §16.1 configuration is built by its callers
   (`nodeAdmitConfig`, `relayAdmitConfig` in `p2p`, explicit bounds in each vector). The three origin prefixes it
   must know are its own constants, pinned to `protocol/limits.go` by `TestPrefixesMatchSpecification`. Its closure
   is now `errors, maps, slices, net/netip` and their runtime parts.
2. **Observer work on the hot path.** `OpenConnection` built a `Stats` snapshot (map clones) and the origin's string
   for every opening, only for the tests' observer. Both are now computed only when `observe` is set, and the
   observer is called after the lock is released.
3. **Every opening walked every bucket.** `expire` paid and swept all origins' buckets on each call, so an opening
   cost O(origins owed a token), which an adversary with many addresses grows. Buckets are now paid when read
   (`bucketAt`: tokens, refund, `Owes`), and the sweep that drops full ones runs at most once per refill period. An
   opening costs O(pending places), at most 48.
4. **Two one-line constructors.** `newResourceManager` and `newRelayResourceManager` only adapted the return type
   for `TestLinkProbeResourceLimits`; the test now adapts `newNodeAdmission` and `newRelayAdmission` itself.
5. **The node's copy of the member count.** `maxPeers` (64) was a private copy of `protocol.MaxConnectedMembers`; it
   is gone, and the address book uses the protocol constant (plan §6).
6. **Branches that only bred equivalent mutants.** `bucket.pay` returned early for zero periods (the general path
   does the same), and the per-member and per-relay maps were cleaned with `<= 0` where the count reaches 0 exactly;
   both simplified (`== 0`, no early return), and a clock read backwards pays nothing instead of panicking.
7. **The fan-out tests stopped at the verdict before the cause.** Under MG5 every member was refused by libp2p's
   rate limiter, the « not up » `Fatalf` fired first and the refusals were never reported as RF-5. The origin and
   refusal checks now run before the liveness verdict, so a failure names what the evidence shows.
8. **Lint.** `gocognit` on the conformance runner (split per event: `adRun.open`, `check`, `adMatch`), `unparam` and
   `maps.Copy` in the generator, `modernize` (promoted fields in `rate.SubnetLimit`, `reflect.Type.Methods`).

## Cross-family code review (`gates/RF1/CODE-REVIEW.md`, « refuser »): each finding reproduced by a named test

1. **(blocking) A Byzantine member starves an honest member's share.** `TestByzantinesCannotTakeShares`
   (`protocol/admit`) plays the reviewer's trace: 63 members each try a third connection, then the 64th, honest,
   asks for its two. It passes on the code as it was, both with the Byzantine members in the roster and outside it:
   `room` checks a third connection against `nMember + owed(p)`, and `owed` counts every roster member's unused share
   (model `SharesKept`). So the trace as written is **refuted**. The reserve is bounded by the membership: the roster
   is the wanted members (`Want`, filtered by `Admit`), at most `Members / Share` = 64 of them. A member the roster
   does not name has no reserve: that is the rule, not a gap. **The case the trace hides is confirmed:** a member
   admitted after the surplus was taken found the 128 full and was refused its share (`TestRosterGrowthShedsSurplus`,
   red before the fix: « sheds 0 surplus connections, want 2 »). Root cause: `SharesKept` held only for a fixed
   roster (the model's `Members` is a constant), and growing the roster broke it with no way back but waiting.
   **Fix:** `SetRoster` returns the member connections to close so that every roster member's unused share fits
   again: one at a time from the member holding the most above its own (its share in the roster, nothing outside),
   newest first. `Node.Want` closes them (`shedSurplus`). `FuzzLedger` now changes the roster at random, closes what
   is shed and checks S9 after each event. `TestRosterGrowthShedsAtLayer` checks the wrapper names the right peers.
2. **(major) An unknown peer holds a relay's per-IP places.** Pinned go-libp2p: the circuit-v2 ACL is checked per HOP
   message only (`p2p/protocol/circuitv2/relay/relay.go:196` reserve, `:296` connect), and `ReservationTTL` (1 h,
   `resources.go:50`) bounds reservations, not connections. An unlisted peer's connection stays open until the
   connection manager trims above 1 024 (`relay.go`, `relayConnHigh`). `TestRelayUnlistedHoldsNoPlace` (a stranger
   dials the Allow-listed relay once a second for six minutes from the members' address and keeps its connections
   alive, then the listed member reserves): red before the fix, with the stranger holding **9** of the address's
   places. They were not held indefinitely: in the simulated stack its connections ended after about 45 s, so the
   bucket (1 per 5 s) times that lifetime bounds them. **Fix at the root:** with an Allow list, the relay's gater
   refuses an unlisted peer at its handshake. It may neither reserve nor connect, so its connection serves nothing;
   it now holds only its handshake's place (peak 1). A 15 s hold for an open relay (no Allow) was rejected. There
   every client is unverified for as long as it is connected, and a deadline would turn §16.1's « connexions 64 par
   IP » into « 64 handshakes per 15 s ». On an open relay an address is one origin by definition; Allow is the
   protection (plan §4, RLY1).
3. **(minor) The sweep mutants.** `TestBucketsBoundedUnderChurn` (internal test: 20 000 distinct origins, one
   admission a second, a quiet minute every 1 000) bounds the buckets kept to 11. It **kills** the negation (`<`)
   and the inversion of the sweep condition: after a quiet gap neither sweeps again, and the map grows without
   bound. So the review is right that they were not equivalent. The boundary (`>`) and `now+l.swept` (a sweep on
   every call) keep the map bounded and change only when the sweep runs: those two stay equivalent.

Both behaviours are in the specification, not only in the code: spec §16.1 rows « connexions » (the shed) and
« circuits par relais » (an unlisted peer refused at its handshake), with `protocol/testdata/limits.json`, the plan
(RF-6, RF-7, the §4 residual with the open-relay measure, → RLY1) and `CHANGES.md`; the model `RelayFanout.tla`
(constant `Late`, variable `admitted`, `AdmitLate`, `ShedStep`), runs `RFRGrowth` and `RFRNoShed` (MFR1).

## Findings, rejected (with the reason)

- **`x/time/rate` for the bucket.** Rejected by the plan (§2, trap 2): `Reservation.CancelAt` restores nothing once
  its time has passed, so a member's paid token could not be told from its unpaid one. The ledger keeps debts.
- **Reusing `directup.Bucket`.** Same reason: it counts tokens, not whose they are; `Give` is the blind refund the
  model's mutant MF12 shows to break `RateBound`.
- **O(pending) counts per opening instead of per-origin counters.** At most 48 places at a node; counters would add
  a second source of truth for what `pend` already says. At the relay (no global pending bound) the count is over
  its unlisted live connections, bounded by libp2p's system limit; kept for the same reason.
- **Enforcing `MaxRelays` in `New`.** The ledger bounds the relays' connections (2 each, 8 in all); refusing a fifth
  configured relay at construction would change the API for no capacity the ledger does not already hold.
- **Running `TestMemberShareKept` and `TestNodeConnsBound` in the simulated network.** They are at the layer that
  keeps their defect class: the admission wrapper over libp2p's real resource manager, built by the node's own
  constructor (`newNodeAdmission`), at spec scale (128 member connections, 48 pending, 8 relay connections, each
  reserving its 1 MiB QUIC window: 184 connections and 184 MiB, plan §3). 64 hosts in the simulated network would
  add hours of real time and no rule.

## mutation protocol/admit

gremlins (pinned version, 4 workers, coefficient 10, `-count=1`), on the final tree (after the code review's
fixes), the gate's `mutation` command, run of record: killed 102, lived 5, not covered 25, timed out 3: **efficacy 95.33 %** (95.41 % on the run before)
(81.25 % at the first measurement; 86.49 % right after the shed of finding 1 was added, whose selection survivors
`TestShedPicksMostAboveNewestFirst` and `TestRosterBoundedByMembers` then killed). The 5 survivors are equivalent:

- the sweep condition `now-l.swept >= l.cfg.Refill`: CONDITIONALS_BOUNDARY (`>`, one tick later), and ARITHMETIC_BASE
  and INVERT_NEGATIVES on its `-` (both `now+l.swept`: a sweep on every call). Every read of a bucket pays it first
  (`bucketAt`), and the map stays bounded (`TestBucketsBoundedUnderChurn`), so neither outcomes nor memory change.
  The negation, which stops the sweep after a quiet gap, is killed by that test (review finding 3);
- in `shed`, `a > 0` as `a >= 0`: while excess remains, some member holds more than its own (the excess is at most
  the sum of what members hold above their own), and a member above beats one at zero, so a member at zero is never
  chosen;
- in `shed`, `p < best` as `p <= best`: two distinct map keys are never equal.

The timed-out mutant (`k == nil` negated in `Peer`) dereferences a nil connection; it is out of the score.

The `case` conditions of `refusal` are reported « not covered » (the gremlins limitation of CLAUDE.md, « Pièges
connus »); each is exercised by a named test at its bound (`TestOriginQuotaBound`, IPv6 /48, `TestGlobalQuotaBound`,
`TestRateBurstAndRefill`) and by the vectors. `errors.go`'s `iota + 1` and `Is` are not covered by mutation for the
same reason; `ReasonOf` is checked by every refusal test.

## Checks run on the final tree

Each gate criterion's command was run once, one at a time, outside `mise run gate` (results in the report of the
session); `python3 scripts/netsim/mutants.py --check RF1`; `golangci-lint` on the touched packages (0 issues).
