# Startup sample sets behind the PB noise model (2026-11-11)

Nine sets of 30 sequential starts (`samples_ms`, the Chromium proxy of PERF-START: server spawn to a usable
`.cm-content`), 270 starts in all, collected between 2026-11-02 and 2026-11-11 on the development host. They show a
level drift of 293-377 ms (+29 %) between sessions with no relevant code change, and 2-3 outliers per set. The model
`scripts/perf/pbmodel.py` fits its noise parameters to them (`python3 scripts/perf/pbmodel.py fit`) and
`pbmodel_test.py` checks that every stress environment dominates the fit. Plan: `docs/plans/2026-11-11-pb-paired-perf.md`
section 2; ADR 0038.

| File | Origin |
|---|---|
| `2026-11-02-reference-start.json`, `2026-11-03-baseline-start.json` | the 2026-11-02 reference and baseline (`docs/perf/baseline/2026-11-02/`) |
| `2026-11-03-mergebase-start.json` | the 2026-11-03 merge-base measurement |
| `2026-11-11-L0-gate-start.json` | the L0 gate run of 2026-11-11 15:51 |
| `2026-11-11-a-head-start.json`, `2026-11-11-a2-head-start.json` | HEAD (f78949b), measured twice |
| `2026-11-11-b-lf-start.json`, `2026-11-11-c-c5s-start.json`, `2026-11-11-d-both-start.json` | LF0a3, C5S, both (uncommitted) |

`SHA256SUMS` seals the files: `sha256sum --strict -c SHA256SUMS`.
