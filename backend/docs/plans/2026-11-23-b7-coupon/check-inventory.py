#!/usr/bin/env python3
"""B7 inventory gate: no empty 去向 cell, every BD has a 處置 line.

Usage: check-inventory.py [path ...]
Without arguments it checks the four inventories next to this script.
Each path is checked by content: files with numbered "N. **" items (defects.md style)
for 處置, everything else for tables whose last header cell is 去向.
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULTS = ["interface-inventory.md", "client-inventory.md", "integration-tests.md", "defects.md"]
EXPECTED_BD = 34


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
            if c and c[-1] == "去向":
                in_dest = True
                continue
            if not in_dest:
                continue
            rows += 1
            if not c[-1]:
                errs.append(f"{path}:{n}: empty 去向 cell")
    return errs, rows


def check_defects(path):
    errs, seen, pending = [], set(), None
    with open(path, encoding="utf-8") as f:
        lines = f.read().split("\n")
    for n, line in enumerate(lines, 1):
        m = re.match(r"^(\d+)\. \*\*", line)
        if m:
            if pending is not None:
                errs.append(f"{path}:{pending[1]}: BD{pending[0]} has no 處置")
            pending = (int(m.group(1)), n)
            seen.add(int(m.group(1)))
            continue
        m = re.match(r"^\s+- 處置：(.*)$", line)
        if m and pending is not None:
            if not m.group(1).strip():
                errs.append(f"{path}:{n}: BD{pending[0]} 處置 is empty")
            pending = None
    if pending is not None:
        errs.append(f"{path}:{pending[1]}: BD{pending[0]} has no 處置")
    missing = set(range(1, EXPECTED_BD + 1)) - seen
    if missing:
        errs.append(f"{path}: missing BD numbers {sorted(missing)}")
    return errs, len(seen)


def main(argv):
    paths = argv or [os.path.join(HERE, p) for p in DEFAULTS]
    errs = []
    for p in paths:
        with open(p, encoding="utf-8") as f:
            is_defects = any(re.match(r"^\d+\. \*\*", ln) for ln in f)
        if is_defects:
            e, n = check_defects(p)
            print(f"{p}: {n} BD items")
        else:
            e, n = check_tables(p)
            print(f"{p}: {n} 去向 rows")
        errs += e
    for e in errs:
        print("FAIL", e)
    if errs:
        return 1
    print("OK")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
