"""Retake reference: INKWELL_HEAVY_SLOTS=1 scripts/heavy.sh python3 scripts/perf/reference.py."""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import statistics
import subprocess

out = Path(os.environ.get("INKWELL_PERF_OUT", "gates/L0"))
out.mkdir(parents=True, exist_ok=True)

# heavy.sh holds slot 0. Also wait for and hold every other slot (default: 2),
# so an already running default-slot job cannot contaminate the reference.
locks = []
lockdir = Path(os.environ.get('XDG_RUNTIME_DIR', '/tmp')) / 'inkwell-heavy'
for path in sorted(set(lockdir.glob('slot-*.lock')) | {lockdir / 'slot-0.lock', lockdir / 'slot-1.lock'}):
    if path.name == 'slot-0.lock' and os.environ.get('INKWELL_HEAVY_HELD'):
        continue
    stream = path.open('r')
    fcntl.flock(stream, fcntl.LOCK_EX)
    locks.append(stream)
packages = re.search(r'^HOT_PATHS: (.+)$', Path('CLAUDE.md').read_text(), re.M)[1].split()
env = dict(os.environ, GOMAXPROCS='20')
report = {'GOMAXPROCS': 20, 'count': 20, 'cv_definition': 'sample standard deviation / mean, ns/op', 'attempts': []}
def frequency():
    return [line.strip() for line in Path('/proc/cpuinfo').read_text().splitlines() if line.startswith('cpu MHz')]
for attempt in range(1, 4):
    info = {'attempt': attempt, 'load_before': os.getloadavg(), 'cpu_frequency_before': frequency()}
    target = out / f'go-reference-attempt-{attempt}.txt'
    with target.open('w') as stream:
        subprocess.run(['go', 'test', '-run', '^$', '-bench', '.', '-benchmem', '-count=20', *('./'+p for p in packages)], env=env, stdout=stream, stderr=subprocess.STDOUT, check=True)
    info.update(load_after=os.getloadavg(), cpu_frequency_after=frequency())
    samples = {}
    for name, value in re.findall(r'^(Benchmark\S+)\s+\d+\s+([\d.]+) ns/op', target.read_text(), re.M):
        samples.setdefault(name, []).append(float(value))
    assert samples and all(len(v) == 20 for v in samples.values()), samples
    info['cv_percent'] = {k: statistics.stdev(v)/statistics.mean(v)*100 for k,v in samples.items()}
    info['status'] = 'noisy' if max(info['cv_percent'].values()) > 10 else 'pass'
    report['attempts'].append(info)
    report['status'] = info['status']
    (out / 'reference-quality.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(info), flush=True)
    if info['status'] == 'pass':
        break
(out / 'go-reference.txt').write_bytes(target.read_bytes())
p = out / 'thresholds.json'
thresholds = json.loads(Path('docs/perf/baseline/2026-11-02/thresholds.json').read_text())
thresholds['go'].pop('limits', None)
thresholds['go']['reference_sha256'] = hashlib.sha256(target.read_bytes()).hexdigest()
p.write_text(json.dumps(thresholds, indent=2)+'\n')
