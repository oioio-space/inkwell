# Startup speed: where the time goes, what can be cut, what budget to pick

Date: 2026-11-11. Status: study for the user's choice of an absolute startup budget (nothing is changed in the product).
Scope: PERF-START (`scripts/perf/start.cjs`, `scripts/perf/baseline.py`: cold server-build process to `.cm-content` in a prelaunched
Chromium) and, as far as measurable, the native WebKitGTK window.

## 1. Question and answer in short

Can startup be made faster? Yes, but less than the headline phases suggest. Of the ~350 ms (Chromium proxy) only ~7 ms is
Inkwell's Go code and the sidecar hashing is not on the path at all (it is already lazy, section 4). The cost is: loading
the GTK/WebKitGTK shared libraries (~33 ms, native and proxy alike), parsing and evaluating one 1.36 MB JS chunk (~90 ms),
initialising Automerge WASM (~47 ms) and building the first document and editor (~115 ms), all on the webview's main thread.
Measured, in the product: preload hints save ~17 ms. Estimated, not yet measured: deferring non-editor code ~15 ms, overlapping
the Go handshake with Automerge init 20-40 ms. A CGO-free proxy build saves 33 ms of the proxy number only (no user-visible gain).

## 2. Conditions, honesty about the measurement

* Host: 20 CPUs, 31 GB, Fedora (kernel 7.2), Go 1.27.3, Playwright Chromium (headless). Everything ran at `nice -n 19`, one
  measurement at a time, from a scratch worktree at HEAD `f78949b` (`.../scratchpad/wt`; the main tree is dirty and was not used).
  Instrumentation (`TP` timestamps, trace/pprof hooks, a Resolve timer) lives only in that worktree: `startup-evidence/instrumentation.diff`, `tp.go`.
* Background load was not zero: `gortex daemon` (110-277 % of one CPU, 6-7 GB RSS), the running Claude session (5-166 %) and GoLand. Load average
  went from 5.2 (first runs) to 2.0 (variants run, 93 % CPU idle). Snapshots (`uptime` + `top`) are in the `load-*.txt` files, one per measurement.
  Absolute numbers carry that noise (the same baseline binary gave 382 ms at load 5 and 351 ms at load 2); the variant comparison is interleaved
  (3 rounds x 10 runs per variant) so the deltas are fair.
* Not available on this host: `strace`, `perf`, `bpftrace` (`command -v` finds none; `perf_event_paranoid` = 2). They were not installed. Substitutes:
  `LD_DEBUG=statistics`, `/usr/bin/time -v`, Go `runtime/trace` + pprof (CPU 2 kHz, block, mutex), Chromium trace and CDP CPU profile, and C/Go control binaries.
  Consequence: syscall-level file I/O of the sidecar hashing was not seen with strace; it was measured as wall time by a Go timer and seen in the Go CPU profile.
* The Playwright WebKit build cannot run here (missing system libraries, install refused). The native window would open on the user's
  Wayland desktop and there is no nested compositor (no Xvfb, weston, cage); no existing script measures native startup (`native_probe.py` needs X11 and
  only captures readiness, `internal/nativeperf` times typing). **The native window path is therefore NOT measured**, see section 6.
* Artefact root: `/tmp/claude-1000/-home-mathieu-GolandProjects-inkwell/2f1a45b9-d928-439b-8250-dad8f64dd9c9/scratchpad/startup-evidence/` (called `EV/` below;
  session scratch space, copy it somewhere permanent if it must be kept). The tooling that produced it is in the same directory.
* The existing gate's poll of `/health` every 20 ms adds up to 20 ms (about 10 on average) to PERF-START. My harness polls every 1 ms. With the original
  `start.cjs` unchanged on my build: median 354 ms, min 342, max 657 (n=30, `out/start.json`), consistent with the archived 334 ms.
* The bundle is bigger than the figures quoted in the request: `App` chunk is 1,357 kB (442 kB gzip), entry `index-client` 51 kB, the WASM 3,644 kB (1,158 kB gzip).
  The Automerge WASM is instantiated twice (main thread and `analysis.worker`); the worker one starts after the editor is usable (335 ms) and is off the path.

## 3. Phase breakdown (Chromium proxy, server build, median)

Source of the totals: baseline variant A, 30 runs, `EV/variants/A-{1,2,3}.json`, summary `EV/variants/summary.txt`: process spawn to editor 351 ms (p95 387);
spawn to `/health` 55 ms; navigation to editor 296 ms. The page timeline is from one traced run (`EV/chromium/trace-cold.json`; tracing adds some overhead), in ms from navigation start.

| # | Phase | ms | On the critical path? | Tool, artefact, reading |
|---|-------|---:|-----------------------|--------------------------|
| 1 | Process exec to `main()`: ld.so + shared-library constructors of GTK4/WebKitGTK/JSC (the server build still links them) | 38 | yes (native too) | `node spawn.cjs`, `EV/spawn-latency.txt`: server 37.7 (min 34.2); hello static Go 1.6; same server with `CGO_ENABLED=0` 4.9; C hello linked only to gtk4+webkitgtk-6.0 31.2 vs plain C 1.3. `LD_DEBUG=statistics` (`EV/ld-statistics.txt`): the dynamic loader itself is only 6.5M cycles, so the rest is library constructors; `/usr/bin/time -v` (`EV/time-v-version.txt`): 4,564 minor faults, 60 MB RSS for `--version`. |
| 2 | Go init, identity, trust, libp2p node, project, Wails server up | 7 + 3 | yes, but tiny | `TP` timestamps (`EV/tp-timeline-run3.txt`, `EV/phase-summary-A.txt`): `main_start`->`pre_run` 7.3 (identity 0.7, trust 0.4, **libp2p `New` 3.4**, everything else < 0.3); `pre_run`->`/health` ok 3.4. Package `init`: 2.9 ms in total over 303 inits (`GODEBUG=inittrace=1`, `EV/init-trace.txt`). |
| 3 | Go-side blocking | 0 | no | Go block and mutex profiles over the first 300 ms (`EV/go/block-top.txt`, `mutex-top.txt`): only idle channel receives and the profiler's own waits; mutex contention 0.96 ms in total. `EV/go/go.trace` (`go tool trace`), `EV/go/cpu-flame.svg`. |
| 4 | HTML, CSS, entry JS (`index`, `index-client`) | 0-24 | yes | trace: ParseHTML 1-9; entry module evaluated 17-28; all local, 1.2 ms for the HTML (`phase-summary-A.txt`). |
| 5 | `App` chunk: request starts only after the entry ran (dynamic import), background parse 42 ms | 24-75 | yes | trace: `v8.parseOnBackground App-*.js` at 29.7 for 42.1 ms (`EV/chromium/analysis-cold.txt`); response at 73-75. |
| 6 | `App` top-level evaluation | 75-121 | yes | trace: `v8.evaluateModule` 75.8, 45.4 ms. The WASM request is only issued at 121.0, after this finishes (`ResourceSendRequest`). |
| 7 | WASM fetch + streaming compile + instantiate + Automerge init | 121-184 | yes | trace: streaming compile is already used (`v8.wasm.streamFromResponseCallback` 122.3, `FinishStreaming` 2.4 ms, response done 137.6); then one 46.8 ms microtask checkpoint on the main thread (137.7-184.5) that holds instantiate and module init. Compile is not the cost. |
| 8 | Wails runtime / stream handshake (`poll`, `runtime`, `send`, events socket) | 184-226 | yes | trace: first `/wails/stream/poll` at 182.4; replies are processed at 197-226 once the main thread is free; Layout 10.7 ms at 185. |
| 9 | First document load + CodeMirror and Svelte build, `.cm-content` visible | 226-341 | yes | trace: `FunctionCall index-client` 249.4, 90.5 ms; CDP profile (`EV/chromium/cpuprofile-summary.txt`): 311 ms sampled, `js-to-wasm` 43.2 ms (Automerge `loadIncremental` 14.8 ms for a near-empty doc), 57 ms native "program", GC 5 ms. |
| 10 | Sidecar hash (typst) | 33 | **no** (after the editor) | see section 4. |
| 11 | Sidecar hash (tinymist) | 46 | **no** (after the editor) | see section 4. |
| 12 | Libp2p, project open | in 2 | - | the node is up 3.4 ms after identity; nothing to defer worth the risk. |

Where the wall time of the page goes (trace): background parse 42, top-level eval 46, WASM instantiate/init 47, editor build 90, Go round trips and layout ~45, rest idle/serial waits.
The main thread is the bottleneck: the WASM, `custom.js` and the `/wails/*` replies are all ready long before the main thread can handle them (e.g. `custom.js` request sent at 121.6, response processed at 197).

## 4. Sidecar verification: is it on the critical path?

No. `sidecar.Store.Resolve` (SHA-256 of the whole executable on every call, "call it right before each launch") is called only from
`internal/typst/engine.go:322` (first compile) and `internal/language/language.go:106` (first language-service connection). Neither runs at app start; both
are triggered from the webview after the editor is mounted (`preview.start`, `configureLanguage` in `App.svelte` `onMount`).

| Item | Number | Tool, artefact |
|------|-------:|----------------|
| Warm verify typst (55.7 MB), median of 20 | 33.3 ms (32.5-34.8) | Go timer `resolvetime`, `EV/resolve-timing.txt` |
| Warm verify tinymist, median of 20 | 46.0 ms (44.9-47.2) | same |
| Same calls inside the running app (loaded host) | 37 ms and 56 ms | `TP resolve_*` lines (`EV/tp-timeline-run3.txt`): tinymist begins 25 ms after the editor is visible, typst at ~486 ms (preview debounce) |
| First run after install or upgrade: gunzip + hash + write of the cached binary | 359 ms (typst), 408 ms (tinymist) | same file; one time per sidecar version |
| CPU profile of the first 300 ms of the app | 20 ms in `sha256.blockSHANI` under `sidecar.verify`, 5 ms in `compress/flate` under `typst.ensureCaches` | `EV/go/cpu.pprof` (`go tool pprof -peek`) |

Consequences. (1) The invariant "revérifiés avant chaque lancement" is already respected as before each sidecar process launch, and it costs the window nothing;
it costs the first preview and the first language-service answer 33 + 46 ms of serial hashing. (2) Moving the check "to the background" would save nothing for the
window and would weaken the invariant if the result were reused for a later launch; do not do it. (3) Hashing the two concurrently would not help: the two launches are ~150 ms apart in the trace (tinymist right after the editor, typst after the preview debounce), so they never wait for each other. The cold-install extraction (360-410 ms, once per sidecar version) is the only large cost, and it is not startup either.

## 5. Candidate fixes

Gains marked "measured" come from the interleaved experiment (`EV/variants/summary.txt`, 30 runs each, spawn to `.cm-content`, median / p95):
A baseline 351 / 387; B (preload hints) 334 / 359; C (`CGO_ENABLED=0` server build) 318 / 596 (one slow outlier round, median is the usable figure); D (B + C) 303 / 316.
"Estimated" means derived from a measured rate (0.065 ms per kB of `App`, i.e. 88 ms parse+eval over 1,357 kB) or from the trace, not yet built and measured.

| Fix | Gain | Risk | Invariants | Status |
|-----|-----:|------|------------|--------|
| **F1. Preload hints in `index.html`**: `modulepreload` of the `App` chunk (it is requested at 24 ms, 18 ms after the entry could have asked) and `preload as=fetch crossorigin` of the Automerge WASM (requested at 121 ms, after `App` evaluates). Needs a Vite plugin to inject the hashed names. | measured -17 ms median (351 -> 334), p95 -28; WASM response end 118 -> 27 ms after navigation | low (CSP `connect-src 'self'` already covers it) | none touched | measured, in a scratch dist only |
| **F2. Defer code the first editor does not need** from the `App` chunk (`bundle-attribution-App.txt`: sheet/CSV UI + papaparse ~70 kB, lang-html/css/javascript lezer ~130 kB, `@codemirror/merge` 22 kB, xstate 38 kB, `SlideStage`/present, history panel) with `import()` on first use. | ~15 ms (~250 kB of 1,357 kB), estimated | medium (first use of each feature pays a small delay; needs the interface-coherence review) | none | estimated |
| **F3. Overlap the Go handshake with Automerge init**: today the first `/wails/stream/*` calls leave at 182 ms because `workspace.start()` runs after the 47 ms WASM microtask; issue the stream opens and the document fetch first, instantiate WASM while the replies are in flight. | 20-40 ms, estimated from the 184-226 serial wait | medium (ordering inside `onMount`/`workspace`, needs tests) | CRDT stays in the webview, Streams bridge unchanged | estimated |
| F4. Lighter editor build: build extensions lazily (typst grammar 113 kB only for `.typ`, merge, blame) | 10-30 ms of the 90 ms, estimated | medium | none | estimated, needs a profile of `session` setup first |
| F5. `CGO_ENABLED=0` for the PERF-START server build | 33 ms measured, **proxy only** (C 318 vs A 351) | low, but widens the gap between proxy and native | none | measured; no user-visible gain |
| Not worth doing | | | | |
| `WebAssembly.compileStreaming` / cached compiled module | ~0: streaming compile is already used and finishes in 2.4 ms on the main thread; the cost is instantiate + init | | | trace, row 7 |
| Go init parallelism, libp2p after first paint, lazy bbolt/sqlite | <= 3 ms (libp2p 3.4 ms; there is no bbolt/sqlite in the start path: identity and trust are plain files) | not worth the risk | | `TP` timeline |
| Sidecar hash in background, before or after the window | 0 for the window | risk to the invariant | | section 4 |
| V8 code cache | not measurable with this proxy (each run uses a new browser context; Chromium needs repeat visits in a persistent profile); WebKitGTK/JSC has no equivalent the application controls | | | not evaluated |

Pointers for the techniques (NOT freshly researched or cited in this session; confirm against current sources before an ADR): HTML `rel=modulepreload`
and `rel=preload as=fetch` (whatwg HTML), V8 "WebAssembly compilation pipeline" (streaming compile + Liftoff/TurboFan tiering),
Vite `build.modulePreload`, web.dev "Optimize LCP/loading". 

Recommended order, by gain against risk: F1 (measured, low risk) -> F3 (largest, needs design and tests) -> F2 -> F4. F5 only if the gate should
stop paying for libraries the server build never uses; it does not help the person using the app.

## 6. Achievable startup and proposed budgets

### Chromium proxy (PERF-START, median spawn to editor)

| State | Median | p95 | Basis |
|-------|-------:|----:|-------|
| Today (this host, load ~2) | 351 | 387 | measured, variant A |
| F1 | 334 | 359 | measured, variant B |
| F1 + F2 | ~320 | ~345 | estimated |
| F1 + F2 + F3 (+F4 partly) | ~280-300 | ~330 | estimated |
| F1 + F5 (CGO-free gate build) | 303 | 316 | measured, variant D |

Proposed proxy budgets (for the user to choose; the current gate already uses archived p95 x 1.10):
* **Hold the line now:** median <= 350 ms, p95 <= 400 ms. Already met; stops regressions.
* **After F1:** median <= 335 ms, p95 <= 380 ms (measured).
* **Stretch, after F1-F3, if the user wants it:** median <= 300 ms, p95 <= 350 ms. Do not adopt it before F2/F3 are built and measured; their gains here are estimates.
Measure at rest (load average < 1), poll `/health` at <= 2 ms, and compare like with like (same build flags: the gate's CGO setting changes the number by 33 ms).

### Native window (WebKitGTK)

Not measured, so no absolute budget can be proposed honestly yet. What is known: it pays the same 31-38 ms library load (row 1) and the same Go 7 ms; it adds GTK
window creation and the WebKit web process launch, and replaces V8 by JavaScriptCore (different parse and WASM tiering costs), all unmeasured. The requested number should come from a measurement:
run the existing `inkwell-native-perf` binary inside a nested compositor (weston or cage, not installed here) with timestamps for window map, first paint and `.cm-content`.
Until then use a relative budget, "native median <= proxy median + a margin fixed after the first native measurement", and do not claim a native figure.

## 7. Next steps if the user agrees

1. Decide the proxy budget tier above and whether F5 is wanted in the gate.
2. File F1 as an `L`-style block with a frozen gate; F3 and F2 as separate blocks after a design note (touches `App.svelte`, `workspace`, build config).
3. Build a native startup measurement (nested compositor + timestamps) before any native budget is written.
4. Install `strace` and `perf` on the host to add the syscall and CPU-hotspot evidence missing here (cgo library constructors in row 1 are inferred from the controls, not seen directly).
