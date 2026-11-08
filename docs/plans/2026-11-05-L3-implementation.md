# L3 — Comprendre, corriger, produire

This worktree wires the pinned tinymist sidecar into the authenticated `lsp`
Stream, with completion, signature/hover help, and diagnostics in Relire.
Automerge ownership and revision conversion remain in the webview. The language
process uses only `internal/sidecar` (manifest SHA-256 verification and sandbox).

## Review correction pass

The earlier implementation and evidence were insufficient. In particular, the
four-line language benchmark measured warm-cache requests, the native readiness
criterion could pass without measurements, and the override reasons incorrectly
claimed approvals and reviews. Those claims are withdrawn. A later user decision rejects another deferral and blocks L3 until native measurement. The current steering instructions are
quoted in `docs/objectif-en-cours.md`.

Added gate-script overrides say only:
`new gate script for L3 (added file, no existing test/threshold/delay/reference changed)`.
The earlier deferral proposal was superseded by the recorded decision to **block L3**. No deferral is accepted.
`NATIVE-readiness` is removed: an exit code and a screenshot do not establish
native input-to-paint latency. The prior production build/launch and blank XGetImage capture are retained under `docs/perf/2026-11-05-native-readiness/` as historical
observations only: the user proved that XGetImage misses WebKitGTK content, not that the native application is blank. No native latency is inferred from those PNGs.

## Freshness and protocol

Versionless diagnostics from pinned tinymist v0.15.10 are rejected. Active compiler
diagnostics identify immutable compilation sources; the webview checks the current
text before anchoring and rechecks the opened target at navigation time. Tests
cover A→B→A source changes and a secondary file changed before the jump.

Language requests carry document, workspace and server-generation revisions.
Superseded requests are cancelled with `$/cancelRequest` and their results are
rejected. Slow **current** results are retained: the 100 ms threshold is a measured
performance requirement, not a timeout that conceals slow responses. Incremental
UTF-16 changes depend on the server's advertised sync capability. The relay's
method list is restricted to the methods the frontend uses; response tests check
that the absolute project root never reaches the webview.

## Interface

The status bar has a single problem button: click advances, Shift+click opens
Relire's list. F8 / Shift+F8 and list clicks navigate to the target range. Shared
messages and docstrings are rendered as text, never HTML or commands. The
counter's ordinary click is covered by the e2e alongside F8 and file/range jumps.
The existing preview e2e preserves its filename, line, caret and focus checks
while using the list's designed entry path; Relire is closed before cleanup.

## Snapshot and latency evidence

The review pass replaces repeated whole-project copying and hashing under the
project write mutex. A large-project Go benchmark records repeated `-benchmem`
samples before and after the change, compared with benchstat (`-count >= 10`).
The snapshot tests retain immutability, dependency-change detection, link refusal
and bounded fallback behavior. Stale `source-snapshots/` are cleaned at startup.
The final benchmark logs, comparison and limitations are retained with the L3
setup evidence; no speedup is claimed without the recorded comparison.

Both language harnesses use the same multi-file fixture with thousands of lines,
imports and edits interleaved with requests. Each measured operation must return
a non-empty, fixture-specific useful result. Reports retain real samples and
nearest-rank p95 for completion, signature and hover. The p95 limit remains
**100 ms**; neither slow responses nor failed measurements are discarded to make
it pass. Measurements run with the exclusive heavy-job lock.

## Reproduction and validation

Use `GOTMPDIR=/home/mathieu/.cache/inkwell/scratch` and
`XDG_RUNTIME_DIR=/tmp/inkwell-l3-runtime`; that runtime's `inkwell-heavy` symlink
points to `/tmp/inkwell-heavy` for shared locks. Using `/tmp` itself as the runtime
hides the mount probe's temporary path and causes sidecar exit 125. pnpm uses
`PNPM_HOME=/home/mathieu/.cache/inkwell/pnpm` and the writable store
`/home/mathieu/.cache/inkwell/pnpm-store` when installation is needed.

The final verdict is written by `mise run gate -- L3` in `gates/L3.json`, including
the L2 → L1 → L0 regression chain and unchanged archived reference checks.
`mise run lint`, `mise run test` and the frontend checks have separate logs in
`gates/L3-setup/`. Earlier gate results and warm-cache numbers are not acceptance
of this correction pass. Human code/UX review before merge remains outstanding.
No commit, stash or merge is performed by this work.

## Dependency cooling period

The Go proxy response for `golang.org/x/net@v0.62.0` reports
`2026-10-27T19:00:16Z`, origin `3de643db506517b3e652a13cc0b56034cea6b8e6`.
This is more than seven days before 2026-11-05; go.mod/go.sum pin v0.62.0.
Source: https://proxy.golang.org/golang.org/x/net/@v/v0.62.0.info.

### Large-project comparison from the correction pass

On Linux/amd64, `BenchmarkProjectionSnapshotLarge` uses 100 source files and
32 MiB of unchanged binary assets. Each iteration edits one source, creates a
snapshot, checks its identity, and removes the compile root. Setup is outside
`b.Loop`; `ReportAllocs`, `-benchmem` and ten repeated samples are used on both
sides via the existing `bench:baseline` / `bench:compare` tasks under the exclusive
heavy wrapper. The benchmark source and fixture are unchanged across the pair.

Benchstat: 63.018 ms → 1.813 ms/op (-97.12%); 147544 KiB → 467.4 KiB/op
(-99.68%); 7291 → 4787 allocations/op (-34.34%). All comparisons have p<0.001.
Raw samples and comparison: `gates/L3-setup/snapshot-bench-baseline.txt`,
`snapshot-bench-new.txt`, `snapshot-compare-task.log`. Reported B/s represents
logical project bytes covered, not bytes copied from storage.

The first ingest still reads changed/uncached contents; subsequent compiles
reuse private read-only content-addressed objects via hardlinks. Source inodes
are never linked, and source hardlinks remain refused. The dependency walk is
still O(number of entries); bulk reads, hashing and copies scale with changed
contents. Device/inode/size/mtime/ctime fingerprints invalidate same-size edits
with restored mtime on Linux/macOS; conservative fallback is used without a
reliable fingerprint. Snapshot work uses its own mutex, never the project write
mutex. Session leases preserve active processes while removing unleased stale
snapshot roots and cached objects. Unix leases are implemented; other platforms
retain conservative preview fallback, subject to the existing native N35 work.

### Realistic language workload

`testdata/perf/lsp/` contains four shared Typst files and 3,019 lines, including
imports and a charter-defined function. Forty actual call-expression edits are
interleaved with completion, signature and hover requests. Completion prefixes,
argument positions and hover positions vary; empty arrays/objects fail. Reports
record document revisions and query positions. Browser checks use actual
CodeMirror content and the Wails Stream. Timing covers request to useful response
after an edit; it does not claim typing-to-paint latency. Preflight p95 was
4.208/0.479/9.735 ms in Go and 14.2/1.7/2.4 ms in the browser. The final gate
regenerates these measurements with the unchanged 100 ms limit.


Help rendering also checks the frozen document, caret and target file instead
of the whole EditorState identity: eight focused tests retain a current result
after 150 ms and a diagnostic-only transaction, and reject text/caret/file
changes. Partial failed snapshots prune obsolete cache objects too; twelve
repeated failures remain bounded while an earlier active hardlinked snapshot
retains its original bytes. These corrections do not alter the successful
snapshot benchmark path or the LSP measurement limit.


## Native measurement harness (awaiting the external graphical run)

`mise run perf:type-native -- --build-only` compiles a dedicated `perf_native`
Linux/WebKitGTK binary and separate `frontend/perf-dist` assets. Normal builds
embed only `frontend/dist`; `production,perf_native` and server/perf combinations
are excluded. The frontend installs no observers, hooks or slow controls before
a challenge-bound native-only Go capability. It instruments native trusted DOM
input and the actual CodeMirror input transaction, including its update/plugins.

The external driver uses existing libX11/libXtst platform libraries (no program
to install), forces the X11 backend under the existing XWayland session, verifies
focus ownership independently and injects one ASCII key per armed/acknowledged
trial. Warm controls run first, followed by 30 warm normal samples. If evidence
collection can continue, 30 fresh processes/application-cache profiles provide
one first-key cold sample each. OS page caches are not evicted and that limit is
recorded. Visibility/focus loss fails; hidden windows are never measured as fast.

Raw handler segments, native event timestamps, document revisions, two rAF
arrivals, continuous heartbeat arrivals and engine EventTiming entries cross the
existing Wails stream. Go writes per-run raw evidence and recomputes the final
`gates/L3/PERF-TYPE.json`. Thresholds stay handler p95 ≤4 ms, frame-wait p95 ≤16 ms,
validated paint upper-bound p95 ≤16 ms, and no normal measured gap >50 ms.
Handler and post-handler forced-layout controls each induce 80 ms and must be
detected in their own raw timing windows; missed controls fail the oracle.

PM-4 is enforced strictly: rAF/double-rAF are diagnostic opportunities, never
paint proof. EventTiming is used only if actually offered and associated with
the trusted input and actual edit; missing/censored entries are not zero-filled.
Its 8 ms quantization is treated conservatively (+4 ms upper bound), so a reported
16 ms cannot certify a ≤16 ms paint budget. An engine without a validated oracle
fails E-2. The harness therefore does not promise that the current WebKitGTK can
pass; only the external run can supply real native samples, and a missing oracle
remains a blocking failure.

The initial build-only pass launched no native window. Without
`INKWELL_NATIVE_PERF_RUN=1`, the command uses the tagged Go verdict-only CLI,
writes a blocked/failed report with no invented samples and exits nonzero.
External command, from this worktree:

```bash
INKWELL_NATIVE_PERF_RUN=1 mise run perf:type-native
```

The same opt-in can be used with `mise run gate -- L3` on the graphical host to
run the complete gate. Pure protocol, hidden-window, invalid-oracle, control,
heartbeat and driver tests run without a display. The old screenshot-readiness
wrapper has been removed; historical captures are not acceptance evidence.

Primary oracle references: [Event Timing](https://www.w3.org/TR/event-timing/)
and [GDK frame clock](https://docs.gtk.org/gdk4/class.FrameClock.html).

### 2026-11-07 — native-host readiness correction

The failed external run `gates/L3/native-raw-1xw3tl9q/` did receive renderer
readiness. Go discarded it when GTK reported inactive, leaving an empty record;
the driver's success-only completion check then obscured that failure. Rejected
readiness is now retained, including the runtime oracle selection, and an early
failed completion preserves the collector's reasons. Early success still fails.

After the native capability challenge, the driver activates the matching PID's
managed X11 window with `_NET_ACTIVE_WINDOW`, a fresh X server timestamp, and
the XWayland `XSetInputFocus` fallback. Go waits for actual GTK visibility/focus
before granting the renderer capability; the driver independently verifies X11
focus before injecting each key. No keyboard input is injected during activation.
This fixes the reproduced GTK-inactive/renderer-focused discrepancy.

The actual WebKitGTK 2.54.1 runtime advertises Event Timing (`paintOracle:
event-timing`), so a surrogate based on rAF/MessageChannel is not substituted.
Upstream confirms [Event Timing enabled since WebKitGTK 2.52](https://webkitgtk.org/2026/03/18/webkitgtk-2.52-highlights.html).
The [API's threshold/quantization](https://www.w3.org/TR/event-timing/) still
limits certification of fast inputs: missing/censored records never become zero
latency. Actual trusted edit association and both slow controls remain required.

The newly authorized `DISPLAY=:0` probe succeeds inside this sandbox. Real
launches now establish GTK **and** renderer visibility/focus and exchange ready
and trial messages. However, the renderer supplies no initial rAF heartbeat;
the retained normal attempt `gates/L3-setup/native-raw-hniq2f8u/` fails explicitly, without any
samples. A separate software-rendering diagnostic under
`gates/L3-setup/native-software-diagnostic/` reproduces the missing heartbeat and
is not acceptance evidence. Graphics warnings are recorded, not treated as a
proven cause. The existing 3-second input deadline now also bounds waiting for
that first frame. Frontend failure frames remain open long enough for the Wails
stream to deliver them; immediate close had previously discarded the reason.
New tests cover the missing first frame, failure delivery, activation focus,
preparation ordering, retained rejected readiness and early failed completion.

The gate runner clears `gates/L3/` when rerun. That cleanup removed the earlier
raw-run directories, including the user-supplied one; its previously observed
fields are explicitly transcribed (not presented as the original artifact) in
`gates/L3-setup/native-original-observation.json`. New raw runs live in retained
`gates/L3-setup/native-raw-*/`, while the aggregate verdict stays at the required
`gates/L3/PERF-TYPE.json`. A regression test verifies survival of gate cleanup;
the gate hashes the retained raw evidence too.

L3 remains **BLOCKED**, with no native timing value invented. Run from this
worktree on the graphical host, with the normal renderer:

```bash
DISPLAY=:0 INKWELL_NATIVE_PERF_RUN=1 mise exec -- bash scripts/perf/type-native.sh
```
