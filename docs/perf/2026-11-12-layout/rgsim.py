"""Prototype of the RG model (scratch): error rates of guard rules under measured noise and layout effects.

Noise: per-process residuals of mux BenchmarkSend/256 (log ns minus its variant's median), bootstrapped from the
mux-diag runs and today's layout experiment (unpinned). Layout: the measured phase effect (two-point, log 1.063) and a
page-level random effect N(0, sigma_l). Rule: N pairs, fresh independent layout per process, random order per pair,
exact one-sided Wilcoxon signed-rank lower bound of the median log ratio > log(1 + m0) at level alpha.
"""
import collections, json, math, os, random, statistics as st, sys
from multiprocessing import Pool
sys.path.insert(0, '/home/mathieu/GolandProjects/inkwell/scripts/perf')
from paired import critical, hl_bounds  # noqa: E402

PHASE = math.log(1.063)

def residuals(paths, keep):
    by = collections.defaultdict(list)
    for p in paths:
        for line in open(p):
            r = json.loads(line)
            if keep(r['variant']):
                by[r['variant']].append(math.log(r['ns']))
    return [x - st.median(v) for v in by.values() if len(v) >= 3 for x in v]

def lower_exceeds(d, theta, c):
    """L = W[c] > theta  <=>  #(Walsh averages <= theta) <= c  <=>  T-(theta) <= c (ranks of |d - theta|)."""
    e = sorted((abs(x - theta), x - theta) for x in d)
    t_minus = sum(rank for rank, (_, v) in enumerate(e, 1) if v <= 0)
    return t_minus <= c

def pairs(rng, noise, n, delta, design, sigma_l, drift, scale):
    d = []
    for i in range(n):
        t = drift * i / n
        if design == 'fixed':
            la, lb = 0.0, PHASE
        elif design == 'phase':
            la, lb = PHASE * rng.getrandbits(1), PHASE * rng.getrandbits(1)
        else:
            la = lb = 0.0
        la += rng.gauss(0, sigma_l); lb += rng.gauss(0, sigma_l)
        a, b = la + scale * rng.choice(noise), lb + scale * rng.choice(noise) + delta
        if design == 'ordered':      # mutant: reference always first, drift and a carry-over penalise B
            b += drift / n + 0.004
        else:                        # random order: the drift between the two starts has a random sign
            if rng.getrandbits(1): a += drift / n
            else: b += drift / n
        d.append(b - a + t - t)
    return d

def shared(rng, noise, n, delta, sigma_l, k, scale):
    ua = [rng.gauss(0, sigma_l) for _ in range(k)]; ub = [rng.gauss(0, sigma_l) for _ in range(k)]
    return [ub[i % k] - ua[i % k] + scale * (rng.choice(noise) - rng.choice(noise)) + delta for i in range(n)]

def mw_p_greater(a, b):
    """One-sided Mann-Whitney p for b > a, normal approximation (no ties in continuous data)."""
    n1, n2 = len(a), len(b)
    u = sum((y > x) + 0.5 * (y == x) for x in a for y in b)
    mu, sd = n1 * n2 / 2, math.sqrt(n1 * n2 * (n1 + n2 + 1) / 12)
    return 0.5 * math.erfc((u - mu) / sd / math.sqrt(2))

def incumbent_session(rng, noise, delta, design, sigma_l, scale, n=20):
    la, lb = (0.0, PHASE) if design == 'fixed' else (rng.gauss(0, sigma_l), rng.gauss(0, sigma_l))
    a = [la + scale * rng.choice(noise) for _ in range(n)]; b = [lb + scale * rng.choice(noise) + delta for _ in range(n)]
    worse = math.exp(st.median(b) - st.median(a)) - 1
    return worse > 0.05 and 2 * min(mw_p_greater(a, b), 1 - mw_p_greater(a, b)) < 0.05

def trial(args):
    kind, seed, trials, n, alpha, m0, delta, design, sigma_l, drift, scale = args
    rng = random.Random(seed); noise = NOISE
    c = critical(n, alpha); theta = math.log1p(m0); hits = 0
    for _ in range(trials):
        if kind == 'new':
            hits += lower_exceeds(pairs(rng, noise, n, delta, design, sigma_l, drift, scale), theta, c)
        elif kind == 'shared':
            hits += lower_exceeds(shared(rng, noise, n, delta, sigma_l, 4, scale), theta, c)
        elif kind == 'point':
            hits += hl_bounds(pairs(rng, noise, n, delta, design, sigma_l, drift, scale), alpha)[1] > math.log(1.05)
        else:  # incumbent: two sessions, both must flag
            hits += incumbent_session(rng, noise, delta, design, sigma_l, scale) and incumbent_session(rng, noise, delta, design, sigma_l, scale)
    return hits

def rate(pool, kind, trials, **kw):
    order = ['n', 'alpha', 'm0', 'delta', 'design', 'sigma_l', 'drift', 'scale']
    defaults = dict(n=64, alpha=1e-3, m0=0.01, delta=0.0, design='canon', sigma_l=0.0, drift=0.0, scale=1.0)
    defaults.update(kw)
    chunks = 20
    jobs = [(kind, hash((kind, i, tuple(sorted(defaults.items())))) & 0xffffffff, trials // chunks, *[defaults[k] for k in order]) for i in range(chunks)]
    return sum(pool.map(trial, jobs)) / (trials // chunks * chunks)

PATHS = sys.argv[1:]
NOISE = residuals(PATHS, lambda v: not v.startswith('pin-') and v not in ('pcores', 'pcore0') and ':' not in v)

if __name__ == "__main__" and not os.environ.get("RG_STAGED"):
    print('residuals', len(NOISE), 'sd', round(st.pstdev(NOISE) * 100, 2), 'pair-diff sd', round(math.sqrt(2) * st.pstdev(NOISE) * 100, 2), flush=True)
    with Pool(16) as pool:
        print('incumbent FP, fixed layout (C5S case):', rate(pool, 'incumbent', 2000, design='fixed'), flush=True)
        print('incumbent power +5%:', rate(pool, 'incumbent', 2000, design='canon', delta=math.log(1.05)), flush=True)
        print('incumbent power +10%:', rate(pool, 'incumbent', 2000, design='canon', delta=math.log(1.10)), flush=True)
        for n in (40, 50, 60, 64, 70, 80):
            for alpha in (1e-3, 5e-3):
                for m0 in (0.005, 0.01):
                    pw = rate(pool, 'new', 4000, n=n, alpha=alpha, m0=m0, delta=math.log(1.05))
                    pws = rate(pool, 'new', 4000, n=n, alpha=alpha, m0=m0, delta=math.log(1.05), sigma_l=0.022)
                    print(f'N={n} alpha={alpha:g} m0={m0:.1%}: power5={pw:.3f} power5(sigma_l 2.2%)={pws:.3f}', flush=True)


def staged(args):
    """Two looks: stage 1 (n1 pairs) certifies if UB < log(budget); else n1 more pairs, regression iff LB(n) > log1p(m0).
    Production arm: p pairs of the unperturbed default build (fixed layouts, phase effect `prod_phase`), flags iff
    LB_prod > log(envelope). Returns (regression flags, certified, inconclusive, prod flags) counts."""
    seed, trials, n1, alpha, m0, budget, envelope, delta, sigma_l, prod_phase, p, scale = args
    rng = random.Random(seed); noise = NOISE
    reg = cert = inc = prod = 0
    for _ in range(trials):
        d1 = pairs(rng, noise, n1, delta, 'canon', sigma_l, 0.0, scale)
        lo, _, hi = hl_bounds(d1, alpha)
        dp = [prod_phase + delta + scale * (rng.choice(noise) - rng.choice(noise)) for _ in range(p)]
        prod_flag = hl_bounds(dp, alpha)[0] > math.log(envelope)
        prod += prod_flag
        if hi < math.log(budget):
            cert += 1; reg += prod_flag; continue
        d = d1 + pairs(rng, noise, n1, delta, 'canon', sigma_l, 0.0, scale)
        lo, _, hi = hl_bounds(d, alpha)
        if lo > math.log1p(m0) or prod_flag: reg += 1
        elif hi < math.log(budget): cert += 1
        else: inc += 1
    return reg, cert, inc, prod


def staged_rate(pool, trials, **kw):
    order = ['n1', 'alpha', 'm0', 'budget', 'envelope', 'delta', 'sigma_l', 'prod_phase', 'p', 'scale']
    cfg = dict(n1=40, alpha=1e-3, m0=0.01, budget=1.05, envelope=1.08, delta=0.0, sigma_l=0.0, prod_phase=0.0, p=20, scale=1.0)
    cfg.update(kw)
    jobs = [(hash((i, tuple(sorted(cfg.items())))) & 0xffffffff, trials // 20, *[cfg[k] for k in order]) for i in range(20)]
    tot = [sum(x) for x in zip(*pool.map(staged, jobs))]
    return [round(t / (trials // 20 * 20), 4) for t in tot]


def main2():
    with Pool(16) as pool:
        print('cols: regression, certified, inconclusive, production-arm flag', flush=True)
        for label, kw in [
            ('A/A fit', {}),
            ('C5S case (prod layout +6.3%, canon 0)', dict(prod_phase=PHASE)),
            ('C5S case, sigma_l 2.2%', dict(prod_phase=PHASE, sigma_l=0.022)),
            ('C5S case, noise x1.5', dict(prod_phase=PHASE, scale=1.5)),
            ('true +1% (margin)', dict(delta=math.log(1.01))),
            ('true +5%', dict(delta=math.log(1.05))),
            ('true +5%, sigma_l 2.2%', dict(delta=math.log(1.05), sigma_l=0.022)),
            ('true +5%, noise x1.5', dict(delta=math.log(1.05), scale=1.5)),
            ('true +5% on a favourable prod layout', dict(delta=math.log(1.05), prod_phase=-PHASE)),
            ('prod-only +15% (layout-specific)', dict(prod_phase=math.log(1.15))),
            ('A/A noise x2.3 (load 6, pin2)', dict(scale=2.3)),
            ('true +5%, noise x2.3', dict(delta=math.log(1.05), scale=2.3)),
        ]:
            print(f'{label:45s}', staged_rate(pool, 20000 if 'A/A fit' in label or 'C5S' in label else 4000, **kw), flush=True)


if __name__ == '__main__' and os.environ.get('RG_STAGED'):
    main2()
