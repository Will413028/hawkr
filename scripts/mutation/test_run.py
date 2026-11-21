import contextlib
import io
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import run  # noqa: E402

# 假的測試指令：檔案內容含 MUTATED 就失敗；RAISE 環境變數存在時，執行測試本身丟例外。
CHECK = "import sys;sys.exit(1 if 'MUTATED' in open(sys.argv[1]).read() else 0)"


class Base(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.repo = Path(self._tmp.name)
        env = mock.patch.dict(os.environ)
        env.start()
        self.addCleanup(env.stop)
        os.environ.pop("GITHUB_STEP_SUMMARY", None)
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

    def test_build_failure_is_not_killed(self):
        calls = []

        def runner(cmd):
            calls.append(cmd.argv[0])
            return "FAIL" if cmd.argv[0] == "build" else "PASS"

        bb = lambda stack, sel, file, repo: run.Command(["build"], repo, {})
        self.assertEqual(run.run_mutant(self.mutant(), self.repo, self.builder, runner, bb), run.BUILD_FAILED)
        self.assertEqual((self.repo / "src.txt").read_text(encoding="utf-8"), "alpha beta\n")

    def test_build_ok_then_killed(self):
        bb = lambda stack, sel, file, repo: run.Command([sys.executable, "-c", "pass"], repo, {})
        self.assertEqual(run.run_mutant(self.mutant(), self.repo, self.builder, run.run_test, bb), run.KILLED)

    def test_no_tests_executed_is_build_failed(self):
        seq = iter(["PASS", run.NO_TESTS])
        r = run.run_mutant(self.mutant(), self.repo, self.builder, lambda c: next(seq))
        self.assertEqual(r, run.BUILD_FAILED)

    def test_escaping_file_is_apply_failed(self):
        self.assertEqual(self.go(self.mutant(file="../x")), run.APPLY_FAILED)

    def test_runner_error_after_mutation_counts_as_killed_only_if_not_pass(self):
        seq = iter(["PASS", "ERROR"])
        r = run.run_mutant(self.mutant(), self.repo, self.builder, lambda c: next(seq))
        self.assertEqual(r, run.KILLED)


class ExecuteTest(Base):
    def test_exit_codes(self):
        self.assertEqual(run.execute([self.mutant()], self.repo, self.builder, build_builder=None), 0)
        self.assertEqual(run.execute([self.mutant(), self.mutant(id="s", new="gamma")], self.repo, self.builder, build_builder=None), 1)
        self.assertEqual(run.execute([], self.repo, self.builder, build_builder=None), 1)

    def test_step_summary_written(self):
        out = self.repo / "summary.md"
        os.environ["GITHUB_STEP_SUMMARY"] = str(out)
        run.execute([self.mutant()], self.repo, self.builder, build_builder=None)
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

    def test_flutter_stack_whitelist(self):
        with self.assertRaises(run.ManifestError):
            self.load({"mutants": [dict(self.good, stack="flutter:other")]})

    def test_validate_rejects_escaping_paths(self):
        outside = self.repo.parent / "outside.txt"
        self.assertIsNotNone(run.check_mutant(self.mutant(file="../outside.txt"), self.repo))
        self.assertIn("repo 之外", run.check_mutant(self.mutant(file="../outside.txt"), self.repo))
        (self.repo / "backend").mkdir()
        self.assertIn("repo 之外", run.check_mutant(self.mutant(test="../../x ^T$"), self.repo))

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
        t = f"{run.FRAMEWORK_TIMEOUT_SECONDS}s"
        self.assertGreater(run.FRAMEWORK_TIMEOUT_SECONDS, run.TIMEOUT_SECONDS)
        self.assertEqual(c.argv, ["go", "test", "-count=1", "-timeout", t, "-tags", "integration", "./internal/x/", "-run", "^TestA$"])
        self.assertEqual((c.cwd, c.env["GOWORK"]), (repo / "backend", "off"))
        self.assertNotIn("integration", run.build_command("go", "p ^T$", repo).argv)
        w = run.build_command("website", "a/b.test.ts some name", repo)
        self.assertEqual(w.argv[-2:], ["-t", "some name"])
        self.assertEqual(run.build_command("website", "a.ts", repo).argv[-1], "a.ts")
        f = run.build_command("flutter:buyer-app", "test/a_test.dart my case", repo)
        self.assertEqual(f.cwd, repo / "buyer-app")
        self.assertEqual(f.argv[-2:], ["--plain-name", "my case"])
        self.assertEqual(f.argv[f.argv.index("--timeout") + 1], t)

    def test_build_check_commands(self):
        repo = Path("/r")
        g = run.build_check_command("go-integration", "internal/x ^T$", "backend/internal/x/a.go", repo)
        self.assertEqual(g.argv, ["go", "test", "-c", "-o", os.devnull, "-tags", "integration", "./internal/x/"])
        self.assertEqual((g.cwd, g.env["GOWORK"]), (repo / "backend", "off"))
        self.assertNotIn("integration", run.build_check_command("go", "p ^T$", "f", repo).argv)
        d = run.build_check_command("flutter:seller-app", "test/a.dart", "seller-app/lib/a.dart", repo)
        self.assertEqual(d.argv[:3], ["dart", "analyze", "--no-fatal-warnings"])
        self.assertEqual(d.cwd, repo / "seller-app")
        self.assertIsNone(run.build_check_command("website", "a.ts", "website/a.ts", repo))

    def test_ran_no_tests(self):
        self.assertTrue(run.ran_no_tests("website", "Error: Transform failed\n Tests  no tests"))
        self.assertFalse(run.ran_no_tests("website", " Tests  1 failed | 3 passed (4)"))
        self.assertTrue(run.ran_no_tests("flutter:buyer-app", "Failed to load \"test/a_test.dart\""))
        self.assertFalse(run.ran_no_tests("flutter:buyer-app", "00:01 +0 -1: Some tests failed."))
        self.assertFalse(run.ran_no_tests("go", "FAIL"))


if __name__ == "__main__":
    unittest.main()
