# `uber_quotes` / `package_plan` 90 天保留期清理（66 號 `UP-19` 結案）

> 對應 [66-Uber Direct 即時配送整合詳細設計書](../詳細設計書/66-Uber%20Direct%20即時配送整合詳細設計書.md) §5.1.4／§5.1.8／§9.5。
> **DB**：`hawkr_shipping`（單一 database，無跨庫操作）。
> **連線**：bastion ＋ 1Password 取憑證，同 [food-safety runbook](./food-safety-incident-runbook.md) 的慣例；dev 先跑完整一輪再上 prod。
> **SQL**：[`sql/01-preview.sql`](./uber-quote-retention/sql/01-preview.sql) → [`sql/02-delete.sql`](./uber-quote-retention/sql/02-delete.sql) → [`sql/03-verify.sql`](./uber-quote-retention/sql/03-verify.sql)

---

## 🔴 0. 先讀這一段：**目前沒有排程執行者**

架構裁決 `DEC-20260912-184025` 第 15 條選定「維運 SQL」而非新增 application scheduler，
理由是它屬**保存期限清理**，不是核心 business state transition
（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)／CLAUDE.md §22：
資料完整性維運不進 application code）。

⚠️ **這份 runbook 的存在，不等於 90 天保留期已經被執行。**
本檔建立當下：

- shipping-service **沒有** `DataCleanupScheduler`（那支只在 payment-service 與 payuni-webhook-service）
- **沒有** cron、**沒有** k8s CronJob、**沒有** CI 排程會呼叫本目錄的任何一支 SQL
- 兩張表**沒有**任何體積或列數告警

⇒ 在有人週期性執行本 runbook 之前，**不得在任何文件、issue、交接或設計書裡宣稱
「90-day retention enforced」**（裁決第 15 條逐字要求）。現況的誠實描述是：
**「清理程序已就緒且已驗證，執行者尚未指定」**。

**要讓它成為真的保證，缺的是一件事**：指定執行週期與執行者（例如隨月度維運一併跑），
或改走選項 A（加排程器，但那要同時加合格的多副本搶佔，見 §5）。

---

## 1. 為什麼要清

`uber_quotes` 的 `EXPIRED` **只改 status 不刪列**（66 §5.1.4：帳務與客訴要查得到報價歷史），
`package_plan` 的生命週期**掛在 quote 上**（66 §5.1.8）。兩者都沒有終點 ⇒ **無限成長**。

而它的失效方式是本 repo 最貴的那一種：**沒有告警、沒有錯誤、查詢只是逐漸變慢**。
每一次結帳預覽都會寫一列 quote ＋ N 列 package_plan，而絕大多數 quote 從不會變成訂單。

---

## 2. 執行步驟

| # | 動作 | 判準 |
|---|---|---|
| 1 | 跑 `01-preview.sql` | **B 段必須 0 列**。非 0 ⇒ 停，走 §4 |
| 2 | 跑 `02-delete.sql`（預設 `ROLLBACK`） | 各 `DELETE` 筆數與 01 的 A/C 段對得上 |
| 3 | 把 `02-delete.sql` 結尾的 `ROLLBACK;` 改成 `COMMIT;`，重跑 | — |
| 4 | 跑 `03-verify.sql` | **三項全為 0**，否則這輪沒做完 |

**可重跑**：述詞是「終態 ＋ 逾期」，刪過的列不會再命中；第二次跑會全部 `DELETE 0`。

⚠️ `02` 的閘門跑在任何 `DELETE` 之前，且**不依賴人記得看 01 的 B 段** ——
B 段有列時它會 `RAISE EXCEPTION` 讓整筆交易失敗，一列都不會刪。

---

## 3. 刪除範圍：**只刪終態**

| `status` | 是否刪除 | 理由 |
|---|---|---|
| `CONSUMED` / `EXPIRED` / `SUPERSEDED` | ✅ 逾 90 天即刪 | 終態（66 §7.3.5），已無狀態要推 |
| `CREATED` / `CHECKOUT_ACCEPTED` | 🔴 **永不刪** | 非終態。逾期還停在這裡＝**有東西壞了**，見 §4 |

🔴 **這個分界是本 runbook 最重要的設計**：逾期的非終態列不是垃圾，是**唯一的證據**。
把清理寫成「凡逾 90 天一律刪」會在每次執行時把 bug 的痕跡一起銷毀，
而症狀是「那個問題好像自己好了」。

### 3.1 刪掉之後會失去什麼（明寫，不要事後才發現）

`uber_deliveries` **永不刪除**（66 §5.1.5，帳務與爭議要追溯），而它帶著 `quote_id`。
quote 被清掉之後，那個 `quote_id` 變成**懸空參照** ——「這筆配送的報價當時長什麼樣」
（`pickup_json` / `dropoff_json` / `items_json` / `payload_hash`）就查不到了。

這是 66 的設計意圖（保留期就是 90 天），不是缺陷。前提是：

- 買家實際被收的運費權威是 **`orders.shipping_fee`**（order-service `V1__init_order_tables.sql:33`，訂單快照），不是 `uber_quotes`
- provider 實際成本權威是 **`logistics_receivables.actual_cost`**（payment-service `V9:68`）＋ **`provider_settlement_lines`**（`V9:438`），對帳關聯鍵是 `provider_delivery_ref = uber_deliveries.delivery_id`（`V9:64`），**不是 quote_id**
- 發票金額來自訂單快照 `Snapshot.ShippingFee`（invoice-service `internal/domain/invoice_lines.go:67`、`:199`），與 `uber_quotes` 無關

⇒ 三條財務／憑證軸都**不以 `uber_quotes` 為權威**，清掉它不影響任何一條。
（逐項證據與對照組見 §6。）

---

## 4. 01 的 B 段有列時怎麼辦

**不要刪，去查。** 兩種 status 指向兩個不同的缺陷：

| status 逾期 | 代表 | 查哪裡 |
|---|---|---|
| `CREATED` | **TTL 掃描沒在跑** —— 它本來該把逾期的 `CREATED` 推成 `EXPIRED`（66 §5.1.4，候選集合＝`idx_uber_quotes_expiry`） | 該排程器是否存在／是否被搶佔卡死 |
| `CHECKOUT_ACCEPTED` | 🔴 **收了錢沒建單**（66 §5.1.4 逐字）。監控指標 `uber_quote_stuck_rows{status}` 本來就該在 30 分鐘時響 | 建單鏈（`UberDirectAdapter`）＋ 為什麼指標沒響 |

處理完根因、那些列被推到終態之後，再回到 §2 第 1 步。

---

## 5. 為什麼不是排程器（裁決第 15 條的取捨紀錄）

| 選項 | 代價 |
|---|---|
| A. shipping-service 加 `@Scheduled` 清理器 | 依[多副本排程搶佔規範](../開發規範/多副本排程搶佔規範.md)⑧，**加排程就要同時加合格搶佔**（lease，不是永久旗標），否則是「擴容那一刻才引爆的地雷」；且它屬資料完整性維運，依 §22 不進 application code |
| B. 維運 SQL（**採用**） | 沒有 code、沒有搶佔問題；代價是「誰記得去跑」沒有機械保證 —— 這正是 §0 要寫明的那件事 |

🔴 **若日後 `uber_quotes` 被納入帳務對帳來源**，保存期限就從清理策略變成**業務約束**，
屆時本裁決要重開：那時 §3.1 的三條「不是權威」會有一條變成假的。

---

## 6. §3.1 三項判定的證據（含對照組）

| 判定 | 證據 | 對照組 |
|---|---|---|
| **不是 provider 實際成本權威** | 成本欄在 payment-service `V9__add_logistics_finance_tables.sql:66`（`estimated_cost`）、`:68`（`actual_cost`，🔴 NULL ＝ 尚未對帳不是 0）；帳單來源 `provider_settlement_lines`（`V9:438`，`statement_ref`/`recon_status`）；關聯鍵 `provider_delivery_ref`（`V9:64`，註解逐字「uber_deliveries.delivery_id；對帳的關聯鍵」） | 同一支 `/usr/bin/grep -rn` 在 payment-service 查 `uber_quotes` 零命中、查 `logistics_receivables` 有命中 ⇒ 查詢有效 |
| **不是發票憑證** | invoice-service 的運費來自傳入快照 `Snapshot.ShippingFee`（`internal/domain/invoice_lines.go:67` 註解逐字「已是買家實際被收的金額」、`:199` 組發票行） | `services/invoice-service` 目錄實存且可掃（339 檔）；同一支 grep 查 `uber_quotes` 零命中。⚠️ `services/java/invoice-service` **不存在**（invoice 是 Go 服務），拿它當查詢範圍會得到空洞的否定 |
| **不是帳務對帳權威** | 對帳走 `provider_settlement_lines` × `logistics_receivables`，分錄在 `ledger_transactions`/`ledger_entries`（`V9:240`/`:304`，含 `assert_ledger_balanced()` 的 DEFERRABLE 平衡 trigger）；全鏈無一處讀 `uber_quotes` | 同上 |

補充：`uber_quotes` 的讀取端**全部在 shipping-service 內**
（`UberQuoteService` / `UberQuoteWriter` / `UberDirectAdapter` / `ShippingGrpcServiceImpl`），
`UberQuoteMapper` 目前零自訂 `@Select`；跨服務只透過 gRPC 回傳 `fee_amount` **值**，
消費端拿到的是數字，不是對本表的參照。
