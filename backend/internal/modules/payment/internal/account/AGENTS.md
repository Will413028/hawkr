# payment/internal/account

收款帳戶：provider 中立的 port、帳戶快取、建立／同步／輪詢／關閉後刪除 job，以及收款事件的 inbox handler。

## 不變量

- 外部呼叫（建立帳戶、重讀帳戶、開通 session）不在 transaction 內；一個擁有者一個建立 job、一個同步 job（唯一鍵 `store_id`）。守門：`TestJobsAreUniquePerOwnerInRiver`。
- 狀態只由帳戶快照推導（`StatusOf`），沒有觀測歷史與事件時間排序，重讀即最新。守門：`TestStatusOfFollowsTheDecisionTable`。
- 快取不倒退（D32）：`ApplyPaymentSnapshot` 只在這次讀取的開始時間不早於已存的 `fetched_at`（NULL 一律寫）時寫入；被擋下的同步不標記 inbox 事件、不發布 `PayoutAccountChanged`，`again` 由唯讀的 `RoutePending` 回答（較晚的讀取之後才到的事件不留給輪詢）。必要條件：除了 job 之外還有同步呼叫（seller 的 `RefreshMyPaymentAccount` 呼叫 `SyncAccountUnlessFresh`），可能與 job 並行，「一個帳戶一個同步」只剩 River 唯一鍵；那個呼叫拿掉時這個條件一起拿掉。守門：`TestAnOlderReadNeverOverwritesANewerCommittedRead`、`TestARejectedSyncDoesNotMarkEventsOrPublish`。
- 賣家觸發的重讀間隔（D33）：`SyncUnlessFresh` 先以條件式 UPDATE（`ClaimPaymentAccountSellerRead`）搶名額再讀 provider，搶不到不讀 provider：快取已涵蓋占名額的讀取（`fetched_at >= seller_read_claimed_at`）才成功，否則 `ResourceExhausted` 帶剩餘時間；失敗與並行的讀取都占名額，用搶到的那一列同步（不重讀），`again` 為 true 時自己排同步 job；同步 job 不經過這裡。必要條件：外部讀取由賣家觸發、頻率不受 job 唯一鍵約束；那個呼叫拿掉時間隔與 `seller_read_claimed_at` 欄一起拿掉。守門：`TestBusyRetryAfterDecidesBetweenSuccessAndRetry`、`TestConcurrentSellerRefreshesReadTheProviderOnce`、`TestSellerTriggeredRereadIsDebouncedAndFailsLoudly`。
- `RequestAccount` 在擁有者的 transaction 內只 INSERT（`ON CONFLICT DO NOTHING`）並排 job，不鎖既有列。守門：`TestRequestKeepsAnExistingRow`。
- 聯絡信箱不存：建立 job 在呼叫 provider 的當下才經 `Owners.OwnerContactEmail` 讀，沒有信箱不呼叫 provider、記建立失敗。守門：`TestEnsureReadsTheOwnersContactEmailAtCreationTime`、`TestEnsureWithoutAContactEmailRecordsACreateFailureAndNeverCallsTheProvider`。
- 清理是「關閉後刪除」（唯一鍵 `store_id`）：有外部 ID 時 provider 外呼叫關閉、成功才刪列，每一列（有或沒有外部 ID）呼叫 provider 前先在自己的 transaction 標 `purge_pending_at`（`purgeNeedsMark`）；沒有外部 ID 且 provider 相同時以冪等鍵取得外部帳戶（`ResolveAccount`）、關閉、再刪（D55），失敗留列由週期掃描重排；provider 不符或沒有設定：有外部 ID 的不動，沒有的直接刪。刪列前鎖列再檢查（`lockedRowMayBeDeleted`）。守門：`TestPlanPurgeDeletesRowsWithoutExternalIDClosesMatchingProviderAndDefersTheRest`、`TestPurgeMarksEveryRowBeforeAnyProviderCall`、`TestPurgeDeletesRowsWithoutAnExternalAccountAndKeepsRowsOfAnotherProvider`。
- 標記的列（`purge_pending_at`，有沒有外部 ID 都一樣）不被建立（建立 job、閒置補排、管理員重試建立）、不同步、不開通 session。守門：`TestCreateIsWantedOnlyForAnUnmarkedRowWithoutExternalIDOrFailure`。
- 同步在寫快照的 transaction 內發 `PayoutAccountChanged` 並以 platform inbox 標記「讀取開始前」收到的事件；handler 只排同步、不標記（對不到帳戶的除外），被合併的同步靠 `RoutePending` 再跑一輪；輪詢對有超過寬限（`eventGrace`）未處理事件的帳戶補排同步。守門：`TestAnEventArrivingWhileTheAccountIsBeingReadIsNotLostWhenItsSyncIsCoalesced`、`TestSweepQueuesSyncForAnOldUnprocessedEventOnAnEnabledAccount`。
- 輪詢也排 DISABLED 但原因碼不是 `rejected*`、`unsupported*` 的帳戶。守門：`TestSweepQueuesSyncForOpenAndLiftableStatusesOnly`。
- 建立外部帳戶的冪等鍵是「建立中」列寫入時產生並保存的 UUID，每次重試沿用；收件方式取自列上的 `requirement_collection`。守門：`TestRequestQueuesOneAccountCreationAndAReplayedCreateKeepsOneExternalAccount`、`TestStripeEnsureAccountUsesTheDashboardOfTheStoredCollection`。
- 建立分兩步（D55）：第一步只帶 `metadata.store_id` 與列上的冪等鍵（本文固定），第二步更新寫入當下的信箱與設定；兩步都成功才回 ID，Fake 一步完成但同一把鍵回同一個帳戶。守門：`TestStripeEnsureAccountSendsTheGivenKeyAndAFixedCreateBodyOnEveryAttempt`、`TestARetryAfterALostCreateResponseWithAnotherEmailReusesTheAccountAndNeverHitsAnIdempotencyError`。
- 擁有者不在營運中（`Owners`）時建立 job 與補排都略過。設定了 provider 時 `Owners` 與 `Inbox` 必填。守門：`TestEnsureSkipsAnInactiveOwner`、`TestNewRequiresOwnersAndRegistersTheProviderWithTheInbox`。
- 明文 fingerprint 與事件 payload 不保存在本模組、不寫 log。
