# Mutter run 1: instrument evidence and limits

Source: the user-provided `PERF-TYPE-run-mutter1.json` in the measurement
scratchpad, plus `gates/L3-setup/native-raw-89kyc13d/warm-01.json`.
This is a diagnosis of that failed run, not a new host measurement.

| Nearest-rank p95, ms | Warm | Cold |
| --- | ---: | ---: |
| Outer browser keystroke bracket | 13 | 33 |
| Keydown bracket | 2 | 5 |
| Input bracket | 5 | 21 |
| Inclusive CodeMirror dispatch | 3 | 15 |
| Closing-task timestamp minus input-bubble end | 5 | 8 |

These distributions overlap; their p95 values cannot be added or subtracted.
In particular, the outer bracket includes the MessageChannel closing-task wait,
two synchronous clock RPCs, browser work and the App. The CodeMirror bracket
also contains instrumentation overhead. With no NULL data in run 1, claiming
that either the entire 11.6/32.4 ms native-clock p95 or its difference from
CodeMirror is application CPU would be unjustified. Cold CodeMirror's inclusive
15 ms p95 is evidence that transport alone has not been proved to explain all
latency; only a new controlled measurement can separate the remaining effects.

The new verifier was also replayed headlessly against the retained raw directory,
writing `.task/perf/mutter1-instrument-diagnostic.json`. Across the combined 60
normal samples it reports p95: outer browser bracket 31 ms, recorded App callback
union 8 ms, inclusive CodeMirror 12 ms, inclusive Automerge stage union 6 ms,
language didChange 1 ms, and unattributed harness/browser/queue remainder 24 ms.
These are overlapping distributions, not an additive decomposition. In particular
the recorded callbacks do not cover every browser/CodeMirror internal callback;
the remainder cannot honestly be called pure harness cost. Replay still FAILS,
including the new explicit missing-NULL reason, and does not modify the host gate.

Handler-control rejection: browser boundaries 489..591 (102 ms), native stamps
691.378918..791.359639 (99.980721 ms). Their difference is 2.019279 ms; preflight
allows only 1 ms quantum + max 1 ms transport + 0.000001 ms clock resolution.
The verifier correctly refuses those inconsistent clocks, but that makes the
frame-work result zero and prevents control validation. The preflight's rounded
transport spans do not establish a per-keystroke transport bound. The new input
path makes no such RPC or cross-process clock-agreement claim; old native-clock
records retain their strict verifier and their failures.

Deferred-control rejection: the real 30 ms interval 8840..8870 is detected
(frame work 32 ms, dropped refresh true). Its preceding heartbeat at 8534 has
the exact same quantized timestamp as keydown 8534, but precedes the actual
input and has no post-input work frame. The old timestamp-only coverage check
calls it uncovered. Capture now records the exact pre-input heartbeat count;
tests still reject any missing post-input frame and an invalid claimed boundary.

Cold replicate 26 reports document change 490, input start 475, duration 16,
processingEnd 491. The duration bucket alone permits presentation 487..495;
this event cannot present before its own processing ends. Intersecting that
bucket with processingEnd minus the 1 ms timestamp uncertainty gives lower 490,
upper 495. The change at 490 is proved before presentation; a change at 491 is
still uncertain and fails. The diagnostic upper remains 495-469=26 ms, never
compared to the 16 ms work budget. Existing genuinely uncertain cases still fail.

The mock clock-cost comparison deliberately uses a known 2 ms RPC and 1 us
browser read. It proves 16 ms for eight RPCs, 4 ms for two RPCs, 0.002 ms for two
local reads, and zero input-path RPC calls in the selected implementation. It
does not purport to benchmark WebKit. Host preflight reports both the native
transport candidate and four 1000-read browser cost batches. The new browser
source does not invent fine precision: q=1 ms is added to each disjoint interval;
raw 4 ms therefore cannot certify <=4 ms. A real 5 ms handler still fails.

The new 30 trusted NULL trials preserve the same event/clock/frame/probe path
but edit an isolated DOM target and must leave CodeMirror unchanged. Their p95
upper bound must be <=1 ms before the instrument can certify the 4 ms budget.
There are no NULL results yet: the next host command is required. Neither NULL
cost nor an attribution remainder is subtracted from any gate sample. The
report retains raw typing metrics and separately unions actual App callbacks,
inclusive editor dispatch and the explicitly unattributed harness/browser/queue
remainder. The pause, frame-work and slow-control requirements remain in force.

## Mutter run 3: NULL fixed, per-callback attribution (2026-11-07 16:10)

Source: `gates/L3/PERF-TYPE.json` from the host command below, raw evidence in `gates/L3-setup/native-raw-hg4c62db`.
Verdict: **fail** (unchanged budgets). Warm handler p95 9 ms, cold 29 ms; frame work upper p95 warm 18, cold 34 ms; max gap 49 ms (upper 50).

**NULL root cause and fix.** Run 2's NULL trial 38 put `x` into the textarea and left CodeMirror unchanged, but the Automerge heads moved: `HistoryState` writes the device claim (`claimActor`, `meta/claimedAt`, no text) `CLAIM_PAUSE_MS = 2_000` after the last edit (`frontend/src/ui/history/history.svelte.ts:78,185`). Normal trials are 1.3 s apart, so every edit reset that timer; the first NULL trial is the run's first 2 s pause. The check now proves what NULL must prove: identical CodeMirror `Text`, no CodeMirror change during the trial (immediate failure), no CodeMirror input handler, and either unchanged heads or recorded `A.diff` paths none of which is under `text` (re-checked in Go, `nullDocumentIsolated`). The trial stays in the NULL population, so the claim's cost can only raise NULL p95. Run 3: **30/30 NULL samples**; only trial 38 had a document write, `[meta, meta/claimedAt]`.

**NULL result:** raw p95 2 ms, upper p95 3 ms > 1 ms, so the instrument still cannot certify 4 ms. The NULL bracket (mean 1.27 ms) is 1.23 ms unrecorded browser time (textarea default action, dispatch, closing task wait) and 0.03 ms recorded JS; with a 1 ms clock any non-zero raw NULL makes upper >= 2 ms.

Self time per keystroke (ms, mean over samples; window = outer keystroke bracket). Means add up exactly to the bracket mean; per-row p95 is 1 ms-quantized. `source` is resolved offline from the perf-only sourcemaps.

**warm** (n=30): bracket mean 6.77 ms (p95 9) = recorded 3.33 + unrecorded 3.43.

| self mean | self p95 | settle mean | in n | who runs | source |
| ---: | ---: | ---: | ---: | --- | --- |
| 3.43 | 5 | 0.00 | 30 | unrecorded: browser dispatch/default action, style/layout, GC/JIT, promise continuations, harness |  |
| 0.83 | 2 | 1.40 | 24 | stage codemirror-update |  |
| 0.83 | 2 | 0.83 | 22 | stage automerge-ui |  |
| 0.30 | 1 | 0.30 | 9 | observer MutationObserver | node_modules/@codemirror/view/dist/index.js:7195 / app: src/editor/setup.ts:112 (EditorView) |
| 0.23 | 1 | 0.40 | 12 | setTimeout anonymous ← stage automerge-ui |  |
| 0.20 | 1 | 0.67 | 19 | stage codemirror-measure |  |
| 0.17 | 1 | 0.17 | 5 | queueMicrotask anonymous ← stage preview-noteEdit |  |
| 0.10 | 1 | 0.10 | 3 | queueMicrotask anonymous ← stage automerge-ui |  |
| 0.10 | 1 | 0.10 | 3 | stage cm-plugin:1:yO | node_modules/@codemirror/language/dist/index.js:1773 |
| 0.07 | 1 | 0.10 | 3 | stage cm-plugin:2:anonymous | node_modules/@codemirror/view/dist/index.js:11523 |
| 0.07 | 1 | 0.07 | 2 | listener window keydown | node_modules/svelte/src/internal/client/dom/elements/events.js:88 / app: src/App.svelte:370 (event) |
| 0.07 | 1 | 0.07 | 2 | stage cm-plugin:10:anonymous | node_modules/@codemirror/view/dist/index.js:10465 |
| 0.07 | 1 | 0.07 | 2 | stage codemirror-requestMeasure |  |
| 0.07 | 1 | 0.07 | 2 | stage preview-noteEdit |  |

**cold** (n=30): bracket mean 20.77 ms (p95 29) = recorded 16.07 + unrecorded 4.70.

| self mean | self p95 | settle mean | in n | who runs | source |
| ---: | ---: | ---: | ---: | --- | --- |
| 4.70 | 7 | 0.00 | 30 | unrecorded: browser dispatch/default action, style/layout, GC/JIT, promise continuations, harness |  |
| 3.97 | 6 | 6.07 | 30 | stage codemirror-update |  |
| 3.03 | 4 | 3.03 | 30 | stage automerge-ui |  |
| 2.07 | 3 | 2.07 | 30 | observer MutationObserver | node_modules/@codemirror/view/dist/index.js:7195 / app: src/editor/setup.ts:112 (EditorView) |
| 1.43 | 2 | 1.43 | 30 | listener div keydown | node_modules/@codemirror/view/dist/index.js:4641 / app: src/editor/setup.ts:112 (EditorView) |
| 1.13 | 2 | 1.17 | 30 | stage codemirror-measure |  |
| 0.90 | 1 | 1.27 | 28 | setTimeout anonymous ← stage automerge-ui |  |
| 0.60 | 1 | 0.63 | 18 | stage cm-plugin:1:yO | node_modules/@codemirror/language/dist/index.js:1773 |
| 0.50 | 1 | 0.50 | 15 | queueMicrotask anonymous ← stage preview-noteEdit |  |
| 0.50 | 1 | 0.50 | 15 | stage preview-noteEdit |  |
| 0.27 | 1 | 0.27 | 8 | queueMicrotask anonymous ← stage automerge-ui |  |
| 0.23 | 1 | 0.37 | 10 | stage cm-plugin:2:anonymous | node_modules/@codemirror/view/dist/index.js:11523 |
| 0.20 | 1 | 0.20 | 6 | listener div keydown | node_modules/svelte/src/internal/client/render.js:248 |
| 0.17 | 1 | 0.17 | 5 | listener window keydown (capture) | node_modules/svelte/src/internal/client/dom/elements/events.js:88 / app: src/present/PresentHost.svelte:65 (event) |

**Top app-side costs (warm / cold mean self ms in the keystroke bracket):**

1. CodeMirror input + view update for the editor built by `frontend/src/editor/setup.ts:createEditor` (DOMObserver MutationObserver, InputState keydown, `EditorView.update`, measure, language highlighter plugin): 1.60 / 9.50.
2. Synchronous Automerge write per keystroke, `frontend/src/editor/amSync.ts:applyTransactions` → `frontend/src/store/chunked.ts:applyEdits` (`handle.change`), plus the 0 ms automerge-repo `asyncThrottle` timer it schedules (sync/storage save; `wait` < 0 when keys are > delay apart): 1.17 / 4.20.
3. App per-change listener `frontend/src/App.svelte` `note` (`handle.on("change")`): `frontend/src/preview/preview.svelte.ts:PreviewModel.noteEdit` and the Svelte effect flush it triggers, problems, LSP didChange: 0.27 / 1.20 (LSP didChange itself < 0.1).

Unrecorded browser time (dispatch, default action, style/layout, GC/JIT, promise continuations, harness brackets) is 3.43 / 4.70 ms; JSC exposes no GC/JIT timing to page JS, so it is reported as one bucket, not split. Medians match run 2 (warm handler 7 ms, cold 20-21 ms); the instrumentation added no systematic cost. Nothing here is subtracted from any gate.

```sh
cd /home/mathieu/GolandProjects/inkwell/.claude/worktrees/l3 && DISPLAY=:0 GDK_BACKEND=x11 INKWELL_NATIVE_PERF_RUN=1 mise exec -- bash scripts/perf/type-native.sh --input-backend=mutter
```

## Optimisation experiments (2026-11-07 16:14-17:50)

Same host command, unchanged budgets, verifier and tests. 21 host runs, each 30 warm + 30 cold processes + 30 NULL. Variants were swapped only between runs and compared on **interleaved** runs, because the host drifted: cold handler mean was 21.5 ms in base run 1 (16:14) and 39 ms in base runs 2-3 (16:27-16:33) with identical code. An Antigravity OAuth window (Epiphany, not ours, left untouched) opened at 16:19; gortex used about one core. One run made while vitest was running was discarded. Tests: Mann-Whitney U (two-sided) on pooled per-keystroke samples. "warm raw" is the outer keystroke bracket from `raw_runs` (diagnostic, includes samples the verifier rejected, see the clock finding below); the other columns are verifier populations. Means / nearest-rank p95, ms.

| Variant (runs) | warm raw | warm handler | cold handler | warm frame work (upper) | cold frame work (upper) | max gap upper |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| base (4) | 7.85 / 11 | 7.61 / 11 | 35.5 / 59 | 13.6 / 18 | 42.7 / 70 | 62-97 |
| (a) deferred write (5) | 5.34 / 10 | 5.51 / 13 | 29.1 / 43 | 13.3 / 21 | 48.3 / 71 | 62-90 |
| **(a)+(c) pre-warm, kept (8)** | **4.97 / 8** | 4.56 / 6 (n=25) | **20.9 / 32** | 13.2 / 18 (n=25) | **35.5 / 54** | 64-85 |
| (a)+(c)+(b) containment (3) | 4.91 / 10 | - | 22.2 / 30 | - | 38.4 / 51 | 64-73 |
| (a)+(c2) deeper pre-warm (3) | 5.54 / 9 | - | 17.1 / 24 | - | 32.1 / 42 | 64-100 |

Pooled rows span different host periods; the decisions below use the interleaved pairs only.

**(a) Automerge write after the next paint — kept.** `frontend/src/editor/amSync.ts`: a highest-precedence `EditorView.inputHandler` marks the transaction CodeMirror dispatches for a typed character (its DOM input path); its edits wait and are written in one `handle.change` from `requestAnimationFrame → setTimeout 0`, so after the paint and after the instrument's closing task (the INP "yield after paint" pattern). Everything else (commands, paste, programmatic dispatch, tests) is written at once, after anything waiting, so order is kept; blur, `visibilitychange`, `pagehide`, a hidden page and `destroy` flush. A change arriving while edits wait (peer, history claim, replace) cannot be written to from inside the handle's `change` listener (automerge-repo's state machine queues it), so the waiting edits go in with `handle.changeAt(reconciledHeads)`, concurrent with that change; when it lands (identified by its deps), CodeMirror receives `A.diff(local branch → merged heads)`, exactly what the other change adds. Same-period runs (3 base vs 3 (a)): warm handler mean -2.3 ms (p<1e-4), cold -10.8 ms (p<1e-4), warm frame work -1.1 ms (p=0.006), cold frame work +0.6 (n.s.). The `automerge-ui` stage, its storage/sync timer and the App `note` chain (PreviewModel.noteEdit, problems, LSP didChange) left the keystroke bracket. New tests in `amSync.test.ts`: one change per frame, command after a waiting key written in order, a change arriving while a key waits (editor and document), two people typing at once converge, blur flushes. Known limit: if WebKit throttled rAF in a focused, visible window, a typed edit would wait until the next frame or any other transaction.

**(d) noteEdit / problems / Svelte flush out of the input task — subsumed by (a).** They run from the document's `change` event, which now fires in the post-paint task; no separate change was needed (`preview-noteEdit` absent from every (a) keystroke table).

**(c) pre-warm at editor creation — kept.** `frontend/src/editor/prewarm.ts`, called once per window from `createEditor`: a hidden (opacity 0, `pointer-events:none`, `aria-hidden`, `contain:strict`, in the viewport so CodeMirror measures) EditorView with the same editing extensions and the Typst language types through `document.execCommand("insertText")` and 8 dispatches; a throwaway in-memory `Repo` (no network, no storage) takes 8 `handle.change`s, `A.diff`, `saveIncremental`, `generateSyncMessage`. Synchronous, so focus is back before any input task; both are removed after the next frame. vs (a), interleaved (A4, A5 vs AC1-3): cold handler mean -8.2 ms (28.8→20.5, p<1e-8), cold frame work -12.9 ms (48→35, p<1e-8); warm unchanged (raw mean 5.07 vs 5.26 on those runs, 4.97 over all 8 kept runs). Test `prewarm.test.ts`: focus, document and page left as they were.

**(c) revised 2026-11-08 (pre-merge review B1).** The hidden editor outlived the call (removed after the next frame), so e2e lookups found two `.cm-content` (ER-1-4, L2 FM-OPEN failed), and `view.focus()` moved focus into an `aria-hidden` subtree. Now the box is also `inert`, focus never moves, and the view is destroyed before `prewarmTyping` returns. Without focus, `execCommand("insertText")` cannot run; CodeMirror's DOM-change path is driven instead by editing the text node and flushing its observer synchronously (`prewarm.test.ts` checks that path ran, that no focus event fired and that no second editor remains). Whether the cold gain holds is measured by the PERF-TYPE-native gate runs recorded in `gates/L3-setup/SUMMARY-fixall.md`.

**(c2) deeper pre-warm — reverted.** Synthetic non-bubbling keydown before each `insertText` (CodeMirror's keydown handler `forceFlush`es the previous DOM change; the programmatic dispatches of (c) clear it, so (c) never warmed the DOM-read path). Interleaved vs (c): cold handler -2.6 ms (p=1e-4), cold frame work -1.7 ms (p=0.049), keydown and MutationObserver cold rows 2.1→0.8 and 2.5→1.0 ms; but warm raw bracket **+0.73 ms (4.82→5.54, p=0.0025)**. Every keystroke after the first is warm, so this trade is a loss.

**(b) unrecorded browser time — explained in part; containment reverted.** NULL (textarea) unrecorded is ~1.2 ms; warm CodeMirror unrecorded ~2.7 ms with (a)+(c). The extra ~1.5 ms is WebKit's contenteditable typing command, which works on `VisiblePosition`s and therefore lays out synchronously in the default action, plus style/layout of the changed line; CodeMirror deliberately lets the browser insert typed text (`handlers.beforeinput` only records it). `contain: strict` on `.cm-scroller` (size+layout containment, a relayout boundary; tooltips live outside it) gave warm raw 4.91 vs 4.97 (n.s.) and **cold handler +1.9 ms (p=7e-4), cold frame work +3.7 ms (p<1e-4)**: reverted. Taking typing away from the browser (`preventDefault` on `beforeinput` insertText and dispatching the transaction directly) would remove the typing command, but there would be no `input` event, which the harness requires for a trusted native key, and it changes IME/spell-check behaviour: not attempted, would need a design decision and harness change. WebKit inspector timelines could not be driven from this host session.

**Where it stands:** warm keystroke ≈ 2.7 ms unrecorded browser + ≈1.5 ms CodeMirror (update, measure, observer) + <0.3 ms App; cold ≈ 7 ms unrecorded + 3.5 update + 2.5 observer + 2.1 keydown + 1.7 measure. The frozen budgets are still **not met**: warm handler p95 6-8 (needs raw ≤3 with q=1), cold handler p95 ~32, cold frame work p95 ~54, max gap upper 62-100 ms in every variant including base (not investigated here). Final run on the kept code: `gates/L3/PERF-TYPE.json`, raw `gates/L3-setup/native-raw-w2akpi05` (cold mean 27.3, again a slow host period).

### Instrument finding: key event timestamps drift against `performance.now()`

Since about 16:30 most warm phases were rejected ("invalid handler timestamps", "invalid NULL keystroke/document isolation proof") in base and experiment runs alike. Cause: `keyDownAt` is `event.timeStamp`, and `event.timeStamp - performance.now()` read in the window keydown capture listener drifted per run: -4..0 ms (16:18), -2..+1 (16:29), +1..+5 (16:48-16:51), -1..+3 (16:59). A positive value puts the keystroke after its own listener, so `handler.start < keyDownAt` and `docChangedAt < keyDownAt` fail. The event's time base (GDK/X event time under XWayland, presumably) is not `performance.now()`'s. Not changed here (the verifier is right to refuse inconsistent clocks). Proposal for a user decision: take the keystroke boundary from the earliest in-page clock read (the window capture listener's `performance.now()`, which already opens the outer bracket) and keep `event.timeStamp` only as a diagnostic, or bound the offset explicitly in preflight.

### NULL rule "upper p95 ≤ 1 ms": the finding, and alternatives (proposal only)

The statement holds per sample: raw values are integers, upper = raw + q with q = 1 ms, so any non-zero raw gives upper ≥ 2 ms; upper p95 ≤ 1 needs raw p95 = 0, that is at most 1 of 30 samples non-zero (nearest-rank, 29th of 30). With a random phase against the 1 ms grid, P(raw > 0) equals the true duration d for d < 1 ms, so passing needs d ≲ 0.02 ms (P(pass) ≈ 0.88 at 0.02). But the clock is not the main obstacle: pooled over 120 NULL samples from 4 complete runs the raw histogram is {0: 1, 1: 80, 2: 36, 3: 2, 5: 1}, mean 1.36 ms, and the mean of quantized differences is unbiased under random phase. The NULL path itself (keydown dispatch, textarea default action, input dispatch, MessageChannel closing task) costs about 1.3 ms of real browser work. Even a perfect clock would fail a 1 ms limit, and that cost is intrinsic keystroke handling, not instrument overhead. Sound alternatives:

1. Measure what the rule means to bound, the instrument's overhead: interleaved NULL trials with the recording hooks on and off (the two outer reads kept in both), with a criterion on the difference of means and its one-sided 95 % confidence bound ≤ 1 ms (means are unbiased under random phase; n=30+30 gives a standard error of about 0.1 ms).
2. A finer clock outside the measured path: serve the perf build cross-origin isolated (COOP/COEP from the Wails asset handler) if WebKitGTK then raises `performance.now()` resolution. Unverified (no web access in this session); the existing preflight measures resolution, so one run would settle it.
3. Batch amortisation (N NULL keys inside one bracket) bounds the mean only, not a per-event p95; acceptable only together with 1.

## Residual fix, frame-gap diagnosis and CodeMirror-owned typing prototype (2026-11-07 18:35-19:10)

Same host command, unchanged budgets, verifier and tests. Runs: `base1` (18:38), then interleaved B1 A2 B2 A3 B3 (18:47-19:04, ~3.4 min each), then A4 (final, default code). Statistics: two-sided Mann-Whitney U on pooled per-keystroke samples (tool kept in the session scratchpad, normal approximation with tie correction). Means / nearest-rank p95, ms.

### 1. Correctness residual: every editor on a handle

`flushTyped` already wrote what waits in every editor of a handle. Two gaps are closed:

- **Save, export, compile.** All three go through the projection (`frontend/src/store/projection.ts`: Ctrl+S and the header-bar actions call `flush`, export calls `workspace.flushProjection`, Go compiles the projected file, the 300 ms timer calls `flush`). `drain` read `handle.doc()` without the keys still waiting for the paint. It now calls `flushTyped(handle)` before every read, and so does the public `dirty()`. The `change` listener (`emitDirty`) reads without flushing, so nothing is written from inside a `change` event (the case amSync's `rebaseWaiting` exists for). There is no native menu and no Go-initiated event in the frontend: the header-bar menu is in the webview (its `pointerdown` already flushes).
- **`whenWritten`** waited only for the first waiting editor. It now re-checks all editors after each write. In practice the second editor's keys already land in the same write, because amSync rebases them synchronously from the first write's `change` (the test passes on the old code too and stays as a guard). The projection test failed first (`expected ['abc'] to equal ['#abcd']`).

Tests in `frontend/src/editor/amSync.test.ts`: two editors with waiting keys, the projection writes both; `whenWritten` waits for both; a peer change arriving while a key waits, with a projection attached, converges with one change per write. Full frontend suite after all three parts (via `scripts/heavy.sh`, no host run in progress): 106 files, 1173 passed, 1 todo; svelte-check 0 errors.

### 2. The 60 ms gap and the missed refreshes: causes

Replayed from raw runs `native-raw-d1kbj9ax` (18:10) and `native-raw-stylk50c` (base1) with the verifier's exact interval and missed-refresh rules (`internal/nativeperf/frame_work.go:frameWork`, threshold = baseline median period + 2 quanta). Run 18:10: 37 missed = **30 cold + 7 warm** (not 37 warm). Base1: 30 cold + 11 warm.

**Cold: one ~50 ms frame in every fresh process, from the first preview result.** All 30 of 30 cold processes in both runs show one long frame (43-60 ms, max 60/64) 626-750 ms after the key. The same sequence comes before it every time (callback sources resolved from the perf sourcemaps):

1. A language-server Stream message (`src/language/client.ts:60`).
2. The **first preview Stream message** of the process (`src/preview/PreviewClient.ts:92`).
3. A 7-12 ms Svelte flush (microtask), then a `codemirror-update`.
4. The `FormatBar` size binding's `ResizeObserver` (`src/ui/toolbar/FormatBar.svelte:61`).
5. One rAF frame doing 6 ms of main-thread rendering (frame record 708→714).
6. The next rAF only 53 ms later.

**The main thread is idle during that gap.** The continuous task probe keeps running every 8 ms (`busy` spans 707-715, 715-723, …, 780-788), so neither JavaScript nor a main-thread task holds the frame. WebKit's compositing side takes ~47 ms to present the first frame that contains the preview pages (`<img src="data:image/svg+xml…">`, new content and layer), and rAF waits for that presentation.

Warm keystrokes also get preview messages (~1170 and ~1300 ms after the key, ≤1 ms each, no ResizeObserver, no long frame). So this is a once-per-process first presentation, not per-keystroke work. It is timed from startup more than from the key: the correlation between key time and gap time is 0.28.

GC/JIT cannot be the cause: those run on the main thread, and the probe shows it free. Storage and sync timers (`GoStorageAdapter.ts:202` messages, 0 ms each) are not involved either. LSP sends take 0 ms.

**Warm: keystroke work straddling a vsync.** The warm misses are intervals of 19-22 ms against a threshold of about 18.7 ms. Their first heartbeat sits 14-17 ms before the key (or in the frame right after it). The key arrives just before a vsync, and the 3-5 ms of input handling pushes the next rAF a few ms late. NULL trials (textarea, no editor work) miss 0-3 of 30 refreshes against 4-10 of 30 for editor keys in the same runs, so the editor's keystroke cost is what tips these frames over the threshold. The only lever is shorter keystroke work (see 3).

**Other instrument findings (not app work):**

- Base1 cold-19 was rejected for two heartbeats at the same rounded millisecond (811, 811): "invalid frame timestamps", hence "missing ≥30 cold samples". The verifier is right to refuse it.
- A2 and A3 lost samples to "observed cadence slower than nominal GdkMonitor refresh" and "stale or post-input cadence baseline". These are host cadence hiccups.

**Fixes kept: none.**

- The cold gap is browser compositor work on the first preview presentation. Cutting it would need a different preview surface (for example pre-rasterised pages) or a compositor warm-up whose effect cannot be predicted. Neither is cheap or clearly app-side, so it was not attempted.
- The 7-12 ms Svelte flush before it is real main-thread work, but removing it would not remove the 47 ms off-main wait.
- The warm misses shrink only with the keystroke cost.

### 3. Prototype: CodeMirror owns the typed insertion (perf builds only, OFF)

Implementation:

- `frontend/src/editor/ownTyping.ts` adds a highest-precedence `beforeinput` DOM handler. For `insertText` with data, outside any composition (`isComposing`, `view.composing`, `view.compositionStarted`), and only in an editable, non-read-only editor, it calls `preventDefault()`. It then asks the `EditorView.inputHandler`s and dispatches `replaceSelection(data)` with `userEvent: "input.type"` and `scrollIntoView`, as CodeMirror's own default insert does.
- `createEditor` adds it only when `VITE_PERF_NATIVE === "1" && VITE_PERF_CM_TYPING === "1"`. It is a function, so the product bundle drops it: a product `vite build` contains no trace of it, and the flagged perf build does.

Harness change (`frontend/src/perf/native.ts`, only when `VITE_PERF_CM_TYPING=1`):

- A trusted `beforeinput` whose default CodeMirror prevented ends the input, in place of the `input` event that no longer exists.
- A sample must then have exactly keydown + beforeinput: an extra `input` event makes it incomplete, and a beforeinput that was not prevented never completes.
- The handler and document positive controls listen on `beforeinput`.

The proof is not weakened. The key is still a real Mutter key whose trusted keydown and beforeinput target CodeMirror's content DOM. `inputData` must equal the key, the CodeMirror transaction must be the exact trusted insertion, and the Go verifier is unchanged (it already accepted beforeinput as the DOM input segment). Tests in `native.test.ts`: complete when prevented, rejected when not prevented, rejected when an input event also arrives.

Interleaved host A/B (A = current code: base1, A2, A3; B = prototype: B1, B2, B3):

| metric | A (current) | B (prototype) | p |
| --- | ---: | ---: | ---: |
| warm handler (outer bracket) | 5.02 / 8 (n=90) | **3.66 / 8** (n=90) | 7e-11 |
| cold handler | 19.2 / 28 | **13.9 / 21** | 9e-11 |
| warm frame work (upper) | 13.1 / 18 | 12.7 / 17 | 0.37 |
| cold frame work (upper) | 35.2 / 47 | **30.8 / 40** | 2e-5 |
| warm max interval per key | 19.4 / 22 | 18.8 / 20 | 0.04 |
| cold max interval (first preview) | 53.0 / 61 | 51.4 / 59 | 0.12 |
| warm missed refreshes | 31 / 90 | 18 / 90 | |

Per-run p95, B runs: warm handler 9, 6, 6; cold handler 22, 20, 21; cold frame work 43, 38, 40 (all ≤50). A runs: warm handler 11, 7, 8; cold frame work 59, 42, 50. B removes about 1.4 ms per warm key and 5 ms per cold key. That is WebKit's contenteditable typing command, as the earlier unrecorded-time analysis predicted (warm unrecorded ~2.7 ms against ~1.2 ms for NULL). It does not reach the 4 ms warm p95: 6 ms at best, and q = 1 ms makes the upper bound 7.

What it changes or would need, with evidence:

- **Composition (IME: ibus/fcitx, CJK input).** Composition events (`insertCompositionText`, `isComposing`, CodeMirror's `composing`/`compositionStarted`) stay on the browser path. The test feeds them and they are not prevented. The committed text after `compositionend` arrives as `insertText` and is inserted by CodeMirror, which is correct. Not yet tried with a real IME on the host: it needs a manual pass with ibus (Pinyin/Hangul) before any decision.
- **Dead keys.** With GTK's IM, a dead key starts a preedit (composition) and stays on the browser path. The composed character is a plain `insertText`. Covered by the same test, not tried with a real keyboard layout.
- **Spell-check, autocorrect, autocapitalize, writing suggestions.** Nothing is lost: CodeMirror 6.43.15 already sets `spellcheck="false"`, `autocorrect="off"`, `autocapitalize="off"` and `writingsuggestions="false"` on the content DOM (`@codemirror/view` `updateAttrs`), and inkwell does not override them (`setup.ts` adds only `aria-label`). A spell-check replacement (`insertReplacementText`) and paste are not intercepted (test).
- **Undo grouping.** Kept: consecutive typed keys are one undo step (test with `history()`). The browser's own undo stack no longer contains typed text. inkwell undoes through CodeMirror (Mod-z keymap), but a native "Undo" from a WebKit context menu, if one is ever exposed, would not see typed text.
- **Typing help (autocompletion), closeBrackets-like input handlers, amSync's after-paint write, multiple cursors, typing over a selection, read-only.** All kept, each with a test in `ownTyping.test.ts`.
- **Accessibility (Orca/AT-SPI).** Not verified. The DOM change now comes from CodeMirror's rendering instead of the typing command, so screen-reader typing echo and caret events need a manual Orca check.
- **Atomic ranges.** Differs: CodeMirror's default insert special-cases a cursor next to an atomic range, and `replaceSelection` does not. inkwell registers no atomic ranges today.
- **Event Timing and other `input` listeners.** There is no `input` event any more: the harness adapts, and no app code listens for `input` on the editor (only the CSV sheet's own `<input>`s).

**Not enabled: this is the user's decision.** Gains: −1.4 ms warm, −5 ms cold, cold frame work back under 50 ms in every run, fewer warm misses. Costs: the manual IME, dead-key and Orca checks above, plus the atomic-range difference.

### Final host result (default code, prototype off)

`gates/L3/PERF-TYPE.json`, raw `gates/L3-setup/native-raw-jsifb7ks` (A4, 19:04-19:08): **fail**.

- Warm handler p95: 9 ms (A runs 7-11).
- Cold handler p95: 28 ms (≤50, passes).
- Warm frame work (upper) p95: 20 ms.
- Cold frame work (upper) p95: 48 ms (≤50, passes in this run; A runs 42-59).
- Max gap: 74 ms (upper 75), from cold-08 at +748 ms, the first-preview frame.
- Missed refreshes: 37 counted (replay: 30 cold first-preview + 8 warm, one cold sample rejected).
- Instrument overhead: upper bound 0.31 ms (≤1, passes).

Remaining reasons:

- warm handler p95 and its upper bound > 4 ms;
- a warm frame > 16 ms and warm frame work p95 > 16 ms;
- missed refresh and gap > 50 ms (cold first preview);
- NULL path missed frame or exceeded frame work;
- this run only: "invalid frame timestamps" (duplicate rounded heartbeat), "observed cadence slower than nominal GdkMonitor refresh", the `measure` positive control not validated, and so fewer than 30 valid cold/warm samples.

L3 stays blocked.

## Automated typing checks of the prototype, then three host runs (2026-11-07 21:40-23:00)

`scripts/perf/typing_checks.py` (`mise run perf:typing-checks`, exclusive heavy slot) replaces the manual pass. It builds the perf collector twice (`VITE_PERF_CM_TYPING` 1 then 0) and runs each in a new Go phase `typing-check` (`cmd/inkwell/typing_check_perf.go`: no measurement; the window loads `/?typing-check=1` and `frontend/src/perf/typingCheck.ts` reports CodeMirror's state over the `typing-check` Stream, printed by Go on stdout). Keys go through the Mutter RemoteDesktop session (`NotifyKeyboardKeysym` / `NotifyKeyboardKeycode`, X11 target and focus verified before each). On this host Mutter ignores pointer motion from a session without a stream, so Ctrl+click uses a second session linked to a ScreenCast of the primary monitor (never read), in a closed loop on the webview's own `mousemove` report. The editor is read twice: CodeMirror's state and AT-SPI (`Atspi` text, caret and events). Result: `gates/L3-setup/typing-checks.json`.

| check | ON | OFF |
| --- | --- | --- |
| a) dead keys (layout has all three; host compose table `en_US.UTF-8/Compose` for fr_FR) | ê, é, `"` once each: pass | same: pass |
| b) IME: real ibus (`libim-ibus.so` loaded, engine `xkb:us::eng`), Ctrl+Shift+U 2603 space | ☃ via compositionstart…end: pass | same: pass |
| c) undo: "seed " + "word", Ctrl+Z, Ctrl+Shift+Z | "seed ", "seed word": pass | same: pass |
| d) AT-SPI while typing "abc" | editor `entry`: **no** insert, **no** caret event (only the parent `text` object): **fail** | `entry` inserts (2,a)(3,b)(4,c), carets 3,4,5: pass |
| e) completion (#le → Return → #left), Ctrl+click, read-only, paste (Ctrl+C/Ctrl+V) | pass | pass, same documents |

Notes. `<dead_diaeresis> <space>` is `"` in this host's compose table (`/usr/share/X11/locale/en_US.UTF-8/Compose:39`), not `¨`; the check expects what the table gives and both modes give it. The product editor never enables `EditorState.allowMultipleSelections` (and `drawSelection()` does not add it in @codemirror/view 6.43), so Ctrl+click moves the caret in both modes: there is no multi-cursor to keep. Check (d) failed twice, identically: with the prototype, WebKit no longer sends the focused editor's text-changed and caret-moved events (the change arrives as a DOM mutation, not an editing command), so Orca would not echo typed characters nor follow the caret. **The prototype stays OFF.**

Three host runs, default code (prototype OFF), unchanged rules: `gates/L3-setup/proto-run-{1,2,3}.json`.

| run | status | warm / cold handler p95 | warm frame work (upper) p95 | warm missed editor vs NULL (p) | cold counted misses | reasons |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | fail | 5 / 31 | 14 | 2/30 vs 0/30 (0.25) | 30 | missed refresh (cold); cold frame work p95 51 > 50 |
| 2 | fail | 5 / 24 | 15 | 7/30 vs 0/30 (0.005) | 28 | missed refresh; warm editor > NULL; cadence slower than GdkMonitor; < 30 samples |
| 3 | fail | 7 / 23 | 14 | 5/30 vs 0/30 (0.026) | 29 | missed refresh; warm editor > NULL |

Cold counted misses = `dropped_frames` − warm editor misses (`internal/nativeperf/verdict.go:570-584`): almost every fresh process still has a counted miss besides its exempted first-preview pause. L3 stays blocked.

## Cold missed refreshes: attribution, after-paint message handling (2026-11-07 22:10-23:15)

Same host command, unchanged budgets, verifier, thresholds and failure rules. Replay of the verifier's exact missed-refresh rule (`frameEvidence`: frame and heartbeat intervals and the settle tail longer than the baseline median period + 2 quanta; `coldCountedMisses`: the longest span ≤ 70 ms and every span overlapping it are exempt) on every cold sample, overlapping spans merged into one pause, each pause labelled by what the recorded callbacks, stages and task probe show inside it (first matching label wins). Tools in the session scratchpad (`/tmp/coldan`: `replay.py`, `classify.py`, `stats.py` with two-sided Mann-Whitney U with tie correction and Fisher exact). Experiment variants were build-time flags (`VITE_PERF_EXP`), removed afterwards.

### 1. Attribution (proto-run-1..3, 90 fresh processes)

| label | processes | what ran in it |
| --- | ---: | --- |
| pre-key | 51 | the first heartbeat interval after the harness subscribes (20-37 ms, always followed by a short one: 20+12, 22+10, 25+8, 28+7 ≈ two periods), so the second rAF after attach starts late. Nothing is recorded there (the trial, task probe and stages start at arming, ~10-28 ms before the key). Not the prewarm (removing it gives *more*, see C), not preview/LSP (none has arrived yet; B leaves it unchanged). Remaining cold startup work around the harness attach, unattributed. |
| key | 49 | the cold keystroke task itself (17-43 ms: CodeMirror keydown 2-7 ms, WebKit typing command ~7 ms unrecorded, MutationObserver 5-11 ms, `codemirror-update` 3-6 ms) straddles a vsync: the heartbeat interval holding the key is 19-44 ms. This is also the 51/61 ms cold frame work of proto-run-1 (cold handler 43 ms + first frame). |
| stream-msg+RO | 41 | **the first preview result** (Wails `preview` Stream message, ~580-700 ms after the key, after the language server's first answers and three `stream-send:lsp`): a 6-9 ms Svelte flush (first render of the page block), then the `FormatBar` ResizeObserver; the frame is 20-27 ms and ends exactly where the exempt ~50 ms compositor pause starts. Adjacent, not overlapping, so counted: the "first part of a split preview pause". |
| stream-msg | 31 (+4 settle tail) | other Stream messages: language-server answers (initialize, diagnostics) with a 1-2 ms microtask and `codemirror-update`, at ~+590-630 ms and at the end of the settle window (+1275-1313 ms). |
| idle | 21 | 18-24 ms intervals with no main-thread work (no task, no busy span > 9 ms, no callback), e.g. +58-77, +444-509 ms: compositor/host jitter against a threshold of 18-19 ms. No CSS animation runs then (the header `pulse` only when joining, the preview `shimmer` only on unloaded pages). |
| worker-msg | 11 | the history analysis Worker's answer (+310-345 ms, 300 ms debounce after the edit): 3-6 ms of `codemirror-update`s. |
| baseline-short | 5 | the cadence baseline's median period is 11-14 ms (catch-up rAFs after a late one), so every following 16-17 ms frame counts: an instrument artefact, not app work. |
| busy-other | 6 | other main-thread work. |

The "60-76 ms after the key" misses are `idle` (+58-77, no work recorded) and the amSync/Stream tail of the key. Every process carries several labels; a process is counted as soon as one remains.

### 2. Experiments (interleaved A B C ×3 at 22:14-22:44, then B A B A at 22:44-22:58; 30 cold + 30 warm + 30 NULL per run)

**A** = code as found. **B** = Stream/Worker messages handled after the next paint. **C** = no prewarm (diagnostic arm).

**B, after-paint message handling — kept.** `frontend/src/ui/afterPaint.ts` queues work and runs it from `requestAnimationFrame → setTimeout 0` (the same "yield after paint" pattern as amSync's write), in arrival order, in 8 ms slices (the rest after the following paint); a hidden page uses a plain task, and a 100 ms fallback covers a page hidden while work waits; a throwing callback is reported (`reportError`) without blocking the queue. Used by the preview Stream (`PreviewClient`), the language-server Stream (`language/client.ts`) and the history analysis Worker (`history/analysis.ts`). A message landing at any phase then starts at the beginning of a frame and has the whole frame (the first preview flush is 6-9 ms). Cost: a result is shown at most one frame later. Pooled A (5 runs) vs B (5 runs), means / p95, ms:

| metric | A | B | p |
| --- | ---: | ---: | ---: |
| warm handler | 3.85 / 6 | 3.61 / 5 | 0.039 (better) |
| warm frame work (upper) | 11.66 / 14 | 11.42 / 14 | 0.058 |
| warm missed editor | 31/150 | 19/150 | 0.088 (Fisher) |
| cold handler | 13.75 / 22 | 15.07 / 21 | 0.067 |
| cold frame work (upper) | 25.99 / 35 | 27.63 / 37 | 0.46 |
| first-preview split (stream-msg+RO) | 65/150 | **0/150** | 7e-24 |
| history worker | 26/150 | **0/150** | 9e-9 |
| other Stream messages | 44/150 | **15/150** | 4e-5 |
| counted pauses per process | 2.41 | **1.65** | 1e-5 (MWU) |
| processes with a counted miss | 136/150 | 139/150 | 0.68 |

The first block (A1-A3 vs B1-B3) showed cold handler +2.1 ms (p=0.014); it did not reproduce in reversed order (A4-A5 vs B4-B5: 15.17 vs 15.28, p=1), and no deferred callback ever runs before or inside the keystroke task (first drain ≥ 13 ms after the key in all 90 B processes), so it is host drift. Warm numbers do not regress. Tests: `src/ui/afterPaint.test.ts` (order, nothing synchronous, slice yield, throwing callback, hidden page); the Stream test fakes (`preview/testkit.ts`, `language/client.test.ts`) play the paint with `drainAfterPaint()`.

**C, no prewarm — diagnostic, not a candidate.** vs A (3+3 runs): cold handler 12.80 → 19.54 ms (p=5e-16), cold frame work 24.7 → 34.6 (p=1e-17), key misses 36 → 78/90 (p=7e-11), pre-key misses 32 → **53**/90 (p=0.003), every process counted (90/90). The prewarm does not cause the pre-key misses; it removes part of them. Its teardown "after the next frame" leaves no trace either: no recorded work in the pre-key intervals, and removing everything it does makes them worse. No change to `prewarm.ts`.

**Not attempted, with reasons.** Deferring the first LSP/preview work "until after the first keystroke's frame" is what B does for every message, without a special case. Moving the prewarm teardown to idle time: no evidence it costs a frame (C). Deeper keystroke warm-up (c2, 17:50) already traded +0.73 ms warm for the cold key: not retried. The cold keystroke (17-43 ms) and the unrecorded startup work around the harness attach are what still counts a miss in nearly every fresh process: the per-process count cannot fall until the cold first key itself fits in about one frame.

### 3. Final host runs (kept code: A + after-paint)

`gates/L3-setup/cold-run-{1,2,3}.json` (23:03-23:13), each 30 cold + 30 warm + 30 NULL.

| run | status | warm / cold handler p95 | warm / cold frame work (upper) p95 | warm missed editor vs NULL (p) | cold counted misses | max gap (upper) | reasons |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | fail | 6 / 21 | 14 / 36 | 6/30 vs 0/30 (0.012) | 29 | 59 | missed refresh (cold); warm editor > NULL |
| 2 | fail | 5 / 22 | 17 / 39 | 6/30 vs 2/30 (0.13) | 28 | 65 | missed refresh (cold); a warm frame > 16 ms, warm frame work p95 17 > 16 |
| 3 | fail | 5 / 20 | 13 / 36 | 3/30 vs 0/30 (0.12) | 29 | 59 | missed refresh (cold) |

Cold frame work p95 is 36-39 ms (≤ 50) in all three. Replay of the 90 cold processes vs the 150 A processes: first-preview split 0 vs 65, history worker 0 vs 26, other Stream messages 7 vs 44, counted pauses per process 1.72 vs 2.41 (p=6e-4); still counted: the cold keystroke (45/90), the pre-key startup interval (51/90), idle compositor jitter (23/90). 86 of 90 processes keep at least one, so the cold verdict does not move. L3 stays blocked: what remains is the cold first key itself (WebKit typing command + CodeMirror's first DOM read, the prototype that removes the former is OFF for accessibility), unrecorded startup work at harness attach, and host jitter at an 18-19 ms threshold.

## Warm frames: cold rule applied, warm attribution, A/B experiments, final runs (2026-11-07 23:17 – 2026-11-08 00:46)

### 1. Cold rule (decision « nuit », `docs/objectif-en-cours.md`)

`internal/nativeperf`: a cold missed refresh no longer gates; it is counted in `missed_refresh.cold_missed_diagnostic` (fresh processes with a miss besides the exempt ≤ 70 ms startup pause) and still in `dropped_frames`. Cold gates: handler p95 ≤ 50 (and its conservative upper bound), frame work p95 ≤ 50, pause ≤ 70. Warm unchanged. Tests: `missed_test.go` (`TestColdFirstPreviewPauseIsNotAMissedRefresh`, `TestColdMissedRefreshIsDiagnosticOnly`, `TestColdPauseOverBudgetFailsByTheStartupPauseRule`), green via `scripts/heavy.sh`. Offline re-verification (`gates/L3-setup/cold-run-{1,2,3}.reverified.json`):

| run | verdict | reasons |
| --- | --- | --- |
| cold-run-1 | fail | warm editor 6/30 vs NULL 0/30 (p = 0.012) |
| cold-run-2 | fail | a warm frame > 16 ms, warm frame work p95 17 |
| cold-run-3 | **pass** | — (cold missed 29/30, diagnostic) |

### 2. Warm attribution (240 warm editor trials on the kept code: cold-run-1..3 + B1..B5)

Same `frameEvidence` replay as above; scratch tools `/tmp/warm` (`cat.py`, `fw.py`, `compare.py`; statistics from `/tmp/coldan/stats.py`).

- **Missed refreshes, 34/240** (NULL 9/240), by where the long interval lies:
  - *tail-idle* 12 (+2 with one Stream message): intervals of 19-24 ms 100-1300 ms after the key with no recorded work. NULL has the same window (settle 1300 ms) and 9/240 of them: host jitter, not app work.
  - *pre* 11: the heartbeat interval holding the key (≈ −16 → +4 ms). The key lands 12-17 ms into the period, and the ~3 ms keystroke task (WebKit typing command, CodeMirror's DOM observer, `codemirror-update`; app stages ≤ 0.3 ms) delays the rAF past the threshold. 0/105 when the key lands < 12 ms into the period.
  - *post* 10: the frame after the key frame is late, with the main thread idle (task probe 8-9 ms spans). The interval after the key frame is ≥ 18 ms in 142/240 editor trials vs 5/240 NULL. It is a miss only when the baseline period reads 16 (threshold 18). This is the rendering of the key frame, not a JS callback.
- **Warm frame work.** In 335/360 trials the verifier's `frame_work_upper_bound` comes from a task-probe span, not from a frame or callback. A span is the probe's 8 ms timer plus whatever runs before its next task: the idle time before the key, the keystroke task, the key frame's rendering, and the amSync write (`setTimeout/flush` 1-2 ms, the App change note, the LSP/preview/store sends). Every warm sample over 16 is a span starting 4-11 ms before the key. Each app callback is ≤ 1 ms; no later frame in the settle window has ≥ 8 ms of work.
- Status bar and word count: the word count already runs 50 ms later (`whenIdle`, no `requestIdleCallback` in WebKit). The cursor position and the FormatBar update in the keystroke microtask (≤ 0.3 ms).

### 3. Experiments (interleaved host runs, 30 cold + 30 warm + 30 NULL each; two-sided Mann-Whitney U, Fisher exact)

| arm | runs | change | result | kept |
| --- | --- | --- | --- | --- |
| X `chrome` | A X X A A X X A | FormatBar and StatusBar follow the editor state on the after-paint queue | warm handler 3.63 → 3.06 ms (p = 7e-6); warm misses 15/120 → 9/120 (p = 0.28); warm frame work 11.58 → 11.57 (p = 0.77); **cold handler 13.7 → 20.8 ms, cold frame work p95 39 → 52 (p < 1e-14)** | no: cold regression |
| F `flush2` | C F S S F C C F S S F C | amSync writes after the frame that follows the key frame | key-frame work 3.54 → 2.18 ms (p = 6e-18), cold frame work 27.2 → 22.8 (p = 1e-12); warm misses 15/120 → 18/90 (p = 0.18); warm frame work p = 0.28; tail misses 3 → 9 (p = 0.03); **F3 aborted: "trusted input did not produce the exact CodeMirror insertion"** | no: correctness |
| S `contain` | same block | `contain: strict` on `.cm-scroller` (`editor/setup.ts`) | warm handler 3.79 → 3.33 ms (p = 2e-4); *pre* misses 10/120 → 2/120 (p = 0.034); warm misses 15 → 11/120 (p = 0.53); warm frame work 11.42 → 11.12 (p = 0.096); cold handler 14.9 → 13.8 (p = 0.008); cold frame work 27.2 → 26.2 (p = 0.043); frame after the key 17.49 → 17.82 ms (p = 0.022, misses after it unchanged 2 vs 2) | **yes** |

Pass rate of the control code under the new cold rule: 5/8 (A1-A4, C1-C4); `contain` 2/4. Failure modes across all 23 runs are single-sample tails: one warm probe span > 16 ms, a borderline Fisher on the warm misses, one cold startup pause > 70 ms (73 and 77 ms in 2 of 480 control cold processes; usually 50-58 ms). Rejected arms were reverted. The experiment flags (`VITE_PERF_EXP`) are removed. The scroller's height always comes from its host (`Editor.svelte` `.page` height 100 %, `ReadOnlyText.svelte` flex 1), so size containment changes no layout; tooltips and panels live outside the scroller. Gates: vitest 1177 passed, svelte-check 0 errors, Go tests green (all via `scripts/heavy.sh`).

### 4. Final host runs (kept code: after-paint + `contain`)

`gates/L3-setup/final-run-{1,2,3}.json` (00:36-00:46).

| run | status | warm / cold handler p95 | warm / cold frame work (upper) p95 | warm missed editor vs NULL (p) | cold missed (diag.) | cold max pause (upper) | reasons |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | fail | 6 / 18 | 15 / 34 | 2/30 vs 1/30 (0.50) | 30 | 63 | one warm frame > 16 ms (trial 12: probe span −7 → +11 ms = idle + key task + key frame + amSync write) |
| 2 | **pass** | 5 / 18 | 14 / 35 | 2/30 vs 0/30 (0.25) | 25 | 55 | — |
| 3 | fail | 5 / 18 | 13 / 33 | 5/30 vs 1/30 (0.097) | 26 | 91 | cold pause 90 ms > 70 (replicate 27: the first-preview compositor pause at +607 ms, main thread free, task probe 8-9 ms spans) |

3/3 in a row is not reached, so L3 stays blocked. What remains are tails. One is the cold first-preview compositor pause, over 70 ms in 3 of 570 cold processes (73, 77, 90 ms). The other is the warm probe span: the key task, the key frame's rendering and the amSync write fit in ~8 ms of a span that also counts up to 8 ms of idle timer. No other threshold or failure rule was changed.

## Frame work from the key, then the gate runs (2026-11-08 00:48 – 01:13)

### 1. Task-probe spans no longer count the time before the key

The warm per-frame sample took the longest task-probe span ending after the key, at full length (`internal/nativeperf/frame_work.go`, `frameEvidence`). A span runs from one probe task to the next: the probe's 8 ms timer plus whatever held the main thread. `KeyDownAt` is the start of the keydown listener, the earliest instant of the key's work, and every positive control is scheduled from that listener. The part of a span before the key is therefore the probe's idle wait or earlier, unrelated work. The span is now counted from `max(span start, key)` to its end, plus one quantum. Everything after the key stays whole, idle or not: the probe cannot tell idle time after the key from work it cannot see, so that time remains a conservative bound. Example: gate-run-4 trial 37, span +7 → +22 ms = timer wait + the 7 ms key frame, counted 16. The per-frame callback sums, the frame brackets, the control detection (`validateControl`, `placementControl`) and the missed-refresh rule are unchanged. No threshold or failure rule changed. `docs/perf/native-typing.md` states the rule.

Tests (`internal/nativeperf/frame_work_test.go`):
- `TestProbeIdleBeforeTheKeyIsNotFrameWork`: the −7 → +11 span gives 11 ms (+ quantum), and the run passes. This test was red before the change (it got 18).
- `TestProbeSpanAcrossTheKeyStillCountsWorkAfterIt`: a −7 → +17 span still fails "per-frame work exceeds16ms".
- `TestEveryControlIsDetectedWhenItsProbeSpanStartsBeforeTheKey`: all 7 controls (handler, layout, early-raf, measure, document, task, deferred) stay validated with frame work > 16 when their probe span starts 7 ms before the key.
- The existing `TestEveryIndependentControlPlacementMustBeCovered`, `TestDeferredThirtyMillisecondWorkFailsAfterSecondFrame` and `TestTwoShortCallbacksCannotHideAnOverBudgetFrame` are green.

Gates via `scripts/heavy.sh`: Go `./...` green, vitest 1177 passed, svelte-check 0 errors.

Offline re-verification of the earlier real runs with the new code (scratch test, removed). All 7 controls are still validated in each run:

| run | old verdict | new verdict | warm frame work p95 (max) |
| --- | --- | --- | --- |
| cold-run-1 | fail | fail (warm misses 6/30 vs 0/30, p = 0.012) | 14 (14) |
| cold-run-2 | fail (frame > 16) | pass | 12 (14) |
| cold-run-3 | pass | pass | 12 (13) |
| final-run-1 | fail (span −7 → +11) | pass | 13 (14) |
| final-run-2 | pass | pass | 13 (13) |
| final-run-3 | fail (cold pause 90) | fail (cold pause 91 upper) | 12 (13) |

Across these 12 runs (6 earlier, 6 below), the old rule put a warm sample over 16 in 3 runs (cold-run-2, final-run-1, gate-run-3: 17). The new rule puts none there.

### 2. Gate runs (kept code), `gates/L3-setup/gate-run-{1..6}.json`

| run | status | warm / cold handler p95 | warm / cold frame work (upper) p95 | warm missed editor vs NULL (p) | warm / cold max gap (upper) | controls | reasons |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | **pass** | 6 / 19 | 13 / 35 | 1/30 vs 1/30 (0.75) | 21 / 58 | 7/7 | — |
| 2 | fail | 6 / 17 | 12 / 33 | 6/30 vs 1/30 (0.051) | 23 / 63 | 6/6 recorded | the handler-control sample has two heartbeats in the same millisecond (+671 ms), so "invalid frame timestamps"; the handler control is therefore missing |
| 3 | fail | 5 / 19 | 12 / 32 | 4/30 vs 1/30 (0.18) | 21 / 58 | 7/7 | cold replicate 11: pre-key cadence median 18 ms > 1000/59.884 + 1 = 17.7, so "observed cadence slower than nominal GdkMonitor refresh"; then fewer than 30 cold samples |
| 4 | **pass** | 6 / 19 | 16 / 34 | 4/30 vs 0/30 (0.056) | 21 / 64 | 7/7 | — |
| 5 | **pass** | 6 / 21 | 14 / 38 | 4/30 vs 0/30 (0.056) | 23 / 57 | 7/7 | — |
| 6 | **pass** | 7 / 19 | 13 / 38 | 3/30 vs 2/30 (0.50) | 21 / 64 | 7/7 | — |

Runs 4, 5 and 6 pass in a row, so the stop rule is met. Neither failure is the cold compositor pause: every cold max gap is ≤ 64 ms. Both failures are validity checks the change does not touch (`gap` in `verdict.go`, the cadence check in `frameEvidence` before the probe code), and the run is refused, not exempted. Under the old rule, runs 4-6 would also have passed (their longest pre-key-inclusive span is ≤ 16). Run 3's warm 17 would have been a second failure reason. Margins stay thin: run 4's warm frame work max is 16, exactly the budget. Warm missed refreshes reach p = 0.051-0.056 in three runs (fail threshold 0.05).
