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


if __name__ == "__main__":
    unittest.main()
