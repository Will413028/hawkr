import importlib.util
import tempfile
import unittest
from pathlib import Path

_spec = importlib.util.spec_from_file_location(
    "check_taipei_display", Path(__file__).with_name("check-taipei-display.py")
)
check = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(check)


class CheckTaipeiDisplayTests(unittest.TestCase):
    def run_on(self, files):
        with tempfile.TemporaryDirectory() as d:
            lib = Path(d) / "lib"
            for rel, text in files.items():
                p = lib / rel
                p.parent.mkdir(parents=True, exist_ok=True)
                p.write_text(text, encoding="utf-8")
            return list(check.violations(lib))

    def flagged(self, text):
        return len(self.run_on({"a.dart": text}))

    def test_clean_lib_passes(self):
        self.assertEqual(self.flagged("final x = d.toUtc();\n"), 0)

    def test_to_local_fails_in_any_file(self):
        self.assertEqual(self.flagged("d.toLocal();\n"), 1)
        self.assertEqual(
            len(self.run_on({"shared/time/taipei_display.dart": "d.toLocal();\n"})), 1
        )

    def test_comments_ignored(self):
        self.assertEqual(self.flagged("  // d.toLocal() banned\n"), 0)
        self.assertEqual(self.flagged("/* d.toLocal() */ final a = 1;\n"), 0)
        self.assertEqual(self.flagged("/* a\n d.toLocal()\n /* nested */ b */\n"), 0)
        self.assertEqual(self.flagged("/// d.toLocal()\n"), 0)

    def test_trailing_comment_ignored_but_code_before_it_counts(self):
        self.assertEqual(self.flagged("final a = 1; // d.toLocal()\n"), 0)
        self.assertEqual(self.flagged("d.toLocal(); // why\n"), 1)

    def test_double_slash_in_string_is_not_a_comment(self):
        self.assertEqual(self.flagged("final u = 'http://x'; d.toLocal();\n"), 1)

    def test_string_contents_ignored(self):
        self.assertEqual(self.flagged("final s = 'call d.toLocal() later';\n"), 0)
        self.assertEqual(self.flagged('final s = "d.toLocal()";\n'), 0)

    def test_raw_string_with_backslash_and_comment_marker(self):
        self.assertEqual(self.flagged("final s = r'C:\\\\ // d.toLocal()';\n"), 0)
        self.assertEqual(self.flagged("final s = r'\\'; d.toLocal();\n"), 1)

    def test_triple_quoted_multiline_string(self):
        self.assertEqual(
            self.flagged("final s = '''\nline // one\nd.toLocal()\n''';\n"), 0
        )
        self.assertEqual(
            self.flagged("final s = '''\nx\n'''; d.toLocal();\n"), 1
        )

    def test_interpolation_is_code(self):
        self.assertEqual(self.flagged("final s = 'at ${d.toLocal()}';\n"), 1)
        self.assertEqual(self.flagged("final s = 'at ${f({1: 2})} d.toLocal()';\n"), 0)

    def test_reports_line_number(self):
        found = self.run_on({"a.dart": "a;\nb;\nd.toLocal();\n"})
        self.assertTrue(found[0].startswith(str(Path(found[0].split(":")[0])) + ":3:"))


if __name__ == "__main__":
    unittest.main()
