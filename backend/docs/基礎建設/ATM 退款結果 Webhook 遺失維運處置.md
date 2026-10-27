# ATM 退款結果 Webhook 遺失維運處置（Runbook）

> 對應 issue #1353；設計依據見 [05 支付服務詳細設計書 §5.5.4.2](../詳細設計書/05-支付服務（Payment%20Service）詳細設計書.md)。
> 本文屬**維運處置**（§22：偵測歸 observability、回復歸維運，不寫 application code）。

## 觸發情境

告警 `AtmRefundStuckAwaitingWebhook`（Discord 業務告警頻道）：有 ATM 退款的 `refund_task` 停在 `AWAITING_WEBHOOK(15)` 超過 5 天。

**背景**：撥款前 ATM 退款走完第一階段（PayUni 已受理）後，終態收斂 100% 依賴 PayUni 主動推的「退款轉匯結果 webhook」（§6.7，型別碼 8255，帶 inner `RefundStatus`）。PayUni 對 ATM／線下退款**不提供任何狀態查詢 API**（[逆向金流 §2.4](../第三方整合/PayUni%20逆向金流%20API%20總覽.md)：`RefundStatus` 僅信用卡欄位），故此 webhook 遺失時無法自動反查收斂，需人工介入。正式環境 §6.7 正常 2–3 工作天到；逾 5 天多為真正遺失。

## 診斷步驟

> DB 位置：payment=`hawkr_payment`、aftersales=`hawkr_aftersales`、webhook=`hawkr_webhook`。dev/prod 各自的 Postgres。

1. **定位卡住的退款**（aftersales）：
   ```sql
   SELECT id, case_no, status, payment_method, external_ref_id, updated_at
   FROM refund_tasks
   WHERE status = '15' AND payment_method = '2004'
     AND updated_at < now() - interval '5 days';
   ```
   `external_ref_id` = payment 端的 `refund_no`（RFD…）。

2. **確認 §6.7 是否真的沒到**（webhook）：以該筆 `MerTradeNo`（`{orderNo}{序號}`）查 inbox 有無 8255：
   ```sql
   SELECT id, webhook_type, event_key, status, received_at
   FROM webhook_inbox
   WHERE webhook_type = '8255' AND event_key LIKE '%<MerTradeNo>%'
   ORDER BY id DESC;
   ```
   無 8255 = §6.7 未送達（確認缺口）；有但 status 非 processed = 另屬處理失敗問題，走 DLQ 檢視非本 runbook。

3. **權威核實**：登入 PayUni 代理商後台，查該 `merId`（`HAWK<accountId>`）UNi 帳戶交易，確認該筆退款轉匯**實際是否已完成 / 失敗**。

## 回復處置（依序）

**(A) 優先：請 PayUni 重送 §6.7**
- 向 PayUni 反映該筆（MerTradeNo / UNi 序號）退款結果 notify 未送達，請其重送。
- 重送的 §6.7 會流經既有冪等 webhook 路徑（payuni-webhook → payment `PayUniRefundNotifyConsumer` → `handleAtmRefundWebhook`），**自動收斂**：`payment_refunds`→SUCCEEDED(2)/FAILED(3)、`refund_task`→DONE(20)、並補發 `refund.succeeded`（exchange `payment.events`，觸發買賣家通知）。這是唯一乾淨路徑（不繞過單一寫入路徑、不漏事件）。

**(B) 次選（A 無法時）：人工校正**
- 僅在 PayUni 確認退款已完成、且無法重送 §6.7 時採用。
- ⚠️ 需人工補齊跨表狀態（`payment_refunds` / `refund_tasks` / `aftersale_cases`）**並**補發 `refund.succeeded` 事件（exchange `payment.events`），否則下游通知不會發出。此路徑繞過單一寫入路徑、易漏，列為 last resort，執行前先與後端確認完整清單。

## 為什麼不做自動對帳排程

見設計書 05 §5.5.4.2：PayUni 無 ATM 退款查詢 API（無權威來源可反查）＋ 無權威確認下自動設終態會違反「權威結果 lossless」原則（可能誤標未匯出為成功）。§22 亦規定偵測屬 observability、不寫進 application。故採「obs 告警 + 本 runbook 人工回復」，不引入 application reconciler。
