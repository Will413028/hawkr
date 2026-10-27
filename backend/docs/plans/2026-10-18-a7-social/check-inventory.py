"""A7 盤點與計畫的一次性檢查：去向、處置欄不得留空，盤點的每個缺陷都在計畫裡有處置。

在 backend/ 執行：python3 -I docs/plans/2026-10-18-a7-social/check-inventory.py
有問題時逐項列出並回 1。
"""

import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
PLAN = HERE.parent / "2026-10-18-a7-social.md"
INVENTORIES = ["domain-inventory.md", "interface-inventory.md", "client-inventory.md"]
REQUIRED_COLUMNS = ("去向", "處置")


def cells(line):
    return [c.strip() for c in line.strip().strip("|").split("|")]


def tables(text):
    """依序回傳 (表頭, [(行號, 列)])。"""
    lines = text.splitlines()
    i = 0
    while i < len(lines):
        if lines[i].lstrip().startswith("|") and i + 1 < len(lines) and re.match(r"^\s*\|[\s:|-]+\|\s*$", lines[i + 1]):
            header = cells(lines[i])
            rows = []
            j = i + 2
            while j < len(lines) and lines[j].lstrip().startswith("|"):
                rows.append((j + 1, cells(lines[j])))
                j += 1
            yield header, rows
            i = j
        else:
            i += 1


def empty_required_cells(path):
    problems = []
    for header, rows in tables(path.read_text(encoding="utf-8")):
        idx = [k for k, h in enumerate(header) if any(r in h for r in REQUIRED_COLUMNS)]
        for lineno, row in rows:
            for k in idx:
                if k >= len(row) or row[k] in ("", "—", "-"):
                    problems.append(f"{path.name}:{lineno} 欄「{header[k]}」是空的")
    return problems


def missing_defects():
    inv = (HERE / "domain-inventory.md").read_text(encoding="utf-8")
    plan = PLAN.read_text(encoding="utf-8")
    found = sorted(set(re.findall(r"^\| (SD\d+) \|", inv, re.M)), key=lambda s: int(s[2:]))
    in_plan = set(re.findall(r"^\| (SD\d+) \|", plan, re.M))
    return [f"{d} 在 domain-inventory.md 有，計畫的「既有缺陷」表沒有" for d in found if d not in in_plan]


def main():
    problems = []
    for name in INVENTORIES:
        problems += empty_required_cells(HERE / name)
    problems += empty_required_cells(PLAN)
    problems += missing_defects()
    for p in problems:
        print(p)
    if problems:
        return 1
    print("ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
