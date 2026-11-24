#!/usr/bin/env python3
"""B4 inventory gate: no empty 去向／對照 cell, every CD has a 處置 line.

Usage: check-inventory.py [path ...]
Without arguments it checks the four inventories next to this script.
Files with "CDn. **" items (defects.md style) are checked for a non-empty
"   - 處置：" line under each item; everything else for tables whose last
header cell is 去向 or 對照 (integration-tests.md).
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULTS = ["interface-inventory.md", "client-inventory.md", "integration-tests.md", "defects.md"]
DEST_HEADERS = {"去向", "對照"}
EXPECTED_CD = 11
ITEM = re.compile(r"^CD(\d+)\. \*\*")


def cells(line):
    s = line.strip()
    if s.startswith("|"):
        s = s[1:]
    if s.endswith("|"):
        s = s[:-1]
    return [c.strip() for c in s.split("|")]


def check_tables(path):
    errs, rows, in_dest = [], 0, False
    with open(path, encoding="utf-8") as f:
        for n, line in enumerate(f, 1):
            if not line.startswith("|"):
                in_dest = False
                continue
            if re.match(r"^\|[\s:|-]+\|\s*$", line):
                continue
            c = cells(line)
            if c and c[-1] in DEST_HEADERS:
                in_dest = True
                continue
            if not in_dest:
                continue
            rows += 1
            if not c[-1]:
                errs.append(f"{path}:{n}: empty {'/'.join(sorted(DEST_HEADERS))} cell")
    return errs, rows


def check_defects(path):
    errs, seen, pending = [], set(), None
    with open(path, encoding="utf-8") as f:
        lines = f.read().split("\n")
    for n, line in enumerate(lines, 1):
        m = ITEM.match(line)
        if m:
            if pending is not None:
                errs.append(f"{path}:{pending[1]}: CD{pending[0]} has no 處置")
            pending = (int(m.group(1)), n)
            seen.add(int(m.group(1)))
            continue
        m = re.match(r"^\s+- 處置：(.*)$", line)
        if m and pending is not None:
            if not m.group(1).strip():
                errs.append(f"{path}:{n}: CD{pending[0]} 處置 is empty")
            pending = None
    if pending is not None:
        errs.append(f"{path}:{pending[1]}: CD{pending[0]} has no 處置")
    missing = set(range(1, EXPECTED_CD + 1)) - seen
    if missing:
        errs.append(f"{path}: missing CD numbers {sorted(missing)}")
    return errs, len(seen)


def main(argv):
    paths = argv or [os.path.join(HERE, p) for p in DEFAULTS]
    errs = []
    for p in paths:
        with open(p, encoding="utf-8") as f:
            is_defects = any(ITEM.match(ln) for ln in f)
        if is_defects:
            e, n = check_defects(p)
            print(f"{p}: {n} CD items")
        else:
            e, n = check_tables(p)
            print(f"{p}: {n} 去向/對照 rows")
        errs += e
    for e in errs:
        print("FAIL", e)
    if errs:
        print(f"{len(errs)} gaps")
        return 1
    print("OK")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
