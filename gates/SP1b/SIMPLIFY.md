# /simplify — block SP1b (interim send path, supersedes SP1 and WB) (2026-11-09)

Scope: the whole uncommitted diff of the block — `internal/transport/frame` (WriteIdle 90 s, FloorRate 2 KiB/s,
MaxOneWayDelay), `internal/transport/mux` (partialTTL derived, 120 s; expiry wake; sendFragmented floor),
`internal/transport/p2p` (our own yamux config `muxer.go`: ConnectionWriteTimeout 120 s, window pinned 256 KiB;
FloorRate on sync/1 and bulk; rule-L test premise), the RuleSP1 tests, `scripts/netsim/mutants.toml` (SP1a-e),
`frontend/e2e/netsim.spec.ts` (NI-3 C1 strict, C2 known), `go.mod` (go-yamux/v5 made a direct dependency, version
unchanged). Two reviews (reuse + simplification, efficiency + altitude), triaged against CLAUDE.md, the frozen gate
`gates/SP1b.toml`, the TLA+ model `SendPath.tla`/SP1 configs and decisions D-SP1-1..4.

## Applied

- Constant relations by construction: compile-time assertions `muxWriteTimeout > frame.WriteIdle` and
  `PartialTTL > frame.WriteIdle`; `PartialTTL = WriteIdle + 2·MaxOneWayDelay + slack` (still 120 s, D-SP1-4), so a
  later change to WriteIdle cannot silently reopen the partial-drop loss the model found.
- `StallReading` panics if it cannot take the whole receive budget (a silent no-op would let the rule-L test pass
  without proving anything).
- Reassembly expiry wakes at the oldest partial's deadline: a partial is freed at PartialTTL, not up to twice it.
  New `TestRuleSP1PartialFreedAfterTTL` (red before: still held just after the TTL); `TestStalePartialExpires`
  tightened to `PartialTTL + 1 s`.
- Tests: the WB leftovers folded into RuleSP1 (`TestRuleSP1SlowLinkDeliversWithoutResend` kept at mux level — only
  it catches SP1e; `TestRuleSP1DripReaderCutWithinHoldBound`); `fragChannel` instead of a duplicate wire byte; one
  `delaySends` wrapper, `recvFor` beside `recv`, one `slowConn`; p2p tests read constants from `export_test.go`;
  the two NoFalseCut frame tests table-driven; the stopped-reader loop tidied; keepalive wait derived from the
  constants. `TestRuleSP1WindowPinned` keeps 2 MiB (1 MiB would not catch the auto-tune mutant SP1b).
- Comments: one sentence per constant with a pointer to the design doc (SP1-n); `WriteProgress`'s parameter renamed
  `floorRate`.

## Declined

- Absolute cap on a message's total send time, and per-peer reassembly accounting: spec changes. At the 2 KiB/s
  floor a 128 MiB message may take ~18 h and a drip-feeding peer can hold its own sender and one partial (global cap
  128 MiB) for 120 s. Recorded as residuals for ADR 0034 steps 4-6 (credits bound the in-flight bytes per peer).
- Removing `muxKeepAlive`/`EnableKeepAlive`: they restate libp2p's defaults on purpose — D-SP1-3 makes the keepalive
  part of the spec, and `TestRuleSP1MuxConfig` guards it against an upstream default change.
- Replacing the `libp2pyamux.Transport` conversion: it is go-libp2p's own pattern (`type Transport yamux.Config`);
  the resulting values are asserted by a test.

## Rule L test premise (decision, main session)

`TestRuleLSendFailureResyncs` keeps its name, property and assertions (Resyncs 1, DownsTotal 0, the change sent
again) but reaches rule L as SP1 defines it: the peer's application stops reading while the connection stays up, so
the write fails at WriteIdle with a live connection. A 92 s blackout would now outlast QUIC's idle timeout (a real
outage, outside the v1 contract per the SP1 model). Mutant RL1a still caught.

## Evidence

- E3 (scenario 7, seeds 1-3): paste delivered at the first attempt (94-95 s), every token arrives, 0 outages, 0
  resyncs; typing worst 44.8 / 53.3 / 49.0 s (NI-3 C2 known failure, gate SP1b).
- Mutants SP1a-e and RL1a caught; `mise run tla` 62 runs green; lint clean.
- Pre-existing go-libp2p `Swarm.nonProxyAddr` nil-deref panic in `p2p` race runs (≈2 in 5 on HEAD
  too): fixed, see « Dependency fix found by the gate » below.

## Mutation

No `protocol/*` package is touched; gremlins step 5 does not apply. The rules are proven by the code mutants
SP1a-e (gate criterion `rule-SP1-mutants`) and the TLA+ mutants.

## Dependency fix found by the gate (2026-11-09)

The SP1b `done` run failed at step 5 on a pre-existing panic (≈2 in 5 `p2p` race runs, also on HEAD before SP1):
go-libp2p v0.51.0 `Swarm.nonProxyAddr` dereferences a nil transport when the hole puncher's detached dial worker runs
after `BasicHost.Close` (`Swarm.close` sets the transports to nil). Our own dials never reach that path (`WithNoDial`),
and the worker is not reachable through any public API, so no ordering in our `Node.Close` can fix it. No upstream
release fixes it (v0.51.0 is the latest). Fix (user decision): a one-commit fork `oioio-space/go-libp2p`
(`inkwell/v0.51.0-nilfix`, cae49c8), pinned by a `replace` in go.mod, with an upstream regression test
`TestAddrsForDialForceDirectAfterClose`; reported upstream: issue libp2p/go-libp2p#3575, PR #3576. Evidence:
`TestWantAgainClearsResetHold` 30/30 race runs (panicked within ~2 before), `p2p` 10/10 race runs, govulncheck clean.
Remove the replace when the fix ships upstream.
