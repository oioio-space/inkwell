# /simplify — RL1 (spec rule L on sync/1) with the NI0 work it rides on (2026-11-08)

Scope: the whole uncommitted diff after `809efec` — rule L fix (`internal/transport/p2p/stream.go`), link
metrics v1 (`internal/transport/link.go`, `p2p/links.go`, `p2p/linkstat.go`, `bridge/links.go`, memnet),
impaired-network tooling (`scripts/netsim/**`, `frontend/e2e/**`, mise tasks, CI job). Four reviews (reuse,
simplification, efficiency, altitude), triaged against the CLAUDE.md invariants and the frozen gates RL1/NI0.
Applied by two delegated agents (Go side, netsim side); every result re-run by the main session.

## Applied — Go

- `Send` doc comment was false after the fix: now says a failed write announces PeerDown then PeerUp (rule L)
  unless the caller cancelled.
- A rule-L resync is not an outage: `transport.Link` gains `Resyncs` and `Resyncs10m`; `DownsTotal`,
  `Downs10m`, `LastDown` count only real connection loss; `Quality()` ignores resyncs (documented).
  Tests: `TestRuleLSendFailureResyncs` (Resyncs=1, Downs=0), `TestLinkResyncIsNotAnOutage`,
  `TestLinkQualityIgnoresResyncs`.
- Cancel vs timeout: `watchCancel` (`context.AfterFunc` + atomic state) classifies a cancel that interrupts the
  write as a cancel and a cancel after the failure as a timeout (resync). Every canceller was traced and ends in
  a down/up or a teardown (`stopPeer` on PeerDown or before `Reset`, session end re-announces, `Close`).
  Test: `TestRuleLWatchCancel`. A boolean was preferred over `context.Cause`: only two classes are needed.
- Loss window resets when the chosen connection or loss scope changes (relay → direct):
  `TestLinkLossWindowResetsOnNewConnection`.
- `Links()` is read-only over sampled state; only the 1 Hz sampler writes loss and starts pings. `openOut`
  takes a first sample so the path is known at PeerUp.
- libp2p/QUIC reads moved out of `lmu` (`probeLink`); `applyLinkLocked` applies plain values. Lock order
  « mu then lmu, lmu never takes mu » commented in `node.go`.
- Nits: p2p aliases dropped; `measureRTT` owns `pinging`; only relayed links are pinged at open; no stats
  entry recreated for a gone peer, stale entries pruned after 10 min; `slices.Delete`; bridge `Links` looks up
  under `omu` and takes `r.mu` once; `sendTo` decrements pending counters on the ctx-cancel return; memnet
  leaves `LossScope` zero.

## Applied — netsim and e2e

- CI job loads `sch_netem`, checks nf_tables, fails hard when netem is missing; timeout raised to 45 min
  (decision Q2: impaired scenarios on every push).
- Scenario 6 C1 / scenario 7 C3 now mean « the transport connection is not torn down » (plan, Clarification
  after RL1); they stay `blocked` until `window.__inkwellLinks` exists — never faked.
- Oracles read the editor document state through the `VITE_E2E_OBSERVE`-only hook, not the rendered DOM nor
  CodeMirror's private tiles; poll loops evaluate in the page and return length+hash, not 100-300 KB per poll.
- Scenario 6 watch 120 s → 60 s (above the 35 s bound); scenario 5 C4 checked at the end of each phase, P1
  timer started once C1 sees the direct path; scenario 8 C5 baseline after a settle.
- Known failures: only measured misses are downgraded; an XPASS line reports a known failure that passes.
- One seeded rng per typist, so a seed replays its schedule.
- `mutants.py`: caught only on a real test failure (never a build failure or exit 124); no-op edits rejected;
  parents created; previous result removed; relay built from the mutated copy; `go build` under `heavy.sh`;
  timeouts on every subprocess. `run.sh`: topology failures fail fast. `.env.reference` sorted.

## Declined

- Prune bridge `peerCounts` on PeerDown: entries exist only for peers with failures, and pruning at PeerDown
  would erase the failure counter at the very moment rule L raises that PeerDown.

## Verification (main session, 2026-11-08 20:50)

- `heavy.sh go test -race -count=1 ./internal/transport/... ./internal/bridge ./internal/app`: ok.
- `mutants.py --check RL1` and `--check NI0`: every anchor found once, no no-op.
- `mise run netsim:mutants -- RL1`: RL1a caught (go test exit 1).
- `mise run lint`, `mise run fe:check` (1199 tests): green.
- Known, not introduced here: `p2p` panics about 1 run in 5 inside libp2p (`swarm.nonProxyAddr` in a synctest
  bubble); reproduced 2/15 on a clean worktree of HEAD. Tracked as a residual.

## Mutation

No `protocol/*` package is touched by this block; gremlins step 5 does not apply. Rule L is proven by the
RL1a mutant above.

## Second pass — Links notification (kind 20), added to unblock NI-2 C1 (2026-11-08 21:15)

NI-2 C1 (« the transport connection is not torn down ») needs the link metrics in the webview, so the
bridge notification Links (spec Annex A.1, kind 20) was added after the first pass, spec first. Four new
reviews (reuse, simplification, efficiency, altitude) over that diff only.

Applied:
- Change detection without a second encode: `Link.Stable()` (ages reduced to set/unset, owned by the
  protocol type) compared with `slices.Equal`; one encode, only on change.
- The conversion rules (saturation, ages, loss permille, sub-ms RTT) live in `protocol/bridge`
  (`Measures`, `LinkOf`, tested in `measures_test.go`); `internal/bridge` only maps `PeerLink` — no
  private copy of a protocol codec in inkwell.
- Sequential decoders: Go `reader` shared by `DecodeRoles` and `DecodeLinks`; TS `Cursor` with
  `peer()`/`count()` shared by `decodeRoles` and `decodeLinks`; field layout listed once (`figures()`,
  `FIGURES`), `LinkFixed`/`LINK_FIXED` derived from it.
- One entry check (`checkEntry`) for encode and decode; `appendPeer` shared by Roles and Links.
- Enum values equal the wire values: direct conversion, asserted at compile time.
- Known-kind table (`kindInfo`) replaces the `lastKind` range plus a special case.
- Initial links computed before `s.mu` in `Serve`; `Router.SetLinksPeriod` (0 = off) lets tests that are
  not about Links turn it off, instead of a hidden filter in the shared `fakeConn`.
- Removed the `EncodeLinksNotification` wrapper and a dead helper; `matchExpect` shared in conformance.
- Webview: `LinkView` as `Omit<PeerLink,…>` plus overrides, `of()` through a `Map`; `__inkwellLinks` and
  `__inkwellDocText` moved to `workspace/observe.ts` with the existing gate and a unit test.
- Hot path (`internal/bridge` in HOT_PATHS): the per-frame enqueue stamp. New `BenchmarkServeMessage`,
  HEAD worktree vs tree, `-count 10`: with `time.Time` +3.5 % (p=0.000), rejected; with an int64
  monotonic stamp `683.8n → 692.5n ~ (p=0.072)`, B/op and allocs/op unchanged — kept.

Declined (outside this diff or a later block, noted for W33 UI):
- Event-driven dirty signal instead of the 1 Hz sampler (spec bound is ≤ 1/s; the sampler is the source).
- Merging link state into the roster store the people popover reads: decided with the W33 UX design.
- Extracting an `outbox` type owning queue, gauges and failure counters (lot 8, with the p2p dupl).
- A drop-latest slot for Links sends to a stalled webview (rare; the session ends on a stalled conn).
- Quality computed in the webview: Go stays the single source (decision Q4 thresholds in `transport`).
- Lifting the `protocol/invite` reader into a shared package (touches invite; same pattern used locally).

Seen while checking: `commands.prop.test.ts` FMT-27 (fast-check) failed once and passed on rerun —
unrelated to this diff, tracked as a flaky residual.

## mutation protocol/bridge

Gremlins (`block_done.py` settings: `-count=1`, `-exec=scripts/memcap.sh`, coefficient 10): first run
67.9 % (Links was only exercised from `protocol/conformance`). In-package boundary tests added, each named
after its Annex A.1 rule (`TestLinksCountBound`, `TestLinksPeerLength`, `TestLinksDomains`,
`TestLinksOrder`, `TestLinksLength`, `TestLinksRoundTrip`, `TestLinksSaturationBoundary`,
`TestLinksLossPermille`, `TestRolesBoundaries` for the shared peer reader, …). Second run: Killed 71,
Lived 2, efficacy 97.3 %. Both survivors are equivalent:
- `envelope.go:94:32` — the capacity of `make` in `Encode`: a size hint, the output bytes are identical.
- `envelope.go:109:12` — `len(b) < 3` mutated to `<= 3`: a 3-byte frame has no room for a non-empty
  peer, so it is rejected one step later with the same `ErrMalformed`.
Capacity hints were dropped in the Roles/Links codecs (1 Hz, ≤ 1024 entries) where they only produced
equivalent mutants; `Encode` keeps its hint (hot path).

## Fix found by the gate (2026-11-08 22:10)

The L1 regression (`anti-weakening`: `gate-guard.py --self-test`) failed: the gate dependency walk now
reaches `scripts/block_done_test.py` (through `scripts/netsim/mutants.py` → `test:py`), whose fixture
used the made-up path `scripts/x.py`, and the self-test requires every dependency to exist. The fixture
now uses a real file (`scripts/gate.py`); the test asserts the same filter. Nothing weakened.
