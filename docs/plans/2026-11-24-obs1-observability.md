# OBS1 — local observability: one point for metrics and debug state (W86)

Status: **proposal, 2026-11-24, revised 2026-11-25 after the blind Opus review** (`gates/OBS1/REVIEW.md`, « accept
with corrections »; integrator's decisions, CLAUDE.md « Autonomie »). Rigour: standard (no peer bytes; local data of
the person): blind Opus review plus Gemini when its quota allows, never OpenRouter (method v2), and `security-auditor`
on the privacy rules before the freeze. Gate `gates/OBS1.toml`, witnesses `gates/OBS1/WITNESSES.md`. Place: after EX0
in `PROGRESS.md`; requirement W86. No interface in this block. **Cut to OBS2** (its line in `PROGRESS.md`): export
with document names, typing speed (from words written and active time), adjustable retention, and the typing,
compilation and sync debug providers (→ RP1).

## 1. Ground and model

Rule 10 of `docs/ux-principles.md` (nothing leaves without an explicit gesture; local statistics under the person's
control) is the only statement of the rule; spec §11 O-2, O-4, O-7 are unchanged. OBS1 adds the point `metrics` to the
registry of ADR 0027 (amendment noted there) and turns O-4 into a local daily store. Shape from the OpenTelemetry
metrics API (synchronous instruments called inline; observable ones read « only when the Meter is being observed »,
<https://opentelemetry.io/docs/specs/otel/metrics/api/>, read 2026-11-24), never its SDK: no exporter exists.

## 2. The point and where each side lives

**Metric** (data in a manifest, validated by `manifest.ts` with the other points' bounds): `id` (`<ext>.<name>`, unique;
a taken id is refused in either order), `kind` ∈ {`counter` (events), `duration` (ms), `volume` (bytes, words)},
`unit` ∈ {`count`, `ms`, `bytes`, `words`}, consistent with the kind; `scope` ∈ {`app`, `document`}; `label` (i18n
key). Every metric of this slice is always on (an opt-in cost class comes with OBS2's first probe). The daily
aggregation follows the kind: sum for counter and volume, sum and count for duration. **Debug provider**: `debug:
{ id, label, entry }`, whose module exports `read()` returning a flat record of values; called only on read, and every
read passes through the registry's check of rule V.

**Value rule (V)**: a value is a finite number ≥ 0, a boolean, an enum value declared in the manifest, or a code of the
diagnostics taxonomy (`docs/plans/2026-11-09-diagnostics.md` §3.3). No other string: never text, a name, a path, a
keystroke, a peer id, an address or a secret. Checked at the manifest, at each `Declare` and batch Go receives, at
each debug read.

- **Go** (`internal/obs`, created here, one role: local observability): `Counter`, `Duration`, `Volume` declared in
  code with the same descriptor (Go is the single source of its own descriptors; the webview merges them with the
  manifests' into one registry view, a clash being a registry error); `Add` on a nil instrument is a no-op; the day
  store; a `StatsService` binding in `internal/app`. Network bytes (sent, received, direct and relayed) are counted in
  `internal/transport/mux`, where `BenchmarkSend` runs: one plain counter per link, owned by that link's writer or
  reader, summed only when read (no atomic shared by all links on the hot path); connections opened likewise.
  Compilations, failures by code, compile time: `internal/app`'s preview path. The one Go debug provider is the
  network's: links per peer (session-salted pseudonyms, O-2), last close codes.
- **Webview** (`frontend/src/obs/`, contributions in a built-in `frontend/src/ext/activity/`): words written, active
  time per document (an edit stamps a time; gaps over 60 s are idle), documents opened, open time, changes received.
  **Words written** are counted from the local transactions' `ChangeSet`s (`userEvent` `input`, `delete`, `paste`),
  composed into one pending set per document and mapped through every remote transaction; at idle (60 s, or close)
  the touched line ranges are counted with `countPlainWords` before and after, and the positive difference is added.
  Per transaction this is one `compose` or `map` of the pending set (`ChangeSet.compose`, `ChangeSet.map`,
  <https://codemirror.net/docs/ref/#state.ChangeSet>, read 2026-11-25: linear in the sets' sections, that is in the
  edits since the last idle, never in the document size); the status bar's count is not used. `StatsService.Declare(descriptors)` once at start; Go
  validates it under rule V and never trusts it. **No IPC on the typing path**: one `StatsService.Flush(batch)` per
  minute, and on `pagehide` (as `editor/amSync.ts` flushes), carries the deltas of declared ids. `src/obs` is imported
  only by `src/ext/activity` and the editor's setup; neither these nor any other importer may hold a runtime import of
  network, transport or CRDT code, a binding other than `StatsService`, or a network API (type-only imports are erased
  and allowed). A document is keyed by its Automerge
  document id (random, never derived from a name); Go stores `HMAC-SHA-256(stats key, id)` truncated to 8 bytes
  (O-2's local pseudonym), never the id and never an unkeyed hash of it.

## 3. Storage, retention, control

`<INKWELL_HOME>/stats/` (0700; files 0600): `key` (32 random bytes), `settings.json`, `v1/YYYY-MM-DD.json` (local
calendar day) holding `{ "v": 1, "day", "records": [{ "m", "k", "sum", "n"? }] }`. Written with
`store.WriteAtomic` at most once a minute and on close: a crash loses at most one interval, never a file. Bounds: 1024
records and 128 KiB per day (excess dropped and counted in `obs.dropped`), retention fixed at 400 days, older files
removed at start and at day change: at most ≈ 51 MiB in all. `obs.Open("")` is an error, never a fallback to the real
profile (CLAUDE.md « Pièges connus »).

Control (the screen comes with UX1): read all; export the pseudonymous records to a path the person chose (a path
inside an open project root is refused with a clear message, so stats never enter a synced folder); erase (every file
and the key, so old pseudonyms can no longer be linked); switch off (no file written; in-memory values still serve
« Détails techniques »).

## 4. Rules and their tests (named in the gate; a mutant per rule, block OBS1)

| Rule | Statement | Test |
|---|---|---|
| M-1 | A `metrics` or `debug` contribution is validated like the other points | Vitest |
| M-2 | A free-text, unbounded, unknown-enum or kind-inconsistent field is refused with its reason | Vitest |
| M-3 | A metric id already taken (manifest or Go descriptor) is refused in either order | Vitest |
| V-1 | Every debug read passes V; a provider breaking it is reported, its value dropped | Vitest |
| V-2 | Go refuses a `Declare` or a batch with a string outside an enum, NaN, a negative, an undeclared id or over its bound | Go |
| V-3 | The network debug snapshot holds only safe fields; a peer appears as a session-salted pseudonym | Go |
| T-1 | Words written count only the person's own edits, mapped through remote edits in the same idle window, at a cost per transaction independent of the document size | Vitest |
| T-2 | Active time stops after 60 s without an edit | Vitest |
| S-1 | Daily aggregation by kind, per local day | Go |
| S-2 | A crash between flushes loses at most one interval; the file stays whole | Go |
| S-3 | Retention removes days older than 400; the size bound drops and counts | Go |
| P-1 | Store and export of a seeded session hold no document text, keystroke, name, path, raw or hashed document id, peer id or key byte; the export's document keys are exactly HMAC(key, id)[:8] | Go + gate |
| P-2 | The store lives under `INKWELL_HOME`, outside every project root; an empty home is refused | Go |
| P-3 | Nothing leaves: a two-peer Go session whose transport fails on any byte carrying a seeded stats value; a webview session (activity on, typing, flush) whose repository and network adapter see only the person's edits; the obs modules and every importer reach no network or CRDT code and no binding but `StatsService` | Go + Vitest + gate |
| P-4 | Export writes only the asked path, refused inside an open project root | Go |
| P-5 | Erase removes every file and the key | Go |
| P-6 | Switched off, no file is written; in-memory values still read | Go |
| Z-1 | Go `Add` allocates nothing and takes no lock (nil instrument included) | Go |
| Z-2 | Typing makes no bridge call until the flush | Vitest |
| Z-4 | A debug provider runs only when read | Vitest |
| Z-5 | No confirmed regression of the touched `HOT_PATHS` packages (`scripts/perf/regression.py --bench`: merge-base vs branch, 20 interleaved processes per side, benchstat, sec/op and allocs/op, confirmed); `internal/transport/mux` is touched, so enforced | gate |
| Z-6 | PERF-START paired (ADR 0038) passes against the merge-base | gate |

PERF-TYPE stays a proxy (`scripts/perf/baseline.py`, Chromium only) and cannot judge a few nanoseconds: the typing path
is held by Z-2 and T-1's cost bound at the cheapest layer that keeps the defect class (method v2), not by a timing.

## 5. Redaction

Debug state never holds a secret or text; a later consent of RP1 is the only way to add either, out of this block.

## 6. Consumers (later blocks; they only read)

Statistics page (activity calendar per day, network volume) → UX1, placed by the function tree (W85); report (gathers
debug state; the person reviews everything before sending) and debug page → RP1.
