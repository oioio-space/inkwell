"""RF1 plan §4 (review round 1, item 2): recovery after a flood that held every member's inbound places through one relay (the
plan's residual, the worst case). At T = 0 the flood stops: every bucket is empty, every origin's places are held by
stranger connections that expire at a uniform time in [0, 15 s], and every pair's dial loops sit at the 60 s backoff
cap (dial.go, jitter 45..75 s). A dial from a to b needs a token and a place at b; it holds them for h (the
handshake), then the token comes back (RF-5) and the pair is connected. Buckets refill one token per `period`, capped
at the burst. Refused: backoff doubles up to 60 s again."""
import heapq, random

def run(n, P, B, period, h, seed):
    rnd = random.Random(seed)
    tok = [0.0] * n            # tokens, refilled lazily
    last = [0.0] * n
    pend = [0] * n
    stranger = [[rnd.uniform(0, 15) for _ in range(P)] for _ in range(n)]
    done, back, ev, end = set(), {}, [], 0.0
    for a in range(n):
        for b in range(n):
            if a != b:
                back[(a, b)] = 60.0
                heapq.heappush(ev, (rnd.uniform(45, 75), 0, a, b))
    def refill(b, t):
        tok[b] = min(B, tok[b] + (t - last[b]) / period)
        last[b] = t
    while ev and len(done) < n * (n - 1) // 2:
        t, kind, a, b = heapq.heappop(ev)
        key = frozenset((a, b))
        refill(b, t)
        if kind == 0:
            if key in done:
                continue
            held = sum(1 for x in stranger[b] if x > t)
            if pend[b] + held < P and tok[b] >= 1:
                tok[b] -= 1
                pend[b] += 1
                heapq.heappush(ev, (t + h, 1, a, b))
            else:
                d = back[(a, b)]
                heapq.heappush(ev, (t + d - d / 4 + rnd.uniform(0, d / 2), 0, a, b))
        else:
            pend[b] -= 1
            tok[b] = min(B, tok[b] + 1)  # RF-5 refund
            if key not in done:
                done.add(key)
                end = t
    return end

if __name__ == "__main__":
    for period in (5.0, 1.0):
        for h in (1.0, 2.0, 4.0):
            ts = [run(64, 16, 16, period, h, s) for s in range(5)]
            print(f"rate 1/{period:.0f}s h={h}: worst {max(ts):5.1f} s after the flood, median {sorted(ts)[2]:5.1f} s")
    ts = [run(64, 8, 16, 5.0, 2.0, s) for s in range(5)]
    print(f"P=8 rate 1/5s h=2: worst {max(ts):5.1f} s")
