#!/usr/bin/env python3
"""跑 mutation 清單：在工作樹裡把檔案的 old 換成 new，執行指定測試，測試要變紅。

清單格式與用法見 mutation/README.md。只用標準函式庫。
"""
from __future__ import annotations

import argparse
import base64
import contextlib
import hashlib
import json
import os
import re
import shlex
import signal
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
TIMEOUT_SECONDS = 15 * 60
FRAMEWORK_TIMEOUT_SECONDS = TIMEOUT_SECONDS + 5 * 60
REQUIRED = ("id", "stack", "file", "old", "new", "test", "why")
FLUTTER_PREFIX = "flutter:"
TS_PACKAGE_PREFIX = "ts-package:"
DART_PACKAGE_PREFIX = "dart-package:"
FIXED_STACKS = (
    "go-integration", "go", "website", "admin", "flutter:buyer-app", "flutter:seller-app",
    "ts-package:hawkr-api-ts", "dart-package:hawkr_api",
)
GO_MODULE_PREFIXES = ("go-integration:", "go:")

KILLED = "KILLED"
SURVIVED = "SURVIVED"
TIMEOUT = "TIMEOUT"
BASELINE_FAILED = "BASELINE-FAILED"
APPLY_FAILED = "APPLY-FAILED"
BUILD_FAILED = "BUILD-FAILED"
BUILD_ERROR = "BUILD-ERROR"
ERROR = "ERROR"


class ManifestError(Exception):
    pass


@dataclass(frozen=True)
class Mutant:
    id: str
    stack: str
    file: str
    old: str
    new: str
    test: str
    why: str
    source: str


@dataclass(frozen=True)
class Command:
    argv: list[str]
    cwd: Path
    env: dict[str, str]
    stack: str = ""


def go_stack(stack: str) -> tuple[bool, str] | None:
    """Go 的 stack 解成 (是否 integration, 模組目錄，相對 repo 根目錄)。
    `go`／`go-integration` 是 backend 單體；`go:<dir>`／`go-integration:<dir>` 是 <dir> 這個獨立模組。"""
    if stack in ("go", "go-integration"):
        return stack == "go-integration", "backend"
    for prefix in GO_MODULE_PREFIXES:
        if stack.startswith(prefix):
            return prefix == "go-integration:", stack[len(prefix):]
    return None


def valid_module_dir(d: str) -> bool:
    parts = d.split("/")
    return d != "" and not d.startswith("/") and all(x not in ("", ".", "..") for x in parts)


def valid_stack(stack: str) -> bool:
    if stack in FIXED_STACKS:
        return True
    g = go_stack(stack)
    return g is not None and valid_module_dir(g[1])


def package_dir(stack: str, repo: Path) -> Path:
    """`ts-package:<名>`、`dart-package:<名>` 是 packages/<名> 這個共用套件。"""
    return repo / "packages" / stack.split(":", 1)[1]


def inside(repo: Path, path: Path) -> bool:
    return path.resolve().is_relative_to(repo.resolve())


WEBSITE_BUILD_ERROR = re.compile(r"Transform failed|SyntaxError|Failed to parse source")
ADMIN_BUILD_ERROR = re.compile(r"TransformError|Transform failed|SyntaxError")
FLUTTER_BUILD_ERROR = re.compile(r"Compilation failed|^\S+:\d+:\d+: Error: ", re.MULTILINE)


def has_build_error(stack: str, output: str) -> bool:
    """測試指令失敗時，輸出是否帶有 mutant 本身壞掉的明確標記（轉換、語法或編譯錯誤）。
    沒有標記的失敗（含沒有測試摘要）一律當作測試擋下。"""
    if stack == "website" or stack.startswith(TS_PACKAGE_PREFIX):
        return WEBSITE_BUILD_ERROR.search(output) is not None
    if stack == "admin":
        return ADMIN_BUILD_ERROR.search(output) is not None
    if stack.startswith(FLUTTER_PREFIX) or stack.startswith(DART_PACKAGE_PREFIX):
        return FLUTTER_BUILD_ERROR.search(output) is not None
    return False


def load_manifest(path: Path) -> list[Mutant]:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as e:
        raise ManifestError(f"{path}: 讀不到或不是 JSON：{e}") from e
    items = data.get("mutants") if isinstance(data, dict) else None
    if not isinstance(items, list) or not items:
        raise ManifestError(f'{path}: 頂層必須是 {{"mutants": [...]}} 且不得為空')
    out: list[Mutant] = []
    seen: set[str] = set()
    for i, raw in enumerate(items):
        where = f"{path}: mutants[{i}]"
        if not isinstance(raw, dict):
            raise ManifestError(f"{where} 必須是物件")
        missing = [k for k in REQUIRED if k not in raw]
        if missing:
            raise ManifestError(f"{where} 缺欄位：{', '.join(missing)}")
        extra = [k for k in raw if k not in REQUIRED]
        if extra:
            raise ManifestError(f"{where} 有未知欄位：{', '.join(extra)}")
        for k in REQUIRED:
            if not isinstance(raw[k], str):
                raise ManifestError(f"{where}.{k} 必須是字串")
        for k in ("id", "file", "old", "test", "why"):
            if raw[k] == "":
                raise ManifestError(f"{where}.{k} 不得為空")
        if raw["old"] == raw["new"]:
            raise ManifestError(f"{where} old 與 new 相同")
        if not valid_stack(raw["stack"]):
            raise ManifestError(f"{where} 未知的 stack：{raw['stack']}")
        if raw["id"] in seen:
            raise ManifestError(f"{where} id 重複：{raw['id']}")
        seen.add(raw["id"])
        out.append(Mutant(source=str(path), **raw))
    return out


def split_selector(selector: str) -> tuple[str, str]:
    parts = selector.strip().split(None, 1)
    if not parts:
        raise ManifestError("test 選擇器不得為空")
    return parts[0], (parts[1] if len(parts) > 1 else "")


UNSAFE_SPECIFIER = re.compile(r"[*?\[\]{}~$`()<>|&;\\\s]")
WEBSITE_SHAPES = (["vitest", "run", "--project", "unit"], ["vitest", "run", "--project=unit"])
EXPECTED_SHAPE = {
    "admin": "node [--import <specifier>]... --test（specifier 不得以 - 開頭，不得含 glob、shell 字元或空白）",
    "website": "vitest run --project unit（或 --project=unit），前後不得有其他 token",
}


def matches_script_grammar(app: str, argv: list[str]) -> bool:
    """runner 要在 script 後面接自己的參數且不經 shell，所以 script 必須完全符合文法，不是只檢查幾個性質。"""
    if app == "website":
        return argv in WEBSITE_SHAPES
    if not argv or argv[0] != "node" or argv[-1:] != ["--test"]:
        return False
    middle = argv[1:-1]
    if len(middle) % 2:
        return False
    for flag, spec in zip(middle[::2], middle[1::2]):
        if flag != "--import" or not spec or spec.startswith("-") or UNSAFE_SPECIFIER.search(spec):
            return False
    return True


def package_script(repo: Path, app: str, name: str) -> list[str]:
    """`<app>/package.json` 的 script 拆成 argv；測試的載入方式只在那裡寫一次。"""
    path = repo / app / "package.json"
    try:
        script = json.loads(path.read_text(encoding="utf-8"))["scripts"][name]
    except (OSError, ValueError, KeyError, TypeError) as e:
        raise ManifestError(f"{path} 讀不到 script {name!r}：{e}") from e
    where = f"{path} 的 script {name!r}"
    if not isinstance(script, str):
        raise ManifestError(f"{where} 必須是字串")
    try:
        argv = shlex.split(script)
    except ValueError as e:
        raise ManifestError(f"{where} 無法拆成 argv：{e}") from e
    if not matches_script_grammar(app, argv):
        raise ManifestError(f"{where} 不符合預期形狀 {EXPECTED_SHAPE[app]}：{script}")
    return argv


def build_command(stack: str, selector: str, repo: Path = REPO) -> Command:
    head, rest = split_selector(selector)
    env = dict(os.environ)
    g = go_stack(stack)
    if g is not None:
        integration, moddir = g
        if not rest:
            raise ManifestError("go 選擇器必須是「<套件> <regex>」")
        argv = ["go", "test", "-count=1", "-vet=off", "-timeout", f"{FRAMEWORK_TIMEOUT_SECONDS}s"]
        if integration:
            argv += ["-tags", "integration"]
        argv += [f"./{head.strip('/')}/", "-run", rest]
        env["GOWORK"] = "off"
        return Command(argv, repo / moddir, env, stack)
    if stack == "admin":
        argv = package_script(repo, "admin", "test:file")
        if rest:
            argv += ["--test-name-pattern", rest]
        argv.append(head)
        return Command(argv, repo / "admin", env, stack)
    if stack == "website":
        argv = ["pnpm", "exec", *package_script(repo, "website", "test:file"), head]
        if rest:
            argv += [f"-t={rest}"]
        return Command(argv, repo / "website", env, stack)
    if stack.startswith(TS_PACKAGE_PREFIX):
        argv = ["pnpm", "exec", "vitest", "run", head]
        if rest:
            argv += [f"-t={rest}"]
        return Command(argv, package_dir(stack, repo), env, stack)
    if stack.startswith(DART_PACKAGE_PREFIX):
        argv = ["dart", "test", head]
        if rest:
            argv += ["--plain-name", rest]
        return Command(argv, package_dir(stack, repo), env, stack)
    app = stack[len(FLUTTER_PREFIX):]
    argv = ["bash", "scripts/ci-flutter-test.sh", "--no-pub", head]
    if rest:
        argv += ["--plain-name", rest]
    return Command(argv, repo / app, env, stack)


def build_check_command(stack: str, selector: str, file: str, repo: Path = REPO) -> Command | None:
    """變更後先確認還編譯得過；website 沒有便宜的逐 mutant 編譯，改看測試輸出（ran_no_tests）。"""
    env = dict(os.environ)
    g = go_stack(stack)
    if g is not None:
        integration, moddir = g
        head, _ = split_selector(selector)
        argv = ["go", "test", "-c", "-o", os.devnull]
        if integration:
            argv += ["-tags", "integration"]
        argv.append(f"./{head.strip('/')}/")
        env["GOWORK"] = "off"
        return Command(argv, repo / moddir, env, stack)
    if stack.startswith(DART_PACKAGE_PREFIX):
        pkg = package_dir(stack, repo)
        if not inside(pkg, repo / file):
            return None
        return Command(["dart", "analyze", "--no-fatal-warnings", str(repo / file)], pkg, env, stack)
    if stack.startswith(FLUTTER_PREFIX):
        app = stack[len(FLUTTER_PREFIX):]
        if not inside(repo / app, repo / file):
            return None
        return Command(["dart", "analyze", "--no-fatal-warnings", str(repo / file)], repo / app, env, stack)
    return None


def test_file(stack: str, selector: str, repo: Path = REPO) -> Path:
    head, _ = split_selector(selector)
    g = go_stack(stack)
    if g is not None:
        return repo / g[1] / head
    if stack == "website":
        return repo / "website" / head
    if stack == "admin":
        return repo / "admin" / head
    if stack.startswith((TS_PACKAGE_PREFIX, DART_PACKAGE_PREFIX)):
        return package_dir(stack, repo) / head
    return repo / stack[len(FLUTTER_PREFIX):] / head


def check_mutant(m: Mutant, repo: Path = REPO) -> str | None:
    """回傳問題描述；沒問題回 None（只讀，不執行測試）。"""
    path = repo / m.file
    if not inside(repo, path):
        return f"檔案在 repo 之外：{m.file}"
    if not path.is_file():
        return f"檔案不存在：{m.file}"
    count = path.read_text(encoding="utf-8").count(m.old)
    if count != 1:
        return f"old 在 {m.file} 出現 {count} 次（必須剛好 1 次）"
    g = go_stack(m.stack)
    if g is not None and ":" in m.stack:
        moddir = repo / g[1]
        if not inside(repo, moddir) or not (moddir / "go.mod").is_file():
            return f"Go 模組目錄不存在或沒有 go.mod：{g[1]}"
    target = test_file(m.stack, m.test, repo)
    if not inside(repo, target):
        return f"測試路徑在 repo 之外：{m.test.split()[0]}"
    if not target.exists():
        return f"測試路徑不存在：{target.relative_to(repo)}"
    return None


def run_test(cmd: Command) -> str:
    try:
        proc = subprocess.Popen(
            cmd.argv, cwd=cmd.cwd, env=cmd.env, start_new_session=True,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
        )
    except OSError as e:
        print(f"    無法執行 {cmd.argv[0]}：{e}", file=sys.stderr)
        return ERROR
    try:
        out, _ = proc.communicate(timeout=TIMEOUT_SECONDS)
    except subprocess.TimeoutExpired:
        with contextlib.suppress(ProcessLookupError):
            os.killpg(proc.pid, signal.SIGKILL)
        proc.communicate()
        return TIMEOUT
    if proc.returncode == 0:
        return "PASS"
    sys.stdout.write("".join("    | " + ln + "\n" for ln in out.splitlines()[-25:]))
    if has_build_error(cmd.stack, out):
        return BUILD_ERROR
    return "FAIL"


def atlas_sum(directory: Path) -> str:
    """Atlas 的 atlas.sum 內容：逐檔累加的 SHA-256，第一行是「檔名＋逐檔雜湊」的總雜湊。"""
    names = sorted(p.name for p in directory.iterdir() if p.is_file() and p.suffix == ".sql")
    running = hashlib.sha256()
    hashes = []
    for name in names:
        body = (directory / name).read_bytes()
        running.update(name.encode("utf-8"))
        running.update(body)
        hashes.append(base64.b64encode(running.copy().digest()).decode("ascii"))
    total = hashlib.sha256()
    for name, h in zip(names, hashes):
        total.update(name.encode("utf-8"))
        total.update(h.encode("ascii"))
    lines = [f"h1:{base64.b64encode(total.digest()).decode('ascii')}"]
    lines += [f"{name} h1:{h}" for name, h in zip(names, hashes)]
    return "\n".join(lines) + "\n"


def run_mutant(m: Mutant, repo: Path, builder, runner, build_builder=None) -> str:
    if not inside(repo, repo / m.file):
        print(f"    檔案在 repo 之外：{m.file}")
        return APPLY_FAILED
    cmd = builder(m.stack, m.test, repo)
    baseline = runner(cmd)
    if baseline != "PASS":
        return BASELINE_FAILED
    path = repo / m.file
    original = path.read_bytes()
    count = original.decode("utf-8").count(m.old)
    if count != 1:
        print(f"    old 出現 {count} 次（必須剛好 1 次）")
        return APPLY_FAILED
    mutated = original.decode("utf-8").replace(m.old, m.new, 1).encode("utf-8")
    sum_path = path.parent / "atlas.sum"
    sum_original = sum_path.read_bytes() if path.suffix == ".sql" and sum_path.is_file() else None
    try:
        path.write_bytes(mutated)
        if sum_original is not None:
            sum_path.write_text(atlas_sum(path.parent), encoding="utf-8")
        build = build_builder(m.stack, m.test, m.file, repo) if build_builder else None
        if build is not None:
            built = runner(build)
            if built in (TIMEOUT, ERROR):
                return built
            if built != "PASS":
                return BUILD_FAILED
        outcome = runner(cmd)
    finally:
        path.write_bytes(original)
        if sum_original is not None:
            sum_path.write_bytes(sum_original)
    if outcome == TIMEOUT:
        return TIMEOUT
    if outcome == BUILD_ERROR:
        return BUILD_FAILED
    return KILLED if outcome != "PASS" else SURVIVED


def select(manifests: list[str], stack: str | None) -> list[Mutant]:
    mutants: list[Mutant] = []
    for p in manifests:
        mutants += load_manifest(Path(p))
    if stack is not None:
        mutants = [m for m in mutants if m.stack == stack]
    return mutants


def summarize(results: list[tuple[Mutant, str]]) -> str:
    lines = ["| 結果 | id | 檔案 | 測試 |", "|---|---|---|---|"]
    for m, r in results:
        lines.append(f"| {r} | {m.id} | `{m.file}` | `{m.test}` |")
    killed = sum(1 for _, r in results if r == KILLED)
    lines.append("")
    lines.append(f"{killed}/{len(results)} KILLED")
    return "\n".join(lines)


def execute(mutants: list[Mutant], repo: Path = REPO, builder=build_command, runner=run_test,
            build_builder=build_check_command) -> int:
    results: list[tuple[Mutant, str]] = []
    for m in mutants:
        print(f"== {m.id} [{m.stack}] {m.file}", flush=True)
        result = run_mutant(m, repo, builder, runner, build_builder)
        print(f"{result}: {m.id}  ({m.why})", flush=True)
        results.append((m, result))
    summary = summarize(results)
    print("\n" + summary)
    out = os.environ.get("GITHUB_STEP_SUMMARY")
    if out:
        with open(out, "a", encoding="utf-8") as f:
            f.write(summary + "\n")
    return 0 if results and all(r == KILLED for _, r in results) else 1


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("manifests", nargs="+")
    ap.add_argument("--stack")
    ap.add_argument("--list-stacks", action="store_true", help="印出清單用到的 stack（JSON 陣列）")
    ap.add_argument("--validate", action="store_true", help="只檢查格式、old 恰好一次與測試路徑，不執行")
    args = ap.parse_args(argv)
    try:
        if args.list_stacks:
            stacks = sorted({m.stack for m in select(args.manifests, None)})
            print(json.dumps(stacks))
            return 0
        mutants = select(args.manifests, args.stack)
        if args.validate:
            bad = 0
            for m in mutants:
                problem = check_mutant(m, REPO)
                if problem is None and m.stack in EXPECTED_SHAPE:
                    try:
                        build_command(m.stack, m.test, REPO)
                    except ManifestError as e:
                        problem = str(e)
                print(f"{'FAIL' if problem else 'ok'}: {m.id}" + (f"  {problem}" if problem else ""))
                bad += bool(problem)
            return 1 if bad else 0
        if not mutants:
            print("沒有符合的 mutant", file=sys.stderr)
            return 1
        return execute(mutants)
    except ManifestError as e:
        print(f"清單錯誤：{e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
