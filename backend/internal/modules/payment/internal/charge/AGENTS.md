# payment/internal/charge

付款：provider 中立的 port（PaymentIntent、退款、轉帳與撤回、付款事件）、付款列、建立（請求內建立、job 補做）、同步與補掃、付款事件的 inbox handler。計畫：`backend/docs/plans/2026-11-23-b2-payment.md` 步驟 3a；3b 轉帳、3c 退款、3d Stripe adapter 接在這個套件上（schema 與 port 已含它們）。

## 不變量

- 外部呼叫（provider、訂單 port）不在 transaction 內：`CreatePayment` 先 transaction 外讀訂單，再一個 transaction 寫「建立中」的列與補做 job，提交後在請求內帶列上的冪等鍵建立 PaymentIntent。守門：fake 的 `Violations`、`TestCreatePaymentCreatesTheIntentAfterCommitOutsideTheTransactionWithTheStoredKey`。
- 失敗或逾時回 pending（`CREATING`），補做 job 與週期掃描以同一把鍵（`create_idempotency_key`）重送；呼叫前先記 `create_called_at`（獨立提交）。守門：`TestCreatePaymentWhenTheProviderFailsReturnsPendingAndTheJobRetriesWithTheSameKey`、`TestAReplayAfterALostResponseRecoversTheSameIntent`。
- 結果不明（呼叫過、沒有外部 ID）超過 `UnresolvedDeadline`（12 小時，必須小於 `IdempotencyRetention` 24 小時，編譯期檢查）進終局 `CREATE_FAILED`，鍵不再重送；買家再次建立時同一列換新鍵重設。必要條件：provider 只保證冪等鍵的保存期（24 小時是保守值，3d 以 Stripe 文件核對）。守門：`TestAnUnresolvedCreatePastTheDeadlineIsTerminalAndTheKeyIsNeverReplayed`。
- 一張訂單一個 PaymentIntent（D8）：`payments_order` 唯一索引與 `ON CONFLICT (order_no)`。守門：`TestASecondCreatePaymentForTheSameOrderProducesNoSecondIntent`。
- 狀態只由 provider 的快照推導（`Next`）：入帳是終局、之後的快照不覆蓋；取消後入帳由 provider 的成功勝出；不認得的狀態、金額不符、入帳沒有 charge 不寫入。守門：`TestNextFollowsTheDecisionTable`、`TestSucceededIsNeverOverwrittenAndALateSuccessBeatsACancel`、`TestASnapshotThatDisagreesWithTheRowIsNeverWritten`。
- 付款事件的 handler 只排以付款 ID 為唯一鍵的同步 job、不標記已處理、不讀事件內容；同步在 transaction 外重讀，寫入同 transaction 發 `PaymentChanged`（狀態改變才發）並標記讀取開始前收到的事件。守門：`TestTheEventHandlerNeverTrustsTheEventContent`、`TestADuplicatePaymentEventIsRecordedOnceAndPublishesOnce`。
- provider 名稱與收款帳戶不同，webhook 路由與簽章密鑰獨立（`POST /webhooks/stripe-payments`）。守門：`TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly`、httpapi 的 `TestChargeWebhookIsSeparateFromTheAccountWebhook`。
- 補掃：閒置的建立中列、久未讀的未結案付款、有舊的未處理事件的付款。守門：`TestSweepQueuesIdleCreatesStaleReadsAndOldUnprocessedEvents`。
- client_secret 不存，`GetPayment` 在等待付款時向 provider 讀；只回自己的付款。守門：`TestGetPaymentReturnsTheSecretOnlyWhileAwaitingAndOnlyToTheOwner`。
- schema 守轉帳與退款的不變量（3b、3c 使用）：一筆付款最多一筆有效轉帳、撤回累計不超過轉帳金額、退款冪等鍵唯一、「撤回完成」要有撤回的外部 ID。守門：`TestTheSchemaGuardsTheTransferAndRefundInvariants`。狀態型別與 CHECK 逐項相同：`TestStatusTypesMatchTheSchemaChecks`。
- 不負責：訂單狀態與自動退款的判斷（訂單一方，D5 修訂；payment 只提供退款並標記原因）；逾期付款的取消 use case（`CancelPayment` 的 port 與 fake 已有，use case 在步驟 4 的 `ClosePaymentByOrderNo`）。
