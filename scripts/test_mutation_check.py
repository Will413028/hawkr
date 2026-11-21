import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path

_spec = importlib.util.spec_from_file_location("mutation_check", Path(__file__).with_name("mutation-check.py"))
mc = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(mc)

SRC = "a := 1\nb := 2\nb2 := 2\n"


class Base(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name)
        (self.root / "backend").mkdir()
        self.f = self.root / "backend" / "x.txt"
        self.f.write_text(SRC)

    def mut(self, old, cmd, new="Z", **kw):
        return {"id": "M", "cmd": cmd, "expect": "", "edits": [{"file": "backend/x.txt", "old": old, "new": new}], **kw}


class RunOneTest(Base):
    def test_old_not_found(self):
        self.assertIn("occurs 0 times", mc.run_one(self.root, self.mut("nope", "false"), False))
        self.assertEqual(self.f.read_text(), SRC)

    def test_old_ambiguous(self):
        self.assertIn("occurs 2 times", mc.run_one(self.root, self.mut(":= 2", "false"), False))

    def test_killed_when_command_fails(self):
        self.assertEqual(mc.run_one(self.root, self.mut("a := 1", "grep -q 'a := 1' x.txt"), False), "killed")

    def test_survived_when_command_passes(self):
        self.assertEqual(mc.run_one(self.root, self.mut("a := 1", "true"), False), "survived")

    def test_mutation_is_visible_to_command_and_restored(self):
        self.assertEqual(mc.run_one(self.root, self.mut("a := 1", "grep -q 'Z' x.txt"), False), "survived")
        self.assertEqual(self.f.read_text(), SRC)

    def test_restored_after_command_runner_raises(self):
        def boom(*a, **k):
            raise RuntimeError("x")
        with self.assertRaises(RuntimeError):
            mc.run_one(self.root, self.mut("a := 1", "true"), False, runner=boom)
        self.assertEqual(self.f.read_text(), SRC)

    def test_check_only_does_not_write_or_run(self):
        def boom(*a, **k):
            raise AssertionError("ran")
        self.assertEqual(mc.run_one(self.root, self.mut("a := 1", "x"), True, runner=boom), "applies")
        self.assertEqual(self.f.read_text(), SRC)

    def test_multi_file_edit_applies_both(self):
        g = self.root / "backend" / "y.txt"
        g.write_text("hello")
        m = self.mut("a := 1", "grep -q Z x.txt && grep -q W y.txt")
        m["edits"].append({"file": "backend/y.txt", "old": "hello", "new": "W"})
        self.assertEqual(mc.run_one(self.root, m, False), "survived")
        self.assertEqual(g.read_text(), "hello")

    def test_dirty_git_after_run_is_an_error(self):
        git = lambda *a: subprocess.run(["git", "-C", str(self.root), "-c", "user.name=t", "-c", "user.email=t@t", *a], check=True, capture_output=True)
        (self.root / "backend" / "y.txt").write_text("y")
        git("init", "-q")
        git("add", ".")
        git("commit", "-qm", "i")
        self.assertEqual(mc.run_one(self.root, self.mut("a := 1", "true"), False), "survived")
        # command leaves a stray change in another tracked file
        self.assertIn("git diff not clean", mc.run_one(self.root, self.mut("a := 1", "echo more >> y.txt; true"), False))


class MainTest(Base):
    def manifest(self, items):
        import json
        d = self.root / "docs" / "mutations"
        d.mkdir(parents=True, exist_ok=True)
        (d / "m.json").write_text(json.dumps(items))

    def test_exit_codes(self):
        e = {"id": "A", "file": "backend/x.txt", "old": "a := 1", "new": "Z", "cmd": "grep -q 'a := 1' x.txt"}
        self.manifest([e])
        self.assertEqual(mc.main(["--root", str(self.root)]), 0)
        self.manifest([{**e, "cmd": "true"}])
        self.assertEqual(mc.main(["--root", str(self.root)]), 1)
        self.manifest([{**e, "old": "missing"}])
        self.assertEqual(mc.main(["--root", str(self.root), "--check-only"]), 1)


if __name__ == "__main__":
    unittest.main()
