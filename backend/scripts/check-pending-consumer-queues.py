#!/usr/bin/env python3
"""「consumer 尚未實作」queue 清單的門檻檢查。

## 這道 gate 守什麼

`queue:consumer_impl_pending:info`（`monitoring/prometheus/rules/infra-alerts.yml`）
列出「topology 有登記、但 repo 裡根本沒有 consumer 實作」的 queue。它讓那些 queue
從 `RabbitMQQueueNoConsumersStalled`（critical／30m）移到
`RabbitMQQueuePendingConsumerImplementation`（warning／`cadence: daily`）。

理由是兩者是**不同的事件**，只是症狀相同（consumer 數 = 0）：

| | 意思 | 正確處置 |
|---|---|---|
| Stalled | 原本有 consumer，現在沒了 | 事故，critical 一直催 |
| PendingImpl | 從來沒有過 consumer | 待排期的實作，一天提醒一次 |

2026-09-12 實測：後者在 prod＋dev 產生 72 則/天，把同頻道的真斷線告警蓋掉。

## 它同時是一個逃生口

「把吵的 queue 加進清單」就安靜了。本檔存在的唯一理由是讓那件事做不到。

## 門檻（六條，全部可機械執行）

1. recording rule 必須存在且解析得出 queue 清單（清單空 ⇒ 整組是死設定，要移除）。
2. 清單裡的每個 queue 必須在 `infra/mq/topology/topology.yaml` 有登記
   （不在 topology ⇒ 這個名字是錯的，或 queue 已被刪，兩種都該修）。
3. 🔴 清單裡的每個 queue **repo 裡確實沒有 consumer 實作**。
   一旦有人實作出來就必須移除 —— 保護因此自動恢復，不靠任何人記得。
4. 專職告警必須引用該 recording rule，否則清單沒有告警面（完全靜默）。
5. 兩條既有告警必須 `unless` 掉它，否則同一個 queue 疊報兩條。
6. 不得繞過 recording rule、直接把 queue 名寫進既有告警的排除 regex
   —— 那會躲掉門檻 1–5 全部。

## 它擋不住什麼（誠實標明，06-no-small-tails.md §3.10 的收尾要求）

- **Java consumer 的偵測是弱的**：只看 queue 名的字串字面值有沒有出現在非測試
  `.java` 檔。Java 側沒有 queue↔topology 的 CI gate（10-engineering-contracts.md §23
  明載），而 `@RabbitListener` 的 queue 名可以來自 properties、SpEL 或組出來的字串。
  ⇒ 門檻 3 對 Java 只擋得住「明目張膽寫死」那一種。方向是**保守的**（寧可誤判成
  「有實作」而要求移出清單），所以不會產生假的安靜。
- **票是不是 open**：本檔不查 GitHub（會讓 gate 依賴網路與權杖）。專職告警的
  issue 引用由 `check-alert-cadence.py` 的門檻 2 檢查，同樣只驗「有寫」。
- **Prometheus 的實際行為**：本檔讀 YAML 結構，不執行 PromQL。`unless` 的排除效果
  要靠實跑驗（2026-09-12 已測：放寬範圍後無 unless 12 筆、有 unless 8 筆，
  差異恰為清單那 4 個，所有 .dlq 仍保留 ⇒ 只排除清單而非排除全部）。
"""
import re
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
RULES = REPO / "monitoring" / "prometheus" / "rules" / "infra-alerts.yml"
TOPOLOGY = REPO / "infra" / "mq" / "topology" / "topology.yaml"

RECORD_NAME = "queue:consumer_impl_pending:info"
PENDING_ALERT = "RabbitMQQueuePendingConsumerImplementation"
MUST_UNLESS = ["RabbitMQNoConsumers", "RabbitMQQueueNoConsumersStalled"]

# 從 recording rule 的 regex 抽 queue 名：^a\.b\.c$ 形式，用 | 串接。
QUEUE_IN_REGEX = re.compile(r"\^([A-Za-z0-9_.\\-]+?)\$")


def rel(p):
    try:
        return p.relative_to(REPO)
    except ValueError:
        return p


def load_rules(errors):
    if not RULES.is_file():
        errors.append(f"[結構] 告警規則不存在：{rel(RULES)}")
        return {}, {}
    doc = yaml.safe_load(RULES.read_text(encoding="utf-8")) or {}
    records, alerts = {}, {}
    for g in doc.get("groups") or []:
        for r in g.get("rules") or []:
            if r.get("record"):
                records[r["record"]] = r
            elif r.get("alert"):
                alerts[r["alert"]] = r
    return records, alerts


def parse_queues(expr):
    """從 recording rule 的 expr 抽出 queue 名（regex 內的 ^...$ 段）。"""
    out = []
    for raw in QUEUE_IN_REGEX.findall(expr):
        out.append(raw.replace("\\\\.", ".").replace("\\.", "."))
    return out


def topology_queues(errors):
    if not TOPOLOGY.is_file():
        errors.append(f"[結構] topology 不存在：{rel(TOPOLOGY)}")
        return set()
    doc = yaml.safe_load(TOPOLOGY.read_text(encoding="utf-8")) or {}
    return {q["name"] for q in (doc.get("queues") or []) if q.get("name")}


def source_files(suffix):
    """非測試的原始碼檔。"""
    skip = ("/node_modules/", "/.git/", "/target/", "/build/", "/scripts/output/")
    for base in ("services", "pkg", "gateway"):
        root = REPO / base
        if not root.is_dir():
            continue
        for f in root.rglob(f"*{suffix}"):
            s = str(f)
            if any(k in s for k in skip):
                continue
            name = f.name
            if name.endswith("_test.go") or name.endswith("Test.java") or "/test/" in s:
                continue
            yield f


def find_consumer_impl(queue):
    """queue 名是否以字面值出現在非測試的 .go / .java —— 回傳出處或 None。"""
    for suffix in (".go", ".java"):
        for f in source_files(suffix):
            try:
                if queue in f.read_text(encoding="utf-8", errors="replace"):
                    return str(rel(f))
            except OSError:
                continue
    return None


def check():
    errors = []
    records, alerts = load_rules(errors)
    if not records and not alerts:
        return errors, []

    rec = records.get(RECORD_NAME)
    if rec is None:
        # 清單整組不存在是合法的（沒有待實作的 queue）——但那時專職告警也不該在。
        if PENDING_ALERT in alerts:
            errors.append(
                f"[門檻1] 告警 {PENDING_ALERT} 存在，但 recording rule {RECORD_NAME} 不見了 "
                f"⇒ 該告警的 expr 永遠沒有資料，完全靜默。"
            )
        return errors, []

    queues = parse_queues(rec.get("expr", ""))
    if not queues:
        errors.append(
            f"[門檻1] {RECORD_NAME} 解析不出任何 queue 名。清單空了就把 recording rule 與 "
            f"{PENDING_ALERT} 一起移除（死設定會讓下一個人以為還有東西靠它）；"
            f"否則是 regex 寫法變了，請同步更新本檔的 QUEUE_IN_REGEX。"
        )
        return errors, []

    topo = topology_queues(errors)
    for q in queues:
        # 門檻 2
        if topo and q not in topo:
            errors.append(
                f"[門檻2] {q!r} 不在 {rel(TOPOLOGY)} 的 queues 清單裡 ⇒ 名字打錯，或 queue 已被刪除。"
            )
        # 門檻 3 —— 這一條是整道 gate 的重點
        where = find_consumer_impl(q)
        if where:
            errors.append(
                f"[門檻3] {q!r} 已經在 {where} 出現 ⇒ consumer 可能已經實作了。"
                f"請把它從 {RECORD_NAME} 移除，讓它回到 RabbitMQQueueNoConsumersStalled "
                f"（critical）的守備範圍。清單的意義是「還沒有人實作」，不是「不想被吵」。"
            )

    # 門檻 4
    pending = alerts.get(PENDING_ALERT)
    if pending is None:
        errors.append(
            f"[門檻4] recording rule {RECORD_NAME} 存在，但沒有 {PENDING_ALERT} 告警引用它 "
            f"⇒ 這些 queue 被排除掉之後**完全沒有訊號**。"
        )
    elif RECORD_NAME not in str(pending.get("expr", "")):
        errors.append(f"[門檻4] {PENDING_ALERT} 的 expr 沒有引用 {RECORD_NAME}。")

    # 門檻 5
    for name in MUST_UNLESS:
        a = alerts.get(name)
        if a is None:
            errors.append(f"[門檻5] 找不到告警 {name}（規則被改名或刪除？本檔的假設要跟著更新）。")
            continue
        expr = str(a.get("expr", ""))
        if RECORD_NAME not in expr or "unless" not in expr:
            errors.append(
                f"[門檻5] {name} 沒有 `unless ... {RECORD_NAME}` ⇒ 清單裡的 queue 會同時觸發它與 "
                f"{PENDING_ALERT}，兩條疊報。"
            )

    # 門檻 6：不得繞過 recording rule 直接寫進排除 regex
    for name in MUST_UNLESS:
        a = alerts.get(name)
        if not a:
            continue
        expr = str(a.get("expr", ""))
        # 🔴 比對前要把 regex 的跳脫正規化：expr 裡寫的是 `q\\.one`，而 queues 裡是
        # `q.one` —— 直接用 `in` 比對永遠對不上（第一版就是這樣，門檻 6 形同不存在）。
        head = expr.split("unless")[0].replace("\\\\.", ".").replace("\\.", ".")
        for q in queues:
            if q in head:
                errors.append(
                    f"[門檻6] {name} 的 expr 直接寫了 {q!r}（在 unless 之前的排除 regex 裡）。"
                    f"降噪一律走 {RECORD_NAME}，那才會被門檻 1–5 檢查到。"
                )
    return errors, queues


def self_test():
    """每條門檻各配一個該讓它失敗的變異（02-judgment.md §2.1.3）。"""
    import tempfile

    global RULES, TOPOLOGY
    real_r, real_t = RULES, TOPOLOGY
    failures = []

    def write(tmp, queues_regex, *, with_alert=True, unless=True, leak=False, topo_has=True):
        # 🔴 巢狀函式裡對 module 層變數賦值必須宣告 global，否則建立的是 local
        # ——第一版漏了，於是 check() 讀的仍是真實檔案，五個變異全部「通過」（實測）。
        global RULES, TOPOLOGY
        rules = {"groups": [{"name": "g", "rules": []}]}
        rr = rules["groups"][0]["rules"]
        if queues_regex is not None:
            rr.append({"record": RECORD_NAME,
                       "expr": 'rabbitmq_detailed_queue_consumers{queue=~"%s"} * 0 + 1' % queues_regex})
        if with_alert:
            rr.append({"alert": PENDING_ALERT, "expr": f"{RECORD_NAME} == 1",
                       "labels": {"severity": "warning", "cadence": "daily"},
                       "annotations": {"description": "見 #3365"}})
        for n in MUST_UNLESS:
            e = 'rabbitmq_detailed_queue_consumers{queue!~".*\\\\.dlq$"} == 0'
            if leak:
                e = 'rabbitmq_detailed_queue_consumers{queue!~"q\\\\.one"} == 0'
            if unless:
                e += f" unless on(env, queue, vhost) {RECORD_NAME}"
            rr.append({"alert": n, "expr": e, "labels": {"severity": "critical"}})
        RULES = Path(tmp) / "infra-alerts.yml"
        RULES.write_text(yaml.safe_dump(rules), encoding="utf-8")
        TOPOLOGY = Path(tmp) / "topology.yaml"
        TOPOLOGY.write_text(
            yaml.safe_dump({"queues": [{"name": "q.one"}] if topo_has else [{"name": "other"}]}),
            encoding="utf-8")

    cases = [
        ("清單空（regex 無 ^...$）", dict(queues_regex="nothing"), "門檻1"),
        ("queue 不在 topology", dict(queues_regex=r"^q\\.one$", topo_has=False), "門檻2"),
        ("有 recording rule 卻沒有專職告警", dict(queues_regex=r"^q\\.one$", with_alert=False), "門檻4"),
        ("既有告警沒 unless", dict(queues_regex=r"^q\\.one$", unless=False), "門檻5"),
        ("繞過：直接寫進排除 regex", dict(queues_regex=r"^q\\.one$", leak=True), "門檻6"),
    ]
    try:
        for desc, kw, want in cases:
            with tempfile.TemporaryDirectory() as d:
                write(d, **kw)
                errs, _ = check()
                if not any(want in e for e in errs):
                    failures.append(f"變異「{desc}」未被 {want} 抓到；實際：{errs}")
        # 陰性對照：合法配置必須零錯誤
        with tempfile.TemporaryDirectory() as d:
            write(d, queues_regex=r"^q\\.one$")
            errs, _ = check()
            if errs:
                failures.append(f"合法配置被誤判：{errs}")
    finally:
        RULES, TOPOLOGY = real_r, real_t

    # 門檻 3 用真實 repo 驗：偵測器對一個「確知有 consumer」的 queue 必須找得到出處，
    # 否則它對清單成員回「沒有實作」就不能當證據（feedback_no_blind_measurement）。
    errs, queues = check()
    if queues:
        probe = "aftersales.events"
        if find_consumer_impl(probe) is None:
            failures.append(
                f"門檻3 的偵測器對照組失敗：{probe!r} 確知有 consumer 卻找不到出處 ⇒ 偵測器無效，"
                f"它對真正的清單成員回「沒有實作」也就不能當證據"
            )

    if failures:
        print("❌ 自我測試失敗（本 gate 本身沒有鑑別力）：")
        for f in failures:
            print(f"   - {f}")
        return 1
    print(f"✅ 自我測試通過（{len(cases)} 個變異全部轉紅、合法配置為綠、門檻3 偵測器有對照組）")
    return 0


def main():
    if "--self-test" in sys.argv:
        return self_test()
    errors, queues = check()
    if errors:
        print("❌ pending-consumer queue 清單門檻未通過：\n")
        for e in errors:
            print(f"   {e}")
        print("\n判準與理由見本檔 docstring。")
        return 1
    if queues:
        print(f"✅ pending-consumer queue 清單門檻通過（{len(queues)} 個 queue 待實作 consumer）")
        for q in queues:
            print(f"   - {q}")
    else:
        print("✅ 沒有待實作 consumer 的 queue（清單未啟用）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
