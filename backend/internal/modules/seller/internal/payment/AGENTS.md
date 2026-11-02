# seller/internal/payment

收款開通：provider 中立的 port、帳戶快取、同步 job 與 webhook inbox。

## 不變量

- 外部呼叫（建立帳戶、重讀帳戶、開通 session）不在 transaction 內；一店一個建立 job、一店一個同步 job（唯一鍵 `store_id`）。守門：`TestPaymentJobsAreUniquePerStoreInRiver`。
- 狀態只由帳戶快照推導（`StatusOf`），沒有觀測歷史與事件時間排序，重讀即最新；`transfers_active` 是閘門的唯一輸入（view `store_requirement_flags.onboarding_complete`）。
- 外部事件先寫 inbox（`payment_provider_events`，(provider, event_id) 唯一），重送的事件不再排 job。事件排入的同步可能被執行中的 job 合併掉，所以 inbox 是不漏的來源：輪詢掃描對任何狀態、有超過 1 分鐘還沒處理的事件的店家補排同步；同步只把「讀取開始前」收到的事件標為已處理。守門：`TestSweepQueuesSyncForAnOldUnprocessedEventOnAnEnabledAccount`。
- 輪詢也排 DISABLED 但原因碼不是 `rejected*`、`unsupported*` 的帳戶（Stripe 可以解除 restricted，掉了 webhook 時只有輪詢補得到）。守門：`TestSweepQueuesSyncForOpenAndLiftableStatusesOnly`。
- 建立外部帳戶的冪等鍵是「建立中」列寫入時產生並保存的 UUID（`create_idempotency_key`），每次重試沿用；收件方式與開通介面取自列上的 `requirement_collection`，不取行程設定。守門：`TestApprovalQueuesOneAccountCreationAndAReplayedCreateKeepsOneExternalAccount`、`TestStripeEnsureAccountUsesTheDashboardOfTheStoredCollection`。
- 帳戶列不存個資；一店一列，關店再開沿用。
