# payment/internal/charge

付款與退款：provider 中立的 port、付款列與嘗試歷程、建立、同步與補掃、inbox handler。計畫：`backend/docs/plans/2026-11-23-b2-payment.md` 步驟 3。

## 不變量

- 外部呼叫（provider、訂單 port）不在 transaction 內：先讀訂單，再一個 transaction 寫「建立中」的列、嘗試與補做 job，提交後在請求內帶列上的冪等鍵建立 PaymentIntent。守門：fake 的 `Violations`、`TestCreatePaymentCreatesTheIntentAfterCommitOutsideTheTransactionWithTheStoredKey`。
- 失敗或逾時回 pending（`CREATING`），補做 job 與掃描以同一把鍵重送；呼叫前先記 `create_called_at`（獨立提交）。補做 job 延到請求內上限之後（`createJobDelay`）。守門：`TestCreatePaymentWhenTheProviderFailsReturnsPendingAndTheJobRetriesWithTheSameKey`、`TestTheCatchUpJobIsScheduledAfterTheInRequestCallTimeout`。
- 搬錢呼叫（建立付款、轉帳、撤回、退款）的結果三分，查回才收斂（D68）：`ErrPermanent` 終局；`ErrRetryableRefusal` 只在這把鍵的第一次呼叫被拒絕才換新冪等鍵、不計期限，重送被拒保持原鍵；其餘不明，`ResendWindow`（23 小時）內同鍵重送，之後只以 `Find*` 查回。守門：`TestAnUnknownTransferFoundByLookupCreatesNoSecondTransfer`。
- 查回找到收下，查無滿 `NotFoundAfter`才 FAILED；連續查回失敗、與 provider 不符、前置條件不成立進 `ANOMALY`：佔住唯一索引與退款額度、補掃跳過、發 `MoneyAnomaly`，由 `ResolveAnomaly` 人工結案（成功前核對證據，同 transaction 寫 adminaudit）。守門：`TestAnUnknownTransferNotFoundIsRebuiltOnlyAfterTheNotFoundWait`。
- 一張訂單一列（`payments_order` 唯一索引）、同時只有一個有效 PaymentIntent（D8）：`payment_attempts` 每次嘗試一列，`superseded_at` 為空的是有效的，付款列的外部 ID、冪等鍵、呼叫時刻是它的鏡像。`CREATE_FAILED` 或 `CANCELED` 且訂單仍可付時，再付同一列換新鍵、新嘗試另寫一列。守門：`TestASecondCreatePaymentForTheSameOrderProducesNoSecondIntent`。
- 事件與同步依外部 ID 經歷程表對回付款列；快照寫進它所屬的嘗試，只有有效嘗試的快照寫付款列。舊 PaymentIntent 晚到成功（`lateOutcomeOf`）：列未入帳就記成功並發事件、原有效的排取消；列已入帳而它也成功則標 `excess_paid_at`（不自動退款）；未結案則排取消。守門：`TestALateSuccessOfTheOldIntentAfterARepayIsRecordedAndTheNewIntentIsCanceled`。
- 改綁 provider：建立中、沒有外部 ID 的列只在舊 provider 從沒呼叫過時才改成目前的，呼叫過的進 `ANOMALY`；快照的 provider 與嘗試的不同就丟棄。守門：`TestACreatingRowOfAnotherProviderThatWasCalledGoesToAnomalyInsteadOfBeingRebound`。
- 狀態只由快照推導（`Next`）：入帳是終局；取消後入帳由成功勝出；不認得的狀態、金額不符不寫入。守門：`TestNextFollowsTheDecisionTable`、`TestSucceededIsNeverOverwrittenAndALateSuccessBeatsACancel`。
- handler 只排同步 job（付款 ID 唯一鍵）、不標記已處理、不讀事件內容；同步重讀後同 transaction 發 `PaymentChanged`（狀態改變才發）並標記讀取開始前收到的事件；入帳缺 charge 或入帳時間是暫時性的（`errIncompleteSuccess`）：不標記、job 重試。守門：`TestTheEventHandlerNeverTrustsTheEventContent`。
- 付款說明存在列上（至多 200 字，沒有商品名用訂單編號）。守門：`TestPaymentDescriptionTrimsFallsBackToTheOrderNoAndCapsByRunes`。
- provider 名稱 `stripe_payments` 與收款帳戶不同，webhook 路由與簽章密鑰獨立。Stripe adapter（`stripe.go`）的錯誤三分依 HTTP 狀態，查回用強一致列表、不用 Search。守門：`TestAChargeProviderNamedLikeTheAccountProviderPanicsAtAssembly`、`TestStripeMoneyErrorsAreClassifiedIntoThreeWays`。
- 補掃：閒置的建立中列、久未讀的未結案付款、被取代又未結案的舊嘗試、有舊的未處理事件的付款。守門：`TestSweepQueuesIdleCreatesStaleReadsAndOldUnprocessedEvents`。
- client_secret 不存，`GetPayment` 等待付款時向 provider 讀。守門：`TestGetPaymentReturnsTheSecretOnlyWhileAwaitingAndOnlyToTheOwner`。
- 轉帳：transaction 只寫 PENDING 列與 job，有效轉帳唯一靠 `transfers_active_payment`（含 ANOMALY）。金額在每把冪等鍵第一次送出時鎖付款列算定（`claimTransfer`，D11），同鍵不變、換新鍵才重算；全額退款先等進行中的退款終局。守門：`TestAConcurrentRefundAndTheTransferSendNeverOverpay`。撤回：一次呼叫一列 `refund_reversals`。
- 退款：鎖付款列後以 `(付款, request_key)` 冪等、加總非失敗退款（含 ANOMALY）守累計上限，不看轉帳狀態；`REFUNDING` 由 `refund.*` 事件或掃描排的同步重讀到終局，發 `RefundChanged`；ANOMALY 分退款與撤回。收回：`reversal_status` 是彙總，轉帳金額已承諾才預留（含補差額轉帳），接受退款且轉帳成立後分列撤回，退款失敗或撤回終局釋出（規格 §12）。守門：`TestTwoConcurrentRefundsThatTogetherExceedThePaymentCreateExactlyOne`。鎖順序見規格 §7。
- 補差額：退款列 `topup_*`，規格 §10。
- schema 守轉帳與退款的不變量：`TestTheSchemaGuardsTheTransferAndRefundInvariants`；狀態型別與 CHECK 相同：`TestStatusTypesMatchTheSchemaChecks`、`TestAttemptStatesMatchTheSchemaCheck`。
- 不負責：訂單狀態與自動退款的判斷（訂單一方，D5 修訂）；逾期付款的取消 use case（步驟 4 的 `ClosePaymentByOrderNo`）。
