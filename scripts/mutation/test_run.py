import contextlib
import io
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import run  # noqa: E402

# 假的測試指令：檔案內容含 MUTATED 就失敗；RAISE 環境變數存在時，執行測試本身丟例外。
CHECK = "import sys;sys.exit(1 if 'MUTATED' in open(sys.argv[1]).read() else 0)"


class Base(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.repo = Path(self._tmp.name)
        quiet = contextlib.ExitStack()
        quiet.enter_context(contextlib.redirect_stdout(io.StringIO()))
        quiet.enter_context(contextlib.redirect_stderr(io.StringIO()))
        self.addCleanup(quiet.close)
        (self.repo / "src.txt").write_text("alpha beta\n", encoding="utf-8")

    def mutant(self, **kw):
        base = dict(id="m", stack="go", file="src.txt", old="beta", new="MUTATED",
                    test="x", why="w", source="t.json")
        base.update(kw)
        return run.Mutant(**base)

    def builder(self, stack, selector, repo):
        return run.Command([sys.executable, "-c", CHECK, str(repo / "src.txt")], repo, dict(os.environ))

    def go(self, m):
        return run.run_mutant(m, self.repo, self.builder, run.run_test)

    def manifest(self, payload, name="m.json"):
        p = self.repo / name
        p.write_text(json.dumps(payload), encoding="utf-8")
        return p


class RunMutantTest(Base):
    def test_killed(self):
        self.assertEqual(self.go(self.mutant()), run.KILLED)

    def test_survived(self):
        self.assertEqual(self.go(self.mutant(new="gamma")), run.SURVIVED)

    def test_baseline_failed(self):
        (self.repo / "src.txt").write_text("MUTATED beta\n", encoding="utf-8")
        self.assertEqual(self.go(self.mutant()), run.BASELINE_FAILED)

    def test_apply_failed_zero_occurrences(self):
        self.assertEqual(self.go(self.mutant(old="nope")), run.APPLY_FAILED)

    def test_apply_failed_two_occurrences(self):
        (self.repo / "src.txt").write_text("beta beta\n", encoding="utf-8")
        self.assertEqual(self.go(self.mutant()), run.APPLY_FAILED)

    def test_restores_after_kill_and_survive(self):
        self.go(self.mutant())
        self.go(self.mutant(new="gamma"))
        self.assertEqual((self.repo / "src.txt").read_text(encoding="utf-8"), "alpha beta\n")

    def test_restores_when_runner_raises(self):
        calls = []

        def runner(cmd):
            calls.append(1)
            if len(calls) == 2:
                raise RuntimeError("boom")
            return "PASS"

        with self.assertRaises(RuntimeError):
            run.run_mutant(self.mutant(), self.repo, self.builder, runner)
        self.assertEqual((self.repo / "src.txt").read_text(encoding="utf-8"), "alpha beta\n")

    def test_timeout_is_not_killed(self):
        seq = iter(["PASS", run.TIMEOUT])
        r = run.run_mutant(self.mutant(), self.repo, self.builder, lambda c: next(seq))
        self.assertEqual(r, run.TIMEOUT)

    def test_runner_error_after_mutation_counts_as_killed_only_if_not_pass(self):
        seq = iter(["PASS", "ERROR"])
        r = run.run_mutant(self.mutant(), self.repo, self.builder, lambda c: next(seq))
        self.assertEqual(r, run.KILLED)


class ExecuteTest(Base):
    def test_exit_codes(self):
        self.assertEqual(run.execute([self.mutant()], self.repo, self.builder), 0)
        self.assertEqual(run.execute([self.mutant(), self.mutant(id="s", new="gamma")], self.repo, self.builder), 1)
        self.assertEqual(run.execute([], self.repo, self.builder), 1)

    def test_step_summary_written(self):
        out = self.repo / "summary.md"
        os.environ["GITHUB_STEP_SUMMARY"] = str(out)
        self.addCleanup(os.environ.pop, "GITHUB_STEP_SUMMARY", None)
        run.execute([self.mutant()], self.repo, self.builder)
        self.assertIn("1/1 KILLED", out.read_text(encoding="utf-8"))


class ManifestTest(Base):
    good = dict(id="a", stack="go", file="f", old="o", new="n", test="t", why="w")

    def load(self, payload):
        return run.load_manifest(self.manifest(payload))

    def test_valid(self):
        self.assertEqual(len(self.load({"mutants": [self.good]})), 1)

    def test_flutter_stack_valid(self):
        self.assertEqual(self.load({"mutants": [dict(self.good, stack="flutter:buyer-app")]})[0].stack, "flutter:buyer-app")

    def test_errors(self):
        bad = [
            {"mutants": []},
            [],
            {"mutants": [dict(self.good, stack="rust")]},
            {"mutants": [dict(self.good, stack="flutter:")]},
            {"mutants": [{k: v for k, v in self.good.items() if k != "why"}]},
            {"mutants": [dict(self.good, extra="x")]},
            {"mutants": [dict(self.good, old="same", new="same")]},
            {"mutants": [dict(self.good, old="")]},
            {"mutants": [self.good, self.good]},
            {"mutants": [dict(self.good, old=1)]},
        ]
        for payload in bad:
            with self.subTest(payload=payload), self.assertRaises(run.ManifestError):
                self.load(payload)

    def test_not_json(self):
        p = self.repo / "x.json"
        p.write_text("{", encoding="utf-8")
        with self.assertRaises(run.ManifestError):
            run.load_manifest(p)

    def test_stack_filter_and_list(self):
        p = self.manifest({"mutants": [self.good, dict(self.good, id="b", stack="website")]})
        self.assertEqual([m.id for m in run.select([str(p)], "website")], ["b"])
        self.assertEqual(len(run.select([str(p)], None)), 2)

    def test_main_exit_codes(self):
        p = self.manifest({"mutants": [self.good]})
        self.assertEqual(run.main([str(p), "--list-stacks"]), 0)
        self.assertEqual(run.main([str(p), "--stack", "website"]), 1)
        bad = self.manifest({"mutants": []}, "bad.json")
        self.assertEqual(run.main([str(bad)]), 2)


class CommandTest(unittest.TestCase):
    def test_commands(self):
        repo = Path("/r")
        c = run.build_command("go-integration", "internal/x ^TestA$", repo)
        self.assertEqual(c.argv, ["go", "test", "-count=1", "-tags", "integration", "./internal/x/", "-run", "^TestA$"])
        self.assertEqual((c.cwd, c.env["GOWORK"]), (repo / "backend", "off"))
        self.assertNotIn("integration", run.build_command("go", "p ^T$", repo).argv)
        w = run.build_command("website", "a/b.test.ts some name", repo)
        self.assertEqual(w.argv[-2:], ["-t", "some name"])
        self.assertEqual(run.build_command("website", "a.ts", repo).argv[-1], "a.ts")
        f = run.build_command("flutter:buyer-app", "test/a_test.dart my case", repo)
        self.assertEqual(f.cwd, repo / "buyer-app")
        self.assertEqual(f.argv[-2:], ["--plain-name", "my case"])


if __name__ == "__main__":
    unittest.main()
