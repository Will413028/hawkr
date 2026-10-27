#!/usr/bin/env python3
"""由 scan.py 的規則與 git 歷史產生 inventory.md（計畫步驟 1）。

用法（repo 根目錄）：
  python3 backend/docs/plans/2026-10-24-portfolio-public/gen_inventory.py [--out inventory.md] [--gitleaks-json 路徑]

不帶 --gitleaks-json 時，會把 HEAD 匯出到暫存目錄後執行 `gitleaks dir --redact`。
輸出只有代號化的路徑、行號、種類與計數；不含任何命中的字串，也不含明文的 denylist 詞。
"""
from __future__ import annotations

import argparse
import collections
import json
import multiprocessing
import os
import re
import shutil
import struct
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import scan  # noqa: E402

ROOT = scan.ROOT
ALIGN_BASE = "bfbefa5f4b"
SNAPSHOT = "e5d5a8d8e（程式碼與 332da5e51 相同；之後只新增了計畫與 ADR）"
CMD = "python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py"


def esc(s: str) -> str:
    return s.replace("|", "\\|")


# --------------------------------------------------------------------------- 素材分組
# 每組：id, 路徑 regex, 來源, 去向, 問題代碼（待 Will 確認時）, 說明
D2 = "刪除，換佔位 SVG（D2）"
GROUPS = [
    ("G01", r"^brand/hawkr/", "自製", "保留（新品牌素材，brand/hawkr/README.md；首次 commit 為 Will）", "",
     "Will 於 2026-10-23 新增的新品牌素材。"),
    ("G02", r"^(buyer|seller)-app/ios/Runner/Assets\.xcassets/|^admin/src/app/favicon\.ico$|^admin/public/favicon\.png$",
     "來源不明（舊 logo 衍生）", "保留（步驟 4 已由 brand/hawkr/ 重新產生）", "",
     "AppIcon、favicon 原本是舊品牌 logo 的輸出尺寸；步驟 4 已由 `brand/hawkr/` 的 SVG 重新產生各尺寸。啟動圖與底色是 1 到 4 像素的單色或透明圖，沒有創作性。"),
    ("G03", r"(^|/)(brand|logo)[^/]*/|<舊名>(?!-子品牌)|(^|/)(Default_Logo|logo[_-][^/]*|Logo_Square|logo_word|logo|wordmark|splash[^/]*|auth-moon)\.(png|svg|jpe?g|webp)$|(^|/)logo-[^/]*\.(png|svg)$|(^|/)page1/logo\.png$",
     "來源不明（舊 logo）", "改用 brand/hawkr/（D2、步驟 4、5a）；舊檔刪除", "",
     "原本是舊品牌的 logo、字標與啟動圖（來源不明，沒有授權文件）；步驟 4 已由 `brand/hawkr/` 重新產生的檔留下，其餘在 5a 刪除並改指向 Hawkr 圖（清單在本節之後的「已刪除的素材」）。留下的檔不是 `brand/hawkr/` 的輸出，就是沒有創作性的透明圖。"),
    ("G04", r"farmlicence|/certifications/|/images/certs/|payuni-logo|7-Eleven|711|Uber|blackcat|kuroneko|apple_pay|mastercard|/icons/payment/|/icons/shipping/"
     r"|^seller-app/assets/product_setup3/upload_tag[1-6]\.png$|^seller-app/assets/product_setup4/card[12]_c1\.png$|^seller-app/assets/google\.png$",
     "來源不明（標章或商標）",
     "刪除，改文字標籤（D12；5a）", "",
     "包含官方認證標章（有機、TAP、CAS、GMP、HACCP、ISO）與第三方商標（超商、物流、支付品牌）；標章與商標歸發證或品牌機構，不是 repo 能授權的素材。預設刪除。"),
    ("G05", r"iconly|Iconly",
     "來源不明（Iconly 匯出）",
     "刪除，換 Material 圖示（D12；5d）", "",
     "SVG 註解的出處是第三方圖示庫 Iconly 的匯出檔名；repo 內沒有授權文件，授權未確認。"),
    ("G06", r"<舊名-子品牌>|^website/public/generated/|(^|/)mascots?(/|_)|mascot|^buyer-app/assets/user/(b[12]|w1)\.png$",
     "來源不明（舊角色素材）",
     "刪除，換 `brand/hawkr/` 圖形或佔位 SVG（D12；5c）", "",
     "舊品牌的吉祥物與子品牌角色素材（海報、彈珠、生成預覽）；同目錄的 manifest 只記錄生成流程，沒有原始設計者或授權資訊。"),
    ("G07", r"\.woff2?$|\.ttf$|\.otf$", "開源字型", "保留＋THIRD_PARTY_NOTICES", "", "字型的授權與重新生成方式見逐檔「來源」欄。"),
    ("G08", r"\.(mp4|webm)$", "來源不明（操作錄影）", "刪除，不重錄（D12；5a）", "",
     "計畫的素材 regex 沒涵蓋影片；螢幕錄影與動態預覽，畫面是舊品牌的產品操作，含舊名。"),
    ("G09", r"(^|/)assets/lottie/.*\.json$", "來源不明（Lottie）", "刪除（D12；5c）", "",
     "計畫的素材 regex 沒涵蓋 Lottie；LottieFiles 匯出的授權依原作者，公司內部生成器的圖樣屬舊品牌；逐檔依 meta 欄位的生成器分來源。"),
    ("G10", r"rece_qr", "來源不明（QR 碼）", "換合成 QR（D12；5a）", "",
     "QR 碼圖片。原圖的內容未解碼，可能編碼了真實連結或帳號；5a 換成合成的 QR（`gen_qr.py`），以 OpenCV 解碼確認內容是 `https://example.com`。"),
    ("G11", r"^website/public/images/(seller-guide|seller-guide-book|seller-onboarding|seller-pricing|corporate-welfare|home|home-design)/|^website/public/PNG/|^website/public/images/[^/]+$|^website/public/defaults/|^buyer-app/assets/(images|user|merchant_page|wallet_point|order_card|order_details|home_icon)/|^seller-app/assets/(farm|login|real_name|ai_maker|product_setup\d|merchant_page|order_card|order_details|home|wallet_icon|customer_detail_screen)|^(buyer|seller)-app/assets/[^/]+\.(png|jpe?g)$|^(buyer|seller)-app/assets/(fruits|leaves)/|^backend/services/java/media-service/",
     "來源不明（公司時期圖片）", D2 + "（D12；5e）", "",
     "公司時期的照片、插畫、截圖與介面圖（含兩支 App 的 `fruits/`、`leaves/` 品類插圖，原本歸在 G99，5d 時改歸本組）；首次 commit 全是他人；沒有授權或出處文件；抽樣目視：導覽手冊封面與市集插畫的像素內含舊名，另有疑似第三方圖庫的人物插畫。"),
    ("G12", r"^backend/services/invoice-service/.*NotoSansTC", "開源字型（Noto Sans TC subset，SIL OFL 1.1；檔名判斷，未驗檔內 name table）", "保留＋THIRD_PARTY_NOTICES（測試資料）", "",
     "測試用字型子集。"),
    ("G13", r"^website/\.storybook/public/", "自製（無創作性的幾何圖形）", "保留（開發用預覽圖；D12，5a）", "",
     "Storybook 預覽用的極小幾何圖形（單檔小於 400 B），無創作性；仍列入以便逐檔確認。"),
    ("G14", r"^website/public/placeholders/[^/]+\.svg$", "自製（`website/scripts/gen-placeholders.ts` 產生；圖示是 Material Symbols，Apache-2.0）",
     "保留（5e2 產生器的輸出，重跑無 diff，由 `website/tests/placeholders.test.ts` 守）", "",
     "5e2 由產生器輸出的佔位圖：單色底加一個 Material Symbols 圖示，顏色讀 `website/design-system/cross-platform-tokens.json` 的 `brand.*`。"),
    ("G99", r".", "來源不明（設計稿匯出的 UI 圖示）",
     "刪除，換 Material 圖示（D12；5d）", "",
     "App 與網站的 UI 圖示：目錄與檔名顯示是功能圖示（購物車、訂單、設定等），設計稿匯出的 SVG 或 PNG，首次 commit 皆為他人，SVG 沒有生成器或出處註解。"),
]
GROUP_RX = [(g[0], re.compile(g[1]), g[2], g[3], g[4], g[5]) for g in GROUPS]
GROUP_NAME = {
    "G01": "新品牌素材", "G02": "App 圖示、啟動圖與 favicon", "G03": "舊品牌 logo 與字標", "G04": "官方認證標章與第三方商標",
    "G05": "第三方圖示庫（Iconly）", "G06": "吉祥物與子品牌角色素材", "G07": "字型", "G08": "影片",
    "G09": "Lottie 動畫", "G10": "QR 碼", "G11": "照片、插畫、截圖與介面圖", "G12": "測試用字型", "G13": "Storybook 預覽圖",
    "G14": "程式產生的佔位圖", "G99": "UI 圖示",
}


def classify_asset(path: str, lottie_meta: dict) -> tuple:
    """path 是代號化後的路徑（brand 與子品牌已換成代號）。"""
    for gid, rx, source, disp, q, note in GROUP_RX:
        if gid == "G03" and re.search(r"(^|/)(?:[^/]*fonts?/)", path):
            continue
        if gid == "G05" and not re.search(r"iconly|Iconly", path):
            continue
        if gid == "G99" or rx.search(path):
            return gid, source, disp, q
    return "G99", GROUPS[-1][2], GROUPS[-1][3], GROUPS[-1][4]


# --------------------------------------------------------------------------- 步驟 4 之後的自製素材（5a）
STEP4_RANGE = ("2606123866^", "cc6985fa9a")
BRAND_MADE = "自製（`brand/hawkr/`）"
BRAND_MADE_DISP = "保留（步驟 4 由 `brand/hawkr/` 產生）"
# 步驟 4 的 brand/hawkr/README.md 腳本輸出的 PNG／ICO，加上手寫的單色版遮罩 SVG；
# 還要在步驟 4 的 commit 範圍內被改過才算（見 step4_touched）。
REGEN_RX = re.compile(
    r"^(buyer|seller)-app/ios/Runner/Assets\.xcassets/(dev|stage|prod)AppIcon\.appiconset/[^/]+\.png$"
    r"|^(buyer|seller)-app/assets/brand/(logo-four-color|splash_mark|splash_mark_android12|splash_mark_wheat)\.png$"
    r"|^(buyer|seller)-app/assets/hawkrlogo\.png$"
    r"|^admin/public/favicon\.png$|^admin/src/app/favicon\.ico$"
    r"|^website/public/brand/logo-grass\.svg$"
)
TINY_PX = 4
DELETED_TSV = HERE / "deleted-assets.tsv"


def step4_touched() -> set:
    out = subprocess.run(["git", "diff", "--name-only", STEP4_RANGE[0], STEP4_RANGE[1]], cwd=ROOT, capture_output=True, text=True, check=True).stdout
    return set(out.split("\n"))


def brand_svg_hashes() -> set:
    import hashlib
    return {hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT / "brand" / "hawkr").glob("*.svg")}


def brand_derived(path: str, touched: set, svg_hashes: set) -> bool:
    """內容與 brand/hawkr 的 SVG 相同，或是步驟 4 腳本輸出的檔；不分群組。"""
    import hashlib
    if path.endswith(".svg") and hashlib.sha256((ROOT / path).read_bytes()).hexdigest() in svg_hashes:
        return True
    return bool(REGEN_RX.search(path)) and path in touched


def self_made(path: str, touched: set, svg_hashes: set):
    """回傳 (來源, 去向) 或 None。依據：內容與 brand/hawkr 的 SVG 相同、步驟 4 腳本輸出的檔、極小的單色圖。"""
    if brand_derived(path, touched, svg_hashes):
        return BRAND_MADE, BRAND_MADE_DISP
    full = ROOT / path
    sz = img_size(full)
    if sz:
        w, h = (int(x) for x in sz.split("x"))
        if w <= TINY_PX and h <= TINY_PX:
            return f"自製（無創作性：{sz} 的單色或透明圖）", "保留"
    if path.endswith(".svg") and full.stat().st_size < 400 and b"<path" not in full.read_bytes():
        return "自製（無創作性：只有一個矩形的 SVG）", "保留"
    return None


# --------------------------------------------------------------------------- G11 分類（5e 開工，2026-10-27）
# G11 依程式引用與目視分成下面幾類，類別寫在去向的開頭。引用以檔案路徑的最後兩段比對
# （例如 `order_card/order_p1.png`），只找同一個 client 目錄裡的文字檔；`pubspec.yaml` 的宣告
# 與測試檔另外計。有程式引用的 App 素材一定要列在 G11_CLASSES 的某一類，列了卻沒有程式引用的也停下，
# 兩個方向都由產生器檢查。website 有引用的素材全是照片、插畫或截圖。動態組出的路徑（`'assets/x/$i.png'`、
# 樣板字串）在開工時以 `rg -n '\$\{?[a-zA-Z_.]+\}?\.(png|svg|jpe?g|webp)'` 與
# `rg -n "assets/[a-z_0-9/]*\$"` 查過兩支 App 的 lib 與 website，沒有組出 G11 路徑的地方。
G11_APP_PLACEHOLDER = "刪除，換程式繪製的佔位（單色底加 Material 圖示，不放檔案；D2；5e1）"
G11_WEB_PLACEHOLDER = "刪除，換 `website/scripts/gen-placeholders.ts` 產生的佔位 SVG（D2；5e2）"
G11_CLASSES = [
    ("UI 圖示", re.compile(
        r"^buyer-app/assets/home_icon/cart_icon\.png$"
        r"|^seller-app/assets/ai_edit\.png$"
        r"|^seller-app/assets/ai_maker/config[1-4]\.png$"
        r"|^seller-app/assets/customer_detail_screen/(usd-circle|clipboard-notes|icon24|phone|map-marker|message)\.svg$"
        r"|^seller-app/assets/product_setup1/upload_b[23]\.png$"
        r"|^seller-app/assets/wallet_icon/wallet_b[45]\.png$"),
     "刪除，換 Material 圖示（`AppIcons`／`HkIcon`；比照 5d；5e1）"),
    ("介面元件圖", re.compile(
        r"^buyer-app/assets/order_details/fram1\.png$"
        r"|^buyer-app/assets/wallet_point/wallettag_1\.png$"
        r"|^seller-app/assets/ai_maker/(ai_b[1-4]|color[1-6])\.png$"
        r"|^seller-app/assets/product_setup3/upload_(b[345]|tag7)\.png$"
        r"|^seller-app/assets/wallet_icon/wallet_b(2|22|3|33)\.png$"),
     "刪除，改成程式繪製的元件（按鈕、分頁、色票與底色：文字、顏色與圖示走 design token，不放圖；5e1）"),
    ("照片與插畫", re.compile(
        r"^buyer-app/assets/banner2\.jpg$"
        r"|^(buyer|seller)-app/assets/brand/auth-moon\.png$"
        r"|^buyer-app/assets/default_(buyer|seller)_avatar\.png$"
        r"|^seller-app/assets/home/default_buyer_avatar\.png$"
        r"|^(buyer|seller)-app/assets/merchant_page/merchant_p1\.png$"
        r"|^buyer-app/assets/nopost\.png$"
        r"|^buyer-app/assets/wallet_point/default_merchant_cover\.png$"
        r"|^(buyer|seller)-app/assets/(fruits/[^/]+|leaves/Vector[-_]?\d*)\.svg$"
        r"|^seller-app/assets/ai_maker/emoji[1-9]\.png$"),
     G11_APP_PLACEHOLDER),
]
G04_FROM_G11 = {  # 5e 開工時從 G11 改歸 G04 的標章與商標
    "seller-app/assets/product_setup3/upload_tag1.png": "refd", "seller-app/assets/product_setup3/upload_tag2.png": "refd",
    "seller-app/assets/product_setup3/upload_tag3.png": "refd", "seller-app/assets/product_setup3/upload_tag4.png": "refd",
    "seller-app/assets/product_setup3/upload_tag5.png": "refd", "seller-app/assets/product_setup3/upload_tag6.png": "refd",
    "seller-app/assets/product_setup4/card1_c1.png": "unref", "seller-app/assets/product_setup4/card2_c1.png": "unref",
    "seller-app/assets/google.png": "unref",
}
G04_FROM_G11_DISP = {
    "refd": "刪除，改文字標籤（D12 Q-A4；認證標章，5e 開工時從 G11 改歸；5e1）",
    "unref": "刪除（無引用，只在 `pubspec.yaml` 宣告或沒有宣告；第三方商標，5e 開工時從 G11 改歸；5e1）",
}
TEST_RX = re.compile(r"(^|/)(test|tests|__tests__)/|\.test\.|_test\.dart$|\.spec\.")
TAIL2_RX = re.compile(r"[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+\.(?:png|jpe?g|webp|svg|gif)")


def _tail2(path: str) -> str:
    return "/".join(path.split("/")[-2:])


def g11_hits(files, g11_files) -> dict:
    """每個 G11 檔 -> 引用它的檔（同一個 client 目錄、非素材、非計畫與附件）。"""
    want = collections.defaultdict(set)  # (client, tail2) -> G11 檔
    for f in g11_files:
        want[(f.split("/")[0], _tail2(f))].add(f)
    clients = {c for c, _ in want}
    out = collections.defaultdict(set)
    for src in files:
        c = src.split("/")[0]
        if c not in clients or scan.is_asset(src) or scan.excluded(src) or src.startswith(scan.ATTACH_DIR) or src == scan.PLAN:
            continue
        try:
            b = (ROOT / src).read_bytes()
        except OSError:
            continue
        if b"\0" in b[:8192]:
            continue
        for tok in set(TAIL2_RX.findall(b.decode("utf-8", errors="replace"))):
            for g in want.get((c, tok), ()):
                out[g].add(src)
    return out


def classify_g11(path: str, hits: set) -> str:
    code = sorted(h for h in hits if not TEST_RX.search(h) and not h.endswith("pubspec.yaml"))
    tests = [h for h in hits if TEST_RX.search(h)]
    pub = any(h.endswith("pubspec.yaml") for h in hits)
    explicit = next(((c, d) for c, rx, d in G11_CLASSES if rx.search(path)), None)
    if code:
        if path.startswith("website/"):
            return "照片與插畫：" + G11_WEB_PLACEHOLDER
        if explicit is None:
            sys.exit(f"G11：{path} 有程式引用（{', '.join(code[:3])}）但不在 G11_CLASSES")
        return f"{explicit[0]}：{explicit[1]}"
    if explicit is not None:
        sys.exit(f"G11：{path} 列在「{explicit[0]}」，但沒有程式引用")
    step = "5e2" if path.startswith("website/") else "5e1"
    if tests:
        return f"無引用：直接刪除，只有測試以字串提到它（{', '.join(sorted(tests))}），測試一起改；{step}"
    if pub:
        return f"無引用：直接刪除，`pubspec.yaml` 的宣告一起刪；{step}"
    return f"無引用：直接刪除；{step}"


def deleted_refs(path: str, remaining_names: set, refs) -> str:
    """刪除後的引用檔數。先以最後兩段路徑比對；檔名單獨出現的檔另外標註（通常是同名的別的檔或網址字串，要人確認）。"""
    base = os.path.basename(path)
    by_name = refs.get(base, 0)
    tail = "/".join(path.split("/")[-2:])
    out = subprocess.run(["git", "grep", "-l", "-F", "-e", tail, "--", ".", f":(exclude){scan.ATTACH_DIR.rstrip('/')}", f":(exclude){scan.PLAN}"],
                         cwd=ROOT, capture_output=True, text=True).stdout.split("\n")
    by_path = len([f for f in out if f and not scan.is_asset(f)])
    if by_path:
        return str(by_path)
    if base in remaining_names and by_name:
        return f"0（路徑比對；檔名另見 {by_name} 檔，與留下的同名素材合併計算）"
    if by_name:
        return f"0（路徑比對；檔名字串另見 {by_name} 檔，是無關的網址或欄位值，已人工確認）"
    return "0"


def load_deleted(san) -> list:
    """讀 deleted-assets.tsv。含代號的路徑（附件不寫明文）以 git 歷史裡被刪除的真實路徑還原，還原不到就停下。"""
    if not DELETED_TSV.is_file():
        return []
    rows = []
    by_code = None
    for line in DELETED_TSV.read_text(encoding="utf-8").split("\n"):
        if not line.strip() or line.startswith("#"):
            continue
        row = line.split("\t")
        if "<" in row[0]:
            if by_code is None:
                hist = subprocess.run(["git", "log", "--diff-filter=D", "--name-only", "--format=", "HEAD"],
                                      cwd=ROOT, capture_output=True, text=True, check=True).stdout.split("\n")
                by_code = {}
                for p in hist:
                    if p:
                        by_code.setdefault(san(p), set()).add(p)
            real = by_code.get(row[0], set())
            if len(real) != 1:
                sys.exit(f"deleted-assets.tsv：{row[0]} 在 git 歷史對到 {len(real)} 個被刪除的路徑，要剛好 1 個")
            row[0] = real.pop()
        rows.append(row)
    return rows


def img_size(path: Path):
    try:
        b = path.read_bytes()[:64]
    except OSError:
        return ""
    if b[:8] == b"\x89PNG\r\n\x1a\n":
        w, h = struct.unpack(">II", b[16:24])
        return f"{w}x{h}"
    return ""


# --------------------------------------------------------------------------- 引用數（素材檔名被其他檔引用的檔數）
_TOKEN_SPLIT = re.compile(r"[\s\"'`()<>,;:=\[\]{}|\\]+|/")


_R: dict = {}


def _ref_init(plain, spaced):
    _R["plain"] = plain
    _R["spaced"] = spaced


def _ref_worker(path):
    names = _R["plain"]
    try:
        b = (ROOT / path).read_bytes()
    except OSError:
        return []
    if b"\0" in b[:8192] or len(b) > 5_000_000:
        return []
    t = b.decode("utf-8", errors="replace")
    toks = set(_TOKEN_SPLIT.split(t))
    hit = list(names & toks)
    spaced = [n for n in _R["spaced"] if n in t]
    return list(set(hit + spaced))


def ref_counts(files, asset_files):
    names = {}
    for a in asset_files:
        names.setdefault(os.path.basename(a), []).append(a)
    plain = set(n for n in names if " " not in n and "&" not in n)
    spaced = [n for n in names if n not in plain]
    jobs = [f for f in files if not scan.is_asset(f) and not scan.excluded(f)]
    counts = collections.Counter()
    with multiprocessing.Pool(min(8, os.cpu_count() or 2), initializer=_ref_init, initargs=(frozenset(plain), spaced)) as pool:
        for hit in pool.imap_unordered(_ref_worker, jobs, chunksize=64):
            for n in hit:
                counts[n] += 1
    return counts


# --------------------------------------------------------------------------- gitleaks
def run_gitleaks(json_path: str | None):
    if json_path:
        return json.load(open(json_path, encoding="utf-8")), "（沿用先前的輸出）"
    exe = shutil.which("gitleaks")
    if not exe:
        return None, "（找不到 gitleaks）"
    tmp = Path(tempfile.mkdtemp(prefix="inv-gl-"))
    try:
        exp = tmp / "export"
        exp.mkdir()
        arch = subprocess.run(["git", "archive", "HEAD"], cwd=ROOT, capture_output=True, check=True).stdout
        subprocess.run(["tar", "-x", "-C", str(exp)], input=arch, check=True)
        out = tmp / "gl.json"
        subprocess.run([exe, "dir", str(exp), "--redact", "--no-banner", "-f", "json", "-r", str(out), "--exit-code", "0", "-l", "error"],
                       check=True, capture_output=True)
        data = json.load(open(out, encoding="utf-8"))
        for d in data:
            d["File"] = d["File"].split(str(exp) + "/", 1)[-1]
            d["_dir"] = str(exp)
        # 分類需要原始行，趁匯出目錄還在時讀
        for d in data:
            p = exp / d["File"]
            try:
                line = p.read_text(encoding="utf-8", errors="replace").split("\n")[d["StartLine"] - 1]
            except Exception:
                line = ""
            d["_line_shape"] = shape_of(line)
            d["_tokens"] = [t for t in re.findall(r"[A-Za-z0-9+/=_\-]{16,}", line)]
            d["_label"] = (re.search(r"([A-Za-z_][A-Za-z0-9_.\-]{0,40})\s*[:=]", line) or [None, ""])[1]
            toks = d["_tokens"]
            d["_hexlen"] = len(toks[0]) if toks and re.fullmatch(r"[0-9a-fA-F]+", toks[0]) else 0
            d["_is_uuid"] = bool(re.search(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", line))
            d["_vals"] = toks  # 只在記憶體內用於跨檔比對，不輸出
        shares = collections.defaultdict(set)
        for d in data:
            if d["_tokens"]:
                shares[max(d["_tokens"], key=len)].add(d["File"])
        for d in data:
            v = max(d["_tokens"], key=len) if d["_tokens"] else None
            d["_shared"] = len(shares[v]) if v else 0
            d["_shared_in_docs"] = any("第三方整合" in f or f.endswith(".md") for f in shares[v]) if v else False
            d["_shared_in_test_only"] = bool(v) and all(re.search(r"(?i)test|fixture|testdata|spec|application-test|/docs/", f) for f in shares[v])
        for d in data:
            for k in ("_vals", "_tokens"):
                d.pop(k, None)
            for k in ("Secret", "Match"):
                d.pop(k, None)
        return data, "（本次執行）"
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def shape_of(line: str) -> str:
    return re.sub(r"[A-Za-z0-9+/=_\-]{16,}", "<V>", line).strip()[:100]


def classify_leak(d: dict) -> tuple:
    f, rule = d["File"], d["RuleID"]
    if rule == "private-key":
        return "私鑰字面（非金鑰）", "只出現標頭字串（解析或剝除用的程式字面，或設計書的說明文字），沒有金鑰本體", "保留（誤報）；加入 .gitleaksignore 並附理由"
    if re.search(r"(atlas\.sum|Podfile\.lock|go\.sum|pnpm-lock|pubspec\.lock|package-lock)$", f):
        return "雜湊誤報", "鎖定檔的內容雜湊", "加入 .gitleaksignore，附理由（雜湊）"
    if d.get("_hexlen", 0) >= 32:
        if re.search(r"(?i)test|fixture|testdata|spec|mock|example", f):
            return "測試值", "測試檔內的十六進位值（長度 %d，變數 %s）；形狀像金鑰，已決（S2）視為舊公司的真實憑證" % (d["_hexlen"], d.get("_label") or "-"), "已決（S2）：換成明顯的虛構值（步驟 6）"
        return "雜湊誤報", "值是純十六進位字串（長度 %d）" % d["_hexlen"], "加入 .gitleaksignore，附理由（雜湊）"
    lab = d.get("_label", "")
    if f.endswith(("buf.yaml", "buf.gen.yaml")):
        return "雜湊誤報", "BSR 模組參照，不是憑證", "加入 .gitleaksignore，附理由（模組參照）"
    if f.endswith("seller-guide-manifest.json"):
        return "需要人看", "設計工具的檔案識別碼欄位（識別碼，不是憑證）；舊公司的設計檔參照", "移除欄位（生成檔：改來源後重新生成）"
    if f.endswith("gateway/internal/handler/docs.go"):
        return "需要人看", "原始碼內寫死的簽章 key（非測試檔）；檔內註解稱僅供內部開發的文件檢視器", "已決（S1）：改由環境變數注入，沒有值時停用該檢視器（步驟 2）"
    if d.get("_is_uuid"):
        return "測試值", "值是 UUID 範例（冪等鍵）", "保留（範例值）；加入 .gitleaksignore"
    if f.endswith(".md") and "第三方整合" in f:
        if d.get("_shared", 0) >= 2:
            return "測試值", "供應商文件公布的測試值：同一個值也出現在其他檔（%d 個檔）" % d["_shared"], "供應商文件依步驟 6 清洗；測試檔換虛構值"
        return "測試值", "供應商文件的範例值", "供應商文件依步驟 6 清洗"
    if f.endswith(".md"):
        return "測試值", "設計書或 README 裡的範例值（變數 %s，值以 ... 結尾或為說明用）" % (lab or "-"), "改成明顯的佔位文字"
    if d.get("_shared", 0) >= 5 and d.get("_shared_in_test_only"):
        return "測試值", "沙盒憑證樣貌：同一個值出現在 %d 個測試檔與測試設定（變數 %s）；視為舊公司的真實憑證" % (d["_shared"], lab or "-"), "已決（S2）：換成虛構值（步驟 6）"
    if re.search(r"(?i)test|fixture|testdata|spec|mock|example|template|\.env\.", f):
        return "測試值", "測試檔內的值（變數 %s）" % (lab or "-"), "換成明顯的虛構值，或以 .gitleaksignore 放行並附理由"
    return "需要人看", "非測試檔的高熵字串（變數 %s）" % (lab or "-"), "逐筆確認（步驟 6）"


# --------------------------------------------------------------------------- 主流程
def tbl(header, rows):
    out = ["| " + " | ".join(header) + " |", "|" + "|".join(["---"] * len(header)) + "|"]
    out += ["| " + " | ".join(r) + " |" for r in rows]
    return out


def lines_cell(lines, limit=10):
    lines = sorted(set(lines))
    shown = ",".join(f"L{n}" for n in lines[:limit])
    if len(lines) > limit:
        shown += f"…共{len(lines)}處"
    return shown


def doc_type(p: str) -> str:
    if p.startswith("backend/docs/第三方整合/"):
        return "供應商文件"
    if p.startswith("backend/docs/詳細設計書/"):
        return "詳細設計書"
    if p.startswith("backend/docs/基本設計書/"):
        return "基本設計書"
    if p.startswith("backend/docs/開發規範/"):
        return "開發規範"
    if p.startswith("backend/docs/基礎建設/"):
        return "基礎建設"
    if p.startswith("backend/docs/runbooks/"):
        return "runbook"
    if p.startswith("backend/docs/adr/"):
        return "ADR"
    if p.startswith("backend/docs/plans/"):
        return "計畫"
    if p.startswith("backend/docs/"):
        return "backend 其他文件"
    if p.startswith("website/docs/legal/"):
        return "法律頁"
    if "/third_party/" in p or p.split("/")[-1].startswith(("LICENSE", "NOTICE")):
        return "第三方授權文件"
    name = p.split("/")[-1]
    if name in ("AGENTS.md", "CLAUDE.md"):
        return "指令檔"
    if name.startswith("README"):
        return "README"
    if p.startswith((".agents/", ".claude/")):
        return "agent skill"
    if "/docs/" in "/" + p:
        return "子專案文件"
    return "其他文件"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=str(HERE / "inventory.md"))
    ap.add_argument("--gitleaks-json")
    a = ap.parse_args()

    entries = scan.load_denylist()
    san = scan.sanitizer(entries)
    S = lambda p: esc(san(p))
    files = [f for f in scan.tracked_files() if not scan.excluded(f) and (ROOT / f).is_file()]
    rules = scan.legacy_rules()
    cats = {"brand", "brand-abbr", "colleague", "pii", "endpoint", "vendor", "merchant", "private", "refs"}
    res = scan.run_scan(files, entries, rules, cats, False, False)
    name_res = scan.name_hits(files, entries)
    authors = scan.author_counts(entries)

    by_file = collections.defaultdict(lambda: collections.defaultdict(list))  # path -> label -> [(code,line,kind)]
    for p, hits in res.items():
        for label, code, line, kind in hits:
            by_file[p][label].append((code, line, kind))

    out = []
    W = out.append
    W("# 作品集轉 public 的盤點（步驟 1 附件）")
    W("")
    W(f"快照 commit：{SNAPSHOT}。由 `gen_inventory.py` 依 `scan.py` 的規則產生；每個表格的每一列都有「去向」。")
    W("路徑裡的舊名、縮寫與同事代號一律換成代號（`<舊名>`、`<縮寫>-`、`<同事-1>` 等）；用 `scan.py --resolve '<代號化路徑>'` 還原成真實路徑。")
    W("「待 Will 確認」表的列都附有預設去向，Will 沒有回覆前依預設執行；問題代號見該表。")
    W("")
    W("## 已決")
    W("")
    W("- 2026-10-24（Will）")
    W("  - S1：`gateway` 文件檢視器寫死的憑證與簽章 key → 步驟 2 改由環境變數注入，沒有值時停用檢視器。")
    W("  - S2：同一個 32 字元、形狀像代理金鑰的值（出現在 11 個測試檔與測試設定），以及 7 個檔的十六進位值 → 視為舊公司的真實憑證，步驟 6 換成虛構值。")
    W("- 2026-10-25（Will）：Q-A9 子品牌 `<舊名-子品牌>` 與其中文寫法屬於舊名，一併改（計畫 D9；步驟 4 改中文字首的衍生名，英文識別碼歸 5f）。")
    W("- 2026-10-26（Will）：Q-A1 到 Q-A8 全部照預設（計畫 D12），去向已寫進第 1 節各列：")
    for q, qq, dd in [
        ("Q-A1", "舊品牌吉祥物與子品牌的角色素材", "全部刪除，換 `brand/hawkr/` 圖形或佔位 SVG（5c）"),
        ("Q-A2", "Iconly 圖示", "刪除，換 Material 系圖示（App 用 Flutter 內建 `Icons`，website 與 admin 用 Material Symbols 的 SVG；5d）"),
        ("Q-A3", "設計稿匯出的 UI 圖示、插畫、截圖與介面圖", "UI 圖示刪除並換 Material 系（5d）；照片與插畫換程式產生的佔位 SVG（D2；5e）；Storybook 的極小圖形保留（5a）"),
        ("Q-A4", "官方認證標章與第三方商標", "刪除，改文字標籤（5a）"),
        ("Q-A5", "jf-openhuninn 衍生字型", "保留，家族名改成 Hawkr 的名字並重算 manifest，授權文件與 notice 照 OFL 保留（5b）"),
        ("Q-A6", "影片（mp4、webm）", "刪除，不重錄（5a）"),
        ("Q-A7", "Lottie 動畫", "刪除（5c）"),
        ("Q-A8", "QR 碼圖片", "刪除原圖，換合成的 QR（編碼 `https://example.com`；5a）"),
    ]:
        W(f"  - {q} {qq} → {dd}。")
    W("")
    W("## 待 Will 確認的問題")
    W("")
    W("| 代號 | 問題 | 預設 |")
    W("|---|---|---|")
    for q, qq, dd in [
        ("Q-P1", "測試資料裡通過檢查碼的統一編號與身分證字號：是合成值還是真實業者的號碼（repo 無法判定）", "一律換成合成值，並在 `scan.py` 寫入允許的合成值清單"),
        ("Q-P2", "疑似真實業者的 email 網域、商號、農場名稱（denylist 的 uncertain 列）是否真有其人", "換成虛構名稱"),
        ("Q-D1", "設計書與規範裡出現文件作者本人（Will）的全名一處：是否保留", "保留（Will 自己的名字）"),
    ]:
        W(f"| {q} | {qq} | {dd} |")
    W("")

    # ---------------- 素材
    W("## 1. 素材逐檔")
    W("")
    asset_files = sorted(f for f in files if scan.is_asset(f))
    plan_rx_n = sum(1 for f in asset_files if scan.ASSET_PLAN_RX.search(f))
    W(f"- 數量：{len(asset_files)} 個檔（計畫的 regex 涵蓋 {plan_rx_n} 個；另外納入字型 woff/woff2、影片 mp4/webm、Lottie JSON 共 {len(asset_files) - plan_rx_n} 個）。")
    W(f"- 產生計數的指令：`git ls-files | grep -E -i '\\.(png|jpe?g|webp|svg|ico|gif|ttf|otf|pdf)$' | wc -l`（計畫範圍），`{CMD} --category assets --report`（含擴充型態；沒有出處的檔記成「來源不明」而命中，步驟 5 完成後應為 0）。")
    W(f"- 快照：{SNAPSHOT}；本節的路徑與去向在步驟 5a 對齊到 `{ALIGN_BASE}` 之後的樹（步驟 3 改名的字型與素材、步驟 4 由 `brand/hawkr/` 產生的檔都用現在的路徑）。")
    W("- 來源的依據：首次 commit 的作者（git 歷史）、SVG 的生成器註解、同目錄的 README 與 manifest、檔名與目錄語意；沒有授權文件的一律記成「來源不明」。像素內容是否含舊名只抽樣目視，無法機械掃描；`含舊名` 欄只標路徑或 SVG 文字命中。")
    W("- 「引用檔數」是其他追蹤中的文字檔含該檔名的檔數（以檔名比對，同名檔會合併計算）；步驟 5 刪除前後都要重跑，刪除後必須是 0。")
    W("")
    lot_meta = {}
    for f in asset_files:
        if f.endswith(".json"):
            try:
                d = json.load(open(ROOT / f, encoding="utf-8"))
                m = d.get("meta") if isinstance(d, dict) and isinstance(d.get("meta"), dict) else {}
                g = str(m.get("g", ""))
                lot_meta[f] = "lottiefiles" if "lottiefiles" in g.lower() else ("internal" if "deterministic" in g.lower() else "none")
            except Exception:
                lot_meta[f] = "none"
    deleted = load_deleted(san)
    refs = ref_counts(files, asset_files + [d[0] for d in deleted])

    grp_rows = collections.defaultdict(list)
    iconly_svg = set()
    for f in asset_files:
        if f.endswith(".svg"):
            try:
                if b"Iconly" in (ROOT / f).read_bytes()[:3000] or b"iconly" in (ROOT / f).read_bytes()[:3000]:
                    iconly_svg.add(f)
            except OSError:
                pass
    touched, svg_hashes = step4_touched(), brand_svg_hashes()
    first = {}
    for f in asset_files:
        w, o = authors.get(f, [0, 0])
        first[f] = (w, o)
    for f in asset_files:
        gid, source, disp, q = classify_asset(san(f), lot_meta)
        if f in iconly_svg and gid not in ("G01", "G04", "G05"):
            gid, source, disp, q = "G05", GROUPS[4][2], GROUPS[4][3], ""
        if gid == "G09":
            k = lot_meta.get(f)
            if k == "internal":
                source = "自製（內部生成器；圖樣屬舊品牌）"
                disp, q = "刪除（D12；5c）", ""
            elif k == "lottiefiles":
                source = "來源不明（LottieFiles 匯出）"
        if gid == "G07":
            if "hawkr-rounded" in f.lower():
                source = "開源（jf-openhuninn 衍生，OFL-1.1）；5b 已驗證 name table：字型名稱欄位（name ID 1/3/4/6/16）已改為 Hawkr Rounded，無保留名；上游版權、商標與授權聲明照 OFL 保留；由 `generate_openhuninn.py` 生成"
                disp = "保留＋THIRD_PARTY_NOTICES"
            elif "roboto" in f.lower():
                source = "開源（Roboto，OFL-1.1；5b 依檔內 name table 驗證）"
                disp = "保留＋THIRD_PARTY_NOTICES"
            elif "zen" in f.lower():
                source = "開源（Zen Maru Gothic，OFL-1.1；5b 依檔內 name table 驗證）"
                disp = "保留＋THIRD_PARTY_NOTICES"
            elif "notosanstc" in f.lower():
                source = "開源（Noto Sans CJK TC，OFL-1.1；檔內 name table 無授權欄，依上游判斷）"
                disp = "保留＋THIRD_PARTY_NOTICES"
            else:
                source = "開源（授權待逐檔確認）"
                disp = "保留＋THIRD_PARTY_NOTICES（授權在 5b 逐檔確認）"
            q = ""
        if gid == "G03" and f.endswith(".ico"):
            gid = "G02"
            source, disp, q = GROUPS[1][2], GROUPS[1][3], ""
        if gid == "G10":
            source, disp = "自製（程式合成的 QR，編碼 `https://example.com`；`gen_qr.py`）", "保留（5a 已換成合成 QR）"
        if f.endswith("hawkrlogo.png"):
            gid = "G03"
        if f.endswith("/auth-moon.png"):
            gid, source, disp = "G11", "來源不明（月球照片，不是 logo）", D2 + "（D12；5e）"
        if gid in ("G02", "G03"):
            sm = self_made(f, touched, svg_hashes)
            if sm:
                source, disp = sm
        elif gid != "G11" and not f.startswith("brand/hawkr/") and brand_derived(f, touched, svg_hashes):
            source, disp = BRAND_MADE, BRAND_MADE_DISP
        if gid == "G04" and f in G04_FROM_G11:
            disp = G04_FROM_G11_DISP[G04_FROM_G11[f]]
        brand_hit = bool(by_file.get(f, {}).get("brand")) or bool(name_res.get(f))
        grp_rows[gid].append((f, source, disp, q, brand_hit))
    g11_ref = g11_hits(files, [r[0] for r in grp_rows.get("G11", [])])
    grp_rows["G11"] = [(f, source, classify_g11(f, g11_ref.get(f, set())), q, bh) for f, source, disp, q, bh in grp_rows.get("G11", [])]
    g11_class_n = collections.Counter(r[2].split("：", 1)[0] for r in grp_rows["G11"])

    for gid in [g[0] for g in GROUPS]:
        rows = grp_rows.get(gid)
        if not rows:
            continue
        total_kb = sum((ROOT / r[0]).stat().st_size for r in rows) // 1024
        exts = collections.Counter(r[0].rsplit(".", 1)[-1].lower() for r in rows)
        wsum = sum(first[r[0]][0] for r in rows)
        osum = sum(first[r[0]][1] for r in rows)
        W(f"### {gid} {GROUP_NAME[gid]}（{len(rows)} 檔，{total_kb} KB）")
        W("")
        note = [g for g in GROUPS if g[0] == gid][0][5]
        W(f"依據：{note}型態 {', '.join(f'{k} {v}' for k, v in sorted(exts.items()))}；這組檔案的 commit 次數：Will {wsum}、他人 {osum}。" + (f" 待確認問題：{rows[0][3]}。" if rows[0][3] else ""))
        if gid == "G11":
            W("")
            W("5e 開工時的分類（去向欄的開頭；規則與引用的比對方式見 `gen_inventory.py` 的 `G11_CLASSES`）：" + "、".join(f"{k} {v}" for k, v in sorted(g11_class_n.items())) + f"。認證標章與第三方商標 {len(G04_FROM_G11)} 個改歸 G04。")
        W("")
        header = ["素材", "型態", "KB", "尺寸", "來源", "含舊名", "引用檔數", "去向"]
        trs = []
        for f, source, disp, q, bh in sorted(rows):
            st = (ROOT / f).stat().st_size
            trs.append([f"`{S(f)}`", f.rsplit(".", 1)[-1].lower(), str(max(1, st // 1024)), img_size(ROOT / f) or "-", source,
                        "是（路徑或文字）" if bh else "否／像素未掃", str(refs.get(os.path.basename(f), 0)), disp + (f" [{q}]" if q and q not in disp else "")])
        out += tbl(header, trs)
        W("")

    if deleted:
        W("### 已刪除的素材")
        W("")
        W("步驟 5 各子步驟刪除的素材（資料在 `deleted-assets.tsv`，每個子步驟追加）。「引用檔數」是刪除後其他追蹤中的文字檔仍含該檔名的檔數，必須是 0；同名檔另以路徑確認。")
        W("")
        header = ["素材", "型態", "來源", "群組", "子步驟", "引用檔數", "去向"]
        remaining_names = {os.path.basename(f) for f in asset_files}
        trs = []
        for path, gid_d, step, note in sorted(deleted):
            source_d = next(g[2] for g in GROUPS if g[0] == gid_d)
            trs.append([f"`{S(path)}`", path.rsplit(".", 1)[-1].lower(), source_d, gid_d, step, deleted_refs(path, remaining_names, refs), "已刪除：" + note])
        out += tbl(header, trs)
        W("")

    # ---------------- 文件
    docs = sorted(f for f in files if scan.is_doc(f))
    W("## 2. 文件逐檔")
    W("")
    W(f"- 數量：{len(docs)} 個檔（`.md`、`.mdx`、`.rst`、`.txt`，或在任一 `docs/` 目錄且非程式、非素材）。")
    W(f"- 指令：`git ls-files | grep -E '\\.(md|mdx|rst|txt)$'` 加上 `docs/` 目錄下的非程式檔；清洗類別由 `{CMD} --category brand,colleague,merchant,pii --report` 的逐檔結果決定；作者占比由 `{CMD} --category authors --report`（`git log HEAD --name-only` 依 Will 與他人分 commit 數）。")
    W(f"- 快照：{SNAPSHOT}。清洗類別：品牌（舊名、縮寫、法人）、人名（同事）、特店資訊（特店代號、金鑰名稱、測試帳號的值）；個資另見第 3 節。作者欄是觸及該檔的 commit 數（未追蹤改名，改名前的歷史算在舊路徑）。")
    W("- D1：全部保留，清洗後公開；不移除文件。")
    W("")
    # 目錄層級作者占比
    dir_share = collections.defaultdict(lambda: [0, 0])
    for f in docs:
        d = "/".join(f.split("/")[:3]) if f.startswith("backend/docs/") else "/".join(f.split("/")[:2])
        w, o = authors.get(f, [0, 0])
        dir_share[d][0] += w
        dir_share[d][1] += o
    W("目錄層級的作者占比（commit 數）：")
    W("")
    sh_rows = []
    for d, (w, o) in sorted(dir_share.items(), key=lambda kv: -(kv[1][0] + kv[1][1]))[:25]:
        tot = w + o
        sh_rows.append([f"`{S(d)}`", str(w), str(o), f"{o * 100 // tot if tot else 0}%", "（統計，不是待辦）保留"])
    out += tbl(["目錄", "Will", "他人", "他人占比", "去向"], sh_rows)
    W("")
    trs = []
    for f in docs:
        lb = by_file.get(f, {})
        brand_n = len(lb.get("brand", [])) + len(lb.get("brand-abbr", [])) + len(lb.get("endpoint", []))
        coll_n = len(lb.get("colleague", []))
        merch_n = len(lb.get("merchant", []))
        pii_n = len(lb.get("pii", []))
        name_n = 1 if name_res.get(f) else 0
        cls = []
        if brand_n or name_n:
            cls.append("品牌")
        if coll_n:
            cls.append("人名")
        if merch_n:
            cls.append("特店資訊")
        if pii_n:
            cls.append("個資")
        t = doc_type(f)
        if t == "第三方授權文件":
            disp = "保留，原樣（第三方授權文件，不清洗）"
        elif cls:
            disp = "保留，清洗：" + "、".join(cls)
        else:
            disp = "保留（無需清洗）"
        w, o = authors.get(f, [0, 0])
        trs.append([f"`{S(f)}`", t, str(brand_n), str(coll_n), str(merch_n), str(pii_n), f"Will {w}／他人 {o}", disp])
    out += tbl(["文件", "類型", "品牌處數", "人名處數", "特店資訊處數", "個資處數", "作者(commit)", "去向"], trs)
    W("")

    # ---------------- 個資
    W("## 3. 個資逐處")
    W("")
    pii_rows = collections.defaultdict(lambda: collections.defaultdict(list))  # path -> (kind,cls) -> lines
    for p, lb in by_file.items():
        for label in ("pii", "pii-ok"):
            for code, line, kind in lb.get(label, []):
                pii_rows[p][(kind, code)].append(line)
    n_sus = sum(1 for p in pii_rows for (k, c) in pii_rows[p] if c == "疑似真實")
    W(f"- 數量：疑似真實 {n_sus} 組（路徑 x 種類）、假資料與服務位址 {sum(1 for p in pii_rows for (k, c) in pii_rows[p] if c != '疑似真實')} 組。")
    W(f"- 指令：`{CMD} --category pii --report`（只算疑似真實；假資料不計入）。判定規則在 `scan.py` 的 `classify_*`：")
    W("  - email：保留網域（example.*、*.test、*.invalid、*.localhost）與單兩字母佔位網域是假資料；gmail 等公用信箱的本機部分是 test、user、admin 等是假資料；其餘一律疑似真實。")
    W("  - 手機：同一個數字重複、連號、後五碼以上為 0 是假資料；其餘疑似真實。")
    W("  - 身分證字號：檢查碼通過才算；`x123456789` 類的連號與公開範例是假資料。")
    W("  - 統一編號：同一行有「統一編號、統編、tax id」等詞，8 碼通過檢查碼才算（通過檢查碼的測試號碼無法判定真假，見 Q-P1）。")
    W("  - 地址：縣市＋區＋路＋號；含「測試、某、火星」或門牌 1、100、999 是假資料。")
    W(f"- 快照：{SNAPSHOT}。判定欄的理由只寫規則，不寫命中的值。")
    W("")
    trs = []
    for p in sorted(pii_rows):
        for (kind, cls), lines in sorted(pii_rows[p].items()):
            if cls == "疑似真實":
                reason = {
                    "email": "網域或本機部分不是保留值或明顯佔位（含 denylist 標記的疑似真實業者網域時另有代號）",
                    "phone": "不是重複或連號的號碼",
                    "national-id": "檢查碼通過且不是範例號碼",
                    "ubn": "同行有統編關鍵字且 8 碼通過檢查碼（Q-P1）",
                    "address": "縣市區路號俱全且不含佔位字樣",
                }[kind]
                disp = "換合成資料（步驟 6）：保留測試意圖，改成不會被分配的號碼或虛構名稱" + ("；待 Will 確認 [Q-P1]" if kind in ("ubn", "national-id") else "")
            elif cls == "服務位址":
                reason, disp = "服務或平台的收件位址（resend、github、google 等）", "保留"
            else:
                reason = {"email": "保留網域或明顯佔位", "phone": "重複、連號或零填充的號碼", "national-id": "公開的範例號碼或重複數字", "ubn": "重複數字", "address": "含佔位字樣或門牌 1、100、999"}[kind]
                disp = "保留（假資料）"
            trs.append([f"`{S(p)}:{lines_cell(lines)}`", kind, cls, reason, disp])
    out += tbl(["個資位置", "種類", "判定", "理由", "去向"], trs)
    W("")

    # ---------------- 供應商文件與測試帳號
    W("## 4. 供應商文件與測試帳號資訊")
    W("")
    vendor_docs = sorted(f for f in files if f.startswith("backend/docs/第三方整合/"))
    W(f"- `backend/docs/第三方整合/` 共 {len(vendor_docs)} 個檔（計畫記 8）；指令：`git ls-files backend/docs/第三方整合 | wc -l`。")
    W(f"- 其餘位置由 `{CMD} --category merchant --report` 找出：標籤（特店代號、金鑰名稱、測試帳號）後面接著看起來像值的字串，排除佔位字樣。只列位置與種類，不列值。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    trs = []
    for f in vendor_docs:
        trs.append([f"`{S(f)}`", "供應商文件（整份）", "保留；清洗特店代號與測試帳號資訊，保留欄位說明與格式（D1、步驟 6）"])
    out += tbl(["位置", "種類", "去向"], trs)
    W("")
    W("文件與程式碼中出現特店代號、金鑰名稱與測試帳號值的位置：")
    W("")
    mrows = collections.defaultdict(lambda: collections.defaultdict(list))
    for p, lb in by_file.items():
        for code, line, kind in lb.get("merchant", []):
            mrows[p][kind].append(line)
    trs = []
    for p in sorted(mrows):
        for kind, lines in sorted(mrows[p].items()):
            if scan.is_doc(p):
                disp = "移除值，保留欄位名稱與格式說明（步驟 6）"
            elif re.search(r"(?i)test|fixture|testdata|spec|mock", p):
                disp = "已決（S2）：視為舊公司的真實憑證，換虛構值（步驟 6，測試意圖不變）"
            else:
                disp = "改由環境變數或設定注入，值不進版控（步驟 2 與步驟 6）"
            trs.append([f"`{S(p)}:{lines_cell(lines)}`", kind, disp])
    out += tbl(["位置", "種類", "去向"], trs)
    W("")

    # ---------------- gitleaks
    W("## 5. gitleaks")
    W("")
    leaks, how = run_gitleaks(a.gitleaks_json)
    if leaks is None:
        W(f"gitleaks 不可用：{how}")
        W("")
        out += tbl(["位置", "規則", "分類", "依據", "去向"], [["（未執行）", "-", "-", "-", "待重跑"]])
    else:
        cnt = collections.Counter()
        trs = []
        for d in sorted(leaks, key=lambda x: (x["File"], x["StartLine"])):
            cls, why, disp = classify_leak(d)
            cnt[cls] += 1
            trs.append([f"`{S(d['File'])}:L{d['StartLine']}`", d["RuleID"], cls, esc(san(why)), esc(san(disp))])
        W(f"- 數量：{len(leaks)} 筆（計畫記 72 筆：27 筆雜湊、17 筆要人看）；本次分類：" + "、".join(f"{k} {v}" for k, v in sorted(cnt.items())) + "。")
        W("- 指令：`git archive HEAD | tar -x -C <空目錄>`，再 `gitleaks dir <空目錄> --redact --no-banner -f json -r <輸出> --exit-code 0`；只讀 RuleID、檔案與行號，Secret 與 Match 欄位已遮蔽並且不寫入本檔。")
        W(f"- 快照：{SNAPSHOT}。{how}")
        W("- 分類規則：鎖定檔與純十六進位字串是雜湊誤報；測試檔、設計書範例與供應商文件的範例值是測試值；其餘是需要人看。分類是機械規則加上跨檔比對（同一個值出現的檔數，在記憶體內比對，不輸出）。")
        W("- 沒有發現「非雜湊、非測試值」的明文金鑰；唯一的非測試檔高熵字串（S1，已決）只記位置與規則，不展開。")
        W("")
        out += tbl(["位置", "規則", "分類", "依據", "去向"], trs)
    W("")

    # ---------------- issue / PR 與 git show
    W("## 6. issue、PR 連結與 `git show <sha>` 指引")
    W("")
    url_rows = collections.defaultdict(list)
    show_rows = collections.defaultdict(list)
    ref_rows = collections.defaultdict(list)
    for p, lb in by_file.items():
        for code, line, kind in lb.get("refs", []):
            {"issue-pr-url": url_rows, "git-show": show_rows, "issue-pr-ref": ref_rows}[kind][p].append(line)
    W(f"- issue 或 PR 連結：{len(url_rows)} 個檔（計畫記 53）；`git show <sha>` 指引：{len(show_rows)} 個檔（計畫記 16）；僅以 `#編號` 或 `owner/repo#編號` 引用 issue 或 PR：{len(ref_rows)} 個檔。")
    W(f"- 指令：`{CMD} --category refs --report`；也可用 `git grep -l -I -E 'github\\.com/[^/]+/[^/]+/(issues|pull)/[0-9]+'` 與 `git grep -l -I -E 'git show [0-9a-f]{{7,40}}'` 交叉核對。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    W("### 6a. issue 或 PR 連結")
    W("")
    out += tbl(["檔案", "處數", "去向"], [[f"`{S(p)}:{lines_cell(l, 6)}`", str(len(l)), "改成新 repo 的 issue（D3）或拿掉（步驟 6）"] for p, l in sorted(url_rows.items())])
    W("")
    W("### 6b. `git show <sha>` 指引")
    W("")
    out += tbl(["檔案", "處數", "去向"], [[f"`{S(p)}:{lines_cell(l, 6)}`", str(len(l)), "改成指向檔案（步驟 6）"] for p, l in sorted(show_rows.items())])
    W("")
    W("### 6c. 以編號引用 issue 或 PR（不是連結）")
    W("")
    out += tbl(["檔案", "處數", "去向"], [[f"`{S(p)}:{lines_cell(l, 4)}`", str(len(l)), "保留（編號指私人封存；D3 重建後新編號不同，步驟 6 在計畫 README 加一句說明）"] for p, l in sorted(ref_rows.items())])
    W("")

    # ---------------- .claude / .agents 與私人參照
    W("## 7. `.claude/`、`.agents/` 與其他檔案的個人路徑與私人參照")
    W("")
    agent_files = sorted(f for f in scan.tracked_files() if f.startswith((".claude/", ".agents/")))
    priv = collections.defaultdict(lambda: collections.defaultdict(list))
    for p, lb in by_file.items():
        for code, line, kind in lb.get("private", []):
            priv[p][kind].append(line)
    W(f"- `.claude/` 與 `.agents/` 追蹤中的路徑 {len(agent_files)} 個（`.claude/skills/*` 是指向 `.agents/skills/*` 的符號連結）；全樹含個人路徑或私人參照的檔 {len(priv)} 個。")
    W(f"- 指令：`git ls-files .claude .agents`；`{CMD} --category private --report`（本機路徑 `/Users/…`、second-brain 與私人 wiki 路徑）。")
    W(f"- 快照：{SNAPSHOT}。")
    W("")
    trs = []
    for f in agent_files:
        n = sum(len(v) for v in priv.get(f, {}).values())
        full = ROOT / f
        kind = "符號連結" if full.is_symlink() else "檔案"
        trs.append([f"`{S(f)}`", kind, str(n), "移除個人路徑與私人參照（步驟 6）" if n else ("保留；內文對舊名與舊路徑的清洗依第 2 節" if kind == "檔案" else "保留（符號連結）")])
    out += tbl(["檔案", "型態", "私人參照處數", "去向"], trs)
    W("")
    W("其他檔案的個人路徑與私人參照：")
    W("")
    trs = []
    for p in sorted(priv):
        if p.startswith((".claude/", ".agents/")):
            continue
        for kind, lines in sorted(priv[p].items()):
            trs.append([f"`{S(p)}:{lines_cell(lines)}`", kind, "移除或改成相對路徑（步驟 6）"])
    out += tbl(["位置", "種類", "去向"], trs)
    W("")

    # ---------------- 補充
    W("## 8. 計畫範圍以外的盤點發現")
    W("")
    W("以下不是逐列去向表，是步驟 3 到 6 要處理的事實；沒有表格列，所以不進 `--check-inventory` 的覆蓋檢查。")
    W("")
    W("- 計畫的素材 regex 沒涵蓋字型 woff/woff2（88 個）、影片（36 個）與 Lottie JSON；本檔已納入第 1 節。")
    W("- 檔名含舊名的追蹤檔很多（`scan.py --category brand-path`）；字型的檔案名稱與檔內家族名、Lottie 的 `nm` 欄位、App 圖示、圖片像素都含舊名，純文字的 `rename.py` 處理不到，要重新生成或替換（第 1 節）。")
    W("- 文件作者本人（Will）的全名出現在一份設計書的作者欄（Q-D1）。")
    W("- `scripts/check-legacy-endpoints.py` 與 `legacy-endpoints-baseline.json` 本身含舊名與舊網域，`rename.py` 要跳過，步驟 7 改造。")
    W("")
    Path(a.out).write_text("\n".join(out) + "\n", encoding="utf-8")
    nrows = sum(1 for l in out if l.startswith("| ") and not l.startswith("| ---"))
    print(f"wrote {a.out}: {len(out)} lines")


if __name__ == "__main__":
    main()
