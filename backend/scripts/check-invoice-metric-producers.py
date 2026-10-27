#!/usr/bin/env python3
"""每一個被 invoice 告警／recording rule 引用的 series，都必須有生產者。

**為什麼需要它**（2026-09-05 第 6 輪稽核實測）：`business-alerts.yml` 的
`InvoicePiiRevealSpike`／`InvoicePdfMissing`／`InvoiceRefundExceedsInvoiced` 三條
引用的 `invoice_admin_pii_reveal_total`（M13）、`invoice_documents_pdf_missing_total`
（M15）、`invoice_refund_exceeds_invoiced_total`（M16）**全 repo 零生產者**——
只活在設計書、告警檔與程式碼註解裡。Prometheus 對不存在的序列不會 fire，而
`increase(X[1h]) > 0` 在 no data 與「一切正常」上輸出**完全一樣** ⇒ 三條告警是
永久靜默的擺設，而且看起來與健康無異。

**為什麼既有的兩道檢查抓不到**（兩者都是對的，只是觀測窗不同）：
  - `internal/worker/metrics_test.go` 的窗 ＝ **手寫的 8 個名字**。窗外的 metric
    不存在檢查，而「新增一條告警卻忘了加進那份清單」正是本 class 的成因。
  - `internal/repository/postgres/exporter_contract_test.go` 的窗 ＝ **已經寫進
    `sql_exporter.yml` 的 metric**。從未落地的 metric 同樣在窗外。
  兩者都是「從生產者那一側」看，沒有任何東西**從告警那一側反查**。

本檔有**兩層**檢查，第二層是第一層漏掉的那一半：

  第一層「有沒有註冊」：被規則引用的 series 必須有生產者。
  第二層「有沒有人觀測」：Go 側註冊的 series 必須至少有一處真正的觀測
      （`.Inc()` / `.Observe(...)` / `.Set(...)` / `.Add(非 0)`）。
      🔴 這一層存在的理由：M3／M4／M14 **通過第一層**——它們宣告完整、被 preRegister
      預先展開、/metrics 上看得到三行值為 0 的序列，而全服務沒有任何一處遞增它們。
      只有第一層的話這道 gate 會對那三支發綠燈，而它們正是本輪要抓的東西。
      預先展開（`.Add(0)` 與裸 `WithLabelValues(...)`）刻意**不算觀測**：它造出來的
      恆 0 序列在 `> 0` 與比率型告警上與「一切正常」完全一樣。

**觀測窗（誠實標明，見 .claude/rules/06-no-small-tails.md §3.10）**：
  - 只管 `invoice_*` 與 recording rule 的 `invoice:*`。其他前綴（`notification_email_*`
    是設計書明列的 Phase 2、`node_*`／`mq_*`／`outbox_*` 是共用或第三方）不在窗內。
  - 第二層做**一層**間接：觀測寫在 `observe*` helper 裡時，會再要求那個 helper
    有呼叫端（2026-09-05 實測：只做零層的版本，對「helper 還在、呼叫端被刪掉」
    完全無感——而那正是同一個失效形狀的下一個版本）。
    **兩層以上的間接、以及「呼叫端自己走不到」它看不出來**：helper 的呼叫端若寫在
    一條永遠不成立的 if 裡，這道 gate 仍會放行（那需要真正的可達性分析）。
  - helper 的判定錨在**命名慣例** `observe*`。用別的名字包一層就會退回零層檢查
    ——本 repo 目前全部走這個字首，新增時請沿用。
  - Go 側的生產者判定錨在 `Name:` 欄位（prometheus.*Opts 的唯一寫法），
    不是任何出現該字串的地方 —— 註解與 log event 名不算生產者，那正是 M13／M16
    當初看起來「有人在用」的原因。
  - sql_exporter 的 metric 不進第二層：它們的「觀測」是那句 SQL 本身，
    由 exporter_contract_test.go 從另一個方向守。

exit 0 = 每個被引用的 series 都有生產者；exit 1 = 有孤兒，或任一側解析結果為空
（空集合視為失敗不是通過：檔案被搬走時 SKIP 會讓這道檢查靜默消失）。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
RULES_DIR = REPO / "monitoring/prometheus/rules"
SQL_EXPORTERS = [
    REPO / "monitoring/sql-exporter/sql_exporter.yml",
]
GO_ROOTS = [REPO / "services", REPO / "pkg", REPO / "gateway"]

# 被引用側只認這兩個形狀，理由見檔頭「觀測窗」。
CONSUMED_RE = re.compile(r"\b(invoice_[a-z0-9_]+|invoice:[a-z0-9_:]+)")
# PromQL 的字串字面值（label matcher 的值、annotation 片段）不是 series 名。
STRING_RE = re.compile(r"\"[^\"]*\"|'[^']*'")
# histogram／summary 自動衍生的後綴：底名存在就算有生產者。
DERIVED_SUFFIXES = ("_count", "_sum", "_bucket")

GO_NAME_RE = re.compile(r"""Name:\s*"(invoice_[a-z0-9_]+)"|Name:\s+"(invoice_[a-z0-9_]+)\"""")


def collect_consumed() -> dict[str, set[str]]:
    """回傳 {series: {出處}}，只掃 rules 的 expr（不掃 annotations）。"""
    out: dict[str, set[str]] = {}
    files = sorted(RULES_DIR.glob("*.yml")) + sorted(RULES_DIR.glob("*.yaml"))
    if not files:
        die(f"{RULES_DIR} 底下沒有任何規則檔 —— 檔案被搬走了？")
    for path in files:
        doc = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        for group in doc.get("groups") or []:
            gname = group.get("name", "?")
            for rule in group.get("rules") or []:
                expr = rule.get("expr")
                if not isinstance(expr, str):
                    continue
                who = rule.get("alert") or rule.get("record") or "?"
                for name in CONSUMED_RE.findall(STRING_RE.sub(" ", expr)):
                    out.setdefault(name, set()).add(f"{path.name}:{gname}:{who}")
    return out


# 一次真正的觀測。`.Add(0)` 與裸 `WithLabelValues(...)` 是預先展開，刻意不算。
OBSERVE_RE = re.compile(r"\.(Inc\(\)|Observe\(|Set\(|Add\(\s*(?!0\s*\))|Dec\(\))")
# 綁定 metric 的識別字：package-level var、或 struct literal 的欄位名。
BIND_RE = re.compile(
    r"^[ \t]*(?:var[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*[:=][ \t]*promauto\.New[A-Za-z]*\(",
    re.MULTILINE,
)


def go_metric_bindings() -> dict[str, tuple[str, Path]]:
    """回傳 {series: (綁定它的識別字, 宣告檔)}。

    只認 `<ident> = promauto.NewXxx(` 與 `<ident>: promauto.NewXxx(` 兩種寫法
    ——本 repo 註冊 metric 只有這兩種，第三種寫法出現時這裡會漏掉它，
    而漏掉的症狀是「第二層檢查對它靜默放行」⇒ `unbound` 會被列出來當作失敗。
    """
    out: dict[str, tuple[str, Path]] = {}
    for root in GO_ROOTS:
        if not root.is_dir():
            continue
        for go in sorted(root.rglob("*.go")):
            if go.name.endswith("_test.go"):
                continue
            text = go.read_text(encoding="utf-8", errors="replace")
            if "invoice_" not in text:
                continue
            for m in GO_NAME_RE.finditer(text):
                series = m.group(1) or m.group(2)
                head = text[: m.start()]
                bind = None
                for b in BIND_RE.finditer(head):
                    bind = b.group(1)
                out[series] = (bind or "", go)
    return out


FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*[\(\[]", re.MULTILINE)
# 觀測 helper 的命名慣例（本 repo 全部走這個字首）。它們多一層檢查：**要有呼叫端**。
HELPER_RE = re.compile(r"^[Oo]bserve")


def _funcs(text: str) -> list[tuple[int, int, str]]:
    """回傳 [(start, end, name)]，top-level 函式的粗略邊界（下一個 `^func ` 為界）。"""
    marks = [(m.start(), m.group(1)) for m in FUNC_RE.finditer(text)]
    out = []
    for i, (start, name) in enumerate(marks):
        end = marks[i + 1][0] if i + 1 < len(marks) else len(text)
        out.append((start, end, name))
    return out


def _enclosing(funcs: list[tuple[int, int, str]], off: int) -> str:
    for start, end, name in funcs:
        if start <= off < end:
            return name
    return ""


def unobserved_go_metrics() -> dict[str, str]:
    """回傳 {series: 診斷句}，列出「有註冊、零觀測」的 Go 指標。

    兩層都要過才算被觀測：
      ① 有一句 `.Inc()`／`.Observe()`／`.Set()`／`.Add(非 0)` 打在那個識別字上
      ② 若那一句寫在 `observe*` helper 裡，那個 helper **要有呼叫端**
         —— 少了 ②，「helper 還在、呼叫端被刪掉」會靜默通過，而那是同一個失效形狀
         的下一個版本（2026-09-05 實測：拿掉 do() 裡的 observeCall，只有 ① 的版本全綠）。
    """
    bindings = go_metric_bindings()
    if not bindings:
        die("一個 Go 側 invoice metric 綁定都沒解析到 —— 解析壞了，不是「真的沒有」")

    sources: list[tuple[Path, str]] = []
    for root in GO_ROOTS:
        if not root.is_dir():
            continue
        for go in sorted(root.rglob("*.go")):
            if go.name.endswith("_test.go"):
                continue
            sources.append((go, go.read_text(encoding="utf-8", errors="replace")))

    def helper_has_caller(helper: str, decl_text: str) -> bool:
        """helper 在**自己的函式體以外**有沒有被呼叫（非測試）。"""
        call_re = re.compile(r"\b" + re.escape(helper) + r"\s*\(")
        for _, text in sources:
            funcs = _funcs(text)
            for c in call_re.finditer(text):
                if _enclosing(funcs, c.start()) != helper:
                    return True
        _ = decl_text
        return False

    bad: dict[str, str] = {}
    for series, (ident, decl) in sorted(bindings.items()):
        if not ident:
            bad[series] = f"{decl.relative_to(REPO)}：認不出綁定它的識別字（本檢查只認 promauto.New* 的兩種寫法）"
            continue
        use_re = re.compile(r"\b" + re.escape(ident) + r"\b")
        observed = False
        helpers_seen: set[str] = set()
        for _, text in sources:
            funcs = _funcs(text)
            for u in use_re.finditer(text):
                # 識別字後面接一段鏈（可能有 WithLabelValues(...)），找真正的觀測動詞。
                chain = text[u.end() : u.end() + 240].split(";")[0]
                if not OBSERVE_RE.search(chain):
                    continue
                enclosing = _enclosing(funcs, u.start())
                if enclosing and HELPER_RE.match(enclosing):
                    helpers_seen.add(enclosing)
                    if not helper_has_caller(enclosing, text):
                        continue  # helper 沒有呼叫端 ⇒ 這一句永遠不會執行
                observed = True
                break
            if observed:
                break
        if not observed:
            why = f"識別字 {ident} 全服務沒有任何一處 .Inc()／.Observe()／.Set()／.Add(非 0)"
            if helpers_seen:
                why = (
                    f"觀測只寫在 {'／'.join(sorted(helpers_seen))} 裡，而那個 helper "
                    "**沒有任何呼叫端** ⇒ 那一句永遠不會執行"
                )
            bad[series] = f"{decl.relative_to(REPO)}：{why}"
    return bad


def collect_produced() -> set[str]:
    produced: set[str] = set(go_metric_bindings())

    # ② sql_exporter 的 metric_name。
    for path in SQL_EXPORTERS:
        if not path.is_file():
            continue
        for line in path.read_text(encoding="utf-8").splitlines():
            m = re.match(r"\s*-\s*metric_name:\s*(invoice_[a-z0-9_]+)\s*$", line)
            if m:
                produced.add(m.group(1))

    # ③ recording rule 自己 record 出來的 series（它是下游告警的生產者）。
    for path in sorted(RULES_DIR.glob("*.yml")):
        doc = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        for group in doc.get("groups") or []:
            for rule in group.get("rules") or []:
                rec = rule.get("record")
                if isinstance(rec, str) and (rec.startswith("invoice_") or rec.startswith("invoice:")):
                    produced.add(rec)

    return produced


def die(msg: str) -> None:
    print(f"FAIL: {msg}", file=sys.stderr)
    sys.exit(1)


def main() -> int:
    consumed = collect_consumed()
    produced = collect_produced()

    if not consumed:
        die("規則檔裡一個 invoice_* series 都沒解析到 —— 解析壞了，不是「都很乾淨」")
    if not produced:
        die("一個 invoice_* 生產者都沒解析到 —— 解析壞了，不是「真的沒有」")

    orphans: dict[str, set[str]] = {}
    for name, where in sorted(consumed.items()):
        if name in produced:
            continue
        base = next(
            (name[: -len(s)] for s in DERIVED_SUFFIXES if name.endswith(s) and name[: -len(s)] in produced),
            None,
        )
        if base:
            continue
        orphans[name] = where

    unobserved = unobserved_go_metrics()

    print(
        f"invoice metric producers: 被引用 {len(consumed)} 個、找到生產者 {len(produced)} 個、"
        f"Go 側註冊 {len(go_metric_bindings())} 個"
    )
    if unobserved:
        print("", file=sys.stderr)
        print("以下 Go 指標**有註冊、零觀測** ⇒ 序列存在但恆為 0，", file=sys.stderr)
        print("在 `> 0` 與比率型告警上與「一切正常」完全一樣：", file=sys.stderr)
        for name, why in unobserved.items():
            print(f"  - {name}\n      {why}", file=sys.stderr)
        print("", file=sys.stderr)
        print("修法：把觀測接到它唯一的發生點上（指標放在該 package，不要跨依賴方向），", file=sys.stderr)
        print("或連同告警一起移除並在設計書說明。**預先展開不算觀測。**", file=sys.stderr)
    if orphans:
        print("", file=sys.stderr)
        print("以下 series 被規則引用但**沒有任何生產者** ⇒ 對應規則永遠不會 fire：", file=sys.stderr)
        for name, where in orphans.items():
            print(f"  - {name}", file=sys.stderr)
            for w in sorted(where):
                print(f"      引用者 {w}", file=sys.stderr)
        print("", file=sys.stderr)
        print("修法二選一（不得只刪告警了事，除非那個風險真的不需要被看見）：", file=sys.stderr)
        print("  a) 補生產者：Go 側 promauto 的 Name 欄位，或 sql-exporter 的 metric_name", file=sys.stderr)
        print("  b) 這個風險已由別的機制覆蓋 ⇒ 連同告警一起移除，並在 58 號設計書 §8.2.3／§8.2.4 說明", file=sys.stderr)
    if orphans or unobserved:
        return 1

    print("OK：每個被引用的 invoice series 都有生產者，且每個 Go 指標都真的被觀測")
    return 0


if __name__ == "__main__":
    sys.exit(main())
