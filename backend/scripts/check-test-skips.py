#!/usr/bin/env python3
"""測試被 skip 必須是**宣告過的決定**，不是環境剛好不符的意外。

背景（2026-07-31）：gateway/internal/middleware 有 14 支測試因為本機與 CI 的 6379
帶密碼而長期 skip，其中 3 支若真跑會直接 FAIL——skip 把失敗也一起蓋掉了。同一個
root cause 在 router（6 支，一跑就 panic 或 503）、pkg/redis（9 支）、crm（19 支）、
notification（2 支）陸續被翻出來。

所以規則不是「不准 skip」，是**不准未宣告的 skip**：每一支會被跳過的測試都要登記在
`scripts/expected-skips.json` 並附理由，於是「這支不跑」變成一個會出現在 PR diff、
需要有人點頭的決定。

雙向檢查（缺一都會讓清單腐化成墳場）：
  - 出現未登記的 skip → 紅（新的洞）
  - 登記了卻沒發生的 skip → 紅（測試已修好或已刪除，清單該跟著清）

用法：
    go test -json ./... | python3 scripts/check-test-skips.py --module <模組相對路徑>
可重複對多個模組執行，最後一次帶 --final 做「登記了卻沒發生」的反向檢查。
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ALLOWLIST = ROOT / "scripts" / "expected-skips.json"
STATE = Path("/tmp/.check-test-skips-seen.json")


def load_allowlist() -> dict[str, str]:
    if not ALLOWLIST.is_file():
        return {}
    raw = json.loads(ALLOWLIST.read_text(encoding="utf-8"))
    # {"<package>::<Test>": "理由"}
    return {k: v for k, v in raw.items() if not k.startswith("//")}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--module", default="", help="模組相對路徑，只用於訊息")
    ap.add_argument(
        "--final",
        action="store_true",
        help="所有模組跑完後執行，檢查清單中有沒有已經不再發生的條目",
    )
    ap.add_argument("--reset", action="store_true", help="清掉跨模組累積狀態")
    args = ap.parse_args()

    if args.reset and STATE.exists():
        STATE.unlink()

    allow = load_allowlist()
    seen: set[str] = set()
    # 也記錄「這一輪實際跑過哪些 package」。反向檢查只能在跑過的 package 上做——
    # PR gate 只跑有變更的模組，否則沒被跑到的模組裡已登記的 skip 會被誤判成墳場。
    packages: set[str] = set()
    if STATE.exists():
        state = json.loads(STATE.read_text(encoding="utf-8"))
        if isinstance(state, dict):
            seen = set(state.get("seen", []))
            packages = set(state.get("packages", []))
        else:  # 舊格式（純 list），相容讀取
            seen = set(state)

    undeclared: list[tuple[str, str]] = []
    if not sys.stdin.isatty():
        for line in sys.stdin:
            line = line.strip()
            if not line or not line.startswith("{"):
                continue
            try:
                ev = json.loads(line)
            except json.JSONDecodeError:
                continue
            # 原樣透傳 go test 的輸出，讓這支腳本是 `go test` 的 drop-in 替代——
            # 否則 CI 為了偵測 skip 就得放棄可讀的測試輸出，或是把測試跑兩次。
            if ev.get("Action") == "output":
                sys.stdout.write(ev.get("Output", ""))
            if pkg := ev.get("Package"):
                packages.add(pkg)
            # package 層級的 skip（沒有 Test 欄位）＝「這個 package 沒有測試檔」，
            # 不是測試被跳過，不能算進來。混在一起會讓數字虛胖 20 倍。
            if ev.get("Action") != "skip" or "Test" not in ev:
                continue
            key = f"{ev.get('Package', '?')}::{ev['Test']}"
            seen.add(key)
            if key not in allow:
                undeclared.append((key, args.module))

    STATE.write_text(
        json.dumps({"seen": sorted(seen), "packages": sorted(packages)}),
        encoding="utf-8",
    )

    failed = False
    if undeclared:
        failed = True
        print("\n未登記的 skip（測試被跳過，但沒有人宣告過這個決定）：\n", file=sys.stderr)
        for key, mod in undeclared:
            print(f"  ✘ {key}" + (f"   [{mod}]" if mod else ""), file=sys.stderr)
        print(
            "\n先問：它為什麼跳過？如果是環境不符（連不到某個服務、缺某個變數），"
            "\n那多半代表這支測試從來沒有真的跑過——本 repo 已經有五批這種測試，"
            "\n其中兩批一跑就 FAIL。正解是讓它能跑（miniredis / testcontainers），"
            "\n不是登記進清單。"
            "\n真的只能跳過時，在 scripts/expected-skips.json 加一行並寫清楚理由。",
            file=sys.stderr,
        )

    if args.final:
        # 只檢查跑過的 package —— 沒跑到的模組不能拿來判斷條目是否過時。
        stale = sorted(
            k for k in set(allow) - seen if k.rsplit("::", 1)[0] in packages
        )
        if stale:
            failed = True
            print(
                "\n清單中有已經不再發生的 skip（測試修好了或被刪了，清單沒跟上）：\n",
                file=sys.stderr,
            )
            for key in stale:
                print(f"  ✘ {key}   理由：{allow[key]}", file=sys.stderr)
            print(
                "\n從 scripts/expected-skips.json 移除這些條目。"
                "\n留著會讓清單慢慢變成墳場，下一個人就分不清哪些還成立。",
                file=sys.stderr,
            )
        if not failed:
            print(
                f"skip 檢查通過：{len(seen)} 支跳過的測試全部已登記，"
                f"且本輪跑過的 {len(packages)} 個 package 內沒有過時條目。"
            )
        STATE.unlink(missing_ok=True)

    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
