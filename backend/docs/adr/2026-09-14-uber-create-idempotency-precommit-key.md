# Uber 建單冪等性：以我方 pre-commit 鍵為第一道，官方 `idempotency_key` 為第二道

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-14 |
| 影響範圍 | shipping-service Uber Direct 建單 |

## 脈絡

66 原本在 8 處把「Uber Create Delivery 支援 idempotency key」當既成事實，
而那是一個**未經權威來源證實**的前提：66 陷阱 A8（「把 5xx 當建單失敗 ⇒ 兩趟車、重複計費」）
要防的後果，正是該前提塌掉時會發生的事——**防線本身建立在對方的行為上**。

官方 Direct API spec（`uber/uber-direct-sdk` 的 `src/deliveries/openapi.yaml`）確實有
`idempotency_key`，逐字：「A key which is used to avoid duplicate order creation with identical
idempotency keys **for the same account**. The key persists for a set time frame,
**defaulting to 60 minutes**.」 ⇒ 它是**有 TTL、且在對方手上**的保證。

## 決定：三層我方保證 ＋ 協議層第二道防線

- **不依賴官方冪等性作為第一道。**
  1. **pre-commit 鍵**：呼叫 Uber **之前**先以 `uber_create_delivery:{order_no}:{leg}` 在我方落地一列
     （`uber_deliveries` 或專用 attempt 表），`UNIQUE` 由**我方** DB 保證
     ⇒ 重試取得同一列，**不是**依賴對方去重
  2. **逾時／5xx 一律 `991 CREATE_UNKNOWN`，不重建**（66 §4.5.4 已有此分流，本 ADR 把它從
     「最佳實務」升格為**唯一合法路徑**）
  3. **收斂靠查詢不靠重送**：`991` 走 T-8 回查（E4／E3），查到就接回、確認沒有才走 `990`
- 🔴 **官方 `idempotency_key` 是第二道防線，不得取代第 1 層**——我方 UNIQUE 是唯一我們能保證的那一道。

## 第二道防線：協議層 dedupe

- 出站 Create Delivery **一併帶 `idempotency_key`**，值即第 1 層那個 pre-commit 鍵。

  🔴 **這不是「把第 1 層送給 provider」，而是「兩層共用同一份 persisted 值」。**

  | 層 | 是什麼 | 誰保證 | 保證什麼 |
  |---|---|---|---|
  | **Layer 1** | **local pre-commit claim record**（internal ownership record，**不是一支 identifier**） | 我方 DB `UNIQUE` / CAS | **ownership / 併發**——同一 leg 只有一個執行者拿得到 claim |
  | **Layer 2** | Uber **`idempotency_key`**（protocol field） | Uber（**預設 60 分鐘 TTL**） | **protocol dedupe**——同鍵重送不會建第二趟 |

  ```
  local pre-commit claim record                  ← Layer 1（機制）
     └─ keyed by  persisted technical dedupe key ← 同一份 immutable 值
            └─ 原值送出成 Uber idempotency_key    ← Layer 2（協議欄位）
  ```

  🔴 **共享 key value ≠ 共享責任**：Layer 1 沒有 TTL 且在我們手上，Layer 2 有 TTL 且在對方手上
  ⇒ **兩層不得寫成互相取代**。
  ⛔ **不得為 Layer 1 另開第四支 identity**（`local_claim_id`／`precommit_id`／`claim_uuid`）——
  多一支就多一張 mapping 與一種 correlation failure mode，而目前沒有客觀需求。
  ⚠️ **`external_id` 不在這兩層之內**：它是 business correlation identity（66 §6.3.2a-1 的 C），
  **語意角色與 `idempotency_key` 不同，不得互換，也不得共用同一個 DB 欄位存放**。
- 🔴 **60 分鐘 TTL 是新的硬邊界**：`991` 的自動收斂改為「帶同一個鍵重送」（66 §7.7），
  **超過 60 分鐘該路徑失效** ⇒ 告警門檻要卡在 60 分，而不是回查窗的 180 分。
- 🔴 **另有官方側唯一性**：`manifest_reference` × `external_id` **組合必須唯一，否則建單直接失敗**
  ⇒ 兩欄的產生規則要保證組合唯一（66 §6.5）。

⚠️ **第 3 層（收斂靠查詢）分兩段**：
List **沒有** `external_id` 的 **exact query**（66 `UP-9` 負面關閉）⇒ 不得以 `external_id` 反查；
**但 List 有 `external_store_id`／`start_dt`／`end_dt` 可做有界過濾**（官方 changelog 2025-04-17，
SDK spec 未收錄）。
⇒ **≤60 分鐘**由 `idempotency_key` replay 承擔（協議層保證）；
**>60 分鐘**走 **bounded candidate search ＋ 嚴格本地比對**，**只有唯一且完整匹配才自動收斂**，
0 筆或多筆一律進 Admin／Dashboard recovery。🔴 **禁止 unbounded `filter=ongoing` 全掃。**
- **已落實於**：66 詳設 §0.2 T-4、§4.5.4、§5.3、§11.2。

## 修訂：設計書 65A、66、68、69 已刪除（2026-10-29）

物流履約前端實作規格（65A）、Uber Direct 整合（66）、7-ELEVEN 大宗寄倉（68）、商家自有出貨（69）的詳細與基本設計書已刪除：65A、68、69 未實作且不在任何計畫內，66 的 Uber 已由 ADR `2026-10-27-general-ecommerce-scope-cut.md` D4 決定刪除。本文中「65A」「66」「68」「69」號設計書與它們的章節編號指向已刪除的文件，內容見 git 歷史。設計書 65 只保留已實作的部分。
