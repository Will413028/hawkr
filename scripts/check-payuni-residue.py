#!/usr/bin/env python3
"""單體與對外契約裡 PayUni 殘留的上限（ratchet）。

PayUni 已由 payment port 與 Stripe adapter 取代；單體裡剩下的提及是舊契約、legacy 接縫與
CodeMeta 這類隨模組搬遷才退場的東西。這支檢查讓殘留只減不增：

  - 範圍內有檔案提及 payuni（不分大小寫）卻不在基準 → 失敗（新增的殘留）
  - 某檔的筆數超過基準 → 失敗
  - 某檔的筆數低於基準或已歸零 → 失敗，要求執行 --update 把基準調低（上限只往下收）

基準在 scripts/payuni-residue-baseline.json（檔案路徑 → 筆數）。

用法：
  check-payuni-residue.py            # 檢查
  check-payuni-residue.py --update   # 依目前狀態重寫基準（只在殘留減少時用）
"""
import argparse
import json
import subprocess
import sys
from pathlib import Path

SCOPE = ["backend/internal", "backend/cmd", "api/hawkr"]
BASELINE = Path(__file__).with_name("payuni-residue-baseline.json")


def current_counts(root: Path) -> dict[str, int]:
    proc = subprocess.run(
        ["git", "grep", "-z", "-c", "-i", "-I", "payuni", "--", *SCOPE],
        cwd=root, capture_output=True, text=True,
    )
    if proc.returncode not in (0, 1):
        sys.exit(f"git grep 失敗：{proc.stderr.strip()}")
    counts = {}
    for line in proc.stdout.splitlines():
        path, _, n = line.rpartition("\0")
        counts[path] = int(n)
    return counts


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--update", action="store_true")
    ap.add_argument("--root", default=None, help="repo 根目錄（預設為本檔的上一層）")
    args = ap.parse_args()
    root = Path(args.root) if args.root else Path(__file__).resolve().parent.parent
    baseline_path = root / "scripts" / BASELINE.name
    now = current_counts(root)

    if args.update:
        baseline_path.write_text(json.dumps(dict(sorted(now.items())), ensure_ascii=False, indent=2) + "\n")
        print(f"基準已更新：{len(now)} 個檔、{sum(now.values())} 筆")
        return 0

    base = json.loads(baseline_path.read_text())
    errors = []
    for path, n in sorted(now.items()):
        if path not in base:
            errors.append(f"新增殘留：{path}（{n} 筆）不在基準。PayUni 已退場，不要新增提及。")
        elif n > base[path]:
            errors.append(f"殘留增加：{path} 基準 {base[path]} 筆，現在 {n} 筆。")
    for path, n in sorted(base.items()):
        if now.get(path, 0) < n:
            errors.append(f"殘留減少：{path} 基準 {n} 筆，現在 {now.get(path, 0)} 筆。執行 scripts/check-payuni-residue.py --update 調低基準。")
    if errors:
        print("\n".join(errors))
        return 1
    print(f"PayUni 殘留未超過基準：{len(now)} 個檔、{sum(now.values())} 筆")
    return 0


if __name__ == "__main__":
    sys.exit(main())
