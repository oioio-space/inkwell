# OBS1 - blind review of the spec and gate (round 1)

Reviewer: blind Opus review, 2026-11-25, with the standard-tier checklist (Gemini to follow when its quota allows).
Verdict: **accept with corrections**, 3 major findings. Each one is answered at its root in the plan and in
`gates/OBS1.toml`; the integrator's cuts are recorded on the OBS2 line of `PROGRESS.md` and in W86's « où ».

| # | Finding | Answer |
|---|---|---|
| 1 (major) | P-3 scanned only `src/obs`: an importer or a side-effect import could reach the network | `local-only` scans `src/obs`, `src/ext/activity` and every module importing `src/obs`; side-effect, dynamic and re-export imports caught; only `StatsService` from the bindings; Vitest P-3 (fake repo and network adapter) and Go P-3 seeded with a distinctive value |
| 2 (major) | T-1 tied words written to the status bar's count, which mixes remote edits | words from the local `ChangeSet`s composed and mapped through remote ones, counted at idle in the touched lines with `countPlainWords`; O(edits) per transaction named in the test title |
| 3 (major) | P-1 grep could not tell a pseudonym from an unkeyed hash | the gate reads `stats/key`, requires the export's document keys to be exactly HMAC(key, id)[:8], refuses sha256(id) and the key's bytes |
| 4 | `forms()` missed encodings | percent-encoding, base64 at three alignments, UTF-16 LE/BE, raw bytes (self-tested on 300 random cases); seeds and count drawn by the gate, passed with `-args` |
| 5 | `Declare` was trusted | `StatsService.Declare` validated by Go under rule V: `TestOBS1V2DeclareAndBatchValueRule` |
| 6 | Network counters off any benchmark, one shared hot atomic | per-link counters in `internal/transport/mux` (`BenchmarkSend`), summed on read: `TestOBS1Z5LinkCountersSummedOnRead`; Z-5 enforced on mux |
| 7 | No mutant for M-1; vitest `name` not tied to the gate's titles | a mutant per frontend title, `name` equal to one of them, checked by `mutants` |
| 8 | Export could land in a synced folder | a path inside an open project root is refused with a clear message (P-4) |
| cuts | Scope over a standard slice | names export, typing speed and Z-3, adjustable retention, three debug providers → OBS2 / RP1; bench unchanged (fits G-1) |
