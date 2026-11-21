#!/usr/bin/env python3
"""跑 mutation 清單：在工作樹裡把檔案的 old 換成 new，執行指定測試，測試要變紅。

清單格式與用法見 mutation/README.md。只用標準函式庫。
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
TIMEOUT_SECONDS = 15 * 60
REQUIRED = ("id", "stack", "file", "old", "new", "test", "why")
FLUTTER_PREFIX = "flutter:"
FIXED_STACKS = ("go-integration", "go", "website")

KILLED = "KILLED"
SURVIVED = "SURVIVED"
TIMEOUT = "TIMEOUT"
BASELINE_FAILED = "BASELINE-FAILED"
APPLY_FAILED = "APPLY-FAILED"


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


def valid_stack(stack: str) -> bool:
    if stack in FIXED_STACKS:
        return True
    return stack.startswith(FLUTTER_PREFIX) and len(stack) > len(FLUTTER_PREFIX)


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


def build_command(stack: str, selector: str, repo: Path = REPO) -> Command:
    head, rest = split_selector(selector)
    env = dict(os.environ)
    if stack in ("go-integration", "go"):
        if not rest:
            raise ManifestError("go 選擇器必須是「<套件> <regex>」")
        argv = ["go", "test", "-count=1"]
        if stack == "go-integration":
            argv += ["-tags", "integration"]
        argv += [f"./{head.strip('/')}/", "-run", rest]
        env["GOWORK"] = "off"
        return Command(argv, repo / "backend", env)
    if stack == "website":
        argv = ["pnpm", "exec", "vitest", "run", "--project", "unit", head]
        if rest:
            argv += ["-t", rest]
        return Command(argv, repo / "website", env)
    app = stack[len(FLUTTER_PREFIX):]
    argv = ["bash", "scripts/ci-flutter-test.sh", "--no-pub", head]
    if rest:
        argv += ["--plain-name", rest]
    return Command(argv, repo / app, env)


def test_file(stack: str, selector: str, repo: Path = REPO) -> Path:
    head, _ = split_selector(selector)
    if stack in ("go-integration", "go"):
        return repo / "backend" / head
    if stack == "website":
        return repo / "website" / head
    return repo / stack[len(FLUTTER_PREFIX):] / head


def check_mutant(m: Mutant, repo: Path = REPO) -> str | None:
    """回傳問題描述；沒問題回 None（只讀，不執行測試）。"""
    path = repo / m.file
    if not path.is_file():
        return f"檔案不存在：{m.file}"
    count = path.read_text(encoding="utf-8").count(m.old)
    if count != 1:
        return f"old 在 {m.file} 出現 {count} 次（必須剛好 1 次）"
    target = test_file(m.stack, m.test, repo)
    if not target.exists():
        return f"測試路徑不存在：{target.relative_to(repo)}"
    return None


def run_test(cmd: Command) -> str:
    try:
        proc = subprocess.run(
            cmd.argv, cwd=cmd.cwd, env=cmd.env, timeout=TIMEOUT_SECONDS,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
        )
    except subprocess.TimeoutExpired:
        return TIMEOUT
    except OSError as e:
        print(f"    無法執行 {cmd.argv[0]}：{e}", file=sys.stderr)
        return "ERROR"
    if proc.returncode == 0:
        return "PASS"
    sys.stdout.write("".join("    | " + ln + "\n" for ln in proc.stdout.splitlines()[-25:]))
    return "FAIL"


def run_mutant(m: Mutant, repo: Path, builder, runner) -> str:
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
    try:
        path.write_bytes(original.decode("utf-8").replace(m.old, m.new, 1).encode("utf-8"))
        outcome = runner(cmd)
    finally:
        path.write_bytes(original)
    if outcome == TIMEOUT:
        return TIMEOUT
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


def execute(mutants: list[Mutant], repo: Path = REPO, builder=build_command, runner=run_test) -> int:
    results: list[tuple[Mutant, str]] = []
    for m in mutants:
        print(f"== {m.id} [{m.stack}] {m.file}", flush=True)
        result = run_mutant(m, repo, builder, runner)
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
                problem = check_mutant(m)
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
