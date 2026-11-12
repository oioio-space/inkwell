"""Pinned to the performance cores (taskset 0-11, GOMAXPROCS unchanged at 20) vs unpinned, all mux benchmarks, 1 s."""
import random, re, subprocess, sys, json, pathlib, os
S = pathlib.Path(sys.argv[1]); b = S / 'bin' / 'c5s-fa-k0.test'
plan = [c for c in ('unpinned', 'pcores') for _ in range(36)]
random.Random(9).shuffle(plan)
out = open(S / 'pin2.jsonl', 'w')
env = dict(os.environ, GOMAXPROCS='20')
for i, c in enumerate(plan):
    prefix = ['taskset', '-c', '0-11'] if c == 'pcores' else []
    text = subprocess.run([*prefix, str(b), '-test.run=^$', '-test.bench=.', '-test.benchmem', '-test.count=1'], capture_output=True, text=True, check=True, env=env).stdout
    for m in re.finditer(r'^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op', text, re.M):
        out.write(json.dumps({'variant': c + ':' + m[1], 'ns': float(m[2])}) + '\n')
    out.flush()
    if i % 10 == 0: print(i, len(plan), c, flush=True)
print('done', flush=True)
