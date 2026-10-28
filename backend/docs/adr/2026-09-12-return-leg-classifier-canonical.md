# 逆向 leg 的分類器收回 canonical：`ReturnShippingRules` 取代 `PAYUNI_RETURN_*`

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-12 |
| 影響範圍 | common-core 逆向 leg 分類器 |

- **脈絡**：前一輪 Focused Review 宣稱「09 有第二套 **provider-specific** status mapping」。
  裁決：**部分成立，且那個措辭會誘導出錯誤的修法**。
  - **成立**：`AfterSalesConstants` 的 5 個 `PAYUNI_RETURN_*` 確實是第二套手寫分類集合，
    與正向 leg 的 `ShippingStatusRules` 完全獨立、**零機制保證一致**
  - 🔴 **不成立**：它**不是 provider-specific**——它比對的是**已正規化的 `ShippingStatus.value()`**，
    不是 PayUni wire code。`PAYUNI_` 前綴是**謊話**，因為 PayUni 的映射恰好是恆等的
    （20 個兩位數 raw ≡ normalized），所以這個謊話至今隱形
  - ⇒ 🔴 **加一套 `UBER_RETURN_*` 是架構上錯誤的修法**：Uber 的 raw 值在 shipping-service
    就被轉成 `930`／`933`，**永遠到不了 aftersales**。而目前這個名字正是會誘導人這樣做

**決定性證據（正逆向同一個 wire 欄位、同一個值域，兩種消費紀律）**：

| | 正向 leg（order-service） | 逆向 leg（aftersales-service） |
|---|---|---|
| 型別 | `ShippingStatus.fromValue()` 枚舉 | 裸 `String` 比對 `Set<String>` |
| 分類器 | `ShippingStatusRules`（common-core 共用） | 5 個**服務內私有**字面值集合 |
| 未知值 | 拋 `IllegalArgumentException` → DLQ（**fail-loud**） | `else` → `log.info` ＋ ACK（🔴 **靜默丟棄，連 warn 都不是**） |

  ⚠️ `aftersales-service` 全服務（main＋test）對 `ShippingStatus` **零引用**
  （對照組：同目錄 `ShippingMethod` **19 檔／49 次**命中、`order-service` 的 `ShippingStatus` 6 檔命中 ⇒ 查詢有效）。
- **真正的缺口**：`normalized → 售後語意` 這個位置**沒有 canonical classifier**，被 `PAYUNI_RETURN_*` 佔著。
- **決定**：
  1. 🆕 在 **common-core** 新增 `ReturnShippingRules`，與 `ShippingStatusRules` **同層同宿**，
     入參型別為 **`ShippingStatus` 枚舉**（不是 `String`）
  2. `AfterSalesConstants` 的 5 個 `PAYUNI_RETURN_*` **刪除**，`ReturnShipmentStatusConsumer` 改用前者
  3. 🔴 未知值處置**對齊正向 leg**：`ShippingStatus.fromValue()` 失敗 ⇒ **fail-loud 進 DLQ**，
     不得 `else` ＋ ACK
  4. 一致性 gate：斷言分類器涵蓋的值域 ⊆ `ShippingStatus` 值域
- 🔴 **同批修正 66 §7.3.2 的 no-op 改名指示**：該節逐字寫「現名 `AfterSalesConstants.PAYUNI_RETURN_IN_TRANSIT`
  … ⇒ 同批改名 **`AfterSalesConstants.PAYUNI_RETURN_IN_TRANSIT`**」——**新舊同名**。
  照字面施工＝名字照樣說謊，而**下一個人看到「已經改過名了」就不會再懷疑它**。
  本 ADR 讓那一格失效：不是改名，是整組刪除並改用 canonical classifier。
- **不修的代價**：Uber `pickup_complete` → `930` → 落 `else`
  （`ReturnShipmentStatusConsumer.java:146`）⇒ 案件永停 `RETURN_PENDING(20)`
  ⇒ 7 天後 `AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)`，其 description 逐字
  「**本狀態為「不退款」終態**」⇒ 🔴 **買家已交件卻拿不到退款，零告警**。
- **已落實於**：09 詳設（逆向 leg 分類器章節）；66 詳設 §7.3.2 U5／U6。

## 修訂：設計書 65A、66、68、69 已刪除（2026-10-29）

物流履約前端實作規格（65A）、Uber Direct 整合（66）、7-ELEVEN 大宗寄倉（68）、商家自有出貨（69）的詳細與基本設計書已刪除：65A、68、69 未實作且不在任何計畫內，66 的 Uber 已由 ADR `2026-10-27-general-ecommerce-scope-cut.md` D4 決定刪除。本文中「65A」「66」「68」「69」號設計書與它們的章節編號指向已刪除的文件，內容見 git 歷史。設計書 65 只保留已實作的部分。
