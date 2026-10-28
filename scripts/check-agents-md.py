#!/usr/bin/env python3
"""檢查每個 AGENTS.md 的大小與單行長度（ADR backend/docs/adr/2026-10-28-agents-md-layering.md）。

AGENTS.md 是 coding agent 的指令檔，碰到那個目錄就整份載入，所以每一層都有上限：

  - 根目錄 AGENTS.md：12,000 bytes
  - 子專案（repo 第一層目錄，例如 backend/、website/）：10,000 bytes
  - 其他（backend/internal/modules/*、packages/* 與更深的目錄）：6,000 bytes
  - 每一行：500 字元

`next dev` 自己寫入的 nextjs-agent-rules 區塊不計入。超限的檔要列在 EXCEPTIONS，並寫明由
哪一步清掉；例外只能縮小：列在裡面卻已經不超限、或檔案已不存在，都算失敗。

用法：
  python3 scripts/check-agents-md.py
"""
import re
import subprocess
import sys

ROOT_LIMIT = 12_000
SUBPROJECT_LIMIT = 10_000
NESTED_LIMIT = 6_000
LINE_LIMIT = 500

NEXTJS_BLOCK = re.compile(
    r"<!-- BEGIN:nextjs-agent-rules -->.*?<!-- END:nextjs-agent-rules -->\n?", re.DOTALL
)

# 超限的檔與清掉它的步驟（backend/docs/plans/2026-10-28-agents-md-layering.md）。
EXCEPTIONS = {
    "backend/internal/modules/buyer/AGENTS.md": "步驟 4",
    "backend/internal/modules/seller/AGENTS.md": "步驟 6（之後由 seller 拆模組計畫清掉，D4）",
}


def limit_for(path: str) -> int:
    depth = path.count("/")
    if depth == 0:
        return ROOT_LIMIT
    if depth == 1:
        return SUBPROJECT_LIMIT
    return NESTED_LIMIT


def violations(path: str, text: str) -> list[str]:
    """回傳這個檔超過上限的地方；空清單代表沒有超限。"""
    body = NEXTJS_BLOCK.sub("", text)
    found = []
    size = len(body.encode("utf-8"))
    limit = limit_for(path)
    if size > limit:
        found.append(f"{path}: {size} bytes，上限 {limit}")
    for number, line in enumerate(body.splitlines(), start=1):
        if len(line) > LINE_LIMIT:
            found.append(f"{path}:{number}: 一行 {len(line)} 字元，上限 {LINE_LIMIT}")
    return found


def check(files: dict[str, str], exceptions: dict[str, str]) -> list[str]:
    """files 是路徑對內容；回傳要讓檢查失敗的訊息。"""
    errors = []
    for path, text in sorted(files.items()):
        found = violations(path, text)
        if found and path not in exceptions:
            errors.extend(found)
        if not found and path in exceptions:
            errors.append(f"{path}: 已經不超限，從 EXCEPTIONS 拿掉")
    for path in sorted(set(exceptions) - set(files)):
        errors.append(f"{path}: 檔案不存在，從 EXCEPTIONS 拿掉")
    return errors


def tracked_agents_files() -> dict[str, str]:
    paths = subprocess.run(
        ["git", "ls-files", "-z", "--", "AGENTS.md", "*/AGENTS.md"],
        check=True, capture_output=True, text=True,
    ).stdout.split("\0")
    files = {}
    for path in filter(None, paths):
        with open(path, encoding="utf-8") as f:
            files[path] = f.read()
    return files


def main() -> int:
    errors = check(tracked_agents_files(), EXCEPTIONS)
    for error in errors:
        print(error, file=sys.stderr)
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
