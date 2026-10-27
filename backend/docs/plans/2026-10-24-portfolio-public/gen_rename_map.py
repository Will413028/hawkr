#!/usr/bin/env python3
"""由 denylist 與追蹤中的檔產生 rename-map.md，以及不進版控的明文對照 .local/publish/rename-map.plain.tsv。

用法（repo 根目錄）：
  python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py                # 產生兩個檔
  python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A05    # 只印某條規則的檔數與處數
  python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --files A05    # 只印某條規則命中的路徑（代號化）

本檔不含任何 denylist 明文：規則樣板用 `{B}`（舊名英文 pattern，取自 denylist 第一個 brand 列）與
denylist 裡的縮寫列（以代號指定）展開。輸出的 md 只有代號；明文只寫進 .local/publish/。
"""
from __future__ import annotations

import argparse
import collections
import os
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import scan  # noqa: E402

ROOT = scan.ROOT
PLAIN_TSV = scan.LOCAL / "publish" / "rename-map.plain.tsv"
SNAPSHOT = "e5d5a8d8e（程式碼與 332da5e51 相同）"
SELF = "backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py"
CMD = f"python3 {SELF}"
D4 = "io.github.will413028.hawkr"

entries = scan.load_denylist()
_brand_first = next(e for e in entries if e.cat == "brand")
B = _brand_first.rx.pattern.replace("(?i)", "")  # 舊名英文 pattern（不含旗標）
BORDER_B = "(?i:" + B + ")"
BU = B.upper()  # 全大寫寫法的 pattern
BSP = next(e.rx.pattern for e in entries if e.code == "<舊名-空格>").replace("(?i)", "")
ABBR = {e.code: e.rx.pattern for e in entries if e.cat == "brand-abbr"}
ABU = ABBR["<縮寫大寫>-"].rsplit("-", 1)[0]  # 大寫縮寫（含左邊界）
san = scan.sanitizer(entries)


# --------------------------------------------------------------------------- 規則
# 欄位：id, 標題, kind(content|path|skip), 樣板, 取代, 範圍說明, 備註
# 樣板的 {B} 展開成舊名英文 pattern；{ABBR:代號} 展開成 denylist 裡該縮寫列的 pattern。
RULES = [
    ("A01", "舊公司端點規則命中處（10 類）", "skip", "@LEGACY", "（不改）", "全樹",
     "`check-legacy-endpoints.py` 的 10 類規則；步驟 2 先移除，rename.py 在步驟 3 才跑並跳過殘餘命中。"),
    ("A02", "舊 GitHub 組織與舊 registry 名稱", "skip", r"(?:github\.com|ghcr\.io)/{B}-food|(?<![A-Za-z0-9]){B}-food(?=/)", "（不改）", "全樹",
     "外部擁有的名字，改名只會產生不存在的連結；issue 連結由步驟 6 處理。"),
    ("A03", "Java 套件與反向網域：com.<舊名>", "content", r"\bcom\.{B}\b", D4, "Java、proto 的 java_package、Spring 設定、logging 與 mapper 設定",
     "D4；比通用規則先套，否則會變成 `com.hawkr`。"),
    ("A04", "iOS bundle ID 與 Apple 商家 ID 的反向網域：food.<舊名>", "content", r"\bfood\.{B}\b", D4, "pbxproj、xcconfig、plist、entitlements、文件",
     "D4；`merchant.food.<舊名>` 同樣改成 `merchant.` 加 D4 前綴，但 Apple 開發者後台要另外重建。"),
    ("A05", "舊服務的 Go module 前綴：github.com/<舊名>/", "content", r"github\.com/{B}/", "github.com/Will413028/hawkr/", "go.mod、go.work 的 replace、import、go_package、buf 設定",
     "計畫步驟 1：舊服務的 module 一併改成這個前綴；後綴（gateway、go-pkg、*-service）不變。"),
    ("A06", "單體的 Go module：github.com/Will413028/<舊名>", "content", r"github\.com/Will413028/{B}", "github.com/Will413028/hawkr", "go.mod、import、buf.gen、Makefile、workflow",
     "只換名字，帳號不變。"),
    ("A07", "通用規則：英文各種寫法", "content", r"{B}|{BSP}", "@CASE", "全樹（除 skip 與排除清單）",
     "依命中字串的大小寫形狀取代：全小寫（含連字號）→ hawkr；首字大寫 → Hawkr；全大寫 → HAWKR。"),
    ("A08", "縮寫（D6）：小寫連字號", "content", r"{ABBR:<縮寫>-}", "hk-", "CSS 自訂屬性、Tailwind class、DOM id、檔名", "D6；左邊界是非英數字，所以 `pdf-` 不會被改。"),
    ("A09", "縮寫（D6）：小寫底線", "content", r"{ABBR:<縮寫>_}", "hk_", "Dart 檔名與 import、識別碼、Go 與 TS 的常數", "D6；同上。"),
    ("A10", "縮寫：大寫連字號與底線（資料值撞名，見第 8 節）", "content", r"{ABBR:<縮寫大寫>-}|{ABBR:<縮寫大寫>_}", "HK-／HK_", "HTTP header、ID 格式、測試常數",
     "D6 沒明講大寫；D8=A（2026-10-24）已決：同族寫法一併改；出現在資料值的部分見第 8 節。"),
    ("A11", "縮寫：UI 元件庫前綴 `<縮寫>ui-`、頁面前綴 `<縮寫>h-`", "content", r"{ABBR:<縮寫>ui-}|{ABBR:<縮寫>h-}", "hkui-／hkh-", "website、admin 的 CSS 與 TSX", "D6 沒列出的同族前綴；D8=A 已決：一併改。"),
    ("A12", "縮寫：Pascal 類別前綴 `<縮寫Pascal>`＋大寫字母", "content", r"{ABBR:<縮寫Pascal>}", "Hk", "Dart、TS 的類別與型別", "D6 沒列出；D8=A 已決：一併改；鎖定檔與 JSON 內的亂數 id 要排除（見第 7 節）。"),
    ("A13", "縮寫：日誌類別 `<縮寫>Log`", "content", r"{ABBR:<縮寫>Log}", "HKLog", "Dart 的日誌工具與文件", "同上。"),
    ("A14", "縮寫：camelCase `<縮寫>`＋大寫字母", "content", r"{ABBR:<縮寫camel>}", "hk", "TS 識別碼", "denylist 標為 uncertain；go.sum 與 lottie 的亂數字串會誤中，要排除。"),
    ("A15", "路徑：Java 套件目錄 com/<舊名>/", "path", r"(?<![A-Za-z0-9])com/{B}/", "io/github/will413028/hawkr/", "`git mv`，目錄深度增加", "依 A03；先於 A16。"),
    ("A16", "路徑：目錄與檔名裡的舊名", "path", r"{B}", "@CASE", "`git mv`", "同 A07，依命中形狀。"),
    ("A17", "路徑：縮寫檔名 <縮寫>_*.dart、<縮寫>-*", "path", r"{ABBR:<縮寫>_}|{ABBR:<縮寫>-}", "hk_／hk-", "`git mv`", "同 A08、A09；內容裡的 import 同步改。"),
]

# 通用規則涵蓋的類別（A07 的明細），每類一個搜尋樣板與一個新樣貌範例
CLASSES = [
    ("K01", "Go import 路徑（單體）", r"github\.com/Will413028/{B}[A-Za-z0-9_./-]*", "github.com/Will413028/hawkr/backend/…", "A06"),
    ("K02", "Go import 路徑（舊服務與 pkg）", r"github\.com/{B}/[A-Za-z0-9_.-]+", "github.com/Will413028/hawkr/<後綴>", "A05"),
    ("K03", "Java 套件與 import", r"\bcom\.{B}\b", D4 + ".…", "A03"),
    ("K04", "npm 套件範圍與 workspace filter", r"@{B}/[a-z-]+", "@hawkr/api、@hawkr/ui-text、@hawkr/twaddr", "A07"),
    ("K05", "Dart 套件 import", r"package:{B}_[a-z_]+", "package:hawkr_<後綴>", "A07"),
    ("K06", "proto package 宣告", r"(?m)^\s*package\s+{B}[A-Za-z0-9_.]*;", "hawkr.<領域>.v1alpha1（v1alpha1 不改）", "A07"),
    ("K07", "proto import 路徑", r"(?m)^\s*import\s+\"{B}/[^\"]*\"", "import \"hawkr/…\"", "A07"),
    ("K08", "容器名與 compose 網路", r"(?m)^\s*(?:container_name:\s*|-\s+|networks?:\s*){B}[A-Za-z0-9_.-]*", "hawkr-<名稱>", "A07"),
    ("K09", "資料庫名稱（連線字串與環境變數）", r"(?:POSTGRES_DB|DB_NAME|DATABASE_URL|PGDATABASE|dbname)[=: ]+[^\s\"']*{B}[^\s\"']*", "hawkr、hawkr_<服務>", "A07"),
    ("K10", "資料庫名稱（URL 路徑）", r"/{B}(?:_[a-z]+)?(?:\?|\"|'|\s|$)", "/hawkr、/hawkr_<服務>", "A07"),
    ("K11", "RabbitMQ exchange、queue 與 routing key", r"(?:exchange|queue|routing[_-]?key|Exchange|Queue)[A-Za-z_]*\W{1,4}[\"']?{B}[A-Za-z0-9_.\-]*", "hawkr.<領域>、hawkr.<領域>.dlx", "A07"),
    ("K12", "環境變數前綴（大寫）", r"(?<![A-Za-z0-9]){BU}[A-Z0-9_]*", "HAWKR_…", "A07"),
    ("K13", "URL scheme（深層連結）", r"{B}[a-z]*://", "hawkr://、hawkrbuyer://、hawkrseller://", "A07"),
    ("K14", "儲存鍵與 JSON 字串鍵（sessionStorage、版本字串）", r"[\"']{B}[:.][a-zA-Z0-9_.:-]+[\"']", "\"hawkr:…\"", "A07"),
    ("K15", "HTTP header 名稱", r"(?i)x-{B}-[a-z-]+", "X-Hawkr-…", "A07"),
    ("K16", "指標名稱（Prometheus）", r"\b{B}_[a-z_]+_(?:total|seconds|bytes|count|days)\b", "hawkr_<…>_total", "A07"),
    ("K17", "Spring 設定前綴與 Java 屬性鍵", r"(?m)^\s*{B}:\s*$|(?<![A-Za-z0-9.]){B}\.(?:order|obs|shipping|payment|media|product|aftersales|cart|coupon)\.", "hawkr.<服務>.*", "A07"),
    ("K18", "buf 模組名稱", r"buf\.build/{B}/[a-z]+", "buf.build/hawkr/api", "A07"),
    ("K19", "開發用 token 的 issuer 與其他字串常數", r"\"{B}-[a-z]+\"", "\"hawkr-dev\"", "A07"),
    ("K20", "網站與其他套件名稱（package.json name、pubspec name）", r"(?m)^\s*(?:\"name\":\s*\"|name:\s*)[^\"\n]*{B}[^\"\n]*", "hawkr-web、@hawkr/api、hawkr_buyer", "A07"),
    ("K21", "CSS 自訂屬性與 Tailwind class（縮寫）", r"(?:--|\b(?:text|bg|border|ring|shadow|from|to|via|fill|stroke|divide|outline|decoration|accent|caret|placeholder)-){ABBR_DASH_NAME}[a-z0-9-]*", "--hk-primary、text-hk-muted", "A08"),
    ("K22", "Dart 縮寫檔名與 import", r"{ABBR_UND_NAME}[a-z0-9_]+\.dart", "hk_tokens.dart", "A09"),
]


def expand(template: str) -> str:
    t = template.replace("{BSP}", BSP).replace("{BU}", BU).replace("{B}", B)

    def abbr(m):
        return ABBR[m.group(1)]

    t = re.sub(r"\{ABBR:([^}]+)\}", abbr, t)
    t = t.replace("{ABBR_DASH_NAME}", ABBR["<縮寫>-"].replace("(?<![A-Za-z0-9])", ""))
    t = t.replace("{ABBR_UND_NAME}", ABBR["<縮寫>_"].replace("(?<![A-Za-z0-9])", "(?<![A-Za-z0-9])"))
    return t


# 規則的檔案層排除（鎖定檔與生成物走重新生成）
LOCK_SUFFIX = ("pnpm-lock.yaml", "pubspec.lock", "go.sum", "package-lock.json", "atlas.sum")


def rel_excluded(p: str) -> bool:
    return scan.excluded(p)


# --------------------------------------------------------------------------- 掃描
def gg(pattern: str, ci: bool = False):
    """git grep -o -P：回傳 [(path, 命中字串)]；只搜追蹤中的檔，排除計畫、附件與鎖定檔。"""
    import subprocess

    cmd = ["git", "-c", "core.quotepath=false", "grep", "-z", "-o", "-P", "-I", "--no-color"]
    if ci:
        cmd.append("-i")
    cmd += ["-e", pattern, "--", "."]
    r = subprocess.run(cmd, cwd=ROOT, capture_output=True)
    if r.returncode not in (0, 1):
        sys.stderr.write("git grep 失敗：" + r.stderr.decode("utf-8", "replace")[:200] + "\n")
        sys.exit(2)
    out = []
    for line in r.stdout.decode("utf-8", "replace").split("\n"):
        if "\0" not in line:
            continue
        path, m = line.split("\0", 1)
        if rel_excluded(path) or path.endswith(LOCK_SUFFIX):
            continue
        out.append((path, m))
    return out


def collect(pattern: str, ci: bool = False, post=None):
    d = collections.defaultdict(set)
    for path, m in gg(pattern, ci):
        v = post(m) if post else m
        if v:
            d[v].add(path)
    return d


def rule_pattern(rid):
    for r in RULES:
        if r[0] == rid and r[2] != "path" and r[3] != "@LEGACY":
            return expand(r[3]), rid in ("A02", "A03", "A04", "A05", "A06", "A07")
    for cid, title, tpl, new, rule in CLASSES:
        if cid == rid:
            t = tpl.replace("{BU}", BU).replace("{B}", B).replace("{ABBR_DASH_NAME}", ABBR["<縮寫>-"].replace("(?<![A-Za-z0-9])", "")).replace("{ABBR_UND_NAME}", ABBR["<縮寫>_"])
            return t, "{B}" in tpl
    raise KeyError(rid)


def legacy_rx():
    return scan.legacy_rules()


def count_rules(files, rule_ids=None):
    res = collections.defaultdict(lambda: [0, 0, []])
    ids = [r[0] for r in RULES if r[2] != "path" and r[3] != "@LEGACY"] + [c[0] for c in CLASSES]
    for rid in ids:
        if rule_ids and rid not in rule_ids:
            continue
        pat, ci = rule_pattern(rid)
        perfile = collections.Counter(p for p, _ in gg(pat, ci))
        res[rid] = [len(perfile), sum(perfile.values()), sorted(perfile)]
    if rule_ids is None or "A01" in rule_ids:
        perfile = collections.Counter()
        for n, r in legacy_rx().items():
            for p, _ in gg(r.pattern, False):
                perfile[p] += 1
        res["A01"] = [len(perfile), sum(perfile.values()), sorted(perfile)]
    return res


def path_counts(files):
    rx_b = re.compile(B, re.I)
    ab = [re.compile(ABBR["<縮寫>_"]), re.compile(ABBR["<縮寫>-"])]
    brand_paths = [f for f in files if rx_b.search(f) and not rel_excluded(f)]
    abbr_paths = [f for f in files if any(r.search(f) for r in ab) and not rel_excluded(f)]
    return brand_paths, abbr_paths


def new_name(p: str) -> str:
    def case(m):
        s = m.group(0)
        return "HAWKR" if s.isupper() else ("Hawkr" if s[0].isupper() else "hawkr")

    p = re.sub(r"(?<![A-Za-z0-9])com/" + B + "/", "io/github/will413028/hawkr/", p, flags=re.I)
    p = re.sub(B, case, p, flags=re.I)
    p = re.sub(ABBR["<縮寫>_"], "hk_", p)
    p = re.sub(ABBR["<縮寫>-"], "hk-", p)
    return p


# --------------------------------------------------------------------------- 輸出
def esc(s):
    return s.replace("|", "\\|")


def tbl(header, rows):
    out = ["| " + " | ".join(header) + " |", "|" + "|".join(["---"] * len(header)) + "|"]
    out += ["| " + " | ".join(esc(str(c)) if False else str(c) for c in r) + " |" for r in rows]
    return out


def distinct_forms(files):
    forms = collections.Counter()
    ffiles = collections.defaultdict(set)
    for path, m in gg("(?:" + B + "|" + BSP + ")", True):
        forms[m] += 1
        ffiles[m].add(path)
    return forms, ffiles


def form_shape(s: str) -> str:
    sep = "空格" if " " in s else ("連字號" if "-" in s else ("底線" if "_" in s else "無分隔"))
    if s.isupper():
        case = "全大寫"
    elif s.islower():
        case = "全小寫"
    elif s[0].isupper() and re.search(r"[a-z][A-Z]|[ \-_][A-Z]", s):
        case = "每個字首大寫（駝峰或 Title）"
    else:
        case = "首字大寫"
    return f"{case}、{sep}"


def new_for_form(s: str) -> str:
    return "HAWKR" if s.isupper() else ("Hawkr" if s[0].isupper() else "hawkr")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--count")
    ap.add_argument("--files")
    ap.add_argument("--out", default=str(HERE / "rename-map.md"))
    ap.add_argument("--tsv", default=str(PLAIN_TSV))
    a = ap.parse_args()
    files = [f for f in scan.tracked_files() if (ROOT / f).is_file()]

    if a.count or a.files:
        rid = a.count or a.files
        res = count_rules(files, {rid})
        if a.count:
            r = res.get(rid, [0, 0, []])
            print(f"{rid}\t檔={r[0]}\t處={r[1]}")
        else:
            for p in sorted(res.get(rid, [0, 0, []])[2]):
                print(san(p))
        return 0

    import time
    t0 = time.time()
    res = count_rules(files)
    print('count_rules',round(time.time()-t0),flush=True)
    forms, fforms = distinct_forms(files)
    brand_paths, abbr_paths = path_counts(files)
    print('forms+paths',round(time.time()-t0),flush=True)

    # ---- 明文對照 TSV
    PLAIN_TSV.parent.mkdir(parents=True, exist_ok=True)
    lines = ["# 明文對照（不進版控）。欄位：id<TAB>kind<TAB>regex<TAB>replace<TAB>flags<TAB>scope<TAB>note", "# 規則依序套用；skip 是保護區間，先於所有改寫；@CASE 依命中字串的大小寫形狀取代。"]
    for rid, title, kind, tpl, repl, scope, note in RULES:
        if kind == "skip" and tpl == "@LEGACY":
            for n, r in legacy_rx().items():
                lines.append("\t".join([f"{rid}:{n}", "skip", r.pattern, "", "", scope, "check-legacy-endpoints 規則"]))
            continue
        flags = "i" if rid in ("A02", "A03", "A04", "A05", "A06", "A07", "A15", "A16") else ""
        lines.append("\t".join([rid, kind, expand(tpl), repl.replace("（不改）", ""), flags, scope, title]))
    # 規格第 8 節 X3、X4：大寫縮寫加數字（不帶連字號，A08 到 A10 命中不到）。左邊界同縮寫規則；右邊界排除英數字、底線與連字號，
    # 所以十六進位或 base64 的長字串不會誤中；Lottie JSON 由 rename.py 整批跳過縮寫家族。
    lines.append("\t".join(["X3", "content", ABU + r"(?=\d{6,}(?![A-Za-z0-9_-]))", "HK", "", "訂單與出貨單編號前綴（6 位數以上）", "X3"]))
    lines.append("\t".join(["X4", "content", ABU + r"(?=\d{1,5}(?![A-Za-z0-9_-]))", "HK", "", "測試用編號前綴（1 到 5 位數）", "X4"]))
    # A03S：A03 的路徑寫法（`/com/<舊名>/`）。規則總表只有 A15 管 `git mv` 的路徑；內容裡的路徑字串（測試用相對路徑讀 Java 原始碼、
    # 生成腳本的輸出目錄、文件）若交給 A07，會得到 `/com/hawkr/` 而不是 A15 搬完的目錄。放在 A03 之後、A07 之前；
    # 樣板的 `hawkr` 備選是為了收拾沒有這條規則時的舊版 rename.py 留下的結果。
    lines.append("\t".join(["A03S", "content", r"(?<=/)com/(?:" + B + r"|hawkr)/", D4.replace(".", "/") + "/", "i", "Java 套件目錄的路徑寫法", "A03S"]))
    lines.append("# 排除的路徑（rename.py 不改內容，改走重新生成）")
    for suffix in LOCK_SUFFIX:
        lines.append(f"EXCLUDE\tpath-suffix\t{suffix}")
    lines.append("EXCLUDE\tpath-prefix\tbackend/docs/plans/2026-10-24-portfolio-public")
    lines.append("EXCLUDE\tpath-exact\tbackend/docs/plans/2026-10-24-portfolio-public.md")
    lines.append("EXCLUDE\tpath-prefix\t.local/")
    lines.append("# 實際出現的英文寫法（明文）與新字串")
    for f, n in forms.most_common():
        lines.append(f"FORM\t{f}\t{new_for_form(f)}\t{n}")
    PLAIN_TSV.write_text("\n".join(lines) + "\n", encoding="utf-8")

    # ---- md
    out = []
    W = out.append
    W("# 識別碼改名對照（步驟 1 附件）")
    W("")
    W(f"快照 commit：{SNAPSHOT}。由 `gen_rename_map.py` 依 denylist 展開規則並在追蹤中的檔上計數；本檔只有代號，明文對照在不進版控的 `.local/publish/rename-map.plain.tsv`，`rename.py`（步驟 3）讀後者。")
    W("")
    W("讀法：`<舊名>` 是 denylist 第一列定義的舊名英文各種寫法；`<縮寫>` 是 D6 的兩字母縮寫；`<舊名-中文>` 是中文名。新字串 `hawkr` 的大小寫變體依命中形狀決定（見第 1 節）。")
    W(f"每條規則的計數指令：`{CMD} --count <規則 id>`；命中路徑：`{CMD} --files <規則 id>`（路徑已代號化，`scan.py --resolve` 可還原）。計數不含計畫、附件與 `.local/`。")
    W("")
    W("D8=A（2026-10-24，Will 已決）：縮寫的所有同族寫法（A10 到 A12 與第 6、7 節的其他家族）與第 8 節的資料值（X1 到 X4）一律改成 `hk`／`HK`／`Hk` 系列，連同產生碼、驗證正則、CHECK 約束、種子資料與 fixture；migration 改寫並在步驟 3 重算 `atlas.sum`。原 R-1 因此結案。")
    W("")
    W("## 待 Will 確認")
    W("")
    W("| 代號 | 問題 | 預設 |")
    W("|---|---|---|")
    W("| R-2 | 舊服務 Go module 只換前綴（`github.com/<舊名>/gateway` → `github.com/Will413028/hawkr/gateway`），不併進 `github.com/Will413028/hawkr/backend/…` 的目錄結構 | 只換前綴，後綴不變（最小改動；`backend/gateway` 預定在階段 B、C 刪除） |")
    W("| R-3 | 中文名 `<舊名-中文>`（含 App 顯示名稱、法律頁、測試資料裡的商號）不在 rename.py：中文在步驟 4（使用者面文字）與步驟 6（測試資料）逐項處理 | 同左 |")
    W("")

    # 第 1 節
    W("## 1. 舊字串對新字串")
    W("")
    W("### 1.1 英文各種寫法（`<舊名>`）")
    W("")
    W(f"- 指令：`{CMD} --count A07`；總檔數另見 `scan.py --category brand --english --report | wc -l`（6,485 個檔）。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    tr = []
    for f, n in forms.most_common():
        tr.append([f"`{san(f)}`", form_shape(f), len(fforms[f]), n, f"`{new_for_form(f)}`"])
    out += tbl(["命中形狀（代號化）", "形狀", "檔數", "處數", "新字串"], tr)
    W("")
    W("說明：代號化後各形狀都顯示成 `<舊名>`，所以另列「形狀」欄；`rename.py` 的明文表（TSV 的 FORM 列）逐形狀列出。沒有出現的寫法（例如底線分隔）不列。")
    W("")
    W("### 1.2 規則總表（依序套用）")
    W("")
    W("先套 skip，再套特例（A03 到 A06，比通用規則先），最後套通用規則與縮寫。同一處只被第一個命中的規則改寫。")
    W("")
    tr = []
    for rid, title, kind, tpl, repl, scope, note in RULES:
        if kind == "path":
            r = None
            cnt = brand_paths if rid in ("A15", "A16") else abbr_paths
            if rid == "A15":
                cnt = [p for p in brand_paths if re.search(r"(?<![A-Za-z0-9])com/" + B + "/", p, re.I)]
            tr.append([rid, title, "路徑", f"`{san(repl) if rid in ('A01',) else repl}`", scope, f"{len(cnt)} 個檔", "`scan.py --category brand-path --report`（路徑）；明細見第 3 節", note])
            continue
        r = res.get(rid, [0, 0, []])
        tr.append([rid, title, "內容" if kind == "content" else "保護", f"`{repl}`", scope, f"{r[0]} 個檔、{r[1]} 處", f"`{CMD} --count {rid}`", note])
    out += tbl(["id", "規則", "對象", "新字串", "範圍", "計數", "搜尋指令", "備註"], tr)
    W("")
    W("### 1.3 通用規則涵蓋的類別（明細）")
    W("")
    W("類別的搜尋樣板是依該類別的語意寫的；這些處數有重疊（同一個字串可能同時屬於兩類）。")
    W("")
    tr = []
    for cid, title, tpl, new, rule in CLASSES:
        r = res.get(cid, [0, 0, []])
        tr.append([cid, title, f"`{new}`", rule, f"{r[0]} 個檔、{r[1]} 處", f"`{CMD} --count {cid}`"])
    out += tbl(["類別", "說明", "新樣貌", "套用規則", "計數", "搜尋指令"], tr)
    W("")

    # 第 2 節 Go module
    W("## 2. Go module 路徑")
    W("")
    gomods = sorted(f for f in files if f.endswith("go.mod"))
    W(f"- 指令：`git ls-files | grep -E '(^|/)go\\.(mod|work)$'` 共 {len(gomods) + 1} 個檔（{len(gomods)} 個 go.mod 加 `backend/go.work`）；`{CMD} --count K01`、`--count K02`。")
    W(f"- 快照：{SNAPSHOT}。改名後每個 module 要 `GOWORK=off go mod tidy` 確認沒有 diff（`AGENTS.local.md`），`go.work` 的 `use` 路徑是目錄、不改。")
    W("")
    tr = []
    # 來源：go.mod 的 module 行與 replace
    rx_mod = re.compile(r"(?m)^module\s+(\S+)")
    rx_rep = re.compile(r"(?m)^replace\s+(\S+)\s+=>\s+(\S+)")
    importers = collections.Counter()
    for f in files:
        if f.endswith(".go"):
            pass
    imp_files = collections.defaultdict(int)
    for gm in gomods:
        t = (ROOT / gm).read_text(encoding="utf-8")
        mod = rx_mod.search(t).group(1)
        new = re.sub(r"github\.com/Will413028/" + B, "github.com/Will413028/hawkr", mod, flags=re.I)
        new = re.sub(r"github\.com/" + B + "/", "github.com/Will413028/hawkr/", new, flags=re.I)
        rep = rx_rep.findall(t)
        rep_s = "；".join(f"replace `{san(a)}` → `{b}`（目錄，不改）" for a, b in rep) or "-"
        note = "沒有舊名，不改" if new == mod else ""
        tr.append([f"`{san(gm)}`", f"`{san(mod)}`", f"`{new}`", rep_s, note or "改前綴；module 路徑與 replace 左側同步"])
    out += tbl(["go.mod", "目前的 module", "新 module", "replace", "備註"], tr)
    W("")
    W("- `backend/go.work` 的 `use` 清單 12 個目錄（`.`、`./gateway`、`./monitoring/discord-webhook-gateway`、`./pkg`、`./services/{ai,crm,im,invoice,notification,pricebook,review,seller,social}-service`）都是相對目錄，不含舊名，不改。")
    W("- proto 的 `option go_package`：`backend/api/proto/**` 的 `github.com/<舊名>/go-pkg/proto/…` 依 A05 改；`backend/buf.gen.connect.yaml` 的 `go_package_prefix` 依 A06 改。這些改完要重新生成 `backend/pkg/proto` 與 `backend/internal/legacy/gen`，不能只改生成物的 import。")
    W("")

    # 第 3 節 目錄名與檔名
    W("## 3. 目錄名與檔名")
    W("")
    W(f"- 路徑含舊名的追蹤檔 {len(brand_paths)} 個；路徑含縮寫的追蹤檔 {len(abbr_paths)} 個（`{CMD} --category brand-path` 的同義指令：`scan.py --category brand-path --report | wc -l` 另含 uncertain 與 colleague 列）。")
    W("- 指令：`git ls-files | …` 依 denylist 展開後的規則；也可用 `scan.py --category brand-path --report`。")
    W(f"- 快照：{SNAPSHOT}。順序：先 `git mv` 所有目錄與檔名（由深到淺），再改內容；`rename.py` 再跑一次不得有 diff。")
    W("")
    dirs = collections.Counter()
    rx_b = re.compile(B, re.I)
    rx_u = re.compile(ABBR["<縮寫>_"])
    rx_d = re.compile(ABBR["<縮寫>-"])
    for f in files:
        if rel_excluded(f):
            continue
        parts = f.split("/")
        for i, p in enumerate(parts[:-1]):
            if rx_b.search(p) or rx_u.search(p) or rx_d.search(p):
                dirs["/".join(parts[: i + 1])] += 1
    tr = []
    for d, c in sorted(dirs.items()):
        # 只列最上層：沒有父目錄也在 dirs 內的才是頂層需要 git mv 的
        parent = d.rsplit("/", 1)[0] if "/" in d else ""
        if parent in dirs:
            continue
        tr.append([f"`{san(d)}`", f"`{new_name(d)}`", c, "Java 套件目錄，深度增加（A15）" if re.search(r"(^|/)com/" + B + "$", d, re.I) else "A16／A17"])
    W("頂層需要 `git mv` 的目錄（巢狀的跟著走）：")
    W("")
    out += tbl(["目錄（代號化）", "新路徑", "底下的追蹤檔數", "規則"], tr)
    W("")
    bf = [p for p in brand_paths if not any(p.startswith(d + "/") for d in dirs)]
    af = [p for p in abbr_paths if not any(p.startswith(d + "/") for d in dirs)]
    W(f"不在上述目錄底下、本身檔名含舊名或縮寫的檔：舊名 {len(bf)} 個、縮寫 {len(af)} 個。依副檔名：")
    W("")
    ext = collections.Counter()
    for p in bf:
        ext[("舊名", p.rsplit(".", 1)[-1].lower() if "." in p.split("/")[-1] else "(無)")] += 1
    for p in af:
        ext[("縮寫", p.rsplit(".", 1)[-1].lower() if "." in p.split("/")[-1] else "(無)")] += 1
    out += tbl(["類型", "副檔名", "檔數"], [[k[0], k[1], v] for k, v in sorted(ext.items(), key=lambda kv: (kv[0][0], -kv[1]))])
    W("")
    W("字型檔（`.woff2`）的檔名與檔內的家族名都含舊名：檔名由 `git mv` 改，檔內 name table 與 manifest 的雜湊要重新生成（第 8 節）。")
    W("")

    # 第 4 節 npm
    W("## 4. npm 套件")
    W("")
    pjs = sorted(f for f in files if f.endswith("package.json") and "node_modules" not in f)
    W(f"- 指令：`git ls-files | grep 'package.json$'`；`{CMD} --count K04`、`--count K20`。")
    W(f"- 快照：{SNAPSHOT}。改完以 `pnpm install --lockfile-only` 重新產生 `pnpm-lock.yaml`（鎖定檔不由 rename.py 改）。")
    W("")
    tr = []
    import json

    for f in pjs:
        try:
            d = json.load(open(ROOT / f, encoding="utf-8"))
        except Exception:
            continue
        n = d.get("name", "")
        nn = new_for_form(n) if False else re.sub(B, lambda m: new_for_form(m.group(0)), n, flags=re.I)
        tr.append([f"`{san(f)}`", f"`{san(n)}`", f"`{nn}`", "套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步" if nn != n else "不變"])
    out += tbl(["package.json", "現名", "新名", "備註"], tr)
    W("")
    W("- 連字號寫法 `<舊名>`（admin 的 `<舊名>-admin`）也由 A07 處理，新名 `hawkr-admin`。")
    W("- `pnpm-workspace.yaml` 的 catalog 與 workflow 裡的 `--filter @<舊名>/…` 由 A07 改。")
    W("")

    # 第 5 節 Dart
    W("## 5. Dart 套件")
    W("")
    pubs = sorted(f for f in files if f.endswith("pubspec.yaml"))
    W(f"- 指令：`git ls-files | grep 'pubspec.yaml$'`；`{CMD} --count K05`、`--count K22`。")
    W(f"- 快照：{SNAPSHOT}。目錄由 `packages/<舊名>_*` 改成 `packages/hawkr_*`（第 3 節）；`pubspec.lock` 由 `fvm flutter pub get` 重新產生；`dart run build_runner build --delete-conflicting-outputs` 重新生成 `*.g.dart`／`*.freezed.dart`（CI 的生成檢查）。")
    W("")
    tr = []
    for f in pubs:
        t = (ROOT / f).read_text(encoding="utf-8")
        m = re.search(r"(?m)^name:\s*(\S+)", t)
        n = m.group(1) if m else ""
        nn = re.sub(B, lambda mm: new_for_form(mm.group(0)), n, flags=re.I)
        tr.append([f"`{san(f)}`", f"`{san(n)}`", f"`{nn}`", "套件名與 `package:` import、path 依賴同步" if nn != n else "不變"])
    out += tbl(["pubspec.yaml", "現名", "新名", "備註"], tr)
    W("")

    # 第 6 節 DB、queue、env、容器、bundle、proto、Java
    W("## 6. 資料庫、訊息佇列、環境變數、容器、bundle ID、proto、Java")
    W("")
    W(f"- 計數指令：K09 到 K19 見第 1.3 節；本節的清單由 `{CMD}` 在追蹤中的檔上展開，不含值（只有名稱的形狀）。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    # 6.1 DB names
    W("### 6.1 資料庫名稱")
    W("")
    names = collect(r"(?<![A-Za-z0-9.\-/@:])(?:" + B + r")(?:_[a-z]+)?(?![A-Za-z0-9_\-.:/])", True)
    tr = [[f"`{san(n)}`", f"`{re.sub(B, lambda mm: new_for_form(mm.group(0)), n, flags=re.I)}`", len(fs)] for n, fs in sorted(names.items(), key=lambda kv: -len(kv[1]))]
    out += tbl(["名稱（代號化）", "新名", "出現的檔數"], tr)
    W("")
    W("單體沒有自己的服務 DB 名；舊服務的 `<舊名>_<服務>` 與單體的 `<舊名>` 都是本機 docker 的資料庫（`backend/docker-compose-*.yml`、`Makefile`、`.env.template`），前提表已確認不在線，直接改名、不留別名。Atlas 的 `atlas.hcl`、`atlas.sum` 要重算（`atlas migrate hash`）。")
    W("")
    # 6.2 dotted
    W("### 6.2 點號名稱（proto package、MQ exchange 與 routing key、Spring 屬性前綴）")
    W("")
    dn = collect(r"(?<![A-Za-z0-9.\-/@:])(?:" + B + r")(?:\.[a-z][a-z0-9_]*){1,3}(?![A-Za-z0-9_\-])", True)
    tr = []
    for n, fs in sorted(dn.items(), key=lambda kv: -len(kv[1]))[:60]:
        tr.append([f"`{san(n)}`", f"`{re.sub(B, lambda mm: new_for_form(mm.group(0)), n, flags=re.I)}`", len(fs)])
    out += tbl(["名稱（代號化，前 60 個）", "新名", "出現的檔數"], tr)
    W("")
    W(f"共 {len(dn)} 個不同的點號名稱。其中以 `.v1alpha1` 結尾的是對外 API 的 proto package（`v1alpha1` 不改，計畫「不做」）；沒有版本後綴的（`<舊名>.order`、`<舊名>.shipping` 等）是舊 gRPC 的 package 與 RabbitMQ exchange 名，兩者字串相同、語意不同但規則相同（都改前綴）。")
    W("")
    # 6.3 container/network/env/metric
    W("### 6.3 容器、網路、環境變數、指標、URL scheme、儲存鍵")
    W("")
    tr = []
    acc = {
        "容器名（compose `container_name`）": collect(r"container_name:\s*\S*(?:" + B + r")\S*", True, lambda m: m.split(":", 1)[1].strip()),
        "環境變數（大寫前綴）": collect(r"(?<![A-Za-z0-9])" + BU + r"[A-Z0-9_]*"),
        "URL scheme": collect(r"(?:" + B + r")[a-z]*://", True, lambda m: m[:-3]),
        "指標名稱": collect(r"\b(?:" + B + r")_[a-z_]+_(?:total|seconds|bytes|count|days)\b", True),
        "HTTP header": collect(r"(?i)\bx-(?:" + B + r")-[a-z-]+"),
        "compose 網路": collect(r"^\s*(?:" + B + r")-network\s*:?\s*$", True, lambda m: m.strip().rstrip(":").strip()),
    }
    for k, d in acc.items():
        for n, fs in sorted(d.items(), key=lambda kv: -len(kv[1])):
            nn = re.sub(B, lambda mm: new_for_form(mm.group(0)), n, flags=re.I)
            if k == "環境變數（大寫前綴）":
                nn = re.sub(BU, "HAWKR", n)
            if k == "HTTP header":
                nn = re.sub(B, "Hawkr", n, flags=re.I)
            tr.append([k, f"`{san(n)}`", f"`{nn}`", len(fs)])
    out += tbl(["種類", "名稱（代號化）", "新名", "出現的檔數"], tr)
    W("")
    W("其他未列出的 `<舊名>-<後綴>` 形式（`<舊名>-website`、`<舊名>-logo`、`<舊名>-postgres` 等）一律走 A07；`<舊名>-food`、`<舊名>-seller-prod` 等外部名稱走 A02 或步驟 2。")
    W("")
    # 6.4 Java / Maven
    W("### 6.4 Java 套件與 Maven 座標")
    W("")
    poms = sorted(f for f in files if f.endswith("pom.xml"))
    tr = []
    for f in poms:
        t = (ROOT / f).read_text(encoding="utf-8", errors="replace")
        g = re.findall(r"<groupId>([^<]*)</groupId>", t)
        ar = re.findall(r"<artifactId>([^<]*)</artifactId>", t)
        hits = [x for x in g + ar if re.search(B, x, re.I)]
        newv = []
        for x in hits:
            nx = re.sub(r"^com\." + B, D4, x, flags=re.I)
            nx = re.sub(B, lambda mm: new_for_form(mm.group(0)), nx, flags=re.I)
            newv.append(f"`{san(x)}` → `{nx}`")
        tr.append([f"`{san(f)}`", "；".join(sorted(set(newv))) or "（沒有舊名）"])
    W(f"{len(poms)} 個 pom.xml。groupId 與 aggregator 的 artifactId 的對照如下（`<name>` 欄位是顯示名稱，依 A07 的 Title 形狀改）：")
    W("")
    out += tbl(["pom.xml", "座標與新值"], tr)
    W("")
    W("Java 原始碼與測試的 `package`／`import`、`src/main/java/com/<舊名>/…` 目錄一併依 A03、A15 改；`application.yml` 的屬性前綴（K17）、`@ConfigurationProperties(prefix=…)`、`logging.level.com.<舊名>`、MyBatis 的 `mapper-locations` 與 `type-aliases-package` 同步（以測試啟動 Spring context 驗證）。Maven Central 承認 `io.github.<帳號>` 命名空間（D4），所以座標不需要自己的網域。")
    W("")
    # 6.5 bundle ids
    W("### 6.5 App 的 bundle ID 與 applicationId（D4）")
    W("")
    bids = collect(r"(?:merchant\.)?food\.(?:" + B + r")[A-Za-z0-9._]*", True)
    tr = []
    for n, fs in sorted(bids.items(), key=lambda kv: -len(kv[1])):
        nn = re.sub(r"^((?:merchant\.)?)food\." + B, lambda mm: mm.group(1) + D4, n, flags=re.I)
        tr.append([f"`{san(n)}`", f"`{nn}`", len(fs)])
    out += tbl(["現值（代號化）", "新值", "出現的檔數"], tr)
    W("")
    W("- Android 的 `applicationId` 不在追蹤中的檔裡（`git ls-files | grep -i '(^|/)android/'` 是 0）；步驟 3c 建立 Android 設定時直接用 `" + D4 + ".buyer`、`.seller`。")
    W("- App 的 URL scheme（K13）：`<舊名>buyer://`、`<舊名>seller://`、`<舊名>://`，與 iOS `Info.plist` 的 `CFBundleURLSchemes`、Dart 的路由與後端回跳網址同步改。")
    W("- 需要在外部服務重建、不是改檔就能完成的識別碼：Apple Developer 的 App ID 與 Merchant ID、Clerk 的 redirect URL 與 OAuth 應用、Firebase 專案（步驟 2 已移除）。這些不屬於 rename.py。")
    W("")
    # 6.6 proto packages
    W("### 6.6 proto package")
    W("")
    pk = []
    rx_pk = re.compile(r"(?m)^\s*package\s+([A-Za-z0-9_.]+);")
    for f in files:
        if f.endswith(".proto") and not rel_excluded(f):
            t = (ROOT / f).read_text(encoding="utf-8", errors="replace")
            m = rx_pk.search(t)
            if m and re.search(B, m.group(1), re.I):
                pk.append((f, m.group(1)))
    pkgs = collections.defaultdict(list)
    for f, p in pk:
        pkgs[p].append(f)
    tr = [[f"`{san(p)}`", f"`{re.sub(B, lambda mm: new_for_form(mm.group(0)), p, flags=re.I)}`", len(fs), ("對外 API（`v1alpha1` 不改）" if "v1alpha1" in p else "舊 gRPC")] for p, fs in sorted(pkgs.items())]
    W(f"{len(pk)} 個 `.proto` 檔、{len(pkgs)} 個不同的 package。`api/<舊名>/` 目錄改名（第 3 節），`import` 路徑、`buf.yaml` 的 `lint.ignore`／`breaking` 路徑與模組名（K18）同步。")
    W("")
    out += tbl(["package（代號化）", "新 package", "檔數", "類別"], tr)
    W("")
    W("- 線路上的識別碼（`/<package>.<Service>/<Method>`、JSON 的 `@type`）跟著 package 變；`v1alpha1` 的 `buf breaking` 還沒生效（上線前），所以不需要相容。四個生成端與 `api-contract.yml` 的 drift 檢查是驗證手段（步驟 3 驗收）。")
    W("- uitext catalog 的 key 是不帶 package 前綴的 descriptor 路徑（`buyer.Gender.…`），不含舊名；`make uitext` 重新生成即可。")
    W("")

    # 第 7 節 D6
    W("## 7. 縮寫（D6）的邊界規則與撞名風險")
    W("")
    W(f"- 指令：各家族的計數見第 1.2 節 A08 到 A14；檔案分佈與撞名搜尋可用 `{CMD} --count A08` 等。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    W("### 7.1 邊界規則")
    W("")
    W("- **左邊界**：縮寫前面一個字元不是英數字（開頭、空白、引號、`-`、`_`、`.`、`/`、`(`、`:`、`=` 等都算邊界）。這樣 `--<縮寫>-primary`、`text-<縮寫>-muted`、`lib/…/<縮寫>_button.dart`、`.<縮寫>-card` 都會被改，`pdf_`、`pdf-` 不會（前面是字母 `p`）。")
    W("- **右邊界**：連字號或底線本身就是右邊界（A08、A09）；Pascal 與 camelCase 家族要求縮寫後面是大寫字母加小寫字母（`<縮寫Pascal>` 後接 `[A-Z][a-z]`）。")
    W("- **大小寫**：A08、A09 只改小寫；`<縮寫大寫>-` 與 `<縮寫大寫>_`（A10）另外處理，因為出現在資料值。")
    W("- **不改的路徑**：`pnpm-lock.yaml`、`pubspec.lock`、`go.sum`、`atlas.sum`（雜湊與 base64 字串會誤中 A12、A14，而且由工具重算）；Lottie JSON 內的亂數 id（整批在 Q-A7 預設刪除）。")
    W("")
    W("### 7.2 家族與數量")
    W("")
    fam = [("A08", "<縮寫>-"), ("A09", "<縮寫>_"), ("A10", "<縮寫大寫>-／_"), ("A11", "<縮寫>ui-、<縮寫>h-"), ("A12", "<縮寫Pascal>"), ("A13", "<縮寫>Log"), ("A14", "<縮寫camel>")]
    tr = []
    for rid, label in fam:
        r = res.get(rid, [0, 0, []])
        ext = collections.Counter(p.rsplit(".", 1)[-1].lower() if "." in p.split("/")[-1] else "(無)" for p in r[2])
        tr.append([rid, f"`{label}`", r[0], r[1], "、".join(f"{k} {v}" for k, v in ext.most_common(6))])
    out += tbl(["規則", "家族", "檔數", "處數", "檔案類型（前 6）"], tr)
    W("")
    W("### 7.3 撞名與誤中的風險（已找到的）")
    W("")
    W("- 目標字串本身在樹裡已存在：`hk-`、`hk_` 只出現在計畫與 ADR（排除範圍）；`Hk`＋大寫小寫的現有出現都在 Lottie JSON 與 `pnpm-lock.yaml` 的亂數字串裡，不是識別碼，沒有語意撞名。")
    W("- `pdf_`、`pdf-` 一類以縮寫字母結尾的單字：左邊界擋掉，已用全樹搜尋確認 `<縮寫>-`／`<縮寫>_` 的命中前一個字元沒有英數字。")
    W("- 十六進位與 base64：`dfe9d9` 之類的色碼不含連字號或底線，不會被 A08、A09 命中；A12 與 A14 會誤中亂數 id，所以要排除鎖定檔與 Lottie JSON（第 7.1 節）。")
    W("- 單獨出現、後面不接連字號或底線的縮寫字母（51 處）是 Unix 指令或變數名，不改。")
    W("- 大寫的 `<縮寫大寫>-`、`<縮寫大寫>_`：是 HTTP header、ID 格式與測試常數，見第 8 節。")
    W("")

    # 第 8 節 撞名
    W("## 8. 撞名清單（步驟 3 一次處理的清單）")
    W("")
    W("同一個字串在兩處代表不同東西，不能只靠通用規則。D8=A 已決：X1 到 X4 與 X7 全部改成 `HK` 系列，列於下表的連動清單，步驟 3 一次處理，不再是停止條件；步驟 3 的停止條件只適用於改名過程中新發現、不在此表的撞名。")
    W("")
    W(f"- 指令：`{CMD} --count A10`（大寫縮寫）；其餘類型見各列。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    # 計算資料值撞名
    rx_hdr = re.compile(ABBR["<縮寫大寫>-"] + r"(?:Operation|Origin)-[A-Za-z]+")
    rx_idf = re.compile(ABBR["<縮寫大寫>-"] + r"(?:[0-9]{4}-[0-9]{4}|YYYY-NNNN|CONTRACT-[0-9]{4}-[0-9]{4}|[0-9]+|ROUTE-SMOKE|DONE-[0-9]+|LAYOUT-[0-9]+|PA-SPEC-[0-9]+|SELLER-)")
    rxs = {
        "hdr": rx_hdr,
        "idf": rx_idf,
        "ord": re.compile(ABU + r"[0-9]{6,}"),
        "orders-short": re.compile(ABU + r"[0-9]{3,5}(?![0-9])"),
        "food-org": re.compile(r"[A-Za-z0-9_.@/:-]*" + B + r"-food[A-Za-z0-9_.@/:-]*", re.I),
        "sentry": re.compile(BU + r"_[A-Z_]+-[0-9A-Z]{1,3}"),
        "echo-const": re.compile(ABBR["<縮寫大寫>_"] + r"[A-Z_]+"),
    }
    cnts = {}
    for k, rx in rxs.items():
        d = collections.Counter(pth for pth, _ in gg(rx.pattern, bool(rx.flags & re.I)))
        cnts[k] = [sum(d.values()), set(d)]
    tr = [
        ["X1", "`<縮寫大寫>-Operation-Session`、`<縮寫大寫>-Origin-Auth`、`<縮寫大寫>-Operation-Credential`", f"HTTP header 名稱（admin BFF、閘道、單體、測試與設計書）", f"{len(cnts['hdr'][1])} 個檔、{cnts['hdr'][0]} 處", "改成 `HK-…`，所有端同步（線路契約）；兩端任一漏改會讓代行授權失敗", "改（A10）"],
        ["X2", "`<縮寫大寫>-YYYY-NNNN` 格式的代行工作階段編號與 `<縮寫大寫>-<數字>` 的測試值", "資料值的格式（後端產生、CHECK 或正則驗證、測試 fixture）與程式識別碼同一個字串", f"{len(cnts['idf'][1])} 個檔、{cnts['idf'][0]} 處", "格式前綴與驗證正則、CHECK 約束、種子資料同步改；migration 內容會變，要重算 `atlas.sum`", "改（D8=A 已決）；連動：產生碼的格式前綴、驗證正則、CHECK 約束、種子資料與 fixture"],
        ["X3", "`<縮寫大寫><日期><流水號>`（6 位數以上）", "訂單與出貨單編號的前綴：產生碼與大量測試 fixture 都用同一個字串，且不帶連字號，A08 到 A10 不會命中", f"{len(cnts['ord'][1])} 個檔、{cnts['ord'][0]} 處", "前綴換成 `HK`；連動：訂單與出貨單編號的產生碼、驗證正則、CHECK 約束、種子資料與所有 fixture 同步改（本列不帶連字號，A08 到 A10 命中不到，rename.py 要有專屬規則）", "改（D8=A 已決）"],
        ["X4", "`<縮寫大寫>` 加 3 到 5 位數", "測試用編號", f"{len(cnts['orders-short'][1])} 個檔、{cnts['orders-short'][0]} 處", "同 X3 的連動清單（測試編號的 fixture 與斷言）", "改（D8=A 已決）"],
        ["X5", "舊 GitHub 組織名與舊 registry 名稱", "外部擁有的名字（issue 連結、`ghcr.io/…`、`<舊名>-food/…`）", f"{len(cnts['food-org'][1])} 個檔、{cnts['food-org'][0]} 處", "A02 保護；改成通用規則會產生不存在的連結；連結由步驟 6 改成新 repo 的 issue 或拿掉", "不改（A02）"],
        ["X6", "`<舊名>_…-<編號>` 形狀的 Sentry 問題編號", "舊公司 Sentry 專案的外部識別碼（出現在註解與文件）", f"{len(cnts['sentry'][1])} 個檔、{cnts['sentry'][0]} 處", "不是我們的名字；改名會變成不存在的識別碼，應整段註解改寫或刪除（步驟 6）", "刪除註解，不改名"],
        ["X7", "`<縮寫大寫>_ECHO_*` 等測試常數", "測試環境的常數名", f"{len(cnts['echo-const'][1])} 個檔、{cnts['echo-const'][0]} 處", "A10 一併改", "改（A10）"],
        ["X8", "字型、Lottie 與其他有內容雜湊的檔", "manifest 內記錄了檔案的 sha256；改名或重新生成檔案後雜湊不同（詳見下方雜湊綁定）", "見下表", "重新生成並更新 manifest，或整批刪除（Q-A5、Q-A7）", "重新生成或刪除"],
        ["X9", "舊 Apple Merchant ID 與 Clerk、Firebase、OAuth 的外部設定", "外部服務裡以舊名建立的資源（不是檔案）", "-", "改名只改檔案；外部資源要在新帳號重建（第 6.5 節）", "不屬於 rename.py"],
        ["X10", "英文 10 個字元變成 5 個字元", "測試裡以字串長度、欄寬、截斷或排序為前提的斷言（用舊名組出邊界長度）", "無法機械搜尋", "全測試套件跑一次，失敗的逐項看；不是風險「已排除」", "改後跑測試"],
    ]
    out += tbl(["代號", "字串", "兩個意思", "數量", "處理", "去向"], tr)
    W("")
    W("雜湊綁定（檔案內容的 sha256 或 sha1 被別的追蹤檔記錄，內容或檔名變動後會對不上）：")
    W("")
    import hashlib

    refs = collections.defaultdict(set)  # token -> referrer files
    for pth, tok in gg(r"(?<![0-9a-fA-F])(?:[0-9a-f]{64}|[0-9a-f]{40})(?![0-9a-fA-F])"):
        refs[tok].add(pth)
    h2f = {}
    for f in files:
        try:
            b = (ROOT / f).read_bytes()
        except OSError:
            continue
        for algo in ("sha256", "sha1"):
            h = getattr(hashlib, algo)(b).hexdigest()
            if h in refs:
                h2f.setdefault(h, f)
    rx_any = re.compile(B + "|" + "|".join(p for c, p in ABBR.items() if "camel" not in c), re.I)
    pairs = collections.defaultdict(set)
    for tok, referrers in refs.items():
        tgt = h2f.get(tok)
        if not tgt:
            continue
        try:
            tt = (ROOT / tgt).read_bytes().decode("utf-8", errors="replace")
        except OSError:
            tt = ""
        if rx_any.search(tt) or rx_any.search(tgt):
            for r in referrers:
                if r != tgt:
                    pairs[r].add(tgt)
    tr = []
    for ref, tg in sorted(pairs.items()):
        tr.append([f"`{san(ref)}`", len(tg), "字型（Q-A5）" if "fonts" in ref else ("Lottie／角色素材（Q-A1、Q-A7）" if "lottie" in ref else "其他"), "改檔名或內容後重新生成 manifest；或隨素材刪除"])
    out += tbl(["記錄雜湊的檔（代號化）", "被記錄的檔數", "性質", "處理"], tr or [["（無）", 0, "-", "-"]])
    W("")

    # 第 9 節 排除與生成物
    W("## 9. 排除清單與生成物")
    W("")
    W("### 9.1 rename.py 不改內容的檔")
    W("")
    exclude_rows = []
    for f in sorted(files):
        pass
    plan_note = "計畫檔與附件目錄（`scan.py` 同樣排除；計畫的 `<舊名>` 代號保留，隨新 repo 公開）"
    lock_counts = {s: sum(1 for f in files if f.endswith(s)) for s in LOCK_SUFFIX}
    tr = [
        ["計畫與附件", f"`backend/docs/plans/2026-10-24-portfolio-public.md`、`backend/docs/plans/2026-10-24-portfolio-public/`", "-", plan_note],
        ["`.local/`", "不進版控", "-", "明文詞表與搬 issue 的腳本"],
    ]
    for s, c in lock_counts.items():
        how = {
            "pnpm-lock.yaml": "`pnpm install --lockfile-only`（改完 package.json 與 pnpm-workspace.yaml 後）",
            "pubspec.lock": "各套件目錄 `fvm flutter pub get`",
            "go.sum": "各 module `GOWORK=off go mod tidy`（local module 不進 go.sum；有舊名的字串只可能出現在雜湊誤中）",
            "package-lock.json": "`npm install --package-lock-only`（`backend/workers/image-gateway`）",
            "atlas.sum": "各模組 db 目錄 `atlas migrate hash --env dev`（遷移檔內容變了，雜湊要重算；已進 main 的 migration 可改寫，上線前沒有相容性約束）",
        }[s]
        tr.append([f"`{s}`", f"{c} 個檔", "-", how])
    tr.append(["`legacy-endpoints` 規則命中處（A01）", f"{res.get('A01', [0, 0, []])[0]} 個檔、{res.get('A01', [0, 0, []])[1]} 處", "-", "步驟 2 先清到 0；此前 rename.py 的 skip 規則保護"])
    out += tbl(["類別", "範圍或數量", "-", "處理"], tr)
    W("")
    W("### 9.2 生成物與生成器（改來源、重新生成，不用文字取代）")
    W("")
    gens = [
        ("`api/` 的對外 API", "`backend/internal/httpapi/gen/`、`backend/internal/legacy/gen/`（`make connect`）；`packages/<舊名>-api-ts/src/gen/`（`pnpm --filter @hawkr/api gen:api`）；`packages/<舊名>_api/lib/gen/`（`../../api/generate.sh buf.gen.yaml`，先暖機兩個 dart 外掛，見 `AGENTS.local.md`）", "buf 版本看 `api/.buf-version`，PATH 先放 `$HOME/.local/bin`；離線時 BSR 檢查跑不了"),
        ("舊 gRPC 的 proto", "`backend/pkg/proto/**`（Go）與 `backend/services/java/shared/java-proto`（Java）；來源 `backend/api/proto/**`", "`backend/buf.gen.legacy.yaml` 與 `buf.gen.yaml`"),
        ("共用文案目錄", "`backend/internal/platform/uitext/catalog_gen.go`、`packages/<舊名>-ui-text-ts` 的 `*.gen.*`、`packages/<舊名>_ui_text` 的 `*.g.*`、golden 檔", "`make uitext`（`backend/cmd/uitext-gen`）"),
        ("twaddr", "`packages/<舊名>_twaddr/lib/src/twaddr_data.g.dart`、`packages/<舊名>-twaddr-ts/src/twaddr.gen.ts`", "由 `backend/internal/platform/twaddr` 的 CSV 生成"),
        ("法律頁", "`website/features/legal/documents/*.gen.ts`", "由 `website/docs/legal/*.md` 生成（步驟 4 先改寫文件）"),
        ("sqlc 與 go-jet", "各模組 `internal/store`、`internal/jet`、`internal/pubsql`", "`make sqlc`、`make jet`（需要 Docker；`make sqlc` 會先 `rm -rf internal/store`，先 `docker info`）"),
        ("CodeMeta", "Go、go-media、web、admin、flutter、Java 六種輸出", "`generate.sh --all`（Java 產生器不刪舊檔）"),
        ("App 的 build_runner", "`*.g.dart`、`*.freezed.dart`（兩支 App）", "`fvm dart run build_runner build --delete-conflicting-outputs`；CI 的生成檢查"),
        ("website 內容清單", "`website/lib/content/generated/seller-guide-manifest.json`", "改來源後重新生成（內含舊設計工具的檔案識別碼，步驟 6 一併移除）"),
    ]
    out += tbl(["生成物", "輸出位置", "生成指令與注意"], [[g[0], g[1], g[2]] for g in gens])
    W("")

    # 第 10 節
    W("## 10. 重跑與驗證")
    W("")
    W("- `rename.py`（步驟 3）讀 `.local/publish/rename-map.plain.tsv`；先 skip，再依本檔規則表的順序改寫；可重跑，第二次執行 `git status` 必須乾淨。")
    W(f"- 改完以 `{'python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py'} --category brand --english` 驗證英文部分是 0，`--category brand-abbr`、`--category brand-path` 也要是 0（`brand-abbr` 含 D6 沒列出的家族，見 D8=A）。")
    W("- mutation：把一個 proto 檔的 package 改回舊名，四個生成端之一的 drift 檢查與 `scan.py` 都要失敗（計畫步驟 3 驗收）。")
    W("")
    Path(a.out).write_text("\n".join(out) + "\n", encoding="utf-8")
    print(f"wrote {a.out} ({len(out)} lines) and {PLAIN_TSV}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
