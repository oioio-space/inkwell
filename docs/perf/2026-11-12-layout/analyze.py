"""Variance components of the layout experiment: per variant median, between-layout spread."""
import json, math, statistics as st, sys, collections
rows = [json.loads(l) for l in open(sys.argv[1])]
by = collections.defaultdict(list)
for r in rows: by[r['variant']].append(math.log(r['ns']))
def show(v): x = by[v]; return f"{v:14s} n={len(x):2d} median={math.exp(st.median(x)):.2f} sd%={st.stdev(x)*100:.2f}"
for v in ['base-def-k0', 'c5s-def-k0', 'c5s-def-k1', 'base-fa-k0', 'c5s-fa-k0']: print(show(v))
def delta(a, b): return (math.exp(st.median(by[b]) - st.median(by[a])) - 1) * 100
print('c5s-def vs base-def %+.2f%%' % delta('base-def-k0', 'c5s-def-k0'))
print('c5s-def-k1 (phase flip) vs c5s-def-k0 %+.2f%%' % delta('c5s-def-k0', 'c5s-def-k1'))
print('c5s-fa vs base-fa %+.2f%%' % delta('base-fa-k0', 'c5s-fa-k0'))
print('fa vs def (c5s) %+.2f%%' % delta('c5s-def-k0', 'c5s-fa-k0'))
for fam in ['c5s-fa-r', 'c5s-def-r', 'base-fa-r']:
    groups = {v: x for v, x in by.items() if v.startswith(fam)}
    means = [st.mean(x) for x in groups.values()]
    within = st.mean([st.variance(x) for x in groups.values()])
    r = st.mean(len(x) for x in groups.values())
    between = max(st.variance(means) - within / r, 0)
    print(f"{fam}: K={len(groups)} layout means %: {sorted(round((math.exp(m - st.mean(means)) - 1) * 100, 1) for m in means)}")
    print(f"   within-process sd%={math.sqrt(within)*100:.2f} between-layout sd%={math.sqrt(between)*100:.2f} grand={math.exp(st.mean(means)):.2f}")
