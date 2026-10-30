#!/usr/bin/env python3
"""govulncheck 的門檻：只在「程式碼實際呼叫到」的漏洞不在允許清單時失敗。

govulncheck 本身沒有忽略檔，所以這裡讀 `govulncheck -format json ./...` 的輸出，
用 scripts/govulncheck-allowlist.json 當已審查的基準：

  - 呼叫到（symbol 層級）而且不在清單 → 失敗（新的漏洞，要升級或補一筆有期限的允許）
  - 清單項目的 deadline 已過 → 失敗（期限規則在根目錄 AGENTS.md 的「共通規則」）
  - 只 import 到套件或只 require 到 module（沒有呼叫）→ 列出但不失敗，govulncheck 預設也是這樣
  - 清單項目這次沒有出現 → 提醒可以刪（不失敗）

用法：
  govulncheck-gate.py --module <路徑，與 go.work 內相同> --report <govulncheck json 檔> [--today YYYY-MM-DD]
"""
import argparse
import datetime
import json
import sys
from pathlib import Path

ALLOWLIST = Path(__file__).with_name("govulncheck-allowlist.json")


def load_messages(path: str) -> list[dict]:
    """govulncheck 的 json 是多個連續的 JSON 物件，不是陣列。"""
    text = Path(path).read_text()
    dec = json.JSONDecoder()
    i, out = 0, []
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            return out
        obj, i = dec.raw_decode(text, i)
        out.append(obj)


def classify(messages: list[dict]) -> dict[str, dict]:
    """回傳 {osv id: {level, module, fixed}}，level 取最深的一層：symbol > package > module。"""
    rank = {"module": 0, "package": 1, "symbol": 2}
    found: dict[str, dict] = {}
    for m in messages:
        f = m.get("finding")
        if not f:
            continue
        frame = f["trace"][0]
        level = "symbol" if frame.get("function") else "package" if frame.get("package") else "module"
        cur = found.setdefault(f["osv"], {"level": level, "module": frame.get("module"), "fixed": f.get("fixed_version")})
        if rank[level] > rank[cur["level"]]:
            cur["level"] = level
    return found


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--module", required=True)
    ap.add_argument("--report", required=True)
    ap.add_argument("--today", default=datetime.date.today().isoformat())
    args = ap.parse_args()

    messages = load_messages(args.report)
    if not any("progress" in m and "Checking" in m["progress"].get("message", "") for m in messages) and not any(
        "config" in m for m in messages
    ):
        print(f"::error::{args.module}: govulncheck 輸出不完整（沒有 config／progress 訊息）")
        return 2
    found = classify(messages)
    allow = json.loads(ALLOWLIST.read_text())["entries"]
    today = datetime.date.fromisoformat(args.today)
    mine = {e["id"]: e for e in allow if args.module in e["modules"]}

    failed = False
    for vid, info in sorted(found.items()):
        if info["level"] != "symbol":
            print(f"info  {args.module}: {vid} 只 {'import 套件' if info['level'] == 'package' else 'require module'}（{info['module']}），未呼叫，不擋")
            continue
        entry = mine.get(vid)
        if not entry:
            print(f"::error::{args.module}: {vid} 被呼叫到（{info['module']}，修在 {info['fixed']}），不在 scripts/govulncheck-allowlist.json")
            failed = True
        elif datetime.date.fromisoformat(entry["deadline"]) < today:
            print(f"::error::{args.module}: {vid} 的允許期限 {entry['deadline']} 已過（{entry['reason']}）")
            failed = True
        else:
            print(f"allow {args.module}: {vid} 在允許清單內，期限 {entry['deadline']}")
    for vid in sorted(set(mine) - {v for v, i in found.items() if i["level"] == "symbol"}):
        print(f"stale {args.module}: {vid} 這次沒有被呼叫到，可以從允許清單刪除")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
