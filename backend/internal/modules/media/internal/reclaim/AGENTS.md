# media/internal/reclaim

擁有者的刪除請求、套用 job、GC 與實體刪除（B1-D25）。

## 不變量

- `MarkDeleted` 只 INSERT `asset_deletion_requests`（只對有 `CONFIRMED` 認領的資產，冪等），不 Enqueue、不更新列；`ApplyDeletions`（River `media.apply_deletions`，`ApplyInterval`）才把資產標 `DELETING`，同一個 job 再做三段式實體刪除與 settle。物理 `DELETING` 的已確認資產一定有刪除請求。守門：`TestMarkDeleted_RequestsOnlyConfirmedAssetsAndEnqueuesNothing`、`TestOrphanDeletingInvariantHoldsAndTheCheckIsNotVacuous`、`TestApplyDeletionsIdempotent`。
- GC 與 Confirm 以 `upload_claims.upload_id` 的唯一鍵互斥；GC 每列一個 transaction 並 `SET LOCAL lock_timeout`（`GCLockTimeout`），撞上擁有者未提交的認領就跳過、下一輪再試，所以多 token 的擁有者 transaction（ConfirmFields、moderation 的複製）不會與 GC 死結或被拖住。守門：`TestConfirmFieldsAndGCNeverDeadlockOrStall`、`TestConfirmAndGC_ConcurrentOnlyOneSucceeds`。
- 認領與請求列隨物理刪除完成一起刪（同一個 transaction，已確認的暫存列轉 `DELETED`），不另設保留期；media 對 `assets`、`staged_uploads` 只做不改主鍵的 UPDATE（`FOR NO KEY UPDATE`），與擁有者 FK 檢查的 `FOR KEY SHARE` 相容。
- 保全（`asset_holds`，D34 只登記在 SECURE 證據副本）：被保全的資產永遠不離開原狀態（`STAGED` 的不會變 `DELETING`），被保全資產所屬的暫存列不會留在 `DELETING`；判斷只看 `until > now`，`media.expire_holds` 只是清列。第一道在 SQL：`ApplyDeletionRequests` 的 `picked`、`ClaimAssetsOfUploads`、`MarkReclaimedUploadDeleting` 帶 `NOT EXISTS` 生效保全，被保全的請求維持 `applied_at IS NULL`，`reclaimUpload` 在認領無列時回滾整個 transaction（含 `RECLAIMED` 認領）。
- 最後一道在 `DeleteAssets`：`deleteUpload`、`deleteAsset` 刪物件前開一個 transaction 鎖列再檢查，有保全就退回（資產回 `STAGED` 並清回請求的 `applied_at`，暫存列回 `READY` 並刪 `RECLAIMED` 認領），不刪任何物件。
- `Hold` 以 `FOR SHARE` 鎖資產列並要求 `STAGED`（否則 `ErrAssetGone`），與認領的更新互斥：認領提交之後的 `Hold` 一定回 `ErrAssetGone`，認領之前提交的保全一定被刪前再檢查看見。守門：`TestApplyDeletionsSkipsHeldAsset`、`TestExpiredHoldAllowsDeletion`、`TestClaimAssetsOfUploadsSkipsHeldAsset`、`TestReclaimUploadWithHeldAssetKeepsUploadStateAndRollsBackClaim`、`TestDeleteUploadRechecksHoldBeforeDeletingObjects`、`TestDeleteAssetRechecksHoldBeforeDeletingObject`、`TestHoldOnDeletingAssetReturnsAssetGone`、`TestTwoHoldSourcesStackAndReleasingOneKeepsAsset`（CI）。
