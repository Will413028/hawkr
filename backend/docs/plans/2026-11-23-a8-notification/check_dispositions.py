#!/usr/bin/env python3
"""A8-0 一次性檢查：inventory.md 每張表的「去向」欄與 event-adjudication.md 裁決表的必填欄不得留白。"""
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent


def cells(line):
    return [c.strip() for c in line.strip().strip("|").split("|")]


def tables(path):
    lines = path.read_text(encoding="utf-8").splitlines()
    i = 0
    while i < len(lines):
        if lines[i].lstrip().startswith("|") and i + 1 < len(lines) and set(lines[i + 1].replace("|", "").strip()) <= set("-: ") and "-" in lines[i + 1]:
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


def check(path, columns):
    errors = []
    for header, rows in tables(path):
        wanted = [header.index(c) for c in columns if c in header]
        for lineno, row in rows:
            for idx in wanted:
                if idx >= len(row) or not row[idx]:
                    errors.append(f"{path.name}:{lineno}: 欄位「{header[idx]}」留白")
    return errors


def main():
    errors = check(HERE / "inventory.md", ["去向"])
    errors += check(HERE / "event-adjudication.md", ["type 或「不做」", "audience", "D4", "呈現", "聚合鍵", "reason / evidence"])
    for e in errors:
        print(e)
    print(f"{len(errors)} 個空格")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
