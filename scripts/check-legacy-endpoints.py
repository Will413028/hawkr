#!/usr/bin/env python3
"""擋下舊公司端點與帳號識別碼（backend/docs/adr/2026-09-24-go-modular-monolith.md D9）。

公司 2026-09 結束營運，它的網域、infra、資料庫與第三方帳號都不屬於這個 repo。
程式碼或設定只要還指向它們，就可能把資料送進別人的帳號，或在對方回收網域後連到陌生的主機。

規則：整棵樹零容忍，任何命中都失敗。Markdown 與 docs/ 底下的文件也在範圍內；只有本檔不掃。
本機 pre-commit 跑它（.pre-commit-config.yaml）；作品集轉 public 的計畫步驟 9 收尾時移除。

用法：python3 scripts/check-legacy-endpoints.py
"""
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SELF = {"scripts/check-legacy-endpoints.py"}

# 識別碼的末字元以字元集括住：規則等價，但檢查器自己不再含有完整的舊識別碼
PATTERNS = {
    "company-domain": r"(?i)(?:[a-z0-9-]+\.)*dailyfres[h]\.food\b",
    "company-infra-ip": r"(?<![0-9.])(?:8\.219\.195\.149|8\.219\.101\.155|172\.17\.0\.115|172\.17\.0\.214|172\.17\.1\.(?:98|99|100)|47\.236\.24\.168|35\.194\.188\.198|8\.219\.246\.185|47\.237\.122\.63|8\.222\.139\.242|203\.69\.87\.142)(?![0-9])",
    "company-prod-db": r"psdb\.cloud",
    "company-r2-account": r"c797ab7caf9cd2cff7f8fcfe66784df[4]",
    "company-registry": r"ghcr\.io/dailyfres[h]-food",
    "company-workers-subdomain": r"car[l]chou\.workers\.dev",
    "company-oss-bucket": r"daily-fres[h]-media\.oss-[a-z0-9-]+\.aliyuncs\.com",
    "company-sentry-org": r"o451160972532121[6]\.ingest",
    "company-firebase-project": r"dailyfres[h]-(?:(?:seller|buyer)-(?:dev|stage|prod)|official)\b",
    "company-google-oauth-client": r"(?<![0-9])(?:33356677456[2]|65296880702[4]|66827474627[6])-[a-z0-9]+\.apps\.googleusercontent\.com",
    "company-gcp-project-number": r"(?<![0-9])(?:33356677456[2]|65296880702[4]|66827474627[6])(?![0-9])",
}
COMPILED = {name: re.compile(p) for name, p in PATTERNS.items()}

def in_scope(path: str) -> bool:
    return path not in SELF


def scan() -> dict:
    out = subprocess.run(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        cwd=ROOT, capture_output=True, check=True,
    ).stdout
    hits = defaultdict(dict)
    for raw in out.split(b"\0"):
        path = raw.decode("utf-8")
        if not path or not in_scope(path):
            continue
        full = ROOT / path
        if not full.is_file():
            continue
        data = full.read_bytes()
        if b"\0" in data[:8192]:
            continue
        text = data.decode("utf-8", errors="replace")
        for name, rx in COMPILED.items():
            n = len(rx.findall(text))
            if n:
                hits[path][name] = n
    return {p: dict(sorted(v.items())) for p, v in sorted(hits.items())}


def lines_of(path: str, name: str) -> list:
    rx = COMPILED[name]
    text = (ROOT / path).read_text(encoding="utf-8", errors="replace")
    return [i for i, line in enumerate(text.splitlines(), 1) if rx.search(line)]


def main() -> int:
    current = scan()
    for p, v in current.items():
        for name, n in v.items():
            where = "、".join(map(str, lines_of(p, name)[:10]))
            print(f"✘ {p}（第 {where} 行）：{name}，{n} 處")
    if current:
        print("\n有舊公司的端點或帳號識別碼。改成本機服務、測試替身，或新環境的設定。")
        return 1
    print("通過：整棵樹沒有舊公司端點或帳號識別碼。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
