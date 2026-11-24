"""RF1 plan §4 (review round 1, item 3): join time of a wave of members behind one relay, every member dialling every other one.
Discrete events. A dial from a to b arrives at b's admission (origin: the relay); admitted if b holds < P pending
from that origin, then pending for h seconds, then the pair is connected (both loops stop). Refused: the dialler's
loop pauses with inkwell's backoff (dial.go: 1 s doubling to 60 s, jitter d - d/4 + U[0, d/2)), then retries.
Rate: tokens never bind an honest wave (members refund at the handshake and pending <= P <= burst 16)."""
import heapq, random, sys

def run(n, P, h, seed, star=False):
    rnd = random.Random(seed)
    pend = [0] * n
    done = set()
    back = {}
    ev = []
    pairs = [(a, b) for a in range(n) for b in range(n) if a != b and (not star or a == 0 or b == 0)]
    want = {frozenset(p) for p in pairs}
    for a, b in pairs:
        back[(a, b)] = 1.0
        heapq.heappush(ev, (rnd.uniform(0, 0.05), 0, a, b))
    last = 0.0
    while ev and len(done) < len(want):
        t, kind, a, b = heapq.heappop(ev)
        key = frozenset((a, b))
        if kind == 0:  # attempt a -> b
            if key in done:
                continue
            if pend[b] < P:
                pend[b] += 1
                heapq.heappush(ev, (t + h, 1, a, b))
            else:
                d = back[(a, b)]
                back[(a, b)] = min(2 * d, 60.0)
                heapq.heappush(ev, (t + d - d / 4 + rnd.uniform(0, d / 2), 0, a, b))
        else:  # handshake done
            pend[b] -= 1
            if key not in done:
                done.add(key)
                last = t
    return last

if __name__ == "__main__":
    for star in (True, False):
        for P in (8, 16, 32):
            for h in (0.5, 1.0, 2.0, 4.0):
                ts = [run(64, P, h, s, star) for s in range(5)]
                print(f"{'star' if star else 'mesh'} P={P:2d} h={h}: worst {max(ts):6.1f} s  median {sorted(ts)[2]:6.1f} s")
