"""Randomised interleaved runs of the layout variants; one sample per process."""
import random, re, subprocess, sys, json, pathlib
S = pathlib.Path(sys.argv[1]); bins = sorted((S / 'bin').glob('*.test'))
core = {'base-def-k0', 'c5s-def-k0', 'c5s-def-k1', 'base-fa-k0', 'c5s-fa-k0'}
plan = [b for b in bins for _ in range(12 if b.stem in core else 4)]
random.Random(7).shuffle(plan)
out = open(S / 'exp.jsonl', 'w')
for i, b in enumerate(plan):
    text = subprocess.run([str(b), '-test.run=^$', '-test.bench=Send/256$', '-test.count=1'], capture_output=True, text=True, check=True).stdout
    ns = float(re.search(r'BenchmarkSend/256-\d+\s+\d+\s+([\d.]+) ns/op', text)[1])
    out.write(json.dumps({'variant': b.stem, 'ns': ns}) + '\n'); out.flush()
    if i % 20 == 0: print(i, len(plan), b.stem, ns, flush=True)
print('done', flush=True)
