# payment/internal/charge

付款：provider 中立的 port（PaymentIntent、退款、轉帳與撤回、付款事件）、付款列、建立（請求內建立、job 補做）、同步與補掃、付款事件的 inbox handler。計畫：`backend/docs/plans/2026-11-23-b2-payment.md` 步驟 3a；3b 轉帳、3c 退款、3d Stripe adapter 接在這個套件上（schema 與 port 已含它們）。

## 不變量

- 外部呼叫（provider、訂單 port）不在 transaction 內：`CreatePayment` 先 transaction 外讀訂單，再一個 transaction 寫「建立中」的列與補做 job，提交後在請求內帶列上的冪等鍵建立 PaymentIntent。守門：fake 的 `Violations`、`TestCreatePaymentCreatesTheIntentAfterCommitOutsideTheTransactionWithTheStoredKey`。
- 失敗或逾時回 pending（`CREATING`），補做 job 與週期掃描以同一把鍵（`create_idempotency_key`）重送；呼叫前先記 `create_called_at`（獨立提交）。守門：`TestCreatePaymentWhenTheProviderFailsReturnsPendingAndTheJobRetriesWithTheSameKey`、`TestAReplayAfterALostResponseRecoversTheSameIntent`。
- 補做 job 排在請求內呼叫上限之後（`createJobDelay`），掃描排的立刻執行；請求結束時列是 AWAITING_PAYMENT 卻沒有 secret（補做先寫入）改走 `view` 取 secret。守門：`TestEnqueueCreateAtDelaysTheJobOnlyWhenAskedTo`、`TestTheCatchUpJobIsScheduledAfterTheInRequestCallTimeout`、`TestACatchUpJobThatWritesFirstStillLetsTheRequestReturnTheSecret`。
- 結果不明（呼叫過、沒有外部 ID）超過 `UnresolvedDeadline`（12 小時，必須小於 `IdempotencyRetention` 24 小時，編譯期檢查）進終局 `CREATE_FAILED`，鍵不再重送；買家再次建立時同一列換新鍵重設（`CANCELED` 且訂單仍可付的列同樣重設：清外部 ID、新鍵，守 D8；守門：`TestACanceledPaymentOfAStillPayableOrderGetsANewIntentOnTheSameRow`）；建立中、沒有外部 ID 的列屬於別的 provider 時補做改成目前的（`TestACreatingRowOfAnotherProviderIsReboundByTheCatchUp`）。必要條件：provider 只保證冪等鍵的保存期（24 小時是保守值，3d 以 Stripe 文件核對）。守門：`TestAnUnresolvedCreatePastTheDeadlineIsTerminalAndTheKeyIsNeverReplayed`。
- 一張訂單一個 PaymentIntent（D8）：`payments_order` 唯一索引與 `ON CONFLICT (order_no)`。守門：`TestASecondCreatePaymentForTheSameOrderProducesNoSecondIntent`。
- 狀態只由 provider 的快照推導（`Next`）：入帳是終局、之後的快照不覆蓋；取消後入帳由 provider 的成功勝出；不認得的狀態、金額不符、入帳沒有 charge 不寫入。守門：`TestNextFollowsTheDecisionTable`、`TestSucceededIsNeverOverwrittenAndALateSuccessBeatsACancel`、`TestASnapshotThatDisagreesWithTheRowIsNeverWritten`。
- 付款事件的 handler 只排以付款 ID 為唯一鍵的同步 job、不標記已處理、不讀事件內容；同步在 transaction 外重讀，寫入同 transaction 發 `PaymentChanged`（狀態改變才發）並標記讀取開始前收到的事件。判斷過的快照（含保留終局列的）一律標記事件已處理，只有讀取過期不標；anomaly 也標記（已告警、列不動）。守門：`TestTheEventHandlerNeverTrustsTheEventContent`、`TestADuplicatePaymentEventIsRecordedOnceAndPublishesOnce`、`TestASnapshotThatKeepsTheSucceededRowStillMarksItsEventsProcessed`、`TestAnAnomalyMarksItsEventsProcessedAndLeavesTheRowAlone`。
- 付款說明存在列上（`description`，至多 200 字，沒有商品名稱用訂單編號），送 provider 的內容在冪等重送時不變。守門：`TestPaymentDescriptionTrimsFallsBackToTheOrderNoAndCapsByRunes`、`TestThePaymentDescriptionIsStoredAndSentUnchangedOnReplay`。
- provider 名稱與收款帳戶不同，webhook 路由與簽章密鑰獨立（`POST /webhooks/stripe-payments`）；簽章密鑰是自己的設定（`STRIPE_PAYMENT_WEBHOOK_SECRET`，與收款帳戶的不同，cmd/hawkr 的 `readChargeStripeConfig`），本文上限與 log 標籤也是付款自己的。守門：`TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly`、httpapi 的 `TestChargeWebhookIsSeparateFromTheAccountWebhook`、`TestChargeWebhookAcceptsSnapshotSizedBodiesButStillCapsThem`、cmd/hawkr 的 `TestChargeProviderConfigHasItsOwnWebhookSecret`。
- 補掃：閒置的建立中列、久未讀的未結案付款、有舊的未處理事件的付款。守門：`TestSweepQueuesIdleCreatesStaleReadsAndOldUnprocessedEvents`。
- client_secret 不存，`GetPayment` 在等待付款時向 provider 讀；只回自己的付款。守門：`TestGetPaymentReturnsTheSecretOnlyWhileAwaitingAndOnlyToTheOwner`。
- schema 守轉帳與退款的不變量（3b、3c 使用）：一筆付款最多一筆有效轉帳、撤回累計不超過轉帳金額、退款冪等鍵唯一、「撤回完成」要有撤回的外部 ID；退款與轉帳的外部 ID 在 provider 已回應的狀態（退款 REFUNDING、FAILED、SUCCEEDED；轉帳 FAILED、SUCCEEDED）都存得下來。守門：`TestTheSchemaGuardsTheTransferAndRefundInvariants`、`TestTheSchemaStoresTheExternalIDOfEveryStateThatHasOne`、`TestExternalIDsAreAllowedInEveryStateTheProviderHasAlreadyAnswered`。狀態型別與 CHECK 逐項相同：`TestStatusTypesMatchTheSchemaChecks`。
- 不負責：訂單狀態與自動退款的判斷（訂單一方，D5 修訂；payment 只提供退款並標記原因）；逾期付款的取消 use case（`CancelPayment` 的 port 與 fake 已有，use case 在步驟 4 的 `ClosePaymentByOrderNo`）。
