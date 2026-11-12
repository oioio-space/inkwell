#!/usr/bin/env python3
"""G4 of block SB1b (gates/SB1b.toml, criterion no-coredump-record; ADR 0042, amendment A5 and its 2026-11-12
re-review; plan docs/plans/2026-11-11-sandbox-crash-reports.md, section "Gate SB1b"): the user-visible criterion of W49.

Runs every hostile sandbox test with INKWELL_SANDBOX_DEATHS naming a manifest; each deliberate sandbox death appends one
JSON line {"test", "pid" (host), "nspid" (list, outermost first), "comm", "signal", "coredumped"}. Requires every named
test and every mandatory subtest to pass, and no SKIP or FAIL line anywhere. Then, where kernel.core_pattern pipes to
systemd-coredump:

1. completion barrier: no systemd-coredump unit active, activating or deactivating and no collector process, seen on two
   polls a second apart; then a marker sent through journald's native datagram socket (`logger --journald`, the same
   socket systemd-coredump writes its record to, read in order) must become visible: every record a collector sent
   before it exited is then indexed (bounded, 60 s each);
2. zero systemd-coredump records whose COREDUMP_PID is a recorded death or whose COREDUMP_EXE is sidecar.test, typst or
   tinymist; zero abrt problems naming them;
3. at least one kernel line "coredump: NSPID(COMM): RLIMIT_CORE is set to 1, aborting core" attributed to a recorded
   death (its innermost namespace PID and its command name), so a silent zero cannot pass.

Every failed or partial read fails. Socket mode (@, @@) fails with RS1. Another pattern prints "not applicable" once the
hostile tests pass. `--self-test` checks this logic against scripted journals, a delayed collector included.
"""
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent
MESSAGE_ID = "fc2e22bc6ee647b6b90729ab34a250b1"
NAMES = ("sidecar.test", "typst", "tinymist")
TAG = "inkwell-sb1b-g4"
KERNEL = re.compile(r"coredump: (\d+)\((.*?)\): RLIMIT_CORE is set to 1, aborting core")
# Top-level tests and, for some, the subtests that must each pass (go test -v names; spaces become underscores).
SUITES = [
    ("./internal/sidecar", 900, {
        "TestSeccompKillsABIMismatch": [], "TestInitPreservesFileSizeSignal": [], "TestRunFileSizeLimit": [],
        "TestRunDirectoryBound": [], "TestRunMemoryLimit": [],
        "TestSandboxDeathStartsNoCoreHelper": ["arch", "x32", "xfsz", "segv"],
        "TestFileSizeCapDispositionResetIsOutputLimit": [], "TestResidentBudgetKillsWithSIGKILL": [],
        "TestAddressBudgetFailsAllocationFirst": []}),
    ("./internal/typst", 900, {
        "TestCompileHostile": [], "TestHostileMemoryDeathStartsNoCoreHelper": ["local-limit", "address-limit"],
        "TestHostileCgroupMemoryIsMemoryLimit": []}),
]


def mode(pattern):
    if pattern.startswith("@"):
        return "socket"
    if pattern.startswith("|"):
        return "systemd-pipe" if "systemd-coredump" in pattern else "other-pipe"
    return "file"


def go_problems(out, tests):
    """Each named test and mandatory subtest printed --- PASS; no SKIP or FAIL line at any depth."""
    problems = [f"skipped or failing: {line.strip()}" for line in out.splitlines()
                if re.match(r"\s*--- (SKIP|FAIL): ", line)]
    for test, subtests in tests.items():
        for name in [test] + [f"{test}/{s}" for s in subtests]:
            if not re.search(rf"^\s*--- PASS: {re.escape(name)} \(", out, re.M):
                problems.append(f"missing or not passing: {name}")
    return problems


def manifest(text):
    records, problems = [], []
    for n, line in enumerate(text.splitlines(), 1):
        if not line.strip():
            continue
        try:
            r = json.loads(line)
            records.append((int(r["pid"]), [int(p) for p in r["nspid"]], str(r["comm"]), int(r["signal"]),
                            r["coredumped"]))
        except (ValueError, KeyError, TypeError) as exc:
            problems.append(f"manifest line {n} unreadable: {exc}")
    if not records and not problems:
        problems.append("no deliberate sandbox death was recorded (INKWELL_SANDBOX_DEATHS manifest empty)")
    problems += [f"pid {r[0]} ({r[2]}): the kernel started a core helper" for r in records if r[4] is not False]
    return records, problems


def text(v):
    if isinstance(v, list):
        return bytes(v).decode("utf-8", "replace") if all(isinstance(x, int) for x in v) else " ".join(map(str, v))
    return str(v or "")


def records_for(json_lines, records):
    pids = {str(r[0]) for r in records}
    return sum(1 for line in json_lines.splitlines() if line.strip()
               and (text(json.loads(line).get("COREDUMP_PID")) in pids
                    or any(x in text(json.loads(line).get("COREDUMP_EXE")) for x in NAMES)))


def attributed(kernel_lines, records):
    deaths = {(r[1][-1], r[2][:15]) for r in records if r[1]}  # comm is TASK_COMM_LEN - 1 = 15 bytes
    return sum(1 for m in KERNEL.finditer(kernel_lines) if (int(m.group(1)), m.group(2)) in deaths)


class Shell:
    def run(self, cmd, stdin=None, env=None):
        p = subprocess.run(cmd, input=stdin, capture_output=True, text=True, cwd=ROOT, env=env)
        return p.returncode, p.stdout, p.stderr

    clock = staticmethod(time.monotonic)
    sleep = staticmethod(time.sleep)


def barrier(sh, t0, nonce, limit=60.0):
    problems, quiet, deadline = [], 0, sh.clock() + limit
    while quiet < 2:
        rc, units, err = sh.run(["systemctl", "list-units", "--all", "--plain", "--no-legend",
                                 "--state=active,activating,deactivating", "systemd-coredump@*"])
        if rc != 0:
            return [f"systemctl list-units failed: {err.strip()[:200]}"]
        prc, procs, _ = sh.run(["pgrep", "-x", "systemd-coredum"])
        if prc not in (0, 1):
            return ["pgrep failed"]
        quiet = quiet + 1 if not units.strip() and prc == 1 else 0
        if quiet < 2:
            if sh.clock() > deadline:
                return ["a systemd-coredump collector was still running after the bound"]
            sh.sleep(1.0)
    rc, _, err = sh.run(["logger", "--journald"], stdin=f"MESSAGE={nonce}\nSYSLOG_IDENTIFIER={TAG}\n")
    if rc != 0:
        return [f"logger --journald failed: {err.strip()[:200]}"]
    deadline = sh.clock() + limit
    while True:
        rc, out, _ = sh.run(["journalctl", "-t", TAG, "--since", f"@{t0}", "-o", "cat"])
        if rc == 0 and nonce in out.split("\n"):
            return problems
        if sh.clock() > deadline:
            return ["the journal marker did not appear within the bound (journal unreadable or not caught up)"]
        sh.sleep(0.5)


def evidence(sh, t0, records):
    """Counts after the barrier; returns problems (an empty list is a pass)."""
    problems = []
    rc, out, err = sh.run(["journalctl", f"MESSAGE_ID={MESSAGE_ID}", "--since", f"@{t0}", "-o", "json"])
    if rc != 0 or re.search(r"insufficient permissions|no journal files", err, re.I):
        problems.append(f"journalctl (coredump records) unreadable: {err.strip()[:200]}")
    else:
        try:
            n = records_for(out, records)
        except ValueError as exc:
            problems.append(f"coredump records unparseable: {exc}")
        else:
            print(f"G4: systemd-coredump records for the tested deaths or sidecars: {n}")
            if n:
                problems.append(f"{n} systemd-coredump record(s) from the hostile runs")
    rc, out, err = sh.run(["journalctl", "-k", "--since", f"@{t0}", "-o", "cat"])
    if rc != 0:
        problems.append(f"journalctl -k failed: {err.strip()[:200]}")
    else:
        k = attributed(out, records)
        print(f"G4: kernel sentinel lines attributed to a recorded death: {k}")
        if k < 1:
            problems.append("no kernel line 'RLIMIT_CORE is set to 1, aborting core' for any recorded death")
    rc, _, _ = sh.run(["sh", "-c", "command -v abrt-cli"])
    if rc == 0:
        rc, out, err = sh.run(["abrt-cli", "list", "--since", str(t0)])
        if rc != 0:
            problems.append(f"abrt-cli list failed: {err.strip()[:200]}")
        else:
            a = sum(1 for line in out.splitlines() if re.search(r"typst|tinymist|sidecar", line))
            print(f"G4: abrt lines naming a sidecar: {a}")
            if a:
                problems.append("abrt recorded problems for a sidecar")
    return problems


def live(sh=Shell()):
    pattern = pathlib.Path("/proc/sys/kernel/core_pattern").read_text().strip()
    t0 = int(time.time())
    problems = []
    with tempfile.TemporaryDirectory(prefix="sb1b-g4-") as tmp:
        deaths = pathlib.Path(tmp) / "deaths.jsonl"
        deaths.touch()
        env = dict(os.environ, INKWELL_SANDBOX_DEATHS=str(deaths))
        for pkg, limit, tests in SUITES:
            regex = "^(" + "|".join(tests) + ")$"
            rc, out, err = sh.run(["./scripts/heavy.sh", "timeout", str(limit), "go", "test", "-count=1", "-v",
                                   "-run", regex, pkg], env=env)
            if rc != 0:
                print("\n".join((out + err).splitlines()[-40:]))
                problems.append(f"go test {pkg} exited with status {rc}")
            problems += go_problems(out, tests)
        records, bad = manifest(deaths.read_text())
        problems += bad
    m = mode(pattern)
    if m == "socket":
        problems.append(f"RS1: core_pattern is in socket mode ({pattern}); the RLIMIT_CORE sentinel does not apply")
    elif m == "systemd-pipe":
        barrier_problems = barrier(sh, t0, f"sb1b-g4-{os.getpid()}-{t0}")
        problems += barrier_problems or evidence(sh, t0, records)
    else:
        print(f"G4 not applicable: core_pattern is {m} ({pattern}); hostile tests checked only")
    for p in problems:
        print("G4:", p)
    print("G4: FAIL" if problems else "G4: pass")
    return 1 if problems else 0


class Fake:
    """A scripted shell and clock: answers(cmd, now) -> (rc, out, err)."""

    def __init__(self, answers):
        self.answers, self.now, self.calls = answers, 0.0, []

    def run(self, cmd, stdin=None, env=None):
        self.calls.append((self.now, cmd[0]))
        return self.answers(cmd, self.now)

    def clock(self):
        return self.now

    def sleep(self, s):
        self.now += s


DEATH = '{"test":"T","pid":4242,"nspid":[4242,2],"comm":"typst","signal":31,"coredumped":false}\n'


def journal(collector_until=0.0, record_at=None, marker=True, k_lines="coredump: 2(typst): RLIMIT_CORE is set to 1, aborting core\n"):
    record = json.dumps({"COREDUMP_PID": "4242", "COREDUMP_EXE": "/x/typst"})

    def answers(cmd, now):
        if cmd[0] == "systemctl":
            return 0, ("systemd-coredump@1-4242-0.service loaded active running x\n" if now < collector_until else ""), ""
        if cmd[0] == "pgrep":
            return (0, "77\n", "") if now < collector_until else (1, "", "")
        if cmd[0] == "logger":
            return 0, "", ""
        if cmd[:2] == ["journalctl", "-t"]:
            return 0, ("nonce\n" if marker else ""), ""
        if cmd[0] == "journalctl" and cmd[1].startswith("MESSAGE_ID"):
            return 0, (record + "\n" if record_at is not None and now >= record_at else ""), ""
        if cmd[:2] == ["journalctl", "-k"]:
            return 0, k_lines, ""
        if cmd[0] == "sh":
            return 1, "", ""
        raise AssertionError(cmd)
    return answers


class SelfTest(unittest.TestCase):
    def test_mode(self):
        self.assertEqual(mode("|/usr/lib/systemd/systemd-coredump %P %u"), "systemd-pipe")
        self.assertEqual(mode("|/usr/share/apport/apport %p"), "other-pipe")
        self.assertEqual(mode("@/run/systemd/coredumpd/kernel"), "socket")
        self.assertEqual(mode("@@/run/x"), "socket")
        self.assertEqual(mode("core"), "file")

    def test_a_skipped_subtest_fails_even_when_the_parent_passes(self):
        out = "    --- SKIP: TestA/x32 (0.0s)\n--- PASS: TestA (0.1s)\n"
        self.assertTrue(go_problems(out, {"TestA": []}))
        self.assertIn("missing or not passing: TestA/x32", go_problems("--- PASS: TestA (0.1s)\n", {"TestA": ["x32"]}))
        self.assertEqual(go_problems("    --- PASS: TestA/x32 (0s)\n--- PASS: TestA (0.1s)\n", {"TestA": ["x32"]}), [])

    def test_manifest(self):
        self.assertTrue(manifest("")[1], "an empty manifest is a failure")
        self.assertTrue(manifest("{bad\n")[1])
        self.assertTrue(manifest(DEATH.replace("false", "true"))[1])
        self.assertEqual(manifest(DEATH)[1], [])

    def test_records_attributed_by_pid_or_exe(self):
        recs = manifest(DEATH)[0]
        self.assertEqual(records_for(json.dumps({"COREDUMP_PID": "4242", "COREDUMP_EXE": "/usr/bin/x"}), recs), 1)
        self.assertEqual(records_for(json.dumps({"COREDUMP_PID": "1", "COREDUMP_EXE": list(b"/a/tinymist")}), recs), 1)
        self.assertEqual(records_for(json.dumps({"COREDUMP_PID": "1", "COREDUMP_EXE": "/usr/bin/gnome-shell"}), recs), 0)

    def test_kernel_line_must_belong_to_a_recorded_death(self):
        recs = manifest(DEATH)[0]
        self.assertEqual(attributed("coredump: 2(typst): RLIMIT_CORE is set to 1, aborting core", recs), 1)
        self.assertEqual(attributed("coredump: 3(typst): RLIMIT_CORE is set to 1, aborting core", recs), 0)
        self.assertEqual(attributed("coredump: 2(bash): RLIMIT_CORE is set to 1, aborting core", recs), 0)

    def test_delayed_collector_is_waited_for_and_its_record_counted(self):
        sh = Fake(journal(collector_until=5.0, record_at=4.5))
        self.assertEqual(barrier(sh, 0, "nonce"), [])
        self.assertGreaterEqual(sh.now, 5.0, "the barrier returned while the collector ran")
        self.assertIn("1 systemd-coredump record(s) from the hostile runs", evidence(sh, 0, manifest(DEATH)[0]))

    def test_clean_run_passes(self):
        sh = Fake(journal())
        self.assertEqual(barrier(sh, 0, "nonce") + evidence(sh, 0, manifest(DEATH)[0]), [])

    def test_bounds_and_failed_reads_fail(self):
        self.assertTrue(barrier(Fake(journal(collector_until=1e9)), 0, "nonce", limit=5))
        self.assertTrue(barrier(Fake(journal(marker=False)), 0, "nonce", limit=5))
        self.assertTrue(evidence(Fake(journal(k_lines="")), 0, manifest(DEATH)[0]))

        def denied(cmd, now):
            if cmd[0] == "journalctl" and cmd[1].startswith("MESSAGE_ID"):
                return 0, "", "No journal files were opened due to insufficient permissions."
            return journal()(cmd, now)
        self.assertTrue(evidence(Fake(denied), 0, manifest(DEATH)[0]))


if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        result = unittest.TextTestRunner(verbosity=1).run(unittest.defaultTestLoader.loadTestsFromTestCase(SelfTest))
        sys.exit(0 if result.wasSuccessful() else 1)
    sys.exit(live())
