# 10　PayUni Webhook 服務（payuni-webhook-service）詳細設計書

> 🔴 **已退場（B3，2026-11-24）。** `payuni-webhook-service` 整個刪除：金流收款改 payment port（B2，Stripe 的 webhook 先寫 inbox、以事件 ID 去重），物流貨態隨出貨改賣家自寄不再存在。

| 項目 | 說明 |
|---|---|
| 取代者 | 收款 webhook 歸 B2 payment（見 `docs/adr/2026-11-13-payment-onboarding-provider-port-stripe.md`）；沒有物流 webhook |
| 決策 | ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`（含 2026-11-24 修訂） |
| 執行計畫 | `backend/docs/plans/2026-11-24-b3-fulfillment.md`（B3 步驟 4、7） |

舊內容見 git 歷史；本檔保留編號，避免既有的設計書與 ADR 引用懸空。
