#!/usr/bin/env python3
"""surefire 的 includes 必須涵蓋 `*IT.java`，否則那些測試被靜默跳過。

為什麼需要這支
--------------
maven-surefire 的**預設** includes 只有四個 pattern：

    **/Test*.java  **/*Test.java  **/*Tests.java  **/*TestCase.java

**不含 `**/*IT.java`**。所以一個叫 `XxxIT.java` 的測試，在沒有顯式 includes 的
模組裡會被**完全跳過**——而「零命中」在 surefire 是**綠燈**。

🔴 這個漏洞有三個性質，讓它特別難發現：

1. **本機看不出來**：`-Dtest=XxxIT` 是顯式指定所以會跑；而 CI 跑的是
   `-Dgroups=integration`，靠 includes 選檔 ⇒ 選不到。
2. **CI 也不會紅**：少跑測試不是失敗，`mvn` 照樣 exit 0。
3. **執行數下限擋不住**：那道檢查防的是「一支都沒跑」，而這裡是
   「少跑了 N 支、總數仍遠高於下限」。

2026-09-17 實測：`LogisticsCostFactJoinIT` 本機 8 支綠，CI 的 log 裡 **0 命中**，
而 `integration tests executed` 與前一輪**一模一樣**（188）。那 8 支從未在 CI 跑過。

判準
----
對每個 Java 服務模組：**有 `*IT.java` 就必須有涵蓋它的 includes**。
沒有 `*IT.java` 的模組不受拘束（加了也無妨，但不強制）。

⚠️ 本檢查刻意**不**驗「includes 的四個預設 pattern 有沒有寫全」：
一旦宣告 includes，預設就被覆蓋，漏寫任一個都會讓對應的測試消失——
但那一類的症狀是**大量**測試消失，執行數下限抓得到。本支專防的是
`*IT.java` 這個**預設本來就不含**、因而完全無聲的缺口。
"""

import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
JAVA_ROOT = REPO_ROOT / "services" / "java"

IT_GLOB = "*IT.java"
INCLUDE_PATTERN = re.compile(r"<include>\s*\*\*/\*IT\.java\s*</include>")


def main() -> int:
    if not JAVA_ROOT.is_dir():
        print(f"🔴 找不到 {JAVA_ROOT} ⇒ 本檢查的掃描範圍不存在，不是「沒有問題」",
              file=sys.stderr)
        return 2

    modules = sorted(p for p in JAVA_ROOT.iterdir()
                     if p.is_dir() and (p / "pom.xml").is_file())
    if not modules:
        print(f"🔴 {JAVA_ROOT} 底下掃不到任何含 pom.xml 的模組 ⇒ 掃描器壞了",
              file=sys.stderr)
        return 2

    # 🔴 對照組：先證明「找得到 *IT.java」這件事是有效的。
    #    少了這一條，glob 寫錯／路徑錯都會讓本檢查變成恆真。
    total_its = sum(len(list((m / "src" / "test").rglob(IT_GLOB)))
                    for m in modules if (m / "src" / "test").is_dir())
    if total_its == 0:
        print("🔴 全部模組都掃不到任何 *IT.java ⇒ 掃描器壞了，而不是「沒有 IT 測試」",
              file=sys.stderr)
        return 2

    problems = []
    checked = 0
    for module in modules:
        test_root = module / "src" / "test"
        if not test_root.is_dir():
            continue
        its = sorted(test_root.rglob(IT_GLOB))
        if not its:
            continue
        checked += 1
        pom = (module / "pom.xml").read_text(encoding="utf-8")
        if not INCLUDE_PATTERN.search(pom):
            problems.append((module.name, [p.name for p in its]))

    if problems:
        print("🔴 以下模組有 *IT.java，但 surefire 的 includes 沒有涵蓋它們 ⇒ "
              "那些測試在 CI 上被**靜默跳過**（零命中是綠燈）：", file=sys.stderr)
        for name, its in problems:
            print(f"   {name}: {', '.join(its)}", file=sys.stderr)
        print("", file=sys.stderr)
        print("   修法：在該模組 pom.xml 的 maven-surefire-plugin <configuration> 內加", file=sys.stderr)
        print("     <includes>", file=sys.stderr)
        print("       <include>**/Test*.java</include>", file=sys.stderr)
        print("       <include>**/*Test.java</include>", file=sys.stderr)
        print("       <include>**/*Tests.java</include>", file=sys.stderr)
        print("       <include>**/*TestCase.java</include>", file=sys.stderr)
        print("       <include>**/*IT.java</include>", file=sys.stderr)
        print("     </includes>", file=sys.stderr)
        print("   ⚠️ 四個預設 pattern 一定要一起寫："
              "宣告 includes 會**覆蓋**預設，只寫 *IT.java 會讓其他測試全部消失。",
              file=sys.stderr)
        return 1

    print(f"✅ surefire includes OK（{checked} 個模組有 *IT.java，"
          f"全部已涵蓋；共掃到 {total_its} 個 IT 檔）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
