#!/usr/bin/env python3
"""以 PR 為單位計算服務（之後是模組）之間的 co-change 耦合度。

只算 "Merge pull request #N" 的 merge commit：它的 first parent 是合併前的基底，
所以 `git diff <first parent> <merge>` 就是這個 PR 的淨變更。把 dev 同步進功能分支
的 merge 不算——那種 diff 是別人的變更，會製造假耦合（第一次盤點就踩過）。

耦合度 = 共同 PR 數 ÷ 兩邊 PR 數的平均（Tornhill 的 degree of coupling）。
一次碰超過 MAX_UNITS 個單位的 PR 視為橫掃（改名、升版），不代表業務耦合，排除。

用法：python3 cochange.py [--path-prefix backend] [--min-shared 5]
單體完成後，把 unit_of() 的規則改成 internal/modules/<name>/ 即可重跑同一份分析。
"""
import argparse
import collections
import itertools
import re
import subprocess

MAX_UNITS = 6


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True, check=True).stdout


def unit_of(path):
    m = re.match(r"backend/services/(?:java/)?([a-z-]+)-service/", path)
    if m:
        return m.group(1)
    m = re.match(r"backend/internal/modules/([a-z]+)/", path)
    return m.group(1) if m else None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--path-prefix", default="backend")
    ap.add_argument("--min-shared", type=int, default=5)
    args = ap.parse_args()
    repo = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()

    touch, pair, sizes, used = collections.Counter(), collections.Counter(), collections.Counter(), 0
    for line in git(repo, "log", "--merges", "--format=%H %P|%s", "--", args.path_prefix).splitlines():
        head, _, subject = line.partition("|")
        if not subject.startswith("Merge pull request #"):
            continue
        commit, *parents = head.split()
        if len(parents) < 2:
            continue
        units = {u for u in map(unit_of, git(repo, "diff", "--name-only", parents[0], commit).splitlines()) if u}
        sizes[min(len(units), MAX_UNITS + 1)] += 1
        if not units or len(units) > MAX_UNITS:
            continue
        used += 1
        touch.update(units)
        pair.update(itertools.combinations(sorted(units), 2))

    print(f"PRs used: {used}; units touched per PR: {dict(sorted(sizes.items()))}")
    print(f"{'pair':36} {'shared':>6} {'A':>5} {'B':>5} {'degree':>6}")
    rows = sorted(((n / ((touch[a] + touch[b]) / 2), n, a, b) for (a, b), n in pair.items()), reverse=True)
    for degree, n, a, b in rows:
        if n >= args.min_shared:
            print(f"{a + ' + ' + b:36} {n:6} {touch[a]:5} {touch[b]:5} {degree:6.2f}")


if __name__ == "__main__":
    main()
