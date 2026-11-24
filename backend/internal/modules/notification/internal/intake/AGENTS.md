# notification/internal/intake

意圖管線：來源 transaction 內只排意圖，worker 展開。規格：計畫 `backend/docs/plans/2026-11-23-a8-notification.md` 的 A8-1、A8-3a2。

## 不變量

- 同步訂閱者（`subscribers.go`、`subscribers_content.go`）只呼叫 `Accept`：插 `intents` 列並排 `notification.process_intent`；token 清除訂閱者是白名單。`Accept` 不接受 `Spawned` 型別（它沒有自己的冪等鍵）。守門：`TestSynchronousSubscribersOnlyAccept`、`TestAcceptOnlyInsertsTheIntentAndEnqueues`。
- `Process` 在 transaction 外讀意圖並解析收件人（`Entry.Expand`：參數裡有帳號的型別等同 `Resolve`；只帶 `store_id`、`buyer_id` 的型別經 `registry.Resolver` 讀 buyer、seller）。沒有存活收件人、actor 帳號就是收件人時略過。再開 transaction：鎖帳號、買家或店家，然後 `Blocked` 檢查收件人與 actor 之間的封鎖。略過時同 transaction 把意圖參數換成 `'{}'`（D16）；重試時意圖已有收件匣連結或投遞紀錄就不清（`IntentHasRecipientRows`）。
- 守門：`TestStoreEventResolvesToTheOwnerAccountAndStore`、`TestSelfNotificationIsSkipped`、`TestRetryAfterABlockDoesNotScrubAnAlreadyDeliveredIntent`。
- 參數裡的 ID 小於等於 0 或依賴模組回 InvalidInput 是永久錯誤（`ErrBadParams`，取消 job）：`TestProcessCancelsAnIntentWithAnInvalidBuyerID`。
- 封鎖略過：買家與買家（任一方向）、買家與店（買家封鎖店，或店封鎖買家）；店與店沒有；讀不到封鎖時回錯誤讓 job 重試，不當作沒封鎖。守門：`TestBlockedPairsAreSkippedInBothDirections`、`TestBlockedCoversEveryPairAndDirection`、`TestBlockedReturnsReadErrorsInsteadOfDelivering`。
- fan-out：母意圖型別宣告 `Broadcast`，`Process` 遇到只排 `notification.fanout_intent`；`Fanout` 每頁呼叫 `Targets`（收 `PageRequest`：游標、頁大小夾到 `registry.MaxFollowerPage`、母意圖事件時間；只通知事件發生時已追蹤的人），回傳的游標與是否結束由底層分頁決定、不由收件人數推得（型別濾掉整頁照樣往下一頁），每位收件人一個子意圖（子型別宣告 `Spawned`，冪等鍵 `registry.SpawnedKey`＝母意圖鍵加收件人 ID）與一個 `process_intent` job，下一頁的 job 同一個 transaction。
- 排頁 job 的唯一鍵不含 Completed、含 Running，只省工；正確性靠子意圖冪等鍵，重跑任何一頁都不重複。
- 守門：`TestFanoutPagesFollowersAndRerunningCreatesNoDuplicates`、`TestFanoutPageJobStatesFollowRiverRulesAndExcludeCompleted`、`TestRiverAcceptsTheFanoutInsertOpts`、`TestFanoutContinuesPastAPageThatTheTypeFilteredEmpty`、`TestFanoutSkipsLateFollowersAndClampsAnOversizedChunk`。
- `Resolver` 由根套件的 `NewResolver(buyer, seller, reporters)` 組成（店家讀 seller 的 `Store`，不用 legacy 的 `ResolvePrincipals`）；只有宣告 `Expand` 或 `Broadcast` 的型別用到。守門：`TestNewResolverReadsStoresAndStoreBlocksFromTheRealSeller`。
- 治理與交易側的同步訂閱者在 `subscribers_governance.go`（`subscribe_governance.go` 登記，每個事件一個）：每個函式語法上剛好一處 `Accept`，只准加上 `governance_intents.go` 的純函式（由事件決定型別與參數，沒有 I/O）。措施請求的裁定（接受與駁回）都登記 `store_measure_request_decided`，接受申覆的撤銷（`RevokeAppealAccepted`）與接受解除請求的結束（帶 `RequestID`）不另登記（D3，同一次處置一則）；作者通知只給買家作者。
- 治理訂閱者的守門：`TestGovernanceSubscribersOnlyAccept`、`TestGovernanceIntentHelpersArePure`、`TestRequestDecidedIntentRegistersEveryDecisionAsRequestDecided`、`TestMeasureRevokedAndEndedSkipTheAcceptedRequestDisposition`、`TestAuthorNoticesAreOnlyForBuyerAuthors`。
- `Resolver.CaseReporters`（根套件的實作從游標接著讀 moderation 那一邊的檢舉人，游標是檢舉人號碼）給結案的檢舉人 fan-out；worker 組 `moderation.CaseReader`，serve 組完整 `moderation.Module`。
- 行銷類推播（偏好分類屬行銷）在 `deliver` 寫投遞紀錄時，台北 22:00（含）到 09:00（不含）建立的，渠道 job 以 `ScheduledAt` 延到下一個 09:00（`QuietHoursEnd`，D26），收件匣列不延後；時間取注入的 `Now`。`voucher_expiring` 每券一則、冪等鍵取 `voucher_id`（D27），送出前由 `deliver` 以 `Entry.StillValid` 重查券仍可用，否則投遞標 `no_longer_valid`。守門：`TestMarketingPushIsHeldDuringQuietHoursButTheInboxRowIsImmediate`、`TestVoucherNoLongerAvailableAtSendTimeSendsNoPush`、`TestQuietHoursEnd`。
