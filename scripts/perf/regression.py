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


# benchstat guidance (golang.org/x/perf/cmd/benchstat, "Tips"): interleave the
# before and after runs, use at least 10 and ideally 20 runs, and expect about
# alpha of the compared benchmarks to look significant with no real change
# ("multiple testing"). Hence one sample per process (-test.count=1: samples of
# one process share its heap, page cache and frequency state and are not
# independent), ROUNDS alternated processes per side, and a family-wise
# mitigation: a regression fails the gate only if a fresh confirmation session
# reproduces it (same benchmark, unit and direction, still > 5 % and p < 0.05).
# A null-hypothesis false failure then needs two independent false positives on
# the same benchmark (about alpha**2 per comparison) while a real regression is
# reproduced by construction. Never the reverse: a pass is never rerun.
ROUNDS = 20
DELTA_PERCENT = 5
ALPHA = 0.05
# Units where a larger value is better; every other unit is lower-is-better.
HIGHER_IS_BETTER = ('B/s', 'ops/s')
# sec/op and allocs/op are the declared metrics (thresholds.json); throughput
# units are enforced in their own direction (a gain never fails).
ENFORCED = ('sec/op', 'allocs/op', *HIGHER_IS_BETTER)
# benchstat configuration lines such as "goos: linux" or "pkg: …".
CONFIG = re.compile(r'^[a-z][a-z0-9_-]*:\s')


def significant(delta, p):
    return delta > DELTA_PERCENT and p < ALPHA


def worsening(unit, old, new):
    """Percent change in the bad direction for unit; negative is an improvement."""
    if unit in HIGHER_IS_BETTER:
        old, new = new, old
    return round((new / old - 1) * 100, 9) if old else (float('inf') if new else 0)


def compare_benchstat(output, informational=False):
    """Parse every benchstat CSV table with its own unit and classify each row.

    Returns {'failures', 'other', 'new', 'missing', 'comparisons'}: failures are
    significant worsenings in enforced units, other the same in informational
    units. A benchmark without base samples is new (listed, informational); one
    without fresh samples is missing and fails closed.
    """
    unit = None
    result = {'failures': [], 'other': [], 'new': [], 'missing': [], 'comparisons': 0}
    for row in csv.reader(io.StringIO(output)):
        row = row + [''] * (7 - len(row))
        if not any(row):
            unit = None  # tables are separated by a blank line
            continue
        if not row[0]:
            if row[2] == 'CI':  # a table header names its unit; never inherit the previous one
                unit = row[1]
            continue
        if row[0] == 'geomean' or CONFIG.match(row[0]):
            continue
        assert unit, ('row before any unit header', row)
        name = (unit, row[0])
        if not row[1] and row[3]:
            result['new'].append(name)
            continue
        if not row[3]:
            result['missing'].append(name)
            continue
        match = re.search(r'p=([0-9.]+)', row[6])
        assert match, ('incomparable benchmark', row)
        delta = worsening(unit, float(row[1]), float(row[3]))
        enforced = unit in ENFORCED
        result['comparisons'] += enforced
        if significant(delta, float(match[1])):
            result['failures' if enforced else 'other'].append(dict(unit=unit, name=row[0], worsening_percent=delta, p=float(match[1])))
    assert result['comparisons'], 'no comparable time/alloc benchmarks'
    assert not result['missing'], ('benchmarks without fresh samples', result['missing'])
    assert informational or not result['failures'], result['failures']
    return result


def compare_e2e(thresholds, fresh):
    for name, rule in thresholds['e2e'].items():
        if rule['status'] == 'deferred':
            continue
        unit = rule.get('unit', 'ms')
        value = fresh[name]['p95_' + unit]
        assert isinstance(value, (int, float)) and 0 <= value <= rule['limit_p95_' + unit], (name, value, rule)


def command(args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True)


def interleaved(binaries, roots, outputs, extra=(), envs=(None, None)):
    for output in outputs:
        output.write_text('')
    for round_number in range(ROUNDS):
        # Reverse each pair to balance linear drift as well as alternating A/B.
        for side in ((0, 1) if round_number % 2 == 0 else (1, 0)):
            with outputs[side].open('a') as stream:
                subprocess.run([str(binaries[side]), '-test.run=^$', '-test.bench=.',
                                '-test.benchmem', '-test.count=1', *extra],
                               cwd=roots[side], stdout=stream, check=True, env=envs[side])


def ab_compare(binaries, roots, folder, name, extra=(), envs=(None, None)):
    """One interleaved session, then a confirmation session if anything regressed.

    Returns (status, report): status is 'pass' or 'regression' (confirmed).
    """
    sessions = []
    for attempt in ('', '-confirm'):
        outputs = [folder / (name + attempt + '-' + side + '.txt') for side in ('base', 'branch')]
        interleaved(binaries, roots, outputs, extra, envs)
        csv_output = command(['benchstat', '-format', 'csv', *map(str, outputs)])
        (folder / (name + attempt + '.csv')).write_text(csv_output)
        (folder / (name + attempt + '-benchstat.txt')).write_text(command(['benchstat', *map(str, outputs)]))
        sessions.append(compare_benchstat(csv_output, informational=True))
        if not sessions[0]['failures']:
            break
    first = sessions[0]
    flagged = {(f['unit'], f['name']) for f in first['failures']}
    confirmed = sorted(flagged & {(f['unit'], f['name']) for f in sessions[-1]['failures']}) if len(sessions) == 2 else []
    report = dict(rounds_per_side=ROUNDS, count_per_process=1, comparisons=first['comparisons'],
                  alpha=ALPHA, expected_false_positives=round(first['comparisons'] * ALPHA, 3),
                  flagged=first['failures'], confirmed=[list(c) for c in confirmed],
                  informational_units=first['other'], new_benchmarks=[list(n) for n in first['new']])
    return ('regression' if confirmed else 'pass'), report


def slowdown_test():
    with tempfile.TemporaryDirectory(prefix='slowdown-', dir=cache()) as folder:
        root = Path(folder)
        source = root / 'fixture_test.go'
        # A spin on the monotonic clock allocates nothing (time.Sleep may, through
        # its timer), so allocs/op is exactly 0 on both sides and only time moves.
        source.write_text('package fixture\nimport ("testing"; "time"; "os")\n'
                          'func BenchmarkSleep(b *testing.B) { d := 10*time.Millisecond; '
                          'if os.Getenv("PERF_SLOW") == "1" { d = 12*time.Millisecond }; '
                          'b.SetBytes(1 << 20); for b.Loop() { for start := time.Now(); time.Since(start) < d; {} } }\n')
        binary = root / 'fixture'
        subprocess.run(['go', 'test', '-c', '-o', str(binary), str(source)], check=True)
        envs = [dict(os.environ, PERF_SLOW=side) for side in ('0', '1')]
        status, report = ab_compare([binary, binary], [root, root], root, 'slow', ['-test.benchtime=5x'], envs)
        # benchstat names carry the GOMAXPROCS suffix (Sleep-8).
        confirmed = {unit for unit, name in report['confirmed'] if name.split('-')[0] == 'Sleep'}
        assert status == 'regression' and 'sec/op' in confirmed, report
        assert 'B/s' in confirmed, ('throughput drop not enforced', report)
        # The reverse direction is a 20 % gain in both sec/op and B/s: never a failure.
        status, report = ab_compare([binary, binary], [root, root], root, 'fast', ['-test.benchtime=5x'], envs[::-1])
        regressed = {unit for unit, _ in report['confirmed']}
        assert not regressed & {'sec/op', 'B/s'} and status == 'pass', report
        print(json.dumps(report), flush=True)


def csv_fixture_test():
    header = ',base.txt,,branch.txt,,,\n,%s,CI,%s,CI,vs base,P\n'
    table = lambda unit, rows: header % (unit, unit) + rows + 'geomean,1,,1,,+0%,\n\n'
    row = lambda name, old, new, p: f'{name},{old},1%,{new},1%,x,p={p} n=20\n'
    for delta, p, fails in [(6, .01, True), (6, .05, False), (5, .01, False), (-30, .001, False)]:
        output = table('sec/op', row('B', 100, 100 + delta, p))
        assert bool(compare_benchstat(output, informational=True)['failures']) == fails, (delta, p)
    # Throughput: a B/s gain after a sec/op table neither inherits sec/op nor fails.
    gain = table('sec/op', row('B', 100, 80, .001)) + table('B/s', row('B', 100, 125, .001))
    assert not compare_benchstat(gain)['failures']
    drop = table('sec/op', row('B', 100, 100, .9)) + table('B/s', row('B', 125, 100, .001))
    assert compare_benchstat(drop, informational=True)['failures'][0]['unit'] == 'B/s'
    # A new benchmark is listed, informational, never silently passed.
    added = table('sec/op', row('B', 100, 100, .9) + 'New,,,100,1%\n')
    assert compare_benchstat(added)['new'] == [('sec/op', 'New')]
    # A benchmark that disappeared from the branch fails closed.
    try:
        compare_benchstat(table('sec/op', row('B', 100, 100, .9) + 'Gone,100,1%\n'))
    except AssertionError:
        pass
    else:
        raise AssertionError('missing fresh benchmark accepted')
    # Unknown units are reported, not enforced; B/op is informational too.
    other = table('sec/op', row('B', 100, 100, .9)) + table('p99-µs', row('B', 100, 200, .001)) + table('B/op', row('B', 16, 32, .001))
    assert not compare_benchstat(other)['failures'] and len(compare_benchstat(other)['other']) == 2


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
    report = {'merge_base': base, 'rounds_per_side': ROUNDS, 'count_per_process': 1, 'packages': []}
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
                # Missing/incomparable samples raise, even for unchanged code.
                status, comparison = ab_compare(binaries, [worktree, root], output, name)
                if status == 'regression' and not informational:
                    failed = True
                report['packages'].append(dict(package=package, informational=informational, comparison=status, **comparison))
                text = (output / (name + '-benchstat.txt')).read_text()
                print(package, 'informational' if informational else 'enforced', status, text, flush=True)
        finally:
            subprocess.run([*git_worktree, 'remove', '--force', str(worktree)], check=True)
    (output / 'session.json').write_text(json.dumps(report, indent=2) + '\n')
    assert not failed, 'confirmed significant regression in changed package/dependencies'


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
        assert thresholds['go']['delta_percent'] == DELTA_PERCENT and thresholds['go']['p_less_than'] == ALPHA
        assert ROUNDS >= 10
        csv_fixture_test()
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
