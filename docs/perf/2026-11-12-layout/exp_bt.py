"""Benchtime 1s vs 250ms vs 100ms on one binary (all mux benchmarks): per-process noise and wall time."""
import random, re, subprocess, sys, json, pathlib, time
S = pathlib.Path(sys.argv[1]); b = S / 'bin' / 'c5s-fa-k0.test'
plan = [bt for bt in ('1s', '250ms', '100ms') for _ in range(16)]
random.Random(5).shuffle(plan)
out = open(S / 'bt.jsonl', 'w')
for i, bt in enumerate(plan):
    t0 = time.monotonic()
    text = subprocess.run([str(b), '-test.run=^$', '-test.bench=.', '-test.benchmem', '-test.count=1', '-test.benchtime=' + bt], capture_output=True, text=True, check=True).stdout
    wall = time.monotonic() - t0
    for m in re.finditer(r'^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op', text, re.M):
        out.write(json.dumps({'variant': bt + ':' + m[1], 'ns': float(m[2]), 'wall': wall}) + '\n')
    out.flush()
    if i % 10 == 0: print(i, len(plan), bt, round(wall, 2), flush=True)
print('done', flush=True)
