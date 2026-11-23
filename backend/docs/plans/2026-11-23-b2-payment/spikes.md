# B2 步驟 1 spike：Stripe 付款、轉帳與退款的實際行為

2026-11-23 在 Stripe sandbox 實測，`Stripe-Version: 2026-10-28.endive`（stripe-go v87.1.0 釘住的版本），平台帳戶 US／USD，收款方是已開通 `stripe_transfers: active` 的 US v2 recipient 帳戶。回應樣本（遮罩後）在 `internal/platform/stripe/testdata/`，檔名見該目錄 `README.md` 的 B2 段。每條標證據等級：**實測**（sandbox 呼叫結果）、**文件**（Stripe 文件原文，附出處）、**推論**。

## 停止條件

未觸發：recipient 帳戶收得到帶 `source_transaction` 的轉帳（實測）；文件沒有寫轉帳距付款的時間上限，帶 `source_transaction` 的轉帳會等該筆付款的款項入帳才執行（文件，separate-charges-and-transfers）。D1 不需重審。

## 付款、轉帳、退款

| # | 情境 | 結果 | 等級 | 樣本 |
|---|---|---|---|---|
| 1 | PaymentIntent 帶 `transfer_group`，測試卡確認 | `succeeded`，`latest_charge` 有值 | 實測 | `payment_intent_confirmed` |
| 2 | 付款尚在 pending（未入帳）時建立帶 `source_transaction` 的轉帳 | 200，立刻建立 | 實測 | `transfer_source_transaction` |
| 3 | 完成後退款：先部分撤回轉帳，再退款 | 撤回 200、退款 `succeeded` | 實測 | `transfer_reversal_partial`、`refund_after_transfer` |
| 4 | 撤回超過轉帳剩餘可撤金額 | 400，訊息含剩餘金額；`error.code` 為空，只能靠 HTTP 狀態與 `type` 分類 | 實測 | `transfer_reversal_exceeds_remaining` |
| 5 | 完成前部分退款後，轉帳超過付款剩餘金額 | 400「must not exceed the source amount」；全額退款後再轉帳也是 400 | 實測 | `transfer_exceeds_refunded_source` |
| 6 | 已轉帳但不撤回就全額退款 | `succeeded`，由平台餘額出錢（平台 pending 變負） | 實測 | — |
| 7 | 退款會不會自動撤回 SCT 的轉帳 | 不會：#6 退款成功後轉帳原封不動，撤回必須另呼叫。`reverse_transfer` 參數屬 destination charge，SCT 不用（文件；帶這參數的呼叫只測到「已全額退款」的 400，未單獨測） | 實測（#6）＋文件 | — |
| 8 | 平台總餘額（available＋pending，含 #6 造成的負數）低於轉帳金額 | 帶 `source_transaction` 仍 400「insufficient available funds」 | 實測（原因是推論：#6 之後才出現，小額重試成功） | `transfer_platform_insufficient_funds` |
| 9 | 賣家已把餘額全部提領後再撤回轉帳 | 200，賣家 available 變成負數；不是錯誤 | 實測 | `transfer_reversal_recipient_drained` |
| 10 | 以 TWD 收款再以 TWD 轉帳 | 400：付款以 USD 入帳，轉帳幣別必須等於入帳幣別 | 實測 | `transfer_currency_mismatch` |

對計畫的影響：

- D1 寫的「賣家餘額不足由平台吸收」要改寫：Stripe 不拒絕撤回，而是讓賣家餘額變負（#9）；平台承擔負餘額（文件：SCT 只建議在平台承擔負餘額時使用）。步驟 3 的撤回 job 不需要「餘額不足」分支，但要讓後台看得到賣家負餘額（推論）。
- 退款不撤回轉帳就會吃平台餘額，並可能讓之後的轉帳失敗（#6、#8）：步驟 3 完成後退款必須「先撤回、再退款」，轉帳 job 要把「平台餘額不足」當可重試的錯誤。
- 轉帳金額上限是付款扣掉已退款後的金額（#5），Stripe 自己守住，Go 端仍以累計上限守（D4）。
- 錯誤分類不能靠 `error.code`（#4、#5 為空）：adapter 以 HTTP 狀態與 `error.type` 分類，訊息只進 log。
- 幣別（#10）是新的決定題 D9，見計畫。

## 付款事件的 event destination

- 一個 destination 只能是一種 payload：`event_payload: snapshot` 的 destination 放入 v2 thin 事件被拒（400「contains 'thin' event types when event_payload is 'snapshot'」）。付款事件（`payment_intent.*`、`refund.updated`、`transfer.*`）要另建一個 snapshot destination，與帳戶的 thin destination 分開（實測，樣本 `event_destination_snapshot_rejects_thin`）。
- 建立時以 `include: ["webhook_endpoint.signing_secret"]` 取回該 destination 自己的簽章密鑰（實測：有值，樣本已遮罩）；兩個 destination 各一把，webhook 端要分開驗證（推論自上述；文件只寫密鑰「associated with your endpoint」）。
- 每個帳戶最多 16 個 destination（文件，event-destinations）。
- 本機轉發 `stripe listen` 已在 A3b 用過（已確認）。

spike 建的 destination 已 disable 並刪除（實測 200／200）。

## D3 的事實（買家付款介面）

- Payment Element 搭配平台自己的 PaymentIntent（帶 `transfer_group`、不帶 `on_behalf_of`）是 SCT 文件的標準寫法（文件，separate-charges-and-transfers 的 Payment Intents＋Elements 版本）。
- PaymentSheet：SCT 的 PaymentIntent 是普通的平台 PaymentIntent，沒有 connected account 參數，推論 PaymentSheet 無額外限制；行動版文件未讀（未核對）。
- `flutter_stripe`：授權 MIT；最新 15.0.0 約 2026-11-14 發佈（未滿兩週）；滿兩週的最新版是 14.2.0（約 2026-10-15）（文件，pub.dev versions 頁，日期是相對時間換算）。README 寫 Stripe 將淘汰 CardField／CardForm、改用 PaymentSheet。
- 推翻 D3 建議 A 的事實：未發現。

## 刪除清單的呼叫者核對（前提段「步驟 1 以 rg 核對」）

指令：`rg -l "\b<類別>\b" services --type java`（排除類別本身與測試）。結論「一併刪」成立，但呼叫者比前提段寫的多，全部是 PayUni 專屬路徑：

- `DuplicatePaymentService`：`PaymentNotifyHandler`（PayUni 付款通知）、`PendingPaymentReconcileService`、`RescueRefundConsumer`、`OutboxEventFactory`／`OutboxEventEntity`／`PaymentTransactionMapper`（duplicate 事件的型別與欄位）。
- `PaymentRedirectFormStore`：`PaymentPageService`、`PaymentGrpcService:609`（PayUni 導轉表單 RPC）。
- `GrantResponseRules`：`PaymentApplicationService:548–816`（PayUni 撥款回應三分類與撥款查詢對帳），由 Stripe 轉帳的冪等鍵取代。
- `PayUniAuditLogger`：`PayUni*Consumer`、`PayUniConfig`、`PaymentGrpcService:132`。
