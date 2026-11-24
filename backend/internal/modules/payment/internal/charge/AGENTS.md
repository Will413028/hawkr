# payment/internal/charge

付款與退款：provider 中立的 port、付款列與嘗試歷程、建立、同步與補掃、inbox handler。計畫：B2 步驟 3。

## 不變量

- 外部呼叫（provider、訂單 port）不在 transaction 內：先讀訂單，再一個 transaction 寫「建立中」的列、嘗試與補做 job，提交後在請求內帶列上的冪等鍵建立 PaymentIntent。守門：fake 的 `Violations`、`TestCreatePaymentCreatesTheIntentAfterCommitOutsideTheTransactionWithTheStoredKey`。
- 失敗或逾時回 pending（`CREATING`），補做 job 與掃描以同一把鍵重送；呼叫前先記 `create_called_at`（獨立提交）；補做 job 延到請求內上限之後。守門：`TestCreatePaymentWhenTheProviderFailsReturnsPendingAndTheJobRetriesWithTheSameKey`。
- 搬錢呼叫的結果三分，查回才收斂（D68）：`ErrPermanent` 終局；`ErrRetryableRefusal` 只在這把鍵的第一次呼叫被拒絕才換新冪等鍵、不計期限；其餘不明，`ResendWindow`（23 小時）內同鍵重送，之後新舊鍵都不重送、只以 `Find*` 查回：帶 `created` 下限（第一次呼叫前記下的時刻減 `ListClockSkew`）、翻到列表結束，查回失敗不算證明不存在（D16）；401／403 仍是不明，另記 error log（`AuthRejectedError`）。守門：`TestAfterTheResendWindowAnUnknownCreateIsNeverReplayedWithoutProvenAbsence`。
- 查回找到收下，查無滿 `NotFoundAfter`才 FAILED；連續查回失敗、與 provider 不符、前置條件不成立進 `ANOMALY`：佔住唯一索引與退款額度、補掃跳過、發 `MoneyAnomaly`，由 `ResolveAnomaly` 人工結案（核對證據；原因代碼與備註必填，同 transaction 寫 adminaudit）。守門：`TestAnUnknownTransferNotFoundIsRebuiltOnlyAfterTheNotFoundWait`。
- 一筆付款對 N 張訂單應收（`payment_orders`，`order_no` 唯一；過渡期 N=1，D13）、同時只有一個有效 PaymentIntent（D8）；`transfer_group` 是付款 ID。`payment_attempts` 每次嘗試一列，`superseded_at` 為空的是有效的，付款列的外部 ID、冪等鍵、呼叫時刻是它的鏡像。`CREATE_FAILED` 或 `CANCELED` 且訂單仍可付時，同一列換新鍵重設，新嘗試另寫一列。守門：`TestASecondCreatePaymentForTheSameOrderProducesNoSecondIntent`。
- 事件與同步依外部 ID 經歷程表對回付款列；快照寫進它所屬的嘗試，只有有效嘗試的快照寫付款列。舊 PaymentIntent 晚到成功：列未入帳就記成功並發事件、原有效的排取消；列已入帳則標 `excess_paid_at`（不自動退款）；未結案則排取消。守門：`TestALateSuccessOfTheOldIntentAfterARepayIsRecordedAndTheNewIntentIsCanceled`。
- 改綁 provider：建立中、沒有外部 ID 的列只在舊 provider 從沒呼叫過時才改成目前的，呼叫過的進 `ANOMALY`。守門：`TestACreatingRowOfAnotherProviderThatWasCalledGoesToAnomalyInsteadOfBeingRebound`。
- 狀態只由快照推導（`Next`）：入帳是終局；取消後入帳由成功勝出；不認得的狀態、金額不符不寫入。守門：`TestNextFollowsTheDecisionTable`。
- handler 只排同步 job（付款 ID 唯一鍵）、不標記已處理、不讀事件內容；同步重讀後同 transaction 發 `PaymentChanged`（帶全部訂單應收）並標記讀取開始前收到的事件。守門：`TestTheEventHandlerNeverTrustsTheEventContent`。
- 付款說明存在列上（至多 200 字，沒有商品名用訂單編號）；client_secret 不存，`GetPayment` 等待付款時向 provider 讀。守門：`TestGetPaymentReturnsTheSecretOnlyWhileAwaitingAndOnlyToTheOwner`。
- provider 名稱 `stripe_payments` 與收款帳戶不同。Stripe adapter 的錯誤三分依 HTTP 狀態，查回用強一致列表、不用 Search。守門：`TestStripeMoneyErrorsAreClassifiedIntoThreeWays`。
- 補掃：閒置的建立中列、久未讀的未結案付款、被取代又未結案的舊嘗試、有舊的未處理事件的付款。守門：`TestSweepQueuesIdleCreatesStaleReadsAndOldUnprocessedEvents`。
- 轉帳：transaction 只寫 PENDING 列與 job；每張訂單應收最多一筆有效轉帳（`transfers_active_order`，含 ANOMALY），已有有效轉帳時重複請求一律回它（D-G）。金額在每把冪等鍵第一次送出時鎖付款列、訂單應收列算定（`claimTransfer`，D11：訂單應收減這張訂單的非失敗退款），同鍵不變；全額退款先等進行中的退款終局。守門：`TestTwoOrderReceivablesUnderOnePaymentGetIndependentTransfers`、`TestAConcurrentRefundAndTheTransferSendNeverOverpay`。撤回：一次呼叫一列 `refund_reversals`。
- 退款：鎖付款列、訂單應收列後以 `(訂單應收, request_key)` 冪等、加總這張訂單的非失敗退款（含 ANOMALY）守累計上限（訂單應收金額，不看轉帳、不看同付款別張訂單）；`REFUNDING` 由事件或掃描排的同步重讀到終局，發 `RefundChanged`。收回只落在這張訂單的轉帳，`reversal_status` 是彙總（規格 §12）。守門：`TestTwoOrderReceivablesUnderOnePaymentGetIndependentRefundCaps`、`TestARefundOnOrderANeverReversesOrderBsTransfer`。鎖順序（付款列、訂單應收列、轉帳列、退款列、撤回列）見規格 §7。補差額：退款列 `topup_*`，規格 §10。
- schema 守轉帳與退款的不變量：`TestTheSchemaGuardsTheTransferAndRefundInvariants`；狀態型別與 CHECK 相同：`TestStatusTypesMatchTheSchemaChecks`。
- 關閉：`CloseForOrder` 排 `payment.charge_close`，`Close` 在 transaction 外取消；建立中先等建立收斂（job snooze、不放棄）才取消，不留孤兒 PaymentIntent。
- 不負責：訂單狀態、逾期與自動退款的判斷（訂單一方，D5 修訂）。
