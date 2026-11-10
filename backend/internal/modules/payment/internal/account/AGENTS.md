# payment/internal/account

收款帳戶：provider 中立的 port、帳戶快取、建立／同步／輪詢／刪除 job，以及收款事件的 inbox handler。

## 不變量

- 外部呼叫（建立帳戶、重讀帳戶、開通 session）不在 transaction 內；一個擁有者一個建立 job、一個同步 job（唯一鍵 `store_id`）。守門：`TestJobsAreUniquePerOwnerInRiver`。
- 狀態只由帳戶快照推導（`StatusOf`），沒有觀測歷史與事件時間排序，重讀即最新。守門：`TestStatusOfFollowsTheDecisionTable`。
- 快取不倒退（D32）：`ApplyPaymentSnapshot` 只在這次讀取的開始時間不早於已存的 `fetched_at`（NULL 一律寫）時寫入；被擋下的同步不標記 inbox 事件、不發布 `PayoutAccountChanged`、`again` 為 false。必要條件：除了 job 之外還有同步呼叫（seller 的 `GetMyOnboarding` 在 `refresh` 時同步呼叫 `SyncAccount`），可能與 job 並行，「一個帳戶一個同步」只剩 River 唯一鍵；那個呼叫拿掉時這個條件一起拿掉。守門：`TestAnOlderReadNeverOverwritesANewerCommittedRead`、`TestARejectedSyncDoesNotMarkEventsOrPublish`。
- `RequestAccount` 在擁有者的 transaction 內只 INSERT（`ON CONFLICT DO NOTHING`）並排 job，不鎖既有列。守門：`TestRequestKeepsAnExistingRowAndItsContactEmail`。
- 同步在寫快照的 transaction 內發 `PayoutAccountChanged` 並以 platform inbox 標記「讀取開始前」收到的事件；handler 只排同步、不標記（對不到帳戶的除外），被合併的同步靠 `RoutePending` 再跑一輪；輪詢對有超過寬限（`eventGrace`）未處理事件的帳戶補排同步。守門：`TestAnEventArrivingWhileTheAccountIsBeingReadIsNotLostWhenItsSyncIsCoalesced`、`TestSweepQueuesSyncForAnOldUnprocessedEventOnAnEnabledAccount`。
- 輪詢也排 DISABLED 但原因碼不是 `rejected*`、`unsupported*` 的帳戶。守門：`TestSweepQueuesSyncForOpenAndLiftableStatusesOnly`。
- 建立外部帳戶的冪等鍵是「建立中」列寫入時產生並保存的 UUID，每次重試沿用；收件方式取自列上的 `requirement_collection`。守門：`TestRequestQueuesOneAccountCreationAndAReplayedCreateKeepsOneExternalAccount`、`TestStripeEnsureAccountUsesTheDashboardOfTheStoredCollection`。
- 擁有者不在營運中（`Owners`）時建立 job 與補排都略過。設定了 provider 時 `Owners` 與 `Inbox` 必填。守門：`TestEnsureSkipsAnInactiveOwner`、`TestNewRequiresOwnersAndRegistersTheProviderWithTheInbox`。
- 明文 fingerprint 與事件 payload 不保存在本模組、不寫 log。
