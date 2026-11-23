# payment/internal/charge

付款：provider 中立的 port、付款列與嘗試歷程、建立、同步與補掃、付款事件的 inbox handler。計畫：`backend/docs/plans/2026-11-23-b2-payment.md` 步驟 3。

## 不變量

- 外部呼叫（provider、訂單 port）不在 transaction 內：先讀訂單，再一個 transaction 寫「建立中」的列、第一個嘗試與補做 job，提交後在請求內帶列上的冪等鍵建立 PaymentIntent。守門：fake 的 `Violations`、`TestCreatePaymentCreatesTheIntentAfterCommitOutsideTheTransactionWithTheStoredKey`。
- 失敗或逾時回 pending（`CREATING`），補做 job 與掃描以同一把鍵重送；呼叫前先記 `create_called_at`（列與有效嘗試，獨立提交）。補做 job 延到請求內全部上限之後（`createJobDelay`），掃描排的立刻跑。守門：`TestCreatePaymentWhenTheProviderFailsReturnsPendingAndTheJobRetriesWithTheSameKey`、`TestTheCatchUpJobIsScheduledAfterTheInRequestCallTimeout`。
- 搬錢呼叫（建立付款、轉帳、撤回、退款）的結果三分，查回才收斂（D68）：`ErrPermanent` 終局；`ErrRetryableRefusal` 只在這把鍵的第一次呼叫被拒絕才換新冪等鍵、不計期限，重送被拒絕保持原鍵；其餘不明，`ResendWindow`（23 小時，小於 `IdempotencyRetention`）內同鍵重送，之後只以 `Find*` 查回。守門：`TestAnUnknownTransferFoundByLookupCreatesNoSecondTransfer`。
- 查回找到收下，查無滿 `NotFoundAfter`才 FAILED；連續查回失敗、與 provider 不符、前置條件不成立進 `ANOMALY`：佔住唯一索引與退款額度、補掃跳過、發 `MoneyAnomaly`，由 `ResolveAnomaly` 人工結案（成功前向 provider 核對證據，同 transaction 寫 adminaudit）。守門：`TestAnUnknownTransferNotFoundIsRebuiltOnlyAfterTheNotFoundWait`、`TestRepeatedLookupFailuresPutTheTransferInAnomalyWhichHoldsTheUniqueSlot`、`TestResolvingATransferAnomalyWritesTheAuditInTheSameTransaction`。
- 一張訂單一列（`payments_order` 唯一索引）、同時只有一個有效 PaymentIntent（D8）：`payment_attempts` 每次嘗試一列，`superseded_at` 為空的是有效的（唯一索引守一個），付款列的外部 ID、冪等鍵、呼叫時刻是它的鏡像。`CREATE_FAILED` 或 `CANCELED` 且訂單仍可付時，買家再付同一列換新鍵重設，舊嘗試記被取代、新嘗試另寫一列。守門：`TestASecondCreatePaymentForTheSameOrderProducesNoSecondIntent`。
- 事件與同步依外部 ID 經歷程表對回付款列；快照寫進它所屬的嘗試，只有有效嘗試的快照寫付款列（舊 ID 不寫回重設的列）。舊 PaymentIntent 晚到成功（`lateOutcomeOf`）：列未入帳就記成功並發事件，它成為有效的、原有效的排取消（transaction 外）；列已入帳而它也成功則標 `excess_paid_at`（後台查，不自動退款）；列已入帳而它未結案則排取消。守門：`TestALateSuccessOfTheOldIntentAfterARepayIsRecordedAndTheNewIntentIsCanceled`。
- 改綁 provider：建立中、沒有外部 ID 的列只在舊 provider 從沒呼叫過時才改成目前的，呼叫過的進 `ANOMALY`；回快照的 provider 與嘗試的不同就丟棄。守門：`TestACreatingRowOfAnotherProviderThatWasCalledGoesToAnomalyInsteadOfBeingRebound`。
- 狀態只由快照推導（`Next`）：入帳是終局；取消後入帳由 provider 的成功勝出；不認得的狀態、金額不符不寫入。守門：`TestNextFollowsTheDecisionTable`、`TestSucceededIsNeverOverwrittenAndALateSuccessBeatsACancel`。
- handler 只排以付款 ID 為唯一鍵的同步 job、不標記已處理、不讀事件內容；同步重讀有效嘗試與未結案或有未處理事件的舊嘗試，同 transaction 發 `PaymentChanged`（狀態改變才發）並標記讀取開始前收到的事件。判斷過的快照與 anomaly 一律標記；入帳缺 charge 或入帳時間是暫時性的（`errIncompleteSuccess`）：不標記、job 重試。守門：`TestTheEventHandlerNeverTrustsTheEventContent`。
- 付款說明存在列上（至多 200 字，沒有商品名稱用訂單編號），重送時不變。守門：`TestPaymentDescriptionTrimsFallsBackToTheOrderNoAndCapsByRunes`。
- provider 名稱與收款帳戶不同，webhook 路由（`POST /webhooks/stripe-payments`）、簽章密鑰（`STRIPE_PAYMENT_WEBHOOK_SECRET`）、本文上限與 log 標籤都獨立。守門：`TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly`。
- 補掃：閒置的建立中列、久未讀的未結案付款、被取代又未結案的舊嘗試、有舊的未處理事件的付款（含舊 ID）。守門：`TestSweepQueuesIdleCreatesStaleReadsAndOldUnprocessedEvents`。
- client_secret 不存，`GetPayment` 在等待付款時向 provider 讀；只回自己的付款。守門：`TestGetPaymentReturnsTheSecretOnlyWhileAwaitingAndOnlyToTheOwner`。
- 轉帳：transaction 只寫 PENDING 列與 job，一筆付款一筆有效轉帳靠 `transfers_active_payment`（含 ANOMALY）。守門：`TestADuplicateTransferRequestReturnsTheExistingActiveTransfer`。撤回：同一個 transaction 累加 `reversed_amount`；provider 已撤回而退款不在 REVERSING 時記 log 並發告警，不靜默。守門：`TestAReversalForARefundThatIsNoLongerReversingIsReportedNotSilent`。
- schema 守轉帳與退款的不變量。守門：`TestTheSchemaGuardsTheTransferAndRefundInvariants`；狀態型別與 CHECK 逐項相同：`TestStatusTypesMatchTheSchemaChecks`、`TestAttemptStatesMatchTheSchemaCheck`。
- 不負責：訂單狀態與自動退款的判斷（訂單一方，D5 修訂）；逾期付款的取消 use case（步驟 4 的 `ClosePaymentByOrderNo`）。
