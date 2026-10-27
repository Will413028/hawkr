"""Offline regression checks for the CI-only Flutter temp wrapper."""

import json
import os
from pathlib import Path
import signal
import shlex
import subprocess
import tempfile
import time
import unittest


WRAPPER = Path(__file__).resolve().with_name("ci-flutter-test.sh")


class FlutterTestTempTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="flutter wrapper # ")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.step = self.root / "actions-step.sh"
        self.step.write_text(
            "set -euo pipefail\n"
            "printf 'Preflight completed\\n'\n"
            f"exec bash {shlex.quote(str(WRAPPER))} \"$@\"\n"
        )
        self.runner_temp = self.root / "runner temp"
        self.runner_temp.mkdir()
        self.sibling = self.runner_temp / "another-job"
        self.sibling.mkdir()
        (self.sibling / "sentinel").write_text("preserve")
        binary_dir = self.root / "bin"
        binary_dir.mkdir()
        binary = binary_dir / "flutter"
        binary.write_text(
            "#!/usr/bin/env python3\n"
            "import json, os, pathlib, signal, sys, time\n"
            "signal.signal(signal.SIGINT, signal.SIG_DFL)\n"
            "temp = pathlib.Path(os.environ['TMPDIR'])\n"
            "assert temp.is_dir()\n"
            "(temp / 'owned-child').mkdir()\n"
            "output = pathlib.Path(os.environ['STUB_OUTPUT'])\n"
            "staging = output.with_suffix('.tmp')\n"
            "staging.write_text(json.dumps({"
            "'args': sys.argv[1:], 'temp': str(temp), 'pid': os.getpid()}))\n"
            "staging.replace(output)\n"
            "print('Flutter stdout remains visible', flush=True)\n"
            "print('Flutter stderr remains visible', file=sys.stderr, flush=True)\n"
            "if os.environ.get('STUB_RELEASE'):\n"
            "    deadline = time.monotonic() + 10\n"
            "    while not pathlib.Path(os.environ['STUB_RELEASE']).exists():\n"
            "        if time.monotonic() > deadline: sys.exit(99)\n"
            "        time.sleep(0.01)\n"
            "sys.exit(int(os.environ.get('STUB_EXIT', '0')))\n"
        )
        binary.chmod(0o755)
        self.env = {
            **os.environ,
            "PATH": str(binary_dir) + os.pathsep + os.environ["PATH"],
            "RUNNER_TEMP": str(self.runner_temp),
            "TMPDIR": str(self.sibling),
            "STUB_OUTPUT": str(self.root / "result.json"),
        }

    def run_wrapper(self, *args, **overrides):
        return subprocess.run(
            ["bash", str(self.step), *args],
            env={**self.env, **overrides},
            capture_output=True,
            text=True,
            timeout=15,
        )

    def assert_kept_for_runner_cleanup(self, result_path):
        data = json.loads(result_path.read_text())
        temporary = Path(data["temp"])
        self.assertEqual(temporary.parent, self.runner_temp)
        self.assertTrue(temporary.name.startswith("flutter-test."))
        self.assertTrue((temporary / "owned-child").is_dir())
        self.assertEqual((self.sibling / "sentinel").read_text(), "preserve")
        return data

    def test_success_preserves_arguments_and_leaves_job_temp_for_runner_cleanup(self):
        args = ["--no-pub", "--coverage-path", "coverage/with space.info"]
        result = self.run_wrapper(*args)
        self.assertEqual(result.returncode, 0, result.stderr)
        data = self.assert_kept_for_runner_cleanup(Path(self.env["STUB_OUTPUT"]))
        self.assertEqual(data["args"], ["test", *args])
        self.assertIn(data["temp"], result.stdout)
        self.assertIn("Flutter stdout remains visible", result.stdout)
        self.assertIn("Flutter stderr remains visible", result.stderr)

    def test_flutter_failure_keeps_exit_code_and_leaves_siblings_intact(self):
        result = self.run_wrapper("--no-pub", STUB_EXIT="17")
        self.assertEqual(result.returncode, 17)
        self.assert_kept_for_runner_cleanup(Path(self.env["STUB_OUTPUT"]))

    def test_missing_or_relative_runner_temp_fails_before_flutter(self):
        for value in ("", "relative", str(self.root / "absent")):
            with self.subTest(value=value):
                result = self.run_wrapper(RUNNER_TEMP=value)
                self.assertEqual(result.returncode, 2)
                self.assertIn("existing absolute RUNNER_TEMP", result.stderr)
                self.assertFalse(Path(self.env["STUB_OUTPUT"]).exists())

    def test_concurrent_invocations_have_separate_live_directories(self):
        release = self.root / "release"
        outputs = [self.root / f"result-{index}.json" for index in range(2)]
        processes = []
        try:
            for output in outputs:
                processes.append(subprocess.Popen(
                    ["bash", str(self.step), "--no-pub"],
                    env={**self.env, "STUB_OUTPUT": str(output), "STUB_RELEASE": str(release)},
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                ))
            deadline = time.monotonic() + 8
            while not all(output.exists() for output in outputs):
                self.assertLess(time.monotonic(), deadline, "Flutter stubs did not start")
                time.sleep(0.01)
            directories = [Path(json.loads(output.read_text())["temp"]) for output in outputs]
            self.assertNotEqual(*directories)
            self.assertTrue(all(directory.is_dir() for directory in directories))
        finally:
            release.touch()
            for process in processes:
                stdout, stderr = process.communicate(timeout=15)
                self.assertEqual(process.returncode, 0, stdout + stderr)
        for output in outputs:
            self.assert_kept_for_runner_cleanup(output)

    def test_pid_and_group_cancellation_reach_flutter_through_the_actions_step_shell(self):
        for target, cancellation in (
            (target, cancellation)
            for target in ("pid", "group")
            for cancellation in (signal.SIGINT, signal.SIGTERM)
        ):
            with self.subTest(target=target, signal=cancellation):
                output = self.root / f"signal-{target}-{cancellation}.json"
                process = subprocess.Popen(
                    ["bash", str(self.step)],
                    env={**self.env, "STUB_OUTPUT": str(output), "STUB_RELEASE": str(self.root / "never-release")},
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    start_new_session=True,
                )
                try:
                    deadline = time.monotonic() + 8
                    while not output.exists():
                        self.assertLess(time.monotonic(), deadline, "Flutter stub did not start")
                        time.sleep(0.01)
                    data = json.loads(output.read_text())
                    self.assertEqual(data["pid"], process.pid, "Step shell and wrapper must both exec in place")
                    send_signal = os.kill if target == "pid" else os.killpg
                    send_signal(process.pid, cancellation)
                    stdout, stderr = process.communicate(timeout=15)
                    self.assertEqual(process.returncode, -cancellation, stdout + stderr)
                    self.assert_kept_for_runner_cleanup(output)
                finally:
                    if process.poll() is None:
                        os.killpg(process.pid, signal.SIGKILL)
                        process.communicate(timeout=15)


if __name__ == "__main__":
    unittest.main()
