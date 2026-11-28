# 22　商品配送驗證（Product Shipping Validation）detailed-design

> 🔴 **已退場（B3，2026-11-24）。** product 建立或更新 SKU 時呼叫 shipping 的 `ValidateProductShipping`（驗配送方式與材積限制）隨 Java shipping 刪除。

| 項目 | 說明 |
|---|---|
| 取代者 | 沒有等價的驗證：商品不再存配送方式與重量尺寸（ADR `docs/adr/2026-11-24-catalog-from-scratch-design.md` D10，B3 P6）；運費只看賣家的固定規則與商品小計 |
| 決策 | ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`（含 2026-11-24 修訂） |
| 執行計畫 | `backend/docs/plans/2026-11-24-b3-fulfillment.md`（B3 步驟 4、7） |

舊內容見 git 歷史；本檔保留編號，避免既有的設計書與 ADR 引用懸空。
