# catalog/internal/restock

補貨通知在 catalog 端的部分（D26）：偵測 SKU 由不可買變可買、寫進佇列表，週期 worker 重新確認後呼叫 `RestockNotifier`；商品或 SKU 刪除後清訂閱走另一張同形的佇列。

## 不變量

- 偵測與排入和庫存、狀態寫入同一個 transaction（`db.InTx` 加入呼叫方的）；通知 port 只在 worker 內、transaction 外呼叫（單體規則 5）。守門：`TestRestockIsEnqueuedInTheCallersTransaction`。
- 可買是庫存大於 0 ∧ SKU 未刪 ∧ 商品 ACTIVE 且未刪 ∧ 店家可曝光；排入時與 worker 執行時各確認一次，執行時不可買就刪列、不通知。店家恢復曝光不觸發（Will 決定不做）。守門：`TestRestockRechecksPurchasabilityWhenItRuns`。
- `restock_queue` 每個 SKU 最多一列：沒被認領就合併（保留較早的 `due_at` 與原本的 `requested_at`），被認領（`claimed_until` 非空）就只把 `requested_at` 往後推至少 1 微秒；worker 完成只在 `requested_at` 仍是認領時讀到的值才刪，否則重新排（`Rearm`）。處理中的轉換因此不會遺失。守門：`TestRestockTransitionWhileTheRowIsClaimedIsNotLost`、`TestRestockTransitionsWithinTheWindowCoalesceIntoOneNotification`。
- 切成 ACTIVE 對商品每個沒刪除的 SKU 都排，不讀庫存：與另一個 SKU 的 0 到正庫存並行時雙方的 transaction 互相看不到對方，讀庫存會漏通知。守門：`TestActivationEnqueuesEveryLiveSkuRegardlessOfStock`。
- 認領用 `FOR UPDATE SKIP LOCKED` 寫租約（`Lease`），crash 後租約過了重新處理；通知是一次性訂閱，重處理安全。守門：`TestRestockClaimedRowIsReprocessedOnlyAfterTheLeaseExpires`。
- 失敗在同一列 `attempts` 加一、`due_at` 指數退避（`Backoff`）、記 `last_error`；到 `MaxAttempts` 設 `failed_at` 停手，列保留不刪，之後的新請求重置它。守門：`TestRestockNotifierFailureBacksOffAndStopsAtMaxAttempts`。
- 清訂閱佇列同一套認領、完成與失敗，一輪的列合併成一次 `PurgeSKUSubscriptions`。守門：`TestDeletingAProductOrSkuEnqueuesAPurgeOfItsSubscriptions`。
- 週期 job 由模組的 `PeriodicJobs()` 與 `AddWorkers` 匯出；`cmd/hawkr` 在 B4-6 之前不登記 catalog。
