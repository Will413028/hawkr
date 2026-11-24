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
- `Resolver` 由根套件的 `NewResolver(buyer, seller)` 組成（店家讀 seller 的 `Store`，不用 legacy 的 `ResolvePrincipals`）；只有宣告 `Expand` 或 `Broadcast` 的型別用到。守門：`TestNewResolverReadsStoresAndStoreBlocksFromTheRealSeller`。
