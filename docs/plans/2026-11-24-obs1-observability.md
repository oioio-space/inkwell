# OBS1 — local observability: one point for metrics and debug state (W86)

Status: **proposal, 2026-11-24**, decided by the integrator (CLAUDE.md « Autonomie »). Rigour: standard (no peer
bytes; local data of the person): blind Opus review plus Gemini when its quota allows, never OpenRouter (method v2,
« When Codex is absent »), and the `security-auditor` agent on the privacy rules before the freeze. Place: right after EX0 in
`PROGRESS.md` « Cap à long terme »; requirement W86. No interface in this block: consumers come later and only read.

## 1. What it builds on, and the rule it obeys

- The privacy rule has one home, rule 10 of `docs/ux-principles.md`: nothing leaves the device without an explicit
  gesture; local statistics are allowed, under the person's control. OBS1 adds no other statement of it.
- Architecture spec §11: O-2 (nothing of the document in a log), O-4 (in-memory counters shown in « Détails
  techniques »), O-7 (the canary test), §11.1 (the resource view, RS-1…RS-8). OBS1 turns O-4 into a registry point
  and a local daily store; it changes none of O-1…O-7.
- The contribution registry of EX0 (ADR 0027, `frontend/src/extensions/`): OBS1 adds one point, `metrics`, so a
  core module, a built-in extension or a plugin declares its own metrics and debug state without touching any other
  module (W83).
- Model: the OpenTelemetry metrics API (synchronous instruments called inline: Counter, UpDownCounter, Histogram,
  Gauge; asynchronous ones whose « callback functions will be called only when the Meter is being observed »,
  <https://opentelemetry.io/docs/specs/otel/metrics/api/>, read 2026-11-24). OBS1 takes the shape, not the SDK: no
  exporter exists, so nothing can be sent. Go's `runtime/metrics` stays the source for runtime figures.

## 2. The point's schema

A contribution of the `metrics` point is data in the manifest (validated by `manifest.ts`, same bounds and rules as
the other points), its code an ordinary module entry:

| Field | Values |
|---|---|
| `id` | `<extension>.<name>`, unique in the registry (conflict = registry error, as for commands) |
| `kind` | `counter` (sum), `duration` (sum of ms), `gauge` (last), `histogram` (fixed buckets, declared) |
| `unit` | `count`, `words`, `ms`, `bytes`, `chars-per-min` |
| `scope` | `app`, `project`, `document` (document and project keys are local pseudonyms: HMAC with the profile's key, as O-2) |
| `cost` | `free` (an increment, always on) or `opt-in` (its probe is installed only once the person enables it) |
| `label` | i18n key (the consumers' words; glossary rules) |

A **debug provider** is the same point's second form: `debug: { id, label, read() }`, where `read()` returns current
state, the last errors and the effective settings. It is called only on demand (debug page, report), never on a timer.

Value rule, checked by the validator and by a test of every provider: numbers, booleans, enums declared in the
manifest, stable event names and error codes of the diagnostics taxonomy (`docs/plans/2026-11-09-diagnostics.md`
§3.3). Never free text, document text, file or project names, keystrokes, peer addresses or secrets.

Go modules (network, compilation, store) register through one small Go package created by this block
(`internal/obs`: `Counter`, `Duration`, `Gauge` over `atomic` values, a `Snapshot()` read by the bridge). The webview
merges both into one registry view; Go never sees document data (invariant of ADR 0011 unchanged).

## 3. Storage, retention, control

- **Where:** under the profile root (`INKWELL_HOME`, `<profile>/stats/`, 0600; one profile per instance, ADR 0037),
  never the project folder: so never
  in Automerge, the BLAKE3 store, the Git projection, a backup or a sync stream. Proof: a test that the store path is
  outside every project root, and a sync test that no stats byte reaches a peer.
- **Shape:** one record per day and key (`metric`, `scope key`) holding the aggregate (sum, last, bucket counts). Written
  through `internal/store`'s atomic write at most once a minute and on close (WC1's drain path), so a crash loses at
  most a minute of counts, never the file.
- **Retention:** 400 days (a year-long activity calendar plus a margin), older days removed at start; configurable
  down to 0.
- **The person's control** (settings exist in this block, their screen comes with the stats page): see everything
  (every stored record is readable by the consumers), export (a JSON file chosen by the person; pseudonyms unless she
  ticks « avec les noms des documents »), erase all, switch off (no record written, in-memory counters still serve
  « Détails techniques »). `opt-in` metrics are off by default.

## 4. Zero cost when unused

- A `free` metric is an in-memory increment (`atomic.Int64.Add` in Go, a number in the webview); no allocation, no
  lock on the hot path; flush once a minute.
- An `opt-in` metric (typing speed) installs no listener and no timer until enabled.
- Debug providers run only when read.
- Proof: PERF-START by the paired rule of ADR 0038 (no regression of the start budget, `docs/engineering-principles.md`
  §3), PERF-TYPE unchanged with typing speed off, and benchstat (`-count 10`, `-benchmem`) on the `HOT_PATHS` packages
  touched (`internal/transport/frame`, `internal/transport/mux` for bytes sent and received): no significant slowdown,
  0 added allocations.

## 5. First instrumented modules (no interface)

| Module | Metrics | Debug state |
|---|---|---|
| typing | words written (counter, document), active time per document (duration, idle after 60 s), typing speed (opt-in) | input mode, last long task |
| network | bytes sent and received (counter, app), connections opened, relayed vs direct share | links per peer (pseudonyms), last close reasons (codes) |
| compilation | compilations, failures (codes), duration histogram | sidecar state, last exit code |
| sync | documents opened and open time (duration), sync sessions, conflicts resolved | per-document sync state (enum) |

## 6. Consumers (later blocks; they only read)

A personal statistics page (activity calendar per day like GitHub's contribution graph, typing speed, network
volume) → UX1, placed by the function tree (W85); the report (RP1: gathers debug state; the person reviews everything
before sending; no text or secret without explicit consent) → RP1; a debug page (developer mode) → RP1.

## 7. Gate criteria proposed (`gates/OBS1.toml`, frozen before the code)

1. `manifest`: the `metrics` point validates; a free-text, unbounded or unknown-enum value is refused (named tests).
2. `values`: every provider's `read()` passes the value rule; the O-7 canary extended to the stats store and every
   debug read.
3. `local-only`: store path under `INKWELL_HOME` and outside project roots; no stats byte in a sync stream (Go
   integration test, controlled time); the tests' temporary-profile check covers it (CLAUDE.md « Pièges connus »).
4. `store`: daily aggregation, atomic write, crash between flushes loses ≤ one interval, retention, erase, switch-off.
5. `opt-in`: with typing speed off, no keydown listener registered (Vitest).
6. `perf`: PERF-START paired, PERF-TYPE, benchstat on the touched `HOT_PATHS` packages.
7. `mutants`: one per rule above (value rule, path rule, switch-off, opt-in probe), each caught.
