# 67　定期配送訂閱（Subscription / Recurring Delivery）詳細設計書

> 共用規約（identity／金額／時間／命名／API envelope／錯誤碼號段／CodeMeta 流程／MQ 兩套 SSOT／
> 排程器判準／新服務登記／媒體 objectKey）**只在 65 §3.9 定義一次，本書只引用**。

---

## 0. 關鍵參數速查

> 本章每一格都是**可被實作直接抄走的數值**。凡標「本書定案」者，需求書未給，推導寫在 §3.7；
> 凡標「待裁決」者，**在裁決前對應 feature flag 維持關閉**（需求書 §20.25 逐字要求）。

### 0.1 🔴 時間軸與**兩個** cutoff（承接 D-10）

🔴 **時間軸有兩個獨立的截止，不是一個**：`edit_cutoff_at`（停止修改）與 `charge_cutoff_at`（停止收款）。
**合成一個名詞會讓「在截單前最多重試兩次」的視窗長度變成負的**——兩者在時間線上相隔一天。

| 時間點 | 名稱 | 需求書原位置 | 語意 | 誰在這一刻動作 |
|---|---|---|---|---|
| T-7 | `forecast_at` | §20.6 T-7 | 生成／刷新 cycle、軟性供貨確認、寄「即將配送」通知 | `RecurringCycleRollingScheduler`（R-S1） |
| **T-3** | **`edit_cutoff_at`** | §20.6「T-3 至 T-2」 | 🔴 **買家可改截止**：skip／reschedule／改地址／改時段／改品項一律關閉；承載檢查（🔴 **MVP 無硬保留庫存**，見 §3.4）；`SCHEDULED → LOCKED` | `RecurringCycleLockScheduler`（R-S2） |
| T-2 → T-1 | 重試視窗 | §20.6「T-2 至 T-1」 | 扣款發動與重試都必須落在這 24h 內 | `RecurringChargeScheduler`（R-S3） |
| **T-1** | **`charge_cutoff_at`** | §20.6「T-2 至 T-1」的右端 | 🔴 **收款截止**：此刻仍未 captured ⇒ 不再嘗試，該期落 **`CANCELLED{CHARGE_FAILED, was_charged=false}`**（與 §7.1 步驟 5／6、§7.3.2 `CHARGING` 出邊、§7.4 同一個值）。🔴 **不是 `SKIPPED`**——`SKIPPED` 的 allowed-from 只有 `SCHEDULED`（§7.3.2），從 `CHARGING` 寫它一律 CAS `rows=0`，而呼叫端會把 `rows=0` 讀成「別人先做過了」 | R-S3 |
| T-0 | `scheduled_for_at` | §20.6 | 預定到貨日（Asia/Taipei 當地日，存 UTC） | 69 的 `merchant_delivery_job` |

🔴 **兩種寫法要分清楚，否則號誌會反向**（D-10 用 T-minus 記法，DB CHECK 必須用絕對時間）：

```
T-minus 記法（D-10 原文）：edit_cutoff ≥ charge_cutoff + 重試視窗
絕對時間記法（本書規範形式，DB CHECK 用這個）：
  ① edit_cutoff_at   ≤ charge_cutoff_at − retry_window        -- 先鎖單才收款
  ② scheduled_for_at − charge_cutoff_at ≥ seller_lead_time + delivery_prep_time
  ③ charge_cutoff_at <  scheduled_for_at
```

| 參數 | 值 | 來源 |
|---|---|---|
| `retry_window` | **24 小時**（T-2 → T-1） | §20.6 時間線 |
| `edit_cutoff` 預設 | **T-3**（賣家可設，受 ① 約束） | §20.6 |
| `charge_cutoff` 預設 | **T-1**（賣家可設，受 ①②③ 約束） | §20.6 |
| `seller_lead_time_hours` | 賣家設定，**無預設**（publish gate 必填） | 需求書 §20.18。🔴 它是 §3.4.2 第二條不等式的輸入，**缺它整條時間軸算不出來** |
| `delivery_prep_time_hours` | 預設 **12**（本書定案；賣家可覆寫） | 本書定案（§3.7-④） |

### 0.2 排程器名冊與 deterministic key

🔴 **排程器名冊只有一份，在 §7.0（`R-S1`～`R-S7`）**——候選述詞、deterministic key、
租約長度全部以該表為準。**全書引用排程器一律用 `R-Sn`。**

⚠️ 另有兩支**非排程**的收斂路徑，不在該名冊內：扣款 `UNKNOWN` 回查（D-1 義①，事件＋回查）與
hold 釋放（由 R-S2／R-S4／取消路徑在同一交易內釋放，非掃描器）。

### 0.3 租約算式（65 §九：租約 **>** 單筆最壞耗時（含每個外部呼叫 deadline）、**<** 候選窗口剩餘壽命）

| 排程器 | 單筆最壞耗時 | 候選窗口 | **租約定案** |
|---|---|---|---|
| R-S1 | 3s（純本地 ＋ 一次 product/shipping 讀取，各 1s deadline） | 7 天 | **60s** |
| R-S2 | 15s（capacity reserve 5s ＋ 本地 tx，含重試）🔴 **不含 inventory reserve**——MVP 無庫存硬保留（§3.7） | 48h（T-3 → T-1） | **120s** |
| R-S3 | **130s**（PAYUNi `/api/credit` 同步等待上限 **60s** ＋ 逾時後查詢 60s ＋ 本地 10s） | 24h | **300s** |
| R-S4 | 30s（order-service CreateOrder 10s ＋ 查詢重試） | 24h | **120s** |
| R-S5／R-S6／R-S7 | 5s | ≥ 24h | **60s** |

🔴 **租約基準逐筆在 claim 當下取**，不得在批次迴圈外算一次（65 §九逐字；#2294 C1 的實例是
「30s 租約 < 100s 外部 deadline ⇒ 持有者還在打第三方就被合法搶走 ⇒ 重複退款」）。
🔴 R-S3 的租約下界由 PAYUNi 的 60s UNKNOWN 門檻決定（§20.14 逐字），**不得低於 130s**。

### 0.4 hold 壽命

| 欄位 | 定案 | 理由 |
|---|---|---|
| **兩張 hold 表的 `expires_at`** | **`= GREATEST(scheduled_for_at + 12h, charge_cutoff_at + 24h)`** —— 🔴 **全書唯一公式，§7.4 不另立** | 前一項涵蓋 materialize 重試與當日出貨；後一項是 INV-10（RESERVE 與 CHARGE 之間的空窗 ＋ 扣款後的 materialize 重試窗，租約必須活過它） |
| `recurring_capacity_hold` | 同公式 | 需求書 §20.16「地址、商品、包裝或 service window 任一改變 ⇒ 舊 hold 立即失效」由 `snapshot_hash` 比對強制，不靠時間 |

🔴 **INV-10 的執行面：把 `charge_cutoff_at` 複製一份進 hold 表，CHECK 就變成單表的。**

`expires_at ≥ charge_cutoff_at + 24h` 的兩個運算元原本分屬 `recurring_capacity_hold` 與
`recurring_cycle`，而 PostgreSQL 的 table CHECK **不能引用另一張表** ⇒ 照字面施工只會留下一條
**永遠紅不了的不變式**（比沒有更糟，**假的保證會排擠掉真的警覺**）。

🔴 **做法**：`recurring_inventory_hold` 與 `recurring_capacity_hold` 各加一欄
`charge_cutoff_at timestamptz NOT NULL`，於 lock 步驟自同一個 cycle 列複製，
再掛單表 CHECK `expires_at >= charge_cutoff_at + INTERVAL '24 hours'`。

⚠️ **這個冗餘欄沒有漂移面**：`cycle.charge_cutoff_at` 與 hold 列都在 lock 那一刻定案且此後不可變
（cycle 進 `LOCKED` 後不再改時間軸，hold 只會被釋放不會被改期）⇒ 沒有第二個寫入者。

被否決的替代解見 [ADR 2026-09-18-subscription-hold-expiry-redundant-column](../adr/2026-09-18-subscription-hold-expiry-redundant-column.md)。

⇒ INV-10 因此是**真的 DB 強制**，而 `chk_rcy_time_order` 維持原本的
`edit_cutoff_at < charge_cutoff_at < scheduled_for_at`，不碰賣家的設定空間。

### 0.5 🔴 三個 amount cap

需求書 §20.4 逐字：「買家可主動設定較高的**商品、物流與當期總額**上限」⇒ **三個獨立的 cap，各自一欄**。
🔴 **不得只建一個 `amount_cap`**——那樣另外兩個上限**無處可存**。

| 欄位 | 語意 | 預設 |
|---|---|---|
| `goods_amount_cap` | 當期商品淨額上限 | 建約當下的首期 `goods_net` |
| `delivery_amount_cap` | 當期自配費（扣補貼後）上限 | 建約當下的首期 `seller_delivery_charge − subsidy`；**容忍 0** |
| `cycle_total_amount_cap` | 當期總扣款上限 | 上兩者之和 |

🔴 **漲價容忍預設為 0**（需求書 §20.4 逐字）⇒ 預設值即當期金額本身；**超過任一上限即 `ACTION_REQUIRED`，不得暗扣**。
🔴 **三者皆為授權範圍，不是信用額度**（§20.3 逐字）；變更任一上限**需重新同意**（§8.7）。

### 0.6 重試次數與政策容忍時間

| 參數 | 值 | 來源 |
|---|---|---|
| charge 重試上限 | **`attempt_no ≤ 3`**（首次 ＋ 最多 2 次重試） | §20.6／§20.14「在 `charge_cutoff` 前最多重試兩次」 |
| 重試最小間隔 | **6 小時** | §20.14「避免同分鐘連打」⇒ 本書定案；3 次 × 6h = 12h < 24h 視窗 |
| 重試前置 | 🔴 每次重試前**必須先 query 上一次的結果**（§20.14「UNKNOWN 一律 query-first」） | §20.14 |
| **政策容忍時間**（漏跑） | **6 小時** | 需求書只寫「超過政策容忍時間不可無條件 catch-up」未給值 ⇒ **本書定案**（§3.7-⑤）。超過 ⇒ 轉 incident（`ACTION_REQUIRED` ＋ ADM case），**不扣款** |
| PAUSE 上限 | **90 天** | §20.6 逐字 |
| rolling horizon | **8 週（56 天）** | §20.6 逐字。🔴 **這是唯一權威**——§7.0 的 R-S1 述詞原寫 90 天，已一併改為 56 天；§7.0／§7.1／§7.3.1 **均已回灌為 56 天** |

### 0.7 本書新增的碼值與錯誤碼速查

**CodeMeta 新增**（落碼流程與時點見 §12.2／附錄 C.3）

| Enum | 值數 | 編碼選擇 | 依據 |
|---|---|---|---|
| `RecurringContractStatus` | **7** | Value 採 UPPER_SNAKE 字串 | 值域見 §7.3.1；沿用 58 號 `InvoiceDocumentStatus` 的慣例（值會出現在 ADM query 與告警訊息，數字碼無法直讀） |
| `RecurringCycleStatus` | **10** | 同上 | §7.3.2（值域見 §5.1.6 的 `chk_rcy_status`） |
| `RecurringPricePolicy` | **3**（`FIXED_PER_CYCLE`／`CURRENT_PRICE_MINUS_DISCOUNT`／`FIXED_TERM_PREPAID`） | 同上 | 需求書 §20.4 |
| `RecurringEventType` | **22**（16 ＋ 補 6） | 同上 | §6.5，本書 §0.7 只記數 |

🔴 **不新建這三個 enum。** 🔴 **欄名與 enum 名是兩件事，分開列**（混在同一句裡列，正是它們各錯一個字的來源）：

| | 三個政策維度 | SSOT |
|---|---|---|
| **欄名** | `fulfillment_mode`／`collection_model`／`invoice_model` | 🔴 **65 §5.1.3 的 `orders` DDL**——65 是骨幹，**欄名屬骨幹 SSOT** |
| **enum 名** | `FulfillmentMode`／`CollectionModel`／`InvoiceModel` | 65 附錄 C.2 |
| **值域** | `CollectionModel` 四值：`SELLER_MERCHANT`／`PLATFORM_AGENT_MERCHANT`／`PROVIDER_COLLECTED`／`EXTERNAL_PREPAID`（本書恆取第一個） | 🔴 **65 附錄 C.2**（65 §5.1.2 列的是同一份，非第二個 SSOT；C.2 該格 2026-09-08 已定案） |

本書**只固定其取值**（§3.1），不另建。再開一份就是 D-5 明禁的「並存兩套值域高度重疊的 enum」。
⚠️ **欄名是 `fulfillment_mode`／`collection_model`／`invoice_model`**——
不是 `fulfillment_policy`／`collection_mode`。欄名的 SSOT 只有 65 §5.1.3 的 `orders` DDL。

**錯誤碼**

🔴 **本書不自行取號。** 65 §6.7.0 逐字：「號碼由本節一次配完，**66／67／68／69 只引用不自行取號**——
各本自己在段內挑會撞號，而撞號只在生成階段才發現。」
⇒ 本書 §6.7 只交出**語意清單**（**37 條**，逐條見 §6.7；去重後淨需求併入 65 C.5.1 的統計），
由 65 §6.7 在 🔴 **`9500~9699`（200 個）** 內配號——**65 佔 `9501~9575`，66／67／68／69 自 `9576` 起連號追加**。
**在 65 配號完成前，本書任何端點不得寫死業務碼。**
⚠️ 兩個**已在需求書出現、名稱要照抄**的碼：`RECURRING_FULFILLMENT_NOT_ALLOWED`（§20.11）、
`RECURRING_SHIPPING_POLICY_VIOLATION`（需求書 §20.18 ADM queue 名）
—— 🔴 **兩者都在 §6.7「試算履約模式」那一群裡**，不得再另創同義碼。
🔴 **號段擴充的門檻量在四本合計，不是量在 65 一本身上**（65 §6.7.4）：
最高已配號超過 `9669` 時才需要再擴段。

---

## 1. 概述與目標

### 1.1a 先讀基本設計書

> **本書的基本設計書：[67 定期配送訂閱基本設計書](../基本設計書/67-定期配送訂閱（Subscription%20Delivery）基本設計書.md)**
> ——兩層模型（契約管授權／週期管履約）、🔴 **為什麼只有商家自送能開**、
> 精確付款授權與三個金額上限、**每期九步與兩道截止**（T-3 截止修改、T-1 截止扣款）、
> **錢與發票怎麼走**（每期各扣一次、款項進賣家、每期一張賣家發票、**沒有平台撥款**）、
> 買家可以改什麼與什麼時候不能改、失敗處置。**先讀它，本書的 §0.1 與 §7 會好讀很多。**

共用骨架另讀 [65 物流履約平台基本設計書](../基本設計書/65-物流履約平台（Fulfillment%20Platform）基本設計書.md)；
配送能力讀 [69 商家自有出貨基本設計書](../基本設計書/69-商家自有出貨（Merchant%20Self-Delivery）基本設計書.md)。
施工前置的詳設閱讀順序：**65（骨幹與 §3.9 共用規約）→ 69（本書唯一的物流方式）→
58（賣家開票、作廢／折讓分流）→ 05 支付 → 03 訂單 → 44 身分模型**。本書不重複它們已定義的任何規約。

⚠️ **基本設計書不含任何可施工的細節**（無 DDL、無端點、無述詞）⇒ **不得拿它當契約來源**。
### 1.2 設計目標

1. **每期獨立**：獨立價格、自配費、扣款、訂單、發票、折讓、退款；取消契約只影響**尚未鎖單**的未來週期。
2. **買家授權精確**：只授權「當期精確金額且不超過三個上限」；超過即停止自動扣款並請買家確認，
   **不得暗扣、不足額出貨或事後補價**（需求書 §20.1 逐字）。
3. **fail-closed**：每期重新檢查服務區、時段、溫層、庫存與整單承載；任一不成立 ⇒ `ACTION_REQUIRED`，
   **不自動改成其他物流**（需求書 §20.5 逐字，D-13）。
4. **帳務軸不外溢**：見 §2.3。

### 1.3 非目標

| # | 不做 | 落成什麼機制 |
|---|---|---|
| 1 | 跨賣家定期購物車、跨收款主體的同一 mandate | `recurring_contract.store_id` 單一；mandate 綁 `seller_merchant_scope_ref`，DB unique 防跨 seller 重用（§5.3） |
| 2 | 自動替換商品、AI 預測改數量、自動加購 | 替換必須有逐項事前同意（§5.1.12） |
| 3 | 一次預收無限期款項、儲值金、禮物訂閱、家庭共享 | `FIXED_TERM_PREPAID` 值域內但**寫入路徑由 flag 擋**（§3.1） |
| 4 | 黑貓、7-11、Uber Direct 或任何非商家自送 provider | 四層防守（§3.1） |
| 5 | 未經同意的價格上調、時段大幅位移、自動改物流 | 三個 amount cap ＋ `ACTION_REQUIRED`；**沉默不等於同意**（需求書 §20.4 逐字） |
| 6 | **`FIXED_TERM_PREPAID` 在遞延收入與未履約退款設計完成前不得開啟** | §20.15 逐字；`recurring_plan_version.price_policy` 的 publish gate 拒絕該值 |

### 1.4 與其他設計書／服務的關係

| 對象 | 關係 | 本書怎麼用 |
|---|---|---|
| **65** | 骨幹 | 65 §3.9 十一節共用規約、訂單快照六欄、normalized 配送值域、錯誤碼號段 |
| **69** | 🔴 **唯一的物流方式** | `merchant_service_zone`／`merchant_service_window`／`merchant_capacity_bucket`／`merchant_delivery_job`／POD 全部由 69 定義（69 §5.1.7 逐字「67 直接引用」），本書**零重複定義** |
| **58** | 賣家開票 | 每期發票走 58 既有鏈；跨月不可作廢只能折讓 ⇒ **接 58 的 void／allowance 分流，不另立一套**（D-2／U-3） |
| **05 payment** | mandate／charge／refund／provider query／webhook | 表與 adapter 落 payment-service（§3.7-①） |
| **03 order** | cycle 具體化的目的地 | `orders` 擴三欄（§5.1.13） |
| **39 buyer** | 既有 `seller_subscriptions`（關注店家）／`restock_subscription`（補貨通知） | 🔴 **資料不遷移**（§2.5） |
| **60** | 買家訂閱計費引擎 | payment-service 既有 `billing_provider_accounts`／`billing_commands` 是**共用底座**，本書擴用不另建（§3.7-①） |

### 1.5 已定案決策一覽（承接 65 的 D 系列）

| 編號 | 對本書的意思 |
|---|---|
| **D-7** | 🔴 帳務軸互斥 ⇒ **本書沒有帳本章、沒有撥款章**（§2.3） |
| **D-8** | 開票時點採**出貨時**；🔴 在 U-3 未確認前**不得產生會觸發開票的 cycle**（擋在建約／生成，不是「產生了但不開票」） |
| **D-10** | `cutoff` 拆 `edit_cutoff`／`charge_cutoff` ＋ 三個連帶缺口（§0.1、§3.4） |
| **D-19 群 D** | `CANCEL_AT_CUTOFF` 與 Contract `COMPLETED` 兩個 S0 ⇒ §7.3.1 逐值補推進者／推走者 |
| **D-4** | 狀態值一律 `CANCELLED`（雙 L）；provider 原生值照抄 |
| **D-5** | Contract／Cycle 兩層同名值（`ACTIVE`／`CANCELLED`／`COMPLETED`）**各自獨立 enum**，不共用值域 |
| **D-13** | fail-closed：賣家關閉自配／服務區變更／容量不足 ⇒ `ACTION_REQUIRED`，**不自動改物流** |
| **D-15** | 🔴 recurring 一律拒絕 `3001`／`3002`／`3003`／`3004`，只允許 **`3005`**（商家自送） |
| **D-16** | 金額對外 `string`；`/seller/*` 改掛 `/app/*`；ID 用 TSID；HTTP 200 ＋ 業務碼 |
| **D-17** | 每個非終態一條 aging，不是挑重點補 |
| **D-20** | maker-checker **沿用 58 號既有**（端點 54 發起／55 覆核、`chk_*_two_person`），不另建 |
| **D-1** | reconcile 四義分流（§3.7-③） |
| **D-2／U-3** | 賣家開票接 58 既有 void／allowance |

---

## 2. 範圍與情境

### 2.1 系統情境圖（C4 Context，以 `recurring-service` 為中心）

```
                    ┌──────────── Buyer (Web / App) ───────────┐
                    │  PDP 定期配送分頁 / 我的定期配送 / 任務卡 │
                    └───────────────────┬──────────────────────┘
                                        │ /app/*  (Gateway :9900)
   ┌──────── Seller (Web / App) ────────┤
   │ 方案設定 / 備貨日曆 / exception     │        ┌──── Admin (ADM) ────┐
   └────────────────────────────────────┤        │ control tower 篩選   │
                                        │        │ maker-checker(→58)  │
                                        │  /admin/*   └────────┬───────┘
                            ┌───────────▼──────────────────────▼──────┐
                            │        Gateway (Go/Gin, 唯一 HTTP 入口)  │
                            └───────────┬─────────────────────────────┘
                                        │ gRPC
                        ┌───────────────▼────────────────┐
                        │  recurring-service (Go, 新)     │  ← 本書擁有
                        │  plan / contract / cycle /      │
                        │  schedule / saga / audit        │
                        └──┬────┬────┬────┬────┬────┬─────┘
       gRPC ┌───────────────┘    │    │    │    │    └───────────────┐ MQ(outbox)
            ▼                    ▼    ▼    ▼    ▼                    ▼
   product-service       payment-service  order-service   shipping-service   notification
   (SKU/價格/庫存)       (mandate/charge)  (普通訂單)      (69 商家自送模組)   (通知)
                                │                                  │
                                ▼                                  ▼
                          PAYUNi (賣家 MerID)              賣家 ERP / 人工 SOP
                                                                   │
                                                          invoice-service (58, 賣家開票)
```

🔴 **圖上沒有 settlement-service、沒有 payout、沒有 ledger** —— 那是 §2.3 的規格，不是漏畫。
⚠️ **2026-09-12 後更強**：`settlement-service` 這個服務**已經不存在**（架構裁決 `DEC-20260912-184025`，網域併入 `payment-service`），
且 payout／ledger 所屬的 F／G 軸判定退役 ⇒ 本圖**永遠不會**需要補上它們。

### 2.2 In-scope

1. 同一賣家、同一交易／開票路徑，支援同賣家多 SKU。
2. 固定頻率：每週、每兩週、**每四週／每月**（🔴 兩者規則不同，見 §5.1.3）；固定到貨星期與賣家可用時段。
3. 固定次數 4／8／12 與**持續配送**兩種契約 —— ⚠️ 持續配送的範圍歸屬見下方已知缺口。
4. 每期扣款、跳過一期、暫停、恢復、改未來配送地址／時段、取消未來期數。
5. 每期獨立訂單、商家自行配送、賣家發票、退款與折讓。
6. 商家自行配送的後端可用性判定與**誠實 disabled reason**（前端不得只靠隱藏選項實施政策）。

🔴 **已知缺口（「持續配送」是否納入 MVP 屬**產品取捨**（它決定使用者看得到的東西變不變少），
本書不裁決，規格採 **fail-closed 預設**：

- 資料模型**兩者都支援**：`recurring_schedule_rule.cycle_count IS NULL` 即持續配送
- `recurring_plan_version` 的 publish gate 對 `cycle_count IS NULL` 的方案 🔴 **預設拒絕**，
  由 feature flag `RECURRING_OPEN_ENDED_ENABLED` 控制

⇒ 裁決結果無論哪一邊，**都不需要改 schema**，只切 flag。

### 2.3 🔴 Out-of-scope：帳務軸禁令（D-7）

**需求書 §20.15 逐字（不改寫，因為它是機械檢查的比對基準）：**

> 「定期配送資料庫**不得出現**平台買家發票、seller-to-platform invoice obligation、1%、seller net、
> negative balance 或 payout batch。」
> 「買家確認收貨只更新 receipt／after-sales 狀態，**不建立** settlement_candidate、payout_instruction 或 Grant。」
> 「PAYUNi 實際撥入賣家帳戶日只供 statement reconciliation 觀測，**平台不發起或攔截撥款**。」

定期配送走 `SELLER_DIRECT` 帳務軸——賣家自收、賣家開票——所以 65 §5A（帳本）與撥款鏈
在本書**整章不存在**。這不是省略，是規格。機械化的不建表清單見 §5.1.14。

⚠️ 對照：65 §3.6 撥款資格的第 11 條軸守門是 `commerce_model = PLATFORM_CONSIGNMENT`
⇒ **`SELLER_DIRECT` 的訂單根本不進那個判斷**（D-7 已回灌 65）。本書因此不需要「排除自己」的邏輯。

🔴 **2026-09-11 商流修正的連帶影響（本書結論不變，但理由的強度變了）**：
客戶已否決「Uber ＝ `PLATFORM_CONSIGNMENT`」的指派 ⇒ **五種履約方式全部走 `SELLER_DIRECT`**
（65 §0.6.1／§3.8b C-1）⇒ **沒有任何履約方式會進撥款軸**。
⇒ 本節的禁令**完全不變**（本來就禁），但「本書是三條商流中唯一不進撥款的那條」這個對照**已不成立**
——現在是**全部都不進**。⚠️ 這直接影響 §13.1 的對照組設計（見該節紅字）。

### 2.4 角色與身分模型（→ 65 §3.9 一）

| 角色 | 業務鍵 | 本書的固定值 |
|---|---|---|
| 買家 | **`buyer_id`**（Gateway `buyerid` resolver 解析） | — |
| 賣家 | **`store_id`**（Gateway `sellerid` resolver 解析） | 一份契約只屬於**一位**賣家 |
| 收款主體 | 賣家 PayUni 子商店 `merId = 'HAWK' \|\| store_id` | 🔴 `collection_model` **恆等** `SELLER_MERCHANT`（欄名以 65 §5.1.3 為準） |
| 發票開立人 | 賣家 | 🔴 `invoice_model` **恆等** `SELLER_TO_BUYER` |

🔴 **不得使用 `account_id` 當業務鍵**，也不得沿用需求書的 `seller_account_id`／`buyer_account_id` 舊名
（65 §3.9 一紅字）。⚠️ `merId` **僅限**付款／退款／Webhook；不得用於商品／訂單／購物車查詢。

### 2.5 與既有「訂閱店家」「補貨通知」的區別（🔴 資料不遷移）

| UI 名詞 | Domain | 既有表 | 處置 |
|---|---|---|---|
| **定期配送** | `recurring_purchase` | **不存在** | 本書全部新建 |
| 關注店家 | seller follow | `seller_subscriptions`（buyer-service，`buyer_id`＋`store_id` UNIQUE，bigserial PK） | 🔴 **完全保留**：ID／API／資料不動；只做前端文案與 route 導覽調整（需求書 §20.21 逐字） |
| 補貨通知 | restock | `restock_subscription`（buyer-service，含 `RestockSubscriptionStatus` 9501/9502/9503） | 🔴 **維持獨立**，不合併 |

🔴 **不得沿用既有 seller subscription／restock subscription 表與 API**（§20 前言逐字）。
新 route 為 `/account/recurring-deliveries`（Web）與對應 App route，**不沿用「我的訂閱」商家列表**。

---

## 3. 約束與前提

### 3.1 產品硬約束：物流方式恆定（四層防守）

🔴 `recurring_plan_version.fulfillment_mode` **恆等於 `SELLER_MANAGED`**，
且每期快照的 `shipping_method` **恆等於 `3005`**。四層同時防守（需求書 §20.5 逐字要求「DB constraint、
domain validation、API contract 與 UI 同時防守」）：

| 層 | 落成 | 拿掉它會怎樣 |
|---|---|---|
| **DB** | `chk_rpv_fulfillment_mode`／`chk_rc_fulfillment_mode`／`chk_rcs_shipping_method`／`chk_orders_recurring_origin`（§5） | 手寫 SQL 造得出違規列 |
| **Domain** | recurring-service 建約／preview／lock 三處收斂點；`3001~3004` 一律 fail closed | 新端點繞過 |
| **API** | preview／create／patch 任一路徑收到 `TAQBIN`／`SEVEN_ELEVEN_STORE_PICKUP`／`UBER_INSTANT`／`3001`～`3004` ⇒ `RECURRING_FULFILLMENT_NOT_ALLOWED`（HTTP 200 ＋ 業務碼，D-16） | 靜默 fallback |
| **UI** | 定期配送分頁只顯示唯讀資訊卡「配送方式：商家自行配送」，無 radio、無 carrier selector；DOM 與 accessibility tree 亦不得含其他選項（需求書 §20.23 逐字） | 前端隱藏 ≠ 政策 |

⚠️ **需求書用的名字不是 repo 的名字**：

| 需求書寫 | repo 實際 Name | Value |
|---|---|---|
| TCAT | `TAQBIN` | 3001 |
| UNIMART_711 | `SEVEN_ELEVEN_STORE_PICKUP` | 3002 |
| UBER_DIRECT | `UBER_INSTANT` | 3003 |
| （需求書 §21 provisional） | `SEVEN_ELEVEN_BULK_WAREHOUSE`（68 號） | 3004 |
| SELLER_MANAGED | `MERCHANT_DELIVERY`（69 號） | **3005** |

🔴 negative contract test 必須用**兩組**輸入各驗一次（需求書名 ＋ repo 名 ＋ 數字碼），
只擋其中一組等於沒擋。

### 3.2 法規與稅務約束

| 約束 | 落成 |
|---|---|
| **每期獨立開票**；無限期契約不得只在最初開一張「未來所有期數」發票 | 一 cycle 一張發票請求，發票 lines 連回 `recurring_cycle_id`＋`order_id`（§20.15） |
| 開票時點 = **出貨時**（D-8） | ⚠️ **法遵確認歸 U-3**；在 U-3 確認前**不得產生會觸發開票的 cycle**（不是「產生了但不開票」） |
| **跨月不可作廢，只能折讓** | 🔴 **接 58 號既有的 void／allowance 分流**（`invoice_documents` 的 `VOIDED` 對比 `invoice_adjustments`），**不得在 67 另立一套**（D-2 逐字） |
| 賣家自行開立並回報 | 58 既有 `InvoiceDocumentStatus.RESOLVED_EXTERNALLY`（`provider_invoice_no` 必為 NULL，平台不冒充開立人） |
| **鮮食不適用七日解除權** | 🔴 **逐商品、交易前**以清楚易懂文句告知（§20.15 逐字）；不得用一句概括排除。`recurring_contract.terms_hash` 必須涵蓋該告知文本；不影響瑕疵擔保、錯誤商品、未送達與可歸責責任 |

### 3.3 PAYUNi 約束：**兩條路徑不可混談**

| 路徑 | 誰管週期 | 適用 | 本書 MVP |
|---|---|---|---|
| **Period**（`POST /api/period/Page`／`/mdfStatus`／`/Modify`） | **provider** 管排程與續期收款 | 只可評估用於**固定金額、固定週期** | ❌ **不採為主路徑**：provider 排程與「截單前庫存／capacity 預檢」有無法消除的競態（§20.14 逐字） |
| **信用卡 Token／CreditHash**（hosted 首刷取得 → `POST /api/credit`） | **我方** 管排程 | 每期金額不同、需先預檢 | ✅ **MVP 主路徑** |

🔴 **`POST /api/period`（幕後建立）需要 `CardNo`／`CardExpired`／`CardCVC` ⇒ 本案不採此 raw-card 路徑**
（未取得 PCI DSS 範圍與法遵核准前，只能用 hosted page／正式 SDK，使前後端都不接觸 PAN／CVV）。

⚠️ **`paid-tw/skills` 的 PAYUNi plugin 不含 Period／CreditHash／mandate 實作** ⇒ 它可作一般交易底座，
**不能當定期扣款模組**（需求書 §20.25 逐字）。
⚠️ **UNKNOWN 的語意是「等待銀行結果逾期」不是失敗**：官方 60 秒未收到結果先回 UNKNOWN，
後續以 NotifyURL 通知或約 15 分鐘後查詢 ⇒ 🔴 **系統不得把 timeout 當失敗另建一筆扣款**（§20.14 逐字）。
這條直接決定 §0.3 的 R-S3 租約下界（130s）與 §5.1.5 的 `unknown_since` 欄。

⚠️ **repo 現況（實測，與需求書「後端沒有 recurring adapter」不完全一致）**：payment-service **已有**
`PayUniSubscriptionProviderAdapter`、`PayUniRecurringWebhookNormalizer`、`PayUniRecurringWebhookConsumer`
與 `V5__create_membership_billing_tables.sql` 的四張表（60 號買家訂閱計費引擎）。
⇒ 那是 **Period／provider-managed** 形狀、且 `chk_membership_provider_subscriptions_kind` 鎖死
`subscription_kind = 'BASE_MEMBERSHIP'`，**不能直接承載本書的 merchant-controlled Token 路徑**；
但它的 `billing_provider_accounts` 與 `billing_commands` 是可共用的底座（§3.7-①）。

### 3.4 🔴 時間軸約束（D-10）

#### 3.4.1 兩個 cutoff 的落點

| 落點 | 用哪一個 |
|---|---|
| `recurring_plan_version` 的賣家可設參數 | 🔴 **兩欄**（`edit_cutoff_*` 與 `charge_cutoff_*`，§5.1.1） |
| preview response | **同時回** `editCutoffAt` 與 `chargeCutoffAt`（§6.1.2） |
| 買家看到的「可改到什麼時候」 | `edit_cutoff` |
| 鎖單（`cycle:{id}:LOCK:v1`） | 錨在 `edit_cutoff` |
| 收款與重試視窗 | `charge_cutoff`（視窗 ＝ T-2 → T-1） |

#### 3.4.2 合法範圍的下界（DB CHECK ＋ publish gate 雙重）

🔴 需求書給了賣家可設 `cutoff`，**卻沒給合法範圍**。若賣家設成 T-1 或 T-0，扣款會早於鎖單
⇒ 違反 §20.6「在庫存、服務區、時段與承載均已鎖定後才扣款」的前提。落成 §0.1 的三條不等式：

```sql
-- recurring_plan_version（下界，publish 前）
CONSTRAINT chk_rpv_cutoff_order
  CHECK (edit_cutoff_hours_before >= charge_cutoff_hours_before + retry_window_hours),
CONSTRAINT chk_rpv_charge_leadtime
  CHECK (charge_cutoff_hours_before >= seller_lead_time_hours + delivery_prep_time_hours),
CONSTRAINT chk_rpv_charge_before_arrival
  CHECK (charge_cutoff_hours_before > 0)
```

⚠️ **兩層都要**：DB CHECK 守住「任何寫入路徑」，publish gate 守住「賣家改了 lead time 之後
既有草稿版本仍合法」——lead time 是**另一張表**（69 的 `merchant_delivery_profile`）的欄位，
DB CHECK 跨不了服務。

#### 3.4.3 賣家 lead time 納入時間軸

🔴 它由 §3.4.2 第二條不等式強制，且 publish gate 必須讀 69 的 profile 版本。
若 lead time > 3 天而 `edit_cutoff` 是 T-3，賣家收到確定訂單時**已來不及備貨**。
⇒ 由 §3.4.2 的第二條不等式強制，且 publish gate 必須讀 69 的 profile 版本（§6.2.1 十項驗證之一）。

#### 3.4.4 hold 必須活過 RESERVE→CHARGE 的空窗

image7 是連續 saga，而 §20.6 把 RESERVE（T-3）與 CHARGE（T-2→T-1）切成兩天
⇒ hold 租約若短於這段空窗，**扣款成功那一刻庫存已經被別人拿走**。
🔴 **這段推導只適用於「容量」hold**：**沒有庫存硬保留**（product-service 無 reserve RPC，§3.7）
⇒ 庫存的超賣風險由 MATERIALIZE 當下的 `DeductStock` 失敗承擔。定案見 §0.4 與 §3.7。

### 3.5 工程約束

全部引用 **65 §3.9**：一 identity／二 金額（元的整數、DB `BIGINT`、對外 JSON `string`、禁浮點）／
三 時間（`TIMESTAMPTZ`、UTC、對外 ISO 8601、顯示層轉 Asia/Taipei）／四 命名（`CANCELLED` 雙 L）／
五 API（三 namespace、envelope、200＋業務碼、TSID）／六 錯誤碼／七 CodeMeta／八 MQ／
九 排程器／十 新服務／十一 媒體。**本書不重述。**

### 3.6 假設

| # | 假設 | 不成立時 |
|---|---|---|
| A-1 | 69 的 `merchant_service_zone`／`_window`／`merchant_capacity_bucket` 在本書施工前已可用 | 本書的 lock 步驟無法原子保留承載 ⇒ 上線順序（§10.1）被違反 |
| A-2 | 58 的賣家開票鏈可接受「一 cycle 一張發票請求」的呼叫頻率 | 需與 58 協調 outbox 節流 |
| A-3 | product-service 的原子庫存扣減服務可供 recurring 與一般訂單**共用**（需求書 §20.16 逐字要求同一服務） | 兩套扣減 ⇒ 超賣 |
| A-4 | PayUni 允許以賣家 MerID 建立 Token mandate 且**不得跨賣家重用** | 整個收款模型重做 ⇒ U-1 |

### 3.7 由本書定版的決策

> 每條先寫推導（§3.2 優先序：① 無技術債 ② 業務邏輯完美閉環 ③ 既有 SSOT ④ 既有實作），再寫結論。

**① `recurring_payment_mandate`／`recurring_charge_attempt` 的 owner 服務**

- 需求書 §20.7 說 payment-service 擁有「mandate／Period adapter、當期 charge、refund、provider query 與
  Webhook；**不擁有排程商業規則**」；§20.14 逐字「payment-service 新增：`payment_mandate` 與
  `recurring_charge_attempt` 表」。
- 實測 payment-service 已有 `billing_provider_accounts`（provider 帳號別名＋`credential_ref`，
  明寫「raw credentials are never stored」）與 `billing_commands`（`(operation, idempotency_key)` UNIQUE
  的共用冪等邊界）。再建第二套冪等邊界 = 重複實作（repo 需求書 §15）。
- 但 `membership_provider_subscriptions` 的 `chk_membership_provider_subscriptions_kind` 鎖死
  `subscription_kind = 'BASE_MEMBERSHIP'`，且它是 provider-managed period 形狀 ⇒ **不能承載 Token mandate**。

⇒ **定案**：兩張表落 **payment-service（Java，Flyway；版號取法見 §5.0）**，名為 `recurring_payment_mandate`／
`recurring_charge_attempt`；**共用** `billing_provider_accounts`（一 seller merchant scope 一列）與
`billing_commands`（擴 `operation` CHECK ＋ 新增 nullable `result_mandate_id`）。
recurring-service **只存 `payment_mandate_ref`（BIGINT），永不存 provider reference 或卡片資訊**。
⚠️ 代價已知：`billing_commands` 同時服務兩個 domain ⇒ §5.5 必須有一條不變式斷言
「`operation` 值域與 `result_*_id` 的非空組合一一對應」。

**② `inventory` / `capacity` 的 owner 服務**

需求書寫「可先在 product／shipping DB 擴充，但契約必須清楚」＝未定。

⇒ **定案**：**庫存 hard reserve 的權威在 product-service**（需求書 §20.16 逐字要求「一般訂單與定期配送使用
**同一原子庫存扣減服務**」，而那個服務在 product-service）；**承載 capacity bucket 的權威在
shipping-service**（69 §5.1.7 的 `merchant_capacity_bucket`，該 enum 的全部落地點都是 shipping-service 的表）。
🔴 **product-service 沒有任何 reserve／hold RPC**（只有 `GetStock`／`DeductStock`／`RestoreStock`）
⇒ 上面那條「hard reserve 的權威在 product-service」**現在無法實現**。

**規格：不做庫存硬保留**（issue #3517 第 5 條）。
- `recurring_inventory_hold` 退化成**本地的意圖記錄**（`external_hold_ref` 改 nullable，
  只有在日後 product 提供預留 RPC 時才填）
- 扣減仍在 **MATERIALIZE 那一刻**由 `DeductStock` 完成 ⇒ **RESERVE→CHARGE 空窗期間有超賣風險**，
  由該次 `DeductStock` 失敗承擔（此時已扣款 ⇒ 走 ACTION_REQUIRED ＋ 退款鏈）
- ⚠️ **INV-10 與 §3.4.4「hold 必須活過空窗」只對容量 hold 成立**，庫存側不適用
- ⚠️ §5.5 B3「本地釋放一半、對方仍鎖著」的情境**在庫存側不會發生**（對方本來就沒鎖）

承載 capacity bucket 的權威仍在 shipping-service，原子操作以 gRPC 在對方交易內完成。
🔴 **跨服務不能同交易**（本 repo 九軸分屬 5 個服務、5 個資料庫）⇒ 兩個 hold 的一致性靠
「保留成功才寫本地列、本地寫入失敗即補償釋放」＋ 到期自動釋放兜底。

**③ `seller_payment_reconciliation` 的 owner 與 D-1 義②歸屬**

該表**只在 §20.15 出現、不在 §20.10 表清單**，owner 未定（65 D-8 已登記）。依 D-1 四義拆解後
發現它其實是**三件事被一個名字蓋住**：

| 拆出 | D-1 義 | 處置 |
|---|---|---|
| (a) 每期的**勾稽事實快照**（cycle／order／seller merchant trade no／captured／refunded／發票／delivery status） | 不是 reconcile，是**事實** | ✅ 建表 `recurring_cycle_settlement_fact`，由 cycle saga 在**同一本地交易**寫入／更新 |
| (b) **差異案件** | **義③（案件）** | ✅ 建表 `recurring_reconciliation_case`，**事件驅動建立**（退款／發票／statement 匯入時），ADM 處置 |
| (c) **自動比對排程**（掃描 statement 對帳） | **義②（對帳）** | 🔴 **不進 application code**（《後端服務責任邊界規範》＋ D-1 逐字：「差異的**呈現**可以做，**自動比對排程**不行」）⇒ 走維運 SQL ＋ ADM 查詢畫面 |

⇒ **定案**：(a)(b) 兩表 owner = **recurring-service**（Go／Atlas）；(c) **不實作**。
⚠️ 需求書的名字 `seller_payment_reconciliation` 對應的是 (a)，§14.2 的命名對照表要收錄。

**④ `delivery_prep_time_hours` 預設值** = **12 小時**。推導：`charge_cutoff` 預設 T-1（24h），
扣除賣家 lead time 後仍須留出「建單 → 通知商家 → 商家 ACCEPTED」的窗口；69 的 outbound 通知
有重試與退避，12h 是「賣家隔天上班才看到」的最壞情況。賣家可覆寫，受 §3.4.2 不等式約束。

**⑤ 政策容忍時間** = **6 小時**。推導：小於 `retry_window`（24h）的四分之一，
確保「漏跑」與「正常重試」在時間上可分辨；超過即轉 incident 不 catch-up 扣款（§20.13 逐字）。

**⑥ 通知是同步 RPC 還是訂事件** —— 需求書 image6 的 Notification **未連 Event Bus**，而 §20.6 有大量通知動作。

⇒ **定案**：**訂事件**。理由：① 同步 RPC 會把 notification-service 變成扣款 saga 的硬依賴
（一次通知失敗回滾一次扣款＝業務上錯的）② 65 §九「向外投遞一律不得 level-triggered」而
outbox＋lease 是既有正解（58 號 `INV.ob` 先例）③ notification-service 是既有 Go 服務、已是事件消費者。
⇒ recurring-service **只寫 outbox**，`recurring_outbox_events` 的 dispatcher 投遞到 MQ，
notification-service 訂閱。⚠️ 連帶要求：每一則通知事件都要有**冪等鍵**，否則重放＝多吵買家一次（D-1 義④）。

**⑦ `recurring-service` 的 Stack／Port／登記** → §4.3。

**⑧「持續配送」的 MVP 歸屬** → 🔴 **本書不定版**，見 §2.2 已知缺口，移交裁決。

## 4. 架構

### 4.1 解法總綱：兩層 ＋ 每期一條 saga

```
Contract（長期名額）                       Cycle（單期實物，每期一條 saga）
────────────────────                       ──────────────────────────────────
PENDING_MANDATE                            SCHEDULED
   │ mandate 啟用                                │ T-3 edit_cutoff（R-S2）
   ▼                                             ▼
ACTIVE ◄──► PAUSED                         LOCKED ──R-S3──► CHARGING ──成功──► PAID
   │  └─► ACTION_REQUIRED                     │                 │               │ R-S4
   │                                          │  🔴 無 LOCKED→SKIPPED
   │                                          │  （§7.3.2：SKIPPED 只從 SCHEDULED 來）
   │                                          │                 ▼               ▼
   ├─► CANCEL_AT_CUTOFF ─► CANCELLED          ├─► CANCELLED  ACTION_REQUIRED  MATERIALIZING
   └─► COMPLETED（R-S5：completed_cycles      └─► ACTION_REQUIRED   │            │
        >= total_cycles）                          （RB-14/15 回 LOCKED／PAID）  ▼
                                                                            MATERIALIZED
                                                                                 │
                                                                            COMPLETED
```

三個結構性選擇（每一個都是為了避開一類 bug）：

1. **兩層而非一層**：Contract 只描述授權與排程，Cycle 只描述排程協調
   ⇒ 「不要把付款失敗、配送失敗或退款狀態塞進 contract status」（需求書 §20.9 逐字）。
2. **每期具體化成普通訂單**，不建平行訂單系統 ⇒ 售後、退款、發票、物流全部沿用既有權威。
3. **每期是一條 saga 而非一次交易**：跨服務不能同交易 ⇒ 每一步都要能單獨重放且冪等（§4.6）。

### 4.2 元件視圖與七服務責任邊界

| 服務 | 責任 | 🔴 禁止 |
|---|---|---|
| **recurring-service**（Go，**新**） | 方案版本、契約、週期、排程、買家授權快照、cycle saga state、操作稽核、`next_action_at` | 複製可變商品主檔；擁有付款／訂單／配送／發票的真實狀態；建立任何撥款或平台帳本 |
| **product-service** | SKU 權威、上下架、價格、重量／尺寸／溫層、可訂閱資格、**原子庫存扣減與 hard reservation** | 讓 recurring 走第二套庫存扣減 |
| **shipping-service（69 模組）** | 商家服務區／時段／承載 bucket、自配費、manual／API delivery job、POD、退回、簽章 merchant webhook | 🔴 在 recurring scope 接黑貓／7-11／Uber adapter |
| **order-service** | 把已鎖定且已付款的週期具體化為普通訂單，**接受唯一 `recurring_cycle_id`** | 用 `recurring_contract_id` 決定當期價格或物流 |
| **payment-service** | mandate、當期 charge、refund、provider query、Webhook | 🔴 擁有排程商業規則；把 HTTP 200 當成最終 captured |
| **invoice-service（58）** | 每期賣家對買家發票、作廢／折讓分流 | 以 contract total 當一次收入 |
| **notification-service** | 訂閱 recurring 事件寄送通知 | 被放進扣款 saga 的同步路徑（§3.7-⑥） |

### 4.3 部署拓撲與 Port（→ 65 §3.9 十）

| 項目 | 決定 | 依據 |
|---|---|---|
| **服務名** | `recurring-service` | 實測 `ls services/` 無此目錄 ⇒ **全新** |
| **Stack** | **Go** | 65 §3.9 十已定案（2 個新服務皆 Go） |
| **TSID Node ID** | 🔴 **不需要**（Go 用 `pkg/tsid.NewGenerator`，見 `pkg/tsid/tsid.go` 的 `NewGenerator`） | 65 §3.9 十／repo §7.3 |
| **HTTP Port** | **`10043`** | 形狀比照 invoice-service（`HTTP_PORT` ＋ `GRPC_PORT` 相鄰兩號），寫進 `k8s/values/recurring-service.yaml` |
| **gRPC Port** | **`10044`** | 同上 |
| **port 取號** | 🔴 **施工當下重跑一次盤點再落值**：`grep -rhoE '100[0-9][0-9]' k8s/values/ \| sort -u` —— 本表的號碼是撰寫當下的可用值，其他設計書可能同期佔用 |
| **DB** | 獨立資料庫（Atlas 宣告式 schema），`services/recurring-service/db/schema/schema.sql` | repo §11.2 |
| **migration** | **Atlas**（改 schema → `atlas migrate diff` → `atlas migrate hash`），**不是 Flyway** | repo §11.2 |

🔴 **六類登記缺一即「CI 全綠而服務從未部署」**（65 附錄 G.8；完整定義以
`docs/開發規範/新增微服務指南.md` §0 為準）：

| # | 項目 | 本書的具體條目 |
|---|---|---|
| ① | CI workflow 手寫服務清單 | `go-checks.yml` module 清單、`code-quality.yml` 的 `for svc in` |
| ② | NetworkPolicy | 自己的 `recurring-service-ingress`（放行 gateway → 10044）＋ **上游 caller 白名單**（product／payment／order／shipping／invoice／notification 各自的 ingress 要放行 recurring-service）。⚠️ label key 是 `app.kubernetes.io/name`；🔴 **fail-open 的呼叫端漏登記完全無聲**（repo 65 §6.1.5） |
| ③ | `scripts/init-databases.sh` 的 `DATABASES` 陣列 ＋ README 對照表 | 加 `recurring` |
| ④ | `services/recurring-service/db/atlas.hcl` ＋ `migrations/` **必須真的產生過一次** | 參照 `services/invoice-service/db/atlas.hcl` |
| ⑤ | 監控 | 告警 group／`ProdServiceAbsent`／`sql_exporter` job 與 collector／兩份 compose DSN／`.env.example` |
| ⑥ | 硬上限依賴登記表 | PAYUNi 呼叫量、DB partition（§9.3） |

⚠️ **build 集鍵＝`services/recurring-service/Dockerfile`、deploy 集鍵＝`k8s/values/recurring-service.yaml`，
兩個是獨立交付項**，由 `test-content-addressed-tag.sh` 性質 8 雙向守住（65 §3.9 十）。
⚠️ **prod `replicas` 與 `strategy` 尚無 values 檔**（65 SO-1）⇒ §0.3 的租約論證在 values 落地前
無法宣稱已驗證；**R-S2/R-S3/R-S4 一律按多副本設計**（lease），不得靠 `replicas:1` 當保證。

### 4.4 命名／契約規約與**命名漂移統一表**（需求書自帶三組）

| 需求書用名 A | 需求書用名 B | 🔴 本書定名 | 落在哪 |
|---|---|---|---|
| `payment_mandate`（§20.10／§20.14） | `recurring_payment_mandate`（§20.10 表清單） | **`recurring_payment_mandate`** | payment-service |
| `materialized_order`（§20.3 domain 名詞） | `recurring_cycle_order`（§20.10 表清單） | **表名 `recurring_cycle_order`；動作名「具體化 materialize」** | recurring-service |
| image7 步驟名（RESERVE／AUTHORIZE／CHARGE／MATERIALIZE） | image8 狀態值（`LOCKED`／`MATERIALIZING`…） | **步驟 ≠ 狀態**：映射表在 §7.2.2 | — |
| `seller_payment_reconciliation`（§20.15） | 不在 §20.10 表清單 | **拆三**：`recurring_cycle_settlement_fact`（事實）／`recurring_reconciliation_case`（案件）／自動比對**不實作** | §3.7-③ |
| `cutoff`（一個名詞兩件事） | — | **`edit_cutoff_at`／`charge_cutoff_at`** | D-10 |
| `provider = MERCHANT`（§20.10） | `fulfillment_mode = SELLER_MANAGED` | **`shipping_method = '3005'`**（65 已裁決 provider 維度由 `orders.shipping_method` 承載，再開 `provider` 欄＝製造第二個真相） | §5.1.13 |
| TCAT／UNIMART_711／UBER_DIRECT | — | `TAQBIN`／`SEVEN_ELEVEN_STORE_PICKUP`／`UBER_INSTANT`（§3.1） | CodeMeta 實測 |

其餘一律引 65 §3.9 四（`CANCELLED` 雙 L、provider 原生值照抄、同名狀態跨軸各自獨立 enum、
URL `snake_case`／JSON `camelCase`）。

### 4.5 PAYUNi 續期收款整合（→ payment-service）

四個介面（§20.14 逐字要求；正式欄位待 U-R3）：

| interface | 方法 | 本書的約束 |
|---|---|---|
| `RecurringMandateProvider` | `create`／`query`／`pause`／`resume`／`terminate` | 首次授權**只走 PAYUNi hosted page 或正式 SDK**；前端／App 永不接觸 PAN／CVV |
| `RecurringChargeProvider` | `authorize`／`capture`／`void`／`query`／`refund` | 🔴 即使 PAYUNi 實際是單步請款，adapter 仍要把 provider 真實結果映射成明確狀態，**不得把 HTTP 200 等同最終 captured** |

每期扣款的固定形狀（§20.14 逐字）：**seller MerID ＋ 新的唯一 `MerTradeNo` ＋ 本期精確 `TradeAmt`
＋ seller-scoped CreditHash**；🔴 **Client 不得傳 `MerID`、金額、Token、invoice issuer、mode／provider
或 recurrence parameters**——全部由伺服器依 cycle snapshot 算出。

失敗策略（§20.14 逐條，落成 §5.1.5 的欄位）：

| provider 結果 | 處置 |
|---|---|
| soft decline | `charge_cutoff` 前最多重試 2 次，間隔 ≥6h，每次通知 |
| authentication required／mandate expired | Contract → `ACTION_REQUIRED`，要求買家回 hosted flow 重新授權 |
| hard decline／revoked | 停止當期與未來自動扣款；**不出貨、不自動換付款方式** |
| **timeout／unknown** | 🔴 **只 query，不以新 `MerTradeNo` 重扣**；超過 SLA 進 reconciliation case |
| chargeback | 標記爭議、依風險政策暫停未來扣款；以 seller merchant statement 對帳，**不自動退所有歷史週期**。平台**沒有可 hold 的 seller payout** |

### 4.6 🔴 跨服務寫入一律 transactional outbox／inbox

「所有跨服務寫入透過 transactional outbox／inbox；**不可用同步鏈式 RPC 假裝成原子交易**」（需求書 §20.7 逐字）。

- **本地交易內**寫業務列 ＋ `recurring_outbox_events`（例外 A，65 §九）；dispatcher 以 lease 投遞。
- **入站**一律先落 `recurring_inbox_events` 去重再處理（PayUni notify、69 的配送事件、58 的發票事件）。
- 🔴 **hold／charge／materialize 三步不得合併成一個「RPC 鏈」**：每一步都要能單獨重放，
  且重放的判定材料是 DB unique（§5.3），不是「上一步回了 200」。

### 4.7 靜默陷阱表（本章）

| # | 陷阱 | 為什麼不會被發現 | 防線 |
|---|---|---|---|
| A1 | `recurring-service` 的 values 檔沒建 | image build 成功、CI 全綠、**服務從未部署** | 六類登記 ④⑥ ＋ `test-content-addressed-tag.sh` 性質 8 |
| A2 | 上游服務的 NetworkPolicy 沒放行 recurring | 呼叫端 fail-open ⇒ 檢查**從部署那一刻起從未執行過**，log 只有一行 `UNAVAILABLE`（repo 65 §6.1.5 實例） | 造一個「應該被拒絕」的情境驗它真的被拒絕，不是「跑一次看有沒有壞」 |
| A3 | 通知走同步 RPC | 通知失敗回滾扣款；或反之扣款成功但通知丟失，兩者都不報錯 | §3.7-⑥ outbox |
| A4 | `billing_commands.operation` CHECK 沒擴 | 新 operation 寫入 23514，但只在 recurring 首次上線時才炸 | 與 §5.1.4／§5.1.5 同一支 migration 內 ALTER ＋ 契約測試 |
| A5 | R-S3 租約設成 120s（沿用其他排程器） | 平時不觸發；只有在 PAYUNi 回應接近 60s UNKNOWN 門檻時被搶佔 ⇒ **重複扣款** | §0.3 下界 130s ＋ 測試注入 130s 延遲 |
| A6 | `edit_cutoff`／`charge_cutoff` 在某處被寫回同一個值 | 兩者相等時系統仍能跑，只是重試視窗為 0 ⇒ 扣款失敗率悄悄上升 | §3.4.2 DB CHECK ＋ §8.2 指標 |

---

## 5. 資料設計

> 本章回答三件事，逐表逐欄：① **誰寫入**（權威來源）② **誰讀它**（漏一個 consumer 就是靜默失效）
> ③ **誰把它推走／誰刪除**（🔴 答不出來就不要建）。
> 金額一律 `BIGINT`（元的整數）、時間一律 `TIMESTAMPTZ`（UTC）、主鍵一律 **TSID**（`BIGINT`，
> Go `pkg/tsid.NewGenerator`）；對外 JSON 一律 `string`（65 §3.9 二）。

### 5.0 表清單：服務歸屬、誰寫／誰讀／誰推走

| # | 表 | 服務 | 新／擴 | migration | 誰寫入 | 誰讀 | 🔴 誰推走／刪除 |
|---|---|---|---|---|---|---|---|
| R1 | `recurring_plan` | recurring | 新 | Atlas | 賣家（S 端點） | 賣家／買家 PDP | 賣家 `retire`（軟性，狀態） |
| R2 | `recurring_plan_version` | recurring | 新 | Atlas | 賣家 | 契約建立時快照 | 🔴 **published 後永不刪、永不改**；只能發新版本 |
| R3 | `recurring_plan_item` | recurring | 新 | Atlas | 賣家（隨 version） | preview／建約 | 同 R2 |
| R4 | `recurring_contract` | recurring | 新 | Atlas | 買家建約／saga | 買家／賣家／ADM | R-S5（`COMPLETED`）／買家 cancel／ADM terminate（maker-checker） |
| R5 | `recurring_contract_item` | recurring | 新 | Atlas | 買家 PATCH | cycle lock 時快照 | 契約終態後凍結；**不物理刪除** |
| R6 | `recurring_schedule_rule` | recurring | 新 | Atlas | 買家建約／reschedule | R-S1 | 同 R4 |
| R7 | `recurring_cycle` | recurring | 新 | Atlas | R-S1 生成 | 全部 | R-S2／R-S3／R-S4／R-S6 推進；終態後不刪（月分割 detach） |
| R8 | `recurring_cycle_snapshot` | recurring | 新 | Atlas | R-S2（lock 當下） | R-S3／R-S4／發票／對帳 | 🔴 **append-only，禁 UPDATE、禁物理刪除** |
| R9 | `recurring_inventory_hold` | recurring | 新 | Atlas | R-S2 | R-S3／R-S4 | R-S4 commit／取消路徑／**到期釋放**（`expires_at`） |
| R10 | `recurring_capacity_hold` | recurring | 新 | Atlas | R-S2 | R-S3／R-S4 | 同 R9 |
| R11 | `recurring_cycle_order` | recurring | 新 | Atlas | R-S4 | 對帳／ADM | 🔴 **永不刪**（它就是防重複建單的那道 unique） |
| R12 | `recurring_preview` | recurring | 新 | Atlas | preview 端點 | 建約端點驗 hash | **`expires_at` 到期由維運 SQL 清理**（無業務值） |
| R13 | `recurring_substitution_consent` | recurring | 新 | Atlas | 買家逐項同意 | R-S2 缺貨決策 | 買家撤回（狀態）；**不物理刪除**（同意證據） |
| R14 | `recurring_cycle_settlement_fact` | recurring | 新 | Atlas | cycle saga（同本地交易） | ADM 對帳畫面 | 🔴 **不推走**（事實表）；retention 到期遮罩 |
| R15 | `recurring_reconciliation_case` | recurring | 新 | Atlas | 事件驅動建立（義③） | ADM | **ADM 關案** |
| R16 | `recurring_action_audit` | recurring | 新 | Atlas | 所有 mutation | ADM export | 🔴 **append-only，禁刪** |
| R17 | `recurring_outbox_events` | recurring | 新 | Atlas | 業務交易同 tx | dispatcher | dispatcher 標 `done`；🔴 **耗盡重試走「停泊」不寫 `failed`**（見 §5.1.15） |
| R18 | `recurring_inbox_events` | recurring | 新 | Atlas | consumer | consumer | 標 `PROCESSED`／`SKIPPED`；**本表永不刪列** |
| R19 | `recurring_admin_proposal` | recurring | 新 | Atlas | RA-4／RA-6（maker） | RA-5／RA-7（checker）／ADM | RA-5／RA-7 覆核：`PENDING`→`APPROVED`／`REJECTED`；🔴 **不物理刪除**（治理留痕）。見 §5.1.16 |
| R20 | `recurring_mandate_consent` | recurring | 新 | Atlas | RB-4 建約 saga（同 tx） | 爭議舉證／ADM | 🔴 **不推走、不刪除**（法遵資產）；變更收款主體或任一 cap ⇒ **新增一列**、舊列轉 `SUPERSEDED`，**不得就地 UPDATE**（§8.7）。見 §5.1.16 |
| P1 | `recurring_payment_mandate` | **payment**（Java） | 新 | **Flyway**（取號見表下註） | hosted flow 回調 | charge worker | 買家撤銷／provider revoke／ADM disable（maker-checker） |
| P2 | `recurring_charge_attempt` | **payment**（Java） | 新 | **Flyway**（同 P1 那一支） | charge worker／webhook | recurring saga | 🔴 **append-only** |
| P3 | `billing_provider_accounts`／`billing_commands` | payment | **擴** | Flyway（同 P1 那一支） | 同上 | 同上 | 既有 |
| O1 | `orders`（＋3 欄＋1 CHECK） | **order**（Java） | **擴** | **Flyway**（取號見表下註） | R-S4 建單 | 全部 | 既有訂單生命週期 |

🔴 **Flyway 版號一律在施工當下取，設計書不寫死**。取號方式：

```bash
ls services/java/{order,payment}-service/src/main/resources/db/migration/ | sort -t_ -k1.2 -n | tail -1
```

`ls` 是字典序，`V9` 會排在 `V10` 之後 ⇒ **必須用上面的數值排序**，否則會撞號，而撞號的症狀是
**Flyway checksum 衝突、該服務起不來**。取號後：**禁跳號、禁改已套用的 migration**；同一波若有
多本設計書一起施工，以施工當下的實際目錄為準，不以任何一本設計書記載的數字為準。
⚠️ P1／P2／P3 三列**必須落在同一支 migration**（`billing_commands` 的 CHECK 擴充與兩張新表
是同一個 expand 單元）；O1 的版號另有順序依賴，見 §5.1.13。

### 5.1 資料模型

#### 5.1.1 `recurring_plan` ／ `_version` ／ `_item`

```sql
CREATE TABLE recurring_plan (
  id          bigint PRIMARY KEY,                 -- TSID
  store_id    bigint      NOT NULL,               -- 賣家業務鍵（65 §3.9 一），禁 account_id
  code        varchar(64) NOT NULL,               -- 賣家自訂識別
  status      varchar(16) NOT NULL DEFAULT 'DRAFT',
  created_at  timestamptz NOT NULL DEFAULT NOW(),
  updated_at  timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_rp_store_code   UNIQUE (store_id, code),
  CONSTRAINT chk_rp_status      CHECK (status IN ('DRAFT','ACTIVE','PAUSED','RETIRED'))
);

CREATE TABLE recurring_plan_version (
  id                        bigint PRIMARY KEY,
  plan_id                   bigint      NOT NULL REFERENCES recurring_plan(id),
  version_no                integer     NOT NULL,
  price_policy              varchar(32) NOT NULL,          -- RecurringPricePolicy
  -- 🔴 三個 policy 欄位恆定（§3.1）；🔴 欄名以 65 §5.1.3 的 orders DDL 為準（collection_model，不是 collection_mode）
  --    值域 SSOT 見 65 附錄 C.2；本書不另建 enum
  fulfillment_mode          varchar(24) NOT NULL DEFAULT 'SELLER_MANAGED',
  collection_model          varchar(32) NOT NULL DEFAULT 'SELLER_MERCHANT',
  invoice_model             varchar(24) NOT NULL DEFAULT 'SELLER_TO_BUYER',
  -- 🔴 D-10：cutoff 拆兩欄，單位小時、相對於 scheduled_for
  edit_cutoff_hours_before   integer    NOT NULL,
  charge_cutoff_hours_before integer    NOT NULL,
  retry_window_hours         integer    NOT NULL DEFAULT 24,
  seller_lead_time_hours     integer    NOT NULL,          -- 🔴 本波新增；來源 69 profile，publish 時快照
  delivery_prep_time_hours   integer    NOT NULL DEFAULT 12,
  allowed_frequencies       jsonb       NOT NULL,          -- ["WEEKLY","BIWEEKLY","EVERY_4_WEEKS","MONTHLY"]
  allowed_cycle_counts      jsonb       NOT NULL,          -- [4,8,12]；null 元素代表持續配送（受 flag 擋，§2.2）
  cancel_policy_text        text        NOT NULL,
  fresh_food_notice_text    text,                          -- 🔴 §20.15 逐商品七日解除權例外告知
  delivery_subsidy_rule     jsonb,                         -- 負擔者/適用方式/門檻/上限/price component（需求書 §20.4）
  terms_hash                char(64)    NOT NULL,
  published_at              timestamptz,
  created_at                timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_rpv_plan_version UNIQUE (plan_id, version_no),
  CONSTRAINT chk_rpv_price_policy CHECK (price_policy IN
      ('FIXED_PER_CYCLE','CURRENT_PRICE_MINUS_DISCOUNT','FIXED_TERM_PREPAID')),
  CONSTRAINT chk_rpv_fulfillment_mode CHECK (fulfillment_mode = 'SELLER_MANAGED'),
  CONSTRAINT chk_rpv_collection_model CHECK (collection_model = 'SELLER_MERCHANT'),
  CONSTRAINT chk_rpv_invoice_model    CHECK (invoice_model    = 'SELLER_TO_BUYER'),
  CONSTRAINT chk_rpv_terms_hash       CHECK (terms_hash ~ '^[0-9a-f]{64}$'),
  -- §3.4.2 三條不等式
  CONSTRAINT chk_rpv_cutoff_order     CHECK (edit_cutoff_hours_before >= charge_cutoff_hours_before + retry_window_hours),
  CONSTRAINT chk_rpv_charge_leadtime  CHECK (charge_cutoff_hours_before >= seller_lead_time_hours + delivery_prep_time_hours),
  CONSTRAINT chk_rpv_charge_positive  CHECK (charge_cutoff_hours_before > 0)
);

CREATE TABLE recurring_plan_item (
  id                     bigint PRIMARY KEY,
  plan_version_id        bigint  NOT NULL REFERENCES recurring_plan_version(id),
  sku_id                 bigint  NOT NULL,
  min_qty                integer NOT NULL,
  max_qty                integer NOT NULL,
  subscription_unit_price bigint NOT NULL,        -- 🔴 元的整數，非 float
  CONSTRAINT uq_rpi_version_sku UNIQUE (plan_version_id, sku_id),
  CONSTRAINT chk_rpi_qty   CHECK (min_qty >= 1 AND max_qty >= min_qty),
  CONSTRAINT chk_rpi_price CHECK (subscription_unit_price >= 0)
);
```

🔴 **published 後不可改**：`published_at IS NOT NULL` 的列由 domain 層拒絕 UPDATE，且
`recurring_contract` 只能指向 `published_at IS NOT NULL` 的版本 ⇒ 「活躍契約永遠指向不可變 plan version；
賣家改方案不回寫舊契約」（需求書 §20.23 驗收）。
⚠️ **SKU 需同賣家**：跨服務無 FK ⇒ 由 publish gate 呼叫 product-service 驗證並記錄驗證時間，
DB 守不住這條（§5.5 INV-6 記為「應用層不變式」而非 DB 保證，不假裝有）。

#### 5.1.2 `recurring_contract` ／ `recurring_contract_item`

```sql
CREATE TABLE recurring_contract (
  id                       bigint PRIMARY KEY,
  buyer_id                 bigint      NOT NULL,      -- 買家業務鍵
  store_id                 bigint      NOT NULL,      -- 賣家業務鍵；🔴 一份契約只屬於一位賣家
  plan_version_id          bigint      NOT NULL REFERENCES recurring_plan_version(id),
  collection_model         varchar(32) NOT NULL DEFAULT 'SELLER_MERCHANT',
  invoice_model            varchar(24) NOT NULL DEFAULT 'SELLER_TO_BUYER',
  fulfillment_mode         varchar(24) NOT NULL DEFAULT 'SELLER_MANAGED',
  seller_merchant_scope_ref varchar(128) NOT NULL,    -- = billing_provider_accounts.account_ref
  payment_mandate_ref      bigint,                    -- payment-service recurring_payment_mandate.id；跨服務無 FK
  status                   varchar(24) NOT NULL DEFAULT 'PENDING_MANDATE',
  paused_until             timestamptz,               -- PAUSE 到期。🔴 **不是任何排程器的述詞來源**
                                              --   （PAUSE 到期不自動恢復，RB-9 純手動）；只供 UI 顯示
  cancel_effective_at      timestamptz,               -- CANCEL_AT_CUTOFF 的生效點（D-19 群 D）
  next_action_at           timestamptz,
  terms_hash               char(64)    NOT NULL,
  consent_evidence_id      bigint      NOT NULL,      -- §8.7 同意證據
  version                  integer     NOT NULL DEFAULT 0,   -- optimistic lock
  created_at               timestamptz NOT NULL DEFAULT NOW(),
  -- ↓ §7.0 排程器候選述詞會讀的欄位（🔴 漏建 ⇒ 排程器的 SQL 42703）
  horizon_filled_until     timestamptz,       -- R-S1：週期生成的水位
  total_cycles             integer,           -- NULL＝無限期
  completed_cycles         integer     NOT NULL DEFAULT 0,
                                              -- R-S5：COMPLETED 的唯一推進者靠這兩欄比較
  mandate_expires_at       timestamptz,       -- 🔴 payment 側 mandate 到期日的**本地快照**
                                              --   寫入＝RB-4 建約 saga（CreateRecurringMandate 回傳 expires_at，同 tx 落）
                                              --   刷新＝RB-18 換卡端點（同一次同步 RPC 內連同 payment_mandate_ref 一起換）
                                              --   🔴 **沒有第三條路徑**：recurring-service 無 webhook ingress
                                              --   （PayUni webhook 落點是 payuni-webhook-service，且無 payment→recurring 事件）
                                              --   ⇒ 發卡行自動延期（account updater）本波**不覆蓋**，後果見 §7.0 裁決
  expiring_notified_for    timestamptz,       -- R-S7 去重欄。🔴 **不是「發過沒」而是「為哪一個到期日發過」**
                                              --   換卡後 mandate_expires_at 變值 ⇒ 本欄自動不相等 ⇒ 重新進候選。
                                              --   若改成「發過沒」＋ IS NULL 述詞會永久封死（換卡後永不再通知）
  status_entered_at        timestamptz NOT NULL DEFAULT NOW(),
                                              -- 🔴 contract_paused_overdue 告警的起算點
                                              --   （無限期暫停時 paused_until 為 NULL，不能拿它起算）
  updated_at               timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rc_collection_model CHECK (collection_model = 'SELLER_MERCHANT'),
  CONSTRAINT chk_rc_invoice_model    CHECK (invoice_model    = 'SELLER_TO_BUYER'),
  CONSTRAINT chk_rc_fulfillment_mode CHECK (fulfillment_mode = 'SELLER_MANAGED'),
  CONSTRAINT chk_rc_status CHECK (status IN
      ('PENDING_MANDATE','ACTIVE','PAUSED','ACTION_REQUIRED','CANCEL_AT_CUTOFF','CANCELLED','COMPLETED')),
  -- 🔴 D-19 群 D：CANCEL_AT_CUTOFF 必須有生效點，否則它是沒有推走者的死狀態
  CONSTRAINT chk_rc_cancel_at_cutoff CHECK (status <> 'CANCEL_AT_CUTOFF' OR cancel_effective_at IS NOT NULL),
  -- 🔴 不得寫成 (status <> 'PAUSED' OR paused_until IS NOT NULL)：它與 §6.1 RB-8
  --    「resumeAt 選填、不填＝無限期」直接互斥 ⇒ 無限期暫停必定 23514，暫停功能整支寫不進去。
  --    PAUSED 必須允許 paused_until IS NULL（＝無限期，須手動恢復）。
  --    ⚠️ 連帶：§7.3.1 的 contract_paused_overdue 告警不能用 paused_until 起算，
  --    改用 status_entered_at（無限期暫停超過 90 天仍要有人看見）。
  -- 🔴 不得寫成 (status = 'PENDING_MANDATE' OR payment_mandate_ref IS NOT NULL)：
  --    它會擋掉「72h 未綁卡 → CANCELLED」這條出邊（此時 mandate_ref 仍為 NULL ⇒ 兩邊皆 false ⇒ 23514）
  --    ⇒ PENDING_MANDATE 既進不去也出不來。只對「已生效」的狀態要求 mandate：
  CONSTRAINT chk_rc_mandate_required CHECK (
      status NOT IN ('ACTIVE','PAUSED','ACTION_REQUIRED') OR payment_mandate_ref IS NOT NULL)
);

CREATE TABLE recurring_contract_item (
  id                   bigint PRIMARY KEY,
  contract_id          bigint  NOT NULL REFERENCES recurring_contract(id),
  sku_id               bigint  NOT NULL,
  qty                  integer NOT NULL,
  item_policy_snapshot jsonb   NOT NULL,          -- 單價/折扣/替代品政策的建約當下快照
  effective_from_seq   integer NOT NULL DEFAULT 1,-- 🔴 只影響「未鎖單未來期」：改品項＝新增一列而非 UPDATE
  effective_to_seq     integer,                   -- NULL = 迄今有效
  created_at           timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rci_qty CHECK (qty >= 1),
  CONSTRAINT chk_rci_seq CHECK (effective_to_seq IS NULL OR effective_to_seq >= effective_from_seq)
);
CREATE UNIQUE INDEX uq_rci_active_sku ON recurring_contract_item (contract_id, sku_id)
  WHERE effective_to_seq IS NULL;
```

⚠️ **`paused_until` 不是任何排程器的候選述詞來源**——PAUSE 到期恢復排程器 §7.0 已裁決**不建**（RB-9：純手動恢復）。該欄只供 UI 顯示；PAUSE 的出口是手動 RB-9，逾時偵測靠 `contract_paused_overdue`（錨在 `status_entered_at`，不是 `paused_until`——無限期暫停時它是 NULL）。原註：
沒有這一欄，PAUSE 就是「進得去出不來」的永久旗標（65 §九判準逐字）。
⚠️ **`cancel_effective_at` 同理**——D-19 群 D 判定 `CANCEL_AT_CUTOFF` 推進者與推走者都不存在，
本書的處置是**實作**（不是刪除）：推進者＝買家 cancel 且當期已鎖單；推走者＝R-S6 在
`cancel_effective_at` 到達後轉 `CANCELLED`。

#### 5.1.3 `recurring_schedule_rule`（🔴 補 `day_of_month`）

需求書把「每四週／每月」寫成**同一個選項**，但兩者規則不同：「每四週」錨在星期（28 天固定），
「每月」錨在**日期**（月長不一）⇒ 而 §20.10 的欄位清單只有 `weekday`，**anchor 無處可存**。

```sql
CREATE TABLE recurring_schedule_rule (
  id            bigint PRIMARY KEY,
  contract_id   bigint      NOT NULL REFERENCES recurring_contract(id),
  frequency     varchar(16) NOT NULL,      -- WEEKLY / BIWEEKLY / EVERY_4_WEEKS / MONTHLY
  interval_n    smallint    NOT NULL DEFAULT 1,
  weekday       varchar(12),               -- WEEKLY/BIWEEKLY/EVERY_4_WEEKS 必填
  day_of_month  smallint,                  -- 🔴 本書新增：MONTHLY 必填（1..31）
  local_time    time        NOT NULL,      -- 到貨時段起點（本地）
  timezone      varchar(48) NOT NULL DEFAULT 'Asia/Taipei',
  start_date    date        NOT NULL,
  end_date      date,
  cycle_count   integer,                   -- NULL = 持續配送（受 flag 擋，§2.2）
  service_window_id bigint  NOT NULL,      -- 69 merchant_service_window
  created_at    timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rsr_frequency CHECK (frequency IN ('WEEKLY','BIWEEKLY','EVERY_4_WEEKS','MONTHLY')),
  -- 🔴 白名單規則，禁止任意 cron（§20.10 逐字）
  CONSTRAINT chk_rsr_anchor CHECK (
      (frequency = 'MONTHLY' AND day_of_month BETWEEN 1 AND 31 AND weekday IS NULL)
   OR (frequency <> 'MONTHLY' AND weekday IS NOT NULL AND day_of_month IS NULL)),
  CONSTRAINT chk_rsr_count CHECK (cycle_count IS NULL OR cycle_count IN (4,8,12)),
  CONSTRAINT chk_rsr_interval CHECK (interval_n >= 1)
);
```

🔴 **29／30／31 的處置**：MVP 採「該月最後一日」，並在 preview **直接列出實際日期**（§20.6 逐字）；
**不保存任意 cron**。所有規則保存 `Asia/Taipei` 與**版本化的賣家 service calendar／黑名日期**快照
（快照落在 R8 而非本表，因為它逐期可能不同）。

#### 5.1.4 `recurring_payment_mandate`（payment-service，Flyway；版號取法見 §5.0）

```sql
CREATE TABLE recurring_payment_mandate (
  id                        BIGINT PRIMARY KEY,             -- payment-service TSID
  contract_ref              BIGINT       NOT NULL,          -- recurring_contract.id；跨 DB 無 FK
  buyer_id                  BIGINT       NOT NULL,
  store_id                  BIGINT       NOT NULL,
  provider                  VARCHAR(32)  NOT NULL DEFAULT 'PAYUNI',
  provider_account_ref      VARCHAR(128) NOT NULL           -- 🔴 共用既有底座
      REFERENCES billing_provider_accounts(account_ref) ON DELETE RESTRICT,
  provider_ref_encrypted    TEXT         NOT NULL,          -- CreditHash 的 envelope encryption 密文
  kms_key_version           VARCHAR(64)  NOT NULL,          -- §8.5 key versioning
  credit_life               VARCHAR(32),                    -- PAYUNi CreditLife
  card_brand                VARCHAR(16),
  card_last4                CHAR(4),
  status                    VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
  consent_version           VARCHAR(64)  NOT NULL,
  -- 🔴 三個 amount cap，各自一欄（§0.5）
  goods_amount_cap          BIGINT       NOT NULL,
  delivery_amount_cap       BIGINT       NOT NULL,
  cycle_total_amount_cap    BIGINT       NOT NULL,
  revoked_at                TIMESTAMPTZ,
  expires_at                TIMESTAMPTZ,
  created_at                TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rpm_status CHECK (status IN ('PENDING','ACTIVE','SUSPENDED','EXPIRED','REVOKED')),
  CONSTRAINT chk_rpm_last4  CHECK (card_last4 IS NULL OR card_last4 ~ '^[0-9]{4}$'),
  CONSTRAINT chk_rpm_caps   CHECK (goods_amount_cap >= 0 AND delivery_amount_cap >= 0
                                   AND cycle_total_amount_cap >= goods_amount_cap + delivery_amount_cap)
);
-- 🔴 一個契約同時最多一個非終態 mandate（PENDING 也要防併發，§20.10 逐字）
CREATE UNIQUE INDEX uq_rpm_active_contract ON recurring_payment_mandate (contract_ref)
  WHERE status IN ('PENDING','ACTIVE','SUSPENDED');
```

🔴 **不存 PAN／CVV**（§20.10 逐字）。🔴 **不得綁平台／代理商 MerID，也不得跨 seller 重用**
⇒ `provider_account_ref` 對應的 `billing_provider_accounts` 列與 `store_id` 的一致性由
`chk` 無法跨表表達 ⇒ 落成 §5.5 的應用層不變式 INV-3 ＋ 一支契約測試（拿掉即轉紅）。

#### 5.1.5 `recurring_charge_attempt`（payment-service，Flyway；版號取法見 §5.0）

```sql
CREATE TABLE recurring_charge_attempt (
  id                    BIGINT PRIMARY KEY,
  cycle_ref             BIGINT       NOT NULL,     -- recurring_cycle.id
  mandate_id            BIGINT       NOT NULL REFERENCES recurring_payment_mandate(id),
  attempt_no            SMALLINT     NOT NULL,
  merchant_trade_no     VARCHAR(128) NOT NULL UNIQUE,   -- 🔴 每期／每次都是新的唯一值
  amount                BIGINT       NOT NULL,
  amount_hash           CHAR(64)     NOT NULL,     -- 對 cycle snapshot 金額組成的 SHA-256
  authorization_status  VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
  capture_status        VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
  provider_trade_no     VARCHAR(128),
  provider_event_key    VARCHAR(512),
  res_code              VARCHAR(128),
  res_code_msg          TEXT,
  unknown_since         TIMESTAMPTZ,               -- 🔴 UNKNOWN 起算點；query-first 的候選述詞
  next_query_at         TIMESTAMPTZ,               -- 回查排程（D-1 義①）
  created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  settled_at            TIMESTAMPTZ,
  CONSTRAINT uq_rca_cycle_attempt UNIQUE (cycle_ref, attempt_no),
  CONSTRAINT chk_rca_attempt_no   CHECK (attempt_no BETWEEN 1 AND 3),
  CONSTRAINT chk_rca_amount       CHECK (amount > 0),
  CONSTRAINT chk_rca_auth  CHECK (authorization_status IN ('PENDING','AUTHORIZED','DECLINED','UNKNOWN','VOIDED')),
  CONSTRAINT chk_rca_cap   CHECK (capture_status       IN ('PENDING','CAPTURED','FAILED','UNKNOWN','REFUNDED')),
  CONSTRAINT chk_rca_unknown CHECK ((authorization_status='UNKNOWN' OR capture_status='UNKNOWN') = (unknown_since IS NOT NULL))
);
-- 🔴 一個 cycle 同時最多一個非終態 attempt（§20.10 逐字）
CREATE UNIQUE INDEX uq_rca_open_per_cycle ON recurring_charge_attempt (cycle_ref)
  WHERE capture_status IN ('PENDING','UNKNOWN') OR authorization_status IN ('PENDING','UNKNOWN');
CREATE UNIQUE INDEX uq_rca_provider_event ON recurring_charge_attempt (provider_event_key)
  WHERE provider_event_key IS NOT NULL;
```

🔴 **`unknown_since` 與那道 partial unique 一起，才是「不得以新 `MerTradeNo` 重扣」的機械保證**：
只要有一列處於 `PENDING`／`UNKNOWN`，第二筆 attempt 的 INSERT 就會 23505。
⚠️ **PostgreSQL 交易內任一句失敗 ⇒ 整個交易 aborted**（repo §11.1）⇒ 「靠 catch 唯一鍵錯誤再繼續查」
在本 repo 必定 500；要「衝突就跳過」只有 `ON CONFLICT DO NOTHING`。

**同批擴用既有底座（與 §5.1.4／§5.1.5 同一支 migration 內，expand-only）**

```sql
ALTER TABLE billing_commands DROP CONSTRAINT chk_billing_commands_operation;
ALTER TABLE billing_commands ADD  CONSTRAINT chk_billing_commands_operation
  CHECK (operation IN ('CREATE_SUBSCRIPTION','SUSPEND_SUBSCRIPTION','RESTART_SUBSCRIPTION','END_SUBSCRIPTION',
                       'CREATE_MANDATE','REVOKE_MANDATE','CHARGE_CYCLE','QUERY_CHARGE','REFUND_CYCLE'));
ALTER TABLE billing_commands ADD COLUMN result_mandate_id BIGINT
  REFERENCES recurring_payment_mandate(id) ON DELETE RESTRICT;
ALTER TABLE billing_commands ADD CONSTRAINT chk_billing_commands_result_exclusive
  CHECK (NOT (result_subscription_id IS NOT NULL AND result_mandate_id IS NOT NULL));
```

#### 5.1.6 `recurring_cycle` ／ `recurring_cycle_snapshot`

```sql
CREATE TABLE recurring_cycle (
  id                bigint PRIMARY KEY,
  contract_id       bigint      NOT NULL REFERENCES recurring_contract(id),
  sequence_no       integer     NOT NULL,
  scheduled_for_at  timestamptz NOT NULL,       -- 到貨日（UTC；本地日期在 snapshot）
  edit_cutoff_at    timestamptz NOT NULL,       -- 🔴 D-10
  charge_cutoff_at  timestamptz NOT NULL,       -- 🔴 D-10
  status            varchar(20) NOT NULL DEFAULT 'SCHEDULED',   -- 🔴 值域見下方 CHECK（§7.3.2 的 10 值）
  next_action_at    timestamptz,
  lease_owner       varchar(64),                -- 65 §九 lease
  lease_expires_at  timestamptz,
  action_required_reason varchar(48),
  action_required_since  timestamptz,
  -- ↓ §7.0 排程器候選述詞會讀的欄位（🔴 漏建 ⇒ 排程器的 SQL 42703）
  next_charge_at        timestamptz,          -- R-S3：退避後的下次可扣時間
  action_deadline_at    timestamptz,          -- R-S6：ACTION_REQUIRED 的處理期限
                                              --   ⚠️ 與 action_required_since 不同：一個是期限、一個是起點
  charge_attempt_count  integer     NOT NULL DEFAULT 0,
                                              -- R-S3 的重試計數。🔴 **不得跨 DB 讀 payment 的 attempt_no**
                                              --   （recurring 與 payment 是兩個獨立資料庫，單一 SQL 述詞求不出來）
  version           integer     NOT NULL DEFAULT 0,
  created_at        timestamptz NOT NULL DEFAULT NOW(),
  updated_at        timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_rcy_contract_seq UNIQUE (contract_id, sequence_no),
  -- 🔴 值域以 §7.3.2 的 10 值為唯一權威。另立一套值域的症狀：與 §7 的述詞互不相交
  --    ⇒ 寫 SCHEDULED／CHARGING／PAID 一律 23514、三支排程器候選述詞恆 0 列、且零告警
  CONSTRAINT chk_rcy_status CHECK (status IN
      ('SCHEDULED','LOCKED','CHARGING','ACTION_REQUIRED','PAID',
       'MATERIALIZING','MATERIALIZED','COMPLETED','SKIPPED','CANCELLED')),
  -- 🔴 這裡**刻意不收緊成 `<= scheduled_for_at - 12h`**。曾經那樣寫過，而 publish 側
  --    （§3.4.2／§5.1.1）只要求 charge_cutoff >= lead_time + prep_time、兩者都無下界
  --    ⇒ 賣家設 prep=4／lead=0／cutoff=4 時 publish gate 全過，之後 R-S1 產出的**每一個 cycle
  --    都 23514**，而錯誤發生在排程器裡、賣家端零訊號。INV-10 改由 hold 表自己的
  --    `chk_*_lifetime` 守（見 §5.1.7），不必回頭限制賣家的截止時間設定。
  CONSTRAINT chk_rcy_time_order  CHECK (edit_cutoff_at < charge_cutoff_at
                                        AND charge_cutoff_at < scheduled_for_at),
  CONSTRAINT chk_rcy_ar_reason   CHECK (status <> 'ACTION_REQUIRED'
                                        OR (action_required_reason IS NOT NULL AND action_required_since IS NOT NULL))
);
CREATE INDEX idx_rcy_due ON recurring_cycle (next_action_at)
  WHERE status IN ('SCHEDULED','LOCKED','CHARGING','ACTION_REQUIRED','PAID','MATERIALIZING','MATERIALIZED');
-- 🔴 **7 個非終態全列**（終態＝COMPLETED／SKIPPED／CANCELLED，見 §7.3.2）。
--    🔴 漏列 PAID 或 MATERIALIZED 的症狀：R-S4 的候選述詞正是 status='PAID'
--    ⇒ 錢已扣、單未建的那一格永遠掃不到（cycle_paid_age > 30m 告警也就永不觸發）。
-- ⚠️ partial index 的述詞集合必須與上方 CHECK 的「非終態」子集一致；
--    漏一個值 ⇒ 那個狀態的候選永遠不進索引 ⇒ 排程器掃不到，而且不會報錯。

CREATE TABLE recurring_cycle_snapshot (
  id                          bigint PRIMARY KEY,
  cycle_id                    bigint      NOT NULL REFERENCES recurring_cycle(id),
  plan_version_id             bigint      NOT NULL,
  items                       jsonb       NOT NULL,   -- sku/qty/unit_price/allocated_discount
  goods_gross                 bigint      NOT NULL,
  recurring_discount_allocated bigint     NOT NULL DEFAULT 0,
  eligible_coupon_allocated   bigint      NOT NULL DEFAULT 0,
  goods_net                   bigint      NOT NULL,
  seller_delivery_charge      bigint      NOT NULL DEFAULT 0,
  seller_delivery_subsidy     bigint      NOT NULL DEFAULT 0,
  cycle_charge_amount         bigint      NOT NULL,
  address_hash                char(64)    NOT NULL,   -- 🔴 不存完整地址
  shipping_method             varchar(8)  NOT NULL DEFAULT '3005',
  seller_delivery_zone_version varchar(32) NOT NULL,
  service_window_id           bigint      NOT NULL,
  temperature                 varchar(16) NOT NULL,   -- 🔴 禁未知值默認常溫（69 §3.3）
  capacity_units              integer     NOT NULL,
  package_hash                char(64)    NOT NULL,
  delivery_fee_policy_version varchar(32) NOT NULL,
  local_scheduled_date        date        NOT NULL,   -- Asia/Taipei 當地日
  policy_hashes               jsonb       NOT NULL,
  created_at                  timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_rcs_cycle UNIQUE (cycle_id),
  CONSTRAINT chk_rcs_shipping_method CHECK (shipping_method = '3005'),   -- 🔴 D-15
  CONSTRAINT chk_rcs_amounts CHECK (
      goods_net = goods_gross - recurring_discount_allocated - eligible_coupon_allocated
  AND cycle_charge_amount = goods_net + seller_delivery_charge - seller_delivery_subsidy
  AND goods_gross >= 0 AND cycle_charge_amount >= 0)
);
```

🔴 **lock 後 append-only**：`recurring_cycle_snapshot` 由 domain 層拒絕 UPDATE；需要改就是新的 cycle。
🔴 **`chk_rcs_amounts` 把 需求書 §20.4 的計算式落成 DB 守恆式** ⇒ 「畫面、payment、seller invoice 與
seller merchant reconciliation 完全一致」（需求書 §20.23 驗收）在 DB 層有一半保證。
⚠️ **MVP 僅首期券**（`eligible_coupon_allocated` 只在 `sequence_no = 1` 可非零）——這條是應用層
不變式 INV-8，DB 守不住（跨表）。

#### 5.1.7 `recurring_inventory_hold` ／ `recurring_capacity_hold`

```sql
CREATE TABLE recurring_inventory_hold (
  id           bigint PRIMARY KEY,
  cycle_id     bigint      NOT NULL REFERENCES recurring_cycle(id),
  sku_id       bigint      NOT NULL,
  qty          integer     NOT NULL,
  -- 🔴 本欄必須 nullable，不得 NOT NULL：product-service 沒有任何 reserve／hold RPC
  --    （只有 GetStock／DeductStock／RestoreStock，§6.4）⇒ 沒有任何值的來源，
  --    設成 NOT NULL 的話 lock 步驟連列都插不進去。
  --    **MVP 決議：無硬保留**。本欄改 nullable，僅在日後 product 提供預留 RPC 時才填。
  external_hold_ref varchar(128),   -- nullable：MVP 無硬保留；有值時＝對方的 reservation id
  status       varchar(16) NOT NULL DEFAULT 'ACTIVE',
  expires_at   timestamptz NOT NULL,         -- 🔴 §0.4 的公式
  -- 🔴 自 recurring_cycle 複製而來，**lock 之後兩者皆不可變 ⇒ 零漂移風險**。
  --    存在的唯一理由：讓 INV-10 變成**單表** CHECK（跨表 CHECK 在 PostgreSQL 寫不出來，
  --    而把約束推回 recurring_cycle 會反過來限制賣家的截止時間設定 ⇒ 每個 cycle 都 23514）
  charge_cutoff_at timestamptz NOT NULL,
  release_reason varchar(32),
  created_at   timestamptz NOT NULL DEFAULT NOW(),
  released_at  timestamptz,
  CONSTRAINT chk_rih_status CHECK (status IN ('ACTIVE','COMMITTED','RELEASED','EXPIRED')),
  CONSTRAINT chk_rih_qty CHECK (qty >= 1),
  -- 🔴 INV-10 的執行面（單表、真的擋得住）
  CONSTRAINT chk_rih_lifetime CHECK (expires_at >= charge_cutoff_at + INTERVAL '24 hours'),
  CONSTRAINT chk_rih_release CHECK (status IN ('ACTIVE','COMMITTED') OR release_reason IS NOT NULL)
);
CREATE UNIQUE INDEX uq_rih_active ON recurring_inventory_hold (cycle_id, sku_id) WHERE status = 'ACTIVE';

CREATE TABLE recurring_capacity_hold (
  id           bigint PRIMARY KEY,
  cycle_id     bigint      NOT NULL REFERENCES recurring_cycle(id),
  bucket_id    bigint      NOT NULL,          -- 69 merchant_capacity_bucket.id（權威在 shipping）
  capacity_units integer   NOT NULL,
  snapshot_hash char(64)   NOT NULL,          -- 綁地址/商品/包裝/window；任一改變即失效
  status       varchar(16) NOT NULL DEFAULT 'ACTIVE',
  expires_at   timestamptz NOT NULL,
  charge_cutoff_at timestamptz NOT NULL,      -- 🔴 同 recurring_inventory_hold，理由見上表
  release_reason varchar(32),
  created_at   timestamptz NOT NULL DEFAULT NOW(),
  released_at  timestamptz,
  CONSTRAINT chk_rch_status CHECK (status IN ('ACTIVE','COMMITTED','RELEASED','EXPIRED')),
  CONSTRAINT chk_rch_units CHECK (capacity_units >= 1),
  CONSTRAINT chk_rch_lifetime CHECK (expires_at >= charge_cutoff_at + INTERVAL '24 hours')
);
CREATE UNIQUE INDEX uq_rch_active ON recurring_capacity_hold (cycle_id) WHERE status = 'ACTIVE';
```

🔴 **兩張表都不是庫存／承載本身**（§3.7-②）：原子扣減發生在 product-service／shipping-service 的
交易內，本表只記引用與到期。
🔴 **MVP 分兩種**（§3.7 改判後）：
- **容量 hold**：對方（shipping-service）真的有鎖 ⇒ **釋放必須雙向**（本地標 `RELEASED` **且**呼叫對方釋放）；
  只做一半就是「本地看起來已釋放、對方仍鎖著」，而**沒有任何錯誤訊息**（靜默陷阱 B3）。
- **庫存 hold**：**對方沒有鎖**（product 無 reserve RPC）⇒ `external_hold_ref` 為 NULL、
  只需本地標 `RELEASED`，B3 的情境不會發生。日後 product 提供預留 RPC 時再回到雙向釋放。

#### 5.1.8 `recurring_cycle_order`

```sql
CREATE TABLE recurring_cycle_order (
  cycle_id   bigint PRIMARY KEY REFERENCES recurring_cycle(id),
  order_id   bigint      NOT NULL,
  order_no   varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_rco_order UNIQUE (order_id),
  CONSTRAINT uq_rco_order_no UNIQUE (order_no)
);
```

🔴 **`cycle_id` 與 `order_id` 皆 unique** ⇒ 「相同 cycle 在多 replica、crash／restart、MQ 重播下
只 materialize 一張 order」（需求書 §20.23 驗收）的**唯一機械保證**。
🔴 **`cycle_id` 就是 PK**——這比「另開 id ＋ 兩道 unique」少一個可寫錯的地方。

#### 5.1.9 `recurring_action_audit`（append-only）

```sql
CREATE TABLE recurring_action_audit (
  id             bigint PRIMARY KEY,
  aggregate_type varchar(24) NOT NULL,   -- PLAN / CONTRACT / CYCLE / MANDATE
  aggregate_id   bigint      NOT NULL,
  action         varchar(48) NOT NULL,
  actor_type     varchar(16) NOT NULL,   -- BUYER / SELLER / ADMIN / SYSTEM
  actor_id       bigint,                 -- BUYER=buyer_id / SELLER=store_id / ADMIN=account_id
  reason         text,
  before_hash    char(64),
  after_hash     char(64),
  trace_id       varchar(64),
  created_at     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_raa_actor CHECK (actor_type IN ('BUYER','SELLER','ADMIN','SYSTEM')),
  CONSTRAINT chk_raa_actor_id CHECK (actor_type = 'SYSTEM' OR actor_id IS NOT NULL),
  CONSTRAINT chk_raa_reason CHECK (actor_type <> 'ADMIN' OR reason IS NOT NULL)
);
CREATE INDEX idx_raa_aggregate ON recurring_action_audit (aggregate_type, aggregate_id, created_at DESC);
```

⚠️ **`actor_id` 的語意隨 `actor_type` 變**（ADMIN 用 `account_id`，其餘用業務鍵）——
這正是 65 §3.9 一警告的混用來源，欄位註解必須寫死，且 §14 術語表要收錄。

#### 5.1.10 `recurring_preview`

§20.11 有 `previewId` ＋ `expiresAt`，且要求「Preview 到期、plan version 變更或任何快照 hash 不一致
就回 409／412 要求重做，**不得後端偷偷換價**」⇒ **hash 必須伺服器保存**，否則無從比對。

```sql
CREATE TABLE recurring_preview (
  id                bigint PRIMARY KEY,
  buyer_id          bigint      NOT NULL,
  store_id          bigint      NOT NULL,
  plan_version_id   bigint      NOT NULL,
  request_hash      char(64)    NOT NULL,
  preview_hash      char(64)    NOT NULL,   -- 對整份 response 的正規化 JSON SHA-256
  payload           jsonb       NOT NULL,
  expires_at        timestamptz NOT NULL,
  consumed_by_contract_id bigint,
  created_at        timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rpw_hash CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT chk_rpw_expiry CHECK (expires_at > created_at)
);
CREATE UNIQUE INDEX uq_rpw_consumed ON recurring_preview (consumed_by_contract_id)
  WHERE consumed_by_contract_id IS NOT NULL;
```

⚠️ **`uq_rpw_consumed` 讓「一份 preview 只能建一份 contract」成為 DB 事實**，
與 `Idempotency-Key` 是兩道**互補**的防線（一道防重送、一道防重用）。

#### 5.1.11 `recurring_cycle_settlement_fact` ／ `recurring_reconciliation_case`（§3.7-③）

```sql
CREATE TABLE recurring_cycle_settlement_fact (
  cycle_id                 bigint PRIMARY KEY REFERENCES recurring_cycle(id),
  order_id                 bigint,
  seller_merchant_trade_no varchar(128),
  captured_amount          bigint  NOT NULL DEFAULT 0,
  refunded_amount          bigint  NOT NULL DEFAULT 0,
  provider_processing_fee  bigint,             -- 🔴 provider 對賣家收的，不是平台費用
  provider_reported_deposit_amount bigint,     -- 🔴 只供觀測（§20.15 逐字）
  seller_invoice_ref       varchar(64),        -- 58 號 invoice_documents 的識別；跨 DB 無 FK
  seller_invoice_status    varchar(32),
  delivery_status          varchar(8),         -- 65 normalized 值（唯讀快照）
  last_synced_at           timestamptz,
  created_at               timestamptz NOT NULL DEFAULT NOW(),
  updated_at               timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rcsf_amounts CHECK (captured_amount >= 0 AND refunded_amount >= 0
                                     AND refunded_amount <= captured_amount)
);

CREATE TABLE recurring_reconciliation_case (
  id             bigint PRIMARY KEY,
  cycle_id       bigint      NOT NULL REFERENCES recurring_cycle(id),
  case_type      varchar(32) NOT NULL,   -- AMOUNT_MISMATCH / INVOICE_MISSING / REFUND_MISMATCH / DELIVERY_MISMATCH
  status         varchar(16) NOT NULL DEFAULT 'OPEN',
  opened_reason  text        NOT NULL,
  opened_at      timestamptz NOT NULL DEFAULT NOW(),
  closed_by_account_id bigint,
  closed_reason  text,
  closed_at      timestamptz,
  CONSTRAINT chk_rrc_status CHECK (status IN ('OPEN','IN_PROGRESS','CLOSED')),
  CONSTRAINT chk_rrc_closed CHECK (status <> 'CLOSED'
      OR (closed_by_account_id IS NOT NULL AND closed_reason IS NOT NULL AND closed_at IS NOT NULL))
);
CREATE UNIQUE INDEX uq_rrc_open ON recurring_reconciliation_case (cycle_id, case_type)
  WHERE status IN ('OPEN','IN_PROGRESS');
```

🔴 **沒有自動比對排程**（D-1 義②）。差異的**呈現**走 ADM 畫面與維運 SQL；
`recurring_reconciliation_case` 只由**事件**建立（退款成立、發票狀態變更、statement 匯入），
每一種都要答「誰推走」＝ **ADM 關案**（`chk_rrc_closed` 強制留下人與理由）。

#### 5.1.12 `recurring_substitution_consent`（🔴 需求書無表無欄）

需求書 §20.16 逐字：「買家若**事前**對『**明確列名的等價 SKU**』**逐項同意**，才可替換」「**沒有明確同意就不替換**」
⇒ 需要一張逐項的同意表，而需求書的表清單裡沒有。

```sql
CREATE TABLE recurring_substitution_consent (
  id                 bigint PRIMARY KEY,
  contract_id        bigint      NOT NULL REFERENCES recurring_contract(id),
  original_sku_id    bigint      NOT NULL,
  substitute_sku_id  bigint      NOT NULL,      -- 🔴 必須明確列名，禁「同類商品」這種範圍授權
  max_unit_price     bigint      NOT NULL,      -- 替代價上限；仍受三個 amount cap 約束
  status             varchar(16) NOT NULL DEFAULT 'ACTIVE',
  consented_at       timestamptz NOT NULL,
  consent_evidence_id bigint     NOT NULL,
  withdrawn_at       timestamptz,
  CONSTRAINT chk_rsc_status CHECK (status IN ('ACTIVE','WITHDRAWN')),
  CONSTRAINT chk_rsc_different CHECK (original_sku_id <> substitute_sku_id),
  CONSTRAINT chk_rsc_withdrawn CHECK (status <> 'WITHDRAWN' OR withdrawn_at IS NOT NULL)
);
CREATE UNIQUE INDEX uq_rsc_active ON recurring_substitution_consent
  (contract_id, original_sku_id, substitute_sku_id) WHERE status = 'ACTIVE';
```

#### 5.1.13 既有 `orders` 擴充（order-service，Flyway；版號取法見 §5.0）

```sql
-- V{N}__add_order_recurring_origin.sql（order-service；N 於施工當下依 §5.0 取號，禁跳號）
-- expand-contract：先帶 DEFAULT 加欄 → 回填 → 才加 CHECK；不得一步 NOT NULL 無 DEFAULT，
-- 否則 rolling 期間舊 pod 的 INSERT 全數 23502。
ALTER TABLE orders ADD COLUMN order_origin          VARCHAR(24) NOT NULL DEFAULT 'ONE_TIME_PURCHASE';
ALTER TABLE orders ADD COLUMN origin_ref            BIGINT;      -- = recurring_cycle_id
ALTER TABLE orders ADD COLUMN recurring_contract_id BIGINT;      -- 🔴 只供查詢

ALTER TABLE orders ADD CONSTRAINT chk_orders_order_origin
  CHECK (order_origin IN ('ONE_TIME_PURCHASE','RECURRING_PURCHASE'));

-- 🔴 §20.10 逐字：order_origin = RECURRING_PURCHASE ⇒ DB 與 domain 同時要求
--    fulfillment_mode = SELLER_MANAGED 且 provider = MERCHANT；其他值直接拒絕。
--    provider 維度由既有 orders.shipping_method 承載（65 §5.1.1：再開一欄就是製造第二個真相）
--    ⇒ provider = MERCHANT 落成 shipping_method = '3005'。
ALTER TABLE orders ADD CONSTRAINT chk_orders_recurring_origin
  CHECK (order_origin <> 'RECURRING_PURCHASE'
         OR (fulfillment_mode = 'SELLER_MANAGED'
             AND shipping_method = '3005'
             AND origin_ref IS NOT NULL
             AND recurring_contract_id IS NOT NULL));

CREATE UNIQUE INDEX uq_orders_origin_ref ON orders (origin_ref) WHERE origin_ref IS NOT NULL;
CREATE INDEX idx_orders_recurring_contract ON orders (recurring_contract_id) WHERE recurring_contract_id IS NOT NULL;
```

⚠️ **`fulfillment_mode` 由 65 建立** ⇒ 本節這支 migration 的版號**必須大於 65 建立 `fulfillment_mode`
的那一支**，否則 CHECK 引用不存在的欄位。這是跨兩本設計書的**部署順序依賴**，§12.2 要寫死。
🔴 **`uq_orders_origin_ref` 是「一 cycle 一張普通訂單」在 order-service 側的第二道防線**
（第一道在 `recurring_cycle_order`，兩個服務各一道，因為它們不能同交易）。

#### 5.1.14 🔴 本書明確**不建立**的表（供 review 機械檢查）

以下九項，**在 recurring-service 的 schema 與 migration 中出現任何一個，即為違反 D-7**：

| # | 禁止出現 | 需求書出處 |
|---|---|---|
| 1 | 平台買家發票（platform buyer invoice） | §20.15 逐字 |
| 2 | seller-to-platform invoice obligation | §20.15 逐字 |
| 3 | 1% 撥款指示費（任何 `fee_rate`／`payout_instruction_fee`） | §20.15 逐字 |
| 4 | `seller_net` | §20.15 逐字 |
| 5 | negative balance | §20.15 逐字 |
| 6 | `payout_batch` | §20.15 逐字 |
| 7 | `settlement_candidate` | §20.15 逐字 |
| 8 | `payout_instruction` | §20.15 逐字 |
| 9 | Grant | §20.15 逐字 |

**機械檢查方式**（§13 要落成一支 gate）：對 `services/recurring-service/db/schema/schema.sql`
grep 上列九個識別字，命中即 fail。🔴 **對照組必須同批驗**：grep 一個確知存在的字串
（例 `recurring_cycle`）必須有命中，否則「零命中」證明的是 gate 沒掃到檔案，不是沒有違規。

#### 5.1.15 `recurring_outbox_events` ／ `recurring_inbox_events`

形狀比照 58 號既有的 `invoice_outbox_events`／`invoice_inbox_events`（**不要重新發明**），並直接
繼承它的那條血淚註解：

🔴 **`status='failed'` 不由任何程式路徑寫入**。耗盡重試走「**停泊**」（維持 `pending`、
`next_attempt_at` 推 1 小時）。理由：候選述詞與 pending index 全是 `status='pending'` 且沒有任何
時間欄能讓 `failed` 回到候選集合 ⇒ 那是**永久旗標**；更糟的是 `CountPending`／`OldestPending`
的述詞同樣是 `pending` ⇒ **一列落到 `failed` 就退出 aging 指標 ⇒ 告警會在事件真正卡死那一刻由
firing 轉 resolved**（65 §3.10 的教科書實例）。`failed` 值域保留只給**維運 SQL 隔離毒 payload**，
並配一條 `recurring_outbox_stranded_rows` 指標把它數出來。

#### 5.1.16 `recurring_admin_proposal` ／ `recurring_mandate_consent`

🔴 **這兩張表是 §6.3 與 §8.7 的前置**，漏建的症狀不是編譯錯：
§6.3 的兩道守衛寫成 `UPDATE … WHERE id=$1 AND status='PENDING' AND maker_account_id<>$2` ＋ DB CHECK，
而 §5.1.2 的 `consent_evidence_id` 是 `NOT NULL`
⇒ 漏建就會做出一個「**建約永遠插不進去、maker-checker 永遠 42P01**」的 schema。

⚠️ **「maker-checker 沿用 58 號既有模式（D-20），不另建」指的是「不另建一套機制」**——
表還是要建：`invoice_manual_unlocks` 住在 invoice-service 的資料庫，
recurring-service 是另一個服務、另一個資料庫，**跨 DB 不能共用一張表也不能下 FK**。

```sql
CREATE TABLE recurring_admin_proposal (
  id                  bigint PRIMARY KEY,               -- TSID；＝ API 的 :proposal_id
  contract_id         bigint      NOT NULL REFERENCES recurring_contract(id),
  proposal_type       varchar(16) NOT NULL,             -- CANCELLATION（RA-4）／TERMINATION（RA-6）
  status              varchar(16) NOT NULL DEFAULT 'PENDING',
  maker_account_id    bigint      NOT NULL,             -- 🔴 admin 的登入 principal，不是 buyer_id／store_id
  maker_reason        text        NOT NULL,
  impact_preview_hash char(64)    NOT NULL,             -- RA-3 的快照 hash；覆核當下重算不符 ⇒ *_STALE（§6.3.2）
  refund_disposition  varchar(16),                      -- TERMINATION 專用：FULL_REFUND／ALLOWANCE
  checker_account_id  bigint,
  checker_reason      text,
  decided_at          timestamptz,
  created_at          timestamptz NOT NULL DEFAULT NOW(),
  updated_at          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rap_type   CHECK (proposal_type IN ('CANCELLATION','TERMINATION')),
  CONSTRAINT chk_rap_status CHECK (status IN ('PENDING','APPROVED','REJECTED')),
  -- 🔴 雙人守衛的 DB 兜底（仿 invoice-service 既有 chk_invmu_two_person）。
  --    應用層的 if 會在下一次重構被抽掉而沒有人發現（§6.3 逐字）。
  CONSTRAINT chk_rap_two_person CHECK (checker_account_id IS NULL
                                       OR checker_account_id <> maker_account_id),
  -- 🔴 離開 PENDING 就必須留下「誰決定的、什麼時候」
  CONSTRAINT chk_rap_decided CHECK (status = 'PENDING'
      OR (checker_account_id IS NOT NULL AND decided_at IS NOT NULL)),
  -- 🔴 駁回必填理由（§6.3.1 逐字）；只寫在應用層的話，駁回原因會靜默留空
  CONSTRAINT chk_rap_reject_reason CHECK (status <> 'REJECTED' OR checker_reason IS NOT NULL)
);
-- 🔴 一份契約同時只能有一件待覆核提案：否則兩個 maker 各提一件、兩個 checker 各批一件，
--    雙人守衛逐件都成立而契約被取消兩次。partial unique 是唯一擋得住的地方。
CREATE UNIQUE INDEX uq_rap_pending ON recurring_admin_proposal (contract_id)
  WHERE status = 'PENDING';
CREATE INDEX idx_rap_contract ON recurring_admin_proposal (contract_id, created_at DESC);

CREATE TABLE recurring_mandate_consent (
  id            bigint PRIMARY KEY,          -- TSID；＝ recurring_contract.consent_evidence_id
                                             --   與 recurring_substitution_consent.consent_evidence_id
  contract_id   bigint,                      -- 🔴 nullable：建約當下契約還沒有 id（同 tx 先寫本表）
  buyer_id      bigint      NOT NULL,
  status        varchar(16) NOT NULL DEFAULT 'ACTIVE',
  -- ↓ 內容欄位的清單以 §8.7 為唯一權威，本處不重述（重述必然漂移）
  consent_payload jsonb     NOT NULL,        -- terms_hash／顯示的三個 cap／收款主體／頻率／取消方式版本／ip／ua／device／provider result
  consented_at  timestamptz NOT NULL,
  superseded_at timestamptz,
  created_at    timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_rmc_status CHECK (status IN ('ACTIVE','SUPERSEDED')),
  CONSTRAINT chk_rmc_superseded CHECK (status <> 'SUPERSEDED' OR superseded_at IS NOT NULL)
);
CREATE UNIQUE INDEX uq_rmc_active ON recurring_mandate_consent (contract_id)
  WHERE status = 'ACTIVE' AND contract_id IS NOT NULL;
```

🔴 **`recurring_mandate_consent` 是 append-only**：變更收款主體或任一 cap ⇒ 舊列
`status='SUPERSEDED'` ＋ 新增一列，**禁止就地 UPDATE `consent_payload`**（§8.7）。
就地改的症狀是爭議時查不出「當時同意的是哪一版」——而那正是這張表存在的唯一理由。

**四問**（65 §3.11）：

| 問 | `recurring_admin_proposal` | `recurring_mandate_consent` |
|---|---|---|
| 誰推進來 | RA-4／RA-6（maker），`status='PENDING'` | RB-4 建約 saga，與 `recurring_contract` **同一個本地 tx** |
| 誰推走 | RA-5／RA-7（checker）→ `APPROVED`／`REJECTED` | RB-18 之外的**變更收款主體或 cap** → `SUPERSEDED` |
| 推走那一句的 allowed-from | `WHERE id=$1 AND status='PENDING' AND maker_account_id<>$2`；🔴 `rows=0` ⇒ **rollback ＋ 回業務碼**（重查後分流 `RECURRING_PROPOSAL_ALREADY_REVIEWED`／`RECURRING_SELF_APPROVAL_DENIED`），**不得記 log 放行**——它是治理主體 | `WHERE id=$1 AND status='ACTIVE'` |
| 卡住誰會知道 | 🔴 **`recurring_proposal_pending_age > 24h`**（本表沒有排程器推走，唯一出口是人）；缺它 ⇒ 提案掛著沒人批而完全無聲 | 🔴 **換卡不算變更收款主體**（§8.7）⇒ 本表不該因換卡而長列；`recurring_mandate_consent_orphan_total`（`contract_id IS NULL` 且建立超過 1h ⇒ 建約 tx 曾經失敗） |

⚠️ **`recurring_proposal_pending_age` 是本節新增的第 8 條 aging**，§8.2 的「Contract 5 格、Cycle 7 格」
不涵蓋它（它不是狀態機的格子，是一張治理表的積壓）⇒ 要單獨列進 §8.2.2。

### 5.2 索引與查詢模式

| 查詢 | 索引 |
|---|---|
| scheduler 取候選 | `idx_rcy_due ON recurring_cycle (next_action_at) WHERE status IN (五個非終態)` —— **partial**，終態不進索引 |
| 買家「我的定期配送」 | `(contract_id, sequence_no)` 既有 unique 足夠 ＋ `idx_rc_buyer ON recurring_contract (buyer_id, status)` |
| 賣家備貨日曆 | `idx_rcy_store_date ON recurring_cycle (contract_id, scheduled_for_at)` ＋ 以 `recurring_contract.store_id` join |
| ADM correlation view | `idx_raa_aggregate`、`uq_rco_order` |
| **分割** | 週期量大後依 `scheduled_for_at` 的**月**分割 `recurring_cycle`（§20.10 逐字）；終態月份 detach 而非 delete |

### 5.3 序號、排序與一致性：unique 約束總表

| # | 保證 | 落成 |
|---|---|---|
| U1 | 一 cycle 最多一張普通訂單 | `recurring_cycle_order` PK(`cycle_id`) ＋ `uq_rco_order` ＋ order 側 `uq_orders_origin_ref` |
| U2 | 一 cycle 最多一個 active inventory hold／SKU | `uq_rih_active` |
| U3 | 一 cycle 最多一個 active capacity hold | `uq_rch_active` |
| U4 | 一 cycle 同時最多一個非終態 charge attempt | `uq_rca_open_per_cycle` |
| U5 | `(cycle, attempt_no)` 唯一 | `uq_rca_cycle_attempt` |
| U6 | 一契約同時最多一個非終態 mandate | `uq_rpm_active_contract`（🔴 **含 `PENDING`**——§20.10 逐字「pending 狀態也要防併發重複」） |
| U7 | provider callback 唯一鍵 | `uq_rca_provider_event`；🔴 鍵**必須含 `provider` ＋ merchant scope ＋ provider event/transaction id**（§20.10 逐字）⇒ `provider_event_key` 的組成式為 `provider\|account_ref\|event_id`，**不得只用 event_id** |
| U8 | 同一 `Idempotency-Key` 重送 100 次只建一份 contract | `billing_commands (operation, idempotency_key)` UNIQUE ＋ `uq_rpw_consumed` |
| U9 | 一契約一 SKU 只有一列生效中的 item | `uq_rci_active_sku` |
| U10 | 一 cycle 同一 case_type 只有一張開啟中的案件 | `uq_rrc_open` |

### 5.4 Schema 演進與遷移

- **expand-only、nullable、feature flag**（需求書 §20.21 逐字）：新欄一律「先帶 DEFAULT 加上去 → 回填 →
  才加 CHECK」；**不得**一步 `NOT NULL` 無 DEFAULT。
- **Go 側走 Atlas**：改 `db/schema/schema.sql` → `atlas migrate diff` → `atlas migrate hash`；
  🔴 **不得手寫 migration 繞過 checksum**。
- **新增能由既有欄位推導的欄位時**，同一個 migration 檔內緊接回填 `UPDATE`（repo §11.2 教訓：
  `ADD COLUMN ... DEFAULT 'x'` 會把**每一列**都填成 `x`，不變式從第一天就破）。
- 🔴 **不重用 buyer-service 的 `seller_subscriptions`／`restock_subscription`**（§2.5）。

### 5.5 不變式清單

| # | 不變式 | 守在哪 | 🔴 拿掉會怎樣 |
|---|---|---|---|
| INV-1 | 一 cycle 一張普通訂單 | DB（U1，兩服務各一道） | 重複建單、重複出貨 |
| INV-2 | 一 cycle 同時最多一筆非終態扣款 | DB（U4） | **重複扣款** |
| INV-3 | mandate 的 `provider_account_ref` 必屬同一 `store_id` | 應用層 ＋ 契約測試（跨表，DB 守不住） | **跨 seller 重用 mandate**（需求書 §20.23 零容忍） |
| INV-4 | `cycle_charge_amount` 恆等於計算式 | DB（`chk_rcs_amounts`） | 畫面、扣款、發票三處金額不一致 |
| INV-5 | `order_origin=RECURRING_PURCHASE ⇒ shipping_method='3005' ∧ fulfillment_mode='SELLER_MANAGED'` | DB（`chk_orders_recurring_origin`） | 定期配送訂單走上黑貓／7-11／Uber |
| INV-6 | plan item 的 SKU 必屬同一賣家 | 應用層（publish gate） | 跨賣家契約 |
| INV-7 | snapshot／audit 禁止物理刪除 | 應用層 ＋ 無 DELETE 語句的 repository | 稽核鏈斷裂 |
| INV-8 | 券只在首期分攤 | 應用層 | 每期重複用券 |
| INV-9 | `edit_cutoff_at < charge_cutoff_at < scheduled_for_at` | DB（`chk_rcy_time_order`） | 扣款早於鎖單（D-10 的原始矛盾復活） |
| INV-10 | **容量** hold 的 `expires_at` 必活過 charge 視窗 | **DB 強制（單表）**：`chk_rch_lifetime`＝`expires_at >= charge_cutoff_at + INTERVAL '24 hours'`，靠 hold 表內的冗餘 `charge_cutoff_at`（lock 時自 cycle 複製、此後不可變）。推導與「為什麼不推回 `recurring_cycle`」見 §0.4 | 扣款成功時承載已被別人拿走。🔴 **庫存側不適用**——無硬保留（§3.7），但 `chk_rih_lifetime` 仍照掛（欄位在、值算得出，日後補硬保留時不必再改 schema） |
| INV-11 | 🔴 recurring schema 不含 §5.1.14 的九個識別字 | CI gate（§13） | D-7 被違反且**沒有任何測試會紅** |
| INV-12 | `billing_commands` 的 `result_subscription_id`／`result_mandate_id` 互斥 | DB（`chk_billing_commands_result_exclusive`） | 兩個 domain 的結果互相覆蓋 |

### 5.6 靜默陷阱表（本章）

| # | 陷阱 | 為什麼不會被發現 | 防線 |
|---|---|---|---|
| B1 | `uq_rpm_active_contract` 漏掉 `PENDING` | 只在「買家在 hosted flow 開兩個分頁」時觸發 ⇒ 兩個 mandate，測試不會造這個情境 | §5.3 U6 明列 ＋ 併發測試 |
| B2 | `provider_event_key` 只用 `event_id` | 單 provider 時完全正常；接第二個 provider 或第二個 merchant scope 才撞 | §5.3 U7 的組成式 |
| B3 | hold 只釋放本地一半 | 本地看起來已釋放、對方仍鎖著；承載慢慢漏光而**零錯誤**。🔴 **只適用於容量 hold**——庫存側無硬保留（§3.7），對方本來就沒鎖 | §5.1.7 雙向釋放 ＋ 對帳指標 `hold_orphan_total` |
| B4 | `orders` 擴充的 migration 早於 65 建立 `fulfillment_mode` 的那一支 | CHECK 引用不存在的欄位 ⇒ **Flyway 失敗擋住整個 order-service 部署** | §5.1.13 順序依賴寫進 §12.2 |
| B5 | outbox 用 `failed` 而非停泊 | 卡死的那一刻 aging 告警**由 firing 轉 resolved** | §5.1.15 |
| B6 | §5.1.14 的 gate 掃不到檔案 | 零命中被讀成「沒有違規」 | 對照組（grep 一個確知存在的字串必須命中） |
| B7 | `chk_rcs_amounts` 被當成「金額一致」的全部保證 | 它只守 snapshot 內部；snapshot 與 payment／invoice 的一致靠 `amount_hash`，兩者是不同的東西 | §5.5 INV-4 與 U7 分開列 |

## 6. API 與介面契約
> 共用規約只引用不重寫 → **65 §3.9**：五 API（namespace／envelope／200＋業務碼／TSID／分頁／Admin group 明示）、二 金額字串、三 ISO 8601 UTC。

### 6.0 四條硬規則

- **R1 路徑分群＝授權分群**：`/app/recurring_*` 買家面（`JWTAuth()`）；`/app/seller_recurring_*` 賣家面（`sellerHandle(g,m,path,cap,h)`，46 號要求 capability 必填）。同一資源不得混掛兩種授權。
- **R2** 🔴 **同層 path param 名必須一致**：`:contract_id`／`:cycle_id`／`:plan_id`／`:mandate_id` **各自獨立 group**，位置 2 的名字群內恆定。gin 對同位置不同參數名是**啟動即 panic**，不是 404。
- **R3** 🔴 **不新增也不修改任何 `/app/orders/*` 路由**：既有 `:order_no`（`gateway/internal/router/constants.go` 的 `OrderNo`／`OrderConfirmReceipt`／`OrderLogistics`）與其 capability、middleware 維持原狀。
### 6.1 Buyer（**18 支**）

全部 `/app/*` ＋ `JWTAuth()`；身分鍵取自 `buyerid` resolver 的 `buyer_id`，**body 不得帶 `buyerId`**。
業務碼對應（名字見 §6.7）：查無 ⇒ `*_NOT_FOUND`；狀態不允許 ⇒ `RECURRING_{CONTRACT,CYCLE}_STATE_INVALID`；過截止 ⇒ `RECURRING_{EDIT,CHARGE}_CUTOFF_PASSED`；preview ⇒ `RECURRING_PREVIEW_{EXPIRED,MISMATCH,INPUT_INVALID}`；授權 ⇒ `RECURRING_MANDATE_*`；容量 ⇒ `RECURRING_CAPACITY_UNAVAILABLE`。

| 代號 | Method｜Path | 用途 |
|---|---|---|
| RB-1／2 | GET `/app/recurring_plans`、`…/:plan_id` | 方案清單（`?store_id=&page=&page_size=`）／詳情（**只回 published version**） |
| RB-3 🆕 | POST `/app/recurring_previews` | **試算**，回 `previewId`／`expiresAt`／`snapshotHash` |
| RB-4 | POST `/app/recurring_contracts` | 建約（**必帶 `previewId`**） |
| RB-5 | GET `/app/recurring_contracts` | 我的契約清單 |
| RB-6 | GET `/app/recurring_contracts/:contract_id` | 詳情 ＋ **`allowedActions[]`**（唯一出口，§8.1） |
| RB-7 | PATCH `/app/recurring_contracts/:contract_id` | 改地址／時段／品項／數量；**`If-Match: <version>` 必填** |
| RB-8／9／10 | POST `…/:contract_id/pauses`｜`/resumptions`｜`/cancellations` | 暫停（`resumeAt` 選填；**不填＝無限期、須手動恢復**，§7.0 已裁決無自動恢復排程）／恢復／取消（`effective`＝`IMMEDIATE`｜`AT_CUTOFF`） |
| RB-11 | GET `…/:contract_id/cycles` | **看未來幾期**（`?from=&to=`，含金額預估與 `allowedActions[]`） |
| RB-12 | POST `/app/recurring_cycles/:cycle_id/skips` | 跳過該期（edit_cutoff 前） |
| RB-13 | POST `…/:cycle_id/reschedules` | 改期 |
| RB-14 | POST `…/:cycle_id/amount_approvals` | **同意本期超額**（語意＝同意超過上限） |
| RB-15 🆕 | POST `…/:cycle_id/charges` | 🔴 **每期手動確認付款**（需求書 §20.22 P2）——與 RB-14 是**兩件事**，需求書混成一支 |
| RB-16／17 🆕 | GET／POST `/app/recurring_mandates`、POST `…/:mandate_id/revocations` | 查詢／建立付款授權（POST 回 PayUni 綁卡導向）／解除授權 |
| RB-18 🆕 | PUT `/app/recurring_contracts/:id/payment_mandate` | **換卡**：把契約指到另一個已 `ACTIVE` 的 mandate。🔴 **同 tx 更新兩欄**：`payment_mandate_ref` ＋ `mandate_expires_at`（取自 `payment.GetRecurringMandate` 的同步回應）。`expiring_notified_for` **不必清空**——新到期日已與舊值不等，R-S7 述詞自動重新成立（少一個要維護的清空點）。⚠️ 缺這支端點的話 `mandate_expires_at` 沒有刷新路徑（§7.0 四問第 2 格）：RB-16 只建 mandate、不會回頭改契約 |

#### 6.1.1 Preview 完全由後端計算

**request 只收**：`planId`＋`versionNo`、`items[]{skuId,quantity}`、`deliveryAddress`、`deliveryWindow`、`scheduleRule`、`couponCodes[]`。🔴 **前端不得傳、後端一律拒絕**（`RECURRING_PREVIEW_INPUT_INVALID`，**不是靜默忽略**——靜默忽略會讓前端以為自己送的值生效）：`fulfillmentMode`、`shippingMethod`／`provider`、`shippingFee`、`discountAmount`、`merId`、`invoiceIssuer`、任何 `settlement*`／`payout*`。

**response 必含**（缺一則不得用於 RB-4）：`previewId`(TSID)、`expiresAt`(TTL 15 分)、`snapshotHash`(SHA-256)、`fulfillmentMode`（恆 `SELLER_MANAGED`）、`shippingMethod`（恆 `3005`）、每期 `itemsAmount`／`shippingAmount`／**`eligibleCouponAllocated`**／`cycleTotalAmount`（**全部 string**）、`firstCycle{scheduledFor,editCutoffAt,chargeCutoffAt}`、`suggestedCaps{三個}`。
> 🔴 `eligibleCouponAllocated` **必須在 response 裡**：跨期優惠分攤的計算式需要它，漏掉的話分攤會落到前端，**而前端算不到**。

**RB-4 三道檢查（順序不可換）**：① `previewId` 未過期 ② 重算並比對 `snapshotHash`，不符即拒（**不得沿用舊值下單**）③ mandate 為 `ACTIVE`。

#### 6.1.2 preview request／response 契約（🔴 補 coupon 欄位——計算式有 `eligible_coupon_allocated` 但 request/response 都沒有）

欄位清單見 §6.1.1（request 六項、response 十項以上）。本節只補**那一個缺的欄位**與它的算法歸屬。

| 欄位 | 在哪一側 | 誰算 | 缺了會怎樣 |
|---|---|---|---|
| `eligibleCouponAllocated` | **response** | 🔴 **後端**（跨期分攤） | 需求書的每期金額計算式用了它，而 request／response 兩邊都沒有 ⇒ 照需求書實作，**分攤會落到前端**，而前端拿不到「這張券還剩多少期可用」 |

**算法歸屬的判準**：這個值需要「跨期」的資訊（券總額 ÷ 剩餘期數）⇒ 只有後端算得出來。
前端傳的 `couponCodes[]` 是**意圖**，`eligibleCouponAllocated` 是**結果**，兩者不可混用。

🔴 **response 的每一個金額欄都要進 `snapshotHash`**——RB-4 會重算並比對，
漏一個欄位進 hash ⇒ 該欄可以被竄改而檢查不出來。
#### 6.1.3 建立／查詢／修改契約（`If-Match` 或 body version；只改未鎖單未來資料）

| 動作 | 端點 | 併發控制 | 可改的範圍 |
|---|---|---|---|
| 建立 | RB-4 | — | — |
| 查詢 | RB-5／6 | — | — |
| **修改** | RB-7 | 🔴 **`If-Match`（ETag ＝ `contract.version`）或 body 帶 `version`，二選一但全書一致** | **只改未鎖單的未來 cycle** |

🔴 **「只改未來」的實作面**：修改只影響 `status='SCHEDULED'` 且 `edit_cutoff_at > now` 的 cycle。
已 `LOCKED` 以後的一律不動——**包含金額**（已鎖單的期數用當時的快照）。

⚠️ **version 衝突回 `RECURRING_CONTRACT_VERSION_CONFLICT`（409）並帶當前版本**，
讓前端可以「重新載入後再送一次」而不是盲目重試。

⚠️ **換卡（RB-18）不走這條**：它只改 `payment_mandate_ref` 與 `mandate_expires_at` 兩欄，
不受「只改未來」的限制（換卡要對**所有**未扣款的期數生效）。
#### 6.1.4 pause／resume／cancel／skip／reschedule／approve_amount

| 動作 | 對象 | 允許的來源狀態 | 副作用 | 可逆 |
|---|---|---|---|---|
| `pause` | Contract | `ACTIVE` | 停止產生新 cycle；已 `LOCKED` 的**照跑** | ✅ |
| `resume` | Contract | `PAUSED` | 恢復產生；🔴 **`resumeAt` 選填**（RB-8）⇒ 不得用 CHECK 強制它非空 | — |
| `cancel` | Contract | `ACTIVE`／`PAUSED`／`ACTION_REQUIRED` | 未鎖單的 cycle 一併 `CANCELLED`；**已扣款的走退款鏈** | ❌ |
| `skip` | **Cycle** | `SCHEDULED` | 該期不出貨、不扣款 | ✅（在 `edit_cutoff_at` 前） |
| `reschedule` | **Cycle** | `SCHEDULED` | 改到貨日 ⇒ **重算三個時間欄與承載** | ✅（同上） |
| `approve_amount` | **Cycle** | `ACTION_REQUIRED{reason=AMOUNT_CAP}` | 同意本期超過 cap 的金額 | ❌（同意即扣款） |

🔴 **`approve_amount` 的語意是「同意超額」不是「手動付款」**——兩者常被混為一談，見 §6.1.5。

🔴 **本表六個動作的值域 SSOT ＝ 65 附錄 C.2 的 `RecurringAction`**（2026-09-09 定案，六值逐字相同）。
⚠️ **`approve-amount` 已改名為 `approve_amount`**：65 §4.4.1c 訂的命名規則是全值域一律 `snake_case`，
而本書原文是全 repo 唯一的 kebab-case ⇒ **前端拿 `approve-amount` 去查生成的 enum 會查不到**，
而依 fail-closed 規約那顆按鈕會**靜默消失**（65 §6A 的 `I-9` 描述的症狀）。
🔴 **本軸與配送軸是兩個 enum**：`allowedActions[].action` 在 `recurring_contract`／`recurring_cycle` 上走 `RecurringAction`，
在 `shipment`／`order` 履約面上走 `FulfillmentAction`。
⚠️ **兩者都有 `cancel` 而對象不同**（契約 vs 配送單）⇒ 前端不得用單一 mapping 把 `cancel` 綁死到一個文案或端點；
URL 一律跟隨 `allowedActions[].href`（65 §6.2.1 硬約束 1）。

🔴 **每一個動作都要檢查 `edit_cutoff_at`**：過了編輯截止的 cycle 一律拒絕（`RECURRING_EDIT_CUTOFF_PASSED`）。
⚠️ **`cancel` 是例外**：它在 `charge_cutoff_at` 前都允許，因為「取消整個契約」比「改這一期」的權重高。
#### 6.1.5 🔴 「每期手動確認付款」端點（🔴 `approve_amount` 的語意是「同意超額」**不是**「手動付款」，兩者必須是兩支端點）

| 端點 | 語意 | 何時用 |
|---|---|---|
| `approve_amount`（既有） | **同意本期金額超過 cap** | 金額變動觸發 `ACTION_REQUIRED{AMOUNT_CAP}` |
| **RB-15**（§6.1 表的同一支，🔴 **不是新代號**） `POST /app/recurring_cycles/:cycle_id/charges` | **手動觸發本期扣款** | 自動扣款失敗且買家已換卡／已排除問題 |

🔴 **兩者不可合併**：`approve_amount` 之後**仍走自動扣款**（R-S3）；
RB-15 是**立即扣一次**。把它們寫成同一支，會讓「同意金額」變成「立刻扣款」——
買家按下去的預期完全不同。

**RB-15 的前置**：`status IN ('ACTION_REQUIRED','LOCKED')` 且 `charge_cutoff_at > now`
且 `charge_attempt_count < 3`。⚠️ **它與 R-S3 共用 `charge_attempt_count`**
⇒ 手動扣款也計次，否則使用者可以無限重試而繞過退避。

🔴 **代號與路徑各只有一個：`RB-15`／`POST /app/recurring_cycles/:cycle_id/charges`**
（Buyer 共 18 支 ＝ RB-1～RB-18，**沒有第 19 支**；§7.3.2／§7.6／§13.4 全部引 `RB-15`）。
⚠️ **不得另取代號或另開路徑**（例如 `RB-19` ＋ `…/manual_charges`）。
⚠️ 這種漂移不會被任何 gate 抓到（設計書不進 Swagger 契約檢查），
而照本節施工會多掛一條 gin 路由、照 §6.1 施工會少一條，兩邊都「照文件做」。
### 6.2 Seller（**11 支** ＝ 需求書 9 ＋ 補 2）

全部 `/app/*` 走 `sellerHandle(...)`；身分鍵 ＝ `sellerid` resolver 的 `store_id`。

| 代號 | Method｜Path | capability〔既有常數〕 | 用途 |
|---|---|---|---|
| RS-1～4 | POST／GET `/app/seller_recurring_plans`、GET `…/:plan_id`、POST `…/:plan_id/versions` | `authz.CapProductManage` | 方案建立／清單／詳情（含全部版本）／新版本草稿（published 版不可改） |
| RS-5 | POST `…/:plan_id/versions/:version_no/publications` | `authz.CapProductManage` | **發布**（前置驗證十項，§6.2.1） |
| RS-6／7 | POST `…/:plan_id/pauses`、`…/retirements` | `authz.CapProductManage` | **暫停新訂**（既有契約不受影響）／**下架**（🔴 既有契約**繼續跑到期末**） |
| RS-8／9 | GET `/app/seller_recurring_cycles`、`…/exceptions` | `authz.CapOrderFulfill` | 週期日曆／forecast（`?from=&to=&status=`）／例外清單（缺貨／超額／容量不足） |
| RS-10 🆕 | POST `…/:cycle_id/exception_responses` | `authz.CapOrderFulfill` | 🔴 **回覆買家 exception 選擇**（需求書 §20.18 要追蹤買家回覆，需求書無端點） |
| RS-11 🆕 | GET／PUT `/app/seller_recurring_capacity_buckets` | `authz.CapOrderFulfill` | 承載槽（資料在 **69 §5.1.7 `merchant_capacity_bucket`**，本端點只是賣家面出口） |

⚠️ **RS-6／RS-7 必須是兩支**：需求書把 `pause|retire` 寫成一行，而兩者副作用完全不同（暫停可回復；下架不可回復且對既有契約有承諾）。合成一支＝誤按無法還原。
⚠️ **capability 分派判準**：碰**供給定義**（方案／版本／發布／上下架）→ `CapProductManage`；碰**既有契約的履約**（日曆／例外／承載）→ `CapOrderFulfill`。🔴 後者在停權白名單內（`gateway/internal/authz/policy.go` 的 `suspendedWhitelist` 逐字含 `CapOrderFulfill`），前者不在 ⇒ **賣家停權時仍能履約既有訂閱、不能再開新方案**。這是刻意的，寫出來是為了讓下一個人不要「順手統一」。

#### 6.2.1 RS-5 發布前置驗證（十項；任一不過即 `RECURRING_PUBLISH_PRECHECK_FAILED`＋`data.failed[]`）

1／2. `allowed_fulfillment_mode == SELLER_MANAGED`（plan version）／品項可用配送方式**僅含 `3005`**（69 §5.1.9 `shipping_method_configs`）— 否則帳務軸禁令整條失效、買家被導去平台物流而本書沒有那條鏈
3～6. 已完成商家自送 onboarding（L0 以上，69 `merchant_delivery_profile`）／服務區涵蓋方案宣稱範圍（`merchant_service_zone`）／每個時段都有 `merchant_service_window`／承載槽已設定且 > 0（`merchant_capacity_bucket`）— 缺任一：訂得成送不到、時段是假的、無上限＝超賣
7. 🔴 賣家 lead time 已設定且 **`charge_lead_days ≥ lead_time_days`**（69 profile SLA 欄，**值待 65 §0.1.7 P-5**）— 否則錢已扣而賣家來不及備貨（§7.4 R②）
8～9. 每個 SKU `ACTIVE` 且允許長期預留（product `GetStock`〔既有 RPC〕）／賣家開票能力已備妥（invoice `GetSelfIssueAttestation`〔既有 RPC〕）— 否則方案上線即缺貨、出貨開不出票
10. 三個 amount cap 的預設算得出來（＝首期各對應金額）— cap 為 NULL ⇒ 扣款閘門形同不存在

🔴 **第 7 項在 65 P-5 定案前恆為 fail** ⇒ 這是 67 的 fail-closed 形式：publish 一律拒絕，不是「先放行之後補」。

#### 6.2.2 cycle 日曆／forecast／exceptions／capacity buckets

| 端點 | 回什麼 | 注意 |
|---|---|---|
| RS-8 日曆／forecast | `?from=&to=&status=` 的 cycle 清單 | 🔴 **forecast 的未來期數受 `horizon_filled_until` 限制**（R-S1 只生成 56 天視野）⇒ 超出視野的**不存在**，不是「查不到」。回應要能區分這兩者 |
| RS-9 exceptions | 缺貨／超額／容量不足 | 三類的處置不同，**要分類回傳**不是混成一個清單 |
| RS-11 capacity buckets | 承載槽 | 🔴 **資料在 69 §5.1.7 `merchant_capacity_bucket`**（shipping-service），本端點只是賣家面出口 |

⚠️ **日曆查詢的分頁**：一個長期契約可能有數十期 ⇒ 一律分頁，`records` key 走 `response.Page`。
#### 6.2.3 🔴 「回覆買家 exception 選擇」端點

**RS-10** `POST /app/seller_recurring_cycles/:cycle_id/exception_responses`

需求書 §20.18 要求「追蹤買家對例外的回覆」，而**沒有任何端點寫得進那個回覆**
⇒ 這是 65 §3.11 第 ① 問答不出來的形狀（狀態存在、沒有推進者）。

🔴 **RS-10 是賣家端點（`sellerHandle` ＋ `CapOrderFulfill`），所以它寫不到買家的選擇。**
🔴 **RS-10 不得收 `buyerChoice`**——那等於**賣家替買家按同意**，而 §8.7／§7.7 要求替換必須有買家的
明確同意。這與 §6.2.5 第四條是同一個形狀（該節的原話講的是「標記為已完成」的按鈕
⇒ 等於讓賣家自己說了算）：**賣家端只要存在一個能代替對方下結論的入口，那個保證就沒有了**，
差別只在前者代替系統、後者代替買家。本節據此拆成兩半，**兩半各自有落點**：

| 誰寫 | 寫什麼 | 端點 | 🔴 落點（表:欄） |
|---|---|---|---|
| **賣家** | 例外類型 ＋ 賣家提出的等價 SKU | **RS-10** | `recurring_action_audit`（R16，append-only）；**不新增欄、不新增表** |
| **買家**：跳過這一期 | — | **RB-12** `…/:cycle_id/skips` | `recurring_cycle.status` → `SKIPPED` |
| **買家**：同意超額 | — | **RB-14** `…/:cycle_id/amount_approvals` | `recurring_cycle.status` → `LOCKED` |
| **買家**：取消契約 | — | **RB-10** `…/:contract_id/cancellations` | `recurring_contract.status` |
| **買家**：接受替換 | — | 🔴 **事前**逐項同意，不是本期臨時同意 | `recurring_substitution_consent`（§5.1.12） |

| 欄位 | 值域 | 說明 |
|---|---|---|
| `exceptionType` | `OUT_OF_STOCK`／`AMOUNT_EXCEEDED`／`CAPACITY_UNAVAILABLE` | 對應 RS-9 的三類 |
| `proposedSubstituteSkuId` | SKU id（選填，僅 `OUT_OF_STOCK`） | 🔴 **必須在 `recurring_substitution_consent` 已有 `ACTIVE` 同意列**，否則回 `RECURRING_SUBSTITUTION_NOT_CONSENTED`；賣家提一個買家沒同意過的 SKU **一律拒絕**，不是「先收下等買家回覆」 |
| `sellerNote` | text | 進稽核留痕；**不是狀態** |

🔴 **`buyerChoice`／`respondedAt` 兩欄一併刪除**，因為它們**沒有落點也不需要落點**：
- 「買家回覆了」＝ 該期**離開** `ACTION_REQUIRED`（由上表四支買家端點之一推走）——狀態本身就是回覆，
  另存一份 `buyerChoice` 就是第二個真相，而兩者不一致時沒有任何東西會紅。
- 「未回覆逾時」已經有機制：R-S6 讀 `recurring_cycle.action_deadline_at`（§7.0），
  aging 是 `cycle_action_required_age`（§8.2）——**不需要 `respondedAt`**。

⚠️ **買家沒回覆時的預設不得是 `SUBSTITUTE`**——替換商品需要事前明確同意（§8.7）。
🔴 **逾時的落點是 `CANCELLED{NO_RESPONSE}`，不是 `SKIPPED`**（§7.3.2 逐值表、§7.7 同一個值）：
`SKIPPED` 的 allowed-from 只有 `SCHEDULED`，從 `ACTION_REQUIRED` 寫它一律 CAS `rows=0`。
兩者對買家的差別是**退款義務**——`SKIPPED` 恆未扣款，而走到 `ACTION_REQUIRED` 的期可能已經扣了。
#### 6.2.4 🔴 `pause|retire` 兩個端點寫成一行 ⇒ 路徑定案

| 端點 | 路徑 | 語意 | 對既有契約 |
|---|---|---|---|
| **RS-6** | `POST /app/seller_recurring_plans/:plan_id/pauses` | **暫停新訂** | 🔴 **既有契約不受影響，繼續跑** |
| **RS-7** | `POST /app/seller_recurring_plans/:plan_id/retirements` | **下架** | 🔴 **既有契約繼續跑到期末**，但不再產生新契約 |

🔴 **必須是兩支端點，不得合成一支**：需求書把 `pause|retire` 寫成一行，
而**暫停可回復、下架不可回復**，且下架對既有買家有承諾（跑到期末）。
合成一支 ＝ 誤按無法還原。

⚠️ **兩者都不影響已 `LOCKED` 的 cycle**——賣家的供給決定不得回溯影響已鎖單的買家。
#### 6.2.5 Seller 不可讀完整 mandate／買家不必要個資／跨商家契約；不可直接把 cycle 改 `paid`／`delivered`／`reconciled`

🔴 **賣家端的四條 deny-by-default**（§13.2 有對應的 negative 測試）：

| 不可 | 為什麼 | 賣家可以看到的替代 |
|---|---|---|
| 讀完整 `mandate` | 它是買家的長期扣款授權 | 只有「授權有效／已失效」布林 |
| 讀買家不必要個資 | 定期配送只需要收件資訊 | 收件人、電話、地址；**不含生日／性別／email** |
| 讀跨商家契約 | 同一個買家可能與多個賣家有契約 | 只回 `store_id` ＝ 自己的 |
| **直接把 cycle 改成 `PAID`／`MATERIALIZED`／已對帳** | 那些是**系統推進的狀態** | 只能透過既定端點觸發，狀態由 saga 寫 |

⚠️ **第四條最容易被繞過**：賣家端若有任何「標記為已完成」的按鈕，就等於讓賣家自己說了算。
本書**沒有這種端點**，且 §13.2 有 negative 測試釘住它。
### 6.3 Admin（**9 支**；🔴 **全部掛 `admin`，無一支掛 `adminNoAuth`**）

> `adminNoAuth` 在 `gateway/internal/router/routes.go` 是真實存在的第二個 group〔既有〕。本書全掛 `admin`——每一支都讀或寫真實買家的長期扣款授權。

| 代號 | Method｜Path | Admin capability 🆕 | 用途 |
|---|---|---|---|
| RA-1／2／3 | GET `/admin/recurring_contracts`、`…/:contract_id`、`…/:contract_id/impact_previews` | `RECURRING_VIEW` | 清單／詳情（＋事件時間線＋traceId）／**高風險 mutation 影響預覽**（§6.3.2） |
| RA-4／5 | POST `…/:contract_id/cancellation_proposals`、`…/:proposal_id/approvals` | `CANCEL_PROPOSE`／🆕 `CANCEL_APPROVE` | maker 提案（附影響預覽快照 hash）／🔴 **checker 覆核——maker-checker 的 checker 側** |
| RA-6／7 | POST `…/:contract_id/termination_proposals`、`…/:proposal_id/approvals` | `TERMINATE_PROPOSE`／🆕 `TERMINATE_APPROVE` | 終止（含已扣款期的退款處置）提案／覆核 |
| RA-8 | GET `/admin/recurring_cycles` | `RECURRING_VIEW` | 異常週期（`MATERIALIZING` 卡住／`ACTION_REQUIRED` 逾時） |
| RA-9 | POST `/admin/recurring_mandates/:mandate_id/disablements` | `MANDATE_DISABLE` | 停用付款授權（詐欺／爭議） |

**maker-checker 沿用 58 號既有模式（D-20），不另建一套機制**——🔴 **但表要建**：提案本體是 **§5.1.16 `recurring_admin_proposal`**（`invoice_manual_unlocks` 住在 invoice-service 的資料庫，跨 DB 不能共用也不能下 FK）。🔴 **兩道守衛都在 SQL 的 `WHERE`**：① DB CHECK `maker_account_id <> checker_account_id`（仿既有 `chk_invmu_two_person`）② `UPDATE … WHERE proposal_id=$1 AND status='PENDING' AND maker_account_id<>$2`，`rows=0` ⇒ 重查後回 `RECURRING_PROPOSAL_ALREADY_REVIEWED` 或 `RECURRING_SELF_APPROVAL_DENIED`。**應用層的 if 會在下一次重構被抽掉而沒有人發現。**

#### 6.3.1 🔴 **補 approve／reject 端點**（maker-checker 的 checker 側）

需求書只給了 `cancellation_proposals` 與 `termination_proposals`（maker 側），
**沒有 checker 側** ⇒ maker-checker 只做了一半，提案永遠沒有出口。

| 端點 | 語意 | capability |
|---|---|---|
| **RA-5** `POST …/:proposal_id/approvals` | 覆核**通過** | 🆕 `CANCEL_APPROVE` |
| **RA-7** `POST …/:proposal_id/approvals` | 同上（終止軸） | 🆕 `TERMINATE_APPROVE` |
| 駁回 | 同一支端點的 `decision=REJECT` 分支 | 同上 |

🔴 **兩道守衛都在 SQL 的 `WHERE`，不是前端擋**：
① DB CHECK `maker_account_id <> checker_account_id`（仿既有 `chk_invmu_two_person`）
② 覆核時的 `UPDATE … WHERE status='PENDING' AND maker_account_id <> :checker`

⚠️ **駁回也要留痕**（必填理由 ＋ 進稽核 log），否則「提案被誰否決、為什麼」查不到
—— 🔴 **落成 DB CHECK**（`chk_rap_reject_reason`，§5.1.16）：只寫在應用層的話，
下一次重構把那個 if 抽掉時，駁回原因會靜默留空而沒有任何東西會紅。
#### 6.3.2 影響預覽（RA-3）必須回答四件事

`會取消哪幾期(cycleId[])`／`哪幾期已扣款(金額＋tradeNo)`／`哪幾期已建訂單(orderNo，需走訂單取消鏈)`／`需要何種退款(FULL_REFUND｜ALLOWANCE)`。🔴 **跨月不可作廢只能折讓**，分流接 58 號既有 void／allowance 兩條路，**不另立一套**。四件缺一 ⇒ `RECURRING_IMPACT_PREVIEW_INCOMPLETE`，**不得回空清單**（空清單會被讀成「沒有影響」）；覆核當下重算與提案快照不符 ⇒ `RECURRING_IMPACT_PREVIEW_STALE`，拒絕覆核。

#### 6.3.3 讀寫邊界（§13.2 有對應 negative 測試）

🔴 **Seller 不得看到**完整 mandate（卡號末四碼以外一律不回）、買家 email／生日／完整電話、跨商家契約；**不得寫入** cycle 的 `PAID`／`MATERIALIZED`／任何終態。
🔴 **Buyer 不得看到**別人的契約、賣家成本欄、`provider_*` 原生欄位；**不得寫入** `fulfillmentMode`／`shippingMethod`／任何金額欄。

### 6.4 gRPC 介面

**提供 🆕**：`GetRecurringContractInternal`（caller＝order，建單時回填 `orders.origin_ref`／`recurring_contract_id`）、`GetRecurringCycleInternal`（caller＝order／notification，**不回 mandate 任何欄位**）。

**呼叫**

| 對端 | RPC | 狀態 | 備註 |
|---|---|---|---|
| order | `CheckoutPreview`／`CheckoutConfirm`／`CancelOrderInternal` | 既有（`api/proto/order/order.proto`） | MATERIALIZE 走 `CheckoutConfirm`，**不另開建單 RPC** |
| product | `GetStock`／`DeductStock`／`RestoreStock` | 既有（`api/proto/product/product.proto`） | 🔴 **沒有預留（reserve／hold）RPC** ⇒ hold 由 recurring-service 自持，扣減仍在 MATERIALIZE 那一刻（issue #3517 第 5 條） |
| payment | `CreateRecurringMandate`／**`GetRecurringMandate`**／`ChargeWithMandate`／`RevokeRecurringMandate`／`RefundRecurringCharge` | 新增。🔴 **`CreateRecurringMandate` 與 `GetRecurringMandate` 的回應必須含 `expires_at`（string，ISO 8601 UTC）**——它是 `recurring_contract.mandate_expires_at` 唯一的值來源（§7.0 四問第 1／2 格）；漏了它，R-S7 的候選述詞恆為 0 列 | 🔴 **不得沿用 `MembershipBillingService`**（`api/proto/membership/billing.proto`）——它是 PayUni **Period（固定金額）**路徑，`period_subscription_id`／`provider_period_id` 逐字綁定固定週期金額，而本書每期金額會變（§7.8） |
| shipping（69） | `GetMerchantServiceZone`／`GetMerchantServiceWindow`／`GetMerchantCapacityBucket` | 由 **69 §6.3** 提供 | §6.2.1 第 3～6 項與 §7.7 的唯一輸入 |
| invoice（58） | **零呼叫** | — | 🔴 開票走訂單軸既有鏈，不建 obligation |

🔴 每條新跨服務呼叫都要做三件（fail-open 時漏了**完全無聲**）：① 對端 `k8s/external-services/networkpolicy.yaml` ingress 放行 recurring-service ② `k8s/values/recurring-service.yaml` 的 `*_SERVICE_HOST`／`*_SERVICE_GRPC_PORT` ③ 對端 policy 註解若寫「只有 X 會呼叫」，新增呼叫端會讓那句變假 ⇒ 一起改。apply 後要 `rollout restart` 呼叫端（gRPC channel 退避不自癒）。

### 6.5 非同步事件（MQ）〔＝ 65 §6A 的對應章〕

**E1** 一條事件必須同時有 producer 與 consumer，只有 producer 的不列。**E2** payload 只放 ID／版本／快照 hash／狀態，🔴 **不放**卡片 token、`CreditHash`、完整地址、電話。

#### 6.5.1 出向 15 條（exchange `hawkr.recurring`；consumer 全部是 notification-service）

**共同欄（不重複列）**：`event_id`、`occurred_at`(ISO8601 UTC)、`buyer_id`、`store_id`（mandate 類無）。
**冪等鍵**：`contract.*`＝`contract_id`（可重複發生者加 `occurred_at`）；`cycle.*`＝`cycle_id`（`charge_failed` 加 `attempt_no`、`rescheduled` 加 `new_scheduled_for`）；`mandate.*`＝`mandate_id`＋該事件的時間欄。

| # | routing key | payload 額外欄（snake_case） |
|---|---|---|
| 1 | `recurring.contract.activated` | `contract_id, plan_id, plan_version_no, first_cycle_id, first_scheduled_for` |
| 2 | `recurring.contract.paused` | `contract_id, resume_at(nullable), reason` |
| 3 | `recurring.contract.resumed` 🆕 | `contract_id, next_cycle_id` |
| 4 | `recurring.contract.cancelled` | `contract_id, effective, cancelled_by, refund_required` |
| 5 | `recurring.contract.completed` 🆕 | `contract_id, total_cycles` |
| 6 | `recurring.contract.action_required` | `contract_id, cycle_id, reason_code, deadline_at` |
| 7 | `recurring.cycle.locked` | `cycle_id, contract_id, scheduled_for, snapshot_hash, charge_cutoff_at` |
| 8 | `recurring.cycle.charge_failed` | `cycle_id, contract_id, attempt_no, failure_code, next_retry_at(nullable)` |
| 9 | `recurring.cycle.paid` | `cycle_id, contract_id, charged_amount(string), trade_no` |
| 10 | `recurring.cycle.materialized` | `cycle_id, contract_id, order_no` |
| 11 | `recurring.cycle.skipped` | `cycle_id, contract_id, skipped_by` |
| 12 | `recurring.cycle.cancelled` 🆕 | `cycle_id, contract_id, was_charged, refund_required, reason_code` |
| 13 | `recurring.cycle.rescheduled` 🆕 | `cycle_id, contract_id, old_scheduled_for, new_scheduled_for` |
| 14 | `recurring.mandate.revoked` 🆕 | `mandate_id, revoked_by, affected_contract_ids[]` |
| 15 | `recurring.mandate.expiring` 🆕 | `mandate_id, expires_at, affected_contract_ids[]` |

> 🔴 需求書 16 條中 **`cycle.skipped` 有而 `cycle.cancelled` 沒有** ⇒ 兩個終態下游可見性不對稱：買家跳過會收到通知，**系統取消**（扣款失敗／賣家下架／Admin 終止）不會 ⇒ 錢沒扣、貨沒到、也沒人告訴買家。第 12 條補這個缺口，並且是 §13 反向斷言的觀察點。

#### 6.5.2 入向 3 條（recurring-service 消費）

三條皆綁 `hawkr.order`〔既有 exchange〕，失敗處置一律 **retry 3 → DLQ，不得 ACK 丟棄**：
- `recurring.order.completed`（key `order.completed`）— **§7.3.3 跨軸觀察者**：commerce `COMPLETED` → Cycle `COMPLETED`
- `recurring.order.cancelled`（key `order.cancelled`）— 訂單被取消 ⇒ Cycle 連動 `CANCELLED` ＋ 退款判定
- `recurring.order.refund.finalized`（key `order.refund.finalized`）— 退款終態回收，寫 `refund_status`

⚠️ 三條入向的 payload **不是我們寫的** ⇒ 契約測試 fixture 必須從 **producer 原始碼**（order outbox payload struct）解析，**禁止手寫**（走 `mq-wire-contract` 三方對拍）。

#### 6.5.3 `topology.yaml` 條目原文

```yaml
# exchanges: 新增兩列（🔴 .dlx 不是展開產物；不手寫則 plan 乾淨、apply 才 404）
- name: hawkr.recurring
  type: topic
- name: hawkr.recurring.dlx
  type: topic
```

```yaml
# queues: 新增四條（維持字典序；全部 Go 家族 ⇒ retry policy 形狀）
- name: notification.recurring
  retry:
    max_attempts: 3
  bindings:
  - exchange: hawkr.recurring
    key: '#'
  arguments:
    x-dead-letter-exchange: hawkr.recurring.dlx

# 另三條入向 queue 形狀相同，只換 name、bindings[0].exchange 改
# hawkr.order、key 改對應 routing key、DLX 改 hawkr.order.dlx。
```

**還要改一行 Go**：`services/notification-service/internal/worker/consumer.go` 的 `notificationQueueByExchange`（既有 map）加 `"hawkr.recurring": "notification.recurring"`。漏了只被同檔啟動檢查記一行 warn，**服務照常起來**。

#### 6.5.4 資源數量 pin 增減對帳

展開規則（65 §6A.4.2 實讀 `expand.py::expand`）：`retry:` queue（N 條 binding）⇒ exchanges **+1**、queues **+2**、bindings **+1×N ＋1 ＋1×N**（N=1 ⇒ +3）。

**增減量**：`hawkr.recurring` ＋ `.dlx`（宣告）⇒ exchanges +2；四條 retry queue（各 1 binding）⇒ exchanges +4／queues +8／bindings +12。**合計 exchanges +6、queues +8、bindings +12。**

🔴 **本節只寫增減量，不寫 pin 的絕對值。** 絕對值會隨任何一本設計書落地而變，
寫死在這裡必然過期，而過期時 CI 會紅在一個與本書無關的地方。

**本書落地帶來的增減量**：

| pin 欄位 | 增減量 |
|---|---:|
| `exchanges` | **+6** |
| `queues` | **+8** |
| `queues_json_args` | +0 |
| `bindings` | **+12** |
| `sum(len(v) for v in out.values())` | **+26** |

**施工當下的做法**：
1. 先讀 `infra/mq/tools/test_expand.py::test_expanded_resource_counts_are_pinned` 的**現值**
2. 逐格加上上表的增減量
3. `sum` 用兩種算法各算一次並對得上（前四格相加 ＝ 舊 sum ＋ 各格增減量之和）

🔴 **不得只抄結果**（65 §6A.4.2）。⚠️ **注意 `dlq:` 不帶 `queue:` 的條目**——
它會自帶一條 dlq queue（queues +2／bindings +2），最容易在對帳時被漏算。
🔴 **這條 pin 是「多產資源」這個方向唯一的偵測**：`check_unmanaged.py` 拿展開結果當白名單
⇒ 多產完全靜默，terraform 會安靜地把多餘資源 apply 上 broker。

#### 6.5.5 CodeMeta EventType 與 36 號落點

- **新建 `RecurringEventType`**（Go 領域慣例：`Code`＝`Value`＝UPPER_SNAKE，routing key 另寫在 `pkg/mq/exchanges.go`／`events.go`），與 `InvoiceEventType`／`BuyerEventType` 一致。⚠️ **不可照 `OrderEventType` 的點分式 `Value`**（65 §6A.5 已就 `ShippingEventType` 記載同一個坑）。🔴 **落碼與服務同一波**（流程見 §12.2／附錄 C.3），一律 `--all`。
- **36 號要動**：§2.1 業務 Exchange 表的 `hawkr.recurring` 拿掉 🔶 未佈建標記；
  §2.2 具名共用 DLX 表補 `hawkr.recurring.dlx`；§3.1 訂單領域三條 key 的
  **Consumer 欄補 recurring-service**；§3.6n 定期配送領域（本書對應節，已存在）；
  §4.2 Go Producer、§5.2 Go Consumer（3 條入向 ＋ `notification.recurring`）、§6.2 DLQ 對照 ＋4 條。

**部署順序（漏了會靜默失效）**：topology 條目與 pin 進 PR → **CI apply**（禁本機 `terraform apply`）→ 確認 4 條 queue `consumers ≥ 1`（仍為 0 就 `rollout restart`，apply 不會讓已被 skip 的訂閱自癒）→ **才**部署會 publish 的那一版 code。反過來做＝事件發出去沒人收，而且不報錯。

#### 6.5.6 36 號落點

本書的事件全部落在 **36 號的 §3.6n 定期配送領域**。

| 項目 | 值 |
|---|---|
| exchange | `hawkr.recurring` |
| DLX | `hawkr.recurring.dlx` |
| 出向 routing key | **15 條**（見 §6.5.1） |
| 入向 | **不新增 routing key**，綁既有 `hawkr.order`；新增 **3 條 queue** |
| consumer | notification-service（queue 🆕 `notification.recurring`，`'#'` 綁定） |

🔴 **§3.1 訂單領域那三列的 Consumer 欄要補 recurring-service**
（`order.completed`／`order.cancelled`／`order.refund.finalized`）——
**只加 queue 不改 catalog 的 Consumer 欄，下一個人會以為沒有人在消費那三條**。
#### 6.5.7 部署順序

> 🔴 **順序不是建議，是硬依賴。** 跳步的後果寫在每一列。

| # | 步驟 | 跳過的後果 |
|---|---|---|
| 1 | CodeMeta 碼值（附錄 C）＋ `generate.sh --all` ＋ 四份前端拷貝 | 寫入被 `CodeMetaValidator` 擋下；CI drift gate 逐位元對拍會紅 |
| 2 | proto ＋ 生成同步 | Gateway 與服務編譯不過 |
| 3 | DB migration（Go Atlas） | — |
| 4 | 🔴 **MQ topology（`topology.yaml`）＋ CI apply** | **consumer 部署前 queue 必須先存在**，否則服務端 passive declare **靜默跳過該筆訂閱**（不 crash、不進 DLQ、無錯誤） |
| 5 | recurring-service 部署 ＋ NetworkPolicy | apply 後要 `rollout restart` 呼叫端（gRPC channel 退避不自癒） |
| 6 | Gateway 路由 ＋ Swagger 註解 | 前端拿不到端點 |
| 7 | 前端（65A §2.6／§3.1） | — |

⚠️ **第 4 步的 `topology.yaml` 條目數量 pin 要一起對帳**：
新增一個帶 `dlq:` 的 queue ＝ queues +2、bindings +2、exchanges +0，
`infra/mq/tools/test_expand.py` 的 pin 沒跟著改就 CI 紅。
### 6.6 PayUni webhook 接收

**落點**：沿用既有 `payuni-webhook-service` 邊界，**不在 recurring-service 開對外 HTTP**（它沒有對外 ingress）。**驗簽用 raw body**（非反序列化後重組）；`timestamp` 容忍窗 ＋ `nonce` 去重；🔴 **不以來源 IP 當唯一信任**；**立即 2xx、業務非同步**（寫 inbox 後回應）。**去重鍵 `provider + mer_id + trade_no + event_type`**，🔴 必須含 `provider`，否則不同 provider 的同號互相吞掉。
🔴 **扣款的權威結果只認 webhook 或主動查詢，不認同步回應**：同步 timeout ⇒ 該 attempt 進 `UNKNOWN`（§7.5），**不得當成失敗直接重扣**。

### 6.7 錯誤碼與業務碼清單

🔴 **只給名字不給號**：依 65 §6.7.4 由 **65 §6.7.1 連號追加**。本書申請 **37 個**
（＝ 8 ＋ 8 ＋ 9 ＋ 6 ＋ 6，**下面每群括號內的數字加起來就是它**），分五群（群內留空位）：

- **契約方案（8）**：`RECURRING_PLAN_NOT_FOUND`／`_STATE_INVALID`／`_INPUT_INVALID`／`RECURRING_PLAN_VERSION_LOCKED`／`RECURRING_PUBLISH_PRECHECK_FAILED`／`RECURRING_CONTRACT_NOT_FOUND`／`RECURRING_CONTRACT_STATE_INVALID`／`RECURRING_CONTRACT_VERSION_CONFLICT`
- **時間軸週期（8）**：`RECURRING_EDIT_CUTOFF_PASSED`／`RECURRING_CHARGE_CUTOFF_PASSED`／`RECURRING_CYCLE_NOT_FOUND`／`RECURRING_CYCLE_STATE_INVALID`／`RECURRING_CYCLE_ALREADY_MATERIALIZED`／`RECURRING_CAPACITY_UNAVAILABLE`／`RECURRING_CAPACITY_CONFIG_INVALID`／`RECURRING_SCHEDULE_RULE_INVALID`
- **金流與授權（9）**：`RECURRING_MANDATE_REQUIRED`／🆕 `RECURRING_MANDATE_NOT_FOUND`（RB-18 換卡時目標 mandate 不存在或非 `ACTIVE`）／`_IN_USE`／`_DISABLED`／`_PROVIDER_UNAVAILABLE`／`RECURRING_AMOUNT_CAP_EXCEEDED`／`RECURRING_AMOUNT_APPROVAL_STALE`／`RECURRING_CHARGE_IN_PROGRESS`／`RECURRING_CHARGE_RESULT_UNKNOWN`
- **試算履約模式（6）**：`RECURRING_PREVIEW_EXPIRED`／`_MISMATCH`／`_INPUT_INVALID`／🔴 `RECURRING_FULFILLMENT_NOT_ALLOWED`／🔴 `RECURRING_SHIPPING_POLICY_VIOLATION`／`RECURRING_SUBSTITUTION_NOT_CONSENTED`

🔴 **後兩個是需求書已有的名字，必須照抄，不得另創同義碼**（§0.7 逐字）：
`RECURRING_FULFILLMENT_NOT_ALLOWED`（需求書 §20.11）與 `RECURRING_SHIPPING_POLICY_VIOLATION`
（需求書 §20.18 的 ADM queue 名）。⚠️ **不得為同一個語意自創第二個名字**（例如 `_FULFILLMENT_MODE_REJECTED`／`_SHIPPING_METHOD_REJECTED`）
——落碼後會變成**兩個碼、一個語意**，前端拿到哪一個取決於走到哪支 handler。
- **Admin 覆核（6）**：`RECURRING_PROPOSAL_NOT_FOUND`／`_ALREADY_REVIEWED`／`RECURRING_SELF_APPROVAL_DENIED`／`RECURRING_IMPACT_PREVIEW_STALE`／`_INCOMPLETE`／`RECURRING_ADMIN_ACTION_NOT_ALLOWED`

**需登錄 `bizHTTPStatus`**（不登錄一律塌縮成 409；值一律 4xx）：`*_NOT_FOUND` → **404**；`*_NOT_ALLOWED`／`SELF_APPROVAL_DENIED` → **403**；`MANDATE_PROVIDER_UNAVAILABLE`／`CHARGE_RESULT_UNKNOWN` → **424**；`*_INPUT_INVALID`／`SCHEDULE_RULE_INVALID`／`CAPACITY_CONFIG_INVALID` → **400**；`CHARGE_IN_PROGRESS` → **409**；其餘沿用預設 409。

🔴 **號段：本書只引用 65，不自行對帳。**

| 數字 | 值 | SSOT |
|---|---|---|
| 號段 | **`9500~9699`（200 個）** ✅ 已定版 | 65 附錄 C.5.1 |
| 65 佔 | `9501~9575`（實配 **50**） | 65 §6.7.0 |
| 本書起始號 | **自 `9576` 起由 65 §6.7.1 連號追加** | 同上 |
| 擴段門檻 | **最高已配號 > `9669`** ⇒ 改配 `9500~9799` | 65 附錄 C.5.1 |

🔴 **本書不自行對帳號段總量**：三個數字（號段、各本實配、擴段門檻）**只准有一份，在 65 附錄 C.5.1**。
在本書另算一份必然與它漂移，而漂移的那一份不會告訴讀者它漂了。

🔴 **本書唯一該回報給 65 的是自己的語意條數：67 ＝ 37**（＝上面五群括號內數字之和）。
配號時以本節的逐條清單為準扣除。

## 7. 執行時視圖
### 7.0 排程器名冊（**7 支**；判準走 65 §3.9 九）

> 🔴 **PAUSE 到期恢復是純手動**（RB-9）：**不建**自動恢復排程器——自動恢復會在買家已忘記時**無預警扣款**，而省下的只是一次點擊。

| 代號 | 名稱 | 候選述詞（時間錨） | deterministic key | 租約 |
|---|---|---|---|---|
| R-S1 | `RecurringCycleGenerator` | `contract.status='ACTIVE' AND (horizon_filled_until IS NULL OR horizon_filled_until < now + interval '56 days')` 🔴 **56 天＝§0.6 的 8 週**；`IS NULL` 分支不可省（該欄可空且無 DEFAULT，`NULL < x` 為 NULL ⇒ 新契約永遠不進候選） | `contract:{id}:GEN:{horizon_end}` | 5m |
| R-S2 | `RecurringCycleLockScheduler` | `cycle.status='SCHEDULED' AND edit_cutoff_at <= now` | `cycle:{id}:LOCK:v1` | 5m |
| R-S3 🔴 **向外投遞，不得 level-triggered** | `RecurringChargeScheduler` | `cycle.status IN ('LOCKED','CHARGING') AND next_charge_at <= now AND charge_cutoff_at > now AND charge_attempt_count < 3` 🔴 **本地欄；不得跨 DB 讀 payment 的 `attempt_no`**。⚠️ `charge_attempt_count` 是 **0-based**（`DEFAULT 0`），上限 `< 3` ＝ 首次 ＋ 最多 2 次重試，與 §0.6 及 `chk_rca_attempt_no BETWEEN 1 AND 3` 對齊 | `cycle:{id}:CHARGE:{charge_attempt_count}` 🔴 **不用 `attempt_no`**（跨 DB 取不到） | **300s** 🔴 **不得低於 §0.3 的 130s 下界**（60s 同步等待 ＋ 60s 逾時查詢 ＋ 本地 10s） |
| R-S4 🔴 同上（建單不可逆） | `RecurringMaterializeScheduler` | `status='PAID' OR (status='MATERIALIZING' AND lease_expires_at <= now)` | `cycle:{id}:MATERIALIZE:v1` | **120s** |
| R-S5 🆕 | `RecurringContractCompletionScheduler` | `status='ACTIVE' AND total_cycles IS NOT NULL AND completed_cycles >= total_cycles` | `contract:{id}:COMPLETE:v1` | 5m |
| R-S6 🆕 | `RecurringActionRequiredTimeoutScheduler` | `cycle.status='ACTION_REQUIRED' AND action_deadline_at <= now` | `cycle:{id}:AR_TIMEOUT:v1` | 5m |
| R-S7 🆕 | `RecurringMandateExpiryScheduler`（只發通知） | `contract.status='ACTIVE' AND mandate_expires_at <= now + interval '30 days' AND mandate_expires_at IS NOT NULL AND expiring_notified_for IS DISTINCT FROM mandate_expires_at` 🔴 **全部本地欄**（見下方裁決）。⚠️ `IS DISTINCT FROM` 不可寫成 `<>`——任一邊為 NULL 時 `<>` 求值為 NULL ⇒ 從未通知過的契約永遠不進候選。⚠️ 前置的 `mandate_expires_at IS NOT NULL` **對選到的列集合毫無影響**（`NULL <= x` 本來就求值為 NULL），寫在這裡只是把「快照為空的契約不在本述詞範圍內」講明；**它不是那個 fail-open 的修復**，修復是下方的 `recurring_mandate_snapshot_missing` 指標 | `contract:{id}:MANDATE_EXPIRING:{mandate_expires_at}` 🔴 **與去重欄同構**（key 變、述詞就重新成立） | 5m |

> 🔴 **上表述詞用到的欄位必須全部建在 §5 的 DDL**：`recurring_cycle` 的 `next_charge_at`／
> `action_deadline_at`／`charge_attempt_count`；`recurring_contract` 的 `horizon_filled_until`／
> `total_cycles`／`completed_cycles`／`mandate_expires_at`／`expiring_notified_for`／`status_entered_at`。
> ⚠️ `status_entered_at` 不被上表任何述詞使用——它是 §8.2 aging 告警的起算點。
>
> 🔴 **跨 DB 的欄位不得進述詞**：`attempt_no` 住在 payment-service 的 `recurring_charge_attempt`
> （獨立資料庫）⇒ 本服務的述詞與 deterministic key **一律用本地的 `charge_attempt_count`**。
> R-S7 同型：`mandate` 的 `status`／`expires_at` 住在 payment-service、去重欄在 recurring-service
> ⇒ 單一 SQL 述詞求不出來 ⇒ **在 `recurring_contract` 存 `mandate_expires_at` 本地快照**，
> R-S7 只讀本地兩欄。

🔴 **本快照逐格通過 65 §3.11 四問**：

| 四問 | 答案（要到 `檔案:符號` 等級，答不出＝死欄） |
|---|---|
| **誰推進來** | **RB-4 建約 saga**。`payment.CreateRecurringMandate` 的回應**必須含 `expires_at`**（§6.4 已補進契約），建約同 tx 連同 `payment_mandate_ref` 一起落。回應缺該欄 ⇒ **建約失敗**，不得寫 NULL 放行。⚠️ 這是**應用層口頭約定**：`recurring_contract.mandate_expires_at` 是 nullable（`ACTIVE` 契約也可能是 NULL），DB 兩側都不守 ⇒ 必須靠 `recurring_mandate_snapshot_missing` 指標偵測，見下方 |
| **誰刷新** | **RB-18 換卡端點**（§6.1 新增；RB-18 是 Buyer 端點，§6.2 是 Seller）。換卡是使用者主動的同步操作 ⇒ 在同一次 RPC 回應裡拿到新 mandate 的 `expires_at`，與 `payment_mandate_ref` **同 tx 一起換**。⚠️ 🔴 **不得寫成「綁卡／換卡 webhook 更新」**：recurring-service **沒有 webhook ingress**（PayUni 回調落點在 payuni-webhook-service），也沒有 payment→recurring 事件 ⇒ 那條路徑不存在 |
| **推走的 allowed-from** | R-S7 不改 contract 狀態、只寫 `expiring_notified_for` ⇒ 無狀態轉換、無 allowed-from 問題。真正推走契約的是 R-S5 與取消端點，與本欄無關 |
| **卡住誰會知道** | `recurring_mandate_snapshot_expired`（見下段） |

🔴 **已知不覆蓋（誠實標明，不假裝守得住）**：發卡行主動延期（PayUni account updater）
不經過上面兩條路徑 ⇒ 快照停在舊到期日、R-S7 **早發**通知。
**後果只有「通知發早了」，沒有資損**——真正的扣款失敗兜底是 R-S6 的 `ACTION_REQUIRED`，
它讀的是當期扣款結果不是快照。要覆蓋這條路徑必須由 payment 發 mandate 變更事件，
那是獨立的一波，**不在本書範圍**。

⚠️ 不選「去重欄搬到 payment」——那會讓 payment 的表為 recurring 的通知需求加欄。

🔴 **指標必須錨在有鑑別力的東西上**。⚠️ **不得用「快照 `updated_at` 超過 N 天未刷新」當告警**——
`updated_at` 是 `recurring_contract` 的**整列**時間戳，而該列每期都被 saga 動到
⇒ 它永遠是新的、**告警永不觸發**（65 §3.10 的 fail-open：機制自以為看得到，其實看不到）。
**改成兩條直接可觀測的事實**（缺一不可——它們是**兩個不同的 fail-open**）：

| 指標 | 述詞 | 門檻 | 它抓的是 |
|---|---|---|---|
| `recurring_mandate_snapshot_expired` | `count(status='ACTIVE' AND mandate_expires_at < now())` | `> 0` 持續 1h | 快照顯示已過期而契約仍在扣款。不依賴任何「有沒有被刷新」的推論 |
| 🔴 `recurring_mandate_snapshot_missing` | `count(status='ACTIVE' AND mandate_expires_at IS NULL)` | **`> 0` 即 P0** | **快照從來沒被寫進來**。這一格沒有指標的話，R-S7 的述詞看不到它（`NULL <= x` 為 NULL）、上一條也看不到（`NULL < now()` 為 NULL）⇒ 該契約的到期預告**永遠不會發，而且完全無聲** |

⚠️ **第二條才是真正關閉那個 fail-open 的東西**。在述詞裡加 `IS NOT NULL` 看起來像修復，
實際上選到的列集合一模一樣——**它讓文件讀起來像已經處理了，而 NULL 的契約照樣隱形**
（**假的保證會排擠掉真的警覺**）。
>
> ⚠️ 每支排程器都要配一支「候選述詞真的撈得到剛建立的那一列」的整合測試——
> 用 fixture 直接把狀態設成目標值會看不到這個缺口（那正是它躲過前幾輪自驗的原因）。
> ✅ **已處理**：R-S1 述詞已補 `IS NULL OR` 分支且改為 56 天。原註：`horizon_filled_until` 可空且無 DEFAULT，
> PostgreSQL 中 `NULL < x` 為 NULL ⇒ 新契約永遠不進候選。述詞要寫成
> `(horizon_filled_until IS NULL OR horizon_filled_until < now+90d)`，或在建約同 tx 初始化。


指標：每支一條 `recurring_scheduler_claim_total{scheduler,result}` ＋ 各自的業務計數。七支形狀相同（批次讀候選→逐筆副作用）⇒ 一律走 lease。
🔴 **R-S3 的 300s／R-S4 的 120s 不是隨手挑的**：它們經 payment-service 打 PayUni，而 **`PaymentGrpcClient.PAYUNI_BACKED_DEADLINE_SEC = 100`**〔既有常數〕⇒ 租約必須 **> 100s**（65 §0.1.6 不等式 ②）；既有正例 `AutoRefundWorker.CLAIM_LEASE_SEC = 120` 同一推導。**租約掉到 100s 以下＝持有者還在打 PayUni 就被合法搶走 ⇒ 重複扣款。**
🔴 **租約基準逐筆在 claim 當下取**，不得在批次迴圈外算一次（否則不變式退化成「批次總耗時 < 租約」，隨 `batch-size` 一起破）。各配一支測試釘住「租約常數 > 它呼叫的 deadline 常數」，**斷言寫在同時讀得到兩邊的地方**。

### 7.1／7.2 主要場景與每期 saga 九步 ↔ Cycle 狀態映射

**建約**：RB-3 preview → RB-16 綁卡取 mandate → RB-4 建約（`PENDING_MANDATE`→`ACTIVE`）→ R-S1 生成 **56 天（8 週）**視野 cycle。**結束**：期數用盡（R-S5）→ Contract `COMPLETED`；或取消 → `CANCELLED`。
**每期**（🔴 兩層＝必須寫死步驟↔狀態的映射，否則兩者各說各話）：

| # | saga 步驟 | 停在哪個 Cycle 狀態 | 失敗去哪 |
|---|---|---|---|
| 1／2 | 生成週期／過 `edit_cutoff` 凍結快照 | `SCHEDULED` → `LOCKED` | — |
| 3 | 建庫存 hold ＋ 容量 hold | `LOCKED`（**不另設狀態**） | `ACTION_REQUIRED{OUT_OF_STOCK\|CAPACITY}` |
| 4 | 算金額、比三個 cap | `LOCKED` | `ACTION_REQUIRED{AMOUNT_CAP_EXCEEDED}` |
| 5／6 | 扣款 attempt 1、2（最後一次） | `CHARGING` | 未過 `charge_cutoff` ⇒ 留原態、`next_charge_at` 退避；過了仍未成功 ⇒ `CANCELLED{CHARGE_FAILED, was_charged=false}` |
| 7 | 扣款成功 | `PAID` | 結果未知（同步 timeout）⇒ 留原態 ＋ `unknown_since`，走 §7.5 |
| 8 | 建普通訂單（`CheckoutConfirm`） | `MATERIALIZING` → `MATERIALIZED` | 重試耗盡 ⇒ **同一次 claim 內發動退款** → `CANCELLED{was_charged=true}`；退款也失敗 ⇒ **留在 `MATERIALIZING`** ＋ 開 Admin 案件 |
| 9 | 履約與收貨（賣家自送 →69：`930`→`31`→`11`） | `MATERIALIZED` → `COMPLETED` | 訂單被取消 →(`order.cancelled`)→ `CANCELLED` |

🔴 **第 8 步的「錢已扣、訂單建不出來」正是需求書 `MATERIALIZING` 的 S1（無出口）**。本書給兩條出邊：① 退款成功→`CANCELLED`（自動）② 退款也失敗→**停在 `MATERIALIZING`，由 RA-6／RA-7 覆核推走**（人工出邊必須真的存在，否則這格仍是 S1）⇒ `cycle_materializing_age` 是**必要**告警，不是可選。

### 7.2 時序圖
#### 7.2.1 每期 saga（image7 九步）

> 九步的**狀態落點**在 §7.1 的表；本節畫**時序**與**失敗分叉**。

```
R-S1      R-S2       R-S3            payment      product/shipping   order      R-S4
生成       鎖單        扣款             (PayUni)     (庫存/承載)        (建單)      落地
 │          │           │                 │              │              │          │
 ├─ T-56d 產生 cycle（SCHEDULED）         │              │              │          │
 │          │           │                 │              │              │          │
 │      T-3d edit_cutoff │                 │              │              │          │
 │          ├─ CAS → LOCKED ──────────────┼──────────────▶│(hold 承載)   │          │
 │          │           │                 │              │              │          │
 │      T-1d charge_cutoff                │              │              │          │
 │          │           ├─ CAS → CHARGING ┤              │              │          │
 │          │           ├──── ChargeWithMandate ────────▶│              │          │
 │          │           │◀─── 成功 ────────┤              │              │          │
 │          │           ├─ → PAID         │              │              │          │
 │          │           │                 │              │              │          │
 │          │           │                 │              │              │◀─ CAS → MATERIALIZING
 │          │           │                 │              │              ├─ CheckoutConfirm ─▶
 │          │           │                 │              │◀─ DeductStock ┤          │
 │          │           │                 │              │              ├─ → MATERIALIZED
 │          │           │                 │              │              │          │
 │      T 到貨日：履約與收貨（賣家自送 →69：930→31→11）→ COMPLETED           │
```

🔴 **每一步都是 lease claim ＋ 單列 CAS**，不是「讀一批然後慢慢做」。
租約的兩個不等式見 §7.0；**租約基準逐筆在 claim 當下取**，不得在批次迴圈外算一次。
#### 7.2.2 🔴 **image7 步驟 ↔ image8 狀態的映射表**（M-6／[疑-21]：兩層＝必須寫死映射，每個步驟停在哪個狀態）

> 需求書有**兩張圖**：image7 是九個**步驟**、image8 是十個**狀態**。
> 兩者是不同層（動作 vs 狀態）⇒ **必須寫死映射**，否則實作者會把步驟名當狀態值建 enum。

| image7 步驟 | 進行中停在 | 成功後 | 失敗後 |
|---|---|---|---|
| 1 生成 | — | `SCHEDULED` | —（R-S1 下輪再生） |
| 2 買家調整 | `SCHEDULED` | `SCHEDULED` | — |
| 3 鎖單 | `SCHEDULED` | `LOCKED` | 留 `SCHEDULED`，下輪重試 |
| 4 檢查容量 | `LOCKED` | `LOCKED` | `ACTION_REQUIRED{CAPACITY}` |
| 5 檢查庫存 | `LOCKED` | `LOCKED` | `ACTION_REQUIRED{OUT_OF_STOCK}` |
| 6 金額試算與 cap 檢查 | `LOCKED` | `LOCKED` | `ACTION_REQUIRED{AMOUNT_CAP}` |
| **7 扣款** | 🔴 **`CHARGING`** | `PAID` | 🔴 **分三種，不是一種**：① 可重試失敗且未過 `charge_cutoff_at` ⇒ **留 `CHARGING`** ＋ `next_charge_at` 退避 6h ② 過 `charge_cutoff_at` 仍未 captured ⇒ **`CANCELLED{CHARGE_FAILED, was_charged=false}`** ③ 不可重試（卡失效／掛失／mandate 停用）⇒ `ACTION_REQUIRED{MANDATE_INVALID}`；結果未知 ⇒ 留 `CHARGING` ＋ `unknown_since`（§7.5） |
| **8 建單** | 🔴 **`MATERIALIZING`** | `MATERIALIZED` | 退款成功 → `CANCELLED{was_charged=true}`；退款也失敗 → **留 `MATERIALIZING`**，RA-6／RA-7 人工推走 |
| 9 履約與收貨 | `MATERIALIZED` | `COMPLETED` | 訂單被取消 → `CANCELLED` |
| （買家主動） | 任一非終態 | `SKIPPED`／`CANCELLED` | — |

🔴 **Cycle 軸沒有 `ACTION_REQUIRED{CHARGE_FAILED}` 這個組合，不要造一個。**
它與 §7.1 步驟 5／6、§7.3.2 的 `CHARGING` 出邊、§7.4 的 `T-1` 三處**互斥**
⇒ 把過了 `charge_cutoff_at` 的期寫成 `ACTION_REQUIRED` 的話，
而那一格的出邊（RB-14／RB-15／買家改內容）**全部要求 `charge_cutoff_at > now`**
⇒ 該期進得去、出不來，只能等 R-S6 逾時，而逾時的落點又是 `CANCELLED{NO_RESPONSE}`
——買家收到的原因是「你沒回應」，而他從頭到尾沒有可回應的東西。

🔴 **`CHARGING` 與 `MATERIALIZING` 是「進行中」不是「已完成」**——
它們存在的唯一理由是**讓崩潰的那一筆有一個可被搶回的狀態**。
若把步驟 7 直接寫成 `SCHEDULED → PAID`，pod 在打 PayUni 時死掉那一筆就無法分辨
「還沒扣」與「扣了但沒記錄」。
#### 7.2.3 `AUTHORIZE/CHARGE →（captured but saga fails）→ REFUND + RECONCILE` 在 Cycle 狀態機的落點（🔴 圖上無此狀態）

需求書的流程圖畫了「扣款成功但 saga 失敗 ⇒ 退款 ＋ 對帳」這條路徑，
**而狀態圖上沒有對應的狀態** ⇒ 實作者會不知道那條路徑上的 cycle 長什麼樣。

**本書的落點（不新增狀態值，用既有值 ＋ 旗標）**：

| 階段 | Cycle 狀態 | 旗標 | 誰推走 |
|---|---|---|---|
| 扣款成功、建單失敗、退款進行中 | `MATERIALIZING` | `was_charged=true` | R-S4（重試建單）或退款完成 |
| 退款成功 | `CANCELLED` | `was_charged=true` | — （終態） |
| 🔴 **退款也失敗** | **留 `MATERIALIZING`** | `was_charged=true` | **RA-6／RA-7 人工覆核**（§6.3） |
| 對帳 | 不改 cycle 狀態 | — | statement 觀測（**不進撥款鏈**，§0.6） |

🔴 **不新增 `REFUNDING` 這類狀態**：它會讓值域多一格而沒有獨立的出邊集合，
而「退款進行中」本來就是 `MATERIALIZING` 的一個子情況（錢已扣、單未成）。
⚠️ **`was_charged` 是旗標不是狀態**——它決定了 `CANCELLED` 之後要不要對帳，
但不影響狀態機的邊。
### 7.3 狀態機（兩層；D-5：同名值各軸獨立 enum，不共用值域）

#### 7.3.1 Contract 軸（**7 值**）逐值四問

| 值 | 入邊（誰推進來） | 出邊（誰推走） | allowed-from | 卡住誰知道 |
|---|---|---|---|---|
| `PENDING_MANDATE` | RB-4 建約而 mandate 未 `ACTIVE` | 綁卡 webhook→`ACTIVE`；72h 未完成→`CANCELLED` | （初始態） | `contract_pending_mandate_age > 24h` |
| `ACTIVE` | `PENDING_MANDATE`／`PAUSED`(RB-9)／`ACTION_REQUIRED`(處置完成) | RB-8→`PAUSED`；RB-10→`CANCEL_AT_CUTOFF`／`CANCELLED`；R-S5→`COMPLETED`；契約級問題→`ACTION_REQUIRED` | `PENDING_MANDATE`,`PAUSED`,`ACTION_REQUIRED` | `contract_active_no_cycle_age`（56 天視野零 cycle ⇒ R-S1 死了） |
| `PAUSED` | RB-8 | **只有 RB-9（手動）**；RB-10→`CANCELLED` | `ACTIVE`,`ACTION_REQUIRED` | `contract_paused_overdue > 90d`（🔴 無自動恢復 ⇒ 這是唯一兜底） |
| `ACTION_REQUIRED` | 某期 `ACTION_REQUIRED` 且 `reason` 屬**契約級**（mandate 失效／賣家下架／服務區變更） | 🔴 **三條，見 §7.3.4**：① 買家換卡（RB-18）／處置完成→`ACTIVE` ② **cycle 級**逾時（R-S6 讀 `recurring_cycle.action_deadline_at`）⇒ 該期 `CANCELLED`、Contract **回 `ACTIVE`** ③ **契約級**逾時（另一個門檻）⇒ Contract `CANCELLED` | `ACTIVE` | `contract_action_required_age > 48h` |
| `CANCEL_AT_CUTOFF` 🆕補 | RB-10 且 `effective=AT_CUTOFF` | 🔴 **R-S2 在該期 `edit_cutoff` 到期時一併推走**→`CANCELLED` | `ACTIVE` | `contract_cancel_at_cutoff_age > 35d` |
| `CANCELLED` | 上列五值 | **終態** | 上列五值 | — |
| `COMPLETED` 🆕補推進者 | 🔴 **R-S5**（`total_cycles` 用盡） | **終態** | `ACTIVE` | — |

🔴 **兩個值各自的推進者與推走者（D-19 群 D）**：
`CANCEL_AT_CUTOFF` 的入邊 ＝ RB-10 的 `AT_CUTOFF` 分支、出邊 ＝ R-S2
——**若最終決定不做「期末取消」就把這個值整個刪掉**，留一個沒人寫也沒人讀的值比沒有它更糟。
`COMPLETED` 的推進者 ＝ **R-S5**；缺它的症狀是**有限期數訂閱永遠停在 `ACTIVE`**，
買家看到「進行中」而其實已結束，且 mandate 永不釋放。

#### 7.3.2 Cycle 軸（**10 值**）逐值四問

🔴 **`SKIPPED` 與 `CANCELLED` 必須拆兩格**：**退款語意不同**——`SKIPPED` 恆未扣款（買家主動、edit_cutoff 前），`CANCELLED` 可能已扣款且需退款或折讓。擠一格會讓「要不要退錢」變成讀 `reason` 字串才知道，而它沒有約束。

| 值 | 入邊 | 出邊 | allowed-from | 卡住誰知道 |
|---|---|---|---|---|
| `SCHEDULED` | R-S1 | R-S2→`LOCKED`；RB-12→`SKIPPED`；RB-13 改期（留原態）；契約取消→`CANCELLED` | （初始態） | `cycle_scheduled_past_cutoff`（過了 `edit_cutoff_at` 仍在本態 ⇒ R-S2 死了） |
| `LOCKED` | R-S2 | R-S3→`CHARGING`；cap／缺貨→`ACTION_REQUIRED`；契約取消→`CANCELLED` | `SCHEDULED`、🔴 **`ACTION_REQUIRED`**（RB-14 買家同意超額後回到可扣款；漏了它 ⇒ CAS rows=0、該期永久卡住） | `cycle_locked_age > 24h + 1 tick` |
| `CHARGING` | R-S3 claim | 成功→`PAID`；過 `charge_cutoff` 未成功→`CANCELLED`；結果未知→留原態＋`unknown_since` | `LOCKED`,`CHARGING`,`ACTION_REQUIRED` | `cycle_charging_age > 26h`；`recurring_charge_unknown_age > 2h` |
| `ACTION_REQUIRED` | 步驟 3／4 閘門；mandate 失效 | RB-14／RB-15／買家改內容→`LOCKED`；R-S6 逾時→`CANCELLED{NO_RESPONSE}` | `LOCKED`,`CHARGING` | `cycle_action_required_age > deadline` |
| `PAID` | R-S3 成功（或 webhook 補收斂） | R-S4→`MATERIALIZING` | `CHARGING`、🔴 **`ACTION_REQUIRED`**（限 `reason = CHARGE_RESULT_UNKNOWN`：RB-15 手動付款成功後直接進 PAID） | 🔴 `cycle_paid_age > 30m`（錢已扣、單未建） |
| `MATERIALIZING` | R-S4 claim | 成功→`MATERIALIZED`；重試耗盡且退款成功→`CANCELLED`；**退款也失敗→留本態，RA-6／RA-7 推走** | `PAID`,`MATERIALIZING`（租約過期重 claim） | 🔴 `cycle_materializing_age > 1h` |
| `MATERIALIZED` | R-S4 建單成功（同 tx 寫 `recurring_cycle_order`） | `order.completed`→`COMPLETED`；`order.cancelled`→`CANCELLED` | `MATERIALIZING` | `cycle_materialized_age > 30d`（訂單軸兜底） |
| `COMPLETED` | `recurring.order.completed` 消費者 | **終態** | `MATERIALIZED` | — |
| `SKIPPED` | RB-12（**恆未扣款**） | **終態** | `SCHEDULED` | — |
| `CANCELLED` | 見上列各出邊 | **終態**（`refund_status` 是**獨立欄不是狀態**，由 `order.refund.finalized` 或 payment 回應寫） | `SCHEDULED`,`LOCKED`,`CHARGING`,`ACTION_REQUIRED`,`MATERIALIZING`,`MATERIALIZED` | `cycle_cancelled_refund_pending_age > 4h` |

**終態**：Contract＝`CANCELLED`／`COMPLETED`；Cycle＝`COMPLETED`／`SKIPPED`／`CANCELLED`。其餘皆非終態，**每一格在 §8.2 都要有一條 aging**。

#### 7.3.3 兩層連動與跨軸接縫

- **cycle 級問題不自動改 Contract**：只有契約級 `reason`（mandate 失效／賣家下架／服務區變更）才推 Contract 到 `ACTION_REQUIRED`；單期缺貨或超額**只影響那一期**。
- **Contract 取消不回頭改已 `MATERIALIZED` 的 cycle**：那期已是普通訂單，走 `CancelOrderInternal`，Cycle 由 `order.cancelled` 被動轉態。**兩條路徑不得同時寫。**
- **D-13 fail-closed**：賣家關閉自送／服務區變更／容量不足 ⇒ 一律 `ACTION_REQUIRED`，🔴 **不得自動改物流方式**（只允許 `3005`）。
- **Cycle `MATERIALIZED` → commerce 入口**：`CheckoutConfirm` 回 `order_no`，同 tx 寫 `recurring_cycle_order`（`cycle_id`／`order_no` **各自 unique**）。🔴 **語意落差**：錢已扣（應為已付款）而 commerce 的建單入口是 **`PENDING_PAYMENT(0)`** ⇒ 本書要求 `CheckoutConfirm` 帶 `prepaidTradeNo`、**在同一次 claim 內**推到 `PAID(10)`；**不得停在 `PENDING_PAYMENT`**（付款逾時掃描器會把它取消掉）。
- **commerce `COMPLETED` → Cycle `COMPLETED`**：`recurring.order.completed` 消費者。需求書零描述 ⇒ 缺它時 Cycle 永停 `MATERIALIZED`、`total_cycles` 永遠算不滿、Contract 永不 `COMPLETED`（連鎖三格）。
- **出貨 → 開票**：**58 號賣家自行開立軸**，觸發點＝配送單進 `HANDED_OVER(930)`；recurring **不參與**（§7.5）。

#### 7.3.4 兩層連動規則（Cycle `ACTION_REQUIRED` 時 Contract 是否連動；同名值各自獨立 enum，D-5）

🔴 **Contract 與 Cycle 的 `ACTION_REQUIRED` 是兩個獨立 enum 的同名值**（D-5），
值域不共用、不得互相賦值。

| Cycle 發生什麼 | Contract 連動嗎 | 理由 |
|---|---|---|
| 單一期 `ACTION_REQUIRED{OUT_OF_STOCK}` | ❌ **不連動** | 一期缺貨不代表整個契約有問題 |
| 單一期 `ACTION_REQUIRED{AMOUNT_CAP}` | ❌ 不連動 | 同上 |
| 🔴 **連續 N 期以扣款失敗收場** | ✅ **Contract → `ACTION_REQUIRED`** | 授權可能已失效，整個契約需要買家介入 |
| mandate 被停用（RA-9） | ✅ Contract → `ACTION_REQUIRED` | 授權沒了，之後每期都會失敗 |

**N 的值**：本書取 **2**（連續兩期扣款失敗）。
⚠️ **不取 1**：單次失敗可能是暫時性的（餘額不足當天補上）；
**不取 3 以上**：買家會在毫無察覺的情況下連續三期沒收到貨。

🔴 **「以扣款失敗收場」是哪兩個值，寫死在這裡**。
⚠️ 要說的是：Cycle 軸**沒有 `ACTION_REQUIRED{CHARGE_FAILED}` 這個 reason 值**
——**不是**「`CHARGING` 只有三條出邊」。`CHARGING → ACTION_REQUIRED` 是合法邊
（§7.3.2 的 `ACTION_REQUIRED` allowed-from ＝ `LOCKED`,`CHARGING`；§7.1 步驟 7 ③ 就在走它），
只是 reason 只能是 `MANDATE_INVALID` 之類、不能是 `CHARGE_FAILED`：

| 收場 | 何時 | 計不計入 N |
|---|---|---|
| `CANCELLED{CHARGE_FAILED}` | 過 `charge_cutoff_at` 仍未 captured | ✅ |
| `ACTION_REQUIRED{MANDATE_INVALID}` | 重試視窗內就判定**不可重試**（卡片失效／掛失／mandate 停用，§7.5） | ✅ |
| `ACTION_REQUIRED{CHARGE_RESULT_UNKNOWN}` | 結果未知，錢**可能已經扣了** | ❌ **不計**——它不是「授權失效」的證據，計進去會讓 provider 抖動直接停掉契約 |

⇒ 計數述詞讀的是**已收場的 cycle**（`recurring_cycle.status` ＋ `action_required_reason`／取消原因），
不是某個「連續失敗次數」欄。🔴 **不得另開一個 `consecutive_charge_failures` 計數欄**：
那是第二個真相，且它的歸零時機（哪一期成功才算中斷連續）沒有任何東西守得住。

🔴 **Contract 的 `ACTION_REQUIRED` 出邊有三條**（與 §7.3.1 逐值表是同一份，不得只寫一邊）：

| # | 觸發 | Contract 落到 | 那一期落到 |
|---|---|---|---|
| ① | 買家換卡（RB-18）或處置完成 | `ACTIVE` | 由買家端點各自推走 |
| ② | **cycle 級**逾時：R-S6 讀 `recurring_cycle.action_deadline_at` | 🔴 **回 `ACTIVE`**（只掉一期） | `CANCELLED{NO_RESPONSE}` |
| ③ | **契約級**逾時：另一個門檻（見下） | 🔴 **`CANCELLED`（終態）** | 未鎖單的一併 `CANCELLED` |

🔴 **②③ 的門檻是兩個不同的常數，且結果相反**——同一個「逾時」，②只掉一期、③結束整個契約
並釋放 mandate。⚠️ **兩者都要做**：只寫其中一條的話，缺的那條**完全無聲**。
引用時一律指明是 cycle 級還是契約級，不要只寫「逾時」。

⚠️ **③ 的兩件事本書尚未定案，一併標明**（不得塞一個猜的數字）：
- **門檻值待定**——卡在「契約級 `ACTION_REQUIRED` 的合理處理期」尚無業務裁決；
  🔴 **不得沿用 Cycle 的 `action_deadline_at`**（那是單期的處理期，量級不同）。
- **推走者待補**——§7.0 名冊七支裡**沒有**掃契約級逾時的那一支（R-S6 的候選述詞是
  `cycle.status='ACTION_REQUIRED'`，只掃 `recurring_cycle`）。
- **在兩者補齊之前，③ 這條出邊不存在**，該格唯一看得見的東西是
  `contract_action_required_age > 48h` 告警（§8.2）⇒ **這是 fail-open，寫出來是為了不讓它被當成已經做完的事。**
#### 7.3.5 跨軸接縫：Cycle `MATERIALIZED` → commerce 軸的入口狀態（已扣款 ⇒ 語意應落 `PAID(10)`，而 commerce 建單入口是 `PENDING_PAYMENT(0)`）

🔴 **這是本書最容易做錯的一條接縫。**

| | 值 | 問題 |
|---|---|---|
| commerce 軸的**建單入口** | **`PENDING_PAYMENT(0)`** | 一般訂單建立時錢還沒收 |
| 定期配送的**實際狀況** | **錢已經扣了**（步驟 7 在步驟 8 之前） | 停在 `PENDING_PAYMENT` 會讓逾時取消排程把它取消掉（→`CANCELLED(80)`） |

🔴 **`OrderStatus` 的值域只有十一個**（`docs/代碼管理（CodeMeta）/業務狀態碼.md`）：
`PENDING_PAYMENT(0)`／`PAID(10)`／`PENDING_SHIPMENT(20)`／`IN_TRANSIT(30)`／
`WAITING_PICKUP(32)`／`SHIPPING_ABNORMAL(35)`／`DELIVERED(40)`／`COMPLETED(50)`／
`PARTIALLY_REFUNDED(60)`／`FULLY_REFUNDED(70)`／`CANCELLED(80)`。
**沒有 `DRAFT`** ——不要為定期配送新增一個中間狀態。

**規格**：`CheckoutConfirm` 之後**立即以既有的付款成功路徑推進到 `PAID(10)`**，
並帶上該期的 `payment_transaction` 參照。

🔴 **不得讓訂單停在 `PENDING_PAYMENT(0)`**，理由有二：
① 逾時取消排程會在付款效期後把它取消 —— 而錢已經扣了
② 買家在訂單列表看到「待付款」，而他其實已經被扣款

⚠️ **實作面**：`CheckoutConfirm` 的回應要能讓 recurring-service 拿到 `order_no`，
再以該期的扣款結果推進 A 軸。**兩步必須在同一次 claim 內完成**——
中間崩潰 ⇒ `MATERIALIZING` 保留、R-S4 重試（冪等鍵是 `cycle_id`，不會建出第二張訂單）。
#### 7.3.6 跨軸接縫：commerce `COMPLETED` → Cycle `COMPLETED` 的觀察者（需求書零描述）

需求書**完全沒有描述**誰把 Cycle 從 `MATERIALIZED` 推到 `COMPLETED`
⇒ 這是 65 §3.11 第 ② 問答不出來的形狀。

**本書的定案**：recurring-service 消費既有的 `order.completed` 事件
（queue 🆕 `recurring.order.completed`，綁 `hawkr.order`）。

| 項目 | 值 |
|---|---|
| 觸發 | `order.completed`（A 軸 → `COMPLETED(50)`） |
| 對應 | `payload.orderNo` → `recurring_cycle.order_no` |
| 動作 | `MATERIALIZED → COMPLETED` ＋ `contract.completed_cycles += 1` |
| allowed-from | 🔴 **只有 `MATERIALIZED`**（其餘一律忽略並計數 `recurring_cycle_unexpected_completion_total`） |
| 冪等 | `cycle_id` ＋ 目標狀態 |

🔴 **`completed_cycles` 的遞增必須與狀態轉換同一個 tx**——
分開寫會讓 R-S5（契約完成判定）讀到不一致的計數。

⚠️ **找不到對應 cycle 的 `order.completed` 要靜默忽略**（絕大多數訂單不是定期配送），
但**要計數**，否則「對應寫錯」與「本來就不是」分不出來。
#### 7.3.7 終態清單（逐軸明列）

| 軸 | 終態 | 之後還會不會變 |
|---|---|---|
| **Contract** | `COMPLETED`（期數用盡，R-S5）／`CANCELLED` | ❌ 不再變 |
| **Cycle** | `COMPLETED`／`SKIPPED`／`CANCELLED` | ❌ 不再變 |

🔴 **`PAUSED` 不是終態**（可 resume）；**`ACTION_REQUIRED` 不是終態**（有兩條出邊）。

🔴 **終態的 allowed-from 要寫成白名單**：
```sql
-- ✅ 白名單
UPDATE recurring_cycle SET status='COMPLETED'
 WHERE id=$1 AND status='MATERIALIZED';
-- ❌ 黑名單（新增狀態時會靜默放行）
UPDATE recurring_cycle SET status='COMPLETED'
 WHERE id=$1 AND status <> 'CANCELLED';
```

⚠️ **`CANCELLED` 帶 `was_charged` 旗標時仍是終態**，但它**還有對帳義務**（§7.2.3）
——對帳不改狀態，所以不影響「終態」的定義。
### 7.4 時間軸與兩道截止

以配送日 `T`（該期 `scheduled_for`，台北日界換算 UTC 存 `TIMESTAMPTZ`）為基準：

- **`T-3` `edit_cutoff_at`（R-S2）**：截止修改（內容／地址／時段／跳過／改期）；同刻建 hold、算金額、比 cap
- **`T-2`（R-S3）**：首次扣款發動；**`T-2 → T-1`** ＝ 重試視窗（合計 attempt ≤ 2）
- **`T-1` `charge_cutoff_at`（R-S3）**：截止扣款；仍未成功 ⇒ 該期 `CANCELLED{CHARGE_FAILED}`、釋放 hold、**不順延到下一期**
- **`T-1 ~ T`（R-S4 → 69）**：建單、賣家備貨與配送

**hold 壽命**：**公式唯一定義在 §0.4，本節不重述**。理由是 **hold 必須活過 MATERIALIZE**——hold 先失效 ⇒ 錢已扣、承載被別人拿走。
🔴 **引用這條公式時不得只引一半**（只寫 `scheduled_for_at + 12h`、掉了 `GREATEST(…, charge_cutoff_at + 24h)` 那一支）。
掉的那一支正是 INV-10，而 §0.4 的落地形式是**單表 CHECK** `expires_at >= charge_cutoff_at + INTERVAL '24 hours'`
⇒ 只要賣家把 `charge_cutoff` 設在 `scheduled_for − 12h` 之內（`chk_rcy_time_order` 完全允許），
`scheduled_for + 12h < charge_cutoff + 24h`，**該方案的每一筆 hold 都 23514**，
而錯誤發生在 R-S2 的鎖單步驟裡、賣家端零訊號。**引用公式一律連 `GREATEST` 一起引，或乾脆只給指標。**
🔴 **不得另立以 `lock_at` 起算的公式**（例如 `lock_at + 72h`）：lock 發生在 `edit_cutoff_at`（預設 T−3d＝72h）⇒ 那種式子恰好等於 T，**materialize 完成的餘裕是 0**；§0.4 的 T+12h 才涵蓋得到當日出貨與重試。**以 §0.4 為準，本處不重述公式。**

🔴 **不等式**（違反的症狀都是靜默的；每條配一支測試，斷言寫在讀得到兩邊的地方）

| # | 不等式 | 違反的症狀 |
|---|---|---|
| R① | `edit_lead(3d) ≥ charge_lead(1d) + 重試視窗(1d)` | 重試還沒跑完就過了修改截止 ⇒ 買家想換卡也來不及，該期直接掉 |
| R② | `charge_lead(1d) ≥ 賣家 lead time + 配送前置` | 錢扣了但賣家來不及備貨。🔴 **值待 65 §0.1.7 P-5** ⇒ 在那之前 RS-5 publish 一律 fail |
| R③ | `hold.expires_at > 最後一次 MATERIALIZE 重試完成時點` | hold 先過期 ⇒ 已付款那期建單時缺貨 |
| R④ | `R-S3 租約(300s) ＞ §0.3 下界(130s) ＞ PAYUNI_BACKED_DEADLINE_SEC(100s)`；`R-S4 租約(120s) > 100s` | **重複扣款／建單** |
| R⑤ | `R-S3 租約(300s) ≪ 候選窗口(24h)` | lease 退化成永久旗標 ⇒ 一次崩潰該期永久卡死、無告警 |

⚠️ **`CANCEL_PENDING(992)` 是 delivery 軸的狀態，本書不借用。** 定期配送的「取消等待」一律落在
**Cycle 軸**（`ACTION_REQUIRED`／`CANCELLED`），**不得去 delivery 軸借 `992`**。

🔴 **交叉引用（65 已定案，本節處置不變）**：65 §7.1.2.0b 的 **M-3** 已把 `992` 的適用通路
**擴到 `3005`**，但 🔴 **限 L1／L2**（有商家 API／webhook、取消需等對方確認，才是 `992` 的語意）；
**L0 純人工不適用**，同步走 `41`。**該格逐字肯定了本書的處置不變**：定期配送的「取消等待」落的是
**Cycle 軸**，M-3 擴的是 **delivery 軸**，**兩者是不同的軸，不衝突**（各有各的推走者，Cycle 軸見 §7.3.2／§7.3.7）。

🔴 **要借 `992` 的門檻是「先說明定期配送的取消在等誰確認」**，不是「`3005` 適不適用 `992`」
（後者 65 §7.1.2.0b 的 M-3 已經擴了，擋不住任何事）。Cycle 軸的取消等的是
**時間**（`cancel_effective_at`／`charge_cutoff`）或買家補件，不是 delivery 對象的確認回覆
⇒ 與 `992` 的語意不同，**擴適用欄擴不出這一條**。

### 7.5 失效與重試

- **扣款失敗分兩類**：**可重試**（餘額不足、發卡行暫時拒絕）走退避 **6h**（§0.6 的重試最小間隔；3 次 × 6h ＝ 12h < 24h 視窗）的第二次 attempt；**不可重試**（卡片失效、掛失、mandate 被停用）⇒ 立即 `ACTION_REQUIRED{MANDATE_INVALID}` ＋ Contract 連動，**不浪費第二次 attempt**。
- **結果未知收斂**：同步 timeout ⇒ 寫 `unknown_since`、**不重扣**；靠 PayUni webhook ＋ 主動查詢（每 10 分鐘，窗口至 `charge_cutoff`）。到期仍未知 ⇒ `ACTION_REQUIRED{CHARGE_RESULT_UNKNOWN}` ＋ Admin 案件。🔴 **不得自動判失敗再扣一次**——那正是重複扣款。
- **漏跑超過政策容忍時間（本書定案 **`6h`**，§0.6 ＋ §3.7-⑤）**：🔴 **先轉 incident，不無條件 catch-up 扣款**——補跑會在同一分鐘對同一買家發動 N 期扣款，而每期都合法。
  ⚠️ 🔴 **不得寫成 `24h`**。差別不是「保守一點」：`24h` 恰好等於
  `retry_window`，於是「漏跑」與「正常重試」在時間上**不可分辨**（§3.7-⑤ 的推導逐字要求
  容忍時間 < `retry_window` 的四分之一）⇒ 漏跑 20 小時的批次會被當成正常重試直接扣下去。
  **速查章（§0.6）是可被實作直接抄走的數值，本節不另立。**
- **開票**：🔴 **時點＝出貨時**（D-8；需求書 §20.6 那句「到貨後」本書回灌修正）。recurring **不觸發任何開票動作**：訂單軸在配送單進 `HANDED_OVER(930)` 時走 58 號既有賣家自行開立鏈（`GetSellerInvoiceJob`／`ReportExternalIssuance`〔既有 RPC〕）。⚠️ **U-3 待會計師確認前不得改動 58 號既有觸發點。**
- **跨月更正**：🔴 **跨月不可作廢只能折讓**，走 58 號既有 void／allowance 分流（§6.3.2 的 `FULL_REFUND`／`ALLOWANCE` 即此分流的輸入）。

### 7.6 並發與順序

一個 cycle **同時最多一個非終態 attempt**：`unique(cycle_id, attempt_no)` ＋ partial unique index on `status IN ('PENDING','SENT')`。
所有轉態走 **CAS**：`UPDATE … SET status=$new, version=version+1 WHERE id=$1 AND status IN (<allowed-from>) AND version=$2`。**`rows=0` 逐表指定**：`recurring_cycle`／`recurring_contract` 是主體（錯了會讓錢或憑證出錯）⇒ **rollback ＋ 回業務碼**；`recurring_action_audit` ⇒ 記 log 放行。
claim 走 `FOR UPDATE SKIP LOCKED` ＋ `lease_owner`／`lease_expires_at`；多來源更新（RB-14／RB-15 vs R-S3）由 allowed-from 收斂，端點回 `RECURRING_CHARGE_IN_PROGRESS`（409），**不排隊等待**。

### 7.7 庫存、承載、缺貨與替代品

三步預設順序（§7.1 步驟 3）：**① 檢查容量槽（69 `merchant_capacity_bucket`）→ ② 建庫存 hold → ③ 建容量 hold**。任一步不成立 ⇒ `ACTION_REQUIRED`，**不自動降級、不自動減量**。
🔴 **沒有明確同意就不替換**：替代品必須是**方案版本裡明確列名的等價 SKU**，且買家**事前**逐項同意
（表與欄位以 **§5.1.12 `recurring_substitution_consent`** 為唯一權威，本節不重述）。
**沉默不是同意**；未同意到 `charge_cutoff` ⇒ `CANCELLED{NO_RESPONSE}` ＋ `RECURRING_SUBSTITUTION_NOT_CONSENTED`。
⚠️ **同意是契約級、事前的，不是「該次替換」的臨時同意**（需求書 §20.16 逐字「**事前**對明確列名的等價 SKU 逐項同意」，
§5.1.12 的 `uq_rsc_active` 也是 `(contract_id, original_sku_id, substitute_sku_id)`）。
🔴 **不得改用 `cycle_id`／`from_sku_id`／`to_sku_id` 這組欄名**——它們在 §5.1.12 不存在，
照那組施工會建出第二張表，而兩張表的同意記錄不一致時**沒有任何東西會紅**。

### 7.8 價格政策與漲價

三種 `price_policy`（🔴 **值域以 §5.1.1 的 `chk_rpv_price_policy` 為準**）：
`FIXED_PER_CYCLE`（每期固定，等同「合約鎖價」）／`CURRENT_PRICE_MINUS_DISCOUNT`（跟隨當期價再折扣）／
`FIXED_TERM_PREPAID`（整期預付，🔴 遞延收入與未履約退款設計完成前**不得開啟**）。
⚠️ 🔴 **不得改用 `LOCK_AT_CONTRACT`／`FOLLOW_CURRENT`／`LOCK_WITH_TOLERANCE` 這組名字**——
它們與 CHECK **完全不重疊** ⇒ 一律 23514。
**「漲價容忍」不是第四個 policy 值**，它是三個 amount cap（商品／物流／當期總額），**預設 0**。超過三個 cap（商品／物流／當期總額）任一 ⇒ `ACTION_REQUIRED{AMOUNT_CAP_EXCEEDED}` ＋ 事件 6，須 RB-14 才放行。🔴 **cap 預設值 ＝ 首期各對應金額（容忍 0）**，不是 NULL、不是無限大；三欄 **NOT NULL ＋ CHECK ≥ 0**。**沉默不等於同意**：不回應到 `charge_cutoff` ⇒ 該期 `CANCELLED`、**不扣款**。

### 7.9 靜默陷阱表

> 🔴 **本表的共同特徵：測試綠、CI 綠、部署綠，而那件事從未發生過。**

| # | 陷阱 | 症狀 | 防線 |
|---|---|---|---|
| 1 | **排程器候選述詞的 allowed-from 漏了起點** | `rows=0`，呼叫端讀成「別人先做過了」 | §13 的整合測試**必須從真正的入口走起**（fixture 直接設狀態看不到） |
| 2 | **topology 漏登記** | 服務端 passive declare ⇒ **靜默跳過該筆訂閱**，不 crash、不進 DLQ | `scripts/mq-consumer-registration-check.py`；⚠️ 只掃 Go |
| 3 | **NetworkPolicy 漏放行 ＋ 呼叫端 fail-open** | 測試綠、smoke 綠，那條檢查從未執行過 | §6.4 的三件；驗證方式是**造一個應該被拒絕的情境** |
| 4 | **`horizon_filled_until` 為 NULL** | `NULL < x` 為 NULL ⇒ **新契約永遠不進 R-S1 候選** | 述詞的 `IS NULL OR` 分支 |
| 5 | **`mandate_expires_at` 為 NULL** | R-S7 與過期指標**都看不到** | `recurring_mandate_snapshot_missing` 指標（§7.0） |
| 6 | **`expiring_notified_for` 寫成「發過沒」** | 換卡後永遠不再通知 | 改記「為哪一個到期日發過」＋`IS DISTINCT FROM` |
| 7 | **hold 的 `expires_at` 算錯** | 扣款成功時承載已被別人拿走 | `chk_rih_lifetime`／`chk_rch_lifetime`（單表 CHECK，§0.4） |
| 8 | **cap 為 NULL** | 扣款閘門形同不存在 | RS-5 發布前置第 10 項 |
| 9 | **CodeMeta 只跑 `--java`** | CI drift gate 跑 `--all` 逐位元對拍 ⇒ 紅在無關的地方 | 一律 `--all` |
| 10 | **`topology.yaml` 數量 pin 沒更新** | `test_expand.py` 紅；而**多產是完全靜默的**（白名單變寬） | 新增帶 `dlq:` 的 queue ＝ queues +2、bindings +2 |

## 8. 跨切面關注點
### 8.1 安全與授權

- **`allowedActions[]` 是買賣雙方的唯一出口**：前端不得自行依狀態推導可用動作（RB-6／RB-11／RS-8 一律回傳）。缺它 ⇒ 前端用自己那份規則，必然漂移。
- **Admin capability 十個 🆕**：`RECURRING_VIEW`／`RECURRING_OPERATE`／`PAYMENT_REVIEW`／`REFUND_PROPOSE`／`REFUND_APPROVE`／`MANDATE_DISABLE`／`CANCEL_PROPOSE`／`TERMINATE_PROPOSE` ＋ `CANCEL_APPROVE`／`TERMINATE_APPROVE`（maker-checker 的 checker 側）。
- **Admin 子角色落點（D-20）**：🔴 **不能塞進 `account_roles`**（那是登入 principal 的角色，不是 ADM 內部職務分工）⇒ 依賴 65 的 Admin RBAC 章；在那之前十個 capability **全部 fail-closed**（未授予即拒）。
- **46 號賣家能力層**：所有 `/app/seller_recurring_*` 一律 `sellerHandle(..., cap, ...)`，**禁止裸 `JWTAuth()` 承載賣家專屬功能**；本書**不申請任何豁免**。

#### 8.1.1 Buyer／Seller 的存取邊界（`allowedActions[]` 是唯一出口）

🔴 **`allowedActions[]` 是唯一出口**：前端不得自行判斷「這一期能不能跳過／改期／取消」。

| 角色 | 讀 | 寫 |
|---|---|---|
| **Buyer** | 自己的契約與 cycle；**不含賣家成本結構** | 建約、調整未鎖單期、跳過、改期、取消、換卡、同意超額 |
| **Seller** | 自己方案下的契約與 cycle（§6.2.5 四條 deny）；**不含完整 mandate、不含買家不必要個資、不含跨商家契約** | 方案／版本／發布／上下架、回覆例外、承載槽 |
| **Admin** | 全部 | 只透過 maker-checker（§6.3） |

⚠️ **同一個 cycle 對三個角色回的 `allowedActions[]` 不同** ⇒ 它必須由後端依身分計算，
**不得回一份共用的清單讓前端過濾**。
#### 8.1.2 Admin capability 六個（`RECURRING_VIEW`／`RECURRING_OPERATE`／`PAYMENT_REVIEW`／`REFUND_PROPOSE`／`REFUND_APPROVE`／`MANDATE_DISABLE`）

| capability | 用在哪 | 為什麼要獨立一個 |
|---|---|---|
| `RECURRING_VIEW` | RA-1／2／8 唯讀 | 客服要看得到，但不該能動 |
| `RECURRING_OPERATE` | RA-3 影響預覽 | 預覽會揭露金額，比純讀敏感 |
| `PAYMENT_REVIEW` | 查扣款明細 | 金流資料 |
| `REFUND_PROPOSE` | 提退款案 | maker |
| `REFUND_APPROVE` | 覆核退款 | checker，**必須與 maker 不同人** |
| `MANDATE_DISABLE` | RA-9 停用授權 | 🔴 **它直接讓買家之後每期都失敗**，權重最高 |

🔴 **六個都是新增的 capability**。⚠️ 它們**不能塞進 `account_roles`**（見 §8.1.4）。

🔴 **本表只涵蓋十個中的六個，照本表施工會少掛四個**：
另外四個是 `CANCEL_PROPOSE`／`TERMINATE_PROPOSE`（maker 側）
＋ `CANCEL_APPROVE`／`TERMINATE_APPROVE`（checker 側）。
⇒ **6 ＋ 2（§8.1.3）＋ 2（僅出現在 §8.1 的 bullet）＝ 10**，與 §8.1 的「十個」相符。
⚠️ **`CANCEL_PROPOSE`／`TERMINATE_PROPOSE` 目前只活在 §8.1 的一行 bullet 裡**
——本書所有「逐項枚舉」的節都看不到它們，而 §8.1 又逐字要求十個**全部 fail-closed（未授予即拒）**
⇒ 漏掛的症狀是**那兩支 propose 端點永遠回拒絕**，而不是「沒有權限檢查」。
#### 8.1.3 🔴 **補 approve 類 capability**（`CANCEL_APPROVE`／`TERMINATE_APPROVE`，需求書漏）

需求書列了 `CANCEL_PROPOSE`／`TERMINATE_PROPOSE`（maker 側），
**沒有 checker 側的 capability** ⇒ 覆核端點無權限可掛。

| 🆕 補的 capability | 掛在 | 硬條件 |
|---|---|---|
| `CANCEL_APPROVE` | RA-5 | 🔴 **持有者不得同時持有 `CANCEL_PROPOSE`** —— 否則一個人可以自提自核 |
| `TERMINATE_APPROVE` | RA-7 | 同上 |

🔴 **「不得同時持有」由授權配置守，而「不得同一人」由 DB CHECK 守**——兩層都要，
因為前者可能被錯誤配置，後者是最後一道（`maker_account_id <> checker_account_id`）。
#### 8.1.4 Admin 子角色落點（D-20：**不能塞進 `account_roles`**，需要 admin 側獨立 RBAC ⇒ 依賴 65 的 RBAC 章）

🔴 **不能塞進 `account_roles`**（D-20）：那張表是**買賣家**的角色，
而這六個是 **admin 側的子角色**。混進去會讓「賣家角色」的值域被 admin 概念污染。

**落點**：依賴 65 的 RBAC 章。⚠️ **而 65 §8.4 已明載 `requireAdmin()` 無參數
⇒ admin 端目前沒有 capability 概念**（既有實例：某服務 30+ 支 admin 端點唯一授權就是 `accountType == 4`）。

⇒ **本書的 fail-closed**：在 admin 側 RBAC 落地前，
RA-4～RA-7（提案與覆核）與 RA-9（停用授權）**一律不啟用**，
只開 RA-1／2／8 唯讀。**不得先用 `accountType == 4` 放行**——
那等於讓任何 admin 都能停掉買家的扣款授權。
### 8.2 可觀測性

**一個指標族** `recurring_state_age_seconds{axis,state}`（`axis ∈ {contract,cycle}`）；**門檻不是 SSOT，`entered_at` 才是**。🔴 **每個非終態各一條 aging**（Contract 5 格、Cycle 7 格，逐格門檻見 §7.3「卡住誰知道」欄），其中五條是：`contract_pending_mandate_age`／`contract_action_required_age`／`cycle_action_required_age`／`contract_paused_overdue`／`cycle_materializing_age`。
🔴 **另有兩條不是 aging 而是快照健康度**（§7.0 的 mandate 裁決要求，不屬於上面的指標族）：`recurring_mandate_snapshot_expired`（`ACTIVE ∧ mandate_expires_at < now()`，> 0 持續 1h）／`recurring_mandate_snapshot_missing`（`ACTIVE ∧ mandate_expires_at IS NULL`，**> 0 即 P0**）。
**每條告警回答四問**（誰推進來／誰推走／推走那一句的 allowed-from 含不含這格／卡住誰知道）。🔴 **述詞錨在時間不錨在狀態集合**：用 `now() - entered_at > 門檻`，不要再疊會讓列「剛好離開述詞」的條件——那會讓告警在最需要時熄滅（`InvoiceOutboxLag` 同一個坑）。

**零容忍指標（恆為 0，非 0 即 P0）**：`recurring_duplicate_charge_total`（同 cycle 兩筆成功扣款）／`recurring_cycle_materialization_duplicate_total`（同 cycle 兩個 `order_no`）／`recurring_capacity_overbook_total`／🔴 `recurring_settlement_write_total`（**對任何結算／撥款／帳本表的寫入次數——帳務軸禁令的執行面偵測**）。

#### 8.2.1 需求書 18 條指標

需求書 §20 列了 18 條指標。本書逐條落點：

| 類別 | 條數 | 落點 |
|---|---|---|
| 契約生命週期（建立／暫停／取消率） | 4 | `recurring_contract_transition_total{from,to}` |
| 週期履約（生成／鎖單／扣款／建單成功率） | 5 | `recurring_cycle_transition_total{from,to}` |
| 扣款（成功率、重試分布、失敗原因） | 4 | `recurring_charge_total{result,reason}` |
| 排程器 | 3 | `recurring_scheduler_claim_total{scheduler,result}`＋`_last_success_timestamp_seconds` |
| 承載與庫存 | 2 | hold 建立／釋放計數 |

🔴 **另有兩條不在需求書 18 條內、但缺了會出事**（§7.0 的 mandate 裁決）：
`recurring_mandate_snapshot_expired`／`recurring_mandate_snapshot_missing`（見 §8.2.2）。
#### 8.2.2 🔴 **補五條缺的 aging** ＋ **兩條 mandate 快照健康度**

- aging（五）：`contract_pending_mandate_age`／`contract_action_required_age`／`cycle_action_required_age`／`contract_paused_overdue`／`cycle_materializing_age`
- 快照健康度（二，見 §7.0 mandate 裁決）：`recurring_mandate_snapshot_expired`／🔴 `recurring_mandate_snapshot_missing`
- 🔴 **治理表（二，見 §5.1.16）**：`recurring_proposal_pending_age`（`recurring_admin_proposal` 停在 `PENDING`；
  🔴 **這張表沒有排程器推走，唯一出口是人** ⇒ 沒有這條就是提案掛著沒人批而完全無聲）／
  `recurring_mandate_consent_orphan_total`（`contract_id IS NULL` 且建立超過 1h ＝ 建約 tx 曾經失敗）。
  ⚠️ **它們不在上面那個 `recurring_state_age_seconds{axis,state}` 指標族裡**——那族是狀態機的格子
  （Contract 5 格、Cycle 7 格），而這兩條量的是治理表的積壓，`axis` 對不上，硬塞進去會讓那族的
  「每個非終態各一條」自我檢查對不起來。
#### 8.2.3 每條告警回答四問（述詞**錨在時間**不錨在狀態集合）

🔴 **述詞錨在時間，不錨在狀態集合。**

```sql
-- ✅ 錨在時間
now() - status_entered_at > interval '24 hours'
-- ❌ 錨在狀態集合（列「卡死」時剛好離開述詞 ⇒ 告警在最需要時熄滅）
status = 'PENDING' AND created_at < now() - interval '24 hours'
```

**每條告警必答四問**：

| 問 | 對本書告警的統一答案 |
|---|---|
| ① 誰推進來 | §7.3 逐值表的「前驅」欄 |
| ② 誰推走 | 同表「推進者」欄；答案是「人工」時**那個端點必須真的存在**（§6.3） |
| ③ allowed-from 含不含它 | §7.3.7 的白名單寫法 |
| ④ **它在什麼情況下不會響** | 🔴 **這一格每條都要單獨寫**——見下 |

🔴 **「不會響」的三種常見來源**：① 述詞疊了會讓列離開的條件（上面那個反例）
② `sql_exporter` 本身掛了（靠 `_last_success_timestamp_seconds` 兜底）
③ 指標的 label 基數爆掉被丟棄（`reason` 這類自由字串要先收斂成 enum）
#### 8.2.4 零容忍指標（`duplicate_charge_total`／`cycle_materialization_duplicate_total`／`capacity_overbook_total`／`reconciliation_imbalance_total` 皆須為 0）

| 指標 | 述詞 | 非 0 代表什麼 |
|---|---|---|
| `recurring_duplicate_charge_total` | 同一 cycle 兩筆成功扣款 | 🔴 **重複扣款**——租約或冪等鍵壞了 |
| `recurring_cycle_materialization_duplicate_total` | 同一 cycle 兩個 `order_no` | 建單冪等壞了 |
| `recurring_capacity_overbook_total` | 承載超賣 | hold 或 CAS 壞了 |
| `reconciliation_imbalance_total` | 對帳不平 | 金額算式或快照壞了 |
| 🔴 `recurring_settlement_write_total` | **對任何結算／撥款／帳本表的寫入次數** | **帳務軸禁令被違反**（§2.3 明文禁止本書出現 payout；§5.1.14 是它的 schema 側機械檢查） |

🔴 **全部恆為 0，非 0 即 P0。**
⚠️ 最後一條是**架構不變式的執行面偵測**——它不是效能指標，
它在回答「有沒有人把撥款鏈接進定期配送」。
### 8.3 錯誤處理與降級

- **payment（PayUni）→ fail-closed**：不扣款、留 `CHARGING`、走 `UNKNOWN` 收斂（開放的代價是重複扣款）。
- **product `GetStock` → fail-closed**：hold 建不出來 ⇒ `ACTION_REQUIRED`（開放＝超賣真實庫存）。
- **shipping（69 服務區／容量）→ fail-closed**：publish 拒絕、cycle 進 `ACTION_REQUIRED`（D-13；開放＝送不到）。
- **notification → fail-open**：事件進 DLQ、業務照走，DLQ 深度要有告警。**invoice** 不適用。

### 8.4 一致性與冪等

**deterministic key 管「同一件事只做一次」**（§7.0 key 欄）、**DB unique 管「同一列只存在一份」**（`unique(cycle_id, attempt_no)`／`recurring_cycle_order` 雙 unique），缺任一都不夠。🔴 **跨服務寫入一律 outbox**：狀態轉換與 `recurring_outbox` 寫入必須**同一交易**（65 §3.9 九例外 A）；**不得用同步鏈式 RPC 假裝原子交易**——recurring／order／payment 是三個獨立資料庫。入向 3 條以 `(routing_key, event_id)` 進 inbox 去重，**去重在業務處理之前**。

### 8.5 設定與機密

- mandate 的 provider reference 走 **envelope encryption**（KMS key versioning，存 `key_version`）；🔴 **DB 不存 PAN／CVV／`CreditHash` 明文**。
- 🔴 **PayUni 幕後授權要綁定來源 IP**（官方文件逐字：「首次交易約定後，後續可幕後授權。需申請審核開通並綁定 IP」）⇒ 出口必須走 **egress-proxy**（固定 IP `203.0.113.30`），在 `serverless/egress-proxy/nginx.conf` 加**精確 location** 白名單，**不得為它另開 proxy**；呼叫端必帶斷路器（仿 `pkg/sms`）。
- **首次授權的同意證據**：`recurring_mandate_consent` 一列一次，存 `consent_hash`、顯示給買家的**三個 amount cap**、**收款主體（`store_id` ＋店名快照）**、頻率、取消方式文案版本、`ip`／`user_agent`／`device_id`、provider 結果、`consented_at`。🔴 **變更收款主體或任一 cap ⇒ 必須重新同意**（舊 mandate 轉 `SUPERSEDED`），**不得就地 UPDATE**。
- **優雅關閉**：SIGTERM 後**先停 claim 新候選、再等已 claim 的 attempt 走完（上限＝租約 **300s**（R-S3））**，`terminationGracePeriodSeconds ≥ 310`（＝R-S3 租約 300s ＋ 10s 餘裕）；🔴 小於租約 ⇒ pod 被砍時 attempt 沒收斂而租約還在，那筆要等租約過期才有人接手。
- k8s Secret 的 **key 名與值來源必須進 git（值不進）**：`RECURRING_MANDATE_KMS_KEY_ID`／`RECURRING_PAYUNI_MER_KEY`／`EGRESS_PROXY_URL`；env 五處同步（`.env.template`／`docker-compose-local.yml`／`k8s/values/`／deployment／實際讀值驗證）。

### 8.6 PII 與遮罩

🔴 log／Sentry／analytics／CSV 不得出現：卡片 token、`CreditHash`、完整卡號、CVV、完整地址、完整電話。遮罩清單是**值的清單不是欄位位置的清單**：同一個值從 body／query／path param／header 進來都要遮（`middleware/logger.go` 的 `sensitiveKeys` 與 `redactQuery` **兩處都要加**）。🔴 **`mandate_id` 本身視為憑證**（單獨持有即可發動 RB-17）⇒ 一律遮。

### 8.7 首次授權的同意證據（consent hash、顯示金額上限、收款主體、頻率、取消方式、IP／device evidence、provider result；**變更收款主體或上限需重新同意**）

> 🔴 **同意證據是法遵資產，不是 UX 細節。** 爭議時它是唯一能證明「買家同意過什麼」的東西。

**建約時必須落庫的欄位**（缺一即不得建約）：

| 欄位 | 內容 | 為什麼 |
|---|---|---|
| `terms_hash` | SHA-256 of 條款全文 | 條款改版後能證明當時同意的是哪一版 |
| **顯示金額上限** | 買家看到的三個 cap | 🔴 **是「顯示的」不是「後端算的」**——兩者若不同，以顯示的為準 |
| **收款主體** | 賣家名稱與統編 | `SELLER_DIRECT` ⇒ 錢進賣家帳戶，買家要知道 |
| 頻率 | 每 N 天／週 | |
| 取消方式 | 文字 | 法規要求 |
| IP／device evidence | 建約當下 | |
| provider result | mandate 建立的回應 | |

🔴 **變更收款主體或任一 cap ⇒ 必須重新同意**（新的 `consent_evidence_id`）。
⚠️ **換卡（RB-18）不算變更收款主體** —— 收款主體是賣家，不是買家的卡。
兩者容易混淆，實作時判準是「**錢進誰的帳戶**」有沒有變。

⚠️ **`consent_evidence_id` 是 `NOT NULL`** ⇒ 沒有同意證據就建不出契約，這是 DB 層的保證。
### 8.8 靜默陷阱表

| # | 陷阱 | 症狀 |
|---|---|---|
| 1 | `allowedActions[]` 回一份共用清單讓前端過濾 | 賣家看得到買家才有的動作 |
| 2 | admin capability 用 `accountType == 4` 放行 | 任何 admin 都能停掉買家的扣款授權 |
| 3 | 告警述詞疊了狀態條件 | 列卡死時剛好離開述詞 ⇒ **在最需要時熄滅** |
| 4 | `reason` 這類自由字串直接當 metric label | 基數爆掉 ⇒ 整條 series 被丟棄，**而不會有錯誤** |
| 5 | 同意證據只存「同意了」布林 | 爭議時證明不了同意的是哪一版條款、哪個金額上限 |
| 6 | 換卡被當成「變更收款主體」而要求重新同意 | 買家換張卡要重簽一次，流失 |
| 7 | 🔴 **結算表被寫入** | 違反 **§2.3** 的帳務軸禁令，而**沒有任何編譯錯**——只有 `recurring_settlement_write_total` 抓得到 |

## 9. 品質需求與 NFR

### 9.1 SLO／SLI

| 指標 | 目標 | 量測 |
|---|---|---|
| 週期生成及時性 | `horizon_filled_until` 恆 **> now + 28 天** | R-S1 的候選數 ＋ 該欄的最小值 |
| 鎖單準時率 | `edit_cutoff_at` 後 **5 分鐘內**完成 `LOCKED` | R-S2 的延遲分布（cadence 5m） |
| 扣款成功率 | **≥ 97%**（首次）／**≥ 99%**（含重試） | `recurring_charge_total{result}` |
| 建單成功率 | **≥ 99.5%** | `MATERIALIZING → MATERIALIZED` 比率 |
| 排程器存活 | 每支 `last_success` **< 3× cadence** | `_last_success_timestamp_seconds` |

⚠️ **扣款成功率的分母要排除「買家主動跳過」**，否則跳過率高的月份會讓 SLO 假性下降。
### 9.2 容量與規模（cycle 生成量、scheduler 掃描量）

| 項目 | 估算 | 依據 |
|---|---|---|
| cycle 生成量 | 每契約 **56 天視野** ÷ 週期天數 | 週配 ⇒ 8 期／契約；日配 ⇒ 56 期／契約 |
| R-S1 掃描量 | `ACTIVE` 契約數（每 5 分鐘一輪） | 🔴 **述詞要走 index**：`(status, horizon_filled_until)` |
| R-S2／R-S3 掃描量 | 非終態 cycle 數 | partial index `WHERE status IN (7 個非終態)` |
| 尖峰 | 🔴 **所有契約的截止時間會叢集**（多數人選同一個到貨日） | 見下 |

🔴 **叢集是本書最大的容量風險**：若多數契約的到貨日都是週三，
則 `edit_cutoff_at`（T-3）與 `charge_cutoff_at`（T-1）都會叢集在同一天同一小時
⇒ R-S2／R-S3 的單輪批次量暴增，而**租約是逐筆取的**（§7.0）⇒ 批次總耗時線性成長。

**緩解**：對 `edit_cutoff_at` **與 `charge_cutoff_at` 同時**施加**同一個**分鐘級偏移
（依 `contract_id` hash 決定，**單向提前 0～60 分鐘**），不改變業務語意但把尖峰攤平。

🔴 **兩個 cutoff 必須一起抖，而且抖同一個量。** ⚠️ **只抖 `edit_cutoff_at` 是錯的**：
尖峰**有一半來自 R-S3**（它的候選述詞錨在 `charge_cutoff_at`，§7.0）
⇒ 只抖一個等於只攤平了一半，扣款那一半原封不動——**而扣款正是打第三方、最貴、最不能塞車的那一半**。

🔴 **偏移必須「同量 ＋ 單向提前」，否則會踩爆 §7.4 的三條不等式**：

| 做法 | ① `edit ≤ charge − retry_window` | ② `scheduled − charge ≥ lead + prep` | ③ `charge < scheduled` | INV-10 `expires ≥ charge + 24h` |
|---|---|---|---|---|
| 只抖 `edit` | 🔴 `edit` 往後抖即違反 | ✅ | ✅ | ✅ |
| 兩者各抖各的 | 🔴 可能違反 | 🔴 可能違反 | 🔴 可能違反 | 🔴 可能違反 |
| **同量、單向提前**（本書定案） | ✅ **差值不變** | ✅ 餘裕變大 | ✅ 更早 | ✅ 門檻變低 |

⚠️ 偏移必須是**決定性的**（同一契約每期偏同樣的量），否則買家看到的截止時間每期都不一樣。
⚠️ 偏移在 **R-S1 生成 cycle 的當下**算進 **`edit_cutoff_at` 與 `charge_cutoff_at` 這兩欄**並落庫，**不是查詢時才算**——
🔴 **`scheduled_for_at` 不抖**（它是買家看到的到貨日，抖它就改了業務語意；且上表的 ②③ 兩個 ✅ 正是建立在它不動之上）。
算在查詢端會讓 DB CHECK、排程器述詞與買家看到的畫面各用一套值。
### 9.3 硬上限依賴治理（PAYUNi 呼叫量、DB partition）

> 判準：這個依賴用滿了會「回錯誤／拒絕服務／丟資料」⇒ 適用。**自架不豁免。**

| 依賴 | 上限 | 接入前置三件 |
|---|---|---|
| **PayUni mandate 扣款** | 待書面確認 | ❌ **未齊**（U-R2／U-R3）⇒ 見 §10 |
| **DB partition** | `recurring_cycle` 會線性成長 | ⚠️ 長期契約 ＋ 日配 ⇒ 單一契約 365 列／年。🔴 **不做 partition**；觸發門檻（表列數 > 1,000 萬）登記在 issue #3517 |
| recurring-service 連線池 | chart 預設 | 上線前對帳 |

🔴 **接入前置三件缺一不得接上任何環境的生產路徑（含 dev）**：
① 與上限對齊的指標（用量＋上限兩個 series）② 告警（逼近＋觸頂各一條）③ 預算數字。
⚠️ **禁止用「只是 dev」豁免**——判準是「這條路徑失敗會不會讓人的工作停下來」。
### 9.4 效能預算

| 路徑 | 預算 | 依據 |
|---|---|---|
| RB-3 preview | **p95 < 1.5s** | 要算金額、查庫存、查承載三件 |
| RB-4 建約 | p95 < 2s | 含 mandate 查詢 |
| 日曆查詢（RS-8） | p95 < 500ms | 純讀，走 index |
| R-S3 單筆扣款 | 🔴 **最壞 100s**（PayUni deadline） | **這個數字決定租約下界**（§7.0 的 300s） |
| R-S4 單筆建單 | 最壞 30s | 同上 |

🔴 **「最壞耗時」不是 p99，是 deadline 總和**——租約要能活過最壞情況，
用 p99 算會讓那 1% 的長尾被合法搶走 ⇒ 重複副作用。
### 9.5 資料保存與清理（contract／cycle／audit／invoice retention **分開**；POD／電話／地址到期遮罩；crypto-shredding）

> 🔴 **四類 retention 分開，不得共用一個值。**

| 類別 | 保存期 | 到期動作 |
|---|---|---|
| **contract** | 契約終止後 **7 年** | 稅務／爭議需要 |
| **cycle** | 同上 | |
| **audit**（狀態歷程、maker-checker） | 同上 | |
| **invoice 參照** | 依 58 號 | 🔴 **不是本書決定** |

**到期後的遮罩（不是刪除）**：POD／電話／地址 ⇒ 遮罩；金額與狀態 ⇒ 保留。

🔴 **crypto-shredding**：個資欄位以 per-buyer 金鑰加密，
到期時**刪金鑰**而不是逐列 UPDATE ——後者在千萬列的表上會鎖太久。
⚠️ **金鑰刪除是不可逆的**，且**同一把金鑰可能被多張表引用** ⇒ 刪除前要有引用計數。

⚠️ **retention 的執行是維運 SQL 或 migration，不是 application code 的排程器**（§12.5）。
### 9.6 產品／營運 KPI（漏斗／留存／履約／財務；🔴 **不得只用 client GA4 當財務權威**）

| 面向 | KPI | 權威來源 |
|---|---|---|
| 漏斗 | preview → 建約轉換率 | **後端事件**（`recurring.contract.activated`） |
| 留存 | 契約存活期數分布 | `completed_cycles` |
| 履約 | 每期準時出貨率 | cycle 狀態轉換 |
| 財務 | 每期實收、退款率 | 🔴 **payment 的交易紀錄** |

🔴 **不得只用 client GA4 當財務權威**：
① 廣告阻擋器與隱私設定會讓事件遺失（且遺失是**有偏的**——關心隱私的族群消費行為不同）
② GA4 的事件是「前端說發生了」，而財務要的是「錢真的動了」

⇒ **財務類 KPI 一律以後端交易紀錄為準**，GA4 只用於漏斗與行為分析。
⚠️ 兩者數字不同是正常的，**報表要標明來源**，不要讓人以為其中一個壞了。

## 10. 未決前提與外部窗口

> 已知取捨與技術債（六條，各附觸發門檻）在 **issue #3517**，本書不留該表——
> 它的內容會隨門檻被觸發而變，而會變的東西不能寫進規格。

下列前提來自**外部窗口**，取得前對應功能以 feature flag 維持關閉。
🔴 **這是 fail-closed 規格，不是待辦清單**：

| # | 缺什麼 | 向誰要 | 🔴 拿不到時的 fail-closed 行為 | 懸在它上面的欄位 |
|---|---|---|---|---|
| **U-3** | 每期開票時點的**會計法遵**確認（工程面已定「出貨時」） | 會計師書面 | 🔴 **不得產生會觸發開票的 cycle**（D-8） | `seller_invoice_ref` 何時非空 |
| **U-R1** | 每一賣家 MerID 能否建 Token mandate、是否嚴格綁該 seller merchant scope、不得跨賣家重用 | PayUni 書面 ＋ sandbox contract tests | 🔴 **整條定期配送不上線**（收款模型要重做） | `recurring_payment_mandate` 全表 |
| **U-R2** | 首期 3D、續期 MIT 標記、Token／Period ID、卡片更新與被動換卡 | 同上 | 續期成功率與失敗分類無法定義 ⇒ **不上線** | — |
| **U-R3** | 建立／查詢／改金額／暫停／恢復／終止／退款 API 的正式欄位、簽章、冪等與錯誤碼；**尤其 `CreateRecurringMandate` 回應是否含 `expires_at`** | 同上 | 🔴 **建約失敗**——§7.0 四問第 1 格明訂**不得寫 NULL 放行**；§4.5 的 adapter 只能用 hermetic fake | `credit_life`／`expires_at` |
| **U-R4** | Webhook event ID、重試、亂序、query fallback 與「扣款結果未知」處理 | 同上 | inbox 去重鍵無法定案；換卡靠 R-S6 的 `ACTION_REQUIRED` 兜底 | `provider_event_key` |
| **U-R5** | 賣家 merchant 的續期額度、費率、provider 撥入帳戶時點、reserve、退款與 chargeback 報表語意；`FIXED_TERM_PREPAID` 的會計與法務 | PayUni 正式契約 ＋ 會計師／法務 | 🔴 **不做 `FIXED_TERM_PREPAID`**，只做每期扣款 | `recurring_cycle_settlement_fact.provider_processing_fee`／`provider_reported_deposit_amount` |
| **U-R6** | 商戶是否已開通 Period／Token、IP allowlist、sandbox、授權有效期、最遲請款 SLA、NT$1 綁卡的帳單／退款語意 | PayUni 商戶設定 | **canary 不得開**；無 sandbox ⇒ 契約測試只能對原始碼 | — |
| **U-R7** | 「持續配送」是否納入範圍（§2.2） | 產品裁決 | flag `RECURRING_OPEN_ENDED_ENABLED` 維持關閉 | `recurring_schedule_rule.cycle_count IS NULL` 的 publish gate |

🔴 **U-R3 把一個外部未決變成 happy path 的硬閘門** ⇒ 它同時登記在 §12.0 的 Phase 0 阻斷項。

⚠️ **U-R1／U-R2 未關閉前，本書可以寫完、可以部署，但不得對外開。**

⚠️ **另有一項由本書自己定**：替代品的「等價」由誰認定（賣家提案 vs 平台白名單）
⇒ 決定 `recurring_substitution_consent` 的建立入口（§6）。

### 10.1 上線順序前置

🔴 必須在物流 v1.2 的付款／發票／ledger／reconciliation／Admin control tower 完成之後；
正式一次性交易**穩定 30 天且無 P0** 才進真金流 canary。
**起算點 ＝ 物流 v1.2 全數完成之日。**

## 12. 部署與維運
### 12.0 Phase 0 阻斷項（任一未解 ⇒ 不得部署到有真實買家的環境）

① PayUni Token 幕後規格、開通與綁定 IP ② `3005` 賣家 lead time（65 P-5）③ 錯誤碼號段擴段 ④ Admin RBAC（D-20）⑤ `recurring-service` 六類登記與 NetworkPolicy 全數就緒。

### 12.1 環境與設定（**新服務**）

**Go 服務**（65 §3.9 十）；🔴 **不需挑 TSID Node ID**（`pkg/tsid.NewGenerator`）。
🔴 **Port 以 §4.3 為唯一權威**：`HTTP_PORT=10043`（health／metrics／探針／`sql_exporter`）、
`GRPC_PORT=10044`。兩處**必須同值**——`k8s/values/recurring-service.yaml` 填的埠與
§4.3-② 的 NetworkPolicy 放行的埠對不上時，**policy 放行的是一個沒有人在聽的埠，
而 gRPC 呼叫只會回 UNAVAILABLE**，fail-open 的呼叫端連錯誤都不會浮上來
（見 65 §6.1.5「新增跨服務呼叫的三件」）。
⚠️ **施工前重跑一次 port 盤點**（`grep -rhoE '100[0-9][0-9]' k8s/values/ | sort -u`）——
同期落地的其他設計書可能已經佔走這兩號。**六類登記**：`services/recurring-service/Dockerfile`（build 集鍵）／`k8s/values/recurring-service.yaml`（deploy 集鍵，`test-content-addressed-tag.sh` 性質 8 雙向守住）／`go.work`／CI path-filter／NetworkPolicy／MQ topology。**env 五處同步**見 §8.5。

### 12.2 遷移步驟（順序不可換；每步標「漏了會怎樣」）

| # | 步驟 | 漏了會怎樣 |
|---|---|---|
| 1 | 🔴 **新建 schema，絕不重用既有「訂閱店家」或補貨通知的表**；既有資料**不遷移** | 兩個語意不同的「訂閱」共用一張表，永遠分不開 |
| 2 | **expand-only**：recurring-service 的新表全新增（Atlas）；既有 `orders` 只 `ADD COLUMN … NULL`（order-service Flyway，三個新增欄 `order_origin`／`origin_ref`／`recurring_contract_id`） | 帶 `DEFAULT` 的回填會把每一列都寫成 recurring |
| 3／4 | CodeMeta 落碼（號段擴完後）`generate.sh --all` ＋四份前端拷貝同 PR；MQ topology apply（§6.5 末段）→ 確認 `consumers ≥ 1` | drift gate 逐位元對拍會紅在無關處；訂閱靜默失效 |
| 5 | 🔴 **writer allowlist**：只開 `SELLER_MANAGED` ＋ `3005`；`3001`／`3002`／`3003`／`3004` writer **永久關閉** ＋ negative contract tests | 漏了就會有一期走進平台物流鏈，而那條鏈會建立 `settlement_candidate` |
| 6 | **shadow scheduler 連續 14 天**：七支全開但**不發動任何外部副作用**，只記「本來會做什麼」 | 直接開＝第一天就對真實買家扣款 |
| 7／8 | canary 1–3 家賣家（每家先跑一期**不收錢的 dry cycle**）；**永久 guard 驗證**：DB CHECK ＋negative tests 在 canary 期間各跑一次變異 | guard 裝了但沒接上 |
| 9 | **rollback 只停新建**：停 RS-5 publish 與 RB-4；🔴 **既有 contract 繼續跑到期末，不得中途斷** | 已扣款的期次無人履約 |

### 12.3 Runbook 與一次性資料處理

**Runbook 骨架**：排程停擺 > 24h（🔴 先轉 incident）／扣款長期 `UNKNOWN`／`MATERIALIZING` 卡住／PayUni 幕後被拒（IP 白名單漂移 ⇒ 查 egress-proxy）／DLQ 堆積／容量超賣／canary 回退。

歷史資料清整用 **migration SQL 或經審核的維運 SQL**；🔴 **不得在 application code 建立 reconciliation job／backfill CLI／drift scanner**（《後端服務責任邊界規範》）。
🔴 **改狀態的維運 SQL 必須先跑四問**：這個值有沒有執行者？推走它的那一句 allowed-from 含不含這個組合？同一交易要不要補一列 `recurring_action_audit`？**裸 UPDATE 不留痕 ⇒ 下一個人看到的軌跡是假的。**

### 12.4 Runbook

> 需求書 §20.20 列了 12 項。**每一項的第一步都是「先查它卡在哪個狀態」**，不是直接動手。

| # | 症狀 | 先查 | 處置 |
|---|---|---|---|
| 1 | 某期沒扣款 | cycle 狀態 ＋ `charge_attempt_count` | `< 3` ⇒ 等 R-S3；`= 3` ⇒ `ACTION_REQUIRED`，通知買家 |
| 2 | 卡在 `CHARGING` | `unknown_since` | 走 §7.5 的 UNKNOWN 收斂 |
| 3 | 卡在 `MATERIALIZING` | `was_charged` | `true` 且退款失敗 ⇒ RA-6／RA-7 人工 |
| 4 | 週期沒生成 | 🔴 **`horizon_filled_until` 是不是 NULL** | R-S1 述詞的 `IS NULL OR` 分支 |
| 5 | 排程器候選恆 0 列 | 🔴 **allowed-from 含不含起點** | rows=0 通常不是「沒東西」 |
| 6 | 到期預告沒發 | `mandate_expires_at` 是不是 NULL | `recurring_mandate_snapshot_missing` |
| 7 | 承載超賣 | hold 的 `expires_at` | `chk_*_lifetime` 有沒有被繞過 |
| 8 | 訂閱建不出來 | RS-5 十項前置哪一項 fail | `data.failed[]` |
| 9 | 訂閱顯示「待付款」 | A 軸狀態 | §7.3.5：應該已推到已付款態 |
| 10 | 契約卡 `ACTION_REQUIRED` | Cycle 連續失敗次數 | §7.3.4 的連動規則 |
| 11 | 事件沒到 consumer | `topology.yaml` 有沒有該 queue；`consumers` 數 | 漏登記 ⇒ 補條目走 CI apply |
| 12 | 對帳不平 | 六個快照欄有沒有齊 | INV-P5 |

🔴 **任何一項都不得以「直接改 DB 狀態」收尾**——那會造出一個沒有出口的狀態（§12.5）。
### 12.5 一次性資料處理的分界

🔴 **判準：目的是業務正確性 ⇒ 可以做；目的是資料完整性監測 ⇒ 一律維運 SQL。**

| 允許 | 禁止（不得寫進 application code） |
|---|---|
| 狀態回查（外部沒推 webhook，主動去問） | `*ReconciliationJob` |
| 業務對帳（與 provider statement 逐筆核對） | `*BackfillCLI` |
| 一次性歷史清整（**migration SQL 或經審核的維運 SQL**） | `reflow_task` 表 |
| | `@Scheduled` drift 偵測 |
| | application-level outbox（既有除外） |

🔴 **改狀態的維運 SQL 要先跑四問**（65 §3.11），額外再問兩件：
- **我要寫的這個值，有沒有執行者？** 把 row 推回非終態 ＝ 手動造一個沒有出口的狀態
- **同一交易要不要補一列狀態歷程？** 裸 `UPDATE` 不留痕 ⇒ 下一個人看到的軌跡是假的

⚠️ **本書的 `recurring_cycle` 特別危險**：它有 10 個值、7 個非終態，
而排程器的 allowed-from 是白名單 ⇒ **手寫 SQL 造得出一個沒有任何排程器認得的組合**。
### 12.6 關鍵路徑（技術依賴順序，一條線）

> **一條線，不是清單。** 每一步的前置是上一步。

```
U-R1/U-R2/U-R3 書面確認
   └─▶ 附錄 C 碼值（generate.sh --all ＋ 四份前端拷貝）
         └─▶ proto（payment 的 5 支 RPC ＋ recurring 的內部契約）
               └─▶ DB migration（Go Atlas；recurring 新庫）
                     └─▶ 🔴 MQ topology（CI apply；queue 必須先於 consumer 存在）
                           └─▶ payment-service 的 mandate 擴充
                                 └─▶ recurring-service 部署 ＋ NetworkPolicy ＋ rollout restart
                                       └─▶ Gateway 路由 ＋ Swagger
                                             └─▶ 前端（65A §2.6／§3.1）
                                                   └─▶ RS-5 發布第一個方案（十項前置）
```

🔴 **第一個 ＋ 最後一個是硬閘門**：沒有 U-R1～U-R3 不能開始；
RS-5 十項前置任一不過就不會有第一個方案（**第 7 項在 65 P-5 定案前恆為 fail**）。

⚠️ **payment 的 mandate 擴充在 recurring-service 之前** ——
反過來做的話 recurring 部署後打不到 mandate RPC，而**呼叫端若是 fail-open 會完全無聲**。

## 13. 測試策略
**分層**：單元（金額、cutoff 換算、CAS allowed-from）→ repository 整合（**帶 `-tags integration` 也要 `go vet` 過**）→ 服務內 saga（hermetic fake PayUni）→ 跨服務契約 → dev E2E（一條契約跑滿三期）。

### 13.1 🔴 反向斷言：買家確認收貨**不觸發任何結算**

獨立成一支測試，因為讀者會照 65 的 `PLATFORM_CONSIGNMENT` 商流推論而做出相反的東西。造一筆 recurring 訂單走完 `confirm_receipt`，斷言**四件**：`settlement_candidates`／`payout_instructions`／`ledger_entries` 對該 `order_no`（或該賣家該期別）**皆零列**，且 `recurring_settlement_write_total` 仍為 **0**。
🔴 **不得拿 `PLATFORM_CONSIGNMENT` 訂單當對照組**：沒有任何路徑會產生那種訂單
（65 §3.8b C-1）⇒ **建不出那筆 fixture**；硬繞過（直接 INSERT 繞開應用層）造出來的是
**應用層永遠不會產生的狀態**，證明不了「查詢打得到真實資料」。

**對照組三選一，至少做第一個**：
1. 🔴 **同表的正向列**：在 `settlement_candidates`／`payout_instructions`／`ledger_entries` 各插入一筆
   **與本測試無關的既有列**（或用既有 seed），對**同一支查詢**斷言**有列** ⇒
   證明「查詢打到對的表、對的述詞」；再對本測試的 `order_no` 斷言零列。
2. **變異驗證**：故意讓 recurring 的流程寫一列進去（注入），斷言測試**轉紅**；還原後轉綠
   ⇒ 證明這支測試抓得到它該抓的東西（比對照組更強，見 `03-delegation-templates.md` §0.4）。
3. **指標側對照**：`recurring_settlement_write_total` 的**同名指標在別的服務有非零值** ⇒
   證明指標名字沒打錯。

⚠️ **不要因為「造不出對照組」就把這支測試刪掉**——它擋的正是「讀者照舊商流推論做出相反的東西」，
而舊文件仍在 repo 裡（65 §5.3／§5.4 整章都還在）。
**再加一道 CI gate**：`recurring-service` 原始碼（含 SQL 與 migration）對 `settlement_candidate`／`payout_instruction`／`seller_net`／`ledger_entr`／`payout_batch` **零命中**（同一支 grep 對 `recurring_cycle` 有命中即證明查詢有效）。

### 13.2 negative contract tests（fail-closed 一律轉紅）

任何路徑傳入 `3001`／`3002`／`3003`／`3004`／`TCAT`／`7-11`／`UBER_DIRECT`／`PLATFORM_CONSIGNMENT` ⇒ 一律拒絕並回 `RECURRING_SHIPPING_POLICY_VIOLATION`／`RECURRING_FULFILLMENT_NOT_ALLOWED`（🔴 **碼名以 §6.7 為準**——那兩個名字是需求書 §20.11／需求書 §20.18 已有的，本書照抄不自創）。**要測的路徑逐條列出**（漏一條＝那條路徑沒有防線）：RB-3 preview／RB-4 建約／RB-7 修改／RS-1 建方案／RS-5 publish／R-S4 傳給 `CheckoutConfirm` 的參數／DB CHECK 直接 INSERT。另加 §6.3.3 那兩行讀寫邊界逐項一支。

### 13.3 契約測試與 fixture 來源

🔴 **禁止手寫 fixture**（自己跟自己對必然全綠）：
- **有唯一定義的**（order／product／payment proto、gateway DTO ＋ swagger 註解、CodeMeta）⇒ fixture 從**那份定義**生。
- **沒有唯一定義的**（MQ payload：兩端各自手寫 struct）⇒ **producer 原始碼就是 de facto 契約**，從 order-service outbox payload struct 解析欄名與型別；正例 `services/invoice-service/internal/mq/producer_contract_test.go`。
- **期望值一律硬寫字面值**，不得呼叫被測函式或它引用的常數（否則期望值跟著被測物一起變，測不到任何東西）。
- **PayUni**：正式 API 未取得前用 **hermetic fake**；🔴 **不把 mock 當 provider proof**——fake 全綠只證明我方邏輯自洽。

### 13.4 鑑別力驗證（每道 guard 弄壞→轉紅→還原→轉綠）

| 變異 | 必須轉紅的東西 |
|---|---|
| 拿掉 `chk_recurring_fulfillment_mode` DB CHECK | negative test 的 DB 直插那一條 |
| `MarkPaid` 的 allowed-from 從 `('CHARGING','ACTION_REQUIRED')` 改成 `('CHARGING','ACTION_REQUIRED','LOCKED')` | 「未扣款不得進 PAID」那一條。⚠️ 基準值含 `ACTION_REQUIRED`（RB-15 手動付款成功後直接進 PAID，見 §7.3.2）——寫成 `('CHARGING')` 會讓這支變異測試的前提是錯的 |
| R-S3 租約 300s → 60s | §7.4 R④ 的租約單元測試 |
| 拿掉 R-S3 述詞的 `charge_cutoff_at > now` | 「過了截止不再扣款」那一條 |
| 在 recurring 的 SQL 加一行 `INSERT INTO settlement_candidates` | §13.1 的 CI gate |

🔴 **測試必須從 claim 走起**，不得用 fixture 直接把 cycle 設成 `LOCKED`／`PAID`——那樣看不到 allowed-from 的缺口。🔴 **fake 要真的維護狀態並鏡射 SQL 的 allowed-from**；只記 `calls` 的替身零鑑別力。
🔴 **變異沒轉紅要三選一並附證據**：測試弱（缺哪個斷言）／變異無鑑別力（哪條冗餘防線接住，`檔案:符號名`）／預期本身錯了（實際由誰守住）。**不准只寫「沒轉紅」**。

### 13.5 測試資料與環境防線

fixture deterministic、可重跑、不含真實個資或 secret；dev smoke 契約用前綴 `RCUR-SMOKE-` 並在驗證後清理——🔴 **含 `recurring_cycle` 與 `recurring_charge_attempt`，否則下一輪 R-S3 會對殘留列真的發動扣款**。dev 不接真實 PayUni Token 幕後（規格未取得），`RECURRING_PROVIDER_MODE=fake` 是 dev 預設，**prod 值不得為 `fake`**（`Config.Validate()` return error）。

### 13.6 與既有 CI gate 的關係

**本地四件**：Go 本地四件（`gofmt -l`／`go build`／逐模組 `go test`／🔴 `go vet -tags integration`）／sqlc generated diff（動到 `db/queries/*.sql` **任何一行含註解**就要重跑 `make sqlc-generate`）／`atlas migrate hash`／MQ 三件（`validate.py --topology`、`unittest discover -s tools`、`mq-consumer-registration-check.py`）／CodeMeta drift（`--all`）／swagger 契約 Check B。
🔴 **動到 product／order／cart／payment／shipping 任一層 ⇒ push 前必跑 `infra/smoke-checkout.sh`**；R-S4 走 `CheckoutConfirm` ⇒ **一定命中**。

**逐 gate 對照**：

| 既有 gate | 本書會不會踩到 | 要做什麼 |
|---|---|---|
| `codemeta-copies`（pre-commit） | ✅ 附錄 C 新增碼值 | 🔴 一律 `generate.sh --all` ＋ commit 四份前端拷貝 |
| `CodeMeta drift diff`（CI，逐位元） | ✅ 同上 | 同上 |
| `proto-numeric`（CI） | ✅ 新 proto | 🔴 **response 側一律 `string`**（走訪起點是 OUTPUT type，不看是否對外） |
| `atlas-validate`（pre-commit） | ✅ 新 migration | `atlas migrate hash` |
| `sqlc Generated Diff`（CI，逐位元） | ✅ 新 query | 🔴 **動到 `db/queries/*.sql` 的任何一行就要重跑 `make sqlc-generate`——包含只改註解** |
| `mq-consumer-registration-check.py` | ✅ 新 consumer | 同 PR 加 `topology.yaml` 條目 |
| `test_expand.py` 資源數量 pin | ✅ 新 queue | 帶 `dlq:` 的 queue ＝ queues +2、bindings +2 |
| `go vet -tags integration`（pre-commit） | ⚠️ 改 repository 簽章時 | 🔴 **`go build`／`go test` 看不見 build-tag 檔** |
| `gofmt`（CI 第一步） | ✅ | 用 CI 的原命令（處理了中文路徑引號） |

⚠️ **`static-gates.yml` 的 MQ 登記一致性 step 只掃 Go** ⇒ 本服務是 Go，會被掃到。

## 14. 術語表
### 14.1 UI 名詞 vs Domain 名詞對照（定期配送／關注店家／補貨通知／配送週期／截單／每期金額上限）

> 🔴 **同一件事對買家與對內部有兩套說法。給買家看的一律用左欄。**

| UI 名詞（給買家） | Domain 名詞（內部） | 不要對買家說 |
|---|---|---|
| **定期配送** | `recurring_contract` | 「訂閱」（易與會員費混淆） |
| 關注店家 | — | — |
| 補貨通知 | — | 「缺貨例外」 |
| **配送週期** | `recurring_cycle` | 「期」「cycle」 |
| **修改截止** | `edit_cutoff_at` | 「鎖單」 |
| **扣款日** | `charge_cutoff_at` | 「截單」 |
| 每期金額上限 | `amount_cap` | 「cap」 |
| 需要您確認 | `ACTION_REQUIRED` | 「異常」（會嚇到買家） |
| 本期已略過 | `SKIPPED` | 「失敗」 |

⚠️ **`SKIPPED` 特別重要**：它可能來自買家主動跳過，也可能來自「未回覆例外逾時」——
兩者對買家的文案不同，**不得共用一句「本期已略過」**而不說原因。
### 14.2 同一件事的多個名字（命名漂移統一表的對外版）

> 對外版；完整的命名漂移表在 §3 的稽核段。

| 這個東西 | 需求書叫 | 本書叫 | repo 既有叫 |
|---|---|---|---|
| 契約 | 訂閱／subscription | `recurring_contract` | — |
| 期 | cycle／期次／配送週期 | `recurring_cycle` | — |
| 授權 | mandate／授權／token | `payment_mandate_ref`（本地）／`recurring_payment_mandate`（payment 側） | — |
| 商家自送 | 自送／商家配送 | `MERCHANT_DELIVERY`／`3005` | 69 號 |
| 承載 | 容量／配額／bucket | `merchant_capacity_bucket` | 69 §5.1.7 |
| 賣家識別 | 賣家 ID／商家 ID | 🔴 **`store_id`**（不是 `account_id`） | 44 號 |

🔴 **最後一列是最容易錯的**：`account_id` 是登入 principal、`store_id` 是營運鍵，
**首任 grandfather 曾經相等，現在已全面分離** ⇒ 不得假設恆等。

## 附錄 C：CodeMeta 與碼表新增清單（Contract 7 值／**Cycle 10 值**／price_policy／事件 22 個／錯誤碼）

> 🔴 **本附錄只是清單，不是 SSOT。** 真正的 SSOT 是 `docs/代碼管理（CodeMeta）/*.md`。
> 落碼時機見 §12.2：**與服務同一波**，一次改完 `.md` 再跑 `generate.sh --all`
> ——分批改會讓 `static-gates.yml` 的 drift gate 逐位元對拍紅在每一個中間階段的 PR 上。

### C.1 新增 enum

| Enum | 值數 | 值 |
|---|---|---|
| `RecurringContractStatus` | **7** | `PENDING_MANDATE`／`ACTIVE`／`PAUSED`／`ACTION_REQUIRED`／`CANCEL_AT_CUTOFF`／`CANCELLED`／`COMPLETED` |
| `RecurringCycleStatus` | **10** | `SCHEDULED`／`LOCKED`／`CHARGING`／`ACTION_REQUIRED`／`PAID`／`MATERIALIZING`／`MATERIALIZED`／`COMPLETED`／`SKIPPED`／`CANCELLED` |
| `RecurringPricePolicy` | **3** | `FIXED_PER_CYCLE`／`CURRENT_PRICE_MINUS_DISCOUNT`／`FIXED_TERM_PREPAID`（見 §0.7／§5.1.1） |
| `RecurringEventType` | **22**（出向 15 ＋ 入向 3 ＋ 保留 4） | 見 §6.5.1；🔴 **Go 慣例：Code ＝ Value ＝ `UPPER_SNAKE`** |

🔴 **`RecurringContractStatus` 與 `RecurringCycleStatus` 的同名值是兩個獨立 enum**（D-5），
**不共用值域**——`ACTION_REQUIRED` 在兩軸的語意與出邊都不同。

🔴 **`RETIRED` 不是 contract 軸的值。** 它是 `recurring_plan.status`（§5.1.1 的
`chk_rp_status`：`DRAFT`／`ACTIVE`／`PAUSED`／`RETIRED`），由 RS-7 下架寫入。
⚠️ **把 `RETIRED` 抄進 `RecurringContractStatus` 會擠掉 `CANCEL_AT_CUTOFF`**，
症狀兩個、都不在編譯期出現：① `CANCEL_AT_CUTOFF` **永遠落不了碼** ⇒ RB-10 的 `AT_CUTOFF`
分支沒有可寫的 enum 值 ② 生成的值域與 §5.1.2 的 `chk_rc_status` 不相交
⇒ 寫 `RETIRED` 進 contract 一律 **23514**，而寫 `CANCEL_AT_CUTOFF` 則過得了 DB、
過不了 CodeMeta validator。
🔴 **值域的唯一權威是 §7.3.1 逐值表 ＋ §5.1.2 的 `chk_rc_status`，本附錄只是抄過來的清單。**
⚠️ 賣家方案軸的 `DRAFT`／`ACTIVE`／`PAUSED`／`RETIRED` 目前**只活在 DB CHECK、沒有對應
CodeMeta enum**；它是否要成為第三個 enum 不在本附錄清單內（前端只看得到 published version，§6.1）。

### C.2 錯誤碼

本書申請 **37 個**（8＋8＋9＋6＋6，逐條見 §6.7），分五群（群內留空位），號段依 65 附錄 C.5 的程序**由 65 §6.7.1 連號追加**。
🔴 **只給名字不給號**——各本自己撿號會撞。

### C.3 落地流程（不得跳步）

1. 改 `docs/代碼管理（CodeMeta）/*.md`
2. `cd docs/代碼管理（CodeMeta）/scripts && ./generate.sh --all`（🔴 **一律 `--all`**）
3. 同一個 PR 內 commit **四份前端拷貝**（pre-commit 的 `codemeta-copies` 會擋）
4. 編譯驗證

⚠️ **禁止直接改生成檔**（下次生成會覆蓋，曾造成執行期 crash）。
⚠️ **禁止在 CodeMeta enum 內放業務輔助邏輯**（routing／mapping／I18n）——放獨立 helper。
## 附錄 D：前端契約與交付對照（端點 ↔ 畫面 ↔ 路由 ↔ 檔案；畫面細規 → 65A）

畫面、路由與檔案的逐項對照見 [65A](65A-物流履約平台前端契約（Fulfillment%20Frontend%20Contract）詳細設計書.md)；
端點清單見本書 §6.1（買家）／§6.2（賣家）／§6.3（ADM）。
