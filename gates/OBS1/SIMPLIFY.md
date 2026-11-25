# OBS1 - /simplify over the block's diff

Reviewer: Claude Opus 5.5, the implementing session (step 3, 2026-11-25), over the whole diff of the worktree
`inkwell-obs1` against `main` 700face; the integrator's own review and Gemini's (after the smoke) follow.
Checklists: `go-style-guide`, `use-modern-go`, `vuln-guard`, `secret-guard`, `helper-ergonomics`.

## Applied

- One pseudonym helper (`internal/obs/obs.go`, `pseudonym(key, id)`): the store's document key and the network
  snapshot's peer key were two copies of HMAC-SHA-256 truncated to 8 bytes; mutants OB11 and OB15 moved with it.
- A debug provider's entry loads through the registry's one `load()` (cache by origin, folder and entry) instead of a
  second map of its own; mutant OB9 moved with it.
- The test mocks of `StatsService` are typed, so the batches are read with `([batch]) => batch` instead of casts.
- mux, measured with benchstat (interleaved A/B, 10 runs per side, `heavy.sh --exclusive`): a counter per link with an
  atomic per Send cost +18 to +23 % on `BenchmarkSend/256`; the counters are now held back in the pooled send buffer
  (plain fields, owned by whoever holds the buffer), the common case inlined in Send (pointer comparison of the peer
  string, `ready()` cost 31), flushed to the link's atomic at 128 KiB, at a peer change, after each read (epoch) or
  when the pool lets the buffer go (`runtime.AddCleanup`). Last A/B: 256 B +3.05 % (p = 0.050), 16 KiB ~ (p = 0.17).
- Labels of the metrics are i18n keys of their own namespace (`metric.*`, fr and en), not the document statistics'
  `stats.*` keys they first collided with.

- Blind review of the diff (« accept with fixes », 5 findings), each fixed after a test that failed first:
  1. (major) `StatsService.Export(path)` let the webview name any path. Root cause: the export copied the shape of a
     store method instead of the preview's export, where Go asks the person. Now `Export()` takes no path; Go opens a
     native save dialog (`obs.Options.Choose`, `app.Options.StatsChoose`, JSON filter), "" is a cancel, no dialog is
     `ErrNoChooser` (`TestOBS1P4ExportOnlyAskedOutsideProjects`: dialog opened once per export, cancel writes nothing).
  2. (major) Durations counted too many measures. Root cause: `Store.add` read a missing `n` as one measure, but a sum
     split over several deltas (webview flush, Go drain, or a drain between its two `Swap`s) carries its count in the
     first part only. Now `n` is explicit everywhere (`TestOBS1S1DailyAggregation`: a 30 h opening, split by the webview
     or by Go's drain, is stored with N = 1); mutant OB12 now puts the default back.
  3. (minor) A clock going back across midnight replaced that day's file. Root cause: `roll()` started an empty day
     instead of reading the day's file as `Open` does; it now loads it (S-1: 15 + 1 words, openings kept).
  4. (minor) Blob traffic was not counted. Root cause: only the channels split by `mux` had counters, the bulk lane
     has none. `mux.Counted` gives that lane the same per-link counters (one lookup and one atomic per chunk) and the
     minute's sample sums both per peer (`TestCountedLaneCountsPerLink`). Cost, benchstat of `internal/blob`
     `BenchmarkFetch` with both lanes counted, interleaved, 10 runs per side, exclusive: sec/op 56.24 ms → 56.88 ms
     `~` (p = 0.739), allocs/op +0.03 % (one link created per peer); no OBS2 entry needed.
  5. (minor) `Declare` bounded each call, not the total. Root cause: the bound was checked on the argument; it is now
     on the declared metrics plus the new ones (V-2: 200 then 100 more refused, the same 200 again accepted).

## Set aside

- Counting words per transaction (word delta of the touched lines at each key): simpler, but its cost is the line's
  length per key (a one-line document of megabytes); the plan's change-set design (compose or map per transaction,
  counting at idle) is kept.
- A seqlock or `//go:norace` plain counter read across goroutines for exact sent bytes on read: a data race by the Go
  memory model; the documented lag (one read, or 128 KiB per send buffer) is the price, and the statistics are
  sampled once a minute.
- `Store.bump` (used for `obs.dropped` only) folded into `add`: it bypasses the day's bounds on purpose; kept apart so
  the bound check stays one place.
- Network bytes split direct or relayed by the link's kind at each minute's sample, not per byte: the mux does not know
  the path; a link that changes kind within a minute is attributed to its kind at the sample.
- « Last close codes » in the network snapshot: the transport's `Link` holds no close code yet; the snapshot carries
  outages, resyncs and the last outage's length instead (→ OBS2, with the sync and typing providers).

## Verified

- Rule V on both sides: `Declare` and each batch checked by Go (V-2), each debug read by the registry (V-1); no string
  value but an enum or a taxonomy code; debug key prefixes limited to a 16-hex pseudonym or an index.
- No Go code reads document content; no CRDT or network import in `frontend/src/obs` or `src/ext/activity`
  (`local-only` green); `internal/obs` reaches only `internal/store`.
- `obs.Open("")` is an error; files 0600 under `stats/` 0700; writes through `store.WriteAtomic`; export refused inside
  the open project, symlinks resolved on both sides.
- Every OBS1 mutant caught (22 of 22, `mutants.py OBS1`), after the review fixes too.

## After the gate

`mise run pregate` on main found six golangci-lint findings the gate does not run (it has no lint criterion; a block gate that skips lint → CI3): an unchecked deferred `Close` and two `real` shadows in `internal/obs/store.go`, gofumpt in `internal/app/preview.go` and `stats.go`, and `ctx` after `t` in `drainBytes`. The pre-commit gosec scan then flagged three lines, annotated with their reason: `unsafe.StringData` in `internal/transport/mux/traffic.go` is read only (same data pointer and length is the same immutable string) and the duration conversion in `internal/obs/meter.go` follows a `t >= 0` check. Fixed without behaviour change; `mise run lint` and the `internal/obs`, `internal/transport/mux` and `internal/app` tests pass again.
