# media/internal/upload

呼叫方是伺服器內部的直接寫入。

## 不變量

- 內部直接寫入（B1-D14）：`PutSecure`（出貨單 PDF 與圖片，同一個分段密文格式：呼叫方宣告 size 定下段總數、邊讀邊依 `SecureChunkSize` 切段依序推進）、`PutPublic`（只收圖片，同步重新編碼）建 `READY` 暫存並回 token，擁有者在自己的 transaction 內 `Confirm`，呼叫時 `Uploader` 帶操作者；沒確認的由 GC 回收。呼叫方是伺服器內部，名額滿了等（受 ctx 約束）不回 ResourceExhausted：`PutSecure` 共用機敏名額，`PutPublic` 的解碼受 `model.PublicPutConcurrency` 約束。不新增資產狀態與身分型別。
- 失敗清理的 `Store.Delete`（`direct.go` 的 `abortDirect`、`process.go` 的 `finish` 與 `lost`、`secure_upload.go` 的 `deleteChunkObject`）只刪尚未成為資產的 key：`abortDirect` 只在 `MarkUploadRejected` 成功（暫存列不是 `READY`、沒有資產）之後刪；`finish` 刪的是 `tmp_key`；`lost` 在暫存列是 `READY`／`CONFIRMED` 時直接回、其餘狀態沒有資產，或資產已在刪除（暫存列 `DELETING`，被保全的資產其暫存列不會是 `DELETING`）；`deleteChunkObject` 刪的是這次請求自己寫的隨機 key，且先確認 `secure_chunks` 沒有記錄它。所以這四處不需要保全再檢查。
- `CopyToSecure` 兩段式（單體規則 5）：擁有者 transaction 內只寫副本資產（`STAGED`、SECURE、預先包好的 DEK）、已確認的暫存列與認領、`asset_copy_requests`，並在來源原圖登記來源為 `media.copy_to_secure` 的短暫保全（`CopyHoldTTL`，複製完成即釋放，不是法律保全）；`ProcessCopies`（`media.copy_to_secure` 排程）在 transaction 外讀來源、加密、寫各段，再用一個 transaction 完成請求、記段並釋放保全；單筆失敗不影響其他筆，來源已不在則放棄請求並釋放。
- 完成前副本的 `ReadSecure` 回 NotFound。守門：`TestCopyToSecureProducesIndependentSecureCopy`、`TestCopyToSecureKeepsOriginalUntilCopyCompletes`、`TestCopyToSecureAbandonsTheRequestWhenTheOriginalIsGone`（CI）。
