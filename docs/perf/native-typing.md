# PERF-TYPE-native: measurement contract (2026-11-07, v6)

The 2026-11-07 decisions in `docs/objectif-en-cours.md` govern the harness
(the last section, "décisions de l'utilisatrice (banc PERF-TYPE natif, après
l'optimisation)", replaces the NULL rule, the keystroke start and the cold budgets).

| Population | Handler | Frame work | Pause |
| --- | --- | --- | --- |
| warm (every keystroke after the first) | p95 <=8 ms (objective 4 ms, not gating) | <=16 ms **for every frame**, and p95 <=16 ms | none >50 ms |
| cold (first keystroke of a fresh process, including the first preview presentation) | p95 <=50 ms | p95 <=50 ms | none >70 ms |
| instrument | overhead (probes on minus off): one-sided 95 % upper bound <=1 ms | | |

The handler budgets apply to the raw values and to their conservative upper bounds.
The evening decisions of 2026-11-07 set the warm handler gate to p95 <=8 ms and the
cold pause budget to 70 ms (the first preview presentation, ~50 ms, happens once per
launch). The 4 ms objective is reported next to the verdict as
`objective_warm_handler_ms` and `objective_met` (warm p95 <=4 ms); it never fails the
gate. The report gives `warm_max_gap_ms` and `cold_max_gap_ms` and their upper bounds.
Every other rule (controls, coverage, cadence, isolation) is unchanged.

**Missed refreshes** (decision "soir, 2", 2026-11-07; Go `internal/nativeperf/missed.go`,
published as `missed_refresh`):

- Cold: in each fresh process, one pause is the first preview presentation and falls under
  the startup exception: the longest missed span whose upper bound is <=70 ms, together with
  the overlapping spans (frame and heartbeat views of the same pause), is not counted
  (`cold_startup_exempt`). Any other missed refresh of a cold trial still fails.
- Warm: a trial misses if it has at least one missed refresh. The gate fails only if the
  editor's proportion is significantly higher than that of the NULL trials with recording
  hooks on, in the same evidence: one-sided Fisher exact test, p < 0.05. Both rates
  (`editor_rate`, `null_rate`), their counts and `p_value` are published.
- NULL trials' own missed refreshes no longer block certification; the instrument is
  certified by the overhead bound alone (<=1 ms, one-sided 95 % upper bound).

Tests and builds do not certify native performance; L3 remains blocked until the
real host produces complete evidence with every oracle validated.

## Keystroke start

`keyDownAt` is the **first `performance.now()` read in the window capture keydown
listener** (`frontend/src/perf/native.ts:capture`, installed before App by
`nativeEarly.ts`). The keydown handler record and the outer `keystroke-task` bracket
start at that same read, so `handler.start >= keyDownAt` and `docChangedAt >=
keyDownAt` hold by construction; Go also rejects a keydown record whose start is not
`keyDownAt`. `event.timeStamp` drifted from -4 to +5 ms against `performance.now()` on
the host, and up to +24 ms in the 18:10 run (GDK/X event time base): it is kept only
as the diagnostic `keyEventTimeStamp`. The beforeinput/input `eventTimeStamp` values,
and the Event Timing entries correlated with them, stay on the `performance.now()`
base; the paint upper bound is measured from `keyDownAt`.

## Native input, focus and permission

The default injector is Mutter's private `org.gnome.Mutter.RemoteDesktop` API,
verified on this host with GNOME Shell 50.6. One owned Gio D-Bus connection creates
and starts one session before the app is launched, then sends evdev keycodes
(`NotifyKeyboardKeycode(ub)`: KEY_LEFTSHIFT=42, KEY_X=45) for the entire run.
Every press has a release attempt in `finally`. Cleanup always attempts `Stop`
and closes the owning connection, including failed Start/input and interrupted
collection. Missing API, failed Start/Stop, a closed session or lost delivery is
a failure. There is no portal, restore token, approval dialog or automatic XTest
fallback. `--input-backend=xtest` explicitly selects the legacy injector and may
prompt on each run. The first default run requires no approval dialog.

The driver explicitly launches GTK with `GDK_BACKEND=x11`. GNOME's session type
does not establish the application's backend. Activation must find exactly one
managed X11 window for the measured PID; no match (including a native Wayland
window), multiple matches, a different XID/PID or wrong focus aborts injection.
Mutter can inject into native Wayland surfaces, but the harness retains X11 for
independent target verification: it has no external Wayland focus/PID verifier.
Activation verifies `_NET_ACTIVE_WINDOW`; each probe and measured key verifies
the measured PID, exact active XID, focus ancestry and refresh before and after
injection. Renderer visibility/focus and trusted delivery remain mandatory.
The measured window's centre selects a `GdkMonitor` through GTK4, read by the
external ctypes driver. Its nominal refresh, physical geometry and XID/PID are
written into the fresh private profile. Go reports the nominal refresh and XID/PID;
the full geometry is also retained in the driver's `NATIVE_TARGET` log. There is no assumed
60 Hz fallback or additional cgo exception in application code. The driver checks
the target's identity, focus ancestry and monitor refresh before real keys.

GTK initialization is process-local: Wails initializing GTK does not initialize
the Python driver. Only after the managed target window exists, the driver's main
thread calls `gtk_init_check()` and verifies `gtk_is_initialized()` before querying
the borrowed default display or any monitor. Initialization failure, a missing
target/display or a query from another thread fails explicitly. No GTK window is
created by this query; the borrowed display is neither closed nor unreferenced.

An injector's successful request return is **not** proof of delivery.
Only in the warm process, the driver probes with Shift_L until a trusted Shift
keydown reaches CodeMirror's content DOM. The Go handshake acknowledges that
proof before readiness; synthetic, outside-editor or out-of-order proofs are rejected.
This harness never treats a missing event as a success. Missing proof fails promptly
with an explicit permission/delivery reason and stops before cold replication.

The same Mutter session and D-Bus connection retain that verified delivery across the thirty
fresh cold processes. They receive **no preparatory key**: each cold process's
first text input is its measured sample. Loss of delivery or target validation
stops further injection. A performance-budget or positive-control failure with
structurally complete input still permits all cold samples to be collected.
Raw runs are preserved; driver metadata can add a failure, never grant a pass.

The previous host failures reached `armed`, which already required six rAF
callbacks, but recorded zero DOM keydowns. Their “frame heartbeat absent” reason
confused missing post-input frames with the continuous heartbeat. The watchdog
now distinguishes the two and records heartbeat age and input-target counts.
Those old logs alone do not establish whether GNOME denied delivery.

## Work coverage and cadence

The dedicated frontend installs inert hooks before importing App; recording and
controls require the native-only Go capability. Early window capture precedes
App listeners, and a MessageChannel task after input bubble closes the outer
keystroke bracket after all bubble listeners and their microtasks. CodeMirror's
forwarding recorder retains coarse stage diagnostics, without extra fine-clock
reads. Failure cleanup defers reconfiguration until dispatch has returned.

The last six rAF timestamps form a rolling pre-input baseline for **each** trial.
Go requires it to precede the key, be no older than twice its median period, and
not exceed the target monitor's nominal period by more than one clock quantum.
A slow startup baseline cannot excuse later missed refreshes.

Every rendering opportunity from input through settlement is bracketed by an
early rAF and a following task. A continuous MessageChannel probe with an 8 ms
timer yield supplies conservative queue-work bounds, including the gaps between
polls. Hooks also record actual App timer, interval, event, microtask, rAF and
worker/message callbacks. Go unions overlapping callback/render/dispatch spans
and sums disjoint work in each frame: two short tasks cannot hide an over-budget
total. Idle gaps between callback spans are not summed as work. Queue-window
(task-probe) bounds start at the keydown listener, never before it: the part of
a probe window before the key is the probe's own timer wait or earlier,
unrelated work. After the key they keep the probe's scheduler overhead, plus one
quantum, and may conservatively refuse a pass.

Each input stays open for at least 1200 ms plus the 100 ms Event Timing delivery
grace, longer than the ordinary projection debounce (300 ms in
`frontend/src/store/projection.ts`). There is no next-key debounce reset during
that window. The full tail and preceding inter-trial heartbeat history are
checked for missed refreshes, relative to cadence plus two observed clock quanta.
Missing rendering opportunities or discontinuous task-probe coverage fail as
uncovered. Retry/error paths outside this successful typing workload are not
claimed as measured. Raw intervals, callbacks and settle boundaries are retained.
The input capture snapshots both cadence and the pre-input heartbeat count;
an entry proven to precede input is not mistaken for a missing post-input frame
when WebKit assigns both the same rounded millisecond. Every post-input entry
still needs a corresponding work frame. Increasing the requested probe yield
from 2 to 8 ms cuts nominal polling frequency by 75%; actual scheduling remains
visible in the raw spans and all yielded time stays in the conservative bound.

The pause budget uses `gap + quantum`, while a control's >=50 ms gap witness
uses `gap - quantum`. The unrounded gap remains a diagnostic. No bound is
compared with a raised threshold, and a single warm frame over 16 ms fails even if
the warm p95 is below 16 ms. Cold first keystrokes are judged on their p95 (<=50 ms),
and their pause upper bound on 70 ms instead of 50 ms.

## Independent positive controls

Warm runs require all seven controls, followed by at least thirty normal inputs:

| Kind | Independent placement |
| --- | --- |
| handler | content-DOM input listener, 80 ms |
| layout | separately registered rAF, 80 ms |
| early-raf | rAF registered before the measuring rAF, 80 ms |
| measure | CodeMirror requestMeasure write phase, 80 ms |
| document | document-bubble input listener, 80 ms |
| task | separate post-input timer task, 30 ms |
| deferred | timer at +300 ms, 30 ms |

Each placement must exceed the work budget and be independently detected by the
frame/task and cadence oracles. Missing or unbracketed controls fail as
“uncovered or missed”; the control's own claimed duration cannot certify it.
The original handler/layout controls still require correlated Event Timing,
including the original processing/quantization checks. Event Timing also
corroborates the other early slow controls. A +300 ms control happens after the
input paint and is validated by the work oracle, not a fabricated Event Timing
duration. `controls` exposes the individual results.

Sixty trusted NULL keystrokes follow the normal warm inputs, in two interleaved
arms of thirty: recording hooks ("sondes") **on** (`null`) and **off** (`null-off`),
in ABBA blocks (on, off, off, on, ...) so that a linear host drift weighs on both
arms equally (`nullTrialKind` in Go, `null_schedule` in the driver). Both edit only an
isolated, focused `<textarea>` (WebKit's default action writes the key into its value,
which must equal the key). The CodeMirror document must stay identical (`Text.eq`; any
CodeMirror document change during a NULL trial fails at once) and no CodeMirror input
handler may run. The Automerge revision must stay unchanged, except for App work that
writes outside `text`: the first NULL trial is the run's first 2 s typing pause, so the
device claim (`claimActor`, `meta/claimedAt`, scheduled by `CLAIM_PAUSE_MS` in
`frontend/src/ui/history/history.svelte.ts`) lands in it. Such a trial records the
changed paths (`nullForeignPaths`, from `A.diff`); Go accepts it only if every path is
outside `text`. That first trial is in the "on" arm, so its cost can only raise the
overhead estimate.

- **on**: the full recorder, as for typing: window capture/bubble records, task hooks,
  stages, frame and 1200+100 ms settle recorder (`nullSamples` of each raw run file).
- **off**: `setNativeProbes(false)` from arming until the trial is cleared. The task,
  listener, observer, timer and message wrappers, stage/method/stream-send profilers and
  per-event records then run without clock reads, trace or records; the outer
  `keystroke-task` bracket is the only record (`nullOffSamples` of each raw run file; Go rejects an
  off sample with any other handler, task, listener or stage).
- Both arms keep the same two outer clock reads: the bracket start (keydown capture
  `performance.now()` + bounded fine read) and its end (closing MessageChannel task).
  The continuous heartbeat rAF and busy probe run in both arms, so their cost is not in
  the difference; the wrappers' bare call indirection also remains when off.

Go computes the outer bracket of each sample with the same `measuredHandlerWork`
in both arms and reports `instrument_overhead`: both means, their difference, the Welch
degrees of freedom, the one-sided 95 % upper confidence bound `diff + t(0.95, df) *
sqrt(var_on/n_on + var_off/n_off)`, and both n. Means are unbiased under the random
phase of the 1 ms clock. Fewer than thirty samples in either arm, or a bound >1 ms,
prevents certification. `null_handler` (on arm) and `null_handler_upper_bound` remain
diagnostics: the former "upper p95 <=1 ms" rule is withdrawn because a NULL key costs
about 1.3 ms of real browser work, which is not instrument overhead. NULL values are
NEVER subtracted from any budget or raw handler sample. The warm collection deadline
adds 60 times the existing 3 s trial allowance (Go: 3 s per trial, at least 90 s); each
trial retains its unchanged watchdog and settle period. Cold processes still receive
only one key.

Event Timing durations and paint upper bounds, and waiting for the next vsync,
remain diagnostics. Fast events censored by Event Timing's 16 ms minimum do not
invalidate normal samples; slow controls must supply their required witness.
`internal/nativeperf/testdata/` retains the actual WebKitGTK 2.54.1 control data.

## Clock resolution

The host's browser clock was observed at 1 ms resolution; the raw p95 is therefore
quantized, not a fine-time estimate. The selected source is `browser-bounded`:
two in-process browser reads at the outer boundaries, ZERO synchronous XHR in a
keystroke or frame. Certification compares the entire raw interval plus its
observed quantum to 4 ms. For q=1, raw 3 ms has upper 4 ms; raw 4 ms has upper
5 ms and FAILS. A true 5 ms handler cannot pass. Frozen clocks and q>1 ms fail.
There is no interpolation or claim that batching resolves individual p95 values.

Four batches of 1000 browser reads measure read-cost upper bounds including one
quantum per batch; `clock.precise.batchSize`/`readSpansMs` preserve them (the field
name is retained for protocol compatibility; its source declares bounded time).
Four preflight native-clock reads remain ONLY as a transport-cost candidate
diagnostic in `clock.candidates`, outside measured inputs. The native-monotonic
legacy verifier and its clock-disagreement rules remain strict for old raw runs.
The mocked comparison models a 2 ms RPC and a 1 us browser read: eight nested
reads cost 16 ms, two outer reads 4 ms, two browser reads 0.002 ms. This proves
operation count and simulated cost, not real-host performance. Actual read-cost
and NULL observations on the next host run determine whether overhead is small.
Batching native reads after the frame cannot recover earlier event boundaries;
it would not supply missing sub-millisecond timing evidence for those events.

`attribution_diagnostic` reports the raw bracket, union of actual recorded App
callbacks, inclusive CodeMirror dispatch and the post-input task tail. Nested
callbacks are never double-counted. The remainder is labelled
`unattributed_harness_browser_or_queue`, not pure instrument or application CPU.
The original outer bracket includes both microtasks and waiting for its closing
task; it stays conservative and is never replaced with a subtracted estimate.
See `2026-11-07-mutter-instrumentation.md` for evidence from the submitted run.

`attribution_table` (diagnostic, never a gate input) gives, per population (warm,
cold, null), each recorded callback's **self time**: window/document/element
listeners (with registration frames), Mutation/Resize/IntersectionObserver
callbacks, timers/microtasks/rAF (labelled with the recorded call that scheduled
them), Wails stream sends (`stage stream-send:<name>`), CodeMirror
dispatch/update/measure, each ViewPlugin update (`stage cm-plugin:*`) and the App's
per-change stages (Automerge, preview, `doc.text`, LSP didChange, problems,
`onState`). One synchronous trace links nested spans; self = clipped span minus
direct children. `unrecorded` = keystroke bracket minus the union of top-level
spans (browser dispatch/default action, style/layout, GC/JIT, promise
continuations, harness). With the 1 ms clock, row means are unbiased and add up to
the bracket mean; per-row p95 is quantized. The perf build alone emits sourcemaps;
`scripts/perf/sourcemap.py` fills each row's `source` after the merge.

## Host command

Tests and `--build-only` open no window. From a lit, unlocked GNOME session:

```sh
cd /home/mathieu/GolandProjects/inkwell/.claude/worktrees/l3 && DISPLAY=:0 GDK_BACKEND=x11 INKWELL_NATIVE_PERF_RUN=1 mise exec -- bash scripts/perf/type-native.sh --input-backend=mutter
```

The wrapper obtains the existing exclusive heavy slot and records its metadata.
The compact verdict is `gates/L3/PERF-TYPE.json` (status, reasons, thresholds, every
p95, maximum gaps); the gate manifest embeds it. The full diagnostics are in
`gates/L3/PERF-TYPE-detail.json`. Fresh raw runs remain under
`gates/L3-setup/native-raw-*`, listed in `raw_evidence` by path and SHA-256. Gate thresholds, blocking reason and production
asset selection (`inkwell.go: !perf_native`) are unchanged.
