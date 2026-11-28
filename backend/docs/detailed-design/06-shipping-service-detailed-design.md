# 06　物流微服務（Shipping Service）detailed-design

> 🔴 **已退場（B3，2026-11-24）。** Java shipping-service 整個刪除（含配送單狀態機、物流商建單與標籤、貨態處理、配送異常、門市地圖與開通、運費規則與假日處理），不搬進單體，也沒有等價的模組。

| 項目 | 說明 |
|---|---|
| 取代者 | 運費設定與報價歸 seller 模組（見 [21](21-shipping-fee-module-detailed-design.md)）；出貨記錄與含出貨的訂單狀態機歸 B8 ordering（見 [65](65-fulfillment-platform-detailed-design.md)）；退貨物流歸 B9 aftersales |
| 決策 | ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`（含 2026-11-24 修訂） |
| 執行計畫 | `backend/docs/plans/2026-11-24-b3-fulfillment.md`（B3 步驟 4、7） |

舊內容見 git 歷史；本檔保留編號，避免既有的設計書與 ADR 引用懸空。
