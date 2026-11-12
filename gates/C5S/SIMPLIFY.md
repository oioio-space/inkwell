# /simplify — block C5S (link probe product code, link quality in three levels) (2026-11-10)

Scope: the product code of `/inkwell/linkprobe/1.0.0` (ADR 0036, accepted; spec §8.5), as listed below. The pure module
`protocol/linkprobe` and the spec were committed earlier (46947f6).

- `internal/transport/p2p`:
  - `probe.go`, the I/O layer on `linkprobe.Host`;
  - the changes to `node.go`, `relay.go`, `links.go`, `linkstat.go` and `stream.go`;
  - the netsim-only filler `fill_server.go` and `fill_default.go`.
- `internal/transport/link.go`, and `internal/bridge/links.go` and `linksnotice.go`.
- `internal/app/node.go` and `linklog_*.go`.
- The tests, including the type-aware architecture test.
- `scripts/netsim`: `topology.sh`, `run.sh`, `mutants.toml` and `c5model.py`.
- `frontend/e2e/netsim.spec.ts`: the new C5.
- `scripts/pkg/no-test-tags.sh` and `release.yml` (AC-14).

## How the block got here (root causes, each fixed at the root)

- **Implementation found two spec assumptions false**, and the spec was reformalized:
  - quic-go drops a packet that arrives more than about a round trip late before counting it. The figure is now
    defined as the share of our packets the peer's stack did not accept: those are resent, so for the person they
    are loss. P4 became reordering within quic-go's tolerance.
  - Typing gives about 8 packets/s, far below what the model needs. A netsim-only filler (`-tags server`) supplies the
    test traffic, and the guarantees were recomputed: 0 false failures in 3000 runs, every mutant rejected.
- **An independent security review said « fix first »**, with twelve findings, all fixed with a red test:
  - Relay role:
    - the client map (quadratic cost, role granted too widely) is replaced by the relay's own connection-manager
      tags;
    - the relay gets probe stream limits.
  - Probe streams:
    - an `openOut` race could leave a stream to a peer without a role; it is closed by an attempt counter;
    - connections opened before `Notify` are now attached;
    - the shared relay hop is computed once per step;
    - the relay shutdown order is safe.
  - `Node.Send`: the nil-dereference fix is completed. It holds the writer lock of the stream it writes on, which
    stops interleaved frames after a reconnect.
  - The architecture test is type-aware, with exact allow-lists.
  - A relayed link is always unknown. The slow-send signal stays separate, as « Lien lent » in the sync indicator,
    one sentence added to Annex A.1.
  - The server build cannot reach a release: AC-14 runs `go version -m` in the release job. The filler is QUIC-only
    and members-only.
  - The netsim step log never blocks: drops are counted, and C5 fails on any drop.

## Review (reuse, simplification, efficiency, altitude)

- **Kept:**
  - the Host stays I/O-free in `protocol/linkprobe`, and the I/O layer only feeds events and executes actions; there
    is one source of truth for every rule;
  - the filler is kept out of every default path by build tags, and a type-aware test checks it.
- **Declined:** a separate probe goroutine pool. The per-connection writer plus always-read reader is what spec §8.5.3
  requires, and it is bounded at one per connection.
- **Follow-up (spec):** the outbound writer has no deadline. A responder that stops reading pins the writer for the
  connection's life (the figure goes unknown). Spec §8.5 defines no bound, so none was invented. To specify: a reset
  after a bound tied to the PTO.
- **Follow-up (test):** the relay-role test drives `relayRoles` directly. An end-to-end test through `RunRelay` is to
  add.

## Evidence

- `go-product`: all 11 named tests pass, plus the new security tests.
- `go test -race` on `internal/transport/...` and `internal/bridge` passes; `mise run lint` is clean.
- Mutants: 12 of 12 caught. M5b and M5d to M5n end to end, M5i and M5j in Go.
- One `e2e-C5` repeat passes all six clauses, P4 included.
- The 30 repeats are run by this block's gate.

## mutation protocol/linkprobe

gremlins v0.6.0 from `protocol/` (block_done's command): `linkprobe` 138 killed, 3 lived, efficacy 97.87 %
(87.4 % for the module before); `conformance` 156 killed, 4 lived, 97.50 % (its 4 survivors are in the shared
`reference.go`, outside the link probe). The « not covered » `case` conditions of `level.go`, `host.go` and
`machines.go` are run by `TestCorpus` (a manual mutation of `level.go`'s good→fair case fails the package). The
three link-probe survivors are equivalent:

- `linkprobe/host.go:69` `r.n > 0` → `>= 0`: with an empty buffer the slow path copies the chunk into the buffer
  and decodes from it, the same messages in the same order; only the copy differs.
- `linkprobe/machines.go:217` `i < len(p.pending)` → `<=`: the loop always stops at or before the last outstanding
  PROBE, whose seq is the highest sent (an ECHO above it is a violation earlier), so `i` never reaches the length.
- `linkprobe/machines.go:255` `s1.sent < s0.sent` → `<=`: equal counters give ΔS = 0, already under MinPackets,
  so the figure is unknown either way.


## mutation protocol (other packages, outside this block)

The module-wide figure also counts survivors in `protocol/trust` and `protocol/invite` that predate this block and
are not touched by it: `trust` left block P3 at 92.4 % with 21 survivors by the user's decision of 2026-11-08
(`gates/P3/SIMPLIFY.md`, « mutation protocol/trust »; `docs/objectif-en-cours.md`), and `invite` (ticket, lobby) is
unchanged by C5S. They stay owned by their packages' blocks and the protocol audit PA; nothing here weakens them.

## Also in the range (tooling, its own commit)

`scripts/block_done.py` gained a preflight right after step 2 (the gate's base guard and the step-8 docs check), so a
run that would fail at its end fails in seconds. Found when this block's gate ran for minutes past a failing base
guard. Reviewed by the main session, pinned by `block_done_test.py`.

## Second pass: two root causes found by the gate's e2e-C5 (2026-11-10)

Seed 2, repeat 1 of scenario 5 failed C2 and C4; both causes were confirmed in the run's artefacts and fixed at the
source, bounds unchanged.

- **C2, texts never equal: a CodeMirror crash, older than C5S.** The amSync plugin ran its `whenWritten` callbacks
  synchronously from `flush()`, and `flush()` may run inside a CodeMirror update. The problem-marks callback
  (`App.svelte`) dispatched there, CodeMirror answered « Calls to EditorView.update are not allowed while an update is
  in progress » and disabled the plugin, so that editor stopped syncing. Fix: one rule, the callbacks run once the
  current call ends (a microtask), never inside it; the write itself stays synchronous. A first version guarded only
  the plugin's own update; it was replaced by the uniform rule, because `flushTyped` and `whenTypedWritten`
  (`language/extension.ts`, called from an update listener) reach the same path from other updates. Red test:
  « whenWritten callbacks never run inside a CodeMirror update… » (red: the plugin died, the second paste never
  reached the document). The test « whenWritten waits for the keys of every editor » now awaits the microtask; what it
  checks is unchanged.
- **C4, RTT below 0.8 R: two measurements of one thing.** The shown RTT was quic-go's smoothed RTT, which samples
  only the newly largest acknowledged packet (RFC 9002 §5.1) and, under the filler's dense traffic on a reordering
  path, keeps mostly the packets that overtook: 48–56 ms for 60 ms (58–62 before C5S). The probe already measures
  every echo for the level. Spec Annex A.1 now says one RTT per link: on lan/direct with the probe figure known, the
  window's low median (the level's value) and the low median of absolute deviations; otherwise QUIC, else ping
  (relayed included, where the probe sees the first hop only). `Prober.RTT` / `Host.RTT` reuse one `lowMedian` with
  the level; `applyLinkLocked` picks the source. Tests: `TestProberRTTLowMedian`, `TestProberRTTUnknownFigure`,
  `TestLinkRTTFromProbe` (direct: within [0.9 R, 1.2 R] on each of 20 seeds, while the median over the seeds of SmoothedRTT / R is below 0.95 — the estimator wanders sample by sample, a per-seed check failed once at 0.979 under the full suite's load; relayed keeps ping).
  The test's path (70 ms, 24 ms jitter) was chosen for the median's sampling noise, measured: on a 33 ms path with
  heavier jitter, 5 of 20 seeds left that band on both sides (median of ratios ≈ 1.0, matching a Monte Carlo of the
  median of 31 draws), not a bias. In scenario 5 (60 ms ± 10 ms) the noise is about 2 ms against a 12 ms margin.
- **Declined:** a vector for the RTT values (the corpus and the TypeScript runner assert level and loss only; Annex A.1
  is outside the public contract).

## Third pass: two defects found by the gate's second run (2026-11-11)

- **A test harness fault, not the product: e2e-C5 seed 2 repeat 8 timed out at 900 s.** The browser shared its network
  namespace with the WAN bridge and veths that `topology.sh up` creates; their settling made Chromium report
  `ERR_NETWORK_CHANGED` during Bob's page load, the stylesheet was never refetched, the toolbar covered « Partager »
  and the click retried for 15 min (confirmed in the trace). Fix at the root: the WAN switch lives in its own namespace
  `wan`, the runner's namespace holds only `lo` and never changes; `helpers.ts` `open()` fails at once, naming the
  URL, when a same-origin document, stylesheet, script or font fails to load (no retry, no timeout change).
- **Mutant anchors drifted with the RTT change** (M5b, M5h, M5j no longer applied), found only hours into the run.
  Re-anchored to the same edits; and, at the root, `block_done.py`'s preflight now runs `mutants.py --check <block>`,
  so a moved anchor fails `done` in seconds (`PreflightTest`, red without it).

- **Smoke after these fixes** (before relaunching the gate): netsim scenario 4 passes in 32 s (it had timed out at 300 s on the same `ERR_NETWORK_CHANGED` cause), scenario 5 seed 2 passes every clause; `mutants.py --check C5S` applies all anchors; base guard passes.
  (helpers.ts reformatted by Biome after the smoke run.)

## Fourth pass: the gate's fifth run (2026-11-11)

- **e2e-C5, 2 runs of 30 (5-s1-r8, 5-s3-r6), one step each at the P2>P3 drop: the test's truth, not the product.** A
  PROBE lost and resent at the end of lossy, high-RTT P2 had its echo accepted because step 3.6's PTO, read at the
  echo, was inflated by rttvar right after the drop; its `r` counted ~110 crossing packets. The product follows the
  spec, the error is inside its prudent bound. c5model now simulates that path (RFC 9002 estimator across phases, PTO at
  the echo, quic-go's resend threshold, head-of-line order) and derives R2's crossing term for transition labels from
  it. Both runs replay green with it, red with the old rule; the old rule fails 3 of 200 corrected-model runs. Spec
  « Exactitude » and ADR 0036 now say 15.4 ‰ (18.4 ‰ for a window starting within 2 s of a drop). Declined for now: a
  stricter step 3.6 (PTO at hand-over and at the echo, the smaller), which needs an ADR amendment, vectors and TLA+;
  filed as a follow-up.
- **regression: L1 anti-weakening (through RL1 > P3 > P2 > L2 > L1).** `gate-guard.py`'s dependency walk took a test
  fixture path named in `scripts/netsim/affected_test.py` (`scripts/perf/p.py`) for a dependency, and its self-test
  required every dependency to exist. A path only named transitively that is no file is not a dependency now; a
  criterion naming a missing script still fails; a self-test assertion pins it (red before).
- Model tests at rest: 24 tests, 597 s (gate bound 900 s).


## Fifth pass: a second-family adversarial review (Codex, 2026-11-12)

Three connection-identity races, each fixed at the root with a red-first deterministic test
(`links_race_test.go`); bounds unchanged.

- **A probe could outlive its connection** (`probe.go`): `attachExisting` snapshots connections; a
  detach running before the insert left the probe and its timer until shutdown. The runner now also
  ends with the QUIC connection (`qc.Context().Done()`) and unregisters itself.
  `TestLinkProbeEndsWithItsConnection`.
- **The level could describe another connection than the one carrying sync** (`links.go`): it took the
  first QUIC connection of the peer. Annex A.1 says the sync connection: `probeLink(id, out, …)` now
  measures `out.Conn()` (a relayed session: its hop, level unknown; a session on TCP or still on the
  circuit: no figure). `TestLinkLevelIsTheSyncConnections`.
- **A sample read before a reconnect could be applied to the new session** (`links.go`): the sampler
  is split into `readLinks` and `applyLinks`; each sample names the peer and the sync stream it was
  read for, and `applyLinks` (holding `n.mu` then `n.lmu`) drops it if `ps.out` changed.
  `TestLinkLevelDropsSampleOfEndedSession`.
- No defect found in the amSync microtask rule. The unbounded outbound writer stays a P3 follow-up.
- Checks: the three tests pass 3× under `-race`; `go-product`, lint and `mutants.py --check C5S` pass.
  The p2p package needs `-timeout 35m` under `-race` (1372 s); 126 s without.
