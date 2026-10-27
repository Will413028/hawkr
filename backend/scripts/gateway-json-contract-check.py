#!/usr/bin/env python3
"""Gateway 對外 JSON 契約棘輪：把「刻意保留」和「漏改」變成機械可分的兩件事。

背景（買賣家身分分離正名，B/C/D/F 四批）：`account_id` 是登入 principal，賣家的
營運鍵是 `store_id`、買家是 `buyer_id`。正名分批進行，每一批都漏掉幾處，而漏掉的
後果是靜默的——前端照舊名接就拿到 `undefined`（已實際發生：admin console disputes
搜尋 TypeError、seller App 拿到錯的買家 ID）。

問題在於「刻意保留舊名等前端相容層上線」和「單純漏改」在程式碼裡長得一模一樣，
每次都得人工重新推導一遍。本 gate 讓兩者可分：**刻意的登記在 baseline 並附理由，
沒登記的一律紅**。

兩個檢查：

Check A — 對外舊名棘輪（主要目的）
    掃 gateway/internal/handler/**/*.go，找出所有帶角色前綴的 account_id 家族
    JSON key（sellerAccountId / buyerAccountIds / applicableSellerAccountId /
    issuerAccountId / seller_account_id / visitor_account_id …）。裸 accountId /
    account_id 不算——那是登入 principal 本身，不是被正名的營運鍵。
    四個出現面都掃：struct tag、map 字面值 key、Swagger 註解，以及現有 API 清單（OpenAPI 規格）的欄位名與參數名。

Check B — OpenAPI 規格 ↔ handler 實際輸出
    同一類靜默失效的第二個來源：規格宣告的欄位與 handler 真正吐的 key 對不上，
    前端照規格接一樣拿 undefined。規格是舊 gateway 還在服務的 REST 的清單，手寫的 client 查 wire 形狀靠它
    （backend/api/openapi/openapi.yaml，ADR 2026-09-26-external-contract-protobuf-connect D5）；handler 以 operationId
    對應（operationId＝handler 函式名，首字小寫），比對成功回應 envelope 的 data。
    保守解析（寧可漏報不要誤報：誤報會讓人關掉 gate，那比沒有 gate 更糟），
    解析不了的 handler 直接跳過並計數。

棘輪語意（兩個檢查共用）：只准收緊。
  - 出現未登記的命中 → 紅（要嘛改名，要嘛登記並寫理由）
  - baseline 有但實際已消失 → 紅（跑 --update 鎖住新低點；否則 baseline 會慢慢
    腐化成沒人敢刪的墳場）
  - 同一條目的出現次數增加 → 紅（比對用 file+key+direction，行號漂移不算）

用法：
    python3 scripts/gateway-json-contract-check.py            # check（CI 用）
    python3 scripts/gateway-json-contract-check.py --update   # 重新產生 baseline
                                                              # （既有 reason 保留）
需要 PyYAML（讀規格）；其餘只用 python3 標準庫。
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
HANDLER_DIR = ROOT / "gateway" / "internal" / "handler"
BASELINE = ROOT / "scripts" / "gateway-json-contract-baseline.json"
SPEC = ROOT / "api" / "openapi" / "openapi.yaml"

NEW_ENTRY_REASON = "TODO：填寫為什麼還沒改（誰在讀舊名／要等哪個前端相容層上線）"
CHECK_B_DEFAULT_REASON = "TODO：填寫為什麼規格與 handler 輸出不一致、要以哪一邊為準"

# ---------------------------------------------------------------------------
# Check A：舊身分名偵測
# ---------------------------------------------------------------------------

# 帶前綴的 account_id 家族 key。刻意不寫死 sellerAccountId/buyerAccountId 清單——
# 寫死的話，下一個新造的 merchantAccountId / ownerAccountId 就掃不到了。規則是
# 「account_id 家族 + 非空的角色前綴」，裸 accountId/account_id（登入 principal）不算。
CAMEL_QUALIFIED = re.compile(r"^[a-z][A-Za-z0-9]*?Account(?:Id|ID|Ids|IDs)$")
SNAKE_QUALIFIED = re.compile(r"^[a-z][a-z0-9]*(?:_[a-z0-9]+)*_account_ids?$")
BARE = {"accountId", "accountID", "account_id", "accountIds", "account_ids"}


def is_legacy_identity_key(key: str) -> bool:
    """key 是否為「帶角色前綴的 account_id 家族」JSON key。"""
    if key in BARE:
        return False
    if CAMEL_QUALIFIED.match(key):
        # 前綴要真的存在：accountId 本身被 BARE 擋掉了，這裡再擋 Account 開頭的變體
        return not key.startswith("account")
    return bool(SNAKE_QUALIFIED.match(key))


JSON_TAG_RE = re.compile(r'json:"([^"]*)"')
MAP_KEY_RE = re.compile(r'"([A-Za-z_][A-Za-z0-9_]*)"\s*:')
SWAGGER_PARAM_RE = re.compile(r"^\s*//\s*@Param\s+(\S+)")
SWAGGER_RESP_RE = re.compile(r"^\s*//\s*@(?:Success|Failure)\b")
SWAGGER_FIELD_RE = re.compile(r"([A-Za-z_][A-Za-z0-9_]*)\s*=")


def go_sources() -> list[Path]:
    """所有非測試的 handler .go 檔（測試檔不產生對外 JSON，且內含「這個 key 不該存在」
    的守門斷言字串，掃進來只會製造噪音）。"""
    return sorted(p for p in HANDLER_DIR.rglob("*.go") if not p.name.endswith("_test.go"))


def mask_source(src: str) -> tuple[str, str]:
    """回傳 (no_comments, code)，長度與 src 相同、offset 一一對應。

    no_comments：註解位置補空白，字串內容保留（map key 掃描用）。
    code       ：再把字串內容也補空白（大括號配對用，避免字串裡的 { } 干擾）。
    """
    n = len(src)
    nc = list(src)
    code = list(src)

    def blank(buf, lo, hi):
        for k in range(lo, min(hi, n)):
            if buf[k] != "\n":
                buf[k] = " "

    i = 0
    while i < n:
        ch = src[i]
        if ch == "/" and i + 1 < n and src[i + 1] == "/":
            j = src.find("\n", i)
            j = n if j < 0 else j
            blank(nc, i, j)
            blank(code, i, j)
            i = j
        elif ch == "/" and i + 1 < n and src[i + 1] == "*":
            j = src.find("*/", i + 2)
            j = n if j < 0 else j + 2
            blank(nc, i, j)
            blank(code, i, j)
            i = j
        elif ch in '"`\'':
            quote = ch
            j = i + 1
            while j < n:
                if quote != "`" and src[j] == "\\":
                    j += 2
                    continue
                if src[j] == quote:
                    j += 1
                    break
                if quote != "`" and src[j] == "\n":  # 未閉合，安全退出
                    break
                j += 1
            blank(code, i + 1, j - 1)  # 保留兩端引號，內容清掉
            i = j
        else:
            i += 1
    return "".join(nc), "".join(code)


def find_struct_blocks(code: str, nc: str) -> list[tuple[int, int, str]]:
    """找出所有 struct 區塊 (body_start, body_end, kind)。

    kind 為 "inbound" 或 "outbound"，判準（只用原始碼本身推得出的訊號，不靠人工標）：
      inbound  — 區塊內任一欄位帶 `binding:"`（gin 綁定 tag ＝ 這是請求體），
                 或具名型別以 Request / Req 結尾。
      outbound — 其餘。

    ⚠️ 大括號配對必須用 code（字串內容已清空），但 binding tag 的偵測必須用 nc
    ——Go 的 struct tag 是反引號字串，在 code 裡整段被清成空白。
    """
    blocks = []
    for m in re.finditer(r"\bstruct\s*\{", code):
        start = m.end() - 1
        end = brace_span(code, start)
        if end < 0:
            continue
        # 型別名：type XxxRequest struct { … / var req struct { …
        head = code[max(0, m.start() - 120) : m.start()]
        name_m = re.search(r"type\s+([A-Za-z0-9_]+)\s+$", head)
        type_name = name_m.group(1) if name_m else ""
        inbound = 'binding:"' in nc[start : end + 1] or bool(
            re.search(r"(Request|Req)$", type_name)
        )
        blocks.append((start, end, "inbound" if inbound else "outbound"))
    return blocks


def direction_for_offset(blocks: list[tuple[int, int, str]], off: int) -> str:
    """offset 落在哪個 struct 區塊裡 → 取最內層那個的方向。"""
    best = None
    for start, end, kind in blocks:
        if start <= off <= end:
            if best is None or (end - start) < (best[1] - best[0]):
                best = (start, end, kind)
    return best[2] if best else "outbound"


def scan_check_a() -> dict[tuple[str, str, str], dict]:
    """回傳 {(file, key, direction): {line_hint, count}}。"""
    hits: dict[tuple[str, str, str], dict] = {}

    def record(path: Path, key: str, direction: str, line: int):
        rel = str(path.relative_to(ROOT))
        k = (rel, key, direction)
        e = hits.setdefault(k, {"line_hint": line, "count": 0})
        e["count"] += 1
        e["line_hint"] = min(e["line_hint"], line)

    for path in go_sources():
        src = path.read_text(encoding="utf-8")
        nc, code = mask_source(src)
        blocks = find_struct_blocks(code, nc)
        line_of = _line_indexer(src)

        # 1) struct tag
        for m in JSON_TAG_RE.finditer(nc):
            key = m.group(1).split(",")[0].strip()
            if key and is_legacy_identity_key(key):
                record(path, key, direction_for_offset(blocks, m.start()), line_of(m.start()))

        # 2) map 字面值 key（gin.H{"x": …} / map[string]any{"x": …}）
        #    註解已被 mask 掉，所以 `// "sellerAccountId": …` 這種說明文字不會誤中。
        for m in MAP_KEY_RE.finditer(nc):
            key = m.group(1)
            if not is_legacy_identity_key(key):
                continue
            # struct tag 已由上面處理，這裡不會重疊（tag 是 json:"x"，沒有 "x": 形狀）
            record(path, key, direction_for_offset(blocks, m.start()), line_of(m.start()))

        # 3) Swagger 註解（在原始碼裡，不能用 mask 過的版本）
        for idx, raw in enumerate(src.splitlines(), start=1):
            pm = SWAGGER_PARAM_RE.match(raw)
            if pm and is_legacy_identity_key(pm.group(1)):
                record(path, pm.group(1), "swagger", idx)
            if SWAGGER_RESP_RE.match(raw):
                for fm in SWAGGER_FIELD_RE.finditer(raw):
                    if is_legacy_identity_key(fm.group(1)):
                        record(path, fm.group(1), "swagger", idx)

    # 4) OpenAPI 規格：schema 的欄位名、path／query／header 參數名
    text = SPEC.read_text(encoding="utf-8")
    lines = text.splitlines()

    def first_line(key: str) -> int:
        pat = re.compile(rf"\b{re.escape(key)}\b")
        return next((i for i, l in enumerate(lines, start=1) if pat.search(l)), 0)

    def walk(node):
        if isinstance(node, dict):
            if node.get("in") in ("path", "query", "header") and isinstance(node.get("name"), str):
                if is_legacy_identity_key(node["name"]):
                    record(SPEC, node["name"], "spec", first_line(node["name"]))
            for k, v in node.items():
                if k == "properties" and isinstance(v, dict):
                    for name in v:
                        if is_legacy_identity_key(name):
                            record(SPEC, name, "spec", first_line(name))
                walk(v)
        elif isinstance(node, list):
            for v in node:
                walk(v)

    walk(load_yaml(SPEC))
    return hits


def _line_indexer(src: str):
    starts = [0]
    for i, ch in enumerate(src):
        if ch == "\n":
            starts.append(i + 1)

    def line_of(off: int) -> int:
        lo, hi = 0, len(starts) - 1
        while lo < hi:
            mid = (lo + hi + 1) // 2
            if starts[mid] <= off:
                lo = mid
            else:
                hi = mid - 1
        return lo + 1

    return line_of


# ---------------------------------------------------------------------------
# Check B：OpenAPI 規格 ↔ handler 實際輸出
# ---------------------------------------------------------------------------

UNKNOWN = object()  # 「這層解析不出來」——不比對，不報，只計入 partial


def brace_span(code: str, open_idx: int) -> int:
    """回傳與 code[open_idx]（'{' 或 '('）配對的收尾 index，找不到回 -1。"""
    pairs = {"{": "}", "(": ")", "[": "]"}
    close = pairs[code[open_idx]]
    depth = 0
    for k in range(open_idx, len(code)):
        if code[k] in pairs:
            depth += 1
        elif code[k] in "})]":
            depth -= 1
            if depth == 0:
                return k if code[k] == close else -1
    return -1


def split_top_level(s: str, sep: str = ",") -> list[str]:
    out, depth, cur = [], 0, []
    for ch in s:
        if ch in "{([":
            depth += 1
        elif ch in "})]":
            depth -= 1
        if ch == sep and depth == 0:
            out.append("".join(cur))
            cur = []
        else:
            cur.append(ch)
    if "".join(cur).strip():
        out.append("".join(cur))
    return out


class HandlerParser:
    """單一檔案內的 handler 解析（helper 解析只跨到同檔的 package-level 函式）。"""

    def __init__(self, files: dict[str, tuple[str, str, str]]):
        # rel -> (src, nc, code)
        self.files = files
        self.helpers: dict[str, str] = {}  # func name -> gin.H 字面值內容（nc 版）
        # func name -> (字面值之後以 m["key"] = … 加入的 {key: 運算式}, 函式體 nc, 函式體 code)
        self.helper_extra: dict[str, tuple[dict[str, str], str, str]] = {}
        for rel, (_src, nc, code) in files.items():
            for m in re.finditer(
                r"^func\s+([A-Za-z0-9_]+)\s*\([^)]*\)\s*gin\.H\s*\{", code, re.M
            ):
                name = m.group(1)
                body_end = brace_span(code, m.end() - 1)
                if body_end < 0:
                    continue
                body_nc = nc[m.end() - 1 : body_end + 1]
                body_code = code[m.end() - 1 : body_end + 1]
                lit = self._first_gin_h_literal(body_nc, body_code)
                if lit is not None:
                    self.helpers[name] = lit
                    var = re.search(r"\b([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*gin\.H\s*\{", body_code)
                    if var:
                        extra = self.assigned_keys(var.group(1), body_nc, body_code)
                        self.helper_extra[name] = (extra, body_nc, body_code)

    @staticmethod
    def assigned_keys(var: str, body_nc: str, body_code: str) -> dict[str, str]:
        """字面值之外以 `var["key"] = <運算式>` 加入的 key（多半是條件式欄位）。

        只看得到「有這個 key」，看不到它在哪些條件下出現；對只比 key 集合的
        檢查來說這樣就夠了，漏掉它反而會把條件式欄位誤報成 handler 不輸出。
        """
        out: dict[str, str] = {}
        for m in re.finditer(
            rf'\b{re.escape(var)}\["([A-Za-z_][A-Za-z0-9_]*)"\]\s*=\s*', body_nc
        ):
            start = m.end()
            lit = re.match(r"(?:\[\])?gin\.H\s*\{", body_code[start:])
            if lit:
                end = brace_span(body_code, start + lit.end() - 1)
                expr = body_nc[start : end + 1] if end > 0 else ""
            else:
                eol = body_nc.find("\n", start)
                expr = body_nc[start : eol if eol >= 0 else len(body_nc)]
            out.setdefault(m.group(1), expr.strip())
        return out

    def with_helper_extra(self, name: str, node, depth: int):
        """helper 的字面值節點加上它在函式體內另外賦值的 key。"""
        if node is UNKNOWN or name not in self.helper_extra:
            return node
        extra, h_nc, h_code = self.helper_extra[name]
        for k, expr in extra.items():
            if k not in node:
                arr, sub = self.resolve_value(expr, h_nc, h_code, depth + 1)
                node[k] = {"array": arr, "node": sub}
        return node

    @staticmethod
    def _first_gin_h_literal(body_nc: str, body_code: str) -> str | None:
        """取函式體內第一個 `gin.H{…}` 字面值（回傳 nc 版內容，含大括號）。

        刻意只取第一個：helper 若有多個分支字面值就代表輸出不唯一，這種
        保守取法可能取到 early-return 的空 map——所以下面用「空 map 視為
        解析失敗」把它擋掉，寧可漏報。
        """
        for m in re.finditer(r"gin\.H\s*\{", body_code):
            end = brace_span(body_code, m.end() - 1)
            if end < 0:
                continue
            inner = body_nc[m.end() - 1 : end + 1]
            if MAP_KEY_RE.search(inner):
                return inner
        return None

    def literal_keys(self, literal_nc: str) -> dict:
        """把 `{"a": …, "b": gin.H{…}}` 解析成 {key: {"array":bool, "node":dict|None|UNKNOWN}}。"""
        inner = literal_nc.strip()
        if inner.startswith("{"):
            inner = inner[1:-1]
        out: dict = {}
        for part in split_top_level(inner):
            m = re.match(r'\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*:', part)
            if not m:
                continue
            out[m.group(1)] = {"array": False, "node": UNKNOWN, "expr": part[m.end() :].strip()}
        return out

    def resolve_value(self, expr: str, body_nc: str, body_code: str, depth: int = 0):
        """把一個 value 運算式解析成 (array, node)；解析不了回 (False, UNKNOWN)。"""
        expr = expr.strip().rstrip(",")
        if depth > 3 or not expr:
            return False, UNKNOWN

        # 內嵌 gin.H{…}
        m = re.match(r"gin\.H\s*\{", expr)
        if m:
            end = brace_span(expr, m.end() - 1)
            if end > 0:
                return False, self.expand(expr[m.end() - 1 : end + 1], body_nc, body_code, depth)

        # []gin.H{ gin.H{…} }
        m = re.match(r"\[\]gin\.H\s*\{", expr)
        if m:
            end = brace_span(expr, m.end() - 1)
            if end > 0:
                inner = expr[m.end() : end]
                m2 = re.search(r"gin\.H\s*\{", inner)
                if m2:
                    e2 = brace_span(inner, m2.end() - 1)
                    if e2 > 0:
                        return True, self.expand(inner[m2.end() - 1 : e2 + 1], body_nc, body_code, depth)
            return True, UNKNOWN

        # helper(...) 直接呼叫
        m = re.match(r"([A-Za-z0-9_]+)\s*\(", expr)
        if m and m.group(1) in self.helpers:
            node = self.expand(self.helpers[m.group(1)], body_nc, body_code, depth)
            return False, self.with_helper_extra(m.group(1), node, depth)

        # 識別字 → 回函式體找它怎麼被填的
        m = re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", expr)
        if m:
            return self.resolve_ident(expr, body_nc, body_code, depth)

        return False, UNKNOWN

    def resolve_ident(self, name: str, body_nc: str, body_code: str, depth: int):
        """x = append(x, <v>) / x := <v> → 解析元素形狀。"""
        ap = re.search(
            rf"\b{re.escape(name)}\s*=\s*append\(\s*{re.escape(name)}\s*,", body_code
        )
        if ap:
            call_open = body_code.rfind("(", 0, ap.end())
            call_end = brace_span(body_code, call_open)
            if call_end > 0:
                args = split_top_level(body_nc[ap.end() : call_end])
                if args:
                    _arr, node = self.resolve_value(args[0], body_nc, body_code, depth + 1)
                    return True, node
            return True, UNKNOWN

        asg = re.search(rf"\b{re.escape(name)}\s*:?=\s*", body_code)
        if asg:
            tail_code = body_code[asg.end() :]
            tail_nc = body_nc[asg.end() :]
            # 取到行尾（字面值可能跨行，交給 brace_span 處理）
            m = re.match(r"(?:\[\])?gin\.H\s*\{", tail_code)
            if m:
                end = brace_span(tail_code, m.end() - 1)
                if end > 0:
                    arr = tail_code.startswith("[]")
                    if arr:
                        inner_nc = tail_nc[m.end() : end]
                        inner_code = tail_code[m.end() : end]
                        m2 = re.search(r"gin\.H\s*\{", inner_code)
                        if m2:
                            e2 = brace_span(inner_code, m2.end() - 1)
                            if e2 > 0:
                                return True, self.expand(
                                    inner_nc[m2.end() - 1 : e2 + 1], body_nc, body_code, depth
                                )
                        return True, UNKNOWN
                    node = self.expand(tail_nc[m.end() - 1 : end + 1], body_nc, body_code, depth)
                    if node is not UNKNOWN:
                        for k, v in self.assigned_keys(name, body_nc, body_code).items():
                            if k not in node:
                                arr, sub = self.resolve_value(v, body_nc, body_code, depth + 1)
                                node[k] = {"array": arr, "node": sub}
                    return False, node
            m = re.match(r"([A-Za-z0-9_]+)\s*\(", tail_code)
            if m and m.group(1) in self.helpers:
                node = self.expand(self.helpers[m.group(1)], body_nc, body_code, depth + 1)
                return False, self.with_helper_extra(m.group(1), node, depth + 1)
        return False, UNKNOWN

    def expand(self, literal_nc: str, body_nc: str, body_code: str, depth: int):
        node = self.literal_keys(literal_nc)
        if not node:
            # 空 map（或一個 key 都解析不出來）＝ 沒有可信的形狀資訊。回 UNKNOWN 而非
            # {}，否則會把「解析失敗」誤報成「handler 什麼都不吐」，@Success 宣告的
            # 欄位會整組被當成 missing_in_handler。
            return UNKNOWN
        out = {}
        for k, v in node.items():
            arr, sub = self.resolve_value(v.get("expr", ""), body_nc, body_code, depth + 1)
            out[k] = {"array": arr, "node": sub}
        return out


def compare_nodes(declared, actual, prefix: str, missing_sw: list, missing_hd: list):
    """遞迴比對。任一側 UNKNOWN → 該層不比對（保守，寧可漏報）。"""
    if declared is UNKNOWN or actual is UNKNOWN or declared is None or actual is None:
        return
    if not isinstance(declared, dict) or not isinstance(actual, dict):
        return
    for k in sorted(set(actual) - set(declared)):
        missing_sw.append(prefix + k)
    for k in sorted(set(declared) - set(actual)):
        missing_hd.append(prefix + k)
    for k in sorted(set(declared) & set(actual)):
        d, a = declared[k], actual[k]
        nxt = prefix + k + ("[]." if (d.get("array") or a.get("array")) else ".")
        compare_nodes(d.get("node"), a.get("node"), nxt, missing_sw, missing_hd)


FUNC_RE = re.compile(r"^func\s+\(h\s+\*Holder\)\s+([A-Za-z0-9_]+)\s*\(c\s+\*gin\.Context\)\s*\{", re.M)


def load_yaml(path: Path):
    try:
        import yaml
    except ImportError:
        sys.exit("需要 PyYAML 才能讀規格：pip install pyyaml")
    return yaml.safe_load(path.read_text(encoding="utf-8"))


class Spec:
    """規格裡每個 operation 成功回應的 data，轉成與 HandlerParser 相同的節點格式。"""

    def __init__(self, path: Path):
        doc = load_yaml(path)
        self.schemas = doc.get("components", {}).get("schemas", {})
        self.ops = {
            op["operationId"]: op
            for item in doc["paths"].values()
            for method, op in item.items()
            if method in ("get", "post", "put", "patch", "delete") and "operationId" in op
        }

    def resolve(self, s: dict, seen: tuple = ()) -> tuple[dict, tuple]:
        if "$ref" in s:
            name = s["$ref"].rsplit("/", 1)[-1]
            if name in seen:  # 遞迴型別（例如分類樹）：這一層不再展開
                return {}, seen
            return self.resolve(self.schemas[name], seen + (name,))
        if "allOf" in s:
            props: dict = dict(s.get("properties") or {})
            for part in s["allOf"]:
                props.update(self.resolve(part, seen)[0].get("properties") or {})
            return {"type": "object", "properties": props}, seen
        return s, seen

    def node(self, s: dict, seen: tuple = ()):
        """schema → (array, node)；node 為 dict（物件）、None（純量）、UNKNOWN（自由物件等）。"""
        s, seen = self.resolve(s, seen)
        if "oneOf" in s or "anyOf" in s:  # 形狀不唯一，這一層不比對
            return False, UNKNOWN
        if s.get("type") == "array":
            return True, self.node(s.get("items") or {}, seen)[1]
        if s.get("type") == "object" or "properties" in s:
            props = s.get("properties")
            if not props:
                return False, UNKNOWN
            out = {}
            for k, v in props.items():
                arr, sub = self.node(v, seen)
                out[k] = {"array": arr, "node": sub}
            return False, out
        return False, (None if s else UNKNOWN)

    def declared_data(self, operation_id: str):
        """回傳 operation 成功回應 envelope 的 data 節點；沒有這個 operation 回 None。"""
        op = self.ops.get(operation_id)
        if op is None:
            return None
        for code, resp in op.get("responses", {}).items():
            if str(code).startswith("2"):
                media = (resp.get("content") or {}).get("application/json")
                if not media:
                    return UNKNOWN
                env = self.resolve(media["schema"])[0]
                data = (env.get("properties") or {}).get("data")
                return UNKNOWN if data is None else self.node(data)[1]
        return UNKNOWN


def scan_check_b() -> tuple[dict[tuple[str, str], dict], dict[str, int]]:
    """回傳 ({(file, handler): {missing_in_spec, missing_in_handler}}, skip 統計)。"""
    files: dict[str, tuple[str, str, str]] = {}
    for path in go_sources():
        src = path.read_text(encoding="utf-8")
        nc, code = mask_source(src)
        files[str(path.relative_to(ROOT))] = (src, nc, code)

    spec = Spec(SPEC)
    parser = HandlerParser(files)
    findings: dict[tuple[str, str], dict] = {}
    stats = {"compared": 0}
    skips: dict[str, int] = {}

    def skip(reason: str):
        skips[reason] = skips.get(reason, 0) + 1

    for rel, (src, nc, code) in files.items():
        for fm in FUNC_RE.finditer(code):
            name = fm.group(1)
            body_open = fm.end() - 1
            body_end = brace_span(code, body_open)
            if body_end < 0:
                continue
            body_code = code[body_open : body_end + 1]
            body_nc = nc[body_open : body_end + 1]

            declared = spec.declared_data(name[0].lower() + name[1:])
            if declared is None:
                skip("規格沒有對應的 operation（不是 handler，或路由還沒寫進規格）")
                continue
            if declared is UNKNOWN:
                skip("規格的成功回應沒有可比對的 data 物件")
                continue

            # 實際輸出
            ok_sites = [
                m for m in re.finditer(r"response\.OK(?:Msg)?\s*\(", body_code)
            ]
            page_sites = [m for m in re.finditer(r"response\.Page\s*\(", body_code)]
            actual = None
            actual_arr = False
            if len(ok_sites) + len(page_sites) != 1:
                skip("回應點不唯一或不存在（0 或 >1 個 response.OK/Page）")
                continue
            if ok_sites:
                m = ok_sites[0]
                call_end = brace_span(body_code, m.end() - 1)
                if call_end < 0:
                    skip("response.OK 括號配對失敗")
                    continue
                args = split_top_level(body_nc[m.end() : call_end])
                payload = args[-1] if args else ""
                arr, node = parser.resolve_value(payload, body_nc, body_code)
                if node is UNKNOWN:
                    skip("response.OK 的 payload 非 gin.H 字面值（多為具名 struct）")
                    continue
                actual, actual_arr = node, arr
            else:
                m = page_sites[0]
                call_end = brace_span(body_code, m.end() - 1)
                if call_end < 0:
                    skip("response.Page 括號配對失敗")
                    continue
                args = split_top_level(body_nc[m.end() : call_end])
                if len(args) < 5:
                    skip("response.Page 參數數量非預期")
                    continue
                arr, node = parser.resolve_value(args[1], body_nc, body_code)
                actual = {
                    "records": {"array": True, "node": node},
                    "total": {"array": False, "node": None},
                    "page": {"array": False, "node": None},
                    "pageSize": {"array": False, "node": None},
                }
                actual_arr = False

            # 只比對 key 集合，不比對「陣列 vs 物件」——那類不一致誤判率高，報出來只會製造雜訊。
            del actual_arr
            missing_sp: list[str] = []
            missing_hd: list[str] = []
            compare_nodes(declared, actual, "", missing_sp, missing_hd)
            stats["compared"] += 1
            if missing_sp or missing_hd:
                findings[(rel, name)] = {
                    "missing_in_spec": sorted(missing_sp),
                    "missing_in_handler": sorted(missing_hd),
                }
    stats["skipped"] = sum(skips.values())
    stats["skip_reasons"] = skips
    return findings, stats


# ---------------------------------------------------------------------------
# baseline I/O
# ---------------------------------------------------------------------------


def load_baseline() -> dict:
    if not BASELINE.is_file():
        return {"check_a": [], "check_b": []}
    return json.loads(BASELINE.read_text(encoding="utf-8"))


def write_baseline(data: dict) -> None:
    BASELINE.write_text(
        json.dumps(data, ensure_ascii=False, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )


HEADER = {
    "//": "Gateway 對外 JSON 契約 baseline。檢查器：scripts/gateway-json-contract-check.py",
    "//check_a": "帶角色前綴的 account_id 家族 JSON key 殘留。每一條都是「刻意還沒改」的宣告——沒登記的一律紅。比對鍵＝file+key+direction（行號漂移不算），count 只准下降。",
    "//direction": "inbound＝請求體欄位（本波正名明文排除）／outbound＝回應欄位（前端照舊名接的風險點）／swagger＝只出現在 @Param/@Success 註解（不再維護）／spec＝OpenAPI 規格的欄位名或參數名（唯一來源）。",
    "//check_b": "規格（backend/api/openapi/openapi.yaml）與 handler 實際輸出的落差。missing_in_spec＝handler 有吐但規格沒寫；missing_in_handler＝規格寫了但 handler 不吐（前端照規格接會拿到 undefined）。",
    "//maintenance": "修好一條就跑 `python3 scripts/gateway-json-contract-check.py --update` 鎖住新低點並 commit；留著不動會讓清單腐化成沒人敢刪的墳場。",
}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument(
        "--update",
        action="store_true",
        help="以當前程式碼重新產生 baseline（既有條目的 reason 保留）",
    )
    ap.add_argument("--stats", action="store_true", help="額外印出 Check B 覆蓋率統計")
    args = ap.parse_args()

    a_hits = scan_check_a()
    b_hits, b_stats = scan_check_b()
    base = load_baseline()

    if args.update:
        old_a = {
            (e["file"], e["key"], e["direction"]): e.get("reason", "")
            for e in base.get("check_a", [])
        }
        old_b = {
            (e["file"], e["handler"]): e.get("reason", "")
            for e in base.get("check_b", [])
        }
        new = dict(HEADER)
        new["check_a"] = [
            {
                "file": f,
                "line_hint": v["line_hint"],
                "key": k,
                "direction": d,
                "count": v["count"],
                "reason": old_a.get((f, k, d)) or NEW_ENTRY_REASON,
            }
            for (f, k, d), v in sorted(a_hits.items())
        ]
        new["check_b"] = [
            {
                "file": f,
                "handler": h,
                "missing_in_spec": v["missing_in_spec"],
                "missing_in_handler": v["missing_in_handler"],
                "reason": old_b.get((f, h)) or CHECK_B_DEFAULT_REASON,
            }
            for (f, h), v in sorted(b_hits.items())
        ]
        write_baseline(new)
        print(
            f"baseline 已更新：Check A {len(new['check_a'])} 條 / Check B {len(new['check_b'])} 條"
        )
        return 0

    failed = False
    out = sys.stderr

    # ---- Check A ----
    base_a = {
        (e["file"], e["key"], e["direction"]): e for e in base.get("check_a", [])
    }
    undeclared = sorted(k for k in a_hits if k not in base_a)
    if undeclared:
        failed = True
        print("\n[Check A] 未登記的舊身分名 JSON key：\n", file=out)
        for f, k, d in undeclared:
            print(f"  ✘ {f}:{a_hits[(f, k, d)]['line_hint']}  key={k}  direction={d}", file=out)
        print(
            "\n  account_id 是登入 principal；賣家的營運鍵是 store_id、買家是 buyer_id。"
            "\n  要嘛把 JSON key 改成正名後的名字，要嘛在 scripts/gateway-json-contract-baseline.json"
            "\n  的 check_a 補一條並寫清楚理由（誰在讀舊名／要等哪個前端相容層上線）。"
            "\n  跑 `python3 scripts/gateway-json-contract-check.py --update` 可先產生條目骨架。",
            file=out,
        )

    gone = sorted(k for k in base_a if k not in a_hits)
    if gone:
        failed = True
        print("\n[Check A] baseline 有登記、但程式碼裡已經不存在：\n", file=out)
        for f, k, d in gone:
            print(f"  ✘ {f}  key={k}  direction={d}   理由：{base_a[(f, k, d)].get('reason', '')}", file=out)
        print(
            "\n  這是好消息（有人改掉了），但 baseline 沒跟上。跑 --update 鎖住新低點並 commit——"
            "\n  留著不動會讓清單腐化成沒人敢刪的墳場。",
            file=out,
        )

    for key in sorted(set(a_hits) & set(base_a)):
        now = a_hits[key]["count"]
        was = base_a[key].get("count", now)
        if now > was:
            failed = True
            print(
                f"\n[Check A] {key[0]} key={key[1]} direction={key[2]}："
                f"出現 {now} 次 > baseline {was} 次 — 新增了舊名的使用點。",
                file=out,
            )
        elif now < was:
            failed = True
            print(
                f"\n[Check A] {key[0]} key={key[1]} direction={key[2]}："
                f"出現 {now} 次 < baseline {was} 次 — 有進展，跑 --update 鎖住新低點。",
                file=out,
            )

    # ---- Check B ----
    base_b = {(e["file"], e["handler"]): e for e in base.get("check_b", [])}
    for key in sorted(set(b_hits) - set(base_b)):
        failed = True
        v = b_hits[key]
        print(f"\n[Check B] {key[0]} / {key[1]}：規格與實際輸出不一致，且未登記", file=out)
        if v["missing_in_spec"]:
            print(f"    handler 有吐但規格沒宣告：{', '.join(v['missing_in_spec'])}", file=out)
        if v["missing_in_handler"]:
            print(f"    規格宣告了但 handler 不吐（前端會拿到 undefined）：{', '.join(v['missing_in_handler'])}", file=out)
        print("    以 wire 為準修正 backend/api/openapi/openapi.yaml；刻意不一致才登記進 baseline 並寫理由。", file=out)
    for key in sorted(set(base_b) - set(b_hits)):
        failed = True
        print(
            f"\n[Check B] {key[0]} / {key[1]}：baseline 登記了落差，但現在已經一致 — 跑 --update 移除該條目。",
            file=out,
        )
    for key in sorted(set(b_hits) & set(base_b)):
        now, was = b_hits[key], base_b[key]
        for kind, label in (
            ("missing_in_spec", "handler 有吐但規格沒宣告"),
            ("missing_in_handler", "規格宣告了但 handler 不吐"),
        ):
            added = sorted(set(now[kind]) - set(was.get(kind, [])))
            removed = sorted(set(was.get(kind, [])) - set(now[kind]))
            if added:
                failed = True
                print(f"\n[Check B] {key[0]} / {key[1]}：新增落差（{label}）：{', '.join(added)}", file=out)
            if removed:
                failed = True
                print(
                    f"\n[Check B] {key[0]} / {key[1]}：落差已修（{label}）：{', '.join(removed)} — 跑 --update 鎖住。",
                    file=out,
                )

    if args.stats or failed:
        print(
            f"\nCheck B 覆蓋率：已比對 {b_stats['compared']} 支 handler / 跳過 {b_stats['skipped']} 支",
            file=out,
        )
        for reason, n in sorted(b_stats["skip_reasons"].items(), key=lambda x: -x[1]):
            print(f"    {n:4d}  {reason}", file=out)

    if not failed:
        print(
            f"gateway JSON 契約 gate 通過："
            f"Check A {len(a_hits)} 個舊名位點全部已登記；"
            f"Check B 已比對 {b_stats['compared']} 支 handler（跳過 {b_stats['skipped']} 支），"
            f"{len(b_hits)} 支的落差全部已登記。"
        )
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
