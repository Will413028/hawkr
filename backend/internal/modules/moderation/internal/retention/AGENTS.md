# moderation/internal/retention

快照的保留期清除與檢舉人刪帳時的資料清除。

## 不變量

- 刪帳訂閱者在 accountdeletion 鎖完買家與店家之後，才鎖 `reports` 與 `reporter_daily_counts`；送出路徑的鎖順序見 `internal/report/AGENTS.md`（`internal/platform/lockorder`）。
- 店家退場訂閱者（`OnStoreRetired`）在 seller 的退場 transaction 內執行，seller 已先鎖店家列（`RetireDueStores` 的 `FOR UPDATE`），之後才鎖 `reports`、`report_requests`、`reporter_daily_counts`；店家檢舉人的送出（`report.attempt`）開頭先以 `LockNotRetired`（`FOR KEY SHARE`）鎖店家列，再鎖 moderation 的列，兩者都是店家列先於 moderation，並行時在店家列上序列化：送出先到，退場這一輪被 `SKIP LOCKED` 跳過；退場先到，送出等它提交後看到已退場回 NotFound（`TestRetiredStoreCannotSubmitAsReporter`）。依賴 `reports_store_reporter_time` 索引找該店的檢舉列。
