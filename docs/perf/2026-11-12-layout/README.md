# Code-layout evidence for block RG (2026-11-12)

Plan: `docs/plans/2026-11-12-rg-layout-robust-guard.md`. Host: 13th Gen Intel Core i7-1370P (6 performance cores,
8 efficiency cores), Linux, go1.27.4. Every sample is one process (`-test.count=1`, 1 s benchtime unless stated).

- `mux-diag/`: the original diagnosis. `r1-*.txt`, `ctl/r2-*.txt`: BenchmarkSend interleaved runs (20 per side, `ab.sh`)
  of HEAD (`base`) and block C5S; `*.Send.asm` and `*.nm`: identical machine code for mux; offsets moved 32 -> 0.
- `exp.jsonl`: layout experiment (`exp_build.sh`, `exp_run.py`): fixed variants x 12 (base/c5s, default alignment or
  `-funcalign=64`, with 0 or 1 pad slot) and 36 random layouts x 4; pads by `genpad.py`; summary by `analyze.py`.
- `pin.jsonl` (`exp_pin.py`), `pin2.jsonl` (`exp_pin2.py`, 36 vs 36): pinning to the performance cores, rejected.
- `bt.jsonl` (`exp_bt.py`): 1 s vs 250 ms vs 100 ms benchtime; shorter is much noisier.
- `muxdiag.jsonl`: Send/256 rows of mux-diag in the model's input format.
- `rgsim.py`, `sim1.log`, `sim2.log`: the model's prototype and its outputs (single look grid; full rule).
- `genpad2.py`, `padcheck2.sh`, `padcheck2.txt`: the minimal pad mechanism leaves inlining/escape decisions, normalized
  assembly and data of mux unchanged across pad sizes; `rgsim2.py`, `sim3.log`, `sim4.log`: review round 2 model.
