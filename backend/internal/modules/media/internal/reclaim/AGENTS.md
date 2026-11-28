# media/internal/reclaim

擁有者的刪除請求、套用 job、GC 與實體刪除（B1-D25）。

## 不變量

- `MarkDeleted` 只 INSERT `asset_deletion_requests`（只對有 `CONFIRMED` 認領的資產，冪等），不 Enqueue、不更新列；`ApplyDeletions`（River `media.apply_deletions`，`ApplyInterval`）才把資產標 `DELETING`，同一個 job 再做三段式實體刪除與 settle。物理 `DELETING` 的已確認資產一定有刪除請求。守門：`TestMarkDeleted_RequestsOnlyConfirmedAssetsAndEnqueuesNothing`、`TestOrphanDeletingInvariantHoldsAndTheCheckIsNotVacuous`、`TestApplyDeletionsIdempotent`。
- GC 與 Confirm 以 `upload_claims.upload_id` 的唯一鍵互斥；GC 每列一個 transaction 並 `SET LOCAL lock_timeout`（`GCLockTimeout`），撞上擁有者未提交的認領就跳過、下一輪再試，所以多 token 的擁有者 transaction（ConfirmFields、moderation 的複製）不會與 GC 死結或被拖住。守門：`TestConfirmFieldsAndGCNeverDeadlockOrStall`、`TestConfirmAndGC_ConcurrentOnlyOneSucceeds`。
- 認領與請求列隨物理刪除完成一起刪（同一個 transaction，已確認的暫存列轉 `DELETED`），不另設保留期；media 對 `assets`、`staged_uploads` 只做不改主鍵的 UPDATE（`FOR NO KEY UPDATE`），與擁有者 FK 檢查的 `FOR KEY SHARE` 相容。
- 保全（D34 只登記在 SECURE 證據副本）：被保全的資產永遠不離開原狀態（`STAGED` 的不會變 `DELETING`），被保全資產所屬的暫存列不會留在 `DELETING`；有效保全是 `until > now` 且沒有解除列。
- 擁有者 transaction 內的 `Hold`、`ExtendHold`、`ReleaseHold` 只 INSERT 與 SELECT：`Hold` 先 INSERT `asset_gates`（`HOLD`，`ON CONFLICT DO NOTHING`）再 SELECT 閘門，是 `DELETE` 就回 `ErrAssetGone`，之後 INSERT `asset_holds`（外鍵指向 `HOLD` 閘門）；`ExtendHold` 只對生效中的來源 INSERT 較晚的新列，`ReleaseHold` INSERT `asset_hold_releases`。
- 與刪除的互斥靠閘門的唯一鍵，同 Confirm 與 GC：把資產推進 `DELETING` 的路徑（`applyRequest`、`reclaimUpload`）都在帶 `lock_timeout`（`GCLockTimeout`）的 transaction 內先 `hold.Claim`：鎖 `HOLD` 閘門、清到期與已解除的保全列、沒有剩餘保全就撤掉閘門，再 INSERT `DELETE` 閘門，讀回不是 `DELETE` 就放棄。
- 放棄時 `reclaimUpload` 回滾整個 transaction（含 `RECLAIMED` 認領），被保全的請求維持 `applied_at IS NULL`；撞上擁有者未提交的閘門或保全列（`55P03`）或外鍵（`23503`）就跳過、下一輪再試，不會死結或被拖住。認領提交之後的 `Hold` 一定回 `ErrAssetGone`，認領之前提交的保全一定被認領看見。
- `media.expire_holds` 與複製完成的釋放走同一套（鎖閘門、刪列、撤閘門）。唯一殘餘：擁有者讀到只剩到期保全的閘門、worker 在擁有者 INSERT 之前撤掉它，擁有者的 `Hold` 回「撞上資產整理，請重試」，transaction 失敗，不會誤登記。
- 最後一道在 `DeleteAssets`：`deleteUpload`、`deleteAsset` 刪物件前開一個 transaction 鎖列再檢查，有保全就退回（資產回 `STAGED` 並清回請求的 `applied_at`，暫存列回 `READY` 並刪 `RECLAIMED` 認領），不刪任何物件。
- 保全守門：`TestApplyDeletionsSkipsHeldAsset`、`TestExpiredHoldAllowsDeletion`、`TestClaimGateRefusesHeldAssetAndTakesFreeOne`、`TestReclaimUploadWithHeldAssetKeepsUploadStateAndRollsBackClaim`、`TestDeleteUploadRechecksHoldBeforeDeletingObjects`、`TestDeleteAssetRechecksHoldBeforeDeletingObject`、`TestHoldOnDeletingAssetReturnsAssetGone`、`TestHoldAfterTheDeletionGateCommitsReturnsAssetGoneEvenWhenStateLooksStaged`、`TestTwoHoldSourcesStackAndReleasingOneKeepsAsset`（CI）。
- 並行守門：`TestWorkerSkipsAnUncommittedHoldWithoutWaitingOrDeleting`、`TestGCSkipsAnUncommittedHoldOnAStagedUpload`（CI）。
