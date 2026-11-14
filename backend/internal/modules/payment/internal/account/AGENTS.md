# payment/internal/account

收款帳戶：provider 中立的 port、帳戶快取、建立／同步／輪詢／關閉後刪除 job，以及收款事件的 inbox handler。

## 不變量

- 外部呼叫（建立帳戶、重讀帳戶、開通 session）不在 transaction 內；一個擁有者一個建立 job、一個同步 job（唯一鍵 `store_id`）。守門：`TestJobsAreUniquePerOwnerInRiver`。
- 狀態只由帳戶快照推導（`StatusOf`），沒有觀測歷史與事件時間排序，重讀即最新。守門：`TestStatusOfFollowsTheDecisionTable`。
- 快取不倒退（D32）：`ApplyPaymentSnapshot` 只在這次讀取的開始時間不早於已存的 `fetched_at`（NULL 一律寫）時寫入；被擋下的同步不標記 inbox 事件、不發布 `PayoutAccountChanged`。必要條件：seller 的 `RefreshMyPaymentAccount` 呼叫 `SyncAccountUnlessFresh`，可能與 job 並行，「一個帳戶一個同步」只剩 River 唯一鍵；那個呼叫拿掉時一起拿掉。守門：`TestAnOlderReadNeverOverwritesANewerCommittedRead`、`TestARejectedSyncDoesNotMarkEventsOrPublish`。
- 賣家與管理員觸發的重讀間隔（D33）：`SyncUnlessFresh` 先以條件式 UPDATE（`ClaimPaymentAccountSellerRead`）搶名額再讀 provider，搶不到不讀，`ResourceExhausted` 帶剩餘時間。必要條件：外部讀取由賣家或管理員觸發、不受 job 唯一鍵約束；那些呼叫拿掉時間隔與 `seller_read_claimed_at` 一起拿掉。守門：`TestBusyRetryAfterDecidesBetweenSuccessAndRetry`、`TestConcurrentSellerRefreshesReadTheProviderOnce`、`TestSellerTriggeredRereadIsDebouncedAndFailsLoudly`。
- 管理員的重新整理（D49-3）：`Refresh` 檢查帳戶狀態後呼叫同一個 `SyncUnlessFresh`並回傳重讀後的帳戶，不另排 job。守門：`TestAdminRefreshRereadsSynchronouslyThroughTheSellerPath`。
- `RequestAccount` 在擁有者的 transaction 內只 INSERT（`ON CONFLICT DO NOTHING`）並排 job，不鎖既有列。守門：`TestRequestKeepsAnExistingRow`。
- 聯絡信箱不存：建立 job 在呼叫 provider 的當下才經 `Owners.OwnerContactEmail` 讀，沒有信箱不呼叫 provider、記建立失敗。守門：`TestEnsureReadsTheOwnersContactEmailAtCreationTime`、`TestEnsureWithoutAContactEmailRecordsACreateFailureAndNeverCallsTheProvider`。
- 清理是「關閉後刪除」（唯一鍵 `store_id`，D52、D59）：呼叫 provider 前先標 `purge_pending_at`；`planPurge`：從沒呼叫直接刪；有外部 ID 就關閉再刪；呼叫過沒有 ID：7 天（`UnresolvedDeadline`）內以同一把鍵重送取回 ID 再關閉，超過進終局（D63）；問不到 provider 留列。
- 清理的終局（D59、D60）：永久拒絕、結果不明逾期（`unresolved_creation`）或標記後 7 天（`PurgeDeadline`）仍不成，列進 `purge_blocked_at`：掃描不再重排，進待辦 `PAYMENT_ACCOUNT_PURGE_STUCK`，管理員重試關閉或刪除對應（寫 `adminaudit`，D62）。守門：`TestAnUnresolvedCreatePast7DaysReachesTheTerminalStateAndNothingIsEverReplayedPastTheRetention`、`TestAPermanentCloseRefusalPutsTheRowInTheTerminalStateAndTheSweepStops`。
- 標記的列（`purge_pending_at`，有沒有外部 ID 都一樣）不被建立、不同步、不開通 session。守門：`TestCreateIsWantedUntilTheSecondStepSucceeds`。
- 同步在寫快照的 transaction 內發 `PayoutAccountChanged` 並以 platform inbox 標記「讀取開始前」收到的事件；handler 只排同步、不標記，被合併的同步靠 `RoutePending` 再跑一輪；輪詢補排有舊的未處理事件的帳戶。守門：`TestAnEventArrivingWhileTheAccountIsBeingReadIsNotLostWhenItsSyncIsCoalesced`、`TestSweepQueuesSyncForAnOldUnprocessedEventOnAnEnabledAccount`。
- 輪詢也排 DISABLED 但原因碼不是 `rejected*`、`unsupported*`、`closed`（已在 provider 關閉）的帳戶；沒有 provider 時只重排清理。守門：`TestSweepQueuesSyncForOpenAndLiftableStatusesOnly`。
- 建立外部帳戶的冪等鍵是「建立中」列寫入時保存的 UUID，每次重送沿用：週期掃描重送到 `UnresolvedDeadline` 為止；管理員重試在 `IdempotencyRetention` 內仍重送同一把鍵，超過才在確認後換新鍵（D63）。守門：`TestRequestQueuesOneAccountCreationAndAReplayedCreateKeepsOneExternalAccount`、`TestStripeConfigureAccountUsesTheDashboardOfTheStoredCollection`、`TestRetryCreateWithinTheKeyRetentionReplaysTheSameKeyWithoutAcknowledgement`、`TestRetryCreateOfAnUnresolvedTerminalRowNeedsAcknowledgementAndRotatesTheKey`。
- 週期掃描以列上同一把鍵重送結果不明的列（`payment.account_resolve`；永久拒絕記 `create_refused_at`，不再重送）。守門：`TestSweepResolvesACalledRowWithoutAnIDForANonOperatingOwnerWithTheSameKey`、`TestSweepResolvesAFailedCreateRowWithinTheWindowAndATransientFailureLeavesItForTheNextSweep`。
- 建立分兩步（D55、D59）：呼叫前記 `create_called_at`；第一步 `AcquireAccount` 拿到 ID 立刻寫進列；第二步 `ConfigureAccount` 成功才記 `create_completed_at`。「已建立」「可開通」「同步」「輪詢」看它，不看外部 ID；半建立的列對外仍是 `NOT_CREATED`。守門：`TestTheFirstStepIDIsKeptWhenTheSecondStepFailsAndAHalfCreatedRowIsNotOnboardable`、`TestStripeAcquireAccountSendsTheGivenKeyAndAFixedBodyOnEveryAttempt`。
- 擁有者不在營運中（`Owners`）時建立 job 與補排都略過。設定了 provider 時 `Owners` 與 `Inbox` 必填。守門：`TestEnsureSkipsAnInactiveOwner`、`TestNewRequiresOwnersAndRegistersTheProviderWithTheInbox`。
- 明文 fingerprint 與事件 payload 不保存在本模組、不寫 log。
