"""Orchestration tests never connect to a display or execute the native desktop."""
import importlib.util
from pathlib import Path
from types import SimpleNamespace
from unittest import mock
import os
import shutil
import tempfile
import unittest

path = Path(__file__).with_name("type_native.py")
spec = importlib.util.spec_from_file_location("type_native", path)
harness = importlib.util.module_from_spec(spec)
spec.loader.exec_module(harness)


class NativeEntryTests(unittest.TestCase):
    def test_default_mutter_and_explicit_xtest_selection_reach_the_driver(self):
        for backend in ("mutter", "xtest"):
            calls = []
            def runner(command, **kwargs):
                calls.append(command)
                return SimpleNamespace(returncode=0)
            options = {} if backend == "mutter" else dict(input_backend="xtest")
            self.assertEqual(harness.execute(Path("collector"), Path("result.json"), Path("raw"), True,
                                             runner, **options), 0)
            self.assertEqual(calls[0][-2:], ["--input-backend", backend])
            self.assertEqual(len(calls), 2)

    def test_native_raw_evidence_survives_gate_output_cleanup(self):
        with tempfile.TemporaryDirectory(dir=harness.ROOT) as directory:
            root = Path(directory)
            out = root / "gates/L3"
            paths, annotated = [], []
            def execute(binary, output, raw, enabled, input_backend="mutter"):
                self.assertTrue(enabled)
                # Repository-relative: the committed manifest must not embed host paths.
                self.assertEqual(output, Path("gates/L3/PERF-TYPE.json"))
                self.assertFalse(raw.is_absolute())
                paths.append(root / raw / "warm-01.json")
                paths[-1].write_text('{"test_evidence":true}')
                return 1
            with mock.patch.object(harness, "ROOT", root), \
                 mock.patch.object(harness, "annotate_sources", annotated.append), \
                 mock.patch.object(harness, "build"), mock.patch.object(harness, "execute", execute), \
                 mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1", INKWELL_PERF_OUT=str(out)), \
                 mock.patch("sys.argv", ["type_native"]):
                self.assertEqual(harness.main(), 1)
            # The attribution table lives in the detail file, not the compact verdict.
            self.assertEqual(annotated, [out / "PERF-TYPE-detail.json"])
            shutil.rmtree(out)
            self.assertTrue(paths[0].is_file(), "gate cleanup deleted raw native evidence")

    def test_default_refusal_only_calls_non_gui_go_verdict(self):
        calls = []
        def runner(command, **kwargs):
            calls.append(command)
            return SimpleNamespace(returncode=1)
        self.assertEqual(harness.execute(Path("collector"), Path("result.json"), Path("raw"), False, runner), 1)
        self.assertEqual(len(calls), 1)
        self.assertTrue(any(x.startswith("--native-perf-unavailable=BLOCKED") for x in calls[0]))
        self.assertNotIn("native_input.py", " ".join(calls[0]))

    def test_failed_driver_is_still_merged_and_never_passes(self):
        calls = []
        codes = iter([1, 0, 1])
        def runner(command, **kwargs):
            calls.append(command)
            return SimpleNamespace(returncode=next(codes))
        self.assertEqual(harness.execute(Path("collector"), Path("result.json"), Path("raw"), True, runner), 1)
        self.assertIn("--native-perf-merge=raw", calls[1])
        self.assertTrue(any(x.startswith("--native-perf-unavailable=") for x in calls[2]))

    def test_verifier_failure_is_not_hidden_by_successful_collection(self):
        codes = iter([0, 1])
        self.assertEqual(harness.execute(Path("collector"), Path("result.json"), Path("raw"), True,
                         lambda *a, **kw: SimpleNamespace(returncode=next(codes))), 1)


class EnvironmentFaultRetryTests(unittest.TestCase):
    """Plan section 8: a fault of the host (driver exit 75) is retried with a fresh raw directory, invalid after
    three, and a valid failure is never retried."""

    def scripted(self, driver_codes, verifier_codes=()):
        """A runner whose driver and verifier calls return the scripted exit codes in order."""
        calls, drivers, verifiers = [], iter(driver_codes), iter(verifier_codes)

        def runner(command, **kwargs):
            calls.append(command)
            if "native_input.py" in " ".join(command):
                return SimpleNamespace(returncode=next(drivers))
            return SimpleNamespace(returncode=next(verifiers, 0))

        return runner, calls

    def run_execute(self, runner, coolings):
        return harness.execute(Path("collector"), Path("result.json"), Path("raw"), True, runner,
                               cool=lambda: coolings.append(1))

    def test_environment_fault_is_retried_with_fresh_raw(self):
        runner, calls = self.scripted([harness.EX_TEMPFAIL, 0], [0])
        coolings = []
        self.assertEqual(self.run_execute(runner, coolings), 0)
        drivers = [c for c in calls if "native_input.py" in " ".join(c)]
        self.assertEqual(len(drivers), 2)
        raws = [c[c.index("--output") + 1] for c in drivers]
        self.assertEqual(raws[0], "raw")
        self.assertNotEqual(raws[0], raws[1])  # a fresh raw directory: nothing of the faulted run is reused
        self.assertEqual(coolings, [1])
        merges = [c for c in calls if any(x.startswith("--native-perf-merge=") for x in c)]
        self.assertEqual(len(merges), 1)
        self.assertIn("--native-perf-merge=" + raws[1], merges[0])  # only the valid attempt is verified

    def test_three_faults_are_invalid_not_fail(self):
        runner, calls = self.scripted([harness.EX_TEMPFAIL] * 3 + [0])
        coolings = []
        self.assertEqual(self.run_execute(runner, coolings), 1)
        drivers = [c for c in calls if "native_input.py" in " ".join(c)]
        self.assertEqual(len(drivers), 3)  # never a fourth
        self.assertEqual(len({c[c.index("--output") + 1] for c in drivers}), 3)
        self.assertEqual(len(coolings), 2)
        invalid = [c for c in calls if any(x.startswith("--native-perf-invalid=") for x in c)]
        self.assertEqual(len(invalid), 1)
        self.assertFalse(any(x.startswith("--native-perf-merge=") or x.startswith("--native-perf-unavailable=")
                             for c in calls for x in c))  # not a verdict of failure, nothing merged

    def test_valid_fail_is_never_retried(self):
        runner, calls = self.scripted([1], [1])
        coolings = []
        self.assertEqual(self.run_execute(runner, coolings), 1)
        self.assertEqual(len([c for c in calls if "native_input.py" in " ".join(c)]), 1)
        self.assertEqual(coolings, [])
        self.assertFalse(any(x.startswith("--native-perf-invalid=") for c in calls for x in c))

    def test_frontend_coded_fault_in_the_verdict_is_retried_too(self):
        """The Go verifier reports status invalid for a coded fault of the webview: same retry."""
        statuses = iter(["invalid", "fail"])
        runner, calls = self.scripted([0, 0], [1, 1])
        with mock.patch.object(harness, "report_status", lambda output: next(statuses)):
            self.assertEqual(self.run_execute(runner, []), 1)
        self.assertEqual(len([c for c in calls if "native_input.py" in " ".join(c)]), 2)
        self.assertFalse(any(x.startswith("--native-perf-invalid=") for c in calls for x in c))

    def test_fault_reasons_come_from_driver_json(self):
        with tempfile.TemporaryDirectory(dir=harness.ROOT) as directory:
            raw = Path(directory)
            (raw / "driver.json").write_text('{"status":"fault","fault":{"kind":"focus-lost","message":"a | b"}}')
            self.assertEqual(harness.fault_reason(raw.relative_to(harness.ROOT)), "focus-lost: a / b")
            self.assertEqual(harness.fault_reason(Path("no-such-dir")), "environment fault (exit 75)")


if __name__ == "__main__":
    unittest.main()
