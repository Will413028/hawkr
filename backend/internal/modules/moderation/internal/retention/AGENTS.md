# moderation/internal/retention

快照的保留期清除與檢舉人刪帳時的資料清除。

## 不變量

- 刪帳訂閱者在 accountdeletion 鎖完買家與店家之後，才鎖 `reports` 與 `reporter_daily_counts`；送出路徑的鎖順序見 `internal/report/AGENTS.md`（`internal/platform/lockorder`）。
- 店家退場訂閱者（`OnStoreRetired`）在 seller 的退場 transaction 內執行，seller 已先鎖店家列（`RetireDueStores` 的 `FOR UPDATE`），之後才鎖 `reports`、`report_requests`、`reporter_daily_counts`；店家檢舉人的送出（`report.attempt` 的 `lockReporter`）開頭先以 `LockNotRetired`（`FOR KEY SHARE`）鎖店家列，再鎖 moderation 的列，兩者都是店家列先於 moderation，並行時在店家列上序列化：送出先到，退場這一輪被 `SKIP LOCKED` 跳過；退場先到，送出等它提交後看到已退場回 NotFound（`TestRetiredStoreCannotSubmitAsReporter`）。
- 鎖的必要條件：新檢舉在凍結時已因 `CapChatRead` 被擋，鎖擋的是凍結前返回的重送與併入進行中案件（`recordRequest`）。
- 買家檢舉人同理以 `Buyer.LockNotClosed` 與 `OnBuyerDeleted` 序列化（`TestClosedBuyerCannotSubmitAsReporter`），買家沒有凍結前的擋，鎖是唯一守門。依賴 `reports_store_reporter_time` 索引找該店的檢舉列。
- 快照的圖片是送出時 `CopyPublic` 複製並在建快照的 transaction 內 `Confirm` 的複本，擁有者（作者、店家、檢舉人）的刪除與退場從不動複本，只有保留期清理（`internal/retention`）在快照過期刪除時對它擁有的複本 `MarkDeleted`（B1-D20；`TestRetireAssetsMarksEveryOwnedCopyOnceAndSkipsMediaWhenEmpty`、`TestASnapshotKeepsItsOwnConfirmedCopyOfTheImage`、`TestASnapshotRollbackLeavesTheCopyUnconfirmed`）；檢舉人刪帳時同 transaction 清他的說明、冪等紀錄與每日計數列，檢舉列與 `reporter_buyer_id` 留著（`TestReporterDeletionClearsDescriptionsAndDailyCountButKeepsTheReports`）；店家退場同理。
