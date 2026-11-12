"""Pinning to P-cores vs unpinned; layout spread when pinned. One sample per process, random order."""
import random, re, subprocess, sys, json, pathlib
S = pathlib.Path(sys.argv[1]); b = S / 'bin'
plan = [('unpinned', b / 'c5s-fa-k0.test', [])] * 16 + [('pcores', b / 'c5s-fa-k0.test', ['taskset', '-c', '0-11'])] * 16
plan += [('pcore0', b / 'c5s-fa-k0.test', ['taskset', '-c', '0'])] * 16
plan += [('pin-' + x.stem, x, ['taskset', '-c', '0-11']) for x in sorted(b.glob('c5s-fa-r*.test')) for _ in range(3)]
plan += [('pin-' + x.stem, x, ['taskset', '-c', '0-11']) for x in sorted(b.glob('c5s-def-r*.test')) for _ in range(3)]
random.Random(11).shuffle(plan)
out = open(S / 'pin.jsonl', 'w')
for i, (cond, binary, prefix) in enumerate(plan):
    text = subprocess.run([*prefix, str(binary), '-test.run=^$', '-test.bench=Send/256$', '-test.count=1'], capture_output=True, text=True, check=True).stdout
    ns = float(re.search(r'BenchmarkSend/256(?:-\d+)?\s+\d+\s+([\d.]+) ns/op', text)[1])
    out.write(json.dumps({'variant': cond, 'ns': ns}) + '\n'); out.flush()
    if i % 20 == 0: print(i, len(plan), cond, ns, flush=True)
print('done', flush=True)
