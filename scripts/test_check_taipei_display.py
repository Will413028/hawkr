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

    def test_clean_lib_passes(self):
        self.assertEqual(self.run_on({"a.dart": "final x = t.taipeiWall;\n"}), [])

    def test_to_local_fails_everywhere_including_helper(self):
        self.assertEqual(len(self.run_on({"a.dart": "d.toLocal();\n"})), 1)
        self.assertEqual(
            len(self.run_on({"shared/time/taipei_display.dart": "d.toLocal();\n"})), 1
        )

    def test_format_call_outside_helper_fails(self):
        for fn in ("Timestamp", "DateTime", "Date"):
            self.assertEqual(
                len(self.run_on({"a.dart": f"formatTaipei{fn}(x);\n"})), 1, fn
            )

    def test_format_call_inside_helper_passes(self):
        self.assertEqual(
            self.run_on({"shared/time/taipei_display.dart": "formatTaipeiDate(x);\n"}),
            [],
        )

    def test_comment_lines_ignored(self):
        self.assertEqual(self.run_on({"a.dart": "  // d.toLocal() banned\n"}), [])


if __name__ == "__main__":
    unittest.main()
