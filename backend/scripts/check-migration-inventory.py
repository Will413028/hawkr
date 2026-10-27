#!/usr/bin/env python3
"""遷移盤點 gate：舊服務刪除時，屬於它的 RPC、事件、副本都必須已經決定去向。

盤點資料：docs/plans/2026-09-24-go-modular-monolith/（計畫的「模組完成的定義」）。
每一列的 disposition 欄寫成「<種類>: <說明>」：

  rpc-servers.csv  api（模組 API）、http（對外 endpoint）、removed（刪除）
  events.csv       call（直接呼叫）、in-tx-event（同 transaction 事件）、job（River job）、removed
  replicas.csv     snapshot（保留的時點快照）、query（刪除副本、改查詢）、read-model（讀取模型）、removed

何時必須填：
  rpc-servers.csv  實作它的服務目錄已刪除
  events.csv       producer 與 consumer 的目錄都已刪除（只刪一邊時，事件還經由 legacy adapter 流動）；
                   只存在於設計書、程式裡從來沒有的列（verified_in_code = doc-only）不需要
  replicas.csv     持有副本的服務目錄已刪除

用法：
  python3 scripts/check-migration-inventory.py
  python3 scripts/check-migration-inventory.py --assume-gone content-service   # 刪除前先看要補哪些
"""
import argparse
import csv
import sys
from pathlib import Path

BACKEND = Path(__file__).resolve().parent.parent
INVENTORY = BACKEND / "docs" / "plans" / "2026-09-24-go-modular-monolith"

KINDS = {
    "rpc-servers.csv": {"api", "http", "removed"},
    "events.csv": {"call", "in-tx-event", "job", "removed"},
    "replicas.csv": {"snapshot", "query", "read-model", "removed"},
}


def exists(deployable: str, assume_gone: set) -> bool:
    if deployable in assume_gone:
        return False
    if deployable == "gateway":
        return (BACKEND / "gateway").is_dir()
    return (BACKEND / "services" / deployable).is_dir() or (BACKEND / "services" / "java" / deployable).is_dir()


def owners(name: str, row: dict) -> list:
    if name == "rpc-servers.csv":
        return [row["implementing_deployable"]]
    if name == "events.csv":
        return [d.strip() for col in ("producer_deployable", "consumer_deployable") for d in row[col].split(";")]
    return [row["holder_deployable"]]


def exempt(name: str, row: dict) -> bool:
    return name == "events.csv" and row["verified_in_code"] == "doc-only"


def check(assume_gone: set) -> list:
    problems = []
    for name, kinds in KINDS.items():
        with open(INVENTORY / name, newline="", encoding="utf-8") as f:
            rows = list(csv.DictReader(f))
        for i, row in enumerate(rows, start=2):
            where = f"{name}:{i}"
            disposition = (row.get("disposition") or "").strip()
            if disposition:
                kind, sep, detail = disposition.partition(":")
                if not sep or kind.strip() not in kinds or not detail.strip():
                    problems.append(f"{where}：disposition「{disposition}」格式不對，應為「<{'|'.join(sorted(kinds))}>: <說明>」")
                continue
            if exempt(name, row):
                continue
            gone = [d for d in owners(name, row) if d]
            if gone and not any(exists(d, assume_gone) for d in gone):
                label = " / ".join(v for k, v in row.items() if k in ("proto_service", "rpc", "routing_key", "table_or_store") and v)
                problems.append(f"{where}：{' 與 '.join(gone)} 已不在，但「{label}」還沒有 disposition")
    return problems


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--assume-gone", action="append", default=[], metavar="DEPLOYABLE",
                    help="當作已刪除的服務（刪除前先列出要補的項目）")
    args = ap.parse_args()
    problems = check(set(args.assume_gone))
    for p in problems:
        print(f"✘ {p}")
    if problems:
        print(f"\n{len(problems)} 列需要補上 disposition（見計畫的「模組完成的定義」）。")
        return 1
    print("通過：已刪除的服務，它的 RPC、事件、副本都有去向。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
