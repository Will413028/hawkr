#!/usr/bin/env python3
"""本機名字檢查（計畫 backend/docs/plans/2026-10-24-portfolio-public.md D7）。

品牌、舊縮寫、同事、法人與合作方的名字只能存在不進版控的明文詞表 `.local/publish/denylist.txt`，
所以這支只在本機 pre-commit（暫存內容）與 pre-push（整棵樹）跑，CI 不跑。詞表不存在（別的 clone）時略過。

詞表格式：每列以 TAB 分隔「類別、pattern、代號[、#uncertain…]」。檢查 brand、brand-abbr、colleague、
entity、partner、domain；`#uncertain` 列、vendor、will-id 不擋。縮寫類另外從 brand-abbr 列推出補漏規則。
輸出只有路徑、類別與計數，不印命中的字串。舊公司 GitHub org 的兩種寫法（`github.com/<org>`、`<org>/<repo>`）
由 brand 列涵蓋。

暫時性：.pre-commit-config.yaml 的 hook 呼叫它，計畫步驟 9 收尾時連 hook 一起移除。

用法（在 repo 內任何位置）：
  python3 backend/docs/plans/2026-10-24-portfolio-public/check-denylist.py            # 整棵樹（git ls-files）
  python3 backend/docs/plans/2026-10-24-portfolio-public/check-denylist.py --staged   # 暫存區內容
exit code：0 通過或略過；1 有命中；2 詞表格式錯誤。
"""
import re
import subprocess
import sys
from collections import Counter
from pathlib import Path

ROOT = Path(subprocess.run(["git", "rev-parse", "--show-toplevel"], cwd=Path(__file__).resolve().parent,
                           capture_output=True, text=True, check=True).stdout.strip())
CATS = {"brand", "brand-abbr", "colleague", "entity", "partner", "domain"}
# base64 或雜湊亂數字串所在的檔，縮寫的補漏規則不掃
NOISE = re.compile(r"(\.sum|\.lock|lock\.yaml|lock\.json|\.svg|\.pbxproj|DomainAssociation\.ts|check-denylist\.py)$|(^|/)assets/lottie/", re.I)


def git(*args: str) -> bytes:
    return subprocess.run(["git", "-c", "core.quotepath=false", *args], cwd=ROOT, capture_output=True, check=True).stdout


def find_denylist() -> Path | None:
    here = ROOT / ".local" / "publish" / "denylist.txt"
    if here.is_file():
        return here
    # linked worktree 沒有 .local 時，改用主要 checkout 的
    common = subprocess.run(["git", "rev-parse", "--path-format=absolute", "--git-common-dir"],
                            cwd=ROOT, capture_output=True, text=True)
    if common.returncode == 0:
        main = Path(common.stdout.strip()).parent / ".local" / "publish" / "denylist.txt"
        if main.is_file():
            return main
    return None


def load(path: Path) -> list[tuple[str, re.Pattern, bool]]:
    rules = []
    tok = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        cols = raw.split("\t")
        if len(cols) < 3:
            print("denylist 有格式錯誤的列（欄位少於 3）", file=sys.stderr)
            sys.exit(2)
        cat = cols[0]
        if cat not in CATS or any(c.strip() == "#uncertain" for c in cols[3:]):
            continue
        rules.append((cat, re.compile(cols[1]), False))
        if cat == "brand-abbr" and tok is None:
            m = re.match(r"\(\?<!\[A-Za-z0-9\]\)([a-z]{2,4})(?![a-z])", cols[1])
            tok = m.group(1) if m else None
    if tok:
        rules += abbr_gap_rules(tok)
    return rules


def abbr_gap_rules(tok: str) -> list[tuple[str, re.Pattern, bool]]:
    """縮寫字母取自 denylist，本檔不含明文。補邊界規則漏掉的寫法。第三欄 True 表示略過亂數字串所在的檔。"""
    lo, up, cap = tok, tok.upper(), tok.capitalize()
    left = r"(?<![A-Za-z0-9])"
    specs = [
        left + lo + r"[a-z]+(?![A-Za-z0-9])",       # 縮寫後接任意小寫單字（<縮寫>ui、<縮寫>origin）
        left + lo + r"(?=:[a-z])",                    # 冒號命名空間
        r"(?<=\(\?:)" + lo + r"(?=\|)",               # 正則 alternation
        r"(?<=\\[bB])" + cap + r"(?=[A-Z][a-z])",     # 正則跳脫之後的 Pascal 前綴
        left + up + r"TEST(?=-)",
        r"(?<=X-)" + cap + r"(?=-)",
        r"(?<=\d_)" + lo + r"(?![A-Za-z0-9])",
        r"(?<=[a-z0-9])" + cap + r"(?![a-z])",         # camelCase 結尾或後接大寫字（fooDf、fooDfBar）
    ]
    return [("brand-abbr", re.compile(s), True) for s in specs]


def files(staged: bool) -> list[tuple[str, bytes | None]]:
    if staged:
        names = git("diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z").split(b"\0")
    else:
        names = git("ls-files", "-z").split(b"\0")
    out = []
    for n in names:
        if not n:
            continue
        p = n.decode("utf-8")
        if staged:
            out.append((p, git("show", ":" + p)))
        else:
            f = ROOT / p
            out.append((p, f.read_bytes() if f.is_file() and not f.is_symlink() else None))
    return out


def main() -> int:
    dl = find_denylist()
    if dl is None:
        print("check-denylist：找不到 .local/publish/denylist.txt，略過名字檢查（只在有詞表的 clone 執行）。")
        return 0
    rules = load(dl)
    hits: Counter = Counter()
    for path, data in files("--staged" in sys.argv[1:]):
        for cat, rx, noisy in rules:
            if noisy and NOISE.search(path):
                continue
            if rx.search(path):
                hits[(path, cat + "-path")] += 1
        if data is None or b"\0" in data[:8192]:
            continue
        text = data.decode("utf-8", errors="replace")
        for cat, rx, noisy in rules:
            if noisy and NOISE.search(path):
                continue
            n = len(rx.findall(text))
            if n:
                hits[(path, cat)] += n
    for (path, cat), n in sorted(hits.items()):
        print(f"{path}\t{cat}\t{n}")
    if hits:
        print(f"\n名字檢查失敗：{len(hits)} 組命中（不印命中字串）。改成新名或移除。", file=sys.stderr)
        return 1
    print("check-denylist 通過")
    return 0


if __name__ == "__main__":
    sys.exit(main())
