# media/internal/reclaim

擁有者的刪除請求、套用 job、GC 與實體刪除（B1-D25）。

## 不變量

- `MarkDeleted` 只 INSERT `asset_deletion_requests`（只對有 `CONFIRMED` 認領的資產，冪等），不 Enqueue、不更新列；`ApplyDeletions`（River `media.apply_deletions`，`ApplyInterval`）才把資產標 `DELETING`，同一個 job 再做三段式實體刪除與 settle。物理 `DELETING` 的已確認資產一定有刪除請求。守門：`TestMarkDeleted_RequestsOnlyConfirmedAssetsAndEnqueuesNothing`、`TestOrphanDeletingInvariantHoldsAndTheCheckIsNotVacuous`、`TestApplyDeletionsIdempotent`。
- GC 與 Confirm 以 `upload_claims.upload_id` 的唯一鍵互斥；GC 每列一個 transaction 並 `SET LOCAL lock_timeout`（`GCLockTimeout`），撞上擁有者未提交的認領就跳過、下一輪再試，所以多 token 的擁有者 transaction（ConfirmFields、moderation 的複製）不會與 GC 死結或被拖住。守門：`TestConfirmFieldsAndGCNeverDeadlockOrStall`、`TestConfirmAndGC_ConcurrentOnlyOneSucceeds`。
- 認領與請求列隨物理刪除完成一起刪（同一個 transaction，已確認的暫存列轉 `DELETED`），不另設保留期；media 對 `assets`、`staged_uploads` 只做不改主鍵的 UPDATE（`FOR NO KEY UPDATE`），與擁有者 FK 檢查的 `FOR KEY SHARE` 相容。
