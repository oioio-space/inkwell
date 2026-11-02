"""PM-5 guard: fixed reference, benchstat significance, and measured p95 limits."""
import argparse
import hashlib
import csv
import io
import json
from pathlib import Path
import re
import subprocess

BASE = Path('docs/perf/baseline/2026-11-02')


def significant(delta, p):
    return delta > 5 and p < 0.05


def compare_benchstat(output):
    metric = None
    checked = 0
    failures = []
    for row in csv.reader(io.StringIO(output)):
        if len(row) >= 7 and row[1] in ('sec/op', 'B/op', 'allocs/op'):
            metric = row[1]
        elif len(row) >= 7 and row[0] and row[0] != 'geomean' and metric in ('sec/op', 'allocs/op'):
            # Missing or incomparable fresh benchmarks must not silently pass.
            assert row[1] and row[3], row
            match = re.search(r'p=([0-9.]+)', row[6])
            assert match, row
            checked += 1
            old, new = float(row[1]), float(row[3])
            delta = round((new / old - 1) * 100, 9) if old else (float('inf') if new else 0)
            if significant(delta, float(match[1])):
                failures.append(row)
    assert checked, 'no comparable time/alloc benchmarks'
    assert not failures, failures


def compare_e2e(thresholds, fresh):
    for name, rule in thresholds['e2e'].items():
        if rule['status'] == 'deferred':
            continue
        unit = rule.get('unit', 'ms')
        value = fresh[name]['p95_' + unit]
        assert isinstance(value, (int, float)) and 0 <= value <= rule['limit_p95_' + unit], (name, value, rule)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--self-test', action='store_true')
    parser.add_argument('--bench')
    parser.add_argument('--e2e')
    args = parser.parse_args()
    thresholds = json.loads((BASE / 'thresholds.json').read_text())
    assert thresholds['go']['delta_percent'] == 5 and thresholds['go']['p_less_than'] == 0.05
    assert 'limits' not in thresholds['go']
    assert thresholds['go']['reference_sha256'] == hashlib.sha256(Path(thresholds['go']['reference']).read_bytes()).hexdigest()
    assert thresholds['go']['metrics'] == ['sec/op', 'allocs/op']
    assert thresholds['e2e_multiplier'] == 1.10
    for rule in thresholds['e2e'].values():
        if rule['status'] == 'deferred':
            assert rule['target_lot'] and rule['reason']
        else:
            assert rule['limit_p95_ms'] == rule['baseline_p95_ms'] * 1.10
    assert (BASE / 'go-reference.txt').is_file()
    if args.self_test:
        assert significant(5.01, .049) and not significant(5, .01) and not significant(6, .05)
        for delta, p, fails in [('6%', '.01', True), ('6%', '.05', False), ('5%', '.01', False)]:
            output = ',sec/op,CI,sec/op,CI,vs base,P\nB,100,,%s,,%s,p=%s n=10\n' % (100+float(delta[:-1]), delta, p)
            try:
                compare_benchstat(output)
            except AssertionError:
                assert fails
            else:
                assert not fails
        fixture = {'e2e': {'start': {'status': 'pass', 'limit_p95_ms': 110}}}
        compare_e2e(fixture, {'start': {'p95_ms': 110}})
        try:
            compare_e2e(fixture, {'start': {'p95_ms': 111}})
        except AssertionError:
            pass
        else:
            raise AssertionError('latency regression accepted')
        print('regression-guard thresholds loaded; self-test passed (no L0 comparison claimed)')
    else:
        assert args.bench or args.e2e, 'fresh --bench or --e2e required'
        if args.bench:
            result = subprocess.run(['benchstat', '-format', 'csv', str(BASE / 'go-reference.txt'), args.bench], check=True, capture_output=True, text=True)
            compare_benchstat(result.stdout)
        if args.e2e:
            compare_e2e(thresholds, json.loads(Path(args.e2e).read_text()))
        print('regression-guard passed')


if __name__ == '__main__':
    main()
