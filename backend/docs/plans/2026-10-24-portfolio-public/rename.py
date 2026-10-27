#!/usr/bin/env python3
"""識別碼改名（作品集轉 public 步驟 3，機械式）。規格是同目錄的 rename-map.md。

本檔不含任何 denylist 明文：全部明文規則在執行時讀不進版控的 `.local/publish/rename-map.plain.tsv`
（linked worktree 沒有 `.local` 時，用主要 checkout 的，見 scan.py 的 local_dir）。輸出只有規則代號、
過 scan.py 代號化的路徑、行號與計數，不印命中的字串。

用法（在 repo 根目錄或任一 worktree 根目錄）：
  python3 backend/docs/plans/2026-10-24-portfolio-public/rename.py --dry-run      # 每條規則的檔數與處數，不寫入
  python3 backend/docs/plans/2026-10-24-portfolio-public/rename.py                # 套用：先 git mv，再改內容
  python3 backend/docs/plans/2026-10-24-portfolio-public/rename.py --check        # 有任何會變動的就 exit 1（冪等閘門）
  python3 backend/docs/plans/2026-10-24-portfolio-public/rename.py --dry-run --files X3   # 某規則命中的路徑（含行數）

順序：
  1. 路徑：A15（Java 套件目錄，先於 A16）→ A16 → A17 → A29（子品牌，D9），逐檔 `git mv`，由深到淺。二進位檔也改路徑。
  2. 內容：先保護區間（A01 全部類別、A02），再特例與通用規則，依規則總表順序；同一處（重疊的區間）只被
     第一個命中的規則改寫。只處理追蹤中的文字檔（前 8192 位元組沒有 NUL）。

與規則總表順序的差異（TSV 追加的列，規格沒有）：
  - A03S（內容裡的 Java 套件目錄路徑寫法 `/com/<舊名>/`，與 A15 搬完的目錄一致）排在 A03 之後。
  - X3、X4（規格第 8 節）排在 A10 之後、A11 之前。
  - A02 的樣板多收不帶網域的 `<舊名>-food/…`（規格 X5 列的 org/repo 寫法）。
A18 到 A27（縮寫補漏：左邊界是非英數字但右邊是小寫單字、冒號、正則 alternation 或跳脫的寫法）排在 A14 之後。
A28（子品牌，D9：只改舊名那一段，分隔與大小寫形狀照命中）排在最後，路徑版是 A29。
其餘照表序。

不改內容的檔（內容完全不碰；路徑仍會依 A15 到 A17 改）：
  - 計畫檔與附件目錄、`.local/`、TSV 的 EXCLUDE 列（鎖定檔與 sum：改走各工具重新生成）。
  - 偵測規則的來源：這些檔的樣板必須繼續偵測得到舊識別碼（計畫不變量表第 4 列、步驟 7 依 D7 改造）。
    見 DETECTION_SOURCES。
  - Lottie JSON 只套 A01 到 A07（舊名全名）；縮寫家族（A08 到 A14、A18 到 A27、X3、X4）的命中是動畫 id 與 base64 的亂數字串
    （規格 7.1「不改的路徑」）。
"""
from __future__ import annotations

import argparse
import collections
import multiprocessing
import os
import re
import subprocess
import sys
from bisect import bisect_right
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import scan  # noqa: E402
from gen_rename_map import new_for_form  # noqa: E402

ROOT = scan.ROOT
PLAIN_TSV = scan.LOCAL / "publish" / "rename-map.plain.tsv"
SAN = scan.sanitizer(scan.load_denylist())

# 偵測規則的來源：樣板含舊識別碼，必須維持原樣才偵測得到。路徑 -> 理由。
DETECTION_SOURCES = {
    "scripts/check-legacy-endpoints.py": "舊公司端點偵測規則的樣板含舊識別碼（不變量表第 4 列；字面值由步驟 7 依 D7 改造）",
}

# 規則順序：保護區間之後，依規則總表；X3、X4 在 A10 之後。
CONTENT_ORDER = ["A03", "A03S", "A04", "A05", "A06", "A07", "A08", "A09", "A10", "X3", "X4", "A11", "A12", "A13", "A14"] + [f"A{n}" for n in range(18, 28)] + ["A28"]
PATH_ORDER = ["A15", "A16", "A17", "A29"]
ABBR_FAMILY = {"A08", "A09", "A10", "X3", "X4", "A11", "A12", "A13", "A14"} | {f"A{n}" for n in range(18, 28)}
CASE_TOKEN = "@CASE"
ALT_SEP = "／"
MAX_SCAN_BYTES = 8192


class Rule:
    __slots__ = ("id", "kind", "rx", "repl", "note")

    def __init__(self, rid, kind, pattern, repl, flags, note):
        self.id = rid
        self.kind = kind
        self.rx = re.compile(pattern, re.I if "i" in flags else 0)
        self.repl = repl
        self.note = note

    def family(self) -> str:
        return self.id.split(":")[0]

    def replacement(self, matched: str) -> str:
        if self.repl == CASE_TOKEN:
            return new_for_form(matched)
        if ALT_SEP in self.repl:
            opts = [o for o in self.repl.split(ALT_SEP)]
            same = [o for o in opts if len(o) == len(matched) and o[-1:] == matched[-1:]]
            if len(same) != 1:
                sys.exit(f"規則 {self.id} 的替換選項無法唯一決定（設定錯誤）")
            return same[0]
        return self.repl


def load_tsv(path: Path):
    if not path.is_file():
        sys.stderr.write(f"找不到明文對照：{path}（不進版控，需要先用 gen_rename_map.py 產生）\n")
        sys.exit(2)
    rules, excludes = [], []
    for raw in path.read_text(encoding="utf-8").splitlines():
        if not raw.strip() or raw.startswith("#"):
            continue
        c = raw.split("\t")
        if c[0] == "FORM":
            continue
        if c[0] == "EXCLUDE":
            excludes.append((c[1], c[2]))
            continue
        c += [""] * (7 - len(c))
        rules.append(Rule(c[0], c[1], c[2], c[3], c[4], c[6]))
    return rules, excludes


class Plan:
    """已排序的規則與排除。"""

    def __init__(self, path: Path = PLAIN_TSV):
        rules, self.excludes = load_tsv(path)
        by_id = {r.id: r for r in rules}
        self.protect = [r for r in rules if r.kind == "skip"]  # A01:* 與 A02，TSV 順序
        missing = [i for i in CONTENT_ORDER + PATH_ORDER if i not in by_id]
        if missing:
            sys.stderr.write("明文對照缺少規則：" + ",".join(missing) + "\n")
            sys.exit(2)
        self.content = [by_id[i] for i in CONTENT_ORDER]
        self.path = [by_id[i] for i in PATH_ORDER]

    def content_excluded(self, p: str):
        """回傳排除理由或 None。"""
        if scan.excluded(p):
            return "計畫、附件或 .local"
        if p in DETECTION_SOURCES:
            return "偵測規則來源"
        for kind, val in self.excludes:
            if kind == "path-suffix" and (p == val or p.endswith("/" + val)):
                return "鎖定檔或 sum（重新生成）"
            if kind == "path-prefix" and p.startswith(val):
                return "計畫、附件或 .local"
            if kind == "path-exact" and p == val:
                return "計畫、附件或 .local"
        return None


# --------------------------------------------------------------------------- 路徑

def new_path(plan: Plan, p: str):
    """回傳 (新路徑, [套用到的規則 id])。"""
    if scan.excluded(p):
        return p, []
    used = []
    for r in plan.path:
        def sub(m, r=r):
            return r.replacement(m.group(0))

        q = r.rx.sub(sub, p)
        if q != p:
            used.append(r.id)
            p = q
    return p, used


# --------------------------------------------------------------------------- 內容

def _overlaps(starts, spans, s, e) -> bool:
    i = bisect_right(starts, s) - 1
    if i >= 0 and spans[i][1] > s:
        return True
    j = i + 1
    return j < len(spans) and spans[j][0] < e


B64_RUN = re.compile(r"[A-Za-z0-9+/]{60,}")


def b64_spans(text: str) -> list:
    """內嵌的 base64 長字串（圖片、字型、雜湊）。縮寫家族的樣板很短，會誤中這些亂數字串，所以整段不改。
    判斷：連續 60 字元以上的 base64 字元，且大寫、小寫、數字都有（排除很長的路徑與類別名稱）。"""
    out = []
    for m in B64_RUN.finditer(text):
        run = m.group(0)
        if sum(c.isupper() for c in run) >= 5 and sum(c.islower() for c in run) >= 5 and sum(c.isdigit() for c in run) >= 2:
            out.append((m.start(), m.end()))
    return out


def _union(spans: list, new: list) -> list:
    merged = sorted(spans + new, key=lambda x: (x[0], -x[1]))
    out = []
    for s, e, r in merged:
        if out and s < out[-1][1]:
            if e > out[-1][1]:
                out[-1] = (out[-1][0], e, out[-1][2])
        else:
            out.append((s, e, r))
    return out


def claim(plan: Plan, text: str, lottie: bool, want: str = ""):
    """回傳 (改寫區間, 計數, 行號)。改寫區間：[(s, e, rule)]，依 s 排序且不重疊。
    計數的 key：規則 id（改寫處數）、'P:<保護規則>'（保護處數）、'S:<規則>'（因重疊略過）、'B:<規則>'（因 base64 略過）。"""
    counts = collections.Counter()
    spans: list = []  # 已佔用區間 (s, e, rule 或 None)
    for r in plan.protect:
        new = [(m.start(), m.end(), None) for m in r.rx.finditer(text) if m.end() > m.start()]
        if new:
            counts["P:" + r.family()] += len(new)
            spans = _union(spans, new)
    b64 = None
    lines = []
    for r in plan.content:
        is_abbr = r.id in ABBR_FAMILY
        if lottie and is_abbr:
            continue
        starts = [x[0] for x in spans]
        new = []
        for m in r.rx.finditer(text):
            s, e = m.span()
            if e == s:
                continue
            if _overlaps(starts, spans, s, e):
                counts["S:" + r.id] += 1
                continue
            if is_abbr:
                if b64 is None:
                    b64 = b64_spans(text)
                if b64 and _overlaps([x[0] for x in b64], b64, s, e):
                    counts["B:" + r.id] += 1
                    continue
            new.append((s, e, r))
            if r.id == want:
                lines.append(text.count("\n", 0, s) + 1)
        if new:
            counts[r.id] += len(new)
            spans = sorted(spans + new, key=lambda x: x[0])
    return [x for x in spans if x[2] is not None], counts, lines


def rewrite(plan: Plan, text: str, lottie: bool, want: str = ""):
    """回傳 (新文字, 計數, 行號)。"""
    edits, counts, lines = claim(plan, text, lottie, want)
    if not edits:
        return text, counts, lines
    out, last = [], 0
    for s, e, r in edits:
        out.append(text[last:s])
        out.append(r.replacement(text[s:e]))
        last = e
    out.append(text[last:])
    return "".join(out), counts, lines


_PLAN: Plan | None = None


def _init(tsv: str):
    global _PLAN
    _PLAN = Plan(Path(tsv))


def work(job):
    """job = (repo 內路徑, 磁碟上的路徑, 是否寫入, 要列行號的規則 id)。回傳 (路徑, 計數, 行號, 狀態)。"""
    rel, disk, write, want = job
    plan = _PLAN
    full = ROOT / disk
    try:
        if full.is_symlink() or not full.is_file():
            return rel, {}, [], "skip-nonfile"
        data = full.read_bytes()
    except OSError:
        return rel, {}, [], "skip-unreadable"
    if b"\0" in data[:MAX_SCAN_BYTES]:
        return rel, {}, [], "binary"
    text = data.decode("utf-8", errors="surrogateescape")
    lottie = bool(scan.LOTTIE_RX.search(rel))
    new, counts, lines = rewrite(plan, text, lottie, want)
    if write and new != text:
        full.write_bytes(new.encode("utf-8", errors="surrogateescape"))
    return rel, dict(counts), lines, "changed" if new != text else "same"


# --------------------------------------------------------------------------- 主流程

def tracked() -> list:
    return scan.tracked_files()


def plan_moves(plan: Plan, files: list):
    moves = {}
    used = collections.defaultdict(int)
    for p in files:
        q, rules = new_path(plan, p)
        if q != p:
            moves[p] = q
            for r in rules:
                used[r] += 1
    # 衝突檢查：兩個舊路徑對到同一個新路徑，或新路徑已是另一個沒被搬的追蹤檔
    fileset = set(files)
    inv = collections.defaultdict(list)
    for a, b in moves.items():
        inv[b].append(a)
    bad = [b for b, a in inv.items() if len(a) > 1 or (b in fileset and b not in moves)]
    if bad:
        sys.stderr.write(f"路徑衝突 {len(bad)} 處（停止，不寫入）：\n")
        for b in sorted(bad)[:20]:
            sys.stderr.write("  " + SAN(b) + "\n")
        sys.exit(3)
    return moves, used


def git(*args, check=True):
    return subprocess.run(["git", "-c", "core.quotepath=false", *args], cwd=ROOT, capture_output=True, check=check)


def apply_moves(moves: dict):
    ordered = sorted(moves.items(), key=lambda kv: (-kv[0].count("/"), kv[0]))
    old_dirs = set()
    for old, new in ordered:
        os.makedirs((ROOT / new).parent, exist_ok=True)
        r = git("mv", "--", old, new, check=False)
        if r.returncode != 0:
            sys.stderr.write("git mv 失敗：" + SAN(old) + " -> " + SAN(new) + "\n" + SAN(r.stderr.decode("utf-8", "replace"))[:200] + "\n")
            sys.exit(4)
        d = os.path.dirname(old)
        while d:
            old_dirs.add(d)
            d = os.path.dirname(d)
    # 只移除搬完後空的目錄（含空的巢狀目錄），由深到淺；有任何殘留檔的不動
    for d in sorted(old_dirs, key=lambda x: -x.count("/")):
        try:
            os.rmdir(ROOT / d)
        except OSError:
            pass


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--dry-run", action="store_true", help="只計數，不寫入")
    ap.add_argument("--check", action="store_true", help="有任何會變動的就 exit 1")
    ap.add_argument("--files", metavar="RULE", help="列出某內容規則實際會改寫的位置（代號化路徑與行號，需搭配 --dry-run 或 --check）")
    ap.add_argument("--tsv", default=str(PLAIN_TSV))
    a = ap.parse_args()
    dry = a.dry_run or a.check or bool(a.files)

    plan = Plan(Path(a.tsv))
    files = [f for f in tracked() if (ROOT / f).exists() or (ROOT / f).is_symlink()]
    moves, path_used = plan_moves(plan, files)

    if not dry and moves:
        apply_moves(moves)
        files = [f for f in tracked() if (ROOT / f).exists() or (ROOT / f).is_symlink()]
    # 內容處理的磁碟路徑：dry 模式讀舊路徑
    jobs, excluded = [], collections.defaultdict(list)
    for p in files:
        why = plan.content_excluded(p)
        if why is None and moves.get(p) and plan.content_excluded(moves[p]):
            why = plan.content_excluded(moves[p])
        if why:
            excluded[why].append(p)
            continue
        jobs.append((moves.get(p, p) if dry else p, p if dry else p, not dry, a.files or ""))
    # job 的 rel 一律用「改名後的路徑」，lottie 判斷與輸出才跟最終狀態一致

    total = collections.Counter()
    perfile = collections.defaultdict(collections.Counter)
    changed, listing = 0, []
    statuses = collections.Counter()
    ctx = multiprocessing.get_context("spawn")
    with ctx.Pool(max(1, min(8, os.cpu_count() or 2)), initializer=_init, initargs=(a.tsv,)) as pool:
        for rel, counts, lines, st in pool.imap_unordered(work, jobs, chunksize=64):
            statuses[st] += 1
            if st == "changed":
                changed += 1
            for k, v in counts.items():
                total[k] += v
                perfile[k][rel] += v
            if lines:
                listing.append((rel, lines))

    if a.files:
        for rel, lines in sorted(listing):
            print(f"{SAN(rel)}\t{len(lines)}\t{','.join(map(str, lines[:12]))}")
        print(f"# 規則 {a.files}：{len(listing)} 個檔、{sum(len(l) for _, l in listing)} 處", file=sys.stderr)
        return 0

    print("規則\t檔數\t處數\t因重疊略過（被保護或被先序規則佔用）\t因 base64 略過")
    for fam in sorted({r.family() for r in plan.protect}):
        k = "P:" + fam
        print(f"{fam}（保護）\t{len(perfile[k])}\t{total[k]}\t-\t-")
    for r in plan.content:
        print(f"{r.id}\t{len(perfile[r.id])}\t{total[r.id]}\t{total['S:' + r.id]}\t{total['B:' + r.id]}")
    print("路徑規則（檔數）：" + "，".join(f"{r.id}={path_used.get(r.id, 0)}" for r in plan.path) + f"；搬移的檔共 {len(moves)}")
    print(f"內容會變動／已變動的檔：{changed}；狀態：" + "，".join(f"{k}={v}" for k, v in sorted(statuses.items())))
    print("不改內容的檔：")
    for why, ps in sorted(excluded.items()):
        shown = ", ".join(SAN(p) for p in sorted(ps)[:6])
        print(f"  {why}：{len(ps)} 個" + f"（{shown}{'…' if len(ps) > 6 else ''}）")
    if a.check:
        dirty = bool(moves) or changed > 0
        print("check：" + ("有未套用的改名（exit 1）" if dirty else "沒有差異（exit 0）"))
        return 1 if dirty else 0
    return 0


if __name__ == "__main__":
    sys.exit(main())
