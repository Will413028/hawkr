# 24　節假日管理（Holiday Management）詳細設計書

> 🔴 **已退場（B3，2026-11-24）。** Java shipping 的 `HolidayService` 與 `holidays` 表（宅配出貨日與配達日的工作天計算）隨 shipping 刪除。主計畫原列的 4 支假日匯入 RPC 在程式中不存在，沒有要刪的 RPC。

| 項目 | 說明 |
|---|---|
| 取代者 | 假日資料的擁有者是單體 `internal/platform/bizcalendar`（A3 計畫 10f-4）；處理天數（工作日）的計算若需要行事曆，使用這個擁有者，不另存一份 |
| 決策 | ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`（含 2026-11-24 修訂） |
| 執行計畫 | `backend/docs/plans/2026-11-24-b3-fulfillment.md`（B3 步驟 4、7） |

舊內容見 git 歷史；本檔保留編號，避免既有的設計書與 ADR 引用懸空。
