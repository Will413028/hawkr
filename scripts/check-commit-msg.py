#!/usr/bin/env python3
"""檢查 commit message 是否符合 Conventional Commits。

本機由 pre-commit 在 commit 時呼叫（.pre-commit-config.yaml）；CI 以同一支腳本檢查推上來的
每個 commit（.github/workflows/commit-messages.yml），兩邊的規則因此不會分岔。

規則：
  - 第一行是 `<type>(<scope>)?!?: <subject>`，type 為
    feat、fix、docs、style、refactor、perf、test、build、ci、chore、revert
  - subject 不以句號結尾
  - merge commit，以及 `fixup!`、`squash!`、`amend!` 開頭的 commit（合併前會被改寫）不檢查

用法：
  python3 scripts/check-commit-msg.py <訊息檔>          # commit-msg hook
  python3 scripts/check-commit-msg.py --range A..B     # 檢查 A..B 之間每個非 merge commit
"""
import re
import subprocess
import sys

TYPES = "feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert"
PATTERN = re.compile(rf"^({TYPES})(\([a-zA-Z0-9_./-]+\))?!?: .+")
EXEMPT = ("fixup! ", "squash! ", "amend! ")


def subject_line(message: str) -> str:
    """回傳第一個非空、非註解的行（編輯器寫的訊息可能還留著 # 開頭的說明）。"""
    for line in message.splitlines():
        if line.strip() and not line.startswith("#"):
            return line.rstrip()
    return ""


def problem(first: str) -> str:
    if first.startswith(EXEMPT) or first.startswith("Merge "):
        return ""
    if not PATTERN.match(first):
        return f"缺少合法的 type 或格式錯誤（格式：<type>(<scope>)?: <subject>，type：{TYPES.replace('|', '、')}）"
    subject = first.split(": ", 1)[1]
    if subject.endswith(".") or subject.endswith("。"):
        return "subject 結尾不可加句號"
    return ""


def check_range(rev_range: str) -> int:
    out = subprocess.run(
        ["git", "log", "--no-merges", "--format=%H%x00%B%x1e", rev_range],
        capture_output=True, text=True, check=True,
    ).stdout
    bad = 0
    for record in filter(str.strip, out.split("\x1e")):
        sha, _, body = record.strip("\n").partition("\x00")
        first = subject_line(body)
        if p := problem(first):
            print(f"✘ {sha[:9]} {first}\n    {p}")
            bad += 1
    if bad:
        print(f"\n{bad} 個 commit 的訊息不符合 Conventional Commits。")
        return 1
    print(f"通過：{rev_range} 的 commit 訊息都符合 Conventional Commits。")
    return 0


def main(argv: list) -> int:
    if len(argv) == 3 and argv[1] == "--range":
        return check_range(argv[2])
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    with open(argv[1], encoding="utf-8") as f:
        first = subject_line(f.read())
    if p := problem(first):
        print(f"commit message 不符合 Conventional Commits：{p}\n你的訊息：{first}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
