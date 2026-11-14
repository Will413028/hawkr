#!/usr/bin/env python3
"""One-off acceptance check for plan step 8c (payment onboarding, Stripe).

Run from the repository root:

    python3 backend/docs/plans/2026-10-29-payment-onboarding-stripe/check_inventory.py [--inventory <path>]

(a) every destination ("去向") in payuni-inventory.md is filled and not TBD;
(b) every `git grep -n -i payuni -- backend/internal backend/cmd api/hawkr` hit
    matches an entry of ALLOWLIST below.

Exit 0 when both pass, otherwise print each violation and exit 1.
"""
import argparse
import fnmatch
import re
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
GREP_PATHS = ["backend/internal", "backend/cmd", "api/hawkr"]
EXPECTED_GROUP_ROWS = 10  # G1-G10, one "| 去向 |" row each
EXPECTED_G11_ROWS = 7

# (a) ---------------------------------------------------------------------


def is_empty_dest(value):
    v = value.strip().strip("`*").strip()
    return v == "" or re.match(r"TBD(?![A-Za-z0-9_])", v, re.I) is not None


def check_inventory(path):
    errors = []
    lines = path.read_text(encoding="utf-8").splitlines()
    group_rows = 0
    g11_rows = 0
    in_g11 = False
    g11_header_seen = False
    for no, line in enumerate(lines, 1):
        if line.startswith("### "):
            in_g11 = line.startswith("### G11")
            g11_header_seen = False
            continue
        if line.startswith("## "):
            in_g11 = False
            continue
        if not line.startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if in_g11:
            if not g11_header_seen:
                g11_header_seen = True
                if cells[-1] != "去向":
                    errors.append(f"{path.name}:{no}: G11 table last column is not 去向")
                continue
            if set("".join(cells)) <= set("-: "):
                continue
            g11_rows += 1
            if is_empty_dest(cells[-1]):
                errors.append(f"{path.name}:{no}: G11 去向 empty or TBD: {cells[0]}")
        elif cells and cells[0] == "去向" and len(cells) >= 2:
            group_rows += 1
            if is_empty_dest(cells[1]):
                errors.append(f"{path.name}:{no}: group 去向 empty or TBD")
    if group_rows != EXPECTED_GROUP_ROWS:
        errors.append(f"{path.name}: found {group_rows} group 去向 rows, expected {EXPECTED_GROUP_ROWS}")
    if g11_rows != EXPECTED_G11_ROWS:
        errors.append(f"{path.name}: found {g11_rows} G11 rows, expected {EXPECTED_G11_ROWS}")
    return errors


# (b) ---------------------------------------------------------------------


class Ctx:
    def __init__(self, root):
        self.root = root


# Each entry: (id, path pattern, kind "glob"|"regex", line regex or None / callable, source).
# A callable gets (ctx, path, lineno, text) and decides on its own; path must still match.
ALLOWLIST = [
    (1, "backend/internal/legacy/gen/**", "glob", None,
     "field names generated from the Java contract; deleted in B2"),
    (2, "backend/internal/legacy/sellerserver/**", "glob", re.compile(r"payuni_mer_id|PayuniMerId"),
     "use and description of the Java contract field payuni_mer_id"),
    (3, r"backend/internal/modules/[^/]+/db/migrations/[^/]+\.sql", "regex", None,
     "applied migrations and the step-8 drop migrations"),
    (3, "backend/internal/modules/*/db/migrations/atlas.sum", "glob", None,
     "atlas.sum hashes the migration file names"),
    (5, r"backend/internal/modules/(seller|discovery|buyer|content)/CARRYOVER\.md", "regex", None,
     "carry-over records and rows pointing at phase B"),
    (6, "backend/internal/platform/codemeta/**", "glob", None,
     "frozen generated codemeta files"),
    (8, "backend/internal/modules/seller/migration_integration_test.go", "glob", None,
     "tests of the applied migrations"),
    (9, "*_test.go", "glob", re.compile(r'"PAYUNI_TERMS"'),
     "test cases that prove the retired agreement kind is rejected, by literal value"),
]


def path_matches(pattern, kind, path):
    if kind == "regex":
        return re.fullmatch(pattern, path) is not None
    if pattern == "**":
        return True
    # fnmatch's * crosses "/" ; "**" and "*" are both fine for our patterns
    if pattern.endswith("/**"):
        return path.startswith(pattern[:-2])
    return fnmatch.fnmatchcase(path, pattern) or (
        "*" in pattern and "/" not in pattern and fnmatch.fnmatchcase(Path(path).name, pattern)
    )


def allowed(ctx, path, no, text):
    for ident, pattern, kind, line_rule, _src in ALLOWLIST:
        if not path_matches(pattern, kind, path):
            continue
        if line_rule is None:
            return ident
        if callable(line_rule):
            if line_rule(ctx, path, no, text):
                return ident
        elif line_rule.search(text):
            return ident
    return None


def check_grep(root):
    out = subprocess.run(
        ["git", "-C", str(root), "-c", "core.quotepath=false", "grep", "-n", "-i", "payuni", "--", *GREP_PATHS],
        capture_output=True, text=True, check=False)
    if out.returncode not in (0, 1):
        return [f"git grep failed: {out.stderr.strip()}"]
    ctx = Ctx(root)
    errors = []
    for raw in out.stdout.splitlines():
        m = re.match(r"([^:]+):(\d+):(.*)", raw)
        if not m:
            errors.append(f"unparsable grep line: {raw[:200]}")
            continue
        path, no, text = m.group(1), int(m.group(2)), m.group(3)
        if allowed(ctx, path, no, text) is None:
            errors.append(f"not in allowlist: {path}:{no}:{text.strip()[:160]}")
    return errors


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--inventory", default=str(HERE / "payuni-inventory.md"))
    args = ap.parse_args()
    root = Path(subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True,
                               text=True, check=True).stdout.strip())
    errors = check_inventory(Path(args.inventory)) + check_grep(root)
    for e in errors:
        print(e)
    if errors:
        print(f"FAIL: {len(errors)} violation(s)")
        return 1
    print("OK: inventory destinations filled; every payuni hit is on the allowlist")
    return 0


if __name__ == "__main__":
    sys.exit(main())
