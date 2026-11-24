# media/internal/upload

呼叫方是伺服器內部的直接寫入。

## 不變量

- 內部直接寫入（B1-D14）：`PutSecure`（出貨單 PDF 與圖片，同一個分段密文格式：呼叫方宣告 size 定下段總數、邊讀邊依 `SecureChunkSize` 切段依序推進）、`PutPublic`（只收圖片，同步重新編碼）建 `READY` 暫存並回 token，擁有者在自己的 transaction 內 `Confirm`，呼叫時 `Uploader` 帶操作者；沒確認的由 GC 回收。呼叫方是伺服器內部，名額滿了等（受 ctx 約束）不回 ResourceExhausted：`PutSecure` 共用機敏名額，`PutPublic` 的解碼受 `model.PublicPutConcurrency` 約束。不新增資產狀態與身分型別。
