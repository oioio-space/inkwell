"""Tests of scripts/netsim/c5model.py, the statistical specification of NI-1 clause C5
(docs/plans/2026-11-09-c5-sound.md) and of the quality levels of spec §8.5.5 step 6 (ADR 0036,
« Levels »). Each test names the rule it proves.

The rule tests are simulations with fixed seeds, so they are deterministic. A bound on a
probability is the exact one-sided 95 % Clopper-Pearson bound: 0 failures in 3000 runs bounds
the false-failure rate by 1e-3, and 3000 rejections out of 3000 bound the power from below by
0.999. Run: ./scripts/heavy.sh timeout 600 python3 -m unittest scripts/netsim/c5model_test.py
"""

import dataclasses
import io
import json
import math
import os
import random
import re
import statistics
import sys
import unittest
from contextlib import redirect_stdout

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import c5model as m  # noqa: E402

SPIKE_SEED = 158  # a run of Transition.PLAN with one step outside the tolerance without the lag term
RUNS = 3000  # the fewest runs for which 0 / RUNS bounds a rate by 1e-3 (95 %)
DESIGN_RATE = 60.0  # packets/s of the accuracy findings (the C5 rule runs at m.DESIGN_RATE, plan section 5)
LIMITS_GO = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "protocol", "limits.go")


class Netem(unittest.TestCase):
    """Plan section 3.1: the model is netem's own per-packet semantics."""

    def test_gemodel_stationary_loss(self):
        """Gilbert-Elliott drops at pi_B * (1-h) + pi_G * (1-k), times (1 - dup) (sch_netem.c:
        a duplicated packet that loses its loss draw still leaves once)."""
        prof = m.PROFILES["wifi"]
        n = 300_000
        _, arrivals, dropped = m.netem_hop([(i * 1e-3, -1) for i in range(n)], prof, random.Random(1), 0, n)
        originals = n
        rate = dropped / originals
        want = prof.ge.loss() * (1 - prof.dup)
        # Losses cluster in bad-state bursts: the variance is about 3 times the binomial one.
        sd = math.sqrt(3.1 * want / originals)
        self.assertLess(abs(rate - want), 4 * sd, (rate, want))
        self.assertAlmostEqual(prof.ge.loss(), 0.01 / 0.31 * 0.5 + 0.30 / 0.31 * 0.001, places=12)
        self.assertGreater(arrivals, originals)  # duplicates go through the qdisc again

    def test_loss_draw_uses_state_before_transition(self):
        """loss_gilb_ell: the transition draw comes first, but the loss probability is that of the
        state the packet found. With p = 1, good loss 0, bad loss 1, packets alternate."""
        prof = m.Netem(0.0, ge=m.GE(1.0, 1.0, 1.0, 0.0))
        rng = random.Random(2)
        rng.random = lambda it=iter([0.9] + [0.5] * 100): next(it)  # start GOOD, then any draw < 1
        out, _, dropped = m.netem_hop([(i, i) for i in range(6)], prof, rng, 0, 10)
        self.assertEqual(sorted(tag for _, tag, _ in out), [0, 2, 4])
        self.assertEqual(dropped, 3)

    def test_jitter_reorders_and_reorder_sends_at_once(self):
        """tfifo sorts by time_to_send, so a normal jitter reorders; `reorder 80%` sends 80 % of
        packets at once (tc sets gap 1)."""
        rng = random.Random(3)
        sent = [(i * 0.002, i) for i in range(5000)]
        out, _, _ = m.netem_hop(sent, m.PROFILES["wifi-clean"], rng, 0, 100)
        order = [tag for _, tag, _ in sorted(out)]
        self.assertGreater(sum(a > b for a, b in zip(order, order[1:])), 100)
        out, _, _ = m.netem_hop(sent, m.PROFILES["wifi-reorder-deep"], rng, 0, 100)
        at_once = sum(1 for (t, tag, _) in out if t == sent[tag][0])
        self.assertAlmostEqual(at_once / len(out), 0.80, delta=0.03)


class Detector(unittest.TestCase):
    """Plan section 2: quic-go v0.64.1 declares losses on reordering (RFC 9002 section 6.1)."""

    def setUp(self):
        self.prof = m.Netem(0.010)  # fixed 10 ms per hop: no reordering of ACKs

    def run_detector(self, arrivals, adapted):
        sends = [i * 0.005 for i in range(len(arrivals))]
        det = m.QuicLossDetector(self.prof, random.Random(4), adapted=adapted)
        return det.run(sends, arrivals, [True] * len(sends), -1, 100)

    def test_in_order_path_declares_nothing(self):
        sends = [i * 0.005 for i in range(400)]
        self.assertEqual(self.run_detector([t + 0.02 for t in sends], adapted=False), 0)

    def test_one_late_packet_is_a_spurious_declaration(self):
        """A packet 200 ms late behind faster ones exceeds 9/8 RTT: declared lost, then ACKed."""
        sends = [i * 0.005 for i in range(400)]
        arrivals = [t + 0.02 for t in sends]
        arrivals[200] += 0.2
        self.assertEqual(self.run_detector(arrivals, adapted=False), 1)

    def test_adaptation_lowers_but_does_not_end_spurious_losses(self):
        """After spurious losses quic-go waits up to 2 RTT; the first P4 (a fifth held 60 ms) still
        exceeds it."""
        m.PHASES["P4D"] = "wifi-reorder-deep"
        tr = m.Traffic(rate=DESIGN_RATE)
        rng = random.Random(5)
        ws = [m.loss_window("P4D", tr, rng, detect=True) for _ in range(20)]
        rates = [w.spurious / w.sent for w in ws]
        self.assertGreater(min(rates), 0.03, rates)  # above the 2.5 % that turns good into fair


class Levels(unittest.TestCase):
    """Spec §8.5.5 step 6: the level, its boundaries and its hysteresis."""

    def test_bands_are_the_protocol_constants(self):
        """One set of numbers: the model's bands are protocol/limits.go's (permille, durations)."""
        with open(LIMITS_GO, encoding="utf-8") as f:
            src = f.read()

        def const(name):
            v = re.search(rf"^\s*{name}\s*=\s*(\d+)(?:\s*\*\s*time\.(Millisecond|Second))?", src, re.M)
            self.assertIsNotNone(v, name)
            n, unit = int(v.group(1)), v.group(2)
            return n / 1000 if unit in (None, "Millisecond") else float(n)

        for b, kind in zip(m.LEVELS.loss, ("Fair", "Poor")):
            self.assertEqual((b.down, b.mid, b.up), tuple(const(f"LinkProbeLoss{kind}{s}") for s in ("Down", "", "Up")))
        for b, kind in zip(m.LEVELS.rtt, ("Fair", "Poor")):
            self.assertEqual((b.down, b.mid, b.up), tuple(const(f"LinkProbeRTT{kind}{s}") for s in ("Down", "", "Up")))

    def test_step_boundaries(self):
        """The same table as protocol/linkprobe TestStepBoundaries, in fractions."""
        b = m.LEVELS.loss
        cases = [(m.UNKNOWN, 0.009, m.GOOD), (m.UNKNOWN, 0.010, m.FAIR), (m.UNKNOWN, 0.100, m.POOR),
                 (m.GOOD, 0.024, m.GOOD), (m.GOOD, 0.025, m.FAIR), (m.GOOD, 0.150, m.POOR),
                 (m.FAIR, 0.005, m.FAIR), (m.FAIR, 0.004, m.GOOD), (m.FAIR, 0.150, m.POOR),
                 (m.POOR, 0.050, m.POOR), (m.POOR, 0.049, m.FAIR), (m.POOR, 0.004, m.GOOD)]
        for prev, x, want in cases:
            self.assertEqual(m.step(prev, x, b), want, (prev, x))

    def test_hysteresis_keeps_a_boundary_link_steady(self):
        """ADR 0036, « Levels »: a steady link whose loss sits exactly at 1 % changes level about
        twice an hour at 60 packets/s with the chosen bands (4 h: 1.75/h), hundreds of times
        without hysteresis."""
        chosen = m.steady_changes(0.01, DESIGN_RATE, 1.0, m.LEVELS, 17)
        flat = m.steady_changes(0.01, DESIGN_RATE, 1.0, m.no_hysteresis(m.LEVELS), 17)
        self.assertLessEqual(chosen, 6.0)
        self.assertGreater(flat, 100.0)

    def test_classifier_keeps_short_unknown_spells(self):
        """Review 4, F4: the hysteresis survives an unknown spell of at most W, not a longer one."""
        c = m.Classifier()
        self.assertEqual(c.update(0.02, now=0), m.FAIR)  # from unknown: against the midpoint
        self.assertEqual(c.update(0.006, now=1), m.FAIR)  # inside the band: kept
        self.assertEqual(c.update(math.nan, now=2), m.UNKNOWN)
        self.assertEqual(c.update(0.006, now=31), m.FAIR)  # 30 s after the last known figure: kept
        self.assertEqual(c.update(math.nan, now=32), m.UNKNOWN)
        self.assertEqual(c.update(0.006, now=62), m.GOOD)  # 31 s: forgotten, the midpoint again
        self.assertEqual(c.update(0.0, rtt=0.7, now=63), m.FAIR)  # the worse of loss and round trip


class Rule(unittest.TestCase):
    """Plan section 4: the classification rule and its application to measured values."""

    def test_level_must_follow_the_figures(self):
        """R1 (review 4): the level of every step is the one spec §8.5.5 step 6 gives for the figures
        shown, so a level that never falls back is caught at the first figure under 0.5 %."""
        rows = [("P3", "AB", t, lv, fig, 0, math.nan, math.nan)
                for t, lv, fig in ((15, m.FAIR, 0.02), (16, m.FAIR, 0.006), (17, m.FAIR, 0.004))]
        ok, why = m.judge_run(m.RULE, rows)
        self.assertFalse(ok)
        self.assertIn("gives good", why[0])
        rows[-1] = rows[-1][:3] + (m.GOOD,) + rows[-1][4:]
        _, why = m.judge_run(m.RULE, rows)
        self.assertFalse([w for w in why if "gives" in w], why)  # R1 holds; R3 still asks for P0 to P2

    def test_truth_from_qstats(self):
        n, t = m.truth_of("P1", [(980, 20), (970, 10)])
        self.assertEqual(n, 1000)
        self.assertAlmostEqual(t, 1 - 0.98 * (1 - 10 / 980) * 0.999**2, places=12)
        self.assertEqual(m.truth_of("P3", [(1000, 0), (1000, 0)]), (1000, 0.0))

    def test_verdict_command(self):
        """netsim.spec.ts pipes its measures to `c5model.py verdict`: one rule, one place. Here a
        run of the model goes through the JSON path, then one step is turned wrong."""
        data = {"counters": {}, "steps": []}
        for i, d in enumerate(("AB", "BA")):
            tr_ = m.trace(m.design_traffic(), random.Random(i), detect=False)
            shown = m.levels_of(tr_)
            data["counters"][d] = [{"t": sec, "hops": [{"sent": a, "dropped": b} for a, b in hops]}
                                   for sec, hops in tr_.counters]
            data["steps"] += [{"t": t, "phase": label, "dir": d, "quality": shown[k],
                               "lossPermille": 0xFFFF if math.isnan(figs["honest"]) else round(figs["honest"] * 1000)}
                              for k, (t, figs, _, label, *_) in enumerate(tr_.steps)]
        self.assertEqual(self.verdict(data), (0, True))
        bad = json.loads(json.dumps(data))
        next(r for r in bad["steps"] if r["phase"] == "P3")["quality"] = m.POOR
        self.assertEqual(self.verdict(bad), (1, False))
        short = {**data, "steps": [r for r in data["steps"] if r["phase"] != "P4"]}
        self.assertEqual(self.verdict(short), (1, False))

    def verdict(self, data):
        stdin, out = sys.stdin, io.StringIO()
        sys.stdin = io.StringIO(json.dumps(data))
        try:
            with redirect_stdout(out):
                code = m.main(["verdict"])
        finally:
            sys.stdin = stdin
        return code, json.loads(out.getvalue())["ok"]


class Transition(unittest.TestCase):
    """Spec §8.5.5 step 3.6 and « Exactitude » (run 5-s1-r8): a sample at a change of the round trip."""

    PLAN = (("P1", 40.0), ("P2", 40.0), ("P3", 40.0))  # the profiles of the run, its drop at t = 80

    def test_pto_climbs_after_the_drop(self):
        """RFC 9002 §5.3, as quic-go updates it: after P2 (300 ms) falls to P3 (60 ms) rttvar takes a
        quarter of the gap at once, so the PTO of C, steady at about 390 ms, reaches 0.6 to 0.8 s for
        a few hundred milliseconds, then settles near P3's 117 ms; `pto_peak` bounds it from above."""
        a, b = m.PROFILES[m.PHASES["P2"]], m.PROFILES[m.PHASES["P3"]]
        steady, peak = m.pto_peak(a, b)
        self.assertAlmostEqual(steady, m.pto_peak(b, b)[1], places=9)  # no change: no lag
        self.assertEqual(m.transition_lag(b, b), 0.0)
        highest = []
        for seed in range(6):
            t_ = m.trace(m.design_traffic(), random.Random(seed), plan=self.PLAN, detect=False)
            r = t_.path.rtt
            before = r.pto(79.9)
            after = [r.pto(80.0 + k / 100) for k in range(100)]
            self.assertLess(abs(before - m.pto_peak(a, a)[0]), 0.05, before)
            highest.append(max(after))
            self.assertLess(r.pto(83.0), 0.16)
        self.assertGreater(min(highest), 0.6, highest)
        self.assertLessEqual(max(highest), peak, (highest, peak))

    def test_single_step_spike_at_the_p2_p3_drop(self):
        """Run 5-s1-r8: one step of P2>P3, its oldest sample a PROBE of the end of P2 resent (or held
        behind a resent one) across the drop and accepted within the inflated PTO, reads a figure above
        the tolerance without the lag term, while its neighbours stay inside; with the lag of
        `transition_lag` (the crossing term of a sample at a change) it is inside."""
        rule, old = m.RULE, dataclasses.replace(m.RULE, lags=())
        t_ = m.trace(m.design_traffic(), random.Random(SPIKE_SEED), plan=self.PLAN, detect=False)
        bad = []
        for t, figs, _, label, n, lo_t, hi_t in t_.steps:
            if math.isnan(lo_t) or math.isnan(figs["honest"]):
                continue
            lo, hi = m.tolerance(old, label, lo_t, hi_t, n)
            if not lo <= figs["honest"] <= hi:
                bad.append((t, figs, label, n, lo_t, hi_t))
        self.assertEqual(len(bad), 1, [b[0] for b in bad])  # red with the old tolerance: one step
        t, figs, label, n, lo_t, hi_t = bad[0]
        self.assertEqual(label, "P2>P3")
        s0 = figs["s0"]  # (hand-over, r read at, counted, round trip, transmissions, held)
        self.assertGreater(figs["honest"], m.tolerance(old, label, lo_t, hi_t, n)[1])  # too much loss
        self.assertTrue(79.0 < s0[0] < 80.0, s0)  # handed over at the end of P2
        self.assertTrue(s0[4] > 1 or s0[5], s0)  # resent, or held behind a resent PROBE
        self.assertGreater(s0[3], m.pto_peak(m.PROFILES["wifi-far"], m.PROFILES["wifi-far"])[0], s0)
        lo, hi = m.tolerance(rule, label, lo_t, hi_t, n)
        self.assertTrue(lo <= figs["honest"] <= hi, (figs["honest"], lo, hi))  # green with the lag


class Statistics(unittest.TestCase):
    """Plan section 4: false failures <= 1e-3 per run, power >= 0.999 against every mutant."""

    @classmethod
    def setUpClass(cls):
        cls.design = m.assess(m.RULE, m.design_traffic(), RUNS, seed=101, workers=4)

    def test_false_failure_rate(self):
        self.assertLessEqual(self.design.false_failure_upper(), 1e-3, (self.design.failures, self.design.reasons))

    def test_power_against_each_mutant(self):
        """Review 4: always fair, sticky, the entry against `up`, thresholds ×2, M5b and a 60 s window
        among them."""
        for mutant in m.MUTANTS:
            with self.subTest(mutant=mutant):
                self.assertGreaterEqual(self.design.power_lower(mutant), 0.999, self.design.failures)
        for mutant in m.EXTRA_MUTANTS:
            with self.subTest(mutant=mutant):
                self.assertGreaterEqual(self.design.power_lower(mutant), 0.99, self.design.failures)


class Findings(unittest.TestCase):
    """Plan sections 1 and 2, ADR 0036: what the model explains and what the redesign rests on."""

    def test_old_rule_is_flaky_on_the_code_of_today(self):
        """Today's traffic (about 17 packets/s per hop) and an ack-eliciting share of 2/3: the old
        fixed [1 %, 10 %] floor on PacketsLost / PacketsSent fails a sizeable share of runs, as
        seen (1 of 4 runs of A in the SI1b A/B)."""
        rng = random.Random(303)
        tr = m.Traffic(rate=17.0, ae=2 / 3)
        fails = 0
        for _ in range(300):
            rows = [(ph, m.loss_window(ph, tr, rng, detect=ph == "P3").incumbent)
                    for ph in ("P1", "P2", "P3") for _ in ("AB", "BA")]
            fails += m.old_rule_fails(rows)
        self.assertGreater(fails / 300, 0.05)
        self.assertLess(fails / 300, 0.60)

    def test_jitter_alone_makes_quic_declare_losses(self):
        """P3 has no loss, yet today's estimator read 0.5 % to 0.7 % there: with its first time
        threshold (9/8 RTT) quic-go declares about 2 reordered packets lost per 30 s window under
        the 5 ms jitter of wifi-clean, at today's traffic; PacketsLost (v0.64.1) keeps them."""
        rng = random.Random(505)
        tr = m.Traffic(rate=17.0, ae=2 / 3)
        ws = [m.loss_window("P3", tr, rng, detect=True, adapted=False) for _ in range(200)]
        self.assertGreater(statistics.fmean(w.spurious for w in ws), 1.0)
        self.assertGreater(sum(w.spurious >= 3 for w in ws) / len(ws), 0.1)  # >= 0.5 % of ~510

    def test_error_bound_without_priority(self):
        """ADR 0036, « Levels » (review 3, F1 to F3): with no priority, crossing retransmissions
        and ACK-only packets, the +-1 read race, a 20 ms read delay and head-of-line blocking, the
        figure stays within 15 permille of its window's loss at the 99.9th percentile, on every
        seed, at typing traffic on the heavy-reordering profile (the worst modelled case)."""
        r = m.errors(m.Traffic(rate=17), "P4", 600, range(31, 36))
        self.assertLess(max(r["error_p999_per_seed"]), 0.015, r)
        r = m.errors(m.Traffic(rate=DESIGN_RATE), "P1", 400, range(41, 44))
        self.assertLess(max(r["error_p999_per_seed"]), 0.008, r)

    def test_late_packets_beyond_the_ack_loop_are_loss(self):
        """Spec §8.5.2 (implementation of C5S): quic-go discards a packet that arrives more than its ACK
        loop after a higher-numbered one, before counting it. With no network loss and an RTT of
        25 ms, 20 % of the packets 60 ms late give about 15 % (measured: 15 %); P4, reordered within the
        loop, reads no loss; the first P4, beyond it, reads more than 10 %."""
        m.PROFILES["ack-loop"] = m.Netem(0.00625, 0.0)
        rng = random.Random(4)
        ours = m._poisson_times(rng, 120, 0, 60)
        late = [t + 0.0125 + (0.060 if rng.random() < 0.2 else 0.0) for t in ours]
        share = m._discard_late(late, lambda _t: m.PROFILES["ack-loop"], rng) / len(ours)
        self.assertGreater(share, 0.12)
        self.assertLess(share, 0.20)
        for prof, lo, hi in (("wifi-reorder", 0.0, 0.003), ("wifi-reorder-deep", 0.10, 0.25)):
            m.PHASES["PX"] = prof
            t_ = m.trace(m.Traffic(rate=120), random.Random(5), plan=(("PX", 60.0),), detect=False)
            figs = sorted(s[1]["honest"] for s in t_.steps if not math.isnan(s[1]["honest"]))
            with self.subTest(profile=prof):
                self.assertGreaterEqual(figs[len(figs) // 2], lo)
                self.assertLessEqual(figs[len(figs) // 2], hi)

    def test_paced_sender_keeps_its_echoes(self):
        """F8: with a paced sender (waits of a few ms) few echoes are late; a 300 ms stack wait
        makes most of them late, so the figure goes unknown rather than wrong."""
        rng = random.Random(2)
        paced = [m.loss_window("P1", m.Traffic(rate=300, stack=0.005), rng) for _ in range(150)]
        stuck = [m.loss_window("P1", m.Traffic(rate=300, stack=0.3), rng) for _ in range(150)]
        self.assertLess(statistics.fmean(w.late / 2 for w in paced), 0.2)
        self.assertGreater(statistics.fmean(w.late / 2 for w in stuck), 1.0)

    def test_minimum_of_200_packets(self):
        """§8.5.5: under 200 packets in the window the figure is unknown. At 2 packets/s (an
        idle link), 50 would show phantom loss of 10 permille or more in some windows."""
        rng = random.Random(9)
        tr = m.Traffic(rate=2)
        ws = [m.loss_window(ph, tr, rng) for ph in ("P3", "P4") for _ in range(600)]
        self.assertEqual([w for w in ws if w.sent >= 200], [])
        self.assertTrue(all(math.isnan(w.honest) for w in ws))

    def test_d1_needs_several_times_the_traffic_of_d2(self):
        """D1 counts only ack-eliciting packets (a sample of share f = 1/2 of ours): its residual
        is about twice as wide, so it needs about four times the traffic for the same rule."""
        rng = random.Random(404)
        tr = m.Traffic(rate=DESIGN_RATE)
        z2, z1 = [], []
        for i in range(1500):
            w = m.loss_window(("P1", "P2")[i % 2], tr, rng)
            s = math.sqrt(w.truth / w.n)
            z2.append((w.honest - w.truth) / s)
            z1.append((w.d1 - w.truth) / s)
        self.assertGreater(statistics.pstdev(z1) / statistics.pstdev(z2), 1.5)
        self.assertLess(abs(statistics.fmean(z2)), 0.15)  # D2 is about unbiased against qdisc truth


if __name__ == "__main__":
    unittest.main()
