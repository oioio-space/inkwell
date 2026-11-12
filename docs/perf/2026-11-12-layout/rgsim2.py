"""Second model round (Codex review): asymmetric side distributions, alpha spent over two certification looks,
sign-test certification, alpha at the maximum comparison count, Clopper-Pearson intervals on every rate."""
import math, random, sys
from multiprocessing import Pool
import rgsim
from rgsim import NOISE, PHASE, critical, hl_bounds

def sign_upper(d, alpha):
    """Exact one-sided upper bound of the median of d (order statistic; any continuous distribution, independent pairs)."""
    n = len(d); s = sorted(d); acc = 0.0; k = n
    # smallest k with P(Bin(n, 1/2) >= k) <= alpha: then the median <= s[n-k] ... use the binomial tail.
    tail = [0.0] * (n + 2)
    for j in range(n, -1, -1):
        tail[j] = tail[j + 1] + math.comb(n, j) / 2 ** n
    k = next(j for j in range(n + 1) if tail[j] <= alpha)
    return s[k - 1]

def side(rng, n, delta, scale_b, stall_p, stall, sigma_l, layout_b):
    d = []
    for _ in range(n):
        a = rng.gauss(0, sigma_l) + rng.choice(NOISE)
        b = rng.gauss(0, sigma_l * layout_b) + scale_b * rng.choice(NOISE) + delta
        if rng.random() < stall_p: b += stall
        d.append(b - a)
    return d

def run(args):
    seed, trials, cfg = args
    rng = random.Random(seed)
    n1, alpha, cert_mode = cfg['n1'], cfg['alpha'], cfg['cert']
    reg = cert = inc = 0
    lb_ok = math.log1p(0.01); budget = math.log(1.05)
    for _ in range(trials):
        d = side(rng, n1, cfg['delta'], cfg['scale_b'], cfg['stall_p'], cfg['stall'], cfg['sigma_l'], cfg['layout_b'])
        up = (lambda x, a: sign_upper(x, a)) if cert_mode == 'sign' else (lambda x, a: hl_bounds(x, a)[2])
        if up(d, alpha / 2) < budget:
            cert += 1; continue
        d += side(rng, n1, cfg['delta'], cfg['scale_b'], cfg['stall_p'], cfg['stall'], cfg['sigma_l'], cfg['layout_b'])
        lo = hl_bounds(d, alpha)[0]
        if lo > lb_ok: reg += 1
        elif up(d, alpha / 2) < budget: cert += 1
        else: inc += 1
    return reg, cert, inc

def cp(k, n, conf=0.95):
    """Clopper-Pearson interval by bisection on the binomial cdf (no scipy)."""
    def term(j, p):
        if p <= 0: return 1.0 if j == 0 else 0.0
        if p >= 1: return 1.0 if j == n else 0.0
        return math.exp(math.lgamma(n + 1) - math.lgamma(j + 1) - math.lgamma(n - j + 1) + j * math.log(p) + (n - j) * math.log1p(-p))
    def cdf(x, p): return min(1.0, sum(term(j, p) for j in range(x + 1)))
    def solve(f):
        lo, hi = 0.0, 1.0
        for _ in range(60):
            mid = (lo + hi) / 2
            if f(mid): lo = mid
            else: hi = mid
        return lo
    a = (1 - conf) / 2
    low = 0.0 if k == 0 else solve(lambda p: 1 - cdf(k - 1, p) < a)
    high = 1.0 if k == n else solve(lambda p: cdf(k, p) > a)
    return low, high

def rates(pool, trials, **kw):
    cfg = dict(n1=40, alpha=1e-3, cert='sign', delta=0.0, scale_b=1.0, stall_p=0.0, stall=0.0, sigma_l=0.0, layout_b=1.0)
    cfg.update(kw)
    chunks = 20
    res = pool.map(run, [(hash((i, tuple(sorted(cfg.items())))) & 0xffffffff, trials // chunks, cfg) for i in range(chunks)])
    n = trials // chunks * chunks
    out = []
    for k in [sum(r[i] for r in res) for i in range(3)]:
        lo, hi = cp(k, n)
        out.append(f'{k / n:.4f} [{lo:.4f},{hi:.4f}]')
    return '  '.join(out)

if __name__ == '__main__':
    with Pool(16) as pool:
        print('cols: regression, certified, inconclusive (rate [95% Clopper-Pearson])', flush=True)
        for label, trials, kw in [
            ('A/A, signed-rank cert at alpha/2', 20000, dict(cert='hl')),
            ('A/A, sign cert at alpha/2', 20000, {}),
            ('A/A, sign cert, alpha 5e-4 (C=40)', 20000, dict(alpha=5e-4)),
            ('B noisier x1.5, no shift', 20000, dict(scale_b=1.5)),
            ('B layout-sensitive x2 (sigma_l 2.2%)', 20000, dict(sigma_l=0.022, layout_b=2.0)),
            ('B 10% stalls +15% (skewed, median ~0)', 4000, dict(stall_p=0.10, stall=math.log(1.15))),
            ('true +1% (margin)', 20000, dict(delta=math.log(1.01))),
            ('true +1%, B noisier x1.5', 20000, dict(delta=math.log(1.01), scale_b=1.5)),
            ('true +5%, sign cert', 4000, dict(delta=math.log(1.05))),
            ('true +5%, alpha 5e-4 (C=40)', 4000, dict(delta=math.log(1.05), alpha=5e-4)),
            ('true +5%, alpha 5e-4, sigma_l 2.2%', 4000, dict(delta=math.log(1.05), alpha=5e-4, sigma_l=0.022)),
            ('true +5%, B less noisy x0.7', 4000, dict(delta=math.log(1.05), scale_b=0.7)),
            ('true +5% in median, B noisier x1.5', 4000, dict(delta=math.log(1.05), scale_b=1.5)),
        ]:
            print(f'{label:42s}', rates(pool, trials, **kw), flush=True)
