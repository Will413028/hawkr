import contextlib
import io
import json
import os
import shlex
import shutil
import sys
import tempfile
import time
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
        seq = iter(["PASS", run.BUILD_ERROR])
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

    def test_new_stacks_valid(self):
        for st in ("admin", "go:backend/services/invoice-service", "go-integration:backend/services/invoice-service"):
            with self.subTest(stack=st):
                self.assertEqual(self.load({"mutants": [dict(self.good, stack=st)]})[0].stack, st)
        for st in ("go:", "go:/abs", "go:../x", "go:a//b", "go:a/./b", "go:a/", "admin:x", "gox:a"):
            with self.subTest(stack=st), self.assertRaises(run.ManifestError):
                self.load({"mutants": [dict(self.good, stack=st)]})

    def test_validate_module_dir(self):
        mod = self.repo / "svc" / "inv"
        (mod / "internal").mkdir(parents=True)
        (self.repo / "svc" / "nomod").mkdir()
        m = self.mutant(stack="go:svc/inv", test="internal ^T$")
        self.assertIn("go.mod", run.check_mutant(m, self.repo))
        (mod / "go.mod").write_text("module x\n", encoding="utf-8")
        self.assertIsNone(run.check_mutant(m, self.repo))
        self.assertIn("go.mod", run.check_mutant(self.mutant(stack="go:svc/nomod", test="internal ^T$"), self.repo))
        self.assertIn("go.mod", run.check_mutant(self.mutant(stack="go:svc/missing", test="internal ^T$"), self.repo))
        self.assertIn("測試路徑不存在", run.check_mutant(self.mutant(stack="go:svc/inv", test="nope ^T$"), self.repo))

    def test_flutter_stack_whitelist(self):
        with self.assertRaises(run.ManifestError):
            self.load({"mutants": [dict(self.good, stack="flutter:other")]})

    def test_validate_rejects_escaping_paths(self):
        with tempfile.TemporaryDirectory() as other:
            real = Path(other) / "outside.txt"
            real.write_text("alpha beta\n", encoding="utf-8")
            (self.repo / "link.txt").symlink_to(real)
            for f in ("link.txt", str(real)):
                with self.subTest(file=f):
                    self.assertIn("repo 之外", run.check_mutant(self.mutant(file=f), self.repo))
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
        self.assertEqual(c.argv, ["go", "test", "-count=1", "-vet=off", "-timeout", t, "-tags", "integration", "./internal/x/", "-run", "^TestA$"])
        self.assertEqual((c.cwd, c.env["GOWORK"]), (repo / "backend", "off"))
        self.assertNotIn("integration", run.build_command("go", "p ^T$", repo).argv)
        w = run.build_command("website", "a/b.test.ts some name", run.REPO)
        self.assertEqual(w.argv[-2:], ["-t", "some name"])
        self.assertEqual(run.build_command("website", "a.ts", run.REPO).argv[-1], "a.ts")
        f = run.build_command("flutter:buyer-app", "test/a_test.dart my case", repo)
        self.assertEqual(f.cwd, repo / "buyer-app")
        self.assertEqual(f.argv[-2:], ["--plain-name", "my case"])
        self.assertNotIn("--timeout", f.argv)
        self.assertIn("-vet=off", c.argv)

    def test_build_check_commands(self):
        repo = Path("/r")
        g = run.build_check_command("go-integration", "internal/x ^T$", "backend/internal/x/a.go", repo)
        self.assertEqual(g.argv, ["go", "test", "-c", "-o", os.devnull, "-tags", "integration", "./internal/x/"])
        self.assertEqual((g.cwd, g.env["GOWORK"]), (repo / "backend", "off"))
        self.assertNotIn("integration", run.build_check_command("go", "p ^T$", "f", repo).argv)
        self.assertIsNone(run.build_check_command("flutter:seller-app", "test/a.dart", "packages/hawkr_ui_text/lib/a.dart", repo))
        d = run.build_check_command("flutter:seller-app", "test/a.dart", "seller-app/lib/a.dart", repo)
        self.assertEqual(d.argv[:3], ["dart", "analyze", "--no-fatal-warnings"])
        self.assertEqual(d.cwd, repo / "seller-app")
        self.assertIsNone(run.build_check_command("website", "a.ts", "website/a.ts", repo))

    def test_module_go_commands(self):
        repo = Path("/r")
        mod = "backend/services/invoice-service"
        c = run.build_command(f"go:{mod}", "internal/usecase ^TestA$", repo)
        self.assertEqual(c.cwd, repo / mod)
        self.assertEqual(c.env["GOWORK"], "off")
        self.assertNotIn("integration", c.argv)
        self.assertEqual(c.argv[-3:], ["./internal/usecase/", "-run", "^TestA$"])
        i = run.build_command(f"go-integration:{mod}", "internal/usecase ^TestA$", repo)
        self.assertEqual(i.argv[i.argv.index("-tags") + 1], "integration")
        self.assertEqual(i.cwd, repo / mod)
        b = run.build_check_command(f"go-integration:{mod}", "internal/usecase ^T$", f"{mod}/a.go", repo)
        self.assertEqual(b.argv, ["go", "test", "-c", "-o", os.devnull, "-tags", "integration", "./internal/usecase/"])
        self.assertEqual((b.cwd, b.env["GOWORK"]), (repo / mod, "off"))
        self.assertEqual(run.test_file(f"go:{mod}", "internal/usecase ^T$", repo), repo / mod / "internal/usecase")
        self.assertEqual(run.test_file("go", "p ^T$", repo), repo / "backend" / "p")

    def test_admin_commands(self):
        repo = run.REPO
        base = run.package_script(repo, "admin", "test:file")
        a = run.build_command("admin", "tests/x.test.ts my case", repo)
        self.assertEqual(a.cwd, repo / "admin")
        self.assertEqual(a.argv, base + ["--test-name-pattern", "my case", "tests/x.test.ts"])
        self.assertEqual(run.build_command("admin", "tests/x.test.ts", repo).argv, base + ["tests/x.test.ts"])
        self.assertEqual(base[-1], "--test")
        self.assertIsNone(run.build_check_command("admin", "tests/x.test.ts", "admin/a.ts", repo))
        self.assertEqual(run.test_file("admin", "tests/x.test.ts", repo), repo / "admin" / "tests/x.test.ts")

    def fake_repo(self, admin=None, website=None):
        repo = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, repo)
        for app, scripts in (("admin", {"test:file": "node --import ./x.mjs --test"} if admin is None else admin),
                             ("website", {"test:file": "vitest run --project unit"} if website is None else website)):
            (repo / app).mkdir()
            (repo / app / "package.json").write_text(json.dumps({"scripts": scripts}), encoding="utf-8")
        return repo

    def test_commands_follow_package_scripts(self):
        repo = self.fake_repo(website={"test:file": "vitest run --project=unit"})
        self.assertEqual(run.build_command("admin", "t.ts n", repo).argv,
                         ["node", "--import", "./x.mjs", "--test", "--test-name-pattern", "n", "t.ts"])
        self.assertEqual(run.build_command("website", "t.ts n", repo).argv,
                         ["pnpm", "exec", "vitest", "run", "--project=unit", "t.ts", "-t", "n"])
        (repo / "admin" / "package.json").write_text("{}", encoding="utf-8")
        with self.assertRaises(run.ManifestError):
            run.build_command("admin", "t.ts", repo)

    def test_script_grammar(self):
        accept = {
            "admin": ["node --test", "node --import tsx --test", "node --import tsx --import ./tests/setup/env.mjs --test",
                      "node --import 'a-b/c.mjs' --test"],
            "website": ["vitest run --project unit", "vitest run --project=unit"],
        }
        reject = {
            "admin": [7, ["node"], "", "FOO=1 node --test", "NODE_ENV=test node --test", "node --test && echo x",
                      "node --test; echo", "node --test | cat", "node --test > out", "node $X --test",
                      "pnpm run test:file --test", "tsx --test", "node --test tests/a.test.ts", "node --test 'unterminated",
                      "node tests/a.test.ts --test", "node --import ./setup/*.mjs --test", "node --import --test",
                      "node --import tsx", "node --import -x --test", "node --import tsx --require y --test",
                      "node --import tsx --test --test", "node --import './a b.mjs' --test", "node --import ~/x.mjs --test",
                      "node --import ./a{b,c}.mjs --test", "node --import ./a?.mjs --test", "node --import tsx extra --test",
                      "node --import tsx --test --experimental-x"],
            "website": [7, "", "CI=1 vitest run --project unit", "vitest run --project unit && true",
                        "pnpm run test:file", "node vitest run --project unit", "vitest --project unit",
                        "vitest run --project browser", "vitest run", "vitest run --project unit tests/a.test.ts",
                        "vitest run --project", "vitest run --project unit | cat", "vitest run --project unit --project=browser",
                        "vitest run --project unit --project browser", "vitest run --project unit --", "vitest run --project unit --reporter=dot",
                        "vitest run --reporter=dot --project unit", "vitest run --project=browser", "vitest run --project=unit extra",
                        "pnpm exec vitest run --project unit", "vitest run --project  unit --"],
        }
        for app, scripts in accept.items():
            for script in scripts:
                repo = self.fake_repo(**{app: {"test:file": script}})
                with self.subTest(app=app, accept=script):
                    self.assertEqual(run.package_script(repo, app, "test:file"), shlex.split(script))
        for app, scripts in reject.items():
            for script in scripts:
                repo = self.fake_repo(**{app: {"test:file": script}})
                with self.subTest(app=app, reject=script):
                    with self.assertRaises(run.ManifestError) as cm:
                        run.build_command(app, "t.ts", repo)
                    self.assertIn(f"{app}/package.json", str(cm.exception))
                    self.assertIn("test:file", str(cm.exception))

    def test_validate_fails_when_package_script_invalid(self):
        repo = self.fake_repo(admin={"test:file": "FOO=1 node --test"})
        with mock.patch.object(run, "REPO", repo):
            (repo / "admin" / "tests").mkdir()
            (repo / "admin" / "tests" / "a.test.ts").write_text("t", encoding="utf-8")
            (repo / "admin" / "a.ts").write_text("let a = 1\n", encoding="utf-8")
            m = repo / "m.json"
            m.write_text(json.dumps({"mutants": [{"id": "x", "stack": "admin", "file": "admin/a.ts",
                                                  "old": "a = 1", "new": "a = 2", "test": "tests/a.test.ts n", "why": "w"}]}), encoding="utf-8")
            err = io.StringIO()
            with contextlib.redirect_stdout(io.StringIO()) as out, contextlib.redirect_stderr(err):
                self.assertEqual(run.main(["--validate", str(m)]), 1, err.getvalue())
            self.assertIn("FAIL: x", out.getvalue())
            (repo / "admin" / "package.json").write_text(
                json.dumps({"scripts": {"test:file": "node --test"}}), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()) as out:
                self.assertEqual(run.main(["--validate", str(m)]), 0)

    def test_real_package_scripts_drive_runner(self):
        repo = run.REPO
        admin = run.package_script(repo, "admin", "test:file")
        website = run.package_script(repo, "website", "test:file")
        self.assertEqual(admin, ["node", "--import", "tsx", "--import", "./tests/setup/env.mjs", "--test"])
        self.assertEqual(website, ["vitest", "run", "--project", "unit"])
        self.assertEqual(run.build_command("admin", "a.test.ts N", repo).argv,
                         ["node", "--import", "tsx", "--import", "./tests/setup/env.mjs", "--test",
                          "--test-name-pattern", "N", "a.test.ts"])
        self.assertEqual(run.build_command("website", "a.test.ts N", repo).argv,
                         ["pnpm", "exec", "vitest", "run", "--project", "unit", "a.test.ts", "-t", "N"])
        self.assertEqual(run.build_command("admin", "tests/x.test.ts my case", repo).argv,
                         [*admin, "--test-name-pattern", "my case", "tests/x.test.ts"])
        self.assertEqual(run.build_command("website", "a.test.ts my case", repo).argv,
                         ["pnpm", "exec", *website, "a.test.ts", "-t", "my case"])
        scripts = {a: json.loads((repo / a / "package.json").read_text(encoding="utf-8"))["scripts"]
                   for a in ("admin", "website")}
        self.assertEqual(scripts["website"]["test"], "pnpm run test:file")
        self.assertTrue(scripts["admin"]["test"].startswith("pnpm run test:file "))

    def test_admin_build_error(self):
        self.assertTrue(run.has_build_error("admin", "x.ts:2:18: ERROR: Unexpected \";\"\n  name: 'TransformError'"))
        self.assertTrue(run.has_build_error("admin", "SyntaxError: Unexpected token"))
        self.assertFalse(run.has_build_error("admin", "AssertionError [ERR_ASSERTION]: 1 !== 2\nℹ fail 1"))
        self.assertFalse(run.has_build_error("go:backend/services/x", "TransformError"))

    def test_has_build_error(self):
        self.assertTrue(run.has_build_error("website", "Error: Transform failed with 1 error"))
        self.assertTrue(run.has_build_error("website", "SyntaxError: Unexpected token"))
        self.assertFalse(run.has_build_error("website", "Error: connect ECONNREFUSED"))
        self.assertFalse(run.has_build_error("website", " Tests  1 failed | 3 passed (4)"))
        self.assertTrue(run.has_build_error("flutter:buyer-app", "Failed to load \"t.dart\":\nCompilation failed for testPath=x"))
        self.assertTrue(run.has_build_error("flutter:buyer-app", "lib/a.dart:3:1: Error: Expected ';'."))
        self.assertFalse(run.has_build_error("flutter:buyer-app", "00:01 +0 -1: Some tests failed."))
        self.assertFalse(run.has_build_error("flutter:buyer-app", "Failed to load \"t.dart\": timeout"))
        self.assertFalse(run.has_build_error("go", "FAIL"))


class ProcessTest(Base):
    def test_timeout_kills_process_tree(self):
        pidfile = self.repo / "pid"
        child = self.repo / "child.py"
        child.write_text(f"import os,time\nopen({str(pidfile)!r},'w').write(str(os.getpid()))\ntime.sleep(60)\n", encoding="utf-8")
        code = f"import subprocess,sys,time\nsubprocess.Popen([sys.executable,{str(child)!r}])\ntime.sleep(60)\n"
        with mock.patch.object(run, "TIMEOUT_SECONDS", 2):
            r = run.run_test(run.Command([sys.executable, "-c", code], self.repo, dict(os.environ)))
        self.assertEqual(r, run.TIMEOUT)
        pid = int(pidfile.read_text())
        time.sleep(0.5)
        with self.assertRaises(ProcessLookupError):
            os.kill(pid, 0)

    def test_cannot_start_is_error(self):
        self.assertEqual(run.run_test(run.Command(["/nonexistent-bin"], self.repo, {})), run.ERROR)


class BuildCheckOutcomeTest(Base):
    def outcome(self, build_result):
        def runner(cmd):
            return build_result if cmd.argv[0] == "build" else "PASS"
        bb = lambda stack, sel, file, repo: run.Command(["build"], repo, {})
        return run.run_mutant(self.mutant(), self.repo, self.builder, runner, bb)

    def test_mapping(self):
        self.assertEqual(self.outcome(run.TIMEOUT), run.TIMEOUT)
        self.assertEqual(self.outcome(run.ERROR), run.ERROR)
        self.assertEqual(self.outcome("FAIL"), run.BUILD_FAILED)


class AtlasSumTest(Base):
    def migration_dir(self):
        d = self.repo / "db" / "migrations"
        d.mkdir(parents=True)
        (d / "20260101000000_a.sql").write_text("CREATE TABLE t (x int NOT NULL);\n", encoding="utf-8")
        (d / "20260102000000_b.sql").write_text("ALTER TABLE t ADD COLUMN y int;\n", encoding="utf-8")
        (d / "atlas.sum").write_text(run.atlas_sum(d), encoding="utf-8")
        return d

    def test_reproduces_committed_sums(self):
        dirs = sorted(run.REPO.glob("backend/**/db/migrations"))
        dirs = [d for d in dirs if (d / "atlas.sum").is_file()]
        self.assertGreater(len(dirs), 0)
        for d in dirs:
            self.assertEqual(run.atlas_sum(d), (d / "atlas.sum").read_text(encoding="utf-8"), str(d))

    def test_mutated_migration_gets_valid_sum_and_restores(self):
        d = self.migration_dir()
        before = {p.name: p.read_bytes() for p in d.iterdir()}
        check = (
            "import sys,pathlib;d=pathlib.Path(sys.argv[1]);sys.path.insert(0,sys.argv[2]);import run;"
            "s=(d/'atlas.sum').read_text();"
            "sys.exit(1 if 'x int)' in (d/'20260101000000_a.sql').read_text() and s==run.atlas_sum(d) and s!=sys.argv[3] else 0)"
        )
        m = self.mutant(file="db/migrations/20260101000000_a.sql", old="x int NOT NULL", new="x int")
        cmd = lambda *a: run.Command([sys.executable, "-c", check, str(d), str(Path(run.__file__).parent), before["atlas.sum"].decode()], self.repo, dict(os.environ))
        self.assertEqual(run.run_mutant(m, self.repo, cmd, run.run_test), run.KILLED)
        self.assertEqual({p.name: p.read_bytes() for p in d.iterdir()}, before)

    def test_non_migration_file_leaves_no_sum_side_effects(self):
        d = self.migration_dir()
        (d / "note.txt").write_text("alpha beta\n", encoding="utf-8")
        before = (d / "atlas.sum").read_bytes()
        check = "import sys;sys.exit(1 if open(sys.argv[1]).read()!=sys.argv[2] else 0)"
        cmd = lambda *a: run.Command([sys.executable, "-c", check, str(d / "atlas.sum"), before.decode()], self.repo, dict(os.environ))
        m = self.mutant(file="db/migrations/note.txt", old="beta", new="MUTATED")
        self.assertEqual(run.run_mutant(m, self.repo, cmd, run.run_test), run.SURVIVED)
        self.assertEqual((d / "atlas.sum").read_bytes(), before)


if __name__ == "__main__":
    unittest.main()
