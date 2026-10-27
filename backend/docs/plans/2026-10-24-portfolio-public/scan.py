#!/usr/bin/env python3
"""作品集轉 public 的掃描器（計畫 backend/docs/plans/2026-10-24-portfolio-public.md 步驟 1）。

輸出只有路徑、類別與計數，不印命中的字串。明文詞表在不進版控的 .local/publish/denylist.txt。

用法（在 repo 根目錄）：
  python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --all
  python3 .../scan.py --category brand --english          # 只用 denylist 第一個 brand 列
  python3 .../scan.py --category colleague --paths 暫存檔   # 追蹤中的檔，再加上指定的檔
  python3 .../scan.py --check-inventory [--inventory 路徑]
  python3 .../scan.py --all --report                      # 盤點用，永遠 exit 0

stdout：每個「檔案 x 類別」一行，欄位以 TAB 分隔：路徑、類別、計數、代號（逗號分隔）。
stderr：每個類別的摘要。--summary 把摘要改印到 stdout 並省略逐檔行。
exit code：0 通過；1 在應為 0 的類別有命中；2 設定錯誤（例如 denylist 不存在）。
"""
from __future__ import annotations

import argparse
import importlib.util
import multiprocessing
import os
import re
import subprocess
import sys
from collections import Counter, defaultdict
from pathlib import Path

HERE = Path(__file__).resolve().parent


def _toplevel() -> Path:
    out = subprocess.run(["git", "rev-parse", "--show-toplevel"], cwd=HERE, capture_output=True, text=True)
    if out.returncode != 0:
        sys.exit("無法定位 git 根目錄")
    return Path(out.stdout.strip())


ROOT = _toplevel()


def local_dir() -> Path:
    """`.local` 不進版控。linked worktree 沒有它時，改用主要 checkout 的（git common dir 的上一層）。"""
    here = ROOT / ".local"
    if here.is_dir():
        return here
    out = subprocess.run(
        ["git", "rev-parse", "--path-format=absolute", "--git-common-dir"], cwd=ROOT, capture_output=True, text=True
    )
    if out.returncode == 0:
        main = Path(out.stdout.strip()).parent / ".local"
        if main.is_dir():
            return main
    return here


LOCAL = local_dir()
DENYLIST = LOCAL / "publish" / "denylist.txt"
DEFAULT_INVENTORY = HERE / "inventory.md"
PLAN = "backend/docs/plans/2026-10-24-portfolio-public.md"
ATTACH_DIR = "backend/docs/plans/2026-10-24-portfolio-public/"

# 這些類別在完成定義裡必須是 0；authors 與 vendor 只回報。
GATING = ["brand", "brand-abbr", "brand-path", "colleague", "pii", "endpoint", "assets", "merchant", "private"]  # merchant、private 於步驟 6c 清到 0 後改為守門
NON_GATING = ["vendor", "refs", "authors"]
ALL_CATEGORIES = GATING + NON_GATING

# 計畫「規模」表的素材 regex，加上計畫沒涵蓋的字型與影音（盤點時另外標示）。
ASSET_PLAN_RX = re.compile(r"\.(png|jpe?g|webp|svg|ico|gif|ttf|otf|pdf)$", re.I)
ASSET_EXTRA_RX = re.compile(r"\.(woff2?|eot|mp4|webm|mov|avif|lottie)$", re.I)
DOC_EXT_RX = re.compile(r"\.(md|mdx|rst|txt)$", re.I)


LOTTIE_RX = re.compile(r"(^|/)assets/lottie/(?!.*manifest\.json$).*\.json$", re.I)


def is_asset(path: str) -> bool:
    return bool(ASSET_PLAN_RX.search(path) or ASSET_EXTRA_RX.search(path) or LOTTIE_RX.search(path))


def is_doc(path: str) -> bool:
    if is_asset(path):
        return False
    if DOC_EXT_RX.search(path):
        return True
    parts = path.split("/")
    return "docs" in parts[:-1] and not path.endswith((".go", ".ts", ".tsx", ".dart", ".java"))


def excluded(path: str) -> bool:
    return path == PLAN or path.startswith(ATTACH_DIR) or path.startswith(".local/")


# --------------------------------------------------------------------------- denylist


class Entry:
    __slots__ = ("cat", "rx", "code", "uncertain", "raw_first", "skip_path")

    def __init__(self, cat, pattern, code, uncertain, skip_path=None):
        self.cat = cat
        self.rx = re.compile(pattern)
        self.code = code
        self.uncertain = uncertain
        self.skip_path = skip_path


# 偵測用的 base64／雜湊亂數字串所在的檔：縮寫的寬鬆偵測不掃它們（改走各工具重新生成，或整批刪除）
ABBR_NOISE_PATH = re.compile(r"(\.sum|\.lock|lock\.yaml|lock\.json|\.svg|\.pbxproj)$|(^|/)assets/lottie/", re.I)


def abbr_extra(entries: list) -> list:
    """從 denylist 的 brand-abbr 列推出縮寫字母（本檔不含明文），補上邊界規則漏掉的寫法：
    左邊是非英數字，但右邊是小寫單字（`<縮寫>ui:`、`<縮寫>origin1:`）、冒號、正則 alternation `(?:<縮寫>|`、
    正則跳脫 `\\b<縮寫Pascal>`、`X-<縮寫Pascal>-`、`<數字>_<縮寫>`。
    另加一列寬鬆偵測（uncertain，不擋）：左邊不是英數字、`+`、`/`、`#` 的任何出現，用來人工確認剩下的都是已記錄的不相關類別。"""
    tok = None
    for e in entries:
        if e.cat == "brand-abbr":
            m = re.match(r"\(\?<!\[A-Za-z0-9\]\)([a-z]{2,4})(?![a-z])", e.rx.pattern)
            if m:
                tok = m.group(1)
                break
    if not tok:
        return []
    lo, up, cap = tok, tok.upper(), tok.capitalize()
    L = r"(?<![A-Za-z0-9])"
    rules = [
        (L + lo + r"ui(?![A-Za-z0-9])", "<縮寫>ui 單獨出現", False),
        (L + lo + r"(?=:[a-z])", "<縮寫>: 冒號命名空間", False),
        (L + lo + r"(?=origin1:|wst1:)", "<縮寫>origin1:／<縮寫>wst1: 簽章前綴", False),
        (L + lo + r"(?=/(?:" + lo + r"ui(?![A-Za-z0-9])|1\.0)|\\xff/1\.0)", "<縮寫>/ 對照寫法與 User-Agent", False),
        (r"(?<=\(\?:)" + lo + r"(?=\|)", "正則 alternation (?:<縮寫>|", False),
        (r"(?<=\\[bB])" + cap + r"(?=[A-Z][a-z])", "正則跳脫 \\b 之後的 Pascal 前綴", False),
        (L + up + r"TEST(?=-)", "<縮寫大寫>TEST- 前綴", False),
        (r"(?<=X-)" + cap + r"(?=-)", "X-<縮寫Pascal>- header 前綴", False),
        (r"(?<=\d_)" + lo + r"(?![A-Za-z0-9])", "<數字>_<縮寫> 活動代碼", False),
        (L + lo + r"x-relay", "<縮寫>x-relay 路由", False),
    ]
    out = [Entry("brand-abbr", p, "邊界補漏:" + c, unc) for p, c, unc in rules]
    wide = r"(?<![A-Za-z0-9+/#])(?:" + lo + "|" + up + "|" + cap + r")(?![A-Za-z0-9])"
    out.append(Entry("brand-abbr", wide, "寬鬆:縮寫獨立出現", True, ABBR_NOISE_PATH))
    return out


def load_denylist() -> list[Entry]:
    if not DENYLIST.is_file():
        sys.stderr.write(f"找不到 denylist：{DENYLIST}（明文詞表不進版控，需要先建立）\n")
        sys.exit(2)
    out = []
    for raw in DENYLIST.read_text(encoding="utf-8").splitlines():
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        cols = raw.split("\t")
        if len(cols) < 3:
            sys.stderr.write("denylist 有格式錯誤的列（欄位少於 3）\n")
            sys.exit(2)
        out.append(Entry(cols[0], cols[1], cols[2], any(c.strip() == "#uncertain" for c in cols[3:])))
    if not any(e.cat == "brand" for e in out):
        sys.stderr.write("denylist 沒有 brand 列\n")
        sys.exit(2)
    return out + abbr_extra(out)


# --------------------------------------------------------------------------- 路徑代號化

def sanitizer(entries):
    """把字串裡的 denylist 命中換成代號。盤點與報告裡的路徑一律先過它，附件才不會出現明文。"""
    ents = [e for e in entries if e.cat not in ("will-id", "vendor")]

    def f(text: str) -> str:
        for e in ents:
            text = e.rx.sub(lambda _m, c=e.code: c, text)
        return text

    return f


# --------------------------------------------------------------------------- PII

EMAIL_RX = re.compile(
    r"(?<![A-Za-z0-9._%+\-])([A-Za-z0-9._%+\-]{1,64})@([A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,})(?![A-Za-z0-9\-])"
)
PHONE_RX = re.compile(r"(?<![0-9A-Za-z])(?:\+?886[- ]?|0)(9\d{2})[- ]?(\d{3})[- ]?(\d{3})(?![0-9])")
NID_RX = re.compile(r"(?<![A-Za-z0-9])([A-Z])([12]\d{8})(?![0-9])")
UBN_CTX_RX = re.compile(r"(統一編號|統編|稅籍|tax[_ ]?id|ubn|business[_ ]?(?:no|number|id)|company[_ ]?(?:no|id)|vat[_ ]?(?:no|number|id)|gui)", re.I)
UBN_RX = re.compile(r"(?<![0-9])(\d{8})(?![0-9])")
ADDR_RX = re.compile(
    r"(?:台|臺)?[一-鿿]{1,3}[縣市][一-鿿]{1,4}[鄉鎮市區][一-鿿0-9]{1,16}?(?:路|街|大道)[一-鿿0-9之\-]{0,12}?[0-9]+號"
)

FILE_EXT_TLD = re.compile(r"\.(png|jpe?g|webp|svg|gif|js|mjs|cjs|ts|tsx|css|json|dart|go|md|yaml|yml|html|patch|lock|so|a|h|c)$", re.I)
FAKE_EMAIL_TLDS = ("example", "test", "invalid", "localhost", "local")
FAKE_EMAIL_DOMAINS = {"example.com", "example.org", "example.net", "example.com.tw", "example.co", "example.co.uk", "example.com.cn"}
SERVICE_EMAIL_SUFFIXES = (
    "resend.dev", "github.com", "users.noreply.github.com", ".google.com", ".googleapis.com", "gserviceaccount.com",
    "apple.com", "icloud.com", "sentry.io", "clerk.dev", "clerk.com", "cloudflare.com", "workers.dev",
)
FAKE_LOCALPARTS = re.compile(r"^(?:test|tester|user|users|dev|demo|foo|bar|baz|admin|ops|hello|info|noreply|no-reply|a|b|c|x|y|z|me|you|someone|name|email|mail|sample|seller|buyer|agent)\d*(?:[._+-][a-z0-9]+)*$", re.I)


def classify_email(local: str, domain: str) -> str:
    """回傳 '假資料'、'服務位址' 或 '疑似真實'。"""
    d = domain.lower()
    if FILE_EXT_TLD.search(d):
        return "忽略"
    if d in FAKE_EMAIL_DOMAINS or d.rsplit(".", 1)[-1] in FAKE_EMAIL_TLDS:
        return "假資料"
    first = d.split(".")[0]
    if len(first) <= 2 and d.count(".") == 1:  # x.com、b.co、a.tw 之類的佔位網域
        return "假資料"
    if d == "evil.com" or d.endswith(".evil.com") or d.endswith(".svc") or d.endswith(".internal"):
        return "假資料"
    if any(d == s.lstrip(".") or d.endswith(s if s.startswith(".") else "." + s) for s in SERVICE_EMAIL_SUFFIXES):
        return "服務位址"
    if d in ("gmail.com", "yahoo.com", "yahoo.com.tw", "hotmail.com", "outlook.com", "icloud.com", "msn.com", "qq.com", "163.com"):
        return "假資料" if FAKE_LOCALPARTS.match(local) else "疑似真實"
    return "疑似真實"


def classify_phone(national9: str) -> str:
    n = national9
    if len(set(n)) <= 2:
        return "假資料"
    if n in ("912345678", "987654321", "912345679", "923456789", "912341234"):
        return "假資料"
    if n.startswith("900") or n.endswith("000000") or n.endswith("00000"):
        return "假資料"
    # 前三碼之外的六碼是 ABCDEF 的連號或同號對
    tail = n[3:]
    if tail in ("123456", "654321", "111222", "222333", "333444", "333555", "333111", "111222", "555666"):
        return "假資料"
    if re.match(r"^9\d{2}(\d)\1\1\1\1\1$", n):
        return "假資料"
    if re.match(r"^9\d{2}(\d)\1\1(\d)\2\2$", n):
        return "假資料"
    return "疑似真實"


_NID_MAP = {c: v for c, v in zip("ABCDEFGHJKLMNPQRSTUVXYWZIO", range(10, 36))}


def nid_valid(letter: str, digits: str) -> bool:
    v = _NID_MAP.get(letter)
    if v is None:
        return False
    nums = [v // 10, v % 10] + [int(c) for c in digits]
    weights = [1, 9, 8, 7, 6, 5, 4, 3, 2, 1, 1]
    return sum(a * b for a, b in zip(nums, weights)) % 10 == 0


def classify_nid(letter: str, digits: str) -> str:
    if not nid_valid(letter, digits):
        return "忽略"
    if digits[1:] in ("23456789", "00000000", "11111111", "22222222", "33333333"):
        return "假資料"
    if letter + digits in ("F131104093",):  # 公開的範例號碼
        return "假資料"
    if len(set(digits)) <= 2:
        return "假資料"
    return "疑似真實"


def ubn_valid(s: str) -> bool:
    w = [1, 2, 1, 2, 1, 2, 4, 1]
    total = 0
    for ch, k in zip(s, w):
        p = int(ch) * k
        total += p // 10 + p % 10
    return total % 10 == 0 or (s[6] == "7" and (total + 1) % 10 == 0)


def classify_ubn(s: str) -> str:
    if not ubn_valid(s):
        return "忽略"
    if len(set(s)) <= 2:
        return "假資料"
    return "疑似真實"


def classify_addr(a: str) -> str:
    if re.search(r"測試|某|火星|外星|範例|示範|XX|OO|○○|甲乙", a):
        return "假資料"
    m = re.search(r"([0-9]+)號$", a)
    if m and m.group(1) in ("1", "999", "100", "123", "0"):
        return "假資料"
    return "疑似真實"


def pii_hits(text: str):
    """產生 (kind, cls, line_no)。不回傳命中的字串。"""
    if "@" in text:
        for m in EMAIL_RX.finditer(text):
            cls = classify_email(m.group(1), m.group(2))
            if cls != "忽略":
                yield "email", cls, text.count("\n", 0, m.start()) + 1
    if "9" in text:
        for m in PHONE_RX.finditer(text):
            yield "phone", classify_phone(m.group(1) + m.group(2) + m.group(3)), text.count("\n", 0, m.start()) + 1
    for m in NID_RX.finditer(text):
        cls = classify_nid(m.group(1), m.group(2))
        if cls != "忽略":
            yield "national-id", cls, text.count("\n", 0, m.start()) + 1
    for line_no, line in enumerate(text.split("\n"), 1):
        if len(line) > 4000:
            continue
        if UBN_CTX_RX.search(line):
            for m in UBN_RX.finditer(line):
                cls = classify_ubn(m.group(1))
                if cls != "忽略":
                    yield "ubn", cls, line_no
    for m in ADDR_RX.finditer(text):
        yield "address", classify_addr(m.group(0)), text.count("\n", 0, m.start()) + 1


# --------------------------------------------------------------------------- legacy endpoints


def legacy_rules() -> dict[str, re.Pattern]:
    p = ROOT / "scripts" / "check-legacy-endpoints.py"
    if not p.is_file():
        return {}
    spec = importlib.util.spec_from_file_location("_legacy_endpoints", p)
    mod = importlib.util.module_from_spec(spec)
    sys.modules["_legacy_endpoints"] = mod
    sys.dont_write_bytecode = True
    spec.loader.exec_module(mod)
    return {n: re.compile(rx) for n, rx in mod.PATTERNS.items()}



# --------------------------------------------------------------------------- 供應商特店資訊、私人參照、issue 參照（只回報，不列入 0 的類別）

_PLACEHOLDER = re.compile(r"(?i)(?:x{3,}|<[^>]{0,30}>|\$\{|\{\{|your[_-]|\.\.\.|…|example|changeme|placeholder|todo|\*{3,}|0{6,}|1234567|abcdef|env\.|getenv|process\.env|os\.Getenv|fake|testonly|假)")
# 合成值的慣例（步驟 6c）：HAWK 加 1 到 9 位的店號（真的 store_code 是 10 位，見 model.FormatMerID）與全樹共用的合成店號 2685914682，以及光貿（AMEGO）公開文件的測試 App Key（步驟 6b 決定保留，.gitleaksignore 與 sign_test.go 同值）。
_SYNTHETIC_ID = re.compile(r"HAWK[0-9]{1,9}|(?:HAWK)?2685914682|sHeq7t8G1wiQvhAuIM27")
_VALUE = r"[\"'`]?([A-Za-z0-9+/=_\-]{6,})[\"'`]?"
MERCHANT_RULES = [
    ("特店代號", re.compile(r"(?:特店|商店)(?:代號|編號|代碼)|(?i:\bMerID\b|\bmer_?id\b)|\bMerchantID\b")),
    ("金鑰名稱", re.compile(r"(?i)\b(?:Hash_?Key|Hash_?IV|App_?Key|Agent_?Key|Mer_?Key|Aes_?Key|Api_?Key|Secret_?Key)\b")),
    ("測試帳號", re.compile(r"(?i)測試(?:帳號|帳戶|特店|商家)|\btest[_ ]?(?:account|merchant|user|login)\b|\bsandbox[_ ]?(?:account|merchant)\b")),
]
_ASSIGN = re.compile(r"^[\s`\"'\]）)]*[:=：]\s*[`\"'（(]*([A-Za-z0-9+/=_\-]{6,})")

PRIVATE_RULES = [
    ("本機路徑", re.compile(r"(?<![A-Za-z0-9_.-])(?:/Users/[A-Za-z0-9._-]+|/home/[a-z][a-z0-9_-]{2,}/)")),
    ("second-brain", re.compile(r"second-brain|\.projects/personal|\.local/publish|~/\.claude|\.claude/projects")),
    ("私人 wiki 路徑", re.compile(r"\bwiki/(?:projects|tech|system)/|\bdaily/20\d\d-")),
]
REFS_RULES = [
    ("issue-pr-url", re.compile(r"github\.com/(?!google/|cloudflare/|protocolbuffers/)[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/(?:issues|pull)/\d+")),  # 第三方（google、cloudflare）的 issue 連結是正當的外部參照
    ("issue-pr-ref", re.compile(r"(?i)(?:\bissues?|\bPRs?|pull request|議題)[ \t]*#\d+|\b[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#\d+")),
    ("git-show", re.compile(r"git show [0-9a-f]{7,40}")),
]


def extra_hits(text: str, cats):
    """產生 (label, kind, line_no)。"""
    if "merchant" in cats:
        for ln, line in enumerate(text.split("\n"), 1):
            if len(line) > 3000:
                continue
            for kind, rx in MERCHANT_RULES:
                m = rx.search(line)
                if not m:
                    continue
                tail = line[m.end(): m.end() + 80]
                v = _ASSIGN.match(tail)
                if v and _SYNTHETIC_ID.fullmatch(v.group(1)):
                    continue
                if v and re.fullmatch(r"[a-z][a-z0-9]*(?:[A-Z][a-z0-9]+)+", v.group(1)):
                    continue  # camelCase 的識別字，不是值
                if v and any(ch.isdigit() for ch in v.group(1)) and not _PLACEHOLDER.search(tail[: v.end() + 6]):
                    yield "merchant", kind, ln
                elif kind == "測試帳號" and re.match(r"[\s`\"']*[:：=]\s*\S{4,}", tail) and not _PLACEHOLDER.search(tail):
                    yield "merchant", kind, ln
    if "private" in cats:
        for kind, rx in PRIVATE_RULES:
            for m in rx.finditer(text):
                yield "private", kind, text.count("\n", 0, m.start()) + 1
    if "refs" in cats:
        for kind, rx in REFS_RULES:
            for m in rx.finditer(text):
                yield "refs", kind, text.count("\n", 0, m.start()) + 1


# --------------------------------------------------------------------------- file scanning

_G: dict = {}


def _init(entries, rules, cats):
    _G["entries"] = entries
    _G["rules"] = rules
    _G["cats"] = cats


def _active_entries(entries, cats, english, include_uncertain):
    out = []
    first_brand_seen = False
    for e in entries:
        if e.cat == "brand":
            first = not first_brand_seen
            first_brand_seen = True
            if english and not first:
                continue
        if e.cat in ("brand", "brand-abbr", "colleague", "vendor") and e.cat in cats:
            out.append(e)
        elif e.cat in ("domain", "entity", "partner") and "endpoint" in cats:
            out.append(e)
    return out


def scan_file(args):
    """回傳 list of (label, code, line_no, kind)；label 以 ? 結尾表示 uncertain。"""
    path, full = args
    entries, rules, cats = _G["entries"], _G["rules"], _G["cats"]
    hits = []
    try:
        data = Path(full).read_bytes()
    except OSError:
        return path, hits
    if b"\0" in data[:8192]:
        return path, hits
    text = data.decode("utf-8", errors="replace")
    for e in entries:
        if e.skip_path is not None and e.skip_path.search(path):
            continue
        label = ("vendor" if e.cat == "vendor" else "endpoint" if e.cat in ("domain", "entity", "partner") else e.cat)
        if e.uncertain:
            label += "?"
        for m in e.rx.finditer(text):
            hits.append((label, e.code, text.count("\n", 0, m.start()) + 1, ""))
    if "endpoint" in cats:
        for name, rx in rules.items():
            for m in rx.finditer(text):
                hits.append(("endpoint", "legacy:" + name, text.count("\n", 0, m.start()) + 1, ""))
    if cats & {"merchant", "private", "refs"} and not path.endswith(("pnpm-lock.yaml", "go.sum", "pubspec.lock", "package-lock.json")):
        for label, kind, line in extra_hits(text, cats):
            hits.append((label, kind, line, kind))
    if "pii" in cats and not path.endswith(("pnpm-lock.yaml", "go.sum", "pubspec.lock", "package-lock.json")):
        for kind, cls, line in pii_hits(text):
            hits.append(("pii" if cls == "疑似真實" else "pii-ok", cls, line, kind))
    return path, hits


def tracked_files() -> list[str]:
    out = subprocess.run(
        ["git", "-c", "core.quotepath=false", "ls-files", "-z"], cwd=ROOT, capture_output=True, check=True
    ).stdout
    return [p.decode("utf-8") for p in out.split(b"\0") if p]


def run_scan(files, entries, rules, cats, english, include_uncertain, extra_paths=()):
    """files: tracked paths；extra_paths: 任意檔（絕對或相對 cwd）。回傳 {path: [(label, code, line, kind)]}。"""
    act = _active_entries(entries, cats, english, include_uncertain)
    jobs = []
    for p in files:
        if excluded(p):
            continue
        full = ROOT / p
        if full.is_file() and not full.is_symlink():
            jobs.append((p, str(full)))
    for x in extra_paths:
        fp = Path(x).resolve()
        rel = str(fp.relative_to(ROOT)) if str(fp).startswith(str(ROOT)) else str(fp)
        jobs.append((rel, str(fp)))
    res = {}
    workers = max(1, min(8, (os.cpu_count() or 2)))
    _init(act, rules, cats)
    if len(jobs) > 200 and workers > 1:
        with multiprocessing.Pool(workers, initializer=_init, initargs=(act, rules, cats)) as pool:
            for path, hits in pool.imap_unordered(scan_file, jobs, chunksize=64):
                if hits:
                    res[path] = hits
    else:
        for j in jobs:
            path, hits = scan_file(j)
            if hits:
                res[path] = hits
    return res


def name_hits(files, entries, extra_paths=()):
    """路徑名稱本身含 brand／brand-abbr／colleague 的檔。"""
    act = [e for e in entries if e.cat in ("brand", "brand-abbr", "colleague")]
    out = {}
    for p in list(files) + [str(Path(x).resolve().relative_to(ROOT)) if str(Path(x).resolve()).startswith(str(ROOT)) else x for x in extra_paths]:
        if excluded(p) and p not in extra_paths:
            continue
        hits = [(("brand-path" + ("?" if e.uncertain else "")), e.code, 0, "") for e in act if e.rx.search(p)]
        if hits:
            out[p] = hits
    return out


# --------------------------------------------------------------------------- authors


def author_counts(entries):
    wills = [e.rx for e in entries if e.cat == "will-id"]
    if not wills:
        sys.stderr.write("denylist 沒有 will-id 列，無法區分 Will 與他人\n")
        sys.exit(2)
    log = subprocess.run(
        ["git", "-c", "core.quotepath=false", "log", "HEAD", "--name-only", "--format=@@%an\t%ae"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout
    cnt = defaultdict(lambda: [0, 0])
    cur = None
    for line in log.splitlines():
        if line.startswith("@@"):
            ident = line[2:]
            cur = 0 if any(rx.search(ident) for rx in wills) else 1
            continue
        if line and cur is not None:
            cnt[line][cur] += 1
    return cnt


# --------------------------------------------------------------------------- inventory parsing

_SPLIT = re.compile(r"(?<!\\)\|")


def parse_inventory(path: Path):
    """回傳 (tables, empty_cells)。tables: list of dict(kind, header, rows)。"""
    text = path.read_text(encoding="utf-8")
    lines = text.split("\n")
    tables = []
    empty = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if line.startswith("|") and i + 1 < len(lines) and re.match(r"^\|[\s:|-]+\|\s*$", lines[i + 1]):
            header = [c.strip() for c in _SPLIT.split(line.strip())[1:-1]]
            rows = []
            j = i + 2
            while j < len(lines) and lines[j].startswith("|"):
                cells = [c.strip() for c in _SPLIT.split(lines[j].strip())[1:-1]]
                rows.append((j + 1, cells))
                j += 1
            if "去向" in header:
                k = header.index("去向")
                for ln, cells in rows:
                    v = cells[k] if k < len(cells) else ""
                    if v in ("", "-", "—", "TBD", "待定", "？", "?"):
                        empty.append(ln)
            tables.append({"header": header, "rows": rows})
            i = j
        else:
            i += 1
    return tables, empty


def inventory_keys(tables):
    assets, docs, pii, unknown_assets = {}, set(), set(), []
    for t in tables:
        h = t["header"]
        if not h:
            continue
        first = h[0]
        for ln, cells in t["rows"]:
            key = cells[0].strip("`") if cells else ""
            if first == "素材":
                src = cells[h.index("來源")] if "來源" in h and h.index("來源") < len(cells) else ""
                assets[key] = src
            elif first == "文件":
                docs.add(key)
            elif first == "個資位置" and "種類" in h:
                kind = cells[h.index("種類")] if h.index("種類") < len(cells) else ""
                path = key.split(":")[0]
                pii.add((path, kind))
    return assets, docs, pii


def cmd_check_inventory(inv: Path, entries):
    if not inv.is_file():
        sys.stderr.write(f"找不到 inventory：{inv}\n")
        return 2
    tables, empty = parse_inventory(inv)
    assets, docs, pii = inventory_keys(tables)
    files = [f for f in tracked_files() if not excluded(f) and (ROOT / f).is_file()]
    san = sanitizer(entries)
    esc = lambda x: san(x).replace("|", "\\|")
    miss_assets = [f for f in files if is_asset(f) and esc(f) not in assets]
    miss_docs = [f for f in files if is_doc(f) and esc(f) not in docs]
    res = run_scan(files, entries, {}, {"pii"}, False, False)
    miss_pii = set()
    for p, hits in res.items():
        for label, code, line, kind in hits:
            if label == "pii" and (esc(p), kind) not in pii:
                miss_pii.add((p, kind))
    # 資料列數
    rows = sum(len(t["rows"]) for t in tables if "去向" in t["header"])
    print(f"inventory: {inv.name}  表格列數={rows}")
    print(f"去向空格數={len(empty)}" + (f"（第 {','.join(map(str, empty[:20]))} 行）" if empty else ""))
    print(f"缺漏：素材={len(miss_assets)} 文件={len(miss_docs)} 個資(path,kind)={len(miss_pii)}")
    for f in miss_assets[:20]:
        print(f"  缺素材\t{f}")
    for f in miss_docs[:20]:
        print(f"  缺文件\t{f}")
    for p, k in sorted(miss_pii)[:20]:
        print(f"  缺個資\t{p}\t{k}")
    return 1 if (empty or miss_assets or miss_docs or miss_pii) else 0


# --------------------------------------------------------------------------- main


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--category", help="逗號分隔：" + ",".join(ALL_CATEGORIES))
    ap.add_argument("--all", action="store_true", help="所有類別")
    ap.add_argument("--english", action="store_true", help="brand 只用 denylist 第一個 brand 列")
    ap.add_argument("--paths", nargs="*", default=[], help="除追蹤中的檔外，再掃這些檔")
    ap.add_argument("--only-paths", action="store_true", help="只掃 --paths 指定的檔（不掃追蹤中的檔）")
    ap.add_argument("--report", action="store_true", help="盤點模式：永遠 exit 0")
    ap.add_argument("--include-uncertain", action="store_true", help="denylist 的 #uncertain 列也計入 exit code")
    ap.add_argument("--summary", action="store_true", help="摘要印到 stdout，不印逐檔行")
    ap.add_argument("--check-inventory", action="store_true")
    ap.add_argument("--inventory", default=str(DEFAULT_INVENTORY))
    ap.add_argument("--resolve", metavar="代號化路徑", help="把盤點裡的代號化路徑還原成真實路徑（讀 denylist）")
    ap.add_argument("--emit-inventory", action="store_true", help="呼叫 gen_inventory.py 重新產生 inventory.md")
    a = ap.parse_args(argv)

    entries = load_denylist()

    if a.resolve:
        san = sanitizer(entries)
        want = a.resolve.replace("\\|", "|")
        found = [f for f in tracked_files() if san(f) == want]
        print("\n".join(found))
        return 0 if found else 1
    if a.emit_inventory:
        gen = HERE / "gen_inventory.py"
        return subprocess.call([sys.executable, str(gen)] + (["--out", a.inventory] if a.inventory else []))
    if a.check_inventory:
        return cmd_check_inventory(Path(a.inventory), entries)

    if a.all:
        cats = list(ALL_CATEGORIES)
    elif a.category:
        cats = [c.strip() for c in a.category.split(",") if c.strip()]
    else:
        ap.error("需要 --category、--all、--check-inventory 或 --emit-inventory")
    bad = [c for c in cats if c not in ALL_CATEGORIES]
    if bad:
        ap.error("不認得的類別：" + ",".join(bad))

    files = [] if a.only_paths else tracked_files()
    content_cats = {c for c in cats if c in ("brand", "brand-abbr", "colleague", "pii", "endpoint", "vendor", "merchant", "private", "refs")}
    rules = legacy_rules() if "endpoint" in cats else {}
    res = run_scan(files, entries, rules, content_cats, a.english, a.include_uncertain, a.paths) if content_cats else {}
    if "brand-path" in cats:
        for p, hits in name_hits(files, entries, a.paths).items():
            res.setdefault(p, []).extend(hits)

    per_file = defaultdict(lambda: defaultdict(lambda: [0, Counter()]))  # path -> label -> [count, codes]
    for p, hits in res.items():
        for label, code, line, kind in hits:
            if label == "pii-ok":
                continue
            if label == "pii":
                code = f"{kind}"
            slot = per_file[p][label]
            slot[0] += 1
            slot[1][code] += 1

    if "assets" in cats:
        inv = Path(a.inventory)
        assets = {}
        if inv.is_file():
            assets = inventory_keys(parse_inventory(inv)[0])[0]
        san = sanitizer(entries)
        for f in files:
            if excluded(f) or not is_asset(f) or not (ROOT / f).is_file():
                continue
            src = assets.get(san(f).replace("|", "\\|"))
            if src is None:
                per_file[f]["assets"][0] += 1
                per_file[f]["assets"][1]["不在盤點"] += 1
            elif "來源不明" in src:
                per_file[f]["assets"][0] += 1
                per_file[f]["assets"][1]["來源不明"] += 1

    authors_total = [0, 0]
    if "authors" in cats:
        cnt = author_counts(entries)
        for f in files:
            if excluded(f) or not is_doc(f):
                continue
            w, o = cnt.get(f, [0, 0])
            authors_total[0] += w
            authors_total[1] += o
            per_file[f]["authors"][0] += w + o
            per_file[f]["authors"][1]["Will"] += w
            per_file[f]["authors"][1]["他人"] += o

    # 摘要
    summary = {}
    for c in cats:
        gate_labels = [c] + ([c + "?"] if a.include_uncertain else [])
        files_n = hits_n = 0
        unc_files = unc_hits = 0
        for p, labels in per_file.items():
            for lab, (n, codes) in labels.items():
                if lab == c:
                    files_n += 1
                    hits_n += n
                elif lab == c + "?":
                    unc_files += 1
                    unc_hits += n
        summary[c] = (files_n, hits_n, unc_files, unc_hits)
    if "endpoint" in cats and "vendor" not in cats:
        pass

    out_lines = []
    for p in sorted(per_file):
        for lab in sorted(per_file[p]):
            n, codes = per_file[p][lab]
            codes_s = ",".join(f"{k}={v}" for k, v in sorted(codes.items()))
            out_lines.append(f"{p}\t{lab}\t{n}\t{codes_s}")
    msg = []
    for c in cats:
        f1, h1, f2, h2 = summary[c]
        if c == "authors":
            msg.append(f"authors\t文件 {sum(1 for p in per_file if 'authors' in per_file[p])} 個\tWill={authors_total[0]}\t他人={authors_total[1]}")
        else:
            msg.append(f"{c}\t檔={f1}\t處={h1}\t（uncertain 另計：檔={f2} 處={h2}）")
    if a.summary:
        print("\n".join(msg))
    else:
        if out_lines:
            print("\n".join(out_lines))
        sys.stderr.write("\n".join(msg) + "\n")

    if a.report:
        return 0
    fail = False
    for c in cats:
        if c in GATING:
            f1, h1, f2, h2 = summary[c]
            if f1 or (a.include_uncertain and f2):
                fail = True
    return 1 if fail else 0


if __name__ == "__main__":
    sys.exit(main())
