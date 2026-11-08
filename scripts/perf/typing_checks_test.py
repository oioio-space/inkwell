"""Typing-check driver tests: no window, bus, key or build."""
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import typing_checks as checks


def results(**override):
    base = dict(
        a_dead_keys=dict(cases=[dict(doc="ê")]), b_ime=dict(doc="☃"),
        c_undo=dict(typed="seed word", after_undo="seed ", after_redo="seed word"),
        d_accessibility=dict(text="x abc", events=[dict(type="object:text-caret-moved", detail1=3, detail2=0, text="", role="entry")]),
        e_editing=dict(completion=dict(doc="#left"), multi_cursor=dict(doc="aa\nXbb"), read_only=dict(doc="ro"), paste=dict(doc="abcabc")))
    base.update(override)
    return base


class TypingChecksTest(unittest.TestCase):
    def test_keysym_latin1_and_unicode(self):
        self.assertEqual(checks.keysym("e"), 0x65)
        self.assertEqual(checks.keysym("é"), 0xE9)
        self.assertEqual(checks.keysym("☃"), 0x1002603)

    def test_compose_lookup_follows_the_locale_table(self):
        with tempfile.TemporaryDirectory() as root:
            base = Path(root)
            (base / "compose.dir").write_text("en_US.UTF-8/Compose:\t\tfr_FR.UTF-8\n")
            (base / "en_US.UTF-8").mkdir()
            (base / "en_US.UTF-8/Compose").write_text(
                '<dead_diaeresis> <space>\t: "\\""\tquotedbl\n<dead_circumflex> <e>\t: "ê"\tecircumflex\n')
            with mock.patch.object(checks, "Path", side_effect=lambda p: base if p == "/usr/share/X11/locale" else Path(p)):
                self.assertEqual(checks.compose_lookup(("dead_diaeresis", "space"), "fr_FR.UTF-8"), '"')
                self.assertEqual(checks.compose_lookup(("dead_circumflex", "e"), "fr_FR.UTF-8"), "ê")
                self.assertIsNone(checks.compose_lookup(("dead_acute", "e"), "fr_FR.UTF-8"))
                self.assertIsNone(checks.compose_lookup(("dead_acute", "e"), "xx_XX.UTF-8"))

    def test_same_reports_only_differences(self):
        self.assertEqual(checks.same(results(), results()), {})
        quiet = results(d_accessibility=dict(text="x abc", events=[]))
        self.assertEqual(list(checks.same(quiet, results())), ["a11y"])


if __name__ == "__main__":
    unittest.main()
