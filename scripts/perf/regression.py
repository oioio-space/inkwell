"""PM-5 guard: same-session interleaving, benchstat significance, and measured p95 limits."""
import argparse
import fcntl
import hashlib
import csv
import io
import json
from pathlib import Path
import re
import subprocess
import tempfile
import os

BASE = Path('docs/perf/baseline/2026-11-02')


def significant(delta, p):
    return delta > 5 and p < 0.05


def compare_benchstat(output, informational=False):
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
    assert informational or not failures, failures
    return failures


def compare_e2e(thresholds, fresh):
    for name, rule in thresholds['e2e'].items():
        if rule['status'] == 'deferred':
            continue
        unit = rule.get('unit', 'ms')
        value = fresh[name]['p95_' + unit]
        assert isinstance(value, (int, float)) and 0 <= value <= rule['limit_p95_' + unit], (name, value, rule)


def command(args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True)


def interleaved(binaries, roots, outputs, extra=()):
    for output in outputs:
        output.write_text('')
    for round_number in range(10):
        # Reverse each pair to balance linear drift as well as alternating A/B.
        for side in ((0, 1) if round_number % 2 == 0 else (1, 0)):
            with outputs[side].open('a') as stream:
                subprocess.run([str(binaries[side]), '-test.run=^$', '-test.bench=.',
                                '-test.benchmem', '-test.count=2', *extra],
                               cwd=roots[side], stdout=stream, check=True)


def slowdown_test():
    with tempfile.TemporaryDirectory(prefix='slowdown-', dir=cache()) as folder:
        root = Path(folder)
        source = root / 'fixture_test.go'
        source.write_text('package fixture\nimport ("testing"; "time"; "os")\n'
                          'func BenchmarkSleep(b *testing.B) { d := 10*time.Millisecond; '
                          'if os.Getenv("PERF_SLOW") == "1" { d = 12*time.Millisecond }; '
                          'for i:=0; i<b.N; i++ { time.Sleep(d) } }\n')
        binary = root / 'fixture'
        subprocess.run(['go', 'test', '-c', '-o', str(binary), str(source)], check=True)
        outputs = [root / 'base.txt', root / 'branch.txt']
        for output in outputs:
            output.write_text('')
        for i in range(10):
            for side in ((0, 1) if i % 2 == 0 else (1, 0)):
                with outputs[side].open('a') as stream:
                    subprocess.run([str(binary), '-test.run=^$', '-test.bench=.',
                                    '-test.count=2', '-test.benchtime=5x'],
                                   env=dict(os.environ, PERF_SLOW=str(side)), stdout=stream, check=True)
        result = command(['benchstat', '-format', 'csv', *map(str, outputs)])
        try:
            compare_benchstat(result)
        except AssertionError as exc:
            assert exc.args[0] and isinstance(exc.args[0], list), exc
        else:
            raise AssertionError('injected 20% sleep slowdown accepted')
        print(result, flush=True)


def cache():
    path = Path.home() / '.cache/inkwell'
    path.mkdir(parents=True, exist_ok=True)
    return path


def session(output):
    # Concurrent L0/L1 invocations share an artifact directory; serialize A/B sessions.
    with (cache() / "perf-ab.lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        measure_session(output)


def measure_session(output):
    root = Path.cwd()
    base = command(['git', 'merge-base', 'HEAD', 'main']).strip()
    packages = re.search(r'^HOT_PATHS: (.+)$', (root / 'CLAUDE.md').read_text(), re.M)[1].split()
    output.mkdir(parents=True, exist_ok=True)
    report = {'merge_base': base, 'rounds': 10, 'count': 2, 'packages': []}
    failed = False
    with tempfile.TemporaryDirectory(prefix='perf-ab-', dir=cache()) as folder:
        temporary = Path(folder)
        worktree = temporary / 'base'
        # Private metadata: never write the source checkout's .git.
        repository = temporary / 'repository.git'
        subprocess.run(['git', 'clone', '--bare', '--shared', str(root), str(repository)], check=True)
        git_worktree = ['git', '--git-dir=' + str(repository), 'worktree']
        subprocess.run([*git_worktree, 'add', '--detach', str(worktree), base], check=True)
        try:
            changed = set(command(['git', 'diff', '--name-only', base, '--']).splitlines())
            changed.update(command(['git', 'ls-files', '--others', '--exclude-standard']).splitlines())
            for package in packages:
                paths = {'go.mod', 'go.sum', 'go.work', 'go.work.sum'}
                for tree in (worktree, root):
                    dirs = command(['go', 'list', '-deps', '-test', '-f', '{{.Dir}}', './' + package], tree).splitlines()
                    for directory in dirs:
                        try:
                            relative = Path(directory).relative_to(tree)
                        except ValueError:
                            continue
                        paths.update(name for name in changed if name.startswith(str(relative) + '/'))
                informational = not bool(paths & changed)
                binaries = [temporary / 'base.test', temporary / 'branch.test']
                for tree, binary in zip((worktree, root), binaries):
                    subprocess.run(['go', 'test', '-c', '-o', str(binary), './' + package], cwd=tree, check=True)
                name = package.replace('/', '-')
                outputs = [output / (name + '-' + side + '.txt') for side in ('base', 'branch')]
                interleaved(binaries, [worktree, root], outputs)
                csv_output = command(['benchstat', '-format', 'csv', *map(str, outputs)])
                (output / (name + '.csv')).write_text(csv_output)
                text = command(['benchstat', *map(str, outputs)])
                (output / (name + '-benchstat.txt')).write_text(text)
                status = 'pass'
                try:
                    failures = compare_benchstat(csv_output, informational=True)
                    if failures:
                        status = 'regression'
                        if not informational:
                            failed = True
                except AssertionError:
                    raise  # Missing/incomparable samples fail even for unchanged code.
                report['packages'].append(dict(package=package, informational=informational, comparison=status))
                print(package, 'informational' if informational else 'enforced', status, text, flush=True)
        finally:
            subprocess.run([*git_worktree, 'remove', '--force', str(worktree)], check=True)
    (output / 'session.json').write_text(json.dumps(report, indent=2) + '\n')
    assert not failed, 'significant regression in changed package/dependencies'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--self-test', action='store_true')
    parser.add_argument('--bench', action='store_true', help='measure merge-base and branch in this session')
    parser.add_argument('--output', default='gates/L0/perf-ab')
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
        slowdown_test()
        print('regression-guard self-test passed: injected slowdown detected')
    else:
        assert args.bench or args.e2e, 'fresh --bench or --e2e required'
        if args.bench:
            session(Path(args.output).resolve())
        if args.e2e:
            compare_e2e(thresholds, json.loads(Path(args.e2e).read_text()))
        print('regression-guard passed')


if __name__ == '__main__':
    main()
