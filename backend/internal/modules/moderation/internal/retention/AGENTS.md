# moderation/internal/retention

快照的保留期清除與檢舉人刪帳時的資料清除。

## 不變量

- 刪帳訂閱者在 accountdeletion 鎖完買家與店家之後，才鎖 `reports` 與 `reporter_daily_counts`；送出路徑的鎖順序見 `internal/report/AGENTS.md`（`internal/platform/lockorder`）。
