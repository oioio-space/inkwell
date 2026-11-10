#!/usr/bin/env python3
"""Statistical specification of clause C5 of netsim scenario 5 (NI-1): the loss a person is shown
follows the loss the network really inflicts. Block C5S, docs/plans/2026-11-09-c5-sound.md.

Standard library only. Every claim of the plan that is a number comes from here:

* `Netem` and `netem_hop` reproduce the per-packet semantics of Linux netem (net/sched/sch_netem.c,
  `netem_enqueue` and `loss_gilb_ell`; the gemodel arguments as tc/q_netem.c stores them):
  duplication, Gilbert-Elliott loss, corruption, delay with jitter (the tfifo sorts by send time,
  so jitter reorders) and `reorder` (a packet sent at once, ahead of the delayed ones).
* `loss_window` simulates one 30 s window of one direction across the two netem hops of the netsim
  topology (the sender's NAT egress to the WAN, then the receiver's NAT egress to its LAN), the
  qdisc counters the test reads, and every estimator: the honest one of the plan's decision
  (D2: packets the peer did not receive over packets sent), the alternative D1 (ack-eliciting
  packets declared lost, spurious ones subtracted), the code as it is (`incumbent`), and the
  mutants the rule must reject.
* `QuicLossDetector` is RFC 9002 section 6.1 loss detection as quic-go v0.64.1 implements it
  (internal/ackhandler/sent_packet_handler.go: time threshold (8 + e)/8, packet threshold 3
  disabled after a spurious loss, e doubled up to 8, a 64-entry lost-packet tracker kept for
  3 PTO; received_packet_tracker.go: an ACK every 2 ack-eliciting packets, at once for a
  reordered or a gap-opening packet, else after 25 ms). It counts the spurious declarations that
  reordering causes.
* `RULE` is the acceptance rule of C5; `verdict` applies it to the values a run measured, so the
  end-to-end test and this specification share one implementation of the rule.

Commands: `report` (false-failure rate and power), `calibrate` (the statistics the constants of
`RULE` are read from), `verdict` (JSON on stdin, from netsim.spec.ts), `spread` (the measured
spread of the residual over the gate's runs, against the model's).
"""

from __future__ import annotations

import argparse
import bisect
import heapq
import json
import math
import random
import statistics
import sys
from dataclasses import dataclass, field

# ── What the product does (internal/transport/link.go, p2p/links.go, bridge) ─────────────────

WINDOW = 30.0  # transport.LossWindow
SAMPLE = 1.0  # p2p sampleEvery: the window is a whole number of sampler steps
PUSH = 1.0  # bridge.LinksPushPeriod: the webview holds the last value pushed
MIN_PACKETS = 200  # spec §8.5.5 N, linkprobe.MinPackets (was transport.MinLossPackets = 50)
STALE_WINDOW = 2 * WINDOW  # the « 60 s window » mutant (M5g): a window twice as long


def permille(x: float) -> float:
    """protocol/bridge LinkOf: the loss travels as whole permille, math.Round (half away from 0)."""
    return math.floor(max(x, 0.0) * 1000 + 0.5) / 1000


# ── netem (Linux 7.2 net/sched/sch_netem.c) ──────────────────────────────────────────────────


@dataclass(frozen=True)
class GE:
    """`loss gemodel p r 1-h 1-k`: p good->bad, r bad->good, loss in bad, loss in good."""

    p: float
    r: float
    bad: float
    good: float

    def stationary_bad(self) -> float:
        return self.p / (self.p + self.r)

    def loss(self) -> float:
        b = self.stationary_bad()
        return b * self.bad + (1 - b) * self.good


@dataclass(frozen=True)
class Netem:
    delay: float  # seconds
    jitter: float = 0.0  # standard deviation, `distribution normal`
    ge: GE | None = None
    dup: float = 0.0
    corrupt: float = 0.0
    reorder: float = 0.0  # probability a packet is sent at once (tc sets gap 1)

    def hop_loss(self) -> float:
        """Probability one packet does not reach the next hop intact (drop, or corruption)."""
        drop = self.ge.loss() * (1 - self.dup) if self.ge else 0.0
        return drop + (1 - drop) * self.corrupt

    def path_loss(self) -> float:
        return 1 - (1 - self.hop_loss()) ** 2


WIFI_GE = GE(0.01, 0.30, 0.50, 0.001)
PROFILES = {
    # scripts/netsim/topology.sh profile(), on the four NAT interfaces.
    "wifi": Netem(0.015, 0.005, WIFI_GE, 0.005, 0.001),
    "wifi-far": Netem(0.075, 0.010, WIFI_GE, 0.005, 0.001),
    "wifi-clean": Netem(0.015, 0.005),
    # Proposed P4 (plan section 4): no loss, a minority of packets held 60 ms on each hop.
    "wifi-reorder": Netem(0.060, 0.0, reorder=0.80),
}
PHASES = {"P1": "wifi", "P2": "wifi-far", "P3": "wifi-clean", "P4": "wifi-reorder"}


def _delay(prof: Netem, rnd, gauss) -> float:
    if prof.reorder and rnd() < prof.reorder:
        return 0.0
    if not prof.jitter:
        return prof.delay
    z = gauss(0.0, 1.0)
    z = 4.0 if z > 4.0 else -4.0 if z < -4.0 else z  # netem's normal table is bounded
    d = prof.delay + prof.jitter * z
    return d if d > 0.0 else 0.0  # a time_to_send in the past is sent at once


def netem_hop(packets, prof: Netem, rng: random.Random, lo: float, hi: float, log: list | None = None):
    """One netem qdisc. `packets` is [(t, tag)] sorted by t; tag >= 0 is one of ours, -1 is
    another flow. Returns (out, arrivals, dropped): out is [(t_out, tag, corrupted)] for every copy
    of ours that leaves; arrivals and dropped are the qdisc counters (sent + dropped, dropped) over
    packets enqueued in [lo, hi]. A duplicate is re-enqueued at the root at once (tc_depth 1: it
    is not duplicated again but draws its own loss, corruption and delay)."""
    rnd, gauss = rng.random, rng.gauss
    ge = prof.ge
    bad = ge is not None and rnd() < ge.stationary_bad()
    dupp, corr = prof.dup, prof.corrupt
    out = []
    arrivals = dropped = 0
    for t, tag in packets:
        inwin = lo <= t <= hi
        depth = 0
        while True:
            dup = depth == 0 and dupp > 0.0 and rnd() < dupp
            lost = False
            if ge is not None:
                if not bad:  # GOOD_STATE: transition draw, then the good-state loss draw
                    if rnd() < ge.p:
                        bad = True
                    if rnd() < ge.good:
                        lost = True
                else:  # BAD_STATE: transition draw, then the bad-state loss draw
                    if rnd() < ge.r:
                        bad = False
                    if rnd() < ge.bad:
                        lost = True
            if inwin:
                arrivals += 1
            count = 1 + dup - lost  # a duplicated packet that is "lost" still leaves once
            if log is not None:
                log.append((t, count == 0))  # the qdisc counters of any interval (trace)
            if count == 0:
                if inwin:
                    dropped += 1
                break
            corrupted = corr > 0.0 and rnd() < corr  # the clone was taken before: it is intact
            if tag >= 0:
                out.append((t + _delay(prof, rnd, gauss), tag, corrupted))
            else:
                _delay(prof, rnd, gauss)
            if count == 2:
                depth = 1
                continue
            break
    return out, arrivals, dropped


def _poisson_times(rng: random.Random, rate: float, t0: float, t1: float):
    out, t = [], t0
    if rate <= 0:
        return out
    while True:
        t += rng.expovariate(rate)
        if t > t1:
            return out
        out.append(t)


def _types(rng: random.Random, n: int, f: float, run_ae: float):
    """Ack-eliciting flags of n packets: alternating runs, mean run_ae of ack-eliciting packets
    (a flight) and run_ae (1 - f)/f of ACK-only ones, so a fraction f is ack-eliciting."""
    if f >= 1.0:
        return [True] * n
    run_na = run_ae * (1 - f) / f
    out, ae = [], rng.random() < f
    while len(out) < n:
        mean = run_ae if ae else run_na
        k = 1 + int(rng.expovariate(1 / max(mean - 1, 1e-9))) if mean > 1 else 1
        out.extend([ae] * k)
        ae = not ae
    return out[:n]


# ── One window of one direction ──────────────────────────────────────────────────────────────


@dataclass
class Traffic:
    rate: float  # our packets per second on the measured connection, this direction
    foreign: float = 0.05  # share of each qdisc's packets that belong to other flows (relay, ...)
    ae: float = 0.5  # share of our packets that are ack-eliciting (D1, detector)
    flight: float = 2.0  # mean run of consecutive ack-eliciting packets
    stale: float = SAMPLE + PUSH  # the value read is up to this old
    spread: float = SAMPLE  # the window covers WINDOW +- spread
    # The crossings of spec §8.5.5 « Exactitude », modelled without any priority (review 3, F1: the
    # stack cannot give it): every packet of ours that leaves between the read of sent(C) and the
    # PROBE's packet crosses it (retransmissions first, ACK-only packets when the congestion window
    # is full, other streams in turn), the PROBE waiting up to `stack` seconds in the sender.
    stack: float = 0.005  # a paced, uncongested sender: the next packet within 5 ms
    # The responder reads received(C) after the read call that returned the PROBE: up to
    # `read_delay` later (scheduling), and quic-go counts the PROBE's own packet only after handing
    # its data to the reader (connection.go handleFrames, then PacketsReceived++): counted or not,
    # each with probability 1/2 (review 3, F3).
    read_delay: float = 0.020
    race: bool = True
    # Head-of-line blocking on the probe stream (review 3, F10): a PROBE whose own packet is lost is
    # resent after the time threshold (9/8 RTT, RFC 9002 §6.1.2), and one that follows a lost PROBE
    # waits for that one's retransmission. Either may still come back within one PTO.
    hol: bool = True
    min_span: float = WINDOW / 2  # spec §8.5.5 step 5: the two samples at least W/2 apart
    phase_rates: tuple = ()  # ((phase, rate), ...): phases the test drives at another rate (trace)

    def rate_of(self, phase: str) -> float:
        return dict(self.phase_rates).get(phase, self.rate)


@dataclass
class Window:
    phase: str
    n: int  # qdisc packets of the first hop over the qdisc window (what the test reads)
    truth: float  # T*: path loss from the qdisc counters, plus the profile's corruption
    honest: float  # D2, permille-rounded
    d1: float  # D1, permille-rounded (nan if no ack-eliciting packet)
    incumbent: float  # the code as it is: ack-eliciting lost (+ spurious) over all sent
    sent: int  # our packets in the window
    lost: int  # of them, not received intact
    spurious: int = 0  # spurious declarations in the window (with detect=True)
    hops: list = field(default_factory=list)  # [(sent, dropped)] as qstats shows them
    late: int = 0  # probes skipped as late at the two boundaries
    span: float = WINDOW  # seconds between the window's two samples
    crossing: int = 0  # net packets that crossed a boundary PROBE: received - (sent - lost)
    rtt: float = math.nan  # the round trip of the newest sample


def _arrivals(ours, prof: Netem, tr: Traffic, rng: random.Random, t0: float, t1: float, logs=None):
    """Our packets (send times) across the two netem hops of a direction: their arrival times
    (inf when lost or corrupted), and each hop's qdisc events when logs is a pair of lists."""
    frate = tr.rate * tr.foreign / (1 - tr.foreign)
    hop1_in = sorted([(t, i) for i, t in enumerate(ours)] + [(t, -1) for t in _poisson_times(rng, frate, t0, t1)])
    out1, a1, d1 = netem_hop(hop1_in, prof, rng, 0.0, WINDOW, logs[0] if logs else None)
    out1.sort()
    hop2_in = sorted([(t, k) for k, (t, _, _) in enumerate(out1)] + [(t, -1) for t in _poisson_times(rng, frate, t0, t1 + 0.2)])
    out2, a2, d2 = netem_hop(hop2_in, prof, rng, 0.0, WINDOW, logs[1] if logs else None)
    arrival = [math.inf] * len(ours)
    for t2, k, bad2 in out2:
        _, tag, bad1 = out1[k]
        if not (bad1 or bad2) and t2 < arrival[tag]:
            arrival[tag] = t2
    return arrival, (a1, d1), (a2, d2)


def _figure(ours, got, arrival, prof, tr, rng, start, end):
    """The prober's figure over a window [start, end] (spec §8.5.5 steps 3 to 5) against the loss
    of the same window: (figure or nan, sent, lost, crossing, late, span, rtt of the newest)."""
    start, read_a, late_a, c_a, _ = _probe(prof, rng, start, tr, SAMPLE)
    end, read_b, late_b, c_b, rtt = _probe(prof, rng, end, tr, -SAMPLE)
    lo_i, hi_i = bisect.bisect_right(ours, start), bisect.bisect_right(ours, end)
    sent = hi_i - lo_i
    received = bisect.bisect_right(got, read_b) - bisect.bisect_right(got, read_a) + c_b - c_a
    lost = sum(1 for i in range(lo_i, hi_i) if arrival[i] == math.inf)
    known = sent >= MIN_PACKETS and end - start >= tr.min_span
    fig = permille((sent - min(received, sent)) / sent) if known else math.nan
    return fig, sent, lost, received - (sent - lost), late_a + late_b, end - start, rtt


def loss_window(phase: str, tr: Traffic, rng: random.Random, detect: bool = False, adapted: bool = True) -> Window:
    """Simulates the window that ends when the test reads the link, at t = WINDOW. The qdisc
    window is [0, WINDOW] (qstats read at both ends); the product's window ends `stale` earlier
    (sampler step plus push) and spans WINDOW +- spread."""
    prof = PROFILES[PHASES[phase]]
    end = WINDOW - rng.uniform(0, tr.stale)
    start = end - rng.uniform(WINDOW - tr.spread, WINDOW + tr.spread)
    t0 = min(start, 0.0) - 1.0
    ours = _poisson_times(rng, tr.rate, t0, WINDOW + 1.0)
    arrival, (a1, d1), (a2, d2) = _arrivals(ours, prof, tr, rng, t0, WINDOW + 1.0)
    # Truth as the test computes it from qstats, plus the expected corruption it cannot see.
    q1, q2 = d1 / a1 if a1 else 0.0, d2 / a2 if a2 else 0.0
    truth = 1 - (1 - q1) * (1 - q2) * (1 - prof.corrupt) ** 2
    got = sorted(a for a in arrival if a < math.inf)
    honest, sent, lost, crossing, late, span, rtt = _figure(ours, got, arrival, prof, tr, rng, start, end)
    lo_i, hi_i = bisect.bisect_right(ours, start), bisect.bisect_right(ours, end)
    types = _types(rng, len(ours), tr.ae, tr.flight)
    ae_sent = sum(1 for i in range(lo_i, hi_i) if types[i])
    ae_lost = sum(1 for i in range(lo_i, hi_i) if types[i] and arrival[i] == math.inf)
    w = Window(
        phase=phase, n=a1, truth=truth, honest=honest,
        d1=permille(ae_lost / ae_sent) if ae_sent else math.nan,
        incumbent=permille(ae_lost / sent) if sent else math.nan,
        sent=sent, lost=lost, hops=[(a1 - d1, d1), (a2 - d2, d2)],
        late=late, span=span, crossing=crossing, rtt=rtt,
    )
    if detect:
        det = QuicLossDetector(prof, rng, adapted=adapted)
        k = det.run(ours, arrival, types, start, end)
        w.spurious = k
        w.incumbent = permille((ae_lost + k) / sent) if sent else math.nan
    return w


def pto(prof: Netem) -> float:
    """RFC 9002 §6.2.1 PTO of a path of four hops of prof: srtt + 4 rttvar + max_ack_delay, with
    srtt the mean round trip and rttvar its mean deviation (0.8 sd for a normal-ish sum)."""
    m = (1 - prof.reorder) * prof.delay
    v = (1 - prof.reorder) * (prof.jitter**2 + prof.delay**2) - m * m
    return 4 * m + 4 * 0.8 * math.sqrt(4 * v) + 0.025


def _probe(prof: Netem, rng: random.Random, t: float, tr: Traffic, step: float):
    """(instant sent(C) is read, instant received(C) is read, late probes skipped, PROBE's own packet
    counted at the read (0 or 1), round trip) of the nearest probe from t, moving by step (+1 s for
    the oldest sample of the window, −1 s for the newest: the product takes the newest accepted
    sample, never a later one), whose echo comes back within one PTO (spec §8.5.5 step 3)."""
    late = 0
    while True:
        res = _attempt(prof, rng, tr)
        if res is not None:
            read, counted, rtt = res
            return t, t + read, late, counted, rtt
        late += 1
        t += step


# ── quic-go v0.64.1 loss detection, to count spurious declarations ───────────────────────────


class QuicLossDetector:
    """The sender's loss detection of one direction and the receiver's ACK policy. Packets are
    our packets of the window (send time, arrival time at the peer, ack-eliciting); the ACKs
    travel back across two hops of the same profile. Only declarations are modelled: no packet is
    resent, since a retransmission is a new packet the traffic already contains."""

    MAX_ACK_DELAY = 0.025
    GRANULARITY = 0.001
    TRACKED = 64

    def __init__(self, prof: Netem, rng: random.Random, adapted: bool = True):
        self.prof, self.rng = prof, rng
        # adapted: the connection already saw reordering (P1-P3), so packet threshold is off and
        # the time threshold is at its maximum, 16/8: the fewest spurious losses (worst case for
        # the power against the "spurious counted" mutant).
        self.e = 8 if adapted else 1
        self.pkt_threshold = not adapted
        self.times: list = []  # (instant, packet) of each spurious declaration

    def run(self, sends, arrivals, ae, lo: float, hi: float) -> int:
        """Spurious declarations made in (lo, hi]: ack-eliciting packets declared lost that did
        reach the peer (arrival < inf)."""
        rnd, gauss, prof = self.rng.random, self.rng.gauss, self.prof
        ev: list = []  # (time, seq, kind, payload)
        seq = 0

        def push(t, kind, data=None):
            nonlocal seq
            seq += 1
            heapq.heappush(ev, (t, seq, kind, data))

        for pn, t in enumerate(arrivals):
            if t < math.inf:
                push(t, "arr", pn)
        # receiver
        largest_obs, largest_t, since_ack, last_ack, alarm = -1, 0.0, 0, None, None
        # sender
        srtt = rttvar = latest = 0.0
        has_rtt = False
        min_rtt = math.inf
        largest_acked, largest_acked_send = -1, -math.inf
        outstanding: list = []  # pns sent and neither acked nor declared lost, ascending
        next_pn = 0
        tracker: list = []  # (pn, send time) declared lost
        min_pn = None
        loss_timer = None
        acked = [False] * len(sends)
        declared = 0

        def take_sent(now):
            nonlocal next_pn, min_pn
            while next_pn < len(sends) and sends[next_pn] <= now:
                if min_pn is None:
                    min_pn = next_pn
                outstanding.append(next_pn)
                next_pn += 1

        def loss_delay():
            return max(max(latest, srtt) * (8 + self.e) / 8, self.GRANULARITY)

        def detect_lost(now):
            nonlocal loss_timer, declared
            cutoff, delay = now - loss_delay(), loss_delay()
            when, keep = None, []
            for pn in outstanding:
                if pn > largest_acked:
                    keep.append(pn)
                    continue
                if sends[pn] <= cutoff + 1e-9 or (self.pkt_threshold and largest_acked - pn >= 3):
                    if ae[pn]:
                        if lo < now <= hi and arrivals[pn] < math.inf:
                            declared += 1  # declared lost, but it reached the peer
                            self.times.append((now, pn))
                        tracker.append((pn, sends[pn]))
                        if len(tracker) > self.TRACKED:
                            tracker.pop(0)
                    continue
                if when is None:
                    when = sends[pn] + delay
                keep.append(pn)
            outstanding[:] = keep
            if when is not None and when != loss_timer:
                push(when, "timer", when)
            loss_timer = when

        def send_ack(tau):
            nonlocal since_ack, last_ack, alarm
            last_ack, since_ack, alarm = (largest_obs, tau), 0, None
            back = _delay(prof, rnd, gauss) + _delay(prof, rnd, gauss)
            push(tau + back, "ack", (tau, largest_obs, tau - largest_t))

        while ev:
            t, _, kind, data = heapq.heappop(ev)
            if t > hi + 5.0:
                break
            if kind == "arr":
                pn = data
                prev = largest_obs
                if pn > largest_obs:
                    largest_obs, largest_t = pn, t
                if not ae[pn]:
                    continue
                since_ack += 1
                missing = last_ack is not None and pn < last_ack[0]
                gap = pn > prev + 1 and arrivals[pn - 1] > t and (last_ack is None or pn - 1 >= last_ack[0])
                if missing or since_ack >= 2 or gap:
                    send_ack(t)
                elif alarm is None:
                    alarm = t + self.MAX_ACK_DELAY
                    push(alarm, "alarm", alarm)
            elif kind == "alarm":
                if alarm is not None and data == alarm:
                    send_ack(t)
            elif kind == "timer":
                if loss_timer is not None and data == loss_timer:
                    loss_timer = None
                    take_sent(t)
                    detect_lost(t)
            else:  # an ACK reaches the sender
                tau, largest, delay_field = data
                take_sent(t)
                newly = [pn for pn in outstanding if arrivals[pn] <= tau]
                ack_delay = min(delay_field, self.MAX_ACK_DELAY)
                if newly and largest in newly and any(ae[pn] for pn in newly) and sends[largest] >= largest_acked_send:
                    sample = t - sends[largest]
                    latest = sample
                    min_rtt = min(min_rtt, sample)
                    if sample - min_rtt >= ack_delay:
                        sample -= ack_delay
                    if not has_rtt:
                        srtt, rttvar, has_rtt = sample, sample / 2, True
                    else:
                        rttvar = 0.75 * rttvar + 0.25 * abs(srtt - sample)
                        srtt = 0.875 * srtt + 0.125 * sample
                    largest_acked_send = sends[largest]
                # spurious losses: declared lost, now acknowledged
                max_tr, keep, raised = 0.0, [], False
                for pn, st in tracker:
                    if arrivals[pn] <= tau:
                        self.pkt_threshold = False
                        if min_pn is not None and pn >= min_pn:
                            max_tr = max(max_tr, (t - ack_delay) - st)
                    else:
                        keep.append((pn, st))
                if max_tr > loss_delay() and self.e < 8:
                    self.e, min_pn, raised = min(2 * self.e, 8), None, True
                pto = srtt + max(4 * rttvar, self.GRANULARITY)
                tracker[:] = [(pn, st) for pn, st in keep if st >= t - 3 * pto]
                for pn in newly:
                    acked[pn] = True
                if newly:
                    outstanding[:] = [pn for pn in outstanding if not acked[pn]]
                largest_acked = max(largest_acked, largest)
                if newly or raised:
                    detect_lost(t)
        return declared


# ── The quality level (spec §8.5.5 step 6, Annex A.1 `quality`) ──────────────────────────────

UNKNOWN, GOOD, FAIR, POOR = 0, 1, 2, 3  # Annex A.1 quality codes
NAMES = {UNKNOWN: "unknown", GOOD: "good", FAIR: "fair", POOR: "poor"}


@dataclass(frozen=True)
class Band:
    """One boundary between two levels: a lower level rises at `up` or above, an upper level falls
    back below `down`, and the first known figure after « unknown » is placed against `mid`."""

    down: float
    mid: float
    up: float


@dataclass(frozen=True)
class Levels:
    loss: tuple  # (good|fair, fair|poor), loss as a fraction
    rtt: tuple  # (good|fair, fair|poor), round trip in seconds


# ADR 0036, « Levels »: the boundaries 1 % and 10 % (sources there), the bands derived by `flicker`
# (a steady link at a boundary changes level 1.75 times an hour at 60 packets/s, over 4 h).
LOSS_BANDS = (Band(0.005, 0.010, 0.025), Band(0.050, 0.100, 0.150))
RTT_BANDS = (Band(0.400, 0.500, 0.625), Band(1.600, 2.000, 2.500))  # seconds
LEVELS = Levels(loss=LOSS_BANDS, rtt=RTT_BANDS)
# Candidates `flicker` compares: (good|fair band, fair|poor band) as (down, up) multiples of 1 % and 10 %.
FLICKER_CANDIDATES = {
    "none": ((1, 1), (1, 1)), "x1.25": ((1.25, 1.25), (1.25, 1.25)), "x1.5": ((1.5, 1.5), (1.5, 1.5)),
    "x2": ((2, 2), (2, 2)), "chosen": ((2, 2.5), (2, 1.5)), "x3": ((3, 3), (3, 3)),
}


def step(prev: int, x: float, bands) -> int:
    """The level after `prev` for a value x (spec §8.5.5 step 6)."""
    b1, b2 = bands
    if prev == UNKNOWN:
        return POOR if x >= b2.mid else FAIR if x >= b1.mid else GOOD
    if prev != POOR and x >= b2.up:
        return POOR
    if prev == GOOD and x >= b1.up:
        return FAIR
    if prev != GOOD and x < b1.down:
        return GOOD
    if prev == POOR and x < b2.down:
        return FAIR
    return prev


class Classifier:
    """The prober's level of one connection (spec §8.5.5 step 6): the worse of the loss level and
    the round-trip level, each with its own hysteresis, advanced at every new figure. While the
    figure is unknown the level shown is unknown; the hysteresis survives an unknown spell of at
    most W, and is forgotten after a longer one (review 4, F4)."""

    def __init__(self, levels: Levels = LEVELS):
        self.levels, self.loss, self.rtt, self.known_at = levels, UNKNOWN, UNKNOWN, None

    def update(self, figure: float, rtt: float = math.nan, now: float = 0.0) -> int:
        if figure is None or math.isnan(figure):
            return UNKNOWN
        if self.known_at is not None and now - self.known_at > WINDOW:
            self.loss = self.rtt = UNKNOWN
        self.known_at = now
        self.loss = step(self.loss, figure, self.levels.loss)
        if not math.isnan(rtt):
            self.rtt = step(self.rtt, rtt, self.levels.rtt)
        return max(self.loss, self.rtt)


def no_hysteresis(levels: Levels) -> Levels:
    flat = lambda bs: tuple(Band(b.mid, b.mid, b.mid) for b in bs)  # noqa: E731
    return Levels(loss=flat(levels.loss), rtt=flat(levels.rtt))


# ── A whole run of scenario 5, second by second ──────────────────────────────────────────────

# P0 (proposed, plan section 4): independent loss of 1.75 % on the path, between the good|fair
# midpoint (1 %) and its rise threshold (2.5 %): the level the prober enters from « unknown » with
# depends on the midpoint (review 4: the entry is judged).
ENTRY_HOP = 1 - math.sqrt(1 - 0.0175)
PROFILES["wifi-entry"] = Netem(0.015, 0.005, GE(ENTRY_HOP, 1 - ENTRY_HOP, 1.0, 0.0))
PHASES["P0"] = "wifi-entry"
# P5 (proposed): wifi with its bursts made rarer, so that the path loss sits at the good|fair
# boundary: the level may be either, but must follow the hysteresis.
EDGE_GE = GE(0.0018, 0.30, 0.50, 0.001)
PROFILES["wifi-edge"] = Netem(0.015, 0.005, EDGE_GE, 0.005, 0.001)
PHASES["P5"] = "wifi-edge"
PLAN = (("P0", 40.0), ("P1", 40.0), ("P2", 40.0), ("P3", 40.0), ("P4", 40.0), ("P5", 120.0))
DESIGN_RATE = 120.0  # packets/s per direction the test produces (plan section 5)
# The test types twice as fast during P0, so that the first figure (a 15 s window) falls between the
# midpoint and the rise threshold in nearly every run: the entry is then judged.
DESIGN_PHASE_RATES = (("P0", 240.0),)


def design_traffic(rate: float = DESIGN_RATE, foreign: float = 0.10) -> "Traffic":
    return Traffic(rate=rate, foreign=foreign, phase_rates=tuple((p, r * rate / DESIGN_RATE) for p, r in DESIGN_PHASE_RATES))


def _attempt(prof: Netem, rng: random.Random, tr: Traffic):
    """One PROBE: None if lost or late, else (read delay after the hand-over, own packet counted,
    round trip). sent(C) is read when the PROBE is handed to the stream; its packet leaves up to
    `stack` later (no priority: every packet of ours sent meanwhile crosses it). A lost PROBE packet
    is resent after the time threshold, 9/8 of the round trip; a PROBE after a lost one waits for
    that one's retransmission (head-of-line blocking). Either may still be accepted, within one PTO."""
    rnd, gauss = rng.random, rng.gauss
    loss, rtt0 = prof.path_loss(), 4 * (1 - prof.reorder) * prof.delay
    wait = rng.uniform(0, tr.stack) if tr.stack else 0.0
    out = _delay(prof, rnd, gauss) + _delay(prof, rnd, gauss)
    if rnd() < loss:
        if not tr.hol or rnd() < loss:
            return None
        out = 9 / 8 * max(rtt0, 0.001) + _delay(prof, rnd, gauss) + _delay(prof, rnd, gauss)
    deliver = wait + out
    if tr.hol and rnd() < loss:
        prev = -0.5 + rng.uniform(0, tr.stack) + 9 / 8 * max(rtt0, 0.001)
        deliver = max(deliver, prev + _delay(prof, rnd, gauss) + _delay(prof, rnd, gauss))
    read = deliver + (rng.uniform(0, tr.read_delay) if tr.read_delay else 0.0)
    back = _delay(prof, rnd, gauss) + _delay(prof, rnd, gauss)
    if read + back > pto(prof):
        return None
    return read, ((rnd() < 0.5) if tr.race else 1), read + back


# The figures a trace computes at each sampler step: the prober's, and those of three mutants.
FIGURES = ("honest", "old", "m5b", "window_60s")


@dataclass
class Trace:
    """One direction of one run: per sampler step, what the prober computes and what the test
    measures."""

    steps: list  # (t, {figure name: figure or nan}, rtt low median, label, n, lowest T*, highest T*)
    counters: list = field(default_factory=list)  # (whole second, [(sent, dropped)] per hop, cumulative)


READ_BACK = 3  # a step's window ends 0 to 3 s before it (the newest accepted sample, a late PROBE)


def window_range(truth, r_t: float):
    """(n, lowest T*, highest T*) over the windows the figure of a step at r_t may cover: ends from
    r_t − 3 to r_t by whole seconds, spans W − 2 to W (the prober's oldest sample is within W of its
    newest); the test reads the qdisc counters every second. `truth(lo, hi)` returns (first-hop
    packets, T*) of [lo, hi]."""
    n, lo_t, hi_t = math.inf, math.inf, -math.inf
    for back in range(READ_BACK + 1):
        for span in (WINDOW - 2, WINDOW - 1, WINDOW):
            k, t = truth(r_t - back - span, r_t - back)
            n, lo_t, hi_t = min(n, k), min(lo_t, t), max(hi_t, t)
    return n, lo_t, hi_t


def trace(tr: Traffic, rng: random.Random, plan=PLAN, detect: bool = True, adapted: bool = False) -> Trace:
    """Simulates the phases of `plan` back to back on one direction of a new connection: our packets
    and a foreign flow across the two netem hops (each phase's profile on the packets enqueued
    during it), a PROBE per second (Interval), and at each sampler step (SAMPLE) the prober's figure
    and the round trip of its window, the test's truth range for that step, and the figures of the
    mutants (the old figure from quic-go's declarations, M5b's since the connection started, a 60 s
    window)."""
    total = sum(d for _, d in plan)
    edges, t = [], 0.0
    for name, d in plan:
        edges.append((t, t + d, name))
        t += d
    t0 = 0.0
    ours = []
    for k, (a, b, name) in enumerate(edges):
        ours += _poisson_times(rng, tr.rate_of(name), a, total + 1.0 if k == len(edges) - 1 else b)
    logs = ([], [])
    arrival = [math.inf] * len(ours)
    out1 = []
    for k, (a, b, name) in enumerate(edges):
        prof = PROFILES[PHASES[name]]
        lo, hi = a, (total + 1.0 if k == len(edges) - 1 else b)
        mine = [(x, i) for i, x in enumerate(ours) if lo <= x < hi]
        frate = tr.rate_of(name) * tr.foreign / (1 - tr.foreign)
        hop1 = sorted(mine + [(x, -1) for x in _poisson_times(rng, frate, lo, hi)])
        o, _, _ = netem_hop(hop1, prof, rng, a, b, logs[0])
        out1 += [(x, tag, bad, name) for x, tag, bad in o]
    out1.sort()
    by_phase: dict = {}
    for k, (x, tag, bad, name) in enumerate(out1):
        by_phase.setdefault(name, []).append((x, k))
    for k, (a, b, name) in enumerate(edges):
        prof = PROFILES[PHASES[name]]
        lo, hi = a, (total + 2.0 if k == len(edges) - 1 else b)
        frate = tr.rate_of(name) * tr.foreign / (1 - tr.foreign)
        hop2 = sorted(by_phase.get(name, []) + [(x, -1) for x in _poisson_times(rng, frate, lo, hi)])
        o, _, _ = netem_hop(hop2, prof, rng, a, b, logs[1])
        for x2, j, bad2 in o:
            _, tag, bad1, _ = out1[j]
            if not (bad1 or bad2) and x2 < arrival[tag]:
                arrival[tag] = x2
    got = sorted(x for x in arrival if x < math.inf)

    def phase_at(x):
        for a, b, name in edges:
            if a <= x < b:
                return name
        return edges[-1][2]

    # The PROBEs, one per Interval: (hand-over, received(C) read at, counted, round trip) or None.
    probes = []
    for k in range(int(total) + 1):
        res = _attempt(PROFILES[PHASES[phase_at(float(k))]], rng, tr)
        probes.append(None if res is None else (float(k), k + res[0], res[1], res[2]))
    acc = [p for p in probes if p is not None]
    acc_t = [p[0] for p in acc]
    # The incumbent (« the old figure »): ack-eliciting packets declared lost, spurious ones kept.
    types = _types(rng, len(ours), tr.ae, tr.flight)
    ae_lost_prefix = [0]
    for i, x in enumerate(arrival):
        ae_lost_prefix.append(ae_lost_prefix[-1] + (types[i] and x == math.inf))
    spur = []
    if detect:
        for a, b, name in edges:
            lo_i, hi_i = bisect.bisect_left(ours, a - 2.0), bisect.bisect_left(ours, b)
            det = QuicLossDetector(PROFILES[PHASES[name]], rng, adapted=adapted)
            det.run(ours[lo_i:hi_i], arrival[lo_i:hi_i], types[lo_i:hi_i], a, b)
            spur += [x for x, _ in det.times]
        spur.sort()

    def figure(i0, j):
        """The figure from sample i0 to sample j (spec §8.5.5 step 5), and the old one."""
        old_s, new = acc[i0], acc[j]
        lo_i, hi_i = bisect.bisect_right(ours, old_s[0]), bisect.bisect_right(ours, new[0])
        sent = hi_i - lo_i
        if i0 >= j or new[0] - old_s[0] < tr.min_span or sent < MIN_PACKETS:
            return math.nan, math.nan
        rec = bisect.bisect_right(got, new[1]) - bisect.bisect_right(got, old_s[1]) + new[2] - old_s[2]
        k_sp = bisect.bisect_right(spur, new[0]) - bisect.bisect_right(spur, old_s[0])
        old = permille((ae_lost_prefix[hi_i] - ae_lost_prefix[lo_i] + k_sp) / sent)
        return permille((sent - min(rec, sent)) / sent), old

    # qdisc counters by time, per hop: prefix sums over each hop's log.
    pref = []
    for log in logs:
        log.sort()
        ts, dr = [x for x, _ in log], [0]
        for _, d in log:
            dr.append(dr[-1] + d)
        pref.append((ts, dr))

    def truth(lo, hi, names):
        keep, n = 1.0, 0
        for h, (ts, dr) in enumerate(pref):
            i, k = bisect.bisect_left(ts, lo), bisect.bisect_right(ts, hi)
            arr, drop = k - i, dr[k] - dr[i]
            keep *= 1 - (drop / arr if arr else 0.0)
            if h == 0:
                n = arr
        return n, 1 - keep * (1 - max(PROFILES[PHASES[x]].corrupt for x in names)) ** 2

    steps = []
    s_t = 0.0
    while s_t <= total:
        figs = dict.fromkeys(FIGURES, math.nan)
        rtt = math.nan
        j = bisect.bisect_right(acc_t, s_t) - 1  # the newest accepted sample
        if j >= 0 and s_t - acc[j][0] <= WINDOW:
            i0 = bisect.bisect_left(acc_t, acc[j][0] - WINDOW)  # the oldest within the window
            figs["honest"], figs["old"] = figure(i0, j)
            if not math.isnan(figs["honest"]):
                rs = sorted(acc[i][3] for i in range(i0, j + 1))
                rtt = rs[(len(rs) - 1) // 2]  # the lower median (§8.5.5 step 6)
            figs["m5b"] = figure(0, j)[0]
            figs["window_60s"] = figure(bisect.bisect_left(acc_t, acc[j][0] - STALE_WINDOW), j)[0]
        label, n, lo_t, hi_t = phase_at(s_t), 0, math.nan, math.nan
        if s_t >= WINDOW + READ_BACK:
            names = sorted({phase_at(x) for x in (s_t - WINDOW - READ_BACK, s_t - 1e-9)})
            label = ">".join(x for _, _, x in edges if x in names)
            n, lo_t, hi_t = window_range(lambda lo, hi: truth(lo, hi, names), s_t)
        steps.append((s_t, figs, rtt, label, n, lo_t, hi_t))
        s_t += SAMPLE
    counters = []
    for sec in range(math.floor(t0), math.ceil(total) + 1):
        hops = []
        for ts, dr in pref:
            k = bisect.bisect_right(ts, sec)
            hops.append((k - dr[k], dr[k]))
        counters.append((sec, hops))
    return Trace(steps, counters)


def levels_of(tr_: Trace, levels: Levels = LEVELS, figure: str = "honest", rtt: bool = True) -> list:
    """The level the prober shows at each sampler step, from one of the trace's figures."""
    c, out = Classifier(levels), []
    for t, figs, r, *_ in tr_.steps:
        out.append(c.update(figs[figure], r if rtt else math.nan, t))
    return out


def sticky_levels(tr_: Trace) -> list:
    """A level that rises but never falls back once known (review 4)."""
    out, cur = [], UNKNOWN
    for lv in levels_of(tr_):
        cur = lv if lv == UNKNOWN or lv > cur or cur == UNKNOWN else cur
        out.append(cur)
    return out


def scaled(levels: Levels, f: float, entry_up: bool = False, rtt_f: float = 1.0) -> Levels:
    """The bands scaled by f (thresholds ×2, ×0.5), the entry against `up` instead of `mid`, or the
    round-trip bands scaled by rtt_f."""
    loss = tuple(Band(b.down * f, (b.up if entry_up else b.mid) * f, b.up * f) for b in levels.loss)
    rtt = tuple(Band(b.down * rtt_f, b.mid * rtt_f, b.up * rtt_f) for b in levels.rtt)
    return Levels(loss=loss, rtt=rtt)


# ── The C5 rule: a classification test (derived by `report`, checked by c5model_test.py) ───────


@dataclass(frozen=True)
class Rule:
    n_min: int  # qdisc packets of the first hop in every window judged against the truth
    q: float  # accuracy tolerance: q * sqrt(T* / n) + c / n around the range of T*
    crossings: tuple  # ((phase, c), ...): the crossing term, in packets, per profile
    t_min: float  # P0, P1, P2: mean T* below this means the impairment did not happen (anti-vacuity)

    def c(self, label: str) -> float:
        return max(dict(self.crossings)[p] for p in label.split(">"))


RULE = Rule(n_min=3000, q=1.5,
            crossings=(("P0", 22), ("P1", 22), ("P2", 32), ("P3", 24), ("P4", 52), ("P5", 22)), t_min=0.01)


def tolerance(rule: Rule, label: str, lo_t: float, hi_t: float, n: int) -> tuple[float, float]:
    """The figures an honest prober may compute for a step whose windows have qdisc truth in
    [lo_t, hi_t]: the range widened by the foreign packets of the qdisc and the crossings."""
    tol = rule.q * math.sqrt(hi_t / n) + rule.c(label) / n
    return lo_t - tol, hi_t + tol


def judge_run(rule: Rule, rows) -> tuple[bool, list]:
    """rows: [(label, dir, t, level, figure, n, lo T*, hi T*)] for every sampler step, in time order
    (the test logs each step's `quality` and `lossPermille`). R1: the level is the level of spec
    §8.5.5 step 6 applied to the figures shown (the round-trip level good: every profile of
    scenario 5 is far below 400 ms). R2: once a window lies in the run, the figure is within the
    tolerance of the qdisc truth, on at least n_min packets. R3: P0, P1 and P2 were impaired."""
    why, tsum, judges = [], {}, {}
    for label, d, t, level, fig, n, lo_t, hi_t in rows:
        c = judges.setdefault(d, Classifier())
        want = c.update(fig, math.nan, t)
        if level != want:
            why.append(f"{label} {d} t={t:g}: {NAMES[level]} shown, the figure {fig:.3f} gives {NAMES[want]}")
        if math.isnan(lo_t):
            continue
        if n < rule.n_min:
            why.append(f"{label} {d} t={t:g}: {n} packets < n_min {rule.n_min}")
            continue
        if math.isnan(fig):
            why.append(f"{label} {d} t={t:g}: no figure")
            continue
        lo, hi = tolerance(rule, label, lo_t, hi_t, n)
        if not lo <= fig <= hi:
            why.append(f"{label} {d} t={t:g}: figure {fig:.3f} outside [{lo:.4f}, {hi:.4f}] (T* {lo_t:.4f}..{hi_t:.4f})")
        tsum.setdefault((label, d), []).append((lo_t + hi_t) / 2)
    for phase in ("P0", "P1", "P2"):
        for d in judges:
            ts = tsum.get((phase, d))
            if not ts:
                why.append(f"{phase} {d}: no judged step")
            elif statistics.fmean(ts) < rule.t_min:
                why.append(f"{phase} {d}: mean T* {statistics.fmean(ts):.4f} < t_min {rule.t_min}: not impaired")
    return not why, why


MUTANTS = ("always_good", "always_poor", "always_fair", "swapped", "no_hysteresis", "sticky", "entry_rise",
           "thresholds_x2", "rtt_tenth", "old_figure", "m5b", "window_60s")
# Measured but not required at 0.999: halved thresholds differ from the rule only between 0.25 % and 0.5 %
# on a fall, or from 1.25 % to 2.5 % on P5's rises (power 0.9974 over 3000 runs).
EXTRA_MUTANTS = ("thresholds_half",)
ESTIMATORS = ("honest", *MUTANTS, *EXTRA_MUTANTS)


def _rows(tr_: Trace, shown: list, d: str, figure: str = "honest") -> list:
    return [(label, d, t, shown[k], figs[figure], n, lo_t, hi_t)
            for k, (t, figs, _, label, n, lo_t, hi_t) in enumerate(tr_.steps)]


def mutant_rows(t_: Trace, d: str) -> dict:
    """{estimator: rows} of one direction: the honest prober and each mutant."""
    honest = levels_of(t_)
    known = [x != UNKNOWN for x in honest]
    return {
        "honest": _rows(t_, honest, d),
        "always_good": _rows(t_, [GOOD if k else UNKNOWN for k in known], d),
        "always_poor": _rows(t_, [POOR if k else UNKNOWN for k in known], d),
        "always_fair": _rows(t_, [FAIR if k else UNKNOWN for k in known], d),
        "swapped": _rows(t_, [4 - x if x else x for x in honest], d),
        "no_hysteresis": _rows(t_, levels_of(t_, no_hysteresis(LEVELS)), d),
        "sticky": _rows(t_, sticky_levels(t_), d),
        "entry_rise": _rows(t_, levels_of(t_, scaled(LEVELS, 1.0, entry_up=True)), d),
        "thresholds_x2": _rows(t_, levels_of(t_, scaled(LEVELS, 2.0)), d),
        "thresholds_half": _rows(t_, levels_of(t_, scaled(LEVELS, 0.5)), d),
        "rtt_tenth": _rows(t_, levels_of(t_, scaled(LEVELS, 1.0, rtt_f=0.1)), d),
        "old_figure": _rows(t_, levels_of(t_, figure="old"), d, "old"),
        "m5b": _rows(t_, levels_of(t_, figure="m5b"), d, "m5b"),
        "window_60s": _rows(t_, levels_of(t_, figure="window_60s"), d, "window_60s"),
    }


def simulate_run(tr: Traffic, rng: random.Random) -> dict:
    """One run: both directions; {estimator: rows} for the honest prober and each mutant."""
    rows = {k: [] for k in ESTIMATORS}
    for d in ("AB", "BA"):
        # adapted: quic-go's loss detection already saw reordering, the fewest spurious declarations
        # (the worst case for the power against the old figure).
        for k, v in mutant_rows(trace(tr, rng, adapted=True), d).items():
            rows[k] += v
    return rows


def clopper_pearson_upper(failures: int, runs: int, conf: float = 0.95) -> float:
    """One-sided upper bound of a binomial proportion (exact, by bisection on the CDF)."""
    if failures >= runs:
        return 1.0
    alpha = 1 - conf

    def cdf(p):  # P(X <= failures)
        return sum(math.comb(runs, k) * p**k * (1 - p) ** (runs - k) for k in range(failures + 1))

    lo, hi = failures / runs, 1.0
    for _ in range(60):
        mid = (lo + hi) / 2
        if cdf(mid) > alpha:
            lo = mid
        else:
            hi = mid
    return hi


@dataclass
class Assessment:
    runs: int
    failures: dict  # estimator -> runs the rule rejects
    reasons: dict = field(default_factory=dict)  # estimator -> a few reasons

    def false_failure_upper(self) -> float:
        return clopper_pearson_upper(self.failures["honest"], self.runs)

    def power_lower(self, mutant: str) -> float:
        return 1 - clopper_pearson_upper(self.runs - self.failures[mutant], self.runs)


def _assess_chunk(args):
    rule, tr, runs, seed = args
    rng = random.Random(seed)
    fails = dict.fromkeys(ESTIMATORS, 0)
    reasons: dict = {}
    for _ in range(runs):
        for k, r in simulate_run(tr, rng).items():
            ok, why = judge_run(rule, r)
            if not ok:
                fails[k] += 1
                reasons.setdefault(k, why[:2])
    return fails, reasons


def assess(rule: Rule, tr: Traffic, runs: int, seed: int, workers: int = 1) -> Assessment:
    """`runs` runs in `workers` processes, each chunk with its own seed (seed, seed + 1, ...)."""
    chunks = max(1, workers)
    per = [runs // chunks + (i < runs % chunks) for i in range(chunks)]
    jobs = [(rule, tr, n, seed + i) for i, n in enumerate(per) if n]
    if workers > 1:
        import multiprocessing

        with multiprocessing.Pool(workers) as pool:
            parts = pool.map(_assess_chunk, jobs)
    else:
        parts = [_assess_chunk(j) for j in jobs]
    fails = {k: sum(p[0][k] for p in parts) for k in ESTIMATORS}
    reasons: dict = {}
    for _, r in parts:
        for k, v in r.items():
            reasons.setdefault(k, v)
    return Assessment(runs, fails, reasons)


# ── The figure's error (spec §8.5.5 « Exactitude »): many seeds, intervals ──────────────────────


def quantile_upper(xs, q: float, conf: float = 0.95) -> float:
    """Distribution-free upper confidence bound of the q-quantile: the order statistic of rank
    ceil(n q + z sqrt(n q (1 - q))) + 1 (normal approximation of the binomial rank)."""
    xs = sorted(xs)
    n = len(xs)
    z = statistics.NormalDist().inv_cdf(conf)
    r = math.ceil(n * q + z * math.sqrt(n * q * (1 - q))) + 1
    return xs[r - 1] if r <= n else math.inf


ERROR_SCENARIOS = {
    # name: traffic -- docs/adr/0036-link-probe.md, « Levels »
    "idle, 4/s": Traffic(rate=4),
    "typing, 17/s": Traffic(rate=17),
    "design, 60/s": Traffic(rate=60),
    "saturated, 300/s, waits <= 300 ms": Traffic(rate=300, stack=0.3),
    "60/s, read 50 ms late": Traffic(rate=60, read_delay=0.05),
}


def errors(tr: Traffic, phase: str, windows: int, seeds) -> dict:
    """The figure against the loss of its own window, over `windows` windows of each seed: the net
    crossing in packets and the error as a fraction, with 95 % upper bounds of their 99.9th
    percentile over all seeds, and the per-seed 99.9th percentiles."""
    cross, err, per_seed, unknown, total = [], [], [], 0, 0
    for seed in seeds:
        rng = random.Random(seed)
        mine = []
        for _ in range(windows):
            w = loss_window(phase, tr, rng)
            total += 1
            if w.sent < MIN_PACKETS or math.isnan(w.honest):
                unknown += 1
                continue
            cross.append(abs(w.crossing))
            e = w.honest - w.lost / w.sent
            err.append(abs(e))
            mine.append(abs(e))
        if mine:
            per_seed.append(sorted(mine)[min(len(mine) - 1, int(0.999 * len(mine)))])
    return {
        "windows": total, "unknown": unknown / total if total else math.nan,
        "crossing_p999_upper95": quantile_upper(cross, 0.999) if cross else math.nan,
        "crossing_max": max(cross) if cross else math.nan,
        "error_p999_upper95": quantile_upper(err, 0.999) if err else math.nan,
        "error_p999_per_seed": per_seed,
    }


def steady_changes(loss: float, rate: float, hours: float, levels: Levels, seed: int) -> float:
    """Level changes per hour on a steady wifi-like link whose path loss is `loss` (the bursts of
    WIFI_GE, made rarer or more frequent), at `rate` packets/s, classified every second."""
    ge = GE(WIFI_GE.r * x / (1 - x), WIFI_GE.r, WIFI_GE.bad, WIFI_GE.good) if (x := _bad_share(loss)) > 0 else None
    PROFILES["steady"] = Netem(0.015, 0.005, ge, 0.0, 0.0)
    PHASES["PS"] = "steady"
    rng = random.Random(seed)
    t_ = trace(Traffic(rate=rate), rng, plan=(("PS", hours * 3600.0),), detect=False)
    lv = [x for x in levels_of(t_, levels) if x != UNKNOWN]
    return sum(1 for a, b in zip(lv, lv[1:]) if a != b) / hours


def _bad_share(loss: float) -> float:
    """Stationary bad-state share of WIFI_GE's two states giving a path loss of `loss` (two hops)."""
    hop = 1 - math.sqrt(1 - loss)
    return max(0.0, (hop - WIFI_GE.good) / (WIFI_GE.bad - WIFI_GE.good))


def old_rule_fails(rows) -> bool:
    """The C5 of before: P1, P2 in [1 %, 10 %], P3 <= 0.5 %, on the two last samples."""
    for phase, loss in rows:
        if phase in ("P1", "P2") and not 0.01 <= loss <= 0.10:
            return True
        if phase == "P3" and loss > 0.005:
            return True
    return False


# ── Commands ─────────────────────────────────────────────────────────────────────────────────


def cmd_report(args):
    tr = design_traffic(args.rate, args.foreign)
    a = assess(RULE, tr, args.runs, args.seed, args.workers)
    print(json.dumps({"rule": RULE.__dict__, "levels": {"loss": [b.__dict__ for b in LEVELS.loss],
                                                        "rtt": [b.__dict__ for b in LEVELS.rtt]},
                      "traffic": tr.__dict__, "runs": a.runs, "rejected": a.failures,
                      "false_failure_upper95": a.false_failure_upper(),
                      "power_lower95": {m: a.power_lower(m) for m in (*MUTANTS, *EXTRA_MUTANTS)},
                      "reasons": a.reasons}, indent=1))


def _errors_job(job):
    name, phase, windows, seeds = job
    return name, phase, errors(ERROR_SCENARIOS[name], phase, windows, seeds)


def cmd_errors(args):
    seeds = list(range(args.seed, args.seed + args.seeds))
    jobs = [(name, p, args.windows, seeds) for name in ERROR_SCENARIOS for p in ("P1", "P2", "P4")]
    import multiprocessing

    with multiprocessing.Pool(args.workers) as pool:
        res = pool.map(_errors_job, jobs)
    out: dict = {}
    for name, p, r in res:
        out.setdefault(name, {})[p] = r
    print(json.dumps(out, indent=1))


def cmd_flicker(args):
    """Level changes per hour of a steady link whose path loss sits exactly at a boundary, for each
    candidate band, at the typing and the design traffic."""
    out = {}
    for name, ((d1, u1), (d2, u2)) in FLICKER_CANDIDATES.items():
        lv = Levels(loss=(Band(0.01 / d1, 0.01, 0.01 * u1), Band(0.10 / d2, 0.10, 0.10 * u2)), rtt=LEVELS.rtt)
        out[name] = {f"{loss:.3f}@{rate:g}/s": steady_changes(loss, rate, args.hours, lv, args.seed)
                     for loss in args.losses for rate in args.rates}
    print(json.dumps(out, indent=1))


def cmd_profiles(args):
    """Where each phase's windows fall: T* quantiles, the level shown, and the figure's error."""
    rng = random.Random(args.seed)
    acc: dict = {}
    for _ in range(args.runs):
        t_ = trace(design_traffic(args.rate), rng, detect=False)
        shown = levels_of(t_)
        for k, (t, figs, _, label, n, lo_t, hi_t) in enumerate(t_.steps):
            if ">" in label or math.isnan(lo_t):
                continue  # a window across two phases, or before the first full window
            a = acc.setdefault(label, {"truth": [], "n": [], "shown": {}})
            a["truth"].append((lo_t + hi_t) / 2)
            a["n"].append(n)
            a["shown"][NAMES[shown[k]]] = a["shown"].get(NAMES[shown[k]], 0) + 1
    out = {}
    for phase, a in acc.items():
        ts = sorted(a["truth"])
        out[phase] = {"nominal_path_loss": PROFILES[PHASES[phase]].path_loss(), "steps": len(ts),
                      "truth_q001": ts[int(0.001 * len(ts))], "truth_median": ts[len(ts) // 2],
                      "truth_q999": ts[min(len(ts) - 1, int(0.999 * len(ts)))], "n_min": min(a["n"]),
                      "shown": a["shown"]}
    print(json.dumps(out, indent=1))


def cmd_verdict(_args):
    """Reads {"counters": {dir: [{"t", "hops": [{"sent", "dropped"}, ...]}]}, "steps": [{"t", "phase",
    "dir", "quality", "lossPermille"}]}: the cumulative qdisc counters of each direction's two hops,
    in path order, every whole second, and every sampler step the prober logged (its level and its
    figure, 65535 = unknown), in time order; `phase` is the phase the step's window lies in, or
    "A>B" across two."""
    data = json.load(sys.stdin)
    snaps = {d: {round(c["t"]): [(h["sent"], h["dropped"]) for h in c["hops"]] for c in cs}
             for d, cs in data["counters"].items()}
    rows = []
    for r in data["steps"]:
        d, label, r_t = r["dir"], r["phase"], float(r["t"])
        fig = math.nan if r["lossPermille"] == 0xFFFF else r["lossPermille"] / 1000
        n, lo_t, hi_t = 0, math.nan, math.nan
        if r_t >= WINDOW + READ_BACK:

            def truth(lo, hi, d=d, label=label):
                a, b = snaps[d].get(round(lo)), snaps[d].get(round(hi))
                if a is None or b is None:
                    return 0, 0.0
                return truth_of(label, [(sb - sa, db - da) for (sa, da), (sb, db) in zip(a, b)])

            n, lo_t, hi_t = window_range(truth, r_t)
        rows.append((label, d, r_t, int(r["quality"]), fig, n, lo_t, hi_t))
    ok, why = judge_run(RULE, rows)
    seen = {p for p, *_ in rows}
    want = {p for p, _ in PLAN}
    if not want <= seen:
        ok = False
        why.append(f"phases missing: {sorted(want - seen)}")
    print(json.dumps({"ok": ok, "why": why[:20]}))
    return 0 if ok else 1


def truth_of(phase: str, hops) -> tuple[int, float]:
    """(n, T*) from the qstats deltas of the direction's two hops [(sent, dropped), ...]: the path
    loss, plus the profile's corruption that the counters cannot see."""
    corrupt = max(PROFILES[PHASES[p]].corrupt for p in phase.split(">"))
    keep = 1.0
    for s, d in hops:
        keep *= 1 - (d / (s + d) if s + d else 0.0)
    s1, d1 = hops[0]
    return s1 + d1, 1 - keep * (1 - corrupt) ** 2


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    r = sub.add_parser("report")
    r.add_argument("--runs", type=int, default=3000)
    r.add_argument("--rate", type=float, default=DESIGN_RATE)
    r.add_argument("--foreign", type=float, default=0.10)
    r.add_argument("--seed", type=int, default=11)
    r.add_argument("--workers", type=int, default=4)
    e = sub.add_parser("errors")
    e.add_argument("--windows", type=int, default=2000)
    e.add_argument("--seeds", type=int, default=10)
    e.add_argument("--seed", type=int, default=13)
    e.add_argument("--workers", type=int, default=4)
    f = sub.add_parser("flicker")
    f.add_argument("--losses", type=float, nargs="+", default=[0.01, 0.10])
    f.add_argument("--rates", type=float, nargs="+", default=[17.0, 60.0])
    f.add_argument("--hours", type=float, default=2.0)
    f.add_argument("--seed", type=int, default=17)
    p = sub.add_parser("profiles")
    p.add_argument("--runs", type=int, default=200)
    p.add_argument("--rate", type=float, default=DESIGN_RATE)
    p.add_argument("--seed", type=int, default=19)
    sub.add_parser("verdict")
    args = ap.parse_args(argv)
    cmds = {"report": cmd_report, "errors": cmd_errors, "flicker": cmd_flicker, "profiles": cmd_profiles,
            "verdict": cmd_verdict}
    return cmds[args.cmd](args) or 0


if __name__ == "__main__":
    sys.exit(main())
