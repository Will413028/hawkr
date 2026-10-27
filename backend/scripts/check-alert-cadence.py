#!/usr/bin/env python3
"""告警投遞節奏（cadence label）的門檻檢查。

## 這道 gate 守什麼

`cadence` label 讓一條告警宣告「我是狀態型的、我會持續 firing 很久」，
Alertmanager 據此把它的 `repeat_interval` 從 30m/2h 放寬到一天一則
（`monitoring/alertmanager/config.yml` 的 `cadence` 子 route）。

它解決的是真問題：business 頻道同時承載「要立刻處理的異常」與「有一件事
一直沒做」，後者以 48 則/天洗版會把前者蓋掉（#3269 的 InvoiceMobileBarcode
VerifierDisabled 就是這樣被抱怨的）。

但它同時是一個**很好用的逃生口**：嫌哪條告警吵，貼上去就安靜了。本檔存在的
唯一理由是讓那件事做不到。

## 門檻（六條，全部可機械執行）

1. `cadence` 的值必須在 ALLOWED_CADENCES 內 —— 打錯字會讓 label 靜默無效
   （Alertmanager 沒有匹配的 route ⇒ 照原頻率投遞，而沒有任何東西會說）。
2. 帶 `cadence` 的告警，annotations 必須引用 issue 編號（`#NNNN`）。
   降頻的正當性來自「有人在追這件事」；沒有票 ＝ 沒有人在追 ＝ 它就該吵。
3. `cadence` 不得貼在 `severity: critical` 上。critical 的定義就是「要立刻
   處理」——一條需要降頻的 critical，錯的是 severity 不是節奏。
4. **逐條模擬路由**：拿每條 cadence 告警的 labels（× 每個已知 env）實際走一次
   route 樹，最終落點必須是 cadence 子 route。
   ⚠️ 這一條原本寫成「cadence 值有沒有被宣告過」的集合比較，2026-09-12 用真實
   變異測出它是盲的：拿掉 prod 的子 route 而 dev 的還在 ⇒ 集合差為空 ⇒ 放行，
   但 prod 實際照 30m 投遞。**「某處宣告過」與「我這條走得到」是兩件事**，
   而漏掉的那一側完全靜默（06-no-small-tails.md §3.10：機制挑了一個觀測窗，
   卻沒列舉真實情況會怎麼跨越它）。
5. 反向：Alertmanager 裡宣告的 cadence route 必須有告警在用，否則是死設定
   （規則被刪掉後 route 留著，下一個人會以為還有東西靠它）。
6. **不得用 alertname 白名單降頻。** 那是繞過本 gate 的唯一途徑：直接在
   Alertmanager 加一條 `match: {alertname: X}` + 長 repeat_interval，規則檔
   完全不用動 ⇒ 上面五條一條都看不到。唯一豁免是 Watchdog（它 route 到
   blackhole，是「完全不送」而非「降頻」，語意不同且已有註解說明）。

## 它擋不住什麼（誠實標明，06-no-small-tails.md §3.10 的收尾要求）

- **票是不是 open**：只驗 annotations 裡有 `#NNNN`，不查 GitHub。查了會讓
  這道 gate 依賴網路與權杖、在 fork PR 上必然失敗。⇒ 票關掉之後 label 仍然
  合法。要接住那一格，靠的是 issue 關閉時回頭看有沒有留下 cadence label。
- **理由對不對**：驗得到「有票」，驗不到「這張票真的說明了為什麼會持續 firing」。
- **Alertmanager 的實際行為**：本 gate 讀的是 YAML 結構，不是 amtool。
  route 繼承語意的驗證要靠 `amtool config routes test`（2026-09-12 已實跑：
  帶 cadence 走子 route、不帶的 critical 不受影響）。
"""
import re
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parent.parent
RULES_DIR = REPO / "monitoring" / "prometheus" / "rules"
AM_CONFIG = REPO / "monitoring" / "alertmanager" / "config.yml"

ALLOWED_CADENCES = {"daily"}


def rel(path):
    """顯示用的相對路徑；self-test 會把目標指到 tempdir，那裡 relative_to 會拋 ValueError。"""
    try:
        return path.relative_to(REPO)
    except ValueError:
        return path
ISSUE_RE = re.compile(r"#\d{3,}")
# alertname 白名單降頻的唯一合法例外：它送到 blackhole（完全不送），不是降頻。
ALERTNAME_ROUTE_EXEMPT = {"Watchdog"}


def collect_cadence_alerts(errors):
    """回傳 [(檔名, alertname, cadence, severity, annotations 全文)]。"""
    if not RULES_DIR.is_dir():
        # 門檻 §3.10 第 4 問：目標不存在時 FAIL 而不是 SKIP——目錄被搬走的那天
        # 這道 gate 不可以靜默轉綠。
        errors.append(f"[結構] 規則目錄不存在：{rel(RULES_DIR)}")
        return []

    files = sorted(RULES_DIR.glob("*.yml")) + sorted(RULES_DIR.glob("*.yaml"))
    if not files:
        errors.append(f"[結構] {rel(RULES_DIR)} 內沒有任何規則檔")
        return []

    found = []
    for f in files:
        doc = yaml.safe_load(f.read_text(encoding="utf-8")) or {}
        for group in doc.get("groups") or []:
            for rule in group.get("rules") or []:
                name = rule.get("alert")
                if not name:
                    continue  # recording rule
                labels = rule.get("labels") or {}
                if "cadence" not in labels:
                    continue
                anns = rule.get("annotations") or {}
                found.append(
                    (
                        f.name,
                        name,
                        str(labels["cadence"]),
                        str(labels.get("severity", "")),
                        " ".join(str(v) for v in anns.values()),
                        {k: str(v) for k, v in labels.items()},
                    )
                )
    return found


def collect_am_routes(errors):
    """回傳 (root route, 宣告的 cadence 值集合, 違規的 alertname 降頻 route, 已知 env 值)。"""
    if not AM_CONFIG.is_file():
        errors.append(f"[結構] Alertmanager 設定不存在：{rel(AM_CONFIG)}")
        return None, set(), [], set()

    cfg = yaml.safe_load(AM_CONFIG.read_text(encoding="utf-8")) or {}
    cadences, alertname_routes, envs = set(), [], set()

    def walk(route):
        m = route.get("match") or {}
        me = route.get("match_re") or {}
        if "cadence" in m:
            cadences.add(str(m["cadence"]))
        if "env" in m:
            envs.add(str(m["env"]))
        # 門檻 6：alertname 白名單 + 自訂 repeat_interval ＝ 繞過本 gate 的降頻
        an = m.get("alertname") or me.get("alertname")
        if an and route.get("repeat_interval") and route.get("receiver") != "blackhole":
            if str(an) not in ALERTNAME_ROUTE_EXEMPT:
                alertname_routes.append(str(an))
        for child in route.get("routes") or []:
            walk(child)

    root = cfg.get("route")
    if not root:
        errors.append("[結構] Alertmanager 設定沒有 route 區塊")
        return None, set(), [], set()
    walk(root)
    return root, cadences, alertname_routes, envs


def resolve_route(root, labels):
    """模擬 Alertmanager 的 first-match-wins 路由，回傳最終落點的 route 物件。

    只實作本 repo 用到的 `match`（全等）。`match_re` 目前零使用；真的有人加了,
    這裡會把它當成不匹配 ⇒ 傾向報錯而不是放行（fail-closed）。
    """
    node = root
    while True:
        for child in node.get("routes") or []:
            m = child.get("match") or {}
            if m and all(str(labels.get(k)) == str(v) for k, v in m.items()):
                node = child
                break
        else:
            return node


def check():
    errors = []
    alerts = collect_cadence_alerts(errors)
    root, declared, alertname_routes, envs = collect_am_routes(errors)

    used = set()
    for fname, name, cadence, severity, anns, labels in alerts:
        used.add(cadence)
        where = f"{fname} :: {name}"
        # 門檻 1
        if cadence not in ALLOWED_CADENCES:
            errors.append(
                f"[門檻1] {where}：cadence={cadence!r} 不在允許集合 {sorted(ALLOWED_CADENCES)}。"
                "打錯字不會報錯，只會讓 label 靜默無效（照原頻率投遞）。"
            )
        # 門檻 2
        if not ISSUE_RE.search(anns):
            errors.append(
                f"[門檻2] {where}：annotations 沒有 issue 編號（#NNNN）。"
                "降頻的正當性來自『有人在追這件事』；沒有票就該讓它吵。"
            )
        # 門檻 3
        if severity == "critical":
            errors.append(
                f"[門檻3] {where}：critical 不得降頻。"
                "critical 的定義就是要立刻處理——需要降頻代表 severity 標錯了，改 severity 而不是改節奏。"
            )

    # 門檻 4：逐條模擬路由。集合比較擋不住「只補了一個 env」——見 docstring。
    if root is not None:
        for fname, name, cadence, severity, _anns, labels in alerts:
            for env in sorted(envs) or [None]:
                probe = dict(labels)
                probe["alertname"] = name
                if env is not None:
                    probe["env"] = env
                landed = resolve_route(root, probe)
                if "cadence" not in (landed.get("match") or {}):
                    errors.append(
                        f"[門檻4] {fname} :: {name}（env={env}）：走完 route 樹之後**沒有**落在 cadence 子 route "
                        f"⇒ 這條告警在該 env 仍照 repeat_interval={landed.get('repeat_interval', '繼承上層')} 投遞，"
                        f"cadence label 對它完全沒有作用而且靜默。"
                        f"請在 {rel(AM_CONFIG)} 對應的 route 底下補 cadence 子 route。"
                    )
    # 門檻 5：宣告了卻沒人用 ＝ 死設定
    for c in sorted(declared - used):
        errors.append(
            f"[門檻5] Alertmanager 宣告了 cadence={c!r} 的 route，但沒有任何告警在用 ⇒ 死設定，請移除。"
        )
    # 門檻 6
    for an in sorted(set(alertname_routes)):
        errors.append(
            f"[門檻6] Alertmanager 用 alertname={an!r} 做降頻 route。"
            "降頻一律走 cadence label（才會被本 gate 的門檻 1–3 檢查到）；alertname 白名單繞過全部門檻。"
        )
    return errors, alerts


SELF_TEST_CASES = [
    # (說明, 規則 YAML, 期望命中的門檻編號)
    ("值打錯字", {"alert": "X", "labels": {"cadence": "hourly", "severity": "warning"},
                  "annotations": {"description": "見 #1234"}}, "門檻1"),
    ("沒有 issue", {"alert": "X", "labels": {"cadence": "daily", "severity": "warning"},
                    "annotations": {"description": "這條很吵"}}, "門檻2"),
    ("critical 降頻", {"alert": "X", "labels": {"cadence": "daily", "severity": "critical"},
                       "annotations": {"description": "見 #1234"}}, "門檻3"),
]


def self_test():
    """每條門檻各配一個『該讓它失敗』的變異（02-judgment.md §2.1.3）。

    沒有這個，本 gate 可能是一道恆真的斷言而沒有人會發現。
    """
    import tempfile

    global RULES_DIR, AM_CONFIG
    real_rules, real_am = RULES_DIR, AM_CONFIG
    failures = []
    try:
        for desc, rule, want in SELF_TEST_CASES:
            with tempfile.TemporaryDirectory() as d:
                RULES_DIR = Path(d) / "rules"
                RULES_DIR.mkdir()
                (RULES_DIR / "t.yml").write_text(
                    yaml.safe_dump({"groups": [{"name": "g", "rules": [rule]}]}), encoding="utf-8"
                )
                AM_CONFIG = Path(d) / "am.yml"
                AM_CONFIG.write_text(
                    yaml.safe_dump(
                        {"route": {"receiver": "r", "routes": [
                            {"match": {"cadence": "daily"}, "repeat_interval": "24h"},
                            {"match": {"cadence": "hourly"}, "repeat_interval": "24h"},
                        ]}}
                    ),
                    encoding="utf-8",
                )
                errs, _ = check()
                if not any(want in e for e in errs):
                    failures.append(f"變異「{desc}」未被 {want} 抓到；實際錯誤：{errs}")

        # 門檻 4-a：規則用了但 AM 完全沒宣告
        with tempfile.TemporaryDirectory() as d:
            RULES_DIR = Path(d) / "rules"
            RULES_DIR.mkdir()
            (RULES_DIR / "t.yml").write_text(
                yaml.safe_dump({"groups": [{"name": "g", "rules": [
                    {"alert": "X", "labels": {"cadence": "daily", "severity": "warning"},
                     "annotations": {"description": "見 #1234"}}]}]}), encoding="utf-8")
            AM_CONFIG = Path(d) / "am.yml"
            AM_CONFIG.write_text(yaml.safe_dump({"route": {"receiver": "r"}}), encoding="utf-8")
            errs, _ = check()
            if not any("門檻4" in e for e in errs):
                failures.append(f"變異「label 完全沒人接」未被門檻4 抓到；實際：{errs}")

        # 門檻 4-b：**只補了一個 env**。這一格是 2026-09-12 用真實變異測出來的盲點——
        # 舊版門檻 4 做的是集合比較（「daily 有沒有被宣告過」），dev 有而 prod 沒有時
        # 集合差為空 ⇒ 放行，而 prod 實際照 30m 投遞。少了這個 case，同一個盲點會回來。
        with tempfile.TemporaryDirectory() as d:
            RULES_DIR = Path(d) / "rules"
            RULES_DIR.mkdir()
            (RULES_DIR / "t.yml").write_text(
                yaml.safe_dump({"groups": [{"name": "g", "rules": [
                    {"alert": "X", "labels": {"cadence": "daily", "severity": "warning",
                                              "channel": "business"},
                     "annotations": {"description": "見 #1234"}}]}]}), encoding="utf-8")
            AM_CONFIG = Path(d) / "am.yml"
            AM_CONFIG.write_text(yaml.safe_dump({"route": {"receiver": "root", "routes": [
                {"match": {"channel": "business", "env": "dev"}, "receiver": "dev-ch",
                 "repeat_interval": "2h",
                 "routes": [{"match": {"cadence": "daily"}, "repeat_interval": "24h"}]},
                # prod 這條刻意沒有 cadence 子 route
                {"match": {"channel": "business", "env": "prod"}, "receiver": "prod-ch",
                 "repeat_interval": "30m"},
            ]}}), encoding="utf-8")
            errs, _ = check()
            if not any("門檻4" in e and "env=prod" in e for e in errs):
                failures.append(f"變異「只補了 dev、漏掉 prod」未被門檻4 抓到；實際：{errs}")
            if any("env=dev" in e for e in errs):
                failures.append(f"門檻4 對已正確設定的 dev 誤報；實際：{errs}")

        # 門檻 6：alertname 白名單降頻
        with tempfile.TemporaryDirectory() as d:
            RULES_DIR = Path(d) / "rules"
            RULES_DIR.mkdir()
            (RULES_DIR / "t.yml").write_text(
                yaml.safe_dump({"groups": [{"name": "g", "rules": []}]}), encoding="utf-8")
            AM_CONFIG = Path(d) / "am.yml"
            AM_CONFIG.write_text(yaml.safe_dump({"route": {"receiver": "r", "routes": [
                {"match": {"alertname": "TooNoisy"}, "receiver": "x", "repeat_interval": "24h"}]}}),
                encoding="utf-8")
            errs, _ = check()
            if not any("門檻6" in e for e in errs):
                failures.append(f"變異「alertname 降頻」未被門檻6 抓到；實際：{errs}")

        # 陰性對照：合法配置必須零錯誤，否則本 gate 是恆真斷言
        with tempfile.TemporaryDirectory() as d:
            RULES_DIR = Path(d) / "rules"
            RULES_DIR.mkdir()
            (RULES_DIR / "t.yml").write_text(
                yaml.safe_dump({"groups": [{"name": "g", "rules": [
                    {"alert": "X", "labels": {"cadence": "daily", "severity": "warning",
                                              "channel": "business"},
                     "annotations": {"description": "見 #1234"}}]}]}), encoding="utf-8")
            AM_CONFIG = Path(d) / "am.yml"
            AM_CONFIG.write_text(yaml.safe_dump({"route": {"receiver": "root", "routes": [
                {"match": {"channel": "business", "env": "dev"}, "receiver": "dev-ch",
                 "repeat_interval": "2h",
                 "routes": [{"match": {"cadence": "daily"}, "repeat_interval": "24h"}]},
                {"match": {"channel": "business", "env": "prod"}, "receiver": "prod-ch",
                 "repeat_interval": "30m",
                 "routes": [{"match": {"cadence": "daily"}, "repeat_interval": "24h"}]},
            ]}}), encoding="utf-8")
            errs, _ = check()
            if errs:
                failures.append(f"合法配置被誤判：{errs}")
    finally:
        RULES_DIR, AM_CONFIG = real_rules, real_am

    if failures:
        print("❌ 自我測試失敗（本 gate 本身沒有鑑別力）：")
        for f in failures:
            print(f"   - {f}")
        return 1
    print(f"✅ 自我測試通過（{len(SELF_TEST_CASES) + 4} 個變異全部轉紅、合法配置為綠）")
    return 0


def main():
    if "--self-test" in sys.argv:
        return self_test()
    errors, alerts = check()
    if errors:
        print("❌ 告警 cadence 門檻未通過：\n")
        for e in errors:
            print(f"   {e}")
        print("\n判準與理由見本檔 docstring。")
        return 1
    print(f"✅ 告警 cadence 門檻通過（{len(alerts)} 條告警宣告了投遞節奏）")
    for fname, name, cadence, severity, _anns, _labels in alerts:
        print(f"   - {name}  cadence={cadence}  severity={severity}  ({fname})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
