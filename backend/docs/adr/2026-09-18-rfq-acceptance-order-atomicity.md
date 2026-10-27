# 買家接受與建單的原子性：Outbox ＋ 冪等建單命令

| 項目 | 值 |
|---|---|
| 狀態 | superseded by `2026-09-24-go-modular-monolith.md`（D4：接受與建單同一個 transaction）。遷移計畫 B8 落地前，現行程式仍依本檔運作 |
| 日期 | 2026-09-18（決策作成於 2026-09-06） |
| 影響範圍 | ai-service（Go/Atlas）、order-service（Java/Flyway） |

規格本身見
`docs/詳細設計書/64-Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）詳細設計書.md` §6。

## 脈絡

不變式要求「一次接受最多產生一張待付款訂單，且必須與接受在同一個原子單位內成立」——
「接受成功但沒有訂單」與「訂單存在但接受未記錄」兩種中間態都不允許被觀察到。

而兩者分屬兩個服務、兩個資料庫、兩種語言、兩套 migration 工具。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| A. 分散式交易（2PC／Saga with strict compensation） | 語意最強，但本平台**沒有任何既有的跨語言分散式交易基礎設施**。引入它會是這條線上最大的單一新機制；且 order-service 是既有服務，改它的寫入路徑會影響非採購訂單 | 否決 |
| **B. Outbox ＋ 冪等建單命令** | 接受與建單之間有一個可觀察的窗口 | **採用** |
| C. 由 order-service 主導，ai-service 只記錄 | acceptance 需要 CAS 的對象（publication、seller response、RFQ snapshot）全都住在 ai-service 的 DB ⇒ CAS 變成跨服務讀取，等於把識別鏈的保證整組削弱 | 否決 |

## 為什麼是 B

與本 repo 既有的 outbox 慣例一致（claim／retry／terminal shell 都已存在），**不引入新機制**。
而不變式真正要防的是「接受成功卻永遠沒有訂單」，那由 outbox 的重試與 terminal 狀態守得住。

不選 A：為一條 pre-launch 的線引入平台層級的新機制，成本與風險都不成比例。
不選 C：它用一個較弱的 CAS 換一個較強的原子性，而錯配路徑是真實發生過的失效模式
（賣家改價、買家在另一個裝置改了需求）。

## 接受的代價

🔴 **中間態必須是產品上明說的狀態，不是隱藏的。**
「已接受，訂單建立中」是買家看得到的狀態，**不得畫成「已成交」**。

## 後果

- 建單命令以 acceptance id 為冪等鍵。
- 重新評估條件：若平台日後真的引入跨語言分散式交易基礎設施，這個決策可以重看；
  在那之前，為單一流程自造一套不划算。
